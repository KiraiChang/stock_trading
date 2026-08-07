package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/store"
)

// --- stubs ---

type marketBackfillJobRepoStub struct {
	mu        sync.Mutex
	jobs      map[string]*store.MarketBackfillJob
	createErr error
	getErr    error
	// progressCalls 記錄每次 UpdateProgress 的 (done, failed)，用來驗證進度是逐檔推進的
	progressCalls [][2]int
}

func newMarketBackfillJobRepoStub() *marketBackfillJobRepoStub {
	return &marketBackfillJobRepoStub{jobs: map[string]*store.MarketBackfillJob{}}
}

func (r *marketBackfillJobRepoStub) Create(_ context.Context, job *store.MarketBackfillJob) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	job.ID = uint64(len(r.jobs) + 1)
	if job.Failures == "" {
		job.Failures = "[]"
	}
	copied := *job
	r.jobs[job.JobID] = &copied
	return nil
}

func (r *marketBackfillJobRepoStub) UpdateProgress(_ context.Context, jobID string, done, failed int, failures store.RawJSON) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.progressCalls = append(r.progressCalls, [2]int{done, failed})
	job, ok := r.jobs[jobID]
	if !ok {
		return errors.New("not found")
	}
	job.Status = "running"
	job.SymbolsDone = done
	job.SymbolsFailed = failed
	job.Failures = failures
	return nil
}

func (r *marketBackfillJobRepoStub) Finish(_ context.Context, jobID, status, errMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok := r.jobs[jobID]
	if !ok {
		return errors.New("not found")
	}
	job.Status = status
	if errMsg != "" {
		job.Error.Valid, job.Error.String = true, errMsg
	}
	return nil
}

func (r *marketBackfillJobRepoStub) GetByJobID(_ context.Context, jobID string) (*store.MarketBackfillJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getErr != nil {
		return nil, r.getErr
	}
	job, ok := r.jobs[jobID]
	if !ok {
		// 回真正的 sql.ErrNoRows 而不是訊息長得像的普通 error：sqlx 的 GetContext
		// 查無資料時回的就是這個哨兵值，handler 靠 errors.Is 分辨 404 與 500。
		return nil, sql.ErrNoRows
	}
	copied := *job
	return &copied, nil
}

// snapshot 取得任務目前狀態（含背景 goroutine 寫入的部分）
func (r *marketBackfillJobRepoStub) snapshot(jobID string) store.MarketBackfillJob {
	job, _ := r.GetByJobID(context.Background(), jobID)
	if job == nil {
		return store.MarketBackfillJob{}
	}
	return *job
}

// marketSourceStub 只實作 BackfillHistory 走到的 FetchDailyCandles
type marketSourceStub struct {
	failFor map[string]bool
	mu      sync.Mutex
	fetched []string
}

func (s *marketSourceStub) FetchDailyCandles(_ context.Context, symbol string, _, _ time.Time) ([]market.Candle, error) {
	s.mu.Lock()
	s.fetched = append(s.fetched, symbol)
	s.mu.Unlock()
	if s.failFor[symbol] {
		return nil, errors.New("fetch failed for " + symbol)
	}
	return []market.Candle{{Symbol: symbol, Timeframe: "1d", Close: 100, Timestamp: time.Now()}}, nil
}

func (s *marketSourceStub) FetchMinuteCandles(context.Context, string, time.Time) ([]market.Candle, error) {
	return nil, errors.New("not used")
}

type marketCandleRepoStub struct{ store.CandleRepo }

func (marketCandleRepoStub) BulkInsert(context.Context, []store.Candle) error { return nil }

// --- helpers ---

func newMarketTestHandler(src market.MarketDataSource) (*MarketHandler, *marketBackfillJobRepoStub) {
	jobs := newMarketBackfillJobRepoStub()
	fetcher := market.NewFetcher(src, marketCandleRepoStub{}, zap.NewNop())
	return NewMarketHandler(fetcher, jobs, zap.NewNop()), jobs
}

func postBackfill(t *testing.T, h *MarketHandler, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/market/backfill", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	h.Backfill(c)
	return w
}

// waitForStatus 等背景 goroutine 把任務推到終態；逾時直接 fail 而不是靜靜通過。
func waitForStatus(t *testing.T, jobs *marketBackfillJobRepoStub, jobID string, want string) store.MarketBackfillJob {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job := jobs.snapshot(jobID)
		if job.Status == want {
			return job
		}
		time.Sleep(5 * time.Millisecond)
	}
	job := jobs.snapshot(jobID)
	t.Fatalf("等不到 status=%q，目前 = %q（done=%d failed=%d）", want, job.Status, job.SymbolsDone, job.SymbolsFailed)
	return job
}

// marketBackfillJobWire 對應端點實際吐出的 JSON，刻意不重用 store.MarketBackfillJob：
// RawJSON 只實作 MarshalJSON（它是 DB 直通字串），failures 送出去是陣列、
// 收回來卻是 string，反序列化必失敗。獨立宣告反而更貼近前端看到的形狀，
// 也順便釘住「failures 在 wire 上是物件陣列而非字串」這件 contract
// （frontend/src/lib/api/market.ts 的 MarketBackfillFailure[] 依此宣告）。
type marketBackfillJobWire struct {
	ID            uint64  `json:"id"`
	JobID         string  `json:"job_id"`
	Symbols       string  `json:"symbols"`
	Days          int     `json:"days"`
	Status        string  `json:"status"`
	SymbolsTotal  int     `json:"symbols_total"`
	SymbolsDone   int     `json:"symbols_done"`
	SymbolsFailed int     `json:"symbols_failed"`
	Error         *string `json:"error"`
	StartedAt     *string `json:"started_at"`
	FinishedAt    *string `json:"finished_at"`
	Failures      []struct {
		Symbol string `json:"symbol"`
		Error  string `json:"error"`
	} `json:"failures"`
}

func decodeJob(t *testing.T, w *httptest.ResponseRecorder) marketBackfillJobWire {
	t.Helper()
	var resp struct {
		Job marketBackfillJobWire `json:"job"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("回應不是 {job:...}: %v body=%s", err, w.Body.String())
	}
	return resp.Job
}

// --- tests ---

func TestMarketBackfillRequiresSymbols(t *testing.T) {
	// symbols 是必填：先前會自動代入 watchlist，讓這支端點無法用於 watchlist 以外的標的。
	cases := []struct {
		name string
		body string
	}{
		{"缺 symbols 鍵", `{"days":120}`},
		{"symbols 為空陣列", `{"days":120,"symbols":[]}`},
		{"整個 body 是空的", `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, jobs := newMarketTestHandler(&marketSourceStub{})
			w := postBackfill(t, h, tc.body)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400，body=%s", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), "symbols is required") {
				t.Fatalf("錯誤訊息 = %s, want symbols is required", w.Body.String())
			}
			jobs.mu.Lock()
			n := len(jobs.jobs)
			jobs.mu.Unlock()
			if n != 0 {
				t.Fatalf("400 不該建立任務，卻建了 %d 筆", n)
			}
		})
	}
}

func TestMarketBackfillAcceptedReturnsJob(t *testing.T) {
	src := &marketSourceStub{}
	h, jobs := newMarketTestHandler(src)

	w := postBackfill(t, h, `{"days":1825,"symbols":["2330","2454"]}`)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202，body=%s", w.Code, w.Body.String())
	}
	job := decodeJob(t, w)
	if job.JobID == "" || !strings.HasPrefix(job.JobID, "bf_") {
		t.Fatalf("job_id = %q, want bf_ 開頭（前端靠它輪詢）", job.JobID)
	}
	if job.Status != "pending" {
		t.Fatalf("status = %q, want pending", job.Status)
	}
	if job.Days != 1825 {
		t.Fatalf("days = %d, want 1825", job.Days)
	}
	if job.SymbolsTotal != 2 {
		t.Fatalf("symbols_total = %d, want 2", job.SymbolsTotal)
	}
	if job.Symbols != `["2330","2454"]` {
		t.Fatalf("symbols = %q, want [\"2330\",\"2454\"]", job.Symbols)
	}
	// failures 必須是空陣列而不是 null——前端直接 .map() 它，null 會炸。
	if job.Failures == nil {
		t.Fatalf("failures 是 null，want []，body=%s", w.Body.String())
	}

	final := waitForStatus(t, jobs, job.JobID, "done")
	if final.SymbolsDone != 2 || final.SymbolsFailed != 0 {
		t.Fatalf("done/failed = %d/%d, want 2/0", final.SymbolsDone, final.SymbolsFailed)
	}
	// 背景回補只能補送出的那兩檔，不能自己去撈 watchlist。
	src.mu.Lock()
	fetched := append([]string(nil), src.fetched...)
	src.mu.Unlock()
	if len(fetched) != 2 || fetched[0] != "2330" || fetched[1] != "2454" {
		t.Fatalf("實際抓取 = %v, want [2330 2454]", fetched)
	}
}

func TestMarketBackfillDefaultsDays(t *testing.T) {
	h, jobs := newMarketTestHandler(&marketSourceStub{})

	w := postBackfill(t, h, `{"symbols":["2330"]}`)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", w.Code)
	}
	job := decodeJob(t, w)
	if job.Days != 120 {
		t.Fatalf("未指定 days 時 = %d, want 預設 120", job.Days)
	}
	waitForStatus(t, jobs, job.JobID, "done")
}

func TestMarketBackfillPartialFailureRecordsFailures(t *testing.T) {
	src := &marketSourceStub{failFor: map[string]bool{"2454": true}}
	h, jobs := newMarketTestHandler(src)

	w := postBackfill(t, h, `{"symbols":["2330","2454","2317"]}`)
	jobID := decodeJob(t, w).JobID

	final := waitForStatus(t, jobs, jobID, "partial")
	if final.SymbolsDone != 3 || final.SymbolsFailed != 1 {
		t.Fatalf("done/failed = %d/%d, want 3/1", final.SymbolsDone, final.SymbolsFailed)
	}
	if !final.Error.Valid || final.Error.String != "some symbols failed" {
		t.Fatalf("error = %+v, want some symbols failed", final.Error)
	}

	var failures []struct {
		Symbol string `json:"symbol"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal([]byte(final.Failures), &failures); err != nil {
		t.Fatalf("failures 不是合法 JSON: %v (%q)", err, final.Failures)
	}
	if len(failures) != 1 || failures[0].Symbol != "2454" {
		t.Fatalf("failures = %+v, want 只有 2454", failures)
	}
	if !strings.Contains(failures[0].Error, "fetch failed for 2454") {
		t.Fatalf("failures[0].error = %q, 應帶出原始錯誤供前端顯示", failures[0].Error)
	}

	// 進度是逐檔回報的（3 檔 → 3 次），不是跑完才一次寫入。
	jobs.mu.Lock()
	calls := append([][2]int(nil), jobs.progressCalls...)
	jobs.mu.Unlock()
	if len(calls) != 3 {
		t.Fatalf("UpdateProgress 呼叫 %d 次, want 3（每檔一次）: %v", len(calls), calls)
	}
	if calls[0] != [2]int{1, 0} || calls[1] != [2]int{2, 1} || calls[2] != [2]int{3, 1} {
		t.Fatalf("進度推進不正確: %v, want [{1 0} {2 1} {3 1}]", calls)
	}
}

func TestMarketBackfillAllFailed(t *testing.T) {
	src := &marketSourceStub{failFor: map[string]bool{"2330": true, "2454": true}}
	h, jobs := newMarketTestHandler(src)

	w := postBackfill(t, h, `{"symbols":["2330","2454"]}`)
	final := waitForStatus(t, jobs, decodeJob(t, w).JobID, "failed")
	if final.SymbolsFailed != 2 {
		t.Fatalf("failed = %d, want 2", final.SymbolsFailed)
	}
	if !final.Error.Valid || final.Error.String != "all symbols failed" {
		t.Fatalf("error = %+v, want all symbols failed", final.Error)
	}
}

func TestMarketGetBackfillJob(t *testing.T) {
	src := &marketSourceStub{}
	h, jobs := newMarketTestHandler(src)
	w := postBackfill(t, h, `{"symbols":["2330"]}`)
	jobID := decodeJob(t, w).JobID
	waitForStatus(t, jobs, jobID, "done")

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/market/backfill/"+jobID, nil)
	c.Params = gin.Params{{Key: "job_id", Value: jobID}}
	h.GetBackfillJob(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200，body=%s", rec.Code, rec.Body.String())
	}
	job := decodeJob(t, rec)
	if job.JobID != jobID || job.Status != "done" || job.SymbolsDone != 1 {
		t.Fatalf("job = %+v, want job_id=%s status=done done=1", job, jobID)
	}
}

func TestMarketGetBackfillJobNotFound(t *testing.T) {
	h, _ := newMarketTestHandler(&marketSourceStub{})

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/market/backfill/bf_nope", nil)
	c.Params = gin.Params{{Key: "job_id", Value: "bf_nope"}}
	h.GetBackfillJob(c)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404，body=%s", rec.Code, rec.Body.String())
	}
}

func TestMarketGetBackfillJobDBErrorReturns500(t *testing.T) {
	// DB 掛掉不是「任務不存在」：先前所有 repo 錯誤都被當成 404，
	// 使用者會以為任務被清掉，實際上是資料庫連不上。
	h, jobs := newMarketTestHandler(&marketSourceStub{})
	jobs.getErr = errors.New("dial tcp: connection refused")

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/market/backfill/bf_x", nil)
	c.Params = gin.Params{{Key: "job_id", Value: "bf_x"}}
	h.GetBackfillJob(c)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500，body=%s", rec.Code, rec.Body.String())
	}
}

func TestMarketBackfillCreateJobFailureReturns500(t *testing.T) {
	jobs := newMarketBackfillJobRepoStub()
	jobs.createErr = errors.New("db is down")
	h := NewMarketHandler(market.NewFetcher(&marketSourceStub{}, marketCandleRepoStub{}, zap.NewNop()), jobs, zap.NewNop())

	w := postBackfill(t, h, `{"symbols":["2330"]}`)

	// 建不出任務就不該啟動背景回補，否則前端拿不到 job_id 卻仍在燒 rate limit。
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500，body=%s", w.Code, w.Body.String())
	}
}

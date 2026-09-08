package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/scheduler"
	"github.com/trading/backend/internal/store"
)

// 終止上市名單對帳的 API 契約（計畫書 docs/todo.md T-071 的 #14／#15）。
//
// ⛔ **這條端點同步取鎖**：202 代表真的啟動了、409 代表真的被擋住。
// 不沿用既有 evaluation_universe_sync 的「先回 202 再由背景 goroutine 靜默跳過」。

type delistingSourceStub struct {
	mu      sync.Mutex
	calls   int
	block   chan struct{}
	entered chan struct{}
}

func (s *delistingSourceStub) FetchDelistings(context.Context) (market.DelistingSourceResult, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	if s.entered != nil {
		close(s.entered)
		s.entered = nil
	}
	if s.block != nil {
		<-s.block
	}
	return market.DelistingSourceResult{
		Rows: []market.DelistingEventRow{{
			Symbol: "2301", CompanyName: "光寶電子",
			DelistedDate: time.Date(2002, 12, 23, 0, 0, 0, 0, time.UTC),
		}},
		RowCount: 265, DistinctSymbols: 265,
	}, nil
}

func (s *delistingSourceStub) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// WithTx 回傳 nil 而不呼叫 fn＝「交易內一切正常且沒有 degradation」。
// 對帳邏輯本身在 internal/market 有自己的測試，這裡只驗 API 契約。
type delistingRepoStub struct{}

func (delistingRepoStub) WithTx(context.Context, func(store.DelistingTx) error) error { return nil }

type delistingReadinessStub struct{}

func (delistingReadinessStub) StockSymbolSyncSucceededToday(context.Context, time.Time) (bool, error) {
	return true, nil
}

func schedulerWithDelisting(source market.DelistingSource) *scheduler.Scheduler {
	// ⛔ jobRuns 不能給 nil：這個 job 的每一輪都從 startRun 開始（runID = 0 就中止），
	// 給 nil 會在背景 goroutine 裡 nil deref、整個測試行程掛掉。
	s := scheduler.New(
		nil, nil, nil, &jobRunRepoStub{}, nil, nil, nil, "0 21 * * *", nil, "", false,
		nil, nil, nil, nil, config.SREvaluationConfig{}, false, zap.NewNop(),
	)
	s.SetDelistingReconcile(
		market.NewDelistingReconciler(source, delistingRepoStub{}, delistingReadinessStub{}, zap.NewNop()),
		config.DelistingConfig{Enabled: true},
	)
	return s
}

func newDelistingHandler(source market.DelistingSource) (*scheduler.Scheduler, *gin.Engine) {
	s := schedulerWithDelisting(source)
	h := NewSchedulerHandler(&jobRunRepoStub{}, s, zap.NewNop())

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/scheduler/delisting-reconcile/run", h.RunDelistingReconcile)
	return s, r
}

func postDelisting(r *gin.Engine, query string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/scheduler/delisting-reconcile/run"+query, nil))
	return w
}

// #14：不加進 knownSchedulerJobs 與 jobStaleThreshold 的話，DB 有紀錄
// 但 GET /scheduler/status 不會回傳它。
func TestSchedulerStatusIncludesDelistingReconcile(t *testing.T) {
	source := &delistingSourceStub{}
	sched := schedulerWithDelisting(source)
	sched.Start()
	defer sched.Stop()

	got := statusByJob(t, sched, []store.JobRun{{
		ID: 1, JobName: "delisting_reconcile", Status: "partial",
		SymbolsTotal: 265, SymbolsFailed: 0, StartedAt: time.Now().Add(-time.Hour),
	}})

	job, ok := got["delisting_reconcile"]
	if !ok {
		t.Fatal("GET /scheduler/status 沒有回傳 delisting_reconcile")
	}
	if job.Status != "partial" {
		t.Fatalf("status = %q, want partial", job.Status)
	}
	// 26 小時門檻：一小時前跑過的那輪不該是 stale。
	if job.Stale {
		t.Fatal("一小時前執行過的排程不該標成 stale——門檻沒有掛上")
	}
	// symbols_failed 恆為 0，這個 job 沒有「逐檔失敗」的概念。
	if job.SymbolsTotal != 265 || job.SymbolsFailed != 0 {
		t.Fatalf("symbols_total/failed = %d/%d, want 265/0", job.SymbolsTotal, job.SymbolsFailed)
	}
}

// 未註冊（config 關閉或依賴未注入）時是 disabled 而不是 never_run ＋ stale。
func TestSchedulerStatusMarksDelistingReconcileDisabledWhenNotRegistered(t *testing.T) {
	sched := schedulerWithSREvaluation(false)
	defer sched.Stop()

	job := statusByJob(t, sched, nil)["delisting_reconcile"]
	if job.Status != "disabled" {
		t.Fatalf("status = %q, want disabled", job.Status)
	}
	if job.Stale {
		t.Fatal("未註冊的排程不該標成 stale")
	}
}

// #15：202 必須代表真的啟動了，不能只是把請求吞掉。
func TestRunDelistingReconcileAcceptsWhenIdle(t *testing.T) {
	source := &delistingSourceStub{}
	_, r := newDelistingHandler(source)

	w := postDelisting(r, "")
	if w.Code != http.StatusAccepted {
		t.Fatalf("空閒時應回 202，got %d body=%s", w.Code, w.Body.String())
	}

	var body struct {
		Message      string `json:"message"`
		AcceptShrink bool   `json:"accept_shrink"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("202 body 不是合法 JSON：%v", err)
	}
	if body.AcceptShrink {
		t.Fatal("預設不得帶 accept_shrink——那是人工核可縮水的 override")
	}

	deadline := time.Now().Add(2 * time.Second)
	for source.callCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if source.callCount() == 0 {
		t.Error("回了 202 卻沒有真的啟動對帳")
	}
}

// ⚠️ **只認字面值 `1`**：其餘一律當成沒帶（fail-closed）。
// 不放行縮水只會讓那輪 failed，而誤放行是靜默寫入一份縮水的資料。
func TestRunDelistingReconcileAcceptShrinkOnlyAcceptsLiteralOne(t *testing.T) {
	for _, tc := range []struct {
		query string
		want  bool
	}{
		{"?accept_shrink=1", true},
		{"?accept_shrink=true", false},
		{"?accept_shrink=0", false},
		{"", false},
	} {
		t.Run(tc.query, func(t *testing.T) {
			_, r := newDelistingHandler(&delistingSourceStub{})

			w := postDelisting(r, tc.query)
			if w.Code != http.StatusAccepted {
				t.Fatalf("want 202, got %d", w.Code)
			}
			var body struct {
				AcceptShrink bool `json:"accept_shrink"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.AcceptShrink != tc.want {
				t.Fatalf("accept_shrink = %v, want %v", body.AcceptShrink, tc.want)
			}
		})
	}
}

// #17b：手動與手動重疊 → 第二次同步回 409。
func TestRunDelistingReconcileConflictsWhileRoundRuns(t *testing.T) {
	source := &delistingSourceStub{block: make(chan struct{}), entered: make(chan struct{})}
	entered := source.entered
	_, r := newDelistingHandler(source)

	if w := postDelisting(r, ""); w.Code != http.StatusAccepted {
		t.Fatalf("持有輪次應回 202，got %d", w.Code)
	}
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("持有輪次沒有進到對帳流程")
	}

	// accept_shrink 的手動觸發共用同一把鎖（#17c）。
	w := postDelisting(r, "?accept_shrink=1")
	if w.Code != http.StatusConflict {
		t.Fatalf("被佔用時應回 409，got %d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("409 body 不是合法 JSON：%v", err)
	}
	if body.Error == "" {
		t.Error("409 應該帶 error 訊息")
	}
	// 被擋掉的請求不得真的跑第二輪。
	if got := source.callCount(); got != 1 {
		t.Fatalf("fetch calls = %d, want 1", got)
	}

	close(source.block)
}

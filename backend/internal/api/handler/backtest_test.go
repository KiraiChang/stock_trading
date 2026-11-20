package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/backtest"
	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
	"github.com/trading/backend/internal/store"
)

// 回測取消（I-134）：條件式更新影響 1 列才是 200；0 列時再查一次——不存在 404、存在 409；
// 任何 DB 錯誤 500（⛔ 不是 404）。

type backtestRepoStub struct {
	store.BacktestRepo // 沒覆寫的方法被呼叫就 panic

	cancelled  bool
	cancelErr  error
	getErr     error
	getCalls   int
	cancelCall int
}

func (s *backtestRepoStub) CancelPendingJob(context.Context, string) (bool, error) {
	s.cancelCall++
	return s.cancelled, s.cancelErr
}

func (s *backtestRepoStub) GetJob(context.Context, string) (*store.BacktestJob, error) {
	s.getCalls++
	if s.getErr != nil {
		return nil, s.getErr
	}
	return &store.BacktestJob{JobID: "j1", Status: "running"}, nil
}

func TestBacktestCancelStatusMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name         string
		stub         backtestRepoStub
		wantStatus   int
		wantBody     string
		wantGetCalls int
	}{
		{"pending cancelled", backtestRepoStub{cancelled: true}, http.StatusOK, "cancelled", 0},
		{"missing", backtestRepoStub{getErr: sql.ErrNoRows}, http.StatusNotFound, "job not found", 1},
		{"claimed or finished", backtestRepoStub{}, http.StatusConflict, "only pending jobs can be cancelled", 1},
		{"update error", backtestRepoStub{cancelErr: errors.New("driver: bad connection secret")}, http.StatusInternalServerError, "internal server error", 0},
		{"lookup error", backtestRepoStub{getErr: errors.New("driver: bad connection secret")}, http.StatusInternalServerError, "internal server error", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.stub
			h := NewBacktestHandler(nil, &repo, zap.NewNop())
			router := gin.New()
			router.DELETE("/backtest/:job_id", h.Cancel)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/backtest/j1", nil))

			if w.Code != tt.wantStatus {
				t.Fatalf("status=%d want %d body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tt.wantBody) {
				t.Fatalf("body=%s want %q", w.Body.String(), tt.wantBody)
			}
			if strings.Contains(w.Body.String(), "secret") {
				t.Fatalf("DB error leaked: %s", w.Body.String())
			}
			if repo.cancelCall != 1 || repo.getCalls != tt.wantGetCalls {
				t.Fatalf("calls: cancel=%d get=%d", repo.cancelCall, repo.getCalls)
			}
		})
	}
}

// backend 的回應⛔ 不帶 execution_owner：BacktestJob 的 json 鍵集合固定。
func TestBacktestJobJSONKeysExcludeExecutionOwner(t *testing.T) {
	raw, err := json.Marshal(store.BacktestJob{Error: "x"})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := []string{"chip_min_score", "created_at", "end_date", "error", "finished_at", "id", "job_id",
		"start_date", "started_at", "status", "strategy", "symbols", "timeframe", "trigger", "type",
		"use_chip_filter"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("keys=%v want %v", keys, want)
	}
}

// 用真的 SQLite repo 走一次取消與查詢：running 的 error 是 NULL，取消要回 409（⛔ 不是 404／500）、
// GET 要回 200。
func TestBacktestCancelAndGetWithRealRepo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := backtestHandlerTestDB(t)
	for _, j := range []struct{ id, status string }{{"p1", "pending"}, {"r1", "running"}} {
		if _, err := db.Exec(`INSERT INTO backtest_jobs
			(job_id, type, strategy, symbols, timeframe, start_date, end_date, status, trigger_source)
			VALUES (?, 'backtest', 'breakout_v1', '["2330"]', '1d', '2026-01-01', '2026-06-30', ?, 'manual')`,
			j.id, j.status); err != nil {
			t.Fatal(err)
		}
	}
	h := NewBacktestHandler(nil, store.NewBacktestRepo(db), zap.NewNop())
	router := gin.New()
	router.DELETE("/backtest/:job_id", h.Cancel)
	router.GET("/backtest/:job_id", h.GetJob)
	router.GET("/backtest", h.ListJobs)

	do := func(method, path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(method, path, nil))
		return w
	}
	if w := do(http.MethodGet, "/backtest/r1"); w.Code != http.StatusOK {
		t.Fatalf("GET running: %d %s", w.Code, w.Body.String())
	}
	if w := do(http.MethodGet, "/backtest"); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"total":2`) {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
	if w := do(http.MethodDelete, "/backtest/r1"); w.Code != http.StatusConflict {
		t.Fatalf("cancel running: %d %s", w.Code, w.Body.String())
	}
	if w := do(http.MethodDelete, "/backtest/p1"); w.Code != http.StatusOK {
		t.Fatalf("cancel pending: %d %s", w.Code, w.Body.String())
	}
	if w := do(http.MethodDelete, "/backtest/p1"); w.Code != http.StatusConflict {
		t.Fatalf("cancel again: %d %s", w.Code, w.Body.String())
	}
	if w := do(http.MethodDelete, "/backtest/nope"); w.Code != http.StatusNotFound {
		t.Fatalf("cancel missing: %d %s", w.Code, w.Body.String())
	}
	var status, owner string
	if err := db.QueryRow(`SELECT status, COALESCE(execution_owner, '') FROM backtest_jobs WHERE job_id='r1'`).Scan(&status, &owner); err != nil {
		t.Fatal(err)
	}
	if status != "running" {
		t.Fatalf("running job changed by cancel: %s", status)
	}
}

func backtestHandlerTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	tmp, err := os.CreateTemp("", "backtest-handler-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	t.Cleanup(func() { os.Remove(tmp.Name()) })
	db, err := store.NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: tmp.Name()})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return db
}

// ── I-137：建任務前的 400、error 只回分類過的代碼、data_coverage 是陣列或 null ──

func TestBacktestSubmitRejectsInvalidRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := backtestHandlerTestDB(t)
	repo := store.NewBacktestRepo(db)
	h := NewBacktestHandler(backtest.NewManager(repo, "", 2, zap.NewNop()), repo, zap.NewNop())
	router := gin.New()
	router.POST("/backtest", h.Submit)

	post := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/backtest", strings.NewReader(body)))
		return w
	}
	cases := []struct {
		body string
		code int
		err  string
	}{
		{`{"strategy":"breakout_v1","symbols":["2330"],"start_date":"2025-02-30","end_date":"2025-12-31"}`, 400, "invalid_date:start_date"},
		{`{"strategy":"breakout_v1","symbols":["2330"],"start_date":"2025-01-01","end_date":"9999-12-31"}`, 400, "invalid_date:end_date"},
		{`{"strategy":"breakout_v1","symbols":["2330"],"start_date":"0000-01-01","end_date":"2025-12-31"}`, 400, "invalid_date:start_date"},
		{`{"strategy":"breakout_v1","symbols":["2330"],"start_date":"2025-06-02","end_date":"2025-06-01"}`, 400, "start_after_end"},
		{`{"strategy":"breakout_swing_atr_v1","symbols":["A","B","C"],"start_date":"2025-01-01","end_date":"2025-12-31"}`, 400, "too_many_symbols:3:2"},
		{`{"strategy":"breakout_v1","symbols":["A","B"],"start_date":"2025-01-01","end_date":"2025-12-31"}`, 400, "backtrader_single_symbol:2"},
		{`{"strategy":"breakout_swing_atr_v1","symbols":["A","B"],"start_date":"2025-06-02","end_date":"2025-06-02"}`, 201, ""},
	}
	for _, c := range cases {
		w := post(c.body)
		if w.Code != c.code {
			t.Fatalf("%s：status=%d，預期 %d（%s）", c.body, w.Code, c.code, w.Body.String())
		}
		if c.err != "" {
			var resp map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp["error"] != c.err {
				t.Fatalf("%s：error=%q，預期 %q", c.body, resp["error"], c.err)
			}
		}
	}
	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM backtest_jobs`); err != nil || n != 1 {
		t.Fatalf("只有合法的那一筆會建任務：%d（%v）", n, err)
	}
}

func TestBacktestGetHidesRawErrorsAndReturnsCoverage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := backtestHandlerTestDB(t)
	raw := `(psycopg2.errors.UndefinedColumn) column "secret_col" does not exist [SQL: SELECT secret_col FROM t]`
	for _, j := range []struct{ id, status, err string }{
		{"old", "failed", raw},
		{"code", "failed", "task_too_large:2330:60001:60000"},
		{"cancel", "failed", "cancelled by user"},
		{"done", "done", ""},
		{"legacy", "done", ""},
	} {
		if _, err := db.Exec(`INSERT INTO backtest_jobs
			(job_id, type, strategy, symbols, timeframe, start_date, end_date, status, trigger_source, error)
			VALUES (?, 'backtest', 'breakout_v1', '["2330"]', '1d', '2026-01-01', '2026-06-30', ?, 'manual', ?)`,
			j.id, j.status, j.err); err != nil {
			t.Fatal(err)
		}
	}
	cov := `[{"symbol":"2330","first_bar":"2026-01-02","last_bar":"2026-06-30","bars":120,"warmup_bars":250,"warmup_required":250,"skipped":false}]`
	for _, r := range []struct {
		id  string
		cov any
	}{{"done", cov}, {"legacy", nil}} {
		if _, err := db.Exec(`INSERT INTO backtest_results (job_id, strategy, total_return, annual_return, win_rate, max_drawdown,
			sharpe_ratio, total_trades, win_trades, loss_trades, avg_pnl, data_coverage)
			VALUES (?, 'breakout_v1', 0, 0, 0, 0, 0, 0, 0, 0, 0, ?)`,
			r.id, r.cov); err != nil {
			t.Fatal(err)
		}
	}
	h := NewBacktestHandler(nil, store.NewBacktestRepo(db), zap.NewNop())
	router := gin.New()
	router.GET("/backtest/:job_id", h.GetJob)
	router.GET("/backtest", h.ListJobs)
	get := func(path string) (string, map[string]any) {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s：%d %s", path, w.Code, w.Body.String())
		}
		var m map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		return w.Body.String(), m
	}

	for path, want := range map[string]string{"/backtest/old": "internal_error", "/backtest/code": "task_too_large:2330:60001:60000",
		"/backtest/cancel": "cancelled by user"} {
		body, m := get(path)
		if got := m["job"].(map[string]any)["error"]; got != want {
			t.Errorf("%s：error=%v，預期 %q", path, got, want)
		}
		if strings.Contains(body, "secret_col") || strings.Contains(body, "SELECT") {
			t.Errorf("%s：回應帶出了原始例外：%s", path, body)
		}
	}
	listBody, _ := get("/backtest")
	if strings.Contains(listBody, "secret_col") || !strings.Contains(listBody, `"error":"internal_error"`) {
		t.Errorf("清單沒有改回 internal_error：%s", listBody)
	}

	_, m := get("/backtest/done")
	got, ok := m["result"].(map[string]any)["data_coverage"].([]any)
	if !ok || len(got) != 1 || got[0].(map[string]any)["symbol"] != "2330" || got[0].(map[string]any)["bars"] != float64(120) {
		t.Errorf("data_coverage 應該是陣列：%#v", m["result"].(map[string]any)["data_coverage"])
	}
	_, m = get("/backtest/legacy")
	if v, present := m["result"].(map[string]any)["data_coverage"]; !present || v != nil {
		t.Errorf("舊結果的 data_coverage 應該是 null：%#v（present=%v）", v, present)
	}
}

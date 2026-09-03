package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/chip"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/store"
)

// 這一組是 docs/issue.md I-104 handler 側的 **call-site** 測試。
//
// ⚠️ **只測 joberr 本身抓不到呼叫點退回 `err.Error()`。** 這裡實際執行三個
// 非同步流程，斷言寫進 job 紀錄／`failures` 的內容不含敏感標記——
// 那些欄位會被持久化並由前端原樣渲染（`SRZones.svelte` 六處、`Backfill.svelte:294`）。

// leakURL 是刻意帶「憑證樣貌」的上游位址：它會出現在 client 的錯誤訊息裡。
const leakURL = "http://svc-user:s3cr3t@python.internal:8001"

func assertJobErrNoLeak(t *testing.T, where, msg string) {
	t.Helper()
	for _, bad := range []string{"s3cr3t", "python.internal", "svc-user"} {
		if strings.Contains(msg, bad) {
			t.Errorf("%s 外洩了 %q：%q", where, bad, msg)
		}
	}
	if msg == "" {
		t.Errorf("%s 應寫入分類後的原因，得到空字串", where)
	}
}

// ── ① chip 的 failures[].error ─────────────────────────────────

type failingChipSource struct{ market.ChipDataSource }

func (failingChipSource) FetchInstitutionalTrades(context.Context, string, time.Time, time.Time) ([]market.InstitutionalTrade, error) {
	return nil, errors.New("dial " + leakURL + ": connection refused")
}

type chipJobRepoStub struct {
	store.ChipSyncJobRepo
	failures []string
}

func (c *chipJobRepoStub) UpdateProgress(_ context.Context, _ string, _, _ int, failures store.RawJSON) error {
	c.failures = append(c.failures, string(failures))
	return nil
}
func (c *chipJobRepoStub) Finish(context.Context, string, string, string) error { return nil }

// chipCandleStub 讓 targetDatePublished 走得下去——回空集合代表「那天不是交易日」，
// 於是不會誤判成「資料尚未發布」，測試專注在 fetch 失敗那條路徑。
type chipCandleStub struct{ store.CandleRepo }

func (chipCandleStub) GetRange(context.Context, string, string, time.Time, time.Time) ([]store.Candle, error) {
	return nil, nil
}

func TestChipSyncFailuresDoNotLeakCause(t *testing.T) {
	jobs := &chipJobRepoStub{}
	syncer := chip.NewSyncer(failingChipSource{}, nil, nil, nil, nil, chipCandleStub{}, zap.NewNop())
	h := NewChipHandler(nil, nil, nil, nil, nil, jobs, syncer, 20, zap.NewNop())

	h.runSync("chip_job_1", []string{"2330"},
		time.Now().AddDate(0, 0, -1), time.Now(), []string{"institutional"})

	if len(jobs.failures) == 0 {
		t.Fatal("應寫入 failures")
	}
	last := jobs.failures[len(jobs.failures)-1]
	assertJobErrNoLeak(t, "chip failures", last)
	// failures[].error 是**裸 reason code**（symbol 已在同一列的 symbol 欄）。
	if !strings.Contains(last, `"error":"conn_refused"`) {
		t.Errorf("failures 應含分類後的 reason code，得到 %s", last)
	}
}

// ── ② sr_evaluation 的 MarkFailed ──────────────────────────────

type evalJobRepoStub struct {
	store.SREvaluationJobRepo
	failedMsg string
}

func (e *evalJobRepoStub) MarkRunning(context.Context, string) error { return nil }
func (e *evalJobRepoStub) MarkFailed(_ context.Context, _ string, msg string) error {
	e.failedMsg = msg
	return nil
}

func TestSREvaluationMarkFailedDoesNotLeakCause(t *testing.T) {
	// 上游回 500，client 的錯誤會帶上請求位址。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal failure at " + leakURL))
	}))
	defer srv.Close()

	jobs := &evalJobRepoStub{}
	h := NewSRRegressionResultHandler(analysis.NewClient(srv.URL), nil, jobs, nil, nil, zap.NewNop())

	h.runEvaluationJob("eval_job_1", analysis.SREvaluationRequest{Symbols: []string{"2330"}})

	assertJobErrNoLeak(t, "sr_evaluation MarkFailed", jobs.failedMsg)
	if !strings.HasPrefix(jobs.failedMsg, "sr_evaluation:") {
		t.Errorf("應為 stage:reason 形式，得到 %q", jobs.failedMsg)
	}
}

// ── ③ sr_train 的 MarkFailed ───────────────────────────────────

type trainJobRepoStub struct {
	store.SRScoringTrainJobRepo
	failedMsg string
}

func (t *trainJobRepoStub) MarkRunning(context.Context, string) error { return nil }
func (t *trainJobRepoStub) MarkFailed(_ context.Context, _ string, msg string) error {
	t.failedMsg = msg
	return nil
}

func TestSRTrainMarkFailedDoesNotLeakCause(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("train failed at " + leakURL))
	}))
	defer srv.Close()

	jobs := &trainJobRepoStub{}
	h := NewSRZoneHandler(analysis.NewClient(srv.URL), nil, nil, jobs, nil, nil, zap.NewNop())

	h.runTrainJob("train_job_1", []string{"2330"}, "1d", 250, "", "", "")

	assertJobErrNoLeak(t, "sr_train MarkFailed", jobs.failedMsg)
	if !strings.HasPrefix(jobs.failedMsg, "sr_train:") {
		t.Errorf("應為 stage:reason 形式，得到 %q", jobs.failedMsg)
	}
}

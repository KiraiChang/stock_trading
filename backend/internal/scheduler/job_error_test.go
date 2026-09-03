package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/trading/backend/internal/indicator"
	"github.com/trading/backend/internal/joberr"
)

// 這一組守的是 docs/architecture.md「寫入失敗的一致性契約」 的兩件事：
// ① job_runs.error 只能出現封閉值域的 reason code；
// ② symbols_failed 是聯集不是相加。

const jobSensitiveMarker = "postgres://trading_user:s3cr3t@db.internal:5432/trading"

func TestSafeJobErrorReasonMapsKnownCauses(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want joberr.Reason
	}{
		{"資料不足走 typed error", fmt.Errorf("%w: got 12", indicator.ErrInsufficientCandles), joberr.InsufficientData},
		{"型別溢位", errors.New("pq: numeric field overflow"), joberr.NumericOverflow},
		{"唯一鍵衝突", errors.New(`pq: duplicate key value violates unique constraint "x"`), joberr.ConstraintViolation},
		{"連不上", errors.New("dial tcp: connection refused"), joberr.ConnRefused},
		{"逾時 sentinel", context.DeadlineExceeded, joberr.Timeout},
		{"READONLY", errors.New("READONLY You can't write against a read only replica"), joberr.Readonly},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := safeJobErrorReason(tt.err); got != tt.want {
				t.Errorf("safeJobErrorReason = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestSafeJobErrorReasonFallsBackToInternalError 是安全邊界最關鍵的一條。
//
// ⛔ 分類不出來時**不得退回 err.Error()**——job_runs.error 會被
// GET /scheduler/status 回傳、前端原樣渲染，而且保留 30 天。
func TestSafeJobErrorReasonFallsBackToInternalError(t *testing.T) {
	err := errors.New("totally unrecognised failure at " + jobSensitiveMarker)
	if got := safeJobErrorReason(err); got != joberr.Internal {
		t.Fatalf("未知錯誤要回 internal_error，得到 %q", got)
	}
	if strings.Contains(string(safeJobErrorReason(err)), jobSensitiveMarker) {
		t.Error("reason code 不得包含原始錯誤文字")
	}
}

func TestJobFailureTallySummaryNeverLeaksCause(t *testing.T) {
	tally := newJobFailureTally()
	tally.addFetchFailure("2330", errors.New("dial "+jobSensitiveMarker+": connection refused"))
	tally.addEvaluateFailure("2454", fmt.Errorf("%w: pq: numeric field overflow (%s)",
		indicator.ErrPersistence, jobSensitiveMarker))
	tally.addEvaluateFailure("6182", fmt.Errorf("%w: got 3", indicator.ErrInsufficientCandles))

	got := tally.summary()

	if strings.Contains(got, jobSensitiveMarker) || strings.Contains(got, "s3cr3t") {
		t.Fatalf("摘要外洩了 cause：%s", got)
	}
	for _, want := range []string{
		"fetch_failed:1", "2330:conn_refused",
		"evaluate_failed:2", "2454:numeric_overflow", "6182:insufficient_data",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("摘要缺少 %q，實際 = %s", want, got)
		}
	}
}

// TestJobFailureTallyUsesUnionNotSum 守住 job_runs 只有一個 symbols_failed 欄位
// 這個限制：同一檔既抓取失敗又評估失敗時，相加會重複計數，
// 甚至讓 failed >= total 成立而把整輪誤判成 failed。
func TestJobFailureTallyUsesUnionNotSum(t *testing.T) {
	tally := newJobFailureTally()
	tally.addFetchFailure("2330", errors.New("connection refused"))
	tally.addEvaluateFailure("2330", errors.New("connection refused")) // 同一檔
	tally.addEvaluateFailure("2454", errors.New("pq: numeric field overflow"))

	if got := tally.failedCount(); got != 2 {
		t.Errorf("聯集應為 2（2330 只算一次），得到 %d", got)
	}
}

// TestJobFailureTallyOpaqueFetchDoesNotDoubleCount 涵蓋只知筆數的路徑
// （runPreMarket 的 BackfillHistory、runIntradayBatch 的整批失敗）。
func TestJobFailureTallyOpaqueFetchDoesNotDoubleCount(t *testing.T) {
	t.Run("opaque 較大時取 opaque", func(t *testing.T) {
		tally := newJobFailureTally()
		tally.addOpaqueFetchFailures(5)
		tally.addEvaluateFailure("2454", errors.New("boom"))
		if got := tally.failedCount(); got != 5 {
			t.Errorf("得到 %d, want 5", got)
		}
	})
	t.Run("聯集較大時取聯集", func(t *testing.T) {
		tally := newJobFailureTally()
		tally.addOpaqueFetchFailures(1)
		tally.addEvaluateFailure("2454", errors.New("boom"))
		tally.addEvaluateFailure("2330", errors.New("boom"))
		if got := tally.failedCount(); got != 2 {
			t.Errorf("得到 %d, want 2", got)
		}
	})
	t.Run("opaque 沒有逐檔資訊時摘要只記數量", func(t *testing.T) {
		tally := newJobFailureTally()
		tally.addOpaqueFetchFailures(3)
		got := tally.summary()
		if got != "fetch_failed:3" {
			t.Errorf("得到 %q, want %q", got, "fetch_failed:3")
		}
	})
}

func TestJobFailureTallyEmptySummaryIsBlank(t *testing.T) {
	if got := newJobFailureTally().summary(); got != "" {
		t.Errorf("沒有失敗時摘要要是空字串，得到 %q", got)
	}
}

// TestJobFailureTallySummaryIsDeterministic 讓同一組失敗每次產生相同字串——
// 測試與人眼比對都需要。
func TestJobFailureTallySummaryIsDeterministic(t *testing.T) {
	build := func() string {
		tally := newJobFailureTally()
		for _, sym := range []string{"6182", "2330", "2454"} {
			tally.addEvaluateFailure(sym, errors.New("pq: numeric field overflow"))
		}
		return tally.summary()
	}
	first := build()
	for i := 0; i < 20; i++ {
		if got := build(); got != first {
			t.Fatalf("摘要不穩定：%q vs %q", got, first)
		}
	}
	if !strings.Contains(first, "2330:numeric_overflow, 2454:numeric_overflow, 6182:numeric_overflow") {
		t.Errorf("明細應以 symbol 排序，得到 %s", first)
	}
}

// ── 5-6：三種 degraded stage 都進摘要，但不增加 symbols_failed ────

func TestDegradedStagesEnterSummaryButNotSymbolsFailed(t *testing.T) {
	tally := newJobFailureTally()
	tally.addEvaluateFailure("2330", errors.New("pq: numeric field overflow")) // 硬失敗
	tally.addDegraded("2454", map[string]error{
		"signal_persist_failed": errors.New("pq: numeric field overflow (" + jobSensitiveMarker + ")"),
	})
	tally.addDegraded("6182", map[string]error{
		"signal_persist_failed": errors.New("pq: numeric field overflow"),
		"queue_failed":          errors.New("READONLY replica"),
	})
	tally.addDegraded("0050", map[string]error{"dedup_degraded": errors.New("dial: connection refused")})

	if got := tally.failedCount(); got != 1 {
		t.Errorf("degraded 不得計入 symbols_failed，得到 %d, want 1", got)
	}
	if !tally.hasDegraded() {
		t.Error("hasDegraded 應為 true，該輪要收成 partial")
	}

	got := tally.summary()
	for _, want := range []string{
		"evaluate_failed:1",
		"degraded:3",
		// **每個 stage 要附代表 reason code**——只有數量的話，看到 degraded 也不知道
		// 是型別溢位還是 Redis 唯讀，摘要就失去診斷價值。
		"dedup_degraded:1/conn_refused",
		"queue_failed:1/readonly",
		"signal_persist_failed:2/numeric_overflow",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("摘要缺少 %q，實際 = %s", want, got)
		}
	}
	// stage_errors 的 cause 同樣要過分類器，不得原文外洩。
	if strings.Contains(got, jobSensitiveMarker) {
		t.Errorf("degraded 的 cause 外洩了：%s", got)
	}
}

func TestDegradedOnlyRunIsPartialWithZeroFailed(t *testing.T) {
	tally := newJobFailureTally()
	tally.addDegraded("2454", map[string]error{"signal_persist_failed": errors.New("pq: numeric field overflow")})

	if got := tally.failedCount(); got != 0 {
		t.Errorf("只有降級時 symbols_failed 應為 0，得到 %d", got)
	}
	if !tally.hasDegraded() {
		t.Error("只有降級也要讓該輪收成 partial")
	}
	if got := tally.summary(); got != "degraded:1 (signal_persist_failed:1/numeric_overflow)" {
		t.Errorf("摘要 = %q", got)
	}
}

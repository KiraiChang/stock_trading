package scheduler

import (
	"net"
	"net/url"
	"os"
	"syscall"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// 層二的 log 要追溯得到「救回的是哪一類錯誤、是哪一輪」（觀察期的追加修正，
// I-123～I-126 計畫書「十二」）：重試成功時，第一次的錯誤⛔ 不會再出現在任何地方。

func unreachableDialErr() error {
	return &url.Error{Op: "Get", URL: "https://tw.stock.yahoo.com/x", Err: &net.OpError{
		Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ENETUNREACH),
	}}
}

func observeIntradayRetry(t *testing.T, errs []error) *observer.ObservedLogs {
	t.Helper()
	src := &retryBatchSource{batch: 2, errs: errs}
	s, jobRuns, _ := newIntradayRetryScheduler(t, src, &retryCandleRepo{}, retryOn, intradayFourSymbols)
	core, logs := observer.New(zapcore.InfoLevel)
	s.log = zap.New(core)
	jobRuns.nextID = 42
	s.runIntradayBatch()
	return logs
}

func onlyEntry(t *testing.T, logs *observer.ObservedLogs, msg string) map[string]any {
	t.Helper()
	entries := logs.FilterMessage(msg).All()
	if len(entries) != 1 {
		t.Fatalf("%q 應有 1 筆，得到 %d 筆", msg, len(entries))
	}
	return entries[0].ContextMap()
}

func assertFields(t *testing.T, msg string, got map[string]any, want map[string]any) {
	t.Helper()
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%q 的 %s = %#v，期望 %#v（全部欄位 %v）", msg, k, got[k], v, got)
		}
	}
}

// 重試一次就成功：成功那筆要帶 run_id 與第一次失敗的原因碼。
func TestIntradayRetrySuccessLogsFirstCause(t *testing.T) {
	logs := observeIntradayRetry(t, []error{transientDialErr()})

	got := onlyEntry(t, logs, "intraday batch fetch succeeded after retry")
	assertFields(t, "succeeded", got, map[string]any{
		"run_id": uint64(42), "first_cause": "dns_timeout", "retries": int64(1), "from": int64(0), "size": int64(2),
	})
	if _, ok := got["error"]; ok {
		t.Fatalf("成功那筆⛔ 不應帶 error 原文：%v", got)
	}
}

// 第一次 DNS 逾時、第二次 IPv6 不可達、第三次成功：每次重試的原因記在 cause，first_cause ⛔ 不被覆蓋。
func TestIntradayRetryKeepsFirstCauseAcrossAttempts(t *testing.T) {
	logs := observeIntradayRetry(t, []error{transientDialErr(), unreachableDialErr()})

	assertFields(t, "retry failed", onlyEntry(t, logs, "intraday batch retry failed"), map[string]any{
		"run_id": uint64(42), "first_cause": "dns_timeout", "cause": "net_unreachable", "retries": int64(1),
	})
	assertFields(t, "succeeded", onlyEntry(t, logs, "intraday batch fetch succeeded after retry"), map[string]any{
		"run_id": uint64(42), "first_cause": "dns_timeout", "retries": int64(2),
	})
}

// 兩次重試都失敗：最後那筆 `intraday batch fetch failed` 的 cause 是第一次失敗的原因碼。
func TestIntradayRetryGiveUpLogsCause(t *testing.T) {
	logs := observeIntradayRetry(t, []error{unreachableDialErr(), transientDialErr(), transientDialErr()})

	if n := logs.FilterMessage("intraday batch retry failed").Len(); n != 2 {
		t.Fatalf("retry failed 應有 2 筆，得到 %d", n)
	}
	assertFields(t, "fetch failed", onlyEntry(t, logs, "intraday batch fetch failed"), map[string]any{
		"run_id": uint64(42), "cause": "net_unreachable", "from": int64(0), "size": int64(2),
	})
	if n := logs.FilterMessage("intraday batch fetch succeeded after retry").Len(); n != 0 {
		t.Fatalf("⛔ 不應有成功那筆，得到 %d", n)
	}
}

// 永久錯誤⛔ 不重試，但 fetch failed 那筆同樣要帶 run_id 與原因碼。
func TestIntradayPermanentFailureLogsCause(t *testing.T) {
	logs := observeIntradayRetry(t, []error{errRetryPermanent})

	assertFields(t, "fetch failed", onlyEntry(t, logs, "intraday batch fetch failed"), map[string]any{
		"run_id": uint64(42), "cause": "not_transient",
	})
	if n := logs.FilterMessage("intraday batch retry failed").Len(); n != 0 {
		t.Fatalf("永久錯誤⛔ 不應重試，得到 %d 筆 retry failed", n)
	}
}

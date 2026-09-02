package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/trading/backend/internal/indicator"
)

// reasonCode 是**可以寫進 job_runs.error 的封閉值域**。
//
// ⚠️ **job_runs.error 是使用者可見面**：GET /scheduler/status 會回傳它，
// 前端 Scheduler.svelte 直接 `{job.error}` 原樣渲染。原始 driver 錯誤常帶 DSN、
// 主機位址或 SQL 片段——寫進去就等於顯示在畫面上，而且 job_runs 保留 30 天，
// 之後每次查詢都會再洩一次。詳見 docs/architecture.md「寫入失敗的一致性契約」。
type reasonCode string

const (
	reasonNumericOverflow      reasonCode = "numeric_overflow"
	reasonConstraintViolation  reasonCode = "constraint_violation"
	reasonConnRefused          reasonCode = "conn_refused"
	reasonTimeout              reasonCode = "timeout"
	reasonReadonly             reasonCode = "readonly"
	reasonSerializationFailure reasonCode = "serialization_failure"
	reasonInsufficientData     reasonCode = "insufficient_data"
	// reasonInternal 是**唯一的 fallback**。
	//
	// ⛔ 分類不出來時**一律回它，不得退回 err.Error()**——那正是最危險的實作空間：
	// 一個沒被涵蓋的錯誤型別就足以讓敏感資訊寫進 job_runs.error。
	// 想知道細節去看 log，那是原始 error 唯一被允許出現的地方。
	reasonInternal reasonCode = "internal_error"
)

// safeJobErrorReason 是**唯一**能決定寫進 job_runs.error 的原因字串的地方。
//
// ⛔ 其他地方不得自行拼接，也不得用 fmt.Sprintf("%v", err) / err.Error() 組摘要。
func safeJobErrorReason(err error) reasonCode {
	if err == nil {
		return reasonInternal
	}
	switch {
	case errors.Is(err, indicator.ErrInsufficientCandles):
		return reasonInsufficientData
	case errors.Is(err, context.DeadlineExceeded):
		return reasonTimeout
	}
	// 這一段是「沒有 typed error 可用」時的退路：driver 不提供可比對的 sentinel，
	// 只能看訊息。**比對用的是錯誤類別關鍵字，輸出的仍是封閉值域的 code**，
	// 原文不會外流。
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "numeric field overflow"), strings.Contains(msg, "out of range"):
		return reasonNumericOverflow
	case strings.Contains(msg, "violates") && strings.Contains(msg, "constraint"),
		strings.Contains(msg, "unique constraint"), strings.Contains(msg, "duplicate key"):
		return reasonConstraintViolation
	case strings.Contains(msg, "connection refused"), strings.Contains(msg, "no such host"),
		strings.Contains(msg, "connection reset"):
		return reasonConnRefused
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline exceeded"),
		strings.Contains(msg, "context canceled"):
		return reasonTimeout
	case strings.Contains(msg, "readonly"), strings.Contains(msg, "read only"):
		return reasonReadonly
	case strings.Contains(msg, "serialization failure"), strings.Contains(msg, "could not serialize"):
		return reasonSerializationFailure
	}
	return reasonInternal
}

// safeJobErrorSummary 是**整輪早退**時寫進 job_runs.error 的安全字串。
//
// ⛔ 早退路徑（watchlist 讀不到、token 等級不足…）原本直接寫 err.Error()，
// 那是分類器的旁路——同一個外洩風險換個入口而已。stage 只用固定字面值，
// 原因一律過 safeJobErrorReason。
func safeJobErrorSummary(stage string, err error) string {
	return fmt.Sprintf("%s:%s", stage, safeJobErrorReason(err))
}

// jobFailureTally 累計一輪排程的失敗，並產出 job_runs 需要的兩個值。
//
// **為什麼要兩個集合而不是兩個計數器**：job_runs 只有一個 symbols_failed 欄位，
// 而同一檔可能同時抓取失敗與評估失敗。相加會重複計數，甚至讓 failed >= total 成立
// 而把整輪誤判成 failed。**要的是聯集。**
type jobFailureTally struct {
	fetchFailed    map[string]reasonCode
	evaluateFailed map[string]reasonCode

	// degraded 是**評估成功但有降級**的 symbol，值是該檔各 stage 的**安全 reason code**。
	//
	// ⛔ **一律不進 symbols_failed**——那些 symbol 的評估其實成功了，
	// 混進去會讓 failed 同時代表兩件事。改由 finishRunDegraded 讓該輪收成 partial。
	//
	// ⚠️ **存的是 reason code 不是原始 error**：三個來源（fetchFailed / evaluateFailed /
	// stage_errors）都要走同一個分類器，原始 cause 只進 log。
	degraded map[string]map[string]reasonCode

	// fetchFailedOpaque 是**只知道筆數、不知道是哪幾檔**的抓取失敗。
	//
	// 兩條路徑給不出 symbol：runPreMarket 的 BackfillHistory 只回一個 int，
	// runIntradayBatch 的 FetchAndStoreIntradayBatch 逐檔失敗只記 log
	// （那個盲區另立 issue.md I-103 追蹤，不在本筆範圍）。
	fetchFailedOpaque int
}

func newJobFailureTally() *jobFailureTally {
	return &jobFailureTally{
		fetchFailed:    map[string]reasonCode{},
		evaluateFailed: map[string]reasonCode{},
		degraded:       map[string]map[string]reasonCode{},
	}
}

// addOpaqueFetchFailures 記「知道有 n 檔抓取失敗，但不知道是哪幾檔」。
func (t *jobFailureTally) addOpaqueFetchFailures(n int) {
	if n > 0 {
		t.fetchFailedOpaque += n
	}
}

// addFetchFailure 記行情抓取／回補失敗。
func (t *jobFailureTally) addFetchFailure(symbol string, err error) {
	t.fetchFailed[symbol] = safeJobErrorReason(err)
}

// addEvaluateFailure 記 Evaluate 的硬失敗。
//
// ⚠️ **命名是 evaluate 不是 persist**：Evaluate 的硬失敗至少有四種——持久化失敗、
// K 棒不足、DB 讀取失敗、其他。叫 persist_failed 會把「資料不足」標成「寫入失敗」，
// 摘要直接說謊。（signal 的 degraded stage 才叫 signal_persist_failed，那是第 2 段。）
func (t *jobFailureTally) addEvaluateFailure(symbol string, err error) {
	t.evaluateFailed[symbol] = safeJobErrorReason(err)
}

// addDegraded 記一檔的降級：stage 名稱 → 原始 cause，內部轉成安全 reason code。
//
// cause 可以是 nil（例如某些降級沒有底層 error），此時 reason code 是 internal_error。
func (t *jobFailureTally) addDegraded(symbol string, stageErrs map[string]error) {
	if len(stageErrs) == 0 {
		return
	}
	reasons := make(map[string]reasonCode, len(stageErrs))
	for stage, cause := range stageErrs {
		reasons[stage] = safeJobErrorReason(cause)
	}
	t.degraded[symbol] = reasons
}

// hasDegraded 供 finishRunDegraded 判斷該輪要不要收成 partial。
func (t *jobFailureTally) hasDegraded() bool { return len(t.degraded) > 0 }

// failedCount 回傳 |fetchFailed ∪ evaluateFailed|，並與 opaque 計數取大者。
//
// ⚠️ **為什麼是聯集不是相加**：同一檔可能既抓取失敗又評估失敗。相加會重複計數，
// 也可能讓 failed >= total 成立而把整輪誤判成 failed。
//
// ⚠️ **為什麼 opaque 是取 max 而不是加上去**：那條路徑不知道是哪幾檔，
// 與 evaluateFailed 的重疊算不出來。相加會重複計數，所以取兩者的大者當**下界**。
// 這是刻意的低估——真正的修法是讓那些路徑回報逐檔失敗（見 issue.md I-103）。
// 判定 partial/failed 不受影響：只要任一邊非零就已經是 partial。
func (t *jobFailureTally) failedCount() int {
	union := make(map[string]struct{}, len(t.fetchFailed)+len(t.evaluateFailed))
	for sym := range t.fetchFailed {
		union[sym] = struct{}{}
	}
	for sym := range t.evaluateFailed {
		union[sym] = struct{}{}
	}
	if t.fetchFailedOpaque > len(union) {
		return t.fetchFailedOpaque
	}
	return len(union)
}

// summary 產生 job_runs.error 的內容。
//
// 格式：`fetch_failed:3; evaluate_failed:2 (2454:numeric_overflow, 6182:insufficient_data)`
//
// **只含 stage、symbol 與封閉值域的 reason code**，不含任何原始錯誤文字。
// 明細以 symbol 排序，讓同一組失敗每次產生相同字串（測試與人眼比對都需要）。
func (t *jobFailureTally) summary() string {
	parts := make([]string, 0, 2)
	if n := len(t.fetchFailed); n > 0 {
		parts = append(parts, fmt.Sprintf("fetch_failed:%d (%s)", n, formatReasons(t.fetchFailed)))
	} else if t.fetchFailedOpaque > 0 {
		// 沒有逐檔資訊時只記數量，不假裝知道是哪幾檔。
		parts = append(parts, fmt.Sprintf("fetch_failed:%d", t.fetchFailedOpaque))
	}
	if n := len(t.evaluateFailed); n > 0 {
		parts = append(parts, fmt.Sprintf("evaluate_failed:%d (%s)", n, formatReasons(t.evaluateFailed)))
	}
	if n := len(t.degraded); n > 0 {
		parts = append(parts, fmt.Sprintf("degraded:%d (%s)", n, formatStageCounts(t.degraded)))
	}
	return strings.Join(parts, "; ")
}

// summaryDetailCap 限制摘要裡列出的明細筆數——job_runs.error 是 text，但整池失敗時
// 135 檔全列出來對判讀沒有幫助，也會撐爆前端那一行。數量本身已在前綴裡。
const summaryDetailCap = 10

// formatStageCounts 產出 `signal_persist_failed:2/numeric_overflow, queue_failed:1/readonly`。
//
// **同一檔可能同時帶多個 stage**，所以計的是 stage 出現次數而不是 symbol 數；
// symbol 總數已在前綴（degraded:N）裡。每個 stage 附一個**代表 reason code**
// （出現最多的那個；同票時取字典序小的，讓輸出穩定）。
func formatStageCounts(m map[string]map[string]reasonCode) string {
	counts := map[string]int{}
	reasonHits := map[string]map[reasonCode]int{}
	for _, stages := range m {
		for st, rc := range stages {
			counts[st]++
			if reasonHits[st] == nil {
				reasonHits[st] = map[reasonCode]int{}
			}
			reasonHits[st][rc]++
		}
	}
	names := make([]string, 0, len(counts))
	for st := range counts {
		names = append(names, st)
	}
	sort.Strings(names)
	items := make([]string, 0, len(names))
	for _, st := range names {
		items = append(items, fmt.Sprintf("%s:%d/%s", st, counts[st], dominantReason(reasonHits[st])))
	}
	return strings.Join(items, ", ")
}

// dominantReason 取出現最多的 reason code；同票取字典序小的以求穩定輸出。
func dominantReason(hits map[reasonCode]int) reasonCode {
	best := reasonInternal
	bestN := -1
	codes := make([]string, 0, len(hits))
	for rc := range hits {
		codes = append(codes, string(rc))
	}
	sort.Strings(codes)
	for _, c := range codes {
		if n := hits[reasonCode(c)]; n > bestN {
			best, bestN = reasonCode(c), n
		}
	}
	return best
}

func formatReasons(m map[string]reasonCode) string {
	syms := make([]string, 0, len(m))
	for sym := range m {
		syms = append(syms, sym)
	}
	sort.Strings(syms)

	truncated := false
	if len(syms) > summaryDetailCap {
		syms = syms[:summaryDetailCap]
		truncated = true
	}
	items := make([]string, 0, len(syms)+1)
	for _, sym := range syms {
		items = append(items, fmt.Sprintf("%s:%s", sym, m[sym]))
	}
	if truncated {
		items = append(items, "...")
	}
	return strings.Join(items, ", ")
}

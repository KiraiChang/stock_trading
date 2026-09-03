// Package joberr 決定「可以寫進使用者可見的 job 錯誤欄位」的字串。
//
// ⚠️ **為什麼要有這個套件**：`job_runs.error` 與各種 job 紀錄的 error 欄位
// **都是使用者可見面**——
//
//   - `job_runs.error` → `GET /scheduler/status` → 前端 `Scheduler.svelte` 原樣渲染
//   - `sr_evaluation_jobs` / 訓練 job 的 error → 前端 `SRZones.svelte` 六處原樣渲染
//
// 而原始 driver 錯誤常帶 DSN、主機位址、連線字串或 SQL 片段，寫進去就等於顯示在畫面上；
// `job_runs` 還保留 30 天，之後每次查詢都再洩一次。
//
// **為什麼獨立成套件而不是放 scheduler 或 store**（原記於 issue.md I-104 的裁決）：
// 它與「job 紀錄的錯誤欄位」綁定，不屬於 store 的資料存取職責；
// 而 handler 也要用，讓 handler 依賴 scheduler 是錯的方向。
package joberr

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/trading/backend/internal/indicator"
)

// Reason 是**唯一**可以寫進使用者可見錯誤欄位的值域。
type Reason string

const (
	NumericOverflow      Reason = "numeric_overflow"
	ConstraintViolation  Reason = "constraint_violation"
	ConnRefused          Reason = "conn_refused"
	Timeout              Reason = "timeout"
	Readonly             Reason = "readonly"
	SerializationFailure Reason = "serialization_failure"
	InsufficientData     Reason = "insufficient_data"
	NotFound             Reason = "not_found"
	Upstream             Reason = "upstream_error"
	// Internal 是**唯一的 fallback**。
	//
	// ⛔ 分類不出來時**一律回它，不得退回 err.Error()**——那是最危險的實作空間：
	// 一個沒被涵蓋的錯誤型別就足以讓敏感資訊寫進去。
	// 想知道細節去看 log，那是原始 error 唯一被允許出現的地方。
	Internal Reason = "internal_error"
)

// Classify 是**唯一**能決定寫進使用者可見錯誤欄位的原因字串的地方。
//
// ⛔ 其他地方不得自行拼接，也不得用 fmt.Sprintf("%v", err) / err.Error() 組摘要。
func Classify(err error) Reason {
	if err == nil {
		return Internal
	}
	switch {
	case errors.Is(err, indicator.ErrInsufficientCandles):
		return InsufficientData
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return Timeout
	}
	// 這一段是「沒有 typed error 可用」時的退路：driver 不提供可比對的 sentinel，
	// 只能看訊息。**比對用的是錯誤類別關鍵字，輸出的仍是封閉值域的 Reason**，
	// 原文不會外流。
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "numeric field overflow"), strings.Contains(msg, "out of range"),
		strings.Contains(msg, "value too long"):
		return NumericOverflow
	case strings.Contains(msg, "violates") && strings.Contains(msg, "constraint"),
		strings.Contains(msg, "unique constraint"), strings.Contains(msg, "duplicate key"):
		return ConstraintViolation
	case strings.Contains(msg, "connection refused"), strings.Contains(msg, "no such host"),
		strings.Contains(msg, "connection reset"), strings.Contains(msg, "broken pipe"):
		return ConnRefused
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline exceeded"),
		strings.Contains(msg, "context canceled"):
		return Timeout
	case strings.Contains(msg, "readonly"), strings.Contains(msg, "read only"):
		return Readonly
	case strings.Contains(msg, "serialization failure"), strings.Contains(msg, "could not serialize"):
		return SerializationFailure
	case strings.Contains(msg, "no rows"), strings.Contains(msg, "not found"):
		return NotFound
	case strings.Contains(msg, "status 4"), strings.Contains(msg, "status 5"),
		strings.Contains(msg, "unexpected status"), strings.Contains(msg, "decode"):
		return Upstream
	}
	return Internal
}

// SafeMessenger 讓**自己產生、訊息本身已確認安全**的錯誤原文通過，不被壓成 Reason。
//
// ⚠️ **為什麼需要它**：分類器是為了擋住**外來**錯誤（driver、上游端點）——那些可能帶
// DSN、主機位址、SQL 片段。但我們自己組的訊息（例如「對照源陳舊: source_as_of=…
// 落後 N 個交易日」）只含日期與數字，把它壓成 `internal_error` 是**資訊淨損失、
// 零安全收益**。
//
// ⛔ **只有滿足這兩個條件才可以實作它**：訊息由本專案的程式碼完整組出、
// 且不含任何來自外部系統的字串（error 原文、回應本文、URL…）。
// 把外來 error 的 `%v` 包進訊息裡就不算。
type SafeMessenger interface {
	SafeJobMessage() string
}

// Describe 回傳可安全寫入使用者可見欄位的描述。
//
// 實作 SafeMessenger 的錯誤回傳它自己的訊息；其餘一律過 Classify。
func Describe(err error) string {
	if err == nil {
		return string(Internal)
	}
	var sm SafeMessenger
	if errors.As(err, &sm) {
		return sm.SafeJobMessage()
	}
	return string(Classify(err))
}

// Summary 是**整輪早退**時寫進錯誤欄位的安全字串，格式 `<stage>:<reason>`。
//
// stage 只用固定字面值（呼叫端寫死），原因一律過 Classify。
func Summary(stage string, err error) string {
	return fmt.Sprintf("%s:%s", stage, Classify(err))
}

// SummaryFor 用於「某一檔標的失敗」的場合，格式 `<stage>:<symbol>:<reason>`。
func SummaryFor(stage, symbol string, err error) string {
	return fmt.Sprintf("%s:%s:%s", stage, symbol, Classify(err))
}

package indicator

import (
	"errors"
	"fmt"
)

// 指標計算失敗分成三種，**呼叫端要分得開**（見 docs/architecture.md「寫入失敗的一致性契約」）：
//
//   - ErrInsufficientCandles：查詢成功，但根數不足以計算指標。是「輸入不夠」，
//     不是故障——API 回 422、排程摘要記 insufficient_data。
//   - ErrPersistence：算出來了但寫不進 DB。**這是本筆的核心**：以前只記 warn 就
//     繼續往下寫 Redis／產訊號，導致 DB 與快取靜默不一致（2026-09-01 的 2454）。
//   - 其餘（例如 CandleRepo 讀取失敗）：包住原 cause 直接回傳，呼叫端一律當
//     未知錯誤處理，回通用 5xx。
//
// ⚠️ **三者都要能用 errors.Is 判斷**，handler 與 scheduler 都靠它分流；
// 不要改回字串比對。
var (
	// ErrInsufficientCandles 表示 K 棒數量不足以計算指標。
	//
	// ⚠️ **它是 sentinel，不包 cause**。原本的寫法是
	// `if err != nil || len(candles) < minCandles { ... %w, err }`，
	// 在 err == nil 時會把 nil 包進去——errors.Is/Unwrap 什麼都拿不到，
	// 訊息還會印出 %!w(<nil>)。讀取失敗與資料不足現在是兩條路。
	ErrInsufficientCandles = errors.New("insufficient candles")

	// ErrPersistence 表示指標算好了但沒能落盤。
	//
	// **落盤是成功的必要條件**：帶著這個錯誤時 snapshot 一定是 nil，
	// 呼叫端不得再拿它寫 Redis 或產生訊號。
	ErrPersistence = errors.New("indicator persistence failed")
)

// insufficientCandlesError 帶上 symbol 與實際根數，方便診斷；
// errors.Is(err, ErrInsufficientCandles) 仍然成立。
func insufficientCandlesError(symbol, timeframe string, got int) error {
	return fmt.Errorf("%w for %s/%s: got %d, need %d",
		ErrInsufficientCandles, symbol, timeframe, got, minCandles)
}

// persistenceError 包住原始 cause——cause 只供內部 log 與錯誤分類使用。
//
// ⛔ **不要把它直接回給 API 呼叫端**：driver 錯誤常帶 DSN、主機位址與 SQL 片段
// （handler/errors.go 的 serverError 有同一條說明）。
func persistenceError(symbol, timeframe string, cause error) error {
	return fmt.Errorf("%w for %s/%s: %w", ErrPersistence, symbol, timeframe, cause)
}

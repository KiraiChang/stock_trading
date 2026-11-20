package market

import "time"

// IntradayDataStatus 是盤中抓取「這一輪、這一檔」的資料結果（T-088＋I-103）。
//
// **「沒有資料」與「失敗」分開**（比照 candle_gap_detection 的三種結論）：開盤第一輪、停牌、
// 當下無成交都會讓回應裡沒有這檔，那⛔ 不是失敗；寫入失敗與整批抓取失敗才是。
// 值域封閉，寫進 intraday_symbol_runs.data_status（現況見 docs/yahoo-intraday-integration.md）。
type IntradayDataStatus string

const (
	// IntradayWritten 回應裡有這檔的 K 棒，而且寫入成功。
	IntradayWritten IntradayDataStatus = "written"
	// IntradayNoData 回應裡沒有這檔（或全是 null 棒、或 key 對不上）。⛔ 不是失敗。
	IntradayNoData IntradayDataStatus = "no_data"
	// IntradayStoreFailed 回應裡有這檔的 K 棒，但寫入失敗（含寫入階段 context 到期）。
	IntradayStoreFailed IntradayDataStatus = "store_failed"
	// IntradayFetchFailed 這一檔所在的請求失敗（整批），回應本身沒拿到。
	IntradayFetchFailed IntradayDataStatus = "fetch_failed"
)

// IntradaySymbolResult 是一檔的結果。Err 只在 store_failed／fetch_failed 時有值，
// **原文只進 log**，呼叫端要對外輸出時先過安全分類（TransientCause）。
type IntradaySymbolResult struct {
	Symbol string
	Status IntradayDataStatus
	// LastBarTS 是回應裡（＝這次寫入的）最後一根的時間；written 以外是零值。
	LastBarTS time.Time
	// Bars 是這次寫入的根數；written 以外是 0。
	Bars int
	Err  error
}

package indicator

import "testing"

// flatCloses 產生 n 根完全相同的收盤價。
func flatCloses(n int, price float64) []float64 {
	closes := make([]float64, n)
	for i := range closes {
		closes[i] = price
	}
	return closes
}

// TestCalcRSIFlatSeriesIsNeutral 釘住「完全沒有波動 → 中性」。
//
// 為什麼要有這條：diff == 0 走的是 else 分支（lossSum -= 0），所以「整段都平盤」
// 與「一路上漲、完全沒有下跌」在 CalcRSI 裡會走到同一個 avgLoss == 0 分支。
// 舊實作對兩者都回 100，於是一檔完全沒成交波動的股票會被判成極度超買
// （isRSIOverbought 據此擋掉 BUY），而且 100 塞不進當時的 rsi14 DECIMAL(6,4)，
// 指標整支寫不進 DB——2026-09-01 的 2454 就是這樣。
// 現況規格見 docs/indicator-spec.md 的 RSI 邊界條件。
func TestCalcRSIFlatSeriesIsNeutral(t *testing.T) {
	// 120 根＝Engine.Compute 的 lookback，跟 live 觸發時的輸入規模一致。
	if got := CalcRSI(flatCloses(120, 4315), 14); got != 50 {
		t.Errorf("完全無波動時 RSI 應為中性 50，得到 %v", got)
	}
}

// TestCalcRSIMonotonicRiseIsHundred 釘住另一半：真的只漲不跌時仍是 100。
//
// **這條路徑不是 bug**，所以不能連它一起改掉。它也說明了為什麼光靠語意修正
// 不足以解決溢位——RSI 100 仍然是合法輸出，型別必須容得下（見 migration 075）。
func TestCalcRSIMonotonicRiseIsHundred(t *testing.T) {
	closes := make([]float64, 120)
	for i := range closes {
		closes[i] = 100 + float64(i)
	}
	if got := CalcRSI(closes, 14); got != 100 {
		t.Errorf("只漲不跌時 RSI 應為 100，得到 %v", got)
	}
}

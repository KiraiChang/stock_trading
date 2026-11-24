package signal

import (
	"math"
	"strconv"
	"strings"

	"github.com/trading/backend/internal/store"
)

// identityPriceScale 是價位正規化的量化尺度，與原本 samePrice 的 1e-6 容差同級。
const identityPriceScale = 1e6

// canonicalPrice 把浮點價位量化成穩定字串。
//
// **為什麼需要它**：DB 判重用容差比較（math.Abs(a-b) < 1e-6），Redis key 卻需要
// 離散字串。兩套定義各自為政時，同一組價位可能 DB 判「同一訊號」、Redis 判「不同訊號」，
// 判重就出現破口。**現在只有一個定義**——sameSignalIdentity 也改成比較這個 key。
//
// ⚠️ 這在 1e-6 邊界上會微幅改變原本的容差語意（剛好差約 1e-6 時可能不同調）。
// **那是刻意的取捨**：一個定義的可預測性，勝過兩個定義在邊界上各自正確。
func canonicalPrice(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "nan"
	}
	return strconv.FormatFloat(math.Round(f*identityPriceScale)/identityPriceScale, 'f', 6, 64)
}

// signalIdentityKey 產生訊號的身分字串。
//
// 取用的價位欄位依型別而定，與原本 sameSignalIdentity 的 switch 一致：
//
//	BREAKOUT                   → Resistance
//	BREAKDOWN / SUPPORT_BOUNCE → Support
//	其餘                        → 不帶價位
func signalIdentityKey(sig *store.Signal) string {
	if sig == nil {
		return ""
	}
	parts := []string{sig.Symbol, sig.SignalType, sig.Direction}
	switch sig.SignalType {
	case "BREAKOUT":
		parts = append(parts, canonicalPrice(sig.Resistance))
	case "BREAKDOWN", "SUPPORT_BOUNCE":
		parts = append(parts, canonicalPrice(sig.Support))
	}
	return strings.Join(parts, "|")
}

// signalIdentityKeyOf 是 store.Signal（值）版本，給 DB 判重比對用。
func signalIdentityKeyOf(sig store.Signal) string {
	return signalIdentityKey(&sig)
}

// emissionKey 是 Redis 與 local map 共用的 key。
func emissionKey(identity string) string {
	return "signal:emitted:" + identity
}

// intradayBounceTimeframe 是 B 規則適用的時間框架（I-140）：盤中評估用的 1 分 K。
const intradayBounceTimeframe = "1m"

// intradayBounceKey 是 B 規則（盤中 1 分 K 的 SUPPORT_BOUNCE，同一檔 15 分鐘只提醒一次）的身分鍵。
//
// **⛔ 不帶價位**——B 的取捨就是「15 分鐘內碰到不同支撐也只提醒一次」。帶 `tf=` 讓它跟 A 的價位鍵
// （`symbol|SUPPORT_BOUNCE|WATCH|價位`）字串格式不同，⛔ 不會互相碰撞；日 K 的 SUPPORT_BOUNCE 仍用 A 的鍵。
func intradayBounceKey(sig *store.Signal) string {
	return strings.Join([]string{sig.Symbol, sig.SignalType, sig.Direction, "tf=" + intradayBounceTimeframe}, "|")
}

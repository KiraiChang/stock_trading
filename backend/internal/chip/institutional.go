package chip

import (
	"fmt"

	"github.com/trading/backend/internal/store"
)

// 對應 docs/chip-analysis-design.md 第5節公式：
//   institutional_score = foreign*0.4 + investment_trust*0.35 + dealer*0.1 + total_net_buy_ratio*0.15
const (
	institutionalForeignWeight    = 0.4
	institutionalTrustWeight      = 0.35
	institutionalDealerWeight     = 0.1
	institutionalTotalRatioWeight = 0.15

	institutionalTrendLookback = 5
)

// CalcConsecutiveNetBuyDays 計算連續買超/賣超天數。輸入依日期升冪排序的淨
// 買超序列，從尾端往回數同號（買超為正、賣超為負）的連續天數，回傳值正負
// 代表方向（+4=連續買超4日，-2=連續賣超2日，0=最後一天無淨買賣超或序列為空）。
func CalcConsecutiveNetBuyDays(netBuys []int64) int {
	n := len(netBuys)
	if n == 0 || netBuys[n-1] == 0 {
		return 0
	}

	positive := netBuys[n-1] > 0
	days := 0
	for i := n - 1; i >= 0; i-- {
		if positive && netBuys[i] > 0 {
			days++
		} else if !positive && netBuys[i] < 0 {
			days++
		} else {
			break
		}
	}
	if positive {
		return days
	}
	return -days
}

// trendScore 依連續天數與近5日累積買超相對均量規模，算出單一法人身份的
// 趨勢分數（-100~100）。係數（15、500）為初始啟發式，待有真實資料回測後校正，
// 測試只驗證方向正確、單調遞增，不鎖死魔術數字。
func trendScore(consecutiveDays int, cumulative5Day, avgVolume20 float64) float64 {
	daysComponent := clamp(float64(consecutiveDays)*15, 100)
	var volumeComponent float64
	if avgVolume20 > 0 {
		volumeComponent = clamp(cumulative5Day/avgVolume20*500, 100)
	}
	return 0.5*daysComponent + 0.5*volumeComponent
}

func sumLastN(vals []int64, n int) float64 {
	if n > len(vals) {
		n = len(vals)
	}
	var sum int64
	for _, v := range vals[len(vals)-n:] {
		sum += v
	}
	return float64(sum)
}

// CalcInstitutionalScore 對應 institutional_score 公式。hist 需依 trade_date
// 升冪排序，最後一筆須為欲計算日的資料；avgVolume20 為近 20 日均量（股），
// dailyVolume 為當日成交量（股）。
func CalcInstitutionalScore(hist []store.InstitutionalTrade, avgVolume20 float64, dailyVolume int64) (score float64, reasons []string) {
	if len(hist) == 0 {
		return 0, []string{"無法人買賣超資料，institutional_score 中性"}
	}

	foreign := make([]int64, len(hist))
	trust := make([]int64, len(hist))
	dealer := make([]int64, len(hist))
	for i, h := range hist {
		foreign[i] = h.ForeignNetBuy
		trust[i] = h.InvestmentTrustNetBuy
		dealer[i] = h.DealerNetBuy
	}

	foreignDays := CalcConsecutiveNetBuyDays(foreign)
	trustDays := CalcConsecutiveNetBuyDays(trust)
	dealerDays := CalcConsecutiveNetBuyDays(dealer)

	foreignScore := trendScore(foreignDays, sumLastN(foreign, institutionalTrendLookback), avgVolume20)
	trustScore := trendScore(trustDays, sumLastN(trust, institutionalTrendLookback), avgVolume20)
	dealerScore := trendScore(dealerDays, sumLastN(dealer, institutionalTrendLookback), avgVolume20)

	latest := hist[len(hist)-1]
	var totalRatioScore float64
	if dailyVolume > 0 {
		totalRatioScore = clamp(float64(latest.TotalNetBuy)/float64(dailyVolume)*1000, 100)
	}

	score = foreignScore*institutionalForeignWeight +
		trustScore*institutionalTrustWeight +
		dealerScore*institutionalDealerWeight +
		totalRatioScore*institutionalTotalRatioWeight

	if foreignDays > 0 {
		reasons = append(reasons, fmt.Sprintf("外資連續買超 %d 日", foreignDays))
	} else if foreignDays < 0 {
		reasons = append(reasons, fmt.Sprintf("外資連續賣超 %d 日", -foreignDays))
	}
	if trustDays > 0 {
		reasons = append(reasons, fmt.Sprintf("投信連續買超 %d 日", trustDays))
	} else if trustDays < 0 {
		reasons = append(reasons, fmt.Sprintf("投信連續賣超 %d 日", -trustDays))
	}
	return score, reasons
}

package chip

// 對應 docs/chip-analysis-design.md 第5節總分公式：
//
//	total_score = institutional*0.35 + margin*0.20 + broker*0.30 + concentration*0.15
const (
	totalInstitutionalWeight = 0.35
	totalMarginWeight        = 0.20
	totalBrokerWeight        = 0.30
	totalConcentrationWeight = 0.15

	// signalThreshold 為 BULLISH/BEARISH 判斷門檻，總分絕對值需達到此門檻才
	// 視為有明顯方向，否則為 NEUTRAL。
	signalThreshold = 20.0
)

// CalcTotalScore 對應 total_score 公式。
func CalcTotalScore(institutional, margin, broker, concentration float64) float64 {
	return institutional*totalInstitutionalWeight +
		margin*totalMarginWeight +
		broker*totalBrokerWeight +
		concentration*totalConcentrationWeight
}

// ClassifySignal 依總分與融資風險判斷籌碼訊號。marginRisk 為 true 時一律
// 覆寫為 RISK（融資使用率過高本身就是要示警，不論總分高低）。
func ClassifySignal(totalScore float64, marginRisk bool) Signal {
	if marginRisk {
		return Risk
	}
	switch {
	case totalScore >= signalThreshold:
		return Bullish
	case totalScore <= -signalThreshold:
		return Bearish
	default:
		return Neutral
	}
}

// Calculate 是唯一對外的計分 orchestrator，串起法人/融資融券/分點/集中度
// 四個子分數與 signal 分類，不做任何 IO（IO 由 chip.Syncer 負責）。
func Calculate(input ChipScoreInput) Score {
	institutionalScore, institutionalReasons := CalcInstitutionalScore(input.InstitutionalHistory, input.AvgVolume20, input.DailyVolume)

	var marginScore float64
	var marginReasons []string
	var marginRisk bool
	if len(input.MarginHistory) > 0 {
		latest := input.MarginHistory[len(input.MarginHistory)-1]
		marginScore, marginReasons = CalcMarginScore(latest, input.PriceChangePercent)
		marginRisk = IsMarginRisk(latest.MarginUsageRate, marginRiskThreshold)
	} else {
		marginReasons = []string{"無融資融券資料，margin_score 中性"}
	}

	brokerScore, brokerReasons := CalcBrokerScore(input.BrokerTrades, input.DailyVolume)

	topBuy, _ := CalcTopNNetBuy(input.BrokerTrades, brokerConcentrationTopN)
	concentrationScore := clamp(CalcConcentration(topBuy, input.DailyVolume)*100, 100)
	if concentrationScore < 0 {
		concentrationScore = 0
	}

	totalScore := CalcTotalScore(institutionalScore, marginScore, brokerScore, concentrationScore)
	signal := ClassifySignal(totalScore, marginRisk)

	reasons := make([]string, 0, len(institutionalReasons)+len(marginReasons)+len(brokerReasons))
	reasons = append(reasons, institutionalReasons...)
	reasons = append(reasons, marginReasons...)
	reasons = append(reasons, brokerReasons...)

	return Score{
		InstitutionalScore: institutionalScore,
		MarginScore:        marginScore,
		BrokerScore:        brokerScore,
		ConcentrationScore: concentrationScore,
		TotalScore:         totalScore,
		Signal:             signal,
		Reasons:            reasons,
	}
}

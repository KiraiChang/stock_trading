package portfolio

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/store"
)

const (
	ActionHold          = "HOLD"
	ActionStopLoss      = "STOP_LOSS"
	ActionTakeProfit    = "TAKE_PROFIT"
	ActionReduce        = "REDUCE"
	ActionAddOnBreakout = "ADD_ON_BREAKOUT"

	defaultTimeframe  = "1d"
	defaultAddOnRatio = 0.25
)

type Analyzer struct {
	client      *analysis.Client
	holdings    store.HoldingRepo
	srZoneRepo  store.SRZoneRepo
	addOnRatio  float64
	defaultLimit int
}

func NewAnalyzer(client *analysis.Client, holdings store.HoldingRepo, srZoneRepo store.SRZoneRepo) *Analyzer {
	return &Analyzer{
		client:       client,
		holdings:     holdings,
		srZoneRepo:   srZoneRepo,
		addOnRatio:   defaultAddOnRatio,
		defaultLimit: 250,
	}
}

type AnalyzeOptions struct {
	Timeframe string
	Limit     int
}

type AnalyzeResult struct {
	Analysis *store.HoldingAnalysis `json:"analysis"`
	SR       *store.SRZoneAnalysis   `json:"sr_zone_analysis"`
	Zones    []store.SRZone         `json:"zones"`
}

func (a *Analyzer) Analyze(ctx context.Context, holdingID uint64, opts AnalyzeOptions) (*AnalyzeResult, error) {
	holding, err := a.holdings.Get(ctx, holdingID)
	if err != nil {
		return nil, err
	}
	if opts.Timeframe == "" {
		opts.Timeframe = defaultTimeframe
	}
	if opts.Limit == 0 {
		opts.Limit = a.defaultLimit
	}

	result, err := a.client.ScoreZones(ctx, holding.Symbol, opts.Timeframe, opts.Limit)
	if err != nil {
		return nil, err
	}
	srAnalysis, zones, err := result.ToStore()
	if err != nil {
		return nil, fmt.Errorf("convert sr zone result: %w", err)
	}
	srID, err := a.srZoneRepo.Create(ctx, srAnalysis, zones)
	if err != nil {
		return nil, fmt.Errorf("create sr zone analysis: %w", err)
	}
	savedSR, err := a.srZoneRepo.Get(ctx, srID)
	if err != nil {
		return nil, fmt.Errorf("get saved sr zone analysis: %w", err)
	}
	savedZones, err := a.srZoneRepo.GetZones(ctx, srID)
	if err != nil {
		return nil, fmt.Errorf("get saved sr zones: %w", err)
	}

	analysisSnapshot, err := a.buildSnapshot(holding, savedSR, savedZones)
	if err != nil {
		return nil, err
	}
	id, err := a.holdings.CreateAnalysis(ctx, analysisSnapshot)
	if err != nil {
		return nil, fmt.Errorf("create holding analysis: %w", err)
	}
	savedAnalysis, err := a.holdings.GetAnalysis(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get saved holding analysis: %w", err)
	}

	return &AnalyzeResult{Analysis: savedAnalysis, SR: savedSR, Zones: savedZones}, nil
}

func (a *Analyzer) buildSnapshot(h *store.Holding, sr *store.SRZoneAnalysis, zones []store.SRZone) (*store.HoldingAnalysis, error) {
	current := sr.CurrentPrice
	unrealized := (current - h.CostPrice) * h.Shares
	unrealizedPct := 0.0
	if h.CostPrice > 0 {
		unrealizedPct = (current - h.CostPrice) / h.CostPrice
	}

	support := pickSupportZone(zones, current)
	resistance := pickResistanceZone(zones, current)
	decisionAction := decisionAction(sr.DecisionSummary)

	action, label := ActionHold, "繼續持有"
	reasons := []string{
		fmt.Sprintf("以 %.2f 的最新收盤價與 SR Zone 快照評估。", current),
	}

	var stopLossPrice, stopLossAmount store.NullFloat64
	if support != nil {
		stopLossPrice = store.NewNullFloat64(support.PriceLow)
		stopLossAmount = store.NewNullFloat64(math.Max(0, (h.CostPrice-support.PriceLow)*h.Shares))
		reasons = append(reasons, fmt.Sprintf("主要停損參考下方支撐區 %.2f~%.2f。", support.PriceLow, support.PriceHigh))
		if current < support.PriceLow || support.Status == "BROKEN" {
			action, label = ActionStopLoss, "停損"
			reasons = append(reasons, "收盤價已跌破主要支撐區，優先控管下檔風險。")
		}
	} else {
		reasons = append(reasons, "目前沒有可用的下方支撐區，停損價暫不給定。")
	}

	var takeProfitPrice, takeProfitAmount, addOnTriggerPrice, addOnAmount store.NullFloat64
	if resistance != nil {
		takeProfitPrice = store.NewNullFloat64(resistance.PriceLow)
		takeProfitAmount = store.NewNullFloat64(math.Max(0, (resistance.PriceLow-h.CostPrice)*h.Shares))
		addOnTriggerPrice = store.NewNullFloat64(resistance.PriceHigh)
		addOnAmount = store.NewNullFloat64(current * h.Shares * a.addOnRatio)
		reasons = append(reasons, fmt.Sprintf("主要停利參考上方壓力區 %.2f~%.2f；若收盤突破 %.2f，可再評估加碼。", resistance.PriceLow, resistance.PriceHigh, resistance.PriceHigh))
		if action == ActionHold && near(current, resistance.PriceLow, 0.02) && resistance.TradingScore >= 60 {
			action, label = ActionTakeProfit, "停利"
			reasons = append(reasons, "收盤價已接近高分壓力區，優先檢查停利或減碼。")
		}
	} else {
		reasons = append(reasons, "目前沒有可用的上方壓力區，加碼與停利價暫不給定。")
	}

	if action == ActionHold && decisionAction == "Avoid" {
		action, label = ActionReduce, "減碼"
		reasons = append(reasons, "SR Zone 決策層給出 Avoid，持股建議降風險。")
	}
	if action == ActionHold && (decisionAction == "Buy" || decisionAction == "BuySmall") && resistance != nil {
		action, label = ActionAddOnBreakout, "突破加碼觀察"
		reasons = append(reasons, "SR Zone 決策層偏多，但仍以突破上方壓力區後再加碼為準。")
	}

	reasonJSON, err := json.Marshal(reasons)
	if err != nil {
		return nil, err
	}
	detailJSON, err := json.Marshal(map[string]any{
		"rule_version":       "holding_sr_zone_v1",
		"add_on_ratio":       a.addOnRatio,
		"sr_decision_action": decisionAction,
		"support_zone":       zoneDetail(support),
		"resistance_zone":    zoneDetail(resistance),
	})
	if err != nil {
		return nil, err
	}

	return &store.HoldingAnalysis{
		HoldingID:           h.ID,
		Symbol:              h.Symbol,
		Shares:              h.Shares,
		CostPrice:           h.CostPrice,
		AnalyzedAt:          sr.AnalyzedAt,
		CurrentPrice:        current,
		SRZoneAnalysisID:    store.NewNullInt64(int64(sr.ID)),
		Action:              action,
		ActionLabel:         label,
		StopLossPrice:       stopLossPrice,
		StopLossAmount:      stopLossAmount,
		TakeProfitPrice:     takeProfitPrice,
		TakeProfitAmount:    takeProfitAmount,
		AddOnTriggerPrice:   addOnTriggerPrice,
		AddOnAmount:         addOnAmount,
		UnrealizedPnL:       unrealized,
		UnrealizedPnLPct:    unrealizedPct,
		Reason:              store.RawJSON(reasonJSON),
		DetailJSON:          store.RawJSON(detailJSON),
	}, nil
}

func pickSupportZone(zones []store.SRZone, current float64) *store.SRZone {
	candidates := make([]store.SRZone, 0)
	for _, z := range zones {
		if effectiveRole(z) == "SUPPORT" && z.PriceLow <= current {
			candidates = append(candidates, z)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].PriceHigh == candidates[j].PriceHigh {
			return candidates[i].TradingScore > candidates[j].TradingScore
		}
		return candidates[i].PriceHigh > candidates[j].PriceHigh
	})
	if len(candidates) == 0 {
		return nil
	}
	return &candidates[0]
}

func pickResistanceZone(zones []store.SRZone, current float64) *store.SRZone {
	candidates := make([]store.SRZone, 0)
	for _, z := range zones {
		if effectiveRole(z) == "RESISTANCE" && z.PriceHigh >= current {
			candidates = append(candidates, z)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].PriceLow == candidates[j].PriceLow {
			return candidates[i].TradingScore > candidates[j].TradingScore
		}
		return candidates[i].PriceLow < candidates[j].PriceLow
	})
	if len(candidates) == 0 {
		return nil
	}
	return &candidates[0]
}

func effectiveRole(z store.SRZone) string {
	if z.ResolvedRole.Valid && z.ResolvedRole.String != "" {
		return z.ResolvedRole.String
	}
	return z.Role
}

func near(current, target, pct float64) bool {
	if target <= 0 {
		return false
	}
	return math.Abs(current-target)/target <= pct
}

func decisionAction(raw store.RawJSON) string {
	if raw == "" || string(raw) == "null" {
		return ""
	}
	var payload struct {
		Action string `json:"action"`
	}
	_ = json.Unmarshal([]byte(raw), &payload)
	return payload.Action
}

func zoneDetail(z *store.SRZone) any {
	if z == nil {
		return nil
	}
	return map[string]any{
		"id":                     z.ID,
		"price_low":              z.PriceLow,
		"price_high":             z.PriceHigh,
		"role":                   effectiveRole(*z),
		"tier":                   z.Tier,
		"confidence":             z.Confidence,
		"confidence_level":       z.ConfidenceLevel,
		"trading_score":          z.TradingScore,
		"trading_recommendation": z.TradingRecommendation,
		"status":                 z.Status,
	}
}

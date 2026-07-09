package portfolio

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/store"
)

const (
	StateFlat = "FLAT"
	StateLong = "LONG"

	ActionEnter      = "ENTER"
	ActionEnterSmall = "ENTER_SMALL"
	ActionWait       = "WAIT"
	ActionAvoid      = "AVOID"
	ActionHold       = "HOLD"
	ActionAdd        = "ADD"
	ActionReduce     = "REDUCE"
	ActionTakeProfit = "TAKE_PROFIT"
	ActionExitStop   = "EXIT_STOP"

	defaultTimeframe        = "1d"
	existingSRSnapshotLimit = 200
)

type Config struct {
	MaxPositionValue         float64       `json:"max_position_value"`
	MaxRiskAmount            float64       `json:"max_risk_amount"`
	AddOnRatio               float64       `json:"add_on_ratio"`
	MinRiskRewardRatio       float64       `json:"min_risk_reward_ratio"`
	BreakoutTargetRR         float64       `json:"breakout_target_risk_reward_ratio"`
	TakeProfitReductionRatio float64       `json:"take_profit_reduction_ratio"`
	SRReuseMaxAge            time.Duration `json:"-"`
}

func DefaultConfig() Config {
	return Config{
		MaxPositionValue: 200000, MaxRiskAmount: 10000, AddOnRatio: 0.25,
		MinRiskRewardRatio: 1.5, BreakoutTargetRR: 2, TakeProfitReductionRatio: 0.5,
		SRReuseMaxAge: 24 * time.Hour,
	}
}

type Analyzer struct {
	client     *analysis.Client
	positions  store.PositionRepo
	srZoneRepo store.SRZoneRepo
	config     Config
	now        func() time.Time
}

func NewAnalyzer(client *analysis.Client, positions store.PositionRepo, srZoneRepo store.SRZoneRepo, config Config) *Analyzer {
	// 逐欄位補預設：任何一項風險/部位參數缺漏或 <= 0 都會讓部位計算靜默失效
	// （例如 MaxRiskAmount=0 會讓 riskShares=0、永遠 target 0），所以每項各自回退。
	d := DefaultConfig()
	if config.MaxPositionValue <= 0 {
		config.MaxPositionValue = d.MaxPositionValue
	}
	if config.MaxRiskAmount <= 0 {
		config.MaxRiskAmount = d.MaxRiskAmount
	}
	if config.AddOnRatio <= 0 {
		config.AddOnRatio = d.AddOnRatio
	}
	if config.MinRiskRewardRatio <= 0 {
		config.MinRiskRewardRatio = d.MinRiskRewardRatio
	}
	if config.BreakoutTargetRR <= 0 {
		config.BreakoutTargetRR = d.BreakoutTargetRR
	}
	if config.TakeProfitReductionRatio <= 0 {
		config.TakeProfitReductionRatio = d.TakeProfitReductionRatio
	}
	if config.SRReuseMaxAge <= 0 {
		config.SRReuseMaxAge = d.SRReuseMaxAge
	}
	return &Analyzer{client: client, positions: positions, srZoneRepo: srZoneRepo, config: config, now: time.Now}
}

type AnalyzeOptions struct {
	Timeframe    string
	Limit        int
	ForceRefresh bool
}

type AnalyzeResult struct {
	Analysis *store.PositionAnalysis `json:"analysis"`
	SR       *store.SRZoneAnalysis   `json:"sr_zone_analysis"`
	Zones    []store.SRZone          `json:"zones"`
}

func (a *Analyzer) Analyze(ctx context.Context, symbol string, opts AnalyzeOptions) (*AnalyzeResult, error) {
	position, err := a.positions.Get(ctx, symbol)
	if errors.Is(err, sql.ErrNoRows) {
		position = &store.Position{Symbol: symbol}
	} else if err != nil {
		return nil, err
	}
	if opts.Timeframe == "" {
		opts.Timeframe = defaultTimeframe
	}
	if opts.Limit == 0 {
		opts.Limit = 250
	}

	sr, zones, err := a.loadSR(ctx, symbol, opts)
	if err != nil {
		return nil, err
	}
	snapshot, err := a.buildSnapshot(position, sr, zones)
	if err != nil {
		return nil, err
	}
	id, err := a.positions.CreateAnalysis(ctx, snapshot)
	if err != nil {
		return nil, fmt.Errorf("create position analysis: %w", err)
	}
	saved, err := a.positions.GetAnalysis(ctx, id)
	if err != nil {
		snapshot.ID = id
		snapshot.CreatedAt = a.currentTime()
		saved = snapshot
	}
	return &AnalyzeResult{Analysis: saved, SR: sr, Zones: zones}, nil
}

func (a *Analyzer) loadSR(ctx context.Context, symbol string, opts AnalyzeOptions) (*store.SRZoneAnalysis, []store.SRZone, error) {
	if !opts.ForceRefresh {
		analyses, err := a.srZoneRepo.List(ctx, symbol, existingSRSnapshotLimit)
		if err != nil {
			return nil, nil, fmt.Errorf("list existing sr zone analyses: %w", err)
		}
		for i := range analyses {
			if analyses[i].Timeframe != opts.Timeframe {
				continue
			}
			age := a.currentTime().Sub(analyses[i].AnalyzedAt)
			if age < 0 || age > a.config.SRReuseMaxAge {
				continue
			}
			zones, err := a.srZoneRepo.GetZones(ctx, analyses[i].ID)
			return &analyses[i], zones, err
		}
	}

	result, err := a.client.ScoreZones(ctx, symbol, opts.Timeframe, opts.Limit)
	if err != nil {
		return nil, nil, err
	}
	sr, zones, err := result.ToStore()
	if err != nil {
		return nil, nil, err
	}
	id, err := a.srZoneRepo.Create(ctx, sr, zones)
	if err != nil {
		return nil, nil, err
	}
	saved, err := a.srZoneRepo.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	savedZones, err := a.srZoneRepo.GetZones(ctx, id)
	return saved, savedZones, err
}

func (a *Analyzer) buildSnapshot(position *store.Position, sr *store.SRZoneAnalysis, zones []store.SRZone) (*store.PositionAnalysis, error) {
	current := sr.CurrentPrice
	state := StateFlat
	if position.Shares > 0 {
		state = StateLong
	}
	unrealized, unrealizedPct := 0.0, 0.0
	if state == StateLong {
		unrealized = (current - position.AvgCost) * position.Shares
		if position.AvgCost > 0 {
			unrealizedPct = (current - position.AvgCost) / position.AvgCost
		}
	}

	support := pickSupportZone(zones, current)
	brokenSupport := pickBrokenSupport(zones, current)
	resistance := pickResistanceZone(zones, current)
	srAction := decisionAction(sr.DecisionSummary)

	var stop, takeProfit, riskAmount, rewardAmount, rr store.NullFloat64
	fullTarget := 0.0
	if support != nil && current > support.PriceLow {
		stop = store.NewNullFloat64(support.PriceLow)
		capitalShares := math.Floor(a.config.MaxPositionValue / current)
		riskShares := math.Floor(a.config.MaxRiskAmount / (current - support.PriceLow))
		fullTarget = math.Max(0, math.Min(capitalShares, riskShares))
	}
	takeProfitSource := ""
	if resistance != nil && resistance.PriceLow > current {
		takeProfit = store.NewNullFloat64(resistance.PriceLow)
		takeProfitSource = "RESISTANCE_ZONE"
	} else if stop.Valid {
		takeProfit = store.NewNullFloat64(current + (current-stop.Float64)*a.config.BreakoutTargetRR)
		takeProfitSource = "BREAKOUT_R_MULTIPLE"
	}
	if stop.Valid && takeProfit.Valid && current > stop.Float64 {
		rr = store.NewNullFloat64((takeProfit.Float64 - current) / (current - stop.Float64))
	}

	action, label, target := ActionWait, "等待", 0.0
	reasons := []string{fmt.Sprintf("以 %.2f 的最新收盤價與 SR Zone v2 快照評估。", current)}
	triggers := []string{}
	invalidations := []string{}
	canIncrease := stop.Valid && rr.Valid && rr.Float64 >= a.config.MinRiskRewardRatio
	if state == StateFlat {
		switch {
		case srAction == "Avoid":
			action, label = ActionAvoid, "避開"
		case srAction == "Buy" && canIncrease:
			action, label, target = ActionEnter, "建立部位", fullTarget
		case srAction == "BuySmall" && canIncrease:
			action, label, target = ActionEnterSmall, "小量建立", math.Floor(fullTarget*0.5)
		default:
			action, label = ActionWait, "等待"
			if !stop.Valid {
				reasons = append(reasons, "沒有有效支撐，無法量化停損風險。")
			} else if !rr.Valid || rr.Float64 < a.config.MinRiskRewardRatio {
				reasons = append(reasons, "風險報酬比未達進場門檻。")
			}
		}
	} else {
		broken := brokenSupport != nil
		switch {
		case broken:
			action, label, target = ActionExitStop, "停損出場", 0
		case resistance != nil && near(current, resistance.PriceLow, 0.02) && resistance.TradingScore >= 60:
			action, label = ActionTakeProfit, "停利減碼"
			target = math.Floor(position.Shares * (1 - a.config.TakeProfitReductionRatio))
		case !stop.Valid:
			action, label = ActionReduce, "降低風險"
			target = math.Floor(position.Shares * 0.5)
			reasons = append(reasons, "沒有有效支撐，無法量化停損風險。")
		case srAction == "Avoid":
			action, label = ActionReduce, "減碼"
			target = math.Floor(position.Shares * 0.5)
		case srAction == "Buy" && canIncrease:
			target = fullTarget
			if target > position.Shares {
				target = math.Min(target, position.Shares+math.Floor(fullTarget*a.config.AddOnRatio))
			}
			action, label = actionForDelta(target-position.Shares, ActionHold, "繼續持有")
		case srAction == "BuySmall" && canIncrease:
			target = math.Floor(fullTarget * 0.5)
			if target > position.Shares {
				target = math.Min(target, position.Shares+math.Floor(fullTarget*a.config.AddOnRatio))
			}
			action, label = actionForDelta(target-position.Shares, ActionHold, "繼續持有")
		default:
			target = position.Shares
			if fullTarget > 0 && target > fullTarget {
				target = fullTarget
				action, label = ActionReduce, "降至風險上限"
			} else {
				action, label = ActionHold, "繼續持有"
			}
		}
	}
	target = math.Max(0, math.Floor(target))
	delta := target - position.Shares
	side := "NONE"
	if delta > 0 {
		side = "BUY"
	} else if delta < 0 {
		side = "SELL"
	}
	if stop.Valid {
		riskAmount = store.NewNullFloat64(math.Max(0, (current-stop.Float64)*target))
	}
	if takeProfit.Valid {
		rewardAmount = store.NewNullFloat64(math.Max(0, (takeProfit.Float64-current)*target))
	}
	if stop.Valid {
		invalidations = append(invalidations, fmt.Sprintf("收盤跌破 %.2f", stop.Float64))
	}
	if takeProfit.Valid {
		triggers = append(triggers, fmt.Sprintf("價格接近或突破 %.2f", takeProfit.Float64))
	}
	reasons = append(reasons, fmt.Sprintf("SR 決策為 %s；目前 %.0f 股，目標 %.0f 股。", srAction, position.Shares, target))

	configJSON, _ := json.Marshal(a.config)
	reasonJSON, _ := json.Marshal(reasons)
	triggerJSON, _ := json.Marshal(triggers)
	invalidationJSON, _ := json.Marshal(invalidations)
	evidenceJSON, _ := json.Marshal(map[string]any{
		"sr_decision_action": srAction,
		"support_zone":       zoneDetail(support),
		"resistance_zone":    zoneDetail(resistance),
		"take_profit_source": takeProfitSource,
	})
	return &store.PositionAnalysis{
		Symbol: position.Symbol, PositionState: state, PositionVersion: position.Version,
		Shares: position.Shares, AvgCost: position.AvgCost, RealizedPnL: position.RealizedPnL,
		AnalyzedAt: sr.AnalyzedAt, CurrentPrice: current,
		SRZoneAnalysisID: store.NewNullInt64(int64(sr.ID)), Action: action, ActionLabel: label,
		TargetShares: target, AdjustmentShares: delta, AdjustmentSide: side,
		AdjustmentAmount: math.Abs(delta) * current, EntryPrice: store.NewNullFloat64(current),
		StopLossPrice: stop, TakeProfitPrice: takeProfit, RiskAmount: riskAmount,
		ExpectedRewardAmount: rewardAmount, RiskRewardRatio: rr,
		UnrealizedPnL: unrealized, UnrealizedPnLPct: unrealizedPct,
		ConfigJSON: store.RawJSON(configJSON), Reason: store.RawJSON(reasonJSON),
		Evidence: store.RawJSON(evidenceJSON), TriggerConditions: store.RawJSON(triggerJSON),
		InvalidationConditions: store.RawJSON(invalidationJSON), RuleVersion: "position_sr_zone_v1",
	}, nil
}

func actionForDelta(delta float64, zeroAction, zeroLabel string) (string, string) {
	if delta > 0 {
		return ActionAdd, "加碼"
	}
	if delta < 0 {
		return ActionReduce, "減碼"
	}
	return zeroAction, zeroLabel
}

func pickSupportZone(zones []store.SRZone, current float64) *store.SRZone {
	candidates := make([]store.SRZone, 0)
	for _, z := range zones {
		if effectiveRole(z) != "SUPPORT" || z.PriceLow > current || z.Status == "BROKEN" {
			continue
		}
		candidates = append(candidates, z)
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
		if effectiveRole(z) == "RESISTANCE" && z.PriceHigh >= current && z.Status != "BROKEN" {
			candidates = append(candidates, z)
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].PriceLow < candidates[j].PriceLow })
	if len(candidates) == 0 {
		return nil
	}
	return &candidates[0]
}

func pickBrokenSupport(zones []store.SRZone, current float64) *store.SRZone {
	var belowOrAt, all []store.SRZone
	for _, z := range zones {
		if effectiveRole(z) != "SUPPORT" {
			continue
		}
		all = append(all, z)
		if z.PriceLow <= current {
			belowOrAt = append(belowOrAt, z)
		}
	}
	// 現價（含）以下的支撐才是多單的保護。有這種支撐時，只有它被標記為 BROKEN
	// 才算跌破停損；上方的支撐不能拿來當停損依據，否則現價下方明明有有效支撐時，
	// 會被上方支撐帶誤判成「跌破」而全數出場。
	if len(belowOrAt) > 0 {
		sort.Slice(belowOrAt, func(i, j int) bool {
			return distanceToZone(belowOrAt[i], current) < distanceToZone(belowOrAt[j], current)
		})
		if belowOrAt[0].Status == "BROKEN" {
			return &belowOrAt[0]
		}
		return nil
	}
	// 現價已跌破所有支撐帶（連最靠近的支撐都在上方）→ 結構性跌破，出場。
	if len(all) > 0 {
		sort.Slice(all, func(i, j int) bool {
			return distanceToZone(all[i], current) < distanceToZone(all[j], current)
		})
		return &all[0]
	}
	return nil
}

func distanceToZone(zone store.SRZone, price float64) float64 {
	if price < zone.PriceLow {
		return zone.PriceLow - price
	}
	if price > zone.PriceHigh {
		return price - zone.PriceHigh
	}
	return 0
}

func effectiveRole(z store.SRZone) string {
	if z.ResolvedRole.Valid && z.ResolvedRole.String != "" {
		return z.ResolvedRole.String
	}
	return z.Role
}

func near(current, target, pct float64) bool {
	return target > 0 && math.Abs(current-target)/target <= pct
}

func decisionAction(raw store.RawJSON) string {
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
		"id": z.ID, "price_low": z.PriceLow, "price_high": z.PriceHigh,
		"role": effectiveRole(*z), "tier": z.Tier, "confidence": z.Confidence,
		"trading_score": z.TradingScore, "status": z.Status,
	}
}

func (a *Analyzer) currentTime() time.Time {
	if a.now != nil {
		return a.now()
	}
	return time.Now()
}

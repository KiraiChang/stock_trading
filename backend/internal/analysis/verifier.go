package analysis

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/trading/backend/internal/store"
)

// Verifier 重新比對已存的分析快照跟後續實際走勢：支撐/壓力有沒有被突破、
// （若進場為 ACTIVE）停損/停利有沒有被觸及。可重複呼叫，每次都用目前為止
// 最新的 candles 重新計算，不是一次性蓋棺定論的狀態機。
type Verifier struct {
	repo    store.AnalysisRepo
	candles store.CandleRepo
}

func NewVerifier(repo store.AnalysisRepo, candles store.CandleRepo) *Verifier {
	return &Verifier{repo: repo, candles: candles}
}

// TouchResult 描述某個價位是否已被觸及
type TouchResult struct {
	Hit      bool       `json:"hit"`
	HitAt    *time.Time `json:"hit_at,omitempty"`
	HitPrice *float64   `json:"hit_price,omitempty"`
}

// TradeVerification 為寫入 stock_analyses.trade_verification 的 JSON 結構
type TradeVerification struct {
	Applicable bool                   `json:"applicable"`
	StopLoss   map[string]TouchResult `json:"stop_loss,omitempty"`
	TakeProfit map[string]TouchResult `json:"take_profit,omitempty"`
}

func (v *Verifier) Verify(ctx context.Context, analysisID uint64) (*store.StockAnalysis, []store.StockAnalysisLevel, error) {
	a, err := v.repo.Get(ctx, analysisID)
	if err != nil {
		return nil, nil, err
	}
	levels, err := v.repo.GetLevels(ctx, analysisID)
	if err != nil {
		return nil, nil, err
	}

	candles, err := v.candles.GetRange(ctx, a.Symbol, a.Timeframe, a.AnalyzedAt, time.Now())
	if err != nil {
		return nil, nil, err
	}
	// 分析當下那根K棒本身不算「之後」，只看嚴格晚於 analyzed_at 的資料，
	// 避免用產生分析當下就已經知道的價格「驗證」自己
	future := make([]store.Candle, 0, len(candles))
	for _, c := range candles {
		if c.Timestamp.After(a.AnalyzedAt) {
			future = append(future, c)
		}
	}

	for i := range levels {
		status, brokenAt, brokenPrice := verifyLevel(levels[i], future)
		if err := v.repo.UpdateLevelStatus(ctx, levels[i].ID, status, brokenAt, brokenPrice); err != nil {
			return nil, nil, err
		}
	}

	verification := verifyTrade(a, future)
	verificationJSON, err := json.Marshal(verification)
	if err != nil {
		return nil, nil, err
	}
	if err := v.repo.UpdateVerification(ctx, analysisID, string(verificationJSON)); err != nil {
		return nil, nil, err
	}

	updated, err := v.repo.Get(ctx, analysisID)
	if err != nil {
		return nil, nil, err
	}
	updatedLevels, err := v.repo.GetLevels(ctx, analysisID)
	if err != nil {
		return nil, nil, err
	}
	return updated, updatedLevels, nil
}

// verifyLevel 檢查支撐/壓力有沒有被突破：支撐用收盤跌破、壓力用收盤漲破
// （跟 Go signal engine 的 breakout/breakdown 判斷一致，不是碰到影線就算數）
func verifyLevel(lv store.StockAnalysisLevel, candles []store.Candle) (status string, brokenAt *time.Time, brokenPrice *float64) {
	for _, c := range candles {
		broken := (lv.Type == "SUPPORT" && c.Close < lv.Price) || (lv.Type == "RESISTANCE" && c.Close > lv.Price)
		if broken {
			t, p := c.Timestamp, c.Close
			return "BROKEN", &t, &p
		}
	}
	return "HELD_SO_FAR", nil, nil
}

// verifyTrade 只在進場狀態為 ACTIVE（真的觸發過進場條件）時才有意義；
// WATCHING（只是觀察中的觸發價位，沒有真正進場）標記為 not applicable。
// 停損用當根最低/最高價觸發（跟 backtester.py 的停損成交邏輯一致），
// 三種停損、三種停利分開獨立檢查，不互相配對，由使用者自行比較先後順序。
func verifyTrade(a *store.StockAnalysis, candles []store.Candle) TradeVerification {
	if a.EntryStatus != "ACTIVE" {
		return TradeVerification{Applicable: false}
	}
	isLong := a.EntryDirection == "LONG"

	checkStop := func(price sql.NullFloat64) TouchResult {
		if !price.Valid {
			return TouchResult{}
		}
		for _, c := range candles {
			hit := (isLong && c.Low <= price.Float64) || (!isLong && c.High >= price.Float64)
			if hit {
				t, p := c.Timestamp, price.Float64
				return TouchResult{Hit: true, HitAt: &t, HitPrice: &p}
			}
		}
		return TouchResult{Hit: false}
	}

	checkTarget := func(price sql.NullFloat64) TouchResult {
		if !price.Valid {
			return TouchResult{}
		}
		for _, c := range candles {
			hit := (isLong && c.High >= price.Float64) || (!isLong && c.Low <= price.Float64)
			if hit {
				t, p := c.Timestamp, price.Float64
				return TouchResult{Hit: true, HitAt: &t, HitPrice: &p}
			}
		}
		return TouchResult{Hit: false}
	}

	return TradeVerification{
		Applicable: true,
		StopLoss: map[string]TouchResult{
			"atr":        checkStop(a.StopLossATR),
			"structural": checkStop(a.StopLossStructural),
			"composite":  checkStop(a.StopLossComposite),
		},
		TakeProfit: map[string]TouchResult{
			"next_level":   checkTarget(a.TakeProfitNextLevel),
			"risk_reward":  checkTarget(a.TakeProfitRiskReward),
			"atr_multiple": checkTarget(a.TakeProfitATR),
		},
	}
}

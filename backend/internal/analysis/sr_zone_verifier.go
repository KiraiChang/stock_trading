package analysis

import (
	"context"
	"time"

	"github.com/trading/backend/internal/store"
)

// SRZoneVerifier 重新比對已存的 SR zone 分析快照跟後續實際走勢：每個 zone
// 有沒有被突破。跟 Verifier（個股分析，見 verifier.go）的差異：zone 是
// 一段價格區間、不是單一價位，且角色可能是 AT_ZONE（分析當下現價落在區間
// 內，方向未定），這裡額外處理「先等價格離開區間才能判斷方向」（見
// verifySRZone）。可重複呼叫，每次都用目前為止最新的 candles 重新計算，
// 不是一次性蓋棺定論的狀態機。
type SRZoneVerifier struct {
	repo    store.SRZoneRepo
	candles store.CandleRepo
}

func NewSRZoneVerifier(repo store.SRZoneRepo, candles store.CandleRepo) *SRZoneVerifier {
	return &SRZoneVerifier{repo: repo, candles: candles}
}

// DefaultConfirmationBars 跟 Python sr_scoring/features.py::count_breakouts
// 的預設值一致（compute_zone_features 呼叫時 confirmation_bars=2），讓
// 「這個 zone 算不算被突破」在特徵工程跟事後驗證用同一套標準，不要各自定
// 一套門檻。
const DefaultConfirmationBars = 2

func (v *SRZoneVerifier) Verify(ctx context.Context, analysisID uint64) (*store.SRZoneAnalysis, []store.SRZone, error) {
	a, err := v.repo.Get(ctx, analysisID)
	if err != nil {
		return nil, nil, err
	}
	zones, err := v.repo.GetZones(ctx, analysisID)
	if err != nil {
		return nil, nil, err
	}

	candles, err := v.candles.GetRange(ctx, a.Symbol, a.Timeframe, a.AnalyzedAt, time.Now())
	if err != nil {
		return nil, nil, err
	}
	// 分析當下那根K棒本身不算「之後」，只看嚴格晚於 analyzed_at 的資料，
	// 避免用產生分析當下就已經知道的價格「驗證」自己（比照 verifier.go）。
	future := make([]store.Candle, 0, len(candles))
	for _, c := range candles {
		if c.Timestamp.After(a.AnalyzedAt) {
			future = append(future, c)
		}
	}

	for i := range zones {
		status, brokenAt, brokenPrice, resolvedRole := verifySRZone(zones[i], future, DefaultConfirmationBars)
		if err := v.repo.UpdateZoneStatus(ctx, zones[i].ID, status, brokenAt, brokenPrice, resolvedRole); err != nil {
			return nil, nil, err
		}
	}

	updated, err := v.repo.Get(ctx, analysisID)
	if err != nil {
		return nil, nil, err
	}
	updatedZones, err := v.repo.GetZones(ctx, analysisID)
	if err != nil {
		return nil, nil, err
	}
	return updated, updatedZones, nil
}

// verifySRZone 判斷這個 zone 後續走勢，回傳新的 status/broken_at/broken_price/
// resolvedRole：
//
//   - role=AT_ZONE：分析當下現價落在區間內，方向未定，不能直接套用
//     SUPPORT/RESISTANCE 的突破判斷。先找「收盤真正離開區間」的第一根K棒
//     決定方向（收在上方 → 這個 zone 對它而言變成支撐；收在下方 → 變成
//     壓力），離開之前維持 PENDING（現價還在區間內，既不是「守住」也不是
//     「跌破」，沒有方向可以驗證）。解析出來的方向回傳在 resolvedRole
//     （不覆寫原始 z.Role，保留「分析當下是 AT_ZONE」這個歷史資訊，見
//     docs/sr-zone-scoring.md 十五「Zone 生命週期驗證」）；role 本身就不是 AT_ZONE 時
//     resolvedRole 永遠是空字串，呼叫端據此把 DB 欄位維持 NULL。
//   - role=SUPPORT：收盤連續 confirmationBars 根低於 price_low 視為 BROKEN
//     （比照 count_breakouts 的 streak state machine，避免單一天雜訊誤判）。
//   - role=RESISTANCE：收盤連續 confirmationBars 根高於 price_high 視為 BROKEN。
//   - 沒被突破，但期間 K 棒範圍曾與區間相交（觸碰過）→ HELD_SO_FAR。
//   - 沒被突破、也從未被觸碰 → 維持 PENDING（還沒有任何驗證資訊）。
//
// 每次都是從候選 candles 的開頭重新掃描（不是從上次驗證結果繼續），所以
// 一旦某次驗證判定 BROKEN，之後不管價格如何反彈，重新呼叫這個函式永遠會
// 在同一根K棒判定 BROKEN——不會被後續反彈改回 HELD_SO_FAR；resolvedRole
// 同理，一旦解析出方向就不會因為後續資料變動而改變。
func verifySRZone(z store.SRZone, candles []store.Candle, confirmationBars int) (status string, brokenAt *time.Time, brokenPrice *float64, resolvedRole string) {
	role := z.Role
	if role == "AT_ZONE" {
		exitAt := -1
		for i, c := range candles {
			if c.Close > z.PriceHigh {
				role = "SUPPORT"
				exitAt = i
				break
			}
			if c.Close < z.PriceLow {
				role = "RESISTANCE"
				exitAt = i
				break
			}
		}
		if exitAt == -1 {
			return "PENDING", nil, nil, ""
		}
		resolvedRole = role
		candles = candles[exitAt:]
	}

	touched := false
	streak := 0
	var streakStart store.Candle
	for _, c := range candles {
		if c.Low <= z.PriceHigh && c.High >= z.PriceLow {
			touched = true
		}

		broken := (role == "SUPPORT" && c.Close < z.PriceLow) || (role == "RESISTANCE" && c.Close > z.PriceHigh)
		if broken {
			if streak == 0 {
				streakStart = c
			}
			streak++
		} else {
			streak = 0
		}

		if broken && streak >= confirmationBars {
			t, p := streakStart.Timestamp, streakStart.Close
			return "BROKEN", &t, &p, resolvedRole
		}
	}

	if touched {
		return "HELD_SO_FAR", nil, nil, resolvedRole
	}
	return "PENDING", nil, nil, resolvedRole
}

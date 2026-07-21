package signal

import (
	"math"

	"github.com/trading/backend/internal/store"
)

type Level struct {
	Price    float64
	Strength float64 // 0.0~1.0
	Type     string  // "Support" / "Resistance"
}

const (
	srLookback  = 60
	mergeThresh = 0.01 // 1% 以內視為同一價位
	maxLevels   = 3
)

// CalcSupportResistance 識別支撐與阻力位
func CalcSupportResistance(candles []store.Candle) (supports, resistances []Level) {
	if len(candles) < 10 {
		return nil, nil
	}

	n := len(candles)
	start := 0
	if n > srLookback {
		start = n - srLookback
	}
	window := candles[start:]

	var resCandidates, supCandidates []float64

	for _, p := range findLocalHighs(window) {
		resCandidates = append(resCandidates, p.Price)
	}
	for _, p := range findLocalLows(window) {
		supCandidates = append(supCandidates, p.Price)
	}

	resistances = buildLevels(resCandidates, "Resistance", window)
	supports = buildLevels(supCandidates, "Support", window)
	return
}

func buildLevels(candidates []float64, levelType string, candles []store.Candle) []Level {
	if len(candidates) == 0 {
		return nil
	}

	type cluster struct {
		sum   float64
		count int
	}

	clusters := []cluster{{sum: candidates[0], count: 1}}
	for _, price := range candidates[1:] {
		merged := false
		for i := range clusters {
			center := clusters[i].sum / float64(clusters[i].count)
			if math.Abs(price-center)/center < mergeThresh {
				clusters[i].sum += price
				clusters[i].count++
				merged = true
				break
			}
		}
		if !merged {
			clusters = append(clusters, cluster{sum: price, count: 1})
		}
	}

	// 計算 Strength（觸碰次數比例）
	maxCount := 0
	for _, cl := range clusters {
		if cl.count > maxCount {
			maxCount = cl.count
		}
	}

	var levels []Level
	for _, cl := range clusters {
		centerPrice := cl.sum / float64(cl.count)
		strength := float64(cl.count) / float64(maxCount)
		levels = append(levels, Level{
			Price:    centerPrice,
			Strength: strength,
			Type:     levelType,
		})
	}

	// 按 Strength 降序排列，取前 maxLevels 個
	for i := 0; i < len(levels)-1; i++ {
		for j := i + 1; j < len(levels); j++ {
			if levels[j].Strength > levels[i].Strength {
				levels[i], levels[j] = levels[j], levels[i]
			}
		}
	}
	if len(levels) > maxLevels {
		levels = levels[:maxLevels]
	}
	return levels
}

package bouncereplay

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/trading/backend/internal/scheduler"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

// cutoffCandles 是引擎讀 K 棒的來源：只給到 cutoff（那一輪記錄的最後一根）為止。
// 只實作引擎與指標引擎會用到的 GetLatestN；其他方法⛔ 不該被呼叫（嵌入的 nil 介面會直接 panic，讓誤用現形）。
type cutoffCandles struct {
	store.CandleRepo
	all    map[string][]store.Candle
	cutoff time.Time
}

func (c *cutoffCandles) GetLatestN(_ context.Context, symbol, timeframe string, n int) ([]store.Candle, error) {
	if timeframe != Timeframe {
		return nil, fmt.Errorf("bouncereplay: unexpected timeframe %q", timeframe)
	}
	return latestUpTo(c.all[symbol], c.cutoff, n), nil
}

// latestUpTo 回 cs（依時間升冪）裡 ts ≤ cutoff 的最後 n 根（升冪，跟 candleRepo.GetLatestN 相同）。
func latestUpTo(cs []store.Candle, cutoff time.Time, n int) []store.Candle {
	end := sort.Search(len(cs), func(i int) bool { return cs[i].Timestamp.After(cutoff) })
	start := end - n
	if start < 0 {
		start = 0
	}
	return append([]store.Candle(nil), cs[start:end]...)
}

// fingerprintAt 用 scheduler 同一個算法，算「最後一根是 cutoff」時評估會讀到的 K 棒指紋。
func fingerprintAt(cs []store.Candle, cutoff time.Time) string {
	return scheduler.CandleFingerprint(latestUpTo(cs, cutoff, Lookback))
}

func withhold(e Eval) bool { return e.DataAt.Sub(e.LastBarTS) > StaleAfter }

func day(t time.Time) string { return t.In(timeutil.TaipeiTZ).Format("2006-01-02") }

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// isDailyTS：日 K 的 ts 是台北的 00:00:00。
func isDailyTS(t time.Time) bool {
	h, m, s := t.In(timeutil.TaipeiTZ).Clock()
	return h == 0 && m == 0 && s == 0 && t.Nanosecond() == 0
}

// 外部訊號的歸因（第 7.3 節「重播工具的實作補充」第 3 點）。
const (
	OriginIntraday      = "intraday"       // 唯一配對到一次盤中評估
	OriginExternalDaily = "external_daily" // 日 K 的 ts、⛔ 沒有配對到盤中評估
	OriginUnknown       = "unknown"        // 盤中時間的 ts、⛔ 沒有唯一配對（可能是手動 1 分 K）
	OriginHistory       = "history"        // 窗口開始之前的起始歷史
)

type attribution struct {
	origin map[uint64]string       // signal id → 來源
	byEval map[evalKey]*LiveSignal // 盤中評估 → 它寫進 signals 的那一筆
	notes  []string                // 來源不明、多重配對、signal_sent 卻沒有配對到的
}

// attribute 把 live 的每一筆訊號歸到「盤中評估」或「外部」。唯一配對的條件：同一檔、ts＝那一列的 last_bar_ts、
// created_at 落在 data_at～eval_at，而且那一列是 signal_sent 或 degraded（degraded 也可能已寫入）。
func attribute(in *Input) attribution {
	a := attribution{origin: map[uint64]string{}, byEval: map[evalKey]*LiveSignal{}}
	bySym := map[string][]int{}
	for i, e := range in.Evals {
		if e.EvalStatus == "signal_sent" || e.EvalStatus == "degraded" {
			bySym[e.Symbol] = append(bySym[e.Symbol], i)
		}
	}
	claimed := map[evalKey][]*LiveSignal{}
	for i := range in.Signals {
		s := &in.Signals[i]
		if s.CreatedAt.Before(in.From) {
			a.origin[s.ID] = OriginHistory
			continue
		}
		var hits []int
		for _, ei := range bySym[s.Symbol] {
			e := in.Evals[ei]
			if s.TS.Equal(e.LastBarTS) && !s.CreatedAt.Before(e.DataAt) && !s.CreatedAt.After(e.EvalAt) {
				hits = append(hits, ei)
			}
		}
		switch {
		case len(hits) == 1:
			a.origin[s.ID] = OriginIntraday
			k := in.Evals[hits[0]].key()
			claimed[k] = append(claimed[k], s)
		case len(hits) == 0 && isDailyTS(s.TS):
			a.origin[s.ID] = OriginExternalDaily
		default:
			a.origin[s.ID] = OriginUnknown
			a.notes = append(a.notes, fmt.Sprintf("來源不明：signal id %d %s %s ts=%s created_at=%s（配對到 %d 次盤中評估）",
				s.ID, s.Symbol, s.SignalType, s.TS.Format(time.RFC3339), s.CreatedAt.Format(time.RFC3339Nano), len(hits)))
		}
	}
	for k, ss := range claimed {
		if len(ss) == 1 {
			a.byEval[k] = ss[0]
			continue
		}
		// 同一次評估配對到多筆：⛔ 不是唯一配對，全部改列來源不明。
		for _, s := range ss {
			a.origin[s.ID] = OriginUnknown
		}
		a.notes = append(a.notes, fmt.Sprintf("同一次評估配對到 %d 筆訊號：run %d %s（改列來源不明）", len(ss), k.runID, k.symbol))
	}
	for _, e := range in.Evals {
		if e.EvalStatus == "signal_sent" && a.byEval[e.key()] == nil {
			a.notes = append(a.notes, fmt.Sprintf("signal_sent 卻沒有配對到 signals：run %d %s", e.RunID, e.Symbol))
		}
	}
	sort.Strings(a.notes)
	return a
}

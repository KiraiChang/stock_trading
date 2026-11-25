package bouncereplay

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/signal"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

// 補充重播（第 7.3 節「輸入」的補充）：11-17～11-20 ⛔ 沒有逐輪存證（T-088 之前），所以⛔ 不能依 live 實際評估的（輪次、檔）重播，
// 只能每 5 分鐘固定一輪（09:00～13:30，55 輪）、每一檔都評估。結果標「保真度較低」：⛔ 不逐筆對帳、⛔ 不作為裁決依據，
// 只用來看 A／B 的差異趨勢是否跟主要窗口一致。

const (
	// FixedRounds 是一天的固定輪數（09:00、09:05 … 13:30）。
	FixedRounds = 55
	// DefaultBarLag：每一輪只用 ts ≤ 輪次時間 − BarLag 的 1 分 K（預設 1 分鐘＝那一輪之前已經收盤的最後一根）。
	DefaultBarLag = time.Minute
)

// FixedRoundTimes 是某一天（台北日期 YYYY-MM-DD）的 55 個輪次時間。
func FixedRoundTimes(d string) ([]time.Time, error) {
	t0, err := time.ParseInLocation("2006-01-02 15:04", d+" 09:00", timeutil.TaipeiTZ)
	if err != nil {
		return nil, err
	}
	out := make([]time.Time, FixedRounds)
	for i := range out {
		out[i] = t0.Add(time.Duration(i) * 5 * time.Minute)
	}
	return out, nil
}

// lastBarUpTo 是 cs 裡 ts ≤ cutoff 的最後一根的時間（沒有就是零值）。
func lastBarUpTo(cs []store.Candle, cutoff time.Time) time.Time {
	end := sort.Search(len(cs), func(i int) bool { return cs[i].Timestamp.After(cutoff) })
	if end == 0 {
		return time.Time{}
	}
	return cs[end-1].Timestamp
}

// ParseWatchlist 讀交接的自選清單（watchlist.csv：表頭 symbol,added_at；added_at 是 PostgreSQL 輸出的 timestamptz）。
// ⛔ 不能是空的、代號⛔ 不能重複。
func ParseWatchlist(r io.Reader) ([]WatchEntry, error) {
	rows, err := csv.NewReader(r).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("自選清單：%w", err)
	}
	if len(rows) == 0 || len(rows[0]) != 2 || rows[0][0] != "symbol" || rows[0][1] != "added_at" {
		return nil, fmt.Errorf("自選清單的表頭要是 symbol,added_at")
	}
	var out []WatchEntry
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		sym := strings.TrimSpace(row[0])
		if sym == "" || seen[sym] {
			return nil, fmt.Errorf("自選清單的代號是空的或重複：%q", row[0])
		}
		seen[sym] = true
		var at time.Time
		for _, layout := range []string{"2006-01-02 15:04:05.999999-07", "2006-01-02 15:04:05.999999-07:00", time.RFC3339Nano} {
			if at, err = time.Parse(layout, strings.TrimSpace(row[1])); err == nil {
				break
			}
		}
		if err != nil {
			return nil, fmt.Errorf("自選清單 %s 的 added_at 格式不對：%q", sym, row[1])
		}
		out = append(out, WatchEntry{Symbol: sym, AddedAt: at})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("自選清單是空的")
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out, nil
}

// FixedMembers 是某一天的分母：自選清單裡台北時間當天 09:00 之前（含）已加入的標的；later 是之後才加入、當天⛔ 不納入的。
// 交接的清單是當下的快照：⛔ 沒有辦法還原交接前已經移除的標的（報告會註明）。
func FixedMembers(in *Input, d string) (members, later []string, err error) {
	t0, err := time.ParseInLocation("2006-01-02 15:04", d+" 09:00", timeutil.TaipeiTZ)
	if err != nil {
		return nil, nil, err
	}
	for _, w := range in.Watchlist {
		if w.AddedAt.After(t0) {
			later = append(later, w.Symbol)
		} else {
			members = append(members, w.Symbol)
		}
	}
	return members, later, nil
}

// barsOnDay 是 cs 裡台北日期 d 的根數。
func barsOnDay(cs []store.Candle, d string) int {
	n := 0
	for _, c := range cs {
		if day(c.Timestamp) == d {
			n++
		}
	}
	return n
}

// SynthesizeFixedEvals 依預定交易日與自選清單合成固定輪次的評估（依時間、再依代號排序）。
// 每天只評估當天的分母（FixedMembers）裡當天有 1 分 K 的檔；整日缺 1 分 K 的檔⛔ 不評估，由報告判缺資料。
// RunID 是合成的流水號（⛔ 不是 live 的 run_id）；DataAt＝EvalAt＝輪次時間；LastBarTS 是 ts ≤ 輪次 − barLag 的最後一根。
// ⛔ 沒有指紋、⛔ 沒有 live 的評估結果。
func SynthesizeFixedEvals(in *Input, days []string, barLag time.Duration) error {
	if len(in.Watchlist) == 0 {
		return fmt.Errorf("沒有自選清單（-watchlist）")
	}
	in.Evals = nil
	var id uint64
	for _, d := range days {
		rounds, err := FixedRoundTimes(d)
		if err != nil {
			return err
		}
		members, _, err := FixedMembers(in, d)
		if err != nil {
			return err
		}
		var syms []string
		for _, s := range members {
			if barsOnDay(in.Candles[s], d) > 0 {
				syms = append(syms, s)
			}
		}
		for _, t := range rounds {
			for _, s := range syms {
				id++
				in.Evals = append(in.Evals, Eval{RunID: id, Symbol: s, DataAt: t, EvalAt: t, LastBarTS: lastBarUpTo(in.Candles[s], t.Add(-barLag))})
			}
		}
	}
	return nil
}

// LoadFixed 讀補充重播的輸入：自選清單的每一檔（K 棒與 signals；暖身與起始歷史的範圍同 Load），
// 以及窗口內有 1 分 K、但不在清單裡的代號（in.Unlisted，⛔ 不納入）。
func LoadFixed(ctx context.Context, src *sqlx.DB, o LoadOptions, watch []WatchEntry) (*Input, error) {
	if len(watch) == 0 {
		return nil, fmt.Errorf("沒有自選清單（-watchlist）")
	}
	if o.WarmupDays <= 0 {
		o.WarmupDays = 4
	}
	if o.HistoryDays <= 0 {
		o.HistoryDays = 7
	}
	in := &Input{From: o.From, To: o.To, Candles: map[string][]store.Candle{}, Watchlist: watch}
	var have []string
	if err := src.SelectContext(ctx, &have, src.Rebind(`
		SELECT DISTINCT symbol FROM candles WHERE timeframe = ? AND ts >= ? AND ts < ? ORDER BY symbol`),
		Timeframe, o.From, o.To); err != nil {
		return nil, fmt.Errorf("load symbols: %w", err)
	}
	listed := map[string]bool{}
	syms := make([]string, 0, len(watch))
	for _, w := range watch {
		listed[w.Symbol] = true
		syms = append(syms, w.Symbol)
	}
	for _, s := range have {
		if !listed[s] {
			in.Unlisted = append(in.Unlisted, s)
		}
	}
	candles := store.NewCandleRepo(src)
	for _, sym := range syms {
		cs, err := candles.GetRange(ctx, sym, Timeframe, o.From.AddDate(0, 0, -o.WarmupDays), o.To)
		if err != nil {
			return nil, fmt.Errorf("load candles %s: %w", sym, err)
		}
		in.Candles[sym] = cs
	}
	q, args, err := sqlx.In(`
		SELECT id, symbol, signal_type, direction, price, volume, COALESCE(vol_ratio, 0) AS vol_ratio,
		       COALESCE(resistance, 0) AS resistance, COALESCE(support, 0) AS support, COALESCE(trend, '') AS trend,
		       COALESCE(note, '') AS note, strength, chip_signal, ts, created_at
		FROM signals
		WHERE symbol IN (?) AND created_at >= ? AND created_at < ?
		ORDER BY created_at, id`, syms, o.From.AddDate(0, 0, -o.HistoryDays), o.To)
	if err != nil {
		return nil, err
	}
	if err := src.SelectContext(ctx, &in.Signals, src.Rebind(q), args...); err != nil {
		return nil, fmt.Errorf("load signals: %w", err)
	}
	return in, nil
}

// fixedLiveIntraday：窗口內、盤中時間的 ts（⛔ 不是日 K 的 00:00）——live 的盤中送出，由重播自己產生，⛔ 不注入。
func fixedLiveIntraday(in *Input, s LiveSignal) bool {
	return !s.CreatedAt.Before(in.From) && !isDailyTS(s.TS)
}

// FixedRunner 跑一次補充重播。
type FixedRunner struct {
	in           *Input
	dbA, dbB     *sqlx.DB
	repoA, repoB store.SignalRepo
	candles      *cutoffCandles
	engA, engB   *signal.Engine
	clock        time.Time
	pending      []LiveSignal
	Records      []Record
	Notes        []string
}

// NewFixedRunner：in.Evals 要先由 SynthesizeFixedEvals 合成；dbA、dbB 是兩個已經 migrate 好、signals 是空的重播庫。
func NewFixedRunner(in *Input, dbA, dbB *sqlx.DB, log *zap.Logger) *FixedRunner {
	r := &FixedRunner{in: in, dbA: dbA, dbB: dbB, repoA: store.NewSignalRepo(dbA), repoB: store.NewSignalRepo(dbB),
		candles: &cutoffCandles{all: in.Candles}}
	clock := func() time.Time { return r.clock }
	r.engA = newEngine(r.candles, r.repoA, store.NewIndicatorRepo(dbA), signal.DedupPriceKey, clock, log)
	r.engB = newEngine(r.candles, r.repoB, store.NewIndicatorRepo(dbB), signal.DedupIntradayBounceBySymbol, clock, log)
	skipped := 0
	for _, s := range in.Signals {
		if fixedLiveIntraday(in, s) {
			skipped++
			continue
		}
		// 起始歷史（窗口之前的全部）與窗口內的日 K 訊號照 created_at 注入兩庫。
		r.pending = append(r.pending, s)
	}
	r.Notes = append(r.Notes, fmt.Sprintf("live 的訊號 %d 筆：注入 %d 筆（起始歷史與窗口內的日 K）、窗口內盤中時間的 %d 筆⛔ 不注入（由重播自己產生）",
		len(in.Signals), len(r.pending), skipped))
	return r
}

func (r *FixedRunner) inject(ctx context.Context, at time.Time) error {
	for len(r.pending) > 0 && !r.pending[0].CreatedAt.After(at) {
		s := r.pending[0]
		r.pending = r.pending[1:]
		for _, db := range []*sqlx.DB{r.dbA, r.dbB} {
			if err := insertRaw(ctx, db, s); err != nil {
				return fmt.Errorf("inject signal %d: %w", s.ID, err)
			}
		}
	}
	return nil
}

// Run 依輪次時間的順序評估每一檔。⛔ 不分群、⛔ 不對帳（沒有 live 的逐輪結果可比）。
func (r *FixedRunner) Run(ctx context.Context) error {
	for _, e := range r.in.Evals {
		if err := r.inject(ctx, e.EvalAt); err != nil {
			return err
		}
		rec := Record{Eval: e, Day: day(e.DataAt), Withhold: e.LastBarTS.IsZero() || withhold(e)}
		histA, errHA := r.repoA.GetBySymbol(ctx, e.Symbol, 5*RecentLimit)
		histB, errHB := r.repoB.GetBySymbol(ctx, e.Symbol, 5*RecentLimit)
		r.candles.cutoff = e.LastBarTS
		r.clock = e.EvalAt
		opts := signal.EvaluateOptions{Withhold: rec.Withhold}
		resA, errA := r.engA.EvaluateWithOptions(ctx, e.Symbol, Timeframe, opts)
		rec.A = statusOf(resA, errA)
		resB, errB := r.engB.EvaluateWithOptions(ctx, e.Symbol, Timeframe, opts)
		rec.B = statusOf(resB, errB)
		if errHA != nil {
			rec.A.ReadErr = "判重歷史：" + errHA.Error()
		}
		if errHB != nil {
			rec.B.ReadErr = "判重歷史：" + errHB.Error()
		}
		if needsAnchor(rec.A, rec.B) {
			anc, err := r.repoB.LatestBySymbolTypeTimeframe(ctx, e.Symbol, "SUPPORT_BOUNCE", Timeframe)
			switch {
			case err != nil:
				if rec.B.ReadErr != "" {
					rec.B.ReadErr += "；"
				}
				rec.B.ReadErr += "錨點：" + err.Error()
			case anc != nil:
				v := anc.Support
				rec.BAnchorSupport = &v
			}
		}
		markNonTarget(&rec, histA, histB, errHA == nil && errHB == nil)
		r.Records = append(r.Records, rec)
	}
	return nil
}

// FixedSummary 是補充重播的結論（⛔ 沒有通過／不通過的裁決，只有完整性）：缺資料或比較不完整都是 Failures（結束碼 3）。
type FixedSummary struct {
	OK         bool
	Failures   []string
	Incomplete []string // 比較不完整的日期（當天的 A／B 差異⛔ 不能用來判讀）
}

// liveDay 是 live 當天實際的盤中 SUPPORT_BOUNCE（趨勢比對用）。
type liveDay struct{ sent, shortPairs, sameSupport int }

// liveBounces 依日期（與 "合計"）統計 live 的盤中 SUPPORT_BOUNCE；keep 決定哪些算進來（例如當天的分母、某一檔）。
func liveBounces(in *Input, keep func(LiveSignal) bool) map[string]*liveDay {
	out := map[string]*liveDay{}
	var ss []LiveSignal
	for _, s := range in.Signals {
		if fixedLiveIntraday(in, s) && s.CreatedAt.Before(in.To) && s.SignalType == "SUPPORT_BOUNCE" && keep(s) {
			ss = append(ss, s)
		}
	}
	sort.SliceStable(ss, func(i, j int) bool {
		if ss[i].Symbol != ss[j].Symbol {
			return ss[i].Symbol < ss[j].Symbol
		}
		return ss[i].CreatedAt.Before(ss[j].CreatedAt)
	})
	get := func(d string) []*liveDay {
		for _, k := range []string{d, "合計"} {
			if out[k] == nil {
				out[k] = &liveDay{}
			}
		}
		return []*liveDay{out[d], out["合計"]}
	}
	for i, s := range ss {
		d := day(s.CreatedAt)
		for _, x := range get(d) {
			x.sent++
		}
		if i > 0 {
			p := ss[i-1]
			if p.Symbol == s.Symbol && day(p.CreatedAt) == d && s.CreatedAt.Sub(p.CreatedAt) < Cooldown {
				for _, x := range get(d) {
					x.shortPairs++
					if round2(p.Support) == round2(s.Support) {
						x.sameSupport++
					}
				}
			}
		}
	}
	return out
}

// sideHealth 是一種規則在一天裡的評估狀況。
type sideHealth struct{ insufficient, failed, degraded, readErr, withheld int }

func (h *sideHealth) add(o Outcome) {
	if o.Status == "withheld_stale" {
		h.withheld++
	}
	switch {
	case o.Insufficient:
		h.insufficient++
	case o.Status == "evaluate_failed":
		h.failed++
	case o.Status == "degraded":
		h.degraded++
	}
	if o.ReadErr != "" {
		h.readErr++
	}
}

func (h sideHealth) bad() bool { return h.failed+h.degraded+h.readErr > 0 }

// fixedDay 是一天的分母、資料與評估狀況。
type fixedDay struct {
	members, later, missing   []string
	bars                      map[string]int
	evals, warmShort, delayed int
	warmBySym                 map[string]int
	a, b                      sideHealth
	insufMismatch             int
	insufExample              string
}

func (fd *fixedDay) reasons() []string {
	var out []string
	for _, x := range []struct {
		name string
		h    sideHealth
	}{{"A", fd.a}, {"B", fd.b}} {
		if x.h.bad() {
			out = append(out, fmt.Sprintf("%s 評估失敗 %d、降級 %d、讀取失敗 %d", x.name, x.h.failed, x.h.degraded, x.h.readErr))
		}
	}
	if fd.insufMismatch > 0 {
		out = append(out, fmt.Sprintf("行情不足 A／B 不一致 %d 筆（例：%s）", fd.insufMismatch, fd.insufExample))
	}
	return out
}

func dash(xs []string) string {
	if len(xs) == 0 {
		return "—"
	}
	return strings.Join(xs, "、")
}

// WriteFixedReport 輸出補充重播的報告。分母是自選清單（每天台北 09:00 之前已加入的）：
//   - 缺資料：分母裡任一檔當天整日沒有 1 分 K（或分母是空的）。
//   - 比較不完整：A 或 B 有任何一筆評估失敗（行情不足除外）、降級或重播庫讀取失敗，或行情不足在同一輪同一檔 A、B ⛔ 不一致。
//
// 兩者與自我檢查做不了（selfCheckFails）都列進 Failures（結束碼 3），報告照樣產出。
func WriteFixedReport(w io.Writer, in *Input, recs []Record, expected []string, barLag time.Duration, notes, selfCheck, selfCheckFails []string) FixedSummary {
	fmt.Fprintf(w, "# I-140 補充重播（固定每 5 分鐘一輪）\n\n")
	fmt.Fprintf(w, "⚠️ **保真度較低**：這段窗口⛔ 沒有逐輪存證，輪次是合成的（每天 %d 輪、分母裡的每一檔都評估，只用 ts ≤ 輪次 − %v 的 1 分 K），"+
		"⛔ 不逐筆對帳、⛔ 不作為裁決依據，只用來看 A／B 的差異趨勢是否跟主要窗口一致；是否採用 B 仍以有逐輪存證的主要窗口（11-23～11-27）為準。"+
		"live 當時是 T-088 之前的版本（例如 09:00 用前一日的 K 棒），A 本來就⛔ 不會跟 live 相同。\n\n", FixedRounds, barLag)
	for _, x := range selfCheck {
		fmt.Fprintf(w, "* 自我檢查：%s\n", x)
	}
	for _, x := range selfCheckFails {
		fmt.Fprintf(w, "* ⚠️ %s\n", x)
	}
	for _, x := range notes {
		fmt.Fprintf(w, "* %s\n", x)
	}
	fmt.Fprintf(w, "* 自選清單 %d 檔（交接當下的快照）：每天的分母是台北時間當天 09:00 之前已加入的標的，之後才加入的當天⛔ 不納入；"+
		"⛔ 沒有辦法還原交接前已經移除的標的（那些標的⛔ 不在分母，也⛔ 不在 live 的比對數字裡）。\n", len(in.Watchlist))
	fmt.Fprintf(w, "* 窗口內有 1 分 K、但不在自選清單的代號（⛔ 不納入）：%s\n", dash(in.Unlisted))

	sum := FixedSummary{OK: true}
	fail := func(f string) { sum.OK = false; sum.Failures = append(sum.Failures, f) }
	for _, f := range selfCheckFails {
		fail(f)
	}
	if len(in.Watchlist) == 0 {
		fail("沒有自選清單")
	}
	if len(expected) == 0 {
		fail("預定交易日清單是空的")
	}

	days := map[string]*fixedDay{}
	member := map[string]map[string]bool{}
	for _, d := range expected {
		fd := &fixedDay{bars: map[string]int{}, warmBySym: map[string]int{}}
		days[d] = fd
		var err error
		if fd.members, fd.later, err = FixedMembers(in, d); err != nil {
			fail(d + "：" + err.Error())
		}
		member[d] = map[string]bool{}
		for _, s := range fd.members {
			member[d][s] = true
			fd.bars[s] = barsOnDay(in.Candles[s], d)
			if fd.bars[s] == 0 {
				fd.missing = append(fd.missing, s)
			}
		}
	}
	for _, r := range recs {
		fd := days[r.Day]
		if fd == nil {
			continue
		}
		fd.evals++
		if len(latestUpTo(in.Candles[r.Symbol], r.LastBarTS, Lookback)) < Lookback {
			fd.warmShort++
			fd.warmBySym[r.Symbol]++
		}
		if r.Withhold {
			fd.delayed++
		}
		fd.a.add(r.A)
		fd.b.add(r.B)
		if r.A.Insufficient != r.B.Insufficient {
			fd.insufMismatch++
			if fd.insufExample == "" {
				fd.insufExample = fmt.Sprintf("%s %s A=%v B=%v", r.Symbol, r.DataAt.In(timeutil.TaipeiTZ).Format("15:04"), r.A.Insufficient, r.B.Insufficient)
			}
		}
	}

	fmt.Fprintf(w, "\n## 分母與判讀狀態\n\n| 日期 | 有當天 1 分 K／分母 | 缺 1 分 K | 後來加入（⛔ 不納入） | 狀態 |\n|---|---|---|---|---|\n")
	for _, d := range expected {
		fd := days[d]
		var st []string
		if len(fd.members) == 0 {
			fail(d + " 缺資料：自選清單沒有當天 09:00 之前加入的標的")
			st = append(st, "⚠️ 缺資料（分母是空的）")
		}
		if len(fd.missing) > 0 {
			fail(fmt.Sprintf("%s 缺資料：自選清單 %d 檔整日沒有 1 分 K（%s）", d, len(fd.missing), dash(fd.missing)))
			st = append(st, "⚠️ 缺資料")
		}
		if rs := fd.reasons(); len(rs) > 0 {
			fail(d + " 比較不完整：" + strings.Join(rs, "；"))
			sum.Incomplete = append(sum.Incomplete, d)
			st = append(st, "⚠️ 比較不完整（"+strings.Join(rs, "；")+"）")
		}
		if len(st) == 0 {
			st = append(st, "可比較")
		}
		fmt.Fprintf(w, "| %s | %d／%d | %s | %s | %s |\n", d, len(fd.members)-len(fd.missing), len(fd.members), dash(fd.missing), dash(fd.later), strings.Join(st, "；"))
	}

	fmt.Fprintf(w, "\n## 評估狀況（A／B 分列）\n\n行情不足是 ErrInsufficientCandles；暖身不足是那一輪讀得到的 K 棒少於 %d 根；資料延遲輪數是最後一根距輪次超過 %v 的（輪次、檔），"+
		"A、B 共用同一份輸入；實際扣住是各規則評估結果為 withheld_stale 的筆數（那一輪⛔ 沒有候選、或先被判重擋下的⛔ 不算）。"+
		"行情不足、暖身不足、資料延遲分開統計；評估失敗⛔ 不含行情不足。\n\n", Lookback, StaleAfter)
	fmt.Fprintf(w, "| 日期 | 評估 | 暖身不足 | 資料延遲輪數 | 實際扣住 A／B | 行情不足 A／B | 評估失敗 A／B | 降級 A／B | 讀取失敗 A／B | 行情不足 A／B 不一致 |\n|---|---|---|---|---|---|---|---|---|---|\n")
	for _, d := range expected {
		fd := days[d]
		fmt.Fprintf(w, "| %s | %d | %d | %d | %d／%d | %d／%d | %d／%d | %d／%d | %d／%d | %d |\n", d, fd.evals, fd.warmShort, fd.delayed, fd.a.withheld, fd.b.withheld,
			fd.a.insufficient, fd.b.insufficient, fd.a.failed, fd.b.failed, fd.a.degraded, fd.b.degraded, fd.a.readErr, fd.b.readErr, fd.insufMismatch)
	}

	writeMetrics(w, computeMetrics(recs, "全部合成的評估（保真度較低）", func(Record) bool { return true }), expected)
	if len(sum.Incomplete) > 0 {
		fmt.Fprintf(w, "* ⚠️ 比較不完整的日期：%s——當天的 A／B 送出數照列，但差異⛔ 不能用來判讀（可能是執行故障，⛔ 不是 B 減少提醒）。\n", dash(sum.Incomplete))
	}
	fmt.Fprintf(w, "* 缺 1 分 K 的標的⛔ 不在 A／B 的數字裡；live 的數字有算進去（見下表）。\n")

	live := liveBounces(in, func(s LiveSignal) bool { return member[day(s.CreatedAt)][s.Symbol] })
	fmt.Fprintf(w, "\n## live 當天實際的盤中 SUPPORT_BOUNCE（趨勢比對；分母內的全部標的，含缺 1 分 K 的）\n\n| 日期 | live 送出 | live 短時間重複（同支撐） |\n|---|---|---|\n")
	for _, d := range append(append([]string{}, expected...), "合計") {
		x := live[d]
		if x == nil {
			x = &liveDay{}
		}
		fmt.Fprintf(w, "| %s | %d | %d（%d） |\n", d, x.sent, x.shortPairs, x.sameSupport)
	}

	fmt.Fprintf(w, "\n## 每日每檔\n\n送出與短時間重複只算 SUPPORT_BOUNCE。\n\n| 日期 | 代號 | 當天 1 分 K（根） | 暖身不足輪數 | A 送出（短時間重複） | B 送出（短時間重複） | live 送出（短時間重複） |\n|---|---|---|---|---|---|---|\n")
	for _, d := range expected {
		fd := days[d]
		for _, s := range fd.members {
			sym, dd := s, d
			lv := liveBounces(in, func(x LiveSignal) bool { return x.Symbol == sym })[dd]
			if lv == nil {
				lv = &liveDay{}
			}
			ab := "— | —"
			if fd.bars[s] > 0 {
				m := computeMetrics(recs, "", func(r Record) bool { return r.Symbol == sym && r.Day == dd })
				a, b := m.A[dd], m.B[dd]
				if a == nil {
					a = &policyStats{}
				}
				if b == nil {
					b = &policyStats{}
				}
				ab = fmt.Sprintf("%d（%d） | %d（%d）", a.Sent, a.ShortPairs, b.Sent, b.ShortPairs)
			}
			bars := fmt.Sprint(fd.bars[s])
			if fd.bars[s] == 0 {
				bars = "⛔ 沒有（缺資料）"
			}
			fmt.Fprintf(w, "| %s | %s | %s | %d | %s | %d（%d） |\n", d, s, bars, fd.warmBySym[s], ab, lv.sent, lv.shortPairs)
		}
	}

	fmt.Fprintf(w, "\n## 完整性\n\n")
	if sum.OK {
		fmt.Fprintf(w, "✅ 預定交易日的每一天，分母裡的每一檔都有 1 分 K，A、B 都⛔ 沒有評估失敗、降級或讀取失敗（⛔ 沒有通過／不通過的裁決）。\n")
	} else {
		for _, f := range sum.Failures {
			fmt.Fprintf(w, "* ⚠️ %s\n", f)
		}
	}
	return sum
}

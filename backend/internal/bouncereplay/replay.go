package bouncereplay

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/indicator"
	"github.com/trading/backend/internal/signal"
	"github.com/trading/backend/internal/store"
)

// noChip：籌碼分數一律「查不到」（第 7.3 節實作補充第 2 點）。籌碼加權只改 strength／note／chip_signal，⛔ 不影響判重欄位。
type noChip struct{ store.ChipScoreRepo }

func (noChip) GetLatest(context.Context, string) (*store.ChipScore, error) { return nil, sql.ErrNoRows }

// newEngine 建一個重播用的 Engine：Redis 停用、時鐘由呼叫端控制、**設定空的廣播**（實作補充第 1 點：
// live 一定有廣播；Redis 停用又沒有廣播時，引擎會把送出後的本機保留鎖釋放掉）。
func newEngine(candles store.CandleRepo, signals store.SignalRepo, ind store.IndicatorRepo, policy signal.DedupPolicy,
	clock func() time.Time, log *zap.Logger) *signal.Engine {
	redis := &store.RedisClient{} // 零值＝停用
	ie := indicator.NewEngine(candles, ind, redis, log)
	e := signal.NewEngine(candles, signals, redis, ie, noChip{}, log)
	e.SetDedupPolicy(policy)
	e.SetClockForTest(clock)
	e.BroadcastFn = func(string, *store.Signal) {}
	return e
}

func statusOf(res *signal.EvaluateResult, err error) Outcome {
	o := Outcome{}
	if err != nil {
		o.Status, o.Err = "evaluate_failed", err.Error()
		o.Insufficient = errors.Is(err, indicator.ErrInsufficientCandles)
		return o
	}
	if res == nil {
		o.Status = "no_signal"
		return o
	}
	o.Candidate, o.SuppressedBy, o.Persisted = res.Candidate, string(res.SuppressedBy), res.DBPersisted
	if res.Candidate != nil {
		o.Key = signal.PriceKeyIdentity(res.Candidate)
	}
	switch {
	case res.Degraded:
		o.Status = "degraded"
	case res.Withheld:
		o.Status = "withheld_stale"
	case res.Suppressed:
		o.Status = "signal_suppressed"
	case res.SignalGenerated:
		o.Status = "signal_sent"
	default:
		o.Status = "no_signal"
	}
	if res.SignalGenerated {
		o.Sent = res.Signal
	}
	return o
}

// insertRaw 把 live 的一筆訊號照原樣注入重播庫：created_at 保留原始精度（⛔ 不經過 repo 的截秒）、timeframe 留 NULL。
func insertRaw(ctx context.Context, db *sqlx.DB, s LiveSignal) error {
	_, err := db.ExecContext(ctx, db.Rebind(`INSERT INTO signals
		(symbol, signal_type, direction, price, volume, vol_ratio, resistance, support, trend, note, strength, chip_signal, ts, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		s.Symbol, s.SignalType, s.Direction, s.Price, s.Volume, s.VolRatio, s.Resistance, s.Support, s.Trend, s.Note,
		s.Strength, s.ChipSignal, s.TS, s.CreatedAt)
	return err
}

// histRow 是 DB 判重歷史逐筆比對的欄位（第 7.3 節：signal_type、direction、resistance、support（兩位小數）、ts）。
type histRow struct {
	typ, dir  string
	res, sup  float64
	ts        int64
	createdAt time.Time
}

func (h histRow) id() string {
	return fmt.Sprintf("%s|%s|%.2f|%.2f|%d", h.typ, h.dir, h.res, h.sup, h.ts)
}

func normalize(rows []histRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.id()
	}
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return out
}

// hold 是「分歧或未確認之後，滿 15 分鐘才重新核對」的狀態。
type hold struct {
	until  time.Time
	group  string
	reason string
}

type liveSend struct {
	createdAt time.Time
	key       evalKey // 盤中送出：配對到的評估；外部送出：零值
	external  bool
}

// Runner 跑一次完整的 A／B 重播。
type Runner struct {
	in           *Input
	attr         attribution
	dbA, dbB     *sqlx.DB
	repoA, repoB store.SignalRepo
	candles      *cutoffCandles
	engA, engB   *signal.Engine
	clock        time.Time
	log          *zap.Logger

	pending   []LiveSignal // 要注入的外部訊號（依 created_at）
	sends     map[string][]liveSend
	holds     map[string]hold
	unconfirm map[string]hold  // 來源不明、degraded 未落盤等造成的未確認時段
	aSentOK   map[evalKey]bool // 重播 A 在這一次送出、指紋相同、跟 live 的送出一致
	lastSend  map[string]Eval  // 重播 A 每個保留鍵最後一次送出的評估
	recIdx    map[evalKey]int  // 評估 → Records 的位置
	Records   []Record
	Notes     []string
}

// NewRunner 準備重播。dbA、dbB 是兩個已經 migrate 好、signals 是空的重播庫。
func NewRunner(in *Input, dbA, dbB *sqlx.DB, log *zap.Logger) *Runner {
	// 依 eval_at 的全域順序重播（Load 已排好；測試或其他呼叫端給的輸入也照這個順序）。
	sort.SliceStable(in.Evals, func(i, j int) bool { return in.Evals[i].EvalAt.Before(in.Evals[j].EvalAt) })
	r := &Runner{
		in: in, attr: attribute(in), dbA: dbA, dbB: dbB,
		repoA: store.NewSignalRepo(dbA), repoB: store.NewSignalRepo(dbB),
		candles: &cutoffCandles{all: in.Candles}, log: log,
		sends: map[string][]liveSend{}, holds: map[string]hold{}, unconfirm: map[string]hold{},
		aSentOK: map[evalKey]bool{}, lastSend: map[string]Eval{}, recIdx: map[evalKey]int{},
	}
	clock := func() time.Time { return r.clock }
	r.engA = newEngine(r.candles, r.repoA, store.NewIndicatorRepo(dbA), signal.DedupPriceKey, clock, log)
	r.engB = newEngine(r.candles, r.repoB, store.NewIndicatorRepo(dbB), signal.DedupIntradayBounceBySymbol, clock, log)
	for _, s := range in.Signals {
		switch r.attr.origin[s.ID] {
		case OriginIntraday:
		default:
			r.pending = append(r.pending, s)
		}
		// 起始歷史、外部日 K、來源不明的送出也是 live 的送出：保留鎖驗不了（實作補充第 3 點）。
		if o := r.attr.origin[s.ID]; o == OriginHistory || o == OriginExternalDaily || o == OriginUnknown {
			r.sends[s.Symbol] = append(r.sends[s.Symbol], liveSend{createdAt: s.CreatedAt, external: true})
		}
	}
	for k, s := range r.attr.byEval {
		r.sends[k.symbol] = append(r.sends[k.symbol], liveSend{createdAt: s.CreatedAt, key: k})
	}
	for sym := range r.sends {
		ss := r.sends[sym]
		sort.Slice(ss, func(i, j int) bool { return ss[i].createdAt.Before(ss[j].createdAt) })
	}
	r.Notes = append(r.Notes, r.attr.notes...)
	return r
}

// inject 把 created_at 不晚於 at 的外部訊號寫進 A、B 兩庫。at 是引擎這一次評估用的時鐘（eval_at）：
// 注入與評估走同一條時間線（複審 2026-11-24）。data_at～eval_at 之間的寫入照樣標「歷史未確認」（classify）。
func (r *Runner) inject(ctx context.Context, at time.Time) error {
	for len(r.pending) > 0 && !r.pending[0].CreatedAt.After(at) {
		s := r.pending[0]
		r.pending = r.pending[1:]
		for _, db := range []*sqlx.DB{r.dbA, r.dbB} {
			if err := insertRaw(ctx, db, s); err != nil {
				return fmt.Errorf("inject signal %d: %w", s.ID, err)
			}
		}
		if r.attr.origin[s.ID] == OriginUnknown {
			r.extendUnconfirm(s.Symbol, s.CreatedAt.Add(Cooldown), "來源不明的訊號（可能是手動 1 分 K）")
		}
	}
	return nil
}

func (r *Runner) extendUnconfirm(sym string, until time.Time, reason string) {
	if h := r.unconfirm[sym]; until.After(h.until) {
		r.unconfirm[sym] = hold{until: until, group: GroupUnconfirmed, reason: reason}
	}
}

func (r *Runner) extendHold(sym string, until time.Time, group, reason string) {
	h := r.holds[sym]
	if until.After(h.until) || (group == GroupDiverged && h.group == GroupUnconfirmed && until.Equal(h.until)) {
		r.holds[sym] = hold{until: until, group: group, reason: reason}
	}
}

// Run 依 eval_at 的順序重播每一次 live 的評估。
func (r *Runner) Run(ctx context.Context) error {
	for _, e := range r.in.Evals {
		if err := r.inject(ctx, e.EvalAt); err != nil {
			return err
		}
		rec := Record{Eval: e, Day: day(e.DataAt), Withhold: withhold(e), LiveSent: r.attr.byEval[e.key()]}
		r.classify(ctx, &rec)
		// 非目標訊號的回歸要能證實「最近 20 筆」的邊界：評估前先記下兩庫較長的歷史。
		histA, errHA := r.repoA.GetBySymbol(ctx, e.Symbol, 5*RecentLimit)
		histB, errHB := r.repoB.GetBySymbol(ctx, e.Symbol, 5*RecentLimit)

		r.candles.cutoff = e.LastBarTS
		r.clock = e.EvalAt
		opts := signal.EvaluateOptions{Withhold: rec.Withhold}
		resA, errA := r.engA.EvaluateWithOptions(ctx, e.Symbol, Timeframe, opts)
		rec.A = statusOf(resA, errA)
		resB, errB := r.engB.EvaluateWithOptions(ctx, e.Symbol, Timeframe, opts)
		rec.B = statusOf(resB, errB)
		if needsAnchor(rec.A, rec.B) {
			if anc, err := r.repoB.LatestBySymbolTypeTimeframe(ctx, e.Symbol, "SUPPORT_BOUNCE", Timeframe); err == nil && anc != nil {
				v := anc.Support
				rec.BAnchorSupport = &v
			}
		}

		markNonTarget(&rec, histA, histB, errHA == nil && errHB == nil)

		fpSame := e.Fingerprint != "" && fingerprintAt(r.in.Candles[e.Symbol], e.LastBarTS) == e.Fingerprint
		if fpSame && rec.A.Status == "signal_sent" && rec.LiveSent != nil && sameSend(rec.A.Sent, rec.LiveSent) == "" {
			r.aSentOK[e.key()] = true
		}
		if rec.A.Status == "signal_sent" {
			r.lastSend[rec.A.Key] = e
		}
		if rec.Group == GroupReconcilable {
			r.reconcile(&rec)
		}
		// live 的 degraded 沒有落盤、signal_sent 卻沒有配對到：可能留下保留鎖 → 之後 15 分鐘未確認。
		if (e.EvalStatus == "degraded" || e.EvalStatus == "signal_sent") && rec.LiveSent == nil {
			r.extendUnconfirm(e.Symbol, e.EvalAt.Add(Cooldown+lockMargin), "live 的 "+e.EvalStatus+" 沒有配對到 signals（可能留下保留鎖）")
		}
		r.recIdx[e.key()] = len(r.Records)
		r.Records = append(r.Records, rec)
	}
	return nil
}

// needsAnchor：B 抑制了 SUPPORT_BOUNCE 的候選、A 這一筆成功寫進 signals（降級但寫入成功的也算）——要查 B 當時的錨點支撐
// （第 7.3 節衡量口徑 3「被合併的不同支撐」；跟送出數同一個依據）。
func needsAnchor(a, b Outcome) bool {
	return b.Status == "signal_suppressed" && b.Candidate != nil && b.Candidate.SignalType == "SUPPORT_BOUNCE" && a.Sent != nil && a.Persisted
}

// isNonTarget：這一邊有非目標訊號（⛔ 不是 SUPPORT_BOUNCE）的候選或送出。
func isNonTarget(o Outcome) bool {
	return (o.Candidate != nil && o.Candidate.SignalType != "SUPPORT_BOUNCE") || (o.Sent != nil && o.Sent.SignalType != "SUPPORT_BOUNCE")
}

func nonTargetType(a, b Outcome) string {
	for _, o := range []Outcome{a, b} {
		if o.Sent != nil {
			return o.Sent.SignalType
		}
		if o.Candidate != nil {
			return o.Candidate.SignalType
		}
	}
	return "?"
}

// markNonTarget：A、B **任一邊**有非目標訊號就比對（複審 2026-11-24：⛔ 不能只從 A 那一邊啟動——A 沒有候選、B 卻送出也要記）；
// 有差異時只在逐筆證實來自最近 20 筆的邊界才算已解釋。重播這一側的降級、評估失敗或產生了卻沒寫入，另記成執行異常（⛔ 不算差異）。
func markNonTarget(rec *Record, histA, histB []store.Signal, histOK bool) {
	if !isNonTarget(rec.A) && !isNonTarget(rec.B) {
		return
	}
	if d := nonTargetDiff(rec.A, rec.B); d != "" {
		rec.NonTargetDiff = d
		rec.NonTargetExplained = histOK && explainRecent20(rec.A, rec.B, histA, histB)
	}
	rec.NonTargetAnomaly = execAnomaly(rec.A, rec.B)
}

// written：這一邊成功寫進 signals 的那一筆（沒有就是 nil）。
func written(o Outcome) *store.Signal {
	if o.Sent != nil && o.Persisted {
		return o.Sent
	}
	return nil
}

// nonTargetDiff：同一次評估、非目標訊號在 A、B **成功寫進 signals** 的紀錄不同——只有一邊寫入，或兩邊寫入的內容不同
// （複審 2026-11-25：跟摘要的送出集合同一個依據；執行狀態的差異——例如 A degraded、B signal_sent 但兩邊寫入相同——⛔ 不算差異，另列執行異常）。
func nonTargetDiff(a, b Outcome) string {
	wa, wb := written(a), written(b)
	if (wa == nil) != (wb == nil) {
		side := "A"
		if wb != nil {
			side = "B"
		}
		return fmt.Sprintf("只有 %s 寫入 signals：A %s（%s）、B %s（%s）", side, a.Status, a.SuppressedBy, b.Status, b.SuppressedBy)
	}
	if wa != nil {
		ls := &LiveSignal{Symbol: wb.Symbol, SignalType: wb.SignalType, Direction: wb.Direction, Price: wb.Price,
			Support: wb.Support, Resistance: wb.Resistance, TS: wb.Timestamp}
		if d := sameSend(wa, ls); d != "" {
			return "寫入的內容不同：" + d
		}
	}
	return ""
}

// execAnomaly：重播這一側的執行異常（降級、評估失敗〔行情不足除外〕、產生了訊號卻沒寫入）。
func execAnomaly(a, b Outcome) string {
	var out []string
	for _, x := range []struct {
		name string
		o    Outcome
	}{{"A", a}, {"B", b}} {
		switch {
		case x.o.Status == "degraded" || (x.o.Status == "evaluate_failed" && !x.o.Insufficient):
			out = append(out, fmt.Sprintf("%s %s（寫入 %v）", x.name, x.o.Status, written(x.o) != nil))
		case x.o.Sent != nil && !x.o.Persisted:
			out = append(out, fmt.Sprintf("%s 產生了訊號卻沒寫入", x.name))
		}
	}
	return strings.Join(out, "、")
}

// explainRecent20 逐筆證實差異來自「最近 20 筆」的邊界（7.2 的那一種原因）：一邊被 DB 判重擋下、另一邊寫入 signals（降級但寫入成功的也算）；
// 擋下的那一邊，讓它擋下的那一列（同一個身分、K 棒時間 15 分鐘內）在最近 20 筆之內；寫入的那一邊也有同一列，但落在第 20 筆之外，
// 而且寫入方的前 20 筆⛔ 沒有任何一列會擋下這個候選。
func explainRecent20(a, b Outcome, histA, histB []store.Signal) bool {
	sup, sent, hs, ht := a, b, histA, histB
	if b.Status == "signal_suppressed" {
		sup, sent, hs, ht = b, a, histB, histA
	}
	if sup.Status != "signal_suppressed" || sup.SuppressedBy != string(signal.SuppressedByDB) || written(sent) == nil ||
		sup.Candidate == nil || sent.Candidate == nil || sup.Key != sent.Key {
		return false
	}
	c := sup.Candidate
	blocker := func(rows []store.Signal) []int {
		var idx []int
		for i := range rows {
			d := c.Timestamp.Sub(rows[i].Timestamp)
			if signal.PriceKeyIdentity(&rows[i]) == sup.Key && d >= 0 && d < Cooldown {
				idx = append(idx, i)
			}
		}
		return idx
	}
	// 寫入方的前 20 筆⛔ 不能有任何一筆會擋下這個候選：有的話，正常的 DB 判重本來就會擋下，這次寫入可能是判重查詢失敗後放行的（複審 2026-11-25）。
	bt := blocker(ht)
	for _, j := range bt {
		if j < RecentLimit {
			return false
		}
	}
	for _, i := range blocker(hs) {
		if i >= RecentLimit {
			continue
		}
		for _, j := range bt {
			if j >= RecentLimit && ht[j].Timestamp.Equal(hs[i].Timestamp) && ht[j].SignalType == hs[i].SignalType {
				return true
			}
		}
	}
	return false
}

// classify 決定這一次評估屬於哪一群（第 7.3 節「判重歷史」與「對帳與覆蓋率」）。
func (r *Runner) classify(ctx context.Context, rec *Record) {
	e := rec.Eval
	if e.Fingerprint == "" || fingerprintAt(r.in.Candles[e.Symbol], e.LastBarTS) != e.Fingerprint {
		rec.Group, rec.GroupReason = GroupInputChanged, "輸入指紋不同"
		return
	}
	if h := r.holds[e.Symbol]; e.DataAt.Before(h.until) {
		rec.Group, rec.GroupReason = h.group, "待重新核對："+h.reason
		return
	}
	group, reasons, until := GroupReconcilable, []string{}, time.Time{}
	mark := func(g, reason string, offending time.Time) {
		if g == GroupDiverged || group == GroupReconcilable {
			group = g
		}
		reasons = append(reasons, reason)
		if t := offending.Add(Cooldown); t.After(until) {
			until = t
		}
	}

	// 1. DB 路徑：重播實際取到的最近 20 筆 vs live 同一時刻應該取到的。
	got, err := r.repoA.GetBySymbol(ctx, e.Symbol, RecentLimit+1)
	if err != nil {
		mark(GroupUnconfirmed, "讀重播庫失敗："+err.Error(), e.DataAt)
	} else {
		// live 那一邊：跟重播庫走同一條時間線（created_at 不晚於 eval_at，扣掉這一次自己寫的那一筆）。
		var live []histRow
		for _, s := range r.in.Signals {
			if s.Symbol == e.Symbol && !s.CreatedAt.After(e.EvalAt) && (rec.LiveSent == nil || s.ID != rec.LiveSent.ID) {
				live = append(live, histRow{s.SignalType, s.Direction, round2(s.Resistance), round2(s.Support), s.TS.UnixNano(), s.CreatedAt})
			}
		}
		sort.SliceStable(live, func(i, j int) bool { return live[i].ts > live[j].ts })
		var rep []histRow
		for _, s := range got {
			rep = append(rep, histRow{s.SignalType, s.Direction, round2(s.Resistance), round2(s.Support), s.Timestamp.UnixNano(), s.CreatedAt})
		}
		if t, tie := boundaryTie(live); tie {
			mark(GroupUnconfirmed, "第 20 筆邊界同 ts（live）", t)
		} else if t, tie := boundaryTie(rep); tie {
			mark(GroupUnconfirmed, "第 20 筆邊界同 ts（重播）", t)
		}
		lt, rt := top(live), top(rep)
		if a, b := normalize(lt), normalize(rt); strings.Join(a, ",") != strings.Join(b, ",") {
			mark(GroupDiverged, "DB 判重歷史不同", latestDiff(lt, rt))
		}
		// (a) data_at～eval_at 之間同一檔另有寫入
		for _, s := range r.in.Signals {
			if s.Symbol == e.Symbol && !s.CreatedAt.Before(e.DataAt) && !s.CreatedAt.After(e.EvalAt) &&
				(rec.LiveSent == nil || s.ID != rec.LiveSent.ID) {
				mark(GroupUnconfirmed, "data_at～eval_at 之間同一檔另有寫入", s.CreatedAt)
			}
		}
	}

	// 2. 保留鎖：前 15 分鐘內 live 的每一筆送出，都要對應到重播裡輸入相同、結果一致的送出。
	from := e.DataAt.Add(-Cooldown - lockMargin)
	for _, s := range r.sends[e.Symbol] {
		if !s.createdAt.After(from) || !s.createdAt.Before(e.DataAt) {
			continue
		}
		switch {
		case s.external:
			mark(GroupUnconfirmed, "前 15 分鐘內有 live 的外部送出（保留鎖驗不了）", s.createdAt)
		case !r.aSentOK[s.key]:
			mark(GroupUnconfirmed, "前 15 分鐘內有 live 的送出對應不到重播", s.createdAt)
		}
	}
	if h := r.unconfirm[e.Symbol]; e.DataAt.Before(h.until) {
		mark(GroupUnconfirmed, h.reason, h.until.Add(-Cooldown))
	}

	rec.Group, rec.GroupReason = group, strings.Join(reasons, "；")
	if group != GroupReconcilable {
		r.extendHold(e.Symbol, until, group, rec.GroupReason)
	}
}

func top(rows []histRow) []histRow {
	if len(rows) > RecentLimit {
		return rows[:RecentLimit]
	}
	return rows
}

// boundaryTie：第 20 筆與第 21 筆的 ts 相同（查詢只依 ts 排序，取到哪幾筆不確定）。
func boundaryTie(rows []histRow) (time.Time, bool) {
	if len(rows) <= RecentLimit || rows[RecentLimit-1].ts != rows[RecentLimit].ts {
		return time.Time{}, false
	}
	t := rows[RecentLimit-1].createdAt
	if rows[RecentLimit].createdAt.After(t) {
		t = rows[RecentLimit].createdAt
	}
	return t, true
}

// latestDiff 是兩邊不同的列裡最晚寫入的時間（重新核對從它起算 15 分鐘）。
func latestDiff(a, b []histRow) time.Time {
	count := map[string]int{}
	for _, x := range a {
		count[x.id()]++
	}
	for _, x := range b {
		count[x.id()]--
	}
	var t time.Time
	for _, rows := range [][]histRow{a, b} {
		for _, x := range rows {
			if count[x.id()] != 0 && x.createdAt.After(t) {
				t = x.createdAt
			}
		}
	}
	return t
}

// sameSend 比對重播送出的訊號跟 live 寫進 signals 的那一筆（實作補充第 2 點的欄位）。一致時回空字串。
func sameSend(got *store.Signal, live *LiveSignal) string {
	if got == nil || live == nil {
		return "缺少一邊的送出"
	}
	var diff []string
	if got.Symbol != live.Symbol || got.SignalType != live.SignalType || got.Direction != live.Direction {
		diff = append(diff, fmt.Sprintf("類型 %s/%s vs %s/%s", got.SignalType, got.Direction, live.SignalType, live.Direction))
	}
	for _, f := range []struct {
		name string
		g, l float64
	}{{"price", got.Price, live.Price}, {"support", got.Support, live.Support}, {"resistance", got.Resistance, live.Resistance}} {
		if round2(f.g) != round2(f.l) {
			diff = append(diff, fmt.Sprintf("%s %.2f vs %.2f", f.name, f.g, f.l))
		}
	}
	if !got.Timestamp.Equal(live.TS) {
		diff = append(diff, "ts 不同")
	}
	return strings.Join(diff, "；")
}

// reconcile 是可對帳那一群 A 對 live 的逐筆對帳。
func (r *Runner) reconcile(rec *Record) {
	e := rec.Eval
	switch {
	case e.EvalStatus == "evaluate_failed" || e.EvalStatus == "degraded":
		rec.AMatch = MatchLiveAbnormal
		persisted := "未落盤"
		if rec.LiveSent != nil {
			persisted = "已落盤"
		}
		rec.MatchDetail = fmt.Sprintf("live %s（%s，%s）；重播 A %s", e.EvalStatus, e.EvalCause, persisted, rec.A.Status)
		return
	case rec.A.Status == e.EvalStatus && e.EvalStatus == "signal_sent":
		if d := sameSend(rec.A.Sent, rec.LiveSent); d != "" {
			rec.AMatch, rec.MatchDetail = MatchUnexplained, "送出的內容不同："+d
			return
		}
		rec.AMatch = MatchOK
		return
	case rec.A.Status == e.EvalStatus:
		rec.AMatch = MatchOK
		return
	}
	if d, ok := r.explainClockBoundary(rec); ok {
		rec.AMatch, rec.MatchDetail = MatchClockBoundary, d
		return
	}
	rec.AMatch, rec.MatchDetail = MatchUnexplained, fmt.Sprintf("live %s、重播 A %s（抑制層 %s）", e.EvalStatus, rec.A.Status, rec.A.SuppressedBy)
}

// explainClockBoundary：時鐘邊界只用來篩選（距上一筆同保留鍵的送出剛好 15 分鐘不到 1 秒）；要已解釋還得同時符合
// 同一個保留鍵、DB 判重⛔ 沒有擋下、上一筆 live 也送出了，而且在可佐證的時間範圍內調整時鐘確實會翻轉結果。
func (r *Runner) explainClockBoundary(rec *Record) (string, bool) {
	e := rec.Eval
	if rec.A.Key == "" {
		return "", false
	}
	p, ok := r.lastSend[rec.A.Key]
	if ok && p.key() == e.key() { // 這一次自己剛送出：要看的是更早那一次
		ok = false
		for i := len(r.Records) - 1; i >= 0; i-- {
			x := r.Records[i]
			if x.A.Status == "signal_sent" && x.A.Key == rec.A.Key {
				p, ok = x.Eval, true
				break
			}
		}
	}
	// 前一筆錨點本身要已對帳：live 也送出、重播 A 送出且內容一致、那一次可對帳而且一致（複審 2026-11-24）。
	if !ok || r.attr.byEval[p.key()] == nil || !r.aSentOK[p.key()] {
		return "", false
	}
	if i, found := r.recIdx[p.key()]; !found || !r.Records[i].counted() {
		return "", false
	}
	delta := e.EvalAt.Sub(p.EvalAt)
	lo, hi := e.DataAt.Sub(p.EvalAt), e.EvalAt.Sub(p.DataAt)
	switch {
	case rec.A.Status == "signal_suppressed" && rec.A.SuppressedBy == string(signal.SuppressedByReservation) && e.EvalStatus == "signal_sent":
		// 這一輪 live 寫入的內容要跟 A 的候選一致（⛔ 不能讓不同類型或價位藉時鐘解釋過關）。
		if sameSend(rec.A.Candidate, rec.LiveSent) != "" {
			return "", false
		}
		if delta >= Cooldown-BoundaryWindow && delta < Cooldown && hi >= Cooldown {
			return fmt.Sprintf("重播被保留鎖擋下（距上一筆 %v），live 的時刻範圍 %v～%v 跨過 15 分鐘", delta, lo, hi), true
		}
	case rec.A.Status == "signal_sent" && e.EvalStatus == "signal_suppressed":
		if delta >= Cooldown && delta < Cooldown+BoundaryWindow && lo < Cooldown {
			return fmt.Sprintf("重播送出（距上一筆 %v），live 的時刻範圍 %v～%v 跨過 15 分鐘", delta, lo, hi), true
		}
	}
	return "", false
}

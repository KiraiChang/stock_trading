package bouncereplay

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/signal"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

func TestFixedRoundTimes(t *testing.T) {
	rs, err := FixedRoundTimes("2026-11-17")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != FixedRounds || len(rs) != 55 {
		t.Fatalf("一天 55 輪：%d", len(rs))
	}
	if got := rs[0].In(timeutil.TaipeiTZ).Format("2006-01-02 15:04"); got != "2026-11-17 09:00" {
		t.Fatalf("第一輪 09:00：%s", got)
	}
	if got := rs[54].In(timeutil.TaipeiTZ).Format("15:04"); got != "13:30" {
		t.Fatalf("最後一輪 13:30：%s", got)
	}
	if _, err := FixedRoundTimes("2026/11/17"); err == nil {
		t.Fatal("日期格式不對要報錯")
	}
}

// oldWatch：很早就加入自選清單的標的。
func oldWatch(syms ...string) []WatchEntry {
	var out []WatchEntry
	for _, s := range syms {
		out = append(out, WatchEntry{Symbol: s, AddedAt: base.AddDate(0, -1, 0)})
	}
	return out
}

// fixedIn：2330、0050 的 1 分 K 從 06:50 到 10:00（synthCandles），自選清單就是這兩檔，窗口是 11-23 一天。
func fixedIn(days ...string) *Input {
	in := &Input{From: base, To: base.Add(24 * time.Hour), Candles: map[string][]store.Candle{}, Watchlist: oldWatch("0050", "2330")}
	if len(days) == 0 {
		days = []string{"2026-11-23"}
	}
	in.ExpectedDays = days
	for _, s := range []string{"2330", "0050"} {
		in.Candles[s] = synthCandles(s, 60)
	}
	return in
}

func TestSynthesizeFixedEvals(t *testing.T) {
	in := fixedIn()
	if err := SynthesizeFixedEvals(in, in.ExpectedDays, time.Minute); err != nil {
		t.Fatal(err)
	}
	if len(in.Evals) != 2*FixedRounds {
		t.Fatalf("兩檔 × 55 輪：%d", len(in.Evals))
	}
	// 依時間、再依代號；RunID 是合成的流水號
	if in.Evals[0].Symbol != "0050" || in.Evals[1].Symbol != "2330" || in.Evals[0].RunID != 1 || in.Evals[1].RunID != 2 {
		t.Fatalf("排序或流水號不對：%+v %+v", in.Evals[0], in.Evals[1])
	}
	e := in.Evals[2] // 09:05 的 0050
	if !e.DataAt.Equal(base.Add(5*time.Minute)) || !e.EvalAt.Equal(e.DataAt) || !e.LastBarTS.Equal(base.Add(4*time.Minute)) {
		t.Fatalf("09:05 那一輪只用 ts ≤ 09:04 的 K 棒：%+v", e)
	}
	if e.Fingerprint != "" || e.EvalStatus != "" {
		t.Fatalf("合成的評估⛔ 沒有指紋與 live 結果：%+v", e)
	}
	last := in.Evals[len(in.Evals)-1] // 13:30：K 棒只到 10:00 → 最後一根是 10:00
	if !last.LastBarTS.Equal(base.Add(60 * time.Minute)) {
		t.Fatalf("13:30 的最後一根是 10:00：%v", last.LastBarTS)
	}
	// bar-lag 0：用到輪次當下那一根
	if err := SynthesizeFixedEvals(in, in.ExpectedDays, 0); err != nil {
		t.Fatal(err)
	}
	if !in.Evals[2].LastBarTS.Equal(base.Add(5 * time.Minute)) {
		t.Fatalf("bar-lag 0：%v", in.Evals[2].LastBarTS)
	}
	// 分母是自選清單：清單外的檔⛔ 不評估；整日缺 1 分 K 的⛔ 不評估；當天 09:00 之後才加入的⛔ 不評估。
	in.Candles["9999"] = synthCandles("9999", 60)
	in.Candles["3008"] = synthCandles("3008", 60)
	in.Watchlist = append(oldWatch("0050", "2330", "2454"), WatchEntry{Symbol: "3008", AddedAt: base.Add(time.Second)})
	if err := SynthesizeFixedEvals(in, in.ExpectedDays, time.Minute); err != nil {
		t.Fatal(err)
	}
	for _, e := range in.Evals {
		if e.Symbol != "0050" && e.Symbol != "2330" {
			t.Fatalf("只評估分母裡當天有 1 分 K 的檔：%+v", e)
		}
	}
	in.Watchlist = nil
	if err := SynthesizeFixedEvals(in, in.ExpectedDays, time.Minute); err == nil {
		t.Fatal("沒有自選清單要報錯")
	}
}

func TestParseWatchlist(t *testing.T) {
	ws, err := ParseWatchlist(strings.NewReader("symbol,added_at\n2330,2026-10-01 09:12:34.123456+08\n0050,2026-11-23 09:00:00+08\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 2 || ws[0].Symbol != "0050" || !ws[0].AddedAt.Equal(base) || ws[1].AddedAt.Nanosecond() != 123456000 {
		t.Fatalf("解析或排序不對：%+v", ws)
	}
	for _, bad := range []string{"", "symbol\n2330\n", "symbol,added_at\n", "symbol,added_at\n2330,2026-10-01 09:00:00+08\n2330,2026-10-02 09:00:00+08\n",
		"symbol,added_at\n2330,2026/10/01\n", "symbol,added_at\n,2026-10-01 09:00:00+08\n"} {
		if _, err := ParseWatchlist(strings.NewReader(bad)); err == nil {
			t.Fatalf("要報錯：%q", bad)
		}
	}
}

func TestFixedMembers(t *testing.T) {
	in := &Input{Watchlist: []WatchEntry{{Symbol: "0050", AddedAt: base}, {Symbol: "2330", AddedAt: base.Add(time.Second)}}}
	m, later, err := FixedMembers(in, "2026-11-23")
	if err != nil || len(m) != 1 || m[0] != "0050" || len(later) != 1 || later[0] != "2330" {
		t.Fatalf("09:00（含）之前加入的才算當天的分母：%v %v %v", m, later, err)
	}
	if m, _, _ := FixedMembers(in, "2026-11-24"); len(m) != 2 {
		t.Fatalf("隔天兩檔都算：%v", m)
	}
}

func TestFixedRunnerInjectionAndMetrics(t *testing.T) {
	in := fixedIn()
	daily := time.Date(2026, 11, 22, 16, 0, 0, 0, time.UTC) // 台北 11-23 00:00（日 K 的 ts）
	in.Signals = []LiveSignal{
		// 起始歷史（窗口之前）→ 注入
		{ID: 1, Symbol: "2330", SignalType: "BREAKOUT", Direction: "BUY", Price: 104, Strength: 1, TS: base.Add(-24 * time.Hour), CreatedAt: base.Add(-20 * time.Hour)},
		// 窗口內的日 K 訊號 → 注入
		{ID: 2, Symbol: "2330", SignalType: "BREAKDOWN", Direction: "SELL", Price: 99, Support: 99.5, Strength: 1, TS: daily, CreatedAt: base.Add(-10 * time.Minute)},
		// 窗口內盤中時間的 live 送出 → ⛔ 不注入（重播自己產生）
		{ID: 3, Symbol: "2330", SignalType: "SUPPORT_BOUNCE", Direction: "BUY", Price: 104, Support: 100, Strength: 1, TS: base.Add(3 * time.Minute), CreatedAt: base.Add(5 * time.Minute)},
		{ID: 4, Symbol: "2330", SignalType: "SUPPORT_BOUNCE", Direction: "BUY", Price: 104, Support: 100, Strength: 1, TS: base.Add(12 * time.Minute), CreatedAt: base.Add(15 * time.Minute)},
	}
	if err := SynthesizeFixedEvals(in, in.ExpectedDays, time.Minute); err != nil {
		t.Fatal(err)
	}
	dbA, dbB := sqliteDB(t), sqliteDB(t)
	r := NewFixedRunner(in, dbA, dbB, zap.NewNop())
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, db := range []string{"A", "B"} {
		d := dbA
		if db == "B" {
			d = dbB
		}
		var injected, liveIntraday int
		if err := d.Get(&injected, `SELECT count(*) FROM signals WHERE signal_type IN ('BREAKOUT', 'BREAKDOWN') AND timeframe IS NULL`); err != nil {
			t.Fatal(err)
		}
		// live 的盤中送出是 created_at 剛好在輪次＋0 的那兩筆；重播自己產生的 created_at 是重播的時鐘，timeframe 是 1m
		if err := d.Get(&liveIntraday, `SELECT count(*) FROM signals WHERE timeframe IS NULL AND signal_type = 'SUPPORT_BOUNCE'`); err != nil {
			t.Fatal(err)
		}
		if injected != 2 || liveIntraday != 0 {
			t.Fatalf("%s：起始歷史與日 K 要注入（%d）、盤中時間的 live 送出⛔ 不注入（%d）", db, injected, liveIntraday)
		}
	}
	if !strings.Contains(strings.Join(r.Notes, ""), "注入 2 筆") {
		t.Fatalf("註記要寫出注入幾筆：%v", r.Notes)
	}

	m := computeMetrics(r.Records, "", func(Record) bool { return true })
	a, b := m.A["合計"], m.B["合計"]
	// A 依 K 棒時間判重、B 依送出時間（時鐘）判重，兩者的送出數⛔ 沒有固定的大小關係；這裡只要求兩邊都有送出。
	if a == nil || b == nil || a.Sent == 0 || b.Sent == 0 {
		t.Fatalf("A、B 都要有送出：A %+v B %+v", a, b)
	}
	if b.ShortPairs != 0 {
		t.Fatalf("B 的 15 分鐘內重複要是 0：%+v", b)
	}
	// K 棒只到 10:00：10:15 之後的輪次資料延遲 → 扣住、⛔ 沒有送出
	held := 0
	for _, x := range r.Records {
		if x.DataAt.After(base.Add(70 * time.Minute)) {
			if !x.Withhold || x.A.Sent != nil || x.B.Sent != nil {
				t.Fatalf("資料延遲要扣住：%+v", x)
			}
			held++
		}
	}
	if held == 0 || len(m.WithholdViol) != 0 {
		t.Fatalf("要有扣住的輪次、⛔ 沒有違規：held=%d viol=%v", held, m.WithholdViol)
	}

	live := liveBounces(in, func(LiveSignal) bool { return true })
	if x := live["2026-11-23"]; x == nil || x.sent != 2 || x.shortPairs != 1 || x.sameSupport != 1 {
		t.Fatalf("live 當天的盤中 SUPPORT_BOUNCE：%+v", x)
	}
}

func TestWriteFixedReportCompleteness(t *testing.T) {
	in := fixedIn("2026-11-23", "2026-11-24")
	if err := SynthesizeFixedEvals(in, in.ExpectedDays, time.Minute); err != nil {
		t.Fatal(err)
	}
	r := NewFixedRunner(in, sqliteDB(t), sqliteDB(t), zap.NewNop())
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	sum := WriteFixedReport(&buf, in, r.Records, in.ExpectedDays, time.Minute, r.Notes, nil, nil)
	out := buf.String()
	if sum.OK || len(sum.Failures) != 1 || !strings.Contains(sum.Failures[0], "2026-11-24 缺資料") || len(sum.Incomplete) != 0 {
		t.Fatalf("11-24 沒有任何 1 分 K 要判缺資料：%+v", sum)
	}
	for _, want := range []string{"保真度較低", "| 2026-11-24 | 0／2 | 0050、2330 | — | ⚠️ 缺資料 |", "live 當天實際的盤中 SUPPORT_BOUNCE", "A／B 比較",
		"以有逐輪存證的主要窗口（11-23～11-27）為準", "沒有辦法還原交接前已經移除的標的", "| 2026-11-23 | 2/2 |"} {
		want = strings.ReplaceAll(want, "2/2", "2／2")
		if !strings.Contains(out, want) {
			t.Fatalf("報告缺「%s」：\n%s", want, out)
		}
	}
	// 每天都有資料 → OK
	in2 := fixedIn()
	if err := SynthesizeFixedEvals(in2, in2.ExpectedDays, time.Minute); err != nil {
		t.Fatal(err)
	}
	r2 := NewFixedRunner(in2, sqliteDB(t), sqliteDB(t), zap.NewNop())
	if err := r2.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if s := WriteFixedReport(&buf, in2, r2.Records, in2.ExpectedDays, time.Minute, r2.Notes, nil, nil); !s.OK {
		t.Fatalf("每天都有 1 分 K 要 OK：%+v", s)
	}
	if s := WriteFixedReport(&buf, in2, r2.Records, nil, time.Minute, nil, nil, nil); s.OK {
		t.Fatal("預定交易日清單是空的要判不完整")
	}
	// 自我檢查做不了：報告照樣產出、列進完整性
	buf.Reset()
	s := WriteFixedReport(&buf, in2, r2.Records, in2.ExpectedDays, time.Minute, nil, nil, []string{"B 的自我檢查：做不了"})
	if s.OK || len(s.Failures) != 1 || !strings.Contains(buf.String(), "* ⚠️ B 的自我檢查：做不了") {
		t.Fatalf("自我檢查做不了要列進完整性：%+v", s)
	}
}

// 分母是自選清單：整段缺 1 分 K 的檔留在分母（2／3）、列出缺 K 的標的、保留它的 live 送出；後來加入與清單外的⛔ 不納入。
func TestWriteFixedReportDenominator(t *testing.T) {
	in := fixedIn()
	in.Watchlist = append(oldWatch("0050", "2330", "2454"), WatchEntry{Symbol: "3008", AddedAt: base.Add(time.Minute)})
	in.Unlisted = []string{"9999"}
	in.Signals = []LiveSignal{
		{ID: 1, Symbol: "2454", SignalType: "SUPPORT_BOUNCE", Direction: "BUY", Price: 50, Support: 49, Strength: 1, TS: base.Add(3 * time.Minute), CreatedAt: base.Add(5 * time.Minute)},
		{ID: 2, Symbol: "2454", SignalType: "SUPPORT_BOUNCE", Direction: "BUY", Price: 50, Support: 49, Strength: 1, TS: base.Add(8 * time.Minute), CreatedAt: base.Add(10 * time.Minute)},
		{ID: 3, Symbol: "3008", SignalType: "SUPPORT_BOUNCE", Direction: "BUY", Price: 50, Support: 49, Strength: 1, TS: base.Add(8 * time.Minute), CreatedAt: base.Add(10 * time.Minute)},
	}
	if err := SynthesizeFixedEvals(in, in.ExpectedDays, time.Minute); err != nil {
		t.Fatal(err)
	}
	r := NewFixedRunner(in, sqliteDB(t), sqliteDB(t), zap.NewNop())
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	sum := WriteFixedReport(&buf, in, r.Records, in.ExpectedDays, time.Minute, r.Notes, nil, nil)
	out := buf.String()
	if sum.OK || len(sum.Failures) != 1 || !strings.Contains(sum.Failures[0], "2026-11-23 缺資料：自選清單 1 檔整日沒有 1 分 K（2454）") {
		t.Fatalf("分母裡整日缺 1 分 K 要判缺資料：%+v", sum)
	}
	for _, want := range []string{
		"| 2026-11-23 | 2／3 | 2454 | 3008 | ⚠️ 缺資料 |",
		"| 2026-11-23 | 2454 | ⛔ 沒有（缺資料） | 0 | — | — | 2（1） |",
		"| 2026-11-23 | 2 | 1（1） |", // live 的合計含缺 K 的 2454、⛔ 不含後來加入的 3008
		"⛔ 不納入）：9999",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("報告缺「%s」：\n%s", want, out)
		}
	}
	if strings.Contains(out, "| 2026-11-23 | 3008 |") {
		t.Fatalf("後來加入的⛔ 不列在當天的每檔明細：\n%s", out)
	}
}

// A 或 B 有一筆評估失敗、降級或讀取失敗，或行情不足 A／B ⛔ 不一致：當天比較不完整、結束碼 3，送出數照列。
func TestWriteFixedReportIncomplete(t *testing.T) {
	in := fixedIn()
	if err := SynthesizeFixedEvals(in, in.ExpectedDays, time.Minute); err != nil {
		t.Fatal(err)
	}
	r := NewFixedRunner(in, sqliteDB(t), sqliteDB(t), zap.NewNop())
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if s := WriteFixedReport(&buf, in, r.Records, in.ExpectedDays, time.Minute, nil, nil, nil); !s.OK {
		t.Fatalf("原樣要完整：%+v", s)
	}
	for _, c := range []struct {
		name, want string
		mut        func(*Record)
	}{
		{"B 評估失敗", "B 評估失敗 1、降級 0、讀取失敗 0", func(x *Record) { x.B.Status, x.B.Err = "evaluate_failed", "boom" }},
		{"A 降級", "A 評估失敗 0、降級 1、讀取失敗 0", func(x *Record) { x.A.Status = "degraded" }},
		{"B 讀取失敗", "B 評估失敗 0、降級 0、讀取失敗 1", func(x *Record) { x.B.ReadErr = "判重歷史：boom" }},
		{"行情不足不一致", "行情不足 A／B 不一致 1 筆", func(x *Record) { x.A.Status, x.A.Insufficient = "evaluate_failed", true }},
	} {
		recs := append([]Record(nil), r.Records...)
		c.mut(&recs[7])
		buf.Reset()
		s := WriteFixedReport(&buf, in, recs, in.ExpectedDays, time.Minute, nil, nil, nil)
		out := buf.String()
		if s.OK || len(s.Incomplete) != 1 || len(s.Failures) != 1 || !strings.Contains(s.Failures[0], "2026-11-23 比較不完整："+c.want) {
			t.Fatalf("%s：要判比較不完整：%+v", c.name, s)
		}
		if !strings.Contains(out, "比較不完整的日期：2026-11-23") || !strings.Contains(out, "不能用來判讀") || !strings.Contains(out, "| 合計 |") {
			t.Fatalf("%s：報告要標比較不完整、送出數照列：\n%s", c.name, out)
		}
	}
}

// 真的行情不足（ErrInsufficientCandles）：A、B 逐輪逐檔一致 → 只統計、⛔ 不算比較不完整；暖身不足與資料延遲分開統計。
func TestFixedInsufficientCandles(t *testing.T) {
	in := fixedIn()
	in.Watchlist = oldWatch("0050", "2330", "1101")
	cs := synthCandles("1101", 60)
	in.Candles["1101"] = cs[120:] // 從 08:50 開始：09:00 那一輪只有 10 根
	if err := SynthesizeFixedEvals(in, in.ExpectedDays, time.Minute); err != nil {
		t.Fatal(err)
	}
	r := NewFixedRunner(in, sqliteDB(t), sqliteDB(t), zap.NewNop())
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, x := range r.Records {
		if x.A.Insufficient {
			n++
			if x.Symbol != "1101" || !x.B.Insufficient || x.A.Status != "evaluate_failed" {
				t.Fatalf("行情不足：%+v", x)
			}
		}
	}
	if n == 0 {
		t.Fatal("1101 開頭幾輪要行情不足")
	}
	var buf bytes.Buffer
	s := WriteFixedReport(&buf, in, r.Records, in.ExpectedDays, time.Minute, nil, nil, nil)
	if !s.OK {
		t.Fatalf("行情不足 A、B 一致⛔ 不算比較不完整：%+v", s)
	}
	row := fmt.Sprintf("| 2026-11-23 | %d | ", 3*FixedRounds)
	if out := buf.String(); !strings.Contains(out, row) || !strings.Contains(out, fmt.Sprintf("| %d／%d | 0／0 | 0／0 | 0／0 | 0 |", n, n)) {
		t.Fatalf("評估狀況要分列行情不足（%d）：\n%s", n, out)
	}
}

// 送出統計以成功寫入 signals 為準（2026-11-25 review）：降級但寫入成功的算、寫入失敗的⛔ 不算；送出數與短時間重複對數用同一批。
func TestStatusOfPersisted(t *testing.T) {
	s := &store.Signal{Symbol: "2330", SignalType: "SUPPORT_BOUNCE"}
	o := statusOf(&signal.EvaluateResult{SignalGenerated: true, Signal: s, Candidate: s, Degraded: true, DBPersisted: true}, nil)
	if o.Status != "degraded" || o.Sent == nil || !o.Persisted {
		t.Fatalf("降級但寫入成功：%+v", o)
	}
	o = statusOf(&signal.EvaluateResult{SignalGenerated: true, Signal: s, Candidate: s, Degraded: true}, nil)
	if o.Status != "degraded" || o.Sent == nil || o.Persisted {
		t.Fatalf("寫入失敗：%+v", o)
	}
}

func TestComputeMetricsPersisted(t *testing.T) {
	sb := func(min int) *store.Signal {
		return &store.Signal{Symbol: "2330", SignalType: "SUPPORT_BOUNCE", Support: 100, CreatedAt: base.Add(time.Duration(min) * time.Minute)}
	}
	bo := &store.Signal{Symbol: "2330", SignalType: "BREAKOUT", Direction: "BUY", Price: 105, Timestamp: base}
	recs := []Record{
		{Eval: Eval{RunID: 1, Symbol: "2330"}, Day: "d", A: Outcome{Status: "signal_sent", Candidate: sb(0), Sent: sb(0), Persisted: true}},
		{Eval: Eval{RunID: 2, Symbol: "2330"}, Day: "d", A: Outcome{Status: "degraded", Candidate: sb(5), Sent: sb(5), Persisted: true}}, // 判重降級、寫入成功
		{Eval: Eval{RunID: 3, Symbol: "2330"}, Day: "d", A: Outcome{Status: "degraded", Candidate: sb(10), Sent: sb(10)}},                // 寫入失敗
		{Eval: Eval{RunID: 4, Symbol: "2330"}, Day: "d", Withhold: true,
			A: Outcome{Status: "signal_sent", Candidate: bo, Sent: bo, Persisted: true},
			B: Outcome{Status: "degraded", Candidate: bo, Sent: bo}}, // B 寫入失敗：⛔ 不在送出集合，但 Withhold 的回歸檢查照原本看有沒有送出
	}
	m := computeMetrics(recs, "", func(Record) bool { return true })
	a := m.A["合計"]
	if a.Candidates != 3 || a.Sent != 2 || a.ShortPairs != 1 || a.SameSupportPairs != 1 {
		t.Fatalf("送出數與短時間重複要用同一批（寫入成功的 0、5 分）：%+v", a)
	}
	if len(m.NonTargetDiff) != 1 || !strings.Contains(m.NonTargetDiff[0], "A 1 次、B 0 次") {
		t.Fatalf("非目標訊號的送出集合只算寫入成功的：%v", m.NonTargetDiff)
	}
	if len(m.WithholdViol) != 1 {
		t.Fatalf("Withhold 的回歸檢查沿用原本的判定（有送出就算）：%v", m.WithholdViol)
	}
}

type failInsert struct{ store.SignalRepo }

func (failInsert) Insert(context.Context, *store.Signal) error { return errors.New("insert boom") }

// 真的寫入失敗：引擎回 DBPersisted=false → 狀態 degraded、Sent 有值、Persisted false；正常的 repo 則是 Persisted true。
func TestEnginePersistFailure(t *testing.T) {
	in := fixedIn()
	if err := SynthesizeFixedEvals(in, in.ExpectedDays, time.Minute); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		fail bool
	}{{"寫入成功", false}, {"寫入失敗", true}} {
		db := sqliteDB(t)
		var repo store.SignalRepo = store.NewSignalRepo(db)
		if c.fail {
			repo = failInsert{repo}
		}
		var now time.Time
		cands := &cutoffCandles{all: in.Candles}
		eng := newEngine(cands, repo, store.NewIndicatorRepo(db), signal.DedupPriceKey, func() time.Time { return now }, zap.NewNop())
		var got *Outcome
		for _, e := range in.Evals {
			cands.cutoff, now = e.LastBarTS, e.EvalAt
			res, err := eng.EvaluateWithOptions(context.Background(), e.Symbol, Timeframe, signal.EvaluateOptions{})
			if o := statusOf(res, err); o.Sent != nil {
				got = &o
				break
			}
		}
		if got == nil {
			t.Fatalf("%s：要有一筆送出", c.name)
		}
		if got.Persisted == c.fail || (c.fail && got.Status != "degraded") {
			t.Fatalf("%s：%+v", c.name, *got)
		}
	}
}

// 報告：降級但寫入成功的送出照算、當天仍標比較不完整；寫入失敗的⛔ 不算。資料延遲輪數與實際扣住（withheld_stale）分開列。
func TestWriteFixedReportPersistedAndWithheld(t *testing.T) {
	in := fixedIn()
	if err := SynthesizeFixedEvals(in, in.ExpectedDays, time.Minute); err != nil {
		t.Fatal(err)
	}
	r := NewFixedRunner(in, sqliteDB(t), sqliteDB(t), zap.NewNop())
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	idx := -1
	for i, x := range r.Records {
		if x.A.Status == "signal_sent" && x.A.Sent.SignalType == "SUPPORT_BOUNCE" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("要有一筆 A 送出的 SUPPORT_BOUNCE")
	}
	sent := func(recs []Record) int {
		return computeMetrics(recs, "", func(Record) bool { return true }).A["合計"].Sent
	}
	base0 := sent(r.Records)
	deg := append([]Record(nil), r.Records...)
	deg[idx].A.Status = "degraded" // 判重降級、寫入成功
	var buf bytes.Buffer
	s := WriteFixedReport(&buf, in, deg, in.ExpectedDays, time.Minute, nil, nil, nil)
	if sent(deg) != base0 || s.OK || len(s.Incomplete) != 1 {
		t.Fatalf("降級但寫入成功：送出 %d／%d、%+v", sent(deg), base0, s)
	}
	lost := append([]Record(nil), deg...)
	lost[idx].A.Persisted = false
	if sent(lost) != base0-1 {
		t.Fatalf("寫入失敗⛔ 不算送出：%d／%d", sent(lost), base0)
	}

	delayed, wa, wb := 0, 0, 0
	for _, x := range r.Records {
		if x.Withhold {
			delayed++
		}
		if x.A.Status == "withheld_stale" {
			wa++
		}
		if x.B.Status == "withheld_stale" {
			wb++
		}
	}
	if delayed == 0 || wa == delayed {
		t.Fatalf("案例要有資料延遲、而且 A 的實際扣住跟延遲輪數不同（先被判重擋下）：delayed=%d A=%d B=%d", delayed, wa, wb)
	}
	buf.Reset()
	WriteFixedReport(&buf, in, r.Records, in.ExpectedDays, time.Minute, nil, nil, nil)
	row := fmt.Sprintf("| 2026-11-23 | %d | 0 | %d | %d／%d | 0／0 |", len(r.Records), delayed, wa, wb)
	if out := buf.String(); !strings.Contains(out, "| 資料延遲輪數 | 實際扣住 A／B |") || !strings.Contains(out, row) {
		t.Fatalf("報告缺「%s」：\n%s", row, out)
	}
}

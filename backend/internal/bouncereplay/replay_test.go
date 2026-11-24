package bouncereplay

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
	"github.com/trading/backend/internal/signal"
	"github.com/trading/backend/internal/store"
)

// 合成資料：1 分 K 每分鐘一根，分鐘序號是 3 的倍數那一根探到 100、收在上半部（觸發 SUPPORT_BOUNCE，支撐 100），
// 其餘最低 101.5（⛔ 不觸發）。盤中每 5 分鐘一輪，最後一根＝排定的時點，所以每 15 分鐘觸發一次。
// 「live」用同一個引擎（A）跑一遍產生，重播應該全部可對帳、逐筆一致。

var base = time.Date(2026, 11, 23, 1, 0, 0, 0, time.UTC) // 台北 09:00

func sqliteDB(t *testing.T) *sqlx.DB {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "replay-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	db, err := store.NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: f.Name()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	return db
}

func synthCandles(sym string, minutes int) []store.Candle {
	var out []store.Candle
	for i := -130; i <= minutes; i++ {
		low := 101.5
		if (i+300)%3 == 0 {
			low = 100
		}
		out = append(out, store.Candle{Symbol: sym, Timeframe: Timeframe, Open: 103, High: 105, Low: low, Close: 104,
			Volume: 1000, Timestamp: base.Add(time.Duration(i) * time.Minute)})
	}
	return out
}

type timing func(k int, sym string) (dataOff, evalDur time.Duration)

func fixedTiming(int, string) (time.Duration, time.Duration) {
	return 200 * time.Millisecond, 300 * time.Millisecond
}

// simulateLive 用引擎 A 模擬 live：每一輪、每一檔評估一次，記下 eval_status、指紋與寫進 signals 的列
// （created_at＝data_at＋100ms，比照 live 由 DB 在寫入當下產生）。
func simulateLive(t *testing.T, syms []string, rounds int, tm timing) *Input {
	t.Helper()
	in := &Input{From: base, To: base.Add(24 * time.Hour), Candles: map[string][]store.Candle{}, ExpectedDays: []string{"2026-11-23"}}
	for _, s := range syms {
		in.Candles[s] = synthCandles(s, rounds*5+5)
	}
	db := sqliteDB(t)
	cc := &cutoffCandles{all: in.Candles}
	var now time.Time
	eng := newEngine(cc, store.NewSignalRepo(db), store.NewIndicatorRepo(db), signal.DedupPriceKey, func() time.Time { return now }, zap.NewNop())
	id := uint64(0)
	for k := 0; k < rounds; k++ {
		at := base.Add(time.Duration(k*5) * time.Minute)
		for i, sym := range syms {
			off, dur := tm(k, sym)
			e := Eval{RunID: uint64(100 + k), Symbol: sym, DataAt: at.Add(off), LastBarTS: at}
			e.EvalAt = e.DataAt.Add(dur + time.Duration(i)*time.Millisecond)
			cc.cutoff, now = e.LastBarTS, e.DataAt.Add(100*time.Millisecond)
			res, err := eng.EvaluateWithOptions(context.Background(), sym, Timeframe, signal.EvaluateOptions{Withhold: withhold(e)})
			o := statusOf(res, err)
			e.EvalStatus, e.Fingerprint = o.Status, fingerprintAt(in.Candles[sym], e.LastBarTS)
			if o.Sent != nil {
				id++
				s := o.Sent
				in.Signals = append(in.Signals, LiveSignal{ID: id, Symbol: sym, SignalType: s.SignalType, Direction: s.Direction,
					Price: round2(s.Price), Volume: s.Volume, VolRatio: s.VolRatio, Resistance: round2(s.Resistance), Support: round2(s.Support),
					Trend: s.Trend, Note: s.Note, Strength: s.Strength, TS: s.Timestamp, CreatedAt: e.DataAt.Add(100 * time.Millisecond)})
			}
			in.Evals = append(in.Evals, e)
		}
	}
	return in
}

func replay(t *testing.T, in *Input) *Runner {
	t.Helper()
	r := NewRunner(in, sqliteDB(t), sqliteDB(t), zap.NewNop())
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	return r
}

func find(t *testing.T, r *Runner, run uint64, sym string) Record {
	t.Helper()
	for _, x := range r.Records {
		if x.RunID == run && x.Symbol == sym {
			return x
		}
	}
	t.Fatalf("找不到 run %d %s", run, sym)
	return Record{}
}

func TestSyntheticFixtureSendsEvery15Min(t *testing.T) {
	in := simulateLive(t, []string{"2330"}, 7, fixedTiming)
	var got []string
	for _, e := range in.Evals {
		got = append(got, e.EvalStatus)
	}
	want := "signal_sent,no_signal,no_signal,signal_sent,no_signal,no_signal,signal_sent"
	if strings.Join(got, ",") != want {
		t.Fatalf("合成資料失效（每 15 分鐘觸發一次才測得到判重）：%v", got)
	}
	if len(in.Signals) != 3 || in.Signals[0].SignalType != "SUPPORT_BOUNCE" || in.Signals[0].Support != 100 {
		t.Fatalf("要送出 3 筆支撐 100 的 SUPPORT_BOUNCE：%+v", in.Signals)
	}
}

func TestReplayReproducesLiveAllReconcilable(t *testing.T) {
	in := simulateLive(t, []string{"2330", "0050"}, 12, fixedTiming)
	r := replay(t, in)
	for _, x := range r.Records {
		if x.Group != GroupReconcilable || x.AMatch != MatchOK {
			t.Fatalf("live 由同一個引擎產生，重播要全部可對帳、一致：run %d %s group=%s（%s）match=%s（%s）",
				x.RunID, x.Symbol, x.Group, x.GroupReason, x.AMatch, x.MatchDetail)
		}
	}
	s := Summarize(r.Records, in.ExpectedDays)
	if !s.CoverageOK || !s.Comparable {
		t.Fatalf("覆蓋率與可比較：%+v", s)
	}
	var buf bytes.Buffer
	WriteReport(&buf, r.Records, in.ExpectedDays, r.Notes, []string{"test"})
	if !strings.Contains(buf.String(), "A／B 比較：全部重播的評估") {
		t.Fatalf("要輸出比較：\n%s", buf.String())
	}
}

func TestInputChangedAndUnmappedLockAreNotReconcilable(t *testing.T) {
	in := simulateLive(t, []string{"2330"}, 6, fixedTiming)
	// live 送出那一輪（run 100，09:00）的最後一根事後被改寫：指紋對不上。
	// 之後的輪次在 live 讀到的已經是改寫後的值（指紋涵蓋最近 120 根），所以它們的指紋照改寫後的重算。
	cs := in.Candles["2330"]
	for i := range cs {
		if cs[i].Timestamp.Equal(base) {
			cs[i].Close = 104.2
		}
	}
	for i := range in.Evals {
		if in.Evals[i].RunID != 100 {
			in.Evals[i].Fingerprint = fingerprintAt(cs, in.Evals[i].LastBarTS)
		}
	}
	r := replay(t, in)
	if x := find(t, r, 100, "2330"); x.Group != GroupInputChanged {
		t.Fatalf("改寫過的那一輪要是輸入已變：%+v", x.Group)
	}
	// 09:05、09:10：前 15 分鐘內 live 的送出對應不到重播（那一輪指紋不同）→ 未確認。
	for _, run := range []uint64{101, 102} {
		if x := find(t, r, run, "2330"); x.Group != GroupUnconfirmed || !strings.Contains(x.GroupReason, "對應不到重播") {
			t.Fatalf("run %d 要歷史未確認（保留鎖驗不了）：%s %s", run, x.Group, x.GroupReason)
		}
	}
}

func TestExternalSignalsAreInjectedAndUnconfirmLock(t *testing.T) {
	in := simulateLive(t, []string{"2330"}, 6, fixedTiming)
	daily := time.Date(2026, 11, 22, 16, 0, 0, 0, time.UTC) // 台北 11-23 00:00（日 K）
	in.Signals = append(in.Signals,
		LiveSignal{ID: 900, Symbol: "2330", SignalType: "BREAKDOWN", Direction: "SELL", Price: 99, Support: 99.5, TS: daily,
			CreatedAt: base.Add(16 * time.Minute)},
		LiveSignal{ID: 901, Symbol: "2330", SignalType: "VOLUME_SPIKE", Direction: "WATCH", Price: 104, TS: base.Add(17 * time.Minute),
			CreatedAt: base.Add(17*time.Minute + time.Second)}, // 盤中時間、沒有配對 → 來源不明
	)
	sortSignals(in)
	r := replay(t, in)
	if r.attr.origin[900] != OriginExternalDaily || r.attr.origin[901] != OriginUnknown {
		t.Fatalf("歸因：%v", r.attr.origin)
	}
	if len(r.Notes) == 0 || !strings.Contains(strings.Join(r.Notes, "\n"), "來源不明") {
		t.Fatalf("來源不明要列出：%v", r.Notes)
	}
	x := find(t, r, 104, "2330") // 09:20：前 15 分鐘內有外部送出
	if x.Group != GroupUnconfirmed {
		t.Fatalf("外部送出之後 15 分鐘內要歷史未確認：%s %s", x.Group, x.GroupReason)
	}
	rows, _ := store.NewSignalRepo(r.dbA).GetBySymbol(context.Background(), "2330", 50)
	seen := 0
	for _, s := range rows {
		if s.SignalType == "BREAKDOWN" || s.SignalType == "VOLUME_SPIKE" {
			seen++
			if s.Timeframe.Valid {
				t.Fatal("注入的外部訊號 timeframe 要留 NULL")
			}
		}
	}
	if seen != 2 {
		t.Fatalf("兩筆外部訊號都要注入重播庫：%d", seen)
	}
}

func TestUnexplainedMismatchStopsAndHistoryDiverges(t *testing.T) {
	in := simulateLive(t, []string{"2330"}, 6, fixedTiming)
	// 竄改：live 在 09:15（run 103）其實沒送出。
	for i := range in.Evals {
		if in.Evals[i].RunID == 103 {
			in.Evals[i].EvalStatus = "no_signal"
		}
	}
	var kept []LiveSignal
	for _, s := range in.Signals {
		if !s.TS.Equal(base.Add(15 * time.Minute)) {
			kept = append(kept, s)
		}
	}
	in.Signals = kept
	r := replay(t, in)
	if x := find(t, r, 103, "2330"); x.AMatch != MatchUnexplained {
		t.Fatalf("live 沒送出、重播送出，要原因不明：%+v", x)
	}
	if x := find(t, r, 104, "2330"); x.Group != GroupDiverged {
		t.Fatalf("重播多了一筆，下一輪 DB 判重歷史要分歧：%s %s", x.Group, x.GroupReason)
	}
	var buf bytes.Buffer
	if s := WriteReport(&buf, r.Records, in.ExpectedDays, r.Notes, nil); s.Comparable || !strings.Contains(buf.String(), "⛔ 不進入 A／B 比較") {
		t.Fatalf("原因不明要停下、⛔ 不輸出比較：\n%s", buf.String())
	}
}

func TestClockBoundaryExplained(t *testing.T) {
	// run 100 評估較慢（eval_at 晚 0.9 秒）、run 103 較快：重播用 eval_at，兩次相差 15 分鐘少 0.6 秒 → 被保留鎖擋下；
	// live 的保留時刻在 data_at＋100ms，相差剛好 15 分鐘 → 送出。live 的時刻範圍跨過 15 分鐘 → 已解釋。
	tm := func(k int, _ string) (time.Duration, time.Duration) {
		if k == 0 {
			return 200 * time.Millisecond, 900 * time.Millisecond
		}
		return 200 * time.Millisecond, 300 * time.Millisecond
	}
	in := simulateLive(t, []string{"2330"}, 5, tm)
	r := replay(t, in)
	x := find(t, r, 103, "2330")
	if x.Group != GroupReconcilable || x.AMatch != MatchClockBoundary || x.A.SuppressedBy != string(signal.SuppressedByReservation) {
		t.Fatalf("要是已解釋的時鐘邊界：group=%s match=%s by=%s（%s）", x.Group, x.AMatch, x.A.SuppressedBy, x.MatchDetail)
	}
}

func TestClockBoundaryNeedsCorroboratedRange(t *testing.T) {
	// 篩選條件成立（重播相差 15 分鐘少 0.4 秒、被保留鎖擋下），但 live 的時刻範圍最多只到 15 分鐘少 0.1 秒、翻不過去 →
	// 竄改成 live 送出時要是原因不明，⛔ 不能算時鐘邊界。
	tm := func(k int, _ string) (time.Duration, time.Duration) {
		if k == 3 {
			return 0, 100 * time.Millisecond
		}
		return fixedTiming(k, "")
	}
	in := simulateLive(t, []string{"2330"}, 5, tm)
	for i := range in.Evals {
		e := &in.Evals[i]
		if e.RunID == 103 {
			if e.EvalStatus != "signal_suppressed" {
				t.Fatalf("前提：live 在 run 103 也被保留鎖擋下，得到 %s", e.EvalStatus)
			}
			e.EvalStatus = "signal_sent"
			in.Signals = append(in.Signals, LiveSignal{ID: 800, Symbol: "2330", SignalType: "SUPPORT_BOUNCE", Direction: in.Signals[0].Direction,
				Price: in.Signals[0].Price, Support: 100, TS: e.LastBarTS, CreatedAt: e.DataAt.Add(50 * time.Millisecond)})
		}
	}
	sortSignals(in)
	r := replay(t, in)
	if x := find(t, r, 103, "2330"); x.AMatch != MatchUnexplained {
		t.Fatalf("翻不過去就⛔ 不能算已解釋：%s（%s）", x.AMatch, x.MatchDetail)
	}
}

func TestWithholdReconstructedFromDataAt(t *testing.T) {
	// 最後一輪（run 106，09:30 那根會觸發）資料延遲 10 分鐘多一點：live 扣住（withheld_stale），重播要用 data_at − last_bar_ts 還原、照樣扣住。
	tm := func(k int, _ string) (time.Duration, time.Duration) {
		if k == 6 {
			return 10*time.Minute + time.Second, 300 * time.Millisecond
		}
		return fixedTiming(k, "")
	}
	in := simulateLive(t, []string{"2330"}, 7, tm)
	if in.Evals[6].EvalStatus != "withheld_stale" {
		t.Fatalf("前提：live 那一輪要扣住，得到 %s", in.Evals[6].EvalStatus)
	}
	r := replay(t, in)
	if x := find(t, r, 106, "2330"); !x.Withhold || x.A.Status != "withheld_stale" || x.B.Status != "withheld_stale" || x.AMatch != MatchOK {
		t.Fatalf("重播要扣住並一致：%+v", x)
	}
}

func TestSelfCheckCatchesMissingBroadcast(t *testing.T) {
	in := simulateLive(t, []string{"2330"}, 4, fixedTiming)
	for _, p := range []signal.DedupPolicy{signal.DedupPriceKey, signal.DedupIntradayBounceBySymbol} {
		if _, err := SelfCheck(context.Background(), in, p, zap.NewNop()); err != nil {
			t.Fatalf("規則 %d 的自我檢查要通過：%v", p, err)
		}
	}
	// 沒有廣播：第一次送出後保留鎖被釋放，第二次⛔ 不會被擋 → 自我檢查要失敗。
	cands := &cutoffCandles{all: in.Candles}
	var now time.Time
	eng := newEngine(cands, noAnchorSignals{}, noopIndicators{}, signal.DedupPriceKey, func() time.Time { return now }, zap.NewNop())
	eng.BroadcastFn = nil
	if _, err := selfCheck(context.Background(), in, eng, cands, &now, signal.DedupPriceKey); err == nil {
		t.Fatal("沒有廣播時自我檢查要失敗")
	}
}

func TestComputeMetrics(t *testing.T) {
	sb := func(sup float64, at time.Time) *store.Signal {
		return &store.Signal{Symbol: "2330", SignalType: "SUPPORT_BOUNCE", Support: sup, CreatedAt: at}
	}
	anchor := 100.0
	recs := []Record{
		{Eval: Eval{RunID: 1, Symbol: "2330"}, Day: "d", Group: GroupReconcilable,
			A: Outcome{Status: "signal_sent", Candidate: sb(100, base), Sent: sb(100, base)},
			B: Outcome{Status: "signal_sent", Candidate: sb(100, base), Sent: sb(100, base)}},
		{Eval: Eval{RunID: 2, Symbol: "2330"}, Day: "d", Group: GroupInputChanged,
			A: Outcome{Status: "signal_sent", Candidate: sb(101.5, base.Add(5*time.Minute)), Sent: sb(101.5, base.Add(5*time.Minute))},
			B: Outcome{Status: "signal_suppressed", Candidate: sb(101.5, base)}, BAnchorSupport: &anchor},
		{Eval: Eval{RunID: 3, Symbol: "2330", EvalStatus: "signal_suppressed"}, Day: "d", Withhold: true,
			A: Outcome{Status: "signal_suppressed", Candidate: &store.Signal{SignalType: "BREAKDOWN"}},
			B: Outcome{Status: "signal_sent", Candidate: &store.Signal{SignalType: "BREAKDOWN"},
				Sent: &store.Signal{Symbol: "2330", SignalType: "BREAKDOWN", Direction: "SELL", Price: 99, Timestamp: base}}},
	}
	m := computeMetrics(recs, "all", func(Record) bool { return true })
	a, b := m.A["合計"], m.B["合計"]
	if a.Candidates != 2 || a.Sent != 2 || a.ShortPairs != 1 || a.SameSupportPairs != 0 || b.Sent != 1 || b.Suppressed != 1 {
		t.Fatalf("A=%+v B=%+v", a, b)
	}
	if m.Merged != 1 || m.MergedDiff != 1 || m.MergedOver1 != 1 || len(m.NonTargetDiff) != 1 || len(m.WithholdViol) != 1 {
		t.Fatalf("合併／非目標／Withhold：%+v", m)
	}
	if r := computeMetrics(recs, "rec", func(r Record) bool { return r.Group == GroupReconcilable }); r.A["合計"].Sent != 1 || r.Merged != 0 {
		t.Fatalf("只算可對帳的範圍：%+v", r.A["合計"])
	}
}

func TestCheckDatabaseName(t *testing.T) {
	for _, ok := range []string{"i140_src", "i140_replay_a"} {
		if CheckDatabaseName(ok) != nil {
			t.Fatalf("%s 要通過", ok)
		}
	}
	for _, bad := range []string{"trading", "trading_dev", "i14_x", ""} {
		if CheckDatabaseName(bad) == nil {
			t.Fatalf("%q 要拒絕", bad)
		}
	}
}

func sortSignals(in *Input) {
	ss := in.Signals
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j].CreatedAt.Before(ss[j-1].CreatedAt); j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}

// ── 複審（2026-11-24）的五點 ───────────────────────────────────────────

func mkRecs(n int, day string) []Record {
	var out []Record
	for i := 0; i < n; i++ {
		out = append(out, Record{Eval: Eval{RunID: uint64(i), Symbol: "2330"}, Day: day, Group: GroupReconcilable, AMatch: MatchOK})
	}
	return out
}

func TestCoverageExcludesClockBoundaryAndLiveAbnormal(t *testing.T) {
	recs := mkRecs(10, "d")
	recs[0].AMatch, recs[1].AMatch = MatchClockBoundary, MatchLiveAbnormal // 留在分母、⛔ 不算分子 → 8/10 < 90%
	s := Summarize(recs, []string{"d"})
	if s.Counted["d"] != 8 || s.CoverageOK || s.Comparable || s.Pass {
		t.Fatalf("時鐘邊界與 live 異常⛔ 不算進覆蓋率分子：counted=%d ok=%v", s.Counted["d"], s.CoverageOK)
	}
	var buf bytes.Buffer
	WriteReport(&buf, recs, []string{"d"}, nil, nil)
	if strings.Contains(buf.String(), "## A／B 比較") || !strings.Contains(buf.String(), "覆蓋率沒有達到門檻") {
		t.Fatalf("覆蓋率沒達到要停止比較：\n%s", buf.String())
	}
}

func TestBRegressionsFailVerdict(t *testing.T) {
	sb := func(at time.Time) *store.Signal {
		return &store.Signal{Symbol: "2330", SignalType: "SUPPORT_BOUNCE", Support: 100, CreatedAt: at}
	}
	t.Run("B 15 分鐘內重複", func(t *testing.T) {
		recs := mkRecs(10, "d")
		recs[0].B = Outcome{Status: "signal_sent", Candidate: sb(base), Sent: sb(base)}
		recs[1].B = Outcome{Status: "signal_sent", Candidate: sb(base.Add(14 * time.Minute)), Sent: sb(base.Add(14 * time.Minute))}
		if s := Summarize(recs, []string{"d"}); s.Pass || !strings.Contains(strings.Join(s.Failures, ""), "15 分鐘內的重複") {
			t.Fatalf("B 的短時間重複要判失敗：%+v", s.Failures)
		}
	})
	t.Run("Withhold 後送出", func(t *testing.T) {
		recs := mkRecs(10, "d")
		recs[0].EvalStatus, recs[0].Withhold = "signal_suppressed", true
		recs[0].B = Outcome{Status: "signal_sent", Candidate: sb(base), Sent: sb(base)}
		if s := Summarize(recs, []string{"d"}); s.Pass || !strings.Contains(strings.Join(s.Failures, ""), "資料延遲") {
			t.Fatalf("Withhold 違規要判失敗：%+v", s.Failures)
		}
	})
	t.Run("非目標差異", func(t *testing.T) {
		recs := mkRecs(10, "d")
		recs[0].A = Outcome{Candidate: &store.Signal{SignalType: "BREAKDOWN"}}
		recs[0].NonTargetDiff = "A signal_sent、B signal_suppressed"
		if s := Summarize(recs, []string{"d"}); s.Pass {
			t.Fatal("無法證實的非目標差異要判失敗")
		}
		recs[0].NonTargetExplained = true
		if s := Summarize(recs, []string{"d"}); !s.Pass {
			t.Fatalf("證實來自最近 20 筆的邊界才通過：%+v", s.Failures)
		}
	})
}

func TestExplainRecent20(t *testing.T) {
	cand := &store.Signal{Symbol: "2330", SignalType: "BREAKDOWN", Direction: "SELL", Support: 99.5, Timestamp: base.Add(10 * time.Minute)}
	key := signal.PriceKeyIdentity(cand)
	blocker := store.Signal{Symbol: "2330", SignalType: "BREAKDOWN", Direction: "SELL", Support: 99.5, Timestamp: base}
	filler := func(n int) []store.Signal {
		var out []store.Signal
		for i := 0; i < n; i++ {
			out = append(out, store.Signal{Symbol: "2330", SignalType: "SUPPORT_BOUNCE", Direction: "WATCH", Support: 100, Timestamp: base.Add(time.Duration(9-i) * time.Second)})
		}
		return out
	}
	// B 少了幾筆 SUPPORT_BOUNCE：blocker 落在第 6 筆（20 筆內）→ B 擋下；A 多了 SUPPORT_BOUNCE，blocker 在第 26 筆（20 筆外）→ A 送出。
	histB := append(filler(5), blocker)
	histA := append(filler(25), blocker)
	a := Outcome{Status: "signal_sent", Candidate: cand, Key: key}
	b := Outcome{Status: "signal_suppressed", SuppressedBy: string(signal.SuppressedByDB), Candidate: cand, Key: key}
	if !explainRecent20(a, b, histA, histB) {
		t.Fatal("這正是最近 20 筆的邊界，要證實")
	}
	if explainRecent20(a, b, histB, histB) {
		t.Fatal("送出那一邊的 blocker 也在 20 筆內：⛔ 不是邊界")
	}
	b.SuppressedBy = string(signal.SuppressedByReservation)
	if explainRecent20(a, b, histA, histB) {
		t.Fatal("被保留鎖擋下：⛔ 不是最近 20 筆的邊界")
	}
}

func TestInjectionFollowsEvalClock(t *testing.T) {
	in := simulateLive(t, []string{"2330"}, 5, fixedTiming)
	var e Eval
	for _, x := range in.Evals {
		if x.RunID == 103 {
			e = x
		}
	}
	daily := time.Date(2026, 11, 22, 16, 0, 0, 0, time.UTC)
	// data_at 之後、eval_at 之前寫入的外部訊號：要在這一輪評估之前就進庫（跟引擎的時鐘同一條時間線），而且這一輪標未確認。
	in.Signals = append(in.Signals, LiveSignal{ID: 700, Symbol: "2330", SignalType: "BREAKDOWN", Direction: "SELL", Price: 99, Support: 99.5,
		Strength: 1, TS: daily, CreatedAt: e.DataAt.Add(50 * time.Millisecond)})
	sortSignals(in)
	r := replay(t, in)
	if x := find(t, r, 103, "2330"); x.Group != GroupUnconfirmed || !strings.Contains(x.GroupReason, "data_at～eval_at") {
		t.Fatalf("data_at～eval_at 之間的寫入要保留未確認：%s %s", x.Group, x.GroupReason)
	}
	var ext, own uint64
	db := r.dbA
	if err := db.Get(&ext, `SELECT id FROM signals WHERE signal_type = 'BREAKDOWN'`); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&own, db.Rebind(`SELECT id FROM signals WHERE signal_type = 'SUPPORT_BOUNCE' AND ts = ?`), e.LastBarTS); err != nil {
		t.Fatal(err)
	}
	if ext > own {
		t.Fatalf("外部訊號要在這一輪評估之前注入：外部 id %d、這一輪送出 id %d", ext, own)
	}
}

func TestClockBoundaryRequiresMatchingContentAndReconciledAnchor(t *testing.T) {
	tm := func(k int, _ string) (time.Duration, time.Duration) {
		if k == 0 {
			return 200 * time.Millisecond, 900 * time.Millisecond
		}
		return 200 * time.Millisecond, 300 * time.Millisecond
	}
	t.Run("這一輪 live 寫入的內容跟 A 的候選不同", func(t *testing.T) {
		in := simulateLive(t, []string{"2330"}, 5, tm)
		for i := range in.Signals {
			if in.Signals[i].TS.Equal(base.Add(15 * time.Minute)) {
				in.Signals[i].Support = 98
			}
		}
		if x := find(t, replay(t, in), 103, "2330"); x.AMatch != MatchUnexplained {
			t.Fatalf("內容不同⛔ 不能用時鐘解釋：%s（%s）", x.AMatch, x.MatchDetail)
		}
	})
	t.Run("前一筆錨點沒有對帳", func(t *testing.T) {
		in := simulateLive(t, []string{"2330"}, 5, tm)
		for i := range in.Signals {
			if in.Signals[i].TS.Equal(base) {
				in.Signals[i].Price += 1 // 前一筆（run 100）live 的內容跟重播不同
			}
		}
		r := replay(t, in)
		if x := find(t, r, 100, "2330"); x.AMatch != MatchUnexplained {
			t.Fatalf("前提：run 100 對不上：%s", x.AMatch)
		}
		if x := find(t, r, 103, "2330"); x.AMatch == MatchClockBoundary {
			t.Fatalf("錨點沒有對帳⛔ 不能用時鐘解釋：%s（%s）", x.AMatch, x.MatchDetail)
		}
	})
	t.Run("前一筆錨點內容一致、但那一輪歷史未確認", func(t *testing.T) {
		in := simulateLive(t, []string{"2330"}, 5, tm)
		// 08:50 的日 K（起始歷史）讓 run 100（09:00）落到歷史未確認；run 103（09:15）已經在 15 分鐘之外、照常可對帳。
		in.Signals = append(in.Signals, LiveSignal{ID: 600, Symbol: "2330", SignalType: "BREAKDOWN", Direction: "SELL", Price: 99, Support: 99.5,
			Strength: 1, TS: time.Date(2026, 11, 22, 16, 0, 0, 0, time.UTC), CreatedAt: base.Add(-10 * time.Minute)})
		sortSignals(in)
		r := replay(t, in)
		if x := find(t, r, 100, "2330"); x.Group != GroupUnconfirmed || !r.aSentOK[x.key()] {
			t.Fatalf("前提：run 100 內容一致但未確認：%s aSentOK=%v", x.Group, r.aSentOK[x.key()])
		}
		if x := find(t, r, 103, "2330"); x.Group != GroupReconcilable || x.AMatch != MatchUnexplained {
			t.Fatalf("錨點那一輪沒有計入可對帳⛔ 不能用時鐘解釋：%s %s（%s）", x.Group, x.AMatch, x.MatchDetail)
		}
	})
}

// ── 複審（2026-11-24 第二輪）的三點 ────────────────────────────────────

func TestPrecheckRequiresExpectedDaysComplete(t *testing.T) {
	in := simulateLive(t, []string{"2330"}, 4, fixedTiming)
	if p := Precheck(in); !p.OK() {
		t.Fatalf("前提：完整的一天要通過：%v", p.Failures)
	}
	cases := map[string]func(*Input){
		"整個窗口是空的":  func(x *Input) { x.Evals = nil },
		"缺一個預定交易日": func(x *Input) { x.ExpectedDays = []string{"2026-11-23", "2026-11-24"} },
		"清單外的日期":   func(x *Input) { x.ExpectedDays = []string{"2026-11-24"} },
		"沒有清單":     func(x *Input) { x.ExpectedDays = nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			x := *in
			mutate(&x)
			p := Precheck(&x)
			var buf bytes.Buffer
			if p.OK() || WritePrecheck(&buf, p) {
				t.Fatalf("要判不通過：%v\n%s", p.Failures, buf.String())
			}
		})
	}
	t.Run("正式判定也依清單", func(t *testing.T) {
		recs := mkRecs(10, "2026-11-23")
		if s := Summarize(recs, []string{"2026-11-23", "2026-11-24"}); s.CoverageOK || s.Pass || !strings.Contains(strings.Join(s.Failures, ""), "2026-11-24") {
			t.Fatalf("缺一天要判不通過：%v", s.Failures)
		}
		if s := Summarize(nil, []string{"2026-11-23"}); s.Pass {
			t.Fatal("沒有任何評估要判不通過")
		}
	})
}

func TestParseExpectedDays(t *testing.T) {
	from := base.Add(-9 * time.Hour) // 台北 11-23 00:00（入口傳進來的是台北日期的零點）
	to := from.Add(4 * 24 * time.Hour)
	got, err := ParseExpectedDays(strings.NewReader("# 新版窗口\n2026-11-24\n\n2026-11-23\n"), from, to)
	if err != nil || strings.Join(got, ",") != "2026-11-23,2026-11-24" {
		t.Fatalf("%v %v", got, err)
	}
	for _, bad := range []string{"", "2026-11-22\n", "2026-11-23\n2026-11-23\n", "2026/11/23\n", "2026-12-01\n"} {
		if _, err := ParseExpectedDays(strings.NewReader(bad), from, to); err == nil {
			t.Fatalf("%q 要拒絕", bad)
		}
	}
}

func TestWithholdViolationAnySend(t *testing.T) {
	sent := &store.Signal{Symbol: "2330", SignalType: "SUPPORT_BOUNCE", Support: 100, CreatedAt: base}
	for name, b := range map[string]Outcome{
		"live 記 withheld_stale、B 送出": {Status: "signal_sent", Candidate: sent, Sent: sent},
		"B 送出但狀態是 degraded":          {Status: "degraded", Candidate: sent, Sent: sent},
	} {
		t.Run(name, func(t *testing.T) {
			recs := mkRecs(10, "d")
			recs[0].EvalStatus, recs[0].Withhold, recs[0].B = "withheld_stale", true, b
			if s := Summarize(recs, []string{"d"}); s.Pass || !strings.Contains(strings.Join(s.Failures, ""), "資料延遲") {
				t.Fatalf("資料延遲時 B 送出就要判失敗：%v", s.Failures)
			}
		})
	}
}

func TestNonTargetDiffFromEitherSide(t *testing.T) {
	bo := &store.Signal{Symbol: "2330", SignalType: "BREAKOUT", Direction: "BUY", Price: 106, Resistance: 105, Timestamp: base}
	rec := Record{Eval: Eval{RunID: 1, Symbol: "2330"}, Day: "d", Group: GroupReconcilable, AMatch: MatchOK,
		A: Outcome{Status: "no_signal"}, B: Outcome{Status: "signal_sent", Candidate: bo, Sent: bo}}
	markNonTarget(&rec, nil, nil, true)
	if rec.NonTargetDiff == "" || rec.NonTargetExplained {
		t.Fatalf("A 沒有候選、B 送出 BREAKOUT 要記差異且未解釋：%+v", rec)
	}
	recs := append(mkRecs(9, "d"), rec)
	if s := Summarize(recs, []string{"d"}); s.Pass || len(s.NonTarget) != 1 {
		t.Fatalf("要判不通過：%v", s.Failures)
	}
	var buf bytes.Buffer
	WriteReport(&buf, recs, []string{"d"}, nil, nil) // A 那一邊沒有候選：報告⛔ 不能 panic
	if !strings.Contains(buf.String(), "BREAKOUT") {
		t.Fatalf("逐筆要列出：\n%s", buf.String())
	}
}

func TestNonTargetDiffOneSidedSendWithSameStatus(t *testing.T) {
	// 複審（2026-11-24 第三輪）：A、B 都是 degraded，但只有 B 真的送出 BREAKOUT——狀態相同也要記差異、判不通過。
	bo := &store.Signal{Symbol: "2330", SignalType: "BREAKOUT", Direction: "BUY", Price: 106, Resistance: 105, Timestamp: base}
	for name, ab := range map[string][2]Outcome{
		"只有 B 送出": {{Status: "degraded", Candidate: bo}, {Status: "degraded", Candidate: bo, Sent: bo}},
		"只有 A 送出": {{Status: "degraded", Candidate: bo, Sent: bo}, {Status: "degraded", Candidate: bo}},
	} {
		t.Run(name, func(t *testing.T) {
			rec := Record{Eval: Eval{RunID: 1, Symbol: "2330"}, Day: "d", Group: GroupReconcilable, AMatch: MatchOK, A: ab[0], B: ab[1]}
			markNonTarget(&rec, nil, nil, true)
			if !strings.Contains(rec.NonTargetDiff, "只有") || rec.NonTargetExplained {
				t.Fatalf("單側送出要記差異且未解釋：%+v", rec)
			}
			if s := Summarize(append(mkRecs(9, "d"), rec), []string{"d"}); s.Pass {
				t.Fatal("要判不通過")
			}
		})
	}
	same := Record{Eval: Eval{RunID: 2, Symbol: "2330"}, Day: "d", A: Outcome{Status: "degraded", Candidate: bo, Sent: bo}, B: Outcome{Status: "degraded", Candidate: bo, Sent: bo}}
	if markNonTarget(&same, nil, nil, true); same.NonTargetDiff != "" {
		t.Fatalf("兩邊都送出、內容相同：⛔ 不是差異：%q", same.NonTargetDiff)
	}
}

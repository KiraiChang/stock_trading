package analysis

import (
	"database/sql"
	"testing"
	"time"

	"github.com/trading/backend/internal/store"
)

func timelineDay(d int) time.Time {
	return time.Date(2026, 8, d, 0, 0, 0, 0, time.UTC)
}

type stateOpt func(*store.MarketEventState)

func withResolvedBy(by string) stateOpt {
	return func(s *store.MarketEventState) {
		s.ResolvedBy = store.NullString{NullString: sql.NullString{String: by, Valid: true}}
	}
}
func withLatestType(t string) stateOpt {
	return func(s *store.MarketEventState) { s.LatestEventType = t }
}
func withActive(a bool) stateOpt {
	return func(s *store.MarketEventState) { s.Active = a }
}
func withZone(z string) stateOpt {
	return func(s *store.MarketEventState) { s.ZoneKey = z }
}
func withFamily(f string) stateOpt {
	return func(s *store.MarketEventState) { s.EventFamily = f }
}

// st 造一列狀態快照。預設是同一個 zone／family，讓測試只需標出真正在變的欄位。
func st(analysisID uint64, at time.Time, state string, opts ...stateOpt) store.MarketEventState {
	s := store.MarketEventState{
		AnalysisID:      analysisID,
		Symbol:          "2330",
		Timeframe:       "1d",
		AnalyzedAt:      at,
		ZoneKey:         "S-1000",
		EventFamily:     "SUPPORT_RECLAIM",
		EventType:       "INTRADAY_RECLAIM",
		RootEventType:   "INTRADAY_RECLAIM",
		LatestEventType: "INTRADAY_RECLAIM",
		Direction:       "BULLISH",
		State:           state,
		Active:          state == "ACTIVE" || state == "CONFIRMED",
		ReasonCodes:     store.RawJSON(`["CLOSE_RECLAIM"]`),
	}
	for _, o := range opts {
		o(&s)
	}
	return s
}

// 分析每天都跑，但多數日子事件沒有變化。若每份快照都記一筆 transition，
// 鏈會被「什麼都沒發生」淹沒而失去可讀性。
func TestBuildEventTimelineSkipsUnchangedSnapshots(t *testing.T) {
	tl := BuildEventTimeline("2330", "1d", []store.MarketEventState{
		st(1, timelineDay(1), "CANDIDATE"),
		st(2, timelineDay(2), "CANDIDATE"),
		st(3, timelineDay(3), "CANDIDATE"),
	}, nil)

	if len(tl.Chains) != 1 {
		t.Fatalf("chain 數 = %d, want 1", len(tl.Chains))
	}
	if got := len(tl.Chains[0].Transitions); got != 1 {
		t.Errorf("transition 數 = %d, want 1——三份相同快照只該有起點那一筆", got)
	}
	// 但 last_seen_at 要跟著推進，否則會看起來像鏈在第一天就停了
	if !tl.Chains[0].LastSeenAt.Equal(timelineDay(3)) {
		t.Errorf("LastSeenAt = %v, want %v", tl.Chains[0].LastSeenAt, timelineDay(3))
	}
	if len(tl.Snapshots) != 3 {
		t.Errorf("snapshot 數 = %d, want 3——快照本身要全數揭露", len(tl.Snapshots))
	}
}

func TestBuildEventTimelineRecordsTransitions(t *testing.T) {
	cases := []struct {
		name        string
		second      store.MarketEventState
		wantChanged []string
	}{
		{"state 改變", st(2, timelineDay(2), "CONFIRMED"), []string{"state", "active"}},
		{"只有 active 改變", st(2, timelineDay(2), "CANDIDATE", withActive(true)), []string{"active"}},
		{"latest_event_type 改變", st(2, timelineDay(2), "CANDIDATE", withLatestType("CLOSE_RECLAIM")), []string{"latest_event_type"}},
		{"resolved_by 出現", st(2, timelineDay(2), "CANDIDATE", withResolvedBy("HIGH_VOLUME_BREAKDOWN")), []string{"resolved_by"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tl := BuildEventTimeline("2330", "1d", []store.MarketEventState{
				st(1, timelineDay(1), "CANDIDATE"), tc.second,
			}, nil)
			if len(tl.Chains) != 1 || len(tl.Chains[0].Transitions) != 2 {
				t.Fatalf("want 1 chain / 2 transitions, got %d chain", len(tl.Chains))
			}
			got := tl.Chains[0].Transitions[1].Changed
			if len(got) != len(tc.wantChanged) {
				t.Fatalf("changed = %v, want %v", got, tc.wantChanged)
			}
			for i, w := range tc.wantChanged {
				if got[i] != w {
					t.Errorf("changed[%d] = %q, want %q", i, got[i], w)
				}
			}
		})
	}
}

// 終結後同一個 (zone, family) 再出現算**新的一條鏈**，不是把舊鏈接下去。
// 否則「這個 zone 被測試過三次」會被摺成一條看不出次數的長鏈。
func TestBuildEventTimelineStartsNewChainAfterClose(t *testing.T) {
	for _, closing := range []string{"RESOLVED", "EXPIRED"} {
		t.Run(closing, func(t *testing.T) {
			tl := BuildEventTimeline("2330", "1d", []store.MarketEventState{
				st(1, timelineDay(1), "CANDIDATE"),
				st(2, timelineDay(2), closing),
				st(3, timelineDay(3), "CANDIDATE"),
			}, nil)
			if len(tl.Chains) != 2 {
				t.Fatalf("chain 數 = %d, want 2——終結後再出現應是新的一條", len(tl.Chains))
			}
			if !tl.Chains[0].Closed || tl.Chains[0].FinalState != closing {
				t.Errorf("第一條鏈未標記終結：closed=%v final=%q", tl.Chains[0].Closed, tl.Chains[0].FinalState)
			}
			if tl.Chains[1].Closed {
				t.Error("第二條鏈不該是終結狀態")
			}
			if !tl.Chains[1].FirstSeenAt.Equal(timelineDay(3)) {
				t.Errorf("第二條鏈 FirstSeenAt = %v, want %v", tl.Chains[1].FirstSeenAt, timelineDay(3))
			}
		})
	}
}

func TestBuildEventTimelineSeparatesChainsByZoneAndFamily(t *testing.T) {
	tl := BuildEventTimeline("2330", "1d", []store.MarketEventState{
		st(1, timelineDay(1), "CANDIDATE"),
		st(1, timelineDay(1), "CANDIDATE", withZone("S-900")),
		st(1, timelineDay(1), "CANDIDATE", withFamily("VOLUME")),
	}, nil)
	if len(tl.Chains) != 3 {
		t.Fatalf("chain 數 = %d, want 3——(zone, family) 不同就是不同鏈", len(tl.Chains))
	}
}

// timeline 的解析度等於分析頻率，而目前沒有任何排程會產生分析（見 todo.md T-045）。
// 鏈上的空白**不代表那段期間沒有事件**，只代表沒有分析——不揭露 gap 會被讀成風平浪靜。
func TestBuildEventTimelineExposesSnapshotGaps(t *testing.T) {
	tl := BuildEventTimeline("2330", "1d", []store.MarketEventState{
		st(1, timelineDay(1), "CANDIDATE"),
		st(2, timelineDay(8), "CONFIRMED"),
	}, nil)

	if len(tl.Snapshots) != 2 {
		t.Fatalf("snapshot 數 = %d, want 2", len(tl.Snapshots))
	}
	if tl.Snapshots[0].GapDays != 0 {
		t.Errorf("第一筆 GapDays = %d, want 0", tl.Snapshots[0].GapDays)
	}
	if tl.Snapshots[1].GapDays != 7 {
		t.Errorf("GapDays = %d, want 7——中間六天沒有分析必須看得出來", tl.Snapshots[1].GapDays)
	}
}

// 摺疊結果必須是決定性的：同一份資料以任何輸入順序進來都要得到同一條鏈，
// 否則同一個查詢兩次會顯示不同的 timeline。
func TestBuildEventTimelineIsOrderIndependent(t *testing.T) {
	rows := []store.MarketEventState{
		st(3, timelineDay(3), "RESOLVED", withResolvedBy("HIGH_VOLUME_BREAKDOWN")),
		st(1, timelineDay(1), "CANDIDATE"),
		st(2, timelineDay(2), "CONFIRMED"),
	}
	shuffled := []store.MarketEventState{rows[1], rows[0], rows[2]}

	a := BuildEventTimeline("2330", "1d", rows, nil)
	b := BuildEventTimeline("2330", "1d", shuffled, nil)

	if len(a.Chains) != 1 || len(b.Chains) != 1 {
		t.Fatalf("chain 數不符：%d / %d", len(a.Chains), len(b.Chains))
	}
	if len(a.Chains[0].Transitions) != len(b.Chains[0].Transitions) {
		t.Fatalf("transition 數不同：%d vs %d", len(a.Chains[0].Transitions), len(b.Chains[0].Transitions))
	}
	for i := range a.Chains[0].Transitions {
		if a.Chains[0].Transitions[i].State != b.Chains[0].Transitions[i].State {
			t.Errorf("第 %d 步狀態不同：%q vs %q", i,
				a.Chains[0].Transitions[i].State, b.Chains[0].Transitions[i].State)
		}
	}
}

func TestBuildEventTimelineEmptyInput(t *testing.T) {
	tl := BuildEventTimeline("2330", "1d", nil, nil)
	if tl.Chains == nil || tl.Snapshots == nil {
		t.Error("空輸入時 Chains／Snapshots 應為空陣列而非 nil——序列化成 null 會讓前端 .map() 爆掉")
	}
}

func TestBuildEventTimelineKeepsAnalysisSnapshotsWhenNoEvents(t *testing.T) {
	analyses := []store.AnalysisSnapshot{
		{ID: 10, AnalyzedAt: timelineDay(1)},
		{ID: 11, AnalyzedAt: timelineDay(3)},
	}
	tl := BuildEventTimeline("2330", "1d", nil, analyses)

	if len(tl.Chains) != 0 {
		t.Fatalf("沒有事件列時 chain 數 = %d, want 0", len(tl.Chains))
	}
	if len(tl.Snapshots) != 2 {
		t.Fatalf("snapshot 數 = %d, want 2——有分析但沒有事件仍是有效觀測", len(tl.Snapshots))
	}
	if tl.Snapshots[1].GapDays != 2 {
		t.Errorf("第二份快照 GapDays = %d, want 2", tl.Snapshots[1].GapDays)
	}
}

func TestDecodeReasonCodesTolerantOfBadJSON(t *testing.T) {
	if got := decodeReasonCodes(store.RawJSON("not json")); got != nil {
		t.Errorf("壞掉的 JSON 應回 nil 而不是報錯，實得 %v", got)
	}
	if got := decodeReasonCodes(store.RawJSON(`["A","B"]`)); len(got) != 2 {
		t.Errorf("正常解析失敗：%v", got)
	}
}

// TestBuildEventTimelineIgnoresTombstoneRepeats：**用 live 資料實測才發現的情況**。
//
// 事件終結後，狀態表會把那筆 EXPIRED／RESOLVED 一直帶在後續每一份快照裡——
// 0050 的 SUPPORT:92.1361:100.5139 在 2026-07-23 轉 EXPIRED 之後，
// 8/03～8/12 的每份快照都還在回報同一筆。
// 若把「已終結的鍵再出現」一律當成新事件，每份快照都會生出一條垃圾鏈。
func TestBuildEventTimelineIgnoresTombstoneRepeats(t *testing.T) {
	tl := BuildEventTimeline("0050", "1d", []store.MarketEventState{
		st(1, timelineDay(1), "ACTIVE"),
		st(2, timelineDay(2), "EXPIRED"),
		st(3, timelineDay(3), "EXPIRED"), // 墓碑
		st(4, timelineDay(4), "EXPIRED"), // 墓碑
		st(5, timelineDay(5), "EXPIRED"), // 墓碑
	}, nil)

	if len(tl.Chains) != 1 {
		t.Fatalf("chain 數 = %d, want 1——重複回報的終結狀態是墓碑，不該各開一條鏈", len(tl.Chains))
	}
	c := tl.Chains[0]
	if len(c.Transitions) != 2 {
		t.Errorf("transition 數 = %d, want 2（ACTIVE → EXPIRED）", len(c.Transitions))
	}
	if !c.Closed || c.FinalState != "EXPIRED" {
		t.Errorf("鏈未正確標記終結：closed=%v final=%q", c.Closed, c.FinalState)
	}
	// 墓碑仍要推進 LastSeenAt——那代表「這條鏈到這天都還看得到」
	if !c.LastSeenAt.Equal(timelineDay(5)) {
		t.Errorf("LastSeenAt = %v, want %v", c.LastSeenAt, timelineDay(5))
	}
}

// 終結之後出現**非終結**狀態才是真的新事件，這時才開新鏈。
func TestBuildEventTimelineNewChainOnlyForNonTerminalState(t *testing.T) {
	tl := BuildEventTimeline("0050", "1d", []store.MarketEventState{
		st(1, timelineDay(1), "ACTIVE"),
		st(2, timelineDay(2), "EXPIRED"),
		st(3, timelineDay(3), "EXPIRED"),   // 墓碑，不開新鏈
		st(4, timelineDay(4), "CANDIDATE"), // 真的有新事件
	}, nil)
	if len(tl.Chains) != 2 {
		t.Fatalf("chain 數 = %d, want 2", len(tl.Chains))
	}
	if !tl.Chains[1].FirstSeenAt.Equal(timelineDay(4)) {
		t.Errorf("新鏈 FirstSeenAt = %v, want %v", tl.Chains[1].FirstSeenAt, timelineDay(4))
	}
}

// 第一次看到就已經是終結狀態：我們是在事件結束後才開始觀測，
// 仍要記一條已終結的鏈，否則這段歷史會完全消失。
// 0050 的 SUPPORT:103.4487:104.0713 / SUPPORT_BREAKDOWN 就是這個形狀（首見即 RESOLVED）。
func TestBuildEventTimelineKeepsChainFirstSeenAsClosed(t *testing.T) {
	tl := BuildEventTimeline("0050", "1d", []store.MarketEventState{
		st(1, timelineDay(1), "RESOLVED", withResolvedBy("INTRADAY_RECLAIM")),
		st(2, timelineDay(2), "RESOLVED", withResolvedBy("INTRADAY_RECLAIM")),
	}, nil)
	if len(tl.Chains) != 1 {
		t.Fatalf("chain 數 = %d, want 1", len(tl.Chains))
	}
	if !tl.Chains[0].Closed {
		t.Error("首見即終結的鏈應標記 closed")
	}
	if len(tl.Chains[0].Transitions) != 1 {
		t.Errorf("transition 數 = %d, want 1", len(tl.Chains[0].Transitions))
	}
}

// TestBuildEventTimelineRecordsPostCloseTransition：終結之後仍可能有真實的狀態變化。
//
// 最典型的是 RESOLVED 老化成 EXPIRED——event_engine.py 的
// `_normalize_previous_event_state` 在 age_bars 達門檻時會把 carried 的 RESOLVED 翻成
// EXPIRED。若把終結後的每一列都當墓碑吞掉，鏈的 final_state 會永遠停在 RESOLVED，
// 與 DB 裡的最新狀態矛盾。
func TestBuildEventTimelineRecordsPostCloseTransition(t *testing.T) {
	tl := BuildEventTimeline("0050", "1d", []store.MarketEventState{
		st(1, timelineDay(1), "ACTIVE"),
		st(2, timelineDay(2), "RESOLVED", withResolvedBy("INTRADAY_RECLAIM")),
		st(3, timelineDay(3), "RESOLVED", withResolvedBy("INTRADAY_RECLAIM")), // 墓碑
		st(4, timelineDay(4), "EXPIRED", withResolvedBy("INTRADAY_RECLAIM")),  // 老化，真實變化
	}, nil)

	if len(tl.Chains) != 1 {
		t.Fatalf("chain 數 = %d, want 1", len(tl.Chains))
	}
	c := tl.Chains[0]
	if c.FinalState != "EXPIRED" {
		t.Errorf("FinalState = %q, want EXPIRED——終結後的老化被吞掉了", c.FinalState)
	}
	// ACTIVE → RESOLVED → EXPIRED，中間那筆相同的墓碑不算
	if len(c.Transitions) != 3 {
		t.Errorf("transition 數 = %d, want 3（含終結後的 RESOLVED→EXPIRED）", len(c.Transitions))
	}
}

// TestBuildEventTimelineSnapshotsUseAllAnalyses：snapshots 必須反映**所有**分析，
// 不是只有留下事件狀態列的那幾次。
//
// 實測 0050 有 14 次分析、只有 11 次產生事件列——用 rows 推導會把那 3 次報成
// 「沒有觀測」，而 snapshots/gap_days 正是揭露觀測缺口的唯一依據。
func TestBuildEventTimelineSnapshotsUseAllAnalyses(t *testing.T) {
	analyses := []store.AnalysisSnapshot{
		{ID: 10, AnalyzedAt: timelineDay(1)}, // 這次分析沒有任何事件
		{ID: 11, AnalyzedAt: timelineDay(2)}, // 這次也沒有
		{ID: 12, AnalyzedAt: timelineDay(3)}, // 事件從這裡才出現
	}
	tl := BuildEventTimeline("0050", "1d", []store.MarketEventState{
		st(12, timelineDay(3), "CANDIDATE"),
	}, analyses)

	if len(tl.Snapshots) != 3 {
		t.Fatalf("快照數 = %d, want 3——沒有事件的分析也是觀測，不能漏", len(tl.Snapshots))
	}
	if !tl.Snapshots[0].AnalyzedAt.Equal(timelineDay(1)) {
		t.Errorf("第一份快照 = %v, want %v——觀測起點被往後挪了",
			tl.Snapshots[0].AnalyzedAt, timelineDay(1))
	}
	// 三次分析連續，不該有任何 gap
	for i, s := range tl.Snapshots {
		want := 0
		if i > 0 {
			want = 1
		}
		if s.GapDays != want {
			t.Errorf("第 %d 份快照 GapDays = %d, want %d", i, s.GapDays, want)
		}
	}
}

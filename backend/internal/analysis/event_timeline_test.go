package analysis

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/trading/backend/internal/store"
)

// Event Timeline 的單元測試（原記於 todo.md T-051，已收斂）。
//
// **2026-08-20 改讀身分層後，原本那批「摺疊快照」的測試整批移除了**——墓碑重複、
// 相鄰快照 diff、終結後開新鏈之類的情況全部消失，因為鏈的邊界不再是讀取時推導出來的，
// 而是 event_instances / event_transitions 存下來的事實。留下來的是 snapshots／gap
// 那幾條（與事件無關，換來源不影響）。

func timelineDay(d int) time.Time {
	return time.Date(2026, 8, d, 13, 30, 0, 0, time.UTC)
}

func chain(uid, family string, seq int, first, last time.Time, opts ...func(*store.EventInstance)) store.EventInstance {
	c := store.EventInstance{
		EventUID:        uid,
		Symbol:          "2330",
		Timeframe:       "1d",
		ZoneUID:         sql.NullString{String: "Z-" + uid, Valid: true},
		ZoneScopeKey:    "Z-" + uid,
		EventScope:      "ZONE",
		EventFamily:     family,
		Seq:             seq,
		RootEventType:   "HIGH_VOLUME_BREAKDOWN",
		LatestEventType: "HIGH_VOLUME_BREAKDOWN",
		State:           "CONFIRMED",
		Active:          true,
		Direction:       "BEARISH",
		FirstSeenAt:     first,
		LastSeenAt:      last,
		LastZoneKey:     sql.NullString{String: "SUPPORT:98.0000:100.0000", Valid: true},
		DecisionVisible: true,
	}
	for _, o := range opts {
		o(&c)
	}
	return c
}

func closedChain(reason string) func(*store.EventInstance) {
	return func(c *store.EventInstance) {
		c.State = "EXPIRED"
		c.Active = false
		c.EndedAt = sql.NullTime{Time: timelineDay(9), Valid: true}
		c.EndReason = sql.NullString{String: reason, Valid: true}
	}
}

// step 產生一筆轉換。**預設帶 AnalyzedAt（K 棒時間）**，因為顯示層一律用它；
// occurred_at 刻意設成一個明顯不同的 wall clock，好讓「用錯軸」的測試會紅。
func step(uid, from, to string, at time.Time, opts ...func(*store.EventTransitionView)) store.EventTransitionView {
	t := store.EventTransitionView{
		EventTransition: store.EventTransition{
			EventUID:    uid,
			ToState:     to,
			OccurredAt:  wallClock,
			ReasonCodes: store.RawJSON(`["SUPPORT_CLOSED_BELOW"]`),
		},
		AnalyzedAt: sql.NullTime{Time: at, Valid: true},
	}
	if from != "" {
		t.FromState = sql.NullString{String: from, Valid: true}
	}
	for _, o := range opts {
		o(&t)
	}
	return t
}

// wallClock 代表「跑分析的那一刻」——身分層存在 occurred_at / first_seen_at 的東西。
// 它與 K 棒日期差了好幾天，任何把兩者搞混的實作都會在斷言上炸開。
var wallClock = time.Date(2026, 9, 30, 9, 36, 55, 0, time.UTC)

func TestBuildEventTimelineMapsChainsAndTransitions(t *testing.T) {
	chains := []store.EventInstance{chain("E1", "SUPPORT_BREAKDOWN", 1, timelineDay(1), timelineDay(3))}
	transitions := []store.EventTransitionView{
		step("E1", "", "CANDIDATE", timelineDay(1)),
		step("E1", "CANDIDATE", "CONFIRMED", timelineDay(2)),
	}

	tl := BuildEventTimeline("2330", "1d", chains, transitions, nil, nil)

	if len(tl.Chains) != 1 {
		t.Fatalf("chain 數 = %d, want 1", len(tl.Chains))
	}
	c := tl.Chains[0]
	if c.EventUID != "E1" || c.ZoneUID == nil || *c.ZoneUID != "Z-E1" || c.Seq != 1 {
		t.Fatalf("鏈身分不對：%+v", c)
	}
	if c.ZoneKey == nil || *c.ZoneKey != "SUPPORT:98.0000:100.0000" {
		t.Errorf("zone_key 應輸出 last_zone_key（人工比對用），得到 %v", c.ZoneKey)
	}
	if len(c.Transitions) != 2 {
		t.Fatalf("transition 數 = %d, want 2", len(c.Transitions))
	}
	// **from_state 留白 ＝ 鏈的誕生**，這是 event_transitions 的不變式。
	if c.Transitions[0].FromState != "" {
		t.Errorf("誕生那步的 from_state 應留白，得到 %q", c.Transitions[0].FromState)
	}
	if c.Transitions[1].FromState != "CANDIDATE" || c.Transitions[1].State != "CONFIRMED" {
		t.Errorf("第二步應是 CANDIDATE→CONFIRMED，得到 %+v", c.Transitions[1])
	}
	if tl.IdentitySince == nil || !tl.IdentitySince.Equal(timelineDay(1)) {
		t.Errorf("identity_since = %v, want %v", tl.IdentitySince, timelineDay(1))
	}
}

// **同一個 (zone, family) 的 seq 是先後兩條鏈，不能合併。** 前一條終結之後再出現同家族
// 事件，寫入端就是當成新的一條——讀取端合併會讓「這個 zone 被測試過幾次」永遠答錯。
// TestBuildEventTimelineUsesInjectedIdentitySince：注入的全域值**優先於**由 chains
// 推導（原記於 todo.md T-051 R5，已收斂）。max_analyses 的視窗會濾掉早已終結的舊鏈，於是「回傳的鏈裡
// 最早的起點」比身分層真正的起點晚——照鏈推導就會把「這次沒查到」說成「更早的分析
// 沒有事件鏈」。
func TestBuildEventTimelineUsesInjectedIdentitySince(t *testing.T) {
	since := timelineDay(1)
	tl := BuildEventTimeline("2330", "1d",
		[]store.EventInstance{chain("E9", "SUPPORT_BREAKDOWN", 1, timelineDay(20), timelineDay(21))},
		[]store.EventTransitionView{step("E9", "", "CONFIRMED", timelineDay(20))},
		nil, &since)

	if tl.IdentitySince == nil || !tl.IdentitySince.Equal(timelineDay(1)) {
		t.Errorf("identity_since = %v, want 注入值 %v", tl.IdentitySince, timelineDay(1))
	}
	if len(tl.Chains) != 1 || !tl.Chains[0].FirstSeenAt.Equal(timelineDay(20)) {
		t.Error("注入 identity_since 不該影響鏈本身")
	}
}

// TestBuildEventTimelineFallsBackToChainsWithoutInjection：未注入時（沒接身分層的
// 呼叫端）維持既有的推導，不因為新參數而變成 nil。
func TestBuildEventTimelineFallsBackToChainsWithoutInjection(t *testing.T) {
	tl := BuildEventTimeline("2330", "1d",
		[]store.EventInstance{chain("E9", "SUPPORT_BREAKDOWN", 1, timelineDay(20), timelineDay(21))},
		[]store.EventTransitionView{step("E9", "", "CONFIRMED", timelineDay(20))},
		nil, nil)

	if tl.IdentitySince == nil || !tl.IdentitySince.Equal(timelineDay(20)) {
		t.Errorf("identity_since = %v, want 由鏈推導的 %v", tl.IdentitySince, timelineDay(20))
	}
}

func TestBuildEventTimelineKeepsSeqAsSeparateChains(t *testing.T) {
	chains := []store.EventInstance{
		chain("E1", "SUPPORT_BREAKDOWN", 1, timelineDay(1), timelineDay(3), closedChain("EXPIRED")),
		chain("E2", "SUPPORT_BREAKDOWN", 2, timelineDay(5), timelineDay(6)),
	}
	transitions := []store.EventTransitionView{
		step("E1", "", "CONFIRMED", timelineDay(1)),
		step("E2", "", "CANDIDATE", timelineDay(5)),
	}

	tl := BuildEventTimeline("2330", "1d", chains, transitions, nil, nil)

	if len(tl.Chains) != 2 {
		t.Fatalf("chain 數 = %d, want 2（seq 不同就是不同鏈）", len(tl.Chains))
	}
	if tl.Chains[0].Seq != 1 || tl.Chains[1].Seq != 2 {
		t.Errorf("seq 應照 first_seen_at 由舊到新：%d, %d", tl.Chains[0].Seq, tl.Chains[1].Seq)
	}
}

// ZONE_IDENTITY_ENDED 是「zone 身分終止所以鏈收攤」，不是事件自己走完生命週期。
// 前端把它畫成一般結束會誤導，所以必須輸出得出來。
func TestBuildEventTimelineExposesZoneIdentityEnded(t *testing.T) {
	chains := []store.EventInstance{
		chain("E1", "SUPPORT_RECLAIM", 1, timelineDay(1), timelineDay(3), closedChain("ZONE_IDENTITY_ENDED")),
	}

	tl := BuildEventTimeline("2330", "1d", chains, nil, nil, nil)

	c := tl.Chains[0]
	if !c.Closed {
		t.Error("ended_at 有值時 closed 應為 true")
	}
	if c.EndReason != "ZONE_IDENTITY_ENDED" {
		t.Errorf("end_reason = %q, want ZONE_IDENTITY_ENDED", c.EndReason)
	}
}

// SYMBOL scope 的事件不屬於任何 zone，zone_uid 是 NULL。取值不能 panic，
// 輸出要是 nil（序列化成 JSON null），而不是 "SYMBOL" 這個投影鍵、也不是空字串。
func TestBuildEventTimelineHandlesSymbolScopeChain(t *testing.T) {
	c := chain("E1", "VOLUME_CONTEXT", 1, timelineDay(1), timelineDay(1))
	c.ZoneUID = sql.NullString{}
	c.ZoneScopeKey = store.SymbolScopeKey
	c.EventScope = "SYMBOL"
	c.LastZoneKey = sql.NullString{}

	tl := BuildEventTimeline("2330", "1d", []store.EventInstance{c}, nil, nil, nil)

	if got := tl.Chains[0].ZoneUID; got != nil {
		t.Errorf("SYMBOL scope 的 zone_uid 應為 nil，得到 %q", *got)
	}
	if got := tl.Chains[0].EventScope; got != "SYMBOL" {
		t.Errorf("event_scope = %q, want SYMBOL", got)
	}
}

// 輸出必須是決定性的：同一份資料以任何輸入順序進來都要得到同一份 timeline，
// 否則同一個查詢兩次會顯示不同結果。
func TestBuildEventTimelineIsOrderIndependent(t *testing.T) {
	chains := []store.EventInstance{
		chain("E2", "SUPPORT_RECLAIM", 1, timelineDay(5), timelineDay(6)),
		chain("E1", "SUPPORT_BREAKDOWN", 1, timelineDay(1), timelineDay(3)),
	}
	transitions := []store.EventTransitionView{
		step("E1", "CANDIDATE", "CONFIRMED", timelineDay(2)),
		step("E1", "", "CANDIDATE", timelineDay(1)),
	}

	a := BuildEventTimeline("2330", "1d", chains, transitions, nil, nil)
	b := BuildEventTimeline("2330", "1d",
		[]store.EventInstance{chains[1], chains[0]},
		[]store.EventTransitionView{transitions[1], transitions[0]}, nil, nil)

	if a.Chains[0].EventUID != "E1" || b.Chains[0].EventUID != "E1" {
		t.Fatalf("鏈應依 first_seen_at 排序：%q / %q", a.Chains[0].EventUID, b.Chains[0].EventUID)
	}
	for i := range a.Chains[0].Transitions {
		if a.Chains[0].Transitions[i].State != b.Chains[0].Transitions[i].State {
			t.Errorf("第 %d 步狀態不同：%q vs %q", i,
				a.Chains[0].Transitions[i].State, b.Chains[0].Transitions[i].State)
		}
	}
}

// 鏈存在但沒有任何轉換：那是寫入端的單一交易應該擋掉的情況。
// 讀取端不吞掉它——鏈照樣輸出（空 transitions），讓它在畫面上看得見而不是靜靜消失。
func TestBuildEventTimelineKeepsChainWithoutTransitions(t *testing.T) {
	tl := BuildEventTimeline("2330", "1d",
		[]store.EventInstance{chain("E1", "SUPPORT_BREAKDOWN", 1, timelineDay(1), timelineDay(1))},
		nil, nil, nil)

	if len(tl.Chains) != 1 {
		t.Fatalf("chain 數 = %d, want 1", len(tl.Chains))
	}
	if tl.Chains[0].Transitions == nil {
		t.Error("Transitions 應為空陣列而非 nil——序列化成 null 會讓前端 .map() 爆掉")
	}
}

// ── 以下與事件無關，換來源不影響 ──────────────────────────

func TestBuildEventTimelineExposesSnapshotGaps(t *testing.T) {
	tl := BuildEventTimeline("2330", "1d", nil, nil, []store.AnalysisSnapshot{
		{ID: 1, AnalyzedAt: timelineDay(1)},
		{ID: 2, AnalyzedAt: timelineDay(8)},
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

func TestBuildEventTimelineEmptyInput(t *testing.T) {
	tl := BuildEventTimeline("2330", "1d", nil, nil, nil, nil)
	if tl.Chains == nil || tl.Snapshots == nil {
		t.Error("空輸入時 Chains／Snapshots 應為空陣列而非 nil——序列化成 null 會讓前端 .map() 爆掉")
	}
	if tl.IdentitySince != nil {
		t.Error("沒有任何鏈時 identity_since 應為 nil")
	}
}

// **有分析但沒有事件仍是有效觀測。** 少了這條，畫面上的空白會被讀成「風平浪靜」，
// 而實際上可能是「那幾天根本沒跑分析」。
func TestBuildEventTimelineKeepsAnalysisSnapshotsWhenNoChains(t *testing.T) {
	tl := BuildEventTimeline("2330", "1d", nil, nil, []store.AnalysisSnapshot{
		{ID: 10, AnalyzedAt: timelineDay(1)},
		{ID: 11, AnalyzedAt: timelineDay(3)},
	}, nil)

	if len(tl.Chains) != 0 {
		t.Fatalf("沒有鏈時 chain 數 = %d, want 0", len(tl.Chains))
	}
	if len(tl.Snapshots) != 2 {
		t.Fatalf("snapshot 數 = %d, want 2", len(tl.Snapshots))
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

// **JSON 層的形狀要釘住。** `zone_uid` 判斷 SYMBOL scope 的自然寫法是
// 「欄位存在但為 null」，若序列化後鍵直接消失，那個判斷會靜默走到 undefined 分支。
// Go 端看到的是 `*string(nil)`，兩者在 struct 上分不出來——只有序列化才驗得到。
func TestEventTimelineJSONKeepsNullZoneUID(t *testing.T) {
	symbolScope := chain("E1", "VOLUME_CONTEXT", 1, timelineDay(1), timelineDay(1))
	symbolScope.ZoneUID = sql.NullString{}
	symbolScope.LastZoneKey = sql.NullString{}
	symbolScope.EventScope = "SYMBOL"

	tl := BuildEventTimeline("2330", "1d", []store.EventInstance{
		symbolScope,
		chain("E2", "SUPPORT_BREAKDOWN", 1, timelineDay(2), timelineDay(2)),
	}, nil, nil, nil)

	raw, err := json.Marshal(tl)
	if err != nil {
		t.Fatalf("marshal 失敗: %v", err)
	}
	var decoded struct {
		Chains []map[string]any `json:"chains"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal 失敗: %v", err)
	}

	for _, key := range []string{"zone_uid", "zone_key"} {
		v, ok := decoded.Chains[0][key]
		if !ok {
			t.Fatalf("SYMBOL scope 的 %s 鍵不該消失——消費端會分不出 null 與 undefined", key)
		}
		if v != nil {
			t.Errorf("SYMBOL scope 的 %s 應為 null，得到 %v", key, v)
		}
	}
	if got := decoded.Chains[1]["zone_uid"]; got != "Z-E2" {
		t.Errorf("ZONE scope 的 zone_uid = %v, want Z-E2", got)
	}
}

// **時間軸回歸**：身分層存的 occurred_at / first_seen_at 是跑分析的 wall clock，
// 而同一份回應裡的 snapshots 用的是 K 棒日期。兩軸混用時整條鏈會擠在「跑分析的那一刻」
// ——實測 2330 的 28 條鏈曾全部顯示成在同一秒內發生，而 snapshots 橫跨一個月。
func TestBuildEventTimelineUsesCandleAxisNotWallClock(t *testing.T) {
	c := chain("E1", "SUPPORT_BREAKDOWN", 1, wallClock, wallClock)
	tl := BuildEventTimeline("2330", "1d", []store.EventInstance{c}, []store.EventTransitionView{
		step("E1", "", "CANDIDATE", timelineDay(1)),
		step("E1", "CANDIDATE", "CONFIRMED", timelineDay(4)),
	}, nil, nil)

	got := tl.Chains[0]
	if !got.FirstSeenAt.Equal(timelineDay(1)) || !got.LastSeenAt.Equal(timelineDay(4)) {
		t.Fatalf("鏈的起訖應落在 K 棒軸上，得到 %v ~ %v", got.FirstSeenAt, got.LastSeenAt)
	}
	for i, s := range got.Transitions {
		if s.OccurredAt.Equal(wallClock) {
			t.Errorf("第 %d 步用了 wall clock，應該用所屬分析的 K 棒時間", i)
		}
	}
	if tl.IdentitySince == nil || !tl.IdentitySince.Equal(timelineDay(1)) {
		t.Errorf("identity_since 也要在 K 棒軸上，得到 %v", tl.IdentitySince)
	}
}

// analysis_id 為 NULL（鏈由排程收尾而不是某次分析）時沒有 K 棒時間可用，
// 退回身分層的 occurred_at——這是明確的降級，不是預設路徑。
func TestBuildEventTimelineFallsBackToOccurredAtWithoutAnalysis(t *testing.T) {
	orphan := step("E1", "CONFIRMED", "EXPIRED", timelineDay(3))
	orphan.AnalyzedAt = sql.NullTime{}

	tl := BuildEventTimeline("2330", "1d",
		[]store.EventInstance{chain("E1", "SUPPORT_BREAKDOWN", 1, wallClock, wallClock)},
		[]store.EventTransitionView{orphan}, nil, nil)

	if got := tl.Chains[0].Transitions[0].OccurredAt; !got.Equal(wallClock) {
		t.Errorf("沒有 analysis_id 時應退回 occurred_at，得到 %v", got)
	}
}

// 決策可見性要一路帶到回應上，而且 **false 不可以被 omitempty 吃掉**——
// false 正是這個欄位唯一有資訊量的值。少了它，顯示端會把「只寫不讀的事實紀錄」
// （SUPPORT_RETEST / RESISTANCE_BREAKOUT）畫成會影響 Bias 或進場的事件。
func TestEventTimelineJSONAlwaysCarriesDecisionVisible(t *testing.T) {
	invisible := chain("E1", "RESISTANCE_BREAKOUT", 1, timelineDay(1), timelineDay(1))
	invisible.DecisionVisible = false

	tl := BuildEventTimeline("2330", "1d", []store.EventInstance{
		invisible,
		chain("E2", "SUPPORT_BREAKDOWN", 1, timelineDay(2), timelineDay(2)),
	}, nil, nil, nil)

	raw, err := json.Marshal(tl)
	if err != nil {
		t.Fatalf("marshal 失敗: %v", err)
	}
	var decoded struct {
		Chains []map[string]any `json:"chains"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal 失敗: %v", err)
	}

	v, ok := decoded.Chains[0]["decision_visible"]
	if !ok {
		t.Fatalf("false 被 omitempty 吃掉了——消費端會分不出「不參與決策」與「舊版沒有這個欄位」")
	}
	if v != false {
		t.Errorf("decision_visible = %v, want false", v)
	}
	if decoded.Chains[1]["decision_visible"] != true {
		t.Errorf("決策可見的鏈要是 true, got %v", decoded.Chains[1]["decision_visible"])
	}
}

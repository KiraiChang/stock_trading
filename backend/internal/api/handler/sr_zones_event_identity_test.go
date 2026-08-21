package handler

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/trading/backend/internal/store"
)

// buildEventIdentityWrite 的錯誤在資料裡看起來都很正常——事件掛錯 zone、鏈斷成兩截、
// root 被 latest 蓋掉、鏈靜默凍結。所以它被拆成純函數，可以在沒有 HTTP 與 DB 的情況下測。
//
// 關聯決策是三段的（T-048 階段 C 修法）：
//
//	① 既有活鏈以 (last_zone_key, family) 命中 → 沿用 chain.zone_uid，不解析 key
//	② 沒有活鏈 ＋ carried_from_previous == true → 不建立新 occurrence
//	③ 沒有活鏈 ＋ 非 carried → 解析 zone（本次 map → alias history），
//	   再以 (zone_uid, family) 查一次活鏈才決定延續或開新鏈
//
// 每一段各有測試，因為每一段失敗的樣子都不一樣、而且都不會報錯。

var eventNow = time.Date(2026, 8, 19, 7, 0, 0, 0, time.UTC)

const (
	zoneKeyA = "SUPPORT:104.7300:105.3700"
	// 同一個 zone 隔天被 ATR 重算後的樣子：區間幾乎一樣，字串完全不同。
	zoneKeyADrifted = "SUPPORT:104.7412:105.3688"
	// 同一個 zone 進了 AT_ZONE：邊界一動也沒動，但 role 編在 key 裡。
	zoneKeyAFlipped = "AT_ZONE:104.7300:105.3700"
)

func testEventUID() func() string {
	n := 0
	return func() string {
		n++
		return fmt.Sprintf("E-new-%d", n)
	}
}

func eventState(zoneKey, family, eventType, state string) store.MarketEventState {
	scope := "ZONE"
	if zoneKey == store.SymbolScopeKey {
		scope = "SYMBOL"
	}
	return store.MarketEventState{
		Symbol: "0050", Timeframe: "1d",
		EventScope: scope, ZoneKey: zoneKey, EventFamily: family,
		RootEventType: eventType, LatestEventType: eventType,
		State: state, Active: true, Direction: "BEARISH",
		StateJSON: store.RawJSON(`{"carried_from_previous": false}`),
	}
}

// carriedState 是 Python 把上次的狀態抄過來的那種——**不是這次發生的事件**。
// 旗標由 _normalize_previous_event_state 無條件設 true。
func carriedState(zoneKey, family, eventType, state string) store.MarketEventState {
	st := eventState(zoneKey, family, eventType, state)
	st.StateJSON = store.RawJSON(`{"carried_from_previous": true}`)
	if state == "RESOLVED" || state == "EXPIRED" {
		st.Active = false
	}
	return st
}

func outcome(pairs map[string]string, ended ...string) *zoneIdentityOutcome {
	return outcomeWithAlias(pairs, nil, ended...)
}

func outcomeWithAlias(pairs, alias map[string]string, ended ...string) *zoneIdentityOutcome {
	set := map[string]struct{}{}
	for _, e := range ended {
		set[e] = struct{}{}
	}
	if alias == nil {
		alias = map[string]string{}
	}
	return &zoneIdentityOutcome{
		UIDByZoneKey: pairs, AliasUIDByZoneKey: alias, EndedZoneUIDs: set,
	}
}

// lastZoneKey 給空字串代表這條鏈是修法前寫入的（欄位可為 NULL），
// 此時第一把鑰匙查不到它，只能走第二把。
func liveChain(uid, zoneUID, lastZoneKey, family, state string, seq int, ended bool) store.LiveEvent {
	inst := store.EventInstance{
		EventUID: uid, Symbol: "0050", Timeframe: "1d",
		ZoneUID:      sql.NullString{String: zoneUID, Valid: zoneUID != store.SymbolScopeKey},
		ZoneScopeKey: zoneUID, EventScope: "ZONE", EventFamily: family, Seq: seq,
		RootEventType: "SUPPORT_BREAKDOWN", LatestEventType: "SUPPORT_BREAKDOWN",
		State: state, Active: true,
		LastZoneKey: sql.NullString{String: lastZoneKey, Valid: lastZoneKey != ""},
		FirstSeenAt: eventNow.Add(-72 * time.Hour), LastSeenAt: eventNow.Add(-24 * time.Hour),
	}
	if ended {
		inst.Active = false
		inst.EndedAt = sql.NullTime{Time: eventNow.Add(-24 * time.Hour), Valid: true}
		inst.EndReason = sql.NullString{String: state, Valid: true}
	}
	return store.LiveEvent{EventInstance: inst}
}

func instanceByUID(t *testing.T, w store.EventIdentityWrite, uid string) store.EventInstance {
	t.Helper()
	for _, inst := range w.Instances {
		if inst.EventUID == uid {
			return inst
		}
	}
	t.Fatalf("找不到 event_uid=%s，got %+v", uid, w.Instances)
	return store.EventInstance{}
}

// ── 基本寫法 ───────────────────────────────────────────────────────────────

func TestBuildEventIdentityWriteCreatesChainOnFirstSight(t *testing.T) {
	states := []store.MarketEventState{eventState(zoneKeyA, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "CONFIRMED")}

	w, stats := buildEventIdentityWrite("0050", "1d", 7, eventNow, states, nil,
		outcome(map[string]string{zoneKeyA: "Z-1"}), testEventUID())

	if len(stats.UnmatchedKeys) != 0 {
		t.Fatalf("不該有關聯失敗，got %v", stats.UnmatchedKeys)
	}
	if len(w.Instances) != 1 || w.Instances[0].Seq != 1 {
		t.Fatalf("首見要開第一條鏈，got %+v", w.Instances)
	}
	if !w.Instances[0].ZoneUID.Valid || w.Instances[0].ZoneUID.String != "Z-1" {
		t.Errorf("事件要掛到 zone 的穩定身分，got %+v", w.Instances[0].ZoneUID)
	}
	if w.Instances[0].LastZoneKey.String != zoneKeyA {
		t.Errorf("last_zone_key 要記下這次事件帶的 key，got %+v", w.Instances[0].LastZoneKey)
	}
	if stats.MatchedByCurrent != 1 {
		t.Errorf("這是本次 map 命中，got %+v", stats)
	}
	if len(w.Transitions) != 1 || w.Transitions[0].FromState.Valid {
		t.Fatalf("誕生是唯一 from_state 留白的轉換，got %+v", w.Transitions)
	}
	if w.Transitions[0].ToState != "CONFIRMED" {
		t.Errorf("to_state 要是誕生時的狀態，got %q", w.Transitions[0].ToState)
	}
}

func TestBuildEventIdentityWriteNoTransitionWhenStateUnchanged(t *testing.T) {
	// 每次分析都寫一筆會讓流水變成「分析次數的紀錄」而不是「狀態轉換的紀錄」。
	live := []store.LiveEvent{liveChain("E-1", "Z-1", zoneKeyA, "SUPPORT_BREAKDOWN", "ACTIVE", 1, false)}
	states := []store.MarketEventState{eventState(zoneKeyA, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "ACTIVE")}

	w, _ := buildEventIdentityWrite("0050", "1d", 7, eventNow, states, live,
		outcome(map[string]string{zoneKeyA: "Z-1"}), testEventUID())

	if len(w.Transitions) != 0 {
		t.Errorf("狀態沒變就不該有 transition，got %+v", w.Transitions)
	}
	if len(w.Instances) != 1 || !w.Instances[0].LastSeenAt.Equal(eventNow) {
		t.Errorf("但 last_seen_at 還是要前進，got %+v", w.Instances)
	}
}

func TestBuildEventIdentityWriteStartsNewChainAfterPreviousEnded(t *testing.T) {
	// 前一條 RESOLVED 之後再出現同家族事件，那是新的一條鏈而不是舊鏈復活
	// （規則與 Python 的 build_event_state_summary 對稱）。
	live := []store.LiveEvent{liveChain("E-1", "Z-1", zoneKeyA, "SUPPORT_BREAKDOWN", "RESOLVED", 1, true)}
	states := []store.MarketEventState{eventState(zoneKeyA, "SUPPORT_BREAKDOWN", "INTRADAY_RECLAIM", "CONFIRMED")}

	w, _ := buildEventIdentityWrite("0050", "1d", 7, eventNow, states, live,
		outcome(map[string]string{zoneKeyA: "Z-1"}), testEventUID())

	if len(w.Instances) != 1 || w.Instances[0].EventUID == "E-1" {
		t.Fatalf("已終結的鏈不可延續，got %+v", w.Instances)
	}
	if w.Instances[0].Seq != 2 {
		t.Errorf("新鏈的 seq 要接在歷史最大值之後，got %d", w.Instances[0].Seq)
	}
	if w.Instances[0].RootEventType != "INTRADAY_RECLAIM" {
		t.Errorf("新鏈的 root 是這次的事件，不是沿用舊鏈的，got %q", w.Instances[0].RootEventType)
	}
	if len(w.Transitions) != 1 || w.Transitions[0].FromState.Valid {
		t.Errorf("新鏈的第一筆也是誕生，from_state 要留白，got %+v", w.Transitions)
	}
}

func TestBuildEventIdentityWriteSymbolScopedEventNeedsNoZone(t *testing.T) {
	// live 的 86 筆 market_event_states 裡有 12 筆是 SYMBOL scope。
	states := []store.MarketEventState{eventState(store.SymbolScopeKey, "VOLUME_CONTEXT", "EXTREME_VOLUME", "ACTIVE")}

	w, stats := buildEventIdentityWrite("0050", "1d", 7, eventNow, states, nil,
		outcome(map[string]string{}), testEventUID())

	if len(stats.UnmatchedKeys) != 0 {
		t.Fatalf("SYMBOL scope 不需要 zone，不該算成關聯失敗，got %v", stats.UnmatchedKeys)
	}
	if len(w.Instances) != 1 || w.Instances[0].ZoneUID.Valid {
		t.Fatalf("SYMBOL scope 的 zone_uid 要留 NULL，got %+v", w.Instances)
	}
	if w.Instances[0].ZoneScopeKey != store.SymbolScopeKey {
		t.Errorf("zone_scope_key 要有值才擋得住重複，got %q", w.Instances[0].ZoneScopeKey)
	}
	if w.Instances[0].LastZoneKey.String != store.SymbolScopeKey {
		t.Errorf("SYMBOL scope 的 last_zone_key 要與 scope key 一致，got %+v", w.Instances[0].LastZoneKey)
	}
}

func TestBuildEventIdentityWriteSkipsWhenZoneIdentityUnavailable(t *testing.T) {
	// 階段 B 失敗時整段要跳過，而不是寫出指向不存在身分的事件鏈。
	// persistEventIdentity 靠 zoneOutcome == nil 判斷，這裡鎖住呼叫端的契約：
	// 空的對照表會讓所有 ZONE scope 事件都算成關聯失敗、不寫入。
	states := []store.MarketEventState{eventState(zoneKeyA, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "CONFIRMED")}

	w, stats := buildEventIdentityWrite("0050", "1d", 7, eventNow, states, nil,
		outcome(map[string]string{}), testEventUID())

	if len(w.Instances) != 0 || len(stats.UnmatchedKeys) != 1 {
		t.Errorf("沒有身分可掛就不該寫入，got instances=%+v unmatched=%v", w.Instances, stats.UnmatchedKeys)
	}
}

func TestBuildEventIdentityWriteReportsUnmatchedZoneScopedEvents(t *testing.T) {
	// **關聯失敗不能靜靜寫成 NULL**：那與「這是 SYMBOL scope 的事件」在資料上
	// 長得一模一樣，之後比對兩套結果只會看到「事件變少了」而查不出原因。
	states := []store.MarketEventState{eventState("SUPPORT:999.0000:1000.0000", "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "CONFIRMED")}

	w, stats := buildEventIdentityWrite("0050", "1d", 7, eventNow, states, nil,
		outcome(map[string]string{zoneKeyA: "Z-1"}), testEventUID())

	if len(stats.UnmatchedKeys) != 1 || stats.UnmatchedKeys[0] != "SUPPORT:999.0000:1000.0000" {
		t.Fatalf("關聯失敗要被回報，got %v", stats.UnmatchedKeys)
	}
	if len(w.Instances) != 0 {
		t.Errorf("關聯不到就不該寫入，got %+v", w.Instances)
	}
}

// ── ① 第一把鑰匙：既有活鏈優先 ────────────────────────────────────────────

func TestBuildEventIdentityWriteContinuesLiveChainByLastZoneKey(t *testing.T) {
	// **這是 F1 的回歸測試。** 本次分析的 map 裡完全沒有這個 key（zone 這次缺席，
	// 或 role 已經翻轉），修法前會算成關聯失敗而跳過——鏈停在上一階的狀態、
	// 資料表面上完全正常，這正是「靜默凍結」難查的地方。
	live := []store.LiveEvent{liveChain("E-1", "Z-1", zoneKeyA, "SUPPORT_RECLAIM", "CONFIRMED", 1, false)}
	states := []store.MarketEventState{eventState(zoneKeyA, "SUPPORT_RECLAIM", "INTRADAY_RECLAIM", "ACTIVE")}

	w, stats := buildEventIdentityWrite("0050", "1d", 7, eventNow, states, live,
		outcome(map[string]string{}), testEventUID())

	if len(stats.UnmatchedKeys) != 0 {
		t.Fatalf("既有鏈不必解析 key，不該算成關聯失敗，got %v", stats.UnmatchedKeys)
	}
	if len(w.Instances) != 1 || w.Instances[0].EventUID != "E-1" {
		t.Fatalf("要沿用既有鏈，got %+v", w.Instances)
	}
	if w.Instances[0].ZoneUID.String != "Z-1" {
		t.Errorf("zone_uid 直接沿用鏈上的，got %+v", w.Instances[0].ZoneUID)
	}
	if !w.Instances[0].FirstSeenAt.Equal(eventNow.Add(-72 * time.Hour)) {
		t.Errorf("first_seen_at 是鏈的起點，不可被本次覆寫，got %v", w.Instances[0].FirstSeenAt)
	}
	if stats.MatchedByChain != 1 {
		t.Errorf("要記成第一把鑰匙命中，got %+v", stats)
	}
	if len(w.Transitions) != 1 || w.Transitions[0].FromState.String != "CONFIRMED" {
		t.Fatalf("狀態變了要留痕且 from_state 不可留白，got %+v", w.Transitions)
	}
}

func TestBuildEventIdentityWriteRefreshesLastZoneKeyOnContinue(t *testing.T) {
	// **last_zone_key 停住等於第一把鑰匙失效。** 鏈記著誕生那天的 key，
	// 而事件帶的 key 每天都被 ATR 推著走，兩者只會越離越遠。
	live := []store.LiveEvent{liveChain("E-1", "Z-1", zoneKeyA, "SUPPORT_BREAKDOWN", "ACTIVE", 1, false)}
	states := []store.MarketEventState{eventState(zoneKeyADrifted, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "ACTIVE")}

	w, _ := buildEventIdentityWrite("0050", "1d", 7, eventNow, states, live,
		outcome(map[string]string{zoneKeyADrifted: "Z-1"}), testEventUID())

	if len(w.Instances) != 1 || w.Instances[0].EventUID != "E-1" {
		t.Fatalf("要沿用既有鏈，got %+v", w.Instances)
	}
	if w.Instances[0].LastZoneKey.String != zoneKeyADrifted {
		t.Errorf("last_zone_key 要更新成本次事件帶的 key，got %+v", w.Instances[0].LastZoneKey)
	}
}

func TestBuildEventIdentityWriteDoesNotContinueChainOnTerminatedZone(t *testing.T) {
	// 守門：第一把鑰匙命中，但那個身分本次因 SPLIT / MERGE / RESHAPE 終止。
	// 放行的話 D4 的迴圈會因為 written 去重而跳過它，鏈就永遠收不掉——
	// 事件留在一個已經不存在的 zone 上，而且 ended_at 永遠是 NULL。
	live := []store.LiveEvent{liveChain("E-1", "Z-P", zoneKeyA, "SUPPORT_BREAKDOWN", "ACTIVE", 1, false)}
	states := []store.MarketEventState{eventState(zoneKeyA, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "ACTIVE")}

	w, stats := buildEventIdentityWrite("0050", "1d", 7, eventNow, states, live,
		outcome(map[string]string{zoneKeyA: "Z-P"}, "Z-P"), testEventUID())

	if len(stats.ZoneEndedSkipped) != 1 {
		t.Fatalf("要記成交給 D4 收攤，got %+v", stats)
	}
	inst := instanceByUID(t, w, "E-1")
	if !inst.EndedAt.Valid || inst.EndReason.String != "ZONE_IDENTITY_ENDED" {
		t.Fatalf("鏈要由 D4 收掉，got %+v", inst)
	}
	if inst.State != "EXPIRED" || inst.Active {
		t.Errorf("終態四個欄位要一致，got state=%q active=%v", inst.State, inst.Active)
	}
}

func TestBuildEventIdentityWriteKeepsFirstChainWhenLastZoneKeyAmbiguous(t *testing.T) {
	// 兩個身分曾用過同一個 key（同區間不同 method）並非不可能。選一個穩定的規則，
	// 而不是讓 map 的寫入順序決定；但衝突本身要看得見。
	live := []store.LiveEvent{
		liveChain("E-1", "Z-1", zoneKeyA, "SUPPORT_BREAKDOWN", "ACTIVE", 1, false),
		liveChain("E-2", "Z-2", zoneKeyA, "SUPPORT_BREAKDOWN", "ACTIVE", 1, false),
	}
	states := []store.MarketEventState{eventState(zoneKeyA, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "CONFIRMED")}

	w, stats := buildEventIdentityWrite("0050", "1d", 7, eventNow, states, live,
		outcome(map[string]string{zoneKeyA: "Z-1"}), testEventUID())

	if len(stats.ChainKeyAmbiguous) != 1 {
		t.Fatalf("重複的 (last_zone_key, family) 要被回報，got %+v", stats)
	}
	if len(w.Instances) != 1 || w.Instances[0].EventUID != "E-1" {
		t.Errorf("保留第一個，got %+v", w.Instances)
	}
}

func TestBuildEventIdentityWritePrefersChainOverCurrentMap(t *testing.T) {
	// 使用者定案的優先序：既有 instance 優先。但本次 map 給出不同身分代表
	// zone 匹配出現了新的成因，要記下來。
	live := []store.LiveEvent{liveChain("E-1", "Z-1", zoneKeyA, "SUPPORT_BREAKDOWN", "ACTIVE", 1, false)}
	states := []store.MarketEventState{eventState(zoneKeyA, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "CONFIRMED")}

	w, stats := buildEventIdentityWrite("0050", "1d", 7, eventNow, states, live,
		outcome(map[string]string{zoneKeyA: "Z-OTHER"}), testEventUID())

	if len(w.Instances) != 1 || w.Instances[0].ZoneUID.String != "Z-1" {
		t.Fatalf("以既有鏈為準，got %+v", w.Instances)
	}
	if len(stats.ChainConflicts) != 1 {
		t.Errorf("衝突要看得見，got %+v", stats)
	}
}

// ── ② carried 護欄 ────────────────────────────────────────────────────────

func TestBuildEventIdentityWriteCarriedTerminalDoesNotRebirth(t *testing.T) {
	// **F2 的回歸測試。** Python 每次分析都重報同一筆終態，舊版把「這次看到」
	// 當成「這次發生」，於是 seq 2 / 3 / 4… 誕生即終態，一路長下去。
	states := []store.MarketEventState{carriedState(zoneKeyA, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "EXPIRED")}

	w, stats := buildEventIdentityWrite("0050", "1d", 7, eventNow, states, nil,
		outcome(map[string]string{zoneKeyA: "Z-1"}), testEventUID())

	if len(w.Instances) != 0 || len(w.Transitions) != 0 {
		t.Fatalf("carried 的終態找不到活鏈就不該寫入，got %+v", w)
	}
	if len(stats.CarriedNoop) != 1 {
		t.Errorf("要記成 carried noop，got %+v", stats)
	}
}

func TestBuildEventIdentityWriteCarriedNonTerminalDoesNotCreateChain(t *testing.T) {
	// 定案規則：**carried_from_previous == false 是建立新 occurrence 的必要條件**，
	// 狀態是不是終態不參與判斷。初版修法用的是「carried==false **或** 非終態」，
	// 那個 `或` 留下這一格——同一種重生，只是新鏈掛在 ACTIVE 而不是 EXPIRED，
	// 資料表面上更像正常事件、更難查。
	states := []store.MarketEventState{carriedState(zoneKeyA, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "ACTIVE")}

	w, stats := buildEventIdentityWrite("0050", "1d", 7, eventNow, states, nil,
		outcome(map[string]string{zoneKeyA: "Z-1"}), testEventUID())

	if len(w.Instances) != 0 {
		t.Fatalf("carried 的非終態找不到活鏈一樣不建立 occurrence，got %+v", w.Instances)
	}
	if len(stats.CarriedNoop) != 1 {
		t.Errorf("要記成 carried noop，got %+v", stats)
	}
}

func TestBuildEventIdentityWriteCarriedContinuesExistingChain(t *testing.T) {
	// 護欄只擋「找不到活鏈」的 carried。有活鏈時 carried 事件仍要推進 last_seen_at，
	// 否則正常的鏈會因為某天沒有新偵測而看起來停住。
	live := []store.LiveEvent{liveChain("E-1", "Z-1", zoneKeyA, "SUPPORT_BREAKDOWN", "ACTIVE", 1, false)}
	states := []store.MarketEventState{carriedState(zoneKeyA, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "ACTIVE")}

	w, stats := buildEventIdentityWrite("0050", "1d", 7, eventNow, states, live,
		outcome(map[string]string{zoneKeyA: "Z-1"}), testEventUID())

	if len(stats.CarriedNoop) != 0 {
		t.Fatalf("有活鏈就不該被護欄擋下，got %+v", stats)
	}
	if len(w.Instances) != 1 || w.Instances[0].EventUID != "E-1" || !w.Instances[0].LastSeenAt.Equal(eventNow) {
		t.Errorf("carried 事件要延續既有鏈並推進 last_seen_at，got %+v", w.Instances)
	}
}

func TestBuildEventIdentityWriteUnparsableCarriedFlagCountsAsFalse(t *testing.T) {
	// 讀不到旗標時當 true 會讓真實的新事件被護欄靜默吃掉；當 false 最多讓一筆重報
	// 開了新鏈，那會被階梯門檻②（誕生即終態）抓到。所以一律當 false 並計數。
	st := eventState(zoneKeyA, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "CONFIRMED")
	st.StateJSON = store.RawJSON(`{"not_the_flag": 1}`)
	states := []store.MarketEventState{st}

	w, stats := buildEventIdentityWrite("0050", "1d", 7, eventNow, states, nil,
		outcome(map[string]string{zoneKeyA: "Z-1"}), testEventUID())

	if len(w.Instances) != 1 {
		t.Fatalf("解析失敗要當成非 carried、照常開鏈，got %+v", w.Instances)
	}
	if stats.CarriedParseFail != 1 {
		t.Errorf("解析失敗本身要被記下來，got %+v", stats)
	}
}

// ── ③ key 解析與匯流點 ────────────────────────────────────────────────────

func TestBuildEventIdentityWriteFallsBackToAliasWhenRoleFlipped(t *testing.T) {
	// role 編在 key 裡，所以 zone 一進 AT_ZONE，事件帶的舊 key 就對不上本次 map——
	// 即使邊界一動也沒動。這是實測 F1 的第一個成因。
	states := []store.MarketEventState{eventState(zoneKeyA, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "CONFIRMED")}

	w, stats := buildEventIdentityWrite("0050", "1d", 7, eventNow, states, nil,
		outcomeWithAlias(
			map[string]string{zoneKeyAFlipped: "Z-1"}, // 本次分析只認得翻轉後的 key
			map[string]string{zoneKeyA: "Z-1"},        // 歷史上這個身分用過事件帶的 key
		), testEventUID())

	if len(stats.UnmatchedKeys) != 0 {
		t.Fatalf("alias 要接住它，got %v", stats.UnmatchedKeys)
	}
	if len(w.Instances) != 1 || w.Instances[0].ZoneUID.String != "Z-1" {
		t.Fatalf("要掛到 alias 指到的身分，got %+v", w.Instances)
	}
	if stats.MatchedByAlias != 1 || stats.MatchedByCurrent != 0 {
		t.Errorf("要記成 alias 命中，got %+v", stats)
	}
}

func TestBuildEventIdentityWriteFallsBackToAliasWhenBoundaryDrifted(t *testing.T) {
	// F1 的第二個成因：邊界被 ATR 重算，區間重疊 99% 但字串完全不同。
	states := []store.MarketEventState{eventState(zoneKeyA, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "CONFIRMED")}

	w, stats := buildEventIdentityWrite("0050", "1d", 7, eventNow, states, nil,
		outcomeWithAlias(
			map[string]string{zoneKeyADrifted: "Z-1"},
			map[string]string{zoneKeyA: "Z-1"},
		), testEventUID())

	if len(w.Instances) != 1 || w.Instances[0].ZoneUID.String != "Z-1" {
		t.Fatalf("alias 要接住漂走的邊界，got %+v", w.Instances)
	}
	if stats.MatchedByAlias != 1 {
		t.Errorf("要記成 alias 命中，got %+v", stats)
	}
}

func TestBuildEventIdentityWriteCurrentMapWinsOverAlias(t *testing.T) {
	// alias 是第二次機會，不是替代品。本次分析算出來的對應永遠比歷史新。
	states := []store.MarketEventState{eventState(zoneKeyA, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "CONFIRMED")}

	w, stats := buildEventIdentityWrite("0050", "1d", 7, eventNow, states, nil,
		outcomeWithAlias(
			map[string]string{zoneKeyA: "Z-CURRENT"},
			map[string]string{zoneKeyA: "Z-STALE"},
		), testEventUID())

	if len(w.Instances) != 1 || w.Instances[0].ZoneUID.String != "Z-CURRENT" {
		t.Fatalf("本次 map 優先，got %+v", w.Instances)
	}
	if stats.MatchedByCurrent != 1 || stats.MatchedByAlias != 0 {
		t.Errorf("計數要落在本次 map，got %+v", stats)
	}
}

func TestBuildEventIdentityWriteDoesNotOpenSecondLiveChainOnSameZone(t *testing.T) {
	// **匯流點的回歸測試（第二把鑰匙）。** 鏈記的是舊 key，本次事件帶的是新 key，
	// 所以第一把鑰匙 miss；但 resolve 得出同一個 zone_uid，而那上面已經有活鏈。
	// 無條件開新鏈的話 ListLatestChains 只回 MAX(seq)，舊鏈從此撈不出來——
	// F1 的靜默凍結原樣重現，只是成因從「key 到不了身分」換成「身分到不了鏈」。
	live := []store.LiveEvent{liveChain("E-1", "Z-1", zoneKeyA, "SUPPORT_BREAKDOWN", "ACTIVE", 1, false)}
	states := []store.MarketEventState{eventState(zoneKeyADrifted, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "CONFIRMED")}

	w, _ := buildEventIdentityWrite("0050", "1d", 7, eventNow, states, live,
		outcome(map[string]string{zoneKeyADrifted: "Z-1"}), testEventUID())

	if len(w.Instances) != 1 {
		t.Fatalf("同一個 (zone, family) 上不可有第二條活鏈，got %+v", w.Instances)
	}
	if w.Instances[0].EventUID != "E-1" || w.Instances[0].Seq != 1 {
		t.Errorf("要沿用既有活鏈，got %+v", w.Instances[0])
	}
	if w.Instances[0].LastZoneKey.String != zoneKeyADrifted {
		t.Errorf("沿用時 last_zone_key 要更新，否則下次第一把鑰匙一樣 miss，got %+v",
			w.Instances[0].LastZoneKey)
	}
}

func TestBuildEventIdentityWriteOpensNewChainWhenZoneHasNoLiveChain(t *testing.T) {
	// 第二把鑰匙只在有活鏈時攔截。同一個身分上前一條已終結，這次就是新的一條鏈。
	live := []store.LiveEvent{liveChain("E-1", "Z-1", zoneKeyA, "SUPPORT_BREAKDOWN", "RESOLVED", 2, true)}
	states := []store.MarketEventState{eventState(zoneKeyADrifted, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "CONFIRMED")}

	w, _ := buildEventIdentityWrite("0050", "1d", 7, eventNow, states, live,
		outcome(map[string]string{zoneKeyADrifted: "Z-1"}), testEventUID())

	if len(w.Instances) != 1 || w.Instances[0].EventUID == "E-1" {
		t.Fatalf("已終結的鏈不可延續，got %+v", w.Instances)
	}
	if w.Instances[0].Seq != 3 {
		t.Errorf("seq 要接在歷史最大值之後，got %d", w.Instances[0].Seq)
	}
}

// ── ④ 終態原子化（F4）────────────────────────────────────────────────────

func TestBuildEventIdentityWriteClosesChainOnTerminalState(t *testing.T) {
	// RESOLVED / EXPIRED 不收掉的話，下次分析會把它當成還活著而延續下去，
	// 同一條鏈就吃掉了兩段本該分開的歷史。
	live := []store.LiveEvent{liveChain("E-1", "Z-1", zoneKeyA, "SUPPORT_BREAKDOWN", "ACTIVE", 1, false)}
	states := []store.MarketEventState{eventState(zoneKeyA, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "RESOLVED")}

	w, stats := buildEventIdentityWrite("0050", "1d", 7, eventNow, states, live,
		outcome(map[string]string{zoneKeyA: "Z-1"}), testEventUID())

	if len(w.Instances) != 1 || !w.Instances[0].EndedAt.Valid {
		t.Fatalf("終態要收掉鏈，got %+v", w.Instances)
	}
	if w.Instances[0].EndReason.String != "RESOLVED" {
		t.Errorf("end_reason 要記終結原因，got %+v", w.Instances[0].EndReason)
	}
	// F4：四個欄位要一起到位。Python 端終態的 active 本來就是 false
	// （RESOLVED / EXPIRED 都不在 gating_states 裡），這裡強制寫死不改變既有語意，
	// 但擋掉「上游哪天回了 true」那種靜默的不一致。
	if w.Instances[0].State != "RESOLVED" || w.Instances[0].Active {
		t.Errorf("state / active 要與 ended_at 同時到位，got state=%q active=%v",
			w.Instances[0].State, w.Instances[0].Active)
	}
	if len(stats.Invariant) != 0 {
		t.Errorf("正常輸入下不變式掃描要是空的，got %v", stats.Invariant)
	}
}

func TestBuildEventIdentityWriteForcesInactiveOnTerminalState(t *testing.T) {
	// 上游若把終態連同 active=true 一起送來，寫進去就是 F4 那種自相矛盾的資料。
	live := []store.LiveEvent{liveChain("E-1", "Z-1", zoneKeyA, "SUPPORT_BREAKDOWN", "ACTIVE", 1, false)}
	st := eventState(zoneKeyA, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "EXPIRED")
	st.Active = true

	w, stats := buildEventIdentityWrite("0050", "1d", 7, eventNow, []store.MarketEventState{st}, live,
		outcome(map[string]string{zoneKeyA: "Z-1"}), testEventUID())

	if len(w.Instances) != 1 || w.Instances[0].Active {
		t.Fatalf("終態一律 active=false，got %+v", w.Instances)
	}
	if len(stats.Invariant) != 0 {
		t.Errorf("擋下來之後不變式就不該再被觸發，got %v", stats.Invariant)
	}
}

func TestBuildEventIdentityWriteEndsChainWhenZoneIdentityEnds(t *testing.T) {
	// D4：parent 身上的事件**不傳給 child**。zone 因 SPLIT/MERGE/RESHAPE 終止時，
	// 那條鏈的前提已經消失。
	live := []store.LiveEvent{liveChain("E-1", "Z-P", zoneKeyA, "SUPPORT_BREAKDOWN", "ACTIVE", 1, false)}
	// 這次的分析裡 Z-P 不見了，取而代之的是 child Z-C。
	states := []store.MarketEventState{eventState("SUPPORT:104.7000:105.2000", "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "CONFIRMED")}

	w, stats := buildEventIdentityWrite("0050", "1d", 7, eventNow, states, live,
		outcome(map[string]string{"SUPPORT:104.7000:105.2000": "Z-C"}, "Z-P"), testEventUID())

	parent := instanceByUID(t, w, "E-1")
	if !parent.EndedAt.Valid {
		t.Fatalf("parent 的鏈要收掉，got %+v", w.Instances)
	}
	if parent.EndReason.String != "ZONE_IDENTITY_ENDED" {
		t.Errorf("要記得出是身分終止造成的，got %+v", parent.EndReason)
	}
	// F4：舊版只設 ended_at / end_reason，state 與 active 沿用舊值，
	// 寫出來就是「ended_at 有值卻 active=true」。
	if parent.State != "EXPIRED" || parent.Active {
		t.Errorf("instance 的終態要與 transition 對齊，got state=%q active=%v",
			parent.State, parent.Active)
	}
	if parent.LastSeenAt.Equal(eventNow) {
		t.Error("這次並沒有看到 parent，last_seen_at 不該前進")
	}
	if len(stats.Invariant) != 0 {
		t.Errorf("不變式掃描要是空的，got %v", stats.Invariant)
	}

	var closing *store.EventTransition
	for i := range w.Transitions {
		if w.Transitions[i].EventUID == "E-1" {
			closing = &w.Transitions[i]
		}
	}
	if closing == nil {
		t.Fatal("收攤也要留痕")
	}
	if closing.FromState.String != "ACTIVE" || closing.ToState != "EXPIRED" {
		t.Errorf("from_state 要保留舊狀態（留白等同誕生），got %+v", closing)
	}

	child := instanceByUID(t, w, "E-new-1")
	if child.Seq != 1 {
		t.Errorf("child 是新的身分，事件從第一條鏈重新開始，got %+v", child)
	}
}

func TestEventIdentityStatsCarriedNoopAloneIsNotAWarning(t *testing.T) {
	// 終態被 carry forward 是常態：每一條走完的鏈，此後每一次分析都會貢獻一筆
	// CarriedNoop。讓它自己升 Warn，warn 就永遠不會歸零，真正的異常會被淹掉。
	// 2026-08-19 的七階 0050 實測就是這樣累積到 5 筆、階階觸發。
	if (eventIdentityStats{CarriedNoop: []string{"SUPPORT:1.0000:2.0000|SUPPORT_RECLAIM"}}).hasWarning() {
		t.Error("只有 carried noop 時不該升 Warn")
	}

	// 對照組：真正該有人看一眼的四類仍然要升 Warn。
	for name, s := range map[string]eventIdentityStats{
		"unmatched":      {UnmatchedKeys: []string{"k"}},
		"chain_conflict": {ChainConflicts: []string{"k"}},
		"chain_ambig":    {ChainKeyAmbiguous: []string{"k"}},
		"alias_ambig":    {AliasAmbiguous: []string{"k"}},
		"parse_fail":     {CarriedParseFail: 1},
	} {
		if !s.hasWarning() {
			t.Errorf("%s 要升 Warn", name)
		}
	}
}

func TestAliasIndexDropsIdentitiesTheMatcherGaveUpOn(t *testing.T) {
	// F5 的回歸測試（2026-08-19 每日階梯實測）。alias 索引原本只看
	// `state='ACTIVE' AND ended_at IS NULL`，而失格只收掉「這一世」、身分本身仍是
	// ACTIVE——於是 matcher 早就放棄的身分照樣是 alias 候選。實測 8150 有兩個
	// role／method／邊界逐位元相同的身分同時活著，缺席次數都已經超過上限。
	refs := []store.ZoneKeyAliasRef{
		// 先到的是殭屍：matcher 這一輪把它列進 expired_previous。
		{ZoneKey: zoneKeyA, ZoneUID: "Z-zombie"},
		{ZoneKey: zoneKeyA, ZoneUID: "Z-live"},
		{ZoneKey: zoneKeyADrifted, ZoneUID: "Z-other"},
	}

	byKey, ambiguous := aliasUIDByZoneKey(refs, []string{"Z-zombie"})

	if byKey[zoneKeyA] != "Z-live" {
		t.Errorf("失格的身分不該佔住 key，got %q", byKey[zoneKeyA])
	}
	if byKey[zoneKeyADrifted] != "Z-other" {
		t.Errorf("沒失格的身分要留著，got %q", byKey[zoneKeyADrifted])
	}
	if len(ambiguous) != 0 {
		t.Errorf("排掉殭屍之後就不該再算撞號，got %+v", ambiguous)
	}

	// 對照組：兩個都沒失格時，撞號仍然要被報出來——它那時是真的有兩個活身分同形。
	if _, amb := aliasUIDByZoneKey(refs, nil); len(amb) != 1 {
		t.Errorf("兩個活身分共用一個 key 仍是撞號，got %+v", amb)
	}
}

// ── 決策可見性（階段 D 的隔離旗標，R1）────────────────────────────────────

// 旗標必須從 state_json 一路搬進身分層。Go 若改成依 event_family 推導，就會多出
// 第二份型別清單，而兩份分歧時沒有任何東西會報錯——與 carried_from_previous 同一個理由。
func TestBuildEventIdentityWriteCarriesDecisionVisibleFromStateJSON(t *testing.T) {
	invisible := eventState(zoneKeyA, "RESISTANCE_BREAKOUT", "RESISTANCE_BREAKOUT", "CANDIDATE")
	invisible.StateJSON = store.RawJSON(`{"carried_from_previous": false, "decision_visible": false}`)

	w, _ := buildEventIdentityWrite("0050", "1d", 7, eventNow,
		[]store.MarketEventState{invisible}, nil,
		outcome(map[string]string{zoneKeyA: "Z-1"}), testEventUID())

	if len(w.Instances) != 1 {
		t.Fatalf("要開一條鏈，got %+v", w.Instances)
	}
	if w.Instances[0].DecisionVisible {
		t.Errorf("decision_visible=false 的事件不該被標成參與決策，got %+v", w.Instances[0])
	}
}

// **缺鍵一律 true**：既有四個事件型別都是決策可見的，而階段 D 之前寫進
// market_event_states 的列根本不會有這個鍵。當成 false 會讓既有事件整批
// 從「參與決策」變成「事實紀錄」，那是最嚴重的行為改變。
func TestBuildEventIdentityWriteDefaultsDecisionVisibleToTrue(t *testing.T) {
	states := []store.MarketEventState{eventState(zoneKeyA, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "CONFIRMED")}

	w, _ := buildEventIdentityWrite("0050", "1d", 7, eventNow, states, nil,
		outcome(map[string]string{zoneKeyA: "Z-1"}), testEventUID())

	if !w.Instances[0].DecisionVisible {
		t.Errorf("缺鍵要視為決策可見，got %+v", w.Instances[0])
	}
}

// zone 身分終止時鏈由既有列收攤（不是這次觀測到的），旗標要原樣留著——
// 這條路徑不帶 state_json，重算會把它寫回預設的 true。
func TestBuildEventIdentityWriteKeepsDecisionVisibleWhenZoneIdentityEnds(t *testing.T) {
	chain := liveChain("E-1", "Z-1", zoneKeyA, "RESISTANCE_BREAKOUT", "CANDIDATE", 1, false)
	chain.DecisionVisible = false

	w, _ := buildEventIdentityWrite("0050", "1d", 7, eventNow, nil,
		[]store.LiveEvent{chain}, outcome(nil, "Z-1"), testEventUID())

	inst := instanceByUID(t, w, "E-1")
	if inst.EndReason.String != "ZONE_IDENTITY_ENDED" {
		t.Fatalf("這條要以 zone 身分終止收攤，got %+v", inst)
	}
	if inst.DecisionVisible {
		t.Errorf("收攤不該把旗標寫回預設值，got %+v", inst)
	}
}

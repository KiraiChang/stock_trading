package handler

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/store"
)

// buildZoneIdentityWrite 的錯誤在資料裡看起來都很正常——身分與 zone 錯位、
// 缺席次數沒存回去、last_seen_at 被寫成本次時間而讓時間軸閘門永遠不觸發。
// 所以它被拆成純函數，可以在沒有 HTTP 與 DB 的情況下測。

var identityNow = time.Date(2026, 8, 18, 7, 0, 0, 0, time.UTC)

// 可預測的 incarnation uid——真實實作用 uuid4，測試不該面對隨機值。
func testUID() func() string {
	n := 0
	return func() string {
		n++
		return fmt.Sprintf("I-new-%d", n)
	}
}

func srZone(low, high float64, role string) store.SRZone {
	return store.SRZone{PriceLow: low, PriceHigh: high, Method: "recent_pivot", Role: role}
}

func liveZone(uid string, low, high float64, seen time.Time, absences int) store.LiveZone {
	return store.LiveZone{
		ZoneInstance: store.ZoneInstance{
			ZoneUID: uid, Symbol: "0050", Timeframe: "1d", Method: "recent_pivot",
			State: "ACTIVE", PriceLow: low, PriceHigh: high,
			FirstSeenAt: seen, LastSeenAt: seen, ObservedAbsences: absences,
		},
	}
}

func TestBuildZoneIdentityWriteKeepsFirstSeenOfExistingIdentity(t *testing.T) {
	first := identityNow.AddDate(0, 0, -30)
	live := []store.LiveZone{liveZone("Z-P", 104.73, 105.37, first, 0)}
	zones := []store.SRZone{srZone(104.73, 105.37, "RESISTANCE")}
	m := &analysis.ZoneIdentityMatchResult{
		ZoneUIDs:             []string{"Z-P"},
		NextObservedAbsences: map[string]int{"Z-P": 0},
	}

	w := buildZoneIdentityWrite("0050", "1d", 42, identityNow, zones, live, m, testUID())

	if len(w.Instances) != 1 {
		t.Fatalf("want 1 instance, got %d", len(w.Instances))
	}
	// 身分第一次出現的時間是事實，用本次分析時間覆寫會讓身分壽命失真。
	if !w.Instances[0].FirstSeenAt.Equal(first) {
		t.Errorf("first_seen_at 應沿用既有值 %v，得到 %v", first, w.Instances[0].FirstSeenAt)
	}
	if !w.Instances[0].LastSeenAt.Equal(identityNow) {
		t.Errorf("這次有看到，last_seen_at 應更新")
	}
	if w.Instances[0].ObservedAbsences != 0 {
		t.Errorf("配到就歸零，得到 %d", w.Instances[0].ObservedAbsences)
	}
}

func TestBuildZoneIdentityWriteDoesNotRefreshLastSeenForAbsentZones(t *testing.T) {
	seen := identityNow.AddDate(0, 0, -5)
	live := []store.LiveZone{liveZone("Z-GONE", 200.0, 201.0, seen, 1)}
	m := &analysis.ZoneIdentityMatchResult{
		ZoneUIDs:             []string{},
		NextObservedAbsences: map[string]int{"Z-GONE": 2},
	}

	w := buildZoneIdentityWrite("0050", "1d", 42, identityNow, nil, live, m, testUID())

	if len(w.Instances) != 1 {
		t.Fatalf("缺席的身分也要寫回（更新次數），got %d", len(w.Instances))
	}
	// **這條最關鍵**：把 last_seen_at 填成本次時間等於宣告「它剛被看到」，
	// 時間軸閘門從此永遠不會觸發——而那是這批功能的一半。
	if !w.Instances[0].LastSeenAt.Equal(seen) {
		t.Errorf("缺席不該更新 last_seen_at，want %v got %v", seen, w.Instances[0].LastSeenAt)
	}
	if w.Instances[0].ObservedAbsences != 2 {
		t.Errorf("缺席次數要存回去，得到 %d", w.Instances[0].ObservedAbsences)
	}
}

func TestBuildZoneIdentityWriteNeverEmitsDuplicateInstances(t *testing.T) {
	// 一個 zone 這次有被看到、同時又出現在 NextObservedAbsences 的話，
	// 會產生同批重複 zone_uid——那只有 postgres 會炸，測試又只跑 sqlite。
	live := []store.LiveZone{liveZone("Z-P", 104.73, 105.37, identityNow, 0)}
	zones := []store.SRZone{srZone(104.73, 105.37, "SUPPORT")}
	m := &analysis.ZoneIdentityMatchResult{
		ZoneUIDs:             []string{"Z-P"},
		NextObservedAbsences: map[string]int{"Z-P": 3}, // 矛盾的輸入，故意的
	}

	w := buildZoneIdentityWrite("0050", "1d", 42, identityNow, zones, live, m, testUID())

	seen := map[string]int{}
	for _, inst := range w.Instances {
		seen[inst.ZoneUID]++
	}
	if seen["Z-P"] != 1 {
		t.Fatalf("同一個 zone_uid 只能出現一次，得到 %d 次", seen["Z-P"])
	}
}

func TestBuildZoneIdentityWriteExpiresIncarnationAndRecordsReason(t *testing.T) {
	seen := identityNow.AddDate(0, 0, -60)
	live := []store.LiveZone{liveZone("Z-EXP", 104.73, 105.37, seen, 3)}
	live[0].IncarnationUID = sql.NullString{String: "I-1", Valid: true}
	live[0].IncarnationRole = sql.NullString{String: "SUPPORT", Valid: true}
	m := &analysis.ZoneIdentityMatchResult{
		ZoneUIDs:             []string{},
		ExpiredPrevious:      []string{"Z-EXP"},
		NextObservedAbsences: map[string]int{"Z-EXP": 4},
	}

	w := buildZoneIdentityWrite("0050", "1d", 42, identityNow, nil, live, m, testUID())

	if len(w.Incarnations) != 1 {
		t.Fatalf("失格要把一世收成 EXPIRED，got %d", len(w.Incarnations))
	}
	inc := w.Incarnations[0]
	if inc.State != "EXPIRED" || inc.EndReason.String != "EXPIRED_BY_ABSENCE" {
		t.Errorf("state/end_reason 不對：%+v", inc)
	}
	// expired_at 是資格閘門的稽核依據，與 ended_at 分開存。
	if !inc.ExpiredAt.Valid {
		t.Error("expired_at 必須有值")
	}
	found := false
	for _, tr := range w.Transitions {
		if tr.ZoneUID == "Z-EXP" && tr.ToState.String == "EXPIRED" {
			found = true
		}
	}
	if !found {
		t.Error("失格要留一筆 transition，否則事後查不出它何時被判定為不再認得")
	}
	// 次數 +1 越過上限——這是與 ListLive 的握手，否則每次分析都會再收攤一次。
	if w.Instances[0].ObservedAbsences != 4 {
		t.Errorf("失格後次數要 +1，得到 %d", w.Instances[0].ObservedAbsences)
	}
}

func TestBuildZoneIdentityWriteCarriesLineageAndTransitions(t *testing.T) {
	live := []store.LiveZone{liveZone("Z-P", 100.0, 110.0, identityNow, 0)}
	zones := []store.SRZone{srZone(100.1, 109.9, "SUPPORT"), srZone(100.2, 110.1, "SUPPORT")}
	m := &analysis.ZoneIdentityMatchResult{
		ZoneUIDs:           []string{"Z-C1", "Z-C2"},
		TerminatedPrevious: []string{"Z-P"},
		Relations: []analysis.ZoneIdentityRelation{
			{ParentZoneUID: "Z-P", ChildZoneUID: "Z-C1", Relation: "SPLIT"},
			{ParentZoneUID: "Z-P", ChildZoneUID: "Z-C2", Relation: "SPLIT"},
		},
		RoleTransitions: []analysis.ZoneIdentityRoleTransition{
			{ZoneUID: "Z-C1", Kind: "ROLE_RESOLVED", FromRole: "", ToRole: "SUPPORT"},
		},
	}

	w := buildZoneIdentityWrite("0050", "1d", 42, identityNow, zones, live, m, testUID())

	if len(w.Relations) != 2 {
		t.Fatalf("血緣邊要照抄，got %d", len(w.Relations))
	}
	// 終止的 parent 另外會有一筆 STATE_CHANGE，所以只挑 role 轉換來看。
	var roleTr *store.ZoneTransition
	for i := range w.Transitions {
		if w.Transitions[i].TransitionKind == "ROLE_RESOLVED" {
			roleTr = &w.Transitions[i]
		}
	}
	if roleTr == nil {
		t.Fatalf("role 轉換要照抄，got %+v", w.Transitions)
	}
	// 空的 from_role 要存 NULL 不是 \'\'，否則問不出「哪些是第一次解析出方向」。
	if roleTr.FromRole.Valid {
		t.Error("空的 from_role 應為 NULL")
	}
	if roleTr.ToRole.String != "SUPPORT" {
		t.Errorf("to_role 不對：%+v", roleTr)
	}
}


// ── review 修正的回歸測試 ──

func TestBuildZoneIdentityWriteTerminatesSplitParents(t *testing.T) {
	// **先前完全漏掉這段**：parent 沒被寫回去就永遠停在 ACTIVE，
	// 下次分析照樣被 ListLive 撈出來、幾何上仍配得到自己的 child，
	// 於是每次都重新分裂一次、child 每次拿到全新 uid——身分永遠不會穩定。
	live := []store.LiveZone{liveZone("Z-P", 100.0, 110.0, identityNow, 0)}
	zones := []store.SRZone{srZone(100.1, 109.9, "SUPPORT"), srZone(100.2, 110.1, "SUPPORT")}
	m := &analysis.ZoneIdentityMatchResult{
		ZoneUIDs:           []string{"Z-C1", "Z-C2"},
		TerminatedPrevious: []string{"Z-P"},
		Relations: []analysis.ZoneIdentityRelation{
			{ParentZoneUID: "Z-P", ChildZoneUID: "Z-C1", Relation: "SPLIT"},
			{ParentZoneUID: "Z-P", ChildZoneUID: "Z-C2", Relation: "SPLIT"},
		},
	}

	w := buildZoneIdentityWrite("0050", "1d", 42, identityNow, zones, live, m, testUID())

	var parent *store.ZoneInstance
	for i := range w.Instances {
		if w.Instances[i].ZoneUID == "Z-P" {
			parent = &w.Instances[i]
		}
	}
	if parent == nil {
		t.Fatal("終止的 parent 必須被寫回去，否則它會永遠停在 ACTIVE 並重新進入候選集合")
	}
	if parent.State != "SPLIT" {
		t.Errorf("終態應由血緣型別決定，want SPLIT got %q", parent.State)
	}
	if !parent.EndedAt.Valid {
		t.Error("身分終止要記 ended_at")
	}
}

func TestBuildZoneIdentityWriteOpensIncarnationForDirectionalZone(t *testing.T) {
	// **先前沒有任何人建立一世**，於是 zone_role_incarnations 永遠是空的、
	// incarnation_role 永遠 NULL，matcher 把每個有向 zone 都當「第一次解析出方向」——
	// ROLE_FLIPPED（整個功能的動機）永遠偵測不到。
	zones := []store.SRZone{srZone(104.73, 105.37, "SUPPORT")}
	m := &analysis.ZoneIdentityMatchResult{ZoneUIDs: []string{"Z-NEW"}}

	w := buildZoneIdentityWrite("0050", "1d", 42, identityNow, zones, nil, m, testUID())

	if len(w.Incarnations) != 1 {
		t.Fatalf("有向的新身分要開第一世，got %d", len(w.Incarnations))
	}
	inc := w.Incarnations[0]
	if inc.Seq != 1 || inc.Role != "SUPPORT" || inc.State != "ACTIVE" {
		t.Errorf("第一世不對：%+v", inc)
	}
}

func TestBuildZoneIdentityWriteDoesNotOpenIncarnationForAtZone(t *testing.T) {
	// AT_ZONE 是方向暫時無法解析，不是角色——live 有連續 16 次分析都是 AT_ZONE 的鏈。
	zones := []store.SRZone{srZone(104.73, 105.37, "AT_ZONE")}
	m := &analysis.ZoneIdentityMatchResult{ZoneUIDs: []string{"Z-NEW"}}

	w := buildZoneIdentityWrite("0050", "1d", 42, identityNow, zones, nil, m, testUID())

	if len(w.Incarnations) != 0 {
		t.Fatalf("AT_ZONE 不該開一世，got %+v", w.Incarnations)
	}
}

func TestBuildZoneIdentityWriteFlipClosesOldIncarnationAndOpensNext(t *testing.T) {
	live := []store.LiveZone{liveZone("Z-P", 104.73, 105.37, identityNow, 0)}
	live[0].IncarnationUID = sql.NullString{String: "I-1", Valid: true}
	live[0].IncarnationRole = sql.NullString{String: "SUPPORT", Valid: true}
	live[0].IncarnationSeq = sql.NullInt64{Int64: 1, Valid: true}
	live[0].IncarnationMaxSeq = sql.NullInt64{Int64: 1, Valid: true}
	zones := []store.SRZone{srZone(104.73, 105.37, "RESISTANCE")}
	m := &analysis.ZoneIdentityMatchResult{
		ZoneUIDs: []string{"Z-P"},
		RoleTransitions: []analysis.ZoneIdentityRoleTransition{
			{ZoneUID: "Z-P", Kind: "ROLE_FLIPPED", FromRole: "SUPPORT", ToRole: "RESISTANCE"},
		},
		NextObservedAbsences: map[string]int{"Z-P": 0},
	}

	w := buildZoneIdentityWrite("0050", "1d", 42, identityNow, zones, live, m, testUID())

	if len(w.Incarnations) != 2 {
		t.Fatalf("翻轉要關舊的、開新的，got %d：%+v", len(w.Incarnations), w.Incarnations)
	}
	if w.Incarnations[0].EndReason.String != "ROLE_FLIPPED" {
		t.Errorf("舊的一世要記 end_reason=ROLE_FLIPPED，得到 %+v", w.Incarnations[0])
	}
	// seq 接在**歷來最大值**之後，不是接在未結束那筆之後——
	// 否則 INVALIDATED 過的身分重新解析方向時會重複用 seq 而撞 UNIQUE。
	if w.Incarnations[1].Seq != 2 || w.Incarnations[1].Role != "RESISTANCE" {
		t.Errorf("新的一世不對：%+v", w.Incarnations[1])
	}
}

func TestBuildZoneIdentityWritePushesExpiredPastTheAbsenceLimit(t *testing.T) {
	// 時間軸造成的失格，observed_absences 可能還很小。只 +1 的話下次仍在
	// ListLive 的範圍內，會重複收攤、留下重複的 EXPIRED_BY_ABSENCE。
	seen := identityNow.AddDate(0, 0, -60)
	live := []store.LiveZone{liveZone("Z-EXP", 104.73, 105.37, seen, 1)}
	live[0].IncarnationUID = sql.NullString{String: "I-1", Valid: true}
	live[0].IncarnationRole = sql.NullString{String: "SUPPORT", Valid: true}
	m := &analysis.ZoneIdentityMatchResult{
		ZoneUIDs:             []string{},
		ExpiredPrevious:      []string{"Z-EXP"},
		NextObservedAbsences: map[string]int{"Z-EXP": 2}, // 只 +1，還沒越界
	}

	w := buildZoneIdentityWrite("0050", "1d", 42, identityNow, nil, live, m, testUID())

	if w.Instances[0].ObservedAbsences <= zoneIdentityMaxAbsences {
		t.Errorf("失格後必須推過上限，得到 %d", w.Instances[0].ObservedAbsences)
	}
	// incarnation_uid 要帶上，否則事後問不出「這筆 EXPIRED 收掉的是哪一世」
	found := false
	for _, tr := range w.Transitions {
		if tr.ToState.String == "EXPIRED" && tr.IncarnationUID.String == "I-1" {
			found = true
		}
	}
	if !found {
		t.Error("EXPIRED 的 transition 要帶 incarnation_uid")
	}
}

func TestBuildZoneIdentityWriteRecordsLastObservedRoleNotIncarnationRole(t *testing.T) {
	// last_role 與一世的角色是兩回事。用一世的角色代替 last_role 的話，
	// 一個已經在 AT_ZONE 好幾次的 zone 每次都會被看成「這次才進 AT_ZONE」，
	// 於是每次分析重複記一筆 ROLE_UNRESOLVED；而它回到原方向時，
	// matcher 的「prev.role == AT_ZONE」永遠不成立，ROLE_RESOLVED 也永遠不會寫。
	live := []store.LiveZone{liveZone("Z-P", 104.73, 105.37, identityNow, 0)}
	live[0].LastRole = "SUPPORT"
	live[0].IncarnationRole = sql.NullString{String: "SUPPORT", Valid: true}
	live[0].IncarnationUID = sql.NullString{String: "I-1", Valid: true}
	zones := []store.SRZone{srZone(104.73, 105.37, "AT_ZONE")}
	m := &analysis.ZoneIdentityMatchResult{
		ZoneUIDs:             []string{"Z-P"},
		NextObservedAbsences: map[string]int{"Z-P": 0},
	}

	w := buildZoneIdentityWrite("0050", "1d", 42, identityNow, zones, live, m, testUID())

	if w.Instances[0].LastRole != "AT_ZONE" {
		t.Errorf("last_role 要記這次觀測到的角色，得到 %q", w.Instances[0].LastRole)
	}
	// 一世不因為進 AT_ZONE 而結束——它只是方向暫時無法解析。
	if len(w.Incarnations) != 0 {
		t.Errorf("進 AT_ZONE 不該動一世，得到 %+v", w.Incarnations)
	}
}

func TestBuildZoneIdentityWriteLinksRoleTransitionToIncarnation(t *testing.T) {
	// 翻轉是唯一會關掉一世又開一世的事件，最需要這條連結——
	// 少了它，事後問「這筆翻轉屬於哪一世」只能靠時間戳猜。
	live := []store.LiveZone{liveZone("Z-P", 104.73, 105.37, identityNow, 0)}
	live[0].LastRole = "SUPPORT"
	live[0].IncarnationUID = sql.NullString{String: "I-1", Valid: true}
	live[0].IncarnationRole = sql.NullString{String: "SUPPORT", Valid: true}
	live[0].IncarnationSeq = sql.NullInt64{Int64: 1, Valid: true}
	live[0].IncarnationMaxSeq = sql.NullInt64{Int64: 1, Valid: true}
	zones := []store.SRZone{srZone(104.73, 105.37, "RESISTANCE")}
	m := &analysis.ZoneIdentityMatchResult{
		ZoneUIDs: []string{"Z-P"},
		RoleTransitions: []analysis.ZoneIdentityRoleTransition{
			{ZoneUID: "Z-P", Kind: "ROLE_FLIPPED", FromRole: "SUPPORT", ToRole: "RESISTANCE"},
		},
		NextObservedAbsences: map[string]int{"Z-P": 0},
	}

	w := buildZoneIdentityWrite("0050", "1d", 42, identityNow, zones, live, m, testUID())

	var flip *store.ZoneTransition
	for i := range w.Transitions {
		if w.Transitions[i].TransitionKind == "ROLE_FLIPPED" {
			flip = &w.Transitions[i]
		}
	}
	if flip == nil || !flip.IncarnationUID.Valid {
		t.Fatalf("翻轉的 transition 要帶 incarnation_uid，得到 %+v", flip)
	}
	// 翻轉後這筆屬於**新開的**那一世，不是被關掉的舊的。
	if flip.IncarnationUID.String == "I-1" {
		t.Errorf("應指向新開的一世，卻指向舊的 I-1")
	}
}

func TestBuildZoneIdentityWriteExpiredKeepsOriginalLastSeen(t *testing.T) {
	// upsert 對 last_seen_at 取大，傳 now 等於宣告「這個剛被判定不再認得的身分
	// 今天有被看到」——而 idx_zone_instances_live 正是照 last_seen_at 建的。
	seen := identityNow.AddDate(0, 0, -60)
	live := []store.LiveZone{liveZone("Z-EXP", 104.73, 105.37, seen, 1)}
	m := &analysis.ZoneIdentityMatchResult{
		ZoneUIDs:             []string{},
		ExpiredPrevious:      []string{"Z-EXP"},
		NextObservedAbsences: map[string]int{"Z-EXP": 2},
	}

	w := buildZoneIdentityWrite("0050", "1d", 42, identityNow, nil, live, m, testUID())

	if !w.Instances[0].LastSeenAt.Equal(seen) {
		t.Errorf("失格不該把 last_seen_at 推到今天，want %v got %v",
			seen, w.Instances[0].LastSeenAt)
	}
}

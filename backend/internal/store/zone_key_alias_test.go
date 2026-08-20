package store

import (
	"fmt"
	"testing"
	"time"
)

// zone_key alias 是 T-048 階段 C 的第二把鑰匙：事件身上帶的 zone_key 是「上次那個
// zone 長什麼樣」，本次分析的 key 由這次的 ATR 邊界與 role 算出來，兩者對不上是常態。
// 這組測試盯的是**寫入端**——上限、first_seen_at 不被覆寫、只回活著的身分。

// aliasMaxAbsences 對應呼叫端的 zoneIdentityMaxAbsences（= zone_matcher 的
// MAX_OBSERVED_ABSENCES）。alias 索引與 ListLive 共用同一個次數軸。
const aliasMaxAbsences = 3

func aliasOf(zoneUID, key string, seen time.Time) ZoneKeyAlias {
	return ZoneKeyAlias{
		ZoneUID: zoneUID, ZoneKey: key,
		FirstSeenAt: seen, LastSeenAt: seen,
	}
}

func TestZoneKeyAliasUpsertKeepsFirstSeenAndAdvancesLastSeen(t *testing.T) {
	// first_seen_at 回答「這個 key 從什麼時候開始代表這個身分」，被覆寫就答不出來。
	repo, ctx := newZoneIdentityRepoForTest(t)

	if err := repo.Apply(ctx, ZoneIdentityWrite{
		Instances:  []ZoneInstance{zoneInstance("Z-1", 104.73, 105.37, zoneSeenAt)},
		KeyAliases: []ZoneKeyAlias{aliasOf("Z-1", "SUPPORT:104.7300:105.3700", zoneSeenAt)},
	}); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	later := zoneSeenAt.Add(48 * time.Hour)
	if err := repo.Apply(ctx, ZoneIdentityWrite{
		KeyAliases: []ZoneKeyAlias{aliasOf("Z-1", "SUPPORT:104.7300:105.3700", later)},
	}); err != nil {
		t.Fatalf("second apply: %v", err)
	}

	refs, err := repo.ListKeyAliases(ctx, "0050", "1d", aliasMaxAbsences)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 {
		t.Fatalf("重複觀測不該長出第二列，got %+v", refs)
	}
	if !refs[0].LastSeenAt.Equal(later) {
		t.Errorf("last_seen_at 要推進，got %v", refs[0].LastSeenAt)
	}
}

func TestZoneKeyAliasPrunesToLimit(t *testing.T) {
	// **一定要有上限**：邊界每次分析都由 ATR 重算，不設限這張表會隨分析次數單調成長。
	repo, ctx := newZoneIdentityRepoForTest(t)

	if err := repo.Apply(ctx, ZoneIdentityWrite{
		Instances: []ZoneInstance{zoneInstance("Z-1", 104.73, 105.37, zoneSeenAt)},
	}); err != nil {
		t.Fatal(err)
	}
	// 一次寫 ZoneKeyAliasLimit + 3 個不同的 key，由舊到新。
	total := ZoneKeyAliasLimit + 3
	aliases := make([]ZoneKeyAlias, 0, total)
	for i := 0; i < total; i++ {
		aliases = append(aliases, aliasOf("Z-1",
			fmt.Sprintf("SUPPORT:%03d.0000:105.0000", i),
			zoneSeenAt.Add(time.Duration(i)*time.Hour)))
	}
	if err := repo.Apply(ctx, ZoneIdentityWrite{KeyAliases: aliases}); err != nil {
		t.Fatal(err)
	}

	refs, err := repo.ListKeyAliases(ctx, "0050", "1d", aliasMaxAbsences)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != ZoneKeyAliasLimit {
		t.Fatalf("每個身分只保留最新 %d 筆，got %d", ZoneKeyAliasLimit, len(refs))
	}
	// 留下來的必須是最新的那幾筆，不是任意的幾筆。
	oldest := fmt.Sprintf("SUPPORT:%03d.0000:105.0000", 0)
	for _, r := range refs {
		if r.ZoneKey == oldest {
			t.Errorf("prune 要砍最舊的，%q 不該還在", oldest)
		}
	}
}

func TestZoneKeyAliasListSkipsTerminatedIdentities(t *testing.T) {
	// 把事件掛到收攤過的身分上比關聯失敗更糟：關聯失敗會進 warn 計數，
	// 掛錯身分不會有任何東西報錯。
	repo, ctx := newZoneIdentityRepoForTest(t)

	alive := zoneInstance("Z-ALIVE", 104.73, 105.37, zoneSeenAt)
	dead := zoneInstance("Z-DEAD", 99.10, 100.20, zoneSeenAt)
	dead.State = "SPLIT"
	if err := repo.Apply(ctx, ZoneIdentityWrite{
		Instances: []ZoneInstance{alive, dead},
		KeyAliases: []ZoneKeyAlias{
			aliasOf("Z-ALIVE", "SUPPORT:104.7300:105.3700", zoneSeenAt),
			aliasOf("Z-DEAD", "SUPPORT:99.1000:100.2000", zoneSeenAt),
		},
	}); err != nil {
		t.Fatal(err)
	}

	refs, err := repo.ListKeyAliases(ctx, "0050", "1d", aliasMaxAbsences)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].ZoneUID != "Z-ALIVE" {
		t.Fatalf("只該回還活著的身分，got %+v", refs)
	}
}

func TestZoneKeyAliasListPutsNewestFirstForSameKey(t *testing.T) {
	// 同一個 key 對到多個活身分時，呼叫端用「先到先得」挑，所以 SQL 要保證
	// 最新的在前面——不然挑到誰會由回傳順序決定。
	repo, ctx := newZoneIdentityRepoForTest(t)

	const shared = "SUPPORT:104.7300:105.3700"
	newer := zoneSeenAt.Add(24 * time.Hour)
	if err := repo.Apply(ctx, ZoneIdentityWrite{
		Instances: []ZoneInstance{
			zoneInstance("Z-OLD", 104.73, 105.37, zoneSeenAt),
			zoneInstance("Z-NEW", 104.73, 105.37, zoneSeenAt),
		},
		KeyAliases: []ZoneKeyAlias{
			aliasOf("Z-OLD", shared, zoneSeenAt),
			aliasOf("Z-NEW", shared, newer),
		},
	}); err != nil {
		t.Fatal(err)
	}

	refs, err := repo.ListKeyAliases(ctx, "0050", "1d", aliasMaxAbsences)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 {
		t.Fatalf("兩筆都要回，衝突交給呼叫端計數，got %+v", refs)
	}
	if refs[0].ZoneUID != "Z-NEW" {
		t.Errorf("最新的要排在前面，got %+v", refs)
	}
}

func TestZoneKeyAliasApplyDedupesWithinOneBatch(t *testing.T) {
	// 同一批出現重複的 (zone_uid, zone_key) 在 postgres 會炸
	// （ON CONFLICT DO UPDATE cannot affect row a second time），而兩個 method
	// 算出完全一樣的區間是合法輸入，不該讓整批寫入失敗。
	repo, ctx := newZoneIdentityRepoForTest(t)

	const key = "SUPPORT:104.7300:105.3700"
	if err := repo.Apply(ctx, ZoneIdentityWrite{
		Instances: []ZoneInstance{zoneInstance("Z-1", 104.73, 105.37, zoneSeenAt)},
		KeyAliases: []ZoneKeyAlias{
			aliasOf("Z-1", key, zoneSeenAt),
			aliasOf("Z-1", key, zoneSeenAt.Add(time.Hour)),
		},
	}); err != nil {
		t.Fatalf("重複的 alias 不該讓整批失敗：%v", err)
	}

	refs, err := repo.ListKeyAliases(ctx, "0050", "1d", aliasMaxAbsences)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || !refs[0].LastSeenAt.Equal(zoneSeenAt.Add(time.Hour)) {
		t.Fatalf("去重要取較新的 last_seen_at，got %+v", refs)
	}
}

func TestZoneKeyAliasListExcludesZonesOverAbsenceLimit(t *testing.T) {
	// **次數軸要與 ListLive 一致**：階段 B 的定案是失格只收掉「這一世」，身分本身仍是
	// ACTIVE，所以只看 state 的話，matcher 早就放棄的身分會照樣留在 alias 索引裡。
	// 2026-08-19 的每日階梯實測就是這樣累出 77 筆 alias_ambiguous
	// （見 docs/sr-zone-scoring.md「實測特性」）。
	repo, ctx := newZoneIdentityRepoForTest(t)

	const shared = "SUPPORT:104.7300:105.3700"
	zombie := zoneInstance("Z-ZOMBIE", 104.73, 105.37, zoneSeenAt)
	zombie.ObservedAbsences = aliasMaxAbsences + 1
	live := zoneInstance("Z-LIVE", 104.73, 105.37, zoneSeenAt)
	live.ObservedAbsences = 0

	if err := repo.Apply(ctx, ZoneIdentityWrite{
		Instances: []ZoneInstance{zombie, live},
		KeyAliases: []ZoneKeyAlias{
			aliasOf("Z-ZOMBIE", shared, zoneSeenAt.Add(24*time.Hour)),
			aliasOf("Z-LIVE", shared, zoneSeenAt),
		},
	}); err != nil {
		t.Fatal(err)
	}

	refs, err := repo.ListKeyAliases(ctx, "0050", "1d", aliasMaxAbsences)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 {
		t.Fatalf("超過次數上限的身分不該進 alias 索引（撞號就是這樣來的），got %+v", refs)
	}
	if refs[0].ZoneUID != "Z-LIVE" {
		t.Errorf("留下的要是還在資格窗內的身分，got %+v", refs)
	}
	// 剛好等於上限的身分還要留著：它下一輪才進 matcher 完成收攤，
	// 用 `<` 會讓收攤流程整條變成不可達的死碼（見 ListLive 的說明）。
	atLimit := zoneInstance("Z-AT-LIMIT", 200.0, 201.0, zoneSeenAt)
	atLimit.ObservedAbsences = aliasMaxAbsences
	if err := repo.Apply(ctx, ZoneIdentityWrite{
		Instances:  []ZoneInstance{atLimit},
		KeyAliases: []ZoneKeyAlias{aliasOf("Z-AT-LIMIT", "SUPPORT:200.0000:201.0000", zoneSeenAt)},
	}); err != nil {
		t.Fatal(err)
	}
	refs, err = repo.ListKeyAliases(ctx, "0050", "1d", aliasMaxAbsences)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 {
		t.Fatalf("剛好等於上限的身分要留著（<= 而非 <），got %+v", refs)
	}
}

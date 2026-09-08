package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
)

const testSource = "twse_suspend_listing"

func delistingRepoTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	tmp, err := os.CreateTemp("", "delisting-repo-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	t.Cleanup(func() { os.Remove(tmp.Name()) })

	// ⚠️ 走 NewDB → NewSQLite：FK 是 connection-local，繞過去的話 RESTRICT 會靜默失效。
	db, err := NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: tmp.Name()})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return db
}

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func event(symbol, name string, at time.Time) DelistingEvent {
	return DelistingEvent{Source: testSource, Symbol: symbol, CompanyName: name, DelistedDate: at}
}

func keySet(evs ...DelistingEvent) map[DelistingEventKey]struct{} {
	out := map[DelistingEventKey]struct{}{}
	for _, e := range evs {
		out[e.Key()] = struct{}{}
	}
	return out
}

// insertSymbol 建一筆主檔列並回傳讀出來的 snapshot。
func insertSymbol(t *testing.T, db *sqlx.DB, s SymbolProjectionState) SymbolProjectionState {
	t.Helper()
	_, err := db.Exec(db.Rebind(`
		INSERT INTO stock_symbols (symbol, name, market, security_type, listed_date, is_listed)
		VALUES (?, ?, ?, ?, ?, ?)`),
		s.Symbol, s.Name, s.Market, s.SecurityType, s.ListedDate, s.IsListed)
	if err != nil {
		t.Fatalf("建主檔列: %v", err)
	}
	var out SymbolProjectionState
	if err := db.Get(&out, db.Rebind(`SELECT `+symbolProjectionColumns+
		` FROM stock_symbols WHERE symbol = ?`), s.Symbol); err != nil {
		t.Fatalf("回讀主檔列: %v", err)
	}
	return out
}

func listedStock(symbol string, listedAt time.Time) SymbolProjectionState {
	return SymbolProjectionState{
		Symbol: symbol, Name: "測試股", Market: "上市", SecurityType: "股票",
		ListedDate: NullTime{NullTime: sql.NullTime{Time: listedAt, Valid: true}},
		IsListed:   false,
	}
}

func firstEventID(t *testing.T, db *sqlx.DB, symbol string) uint64 {
	t.Helper()
	var id uint64
	if err := db.Get(&id, db.Rebind(
		`SELECT id FROM delisting_events WHERE symbol = ? ORDER BY id LIMIT 1`), symbol); err != nil {
		t.Fatalf("查事件 id: %v", err)
	}
	return id
}

// ── 事件層 ────────────────────────────────────────────────────────────────

// #10d／#10e／#10f：來源更正的三種轉換，以及「消失 → 重新出現」必須恢復投影資格。
func TestDelistingEventUpsertAndMissingTransitions(t *testing.T) {
	db := delistingRepoTestDB(t)
	repo := NewDelistingRepo(db)
	ctx := context.Background()
	t0 := time.Date(2026, 9, 7, 7, 0, 0, 0, time.UTC)

	a := event("T1", "甲公司", day(2026, 9, 1))
	b := event("T2", "乙公司", day(2026, 8, 1))

	// 第 1 輪：兩筆都在。
	mustTx(t, repo, ctx, func(tx DelistingTx) error {
		for _, e := range []DelistingEvent{a, b} {
			if err := tx.UpsertEvent(e, t0); err != nil {
				return err
			}
		}
		n, err := tx.MarkMissing(testSource, keySet(a, b), t0)
		if err != nil {
			return err
		}
		if n != 0 {
			t.Errorf("首輪不該有缺席，得到 %d", n)
		}
		return nil
	})

	// 第 2 輪：b 從來源消失 → 首次缺席，計 1。
	t1 := t0.Add(24 * time.Hour)
	mustTx(t, repo, ctx, func(tx DelistingTx) error {
		if err := tx.UpsertEvent(a, t1); err != nil {
			return err
		}
		n, err := tx.MarkMissing(testSource, keySet(a), t1)
		if err != nil {
			return err
		}
		if n != 1 {
			t.Errorf("首次缺席應計 1，得到 %d", n)
		}
		return nil
	})
	missingAt := missingTimestamp(t, db, "T2")
	if !missingAt.Valid {
		t.Fatal("T2 應已被標記消失")
	}

	// 第 3 輪：b 持續缺席 → ⛔ 不計、⛔ timestamp 不得改寫。
	// 少了這條，一筆早年消失的事件會讓 job **永久 partial**。
	t2 := t1.Add(24 * time.Hour)
	mustTx(t, repo, ctx, func(tx DelistingTx) error {
		if err := tx.UpsertEvent(a, t2); err != nil {
			return err
		}
		n, err := tx.MarkMissing(testSource, keySet(a), t2)
		if err != nil {
			return err
		}
		if n != 0 {
			t.Errorf("持續缺席不得計入更正，得到 %d", n)
		}
		return nil
	})
	again := missingTimestamp(t, db, "T2")
	if !again.Valid || !again.Time.Equal(missingAt.Time) {
		t.Errorf("持續缺席不得改寫 timestamp：%v → %v", missingAt.Time, again.Time)
	}

	// 第 4 輪：b 重新出現 → missing_from_source_at 必須清回 NULL，恢復投影資格。
	t3 := t2.Add(24 * time.Hour)
	mustTx(t, repo, ctx, func(tx DelistingTx) error {
		for _, e := range []DelistingEvent{a, b} {
			if err := tx.UpsertEvent(e, t3); err != nil {
				return err
			}
		}
		_, err := tx.MarkMissing(testSource, keySet(a, b), t3)
		return err
	})
	if got := missingTimestamp(t, db, "T2"); got.Valid {
		t.Errorf("重新出現後 missing_from_source_at 必須清回 NULL，得到 %v", got.Time)
	}

	// ⛔ 全程不得 DELETE：事件表永遠保留歷史。
	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM delisting_events`); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("事件永不刪除，應仍有 2 筆，得到 %d", n)
	}
}

// 改名：同 (symbol, date) 但 company_name 變了，視為來源更正。
func TestDelistingEventUpsertUpdatesCompanyName(t *testing.T) {
	db := delistingRepoTestDB(t)
	repo := NewDelistingRepo(db)
	ctx := context.Background()
	t0 := time.Date(2026, 9, 7, 7, 0, 0, 0, time.UTC)

	mustTx(t, repo, ctx, func(tx DelistingTx) error {
		return tx.UpsertEvent(event("T1", "舊名", day(2026, 9, 1)), t0)
	})
	mustTx(t, repo, ctx, func(tx DelistingTx) error {
		return tx.UpsertEvent(event("T1", "新名", day(2026, 9, 1)), t0.Add(time.Hour))
	})

	var name string
	if err := db.Get(&name, db.Rebind(`SELECT company_name FROM delisting_events WHERE symbol = ?`), "T1"); err != nil {
		t.Fatal(err)
	}
	if name != "新名" {
		t.Errorf("company_name 應更新為新名，得到 %q", name)
	}
	var cnt int
	if err := db.Get(&cnt, `SELECT COUNT(*) FROM delisting_events`); err != nil {
		t.Fatal(err)
	}
	if cnt != 1 {
		t.Errorf("同 (symbol,date) 是同一筆事件，應仍為 1 筆，得到 %d", cnt)
	}
}

// 日期被改 = 舊事件消失 + 新事件出現（⛔ 舊的不刪除）。
func TestDelistingEventDateChangeCreatesNewEvent(t *testing.T) {
	db := delistingRepoTestDB(t)
	repo := NewDelistingRepo(db)
	ctx := context.Background()
	t0 := time.Date(2026, 9, 7, 7, 0, 0, 0, time.UTC)

	old := event("T1", "甲公司", day(2026, 9, 1))
	mustTx(t, repo, ctx, func(tx DelistingTx) error { return tx.UpsertEvent(old, t0) })

	fresh := event("T1", "甲公司", day(2026, 9, 2))
	mustTx(t, repo, ctx, func(tx DelistingTx) error {
		if err := tx.UpsertEvent(fresh, t0.Add(time.Hour)); err != nil {
			return err
		}
		n, err := tx.MarkMissing(testSource, keySet(fresh), t0.Add(time.Hour))
		if err != nil {
			return err
		}
		if n != 1 {
			t.Errorf("舊事件應被標記缺席一次，得到 %d", n)
		}
		return nil
	})

	var cnt int
	if err := db.Get(&cnt, `SELECT COUNT(*) FROM delisting_events`); err != nil {
		t.Fatal(err)
	}
	if cnt != 2 {
		t.Errorf("日期改變會產生第二筆事件且舊的不刪，應為 2 筆，得到 %d", cnt)
	}
}

// ── 快照層 ────────────────────────────────────────────────────────────────

// bootstrap：沒有任何 accepted 快照時回 nil（不是錯誤）。
func TestLastAcceptedSnapshotBootstrap(t *testing.T) {
	repo := NewDelistingRepo(delistingRepoTestDB(t))
	mustTx(t, repo, context.Background(), func(tx DelistingTx) error {
		got, err := tx.LastAcceptedSnapshot(testSource)
		if err != nil {
			return err
		}
		if got != nil {
			t.Errorf("首次執行應回 nil，得到 %+v", got)
		}
		return nil
	})
}

// ── CAS ───────────────────────────────────────────────────────────────────

// 投影成功，且 ownership 由「空值」建立起來（首次投影）。
func TestProjectFromEmptyState(t *testing.T) {
	db := delistingRepoTestDB(t)
	repo := NewDelistingRepo(db)
	ctx := context.Background()
	t0 := time.Date(2026, 9, 7, 7, 0, 0, 0, time.UTC)

	mustTx(t, repo, ctx, func(tx DelistingTx) error {
		return tx.UpsertEvent(event("T1", "甲公司", day(2026, 9, 1)), t0)
	})
	prior := insertSymbol(t, db, listedStock("T1", day(2020, 1, 1)))
	eventID := firstEventID(t, db, "T1")

	mustTx(t, repo, ctx, func(tx DelistingTx) error {
		ok, err := tx.Project(prior, day(2026, 9, 1), eventID, t0)
		if err != nil {
			return err
		}
		if !ok {
			t.Error("首次投影應成功——空值狀態就是建立 ownership 的起點")
		}
		return nil
	})

	var gotID sql.NullInt64
	if err := db.Get(&gotID, db.Rebind(`SELECT delisted_event_id FROM stock_symbols WHERE symbol = ?`), "T1"); err != nil {
		t.Fatal(err)
	}
	if !gotID.Valid || uint64(gotID.Int64) != eventID {
		t.Errorf("provenance 應指向該事件，得到 %+v", gotID)
	}
}

// #4b｜CAS 落空：讀取後、寫入前主檔被改動 → 必須回 false（歸 X），⛔ 不得寫入。
//
// 逐欄各測一次：name / market / security_type / listed_date / ownership。
// ⛔ 只測其中一個的話，其餘欄位漏出 CAS 條件不會被發現——它們**都是** candidate 的輸入。
func TestProjectCASFailsWhenSnapshotStale(t *testing.T) {
	mutations := map[string]string{
		"name":          `UPDATE stock_symbols SET name = '改過的名字' WHERE symbol = ?`,
		"market":        `UPDATE stock_symbols SET market = '上櫃' WHERE symbol = ?`,
		"security_type": `UPDATE stock_symbols SET security_type = '創新板' WHERE symbol = ?`,
		"listed_date":   `UPDATE stock_symbols SET listed_date = '2021-01-01 00:00:00' WHERE symbol = ?`,
		"is_listed":     `UPDATE stock_symbols SET is_listed = 1 WHERE symbol = ?`,
	}
	for field, mutation := range mutations {
		t.Run(field, func(t *testing.T) {
			db := delistingRepoTestDB(t)
			repo := NewDelistingRepo(db)
			ctx := context.Background()
			t0 := time.Date(2026, 9, 7, 7, 0, 0, 0, time.UTC)

			mustTx(t, repo, ctx, func(tx DelistingTx) error {
				return tx.UpsertEvent(event("T1", "甲公司", day(2026, 9, 1)), t0)
			})
			prior := insertSymbol(t, db, listedStock("T1", day(2020, 1, 1)))
			eventID := firstEventID(t, db, "T1")

			// 模擬 ISIN sync／人工在讀取之後改了主檔。
			if _, err := db.Exec(db.Rebind(mutation), "T1"); err != nil {
				t.Fatalf("製造並行改動: %v", err)
			}

			mustTx(t, repo, ctx, func(tx DelistingTx) error {
				ok, err := tx.Project(prior, day(2026, 9, 1), eventID, t0)
				if err != nil {
					return err
				}
				if ok {
					t.Errorf("%s 被改動後 CAS 應落空（歸 X），但寫入成功了", field)
				}
				return nil
			})

			var gotID sql.NullInt64
			if err := db.Get(&gotID, db.Rebind(`SELECT delisted_event_id FROM stock_symbols WHERE symbol = ?`), "T1"); err != nil {
				t.Fatal(err)
			}
			if gotID.Valid {
				t.Errorf("CAS 落空時不得寫入，delisted_event_id 得到 %d", gotID.Int64)
			}
		})
	}
}

// #20g①｜listed_date 為 NULL 且**無競爭**時，撤銷必須成功。
//
// ⛔ 這是 null-safe 比較的核心：直接組出 `listed_date = NULL` 永遠不為 true，
// 於是**沒有任何競爭也會誤判成 RX**，那些列就永遠撤銷不掉——
// 而 listed_date IS NULL 正是觸發撤銷的原因之一（投影規則 3）。
func TestRevokeWithNullListedDateSucceeds(t *testing.T) {
	db := delistingRepoTestDB(t)
	repo := NewDelistingRepo(db)
	ctx := context.Background()
	t0 := time.Date(2026, 9, 7, 7, 0, 0, 0, time.UTC)

	mustTx(t, repo, ctx, func(tx DelistingTx) error {
		return tx.UpsertEvent(event("T1", "甲公司", day(2026, 9, 1)), t0)
	})
	// listed_date 刻意留 NULL。
	base := listedStock("T1", time.Time{})
	base.ListedDate = NullTime{}
	prior := insertSymbol(t, db, base)
	eventID := firstEventID(t, db, "T1")

	// 先投影成 job-owned。
	mustTx(t, repo, ctx, func(tx DelistingTx) error {
		ok, err := tx.Project(prior, day(2026, 9, 1), eventID, t0)
		if err != nil {
			return err
		}
		if !ok {
			t.Fatal("前置投影應成功")
		}
		return nil
	})

	var owned SymbolProjectionState
	if err := db.Get(&owned, db.Rebind(`SELECT `+symbolProjectionColumns+
		` FROM stock_symbols WHERE symbol = ?`), "T1"); err != nil {
		t.Fatal(err)
	}

	mustTx(t, repo, ctx, func(tx DelistingTx) error {
		ok, err := tx.Revoke(owned, t0)
		if err != nil {
			return err
		}
		if !ok {
			t.Error("listed_date 為 NULL 且無競爭時撤銷必須成功，⛔ 不得誤判成 RX")
		}
		return nil
	})

	date, id, _ := readDelistedPair(t, db, "T1")
	if date.Valid || id.Valid {
		t.Errorf("撤銷後兩欄都應為 NULL，得到 date=%v id=%v", date, id)
	}
}

// #20e②｜R 因名稱不符而準備撤銷，期間名稱被修正成相符 → 撤銷 CAS 必須落空（RX）。
//
// ⛔ 這條證明撤銷 CAS 帶了 name/market/security_type/listed_date，不是只比 ownership。
// 只比 ownership 的話會**清掉一筆已經重新成立的正確投影**。
func TestRevokeCASFailsWhenNameFixedConcurrently(t *testing.T) {
	db := delistingRepoTestDB(t)
	repo := NewDelistingRepo(db)
	ctx := context.Background()
	t0 := time.Date(2026, 9, 7, 7, 0, 0, 0, time.UTC)

	mustTx(t, repo, ctx, func(tx DelistingTx) error {
		return tx.UpsertEvent(event("T1", "甲公司", day(2026, 9, 1)), t0)
	})
	prior := insertSymbol(t, db, listedStock("T1", day(2020, 1, 1)))
	eventID := firstEventID(t, db, "T1")
	mustTx(t, repo, ctx, func(tx DelistingTx) error {
		ok, err := tx.Project(prior, day(2026, 9, 1), eventID, t0)
		if err != nil || !ok {
			t.Fatalf("前置投影應成功: ok=%v err=%v", ok, err)
		}
		return nil
	})

	var owned SymbolProjectionState
	if err := db.Get(&owned, db.Rebind(`SELECT `+symbolProjectionColumns+
		` FROM stock_symbols WHERE symbol = ?`), "T1"); err != nil {
		t.Fatal(err)
	}

	// 讀完 snapshot 之後，名稱被改了（模擬「名稱被修正成相符」）。
	if _, err := db.Exec(db.Rebind(`UPDATE stock_symbols SET name = ? WHERE symbol = ?`), "甲公司股份", "T1"); err != nil {
		t.Fatal(err)
	}

	mustTx(t, repo, ctx, func(tx DelistingTx) error {
		ok, err := tx.Revoke(owned, t0)
		if err != nil {
			return err
		}
		if ok {
			t.Error("名稱在期間被改動後，撤銷 CAS 必須落空（RX），⛔ 不得清掉已重新成立的投影")
		}
		return nil
	})

	date, id, _ := readDelistedPair(t, db, "T1")
	if !date.Valid || !id.Valid {
		t.Error("撤銷落空時投影必須保留不動")
	}
}

// ConfirmProjection：目標值與現值相同時走 current read。
//
// ⛔ 不可改發 no-op UPDATE 再看 affected rows——MySQL 更新成相同值回 0，
// 一個穩定且完全正確的投影會每天被誤判成衝突。
func TestConfirmProjectionMatchesAndDetectsDrift(t *testing.T) {
	db := delistingRepoTestDB(t)
	repo := NewDelistingRepo(db)
	ctx := context.Background()
	t0 := time.Date(2026, 9, 7, 7, 0, 0, 0, time.UTC)

	mustTx(t, repo, ctx, func(tx DelistingTx) error {
		return tx.UpsertEvent(event("T1", "甲公司", day(2026, 9, 1)), t0)
	})
	prior := insertSymbol(t, db, listedStock("T1", day(2020, 1, 1)))
	eventID := firstEventID(t, db, "T1")
	mustTx(t, repo, ctx, func(tx DelistingTx) error {
		ok, err := tx.Project(prior, day(2026, 9, 1), eventID, t0)
		if err != nil || !ok {
			t.Fatalf("前置投影應成功: ok=%v err=%v", ok, err)
		}
		return nil
	})

	var owned SymbolProjectionState
	if err := db.Get(&owned, db.Rebind(`SELECT `+symbolProjectionColumns+
		` FROM stock_symbols WHERE symbol = ?`), "T1"); err != nil {
		t.Fatal(err)
	}

	// 狀態未變 → 必須計 P。
	mustTx(t, repo, ctx, func(tx DelistingTx) error {
		ok, err := tx.ConfirmProjection(owned)
		if err != nil {
			return err
		}
		if !ok {
			t.Error("值相同且狀態未變時必須計 P，⛔ 不得因為「沒有實際更新」就判成衝突")
		}
		return nil
	})

	// 狀態被改動 → 查無，歸 X。
	if _, err := db.Exec(db.Rebind(`UPDATE stock_symbols SET name = ? WHERE symbol = ?`), "別的名字", "T1"); err != nil {
		t.Fatal(err)
	}
	mustTx(t, repo, ctx, func(tx DelistingTx) error {
		ok, err := tx.ConfirmProjection(owned)
		if err != nil {
			return err
		}
		if ok {
			t.Error("狀態已被改動時 guarded read 應查無 → X")
		}
		return nil
	})
}

// 方向二的母體限縮：⛔ 權證不得命中，否則 4445 筆會全部進 A 類。
func TestDelistedListedStocksExcludesWarrants(t *testing.T) {
	db := delistingRepoTestDB(t)
	repo := NewDelistingRepo(db)

	insertSymbol(t, db, SymbolProjectionState{
		Symbol: "T1", Name: "普通股", Market: "上市", SecurityType: "股票", IsListed: false})
	insertSymbol(t, db, SymbolProjectionState{
		Symbol: "T2", Name: "權證", Market: "上市", SecurityType: "上市認購(售)權證", IsListed: false})
	insertSymbol(t, db, SymbolProjectionState{
		Symbol: "T3", Name: "上櫃股", Market: "上櫃", SecurityType: "股票", IsListed: false})
	insertSymbol(t, db, SymbolProjectionState{
		Symbol: "T4", Name: "仍在交易", Market: "上市", SecurityType: "股票", IsListed: true})

	mustTx(t, repo, context.Background(), func(tx DelistingTx) error {
		rows, err := tx.DelistedListedStocks()
		if err != nil {
			return err
		}
		if len(rows) != 1 || rows[0].Symbol != "T1" {
			got := make([]string, 0, len(rows))
			for _, r := range rows {
				got = append(got, r.Symbol)
			}
			t.Errorf("母體應只有已下市的上市股票 [T1]，得到 %v", got)
		}
		return nil
	})
}

// transaction：函式回錯時三種寫入全部 rollback。
func TestWithTxRollsBackAllWrites(t *testing.T) {
	db := delistingRepoTestDB(t)
	repo := NewDelistingRepo(db)
	ctx := context.Background()
	t0 := time.Date(2026, 9, 7, 7, 0, 0, 0, time.UTC)

	prior := insertSymbol(t, db, listedStock("T1", day(2020, 1, 1)))

	wantErr := sql.ErrConnDone
	err := repo.WithTx(ctx, func(tx DelistingTx) error {
		if err := tx.UpsertEvent(event("T1", "甲公司", day(2026, 9, 1)), t0); err != nil {
			return err
		}
		if err := tx.InsertSnapshot(DelistingSourceSnapshot{
			Source: testSource, FetchedAt: t0, RowCount: 1, ContentHash: "h", Accepted: true,
		}); err != nil {
			return err
		}
		_ = prior
		return wantErr // 模擬中途失敗
	})
	if err != wantErr {
		t.Fatalf("錯誤應原樣往上拋，得到 %v", err)
	}

	// 三項都不得留下痕跡。
	var events, snaps int
	if err := db.Get(&events, `SELECT COUNT(*) FROM delisting_events`); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&snaps, `SELECT COUNT(*) FROM delisting_source_snapshots`); err != nil {
		t.Fatal(err)
	}
	if events != 0 || snaps != 0 {
		t.Errorf("交易應完全 rollback，得到 events=%d snapshots=%d", events, snaps)
	}
	date, id, _ := readDelistedPair(t, db, "T1")
	if date.Valid || id.Valid {
		t.Errorf("主檔投影也應 rollback，得到 date=%v id=%v", date, id)
	}
}

func mustTx(t *testing.T, repo DelistingRepo, ctx context.Context, fn func(DelistingTx) error) {
	t.Helper()
	if err := repo.WithTx(ctx, fn); err != nil {
		t.Fatalf("WithTx: %v", err)
	}
}

func missingTimestamp(t *testing.T, db *sqlx.DB, symbol string) NullTime {
	t.Helper()
	var got NullTime
	if err := db.Get(&got, db.Rebind(
		`SELECT missing_from_source_at FROM delisting_events WHERE symbol = ?`), symbol); err != nil {
		t.Fatalf("查 missing_from_source_at: %v", err)
	}
	return got
}

// ── #9／#9b：交易的 all-or-nothing，兩個注入點各一 ────────────────────────

// ⛔ **三項寫入必須在同一個 transaction 內**：事件、投影、快照。
// 「事件更新了但快照沒寫」會讓下一輪拿舊基準比新事件；
// 「投影寫了但事件 rollback」則會做出指向不存在事件的 provenance。
//
// ⚠️ 兩個注入點分開測：**投影中途**（#9）與**快照寫入時**（#9b）。
// 只測其中一個的話，另一段之後才加的寫入不會被涵蓋。
func TestWithTxRollsBackEveryWriteAtBothInjectionPoints(t *testing.T) {
	cases := []struct {
		name string
		// injectAt 決定在哪一步之後回傳錯誤。
		injectAfterProjection bool
	}{
		{"#9 投影中途失敗", true},
		{"#9b 快照寫入時失敗", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := delistingRepoTestDB(t)
			repo := NewDelistingRepo(db)
			ctx := context.Background()
			t0 := day(2026, 9, 7)
			ev := event("T1", "甲公司", day(2026, 9, 1))

			// 前置：先寫一筆事件與一列主檔，**並記下事後要比對的基準**。
			if err := repo.WithTx(ctx, func(tx DelistingTx) error {
				return tx.UpsertEvent(ev, t0)
			}); err != nil {
				t.Fatalf("前置寫事件: %v", err)
			}
			insertSymbol(t, db, listedStock("T1", day(2020, 1, 1)))
			eventID := firstEventID(t, db, "T1")

			type eventRow struct {
				CompanyName string    `db:"company_name"`
				LastSeenAt  time.Time `db:"last_seen_at"`
			}
			var before eventRow
			if err := db.Get(&before, db.Rebind(
				`SELECT company_name, last_seen_at FROM delisting_events WHERE id = ?`),
				eventID); err != nil {
				t.Fatalf("讀事件基準: %v", err)
			}

			wantErr := errors.New("注入的 DB 失敗")
			err := repo.WithTx(ctx, func(tx DelistingTx) error {
				// ①事件層：改名 ＋ 推進 last_seen_at。
				renamed := event("T1", "甲公司改名", day(2026, 9, 1))
				if err := tx.UpsertEvent(renamed, t0.Add(24*time.Hour)); err != nil {
					return err
				}
				// ②投影。
				prior, err := tx.SymbolStates([]string{"T1"})
				if err != nil {
					return err
				}
				ok, err := tx.Project(prior["T1"], day(2026, 9, 1), eventID, t0)
				if err != nil {
					return err
				}
				if !ok {
					t.Fatal("前置：投影應成功")
				}
				if tc.injectAfterProjection {
					return wantErr
				}
				// ③快照——注入點就在它「之後、commit 之前」。
				if err := tx.InsertSnapshot(DelistingSourceSnapshot{
					Source: testSource, FetchedAt: t0, RowCount: 1,
					ContentHash: "abc", Accepted: true,
				}); err != nil {
					return err
				}
				return wantErr
			})
			if !errors.Is(err, wantErr) {
				t.Fatalf("應原樣回傳注入的錯誤，得到 %v", err)
			}

			// ①事件無任何變更——**含 last_seen_at**。
			var after eventRow
			if err := db.Get(&after, db.Rebind(
				`SELECT company_name, last_seen_at FROM delisting_events WHERE id = ?`),
				eventID); err != nil {
				t.Fatalf("回讀事件: %v", err)
			}
			if after.CompanyName != before.CompanyName {
				t.Errorf("company_name 應 rollback：%q → %q", before.CompanyName, after.CompanyName)
			}
			if !after.LastSeenAt.Equal(before.LastSeenAt) {
				t.Errorf("last_seen_at 也必須 rollback：%v → %v", before.LastSeenAt, after.LastSeenAt)
			}

			// ②快照無新增。
			var snaps int
			if err := db.Get(&snaps, `SELECT COUNT(*) FROM delisting_source_snapshots`); err != nil {
				t.Fatal(err)
			}
			if snaps != 0 {
				t.Errorf("快照不得留下，得到 %d 筆", snaps)
			}

			// ③投影未變（兩欄都還是 NULL）。
			var gotDate NullTime
			var gotID sql.NullInt64
			if err := db.Get(&gotDate, db.Rebind(
				`SELECT delisted_date FROM stock_symbols WHERE symbol = ?`), "T1"); err != nil {
				t.Fatal(err)
			}
			if err := db.Get(&gotID, db.Rebind(
				`SELECT delisted_event_id FROM stock_symbols WHERE symbol = ?`), "T1"); err != nil {
				t.Fatal(err)
			}
			if gotDate.Valid || gotID.Valid {
				t.Errorf("投影必須 rollback，得到 date=%v id=%v", gotDate, gotID)
			}
		})
	}
}

// ── #20f①：reconcile 先持鎖 → 拿到 busy 的是競爭者 ──────────────────────

// ⚠️ SQLite 的 writer 是串行的，所以**正常情況不產生 X／RX**——要驗的是
// 「競爭被串行化」而不是 CAS 落空。
//
// ⛔ 兩個 handle 必須指向**同一個檔案 DB**：`MaxOpenConns(1)` 只限單一 handle
// 內的併發，不代表跨 handle 不會競爭。競爭者刻意用 `busy_timeout(0)` 立刻失敗，
// 讓時序是確定的（否則要等滿 5 秒才看得到結果）。
func TestSQLiteWriterContentionReconcileHoldsLockFirst(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/contention.db"
	db, err := NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: path})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	// 外部 writer：獨立 handle、不等待。
	rival, err := sqlx.Connect("sqlite", "file:"+path+"?_pragma=busy_timeout(0)")
	if err != nil {
		t.Fatalf("開競爭者連線: %v", err)
	}
	t.Cleanup(func() { rival.Close() })

	repo := NewDelistingRepo(db)
	var rivalErr error
	err = repo.WithTx(context.Background(), func(tx DelistingTx) error {
		// 先讓本輪取得 write lock。
		if err := tx.UpsertEvent(event("T1", "甲公司", day(2026, 9, 1)), day(2026, 9, 7)); err != nil {
			return err
		}
		// 競爭者此時嘗試寫入 → 它才是拿到 busy 的那個。
		_, rivalErr = rival.Exec(
			`INSERT INTO stock_symbols (symbol, name, market, security_type, is_listed)
			 VALUES ('T2', '別的股', '上市', '股票', 1)`)
		return nil
	})
	if err != nil {
		t.Fatalf("⛔ 先持鎖的本輪必須正常收斂，卻失敗了: %v", err)
	}
	if rivalErr == nil {
		t.Error("⛔ 競爭者應該拿到 busy——沒拿到代表兩邊沒有真的競爭同一個檔案 DB")
	}

	// 本輪的寫入必須留下。
	var events int
	if err := db.Get(&events, `SELECT COUNT(*) FROM delisting_events`); err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Errorf("本輪的事件應已 commit，得到 %d 筆", events)
	}
}

// ── DATE 的身分鍵：⛔ 不做時區換算（2026-09-08 review）────────────────────

// `delisted_date` 是 DATE——沒有時刻也沒有時區，但各 driver 交還的 time.Time
// 帶的 location 不同。MySQL 依 config.yaml 的範例 DSN 會帶 `loc=Asia/Taipei`，
// 於是 2026-09-01 回來是 `00:00 +08`；先 `.UTC()` 再取日期就變成 **2026-08-31**，
// 身分鍵、事件 id 查找與「同一天」比對會整批差一天，而且不會有任何東西報錯。
func TestDateKeyUsesCalendarDateNotUTC(t *testing.T) {
	taipei := time.FixedZone("CST", 8*60*60)
	taipeiMidnight := time.Date(2026, 9, 1, 0, 0, 0, 0, taipei)
	utcMidnight := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	// 前提：這個 fixture 真的會踩到舊寫法（否則測試沒有證明力）。
	if got := taipeiMidnight.UTC().Format("2006-01-02"); got != "2026-08-31" {
		t.Fatalf("fixture 無效：轉 UTC 後應該是前一天，得到 %s", got)
	}

	if got := DateKey(taipeiMidnight); got != "2026-09-01" {
		t.Errorf("DateKey(+08 午夜) = %s, want 2026-09-01", got)
	}
	if !SameCalendarDay(taipeiMidnight, utcMidnight) {
		t.Error("同一個 DATE 在不同 location 下必須是同一天")
	}

	a := DelistingEvent{Source: testSource, Symbol: "T1", DelistedDate: taipeiMidnight}
	b := DelistingEvent{Source: testSource, Symbol: "T1", DelistedDate: utcMidnight}
	if a.Key() != b.Key() {
		t.Errorf("身分鍵不得隨 driver 的 location 改變：%+v vs %+v", a.Key(), b.Key())
	}
}

// ── delisted_event_id 的 JSON 形狀（2026-09-08 review）────────────────────

// ⛔ **不能用 sql.NullInt64**：它會序列化成 `{"Int64":123,"Valid":true}`，
// 而 `omitempty` 對 struct 無效（NULL 時照樣出現一個空殼）。
// `GET /stocks` 系列會把這個欄位直接送到前端。
func TestStockSymbolDelistedEventIDJSONShape(t *testing.T) {
	withID := StockSymbol{Symbol: "2867",
		DelistedEventID: NullInt64{NullInt64: sql.NullInt64{Int64: 123, Valid: true}}}
	got, err := json.Marshal(withID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"delisted_event_id":123`) {
		t.Errorf("有值時應序列化成純數字，得到 %s", got)
	}
	if strings.Contains(string(got), `"Valid"`) || strings.Contains(string(got), `"Int64"`) {
		t.Errorf("⛔ 不得洩漏 sql.NullInt64 的內部欄位：%s", got)
	}

	empty, err := json.Marshal(StockSymbol{Symbol: "2330"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(empty), `"delisted_event_id":null`) {
		t.Errorf("NULL 時應是 null，得到 %s", empty)
	}
}

// ── BEGIN IMMEDIATE（原記於 issue.md I-111，已收斂）────────────────────────────────────────

// openContentionDB 建一個檔案 DB ＋ 一條**獨立 handle** 當外部 writer。
func openContentionDB(t *testing.T) (*sqlx.DB, string) {
	t.Helper()
	path := t.TempDir() + "/immediate.db"
	db, err := NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: path})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return db, path
}

// holdExternalWriteLock 用另一個 handle 持有 write lock 一段時間。
func holdExternalWriteLock(t *testing.T, path string, hold time.Duration) (done <-chan struct{}) {
	t.Helper()
	rival, err := sqlx.Connect("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("開競爭者連線: %v", err)
	}
	tx, err := rival.Beginx()
	if err != nil {
		t.Fatalf("競爭者開交易: %v", err)
	}
	if _, err := tx.Exec(`INSERT INTO stock_symbols (symbol, name, market, security_type, is_listed)
		VALUES ('RIVAL', '競爭者', '上市', '股票', 1)`); err != nil {
		t.Fatalf("競爭者寫入: %v", err)
	}
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		time.Sleep(hold)
		tx.Rollback()
		rival.Close()
	}()
	return ch
}

// ⛔ **這是 BEGIN IMMEDIATE 的核心價值**：外部 writer 短暫持鎖時，本交易要在 `busy_timeout` 內
// **等待然後成功**。deferred transaction（舊版 `BeginTxx`）在這個情境會立刻回
// `SQLITE_BUSY`——先讀後寫的升級不走 busy handler。
func TestWithImmediateTxWaitsForShortExternalLock(t *testing.T) {
	db, path := openContentionDB(t)
	repo := NewDelistingRepo(db)

	done := holdExternalWriteLock(t, path, 1*time.Second)
	start := time.Now()

	err := repo.WithTx(context.Background(), func(tx DelistingTx) error {
		// 先讀後寫——正是舊版升級失敗的形態。
		if _, err := tx.LastAcceptedSnapshot(testSource); err != nil {
			return err
		}
		return tx.UpsertEvent(event("T1", "甲公司", day(2026, 9, 1)), day(2026, 9, 7))
	})
	elapsed := time.Since(start)
	<-done

	if err != nil {
		t.Fatalf("⛔ 短暫持鎖應該等待後成功（BEGIN IMMEDIATE 讓 busy handler 生效），得到 %v", err)
	}
	if elapsed < 500*time.Millisecond {
		t.Errorf("看起來沒有真的等待（%v）——fixture 可能沒造成競爭", elapsed)
	}
	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM delisting_events`); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("等到鎖之後的寫入必須留下，得到 %d 筆", n)
	}
}

// 持鎖**超過** busy_timeout（5 秒）→ 整輪失敗，且**連線沒有外洩**：
// 鎖放掉之後同一個 pool 仍然可用（`MaxOpenConns(1)`，外洩的話下一次會卡死）。
func TestWithImmediateTxFailsAfterBusyTimeoutAndKeepsPoolUsable(t *testing.T) {
	db, path := openContentionDB(t)
	repo := NewDelistingRepo(db)

	done := holdExternalWriteLock(t, path, 6*time.Second)
	err := repo.WithTx(context.Background(), func(tx DelistingTx) error {
		return tx.UpsertEvent(event("T1", "甲公司", day(2026, 9, 1)), day(2026, 9, 7))
	})
	if err == nil {
		t.Fatal("持鎖超過 busy_timeout 時整輪必須失敗")
	}
	<-done

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := repo.WithTx(ctx, func(tx DelistingTx) error {
		return tx.UpsertEvent(event("T2", "乙公司", day(2026, 9, 2)), day(2026, 9, 7))
	}); err != nil {
		t.Fatalf("⛔ 失敗那輪不得吃掉 pool 的唯一連線，後續交易失敗：%v", err)
	}
}

// callback 失敗 → **完整 rollback**，而且連線立刻可以再用。
//
// ⚠️ COMMIT 失敗在 WAL 模式下沒有辦法穩定製造（我們在 BEGIN 時就拿到 write lock），
// 但它與這裡走的是**同一個 defer**：`committed` 沒被設成 true 就會 rollback ＋ 收連線。
func TestWithImmediateTxRollsBackAndReusesConnection(t *testing.T) {
	db, _ := openContentionDB(t)
	repo := NewDelistingRepo(db)
	wantErr := errors.New("注入的 callback 失敗")

	if err := repo.WithTx(context.Background(), func(tx DelistingTx) error {
		if err := tx.UpsertEvent(event("T1", "甲公司", day(2026, 9, 1)), day(2026, 9, 7)); err != nil {
			return err
		}
		return wantErr
	}); !errors.Is(err, wantErr) {
		t.Fatalf("應原樣回傳 callback 的錯誤，得到 %v", err)
	}

	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM delisting_events`); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("callback 失敗必須完整 rollback，得到 %d 筆", n)
	}

	// 連線可重用：⛔ MaxOpenConns(1)，外洩的話這裡會卡到 ctx 逾時。
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := repo.WithTx(ctx, func(tx DelistingTx) error {
		return tx.UpsertEvent(event("T2", "乙公司", day(2026, 9, 2)), day(2026, 9, 7))
	}); err != nil {
		t.Fatalf("rollback 之後連線必須可重用：%v", err)
	}
	if err := db.Get(&n, `SELECT COUNT(*) FROM delisting_events`); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("後續交易應成功寫入 1 筆，得到 %d", n)
	}
}

// ctx 已取消 → 取連線的當下就失敗，⛔ 不得留下開著的交易；pool 仍然可用。
func TestWithImmediateTxWithCanceledContextLeavesPoolUsable(t *testing.T) {
	db, _ := openContentionDB(t)
	repo := NewDelistingRepo(db)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := repo.WithTx(ctx, func(DelistingTx) error {
		t.Error("ctx 已取消時不該進到 callback")
		return nil
	}); err == nil {
		t.Fatal("ctx 已取消時必須回錯")
	}

	okCtx, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()
	if err := repo.WithTx(okCtx, func(tx DelistingTx) error {
		return tx.UpsertEvent(event("T1", "甲公司", day(2026, 9, 1)), day(2026, 9, 7))
	}); err != nil {
		t.Fatalf("取消那次不得吃掉連線：%v", err)
	}
}

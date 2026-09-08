package database_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"
	// ⚠️ 測試執行檔跑在 alpine（`scripts/test-*-migrations.sh` 的 RUN_IMAGE），
	// **image 裡沒有 tzdata**，`loc=Asia/Taipei` 會回 `unknown time zone`。
	// 這個 blank import 把 IANA 時區資料庫嵌進測試 binary。
	// ℹ️ 正式程式不需要它——`pkg/timeutil` 在 LoadLocation 失敗時退回 FixedZone(+8)。
	_ "time/tzdata"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/database"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/store"
)

// migration 076 的**跨連線 CAS 與計數一致性**驗證
//（計畫書 docs/todo.md T-071 的 #4c／#10j／#20g）。
//
// ⚠️ **為什麼不能用 mock，也不能「在 UPDATE 前改 fixture」**：那驗不到 MySQL 的
// transaction snapshot 問題。本專案沒有覆寫 isolation，MySQL 預設 REPEATABLE READ——
// **同一個交易內的一般讀看不到別的連線已提交的變更**。所以這裡一律是
// **兩個真實 connection／transaction 交錯**：主交易先讀 snapshot，競爭者用**另一個
// handle** 提交改動，主交易才做寫入判定。
//
// ⛔ 函式名必須是 `TestPostgresMigrations…` / `TestMySQLMigrations…` 前綴：
// 兩支腳本只編譯 `./internal/database` 並用 `-test.run` 篩名稱。

// ── 共用 fixture ─────────────────────────────────────────────────────────

// secondHandle 開第二個**獨立 pool**當競爭者。
// ⛔ 不共用主 handle：同一個 pool 的兩次操作可能落在同一條 connection 上，
// 那就不是跨連線，MySQL 的 snapshot 陷阱也就驗不到。
func secondHandle(t *testing.T, sqlDriver, dsn string) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Connect(sqlDriver, dsn)
	if err != nil {
		t.Fatalf("開第二個連線: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

type casFixture struct {
	symbol    string
	name      string
	eventID   uint64
	delisted  time.Time
	listedDay time.Time
}

// seedProjectableSymbol 建一筆「已下市的上市股票」＋對應事件。
func seedProjectableSymbol(t *testing.T, db *sqlx.DB, symbol, name string) casFixture {
	t.Helper()
	f := casFixture{
		symbol:    symbol,
		name:      name,
		delisted:  time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		listedDay: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if _, err := db.Exec(db.Rebind(`
		INSERT INTO stock_symbols (symbol, name, market, security_type, is_listed, listed_date)
		VALUES (?, ?, ?, ?, ?, ?)`),
		symbol, name, "上市", "股票", false, f.listedDay); err != nil {
		t.Fatalf("建主檔列: %v", err)
	}
	if _, err := db.Exec(db.Rebind(`
		INSERT INTO delisting_events (source, symbol, company_name, delisted_date)
		VALUES (?, ?, ?, ?)`), "t071_test", symbol, name, f.delisted); err != nil {
		t.Fatalf("建事件: %v", err)
	}
	var id int64
	if err := db.Get(&id, db.Rebind(
		`SELECT id FROM delisting_events WHERE source = ? AND symbol = ?`),
		"t071_test", symbol); err != nil {
		t.Fatalf("查事件 id: %v", err)
	}
	f.eventID = uint64(id)
	return f
}

func projectionOf(t *testing.T, db *sqlx.DB, symbol string) (store.NullTime, sql.NullInt64) {
	t.Helper()
	var row struct {
		DelistedDate    store.NullTime `db:"delisted_date"`
		DelistedEventID sql.NullInt64  `db:"delisted_event_id"`
	}
	if err := db.Get(&row, db.Rebind(
		`SELECT delisted_date, delisted_event_id FROM stock_symbols WHERE symbol = ?`),
		symbol); err != nil {
		t.Fatalf("查投影: %v", err)
	}
	return row.DelistedDate, row.DelistedEventID
}

// ── #4c：跨連線 CAS 的三條 ───────────────────────────────────────────────

func runDelistingCASSuite(t *testing.T, db, other *sqlx.DB) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	// ①目標值相同、狀態未變 → **必須計 P**。
	//
	// ⛔ 這條**不能**改用 no-op UPDATE 看 affected rows：MySQL 把欄位更新成相同值時
	// 回報 0 列，一個穩定且完全正確的投影會每天被誤判成衝突。實作走的是帶同一組 CAS
	// 條件的 guarded read（pg/mysql 是 FOR UPDATE）。
	t.Run("4c-1-confirm-unchanged-counts-as-P", func(t *testing.T) {
		cleanDelisting(t, db)
		f := seedProjectableSymbol(t, db, "T071CAS1", "甲公司")
		repo := store.NewDelistingRepo(db)

		// 先投影，讓它成為 job-owned。
		mustDelistingTx(t, repo, ctx, func(tx store.DelistingTx) error {
			prior := stateOf(t, tx, f.symbol)
			ok, err := tx.Project(prior, f.delisted, f.eventID, now)
			if err != nil {
				return err
			}
			if !ok {
				t.Fatal("前置：首次投影應成功")
			}
			return nil
		})

		mustDelistingTx(t, repo, ctx, func(tx store.DelistingTx) error {
			prior := stateOf(t, tx, f.symbol)
			// 交易進行中，競爭者提交一個**不在 CAS 條件內**的欄位改動。
			// 穩定的投影不該因此被誤判成衝突。
			if _, err := other.Exec(other.Rebind(
				`UPDATE stock_symbols SET last_seen_at = ? WHERE symbol = ?`),
				now, f.symbol); err != nil {
				t.Fatalf("競爭者寫入: %v", err)
			}
			ok, err := tx.ConfirmProjection(prior)
			if err != nil {
				return err
			}
			if !ok {
				t.Error("⛔ 值相同且 CAS 條件未變時必須計 P，卻回報衝突")
			}
			return nil
		})
	})

	// ②投影 CAS 落空 → **X**。
	t.Run("4c-2-projection-cas-lost-is-X", func(t *testing.T) {
		cleanDelisting(t, db)
		f := seedProjectableSymbol(t, db, "T071CAS2", "乙公司")
		repo := store.NewDelistingRepo(db)

		mustDelistingTx(t, repo, ctx, func(tx store.DelistingTx) error {
			prior := stateOf(t, tx, f.symbol)
			// snapshot 讀完之後，另一個連線改掉主檔並提交。
			if _, err := other.Exec(other.Rebind(
				`UPDATE stock_symbols SET name = ? WHERE symbol = ?`),
				"改過的名字", f.symbol); err != nil {
				t.Fatalf("競爭者寫入: %v", err)
			}
			ok, err := tx.Project(prior, f.delisted, f.eventID, now)
			if err != nil {
				return err
			}
			if ok {
				t.Error("⛔ snapshot 過期後投影必須落空（歸 X），卻寫入成功")
			}
			return nil
		})

		if date, id := projectionOf(t, db, f.symbol); date.Valid || id.Valid {
			t.Errorf("CAS 落空時不得留下任何寫入，得到 date=%v id=%v", date, id)
		}
	})

	// ③R 撤銷競爭 → **RX**。
	//
	// ⛔ 撤銷的 CAS 條件必須與投影**完全相同**：只比 ownership 的話，
	// 「因名稱不符而準備撤銷、期間名稱被修正成相符」會清掉已重新成立的投影。
	t.Run("4c-3-revoke-cas-lost-is-RX", func(t *testing.T) {
		cleanDelisting(t, db)
		f := seedProjectableSymbol(t, db, "T071CAS3", "丙公司")
		repo := store.NewDelistingRepo(db)

		mustDelistingTx(t, repo, ctx, func(tx store.DelistingTx) error {
			prior := stateOf(t, tx, f.symbol)
			ok, err := tx.Project(prior, f.delisted, f.eventID, now)
			if err != nil {
				return err
			}
			if !ok {
				t.Fatal("前置：首次投影應成功")
			}
			return nil
		})

		mustDelistingTx(t, repo, ctx, func(tx store.DelistingTx) error {
			prior := stateOf(t, tx, f.symbol)
			if _, err := other.Exec(other.Rebind(
				`UPDATE stock_symbols SET name = ? WHERE symbol = ?`),
				"改回相符的名字", f.symbol); err != nil {
				t.Fatalf("競爭者寫入: %v", err)
			}
			ok, err := tx.Revoke(prior, now)
			if err != nil {
				return err
			}
			if ok {
				t.Error("⛔ snapshot 過期後撤銷必須落空（歸 RX），卻清除成功")
			}
			return nil
		})

		date, id := projectionOf(t, db, f.symbol)
		if !date.Valid || !id.Valid {
			t.Errorf("RX 時投影必須原封不動，得到 date=%v id=%v", date, id)
		}
	})
}

// stateOf 讀單一 symbol 的 snapshot（走 repo 的真實查詢）。
func stateOf(t *testing.T, tx store.DelistingTx, symbol string) store.SymbolProjectionState {
	t.Helper()
	states, err := tx.SymbolStates([]string{symbol})
	if err != nil {
		t.Fatalf("讀 snapshot: %v", err)
	}
	state, ok := states[symbol]
	if !ok {
		t.Fatalf("snapshot 查無 %s", symbol)
	}
	return state
}

// seedJobRun 建一筆 job_run 並回傳 id。
//
// ⚠️ 名稱固定用 `delisting_reconcile_test`，那正是 cleanDelisting 會清掉的那個
//（⛔ 不要用正式的 `delisting_reconcile`，清理會漏掉）。
func seedJobRun(t *testing.T, db *sqlx.DB) uint64 {
	t.Helper()
	if _, err := db.Exec(db.Rebind(`
		INSERT INTO job_runs (job_name, status, symbols_total, symbols_failed, started_at)
		VALUES (?, ?, ?, ?, ?)`),
		"delisting_reconcile_test", "running", 0, 0, time.Now().UTC()); err != nil {
		t.Fatalf("建 job_run: %v", err)
	}
	var id uint64
	if err := db.Get(&id, db.Rebind(
		`SELECT id FROM job_runs WHERE job_name = ? ORDER BY id DESC`+" LIMIT 1"),
		"delisting_reconcile_test"); err != nil {
		t.Fatalf("查 job_run id: %v", err)
	}
	return id
}

func mustDelistingTx(t *testing.T, repo store.DelistingRepo, ctx context.Context, fn func(store.DelistingTx) error) {
	t.Helper()
	if err := repo.WithTx(ctx, fn); err != nil {
		t.Fatalf("交易失敗: %v", err)
	}
}

// ── #4c④：MySQL 專屬，只有大小寫差異的並行改動 ──────────────────────────

// runMySQLCollationCASCases 驗 `BINARY col = BINARY ?`。
//
// ⛔ **fixture 必須把實際受測的欄位 ALTER 成指定的 CI collation 再還原**：
// 依賴環境預設的話，剛好跑在 case-sensitive 的 database 上時這個測試什麼都證明不了。
// ⛔ **也不可用「環境不合就 skip」當出口**——那會讓一個沒有證明力的測試被關閉條件
// 當成通過。所以確認不到 CI 語意時是 **Fatal**，不是 Skip。
func runMySQLCollationCASCases(t *testing.T, db, other *sqlx.DB) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	const ciCollation = "utf8mb4_0900_ai_ci"
	var original string
	if err := db.Get(&original, `
		SELECT COLLATION_NAME FROM information_schema.COLUMNS
		 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'stock_symbols'
		   AND COLUMN_NAME = 'name'`); err != nil {
		t.Fatalf("查 name 欄位的 collation: %v", err)
	}
	alter := func(collation string) {
		stmt := fmt.Sprintf(
			"ALTER TABLE stock_symbols MODIFY COLUMN name VARCHAR(120) "+
				"CHARACTER SET utf8mb4 COLLATE %s NOT NULL", collation)
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("ALTER name COLLATE %s: %v", collation, err)
		}
	}
	alter(ciCollation)
	t.Cleanup(func() { alter(original) })

	// 前置：確認這個 collation 真的是大小寫不敏感的，否則 ④a／④b 驗到的是空氣。
	cleanDelisting(t, db)
	probe := seedProjectableSymbol(t, db, "T071COLL0", "ABC-KY")
	var hits int
	if err := db.Get(&hits, db.Rebind(
		`SELECT COUNT(*) FROM stock_symbols WHERE symbol = ? AND name = ?`),
		probe.symbol, "abc-ky"); err != nil {
		t.Fatalf("collation 前置查詢: %v", err)
	}
	if hits != 1 {
		t.Fatalf("fixture 無效：name 欄位在 %s 之下必須是大小寫不敏感的"+
			"（原 collation=%s，'abc-ky' 命中 %d 列）", ciCollation, original, hits)
	}

	cases := []struct {
		name   string
		symbol string
		// revoke 為 true 時驗撤銷路徑（④b），否則驗投影路徑（④a）。
		revoke bool
	}{
		{"4c-4a-case-only-change-blocks-projection", "T071COLL1", false},
		{"4c-4b-case-only-change-blocks-revoke", "T071COLL2", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cleanDelisting(t, db)
			alter(ciCollation) // cleanDelisting 不影響 schema，但重跑保險
			f := seedProjectableSymbol(t, db, tc.symbol, "ABC-KY")
			repo := store.NewDelistingRepo(db)

			if tc.revoke {
				mustDelistingTx(t, repo, ctx, func(tx store.DelistingTx) error {
					prior := stateOf(t, tx, f.symbol)
					ok, err := tx.Project(prior, f.delisted, f.eventID, now)
					if err != nil {
						return err
					}
					if !ok {
						t.Fatal("前置：首次投影應成功")
					}
					return nil
				})
			}

			mustDelistingTx(t, repo, ctx, func(tx store.DelistingTx) error {
				prior := stateOf(t, tx, f.symbol)
				// **只有大小寫不同**——CI collation 下 `name = ?` 仍然成立，
				// 所以只有 BINARY 比較擋得住。
				if _, err := other.Exec(other.Rebind(
					`UPDATE stock_symbols SET name = ? WHERE symbol = ?`),
					"abc-KY", f.symbol); err != nil {
					t.Fatalf("競爭者寫入: %v", err)
				}
				if tc.revoke {
					ok, err := tx.Revoke(prior, now)
					if err != nil {
						return err
					}
					if ok {
						t.Errorf("⛔ 只有大小寫差異的並行改動必須讓撤銷落空（RX）"+
							"（collation=%s）", ciCollation)
					}
					return nil
				}
				ok, err := tx.Project(prior, f.delisted, f.eventID, now)
				if err != nil {
					return err
				}
				if ok {
					t.Errorf("⛔ 只有大小寫差異的並行改動必須讓投影落空（X）"+
						"（collation=%s）", ciCollation)
				}
				return nil
			})
		})
	}
}

// ── #20g：nullable 欄位的 null-safe 比較 ────────────────────────────────

// ⛔ 組出 `listed_date = NULL` 的話那個條件永遠不為真，**無競爭也會落空**，
// 那些列就永遠撤銷不掉。
func runDelistingNullSafeCAS(t *testing.T, db, other *sqlx.DB) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	seedNullListedDate := func(symbol string) casFixture {
		f := seedProjectableSymbol(t, db, symbol, "空上市日公司")
		if _, err := db.Exec(db.Rebind(
			`UPDATE stock_symbols SET listed_date = NULL WHERE symbol = ?`), symbol); err != nil {
			t.Fatalf("清 listed_date: %v", err)
		}
		f.listedDay = time.Time{}
		return f
	}

	// ①三 engine 都要：snapshot 的 listed_date 為 NULL 且**無競爭** → 必須成功 R。
	t.Run("20g-1-null-listed-date-revokes-without-competition", func(t *testing.T) {
		cleanDelisting(t, db)
		f := seedNullListedDate("T071NULL1")
		repo := store.NewDelistingRepo(db)

		// 先用「有 listed_date」的狀態投影，再清成 NULL，模擬真實的人工／來源改動。
		mustDelistingTx(t, repo, ctx, func(tx store.DelistingTx) error {
			prior := stateOf(t, tx, f.symbol)
			if prior.ListedDate.Valid {
				t.Fatal("前置：listed_date 應已是 NULL")
			}
			ok, err := tx.Project(prior, f.delisted, f.eventID, now)
			if err != nil {
				return err
			}
			if !ok {
				t.Fatal("⛔ listed_date 為 NULL 時投影也必須成功（null-safe 比較）")
			}
			return nil
		})

		mustDelistingTx(t, repo, ctx, func(tx store.DelistingTx) error {
			prior := stateOf(t, tx, f.symbol)
			ok, err := tx.Revoke(prior, now)
			if err != nil {
				return err
			}
			if !ok {
				t.Error("⛔ 無競爭時撤銷必須成功——組出 `listed_date = NULL` 會讓它永遠撤不掉")
			}
			return nil
		})

		if date, id := projectionOf(t, db, f.symbol); date.Valid || id.Valid {
			t.Errorf("撤銷後兩欄都應是 NULL，得到 date=%v id=%v", date, id)
		}
	})

	// ②僅 PG／MySQL：真實跨連線的 NULL ↔ 非 NULL 改動 → 必須落空成 RX。
	t.Run("20g-2-null-to-value-change-makes-revoke-fail", func(t *testing.T) {
		cleanDelisting(t, db)
		f := seedNullListedDate("T071NULL2")
		repo := store.NewDelistingRepo(db)

		mustDelistingTx(t, repo, ctx, func(tx store.DelistingTx) error {
			prior := stateOf(t, tx, f.symbol)
			ok, err := tx.Project(prior, f.delisted, f.eventID, now)
			if err != nil {
				return err
			}
			if !ok {
				t.Fatal("前置：投影應成功")
			}
			return nil
		})

		mustDelistingTx(t, repo, ctx, func(tx store.DelistingTx) error {
			prior := stateOf(t, tx, f.symbol)
			// NULL → 非 NULL：ISIN sync 補上了 listed_date。
			if _, err := other.Exec(other.Rebind(
				`UPDATE stock_symbols SET listed_date = ? WHERE symbol = ?`),
				time.Date(2021, 3, 4, 0, 0, 0, 0, time.UTC), f.symbol); err != nil {
				t.Fatalf("競爭者寫入: %v", err)
			}
			ok, err := tx.Revoke(prior, now)
			if err != nil {
				return err
			}
			if ok {
				t.Error("⛔ listed_date 由 NULL 變成有值後撤銷必須落空（RX）")
			}
			return nil
		})

		date, id := projectionOf(t, db, f.symbol)
		if !date.Valid || !id.Valid {
			t.Errorf("RX 時投影必須原封不動，得到 date=%v id=%v", date, id)
		}
	})
}

// ── #10j：source_corrected 的三 engine 一致性 ───────────────────────────

// delistingSourceStub 讓每一輪餵不同的 CSV 內容。
type delistingSourceStub struct{ rows []market.DelistingEventRow }

func (s *delistingSourceStub) FetchDelistings(context.Context) (market.DelistingSourceResult, error) {
	symbols := map[string]struct{}{}
	for _, r := range s.rows {
		symbols[r.Symbol] = struct{}{}
	}
	return market.DelistingSourceResult{
		Rows: append([]market.DelistingEventRow(nil), s.rows...),
		// ⚠️ 縮水防護比的是 RowCount，所以它必須跟著 rows 變動。
		RowCount:        len(s.rows),
		DistinctSymbols: len(symbols),
		ContentHash:     fmt.Sprintf("h%d", len(s.rows)),
	}, nil
}

type readyStub struct{}

func (readyStub) StockSymbolSyncSucceededToday(context.Context, time.Time) (bool, error) {
	return true, nil
}

// runSourceCorrectedSuite 驗四個分支。
//
// ⚠️ ②③是**唯一仍依賴 affected rows 的分支**（`MarkMissing` 帶
// `WHERE missing_from_source_at IS NULL`），不驗等於沒驗到那條路徑；
// ①會在誤用主 upsert 的 `RowsAffected` 時等於既有事件數
//（MySQL 未變 0／更新 2／插入 1；postgres 一律 1）。
func runSourceCorrectedSuite(t *testing.T, db *sqlx.DB) {
	t.Helper()
	cleanDelisting(t, db)
	ctx := context.Background()

	day := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	rowA := market.DelistingEventRow{Symbol: "T071SC1", CompanyName: "甲公司", DelistedDate: day}
	rowB := market.DelistingEventRow{Symbol: "T071SC2", CompanyName: "乙公司", DelistedDate: day}

	source := &delistingSourceStub{rows: []market.DelistingEventRow{rowA, rowB}}
	rec := market.NewDelistingReconciler(source, store.NewDelistingRepo(db), readyStub{}, zap.NewNop())

	run := func(t *testing.T, round int, acceptShrink bool) market.DelistingOutcome {
		t.Helper()
		// ⚠️ **每輪都要一筆真的 job_run**：快照的 job_run_id 有 FK，
		// 隨便塞一個不存在的 id 會被 constraint 擋掉（SQLite 開了 foreign_keys）。
		out, err := rec.Reconcile(ctx, seedJobRun(t, db), acceptShrink,
			time.Now().UTC().Truncate(time.Second))
		if err != nil {
			t.Fatalf("第 %d 輪失敗: %v", round, err)
		}
		return out
	}

	// 第 1 輪：建立基準（首次匯入，兩筆都是新的）。
	run(t, 1, false)

	// ①無更正的一輪（只有 last_seen_at 推進）→ 0。
	if out := run(t, 2, false); out.SourceCorrected != 0 {
		t.Errorf("①內容沒變的一輪 source_corrected 應為 0，得到 %d"+
			"（誤用主 upsert 的 RowsAffected 時會等於事件數）", out.SourceCorrected)
	}

	// ②首次缺席 → 1。
	source.rows = []market.DelistingEventRow{rowA}
	if out := run(t, 3, true); out.SourceCorrected != 1 {
		t.Errorf("②首次缺席 source_corrected 應為 1，得到 %d", out.SourceCorrected)
	}
	var firstMissing time.Time
	if err := db.Get(&firstMissing, db.Rebind(
		`SELECT missing_from_source_at FROM delisting_events WHERE symbol = ?`),
		rowB.Symbol); err != nil {
		t.Fatalf("查 missing_from_source_at: %v", err)
	}

	// ③同一事件第二輪持續缺席 → 0 且 timestamp 不變。
	if out := run(t, 4, true); out.SourceCorrected != 0 {
		t.Errorf("③持續缺席不得重複計數，得到 %d（會讓 job 永久 partial）", out.SourceCorrected)
	}
	var stillMissing time.Time
	if err := db.Get(&stillMissing, db.Rebind(
		`SELECT missing_from_source_at FROM delisting_events WHERE symbol = ?`),
		rowB.Symbol); err != nil {
		t.Fatalf("查 missing_from_source_at: %v", err)
	}
	if !stillMissing.Equal(firstMissing) {
		t.Errorf("③持續缺席不得改寫 missing_from_source_at：%v → %v", firstMissing, stillMissing)
	}

	// ④同筆同時重現＋改名 → 1（不是 2）。
	renamed := rowB
	renamed.CompanyName = "乙公司-改名"
	source.rows = []market.DelistingEventRow{rowA, renamed}
	if out := run(t, 5, false); out.SourceCorrected != 1 {
		t.Errorf("④同筆重現＋改名應收斂成 1，得到 %d", out.SourceCorrected)
	}
}

// ── engine 進入點 ───────────────────────────────────────────────────────

func TestPostgresMigrationsDelistingCAS(t *testing.T) {
	dsn := skipUnlessDSN(t, "POSTGRES_MIGRATION_DSN", "scripts/test-postgres-migrations.sh")
	db := migratedDB(t, "postgres", "pgx", dsn)
	runDelistingCASSuite(t, db, secondHandle(t, "pgx", dsn))
}

func TestPostgresMigrationsDelistingNullSafeCAS(t *testing.T) {
	dsn := skipUnlessDSN(t, "POSTGRES_MIGRATION_DSN", "scripts/test-postgres-migrations.sh")
	db := migratedDB(t, "postgres", "pgx", dsn)
	runDelistingNullSafeCAS(t, db, secondHandle(t, "pgx", dsn))
}

func TestPostgresMigrationsDelistingSourceCorrected(t *testing.T) {
	dsn := skipUnlessDSN(t, "POSTGRES_MIGRATION_DSN", "scripts/test-postgres-migrations.sh")
	runSourceCorrectedSuite(t, migratedDB(t, "postgres", "pgx", dsn))
}

func TestMySQLMigrationsDelistingCAS(t *testing.T) {
	dsn := skipUnlessDSN(t, "MYSQL_MIGRATION_DSN", "scripts/test-mysql-migrations.sh")
	db := migratedDB(t, "mysql", "mysql", dsn)
	other := secondHandle(t, "mysql", dsn)
	runDelistingCASSuite(t, db, other)
	// ④a／④b 是 MySQL 專屬：collation 可能大小寫不敏感。
	runMySQLCollationCASCases(t, db, other)
}

func TestMySQLMigrationsDelistingNullSafeCAS(t *testing.T) {
	dsn := skipUnlessDSN(t, "MYSQL_MIGRATION_DSN", "scripts/test-mysql-migrations.sh")
	db := migratedDB(t, "mysql", "mysql", dsn)
	runDelistingNullSafeCAS(t, db, secondHandle(t, "mysql", dsn))
}

func TestMySQLMigrationsDelistingSourceCorrected(t *testing.T) {
	dsn := skipUnlessDSN(t, "MYSQL_MIGRATION_DSN", "scripts/test-mysql-migrations.sh")
	runSourceCorrectedSuite(t, migratedDB(t, "mysql", "mysql", dsn))
}

// SQLite 版由 backend/scripts/test.sh 每次跑。
//
// ⚠️ **SQLite 驗的是不同的東西**：writer 是串行的，所以正常情況不產生 X／RX。
// 這裡只驗 #20g① 的 null-safe（三 engine 都要）與 #10j 的四個分支。
func TestSQLiteDelistingSourceCorrectedAndNullSafe(t *testing.T) {
	dir := t.TempDir()
	db, err := store.NewSQLite(dir + "/delisting_cas.db")
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("sqlite migration 失敗: %v", err)
	}

	t.Run("source-corrected", func(t *testing.T) { runSourceCorrectedSuite(t, db) })
	t.Run("null-safe-revoke", func(t *testing.T) {
		cleanDelisting(t, db)
		ctx := context.Background()
		now := time.Now().UTC().Truncate(time.Second)
		f := seedProjectableSymbol(t, db, "T071NULLS", "空上市日公司")
		if _, err := db.Exec(db.Rebind(
			`UPDATE stock_symbols SET listed_date = NULL WHERE symbol = ?`), f.symbol); err != nil {
			t.Fatalf("清 listed_date: %v", err)
		}
		repo := store.NewDelistingRepo(db)

		mustDelistingTx(t, repo, ctx, func(tx store.DelistingTx) error {
			prior := stateOf(t, tx, f.symbol)
			ok, err := tx.Project(prior, f.delisted, f.eventID, now)
			if err != nil {
				return err
			}
			if !ok {
				t.Fatal("⛔ listed_date 為 NULL 時投影也必須成功")
			}
			return nil
		})
		mustDelistingTx(t, repo, ctx, func(tx store.DelistingTx) error {
			prior := stateOf(t, tx, f.symbol)
			ok, err := tx.Revoke(prior, now)
			if err != nil {
				return err
			}
			if !ok {
				t.Error("⛔ 無競爭時撤銷必須成功（null-safe 比較）")
			}
			return nil
		})
	})
}

// ── MySQL 帶 loc=Asia/Taipei 時 DATE 不得差一天（2026-09-08 review）──────

// `config.yaml` 的 MySQL DSN 範例是 `parseTime=true&loc=Asia%2FTaipei`，
// 而 `scripts/test-mysql-migrations.sh` 的 DSN **沒帶 loc**——所以這個 class 的 bug
// 在既有測試裡永遠測不到。這裡刻意另開一個帶 loc 的 handle。
//
// ⛔ 身分鍵若先 `.UTC()` 再取日期，`2026-09-01 00:00 +08` 會變成 **2026-08-31**：
// present key、事件 id 查找與 sameDay 比對整批差一天，而且不會有任何東西報錯。
func TestMySQLMigrationsDelistingDateKeyUnderTaipeiLoc(t *testing.T) {
	dsn := skipUnlessDSN(t, "MYSQL_MIGRATION_DSN", "scripts/test-mysql-migrations.sh")
	db := migratedDB(t, "mysql", "mysql", dsn)
	tz := secondHandle(t, "mysql", dsn+"&loc=Asia%2FTaipei")

	cleanDelisting(t, db)
	seedProjectableSymbol(t, db, "T071TZ", "時區測試")

	var ev store.DelistingEvent
	if err := tz.Get(&ev, tz.Rebind(`
		SELECT source, symbol, company_name, delisted_date, first_seen_at, last_seen_at,
		       missing_from_source_at
		  FROM delisting_events WHERE symbol = ?`), "T071TZ"); err != nil {
		t.Fatalf("用 Asia/Taipei 連線讀事件: %v", err)
	}

	// 前提：確認 loc 真的生效（否則這個測試沒有證明力）。
	if _, offset := ev.DelistedDate.Zone(); offset != 8*60*60 {
		t.Fatalf("fixture 無效：loc=Asia/Taipei 沒生效，offset=%d 秒", offset)
	}
	if got := ev.DelistedDate.UTC().Format("2006-01-02"); got != "2026-08-31" {
		t.Fatalf("fixture 無效：轉 UTC 後應該是前一天，得到 %s", got)
	}

	if got := ev.Key().DelistedDate; got != "2026-09-01" {
		t.Errorf("身分鍵差了一天：得到 %s，want 2026-09-01", got)
	}
}

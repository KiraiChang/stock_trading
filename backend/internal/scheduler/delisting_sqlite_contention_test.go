package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/store"
)

// SQLite 的兩種 busy 情境（計畫書 docs/todo.md T-071 的 #20f②③ 與 #20f2）。
//
// ⚠️ **這兩種要分開**：
//   - **#20f｜外部 writer**（另一個 handle 持有 write lock）→ 我們在 `busy_timeout`
//     內等待，逾時才整輪 `failed`。
//   - **#20f2｜同一個 pool 的排隊**（`MaxOpenConns(1)`）→ 第二個呼叫在 `database/sql`
//     層排隊，⛔ **不會拿到 `SQLITE_BUSY`**，等到的是 context 逾時。
//
// ⛔ **時序都是寫死的**：`startRun` 必須**先取得有效 runID**，否則
// `runID = 0` 時 `Finish` 會用 `WHERE id = 0` 靜默更新零列
//（`job_run_repo.go` 不檢查 affected rows），DB 裡根本沒有可驗證的 `failed` 紀錄，
// 測試會驗到空氣。

// contentionSourceStub 在 FetchDelistings 當下呼叫 hook——那個時間點
// **保證在 startRun 之後、業務交易之前**。
type contentionSourceStub struct {
	mu    sync.Mutex
	calls int
	hook  func()
}

func (s *contentionSourceStub) FetchDelistings(context.Context) (market.DelistingSourceResult, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	if s.hook != nil {
		s.hook()
	}
	return market.DelistingSourceResult{
		Rows: []market.DelistingEventRow{{
			Symbol: "T071", CompanyName: "測試公司",
			DelistedDate: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		}},
		RowCount: 1, DistinctSymbols: 1, ContentHash: "h1",
	}, nil
}

func (s *contentionSourceStub) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// newSQLiteContentionFixture 建**正式拓撲**的 SQLite：走 store.NewDB → NewSQLite，
// 所以帶 `busy_timeout(5000)`、`foreign_keys(1)` 與 `MaxOpenConns(1)`。
func newSQLiteContentionFixture(
	t *testing.T, source market.DelistingSource,
) (*Scheduler, *sqlx.DB, string) {
	t.Helper()
	path := t.TempDir() + "/contention.db"
	db, err := store.NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: path})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	s := &Scheduler{jobRuns: store.NewJobRunRepo(db), log: zap.NewNop()}
	s.SetDelistingReconcile(
		market.NewDelistingReconciler(
			source, store.NewDelistingRepo(db), delistingReadinessStub{ready: true}, zap.NewNop()),
		config.DelistingConfig{Enabled: true},
	)
	return s, db, path
}

// holdWriteLock 用**另一個 handle**（模擬 ISIN sync 之類的外部 writer）持有 write lock
// 一段時間。⚠️ 兩個 handle 必須指向同一個檔案 DB：`MaxOpenConns(1)` 只限單一 handle
// 內的併發，不代表跨 handle 不會競爭。
func holdWriteLock(t *testing.T, path string, hold time.Duration) (release func()) {
	t.Helper()
	rival, err := sqlx.Connect("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("開競爭者連線: %v", err)
	}
	tx, err := rival.Beginx()
	if err != nil {
		t.Fatalf("競爭者開交易: %v", err)
	}
	// 真的寫一筆才會拿到 write lock（BEGIN 本身是 deferred 的）。
	if _, err := tx.Exec(`INSERT INTO stock_symbols (symbol, name, market, security_type, is_listed)
		VALUES ('T071RIVAL', '競爭者', '上市', '股票', 1)`); err != nil {
		t.Fatalf("競爭者寫入: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		time.Sleep(hold)
		tx.Rollback()
		rival.Close()
	}()
	return func() { <-done }
}

func jobRunRows(t *testing.T, db *sqlx.DB) []store.JobRun {
	t.Helper()
	var rows []store.JobRun
	if err := db.Select(&rows,
		`SELECT id, job_name, status, symbols_total, symbols_failed, error, started_at, finished_at
		   FROM job_runs ORDER BY id`); err != nil {
		t.Fatalf("查 job_runs: %v", err)
	}
	return rows
}

// #20f②｜startRun 成功之後，外部 writer 才持鎖。
//
// 整輪 `failed`，並把**那筆既有的** run 寫成 `failed`。
// ⛔ 不歸 X／RX：那是 CAS 落空的分類，這裡根本沒寫進去。
//
// ⚠️ **實測（2026-09-07）與計畫書的假設不同，這裡釘住的是實際行為**：
// 業務交易**不會**在 `busy_timeout` 內等待——`WithTx` 是 deferred transaction，
// 先讀（`LastAcceptedSnapshot`／`LoadEvents`）後寫，升級成 writer 時 SQLite
// **不呼叫 busy handler**（重試會破壞它已經拿到的讀快照），直接回 `SQLITE_BUSY`。
// `busy_timeout` 只保護**單句寫入**——也就是後面 `finishRunStatus` 那一句。
// 現況與影響記在 `docs/issue.md` I-111。
func TestDelistingReconcileExternalWriterAfterStartRunWritesFailed(t *testing.T) {
	var release func()
	source := &contentionSourceStub{}
	s, db, path := newSQLiteContentionFixture(t, source)
	// 持鎖 1.5 秒：業務交易會**立刻**失敗（見上方說明），而 finishRunStatus 那一句
	// 走 busy handler，等 1.5 秒拿到鎖後仍寫得回去（它的 writer timeout 是 10 秒，
	// 連線的 busy_timeout 是 5 秒，兩個都夠）。
	source.hook = func() { release = holdWriteLock(t, path, 1500*time.Millisecond) }

	s.RunDelistingReconcile()
	if release != nil {
		release()
	}

	rows := jobRunRows(t, db)
	if len(rows) != 1 {
		t.Fatalf("應**只有** startRun 那一筆紀錄，得到 %d 筆：%+v", len(rows), rows)
	}
	if rows[0].JobName != delistingReconcileJobName || rows[0].Status != "failed" {
		t.Fatalf("那筆既有的 run 必須被寫成 failed，得到 %+v", rows[0])
	}
	// symbols_failed 恆為 0——⛔ 不得用假的 1/1 去湊。
	if rows[0].SymbolsFailed != 0 {
		t.Errorf("symbols_failed = %d, want 0", rows[0].SymbolsFailed)
	}
	if !rows[0].Error.Valid || rows[0].Error.String == "" {
		t.Error("整輪失敗必須寫得出原因（joberr 的封閉值域）")
	}
	if !rows[0].FinishedAt.Valid {
		t.Error("⛔ 不得停在 running——finishRunStatus 必須走 WithoutCancel 的 writer")
	}
}

// #20f③｜外部 writer 在 startRun **之前**就持鎖。
//
// `startRun` 自己逾時 → 回傳 `runID = 0` → **沒有 job row 可寫**，只能記 log，
// 且核心流程必須就此中止。⚠️ 沒有這條的話，鎖釋放後會寫入一整輪資料
// 卻在 job_runs 上完全看不到。
func TestDelistingReconcileExternalWriterBeforeStartRunAborts(t *testing.T) {
	source := &contentionSourceStub{}
	s, db, path := newSQLiteContentionFixture(t, source)

	// 持鎖 6 秒 > 連線的 busy_timeout（5 秒），所以 startRun 那一句一定等到逾時。
	// ⚠️ 這一句**是**單句寫入，busy handler 有效——與上一支測試的業務交易不同。
	release := holdWriteLock(t, path, 6*time.Second)
	s.RunDelistingReconcile()
	release()

	if got := source.callCount(); got != 0 {
		t.Errorf("⛔ 沒有 job_run id 就不得做任何業務工作，fetch 次數 = %d", got)
	}
	if rows := jobRunRows(t, db); len(rows) != 0 {
		t.Errorf("startRun 失敗時不該有任何 job_run，得到 %+v", rows)
	}
	// 鎖釋放後必須還能正常跑（早退不得留下鎖）。
	if !s.TryStartDelistingReconcile(false) {
		t.Fatal("早退後 ownership 應已釋放")
	}
}

// #20f2｜同一個 pool 的排隊（正式拓撲，`MaxOpenConns(1)`）。
//
// ⛔ **不會拿到 SQLITE_BUSY**：第二個呼叫是在 `database/sql` 層等一條空閒 connection，
// 等到的是 context 逾時。⚠️ 競爭者要模擬成 **ISIN-like 的 DB consumer**，
// ⛔ 不是第二個 T-071 入口——後者會先被 single-flight 擋掉，根本到不了 pool 排隊。
func TestDelistingReconcileQueuesOnSinglePoolConnection(t *testing.T) {
	source := &contentionSourceStub{}
	s, db, _ := newSQLiteContentionFixture(t, source)

	grabbed := make(chan struct{})
	released := make(chan struct{})
	// ①～②：startRun 已經拿到有效 runID（FetchDelistings 保證在它之後），
	// 這時競爭者占用 pool 裡**唯一**那條 connection。
	source.hook = func() {
		go func() {
			defer close(released)
			tx, err := db.Beginx()
			if err != nil {
				close(grabbed)
				return
			}
			var n int
			_ = tx.Get(&n, `SELECT COUNT(*) FROM stock_symbols`)
			close(grabbed)
			time.Sleep(600 * time.Millisecond)
			tx.Rollback()
		}()
		<-grabbed
	}

	// ③業務流程等不到 connection，走 context 逾時。
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	s.runDelistingReconcileOwned(ctx, false)
	<-released // ④競爭者釋放

	// ⑤finishRunStatus 用獨立的 writer timeout 把那筆既有 run 寫成 failed。
	rows := jobRunRows(t, db)
	if len(rows) != 1 {
		t.Fatalf("應只有 startRun 那一筆，得到 %d 筆：%+v", len(rows), rows)
	}
	if rows[0].Status != "failed" || rows[0].SymbolsFailed != 0 {
		t.Fatalf("排隊逾時應寫成 failed ＋ symbols_failed=0，得到 %+v", rows[0])
	}
	if !rows[0].FinishedAt.Valid {
		t.Error("⛔ 不得停在 running")
	}
}

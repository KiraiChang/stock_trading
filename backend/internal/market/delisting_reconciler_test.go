package market

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
	"github.com/trading/backend/internal/store"
)

// ── 測試替身 ──────────────────────────────────────────────────────────────

type stubDelistingSource struct {
	res DelistingSourceResult
	err error
}

func (s *stubDelistingSource) FetchDelistings(context.Context) (DelistingSourceResult, error) {
	return s.res, s.err
}

type stubReadiness struct {
	ok  bool
	err error
}

func (s *stubReadiness) StockSymbolSyncSucceededToday(context.Context, time.Time) (bool, error) {
	return s.ok, s.err
}

func reconcilerTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	tmp, err := os.CreateTemp("", "delisting-reconciler-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	t.Cleanup(func() { os.Remove(tmp.Name()) })

	// ⚠️ 走 NewDB → NewSQLite：FK 是 connection-local，繞過去會讓 RESTRICT 靜默失效。
	db, err := store.NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: tmp.Name()})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return db
}

func rowsOf(rows ...DelistingEventRow) DelistingSourceResult {
	symbols := map[string]struct{}{}
	for _, r := range rows {
		symbols[r.Symbol] = struct{}{}
	}
	return DelistingSourceResult{
		Rows: rows, RowCount: len(rows),
		DistinctSymbols: len(symbols), ContentHash: "hash",
	}
}

func row(symbol, name string, y int, m time.Month, d int) DelistingEventRow {
	return DelistingEventRow{
		Symbol: symbol, CompanyName: name,
		DelistedDate: time.Date(y, m, d, 0, 0, 0, 0, time.UTC),
	}
}

func newReconciler(t *testing.T, db *sqlx.DB, res DelistingSourceResult) *DelistingReconciler {
	t.Helper()
	return NewDelistingReconciler(
		&stubDelistingSource{res: res},
		store.NewDelistingRepo(db),
		&stubReadiness{ok: true},
		zap.NewNop(),
	)
}

// seedMaster 建一筆主檔列。listedDate 為零值代表 NULL。
func seedMaster(t *testing.T, db *sqlx.DB, symbol, name, market, secType string, isListed bool, listedDate time.Time) {
	t.Helper()
	var ld any
	if !listedDate.IsZero() {
		ld = listedDate
	}
	if _, err := db.Exec(db.Rebind(`
		INSERT INTO stock_symbols (symbol, name, market, security_type, listed_date, is_listed)
		VALUES (?, ?, ?, ?, ?, ?)`), symbol, name, market, secType, ld, isListed); err != nil {
		t.Fatalf("建主檔列 %s: %v", symbol, err)
	}
}

func readPair(t *testing.T, db *sqlx.DB, symbol string) (sql.NullTime, sql.NullInt64) {
	t.Helper()
	var date sql.NullTime
	var id sql.NullInt64
	if err := db.Get(&date, db.Rebind(`SELECT delisted_date FROM stock_symbols WHERE symbol = ?`), symbol); err != nil {
		t.Fatalf("查 delisted_date: %v", err)
	}
	if err := db.Get(&id, db.Rebind(`SELECT delisted_event_id FROM stock_symbols WHERE symbol = ?`), symbol); err != nil {
		t.Fatalf("查 delisted_event_id: %v", err)
	}
	return date, id
}

var runNow = time.Date(2026, 9, 7, 7, 0, 0, 0, time.UTC)

// runOnce 建一筆**真的** job_run 再跑一輪。
//
// ⚠️ 刻意不傳假的 runID：delisting_source_snapshots.job_run_id 有 FK 指向 job_runs，
// 傳一個不存在的 id 會被擋下（實測 SQLITE_CONSTRAINT_FOREIGNKEY 787）。
// 用真的 job_run 才走得到正式路徑。
func runOnce(
	t *testing.T, db *sqlx.DB, res DelistingSourceResult, acceptShrink bool, now time.Time,
) (DelistingOutcome, error) {
	t.Helper()
	runID, err := store.NewJobRunRepo(db).Start(context.Background(), "delisting_reconcile")
	if err != nil {
		t.Fatalf("startRun: %v", err)
	}
	return newReconciler(t, db, res).Reconcile(context.Background(), runID, acceptShrink, now)
}

// runOnceWith 讓呼叫端替換 reconciler（例如 readiness stub）。
func runOnceWith(t *testing.T, db *sqlx.DB, rec *DelistingReconciler, acceptShrink bool, now time.Time) (DelistingOutcome, error) {
	t.Helper()
	runID, err := store.NewJobRunRepo(db).Start(context.Background(), "delisting_reconcile")
	if err != nil {
		t.Fatalf("startRun: %v", err)
	}
	return rec.Reconcile(context.Background(), runID, acceptShrink, now)
}

// ── #3：投影規則用四個真實案例當 fixture ──────────────────────────────────

// 這四筆是 2026-09-04 從 TWSE 實抓的名單與 live 主檔的真實組合。
// ⛔ 只有 2867 該被投影；另外三筆各自代表一種必須擋住的情境。
func TestReconcileRealWorldFourCases(t *testing.T) {
	db := reconcilerTestDB(t)

	// live 主檔的真實狀態（2026-09-04 查得）。
	seedMaster(t, db, "2867", "三商壽", "上市", "股票", false, time.Date(2012, 12, 18, 0, 0, 0, 0, time.UTC))
	seedMaster(t, db, "6423", "億而得", "上櫃", "股票", true, time.Date(2026, 1, 22, 0, 0, 0, 0, time.UTC))
	seedMaster(t, db, "2432", "倚天酷碁-創", "上市臺灣創新板", "創新板", true, time.Date(2023, 5, 31, 0, 0, 0, 0, time.UTC))
	seedMaster(t, db, "2301", "光寶科", "上市", "股票", true, time.Date(1995, 11, 17, 0, 0, 0, 0, time.UTC))

	res := rowsOf(
		row("2867", "三商壽", 2026, 9, 1),
		row("6423", "億而得-創", 2026, 1, 22),
		row("2432", "倚天資訊", 2008, 9, 1),
		row("2301", "光寶電子", 2002, 11, 4),
	)
	out, err := runOnce(t, db, res, false, runNow)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// 2867：主檔同名、已下市、終止日晚於上市日 → 投影。
	date, id := readPair(t, db, "2867")
	if !date.Valid || !id.Valid {
		t.Errorf("2867 應被投影，得到 date=%v id=%v", date, id)
	} else if got := date.Time.UTC().Format("2006-01-02"); got != "2026-09-01" {
		t.Errorf("2867 的日期得到 %s，期望 2026-09-01", got)
	}

	// ⛔ 其餘三筆都不得投影——它們正是「直接套用 CSV 會弄壞正在交易的股票」那三檔。
	for _, symbol := range []string{"6423", "2432", "2301"} {
		date, id := readPair(t, db, symbol)
		if date.Valid || id.Valid {
			t.Errorf("%s ⛔ 不得被投影，得到 date=%v id=%v", symbol, date, id)
		}
	}

	if out.Projected != 1 {
		t.Errorf("Projected 應為 1，得到 %d", out.Projected)
	}
	// 三筆都因 is_listed=true 在第 2 步歸 B。
	// ⚠️ 順序有意義：2432 不會再被算進 C，那正是修掉重複計數的地方。
	if out.StillListed != 3 {
		t.Errorf("StillListed 應為 3，得到 %d", out.StillListed)
	}
	if out.GenerationGap != 0 {
		t.Errorf("2432 已在 B 歸類，不得重複計入 C，得到 %d", out.GenerationGap)
	}
	// 方向一每筆恰好一類：總和必須等於事件數。
	assertDirectionOneTotals(t, out, 4)
}

// #5｜mutation 的目標：沒有名稱比對，「2301 假設已下市」就會被回填 2002-11-04。
func TestReconcileNameMismatchBlocksProjection(t *testing.T) {
	db := reconcilerTestDB(t)
	// 假設光寶科將來真的下市——這正是「延遲觸發」那個錯誤的情境。
	seedMaster(t, db, "2301", "光寶科", "上市", "股票", false, time.Date(1995, 11, 17, 0, 0, 0, 0, time.UTC))

	res := rowsOf(row("2301", "光寶電子", 2002, 11, 4))
	out, err := runOnce(t, db, res, false, runNow)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	date, id := readPair(t, db, "2301")
	if date.Valid || id.Valid {
		t.Error("⛔ 名稱不符必須擋住投影——否則光寶科會被回填上一代證券的 2002-11-04")
	}
	if out.IdentityDoubt != 1 {
		t.Errorf("應歸 D 類，得到 IdentityDoubt=%d", out.IdentityDoubt)
	}
}

// #6｜身分歧義 → D；listed_date IS NULL → U。**兩者分類不同**。
func TestReconcileAmbiguityAndNullListedDate(t *testing.T) {
	t.Run("多筆歧義歸 D", func(t *testing.T) {
		db := reconcilerTestDB(t)
		seedMaster(t, db, "9001", "甲公司", "上市", "股票", false, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
		// 同代號、同名、兩個不同日期 → 兩筆都是 candidate → 數量 2 → 歧義。
		res := rowsOf(
			row("9001", "甲公司", 2020, 3, 1),
			row("9001", "甲公司", 2024, 6, 1),
		)
		out, err := runOnce(t, db, res, false, runNow)
		if err != nil {
			t.Fatalf("Reconcile: %v", err)
		}
		if out.IdentityDoubt != 2 {
			t.Errorf("兩筆都應歸 D，得到 %d", out.IdentityDoubt)
		}
		if date, _ := readPair(t, db, "9001"); date.Valid {
			t.Error("歧義時不得寫入主檔")
		}
		assertDirectionOneTotals(t, out, 2)
	})

	t.Run("listed_date IS NULL 歸 U", func(t *testing.T) {
		db := reconcilerTestDB(t)
		seedMaster(t, db, "9002", "乙公司", "上市", "股票", false, time.Time{})
		res := rowsOf(row("9002", "乙公司", 2020, 3, 1))
		out, err := runOnce(t, db, res, false, runNow)
		if err != nil {
			t.Fatalf("Reconcile: %v", err)
		}
		if out.Unresolvable != 1 {
			t.Errorf("應歸 U，得到 Unresolvable=%d IdentityDoubt=%d", out.Unresolvable, out.IdentityDoubt)
		}
		assertDirectionOneTotals(t, out, 1)
	})
}

// #20⓪①｜首次投影（空值 → 建立 ownership）與來源更正（D1 → D2 走 P 替換）。
//
// ⛔ 兩者都不得落到 D：規則 7 若寫成「尚未有值／值相同／本 job 投影的」三個並列條件，
// 第一次 ownership 根本建立不起來、D1 → D2 也永遠換不掉。
func TestReconcileProjectionConverges(t *testing.T) {
	db := reconcilerTestDB(t)
	seedMaster(t, db, "9003", "丙公司", "上市", "股票", false, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))

	// ⓪ 首次投影。
	first := rowsOf(row("9003", "丙公司", 2026, 3, 1))
	if _, err := runOnce(t, db, first, false, runNow); err != nil {
		t.Fatalf("首輪: %v", err)
	}
	date, id := readPair(t, db, "9003")
	if !date.Valid || !id.Valid {
		t.Fatal("首次投影應建立 ownership")
	}
	firstEventID := id.Int64

	// ① 來源把日期更正成 D2 → 舊事件標消失、新事件投影、主檔跟著換。
	second := rowsOf(row("9003", "丙公司", 2026, 3, 15))
	out, err := runOnce(t, db, second, false, runNow.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("次輪: %v", err)
	}
	date, id = readPair(t, db, "9003")
	if got := date.Time.UTC().Format("2006-01-02"); got != "2026-03-15" {
		t.Errorf("來源更正後日期應變成 2026-03-15，得到 %s", got)
	}
	if id.Int64 == firstEventID {
		t.Error("provenance 應指向新事件")
	}
	if out.Projected != 1 {
		t.Errorf("應走 P 替換，得到 Projected=%d IdentityDoubt=%d", out.Projected, out.IdentityDoubt)
	}
	// 舊事件被標記消失 → 算一次來源更正 → 該輪 partial。
	if out.SourceCorrected != 1 {
		t.Errorf("舊事件首次缺席應計 1 次更正，得到 %d", out.SourceCorrected)
	}
	if !out.Degraded() {
		t.Error("有來源更正的那一輪必須是 partial")
	}
}

// #20c｜人工所有權是單向的，永不升級成 job-owned。
func TestReconcileManualOwnershipIsOneWay(t *testing.T) {
	setup := func(t *testing.T, manualDate time.Time, csvRow DelistingEventRow) (*sqlx.DB, DelistingOutcome) {
		db := reconcilerTestDB(t)
		seedMaster(t, db, "9004", "丁公司", "上市", "股票", false, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
		// 人工填入日期：delisted_date 有值但 delisted_event_id 為 NULL。
		if _, err := db.Exec(db.Rebind(
			`UPDATE stock_symbols SET delisted_date = ? WHERE symbol = ?`), manualDate, "9004"); err != nil {
			t.Fatal(err)
		}
		out, err := runOnce(t, db, rowsOf(csvRow), false, runNow)
		if err != nil {
			t.Fatalf("Reconcile: %v", err)
		}
		return db, out
	}

	t.Run("① 人工日期不同 → D，不覆蓋不清除", func(t *testing.T) {
		manual := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		db, out := setup(t, manual, row("9004", "丁公司", 2026, 3, 1))
		if out.IdentityDoubt != 1 {
			t.Errorf("應歸 D，得到 %+v", out)
		}
		date, id := readPair(t, db, "9004")
		if !date.Valid || date.Time.UTC().Format("2006-01-02") != "2025-01-01" {
			t.Errorf("人工值不得被覆蓋，得到 %v", date)
		}
		if id.Valid {
			t.Error("⛔ 不得補上 provenance")
		}
	})

	t.Run("② 人工日期相同 → M，⛔ 仍不得補 provenance", func(t *testing.T) {
		manual := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		db, out := setup(t, manual, row("9004", "丁公司", 2026, 3, 1))
		if out.ManualMatched != 1 {
			t.Errorf("應歸 M，得到 %+v", out)
		}
		_, id := readPair(t, db, "9004")
		if id.Valid {
			t.Error("⛔ 日期相同也不得補 provenance——否則人工值會被轉成 job-owned，" +
				"之後來源一消失就被撤銷規則清掉")
		}
		// M 是 Info，不該讓整輪變 partial。
		if out.Degraded() {
			t.Errorf("只有 M 的一輪應為 success，得到 %+v", out)
		}
	})

	t.Run("③ 承②，來源事件消失後人工值仍在", func(t *testing.T) {
		manual := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		db, _ := setup(t, manual, row("9004", "丁公司", 2026, 3, 1))
		// 下一輪：該事件從來源消失（用別的代號維持筆數，避開縮水防護）。
		seedMaster(t, db, "9005", "戊公司", "上市", "股票", false, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
		next := rowsOf(row("9005", "戊公司", 2026, 4, 1))
		if _, err := runOnce(t, db, next, false, runNow.Add(24*time.Hour)); err != nil {
			t.Fatalf("次輪: %v", err)
		}
		date, id := readPair(t, db, "9004")
		if !date.Valid || date.Time.UTC().Format("2006-01-02") != "2026-03-01" {
			t.Errorf("⛔ 人工值不得被撤銷規則清掉，得到 %v", date)
		}
		if id.Valid {
			t.Error("provenance 仍應為 NULL")
		}
	})

	t.Run("④ 日期相同但名稱不符 → ⛔ 不得判 M", func(t *testing.T) {
		// candidate 數為 0（名稱不符），連 M 的門檻都到不了。
		manual := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		_, out := setup(t, manual, row("9004", "另一家公司", 2026, 3, 1))
		if out.ManualMatched != 0 {
			t.Error("⛔ 名稱不符不得被說成「來源佐證一致」")
		}
		if out.IdentityDoubt != 1 {
			t.Errorf("應歸 D，得到 %+v", out)
		}
	})
}

// #12｜方向二：R 與 A 是同一件事的兩個階段，同一輪不得同時命中。
func TestReconcileRevokeThenSourceMissing(t *testing.T) {
	db := reconcilerTestDB(t)
	seedMaster(t, db, "9006", "己公司", "上市", "股票", false, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	// 陪跑的第二檔，用來維持筆數避開縮水防護。
	seedMaster(t, db, "9007", "庚公司", "上市", "股票", false, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))

	first := rowsOf(row("9006", "己公司", 2026, 3, 1), row("9007", "庚公司", 2026, 3, 2))
	if _, err := runOnce(t, db, first, false, runNow); err != nil {
		t.Fatalf("首輪: %v", err)
	}
	if _, id := readPair(t, db, "9006"); !id.Valid {
		t.Fatal("前置：9006 應已投影成 job-owned")
	}

	// 第 2 輪：9006 的事件從來源消失（換成另一筆維持筆數）。
	second := rowsOf(row("9007", "庚公司", 2026, 3, 2), row("9008", "辛公司", 2026, 3, 3))
	out, err := runOnce(t, db, second, false, runNow.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("次輪: %v", err)
	}
	if out.Revoked != 1 {
		t.Errorf("9006 應被撤銷（R=1），得到 %d", out.Revoked)
	}
	// ⛔ 同一輪不得同時計進 A——R／RX／A 都基於**撤銷前**的同一份 snapshot。
	if out.SourceMissing != 0 {
		t.Errorf("同一輪 A 應為 0（撤銷後下一輪才輪到），得到 %d", out.SourceMissing)
	}
	date, id := readPair(t, db, "9006")
	if date.Valid || id.Valid {
		t.Errorf("撤銷後兩欄都應為 NULL，得到 date=%v id=%v", date, id)
	}

	// 第 3 輪：同樣的輸入 → 這次才輪到 A。
	third := rowsOf(row("9007", "庚公司", 2026, 3, 2), row("9008", "辛公司", 2026, 3, 3))
	out3, err := runOnce(t, db, third, false, runNow.Add(48*time.Hour))
	if err != nil {
		t.Fatalf("第三輪: %v", err)
	}
	if out3.Revoked != 0 {
		t.Errorf("第三輪不該再撤銷，得到 R=%d", out3.Revoked)
	}
	if out3.SourceMissing != 1 {
		t.Errorf("第三輪才輪到 A=1，得到 %d", out3.SourceMissing)
	}
}

// #10h｜一筆早年消失的事件不得讓 job 永久 partial。
func TestReconcileContinuedAbsenceIsNotRecounted(t *testing.T) {
	db := reconcilerTestDB(t)
	// ⚠️ fixture 刻意讓消失的那筆**不在主檔**（歸 N），避免它命中 A／R 而蓋掉本條要驗的東西。
	seedMaster(t, db, "9010", "壬公司", "上市", "股票", false, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))

	first := rowsOf(row("9010", "壬公司", 2026, 3, 1), row("9999", "查無此檔", 2026, 3, 2))
	if _, err := runOnce(t, db, first, false, runNow); err != nil {
		t.Fatalf("首輪: %v", err)
	}

	// 第 2 輪：9999 消失（補一筆維持筆數）→ 首次缺席，計 1 次更正 → partial。
	second := rowsOf(row("9010", "壬公司", 2026, 3, 1), row("9998", "另一檔", 2026, 3, 3))
	out2, err := runOnce(t, db, second, false, runNow.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("次輪: %v", err)
	}
	if out2.SourceCorrected != 1 {
		t.Errorf("首次缺席應計 1，得到 %d", out2.SourceCorrected)
	}
	if !out2.Degraded() {
		t.Error("有來源更正的那一輪應為 partial")
	}

	// 第 3 輪：9999 持續缺席 → ⛔ 不得再計、狀態回 success。
	third := rowsOf(row("9010", "壬公司", 2026, 3, 1), row("9998", "另一檔", 2026, 3, 3))
	out3, err := runOnce(t, db, third, false, runNow.Add(48*time.Hour))
	if err != nil {
		t.Fatalf("第三輪: %v", err)
	}
	if out3.SourceCorrected != 0 {
		t.Errorf("⛔ 持續缺席不得再計入更正，得到 %d —— 否則 job 會永久 partial", out3.SourceCorrected)
	}
	if out3.Degraded() {
		t.Errorf("第三輪應回到 success，得到 %+v", out3)
	}
}

// #10e｜消失 → 重新出現：必須恢復投影資格。
func TestReconcileReappearRestoresProjection(t *testing.T) {
	db := reconcilerTestDB(t)
	seedMaster(t, db, "9020", "癸公司", "上市", "股票", false, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	seedMaster(t, db, "9021", "陪跑", "上市", "股票", false, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))

	full := rowsOf(row("9020", "癸公司", 2026, 3, 1), row("9021", "陪跑", 2026, 3, 2))
	if _, err := runOnce(t, db, full, false, runNow); err != nil {
		t.Fatalf("首輪: %v", err)
	}

	// 消失一輪（補別的維持筆數）。
	gone := rowsOf(row("9021", "陪跑", 2026, 3, 2), row("9022", "他檔", 2026, 3, 3))
	if _, err := runOnce(t, db, gone, false, runNow.Add(24*time.Hour)); err != nil {
		t.Fatalf("次輪: %v", err)
	}
	if date, _ := readPair(t, db, "9020"); date.Valid {
		t.Fatal("事件消失後投影應被撤銷")
	}

	// 重新出現 → 事件的 missing_from_source_at 清回 NULL，重新可投影。
	back := rowsOf(row("9020", "癸公司", 2026, 3, 1), row("9021", "陪跑", 2026, 3, 2), row("9022", "他檔", 2026, 3, 3))
	out, err := runOnce(t, db, back, false, runNow.Add(48*time.Hour))
	if err != nil {
		t.Fatalf("第三輪: %v", err)
	}
	date, id := readPair(t, db, "9020")
	if !date.Valid || !id.Valid {
		t.Error("⛔ 重新出現後必須恢復投影資格——少了清 missing_from_source_at 這一步，" +
			"暫時性缺漏會讓事件永久不可投影而且不報錯")
	}
	if out.SourceCorrected != 1 {
		t.Errorf("重現算一次更正，得到 %d", out.SourceCorrected)
	}
}

// ── 縮水防護與前置條件 ────────────────────────────────────────────────────

func TestReconcileShrinkGuard(t *testing.T) {
	seed := func(t *testing.T) *sqlx.DB {
		db := reconcilerTestDB(t)
		seedMaster(t, db, "9030", "甲", "上市", "股票", false, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
		base := rowsOf(row("9030", "甲", 2026, 3, 1), row("9031", "乙", 2026, 3, 2), row("9032", "丙", 2026, 3, 3))
		if _, err := runOnce(t, db, base, false, runNow); err != nil {
			t.Fatalf("建立基準: %v", err)
		}
		return db
	}

	t.Run("#10 縮水 → 放棄本輪、零寫入", func(t *testing.T) {
		db := seed(t)
		var before int
		if err := db.Get(&before, `SELECT COUNT(*) FROM delisting_events`); err != nil {
			t.Fatal(err)
		}
		shrunk := rowsOf(row("9030", "甲", 2026, 3, 1))
		_, err := runOnce(t, db, shrunk, false, runNow.Add(time.Hour))
		if !errors.Is(err, ErrSourceShrunk) {
			t.Fatalf("應回 ErrSourceShrunk，得到 %v", err)
		}
		var after, snaps int
		if err := db.Get(&after, `SELECT COUNT(*) FROM delisting_events`); err != nil {
			t.Fatal(err)
		}
		if err := db.Get(&snaps, `SELECT COUNT(*) FROM delisting_source_snapshots`); err != nil {
			t.Fatal(err)
		}
		if after != before {
			t.Errorf("縮水時必須零寫入，事件數 %d → %d", before, after)
		}
		if snaps != 1 {
			t.Errorf("縮水時不得新增快照，得到 %d 筆", snaps)
		}
	})

	t.Run("#10b accept_shrink=1 放行縮水", func(t *testing.T) {
		db := seed(t)
		shrunk := rowsOf(row("9030", "甲", 2026, 3, 1))
		if _, err := runOnce(t, db, shrunk, true, runNow.Add(time.Hour)); err != nil {
			t.Fatalf("override 應放行: %v", err)
		}
		var snaps int
		if err := db.Get(&snaps, `SELECT COUNT(*) FROM delisting_source_snapshots`); err != nil {
			t.Fatal(err)
		}
		if snaps != 2 {
			t.Errorf("放行後應新增第二筆快照，得到 %d", snaps)
		}
	})

	t.Run("#10g row_count=0 ⛔ 連 accept_shrink 也擋", func(t *testing.T) {
		db := seed(t)
		// ⛔ 放行 0 的後果是災難性的：所有事件被標記消失，且基準被降成 0，
		// 之後任何筆數都「不低於基準」，防護永久失效。
		empty := DelistingSourceResult{RowCount: 0, ContentHash: "h"}
		_, err := runOnce(t, db, empty, true, runNow.Add(time.Hour))
		if !errors.Is(err, ErrEmptySourceRows) {
			t.Fatalf("零筆一律失敗，得到 %v", err)
		}
		var snaps int
		if err := db.Get(&snaps, `SELECT COUNT(*) FROM delisting_source_snapshots`); err != nil {
			t.Fatal(err)
		}
		if snaps != 1 {
			t.Errorf("零筆時不得新增快照，得到 %d", snaps)
		}
	})

	t.Run("#10c bootstrap：無 accepted 快照時任何非零筆數都通過", func(t *testing.T) {
		db := reconcilerTestDB(t)
		one := rowsOf(row("9040", "首次", 2026, 3, 1))
		if _, err := runOnce(t, db, one, false, runNow); err != nil {
			t.Fatalf("bootstrap 應通過: %v", err)
		}
	})
}

// #8｜當日 stock_symbol_sync 未成功 → 零寫入。
func TestReconcileRequiresStockSymbolSyncToday(t *testing.T) {
	db := reconcilerTestDB(t)
	rec := NewDelistingReconciler(
		&stubDelistingSource{res: rowsOf(row("9050", "甲", 2026, 3, 1))},
		store.NewDelistingRepo(db),
		&stubReadiness{ok: false}, // sync 今天沒成功
		zap.NewNop(),
	)
	_, err := runOnceWith(t, db, rec, false, runNow)
	if !errors.Is(err, ErrStockSymbolSyncNotReady) {
		t.Fatalf("應回 ErrStockSymbolSyncNotReady，得到 %v", err)
	}
	var events, snaps int
	if err := db.Get(&events, `SELECT COUNT(*) FROM delisting_events`); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&snaps, `SELECT COUNT(*) FROM delisting_source_snapshots`); err != nil {
		t.Fatal(err)
	}
	if events != 0 || snaps != 0 {
		t.Errorf("依賴不成立時必須零寫入（連 delisting_events 都不寫），得到 events=%d snaps=%d", events, snaps)
	}
}

// #11｜冪等：同一天跑兩次。
//
// ⛔ **不是「第二次零寫入」**——快照每輪都會新增一筆，那是設計而非缺陷。
func TestReconcileIdempotent(t *testing.T) {
	db := reconcilerTestDB(t)
	seedMaster(t, db, "9060", "甲", "上市", "股票", false, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	res := rowsOf(row("9060", "甲", 2026, 3, 1))

	if _, err := runOnce(t, db, res, false, runNow); err != nil {
		t.Fatalf("首輪: %v", err)
	}
	date1, id1 := readPair(t, db, "9060")

	out, err := runOnce(t, db, res, false, runNow.Add(time.Hour))
	if err != nil {
		t.Fatalf("次輪: %v", err)
	}

	// ① 不新增重複事件
	var events int
	if err := db.Get(&events, `SELECT COUNT(*) FROM delisting_events`); err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Errorf("不得新增重複事件，得到 %d 筆", events)
	}
	// ② 投影結果完全不變
	date2, id2 := readPair(t, db, "9060")
	if !date1.Time.Equal(date2.Time) || id1.Int64 != id2.Int64 {
		t.Errorf("投影結果應完全不變：%v/%v → %v/%v", date1, id1, date2, id2)
	}
	// ③ last_seen_at 正常推進
	var lastSeen time.Time
	if err := db.Get(&lastSeen, `SELECT last_seen_at FROM delisting_events`); err != nil {
		t.Fatal(err)
	}
	if !lastSeen.After(runNow) {
		t.Errorf("last_seen_at 應推進到第二輪，得到 %v", lastSeen)
	}
	// ④ 每個成功 run 各新增一筆 accepted 快照
	var snaps int
	if err := db.Get(&snaps, `SELECT COUNT(*) FROM delisting_source_snapshots`); err != nil {
		t.Fatal(err)
	}
	if snaps != 2 {
		t.Errorf("快照每輪一筆，兩輪應有 2 筆，得到 %d", snaps)
	}
	// 第二輪沒有任何 degradation。
	if out.Degraded() {
		t.Errorf("穩定狀態的第二輪應為 success，得到 %+v", out)
	}
	if out.Projected != 1 {
		t.Errorf("值相同時仍應計 P（走 guarded read），得到 %+v", out)
	}
}

// #12｜方向一每筆恰好一類：計數總和必須等於事件數。
func assertDirectionOneTotals(t *testing.T, out DelistingOutcome, want int) {
	t.Helper()
	sum := out.NoMaster + out.StillListed + out.Unresolvable + out.GenerationGap +
		out.IdentityDoubt + out.ManualMatched + out.Conflict + out.Projected
	if sum != want {
		t.Errorf("方向一每筆恰好一類，總和應為 %d，得到 %d（%+v）", want, sum, out)
	}
}

// 真實 fixture 的端到端：265 筆全部跑一次，計數總和必須對得上。
func TestReconcileRealFixtureEndToEnd(t *testing.T) {
	db := reconcilerTestDB(t)
	seedMaster(t, db, "2867", "三商壽", "上市", "股票", false, time.Date(2012, 12, 18, 0, 0, 0, 0, time.UTC))
	seedMaster(t, db, "6423", "億而得", "上櫃", "股票", true, time.Date(2026, 1, 22, 0, 0, 0, 0, time.UTC))
	seedMaster(t, db, "2432", "倚天酷碁-創", "上市臺灣創新板", "創新板", true, time.Date(2023, 5, 31, 0, 0, 0, 0, time.UTC))
	seedMaster(t, db, "2301", "光寶科", "上市", "股票", true, time.Date(1995, 11, 17, 0, 0, 0, 0, time.UTC))

	parsed, err := ParseSuspendListingCSV(bytes.NewReader(realFixture(t)))
	if err != nil {
		t.Fatalf("解析 fixture: %v", err)
	}
	out, err := runOnce(t, db, parsed, false, runNow)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	assertDirectionOneTotals(t, out, 265)
	if out.NoMaster != 261 {
		t.Errorf("261 筆不在主檔，得到 N=%d", out.NoMaster)
	}
	if out.Projected != 1 {
		t.Errorf("只有 2867 該被投影，得到 P=%d", out.Projected)
	}
	if out.StillListed != 3 {
		t.Errorf("三筆仍在交易，得到 B=%d", out.StillListed)
	}
	if out.SourceMissing != 0 {
		t.Errorf("2867 有對應事件，A 應為 0，得到 %d", out.SourceMissing)
	}
	if out.Degraded() {
		t.Errorf("這一輪應為 success，得到 %s", out.Summary())
	}
}

// ── #3b／#3c：名稱正規化 ───────────────────────────────────────────────────

// #3c｜正向：四個合法 suffix 各一個，移除後與無後綴版本相等。
func TestNormalizeCompanyNameRemovesAllowedSuffixes(t *testing.T) {
	cases := []struct{ in, want string }{
		{"億而得-創", "億而得"},
		{"聖馬丁-DR", "聖馬丁"},
		{"某某-KY", "某某"},
		{"新創-創櫃", "新創"},
	}
	for _, c := range cases {
		if got := NormalizeCompanyName(c.in); got != c.want {
			t.Errorf("NormalizeCompanyName(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
	// 全半形與空白統一。
	if NormalizeCompanyName("台 積 電") != NormalizeCompanyName("台積電") {
		t.Error("空白應被去除")
	}
	if NormalizeCompanyName("ＡＢＣ") != NormalizeCompanyName("ABC") {
		t.Error("全形英數應摺疊成半形")
	}
}

// #3b｜負向：兩個原本不同的公司名稱，正規化後**必須仍然不同**。
//
// ⛔ **①②③ 是關鍵**——只有無連字號的案例時，實作錯成「從第一個 `-` 截斷」
// 或「移除 `-` 之後的所有文字」仍會全過。名稱是唯一能辨識代號重用的訊號，
// 過度正規化等於拆掉最後一道防線。
func TestNormalizeCompanyNameDoesNotOverNormalize(t *testing.T) {
	cases := []struct {
		a, b string
		note string
	}{
		{"甲-未知", "甲", "① 不在 allowlist 的後綴⛔不得移除"},
		{"甲-DR科技", "甲科技", "② `-DR` 不在尾端就不移除"},
		{"台光電", "台光電子材料", "④ 無連字號的不同名稱"},
		{"中華", "中華電", "④ 無連字號的不同名稱"},
		{"光寶科", "光寶電子", "真實案例：2301 的兩代公司"},
		{"倚天酷碁-創", "倚天資訊", "真實案例：2432 的兩代公司"},
	}
	for _, c := range cases {
		if NormalizeCompanyName(c.a) == NormalizeCompanyName(c.b) {
			t.Errorf("%s：%q 與 %q 正規化後不得相等（都得到 %q）",
				c.note, c.a, c.b, NormalizeCompanyName(c.a))
		}
	}

	// ③ 連續 suffix **最多移除一次**。
	//
	// ⚠️ 這裡用**同一個** suffix 連兩次（`甲-創-創`）而不是兩個不同的（`甲-創-DR`）：
	// 後者碰巧會被 allowlist 的排列順序救掉——移除 `-DR` 之後外層迴圈已經走過 `-創`，
	// 不會再回頭，於是「移除一次」與「移除到不能移除為止」得到相同答案，測不出差別。
	// 實測過：用 `甲-創-DR` 時「改成 for 迴圈」那個 mutation 不會變紅。
	if got := NormalizeCompanyName("甲-創-創"); got != "甲-創" {
		t.Errorf("③ 連續 suffix 只移除一次：得到 %q，期望 %q", got, "甲-創")
	}
}

// #12b｜狀態推導的八種情形。
//
// ⚠️ **M（人工值與來源一致）是 Info 不是 degradation**——「人工填的日期與官方相符」
// 是好事，不該讓整輪變 partial。
// ⛔ **來源更正要算**：否則「日期被更正、新事件成功投影為 P」會出 Warn 卻收成 success，
// 排程頁上完全看不出來源動過。
func TestDelistingOutcomeDegradedCoversEightCases(t *testing.T) {
	cases := []struct {
		name string
		out  DelistingOutcome
		want bool
	}{
		{"①只有 N／B／C／P", DelistingOutcome{
			NoMaster: 3, StillListed: 2, GenerationGap: 1, Projected: 4}, false},
		{"②只有 M，無其他 degradation", DelistingOutcome{ManualMatched: 2}, false},
		{"③A／U／D 任一 > 0（U）", DelistingOutcome{Unresolvable: 1}, true},
		{"③A／U／D 任一 > 0（D）", DelistingOutcome{IdentityDoubt: 1}, true},
		{"③A／U／D 任一 > 0（A）", DelistingOutcome{SourceMissing: 1}, true},
		{"④只有 R", DelistingOutcome{Revoked: 1}, true},
		{"⑤R 與來源更正同時", DelistingOutcome{Revoked: 1, SourceCorrected: 1}, true},
		{"⑥沒有 A／U／D／R 但有來源更正", DelistingOutcome{Projected: 1, SourceCorrected: 1}, true},
		{"⑦只有 X", DelistingOutcome{Conflict: 1}, true},
		{"⑧只有 RX", DelistingOutcome{RevocationConflict: 1}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.out.Degraded(); got != tc.want {
				t.Errorf("Degraded() = %v, want %v（%+v）", got, tc.want, tc.out)
			}
		})
	}
}

// ⑤的補充：`projection_revoked` 與 `source_corrected` **各自只計自己的單位**，
// ⛔ 不得互相灌數——摘要是排程頁上唯一看得到成因的東西。
func TestDelistingOutcomeSummaryKeepsCountersSeparate(t *testing.T) {
	got := DelistingOutcome{Revoked: 1, SourceCorrected: 2, IdentityDoubt: 3}.Summary()

	for _, want := range []string{
		"projection_revoked=1", "source_corrected=2", "identity_doubt=3",
		"source_missing=0", "concurrency_conflict=0", "revocation_conflict=0",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("摘要缺 %q：%s", want, got)
		}
	}
}

// ── review 修正的回歸測試（2026-09-08）─────────────────────────────────────

// failingProjectionRepo 把真實的 repo 包起來，只讓指定的寫入方法回 DB error。
//
// ⚠️ **不能用「callback 主動回錯」代替**：那條路徑早就有 rollback 測試，
// 但它證明不了「repo 的單句寫入失敗時整輪會中止」——那正是原本被吞掉的路徑。
type failingProjectionRepo struct {
	inner store.DelistingRepo
	fail  string // project / confirm / revoke
	err   error
}

func (r failingProjectionRepo) WithTx(ctx context.Context, fn func(store.DelistingTx) error) error {
	return r.inner.WithTx(ctx, func(tx store.DelistingTx) error {
		return fn(failingProjectionTx{DelistingTx: tx, fail: r.fail, err: r.err})
	})
}

type failingProjectionTx struct {
	store.DelistingTx
	fail string
	err  error
}

func (t failingProjectionTx) Project(
	prior store.SymbolProjectionState, date time.Time, eventID uint64, now time.Time,
) (bool, error) {
	if t.fail == "project" {
		return false, t.err
	}
	return t.DelistingTx.Project(prior, date, eventID, now)
}

func (t failingProjectionTx) ConfirmProjection(prior store.SymbolProjectionState) (bool, error) {
	if t.fail == "confirm" {
		return false, t.err
	}
	return t.DelistingTx.ConfirmProjection(prior)
}

func (t failingProjectionTx) Revoke(prior store.SymbolProjectionState, now time.Time) (bool, error) {
	if t.fail == "revoke" {
		return false, t.err
	}
	return t.DelistingTx.Revoke(prior, now)
}

// ⛔ **投影／撤銷的 DB error 不是 CAS 落空**：不能記成 X／RX 然後繼續寫 accepted 快照。
//
// MySQL／SQLite 的單句錯誤**不會**中止整個 transaction，所以吞掉它就會 commit
// 「事件寫了、快照寫了、投影沒寫」的半套狀態，違反 all-or-nothing 契約。
func TestReconcileAbortsWhenProjectionWriteFails(t *testing.T) {
	dbErr := errors.New("disk I/O error")

	t.Run("project", func(t *testing.T) {
		db := reconcilerTestDB(t)
		seedMaster(t, db, "T1", "甲公司", "上市", "股票", false, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
		res := rowsOf(row("T1", "甲公司", 2026, time.September, 1))
		rec := NewDelistingReconciler(&stubDelistingSource{res: res},
			failingProjectionRepo{inner: store.NewDelistingRepo(db), fail: "project", err: dbErr},
			&stubReadiness{ok: true}, zap.NewNop())

		// ⚠️ 用 runOnceWith 而不是自己傳 runID：快照的 job_run_id 有 FK。
		_, err := runOnceWith(t, db, rec, false, time.Now())

		if !errors.Is(err, dbErr) {
			t.Fatalf("投影寫入失敗必須讓整輪失敗並保留原因，得到 %v", err)
		}
		assertNothingCommitted(t, db)
	})

	// confirm 走的是「值相同 → guarded read」那條，與 project 是不同的 SQL 路徑，
	// 所以要各驗一次（failingProjectionTx 支援注入這個分支）。
	t.Run("confirm", func(t *testing.T) {
		db := reconcilerTestDB(t)
		seedMaster(t, db, "T1", "甲公司", "上市", "股票", false,
			time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
		res := rowsOf(row("T1", "甲公司", 2026, time.September, 1))
		// 先正常投影一輪，下一輪同樣的來源就會走 ConfirmProjection。
		if _, err := runOnce(t, db, res, false, time.Now()); err != nil {
			t.Fatalf("前置投影: %v", err)
		}

		rec := NewDelistingReconciler(&stubDelistingSource{res: res},
			failingProjectionRepo{inner: store.NewDelistingRepo(db), fail: "confirm", err: dbErr},
			&stubReadiness{ok: true}, zap.NewNop())

		_, err := runOnceWith(t, db, rec, false, time.Now())

		if !errors.Is(err, dbErr) {
			t.Fatalf("confirm 失敗必須讓整輪失敗，得到 %v", err)
		}
		// 第一輪的投影與快照必須原封不動。
		var eventID sql.NullInt64
		if err := db.Get(&eventID, db.Rebind(
			`SELECT delisted_event_id FROM stock_symbols WHERE symbol = ?`), "T1"); err != nil {
			t.Fatal(err)
		}
		if !eventID.Valid {
			t.Error("失敗的那一輪不得動到既有投影")
		}
		var snaps int
		if err := db.Get(&snaps, `SELECT COUNT(*) FROM delisting_source_snapshots`); err != nil {
			t.Fatal(err)
		}
		if snaps != 1 {
			t.Errorf("只有第一輪那筆快照該存在，得到 %d 筆", snaps)
		}
	})

	t.Run("revoke", func(t *testing.T) {
		db := reconcilerTestDB(t)
		seedMaster(t, db, "T1", "甲公司", "上市", "股票", false, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
		// 先讓一輪正常投影，下一輪才有 job-owned 的列可以撤銷。
		first := rowsOf(row("T1", "甲公司", 2026, time.September, 1))
		if _, err := runOnce(t, db, first, false, time.Now()); err != nil {
			t.Fatalf("前置投影: %v", err)
		}

		// 第二輪：來源改成別的代號（原事件消失）→ 走撤銷，而撤銷會失敗。
		second := rowsOf(row("T9", "別家公司", 2026, time.September, 2))
		rec := NewDelistingReconciler(&stubDelistingSource{res: second},
			failingProjectionRepo{inner: store.NewDelistingRepo(db), fail: "revoke", err: dbErr},
			&stubReadiness{ok: true}, zap.NewNop())

		_, err := runOnceWith(t, db, rec, false, time.Now())

		if !errors.Is(err, dbErr) {
			t.Fatalf("撤銷失敗必須讓整輪失敗，得到 %v", err)
		}
		// 第一輪的投影必須原封不動（第二輪整個 rollback）。
		var eventID sql.NullInt64
		if err := db.Get(&eventID, db.Rebind(
			`SELECT delisted_event_id FROM stock_symbols WHERE symbol = ?`), "T1"); err != nil {
			t.Fatal(err)
		}
		if !eventID.Valid {
			t.Error("失敗的那一輪不得留下半套狀態：第一輪的投影被清掉了")
		}
		var snaps int
		if err := db.Get(&snaps, `SELECT COUNT(*) FROM delisting_source_snapshots`); err != nil {
			t.Fatal(err)
		}
		if snaps != 1 {
			t.Errorf("只有第一輪那筆快照該存在，得到 %d 筆", snaps)
		}
	})
}

// assertNothingCommitted 驗三種寫入都沒有留下痕跡。
func assertNothingCommitted(t *testing.T, db *sqlx.DB) {
	t.Helper()
	var events, snaps int
	if err := db.Get(&events, `SELECT COUNT(*) FROM delisting_events`); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&snaps, `SELECT COUNT(*) FROM delisting_source_snapshots`); err != nil {
		t.Fatal(err)
	}
	if events != 0 || snaps != 0 {
		t.Errorf("整輪失敗必須 rollback，得到 events=%d snapshots=%d", events, snaps)
	}
	var projected int
	if err := db.Get(&projected,
		`SELECT COUNT(*) FROM stock_symbols WHERE delisted_date IS NOT NULL`); err != nil {
		t.Fatal(err)
	}
	if projected != 0 {
		t.Errorf("整輪失敗不得留下投影，得到 %d 列", projected)
	}
}

// ── 逐項 Warn（計畫書分類表的「等級」欄）──────────────────────────────────

// ⚠️ **只有 aggregate 數字不夠**：U／D 是「等人工」的類別，而 upsert 之後舊公司名稱、
// 重現前的缺席時間就消失了。看到 `identity_doubt=2` 的人必須查得到是哪幾檔、差在哪。
func TestReconcileLogsPerItemWarnings(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	db := reconcilerTestDB(t)

	// D：名稱不符（代號重用的典型案例）。
	seedMaster(t, db, "2301", "光寶科", "上市", "股票", false,
		time.Date(1995, 11, 17, 0, 0, 0, 0, time.UTC))
	// U：listed_date 為 NULL。
	seedMaster(t, db, "T2", "缺上市日公司", "上市", "股票", false, time.Time{})

	first := rowsOf(
		row("2301", "光寶電子", 2002, time.November, 4),
		row("T2", "缺上市日公司", 2026, time.September, 1),
	)
	rec := NewDelistingReconciler(&stubDelistingSource{res: first},
		store.NewDelistingRepo(db), &stubReadiness{ok: true}, zap.New(core))
	runID, err := store.NewJobRunRepo(db).Start(context.Background(), "delisting_reconcile")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rec.Reconcile(context.Background(), runID, false, time.Now()); err != nil {
		t.Fatalf("第一輪: %v", err)
	}

	assertWarnWith(t, logs, "名稱不符", "symbol", "2301", "master_name", "光寶科", "source_name", "光寶電子")
	assertWarnWith(t, logs, "缺身分依據", "symbol", "T2")

	// 第二輪：2301 改名（來源更正）＋ T2 從來源消失（首次缺席）。
	logs.TakeAll()
	second := rowsOf(row("2301", "光寶電子工業", 2002, time.November, 4))
	rec2 := NewDelistingReconciler(&stubDelistingSource{res: second},
		store.NewDelistingRepo(db), &stubReadiness{ok: true}, zap.New(core))
	runID2, err := store.NewJobRunRepo(db).Start(context.Background(), "delisting_reconcile")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rec2.Reconcile(context.Background(), runID2, true, time.Now()); err != nil {
		t.Fatalf("第二輪: %v", err)
	}

	assertWarnWith(t, logs, "公司名稱已變更", "symbol", "2301", "old_name", "光寶電子", "new_name", "光寶電子工業")
	assertWarnWith(t, logs, "已從官方名單消失", "symbol", "T2")

	// 第三輪：T2 持續缺席 → ⛔ 不得再 Warn 一次（否則會每天重複告警）。
	logs.TakeAll()
	rec3 := NewDelistingReconciler(&stubDelistingSource{res: second},
		store.NewDelistingRepo(db), &stubReadiness{ok: true}, zap.New(core))
	runID3, err := store.NewJobRunRepo(db).Start(context.Background(), "delisting_reconcile")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rec3.Reconcile(context.Background(), runID3, false, time.Now()); err != nil {
		t.Fatalf("第三輪: %v", err)
	}
	for _, entry := range logs.All() {
		if strings.Contains(entry.Message, "已從官方名單消失") {
			t.Error("⛔ 持續缺席不得重複 Warn（邊緣觸發，見 #10h）")
		}
	}

	// 第四輪：T2 重新出現（#10e／#10i）→ 必須 Warn，且帶得出「上次是什麼時候消失的」。
	// ⚠️ 這個分支要**真的跑到**才算驗過——只看實作正確不算。
	logs.TakeAll()
	fourth := rowsOf(
		row("2301", "光寶電子工業", 2002, time.November, 4),
		row("T2", "缺上市日公司", 2026, time.September, 1),
	)
	rec4 := NewDelistingReconciler(&stubDelistingSource{res: fourth},
		store.NewDelistingRepo(db), &stubReadiness{ok: true}, zap.New(core))
	runID4, err := store.NewJobRunRepo(db).Start(context.Background(), "delisting_reconcile")
	if err != nil {
		t.Fatal(err)
	}
	out, err := rec4.Reconcile(context.Background(), runID4, false, time.Now())
	if err != nil {
		t.Fatalf("第四輪: %v", err)
	}

	assertWarnWith(t, logs, "重新出現", "symbol", "T2")
	// 重現算一次來源更正（該輪 partial），⛔ 不是 0。
	if out.SourceCorrected != 1 {
		t.Errorf("重現應計一次來源更正，得到 %d", out.SourceCorrected)
	}
	// missing_from_source_at 必須清回 NULL，事件才恢復投影資格。
	var missing sql.NullTime
	if err := db.Get(&missing, db.Rebind(
		`SELECT missing_from_source_at FROM delisting_events WHERE symbol = ?`), "T2"); err != nil {
		t.Fatal(err)
	}
	if missing.Valid {
		t.Errorf("重現後標記必須清回 NULL，得到 %v", missing.Time)
	}
}

// assertWarnWith 找一筆訊息含 substr 且欄位符合的 Warn。
func assertWarnWith(t *testing.T, logs *observer.ObservedLogs, substr string, kv ...string) {
	t.Helper()
	for _, entry := range logs.All() {
		if !strings.Contains(entry.Message, substr) {
			continue
		}
		if entry.Level != zapcore.WarnLevel {
			continue
		}
		fields := entry.ContextMap()
		matched := true
		for i := 0; i+1 < len(kv); i += 2 {
			if got, ok := fields[kv[i]]; !ok || got != kv[i+1] {
				matched = false
				break
			}
		}
		if matched {
			return
		}
	}
	t.Errorf("找不到含 %q 且欄位 %v 的 Warn；實際 log：%v", substr, kv, logs.All())
}

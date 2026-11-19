package database_test

import (
	"context"
	_ "embed"
	"fmt"
	"testing"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/database"
)

// backtestTradesCheckSQL 是 scripts/smoke-dev-backtest-claim.sh 的 trades 情境執行的同一份查詢（I-135）：
// 參數 $1＝job_id、$2＝前綴，回傳不符合規格的列數。這裡在 PostgreSQL 上真的執行它，驗它會不會漏判。
//
//go:embed testdata/backtest_trades_check.sql
var backtestTradesCheckSQL string

// 一筆符合規格的列：BUY 10 股，100 → 110，毛損益 100 = pnl 98 + commission 2；時間是台北的日 K 時刻。
func goodTradeRow() map[string]any {
	return map[string]any{
		"symbol": "2330", "direction": "BUY",
		"entry_time": "2025-01-02T00:00:00+08:00", "exit_time": "2025-01-10T00:00:00+08:00",
		"entry_price": "100", "exit_price": "110", "size": "10",
		"pnl": "98", "pnl_pct": "0.098", "commission": "2",
	}
}

var tradeCheckCols = []string{"symbol", "direction", "entry_time", "exit_time",
	"entry_price", "exit_price", "size", "pnl", "pnl_pct", "commission"}

// TestPostgresMigrationsBacktestTradesCheck：共用查詢對每一種壞列都要算 1，對好的列算 0。
//
// 用暫存表 `backtest_trades`（`LIKE public.backtest_trades`，⛔ 保留欄位型別），放在同一條連線上：
// pg_temp 在 search_path 最前面，查詢不用改就讀到暫存表。正式表的 symbol、direction 有 NOT NULL，
// 寫不進 NULL，所以暫存表拿掉這兩個約束，NULL 的負向案例才真的進得了查詢。⛔ 不動正式表。
func TestPostgresMigrationsBacktestTradesCheck(t *testing.T) {
	dsn := skipUnlessDSN(t, "POSTGRES_MIGRATION_DSN", "scripts/test-postgres-migrations.sh")
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatalf("連不上 postgres: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	if err := database.RunMigrations(ctx, db, "postgres", zap.NewNop()); err != nil {
		t.Fatalf("migration 失敗: %v", err)
	}
	conn, err := db.Connx(ctx)
	if err != nil {
		t.Fatalf("取得連線: %v", err)
	}
	defer conn.Close()

	for _, stmt := range []string{
		`CREATE TEMP TABLE backtest_trades (LIKE public.backtest_trades INCLUDING DEFAULTS)`,
		`ALTER TABLE pg_temp.backtest_trades ALTER COLUMN symbol DROP NOT NULL, ALTER COLUMN direction DROP NOT NULL`,
	} {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("建暫存表（%s）: %v", stmt, err)
		}
	}

	const prefix = "i135chk_"
	seq := 0
	insert := func(jobID string, row map[string]any) {
		t.Helper()
		args := []any{jobID}
		for _, c := range tradeCheckCols {
			args = append(args, row[c])
		}
		_, err := conn.ExecContext(ctx, `INSERT INTO backtest_trades
			(job_id, symbol, direction, entry_time, exit_time, entry_price, exit_price, size, pnl, pnl_pct, commission)
			VALUES ($1, $2, $3, CAST($4 AS timestamptz), CAST($5 AS timestamptz),
			        CAST($6 AS numeric), CAST($7 AS numeric), CAST($8 AS numeric),
			        CAST($9 AS numeric), CAST($10 AS numeric), CAST($11 AS numeric))`, args...)
		if err != nil {
			t.Fatalf("寫入 %s: %v", jobID, err)
		}
	}
	check := func(jobID, pfx string) int {
		t.Helper()
		var n int
		if err := conn.GetContext(ctx, &n, backtestTradesCheckSQL, jobID, pfx); err != nil {
			t.Fatalf("執行共用查詢（%s）: %v", jobID, err)
		}
		return n
	}
	expect := func(name string, row map[string]any, want int) {
		t.Helper()
		seq++
		id := fmt.Sprintf("%s%02d", prefix, seq)
		insert(id, row)
		if got := check(id, prefix); got != want {
			t.Errorf("%s：不符合規格的列數 = %d，預期 %d（row=%v）", name, got, want, row)
		}
	}
	with := func(overrides map[string]any) map[string]any {
		row := goodTradeRow()
		for k, v := range overrides {
			row[k] = v
		}
		return row
	}

	expect("好的多方列", goodTradeRow(), 0)
	expect("好的空方列", with(map[string]any{"direction": "SELL", "entry_price": "110", "exit_price": "100"}), 0)
	// 界線 0.01×size＋0.01＝0.11：差 0.10 通過、差 0.12 判壞
	expect("差 0.10（界線內）", with(map[string]any{"pnl": "97.90"}), 0)
	expect("差 0.12（超出界線）", with(map[string]any{"pnl": "97.88"}), 1)

	for _, c := range tradeCheckCols {
		expect(c+" 是 NULL", with(map[string]any{c: nil}), 1)
	}
	for _, c := range []string{"entry_price", "exit_price", "size", "pnl", "pnl_pct", "commission"} {
		expect(c+" 是 NaN", with(map[string]any{c: "NaN"}), 1)
	}
	expect("entry_time 是 infinity", with(map[string]any{"entry_time": "infinity"}), 1)
	expect("exit_time 是 -infinity", with(map[string]any{"exit_time": "-infinity"}), 1)
	expect("direction 不是 BUY／SELL", with(map[string]any{"direction": "HOLD"}), 1)
	expect("size 是 0", with(map[string]any{"size": "0"}), 1)
	expect("entry_price 是 0", with(map[string]any{"entry_price": "0"}), 1)
	expect("exit_price 是負數", with(map[string]any{"exit_price": "-1"}), 1)
	expect("方向反了（SELL 卻用多方的價格）", with(map[string]any{"direction": "SELL"}), 1)
	expect("時間不是台北的日 K 時刻", with(map[string]any{"entry_time": "2025-01-02T00:00:00Z"}), 1)

	// 前綴不符的壞列⛔ 不算（只看這一輪的任務）
	insert("other_01", with(map[string]any{"size": nil}))
	if got := check("other_01", prefix); got != 0 {
		t.Errorf("前綴不符的列被算進來：%d", got)
	}
}

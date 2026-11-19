package database_test

import (
	_ "embed"
	"testing"
)

// backtestTradesFeeCheckSQL 是 scripts/smoke-dev-backtest-claim.sh 的 trades 情境執行的同一份費用查詢（I-136）：
// 參數 $1＝job_id、$2＝前綴、$3＝手續費率、$4＝稅率，回傳 commission 對不上模擬費用的列數。
//
//go:embed testdata/backtest_trades_fee_check.sql
var backtestTradesFeeCheckSQL string

// TestPostgresMigrationsBacktestTradesFeeCheck：費用查詢在 PostgreSQL 上真的執行。暫存表與寫入方式同 I-135 的 tradesProbe。
//
// 費率 c＝0.001425、t＝0.003，界線 0.005×size×(2c+t)＋0.005（size 10 時約 0.00529）：
//   - 多單 10 股 100 → 110：10×(100×c＋110×(c＋t))＝6.2925，存兩位是 6.29（差 0.0025，界線內）；6.30 差 0.0075（界線外）。
//   - 空單 10 股 100 → 110：10×(100×(c＋t)＋110×c)＝5.9925，存 5.99；6.00 界線外；用多單公式的 6.29（稅算錯邊）差 0.2975。
func TestPostgresMigrationsBacktestTradesFeeCheck(t *testing.T) {
	p := newTradesProbe(t)
	q := backtestTradesFeeCheckSQL
	const c, tax = "0.001425", "0.003"
	expect := func(name string, row map[string]any, want int) { p.expect(name, q, row, want, probePrefix, c, tax) }
	long := func(o map[string]any) map[string]any {
		row := withRow(map[string]any{"commission": "6.29"})
		for k, v := range o {
			row[k] = v
		}
		return row
	}
	short := func(o map[string]any) map[string]any {
		row := withRow(map[string]any{"direction": "SELL", "entry_price": "100", "exit_price": "110", "commission": "5.99"})
		for k, v := range o {
			row[k] = v
		}
		return row
	}

	expect("多單符合預期值", long(nil), 0)
	expect("多單超出界線（6.30）", long(map[string]any{"commission": "6.30"}), 1)
	expect("空單符合預期值", short(nil), 0)
	expect("空單超出界線（6.00）", short(map[string]any{"commission": "6.00"}), 1)
	expect("空單的稅算錯邊（用多單公式）", short(map[string]any{"commission": "6.29"}), 1)
	expect("費用沒有含稅（只有手續費 2.99）", long(map[string]any{"commission": "2.99"}), 1)

	for _, col := range []string{"commission", "size", "entry_price", "exit_price", "direction"} {
		expect(col+" 是 NULL", long(map[string]any{col: nil}), 1)
	}
	expect("commission 是 NaN", long(map[string]any{"commission": "NaN"}), 1)
	expect("size 是 NaN", long(map[string]any{"size": "NaN"}), 1)
	expect("direction 不是 BUY／SELL", long(map[string]any{"direction": "HOLD"}), 1)

	// 費率是 NULL：預期值算不出來，好的列也要判壞（⛔ 不能靜默通過）
	id := probePrefix + "nullrate"
	p.insert(id, long(nil))
	if got := p.count(q, id, probePrefix, nil, tax); got != 1 {
		t.Errorf("費率是 NULL 時應判壞，實際 %d", got)
	}
	// 前綴不符的壞列⛔ 不算
	p.insert("other_fee", long(map[string]any{"commission": "99"}))
	if got := p.count(q, "other_fee", probePrefix, c, tax); got != 0 {
		t.Errorf("前綴不符的列被算進來：%d", got)
	}
}

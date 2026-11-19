-- backtrader 路徑寫進 backtest_trades 的明細，有幾筆不符合規格（I-135；規格見 docs/database-schema.md 的 backtest_trades）。
-- 參數：$1 = job_id，$2 = 這一輪的前綴（只看前綴相符的任務）。
-- 兩個地方執行同一份查詢：scripts/smoke-dev-backtest-claim.sh 的 trades 情境（psql 用 PREPARE／EXECUTE 綁參數），
-- 以及 backtest_trades_check_postgres_test.go（Go 綁參數）。⛔ 結尾不加分號：psql 端會把它包進 PREPARE。
--
-- 寫法：把「好的列」的條件全部 AND 起來，再用 IS NOT TRUE 判壞。NULL 參與比較得到 UNKNOWN，
-- 直接寫「壞的條件」會讓那一列被 WHERE 濾掉、判成通過；IS NOT TRUE 把 UNKNOWN 也算成壞的。
-- 必要欄位仍逐一寫出 IS NOT NULL，讓規格看得到。DECIMAL 欄位可以存 NaN，而且 PostgreSQL 的 NaN 等於 NaN、
-- 大於任何數，界線比較擋不住所有欄位，所以逐欄排除 NaN；時間另外要求 isfinite（排除 infinity）。
SELECT count(*) FROM backtest_trades t
WHERE starts_with(t.job_id, $2) AND t.job_id = $1 AND (
      t.symbol IS NOT NULL AND t.direction IS NOT NULL
  AND t.entry_time IS NOT NULL AND t.exit_time IS NOT NULL
  AND t.entry_price IS NOT NULL AND t.exit_price IS NOT NULL AND t.size IS NOT NULL
  AND t.pnl IS NOT NULL AND t.pnl_pct IS NOT NULL AND t.commission IS NOT NULL
  AND t.entry_price <> 'NaN' AND t.exit_price <> 'NaN' AND t.size <> 'NaN'
  AND t.pnl <> 'NaN' AND t.pnl_pct <> 'NaN' AND t.commission <> 'NaN'
  AND isfinite(t.entry_time) AND isfinite(t.exit_time)
  AND t.direction IN ('BUY', 'SELL')
  AND t.size > 0 AND t.entry_price > 0 AND t.exit_price > 0
  AND abs((CASE t.direction WHEN 'BUY' THEN 1 ELSE -1 END) * (t.exit_price - t.entry_price) * t.size
          - (t.pnl + t.commission)) <= 0.01 * t.size + 0.01
  AND to_char(t.entry_time AT TIME ZONE 'Asia/Taipei', 'HH24:MI:SS') = '00:00:00'
  AND to_char(t.exit_time AT TIME ZONE 'Asia/Taipei', 'HH24:MI:SS') = '00:00:00'
) IS NOT TRUE

-- backtrader 路徑寫進 backtest_trades 的 commission，有幾筆跟「config 的原始費率算出來的模擬費用」對不上（I-136；規格見
-- docs/database-schema.md 的 backtest_trades）。參數：$1 = job_id、$2 = 這一輪的前綴、$3 = 手續費率、$4 = 交易稅率（比例值）。
-- 兩個地方執行同一份查詢：scripts/smoke-dev-backtest-claim.sh 的 trades 情境（psql 用 PREPARE／EXECUTE 綁參數），
-- 以及 backtest_trades_fee_check_postgres_test.go（Go 綁參數）。⛔ 結尾不加分號：psql 端會把它包進 PREPARE。
--
-- 預期值（c＝手續費率、t＝稅率）：多單 size × (entry_price × c + exit_price × (c + t))，空單 size × (entry_price × (c + t) + exit_price × c)
-- ——買進只收手續費、賣出另收交易稅；空單的賣出在進場。界線 0.005 × size × (2c + t) + 0.005：兩個價格各自四捨五入最多差 0.005、
-- commission 本身最多差 0.005。寫法同 backtest_trades_check.sql：好的條件 AND 起來、IS NOT TRUE 判壞（NULL 也算壞）、排除 NaN。
SELECT count(*) FROM backtest_trades t
WHERE starts_with(t.job_id, $2) AND t.job_id = $1 AND (
      t.direction IS NOT NULL AND t.size IS NOT NULL
  AND t.entry_price IS NOT NULL AND t.exit_price IS NOT NULL AND t.commission IS NOT NULL
  AND t.size <> 'NaN' AND t.entry_price <> 'NaN' AND t.exit_price <> 'NaN' AND t.commission <> 'NaN'
  AND t.direction IN ('BUY', 'SELL')
  AND abs(t.commission - t.size * (CASE t.direction
        WHEN 'BUY' THEN t.entry_price * CAST($3 AS numeric) + t.exit_price * (CAST($3 AS numeric) + CAST($4 AS numeric))
        ELSE t.entry_price * (CAST($3 AS numeric) + CAST($4 AS numeric)) + t.exit_price * CAST($3 AS numeric) END))
      <= 0.005 * t.size * (2 * CAST($3 AS numeric) + CAST($4 AS numeric)) + 0.005
) IS NOT TRUE

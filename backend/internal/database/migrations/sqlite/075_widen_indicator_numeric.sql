-- no-op：sqlite 對應欄位是 REAL，沒有精度概念，本來就容得下 rsi14 = 100
-- 與任意大的 vol_ratio。完整設計理由見 postgres 版的註解。
--
-- **這個檔案存在的理由是編號對齊**：三份 migration 的版本號必須一致，
-- 少一份會讓 sqlite 停在 74 而 postgres 在 75，之後每一支都要多想一次對照關係。
-- （body 只有註解的 goose migration 是既有慣例，例如 009_add_user_status.sql 的 Down。）
--
-- ⚠️ 順帶記一件事：sqlite 用 REAL 正是這個 bug 逃過單元測試的原因——
-- backend/scripts/test.sh 跑的是 sqlite，**這一類溢位在它底下永遠不會出現**。
-- 值域是否夠寬只能靠 scripts/test-postgres-migrations.sh 與
-- scripts/test-mysql-migrations.sh 裡的 …MigrationsRealValuesFitAllColumns 驗。

-- +goose Up
-- 無事可做。

-- +goose Down
-- 無事可做。

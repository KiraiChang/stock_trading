-- +goose Up
-- 【resolved_role 設計，見 docs/sr-zone-scoring.md 十五】role 是分析當下依現價判斷的角色，寫入後不會
-- 再變動；AT_ZONE 的 zone 在後續驗證（見 internal/analysis/sr_zone_verifier.go
-- 的 verifySRZone）等到價格真正收盤離開區間才能解析出方向，這個解析結果
-- 需要獨立欄位保存，不能覆寫原始 role（否則會失去「分析當下是 AT_ZONE」
-- 這個歷史資訊），也不能不存（否則前端只能看到 role=AT_ZONE，卻同時看到
-- status 已經是 HELD_SO_FAR/BROKEN，兩者互相矛盾）。role != AT_ZONE 的
-- zone 永遠是 NULL（角色從分析當下就已明確，不需要另外解析）。
ALTER TABLE stock_sr_zones ADD COLUMN resolved_role VARCHAR(15) NULL;

-- +goose Down
ALTER TABLE stock_sr_zones DROP COLUMN resolved_role;

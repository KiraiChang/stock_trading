-- 除權息還原需要**第二個**係數（T-042 Phase 2）。
--
-- 現金股利讓價格下修，但**股數沒有改變**，所以成交量不可以跟著調整；
-- 分割與配股才會改變股數。價與量因此不能共用同一個累積係數。
--
-- 連帶後果：Phase 1 的恆等式 adj_price * adj_volume == price * volume
-- 不再全域成立（現金股利發生時錢真的離開公司），只對 adj_factor = vol_factor 的列成立。

-- +goose Up
ALTER TABLE corporate_actions
    ADD COLUMN IF NOT EXISTS volume_factor DECIMAL(18,10) NOT NULL DEFAULT 1;
ALTER TABLE candles
    ADD COLUMN IF NOT EXISTS vol_factor DECIMAL(18,10) NOT NULL DEFAULT 1;

-- Phase 1 已寫入的事件（分割／反分割／面額變更）：股數有變，成交量係數等於價格係數。
--
-- **必須排除除權息**：現金股利的 volume_factor 正確值就是 1，若一併套用
-- `volume_factor = factor` 會把它改成價格係數，等於把現金股利也算進成交量調整。
-- 光靠 `volume_factor = 1` 過濾不夠——那正是現金股利的正確值。
-- 這個情境在 down→up 重跑本 migration 時會真實發生（欄位被 drop 後全部回到預設 1）。
UPDATE corporate_actions SET volume_factor = factor
 WHERE factor <> 1 AND action_type NOT LIKE 'DIVIDEND%';

-- 檢查 6（verify-adjustment.sh）用 LN(volume_factor) 連乘，0 或負數會變成 -inf。
ALTER TABLE corporate_actions
    ADD CONSTRAINT ck_corporate_actions_volume_factor CHECK (volume_factor > 0);

-- +goose Down
ALTER TABLE corporate_actions DROP CONSTRAINT IF EXISTS ck_corporate_actions_volume_factor;
ALTER TABLE candles DROP COLUMN IF EXISTS vol_factor;
ALTER TABLE corporate_actions DROP COLUMN IF EXISTS volume_factor;

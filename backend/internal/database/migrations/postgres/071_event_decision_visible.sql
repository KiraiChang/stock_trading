-- 事件鏈補上決策可見性旗標（todo.md T-041 的 review 發現 R1）。
-- 現況規格見 docs/sr-zone-scoring.md「事件的決策可見性」與 docs/database-schema.md。
--
-- 要解的問題：階段 D 的隔離在**顯示層**漏掉一段。SUPPORT_RETEST 與 RESISTANCE_BREAKOUT
-- 兩個 family 是「只寫不讀的事實紀錄」（state_json.decision_visible=false，不進任何
-- 決策桶），但 event_instances 沒有存這個旗標，於是 GET /sr-zones/event-timeline 回傳的
-- 鏈與決策可見的鏈長得一模一樣——使用者會把 RESISTANCE_BREAKOUT / CONFIRMED / BULLISH
-- 讀成突破買訊。
--
-- **旗標仍由 Python 單一產生**（event_engine.EVENT_TYPE_META），Go 只是把它從
-- state_json 帶進身分層，不自己依 event_family 推導。理由與 carried_from_previous 相同：
-- 兩份型別清單分歧時沒有任何東西會報錯。
--
-- **預設 TRUE**：既有四個事件型別都是決策可見的，階段 D 之前寫進去的列也不會有這個鍵。
-- 當成 FALSE 會讓既有事件整批被標成不參與決策——那是最嚴重的行為改變。

-- +goose Up
ALTER TABLE event_instances
    ADD COLUMN IF NOT EXISTS decision_visible BOOLEAN NOT NULL DEFAULT TRUE;

-- 一次性回填：已終結的鏈不會再被寫入，不回填就會永遠標錯。
--
-- **這是資料修正，不是執行期推導。** 執行期的值一律來自 state_json；這裡寫死兩個
-- family 名字只因為回填母體固定（階段 D 於 2026-08-20 上線後產生的列）且三個 engine
-- 的 JSON 取值語法各不相同。**不要把這行當成「Go 側可以照 family 判斷」的先例。**
UPDATE event_instances SET decision_visible = FALSE
 WHERE event_family IN ('SUPPORT_RETEST', 'RESISTANCE_BREAKOUT');

-- +goose Down
ALTER TABLE event_instances DROP COLUMN IF EXISTS decision_visible;

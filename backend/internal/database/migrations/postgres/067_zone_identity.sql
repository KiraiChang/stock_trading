-- Zone 身分、一世與轉換（T-048 階段 B）。規格見 docs/todo.md T-048。
--
-- 要解的問題：現行 zone 身分是 event_engine._zone_key()，也就是
-- `role:price_low:price_high`——身分綁在浮點邊界與角色上。2026-08-18 對 live 的盤點
-- 抓到兩個後果：同一個支撐分裂成兩條事件鏈（0050 從 08-05 起兩個 key 每天並存，
-- 價格區間重疊 99%），以及角色翻轉必然斷鏈（實測有 IoU=1.000 的翻轉，邊界一動也沒動）。
--
-- **本批只寫不讀**：四張表都沒有任何決策路徑會查詢它們。這讓階段 B 可以先累積真實資料，
-- 等階段 C／T-049 有東西可比對之後再切換。既有的 market_event_states /
-- market_event_detections 不刪、繼續並行寫入。
--
-- 分三層是刻意的：
--   zone_instances          身分。跨越失效與角色翻轉。
--   zone_role_incarnations  一世。INVALIDATED 是「這一世」的終態，不是身分的終態——
--                           同一個價位失效後又重新有效，是同一個身分的下一世。
--                           這樣「這個價位長期是不是關鍵」與「這一世活了多久」都答得出來。
--   zone_transitions        轉換流水。狀態與角色分開記，見 transition_kind。
-- 外加 zone_relations 表達分裂／合併的血緣。

-- +goose Up

CREATE TABLE IF NOT EXISTS zone_instances (
    -- opaque UUID。**不可把價格或 role 編進去**——那正是現行 _zone_key() 的問題成因。
    zone_uid    VARCHAR(64)  PRIMARY KEY,
    symbol      VARCHAR(10)  NOT NULL,
    timeframe   VARCHAR(8)   NOT NULL,
    -- atr / volume_profile / recent_pivot。不同方法算出來的東西不互相匹配，
    -- 價格碰巧重疊不代表是同一個結構。
    method      VARCHAR(32)  NOT NULL,
    -- ACTIVE：身分還在（不管當前這一世是不是 INVALIDATED）。
    -- SPLIT / MERGED / RESHAPED：身分終態，血緣在 zone_relations。
    state       VARCHAR(16)  NOT NULL DEFAULT 'ACTIVE',
    -- 最近一次觀測到的邊界。**這不是身分**，只是讓人查表時看得懂這個 uid 大概在哪。
    price_low   DECIMAL(18,6) NOT NULL,
    price_high  DECIMAL(18,6) NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL,
    -- 資格閘門的第一個軸（wall-clock 陳舊度）。**距離用交易日算不是日曆天**，
    -- 交易日曆由呼叫端提供，這裡只存最後一次被觀測到的時間點。
    last_seen_at  TIMESTAMPTZ NOT NULL,
    -- 資格閘門的第二個軸：連續幾次「觀測到它不存在」。與時間軸獨立——
    -- 單靠時間分不出「zone 消失了」與「我們根本沒看」（實測 2330 全期只有 4 次分析、
    -- 橫跨 5 週）。達到 MAX_OBSERVED_ABSENCES 就移出候選集合。
    observed_absences INTEGER NOT NULL DEFAULT 0,
    ended_at      TIMESTAMPTZ NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_zone_instances_prices CHECK (price_high >= price_low)
);
-- 熱路徑：每次分析要撈「這檔還活著的身分」當 matcher 的 previous。
CREATE INDEX IF NOT EXISTS idx_zone_instances_live
    ON zone_instances (symbol, timeframe, state, last_seen_at);

CREATE TABLE IF NOT EXISTS zone_role_incarnations (
    incarnation_uid VARCHAR(64) PRIMARY KEY,
    zone_uid        VARCHAR(64) NOT NULL REFERENCES zone_instances(zone_uid),
    -- 這是該身分的第幾世，從 1 起算。
    seq             INTEGER     NOT NULL,
    -- **只收 SUPPORT / RESISTANCE。** AT_ZONE 是「方向暫時無法解析」不是角色：
    -- live 有一條連續 16 次分析都是 AT_ZONE 的鏈，讓它開一世會產生沒有語意的紀錄。
    -- AT_ZONE 期間沿用這一世原本的角色，只在 zone_transitions 留下 UNRESOLVED/RESOLVED。
    role            VARCHAR(16) NOT NULL,
    -- ACTIVE / TESTING（可來回）/ INVALIDATED / EXPIRED（後兩者是這一世的終態）
    --
    -- **EXPIRED 與 INVALIDATED 的差別是誰造成的**：INVALIDATED 是市場事件
    -- （被跌破/突破），EXPIRED 是「我們不再認得它」——長期缺席而失去匹配資格。
    -- 兩者都不終止身分本身，同一個價位之後可以開下一世。
    state           VARCHAR(16) NOT NULL DEFAULT 'ACTIVE',
    started_at      TIMESTAMPTZ NOT NULL,
    ended_at        TIMESTAMPTZ NULL,
    -- 因缺席而收攤的時間點。與 ended_at 分開存是刻意的：ended_at 回答「這一世何時結束」，
    -- expired_at 回答「何時被判定為不再認得」，後者才是資格閘門的稽核依據。
    expired_at      TIMESTAMPTZ NULL,
    -- INVALIDATED（被跌破/突破）/ ROLE_FLIPPED（翻轉，緊接著開下一世）/
    -- EXPIRED_BY_ABSENCE（長期缺席）/ IDENTITY_ENDED（身分本身終止）
    end_reason      VARCHAR(32) NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_zone_incarnation_seq UNIQUE (zone_uid, seq)
);
CREATE INDEX IF NOT EXISTS idx_zone_incarnations_open
    ON zone_role_incarnations (zone_uid, ended_at);

CREATE TABLE IF NOT EXISTS zone_transitions (
    id              BIGSERIAL   PRIMARY KEY,
    zone_uid        VARCHAR(64) NOT NULL REFERENCES zone_instances(zone_uid),
    incarnation_uid VARCHAR(64) NULL REFERENCES zone_role_incarnations(incarnation_uid),
    -- 觸發這次轉換的分析。可為 NULL：身分終止有可能由排程收尾而不是某次分析。
    analysis_id     BIGINT      NULL,
    -- STATE_CHANGE / ROLE_RESOLVED / ROLE_UNRESOLVED / ROLE_FLIPPED
    --
    -- **role 的三種變化必須分開**，混為一談會讓真正的翻轉被雜訊淹沒：實測 161 個匹配
    -- 配對裡，AT_ZONE 的進出有 15 筆、真正的 SUPPORT↔RESISTANCE 翻轉只有 3 筆。
    transition_kind VARCHAR(24) NOT NULL,
    from_state      VARCHAR(16) NULL,   -- 純 role 轉換時為 NULL
    to_state        VARCHAR(16) NULL,
    from_role       VARCHAR(16) NULL,
    to_role         VARCHAR(16) NULL,
    -- **不合法的轉換照樣寫進來，只標記不擋。** 階段 B 沒有任何決策依賴這些資料，
    -- 這時候應該先搞清楚現實世界真的會發生什麼，而不是拿猜想的規則把證據擋掉。
    -- 判讀時記得過濾這個旗標。
    is_illegal      BOOLEAN     NOT NULL DEFAULT FALSE,
    -- 用 TEXT 而不是 JSONB：三個 engine 一致（mysql 的 JSON 欄位給不起 DEFAULT），
    -- 讀寫走 store.RawJSON。比照 migration 028 的既有做法。
    reason_codes    TEXT        NOT NULL DEFAULT '[]',
    occurred_at     TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_zone_transitions_zone
    ON zone_transitions (zone_uid, occurred_at);
-- 判讀「引擎有沒有算錯」時查這個。刻意不用 postgres 專屬的部分索引，
-- 三個 engine 用同一種複合索引，少一個 engine 差異就少一個 mysql 專屬的驚喜。
CREATE INDEX IF NOT EXISTS idx_zone_transitions_illegal
    ON zone_transitions (is_illegal, occurred_at);

CREATE TABLE IF NOT EXISTS zone_relations (
    parent_zone_uid VARCHAR(64) NOT NULL REFERENCES zone_instances(zone_uid),
    child_zone_uid  VARCHAR(64) NOT NULL REFERENCES zone_instances(zone_uid),
    -- SPLIT（1→N）/ MERGE（N→1）/ RESHAPE（N→M，血緣無法解析）
    --
    -- **沒有 CONTINUE。** 身分延續由 zone_uid 不變表達即可；寫成 parent = child 的
    -- 自環會讓沿 parent 遞迴回溯祖先的查詢無法終止（WITH RECURSIVE 沒有 cycle 偵測
    -- 會直接失敗）。CHECK 把這件事釘在 schema 上。
    relation        VARCHAR(16) NOT NULL,
    analysis_id     BIGINT      NULL,
    occurred_at     TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (parent_zone_uid, child_zone_uid, occurred_at),
    CONSTRAINT ck_zone_relations_no_self_loop CHECK (parent_zone_uid <> child_zone_uid)
);
CREATE INDEX IF NOT EXISTS idx_zone_relations_child
    ON zone_relations (child_zone_uid);

-- +goose Down
DROP INDEX IF EXISTS idx_zone_relations_child;
DROP TABLE IF EXISTS zone_relations;
DROP INDEX IF EXISTS idx_zone_transitions_illegal;
DROP INDEX IF EXISTS idx_zone_transitions_zone;
DROP TABLE IF EXISTS zone_transitions;
DROP INDEX IF EXISTS idx_zone_incarnations_open;
DROP TABLE IF EXISTS zone_role_incarnations;
DROP INDEX IF EXISTS idx_zone_instances_live;
DROP TABLE IF EXISTS zone_instances;

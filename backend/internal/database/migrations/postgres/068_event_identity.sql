-- 事件身分與生命週期（T-048 階段 C）。
-- 現況規格見 docs/sr-zone-scoring.md「Zone 身分與 ZoneMatcher」與 docs/database-schema.md。
--
-- 要解的問題有兩個：
--   1. 現行 market_event_states 的鍵是 _zone_key()（role:price_low:price_high），
--      邊界每次由 ATR 重算，同一個支撐會分裂成兩條並存的鏈。本表改用階段 B 做出來的
--      穩定 zone_uid。
--   2. 事件鏈**沒有被存下來**：internal/analysis/event_timeline.go 是回讀
--      market_event_states 的歷史快照、在讀取時摺疊出 timeline，而那份摺疊規則與
--      Python 的 build_event_state_summary 是兩份必須保持對稱的實作，
--      沒有任何東西會在它們分歧時報錯。本表把生命週期變成存下來的事實。
--
-- **本批只寫不讀**：沒有任何決策路徑會查這兩張表。既有的 market_event_states /
-- market_event_detections 不刪、繼續並行寫入，逐欄比對是驗收條件。

-- +goose Up

CREATE TABLE IF NOT EXISTS event_instances (
    event_uid   VARCHAR(64) PRIMARY KEY,
    symbol      VARCHAR(10) NOT NULL,
    timeframe   VARCHAR(8)  NOT NULL,
    -- **可為 NULL**：live 的 86 筆 market_event_states 裡有 12 筆 event_scope='SYMBOL'，
    -- 它們本來就不屬於任何 zone。設成 NOT NULL 會逼這些事件塞一個假的 zone。
    zone_uid    VARCHAR(64) NULL REFERENCES zone_instances(zone_uid),
    -- zone_uid 的可空版本無法直接進唯一鍵：postgres 的 UNIQUE 不擋多個 NULL，
    -- mysql 也不擋，sqlite 同樣不擋——三個 engine 都「不擋」，於是同一個 SYMBOL 事件
    -- 可以無限重複建立而不報錯。所以另存一個 NOT NULL 的投影鍵：
    -- ZONE scope 時等於 zone_uid，SYMBOL scope 時是字串 'SYMBOL'。
    zone_scope_key VARCHAR(64) NOT NULL,
    event_scope VARCHAR(20) NOT NULL,   -- ZONE / SYMBOL
    event_family VARCHAR(80) NOT NULL,
    -- 同一個 (zone, family) 可以有多條**先後**的鏈：前一條 RESOLVED／EXPIRED 之後
    -- 再出現同家族事件，那是新的一條鏈而不是舊鏈復活（規則與 Python 的
    -- build_event_state_summary 對稱）。用 seq 表達先後，比照 zone_role_incarnations。
    seq         INTEGER     NOT NULL DEFAULT 1,
    -- root 是這條鏈的起點，latest 是最近一次偵測。**root 不可被新偵測蓋掉**，
    -- 否則欄位名叫 root 卻永遠等於 latest，鏈的起點無法還原（T-045 踩過）。
    root_event_type   VARCHAR(80) NOT NULL,
    latest_event_type VARCHAR(80) NOT NULL,
    -- CANDIDATE / CONFIRMED / ACTIVE / RESOLVED / EXPIRED
    state       VARCHAR(40) NOT NULL,
    -- active 不等於 state：它還要通過該 family 的 gating_states 才算數。
    active      BOOLEAN     NOT NULL DEFAULT FALSE,
    direction   VARCHAR(20) NOT NULL,
    resolved_by VARCHAR(80) NULL,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at  TIMESTAMPTZ NOT NULL,
    -- 只有鏈終結（RESOLVED / EXPIRED / zone 身分終止）才有值。
    ended_at    TIMESTAMPTZ NULL,
    -- 這條鏈最近一次被觀測到時，事件身上帶的 zone_key（Python 的 zone_identity_key 產出）。
    -- **這是鏈延續的第一把鑰匙。** zone 的邊界每次由 ATR 重算、role 也會翻轉，
    -- 所以「事件帶的 key → 本次分析的 zone」有很高機率對不上（實測 26/41 筆）；
    -- 但鏈本身還活著。用這個欄位以 (last_zone_key, event_family) 直接找回活鏈，
    -- 就能完全跳過 key 解析，這是 F1「鏈靜默凍結」的根治。
    -- 每次寫入都要更新成本次事件帶的 key，停在誕生那天的值會讓這把鑰匙永遠 miss。
    last_zone_key  VARCHAR(96) NULL,
    -- RESOLVED / EXPIRED / ZONE_IDENTITY_ENDED。後者是階段 C 的定案：zone 因
    -- SPLIT / MERGE / RESHAPE 終止時，parent 身上的事件**不傳給 child**——
    -- 那條鏈的前提（那個 zone 存在）已經消失，接到 child 等於宣稱鏈延續了，
    -- 而 RESHAPE 的定義正是「血緣無法解析」。血緣留在 zone_relations，
    -- 之後若要沿 parent 回溯補得回來；反過來先接了再拆拆不回來。
    end_reason  VARCHAR(32) NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_event_instance_seq UNIQUE (symbol, timeframe, zone_scope_key, event_family, seq)
);
-- 熱路徑：每次分析要撈「這檔還沒終結的事件鏈」當 previous。
CREATE INDEX IF NOT EXISTS idx_event_instances_live
    ON event_instances (symbol, timeframe, ended_at, last_seen_at);
CREATE INDEX IF NOT EXISTS idx_event_instances_zone
    ON event_instances (zone_uid);
-- 第一把鑰匙的查找路徑：(last_zone_key, event_family) → 活鏈。
CREATE INDEX IF NOT EXISTS idx_event_instances_last_zone_key
    ON event_instances (symbol, timeframe, last_zone_key, event_family);

CREATE TABLE IF NOT EXISTS event_transitions (
    id          BIGSERIAL   PRIMARY KEY,
    event_uid   VARCHAR(64) NOT NULL REFERENCES event_instances(event_uid),
    -- 可為 NULL：鏈的終結有可能由排程收尾而不是某次分析。
    analysis_id BIGINT      NULL,
    -- **from_state IS NULL 恰好等於「鏈誕生」**（to_state 是誕生時的 state）。
    -- 這條不變式沿用 067 的定案：zone 那邊因為誕生不寫 transition、
    -- 而失格也留白 from_state，導致 `from_state IS NULL` 什麼都問不出來（原 issue.md
    -- I-076）。事件表不再踩第二次——所有非誕生的轉換都必須明寫 from_state。
    from_state  VARCHAR(40) NULL,
    to_state    VARCHAR(40) NOT NULL,
    -- 觸發這次轉換的事件型別。鏈誕生與 carried 事件過期時可能沒有觸發事件。
    trigger_event_type VARCHAR(80) NULL,
    reason_codes TEXT       NOT NULL DEFAULT '[]',
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_event_transitions_event
    ON event_transitions (event_uid, occurred_at);

-- zone 身分歷來用過的 zone_key。**這是 key 解析的第二把鑰匙**（T-048 階段 C 修法）。
--
-- 要解的問題：事件身上帶的 zone_key 是「上次那個 zone 長什麼樣」，而本次分析的
-- zone_key 由這次的 ATR 邊界與 role 算出來——兩者對不上是常態而非例外。
-- 實測兩個成因：role 從 SUPPORT 變成 AT_ZONE（role 編在 key 裡），
-- 以及邊界漂走。身分都還 ACTIVE，只是 key 到不了它。
--
-- **刻意不做成 zone_instances.last_zone_key 單一欄位**：那只記得住最後一次，
-- 而缺席容忍是 3 次、翻轉前後還要再回溯一段，單一欄位覆蓋不到。更重要的是
-- 那會與本表成為兩份事實，分歧時沒有任何東西會報錯——那正是 T-048 一路在解的那類問題。
--
-- **有上限**：每個 zone_uid 只保留最新 8 筆（prune 在寫入的同一個交易內做）。
-- 邊界每次重算，不設上限這張表會隨分析次數單調成長。
CREATE TABLE IF NOT EXISTS zone_key_aliases (
    zone_uid      VARCHAR(64) NOT NULL REFERENCES zone_instances(zone_uid),
    -- role:price_low:price_high，與 market_event_states.zone_key 同一個函數產生。
    zone_key      VARCHAR(96) NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (zone_uid, zone_key)
);
-- 查找方向是 zone_key → zone_uid，且同一個 key 對到多個活身分時取最新的那個。
CREATE INDEX IF NOT EXISTS idx_zone_key_aliases_key
    ON zone_key_aliases (zone_key, last_seen_at);

-- +goose Down
DROP INDEX IF EXISTS idx_zone_key_aliases_key;
DROP TABLE IF EXISTS zone_key_aliases;
DROP INDEX IF EXISTS idx_event_instances_last_zone_key;
DROP INDEX IF EXISTS idx_event_transitions_event;
DROP TABLE IF EXISTS event_transitions;
DROP INDEX IF EXISTS idx_event_instances_zone;
DROP INDEX IF EXISTS idx_event_instances_live;
DROP TABLE IF EXISTS event_instances;

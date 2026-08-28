# API Reference

Base URL：`http://localhost:8080/api/v1`

除 `/auth/*` 外，所有端點皆需在 Header 帶入 JWT Token：

```
Authorization: Bearer <token>
```

---

## Auth API（公開，不需 token）

### POST `/auth/register`

註冊新使用者。**新帳號預設 `status: inactive`，無法立即登入，需由已登入的使用者在管理頁面或透過 `PATCH /users/:id/status` 啟用。**

**Request Body：**
```json
{ "email": "user@example.com", "password": "secret123" }
```

**Response（201）：**
```json
{ "user_id": 1, "email": "user@example.com", "status": "inactive" }
```

### POST `/auth/login`

登入取得 JWT token。

**Request Body：**
```json
{ "email": "user@example.com", "password": "secret123" }
```

**Response（200）：**
```json
{ "token": "eyJhbGci...", "expires_in": 86400 }
```

**錯誤回應：**

| 狀態碼 | 說明 |
|--------|------|
| 401 | 帳號或密碼錯誤 |
| 403 | 帳號尚未啟用（`status != active`），需管理員開通 |

Token 有效期 24 小時。之後請求帶入 `Authorization: Bearer <token>`。

---

## User Management API

### GET `/users`

列出所有使用者（不含密碼雜湊）。

**Response：**
```json
{
  "users": [
    {
      "id": 1,
      "email": "admin@trading.com",
      "status": "active",
      "created_at": "2024-01-15 10:00:00"
    },
    {
      "id": 2,
      "email": "newuser@example.com",
      "status": "inactive",
      "created_at": "2024-01-16 09:30:00"
    }
  ]
}
```

### PATCH `/users/:id/status`

啟用或停用指定使用者。

**Request Body：**
```json
{ "status": "active" }
```

`status` 只接受 `"active"` 或 `"inactive"`。

**Response（200）：**
```json
{ "id": 2, "status": "active" }
```

---

## Candle API

### GET `/candles/:symbol`

取得 K 棒資料。**回傳的是還原價**（見下方說明）。

**Query Parameters：**

| 參數 | 預設 | 說明 |
|------|------|------|
| timeframe | `1d` | `1m`, `5m`, `1d` |
| limit | `60` | 最多 1000 |

**Response：**
```json
{
  "symbol": "2330",
  "timeframe": "1d",
  "candles": [
    {
      "symbol": "2330", "timeframe": "1d",
      "open": 975.0, "high": 980.0, "low": 970.0, "close": 978.0,
      "volume": 25000000, "amount": 24450000000,
      "ts": "2024-01-15T00:00:00+08:00",
      "adj_factor": 1, "vol_factor": 1
    }
  ]
}
```

#### 回傳的是還原價，不是原始成交價

`open/high/low/close` 已套用累積還原係數，`volume` 已套用成交量係數。DB 存的是**原始價**，
調整在 handler 完成——**呼叫端不需要、也不應該自己乘係數**（前端是第三個語言，
同一段邏輯散到三處時，算錯的那一處不會有任何東西告訴你）。

不還原的話，跨越公司行動的 K 線圖會出現一根從未發生的暴跌：0050 在 2025-06-18 的 1:4 分割
會讓價格從 188.65 掉到 47.57。

| 欄位 | 說明 |
|------|------|
| `adj_factor` | 價格的累積還原係數。`1` 代表該區間之後沒有公司行動 |
| `vol_factor` | **成交量**的係數。與 `adj_factor` 不同：現金股利讓價格下修但股數沒變，所以量不調整；只有分割與配股會讓兩者不相等 |
| `amount` | **不調整**。成交金額是錢，不隨股數重新定義 |

因此 `close × volume`（還原後）**只在 `adj_factor == vol_factor` 時**等於原始的乘積。
現金股利發生時錢真的離開公司，乘積本來就該變小。

需要「當時實際成交在哪裡」的原始價時，這支端點目前沒有開關——現況沒有這種呼叫端，
等真的有需求再加（背景見 [`todo.md`](./todo.md) T-042）。

---

## Indicator API

### GET `/indicators/:symbol`

取得最新技術指標快照。

**Query Parameters：**

| 參數 | 預設 |
|------|------|
| timeframe | `1d` |

**Response：**
```json
{
  "id": 1, "symbol": "2330", "timeframe": "1d",
  "ts": "2024-01-15T00:00:00+08:00",
  "ma5": 975.4, "ma10": 968.2, "ma20": 955.1, "ma60": 920.3,
  "rsi14": 63.5,
  "macd": 8.2, "macd_signal": 6.1, "macd_hist": 2.1,
  "bb_upper": 995.2, "bb_middle": 955.1, "bb_lower": 915.0,
  "atr14": 12.5,
  "vwap": 976.3,
  "vol_ma20": 20000000, "vol_ratio": 1.25
}
```

---

### POST `/indicators/:symbol/compute`

手動計算單一股票的最新指標快照並寫入 DB，**不要求該股票在監控清單裡**，
只要求 `candles` 至少有 35 根（`timeframe` 對應的週期）。同步執行、直接
回傳算出來的結果，用來補算「有 candles 但從未被排程算過指標」的股票
（例如剛用 backfill 拉完歷史資料、但還沒加進監控清單的股票）。

**Query Parameters：** `timeframe`（預設 `1d`）

**Response（200）：** 格式同 `GET /indicators/:symbol`。

**錯誤：** candles 不足 35 根時回傳 `422 Unprocessable Entity`。

前端「歷史資料回補」頁面（`/backfill`）下方有「手動計算指標」區塊，輸入任意
股票代號即可觸發，不需要透過 API 手動呼叫。

---

## Signal API

### GET `/signals`

取得訊號記錄。

**Query Parameters：**

| 參數 | 說明 |
|------|------|
| limit | 筆數（預設 50） |
| symbol | 篩選特定股票 |

**Response：**
```json
{
  "signals": [
    {
      "id": 1, "symbol": "2330",
      "signal_type": "BREAKOUT", "direction": "BUY",
      "price": 980.0, "volume": 45000000, "vol_ratio": 2.25,
      "resistance": 975.0, "support": 0.0, "trend": "BULLISH",
      "note": "突破阻力 975.00，量比 2.25x",
      "ts": "2024-01-15T10:30:00+08:00"
    }
  ],
  "total": 1
}
```

---

### POST `/signals/:symbol/evaluate`

手動觸發訊號評估。完全基於 `candles`（OHLCV）計算——內部會先呼叫指標計算
（同 `/indicators/:symbol/compute`），再做支撐/壓力/趨勢判斷與
`CheckBreakout`——不需要即時行情、不要求該股票在監控清單裡，適合**收盤後**
立刻確認某支股票當天有沒有觸發訊號，不用等 `daily_close` 排程（15:00 才對
監控清單跑）。

**Query Parameters：** `timeframe`（預設 `1d`）

**Response（200，有觸發）：**
```json
{ "signal": { "id": 1, "symbol": "2330", "signal_type": "BREAKOUT", "direction": "BUY", "...": "..." } }
```

**Response（200，沒有觸發）：**
```json
{ "signal": null, "message": "沒有觸發訊號（不符合突破/跌破/爆量條件）" }
```

**錯誤：** candles 不足 35 根時回傳 `422 Unprocessable Entity`。

前端「歷史資料回補」頁面（`/backfill`）下方有「手動評估訊號」區塊，輸入任意
股票代號即可觸發，不需要透過 API 手動呼叫。

---

## Watchlist API

### GET `/stock-symbols/search`

搜尋股票主檔，供 watchlist 新增股票時 autocomplete 使用。預設只回最近一次
TWSE ISIN 同步仍存在的標的。

**Query：**

| 參數 | 說明 |
|------|------|
| q | 代號或名稱關鍵字 |
| listed | 是否只查仍上市，預設 `true` |
| security_type | 依 TWSE ISIN 分類過濾。**值是中文**，例如 `股票` / `ETF`（實測值見 `stock_symbols.security_type`） |
| limit | 回傳筆數，預設 20、上限 100 |

**Response：**
```json
{
  "symbols": [
    {
      "symbol": "2330",
      "name": "台積電",
      "isin_code": "TW0002330008",
      "market": "上市",
      "security_type": "股票",
      "industry": "半導體業",
      "is_listed": true
    }
  ]
}
```

### GET `/stock-symbols/facets`

回傳可用的篩選選項與**母體**筆數，供前端產生選單。沒有這支的話，使用者只能手打
`半導體業` 這類 TWSE ISIN 的原始中文分類——**打錯的後果是 HTTP 200 ＋ 0 筆**，
與「條件真的沒匹配」在畫面上無法區分。

**Query：**

| 參數 | 說明 |
|------|------|
| security_type | 逗號分隔。**只縮放 `industries` 的範圍，不影響回傳的 `security_types` 清單**——選單本身要一直完整，否則使用者選了某個類型之後就換不回來 |
| include_delisted | 預設 `false`，與 `/candidates` 一致 |

**Response：**
```json
{
  "security_types": [
    {"value": "上市認購(售)權證", "count": 31090},
    {"value": "股票", "count": 1945},
    {"value": "ETF", "count": 354}
  ],
  "industries": [
    {"value": "電子零組件業", "count": 209},
    {"value": "半導體業", "count": 201}
  ]
}
```

- `count` 是**母體**筆數，不是取樣後的數量。挑 `/candidates` 的 `per_industry` 時要看母體
  才知道 9 是多是少（半導體業 201 檔 vs 玻璃陶瓷 5 檔）；`/candidates` 的 `by_industry`
  給的是取樣**後**的數字，兩者不要混用。
- `industries` **排除 `industry = ''`**：那是「未分類」而不是一個產業，ETF 與權證全落在那裡。
  所以 `?security_type=ETF` 會回傳空的 `industries`。
- 兩個陣列都保證是 `[]` 而非 `null`。

### GET `/stock-symbols/candidates`

批次產生**研究用**的候選標的清單，供擴評估標的池使用（見 [`todo.md`](./todo.md) T-040
的 Step 1／Step 3）。回傳的 `symbols` 可直接餵給 `POST /market/backfill`。

**與 `/search` 的分野**：`/search` 是 watchlist 的 autocomplete，筆數上限刻意壓在 100；
本端點是研究用的批次取用，上限 5,000。兩者用途不同，不要互相取代。

**Query：**

| 參數 | 說明 |
|------|------|
| security_type | 逗號分隔。**留空預設 `股票,ETF`**，見下方警告 |
| industry | 逗號分隔，空 = 不限 |
| listed_years | 只留上市滿 N 年的標的。**`listed_date` 為 NULL 者一律排除**——證不出上市夠久就不該進研究母體。`0` 或留空 = 不限 |
| per_industry | 每個產業最多幾檔。半導體業有 201 檔，不設限時抽樣會被它主導。**在該產業的代號區間內等距取樣**（不是取代號最小的前 N 檔），且是**決定性的**——同條件每次拿到同一批 |
| limit | 總筆數上限，預設 3000、上限 5000 |
| include_delisted | 預設 `false`，研究母體不含已下市標的 |

> **`security_type` 為什麼有預設值**：`stock_symbols` 存的是完整的 TWSE ISIN 主檔。
> 實測 43,061 筆上市資料裡有 **40,658 筆是認購（售）權證**（佔 94%），而且代號排序在股票之前。
> 沒有預設值的話，一個不帶參數的請求會回傳「ETF ＋ 權證」而一檔股票都沒有——
> 這份 `symbols` 又被設計成可直接餵給 `POST /market/backfill`（無筆數上限、5 req/min），
> 等於把數小時的 FinMind 配額花在沒有 K 線的商品上。要權證請明確指定。

> **`per_industry` 對「沒有產業分類」的列不生效**：`industry` 是 `NOT NULL DEFAULT ''`，
> 而 ETF 與權證的產業欄位都是空字串。空字串代表「未分類」而不是一個產業，
> 若一併套用上限，`security_type=股票,ETF&per_industry=9` 會讓 354 檔 ETF 只剩 9 檔。

**Response：**
```json
{
  "count": 2,
  "symbols": ["2330", "2603"],
  "by_industry": {"半導體業": 1, "航運業": 1},
  "rows": [{"symbol": "2330", "name": "台積電", "industry": "半導體業", "...": "同 /search 的欄位"}],
  "truncated": false
}
```

- `by_industry`：給人工核對產業分佈用——Step 1 要確認抽樣沒有被單一產業壓垮。
- `truncated`：`true` 代表還有更多符合條件的標的被 `limit` 砍掉。**截斷依代號順序**，
  會整批砍掉高代號的產業，正是 `per_industry` 要消除的偏斜，所以拿到 `true` 時
  應該調高 `limit` 或收緊條件，而不是直接使用這份清單。

### GET `/sr-zones/event-timeline`

取得事件鏈（Event Timeline），供決策畫面呈現「事件如何一路演進到現在」。

**資料來源是身分層**（`event_instances` ＋ `event_transitions`），不是把
`market_event_states` 的快照摺疊出來的——後者以 `zone_key` 為鍵，會因邊界漂移把同一個
zone 的鏈拆開（見 [`issue.md`](./issue.md) I-080）。背景見 [`todo.md`](./todo.md)
T-045 / T-048 / T-051。

**與 decision summary 的 `event_sequence` 不同，兩者不要混用：**

| | `event_sequence`（decision summary 內） | 本端點的 `chains` |
|---|---|---|
| 範圍 | **當次分析**偵測到的事件，依優先序排序去重 | **跨分析**的完整演進 |
| 有無時間 | 無 | 每一步都有 `occurred_at` |
| 有無狀態轉換 | 無 | 有，`from_state → state` |

**為什麼是 query 而不是 `/sr-zones/:symbol/...`**：同層已有 `GET /sr-zones/:id`，
gin 不允許同一位置有兩個不同名的 wildcard，那樣寫會在服務啟動時 panic。

**2026-08-20 起會出現只寫不讀的事件鏈。** `SUPPORT_RETEST` 與 `RESISTANCE_BREAKOUT`
兩個 family 的鏈也會被寫入並在這裡回傳，但**它們不參與任何決策**，只是事實紀錄。
每條 chain 都帶 **`decision_visible`**（2026-08-21 起，見下方欄位說明），顯示端要靠它
把事實紀錄與決策事件分開，不要當成會影響 Bias 或進場的事件。同一件事在 `POST /sr-zones` 的
`decision_summary.market_events[]` 與 `event_state_summary.states[]` 也看得到；
`active` / `candidates` / `confirmed` / `resolved` / `expired` 與兩個方向桶
**不會**包含它們。`decision_visible` 這個鍵本身是**純新增**，既有事件一律是
`true`，缺鍵時也視為 `true`。語意見
[`sr-zone-scoring.md`](./sr-zone-scoring.md)「事件的決策可見性」。

**Query：**

| 參數 | 說明 |
|------|------|
| symbol | **必填** |
| timeframe | 預設 `1d` |
| max_analyses | 回溯幾次分析（不是幾列），預設 60、上限 500。**同時決定 `chains` 的視窗**——最舊那次分析的 K 棒時間就是起點 |

**Response：**
```json
{
  "symbol": "0050", "timeframe": "1d",
  "identity_since": "2026-08-01T00:00:00Z",
  "chains": [{
    "event_uid": "9a1c...",
    "zone_uid": "0f1c9a2e-8b5d-4c31-9a77-2f6e1d4b8c05",
    "zone_key": "SUPPORT:103.4487:104.0713",
    "event_scope": "ZONE",
    "event_family": "SUPPORT_RECLAIM",
    "seq": 1,
    "direction": "BULLISH",
    "root_event_type": "INTRADAY_RECLAIM",
    "latest_event_type": "INTRADAY_RECLAIM",
    "active": true,
    "first_seen_at": "2026-08-10T00:00:00Z",
    "last_seen_at": "2026-08-12T00:00:00Z",
    "closed": false,
    "final_state": "CONFIRMED",
    "decision_visible": true,
    "transitions": [
      {"occurred_at": "2026-08-10T00:00:00Z", "analysis_id": 41,
       "from_state": "CANDIDATE", "state": "CONFIRMED",
       "event_type": "INTRADAY_RECLAIM",
       "reason_codes": ["CLOSE_RECLAIM"]}
    ]
  }],
  "snapshots": [
    {"analysis_id": 40, "analyzed_at": "2026-08-09T00:00:00Z", "gap_days": 0},
    {"analysis_id": 41, "analyzed_at": "2026-08-10T00:00:00Z", "gap_days": 1}
  ]
}
```

**判讀時必須注意的七件事：**

1. **一條 chain ＝ 一個 zone 身分 × 一個 family × 一個 `seq`。** 鍵是 **`zone_uid`**，
   不是 `zone_key`——後者只是「最近一次觀測到時事件帶的 key」，供人工比對用。
   zone 邊界每次由 ATR 重算、role 也會翻轉，**同一個 zone 的 key 會一直變**
   （實測 329 個身分裡有 102 個漂移過 key），所以拿 `zone_key` 當身分會把一條鏈拆成好幾條。

   **`seq > 1` 是新的一條鏈，不是舊鏈復活**：前一條 `RESOLVED`／`EXPIRED` 之後再出現
   同家族事件就開新的一條，與寫入端語意一致。

2. **`end_reason = ZONE_IDENTITY_ENDED` 不是自然結束。** 那代表 zone 因
   `SPLIT`／`MERGE`／`RESHAPE` 而身分終止，鏈跟著收攤——把它畫成一般的
   `RESOLVED`／`EXPIRED` 會誤導。血緣留在 `zone_relations`，本端點不接。

3. **`snapshots[].gap_days` 不能忽略。** timeline 的解析度**等於 SR 分析的執行頻率**。
   鏈上的空白**不代表那段期間沒有事件**，只代表那段期間沒有分析。分析排程見
   `POST /scheduler/sr-analysis/run`（平日 17:00／22:00，預設關閉）。

   `snapshots` 取自 **`stock_sr_zone_analyses`（所有分析）**，不是事件鏈——
   一次沒有偵測到任何事件的分析不會留下任何鏈，只看鏈會把它報成「沒有觀測」。
   實測 0050 有 14 次分析、只有 11 次產生事件。

4. **視窗選的是「哪些鏈」，不是「鏈的哪幾步」。** 一條鏈只要**有一步落在視窗內、或它還沒
   結束**就會整條回傳（含視窗之前的步驟）。把鏈從中間切開會失去誕生那一步，而
   `from_state` 留白正是「鏈誕生」的標記——切一半的鏈會讓人以為它從中途冒出來。
   「還沒結束也算」是必要的：一條長壽而這段期間剛好沒有狀態變化的鏈，正是最該看到的。

5. **所有時間都在 K 棒軸上。** `transitions[].occurred_at`、`chains[].first_seen_at` /
   `last_seen_at`、`identity_since` 用的都是**該次分析的 K 棒時間**，與
   `snapshots[].analyzed_at` 同軸。身分層內部存的是 as_of 的 wall clock（那個軸量的是
   「我們看了幾次」），但**對外一律換算過**——否則整條鏈會擠在「跑分析的那一刻」。
   只有鏈由排程收尾、沒有 `analysis_id` 可依附時才會退回 wall clock。

6. **`identity_since` 之前沒有鏈可看。** 事件鏈是身分層開始寫入之後才有的，更早的分析
   **刻意不回填**（回填要解的正是「兩個舊 key 是不是同一個 zone」，而那正是身分層本身
   要建的能力）。早於它的 `snapshots` 照常列出，讓「這段沒有鏈」與「這段沒有分析」分得開。

   **它不受 `max_analyses` 影響**：問的是「身分層何時開始有紀錄」，不是「這次查了多久」。
   值由 `event_instances` 的**全歷史**算出（每條鏈取第一步所屬分析的 K 棒時間，沒有
   `analysis_id` 才退回 `occurred_at`，完全沒有轉換的鏈才退回 `first_seen_at`），
   所以它**可以早於 `chains` 裡最早的 `first_seen_at`**——那代表視窗之前還有已終結的鏈
   沒被撈進來，不是資料不一致。

7. **`decision_visible=false` 的鏈要標記，不要濾掉。** 這個欄位（2026-08-21 起）逐條帶出
   階段 D 的隔離旗標：`false` 代表這條鏈只寫不讀，不進任何決策桶。值一路來自 Python 的
   `event_engine.EVENT_TYPE_META`，**不要在呼叫端依 `event_family` 自己推導**——那等於
   維護第二份型別清單，兩份分歧時沒有任何東西會報錯。**沒有 `omitempty`**，`false`
   一定會出現在 JSON 裡；缺鍵（舊後端）一律視為 `true`。
   顯示端的定案是**標記而非隱藏**：這些鏈是「這個 zone 最近有沒有被測試過」的唯一依據，
   藏起來會讓人工判讀失去資訊。

這份資料定位為 **display chain**——供顯示與人工檢查，**不是** Lifecycle Engine 的 runtime 輸入。

### GET `/sr-zones/identity-stats`

身分關聯決策的逐次分析拆解與區間聚合（todo.md T-050）。

**這個端點存在的理由是趨勢**：同一組數字已經有結構化 log，但 log 答不出
「alias 命中率是不是在爬」「`chain_conflicts` 是不是開始非零」，而那正是這類缺陷的形狀
——它們的症狀是資料表面上完全正常。（立表時另一個理由是 `job_runs` 當時**只保留當天**、
隔天就查不到前一晚的排程；那一點已於 2026-08-25 改成保留 30 天，但兩者粒度不同——
`job_runs` 記每輪排程的成敗與標的數，答不出上面那些逐次分析的比率問題。）

**Query：**

| 參數 | 說明 |
|------|------|
| symbol | 選填，省略代表全部標的 |
| days | 回溯幾天，預設 30、範圍 1~365 |
| limit | 最多幾列，預設 200、範圍 1~1000 |

**Response：**
```json
{
  "rows": [
    {"analysis_id": 128, "symbol": "2330", "timeframe": "1d",
     "matched_by_chain": 6, "matched_by_current": 2, "matched_by_alias": 0,
     "unmatched_keys": 0, "chain_conflicts": 0, "alias_ambiguous": 0,
     "invariant_violations": 0,
     "zone_identity_degraded": false, "event_identity_degraded": false,
     "zone_live_candidates": 14, "zone_ended": 1,
     "created_at": "2026-08-21T17:00:12Z"}
  ],
  "summary": {
    "analyses": 22, "degraded_analyses": 0,
    "matched_by_chain": 130, "matched_by_current": 41, "matched_by_alias": 0,
    "matched_total": 171, "alias_hit_rate": 0.0,
    "unmatched_keys": 0, "chain_conflicts": 0, "chain_key_ambiguous": 0,
    "alias_ambiguous": 0, "carried_parse_fail": 0, "invariant_violations": 0
  }
}
```

**判讀時必須注意的三件事：**

1. **先看 `degraded_analyses` 再看比率。** 身分比對整段沒跑成的那幾次，所有計數都是 0，
   會把比率稀釋成看起來很健康。`degraded_analyses` 非零代表那段期間有分析根本沒算身分。
2. **`alias_hit_rate` 是主要的趨勢指標。** 它在爬代表 zone 邊界漂移在惡化、第一段
   （既有鏈命中）愈來愈接不住。單一數值沒有意義，要看走勢。
3. **`invariant_violations` 必須是 0。** 它與其他欄位語意不同——不是「比較差」而是
   不變式被違反，所以獨立成欄而不併進比率。非零要當 bug 查。

**母體是分析的子集**：統計只在 `reuse_existing=false` 那條路徑產生，`analyses` 不等於
該期間的所有分析。要算真正的比例得用 `analysis_id` join `stock_sr_zone_analyses`。

**未啟用時回 503**（repo 未注入）。

---

### GET `/watchlist`

取得監控清單。回傳會附帶 `stock_symbol` 主檔狀態；`exists=false` 代表該
watchlist symbol 不在目前股票主檔內，`is_listed=false` 代表曾在主檔但最近
一次 TWSE ISIN 同步已未出現。

**Response：**
```json
{
  "watchlist": [
    {
      "symbol": "2330",
      "name": "台積電",
      "sector": "半導體業",
      "watched": true,
      "stock_symbol": {
        "exists": true,
        "is_listed": true,
        "isin_code": "TW0002330008",
        "market": "上市",
        "security_type": "股票",
        "industry": "半導體業"
      }
    }
  ]
}
```

### POST `/watchlist`

新增股票至監控清單。`name` / `sector` 可省略；省略時後端會從
`stock_symbols` 補股票名稱與產業。若 symbol 不在主檔且未提供 `name`，回 400。

**Request Body：**
```json
{ "symbol": "2330" }
```

### POST `/watchlist/bulk`

批次新增股票（已存在的 symbol 會更新名稱與產業）。可傳完整 `items`，也可只傳
`symbols` 讓後端從股票主檔補資料。

**Request Body：**
```json
{
  "symbols": ["2330", "2454"],
  "items": [
    { "symbol": "2330", "name": "台積電", "sector": "半導體" },
    { "symbol": "2454", "name": "聯發科", "sector": "半導體" },
    { "symbol": "2317", "name": "鴻海",   "sector": "電子" }
  ]
}
```

**Response：**
```json
{ "added": 3, "failed": 0, "total": 3 }
```

### DELETE `/watchlist/:symbol`

從監控清單移除股票。

### PATCH `/watchlist/:symbol/watch`

設定或取消該股票的**即時監聽**（是否透過 WebSocket 推播）。監控清單本身可以
很大，但即時監聽刻意限制**同時最多 3 檔**（`store.MaxWatchedSymbols`），跟這
套系統「非高頻」的定位一致；前端只會對監聽中的股票送出 WebSocket 訂閱。

**Request Body：**
```json
{ "watched": true }
```

**Response（200）：**
```json
{ "symbol": "2330", "watched": true }
```

**錯誤：** 已有 3 檔在監聽時，再設定第 4 檔會回傳 `409 Conflict`：
```json
{ "error": "已達監聽上限（3 檔），請先取消其他股票的監聽" }
```

### GET `/scheduler/status`

回傳每個 `knownSchedulerJobs` 的最新一筆執行紀錄。**即使從未執行過也會回一列。**

#### 取數方式：每個 job 各取最新一筆（SQL 層），不是「取最近 N 筆再分組」

`GetStatus` 走 `JobRunRepo.GetLatestPerJob()`，由 SQL 直接回每個 `job_name` 的最新一筆：

```sql
ROW_NUMBER() OVER (PARTITION BY job_name ORDER BY started_at DESC, id DESC) = 1
```

**要分清楚兩層保證**：

* **repo 層**（`GetLatestPerJob`）回的是「**表裡有紀錄的** `job_name` 各一列」——
  沒跑過的 job 不會出現，空表就回 0 筆。它保證的是「每個 job 至多一列」，不是「一定 11 列」。
* **API 層**才固定回 11 列：`GetStatus` 遍歷 `knownSchedulerJobs`，把 repo 沒回到的
  補成 `never_run` / `disabled`（見下一節）。

所以**回傳筆數不隨 `job_runs` 累積多少資料而增加**——但查詢本身仍要處理保留期內的
資料，受限的是輸出量而不是掃描量。

**不能改回「取最近 N 筆再在記憶體分組」**（2026-08-25 修）：
`intraday` 每 5 分鐘寫一筆、09:00–13:30 一天就 55 筆，單日 `job_runs` 實測 62 筆。
舊版取最近 50 筆，於是**每天過了 13:30，06:30 的 `corporate_action_sync` 與 08:50 的
`pre_market` 全被擠出視窗**，狀態頁把當天早上剛跑完的 job 報成 `never_run` ＋ `stale`
——正是本頁反覆警告的那種「訓練使用者忽略 stale 旗標」，而且誤報方向是最不該錯的
「該跑卻沒跑」。放大 N 只是把撞牆時間往後推，watchlist 一擴就再度失效。

`ORDER BY` 帶 `id DESC` 有實際作用：`started_at` 精度到秒，手動觸發撞上排程時同一秒
可能有兩筆，沒有它狀態頁會在兩筆之間跳動。

查詢由 `idx_job_runs_job_name_started_at (job_name, started_at DESC)` 支撐
（migration 072，三個引擎同步）。window function 需要 PostgreSQL / MySQL 8.0+ /
SQLite 3.25+，三者都滿足——sqlite 走 `modernc.org/sqlite`，單元測試每次都會跑到這句。

**測試慣例：`jobRunRepoStub` 必須遵守 `limit` 與排序。** 舊 stub 的 `GetRecent`
忽略 limit 直接回傳全部餵進去的紀錄，而真實 repo 是 `ORDER BY started_at DESC LIMIT ?`
——「視窗放不下」這一整類問題在測試裡永遠不會出現，這個誤報因此活了很久。
stub 偏離真實 repo 的語意時，測試保護的是一個不存在的系統。

#### `job_runs` 保留 30 天

`runPreMarket` 每天開盤前刪掉 `jobRunRetentionDays`（30 天）以前的紀錄
（`scheduler.go` 的 `jobRunRetentionCutoff()`）。

**原本是「只保留當天」**，那讓排程健康史每天歸零：「昨天那輪是不是 `partial`」
「這支這週失敗過幾次」隔天就查不到，而 2026-08-24 那次 `corporate_action_sync`
的狀態修正（原記於 `issue.md` I-084，已收斂）正是靠 `job_runs` 的
`failed=808` 發現的（2026-08-25 改）。

30 天約 1900 筆，對三個引擎都微不足道。`/scheduler/status` 的**回傳筆數**不受保留期
影響（見上節），但那條 window query 仍要處理保留期內的資料——真的把保留期拉到很長時，
要重新評估的是掃描成本，不是回傳量。要調整時 `DeleteBefore` 的呼叫點只有一處。

#### `status` 的三種「沒有執行紀錄」情形

| `status` | `stale` | 意義 |
|---|---|---|
| `disabled` | `false` | **排程沒有被註冊**——config 關閉（`sr_evaluation`、`evaluation_universe`）或相依未注入（`adjuster`、`stockSyncer`）。刻意沒開，不是異常 |
| `never_run` | `true` | 已註冊但從未跑過——**該跑卻沒跑**，要查 |
| 實際狀態（`success` / `partial` / `failed` / `aborted`） | 依 `jobStaleThreshold` | 跑過；`stale` **只在仍註冊時才計算** |
| `running` | 恆為 `false` | 正在跑。**`stale` 對 `running` 永遠不成立**，不套門檻，見下節 |

**為什麼要分開**（2026-08-18 修正）：早期版本一律回 `never_run` ＋ `stale=true`，
於是兩個預設關閉的排程（`sr_evaluation`、`evaluation_universe_sync`）常態顯示成 stale。
**那會訓練使用者忽略這個旗標——真的有 job 卡住時反而看不出來。**

註冊與否由 scheduler 自己回報（`Scheduler.IsJobRegistered`），API 層**不重算 config 條件**，
避免同一個判斷散在兩處而不一致。

**cron 字串打錯導致註冊失敗時也回 `disabled`**——`AddFunc` 出錯只記 log 不中止，
行為上與「沒開」相同（都不會跑），成因要看啟動 log 的 `cron register failed`。

「跑過、後來被關閉」的 job 保留實際狀態，但 `stale` 為 `false`——舊紀錄不代表排程卡住。

#### `running` 不套 stale 門檻，孤兒紀錄由啟動時回收

`stale` 的判定明確排除 `running`：

```go
stale := registered && r.Status != "running" && time.Since(r.StartedAt) > jobStaleThreshold[name]
```

排除本身是對的——**真的在跑的 job 不該被報成卡住**，而 `evaluation_universe_sync`
一輪就要約 27 分鐘。但它的代價是：一筆停在 `running` 的紀錄**永遠不會被標成 stale**，
不是「80 小時後才亮」，是不會亮。

所以 `running` 應該在資料上只代表「真的正在跑」。process 被外力砍掉時
（部署重啟、OOM kill、crash）`finishRun` 沒機會執行，紀錄會留在 `running`，
於是 **`main.go` 在啟動時、`sched.Start()` 與 `srv.Run()` 之前**呼叫一次
`JobRunRepo.AbortRunning()`，把殘留的 `running` 改寫成 `aborted`
（2026-08-25 加，原記於 `issue.md` I-090，已收斂）。

> ⚠️ **這是 best-effort，不是不變式。** 回收帶 10 秒 timeout，**失敗只記 Error log
> 並繼續啟動**（`cmd/server/main.go`）——DB 當下不通或那句 UPDATE 逾時的話，孤兒會
> 原封不動留在 `running`，而它**永遠不會被標成 stale**（見上），排程頁照樣顯示「執行中」。
>
> 所以「畫面上是 `running`」只能推出「**要嘛真的在跑，要嘛上次回收失敗過**」。
> 要斷定是哪一種，去看啟動 log：
>
> * `回收上次未跑完的排程紀錄`（Warn，帶 `aborted` 筆數）＝ 有回收到東西，正常。
> * `回收孤兒 job_runs 失敗，被中斷的排程可能仍顯示為執行中`（**Error**）＝ 這次沒收成功，
>   畫面上的 `running` 不可信。
> * 兩者都沒有 ＝ 上次關機時沒有 job 在跑，畫面上的 `running` 就是真的在跑。
>
> **刻意不讓回收失敗中止啟動**：它只影響狀態頁的顯示，不影響任何排程執行，
> 為了一個顯示問題而讓整個 backend 起不來是更糟的交換。代價就是上面這條判讀規則。

實例：2026-08-25 16:00 的 `evaluation_universe_sync` 跑到 135 檔中的第 54 檔時
遇到重新部署，那筆紀錄停在 `running`、`finished_at` 為空，狀態頁一路顯示「執行中」
直到隔天 16:00 的下一輪產生新紀錄才被蓋掉——**中間約 24 小時，排程頁在主動報平安。**

**這與 `finishRun` 的 ctx 修法是同一個病灶的兩個入口**：2026-08-24 那次
（原記於 `issue.md` I-084）堵的是「ctx 逾時之後寫不進 `job_runs`」，
啟動回收堵的是「連寫的機會都沒有」。少任何一個，`running` 都會有假陽性。

**為什麼不在 shutdown handler 收尾**：`main.go` 的訊號處理只收 SIGTERM，
涵蓋不到 SIGKILL、OOM kill 與 crash；而 `docker compose down` 預設 10 秒後就把
SIGTERM 升級成 SIGKILL，一輪 27 分鐘的 job 等不到自己收尾。**啟動時回收才是所有
中斷路徑的共同出口。** 兩者可以並存，但單靠 shutdown 一定會漏。

⚠️ **前提：同一個 DB 只有一個 backend 實例。** `AbortRunning` 不分實例，
第二台 backend 啟動會把第一台**正在跑**的 job 一起標成 `aborted`。
要橫向擴充 backend 之前，這裡必須先加實例識別。

#### `aborted`：沒跑完，結果未知

| 狀態 | 語意 | 該怎麼辦 |
|---|---|---|
| `failed` | 跑了，但名單內全軍覆沒（`failed >= total`） | 查資料源或上游 |
| `aborted` | **沒跑完**——process 在結束前就被中斷 | 看該 job 是否需要補跑 |

`aborted` 刻意不併進 `failed`：已完成的部分仍然有效（前述那次是 135 檔中的 54 檔），
混用會讓「要不要重跑」失去判斷依據。

`aborted` 的紀錄 `symbols_total` / `symbols_failed` **維持 0**——那兩個數字只在
`finishRun` 寫入，沒跑完就沒有回報過，0 正確表達「未知」。
真正跑了多少要看 log 的進度行（`evaluation universe sync progress` 等）。

**狀態字串不得超過 10 字元**，但**三個引擎並不相同**（2026-08-25 review 更正）：

| 引擎 | `job_runs.status` |
|---|---|
| postgres | `VARCHAR(10)` |
| mysql | `VARCHAR(10)` |
| sqlite | **`TEXT`——不限長度** |

三者都沒有 CHECK 約束擋錯字。`interrupted` 是 11 字元，會踩到 2026-08-11
`corporate_action_sync` 撞 `VARCHAR(20)` 的同一顆雷，所以用 `aborted`（7 字元）。

**這個上限不能靠單元測試守**：測試只跑 sqlite（見 `issue.md` I-054），而 sqlite 是
`TEXT`，超長的值照樣寫得進去也讀得回來——「寫進去讀回來一樣」在那裡證明不了任何事，
真正會爆的 postgres 與 mysql 反而沒有 repo 層測試。所以改由 `store/job_run_repo.go`
的**編譯期斷言**把守（`const _ = uint(jobRunStatusMaxLen - len(...))`，超長時
`go vet` / build 直接失敗）。日後新增狀態值時比照加一行。

#### `symbols_total` / `symbols_failed` 的單位是**標的數**，`status` 由它們推導

**`symbols_total` 的分母一律是「本輪該做的清單有多大」，跳過的不扣**（2026-08-26 起統一，
原記於 `issue.md` I-092）。三支有「跳過」概念的排程共用這個通則：

| Job | 分母 | 跳過／未處理怎麼算 |
|---|---|---|
| `corporate_action_sync` | 當日名單大小 | **併進 `symbols_failed`**——那邊的「沒輪到」是**逾時導致該做而沒做** |
| `evaluation_universe_sync` | 池大小（135） | 不計入失敗，只記進 log 的 `skipped` |
| `candle_gap_detection` | 本輪有效池大小 | `symbols_failed` **固定 0**；缺漏與不可用一律走 `degraded`，見下方 `partial` 的第四種成因 |
| `sr_analysis` / `sr_analysis_chip` | watchlist 大小（11） | 同上 |

**分母不能換成「實際處理數」**：那會讓狀態頁的數字每天浮動（11 / 10 / 11 …），
而浮動的原因在畫面上看不到。`sr_analysis` 2026-08-26 之前就是這樣——同一輪裡
log 印 `total=11`、`job_runs` 記 `10`，兩個都叫 total 卻是不同的值。

**第三欄刻意不統一**：「逾時沒輪到」與「判定後確認不需要做」語意不同，前者該算失敗、
後者不該。要看實際跑了多少，看各 job 完成 log 的 `skipped` 欄位。

⚠️ **`sr_analysis` 的歷史資料有一個階梯**：2026-08-26 之前的列是「扣掉跳過後的處理數」，
之後是 watchlist 大小，**不回填**。跨這個時點比較該 job 的 `symbols_total` 會看到跳變。

**這次統一唯一會改到的狀態字串**：`sr_analysis` 在「多數標的被跳過、剩下的全部失敗」時，
分母變大讓 `failed >= total` 不再成立，狀態從 `failed` 變成 `partial`。
`partial` 才是對的——那輪確實有跑，只是跑得不完整，而被跳過的並不是失敗。

`Scheduler.finishRun` 依這兩個數字換算 `status`：`failed >= total`（且 `total > 0`）記 `failed`、
`failed > 0` 記 `partial`、其餘記 `success`。因此**呼叫端一定要傳標的數，不能傳事件筆數**——
單位混用會讓比較失去意義（`corporate_action_sync` 早期就是傳事件筆數，2026-08-24 修）。

「跑到一半被逾時砍斷」也算失敗：`corporate_action_sync` 的 `symbols_failed` 是
**實際失敗檔數 ＋ 因逾時沒輪到的檔數**（「實際失敗」涵蓋**抓取、寫入、還原係數重算**
三個階段——重算失敗代表事件進了資料庫但 K 棒的 `adj_factor` 沒跟上，那檔的價格是不一致的，
所以不能只當成一行 log），`error` 欄會帶 `context deadline exceeded`
或「N 檔未處理」。少了未處理那一項，逾時的那輪會因為「零失敗」被記成 `success`，
看起來一切正常，實際只跑了一小部分。

**但 `error` 欄帶逾時訊息的條件很窄：必須真的有標的是在預算到期之後才失敗。**
先前某檔因為一般 API 錯誤失敗、最後一檔跑完之後預算才到期的那種組合，
`error` 欄會是**空的**（`symbols_failed` 仍為 1，個別標的的失敗原因只進 log）——
這是刻意的，用整輪的 `failed > 0` 去推斷逾時會把資料源錯誤誤標成預算不足，
讓人去調 `timeout_sec` / `shard_count` 而不是查資料源。
所以讀 `error` 欄時：**有逾時訊息＝真的撞到預算；空白＋`symbols_failed > 0`＝去看 log 的個別失敗。**

**`partial` 還有第二種成因：名單本身不完整。** `corporate_action_sync` 讀不到 watchlist 時
會**降級成只跑當日分片**（不整輪放棄），此時 watchlist 那批根本沒進名單，
不可能被算進 `symbols_failed`——純看數字必然推導出 `success`。所以這種情況由
`finishRunDegraded` 的 `degraded` 旗標強制記成 `partial`，`error` 欄會帶
「列出 watchlist 失敗: …」。讀 `partial` 的正確語意是**「這輪跑得不完整」**，
涵蓋「名單內有標的失敗」「逾時沒跑完」「名單本身就少了一批」三種，
`symbols_failed = 0` 的 `partial` 就是第三種。

**`candle_gap_detection` 為 `partial` 加了第四種成因：結論本身不完整或是壞消息**
（原記於 `issue.md` I-091）。它的 `symbols_failed` **固定為 0**——缺漏與驗證不可用都不是
「標的失敗」（那些標的的回補本身是成功的，上游回什麼就寫什麼），所以一律走 `degraded`：

| `error` 前綴 | 意義 |
|---|---|
| `upstream_data_gap: …` | **驗過了，而且真的缺**——交易所那天有成交、我們沒有 K 棒 |
| `verification_unavailable: …` | **驗不了**——對照源失敗／格式變動／回應歸屬對不上／breaker 開啟／主檔查不到 |
| `verification_state_read_failed` / `verification_state_write_failed` | 公平排序簿記讀寫失敗，該輪的排序可能停滯 |

⚠️ **`gap` 與 `unavailable` 不是同一件事的輕重**：前者是「驗成功了，結論是壞消息」，
後者是「根本沒驗成」。把驗不了記成驗過了，這個機制就會在最需要它的時候靜默失效——
那正是它要消滅的問題形狀。

`error` 欄一輪可能同時記到兩件事（例如降級 ＋ 逾時），以 `; ` 串接。
`candle_gap_detection` 同理：缺口、驗證不可用與 breaker 跳過會一起出現，**互不覆蓋**。

#### 「整輪沒開始跑」記 `failed`，「沒有東西要跑」記 `success`

兩者的實際標的數都是 0，分界**看輸入的查詢有沒有失敗，不是看 `symbols_total` 是不是 0**：

| 情境 | 傳給 `finishRun` | `status` |
|---|---|---|
| 取不到輸入（watchlist、待驗清單、標的清單查詢失敗） | `(1, 1, err)` | `failed` |
| 輸入拿得到但是空的（watchlist 還沒加股票、沒有待驗分析） | `(0, 0, "")` | `success` |

第一種**刻意傳 `(1, 1)` 而不是真實的 0**：`finishRun` 只在 `total > 0 && failed >= total`
時判 `failed`，所以 `total=0` 會讓「整輪連輸入都沒拿到」落到 `success`（傳 `(0,0)`）
或 `partial`（傳 `(0,1)`）——兩者都不誠實，那輪一檔都沒處理。
`corporate_action_sync`、`sr_zone_verify`、`sr_analysis` / `sr_analysis_chip`
四支都適用這條（2026-08-24）。

⚠️ **`candle_gap_detection` 刻意不套這條**（原記於 `issue.md` I-091）：它取不到候選清單時
記 **`partial`（`degraded`）而不是 `failed`**。理由是這支的產出是「結論」不是「處理量」——
`failed` 的語意是「這輪整個沒跑起來」，而「拿不到清單所以驗不了」要落在**與其他驗不了
情境同一個桶**（`verification_unavailable`），否則讀的人得分兩個地方看同一類問題。

**「讀不到 watchlist」在不同 job 有不同的正確答案，不要互相套用**：
`corporate_action_sync` 讀不到時仍會跑當日分片，記 `partial` 是對的——真的跑了一批，
只是名單不完整；`sr_analysis` / `sr_analysis_chip` 的標的來源**只有** watchlist，
讀不到就等於整輪沒有輸入，那是 `failed`。判準始終是**那一輪到底有沒有處理任何標的**。

**不要把它簡化成「`total=0` 一律 `failed`」**——那會誤傷第二種合法的零標的輪。
`scheduler_test.go` 的 `TestSRZoneVerifySucceedsOnEmptyList` 與
`TestSRAnalysisSucceedsWhenWatchlistEmpty` 兩條對照組釘住這個分界；變異測試實測：
把判定改成「`total=0` 一律 `failed`」時**恰好只有那兩條 fail**。

#### 逾時的 job 不會卡在 `running`

`finishRun` **不沿用 job 自己的 ctx**，而是用 `context.WithoutCancel` 切斷取消訊號、
另外套一個 10 秒的寫入預算。job 的 ctx 逾時之後，用它去寫 `job_runs` 一定會失敗，
那筆紀錄就會**永遠停在 `running`**——看起來像還在跑，實際早就結束了（2026-08-24 修）。這條對**所有 job** 都成立，不限公司行動同步。

所以 `running` 現在可以照字面解讀：真的還在跑，或行程在寫回之前就被砍掉。

#### `stale` 門檻寫死在程式裡，不隨 cron 調整

`jobStaleThreshold` 是每個 job 各自的常數（`handler/scheduler.go`），**不會依 config 的 cron 重算**。
四支 cron 走 config 的排程都受影響：

| job | 門檻 | 隱含假設 |
|---|---|---|
| `chip_daily_sync` | 72 小時 | 每日 |
| `sr_evaluation` | 72 小時 | 每日 |
| `corporate_action_sync` | 80 小時 | 平日每日（跨週末最長週五→週一） |
| `evaluation_universe_sync` | 80 小時 | 平日每日 |
| `candle_gap_detection` | 80 小時 | 跟著 `evaluation_universe_sync` 那輪跑，所以門檻相同 |

**把 cron 設得比門檻稀疏會讓該 job 永遠顯示 stale**（例如改成每週一跑，間隔 168 小時 > 80），
即使它完全照設定執行。這正是本頁上面警告的那種「訓練使用者忽略 stale 旗標」。
目前的做法是**不要把這幾支設成稀疏排程**；真的需要時，正解是讓門檻從 cron 推導
（例如取下兩次觸發間隔再加緩衝），那是還沒做的改造。

### POST `/scheduler/stock-symbol-sync/run`

手動觸發 TWSE ISIN 股票主檔同步，與每日 `stock_symbol_sync` 排程共用同一份邏輯。
此端點會立即回應，實際同步在背景執行；進度與結果可透過 `GET /scheduler/status`
查看 `stock_symbol_sync` 這個 job。

**Response（202 Accepted）：**
```json
{ "message": "stock_symbol_sync 已在背景重新觸發" }
```

### POST `/scheduler/corporate-action-sync/run`

手動觸發公司行動同步（分割 ＋ 除權息）與股價還原係數重算，與每日 `corporate_action_sync`
排程（`corporate_action.cron`，**預設**平日 06:30）共用同一份邏輯。立即回應，
實際執行在背景；結果查 `GET /scheduler/status` 的 `corporate_action_sync`。

**為什麼需要這個入口**：排程一天只跑一次，部署若晚於排程時間，沒有它就得等到隔天才驗得了
還原是否正確（驗證方式見 `scripts/verify-adjustment.sh`）。**重算是冪等的**，
重複觸發不會累積誤差——`adj_factor` 是事件表的純函數，每次都整段覆寫。

執行內容：

1. 一次批次請求抓全市場的分割／反分割／面額變更（FinMind `TaiwanStockSplitPrice`）。
2. 逐檔抓除權息（Yahoo `dividendsByYear`）與減資（FinMind），標的來源是
   **`candles` 內所有相異 symbol**，不是 watchlist。
3. 重算受影響標的的 `adj_factor`（價）與 `vol_factor`（量）。

**手動觸發跑的是「當天那一份名單」，不是全市場**（2026-08-24 起）：逐檔那一步是
**watchlist 全量 ＋ 其餘標的的當日分片**（預設 5 片，週一到週五各一片）。所以連按兩次
不會多涵蓋任何標的——同一天算出來的片號一樣。要臨時全量補跑，把
`corporate_action.shard_count` 設成 1 再觸發。分片規則見
[`architecture.md`](./architecture.md)「公司行動同步是唯一『watchlist 優先、其餘輪替』的排程」。

**Response（202 Accepted）：**
```json
{ "message": "corporate_action_sync 已在背景重新觸發" }
```

---

### POST `/scheduler/evaluation-universe-sync/run`

手動觸發評估標的池的日 K 維護，與 `evaluation_universe_sync` 排程（平日 16:00）共用邏輯。
立即回應，實際執行在背景；結果查 `GET /scheduler/status` 的 `evaluation_universe_sync`。

**為什麼需要這個入口**：該排程**預設關閉**（一次約 131 個 FinMind 請求、26 分鐘），
而跑 evaluation 之前必須先把池的尾端對齊——在排程開啟前這是唯一的對齊方式。
不對齊的後果：各檔最後交易日相差 1～3 天，evaluation 取「最後 N 根」會讓評估視窗錯開，
同一份報告隔幾天重跑得到不同結果，且分不清是策略變了還是資料窗變了。

**重複觸發會被擋掉**（scheduler 內的行程旗標），不會有兩批請求互搶 5 req/min 的節流器。

**Response（202 Accepted）：**
```json
{ "message": "evaluation_universe_sync 已在背景重新觸發" }
```

---

### POST `/scheduler/sr-analysis/run`

手動觸發「對 watchlist 逐檔產生 SR zone 分析」，與 `sr_analysis` / `sr_analysis_chip`
排程共用邏輯。立即回應、背景執行，結果查 `GET /scheduler/status`。

**Query：** `with_chip=true` 走含當日籌碼那一輪（`sr_analysis_chip`），省略則是不含的那輪。

**為什麼一天兩輪**：SR 分析吃籌碼（`trading_score` 的 Chip 佔 15%），而 FinMind 的法人／
融資券要晚間才發布。`sr_analysis`（平日 17:00）拿到的是**前一日**籌碼，
`sr_analysis_chip`（平日 22:00，晚於 21:00 的籌碼採集）才有當日的。兩輪站在同一根 K 棒上，
只有籌碼不同。

**跳過不是失敗**，每檔各自判斷，`job_runs.symbols_total` 會把跳過的扣掉：

| 情況 | 適用 | 原因 |
|---|---|---|
| 最新 K 棒不是今天 | 兩輪 | 假日、停牌，或 `daily_close` 還沒跑完 |
| 已分析過今日 K 棒 | 僅 17:00 那輪 | 同一根 K 棒不必重算 |
| **當日籌碼尚未入庫** | 僅 22:00 那輪 | 21:00 的籌碼採集失敗或還沒寫完時，這一輪算出來的東西會與 17:00 那輪相同 |
| 已用今日籌碼分析過 | 僅 22:00 那輪 | 再算一次結果相同，還會多推一次 zone 身分的缺席計數 |
| 沒有任何 K 棒 / 沒有任何籌碼資料 | 依上面兩類 | 這檔還沒被採集過（新標的，或來源沒有收錄）。**不是錯誤**，不記 warn |
| 查不到 K 棒 / 籌碼查詢失敗 | 依上面兩類 | 查詢本身失敗。記 warn，該檔跳過但不影響其餘標的 |

**判斷帶 `timeframe`**：跳過與否只看同一個 `timeframe` 的分析。手動跑過的 5m 分析不會
擋掉 1d 的排程。

**為什麼需要這個入口**：兩輪都**預設關閉**（`sr_analysis.enabled`），而 decision replay
的驗證母體得先有辦法補跑；排程漏跑時也只有這裡能補。重複觸發由行程旗標擋掉，
**兩輪各自一個**——17:00 那輪還在跑不會擋掉 22:00。

**Response（202 Accepted）：**
```json
{ "message": "sr_analysis 已在背景觸發" }
```

---

## Evaluation Universe API

評估標的池（T-040 Step 5）。**這個池不是 watchlist**：它只驅動每日盤後的日 K 維護，
不進盤中掃描、籌碼同步、signal 或 production SR 分析——那是「新標的不能放進 `watchlists`」
的核心約束（把 131 檔塞進 watchlist 會讓六個流程各乘上約 12 倍）。
選取規格見 [`evaluation-universe-selection-plan.md`](./evaluation-universe-selection-plan.md)。

### GET `/evaluation-universe`

| 參數 | 說明 |
|---|---|
| `active` | `true` 時只回仍納入每日維護的成員。**預設回全部**（含停用者）——入退池歷史本身是研究紀錄 |

**Response：**
```json
{
  "items": [
    {
      "id": 1, "symbol": "2330", "bucket_hint": "LOW_VOLATILITY",
      "bucket_edge_low": 0.046089927430152715,
      "bucket_edge_high": 0.06278197721225691,
      "universe_version": "v2", "universe_role": "primary",
      "selected_at": "2026-08-17T00:00:00+08:00",
      "source": "T-040_STEP3", "active": true, "note": ""
    }
  ],
  "total": 131,
  "active_count": 131,
  "active_buckets": { "LOW_VOLATILITY": 53, "NORMAL_VOLATILITY": 46, "HIGH_VOLATILITY": 32 }
}
```

`bucket_edge_low` / `bucket_edge_high` 是**入池時實際使用的分位數邊界**，每一列都存。
`bucket_hint` 單獨存在無法回答「這個 bucket 是用哪組邊界判的」——實測 2026-08-17 有 3 檔
`atr_pct` 完全未變卻換桶，只因母體變了、邊界移動。

`active_buckets` **只統計 active 成員**：停用的標的不再進 evaluation，算進去會高估樣本量。
三個 bucket 是否都還有足夠樣本，直接決定 T-003 的 sweep 有沒有意義。

### POST `/evaluation-universe`

匯入（或更新）選池成員，以 `symbol` 為鍵 upsert。

**Request：**
```json
{
  "items": [
    {
      "symbol": "2330", "bucket_hint": "LOW_VOLATILITY",
      "bucket_edge_low": 0.046089927430152715,
      "bucket_edge_high": 0.06278197721225691,
      "universe_version": "v2", "universe_role": "primary",
      "source": "T-040_STEP3", "note": ""
    }
  ]
}
```

必填：`symbol`、`bucket_hint`、`bucket_edge_low`、`bucket_edge_high`、`universe_version`、
`source`。`universe_role` 省略時為 `primary`。

**刻意不接受 `active`**：入退池是獨立的人工決定，不該被一次重新匯入靜默覆寫。要改用 `PATCH`。
**`selected_at` 由伺服器決定**：讓呼叫端指定會讓「何時入池」變成可偽造的欄位，
而它是研究紀錄的一部分。

400 的情形：`items` 為空、缺必填欄位、`bucket_edge_high <= bucket_edge_low`、
以及**同一次請求內有重複的 symbol**（它們會在同一個 transaction 內互相覆蓋，
留下哪一筆取決於順序，那是靜默的資料遺失）。

**Response：** `{ "upserted": 131 }`

### PATCH `/evaluation-universe/:symbol`

切換該標的是否納入每日日 K 維護。目前只支援 `active`。

**Request：** `{ "active": false }`

`active` 為**必填**（後端用指標型別）：缺欄位與 `false` 必須分得開，
否則漏帶欄位會被當成「停用」。標的不在池內時回 **404**。

**Response：** `{ "symbol": "2330", "active": false }`

---

## Market API

### POST `/market/backfill`

觸發歷史 K 棒資料補撈，立即建立 `market_backfill_jobs` 紀錄並背景執行，回傳 job 供輪詢
（形狀與 `POST /chips/sync` 一致）。

**`symbols` 為必填**——空陣列或缺鍵一律回 `400 symbols is required`。API 層不認識 watchlist：
要回補哪些股票由呼叫端明講，因此這支端點也可用於 watchlist 以外的標的。前端「歷史資料回補」
頁面留空時代入整個監控清單，那是**前端的語法糖，不是 API 行為**。`days` 預設 120。

**Request Body：**
```json
{ "days": 120, "symbols": ["2330", "2454"] }
```

**Response（202 Accepted）：**
```json
{
  "job": {
    "job_id": "bf_20260807_120000_000",
    "symbols": "[\"2330\",\"2454\"]",
    "days": 120,
    "status": "pending",
    "symbols_total": 2,
    "symbols_done": 0,
    "symbols_failed": 0,
    "failures": []
  }
}
```

`symbols` 在 job 上是 JSON 陣列**字串**（DB 直存），`failures` 則是物件陣列。
`job_id` 格式為 `bf_<UTC 時間戳到毫秒>_<4 位隨機碼>`；隨機碼是必要的，只有時間戳時
同一毫秒進來的兩個請求會產生相同 id 而撞上 UNIQUE constraint。
（`POST /chips/sync` 的 `chip_` 前綴同此格式。）

### GET `/market/backfill/:job_id`

查詢股價回補任務進度。每檔回補完成（成功或失敗）就更新一次，所以進度是逐檔推進的。

**Response：**
```json
{
  "job": {
    "job_id": "bf_20260807_120000_000",
    "status": "partial",
    "symbols_total": 2,
    "symbols_done": 2,
    "symbols_failed": 1,
    "failures": [{ "symbol": "2454", "error": "finmind 429" }],
    "error": "some symbols failed"
  }
}
```

`status` 可為 `pending`、`running`、`done`、`partial`（部分失敗）、`failed`。
`failed` 涵蓋兩種情況：所有 symbol 都失敗（`error` 為 `all symbols failed`），
或背景執行時發生 panic 被攔下（`error` 為 `internal error`）——後者的用意是讓輪詢
能收斂到終態，而不是讓任務永遠停在 `running`。

**錯誤語意**：找不到 job 回 `404`；查詢過程發生其他錯誤（DB 連線中斷等）回 `500`。
這兩者刻意分開——先前所有 repo 錯誤都被當成 `404`，DB 掛掉時呼叫端看到的是
「任務不存在」，會誤以為任務被清掉了。`GET /chips/sync/:job_id` 同此語意。

> 請求頻率依 `finmind.rate_limit`（每分鐘請求數，`config.yaml`）節流，非固定間隔；
> 20 檔約 4 分鐘、650 檔約 2.2 小時。前端「歷史資料回補」頁面（`/backfill`）提供代號
> 輸入框（逗號或空白分隔）與每 3 秒輪詢的進度顯示。
>
> **前端不設固定逾時**：長任務本來就會跑數小時，固定上限會把正常任務誤判成卡住。
> 改用停滯偵測——連續 5 分鐘 `symbols_done` 沒有推進才停止追蹤並解鎖送出按鈕，
> 且訊息明講「後端可能仍在執行」而不謊稱失敗（backend 重啟不會接手既有任務，
> 那種情況下 job 會永遠停在 `running`）。籌碼同步的輪詢同此設計。

> **相容性提醒（2026-08-07 變更）**：本端點原本回 `{message, symbols, days}` 且 `symbols`
> 省略時自動代入 watchlist。現已改為回 `{job}` 且 `symbols` 必填，兩者皆為 breaking change，
> 手動 curl 這支端點的腳本要一併調整。`GET /market/backfill/:job_id` 屬相容新增。

---

## Backtest API

回測是**非同步 job 模式**：`POST` 送出後立即回傳 `pending` 狀態的 job，實際計算
由 Python worker（輪詢）或 HTTP server（即時推播）在背景執行，需要輪詢
`GET /backtest/:job_id` 直到 `status` 變成 `done`/`failed` 才會有 `result`。
前端「策略回測」頁面（`/backtest`）已內建每 5 秒輪詢的邏輯。

### POST `/backtest`

提交回測任務。

**Request Body：**
```json
{
  "strategy": "breakout_v1",
  "symbols": ["2330", "2454"],
  "timeframe": "1d",
  "start_date": "2023-01-01",
  "end_date": "2024-12-31",
  "use_chip_filter": false,
  "chip_min_score": 0
}
```

`use_chip_filter` / `chip_min_score` 為選填。啟用後只對模組化策略生效，Python 端會用
`chip_scores.total_score` 逐 bar 過濾進場訊號；缺少該日籌碼資料時視為中性 `0`。
legacy `breakout_v1` 收到這兩個欄位時會忽略並記 warning log，不中斷回測。

`strategy` 可用值：

| 值 | 引擎 | 說明 |
|----|------|------|
| `breakout_v1` | backtrader | 與 Go signal engine 1:1 對齊的既有策略 |
| `breakout_swing_atr_v1` | 模組化（純 pandas/numpy） | Swing High/Low 支撐壓力 + 突破進場 + ATR 停損 |
| `breakout_volprofile_composite_v1` | 模組化 | Volume Profile 支撐壓力 + 突破進場 + 複合停損 |
| `pullback_atrchannel_structural_v1` | 模組化 | ATR 通道支撐壓力 + 回測支撐進場 + 結構停損 |
| `pullback_swing_composite_v1` | 模組化 | Swing High/Low 支撐壓力 + 回測支撐進場 + 複合停損 |

模組化策略的完整數學定義見 [backtest-modular-strategy.md](./backtest-modular-strategy.md)。

**Response（201 Created）：**
```json
{
  "job": {
    "job_id": "bt_20260115_103000_000",
    "type": "backtest",
    "strategy": "breakout_swing_atr_v1",
    "symbols": "[\"2330\",\"2454\"]",
    "timeframe": "1d",
    "start_date": "2023-01-01",
    "end_date": "2024-12-31",
    "use_chip_filter": false,
    "chip_min_score": 0,
    "status": "pending",
    "trigger": "manual",
    "created_at": "2026-01-15T10:30:00+08:00"
  }
}
```

### GET `/backtest`

列出所有回測任務（依 `created_at` 由新到舊）。

**Query Parameters：** `limit`（預設 20，最多 200）

**Response：**
```json
{ "jobs": [ { "job_id": "...", "status": "done", "...": "..." } ], "total": 1 }
```

### GET `/backtest/:job_id`

取得特定回測任務狀態與結果；`result` 在任務未完成時為 `null`。

**Response（完成後）：**
```json
{
  "job": { "job_id": "bt_20260115_103000_000", "status": "done", "...": "..." },
  "result": {
    "job_id": "bt_20260115_103000_000",
    "strategy": "breakout_swing_atr_v1",
    "total_return": 0.182,
    "annual_return": 0.091,
    "win_rate": 0.62,
    "max_drawdown": 0.083,
    "sharpe_ratio": 1.42,
    "total_trades": 24,
    "win_trades": 15,
    "loss_trades": 9,
    "avg_pnl": 3250.5
  }
}
```

### GET `/backtest/:job_id/trades`

取得回測每筆交易明細。

**Response：**
```json
{
  "job_id": "bt_20260115_103000_000",
  "trades": [
    {
      "symbol": "2330", "direction": "BUY",
      "entry_time": "2023-03-01T00:00:00+08:00", "exit_time": "2023-03-10T00:00:00+08:00",
      "entry_price": 550.0, "exit_price": 570.0,
      "size": 1818.18, "pnl": 34500.0, "pnl_pct": 0.0345, "commission": 1560.2
    }
  ],
  "total": 1
}
```

### DELETE `/backtest/:job_id`

取消回測任務，**只能取消 `pending` 狀態**（已開始執行的無法取消，`409 Conflict`）。

---

## Stock Analysis API

針對單一個股，用歷史 OHLCV 算出支撐/壓力/進場/停損/停利，供人工判斷用
（不是自動下單訊號）。實際計算由 Python 完成（重用
[backtest-modular-strategy.md](./backtest-modular-strategy.md) 的模組化元件），
**需要 `python.service_url` 已設定且 Python HTTP service 已啟動**，否則
`POST /analysis` 會回傳 `502 Bad Gateway`。驗證（`POST /analysis/:id/verify`）
不依賴 Python，純粹比對 Go 這邊的 `candles` 表，Python 沒開也能用。
完整規格見 [stock-analysis.md](./stock-analysis.md)。

### POST `/analysis`

觸發一次分析並寫入 DB。

**Request Body：**
```json
{ "symbol": "2330", "timeframe": "1d" }
```

`timeframe` 省略時預設 `1d`。

**Response（201 Created）：**
```json
{
  "analysis": {
    "id": 1,
    "symbol": "2330",
    "timeframe": "1d",
    "analyzed_at": "2026-07-01T00:00:00+08:00",
    "current_price": 978.0,
    "trend": "BULLISH",
    "entry_status": "WATCHING",
    "entry_direction": "LONG",
    "entry_price": 985.0,
    "entry_reason": "等待突破壓力 985.00（來源：swing）",
    "stop_loss_atr": 960.2,
    "stop_loss_structural": 965.0,
    "stop_loss_composite": 965.0,
    "take_profit_next_level": 1020.0,
    "take_profit_risk_reward": 1025.0,
    "take_profit_atr": 1030.5,
    "trade_verification": null,
    "verified_at": null,
    "created_at": "2026-07-01T10:00:00+08:00"
  },
  "levels": [
    { "id": 1, "analysis_id": 1, "price": 985.0, "type": "RESISTANCE", "strength": 1.0, "method": "swing", "status": "PENDING" },
    { "id": 2, "analysis_id": 1, "price": 955.0, "type": "SUPPORT", "strength": 0.9, "method": "volume_profile_poc", "status": "PENDING" }
  ]
}
```

### GET `/analysis`

列出歷史分析紀錄。

**Query Parameters：** `symbol`（篩選特定股票）、`limit`（預設 20，最多 200）

### GET `/analysis/:id`

取得單筆分析詳情（含支撐/壓力清單），格式同 `POST /analysis` 的回應。

### POST `/analysis/:id/verify`

手動重新驗證：比對這筆分析之後的實際 K 棒，更新每個支撐/壓力位的
`status`（是否被突破），以及（若 `entry_status=ACTIVE`）三種停損/三種停利
各自有沒有被觸及。**可重複呼叫**，每次都用目前為止最新的資料重新計算，
不是一次性判定。沒有自動排程，需要主動呼叫這支 API 才會更新。

**Response：** 格式同 `GET /analysis/:id`，但 `trade_verification` 會有值：
```json
{
  "analysis": { "...": "...", "trade_verification": "{\"applicable\":true,\"stop_loss\":{...},\"take_profit\":{...}}", "verified_at": "2026-07-05T09:00:00+08:00" },
  "levels": [ { "...": "...", "status": "BROKEN", "broken_at": "2026-07-03T00:00:00+08:00", "broken_price": 950.0 } ]
}
```

### DELETE `/analysis/:id`

刪除一筆分析紀錄（連同其支撐/壓力位一併刪除）。前端「個股分析」頁面的
歷史紀錄列表提供刪除按鈕（會先跳出確認列，比照監控清單的刪除確認方式）。

---

## SR Zone Scoring API

機構級支撐/壓力機率評分——輸出**價格區間（zone）**而非單一價位，每個 zone
帶有機率模型算出的反彈/跌破機率、期望值、風險報酬比、可拆解的交易分數等。
跟 Stock Analysis API 是完全獨立的兩套系統，不要混淆。完整演算法規格見
[sr-zone-scoring.md](./sr-zone-scoring.md)。

**需要 `python.service_url` 已設定、Python HTTP service 已啟動、且機率模型
已訓練過**（`POST /sr-zones/train` 或 CLI `python -m
backtest.modular.sr_scoring.train`），否則 `POST /sr-zones` 會回傳
`502 Bad Gateway`（Python service 沒開）或模型未訓練時的錯誤（fail-fast，
不會靜默回傳中性機率）。`status`/`broken_at`/`broken_price` 由
`POST /sr-zones/:id/verify` 更新（見下方），或由 `daily_close` 排程每天
自動對最近幾筆分析重新驗證一次。

### POST `/sr-zones`

觸發一次分析並寫入 DB。

**Request Body：**
```json
{ "symbol": "2330", "timeframe": "1d", "limit": 250, "reuse_existing": false }
```

`timeframe` 省略時預設 `1d`；`limit` 為抓取的歷史K棒根數，省略或 0 時使用
Python 端預設值（250）。`reuse_existing` 預設 `false`，維持舊契約：每次呼叫
都重新分析並寫入一筆 DB 快照；只有明確傳 `true` 時，後端才會優先重用同
timeframe 且仍在重用期限內（目前 24 小時）的既有快照，找不到可重用快照才會
建立新分析。

**Response（201 Created）：**
```json
{
  "pipeline_version": "v2",
  "analysis": {
    "id": 4,
    "symbol": "2330",
    "timeframe": "1d",
    "analyzed_at": "2026-07-01T00:00:00+08:00",
    "current_price": 985.0,
    "model_version": "v4",
    "model_config_hash": "a1b2c3d4e5f6",
    "period_summaries": [{ "key": "short", "label": "短期", "support": {}, "resistance": null }],
    "analysis_tips": ["短期支撐守穩，接近區間時觀察量價確認。"],
    "chip_summary": { "missing": false, "score": 42.5, "signal": "BULLISH" },
    "created_at": "2026-07-01T10:00:00+08:00"
  },
  "features": {
    "global_trend": 0.032,
    "global_volatility": 0.018
  },
  "score": {
    "global_expected_value": 0.004,
    "global_confidence": 0.61,
    "global_risk_reward_ratio": 0.92
  },
  "evidence": {
    "trend": 0.032,
    "volatility": 0.018,
    "metrics": { "expected_value": 0.004, "confidence": 0.61, "risk_reward_ratio": 0.92 },
    "chip": { "missing": false, "score": 42.5, "signal": "BULLISH" },
    "model": {
      "version": "v4",
      "config_hash": "a1b2c3d4e5f6",
      "explainer": "permutation_shap",
      "explained_output": "calibrated_normalized_probability"
    }
  },
  "decision": {
    "action": "BuySmall",
    "action_label": "小量試單",
    "market_regime": {
      "primary": "TREND_UP",
      "flags": ["HIGH_VOLATILITY"],
      "label": "偏多趨勢但波動偏高",
      "reasons": ["整體趨勢 3.2%"]
    },
    "decision_derived_view": {
      "version": "decision-derived-view-p2",
      "semantic_pipeline": {
        "version": "decision-semantic-pipeline-p4",
        "event_signal": "CLOSE_RECLAIM",
        "lifecycle_phase": "CONFIRMED",
        "market_state": "BULLISH_RECOVERY",
        "bias_state": "BULLISH_BIAS",
        "action_state": "HOLD",
        "entry_permission_state": "PROBE_ALLOWED",
        "reason_codes": ["CLOSE_RECLAIM"],
        "source_order": ["Event", "Lifecycle", "Market State", "Bias", "Action", "Entry"]
      }
    },
    "market_bias": "BULLISH_BIAS",
    "final_entry_permission": {
      "state": "PROBE_ALLOWED",
      "label": "允許觀察性試探",
      "entry_action_state": "PROBE_ENTRY",
      "daily_confirmation_state": "PROBE_ALLOWED",
      "reason_codes": ["CLOSE_RECLAIM", "WAIT_PRICE_FOLLOW_THROUGH"]
    },
    "position_action_condition": {
      "state": "HOLD",
      "structure_state": "SUPPORT_RECLAIM_CONFIRMED",
      "invalidation_price": 960.0,
      "recovery_price": 970.0,
      "reason_codes": ["PRIMARY_SUPPORT", "SUPPORT_RECLAIM_CONFIRMED"]
    },
    "primary_zone": {
      "label": "960.00 ~ 970.00",
      "role": "SUPPORT",
      "distance_label": "1.5%",
      "trading_score": 78.5
    },
    "market_context": [],
    "confidence_explanation": {
      "value": 0.72,
      "level": "HIGH",
      "label": "72%（高）",
      "formula_factors": [],
      "context_factors": []
    },
    "risk_notes": ["波動偏高，倉位需保守。"],
    "secondary_zones": []
  },
  "explanation": {
    "schema_version": "sr_explain_v1",
    "summary": "2330 目前建議以「小量試單」解讀 SR Zone 結果。",
    "action_reason": "Action 為「小量試單」，主因是主交易區 960.00 ~ 970.00 目前被判定為支撐，交易分數 78.5。",
    "market_drivers": ["整體趨勢 +3.2%", "整體波動 1.8%", "整體信心 61%", "籌碼總分 42.5"],
    "risk_notes": ["波動偏高，倉位需保守。"],
    "model_context": {
      "version": "v4",
      "config_hash": "a1b2c3d4e5f6",
      "uses_shap_evidence": true
    }
  },
  "zones": [
    {
      "data": {
        "id": 16,
        "analysis_id": 4,
        "price_low": 960.0,
        "price_high": 970.0,
        "method": "atr",
        "role": "SUPPORT",
        "zone_uid": "0f1c9a2e-8b5d-4c31-9a77-2f6e1d4b8c05"
      },
      "features": {
        "support": { "touch_count": 4, "rejection_count": 3, "breakout_count": 0 },
        "resistance": { "touch_count": 4, "rejection_count": 1, "breakout_count": 1 }
      },
      "score": {
        "tier": "TIER_1_MAIN_STRUCTURE",
        "tier_label": "主結構",
        "support_score": 0.68,
        "resistance_score": 0.30,
        "net_score": 0.38,
        "net_score_label": "STRONG_SUPPORT",
        "confidence": 0.72,
        "confidence_level": "HIGH",
        "bounce_probability": 0.66,
        "break_probability": 0.21,
        "expected_value": 0.0272,
        "risk_reward_ratio": 2.29,
        "touch_count": 4,
        "support_touch_count": 3,
        "resistance_touch_count": 1,
        "recent_validation": "VALIDATED_RECENTLY",
        "trading_score": 78.5,
        "trading_score_breakdown": {
          "expected_value": 26.7,
          "risk_reward": 13.4,
          "trend": 10.0,
          "volume": 10.2,
          "confidence": 7.2,
          "chip": 11.0
        },
        "trading_recommendation": "BUY",
        "overlap_group": 0,
        "confluence_count": 2
      },
      "evidence": {
        "support": {
          "role": "SUPPORT",
          "targets": {
            "hold": {
              "baseline_probability": 0.50,
              "final_probability": 0.66,
              "additivity_error": 0.000002,
              "contributions": [
                {
                  "feature": "rejection_count",
                  "value": 3.0,
                  "contribution": 0.08,
                  "direction": "supportive"
                }
              ]
            }
          }
        },
        "resistance": {},
        "risk_flags": []
      },
      "explanation": {
        "schema_version": "sr_explain_v1",
        "role_summary": "960.00 ~ 970.00 位於現價下方或回測區，暫以支撐解讀。",
        "score_reason": "Trading Score 78.5 主要由期望值貢獻 26.7 分推動；最低分量是信心 7.2 分。",
        "probability_reason": "此區間目前按支撐解讀，反彈/守住機率為 66.0%，跌破/突破機率為 21.0%；期望值為 +2.72%。",
        "confidence_reason": "信心為 72%（高），主要參考目前角色方向樣本 3 次、整體觸碰 4 次、守住 3 次、跌破/突破 0 次；近期性為「最近有守住驗證」。",
        "positive_factors": ["信心等級高", "最近有有效驗證", "多方法共振 ×2"],
        "negative_factors": ["目前沒有明顯扣分因素"],
        "watch_conditions": ["觀察價格回測 960.00 ~ 970.00 時是否止跌", "若收盤跌破 960.00，支撐判斷失效風險升高"]
      },
      "lifecycle": {
        "status": "PENDING",
        "broken_at": null,
        "broken_price": null,
        "resolved_role": null
      }
    }
  ]
}
```

`role=AT_ZONE`（現價落在區間內）的 zone，`bounce_probability` 到
`volume_confirmation` 這些「已解析方向」才有意義的欄位一律是 `null`。
`trading_score_breakdown` 的六個分量加總即為 `trading_score`：EV(34%) + RR(17%) +
Trend(12.75%) + Volume(12.75%) + Confidence(8.5%) + Chip(15%)（見
sr-zone-scoring.md「十二」）。`zones` 陣列依 Tier 由粗到細排序，同層內依
`trading_score` 由高到低排序（`confluence_count` 只當第三順位 tie-
breaker，不改變主要排序規則）。`confidence` 依角色只用該方向
（`support_touch_count`/`resistance_touch_count` 其中之一）的樣本計算，見
sr-zone-scoring.md「六」。`overlap_group`/`confluence_count` 是跨方法重疊
分群結果，`overlap_group` 只有 `confluence_count > 1` 時才有值，見
sr-zone-scoring.md「十七」。

頂層依序對應 Data/Features/Score/Evidence/Decision。`analysis` 同時保存
`period_summaries`、`analysis_tips` 與專屬 `chip_summary`；`decision` 是決策
摘要，其中 `decision_derived_view.semantic_pipeline` 是
`Event -> Lifecycle -> Market State -> Bias -> Action -> Entry` 的權威語意鏈。
`market_bias`、`final_entry_permission.state` 與 `position_action_condition.state`
應優先依此鏈路解讀；legacy `action` / `entry_action_state` 保留作相容明細。
`decision_derived_view.position_gate_state` 仍可能存在於舊相容 payload，但只是
`semantic_pipeline.action_state` 的 deprecated alias。

> **`lifecycle` 這個字在本專案有五種不同意思，讀 response 時務必先確認是哪一種：**
>
> | 出現位置 | 意思 |
> |---|---|
> | zone 的 `lifecycle` 物件（`status`/`broken_at`/`broken_price`/`resolved_role`） | zone 的**驗證狀態**，本文件下方描述的就是這個 |
> | `semantic_pipeline.lifecycle_phase` | **整體事件演進**（`TESTING`/`CONFIRMED`/`CONTINUATION`…） |
> | decision summary zone 的 `zone_health_state` | **zone 本身的健康度**；同物件的 `lifecycle` 字串鍵是它的 deprecated alias |
> | 事件狀態的 `state` | **單一事件**的生老病死（`CANDIDATE`/`ACTIVE`/`RESOLVED`/`EXPIRED`） |
> | `/sr-zones/event-timeline` 的 chain | 跨分析的**事件鏈**，見上方 Event Timeline 章節 |
>
> **`semantic_pipeline.lifecycle_phase` 不再受 RR 影響**（`p4` 起）。分層原則與
> 已知的行為改變見 [`sr-zone-scoring.md`](./sr-zone-scoring.md) 的
> 「分層原則：lifecycle 不看 RR」。
`explanation` 是 deterministic 白話解釋層。每個 zone 也分成
`data/features/score/evidence/explanation/scenario/lifecycle`，驗證 API 只更新
lifecycle。`score` 只帶評分欄位；zone 的識別（id/price_low/method/role/zone_uid）在
`data`、生命週期（status/broken_at…）在 `lifecycle`、
`features/evidence/explanation/scenario` 各自為兄弟鍵，不在 `score` 內重複。
欄位語意見 sr-zone-scoring.md「十四、十九」。

**`data.zone_uid` 是這個 zone 的跨交易日身分**（同一個 uid 出現在不同分析＝系統認為是
同一個 zone），對應 `zone_instances.zone_uid`。**可能是 `null`，但 `null` 不代表
「這個 zone 沒有身分」**——三種成因（舊分析、當次身分寫入降級、`reuse_existing=true`
那條不做身分追蹤的路徑）見 [`database-schema.md`](./database-schema.md) 的
`stock_sr_zones.zone_uid`。客戶端要當作可選欄位處理，不要用它的有無去判斷 zone 是否有效。

`explanation` 不取代 `evidence`：前者給前端直接呈現白話結論、加分/扣分因素與
風險提醒；後者保留 SHAP baseline、最終機率與特徵貢獻等進階模型證據。舊分析
可能沒有 explanation，客戶端應回退顯示 `decision`、`analysis.analysis_tips`
與既有 evidence。

**Explanation 欄位：**

頂層 `explanation`：

| 欄位 | 型別 | 說明 |
|---|---|---|
| `schema_version` | string | Explanation schema 版本，目前為 `sr_explain_v1` |
| `summary` | string | 一句整體白話結論，對齊 `decision.action` |
| `action_reason` | string | 為什麼得到目前 action |
| `market_drivers` | string[] | 趨勢、波動、信心、籌碼等主要因素 |
| `risk_notes` | string[] | 風險提醒；通常整合 decision risk notes 與全局風險 |
| `model_context.version` | string | 產生解釋時使用的模型版本 |
| `model_context.config_hash` | string | 模型訓練設定 hash |
| `model_context.uses_shap_evidence` | boolean | 本次 explanation 是否可引用 SHAP evidence |

每個 `zones[].explanation`：

| 欄位 | 型別 | 說明 |
|---|---|---|
| `schema_version` | string | Explanation schema 版本，目前為 `sr_explain_v1` |
| `role_summary` | string | 支撐、壓力或 `AT_ZONE` 方向未定的白話描述 |
| `score_reason` | string | trading score 的最高與最低分量說明 |
| `probability_reason` | string | 反彈/跌破機率、期望值的解釋；`AT_ZONE` 不給方向性結論 |
| `confidence_reason` | string | 樣本數、近期性、守住/跌破穩定度如何影響 confidence |
| `positive_factors` | string[] | 加分因素 |
| `negative_factors` | string[] | 扣分或風險因素 |
| `watch_conditions` | string[] | 後續要觀察的價位、量能、突破或跌破條件 |

`explanation` 是 deterministic template output，不是 LLM 文字。客戶端可以直接顯示，
但不應把它當成新的 scoring 欄位或交易門檻。

**籌碼摘要欄位**（見 sr-zone-scoring.md「十二之一」）：`analysis.chip_summary`
是整檔層級的籌碼拆解，`score`/`institutional_score`/`margin_score`/`broker_score`
為 −100~100、`concentration_score` 為 0~100、`signal` 為
`BULLISH`/`BEARISH`/`NEUTRAL`/`RISK`。查無籌碼資料時 `chip_summary.missing=true`
且各分數為 `null`（跟「分數接近 0 的中性」不同）；更舊、尚未帶此欄位的分析
則整個 `chip_summary` 為 `null`。新分析的 `evidence.chip` 使用同一份計算結果；
舊分析沒有 evidence 時，客戶端應回退讀取 `analysis.chip_summary`。

每張摘要卡另在 `period_summaries[].support`／`.resistance` 底下帶一個角色化的
`chip` 物件：

```json
"chip": {
  "direction": "bullish",        // bullish / bearish / neutral / none（none=無資料）
  "contribution": 11.0,          // 籌碼對這個角色 trading_score 的直接加權貢獻（0~15，已依支撐/壓力翻號）
  "bounce_delta_pp": 6.2,        // 籌碼對本 zone 反彈（hold）機率的模型邊際貢獻（百分點）；無資料時 null
  "break_delta_pp": -3.0         // 籌碼對本 zone 跌破/突破機率的模型邊際貢獻（百分點）；無資料時 null
}
```

`contribution`（直接加權）與 `*_delta_pp`（v4 模型特徵）是籌碼影響分數的兩條
獨立路徑，不是重複計分。前端摘要卡對支撐顯示 `bounce_delta_pp`（反彈守住）、
對壓力顯示 `break_delta_pp`（突破壓力），兩者是不同事件。

### GET `/sr-zones`

列出歷史分析紀錄。

**Query Parameters：** `symbol`（篩選特定股票）、`limit`（預設 20，最多 200）

### GET `/sr-zones/:id`

取得單筆分析詳情（含 zones 清單），格式同 `POST /sr-zones` 的回應。

### POST `/sr-zones/:id/verify`

手動重新驗證：比對這筆分析之後的實際 K 棒，更新每個 zone 的 `status`
（是否被突破）。**可重複呼叫**，每次都用目前為止最新的資料重新計算，
不是一次性判定；`daily_close` 排程也會每天自動對最近幾筆分析呼叫一次
（見 sr-zone-scoring.md「十四」）。

**Response：** 格式同 `GET /sr-zones/:id`，但 `zones[].lifecycle` 會反映最新
驗證結果：
```json
{
  "pipeline_version": "v2",
  "analysis": { "id": 4, "symbol": "2330" },
  "features": {},
  "score": {},
  "evidence": {},
  "decision": {},
  "zones": [
    {
      "data": { "id": 16, "price_low": 960.0, "price_high": 970.0 },
      "features": {},
      "score": {},
      "evidence": {},
      "lifecycle": {
        "status": "BROKEN",
        "broken_at": "2026-07-05T00:00:00+08:00",
        "broken_price": 940.0,
        "resolved_role": null
      }
    }
  ]
}
```

`role=AT_ZONE` 的 zone 在分析當下現價落在區間內、方向未定，會維持
`PENDING` 直到後續某根K棒收盤真正離開區間才開始判斷突破；`BROKEN` 的 zone
不會因為後續反彈被改回 `HELD_SO_FAR`（沒有另外設計「重置」API）。

### POST `/sr-zones/train`

觸發 `hold_model`/`break_model` 重新訓練。**非同步**——立即建立一筆
`sr_scoring_train_jobs` 紀錄並回傳 `job_id`（`status=pending`），實際訓練在
背景 goroutine 執行（視資料量可能耗時數十秒到數分鐘），依序更新
`pending → running → done`/`failed`。用 `job_id` 輪詢
`GET /sr-zones/train-jobs/:job_id` 查詢進度，不需要只靠伺服器 log 或重新
呼叫 `POST /sr-zones` 猜測新模型是否已生效。

**Request Body：**
```json
{
  "symbols": ["2330", "2454"],
  "timeframe": "1d",
  "limit": 1500,
  "model_type": "gradient_boosting",
  "split_method": "time",
  "calibration_method": "sigmoid"
}
```

`symbols` 省略或空陣列時自動使用整個監控清單（watchlist 為空則回
`400`）；`limit` 為每檔股票訓練用的歷史K棒根數（預設 1500）；`model_type`
可選 `gradient_boosting`（預設）、`hist_gradient_boosting`、`lightgbm` 或
`logistic_regression`。`split_method` 可選 `time`（預設，正式評估建議）或
`random`（舊行為，僅建議比較）；`calibration_method` 可選 `sigmoid`（預設）、
`isotonic` 或 `none`。

目前系統只維持一個現行模型；訓練成功會覆蓋 `SR_SCORING_MODEL_PATH` 指向的
active model。`sr_scoring_train_jobs` 是訓練任務紀錄，不是可切換的模型清單。

**Response（202 Accepted）：**
```json
{ "job_id": "sr_train_20260703_090000_000", "status": "pending", "message": "模型訓練已在背景啟動", "symbols": 12 }
```

### GET `/sr-zones/train-jobs`

列出最近的訓練任務。

**Query Parameters：** `limit`（預設 20，最多 200）

**Response：**
```json
{
  "jobs": [
    {
      "id": 3,
      "job_id": "sr_train_20260703_090000_000",
      "status": "done",
      "symbols": "[\"2330\",\"2454\"]",
      "timeframe": "1d",
      "fetch_limit": 1500,
      "model_type": "gradient_boosting",
      "rows": 128,
      "sources": 2,
      "split_method": "time",
      "metrics": {
        "hold": { "auc": 0.81, "accuracy": 0.76, "brier_score": 0.18, "log_loss": 0.52, "calibrated": 1.0, "train_rows": 102, "test_rows": 26, "positive_rate_train": 0.48, "positive_rate_test": 0.5 },
        "break": { "auc": 0.77, "accuracy": 0.72, "brier_score": 0.21, "log_loss": 0.58, "calibrated": 1.0, "train_rows": 102, "test_rows": 26, "positive_rate_train": 0.31, "positive_rate_test": 0.35 }
      },
      "model_path": "models/sr_scoring_v4.joblib",
      "model_version": "v4",
      "dataset_summary": {
        "rows": 128, "rows_by_symbol": { "2330": 90, "2454": 38 },
        "role_counts": { "SUPPORT": 70, "RESISTANCE": 58 },
        "hold_positive_rate": 0.49, "break_positive_rate": 0.33,
        "feature_zero_rate": { "breakout_count": 0.62, "touch_count": 0.0 },
        "rr_reference_count": 41
      },
      "error": null,
      "started_at": "2026-07-03T09:00:01+08:00",
      "finished_at": "2026-07-03T09:01:47+08:00",
      "created_at": "2026-07-03T09:00:00+08:00"
    }
  ],
  "total": 1
}
```

`rows`/`sources`/`metrics`/`model_path`/`model_version`/`dataset_summary`
只有 `status=done` 才有值；`error` 只有 `status=failed` 才有值。
`split_method` 是 `"time"`（預設，依 `touch_time` 逐股票切分 holdout）或
`"random"`（舊行為）；`metrics.calibrated` 是 `1.0`/`0.0`，訓練集太小時會
自動降級為不校準（見 sr-zone-scoring.md「四」）。

### GET `/sr-zones/train-jobs/:job_id`

取得單筆訓練任務詳情，格式同上方陣列裡的單一物件（`{ "job": {...} }`）。
找不到回 `404`。

### DELETE `/sr-zones/train-jobs`

清理舊的訓練任務紀錄，只刪除 `done` / `failed`，不刪 `pending` / `running`。

**Query Parameters：** `keep`（預設 20；小於 5 會提升為 5，最多 200）

**Response：**
```json
{ "deleted": 12, "keep": 20 }
```

### GET `/sr-zones/model-status`

查詢目前機率模型的狀態，讓前端在觸發分析前就能知道模型準備好了沒——
**永遠回 200**，不像 `POST /sr-zones` 那樣在模型不存在時回 503，用
`exists` 欄位表示狀態。

**Response（模型存在）：**
```json
{
  "exists": true,
  "version": "v4",
  "trained_at": "2026-07-01T13:30:00+08:00",
  "model_path": "models/sr_scoring_v4.joblib",
  "split_method": "time",
  "metrics": { "hold": { "auc": 0.81, "calibrated": 1.0 }, "break": { "auc": 0.77, "calibrated": 1.0 } },
  "feature_names": ["touch_count", "rejection_count", "..."],
  "config_hash": "a1b2c3d4e5f6",
  "training_config": {
    "dataset_config": { "forward_bars_support": 5, "threshold_pct_support": 0.03 },
    "zone_builders": { "ATRZoneBuilder": { "atr_width_multiplier": 1.5 }, "VolumeProfileZoneBuilder": { "num_bins": 24 } },
    "model_type": "gradient_boosting", "split_method": "time", "calibration_method": "sigmoid"
  }
}
```
`config_hash`/`training_config` 見 sr-zone-scoring.md「十六」——`config_hash`
跟分析快照的 `model_config_hash` 是同一個值，可以用來確認「現在的模型」
跟「某筆舊分析用的模型」是不是同一組訓練設定。

**Response（模型不存在）：**
```json
{
  "exists": false, "version": null, "trained_at": null, "model_path": null,
  "split_method": null, "metrics": null, "feature_names": null,
  "config_hash": null, "training_config": null
}
```

### DELETE `/sr-zones/:id`

刪除一筆分析紀錄（連同其 zones 一併刪除）。

---

## Chip API

籌碼分析 API 使用已同步到 DB 的三大法人、融資融券、券商分點與 `chip_scores`。
資料同步由 `POST /chips/sync` 建立非同步 job；收盤後 `daily_close` 也會另外跑
`chip_daily_sync`，其紀錄在 `job_runs`，不是 `chip_sync_jobs`。

### GET `/chips/:symbol/summary`

查詢單一股票籌碼摘要。`date` 省略時回傳最新一筆 `chip_scores`；若指定日期但查無
分數，回 `404`。

**Query Parameters：** `date`（選填，`YYYY-MM-DD`）

**Response：**
```json
{
  "symbol": "2330",
  "date": "2026-07-03",
  "signal": "BULLISH",
  "totalScore": 72.5,
  "reason": ["外資連續買超 4 日"],
  "institutional": {
    "foreignNetBuy": 12000,
    "investmentTrustNetBuy": 3000,
    "dealerNetBuy": -500,
    "consecutiveDays": 4
  },
  "margin": {
    "marginBalance": 23000,
    "marginChange": -1200,
    "shortBalance": 4200,
    "shortChange": 800
  },
  "broker": {
    "topNetBuy": 9000,
    "concentration": 0.18
  }
}
```

`institutional` / `margin` / `broker` 會各自獨立查詢；某區塊查無資料時省略該區塊，
不會讓整個 summary 失敗。

### GET `/chips/:symbol/scores`

查詢歷史籌碼分數。

**Query Parameters：** `from`、`to`（必填，`YYYY-MM-DD`）

**Response：**
```json
{ "symbol": "2330", "scores": [ { "trade_date": "2026-07-03T00:00:00+08:00", "total_score": 72.5, "...": "..." } ] }
```

### GET `/chips/:symbol/brokers`

查詢券商分點買賣超排行。

**Query Parameters：**

| 參數 | 預設 | 說明 |
|------|------|------|
| date | 必填 | `YYYY-MM-DD` |
| limit | `20` | 1～200，超出範圍會退回 20 |

**Response：**
```json
{ "symbol": "2330", "date": "2026-07-03", "topBuy": [ { "...": "..." } ], "topSell": [ { "...": "..." } ] }
```

### POST `/chips/sync`

手動同步籌碼資料，立即建立 `chip_sync_jobs` 紀錄並背景執行。

**Request Body：**
```json
{
  "mode": "manual",
  "symbols": ["2330", "2317"],
  "from": "2026-07-01",
  "to": "2026-07-03",
  "dataTypes": ["institutional", "margin", "broker", "scores"],
  "force": false
}
```

`mode` 可為 `manual` 或 `backfill`，省略時為 `manual`。`manual` 未指定日期時只同步
今天；`backfill` 未指定 `from` 時會使用 `chip.sync.history_trading_days` 往回推。
`force` 目前會記錄在 job，但 upsert 本身已具冪等性，尚未實作跳過既有資料的特殊邏輯。

**Response（202 Accepted）：**
```json
{
  "job": {
    "job_id": "chip_20260708_120000_000",
    "mode": "manual",
    "status": "pending",
    "symbols_total": 2,
    "symbols_done": 0,
    "symbols_failed": 0
  }
}
```

### GET `/chips/sync/:job_id`

查詢 manual/backfill 籌碼同步任務。

**Response：**
```json
{
  "job": {
    "job_id": "chip_20260708_120000_000",
    "status": "done",
    "symbols_done": 2,
    "symbols_failed": 0,
    "failures": []
  }
}
```

`status` 可為 `pending`、`running`、`done`、`partial`、`failed`（`failed` 也涵蓋背景執行
panic 被攔下的情況，`error` 為 `internal error`）。

找不到 job 回 `404`；查詢過程發生其他錯誤（DB 連線中斷等）回 `500`——語意與
`GET /market/backfill/:job_id` 一致，該處有詳細說明。前端輪詢的停滯偵測也相同。

`job_id` 格式為 `chip_<UTC 時間戳到毫秒>_<4 位隨機碼>`。

---

## Trade Analysis API

Trade Analysis 是新前端與 API 的統一交易決策入口。呼叫端只需要提供股票代號；
後端會自動讀取 `positions` projection，若資料庫沒有持股資料或股數為 0，就以
`FLAT` 空手情境分析；若有股數則以 `LONG` 持股情境分析。

- `POST /trade-analysis/analyze`：body 為
  `{"symbol":"2330","timeframe":"1d","limit":250,"force_refresh":false}`。
- `GET /trade-analysis/:symbol/history?limit=20`：列出該股票 FLAT/LONG 共用分析歷史。

`POST /trade-analysis/analyze` 回應：

```json
{
  "context": {
    "symbol": "2330",
    "position_state": "FLAT",
    "has_position": false
  },
  "analysis": {},
  "sr_zone_analysis": {},
  "zones": []
}
```

`analysis` 沿用 Position Analysis 快照格式；`sr_zone_analysis` 與 `zones` 沿用
SR Zone 快照格式。分析入口統一由 `/trade-analysis/*` 提供（舊的
`/position-analyses` 分析 endpoints 已移除）。

`analysis.sr_zone_analysis_id` 是 best-effort historical reference。若對應 SR Zone
快照後來被刪除，trade-analysis 歷史仍會保留 `analysis` 內的決策快照欄位，但
`sr_zone_analysis` / `zones` 可能無法再由該 id 回查完整市場結構明細。

---

## Position Analysis API

Position Analysis 是 Trade Analysis 背後的決策快照與部位帳務 API。沒有
transaction/projection 時視為 `FLAT`，有股數時為 `LONG`。交易決策入口統一為
`/trade-analysis/*`；以下 endpoints 提供 position ledger 與 projection。

- `GET /groups`：列出目前使用者加入的 groups。
- `POST /groups`：建立 group；建立者自動成為 `OWNER`。
- `POST /groups/:id/members`：由 group `OWNER` / `ADMIN` 新增或更新成員 role（body：`user_id`、`role`）。
  角色保護：actor 不得修改自己的 role；只有 `OWNER` 能授予 `OWNER`、或異動一個現任 `OWNER`（`ADMIN`
  不得碰 `OWNER`）；不得把最後一名 `OWNER` 降級。被加入者必須已是 group tenant 的成員，否則回 403
  （不自動補 tenant membership，避免把任意 user 拉進 tenant 取得 TENANT portfolio 寫入權的提權副作用）。
- `GET /portfolios`：列出目前使用者可用的 portfolios（個人、已加入 group 的 portfolio）；
  response item 包含 `can_write`，前端用來停用唯讀 portfolio 的寫入操作。
- `POST /portfolios`：建立 portfolio；body 包含 `name`，若帶 `group_id` 則建立 group-owned portfolio，
  且呼叫者必須是該 group 的 `OWNER` / `ADMIN`。
- `GET /positions?portfolio_id=...`：列出目前 LONG positions；`portfolio_id` 為必要 query。
- `GET /positions/:symbol?portfolio_id=...`：取得 projection；空手回傳股數、AVG、version 均為 0。
- `GET /positions/:symbol/transactions?portfolio_id=...`：取得 immutable ledger。
- `POST /positions/:symbol/transactions`：新增 BUY/SELL；body 包含
  `portfolio_id`、`event_type`、`shares`、`price`、`fee`、`tax`、`occurred_at`、
  `expected_version`、`note`。SELL 不得超賣。
- `POST /positions/:symbol/adjustments`：新增 ADJUSTMENT；body 包含更正後
  `portfolio_id`、`target_shares`、`target_avg_cost`、`expected_version` 與必填 `reason`。
  ADJUSTMENT 只校正 projection，不代表成交、不改變 `realized_pnl`；實際交易使用
  BUY/SELL transaction。

所有 position / trade-analysis endpoints 都會檢查目前 JWT 使用者是否可存取該 `portfolio_id`。
讀取 endpoints 需要 read access；BUY/SELL/ADJUSTMENT 與 `POST /trade-analysis/analyze`
需要 write access，因為 analyze 會新增 `position_analyses` 快照。

`POST /trade-analysis/analyze` body 必須帶 `portfolio_id`；Position Engine 會使用該 portfolio 的
持倉成本、股數與 version 產生 FLAT/LONG 決策。分析歷史改由
`GET /trade-analysis/:symbol/history?portfolio_id=...` 取得；分析輸出包含 `position_state`、Position version、目前／目標／調整股數、
`adjustment_side`/`adjustment_amount`、Action、進場／停損／停利價、風險金額、
預期報酬、RR、已實現／未實現損益、設定快照、Evidence、觸發與失效條件。

固定預設為：單股上限 200,000、最大風險 10,000、加碼 tranche 25%、
最低 RR 1.5、突破後無上方壓力時以 2R 推導停利目標、停利減碼 50%。設定由
`backend/config.yaml::position_analysis` 覆寫。分析 evidence 的
`take_profit_source` 為 `RESISTANCE_ZONE` 或 `BREAKOUT_R_MULTIPLE`，可區分停利價來源。

## Legacy Holdings API（已移除）

`/holdings*` 與 `/holding-analyses*` routes 已由 migration 038 移除，不應再由
新客戶端呼叫。舊 `holdings` 會依 symbol 轉成一筆 `OPENING_BALANCE`
transaction 與 `positions` projection；舊 `holding_analyses` 會搬到
`position_analyses`，並以 `rule_version=holding_sr_zone_v1_legacy` 保留歷史來源。

新流程請使用 `/positions*` 管理 immutable ledger / AVG projection，並使用
`/trade-analysis/*` 或相容的 `/position-analyses*` 產生分析快照。

---

## WebSocket

**連線：** `ws://localhost:8080/ws/market`

**訂閱：**
```json
{ "action": "subscribe", "symbols": ["2330", "2454"] }
```

同時訂閱檔數**最多 3 檔**（跟 watchlist 的 `watched` 欄位上限一致，見
Watchlist API 的 `PATCH /watchlist/:symbol/watch`）；超過上限的 symbol 會被
忽略並記一筆 server log，不會回錯誤給 client（目前協定沒有 ack/error 訊息
機制）。真正的把關點是 `watched` 欄位——前端只會對監聽中的股票送出
subscribe，這裡的檔數上限只是防禦性的第二層保護。

**取消訂閱：**
```json
{ "action": "unsubscribe", "symbols": ["2330"] }
```

**Server 推送事件：**
```json
{ "type": "candle",    "symbol": "2330", "data": { ...Candle } }
{ "type": "indicator", "symbol": "2330", "data": { ...Snapshot } }
{ "type": "signal",    "symbol": "2330", "data": { ...Signal } }
```

> **目前只有 `signal` 事件會真的被推播**（`signal.Engine.BroadcastFn`，在
> `cmd/server/main.go` 註冊）。`candle`/`indicator` 事件型別雖然定義在前端
> `ws/socket.ts`，但後端從未送出，一般情況（沒有觸發突破/爆量）下不會有
> 推播。前端 Dashboard 因此改用 REST（`/candles`、`/indicators`、`/signals`）
> 在頁面載入時主動 hydrate 監控清單欄位，WebSocket 只負責之後的訊號覆蓋更新。

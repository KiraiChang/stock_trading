# ISSUE：遇到的問題與已知限制

記錄實際發生過的 bug、矛盾結果、文件/程式碼不一致，以及設計上刻意接受的
限制。跟「想做的優化」無關的項目放這裡；未來想做的功能擴充記錄在
[todo.md](./todo.md)。

## 使用說明

- **狀態**：`待修復` / `修復中` / `已修復` / `已知限制（不計畫修復）`
- **嚴重度**：`高`（結果矛盾/資料錯誤）/ `中`（誤導但不影響核心功能）/
  `低`（文件或註解落後，不影響 runtime）
- 新增項目時往下加一筆，編號遞增（`I-0xx`）。修復後若仍需短期追蹤，先把「狀態」改成
  `已修復` 並補上「修復方式」；若修復紀錄已移到對應主題文件或 review 文件，
  則可從本清單移除。

---

### I-040：production regression governance gate 在該模型首次 decision-replay 寫入前為 no-op（by-design）

| 欄位 | 內容 |
|---|---|
| 狀態 | 已知限制（刻意，by-design） |
| 嚴重度 | 低 |
| 分類 | Python / SR Zone / 模型治理 |
| 發現日期 | 2026-07-27 |
| 來源 | T-002 P2 review |

`pipeline._merge_regression_governance_gate` 只有在 `fetch_latest_sr_regression_governance(model_config_hash)`
查得同模型、`schema_version=sr_zone_decision_replay_p0` 的最新 replay 結果時才會作用。若該
`model_config_hash` 尚未跑過任何 `--write-db` 的 decision replay（新訓練模型、或 scheduler 關閉且從未手動
執行），fetch 回 None → gate 為 no-op，分析維持原本模型治理。這是刻意的安全預設（gate 只趨保守、
不因缺資料而誤擋），但意味著**這層 P2 安全網要等該模型至少跑過一次 evaluation 並寫入 DB 後才生效**。
上線流程若倚賴此 gate，需確保新模型部署後有排入一次 decision replay。屬營運相依，非 bug。

---

### I-053：2 GiB host 下 live stack 與本機開發工具（claude / codex）不可併存，會被 host OOM killer 砍掉呼叫端

| 欄位 | 內容 |
|---|---|
| 狀態 | 已知限制（環境限制，不改 runtime） |
| 嚴重度 | 中 |
| 分類 | 開發環境 / Docker / 記憶體 |
| 發現日期 | 2026-08-05 |
| 來源 | 2026-08-05 16:22 claude 被 OOM kill 事故調查 |

**現象**：Claude Code 這端完全閒置（沒跑測試、沒 build）時，claude 行程仍被 host OOM killer
砍掉，並連帶引發 `docker-proxy` × 8 與 `dockerd` 被砍、所有 container 被停的 cascade。

**時序（2026-08-05，CST）**：

| 時間 | 事件 |
|---|---|
| 15:56:03 | live project `stock_trading`（backend / python-server / python-worker）被啟動 |
| 16:22:02 | host OOM killer 砍掉 claude（anon-rss 402 MB、total-vm 5.9 GB） |
| 16:22:31–16:23:57 | 連續砍掉 8 個 `docker-proxy` |
| 16:23:57 | 砍掉 `dockerd`（anon-rss 314 MB） |
| 16:23:58 | systemd 重啟 dockerd，接手後把殘留 container 全停（`python-worker` 逾時被 SIGKILL → `exit=137`） |
| 16:24:13 | 只有 gitea（restart policy）回來 |

**根因**：host cgroup 上限只有 2 GiB，事發當下同時有 **10 個 container**（15:56 才起的 live
stack 3 個 ＋ 既有 7 個：dev postgres、postgres、redis、caddy、fin-api、akatengu、gitea），
外加 host 上的 claude ~400 MB、codex ~137 MB、dockerd ~314 MB、containerd 與 10 個 shim。
每個 container 都設了 `mem_limit: 512m`，但**加總約 5 GB 遠超過 host 的 2 GiB**——per-container
上限只保證單一 container 不超標，不保證總和。所以沒有任何 container 撞到自己的 cgroup 上限
（全部 `OOMKilled=false`），是 host 先耗盡，再由 host 層級 OOM killer 挑 badness 最高的行程，
也就是持有最大 heap 的 claude。詳見 `development-workflow.md` 的
「`MEM` 是上限，不是預留」與「container 上限的**總和**也要顧」。

**規避方式**：本機同時只允許一組 stack 常駐。要跑 live project 前先確認沒有其他 stack 佔著
記憶體；驗收一律用 `docker-compose.dev.yml` 的 dev project（CLAUDE.md 規定），不要在本機把
live/deploy project 拉起來。

**調查此類事故時的陷阱**：這台沙箱的 `dmesg -T` **絕對時間不可信**（kernel 單調時鐘與
wallclock 有數十小時偏移，本次為 ~44.8 小時，會把當天事件標成兩天前）。判讀方式：

- 用 `dmesg` 的**相對間隔**搭配 `docker inspect` 的 `StartedAt` / `FinishedAt`（docker 用自己的
  wallclock，可信）交叉定位。
- 決定性驗證：`dmesg` 最後一行的 veth 名稱是否等於 `ip -o link` 目前唯一存在的 veth，
  是的話該行就是「最近一次 container 啟動」。
- kernel ring buffer 只留約 60 行 / 1.7 小時，更早的事故不會留下紀錄，別把「查不到」當成
  「沒發生」。

**附帶發現（待套用）**：`/opt/stacks/scripts/gitea/compose.yml` 是唯一沒有 `cpus` /
`mem_limit` / `memswap_limit` 設定的 stack（實測佔 154 MB），其他 stack 都是
`0.5` CPU / `512m` / `768m`。已決定補齊，但該路徑屬 `kirai`（uid 1000），需由該帳號或
sudo 手動套用後重啟 gitea。

---

### I-054：mysql 的執行期支援仍未被驗證（DDL 已驗，CRUD 未驗）

| 欄位 | 內容 |
|---|---|
| 狀態 | 已知限制（部分已解，剩餘部分暫不修） |
| 嚴重度 | 低（目前無人以 MySQL 部署） |
| 分類 | Go / DB / migration |
| 發現日期 | 2026-08-06（2026-08-07 縮小範圍） |
| 來源 | T-037 B review |

**已解決的部分**（2026-08-07）：mysql migration 現在有可重複執行的驗證路徑
（`scripts/test-mysql-migrations.sh`），首次執行抓到並修好 5 個保留字語法錯誤，
57 個 migration 已全數 up 成功。用法與設計見
[`development-workflow.md`](./development-workflow.md)；欄位命名規範見
[`database-schema.md`](./database-schema.md) 的「欄位命名規範：避開 MySQL 保留字」。

**仍未解的兩項**：

1. **驗證只涵蓋 DDL，不涵蓋 repo 層的 CRUD round-trip。** 「表建得起來」不等於
   「INSERT/SELECT 跑得動」——目前只能說不再有*已知的*保留字阻礙，但沒有任何自動化
   流程證明 Go repo 的查詢在 MySQL 上真的能執行。要補的話是讓 `internal/store` 的
   repo 測試能對著真實 MySQL 跑一輪，而不是只跑 sqlite。
2. **`057_add_sr_zone_builder_runtime_config.sql` 的 DEFAULT 不對稱**：mysql 版走
   `ADD COLUMN ... NULL` → `UPDATE` → `MODIFY ... NOT NULL` 三步（比照 033），
   postgres / sqlite 各自一步用 `NOT NULL DEFAULT`，導致 **mysql 版沒有 DEFAULT**——
   省略該欄位的 INSERT 在 mysql 會失敗、另外兩個 engine 會成功。目前唯一的寫入路徑
   （`sr_zone_repo.Create`）永遠帶齊欄位且有 normalization guard，所以不構成實際問題。

`backend/config.yaml` 目前仍把 mysql 列為生產可選 driver——要嘛補上第 1 項的驗證，
要嘛明確宣告不支援並移除該選項。

---

### I-056：SR evaluation 的規模上限——`sources` 與 `dataset` 必須同時常駐記憶體

| 欄位 | 內容 |
|---|---|
| 狀態 | 已知限制（擴標的池前的必經改造，見 todo.md T-040） |
| 嚴重度 | 中 |
| 分類 | Python / SR Zone / 效能 |
| 發現日期 | 2026-08-06 |
| 來源 | T-039 sweep 實跑 ＋ 擴標的池的可行性評估 |

`run_evaluation()`（`evaluation.py:335`）的結構決定了它的規模上限：

```python
sources = _load_db_sources(symbols, timeframe, limit)   # 所有標的的 DataFrame 一次全載
dataset = build_training_dataset(sources, builders, ...)  # 全部 touch 併成一張表
...
model_metrics = _model_metrics(dataset, bundle)
volatility_profiles = _volatility_profiles(sources, dataset)   # ← 同時要 sources 和 dataset
```

**`sources` 無法在建完 `dataset` 後釋放**，因為 `_volatility_profiles` 還要用它。所以峰值
記憶體 = 全部原始 K 線 ＋ 全部 touch dataset ＋ 模型 ＋ 中間物，四者同時存在。

**實測基準（2026-08-06）**：11 檔 × `limit=1500` → 5,928 touches，container **270MB 跑得完**，
單次約 70 秒；6 組 sweep 約 7 分鐘。

**但不要拿 270MB 做線性外推**——那 270MB 幾乎都是 Python ＋ pandas / numpy / sklearn /
lightgbm / shap 的 import 開銷，資料本身只有數 MB（原始 frame 約 1MB、dataset 約 1MB）。
真正隨標的數成長的是原始 frames、touch dataset，以及建 zone／算特徵時的中間物：

| 標的數 | 原始 frames | touch dataset | 粗估峰值 | 單次耗時 |
|---|---|---|---|---|
| 11（現況） | ~1MB | ~1MB | **270MB（實測）** | **70 秒（實測）** |
| 150 | ~14MB | ~13MB | 約 350～450MB（**未實測**） | 約 16 分鐘 |
| 2,298（全市場） | ~220MB | ~200MB | 約 0.8～1.2GB（**未實測**） | 約 4 小時 |

**時間是硬性的線性成長**（walk-forward 逐檔跑），sweep 還要再乘候選數——全市場的 6 組 sweep
約 24 小時。這台 host 的 `MemAvailable` 常態只有 450～510MB、mem-guard 再保留 150MB，
**150 檔已經在邊緣，全市場給不起**。

**可行的改造方向（尚未實作，記下來避免重新推導）**：

1. **串流化**：逐檔建完 dataset 後立刻釋放該檔的原始 frame。前提是把
   `_volatility_profiles` 改成逐檔算好 profile 再丟掉 frame，而不是最後才一次算。
   這是最有效的一刀——原始 frames 是全市場情境下最大的一塊。
2. **指標可以串流，但不能分批平均**：AUC 是非線性的排序統計量，**把各批的 AUC 平均是錯的**。
   正確做法是只累積「預測機率 ＋ label」兩個一維陣列（全市場約 124 萬列 × 2 × 8B ≈ **20MB**），
   最後一次算 AUC / Brier / log loss。這條路可行且便宜。
3. 降 `--limit`（每檔取較少 K 棒）或對標的抽樣——最省事，但直接犧牲樣本量，
   而樣本量正是擴標的池要解決的問題，只適合當臨時手段。

**現況結論**：擴到 100～200 檔可以先不改造（但要實測，不要假設），**全市場路線必須先做第 1、2 項**。

### I-062：decision replay row 沒有前端型別（刻意）

| 欄位 | 內容 |
|---|---|
| 狀態 | 已知限制（刻意，待有消費者時再處理） |
| 嚴重度 | 低 |
| 分類 | Frontend / SR Zone / API Contract |
| 發現日期 | 2026-08-07 |
| 來源 | T-028 RR distribution ＋ 更細分層實作 review |

`SREvaluationReport` 只有 `[key: string]: unknown`，TypeScript 沒有任何型別描述
`replay_rows[].primary_zone`（含 `relative_volume`）。**這是刻意的**：前端目前不渲染
`replay_rows`，加一個沒有消費者的宣告只會成為下一個默默寫錯的型別。

判準與「沒有消費者時該怎麼辦」已升級為通則，見
[`development-workflow.md`](./development-workflow.md) §3 的
「什麼時候才該新增跨語言的型別宣告」。權威形狀由 `test_evaluation.py` 對
`primary_zone` 的 key 集合斷言鎖住。

**本筆保留的原因**：它是一個仍然成立的**現況限制**（要做 replay row drilldown、匯出或
event timeline 時會缺型別基礎），不是待辦也不是已修的 bug。等真的有消費者時一併處理即可。

---

### I-064：live `candles` 有 4 根全零 K 棒（OHLCV 皆為 0）

| 欄位 | 內容 |
|---|---|
| 狀態 | 已實作／待 review（防護已加、live 資料已清；**`VALIDATE CONSTRAINT` 待 060 部署後執行**） |
| 嚴重度 | 中 |
| 分類 | 資料品質 / Go 抓取 |
| 發現日期 | 2026-08-10 |
| 來源 | T-028 的 drawdown-like failure window 實跑（見 `sr-zone-scoring.md`） |

live DB 的 `candles`（`timeframe='1d'`）有 4 列 open/high/low/close/volume **全部為 0**：

| symbol | 交易日（Taipei） | `ts`（UTC） |
|---|---|---|
| 2454 | 2016-05-13 | 2016-05-12 16:00+00 |
| 3630 | 2024-12-18 | 2024-12-17 16:00+00 |
| 2317 | 2025-07-30 | 2025-07-29 16:00+00 |
| 1101 | 2025-08-13 | 2025-08-12 16:00+00 |

```sql
select symbol, (ts at time zone 'Asia/Taipei')::date, open, high, low, close, volume
from candles where timeframe='1d' and low <= 0;
```

> `ts` 存 UTC（Taipei 00:00 = 前一日 16:00+00），查詢一定要 `at time zone 'Asia/Taipei'`
> 再取 date，否則列出來的日期會整整差一天。

**怎麼被發現的**：終點報酬看不到它——一根零價 K 棒夾在中間，前後收盤照常，
`two_bar_close_return` 只是略微失真。是 `max_favorable_excursion_pct` **剛好等於 1.0000**
（窗口內最低價為 0 ⇒ 相對確認日收盤下跌 100%）才把它逼出來。**看路徑才看得到的錯誤，
看終點看不到。**

**已查證的部分**：`market/fetcher.go:203` 的 `toStoreCandles` 對價格**沒有任何驗證**，
上游給什麼就寫什麼；`finmind.go:181` 的 `FetchDailyCandles` 同樣直接透傳 `raw.Open/High/Low/Close`。
所以只要上游回一天零值，它就會進 DB。

**成因無法從現有資料判定**：那 4 天其他 27～28 檔都正常交易且有量，所以**不是整輪抓取失敗**，
而是單檔單日異常。個股停牌（上游以 0 表示無成交）與上游 glitch 兩種可能都說得通，
現有資料分不出來——但**兩種情況的修法相同**：無成交的日子應該是「沒有那筆資料」，
不是「一根價格為 0 的 K 棒」。

**現況影響**：所有跨到這 4 天的技術指標、zone 建構與 replay 結果都被污染。

#### 已完成（2026-08-10）

**兩層防護**，因為單靠任何一層都不夠——Go guard 擋不住手動 SQL 與未來新增的匯入工具，
DB 約束則不會告訴你「哪一檔哪一天被丟掉了」：

1. **寫入端**：`market/fetcher.go` 的 `toStoreCandles` 改成 method，擋掉
   `open/high/low/close` 任一 `<= 0` 的 K 棒並記 Warn log（帶 symbol / ts / 四個價格）。
   一根壞的只丟那一根，不影響同批其他 K 棒。
   **只驗價格不驗 volume**：成交量為 0 在盤中分K 是正常的，價格為 0 不是。
2. **DB 約束**：migration 060 對三種 engine 加 `ck_candles_positive_price`。

**postgres 用 `NOT VALID` 是刻意的**：live 那 4 列還在（清資料未授權），完整約束會讓
migration 直接失敗。`NOT VALID` 只約束之後的寫入、不回頭驗既有列，所以髒資料還在時也套得上。
清完之後要再跑一次升級成完整驗證：

```sql
ALTER TABLE candles VALIDATE CONSTRAINT ck_candles_positive_price;
```

sqlite 沒有 `ALTER TABLE ADD CONSTRAINT`，060 是整張表重建；Up / Down 都保留資料
（與 017／018 那種破壞性重建不同）。

**測試**：`fetcher_test.go` 涵蓋四個欄位各自為 0、負價、零成交量不該被擋、
一根壞的不影響同批其他根；`migrate_sqlite_test.go` 驗約束生效、**資料活過重建**、
Down 之後約束消失且資料仍在；`migrate_mysql_test.go` 實際寫違規列驗約束真的被強制執行
（migration 跑得過不代表約束有效）。

順手修掉一個 stub 保真度問題：`stubDailySource` 原本只設 `Close`，Open/High/Low 都是 0，
加了 guard 之後那些 K 棒會被整根丟掉——測試仍然會過，但**是因為錯誤的理由**。已補齊四個價格。

#### live 資料清理（2026-08-10，已完成）

那 4 列已從 live 刪除，以明確的 id 逐筆刪（`WHERE id IN (15326, 239015, 1130675, 1159370)`），
不用 `WHERE low<=0` 這類條件式——條件寫錯的代價是刪掉真實資料。整段包在交易裡，
並在 COMMIT 前斷言「非正價格剩餘 0 列」，不成立就整筆回滾。

刪除後核對：非正價格 0 列；四檔各恰好少 1 列（2454 4867→4866、3630 2413→2412、
2317 1594→1593、1101 1594→1593）；2317 的 2025-07-30 消失而 07-29 與 07-31 仍在。
被刪的 4 列內容（全為 0）已留底，需要時可還原。

#### 未做：`VALIDATE CONSTRAINT`

**live 目前在 goose 版本 59，migration 060 還沒部署**，所以 `ck_candles_positive_price`
在 live 上還不存在，`VALIDATE CONSTRAINT` 無從執行。060 會在下次部署、backend 啟動時
自動套用（`cmd/server/main.go` 的 `RunMigrations`）。部署後執行：

```sql
ALTER TABLE candles VALIDATE CONSTRAINT ck_candles_positive_price;
```

**為什麼不趁 live 已清乾淨就把 060 改成一般（會驗證既有列）的約束**：寫入端的 guard
**也還沒部署**。從現在到部署之間，排程仍可能寫進新的零價 K 棒；那時一個會驗證的約束
會讓 migration 失敗、連帶擋住整個部署。維持 `NOT VALID` 則不管資料當下乾不乾淨都套得上，
把「驗證既有列」留成部署後的獨立動作——那時寫入端的 guard 也已生效，不會再有新的髒資料進來。
（postgres 的 060 原本沒有實跑證據，2026-08-10 已補上
`scripts/test-postgres-migrations.sh` 與 `migrate_postgres_test.go` 驗過——
其中 `TestPostgresMigrationsToleratePreexistingBadRows` 直接重現 live 的處境：
先寫一列髒資料再套 060，套得上去、新寫入被擋、`VALIDATE CONSTRAINT` 在髒資料還在時失敗、
清乾淨後才成功。）

---

### I-065：`candles` 存的是**未還原**股價，除權除息／分割會產生假跳空

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 高 |
| 分類 | 資料品質 / 回測正確性 |
| 發現日期 | 2026-08-10 |
| 來源 | 同 I-064 |

0050 在 2025-06 分割（1:4），而 `candles` 存的是未還原價，於是序列上出現一根假的 −75%：

| 交易日（Taipei） | close |
|---|---|
| 2025-06-10 | 188.65 |
| 2025-06-18 | 47.57 |

期間 2025-06-11～06-17 **0050 完全沒有 K 棒，而同期其他 28 檔都有**——這是分割換發期間
停止交易，不是資料缺漏。查到這段空白時不用再追一次。

**影響範圍遠大於 T-028**：所有以 `candles` 為輸入的東西都受影響——MA / RSI / MACD / ATR、
zone 建構、breakout / volume spike 偵測、decision replay、模型訓練特徵。凡是窗口跨過
公司行動日的樣本，看到的都是一個從未發生的暴跌或暴漲。

**為什麼一直沒被發現**：日常使用看的是**近期**資料，而公司行動稀疏；只有回測與 replay
會系統性掃過歷史，才會踩到。本次是 MAE 的 `min = -1.0` 與 0050 的 −75.5% 兩個離群值
把它翻出來的。

**修復方向（未定案，需另立計畫）**：

1. 抓取端改存還原股價，或另存還原係數欄位。
   > **待查證**：目前用的是 `dataset=TaiwanStockPrice`（`finmind.go:183`），確定是未還原。
   > FinMind 是否提供還原版 dataset、以及**是否需要更高的 token tier**，我沒有查證過。
   > repo 已有 `ErrInsufficientTier`（`finmind.go:26`）顯示 tier 限制在這裡是真實約束，
   > 所以這條路可不可行必須先確認，不能當成已知選項。
2. 或在讀取端套用還原係數——但這樣每個消費者都要記得套，容易漏。
3. 無論哪種，都要決定既有歷史資料如何回填／重抓。

**現況提醒**：在此之前，**所有跨越公司行動的回測與 replay 數字都要當成不可靠**。

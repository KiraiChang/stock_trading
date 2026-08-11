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
| 來源 | SR Zone RR distribution ＋ 更細分層實作 review（2026-08-07） |

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

### I-065：`candles` 存的是**未還原**股價，除權除息／分割會產生假跳空

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 高 |
| 分類 | 資料品質 / 回測正確性 |
| 發現日期 | 2026-08-10 |
| 來源 | SR Zone drawdown-like failure window 實跑（2026-08-10） |

0050 在 2025-06 分割（1:4），而 `candles` 存的是未還原價，於是序列上出現一根假的 −75%：

| 交易日（Taipei） | close |
|---|---|
| 2025-06-10 | 188.65 |
| 2025-06-18 | 47.57 |

期間 2025-06-11～06-17 **0050 完全沒有 K 棒，而同期其他 28 檔都有**——這是分割換發期間
停止交易，不是資料缺漏。查到這段空白時不用再追一次。

**影響範圍遠大於發現它的那筆工作**：所有以 `candles` 為輸入的東西都受影響——
MA / RSI / MACD / ATR、zone 建構、breakout / volume spike 偵測、decision replay、
模型訓練特徵。凡是窗口跨過公司行動日的樣本，看到的都是一個從未發生的暴跌或暴漲。

**為什麼一直沒被發現**：日常使用看的是**近期**資料，而公司行動稀疏；只有回測與 replay
會系統性掃過歷史，才會踩到。這次是 MAE 的 `min = -1.0` 與 0050 的 −75.5% 兩個離群值
把它翻出來的。

**修復方向**：計畫書見 [`todo.md`](./todo.md) T-042（2026-08-11，待確認）。摘要：

##### FinMind 資料源查證結果（2026-08-10，以 live token 實際打 API）

| dataset | 現有 tier（`register`） | 內容 |
|---|---|---|
| `TaiwanStockPriceAdj`（還原股價） | ❌ **HTTP 400，需 Sponsor** | — |
| `TaiwanStockKBar`（分K） | ❌ 同上 | 早已知（`intraday_enabled` 預設 false） |
| `TaiwanStockPrice`（現用，未還原） | ✅ 200 | 現況資料來源 |
| `TaiwanStockSplitPrice` | ✅ 200，**且可不帶 `data_id` 整批抓** | 分割前後價 |
| `TaiwanStockDividendResult` | ✅ 200，**但只能逐檔**（整批需 Sponsor） | 除權息前後參考價 |

錯誤訊息原文：`Your level is register. Please update your user level.`

**結論：直接抓還原股價這條路被 tier 擋死，但「自行計算還原係數」在現有 tier 完全可行。**
兩個 dataset 直接給出前後價，係數 = `after_price / before_price`，不需要自己從股利金額推算：

- `TaiwanStockSplitPrice`：全市場 2015-01-01～2026-08-10 只有 **33 筆、31 檔**，
  其中命中我們目前有資料的標的只有 **1 筆**——`0050 2025-06-18 分割 188.65 → 47.16`
  （係數 0.2500）。**一次批次請求就抓得完整個歷史。**
- `TaiwanStockDividendResult`：2330 在 2023～2025 有 **12 筆**（約每年 4 次），
  例：`2023-03-16 511.0 → 508.25`。**只能逐檔抓**，受 `finmind.rate_limit`
  （每分鐘請求數，預設 5）節流。

**分割與除權息的性質不同，修復優先度也不同**：

- **分割**造成的是**單根災難性假跳空**（0050 的 −75%）——罕見但足以讓 breakout / MAE
  之類的判斷完全失真。33 筆就能全部處理掉。
- **除權息**造成的是**緩慢累積的偏移**——2330 每次僅約 0.5%，但一年四次，十年累積約
  20%。單根不會觸發假訊號，但會系統性扭曲長期回測的報酬與波動。

**修復方向（未定案，需另立計畫）**：

1. ~~抓取端改存還原股價~~ ——**已排除**，`TaiwanStockPriceAdj` 需 Sponsor tier。
2. **自行維護還原係數表**（新表存 symbol / 事件日 / before_price / after_price / 係數），
   讀取端套用累積係數。資料來源就是上表那兩個 dataset，現有 tier 拿得到。
   代價：每個消費者都要記得套，容易漏——需要在讀取層（`store.CandleRepo` 或 Python
   的 dataset 載入）統一處理，而不是散在各處。
3. 或在寫入時就存還原後價格（另存原始價欄位）。好處是消費者不用改；代價是每次有新的
   公司行動就要回頭改寫歷史列，且「當時實際成交價」會失真。
4. 無論哪種，都要決定既有歷史資料如何回填／重抓。

**規模提醒**：逐檔抓除權息在 `rate_limit=5`（每分鐘）下，目前 28 檔約 6 分鐘；
但 T-040 要擴到 100～200 檔、最終全市場約 1,900 檔時，一次全量回補是數小時等級，
需要設計成增量更新而非每次全抓。

**現況提醒**：在此之前，**所有跨越公司行動的回測與 replay 數字都要當成不可靠**。

---

### I-066：手動回補之後還原係數不會立即重算

| 欄位 | 內容 |
|---|---|
| 狀態 | 已實作／待 review（2026-08-11） |
| 嚴重度 | 中 |
| 分類 | Go / 資料正確性 |
| 發現日期 | 2026-08-11 |
| 來源 | T-042 Phase 1 實作後的 review |

`adj_factor` 只由排程的 `corporate_action_sync`（每天 06:30）重算。但**回補可以插入比
公司行動更早的 K 棒**，而 `BulkInsert` 寫入的 `adj_factor` 是欄位預設值 1：

```
手動回補 0050 的 2020 年資料  →  那批 K 棒 adj_factor = 1（未還原）
                              →  直到隔天早上 06:30 才會被修正
```

這段期間，跨越 2025-06 分割的任何計算都會看到假跳空——**而且不會有任何東西報錯**。

**為什麼沒有在 Phase 1 一併做**：計畫（T-042）已經寫明「回補之後必須重算」，但實作只掛了
排程，沒有把重算接到回補完成的路徑上。這是實作沒有覆蓋到計畫的一個項目，不是設計取捨。

**修法**：`fetcher.BackfillHistory` 成功後、以及 `POST /market/backfill` 的 job 完成後，
對受影響的 symbol 呼叫 `Adjuster.RecomputeSymbol`。重算是冪等的（見 T-042），
所以多呼叫幾次沒有副作用；漏呼叫才是問題。

**偵測**：`scripts/verify-adjustment.sh` 的檢查 3 會抓到（實際係數與獨立重算不符）。

#### 已完成（2026-08-11）

`Fetcher` 新增選填的 `adjuster`（`SetAdjuster`），`BackfillHistory` 蒐集成功寫入的標的，
回補結束後呼叫 `Adjuster.RecomputeAffected`。兩個回補入口（排程的 `RunPreMarket`
與 `POST /market/backfill`）都走 `BackfillHistory`，所以一處就全覆蓋。

**只掛在回補、不掛在每日抓取**：每日抓取寫入的是最新一根 K 棒，位置在所有公司行動之後，
係數本來就是 1；只有回補會插入比事件更早的 K 棒。

**`RecomputeAffected` 先過濾出「有事件」的標的**，而不是無腦對每個回補過的 symbol 重算——
後者對沒有事件的檔也會執行一次整段歸零的 UPDATE，回補 200 檔就是 200 次全表掃描，
而全市場 33 筆事件只涵蓋 31 檔。

**重算失敗不算進回補的 failed 計數**：K 棒**已經寫進去了**，把它算成「回補失敗」會誤導
呼叫端去重抓。改成記 Error log，並靠隔天排程與 `verify-adjustment.sh` 補救。

測試：`TestBackfillRecomputesAdjFactor`（回補後係數立即正確）、
`TestBackfillWithoutAdjusterStillWorks`（未掛 adjuster 不 panic）、
`TestRecomputeAffectedSkipsSymbolsWithoutEvents`（沒有事件的標的不被動到）。

---

### I-067：既有的衍生資料仍以未還原價計算

| 欄位 | 內容 |
|---|---|
| 狀態 | 已知限制（刻意，T-042 明列為不做的範圍） |
| 嚴重度 | 低 |
| 分類 | 資料一致性 |
| 發現日期 | 2026-08-11 |
| 來源 | T-042 Phase 1 實作後的 review |

`indicator_snapshots`、`stock_analyses`、`stock_sr_zone_analyses` 裡既有的列，都是在還原
功能上線**之前**用原始價算出來的。上線後新產生的列會用還原價，於是同一張表裡會有兩種
基準的資料並存。

**為什麼刻意不回頭重算**：`stock_analyses` 與 `stock_sr_zone_analyses` 是「當時做了什麼
判斷」的紀錄，不是快取——回頭改寫會讓歷史紀錄與當初的決策對不上。
`indicator_snapshots` 比較接近快取，會在下次計算時自然被覆蓋。

**實務影響有限**：只有跨越公司行動的窗口會有差異，而目前有資料的標的裡只有 0050 受影響
（全市場 33 筆事件命中我們的只有 1 筆）。

**要注意的時機**：擴充標的池（T-040）之後受影響的檔數會變多，屆時若要做跨期比較，
要記得舊列的基準不同。

---

### I-068：Yahoo 的除權息資料把息與權合併，兩者不同天時還原係數會算錯

| 欄位 | 內容 |
|---|---|
| 狀態 | 已知限制（刻意接受，2026-08-11 決定） |
| 嚴重度 | 低（實測 146 筆中 1 筆受影響） |
| 分類 | 資料品質 / 外部資料源 |
| 發現日期 | 2026-08-11 |
| 來源 | T-042 Phase 2 的資料源查證 |

Phase 2 的除權息來源是 Yahoo 的 `StockServices.dividendsByYear`，其 URL 參數字面上就是
`action=combineCashAndStock`——它把同一年的現金股利與股票股利**合併成一筆**，並掛在
其中一個除權息日上。**當除息日與除權日不同天時，現金的調整會被套到錯誤的日期。**

實測案例（2891，2016-10-12）：

| 來源 | 內容 | 係數 |
|---|---|---|
| Yahoo | prev 18.14、cash 0.81、stock 0.80（合併成一筆） | 0.884581 |
| FinMind `TaiwanStockDividendResult` | 標為**權**、18.15 → 16.80 | 0.925620 |

差 **4.1%**。FinMind 的 16.80 = 18.15 ÷ 1.08 精確吻合官方除權參考價，代表那天
**只有除權沒有除息**，現金的除息日是另一天。

**為什麼接受**：跨 10 檔 146 筆事件的交叉比對中，只有這 1 筆差異超過 1e-3；
其餘純現金的案例與 FinMind 精確到浮點極限（0050 差 2e-16、2308 差 0）。
影響是「某一天的價格少調整了幾個百分點」，不會產生假跳空那種等級的失真。

**日後要修的話**：FinMind 的 `TaiwanStockDividendResult` 把息與權分成不同列
（實測 2891 的分佈是「息 22、權 14、權息 1」），逐檔查詢在現有 `register` tier
可用（只有**整批**查詢需要 Sponsor）。可以拿它當第二來源交叉驗證，
兩邊係數不一致時報出來——與 `scripts/verify-adjustment.sh` 用 SQL 獨立重算的思路相同。

**偵測**：目前沒有自動偵測。要做的話就是上面那條交叉驗證。

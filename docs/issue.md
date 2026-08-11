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

---

### I-069：減資／合併造成的價格重訂未被還原

| 欄位 | 內容 |
|---|---|
| 狀態 | 減資已實作／待部署驗證；合併與下市重編仍不涵蓋（2026-08-11） |
| 嚴重度 | 中 |
| 分類 | 資料品質 / 回測正確性 |
| 發現日期 | 2026-08-11 |
| 來源 | T-042 上線後的 live 驗證 |

股價還原（見 [`database-schema.md`](./database-schema.md) 的「股價還原」）涵蓋分割與除權息，
**不涵蓋減資、合併、下市重編**——兩個資料源都沒有這類事件。

live 實測殘留（還原**之後**仍超過單日漲跌幅上限 ±10%，因此不可能是真實行情）：

| 標的 | 日期 | 前一根 | 變動 | 中斷天數 |
|---|---|---|---|---|
| 6243 | 2021-11-04 | 10-27 | **+126.8%** | 8 |
| 2603 | 2022-09-19 | 09-06 | **+109.2%** | 13 |
| 2478 | 2019-10-07 | 09-25 | +36.3% | 12 |

**判定方法**（可直接拿去找新的案例）：

```sql
with adj as (
  select symbol, ts, close*adj_factor as p,
         lag(close*adj_factor) over (partition by symbol order by ts) as prev,
         lag(ts) over (partition by symbol order by ts) as prev_ts
  from candles where timeframe='1d')
select symbol, (prev_ts at time zone 'Asia/Taipei')::date, (ts at time zone 'Asia/Taipei')::date,
       round((p/prev-1)*100,1) as pct, (ts::date - prev_ts::date) as gap_days
from adj where prev > 0 and abs(p/prev-1) > 0.25 order by abs(p/prev-1) desc;
```

三筆的模式完全一致：**交易中斷數日 ＋ 價格重訂**。台股單日漲跌幅上限是 ±10%，
所以「還原後仍超過 10%」本身就是未處理公司行動的指紋；中斷天數則區分它與單純的資料缺漏。

**影響**：跨越這些日期的窗口，指標與回測結果不可靠。目前有資料的標的中影響 3 檔。

#### 資料源已查證（2026-08-11）：FinMind 逐檔查詢可用

`TaiwanStockCapitalReductionReferencePrice`，**與除權息同一個模式——整批需 Sponsor tier，
但逐檔（帶 `data_id`）在現有 `register` tier 就能用**。

```
GET /api/v4/data?dataset=TaiwanStockCapitalReductionReferencePrice&data_id=2603&…
{"date":"2022-09-19","stock_id":"2603",
 "ClosingPriceonTheLastTradingDay":80.8,"PostReductionReferencePrice":187.0,
 "OpeningReferencePrice":187.0,"LimitUp":205.5,"LimitDown":168.5,
 "ExrightReferencePrice":-1.0,"ReasonforCapitalReduction":"…"}
```

係數 = `PostReductionReferencePrice / ClosingPriceonTheLastTradingDay`。
減資讓股數變少、價格變高，所以**係數 > 1**（與反分割同向），成交量係數相同（股數確實改變）。

三個已知案例全部命中，且 `ClosingPriceonTheLastTradingDay` 與我們 DB 裡的前一根收盤價
**完全相同**——這是資料源正確性的獨立佐證：

| 標的 | 日期 | 停前收盤（來源／我方） | 減資後參考價 | 係數 |
|---|---|---|---|---|
| 6243 | 2021-11-04 | 22.75 / **22.75** | 46.95 | 2.0637 |
| 2603 | 2022-09-19 | 80.80 / **80.80** | 187.00 | 2.3144 |
| 2478 | 2019-10-07 | 35.80 / **35.80** | 44.40 | 1.2402 |

另發現一筆先前未列出的：2478 於 2016-10-21（17.65 → 20.99，+18.9%），
因低於當初 25% 的篩選門檻而沒出現在上表。

#### 已排除的來源

| 來源 | 結論 |
|---|---|
| FinMind `TaiwanStockCapitalReductionReferencePrice` **整批** | ❌ 需 Sponsor tier |
| FinMind 其他 dataset | ❌ 完整目錄（104 個，台股 61 個）裡減資只有這一個，**合併則完全沒有** |
| TWSE `exchangeReport/TWTAUU` | ⚠️ 欄位齊全且免費，但**只服務當年度**——跨年區間會被靜默截斷成當年，純過去的區間回一個與實際原因不符的錯誤（「查詢結束日期小於查詢開始日期」）。可用於**日後新增**，不能回補歷史 |
| Yahoo `dividendsByYear` | ❌ 只有除權息 |

#### 已實作（2026-08-11）

`FinMindClient.FetchCapitalReductions` 逐檔查詢，寫進既有的 `corporate_actions`
（`action_type = CAPITAL_REDUCTION`），由既有重算流程處理。
**沒有新增表、欄位或係數概念**——減資與反分割在數學上是同一件事。

除權息與減資合併在 `Adjuster.SyncPerSymbolEvents` 的同一個迴圈，**每檔只重算一次**
（重算要 UPDATE 整段歷史，分開跑會做兩次）。兩者打不同 host，各有節流器，不互相排擠。

現況說明見 [`database-schema.md`](./database-schema.md) 的 `corporate_actions`
與 [`architecture/data-pipeline.md`](./architecture/data-pipeline.md) 的「公司行動同步」。

**權威來源比門檻式偵測多找到 2 筆**：上表那 3 筆是用「還原後單日變動 > 25%」篩出來的，
而 FinMind 逐檔查詢顯示目前標的中共有 **5 筆**——多出 2478 的 2016-10-21（+18.9%）
與 2317 的 2018-10-26。**篩選門檻會漏掉真實事件**，不能拿它當事件清單，只能拿來當異常偵測。

**減資日與除權息日目前無重疊**（實測 4 檔全部無重疊）。這點值得留意但**不是零風險**：
兩者若同日發生，會產生兩筆 `corporate_actions`（`action_type` 不同，UNIQUE 擋不住）
而被各套一次。資料源的 `ExrightReferencePrice` 欄位（目前恆為 `-1.0`）暗示來源本身
有能力表達合併事件，屆時要確認是否重複計算。

**待部署後驗證**：`scripts/verify-adjustment.sh` 的檢查 3／6 會涵蓋；
另可用 I-069 上面那段 SQL 確認殘留消失。

#### 規模警訊：FinMind 的節流器與每日抓價共用

減資是逐檔查詢，走的是 `finmind.rate_limit`（預設 **5/分**）——**與每日的 K 棒抓取同一個節流器**：

| 標的數 | 光是減資查詢就要 |
|---|---|
| 29（目前） | 5.8 分鐘 |
| 200（T-040 第一階段） | 40 分鐘 |
| 1,900（全市場） | **6.3 小時** |

除權息走 Yahoo 的節流器（20/分）不受影響，但**減資會直接排擠每日抓價的額度**。
因此增量更新對減資比對除權息更迫切——後者最多是自己慢，前者會拖累行情資料。

**合併與下市重編仍不涵蓋**——FinMind 的完整 dataset 目錄（104 個，台股 61 個）裡沒有這類資料，
TWSE 的 `TWTAUU` 只服務當年度。維持為已知限制。

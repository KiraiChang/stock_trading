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

### I-054：mysql migration 從未在真實 MySQL 上跑過，只靠比照既有寫法

| 欄位 | 內容 |
|---|---|
| 狀態 | 已知限制（受環境限制，暫不修復） |
| 嚴重度 | 中 |
| 分類 | Go / DB / migration |
| 發現日期 | 2026-08-06 |
| 來源 | T-037 B review |

`backend/internal/database/migrations/` 維護 mysql / postgres / sqlite 三份 migration，
但**只有 postgres 與 sqlite 真的被執行過**：

- sqlite：`internal/store` 測試每次都實跑 goose migration 後做 round-trip。
- postgres：dev / live compose 都用 postgres，backend 啟動時由 goose 實際套用。
- **mysql：沒有任何自動或手動路徑會執行它。** 本機沒有 MySQL 實例，dev compose 用 postgres，
  拉一個 MySQL container 需要的記憶體在這台 2 GiB host 上會踩到 I-053。

所以每一份 mysql migration 都只是「比照該檔案既有寫法撰寫」，語法錯誤、型別不相容或
backfill 漏做都不會被任何流程擋下來，要等真的有人用 MySQL 部署才會爆。

最近一例是 `057_add_sr_zone_builder_runtime_config.sql`：mysql 版走
`ADD COLUMN ... NULL` → `UPDATE ... SET 'null'` → `MODIFY ... NOT NULL` 三步（比照 033），
postgres / sqlite 則各自一步用 `NOT NULL DEFAULT`。三者最終狀態有個小差異——
**mysql 版沒有 DEFAULT**，所以省略該欄位的 INSERT 在 mysql 會失敗、在另外兩個 engine 會成功。
目前唯一的寫入路徑（`sr_zone_repo.Create`）永遠帶齊欄位且有 `== ""` 的 normalization guard，
所以不構成實際問題，但這正是「mysql 分支沒被執行過」會累積出來的那種不對稱。

**要解掉需要**：CI 或本機起一個 MySQL 實例把三份 migration 都跑一遍（up + down），
或明確宣告不再支援 MySQL 並刪掉那份 migration 目錄。兩者都不是順手能做的，故先記為已知限制。
`backend/config.yaml` 目前仍把 mysql 列為生產可選 driver。

---

### I-055：`zone_outcomes` 分層的三個比率欄位在前端永遠顯示 `—`（欄位名不一致）

| 欄位 | 內容 |
|---|---|
| 狀態 | 已修復（待 review） |
| 嚴重度 | 中 |
| 分類 | Python / Frontend / SR Zone |
| 發現日期 | 2026-08-06 |
| 修復日期 | 2026-08-06 |
| 來源 | T-039 sweep 取樣 Pass 0 的實跑結果 |

**症狀**：SR Zone evaluation report 的「Zone 層指標」分區裡，`by_role` / `by_method` /
`by_volatility_bucket` 三張分層表的**「支撐守住」「壓力壓回」「突破率」三欄永遠是 `—`**，
只有 `rows` 與「平均報酬」有值。頂層的同名三個比率是正常的。

**根因**：欄位名不一致。`_zone_outcome_group`（`evaluation.py:198`）回傳的是

```python
{"rows", "hold_rate", "break_rate", "average_forward_return"}
```

但 TS 型別 `SRZoneOutcomeGroup`（`srZones.ts:1136`）宣告、且 `SRZones.svelte` 實際渲染的是
`support_hold_rate` / `resistance_rejection_rate` / `break_positive_rate`——**三個 key 在 Python
輸出裡根本不存在**，取值得到 `undefined`，`fmtPct()` 照設計印成 `—`。

2026-08-06 實跑驗證（11 檔、5,928 筆 touch）：頂層 `support_hold_rate=0.426`、
`resistance_rejection_rate=0.318`、`break_positive_rate=0.403` 都有值，
但 `by_role` / `by_volatility_bucket` 每一組的這三個欄位全是 `None`。

**為什麼沒被任何測試擋下來**：

- 前端測試（`SRZones.test.ts:392`）的 fixture 是**手寫**的
  `by_role: { SUPPORT: { rows: 130, support_hold_rate: 0.62, ... } }`——
  用了一個 Python 從來不會產生的 key，所以測試「通過」的是一份不存在的資料形狀。
- Python 測試（`test_evaluation.py:305-306`）只斷言 `by_volatility_bucket` 非空、rows 加總正確，
  **沒有斷言任何比率欄位**。

兩邊各自為政、都沒有對照另一邊的實際輸出，於是「型別、渲染、測試」三者一致地錯。
這與 [`development-workflow.md`](./development-workflow.md) §3 記的
「沒被消費的型別還會默默寫錯」是同一類，但更難發現——**這個欄位有被消費、有被測試，
只是消費與測試的都是虛構的形狀**，而 `—` 看起來就像「這組沒資料」。

**連帶影響**：T-039 的 sweep 取樣 Pass 1 要比較的正是各候選在各 bucket 的守住率／突破率差異，
在修好之前那些欄位是 `None`，只剩 `average_forward_return` 一個維度可比。**Pass 1 應等本項修完再跑。**

**修復計畫（待確認）**

1. **`evaluation.py`**：`_zone_outcome_group` 補齊與頂層同名、同算法的三個比率——
   `support_hold_rate` 只取組內 `is_support==1`、`resistance_rejection_rate` 只取 `is_support==0`、
   `break_positive_rate` 取整組。
2. **保留 `hold_rate`**：它有真正的消費者（`_bucket_candidate_score:580` 以 0.7 權重用它排序
   bucket 建議），而且「不分支撐／壓力的 zone 守住率」本身是有意義的獨立指標。
   加註解寫明它與 `support_hold_rate` 的差別，避免下次有人誤刪。
   **本次不動 `_bucket_candidate_score` 的評分邏輯**——同時改欄位與改評分會讓 sweep 結果無從對照。
3. **移除 `break_rate`**：與新增的 `break_positive_rate` 同義，且 grep 全庫**沒有任何消費者**。
4. **`srZones.ts`**：`SRZoneOutcomeGroup` 補上 `hold_rate`，讓型別反映實際輸出。
5. **測試**：Python 斷言分層比率的**實際數值**（不只是 key 存在）；前端 fixture 改成 Python 真的
   會產生的形狀，並斷言畫面上出現**具體百分比字串**——只斷言「表格有出現」正是這次失效的原因。

**驗證**：`python/scripts/test.sh` ＋ `frontend/scripts/test.sh` 全綠，然後**重跑一次 Pass 0**
確認分層比率確實有值（這是唯一能證明修好的方式——單元測試用的是合成資料）。

**修復結果（2026-08-06）**

五項全數完成。`python/scripts/test.sh backtest/modular/sr_scoring/tests/test_evaluation.py`
→ 43 passed；`VITEST_ARGS="src/routes/SRZones.test.ts" frontend/scripts/test.sh` → 23 passed。

**決定性驗證是重跑 Pass 0**（真實資料，11 檔 / 5,928 筆 touch），三種分層的比率全部有值：

| 分層 | rows | 支撐守住 | 壓力壓回 | 突破 |
|---|---|---|---|---|
| （頂層） | 5,928 | 42.6% | 31.8% | 40.3% |
| `by_role` / SUPPORT | 2,728 | 42.6% | — | 34.3% |
| `by_role` / RESISTANCE | 3,200 | — | 31.8% | 45.5% |
| `by_method` / atr | 4,305 | 42.7% | 31.7% | 40.0% |
| `by_method` / volume_profile | 1,623 | 42.4% | 32.0% | 41.1% |
| `by_volatility_bucket` / HIGH | 4,676 | 45.3% | 34.1% | 43.8% |
| `by_volatility_bucket` / NORMAL | 1,252 | 33.7% | 22.6% | 27.2% |

`by_role` 的值與頂層完全吻合（SUPPORT 組的支撐守住 = 頂層 42.6%），互為交叉驗證；
只有一種角色的那一欄正確顯示 `None` 而非 0。`break_rate` 已從所有分層消失。

現況說明已歸檔到 [`sr-zone-scoring.md`](./sr-zone-scoring.md) 的「Zone 層分層的欄位語意」
（含 `hold_rate` 與 `support_hold_rate` 的差別、`by_role` 必有一欄為 null 是正常的），
測試面的教訓補到 [`development-workflow.md`](./development-workflow.md) §3
（fixture 要從真實輸出取樣、斷言要到值、跨語言契約最終要靠實跑驗證）。

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

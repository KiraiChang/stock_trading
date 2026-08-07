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

---

### I-059：decision replay 對真實 DB 跑時，chip 的 `trade_date` 讓 JSON 序列化整個失敗

| 欄位 | 內容 |
|---|---|
| 狀態 | **已修復／待 review**（2026-08-07，已對真實 DB 驗證） |
| 嚴重度 | 高（decision replay 對有籌碼資料的標的**必定失敗**，含排程／API 的 `--write-db` 路徑） |
| 分類 | Python / SR Zone / 跨語言契約 |
| 發現日期 | 2026-08-07 |
| 來源 | T-028 用 live DB 的 29 檔實跑 decision replay |

**症狀**：`--decision-replay` 對 live DB 跑完整個 walk-forward（11 檔約 30 分鐘）後，
**在最後一步序列化時才失敗**：

```
TypeError: Object of type date is not JSON serializable
  evaluation.py:2272  encoded = json.dumps(report, indent=2, ensure_ascii=False)
```

**根因**：`scoring.py:562` 的 `_build_chip_summary` 把 chip row 的 `trade_date`
**原樣**塞進輸出：

```python
"trade_date": chip_row.get("trade_date"),
```

live `chip_scores.trade_date` 的欄位型別是 `date`，psycopg2 讀出來就是
`datetime.date` 物件。同一份 chip_summary 會被嵌進 decision replay 的每一列
（`daily_confirmation_context`），於是整份 report 無法序列化。

**為什麼測試沒擋下來**：測試 fixture 的 `trade_date` 一律是**字串**
（`{"trade_date": "1970-01-01", ...}`），而真實 DB 給的是 `date` 物件。
**這與 2026-08-06 的 `zone_outcomes` 欄位名事件是同一類問題**（教訓見
[`development-workflow.md`](./development-workflow.md) 的「測試 fixture 必須是後端真的會
產生的形狀，斷言必須到值」）——fixture 用了後端真實路徑不會產生的東西，所以三層測試全綠，
但只要接上真的資料就爆。**那次是 key 名不一致，這次是型別不一致**——同一條規則的兩種踩法，
代表「fixture 從真實輸出取樣」這條要求還沒有被落實到位。

**影響範圍不只 CLI**：`write_evaluation_result`（`evaluation.py:426`）也是同一個
`json.dumps(report)`，所以**排程或 API 觸發、帶 `--write-db` 的 decision replay
會以同樣方式失敗**。只有在該標的完全沒有籌碼資料時才不會踩到
（`chip_row is None` → `_missing_chip_summary` 的 `trade_date` 是 `None`）。
live 有 31 檔、34,260 筆籌碼分數，所以實務上一定會踩到。

**代價**：失敗發生在最後一步，前面數十分鐘的運算全部白跑。

**修復（2026-08-07）**：新增 `scoring._iso_date()`，在 `_build_chip_summary` 把
`trade_date` 正規化成 ISO 字串。**刻意不用 `json.dumps(default=str)` 這種全域逃生口**
——那會讓下一個型別洩漏同樣無聲無息地混進 API 回應。`_iso_date` 以 `isoformat` 是否可呼叫
來判斷，所以 postgres 的 `date`、sqlite 的字串都吃得下。

測試 `test_build_chip_summary_trade_date_is_json_serializable`：fixture 直接用
`datetime.date(2026, 8, 6)`（psycopg2 真正會給的型別），斷言輸出是 `"2026-08-06"`、
**整份 summary 可 `json.dumps`**，另外覆蓋字串輸入原樣保留與查無資料時維持 `None`。

**驗證**：`python/scripts/test.sh` 的 scoring 測試 76 passed；並對 **live DB 實跑**
decision replay（2 檔 / limit 400）確認 `trade_date` 輸出為 ISO 字串且整份 report
序列化成功。

**這次學到的操作經驗**：這條路徑跑一次要數十分鐘且失敗在最後一步，所以**改完先用
小 `--limit` ＋ 少量標的做一次快速預檢**（2 檔 / limit 400 約 2 分鐘），確認真實 DB 的
型別都過得了序列化，再投入完整規模的跑。

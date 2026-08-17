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
- **編號只增不重用。** 已移除的條目編號不得再發給新問題——程式碼註解與其他文件會留著舊 ID，
  重用會讓兩件無關的事共用一個代號。**`I-070` 已經發生過一次**（先發給 T-045 的事件鏈墓碑，
  移除後又發給 T-040 的 `keep_symbols` 靜默丟棄，兩筆現在都已收斂），
  見 `todo.md` T-045 那段的註記。
- **下一個新編號從 `I-073` 起算。** 本檔目前最大是 I-069，但 **I-070～I-072 都發放過**
  （T-045 那批，在同一個工作階段內建立又移除，git 歷史裡看不到）。
  **不要用「檔案裡最大值 + 1」決定編號**——被移除的條目正是看不見的那些。
- **移除條目前要先反轉依賴。** 主題文件與程式碼註解常寫「見 issue.md I-0xx」，
  這種寫法讓 issue.md 變成權威來源，一刪就斷鏈。移除前先把說明**內嵌**到對應主題文件，
  再把所有引用改指向該文件。收斂後用下面這條檢查沒有殘留：

  ```bash
  comm -13 <(grep -o '^### I-0[0-9][0-9]' docs/issue.md | sed 's/### //' | sort -u) \
           <(grep -rho "I-0[0-9][0-9]" --include="*.md" --include="*.go" --include="*.py" \
              --include="*.sh" . | grep -v node_modules | sort -u)
  ```

  列出的 ID 必須**只剩明確標為歷史沿革的引用**（「原記於…」「當時編號…」），
  不能有任何「見 I-0xx」形式的活指標。

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

### I-054：mysql 的執行期支援仍未被驗證（DDL 已驗，CRUD 未驗）

| 欄位 | 內容 |
|---|---|
| 狀態 | 已知限制（部分已解，剩餘部分暫不修） |
| 嚴重度 | 低（目前無人以 MySQL 部署） |
| 分類 | Go / DB / migration |
| 發現日期 | 2026-08-06（2026-08-07、2026-08-12 兩次縮小範圍） |
| 來源 | T-037 B review |

**DDL 部分已解**：mysql migration 有可重複執行的驗證路徑
（`scripts/test-mysql-migrations.sh`，2026-08-12 起共三支測試），全部 migration 都能
up 到最新並 down 回 0。用法、測試清單與命名限制見
[`development-workflow.md`](./development-workflow.md)；欄位命名規範與現況欄寬見
[`database-schema.md`](./database-schema.md)。

**仍未解的三項**：

1. **驗證仍不涵蓋 repo 層的 CRUD round-trip。** 「表建得起來」不等於「INSERT/SELECT 跑得動」。
   `CorporateActionRepo.Upsert` 的 mysql 分支（`ON DUPLICATE KEY UPDATE`）已有真實 MySQL 的
   執行證明，但那是**唯一**被涵蓋的 repo 寫入路徑，其餘 `internal/store` 的查詢仍只跑 sqlite。
   要補的話是讓 repo 測試整批對著真實 MySQL 跑一輪。
2. **`time.Time` 寫進 DATE／DATETIME 的時區處理，mysql 與 postgres 不一致**（2026-08-12 發現）：
   `go-sql-driver` 寫入前會 `v.In(cfg.Loc)`（`connection.go:262`），驗證用 DSN 沒帶 `loc` 即 UTC，
   所以**台北午夜會被存成前一天**；`pgx` 取的是值本身時區的日曆日
   （`pgtype/date.go:164`），存進去就是那天。受影響最明顯的是
   `corporate_actions.event_date`——語意是「新價的第一個交易日」，adjuster 用 `ts < event_date`
   決定套用範圍，差一天等於係數套錯一根 K 棒。這正是第 1 項所預言的那類問題。
   目前不修：mysql 沒有任何部署，且正確的修法（DSN 帶 `loc`／改成傳日期字串）要連同
   第 1 項的整批驗證一起做，才不會只修掉看得見的那一個欄位。
3. **`057_add_sr_zone_builder_runtime_config.sql` 的 DEFAULT 不對稱**：mysql 版走
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

| 標的數 | 原始 frames | touch dataset | 峰值 | 單次耗時 |
|---|---|---|---|---|
| 11（現況） | ~1MB | ~1MB | **270MB（實測）** | **70 秒（實測）** |
| 10 | — | 6,032 rows | **281MB（實測 2026-08-12）** | — |
| 20 | — | 11,859 rows | **281MB（實測）** | **131 秒（實測）** |
| 30 | — | 17,447 rows | **317MB（實測）** | **191 秒（實測）** |
| 40 | — | 22,401 rows | **310MB（實測）** | **241 秒（實測）** |
| 150 | ~14MB | ~13MB | 約 420MB（外推） | 約 14 分鐘（外推） |
| 2,298（全市場） | ~220MB | ~200MB | 約 0.8～1.2GB（**未實測**） | 約 4 小時 |

**2026-08-12 的實測校準了上面的推論**（方法與完整判讀見 [`todo.md`](./todo.md) T-040
「Step 0 實測結果」）：

- **本筆對「270MB 幾乎都是 import 開銷」的判斷正確**。標的數 10→40（4 倍）、
  rows 6,032→22,401（3.7 倍），峰值只從 281MB 增到 310MB。**邊際成本約 1.0 MB/檔。**
- 所以正確的外推方式是「**固定基線 ＋ 線性邊際**」，只外推量到的那 ~30MB 資料相依部分；
  外推總量（例如把 270MB 乘以標的倍數）會高估一個量級，那正是本筆一開始要糾正的錯誤。
- 量測噪音約 ±7MB（N=30 的 317MB 高於 N=40 的 310MB，但兩者是超集關係）——
  cgroup v1 的峰值含 page cache。
- **量測工具**：`MEASURE_PEAK=1 scripts/run-evaluation.sh`。峰值由容器在退出前自報
  cgroup 單調最大值，**不能從外面輪詢**——`_volatility_profiles`（就是本筆講的
  「sources 與 dataset 同時常駐」那一段）跑在流程最後，輪詢會系統性低估。

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

**現況結論**（2026-08-12 依實測更新）：擴到 **150 檔可以先不改造**，推估峰值約 420MB；
但**上限是 150 檔不是 200 檔**——200 檔推估 470MB，加上 150MB 保留需要 620MB available，
在這台 host 上實質不可行。而且 150 檔也**要求執行當下不常駐 gitea 那一級的服務**
（實測 gitea 常駐時 available 只有 398MB，mem-guard 直接擋下、連 10 檔都跑不起來；
背景見 `development-workflow.md`「container 上限的總和也要顧」，補上限的待辦是 todo.md T-046）。**全市場路線仍必須先做第 1、2 項。**

---

### I-069：合併與下市重編造成的價格重訂未被還原

| 欄位 | 內容 |
|---|---|
| 狀態 | 已知限制（無資料源） |
| 嚴重度 | 低（目前標的中未觀察到實例） |
| 分類 | 資料品質 / 回測正確性 |
| 發現日期 | 2026-08-11 |

股價還原（見 [`database-schema.md`](./database-schema.md) 的「股價還原」）已涵蓋
**分割、反分割、面額變更、除權息、減資**，但**不涵蓋合併與下市重編**。

**沒有資料源**：FinMind 的完整 dataset 目錄（104 個，台股 61 個）裡沒有這類資料；
TWSE 的 `exchangeReport/TWTAUU` 欄位齊全但**只服務當年度**（跨年區間會被靜默截斷成當年，
純過去的區間回一個與實際原因不符的錯誤「查詢結束日期小於查詢開始日期」）。

**目前標的中沒有觀察到實例**——2026-08-11 減資上線後，全庫已無未解釋的假跳空（見下）。

#### 偵測方法與它的例外

```sql
with adj as (
  select symbol, ts, close*adj_factor as p,
         lag(close*adj_factor) over (partition by symbol order by ts) as prev,
         lag(ts) over (partition by symbol order by ts) as prev_ts
  from candles where timeframe='1d')
select symbol, (prev_ts at time zone 'Asia/Taipei')::date, (ts at time zone 'Asia/Taipei')::date,
       round((p/prev-1)*100,1) as pct, (ts::date - prev_ts::date) as gap_days
from adj where prev > 0 and abs(p/prev-1) > 0.15 order by abs(p/prev-1) desc;
```

原理是「台股單日漲跌幅上限 ±10%，還原後仍超過就代表有未處理的公司行動」。
**但有兩個例外，不先排除會追到不存在的問題**：

1. **國外成分證券 ETF 不受 ±10% 限制**。實測 `00830` 在 2025-04-07 為 −20.6%、
   04-10 為 +19.1%——但**同日 28 檔全部同向**（04-07 平均 −10.0%、04-10 平均 +9.7%），
   是 2025 年 4 月關稅衝擊的市場性事件，不是資料問題。
   **判別方法：看同一天其他標的動了沒。**
2. **門檻本身會漏掉真實事件**。當初用 25% 只找到 3 筆減資，權威來源實際有 **7 筆**
   （多出 2412、2478 的 2016 那筆、2609、2317）。門檻只能當異常偵測，不能當事件清單。

---


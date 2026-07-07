# TODO：優化與待實作項目

記錄想做但還沒做的優化方向、功能擴充、架構升級。跟 bug/矛盾/限制無關的項目
放這裡；已經發生的問題或已知限制記錄在 [issue.md](./issue.md)。

## 使用說明

- **狀態**：`待規劃` / `規劃中` / `進行中` / `已完成` / `擱置`
- **優先度**：`高` / `中` / `低`（主觀評估，會隨情境調整，不是嚴格排序）
- 新增項目時往下加一筆，編號遞增（`T-0xx`），不要覆蓋舊編號。
- 項目狀態改變時直接更新該筆的「狀態」欄位，不需要搬移位置；若項目已完成
  且不需要保留歷史，可以整筆刪除或搬到文件最下方的「已完成封存」。

---

### T-001：拆分 `python/backtest/modular/sr_scoring/scoring.py`

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 低 |
| 分類 | Python / SR Zone |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/sr_zone_improve.md`「建議優先處理順序」第 6 項 |

`scoring.py` 目前同時負責機率正規化、confidence/recent validation、tier 排序、
overlap grouping、trading score/recommendation、global metrics、API response
序列化，單檔責任偏重。文件中明確標註「視後續維護成本」才拆，不是立即要做。
建議拆法：`scoring_rules.py`（各項分數規則）、`ranking.py`（tier/排序/分群）、
`serialization.py`（API response 組裝）。

---

### T-002：SR Zone 機率模型自動化回測 pipeline

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 中 |
| 分類 | Python / SR Zone / 模型驗證 |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/sr-zone-scoring.md` 已知限制 |

目前只有 `train.py` 手動訓練 + train job metrics（time-split holdout、校準、
dataset diagnostics），沒有「模型上線後過去一段時間的訊號實際表現如何」的
自動化回測驗證。可以考慮串接既有 `backtest/modular` 引擎，定期用 SR Zone
訊號跑一次回測並記錄下來，跟訓練時的 metrics 對照。

---

### T-003：ATR zone 寬度乘數依個股特性調校

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 低 |
| 分類 | Python / SR Zone / Zone Builder |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/sr-zone-scoring.md` 已知限制 |

`atr_width_multiplier`、`max_merge_width_multiple` 目前是全域固定預設值，
沒有依個股的波動特性（例如高波動的中小型股 vs 低波動的權值股）系統化調整。

---

### T-004：籌碼分析 Phase 2 擴充指標

| 欄位 | 內容 |
|---|---|
| 狀態 | 擱置 |
| 優先度 | 低 |
| 分類 | Go / 籌碼分析 |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/chip-analysis-design.md` |

股權分散表、董監持股、借券/當沖比、大戶散戶持股比例。設計文件明確標註
「Phase 2 再評估，不應阻塞 Phase 1」，目前 Phase 1（三大法人、融資融券、
分點、綜合籌碼分數）已完整上線。

---

### T-005：Fugle 即時行情盤中更新訊息格式驗證

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 中 |
| 分類 | Go / 即時行情 |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/fugle-integration.md` Roadmap |

目前 Fugle WebSocket 盤中更新的訊息格式解析未在實盤交易時段實際驗證過，
需要在開盤時段跑一次確認欄位、頻率、斷線重連行為符合預期。

---

### T-006：Fugle Tier 1 REST 輪詢掃描接上排程器

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 中 |
| 分類 | Go / 即時行情 / 排程 |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/fugle-integration.md` Roadmap、`docs/architecture.md` |

Tier 1（非熱門股）用 REST 輪詢掃描的機制已設計但尚未實際掛上排程器
（`internal/scheduler`）自動執行。

---

### T-007：Fugle Tier 2 熱門股 WebSocket 訂閱管理

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 中 |
| 分類 | Go / 即時行情 |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/fugle-integration.md` Roadmap |

Tier 2（熱門股）動態訂閱/取消訂閱 WebSocket 頻道的管理邏輯尚未實作
（目前只有靜態 client，見 `docs/CLAUDE.md` 提到的重複連線問題修正）。

---

### T-008：Fugle → FinMind 自動 fallback

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 中 |
| 分類 | Go / 即時行情 |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/fugle-integration.md` Roadmap |

Fugle 連線失敗或資料異常時，尚未實作自動切換回 FinMind 補資料的邏輯，
目前需要人工介入。

---

### T-009：導入 Shioaji tick-level streaming 取代批次量能計算

| 欄位 | 內容 |
|---|---|
| 狀態 | 擱置 |
| 優先度 | 低 |
| 分類 | Go / 即時行情 / Phase 2 |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/architecture.md`、`CLAUDE.md` Phase 2 Roadmap |

目前量能計算是批次（K棒收盤後）而非 tick-level 即時累加，`CLAUDE.md`
Roadmap 中列為 Phase 2（Shioaji 整合）項目，非近期規劃。

---

### T-010：訊號引擎假突破過濾

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 中 |
| 分類 | Go / 訊號引擎 |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/signal-spec.md` Phase 2 計畫 |

目前 Phase 1 沒有假突破過濾機制。Phase 2 計畫加入：收盤確認、連續 2 根
K棒持有、RSI < 80 等條件，降低突破訊號的假陽性。

---

### T-011：多檔回測改為共用資金的投資組合回測

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 中 |
| 分類 | Python / 回測引擎 |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/backtest-modular-strategy.md` 已知限制 |

目前多檔股票回測是每檔獨立跑（各自有自己的模擬資金），不是真正共用同一筆
資金池、會互相排擠部位的投資組合回測。

---

### T-012：Volume Profile 改用盤中 tick 資料

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 低 |
| 分類 | Python / 回測引擎 / SR Zone |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/backtest-modular-strategy.md` 已知限制 |

目前 Volume Profile 用單一 typical price `(H+L+C)/3` 近似整根K棒的成交
分布，沒有盤中 tick 資料可用時的權宜做法。Shioaji tick 資料到位後
（見 T-009）可以改成更精確的價位分布。

---

### T-013：CLAUDE.md Roadmap 長期項目彙整

| 欄位 | 內容 |
|---|---|
| 狀態 | 擱置 |
| 優先度 | 低 |
| 分類 | 架構 / 長期規劃 |
| 建立日期 | 2026-07-07 |
| 來源 | `CLAUDE.md` Roadmap Phase 2~4 |

`CLAUDE.md` 定義的長期方向，目前都還沒排入近期工作：

- Phase 2：Dashboard 升級
- Phase 3：Portfolio tracking、Position management、Strategy templates
- Phase 4：Semi-auto execution（optional）、Risk engine enhancement

（Phase 2 的 Shioaji 整合、假突破過濾已拆成 T-009/T-010 個別追蹤。）

---

### T-014：評估籌碼訊號在 trading_score 雙重計入的影響

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 中 |
| 分類 | Python / SR Zone / 模型 |
| 建立日期 | 2026-07-07 |
| 來源 | 審視 commit `07da5c2`「調整模型以及納入籌碼分析到模型內」時發現，另見 [sr-zone-v3-chip-model-update.md](./sr-zone-v3-chip-model-update.md)「後續驗證」 |

commit `07da5c2` 把籌碼分數（`chip_total_score`/`chip_institutional_score`/
`chip_margin_score`/`chip_broker_score`/`chip_concentration_score`/
`chip_missing`）加進 ML 模型的訓練特徵（`FEATURE_COLUMNS`，`MODEL_VERSION`
bump 到 `v3`），讓模型自己學籌碼跟 bounce/break 機率的關係。但
`TRADING_SCORE_WEIGHTS` 裡原本（`6660e97`「調整分析結果增加可讀性」加入的）
獨立 `chip` 權重（15%）並沒有拿掉或調整——現在籌碼訊號會透過兩條路徑影響
最終 `trading_score`：

1. 模型特徵 → 影響 `bounce_probability`/`break_probability` → 影響
   `expected_value`（34%）與 `support_score`/`resistance_score`
2. 獨立的 `chip` 加權分量（15%，直接用原始 `chip_score`）

兩條路徑方向通常一致（籌碼偏多時兩邊都會加分），實際效果可能是籌碼訊號
被放大超過原本設計的 15% 權重，且放大幅度不透明（取決於模型學到的係數，
無法從權重常數直接看出）。

**建議做法**（幾個方向，需要實際訓練/回測資料佐證才能決定）：

- 方案A：拿掉獨立的 `chip` 加權分量，完全交給模型學習籌碼與機率的關係，
  `TRADING_SCORE_WEIGHTS` 五個分量的權重比例恢復或重新分配到 100%。
- 方案B：保留兩條路徑，但重新訓練後比較「含 chip 特徵」vs「不含 chip
  特徵」兩版模型在相同資料集上的表現（AUC/brier/calibration），確認
  雙重計入沒有讓模型過度依賴籌碼、犧牲其他特徵的訊號。
- 方案C：維持現狀，但至少要把「chip 現在有兩條路徑影響分數」寫進
  `docs/sr-zone-scoring.md`，避免文件跟行為長期不一致（對應註解問題已先行修正，可視為這個方案的最小可行版本）。

在還沒做實驗確認之前，先不要假設哪個方案比較好；這筆先記錄現象，決定
方向後再展開實作。

---

### T-015：SR Zone 短中長摘要選價規則優化

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 中 |
| 分類 | Python / SR Zone / Frontend |
| 建立日期 | 2026-07-07 |
| 來源 | 支撐壓力頁面檢視 |

目前短 / 中 / 長不是用時間週期切分，而是依 zone 寬度分 tier：長期是最寬 1/3、中期是中間 1/3、短期是最窄 1/3。每個 tier 內支撐候選必須在現價下方、壓力候選必須在現價上方，再依 `trading_score`、`confidence`、位置排序各挑一個。

這套做法可避免摘要價位與現價位置矛盾，但 `trading_score` 會壓過「距離現價」：某個分數很高但距離較遠的 zone，可能排在較近但分數略低的 zone 前面。後續應明確決定摘要選價策略：

- 分數優先：維持現況，顯示模型評分最高的區間，但 UI 要說清楚不一定最近。
- 距離優先：先挑離現價最近的一到數個候選，再用分數與信心排序，偏短線操作。
- 混合排序：用 `trading_score`、`confidence`、距離現價、`confluence_count` 組成摘要專用排序，避免單一分數主導。

---

### T-016：SR Zone tips 改為技術分析小知識

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 中 |
| 分類 | Python / SR Zone / Frontend / UX |
| 建立日期 | 2026-07-07 |
| 來源 | 支撐壓力頁面檢視 |

目前 `analysis_tips` 混有使用者有幫助的市場解讀，以及產品/實作說明，例如「預設只列短中長期各一個支撐與壓力」、「完整 zone 可在明細展開」、「模型找不到符合現價位置的合理區間」。使用者看到 tip 時，期待的是分析輔助或技術分析知識，不應看到資料結構、篩選規則或 UI 操作說明。

後續應把 tip 定位成「盤勢閱讀小知識」，例如：

- 支撐不是買點：接近支撐時仍要觀察量能、K 線收盤與籌碼是否配合。
- 壓力不是放空點：壓力區若被放量站上，常代表原壓力可能轉為支撐。
- 區間越窄越適合短線觀察，區間越寬越偏向中長期結構。
- 多方法共振代表不同算法看到相近價位，但仍需要後續 K 棒確認。
- 低信心不是看空，而是樣本少或近期未驗證，應等待確認。

---

### T-017：Watching 進場點升級為機率模型

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 低 |
| 分類 | Python / stock-analysis / 模型 |
| 建立日期 | 2026-07-07 |
| 來源 | 原 `docs/issue.md` I-010，2026-07-07 分流至 todo |

個股分析的「Watching」（觀察中）進場點目前是規則式近似（`analysis.py`
`_watching_entry`）：純用「趨勢方向 + 離現價最近的支撐/壓力價位」挑一個該盯的
價位，沒有機率、期望值或風險報酬比。相對地，隔壁 SR Zone 已經是訓練過的機率
模型，會輸出 bounce/break probability。這筆是把 Watching 也升級成機率模型的規劃
（屬功能擴充，不是 bug）。

實作範式可**直接類比 `sr_scoring/` 既有管線**（兩者是獨立系統、獨立資料表，不共用
模型），需要新增對應元件：

- **Label 定義**：進場後 N 根K棒內是否達到目標／觸及停損（可複用
  `labeling.py::label_touch` 的「forward window + threshold」自動標籤範式，免人工標註）。
- **特徵工程**：以現有規則式指標（趨勢、S/R 距離、爆量比、ATR、pullback 容忍帶等）
  為基礎的特徵向量。
- **walk-forward dataset**：類比 `dataset.py`，逐根用「至今」資料算特徵 + 未來 label。
- **train/predict + 機率校準**：類比 `model.py`（time-split holdout、`CalibratedClassifierCV`）。
- **模型檔管理**：類比 SR Zone 的 joblib lazy singleton。

可用資料已具備：`candles` 歷史、`backtest_trades` 逐筆交易結果、SR Zone 的自動標籤
與 walk-forward 範式；缺的是專屬 Watching 的 label 定義與特徵 pipeline，而非資料本身。

---
## 已完成封存

（目前沒有項目）

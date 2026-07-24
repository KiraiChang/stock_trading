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

### T-002：SR Zone 機率模型自動化回測 pipeline

| 欄位 | 內容 |
|---|---|
| 狀態 | 擱置 |
| 優先度 | 低 |
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

註：盤中源相關工作暫不列入近期處理；目前先沿用既有資料流程，等後續有更合適
的盤中資料源或明確需求時再重新評估。

---

### T-006：Fugle Tier 1 REST 輪詢掃描接上排程器

| 欄位 | 內容 |
|---|---|
| 狀態 | 擱置 |
| 優先度 | 低 |
| 分類 | Go / 即時行情 / 排程 |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/fugle-integration.md` Roadmap、`docs/architecture.md` |

Tier 1（非熱門股）用 REST 輪詢掃描的機制已設計但尚未實際掛上排程器
（`internal/scheduler`）自動執行。

註：Yahoo 盤中源為另一個可作為 Tier-1 廣度掃描的選項，且支援單次批次多檔，兩者擇一或並列。
Yahoo 的 client／設定／排程批次路徑已實作（見 `docs/yahoo-intraday-integration.md`），僅剩
fallback（T-031）與實盤驗證（T-032）待處理。

註：盤中源相關工作暫不列入近期處理；目前先沿用既有資料流程，等後續有更合適
的盤中資料源或明確需求時再重新評估。

---

### T-007：Fugle Tier 2 熱門股 WebSocket 訂閱管理

| 欄位 | 內容 |
|---|---|
| 狀態 | 擱置 |
| 優先度 | 低 |
| 分類 | Go / 即時行情 |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/fugle-integration.md` Roadmap |

Tier 2（熱門股）動態訂閱/取消訂閱 WebSocket 頻道的管理邏輯尚未實作
（目前只有靜態 client，見 `docs/CLAUDE.md` 提到的重複連線問題修正）。

註：盤中源相關工作暫不列入近期處理；目前先沿用既有資料流程，等後續有更合適
的盤中資料源或明確需求時再重新評估。

---

### T-008：Fugle → FinMind 自動 fallback

| 欄位 | 內容 |
|---|---|
| 狀態 | 擱置 |
| 優先度 | 低 |
| 分類 | Go / 即時行情 |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/fugle-integration.md` Roadmap |

Fugle 連線失敗或資料異常時，尚未實作自動切換回 FinMind 補資料的邏輯，
目前需要人工介入。

註：與 T-031（Yahoo→FinMind fallback）共用「盤中源異常時回退 FinMind」的設計，
應規劃為單一通用的盤中源 fallback 機制，而非每個源各寫一套。

盤中源相關工作暫不列入近期處理；目前先沿用既有資料流程，等後續有更合適
的盤中資料源或明確需求時再重新評估。

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

（Phase 2 的 Shioaji 整合已拆成 T-009 個別追蹤。）

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

### T-020：Position 資料加入使用者/擁有者 scoping

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 低 |
| 分類 | Go / Portfolio / DB |
| 建立日期 | 2026-07-08 |
| 來源 | 審視 commit `37b6b4f`「加入持股操作分析」時發現 |

`positions` / `position_transactions` / `position_analyses` 沒有 `user_id`，所有登入者共用同一份
清單與分析歷史。若系統定位為單人／管理工具可接受，但需明確；若要支援多使用者，
需替兩張表補 owner 欄位、migration，並在 repo/handler 依當前使用者過濾（可參考
既有 JWT / `userRepo` 機制）。屬功能擴充，非 bug。

---

### T-028：SR Zone Daily Confirmation 回測與評估

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 中 |
| 分類 | Python / SR Zone / 模型驗證 |
| 建立日期 | 2026-07-15 |
| 來源 | SR Zone Decision Engine P2 後續限制；T-002 的子任務 |

`decision_summary.daily_confirmation` 是單筆 EOD runtime 判讀，不能代表規則已完成歷史驗證。
後續需在 SR Zone evaluation/backtest pipeline（T-002）中加入 daily confirmation label 與成效統計：

- 候選支撐隔日守住率。
- 候選壓力隔日壓回率或突破延續率。
- 兩日確認後的勝率、風險報酬分布與失效率。
- 不同量能條件、event sequence、RR gate 下的分層表現。

---

### T-031：runIntradayBatch 批次失敗時的 Yahoo→FinMind fallback

| 欄位 | 內容 |
|---|---|
| 狀態 | 擱置 |
| 優先度 | 低 |
| 分類 | Go / 即時行情 / 排程 |
| 建立日期 | 2026-07-15 |
| 來源 | `docs/yahoo-intraday-integration.md` |

Yahoo 盤中源的 client、設定、main 組裝、`scheduler.runIntradayJob → runIntradayBatch` 批次路徑
**皆已實作**（現況見 `docs/yahoo-intraday-integration.md`）。**剩餘唯一工作**：批次請求失敗
（Yahoo 被限流/封鎖）時回退補資料——目前 `scheduler.go` 的 `runIntradayBatch` 只記 log 續跑其他批次
（見該處 TODO 註解），未回退 FinMind。

設計取捨（本次已確認）：

- 僅 `finmind.intraday_enabled=true` 時才回退逐檔 FinMind 分K；`intraday_enabled=false`（預設，無
  Sponsor token）時**不回退**——FinMind 分K（TaiwanStockKBar）注定 422/tier 不足，回退只會徒耗額度。
- 回退時比照現有 `ErrInsufficientTier` 邏輯：撞到 tier 不足就整輪跳過，不對每檔重打注定失敗的請求。
- 與 T-008（Fugle→FinMind fallback）共用「盤中源異常時回退」的單一通用設計，避免各源各寫一套。

註：盤中源相關工作暫不列入近期處理；目前先沿用既有資料流程，等後續有更合適
的盤中資料源或明確需求時再重新評估。

---

### T-032：Yahoo 盤中源實盤時段驗證

| 欄位 | 內容 |
|---|---|
| 狀態 | 擱置 |
| 優先度 | 低 |
| 分類 | Go / 即時行情 / 驗證 |
| 建立日期 | 2026-07-15 |
| 來源 | `docs/yahoo-intraday-integration.md` 風險與限制 |

Yahoo 為非官方 API，上線前須於台股盤中時段（09:00–13:30）用 `cmd/yahoo-check` 實測：

- minute 陣列覆蓋率：確認 `null` 僅出現在盤前/盤後，而非 ETF（如 `0050.TW`）系統性缺值——實測盤後 `0050` 陣列全為 null 但 `2330` 正常，需釐清成因。
- 延遲：`quote.refreshedTs` vs 本地時間差。
- 封鎖風險：連續批次請求是否觸發反爬/限流，據以定 `rate_limit`/`batch_size`。

註：盤中源相關工作暫不列入近期處理；目前先沿用既有資料流程，等後續有更合適
的盤中資料源或明確需求時再重新評估。

---

### T-033：frontend 缺自動化測試框架

| 欄位 | 內容 |
|---|---|
| 狀態 | 已實作（待 review） |
| 優先度 | 中 |
| 分類 | Frontend / 測試 |
| 建立日期 | 2026-07-22 |
| 來源 | 建立 `frontend/scripts/test.sh` 時發現 |

`frontend/package.json` 只有 `dev` / `build` / `preview`，沒有 vitest、也沒有
svelte-check，所以 `frontend/scripts/test.sh` 目前唯一能做的檢查是 `vite build`
能否通過——只涵蓋「編得起來」，不涵蓋型別錯誤（`tsconfig.json` 有 `strict: true`
但沒有任何步驟真的去跑型別檢查）與元件行為。

建議分兩步：

- 先加 `svelte-check`（含 `.svelte` 內的 TS 檢查），接進 `frontend/scripts/test.sh`
  的預設流程，成本低、立刻補上型別缺口。
- 再視需求評估 vitest + @testing-library/svelte，優先覆蓋 store（`src/lib/stores/`）
  與 API 層（`src/lib/api/`）這類純邏輯。

#### 實作計畫書（2026-07-24，待 review）

決策範圍：**完整**——`svelte-check`（型別）＋ `vitest`（純邏輯單元）＋
`@testing-library/svelte`（元件渲染）三層一次建立，本輪先建框架＋種子測試，不追求全面覆蓋。

現況（已盤點）：Svelte 4 + Vite 5 + TS 5.5，純 Svelte（非 SvelteKit）。`tsconfig.json` 有
`strict: true` + `noEmit` 但無任何步驟執行；`tsconfig` 無 `paths` alias（相對 import，vitest 免設
resolve alias）。`frontend/scripts/test.sh` 在 docker `node:20-alpine`（MEM 1024m / CPUS 1）掛 repo
root、workdir `frontend`，跑 `npm ci`（optional）＋ `npm run build`；`vite.config.ts` 的 build outDir
為 `../backend/internal/ui/dist`（`emptyOutDir: true`）。程式碼量：23 `.svelte`、4 store、15 api、4 util。

**修改目標**

1. 補三種檢查並接進 `frontend/scripts/test.sh` 的預設流程：型別（svelte-check）→ 單元（vitest run）
   → build，型別/單元失敗即 fail-fast。
2. 建立可持續擴充的測試骨架＋每類一組種子測試（util、store、api、component 各至少一個），
   證明三層 harness 都能跑。

**不做的範圍（本輪）**

- 不追求 23 元件 / 15 api / 4 store 全面覆蓋；只放代表性種子，後續逐步補。
- 不引入 e2e（Playwright/Cypress）、不改 CI 平台設定（本專案以 `scripts/test.sh` docker 驗收為準）。
- 不改任何 runtime 程式邏輯或 API contract；除非 svelte-check 掃出的既有型別錯誤需修（見風險）。

**受影響檔案 / 模組**

- `frontend/package.json`：加 devDeps 與 scripts（`check` / `test:unit` / 選配 `test:watch`）。
- 新增 `frontend/vitest.config.ts`（獨立於 `vite.config.ts`，避免帶入 build outDir/`emptyOutDir`/
  manualChunks；重用 `svelte()` plugin，`environment: 'jsdom'`，載入 setup file）。
- 新增 `frontend/vitest-setup.ts`（`@testing-library/jest-dom/vitest`、testing-library 清理）。
- 修改 `frontend/scripts/test.sh`：build 前串 `npm run check` 與 `npm run test:unit`，並更新檔頭註解
  （移除「目前前端沒有測試框架」的說明）。
- 種子測試（`*.test.ts` 與元件 `*.test.ts`）：util（如 `format.ts` / `date.ts`）、store（如 `router.ts`
  或 `auth.ts`）、api（挑一個薄 wrapper，mock `client.ts` 的 fetch）、component（挑 1~2 個單純呈現型
  元件，如 layout 下的 `Topbar`/`Sidebar` 或 signal panel）。
- `package-lock.json`：隨新 devDeps 更新（docker `npm ci` 需要）。
- runtime / contract 變化：**無**（純測試工具與驗證流程）。

**版本相容（關鍵：對齊 Svelte 4）**

- `svelte-check@^3`（v4 需 Svelte 5，本專案 Svelte 4 必須用 v3）。
- `@testing-library/svelte@^4`（v5 targets Svelte 5；v4 對 Svelte 4，並提供 `svelteTesting` vite plugin
  設定 browser resolve conditions 與 auto-cleanup）。
- `vitest@^2` + `jsdom@^24` + `@testing-library/jest-dom@^6`（皆相容 Vite 5 / node 20）。
- 全部 pin 主版本，鎖進 `package-lock.json`。

**主要風險與回滾／相容策略**

1. **svelte-check 可能一次掃出大量既有型別錯誤**（strict 從未被強制執行）——最大風險。
   對策：實作第一步先跑一次 `svelte-check` 取得 baseline；
   - 若錯誤少 → 一併修掉（僅型別，不動行為），列在計畫執行紀錄。
   - 若錯誤多且牽涉行為 → 本輪先修「種子測試涵蓋範圍 + 明顯 bug」，其餘型別缺口另開 todo 追蹤，
     `check` 先以「不阻斷 build、僅報告」過渡（例如 `test.sh` 對 `check` 步驟暫允許非零退出並印出），
     待清乾淨再改為 fail-fast。此分岔在 baseline 出來後回報你再定。
2. **docker 2GiB host 記憶體**：vitest + jsdom 比純 build 吃資源。比照本機慣例（Python `-p=1`、Go
   `GOMAXPROCS=1`）限制併發：`vitest run` 加 `--pool=forks --poolOptions.forks.singleFork` 或
   `--maxWorkers=1`，`test.sh` 預設 MEM 視需要上調（可經環境變數覆寫，不寫死）。
3. **回滾**：全部異動集中在 `frontend/`（package.json / lock / vitest 設定 / 種子測試 / test.sh）與
   docs，無 runtime 影響；回滾即還原這些檔，`test.sh` 退回只跑 build。

**測試與驗證策略**

- 本地：`frontend/scripts/test.sh --install` 在 docker 內完整跑 `check → test:unit → build`，三段皆綠。
- 確認種子測試真的會抓錯：故意引入一個型別錯誤與一個失敗斷言，確認 `check` / `test:unit` 各自 fail，
  再還原。
- 確認 build 產物仍正確輸出到 `backend/internal/ui/dist`（vitest 用獨立 config，不觸發 build outDir）。

**完成後歸檔**

- 把 frontend 三層測試流程與如何執行（`scripts/test.sh` 的 check/unit/build 段、版本相容、記憶體限制）
  補進 [`docs/development-workflow.md`](./development-workflow.md)（共用開發與 Docker 驗收流程主文件），
  成為現況說明；`frontend/scripts/test.sh` 檔頭註解同步更新。review 通過後再從本清單移除 T-033。

#### 實作紀錄（2026-07-24，待 review）

- **三層框架**：`package.json` 加 devDeps（`svelte-check@^3`、`vitest@^2`、
  `@testing-library/svelte@^4`、`@testing-library/jest-dom@^6`、`jsdom@^24`）與 scripts
  （`check` / `test:unit` / `test:watch`）；新增 `vitest.config.ts`、`vitest-setup.ts`、
  `src/vitest.d.ts`（jest-dom 型別）、`src/vite-env.d.ts`（補 `vite/client` 讓 `import.meta.env`
  可用）。`frontend/scripts/test.sh` 改為 `check → test:unit → build`（fail-fast）。
- **`@testing-library/svelte` v4 沒有 `/vite`（svelteTesting plugin 是 v5 才有、需 Svelte 5）**，
  改用 `resolve.conditions=['browser']` + `vitest-setup.ts` 手動 `afterEach(cleanup)`。
- **svelte-check baseline = 13 個型別錯誤（3 檔），全數修正（型別 only、不動 runtime）**：
  - `SRZones.svelte`：`trainModelType` / `trainSplitMethod` / `trainCalibrationMethod` 改用具名
    type alias（`typeof` 在 Svelte 下被 narrow 到初始字面量，導致 `Record<typeof x>` 對不上完整
    union）；`formatDateTime` 參數放寬為 `string | null`。
  - `WatchlistTable.svelte`：`statusFilter` 改用 type alias `StatusFilter`。
  - `ws/socket.ts`：兩處不安全 `as` 改 `as unknown as`。
  - 修正後 `svelte-check` 0 errors / 0 warnings；`strict` 從此在 `check` 強制執行。
- **種子測試（9 tests / 4 檔全過）**：`lib/utils/format.test.ts`、`lib/stores/router.test.ts`、
  `lib/api/client.test.ts`（mock fetch + stores/auth，測 200/404 error body/401 登出）、
  `components/layout/Topbar.test.ts`（jsdom 渲染 + jest-dom matcher）。
- **驗證**：`frontend/scripts/test.sh` 完整跑過 `check`(0) → `test:unit`(9 passed) → `build`(dist 正常
  輸出到 `backend/internal/ui/dist`)；fail-fast 已實證（先前 vitest 設定錯時 build 未執行）。
- **現況已歸檔** `docs/development-workflow.md`「Frontend 測試框架（三層）」段。
- **後續（非本輪）**：逐步補齊 23 元件 / 15 api / 4 store 的覆蓋；本輪只放代表性種子。
- **注意**：build 重新產出 `backend/internal/ui/dist`（hash 檔名變動），commit 時需比照 DoD
  `git add backend/internal/ui/dist`。

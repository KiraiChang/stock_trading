# CLAUDE.md 只留開發規則、計畫書獨立成檔 v1（2026-10-27，✅ 確認（D1～D4 照建議）；實作已完成／待 review）

需求確認（2026-10-27）：

1. `CLAUDE.md` 只留開發時要遵守的規則；其餘的專案說明搬到 `docs/` 既有的主題文件（都⛔ 不適合的才新建）。
2. 計畫書改成獨立成檔、放在 `docs/plans/`：先把 `docs/issue.md` 的「Stage 2 步驟 ⑨-2 P1 修正計畫 v1」搬過來；之後的計畫書一律如此；既有寫在 `issue.md`／`todo.md` 裡的計畫⛔ 不搬。

✅ 2026-10-27 使用者確認本計畫，D1～D4 全部照建議；⑨-2 P1 修正計畫只搬檔、內容⛔ 不改。依 CLAUDE.md，Claude ⛔ 不 commit。

## 〇、盤點（2026-10-27，HEAD `cbde340`）

| 項目 | 現況 |
|---|---|
| `CLAUDE.md` | 539 行：第 1～50 行是規則（語言、驗收用 dev、協作流程）；第 54 行之後是專案說明（定位、技術選型、資料模型、資料流、各模組的演算法、通知、掃描範圍、Roadmap、設計理念），其中夾著幾條規則（下面「二」） |
| 與主題文件重疊 | 資料流、candles 的資料模型、指標、S/R、趨勢、突破、停損、爆量在 `architecture.md`、`database-schema.md`、`indicator-spec.md`、`signal-spec.md` 都已經有，而且**比 `CLAUDE.md` 新**（差異見「三」）；系統定位、Non Goals、設計理念、Roadmap 在主題文件裡⛔ 沒有 |
| 引用 `CLAUDE.md` 的程式註解 | 13 個程式檔在註解裡引用它，其中 11 個引用的是會保留的規則（驗收⛔ 不碰 live 等）；2 個引用的是要搬走的說明：`python/backtest/modular/backtester.py` 的「Non-HFT / Not low-latency」、`python/backtest/modular/exits/structural_stop.py` 的「Structure Broken（HL失效）」 |
| ⑨-2 的限制 | ⑨-2 的 P1 要求 ⑨-1 的 `E91` 之後只動文件，`--formal` 要求 `scripts/`、`python/`、`.gitattributes` ＝ HEAD → ⑩ 之前⛔ 不能改 `python/`、`scripts/`（含上面兩個註解與 `scripts/check-doc-refs.py`） |
| doc-refs | `scripts/check-doc-refs.py` 只掃 `docs/*.md`，⛔ 不掃 `docs/plans/` |
| `docs/plans/` | 尚不存在 |

## 一、目標、範圍與⛔ 不做

| 項目 | 內容 |
|---|---|
| 目標 | ① `CLAUDE.md` 只剩規則 ＋ 一段兩三行的專案概要與文件索引；② 搬出的每一段都有明確的去處（搬入、或主題文件已有而刪除），⛔ 沒有內容憑空消失；③ 計畫書的位置與生命週期寫成規則；④ ⑨-2 P1 修正計畫搬進 `docs/plans/` |
| 範圍 | `CLAUDE.md`；`docs/architecture.md`（新增「系統定位」一節、補「技術選型」「Python 服務」「前端頁面」）；`docs/issue.md`（⑨-2 P1 修正計畫一節換成指標、三處指向它的引用）；`docs/todo.md` 與 `docs/backtest-modular-strategy.md` 裡指向已搬走內容的引用（第一輪 review 補）；新檔 `docs/plans/i074-stage2-step92-p1.md`；本檔 |
| ⛔ 不做 | ⛔ 不改程式與腳本（含上面兩個註解、`check-doc-refs.py`——⑩ 之後另議）；⛔ 不搬既有寫在 `issue.md`／`todo.md` 裡的計畫；⛔ 不改主題文件既有的規格內容（只補 `CLAUDE.md` 獨有、而且仍然正確的部分）；⛔ 不順手修主題文件與程式的不一致 |

## 二、逐段的處置

處置規則：**主題文件已有、而且較新** → 只刪 `CLAUDE.md` 的那一段（差異列在「三」供 review，⛔ 不把舊的寫法搬過去）；**主題文件⛔ 沒有** → 照搬到主題文件，註明「2026-10-27 自 `CLAUDE.md` 搬入」；**規則** → 留在 `CLAUDE.md`。

| # | `CLAUDE.md` 的段落 | 處置 | 去處 |
|---|---|---|---|
| 1 | 開頭（繁中說明、`development-workflow.md` 為準、驗收用 dev project） | 保留（規則） | — |
| 2 | 「協作流程」 | 保留；計畫書的規則依「四」修改 | — |
| 3 | Trading（目標）、Architecture Principles、System Philosophy、Non Goals、Key Design Shift、Market Data Model 的 Design Notes、Core Insight | 照搬（主題文件⛔ 沒有） | `architecture.md` 新增「## 系統定位」（放在「## Pipeline 架構」之前） |
| 4 | Roadmap（Phase 1～4）、Scanner Scope、Notification System、Market Data Source 的 Phase 2（Shioaji） | 照搬，標明「規劃；各項的現況以 `todo.md` 為準」；Notification 另註「程式⛔ 沒有 LINE／Telegram／Email 的實作」 | `architecture.md`「系統定位」之下的「### 長期方向」 |
| 5 | Technology Stack／Backend（選 Go 的理由） | 「技術選型」表的 Go 一列只寫了「goroutine 適合多股票並行監控」→ 補上其餘理由（WebSocket／streaming、1000～2000 檔、工程效率與維護成本） | `architecture.md`「技術選型」 |
| 6 | Analytics Runtime（Python） | 規則留在 `CLAUDE.md`（「二之一」）；技術清單、兩個進入點、主要模組、lazy singleton、記憶體特性照搬 | `architecture.md`「Python 服務」的開頭（既有內容⛔ 不動） |
| 7 | Database | 規則留在 `CLAUDE.md`；「PostgreSQL 是 system of record、SQLite 只用於測試、MySQL 從未部署（I-054）」→ 改寫「技術選型」表的 Database 一列（它現在寫「三種環境皆支援」）；用途清單刪（`database-schema.md` 已逐表說明） | `architecture.md`「技術選型」 |
| 8 | Cache Layer | 用途補進「技術選型」表的 Redis 一列 | `architecture.md`「技術選型」 |
| 9 | Frontend | 版本（Svelte 4、Vite 5、Tailwind 3、Vitest 2）、原始碼結構與「⛔ 沒有使用 router 套件」照搬；用途清單刪（頁面表已有） | `architecture.md`「前端頁面」的開頭 |
| 10 | Market Data Source 的 Phase 1（FinMind） | 刪（「整體資料流」已有 FinMind 與 Fugle） | — |
| 11 | Market Data Model（`Candle` struct） | 刪（`database-schema.md`「candles」較新：多了 `adj_factor`、`vol_factor` 與約束） | — |
| 12 | Data Flow Architecture | 刪（`architecture.md`「整體資料流」較新、較細） | — |
| 13 | Core Modules、Indicator Engine、MA Calculation Rule | 刪（`indicator-spec.md`、`architecture.md`「模組關係」） | — |
| 14 | Support & Resistance Engine、Trend Detection、Breakout Detection、Stop Loss Logic、Volume Spike | 刪（`signal-spec.md`、`indicator-spec.md`） | — |

### 二之一、留在 `CLAUDE.md` 的規則（逐條）

* 繁中說明；開發與 Docker 驗收以 `development-workflow.md` 為準；驗收一律用 `docker-compose.dev.yml` 的 dev project。
* 協作流程全部（含 2026-10-27 的「一律⛔ 不 commit」）。
* `python/config.yaml` 的 `database` 區段要與 `backend/config.yaml` 保持同步。
* pytest 一律用 `python/scripts/test.sh`（在 docker 內跑、記憶體經 mem-guard 下修）；Go 的單元測試用 `backend/scripts/test.sh`（含 SQLite）。
* `db.py` 是 Python 端唯一的 candles 進入點（預設還原價）。
* 模組 import ⛔ 不得有連線等副作用（DB 連線檢查放在 FastAPI lifespan）。
* 三份 migration（postgres、sqlite、mysql）都要同步維護；MySQL 從未部署，要用之前先讀 `docs/issue.md` I-054。
* 新增：**專案概要**（兩三行：台股技術分析與交易輔助系統、只產生訊號由人決策、⛔ 不實作自動下單）與**文件索引**（系統定位與長期方向 → `architecture.md`；指標 → `indicator-spec.md`；訊號 → `signal-spec.md`；資料表 → `database-schema.md`；SR Zone → `sr-zone-scoring.md`）。兩個程式註解引用的「Non-HFT」「Structure Broken（HL失效）」靠索引找得到（`architecture.md`「系統定位」與 `backtest-modular-strategy.md`「StructuralStopLoss」；第一輪 review 訂正：原寫 `signal-spec.md`）。

## 三、刪除的段落與主題文件的差異（review 用；⛔ 不搬、⛔ 不在本次修）

| 主題 | `CLAUDE.md` 的寫法 | 主題文件（現況） |
|---|---|---|
| 突破 | Close > Resistance、Volume > AvgVolume(20)、Trend Bullish | `signal-spec.md`：前一根收在壓力之下、量比 ≥ 2.0、之後連續 2 根站穩、RSI14 < 80、Trend BULLISH；第二根確認 K 棒才發訊 |
| 爆量 | Volume > 2 × MA(Volume, 20) | `indicator-spec.md`：`VolRatio ≥ 2.0` 為 spike，`≥ 3.0` 才觸發 `VOLUME_SPIKE` 訊號 |
| 停損 | Close < Support、HL 失效、1～2 根內⛔ 沒有收回 → SELL | `signal-spec.md` 的 Breakdown：跌破支撐 ＋ 之後連續 2 根⛔ 沒有收回 ＋ 空頭；結構停損（HL 失效）的規格在 `backtest-modular-strategy.md`「StructuralStopLoss」（第一輪 review 補）；停損停利另見 `architecture/decision-pipeline.md` |
| MA | Rolling：`(PreviousSum − OldClose + NewClose) / N`，避免 full scan | `indicator-spec.md`：每次從最近 120 根計算；程式裡有 `RollingMA()` 但⛔ 沒有呼叫端 |
| 資料流 | FinMind → Go → PostgreSQL → Indicator → Redis → Signal → 通知 | `architecture.md`：含 Fugle、三種 DB、indicator_snapshots、chip_scores 加權、WebSocket 推播；⛔ 沒有通知管道 |
| candles | 9 個欄位 | `database-schema.md`：另有 `adj_factor`、`vol_factor`、唯一鍵與價格約束 |

## 四、計畫書的規則（`CLAUDE.md`「協作流程」的修改）

* 「大規模或高影響異動必須先留下可追溯的計畫書」之後加：**新建的**計畫書一律**獨立成檔**（2026-10-27 之前已寫在 `issue.md`／`todo.md` 裡的計畫⛔ 不追溯搬移——第一輪 review 補），放在 `docs/plans/`，一份計畫一個檔；`docs/issue.md`／`docs/todo.md` 的對應項目只留一行指向計畫書。修訂在同一個檔裡進行（標題標版本、檔內記每一輪的修正）。
* 「實作完成後先不要刪除 `docs/todo.md`／`docs/issue.md` 內的修改計畫書」改成：實作完成後先⛔ 不刪計畫書（`docs/plans/` 的檔，以及既有寫在 `issue.md`／`todo.md` 裡的計畫）；review 確認後，現況規格歸檔到主題文件，再刪除計畫書、移除指向它的引用；刪除之前計畫書必須已經 commit 過（尚未進版控的先 commit 保存、下一次收斂的 commit 再刪除——第一輪 review 補）。
* 「只有在使用者明確要求新增獨立文件……才建立新的 docs 文件」加註：`docs/plans/` 的計畫書⛔ 不受此限。
* 新增：`docs/plans/` 的計畫書⛔ 不寫 `檔名:行號` 形式的程式引用——`check-doc-refs.py` ⛔ 不掃這個目錄（⑩ 之後再擴充檢查範圍）。

## 五、⑨-2 P1 修正計畫的搬移

* `docs/issue.md` 的「Stage 2 步驟 ⑨-2 P1 修正計畫 v1」整節搬到 `docs/plans/i074-stage2-step92-p1.md`（內容原樣；只把標題層級改成檔案的 `#`、節內的相對連結改成 `../issue.md`）。
* 原位置留一行：「⑨-2 P1 修正計畫 v1：見 `plans/i074-stage2-step92-p1.md`（⚠️ 待確認）」（連到該檔）。
* 指向它的三處（⑨-1 結果一節的「一」「四」、I-074 狀態列、roadmap）改成連結到該檔。
* ⚠️ 本計畫會再改一次 `CLAUDE.md` → P1 修正計畫的 D1 只剩 (a)「根目錄的 `CLAUDE.md` 任何內容都允許」可行；(b)「只允許 `cbde340` 那一版」會失效。搬過去時在 D1 加一行註記（⛔ 不代替你裁決 D1）。

## 六、決定

| # | 決定 | 建議 |
|---|---|---|
| D1 | 刪除的段落與主題文件有差異（「三」） | **建議只刪、⛔ 不搬**：主題文件是較新的現況；差異只列在本計畫供 review。另一個選項是把差異記進 `issue.md`——但刪掉之後已經⛔ 沒有不一致，記了也⛔ 沒有東西可修 |
| D2 | 計畫書的檔名 | **建議⛔ 不帶版本**（`i074-stage2-step92-p1.md`）：改版時連結⛔ 不會斷，版本寫在標題；另一個選項是帶版本（`…-v1.md`），每次改版另開新檔 |
| D3 | 完成的計畫書 | **建議 review 確認、歸檔之後刪除**（與現行 `issue.md`／`todo.md` 的收斂規則一致，歷史留在 git）（⚠️ 第一輪 review 補：刪除之前必須已經 commit 過，歷史才留在 git）；另一個選項是保留在 `docs/plans/`、標示「已完成」 |
| D4 | 兩個引用搬走內容的程式註解 | **建議 ⑩ 之後另立**（記進 `todo.md`）：⑩ 之前改 `python/` 會讓 ⑨-2 的 P1 與 `--formal` 守門⛔ 不成立；在那之前靠 `CLAUDE.md` 的索引找到 |

## 七、風險與回滾

| 風險 | 處置 |
|---|---|
| 搬的時候漏掉規則 | 「二之一」逐條列出；實作後逐句比對刪除的每一行都屬於「二」的某一列 |
| 照搬的文字改了意思 | 照搬的段落與 `CLAUDE.md` 原文逐字比對（只允許標題層級與「搬入」的註記不同） |
| 之後的 session 少了專案背景 | `CLAUDE.md` 留概要與文件索引 |
| ⑨-2 | 所有改動都在 `docs/` 與 `CLAUDE.md`；P1 照修正計畫 D1 (a) 成立；`code_worktree` 的增減是 KB 級（`CLAUDE.md` 變小、`architecture.md` 變大） |
| 回滾 | revert 文件的 commit；⛔ 不影響程式與 artifact |

## 八、測試與驗證

* `CLAUDE.md`：刪掉的每一行都對應到「二」的一列（以 `git diff` 逐段核對）；「二之一」的每一條規則都還在。
* 照搬的段落：從 `git show HEAD:CLAUDE.md` 取出原文，與 `architecture.md` 搬入的段落比對（逐字相同，標題層級除外）。
* ⑨-2 P1 修正計畫：搬過去的內容與 `issue.md` 原本那一節逐字相同（標題層級、相對連結與 D1 的註記除外）。
* 連結：`docs/plans/` 與 `issue.md` 之間的相對連結都指得到；`grep` 確認⛔ 沒有殘留的「Stage 2 步驟 ⑨-2 P1 修正計畫 v1」純文字引用（改成連結的除外）。
* 文件：doc-refs、`git diff --check`、表格欄數、新增的行掃禁止符號後面的否定詞。
* ⑨-2 P1 的候選寫法：`git diff --name-only d3df284 HEAD -- . ':(exclude)docs' ':(exclude)CLAUDE.md'` 仍然沒有輸出（工作區的修改⛔ 不影響它，commit 之後再跑一次）。

## 九、歸檔與執行步驟

1. 本計畫確認（含 D1～D4）。
2. 實作：「二」「四」「五」→「八」的驗證 → ⛔ 不 commit，停下等 review。
3. review 確認之後：本計畫的現況規格（計畫書的規則）已經在 `CLAUDE.md`、搬入的說明已經在 `architecture.md` → 本檔與 `i074-stage2-step92-p1.md` 目前都**未進版控**：先由使用者 commit 保存，下一次收斂的 commit 再刪除本檔（D3；第一輪 review 訂正）；D4 記進 `todo.md`。

## 十、實作結果（2026-10-27，待 review）

| 項目 | 結果 |
|---|---|
| `CLAUDE.md` | 539 → 87 行：保留開頭與「協作流程」（依「四」改了三處），新增「專案概要」（含文件索引）與「開發規則」（「二之一」的 5 條）；第 52 行之後的專案說明全部移除 |
| `docs/architecture.md` | ＋199／−3：開頭新增「## 系統定位」（「二」#3、#4）；「技術選型」表的 Go、Database、Redis 三列改寫（#5、#7、#8）；「Python 服務」「前端頁面」的開頭加入搬入的段落（#6、#9）；其餘內容⛔ 沒有動 |
| ⑨-2 P1 修正計畫 | `docs/plans/i074-stage2-step92-p1.md`（標題層級 −3、標題下加一行出處、D1 之後加一行註記）；`docs/issue.md` 原位置換成一行連結；四處引用（⑨-1 結果一節的「一」「四」、I-074 狀態列、roadmap）改成連結 |

驗證：

* 刪除區域的 46 個標題都對應到「二」的某一列（未對應 0 個）。
* 照搬的 13 個區塊（#3、#4、#6、#9 與 Shioaji 一行）與 `git show HEAD:CLAUDE.md` 的原文逐字相同（比對時忽略標題層級、分隔線與空行）。
* 「二之一」的規則 14 個關鍵句都還在 `CLAUDE.md`；原本的協作流程只有「四」刻意改寫的 3 行不在新版裡。
* ⑨-2 P1 修正計畫：那一節原本只在工作區（⛔ 沒有 commit 過），⛔ 沒有獨立的副本可以比對；搬移是以腳本完成，只改標題層級並插入上面兩行註記；`issue.md` 已⛔ 沒有那一節的標題。
* 相對連結都指得到（`CLAUDE.md`、`issue.md`、`architecture.md` 與 `docs/plans/` 的兩個檔）；doc-refs 問題 0；`git diff --check` 通過；表格欄數都對；新增的行裡禁止符號後面都有否定詞。
* ⑨-2 P1 的候選寫法對 HEAD 仍然⛔ 沒有輸出（工作區的修改⛔ 不影響它；commit 之後要再跑一次）。

與計畫的差異（review 時確認）：

1. 「協作流程」的「不能只留在對話、`docs/todo.md` 或 `docs/issue.md`」多加了「計畫書」——D3 讓完成的計畫書在 review 之後刪除，現況規格同樣不能只留在計畫書裡。
2. 搬過去的 ⑨-2 P1 修正計畫在標題下多一行出處（說明文中的章節都指 `issue.md` I-074）；「五」只寫了 D1 的註記。
3. 「五」寫「指向它的三處」，實際是四處（「一」「四」各算一處）。

## 第一輪 review 的修正（2026-10-27）

裁決：搬移方向適切，主要的開發規則⛔ 沒有遺漏；補齊下面 4 點之後再收斂。

| # | review 的意見 | 查證 | 處置 |
|---|---|---|---|
| 1 | （P2）引用遷移不完整：`CLAUDE.md` 把「Structure Broken／HL 失效」指向 `signal-spec.md`，具體規格其實在 `backtest-modular-strategy.md`；`todo.md` 的 T-013、T-009 等仍引用已搬走的 Roadmap | ✅ 成立：`signal-spec.md` ⛔ 沒有 Structure Broken／HL 失效；`backtest-modular-strategy.md`「StructuralStopLoss」自己也還寫「對應 CLAUDE.md」；`todo.md` 指向已搬走內容的引用共 9 處（Scanner Scope 1、Roadmap 8：T-009 的來源與說明、T-013 的標題／來源／說明，以及另外三處 Roadmap Phase 2） | `CLAUDE.md` 的索引拆成兩列（訊號 → `signal-spec.md`；結構停損 → `backtest-modular-strategy.md`「StructuralStopLoss」）；`backtest-modular-strategy.md` 的那一句改成「本節是它的規格」；`todo.md` 的 9 處改指 `architecture.md`「系統定位」（標「原在 CLAUDE.md」）。程式註解照 D4 延到 ⑩ 之後。⚠️ 另外看到 `todo.md` 的 T-007 寫「見 `docs/CLAUDE.md` 提到的重複連線問題修正」——它在這次搬移之前就指不到（`CLAUDE.md` 從來⛔ 沒有那段），與本次無關，⛔ 不在本次修 |
| 2 | （P2）本計畫尚未進版控，⛔ 不能直接刪除並宣稱歷史留在 git | ✅ 成立（`docs/plans/` 是 untracked） | 「九」3：先由使用者 commit 保存，下一次收斂的 commit 再刪除；`CLAUDE.md` 的規則加上「刪除之前計畫書必須已經 commit 過」；D3 加註 |
| 3 | （P2）正式規則漏寫「既有計畫不搬」的例外 | ✅ 成立（已確認的需求是既有的⛔ 不搬，`CLAUDE.md` 卻寫「一律」） | `CLAUDE.md` 改成「新建的計畫書一律獨立成檔」，並明寫 2026-10-27 之前已寫在 `issue.md`／`todo.md` 裡的計畫不追溯搬移、照原規則收斂；「四」同步 |
| 4 | （P3）記憶體數字要保留量測條件 | ✅ 成立：T-054（2026-08-21）是 dev stack 的量測（`/sr-zones` 連續 75 次、postgres、`limit=400`），同一筆記錄 live 啟動 2 分鐘就到 323MB、⛔ 不能拿 dev 的數字外推 | `architecture.md`「Python 服務」的記憶體特性之後補一行量測條件（標「2026-10-27 補」；原文⛔ 不動） |

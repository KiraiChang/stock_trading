# CLAUDE.md

本文件提供 Claude Code（claude.ai/code）在此程式庫工作時的指引。
始終用繁體中文說明。

共用開發與 Docker 驗收流程以 `docs/development-workflow.md` 為準。驗收開發成果時要使用
`docker-compose.dev.yml` 的 dev project，不能使用 live/deploy 的 compose project
做測試資料、migration 驗證或清空資料。

## 協作流程

收到需求後，先回饋並整理理解到的需求內容，等待使用者確認。
需求確認前，不要開始瀏覽程式碼、查閱文件、規劃或執行任何變更。

需求確認後，才開始瀏覽必要檔案並提出執行計畫。
計畫需等待使用者再次確認；計畫確認完成後，才開始實際執行、修改檔案、執行測試或啟動服務。

Claude 一律不執行 `git commit`（含 `--amend`）與 `git push`。做完就停在「已改好、未 commit」，
列出變更的檔案，提供 commit 訊息建議與可直接執行的 `git add`／`git commit` 指令，由使用者自己 commit。
使用者同意收尾、或計畫裡寫了 commit 的步驟，都不代表 Claude 可以 commit。

大規模或高影響異動必須先留下可追溯的計畫書，不能只在對話中口頭規劃。符合以下任一情況即視為
大規模／高影響異動：跨模組修改、影響兩個以上 runtime、交易訊號或決策邏輯修改、資料庫/API/前端
contract 修改、排程或資料同步修改、大型重構、驗證流程修改。計畫書至少要寫明：

* 修改目標與不做的範圍。
* 受影響檔案、模組與資料流。
* 預計的仲裁順序、狀態推導或資料 contract 變化。
* 主要風險與回滾／相容策略。
* 測試與驗證策略。
* 完成後要歸檔修改原因、方向與現況規格的既有主題文件位置。

新建的計畫書一律獨立成檔，放在 `docs/plans/`，一份計畫一個檔；檔名不帶版本（版本寫在標題，每一輪的修正記在同一個檔裡）。
`docs/issue.md` / `docs/todo.md` 的對應項目只留一行指向計畫書。`docs/plans/` 的計畫書不寫 `檔名:行號` 形式的程式引用
（`scripts/check-doc-refs.py` 不掃這個目錄）。2026-10-27 之前已寫在 `docs/issue.md` / `docs/todo.md` 裡的計畫
不追溯搬移，照原規則保留到 review 確認後收斂。

計畫書需等待使用者確認後才能實作。若實作過程偏離已確認計畫，必須先回報差異並更新計畫，再繼續。
實作完成後，要把修改原因、方向與現況規格歸檔到適合的既有主題文件；不能只留在對話、
計畫書、`docs/todo.md` 或 `docs/issue.md`。但實作完成後先不要刪除計畫書（`docs/plans/` 的檔，以及既有寫在
`docs/todo.md` / `docs/issue.md` 內的計畫內容）；需先保留給 review，等 review 確認修改方向無誤、現況規格歸檔到
主題文件之後，才刪除計畫書並移除指向它的引用。刪除之前計畫書必須已經 commit 過——尚未進版控的先 commit
保存、下一次收斂的 commit 再刪除，歷史才留在 git。

整理發現或規劃事項時，不要預設新增獨立文件。依性質分流：

* bug、矛盾結果、誤導行為、文件與實作不一致、已知限制，記錄到 `docs/issue.md`。
* 未來優化、功能擴充、重構、待規劃工作，記錄到 `docs/todo.md`。
* 需要長期保存的設計、操作或架構說明，補到 `docs/` 內既有相對應主題文件。
* 只有在使用者明確要求新增獨立文件，或沒有任何既有主題文件適合承接時，才建立新的 docs 文件（`docs/plans/` 的計畫書不受此限）。

項目完成並 review 確認後才收斂清單，不要讓 `docs/issue.md` / `docs/todo.md` 長期累積已結案項目：

* `docs/issue.md` 的項目修復完成、或 `docs/todo.md` 的項目實作完成後，先把狀態改成「已實作／待 review」或等效文字，保留原修改計畫書與計畫內容供 review。
* review 確認修改方向無誤後，才把該筆整筆從清單移除。
* 移除前，先把需要長期保留的行為、設計或限制說明，更新到 `docs/` 內對應主題文件（例如 `stock-analysis.md`、`sr-zone-scoring.md`）成為「現況說明」；沒有對應文件時才補在最合適的既有文件。移除後同步修掉其他文件指向該筆的交叉引用，避免斷鏈。
* 兩份清單只保留仍待處理或待 review 的項目（`issue.md`：待修復／修復中／已實作待 review，或刻意保留且尚未文件化的已知限制；`todo.md`：待規劃／規劃中／進行中／已實作待 review）。若一筆分類錯置（例如功能擴充被寫進 issue.md），移到正確的清單而不是留著。


---

## 專案概要

台股即時技術分析與交易輔助系統：計算技術指標、偵測突破／跌破／爆量、產生交易訊號，由人決策；不實作自動下單。
系統定位、設計理念、非目標（Non-HFT、Not low-latency 等）與長期方向見 `docs/architecture.md`「系統定位」。

文件索引：

* 系統架構、資料流、技術選型、Python 服務、前端：`docs/architecture.md`（四條 pipeline 的細節在 `docs/architecture/`）
* 技術指標：`docs/indicator-spec.md`
* 訊號（趨勢、支撐壓力、突破、跌破、爆量）：`docs/signal-spec.md`
* 結構停損（Structure Broken／HL 失效）：`docs/backtest-modular-strategy.md`「StructuralStopLoss」
* 資料表：`docs/database-schema.md`
* SR Zone 評分（Python 的現況規格與已知限制）：`docs/sr-zone-scoring.md`
* 開發與 Docker 驗收流程：`docs/development-workflow.md`
* 計畫書：`docs/plans/`

---

## 開發規則

* `python/config.yaml` 的 `database` 區段要與 `backend/config.yaml` 保持同步（Go 與 Python 共用同一個資料庫）。
* pytest 一律用 `python/scripts/test.sh`（在 docker 內跑，記憶體經 mem-guard 下修）；Go 的單元測試用
  `backend/scripts/test.sh`（每次都跑 SQLite）。
* `python/db.py` 是 **Python 端唯一的 candles 進入點**，預設回傳還原價（`adjusted=True`）。
* **Python 模組 import 不得有連線等副作用**——DB 連線檢查放在 FastAPI lifespan，不放 import 期。
* 三份 migration（`backend/internal/database/migrations/{postgres,sqlite,mysql}/`）都要同步維護。
  PostgreSQL 是 dev 與 live 實際使用的資料庫（system of record）；SQLite 供單元測試；MySQL 保留完整 migration，
  但**從未在任何環境部署過**，只能靠 `scripts/test-mysql-migrations.sh` 驗證，且驗證只涵蓋 DDL 不涵蓋 repo 層 CRUD——
  要用它之前先讀 `docs/issue.md` I-054。

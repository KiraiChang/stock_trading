# I-141 計畫書 v1.1：盤前、收盤、籌碼同步讀不到自選清單時記成 `failed`

> 狀態：v1.1（✅ 2026-11-24 review 可採，補齊收盤尾端的驗收後開工）；已實作（實作備註與驗證見第九節）；✅ 2026-11-24 實作複審通過，**待上 live**（I-139 上 live 之後；程式在 `i141` 分支，合併進 init 之前⛔ 不上版）。對應：`docs/issue.md` I-141。
> 需求：2026-11-24 使用者裁決 I-139 實作備註裡「讀碼時順帶發現」的那一點，採 A：三個排程讀不到自選清單時都明確記 `failed`（比照盤中），
> `daily_close` 讀不到自選清單時，尾端的 `sr_zone_verify` 照跑。⛔ 不跟 I-139 一起在 11-28 上版，I-139 上 live 之後另外上。

## 一、目標與不做的範圍

**目標**

* `pre_market`、`daily_close`、`chip_daily_sync` 讀不到自選清單時，`job_runs.status` 記 `failed`，`symbols_total=0`、`symbols_failed=0`，
  `error` 照舊是 `watchlist_fetch:<原因碼>`（安全摘要，⛔ 不改格式）。
* `daily_close` 讀不到自選清單時，照樣執行收盤尾端（`runDailyCloseTail`，現在只有 `sr_zone_verify`）。它跟自選清單無關，
  跟 I-139「休市日 `daily_close` 記 `skipped`、`sr_zone_verify` 照跑」的規則一致。

**不做**

* ⛔ 不改自選清單讀得到時的狀態推導（`deriveRunStatus`）。
* ⛔ 不改其他排程：`evaluation_universe_sync` 讀不到候選清單是 `(0, 1)` → `partial`，`corporate_action_sync` 讀不到自選清單是降級成 `partial`（還有當日分片可跑），
  兩者的語意不同，這次⛔ 不碰；SR 分析已經是 `failed`。
* ⛔ 不加自動重跑：這三個排程本來就不在 `job_retry` 的範圍內。
* ⛔ 不改前端：排程頁照實顯示 `failed`。

## 二、現況（2026-11-24 讀碼）

* 三個排程讀自選清單失敗時都用 `finishRun(…, 0, 0, safeJobErrorSummary("watchlist_fetch", err))` 收尾；`deriveRunStatus(0, 0, false)` 推導出 `success`。
  `error` 欄有 `watchlist_fetch:…`，但狀態、`/scheduler/status`、排程頁都顯示成功。
* `RunDailyClose` 在讀自選清單失敗時提早 `return`，尾端的 `runDailyCloseTail`（`sr_zone_verify`）不會執行，當天沒有它的 `job_runs` 紀錄。
* 同一類情況的既有處理：盤中（T-088 review 2026-11-20）用 `finishRunStatus(…, "failed", 0, 0, …)`；SR 分析用 `finishRun(…, 1, 1, …)`（也是 `failed`）。
* 讀 `job_runs` 狀態做決策的只有 `delisting_reconcile`（看主檔同步），跟這三個排程無關。
* 既有測試：`TestRunPreMarketEarlyReturnDoesNotLeakCause`、`TestRunDailyCloseEarlyReturnDoesNotLeakCause`（只驗 `error` 不外洩原始錯誤）、
  `job_error_leak_test.go` 的三個排程案例。

## 三、設計

* 三個呼叫點改用 `finishRunStatus(ctx, runID, job, "failed", 0, 0, summary)`，跟盤中同一種寫法（`symbols_total` 照實是 0，⛔ 不用 `(1, 1)` 假裝有一檔）。
* `RunDailyClose`：讀不到自選清單時記 `failed` 之後呼叫 `runDailyCloseTail(ctx)` 再返回。手動端點（`daily-close/run`）走同一個函式，行為一樣。
* 程式註解標 I-141，說明為什麼⛔ 不用 `finishRun(…, 0, 0)`。

## 四、受影響的檔案、模組與資料流

* `backend/internal/scheduler/scheduler.go`：`runPreMarketWork`、`RunDailyClose`、`runChipDailySync` 的讀自選清單失敗分支。
* 測試：`backend/internal/scheduler` 補三個排程的狀態斷言與 `daily_close` 尾端照跑。
* 資料流：
  * 三個排程讀不到自選清單時，`job_runs.status` 從 `success` 改成 `failed`；⛔ 沒有 schema 變更、⛔ 沒有 API contract 變更（`status` 本來就有 `failed`）。
  * **`daily_close` 讀不到自選清單時會多執行收盤尾端**：`sr_zone_verify` 多一筆自己的 `job_runs`，而且會照常驗證最近 N 天的 SR 分析、
    **更新 `sr_zones` 的狀態**（`status`、`broken_at`、`broken_price`、`resolved_role`）。這跟交易日、休市日的尾端做的事完全相同（它只讀 SR 分析與 K 棒，⛔ 不讀自選清單）；
    以前這種日子它不跑，zone 狀態要等下一個交易日才更新。

## 五、主要風險與回滾

| 風險 | 處理 |
|---|---|
| 觀察判讀（I-123）把這種輪次算成失敗 | 這正是要的結果：那一輪確實沒做事。新版窗口（11-23～11-27）在上版之前就結束，⛔ 不影響 |
| `daily_close` 讀不到自選清單時多跑一次 `sr_zone_verify` | 它本來每天都跑、跟自選清單無關；DB 讀不到自選清單時它多半也會失敗，記自己的 `failed`，⛔ 不影響 `daily_close` 的紀錄 |
| 回滾 | 退回舊 image；⛔ 沒有 migration |

## 六、測試與驗證

* 三個排程各一個案例：自選清單回錯誤 → `job_runs` 一筆 `failed`、`symbols_total=0`、`symbols_failed=0`、`error` 是 `watchlist_fetch:conn_refused`（沿用既有的外洩檢查）。
* **清單讀得到但是空的**（合法的零標的輪次）→ 三個排程照舊是 `success`、`0/0`，⛔ 不能被改成 `failed`；`daily_close` 照樣執行尾端。
* `daily_close` 讀不到自選清單：注入 SR repo 的測試替身（既有的早退測試沒有注入，接上尾端後會走到 `ListRefsSince`），驗證
  * `job_runs` **恰好兩筆、依序**是 `daily_close`、`sr_zone_verify`；
  * `daily_close` 是 `failed`、`0/0`、`watchlist_fetch:…`；
  * `sr_zone_verify` 的狀態**獨立**：替身回一筆分析、驗證成功 → `success`、`1/0`；另一個案例替身的清單查詢失敗 → `sr_zone_verify` 自己 `failed`，`daily_close` 的紀錄⛔ 不受影響。
* 自選清單讀得到時的既有測試全部照舊通過。
* 反證：把三處改回 `finishRun(…, 0, 0)`、或拿掉 `daily_close` 的尾端呼叫，新測試要失敗；把「空清單」也改成 `failed`，空清單的案例要失敗。
* 完整回歸 `backend/scripts/test.sh`；`scheduler` 開 race。
* ⛔ 不做 dev 實跑：在 dev 讓自選清單讀不到要動 DB（例如暫時改表名），風險比收益大；單元測試直接注入失敗的自選清單已經涵蓋。

## 七、完成後要歸檔的位置

* `docs/architecture/data-pipeline.md`：「盤中的逐輪存證」那條「讀不到自選清單記 `failed`」擴大成每日排程（盤前、收盤、籌碼同步）的規則，含 `daily_close` 尾端照跑。
* `docs/architecture.md`「休市日的行為」旁：`daily_close` 尾端在兩種提早結束（休市、讀不到自選清單）都照跑。
* `docs/database-schema.md` 的 `job_runs`：`failed` 也包括「讀不到自選清單、整輪沒有執行」。
* `docs/issue.md` I-141 改成「已實作／待 review」；review 確認後收斂。

## 八、上版

* I-139 上 live（預定 11-28）之後，照一般部署規範另外上；⛔ 沒有設定鍵或 migration。
* 上版後⛔ 不需要特別觀察：只有讀不到自選清單時才會走到，正常情況看不到差別。

## 九、實作備註（2026-11-24）

* `scheduler.go`：新增 `failRunOnWatchlist`（`finishRunStatus(…, "failed", 0, 0, watchlist_fetch 摘要)`），`runPreMarketWork`、`RunDailyClose`、`runChipDailySync` 三處改用它；
  `RunDailyClose` 記 `failed` 之後呼叫 `runDailyCloseTail` 再返回。
* 測試（新檔 `watchlist_fetch_status_test.go`）：
  * 三個排程讀不到自選清單 → 第一筆是該排程、`failed`、`0/0`、`watchlist_fetch:conn_refused`、⛔ 沒有外洩原始錯誤；
  * 三個排程清單是空的 → `success`、`0/0`、`error` 空白，`daily_close` 照樣執行尾端；
  * `daily_close` 讀不到自選清單：恰好兩筆、依序 `daily_close` → `sr_zone_verify`；尾端驗證成功時 `sr_zone_verify` 是 `success`、`1/0`、zone 狀態有更新（替身記下 `UpdateZoneStatus`）；
    尾端清單失敗時 `sr_zone_verify` 自己 `failed`，`daily_close` 的紀錄⛔ 不受影響。
* **跟計畫的差異**：既有測試共用的 `newWatchlistFailScheduler` 補上沒有分析的 SR repo 替身（接上尾端之後，三支既有測試的 `daily_close` 會走到 `ListRefsSince`，沒有替身會 panic）；
  `TestRunDailyCloseEarlyReturnDoesNotLeakCause` 自己建 Scheduler，同樣補上替身。既有斷言⛔ 沒有改。
* **驗證**：scheduler 套件通過、開 race 通過；backend 完整測試（`backend/scripts/test.sh`）通過。反證：三處改回 `finishRun(…, 0, 0)` → 讀不到清單的兩支測試失敗；
  拿掉 `daily_close` 的尾端呼叫 → 尾端那支失敗；`pre_market` 的空清單也改記 `failed` → 空清單那支失敗。
* 實作複審（2026-11-24）通過；依複審更正 `data-pipeline.md`：「⛔ 不在 `job_retry` 範圍」明指本案的三個每日排程，`intraday` 有既有的輪內批次重試。
* 主題文件已歸檔：`architecture/data-pipeline.md`「讀不到自選清單：整輪記 failed」、`architecture.md`「休市日的行為」的 `daily_close` 列、`database-schema.md` 的 `job_runs`。

## 版本紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1.0 | 2026-11-24 | 初版：使用者裁決採 A（三個排程記 `failed`、`daily_close` 尾端照跑），⛔ 不跟 I-139 一起上版 |
| v1.1 | 2026-11-24 | review 可採：驗收補「清單讀得到但是空的」仍是 `success`；`daily_close` 尾端的測試注入 SR repo 替身，驗恰好兩筆、依序、狀態各自獨立；第四節補上尾端會更新 `sr_zones` 的狀態 |
| v1.1（實作複審通過） | 2026-11-24 | 實作複審通過；更正 `data-pipeline.md` 的重跑範圍描述（只指本案三個每日排程） |

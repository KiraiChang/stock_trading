# I-123～I-126 計畫 v4：暫時性失敗的自動重跑機制

> 狀態：✅ v4 已確認（2026-10-30，`7fd3962`；D1～D6 依建議定案，D5 採 (b)）。階段 A 已實作、⚠️ 待 review（實作備註見「十二」）；階段 B 尚未開始。
> 對應：[`issue.md`](../issue.md) I-123～I-126；背景是 I-122 的調查（計畫書 [`i122-live-batch-failures.md`](./i122-live-batch-failures.md)）。

## 一、目標與不做的範圍

需求（2026-10-30 使用者確認）：

* I-123（DNS 查詢逾時、IPv6 不可達）與 I-124（postgres 進入 recovery mode）**⛔ 不再追查原因**，視為網路波動或第三方維修造成的暫時性失敗；
  改做**失敗後自動重跑**，讓這類失敗自己恢復，⛔ 不必人工補。I-124 原本要保全的 postgres log 與 kernel log ⛔ 不再收集。
* 重跑的範圍（依 I-122 觀察到的失敗）：
  1. **整輪失敗的每日工作**隔一段時間自動重跑；`stock_symbol_sync` 成功之後，被它擋下的 `delisting_reconcile` 跟著重跑。
  2. **`intraday` 的批次抓取失敗**：同一輪內短暫重試。
  3. **`corporate_action_sync` 個別失敗的標的**：同一輪的預算內再試一次。
  4. **`job_runs` 的結束狀態寫不進去**（I-125）：有限次數地重試寫入，**大幅降低**停在 `running` 的機會。
     ⚠️ 剩餘限制：重試全部失敗時仍會停在 `running`，直到下次啟動回收（「3.5」）。
* **I-126 一併處理**：重跑要靠正確的狀態判斷，整輪失敗要記成 failed、前置條件不成立要分得出來。
* 重跑要有上限與截止時間：間隔 10 分鐘、每天 3 次、**整輪在盤前的截止時刻之前結束**（「3.2」）。

不做：

* ⛔ 不查 DNS、IPv6、postgres 的根因，⛔ 不改 live 主機或 Docker 的網路設定。
* ⛔ 不新增 `job_runs` 的欄位、狀態值或端點。
* ⛔ 不改 FinMind client 的請求層重試政策（網路錯誤、5xx、429、402 退避重試 3 次）；只讓最後回傳的錯誤保有 HTTP 狀態的型別（「3.1」）。
* `startRun` 只加上寫入的上限（10 秒，「3.5」）；寫不進去時（`runID` 為 0）的既有處理⛔ 不改。
* ⛔ 不讓重跑跨日：每一條重跑鏈只在當天（台北）有效。
* ⛔ 不保證 `intraday` 的兩輪⛔ 不重疊（現在就是各自獨立的 goroutine）；本計畫只限制重試增加的時間（「3.3」）。
* ⛔ 不碰 live：live 的部署設定、部署與開關由使用者執行（「3.7」）。

## 二、現況（讀碼，2026-10-30）

| 工作／路徑 | 現在失敗時怎麼處理 | I-122 看到的 |
|---|---|---|
| `stock_symbol_sync`（每日 06:30，預算 20 分鐘） | 整輪失敗即結束，**刻意不重試**（`data-pipeline.md`「為什麼不自動重試」；`twse_isin.go` 的說明），靠人工按手動同步；寫 `(0, 1)` → 落到 partial（I-126） | 10-24、10-29、10-30 DNS 逾時 |
| `delisting_reconcile`（每日 07:00，預算 5 分鐘） | 前置條件（當日最新一筆 `stock_symbol_sync` 是 success）不成立 → failed、`internal_error`（I-126）；⛔ 沒有之後再跑的機制 | 同樣三天 |
| `corporate_action_sync`（平日 06:30，預算 45 分鐘） | 分割批次或列標的失敗 → 整輪 failed（`(1, 1)`）結束；逐檔失敗只計數，**下一次輪到那個分片時**（5 個平日後）才補 | 10-22 DB recovery 讓整輪中止；每天 1～3 檔 |
| `intraday`（每 5 分鐘） | 批次抓取失敗就記失敗數，⛔ 不重試，接著用資料庫裡既有的 K 棒評估 | 31 輪 |
| 結束寫入（`finishRunStatus`） | 只寫一次，失敗只記 Error log | 4573 停在 `running` 6 天 |

* 既有的重試：只有 FinMind client 有請求層重試（`fetchMaxRetries` 3 次、退避 1／2／4 秒）；用完時回 `finmind fetch failed after 3 retries: …`，
  裡面的 5xx 是一般的文字錯誤（⛔ 沒有可判斷狀態碼的型別）。Yahoo 報價、Yahoo 配息、TWSE ISIN、TWSE 終止上市名單的 client 都⛔ 沒有重試，非 200 時同樣只回文字錯誤。
* 防重疊：`delisting_reconcile` 與 SR 分析有 single-flight（手動撞上時回 409，前端有對應的訊息）；`stock_symbol_sync`、`corporate_action_sync` 的手動入口直接
  `go` 出去，cron 與手動**可以重疊**（既有風險），前端也⛔ 沒有處理 409。
* 盤中 K 棒寫入是 `ON CONFLICT … DO UPDATE`；Yahoo 的批次端點一次回傳**當日全部**的 1 分 K。之後成功的那一輪理論上會補回前面漏掉的棒——
  ⚠️ 尚未驗證（I-122「影響」），本計畫在「八」以正式的抓取流程驗證；13:30 是最後一輪，那一輪失敗就沒有下一輪可以補。
* 啟動回收：`AbortRunning` 把殘留的 `running` 改成 `aborted`，訊息寫死「process 在此 job 完成前結束」（I-125）。
* live 的部署：環境變數要在 repo 的 `docker-compose.yml`、repo 的 `deploy.sh` 範本、live 的 `compose.yml`（repo 外，手動維護）三處都有，否則靜默失效
  （`development-workflow.md`「新增排程開關要改三個地方」）。

## 三、設計

### 3.1 暫時性錯誤的判定（共用）

新增 `market.IsTransient(err) bool`（`backend/internal/market/transient.go`），所有重跑與重試都用它決定「值不值得再試」。**只認型別、⛔ 不比對訊息文字**：

| 判定為暫時性 | 依據 |
|---|---|
| DNS 查詢逾時或暫時性失敗 | `*net.DNSError` 的 `IsTimeout` 或 `IsTemporary` |
| 連線層錯誤（拒絕、重設、網路不可達、讀寫逾時） | `*net.OpError` |
| HTTP client 逾時 | `url.Error` 的 `Timeout()` |
| 連線中途斷掉 | `io.ErrUnexpectedEOF` |
| 上游 5xx（含維修常見的 503） | 新增的 `market.HTTPStatusError`，`Code >= 500`（D3） |
| DB 連線類錯誤 | 錯誤鏈上有 `SQLState()`（pgconn 的錯誤型別有這個方法，⛔ 不必 import pgx）且屬於 class `08`、`57P01`～`57P03`、`53300` |

⛔ **不算暫時性**：4xx（含 404、429——429 是限流，縮短間隔重試只會更糟）、FinMind 的 `api_status`（402、429 等）、解析錯誤、主檔縮水防護、
`ErrEmptyStockSymbolSnapshot`、前置條件不成立、DNS 的 NXDOMAIN（`IsNotFound`），以及**工作自己的預算用完**（呼叫端先檢查自己的 `ctx.Err()`）。

`HTTPStatusError` 加在五個 client 的非 200 分支：TWSE ISIN、TWSE 終止上市名單（`delisting_reconcile` 的來源）、Yahoo 報價、Yahoo 配息、**FinMind**。
`Error()` 的文字**與現在完全相同**（log 與 `joberr.Classify` 的結果都不變）。FinMind 的請求層重試政策⛔ 不變，只是 5xx／429 那個分支改回
`HTTPStatusError`；用完重試時的外層錯誤本來就以 `%w` 包住最後一個錯誤，所以公司行動的分割批次、減資查詢遇到 5xx 時，外層的 `IsTransient` 判斷得出來。

### 3.2 層一：整輪重跑（三個每日工作）

適用 `stock_symbol_sync`、`corporate_action_sync`（只限整輪早退：分割批次、列標的失敗）、`delisting_reconcile`（D2）。

**觸發**：一輪以 **failed** 結束、原因 `IsTransient`、而且這一輪是 cron 或重跑發起的（手動觸發的失敗⛔ 不排重跑）。

**時間**：失敗結束後 `interval`（D1：10 分鐘）重跑，每個工作每天最多 `max_reruns`（D1：3 次）。

* **整輪的截止時刻** `cutoff`（預設 08:45，`JOB_RETRY_CUTOFF`，「3.7」）：重跑的**整輪**——寫開始紀錄、業務、寫結束紀錄——都要在它之前結束，
  ⛔ 不壓到 08:50 的盤前（`pre_market`）。所以每個工作的
  **最晚起跑時刻 ＝ cutoff −（業務預算）− 3 分鐘**，3 分鐘涵蓋紀錄寫入：`startRun` 上限 10 秒 ＋ 結束寫入重試最多約 133 秒（「3.5」）＋ 餘裕。
  預設下是 `corporate_action_sync` 07:57（業務預算 45 分鐘）、`stock_symbol_sync` 08:22（20 分鐘）、`delisting_reconcile` 08:37（5 分鐘）。
  重跑時刻晚於它就⛔ 不排。
  驗算：最晚在 `cutoff − B − 3 分鐘` 起跑，`startRun` ≤ 10 秒、業務 ≤ B、結束寫入 ≤ 133 秒，最晚在 `cutoff − 37 秒` 結束。
  公司行動的業務預算可由設定調整（`CORPORATE_ACTION_TIMEOUT_SEC`），最晚起跑時刻依實際的預算計算，⛔ 不寫死。
* 重跑時刻跨到隔天（台北）也⛔ 不排。

**重跑鏈的規則**：

1. **同一天、同一個工作只有一條鏈**：計數（已排幾次）與至多一個待跑的計時器。換日就重置。
2. **任一入口成功，舊鏈就失效**：cron、手動、重跑、連帶觸發，只要這個工作當天成功一次，就取消待跑的計時器，當天⛔ 不再重跑。
   例：06:30 失敗、06:35 手動成功 → 06:40 的重跑⛔ 不會執行（最新狀態⛔ 不會被一次舊的重跑改回 failed）。
3. **計時器到期時**先確認：還是同一天、還沒成功、還沒超過最晚起跑時刻；再取得執行所有權（D5）。取不到 → 這次重跑取消、記 Warn，**這條鏈結束**
   （正在跑的那一輪由它自己的結果決定，手動除外）。
4. **連帶重跑**（狀態都在**同一把鎖**下判斷，避免漏掉成功通知）：
   * **「主檔最新一輪已成功落盤」**——⛔ 不是「主檔今天曾成功」。任何一輪 `stock_symbol_sync` **開始時**（任一入口取得執行所有權時）設為假；
     結束紀錄以 success **寫入成功之後**才設為真；失敗或結束寫入失敗都維持假。它對應前置條件讀到的「當日最新一筆是 success」（前置條件讀的是 `job_runs`）。
     例：06:30 主檔成功（真）→ 06:55 手動同步開始（假）→ 失敗（仍是假）→ 07:00 對帳被最新那筆 failed 擋住 → ⛔ 不觸發。
   * **「主檔成功的代號」**：每次設為真時加 1。對帳記下自己**已經用掉的代號**；**同一個代號最多觸發一次對帳**——一次主檔成功最多帶動一次對帳，
     ⛔ 不會因為對帳又被擋下而反覆觸發、耗盡額度。
   * 其餘狀態：「對帳今天在等主檔」、「對帳待觸發」。
   * **對帳登記等待時重新確認**：`delisting_reconcile` 因前置條件不成立而失敗（cron 或重跑發起的；⛔ 不排自己的計時器，前置不成立⛔ 不是暫時性）時，
     在鎖內看：主檔最新一輪已成功落盤、而且代號比對帳用掉的新 → 直接設「待觸發」；否則 → 設「在等」。
     這擋住「對帳讀到主檔還在 running → 主檔成功並通知 → 對帳才登記等待」的交錯。
   * **主檔成功落盤時**：在鎖內設為真、代號加 1；對帳在等 → 改成「待觸發」。接著嘗試觸發。
   * **觸發**：在鎖內再確認一次「主檔最新一輪已成功落盤」與代號、當天對帳還沒成功、還有額度、還沒超過對帳的最晚起跑時刻，再取得對帳的執行所有權；
     **取不到（對帳正在跑或還沒釋放）→「待觸發」留著**。
   * **所有權釋放後補觸發**：每一輪 `delisting_reconcile` 釋放執行所有權時都檢查「待觸發」，有就再嘗試一次。這擋住「通知當下對帳還持有所有權，觸發被取消」的交錯。
   * 觸發成功時清掉「待觸發」與「在等」、記下用掉的代號，**計入對帳自己的每日上限**，並取消對帳待跑的計時器（若有）。
5. `Stop()` 時取消全部計時器；重跑鏈只在記憶體裡，backend 重啟後⛔ 不保留（照實寫進文件）。

**每次重跑都是一輪完整的執行**：新的一筆 `job_runs`（⛔ 不改寫前一筆）；TWSE 的兩個來源整批重抓，all-or-nothing 不變。

**記錄**：排了重跑的那一筆，`error` 後面附固定的安全文字 `; 自動重跑 n/3（HH:MM）`；用完時附 `; 自動重跑已用完（3/3）`；
因最晚起跑時刻而⛔ 不排時附 `; 自動重跑已停止（最晚起跑 HH:MM）`。`/scheduler/status` 取每個工作的最新一筆，所以等待期間看得到「失敗、幾點重跑」，成功後顯示 success。

**對「為什麼不自動重試」三個理由的回應**（文件要一起改）：
① 漏同步一天的代價——I-122 已看到它會連帶擋下終止上市對帳，代價比原先估的大；
② all-or-nothing 不變，重跑是整批重抓，⛔ 不會只拿到半個市場；
③ 「連續打同一站台」——重跑間隔 10 分鐘、一天最多 3 次、盤前截止，而且只在暫時性錯誤時才跑；I-122 的三次失敗都在 DNS 階段，請求根本沒有送到 TWSE。

### 3.3 層二：`intraday` 同一輪內重試批次

* 批次抓取失敗、原因 `IsTransient` → 等 10 秒重試，再失敗等 20 秒再試一次（每批最多 2 次）；仍失敗才計入失敗數，接著照現在的流程評估。
* **整輪的重試預算 120 秒，用 context 執行**：每一輪開始時建一個在「開始時刻 ＋ 120 秒」到期的重試 context；**重試的等待、Yahoo 節流器的等待、
  HTTP 請求與寫入 K 棒都用它**，到期就中止（不論有幾批）。所以重試**最晚在一輪開始後 120 秒結束**——⛔ 不是只檢查「要不要開始重試」。
  每一批的**第一次**抓取⛔ 不受它限制（與現在相同）；重試在寫 K 棒時被中止的話，已寫入的棒保留（`ON CONFLICT DO UPDATE`，下一輪會覆蓋）。
* ⛔ 不保證兩輪⛔ 不重疊：批次多、第一次抓取、評估與結束寫入都還要時間，這與現在相同；重試只會讓一輪最多晚約 120 秒結束重試的部分。
* **重試成功的判定**：`FetchAndStoreIntradayBatch` 回傳 `nil`，**而且**呼叫返回時重試 context 還沒到期（`ctx.Err() == nil`）。
  ⚠️ 只看回傳值不夠：它遇到逐檔寫入失敗只記 log、最後仍回傳 `nil`，所以「HTTP 成功、寫入途中預算到期」會被誤當成成功。
  context 已到期 → 這一批**保留失敗計數**（與現在的整批失敗相同，⛔ 不算重試成功）。
  ⛔ 不在這裡處理其他原因的逐檔寫入失敗——那是 I-103 的範圍（批次路徑給不出逐檔寫入失敗），本計畫⛔ 不擴大。
* 重試成功時記 Info（含重試次數），⛔ 不計入失敗。

### 3.4 層三：`corporate_action_sync` 失敗標的的第二輪

* `SyncFailure` 多一個欄位 `Transient`（在記錄失敗的當下用 `IsTransient(cause)` 判斷；⛔ 不寫進 `job_runs`，只供重試決策）。
* 第一輪跑完之後，挑出**所有失敗階段都是暫時性**的標的；剩餘預算超過 60 秒時先等 60 秒，再對它們跑一次 `SyncPerSymbolEvents`（仍在同一個 45 分鐘的預算內）。
* 合併：第二輪成功的標的從失敗清單移除；第二輪沒跑到（預算用完）的保留第一輪的失敗；`symbols_failed` 與 `error` 依合併後的結果寫。
* 404 之類的永久失敗⛔ 不重試（例如 00847B，I-122）。重算是冪等的，重試⛔ 不會累積誤差。

### 3.5 層四：結束寫入的重試與回收訊息（I-125）

* `finishRunStatus`（以及 `intraday` 的 `skipped` 那條直接呼叫 `Finish` 的路徑）寫入失敗時，退避 1／2／4／8／16／32 秒重試（每次仍是 `finishRunWriteTimeout` 10 秒），
  最壞 `7 × 10 + 63 = 133` 秒；全部失敗才記 Error（帶重試次數）。這條**⛔ 不受 D4 的開關控制**（缺陷修正）。
* `startRun` 加上寫入的上限 `startRunWriteTimeout`（10 秒；`context.WithoutCancel` 加自己的 timeout，比照結束寫入）。
  現在 `stock_symbol_sync`、`delisting_reconcile` 的 `startRun` 用的是沒有期限的 `context.Background()`，DB 卡住時會無限等——有了上限，「3.2」的截止才算得出來。
  寫不進去（`runID` 為 0）時沿用各工作既有的處理（對帳中止；其他工作照跑、只是沒有紀錄）。⚠️ 這條套用到**所有**工作。
* ⚠️ **剩餘限制**：重試全部失敗時那一筆仍停在 `running`，直到下次啟動由 `AbortRunning` 回收；期間狀態頁顯示「執行中」，
  依賴它的前置條件（終止上市對帳要求當日最新一筆 `stock_symbol_sync` 是 success）也會被擋下。這條限制在 I-125 收斂時寫進 `api-reference.md`。
* `AbortRunning` 的訊息改成中性：「沒有寫入結束狀態（process 中斷，或結束狀態寫入失敗），結果未知（啟動時回收）」。
* 文件：`api-reference.md` 的 `running` 判讀規則補上「結束狀態寫不進去」這條路徑與上面的剩餘限制。

### 3.6 I-126：狀態與原因

* `stock_symbol_sync` 整輪失敗改用明確的狀態：`finishRunStatus(…, "failed", 0, 0, …)`（D6；比照 `delisting_reconcile` 的寫法），⛔ 不再落到 partial。
  ⚠️ `0/0` 的意思是「**沒有回報計數**」，⛔ 不代表已證實沒有處理任何資料。
* `delisting_reconcile` 的前置條件不成立寫成專屬的安全文字 `delisting_reconcile:stock_symbol_sync_not_ready`（`errors.Is(err, ErrStockSymbolSyncNotReady)`），
  ⛔ 不再壓成 `internal_error`。狀態仍是 failed（當天的對帳確實沒有做；連帶重跑成功後會被新的一筆蓋過）。

### 3.7 開關、設定與部署（D4）

| 設定 | 環境變數 | 預設 | 說明 |
|---|---|---|---|
| `job_retry.enabled` | `JOB_RETRY_ENABLED` | `false` | 控制層一～三；⛔ 不控制層四與 I-126 |
| `job_retry.interval_min` | `JOB_RETRY_INTERVAL_MIN` | `10` | 層一的間隔（≤ 0 時退回預設） |
| `job_retry.max_reruns` | `JOB_RETRY_MAX_RERUNS` | `3` | 層一每個工作每天的上限（≤ 0 時退回預設） |
| `job_retry.cutoff` | `JOB_RETRY_CUTOFF` | `08:45` | 層一的整輪截止時刻（台北，`HH:MM`）；格式錯誤時退回預設並記 Error。live ⛔ 不需要改；dev 驗收時調晚（「八」） |

層二、層三的秒數是程式常數。scheduler 啟動時記一行 Info：開關、間隔、上限、截止時刻與三個工作的最晚起跑時刻，供部署後確認。

**傳遞到 live 容器**（漏任何一處，開關都會靜默失效）：

| 位置 | 誰改 | 動作 |
|---|---|---|
| repo 的 `docker-compose.yml` | Claude（階段 B） | backend 的 `environment:` 加四個鍵（`${JOB_RETRY_ENABLED:-false}` 等） |
| repo 的 `docker-compose.dev.yml` | Claude（階段 B） | dev 的 backend 也加同樣四個鍵（預設關閉），dev 驗收才設得進去 |
| repo 的 `deploy.sh`（範本） | Claude（階段 B） | 加對應的 `export`（範本值是預設） |
| live 的 `/opt/stacks/scripts/stock_trading/compose.yml`（repo 外） | 使用者 | backend 的 `environment:` 加同樣四個鍵 |
| live 的 `/opt/stacks/scripts/stock_trading/deploy.sh` | 使用者 | `export JOB_RETRY_ENABLED=true`（其餘三個留預設即可） |

**部署後的確認**（使用者執行；⛔ 不印 `deploy.sh` 的行內容，只看鍵名）：

1. `docker exec stock_trading-backend-1 sh -c 'echo $JOB_RETRY_ENABLED'` 印出 `true`。
2. backend 的啟動 log 有那一行 Info：開關是開的、截止時刻 08:45、最晚起跑時刻 07:57／08:22／08:37。
3. live 的 `compose.yml` 有那四個鍵、`deploy.sh` 有 `JOB_RETRY_ENABLED`（`grep -c` 鍵名）。

**回滾**：live 的 `deploy.sh` 改成 `JOB_RETRY_ENABLED=false` 再部署，確認 `echo` 印出 `false`、啟動 log 顯示關閉。層四與 I-126 要回滾就退回前一個 image。

## 四、受影響的檔案與資料流

| 檔案 | 改什麼 |
|---|---|
| `backend/internal/market/transient.go`（新） | `IsTransient`、`HTTPStatusError` |
| `backend/internal/market/twse_isin.go`、`twse_suspend_listing.go`、`yahoo_quote.go`、`yahoo_dividend.go`、`finmind.go` | 非 200 回 `HTTPStatusError`（文字不變）；`twse_isin.go` 的「不重試」說明改寫 |
| `backend/internal/market/adjuster.go` | `SyncFailure.Transient` |
| `backend/internal/scheduler/job_retry.go`（新） | 重跑鏈（計數、計時器、截止時刻、連帶重跑；可注入的時鐘、計時器與等待函式供測試） |
| `backend/internal/scheduler/scheduler.go` | 層一的掛點、兩個工作的執行所有權（D5）、層二、層三、層四、I-126 (1)、`Stop()` 取消、啟動 log |
| `backend/internal/scheduler/delisting_reconcile.go` | 層一、連帶重跑、I-126 (2) |
| `backend/internal/api/handler/scheduler.go` | 兩個手動入口改用 `TryStart…`，撞上回 409（D5） |
| `backend/internal/store/job_run_repo.go` | `AbortRunning` 的訊息 |
| `backend/internal/config/config.go`、`backend/config.yaml`、`backend/cmd/server/main.go` | `job_retry` 設定與注入 |
| `docker-compose.yml`、`docker-compose.dev.yml`、`deploy.sh`（repo 範本） | 四個環境變數的傳遞 |
| `frontend/src/routes/Scheduler.svelte` | 兩個手動按鈕遇到 409 時顯示「已有一輪在執行中」（比照對帳） |
| 各自的測試 | 見「八」 |

資料流：⛔ 沒有新的表、欄位或端點。重跑只是多幾筆 `job_runs`；`error` 多一段固定的安全文字。

## 五、狀態推導與 contract 的變化

| 項目 | 變化 | 相容性 |
|---|---|---|
| `job_runs.status` 的值域 | ⛔ 不變（仍是 `running`／`success`／`partial`／`failed`／`aborted`／`skipped`） | 前端的狀態顯示⛔ 不必改 |
| `stock_symbol_sync` 整輪失敗 | partial → **failed**，`symbols_total`／`symbols_failed` 由 `0/1` 改成 `0/0`（沒有回報計數） | 讀 status 的地方只有前置條件（要求 success，不受影響）與狀態頁 |
| `delisting_reconcile` 前置不成立的 `error` | `…:internal_error` → `…:stock_symbol_sync_not_ready` | 合法形式（本專案自己組的固定安全文字） |
| 排了重跑的那一筆 `error` | 多一段重跑的固定文字（「3.2」） | 同上 |
| `aborted` 的 `error` | 改成中性的訊息 | 同上 |
| 同一天同一個工作的筆數 | 失敗時最多多 3 筆 | `GetLatestPerJob` 取最新一筆，⛔ 不受影響 |
| `startRun` | 加 10 秒的寫入上限（所有工作） | 超過時 `runID` 為 0，沿用既有處理（「3.5」） |
| `POST /scheduler/stock-symbol-sync/run`、`/corporate-action-sync/run` | 已有一輪在跑時回 **409**（原本一律 202）；cron 撞上時只記 Warn、⛔ 不寫 `job_runs`（比照對帳） | 前端改兩個按鈕的錯誤訊息；`api-reference.md` 同步 |

## 六、決定（✅ 2026-10-30 定案：全部採建議，D5 採 (b)）

| | 選項 | 建議 |
|---|---|---|
| D1 層一的間隔、上限與截止 | (a) 10 分鐘、每天 3 次、整輪截止 08:45（可設定）、最晚起跑 ＝ 截止 − 業務預算 − 3 分鐘；(b) 其他數字 | **(a)**：整輪（含紀錄寫入）最晚在 08:44:23 結束 |
| D2 層一適用的工作 | (a) `stock_symbol_sync`、`corporate_action_sync`（**只限整輪早退**）、`delisting_reconcile`；(b) 只有前兩者加上連帶重跑 | **(a)**：10-22 的 DB recovery 正是公司行動的整輪早退；對帳自己的抓取也可能遇到 DNS 逾時 |
| D3 暫時性的範圍 | (a) 網路與 DNS、DB 連線類、HTTP 5xx（含 FinMind 的型別傳遞）；(b) 只有網路與 DB | **(a)**：第三方維修常回 503；429 與其他 4xx ⛔ 不算 |
| D4 開關 | (a) 預設關閉，live 依「3.7」的四處設定開啟；(b) 預設開啟 | **(a)**：回滾只要改環境變數；層四與 I-126 是缺陷修正，一律生效 |
| D5 執行所有權 | (a) 手動入口⛔ 不改：**接受**重跑與手動仍可能重疊（重跑開始後才進來的手動會並行）；(b) **四個入口共用執行所有權**：手動撞上回 409、cron 撞上記 Warn、重跑撞上取消 | **(b)**：兩輪 `stock_symbol_sync` 並行時，較舊的快照可能較晚寫入（與對帳的 single-flight 同一個理由）；代價是兩個手動端點的 409 與前端訊息 |
| D6 整輪失敗的計數 | (a) `0/0` ＋ 明確的 failed（比照 `delisting_reconcile`）；(b) `1/1`（比照 `corporate_action_sync`） | **(a)**：`symbols_*` 的單位是標的數；⚠️ `0/0` 只表示沒有回報計數，⛔ 不代表已證實沒有處理任何資料 |

## 七、風險與對策

| 風險 | 對策 |
|---|---|
| 重跑打壞上游（限流、維修中被連打） | 只在暫時性錯誤時重跑；層一間隔 10 分鐘、每天 3 次、盤前截止；429 ⛔ 不算暫時性；FinMind 維持既有的節流與重試 |
| 同一個工作重疊執行 | D5 (b)：四個入口共用執行所有權；`RACE=1` 跑 scheduler 套件的測試 |
| 舊的重跑把新的成功蓋回 failed | 任一入口成功就讓舊鏈失效（「3.2」規則 2） |
| 連帶重跑漏掉成功通知，或反覆觸發 | 狀態同一把鎖；旗標是「最新一輪已成功落盤」、⛔ 不是「今天曾成功」；同一個成功代號最多觸發一次；登記等待時重新確認；所有權釋放後補觸發（「3.2」規則 4） |
| `startRun` 的上限讓慢的 DB 變成「沒有紀錄」 | 只在寫入超過 10 秒時發生，沿用既有的 `runID` 0 處理；log 有 Error |
| 重跑壓到盤前或盤中 | 最晚起跑時刻 ＝ 截止 − 業務預算 − 3 分鐘；`startRun` 與結束寫入都有上限（「3.2」「3.5」） |
| `intraday` 的重試拖長一輪 | 重試用 120 秒到期的 context（涵蓋等待、節流、請求與寫入）；兩輪重疊的可能與現在相同（⛔ 不宣稱消除） |
| 重試寫到一半到期卻被當成成功 | 成功要同時滿足「回傳 `nil`」與「context 還沒到期」；到期就保留失敗計數（「3.3」） |
| 重跑掩蓋持續性的故障 | 每次失敗都留一筆 `job_runs`；用完或截止時 `error` 註明；log 記每一次重跑 |
| 誤把永久錯誤當成暫時性 | 只認型別、⛔ 不比對訊息；有上限，最壞每天多 3 輪 |
| 結束寫入重試全部失敗 | 仍停在 `running` 到下次啟動（剩餘限制，「3.5」）；文件照實寫 |
| 重啟後重跑鏈消失 | 照實寫進文件；啟動回收與狀態頁的判讀規則不變 |
| 開關沒進 live 容器 | 「3.7」的四處設定與部署後的三項確認 |
| 回滾 | 層一～三：改 live 的 `deploy.sh` 為 `false` 後重新部署並確認；層四、I-126、409：退回前一個 image（⛔ 沒有資料遷移） |

## 八、測試與驗證

* Go 單元測試（`backend/scripts/test.sh`，SQLite；重跑相關的時間一律用注入的時鐘、計時器與等待函式，⛔ 不真的等）：
  * `IsTransient`：各型別的正反案例（DNS 逾時／NXDOMAIN、`OpError`、`url.Error` 逾時、`HTTPStatusError` 503／404／429、SQLState `57P03`／`23505`、
    job 自己的 `ctx` 逾時⛔ 不算、解析錯誤⛔ 不算）。
  * `HTTPStatusError` 的 `Error()` 與現在的字串逐字相同（五個 client）；FinMind 的 5xx 重試用完之後，外層錯誤 `IsTransient` 為真、429 為假。
  * 層一：失敗 → 排在 +10 分鐘；成功 → ⛔ 不排；第 3 次之後⛔ 不再排、`error` 註明用完；**最晚起跑時刻**（三個工作各一，含可設定的截止與
    公司行動可調的預算）之後⛔ 不排、`error` 註明；跨日⛔ 不排；手動的失敗⛔ 不排；`Stop()` 取消全部。
  * 紀錄寫入的上限：`startRun` 超過 10 秒即放棄（`runID` 0）；結束寫入重試的總時間⛔ 不超過 133 秒。
  * 重跑鏈：**06:30 失敗 → 06:35 手動成功 → 06:40 的計時器⛔ 不執行**；重跑成功後當天⛔ 不再排；換日重置；
    連帶重跑（對帳等待 → 主檔成功 → 對帳執行一次、計入對帳的上限；對帳額度用完或過了最晚起跑時刻⛔ 不觸發）。
  * 連帶重跑的**時序**（以 hook 控制交錯，`RACE=1`）：
    (i) 對帳讀到主檔還在 running → 主檔成功並通知 → 對帳才登記等待 → 仍會觸發一次；
    (ii) 主檔成功通知時對帳還持有所有權 → 觸發被擋 →「待觸發」留著 → 對帳釋放所有權後補觸發一次；
    (iii) 兩者都⛔ 不會觸發兩次；
    (iv) **06:30 主檔成功 → 06:55 手動同步開始並失敗 → 07:00 對帳因前置不成立失敗** → ⛔ 不觸發（旗標已在 06:55 變假）；之後主檔再成功一次 → 觸發一次；
    (v) 同一個代號：對帳被觸發後又因前置不成立失敗（例如中間又有一輪主檔開始）→ ⛔ 不用同一個代號再觸發。
  * 執行所有權（D5 (b)）：**兩種起跑順序**——重跑先開始時，手動回 409、cron 記 Warn；手動先開始時，重跑取消且鏈結束。
    以 `RACE=1 backend/scripts/test.sh ./internal/scheduler/...` 執行（⚠️ host 的 MemAvailable 要有 850 MB 以上，見該腳本的說明）。
  * 層二：暫時性失敗一次後成功 → ⛔ 不計失敗；永久錯誤⛔ 不重試；兩次都失敗 → 計入失敗；
    **重試 context 的期限**：多批、慢的節流器與慢的 HTTP 下，重試在一輪開始後 120 秒內結束（注入的時鐘），每批的第一次抓取⛔ 不受影響；
    **HTTP 成功、DB 寫入逾時**：重試的回應成功、寫 K 棒時重試 context 到期（假的 candle repo 等到 context 結束才回錯誤）→ 那一批仍計入失敗、⛔ 不算重試成功。
  * **盤中的補回**：第一輪抓取失敗 → 下一輪的回應包含前一輪缺的棒 → 經 `FetchAndStoreIntradayBatch` 寫入後，資料庫裡有那根棒
    （回答 I-122「缺的 K 棒會不會補回」）；13:30 最後一輪失敗、當天沒有下一輪的限制照實寫。
  * 層三：暫時性失敗的標的在第二輪成功 → 從失敗移除；404 ⛔ 不重試；預算用完時保留第一輪的失敗。
  * 層四：寫入失敗 N 次後成功；全部失敗只記 log、那一筆仍是 `running`（剩餘限制的測試）；`AbortRunning` 的新訊息。
  * I-126：整輪失敗記 failed、`0/0`；前置不成立的 `error` 是專屬文字。
  * handler：兩個手動端點在執行中時回 409。
* 前端（`frontend/scripts/test.sh`）：兩個手動按鈕遇到 409 時顯示對應的訊息。
* 反向驗證：拿掉 `IsTransient` 的任一類、拿掉執行所有權、拿掉「成功讓舊鏈失效」、拿掉截止時刻、拿掉跨日檢查，對應的測試要變紅。
* `scripts/check-doc-refs.py` 問題 0。
* dev 的實測（`docker-compose.dev.yml` 的 dev project；只為驗收用的設定放在 scratchpad 的 override 檔，⛔ 不進 repo）：
  * 設定：`JOB_RETRY_ENABLED=true`、`JOB_RETRY_INTERVAL_MIN=1`、`JOB_RETRY_CUTOFF` 設成**當天晚於驗收結束的時刻**（例如 23:30——這樣任何時段都能驗，
    ⛔ 不受 08:45 的預設擋住）；`STOCK_SYMBOLS_SYNC_URLS` 指向會拒絕連線的位址；`STOCK_SYMBOLS_CRON` 設在啟動後幾分鐘。
  * 看到：失敗 → 每分鐘重跑 → 3 次後用完的 `job_runs` 與 log；途中按手動同步得到 409；啟動 log 的開關與最晚起跑時刻。
  * 截止的行為由單元測試（注入的時鐘）驗；dev 只驗「開關、傳遞與實際排程」。驗完 `down`（⛔ 不帶 `-v`）。
* live（使用者部署並開啟之後）：「3.7」的三項確認；觀察 5 個交易日，看 `job_runs` 有沒有「失敗 → 重跑成功」、`intraday` 的失敗輪數有沒有下降、⛔ 沒有重疊執行。

## 九、執行順序

1. 使用者確認本計畫（含 D1～D6）。
2. **階段 A**：`IsTransient` 與 `HTTPStatusError`（五個 client）、層四（I-125）、I-126 → 測試 → review → 使用者 commit。（⚠️ 已實作、待 review：「十二」）
3. **階段 B**：層一～三、執行所有權與 409、前端訊息、設定與兩份 repo 範本 → 測試、`RACE=1` 與反向驗證 → dev 實測 → review → 使用者 commit。
4. 使用者依「3.7」改 live 的兩份設定、重建 image 並部署、完成三項確認；觀察 5 個交易日。
5. 收斂：歸檔（「十」）、I-123～I-126 與 I-122 一起收斂（I-122 的證據到這時才刪）。

## 十、完成後的歸檔位置

* `docs/architecture/data-pipeline.md`：「為什麼不自動重試」改寫成重跑的規則（暫時性的定義、間隔、上限、截止時刻、重跑鏈的規則、連帶重跑、開關）；
  公司行動同步的第二輪。
* `docs/api-reference.md`：`/scheduler/status` 的狀態語意（整輪失敗記 failed 與 `0/0` 的意思、`error` 的重跑文字、`aborted` 的新訊息、
  `running` 的判讀規則與**結束寫入重試全部失敗時的剩餘限制**）；兩個手動端點的 409。
* `docs/yahoo-intraday-integration.md`：同一輪內的批次重試與整輪預算，以及「之後成功的一輪會補回漏掉的棒、13:30 那輪例外」的結論。
* `docs/development-workflow.md`：「新增排程開關要改三個地方」補上 `JOB_RETRY_*` 這個實例（若部署時又踩到差異）。
* `issue.md`：I-123、I-124 收斂時把「⛔ 不追查原因、由重跑吸收」與 I-122 的觀察數字歸到 `data-pipeline.md` 的重跑段。

## 十一、修訂紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1 | 2026-10-30 | 初版 |
| v2 | 2026-10-30 | 依 v1 的 review 修正：#1 層一改成每個工作的最晚起跑時刻（08:45 − 預算），撤掉「07:45 前結束」的推算；層二改成整輪 120 秒的重試預算，撤掉「⛔ 不會和下一輪重疊」的保證（「3.2」「3.3」）；#2 D5 改建議四個入口共用執行所有權，測試涵蓋兩種起跑順序與 `RACE=1`（「3.2」「六」「八」）；#3 補上重跑鏈的規則——同日單一鏈、任一入口成功讓舊鏈失效、連帶重跑計入對帳的上限（「3.2」）；#4 FinMind 的 5xx 也回 `HTTPStatusError`、請求層政策不變（「3.1」）；#5 補上 live 的四處設定、部署後的確認與回滾（「3.7」）；#6 層四的剩餘限制照實寫、列入 I-125 收斂後的文件（「一」「3.5」「十」）；#7 補回的驗證改成走正式抓取流程，保留 13:30 的限制（「八」）。D6 補「`0/0` 只表示沒有回報計數」 |
| v3 | 2026-10-30 | 依 v2 的 review 修正：#1 截止改成**整輪**的截止時刻（預設 08:45、可設定），最晚起跑 ＝ 截止 − 業務預算 − 3 分鐘（涵蓋 `startRun` 10 秒與結束寫入最多 133 秒），`startRun` 加寫入上限（「3.2」「3.5」）；#2 層二的 120 秒改用到期的 context，涵蓋等待、節流、請求與寫入（「3.3」）；#3 連帶重跑改成三個旗標同一把鎖、登記等待時重新確認、所有權釋放後補觸發，並加時序測試（「3.2」「八」）；#4 dev 驗收明列 `JOB_RETRY_ENABLED=true` 與調晚的 `JOB_RETRY_CUTOFF`，`docker-compose.dev.yml` 也加鍵（「3.7」「八」） |
| v4 | 2026-10-30 | 依 v3 的 review 修正：#1 連帶重跑的旗標改成「主檔最新一輪已成功落盤」（任一輪開始時設假、success 落盤後才設真），加「成功代號」讓同一次成功最多觸發一次對帳，補 06:55 手動失敗與同代號的時序測試（「3.2」「八」）；#2 層二的重試成功要同時滿足回傳 `nil` 與重試 context 未到期，到期就保留該批失敗計數，補「HTTP 成功、DB 寫入逾時」的測試，⛔ 不擴大處理 I-103（「3.3」「八」） |

## 十二、實作備註

### 階段 A（2026-10-30，前兩輪 review 各 1 項已修正，⚠️ 待 review）

改了什麼（依「3.1」「3.5」「3.6」）：

* `market.IsTransient` 與 `HTTPStatusError`（`transient.go`）；五個 client 的非 2xx 分支改回 `HTTPStatusError`，文字逐字不變。
* `startRun` 加 10 秒的寫入上限；`finishRunStatus` 退避重試（最壞 133 秒），**回傳寫入是否成功**——階段 B 判斷「主檔最新一輪已成功落盤」要用；
  重試的等待序列放在 `Scheduler.finishRetryBackoffs`（nil 時用預設），只有測試會改它。`intraday` 的 `skipped` 改走 `finishRunStatus`。
* **review 修正**：「寫入成功」必須是**確實寫進一筆存在的紀錄**。初版只看 `err == nil`，而 UPDATE 零筆在 SQL 上不是錯誤——
  `startRun` 失敗拿到 0、之後 DB 恢復時，`finishRunStatus` 會在表裡一筆都沒有的情況下回報成功，誤導階段 B 的「主檔最新一輪已成功落盤」。
  改成：`store.JobRunRepo.Finish` 一筆都沒更新到時回 `store.ErrJobRunNotFound`；`finishRunStatus` 在 `runID` 為 0 時⛔ 不送 UPDATE、回 false，
  遇到 `ErrJobRunNotFound` ⛔ 不重試、回 false。
* **第二輪 review 修正**：UPDATE 零筆⛔ 不等於紀錄不存在——mysql 的 `RowsAffected` 預設計「值有變的列」，同一秒內以相同的值重寫已完成的紀錄會得到 0。
  零筆時改成再查一次紀錄存不存在（`confirmExists`）：存在 → 成功；不存在才回 `ErrJobRunNotFound`。
* I-126：`stock_symbol_sync` 整輪失敗記 `failed`、`0/0`；對帳前置不成立記 `delisting_reconcile:stock_symbol_sync_not_ready`。
* `AbortRunning` 的訊息改成中性。
* 文件：`api-reference.md` 依「3.5」補上 `running` 的判讀規則（結束狀態寫不進去這條路徑與剩餘限制）、`aborted` 的語意，
  以及主檔整輪失敗與對帳前置不成立的新字串（這幾句描述的是階段 A 之後的現況，⛔ 不等到收斂才改）；`issue.md` 一處 `scheduler.go` 的行號引用
  改寫成不帶行號的「`scheduler.go` 的 `runIntradayJob()`」（階段 B 還會再改這個檔，帶行號會一直漂）。

驗證：

* `go vet ./...`：rc 0。`go build ./cmd/...`：通過。改到的檔 `gofmt` 乾淨。
* `go test -vet=off -count=1 ./...`：13 個套件 12 個通過；**唯一失敗**是 store 的 `TestZoneIdentityListTradingDaysReturnsDistinctDatesNewestFirst`——
  ⚠️ **既有的時間炸彈**：測試資料寫死 2026-08-14～08-18，查詢只回看「今天 − 30 天」，2026-09-13 之後必然失敗；在 HEAD（`7fd3962`）的隔離 clone 上同樣失敗。⛔ 不在本計畫修。
* ⚠️ `backend/scripts/test.sh` 在這台 host 目前的可用記憶體下（mem-guard 下修到約 460m）**一定印 `FAIL`**：`go vet` 本身會過，
  但 `go test` 內建的那一輪 vet 分析 `modernc.org/sqlite/lib` 時被 OOM 砍掉，各套件的測試仍是 `ok`。所以本階段的測試結果以 `-vet=off` 為準（`go vet` 另外跑）。
* 新增的測試：`market/transient_test.go`（判定的正反案例、五個 client 的文字與型別；FinMind 的 5xx／429 因退避常數要等約 7 秒）、
  `scheduler/job_run_write_test.go`（結束寫入的重試與每次的期限、全部失敗仍未寫入、133 秒的上界、`startRun` 的上限、I-126 的兩項）、
  `store/job_run_repo_test.go` 補回收訊息的斷言。review 修正補上：**真實 SQLite repo** 的 id 0／不存在的 id（store 與 scheduler 各一，
  後者重現「表裡 0 筆卻回報成功」）、`runID` 0 時⛔ 不送 UPDATE、紀錄不存在時⛔ 不重試；第二輪補「相同的值重寫」與 `confirmExists` 的存在／不存在。
  ⚠️ sqlite 的 `RowsAffected` 計符合條件的列、碰不到零筆變更，所以 mysql 那條路徑是直接驗 `confirmExists`；repo 層沒有 mysql 的測試（I-054）。
* 反向驗證 22 項（在隔離 clone 逐一改壞、`go test -vet=off`；對照組全綠）——第二輪的 2 項：`confirmExists` 一律回不存在、一律回存在；
  第一輪的 3 項：store 的 `Finish` ⛔ 不檢查筆數、拿掉 `runID` 0 的判斷、紀錄不存在也重試；其餘 17 項：拿掉 `IsTransient` 的各類（`HTTPStatusError`、SQLState、`OpError`、`url.Error` 逾時、
  `ErrUnexpectedEOF`）、5xx 門檻改成 ≥ 400、DNS 一律算暫時性、Yahoo 配息與 FinMind 改回一般錯誤、TWSE ISIN 的文字被改、結束寫入⛔ 不重試、
  結束寫入改用呼叫端的 ctx、`startRun` 拿掉上限、預設等待改短、主檔整輪失敗改回 `0/1`、對帳改回分類器、回收訊息改回舊文字——都讓指定的測試變紅。

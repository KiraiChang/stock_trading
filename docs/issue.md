# ISSUE：遇到的問題與已知限制

記錄實際發生過的 bug、矛盾結果、文件/程式碼不一致，以及設計上刻意接受的
限制。跟「想做的優化」無關的項目放這裡；未來想做的功能擴充記錄在
[todo.md](./todo.md)。

## 使用說明

- **狀態**：`待修復` / `修復中` / `已修復` / `已實作／待 review` / `待決策` /
  `已知限制（不計畫修復）`
  - `已修復` 與 `已實作／待 review` 的差別是**有沒有經過 review**：改完先標後者並保留
    修復方式與計畫書，review 通過才收斂移除（見下方「移除條目前要先反轉依賴」）。
  - `待決策`用於**還不知道要不要修**的項目——需要先取得外部事實（上游狀態、實測數據）
    才能決定處置方向。它與`待修復`的差別是後者已經確定要修、只是還沒動工。
  - `已知限制`後面用括號寫清楚是哪一種（不計畫修復／操作程序上避免／部分已解等），
    這是既有慣例。
- **嚴重度**：`高`（結果矛盾/資料錯誤）/ `中`（誤導但不影響核心功能）/
  `低`（文件或註解落後，不影響 runtime）
- 新增項目時往下加一筆，編號遞增（`I-0xx`）。修復後若仍需短期追蹤，先把「狀態」改成
  `已修復` 並補上「修復方式」；若修復紀錄已移到對應主題文件或 review 文件，
  則可從本清單移除。
- **編號只增不重用。** 已移除的條目編號不得再發給新問題——程式碼註解與其他文件會留著舊 ID，
  重用會讓兩件無關的事共用一個代號。**`I-070` 已經發生過一次**（先發給 T-045 的事件鏈墓碑，
  移除後又發給 T-040 的 `keep_symbols` 靜默丟棄，兩筆現在都已收斂），
  見 `todo.md` T-045 那段的註記。
- **下一個新編號從 `I-103` 起算。**（I-101 / I-102 於 2026-09-01 發出——前者來自 live 的 indicator upsert 溢位，後者由它的 review 分出；I-100 於 2026-09-01 發出，由 `todo.md` T-068 同日改列——**T-068 編號不回收**；**I-099 於 2026-08-31 發出後同日作廢**——誤把 `deploy.sh` 的保守預設當成與 live 的衝突，實際上該檔是範本、所有開關一律預設 `false` 是既有慣例；**編號不回收**；I-098 於 2026-08-31 由 I-096 的 review 發現分出；I-081～I-083 於 2026-08-21 發出（**I-081 / I-082 於 2026-08-27 隨 `todo.md` T-055 收斂**），I-084～I-087 於 2026-08-24 發出，I-088～I-092 於 2026-08-25 發出（**I-091 於 2026-08-28 收斂**），I-093 / I-094 於 2026-08-26 發出（I-093 已於同日收斂，**I-094 於 2026-08-28 收斂**），I-095～I-097 於 2026-08-27 發出，其中 **I-097 於同日改列 `todo.md` T-064**——編號**不回收**。）
  **發出新編號時記得把這一行一起往前推**——上一次就是漏了這步，I-089 發出去之後
  這裡還寫著「從 I-089 起算」，差一點又重用一次（I-070 已經發生過）。
  檔案裡看得到的最大是 I-102（2026-09-01 發出，**下一個可用的是 I-103**——I-096 / I-098
  已於 2026-08-31 收斂、I-099 已發出並作廢、T-068 改列為 I-100，編號都不回收），但被移除的條目
  （I-040 / I-056 / I-069 已於 2026-08-18 收斂，I-076 於 2026-08-19 收斂，
  I-083 / I-084 於 2026-08-24 收斂，I-086～I-090 於 2026-08-25 收斂，
  I-093 於 2026-08-26 收斂，I-070～I-072 更早）都佔用過編號。
  **不要用「檔案裡最大值 + 1」決定編號**——被移除的條目正是看不見的那些；
  必要時翻 git log 或本節的收斂紀錄。
- **移除條目前要先反轉依賴。** 主題文件與程式碼註解常寫「見 issue.md I-0xx」，
  這種寫法讓 issue.md 變成權威來源，一刪就斷鏈。移除前先把說明**內嵌**到對應主題文件，
  再把所有引用改指向該文件。收斂後用下面這條檢查沒有殘留：

  ```bash
  comm -13 <(grep -o '^### I-0[0-9][0-9]' docs/issue.md | sed 's/### //' | sort -u) \
           <(rg --no-filename --only-matching --no-messages \
                --glob '!**/node_modules/**' --glob '!**/dist/**' \
                --glob '*.{md,go,ts,svelte,py,sh,yml,yaml,sql}' \
                'I-0[0-9][0-9]' . | sort -u)
  ```

  **用 `rg` 而不是 `grep -r`，兩個理由都是踩過的**（2026-08-25 re-review 修正）：

  * **副檔名清單每一種都是踩過才加的，不要精簡它。**
    * `.ts` / `.svelte`：舊版只掃 md/go/py/sh，漏掉整個前端——`Scheduler.test.ts` 曾經
      留著一個 `issue.md I-090` 的活指標，那次檢查完全沒抓到。
    * **`.yml` / `.yaml` / `.sql`**（2026-08-28 補）：收斂 I-091 時，
      `docker-compose.yml` 與 `docker-compose.dev.yml` 各留了一個
      `issue.md I-091` 的活指標，**檢查指令整批看不到**——因為 compose 是 `.yml`
      而清單裡只有 `.yaml`。migration 檔頭（`.sql`）與 `backend/config.yaml` 同理。
      **這與下一點是同一類錯誤：檢查本身有洞，而且不會報錯。**
  * **`grep -rho … | grep -v node_modules` 濾不掉任何東西。** `-h` 已經把檔名拿掉了，
    留下的每一行就只是 `I-0xx` 本身，永遠不含 `node_modules` 字串——那個後置過濾
    從第一天起就是空操作。`grep -r` 照樣會遞迴進 `node_modules`；舊指令之所以沒有
    出現誤報，只是**那裡剛好沒有符合 `I-0xx` / `T-0xx` 的內容落在被掃的副檔名裡**，
    是運氣不是過濾。`rg` 的 `--glob '!**/node_modules/**'` 才是真的排除。

  列出的 ID 必須**只剩明確標為歷史沿革的引用**（「原記於…」「當時編號…」），
  不能有任何「見 I-0xx」形式的活指標。
  **本節自己會出現在輸出裡**（上面提到 I-040 / I-056 / I-069 / I-070～I-072 / I-076 /
  I-081～I-084 / I-086～I-090 / I-093、I-096、I-098、已作廢的 I-099、本檔現有的 I-100 / I-101 / I-102
  與下一個可用的 I-103），
  那是預期的，不是殘留。

---

### I-078：T-048 身分層有兩條路徑在驗收母體裡從未被執行

| 欄位 | 內容 |
|---|---|
| 狀態 | 已知限制（不計畫單獨修復）——**兩個關閉條件已關掉一個**：收攤那半於 2026-08-27 以 live 證據關閉，alias 那半仍未觸發 |
| 嚴重度 | 中（不影響現行結果。**剩下的** alias 路徑**單元層已覆蓋**，缺的是 integration／production 證據——真的被走到時沒有真實母體的行為可對照） |
| 分類 | Go / Python / SR Zone / 身分追蹤 / 驗證缺口 |
| 發現日期 | 2026-08-20 |
| 來源 | T-048 全案 review（階段 A～E 實作後盤點） |

四檔 21 階、84 次分析的 as-of 階梯是 T-048 唯一的端到端證據，但這份母體裡有兩條
設計上很微妙的路徑**一次都沒被觸發**（**指這份 as-of 階梯母體，不是指沒有測試**——
兩條路徑在單元層都有斷言，見各條的說明）：

* **缺席收攤（`EXPIRED_BY_ABSENCE`）**：兩輪階梯裡一次都沒被觸發。
  `ZoneIdentityRepo.ListLive` 的註解說明了這段最容易寫錯的地方——資格用
  `<= maxObservedAbsences` 而不是 `<`，否則剛好累到上限的身分再也撈不出來、
  永遠不會被判失格、收攤流程整條變成不可達的死碼。**這個陷阱的反面（正確版本）
  當時只有單元測試證明。**

  ⚠️ **本筆原本把觀察對象寫成「`zone_instances` 出現 `EXPIRED`」，那是錯的**
  （2026-08-27 更正）。依 `migrations/postgres/067_zone_identity.sql:31-32`，
  兩張表的 `state` 值域**刻意不同**：

  | 表 | `state` 值域 | 語意 |
  |---|---|---|
  | `zone_instances`（身分） | `ACTIVE` / `SPLIT` / `MERGED` / `RESHAPED` | **沒有 `EXPIRED`**。身分不因缺席而終止 |
  | `zone_role_incarnations`（一世） | `ACTIVE` / `TESTING` / `INVALIDATED` / `EXPIRED` | `EXPIRED` ＝「我們不再認得它」 |

  067 明寫「`INVALIDATED` 與 `EXPIRED` **兩者都不終止身分本身**，同一個價位之後可以開下一世」，
  `buildZoneIdentityWrite` 的 doc comment 也是「失格的 → **一世**收成 EXPIRED」。
  所以「`zone_instances` 出現 `EXPIRED`」是個**永遠不可能成立**的條件，
  而原文引用的「ACTIVE 293 / … / `EXPIRED` 是 0」也是查錯表得到的數字。
  **正確的觀察對象是 `zone_role_incarnations.state='EXPIRED'` ＋ `expired_at`。**
* **alias 備援命中（`matched_by_alias`）**：兩輪階梯都是 0（階段 C 已記錄）。
  三段關聯決策的第一段（既有鏈命中）把所有情況都吃掉了。

  **單元層已經覆蓋這條路徑**（`sr_zones_event_identity_test.go`）：
  `TestBuildEventIdentityWriteFallsBackToAliasWhenRoleFlipped`（`:417`，斷言
  `MatchedByAlias == 1`）、`...WhenBoundaryDrifted`（`:439`）、
  `...CurrentMapWinsOverAlias`（`:457`，證明 current map 優先於 alias），
  另有 `TestAliasIndexDropsIdentitiesTheMatcherGaveUpOn`（`:631`）與
  `TestSummarizeIdentityStatsComputesAliasHitRate`（2026-08-31 起吃
  `store.SRIdentityStatsAggregate`，測的是 `alias_hit_rate` 的推導；名稱未變）。
  **缺的是 as-of 階梯／integration／live 母體的自然命中**，不是缺測試。

**成因是母體太小而不是實作有問題**（**立案當時**：21 個交易日、4 檔，身分還來不及缺席到失格；
母體現況見 [`todo.md`](./todo.md) T-049 前置②的分析次數表——**本筆的母體確實是
`stock_sr_zone_analyses`**，與 I-074 的 replay 母體是兩回事）。
真正的解法是補分析排程（**已於 2026-08-20 上線**，現況見
[`architecture.md`](./architecture.md)「SR 分析的兩個時段共用一個執行所有權」），
不是為這兩條路徑另外造假資料。
**這個判斷對收攤那半已被 2026-08-27 的 live 查證證實**（排程上線第 2 個交易日就觸發）；
alias 那半則仍未觸發，見下方關閉條件。

**關閉條件拆成兩段**：

* **缺席收攤**：分析排程上線、production 母體累積到身分會自然失格後，
  確認 `zone_role_incarnations` 出現 `state='EXPIRED'`（**不是 `zone_instances`**，見上），
  且行為與單元測試一致。
  ✅ **已達成（2026-08-27 查證 live）**——詳見下方「收攤路徑的 production 證據」。
* **alias 備援**：不能假設分析排程一定會讓 `eventIdentityStats.MatchedByAlias` 自然非零——
  T-048 實測中第一段既有鏈命中把多數情況吃掉了。排程上線後先觀察一段時間；若仍為 0，
  改由 targeted integration/live fixture 或 `GET /sr-zones/identity-stats` 的
  `alias_hit_rate`（已可用）證明這條路徑，
  而不是把本筆卡死在不可控的自然觸發上。
  ⬜ **仍未達成（2026-08-27 查證 live）**：`sr_identity_stats` 自 2026-08-21 起 88 筆，
  `matched_by_alias` 合計 **0**（`matched_by_chain` 268 / `matched_by_current` 121 /
  `unmatched_keys` 0）。觀察期已過，**該改走 fixture 或 metric 那條路**。

#### 收攤路徑的 production 證據（2026-08-27 查 live）

分析排程於 2026-08-20 上線後，缺席收攤**自 2026-08-24 起實際發生**，且與單元測試的斷言逐項吻合：

| 單元測試斷言 | 測試 | live 實際 |
|---|---|---|
| 一世 `state='EXPIRED'` ＋ `end_reason='EXPIRED_BY_ABSENCE'` ＋ `expired_at` 有值 | `TestBuildZoneIdentityWriteExpiresIncarnationAndRecordsReason` | **38 筆，`ended_at` / `expired_at` 皆 38/38 有值** |
| transition 存在，且 `to_state='EXPIRED'`、`from_state='ACTIVE'` | `TestBuildZoneIdentityWriteExpiresIncarnationAndRecordsReason`（`:136-145`） | **48 筆，`from_state` 全為 `ACTIVE`** |
| transition 帶 `incarnation_uid` | `TestBuildZoneIdentityWritePushesExpiredPastTheAbsenceLimit`（`:310-317`） | **38 筆帶 `incarnation_uid`**（差額見下） |
| 缺席次數推過上限（與 `ListLive` 的 `<=` 握手，避免重複收攤） | 同上（`:308-310`） | **48 個身分 `observed_absences=4 > 3`；48 筆 transition 對 48 個 distinct zone_uid，無重複收攤** |

**48 vs 38 的差額是正常的**：`sr_zones.go:1947` 的 `if prev.IncarnationUID.Valid`——
AT_ZONE 期間不開一世（067：「AT_ZONE 是『方向暫時無法解析』不是角色」），
那些身分只有 transition、沒有一世可收。

**那 48 個 `observed_absences=4` 的 `ACTIVE` 身分不是異常資料**：身分仍在（規格如此），
只是被 `ListLive` 的次數軸擋在候選集合外，所以也不會被重複收攤。
`zone_instances.ended_at` 維持 `NULL` 同理——結束的是一世，不是身分。

---

### I-079：`zone_key_aliases` 每身分 8 筆上限已有 23 個身分撞頂

| 欄位 | 內容 |
|---|---|
| 狀態 | 已知限制 |
| 嚴重度 | 中（現在不影響關聯，但超出的舊 key 已經永久查不回來） |
| 分類 | Go / DB / SR Zone / 身分追蹤 |
| 發現日期 | 2026-08-20 |
| 來源 | T-048 全案 review，對 84 次分析的 `zone_key_aliases` 實測 |

`zone_key_aliases` 每個 `zone_uid` 只保留最新 8 筆，prune 在寫入的同一個交易內做
（見 [`database-schema.md`](./database-schema.md)）。實測 329 個身分用過的 key 數分布：

| 用過的 key 數 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 |
|---|---|---|---|---|---|---|---|---|
| 身分數 | 227 | 28 | 20 | 12 | 9 | 5 | 5 | **23** |

**102 個身分（31%）漂移過 key**，而**卡在 8 的那 23 個實際上是「≥ 8」**——超出的部分
已經被 prune 掉了。只有 21 個交易日就這樣，母體變大後撞頂的比例只會更高。

**目前不影響正確性**：關聯決策的第一段（既有鏈命中）不需要 alias，所以
`matched_by_alias` 一直是 0（見 [I-078](#i-078t-048-身分層有兩條路徑在驗收母體裡從未被執行)）。
**但一旦需要靠 alias 回溯歷史 key，撞頂的那些就查不到了**，而且不會報錯——
查不到與「這個 key 從來不屬於任何身分」長得一模一樣。

**要決定的事**（不急，但不該默默留著）：上限值 8 是否該隨母體調整、或改成
「保留最近 N 個交易日」而不是「最近 N 筆」。調整前要先看 alias 表的實際成長速度，
避免把一個有界的表變成無界。

**承接觸發點**：分析排程已於 2026-08-20 上線（每交易日 22 筆分析），母體累積到一定量後
重新量測「alias 數撞頂」（`alias_count >= 8`）的身分比例。

**這是兩個不同的問題，資料來源也不同**：

* **alias 備援有沒有進入日常路徑**——看 `GET /sr-zones/identity-stats` 的
  `alias_hit_rate`（`matched_by_alias / matched_total`）。
* **撞頂比例**——⚠️ **要直接統計 `zone_key_aliases`**（每個 `zone_uid` 的 alias 筆數分佈）。
  上面那個端點**答不出這一題**：它只有整體的關聯決策計數，沒有 per-zone 的 alias 數量。

兩者任一惡化再規劃上限策略。現在保留為已知限制，不單獨開修法。

---

### I-075：重跑選池會因池外資料變舊而「靜默通過」

| 欄位 | 內容 |
|---|---|
| 狀態 | 已知限制（操作程序上避免，暫不改） |
| 嚴重度 | 中（不影響 runtime，但會產生看起來正確的錯誤結論） |
| 分類 | Python / 評估標的池 / 驗證方法 |
| 發現日期 | 2026-08-18 |
| 來源 | T-040 重跑選池的可行性盤點 |

`evaluation_universe_sync` **只維護池內成員**的日 K，池外標的因此逐日變舊；而
`selection_report.py` 的 stale 容忍窗只有 **3 個市場交易日**，且市場交易日是從全庫
distinct 日期算的——於是那個窗會被池內成員自己定義，池外標的整批落在窗外被排除。

結果是重跑選池時候選母體塌縮成「目前的池」，**報告會顯示「池沒變」且沒有任何異常訊號**。
那是循環論證，不是驗證。

**避免方式**：重跑選池前先把池外標的回補到最近 3 個交易日內（2026-08-18 實測需回補
706 檔，FinMind 5 req/min 下約 2.4 小時）。完整數據、成因與判讀方式見
[`evaluation-universe-selection-plan.md`](./evaluation-universe-selection-plan.md)
「重跑選池前必須先回補池外標的」。

**為什麼不直接修**：可行的修法（stale 判定改用外部交易日曆、或報告在候選數暴跌時警告）
都要先決定「選池多久該重跑一次」，而 T-040 明訂**不自動重選池**，目前沒有重跑需求。
在有需求之前，這是操作程序的問題而不是程式的問題。

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
   目前對真實 MySQL 有執行證明的**只有兩個 repo 寫入路徑**，都是靠
   `TestMySQLMigrationsRealValuesFitAllColumns` 順帶涵蓋的：

   * `CorporateActionRepo.Upsert`（`ON DUPLICATE KEY UPDATE`）
   * `EvaluationUniverseRepo.Upsert`（2026-08-17 T-040 Step 5 新增，同樣是
     `ON DUPLICATE KEY UPDATE` 分支）

   其餘 `internal/store` 的查詢與寫入仍只跑 sqlite。**每新增一個有 mysql 分支的 repo，
   這一項的缺口就多一個**——`EvaluationUniverseRepo` 的 `ListActive` / `SetActive`
   在真實 MySQL 上從未執行過。要補的話是讓 repo 測試整批對著真實 MySQL 跑一輪。

   **2026-08-31 又多一個**：`SRIdentityStatsRepo.Summarize` 用了
   `COUNT(*)`、`COALESCE(SUM(...), 0)` 與 `CASE WHEN <boolean> THEN 1 ELSE 0 END`。
   三個 engine 的布林表示不同（postgres `BOOLEAN`／sqlite 0/1／mysql `TINYINT`），
   寫法選成三者都成立的形式，但**只有 sqlite 被實際執行過**。
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

### I-074：Lifecycle Engine 的 RR 解耦，decision replay 已跑但一次都沒觸發到

| 欄位 | 內容 |
|---|---|
| 狀態 | **待決策**（2026-09-01：replay 已執行，三個欄位零差異、逐列 0 筆轉移，但**符合完整觸發 predicate 的列數為 0**——是「未觸發」不是「已證明無影響」。**舊關閉條件形式上已達成但有缺陷，已重寫**，見下方「關閉條件（2026-09-01 重寫）」。待決策：投入定向 cohort，或明示接受並轉為已知限制。**在二者之一完成前不得移除本筆**） |
| 嚴重度 | 中（行為已改變且已上線，但驗證深度不足） |
| 分類 | Python / SR Zone / Lifecycle |
| 發現日期 | 2026-08-13（2026-08-18 確認缺口仍未關閉） |
| 來源 | Lifecycle Engine 抽離（原 `todo.md` T-044，已於 2026-08-18 收斂移出）P0 實作後的驗證盤點 |

`lifecycle_engine.py` 的抽離把 `rr_gate.qualified` 從 `CONTINUATION` 的判定條件移除（分層原則與
四套同名詞彙對照見 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「分層原則：lifecycle 不看 RR」）。
**這是一個已經上線的行為改變**，計畫要求用 decision replay 對真實資料比對
`final_entry_state` / `lifecycle_phase` / `market_bias` 的分佈變化來評估影響。

**立案時（2026-08-13）判定「跑不了」，理由是 `stock_sr_zone_analyses` 只有 4 檔 / 20 次分析。
那個理由於 2026-09-01 證實是錯的**——replay 根本不讀那張表，見下方
「replay 母體是 candles」。真正擋住本筆的東西直到 2026-09-01 才量出來：
不是資料量，是**沒有任何一列符合觸發條件**。

**現有證據的等級要說清楚**：抽離後 428 支既有測試全數通過，
但那**不是**「沒有行為改變」的證據——它是「沒有任何既有測試涵蓋 RR 解耦那條路徑」的證據。
行為改變是**結構上可證明**的，並由兩支測試鎖住：

* `test_continuation_only_needs_price_evidence`——延續只看三項價格證據
* `test_widened_path_previously_testing_now_continuation`——真正變寬的那條路徑

**但前者無法防守「RR 被加回來」**：`resolve_lifecycle` 簽章裡沒有 `rr_gate`，
真要加回來會是新增參數，那支測試照樣綠燈。目前靠 `sr-zone-scoring.md` 的
「請不要加回去」與本筆記錄把守。

#### replay 母體是 candles，不是 `stock_sr_zone_analyses`（2026-09-01 更正並收斂）

**本筆從立案到 2026-08-31 的「母體太小」推理，量錯了對象。**
`run_decision_replay()`（`evaluation.py:2254`）的資料來源是 `_load_db_sources()`——
**`candles`**；整個 `python/` 沒有任何一處 reference `stock_sr_zone_analyses`（grep 0 命中）。
決定 cohort 的是**每檔日 K 根數**與 `replay_max_rows`（總預算跨股票均分，
`MIN_ROWS_PER_SYMBOL = 5`）。

所以：

* **分析排程（`SR_ANALYSIS_CRON`）從來不是本筆的前置。** 那些「開了排程才做得了
  I-074 的分佈比較」的註解已於 2026-09-01 一併修掉（`backend/config.yaml`、
  `docker-compose.yml`、`docker-compose.dev.yml`、`deploy.sh`）。分析排程仍是
  [`todo.md`](./todo.md) **T-049 前置①**（新舊兩套 active 事件集合的逐日並行比對）
  的必要條件——**那一筆確實讀 `stock_sr_zone_analyses`**，不要一起拿掉。
* 曾經記錄的分析次數成長（4 檔/20 次 → 11 檔/155 次，2026-08-13 ~ 08-31）
  **與本筆無關**，已從本筆移除；它仍是 T-049 的 dated evidence，保留在該筆。
* 實際用到的母體：11 檔各有 310～4882 根日 K（2026-09-01 實測），
  抽出 200 列 as-of。日 K 從來就夠，**本筆從頭到尾都不曾被母體擋住**。

#### 2026-09-01 replay 實測：零差異，但屬於「未觸發」

以 `scripts/run-evaluation.sh` 走 **live 唯讀**（同一顆 `sr_scoring_v4.joblib`），
before ＝ `ecbc141^`、after ＝ `ecbc141`，11 檔 200 列、cohort 10/10 核對通過。
數字與完整判讀已歸檔到 [`sr-zone-scoring.md`](./sr-zone-scoring.md)
「已知並接受的行為改變 › decision replay 實測（2026-09-01）」。

| 觀察 | 結果 |
|---|---|
| `final_entry_state` / `market_bias` / `lifecycle_phase` 分佈 | **完全相同** |
| 逐列翻轉 | **0 筆**（after 相對 before 新增的 `CONTINUATION` 列數為 0） |
| `CONTINUATION` 出現次數 | before 1 / after 1——且那列（`5490`，2026-08-02）`rr_gate.qualified=true`，**被移除的條件對它本來就不起作用** |
| 符合**完整 predicate** 的列數 | **0**（＝翻轉列數）。完整 predicate ＝ `CLOSE_RECLAIM` ＋ 上行跟隨 ＋ 動能確認 ＋ 明確突破 ＋ `rr_gate.qualified=false` |
| 搜尋母體（**不是**候選集合） | before 版 RR 不合格的 `CONFIRMED` 54 列 ＋ `TESTING` 33 列 ＝ **87 列**。⚠️ **不要稱它們為「只差 RR」**——把搜尋母體講成候選集合，等於把「實際命中 0 列」說成「87 列瀕臨翻轉」 |
| 由既有欄位推導的漏斗 | 65（上行跟隨）→ 5（＋動能確認）→ 3（＋明確突破，**只能近似**）→ 2（＋RR 不合格）。⛔ **那 2 列是偽陽性**：`5490` 2026-07-30 與 `6243` 2026-08-13 依推導應翻轉，實際 after 版仍是 `CONFIRMED`。**成因是 replay row 的 `primary_zone` 是「排序第一筆」，不是 lifecycle 用的 `_pick_primary_zone()` decision primary zone**——消去法見 [`sr-zone-scoring.md`](./sr-zone-scoring.md) |

**所以這一輪回答不了本筆要問的問題**：`CONTINUATION` 這條路徑在自然樣本裡只有 0.5% 的出現率，
**隨機加大列數效益很低**。要真正驗到，需要**定向 cohort**——刻意挑含
「收復 ＋ 上行跟隨 ＋ 動能確認 ＋ 明確突破」形態的標的與期間。要不要投入這個成本待決策；
在那之前，這個行為改變的接受仍然是**明示的決定**，不是「驗過沒問題」。

⚠️ 順帶記一條方法論教訓：`lifecycle_phase` 原本不在 replay 報告裡，
**分佈全同時分不出「未觸發」與「無影響」**。該欄位已於本次補進
`_decision_fields_from_summary()`，並一併補進 `planned_fields` 與 `outcome_summary`
（`lifecycle_phase_counts` / `by_lifecycle_phase`）與對應測試。

#### 「dev 沒有 model bundle」不是 blocker（2026-09-01 定案）

| 環境 | model bundle | 依據 |
|---|---|---|
| **live** | ✅ **有**——`sr_scoring_v4.joblib`，2026-08-11 訓練 | 2026-08-31 實測 `GET /sr-scoring/model-status`（live python-server，唯讀）；2026-09-01 再以 `MODELS_DIR` 直接確認檔案 |
| **dev** | ❌ 沒有（`model_available: false`） | 2026-08-27 實測（原記於 `todo.md` T-066，已於 2026-09-01 收斂） |

**`model_available: false` 是事實，但「所以驗不了」是路徑選擇的結果。**
`scripts/run-evaluation.sh` 從設計上就接 **live DB 唯讀**、唯讀掛載 live 主機的
`MODELS_DIR`（`/opt/stacks/scripts/stock_trading/python/models/`），預設不帶
`--write-db`。那顆 bundle 一直都在，所以本筆從來沒有被 model bundle 擋住。

**與 CLAUDE.md「驗收走 dev」的調和**：CLAUDE.md 禁的是拿 live 做**測試資料、
migration 驗證與清空資料**；replay 全程不寫任何一張表，不在其列。
（走 dev 的代價不只 bundle：dev 只有零星日 K，要先把資料搬進去才有母體。）

⚠️ **通則**：下次遇到「某環境缺某資源所以驗不了」，先確認是不是**只有那一條路徑**缺。

#### 關閉條件（2026-09-01 重寫）

**舊條件已於 2026-09-01 執行完畢**：「跑 `MODE=replay scripts/run-evaluation.sh` 比對
`final_entry_state` / `lifecycle_phase` / `market_bias` 三個欄位的分佈」——三個分佈都比了，
結果是零差異。**但那個條件本身有缺陷**：它沒有要求 cohort 必須**命中被改動的路徑**，
所以一個「一次都沒觸發」的 run 也能形式上滿足它，而那並沒有回答本筆要問的
「這個已上線的行為改變影響多大」。

**新條件（二選一即可關閉）**：

1. **命中式**：跑一次 decision replay，並提出**至少一筆逐列證據**——同一個
   `(symbol, as_of)` 上 **`before != CONTINUATION` 且 `after == CONTINUATION`**，
   且該列符合完整觸發 predicate（`CLOSE_RECLAIM` ＋ 上行跟隨 ＋ 動能確認 ＋ 明確突破
   ＋ `rr_gate.qualified = false`），並記錄 before/after 在該列上的下游欄位差異。

   ⛔ **不接受 aggregate 當命中證據**（例如「after 的 `CONTINUATION` 總數大於 before」）——
   總數可能被反向轉移抵銷或混淆，2026-09-01 那輪的 `qualified` 淨 +3 底下藏著 37 列
   雙向流動就是現成的例子。**要逐列。**

   ⚠️ **實務前置**：用現有欄位重建 predicate 會有偽陽性（見上表「由既有欄位推導的漏斗」）。
   **根因是 replay row 的 `primary_zone` 是 `_sort_zone_scores()[0]`，而 lifecycle 用的是
   `_pick_primary_zone()` 選出的 decision primary zone**——兩顆不是同一個。
   所以要證明命中，得先輸出 **decision primary zone 或 `clear_zone_breakout` 本身**
   （或直接一個 predicate flag）。`event_signal` / `structure_state` 也值得一併帶出來
   （可省掉逐列的消去法推理），但**它們不是偽陽性的成因**。
2. **明示接受**：判定「定向 cohort 的成本不值得」，把本筆轉為**已知限制**並在
   [`sr-zone-scoring.md`](./sr-zone-scoring.md) 寫明「此行為改變在自然樣本上未觀測到，
   接受是明示決定而非實測結論」。

⚠️ **在二者之一完成前，本筆不得移除**。目前的接受仍是**明示的決定**
（2026-08-13 決定放寬、2026-08-18 確認維持、2026-09-01 實測未觸發），不是「驗過沒問題」。

**待決策**：要不要投入定向 cohort（條件 1），或直接走條件 2。

---

### I-100：decision replay 沒有 as-of 上界，cohort 隔天就重現不了

| 欄位 | 內容 |
|---|---|
| 狀態 | 已知限制（**已造成一次實際後果**，見下） |
| 嚴重度 | 中（不影響 runtime，只影響**驗收證據能不能被獨立複核**） |
| 分類 | Python / SR Zone / 驗證工具 |
| 發現日期 | 2026-09-01 |
| 來源 | 原 `todo.md` T-066（decision replay 前後比對，已於 2026-09-01 收斂）執行時發現——四份 report 的數字在當天之後就無法用同一條指令重建 |

⚠️ **這一筆原本寫在 `todo.md` T-068，2026-09-01 同日改列到這裡**（編號 T-068 不回收）。
理由是 CLAUDE.md 的分流規則：它是**已經發生的已知限制**，不是待規劃的優化。
下方「可能做法」是它的解法草案，不是另一個 todo 項目。

#### 問題

`fetch_candles()` 取的是**最新** N 根（`ORDER BY ts DESC LIMIT` 後反轉，`python/db.py`），
而 `evaluation.py` 的 CLI **沒有 as-of 截止參數**——`--limit` 只能控制根數，
不能把資料尾端釘在某一天。`_decision_replay_rows` 的取樣窗又是
`window_start = max(first_idx, last_idx - quota + 1)`，**錨在資料尾端**。

所以 live 每天收盤新增一根 K 棒，同一條指令隔天就抽到**不同的 200 列**。

#### 已造成的實際後果

2026-09-01 那四次 run 的結論（逐列 transition matrix、`CONTINUATION` 只有 1 列、
完整 predicate 命中 0）**當天過後無法用指令重建**。
處置是**把逐列比較資料進版控**：
[`python/baselines/replay_cohort_2026-09-01.json`](../python/baselines/replay_cohort_2026-09-01.json)
（四份 report × 200 列的比較欄位），配合
[`sr-zone-scoring.md`](./sr-zone-scoring.md)「分佈影響：decision replay 實測（2026-09-01）」
的分佈表與 cohort 佐證表。**那份 JSON 就是最終證據**——結論可以從它重算，
但**不能**由獨立 reviewer 重跑 replay 得到。

**這不只是存檔問題**：沒有 as-of 上界，任何「同一 cohort 跑兩次」的驗證都必須
擠在同一個資料凍結窗口內（當日 09:00–15:00，避開 `pre_market` / `daily_close` /
池同步 / chip 同步），跨日就得整批重跑。

#### 可能做法（待評估）

* 加 `--as-of <date>`：`_load_db_sources()` 傳入上界，`fetch_candles()` 加 `ts <= :as_of`。
  **要注意 chip / model governance context 也是依 `[dataset_from, dataset_to]` 當下從 DB 撈的**
  （`_load_db_replay_chip_context` / `_load_db_replay_model_governance_context`），
  只釘 candles 不夠，那兩個也會隨 DB 內容變動。
* 或者接受不可重現，改為**強制輸出 cohort 身分指紋**並要求驗收報告一律附上它。

#### 關閉條件

同一條指令在不同日期執行，能產出 `(symbol, as_of)` 完全相同的 cohort（指紋一致）；
或明確決定走第二條路，並把「驗收報告必須附 cohort 指紋與逐列比較資料」寫進
[`development-workflow.md`](./development-workflow.md)。

---

### I-101：`rsi14` / `vol_ratio` 的 `DECIMAL(6,4)` 容不下合法值（indicator 與 signal 兩張表）

| 欄位 | 內容 |
|---|---|
| 狀態 | **已規劃／待實作**（計畫書見下，未經確認前不實作） |
| 嚴重度 | 中（單一標的的指標停止落地，只留 warn，且 Redis 與 DB 會不一致） |
| 分類 | Go / 指標 / DB schema |
| 發現日期 | 2026-09-01 |
| 來源 | live warn log：`indicator upsert failed symbol=2454 error=ERROR: numeric field overflow (SQLSTATE 22003)` |

#### 現象與成因

2026-09-01 13:15 live 出現：

```
{"level":"warn","caller":"indicator/engine.go:69","msg":"indicator upsert failed",
 "symbol":"2454","error":"ERROR: numeric field overflow (SQLSTATE 22003)"}
```

**已用當時的完整輸入重現**（2026-09-01，唯讀）。`Engine.Compute` 取 `lookback = 120` 根，
`CalcRSI` 用的是**整份輸入**（前 14 個 diff 算初始平均，其餘全部做 Wilder smoothing），
所以必須看完整視窗才能斷定，不能只看最後幾根：

```
2454 / 1m / 截至 13:15 的 120 根：唯一收盤價 = [4315.0]
119 個 diff：上漲 0、下跌 0、平盤 119
初始 avgGain=0.000000 avgLoss=0.000000 → 最終 avgGain=0.0 avgLoss=0.0 → RSI = 100
```

`CalcRSI`（`backend/internal/indicator/rsi.go:34`）在 `avgLoss == 0` 時回傳 **100**，
而 `indicator_snapshots.rsi14` 是 `DECIMAL(6,4)`——上限 **99.9999**，於是 `100.0000` 溢位。

⚠️ **兩件事要分開講**（第一版計畫書把它們混在一起，只拿「最後 19 根平盤」當證據，不足）：

1. **RSI = 100 會溢位**——這是型別問題，與怎麼走到 100 無關。
   `avgLoss == 0 且 avgGain > 0`（單邊連漲、完全沒有下跌）**也**會回 100，
   那條路徑**不是**下面的語意修正能解決的，只能靠放寬型別。
2. **2454 這一次確實是「整個視窗完全無波動」**——上面的重現是這一項的證據。

⚠️ **`diff == 0` 走的是 else 分支**（`lossSum -= 0`），所以「完全不動」與「單邊連漲」
在 `CalcRSI` 裡是**同一條路徑**，都得到 RSI=100。對前者而言這個語意是錯的：
沒有任何波動時 RSI 應為中性（50），而 100 表示「極度超買」。

#### 影響

* **2454 的 1m 指標從 11:24 之後就沒再寫進 DB**（最後一筆 `rsi14=98.7805`）。
* `Upsert` 失敗只 `log.Warn` 不中斷，而 `cacheToRedis` 排在它**後面**照樣執行——
  **Redis 有新值、DB 沒有**，這個不一致沒有任何其他訊號會提醒。
* **這個不一致是使用者看得到的**：`GET /api/v1/indicators/:symbol`
  （`api/handler/indicator.go:25`）走的是 `IndicatorRepo.GetLatest`——**只讀 DB，不讀 Redis**。
  所以前端看到的 2454 1m 指標會停在 11:24，而且是 200 OK，不是錯誤。
* RSI=100 會讓 `isRSIOverbought`（`signal/breakout.go:95`）擋掉 BUY 訊號，
  所以「完全沒成交波動的標的」目前會被當成極度超買。

#### 同一類的第二顆地雷：`vol_ratio`（**兩張表都有**）

`vol_ratio` 也是 `DECIMAL(6,4)`（上限 99.9999），但量比 `current / MA20` **沒有上界**。

⚠️ **它在兩張表裡各出現一次，只修一張等於沒修**：

| 表 | 欄位 | 寫入路徑 |
|---|---|---|
| `indicator_snapshots` | `vol_ratio DECIMAL(6,4)` | `indicator/engine.go` → `IndicatorRepo.Upsert` |
| `signals` | `vol_ratio DECIMAL(6,4)`（`003_create_signals.sql:9`） | `signal/breakout.go:37` → `SignalRepo.Insert`，**BREAKOUT / BREAKDOWN / VOLUME_SPIKE 三種訊號都會帶** |

2026-09-01 實測 live 現況：

| 欄位 | 型別上限 | 目前實際最大值（1m） |
|---|---|---|
| `rsi14` | 99.9999 | 99.5096 |
| `vol_ratio` | 99.9999 | **92.5000** |

**只差 8% 就會撞上同一個錯誤**，不是理論風險。而且量比越大越可能觸發爆量訊號——
**最該被記錄的那些列，正是最可能寫不進去的**。

#### 為什麼選 `DECIMAL(23,4)`（原訂 `DECIMAL(12,4)` → `DECIMAL(18,4)`，兩次 review 後定案）

⚠️ **第一版用「單分鐘約 10⁵ 張」推導上界，那個推導是壞的**——它混用了成交量單位。
`candles.volume` 的單位**依來源而不同**：

| 來源 | 單位 | 實際落在哪個 timeframe |
|---|---|---|
| Yahoo（`volume`） | **張** | `1m` |
| FinMind（`Trading_Volume`） | **股** | `1d` |

（來源對照見 [`yahoo-intraday-integration.md`](./yahoo-intraday-integration.md)「成交量：單位是張」
與 `backend/internal/market/model.go`。**ratio 本身會消掉單位**，因為分子分母同屬一條序列，
而 `todo.md` T-043 明訂日 K 補價不補量、成交量仍走 FinMind，所以同一個
`(symbol, timeframe)` 序列不會混用單位——**但拿 raw volume 去推 ratio 上界時不能混用**。）

**改用可證明的上界＋實測值**：

1. **`ratio ≤ current_volume`**。`MA20` 是整數除法（`volume.go:22`）且有 `ma > 0` 守門，
   所以分母**最小為 1**——這是不等式，不是估計。因此「成交量的上界」就是「ratio 的上界」。
2. **實測 live `candles` 的單根最大量**（2026-09-01，唯讀，含全部來源與單位）：

   | timeframe | 列數 | max(volume) | p99.9 |
   |---|---:|---:|---:|
   | `1d` | 568,949 | **4,204,257,454**（約 4.2×10⁹，股） | 3.56×10⁸ |
   | `1m` | 79,289 | 48,455（張） | 7,662 |

3. **所以 `DECIMAL(12,4)`（上限約 10⁸）不夠**——1d 的最大量 4.2×10⁹ 已經超過它兩個數量級。
   原本「多兩個數量級餘裕」的說法是錯的，方向還相反。

**所以型別要蓋住的是「非負 `int64` 的完整值域」，不是實測值加餘裕。**
`volume` 在 DB 是 `BIGINT`、在 Go 是 `int64`，上界 9.22×10¹⁸ 需要 **19 位整數位**。
`DECIMAL(23,4)` 提供 19 位整數 ＋ 4 位小數，**涵蓋 `ratio` 在型別上可能出現的全部值**——
於是「不會溢位」變成**建構上成立**，而不是「依實測估計應該夠」。
mysql 的 DECIMAL 精度上限是 65、postgres 是 1000，23 對兩者都遠低於上限；
兩張表各只有數千到兩萬列，儲存成本可忽略。

⚠️ **中間版本曾訂為 `DECIMAL(18,4)`（約 10¹⁴），那是「營運容量選擇」不是可證明的上界**
——它只比實測最大量多四個數量級，仍然**證明不了**不會溢位。既然這一整筆處理的就是
「型別容不下合法值」這一類缺陷，就不要再留一個同型的估計值。

**為什麼不用浮點**：這兩欄會參與比較與排序，維持 DECIMAL 與其他欄位一致。
**為什麼不加 cap**：夾值會讓「100 倍爆量」與「1000 倍爆量」變成同一個數字，
而那正是爆量訊號最需要分辨的資訊。

#### 為什麼測試抓不到

sqlite 那份 migration 的 `rsi14` / `vol_ratio` 是 **`REAL`（無精度限制）**，
而 `backend/scripts/test.sh` 跑的就是 sqlite——**這類溢位在單元測試裡永遠不會出現**。
這是 CLAUDE.md 記的「三份 migration 都要同步維護」在**型別語意**上的漏洞：
編號同步了，精度約束沒有等價物。

#### 修改目標

1. 讓 schema 容得下兩個欄位的合法值域。
2. 修正 `CalcRSI` 在「完全無波動」時的語意。

**不做的範圍**：不改 RSI 的計算公式與 Wilder smoothing、不改 `isRSIOverbought` 的門檻、
不改其他指標欄位的精度。另外兩項刻意排除，理由如下。

##### 排除一：`Upsert` / `Insert` 失敗後繼續執行的行為 → 另立 **I-102**

`indicator/engine.go:68` 與 `signal/engine.go:99` 都是**寫 DB 失敗只記 warn，
然後照樣寫 Redis／推 WebSocket 並回傳成功**。放寬型別只消掉**目前已知的兩種**溢位，
任何其他 DB 錯誤仍會造成同樣的 DB／Redis 不一致。

**那是另一類缺陷**（錯誤處理與快取一致性，且牽涉「DB 掛掉時該不該繼續推訊號」這個設計取捨），
不放在本筆一起改，**另立 [I-102](#i-102) 追蹤**。
⚠️ **所以本筆不宣稱解決「靜默失效」**——標題已於 2026-09-01 依此修正。

##### 排除二：不回填 11:24 之後已經漏掉的 indicator 列

⚠️ **第一版計畫書寫「下一輪計算會自然覆蓋」，那是錯的。**
`indicator_snapshots` 的唯一鍵是 `(symbol, timeframe, ts)`（`002_create_indicators.sql:23`），
下一次 `Compute` 只會寫**最新**的 ts，**中間漏掉的那些 ts 永遠不會被補回**。

**明示接受這個歷史缺口**，理由是它讀不到也用不到：`IndicatorRepo` 的介面只有
`Upsert` 與 `GetLatest`（`store/indicator_repo.go:9`），**沒有任何區間查詢**，
唯一的讀取端 `GET /api/v1/indicators/:symbol` 也只取最新一列。
所以缺口的實質影響是「修好之前 API 一直回 11:24 的舊值」，
**修好之後第一次成功 upsert 就恢復**，不需要回算。

#### 受影響檔案與資料 contract 變化

| 檔案 | 變更 |
|---|---|
| `migrations/postgres/075_widen_indicator_numeric.sql` | `indicator_snapshots`：`rsi14 → DECIMAL(7,4)`、`vol_ratio → DECIMAL(23,4)`；**`signals`：`vol_ratio → DECIMAL(23,4)`** |
| `migrations/mysql/075_widen_indicator_numeric.sql` | 對應的 `MODIFY COLUMN`，同樣兩張表 |
| `migrations/sqlite/075_widen_indicator_numeric.sql` | **no-op**（`REAL` 無精度概念），只為維持三份編號對齊，body 只有註解（既有慣例見 `sqlite/009_add_user_status.sql` 的 Down） |
| `backend/internal/indicator/rsi.go` | `avgLoss == 0` 時再分兩種：`avgGain == 0` → **50**；否則維持 **100** |
| `backend/internal/indicator/rsi_test.go` | 補「完全無波動 → 50」與「單邊連漲 → 100」兩條 |
| `backend/internal/database/adjuster_postgres_test.go` | 擴充既有的 `TestPostgresMigrationsRealValuesFitAllColumns`（見下方測試策略） |
| `backend/internal/database/adjuster_mysql_test.go` | 擴充既有的 `TestMySQLMigrationsRealValuesFitAllColumns` |
| `docs/database-schema.md` | `indicator_snapshots` 與 `signals` 的欄位型別更新（現在寫的是 `DECIMAL(6,4)` ＋「RSI（0～100）」，型別容不下自己宣稱的上界，是文件與實作矛盾） |
| `docs/indicator-spec.md` | RSI 的邊界定義補上「無波動 → 50」 |
| `docs/issue.md` I-054 | 實作後把「已覆蓋的 repo 寫入路徑」由 2 條更新為 4 條 |

**contract 變化**：`rsi14` 值域仍是 `[0, 100]`（型別終於容得下上界）；
兩張表的 `vol_ratio` 上限由 99.9999 放寬到涵蓋非負 `int64` 的完整值域。
**全部都是放寬，不影響既有讀取端。**

#### 主要風險與回滾

| 風險 | 處置 |
|---|---|
| **Down migration 會縮回原精度**，若屆時已有 `rsi14 = 100` 或任一張表的 `vol_ratio > 99.9999` 會失敗 | **預設中止，不自動刪任何列**——程序見下方「回滾程序」 |
| RSI 語意改變影響訊號 | 只影響「完全無波動」這一種輸入。該情境下原本 RSI=100 會擋掉 BUY，改為 50 後不再擋——這是**刻意的修正**，已於 2026-09-01 取得同意 |
| live 需要重建 image 才會套用 migration | migration 是 **embed 進 binary** 的，所以 image 的新舊決定 schema 的新舊（T-067 踩過）。live 的部署入口是 `/opt/stacks/scripts/stock_trading/deploy.sh`（`git pull origin init` → `down` → `up --build -d`），⛔ **不是** repo 的 compose——理由、窗口與套用後驗收見下方「live 執行程序」 |

#### live 執行程序

**兩張表都很小，鎖定時間不是主要風險**（2026-09-01 實測 live）：

| 表 | 列數 | 大小 |
|---|---:|---:|
| `indicator_snapshots` | 18,159 | 5,360 kB |
| `signals` | 5,265 | 1,480 kB |

即使 postgres 對 numeric typmod 變更走整表重寫，這個量級也是次秒級。**但仍照程序做**：

1. **`lock_timeout` 必須寫在 migration 裡，不能用外部 psql 設。**
   ⚠️ migration 是 **backend 啟動時由 goose 用它自己的連線執行**
   （`database/migrate.go` 的 `goose.UpContext`）——在另一個 psql session 下
   `SET lock_timeout` 對它**完全無效**。
   所以 postgres 的 075 在 Up 與 Down **各自的第一行**寫
   `SET LOCAL lock_timeout = '5s';`（goose 預設把單一 migration 包在交易裡，
   `SET LOCAL` 的作用域正好是那個交易，結束即還原，不影響 backend 後續連線）。
   ⚠️ **不要加 `-- +goose NO TRANSACTION`**，否則 `SET LOCAL` 失去依附的交易。
   `ALTER TABLE` 要 `ACCESS EXCLUSIVE`，卡住時寧可失敗中止，也不要排隊阻塞
   `indicator upsert` 與 `signal insert`。

2. **先在 dev 量耗時**：套用 075 並記錄；dev 資料量與 live 不同時以 live 列數推算，
   不要直接套用 dev 的秒數。

3. **挑窗口——「收盤後」不等於安全**。會寫這兩張表的排程：

   | 時間 | 排程 | 為什麼會寫到 |
   |---|---|---|
   | 08:50 | `pre_market` | 預熱日線指標 |
   | 09:00–13:55 每 5 分 | `intraday` | `signalEng.Evaluate(…, "1m")` |
   | **15:00** | **`daily_close`** | 逐檔 `signalEng.Evaluate(…, "1d")`，而 `Evaluate` 第一行就是 `indicator.Compute`（`signal/engine.go:59`）——**兩張表都寫** |
   | 16:00 | 池同步 ＋ 缺漏偵測 | 寫日 K，間接影響下一輪計算 |

   → 安全窗口是 **13:56–14:59**，或**確認 `daily_close` 已跑完之後到 16:00 之前**
   （查 `SELECT status, started_at FROM job_runs WHERE job_name='daily_close' ORDER BY id DESC LIMIT 1;`）。

4. **部署走 live 的權威腳本，⛔ 不要用 repo 的 compose。**

   ⚠️ **這是 2026-08-28 已經踩過一次的坑**（見
   [`development-workflow.md`](./development-workflow.md)「停 live 之前先確認它是怎麼起來的」）：
   live 的 `stock_trading` project **不是**由 repo 的 `docker-compose.yml` 部署的，
   而是 `/opt/stacks/scripts/stock_trading/`（`compose.yml` ＋ `deploy.sh`，
   `--project-directory` 指向 `/opt/stacks/deploy/...` 的另一份 clone）。
   **project 名稱相同**，所以在 repo 目錄下操作**會成功動到 live**——
   但拉回來時 `SR_ANALYSIS_ENABLED` / `EVALUATION_UNIVERSE_ENABLED` /
   `CANDLE_GAP_DETECTION_ENABLED` 會全部落回 `${VAR:-false}`、`FINMIND_API_KEY` 變空字串，
   **服務照樣起得來、什麼都不報錯**，只是生產排程被靜默關掉、抓取全部失敗。

   正確流程：

   1. 把變更**推上 `init` 分支**（`deploy.sh` 會 `git pull origin init`）。
   2. 執行 `/opt/stacks/scripts/stock_trading/deploy.sh`。它會 `git pull` →
      `docker compose … down` → `up --build -d`，**開關與金鑰由該腳本的 `export` 提供**。
   3. 因為它本身就是 `up --build`，**不需要（也不要）另外下 `build` 或 `up -d`**。
      ⚠️ 它會先 `down` 再起，**有停機**；配合下面的窗口安排。

5. **套用後驗收——分兩個時點，不要混在一起。**

   **A. 部署後立即可驗**（migration 在 backend 啟動時就跑完）：

   ```sql
   -- ① goose 版本應為 75。WHERE is_applied 不能省——goose_db_version 會留下
   --    已回捲的紀錄，少了這個條件版本判讀會失真（慣例見 scripts/verify-adjustment.sh:57）
   SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version WHERE is_applied;

   -- ② 三個欄位的精度真的變了
   SELECT table_name, column_name, numeric_precision, numeric_scale
   FROM information_schema.columns
   WHERE (table_name, column_name) IN
         (('indicator_snapshots','rsi14'), ('indicator_snapshots','vol_ratio'), ('signals','vol_ratio'));
   -- 期望：(7,4) / (23,4) / (23,4)
   ```

   **B. 要有新的 1m 寫入才驗得到**——1m 只在盤中寫（`*/5 9-13`），
   ⚠️ **部署後直接查會看到 11:24 的舊值，那不代表沒修好**。兩條路擇一：

   * **主動觸發（建議，當下就有結果）**：呼叫
     `POST /api/v1/indicators/2454/compute?timeframe=1m`（`api/server.go:104`，需帶 auth）。
     它同步跑一次 `Compute`，走的是同一條 `Upsert` 路徑。

     ⛔ **不要拿 API 的回應當證據。** `Compute` 在 `Upsert` 失敗時**只記 warn 就繼續**
     （`indicator/engine.go:68`），照樣回傳算好的 snapshot——**溢位時它一樣是 200**。
     這正是 [I-102](#i-102) 那個行為。**API 只負責觸發，判定要看 DB。**

     ```sql
     -- ③ 唯一的判定證據：2454 的 1m 最新 ts 已經前進（晚於 2026-09-01 11:24）
     SELECT ts AT TIME ZONE 'Asia/Taipei', rsi14 FROM indicator_snapshots
     WHERE symbol='2454' AND timeframe='1m' ORDER BY ts DESC LIMIT 1;
     ```

     **ts 前進 ＝ `Upsert` 成功 ＝ 型別問題解除**，這就夠了。

   * **被動等**：延到**下一個交易日**盤中再查 ③。

   ⚠️ **不要硬性要求 `rsi14 = 50`。** 那個值只有在**呼叫當下的最新 120 根仍然完全無波動**
   時才成立；隔一天或有任何成交波動，RSI 就會是別的值，而那**不代表沒修好**。
   要順帶驗語意時，先確認輸入條件：

   ```sql
   -- 最新 120 根的相異收盤價數；等於 1 才適用「應為 50」這個期望
   SELECT count(DISTINCT close) FROM (
     SELECT close FROM candles WHERE symbol='2454' AND timeframe='1m'
     ORDER BY ts DESC LIMIT 120) x;
   ```

   **RSI 的語意修正本來就該由 dev 的固定輸入單元測試證明**（見下方測試策略），
   不需要靠 live 的浮動資料。

   **④ 下一個交易日**再確認
   `docker logs stock_trading-backend-1 | grep 'indicator upsert failed'` 不再出現。
   ⚠️ `docker logs` 在這台機器上不耐久（容器會被重建，T-067 已記），
   所以**一律以 ③ 的 DB 證據為主、log 只是輔助**。

#### 回滾程序

⛔ **live 沒有執行 goose Down 的入口。** `RunMigrations` 只呼叫 `goose.UpContext`
（`database/migrate.go`），整個 repo 裡 `DownToContext` **只出現在測試**。

⛔ **也沒有「回舊 image tag」這個機制。** live 的 `compose.yml` 只有 `build:`、
沒有 `image:` tag，`deploy.sh` 每次都是 `up --build` 現地重建。

**而這一筆根本不需要回滾 schema**：075 是**純放寬**，舊 binary 對更寬的欄位完全相容。
所以退版的做法是**退程式碼、不退 schema**：

1. 在 `init` 分支上 revert 掉這次的 commit 並推上去。
2. 再跑一次 `/opt/stacks/scripts/stock_trading/deploy.sh`——它會 `git pull` 到 revert 後的
   狀態並重建 image。舊 binary 對 `DECIMAL(23,4)` 的欄位讀寫完全正常。
3. **schema 留著**。留著寬型別沒有任何害處，也不影響舊 binary。
   ⚠️ **不要為了「型別看起來太寬」去縮回去**——那才是製造風險的動作。

真的有非退 schema 不可的理由時（例如發現放寬本身造成問題），**預設是中止而不是繼續**：

1. 先查三個超界條件的筆數，**任一非零就停**：

   ```sql
   SELECT count(*) FROM indicator_snapshots WHERE rsi14 > 99.9999;
   SELECT count(*) FROM indicator_snapshots WHERE vol_ratio > 99.9999;
   SELECT count(*) FROM signals            WHERE vol_ratio > 99.9999;
   ```

2. 非零時**先匯出留底**（`\copy (SELECT …) TO '…' CSV HEADER`），並取得**明確的人工確認**
   要刪哪張表、可接受損失哪些列。兩張表性質不同：
   * `indicator_snapshots` 是衍生快取，但唯一鍵含 `ts`，**只有最新那個 ts 會被重算補回**，
     中間的歷史列不會回來。
   * ⛔ **`signals` 是事件紀錄，刪掉永久消失**，沒有任何地方能重算。
3. 縮型別本身只能**手動下 DDL**（backend 的啟動流程只跑 Up），這一步要先想清楚怎麼做、
   並在 `goose_db_version` 留下對應處置，否則下次啟動會與 embed 的 migration 狀態不一致。

#### 測試與驗證策略

⚠️ **第一版寫「postgres migration 腳本是唯一能證明型別夠寬的驗證」，那是錯的**——
`TestPostgresMigrationsUpAndDown` 只驗 migration 能 up/down，**不會檢查精度**；
而 dev 上的「flat → RSI 50」在**舊的** `DECIMAL(6,4)` 底下也會通過（50 塞得進去），
所以那一步也證明不了型別放寬。**必須有真的寫入邊界值的測試。**

**擴充既有的 `…MigrationsRealValuesFitAllColumns`**（postgres 與 mysql 各一支）——
那支測試的存在目的就是這一類「值塞不塞得下」的問題，目前涵蓋 `corporate_actions` /
`job_runs` / `evaluation_universe`，**沒有涵蓋 `indicator_snapshots` 與 `signals`**：

1. 走**真實 repo** `IndicatorRepo.Upsert`，`rsi14 = 100.0000`（RSI 的數學上界），
   `vol_ratio` **兩個值都要測**：
   * `4204257454.0000`——live 實測的最大單根量，等同 MA20=1 時的實際 ratio 上界；
   * `float64(math.MaxInt64)`——**非負 `int64` 上界在 Go 端的實際表示**。
     ⚠️ **不要寫成 `9223372036854775807`**：`VolRatio` 是 `float64`，
     `math.MaxInt64` 無法精確表示，轉型後實際是 **9223372036854775808**（2⁶³）。
     測試要用 `float64(math.MaxInt64)` 這個運算式本身，並斷言
     **round-trip 回讀的值等於它**——目標是「不溢位且不失真地走完 repo/driver」，
     **不是**「與 int64 上界逐位相同」，後者在 float64 上本來就做不到。
     （2⁶³ ≈ 9.22×10¹⁸ 有 19 位整數位，`DECIMAL(23,4)` 的 19 位整數剛好容得下。）

   ⚠️ **只測前者不算數**：那只證明「目前的資料塞得下」，而本筆的主張是
   「**型別上不可能溢位**」。後者才驗得到 repo 與 driver 有沒有能力把這個量級的數
   送進 DB 再讀回來——精度 metadata 只驗得到 DDL，驗不到傳遞路徑。
2. 走**真實 repo** `SignalRepo.Insert`，同樣兩個 `vol_ratio` 值各寫一次。
3. 讀 `information_schema.columns` 斷言三個欄位的 `numeric_precision` / `numeric_scale`
   是 `(7,4)` / `(23,4)` / `(23,4)`——這一條在**放寬前會紅**，是「migration 真的生效」的證據。

其餘：

4. `backend/scripts/test.sh ./internal/indicator/...`——RSI 的兩條新測試。
5. `scripts/test-postgres-migrations.sh` / `scripts/test-mysql-migrations.sh`——
   帶著上面擴充的斷言跑。
   ⚠️ **不要寫成「mysql 只涵蓋 DDL 層」**：`TestMySQLMigrationsRealValuesFitAllColumns`
   本來就會走真實 repo（目前是 `CorporateActionRepo.Upsert` 與
   `EvaluationUniverseRepo.Upsert` 兩條），本計畫再加 `IndicatorRepo.Upsert` 與
   `SignalRepo.Insert`。正確的說法是**仍未涵蓋完整的 CRUD round-trip**
   （只有寫入、沒有查詢與更新的回路），既有限制見 [I-054](#i-054)。
   **實作完成後要同步更新 I-054 的「已覆蓋的 repo 寫入路徑」清單**（會從 2 條變成 4 條）——
   那份清單是判斷 mysql 缺口大小的依據，不更新就會低估覆蓋。
6. **dev compose 驗收**：套用 migration 後造一組「完全無波動」序列跑 `Compute`，
   確認落地為 `rsi14 = 50` 且無 overflow。**這一步驗的是語意修正，不是型別**。
7. ⚠️ **sqlite 永遠測不到溢位**（`REAL` 無精度），所以第 1～3 步是唯一的防線。

#### 完成後歸檔

`docs/database-schema.md`（兩張表的欄位型別與值域，含 `DECIMAL(23,4)` 的選型依據與 ratio ≤ volume ≤ int64 的推導）、
`docs/indicator-spec.md`（RSI 邊界語意：無波動 → 50、單邊連漲 → 100）。
review 通過後本筆整筆移除；I-102 獨立追蹤，不隨本筆移除。

---

### I-102：寫 DB 失敗後照樣寫 Redis／推 WebSocket，DB 與快取會靜默不一致

| 欄位 | 內容 |
|---|---|
| 狀態 | 待決策（要修成哪一種行為需要先定） |
| 嚴重度 | 中（**只有 warn log，沒有錯誤傳播、也沒有可操作的告警**；使用者看到的資料會依讀取路徑而不同） |
| 分類 | Go / 錯誤處理 / 快取一致性 |
| 發現日期 | 2026-09-01 |
| 來源 | [I-101](#i-101) 的 review 分出——放寬型別只消掉已知的兩種溢位，這個行為本身沒被處理 |

#### 現象

兩個引擎都是「寫 DB 失敗只記 warn，然後照樣往下走」：

| 位置 | 失敗後仍然做的事 |
|---|---|
| `indicator/engine.go:68` | 寫 Redis（`cacheToRedis` 排在後面）、回傳成功的 snapshot、讓 signal engine 繼續用它 |
| `signal/engine.go:99` | `redis.LPush("signal:queue")`、`BroadcastFn` 推 WebSocket、記 `signal generated` |

**後果是同一份資料依讀取路徑而不同。** 它不是完全無聲——兩處都會記一行 `log.Warn`——
但**僅止於此**：錯誤不往上傳、呼叫端拿到的是成功、沒有失敗計數或告警可以觸發處置，
所以除非有人正好在翻 log，否則不會被發現：

* `GET /api/v1/indicators/:symbol`（`api/handler/indicator.go:25`）**只讀 DB**——
  會回舊值，而且是 200 OK。
* Redis 與 WebSocket 拿到的是新值。
* `signals` 表少一列，但 `signal:queue` 與前端推播都有那一筆——
  **訊號歷史與實際發出的訊號對不起來**。

I-101 的 2454 就是這個行為的一次實證：11:24 之後 API 一直回舊指標，而唯一的痕跡是那行 warn。

#### 待決策：要修成哪一種

1. **失敗即中止**：`Upsert` / `Insert` 的錯誤往上回傳，不寫 Redis、不推 WebSocket。
   語意最一致，但 **DB 短暫故障時會完全停掉即時推播**——要先確認這是想要的取捨。
2. **維持繼續，但要看得見**：保留現行流程，改成錯誤等級並加上可觀測性
   （失敗計數、連續失敗告警、回應標記資料為 stale）。侵入性小，但不一致仍然存在。
3. **分路徑決定**：indicator 走 (1)（它有 API 讀 DB，不一致直接可見），
   signal 走 (2)（推播的即時性比歷史完整性重要）。

**決定前不要動手**——這牽涉「DB 掛掉時系統該表現成什麼樣子」，不是單純的錯誤處理清理。

#### 完成條件

選定一種行為並實作，補上**失敗路徑**的測試（目前兩條路徑都沒有測到「寫 DB 失敗時會怎樣」），
並把決定寫進 [`architecture.md`](./architecture.md)。

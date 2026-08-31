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
- **下一個新編號從 `I-100` 起算。**（**I-099 於 2026-08-31 發出後同日作廢**——誤把 `deploy.sh` 的保守預設當成與 live 的衝突，實際上該檔是範本、所有開關一律預設 `false` 是既有慣例；**編號不回收**；I-098 於 2026-08-31 由 I-096 的 review 發現分出；I-081～I-083 於 2026-08-21 發出（**I-081 / I-082 於 2026-08-27 隨 `todo.md` T-055 收斂**），I-084～I-087 於 2026-08-24 發出，I-088～I-092 於 2026-08-25 發出（**I-091 於 2026-08-28 收斂**），I-093 / I-094 於 2026-08-26 發出（I-093 已於同日收斂，**I-094 於 2026-08-28 收斂**），I-095～I-097 於 2026-08-27 發出，其中 **I-097 於同日改列 `todo.md` T-064**——編號**不回收**。）
  **發出新編號時記得把這一行一起往前推**——上一次就是漏了這步，I-089 發出去之後
  這裡還寫著「從 I-089 起算」，差一點又重用一次（I-070 已經發生過）。
  檔案裡看得到的最大是 I-079（**下一個可用的是 I-100**——I-096 / I-098 已於 2026-08-31 收斂、
  I-099 已發出並作廢，編號都不回收），但被移除的條目
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
  I-081～I-084 / I-086～I-090 / I-093、I-096、I-098、已作廢的 I-099 與下一個可用的 I-100），
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
  `TestSummarizeIdentityStatsComputesAliasHitRate`。
  **缺的是 as-of 階梯／integration／live 母體的自然命中**，不是缺測試。

**成因是母體太小而不是實作有問題**：21 個交易日、4 檔，身分還來不及缺席到失格。
真正的解法是補分析排程（見 `todo.md` T-052），不是為這兩條路徑另外造假資料。
**這個判斷對收攤那半已被 2026-08-27 的 live 查證證實**（排程上線第 2 個交易日就觸發）；
alias 那半則仍未觸發，見下方關閉條件。

**關閉條件拆成兩段**：

* **缺席收攤**：分析排程（todo.md T-052）上線、production 母體累積到身分會自然失格後，
  確認 `zone_role_incarnations` 出現 `state='EXPIRED'`（**不是 `zone_instances`**，見上），
  且行為與單元測試一致。
  ✅ **已達成（2026-08-27 查證 live）**——詳見下方「收攤路徑的 production 證據」。
* **alias 備援**：不能假設 T-052 一定會讓 `eventIdentityStats.MatchedByAlias` 自然非零——
  T-048 實測中第一段既有鏈命中把多數情況吃掉了。排程上線後先觀察一段時間；若仍為 0，
  改由 targeted integration/live fixture 或 T-050 的可觀測性 metric 證明這條路徑，
  而不是把 T-052 卡死在不可控的自然觸發上。
  ⬜ **仍未達成（2026-08-27 查證 live）**：`sr_identity_stats` 自 2026-08-21 起 88 筆，
  `matched_by_alias` 合計 **0**（`matched_by_chain` 268 / `matched_by_current` 121 /
  `unmatched_keys` 0）。觀察期已過，**該改走 fixture 或 metric 那條路**。

#### 收攤路徑的 production 證據（2026-08-27 查 live）

T-052 於 2026-08-20 上線後，缺席收攤**自 2026-08-24 起實際發生**，且與單元測試的斷言逐項吻合：

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

**承接觸發點**：T-052 上線並累積 production 母體後，重新量測「alias 數撞頂」
（`alias_count >= 8`）的身分比例；若比例繼續上升，或 T-050 metric 顯示 alias 備援／撞頂
已進入日常路徑，再規劃上限策略。現在保留為已知限制，不單獨開修法。

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

### I-074：Lifecycle Engine 的 RR 解耦只有單元測試層級的證據，decision replay 驗證無法執行

| 欄位 | 內容 |
|---|---|
| 狀態 | 已知限制（缺 production 分析資料，暫不修） |
| 嚴重度 | 中（行為已改變且已上線，但驗證深度不足） |
| 分類 | Python / SR Zone / Lifecycle |
| 發現日期 | 2026-08-13（2026-08-18 確認缺口仍未關閉） |
| 來源 | Lifecycle Engine 抽離（原 `todo.md` T-044，已於 2026-08-18 收斂移出）P0 實作後的驗證盤點 |

`lifecycle_engine.py` 的抽離把 `rr_gate.qualified` 從 `CONTINUATION` 的判定條件移除（分層原則與
四套同名詞彙對照見 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「分層原則：lifecycle 不看 RR」）。
**這是一個已經上線的行為改變**，計畫要求用 decision replay 對真實資料比對
`final_entry_state` / `lifecycle_phase` / `market_bias` 的分佈變化來評估影響。

**那個驗證至今無法執行**，因為 replay 的母體太小：

```
stock_sr_zone_analyses：4 檔標的 / 20 次分析（2026-07-14 ~ 2026-08-13）
```

2026-08-18 重新確認，**與 2026-08-13 記錄時完全相同，一筆都沒增加**。
20 次分析做不出有統計意義的分佈比較。

這裡的 **4 檔 / 20 次** 是 production live DB 的自然母體。T-048 收斂時使用的
**4 檔 / 84 次** 是 isolated/as-of 階梯驗證 fixture，用來證明回歸與身分層寫入，
不能替代 production 分佈比較母體。

**評估標的池幫不上這個忙**：池只維護日 K，不產生 `stock_sr_zone_analyses`。
要有 replay 母體必須先對標的跑 SR 分析，而那是 watchlist 的職能
（分工見 [`architecture.md`](./architecture.md)「兩個標的清單」）。

**現有證據的等級要說清楚**：抽離後 428 支既有測試全數通過，
但那**不是**「沒有行為改變」的證據——它是「沒有任何既有測試涵蓋 RR 解耦那條路徑」的證據。
行為改變是**結構上可證明**的，並由兩支測試鎖住：

* `test_continuation_only_needs_price_evidence`——延續只看三項價格證據
* `test_widened_path_previously_testing_now_continuation`——真正變寬的那條路徑

**但前者無法防守「RR 被加回來」**：`resolve_lifecycle` 簽章裡沒有 `rr_gate`，
真要加回來會是新增參數，那支測試照樣綠燈。目前靠 `sr-zone-scoring.md` 的
「請不要加回去」與本筆記錄把守。

**關閉條件**：`stock_sr_zone_analyses` 累積到足以做分佈比較的量之後，
跑 `MODE=replay scripts/run-evaluation.sh` 比對上述三個欄位。
在那之前，這個行為改變的接受是**明示的決定**（2026-08-13 決定放寬、2026-08-18 確認維持），
不是「驗過沒問題」。

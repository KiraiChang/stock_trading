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
  檔案裡看得到的最大是 I-098（**下一個可用的是 I-100**——I-099 已發出並作廢，編號不回收），但被移除的條目
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
  I-081～I-084 / I-086～I-090 / I-093、已作廢的 I-099 與下一個可用的 I-100），那是預期的，不是殘留。

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

---

### I-096：`structure_state` 把單純碰觸命名成「收復候選」，並會回饋 Lifecycle／Decision

| 欄位 | 內容 |
|---|---|
| 狀態 | **review 不通過／待修復**（2026-08-31。B 的實作本身正確且已上版控，但**前提被推翻**：它靠 `reclaim_type == UNDERCUT_RECLAIM` 分流，而那個旗標本身會誤標，見 **I-098**。**卡在 I-098，修好才能收斂**） |
| 嚴重度 | **低**（**程式碼存在 touched-only fallback 的潛在風險，但 production 至今 0 筆實例**。原本標「已在 production 發生」是錯的：實際發生的是 `UNDERCUT_RECLAIM`，不是單純 touched） |
| 分類 | Python / SR Zone / 決策語意 / 前端呈現 |
| 發現日期 | 2026-08-26 |
| 來源 | 0050 `2026-08-26` 分析內容的逐項核實 |

#### 現象：畫面上有兩個不同粒度的「收復」輸出，但不是兩條獨立 SSOT

| 畫面上的東西 | 欄位 | 推導來源 |
|---|---|---|
| `SEMANTIC_BULLISH_RECOVERY` | `decision_derived_view.bias_reason_codes` | ← `semantic_pipeline.market_state` ← Lifecycle；⚠️ Lifecycle **直接吃同一個 `structure_state`**，另加 event states 與價格延續證據 |
| 「支撐收復候選」 | `position_action_condition.structure_state` → 前端 `structureStateText` | ← `_structure_state()` ← primary zone 的 `role` ＋價格互動證據（`closed_below` / `reclaim_type` / `touched`） |

前一版寫「兩者沒有任何共用的中間結論」是錯的（2026-08-27 再核實）：
`_decision_semantic_pipeline()` 把 `_structure_state()` 的輸出直接傳進 `resolve_lifecycle()`；
`resolve_event_signal()` 又把 `SUPPORT_RECLAIM_CANDIDATE` / `CONFIRMED` 直接視為
`CLOSE_RECLAIM`。正確資料流是：

```text
zone interaction → structure_state ───────────────→ position_action_condition / UI badge
                         └→ Lifecycle + event states + price evidence
                                      → semantic market_state → SEMANTIC_*
```

所以 semantic 是 structure 的**下游加料推導**，不是另一個互不相干的 SSOT。

#### 不是理論風險——live 已出現多種上下游組合

2026-08-21 起的 88 筆分析，兩個欄位的組合分佈：

| `market_state` | `structure_state` | 筆數 | |
|---|---|---|---|
| `BULLISH_RECOVERY` | `SUPPORT_RECLAIM_CANDIDATE` | 34 | ✅ 一致 |
| `BULLISH_RECOVERY` | **`NORMAL`** | **19** | 需逐筆分類：可能由 active event state 提供 reclaim 證據 |
| `NORMAL` | `NORMAL` | 14 | ✅ |
| `BULLISH_RECOVERY` | `SUPPORT_RECLAIM_CONFIRMED` | 12 | ✅ |
| `REVERSAL_CANDIDATE` | `NORMAL` | 8 | 可由 candidate event state 提供，不必與當根 structure 相同 |
| **`BULLISH_CONTINUATION`** | **`SUPPORT_RECLAIM_CANDIDATE`** | **1** | ⚠️ 名稱容易誤讀，但依現行 Lifecycle 契約不是互斥 |

最後一筆是 **`2454`，`analysis_id=127`，2026-08-26 22:00**：

* `market_state = BULLISH_CONTINUATION`、`lifecycle_phase = CONTINUATION`。
* `event_signal = CLOSE_RECLAIM`，reason codes 明列 `CLOSE_RECLAIM` 與
  `PRICE_UPSIDE_FOLLOW_THROUGH`。
* `structure_state = SUPPORT_RECLAIM_CANDIDATE`。

前一版把 `BULLISH_CONTINUATION` 解讀成「結構從未跌破」是錯的。現行 Lifecycle 的
`CONTINUATION` 是：先有 `CLOSE_RECLAIM`，再同時滿足 price follow-through、momentum
confirmed 與 clear zone breakout。它描述的是**收復後已有延續證據**，不是「從未跌破」。
因此這一組不是邏輯互斥案例。

#### 為什麼會這樣

真正要核實的是 `_structure_state()` 的最後一個分支：只要 primary SUPPORT 的
`touched=true`，即使 `reclaim_type` 不是 `UNDERCUT_RECLAIM`、也沒有先前跌破證據，
仍回傳 `SUPPORT_RECLAIM_CANDIDATE`。這不只讓 UI 顯示「支撐收復候選」：Lifecycle
會再把它當成 `CLOSE_RECLAIM`，因此可能改變 `semantic market_state`、Bias 與後續 gate。

問題應改寫成：**「touch／test」是否應與「reclaim」使用同一個 structure state，
以及沒有實際 reclaim event 時，structure state 是否有權單獨產生 `CLOSE_RECLAIM`。**

#### 這不只是顯示問題

`structure_state` 不是「回過頭」間接影響，而是 Lifecycle 的**明示輸入**；
`SUPPORT_RECLAIM_CANDIDATE` 會直接產生 `CLOSE_RECLAIM`。所以這不是純顯示或雙 SSOT
整理，若修改 `touched` 分支或 event-signal arbitration，屬於交易訊號／Decision 邏輯修改，
必須先做 replay 分佈與案例分類。

#### 處理方向（**擇一，未定案**）

**A. 只改 UI 名稱**：若 `touched` 的現行決策語意是刻意的，把
`SUPPORT_RECLAIM_CANDIDATE` 顯示成「支撐測試／互動候選」，避免宣稱已發生 reclaim。
這不改 Decision，但仍保留「touch 可產生 `CLOSE_RECLAIM`」的現況。

**B. 拆開狀態**：新增 `SUPPORT_TEST_CANDIDATE`（或等價名稱），只有真正的
`UNDERCUT_RECLAIM` 才回傳 `SUPPORT_RECLAIM_CANDIDATE`；Lifecycle 再分別仲裁
`SUPPORT_TEST` 與 `CLOSE_RECLAIM`。這會改 Decision，必須做 replay。

**C. 收緊 Lifecycle arbitration**：structure 的 touch state 可保留供 UI 使用，
但沒有 `INTRADAY_RECLAIM` 或其他實際 reclaim 證據時，不得只靠它產生 `CLOSE_RECLAIM`。
同樣屬於決策修改。

**先做診斷**：把上述 88 筆依「有無 active/candidate event、是否真的
UNDERCUT_RECLAIM、是否只有 touched」分類，再決定 A、B 或 C。原本以「兩個 SSOT
不一致」分類 20 筆的方向作廢。

#### 診斷結果（2026-08-27，88 筆全數分類）

判別欄位取自 `primary_zone.zone_interaction.price_action_evidence.reclaim_type`
（`zone_interaction` 頂層沒有這個鍵，要往 `price_action_evidence` 取）：

| `structure_state` | `reclaim_type` | 有 active `INTRADAY_RECLAIM` | 筆數 |
|---|---|---|---|
| `SUPPORT_RECLAIM_CANDIDATE` | **`UNDERCUT_RECLAIM`** | ✅ | **35** |
| `SUPPORT_RECLAIM_CONFIRMED` | `UNDERCUT_RECLAIM` | ✅ | 6 |
| `SUPPORT_RECLAIM_CONFIRMED` | `NONE` | ✅ | 4 |
| `SUPPORT_RECLAIM_CONFIRMED` | `NONE` | ❌ | **2** |
| `NORMAL` | — | — | 41 |

**結論一：本筆最擔心的「只有 `touched` 就被命名成收復候選」，production 至今 0 筆。**
全部 35 筆 `SUPPORT_RECLAIM_CANDIDATE` 的 `reclaim_type` 都是 `UNDERCUT_RECLAIM`——
走的是 `_structure_state` 上面那個分支，**最後那個 `touched` 兜底分支從來沒被命中過**。

**結論二：`structure_state` 單獨驅動 `CLOSE_RECLAIM` 的只有 2 筆**
（`5490`，`analysis_id=122` / `133`），且都是 `SUPPORT_RECLAIM_CONFIRMED`——
來自「前一根 `UNDERCUT_RECLAIM` 且本根未收破」那條分支，本身有跨根證據，
不是無中生有。其餘 **45** 筆非 `NORMAL` 的案例都另有 active `INTRADAY_RECLAIM` 佐證
（非 `NORMAL` 共 35+6+4+2 = **47** 筆，扣掉那 2 筆沒有 active reclaim 的），
**`resolve_event_signal` 的 `or` 兩邊同時成立**，拿掉 `structure_state` 那半也不改結果。

**結論三：`NORMAL` ＋ `touched=true` 不是矛盾。** 抽查（`analysis_id` 49 / 52 / 55 / 57）
四筆的 `primary_zone.role` 都是 `RESISTANCE`——`_structure_state` 對非 SUPPORT 的
primary zone 直接回 `NORMAL`，與 `touched` 無關。

#### 診斷後的取捨修正

* **B / C 的急迫性下降**：它們要解的「touch 被當成 reclaim 灌進 Decision」
  **目前沒有任何實際案例**，而兩者都要動決策並跑 replay。
* ⛔ **A（只改 UI 名稱）經診斷後不建議直接採用**（2026-08-27 review 定調，
  取代先前「A 的價值上升且風險最低」那個結論——兩者不該並列）。

  A 的前提是「名字宣稱得比實際強」，但診斷顯示**恰好相反**：現行 35 筆
  `SUPPORT_RECLAIM_CANDIDATE` **全都是真正的 `UNDERCUT_RECLAIM`**，
  改名成「支撐測試／互動候選」會讓**現行輸出的準確度下降**。

  **根本原因是一個名字要同時描述兩種強度不同的事**——真正的 undercut-reclaim，
  與（尚未發生但程式碼允許的）touched-only。**單一較弱的名稱兩者都描述不準**：
  對前者太弱，對後者才剛好。所以「改名」解不了這個問題，只會換一個方向的失準。

* ⚠️ **兜底分支仍在程式碼裡，但「保留現況」與「測試禁止 candidate」互相矛盾**
  （2026-08-27 review 修正）。原本兩句話並列是錯的——現行 fallback **就是**會回傳
  candidate。必須二選一：

  | | 做法 | 測試 |
  |---|---|---|
  | **維持現況** | 不改 fallback | 測試**記錄**「touched-only 目前會回傳 `SUPPORT_RECLAIM_CANDIDATE`」，把現行契約釘住，日後有人改動時會被測試擋下並被迫做決定 |
  | **收緊契約** | **先改** fallback 回傳 `SUPPORT_TEST_CANDIDATE`（或不回傳） | **改完之後**再加「touched-only 不得回傳 candidate」的 regression test |

  在 0 筆實例的前提下，「收緊契約」是**預防性重構**而非修 bug，
  且它會改 Decision（`resolve_event_signal` 吃 `structure_state`）而需要 replay——
  取捨要用這個框架談，不要當成修 bug 排程。

#### 真正待決策的只有一件事

診斷之後**排除 A**，剩下三條路（**B 與 C 是不同的方案，不可合併**——
2026-08-27 review 修正：前一版把兩者併成「拆分語意（B／C）」是錯的）：

| | 改哪一層 | 做法 | 代價 |
|---|---|---|---|
| **維持現況** | 不改 | 承認「touched-only 也會回傳 `SUPPORT_RECLAIM_CANDIDATE`」是現行契約，用測試釘住 | 名稱對 touched-only 過強的風險**繼續存在**，只是無意的改動會被測試擋下 |
| **B：拆狀態** | **`_structure_state`** | 新增 `SUPPORT_TEST_CANDIDATE`，只有真正的 `UNDERCUT_RECLAIM` 才回傳 `SUPPORT_RECLAIM_CANDIDATE`；Lifecycle 再分別仲裁 `SUPPORT_TEST` 與 `CLOSE_RECLAIM` | 改 Decision，需 replay。**UI 與資料層的狀態集合都變**，前端對照表要一起改 |
| **C：收緊 arbitration** | **`resolve_event_signal`** | **`structure_state` 完全不動**，UI 照樣顯示 touch state；但沒有 `INTRADAY_RECLAIM` 或其他實際 reclaim 證據時，**不得只靠 `structure_state` 產生 `CLOSE_RECLAIM`** | 改 Decision，需 replay。狀態集合不變，**改動面比 B 小** |

**B 與 C 的分野**：B 改「怎麼命名這件事」，C 改「這件事能不能單獨驅動決策」。
兩者可以獨立採用，也可以都做——**C 不是 B 的一部分**。

**診斷對 C 的影響範圍已經量出來了**：`structure_state` 單獨驅動 `CLOSE_RECLAIM` 的
只有 **2 筆**（`5490`，`analysis_id=122` / `133`），且都是 `SUPPORT_RECLAIM_CONFIRMED`
（來自前一根 `UNDERCUT_RECLAIM`）。**C 若把 CONFIRMED 也一併排除，就會改到這 2 筆**；
若只排除 `CANDIDATE`，則**目前 0 筆會被改到**——這個界線要在計畫書裡定死。

**三條路都不含「只改名」**——名稱的問題來自「一個名字要涵蓋兩種強度」，
那要靠 B 拆分狀態解決，換一個字只會把失準的方向調換。

#### 實作計畫（B）——2026-08-28 定案，**待使用者確認後才實作**

依 CLAUDE.md，本筆同時觸及**交易訊號／決策邏輯**與**前端 contract**，屬大規模／高影響異動。

##### 1. 目標與不做的範圍

**目標**：把 `_structure_state` 最後那個 touched-only 兜底分支的回傳值，從
`SUPPORT_RECLAIM_CANDIDATE` 拆成新的 `SUPPORT_TEST_CANDIDATE`；只有真正的
`reclaim_type == "UNDERCUT_RECLAIM"` 才保留 `SUPPORT_RECLAIM_CANDIDATE`。
Lifecycle 端把 `SUPPORT_TEST_CANDIDATE` 仲裁成 `SUPPORT_TEST`，**不得**產生 `CLOSE_RECLAIM`。

**明確不做**：

* **不做 C**（收緊 `resolve_event_signal`，讓 `SUPPORT_RECLAIM_CONFIRMED` 不能單獨驅動
  `CLOSE_RECLAIM`）。C 是獨立方案不是 B 的一部分；`5490` 的 `analysis_id=122` / `133`
  那 2 筆**維持現行行為不變**。
* 不動 `BREAKDOWN` / `SUPPORT_RECLAIM_INVALIDATED` / `SUPPORT_RECLAIM_CONFIRMED`
  三個分支——診斷顯示它們各自都有 EXPIRED、收破或跨根 `UNDERCUT_RECLAIM` 證據。
* 不動事件層任何型別或 `decision_visible` 旗標（角色翻轉的事件層缺口已決議維持現狀；
  現況見 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「事件層不涵蓋『角色翻轉當下』的突破」，
  原記於 I-095，已收斂）。
* 不順手整理 `structureStateText` 裡已經沒人產生的舊鍵（`RECOVERY_CANDIDATE` /
  `RECOVERY` / `RECOVERY_INVALIDATED`）——那是 [`todo.md`](./todo.md) T-064 的範圍。

##### 2. 受影響檔案與資料流

```text
zone interaction ─→ _structure_state ─┬─→ position_action_condition / market_regime / UI badge
                                      └─→ resolve_event_signal ─→ resolve_lifecycle
                                              → semantic market_state → SEMANTIC_* / Bias / gate
```

| 檔案 | 位置 | 要改什麼 |
|---|---|---|
| `decision_engine.py` | `_structure_state`（約 `:2467`，最後的 `touched` 分支 `:2490-2491`） | 回傳 `SUPPORT_TEST_CANDIDATE` |
| `decision_engine.py` | `:218` reason code 分支 | 新增 `SUPPORT_TEST_AWAIT_RECLAIM`，**不可**讓它落到 `else` 的 `SUPPORT_DEFENSE` |
| `decision_engine.py` | `:307` `structure_label` 中文表 | 新增「支撐測試候選」 |
| `decision_engine.py` | `:685` entry state | **新狀態必須與 `SUPPORT_RECLAIM_CANDIDATE` 同待遇**（見風險 R1） |
| `lifecycle_engine.py` | `:106-111` `resolve_event_signal` | 新狀態走 `SUPPORT_TEST`，不進 `CLOSE_RECLAIM` |
| `frontend/src/lib/api/srZones.ts` | `:326` `SRStructureState` | 封閉 union，**必須加值否則 TS build 失敗** |
| `frontend/src/routes/SRZones.svelte` | `:647` `structureStateText` | 新增中文標籤 |

**Go 端無 contract 變化**：`grep structure_state --include=*.go` 在 `backend/internal`
只命中 `portfolio/analyzer_test.go:163` 的 JSON fixture，**沒有任何依名字分支的程式碼**，
`structure_state` 對 Go 是純 passthrough。

**`recovery_state` 會自動跟著變**：`decision_engine.py:327` 是
`"RECOVERY" if ... == "SUPPORT_RECLAIM_CONFIRMED" else structure_state`，新狀態會**原樣流出**到
`market_regime.recovery_state`，所以前端 union 兩處都要涵蓋。

##### 3. 仲裁順序的變化（唯一需要拍板的設計取捨）

`resolve_event_signal` 現行順序：
`active_bearish` → `CLOSE_RECLAIM` → `REVERSAL_CANDIDATE` → `PENDING_ZONE_VALIDATION`
→ `EXTREME_VOLUME` → `NO_EVENT`。

**新分支放在 `REVERSAL_CANDIDATE` 之後、`PENDING_ZONE_VALIDATION` 之前**，回
`EVENT_SIGNAL_SUPPORT_TEST` + reason code `STRUCTURE_SUPPORT_TOUCH`。

理由：擺在這個位置，新狀態**只在原本會落到 `EXTREME_VOLUME` / `NO_EVENT` 的情況下**
才改變答案；`REVERSAL_CANDIDATE`（有 candidate event 佐證）優先序不變，
`PENDING_ZONE_VALIDATION` 與它同樣回 `SUPPORT_TEST`、只差 reason code，順序不影響 signal。
這是最小擾動的插入點。

##### 4. 主要風險與回滾

* **R1（最高）：拆分不可以變成放寬。** 現行 touched-only 走
  `SUPPORT_RECLAIM_CANDIDATE`，在 `decision_engine.py:685` 會被壓成
  `PROBE_ENTRY` / `WAIT_CONFIRMATION`（保守）。若新狀態沒被加進那個條件，
  touched-only 反而會落到 `SMALL_ENTRY` / `Buy` 路徑——**比現況更寬鬆**，
  與本筆「名稱對 touched-only 過強」的動機完全相反。這是 B 最容易踩的坑。
* **R2：`_structure_state` 有隱含守門。** `SUPPORT` ＋ `EXPIRED` → `BREAKDOWN` →
  `_decision_action` 的 `structure_broken` 提前 `return "AVOID"`，是
  `test_expired_primary_zone_never_upgrades_to_buy` 釘住的路徑（I-082）。
  **動這個函式前先讀那兩條測試**，見 [`sr-zone-scoring.md`](./sr-zone-scoring.md)
  「Decision Action 判定順序」第 4 步。
* **R3：中文標籤再次雙寫。** 新標籤會在 `decision_engine.py:307` 與
  `SRZones.svelte:647` 各定義一次，讓 T-064 的問題多一筆。實作時在兩處都留
  `見 todo.md T-064` 註記，不在本筆順手做 SSOT。
* **回滾**：單一 commit 可整包 revert。相容性上新狀態**只增不改**既有值，
  Go 是 passthrough，舊前端遇到未知值會走 `?? 原字串` 顯示英文而不會壞。

##### 5. 測試與驗證策略

**單元／端到端**（`python/scripts/test.sh`）：

1. touched-only fixture → `SUPPORT_TEST_CANDIDATE`（新行為）。
2. `UNDERCUT_RECLAIM` fixture → 仍 `SUPPORT_RECLAIM_CANDIDATE`（regression，確保沒改到那 35 筆的路徑）。
3. `resolve_event_signal("SUPPORT_TEST_CANDIDATE")` → `SUPPORT_TEST`，**且不是** `CLOSE_RECLAIM`。
4. **R1 專屬**：touched-only ＋ `action="BuySmall"` → 仍 `PROBE_ENTRY`（防放寬）。
5. `test_expired_primary_zone_never_upgrades_to_buy` 與其對照組必須續存並通過。

**影響面驗證——純函式重放，不需要 model bundle**：

`_structure_state` 是純函式，輸入全在既有分析的
`primary_zone.zone_interaction.price_action_evidence` 裡。拿 2026-08-21 起那 88 筆
分析的 payload 對新舊兩版各跑一次，**預期 0 筆改變**——診斷已證 35 筆
`SUPPORT_RECLAIM_CANDIDATE` 全是 `UNDERCUT_RECLAIM`，touched-only 兜底 0 筆命中。
**若差異非 0，代表實作改到了 `UNDERCUT_RECLAIM` 分支，直接視為失敗。**

⚠️ **完整 decision replay 跑不了，前置與 [`todo.md`](./todo.md) T-066 同源**
（dev 沒有 model bundle，`model_available: false`）。本筆採上述純函式重放作為影響面證據，
並在歸檔時**明寫「未經 decision replay 驗證」**；等 T-066 前置解除後可補跑。
**這個取捨要在動工前確認**——不接受的話，本筆要排在 T-066 之後。

##### 6. 完成後的歸檔位置

* [`sr-zone-scoring.md`](./sr-zone-scoring.md) `market_regime` 欄位說明（約 `:727-729`）——
  補上 `SUPPORT_TEST_CANDIDATE` 的語意，以及它與 `SUPPORT_RECLAIM_CANDIDATE` 的分野。
* 同檔「Decision Action 判定順序」第 4 步——順手修掉那裡對
  `decision_engine.py:2242-2243` 的**過期行號**（實際在 `:2467` 起）。
* 前端狀態集合的變更記在同一節，不另開文件。

#### 實作結果（2026-08-28）

依計畫書的 B 實作完畢，六個檔案：

| 檔案 | 改動 |
|---|---|
| `decision_engine.py` | `_structure_state` 兜底分支改回 `SUPPORT_TEST_CANDIDATE`；新增 reason code `SUPPORT_TEST_AWAIT_RECLAIM`；中文標籤「支撐測試候選」；**`_entry_action_state` 納入保守分支（R1）** |
| `lifecycle_engine.py` | `resolve_event_signal` 新增分支回 `SUPPORT_TEST` + `STRUCTURE_SUPPORT_TOUCH`，位置在 `REVERSAL_CANDIDATE` 之後、`PENDING_ZONE_VALIDATION` 之前 |
| `srZones.ts` / `SRZones.svelte` | union 加值、中文標籤，並補上與 Python 雙寫的 T-064 交叉註記 |
| `test_decision_engine.py` / `test_lifecycle_engine.py` | 8 支新測試（含 R1 防放寬與兩支 `UNDERCUT_RECLAIM` 回歸防線） |

Go 端一行未改——已確認 `structure_state` 對 Go 是純 passthrough，無依名字分支的程式碼。

##### 驗證結果

| 項目 | 結果 |
|---|---|
| `python/scripts/test.sh backtest/modular/sr_scoring/tests` | **444 passed, 1 skipped** |
| `frontend/scripts/test.sh`（svelte-check → vitest → build） | **147 passed**，型別與 build 全過 |
| `backend/scripts/test.sh` | 全過（dist 已重新 build 並納入版控） |

##### 影響面驗證：純函式重放（**不需要 model bundle**）

依計畫書 §5，對 live `stock_sr_zone_analyses` 的 **144 筆**分析做唯讀取樣，
拿 `primary_zone.zone_interaction` 餵給新舊兩版 `_structure_state` 各跑一次。
⚠️ `previous_interaction` 沒有被持久化（分析當下才用前一根 K 棒現算），重放一律傳 `None`，
所以**絕對值**不等於 `stored_state`；但兩版拿到的輸入完全相同，**版本間的 diff 有效**。

| 母體 | 筆數 | 期間 | 差異 |
|---|---|---|---|
| 帶 `price_action_evidence`（現行 schema） | 138 | 2026-07-16 ~ 08-28 | **0 筆** |
| 沒有該鍵的舊 payload | 6 | 2026-07-14 ~ 07-15 | **3 筆** |

現行 schema 那 138 筆裡，48 筆 `SUPPORT_RECLAIM_CANDIDATE` 的 `reclaim_type`
**全部**是 `UNDERCUT_RECLAIM`——與 2026-08-27 的診斷一致，拆分沒有誤傷它們。

##### ⚠️ 診斷的「production 至今 0 筆」是**視窗內**成立，不是全期

那句話是拿 2026-08-21 起的 88 筆算的。把母體拉到全部 144 筆之後，
**兜底分支在 production 歷史上被命中過 3 次**（`id=22` `0050` 07-14、
`id=25` `0050` 07-15、`id=27` `2330` 07-15），全部落在 2026-07-16 之前。

成因不是價格行為，是 **payload schema**：那批分析的 `zone_interaction`
**根本沒有 `price_action_evidence` 這個鍵**（欄位當時還不存在），於是
`evidence.get("reclaim_type")` 取到 `None`，一路掉到 touched 兜底。

**這不是回歸，是修好的證據**：那 3 筆在舊版被叫做「支撐收復候選」時
**手上一點收復證據都沒有**——正是 I-096 描述的失準。新版改叫「支撐測試候選」更準確。

**對未來的影響仍是 0**：現行 schema 一律帶 `price_action_evidence`，
兜底分支要再被命中得先有 payload 缺鍵。但它**確實可達**，不是純理論分支。

##### 尚未做、與 T-066 同源的缺口

完整 decision replay（`by_rr_gate` / `by_entry_executability` 分佈）**沒有跑**，
前置與 [`todo.md`](./todo.md) T-066 相同：dev 沒有 model bundle，`model_available: false`。
本筆以上述純函式重放作為影響面證據，**此變更未經 decision replay 驗證**——
這句話已一併寫進 [`sr-zone-scoring.md`](./sr-zone-scoring.md)。T-066 前置解除後可補跑。

##### 現況說明歸檔位置

[`sr-zone-scoring.md`](./sr-zone-scoring.md) `market_regime` 欄位說明的
`structure_state` 五值對照表（含 R1 那條「拆分不等於放寬」的警告）。
同時修掉了該檔對 `decision_engine.py:2242-2243` 的過期行號。

#### review 發現（2026-08-31）——**不通過，卡在 I-098**

實作、測試與歸檔都通過 review，**但拆分的分流依據本身是壞的**。

B 用 `reclaim_type == "UNDERCUT_RECLAIM"` 當「真收復 vs 只是碰到」的判準。
review 指出（並已實測重現）：對 SUPPORT 而言那個旗標**根本沒有在判斷有沒有跌破帶底**，
詳見 **I-098**。結果是本筆想擋的那件事只擋掉了一部分：

| 價格行為 | 拆分後的 `structure_state` | 對不對 |
|---|---|---|
| 跌破帶底後收回帶頂上方（真 undercut-reclaim） | `SUPPORT_RECLAIM_CANDIDATE` | ✅ |
| **從帶內往上穿出、從未跌破帶底** | **`SUPPORT_RECLAIM_CANDIDATE`** | ❌ **仍被叫成收復，且仍驅動 `CLOSE_RECLAIM`** |
| 碰到帶子、收在帶內 | `SUPPORT_TEST_CANDIDATE` | ✅（這一類是本次真正修好的） |

也就是說**本筆實際達成的分野是「收在帶內 vs 收在帶上」，不是「碰到 vs 收復」**。
名稱對第二列仍然過強——正是 I-096 一開始要解的問題，只是換了一個入口。

⚠️ **2026-08-28 那次純函式重放看不出這件事**，因為它比對的是新舊兩版
`_structure_state`，而兩版都吃同一個被誤標的 `reclaim_type`。
**重放的「138 筆 0 差異」仍然成立，但它證明的是「拆分沒有誤傷既有分類」，
不是「分類本身正確」。** 這個界線之前沒有寫清楚。

**處置**：本筆不回滾（拆分方向正確、測試有價值、第三列確實修好了）。
先修 I-098 讓 `reclaim_type` 誠實，再回來重跑重放並確認第二列落到
`SUPPORT_TEST_CANDIDATE`，本筆才能收斂。

**既有測試 `test_zone_interaction_uses_intraday_high_low_close_not_only_current_price`
（`test_decision_engine.py:1276`）目前把錯誤行為釘住了**，修 I-098 時要一併處理。

**相關**：[`sr-zone-scoring.md`](./sr-zone-scoring.md)「RR 語意分層」（原記於 `todo.md`
T-055，已收斂）——同一類問題的另一個面向：決策語意的多個數字／狀態並列而未分層。

---

### I-098：`reclaim_type` 的 undercut 判定對 SUPPORT 恆真，`penetration_pct > 0` 守衛沒有作用

| 欄位 | 內容 |
|---|---|
| 狀態 | **待修復**（2026-08-31 由 I-096 的 review 發現分出，已實測重現） |
| 嚴重度 | **中**（誤導且會驅動決策：把「從未跌破支撐」標成 `UNDERCUT_RECLAIM`，一路產生 `CLOSE_RECLAIM`。不影響 zone 本身的計算，但影響 Lifecycle 與 Bias 的語意） |
| 分類 | Python / SR Zone / 事件證據 / 決策語意 |
| 發現日期 | 2026-08-31 |
| 來源 | I-096 的 review。**這不是 I-096 改壞的**——缺陷早於 I-096，但 I-096 的分流正好建立在它之上，所以被暴露出來 |
| 阻擋 | **I-096 收斂**（B 的分流依據就是這個旗標） |

#### 成因：`penetration_pct` 混用兩側，而收在帶上必然讓它 > 0

`zone_interaction`（`event_engine.py:183-187`）的 `penetration_pct` **同時採計兩側**：

```python
penetration_pct = 0.0
if low < z.price_low:                 # 跌破帶底
    penetration_pct = max(penetration_pct, (z.price_low - low) / z.price_low)
if high > z.price_high:               # 突破帶頂
    penetration_pct = max(penetration_pct, (high - z.price_high) / z.price_high)
```

而 `:209` 判 undercut 時只檢查它是不是 > 0：

```python
if z.role == ZoneType.SUPPORT.value and touched and closed_above and penetration_pct > 0:
    reclaim_type = "UNDERCUT_RECLAIM"
```

**關鍵在於這個守衛恆真**：`closed_above` 的定義是 `close > price_high`，
而 K 棒的 `high >= close`，所以 `closed_above` ⇒ `high > price_high` ⇒
`penetration_pct > 0`。**`penetration_pct > 0` 對 SUPPORT 完全不做任何事**，
判定實際上退化成：

```text
UNDERCUT_RECLAIM  ⟺  touched ∧ closed_above
```

——與「有沒有跌破帶底」**無關**。名字說的是 undercut，實際判的是 close-above。

#### 重現（2026-08-31 實測，support zone `[98.0, 100.0]`）

| 情境 | high | low | close | `low < 98`？ | `penetration_pct` | `reclaim_type` | `structure_state` | `event_signal` |
|---|---|---|---|---|---|---|---|---|
| 真 undercut | 100.8 | **97.0** | 100.5 | ✅ | 0.0102 | `UNDERCUT_RECLAIM` | `SUPPORT_RECLAIM_CANDIDATE` | `CLOSE_RECLAIM` |
| **⚠️ 無 undercut** | 101.5 | **99.0** | 101.0 | ❌ | 0.0150 | **`UNDERCUT_RECLAIM`** | **`SUPPORT_RECLAIM_CANDIDATE`** | **`CLOSE_RECLAIM`** |
| 對照（收在帶內） | 100.0 | 99.0 | 99.5 | ❌ | 0.0000 | `NONE` | `SUPPORT_TEST_CANDIDATE` | `SUPPORT_TEST` |

第二列的 `penetration_pct` 比第一列**還大**，但它量的是往上穿出帶頂的幅度，
不是往下跌破的深度。第三列是對照組：真的沒有任何穿越時才落到 `NONE`。

#### live 母體的佐證

144 筆分析裡 90 筆 primary 是 SUPPORT，`(reclaim_type, closed_above)` 只有兩種組合：

| 組合 | 筆數 | 說明 |
|---|---|---|
| `(UNDERCUT_RECLAIM, True)` | 48 | 全部是 `touched ∧ closed_above` |
| `(NONE, True)` | 42 | `touched=False`（價格整根都在帶子上方，沒碰到） |

**沒有任何一筆 `touched ∧ closed_above ∧ NONE`**——與「守衛恆真」的推論完全一致。
換句話說 live 那 48 筆 `UNDERCUT_RECLAIM` **無法分辨**哪些真的跌破過帶底。

#### 修法方向（待計畫書）

方向清楚但**會改決策語意**（`reclaim_type` 餵給 `_structure_state` →
`resolve_event_signal` → Lifecycle → Bias），屬大規模／高影響異動，實作前要先寫計畫書：

* undercut 深度**只能用 `low < price_low`**；overthrow 深度**只能用 `high > price_high`**。
  兩側分開存（例如 `undercut_ratio` / `overthrow_ratio`），不要再共用一個
  `penetration_pct`——共用正是這個缺陷的來源。
* `penetration_pct` 這個欄位有別的讀者，**不能直接改它的語意**，要先盤點。
* ⚠️ **既有測試 `test_zone_interaction_uses_intraday_high_low_close_not_only_current_price`
  （`test_decision_engine.py:1276`）目前把錯誤行為釘住了**：它用
  zone `[98,100]`、`low=99`（從未跌破）、`high=101.5`、`close=101`，
  然後斷言 `structure_state == "SUPPORT_RECLAIM_CANDIDATE"`。修的時候要一起改，
  並補「low 未跌破、但 close 在帶頂上方」的反例。
* 修完要回頭重跑 I-096 的純函式重放，確認那一類落到 `SUPPORT_TEST_CANDIDATE`。

**相關**：I-096（拆分 `SUPPORT_TEST_CANDIDATE`，卡在本筆）。

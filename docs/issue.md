# ISSUE：遇到的問題與已知限制

記錄實際發生過的 bug、矛盾結果、文件/程式碼不一致，以及設計上刻意接受的
限制。跟「想做的優化」無關的項目放這裡；未來想做的功能擴充記錄在
[todo.md](./todo.md)。

## 使用說明

- **狀態**：`待修復` / `修復中` / `待執行` / `已修復` / `已實作／待 review` / `待決策` /
  `已知限制（不計畫修復）`
  - `已修復` 與 `已實作／待 review` 的差別是**有沒有經過 review**：改完先標後者並保留
    修復方式與計畫書，review 通過才收斂移除（見下方「移除條目前要先反轉依賴」）。
  - `待決策`用於**還不知道要不要修**的項目——需要先取得外部事實（上游狀態、實測數據）
    才能決定處置方向。它與`待修復`的差別是後者已經確定要修、只是還沒動工。
  - `待執行`（沿用 [`todo.md`](./todo.md) 的同名定義）用於**處置已定、只剩明確動作沒做**
    的項目：判準與步驟都寫完了，剩下的是照做。它與`待修復`的差別是後者的內容仍在描述
    一個要修的行為，前者的內容已經是一份可以直接執行的步驟。
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
- **下一個新編號從 `I-104` 起算。**（**I-103 於 2026-09-02 發出**——由 I-102 計畫書 review 分出，Yahoo 批次盤中路徑給不出逐檔寫入失敗；I-101 / I-102 於 2026-09-01 發出——前者來自 live 的 indicator upsert 溢位、**已於同日修復並收斂**（未完成的 live 部署由 `todo.md` T-069 承接，**該筆已於 2026-09-02 部署驗收完成並收斂**），後者由它的 review 分出；I-100 於 2026-09-01 發出，由 `todo.md` T-068 同日改列——**T-068 編號不回收**；**I-099 於 2026-08-31 發出後同日作廢**——誤把 `deploy.sh` 的保守預設當成與 live 的衝突，實際上該檔是範本、所有開關一律預設 `false` 是既有慣例；**編號不回收**；I-098 於 2026-08-31 由 I-096 的 review 發現分出；I-081～I-083 於 2026-08-21 發出（**I-081 / I-082 於 2026-08-27 隨 `todo.md` T-055 收斂**），I-084～I-087 於 2026-08-24 發出，I-088～I-092 於 2026-08-25 發出（**I-091 於 2026-08-28 收斂**），I-093 / I-094 於 2026-08-26 發出（I-093 已於同日收斂，**I-094 於 2026-08-28 收斂**），I-095～I-097 於 2026-08-27 發出，其中 **I-097 於同日改列 `todo.md` T-064**——編號**不回收**。）
  **發出新編號時記得把這一行一起往前推**——上一次就是漏了這步，I-089 發出去之後
  這裡還寫著「從 I-089 起算」，差一點又重用一次（I-070 已經發生過）。
  檔案裡看得到的最大是 I-103（2026-09-02 發出，**下一個可用的是 I-104**——I-096 / I-098
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
  # ⚠️ 樣式是 I-[0-9]{3} 不是 I-0[0-9][0-9]——後者在編號進到 I-100 之後就掃不到了
  # （2026-09-01 發現：當時的指令對 I-101 完全無效，等於檢查形同虛設）。
  comm -13 <(grep -oE '^### I-[0-9]{3}' docs/issue.md | sed 's/### //' | sort -u) \
           <(rg --no-filename --only-matching --no-messages \
                --glob '!**/node_modules/**' --glob '!**/dist/**' \
                --glob '*.{md,go,ts,svelte,py,sh,yml,yaml,sql}' \
                'I-[0-9]{3}' . | sort -u)
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
  I-081～I-084 / I-086～I-090 / I-093、I-096、I-098、已作廢的 I-099、已收斂的 I-101、
  本檔現有的 I-100 / I-102 / I-103 與下一個可用的 I-104），
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
   目前對真實 MySQL 有執行證明的**只有四個 repo 寫入路徑**，都是靠
   `TestMySQLMigrationsRealValuesFitAllColumns` 順帶涵蓋的：

   * `CorporateActionRepo.Upsert`（`ON DUPLICATE KEY UPDATE`）
   * `EvaluationUniverseRepo.Upsert`（2026-08-17 T-040 Step 5 新增，同樣是
     `ON DUPLICATE KEY UPDATE` 分支）
   * `IndicatorRepo.Upsert` ＋ `GetLatest`（2026-09-01 隨 migration 075 新增，
     **唯一一條有 round-trip（寫入後讀回比對）的路徑**）
   * `SignalRepo.Insert`（2026-09-01 隨 migration 075 新增）

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
| 狀態 | **待執行**（處置已定；目前受 [I-100](#i-100decision-replay-沒有-as-of-上界cohort-隔天就重現不了) 的重現性前置阻擋）。處置＝**只執行一次有界定向驗證，零命中即收斂成已知限制**，步驟與判準見下方「處置（2026-09-01 定案）」與「關閉條件（2026-09-01 改為單一決策樹）」。**在決策樹的某一個分支被走完之前不得移除本筆** |
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
「收復 ＋ 上行跟隨 ＋ 動能確認 ＋ 明確突破」形態的標的與期間。
**這個成本要不要投入已於 2026-09-01 定案**（投入一次、界線先畫死），見下方
「處置（2026-09-01 定案）」；在那條路徑跑完之前，這個行為改變的接受仍然是**明示的決定**，
不是「驗過沒問題」。

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

#### 處置（2026-09-01 定案）：一次有界定向驗證，結果決定收斂方式

**本筆不再是待決策。** 舊的「命中式／明示接受二選一」已移除——那個寫法允許在看到結果之後
才選路線，等於把判準留到判定當下才定。現在的處置是**單一決策樹**：跑一次預先界定死的定向
驗證，由結果落在哪一個分支決定怎麼收。

**舊關閉條件為什麼被換掉**（保留紀錄，不要再走回去）：2026-09-01 執行的舊條件是「比對
`final_entry_state` / `lifecycle_phase` / `market_bias` 三個欄位的分佈」，三個分佈都比了、
結果零差異，**形式上已達成**。但它沒有要求 cohort 必須**命中被改動的路徑**，所以一個
「一次都沒觸發」的 run 也能滿足它——那並沒有回答本筆要問的「這個已上線的行為改變影響多大」。

##### 驗證目標，以及它證明不了的事

確認「RR 不合格、但價格事實已滿足延續」時，before/after 的 lifecycle 與持倉建議實際如何變化。

⛔ **定向命中只能證明該路徑可達且下游行為符合設計，不能用來推論自然母體的發生率或整體
績效影響。** 收斂時的措辭必須守住這一條。

##### 四個階段

| Stage | 內容 | 產出 |
|---|---|---|
| **0** | 完成可重現性前置（I-100）與診斷欄位，**並產生封存凍結輸入 bundle**（此時才讀 DB） | 可重跑的 replay 路徑 ＋ 新欄位 ＋ 已封存的 bundle |
| **1** | **只用 after 版本**、**從 bundle 載入**連續 replay 整個範圍，掃描精確 predicate | 候選 manifest（`(symbol, as_of)` 清單）＋ after 側的完整逐列輸出 |
| **2** | 用 **before 版本**、**從同一份 bundle 載入**連續 replay **同一個範圍**，再與 Stage 1 的 after 輸出逐列對照 | **全候選**逐列比較 artifact（附 SHA-256）＋ 前 200 列的人讀報告 |
| **3** | 依下方決策樹收斂並歸檔 | `sr-zone-scoring.md` 的結論 |

⛔ **manifest 只表示「要輸出／比對的觀察列」，不是「要計算的列」。**
`_decision_replay_rows()` 用 `previous_event_states_by_symbol` 把同一檔的相鄰列串起來——
每一列把前一列的 `event_state_summary["states"]` 餵進 decision engine
（`evaluation.py:1505`），算完再更新給下一列（`:1521`）。原始碼自己就註明
「維持連續區間（而非等間距抽樣），event lifecycle 的 `previous_event_states`
才有連續的前一根狀態可以接」（`:1428-1430`）。

**所以 Stage 2 不能只孤立計算 manifest 裡的那幾列**：候選之間的狀態演進會消失，
`age_bars`、carry-forward、active 事件集合全都會不同，算出來的 `event_signal` 與
`lifecycle_phase` 可能與 Stage 1 對不起來——那會製造出一個看起來像分支 C、
實際上是取樣方式造成的假矛盾。**before 與 after 都必須從相同的固定起點連續 warm-up 到
候選列，最後才依 manifest 過濾輸出。**

**Stage 1 是掃描不是驗證**——它產出候選名單、不產出命中證據，因此**不違反下方的「只允許
一次正式 scan」**：那條限制的對象是「跑完看結果再回頭改條件」，不是階段拆分。先掃再驗是
必要的，因為候選要靠 `rr_decoupling_candidate` 才挑得出來，而那個 flag 正是 Stage 0 要補的
東西；用相近欄位反推會產生偽陽性（見上方漏斗表那兩列）。

##### 寫死的範圍（Stage 1 執行前就固定，不得因結果調整）

* **標的**：2026-09-01 baseline 的 11 檔，取自
  [`python/baselines/replay_cohort_2026-09-01.json`](../python/baselines/replay_cohort_2026-09-01.json)
  的 `runs[*].rows[*].symbol` distinct 集合——`0050` / `00830` / `00947` / `00981A` /
  `2330` / `2399` / `2454` / `2478` / `3630` / `5490` / `6243`。
  ⚠️ **不要去讀 `_cohort`**：它只存 `symbols_count: 11`，沒有清單本身。
* **as-of 截止日**：固定 **2026-09-01**。
* **每檔載入根數**：`--limit 1500`，**與 baseline 同設定**（`_cohort.limit`）。
  ⚠️ **「所有可用 candidate bars」在現行 `fetch_candles()` 下完全由 `--limit` 決定**——
  不寫死就等於「最近 1500 根」。11 檔的可用上限是 310～4,882 根，但那是**可用量不是載入量**。
  本筆刻意不拉到 4,882，理由是與 baseline 取用**同一段資料窗**，出現異常時可以拿
  2026-09-01 那份逐列資料當對照起點；代價是老標的較早年份的歷史掃不到，
  **這是明示的取捨**。
  ⛔ **但不要宣稱兩者的分佈可以直接比較**（2026-09-01 review 修正）：baseline 是
  `replay_max_rows=200` 的**配額取樣**（每檔 18～19 個 as_of，全部擠在資料尾端），
  本筆是**全候選連續掃描**。母體構成不同，分佈數字不可互相代入。
* **候選列數推導**：每檔 ＝ 載入根數 − `min_history_bars`(80) − `forward_bars`(5)。
  ⚠️ 因為 `forward_bars=5` 從尾端預留，**截止日 2026-09-01 的最後一個可用 as_of 會落在
  08-25 前後而不是 09-01**——baseline 停在 `2026-08-23` 是同一個原因，不是資料缺漏。
* **證據保存：全候選逐列，200 只是「人看的」上限**（2026-09-01 第二輪 review 修正——
  前一版寫「其餘只留彙總」，那與下方關閉條件的「⛔ 不接受 aggregate 當命中證據」直接矛盾：
  要證明**所有**候選都沒有落入分支 C，就必須保留**所有**候選的逐列比較，
  被彙總掉的那些無法獨立複核）。兩個 Stage 都是**全範圍**連續 replay，
  **200 這個上限已經不省任何運算成本**，它只管人類閱讀的報告長度。所以拆成兩份產物：

  | 產物 | 內容 | 用途 |
  |---|---|---|
  | **完整 artifact**（機器可讀） | **全部候選**的逐列 before/after 比較欄位，附自身的 **SHA-256** | 關閉證據的來源。**A/B/C 的統計一律由它產生**，不由報告產生 |
  | **人類閱讀的 report** | 依 `(symbol, as_of)` 固定排序的**前 200 列** | 給人看的摘要，**不是證據** |

  * **總候選數必須全數統計，且全數套用決策樹判定**——分支 A/B/C 看的是全部候選。
    漏判會讓分支 C 被藏起來。
  * **報告必須同時寫明總候選數、附了幾列，以及完整 artifact 的 SHA-256**，
    讓讀報告的人知道自己看到的是子集、並且能取到全集核對。
  * 這份 artifact 的形式已有前例：
    [`python/baselines/replay_cohort_2026-09-01.json`](../python/baselines/replay_cohort_2026-09-01.json)
    就是「逐列比較欄位進版控、原始 report 只留 hash」的同一個做法。
  * 💡 **200 這個上限實務上很可能不會被觸發**：依 2026-09-01 自然樣本 `CONTINUATION`
    佔 0.5% 外推，15,600 列大約只產生 ~78 列，再篩掉 RR 合格的更少。
    規則仍要寫清楚，是為了真的超過時處置不會臨時決定。
* **執行次數**：只允許 **Stage 1 一趟 after 全掃** 與 **Stage 2 一趟 before 全掃**。
  不因結果調整條件、不擴大標的或日期、不加大範圍。
  ⚠️ **preflight 與輸入指紋檢查失敗不計入這個次數**——那是輸入還沒就位，不是驗證跑過了。

**預估成本**（2026-09-01 review 後重算——原估只算了 Stage 1，漏了 Stage 2 必須連續
warm-up 而不是只跑候選列）：

[`sr-zone-scoring.md`](./sr-zone-scoring.md)「規模上限」實測 11 檔 × `--limit 1500` ×
200 列 ＝ 2 分 50 秒，且「replay 的時間由 `replay_max_rows` 決定，每一列都要重建 zone
並跑完整 decision engine」——約 **0.85 秒／列**。範圍約 **15,600 列**。

| Stage | 內容 | 成本 |
|---|---|---|
| 1 | after 版全範圍連續 replay | ~3.7 小時 |
| 2 | before 版全範圍連續 replay | ~3.7 小時 |
| | **合計** | **約 7～8 小時（兩趟全程 replay）** |

**Stage 2 不需要再跑一次 after**——Stage 1 已經是一趟完整的 after 全掃，它的逐列輸出
直接充當比對的 after 半邊。這是把總成本壓在「兩趟」而不是「三趟」的關鍵，
Stage 1 因此必須輸出**完整逐列資料**而不是只有候選名單。

記憶體不是瓶頸（邊際約 1.0 MB/檔，此規模約 300MB，131 檔實測才 382MB），**時間才是**——
7～8 小時遠遠跨出當日 09:00–15:00 的資料凍結窗，這正是 Stage 0 必須先做 I-100 的原因。

##### 需要補的診斷欄位（Stage 0）

replay row 目前拿不到 lifecycle 真正使用的判斷輸入，用相近欄位反推會有偽陽性：

| 欄位 | 現況 | 取得方式 |
|---|---|---|
| **decision primary zone** | ⚠️ **已經有了，只是 replay 取錯顆** | `build_decision_summary()` 早就輸出 `decision_summary["primary_zone"]`（`decision_engine.py:2962`，來源是 `_pick_primary_zone()`）；`evaluation.py` 卻用 `_historical_zone_score_summary` 的排序第一筆（`:773`）。**只要改 `evaluation.py` 的取值來源**（本項不需要動 `decision_engine.py`） |
| `price_follow_through_state` / `momentum_confirmation_state` | ✅ 已在 replay row | `evaluation.py:968-969` 的 `daily_price_follow_through` / `daily_momentum_confirmation` |
| `rr_gate.qualified` | ✅ 已在 replay row | `_decision_fields_from_summary`（`:804`） |
| `event_signal` / `structure_state` | 需補 | 自 decision summary 帶出 |
| `clear_zone_breakout` | ❌ **拿不到** | `resolve_lifecycle()` 內的區域變數（`lifecycle_engine.py:161`），從不回傳。**由 lifecycle 層新增輸出**（診斷用） |
| `continuation_price_evidence_met` | 新增 | **診斷用，不是 candidate 的定義**：三項價格證據齊備與否。**由 lifecycle 層新增輸出** |
| `rr_decoupling_candidate` | 新增 | 定義見下方「candidate 的精確定義」。**由 decision semantic pipeline 組合**——lifecycle 拿不到 RR，組不出這個值 |
| `action_state` | 需補 | semantic pipeline 的 `action_state`，也是 `position_action_condition.state` 的來源 |
| `position_action_condition.state` | 需補 | **判定分支 B/C 的對象** |
| top-level `position_action` | 需補 | 另一條推導（`_decision_action()`），**只記錄不判定**，見下方關閉條件的警語 |

##### candidate 的精確定義（2026-09-01 review 修正）

⚠️ **原文把 `rr_decoupling_candidate` 定義成「三項價格證據 ＋ RR 不合格」，那個範圍太寬。**
真正的 `CONTINUATION` 分支還要求 `event_signal == CLOSE_RECLAIM`
（`lifecycle_engine.py:175`），而且它前面還有一條**優先序更高**的分支
（`:168` 的 `active_bearish_states` / `SUPPORT_RECLAIM_INVALIDATED` / `BREAKDOWN`）會先把
整列吃掉。漏掉任一個，不會翻轉的列都會被收進候選，最後被誤判成分支 C。

**after 版用這個定義，它可以證明是精確的（不是近似）：**

```
rr_decoupling_candidate  ≡  lifecycle_phase == "CONTINUATION"  且  rr_gate.qualified == false
```

**為什麼這一條就等價於「在 before 版不會是 CONTINUATION」**：after 版判定成
`CONTINUATION`，本身已經蘊含「高優先分支不成立 ＋ `CLOSE_RECLAIM` ＋ 上行跟隨 ＋ 動能確認
＋ 明確突破」全部成立；而 before 版的同一條分支只多一個 `and rr_qualified`
（`ecbc141^:decision_engine.py:958-963`）。所以只要 `rr_qualified == false`，before 版那條
必然不成立，往下掉到 `CONFIRMED`（`SUPPORT_RECLAIM_CONFIRMED` 或 `reclaim_age >= 1`）
或再往下的 `TESTING`。**兩個方向都成立，是等價不是充分條件。**

⚠️ **這個定義成立的前提，是 `rr_gate` 在兩個版本裡對同一列會算出相同的值**——已查證：
兩版都是 `market_action` → `entry_action_state` → `rr_gate`
（after `:2674`／`:2677`／`:2678`，before `ecbc141^:2437`／`:2440`／`:2441`），
**全部排在 `decision_derived_view` 之前**（after `:2718`，before `ecbc141^:2472`），
不依賴 `lifecycle_phase`。同理 `event_state_summary` 建於 `:2626`，也在 lifecycle 之前——
所以**某一列的 lifecycle 差異不會回饋到下一列的事件狀態**，before/after 兩趟 replay 會保持
逐列對齊。

**before 版的等價 flag 不能用 `lifecycle_phase`**（它在 before 版本來就不是 `CONTINUATION`），
必須寫成展開式：

```
not (active_bearish_states or structure_state in ("SUPPORT_RECLAIM_INVALIDATED", "BREAKDOWN"))
and event_signal == "CLOSE_RECLAIM"
and price_follow_through == "PRICE_UPSIDE_FOLLOW_THROUGH"
and momentum_state == "MOMENTUM_CONFIRMED"
and clear_zone_breakout
and not rr_qualified
```

兩邊算出來的 candidate 集合必須完全相同；**不相同本身就是分支 C**（tooling 不對稱）。

##### 責任層：三層各做各的，沒有任何一層自行重算

⚠️ **本節於 2026-09-01 review 修正。** 原文寫「`clear_zone_breakout` 與
`rr_decoupling_candidate` 需要的輸入只在 `resolve_lifecycle()` 內同時存在」——**後半是錯的**：
`resolve_lifecycle()` 的簽章**刻意沒有 `rr_gate`**（`lifecycle_engine.py:136-142`，
docstring 明寫「參數裡沒有 rr_gate，是刻意的」）。lifecycle 拿不到 RR，就組不出
`rr_decoupling_candidate`。

正確的分層是：

| 層 | 產出 | 理由 |
|---|---|---|
| `lifecycle_engine.py` | `clear_zone_breakout`、`continuation_price_evidence_met` | 它有三項價格證據，且**只有它**有 `clear_zone_breakout`（`:161` 的區域變數） |
| `decision_engine.py` 的 semantic pipeline | `rr_decoupling_candidate`（定義見上一節） | RR 只在這一層才存在——`rr_qualified` 定義於 **`:1051`**，lifecycle 拿不到 |
| `evaluation.py` | **只匯出，不自行重算** | 任何在 replay 端重算的版本都會重蹈偽陽性 |

**所以 `decision_engine.py` 確實需要修改**——原文寫「預期不需要修改」不成立，已改正。
它只在 semantic pipeline 內組合上游已經給定的兩個布林值，不新增判斷、不改任何既有分支。

**護欄（維持不變）**：三層都**只允許新增輸出欄位，不得更動任何判定條件與優先序**。
為了製造命中而放寬 predicate 是本筆的頭號禁止事項。

##### before 版沒有 `lifecycle_engine.py`——同一套 tooling 要套到兩種形狀

⚠️ **這一點於 2026-09-01 review 補上，原文完全沒有涵蓋。**
before ＝ `ecbc141^`，而 `lifecycle_engine.py` 是 `ecbc141` 才新增的（`git ls-tree` 實查：
`ecbc141^` 下不存在該檔）。before 版的整段判定**內嵌在
`decision_engine.py`**（`ecbc141^` 的 `:946-975`），而且 `rr_qualified` 就寫在
`CONTINUATION` 的條件裡——**那正是本筆要驗的那個條件**。

所以：

* 診斷欄位是 **validation-only tooling**，必須以**兩種不同的形狀**套用到兩個版本：
  after 版走上表的三層分工；before 版的兩個欄位都落在 `decision_engine.py` 同一個函式內。
* **兩邊必須產出語意完全相同的欄位**，否則逐列對照沒有意義。
* **要記錄套用方式與版本**：before／after 各自的 base commit、所套 patch 的內容 hash，
  以及套用後實際執行的檔案 hash，一併寫進 Stage 1／2 的輸出中繼資料。
  ⛔ **沒有這些 hash，「before/after 只差 RR 那一個條件」就是一句無法查核的宣稱**——
  兩邊的 tooling 是分別手工套上去的，任何不對稱都會被算進差異裡。
* tooling 不得改變任一版本的既有判定；套用前後，該版本的既有測試必須全綠。

預計影響 `python/backtest/modular/sr_scoring/` 下的 `evaluation.py`（replay 欄位、報告、
掃描與 warm-up 路徑）、`lifecycle_engine.py`（只加回傳）、`decision_engine.py`
（只組合、只加輸出），以及 before 版對應位置的等價 tooling。

⚠️ **Stage 1 的掃描不能沿用 `_decision_replay_rows` 的取樣窗**：它是
`window_start = max(first_idx, last_idx - quota + 1)`，錨在資料尾端且被 `replay_max_rows`
均分（baseline 那輪 11 檔 200 列 ＝ 每檔只有 18～19 個 as_of，全部擠在 07-28～08-23）。
照抄會讓「全掃」實際變成「只掃尾端 19 個交易日」。

#### 關閉條件（2026-09-01 改為單一決策樹）

結果只會落在三個分支之一。**分支 A 在 Stage 1 就判得出來**——零候選代表沒有東西可比，
**不必再跑 Stage 2**（那趟 before 全掃約 3.7 小時，省下來是實質的）。B 與 C 才需要 Stage 2：

| # | 結果 | 處置 |
|---|---|---|
| **A** | **精確候選數 ＝ 0** | 記錄實際掃描的標的、日期範圍、載入根數、eligible rows、模型 bundle 與設定，**轉為已知限制**並依下方措辭歸檔。本筆關閉 |
| **B** | **候選數 > 0，且 before/after 如預期翻轉** | 記錄**全候選**的逐列證據與下游影響（含 artifact 的 SHA-256），**驗證完成**。本筆關閉 |
| **C** | **候選數 > 0，但沒有翻轉，或下游欄位不符合下表的逐項預期** | ⛔ **這是新的實作／驗證矛盾，不是零命中。本筆不得關閉**，另立新 issue 調查（編號依本檔使用說明的下一個可用值，**不要預先佔號**） |

分支 C 存在的理由：沒有它的話，「掃到候選但行為不符預期」會被歸進 A 一起收成已知限制，
等於把一個**實作問題**寫成「未觀測到」。

**分支 B 的「如預期」是有明確定義的**，不是判定當下的主觀認定。

⚠️ **before 有兩種形狀，兩種的下游預期不同**（2026-09-01 review 修正——原文只寫了
`CONDITIONAL_HOLD → HOLD` 那一種，會把另一種合法命中誤判成分支 C）。成因是
`lifecycle_engine.py:187-192`：RR 被移除前，價格證據齊備但 RR 不合格的列會**往下掉一格**，
落到 `CONFIRMED`（`structure_state == "SUPPORT_RECLAIM_CONFIRMED"` 或 `reclaim_age >= 1`）
或再往下的 `TESTING`。而 `decision_engine.py:1086-1094` 的對照是
**`TESTING → CONDITIONAL_HOLD`、`CONFIRMED`／`CONTINUATION` → `HOLD`**。

**共同必要條件——這一層就是「命中」的定義**（同一個 `(symbol, as_of)` 上）：

* `rr_decoupling_candidate = true`（定義見上方「candidate 的精確定義」）；
* `before ∈ {TESTING, CONFIRMED}` 且 `after == CONTINUATION`；
* `market_state`：`BULLISH_RECOVERY` → `BULLISH_CONTINUATION`。

⛔ **持倉欄位不屬於共同必要條件。** 它依 `market_action` 與 before lifecycle 而定，
**不得用來反過來收窄 candidate**——那會把合法的 lifecycle 翻轉排除掉。

**持倉與進場欄位的逐格預期**（`position_action_condition.state`，before → after）：

| before lifecycle | `market_action != AVOID` | `market_action == AVOID` |
|---|---|---|
| `TESTING` | **`CONDITIONAL_HOLD` → `HOLD`** | **`AVOID` → `AVOID`（不變）** |
| `CONFIRMED` | **`HOLD` → `HOLD`（不變）** | **`AVOID` → `AVOID`（不變）** |

`entry_permission_state` 在**四格全部**都是 `BLOCKED` → `BLOCKED`（不變）。

⚠️ **`market_action == AVOID` 會蓋掉整條 lifecycle 對照**（2026-09-01 review 補上）。
`decision_engine.py:1079-1082` 的 `if market_action == "AVOID"` 是**最外層短路**，
直接令 `action_state = "AVOID"`、`entry_permission_state = "BLOCKED"`，
`elif lifecycle_phase == ...` 那一整串（`:1086-1094`）根本不會被評估。
`market_action` 與 `rr_gate` 一樣算在 lifecycle 之前（`:2674`），
**兩個版本對同一列會得到相同的 `market_action`**，所以 `AVOID → AVOID` 是預期結果。

⚠️ **上表有三格是「不變」，那全都是分支 B 不是分支 C。** 只有 `TESTING` ＋ 非 `AVOID`
那一格會看到持倉建議改變；其餘三格的可觀察差異只有 `lifecycle_phase` 與 `market_state`。
**把「沒變」當成失敗會誤殺絕大多數的合法命中。**

💡 **若之後想專門量測「持倉影響」，另外定義 `position_impact_candidate`**
（＝命中且落在 `TESTING` ＋ 非 `AVOID` 那一格），**不要拿它去改窄 lifecycle candidate**。
兩個問題不同：「這條路徑可不可達」與「它改變了多少持倉建議」。

⚠️ **要看的是 `position_action_condition.state`，不是 top-level `position_action`。**
`_position_action_condition()` 複製的是 semantic pipeline 的 `action_state`
（`decision_engine.py:205`）；top-level 的 `position_action` 是
`_decision_action()` / `_final_action_from_entry()` 另一條推導的產物
（`:2674` / `:2792` / `:2921`），**與 `action_state` 不是同一個東西**。
top-level `position_action` 可以一併記錄供觀察，但**不得拿它判定 RR 解耦是否正確**。

⚠️ **`entry_permission_state` 兩個子案都不會變，這是預期而非異常。** 因為
`decision_engine.py:1098` 的 `elif not rr_qualified: entry_permission_state = "BLOCKED"`
排在 `CONTINUATION` 那條規則**之前**，而候選的定義本身就要求 `rr_qualified = false`——
**RR 解耦不會打開進場閘門**，那正是「lifecycle 只描述事件事實、RR 由 entry gate 處理」的
設計意圖。

**所以下游影響要這樣講才精確**（2026-09-01 review 修正——前一版寫成「只出現在持倉建議線」，
與上表自相矛盾）：**`entry_permission_state` 在四格全部不變**；`lifecycle_phase` 與
`market_state` **四格全部會變**；**額外的持倉建議變化只出現在 `TESTING` ＋ 非 `AVOID`
那一格**（持倉線沒有 RR gate，這與
`test_widened_path_previously_testing_now_continuation` 的敘述一致）。

上表任一格不符就是**分支 C**。這張表存在的唯一理由，是讓 B 與 C 在看到結果之前就已經分得開。

⛔ **不接受 aggregate 當命中證據**（例如「after 的 `CONTINUATION` 總數大於 before」）——
總數可能被反向轉移抵銷或混淆，2026-09-01 那輪的 `qualified` 淨 +3 底下藏著 37 列雙向流動
就是現成的例子。**要逐列。**

⚠️ **這條同時約束了證據要留多少**：三個分支的判定都必須能從**全部候選的逐列資料**重算，
不能只有前 200 列加一份彙總——被彙總掉的候選無法證明它沒有落入分支 C。
保存方式見上方「證據保存：全候選逐列，200 只是『人看的』上限」。

##### 分支 A 的收斂措辭

轉為已知限制，並於 [`sr-zone-scoring.md`](./sr-zone-scoring.md) 明載：

> 此行為改變有單元測試保護，但在已記錄的自然樣本**與一次有界定向掃描**中皆未觀測到；
> 實際發生率與績效影響未知，接受上線是**明示決定**而非實測結論。

之後只有在實際資料出現完整 predicate，或觀察到 `position_action_condition.state` 的實際
影響時才重開（**不是 top-level `position_action`**，理由見上方分支 B 的警語）。

#### 前置：I-100 的可重現路徑（硬性，Stage 1 前必須完成）

⚠️ **這不是註腳，是 blocker。** 沒有 as-of 上界時 cohort 錨在資料尾端，live 每天收盤新增
一根 K 棒，**同一條指令隔天就抽到不同的列**。於是「只跑一次」在實務上等於「必須在當日
09:00–15:00 的資料凍結窗內跑完」，而 Stage 1 預估就要 3～4 小時；一旦跨日就被迫換 cohort，
**那正好違反本筆自己的「不因結果調整條件」**。留 JSON 與指紋只能保存結果，不能確保重新
執行相同輸入。

[I-100](#i-100decision-replay-沒有-as-of-上界cohort-隔天就重現不了) 必須至少做到：

1. `--as-of` **同時**限制 `candles`、chip context 與 model-governance context——只釘 candles
   不夠，後兩者是依 `[dataset_from, dataset_to]` 當下從 DB 撈的
   （`_load_db_replay_chip_context` / `_load_db_replay_model_governance_context`）。
2. 支援**匯出與讀入**明確的 `(symbol, as_of)` cohort manifest。
3. 記錄 model bundle、設定、before/after commit 與各資料來源的內容指紋。
4. **輸入指紋漂移時直接中止**，不得產出看起來可比較的報告。
5. **凍結輸入 bundle 由 Stage 0 產生並封存，Stage 1 與 Stage 2 一律從同一份 bundle 載入**，
   兩個 Stage 全程不碰 DB。⛔ **不要讓 Stage 1 一邊讀 DB 一邊產出 bundle**——那樣 bundle 是
   Stage 1 的副產物，就證明不了 Stage 1 自己用的是哪一份輸入。bundle 的格式、儲存、原子化
   與完整性檢查等六項交付規格見 I-100 的「bundle 的交付規格」。

⚠️ **第 5 項是 2026-09-01 review 補上的，理由是前四項只做到「偵測漂移」不是「保證可重現」**：
`--as-of` 能釘住列範圍、指紋能發現內容變了，但一旦 DB 歷史被修正、還原係數更新或 model
bundle 被換掉，**能做的只有中止，沒有辦法重跑原來那份輸入**。而本筆只允許跑一次，
中止就等於整件事重來。**manifest 是「要觀察哪些列」，bundle 才是 candles／chip／governance／
模型／設定的重現來源**，兩者不能互相取代。

⚠️ **preflight 與指紋檢查失敗不計入「一次正式 scan」。** 那是輸入還沒就位，不是驗證跑過了；
把它算進去會逼人在輸入有問題時硬跑完，正好毀掉這套有界設計。

#### 測試、風險與歸檔

* **測試**：
  * replay 匯出的是 decision primary zone（不是排序第一筆）；`rr_decoupling_candidate`
    由 semantic pipeline 組合而非 replay 端重算；`position_action_condition.state` 與
    top-level `position_action` 兩者都有被帶出**且沒有混用**。
  * **candidate 定義的邊界**各要一支：`event_signal != CLOSE_RECLAIM` 不得入選、
    高優先分支成立（`active_bearish_states` 或 `SUPPORT_RECLAIM_INVALIDATED`／`BREAKDOWN`）
    不得入選、`rr_gate.qualified = true` 不得入選。**這三支就是本輪 review 抓到的三種偽陽性。**
  * **B/C 的四格判定**：`before ∈ {TESTING, CONFIRMED}` × `market_action ∈ {AVOID, 非 AVOID}`
    各一支；三個「持倉不變」的格子都必須斷言**仍屬分支 B**。
  * **before/after 的 candidate 集合等價**：同一組 fixture 下，after 版用
    `lifecycle_phase == CONTINUATION and not rr_qualified`、before 版用展開式，
    兩邊選出的列必須完全相同。
  * **Stage 2 的 warm-up 連續性**：對同一個候選列，連續 warm-up 與孤立計算會得到不同的
    `event_state_summary`。
  * `lifecycle_engine.py` 既有的優先序與 RR 獨立性測試必須全數續存且不修改斷言。
* **風險**：定向挑樣造成代表性誤讀（以「不推論盛行率」的措辭處理）；I-100 未落實造成資料
  漂移（以硬性前置處理）；為求命中而鬆動 predicate（以「只加回傳欄位」的護欄處理）。
* **歸檔**：驗收結論與仍需保留的限制寫進 [`sr-zone-scoring.md`](./sr-zone-scoring.md)，
  本筆經 review 確認後再移除。

---

### I-100：decision replay 沒有 as-of 上界，cohort 隔天就重現不了

| 欄位 | 內容 |
|---|---|
| 狀態 | **待修復**（2026-09-01 由已知限制升級——[I-074](#i-074lifecycle-engine-的-rr-解耦decision-replay-已跑但一次都沒觸發到) 已把本筆列為硬性前置，見下方「必須做到的範圍」。**已造成一次實際後果**，見下） |
| 嚴重度 | 中（不影響 runtime，只影響**驗收證據能不能被獨立複核**） |
| 分類 | Python / SR Zone / 驗證工具 |
| 發現日期 | 2026-09-01 |
| 來源 | 原 `todo.md` T-066（decision replay 前後比對，已於 2026-09-01 收斂）執行時發現——四份 report 的數字在當天之後就無法用同一條指令重建 |

⚠️ **這一筆原本寫在 `todo.md` T-068，2026-09-01 同日改列到這裡**（編號 T-068 不回收）。
理由是 CLAUDE.md 的分流規則：它是**已經發生的已知限制**，不是待規劃的優化。
下方「必須做到的範圍」是它的解法，不是另一個 todo 項目。
（該節原名「可能做法（待評估）」，已於 2026-09-01 收束為必須做到的範圍。）

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

#### 必須做到的範圍（2026-09-01 定案，原「可能做法」已收束）

⚠️ **原本這裡列的是兩個並列選項，其中第二個是「接受不可重現、只強制輸出 cohort 身分指紋」。
那條路已於 2026-09-01 被排除**——[I-074](#i-074lifecycle-engine-的-rr-解耦decision-replay-已跑但一次都沒觸發到)
把本筆列為硬性前置，而它的 Stage 1 掃描預估要 3～4 小時、跨得出當日 09:00–15:00 的資料
凍結窗才跑得完。**只保存結果的指紋無法確保重新執行相同輸入**，撐不住那個用途。

要做到的至少是：

1. **`--as-of <date>` 同時限制三個資料來源**：`_load_db_sources()` 傳入上界、`fetch_candles()`
   加 `ts <= :as_of`；**chip 與 model-governance context 也要一起釘**——它們是依
   `[dataset_from, dataset_to]` 當下從 DB 撈的（`_load_db_replay_chip_context` /
   `_load_db_replay_model_governance_context`），只釘 candles 不夠。
2. **支援匯出與讀入明確的 `(symbol, as_of)` cohort manifest**，讓 Stage 1 產出的候選名單能
   原封不動餵給 Stage 2。
3. **記錄 model bundle、設定、before/after commit 與各資料來源的內容指紋。**
4. **輸入指紋漂移時直接中止**，不得產出看起來可比較的報告。
5. **產出並支援載入「凍結輸入 bundle」**——candles、chip context、model-governance context、
   model bundle 與設定的實際內容，而不只是它們的指紋。

##### 為什麼第 5 項是必要的，不是「不做 as-of 時的替代方案」

⚠️ **本節於 2026-09-01 review 補上。** 原文把凍結 bundle 寫成「若最終決定不實作 `--as-of`」
才要的退路，那低估了依賴：**第 1～4 項合起來只做到「偵測漂移」，不是「保證可重現」。**

`--as-of` 固定的是**列範圍**，指紋能告訴你內容變了——但當 DB 歷史被修正、還原係數更新、
或 model bundle 被替換時，**能做的只有中止，沒有任何機制讓你重新執行原來那份輸入**。
而 [I-074](#i-074lifecycle-engine-的-rr-解耦decision-replay-已跑但一次都沒觸發到)
只允許跑一次，中止就等於整件事重來。

**兩者的分工要講清楚，不能互相取代：**

| 產物 | 回答的問題 |
|---|---|
| **manifest** | 「要觀察／比對哪些 `(symbol, as_of)` 列」 |
| **凍結 bundle** | 「用什麼輸入算出來的」——candles、chip、governance、模型、設定 |

⚠️ **preflight 與指紋檢查失敗不計入 I-074 的「一次正式 scan」**：那是輸入還沒就位，
不是驗證跑過了。這一條要與 I-074 的停止條件一起讀。

##### bundle 的交付規格（實作前要先定，不能邊做邊決定）

⚠️ **2026-09-01 review 補上**：原文只說「要保存實際內容」，那還不是可交付的規格。
下列六項要在實作前定案並寫進計畫書：

| 項目 | 要決定什麼 |
|---|---|
| **格式與 schema version** | 檔案格式、目錄結構，以及一個顯式的 `schema_version`——沒有它，日後改格式就無法分辨「載不進來」是壞檔還是版本不符 |
| **儲存位置與保留期限** | 放哪裡、留多久、誰負責清 |
| **進版控 vs artifact storage** | ⚠️ 這一項要先量體積再決定：11 檔 × `--limit 1500` 的 candles 加上 chip／governance context，很可能不適合直接進 git（現有 `replay_cohort_2026-09-01.json` 已經 553KB，而它只存 4 份 report × 200 列的**比較欄位**，不含任何原始輸入） |
| **原子化產生** | 產生到一半失敗不得留下半份可載入的 bundle——先寫暫存再原子 rename，或寫入完成標記 |
| **bundle ID 與檔案 hash** | 每份 bundle 一個穩定 ID，加上各檔案的內容 hash |
| **loader 完整性檢查** | 載入時逐檔驗 hash，不符就中止（與上方第 4 項的失敗行為一致） |

**產生與消費的順序要分清楚**：

1. **Stage 0 產生並封存 bundle**（此時才讀 DB），封存後不再變動。
2. **Stage 1 與 Stage 2 一律從同一份 bundle 載入**，全程不碰 DB。

⛔ **不要讓 Stage 1 一邊讀 DB 一邊產出 bundle。** 那樣 bundle 是 Stage 1 執行過程的副產物，
Stage 1 自己的輸入就不是「從 bundle 載入的那一份」——真要重跑 Stage 1 時，
你只能證明 Stage 2 用了同一份輸入，證明不了 Stage 1 用了。

⚠️ **本筆已不是單純的工具小修**：它同時改到 replay 的取數邊界、cohort 的身分與失敗行為，
依 CLAUDE.md 屬於「驗證流程修改」＝大規模／高影響異動，**實作前要先寫計畫書**。

#### 關閉條件

三項都要成立：

1. **cohort identity 可重現**：同一條指令在不同日期執行，能產出 `(symbol, as_of)` 完全相同的
   cohort。
2. **輸入可重現**（2026-09-01 review 補上——只有第 1 項不夠）：**在不同日期載入同一份凍結
   bundle，能得到相同的輸入指紋與相同的逐列結果**。⚠️ **驗收要真的跨日做**，
   同一天跑兩次證明不了任何事——當日資料本來就沒變。
3. **失敗行為正確**：輸入指紋漂移時會中止，而不是照樣輸出一份看起來可比較的報告。

並把「驗收報告必須附 cohort 指紋、凍結輸入 bundle，以及**涵蓋全部候選**的逐列比較
artifact 及其 SHA-256」寫進
[`development-workflow.md`](./development-workflow.md)。

⛔ **不再接受「明確決定走第二條路（接受不可重現）」當關閉方式**——理由見上方「必須做到的
範圍」。要改回去必須先解除 I-074 對本筆的前置依賴，不能只動本筆。

---

### I-103：Yahoo 批次盤中路徑的逐檔寫入失敗不會回報，`symbols_failed` 在 live 路徑只有批次粒度

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 中（**live 走的正是這條路**；逐檔寫入失敗會靜默，`job_runs` 照樣 `success`。與本檔的 **I-102** 是同一類「失敗被吞掉」，但發生在**更上游的行情抓取層**） |
| 分類 | Go / 排程 / 市場資料 / 可觀測性 |
| 發現日期 | 2026-09-02 |
| 來源 | I-102 計畫書 review——要定義 `fetchFailed` 集合時發現這條路徑給不出逐檔資訊 |

#### 現象

`FetchAndStoreIntradayBatch`（`backend/internal/market/fetcher.go:76-98`）的回傳值是
`(stored int, err error)`，**沒有逐檔結果**：

* 逐檔 `BulkInsert` 失敗時只 `log.Warn` 然後 `continue`（`:90-93`），**錯誤不往上傳**。
  失敗確實會**間接**反映成 `stored` 沒有增加——但那只是一個數字，
  **看不出是哪一檔失敗**。
* 而且 `stored` 沒增加有**兩種成因**：寫入失敗，或**回應裡本來就沒有這檔的 K 棒**
  （`:87-89` 直接 `continue`）。兩者被壓進同一個計數，**呼叫端分不開**。

**所以問題不是「完全沒有訊號」，而是「訊號不可用」**：知道少了幾檔，
但不知道是哪幾檔、也不知道該不該告警。

而呼叫端 `runIntradayBatch`（`scheduler.go:532-537`）**只有在整批呼叫回 error 時**才
`failed += len(batch)`。所以「批次成功、但其中幾檔沒寫進去」這個形狀，
**`job_runs` 完全看不到**。

⚠️ **live 走的就是這條路**：`YAHOO_ENABLED=true`、`FINMIND_INTRADAY_ENABLED=false`，
`runIntradayJob` 在 `HasIntradaySource()` 為真時轉給 `runIntradayBatch`（`scheduler.go:461-464`）。
FinMind 的 `runIntradayJob` 路徑是逐檔呼叫，反而有逐檔粒度——**但那條在 live 沒有啟用**。

#### 影響

* `job_runs.symbols_failed` 在 live 的盤中路徑**只有批次粒度**，不是逐檔。
* 本檔 **I-102** 要定義的 `fetchFailed` 集合因此在這條路徑上**無法精確**——
  該筆已明確把這個盲區排除在範圍外，並把 union 契約限定為「scheduler 目前識別得到的失敗」。

#### 可能做法（待評估）

* 把回傳值擴充成逐檔結果（例如 `map[string]error` 或 `failedSymbols []string`），
  呼叫端據以累計。
* ⚠️ **要把「寫入失敗」與「回應裡沒有這檔」分開**——後者可能是合法的（停牌、當下無成交），
  當成失敗會製造誤報，那正是 `candle_gap_detection` 花很大力氣分開的同一件事
  （見 [`architecture.md`](./architecture.md)「三種結論必須分得開」）。

#### 關閉條件

`runIntradayBatch` 能區分「整批失敗」「逐檔寫入失敗」「該檔本來就沒有資料」三種情形，
並讓前兩種在 `job_runs` 可辨識；或明確決定不修，轉為已知限制並在
[`architecture.md`](./architecture.md) 寫明 `symbols_failed` 在批次路徑的粒度限制。

---

### I-102：寫 DB 失敗後照樣寫 Redis／推 WebSocket，DB 與快取會靜默不一致

| 欄位 | 內容 |
|---|---|
| 狀態 | **待修復**——**計畫書已完備（2026-09-02），待你確認後才實作**。方向：indicator 採失敗即中止、signal 保留即時送出但必須顯式降級；可觀測性走既有 `job_runs`（不新增表或端點）；分兩段部署，回滾為純 image 回退。依 CLAUDE.md 本筆屬大規模／高影響異動，**計畫書需經確認才能動工** |
| 嚴重度 | 中（**只有 warn log，沒有錯誤傳播、也沒有可操作的告警**；使用者看到的資料會依讀取路徑而不同） |
| 分類 | Go / 錯誤處理 / 快取一致性 |
| 發現日期 | 2026-09-01 |
| 來源 | 原 I-101（`rsi14` / `vol_ratio` 型別溢位，已於 2026-09-01 修復並收斂）的 review 分出——放寬型別只消掉已知的兩種溢位，這個行為本身沒被處理 |

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

**2026-09-01 的 2454 就是這個行為的一次實證**：`rsi14` 算出 100 卻塞不進當時的
`DECIMAL(6,4)`，於是 11:24 之後 API 一直回舊指標，而唯一的痕跡是那行 warn。
（型別本身已放寬並收斂，見 [`database-schema.md`](./database-schema.md)；
**但那只消掉了那一種溢位，本筆的行為沒有改變**。）

##### live 實證：整個盤中 66 輪排程都回報成功（2026-09-01 收盤後查證）

當天 live 的完整數字如下，**它同時證實了「靜默」與「排程看不見」兩件事**：

| 觀察 | 值 |
|---|---|
| `2454` `indicator_snapshots` 最新 `ts` | **11:24**（`rsi14 = 98.7805`——再往上一格就是 100） |
| `2454` `candles` 最新 `ts`（1m） | **13:25** |
| 兩者之間的 1m K 棒 | **66 根，全部沒有對應的 indicator 列** |
| 其餘 10 檔（`0050` / `2330` / `5490` …）indicator 與 candle | **全部對齊** |
| 同期間 `intraday` 排程 | **66 輪，每一輪都是 `success`、`symbols_total=11`、`symbols_failed=0`** |

⛔ **這 66 筆 `success` 就是本筆完成條件第 2 項要消滅的東西。** 排程逐檔呼叫
`Compute`／`Evaluate`，單一 symbol 的寫入失敗既不回傳、也不計數，於是
`job_runs` 一整天都顯示「11 檔全部成功」，而實際上有一檔從 11:24 起就沒再落盤過。
**光看 `/scheduler/status` 或 `job_runs` 完全看不出來**——要比對
`indicator_snapshots` 與 `candles` 的最新 `ts` 才會發現。

⚠️ **但最新 `ts` 比對只抓得到「還在持續落後」這一種**，不要外推成通用的完整性檢查：
中段漏掉幾筆之後只要成功寫入一筆，兩邊的最新 `ts` 就會重新對齊，缺口卻永久留著。
而且三張表與 K 棒**不是一對一**（`2454` 於 2026-09-02 09:05–10:00 實測：
candles 56 根、`indicator_snapshots` 12 列、`signals` 5 列）。
要檢查某段時間有沒有缺列，得依各表自己的寫入契約算預期集合——
見 [`development-workflow.md`](./development-workflow.md)「挑窗口」那節的對照表。

⚠️ **這 66 輪實際走的路徑是 `Scheduler → signalEng.Evaluate → indicator.Compute`**
（2026-09-01 review 修正——前一版寫成 `ComputeAll` 是錯的，**production scheduler 從來沒有
呼叫過它**；`ComputeAll` 與 `EvaluateAll` 目前在 `backend/` 內除了測試以外都沒有呼叫者）。
失敗被吞掉的地方有兩層，兩層都要處理：

**第一層**：`indicator/engine.go:68` 的 `Upsert` 失敗只記 `log.Warn`，`Compute` 照樣回傳成功的
snapshot，所以 `Evaluate` 收到的是成功。

**第二層**：四個 scheduler 呼叫點**都是 `s.signalEng.Evaluate(ctx, sym, tf)`，回傳值連接都沒接**。
各路徑的 `failed` / `lastErr` **只統計自己那條行情抓取／回補的失敗**，來源各不相同：

| 路徑 | 函式 | `failed` 的唯一來源 | `Evaluate` |
|---|---|---|---|
| 盤前預熱 | `runPreMarket`（`:420`） | `BackfillHistory`（`:441`） | `:445`，丟棄 |
| 盤中（FinMind 分 K） | `runIntradayJob`（`:456`） | `FetchAndStoreMinute`（`:487`） | `:501`，丟棄 |
| **盤中（Yahoo 批次）** | `runIntradayBatch`（`:508`） | `FetchAndStoreIntradayBatch`（`:532`，以**整批**計 `failed += len(batch)`） | `:542`，丟棄 |
| 收盤 | `RunDailyClose`（`:549`） | `FetchAndStoreDaily`（`:564`） | `:569`，丟棄 |

⚠️ **這次 66 輪走的是 Yahoo 批次那條，不是 `FetchAndStoreMinute`**——live 的
`YAHOO_ENABLED=true`、`FINMIND_INTRADAY_ENABLED=false`，而 `runIntradayJob` 在
`HasIntradaySource()` 為真時會直接轉給 `runIntradayBatch`（`:461-464`）。
該路徑的抓取與評估**分成兩個迴圈**（先跑完所有批次，再逐檔 `Evaluate`），
所以失敗統計與 `Evaluate` 在結構上完全不相干。

**四條路徑的共同問題是同一個**：只統計行情抓取／回補的失敗，**完全沒有統計 `Evaluate`
的失敗**，所以 `finishRun` 永遠看不到指標或訊號寫入的問題。

**所以 indicator 改成 fail-fast 只解掉第一層**：`Compute` 會回錯、`Evaluate` 會往上傳，
但**四個 scheduler 呼叫點仍然必須改成接住 `Evaluate` 的 error 並彙整到 `job_runs`**，
否則 66 輪 `success` 的現象一模一樣。這正是本筆完成條件第 2 項的內容。

（`ComputeAll` / `EvaluateAll` 裡同樣的吞錯寫法可以順手一起修，但**它們不在這次實證經過的
路徑上**，不要把它們當成主要修法。）

**影響面剛好被限制住是運氣不是設計**：只有 `2454` 的 `rsi14` 撞到那個上限，
所以只有它靜默停擺。同樣的行為換成 DB 短暫不可用，會是整池一起靜默。

#### 裁決（2026-09-01）：採分路徑一致性契約

採原方案 (3)，但不是單純把 warn 升級，而是把兩條路徑的成功語意分開定義：

* **indicator 採 fail-fast**：DB `Upsert` 是成功的必要條件。失敗時回傳可辨識的 persistence
  error，**不得**寫 Redis，也不得讓 signal evaluation 繼續使用這份未落盤 snapshot。
  手動 API 將此類錯誤映射為 **503**，而不是目前籠統的 422。
* **signal 採 degraded-success**：即時送出優先於歷史完整性。`signals` 的 `Insert` 失敗時
  仍可嘗試 Redis queue 與 WebSocket，但結果必須明示哪些階段成功。
  這個決定**明示接受局部故障期間可能永久缺少 signal 歷史**。

##### ⚠️ degraded-success 涵蓋的是「局部失敗」，不是 DB outage

**2026-09-02 review 修正——前一版的理由寫成「DB 短暫故障不應同時中斷 signal 即時推播」，
那是錯的。** `Evaluate` 的第一行就是 `indicator.Compute`（`signal/engine.go:58-61`），
所以 **indicator 一旦 fail-fast，全域 DB outage 會在那裡就 return，signal 根本不會被評估**，
Redis queue 與 WebSocket 一樣不會被碰到。

| 故障形狀 | 實際行為 |
|---|---|
| **全域 DB 不可用** | `Compute` 讀 candles 或 `Upsert` 就失敗 → `Evaluate` 早退 → **沒有推播**。degraded-success 這條路走不到 |
| **`signals` 單表／單欄位／單筆失敗**（indicator 落盤正常） | `Insert` 失敗 → **走 degraded-success**：照樣推播，但標記 `db_persisted=false` |

**所以真正的取捨是**：「`signals` 寫不進去時，不要因為歷史缺一列就吞掉一個已經算出來的
即時訊號」——**不是**「DB 掛掉時保住推播」。這個範圍**剛好涵蓋 I-101 那種形狀**
（`signals.vol_ratio` 的型別溢位就是單欄位失敗），也是本筆最可能真實遇到的情境。

不採兩條路徑全面 fail-fast，是因為單表失敗不該連帶吞掉即時訊號；也不採兩條路徑都繼續，
是因為 indicator API 直接讀 DB，同一 snapshot 在 DB 與 Redis 不一致會立即形成對外矛盾。

⚠️ **實作與測試要照上表分兩種情境造**，不要只造「DB 全掛」——那個情境測不到 degraded 路徑。

#### 目標流程與回傳契約

**Indicator：** `Compute` 成功計算 → DB `Upsert` → Redis cache → 回傳 snapshot。
DB 失敗就停在該步；Redis 失敗的既有容錯是否維持，實作時需明確保留或另案處理，不得把
「Redis disabled／READONLY backoff 回 nil」誤記成已成功寫入。排程器應繼續處理其他 symbol，
但該 symbol 必須計為失敗，並讓 `job_runs` 呈現 partial／failed，而不是整輪看似成功。

**Signal：** indicator 成功 → 評估訊號 → DB `Insert` → Redis queue → WebSocket broadcast。
回傳值需至少能表達：

| 欄位 | 語意 |
|---|---|
| `signal_generated` | 本次是否產生訊號 |
| `db_persisted` | signal history 是否成功落 DB |
| `queue_enqueued` | 是否確認加入 Redis queue |
| `broadcast_attempted` | 是否呼叫 broadcast；這只代表嘗試／送入程序內流程，不宣稱客戶端已收到 |
| `degraded` | 任一必要持久化或分發步驟失敗（等同 `len(degraded_stages) > 0`） |
| **`degraded_stages`** | **降級的分類清單**，值域為 `signal_persist_failed` / `queue_failed` / `dedup_degraded`。⚠️ **沒有這個欄位，scheduler 就產不出分類摘要**——只有一個布林時，`job_runs.error` 無法區分是哪一種降級 |
| **`stage_errors`**（內部用） | `map[Stage]error`——**各 stage 的實際錯誤**。⚠️ **只有 stage 分類拿不到錯誤內容**：`job_runs.error` 的摘要要附**代表 reason code**，而 reason code 得由原始 error 分類而來——光有 `degraded_stages` 只能產出數量。**這是 Go 端的內部欄位，不進對外 JSON**——但 `json:"-"` **不足以防外洩**，見下方「錯誤訊息的外洩邊界」 |

Signal API 在訊號已產生但 DB 寫入失敗時仍可回 200，但回應必須帶上述降級資訊；沒有 retry
worker 時不得用 202 暗示稍後一定完成。若連 duplicate lookup 都因 DB 故障而失敗，仍要有一個
**不依賴 signal DB 的短期 emission dedup**（以目前 15 分鐘 cooldown 為界），避免每次排程都
重複推送同一訊號。`signal:queue` 目前沒有 consumer，不能把它描述成補寫 DB 的 retry 機制。

#### 獨立 dedup 的設計（2026-09-02 補完）

⚠️ **前一版只在風險段寫「必須定義 Redis 不可用時的行為」，那不是設計。**
沒有下列裁決，result contract 與測試都沒有唯一答案。

**為什麼需要它**：現行 `shouldSuppressDuplicate()`（`signal/engine.go:118`）靠
`signals.GetBySymbol` 讀 DB 判重，**失敗時 fail-open**（`:86-88` 只記 warn 就往下走）。
於是 `signals` 寫不進去時，判重與寫入**同時失效**——每一輪排程都會重新推送同一個訊號。

| 項目 | 裁決 |
|---|---|
| **儲存位置** | Redis，key `signal:emitted:{symbol}:{identity}` |
| **identity key** | 由 `sameSignalIdentity()` 現有的比對欄位推導（`signal_type` ＋ `direction` ＋ 該型別用到的價位）。⛔ **不要另立一套**，否則兩層判重對「同一個訊號」的定義會分歧 |
| **原子 check-and-reserve** | `SET key <uuid_token> NX PX <cooldown_ms>`。**用 `NX` 而不是先 GET 再 SET**，多 instance 才不會同時放行。⚠️ **值是 UUID token 不是時間戳**，釋放時要靠它做 compare-and-delete。⛔ **API 必須回明確狀態，不能回 `(bool, error)` 再靠 `err == nil` 判斷**，見下 |
| **TTL** | ＝ `signalCooldown`（現值 15 分鐘，`signal/engine.go:21`），自動過期，不需要清理 |
| **何時保留／釋放** | **送出前先 reserve**；若 `queue_enqueued == false` **且** `broadcast_attempted == false`，以 compare-and-delete 釋放。⚠️ **判準是「有沒有嘗試」不是「有沒有成功」**——`BroadcastFn` 沒有回傳值（`signal/engine.go:34`），成敗根本表達不了。⚠️ **釋放只在「`signals.Insert` 也失敗」時真的有效**，見下 |
| **與 DB 判重的順序** | ① DB 判重成功且說抑制 → 抑制、**不 reserve**；② DB 判重成功且說放行 → reserve 後送出；③ **DB 判重失敗（現在會 fail-open）→ 直接進 reserve**，由它擋下重複 |

⚠️ **通過 DB 判重時也要 reserve**（上表第 ②列）——否則 cooldown 中途 DB 才掛掉時，
Redis 裡沒有那筆保留，重複推送照樣發生。

##### 三個必須先定義的邊界（2026-09-02 review 補上）

**(a) 價位的 canonicalization——DB 與 Redis 必須用同一個定義**

`sameSignalIdentity()` 現在用 `samePrice()` 的 `math.Abs(a-b) < 1e-6`（`signal/engine.go:151-153`）
做**容差比較**，而 Redis key 需要的是**離散字串**。兩者各自為政時，同一組價位可能
DB 判「同一訊號」、Redis 判「不同訊號」（或反之），判重就會出現破口。

**裁決：只保留一個定義。** 新增

```
signalIdentityKey(sig) = symbol | signal_type | direction | canonicalPrice(該型別的價位)
canonicalPrice(f)      = strconv.FormatFloat(math.Round(f*1e6)/1e6, 'f', 6, 64)
```

價位欄位依型別選取，與現行 `switch` 一致：`BREAKOUT` → `Resistance`；
`BREAKDOWN` / `SUPPORT_BOUNCE` → `Support`；其餘型別不帶價位。

**並把 `sameSignalIdentity()` 改成比較這個 key**，而不是留兩套。
⚠️ **這會在 1e-6 邊界上微幅改變 DB 判重的語意**（原本的容差比較與量化後的字串相等
在剛好差 ~1e-6 時可能不同調）。**這是刻意的取捨**：一個定義的可預測性，勝過兩個定義在
邊界上各自正確。要有測試釘住量化行為與型別選欄。

**(b) Redis 復原後不得繞過還沒過期的 local reservation**

Redis 故障期間 reservation 寫在 process 內 map；Redis 一旦復原，**若只查 Redis 就會看不到
那些還在 TTL 內的 local reservation**，於是同一個訊號在 cooldown 內被重複推送——
故障復原的當下正是最容易重複的時刻。

**裁決：先 local、後 Redis，五步一條路，不得改寫成別的順序。**

前一版寫成「先查 local → Redis `SET NX` 成功後再寫 local」，那**沒有定義跨儲存的競爭**
（2026-09-02 review 修正）：在第一個 goroutine 等待 Redis 回應的空窗裡，
另一個 goroutine 可能因 Redis 暫時失敗而先取得 local reservation；等第一個的 Redis 成功回來，
兩邊都認為自己拿到了，**同時放行**。

**唯一流程：**

| 步 | reservation API 的結果 | 動作 | degraded |
|---|---|---|---|
| 1 | — | **以 UUID token 原子取得 local reservation**（單一操作，見下方 mutex 規則） | — |
| 2 | — | local **沒取得** → **抑制**，結束（不必問 Redis） | — |
| 3 | `reserved` | **兩邊都保留** → **放行** | 無 |
| 4 | `already_exists` | 以 compare-and-delete **回滾自己的 local token** → **抑制** | 無 |
| 5 | `disabled`（`rdb == nil`，設定如此） | **保留 local reservation** → **放行** | **無**——設定狀態不是故障 |
| 6 | `backoff`（READONLY 退避中，或本次就是 READONLY 錯誤） | **保留 local reservation** → **放行** | **`dedup_degraded`** |
| 7 | `error`（其他非預期錯誤） | **保留 local reservation** → **放行** | **`dedup_degraded`** |

⚠️ **第 5 與第 6 步的差別是本筆的既有裁決，不能合併**（2026-09-02 review 補上——
前一版只有「成功／已存在／非預期錯誤」三個分支，把這兩種都漏了）：
Redis **設定停用**是預期狀態，**不算 degraded**；**READONLY backoff** 是非預期失敗，
**算 `dedup_degraded`**。兩者在現有 client 裡長得一模一樣（都回 `nil`），所以**必須靠
reservation API 自己回報狀態**，見下。

⚠️ **第 6 步只涵蓋 reservation 這一階段。** 同一輪稍後的 `LPush` 若也因 backoff 被跳過，
那是**另一個 stage**，記 `queue_failed`，兩者可以同時出現。

**這個順序的好處是 mutex 不必跨越 Redis 網路呼叫**——local 那一步自己就是完整的臨界區，
Redis 抖動也不會讓兩邊同時放行（第 4 步負責回滾）。

⛔ **第 1 步的「查 ＋ 寫」必須在同一個 mutex critical section 內完成。**
寫成「先查、放開鎖、再寫」時，兩個 goroutine 會同時查無資料而一起通過第 1 步。
介面上就設計成單一操作（`localReserve(key, token, ttl) (ok bool)`），
**不要暴露分開的 `get` / `set`**，避免呼叫端組錯。

* local map 用與 Redis **相同的 key 與 TTL**。

  ⛔ **不能只做「讀到同一個 key 才刪」的惰性過期**（2026-09-02 review 修正——
  前一版就是這樣寫的）：identity 帶價位，**價位一變就是新的 key**，
  舊 key 再也不會被讀到，於是永遠不會被清掉——長時間執行下 map 會**持續成長**。

  **裁決：定期整掃，只清除已過期項目**（2026-09-02 review 定案——前一版留了「二選一」，
  其中的容量上限方案會破壞 cooldown 保證，見下）。

  在 `localReserve` 的 mutex 內，**每分鐘至多一次**掃除**所有已過期**的項目
  （用上次清理時間當節流，不另外開 goroutine）。

  ⛔ **不採容量上限＋淘汰最舊項目。** Redis 處於 `disabled` / `backoff` 時，
  **local map 是唯一的判重層**；淘汰一個**尚未過期**的 reservation 會讓同一個 identity
  在 cooldown 內再次放行——**直接違反 dedup 契約**，而且正好發生在保證最弱的時候。
  真要加容量上限，就得另外裁決「容量滿且全部未過期」時要**抑制新訊號**還是**接受重複**，
  兩者都有業務影響，**不在本筆範圍**。

  ⚠️ **成本的正確說法**：整掃仍由**某一次** `localReserve` 支付 O(n)，
  只是**掃描頻率有上界**（每分鐘一次）。**不需要背景 goroutine**，
  但不能說「不會讓清理成本落在單一次呼叫上」——前一版那句是錯的。
* 第 4 步的回滾**同樣是 compare-and-delete**（只在值等於自己的 token 時才刪），
  理由與 Redis 端相同。

**(c)「送出成功」的定義，以及釋放時的 compare-and-delete**

`BroadcastFn` 的簽章是 `func(symbol string, sig *store.Signal)`——**沒有回傳值**
（`signal/engine.go:34`），所以**無法證明客戶端收到**，只能證明呼叫完成。

**裁決：**

* **`broadcast_attempted` ＝「`BroadcastFn` 非 nil 且已回傳」**，語意就是 *delivery
  attempted*，**不宣稱 delivered**。`api-reference.md` 要照這個措辭寫。

  ⛔ **本筆沒有「broadcast 失敗」這個狀態，全文不得使用該說法**（2026-09-02 review 修正）。
  `BroadcastFn` 的簽章是 `func(symbol string, sig *store.Signal)`，**沒有回傳值**
  （`signal/engine.go:34`），所以只分得出「有嘗試／沒嘗試」，**判定不了成敗，
  更判定不了使用者有沒有收到**。要能觀測 broadcast 失敗，就得改 callback 與 Hub 的
  回傳契約——**那會擴大本筆範圍，不在這次做**。
* **保留 reservation 的條件**：`queue_enqueued`（真的寫進 Redis）**或** `broadcast_attempted`
  任一成立。
* **兩者都沒成立才釋放**，而且**必須是 compare-and-delete**：reserve 時寫入一個
  **reservation token**，釋放時只在**值仍等於自己的 token** 時才刪。

  ⛔ **token 一律用 UUID**（2026-09-02 review 裁決——前一版還列了 `runID+ts` 當選項，那不可行）：
  signal engine 與手動 API **不一定有 scheduler 的 run ID**，而且**同一根 candle 的 `ts`
  可以被重複評估**，兩者組不出唯一值。`github.com/google/uuid v1.6.0` **已在 `go.mod`**
  且已被 `api/handler/sr_zones.go` 使用，直接用即可，不必新增相依。
  ⛔ **不要用裸 `DEL`**——舊請求的釋放會刪掉後來那次已經成立的新 reservation，
  等於自己打開重複推送的破口。
  ⛔ **也不要用「`GET` 比對後再 `DEL`」**（2026-09-02 review 修正——前一版把它列為可行方案，
  那是錯的）：兩個指令之間 reservation 可能已經過期並被別人重新取得，
  於是照樣刪掉別人的。**Redis 端必須是原子的 compare-and-delete**——
  用 **Lua 腳本**（`if redis.call('get',KEYS[1])==ARGV[1] then return redis.call('del',KEYS[1]) end`）
  或 `WATCH` 交易，二選一。local map 的比對與刪除同樣要在**同一個 critical section** 內。

##### ⚠️ reservation API 不能沿用現有 client 的 `nil` 語意

**2026-09-02 review 補上。** 現有 `RedisClient` 把**三種完全不同的狀態折成同一個 `nil`**：

* `rdb == nil`（設定停用）→ `return nil`（例如 `Set` 在 `store/redis.go:126-128`）；
* `skipWrite()`（READONLY 退避期間）→ `return nil`（`:129-131`）；
* **實際的 READONLY 錯誤** → `handleWriteErr` 設定退避後**也 `return nil`**（`:151-154`）。

所以 `err == nil` **既不代表寫成功、也分不出是哪一種跳過**——這正是 `LPush` 那個坑的同一個成因。

**裁決：reservation 走自己的 API，回封閉的結果值域**：

```
type ReserveResult int   // reserved / already_exists / disabled / backoff / error
```

上表第 3～7 步就是照這五個值分支。**`disabled` 與 `backoff` 必須分得開**，否則
「設定停用不算 degraded、READONLY 算 degraded」這條裁決在實作層無從落實。

⛔ **compare-and-delete 也要走明確語意**，但**值域與 reservation 不同**：
`deleted` / `not_owner` / `disabled` / `backoff` / `error`。

**本筆新增或調整的三個 Redis 操作**（reservation / enqueue / compare-and-delete）
**一律回「status ＋ cause」**，例如：

```
type ReserveOutcome struct { Status ReserveStatus; Err error }   // enqueue / 釋放各自同形
```

⛔ **限於這三個**（2026-09-02 review 修正——前一版寫「所有 Redis 操作」，
字面上會要求連既有的 `HSet` / `SAdd` / `Set` 等一起改契約，那超出本筆範圍）。

* **`Status` 決定流程**，⛔ **不得再用 `Err == nil` 推導狀態**。
* **`Err` 只供內部觀測**——餵給 `stage_errors` 與 log，**寫進 `job_runs.error` 前一律
  過分類器**（見「錯誤訊息的外洩邊界」）。

##### `Err` 的不變量（三個操作一體適用）

⚠️ **`backoff` 時沒有底層 error 可帶**——`skipWrite()`（`store/redis.go:142-145`）
只回一個布林。若 `Err` 留空，`safeJobErrorReason(error)` 產不出 `readonly`，
`stage_errors` 也沒有東西可存。**所以要有 sentinel**：

| Status | `Err` |
|---|---|
| `backoff` | **必須是固定 sentinel**（例如 `ErrRedisWriteBackoff`），可用 `errors.Is` 判斷 |
| `error` | **必須是非空的原始 cause** |
| `reserved` / `enqueued` / `already_exists` / `disabled` / `deleted` / `not_owner` | **`Err == nil`** |

* **分類器把 `ErrRedisWriteBackoff` 固定映射成 `readonly`。**
* 這條不變量**三個操作都要遵守**，並各自有測試（見完成條件）。

**compare-and-delete 的呼叫端處置（2026-09-02 review 補上，前一版只有值域沒有處置）：**

| status | 處置 | `dedup_degraded`？ |
|---|---|---|
| `deleted` | 正常完成，接著做 local compare-delete | 否 |
| `not_owner` | **正常完成**——那筆已經是別人的 reservation，本來就不該刪；接著做 local compare-delete | 否 |
| `disabled` | 正常完成（Redis 本來就沒有那筆），接著做 local compare-delete | 否 |
| `backoff` / `error` | **標記 `dedup_degraded`**、記錄 `cause`；**Redis 端的殘留交給 TTL 自然過期**（`PX` 已經設過，最多多擋一個 cooldown） | **是** |

⚠️ **`backoff` / `error` 時 local token 仍要釋放。** 兩邊的保證等級不同：Redis 那筆
反正會過期、且此刻也刪不掉；**local 若不釋放，本 instance 會被自己多擋一個 cooldown**，
而那個 reservation 對應的是一次「連嘗試都沒送出去」的訊號。
**寧可讓本 instance 下一輪能重試，也不要靠一個刪不掉的殘留把自己鎖住。**

##### ⚠️ 錯誤訊息的外洩邊界：`job_runs.error` 是使用者可見面

**2026-09-02 review 補上。** 前一版只規定「對外 JSON 要安全轉換」，那漏了一條路徑：
**`job_runs.error` 本身就會流到前端**——

* `GET /scheduler/status` 把 `error` 放進回應（`api/handler/scheduler.go:197` 起的 `jobStatus`）；
* 前端 `Scheduler.svelte:227` 以 `{job.error}` **原樣渲染**。

所以**只把 `stage_errors` 標成 `json:"-"` 完全擋不住**：原始 driver 錯誤常帶 DSN、
主機位址、連線字串或底層 SQL 片段，寫進 `job_runs.error` 就等於直接顯示在畫面上。

**規則（三個原始錯誤來源一體適用：`fetchFailed` / `evaluateFailed` / `stage_errors`）：**

| 用途 | 可以帶什麼 |
|---|---|
| **內部 log／debug** | 原始 error 原文 |
| **寫進 `job_runs.error`** | **只准 stage ＋ symbol ＋一個封閉值域的 reason code**，見下 |

⚠️ **轉換要在寫入前做，不是在顯示端做**——`job_runs` 是持久化的，一旦寫進去，
30 天內每次查詢都會再洩一次。

**reason code 必須是封閉值域，而且未知一律吃掉**（2026-09-02 review 補上——
前一版只舉了幾個例子沒有定義值域，那會留下最危險的實作空間：
分類器遇到不認得的錯誤時 fallback 成 `err.Error()`，敏感資訊照樣寫進去）：

* **由單一函式負責分類**，例如 `safeJobErrorReason(error) ReasonCode`。
  ⛔ **只有它能決定寫進 `job_runs.error` 的原因字串**，其他地方不得自行拼接。
* **`ReasonCode` 是封閉列舉**，起始值域（不足時再擴，但擴的是列舉不是規則）：

  | reason code | 對應 |
  |---|---|
  | `numeric_overflow` | 欄位型別容不下（I-101 那種） |
  | `constraint_violation` | 唯一鍵／外鍵／check 違反 |
  | `conn_refused` | 連不上 DB |
  | `timeout` | ctx 逾時或 statement timeout |
  | `readonly` | Redis READONLY／退避 |
  | `serialization_failure` | 交易衝突需重試 |
  | `insufficient_data` | **資料不足**（例如 candles 少於 35 根）。⚠️ **必須有這一項**——否則排程上很常見的「這檔資料還不夠」會被歸成 `internal_error`，摘要就失去診斷價值 |
  | **`internal_error`** | **以上都不匹配的一律歸這裡** |

* ⛔ **未辨識的錯誤一律輸出 `internal_error`，不得退回原文**。
  想知道細節就去看 log——那是原始 error 唯一被允許出現的地方。
* ⛔ **禁止用原始 error 字串直接拼接**（`fmt.Sprintf("%v", err)`、`err.Error()` 等）
  組出寫進 `job_runs.error` 的內容。
* **三個原始錯誤來源（`fetchFailed` / `evaluateFailed` / `stage_errors`）全部走同一個分類器**，
  不要各寫各的。

##### 明確的限制（要寫進文件，不要事後被當成 bug）

* **Redis 不可用時失效的是「跨 instance 保證」，不是整層**（2026-09-02 review 修正——
  前一版寫「這層也失效」，不精確）。**local dedup 仍然有效**，只是保證範圍縮小：

  | Redis 狀態 | 判重保證 |
  |---|---|
  | 正常 | **跨 instance**（`SET NX` 是全域仲裁） |
  | 停用／backoff／錯誤 | **僅 per-instance**（只剩 local map） |

  ⛔ **local map 是 per-instance 且重啟即失憶**——不能宣稱具有強一致性。
  這是**刻意接受的降級**，不是待修的缺陷。
* **「DB 與 Redis 同時不可用」不在保證範圍內。** 那個形狀由本筆下方
  「`job_runs` 有循環依賴，接不住全域 DB outage」所定的 **`/health` 那一格**承接，
  不是靠 dedup 硬撐。
* ⛔ **「釋放 reservation 讓下一輪重試」只對「`signals.Insert` 也失敗」的情況成立**
  （2026-09-02 review 修正——前一版一概宣稱能重試，那是錯的）。
  若 **`Insert` 已成功、而 `queue_enqueued == false` 且 `broadcast_attempted == false`**，
  下一輪會先被 **DB 判重**擋下（`shouldSuppressDuplicate` 讀得到剛寫進去的那一列，
  且在 cooldown 內），**根本不會重送**——釋放 reservation 完全沒有作用。

  **裁決：明示接受「DB 已落盤、但 `queue_enqueued` 與 `broadcast_attempted` 皆為 false 時
  不自動重送」。**
  理由是另一條路（delivery outbox ／ retry worker）會把本筆從「錯誤處理語意」
  擴張成「投遞保證」，那是獨立的一階，且現況 `signal:queue` 連 consumer 都沒有。
  **真有業務需求時另立項目**，見「風險與非範圍」。

  ⚠️ **若 queue 是非預期失敗（READONLY／enqueue 回錯），這個情形仍必須是 `degraded`
  且看得見**——歷史有那一列、即時通道卻一個都沒送出去，
  是**最容易誤判成「系統沒發訊號」**的形狀。
  ⛔ **但不要寫成「使用者沒收到推播」**：`broadcast_attempted` 只代表有沒有嘗試，
  系統無從得知客戶端收到與否。

##### 降級分類的裁決（測試要照這個斷言）

| 情形 | `degraded`？ | 理由 |
|---|---|---|
| `signals.Insert` 失敗 | ✅ **是** | 歷史缺列，是非預期失敗 |
| Redis **停用**（`rdb == nil`，設定如此） | ❌ **否** | 那是設定狀態不是故障。dedup 退回 process 內 map，屬已知限制 |
| Redis **READONLY backoff 或 enqueue 回錯** | ✅ **是**（`queue_failed`） | 非預期失敗 |
| Redis **非預期故障**導致 reservation 退回 local map | ✅ **是**（`dedup_degraded`） | 判重從跨 instance 退成 per-instance，保證變弱了 |
| `BroadcastFn == nil` | ❌ **否** | 那是「沒有接 API 層」的接線方式（測試／CLI），不是故障 |
| DB 判重查詢失敗但 reserve 成功、訊號正常送出 | ✅ **是** | 判重已降級成單層，要看得見 |

⚠️ **`queue_enqueued` 只在「真的寫進去」時為 true。** 現行 `LPush`
（`store/redis.go:77-88`）在 `rdb == nil` 與 `skipWrite()` 兩種情況都回 `nil`，
**分不出「寫了」與「跳過」**，不能用 `err == nil` 推導。

⛔ **三態不夠，必須四態**（2026-09-02 review 修正——前一版寫「寫入／跳過／錯誤」，
但「跳過」同時混了兩種語意不同的情況）：

```
type EnqueueResult int   // enqueued / disabled / backoff / error
```

| 結果 | `queue_enqueued` | `queue_failed`？ |
|---|---|---|
| `enqueued` | `true` | 否 |
| `disabled`（`rdb == nil`，設定如此） | `false` | **否**——設定狀態不是故障 |
| `backoff`（退避中，**或本次就收到 READONLY**） | `false` | **是** |
| `error` | `false` | **是** |

⚠️ **第一次收到 READONLY 也必須回 `backoff`。** 現行 `handleWriteErr`
（`store/redis.go:151-154`）在偵測到 READONLY 時**設完退避就 `return nil`**——
等於把那一次的失敗吃成成功。**新 API 不得沿用這個行為。**

這與 reservation 的五態是同一條規則：**disabled 與 backoff 必須分得開**，
否則「停用不算 degraded、READONLY 算」在實作層落實不了。

#### 可觀測性：承接點定為 `job_runs`，不新增表也不新增端點

⚠️ **本節於 2026-09-02 定案。** 前一版只說「沿用 `architecture.md` 的可觀測性範圍、
維護總失敗數與連續失敗數、透過既有 health/status 暴露」——那是**第三種做法**，
既有的兩個承接點都接不住它：

| 候選承接點 | 為什麼不用 |
|---|---|
| `/health`（`api/server.go:76-84`） | **不適合放 per-symbol 失敗計數**：它是**未授權**的 docker 探針，註解明寫「錯誤細節只寫 log，不回給呼叫端」，放逐檔失敗同時牴觸這個設計與授權邊界。⚠️ **但它並非只回 `ok`**——DB ping 失敗時會回 **503 `{"status":"down"}`**，所以它**正是「全域 DB 不可用」那一格的承接點**，見下方「`job_runs` 有循環依賴」 |
| process 內計數器 | `architecture.md`「可觀測性：為什麼沒有 metrics 依賴」立的模式是**一張表 ＋ 一個查詢端點**（`sr_identity_stats`），不是記憶體計數器。計數器重啟歸零，也沒有查詢介面 |
| 新開一張表 ＋ 端點 | 那個模式是為**趨勢型**觀測設計的（「這個月 alias 命中率有沒有在爬」）。本筆要答的是「**這一輪有沒有掉寫入**」——那是 per-run 事實，屬於執行紀錄，不是時間序列。而且會引入 migration，讓回滾多一個維度 |

**定案：用既有的 `job_runs`。** 理由是它已經備妥了本筆需要的每一項：

* **degraded 機制已存在**——`finishRunDegraded()`（`scheduler.go`）已經會把
  `degraded && status == "success"` 改寫成 **`partial`**，`candle_gap_detection` 就是這樣用的。
  本筆不需要新的 primitive，只要在四個 scheduler 迴圈裡**接住 `Evaluate` 的 error 並餵給它**。
* **有查詢介面**——`GET /scheduler/status`（`handler/scheduler.go:149`）讀的就是這張表。
  ⚠️ **但它每個 job 只回最新一筆**（`GetLatestPerJob`，那是刻意的：取最近 N 筆再分組會讓
  早上跑過的 job 被 intraday 的 55 筆擠掉而誤報 `never_run`）。
  **所以「連續降級」不是這個端點的能力**——它只能回答「最近一輪是不是 `partial`」。
* **已經持久化且保留 30 天**——`jobRunRetentionDays = 30`（`scheduler.go:43`）。
  連續失敗**不需要維護計數器**，但**要直連 DB 手動查**，屬人工診斷而非自動告警：

  ```sql
  SELECT started_at, status, symbols_failed, error
  FROM job_runs WHERE job_name = 'intraday'
  ORDER BY id DESC LIMIT 20;
  ```

  這正是保留期從「只留當天」改成 30 天的目的（見 `scheduler.go:32-38` 的沿革）。

  ⛔ **本筆不提供自動的連續降級告警。** 想要的話是另一件事（要嘛給
  `/scheduler/status` 加歷史查詢參數，要嘛照 `architecture.md` 的模式另開表與端點），
  **不要夾帶在本筆交付**——那正是 `architecture.md`「可觀測性」那節說的
  「引入 metrics 是跨系統決策，不該夾帶在單一功能裡」。
* **不新增表 ⇒ 沒有 migration ⇒ 回滾是單純的 image 回退**（見下方回滾策略）。

##### ⚠️ `job_runs` 有循環依賴，接不住全域 DB outage

**2026-09-02 review 補上。** `job_runs` **和受影響的表在同一個 DB**，所以 DB 整個不可用時
它自己也寫不進去，而且**失敗是靜默的**：

* `startRun()`（`scheduler.go:369-375`）在 `jobRuns.Start` 失敗時**只記 Error log 並回傳
  `runID = 0`**，呼叫端拿不到錯誤，照樣往下跑。
* `Finish()`（`store/job_run_repo.go:112-119`）是 `UPDATE job_runs … WHERE id=?`。
  `id=0` 更新到**零列**，但 `ExecContext` **不回錯**（零列不是錯誤），
  於是 `finishRunDegraded` 也看不出問題。

**結果：全域 DB outage 期間，`job_runs` 可能一筆紀錄都沒有**——不是「有一筆 failed」，
是「什麼都沒有」。拿它當唯一觀測點會在最需要的時候失效。

**所以觀測要照失敗形狀分工，不能只靠一個承接點：**

| 失敗形狀 | 承接點 | 說明 |
|---|---|---|
| **單欄位／單表／單筆寫入失敗**（本筆的主要目標，含 I-101 那種型別溢位） | **`job_runs`** | DB 本身還活著，紀錄寫得進去。這是下面「具體要做的四件事」的範圍 |
| **全域 DB 不可用** | **`/health` 的 DB ping ＋ 外部健康監控** | `/health`（`api/server.go:76-84`）本來就會 `db.PingContext`，失敗時回 **503 `{"status":"down"}`**。docker／uptime 探測接的就是這個 |

⚠️ **前一版把 `/health` 描述成「只回 `{"status":"ok"}`」是不精確的**——它**會**回 503 down。
它刻意隱藏的是**錯誤細節**（DSN 等只寫 log），不是「掛了」這件事本身。
所以它不適合承載 per-symbol 的失敗計數（那是授權邊界問題），
但**完全適合承接「DB 整個不可用」**，而那正是 `job_runs` 接不住的那一格。

⛔ **不要為了補這一格去改 `startRun` / `Finish` 的錯誤處理。** 讓它們在 DB 不可用時噴錯，
只會把排程 goroutine 變成另一個故障點，而 `/health` 已經覆蓋這個形狀。
**要記的是「這兩個承接點各自的盲區」**，本節就是那份紀錄。

##### 具體要做的四件事

1. **四個 scheduler 呼叫點接住 `Evaluate` 的 error**（`scheduler.go:445` / `:501` / `:542` / `:569`）。

   ⚠️ **`job_runs` 只有一個 `symbols_failed` 欄位，所以「分開累計」不能寫成相加**
   （2026-09-02 review 修正——前一版只說「分開累計」，那會導致同一檔既抓取失敗又持久化失敗時
   **被重複計數**，甚至讓 `failed >= total` 成立而把該輪誤判成 `failed`）。定義如下：

   | 集合 | 內容 |
   |---|---|
   | `fetchFailed` | 行情抓取／回補失敗的 symbol（現有的 `failed` 來源，各路徑不同——見上方「live 實證」的對照表）。⚠️ **粒度受限，見下** |
   | `evaluateFailed` | **`Evaluate` 回 error 的所有 symbol**（硬失敗：該檔這一輪等於沒產出）。⚠️ **名稱不叫 `persistFailed`**——見下 |
   | `degraded` | **`EvaluateResult.degraded` 為真的所有 symbol**（不只「沒落盤」那一種——見下）|

   ⚠️ **`degraded` 的來源是 `EvaluateResult.degraded`，不是「有沒有落盤」單一條件**
   （2026-09-02 review 修正——前一版定義過窄，與上方「降級分類的裁決」那張表對不起來）。
   依那張表，**至少三種 stage 都算 degraded**，scheduler 要一併收集並在摘要裡分開：

   | stage 標記 | 觸發 |
   |---|---|
   | `signal_persist_failed` | `signals.Insert` 失敗 |
   | `queue_failed` | Redis READONLY backoff 或 enqueue 回錯（**Redis 停用不算**） |
   | `dedup_degraded` | 判重降級成單層或更弱：**DB 判重查詢失敗**，或 **Redis 非預期故障而改走 local reservation**。⛔ **Redis 設定為停用（`rdb == nil`）不算**——那是設定狀態，屬已知限制不是降級 |

   ⛔ **`fetchFailed` 在 live 的盤中路徑只有批次粒度，本筆不修那一段。**
   `FetchAndStoreIntradayBatch` 的逐檔 `BulkInsert` 失敗只記 log、不往上傳
   （`market/fetcher.go:90-93`），呼叫端只有整批失敗時才累計（`scheduler.go:532-537`），
   而 **live 走的正是這條路**。這個盲區已另立
   [I-103](#i-103yahoo-批次盤中路徑的逐檔寫入失敗不會回報symbols_failed-在-live-路徑只有批次粒度)
   追蹤，**不在本筆範圍**——擴充批次回傳值會把本筆的範圍推進 market 套件，是另一件事。

   **所以本筆的 union 契約限定為「scheduler 目前識別得到的失敗」**：
   `fetchFailed` 就是各路徑現有 `failed` 累計到的那些，批次路徑即批次粒度。
   **`evaluateFailed` 與 `degraded` 則是逐檔精確的**——那兩個是本筆自己新增的。

   * **`symbols_failed` 寫 `|fetchFailed ∪ evaluateFailed|`——聯集，不是相加。**

   ⚠️ **命名為什麼是 `evaluateFailed` 而不是 `persistFailed`**（2026-09-02 review 修正）：
   加入三分支錯誤契約後，`Evaluate` 的 hard error **至少有四種**——persistence error、
   **candles 不足**、**DB read 失敗**、其他。仍叫 `persist_failed` 會把
   「資料不足／讀取失敗」標成「持久化失敗」，摘要直接說謊。
   **`signal_persist_failed` 不受影響**——那是 degraded stage，指的確實是 `signals.Insert` 失敗。
   * **`degraded` 的 symbol 一律不進聯集**（那一檔評估其實成功了），
     改由 `finishRunDegraded(..., degraded=true)` 讓該輪收成 `partial`。
   * 同一檔可能同時帶多個 degraded stage，摘要要能表達（例如
     `degraded:2 (signal_persist_failed:2, queue_failed:1)`）。
   * **`error` 寫分類摘要**，保存各類的數量與一個**代表 reason code**（⛔ **不是代表錯誤**——
     原始 error 只進 log，見「錯誤訊息的外洩邊界」），而不是只留最後一個 `lastErr`——
     否則三種失敗混在一起，看到 `error` 也不知道是哪一類。
     用可辨識前綴，比照 `candle_gap_detection` 的 `upstream_data_gap:` /
     `verification_unavailable:` 慣例，例如：

     ```
     fetch_failed:3; evaluate_failed:2 (2454:numeric_overflow, 6182:insufficient_data);
     degraded:2 (signal_persist_failed:2, queue_failed:1)
     ```

     ⚠️ **括號裡是 reason code 不是錯誤原文**（2026-09-02 review 修正——前一版範例寫的是
     `2454: indicator upsert: numeric field overflow`，那是自由文字，與封閉值域的規則矛盾，
     照抄會直接把原始 driver 訊息寫進去）。
2. **indicator 持久化失敗 → 該 symbol 計入 failed**，其餘 symbol 繼續，
   由 `finishRunDegraded` 自然收成 `partial` / `failed`。
3. **收集 `EvaluateResult.degraded` 為真的所有 symbol → 走 `finishRunDegraded(..., degraded=true)`**，
   該輪收 `partial`。
   ⚠️ **不是只有「送出但沒落 DB」那一種**（2026-09-02 review 修正——前一版在這裡縮回了，
   與上方「降級分類的裁決」對不起來）：**三個 stage 都要進 degraded 集合**，
   並依 `degraded_stages` 產生分類摘要。
   ⚠️ **一律不計入 `symbols_failed`**——那些 symbol 的評估其實成功了；
   混進去會讓 `failed` 同時代表兩件事。
4. **手動 API 路徑不需要計數器**——`POST /indicators/:symbol/compute` 直接回 **503**，
   失敗當場就回給呼叫端了。

##### log 分級

* **每一輪結束時記一筆彙總 log**：帶該輪的 `fetch_failed` / `evaluate_failed` / `degraded`
  各自的數量與**代表 reason code**（同樣過分類器，log 本身可另外帶原始 error）。
  有任一類非零記 **Error**，全為零時**不額外記**
  （成功那筆本來就有 `job runs`／job 完成的既有 log）。
  ⛔ **不要寫成「進入 degraded 記 Error、恢復記 Info」**（2026-09-02 review 修正）——
  那是**轉態**語意，需要在某處保存 per-symbol／per-stage 的前一狀態，
  而本筆**沒有、也不打算**維護那份狀態（維護它等於在記憶體裡重建一個小狀態機，
  重啟即失憶，還會與 `job_runs` 形成第二個真相源）。**每輪彙總就夠了**：
  要看趨勢就查 `job_runs`，那才是持久化的那一份。
* 這樣同時解掉現況的問題——現在是**每檔每輪各記一行 `Warn`**，量大又不可操作。
* ⛔ **不要把 log 當成判定依據**——`docker logs` 在這台機器留不住（容器會被重建，
  見 [`development-workflow.md`](./development-workflow.md)「挑窗口」）。判定一律看 `job_runs`。

#### 受影響範圍

**本次不引入 Prometheus，也不新增任何表或端點**（理由見上一節）。

| 檔案 | 要改什麼 |
|---|---|
| `backend/internal/indicator/engine.go` | ①`Compute` 的 `Upsert` 失敗改回 **persistence typed error**、不再往下寫 Redis；②**拆開 `GetLatestN` 失敗與「不足 35 根」**（現況 `:33-36` 合在同一分支，且 `err == nil` 時 `%w` 包了 nil），新增 **`ErrInsufficientCandles`** sentinel |
| `backend/internal/signal/engine.go` | `Evaluate` 的回傳值改成能表達降級；不依賴 signal DB 的短期 emission dedup；**新增最小 emission 介面與可注入時鐘**（見「測試接縫」）——`redis` 欄位由具體的 `*store.RedisClient`（`:28`）改成該介面 |
| `backend/internal/scheduler/scheduler.go` | 四個 `Evaluate` 呼叫點（`:445` / `:501` / `:542` / `:569`）接住 error 並餵給 `finishRunDegraded` |
| `backend/internal/api/handler/indicator.go` | 依三分支契約映射；⛔ **不得再直接回 `err.Error()`**（現況 `:44`） |
| `backend/internal/api/handler/errors.go` | 新增 `serviceUnavailableError`（比照既有 `serverError` / `badGatewayError`：記 log、回 503 ＋ 固定安全訊息），兩條端點共用 |
| `backend/internal/api/handler/signal.go` | 回應帶上降級欄位；依同一套三分支契約映射；⛔ **不得再直接回 `err.Error()`**（現況 `:57`） |
| `backend/internal/store/redis.go` | ①`LPush` 改回 `EnqueueResult`（`enqueued`／`disabled`／`backoff`／`error`）；②新增 **`SET NX` reservation**，回 `reserved`／`already_exists`／`disabled`／`backoff`／`error`；③新增 **Lua compare-and-delete**，回 `deleted`／`not_owner`／`disabled`／`backoff`／`error`。⚠️ **三者的值域各不相同，不要共用同一個型別**。全部採「status ＋ cause」，各自補測試 |
| **錯誤分類器**（`safeJobErrorReason`） | 放在 **scheduler 套件**——它是「寫進 `job_runs` 之前」的最後一道，與 `finishRunDegraded` 同層；⛔ **不要放在 store 或 handler**，那會讓「誰負責脫敏」變得不明確 |

⛔ **`ComputeAll` / `EvaluateAll` 不在必要範圍內**——它們目前**沒有任何呼叫者**
（`backend/` 內除測試外 0 處），不是這次 live 實證經過的路徑。同樣的吞錯寫法可以順手一起修，
但不要當成主要修法。

⚠️ **兩個 handler 都要改，不只 indicator**（2026-09-02 review 補上）。
`SignalHandler.Evaluate`（`handler/signal.go:57`）目前把**所有** `Evaluate` 的 error 映射成
**422**，而 `signal.Evaluate` 的第一行就是 `indicator.Compute`——**indicator 的 persistence
typed error 會原樣從這條 API 傳出去**。只改 indicator handler 的話，同一個錯誤
在 `/indicators/:symbol/compute` 回 503、在 `/signals/:symbol/evaluate` 回 422，
對外契約自相矛盾。

**兩個 handler 的映射規則一致**——照下一節「兩條手動 API 也是外洩面」的**三分支契約**
（persistence → 503／`ErrInsufficientCandles` → 422／其餘含 DB read 失敗 → 5xx），
兩邊都要有測試釘住每一個分支。
⛔ **前一版在這裡寫「其餘 → 422」，那是錯的**，已由三分支取代。

新的 typed error 或 result type **留在相應的 domain package**，不要讓 handler 以字串比對錯誤
——兩個 handler 共用同一個 `errors.Is` 判斷，不要各寫各的。

##### ⚠️ 兩條手動 API 也是外洩面，不能只改狀態碼

**2026-09-02 review 補上。** 前一版的安全邊界只保護了 `job_runs.error`，
**漏掉 API 這條路**——而兩個 handler 現況都是**直接把 `err.Error()` 回給前端**：

* `handler/indicator.go:44`——`c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})`
* `handler/signal.go:57`——同樣寫法

**新的 persistence typed error 若包住 driver cause**，實作者只把狀態碼改成 503、
保留 `err.Error()`，**DSN、主機位址、SQL 片段照樣從 API 洩出去**——
剛建立的安全原則被整個繞過。

⚠️ **這條規則專案早就有了**：`handler/errors.go` 的 `serverError()` doc comment 明寫
「`err.Error()` 可能包含 DB 連線字串、SQL 片段、內部檔案路徑等不該外洩給客戶端的資訊，
這些細節只留在伺服器的 log 裡」。**本筆要做的是把同一條套到這兩個端點，不是發明新規則。**

##### 前置：`indicator.Compute` 現在分不出「讀失敗」與「資料不足」

⚠️ **2026-09-02 review 補上——沒有這一步，上面的三分支契約在 handler 端根本實作不出來。**

現況把兩件事併在同一個分支（`indicator/engine.go:33-36`）：

```go
candles, err := e.candles.GetLatestN(ctx, symbol, timeframe, lookback)
if err != nil || len(candles) < 35 {
    return nil, fmt.Errorf("not enough candles for %s/%s: %w", symbol, timeframe, err)
}
```

* **DB `GetLatestN` 失敗**與**真的不足 35 根**產出**同一種 error**，handler 無從分辨；
* 更糟的是 `len(candles) < 35` 但 `err == nil` 時，`%w` 包的是 **nil**——
  `errors.Is` / `errors.Unwrap` 什麼都拿不到，訊息還會出現 `%!w(<nil>)`。

**要拆成三種，且都可用 `errors.Is` 判斷：**

| 情況 | 回傳 |
|---|---|
| `GetLatestN` 回 error | **保留 cause 的 infrastructure error**（包原 error） |
| 查詢成功但 `len(candles) < 35` | **`ErrInsufficientCandles`**（indicator domain 的 sentinel，**不包 nil**） |
| `Upsert` 失敗 | **persistence typed error**（本筆新增的那個） |

**兩個 handler 的判定順序（先特殊後一般）：**

1. `errors.Is(err, ErrPersistence)` → **安全 503**（`serviceUnavailableError`）
2. `errors.Is(err, ErrInsufficientCandles)` → **安全 422**（訊息說明資料不足，不含原始文字）
3. 其餘（含 DB read 失敗）→ **`serverError` 的安全 500**

⚠️ **順序不能反**：`ErrInsufficientCandles` 若放在最前面用字串比對，DB read 失敗會被誤吃。

⚠️ **scheduler 端也吃這個分類**——`insufficient_data` 這個 reason code 就是由
`ErrInsufficientCandles` 映射出來的（見「錯誤訊息的外洩邊界」的值域表）。

**API 錯誤契約（兩條端點一致）：**

| 錯誤種類 | 狀態碼 | 回應內容 | cause |
|---|---|---|---|
| **persistence error**（本筆新增的 typed error） | **503** | **固定的安全訊息**，不含任何原始文字 | 只進 log |
| **已知 domain／資料不足**（例如 candles 不足 35 根） | **422** | 安全的說明訊息（使用者要看得懂為什麼失敗） | 視情況 |
| **其他未知錯誤，含 DB read 失敗** | **5xx**（沿用 `serverError` 的 500） | 通用訊息，不含原始文字 | 只進 log |

⛔ **不要寫成「persistence error 回 503、其餘一律 422」**——那會把 **DB read 失敗
（例如 `GetLatestN` 連不上）錯誤映射成 422**，對呼叫端等於謊稱「你的輸入有問題」。
**只有已知的 domain error 才配 422。**

**實作方式**：比照 `errors.go` 既有的 `serverError` / `badGatewayError`，
**新增一個同模式的 helper**（例如 `serviceUnavailableError(c, log, err, context)`）——
記 log、回 503 ＋ 固定訊息。兩條端點共用它，不要各寫各的。

##### 測試接縫：現在沒有，要一併設計

⚠️ **2026-09-02 review 補上。** 完成條件要求「可控的 Redis stub」「兩個獨立 Engine 共用
同一個 Redis stub」「精確控制首次 READONLY／backoff／compare-delete」「TTL 與每分鐘整掃」，
**但目前這些都做不到**——`signal.Engine.redis` 是**具體型別** `*store.RedisClient`
（`signal/engine.go:28`），從 signal 的測試注入不了 stub；repo 也沒有 miniredis 之類的替代方案。

⛔ **不把接縫寫進計畫，實作時就會臨時決定**——結果不是被迫接真 Redis，
就是塞進一個沒經過確認的測試相依。**這正是 T-061 記錄過的同一種形狀**
（verifier 是具體型別、排程層驗收補不齊）。

**要一併做的三件事：**

| # | 內容 |
|---|---|
| 1 | **signal 套件定義最小的 emission 介面**，只含本筆用到的三個操作：reservation、enqueue、compare-and-delete。⛔ **不要把整個 `RedisClient` 抽成介面**——那會把無關方法一起拖進來 |
| 2 | **`*store.RedisClient` 實作該介面**；測試用的可控 stub 也實作它，能逐次指定回哪個 outcome（含 `Err` 不變量） |
| 3 | **Engine 注入時鐘**：`now func() time.Time` 或最小 clock 介面，供 TTL、每分鐘整掃節流、以及「Redis 恢復後仍被未到期 local reservation 抑制」等時序測試使用 |

⚠️ **介面定義在 signal 套件（消費端），不是 store**——依賴反轉，讓 store 不必知道 signal
的需求。這與專案既有的 repo 介面慣例一致。

**production constructor 維持傳現有的 Redis client**，`NewEngine` 的呼叫端不必改，
**不動任何部署設定**。時鐘的預設值就是 `time.Now`。

#### 回滾與相容策略

⚠️ **本節於 2026-09-02 補上**——CLAUDE.md 對大規模／高影響異動要求「主要風險與回滾／相容
策略」，前一版只有風險沒有回滾。

##### 回滾：純 image 回退，沒有資料維度

**本筆不含 migration、不新增表、不改任何欄位型別**（可觀測性走既有的 `job_runs`），
所以回滾就是**把 backend image 換回前一版**，沒有 schema 要退、沒有資料要修。

* 回滾後**不會留下壞資料**：新版在 DB 寫入失敗時是**少寫**（indicator 不落盤、
  signal 不落盤），不是寫錯；退回舊版只是恢復「失敗照樣往下走」的舊行為。
* ⚠️ **唯一不可逆的是 outage 期間的缺列**——那與版本無關，新舊版都補不回來
  （本筆明示接受，見「風險與非範圍」）。
* 走 `/opt/stacks/scripts/stock_trading/deploy.sh`，程序見
  [`development-workflow.md`](./development-workflow.md)「schema migration 上 live 的程序」
  （本筆雖無 migration，部署窗口與復原來源的規則一樣適用）。

##### 分段上線：兩段可以拆開，而且應該拆

建議分兩次部署，讓每一段的觀察期乾淨。

⚠️ **兩段不是「互不相干」**（2026-09-02 review 修正——前一版寫「沒有互相依賴」）：
**兩段都會改到 `scheduler.go` 的同四個呼叫點**——第 1 段讓它接住 `Evaluate` 的 error，
第 2 段讓它再接住 degraded result 並餵給 `finishRunDegraded`。所以第 2 段是**疊在第 1 段
之上**的增量，不能反序、也不能只上第 2 段。可以分開部署的是**行為**，不是檔案。

| 段 | 內容 | 觀察什麼 |
|---|---|---|
| **第 1 段** | indicator fail-fast ＋ 四個 scheduler 呼叫點接住 error ＋ handler 503 | `job_runs` 是否如預期出現 `partial`；**正常日不應該有任何一輪變 partial**——若有，代表現在就有被吞掉的失敗 |
| **第 2 段** | signal degraded-success ＋ 獨立 dedup ＋ Redis enqueue 語意 | degraded 是否只在該出現時出現；`signal:queue` 沒有重複推送 |

⚠️ **第 1 段本身就是一次診斷**：它會把「目前有多少寫入失敗被吞掉」變成可見的
`partial`。上線後第一個交易日如果 `partial` 大量出現，**先別當成新版有 bug**——
比對 `indicator_snapshots` 與 `candles` 的最新 `ts` 確認是不是本來就在掉。

⛔ **不做 feature flag。** 這是「DB 掛掉時系統該表現成什麼樣子」的語意選擇，
用開關保留舊行為等於讓兩種語意同時存在，之後沒有人知道當下適用哪一種——
而**分段部署已經提供了同等的風險控制**，回滾成本也只是 image 回退。

##### 相容性：對外契約的三處變化

| 變化 | 影響 | 相容性 |
|---|---|---|
| `POST /indicators/:symbol/compute` 在**持久化失敗**時由 422 → **503** | 前端 | ✅ **安全**。`apiFetch`（`frontend/src/lib/api/client.ts:34-42`）對所有非 2xx 一律 `throw new ApiError(status, body.error)`，**除了 401 之外沒有任何狀態碼分支**；`computeIndicators` 也只是把錯誤往上拋給呼叫端顯示。使用者看到的訊息會從「candles 不足」變成落盤失敗的訊息，那正是想要的 |
| **已知 domain／資料不足**（candles 不足 35 根）**維持 422** | 前端 | ⛔ **這是硬性要求，不是選項**。`indicators.ts:35-37` 的註解明寫「candles 至少 35 根，不足時後端回 422」；把它改成 5xx 會讓「輸入不足」看起來像伺服器故障。⚠️ **但「其餘錯誤一律 422」不成立**——未知錯誤與 DB read 失敗要回 5xx，見「兩條手動 API 也是外洩面」的三分支契約 |
| signal 回應**新增**降級欄位 | API 消費者 | ✅ **純新增，向後相容**。舊消費者忽略新欄位即可。⚠️ 但要在 [`api-reference.md`](./api-reference.md) 寫明：**HTTP 200 不再等於「已寫入 signal history」**，要看 `db_persisted` |

⚠️ **`signal:queue` 的消費者**：目前**沒有任何 consumer**（全 repo 只有
`signal/engine.go:103` 一個 producer），所以 enqueue 語意的改動不會影響任何下游。
**但也因此不能把它描述成補寫 DB 的 retry 機制**（同「待決策」那節的警語）。

#### 風險與非範圍

* signal 的 degraded-success 會在 **`signals` 局部寫入失敗**時保住即時性，但那段期間的歷史
  缺口可能永久存在；前端與操作人員不能把「收到 WebSocket」解讀成「已寫入 signal history」。
  ⚠️ **不要寫成「DB outage 期間仍保住即時性」**——全域 outage 時 `Compute` 就先失敗了，
  根本走不到推播，見上方「degraded-success 涵蓋的是『局部失敗』，不是 DB outage」。
* 獨立 dedup 必須定義 Redis 不可用時的行為，且不能因程序重啟或多 instance 而宣稱具有強一致性。
* 補寫歷史若有業務需求，另立 outbox／retry task；本筆不在沒有 consumer 的現況下承諾最終一致。
* 本筆不順帶改變 signal 判斷條件、cooldown 長度或 Redis disabled／READONLY 的全域政策。

#### 完成條件

**第 1 段（indicator）**

1. Indicator `Upsert` 失敗會回 typed error、不寫 Redis、不產生 signal。
   **`POST /indicators/:symbol/compute` 與 `POST /signals/:symbol/evaluate` 兩條 API**
   照「兩條手動 API 也是外洩面」那節的三分支契約：**persistence → 503**、
   **已知 domain／資料不足（例如 candles 不足 35 根）→ 422**、
   **其他未知錯誤含 DB read 失敗 → 5xx 通用訊息**。
   ⛔ **不是「persistence 503、其餘一律 422」**——那會把 DB read 失敗謊報成輸入問題。
   **每條端點的三個分支都要有測試釘住。**
2. 四個 scheduler 呼叫路徑會接住 `Evaluate` 的 error、記錄該 symbol 的失敗、繼續其他 symbol，
   並經 `finishRunDegraded` 正確收成 `partial` / `failed`；且該計數**與行情抓取／回補的
   `failed` 分開**，`job_runs.error` 用可辨識前綴區分。

**第 2 段（signal）**

3. Signal `Insert` 失敗仍只推送一次，結果標示 degraded 且該輪 `job_runs` 收 `partial`
   （**不計入 `symbols_failed`**）；DB duplicate lookup 與 `Insert` 都失敗時，
   獨立 dedup 仍能阻止 cooldown 內重複推送。
4. Redis enqueue 的成功語意可辨識——**`LPush` 回 `nil` 不再等於「已入列」**
   （目前 disabled／READONLY backoff 都回 `nil`）；結果與 `job_runs` 看得到，
   成功路徑行為維持不變。

**兩段共同**

5. 補齊 engine、handler、scheduler 的**失敗路徑**測試（目前兩條路徑都沒有測到
   「寫 DB 失敗時會怎樣」）。⚠️ **「失敗路徑測試」太籠統，鎖不住本筆新增的並行契約**
   （2026-09-02 review 補上），**下列每一條都要有對應測試**：

   | # | 測試 | 釘住的契約 |
   |---|---|---|
   | 5-1 | canonical identity 的**量化邊界**（1e-6 附近）與**各 signal type 的選欄**（`BREAKOUT`→`Resistance`、`BREAKDOWN`／`SUPPORT_BOUNCE`→`Support`、其餘不帶價位） | DB 與 Redis 對「同一訊號」的定義只有一個 |
   | 5-2 | **Redis 不可用時兩個 goroutine 競爭**同一個 identity，**只有一個**取得 local reservation | local check-and-reserve 在同一個 critical section |
   | 5-3 | **Redis 恢復後**，仍會被**未到期的 local reservation** 抑制 | 復原當下不會重複推送 |
   | 5-3b | **Redis 已有 reservation 時（第 4 步）會回滾自己的 local token** | 跨儲存仲裁不留孤兒 reservation |
   | 5-3c | **兩個獨立的 Engine／local store 共用同一個 Redis stub**（模擬多 instance）→ **只有一個放行** | 跨 instance 仲裁由 Redis `SET NX` 負責 |
   | 5-3d | **同一個 Engine 的第二個 goroutine 在第 2 步就被抑制，且完全沒有呼叫 Redis** | local-first 的短路 |
   | 5-3e | 某 identity 過期後，**即使後續只操作其他 identity**，舊項目仍會被整掃清除 | local map 不會無界成長 |
   | 5-3f | **未過期的 reservation 在任何情況下都不會被清理掉**（含大量 identity 湧入時） | 清理不得破壞 cooldown 保證 |
   | 5-4 | compare-and-delete **不會刪掉不同 token 的新 reservation**——**Redis 端與 local map 兩邊都要斷言**（Redis 回 `not_owner` 且該 key 仍在；local 的替代 token 也未被刪） | 釋放不製造破口 |
   | 5-5 | **DB 已落盤、`queue_enqueued=false`、`broadcast_attempted=false`**（queue 為非預期失敗）→ 結果為 `degraded`、**且下一輪會被 DB 判重抑制、不自動重送** | 明示接受的那個取捨。⛔ **斷言的是這三個欄位，不是「broadcast 失敗」**——那個狀態不存在 |
   | 5-6 | **三種 degraded stage** 都會進 `job_runs` 摘要、**且不增加 `symbols_failed`** | 聯集語意與分類摘要 |
   | 5-7 | 兩條 API（indicator／signal）× 三分支：persistence → **503**、**已知 domain／資料不足** → **422**、未知含 DB read 失敗 → **5xx** | 完成條件 1 的六個方向 |
   | 5-7b | 兩條 API **各注入一個帶敏感標記的 persistence cause**，斷言**回應本文不含該標記**（標記只出現在 log） | API 也是外洩面 |
   | 5-7c | 兩條 API **各注入一個帶敏感標記的 DB read error**（`GetLatestN` 失敗）→ 回**通用 5xx**、回應本文不含該標記 | 未知錯誤不得被當成 422，也不得外洩 |
   | 5-7d | `GetLatestN` 成功但**不足 35 根** → `errors.Is(err, ErrInsufficientCandles)` 成立、API 回 **422**；⛔ 且該 error **不得包 nil cause** | 讀失敗與資料不足分得開 |
   | 5-8 | `job_runs.error` 的摘要**保留 stage、symbol 與安全的 reason code**，**但不含測試植入的 DSN／連線字串**（注入一個帶假 DSN 的 error，斷言摘要裡沒有它） | 錯誤訊息的外洩邊界 |
   | 5-8b | **注入一個分類器不認得、且帶唯一敏感字串的 error** → 輸出**只剩 `internal_error`**，該字串不出現在摘要任何位置 | 未知錯誤不得 fallback 成原文 |
   | 5-9 | **enqueue** ＋ Redis **停用** → `queue_enqueued=false`，**且不產生 `queue_failed`** | `disabled` 不是故障 |
   | 5-10 | **enqueue** ＋ Redis **READONLY／退避** → `queue_enqueued=false`，**且有 `queue_failed`**；**第一次收到 READONLY 就要如此**，不得被當成成功 | `backoff` 是故障 |
   | 5-11 | **reservation** ＋ Redis **停用** → 保留 local、**放行**，**不產生 `dedup_degraded`** | 七步流程第 5 步 |
   | 5-12 | **reservation** ＋ **首次 READONLY** 與**持續 backoff** 兩種情況 → 保留 local、**放行**，**產生 `dedup_degraded`**，且 `Err` 為 backoff sentinel | 七步流程第 6 步 |
   | 5-13 | **compare-delete** ＋ `backoff` / `error` → **釋放自己的 local token**、標記 `dedup_degraded`，Redis 端殘留交由 TTL（不重試刪除） | 釋放的降級路徑 |
   | 5-14 | **三個操作**各自的 **status／cause 不變量**：`backoff` 帶 sentinel（`errors.Is` 可判）、`error` 帶非空 cause、其餘狀態 `Err == nil` | 分類器產得出 `readonly` |

   ⚠️ **5-2 / 5-3 系列是並行／時序測試**，要用可注入的時鐘與可控的 Redis stub，
   不要靠 `time.Sleep` 賭時序。

   ⛔ **5-3c 不能寫成「同一個 Engine 的兩個 goroutine，Redis 回不同結果」**
   （2026-09-02 review 修正——前一版就是這樣寫的，它與 local-first 流程互斥）：
   同一個 local store 底下，第二個 goroutine 會在**第 2 步**就被抑制、
   **根本不會呼叫 Redis**，那個情境造不出來。要驗跨 instance 就得用
   **兩個獨立的 local store 共用一個 Redis stub**。

   實作完成後維持本筆為「已實作／待 review」。
6. Review 通過後，把分路徑一致性、API degraded contract 與操作觀測方式分別歸檔到
   [`architecture.md`](./architecture.md) 與 [`api-reference.md`](./api-reference.md)
   （後者要寫明 **HTTP 200 不再等於已寫入 signal history**），修正引用後再移除本筆。

⛔ **局部寫入失敗的驗收一律看 `job_runs` 與 DB，不看 log**——`docker logs` 在這台機器留不住。
⚠️ **但「全域 DB 不可用」不能用 `job_runs` 驗**（它自己也寫不進去，見上方循環依賴那節），
那一格的驗收看 **`/health` 回 503 `{"status":"down"}`**。**兩種形狀用兩套判準，不要混用。**

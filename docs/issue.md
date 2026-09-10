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
- **下一個新編號從 `I-113` 起算。**（**I-104 / I-105 / I-112 於 2026-09-09 收斂**——三筆的現況都歸檔在 `architecture.md`：I-104 的「**外來錯誤必須先分類，不得把原始錯誤寫進使用者可見欄位**」與**合法形式表**在「寫入失敗的一致性契約」；I-105 的「`verification_unavailable` 一定要帶得出成因」在「日 K 缺漏偵測」；I-112 的 `<stage>_failed:N (symbol:reason, …)` 在「逐檔失敗要帶得出哪一檔、哪個階段、什麼類別」。**編號都不回收。** **I-112 於 2026-09-08 發出**——`todo.md` T-070（已收斂）的 live 觀察時發現 `corporate_action_sync` 的逐檔失敗不寫進 `error`。**I-109 / I-110 / I-111 於 2026-09-07 發出、2026-09-08 全部修復並收斂**——三筆都由 `todo.md` T-071 的實作與 review 分出：I-109 是 `parseROCDate` 收下不存在的民國年（現況歸檔在 `architecture.md`「民國日期的解析是嚴格的」）；I-110 是前端 job 清單漂移（歸檔在 `development-workflow.md`「新增排程還要同步前端的 job 清單」，並由 `scripts/check-job-names.sh` 擋住）；I-111 是 SQLite 的 `busy_timeout` 保護不到 deferred transaction 的升級（歸檔在 `database-schema.md` 的 CAS 契約，改用 `BEGIN IMMEDIATE`）。**編號都不回收。** **I-108 於 2026-09-04 發出**——由 I-106 計畫書的非有限值實測分出；**I-106 / I-107 於 2026-09-03 發出**——I-107 由 I-106 的 review 分出（TR SMA(14) 與 Wilder ATR(14) 的公式分歧）；I-106 來自 T-040 regression baseline 實跑——T-040 regression baseline 實跑時發現 `atr_pct` 的窗口與註解不符、且 evaluation 與 runtime 用的是兩個不同的 ATR 演算法；**I-103 / I-104 / I-105 於 2026-09-02 發出**——I-103 由 I-102 計畫書 review 分出（Yahoo 批次路徑給不出逐檔寫入失敗）；I-104 由 I-102 實作 review 分出（其餘排程與 job 紀錄仍直接寫入原始錯誤）；I-105 來自 `2867` 跨月當天 live 首次 `partial`（`verification_unavailable` 的成因被丟棄）；I-101 / I-102 於 2026-09-01 發出——前者來自 live 的 indicator upsert 溢位、**已於同日修復並收斂**（未完成的 live 部署由 `todo.md` T-069 承接，**該筆已於 2026-09-02 部署驗收完成並收斂**），後者由它的 review 分出、**2026-09-02 實作部署完成並收斂**（現況規格歸檔在 `architecture.md`「寫入失敗的一致性契約」與 `api-reference.md` 的兩條端點，未完成的執行期觀察由 `todo.md` T-070 承接）；I-100 於 2026-09-01 發出，由 `todo.md` T-068 同日改列——**T-068 編號不回收**；**I-099 於 2026-08-31 發出後同日作廢**——誤把 `deploy.sh` 的保守預設當成與 live 的衝突，實際上該檔是範本、所有開關一律預設 `false` 是既有慣例；**編號不回收**；I-098 於 2026-08-31 由 I-096 的 review 發現分出；I-081～I-083 於 2026-08-21 發出（**I-081 / I-082 於 2026-08-27 隨 `todo.md` T-055 收斂**），I-084～I-087 於 2026-08-24 發出，I-088～I-092 於 2026-08-25 發出（**I-091 於 2026-08-28 收斂**），I-093 / I-094 於 2026-08-26 發出（I-093 已於同日收斂，**I-094 於 2026-08-28 收斂**），I-095～I-097 於 2026-08-27 發出，其中 **I-097 於同日改列 `todo.md` T-064**——編號**不回收**。）
  **發出新編號時記得把這一行一起往前推**——上一次就是漏了這步，I-089 發出去之後
  這裡還寫著「從 I-089 起算」，差一點又重用一次（I-070 已經發生過）。
  **現存條目**裡最大的是 I-108（⚠️ 本行下方的歷史索引仍看得到更大的編號，那是紀錄不是條目）。I-109～I-111 於 2026-09-08 收斂、I-104／I-105／I-112 於 2026-09-09 收斂，**下一個可用的是 I-113**；I-102 已於 2026-09-02 收斂、編號不回收——I-096 / I-098
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
  本檔現有的 I-100 / I-103 / I-106 / I-107 / I-108、已收斂的 I-102 / I-104 / I-105 / I-109～I-112、
  以及下一個可用的 I-113），
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

#### 計畫書 v22（2026-09-10，**待確認**）

⚠️ v21 的 review 再抓到 3 項（2 中 1 低），全部已反映；修訂摘要在最後一節。
**量測、儲存、目標與不做範圍承前不變**：整包約 4.9 MB 進版控放
`python/baselines/<bundle_id>/`、I-074 收斂後不刪除；不動 runtime、不改演算法、
不建通用 artifact storage、不改既有預設行為。

##### 一、三種模式與判定規則

| Stage | 指令 | 讀 DB | 跑 replay | 產出（`--output-dir` 下固定檔名） |
|---|---|---|---|---|
| **0** | `--as-of <d> --symbols … --emit-bundle <dir> --model-path … --image-digest …（後兩者由官方腳本注入）[--report-max-rows 200] [--trading-calendar <f>]` | ✅ 唯一 | ⛔ 不跑 | bundle |
| **1** | `--bundle <dir> --output-dir <dir> --before-ref <ref>`（⚠️ `--base-commit`／`--tooling-patch-sha256`／`--image-digest`／`--source-root` **由腳本內部注入，不是使用者參數**） | ❌ | ✅ 全候選 | `after_artifact.json` ＋ `cohort_manifest.json` |
| **2** | Stage 1 的參數 ＋ `--after-artifact <f> --cohort-manifest <f>` | ❌ | ✅ 全候選 | `comparison_artifact.json`（⛔ 不截斷）＋ `report.json` |

⛔ **Stage 1／2 的判定是「成對」規則**（v7 新增——v6 的允許清單讓兩個參數可以各自出現）：

```
兩個都沒給 → Stage 1
兩個都給   → Stage 2
只給其中一個 → ⛔ 中止（在跑 replay 之前）
```

⚠️ **必須在 replay 開始前就擋下**——否則跑滿約 3.7 小時才發現模式不完整。

##### 二、Stage 0 的輸入所有權（v7 新增）

⛔ **v6 的所有權清單只涵蓋 `--bundle`，Stage 0 用的是 `--emit-bundle`，等於沒有規範。**
而官方腳本可以用 `WRITE_DB=1` 注入 `--write-db`、`MODE=sweep` 注入 `--sweep`——
前者會讓 Stage 0 真的寫進 live DB。

**Stage 0 允許**：`--as-of`（**必填**）／`--symbols`（必填）／`--timeframe`／`--limit`／
`--emit-bundle`（必填）／`--report-max-rows`／`--run-id`／`--pipeline-version`／
**`--model-path`（必填）**／**`--image-digest`（必填）**／**`--trading-calendar`（選填）**。

⚠️ **後三個是 v7 漏掉的，而漏掉會讓正式指令被自己擋下**：官方腳本**無條件**在指令尾端
加 `--model-path`（`run-evaluation.sh` 的 `CMD_ARGS`），而 image digest 也定案由該腳本
inspect 後注入。封閉式清單少列它們，Stage 0 一跑就中止。

⚠️ **兩者的規則不同，⛔ 不能一視同仁**（v8 寫成「都可覆蓋」是錯的）：

| 參數 | 可否覆蓋 | 理由 |
|---|---|---|
| `--model-path` | ✅ 可 | 驗另一個模型是合法用途；**hash 的是實際載入的那個檔案**，覆蓋不影響可追溯性 |
| `--image-digest` | ⛔ **不可任意指定**（**三個 Stage 都是**） | 它是**執行環境的客觀識別值**，讓使用者填等於允許偽造 provenance |

⚠️ **驗證只能發生在 shell 層，不能指望 Python CLI**（v9 沒講清楚）：容器內的 Python
**無法自己 `docker image inspect` 得知自己跑在哪個 image**。所以定案：

* **`run-evaluation.sh`（Stage 0）與 `run-replay-offline.sh`（Stage 1／2）都
  ⛔ 拒絕使用者傳入 `--image-digest`**，再注入自己 build／resolve 後推導的值；
* **Python CLI 端**只做一件事：偵測到**重複的 `--image-digest`**（使用者一個、腳本一個）
  即中止，⛔ 不靜默採用最後一個；
* **測試**：Stage 0 與離線腳本**各一條 spoof 測試**（使用者傳入偽造 digest → 腳本拒絕）
  ＋ **各一條重複參數測試**（CLI 中止）。
**CLI 衝突測試必須用官方腳本實際組出的完整 Stage 0 argv 跑一次。**

**Stage 0 一律中止**：`--decision-replay`／`--write-db`／`--passed`／`--sweep` 及所有
sweep 相關參數／`--bundle`／`--output-dir`／`--after-artifact`／`--cohort-manifest`／
`--csv`／`--chip-json`／`--model-governance-json`／`--output`／grid 與 builder 參數。

⚠️ **所有衝突都要在 `check_connection()` 與任何擷取動作之前中止**——
Stage 0 是唯一會碰 live DB 的階段，錯誤的參數組合不該先連上去再說。

##### 三、Stage 2 的四道集合檢查（v7 補多重集合語意）

⛔ **`keys(...) == keys(...)` 不能用 set 實作**——重複列會被折疊掉，
而「before 版重複算了一列」正是要抓的錯誤之一。

**每一份資料先各自驗唯一性，再做排序後的 list 相等**：

```
對 universe / after_artifact / cohort_manifest / comparison_artifact 各驗：
    len(keys) == len(set(keys))        # ⛔ 不唯一即中止

再驗：
① sorted(keys(bundle universe))  == sorted(keys(after_artifact.rows))
② sorted(keys(before 全範圍))     == sorted(keys(after_artifact.rows))
③ sorted(cohort_manifest.keys)   == sorted(候選列的 key)   # 候選＝ rr_decoupling_candidate
④ sorted(keys(comparison_artifact.rows)) == sorted(cohort_manifest.keys)
```

`key = (symbol, timeframe, as_of)`；另驗 after artifact 的 SHA-256 與 manifest 記的相符。
⛔ Stage 2 **不得重算 predicate**——③ 依 after artifact 既有的欄位。

⚠️ **為什麼 ①② 不能省**：I-074 要「全部候選逐列資料都能重算分支」，
before 少算／多算一列時比較就不完整，而只看 after 與 manifest 完全看不到。

##### 四、`--as-of` 的「資料是否到齊」怎麼判（v7 新增定義）

⛔ **v6 只寫「`--as-of` 晚於最新資料就中止」，沒說「最新」是誰的最新。**
逐檔判會把**停牌、下市、或當天本來就沒有 K 棒的合法標的**誤判成資料未到齊。

⛔ **v7 的公式 `as_of > market_latest → 中止` 與自己的週末測試直接矛盾**——
週六 > 週五必然成立，那條測試永遠過不了。而且
[`architecture.md`](./architecture.md) 早就寫明：**實際日期集合回答不了「少了哪些天」**，
休市日要靠**交易日曆**判斷，⛔ 不能把「缺列的日子」都當成休市。

**定案：用交易日曆算出「`as_of` 當下應該要有的最後一個交易日」再比**：

```
expected_latest = max(trading_day ≤ as_of)          -- 依權威交易日曆
market_latest   = MAX(ts) over candles WHERE timeframe = :tf   -- 不分 symbol

market_latest < expected_latest → 中止（資料還沒到齊）
market_latest ≥ expected_latest → 通過
```

⚠️ **比較一律用 Asia/Taipei 的「日期」**，⛔ 不直接拿 `date` 去比 DB 的 timestamp。

**交易日曆來源**：TWSE 的 `holidaySchedule`（與 Go 端 `exchange_reference.go` 同一個來源，
整年預先公布、不會停滯）。
⛔ **取得失敗即中止**（fail-closed）——猜不得。

**HTTP request 契約（v14 補；⛔ 不能只寫「同一個來源」）**——Go 端
（`exchange_reference.go:296` 的註解與 `:334` 的實作）已經踩過並記錄了這個坑：

| 項目 | 定案 |
|---|---|
| query | **`date=<YYYY>0101&response=json`** |
| ⛔ 禁用 | **`queryYear`**——2026-08-26 實測它會被**完全忽略**：端點照樣回 **HTTP 200**、格式正常，但回的是**當年**資料 |
| 逐列驗年 | 每列日期解析後 `year == 請求年度`，⛔ 不符即中止（`exchange_reference.go:357`） |
| ⛔ 不做的事 | **不用筆數當完整性門檻**（2026 是 27 筆、2025 是 24 筆，逐年本來就不同）；只驗「非空 ＋ 年份相符」 |

⚠️ **少了逐列驗年，用錯參數名的實作會拿當年日曆去判斷去年的交易日，而且完全不會報錯**
——這正是 v13「錯誤年份測試」擋不住的情況：它驗的是回應內容，沒有驗**實際送出的 query**。
**測試**：①斷言實際送出的 query 恰為 `date=<YYYY>0101` ＋ `response=json`
（⛔ **不得出現 `queryYear`**）；②回應是他年資料 → 中止；③`data` 為空 → 中止。
**`--trading-calendar <file>` 是明確的替代入口**，⛔ **只接受一種格式：
與 bundle 內完全相同的 canonical `trading_calendar.json`**（同一個 `schema_version`、
同一套年度涵蓋與不變條件、同樣要通過下方所有驗證）。
⛔ **不接受 TWSE 原始 response、不接受單年度片段、不接受多份 response 的集合**——
那些都要求 loader 自己重跑一次正規化，就回到「無法保證與 Stage 0 得到相同答案」的問題。
**測試**：線上取得與 frozen override 產出**逐位元相同的 normalized payload**（等價性）
——⚠️ 比的是 payload 與 `bundle_id`，**manifest 的 calendar provenance 本來就會不同**（模式不同）。
**Stage 0 一律把實際使用的日曆存進 bundle（`trading_calendar.json`）**，
它是 payload 的一員（進 `content_hash8`），讓 readiness 這個判斷本身也可重現。

**日曆的涵蓋範圍與解析規則（v9 補；v8 只說「用 holidaySchedule」是不夠的）**：

* **涵蓋範圍**：必須包含 `as_of` 所在年度，**以及求得「前一個交易日」所需的前一年度**
  ——⚠️ `as_of` 落在年初時（例如 1/2），前一個交易日在去年，只抓當年會算錯；
* ⛔ **逐列分類，不是「休市日清單」**：`architecture.md` 已記錄實測至少四種列型
  （放假休市／正常交易日的標記／市場無交易僅辦結算交割／週末列），
  **全部扣除或只扣「放假」兩個方向都會錯**；
* ⛔ **解析器要移植的是 `parseCalendarDate` ＋ `newStrictDate`，不是 `parseROCDate`**
  （v9 寫錯）：holidaySchedule 的日期是 **ISO（`2026-01-01`）或 compact 民國（`1150101`）**，
  而 `parseROCDate` 吃的是 `115/01/01`——照 v9 實作會把 TWSE 的合法格式**全部拒絕**。
  嚴格語意（`1150231` 不得被正規化成 3/3）沿用 `newStrictDate`；列型判定沿用 Go 端那套。
  **測試用與 Go 端相同的 fixture**：ISO、compact 民國、閏日、不存在的日期各一條；
* **`trading_calendar.json` 的 schema（v10 定案）**：存的是**正規化後的逐日分類結果**，
  ⛔ 不是 TWSE 原始列、也不是「例外日清單」——後兩者都要求 loader 重跑一次分類，
  那就無法保證與 Stage 0 得到相同答案：

  ```json
  {"schema_version": 1,
   "source": "twse_holidaySchedule",
   "covered_years": [2025, 2026],
   "days": [{"date": "2026-01-01", "is_trading_day": false, "row_type": "holiday"}, …]}
  ```

  **精確欄位與型別（v14 定案，⛔ 範例＋不變條件不夠當 contract）**——
  ⚠️ 沒有 exact-field 規則時，**frozen 檔多一個 loader 會忽略的欄位，語意不變卻換一個
  `bundle_id`**（多的欄位仍進 payload bytes → 進 `content_hash8`）：

  | 位置 | 欄位 | 型別 |
  |---|---|---|
  | top-level | `schema_version` | `int`，必須 `== 1` |
  | top-level | `source` | `str`，必須 `== "twse_holidaySchedule"` |
  | top-level | `covered_years` | `list[int]`（嚴格升冪、不重複，見下） |
  | top-level | `days` | `list[object]` |
  | `days[]` | `date` | `str`，`YYYY-MM-DD`，⛔ 嚴格日期（`2026-02-30` 拒絕） |
  | `days[]` | `is_trading_day` | **JSON `true`／`false`**，⛔ 不接受 `0`／`1`／`"true"` |
  | `days[]` | `row_type` | `str`，五值封閉 enum |

  * ⛔ **exact-field validation：多一個未知欄位、少一個必要欄位，一律中止**（兩層都是）；
  * ⚠️ **驗 bool 要用 `isinstance(x, bool)`，⛔ 不能用 `isinstance(x, int)`**
    ——Python 的 `bool` 是 `int` 的子類，後者會讓 `1` 通過；
  * ⚠️ **整數欄位（`schema_version`、`covered_years[]`）一律用 `type(x) is int`，
    ⛔ 不用 `isinstance(x, int)`**（v15 補）——同一個子類關係反過來也成立：
    `isinstance(True, int)` 是 `True`，而且 `True == 1`，所以
    `{"schema_version": true}` 連「值必須等於 1」那道檢查都會一起通過；
    `"2026"`／`2026.0` 同樣拒絕；
  * **loader 收下 frozen 檔後要重做一次 canonical 序列化，與輸入 bytes 逐位元比對，
    不符即拒絕**——⚠️ 這條是**故意嚴格**的：語意相同但縮排、鍵序或多餘空白不同的檔案
    也會被擋，因為它就是要求「與 bundle 內那一份完全相同」；
  * **測試**：多餘欄位／缺欄位／型別錯（`"true"`、`0`、`"2026"`、
    **`schema_version: true`**、**`covered_years: [true]`**）／
    非 canonical 排版（pretty-print、鍵序不同）——**一律拒絕**。

  **manifest 的 calendar provenance 依模式分流（v13 定案）**——⛔ v12 要求
  「按年度記 `fetched_at`／`raw_row_count`」，但 frozen override 只收 normalized payload，
  裡面**根本沒有原始抓取時間與原始列數**，那個要求對 frozen 模式不可能滿足：

  | `mode` | manifest 記什麼 |
  |---|---|
  | `online` | **每個年度**各記 `fetched_at` 與 `raw_row_count` |
  | `frozen` | 記 **frozen 輸入檔的 SHA-256** 與 `loaded_at`；⚠️ 原始抓取資訊**明確標為不可得**，⛔ 不假造 |

  ⚠️ **兩種模式都只用 normalized payload 決定 `bundle_id`**——provenance 不參與 content identity。

  ⛔ **`fetched_at` 與 `raw_row_count` 一律不在這個檔案裡**（v10 誤放，v11 修正）：
  它進了 payload 就會進 `content_hash8`——**日曆內容完全相同、只是抓取時間不同，
  bundle ID 就會變**，與「同輸入重產只有 `manifest.captured_at` 不同」直接矛盾。
  兩者移到 **manifest 的 provenance 區**（不參與 content identity）。
  ⚠️ `raw_row_count` 也要移：TWSE 多一列無實質影響的原始列時，正規化後的
  `days[]` 可能完全一樣，不該因此換一個 bundle ID。

  **完整年度不變條件（v11 新增）**：

  * 每個 `covered_years` 的年度，**1/1～12/31 每一天恰好一列**（⛔ 不是「只列例外日」）；
  * 每筆 `date` 必須落在 `covered_years` 內；
  * ⛔ **`covered_years` 必須嚴格升冪、不得重複，且與 `days[].date` 的年度集合完全相等**
    （v13 新增）：canonical JSON 的 `sort_keys` **只排物件的鍵，不排陣列**——
    `[2025, 2026]` 與 `[2026, 2025]` 語意相同卻會得到**不同的 payload hash**，
    同一份日曆因此可能拿到兩個 bundle ID。**測試**：非 canonical 的 frozen 檔
    （年度倒序／重複年度／年度集合與 `days[]` 不符）**一律拒絕**；
  * `row_type` 是**封閉 enum**：`trading`（普通交易日）／`weekend`（一般週末）／
    `holiday`（放假休市）／`settlement_only`（市場無交易僅辦結算交割）／
    `trading_marked`（正常交易日的標記列，例如「開始交易日」）——
    ⛔ 未知值即中止；
  * **年度內少任何一天 → loader 中止**，⛔ 不得把「查不到」當成交易日或非交易日；
  * ⛔ **`row_type` 與 `is_trading_day` 必須一致**（v12 新增——同時存兩個能表達交易狀態的
    欄位卻沒有映射規則，`{"row_type":"holiday","is_trading_day":true}` 目前不會被擋）：

    | `row_type` | `is_trading_day` |
    |---|---|
    | `trading`／`trading_marked` | **true** |
    | `weekend`／`holiday`／`settlement_only` | **false** |

    **builder 與 loader 兩端都要驗**，矛盾組合即中止（補一條測試）。

  **loader 直接查表**得到 `is_trading_day`，⛔ 不重新分類；
  ⚠️ **查詢落在 `covered_years` 之外一律中止**；
* **一律中止的情況**：取得失敗／缺年（含查詢超出 `covered_years`）／未知列型／
  重複日期／**算不出任何 `trading_day ≤ as_of`**；
* **測試（五種中止條件一一對應）**：**取得失敗**／缺年（含查詢超出 `covered_years`）／
  未知列型／重複日期／**算不出任何 `trading_day ≤ as_of`**；
  另加跨年（`as_of` 在年初、前一交易日落在去年）、日曆檔 hash 不符，
  以及**解析器本身**的：ISO 格式、compact 民國格式、錯誤年份、空回應、不存在的日期。

**逐檔只檢查兩件事**：①該 symbol 在 bundle 的擷取範圍內**存在**；
②歷史根數足夠（≥ `min_history_bars` ＋ `forward_bars`）。
⛔ **不要求每一檔的最後一根都等於 `as_of`**——那對停牌／下市標的永遠不成立。

**測試**：`as_of` 落在週末／休市日（`expected_latest` 會退回前一個交易日，**應通過**）；
某檔的最後交易日早於 `as_of`（下市／停牌，**應通過**且該檔照常入 bundle）；
`market_latest < expected_latest`（資料還沒到齊，**應中止**）。

##### 四-B、Stage 0 的跨來源一致性快照（v15 新增）

⛔ **v14 為止，Stage 0 的四次擷取各自開一條連線**：`db.py` 的每個 helper 都是
`with engine.connect()`（`db.py:94`／`:145`／`:257` 等），`evaluation.py:2298`／`:2319`／`:2321`
也是分三次呼叫。**同步工作在擷取途中 commit，bundle 就會混進不同時間點的資料**——
一份 candles 停在 commit 前、chip／governance 已是 commit 後的組合
**在資料庫裡從未同時存在過**，而它還會被封成「可重現的證據」並拿去比對 before／after。

**定案：readiness 與三份 payload 在同一個唯讀快照內完成。**

| 步驟 | 內容 |
|---|---|
| ① 交易日曆（HTTP） | **在快照之外先做完**——⛔ 不把網路等待放進 DB 交易 |
| ② 開唯讀快照（單一 connection） | **逐 driver 的精確順序見下表**——⛔ v15 只有 PG 真的唯讀，另兩個僅一致讀取 |
| ③ readiness | `market_latest` 在**同一個交易**內查 |
| ④ 三份 payload | candles／chip／governance **全部走同一個 connection** |
| ⑤ 結束 | 取完立刻 `rollback` 結束交易（唯讀不需要 commit；⚠️ PG 的長交易會擋 vacuum） |

**逐 driver 的交易建立順序（v16 定案，⛔ 全部在第一個 SELECT 之前完成）**：

| driver | 順序 | 唯讀強制 | 收尾 |
|---|---|---|---|
| **postgres** | `execution_options(isolation_level="REPEATABLE READ")` → `BEGIN` → **`SET TRANSACTION READ ONLY`** | ✅ **交易層級**（寫入直接報錯） | `ROLLBACK`（交易結束即失效，⛔ 不需復原） |
| **mysql**（InnoDB） | `execution_options(isolation_level="REPEATABLE READ")` → **`START TRANSACTION READ ONLY`** | ✅ **交易層級** | 同上（isolation 由 SQLAlchemy 在歸還 pool 時還原） |
| **sqlite**（WAL） | **`PRAGMA query_only = 1`** → **`BEGIN DEFERRED`** | ✅ **連線層級** | `ROLLBACK` ＋ ⚠️ **`finally` 內把 `query_only` 復原成 0** |

⚠️ **sqlite 的 `query_only` 一定要復原**：那是**連線層級**的設定，而連線用完會回到 pool
——不復原的話，後續拿到同一條連線的寫入路徑會直接報錯。
⚠️ **`BEGIN DEFERRED` 不能省**：pysqlite 預設不會為純 SELECT 開交易，
每個 SELECT 會各自成為一次獨立讀取，WAL 的快照語意等於沒生效。
⚠️ **三個 engine 的快照錨點一律是「交易內第一個查詢＝readiness」**
——PG 的 REPEATABLE READ 快照建立在**第一個語句**而不是 `BEGIN`；
⛔ 不能在交易外先查 readiness 再開交易。
（mysql 亦可用 `WITH CONSISTENT SNAPSHOT` 把錨點提前到 `BEGIN`，
**本計畫不依賴它**——統一走「readiness 當第一個查詢」這條規則，三個 engine 同一套語意。）

⛔ **mysql 不能只送 `START TRANSACTION READ ONLY`**（v16 的漏洞）：`READ ONLY` 設的是
**存取模式，不是 isolation**，而「InnoDB 預設是 `REPEATABLE READ`」是**可被改掉的**
server／session 預設。落在 `READ COMMITTED` 時**每次 consistent read 都會取新快照**，
同步期間的 commit 照樣混得進來——⚠️ 而且**完全不會報錯**，
產出的 bundle 看起來一切正常。`db.py:20` 的 `create_engine()` 也沒有固定 isolation。
**定案：用 SQLAlchemy 的 `execution_options(isolation_level=…)` 明確設定**，
⛔ 不自己送 `SET SESSION TRANSACTION ISOLATION LEVEL` 再手動復原——
那是 session 層級的污染，漏復原就會跟著連線回到 pool；
SQLAlchemy 的 isolation API 會在連線歸還時還原成 dialect 預設。
⛔ **不改全域 engine 的預設**（`http_server`／`worker` 共用同一個 engine）。
**測試**：先把 session 設成 `READ COMMITTED`，再跑 Stage 0 →
必須看到它**主動切回 `REPEATABLE READ`** 才開交易。

**helper 介面**：`fetch_candles`／`fetch_chip_scores`／`fetch_sr_model_governance`
與新的 readiness 查詢各加一個 **optional `conn` 參數**，`None` 時維持現行
`engine.connect()` 行為——⛔ 不改既有呼叫端語意（`http_server` / `worker` / 既有 CLI 路徑照舊）；
`_load_db_sources`／`_load_db_replay_chip_context`／`_load_db_replay_model_governance_context`
把 `conn` 往下傳。

**⛔ 傳 `conn` 還不夠——那兩個 loader 現在會把查詢失敗轉成 warning 後繼續**（v16 補）：
`evaluation.py:2467` 與 `:2498` 是**逐 symbol** 的 `except → warnings.append → continue`，
`:2456`／`:2487` 是 import 失敗、`:2450`／`:2482` 是「dataset range missing」。
照 v15 的寫法，**某一檔的 chip 查詢在快照中途失敗，Stage 0 會照常發布一份少了那檔
context 的 bundle，只在 warnings 裡留一行字**——那正是本筆要消滅的東西。
v14「`--emit-bundle`／`--bundle` 一律 strict」只是一句概括，⛔ 沒有指定機制：

兩個 loader 各加 **`strict: bool = False`**，**Stage 0 一律 `strict=True`**，
既有呼叫端不傳即維持現行行為。**六個分支的處理方式不一樣，逐條列明**
（v17 修正——⛔ v16 寫「六個都原樣往上拋」，但其中兩條**根本沒有例外可拋**）：

| # | 位置 | 現況 | `strict=True` |
|---|---|---|---|
| 1 | `:2450` chip `dataset range missing` | warning ＋ `return {}` | ⚠️ **主動 `raise ValueError`**（無原始例外，⛔ 不能 bare `raise`） |
| 2 | `:2456` chip `from db import` 失敗 | warning ＋ `return {}` | bare `raise`（原樣） |
| 3 | `:2467` chip **逐 symbol** 查詢例外 | warning ＋ `continue` | bare `raise`（原樣） |
| 4 | `:2482` governance `dataset range missing` | warning ＋ `return {}` | ⚠️ **主動 `raise ValueError`** |
| 5 | `:2487` governance `from db import` 失敗 | warning ＋ `return {}` | bare `raise`（原樣） |
| 6 | `:2498` governance **逐 symbol** 查詢例外 | warning ＋ `continue` | bare `raise`（原樣） |

* ⚠️ **「合法零筆」與「查詢失敗」分開**：某檔本來就沒有 chip／governance 列
  → **照常入 bundle（零筆）**，⛔ 不是失敗；只有上表六個分支才中止；
* **`rollback` 放在 `finally`**，涵蓋 readiness／candles／chip／governance **任一步**失敗
  （sqlite 另含 `query_only` 復原）；
* 失敗即**非零結束碼**；**失敗後的正式目錄狀態依執行前狀態分流**（v18 修正——
  ⛔ v17 無條件寫「正式目錄不得存在」，與「重複產生」允許正式目錄原本就存在、
  驗證後 no-op 或中止、**不得覆蓋**的契約**無法同時成立**；照 v17 實作，
  失敗清理會**刪掉原本有效的基準 bundle**，而那正是 I-074 要長期保留的證據）：

  | 執行前 | 失敗後必須 |
  |---|---|
  | 正式目錄**不存在** | **仍不存在**（⛔ 不留半份可載入的產物） |
  | 正式目錄**已存在**（同 ID 既有 bundle） | **逐位元完全不變**，且**仍能通過正式 loader** |
  | 正式目錄**已存在但為空或損壞** | **同樣逐位元不變**（空的仍是空的）；⛔ 不自動刪除，fail-closed 中止並指出需人工處理 |

  ⚠️ **發布本身不會製造中間狀態**：七-B 用 `RENAME_NOREPLACE` **一次**讓正式路徑出現，
  ⛔ 全程不建立空的正式目錄——所以「失敗後仍不存在」在 rename 前的**任何**時點都成立。
  ⚠️ **上表只涵蓋 rename 之前的失敗**；rename 成功之後只剩一種例外
  （fsync 失敗 → 已發布但 durability 未確認），見七-B 的 commit point 分流表。

  ⚠️ **清理只能刪「本次執行建立的 temp」**（隨機後綴），
  ⛔ **任何情況都不得碰既有正式目錄**——包含失敗路徑與 `finally`。

**測試矩陣**：上表**六個分支各一條**（⛔ v16 只測了查詢例外，漏掉 range missing 與
import 失敗），另加 **readiness 失敗**與 **candles 中途失敗**兩條，共八條——
每一條都要斷言①**非零結束碼**、②`rollback` 被呼叫（sqlite 另驗 `query_only` 已復原）、
③**正式 bundle 目錄不存在且沒有殘留可載入的 temp**；
另加**兩組「執行前已有同 ID 有效 bundle」情境**（取 chip 逐 symbol 失敗與 readiness 失敗各一）
——斷言既有目錄**逐檔 hash 與 `manifest` 逐位元不變**且**仍能通過正式 loader**；
⚠️ **空目錄／損壞 bundle 的不變性**另由七-B 的發布測試涵蓋；
另補**對照組**：某檔 chip 為**合法零筆** → **正常產出 bundle**（⛔ 不得因此中止）。

**測試**：

* **concurrent writer**（sqlite WAL，可進常態回合）：擷取進行中由另一條連線 commit
  一批新 candles → bundle 必須**全部是 commit 前**或全部是 commit 後，⛔ 不得混合；
* **語句順序斷言（三個 driver 各一條）**：實際送出的語句與順序完全符合上表
  （PG 的 `SET TRANSACTION READ ONLY` 在 `BEGIN` 之後、第一個 SELECT 之前；
  **mysql 的 `START TRANSACTION READ ONLY`**；sqlite 的 `PRAGMA query_only=1` → `BEGIN DEFERRED`，
  以及**收尾把 `query_only` 復原**）；
* **唯讀強制**：sqlite 在快照內嘗試寫入 → 報錯（可進常態回合）；
* ⚠️ **postgres 與 mysql 的實機快照驗證列進 `development-workflow.md` 的手動步驟**——
  python 測試容器不連這兩者，⛔ 不宣稱 CI 已涵蓋；
  ⚠️ **mysql 另受 I-054 限制**（從未部署，`test-mysql-migrations.sh` 只驗 DDL 不驗 repo 層），
  所以 mysql 這一路只有**語句順序測試**是自動化的，快照行為僅止於手動驗證。

##### 五、provenance 的檔案範圍（v7 修正）

⛔ **v6 寫「依所有已 import 模組的 `__file__` 取 hash 並要求位於 `source_root`」會讓正常執行中止**
——replay 必然載入 stdlib、pandas、numpy、joblib，那些本來就在 `source_root` 外。

**分流**：

| 類別 | 處理 |
|---|---|
| **專案模組**（⚠️ **依模組名稱判定，不是依路徑**） | 先用名稱界定：`backtest.modular.sr_scoring.*` ＋ 本次執行涉及的 top-level `config`／`db` ＋ 明列的 runner／loader 模組。再驗這些模組 resolved 後的 `__file__` **必須落在唯讀 `source_root` 內**，否則中止；通過的逐檔 SHA-256 |
| **其餘（stdlib／site-packages）** | ⛔ 不做 containment 檢查、不逐檔 hash；改由 `image_digest` ＋ `python_version` ＋ `pip_freeze` 識別 |

⛔ **不能用「`__file__` 在 `source_root` 下」來定義專案模組**（v7 的寫法是循環定義）：
若真的從別的路徑 import 了一份 `backtest.modular.sr_scoring`，它會因為**路徑在 root 外**
而被歸類成第三方套件，**正好繞過那道檢查**——那正是這道檢查要抓的情況。
**runner script** 另記 `runner_sha256`（⚠️ v8 誤把這一列排在說明段落之後，
markdown 不會當成同一張表）。

其餘 provenance 承 v6：`base_commit` 來自 `git -C <worktree> rev-parse HEAD`；
`tooling_patch_sha256` 來自腳本自建 worktree 後 `git diff --binary <base_commit>` 的**實際輸出**
（⛔ 不是「傳進來那個 patch 檔的 hash」）——⚠️ **套用方式必須讓新增檔案進得了 diff**（v13 修正）：
`git diff --binary <base_commit>` **預設不含 untracked file**，而本計畫明確會新增
`replay_bundle.py` 與 `run-replay-offline.sh`，照 v12 的寫法**那兩個新檔不會進 hash**，
tooling patch 就沒有被完整涵蓋。定案：**用 `git apply --index` 套用**（新增檔直接進 index），
套用後補一次 `git add -A -N`，取完 diff 再斷言 `git status --porcelain` **沒有 `??` 行**
（還有 untracked 就代表仍有東西漏在 hash 外 → 中止）。
**測試**：patch **只新增一個檔案**時，`tooling_patch_sha256` 必須改變。`image_digest` 由 `docker image inspect` 推導，
**Stage 0 也要**（由 `run-evaluation.sh` 注入）；另記 `argv` 與非敏感 `runtime_settings`
（⛔ 不記 DSN／密碼）。
**before／after 執行模型**：兩個 git worktree ＋ **同一個 image**，皆唯讀掛載。

##### 六、bundle 規格與 canonical 規則（承 v6）

* 檔案：`manifest.json`（`schema_version: 1`）＋ `manifest.sha256` ＋ payload
  （`candles.json.gz`／`chip.json.gz`／`governance.json.gz`／`replay_config.json`／
  **`trading_calendar.json`**／`model.joblib`）；
  ⚠️ **`trading_calendar.json` 是 payload 不是附註**（v8 只說「存進 bundle 並記 hash」，
  沒放進清單）：它是 readiness 判斷的依據，不納入 `content_hash8` 的話，
  **兩份用不同日曆判定出來的 bundle 會拿到相同的 bundle ID**，
  readiness 就沒有真正綁進證據身分。它同樣要走 canonical JSON、逐檔 hash、
  loader 完整驗證，並補一條**竄改日曆要被 loader 抓到**的測試
* ⛔ 不複製 `config.yaml`，改輸出**實際生效的 replay config**；
* **排序鍵**：`trading_calendar.json` 依 `date` 升冪（⚠️ **重複日期即中止**，見下方日曆規格）／
  candles `(symbol, timestamp)`／chip `(symbol, trade_date)`／
  governance `(symbol, timeframe, as_of, created_at, model_version, model_config_hash)`，
  **三份一律以「整列 canonical JSON bytes」當最終 tie-breaker**（⛔ 不加 `id`，那會動到
  Go 注入 Python 的 row 形狀）；
* canonical JSON：`sort_keys` ／固定 separators ／`ensure_ascii=False` ／**`allow_nan=False`** ／無結尾換行；
  datetime→ISO-8601 UTC、Decimal→字串、float→最短往返 `repr`；
* canonical gzip：`mtime=0` ＋ header 不寫 filename ＋ `compresslevel=9`；
* `content_hash8` 只涵蓋 payload（⛔ 排除 `manifest.json`／`manifest.sha256`）；
  `symbols_hash8` = 去重、UTF-8 位元組序排序、`\n` 連接後 SHA-256 前 8 碼；
* **`content_hash8` 的組合演算法（v14 定案）**——⛔ 先前只說「涵蓋 payload」，
  沒定多檔如何排序、如何分隔、如何綁檔名，不同實作者會算出不同 ID：

  ```
  ① 逐檔取完整 SHA-256，對**最終落地的 bytes**（.gz 就是壓縮後的 bytes，不是解壓內容）
  ② 組成 mapping {bundle 內相對檔名: sha256 十六進位小寫}
  ③ 依**檔名的 UTF-8 位元組序**排序，以同一套 canonical JSON 規則序列化
  ④ content_hash8 = SHA-256(該 JSON 的 bytes) 的**前 8 個 hex 字元**
  ```

  ⚠️ **必須是「檔名→hash」的 mapping，不能只把各檔 hash 串起來**：後者在
  `candles.json.gz` 與 `chip.json.gz` 內容互換時會得到**相同的 hash**——
  檔名沒有綁進身分，等於少驗了「哪一份資料放在哪個角色」。

* **`bundle_id` 的完整格式（v14 定案）**：

  ```
  bundle_id = "b<manifest schema_version>_<as_of:YYYYMMDD>_<timeframe>_<symbols_hash8>_<content_hash8>"
  例：b1_20260901_1d_3f9a2c14_7b0e55d2      ⚠️ 必須完全符合 ^b[0-9]+_[0-9]{8}_[0-9a-z]+_[0-9a-f]{8}_[0-9a-f]{8}$
  ```

  ⛔ **不放 `run_id`／`pipeline_version`／`captured_at`**——前兩者是 Stage 1／2 的使用者參數
  （同一份 bundle 會被不同 run 重複消費），後者一變 ID 就變，與決定性直接衝突。
  字元集限定 `[a-z0-9_]`，可直接當目錄名。

  **loader 的三方相等契約（v15 定案）**——⛔ 只驗逐檔 hash 不夠：

  ```
  目錄 basename  ==  manifest.bundle_id  ==  由 (schema_version, as_of, timeframe,
                                              symbols, payload 逐檔 hash) 重新計算的值
  ```

  ⚠️ **任一不等即中止**。少了這條，**被改名的 bundle、或 `manifest` 的身分欄位
  （`as_of`／`timeframe`／`symbols`）被竄改的 bundle 照樣載得進來**——逐檔 hash 只證明
  「payload 沒被動過」，證明不了「這份 payload 就是這個身分宣稱的那一份」，
  而那些欄位會被 Stage 1／2 當成事實寫進 artifact。
  **測試**：目錄改名／`manifest.bundle_id` 改字／`as_of` 改／`timeframe` 改／
  `symbols` 增刪——**一律中止**。
  ⚠️ **8 hex ＝ 32 bit，本來就不是防碰撞用的**：真正的完整性由 loader 的**逐檔完整
  SHA-256**保證，而目錄同名時走的是「用正式 loader 完整驗證既有 bundle，任一不符即中止」
  （見下方「重複產生」）——所以碰撞會在載入時被抓到，⛔ 不會靜默採用另一份輸入。
* **重複產生**：staging 用隨機後綴；正式目錄已存在時（含**發布時才發現**的競爭情況）
  **用正式 loader 完整驗證**既有 bundle，通過且 payload 相同 → no-op，任一不符 → 中止；
  ⛔ 不因新的 `captured_at` 覆蓋。**判定「是否已存在」一律靠七-B 的 no-clobber rename**
  （`RENAME_NOREPLACE` 回 `EEXIST`），⛔ 不用 `exists()` 先查再發布（那有 TOCTOU）。
  ⚠️ 比對用**六份 payload 的完整 SHA-256 mapping**，⛔ 不比 manifest bytes。

##### 七、原子發布契約（承 v6）

**Stage 1／2（單檔 artifact）**：`--output-dir` 必須不存在或為空；逐檔先寫同目錄 temp
再 `os.replace`；**Stage 1 最後才發布 `cohort_manifest.json`；Stage 2 先驗
`comparison_artifact.json` 的 hash，最後才發布 `report.json`**——
「manifest／report 存在」即代表其指向的證據已完整落地。

⛔ **Stage 0 不能沿用「逐檔 `os.replace`」**（v19 修正——v18 只寫「同受本契約約束」，
但 bundle 是**多檔目錄**）：先建正式目錄再逐檔替換的話，①別的程序會在發布途中
**看到一個半成品的正式目錄**；②中途失敗時正式目錄**會存在**，與四-B 的分流表直接矛盾；
③兩個 producer 同時產同一 ID 時，「先檢查存在再發布」本身就有 TOCTOU。
**Stage 0 走整包 no-clobber 發布，流程定死在七-B**：

##### 七-B、Stage 0 的整包原子發布（v20 定案）

⛔ **v19 的 `os.mkdir` claim 不是整包原子發布**：它**先把一個空的正式目錄公開出去**，
之後才 rename 蓋掉。三個後果——①這段期間 reader 看得到空目錄；②A claim 完卡住時，
B 拿到 `FileExistsError` 後用 loader 讀那個空目錄只能**中止**，
所謂「其中一方 no-op」根本不成立；③rename 前崩潰就留下空的正式目錄，
直接違反「執行前不存在 → 失敗後仍不存在」。既然 runtime 已限定 Linux，就用**真正的
no-clobber rename**：

```
① staging：<baselines>/.staging-<random>/<bundle_id>/          ← 與正式路徑同一個 filesystem
   ⚠️ 子目錄名必須**正好是 bundle_id**——loader 的三方相等契約要驗目錄 basename
② 全部 payload、manifest、manifest.sha256 **只寫進 staging**；⛔ 正式路徑全程不得被建立
③ 逐檔 fsync ＋ fsync staging 目錄，再**用正式 loader 完整驗證 staging**
④ 發布：renameat2(AT_FDCWD, staging/<bundle_id>, AT_FDCWD, <baselines>/<bundle_id>,
                  RENAME_NOREPLACE)
      成功   → 正式路徑**一次完整出現**（⛔ 中間不存在任何可見的空目錄或半成品）
      EEXIST → 走 ⑤（本次是競爭的輸家，或本來就有同 ID bundle）
⑤ 用**正式 loader** 完整驗證既有的那一份，再比對「六份 payload 的完整 SHA-256 mapping」：
      通過且 mapping 完全相同 → **no-op**（刪掉 staging）→ 一樣走 ⑥
      任一不符或驗證失敗      → **中止**（⛔ 不覆蓋、⛔ 不刪除）
⑥ **兩條成功路徑（④ rename 成功、⑤ no-op）都要在回傳前 fsync `<baselines>`**，才算
   crash-durable ⚠️ v21 只寫「發布成功後」——重跑一份 durability 未確認的 bundle 時
   走的正是 no-op 這條，反而**不會**執行文件承諾的重新 fsync
⑦ finally：只刪本次的 .staging-<random>；⛔ 任何情況都不碰正式路徑
```

⛔ **不用 `os.rename`／`os.replace` 發布目錄**：POSIX 的 `rename()` 會**直接蓋掉既有的空目錄**。

**怎麼呼叫 `renameat2`（v21 收斂，低 3）**：

1. **優先用 libc 的 `renameat2` symbol**（`ctypes.CDLL(None, use_errno=True)`；
   glibc ≥ 2.28 有這個包裝，`python:3.11-slim` 的 Debian glibc 符合）——
   ⛔ 這樣就不必自己維護 syscall 號碼；
2. 找不到 symbol 時才退回 `syscall()`，且**限定架構 allowlist**
   （x86_64 = 316、aarch64 = 276）；⛔ **未知架構一律 fail-closed，
   絕不套用其中任一號碼**（號碼是 per-ABI 的，猜錯會呼叫到完全不同的系統呼叫）；
3. `AT_FDCWD = -100`、`RENAME_NOREPLACE = 1`；errno 一律用 `ctypes.get_errno()` 取得
   （⛔ 不看回傳值猜原因）。

⚠️ **`RENAME_NOREPLACE` 還需要檔案系統支援**，不支援時回 `EINVAL`／`ENOSYS`
（`python/baselines/` 是 bind mount，實際 fs 依 host 而定）。**啟動時先 probe**，
⚠️ **probe 要完全複製正式操作的形狀：rename 的是「目錄」，⛔ 不是檔案**（v22 修正——
檔案 rename 通過不代表目錄 rename 也通過）。在 `<baselines>` 內用隨機名稱實測兩組：

| 組 | 形狀 | 預期 |
|---|---|---|
| A | 隨機 **source 目錄** → **不存在**的 destination 目錄 | **成功** |
| B | 隨機 **source 目錄** → **已存在**的 destination 目錄 | 回 **`EEXIST`**，且 **source 與 destination 兩邊都完全不變** |

⚠️ **所有 probe 目錄在每一條路徑（含例外）都要清掉**；結果寫進執行 log
（⛔ 不寫進 manifest——那是內容身分，與環境無關）。

⛔ **probe 不通過就 fail-closed，本計畫不提供弱化的 fallback**（v21 定案）：
v20 的 `O_EXCL` 發布鎖**沒有定義正常競爭下怎麼取得鎖**——它把「鎖已存在」一律當殘留鎖
中止，於是兩個正常 producer 同時跑 fallback 時，第二個看到的是**有效鎖**卻直接中止，
「一方成功、一方 no-op」根本達不到。而且那條路的 no-clobber 只在**合作者之間**成立，
對不遵守鎖的 writer 沒有保證——**用弱化保證去發布「不可竄改的證據」是本末倒置**。
**中止訊息要指出可行的補救——⛔ 不是「產在別處再搬進來」**（v22 修正）：
`RENAME_NOREPLACE` 的能力取決於**最終 `<baselines>` 所在的 filesystem**，
而從容器 volume 搬進 bind mount 通常是**跨 filesystem**——`renameat2` 直接回 `EXDEV`，
`mv` 則**退化成 copy ＋ delete**：正式路徑又暴露半成品，no-clobber 也沒了。
正解是**讓最終的 `python/baselines/` 本身落在支援的 filesystem 上**
（搬移或重新掛載整個 repo／該目錄）再重跑。
⚠️ 要支援「從別處匯入既有 bundle」的話，得另定一套**同樣原子且 no-clobber** 的匯入流程，
**本計畫不做**。
⚠️ 同理，① 的 staging **必須與正式路徑同一個 filesystem**——不是的話 `renameat2` 回 `EXDEV`，
一律 **fail-closed**，⛔ 不得改用 copy 補上。

⚠️ **⑤ 比的是「六份 payload 的完整 SHA-256 mapping」**（v20 修正——
v19 寫「逐檔 hash 相同」會把 `manifest.json`／`manifest.sha256` 也算進去，
但那兩份**本來就允許 volatile 欄位不同**（`captured_at`、calendar 時間欄位），
於是**同一份 payload 的第二次產生會被判成不同而中止**，與「payload 相同 → no-op」矛盾）：

* 比對來源是 manifest 內、用來算 `content_hash8` 的那份 `{檔名: 完整 SHA-256}` mapping；
* ⛔ **不比 `manifest.json`／`manifest.sha256` 的 bytes**；
* 兩份 bundle **都要先通過正式 loader 的完整驗證**才進行比對；
* ⚠️ 用**完整 SHA-256** 比對，順帶把 8-hex `bundle_id` 的碰撞抓出來
  （同 ID 但 payload 不同 → mapping 不同 → 中止）。

**④ 的 rename 成功 ＝ 邏輯上的 commit point（v21 補，中 2）**——⚠️ 之後 ⑥ 的
parent-directory fsync **仍可能失敗**，而那一刻正式路徑**已經存在且 loader-valid**：
此時回一般失敗碼會違反「失敗後仍不存在」，刪掉它又違反「⛔ 不碰正式路徑」。定案：

| 時點 | 結果 |
|---|---|
| rename **之前**任一步失敗 | 一般失敗：非零、**正式路徑不存在**、staging 清掉 |
| rename **成功**、⑥ fsync 失敗 | ⚠️ **獨立狀態：「已發布且 loader-valid，但 durability 未確認」**——⛔ **不刪正式路徑**，回**與「未發布」不同的非零碼**，訊息明說 bundle 已發布 |
| **no-op**（⑤ 判定相同）、⑥ fsync 失敗 | ⚠️ 同一類狀態：**「既有 bundle 有效，但 durability 未確認」**——⛔ **完全不修改正式目錄**，回同一個專屬非零碼 |

⚠️ 這個狀態**不是**「發布失敗」，是「發布已 commit、只是還沒確認落盤」：
重跑同一條指令會走 ⑤ → mapping 相同 → **no-op**（並重新 fsync），⛔ 不需要也不應該刪除重來。

**測試**：①**兩個 producer 同時發布同一 ID**——在 ④ 的 rename **呼叫前**插入
deterministic barrier，讓兩者在同一點碰撞：**一方成功、另一方 no-op**，
⛔ 最終內容等於先寫入的那一份；②執行前已有**空的正式目錄** → **中止**且該目錄**仍是空的**；
③執行前已有**損壞的 bundle** → **中止**且該目錄**逐位元不變**；
④在 ③～④ 之間與 **④ 的 rename 呼叫前**各注入一次失敗 → 正式路徑**不存在**、staging 已清掉；
⑤**同一份 payload 重產**（`captured_at` 不同）→ **no-op**，⛔ 不得因 manifest 差異中止；
⑥**注入 ⑥ 的 fsync 失敗**——**rename 路徑與 no-op 路徑各一條**：都回專屬非零碼、
**正式 bundle 完整且可載入**（⛔ 未被刪除、no-op 路徑⛔ 完全未被修改）、
重跑該指令 → **no-op 並重新 fsync**；
⑦**probe 的 A／B 兩組各一條**（目錄形狀：不存在的 destination → 成功；
已存在的 destination → `EEXIST` 且 **source 與 destination 都不變**；probe 目錄已清掉）；⑧`RENAME_NOREPLACE` 不受支援 → **fail-closed**，
訊息含補救指引（⛔ 不得有任何弱化的發布路徑）。

##### 八、bundle 模式（Stage 1／2）的輸入所有權

**允許（使用者可給）**：`--output-dir`／`--run-id`／`--pipeline-version`／
`--after-artifact`／`--cohort-manifest`／**`--before-ref`**（要比對的 before 版本，
可以是 branch／tag／commit——**這是使用者唯一能指定的版本入口**）。

**⛔ 僅限官方腳本注入，使用者傳入即被腳本拒絕、CLI 對重複值中止**：
`--image-digest`／`--base-commit`／`--tooling-patch-sha256`／`--source-root`。

⚠️ **v10 只封閉了 `image_digest`，其餘三個仍是一般 CLI 參數**——那等於留著同一個偽造入口：
後文要求它們「由實際 worktree／diff 推導」，但只要使用者能傳同名參數，
就能寫出與實際執行不符的 provenance。定案的分工是：

| 值 | 誰決定 |
|---|---|
| **要比哪個 before 版本** | 使用者（`--before-ref`） |
| `base_commit` | **腳本**，且⛔ **順序固定**（見下方 TOCTOU 說明） |
| `tooling_patch_sha256` | **腳本**：worktree 實際 `git diff --binary <base_commit>` 的輸出 hash，且⛔ **新增檔案必須含在內**（見下方套用方式） |
| `source_root` / `image_digest` | **腳本**：實際掛載路徑與 `docker image inspect` 推導值 |

⛔ **`base_commit` 的取得順序必須固定，否則有 TOCTOU**（v11 兩處寫法互相矛盾——
一處說「從 worktree 的 HEAD 取」、一處說「在 worktree 重新解析 `<before-ref>`」）：

```
① ref → immutable OID：  oid = git rev-parse <before-ref>^{commit}
② 用該 OID 建 detached worktree（⛔ 不是用 branch 名建）
③ 從 worktree 讀 HEAD：  head = git -C <worktree> rev-parse HEAD
④ 斷言 head == oid，不符即中止
```

⚠️ **少了 ①③④ 的斷言**：branch 在 worktree 建立後被移動時，
記進 manifest 的 commit 可能已經不是實際執行的那份程式碼。

**測試**：四個欄位**各一條 spoof**（使用者傳同名參數 → 腳本拒絕）
＋ **各一條「實際值與宣稱值不一致要被抓到」**
＋ **兩條 TOCTOU**（⛔ v12 那條的預期結果與流程相反——先解析 OID 再用 OID 建
detached worktree，之後移動 branch **不可能**改變 worktree 的 HEAD）：

| 情境 | 預期 |
|---|---|
| 解析出 OID 後**移動 branch** | worktree 仍建在原 OID，`head == oid` → **應通過**（這正是先解析 OID 的目的） |
| **人為改動 detached worktree 的 HEAD** | `head != oid` → **中止** |

**中止**：`--symbols`／`--csv`／`--timeframe`／`--limit`／`--as-of`／`--model-path`／
`--chip-json`／`--model-governance-json`／`--replay-max-rows`／`--report-max-rows`／
grid 與 builder 參數／`--write-db`／`--passed`／`--output`／`--emit-bundle`／`--sweep` 系列。

⚠️ `--write-db` 要在 `check_connection()` 之前擋下。
⛔ 「有沒有明確傳入」用 `argparse.SUPPRESS` ＋ 解析後套預設值表判斷。

##### 九、其餘承前

* `replay_scope`：`--emit-bundle` 寫死 `all_candidates`；`report_max_rows` 只截斷人讀報告；
  舊路徑 `--replay-max-rows` 語意不變；Stage 2 先完整 warm-up ＋ 全範圍運算再過濾；
* **三份 artifact 各有 `schema_version` 與必要欄位**，未知版本中止；
* strict：`--emit-bundle` / `--bundle` 一律 strict，**四-B 表列的六個 fail-open 分支**
  改中止（其中兩條是主動 `raise`，見該表），⚠️「合法零筆」與「查詢失敗」分開；
* `--as-of` 邊界：台北交易日含當日 → `ts < 次日 00:00`，三 engine 各自換算，測三邊界；
* 離線：`--network none`、無 DB 環境變數、唯讀掛載、只有 `--output-dir` 可寫、
  ⛔ 不注入 `--model-path`。

##### 十、受影響檔案

`python/db.py`、`evaluation.py`、**新檔** `replay_bundle.py`、
`scripts/run-evaluation.sh`（Stage 0 掛載、透傳、image digest 注入、**與 Stage 0 禁用參數的互斥**）、
**新檔** `scripts/run-replay-offline.sh`、`docs/development-workflow.md`、`docs/sr-zone-scoring.md`。

##### 十一、失敗行為

Stage 0 或 bundle 模式併用被禁參數（含明確傳入等於預設值）／`--after-artifact` 與
`--cohort-manifest` 只給其一／**`market_latest < expected_latest`**（資料未到齊）／任何 hash 不符／
未知 `schema_version`（**受管制檔案：`manifest.json`／`trading_calendar.json`／`cohort_manifest.json`／`after_artifact.json`／`comparison_artifact.json`**——⛔ 列出檔名而不是寫數量，避免再漂移）／**strict 的六個 fail-open 分支**（四-B 表）／**四道集合檢查任一不成立或鍵不唯一**／
`--output-dir` 非空／既有同 ID bundle 驗證失敗／專案模組落在 `source_root` 外／
provenance 推導失敗／canonical 遇到 NaN／Infinity——**一律中止**。
⚠️ preflight 失敗不計入 I-074 的正式 scan。
⚠️ **唯一不屬於「中止」的非零結束**：七-B ⑥ 的 fsync 失敗（**rename 成功與 no-op 兩條路徑都算**）
——那時 bundle **已發布且可載入**，⛔ 不刪除、不重來，回專屬錯誤碼
（見七-B 的 commit point 分流表）。

##### 十二、測試與驗證策略

| 層次 | 內容 |
|---|---|
| 單元 | `fetch_candles(as_of=…)` 三邊界 × 三 engine；bundle 產生／載入／hash 不符／未知 schema／原子化／NaN 中止 |
| **決定性** | 同輸入產兩次：payload hash 與 `bundle_id` 逐位元相同；`manifest.json` **只允許 volatile 欄位不同**——⚠️ **明列**：`captured_at`；**online 模式**另有 `calendar.years[].fetched_at`／`calendar.years[].raw_row_count`；**frozen 模式**另有 `calendar.loaded_at`（⛔ `frozen_sha256` **不是** volatile，同一份檔案必須得到相同值），其餘欄位必須完全相同；涵蓋 canonical JSON、**四份 payload 各自的排序鍵**（含 `trading_calendar.json` 依 `date`）**與整列 bytes tie-breaker**、gzip 三項 |
| **語意等價** | loader 重建的 candles／chip／governance 與擷取前型別與值都相等；**日曆 loader 重建出的 `IsTradingDay` 與 Stage 0 當下的結果完全相同** |
| **重複產生** | 同 ID 第二次 no-op；既有 bundle 的 manifest 損壞要中止；殘留 `.tmp` 不影響 |
| **原子發布** | **Stage 0／1／2 各一組**：中途失敗不覆蓋既有證據、不留可載入的混合產物；**Stage 0 另驗七-B 的八條**：兩 producer 在 rename 前的 barrier 上碰撞（一方 no-op）／既有**空目錄**中止且仍為空／既有**損壞 bundle** 中止且逐位元不變／**rename 呼叫前**注入失敗則正式路徑不存在／同 payload 重產（`captured_at` 不同）仍 no-op／**fsync 失敗**（rename 與 no-op 兩條路徑各一）回專屬碼且 bundle 仍完整可載入、重跑 no-op ＋ 重新 fsync／probe 的 A／B 兩組（**目錄**形狀，`EEXIST` 時兩邊都不變，probe 目錄清掉）／`RENAME_NOREPLACE` 不受支援時 **fail-closed** |
| **模式判定** | 三種模式各一條；**只給 `--after-artifact` 或只給 `--cohort-manifest` → 在 replay 前中止**；⛔ Stage 0 不得呼叫 replay engine（spy／mock 斷言） |
| **CLI 衝突矩陣** | **Stage 0 與 bundle 模式各一組**；每個被禁參數各一條；「明確傳入等於預設值仍中止」；`--write-db` 在連 DB 前被擋 |
| **as-of readiness** | 週末／休市日（`expected_latest` 退回前一交易日 → `market_latest ≥ expected_latest`，**應通過**）；某檔最後交易日早於 `as_of`（下市／停牌，**應通過**）；**`market_latest < expected_latest`（應中止）**；**跨年**（`as_of` 在年初、前一交易日落在去年） |
| **日曆（五種中止條件一一對應）** | 取得失敗／缺年（含查詢超出 `covered_years`）／未知列型／重複日期／**算不出任何 `trading_day ≤ as_of`**；另加**年度內少一天**、日曆檔 hash 不符 |
| **日曆解析器** | ISO（`2026-01-01`）／compact 民國（`1150101`）／閏日／不存在的日期／錯誤年份／空回應 |
| **四道集合檢查** | 唯一性：universe／after／cohort／comparison **各一條重複列**；相等：before universe 缺／多列、comparison 少列、cohort 三種漂移；⚠️ before/after 的合法差異**不得**被判成輸入錯誤（正向對照組） |
| **provenance** | patch hash 來自實際 worktree diff（傳錯 patch 檔要抓得到）；**專案模組落在 `source_root` 外要中止，stdlib／site-packages 不受此限**；`--image-digest`／`--base-commit`／`--tooling-patch-sha256`／`--source-root` **各一條 spoof（腳本拒絕）＋ 各一條實際值不符（中止）**；**兩支腳本各一條重複參數測試**（CLI 中止，⛔ 不採用最後一個） |
| **結構性離線** | `--network none` 下跑完 Stage 1／2；驗無 DB 環境變數、唯讀掛載 |
| **跨日驗收** | ⛔ D 日產 bundle 並跑，**D+1 日載入同一份再跑**，輸入指紋與逐列結果相同 |

##### 十三、完成後歸檔位置

* [`sr-zone-scoring.md`](./sr-zone-scoring.md)——as-of 語意與 readiness 判準、bundle 與三份
  artifact 的 schema 與 canonical 規則、四道集合檢查、`replay_scope`／`report_max_rows`
  分工、strict、Stage 分工；
* [`development-workflow.md`](./development-workflow.md)——驗收報告要附什麼、
  `run-replay-offline.sh` 用法、before／after 的 worktree 執行模型、provenance 推導、
  原子發布契約、Stage 0 與 bundle 模式的參數互斥。

##### 修訂紀錄

| 版本 | 變更 |
|---|---|
| v1～v5 | 見前版（官方腳本離線／輸入所有權／fail-closed／as-of 邊界／manifest 規格／儲存位置／`--write-db` 連 DB／scope 退化／cohort 權威／canonical／provenance 分層／兩份產出／schema contract／排序鍵／重複產生） |
| v6 | 四道集合檢查／原子發布／no-op 完整驗證／total order tie-breaker ＋ `allow_nan=False`／patch hash 來自實際 worktree diff |
| v7 | ①Stage 0 沒有自己的輸入所有權（官方腳本可用 `WRITE_DB=1` 注入 `--write-db`）→ 新增 Stage 0 allow／deny 清單，並要求在連 DB 前中止；②Stage 1／2 缺成對參數規則 → 定義「都沒給／都給／只給一個」三種結果，且**在 replay 前**中止；③四道檢查需為多重集合 → 先驗 `len(keys)==len(set(keys))` 再比排序後 list，四份資料各補重複列測試；④`source_files_sha256` 會把 stdlib／site-packages 一起擋掉 → 分流：專案模組驗 containment ＋ 逐檔 hash，第三方套件改由 image digest／python version／pip freeze 識別；⑤`as-of` 的「最新」未定義 → 定為**該 timeframe 的市場整體最新交易日**，逐檔只驗存在與歷史根數，並補週末／停牌／下市的測試 |
| v8 | ①Stage 0 的 allowlist 漏了 `--model-path`／`--image-digest`（官方腳本**無條件注入**前者，後者也定案由它注入）→ 補進清單並定為必填、允許使用者覆蓋但一律寫進 manifest；CLI 衝突測試要用**官方腳本實際組出的 argv**；②readiness 公式與週末測試**直接矛盾**（週六 > 週五必然中止）→ 改成「依交易日曆算 `expected_latest = max(trading_day ≤ as_of)`，`market_latest < expected_latest` 才中止」，日曆用 TWSE `holidaySchedule`（與 Go 端同源）、取得失敗即中止、`--trading-calendar` 為替代入口，且**實際用的日曆存進 bundle 並記 hash**；③專案模組的判定是循環定義 → 改成**先用模組名稱界定**（`backtest.modular.sr_scoring.*` ＋ `config`／`db` ＋ runner／loader），再驗 resolved `__file__` 落在 `source_root` 內 |
| v9 | ①`trading_calendar.json` 沒進 payload 與 `content_hash8`（不同日曆會拿到相同 bundle ID）→ 納入 payload、hash、canonical 排序與 loader 驗證，補竄改測試；②`--image-digest` 不該可覆蓋（那等於允許偽造 provenance）→ 與 `--model-path` 拆開規則：前者只能由腳本推導、外部值須與實際 image 驗證一致；③日曆的涵蓋範圍與解析失敗條件未定義 → 補跨年度涵蓋、逐列分類、嚴格民國日期、五種中止條件與五條測試；④失敗條件與測試仍寫「`as_of` 晚於市場最新」→ 全部改成 `market_latest < expected_latest`；⑤`runner_sha256` 那列落在表格外 → 改成段落 |
| v10 | ①`image_digest` 的信任邊界沒閉合（Stage 1／2 仍當一般參數、且**容器內的 Python 無法自己 inspect**）→ 定案由兩支腳本各自拒絕使用者傳入再注入推導值，CLI 只負責對重複值中止，補 spoof 與重複參數測試；②日期解析器寫錯（holidaySchedule 是 ISO／compact 民國 `1150101`，`parseROCDate` 吃的是 `115/01/01`，照 v9 實作會全部拒絕）→ 改為移植 `parseCalendarDate` ＋ `newStrictDate`，並用同一組 fixture 驗；③`trading_calendar.json` 無可實作 schema → 定案存**正規化後的逐日分類結果**（含 `covered_years`），loader 直接查表、⛔ 不重新分類，超出涵蓋年度即中止；④中止條件與測試未一一對應 → 補「取得失敗」「算不出 `trading_day ≤ as_of`」及解析器的五條；⑤摘要殘留 → Stage 0 指令補必填參數、決定性測試改「四份 payload」、語意等價加日曆重建 |
| v11 | ①`fetched_at`／`raw_row_count` 放進 hashed payload 會**破壞 bundle ID 決定性**（同內容不同抓取時間就換 ID）→ 移到 manifest 的 provenance 區，日曆 payload 只留影響交易日判定的穩定內容；②`base_commit`／`tooling_patch_sha256`／`source_root` 仍可從 CLI 注入 → 使用者只能給 `--before-ref`，其餘四個欄位一律腳本推導並拒絕同名外部參數，補 spoof 與「實際值不符」測試；③日曆缺「完整年度」不變條件 → 每個 covered year 的 1/1～12/31 **每天恰好一列**、`row_type` 封閉 enum（含普通平日與一般週末）、少一天即中止；④總表未反映前文新增測試 → 補日曆五種中止、解析器六種格式、provenance 的 spoof／不符／重複參數 |
| v12 | ①`--before-ref` 有 TOCTOU 且主表與後文矛盾（主表仍列 `--base-commit`，一處說讀 worktree HEAD、一處說重新解析 ref）→ 主表改列 `--before-ref`、其餘標為腳本內部注入，並定死順序「ref→OID→用 OID 建 detached worktree→讀 HEAD→斷言相同」，補 TOCTOU 測試；②`--trading-calendar` 的輸入格式未定義 → **只接受與 bundle 內相同的 canonical `trading_calendar.json`**，⛔ 不收 TWSE 原始 response／單年度片段／多份集合，補「線上與 frozen 產出逐位元相同」等價測試；③決定性測試仍只允許 `captured_at` 不同 → 明列 volatile 欄位（`captured_at`／`calendar.fetched_at`／`calendar.raw_row_count`，多年度時按年度各記一份）；④`row_type` 與 `is_trading_day` 無一致性守門 → 定固定映射、builder 與 loader 兩端都驗、補矛盾組合測試；⑤「未知 schema version 四種檔案」數量漂移 → 改成列出五個受管制檔名 |
| v13 | ①TOCTOU 測試的預期與固定流程相反（先解析 OID 再用 OID 建 detached worktree，之後移動 branch **不可能**改變 worktree HEAD）→ 拆成兩條：解析後移動 branch **應通過**、人為改動 worktree HEAD **應中止**；②frozen 模式**產不出**規定的 calendar provenance（canonical payload 不含 `fetched_at`／`raw_row_count`）→ 改成 discriminated union：`online` 按年度記抓取時間與原始列數、`frozen` 記輸入檔 SHA-256 與載入時間並標明原始抓取資訊不可得，兩者都只用 normalized payload 決定 `bundle_id`，決定性測試的 volatile 欄位隨之分流；③`git diff --binary` 不含 untracked，會漏掉 patch 新增的 `replay_bundle.py`／`run-replay-offline.sh` → 定案用 `git apply --index` ＋ `git add -A -N`，取完 diff 斷言無 `??` 行，補「只新增檔案也要改變 hash」測試；④`covered_years` 未定義 canonical 排序與唯一性（`sort_keys` 不排陣列，`[2025,2026]` 與 `[2026,2025]` 會得到不同 hash）→ 要求嚴格升冪、不重複、且與 `days[].date` 的年度集合完全相等，補非 canonical frozen 檔的拒絕測試 |
| v14 | ①`content_hash8`／`bundle_id` 的組合演算法未定案（多檔如何排序、分隔、綁檔名都沒寫，不同實作者會算出不同 ID）→ 定成「逐檔完整 SHA-256 → `{檔名: hash}` mapping → 依檔名 UTF-8 位元組序 canonical JSON → 取前 8 hex」，並明列 `bundle_id` 的完整格式與正規表示式、說明為何不放 `run_id`／`pipeline_version`／`captured_at`、以及 8 hex 不負責防碰撞（完整性由 loader 逐檔驗）；②Python 端的 TWSE request 契約沒寫進計畫（Go 端已記錄 `queryYear` 會被忽略卻照回 200 ＋ 當年資料）→ 明訂 `date=<YYYY>0101&response=json`、⛔ 禁用 `queryYear`、逐列驗年、不用筆數當門檻，並補**斷言實際送出 query** 的測試；③frozen calendar 只有 JSON 範例與不變條件，缺精確欄位／型別／unknown-field 規則（多一個被忽略的欄位 → 語意不變卻換 bundle ID）→ 補 exact-field 表、bool 不得用 `isinstance(int)`、loader 重做 canonical 序列化並逐位元比對，補多餘／缺少／型別錯／非 canonical 排版四類測試；④標題與開頭仍寫 v12／v11 而修訂紀錄已是 v13 → 版本標示統一 |
| v15 | ①**Stage 0 沒有跨來源的一致性快照**（readiness／candles／chip／governance 各自 `engine.connect()`，同步工作中途 commit 就會封出一份「資料庫裡從未同時存在過」的組合）→ 定案「日曆 HTTP 先做完 → 開單一唯讀快照（PG `REPEATABLE READ` ＋ `READ ONLY`／InnoDB `REPEATABLE READ`／sqlite 明確 `BEGIN`）→ readiness 為交易內第一個查詢 → 三份 payload 同一 connection → rollback 結束」，helper 加 optional `conn` 且預設行為不變，補 concurrent writer 與語句斷言測試（PG 實機驗證列為手動步驟）；②`bundle_id` 未綁定目錄名與 manifest → 補「目錄 basename ＝ `manifest.bundle_id` ＝ 重新計算值」三方相等契約，補改名與身分欄位竄改測試；③整數欄位的守門缺 bool 反例（`isinstance(True, int)` 為真且 `True == 1`，連值檢查都會通過）→ 整數一律用 `type(x) is int`，補 `schema_version: true`／`covered_years: [true]` 拒絕測試 |
| v16 | ①三個 engine 的「唯讀快照」名實不符（只有 PG 真的唯讀，mysql 只有 `REPEATABLE READ`、sqlite 只有 `BEGIN`；測試也只列 PG 與 sqlite）→ 補逐 driver 的精確建立順序表（PG `BEGIN` → `SET TRANSACTION READ ONLY`；mysql `START TRANSACTION READ ONLY`；sqlite `PRAGMA query_only=1` → `BEGIN DEFERRED` ＋ **finally 復原**），統一「readiness 當交易內第一個查詢」為三 engine 的快照錨點，並補三 driver 的語句順序斷言、sqlite 的唯讀強制測試，明列 mysql 受 I-054 限制只有語句測試自動化；②快照內的查詢失敗仍會被既有 fail-open 吞掉（`evaluation.py:2467`／`:2498` 逐 symbol 轉 warning 後 `continue`，會發布一份少了 context 的 bundle）→ 兩個 loader 加 `strict` 參數、Stage 0 一律 `strict=True` 讓六個 fail-open 點原樣拋出，`rollback` 放 `finally`，失敗必須非零結束且正式目錄不存在，補四種注入失敗測試與「合法零筆仍應成功」對照組 |
| v17 | ①mysql 的快照仍靠**可被改掉的預設 isolation**（`START TRANSACTION READ ONLY` 只設存取模式；session／server 落在 `READ COMMITTED` 時每次 consistent read 都取新快照，同步期間的 commit 照樣混入且**不會報錯**，`db.py:20` 也沒固定 isolation）→ 定案先用 SQLAlchemy 的 `execution_options(isolation_level="REPEATABLE READ")` 明確設定再 `START TRANSACTION READ ONLY`，⛔ 不自己 `SET SESSION …` 手動復原（會跟著連線污染 pool）、⛔ 不改全域 engine，補「session 先設成 `READ COMMITTED`，Stage 0 必須主動切回」的測試；②strict 的分支數量與處理方式全文不一致（詳細段寫六個、他處仍寫「四處」「四種」；而 `dataset range missing` 兩條**沒有原始例外可 bare `raise`**）→ 列出六分支矩陣並標明其中兩條要**主動 `raise ValueError`**、其餘四條 bare `raise`，測試補到八條（六分支 ＋ readiness ＋ candles），全文數量統一改為「四-B 表列的六個分支」 |
| v18 | 失敗清理與既有 bundle 的保留契約**互斥**（v17 無條件要求「失敗後正式目錄不得存在」，但「重複產生」明定正式目錄可以原本就存在且**不得覆蓋**——照 v17 實作會刪掉原本有效的基準 bundle，也就是 I-074 要長期保留的證據）→ 改成依**執行前狀態**分流（原本不存在 → 仍不存在；原本已存在 → **逐位元不變且仍通過正式 loader**），明定清理只能刪本次建立的 temp、⛔ 任何路徑都不得碰既有正式目錄，失敗測試加兩組「預先放置有效同 ID bundle」情境，並把 **Stage 0 納入原子發布契約與總表**（原文只寫 Stage 1／2） |
| v19 | Stage 0 的「整包原子發布」沒真的定案（v18 只說同受契約約束，而契約寫的是**逐檔** `os.replace`；多檔 bundle 照這樣做會讓別的程序看到半成品正式目錄、失敗後正式目錄仍存在、且「先 `exists()` 再發布」有 TOCTOU）→ 新增七-B：staging 目錄放在同一 filesystem 且子目錄名正好是 `bundle_id`（滿足三方相等契約）、fsync 後先用正式 loader 驗 staging、以 **`os.mkdir` 原子 claim** 決定勝負、勝方一次 `os.rename` 整包發布、敗方改用正式 loader 驗既有那份（相同 no-op／不同中止）、`finally` 只刪本次 staging；明寫 POSIX rename 會蓋掉既有空目錄所以只對自己剛建的空目錄 rename、claim 與 rename 之間崩潰會留空目錄並在下次 fail-closed（⛔ 不自動刪正式路徑），四-B 分流表補「空／損壞」列，測試補四條 |
| v20 | ①`os.mkdir` claim **不是**整包原子發布（先把空的正式目錄公開再 rename 蓋掉：reader 看得到空目錄、輸家只能在空目錄上中止而不是 no-op、rename 前崩潰就留下空的正式目錄）→ 改用 Linux 的 **`renameat2(RENAME_NOREPLACE)`**（`ctypes` 呼叫 `syscall`），正式路徑一次完整出現、全程不建立空目錄，`EEXIST` 才驗既有那份；補檔案系統 probe 與 **`O_EXCL` 發布鎖 fallback**（明說只在所有 producer 遵守同一把鎖時成立、殘留鎖 fail-closed）、發布後 fsync `<baselines>`、以及 rename 呼叫前的 deterministic barrier 測試；②競爭輸家比「逐檔 hash」會把允許 volatile 的 `manifest.json`／`manifest.sha256` 算進去，**同一份 payload 重產會被判成不同而中止** → 改比 manifest 內那份「六份 payload 的完整 SHA-256 mapping」，⛔ 不比 manifest bytes，兩份都先通過正式 loader；完整 hash 順帶抓 8-hex `bundle_id` 碰撞 |
| v21 | ①`O_EXCL` fallback **沒定義正常競爭下怎麼取得鎖**（把「鎖已存在」一律當殘留鎖中止，兩個正常 producer 併發時第二個看到有效鎖也直接中止，達不到「一方 no-op」；且它的 no-clobber 只在合作者之間成立）→ **移除弱化 fallback**，probe 不通過即 fail-closed，訊息給出「產在支援的路徑再搬進版控」的補救；②rename 成功後 parent fsync 失敗**無處可歸**（回一般失敗違反「失敗後仍不存在」，刪掉又違反不碰正式路徑）→ 定義 **rename 成功＝commit point**，之後 fsync 失敗改回**專屬非零碼**「已發布且 loader-valid、durability 未確認」，⛔ 不刪正式路徑，重跑走 no-op ＋ 重新 fsync，四-B 與失敗行為段各補交叉說明；③raw syscall 的平台守門未閉合 → 優先用 **libc 的 `renameat2` symbol**，找不到才退回 `syscall()` 且**限定架構 allowlist**、未知架構 fail-closed，errno 用 `ctypes.get_errno()`，probe 明確驗「目的不存在→成功／已存在→`EEXIST` 且既有目的不變」並在所有路徑清掉 probe 檔 |
| **v22** | ①**no-op 分支其實不會重新 fsync**（⑤ 直接「刪 staging、正常結束」，⑥ 只寫「發布成功後」——重跑一份 durability 未確認的 bundle 走的正是 no-op 這條）→ 改成**兩條成功路徑都要在回傳前 fsync `<baselines>`**，no-op 路徑的 fsync 失敗回同一個專屬狀態「既有 bundle 有效、durability 未確認」且⛔ 完全不修改正式目錄，commit point 分流表與測試各補一列；②「先產在支援路徑再搬進版控」**不是有效補救**（跨 filesystem 時 `renameat2` 回 `EXDEV`、`mv` 退化成 copy＋delete，正式路徑又暴露半成品且失去 no-clobber）→ 補救改為「讓最終的 `python/baselines/` 本身落在支援的 filesystem（搬移或重新掛載後重跑）」，另聲明「從別處匯入」需要另一套同樣原子的流程且本計畫不做，並明訂 staging 與正式路徑不同 fs（`EXDEV`）一律 fail-closed、⛔ 不得改用 copy；③probe 形狀不對 → 改成**目錄** rename 的 A／B 兩組（不存在的 destination → 成功；已存在 → `EEXIST` 且 source 與 destination 都不變），所有 probe 目錄在每條路徑都要清掉 |

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
| 嚴重度 | 中（**live 走的正是這條路**；逐檔寫入失敗會靜默，`job_runs` 照樣 `success`。與 **I-102**（已收斂）是同一類「失敗被吞掉」，但發生在**更上游的行情抓取層**） |
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

而呼叫端 `runIntradayBatch`（`scheduler.go:580-586`）**只有在整批呼叫回 error 時**才
`failed += len(batch)`。所以「批次成功、但其中幾檔沒寫進去」這個形狀，
**`job_runs` 完全看不到**。

⚠️ **live 走的就是這條路**：`YAHOO_ENABLED=true`、`FINMIND_INTRADAY_ENABLED=false`，
`runIntradayJob` 在 `HasIntradaySource()` 為真時轉給 `runIntradayBatch`（`scheduler.go:506-513`）。
FinMind 的 `runIntradayJob` 路徑是逐檔呼叫，反而有逐檔粒度——**但那條在 live 沒有啟用**。

#### 影響

* `job_runs.symbols_failed` 在 live 的盤中路徑**只有批次粒度**，不是逐檔。
* `job_runs` 的 `fetchFailed` 集合因此在這條路徑上**無法精確**。
  該契約（`symbols_failed` ＝ `fetchFailed ∪ evaluateFailed`，且**限定為 scheduler 目前
  識別得到的失敗**）現況見 [`architecture.md`](./architecture.md)「寫入失敗的一致性契約」；
  這個盲區當初就被明確排除在該筆範圍外（原記於 `issue.md` I-102，已收斂）。

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

### I-106：`verify-regression-baseline.sh` 不是程式回歸測試，它比的是會自己變的 live 資料

| 欄位 | 內容 |
|---|---|
| 狀態 | 待執行（2026-09-03 review 定案：**改造版方向 3 ＋ 最後執行方向 1**；公式裁決另立 I-107） |
| 嚴重度 | 中（不影響 live 交易邏輯；影響的是 T-040 的驗收方法本身——**基準每隔幾個交易日就會失敗，且失敗不代表迴歸**） |
| 分類 | Python / 驗收方法 / 文件與實作不一致 |
| 建立日期 | 2026-09-03 |
| 來源 | T-040 Step 5 的 regression baseline 驗收實跑（2026-09-03 11:11–11:23） |

#### 現象

2026-09-03 拿池內 135 檔重跑 `run-evaluation.sh --limit 1500` 再比對基準，
**兩個 blocking 檢查失敗**：

| 檢查 | blocking | 結果 |
|---|---|---|
| 所有基準標的都有 profile | ✅ | 通過 |
| 波動最高者不變 | ✅ | ❌ `6243` → `2478` |
| 波動最低兩檔不變 | ✅ | 通過 |
| `atr_pct` 排名 Spearman ≥ 0.9 | ✅ | ❌ **0.8833** |
| 門檻與現行一致（觀察項） | — | 通過（兩個門檻值完全相同） |
| bucket 跨越（觀察項） | — | ⚠️ `5490` `HIGH_VOLATILITY` → `LOW_VOLATILITY` |

#### 這不是迴歸——已證明是資料滾動

**① pipeline 沒變**：`pipeline_version`（`sr_zone_evaluation_p1`）、
`source_schema_version`（`sr_zone_evaluation_p0`）、`timeframe`、兩個門檻值、
`lookback_bars`（60）、`candle_count`（1500）**全部與基準相同**。

**② 從 raw DB 完整重現了兩個日期的 `atr_pct`，小數三位全中**：

```sql
-- 14 根 true range 平均 / 最後一根 close
0050  2026-08-17 → 2.597%   2026-09-02 → 1.555%
5490  2026-08-17 → 6.375%   2026-09-02 → 2.692%
6243  2026-08-17 → 8.596%   2026-09-02 → 5.483%
```

與報告的 `atr_pct` **逐檔逐位相符**，所以數值本身沒有算錯。

**③ 根因**：`evaluation.py:287` 的 `_atr_pct(df, atr_period: int = 14)` 取的是
**`true_range.tail(14)`**，而 `_volatility_profiles` 只把 `recent = df.tail(60)` 傳進去
（`:316`、`:321`）。也就是說：

* `average_range_pct` 是**60 根**窗口 → 08-17→09-02 只掉 −0.4% ～ −8.7%
* `atr_pct` 是 **14 根**窗口 → 同期掉 **−16% ～ −57.8%**

兩者用同一批 K 棒、同一個 `lookback_bars: 60` 欄位，漂移幅度卻差一個數量級。
`verify-regression-baseline.sh` 的檔頭註解寫「`atr_pct` 取近 60 根」——**與實作不符**，
基準檔記的 `lookback_bars: 60` 也只是 `min(lookback, candle_count)`，
**不是 `atr_pct` 真正用的窗口**。

⚠️ **11 個交易日就換掉 14 根窗口裡的 11 根（79%）**，序數穩定性這個前提在這個窗口長度下
本來就不成立。基準建立於 08-17，比對於 09-02——**基準的設計壽命遠短於當初的假設**。

#### ⚠️ 前一版的兩個錯誤結論（2026-09-03 review 指出，已更正）

**錯誤 A：宣稱 evaluation 與 runtime 的 ATR 實作一致。** 不成立——**是兩個不同的指標**：

| 位置 | 演算法 | 誰在用 |
|---|---|---|
| `evaluation.py:287` `_atr_pct` | **最後 14 根 true range 的算術平均** / 最後一根 close | evaluation 報表、**`selection_report.py:119`（即凍結門檻的來源）** |
| `scoring.py:216` → `indicators.py:100` `calc_atr` | **Wilder smoothing**：seed = `mean(tr[1:15])`，再一路平滑到第 60 根 | `_adaptive_zone_builder_profile`（runtime 自適應 builder） |

**同一批 K 棒上的實測差距（2026-09-02）**：

| 標的 | TR SMA(14) | Wilder ATR(14) | 差異 |
|---|---|---|---|
| `0050` | 1.555% | 1.933% | **+24.3%** |
| `2330` | 1.722% | 2.052% | +19.2% |
| `2478` | 7.025% | 8.319% | +18.4% |
| `5490` | 2.692% | 3.832% | **+42.4%** |
| `6243` | 5.483% | 6.435% | +17.4% |

⚠️ **這是一個潛伏問題，不只是命名問題**：凍結門檻 `LOW/HIGH_VOLATILITY_THRESHOLD` 的
P33/P67 是拿 **SMA 基準**量的，而 runtime 的自適應 builder 用 **Wilder**。
Wilder 系統性高 17～42%，**一旦 `SR_SCORING_ADAPTIVE_ZONE_BUILDERS_ENABLED` 打開，
分類就會系統性偏向高波動**——門檻與 runtime 不同源。目前該旗標是 `False`，所以還沒發作。

**⛔ 這需要裁決，不是我可以逕行決定的**（三個選項互斥、都會動到已凍結的量）：

1. **統一用 Wilder ATR(14)**——與 Go `CalcATR`、與 runtime 一致；但**凍結門檻必須重測**。
2. **統一用 TR SMA(14)**——與凍結門檻、選池、既有基準一致；但 runtime 要改，
   且與 Go 端的 ATR 定義分家。
3. **承認是兩個指標**——分別命名（例如 `atr_pct_sma14` / `atr_pct_wilder14`）、
   各自記 provenance，並明確寫出哪一個在分桶、哪一個在門檻。

**錯誤 B：宣稱「pipeline 迴歸會動 `pipeline_version` 等不變量」。** 不成立——
`pipeline_version` 是 `evaluation.py:46` 的**人工常數** `DEFAULT_PIPELINE_VERSION`。
公式改壞而忘記升版時，pipeline_version / schema / 門檻 / timeframe **可以全部不變**。
把它們當成迴歸的守門是假的保證。

#### 定案（2026-09-03 review）

⛔ **不是三選一。** 定案是「**改造版方向 3 ＋ 最後執行方向 1**」，方向 2 另立議題。

| 方向 | 能解決什麼 | 主要問題 | 處置 |
|---|---|---|---|
| 1. 只重建基準 | 暫時讓驗收通過 | 幾週後可能再次失敗 | **不能單獨採用**；改造完成後作為**最後一步的 migration 動作** |
| 2. 窗口 14→60 | 降低短窗口漂移 | 改變指標與 bucket 分佈，**且沒有解決現有的公式分歧** | **本次不做**，拆為 **I-107** |
| 3. 改比對方法 | 解除 T-040 的假失敗 | 只換成 `average_range_pct` **仍不是程式回歸測試** | **改造後採用** |

⚠️ **方向 1 是 migration／初始化動作，不是問題解法**——順序必須是
「先改 schema 與比對方法 → 記錄公式／period／lookback／snapshot → 才重建一次新格式基準」。

⚠️ **方向 2 單改 period 會變成 `TR SMA(60)`，仍然不等於 runtime 的 Wilder ATR(14)。**
真正的維度有三個：**period 多少、方法是 SMA 還是 Wilder、給多少根 warm-up／lookback**。
所以它不是「14 或 60」的選擇題，見 I-107。

#### 改造內容：一份基準拆成兩層

**blocking 語意（三段式，全文以此為準）**

| | 內容 | 語意 |
|---|---|---|
| **A. 固定輸入的程式回歸** | golden `atr_pct` / `average_range_pct` / `bucket` 的**絕對值**；golden 記錄的 `calculation` 與門檻對照現況常數 | **blocking**——唯一真正偵測迴歸的一層 |
| **B-1. 相容性與完整性** | **7 項**：`schema_version`／`calculation`／`timeframe`／`source_schema_version`／`thresholds`／profile 完整性／`pipeline_version`（**完整定義見下方 B-1 表，該表是唯一 contract**） | **blocking 前置條件**——不過就不進入漂移比對 |
| **B-2. 市場漂移指標** | `average_range_pct` / `atr_pct` / `max()` 排名、波動最高／最低者、bucket 移動與分佈 | **一律只產生 warning**（見下方三行契約） |

* **只有「相同輸入、不同程式輸出」才叫程式回歸**——那是 A。
* B-2 回答的是「市場變了嗎」，不是「程式寫壞了嗎」，所以不阻擋。

⚠️ **`passed` 有兩個層級，全文一律用這三行描述 B-2，不要再寫「必須失敗」或
「`passed` 恆為 `true`」**（那兩種寫法都會被讀成整體失敗／單項不會紅）：

1. `checks[i].passed = false`
2. 該項名稱加入 `warnings`
3. **`compare()["passed"]` 不受影響**（只由 B-1 決定）

⚠️ **metadata 檢查保留為 blocking contract，但不宣稱它能單獨偵測程式迴歸**——
`pipeline_version` 是 `evaluation.py:46` 的人工常數，改壞公式而忘記升版時它不會動。

#### baseline schema（升版 p0 → p1）

```json
{
  "schema_version": "sr_volatility_baseline_p1",
  "calculation": {
    "atr_method": "tr_sma",
    "atr_period": 14,
    "profile_lookback_bars": 60,
    "average_range_period": 60,
    "bucket_basis": "max(atr_pct,average_range_pct)"
  }
}
```

⚠️ **上面只是 `calculation` 的節錄**——p1 的完整必填欄位見下方計畫書「四、contract 變化」。

* **只加 `atr_period: 14` 不夠**——兩種演算法的 period 都是 14，區分不出來。
* **舊 p0 一律明確拒絕或要求重建，不得靜默補預設值**：p0 沒記演算法，
  硬套會做出錯誤的「通過」。
* I-107 若裁決成 Wilder，再把 `atr_method` 改為 `wilder`。

#### 執行順序

1. 修正 I-106 的前提敘述（不只是文件錯，還有 SMA14／Wilder14 差異）。✅ 已完成
2. 把「程式回歸」與「live 資料漂移」拆成兩種檢查。
3. 固定輸入 regression（A）設為 blocking。
   ⚠️ B-1 是**七項**（`schema_version`／`calculation`／`timeframe`／`source_schema_version`／
   `thresholds`／profile 完整性**兩側**／`pipeline_version`），不是前段可能讀到的三項。
4. live 的 ATR／average range／max／bucket 改為 B-2 warning。
5. B-1（schema／`calculation`／完整性）保留為 blocking 前置條件，但不宣稱它能單獨偵測迴歸。
6. baseline schema 升版並記錄 method／period／lookback；
   `build_baseline` 在 `missing` 非空時 `raise`，**且驗證必須早於開檔**（否則舊基準會先被截斷）；
   寫檔改為 **`os.replace` atomic write**（基準檔是 migration artifact，不能被寫到一半的失敗截斷）。
7. **完成上述後**才執行方向 1，重建基準。
8. 重新跑 T-040：固定輸入 regression 通過、live drift 有合理說明即可驗收。

#### 測試清單

`python/tests/test_baseline_check.py` 現有 10 支全數通過，但**只驗舊的 ATR 排序邏輯**。要補：

1. 固定輸入的 regression test（golden profile 逐位比對絕對值）。
2. `average_range_pct` 排序翻轉 → 該項 `checks[i].passed=false`、進 `warnings`，**`compare()["passed"]` 不受影響**。
3. 正常資料漂移（用本次 08-17→09-02 的實際數值）→ 產生 warning，**整體 `passed` 仍為 `true`**。
4. p0 基準檔 → 明確拒絕並說明原因。
5. **參數化竄改測試**：逐一竄改 B-1 的各項（`schema_version` 降回 p0、`calculation` 五欄位各一、`timeframe` 改 `1h`、`source_schema_version` 改版、`thresholds` 改值、**抽掉一檔 profile（baseline 側與 current report 側各測一次**，見下方測試 8b）、`pipeline_version` 改版）→ **每一項都必須讓 `compare()["passed"]` 變 `false`**。
6. ⚠️ **既有的 threshold 檢查目前是 warning 語意，實作時必須改寫成 blocking**（`baseline_check.py:172-180`）。
7. **`build` CLI 的檔案安全**：`missing` 非空時不得建立新檔、不得截斷既有檔（測試 8c）；
   寫檔改為 **`os.replace` atomic write**，序列化中途失敗時舊檔不變（測試 8d）。

#### 計畫書（2026-09-03 定案；固定輸入採**版控的合成 OHLC fixture**）

##### 一、目標與不做的範圍

**要做**：把現有的一份「基準比對」拆成 **A 固定輸入的程式回歸（blocking）** 與
**B live 資料漂移觀察（B-1 前置條件 blocking、B-2 只出 warning）**，
並讓 baseline 檔記錄自己是用哪個公式算的。語意以上方「blocking 語意（三段式）」表為準。

⛔ **不做**：
* **不改任何 ATR 公式**——`_atr_pct` 的 TR SMA(14) 與 `calc_atr` 的 Wilder 都原封不動。
  公式裁決是 **I-107**，本筆只負責**如實記錄現況**（`atr_method: "tr_sma"`）。
* **不動門檻**、不升 `universe_version`、不改 `evaluation_universe` 的 135 列。
* **不動 runtime**（`scoring.py`、`pipeline.py`、adaptive builder 旗標）。
  ⚠️ `zone_builder.py` 有動到，但**只加一個模組層級的字串常數**，不碰門檻值、
  不碰 `volatility_bucket_from_profile` 的簽章與邏輯，行為完全不變。
* 不改 `run-evaluation.sh` 的取數與資源行為。

##### 二、受影響檔案與資料流

| 檔案 | 動作 |
|---|---|
| `python/backtest/modular/sr_scoring/evaluation.py` | **只新增兩個導出常數** `ATR_PERIOD = 14`、`ATR_METHOD = "tr_sma"`，並讓 `_atr_pct` 用它們（行為不變） |
| `python/backtest/modular/sr_scoring/zone_builder.py` | **只新增一個宣告式常數** `BUCKET_BASIS`，緊鄰 `volatility_bucket_from_profile`（**行為不變**，不動門檻、不動函式簽章） |
| `python/baseline_check.py` | `build_baseline` 產 p1 schema；`compare` 收斂為 B-1 blocking 前置條件 ＋ B-2 全 warning；拒絕 p0 |
| `python/tests/fixtures/volatility_regression/*.csv` | **新增**：版控的合成 OHLC |
| `python/tests/fixtures/volatility_regression/golden_profiles.json` | **新增**：golden 輸出 |
| `python/tests/test_volatility_regression.py` | **新增**：A 層，pytest |
| `python/tests/test_baseline_check.py` | 改寫既有 10 支（它們驗的是舊的 ATR 排序 blocking 語意） |
| `scripts/verify-regression-baseline.sh` | **更名**為 `scripts/observe-volatility-drift.sh` |
| `python/baselines/sr_volatility_baseline.json` | **最後一步**才重建為 p1 |

**兩層的執行位置刻意不同**（這是本計畫的核心決定）：

| 層 | 跑在哪 | 何時跑 | 資料 |
|---|---|---|---|
| **A 程式回歸** | `python/scripts/test.sh`（pytest） | **每次測試都跑** | 版控 fixture，**完全不碰 DB** |
| **B 漂移觀察** | `scripts/observe-volatility-drift.sh` | 驗收時手動 | live |

⚠️ A 層放進測試套件才是真的 blocking——放在手動腳本裡的「blocking」沒有人擋得住。

##### 三、fixture 設計

* **合成，不從 live 抽樣**。理由：live 抽樣會把 `adj_factor` 重算改寫歷史的問題帶回來，
  而那正是要脫鉤的東西（`baseline_check.py:3` 原本就把它列為「不能比絕對值」的理由之一）。
* **每檔 80 根**（> `VOLATILITY_PROFILE_LOOKBACK` 60），才驗得到 `tail(60)` 有真的切。
* 必須涵蓋：
  1. 三個 bucket 各至少一檔，且 **`max()` 基準刻意遠離門檻**（避免 I-107 若重測門檻就整片翻桶）。
  2. **一檔以跳空為主**——TR 由 `|high − prev_close|` 主導，才驗得到三項取 max
     而不是只算 `high − low`。
  3. 邊界：**只有 1 根**（`_atr_pct` 回 `None`）、`inf` / `NaN` 的四種情境（見下）。

⚠️ **fixture 不是隨便造就能讓 mutation 變紅——每個 mutation 都要有對應的構造條件。**
前一版只寫了「大小關係相反」「80 根」，**兩者都不足以保證**：

| mutation（測試 #） | fixture 必須滿足 |
|---|---|
| `tail(14)` → `tail(20)`（#2） | **倒數第 15～20 根的 TR 必須刻意與最後 14 根明顯不同**。若那 6 根與其餘同分佈，平均值只會有小數末位的差異，`rel_tol=1e-12` 之外但肉眼難辨，甚至可能剛好相同 |
| `_atr_pct` → `calc_atr`（Wilder）（#3） | ⛔ **不能拿 live 的 17～42% 當保證**——那是 live 資料的性質，與合成 fixture 沒有必然關係。要求：**fixture 的早期 TR 與最後 14 根屬於不同 regime**（Wilder 從第 15 根一路平滑，早期 regime 會殘留在結果裡；SMA 只看最後 14 根），並在**建 fixture 時實際算一次 SMA14 與 Wilder14，確認差距超出 `rel_tol=1e-12` 且肉眼可辨**，把該數字寫進測試註解 |
| `max` → `min`（#4） | **至少一檔的 `atr_pct` 與 `average_range_pct` 要落在門檻的不同側**，使 `max` 與 `min` 得到**不同的 bucket**。⛔ **只有「大小關係相反」不夠**——兩個值若同在 NORMAL 區間內，換成 `min` 仍是 `NORMAL_VOLATILITY`，測試照樣綠 |
| `tail(60)` 真的有切（切片本身） | **80 根的前 20 根要刻意放入不該參與計算的異常波動**（例如 10 倍振幅）。若前 20 根與其餘同分佈，拿掉 `tail(60)` 也算得出幾乎一樣的值 |
* golden 比對 `atr_pct` / `average_range_pct` / `bucket` / `candle_count` / `lookback_bars`，
  **浮點用 `math.isclose(rel_tol=1e-12)`**：足以抓公式改變（SMA↔Wilder 差 17～42%），
  又不會被 numpy 版本的 ULP 差異誤擋。

**`bucket` 怎麼比**（前一版寫「用 golden 記的門檻重算」，**沒說怎麼做，實作不出來**）：
`volatility_bucket_from_profile()`（`zone_builder.py:401`）**不收門檻參數**，
直接讀模組常數 `LOW/HIGH_VOLATILITY_THRESHOLD`。於是有兩條路，都不能單獨走：

| 做法 | 問題 |
|---|---|
| 測試自己重寫一份 bucket 判定 | 正式函式的 `max` → `min` 改壞時**抓不到**（測試比的是自己那份） |
| 直接呼叫正式函式、用現行常數 | I-107 一重測門檻，A 層整批變紅——**那不是程式迴歸** |

✅ **定案**：**用 `monkeypatch` 把 `zone_builder` 的兩個門檻常數暫時換成 golden 記的值，
再呼叫正式的 `volatility_bucket_from_profile()`。** 兩個問題同時解決——
走的是正式函式（mutation 抓得到），用的是固定門檻（不受 I-107 影響）。
* golden 檔**必須明確保存這兩個門檻值**（`thresholds.low_volatility_max` /
  `high_volatility_min`），否則 monkeypatch 沒有來源。
* ⚠️ `volatility_bucket_from_profile` 的邊界是 `basis < LOW` → LOW、`basis > HIGH` → HIGH，
  **等於門檻時是 NORMAL**。fixture 的 `max()` 基準要遠離這兩個值，別去測邊界語意。

**非有限值 fixture 的預期輸出**（前兩版都寫錯，**以下是 2026-09-04 的實測結果**）：

前一版把不同的非有限值混成「髒列」一概而論，**不成立**——`NaN` 與 `inf` 的路徑完全不同，
而且**位置**與**是整列還是單欄**也會改變結果。實測（30 根合成資料，baseline
`atr_pct = 0.019436345966958212`）：

| # | 情境 | `atr_pct` 實測 | 為什麼 |
|---|---|---|---|
| 1 | `high=NaN`，在最後 14 根**內** | **0.018672775232542**（有限，但**與 baseline 不同**） | 三個候選中 `\|low − prev_close\|` 仍有限，`max(skipna=True)` 取它，**該列沒有被丟掉**，只是 TR 變小 |
| 2 | `high=NaN`，在最後 14 根**外** | 與 baseline 相同 | 不在 `tail(14)` 內 |
| 3 | **整列 NaN**，在最後 14 根內 | 與 baseline 相同 | 三個候選全 NaN → `max` 回 NaN → `.dropna()` **真的丟掉該列**，`tail(14)` 因此往前多取一根 |
| 4 | TR 候選 `+inf`（`high=inf`），在最後 14 根內 | **`None`** | `inf` **不會被 `.dropna()` 移除**，`mean` 為 `inf`，最後由 `_clean_metric` 轉成 `None` |
| 5 | TR 候選 `+inf`，在最後 14 根外 | 與 baseline 相同 | |
| 6 | `last_close = NaN` | **`None`** | `atr / nan = nan` |
| 7 | **`last_close = +inf`** | **`0.0`**（**不是 `None`**） | `inf <= 0` 為 `False` 通過守門，`atr / inf = 0.0`，而 `0.0` 是有限值 |
| 8 | `last_close = 0` | `None` | 被 `last_close <= 0` 擋掉 |
| 9 | **整段 `close` 都是 `+inf`** | **`None`**（`atr_pct` 正確） | `prev_close` 也是 `inf` → TR 為 `inf` → `inf / inf = nan` → `None`。⚠️ **但同一檔的 `average_range_pct` 是 `0.0`，bucket 因此變 `LOW_VOLATILITY`**——見下 |

`average_range_pct` 走的是另一條清理路徑（`.replace([inf,-inf], nan).dropna().mean()`），
實測 `high=NaN` 與 `close=0`（振幅變 `inf`）**都得到同一個有限值** `0.019722593362700876`
——兩種情況都只是把該列丟掉。

⛔ **但那條清理路徑擋不住 `close = +inf`**：`(high - low) / inf = 0.0` 是**有限值**，
`.replace([inf,-inf], nan)` 清不到它。實測情境 9：`atr_pct = None`（正確）、
**`average_range_pct = 0.0`**、`volatility_bucket_from_profile(None, 0.0)` →
`values = [0.0]` → `max` = `0.0` < `LOW` → **`LOW_VOLATILITY`**。
這是整組情境裡**唯一真的把壞資料判成「最穩」的一條**，且入口是 `average_range_pct`
而不是 `_atr_pct`。已併入 [I-108](#i-108volatility-profile-沒有完整拒絕非有限的-close會產出看似合法的-00) 的修正範圍。

⚠️ **情境 9 的 golden 同樣是「I-108 修正前的觀察值」**：I-108 定案採逐列語意後，
這一檔會變成 `average_range_pct = None`、bucket = `UNKNOWN_VOLATILITY`。
本筆照現況記 `0.0` / `LOW_VOLATILITY` 並在 golden 與測試註解標明，
**由 I-108 在自己的 commit 內一併更新**（順序：I-106 先、I-108 後）。

✅ **fixture 依情境 1／3／4／6／7／9 各給一檔**（**6 種**），不再用單一「髒列」概括，
也不再一律期待 `null`。**情境 9 是唯一需要同時釘住 `average_range_pct` 與 `bucket` 的**，
其餘只釘 `atr_pct`。

⛔ **上表的數字是診斷證據，不是 golden 值。** 它們來自 30 根的合成資料；正式 fixture
要求每檔 80 根且**含 regime 變化**，算出來的有限值必然不同，而「只有 1 根」那個
edge case 更不可能有 80 根。**fixture 長度依用途分**：

| 用途 | 根數 |
|---|---|
| mutation／一般 profile fixture | **80**（含前 20 根的異常 regime） |
| 非有限值 edge fixture | 依案例，**1／30／80 皆可** |

**正式 golden 一律由版控 fixture 重新算出並獨立核對**（手算驗一檔，見風險表），
**不得直接抄上表的數字**。上表只用來確定「哪些情境會落在哪一種輸出型態」
（有限值／`None`／`0.0`）。

⚠️ **情境 7 是既有的健壯性缺口，已另立 I-108**：`_atr_pct` 只守 `last_close <= 0`，
沒守 `math.isfinite`，於是 `+inf` 產出 `0.0`——一個看似合法、實際是垃圾的數字。
⛔ **不要寫成「因此會被分類成 `LOW_VOLATILITY`」**：bucket 用的是
`max(atr_pct, average_range_pct)`，`0.0` 通常被另一個分量蓋過去，
實測三種波動 regime 的 bucket **都沒有改變**（見 I-108 的表）。
所以 golden 要釘住的是 **`atr_pct = 0.0` 這個欄位值**，不是 bucket。
**本筆不修**（範圍是「行為不變」），golden 如實記錄 `0.0`，
但**必須在 golden 檔與測試註解標明這是「I-108 修正前的觀察值」，不是認可的長期正確行為**；
I-108 修正時要同步更新 golden。

⛔ **不要說它「只是理論情境」**——前一版這樣寫是錯的。`candles.close` 確實是
`numeric(10,2) NOT NULL` 取不到 `inf`，但 **evaluation 有 `--csv` 輸入路徑**
（`evaluation.py:2508` → `_load_csv_sources` → `load_ohlcv_csv`），CSV 可以載入 `inf`。

⚠️ **產生 golden 時要用 `json.dump(..., allow_nan=False)`**——Python 預設會寫出
非標準的 `NaN` / `Infinity` 字面值，那種檔案別的 JSON parser 讀不了，
而且會讓「漏轉 `None`」這個 bug 靜靜地被寫進 golden 當成正確答案。

##### 四、contract 變化

**baseline schema `p0` → `p1`**：

```json
{
  "schema_version": "sr_volatility_baseline_p1",
  "source_schema_version": "sr_zone_evaluation_p0",
  "pipeline_version": "sr_zone_evaluation_p1",
  "timeframe": "1d",
  "thresholds": {
    "low_volatility_max": 0.046089927430152715,
    "high_volatility_min": 0.06278197721225691
  },
  "calculation": {
    "atr_method": "tr_sma",
    "atr_period": 14,
    "profile_lookback_bars": 60,
    "average_range_period": 60,
    "bucket_basis": "max(atr_pct,average_range_pct)"
  },
  "snapshot": { "...": "產生當時的資料狀態，維持 p0 既有語意" },
  "symbols": ["..."], "missing": [], "profiles": { "...": {} }
}
```

⚠️ **這是完整的必填欄位，不是節錄**——前四個欄位 p0 就已經在存了
（`baseline_check.py:91-95`），只是 `compare` 從來沒讀。p1 把它們全部升為 B-1 blocking。

**五個欄位的來源不一樣，前一版說「全部由現況常數讀入」是錯的**：

| 欄位 | 來源 | 是否從實作推導 |
|---|---|---|
| `atr_method` | 新增 `evaluation.ATR_METHOD` | ⚠️ **宣告式**——常數與 `_atr_pct` 的實作**綁在一起改**，靠 mutation test 約束 |
| `atr_period` | 新增 `evaluation.ATR_PERIOD`，**`_atr_pct` 的預設參數改用它** | ✅ 真的從實作讀 |
| `profile_lookback_bars` | `VOLATILITY_PROFILE_LOOKBACK` | ✅ |
| `average_range_period` | `VOLATILITY_PROFILE_LOOKBACK`（**與上一欄同源**，`average_range_pct` 就是在同一個 `tail(lookback)` 切片上算的） | ✅ |
| `bucket_basis` | 新增 `zone_builder.BUCKET_BASIS = "max(atr_pct,average_range_pct)"` | ⚠️ **宣告式，不是自動推導** |

⚠️ **`bucket_basis` 與 `atr_method` 是宣告式 contract，不能假裝它們是從實作推導出來的。**
`volatility_bucket_from_profile` 裡的 `max(values)`（`zone_builder.py:405`）沒有任何
可供 metadata 讀取的 contract 常數，字串「`max(...)`」無論放哪裡都是人寫的。
處置是**兩件事一起做**：

1. **常數放在被描述的程式碼旁邊**（`BUCKET_BASIS` 緊鄰 `volatility_bucket_from_profile`、
   `ATR_METHOD` 緊鄰 `_atr_pct`），改實作時看得到要一起改；
2. **由 mutation test 保證一致**——`max` → `min` 必須讓 A 層變紅（測試 #4）。

⛔ **不要在 `baseline_check.py` 裡手寫 `"max(...)"`**——那正是本筆要修的
「metadata 與實作分離」再犯一次。

**p0 一律拒絕**：`compare` 見到非 p1 直接 blocking 失敗並說明要重建，
**不靜默補預設值**（p0 沒記演算法，補預設會做出錯誤的「通過」）。

⛔ **`build_baseline` 遇到 `missing` 非空時改為直接失敗**（現行是寫檔後印 WARNING、
仍回傳 0——`baseline_check.py:243-245`）。一份先天就缺標的的基準，之後每次比對都不會
發現那一檔不見了——B-1 的 6a 是在補這個洞的下游，這裡則是堵住源頭。
**寧可當場不產出，也不要產出一份看起來正常的殘缺基準。**

⛔ **「不產出」必須包含檔案安全，光是回傳非零不夠。** 現行 CLI 的順序是
`build_baseline()` → **`open(args.output, "w")` 覆寫** → 才檢查 `missing`
（`baseline_check.py:230-241`）——`open(..., "w")` 一執行就已經把舊檔截斷成 0 bytes。
若只在寫檔後加 `return 1`，結果是「回了非零，但舊基準已經沒了」，比現在更糟。

**定案：驗證放在 `build_baseline()` 內部，`missing` 非空時 `raise ValueError`**
（訊息帶上缺漏清單）。選這個而不是「CLI 在 open 前先檢查」的理由：純函式自己守住不變量，
之後任何呼叫端（測試、其他腳本）都拿不到殘缺結果，不必各自記得複製那段檢查。
CLI 只負責把例外轉成訊息與非零 exit code。四條驗收條件：

1. `missing` 的驗證在**開啟 `args.output` 之前**完成（由 raise 的位置自然保證）；
2. exit code 非零；
3. **新檔不得被建立**；
4. **`args.output` 已存在時，內容必須與執行前逐位元組相同**（不得截斷）。

ℹ️ 這樣改之後，**新 `build` 產出的檔案 `missing` 恆為 `[]`**。
`missing` 欄位仍留在 p1 schema，B-1 的 6a 也仍要檢查它——那是為了擋
手改過的、或由舊版／其他來源產生的基準檔，不是為了擋新 `build` 的輸出。

⛔ **`missing` 的 raise 只擋住了「已知的失敗」，寫檔本身仍不安全。**
`open(args.output, "w")` 一旦執行，之後任何錯誤都會留下被截斷的舊基準——
`json.dump` 的序列化例外、磁碟寫到一半、行程被中斷都算。
基準檔是 **migration artifact**（第 7 步產出、之後每次比對都以它為準），
截斷等於把唯一的參考點弄丟。

**定案：改成 atomic write。**

1. 寫到**同目錄**的暫存檔（同目錄才保證同一 filesystem，`os.replace` 才是原子的）；
2. `flush()` 成功、檔案關閉後，再用 **`os.replace(tmp, output)`** 換上去；
3. 任何一步失敗都要**刪掉暫存檔**並讓例外往上拋，`args.output` 保持原內容。

⛔ **測試必須打到 CLI 層**（`main(["build", ...])` 或 subprocess）。
只測 `build_baseline()` 純函式會通過，卻完全證明不了第 3、4 條——
「先開檔再檢查」這個 bug 就活在純函式之外。

**B-1 blocking 前置條件**（對應定案表）：

| # | 檢查 | 為什麼是 blocking |
|---|---|---|
| 1 | `schema_version` 為 p1 | p0 沒記演算法，比不了 |
| 2 | `calculation` 五欄位與當下實作相同 | 公式換了就不是同一把尺 |
| 3 | **`timeframe` 相同** | ⚠️ **拿 1h report 比 1d baseline，只要標的齊全就會通過**——現有 `compare` 完全沒讀這個欄位 |
| 4 | **`source_schema_version` 相同** | report 的結構換版就不保證欄位語意相同 |
| 5 | **`thresholds` 相同** | bucket 是門檻的函數；門檻變了 bucket 比對不是同一把尺 |
| 6a | **baseline 自身完整**：`missing` 為空，且 `symbols` 與 `profiles.keys()` 相同 | ⚠️ **現行漏檢**——`compare` 的 `base` 直接取 `baseline["profiles"]`（`baseline_check.py:142`），基準在**產生時**就漏掉的標的**根本不會進入 `base`**，於是 `set(base) - set(cur)` 永遠看不到它 |
| 6b | **current report 完整**：所有 baseline symbols 在 report 裡都有 profile | 缺標的就沒有可比的母體（這是現行唯一有做的一項） |
| 7 | `pipeline_version` 相同 | ⚠️ **相容性 contract，不是迴歸偵測器**（人工常數，見下） |

⚠️ **`build_baseline` 本來就存了 `timeframe` / `source_schema_version` /
`pipeline_version`（`baseline_check.py:93-95`），但 `compare` 一次都沒讀**——
這是既有缺口，本筆一併補上。

⛔ **`thresholds` 從觀察項升為 B-1 blocking**（前一版只在 A 層提到門檻、B-1 完全省略）。
理由：門檻重定是刻意動作，**基準必須跟著重建**；讓它只出 warning 等於容忍
「thresholds 與 profiles 不同源」的基準繼續被使用。

其餘（`atr_pct` 排名、`average_range_pct` 排名、`max()` 排名、波動最高／最低者、
bucket 移動與分佈）是 **B-2**，一律照三行契約處理：
`checks[i].passed=false` → 進 `warnings` → **`compare()["passed"]` 不受影響**。

⚠️ **metadata 是 contract 檢查，不是迴歸偵測**——`pipeline_version` 是
`evaluation.py:46` 的人工常數，改壞公式而忘記升版時它不會動。真正的迴歸偵測在 A 層。

##### 五、風險與回滾

| 風險 | 處置 |
|---|---|
| 合成 fixture 不像真實資料，驗不到真實情形 | A 層只驗**公式正確性**，真實資料的規模／完整性由 B 層與 T-040 的 live run 承接，兩者不互相冒充 |
| golden 值算錯 → 把錯的固定成「正確」 | golden 由程式產生後，**用手算獨立驗一檔**（TR 三項取 max、tail(14) 平均、除以最後 close），寫進測試註解 |
| 降為觀察項後真的迴歸沒人擋 | 那正是 A 層存在的理由；且 B 層本來就擋不住（時間推進就會失敗，訊號早被雜訊蓋掉） |
| 更名斷連結 | 更新 4 處引用：`evaluation-universe-selection-plan.md:509`、`:747`，本筆與 T-040 的敘述 |
| `BUCKET_BASIS` / `ATR_METHOD` 是人寫的字串，改實作時忘了改它 | 常數放在被描述的函式旁邊；**mutation test #3／#4 是真正的保證**，不是靠自律 |

**回滾**：本筆不動 live、不動 DB、不動 runtime，**回滾即 `git revert`**。
舊的 `sr_volatility_baseline.json`（p0）在第 7 步之前都保持不動，
真要退回只需還原腳本檔名與 `baseline_check.py`。

##### 六、測試與驗證策略

| # | 測試 | 期望 |
|---|---|---|
| 1 | fixture → golden 逐位比對 | 通過 |
| 2 | **mutation**：把 `_atr_pct` 的 `tail(14)` 改成 `tail(20)` | **必須紅**（靠倒數 15～20 根的 TR 刻意不同） |
| 3 | **mutation**：把 `_atr_pct` 換成 `calc_atr`（Wilder） | **必須紅** |
| 4 | **mutation**：`volatility_bucket_from_profile` 的 `max` 改成 `min` | **必須紅**（靠那檔兩個值跨門檻不同側） |
| 4b | **mutation**：拿掉 `_volatility_profiles` 的 `df.tail(lookback)` | **必須紅**（靠前 20 根的異常波動） |
| 5 | `average_range_pct` 排序翻轉 | 該 check `passed=false` 並列入 `warnings`，**但整體 `passed` 仍為 `true`** |
| 6 | 正常資料漂移（用本次 08-17→09-02 的**實際數值**當 fixture） | 整體 `passed` 為 `true`（可有 warnings） |
| 6b | `inf` / `NaN` 輸入的**六種情境**（上表的 1／3／4／6／7／9） | 各自符合實際清理規則的輸出型態（**不是一律 `null`**），且 golden 檔不含 `NaN` / `Infinity` 字面值。情境 9 另需釘住 `average_range_pct = 0.0` 與 `bucket = LOW_VOLATILITY`（**I-108 修正前的觀察值**） |
| 7 | p0 基準檔 | B-1 blocking 失敗，訊息指出要重建 |
| 8 | **參數化竄改 B-1 的各項**（含 `timeframe`→`1h`、`source_schema_version`、`thresholds`、`pipeline_version`、`calculation` 五欄位） | **每一項都讓整體 `passed` 變 `false`** |
| 8b | **profile 缺漏要分兩側各測一次**：①從 **baseline** 的 `profiles` 抽掉一檔（`symbols` 仍留著）②從 **current report** 抽掉一檔 | 兩者都必須 blocking 失敗。⛔ 只測其中一側會漏掉 6a 那條 |
| 8c | **`build` CLI 的檔案安全**（`missing` 非空時）：①目標檔不存在 ②目標檔已存在且有內容 | ①exit code 非零且**檔案沒被建立**；②exit code 非零且**內容逐位元組不變**。⛔ 必須走 CLI，只呼叫 `build_baseline()` 證明不了這兩條 |
| 8d | **atomic write**：`missing` 為空、但**序列化中途失敗**（注入一個 `json.dump` 會拋的值）| 例外往上拋、**既有 `args.output` 內容逐位元組不變**、**同目錄不留暫存檔**。⛔ 這條與 8c 不同：8c 擋的是已知的前置失敗，8d 擋的是寫檔過程本身 |

⚠️ 第 2、3、4、4b 項是**驗「測試有效」而不是驗程式**——沒跑過 mutation 的 golden test
很可能只是在比對自己產生的空氣（I-104 已經踩過一次，該筆已收斂）。

##### 七、執行順序

1. `evaluation.py` 加 `ATR_PERIOD` / `ATR_METHOD`、`zone_builder.py` 加 `BUCKET_BASIS`（皆行為不變）。
2. 建 fixture 與 golden，寫 A 層測試，跑 mutation **2、3、4、4b**。
3. `baseline_check.py`：p1 schema、拒絕 p0、**blocking 收斂為 B-1 那 7 項**、其餘轉 B-2 warning。
4. 改寫 `test_baseline_check.py`，補測試 5～8d（含 8c 的 CLI 檔案安全與 8d 的 atomic write）。
5. 腳本更名 `observe-volatility-drift.sh` 並更新 4 處引用。
6. 更正[上表](#待更正的敘述完整盤點2026-09-03) 5 處錯誤與 2 處含混敘述。
7. **最後**才重建 `sr_volatility_baseline.json` 為 p1（方向 1，migration 動作）。
8. 重跑 T-040 驗收：A 層綠、B 層 drift 有合理說明。

##### 八、完成後的歸檔位置

* 兩層檢查的定位、fixture 設計原則、p1 schema →
  [`sr-zone-scoring.md`](./sr-zone-scoring.md)（該檔已有門檻與 provenance 的段落）。
* 驗收步驟與腳本新名 →
  [`evaluation-universe-selection-plan.md`](./evaluation-universe-selection-plan.md)
  「live 現況與端到端驗收」。
* 「固定輸入 regression 與 live drift 不可互相冒充」的原則 →
  [`development-workflow.md`](./development-workflow.md) 品質守則
  （已有 §4「測試不要依賴『真實今天』」，這是同一類）。

⛔ **本計畫書需經確認才進入實作。**

#### 順帶量測到的事實（既有風險的量化，不是新 bug）

以 09-02 的資料重算池內 135 檔的 bucket，**37 檔（27%）已與存下的 `bucket_hint` 不同**
（`1736`、`2540` HIGH→LOW，`2615` LOW→HIGH 是跨兩桶）；分佈從
LOW/NORMAL/HIGH = 53/49/33 變成 **75/39/21**（有一部分是全市場波動收斂）。

這**符合既有設計**——`bucket_hint` 明定是「入池時」的快照，
`bucket_edge_low/high` 存在每一列就是為了讓下游知道當初用的是哪組邊界
（`store/model.go:951-954`），該風險也已列在
[`evaluation-universe-selection-plan.md`](./evaluation-universe-selection-plan.md)
的風險表。這裡只是補上**幅度**：16 個日曆日 27%，比原本設想的「母體變動造成邊界移動」大得多。

#### 待更正的敘述（完整盤點，2026-09-03）

**明確寫錯的 5 處**（宣稱 `atr_pct` 取近 60 根）：

| 位置 | 現況敘述 |
|---|---|
| `docs/evaluation-universe-selection-plan.md:62` | `atr_pct` \| 近 60 根 ATR / close |
| `docs/evaluation-universe-selection-plan.md:698` | `atr_pct` 取**近 60 根** |
| `python/backtest/modular/sr_scoring/zone_builder.py:67` | `"basis": "max(atr_pct, average_range_pct)，近 60 根"` |
| `scripts/verify-regression-baseline.sh:5` | `atr_pct` 取近 60 根 |
| `python/baseline_check.py:3` | `atr_pct` 取近 60 根 |

**語意含混的 2 處**（講的是 60 根的**切片**，但讀起來像 ATR 窗口）：
`evaluation-universe-selection-plan.md:100`、`selection_report.py:16`。

**正確、不用改的 2 處**：`plan:63`（`average_range_pct` 確實是 60 根）、
`db.py:166`（selection report 確實抓 60 根）。

#### 關閉條件

1. 上表 5 處錯誤敘述與 2 處含混敘述全部更正。
2. **固定輸入的程式回歸測試存在、blocking，且會因公式改壞而紅**（不依賴 live 資料）。
3. live 資料的排序／bucket 比較改為觀察項，**不再以 regression baseline 為名**。
4. baseline schema 升到 `p1` 並記錄 `atr_method` / `atr_period` /
   `profile_lookback_bars`；p0 被明確拒絕。
5. 新格式基準重建完成，T-040 重跑後固定輸入 regression 通過、live drift 有合理說明。

⚠️ **公式裁決不是本筆的關閉條件**——那是 I-107。本筆的 `atr_method` 先記錄**現況**
（`tr_sma`），I-107 若改變 canonical formula 再更新。這樣兩件事才不會互相卡住。

---

### I-107：evaluation／selection 用 TR SMA(14)，runtime 用 Wilder ATR(14)，凍結門檻與 runtime 不同源

| 欄位 | 內容 |
|---|---|
| 狀態 | 待決策 |
| 嚴重度 | 中（**目前不發作**——`SR_SCORING_ADAPTIVE_ZONE_BUILDERS_ENABLED` 在 live 為 `False`；一旦打開就會系統性偏向高波動） |
| 分類 | Python / 指標定義 / 已知限制 |
| 建立日期 | 2026-09-03 |
| 來源 | I-106 的 review——原以為只是「文件寫 60、實作是 14」，查證後發現是兩個不同演算法 |

#### 現象

| 位置 | 演算法 | 誰在用 |
|---|---|---|
| `evaluation.py:287` `_atr_pct` | **最後 14 根 true range 的算術平均** / 最後一根 close | evaluation 報表、**`selection_report.py:119`——即凍結門檻的來源** |
| `scoring.py:216` → `indicators.py:100` `calc_atr` | **Wilder smoothing**：seed = `mean(tr[1:15])`，再一路平滑到第 60 根 | `_adaptive_zone_builder_profile`（runtime 自適應 builder） |

同一批 K 棒的實測差距（2026-09-02，60 根切片）：

| 標的 | TR SMA(14) | Wilder ATR(14) | 差異 |
|---|---|---|---|
| `0050` | 1.555% | 1.933% | **+24.3%** |
| `2330` | 1.722% | 2.052% | +19.2% |
| `2478` | 7.025% | 8.319% | +18.4% |
| `5490` | 2.692% | 3.832% | **+42.4%** |
| `6243` | 5.483% | 6.435% | +17.4% |

#### 為什麼是問題

`LOW/HIGH_VOLATILITY_THRESHOLD` 的 P33/P67 是拿 **TR SMA(14)** 基準對 319 檔量的
（`zone_builder.py:59-70` 的 `VOLATILITY_THRESHOLD_PROVENANCE`），
而 runtime 的自適應 builder 用 **Wilder**。Wilder 系統性高 17～42%，
**門檻與 runtime 不同源**——打開旗標後分類會系統性偏向高波動。

⚠️ **維度有三個，不是「14 或 60」的選擇題**：ATR period 多少、方法是 SMA 還是 Wilder、
給多少根 warm-up／lookback。單把 evaluation 的 period 改成 60 會得到 **`TR SMA(60)`**，
仍然不等於 runtime 的 Wilder ATR(14)。

#### 待決策：canonical formula 三選一

1. **統一用 Wilder ATR(14)**——與 Go `CalcATR`、與 runtime 一致；**凍結門檻必須重測**。
2. **統一用 TR SMA(14)**——與凍結門檻、選池、既有基準一致；runtime 要改，
   且與 Go 端的 ATR 定義分家。
3. **承認是兩個指標**——分別命名（`atr_pct_sma14` / `atr_pct_wilder14`）、各自記
   provenance，明確寫出哪一個在分桶、哪一個在門檻。

#### 決策前的必要量測與步驟

1. 先量測 **SMA14 與 Wilder14 在 319 檔上的分佈與 bucket 差異**（不能只看 5 檔）。
2. 選定 canonical formula。
3. 若公式改變：重算 P33/P67 → 升 `universe_version` → 更新 135 列的
   `bucket_edge_low/high` → 與 T-003「bucket 邊界必須凍結」對齊。
4. **完成前 `SR_SCORING_ADAPTIVE_ZONE_BUILDERS_ENABLED` 維持關閉。**

#### 已知限制（在決策完成前成立）

⛔ **evaluation／選池的 bucket 與未來 runtime adaptive 的 bucket 尚未證明同義。**
現有報表、`evaluation_universe.bucket_hint`、前端顯示的 bucket 都是 **TR SMA(14)** 基準；
不要拿它們去推論自適應 builder 打開後 runtime 會怎麼分類。

#### 關閉條件

canonical formula 有結論，evaluation、`selection_report.py`、runtime 三處與
`VOLATILITY_THRESHOLD_PROVENANCE` 同源；若公式改變則門檻已重測、`universe_version` 已升、
135 列邊界已更新。決策結果歸檔到 [`sr-zone-scoring.md`](./sr-zone-scoring.md)。

---

### I-108：volatility profile 沒有完整拒絕非有限的 `close`，會產出看似合法的 `0.0`

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 低（live PostgreSQL 路徑取不到 `inf`；**但 `--csv` 明確可達，SQLite 也擋不住**） |
| 分類 | Python / 健壯性 |
| 建立日期 | 2026-09-04 |
| 來源 | I-106 計畫書為了寫 fixture 而做的非有限值實測 |

#### 現象

`evaluation.py:303` 的守門只有 `if last_close <= 0: return None`。
`float("inf") <= 0` 是 `False`，於是流程繼續：

```
atr / inf = 0.0   →   _clean_metric(0.0) = 0.0（有限值，不會變 None）
```

**實測**（30 根合成資料）：`last_close = +inf` → `atr_pct = 0.0`。

對照組：`last_close = NaN` → `atr / nan = nan` → `_clean_metric` → **`None`**（正確）。
`last_close = 0` → 被既有守門擋掉 → `None`（正確）。
**只有 `+inf` 這條路徑會產出一個看起來正常的值。**

#### bucket 的實際影響（訂正）

⛔ **前一版寫「於是會被判成 `LOW_VOLATILITY`」是錯的**——那句話忽略了
`volatility_bucket_from_profile` 用的是 `basis = max(atr_pct, average_range_pct)`
（`zone_builder.py:405`）。`atr_pct = 0.0` 在 `max()` 裡是最小的那個，
**通常會被另一個分量整個蓋過去**。

實測（30 根合成資料，只把**最後一根** `close` 換成 `+inf`）：

| fixture 波動 | 正常 `atr_pct` | `+inf` 後 `atr_pct` | `+inf` 後 `average_range_pct` | bucket |
|---|---|---|---|---|
| 2%   | 0.0200 | **0.0** | 0.01933 | `LOW_VOLATILITY`（**與正常時相同**） |
| 5.5% | 0.0550 | **0.0** | 0.05317 | `NORMAL_VOLATILITY`（**與正常時相同**） |
| 8%   | 0.0800 | **0.0** | 0.07733 | `HIGH_VOLATILITY`（**與正常時相同**） |

正確的說法是三段：

1. **`atr_pct` 本身一定是錯的**——該回 `None` 卻回 `0.0`，一個看似合法的數字；
2. **bucket 只在 `average_range_pct` 也低於 `LOW` 門檻（或為 `None`）時才會被拖成 `LOW`**；
3. 其餘情況 bucket 由另一個分量決定、外觀正常，**於是這個錯誤更難被發現**——
   `atr_pct` 欄位已經是垃圾，卻沒有任何下游徵兆。

⚠️ **另一個入口：`average_range_pct` 自己也守不住 `+inf`。**
`_volatility_profiles`（`evaluation.py:319`）算的是 `(high - low) / close`，
再用 `.replace([np.inf, -np.inf], np.nan).dropna()` 清理——但 `(h - l) / inf` 的結果是
**`0.0`（有限值）**，清不掉。實測：**整段 `close` 都是 `+inf` 時
`atr_pct` 正確回 `None`，`average_range_pct` 卻是 `0.0`，bucket 被判成 `LOW_VOLATILITY`。**
「資料壞掉的標的被歸類成最穩」這個情境**確實存在，但入口是 `average_range_pct`，
不是 `_atr_pct`**。修正範圍要涵蓋兩者。

#### 可達性

* ✅ **CSV 路徑明確可達**：`evaluation.py:2508` 的 `--csv` → `_load_csv_sources`
  → `load_ohlcv_csv`，pandas 預設會把 `inf` / `Infinity` 字面值讀成 `float("inf")`。
* ⛔ **live PostgreSQL 路徑不可達**：`candles.close` 是 `DECIMAL(10,2)`
  （`migrations/postgres/001_create_candles.sql:9`）。實測 PostgreSQL 16.14：
  `INSERT INTO t(close numeric(10,2)) VALUES ('Infinity')` →
  `ERROR: numeric field overflow / A field with precision 10, scale 2 cannot hold an infinite value`。
  ⚠️ **擋住它的是宣告精度，不是 CHECK**——同一測試裡不帶精度的 `numeric` **接受** `Infinity`，
  而 `060_candle_positive_price_check.sql` 的 `close > 0` 對 `Infinity` 為真、擋不下來。
* ⚠️ **SQLite 未排除**：`close` 是 `REAL`、約束同樣只有 `close > 0`
  （`migrations/sqlite/060_candle_positive_price_check.sql:14,20`）。
  實測：以該 DDL 建表後 `INSERT` `float("inf")` **成功寫入並讀回 `inf`**。
* ⚠️ **Go 寫入前的守門也擋不住**：`fetcher.go:245` 只比 `c.Close <= 0`，
  `inf <= 0` 為 `false`，直接放行。

因此範圍是「**目前 live PostgreSQL DB 路徑不可達；CSV 明確可達，
SQLite／直接 repository 寫入也未完整排除**」，而不是前一版寫的「DB 路徑不可達」。

#### 處置

✅ **定案：沿用現有的逐列清理語意，不改成「整檔失效」。**
前一版把「剔除單列」與「整檔判為不可用」並列成兩個選項、關閉條件卻只寫得出後者，
是沒定案就先寫驗收——這裡收斂掉。

選逐列的理由：`average_range_pct` 現行就是逐列清理（`NaN` 列丟掉、其餘照算），
`_atr_pct` 的 `last_close <= 0` 也只是**除數**守門而不是整檔守門。
改成「任何一列 `close` 非有限就整檔 `UNKNOWN`」是**更嚴格的新 contract**，
不在本筆範圍。

⛔ **前一版寫「會改動情境 1／3／6／8」是錯的。** 那條 contract 的判準是
**`close` 非有限**，所以涉及的是 `close` 為 `NaN` / `Inf` 的**情境 3／6／7／9**：

* **情境 1**（`high=NaN`，`close` 仍有限）**不觸發**；
* **情境 8**（`close=0`）也**不觸發**——`0.0` 是有限值，它是被既有的
  `last_close <= 0` 擋掉的，與「非有限」是兩條不同的守門。

**兩個 contract 的結果差異在情境 3／6／7**；情境 9 兩者殊途同歸：

| 情境 | 現況 | 逐列（本筆定案） | 整檔失效（未採用） | 兩者是否不同 |
|---|---|---|---|---|
| 3 整列 `NaN` | 0.02 / 0.02 / LOW | **完全不變**（實測） | → `UNKNOWN` | **是** |
| 6 `last_close=NaN` | `None` / 0.02 / LOW | **完全不變**（實測） | → `UNKNOWN` | **是** |
| 7 `last_close=+inf` | `0.0` / 0.01933 / LOW | `None` / 0.02 / **`LOW`**（實測） | → `UNKNOWN` | **是** |
| 9 全部 `close=+inf` | `None` / `0.0` / LOW | `None` / `None` / **`UNKNOWN`**（實測） | → `UNKNOWN` | 否 |

⛔ **不要把「相對現況有變」與「兩個 contract 之間有差異」混為一談**——前一版寫
「7／9 在兩種 contract 下都會變，不是差異點」正是犯了這個錯。情境 7 兩種 contract
都會改動現況，但**改成的結果不同**（逐列 → `LOW`，整檔失效 → `UNKNOWN`），
所以它是差異點；只有情境 9 兩邊最後都落在 `UNKNOWN`。

也就是說，改用整檔失效會把「一根壞列」升級成「整檔沒有波動度可用」，
**連帶改掉 `NaN` 現行被逐列丟棄的既有行為，也會讓情境 7 從 `LOW` 變成 `UNKNOWN`**
——這正是不採用它的理由。
若日後要改用整檔失效，必須把 contract 寫完整（是只看 `close`，
還是連 OHLC 任一欄非有限、`close <= 0` 都算），並重建對應 fixture。

**兩處改動：**

1. **`_atr_pct`**：守門改成 `if not math.isfinite(last_close) or last_close <= 0`。
   ⚠️ **只加除數守門，不動 TR 的逐列清理**——`atr_pct` 是用**最後一根 close** 正規化的，
   最後一根是垃圾就沒有有意義的分母，該回 `None`。
   `math` 已經 import 過（`_clean_metric` 在用），不需要新相依。
2. **`_volatility_profiles` 的 `average_range_pct`**：在算 `range_pct` **之前**
   剔除 `close` 非有限的列（`recent[np.isfinite(recent["close"].astype(float))]`）。
   現行的 `.replace([np.inf, -np.inf], np.nan)` 只清得掉**結果**為 `inf` 的列，
   清不掉 `(h - l) / inf = 0.0`。

⛔ **過濾後的 df 不可以拿去餵 `_atr_pct`**——那會讓「最後一根壞掉」變成
「用倒數第二根當分母」，靜默算出一個有限值，等於換一種方式說謊。
`_atr_pct` 要收到**未過濾**的 `recent`，由它自己的守門回 `None`。

#### 處置後的預期輸出（原型實測，2026-09-04）

30 根合成資料，`→` 標示與現況不同者：

| 情境 | 現況 `atr_pct` / `avg` / bucket | 處置後 |
|---|---|---|
| 正常有限 | 0.02 / 0.02 / LOW | **完全不變** |
| → 最後一根 `close=+inf` | **0.0** / 0.01933 / LOW | **`None`** / **0.02** / LOW |
| → 中段 `close=+inf`（tail14 內） | `None` / 0.01933 / LOW | `None` / **0.02** / LOW |
| → **全部 `close=+inf`** | `None` / **0.0** / **LOW** | `None` / **`None`** / **`UNKNOWN`** |
| 最後一根 `close=NaN` | `None` / 0.02 / LOW | **完全不變** |
| 最後一根 `close=0` | `None` / 0.02 / LOW | **完全不變** |
| 中段 `high=NaN` | 0.01929 / 0.02 / LOW | **完全不變** |

三件事值得先講清楚：

* **bucket 只在「全部 `close` 非有限」時改變**（LOW → UNKNOWN）。單一壞列時
  `average_range_pct` 仍由其餘有效列算得出來，bucket 由它決定——這正是逐列語意。
* `average_range_pct` **本身會變**（0.01933 → 0.02）：現況把 `inf` 那列當成振幅 0
  混進平均，處置後那列被剔除。這是修正，但**會動到既有數值**，golden 要跟著更新。
* `NaN` / `0` / 中段 `high=NaN` **一位數都不變**——非回歸案例要把這幾條釘住。

#### 實作計畫（最低限度）

**受影響檔案**：`python/backtest/modular/sr_scoring/evaluation.py`
（`_atr_pct`、`_volatility_profiles`）。**不動** `zone_builder.py`——
`volatility_bucket_from_profile(None, None)` 本來就回 `UNKNOWN_VOLATILITY`。

**測試**：`python/tests/`（與 I-106 的 A 層 golden 同一組 fixture），涵蓋
* 上表七個情境全部，**四個「完全不變」的要當非回歸斷言寫死**；
* 三種 `inf` 位置分開測：**單一最後一根／單一中段／全部**——只測其中一種會漏掉
  「bucket 只在全部非有限時才變」這個結論；
* `-inf` 與 `+inf` 各一（守門用 `math.isfinite`，兩者應同行為）。

**與 I-106 的順序**：⚠️ **I-106 先做完、本筆才動手。**
理由是**基礎設施依賴**——本筆的測試要沿用 I-106 建立的版控 OHLC fixture 與
golden 比對機制，沒有那套東西就得自己再造一份。

⛔ **不要寫成「反過來做 I-106 的 golden 一建立就是紅的」**——那個理由不成立：
先做 I-108 的話，I-106 只會直接以**修正後**的行為建立 golden，不會先紅。
順序是為了不重造 fixture，不是為了避開失敗。

依現在這個順序，本筆會改動 I-106 golden 裡情境 7 與情境 9 的值
（情境 7：`atr_pct` `0.0` → `None`；情境 9：`average_range_pct` `0.0` → `None`、
bucket `LOW` → `UNKNOWN`），**同一個 commit 內更新 golden 並移除
「I-108 修正前的觀察值」註解**。

⚠️ **這是行為變更**：`I-106` 的 golden 檔會記錄修正前的 `0.0`，
**本筆修好時要同步更新 I-106 的 golden 與測試註解**（那裡已標明是「I-108 修正前的觀察值」）。

#### 關閉條件

「處置後的預期輸出」那張表的七個情境全部成立，具體是：

1. **最後一根 `close` 為 `±inf`** → `atr_pct` 回 `None`（不再是 `0.0`）；
   `average_range_pct` 由**其餘有效列**算出有限值；bucket 由它決定。
2. **全部 `close` 非有限** → 兩個 metric 都是 `None`，
   bucket 為 `UNKNOWN_VOLATILITY`（不再是 `LOW_VOLATILITY`）。
3. **有限正常資料、`NaN`、`0` 的輸出逐位不變**（非回歸斷言）。

且 I-106 的 golden 已在同一個 commit 內同步更新、
「I-108 修正前的觀察值」註解已移除。

**歸檔位置**：[`sr-zone-scoring.md`](./sr-zone-scoring.md) 的
「Volatility bucket 門檻＝凍結的全市場分位數」章節之下，補一節現況說明，保留三件事：

1. **非有限 `close` 採逐列清理**——該列從 `average_range_pct` 的母體剔除，
   其餘列照算；不是整檔失效。
2. **ATR 與 average range 的輸入處理不同**——`atr_pct` 是**除數**守門
   （最後一根 `close` 非有限或 `<= 0` 就回 `None`，不改 TR 的逐列清理）；
   `average_range_pct` 是**逐列**守門。兩者刻意不一樣，理由寫進去。
3. **有效輸入全部消失時回 `UNKNOWN_VOLATILITY`** 的契約——
   `volatility_bucket_from_profile(None, None)` → `UNKNOWN`，
   ⛔ **不是** `LOW_VOLATILITY`。

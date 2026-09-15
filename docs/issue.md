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
- **下一個新編號從 `I-114` 起算。**（**I-113 於 2026-09-10 發出**——由 I-107 步驟 1 量測時發現池內成員數對不上（`evaluation_universe` 135 筆 `active`，實際只掃到 134 檔）。**I-104 / I-105 / I-112 於 2026-09-09 收斂**——三筆的現況都歸檔在 `architecture.md`：I-104 的「**外來錯誤必須先分類，不得把原始錯誤寫進使用者可見欄位**」與**合法形式表**在「寫入失敗的一致性契約」；I-105 的「`verification_unavailable` 一定要帶得出成因」在「日 K 缺漏偵測」；I-112 的 `<stage>_failed:N (symbol:reason, …)` 在「逐檔失敗要帶得出哪一檔、哪個階段、什麼類別」。**編號都不回收。** **I-112 於 2026-09-08 發出**——`todo.md` T-070（已收斂）的 live 觀察時發現 `corporate_action_sync` 的逐檔失敗不寫進 `error`。**I-109 / I-110 / I-111 於 2026-09-07 發出、2026-09-08 全部修復並收斂**——三筆都由 `todo.md` T-071 的實作與 review 分出：I-109 是 `parseROCDate` 收下不存在的民國年（現況歸檔在 `architecture.md`「民國日期的解析是嚴格的」）；I-110 是前端 job 清單漂移（歸檔在 `development-workflow.md`「新增排程還要同步前端的 job 清單」，並由 `scripts/check-job-names.sh` 擋住）；I-111 是 SQLite 的 `busy_timeout` 保護不到 deferred transaction 的升級（歸檔在 `database-schema.md` 的 CAS 契約，改用 `BEGIN IMMEDIATE`）。**編號都不回收。** **I-108 於 2026-09-04 發出**——由 I-106 計畫書的非有限值實測分出；**I-106 / I-107 於 2026-09-03 發出**——I-107 由 I-106 的 review 分出（TR SMA(14) 與 Wilder ATR(14) 的公式分歧）；I-106 來自 T-040 regression baseline 實跑——T-040 regression baseline 實跑時發現 `atr_pct` 的窗口與註解不符、且 evaluation 與 runtime 用的是兩個不同的 ATR 演算法；**I-103 / I-104 / I-105 於 2026-09-02 發出**——I-103 由 I-102 計畫書 review 分出（Yahoo 批次路徑給不出逐檔寫入失敗）；I-104 由 I-102 實作 review 分出（其餘排程與 job 紀錄仍直接寫入原始錯誤）；I-105 來自 `2867` 跨月當天 live 首次 `partial`（`verification_unavailable` 的成因被丟棄）；I-101 / I-102 於 2026-09-01 發出——前者來自 live 的 indicator upsert 溢位、**已於同日修復並收斂**（未完成的 live 部署由 `todo.md` T-069 承接，**該筆已於 2026-09-02 部署驗收完成並收斂**），後者由它的 review 分出、**2026-09-02 實作部署完成並收斂**（現況規格歸檔在 `architecture.md`「寫入失敗的一致性契約」與 `api-reference.md` 的兩條端點，未完成的執行期觀察由 `todo.md` T-070 承接）；I-100 於 2026-09-01 發出，由 `todo.md` T-068 同日改列——**T-068 編號不回收**；**I-099 於 2026-08-31 發出後同日作廢**——誤把 `deploy.sh` 的保守預設當成與 live 的衝突，實際上該檔是範本、所有開關一律預設 `false` 是既有慣例；**編號不回收**；I-098 於 2026-08-31 由 I-096 的 review 發現分出；I-081～I-083 於 2026-08-21 發出（**I-081 / I-082 於 2026-08-27 隨 `todo.md` T-055 收斂**），I-084～I-087 於 2026-08-24 發出，I-088～I-092 於 2026-08-25 發出（**I-091 於 2026-08-28 收斂**），I-093 / I-094 於 2026-08-26 發出（I-093 已於同日收斂，**I-094 於 2026-08-28 收斂**），I-095～I-097 於 2026-08-27 發出，其中 **I-097 於同日改列 `todo.md` T-064**——編號**不回收**。）
  **發出新編號時記得把這一行一起往前推**——上一次就是漏了這步，I-089 發出去之後
  這裡還寫著「從 I-089 起算」，差一點又重用一次（I-070 已經發生過）。
  **現存條目**裡最大的是 I-113（⚠️ 本行下方的歷史索引仍看得到更大的編號，那是紀錄不是條目）。I-109～I-111 於 2026-09-08 收斂、I-104／I-105／I-112 於 2026-09-09 收斂，I-113 於 2026-09-10 發出，**下一個可用的是 I-114**；I-102 已於 2026-09-02 收斂、編號不回收——I-096 / I-098
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
  本檔現有的 I-100 / I-103 / I-106 / I-107 / I-108 / I-113、已收斂的 I-102 / I-104 / I-105 / I-109～I-112、
  以及下一個可用的 I-114），
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

**2026-09-10 實證：影響範圍不只「重跑選池」。** I-107 步驟 1 要在「資料新鮮、
且套用既定資格規則的全市場股票母體」上量 SMA14 vs Wilder14（同一套規則在 2026-08-17
得到 319 檔），實跑得到的母體只有 **125 檔、且全部都在池內**——503 檔上市股票裡
**377 檔被判 `stale_candle`**（池外日 K 停在 **2026-08-12**，池內是 2026-09-08）。
**本筆原本只描述選池重跑，實際上任何需要全市場母體的量測都會被擋**，包含門檻重測。
數據見 [I-107](#i-107evaluationselection-用-tr-sma14runtime-用-wilder-atr14凍結門檻與-runtime-不同源)
的「步驟 1 量測（2026-09-10 執行）」。

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
| **0** | 完成可重現性前置（I-100）與診斷欄位，**並產生封存凍結輸入 bundle**（此時才讀 DB）。⚠️ **I-100 的工具已於 2026-09-10 實作完成**（`--as-of`／`--emit-bundle`／`scripts/run-replay-offline.sh`，用法見 [`development-workflow.md`](./development-workflow.md)）；本階段還缺的是**診斷欄位**與**實際產出並進版控的 bundle** | 可重跑的 replay 路徑 ＋ 新欄位 ＋ 已封存的 bundle |
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
  | **`candidate_mismatch.json`**（v8 新增） | before／after 的候選集合不一致時的**完整差集** ＋ `after_artifact_sha256` | ⚠️ **另一種終止狀態**：此時⛔ 不產出上面兩份，**差集本身就是分支 C 的證據**（見下方決策樹） |

  ⚠️ **Stage 2 有兩種 terminal outcome，⛔ 不是只有一種**：

  | 集合檢查 | 產出 | 結束碼 |
  |---|---|---|
  | **一致** | `comparison_artifact.json` ＋ `report.json` | 0 |
  | **不一致** | `candidate_mismatch.json` | `EXIT_CANDIDATE_MISMATCH = 4`，**直接判為分支 C** |

  * **總候選數必須全數統計，且全數套用決策樹判定**——分支 A/B/C 看的是全部候選。
    漏判會讓分支 C 被藏起來。
  * **報告必須同時寫明總候選數、附了幾列，以及完整 artifact 的 SHA-256**，
    讓讀報告的人知道自己看到的是子集、並且能取到全集核對。
  * 這份 artifact 的形式已有前例：
    [`python/baselines/replay_cohort_2026-09-01.json`](../python/baselines/replay_cohort_2026-09-01.json)
    就是「逐列比較欄位進版控、原始 report 只留 hash」的同一個做法。
  * 💡 **200 這個上限實務上很可能不會被觸發**：依 2026-09-01 自然樣本 `CONTINUATION`
    佔 0.5% 外推，**13,417** 列大約只產生 ~67 列（v8 更正——舊文用產 bundle 前的概估
    15,600），再篩掉 RR 合格的更少。
    規則仍要寫清楚，是為了真的超過時處置不會臨時決定。
* **執行次數**（⚠️ 2026-09-11 依已確認的跨日政策改寫——舊文寫「Stage 1 一趟 ＋ Stage 2
  一趟」，與 D／D+1 合算一次的裁決不符）：

  | 層面 | 內容 |
  |---|---|
  | **邏輯計次** | **D／D+1 的兩趟 after replay 合為「一次」正式 Stage 1 scan** |
  | **實體執行** | **兩趟 after**（D 與 D+1，驗重現性）；**若有候選，再加一趟 before**（Stage 2） |
  | ⛔ 上限 | 最壞 **三趟** replay；⛔ 不得再有第四趟 |

  ⛔ 不因結果調整條件、不擴大標的或日期、不加大範圍；
  ⛔ **兩趟 after 之間不得修改 predicate、程式碼或其他判定條件**。
  ⚠️ **preflight 與輸入指紋檢查失敗不計入這個次數**——那是輸入還沒就位，不是驗證跑過了。
  ⚠️ 兩趟 after 逐列不一致時**必須立案調查**，⛔ 不得以重新執行覆蓋或取代失敗結果。

**預估成本**（2026-09-01 review 後重算——原估只算了 Stage 1，漏了 Stage 2 必須連續
warm-up 而不是只跑候選列）：

[`sr-zone-scoring.md`](./sr-zone-scoring.md)「規模上限」實測 11 檔 × `--limit 1500` ×
200 列 ＝ 2 分 50 秒，且「replay 的時間由 `replay_max_rows` 決定，每一列都要重建 zone
並跑完整 decision engine」——約 **0.85 秒／列**。

⚠️ **範圍已由凍結 bundle 確定為 13,417 列**（v8 更正——舊估的 15,600 是產 bundle 之前的
概估）：`14352 − 11 × (min_history_bars 80 + forward_bars 5)`。
**0.85 × 13,417 ≈ 3.17 小時／趟**；下表沿用 **3.7 小時**當**保守上界**。

⚠️ **2026-09-11 更新**：依已確認的跨日政策，實體執行是**兩趟 after ＋（有候選時）一趟
before**，⛔ 不是舊文的「一趟 after ＋ 一趟 before」。

| 趟次 | 內容 | 成本 |
|---|---|---|
| after（D） | after 版全範圍連續 replay | ~3.7 小時 |
| after（D+1） | **同一份 bundle 重跑**，驗跨日重現性 | ~3.7 小時 |
| before | before 版全範圍連續 replay（**零候選時⛔ 不需要**） | ~3.7 小時 |
| | **合計** | **零候選約 7～8 小時；有候選約 11 小時（三趟）** |

**Stage 2 不需要再跑一次 after**——D+1 那趟已經是一趟完整的 after 全掃，它的逐列輸出
直接充當比對的 after 半邊。這是把成本壓在「三趟」而不是「四趟」的關鍵，
兩趟 after 因此都必須輸出**完整逐列資料**而不是只有候選名單。

記憶體不是瓶頸（邊際約 1.0 MB/檔，此規模約 300MB，131 檔實測才 382MB），**時間才是**。

⚠️ **v8 更正一句 v7 寫錯的話**：v7 寫「單趟 3.7 小時就遠遠跨出當日 09:00–15:00 的凍結窗」
——**那不成立**，09:00–15:00 有 6 小時，單趟 3.2～3.7 小時塞得進去。

**凍結 bundle 之所以是硬性前置，理由是另外兩個**：

1. **驗收明訂跨日**（I-100 關閉條件 2：「不同日期載入同一份 bundle」），**跨日就必然跨出
   任何單日的凍結窗**；
2. ⚠️ **上游資料本來就會變**——2026-09-11 已實證還原係數在 **24 小時內**就動了
   （見 [I-100](#i-100decision-replay-沒有-as-of-上界cohort-隔天就重現不了) 十九）。
   沒有 bundle，第二趟的輸入根本不會與第一趟相同。

##### 需要補的診斷欄位（Stage 0）

replay row 目前拿不到 lifecycle 真正使用的判斷輸入，用相近欄位反推會有偽陽性：

| 欄位 | 現況 | 取得方式 |
|---|---|---|
| **decision primary zone** | ⚠️ **已經有了，只是 replay 取錯顆** | `build_decision_summary()` 早就輸出 `decision_summary["primary_zone"]`（`decision_engine.py:2962`，來源是 `_pick_primary_zone()`）；`evaluation.py` 卻用 `_historical_zone_score_summary` 的排序第一筆（`:773`）。⚠️ **2026-09-11 修正：「不需要動 `decision_engine.py`」不成立**——`_decision_summary_zone()`（`:112`）**沒有 `relative_volume`**，直接切過去會讓 `_volume_strength_bucket()` 靜默退化成 unavailable，所以要在那裡補一個欄位；且 `evaluation.py` 有**三個** consumer 必須一起切（replay row `:1519`／`daily_confirmation_context` `:1078`／`daily_confirmation_outcome` `:1215`） |
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
rr_decoupling_candidate  ≡  lifecycle_phase == "CONTINUATION"  且  setup_rr_qualified == false
```

⚠️ **`setup_rr_qualified` ⛔ 不是對外的 `rr_gate.qualified`**（2026-09-11 Stage 0 計畫書
v2 修正——原文寫成後者是錯的）。它指的是 **setup gate**（`decision_engine.py:2678` 的
`_rr_gate()`），也就是 semantic pipeline 在 `:1051` 讀到的那顆；對外
`decision_summary["rr_gate"]` 已在 `:2779` 被 `_execution_rr_gate()` **覆寫**。
拿對外那顆去驗這條等價式會得到矛盾。Stage 0 會把 setup 那顆明確匯出成 `setup_rr_qualified`。

**為什麼這一條就等價於「在 before 版不會是 CONTINUATION」**：after 版判定成
`CONTINUATION`，本身已經蘊含「高優先分支不成立 ＋ `CLOSE_RECLAIM` ＋ 上行跟隨 ＋ 動能確認
＋ 明確突破」全部成立；而 before 版的同一條分支只多一個 `and rr_qualified`
（`ecbc141^:decision_engine.py:958-963`）。所以只要 `rr_qualified == false`，before 版那條
必然不成立，往下掉到 `CONFIRMED`（`SUPPORT_RECLAIM_CONFIRMED` 或 `reclaim_age >= 1`）
或再往下的 `TESTING`。**兩個方向都成立，是等價不是充分條件。**

⚠️ **這個定義成立的前提，是 setup gate 在兩個版本裡對同一列會算出相同的值**——已查證：
兩版都是 `market_action` → `entry_action_state` → `_rr_gate()`
（after `:2674`／`:2677`／`:2678`，before `ecbc141^:2437`／`:2440`／`:2441`），
**全部排在 `decision_derived_view` 之前**（after `:2718`，before `ecbc141^:2472`），
不依賴 `lifecycle_phase`。同理 `event_state_summary` 建於 `:2626`，也在 lifecycle 之前——
所以**某一列的 lifecycle 差異不會回饋到下一列的事件狀態**，before/after 兩趟 replay 會保持
逐列對齊。
⚠️ **這條前提只涵蓋 setup gate**：after 版在 `:2779` 還有一次 `_execution_rr_gate()` 覆寫，
⛔ 那一顆不在本前提的保證範圍內，也⛔ 不是 candidate 的依據。

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
| `decision_engine.py` 的 semantic pipeline | **四鍵**：`clear_zone_breakout`／`continuation_price_evidence_met`（**逐鍵透傳** lifecycle 的輸出）＋ `setup_rr_qualified`（透傳自 setup gate）＋ `rr_decoupling_candidate`（組合） | RR 只在這一層才存在——setup gate 的 `rr_qualified` 讀於 **`:1051`**，lifecycle 拿不到。⚠️ **透傳不能省**（2026-09-11 修正）：`:1046-1049` 目前只取 lifecycle 的三個既有鍵，不透傳的話新增的欄位會在這裡消失 |
| `evaluation.py` | **只匯出，不自行重算**；⚠️ 例外是 **no-zone 列的合法缺席 fallback**（見 Stage 0 計畫書），那是 serialization 層填預設值，⛔ 不是重算上游判斷 | 任何在 replay 端重算的版本都會重蹈偽陽性 |

**所以 `decision_engine.py` 確實需要修改**——原文寫「預期不需要修改」不成立，已改正。
它只在 semantic pipeline 內組合上游已經給定的兩個布林值，不新增判斷、不改任何既有分支。

**護欄（2026-09-11 精確化）**：⛔ **`lifecycle_engine.py` 與 `decision_engine.py` 的交易判定
分支與優先序一行都不得改**——為了製造命中而放寬 predicate 是本筆的頭號禁止事項。
⚠️ 但「三層都只是純新增欄位」的舊說法**不精確**：`evaluation.py` 另外還有
**primary zone 來源切換**、**Stage 1／2 的 fail-closed 控制流**與 **no-zone fallback**，
那三項是行為變更而非純新增，⛔ 不能用「移除欄位即回滾」一句帶過。

##### before 版沒有 `lifecycle_engine.py`——同一套 tooling 要套到兩種形狀

⚠️ **這一點於 2026-09-01 review 補上，原文完全沒有涵蓋。**
before ＝ `ecbc141^`，而 `lifecycle_engine.py` 是 `ecbc141` 才新增的（`git ls-tree` 實查：
`ecbc141^` 下不存在該檔）。before 版的整段判定**內嵌在
`decision_engine.py`**（`ecbc141^` 的 `:946-975`），而且 `rr_qualified` 就寫在
`CONTINUATION` 的條件裡——**那正是本筆要驗的那個條件**。

所以：

* 診斷欄位是 **validation-only tooling**，必須以**兩種不同的形狀**套用到兩個版本：
  after 版走上表的三層分工；before 版因為沒有 `lifecycle_engine.py`，全部落在
  `decision_engine.py` 同一個函式內。
  ⚠️ **2026-09-11 修正：before 側要產出的不是「兩個欄位」，而是與 after 對齊的完整 replay
  contract**（九個診斷欄位，見 Stage 0 計畫書六），否則 Stage 2 的 validator 與第五道集合
  檢查都無從比起。**before tooling 的完整輸出責任延到 Stage 2 計畫定義**，
  ⛔ 不在 Stage 0 範圍；Stage 0 只負責讓 validator **有能力**驗它。
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

#### Stage 0 計畫書 v17（2026-09-11 起草，2026-09-14 **已確認；已實作，review 已通過**）

⚠️ **v16 的 review 抓到 3 中 1 低，已修**；修訂摘要在本節最後。
v13～v16 都是**純格式／紀錄修正**，v17 修的是**實作與測試**——⛔ 三者都沒有動任何語意契約，
語意在 v12 就收斂了。

⚠️ v11 的教訓仍然有效：**描述形狀時要貼實際產物**（`compare_rows()` 在
`artifacts.py:229`），⛔ 不能憑印象寫「同形狀」。

✅ **計次裁決已由使用者於 2026-09-11 明文確認**，見四。

##### 一、目標與不做的範圍

**目標**：補上 Stage 1 挑候選所需的診斷欄位，讓 replay row 帶得出
`rr_decoupling_candidate`，並把判定分支 B／C 的對象一併匯出。

**⛔ 不做**：

* ⛔ **`lifecycle_engine.py` 與 `decision_engine.py` 的交易判定條件與優先序一行都不得改**；
  `evaluation.py` 則**只允許本計畫明列的來源切換與驗證守門**（primary zone 來源、
  no-zone fallback、Stage 1／2 fail-closed）。⚠️ v3 之前寫的「三層只允許新增輸出欄位」
  **不精確**，已改正。為了製造命中而放寬 predicate 是本筆的**頭號禁止事項**；
* ⛔ 不改 `resolve_lifecycle()` 吃不吃 `rr_gate`（它仍然不吃，見 `:136-142`）；
* ⛔ 不動 before 版（`ecbc141^`）——那是 Stage 2 的 tooling patch 範圍；
* ⛔ 不重產 bundle（見 [I-100](#i-100decision-replay-沒有-as-of-上界cohort-隔天就重現不了) 十九）；
* ⛔ **不跑正式 Stage 1 全掃**（v2 修正，見四）；
* ⛔ 不併入 [I-107](#i-107evaluationselection-用-tr-sma14runtime-用-wilder-atr14凍結門檻與-runtime-不同源)
  ——本次沒有重測門檻或改公式，兩筆維持獨立。

##### 一-B、受影響檔案（v3 補回——v2 改寫時漏掉了這張表）

| 檔案 | 變更 | 性質 |
|---|---|---|
| `lifecycle_engine.py` | `resolve_lifecycle()` 回傳新增 `clear_zone_breakout`、`continuation_price_evidence_met` | 純新增 |
| `decision_engine.py` | `_decision_semantic_pipeline()` 回傳新增 **4 鍵**（2 透傳 ＋ `setup_rr_qualified` ＋ `rr_decoupling_candidate`）；`_decision_summary_zone()` 補 `relative_volume` | 純新增 |
| `evaluation.py` | 匯出九個欄位；**primary zone 來源切換**（三個 consumer）；**no-zone fallback**；**Stage 1／2 fail-closed** | ⚠️ 含行為變更 |
| `frontend/src/lib/api/srZones.ts` | `SRSemanticPipeline` 補 4 鍵、`SRDecisionZoneSummary` 補 `relative_volume`（**僅型別，⛔ 不接線**，理由見八） | 純新增 |
| `replay_bundle/artifacts.py` | **九欄位 schema validator**（見六-C，含 `lifecycle_phase` 依賴與欄位交叉一致性）＋ **`candidate_mismatch.json` 的 builder 與 validator** ＋ **`CandidateMismatch` 例外**（見六-D） | 純新增 |
| `replay_bundle/__init__.py` | **匯出新的公開 API**：`CandidateMismatch`／`EXIT_CANDIDATE_MISMATCH`／mismatch 的 builder 與 validator／九欄位 validator。⚠️ `evaluation.py` 依既有 contract **只能從這裡取用**，⛔ 不直接 import 子模組 | 純新增 |
| `evaluation.py`（Stage 2） | **第五道集合檢查** ＋ 不一致時**發布 `candidate_mismatch.json`**；⚠️ `main()` 要在 generic `except ValueError` **之前**先 catch `CandidateMismatch` 並回 `EXIT_CANDIDATE_MISMATCH`（見六-D） | ⚠️ 行為變更 |
| `scripts/smoke-replay-offline.sh` | ⛔ **移除注入的假 candidate ＋ 非空 cohort 斷言**（見四-B）——不移除會造成 false pass／誤判失敗 | ⚠️ 行為變更 |
| `scripts/test-replay-args.sh` | **新增 exit 4 的 passthrough 測試**（fake `docker` → `exit 4`），見十的測試表 | 純新增 |
| 測試 | `test_lifecycle_engine.py`／`test_decision_engine.py`／`test_evaluation.py`／`test_replay_bundle_stages.py`／前端型別 fixture | 新增 |

**資料流**見三。

##### 二、⚠️ candidate 要用的是 **setup** RR gate，不是對外的 `rr_gate`

⛔ **v1 最嚴重的錯誤**：它宣稱
`rr_decoupling_candidate ≡ lifecycle_phase == "CONTINUATION" 且 rr_gate.qualified == false`，
但那兩個 `rr_gate` **不是同一個東西**：

```
:2678  rr_gate = _rr_gate(primary_zone, entry_action_state)          ← setup gate
           ↓ 傳進 _decision_derived_view → _decision_semantic_pipeline
        rr_qualified = bool(rr_gate.get("qualified"))   (:1051)      ← candidate 用的是這個
:2779  rr_gate = _execution_rr_gate(primary_zone, entry_action_state, rr_context, rr_gate)
           ↓ **覆寫**後才進 decision_summary["rr_gate"]              ← replay row 匯出的是這個
```

所以會出現 `rr_decoupling_candidate = true` 但 `replay row 的 rr_gate.qualified = true`，
**v1 宣稱的等價式在對外欄位上直接不成立**。

**定案：用 setup gate，並把它明確匯出成 `setup_rr_qualified`。**
理由是本筆要驗的是**歷史上被移除的那個條件**，而 before 版的 `rr_qualified` 同樣來自
setup gate（`ecbc141^:2441`，見上方「candidate 的精確定義」的查證）。⛔ 改用 execution gate
會碰到 T-065 的資料流環，超出 Stage 0 範圍。

```
rr_decoupling_candidate ≡ lifecycle_phase == "CONTINUATION" 且 setup_rr_qualified == false
```

⚠️ **歸檔時要寫明 `setup_rr_qualified ≠ rr_gate.qualified`**，否則下一個讀 replay row 的人
會拿對外那顆去驗等價式而得到矛盾。

##### 三、資料流：semantic pipeline 要**透傳**，⛔ 不是只加一個鍵

⛔ **v1 的第二個錯誤**：`_decision_semantic_pipeline()`（`:1046-1049`）目前只從 lifecycle
取三個鍵（`event_signal`／`lifecycle_phase`／`reason_codes`）。**在 `resolve_lifecycle()` 加
輸出鍵，值會在 lifecycle → decision summary 之間直接消失**，`evaluation.py` 就做不到
「只匯出、不重算」。

```
resolve_lifecycle()  新增 2 鍵：clear_zone_breakout / continuation_price_evidence_met
        ↓  ⚠️ semantic pipeline **必須逐鍵透傳**，⛔ 不透傳就斷在這裡
_decision_semantic_pipeline()  新增 4 鍵：上面 2 個（透傳）
                                        ＋ setup_rr_qualified（透傳自 rr_gate）
                                        ＋ rr_decoupling_candidate（組合）
        ↓
decision_derived_view.semantic_pipeline → decision_summary
        ↓
evaluation._decision_fields_from_summary()  ← **只匯出，不重算**
```

**測試要有 pass-through identity 斷言**：lifecycle 回傳什麼、semantic pipeline 就有什麼，
⛔ 不允許中途重算。

##### 四、⚠️ 端到端驗證只用 smoke bundle，⛔ 不跑正式 Stage 1

⛔ **v1 自相矛盾**：開頭說「Stage 1／2 不在範圍」，第六節卻要求拿**正式 frozen bundle**
跑官方 Stage 1。那**就是**本筆唯一一次正式 after 全掃，⛔ 不能因為叫它「測試」就不計次。

**定案**（⚠️ v6 改寫——v5 的舊表寫「I-100 跨日不計、之後另跑正式 Stage 1」，
與下方**已確認**的政策直接衝突，⛔ 已刪除，避免執行時出現兩套真相）：

| 用途 | 用哪份 bundle | 計不計入「一次正式 scan」 |
|---|---|---|
| Stage 0 的端到端驗證 | **小型 smoke bundle**（`python/scripts/make_smoke_bundle.py`） | ⛔ 不計 |
| **D／D+1 的跨日 replay** | 正式 frozen bundle | ✅ **兩趟合起來算一次**——D+1 的 after artifact **就是** I-074 的正式 Stage 1 artifact |
| 第三趟 Stage 1 | — | ⛔ **不執行** |

I-100 關閉條件 2 要求「不同日期載入同一份 bundle 得到相同逐列結果」，那必然要用正式
bundle 跑兩趟 replay——而那兩趟**同時就是** I-074 的正式 scan。

**✅ 使用者已於 2026-09-11 明文裁決**（採納 review 的提議，比 v3 更省一趟）：
⛔ **不要跑完 I-100 的兩趟之後再跑第三趟相同的 Stage 1**——那三趟的 input／code／predicate
完全相同，第三趟不產生任何新資訊，卻要多燒約 3.7 小時。

| 條款 | 內容 |
|---|---|
| 性質 | 兩次跨日執行視為**同一組固定 input／code／predicate 的 deterministic replay** |
| 產物 | **D+1 驗證通過的 after artifact 直接作為 I-074 的正式 Stage 1 artifact** |
| 計次 | 這一組**只算一次**正式 scan |
| ⛔ 硬性限制 | **期間不得修改任何 predicate 或條件**；第二趟**只驗重現性**，⛔ 不得依其結果調整任何東西——那樣才不構成「結果導向重跑」 |
| ⛔ 硬性限制（2） | **兩次執行期間不得修改 predicate、程式碼或其他判定條件** |
| 失敗時 | 兩趟逐列結果不一致 → **必須立案調查**，⛔ **不得以重新執行覆蓋或取代失敗結果** |

✅ **已確認，可以據此執行**（使用者 2026-09-11 的五條裁決原文即上表）。

##### 四-B、⛔ 必須移除 smoke 腳本注入的假 candidate（v3 新增）

`scripts/smoke-replay-offline.sh` 目前用 tooling patch 往 replay row 注入
`"rr_decoupling_candidate": bool(idx % 7 == 0)`——那是 I-100 時期的**欄位替身**，
因為當時產品端還沒有這個欄位。

⛔ **Stage 0 完成後它會變成 false pass 的來源**：產品端開始產出真欄位，而 patch 注入的
同名鍵在同一個 dict literal 裡**排在後面會覆蓋掉它**——於是即使正式資料流壞掉，
端到端測試照樣綠燈。

**定案**：

1. ⛔ **移除 smoke patch 裡的 candidate 注入**（連同那段 `[smoke-only]` 註解），
   Stage 1 改用**真實 candidate** 驗欄位；
2. ⛔ **同時移除 `scripts/smoke-replay-offline.sh:205` 的
   `assert cohort["keys"]`**（v4 新增——⚠️ v3 只講移除注入，**漏了這一條**：
   注入移除後合成資料自然零命中，Stage 1／2 明明都成功，最後仍會被這條斷言判失敗）；
3. **空 cohort 時改驗「合法空集合」**：`comparison_artifact.rows == []`、
   `report.candidate_rows == 0`，且兩份 artifact **仍須產出**；
4. **「非空 cohort 的 Stage 2 比較路徑」改由既有的 pytest 涵蓋**
   （`test_replay_bundle_stages.py` 的 stub 是 **artifact layer** 的測試，
   ⛔ 不經過產品資料流，不會污染 candidate）；
5. ⛔ **不為了讓 smoke 命中而改造 predicate 或合成輸入的判定條件**——那等於違反頭號禁止事項。

##### 五、⚠️ 正式掃描必須 fail-closed，⛔ 不得把運算失敗變成零候選

⛔ **v1 完全沒處理**：`evaluation.py:1561` 對每一列 `except Exception` 後**繼續**
（`# one replay row must not abort the whole report`），而 I-100 的 Stage 1 守門
（`artifacts.py` 的 `validate_candidate_flags`）**只驗 candidate 是不是嚴格 boolean**。
若實作為了讓每列都有 boolean 而把錯誤列填成 `false`，**大量運算失敗也會產出一份合法的
空 cohort**，然後被誤判成分支 A 而關閉本筆。

**定案**：

**分三層，⛔ 不要把正式 bundle 的數字硬編進通用路徑**（v3 修正——v2 只寫「正式 Stage 1」，
而 Stage 2 走的是同一條會吞 exception 的 replay 路徑，before 出錯同樣不該產出比較 artifact；
且把 13417 寫死在通用檢查裡，小型 smoke bundle 會必然失敗）：

| 層 | 適用 | 規則 |
|---|---|---|
| **① 運算完整性**（通用） | **Stage 1 與 Stage 2 都是** | 除 `NO_ZONE_SCORES` 外，任一 `decision_error`／`zone_score_error` → **在發布 artifact 之前中止** |
| **② 範圍完整性**（通用） | Stage 1 與 Stage 2 | 實際 row keys **必須等於 bundle 推導出的 universe**（I-100 的 ① 已有，這裡重申它也擋運算缺列） |
| **③ 正式 bundle preflight**（專屬） | 只在正式 frozen bundle 上 | `bundle_id == b1_20260901_1d_74350966_5d7ecb10` 且 eligible 列數 **== 13417** |

⚠️ **③ 必須在 `_decision_replay_rows()` 之前完成**（v4 新增）——跑完約 3.7 小時才發現
bundle 拿錯或列數不對，那就不叫 preflight 了。eligible 列數由 `_bundle_universe_keys()`
算得出來，⛔ 不需要先跑 replay。

⛔ **I-074 的固定 bundle ID 不得硬編進通用 evaluation contract**：它是**這一筆**的執行參數，
不是 replay 工具的性質。

**落點定案（v5）：③ ⛔ 不是 Stage 0 的程式交付，延到 Stage 1 計畫。**
理由是 Stage 0 的範圍是診斷欄位；bundle ID 與 13417 是**執行那一次全掃時**的前置檢查，
屬於 Stage 1 的執行入口。⚠️ **①②（運算完整性、範圍完整性）仍是 Stage 0 的交付**——
它們是通用的 fail-closed 行為，與哪一份 bundle 無關。
⛔ 因此一-B 的檔案表**不含**任何專屬 preflight 入口。

⚠️ **「合法零 zone」與「運算失敗」要分開**：`zone_score_error == "NO_ZONE_SCORES"` 是合法的
——那一列本來就沒有 zone，`candidate = false` 是**真實的 false**。⛔ 其餘任何 error 值都是
運算失敗。13417 的算式是 `14352 − 11 × (min_history_bars 80 + forward_bars 5)`。

**測試**：注入 decision／zone exception → 斷言 **Stage 1 與 Stage 2 都不發布 artifact**；
合法零 zone → 斷言正常產出且該列 `candidate = false`；
小型 smoke bundle → 斷言**不會**因為列數不是 13417 而失敗。

##### 六、精確 schema（v2 新增——⛔ v1 的「七個欄位都是 boolean」是錯的）

⚠️ v1 的欄位表有 8 列但文字寫「七個」，又說「七個欄位都是嚴格 boolean」——**九欄位裡
只有 4 個是 boolean**（v7 更正：v2 寫「3 個」，但 `setup_rr_qualified` 就是 v2 自己加的，
數字當下就已經過期），其餘是字串與巢狀物件。

| JSON path（replay row） | 型別 | nullable | 來源 | 不可用時 |
|---|---|---|---|---|
| `clear_zone_breakout` | `bool` | ⛔ 否 | lifecycle `:161` | 無 primary zone → `false` |
| `continuation_price_evidence_met` | `bool` | ⛔ 否 | lifecycle（三項證據合取） | 同上 → `false` |
| `setup_rr_qualified` | `bool` | ⛔ 否 | `_rr_gate()` 的 `qualified`（**setup**） | 無 gate → `false` |
| `rr_decoupling_candidate` | `bool` | ⛔ 否 | semantic pipeline 組合 | 同上 → `false` |
| `event_signal` | `str` | 否 | semantic pipeline（已有） | `"NO_EVENT"` |
| `structure_state` | `str` | 否 | ⚠️ **`position_action_condition.structure_state`**（v3 修正——`build_decision_summary` 的 top-level **沒有**這個欄位，它只存在於 `_position_action_condition()` 的回傳，`decision_engine.py:210/233/242/250`） | `"UNKNOWN"` |
| `action_state` | `str` | 否 | semantic pipeline（已有） | `"WATCH"` |
| `position_action_condition` | `object` | ✅ 是 | `decision_engine.py:2922` | `null` |
| `position_action` | `str` | ✅ 是 | `:2921`；⚠️ **只記錄不判定** | `null` |

⚠️ **`position_action_condition` 保留完整物件，⛔ 不扁平化成單一 `state`**：它的
`invalidation_price`／`recovery_price`／`reason_codes` 是判讀分支 B／C 時要看的佐證，
扁平化會在需要時再補一次。`structure_state` 也從這個物件取，⛔ 不另設第二個來源。

##### 六-B、no-zone 列的「合法缺席 fallback」（v3 新增）

⛔ **v2 的規格自相矛盾**：`evaluation.py:1511` 只有在 `zone_score_available` 且
trend／volatility／metrics 都在時才呼叫 `build_decision_summary()`，所以
**`NO_ZONE_SCORES` 的列根本不會進 lifecycle 或 semantic pipeline**。
於是「只匯出不重算」「九個欄位皆有非 nullable 預設」「no-zone 列得到
`candidate=false` 與 `lifecycle_phase=NO_PRIMARY_ZONE`」**三件事無法同時成立**——
實作者沒有合法路徑產生 strict boolean。

**定案：這是 replay **serialization 層**的合法缺席 fallback，⛔ 不是重算上游判斷。**
語意是「這一列沒有 decision summary 可匯出」，而不是「我替它算了一個答案」。

⚠️ **但觸發條件必須收窄到「真的沒有 zone」**（v4 修正——v3 寫「沒有 decision summary 時」
**過寬**）：`:1524` 的守門是**四個條件的合取**

```
zone_score_available and global_trend is not None
                     and global_volatility is not None and global_metrics
```

所以「沒有 decision summary」還包含**zone 存在、但 trend／volatility／metrics 不完整**。
⛔ **那不是合法缺席，是異常**，套上九個正常預設值等於把運算問題偽裝成正常結果。

| 情況 | 處置 |
|---|---|
| `zone_score_available == false` **且** `zone_score_error == "NO_ZONE_SCORES"` | ✅ **套下表的合法 fallback** |
| 其餘任何「沒有 decision summary」（zone 在、但 trend／volatility／metrics 缺） | ⛔ **建立明確錯誤並 fail-closed**（歸入五-① 的運算完整性） |

`evaluation.py` 只在**第一種**情況填入下表的固定值：

| 欄位 | no-zone 時的值 | 理由 |
|---|---|---|
| `clear_zone_breakout` | `false` | 沒有 primary zone 就不可能有突破 |
| `continuation_price_evidence_met` | `false` | 三項證據之一（`clear_zone_breakout`）必然不成立 |
| `setup_rr_qualified` | `false` | 沒有 zone 就沒有 setup gate |
| `rr_decoupling_candidate` | `false` | ⚠️ **真實的 false**，⛔ 不是「不知道」 |
| `event_signal` | `"NO_EVENT"` | 與 `resolve_event_signal()` 的預設一致 |
| `structure_state` | `"UNKNOWN"` | ⛔ 不猜 |
| `action_state` | `"WATCH"` | 與 semantic pipeline 的預設一致 |
| `position_action_condition` | `null` | ✅ nullable |
| `position_action` | `null` | ✅ nullable |
| `lifecycle_phase`（**既有欄位**） | ⚠️ **維持 `null`** | v4 定案——`:1498` 的既有預設就是 `None`，而 no-zone 列**根本不會進 lifecycle**。⛔ 不補成 `NO_PRIMARY_ZONE`：那是 evaluation 替上游推導答案，違反「只匯出不重算」。⚠️ v3 內文誤寫「必然是 `NO_PRIMARY_ZONE`」，已改正（那句話只在**真的進了 lifecycle** 時才成立） |

⚠️ **`lifecycle_phase = null` 與 `candidate = false` 不矛盾**：candidate 的定義是
`lifecycle_phase == "CONTINUATION" 且 setup_rr_qualified == false`，
`null != "CONTINUATION"` → `false`，兩者一致。

⚠️ **同時要有一個可辨識的旗標**：replay row 既有的 `zone_score_available=false` ＋
`zone_score_error="NO_ZONE_SCORES"` 就是那個旗標，Stage 2 判讀時要能把這類列與
「真的算過而得到 false」區分開。**測試**：no-zone 列的九個欄位 ＋ `lifecycle_phase`
逐一斷言到值，且斷言 `zone_score_available is False`；
**另加對照組**：zone 在但 `global_metrics` 為空 → 斷言**中止**，⛔ 不得套 fallback。

##### 六-C、九欄位的 runtime schema validator（v5 新增）

⛔ **v4 為止的缺口**：Stage 1 發布前只呼叫 `validate_candidate_flags()`
（`evaluation.py:2923`），載入 after artifact 時也只驗 candidate
（`replay_bundle/artifacts.py:123`）。**少了 `action_state`／`position_action_condition`／
`structure_state`，或型別錯了，正式 artifact 照樣發布**——而那些正是之後判讀分支 B／C 要用的
欄位，等到判讀時才發現就來不及了（那份 artifact 是唯一一次正式 scan 的產物）。

**定案：新增診斷欄位的 schema validator，三個時點都要過。**

| 時點 | 驗什麼 |
|---|---|
| **Stage 1 發布前** | 本次 replay 的每一列都要通過九欄位驗證，⛔ 通過才發布 |
| **載入 after artifact** | 再驗一次（⚠️ 檔案可能來自別處或被改過） |
| **Stage 2 的 before rows** | ⚠️ **也要驗**，⛔ 通過才進比較與發布 |

**逐欄位規則**（型別見六的 schema 表）：

* 四個 boolean 欄位一律 `isinstance(x, bool)`，⛔ 不收 `0`／`1`／`"true"`／`None`；
* `event_signal`／`structure_state`／`action_state` 必須是**非空字串**；
* `position_action_condition` **必須是 object**，且至少驗 `state` 與 `structure_state` 兩鍵；
  ⚠️ **只有明確的 no-zone fallback 列**（`zone_score_available == false` 且
  `zone_score_error == "NO_ZONE_SCORES"`）才允許為 `null`；
* **既有依賴欄位 `lifecycle_phase`**（v7 新增——⛔ 少了它，等價式會被空值蒙混過去）：
  after 的等價式 `candidate == (lifecycle_phase == "CONTINUATION" and not setup_rr_qualified)`
  **依賴 `lifecycle_phase` 存在**。若它缺失而 candidate 是 `false`，
  `false == (None == "CONTINUATION" and …)` → `false == false` → **照樣通過**。

  | 列型 | `lifecycle_phase` 規則 |
  |---|---|
  | 一般 decision row | **必須是非空字串** |
  | 精確的 no-zone fallback 列 | **必須是 `null`**（見六-B） |
  | 欄位缺失（key 不存在） | ⛔ **一律 fail-closed** |

* **欄位交叉一致性**（v6 新增——⛔ 少了它，互相矛盾的證據仍會通過）：

  ```
  structure_state == position_action_condition.structure_state
  action_state    == position_action_condition.state
  ```

  ⚠️ 第二條有一個邊界：`_position_action_condition()`（`decision_engine.py:205`）寫的是
  `str(semantic_pipeline.get("action_state") or "WATCH")`，所以 `action_state` 為**空字串**時
  兩者會分岔成 `""` vs `"WATCH"`。**驗證規則要寫成「`action_state` 非空時必須相等」**，
  而空字串本身已被「非空字串」那條擋掉。

* ⚠️ **等價式的驗法依版本分流，⛔ 不可錯套——但 before 的展開式⛔ 不在 artifact validator 裡**：

  | 列來自 | artifact validator 驗什麼 | 展開式在哪裡驗 |
  |---|---|---|
  | **after** | ✅ `candidate == (lifecycle_phase == "CONTINUATION" and not setup_rr_qualified)` | 同一處 |
  | **before** | ⚠️ **只驗 schema 與欄位交叉一致性**，⛔ **不重算展開式** | **before tooling 內**（見下） |

  ⛔ **v5 的錯誤**：它要求 artifact validator 對 before row 用展開式重算，但展開式含
  **`active_bearish_states`**（見主文「before 版的等價 flag」），而那**不在九欄位裡**——
  它是 `event_state_summary.active_bearish_events`，是 lifecycle 的**獨立輸入**
  （`lifecycle_engine.py:150`）。**光靠 row 根本算不出來**，那個承諾兌現不了。

  **定案**：展開式在 **before tooling 內**驗證——那裡**還持有原始 `event_state_summary`**，
  算得出 `active_bearish_states`。⚠️ 這是 **Stage 2 的 patch 責任**，⛔ 不是 Stage 0 的交付。

##### 六-D、Stage 2 必須比對**兩邊**的候選集合（v6 新增）

⛔ **現行 Stage 2 只用 after 的 cohort 過濾比較**（`evaluation.py:2960` 的
`for key in sorted(cohort_keys)`），所以**before 多出來的候選會被靜默漏掉**。

⚠️ 而 I-074 主文明寫「**兩邊算出來的 candidate 集合必須完全相同；不相同本身就是分支 C**」
——漏掉 before 側，等於把分支 C 的其中一種形態變成看不見。

**定案：Stage 2 新增第五道集合檢查**（I-100 的四道之外）：

```
⑤ sorted(candidate_keys(before_rows)) == sorted(candidate_keys(after_rows))
```

⛔ **不相等即中止**。⚠️ 但 v6 只寫「保留差集」是不夠的——`assert_same_keys()` 只把**前五筆**
放進錯誤訊息然後 raise，差集最後只會出現在 stderr。**而那個差集本身就是分支 C 的證據**，
與「完整 artifact 是關閉證據」的要求直接衝突。

**定案（v7）：發布專屬的 mismatch artifact，⛔ 不只印訊息。**

| 項目 | 內容 |
|---|---|
| 檔名 | `candidate_mismatch.json`（與其他 artifact 同一個 `--output-dir`） |
| `kind` | `sr_zone_replay_candidate_mismatch`，`schema_version: 1` |
| 內容 | `before_only`／`after_only` 的**完整** key 列表（⛔ **不截斷**）＋ 兩邊候選數 ＋ `bundle_id`／**`after_artifact_sha256`**／`before_ref`／`generated_at`／`provenance` |
| ⚠️ **差集各列的完整 row**（v9 新增，v10／v11 兩度改形狀，⛔ 不可省） | **單一 `rows` 列表，直接重用 `compare_rows()`**（v11 定案）：<br>`mismatch_keys = sorted(set(before_only) \| set(after_only))`<br>`rows = [compare_rows(before_by_key[k], after_by_key[k]) for k in mismatch_keys]`<br>所以每一項的形狀就是 **`{symbol, timeframe, as_of, differences, before, after}`**（`artifacts.py:229`），⛔ **不是** v10 寫的 `{key, before, after}` wrapper。<br>⚠️ **選它而不是自訂 wrapper 的理由**：①`differences` 是免費得到的，而它**直接告訴你兩邊差在哪些欄位**——那正是調查 tooling 不對稱要看的；②與 comparison artifact 真正同形狀，判讀工具可共用；③少維護一套形狀。<br>⛔ **不用 `before_rows`／`after_rows` 兩份 dict**（v10 修正 v9——那個形狀讓「各自等於對應的差集」這種讀法成立，照它實作就只存了各一半）。⛔ 只存 key 的話，`before` 的 `lifecycle_phase`／`setup_rr_qualified`／`event_signal`／`structure_state` 全都隨行程消失——**Stage 2 的 before rows 只存在記憶體**，正常路徑靠 comparison artifact 保存（`evaluation.py:2963`），而 mismatch 路徑明訂⛔ 不產 comparison。⚠️ 那是一次 3 小時以上、**⛔ 不允許用重跑取代結果**的正式 replay：查不出 tooling 為何不對稱，就只剩一次不該發生的重跑。⚠️ **after row 雖然理論上能由 `after_artifact_sha256` 找回，仍一併保存**——差集本來就小，自足的證據不必依賴另一份檔案還在手上 |
| ⚠️ **`after_artifact_sha256` 不可省**（v8 補） | Stage 2 的 `provenance` 記的是 **before worktree**，而 `bundle_id` 只證明「同一份輸入」，**證明不了 `after_only` 是由哪一份 after artifact 算出來的**。comparison artifact 已經有這個欄位（`evaluation.py:2973`），mismatch 沒理由比它弱 |
| 不變條件 | `before_only`／`after_only` **各自排序且 key 唯一**；兩者**互斥**；`before_candidate_count − after_candidate_count == len(before_only) − len(after_only)`——⛔ 數字與差集內容對不上即中止 |
| **精確 schema** | ⚠️ 見表格後的「`candidate_mismatch.json` 的精確 schema」——⛔ 它含 code fence 與多段說明，**塞在表格列裡會把整張表從這裡截斷**（v13 修正） |
| 發布 | 走既有的原子發布；**發布完成後**才回**專屬非零結束碼** |
| ⚠️ **結束碼與例外型別**（v8 定案） | `EXIT_CANDIDATE_MISMATCH = 4`（既有：1＝一般中止、3＝durability 未確認）。⛔ **專屬例外 `CandidateMismatch` 不得繼承 `ValueError`**——`ArtifactError` 就是 `ValueError` 的子類（`artifacts.py:44`），而 CLI 的 bundle 分支是 `except (CliUsageError, ValueError, OSError) → sys.exit(1)`（`evaluation.py:3219`），繼承下去**專屬碼根本出不來**。`main()` 要在那個 generic catch **之前**先處理它 |
| ⛔ 不產出 | `comparison_artifact.json` 與 `report.json`——⚠️ 這是**預期的終止狀態**，不是失敗殘骸 |
| SHA-256 | 結束訊息印出該檔的 SHA-256，比照其他 artifact 當證據引用 |

###### `candidate_mismatch.json` 的精確 schema

⚠️ **v13 把這一段從表格列移出來**：它含 fenced code block 與多段說明，
而 GitHub Markdown 的表格列**必須是單一實體行**——原本的寫法會讓整張表從這裡截斷，
後面的「發布／結束碼／不產出／SHA-256」就不再屬於同一張表。

**top-level 封閉欄位集合**（⛔ 多欄或缺欄一律拒絕）：`schema_version`／`kind`／`bundle_id`／`after_artifact_sha256`／`before_ref`／`generated_at`／`provenance`／`before_only`／`after_only`／`before_candidate_count`／`after_candidate_count`／`rows`。<br>**型別**：`bundle_id`／`before_ref`／`generated_at` 為**非空字串**；`provenance` 為 **object**；`after_artifact_sha256` 為 **64 字元小寫 hex**；兩個 count 用 **`type(x) is int` 且 >= 0**（⚠️ 排除 `bool`——它是 `int` 的子類，沿用 I-100 既有慣例）。<br>**差集**：`before_only ∪ after_only` **必須非空**（⛔ 擋掉「零差集卻宣稱 mismatch」）；兩者互斥；各自排序且 key 唯一；每個 key 是**三個非空字串**。<br>**`rows`**：⚠️ **有序契約寫成可執行的斷言**——`mismatch_keys = sorted(set(before_only) | set(after_only))`，然後 **`[row_key(item) for item in rows] == mismatch_keys`**（v12 修正：v11 寫「key 集合恰好等於…且**同順序**」，但**集合本身沒有順序**，那句話不可執行）；⛔ **`rows[]` 也是封閉欄位集合**（`symbol`／`timeframe`／`as_of`／`differences`／`before`／`after`）；⚠️ 每一列 **`row_key(item) == row_key(item["before"]) == row_key(item["after"])`**——⛔ 外層三欄與內嵌兩側 row 的 key 必須完全一致；`before`／`after` 都必須是 object，**⛔ 不得為 `null`**（第五道檢查排在②之後，②已保證兩邊 row key 集合相同，所以差集裡每個 key **兩邊一定都有 row**；⚠️「那一側不是候選」⛔ 不等於「那一側沒有 row」）。<br>⚠️ **`differences` 的內容契約**（v12 新增，⛔ 不可只列進封閉欄位就算數）：

它在 v11 被扶正成判讀 tooling 不對稱的**正式證據**，所以必須與 `compare_rows()`
（`artifacts.py:229`）**算出完全相同的值**，⛔ 不接受空陣列、錯誤欄位名、重複或未排序：

```
expected = sorted(f for f in set(before) | set(after) if before.get(f) != after.get(f))
item["differences"] == expected
```

⚠️ **另有一條必然成立的不變條件**：mismatch key 來自候選集合的 **symmetric difference**，
所以那一列的 `rr_decoupling_candidate` 在兩側**必然不同** → **`"rr_decoupling_candidate"`
一定會出現在 `differences` 裡**。⛔ 不在就是產物本身壞了，publish 前即拒絕。

⚠️ **數量下界**（v11 新增，⛔ 「數量差相等」取代不了）：**`before_candidate_count >= len(before_only)`** 且 **`after_candidate_count >= len(after_only)`**。⛔ 少了它，「兩側 count 都是 0、兩側差集各有一筆」會通過差值公式（`0−0 == 1−1`），但那在集合上**根本不可能成立**。

⚠️ **為什麼⛔ 不改用「兩邊候選的聯集」產 comparison**（review 提的另一個選項）：
I-100 的**第四道集合檢查**是 `keys(comparison) == keys(cohort_manifest)`，而
`cohort_manifest` 是 Stage 1 產的、**只含 after 側候選**。改用聯集會讓第四道必然不成立，
等於要動 I-100 已收斂的規格。**新增一份 artifact 比鬆動既有契約安全。**

這道檢查排在比較與發布**之前**。

**⚠️ 執行順序定死（v10 新增）**——⛔ validator 只是「存在」保護不了任何東西，
必須被流程**實際呼叫**，而且要在**發布之前**：

```
① 第五道集合檢查不通過
② build mismatch artifact
③ **validate mismatch**（封閉 schema ＋ 全部不變條件 ＋ 對照實際的 before／after rows
   與 after_artifact_sha256）        ← ⛔ 沒過就不准往下走
④ publish_artifacts（原子發布）
⑤ raise CandidateMismatch
⑥ CLI 回 EXIT_CANDIDATE_MISMATCH = 4
```

**測試**：讓 ③ 失敗 → 斷言 **`candidate_mismatch.json` 不存在**（⛔ 不得留下半份沒通過
驗證的證據），且結束碼是**一般中止 1**（⚠️ ⛔ 不是 4——4 的語意是「mismatch 已經被完整
記錄下來」，驗證失敗時那個承諾並沒有兌現）。

⚠️ **`replay_bundle/artifacts.py` 與 `evaluation.py` 因此都列入受影響檔案**（見一-B）。

##### 七、primary zone 修正：三個 consumer 要**一起**切

⛔ **v1 只說改取值來源，漏了下游**。同一顆 `primary_zone` 現在同時供三處使用：

| consumer | 位置 |
|---|---|
| replay row 的 `primary_zone` | `evaluation.py:1519` |
| `daily_confirmation_context` | `:1078` |
| `daily_confirmation_outcome`（含 `_excursion_window`） | `:1215` |

⚠️ **三處必須一起切到 decision primary**，否則同一列會混用兩顆 zone。

⛔ **另一個 v1 漏掉的**：`_decision_summary_zone()`（`decision_engine.py:112`）**沒有
`relative_volume`**，而 `evaluation.py:1006` 的 `_volume_strength_bucket()` 正是讀它。
直接切過去會讓 volume context **靜默退化成 unavailable**。

**定案**：`_decision_summary_zone()` 補 `relative_volume`（⚠️ 純新增欄位），
前端 `SRDecisionZoneSummary` 同步補 optional 型別。
**測試**：構造「排序第一顆 ≠ decision primary」的案例，斷言三處的 zone identity 一致。

##### 八、前端處置

依 `development-workflow.md`「新增 `decision_summary` 欄位，把『前端消費』納入完成定義」
逐欄位標記：**全部 ⛔ 不接線，但補 TS 型別**（`SRSemanticPipeline` 加 4 鍵、
`SRDecisionZoneSummary` 加 `relative_volume`）。

理由：這些是**驗證用的診斷欄位**，不是給使用者判讀的資訊，渲染只會增加噪音；
⚠️ 但型別必須補——⛔ 不補會讓型別與實際回應分岔，而那正是該節記錄過**踩過兩次**的坑。
⚠️ fixture **從真實輸出取樣**，⛔ 不憑記憶手寫。

##### 九、contract 變化與回滾

**向後相容，⛔ 不需要 migration**：Go 端把 `decision_derived_view` 當 **raw JSON** 存
（`analysis/client.go:537` 的 `decisionRawJSONAt`），**不解析欄位**；前端
`SRSemanticPipeline`（`srZones.ts:720`）全是 optional；DB 存 JSON 欄位。

**回滾**（v3 精確化——⛔ v2 的「純新增欄位，移除即回滾」不成立）：

| 變更 | 性質 | 回滾 |
|---|---|---|
| lifecycle／decision 的新增輸出鍵 | **純新增** | 移除即回滾 |
| primary zone 來源切換（三個 consumer ＋ `relative_volume`） | ⚠️ **行為變更** | 要改回舊來源，且既有統計不可與新的直接比較 |
| Stage 1／2 的 fail-closed 控制流 | ⚠️ **行為變更** | 拿掉守門等於恢復吞錯誤 |
| no-zone fallback | ⚠️ **新增行為** | 移除會讓那些列沒有欄位 |

⚠️ 已凍結的 bundle 不受影響——它凍的是輸入。

⚠️ **一個不可回滾的副作用**：primary zone 換來源會改變
`primary_zone_role_counts` 等既有統計，**新舊 replay report 不可直接比較**，
歸檔時要寫明分界點。

##### 十、測試與驗證策略

| 層 | 內容 |
|---|---|
| `lifecycle_engine` | `clear_zone_breakout` 真／假各一；`continuation_price_evidence_met` 三項證據**逐項缺一**共四條；⚠️ 既有 15 條逐鍵斷言全部不變即通過（已查證⛔ 無 exact-dict 比較） |
| `decision_engine` | `rr_decoupling_candidate` 的四格真值表；⚠️ **等價性斷言**只含兩個條件；**pass-through identity**：lifecycle 的 2 鍵原值出現在 semantic pipeline；`setup_rr_qualified` **≠** 對外 `rr_gate.qualified` 的對照案例 |
| `evaluation` | 九個欄位的型別逐一斷言（⛔ 不是「都是 boolean」）；primary zone 三處 identity 一致；**注入 exception → 不發布 artifact**；**合法零 zone → 正常產出且 candidate=false** |
| **schema validator** | 九欄位逐一的缺欄位／型別錯各一條；`position_action_condition` 為 `null` 時**只有 no-zone fallback 列**可通過；**欄位交叉一致性**兩條（含 `action_state` 空字串的邊界）；⚠️ **after 驗等價式、before ⛔ 不驗展開式**各一條；Stage 1 發布前／載入 after／Stage 2 的 before rows **三個時點各一條** |
| **第五道集合檢查** | before 多一個候選 → **發布 `candidate_mismatch.json` 後回專屬碼**；before 少一個 → 同樣；兩邊相同 → 通過。⚠️ 斷言 ①差集**完整**（⛔ 不截斷）且排序唯一、②`comparison_artifact.json` 與 `report.json` **不產出**、③它排在比較與發布**之前**、④`after_artifact_sha256` 與實際載入的那份相符、⑤候選數與差集的**差值公式與下界**都成立 |
| **mismatch 的 `rows` contract**（v11 新增） | ①`rows` 的 key **恰好涵蓋差集聯集**且**依固定順序**；②**before-only 與 after-only 的 key 都同時保存兩側完整 row**（⚠️ 這是 v9→v11 反覆修的那一條，要有專門測試釘死）；③`row_key(item) == row_key(item["before"]) == row_key(item["after"])`；④`before`／`after` **缺失／`null`／型別錯／key 不符** → **validator 在 publish 前拒絕**；⑤**top-level 或 `rows[]` 多欄／缺欄** → 拒絕；⑥「兩側 count 都是 0、兩側差集各一筆」→ **被下界擋掉**（⛔ 差值公式放它過） |
| **`differences` 的內容**（v12 新增） | ①與 `compare_rows()` 算出的值**逐項相同**；②**tampering 參數化**：`differences` 被**刪欄／加欄／重複／改順序**四種各一組 → **publish 前拒絕**；③⚠️ 每一列都斷言 **`"rr_decoupling_candidate" in differences`**（symmetric difference 的必然結果），不在即拒絕 |
| **專屬結束碼** | `CandidateMismatch` ⛔ **不得被 generic `except ValueError` 吃掉**——斷言 CLI 回 **4** 而不是 1 |
| **exit 4 的 passthrough**（v9 定案落點，v10 修正 fake 的行為） | ⚠️ **smoke 兩側都跑同一個 HEAD，正常⛔ 不會 mismatch，驗不到這條** → 改在 **`scripts/test-replay-args.sh`** 用**可控的 fake `docker`**（放在 `PATH` 前面）驗 `run-replay-offline.sh` **原樣傳出 4**。⛔ **fake 不能「一律 exit 4」**（v10 修正——腳本的順序是 `docker build`（`:143`）→ `docker image inspect`（`:144`）→ `exec docker run`（`:176`），一律回 4 的話**在 build 就退出**，測試看到 4 卻**完全沒驗到 passthrough**，是不折不扣的 false pass）。**fake 要依子命令分流**：`build` → **0**；`image inspect` → 印出合法 digest 並回 **0**；`run` → **記錄自己被呼叫過**再回 **4**。⚠️ **測試必須同時斷言「確實執行到 `docker run`」**，⛔ 只看結束碼不夠。⛔ **不為了測 exit code 而去製造真的 mismatch**——那要讓 before／after 產出不同候選，等於動判定條件 |
| **`lifecycle_phase` 依賴** | 一般 row 缺 `lifecycle_phase` → 中止（⚠️ **即使 candidate 是 `false` 也要中止**，那正是 v6 會漏掉的情況）；no-zone fallback 列為 `null` → 通過；一般 row 為 `null` → 中止 |
| 前端 | 型別 fixture 從真實輸出取樣；⛔ 不新增渲染斷言 |
| **端到端** | **只用 smoke bundle** 跑官方 Stage 1（見四）；⛔ 不碰正式 frozen bundle；⚠️ smoke 改用**真實 candidate**（見四-B），空 cohort 視為合法通過 |

##### 十一、完成後歸檔位置

* [`sr-zone-scoring.md`](./sr-zone-scoring.md)——九個診斷欄位的 schema 表、責任層、
  ⚠️ **`setup_rr_qualified` 與對外 `rr_gate.qualified` 的差別**、
  `rr_decoupling_candidate` 的精確定義、**primary zone 取值來源修正的分界點**、
  以及前端顯式不接線的決定。
* **新的 terminal contract**（v10 補——⛔ v9 的歸檔清單漏了整組）：
  **`candidate_mismatch.json` 的 schema**、**Stage 2 的兩種 terminal outcome**、
  **`EXIT_CANDIDATE_MISMATCH = 4`**，以及 **mismatch 時⛔ 不產 comparison／report 的規則**。
  ⚠️ 後三項同時要寫進 [`development-workflow.md`](./development-workflow.md) 的驗收章節
  ——那是操作者會查的地方。

##### 十二、輸入已凍結

Stage 1／2 一律用 **`python/baselines/b1_20260901_1d_74350966_5d7ecb10`**
（2026-09-11 產，11 檔 × as-of 2026-09-01，14352 列 → **13417 列 eligible**）。
⛔ **不得重產**——理由見 I-100 十九。

⚠️ **狀態：八個檔案已 `git add`（staged），但尚未 commit**（v3 更新——v2 寫的
「untracked、可能被 `git clean` 刪除」已過期）。staged 已經避開 `git clean` 的風險，
但**在 commit 之前仍不算正式進版控**，主計畫要求的是「產出**並進版控**」。

##### 修訂紀錄

| 版本 | 變更 |
|---|---|
| v1 | 初版：三層責任、七欄位、contract 相容性查證 |
| v2 | review 抓到 4 高 3 中，全部反映：①**RR gate 用錯一個**——candidate 用的 `rr_qualified` 來自 setup gate（`:2678`），而 replay row 的 `rr_gate` 已被 execution gate 覆寫（`:2779`），v1 宣稱的等價式不成立 → 改用並明確匯出 `setup_rr_qualified`；②**lifecycle 新增的欄位流不到 replay**——semantic pipeline 只取三個既有鍵 → 改為逐鍵透傳，共新增 4 鍵並補 pass-through identity 測試；③**運算失敗會被誤判成零候選**——逐列 `except` 吞錯誤 ＋ 守門只驗 boolean → 正式掃描對任一 decision／zone error fail-closed，明訂 13417 列與「合法零 zone vs 運算失敗」的分界，補注入測試；④**Stage 0 範圍與正式 scan 次數矛盾** → 端到端只用 smoke bundle，正式 Stage 1 留給下一階段，並要求對「I-100 的跨日兩趟不計次」做明文裁決；⑤**primary zone 修正漏了下游** → 三個 consumer 一起切，`_decision_summary_zone()` 補 `relative_volume`（否則 volume context 靜默退化），補 identity 測試；⑥**欄位數與型別描述錯誤**（表 8 列／文字七個／「都是 boolean」但實際只有 3 個）→ 補精確 schema 表並定案 `position_action_condition` 保留物件；⑦**bundle 仍 untracked** → 明寫在納入版控前不得視為正式凍結輸入，並提出提前納入的裁決點 |
| v3 | review 再抓到 3 高 3 中 1 低，全部反映：①**主文仍留著舊的錯誤定義**（candidate 用對外 `rr_gate.qualified`、宣稱兩版 rr_gate 相同、責任表只說產一個鍵、primary zone 說「不用動 decision engine」、護欄說「三層都是純新增」）→ **五處主文全部同步修正**，⛔ 不留雙真相源；②**no-zone 列走不到計畫的資料流**（`:1511` 只在有 zone 時才呼叫 `build_decision_summary()`，三項宣稱無法同時成立）→ 新增六-B，明訂為 **serialization 層的合法缺席 fallback**（⛔ 不是重算上游）並列出九個欄位的固定值與可辨識旗標；③**smoke 腳本注入的假 candidate 會覆蓋真欄位造成 false pass** → 新增四-B，⛔ 移除注入、Stage 1 改用真實 candidate、非空 cohort 的比較路徑改由 artifact-layer 的 pytest 涵蓋，⛔ 不為了命中而改造判定；④`structure_state` 來源寫錯（top-level 沒有它）→ 改從 `position_action_condition.structure_state` 取，⛔ 不另設第二來源；⑤fail-closed 只寫 Stage 1、且把 13417 硬編進通用路徑 → 拆成三層（運算完整性與範圍完整性**通用於 Stage 1／2**、13417 與 bundle_id 只在**正式 bundle 的 preflight**），補「smoke bundle 不得因列數失敗」的測試；⑥回滾描述把行為變更寫成純新增 → 改成性質表，明列 primary zone 切換／fail-closed 控制流／no-zone fallback 都是行為變更；⑦bundle 狀態文字過期（已 staged 非 untracked）→ 更正，並維持「commit 前不算正式進版控」；⑧v2 改寫時漏掉的「受影響檔案」表補回（一-B） |
| v4 | review 再抓到 2 高 2 中 1 待裁決，全部反映：①**fallback 觸發條件過寬**——`:1524` 的守門是四條件合取，「沒有 decision summary」還包含「zone 在、但 trend／volatility／metrics 缺」，那是異常⛔ 不是合法缺席 → 收窄成「`zone_score_available==false` **且** `zone_score_error=="NO_ZONE_SCORES"`」才套 fallback，其餘一律 fail-closed，並補「zone 在但 metrics 空 → 中止」的對照組測試；②**fallback 表漏了 `lifecycle_phase`**，且 v3 內文誤稱它「必然是 `NO_PRIMARY_ZONE`」 → 定案**維持 `null`**（`:1498` 的既有預設；no-zone 根本不進 lifecycle，補值等於替上游推導），並說明 `null` 與 `candidate=false` 為何一致；③**移除假 candidate 後 smoke 仍會拒絕合法空 cohort**——`smoke-replay-offline.sh:205` 的 `assert cohort["keys"]` 會把自然零命中判成失敗 → 明列同步移除，改驗「合法空集合」（comparison 空、report `candidate_rows==0`、兩份 artifact 仍須產出）；④**正式 bundle preflight 未定位置** → 明訂必須在 `_decision_replay_rows()` **之前**完成（否則跑完 3.7 小時才發現就不叫 preflight），且⛔ I-074 的 bundle ID 不得硬編進通用 evaluation contract，放在專屬 preflight helper；⑤**v3 自己仍留著「三層只允許新增欄位」** → 改成「lifecycle／decision 的判定條件與優先序不得改；evaluation 允許本計畫明列的來源切換與守門」；⑥**計次裁決採納 review 提議** → 兩次跨日執行視為同一組 deterministic replay，**D+1 的 after artifact 直接作為 I-074 正式 Stage 1 artifact、只算一次**，⛔ 期間不得改 predicate，不一致時要立案⛔ 不得跑第三趟取代 |
| v5 | review 再抓到 1 高 2 中 1 低，全部反映；**計次裁決已由使用者明文確認**：①**正式 artifact 只驗 candidate，沒守九欄位 contract**——`evaluation.py:2923` 與 `artifacts.py:123` 都只驗 candidate，少了 `action_state`／`position_action_condition`／`structure_state` 或型別錯照樣發布，之後無法完整判讀 B／C → 新增六-C 的 **runtime schema validator**，**Stage 1 發布前／載入 after artifact／Stage 2 的 before rows 三個時點都要過**；`position_action_condition` 必須是 object 且至少驗 `state`／`structure_state`，**只有明確 no-zone fallback 列可為 `null`**；⚠️ **等價式依版本分流**——after 用 `lifecycle_phase == CONTINUATION and not setup_rr_qualified`、**before ⛔ 必須用展開式**（before 版沒有 `lifecycle_engine.py`，錯套會全部判成不符）；`replay_bundle/artifacts.py` 列入受影響檔案；②**專屬 preflight 落點未裁決** → 定案 **③ 不是 Stage 0 的程式交付，延到 Stage 1 計畫**（Stage 0 只交付通用的 ①②），一-B 因此⛔ 不含專屬入口檔；③**主文測試與風險段仍有舊語意** → `rr_gate.qualified = true` 改成 `setup_rr_qualified = true`、等價測試補上「before ⛔ 不得套 after 等價式」、風險段的「只加回傳欄位」改成分層護欄，並明訂**受影響檔案以一-B 為準**⛔ 不另列一套；④**I-100 的 smoke 敘述會被當成現況** → 在該列註明「注入假 candidate 是當時的權宜做法，I-074 Stage 0 完成後會移除」，⛔ 標為歷史紀錄 |
| v6 | review 再抓到 2 高 3 中低，全部反映：①**before 的展開式無法由九欄位重算**——它含 `active_bearish_states`，那是 `event_state_summary` 的內容、lifecycle 的獨立輸入（`lifecycle_engine.py:150`），⛔ 不在 row 裡，v5 要 artifact validator 重算它是兌現不了的承諾 → 改成**分流**：validator 對 before **只驗 schema 與欄位交叉一致性**，**展開式改在 before tooling 內驗**（那裡還持有原始 `event_state_summary`），且那是 **Stage 2 的 patch 責任**⛔ 不是 Stage 0 交付；②**Stage 2 只用 after 的 cohort 過濾比較**（`evaluation.py:2960`），**before 多出的候選被靜默漏掉**——而主文明寫「兩邊集合不相同本身就是分支 C」 → 新增六-D 的**第五道集合檢查** `candidate_keys(before) == candidate_keys(after)`，⛔ 不相等即中止並保留差集，排在比較與發布之前；③**跨日執行表與已確認政策衝突**（舊表寫「I-100 不計、之後另跑正式 Stage 1」）→ ⛔ 刪除舊表改寫成「D／D+1 兩趟合起來算一次、第三趟不執行」，避免兩套真相；④**before tooling 仍寫成只處理「兩個欄位」** → 改成「與 after 對齊的完整 replay contract」，並明訂**完整輸出責任延到 Stage 2 計畫**，Stage 0 只負責讓 validator 有能力驗它；⑤**validator 缺欄位交叉一致性** → 補 `structure_state == position_action_condition.structure_state` 與 `action_state == position_action_condition.state`，⚠️ 並處理 `:205` 的 `or "WATCH"` 造成的空字串邊界；⑥**風險段稱 I-100「已落實」過度樂觀** → 改成「工具與凍結 bundle 已完成，**跨日逐列驗收仍是 blocker**」 |
| v7 | review 再抓到 2 高 1 中 1 低，全部反映：①**候選集合不一致時差集沒有保存方式**——`assert_same_keys()` 只把前五筆放進錯誤訊息，而那個差集**本身就是分支 C 的證據** → 定案發布專屬的 **`candidate_mismatch.json`**（完整 `before_only`／`after_only`、⛔ 不截斷、含 provenance 與 SHA-256），發布後才回專屬非零碼，⛔ 不產 comparison／report（那是**預期的終止狀態**）；⚠️ **⛔ 不採「聯集」方案**，因為它會讓 I-100 已收斂的第四道檢查必然不成立，新增一份 artifact 比鬆動既有契約安全；②**主計次與成本段落還停在舊政策**（寫「Stage 1 一趟 ＋ Stage 2 一趟」、成本 7～8 小時）→ 改成**邏輯計次一次／實體最壞三趟**（兩趟 after ＋ 有候選時一趟 before），成本改為「零候選 7～8 小時、有候選約 11 小時」，並補上「跨日兩趟正是凍結 bundle 為硬性前置的原因」；③**`lifecycle_phase` 未納入 validator contract**——after 等價式依賴它，缺失而 candidate 為 `false` 時 `false == false` **照樣通過** → 明訂一般 row 必須非空字串、no-zone fallback 必須 `null`、缺 key 一律 fail-closed，並補「即使 candidate 為 false 也要中止」的測試；④**boolean 數量寫錯**（正文仍寫「實際只有 3 個」，但 `setup_rr_qualified` 是 v2 自己加的，當下就已過期）→ 改成**四個** |
| v8 | review 再抓到 2 高 2 中，全部反映——**都是 v7 新增 `candidate_mismatch.json` 之後沒串起來的 contract**：①**mismatch 沒綁定確切的 after artifact**（Stage 2 的 provenance 記的是 before worktree，`bundle_id` 只證明同一份輸入）→ 補 **`after_artifact_sha256`**（comparison 早就有，`evaluation.py:2973`）＋ 差集排序唯一、互斥、與候選數的不變條件，並補 validator；②**全域決策樹與驗收規格不認得這個終止狀態**——四個權威段落都假設 Stage 2 一定產 comparison＋report → 同步四處：產物表補第三種產物與**兩種 terminal outcome**、**分支 C 的定義補「候選集合不一致」**、I-100 的 Stage 2 輸出表、`development-workflow.md` 的驗收報告；③**專屬結束碼與公開 API 未定義**——⚠️ `ArtifactError` 是 `ValueError` 子類（`artifacts.py:44`）而 CLI 是 `except (…, ValueError, …) → exit 1`（`:3219`），**繼承下去專屬碼根本出不來** → 定案 `EXIT_CANDIDATE_MISMATCH = 4` ＋ **`CandidateMismatch` ⛔ 不繼承 `ValueError`** ＋ `main()` 在 generic catch 之前處理，`replay_bundle/__init__.py` 列入受影響檔案（`evaluation.py` 只能用公開 API），並補「shell／docker runner 原樣傳出 exit code」的測試；④**成本段數字與敘述錯誤** → 範圍由 15,600 更正為凍結 bundle 的 **13,417**（0.85 × 13,417 ≈ **3.17 小時**，3.7 留作保守上界），並⛔ 刪掉 v7 自己寫錯的「單趟 3.7 小時遠遠跨出 09:00–15:00」（6 小時窗塞得進去），改寫成真正的兩個理由：**驗收明訂跨日**、**上游資料 24 小時內就會變** |
| v9 | review 再抓到 1 高 2 中 1 低，全部反映：①**mismatch 只存 key，before row 的診斷欄位會隨行程消失**——Stage 2 的 before rows 只在記憶體，正常路徑靠 comparison artifact 保存（`evaluation.py:2963`）而 mismatch 路徑⛔ 不產 comparison；⚠️ 那是一次 3 小時以上、⛔ 不允許用重跑取代結果的正式 replay，查不出 tooling 為何不對稱就只剩一次不該發生的重跑 → mismatch artifact 新增 **`before_rows`／`after_rows`：差集裡每一個 key 的完整 replay row（兩邊都存）**，⚠️ after row 雖可由 `after_artifact_sha256` 找回仍一併保存，讓證據自足；②**mismatch validator 的 schema 未完全定義** → 補 top-level 封閉欄位、`before_only ∪ after_only` 非空（⛔ 擋「零差集卻宣稱 mismatch」）、candidate count 用 `type(x) is int` 且 >= 0（⚠️ 排除 `bool`）、`after_artifact_sha256` 為 64 字元小寫 hex、key 是三個非空字串、`before_rows`／`after_rows` 的 key 集合恰好等於差集；③**exit 4 的 shell 測試沒有落點**——⚠️ smoke 兩側跑同一個 HEAD，正常⛔ 不會 mismatch，驗不到 → 定案改在 **`scripts/test-replay-args.sh`** 用可控的 **fake `docker`（`exit 4`）**驗 passthrough，⛔ 不為了測 exit code 去製造真的 mismatch（那要動判定條件），並把該檔列入受影響檔案；④**I-100 的受管制 artifact 清單漏了新檔** → 補入 `candidate_mismatch.json` |
| v10 | review 再抓到 1 高 2 中 1 低，全部反映：①**v9 自己的兩句話互相矛盾**——正文要「差集每個 key 的兩側 row」，schema 卻寫「`before_rows`／`after_rows` 各自等於**對應的**差集」，照後者實作會**只存各一半**，v9 想解決的問題原封不動回來 → 改成**單一 `rows` 列表** `{"key", "before", "after"}`，從結構上消除位置與集合語意的歧義，且與 comparison artifact **同形狀**；補不變條件「key 集合 == `sorted(before_only ∪ after_only)`」與「`before`／`after` ⛔ 不得為 `null`」（②已保證兩邊都有 row）；②**validator 沒明訂要被實際呼叫** → 定死順序 `build → validate → publish → raise → exit 4`，並補「validator 失敗 → ⛔ 不留下 `candidate_mismatch.json`，且結束碼是**一般中止 1 而不是 4**」的測試（⚠️ 4 的語意是「mismatch 已完整記錄」，驗證失敗時那個承諾沒兌現）；③**fake `docker` 一律 exit 4 會在 build 階段就退出**（腳本順序是 `build:143` → `image inspect:144` → `exec docker run:176`），測試看到 4 卻⛔ 完全沒驗到 passthrough → fake 改**依子命令分流**（`build`→0、`image inspect`→合法 digest＋0、`run`→記錄後回 4），並要求**同時斷言確實執行到 `docker run`**；④**歸檔清單漏掉整組新 terminal contract** → 補入 mismatch schema、兩種 terminal outcome、`EXIT_CANDIDATE_MISMATCH`、⛔ 不產 comparison／report 的規則，⚠️ 後三項同時寫進 `development-workflow.md` |
| v11 | review 再抓到 1 高 2 中，全部反映：①**`rows` 又被同時定義成兩種不相容的形狀**——v10 寫 `{key, before, after}` 卻同時聲稱「與 comparison artifact 同形狀」，而 `compare_rows()` 實際產的是 `{symbol, timeframe, as_of, differences, before, after}`（`artifacts.py:229`），實作者無從判斷該重用還是另做 wrapper → **定案直接重用 `compare_rows()`**（`rows = [compare_rows(before_by_key[k], after_by_key[k]) for k in mismatch_keys]`），理由是 `differences` **直接指出兩邊差在哪些欄位**、判讀工具可共用、少維護一套形狀；⚠️ **連續兩輪栽在同一處，教訓是描述形狀要貼實際產物，⛔ 不能憑印象寫「同形狀」**；②**所謂「精確 schema」其實沒封閉** → 補齊 top-level 的**確切欄位集合**與型別（`bundle_id`／`before_ref`／`generated_at` 非空字串、`provenance` 為 object）、`rows[]` 的封閉欄位集合、**外層 comparison row 與內嵌兩側 row 的 key 三方一致**（⚠️ v12 改寫措辭——原文寫「wrapper」容易被誤讀成已廢棄的 `{key, before, after}` 形狀），以及**數量下界 `count >= len(對應差集)`**——⛔ 「數量差相等」取代不了它（「兩側 count 都是 0、兩側差集各一筆」會通過 `0−0 == 1−1`，但集合上不可能）；③**測試矩陣沒涵蓋 v10 新增的 row contract** → 補六條，特別把「**兩側 row 都要保存**」單獨釘死（那正是 v9→v11 反覆修的那一條） |
| v12 | review 抓到 1 中 1 低，兩項都反映：①**`differences` 在 v11 被扶正成正式證據，卻沒有任何內容驗證契約**——只被列進封閉欄位集合，所以 `"differences": []` 或一串亂寫／重複／未排序的欄位名都會通過，**最重要的診斷摘要因此不可信** → 定死為與 `compare_rows()`（`artifacts.py:229`）**算出完全相同的值**，並補一條**必然成立的不變條件**：mismatch key 來自候選集合的 **symmetric difference**，所以每一列的 `rr_decoupling_candidate` 兩側**必然不同**，**`"rr_decoupling_candidate"` 一定在 `differences` 裡**，⛔ 不在即 publish 前拒絕；測試補「刪欄／加欄／重複／改順序」四組 tampering；②**「key 集合且同順序」不可執行**（集合本身沒有順序）→ 改成唯一可執行的斷言 `[row_key(item) for item in rows] == mismatch_keys`；並把 v11 修訂紀錄裡的「wrapper」改成「外層 comparison row」，⛔ 避免被誤讀成已廢棄的 `{key, before, after}` |
| v13 | review 抓到 1 中（Markdown 格式），已修，⛔ **未動任何語意契約**：**含 fenced code block 與多段說明的「精確 schema」被塞在表格列裡**，而 GitHub Markdown 的表格列**必須是單一實體行**——整張表會從該處截斷，後面的「發布／結束碼／不產出／SHA-256」四列**不再屬於同一張表** → 表格列改成一行指標，完整 schema 移到表格後的 `###### candidate_mismatch.json 的精確 schema` 小節。⚠️ **成因是我從 v8 起一路往同一張表裡加內容**——每次只加一列看起來沒事，累積到含 code fence 就壞了；已**全檔掃過**⛔ 無其他跨行表格列，fenced code block 也全部配對 |
| **v14** | review 抓到 1 低（格式殘留），已修，⛔ **未動任何語意契約**：v13 把 schema 從表格列移出來時，**保留了原儲存格結尾的 `|`**——它已不再負責結束儲存格，會被渲染成正文裡的多餘字元 → 刪除。⚠️ **成因是移動內容時只搬了本文、沒清邊界**；已**全檔掃過**⛔ 無其他「非表格行卻以 `|` 結尾」的殘留 |
| v15 | review 抓到 1 低（紀錄數字過期），已修，⛔ **未動任何語意契約**：v13／v14 的修訂紀錄曾寫入當下的**檔案行數**，但本檔每次編輯都會變 → ⚠️ **⛔ 不更新數字，直接移除**：驗證紀錄要記「**做了什麼檢查**」而不是「當時的檔案有多大」，後者必然過期而且沒有人會回頭更新它。fence 數量同理一併移除 |
| **v16** | review 抓到 1 低，已修，⛔ **未動任何語意契約**：⚠️ **v15 在「移除會過期數字」那一列裡，自己又寫進了三個具體行數**——而且送審時它們就已經過期了。**規則寫對、執行時違反自己**：宣告某個東西不該寫，最容易的犯法方式就是在宣告它的同一句話裡示範一次。→ 該列改成⛔ 不含任何具體數字的敘述 |
| **v17** | review 抓到 3 中 1 低，全部反映——⚠️ **這一輪抓的是實作與測試，⛔ 不是計畫書文字**，語意契約一個字沒動：①**no-zone validator 沒對齊 durable schema**——只釘三個 boolean，於是 before 側的 `rr_decoupling_candidate=true`（before ⛔ 不驗等價式，攔不到）、三個字串被改成一般列的值、`position_action_condition`／`position_action` 非 null 全都靜默通過 → 改成**逐欄對照完整 mapping**，並把 `DIAGNOSTIC_NO_ZONE_FALLBACK`／`DIAGNOSTIC_FIELDS`／`NO_ZONE_SCORES_ERROR` 統一落在 `replay_bundle/artifacts.py`、經 package 公開 API 匯出（⚠️ 順帶收掉 `NO_ZONE_SCORES_ERROR` 的**雙真相源**——`evaluation.py` 與 `artifacts.py` 各有一份同值常數，改了一邊而另一邊沒跟上時⛔ 沒有任何東西會報錯）；⚠️ 舊的 tamper 測試對三個字串欄位會**一併改 `lifecycle_phase`**，實際攔下它的是 phase 規則，⛔ 沒有證明字串欄位本身會被拒 → 改成**九欄全驗、每案只動目標欄位**，壞值挑型別與結構都合法的（`position_action_condition` 給與兩個 state **交叉一致**的 object），並用 `side="before"` 跑以證明擋下 candidate 的是固定 mapping 而非等價式；②**mismatch validator 只驗內部自洽**——一份「漏掉一個真實 mismatch」的產物可以完全自洽（集合、差值公式、下界、方向、`differences` 全過，只是少記一筆）→ 新增**第三層：與實際來源精確比對**，`before_by_key`／`after_by_key`／`expected_after_sha256` 三個參數**必填**（⛔ 不提供「不給就跳過」的模式，可選等於留一道 fail-open），驗差集、計數、每列兩側 row、after artifact 的 SHA，並要求來源列的 candidate 是嚴格 boolean；⚠️ 測試必須持 **pristine copy**——`compare_rows()` 把來源 dict 直接放進 artifact，竄改 artifact 會一併改到來源，來源對照就變成自己跟自己比；③**三個新測試沒刺激到聲稱保護的產品分支** → pass-through 改用 **monkeypatch sentinel**（回傳與輸入推導相反的值，重算就會紅；⛔ 不再「同一組輸入跑兩次」）、補 **primary zone 的 replay 層 regression**（decision primary 為 `None`、排序第一筆非空）、「zone 在但 metrics 缺」改成 monkeypatch `_historical_zone_score_summary()` 的**完整資料形狀**後走**真正的 `_decision_replay_rows()`**（⛔ 不在 stub 輸出上手改欄位），並補「真的沒有 zone 時**應該**套 fallback」的對照組；④**低：`evaluation.py` 的 primary-zone 註解描述錯誤**——寫成「EXPIRED／LOW confidence／缺 expected_value 也會讓 `_pick_primary_zone()` 回 None」，但 `decision_engine.py` 的**第二層 fallback 只看 `role != AT_ZONE`**，那三者只影響第一層篩選、會被收回來 → 更正為「**沒有任何非 AT_ZONE zone** 才回 None」，並把它變成**可執行的斷言**放進 `test_decision_engine.py`（⚠️ 單元契約留在它所屬模組），補 LOW／缺 expected_value 的 fallback 案例 |

#### Stage 0 實作結果（2026-09-14）

**已依 v17 計畫書實作完成，2026-09-14 review 已確認實作方向、⛔ 無高／中嚴重度問題。**
（v16 的 review 抓到 3 中 1 低，修正內容見修訂表 v17 與下方各列的 ⚠️ 標註。）

⚠️ **review 通過的是 Stage 0，⛔ 不是本筆**：I-074 的狀態仍是「待執行」——決策樹的三個分支
都還沒被走完，正式 Stage 1／2 尚未執行。⛔ 本筆在那之前不得移除。

| 檔案 | 內容 |
|---|---|
| `lifecycle_engine.py` | `resolve_lifecycle()` 回傳新增 `clear_zone_breakout`、`continuation_price_evidence_met` |
| `decision_engine.py` | `_decision_semantic_pipeline()` 新增**四鍵**（兩個透傳 ＋ `setup_rr_qualified` ＋ `rr_decoupling_candidate`）；`_decision_summary_zone()` 補 `relative_volume` |
| `evaluation.py` | 匯出九欄位；no-zone fallback；primary zone 三個 consumer 一起切；Stage 1／2 的運算完整性 fail-closed；第五道集合檢查 ＋ mismatch 發布；CLI 在 generic catch **之前** handle `CandidateMismatch` |
| `replay_bundle/artifacts.py` | `validate_diagnostics()`（九欄位 ＋ `lifecycle_phase` 依賴 ＋ 交叉一致性 ＋ 依版本分流的等價式 ＋ ⚠️ **v17：no-zone 逐欄對照完整 mapping**）、mismatch 的 builder／validator（⚠️ **v17：新增與實際來源精確比對的第三層，三個來源參數必填**）、`CandidateMismatch`；⚠️ **v17：`DIAGNOSTIC_NO_ZONE_FALLBACK`／`DIAGNOSTIC_FIELDS`／`NO_ZONE_SCORES_ERROR` 統一落在這裡**（⛔ 收掉雙真相源） |
| `replay_bundle/publish.py`／`__init__.py` | `EXIT_CANDIDATE_MISMATCH = 4`；公開 API 匯出（⚠️ v17 加上三個診斷常數） |
| `test_decision_engine.py` | ⚠️ **v17**：`_pick_primary_zone()` 真正的 `None` 條件（單元契約留在它所屬模組），補 LOW／缺 `expected_value` 的 fallback 案例 |
| `frontend/.../srZones.ts` | `SRSemanticPipeline` 四鍵、`SRDecisionZoneSummary.relative_volume`（**僅型別，⛔ 不接線**） |
| `scripts/smoke-replay-offline.sh` | ⛔ 移除假 candidate 注入 ＋ 非空 cohort 斷言，改驗合法空集合 |
| `scripts/test-replay-args.sh` | exit 4 passthrough（fake `docker` 依子命令分流，並斷言**確實走到 `docker run`**） |

**新增測試**：`test_i074_diagnostics.py`（84）、`test_i074_mismatch.py`（36），
並擴充 `test_replay_bundle_stages.py`（57）與 `test_decision_engine.py`。

**驗證**（2026-09-14 v17 修正後重跑）：

| 層 | 結果 |
|---|---|
| `python/scripts/test.sh` | **1090 passed, 1 skipped**（v16 當時是 1040 passed, 1 skipped） |
| `scripts/test-replay-args.sh` | 62 項全過。⚠️ **已由 `python/scripts/test.sh` 自動先跑**，⛔ 不必單獨再跑一次 |
| `scripts/smoke-replay-offline.sh` | 全過——⚠️ **用的是真實 candidate**，`after rows=35 cohort=0 comparison=0` 是自然零命中（合法結果） |
| `frontend/scripts/test.sh` | svelte-check ＋ vitest（21 檔 155 測試）＋ vite build ＋ dist／job-name 檢查全過 |

⚠️ **前端那一列是 2026-09-14 後續 review 時補跑的實際結果，⛔ 不是 v16 當時就執行過的另一條命令。**
v16 記的是 host 上的 `npx tsc --noEmit`——那條在這台 2GiB 的機器上會被 host OOM killer 砍掉
**呼叫端**（node 沒有 cgroup 擋、也沒給 `--max-old-space-size`）。前端唯一的驗收入口是
`frontend/scripts/test.sh`（三步各一個 container），見 [`development-workflow.md`](./development-workflow.md)。

⚠️ **v17 的新測試做過「會不會假綠」的反向驗證**：把兩個回歸注回產品程式——
① primary zone 改回 `if decision_primary is not None` 的舊寫法、
② no-zone fallback 的守門條件放寬成 `else`——
`test_replay_primary_zone_follows_decision_primary_even_when_it_is_none` 與
`test_zone_present_but_metrics_missing_is_not_a_legal_fallback` 各自紅了，
而對照組 `test_real_no_zone_rows_do_get_the_fallback` 正確地維持綠。
⛔ 沒做這一步的話，「測試通過」證明不了測試有在測東西——那正是 v17 review 抓到的第 3 項。

⚠️ **一個既有契約測試被正確地攔下來**：`test_evaluation.py` 有一條「replay row 的
`primary_zone` 欄位增減要在這裡被擋一次」的斷言，註解寫明它是**Python 與前端型別之間唯一的
連結點**。primary zone 換來源後它紅了——**那正是它該做的事**。已更新清單並寫明分界點，
⛔ 不是把斷言放寬。

**歸檔**：現況規格已寫進 [`sr-zone-scoring.md`](./sr-zone-scoring.md)
「Decision Replay 的診斷欄位（I-074 Stage 0）」；Stage 2 兩種 terminal outcome 與結束碼 4
補進 [`development-workflow.md`](./development-workflow.md) 的驗收章節。

⛔ **尚未執行**：正式 Stage 1／2（那是下一階段，且依已確認的計次裁決，D／D+1 兩趟 after
合為一次）。

#### Stage 1 計畫書 v25（2026-09-14 起草，**待確認**）

⚠️ **v24 的 review 抓到 1 中，已反映**；修訂摘要在本節最後。
⚠️ **本版依「零、改版守則」以增補方式修訂，⛔ 未整段重寫**——v7 的條款逐條保留。

##### ⚠️ 零、改版守則（v7 新增——**連續兩輪栽在同一種錯**）

**v4 弄丟了 v3 的「validator 來源綁定」契約，v6 又弄丟了 v4 的「crossday 型別契約」。**
兩次都不是判斷錯誤，是**整段重寫時的遺失**——而且兩次都被 review 當成高風險抓回來。

**守則一**：改版時⛔ **不得刪除任何既有契約條款**，只能改寫表達或補強；精簡只能作用在
敘述文字上，⛔ 不能作用在**可驗證的條款**上。每次改版後要逐條回查前一版的條款是否都還在。

**守則二（v11 新增——⚠️ 已經漏了三次）**：同一個規則常常同時寫在**正文**與**測試矩陣**兩處
（v8 改了 orchestrator 流程卻漏改測試矩陣、v10 改了 recovery 回傳卻又漏改 11-B）。
⛔ **改動任何正文條款後，必須同步檢查測試矩陣裡對應的那一條**；改完的驗證要**限定在計畫書正文
行號範圍內**（⛔ 不用全檔 grep——修訂紀錄的命中會掩蓋正文沒改到的事實，v8 就是這樣過關的）。

⚠️ **v17 補一個可操作的做法**（⛔ 光有原則不夠——這條到 v16 為止已經被違反**五次**）：
每一輪改完，**逐一列出本輪新增或修改的「可驗證條款」，對照測試矩陣確認每一條都有對應項**，
⛔ 沒有對應項就是還沒改完。

⚠️ **v22 再加一道**（⛔ 連 checklist 也不夠——v21 又違反了一次，累計**六次**）：
**修訂紀錄裡每一條新增契約都要標註它對應的測試項編號**。漏掉的話，在寫修訂紀錄時就會卡住，
⛔ 不必等到下一輪 review 才發現。

##### 一、目標與不做的範圍

**目標**：交付 **preflight**、**crossday**、**capacity probe**、**evidence finalizer**、
**image pin** 與 **orchestrator**，完成 D／D+1 兩趟正式 Stage 1 replay，並結清 I-100 關閉條件 2
的後半與條件 3。

**⛔ 不做**：⛔ 不跑 Stage 2；⛔ 不改交易 predicate、decision 結果或任何通用的未啟用路徑；
⛔ 不重產 bundle；⛔ 不把 bundle ID 硬編進通用 evaluation contract；
⛔ 不動 `load_artifact()`／`after_sha`／cohort／受管制清單的既有 contract，
且⛔ 不改 `build_provenance()` 的輸出形狀。

##### 二、受影響檔案

| 檔案 | 內容 |
|---|---|
| `replay_bundle/i074_preflight.py`（**新增**） | preflight（三、四） |
| `replay_bundle/crossday.py`（**新增**） | crossday builder／validator ＋ 獨立 CLI（五～七） |
| `replay_bundle/provenance.py` | `PROVENANCE_FIELDS` ＋ `validate_provenance(prov, *, role)`（八） |
| `replay_bundle/evidence.py`（**新增**） | finalizer、`evidence_manifest.json`、archived 層（十、十一） |
| `replay_bundle/publish.py` | `EXIT_CROSSDAY_MISMATCH = 5`（既有 `rename_noreplace`／`fsync_*`／`remove_tree` 沿用） |
| `replay_bundle/artifacts.py`／`canonical.py`／`__init__.py` | `CROSSDAY_*`、公開 API |
| `evaluation.py` | preflight 與 capacity probe 兩個 opt-in（⛔ 不含 crossday） |
| `scripts/pin-replay-image.sh`（**新增**） | **唯一** build／pin 入口（十三）；⚠️ 同時是 **run identity 檔的 producer**（十三-B） |
| `replay_bundle/run_identity.py`（**新增**） | run identity 的 schema、`validate_run_identity()`、原子建立與重入語意（十三-B） |
| `scripts/run-i074-stage1.sh`（**新增**） | **orchestrator**（十-B） |
| `scripts/run-replay-offline.sh` | 新參數、**一律以 image ID 執行**、peak 量測 fail-closed |
| `scripts/compare-replay-crossday.sh`／`scripts/finalize-evidence.sh`（**新增**） | crossday 與 finalizer 入口 |
| `scripts/lib/replay-args.sh`／`scripts/test-replay-args.sh` | 參數所有權、stage 限定、exit 5 passthrough、pinned-image 未 build |
| `python/scripts/fixtures/stage1_argv.json`（**新增**） | 目前只有 `stage0_argv.json` |
| `python/scripts/fixtures/finalizer_argv.json`（**新增**） | finalizer 專屬 argv fixture，⚠️ **分別保存 normal 與 recovery 兩組**（十三-B、十七-12） |
| `python/scripts/validate-i074-run-identity.py`（**新增**） | ⚠️ **host 端**（無 pandas）的 identity 驗證入口，**四類角色**在 `docker run` 前都呼叫它；⚠️ **同時支援 `.json` 與 `.json.gz`**（十三-C-B） |
| `python/scripts/fixtures/comparator_argv.json`（**新增**） | comparator 的 argv fixture（含 `--run-identity`；v21） |
| `tests/test_i074_preflight.py`／`test_crossday.py`／`test_evidence.py`（**新增**） | 單元測試 |
| `docs/development-workflow.md` | **`:930` 要修**（十四） |

##### 三、preflight

| 時點 | 檢查 |
|---|---|
| **pre**（replay context 推導後、`_decision_replay_rows()` 之前） | ① `bundle_id` 相符；② symbols 集合恰為 11 檔；③ 每檔末根**台北日期**是 `2026-09-01`；④ `sum(quota) == 13417` 且 `len(universe) == 13417` |
| **post**（replay 後、發布前） | ⑤ 實際 `rows` 數 == 13417 且 `keys` 集合與 ④ 的 universe 完全相等 |

⛔ helper 不得自己重寫 `bars − 80 − 5`（唯一真相源 `_candidate_bar_range()`）。
**13417** ＝ 14352 − 11 × 85（**11 檔合計**）。

##### 四、`2026-09-01` 是**台北交易日**

末根 epoch **1788192000**：UTC 是 `2026-08-31T16:00:00Z`（日期 **08-31**），Asia/Taipei 才是
`2026-09-01`。明訂 **epoch → UTC → `Asia/Taipei` → local date**。

##### 五、crossday 輸入有效性（⛔ 不合法一律 exit 1 且不產 artifact）

兩側 SHA 不同；`generated_at` 合法含時區、轉台北後 `d1 == d + 1 day` 且順序正確；記錄的時間與來源
完全相同；⚠️ **同檔／同 SHA／同日 ＝ invalid input**（⛔ 不是 mismatch）；
**來源驗證鏈** **`load_canonical_evidence_artifact()`**（見八-B）→ `validate_after_artifact()`
→ **`validate_diagnostics(side="after")`**（⚠️ 中間那個**不驗**九欄位）；
五個身分欄位兩側相同；來源 provenance 形狀錯亦屬 invalid input。

##### 六、crossday 的**完整 schema 與型別契約**

⚠️ **來源重算⛔ 取代不了型別驗證**：Python 裡 `1 == True`、`0 == False`，一份用 `1`／`0` 冒充
boolean 的 artifact 可以通過所有「重算後相等」的比對。v6 精簡時把型別條款刪掉了，v7 恢復。

```
rows_match       = d_key_order == d1_key_order 且 d_only／d1_only／row_differences 三者皆空
provenance_match = provenance_differences 為空
matched          = rows_match and provenance_match
```

| 欄位 | 型別契約 |
|---|---|
| `schema_version` | `type(x) is int` 且 **== 1** |
| `kind` | **== `"sr_zone_replay_crossday"`** |
| `bundle_id` | 非空 str |
| `d_artifact_sha256`／`d1_artifact_sha256` | **裸 64 字元 lowercase hex**，且**兩者不同** |
| `d_generated_at`／`d1_generated_at` | 非空 str、含時區 RFC3339 |
| `d_row_count`／`d1_row_count` | **`type(x) is int` 且 >= 0**（⛔ 排除 bool） |
| `d_key_order`／`d1_key_order` | 陣列，每個元素是 **`[symbol, timeframe, as_of]` 三個非空 str**；元素**唯一**；⛔ 不排序（順序本身是語意） |
| `rows_match`／`provenance_match`／`matched` | **`isinstance(x, bool)` 嚴格 bool** |
| `outcome` | 封閉列舉，且**與三個旗標精確對應**（見下真值表） |
| `d_only`／`d1_only` | key list：元素同上三元組、**排序**、**唯一**、兩者**互斥** |
| `d_only_rows`／`d1_only_rows` | 完整 rows，**key 序列逐項等於對應 key list** |
| `row_differences` | 重用 `compare_rows()` 的形狀；⛔ **不得包含空 `differences`** |
| `provenance_differences` | `{field, d, d1}`，依 `field` 排序且唯一 |
| `comparator_provenance` | 通過 `validate_provenance(role="comparator")` |
| `generated_at` | 非空、含時區 RFC3339，⛔ 不早於 `d1_generated_at` |

⚠️ **top-level 欄位集合是封閉的**（v8 明訂）：以 `_CROSSDAY_FIELDS` 表示，**缺欄或多一個未知欄位
一律拒絕**——三份 probe 與 evidence manifest 同樣各有自己的 `_FIELDS` 封閉集合。

**`outcome` 真值表**（⛔ 任一組合不符即拒絕）：

| `rows_match` | `provenance_match` | `outcome` | `matched` |
|---|---|---|---|
| true | true | `MATCH` | true |
| false | true | `ROW_MISMATCH` | false |
| true | false | `PROVENANCE_MISMATCH` | false |
| false | false | `ROW_AND_PROVENANCE_MISMATCH` | false |

**`row_differences[]` ＝ 直接重用 `compare_rows()`**（`artifacts.py:410`），**`before` 裝 D、
`after` 裝 D+1**——⛔ 不改欄位名、⛔ 不複述形狀。

**validator 的來源綁定**：`validate_crossday()` **必填**收兩份**實際** artifact 與兩個**實際** SHA，
**全部由來源重算後精確比對**（SHA、row count、key order、兩個差集、兩份 exclusive rows、
所有共同 key 的 `compare_rows()`、來源時間與五個身分欄位、`provenance_differences`、
三個 flag 與 `outcome`），⛔ 不接受 artifact 自報的任何值。
⚠️ **`by_key` 由 validator 自己從兩份 artifact 的 `rows` 建**，⛔ 不收外部傳入版本
（⛔ 不製造第三個真相源）。

**有效來源 ＋ `outcome != MATCH` → 先發布再 exit 5；來源不合法或 validator 失敗 → exit 1 且⛔ 不發布。**

##### 七、provenance 的型別契約

⚠️ `build_provenance()` 的輸出形狀⛔ 不動（只補 validator）。`PROVENANCE_FIELDS` 是 10 個欄位的
單一真相源；`validate_provenance(prov, *, role)` 的 `role` 是封閉列舉
**`stage0`／`stage1`／`comparator`／`finalizer`**。

| 欄位 | 型別 |
|---|---|
| `image_digest` | `sha256:` ＋ 64 hex |
| `pip_freeze_sha256`／`runner_sha256`／`tooling_patch_sha256`／module hash values | 裸 64 hex |
| `base_commit` | 40 hex |
| `source_root` | 絕對路徑字串 |
| `argv` | 非空字串陣列 |
| `python_version` | 非空字串 |
| `project_modules_sha256` | ⚠️ **「Python module name → 64 hex」的 mapping**（`out[name] = …`，`provenance.py:59-75`）——⛔ **不是相對路徑**，寫錯會拒絕現有所有合法 provenance |
| `runtime_settings` | ⚠️ 鍵集合是**「⊆ 五個」⛔ 不是「恰好五個」**（builder 是 `if hasattr` 才寫，`provenance.py:104`）：`db_driver`(str)／`sr_scoring_model_path`(str)／`sr_scoring_evidence_enabled`(bool)／`sr_scoring_evidence_max_zones`(int)／`sr_scoring_adaptive_zone_builders_enabled`(bool) |

**nullability 依 role**：`base_commit`／`tooling_patch_sha256` 在 `stage0` 可為 null；
在 `stage1`／`comparator`／`finalizer` ⛔ 不得為 null。
**唯一允許不同的是 `argv` 裡的 `--output-dir` 值**：接受兩種寫法，取值後換成固定佔位符再比較；
⛔ 缺值或重複即 invalid input。
⚠️ **`comparator_provenance` 要與 comparator 的實際執行環境對照**——comparator 執行時自產一份，
validator 必填收它並逐欄比對。

⚠️ **comparator／finalizer 的 provenance 怎麼推導出來**（v8 新增——v7 只定了 image ID 的來源）：
兩支官方腳本**沿用與 replay runner 相同的 worktree／tooling-patch 推導流程**
（`replay_args_prepare_worktree` → `replay_args_tooling_patch_sha256` → `replay_args_runner_sha256`），
取得 immutable `base_commit`、實際 tooling patch SHA、`source_root` 與 runner SHA；
⚠️ **provenance 要在實際工作完成、lazy import 都發生之後才建**（與 Stage 1 同一條理由：
太早建的話 `project_modules_sha256` 少記的正是實際跑過的那些檔案）。
⛔ **不得直接複製 D 的 provenance**——comparator 與 finalizer 載入的模組集合與 replay 不同。

##### 八、SHA 與 canonical 的精確語意

⛔ `load_artifact()` 算的是**檔案 raw bytes** 的 SHA（`artifacts.py:725`），⛔ 沒有重新 canonicalize。

| 輸入 | 規則 |
|---|---|
| `.json` | **要求 raw bytes == `canonical_json_bytes(parsed)`**，`artifact_sha256` **直接算 raw bytes**（與現行語意一致） |
| `.json.gz` | ⚠️ **`raw_gzip == canonical_gzip_bytes(gunzip_bytes(raw_gzip))`**（見下），解壓後亦須 == `canonical_json_bytes(parsed)`；`artifact_sha256` 算**解壓後的 bytes** |
| 兩者 | `stored_sha256` 是**實際落地檔案**的 SHA |

⚠️ **canonical gzip ⛔ 不能只驗 `mtime=0` 與無 filename**（v7 修正）：compression level、XFL、
OS byte 或 deflate 表示不同都會產出不同 bytes 卻通過那兩項。既然 archive 的**唯一 producer**
是 `canonical_gzip_bytes()`，契約直接定為**整段 round-trip byte-identical**——它同時驗 header、
壓縮內容與 footer。
**✅ 2026-09-14 實測**：既有 bundle 的三個 payload（`candles`／`chip`／`governance`）
全部 round-trip byte-identical，所以此契約可行，且**既有 bundle 可直接當測試 fixture**。
測試補「**不同 compression level 或其他 header byte → 拒絕**」。

##### 八-B、⚠️ `.json.gz` 的 loader 落點（v8 新增）

⛔ **v7 的矛盾**：來源鏈寫「`load_artifact`」，SHA 章節卻允許 `.json.gz`，而範圍又明訂⛔ 不改
`load_artifact()`——實作者無從判斷那是指舊函式還是某個新 wrapper。而既有 loader 直接把 raw bytes
解碼成 UTF-8 JSON（`artifacts.py:703`），**確實讀不了 gzip**。

**定案：新增 `load_canonical_evidence_artifact(path, kind)`**，⛔ 既有 `load_artifact()` **完全不動**。

| 副檔名 | 流程 |
|---|---|
| `.json` | 呼叫**既有** `load_artifact()`，取它回傳的 `(parsed, raw_sha)`，再驗 **`raw_sha == sha256_hex(canonical_json_bytes(parsed))`** |
| `.json.gz` | 讀 raw gzip → 驗 **round-trip byte-identical** → 解壓 → 驗 **== `canonical_json_bytes(parsed)`** → 檢查 `schema_version`／`kind` |

* **回傳 `EvidenceLoad`**（v11 擴充——⚠️ v10 只回 `(parsed, artifact_sha256)`，但 finalizer
  還需要 `stored_sha256` 與 `stored_bytes`，而 raw bytes ⛔ 沒有從 loader 傳出；finalizer 若自己
  再讀一次就**違反「同一次讀取」契約**）：

  | 欄位 | 內容 |
  |---|---|
  | `parsed` | 解析後的 artifact |
  | `artifact_sha256` | **解壓後 bytes** 的 SHA（`.json` 時即 raw bytes，與 `load_artifact()` 語意一致） |
  | `stored_sha256` | **實際落地檔案** raw bytes 的 SHA |
  | `stored_bytes` | 該 raw bytes 的長度 |

  ⚠️ **crossday 只取前兩項**，**finalizer／recovery 用完整結果**——這樣三項 metadata 都來自
  **同一次讀取**。既有 `load_artifact()` 的公開 API ⛔ 不變；
* ⚠️ **crossday CLI 與 finalizer 都只用這個入口**；既有 Stage 1／2 繼續用 `load_artifact()`。

⚠️ **`.json` 這條⛔ 不得讀第二次檔案**（v9 修正）：`load_artifact()` 只回傳 `(parsed, sha)`，
**raw bytes ⛔ 不會傳出**——wrapper 若自己再讀一次，parsed／SHA 與第二份 raw 可能已不是同一版本
（檔案在兩次讀取之間被換掉）。改用 **SHA 對照**：`raw_sha == sha256_hex(canonical_json_bytes(parsed))`
等價於「raw bytes 就是 canonical bytes」，且只讀一次。⛔ 不為此改 `load_artifact()` 的公開 API。

##### 九、operational 與 archived 是兩層

| 層 | 形式 | 產生者 | SHA |
|---|---|---|---|
| **operational** | `.json`（canonical，現況不動） | replay／crossday／probe 各自的行程 | `artifact_sha256` |
| **archived** | `.json.gz` | ⚠️ **只有 evidence finalizer** | `stored_sha256` ＋ `stored_bytes` |

**所有產生者只寫 canonical operational `.json`**；`.json.gz` 與 `stored_sha256` **一律由 finalizer
統一產生**。probe completion 引用的是前兩份的 **`artifact_sha256`**。
**實測依據**：baseline replay row 每列約 599 B → 13417 列約 7.7 MiB、gzip 25x → 約 0.3 MiB。
⚠️ Git LFS **未安裝**、無不可變外部儲存。

##### 十、⚠️ 三個位置**完全分離**——v6 的 root 與 logs 互相衝突

⛔ **v6 的邏輯矛盾**：finalizer 要求正式 root **必須不存在**（整包 rename 發布），卻又把執行 log
放在該 root 底下的 `logs/`——D／D+1 執行期間一建 log 目錄，root 就存在了，最後的
`rename_noreplace()` **必然失敗**。**定案：三個位置完全分離**：

| 用途 | 位置 | 版控 |
|---|---|---|
| **operational artifacts**（runner 的 `--output-dir`） | **repo 外**的暫存／工作目錄 | ⛔ 否 |
| **execution logs** | **repo 外**的持久工作目錄 | ⛔ 否 |
| **archived evidence** | `python/baselines/i074_stage1/` | ✅ 是 |

⚠️ **archived evidence root 在 finalizer 執行前必須完全不存在**，⛔ 任何其他步驟都不得在它底下
建任何東西。

**精確路徑表**（共 **10 個檔案**）：

| 子目錄 | 檔案 |
|---|---|
| `d/` | `after_artifact.json.gz`、`cohort_manifest.json.gz` |
| `d1/` | `after_artifact.json.gz`、`cohort_manifest.json.gz` |
| `crossday/` | `crossday_artifact.json.gz` |
| `probe/` | `capacity_probe_computation.json.gz`、`capacity_probe_measurement.json.gz`、`capacity_probe.json.gz` |
| `identity/` | `run_identity.json.gz`（⚠️ **v11 納入**——見下） |
| root | `evidence_manifest.json`（⛔ **不壓縮**——索引要能直接讀；**排除自身**） |

⛔ **禁止未知 evidence**（多檔即中止）。

⚠️ **run identity 要進證據包**（v11 修正）：v10 一邊說 recovery「⛔ 不讀 operational inputs、
只驗既有九檔」，一邊又要求 manifest 與 **repo 外**的 run identity 比對——**兩者矛盾**，
而且外部 identity 一旦遺失，**已完整發布的 evidence 就再也 recovery 不了**。
→ **把 canonical run identity 納入 archived evidence**（`identity/run_identity.json.gz`），
於是 **recovery 只依賴既有 root 內部的關係**，證據自足。
⚠️ repo 外那份仍然存在，它是**操作期的跨日協調檔**；歸檔的這份是**證據**，兩者內容必須逐欄相同
（正常 finalization 時比對，⛔ recovery 時不需要外部那份）。

##### 十-B、⚠️ orchestrator：exit 5 之後誰跑 finalizer

⛔ **v6 沒定義**。`compare-replay-crossday.sh` 一回傳 5，普通 `set -e` 流程就停了，
finalizer 永遠不會執行——而 mismatch **正是**最需要保存證據的情況。

**`scripts/run-i074-stage1.sh`（orchestrator）的流程定死**：

1. **捕捉** crossday 的 exit code（⛔ 不讓 `set -e` 直接中斷）；
2. **只接受 0 或 5**——其餘一律視為失敗，⛔ 不執行 finalizer；
3. 執行 **finalizer**；
4. finalizer 成功 → **回傳原始的 0 或 5**；
5. finalizer 失敗 → **回 1**；
6. ⚠️ **finalizer 回 `EXIT_DURABILITY_UNCONFIRMED`（3）時，durability code 優先於原始 0／5**
   （v8 新增）——證據已發布但落盤未確認，那是**必須被看見**的狀態，⛔ 不得被 `MATCH` 的 0 蓋掉。

##### 十一、evidence finalizer：一次發布整包

⚠️ **順序定死為兩階段**（v10 修正——v9 的「寫 manifest → 全圖驗證」與「provenance 要在 lazy
import 都發生後才建」直接衝突：manifest 內含 `finalizer_provenance`，而全圖驗證期間才載入的專案
模組**不會進** provenance）：

* **階段 A**：讀入所有 operational inputs、完成 **9 個 archive payload** 的**全部驗證**
  （⚠️ **含 `identity/run_identity.json.gz`**；manifest 才在階段 B 建立）——此時所有 project import 都已發生；
* **階段 B**：**才建 `finalizer_provenance` 與 manifest**，放進 staging；
* ⚠️ 階段 B 之後的封閉 schema 檢查與來源複核**⛔ 不得再產生任何新的 project import**。

⚠️ **「import 都已發生」需要可執行的保證，⛔ 不能只用文字宣告**（v11）：
`build_provenance()` 的 dict literal 裡 **`project_module_hashes()` 排在 `runtime_settings()`
之前**求值（`provenance.py:133-147`），而後者在 `config_module is None` 時會 **lazy-import
`config`**——⚠️ 而 **`config` 正是專案模組**（`PROJECT_MODULE_NAMES = ("config", "db")`，`:26`）。
所以 finalizer 若沒先載入 config 就呼叫 builder，**`config` 的 hash 會被漏記**。

* **階段 A 要預先 `import config` 並把它以 `config_module=` 傳進 builder**；
* **階段 B 完成後重算一次 project-module mapping，斷言與 manifest 裡的 provenance 完全相同**
  ——這才是「⛔ 不得新增 import」的**實際守門**，⛔ 不是註解。

沿用 `publish.py` 既有機制：**`probe_no_clobber()` 先實測目錄 rename 語意** → **sibling staging**
建完整 tree → **階段 A 全圖驗證** → **階段 B 建 provenance 與 manifest** → fsync →
**`rename_noreplace()` 一次發布整個 root**
（正式路徑「**不存在** → **完整**」）→ rename 前任一步失敗**只 `remove_tree(staging)`**，⛔ 不碰正式路徑。

⚠️ **rename 成功不等於落盤——commit point 之後是另一種狀態**（v8 新增）。
既有 bundle 發布已經處理過這個分界（`bundle.py:362`）：rename 後還要 fsync 正式 root 的
**parent**，失敗時拋 `DurabilityUnconfirmed` 並明寫「⛔ 不刪除、不重來」。
⛔ **v7 的「finalizer 失敗一律回 1」在這個狀態下是錯的**——那時正式 root **已經存在且有效**。

| 階段 | 失敗處置 |
|---|---|
| rename **前**（建 staging、全圖驗證、逐一 `fsync_file` **10 個檔案** ＋ 所有巢狀目錄的 `fsync_dir`） | `remove_tree(staging)`、**exit 1**，正式路徑仍不存在 |
| `rename_noreplace()` 本身 | 同上（`FileExistsError` 代表正式 root 已存在 → ⛔ 不覆蓋，exit 1） |
| rename **後** fsync `python/baselines/` 失敗 | ⚠️ **正式 root 保留、⛔ 不刪除**，回 **`EXIT_DURABILITY_UNCONFIRMED = 3`** |

⚠️ **復原路徑要用專屬模式，⛔ 不能靠「偵測到 root 已存在」分流**（v9 修正）：
「重複發布」與「durability recovery」**從檔案狀態看完全相同**——都是正式 root 已存在。
v8 同時規定前者 exit 1、後者回 0，實作者無從判斷該走哪條。**定案：`--recover-durability` 明示模式**：

| 模式 | root 已存在時 |
|---|---|
| 一般 finalizer | ⛔ **仍回 1**（那是重複發布，⛔ 不覆蓋、⛔ 不當成成功） |
| **`--recover-durability`** | **要求 root 必須已存在**；⛔ 不讀 operational inputs、⛔ 不重新壓縮、⛔ 不覆寫任何檔案；**只驗證既有 10 個檔案（全圖驗證，⚠️ 含歸檔的 run identity）並重新 fsync parent**，通過後⚠️ **依已驗證的 crossday artifact 還原原始結果**（見下） |
| `--recover-durability` 但 root 不存在／缺檔／驗證不過 | **回 1** |

⚠️ **recovery ⛔ 不得固定回 0——那會讓 mismatch 的 5 永遠消失**（v10 修正）。情境是：
crossday mismatch（5）→ finalizer 已發布但 parent fsync 失敗（**3 覆蓋 5**）→ recovery 成功（**0**）。
⛔ 最後**沒有任何一次成功結束的指令回傳過 5**，機器端會把 recovery 的 0 讀成「整組 Stage 1 成功匹配」。
**定案**：recovery 成功 fsync 後，**從已通過驗證的 `crossday_artifact.json.gz` 讀 `outcome`**
還原終端結果——**`MATCH` → 0**、**其餘三種 → 5**。測試要**分別覆蓋原始 0 與原始 5 兩條路徑**。

**全圖驗證**：D／D+1 的 after 與 cohort 的 schema ＋ **diagnostics**；**cohort 記的 SHA 確實指向
對應的 after**；**crossday 以這兩份實際 after 重跑 `validate_crossday()`**；probe completion
**指向實際的 computation／measurement**，且⚠️ **呼叫三份完整的 probe schema validator**
（v9 修正——v8 只寫「指向」，那會讓「computation、completion SHA 與 manifest 被同步改掉」的
**無效 probe** 照樣歸檔），並驗**三份 provenance 完全相同、確實來自同一個 runner invocation**；
**所有檔案同一個 `bundle_id`**；
**provenance 依 role 驗證**；⚠️ **所有 provenance 的 `image_digest` 等於 manifest 記的
`expected_image_id`**（見十三）。

⚠️ **manifest 的每一項 metadata 都要從實際 archive 重算並逐項比對**（v10 新增——v9 只定了型別，
而 **recovery 完全依賴既有 root**，只驗型別的話一份索引值錯誤的 manifest 照樣通過）：

| 欄位 | 必須等於 |
|---|---|
| `artifact_sha256` | **同一次讀取**所解壓內容的 SHA |
| `stored_sha256` | **同一次讀取**的 gzip raw bytes 的 SHA |
| `stored_bytes` | 該 raw bytes 的**長度** |

並且 manifest 的 **`bundle_id` 與 `expected_image_id` 要與**歸檔的** `identity/run_identity.json.gz`
比對**（⚠️ **recovery 也走這條**，⛔ 不依賴 repo 外那份）；正常 finalization 時**另外**確認歸檔的
這份與 repo 外那份逐欄相同（十三-B）。
測試要含**三種 metadata 各自被竄改**的拒絕案例。

**`evidence_manifest.json` 的封閉 schema**（v8 補型別）：

| 欄位 | 型別 |
|---|---|
| `schema_version` | `type(x) is int` 且 **== 1** |
| `kind` | == `"sr_zone_evidence_manifest"` |
| `bundle_id`／`generated_at` | 非空 str（後者含時區 RFC3339） |
| `expected_image_id` | **`sha256:` ＋ 64 lowercase hex** |
| `files` | mapping：**key 精確等於那 9 個 archive 相對路徑**（⚠️ manifest 排除自身，故是 9 不是 10）；value 為 `{artifact_sha256, stored_sha256, stored_bytes}`，兩個 SHA 皆**裸 64 lowercase hex**、`stored_bytes` 為 **`type(x) is int` 且 > 0** |
| `finalizer_provenance` | 通過 `validate_provenance(role="finalizer")` |

⛔ 缺欄／多欄／`files` key 不符一律拒絕。

##### 十二、probe 三檔的封閉 schema 與**不變條件**

三份都是 operational `.json`，都有 `schema_version`（== 1）、`kind`、`bundle_id`、`generated_at`、
`provenance`。⚠️ **⛔ 不設頂層 `argv`**（v7 修正）——只留 `provenance.argv`，⛔ 不製造雙真相源。
⚠️ 三份的 provenance **role 都是 `stage1`**，且 measurement／completion 是**同一個 runner
invocation 的外層產物**，⛔ 不各自虛構一份執行身分。

**共同欄位的型別契約**（v9 補——⚠️ 與 crossday 同一條理由：`True == 1`、`1 == True`，
只寫「== 1」而不定型別的話，`True` 會通過）：

| 欄位 | 型別 |
|---|---|
| `schema_version` | **`type(x) is int`** 且 == 1 |
| `kind` | 各自的常數字串 |
| `bundle_id` | **非空 str** |
| `generated_at` | 非空 str、**含時區 RFC3339** |
| `provenance` | 通過 **`validate_provenance(role="stage1")`** |
| `completed`（completion） | **`type(x) is bool` 且 `is True`** |
| 兩個 `*_artifact_sha256`（completion） | **裸 64 字元 lowercase hex** |

測試要含「用 `True` 冒充 `schema_version`」「用 `1` 冒充 `completed`」「大寫 SHA」
「`generated_at` 缺時區」四組拒絕案例。

⚠️ **專屬欄位同樣要嚴格型別，⛔ 不能只靠數值等式**（v10）：`200.0 == 200` 為真，所以
`row_count` 只寫「== 200」會放行 float。測試補 **`row_count=200.0`**、
**`elapsed_seconds=True`**（bool 是 int 的子類）與 `quota_by_symbol`／`rows` **容器型別錯**三組。

| 檔案 | `kind` | 專屬欄位 |
|---|---|---|
| `capacity_probe_computation.json` | `sr_zone_probe_computation` | `quota_by_symbol`（**mapping**）、`row_count`（**`type(x) is int`**）、`rows`（**list**）、`elapsed_seconds`（**`type(x) is float`** 且 > 0） |
| `capacity_probe_measurement.json` | `sr_zone_probe_measurement` | `peak_rss_bytes`／`host_low_bytes`／`cgroup_limit_bytes`（**皆 `type(x) is int` 且 > 0，單位 bytes**，⛔ 不得 null 或 0 佔位） |
| `capacity_probe.json`（**最後寫**） | `sr_zone_probe_completion` | `computation_artifact_sha256`／`measurement_artifact_sha256`（前兩份的 **`artifact_sha256`**，64 hex）、`completed`（必須 `true`） |

**computation 的不變條件**（v7 新增）：

* **`row_count == len(rows) == sum(quota_by_symbol.values()) == 200`**；
* **`quota_by_symbol` 的 key 恰為 preflight 的 11 個 symbols**；
* 每個 quota 是**嚴格正整數**（`type(x) is int` 且 > 0，⛔ 排除 bool）；
* **`rows` 的 key 唯一**，且**每個 symbol 的實際列數等於它的 quota**；
* `rows` 必須通過 **`validate_replay_errors()` 與 `validate_diagnostics()`**。
  ⚠️ **守門要先搬家**（v8）：它目前是 `evaluation.py` 私有的 `_assert_no_replay_errors()`
  （`evaluation.py:2954`），而 `replay_bundle` ⛔ 不得反向 import `evaluation.py`；另抄一份又是
  **雙真相源**（`NO_ZONE_SCORES_ERROR` 已經犯過一次）→ **移到 `replay_bundle/artifacts.py`
  改成公開的 `validate_replay_errors()`**，`evaluation.py` 與 probe／finalizer **共用同一份**。
  ⛔ **`NO_ZONE_SCORES` 仍是唯一合法例外，行為一字不改。**

⚠️ **peak／host low／cgroup limit 任一取得失敗 → ⛔ 不發布 completion**
（`run-evaluation.sh:261` 現在是「警告 ＋ 寫 0」，正式 probe ⛔ 不得把「量不到」歸檔成 0）。
三份都納入受管制清單。

##### 十三、image pin：唯一入口與**全部消費者**

* **唯一入口 `scripts/pin-replay-image.sh`**：build → inspect 取 ID →
  **stdout 只印 machine-readable 的 `sha256:…`**（其餘訊息走 stderr）；
* ⚠️ **消費者是五個**（v7 修正——v6 漏了 probe，那會讓容量驗證量到**另一個 image**）：
  **capacity probe、D、D+1、crossday comparator、evidence finalizer**。
  其中 **probe／D／D+1 必須使用完全相同的 image ID**；
* `evidence_manifest.json` 保存 **`expected_image_id`**，由**全圖驗證**確認各 provenance 的
  `image_digest` 與它一致；⚠️ **probe／D／D+1 的 image ID 相同這件事要在執行時就驗**
  （v8）——⛔ 不能只留到 finalizer 最後才發現，那時三趟都跑完了；
* ⚠️ **runner 無 `REPLAY_IMAGE_ID` 時的行為定案**（v7 修正——v6 只寫「有 ID 時禁止 build」，
  留下兩種都符合文字的實作）：**runner 自己呼叫 `pin-replay-image.sh` 取得 ID，再一律以 ID 執行**。
  這樣既保留「不設環境變數也能跑」的既有使用方式，又維持**單一 build 實作**；
  **正式模式則要求預先提供 ID**——⚠️ **「正式模式」的判定定死**（v8）：
  帶 **`--i074-preflight` 或 `--i074-capacity-probe`** 即視為 I-074 正式流程，
  **缺 `REPLAY_IMAGE_ID` 立即拒絕**（⛔ 不自動 pin）；**crossday 與 finalizer 一律要求** ID；
  其餘既有 Stage 1／2 才允許無 ID 時自動呼叫 pin script；
* ⛔ **衝突變數精確列為 `PY_IMAGE`**，同時設定即拒絕；
* ⚠️ **跨日的共同狀態由 run identity 檔承擔**——完整 contract 見十三-B。
* ⚠️ **所有模式最終都以 image ID 執行**——⛔ 不用 tag（現行 `:176` 用 `$IMAGE`，build 與 run
  之間有 tag 移動窗口）。測試要證明**無 pin 路徑也是 `docker run <image-id>`**。

##### 十三-B、⚠️ run identity 檔的完整 contract（v10）

⛔ **v9 只寫「repo 外、原子建立、no-clobber、至少兩欄」，那不足以唯一實作**——producer、路徑、
schema、重入語意、取得方式全都沒定，不同實作者會做出互不相容的流程。

| 項目 | 定案 |
|---|---|
| **producer** | **`scripts/pin-replay-image.sh`**（它已是唯一 build／pin 入口，image ID 就在它手上） |
| **位置** | **repo 外的持久目錄**：`${XDG_DATA_HOME:-$HOME/.local/share}/stock_trading/i074_stage1/run_identity.json`。⛔ **不得放 `/tmp`**——那會被清掉，而這份要跨日存活 |
| **封閉 schema** | `schema_version`（`type(x) is int` 且 == 1）、`kind`（== `"sr_zone_run_identity"`）、`bundle_id`（非空 str）、`expected_image_id`（`sha256:` ＋ 64 lowercase hex）、`created_at`（含時區 RFC3339）。⛔ 缺欄／多欄一律拒絕 |
| **序列化** | `canonical_json_bytes()` ＋ 同目錄 temp ＋ `rename_noreplace()`（與既有原子發布同一機制） |
| **validator** | `validate_run_identity()`，與其他 artifact 同樣是封閉 schema ＋ 型別 |
| **重入語意** | ⚠️ **依「檔案存不存在」分流，⛔ 不比對 `created_at`**（見下） |
| **消費者** | ⚠️ **依模式分流**（v18 修正）：**probe／D／D+1 三趟 runner ＋ comparator ＋ normal finalizer** 讀**這份 repo 外的協調檔**；⛔ **`--recover-durability` 例外**——它**只讀 evidence 內的 archived copy**（`identity/run_identity.json.gz`），⛔ 不碰 repo 外那份 |
| **比對時機** | ⚠️ **依角色分成四類**（v20 修正）——⛔ **「有沒有 bundle path」才是分界**，見下表 |

⚠️ **v19 把「Docker 前驗四種」套到了全部一般消費者，但只有 runner 手上有 bundle path**：
`compare-replay-crossday.sh` 只收 `--d`／`--d1`／`--output-dir`（十三-C 的 A），
normal finalizer 只收 evidence root ＋ 8 份 operational artifact ＋ identity ＋ provenance 參數
（同表）——**兩者都⛔ 沒有 bundle path**，照 v19 的文字⛔ 無法唯一實作。

| 角色 | 缺檔／schema／`expected_image_id`／image 存在 | **`bundle_id` 的比對時點** |
|---|---|---|
| **probe／D／D+1**（runner） | **Docker 前** | **Docker 前**——⚠️ 它們有實際的 `--bundle` path 可當第二來源 |
| **comparator** | **Docker 前** | **容器內**：來源驗證（兩份 after artifact 讀進來）之後、**crossday artifact 發布之前**——⚠️ **需要 identity 當獨立第二來源**，見下 |
| **normal finalizer** | **Docker 前** | **容器內**：**階段 A**，**manifest／fsync／rename 之前**（v21 修正——既定流程是**先建 sibling staging、才做階段 A 全圖驗證**，所以⛔ 不能承諾「staging 前」；失敗時**清除 staging、正式 root 不存在**） |
| **`--recover-durability`** | **Docker 前** | **容器內**：全圖驗證的「所有檔案同一個 `bundle_id`」，**fsync 之前** |

⚠️ **comparator 也要掛 identity**（v21 修正）：v20 說它「載入兩份 after 後比對」，但
`compare-replay-crossday.sh` 只有 `--d`／`--d1`／`--output-dir`，⛔ **沒有把 identity 掛入或注入**
——兩份 after 只能**互相**比對，**若兩份都帶同一個錯誤 `bundle_id`，就沒有任何獨立來源能發現**。
→ **comparator 比照 normal finalizer**：**same-path 唯讀掛載** ＋ 受保護的 **`--run-identity`**
（自己的 ownership checker、`allow_abbrev=False`、拒絕重複），以 identity 對照兩份 after 的
`bundle_id`。新增 **`python/scripts/fixtures/comparator_argv.json`**，並補
**comparator 的 argv／mount／ownership 測試**。

⚠️ 四類的共同點是**前四項一律在 Docker 前**；差別只在 **`bundle_id` 要等到哪一刻才有第二來源**。
⛔ **不得為了統一而在 comparator／finalizer／recovery 的 host 端補一個 bundle path**——
那等於把 operational input 帶進不該有它的角色。

⚠️ **v10 的重入語意自相矛盾**：pin script 是 build → inspect，而 identity 含**每次都會變的
`created_at`**，所以「所有欄位完全相同才 no-op」**永遠不成立**；而且重新 build 也⛔ 不保證得到
相同的 image ID。**定案：依檔案存不存在分流**：

| identity | pin script 的行為 |
|---|---|
| **已存在** | 驗 schema ＋ 確認 `bundle_id` 與請求的相同 → **`docker image inspect` 確認其中的 image ID 仍存在** → **直接輸出既有 ID**。⛔ **不 build、⛔ 不產新 `created_at`、⛔ 不重寫檔案** |
| **不存在** | 才 build → inspect → 產生**一次性**的 `created_at` → 原子發布 |
| 已存在但**該 image 已不在本機** | ⚠️ **fail-closed（exit 1）**——⛔ **不得重建後換一個 ID**，那會讓跨日的三趟跑在不同 image 上 |

⚠️ **identity 也要有 commit-point 狀態機**（v12——v11 只說「要 fsync」，⛔ 沒定失敗處置）。
⚠️ 少了它會有一個**永遠修不好**的狀態：rename 成功但 parent fsync 失敗後 identity 已存在，
下次執行走「既有 identity」分支直接回傳 ID 而**不重新 fsync**，durability 就再也不會被確認。
**比照 evidence root 定案**：

| 階段 | 處置 |
|---|---|
| temp 寫入後 `fsync_file` 失敗 | **pre-commit**：清 temp、**回 1**，正式路徑仍不存在 |
| `rename_noreplace()` 成功、parent `fsync_dir` 失敗 | ⚠️ **保留正式 identity**、**回 3** |
| **既有合法 identity 的 no-op 路徑** | ⚠️ **也必須重新 `fsync_file` ＋ parent `fsync_dir`**，成功才回 0——**這就是上述狀態的修復路徑** |
| **no-op 路徑的兩次 fsync 任一失敗** | ⚠️ **保留 identity**、⛔ **不得 build 或重寫**、回 **3**（v13 補——v12 只定義了成功回 0） |
| 既有 identity 但 image 已不在本機 | 仍 **fail-closed（1）**，⛔ 不因 recovery 而重建 |

⚠️ **非零結果時⛔ 不得在 stdout 輸出 image ID**（v13）：stdout 是消費者讀取 ID 的唯一管道，
失敗卻照樣印出來，下游會拿著一個 durability 未確認的 ID 繼續跑。
⚠️ **v14 修正**：上面這段在 v13 被夾在**表格中間**，把最後一列切出了表外——Markdown ⛔ 不會把
它算進狀態表。⚠️ 這與 I-100 Stage 0 計畫書 v13 踩到的是**同一種錯**（表格被段落截斷）；
增補式修訂要特別注意**插入點是否落在表格內部**。
⚠️ **⛔ 移除 `I074_RUN_IDENTITY`**（v11）——「只供測試」在實作上**無法強制**。
測試改為**覆寫 `XDG_DATA_HOME`**，正式流程一律用固定推導值。

**消費端的改動**：`scripts/run-replay-offline.sh`、`scripts/compare-replay-crossday.sh`、
`scripts/finalize-evidence.sh` 各自在 `docker run` 前讀取並比對；`scripts/test-replay-args.sh`
補 shell 測試矩陣：**四類角色的前四項**（缺檔／schema／`expected_image_id` 不符／image 不存在）
**都要在 Docker 啟動前**被拒絕；⚠️ **`bundle_id` 那一項只有 runner 在 Docker 前**，
comparator／finalizer／recovery 各自在容器內的對應時點（見上表）——
⛔ 測試⛔ 不得承諾某個角色的 host 端做不到的檢查。producer 與 schema 見二的檔案表。

⚠️ **容器內怎麼讀到它——normal finalizer 的資料流**（v12 新增）：finalizer 跑在容器裡，而
identity 在 **host 的 repo 外**；⛔ **容器內⛔ 不能自己依 `XDG_DATA_HOME` 重新推導**——容器的
`HOME` 與環境和 host 不同，推出來的是另一個路徑。**定案**：

| 模式 | 資料流 |
|---|---|
| **normal finalization** | shell 推導並驗證 **host 的絕對路徑** → ⚠️ **same-path 唯讀掛載**（見下） → 由官方腳本注入受保護的 **`--run-identity <該同一個絕對路徑>`** |
| **`--recover-durability`** | ⛔ **不掛載、不注入**外部 identity——**只讀 archived 的那份**（`identity/run_identity.json.gz`） |

⚠️ **`--run-identity` ⛔ 不得加進既有的兩份 injected-args 清單**（v13 修正——v12 寫錯了）：

* `SCRIPT_INJECTED_ARGS` 在 `evaluation.py:3186`，那是 **evaluation CLI** 的；
  而 finalizer 規劃在 **`replay_bundle/evidence.py`**，⛔ 根本不走那個 parser；
* shell 側的攔截由 `REPLAY_INJECTED_ARGS`（`scripts/lib/replay-args.sh:16`）控制，
  而它會**連唯一前綴縮寫一起擋**。⚠️ **實測（2026-09-14）**：把 `--run-identity` 加進去之後，
  **既有合法的 `--run-id` 立刻被拒絕**，訊息還會誤稱它「會被展開成 `--run-identity`」——
  ⛔ 那會直接弄壞 Stage 1／2 的既有參數。

**定案**：

| 項目 | 做法 |
|---|---|
| 所有權檢查 | finalizer **自己的** injected-args ownership checker，⛔ **不共用** `REPLAY_INJECTED_ARGS` 的前綴清單 |
| parser | `evidence.py` 的 finalizer parser 自設 **`allow_abbrev=False`** ＋ 自行檢查**重複參數** |
| 模式規則 | **normal 模式要求** identity path；**recovery 模式明確禁止**（傳了即拒絕） |
| fixture | **`python/scripts/fixtures/finalizer_argv.json`**，⛔ 不把 finalizer 參數塞進 `stage1_argv.json`；⚠️ **分別保存 normal 與 recovery 兩組 argv** |
| fixture 的用法 | ⚠️ **兩端共用同一份**：shell 測試斷言**官方腳本實際產生的 argv 與 fixture 逐 token 相同**；`test_evidence.py` 用**同一份** fixture 餵 finalizer parser（normal 恰好一個 identity path、recovery ⛔ 不含）。⛔ 只驗行為而不比對 argv，等於沒有釘住兩端的契約 |
| fixture 的來源 | ⚠️ **依十三-C 的 CLI matrix 建立**，⛔ 不是由實作者自選 argv 再凍結——後者只凍結「實作者選了什麼」，⛔ 證明不了它符合已裁決的契約 |
| 回歸 | ⚠️ 必須有測試確認**既有合法的 `--run-id` 仍可使用** |

##### 十三-C、⚠️ finalizer 的完整 CLI matrix 與 recovery 的 image 守門（v15）

⛔ **v14 的缺口**：十三要求 finalizer 一律以指定 image ID 執行、十三-B 要求 identity 在
`docker run` **之前**比對，但 recovery **只讀 evidence 內的 archived identity**（一個 `.json.gz`）
——**「Docker 啟動前讀 gzip 內的 identity」沒有定義路徑**。照 v14 的文字，實作者只能三選一，
而三條都不可接受：①先用**尚未驗證**的 image 啟動 Docker 再在容器內檢查（違反「Docker 前拒絕」）；
②**完全不比對** recovery 的 image（recovery 會在另一個 image 上完成）；③各自發明 host 端驗證入口
（不同實作不一致，且是**雙真相源**）。

**A-0、⚠️ `--run-identity` 的路徑語意：same-path bind mount**（v16 定案）

⛔ **v15 自相矛盾**：十三-B 寫「注入**容器內**絕對路徑」，十三-C 的 matrix 卻寫
「`--run-identity <**host** 絕對路徑>`」。⚠️ 除非明訂掛載方式，否則 Python finalizer 會收到
**容器內不存在**的 host path。

**定案：沿用既有慣例 same-path bind mount**——`run-replay-offline.sh:113` 對 bundle 與 output dir
就是這樣做的（`-v "$ABS":"$ABS":ro`），註解寫明理由：**「掛在與 host 相同的絕對路徑上，
這樣參數不用改寫，也就不會改寫錯」**。

| 層 | 內容 |
|---|---|
| shell | 推導 identity 的 **host 絕對路徑** `$ID_ABS` 並驗證存在 |
| docker | `-v "$ID_ABS":"$ID_ABS":ro`——⚠️ **同一個絕對路徑**，⛔ 不改寫 |
| python argv | `--run-identity "$ID_ABS"`——host 與 container 看到的是同一個路徑，⛔ **不存在兩種路徑之分** |
| fixture | 保存該 argv（動態路徑用 placeholder，與 `stage0_argv.json` 同一慣例） |
| shell 測試 | 另外斷言 **mount 是 same-path 且為 `:ro`** |

**A、CLI matrix**（⚠️ fixture 依這張表建立，⛔ 不是由實作者自選 argv 再凍結——
fixture 只能凍結「實作者選了什麼」，⛔ 證明不了它符合已裁決的契約）：

| | **normal finalization** | **`--recover-durability`** |
|---|---|---|
| evidence root | **必填** | **必填** |
| 8 份 operational artifact（D 的 after＋cohort、D+1 的 after＋cohort、crossday、probe ×3） | **全部必填** | ⛔ **一律禁止** |
| `--run-identity <絕對路徑>` | **必填**，且⚠️ **只能由官方腳本注入**（finalizer 自己的 ownership checker）；⚠️ **same-path**，見十三-C-0 | ⛔ **禁止**（傳入即拒絕） |
| `--recover-durability` | ⛔ **禁止** | **必填** |
| provenance 注入參數（`image_digest`／`base_commit`／`tooling_patch_sha256`／`source_root`／`runner_sha256`） | 由腳本依十三-B 的推導流程取得後注入，**寫進新建的 `finalizer_provenance`** | ⚠️ **同樣注入，但用途不同**——recovery ⛔ 不建新 provenance，改**逐欄比對**，見 A-2 |

**B、recovery 的 image 守門——host 端、Docker 之前**：

recovery shell 在 `docker run` **之前**，以**host 端、dependency-light 的入口**
讀取並**完整驗證** `<evidence-root>/identity/run_identity.json.gz`：
canonical gzip **round-trip byte-identical** → 解壓 → canonical JSON → **`validate_run_identity()`**
（⚠️ **同一份 validator**，⛔ 不在 shell 裡另寫一套 schema 檢查）。
接著把其中的 **`expected_image_id` 與 `REPLAY_IMAGE_ID` 比對**，並以 `docker image inspect`
**確認該 image ID 存在**。⚠️ **任一不符 → Docker 尚未啟動就中止。**

⚠️ **這一關⛔ 不驗 `bundle_id`**（v19）：recovery ⛔ 不讀 operational inputs，**host 端沒有第二個
可信的 bundle_id 來源**——⚠️ **validator 雖然支援 `--bundle`，但 recovery ⛔ 不傳**（v23 澄清：
它手上根本沒有 bundle 路徑），所以「與誰不符」無從判斷。`bundle_id` 與 manifest／其他 evidence 的關係由**容器內的全圖驗證**
（「所有檔案同一個 `bundle_id`」）負責，**在 fsync 之前**拒絕。
⛔ **不得為此在 host 端補一個「可信 bundle_id 來源」**——那等於把 operational input 帶回 recovery，
違反它「只依賴既有 root」的前提。

⚠️ **這個入口要有正式、可測的落點，⛔ 不能散在 shell heredoc 裡**（v16 修正）：

| 項目 | 定案 |
|---|---|
| 檔案 | **`python/scripts/validate-i074-run-identity.py`**（列入受影響檔案表） |
| 用法 | `validate-i074-run-identity.py <identity 路徑> --expect-image-id sha256:… [--bundle <bundle 目錄>]` |
| ⚠️ **`--bundle`**（v22 新增、**v23 改為收路徑**，**可選**） | ⛔ v21 的 CLI 只收 identity path 與 `--expect-image-id`，於是 **runner 要做 Docker 前的 `bundle_id` 比對就只能在 shell 再解析一次 identity**——那是**雙真相源**且兩次讀取之間有 **TOCTOU**。⚠️ **v23 進一步修正**：v22 寫「`--expect-bundle-id`，值取自實際 bundle」仍未唯一化——**取目錄 basename／直接讀 `manifest.bundle_id`／經正式 loader 驗證**是**三種強度不同**的做法，前兩種擋不住偽造。→ **改收 `--bundle <目錄路徑>`，由 validator 自己呼叫既有的 `load_bundle()`**（`bundle.py:401`，做**三方相等**與**完整 hash 驗證**）取得 `bundle_id` 再與 identity 比對。⚠️ **這比傳值更強**：shell ⛔ 完全不解析，`bundle_id` 的來源只有一條路，而且 identity 與 bundle 在**同一個行程**內讀完——⚠️ **精確地說（v24 修正，v23 的「連 TOCTOU 都消除」講過頭了）：它消除的是「shell 與 validator 各解析一次 identity」那個窗口**；⛔ **兩個路徑仍是依序讀取**，且 **validator 結束到 `docker run` 之間仍有窗口**。⚠️ 那一段由**容器內的正式 loader 在 replay 前重新完整驗證**兜底——本計畫的威脅模型⛔ 不處理並行的外部竄改。✅ **實測（2026-09-14）**：`bundle.py` 只依標準庫與同 package 模組，**在 host（無 pandas）能跑完 `load_bundle()`** 並取得正確的 `bundle_id`。**probe／D／D+1 必須傳**；**comparator／normal finalizer／recovery ⛔ 不傳**。**不符 → 非零退出且 stdout ⛔ 無輸出** |
| ⚠️ **兩種格式** | v21 修正——⛔ v20 只定義了 archived `.json.gz`，但 **probe／D／D+1／comparator／normal finalizer 讀的是 repo 外的 plain `run_identity.json`**，⛔ 沒有入口就只能各自發明解析方式（**雙真相源**）→ **同一支 validator 同時支援 `.json` 與 `.json.gz`**：`.json` 驗 **raw bytes == `canonical_json_bytes(parsed)`**；`.json.gz` 驗 **round-trip byte-identical** 後解壓再驗 canonical；⚠️ **兩條路最後共用同一個 `validate_run_identity()`** |
| stdout | ⚠️ **成功時只印 `expected_image_id` 一行**（machine-readable），其餘訊息一律走 stderr |
| exit code | 0 ＝ 通過；**非 0 ＝ 拒絕**，且 stdout ⛔ 無輸出 |
| package bootstrap | ⚠️ **封裝在這個檔案裡**，⛔ 不散落在 shell |

⚠️ **這條路徑實測可行**（2026-09-14）：host 沒有 pandas，而
`backtest/modular/sr_scoring/__init__.py` 會 `import pandas`——但用**最小 package context**
（`types.ModuleType` ＋ `__path__`）繞過它之後，`canonical`／`publish`／`artifacts` **含相對 import
都能在 host 載入**。⚠️ **這個技法要當成受測的正式相容層**，⛔ 不是「記錄一次人工實測」就算數：
測試必須涵蓋「**host 環境沒有 pandas 時仍能正常驗證**」以及「**malformed gzip／schema 一律非零退出**」。
⛔ **因此有一條硬性約束**（v24 擴大範圍、⚠️ **v25 修正措辭**）：
**host validator 實際執行的 import／call graph 必須 dependency-light**——
⚠️ v23 讓它多呼叫 `load_bundle()`，涉及的模組因此從 `canonical`／`publish`／`artifacts`
擴大為 **`canonical`／`publish`／`calendar`／`artifacts`／`bundle`／`run_identity`**
（⛔ v23 的約束只套在 `run_identity.py`，涵蓋不到新增的這兩個）。
該路徑上⛔ **不得**（直接或間接）碰 pandas／sklearn／lightgbm 等第三方套件，否則守門會失效。

⚠️ **⛔ 這條契約⛔ 不是「整個模組只能 import 標準庫」**（v25 修正——v24 那樣寫與現況直接衝突）：
`calendar.py` 的**線上抓取分支** `fetch_year_rows()`（`:110`）裡有 **lazy `import httpx`**（`:152`），
那是第三方套件。但 **loader 走的是 `validate_calendar_payload()`（`:272`），⛔ 不經過那個分支**，
所以 lazy import 從不發生。
⛔ **不為此把 HTTP 抓取移出 `calendar.py`**——那會動到 I-100 已收斂的模組，也違反本計畫
「⛔ 不改通用未啟用路徑」的範圍宣告；lazy import 本來就是為了這種情形而存在的。

✅ **實測（2026-09-14）**：在 **`python3 -S`**（確認 site-packages ⛔ 不在 `sys.path`）下，
`canonical → publish → calendar → artifacts → bundle → load_bundle()` **全鏈跑通**並取得正確的
`bundle_id`。⚠️ **這個實測證明的正是「實際呼叫路徑不碰第三方」**——⛔ 它證明不了（也不需要證明）
「整個 `calendar.py` 只依標準庫」，因為 `python3 -S` 下若那條路徑真的碰到 `httpx` 就會 ImportError，
**測試本身就是這條契約的守門**。
⚠️ **一個陷阱要記著**：`replay_bundle/calendar.py` 與**標準庫的 `calendar`** 同名——
bootstrap 以 `rb.calendar` 之類的前綴註冊才不會撞名，⛔ 別用裸名載入。

**A-2、⚠️ recovery 為什麼也要注入那五個參數**（v16 裁決）

⛔ **v15 沒說用途**：matrix 要求 recovery 也注入五個 provenance 參數，但 recovery 正文只說
「驗既有證據並 fsync、⛔ 不建新 provenance」——那它們就成了沒有消費者的參數。

**定案：用來確認 recovery 跑的是同一份程式碼**（⛔ 不是拿來建新 provenance）。
recovery 以本次執行身分**逐欄比對** archived 的 `finalizer_provenance`：

| 欄位 | 比對？ | 理由 |
|---|---|---|
| `image_digest`／`base_commit`／`tooling_patch_sha256`／`runner_sha256`／`source_root` | ✅ **必須相同** | 這五個決定「跑的是哪一版程式碼與哪個環境」 |
| **`runtime_settings`** | ✅ **必須相同**（v17 改） | ⚠️ 它**⛔ 不是由 image 決定**——`config.py` 的五個值全是 **`os.getenv(...) or config.yaml`**（`config.py:14`／`:40`／`:54`／`:57`／`:60`），環境變數可覆寫，`TRADING_CONFIG` 甚至能換掉整個 config 檔。normal 與 recovery 本來就該在**相同的封閉環境**跑，直接逐欄比最清楚 |
| `argv` | ⛔ 不比 | normal 與 recovery 的參數**天生不同**（A 的 matrix） |
| `project_modules_sha256` | ⛔ 不比 | recovery ⛔ 不讀 operational inputs，**載入的模組集合本來就較少**，那是預期而非漂移 |
| `python_version`／`pip_freeze_sha256` | ⛔ 不比 | 這兩個**確實**由 image 內的 Python 與套件決定，`image_digest` 相同即涵蓋 |

⚠️ **任一比對欄位不符 → 在 fsync 之前中止**（⛔ 不得先 fsync 再報錯）。
⚠️ 比對欄位共 **6 個**（五個執行身分 ＋ `runtime_settings`）。

**C、測試**：跨模式參數（normal 帶 `--recover-durability`、recovery 帶任一 operational input 或
`--run-identity`）、缺必填參數、**recovery 的 image ID 與 archived identity 不符**——
⚠️ **三類都必須在 `docker run` 與任何寫檔之前被拒絕**。
**再加一類**：recovery 的**本次執行身分與 archived `finalizer_provenance` 的六個欄位任一不符**
（五個執行身分 ＋ **`runtime_settings`**）→ ⚠️ **在 fsync 之前中止**（A-2）。

##### 十四、I-100 條件 3 的 negative acceptance

複製正式 bundle 到暫存目錄——⚠️ **basename 必須仍是正式 `bundle_id`**（`load_bundle()` 要求
目錄 basename == `manifest.bundle_id`）→ 竄改任一 payload 或 `manifest.json` → 經官方入口執行
→ 斷言 **replay 前中止**且 output dir ⛔ 無任何 artifact。⛔ 不計正式 scan。
⚠️ **同時修 `development-workflow.md:930`**（「⛔ D 日產 bundle 並跑」）。

##### 十五、capacity probe 的執行語意

flag **`--i074-capacity-probe`**，quota **固定 200**（`MIN_ROWS_PER_SYMBOL = 5`，11 檔下限 55），
由 `_allocate_replay_quota()` 分配並**驗證 11 檔全部拿到**；**隱含執行 preflight-pre**，
⛔ 不得與 `--i074-preflight` 同時出現；⛔ 跳過 post 守門、⛔ 不發布 after／cohort。
⚠️ 結果**只作 sanity check**，⛔ 不得線性外推。

##### 十六、主要風險

| 風險 | 處置 |
|---|---|
| **記憶體**（最高） | 十五的 bounded probe（⚠️ 與正式同一個 image ID）；⛔ 不線性外推 |
| **時間**：約 3.2 小時／趟 | 背景執行、log 落地（**repo 外**）；⛔ 不用 pipe 掩蓋 exit code |
| **被 host OOM killer 砍掉呼叫端** | 開跑前清場；被砍後**先撈 log** |
| **兩趟 image 不同** | 十三的 pin ＋ 七的 `provenance_differences` ＋ 全圖驗證 |
| **兩趟結果不一致** | ⛔ 立案調查，**不得以重跑覆蓋或取代** |
| **假跨日通過** | 五 |
| **半包證據** | 十一的 staging ＋ 整包 `rename_noreplace` |
| **mismatch 時證據沒被保存** | 十-B 的 orchestrator |

##### 十七、測試與驗證策略

1. **preflight**：`bundle_id`／symbols／**末根台北日期**（UTC 是 08-31 的專屬案例）／`sum(quota)`。
2. **CLI matrix**：Stage 2 帶任一 flag → 拒絕；preflight ＋ probe 同時出現 → 衝突；重複 flag → 中止。
3. **crossday 正向**：rows 相同、僅 `generated_at` 與 output dir 不同 → `MATCH`。
4. **crossday 型別 tamper**（v7 恢復）：SHA 非 64 hex 或大寫；row count 用 `True`／負數；
   三個 flag 用 `1`／`0` 冒充 bool；key 三元組含空字串；`d_only` 未排序／有重複／與 `d1_only` 重疊；
   `d_key_order` 元素形狀錯；`row_differences` 含**空 `differences`**；
   **`outcome` 與三旗標的組合不符真值表**。
5. **crossday 內容 tamper**：兩份 exclusive rows 缺列／換列／順序錯；只有 row order 不同；
   `provenance_differences` 漏記／多記／值被改；`comparator_provenance` 缺欄或**格式合法但偽造**；
   五個身分欄位任一不一致；**漏記一筆真實差異 → 由來源重算抓到**。
6. **invalid input**：同檔／同 SHA／同日；來源 provenance 形狀錯 → 皆 exit 1 且⛔ 無 artifact。
7. **serialization**：JSON 語意相同但**非 canonical 編碼**（空白／key order）→ 拒絕；
   **gzip round-trip 不 byte-identical**（不同 compression level／header byte）→ 拒絕；
   gzip 損毀；錯副檔名；**cohort 記的 SHA 與實際 after bytes 不符**；
   ⚠️ **既有 bundle 的三個 payload 必須通過**（防止「照文件實作反而拒絕合法輸入」）。
8. **provenance**：`project_modules_sha256` **以 module name 為 key 的合法值必須通過**；
   `runtime_settings` **少一個鍵仍通過**、多未知鍵拒絕；nested 型別錯；
   `role="stage1"` 時 `base_commit` 為 null → 拒絕，`role="stage0"` → 通過。
9. **probe**：三檔缺欄／多欄／錯 SHA；**不變條件**各自違反（`row_count` 與 quota 和不符、
   quota key 不是 11 檔、quota 用 `True`、某 symbol 列數與 quota 不符）；
   **量測任一項缺失 → ⛔ 無 completion**。
10. **finalizer**：全圖驗證各項各自失敗；**中途失敗 → 正式 root 仍不存在、staging 已清除**；
    重複發布（no-clobber）；**未知 evidence** → 中止；manifest 排除自身；
    **`expected_image_id` 與某份 provenance 不符 → 中止**。
11. **orchestrator**：crossday 回 0 → finalizer 跑 → 回 0；回 5 → **finalizer 照樣跑 → 回 5**；
    回其他碼 → ⛔ 不跑 finalizer；finalizer 一般失敗 → 回 1；
    ⚠️ **finalizer 回 3（durability 未確認）→ orchestrator 回 3**，⛔ 不被原始 0／5 蓋掉
    （v9 修正——v8 的測試矩陣仍寫「一律回 1」，與同版新增的 rc 3 優先規則直接衝突）。
11-B. **durability**：rename 後 parent fsync 失敗 → **正式 root 保留**、回 3；一般模式在
    root 已存在時 → 回 1；
    `--recover-durability` 遇 root 不存在／缺檔／驗證不過 → 回 1。
11-C. **recovery 還原原始結果**（v11 補——⚠️ v10 改了正文卻漏改這裡）：
    已歸檔 `outcome == MATCH` → **recovery 回 0**；已歸檔**任一 mismatch** → **recovery 回 5**。
    ⛔ 兩條都要測——只測前者的話，「mismatch 的 5 被 recovery 吞成 0」正好測不出來。
11-D. **run identity producer 的三條分支**（v12 補）：**不存在** → build ＋ 發布；
    **既有合法** → ⛔ 不 build、⛔ 不改 `created_at`、直接輸出既有 ID；
    **既有但該 image 已不在本機** → **fail-closed 回 1**，⛔ 不得重建後換 ID。
11-E. **identity 的 commit point**（v12 補）：temp `fsync_file` 失敗 → 清 temp、回 1、
    正式路徑仍不存在；**rename 後 parent fsync 失敗 → 檔案保留、回 3**；
    ⚠️ **接著重跑 → 走 no-op 分支並重新 fsync → 回 0**（⛔ 這條就是那個狀態的唯一修復路徑，
    少了它 durability 會永遠未確認）。
11-E-2. **no-op 修復本身再次失敗**（v13 補）：no-op 路徑的 **`fsync_file` 失敗**、
    **parent `fsync_dir` 失敗**兩種各測一次 → 都要**保留 identity、⛔ 不 build／不重寫、回 3**，
    且 **stdout ⛔ 沒有 image ID**；再重試成功 → 回 0 且 **`created_at` 不變**。
11-F. **identity 與 evidence 的一致性**（v12 補）：**archived identity 與 repo 外那份不一致**
    → normal finalization 中止；⚠️ **repo 外那份已被刪除時，`--recover-durability` 仍能完成**
    （⛔ recovery 只依賴 archived 的那份）。
11-G. **`EvidenceLoad`**（v12 補）：四個欄位的值各自正確（`artifact_sha256` ＝ 解壓內容、
    `stored_sha256` ＝ raw bytes、`stored_bytes` ＝ raw 長度）；⚠️ **每個檔案只被讀取一次**
    （以 spy／計數斷言，⛔ 不是靠註解宣告）。
11-H. **階段 B 之後⛔ 不得新增 project import**（v12 補）：在階段 B 之後故意觸發一個新的專案模組
    import → **重算的 mapping 與 manifest provenance 不符 → 中止**。
12. **shell**：fake-docker 證明走到 `docker run`、原樣傳出 exit 5、
    **pinned 模式沒有執行 `docker build`**、**無 pin 路徑也是 `docker run <image-id>` 而非 tag**；
    `pin-replay-image.sh` 的 stdout 只有 image ID；
    ⚠️ **normal finalizer 以唯讀掛入 identity 並注入 `--run-identity`**，
    使用者自行傳入或重複傳入 → **拒絕**（由 finalizer **自己的** ownership checker 擋，
    ⛔ 不是 `REPLAY_INJECTED_ARGS`）；**recovery 模式傳入 `--run-identity` → 拒絕**、
    且⛔ 不掛載也不注入；⚠️ **回歸：既有合法的 `--run-id` 仍可使用**
    （⛔ 前綴攔截不得誤殺它——實測若共用清單會壞）；
    ⚠️ **argv 逐 token 比對**：normal 與 recovery 兩組實際 argv 各自等於
    `finalizer_argv.json` 裡對應的那組（⛔ 非 `eval` 字串，動態值用固定測試值或 placeholder，
    與 `stage0_argv.json` 同一慣例）；**Docker 唯讀 mount 由 shell 測試另外斷言**；
    ⚠️ **十三-C 的三類拒絕**（跨模式參數、缺必填、recovery image ID 與 archived identity 不符）
    **都要斷言發生在 `docker run` 與任何寫檔之前**。
12-A. **comparator 的 argv／mount／ownership**（v22 補——⚠️ v21 在正文新增了 comparator 的
    identity 契約卻漏了測試矩陣，**第六次違反守則二**）：官方腳本實際產生的 argv
    **逐 token 等於 `comparator_argv.json`**；**same-path 且 `:ro` 的 mount**；
    使用者自行注入／**唯一前綴縮寫**／**重複** `--run-identity` **三種都拒絕**
    （由 comparator **自己的** ownership checker 擋）；
    ⚠️ 三種都要斷言**發生在 `docker run` 與任何 artifact 寫入之前**。
    ⚠️ **核心產品分支**（v23 補——⛔ v22 的 12-A 只驗 argv／mount／所有權，
    **沒測到新增 identity 真正要抓的那件事**，identity 就算成功掛入、實作者漏掉實際比對仍會全綠）：
    **D 與 D+1 兩份 after 都合法且帶相同的 `bundle_id = B`、identity 合法但記 `bundle_id = A`**
    → comparator 必須**一般失敗**且⛔ **不得發布 crossday artifact**。
12-B. **host 端 validator**（v17 補；v19／v20／v23 修正承諾範圍——⚠️ v16 把它寫進正文卻漏了
    測試矩陣，**違反守則二**）：⚠️ **`bundle_id` 檢查是條件式的**（v23 統一）——
    **runner 必須傳 `--bundle`**，validator 以 `load_bundle()` 取得 ID 後比對；
    **comparator／normal finalizer／recovery ⛔ 不傳**（它們手上沒有可信的 bundle 第二來源）；⚠️ **`bundle_id` 的斷言依角色分四個時點**（十三-B 的表）：runner 在 **Docker 前**、
    comparator 在 **crossday 發布前**、normal finalizer 在**階段 A**、recovery 在 **fsync 前**
    ——⛔ 測試要按這四類分別描述，⛔ 不得把 runner 才有的 bundle path 套到其他角色；
    以**實際的 `python/scripts/validate-i074-run-identity.py`** 驗 valid identity 通過、
    **malformed gzip**、**錯 schema** 各自非零退出；⚠️ **兩種格式都要測**（v21）：
    repo 外的 plain `.json`（含**非 canonical 編碼 → 拒絕**）與 archived `.json.gz`
    （含**非 canonical gzip → 拒絕**）；⚠️ **`--bundle` 的 match／mismatch 各一組**
    （v22 提出、v23 改為傳路徑，以 plain `.json` 測）——**mismatch 要非零退出且 stdout 無輸出**；
    ⚠️ **不傳該參數時⛔ 不得因此失敗**（comparator／finalizer／recovery 的用法）；
    ⚠️ **runner 的接線整合測試**（v23 補）：由**官方 runner 實際**帶 `--bundle` 呼叫 validator，
    **identity 的 `bundle_id` 與該 bundle 不符時，在 `docker run` 與任何 artifact 寫入之前中止**
    ——⛔ 只直接測 validator 證明不了 runner 真的接上了這段；**stdout／exit code contract**
    （成功只印一行 image ID、失敗時 stdout ⛔ 無輸出）；
    ⚠️ 用 **`python3 -S`** 或等價隔離方式證明它**⛔ 不依賴 site-packages／pandas**——
    ⚠️ **且至少一組必須同時帶 `--bundle <合法 bundle>` 並真的跑完 `load_bundle()`**
    （v24 補：⛔ 只測不帶 bundle 的路徑會全綠，卻測不到 `bundle`／`calendar` 這兩個
    v23 才加進閉包的 import）；**再一組給損壞的 bundle → 非零退出且 stdout ⛔ 無輸出**。
12-C. **recovery 的執行身分比對**（v17 補，v18 釘死 tamper 的一側）：
    ⚠️ **archived evidence 必須維持完全合法且一個位元都不改**——⛔ **不得改 archived 端**：
    改了 `image_digest` 之類的欄位，會**先被既有的 manifest／image 一致性守門攔下**，
    那條測試就證明不了「本次身分 vs archived provenance」這個**新分支**真的生效
    （⚠️ 與前幾輪「測試沒刺激到它聲稱保護的分支」是同一類錯）。
    **改的是本次 recovery 這一側**：五個注入值（`image_digest`／`base_commit`／
    `tooling_patch_sha256`／`runner_sha256`／`source_root`）逐一改動，
    **`runtime_settings` 則由當次的 config／env 改動**（⚠️ 它本來就可由 env 覆寫）。
    斷言：**確實在身分比較這個分支中止**，且 ⚠️ **`fsync_dir` 尚未被呼叫、正式 root 完全未變**
    （以 spy 斷言，⛔ 不是只看 exit code）。
13. **smoke**：⛔ 不帶 flag；另補「帶 flag → replay 前因身分不符失敗」。
14. **I-100 條件 3**：十四。
15. **正式**：D 與 D+1 各一趟（**符號日期**），除 output dir 外參數正規化後相同、image ID 相同。

⚠️ **「evidence 已進版控」的驗收分三個時點**：實作期自動化測試**用 fixture**（⛔ 不碰正式
evidence）；正式執行後人工確認 **staged／tracked**；**本筆關閉前**用
**`git ls-tree -r <commit>`** 確認**目標 commit 確實包含**，⛔ 不是只在工作樹或 index 裡。

##### 十八、回滾／相容策略與歸檔位置

所有入口都是**純新增且 opt-in**，不帶 flag 時既有路徑行為完全不變。回滾單位＝本輪 diff。
歸檔：`sr-zone-scoring.md`（preflight／crossday／probe／evidence 四個契約）、
`development-workflow.md`（執行程序、evidence 規則、`:930` 修正）、I-100 的受管制清單與關閉條件。

##### 修訂紀錄

| 版本 | 內容 |
|---|---|
| v1～v3 | 起草；3 高 5 中 3 低（crossday 成功失敗都產出、兩個全新目錄、bounded probe、**台北交易日**、flag 限定 Stage 1）；3 高 6 中 3 低（**假跨日**、`matched` 拆解 ＋ `outcome`、CLI 落點分流、`REPLAY_IMAGE_ID`、exit 5 落點） |
| v4～v5 | 3 高 4 中 3 低（schema 補兩份 exclusive rows、**`row_differences[]` 改為重用 `compare_rows()`**、provenance 補 `source_root`、evidence gzip 進版控、`d_key_order`）；4 高 3 中（**operational／archived 兩層**、恢復 v4 弄丟的 validator 來源綁定、`validate_provenance`、probe 封閉 schema ＋ peak fail-closed、finalizer） |
| v6 | 4 高 3 中：`project_modules_sha256` 是 **module name → hash**（⛔ 不是相對路徑）；`runtime_settings` 是「⊆ 五個」；`artifact_sha256` 維持 **raw bytes** 語意；finalizer 改為 staging ＋ 整包 rename；probe 三檔只寫 operational JSON；`by_key` 由 validator 自建；`pin-replay-image.sh`；git tracked 驗收分三時點 |
| v7 | review 抓到 3 高 3 中，全部反映：①**crossday 的型別契約在 v6 精簡時遺失**——v6 只剩欄位名，而**來源重算⛔ 取代不了型別驗證**（Python 裡 `1 == True`，用 `1`／`0` 冒充 bool 可通過所有「重算後相等」的比對）→ **恢復完整型別表**（`kind` 常數、裸 64 lowercase hex、`type(x) is int` 非負、三個 flag 嚴格 bool、key 三元組形狀／排序／唯一／互斥、`d_key_order` 的完整 schema、`row_differences` ⛔ 不得含空 `differences`）並補 **`outcome` 真值表**；②**正式 evidence root 與 `logs/` 互相衝突**——finalizer 要求 root 必須不存在，v6 卻把 log 放在 root 底下，D 趟一開始建 log 目錄就讓最後的 `rename_noreplace()` **必然失敗** → **三個位置完全分離**（operational 與 log 都在 **repo 外**，archived root 在 finalizer 前⛔ 必須完全不存在）；⚠️ 並補上 v6 沒定義的 **orchestrator**：`compare-replay-crossday.sh` 一回 5，`set -e` 就會讓 finalizer 永遠不執行——而 mismatch 正是最需要保存證據的情況 → `scripts/run-i074-stage1.sh` 定死「捕捉 exit code → 只接受 0 或 5 → 跑 finalizer → 成功回原碼 → finalizer 失敗一律回 1」；③**capacity probe 沒綁定 pinned image**（v6 只列 D／D+1／crossday）→ 消費者定為**五個**（probe、D、D+1、comparator、finalizer），probe／D／D+1 必須完全相同 ID，manifest 存 **`expected_image_id`** 並由全圖驗證確認各 provenance 一致；④**probe 缺欄位間不變條件** → 補 `row_count == len(rows) == sum(quota) == 200`、quota key 恰為 11 檔、quota 嚴格正整數（排除 bool）、rows key 唯一且每檔列數等於 quota、rows 須過 replay error 與 diagnostics 守門；⚠️ **移除頂層 `argv`**（只留 `provenance.argv`，⛔ 不製造雙真相源），三份的 role 都是 `stage1` 且 measurement／completion 是**同一個 runner invocation 的外層產物**；⑤**canonical gzip ⛔ 不能只驗兩個 header 欄位**（compression level／XFL／OS byte 不同仍會通過）→ 契約改為**整段 round-trip byte-identical**：`raw_gzip == canonical_gzip_bytes(gunzip_bytes(raw_gzip))`；**✅ 實測既有 bundle 三個 payload 全部成立**，可直接當 fixture；⑥**runner 無 pin 時的行為未定案**（v6 只寫「有 ID 時禁止 build」，兩種實作都符合文字）→ 定案「**無 ID 時 runner 自己呼叫 `pin-replay-image.sh` 取得 ID，再一律以 ID 執行**」，保留既有使用方式同時維持單一 build 實作，並補測試證明**無 pin 路徑也是 `docker run <image-id>` 而非 tag**；⑦**新增「改版守則」**（零）——v4 弄丟 v3 的 validator 來源綁定、v6 弄丟 v4 的型別契約，**連續兩輪栽在同一種錯**：改版時⛔ 不得刪除任何既有契約條款，精簡只能作用在敘述文字上 |
| v8 | review 抓到 3 高 3 中，全部反映——⚠️ **本版依「零、改版守則」以增補方式修訂，⛔ 未整段重寫**：①**`.json.gz` 沒有合法 loader 落點**——來源鏈寫 `load_artifact`、SHA 章節卻允許 `.json.gz`，而範圍又明訂⛔ 不改 `load_artifact()`（它直接把 raw bytes 解碼成 UTF-8 JSON，`artifacts.py:703`，**讀不了 gzip**），實作者無從判斷是舊函式還是新 wrapper → 新增 **`load_canonical_evidence_artifact(path, kind)`**（`.json` 走既有 loader ＋ 驗 canonical；`.json.gz` 驗 round-trip byte-identical ＋ 解壓驗 canonical ＋ 檢查 schema／kind），**統一回傳解壓後 bytes 的 SHA**，crossday 與 finalizer 只用它，⛔ 既有 `load_artifact()` 完全不動；②**整包發布缺 rename 後的 durability 分流**——⛔ v7 的「finalizer 失敗一律回 1」在「rename 已成功、parent fsync 失敗」時是錯的，那時正式 root **已存在且有效**；既有 bundle 發布早就處理過這個分界（`bundle.py:362` 的 `DurabilityUnconfirmed`，訊息明寫「⛔ 不刪除、不重來」）→ 補**三階段分流**（rename 前失敗 → 清 staging ＋ exit 1；rename 本身 `FileExistsError` → ⛔ 不覆蓋；**rename 後 parent fsync 失敗 → 正式 root 保留 ＋ `EXIT_DURABILITY_UNCONFIRMED`(3)**），明訂**復原路徑是「只重新驗證並重新 fsync 已發布 root」⛔ 不重產證據**，並沿用 **`probe_no_clobber()`** 先實測目錄 rename 語意；orchestrator 補第 6 條：**durability code 優先於原始 0／5**（⛔ 不得被 `MATCH` 的 0 蓋掉）；③**probe 的 replay-error 守門沒有共用落點**——它是 `evaluation.py` 私有的 `_assert_no_replay_errors()`（`:2954`），而 package ⛔ 不得反向 import，另抄一份又是雙真相源（`NO_ZONE_SCORES_ERROR` 已犯過一次）→ **移到 `replay_bundle/artifacts.py` 改成公開的 `validate_replay_errors()`**，兩邊共用，⛔ 行為一字不改；全圖驗證改為**呼叫完整的 probe schema validator**，⛔ 不只比對 completion 的兩個 SHA；④**封閉欄位集合要明寫** → crossday、三份 probe、evidence manifest 各自 `_FIELDS`，**缺欄／多欄一律拒絕**並補 unknown／missing 測試；evidence manifest 補完整型別（`schema_version == 1`、`expected_image_id` 為 `sha256:` ＋ 64 hex、`stored_bytes` 嚴格正整數、兩個 SHA 裸 64 lowercase hex、**`files` key 精確等於 8 個 archive 相對路徑**——manifest 排除自身故是 8 不是 9）；⑤**正式模式的判定未明確** → 定死「帶 `--i074-preflight` 或 `--i074-capacity-probe` 即為正式流程，**缺 `REPLAY_IMAGE_ID` 立即拒絕**、⛔ 不自動 pin；crossday 與 finalizer 一律要求 ID；其餘既有 Stage 1／2 才允許自動 pin」，且 **probe／D／D+1 的 image ID 相同要在執行時就驗**，⛔ 不能留到 finalizer 最後（那時三趟都跑完了）；⑥**comparator／finalizer 的 provenance 推導未落地** → 明訂沿用與 replay runner **相同的 worktree／tooling-patch 推導流程**取得 immutable `base_commit`／tooling patch SHA／`source_root`／runner SHA，且**在實際工作完成、lazy import 都發生後才建**，⛔ **不得直接複製 D 的 provenance**（載入的模組集合不同） |
| v9 | review 抓到 2 高 3 中，全部反映——⚠️ **本版同樣以增補方式修訂**，並補掉 v8 的**兩處遺漏**：①**跨日 image ID 沒有可執行的共同狀態**——probe／D／D+1 是**跨日、分開啟動**的三個行程，D+1 ⛔ 無從知道前兩趟用了哪個 ID，v8 的「執行時就驗」在沒有共同狀態時**根本不可執行**，實際只能靠操作者手動傳對值 → 新增 **run identity 檔**（**repo 外**、**原子建立**、**no-clobber**，記 `bundle_id` 與 `expected_image_id`），三趟正式 runner 都必須讀同一份並在 **`docker run` 之前**比對，crossday／finalizer 沿用；測試涵蓋「**D+1 傳入不同 ID → Docker 尚未啟動就拒絕**」；②**既有 evidence root 分不出「重複發布」與「durability recovery」**——兩者從檔案狀態看**完全相同**，v8 卻同時規定前者 exit 1、後者回 0 → 定案 **`--recover-durability` 明示模式**（一般模式遇 root 已存在**仍回 1**；recovery **要求 root 必須已存在**，⛔ 不讀 operational inputs、⛔ 不重新壓縮、⛔ 不覆寫，只驗證既有 9 檔並重新 fsync parent，通過回 0；root 不存在／缺檔／驗證不過回 1）；③**正文的全圖驗證只驗 probe SHA 指向**——⚠️ **v8 宣稱改了但實際只寫進修訂紀錄，正文沒改到**（替換未匹配而我只用全檔 grep 驗證，被修訂紀錄的命中掩蓋）→ 正文補「**呼叫三份完整的 probe schema validator**」與「**三份 provenance 完全相同、確實來自同一 runner invocation**」，否則「computation、completion SHA 與 manifest 被同步改掉」的無效 probe 會被歸檔；④**probe 的型別契約不完整** → 補共同欄位型別表（`type(x) is int`、`type(x) is bool and is True`、lowercase hex、含時區 RFC3339、`validate_provenance(role="stage1")`），並補「用 `True` 冒充 `schema_version`」「用 `1` 冒充 `completed`」「大寫 SHA」「`generated_at` 缺時區」四組拒絕測試；⑤**`.json` loader 暗示要讀兩次檔**——`load_artifact()` 只回傳 `(parsed, sha)`，**raw bytes ⛔ 不傳出**，wrapper 再讀一次可能已不是同一版本 → 改用 **SHA 對照** `raw_sha == sha256_hex(canonical_json_bytes(parsed))`（等價且只讀一次），⛔ 不為此改 `load_artifact()` 的公開 API；⑥**同時修掉 v8 漏改的測試矩陣**——它仍寫「finalizer 失敗一律回 1」，與同版新增的 **rc 3 優先**規則直接衝突 → 改為「一般失敗回 1、**回 3 時 orchestrator 回 3**」並新增 11-B 的 durability／recovery 案例 |
| v10 | review 抓到 3 高 2 中，全部反映（增補式）：①**run identity 仍不足以唯一實作**——v9 只寫「repo 外、原子建立、no-clobber、至少兩欄」，producer／路徑／schema／重入語意／取得方式全未定 → 新增**十三-B 完整 contract**：producer 是 `pin-replay-image.sh`、位置是 **`${XDG_DATA_HOME:-$HOME/.local/share}/stock_trading/i074_stage1/run_identity.json`**（⛔ 不得放 `/tmp`，要跨日存活）、封閉 schema ＋ `validate_run_identity()`、canonical ＋ `rename_noreplace()` 原子建立、**重入語意定為「內容逐欄相同 → no-op 回 0；任一欄不同 → 拒絕」**、消費者是五個角色、**四種拒絕條件全部在 `docker run` 之前**，並把 `replay_bundle/run_identity.py` 補進受影響檔案表；②**durability recovery 會讓 mismatch 的 exit 5 永遠消失**——crossday mismatch（5）→ parent fsync 失敗（**3 覆蓋 5**）→ recovery 固定回 **0**，⛔ 最後沒有任何一次成功結束的指令回傳過 5，機器端會把它讀成「整組成功匹配」 → 定案 **recovery 成功後從已驗證的 crossday artifact 讀 `outcome` 還原終端結果**（`MATCH` → 0、其餘 → 5），測試分別覆蓋原始 0 與原始 5；③**evidence manifest 的 metadata 沒有來源綁定**——v9 只定型別，而 **recovery 完全依賴既有 root**，索引值錯誤的 manifest 照樣通過 → 明訂三項都要**從同一次讀取的實際 archive 重算**（`artifact_sha256` ＝ 解壓內容、`stored_sha256` ＝ gzip raw bytes、`stored_bytes` ＝ raw bytes 長度），且 manifest 的 `bundle_id`／`expected_image_id` 要與 **run identity 比對**，並補三種竄改的拒絕測試；④**finalizer provenance 的建立時點與發布順序衝突**——manifest 內含 `finalizer_provenance`，而 v9 是「寫 manifest → 全圖驗證」，**全圖驗證期間才載入的模組不會進 provenance** → 定死**兩階段**（A：讀入 inputs 並完成八檔全部驗證，此時 import 都已發生；B：才建 provenance 與 manifest），且 **B 之後⛔ 不得再產生任何新的 project import**；⑤**probe computation 專屬欄位缺嚴格型別**——`200.0 == 200` 為真，只寫「== 200」會放行 float → 補 `type(row_count) is int`、`type(elapsed_seconds) is float`、`quota_by_symbol` 為 mapping、`rows` 為 list，測試補 `row_count=200.0`／`elapsed_seconds=True`／容器型別錯三組 |
| v11 | review 抓到 2 高 3 中，全部反映（增補式）：①**run identity 的重入語意自相矛盾**——pin script 是 build → inspect，而 identity 含**每次都會變的 `created_at`**，所以 v10 的「所有欄位完全相同才 no-op」**永遠不成立**；重新 build 也⛔ 不保證得到相同 image ID → 改為**依檔案存不存在分流**（已存在：驗 schema ＋ `bundle_id` 相符 ＋ **確認該 image 仍在本機** → **直接輸出既有 ID**，⛔ 不 build／不產新 `created_at`／不重寫；不存在：才 build 並發布；**image 已不在本機 → fail-closed，⛔ 不得重建後換 ID**），發布後補 **`fsync_file` ＋ parent `fsync_dir`**（rename 的原子性⛔ 不保證跨日 durability），並⛔ **移除 `I074_RUN_IDENTITY`**（「只供測試」無法強制，測試改覆寫 `XDG_DATA_HOME`）；②**manifest metadata 的「同一次讀取」拿不到**——v10 的 loader 只回 `(parsed, artifact_sha256)`，raw bytes ⛔ 沒傳出，finalizer 再讀一次就違反契約 → loader 改回 **`EvidenceLoad`**（`parsed`／`artifact_sha256`／`stored_sha256`／`stored_bytes`），crossday 只取前兩項、finalizer 與 recovery 用完整結果，既有 `load_artifact()` API ⛔ 不變；③**recovery 是否依賴外部 run identity 前後矛盾**——v10 一邊說 recovery ⛔ 不讀 operational inputs、一邊要求與 **repo 外**的 identity 比對，且外部那份一旦遺失，**已完整發布的 evidence 就再也 recovery 不了** → **把 canonical run identity 納入 archived evidence**（`identity/run_identity.json.gz`），證據自足；檔案數同步更新（**總數 9 → 10**、**manifest `files` key 8 → 9**、fsync 與 recovery 的驗證檔數一併改），repo 外那份降為**操作期協調檔**，正常 finalization 時另外確認兩份逐欄相同；④**測試矩陣仍寫 recovery「成功 → 0」**（⚠️ **第三次漏改測試矩陣**）→ 拆出 **11-C**：已歸檔 `MATCH` → 回 0、已歸檔任一 mismatch → **回 5**，⛔ 兩條都要測；並在**改版守則新增第二條**：改正文後必須同步檢查測試矩陣，且驗證要**限定在計畫書正文行號範圍內**（⛔ 全檔 grep 會被修訂紀錄的命中掩蓋——v8 就是這樣過關的）；⑤**「import 都已發生」缺可執行保證**——`build_provenance()` 的 dict literal 裡 **`project_module_hashes()` 排在 `runtime_settings()` 之前**求值（`provenance.py:133-147`），後者在 `config_module is None` 時 **lazy-import `config`**，而 ⚠️ **`config` 正是專案模組**（`PROJECT_MODULE_NAMES = ("config", "db")`）→ 明訂**階段 A 預先 import config 並以 `config_module=` 傳入 builder**、**階段 B 後重算 project-module mapping 並斷言與 manifest 的 provenance 完全相同**，讓「⛔ 不得新增 import」成為實際守門而非註解 |
| v12 | review 抓到 2 高 2 中，全部反映（增補式）：①**normal finalizer 沒有取得 repo 外 identity 的容器資料流**——finalizer 跑在容器裡、identity 在 host 的 repo 外，而⛔ **容器內不能自己依 `XDG_DATA_HOME` 重新推導**（容器的 `HOME` 與環境和 host 不同，推出來的是另一個路徑）→ 定案 normal finalization 由 **shell 推導並驗證 host 絕對路徑 → 唯讀掛入 → 注入受保護的 `--run-identity <容器內絕對路徑>`**，該參數**納入 `SCRIPT_INJECTED_ARGS`**（使用者傳入或重複傳入一律拒絕，⛔ 不靜默採用最後一個），**`--recover-durability` 則⛔ 不掛載也不注入**、只讀 archived 的那份；`stage1_argv.json` fixture 與 shell 測試同步；②**identity 的 fsync 失敗沒有 commit-point 狀態機**——v11 只說「要 fsync」，而 rename 成功但 parent fsync 失敗後 identity 已存在，下次走「既有 identity」分支**直接回傳 ID 而不重新 fsync**，durability 會**永遠**未確認 → 比照 evidence root 定三段（temp fsync 失敗 → 清 temp 回 1；rename 後 parent fsync 失敗 → **保留檔案回 3**；⚠️ **既有合法 identity 的 no-op 路徑也必須重新 `fsync_file` ＋ parent `fsync_dir`，成功才回 0**——那是上述狀態的**唯一修復路徑**），image 不在本機仍 fail-closed；③**階段 A 仍寫「八檔」**，與 v11 改成的 9 個 archive 不一致 → 改為「完成 **9 個 archive payload** 的全部驗證（含 `identity/run_identity.json.gz`），manifest 才在階段 B 建立」；④**測試矩陣未涵蓋 v11／v12 的新契約** → 新增 **11-D**（producer 三分支）、**11-E**（identity commit point 與 **no-op 重新 fsync 的修復路徑**）、**11-F**（archived 與 repo 外不一致要中止；**repo 外那份被刪除時 recovery 仍能完成**）、**11-G**（`EvidenceLoad` 四欄值 ＋ ⚠️ **以 spy 斷言每檔只讀一次**，⛔ 不靠註解宣告）、**11-H**（階段 B 後故意觸發新 import → 重算 mapping 與 manifest 不符 → 中止），並在第 12 項補 normal finalizer 的唯讀掛載與受保護參數、recovery ⛔ 不掛載 |
| v13 | review 抓到 1 高 1 中，全部反映（增補式）：①**`--run-identity` 放錯 CLI 所有權邊界**——v12 要求把它納入 `SCRIPT_INJECTED_ARGS`，但那是 **evaluation CLI** 的清單（`evaluation.py:3186`），而 finalizer 規劃在 **`replay_bundle/evidence.py`**、⛔ 根本不走那個 parser；shell 側的 `REPLAY_INJECTED_ARGS`（`replay-args.sh:16`）又會**連唯一前綴縮寫一起擋** → ⚠️ **實測證實**：把 `--run-identity` 加進去之後，**既有合法的 `--run-id` 立刻被拒絕**（訊息還誤稱它「會被展開成 `--run-identity`」），⛔ 那會直接弄壞 Stage 1／2 的既有參數 → 定案 **finalizer 自建 ownership checker**（⛔ 不共用前綴清單）、parser 自設 **`allow_abbrev=False`** ＋ 自查重複、**normal 要求／recovery 禁止** identity path、**finalizer 專屬 argv fixture**（⛔ 不塞進 `stage1_argv.json`），並補**「既有 `--run-id` 仍可使用」的回歸測試**；②**identity no-op 修復失敗的狀態未定義**——v12 只定義兩次 fsync 都成功回 0 → 補「no-op 路徑的 `fsync_file` 或 `fsync_dir` **任一失敗 → 保留 identity、⛔ 不 build 或重寫、回 3**」，並明訂⚠️ **非零結果時 stdout ⛔ 不得輸出 image ID**（否則下游會拿著一個 durability 未確認的 ID 繼續跑）；測試新增 **11-E-2**（兩種 fsync 失敗各一次、stdout 無 ID、重試成功且 **`created_at` 不變**） |
| v14 | review 抓到 1 中 1 低，全部反映（增補式）：①**finalizer 專屬 fixture 沒進檔案與測試閉環**——v13 的正文要求新增它，但受影響檔案表仍只列 `stage1_argv.json`、⛔ 沒指定路徑，測試矩陣也只驗行為、⛔ 沒要求比對 argv → 新增並列入 **`python/scripts/fixtures/finalizer_argv.json`**（⚠️ **分別保存 normal 與 recovery 兩組**），明訂**兩端共用同一份**：shell 測試斷言**官方腳本實際產生的 argv 與 fixture 逐 token 相同**、`test_evidence.py` 用**同一份**餵 parser（normal 恰好一個 identity path、recovery ⛔ 不含），**Docker 唯讀 mount 另由 shell 測試斷言**；⚠️ ⛔ 只驗行為而不比對 argv 等於沒釘住兩端契約；②**低：commit-point 表格被段落截斷**——v13 把「非零結果 stdout ⛔ 不得輸出 ID」那段插在**表格中間**，於是「image 已不在本機」那一列落到表外、Markdown ⛔ 不會算進狀態表 → 把該列移回表內、說明移到整張表之後。⚠️ 這與 **I-100 Stage 0 計畫書 v13 是同一種錯**（表格被段落截斷），已在正文註記：**增補式修訂要檢查插入點是否落在表格內部** |
| v15 | review 抓到 1 高，已反映（增補式）：**finalizer／recovery 的完整 CLI contract 未定義，且 recovery 的 image 守門沒有可執行路徑**——十三要求一律以指定 image ID 執行、十三-B 要求 identity 在 `docker run` **之前**比對，但 recovery **只讀 evidence 內的 archived identity**（一個 `.json.gz`），而「**Docker 啟動前讀 gzip 內的 identity**」⛔ 沒有定義路徑；照 v14 的文字只剩三條都不可接受的選擇（先用未驗證的 image 啟動再在容器內檢查／完全不比對 recovery 的 image／各自發明 host 端入口造成**雙真相源**）→ 新增**十三-C**：①**精確 CLI matrix**（normal：evidence root ＋ **8 份 operational artifact** ＋ 腳本注入的 `--run-identity`，⛔ 禁 `--recover-durability`；recovery：只收 evidence root ＋ `--recover-durability`，⛔ 禁全部 operational input 與 `--run-identity`；provenance 注入參數依十三-B 推導），並明訂 **fixture 依這張 matrix 建立**⛔ 不是由實作者自選 argv 再凍結（後者證明不了符合契約）；②**recovery 的 host 端守門**：`docker run` 之前以 **host 端 dependency-light 入口**讀 `identity/run_identity.json.gz`，走 canonical gzip round-trip → 解壓 → canonical JSON → **`validate_run_identity()`（同一份 validator，⛔ 不在 shell 另寫 schema 檢查）**，再把 `expected_image_id` 與 `REPLAY_IMAGE_ID` 比對並 `docker image inspect` 確認該 image 存在，**任一不符即 Docker 尚未啟動就中止**；⚠️ **實測（2026-09-14）此路徑可行**——host 無 pandas 且 `sr_scoring/__init__.py` 會 import 它，但用**最小 package context**（`types.ModuleType` ＋ `__path__`）繞過後，`canonical`／`publish`／`artifacts` **含相對 import 都能在 host 載入**；⛔ 因此加一條硬性約束：**`run_identity.py` 只能 import 標準庫與同 package 的 dependency-light 模組**，⛔ 不得直接或間接碰 pandas／sklearn／lightgbm，否則這道守門會失效；③測試補**三類拒絕**（跨模式參數、缺必填、recovery image ID 與 archived identity 不符）**都要發生在 `docker run` 與任何寫檔之前** |
| v16 | review 抓到 1 高 2 中，全部反映（增補式）：①**`--run-identity` 的路徑語意自相矛盾**——十三-B 寫「注入**容器內**絕對路徑」、十三-C 的 matrix 卻寫「**host** 絕對路徑」，⚠️ 除非明訂掛載方式，Python finalizer 會收到**容器內不存在**的 host path → **定案沿用既有慣例 same-path bind mount**（`run-replay-offline.sh:113` 對 bundle 與 output dir 就是 `-v "$ABS":"$ABS":ro`，註解寫明「掛在與 host 相同的絕對路徑，參數不用改寫，也就不會改寫錯」），新增 **A-0** 逐層定死 shell／docker／argv／fixture／測試，於是⛔ **不存在兩種路徑之分**；②**host 端 validator 沒有正式可測的落點**——v15 只記錄了一次人工實測的 `types.ModuleType` 技法，⛔ 沒有檔案、沒有 contract，最後會變成 shell heredoc 裡的特殊 bootstrap → 指定 **`python/scripts/validate-i074-run-identity.py`** 並列入受影響檔案表，明訂 **stdout 只印 image ID 一行／其餘走 stderr／非 0 時 stdout 無輸出**，**package bootstrap 封裝在該檔**，並把該技法列為**受測的正式相容層**（測試涵蓋「host 無 pandas 仍能驗證」與「malformed gzip／schema 非零退出」）；③**recovery 注入五個 provenance 參數卻沒有消費者**——recovery ⛔ 不建新 provenance → 裁決其用途是**確認 recovery 跑的是同一份程式碼**：以本次執行身分**逐欄比對** archived `finalizer_provenance` 的 **`image_digest`／`base_commit`／`tooling_patch_sha256`／`runner_sha256`／`source_root`** 五欄，⛔ **不比** `argv`（模式天生不同）、`project_modules_sha256`（recovery 不讀 operational inputs，載入集合本來就較少）與 `python_version`／`pip_freeze_sha256`／`runtime_settings`（由 image 決定，`image_digest` 已涵蓋）；⚠️ **任一不符 → 在 fsync 之前中止**，並補對應測試 |
| v17 | review 抓到 1 中 1 低，全部反映（增補式）：①**v16 新增的驗證要求沒有同步進測試矩陣**——「host 無 pandas 仍可執行／malformed gzip 與錯 schema 必須失敗」與「recovery 的執行身分欄位任一不符要在 fsync 前中止」都只寫在正文，第十七節第 12 項仍只有跨模式、缺必填、image ID 不符三類；⚠️ **這正好違反計畫書自己的守則二** → 新增 **12-B**（以**實際的 `validate-i074-run-identity.py`** 驗 valid／malformed gzip／錯 schema、stdout 與 exit code contract，並用 **`python3 -S`** 或等價隔離證明⛔ 不依賴 site-packages／pandas）與 **12-C**（**6 個欄位逐一 tamper**，各自中止並以 spy 斷言 ⚠️ **正式 root 完全未變、`fsync_dir` 尚未被呼叫**）；⚠️ 同時在**守則二補一個可操作的做法**——每輪改完**逐一列出本輪的可驗證條款並對照測試矩陣**，⛔ 沒有對應項就是還沒改完（這條原則到 v16 為止已被違反**五次**，光有原則顯然不夠）；②**低：`runtime_settings`「由 image 決定」的理由不精確**——`config.py` 的五個值全是 **`os.getenv(...) or config.yaml`**（`:14`／`:40`／`:54`／`:57`／`:60`），環境變數可覆寫、`TRADING_CONFIG` 甚至能換掉整個 config 檔，⛔ 不是單由 image filesystem 決定 → 採「**recovery 直接逐欄比較 `runtime_settings`**」（normal 與 recovery 本來就該在相同封閉環境跑，最清楚），比對欄位由 5 個增為 **6 個**；`python_version`／`pip_freeze_sha256` 維持不比——那兩個**確實**由 image 內的 Python 與套件決定 |
| v18 | review 抓到 1 中 2 低，全部反映（增補式）：①**run identity 的消費來源自相矛盾**——十三-B 寫「五個消費者**全部讀同一份**（repo 外）」，但十三-C 與 recovery 正文明訂 **recovery 只讀 archived copy** → 改成**依模式分流**：**probe／D／D+1／comparator／normal finalizer** 讀 repo 外的協調檔，⛔ **`--recover-durability` 只讀 `identity/run_identity.json.gz`**、⛔ 不碰 repo 外那份（這也正是 v11 把 identity 納入證據包的目的——外部那份遺失仍能 recovery）；②**低：正文殘留「五個欄位」**（十三-C 的 C 段）與 A-2 的六欄契約及 12-C 不一致 → 改為**六個**並明列含 `runtime_settings`；③**低：12-C 沒釘死 tamper 的一側**——若竄改 **archived** 端的 `image_digest`，會**先被既有 manifest／image 一致性守門攔下**，那條測試就證明不了「本次身分 vs archived provenance」這個**新分支**真的生效（⚠️ 與前幾輪「測試沒刺激到它聲稱保護的分支」同類） → 明訂 **archived evidence 維持完全合法且一位元不改**，**改的是本次 recovery 這一側**（五個注入值逐一改動，`runtime_settings` 由當次 config／env 改動），並斷言**確實在身分比較分支中止**且 `fsync_dir` 未呼叫、正式 root 未變 |
| v19 | review 抓到 1 中，已反映（增補式）：**recovery 的 pre-Docker `bundle_id` 比對沒有可執行來源**——十三-B 統一要求四種（缺檔／`bundle_id` 不符／`expected_image_id` 不符／schema 不合法）都在 Docker 前拒絕，但 recovery 的 host validator CLI 只收 identity 路徑與 `--expect-image-id`，⛔ **沒有 expected bundle ID、也沒有可信的 host 端來源**，於是「`bundle_id` 與誰不符」根本無從判斷；⚠️ 而那層關係實際上是容器內**全圖驗證**的「所有檔案同一個 `bundle_id`」在做 → **依模式拆開**：**一般消費者**（probe／D／D+1／comparator／normal finalizer，手上有 bundle 路徑可當第二來源）維持**四種都在 Docker 前拒絕**；**recovery** 在 Docker 前只驗**缺檔／schema／`expected_image_id != REPLAY_IMAGE_ID`／image 存在**，**`bundle_id` 留到容器內、fsync 之前**拒絕；⛔ **不得為此在 host 端補一個「可信 bundle_id 來源」**——那等於把 operational input 帶回 recovery，違反它「只依賴既有 root」的前提；十三-B 的拒絕描述、十三-C-B 的守門範圍與測試矩陣 12-B 同步調整，⚠️ **⛔ 不讓測試承諾一個 host validator 做不到的檢查** |
| v20 | review 抓到 1 中，已反映（增補式）：**v19 把「Docker 前驗四種」從 recovery 移走，卻套到了全部「一般消費者」——但只有 runner 手上有 bundle path**：`compare-replay-crossday.sh` 只收 `--d`／`--d1`／`--output-dir`，normal finalizer 只收 evidence root ＋ 8 份 operational artifact ＋ identity ＋ provenance 參數，**兩者都⛔ 沒有 bundle path**，照 v19 的文字⛔ 無法唯一實作 → **依角色拆成四類**，共同點是**前四項（缺檔／schema／`expected_image_id`／image 存在）一律在 Docker 前**，差別只在 **`bundle_id` 何時才有第二來源**：**probe／D／D+1** 在 **Docker 前**（有 `--bundle`）、**comparator** 在容器內**兩份 after artifact 讀進來之後、crossday 發布之前**、**normal finalizer** 在容器內**階段 A、建立 staging 或發布之前**、**recovery** 在容器內**全圖驗證、fsync 之前**；⛔ **不得為了統一而在 comparator／finalizer／recovery 的 host 端補一個 bundle path**（那等於把 operational input 帶進不該有它的角色）；十三-B 的時機表、shell 測試描述與測試矩陣 12-B 同步改成四類時點，⛔ 不得把 runner 才有的 bundle path 套到其他角色 |
| v21 | review 抓到 3 中，全部反映（增補式）：①**repo 外的 plain JSON identity 沒有正式 host 驗證入口**——v20 要求四類角色都在 Docker 前驗，但唯一的 host validator 只定義接收 archived `.json.gz`、且明確是 recovery 入口；probe／D／D+1／comparator／normal finalizer 讀的是 repo 外的 plain `run_identity.json`，⛔ 只能各自發明解析方式（**雙真相源**）→ **同一支 validator 同時支援 `.json` 與 `.json.gz`**（前者驗 raw bytes == canonical bytes、後者驗 round-trip byte-identical 後解壓再驗 canonical），⚠️ **兩條路共用同一個 `validate_run_identity()`**，12-B 覆蓋兩種格式（含非 canonical 編碼／非 canonical gzip 的拒絕）；②**comparator 在容器內沒有獨立的 `bundle_id` 第二來源**——`compare-replay-crossday.sh` 只有 `--d`／`--d1`／`--output-dir`，⛔ 沒把 identity 掛入或注入，兩份 after 只能互比，**若兩份都帶同一個錯誤 `bundle_id` 就沒有任何獨立來源能發現** → **comparator 比照 normal finalizer**：same-path 唯讀掛載 ＋ 受保護的 `--run-identity`（自己的 ownership checker、`allow_abbrev=False`、拒絕重複），新增 **`comparator_argv.json`** 並補 argv／mount／ownership 測試（⚠️ ⛔ 不採「只由 finalizer 最終攔截」——crossday artifact 會進證據包，`bundle_id` 錯了該當場擋）；③**normal finalizer 的「建立 staging 前」與既定發布順序衝突**——既定流程是**先建完整 sibling staging、才做階段 A 全圖驗證**，且既有測試只保證失敗後 staging 被清除、⛔ 沒保證從未建立 → 改寫為「**階段 A、manifest／fsync／rename 之前**拒絕；失敗時清除 staging、正式 root 不存在」，⛔ 不為此重排兩階段流程 |
| v22 | review 抓到 2 中，全部反映（增補式）：①**runner 的 pre-Docker `bundle_id` 比對無法透過唯一 host validator 完成**——v21 的 CLI 只收 identity path 與 `--expect-image-id`，成功也只輸出 image ID，於是 runner 要做這項比對**只能在 shell 再解析一次 identity**，⛔ 那是**雙真相源**且兩次讀取之間有 **TOCTOU** → validator 新增**可選的 `--expect-bundle-id`**：**probe／D／D+1 傳入**（值取自實際 bundle）、**comparator／finalizer／recovery ⛔ 不傳**（依既定容器內時點比對），**不符即非零退出且 stdout ⛔ 無輸出**〔測試：**12-B**，含 match／mismatch 各一組，以及「**不傳時⛔ 不得因此失敗**」〕；②**v21 新增的 comparator 契約沒進測試矩陣**——正文要求 comparator fixture 與 argv／mount／ownership 驗證、受影響檔案表也加了 `comparator_argv.json`，但第 12 項仍只列 normal finalizer／recovery，⚠️ **第六次違反守則二** → 新增 **12-A**：argv **逐 token 等於 `comparator_argv.json`**、**same-path `:ro` mount**、使用者注入／**縮寫**／**重複** `--run-identity` 三種都拒絕，且**都發生在 `docker run` 與任何 artifact 寫入之前**〔測試：**12-A**〕；⚠️ 同時在**守則二再加一道機制**：**修訂紀錄裡每條新增契約都要標註對應的測試項編號**——⛔ 連 checklist 都擋不住（v21 又漏一次、累計六次），標編號的話在寫修訂紀錄當下就會卡住 |
| v23 | review 抓到 3 中，全部反映（增補式）：①**`--expect-bundle-id` 的來源未唯一化**——v22 只寫「值取自實際 bundle」，而**取目錄 basename／直接讀 `manifest.bundle_id`／經正式 loader 驗證**是**三種強度不同**的做法，前兩種擋不住偽造 → **改為 `--bundle <目錄路徑>`，由 validator 自己呼叫既有 `load_bundle()`**（`bundle.py:401`，含三方相等與完整 hash 驗證）取得 ID 再比對；⚠️ **這比傳值更強**：shell ⛔ 完全不解析、來源只有一條路，且 identity 與 bundle 在**同一行程內**讀完、⛔ 連 TOCTOU 都消除；✅ **實測**：`bundle.py` 只依標準庫與同 package 模組，**host（無 pandas）能跑完 `load_bundle()`**〔測試：**12-B**，含 match／mismatch、不傳不得失敗，**以及 v23 新增的 runner 接線整合測試**——⛔ 只直接測 validator 證明不了 runner 真的接上〕；②**12-A 沒測到 comparator 新增契約的核心分支**——identity 的目的是抓「**兩份 after 都帶同一個錯誤 `bundle_id`**」，但 12-A 只驗 argv／mount／所有權，identity 就算成功掛入、實作者漏掉實際比對仍會全綠（⚠️ 與前幾輪「測試沒刺激到它聲稱保護的分支」同類） → 補產品分支：**兩份 after 合法且同為 `bundle_id = B`、identity 記 `bundle_id = A`** → **一般失敗且⛔ 不得發布 crossday artifact**〔測試：**12-A**〕；③**正文與測試矩陣殘留舊 CLI 契約**（一處寫「只收 identity 路徑與 `--expect-image-id`」、一處寫「不承諾 `bundle_id` 檢查」，與新增的可選 bundle 參數衝突）→ 統一為「**validator 條件式支援 bundle ID：runner 必須傳 `--bundle`；comparator／normal finalizer／recovery ⛔ 不傳**（該角色沒有可信的 bundle 第二來源），recovery 的 invocation 只傳 image ID」 |
| v24 | review 抓到 1 中 1 低，全部反映（增補式）：①**`python3 -S` 沒覆蓋 v23 新增的 bundle 載入路徑**——v23 讓 host validator 多呼叫 `load_bundle()`，依賴閉包因此多了 **`bundle.py` → `calendar.py`**，但 bootstrap 說明仍只列 `canonical`／`publish`／`artifacts`、dependency-light 約束也只套在 `run_identity.py`，且 12-B 沒明訂 `python3 -S` 那組要帶 `--bundle`；⚠️ **於是只測不帶 bundle 的 recovery 路徑就會全綠，而 runner 的新路徑仍可能因 bundle／calendar 的 import 失敗** → **約束擴及 host validator 實際載入的完整模組閉包**（`canonical`／`publish`／`calendar`／`artifacts`／`bundle`／`run_identity`），12-B **至少一組用 `python3 -S` ＋ plain identity ＋ `--bundle <合法 bundle>` 並真的跑完 `load_bundle()`**，**再一組給損壞 bundle → 非零退出且 stdout 無輸出**；✅ **實測**：`python3 -S`（site-packages ⛔ 不在 `sys.path`）下全鏈跑通；⚠️ 並記下陷阱——**`replay_bundle/calendar.py` 與標準庫的 `calendar` 同名**，bootstrap 要用前綴註冊、⛔ 別用裸名〔測試：**12-B**〕；②**低：「連 TOCTOU 都消除」講過頭**——兩個路徑仍是**依序讀取**，且 **validator 結束到 `docker run` 之間仍有窗口** → 改成精確描述「消除的是**shell 與 validator 各解析一次 identity** 那個窗口」，並明講**那一段由容器內的正式 loader 在 replay 前重新完整驗證兜底**，本計畫的威脅模型⛔ 不處理並行的外部竄改 |
| **v25** | review 抓到 1 中，已反映（增補式）：**dependency-light 契約與現有 `calendar.py` 衝突**——v24 寫「閉包內模組**只能 import 標準庫與彼此**」，但 `calendar.py` 的線上抓取分支 `fetch_year_rows()`（`:110`）裡有 **lazy `import httpx`**（`:152`），那是第三方套件，⛔ 該敘述與現況直接衝突；⚠️ 而 `python3 -S` 的實測之所以仍通過，是因為 **loader 走的是 `validate_calendar_payload()`（`:272`）、⛔ 不經過那個分支**——也就是說，那個測試證明的是「**host validator 實際呼叫路徑不碰第三方**」，⛔ 不是「整個模組只依標準庫」 → **契約限縮為「host validator 實際執行的 import／call graph 必須 dependency-light」**，並明列 `calendar.py` 的線上抓取分支與 lazy `httpx` ⛔ **不在該路徑內**；⛔ **不採「把 HTTP 抓取移出 `calendar.py`」**——那會動到 I-100 已收斂的模組，也違反本計畫「⛔ 不改通用未啟用路徑」的範圍宣告，而 lazy import 本來就是為這種情形存在的；⚠️ 同時把實測的**證明範圍**寫精確：`python3 -S` 下那條路徑若真的碰到 `httpx` 就會 ImportError，**測試本身就是這條契約的守門**〔測試：**12-B**，沿用既有的 `python3 -S` ＋ `--bundle` 那組〕 |

#### 關閉條件（2026-09-01 改為單一決策樹）

結果只會落在三個分支之一。**分支 A 在 Stage 1 就判得出來**——零候選代表沒有東西可比，
**不必再跑 Stage 2**（那趟 before 全掃約 3.7 小時，省下來是實質的）。B 與 C 才需要 Stage 2：

| # | 結果 | 處置 |
|---|---|---|
| **A** | **精確候選數 ＝ 0** | 記錄實際掃描的標的、日期範圍、載入根數、eligible rows、模型 bundle 與設定，**轉為已知限制**並依下方措辭歸檔。本筆關閉 |
| **B** | **候選數 > 0，且 before/after 如預期翻轉** | 記錄**全候選**的逐列證據與下游影響（含 artifact 的 SHA-256），**驗證完成**。本筆關閉 |
| **C** | **候選數 > 0，但沒有翻轉，或下游欄位不符合下表的逐項預期**；⚠️ **或 before／after 的候選集合不一致**（v8 補——那是 tooling 不對稱，見主文「兩邊算出來的 candidate 集合必須完全相同」） | ⛔ **這是新的實作／驗證矛盾，不是零命中。本筆不得關閉**，另立新 issue 調查（編號依本檔使用說明的下一個可用值，**不要預先佔號**）。⚠️ **集合不一致這一種的證據是 `candidate_mismatch.json`**，⛔ 此時沒有 comparison artifact，判讀依差集 |

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
    不得入選、**`setup_rr_qualified = true`** 不得入選（⚠️ 2026-09-11 修正——原文寫
    `rr_gate.qualified` 是錯的，對外那顆已被 execution gate 覆寫，見上方 candidate 定義）。
    **這三支就是本輪 review 抓到的三種偽陽性。**
  * **B/C 的四格判定**：`before ∈ {TESTING, CONFIRMED}` × `market_action ∈ {AVOID, 非 AVOID}`
    各一支；三個「持倉不變」的格子都必須斷言**仍屬分支 B**。
  * **before/after 的 candidate 集合等價**：同一組 fixture 下，after 版用
    `lifecycle_phase == CONTINUATION and not setup_rr_qualified`、before 版用展開式，
    兩邊選出的列必須完全相同。⛔ **before row 不得套用 after 的
    `lifecycle_phase == CONTINUATION` 等價式**——before 版沒有 `lifecycle_engine.py`，
    那個欄位在它那邊本來就不會是 `CONTINUATION`。
  * **Stage 2 的 warm-up 連續性**：對同一個候選列，連續 warm-up 與孤立計算會得到不同的
    `event_state_summary`。
  * `lifecycle_engine.py` 既有的優先序與 RR 獨立性測試必須全數續存且不修改斷言。
* **風險**：定向挑樣造成代表性誤讀（以「不推論盛行率」的措辭處理）；I-100 未落實造成資料
  漂移（⚠️ **工具與凍結 bundle 已完成，但跨日逐列驗收仍是 blocker**——見
  [I-100](#i-100decision-replay-沒有-as-of-上界cohort-隔天就重現不了) 的關閉條件 2 與 3）；為求命中而鬆動 predicate
  ——⚠️ **護欄不是「只加回傳欄位」**（2026-09-11 修正）：正確的表述是
  **`lifecycle_engine.py` 與 `decision_engine.py` 的判定條件與優先序一行都不得改，
  而 `evaluation.py` 只允許 Stage 0 計畫書明列的來源切換與驗證守門**。
  受影響檔案的完整清單以 **Stage 0 計畫書一-B** 為準，⛔ 本段不另列一套。
* **歸檔**：驗收結論與仍需保留的限制寫進 [`sr-zone-scoring.md`](./sr-zone-scoring.md)，
  本筆經 review 確認後再移除。

---

### I-100：decision replay 沒有 as-of 上界，cohort 隔天就重現不了

| 欄位 | 內容 |
|---|---|
| 狀態 | **已實作／待正式 bundle 與跨日驗收**（2026-09-10 依 v23 計畫書完成程式與自動化測試，見下方「實作結果」；2026-09-01 由已知限制升級——[I-074](#i-074lifecycle-engine-的-rr-解耦decision-replay-已跑但一次都沒觸發到) 已把本筆列為硬性前置，見下方「必須做到的範圍」。**已造成一次實際後果**，見下）。⚠️ **本輪只交付程式與自動化測試**，正式 bundle 與跨日驗收另立一輪，兩者仍是關閉條件，⛔ 本筆在那之前不得移除 |
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

#### 計畫書 v23（2026-09-10，**已確認／實作中**）

⚠️ v22 於 2026-09-10 經使用者確認，並在確認實作範圍時裁決了 4 項（結構、Stage 1 的欄位守門、
腳本測試的落地方式、自動化／手動驗證界線），全部已反映成 v23；修訂摘要在最後一節。
⚠️ **本輪只做程式實作與自動化測試**：連 dev／live DB 產出正式 bundle 與**跨日驗收**
另立一輪，兩者仍是 I-100 的必要交付與關閉條件，⛔ 不因本輪完成而省略。
**量測、儲存、目標與不做範圍承前不變**：整包約 4.9 MB 進版控放
`python/baselines/<bundle_id>/`、I-074 收斂後不刪除；不動 runtime、不改演算法、
不建通用 artifact storage、不改既有預設行為。

##### 一、三種模式與判定規則

| Stage | 指令 | 讀 DB | 跑 replay | 產出（`--output-dir` 下固定檔名） |
|---|---|---|---|---|
| **0** | `--as-of <d> --symbols … --emit-bundle <dir> --model-path … --image-digest …（後兩者由官方腳本注入）[--report-max-rows 200] [--trading-calendar <f>]` | ✅ 唯一 | ⛔ 不跑 | bundle |
| **1** | `--bundle <dir> --output-dir <dir> --before-ref <ref>`（⚠️ `--base-commit`／`--tooling-patch-sha256`／`--image-digest`／`--source-root`／**`--runner-sha256`**（v23 review 新增） **由腳本內部注入，不是使用者參數**） | ❌ | ✅ 全候選 | `after_artifact.json` ＋ `cohort_manifest.json` |
| **2** | Stage 1 的參數 ＋ `--after-artifact <f> --cohort-manifest <f>` | ❌ | ✅ 全候選 | **兩種終止狀態之一**：集合一致 → `comparison_artifact.json`（⛔ 不截斷）＋ `report.json`；⚠️ 集合不一致 → `candidate_mismatch.json` ＋ 專屬結束碼（見 I-074 Stage 0 計畫書六-D） |

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

**`rr_decoupling_candidate` 的欄位守門（v23 裁決）**——⚠️ 這個欄位是
[I-074](#i-074lifecycle-engine-的-rr-解耦decision-replay-已跑但一次都沒觸發到) Stage 0 才會補上的
診斷欄位（責任層見該筆「責任層」表），**本筆只消費、⛔ 不自行產生也不用近似欄位反推**：

* **Stage 1**：每一列 replay row 都必須有 `rr_decoupling_candidate`，且型別必須是**嚴格 boolean**
  （`isinstance(x, bool)`，⛔ 不收 `0`／`1`／`"true"`／`None`）。缺欄位、`null` 或非 boolean
  → **在發布 `after_artifact.json`／`cohort_manifest.json` 之前中止**，訊息明講「I-074 Stage 0
  的診斷欄位尚未就位」；
* **Stage 2**：載入 after artifact 時**再驗一次**同一條 schema，只讀既有欄位；
* ⚠️ **欄位完整、但所有列都明確為 `false` 時，空 cohort 是合法結果**——⛔ 不得把
  「真正零命中」判成錯誤。這道守門禁止的是**因欄位缺失而靜默變成空 cohort**，兩者要分開；
* ⚠️ **本筆可以先交付工具與護欄，但正式 Stage 1 要等 I-074 Stage 0 補齊欄位後才跑得動。**

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
`replay_bundle/` 整個 package 與三支腳本檔（見「十、受影響檔案」），照 v12 的寫法**那些新檔不會進 hash**，
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
`--image-digest`／`--base-commit`／`--tooling-patch-sha256`／`--source-root`／
**`--runner-sha256`**（v23 review 新增——runner 是容器外的 shell 腳本，容器內的 Python
讀不到它，所以 hash 只能由腳本自己算好後注入）。
⛔ **拒絕的範圍含唯一前綴縮寫**（`--image-d` 會被 argparse 展開），見「十五、review 修正」#3。

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

**測試**：**五個**欄位（v23 review 補上 `--runner-sha256`）**各一條 spoof**
（使用者傳同名參數或**唯一前綴縮寫** → 腳本拒絕）
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

⚠️ **v23 更新**（結構裁決與腳本測試落地方式）：

* `python/db.py`——`as_of` 上界、optional `conn`、逐 driver 唯讀快照、readiness 查詢；
* `python/backtest/modular/sr_scoring/evaluation.py`——**只做流程協調**：三種模式判定、
  參數所有權、strict loader、呼叫下方 package；
* **新 package** `python/backtest/modular/sr_scoring/replay_bundle/`——
  `__init__.py`（**穩定公開 API**）／`canonical.py`／`calendar.py`／`provenance.py`／
  `publish.py`／`bundle.py`。⛔ **不得反向 import `evaluation.py`**（避免循環依賴）；
  ⚠️ 拆成 package 只是結構調整，**v22 的資料 contract 一律不變**；
* `scripts/run-evaluation.sh`（Stage 0 掛載、透傳、image digest 注入、**與 Stage 0 禁用參數的互斥**）；
* **新檔** `scripts/run-replay-offline.sh`；
* **新檔** `scripts/lib/replay-args.sh`——兩支腳本共用的 **argv builder ＋ 參數所有權驗證**；
  ⚠️ `run-evaluation.sh` **必須呼叫同一個 builder**，shell 測試才算驗到官方實際組法；
* **新檔** `scripts/test-replay-args.sh`——驗 spoof 與參數所有權；
* **新檔** argv fixture（版控）——**存的是參數 token 序列，⛔ 不是需要 `eval` 的 shell 字串**；
  digest、source root 等動態值一律用固定測試值或明確 placeholder；
* `python/scripts/test.sh`——**在啟動 pytest container 之前先跑 `scripts/test-replay-args.sh`**，
  ⚠️ 否則新測試會變成沒人固定執行的手動項目（掛載邊界只影響 container 內，host 端本來就讀得到 repo root）；
* `docs/development-workflow.md`、`docs/sr-zone-scoring.md`。

##### 十一、失敗行為

Stage 0 或 bundle 模式併用被禁參數（含明確傳入等於預設值）／`--after-artifact` 與
`--cohort-manifest` 只給其一／**`market_latest < expected_latest`**（資料未到齊）／任何 hash 不符／
未知 `schema_version`（**受管制檔案：`manifest.json`／`trading_calendar.json`／`cohort_manifest.json`／`after_artifact.json`／`comparison_artifact.json`／**`candidate_mismatch.json`**（2026-09-11 由 I-074 Stage 0 計畫書 v8 定義 schema 與 validator 後納入）**——⛔ 列出檔名而不是寫數量，避免再漂移）／**strict 的六個 fail-open 分支**（四-B 表）／**四道集合檢查任一不成立或鍵不唯一**／
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
| **provenance** | patch hash 來自實際 worktree diff（傳錯 patch 檔要抓得到）；**專案模組落在 `source_root` 外要中止，stdlib／site-packages 不受此限**；`--image-digest`／`--base-commit`／`--tooling-patch-sha256`／`--source-root`／**`--runner-sha256`** **各一條 spoof（腳本拒絕，含縮寫形式）＋ 各一條實際值不符（中止）**；**兩支腳本各一條重複參數測試**（CLI 中止，⛔ 不採用最後一個）；**`--before-ref`／`--bundle`／`--output-dir`／`--after-artifact`／`--cohort-manifest` 各一條重複測試**（腳本中止——腳本取第一個、argparse 取最後一個，重複會讓實際版本與報告宣稱的版本不同） |
| **結構性離線** | `--network none` 下跑完 Stage 1／2；驗無 DB 環境變數、唯讀掛載 |
| **跨日驗收** | ⛔ D 日產 bundle 並跑，**D+1 日載入同一份再跑**，輸入指紋與逐列結果相同 |

##### 十二-B、自動化與手動驗證的界線（v23 裁決）

| 範圍 | 方式 |
|---|---|
| PostgreSQL／MySQL／SQLite 的**交易語句與順序**斷言 | **自動化**（三 driver 各一條） |
| SQLite 的**實際**唯讀強制、`rollback`、`query_only` 復原、concurrent-writer 快照 | **自動化** |
| PostgreSQL 的**實機**一致性快照 | **手動**（`development-workflow.md` 的步驟） |
| MySQL／InnoDB 的**實機**一致性快照 | **手動**，⚠️ 另受 [I-054](#i-054mysql-的執行期支援仍未被驗證ddl-已驗crud-未驗) 限制 |

⛔ **文件與驗收報告一律不得宣稱 CI 已涵蓋 PostgreSQL／MySQL 的實機快照。**

**腳本層測試怎麼跑（v23 裁決）**：`scripts/test-replay-args.sh` 由
`python/scripts/test.sh` 在啟動 pytest container **之前**呼叫；shell 側驗 spoof 與參數所有權，
Python 側從**同一份版控 argv fixture** 加上各種衝突參數，驗 CLI 在
`check_connection()` 或 replay **之前**中止——兩側夾住同一份 fixture 才不會漂移。

##### 十三、完成後歸檔位置

**已歸檔（2026-09-10）**：

* [`sr-zone-scoring.md`](./sr-zone-scoring.md)「**Decision Replay 的可重現性：as-of、凍結
  bundle 與三階段**」——as-of 語意與 readiness 判準、bundle 與三份 artifact 的 schema 與
  canonical 規則、交易日曆、跨來源快照、strict、四道集合檢查、`replay_scope`／
  `report_max_rows` 分工、Stage 分工。同時修掉了「1830 行附近仍寫『需要補一個 as-of 上界』」
  的過期敘述；
* [`development-workflow.md`](./development-workflow.md)「**Decision Replay 的可重現執行
  （I-100）**」——三條指令、參數所有權、before／after 的 worktree 執行模型與 TOCTOU 順序、
  tooling patch hash、結構性離線、Stage 0 的整包原子發布契約與 commit point、
  **驗收報告要附什麼**、**PG／MySQL 實機快照的手動步驟**、腳本層測試。

##### 十四、實作結果（2026-09-10）

**本輪交付程式與自動化測試；正式 bundle 與跨日驗收另立一輪**（使用者 2026-09-10 裁決）。

新增／修改：

| 檔案 | 內容 |
|---|---|
| `python/db.py` | `fetch_candles(as_of=…)`、四個 helper 的 optional `conn`、`readonly_snapshot()`（逐 driver）、`fetch_market_latest_trading_date()`、`as_of_cutoff_epoch()` |
| `python/backtest/modular/sr_scoring/replay_bundle/` | 新 package：`canonical` / `calendar` / `provenance` / `publish` / `bundle` / `artifacts`，`__init__` 提供穩定公開 API |
| `.../evaluation.py` | `emit_replay_bundle()`（Stage 0）、`run_bundle_stage()`（Stage 1／2）、三種模式判定與封閉式參數所有權、兩個 context loader 的 `strict` |
| `scripts/lib/replay-args.sh` | 共用 argv builder ＋ 參數所有權 ＋ worktree／tooling patch |
| `scripts/run-evaluation.sh` | Stage 0 支援、image digest 注入、與 `WRITE_DB`／`MODE`／`OUTPUT` 互斥 |
| `scripts/run-replay-offline.sh` | 新檔，Stage 1／2 的結構性離線執行 |
| `scripts/test-replay-args.sh` ＋ `python/scripts/fixtures/stage0_argv.json` | 腳本層測試與版控 argv fixture |
| `scripts/smoke-replay-offline.sh` ＋ `python/scripts/make_smoke_bundle.py` | 端到端 smoke：真的用官方腳本跑完 Stage 1／2（`REPLAY_SMOKE=1` 才跑） |
| `python/scripts/test.sh` | pytest 之前先跑 `scripts/test-replay-args.sh`；`REPLAY_SMOKE=1` 時另跑 smoke |

測試：`python/scripts/test.sh` **951 passed, 1 skipped**（含新增的
`test_replay_bundle_{canonical,identity,publish,calendar,provenance,stage0,stages,cli}.py`
與 `python/tests/test_db_snapshot.py`），`scripts/test-replay-args.sh` 全數通過。

⚠️ **兩個尚未滿足的關閉條件**（見下方「關閉條件」第 2、3 項與十二的「跨日驗收」）：

1. **正式 bundle 尚未產出**——要連 dev／live DB 跑一次 Stage 0，把
   `python/baselines/<bundle_id>/` 納入版控；
2. **跨日驗收尚未執行**——⛔ 必須真的 D 日產、D+1 日載入同一份再跑，同日重跑不能替代。

⚠️ **Stage 1 目前跑不動，這是設計上的預期**：候選要靠 `rr_decoupling_candidate`，而那個
欄位是 [I-074](#i-074lifecycle-engine-的-rr-解耦decision-replay-已跑但一次都沒觸發到)
Stage 0 才會補上的。本筆已交付**工具與護欄**（缺欄位會在發布 artifact 之前中止並指名
I-074），欄位就位後才跑得動正式 Stage 1。

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
| v13 | ①TOCTOU 測試的預期與固定流程相反（先解析 OID 再用 OID 建 detached worktree，之後移動 branch **不可能**改變 worktree HEAD）→ 拆成兩條：解析後移動 branch **應通過**、人為改動 worktree HEAD **應中止**；②frozen 模式**產不出**規定的 calendar provenance（canonical payload 不含 `fetched_at`／`raw_row_count`）→ 改成 discriminated union：`online` 按年度記抓取時間與原始列數、`frozen` 記輸入檔 SHA-256 與載入時間並標明原始抓取資訊不可得，兩者都只用 normalized payload 決定 `bundle_id`，決定性測試的 volatile 欄位隨之分流；③`git diff --binary` 不含 untracked，會漏掉 patch 新增的檔案（v23 起是 `replay_bundle/` 整個 package 與新增的腳本檔）→ 定案用 `git apply --index` ＋ `git add -A -N`，取完 diff 斷言無 `??` 行，補「只新增檔案也要改變 hash」測試；④`covered_years` 未定義 canonical 排序與唯一性（`sort_keys` 不排陣列，`[2025,2026]` 與 `[2026,2025]` 會得到不同 hash）→ 要求嚴格升冪、不重複、且與 `days[].date` 的年度集合完全相等，補非 canonical frozen 檔的拒絕測試 |
| v14 | ①`content_hash8`／`bundle_id` 的組合演算法未定案（多檔如何排序、分隔、綁檔名都沒寫，不同實作者會算出不同 ID）→ 定成「逐檔完整 SHA-256 → `{檔名: hash}` mapping → 依檔名 UTF-8 位元組序 canonical JSON → 取前 8 hex」，並明列 `bundle_id` 的完整格式與正規表示式、說明為何不放 `run_id`／`pipeline_version`／`captured_at`、以及 8 hex 不負責防碰撞（完整性由 loader 逐檔驗）；②Python 端的 TWSE request 契約沒寫進計畫（Go 端已記錄 `queryYear` 會被忽略卻照回 200 ＋ 當年資料）→ 明訂 `date=<YYYY>0101&response=json`、⛔ 禁用 `queryYear`、逐列驗年、不用筆數當門檻，並補**斷言實際送出 query** 的測試；③frozen calendar 只有 JSON 範例與不變條件，缺精確欄位／型別／unknown-field 規則（多一個被忽略的欄位 → 語意不變卻換 bundle ID）→ 補 exact-field 表、bool 不得用 `isinstance(int)`、loader 重做 canonical 序列化並逐位元比對，補多餘／缺少／型別錯／非 canonical 排版四類測試；④標題與開頭仍寫 v12／v11 而修訂紀錄已是 v13 → 版本標示統一 |
| v15 | ①**Stage 0 沒有跨來源的一致性快照**（readiness／candles／chip／governance 各自 `engine.connect()`，同步工作中途 commit 就會封出一份「資料庫裡從未同時存在過」的組合）→ 定案「日曆 HTTP 先做完 → 開單一唯讀快照（PG `REPEATABLE READ` ＋ `READ ONLY`／InnoDB `REPEATABLE READ`／sqlite 明確 `BEGIN`）→ readiness 為交易內第一個查詢 → 三份 payload 同一 connection → rollback 結束」，helper 加 optional `conn` 且預設行為不變，補 concurrent writer 與語句斷言測試（PG 實機驗證列為手動步驟）；②`bundle_id` 未綁定目錄名與 manifest → 補「目錄 basename ＝ `manifest.bundle_id` ＝ 重新計算值」三方相等契約，補改名與身分欄位竄改測試；③整數欄位的守門缺 bool 反例（`isinstance(True, int)` 為真且 `True == 1`，連值檢查都會通過）→ 整數一律用 `type(x) is int`，補 `schema_version: true`／`covered_years: [true]` 拒絕測試 |
| v16 | ①三個 engine 的「唯讀快照」名實不符（只有 PG 真的唯讀，mysql 只有 `REPEATABLE READ`、sqlite 只有 `BEGIN`；測試也只列 PG 與 sqlite）→ 補逐 driver 的精確建立順序表（PG `BEGIN` → `SET TRANSACTION READ ONLY`；mysql `START TRANSACTION READ ONLY`；sqlite `PRAGMA query_only=1` → `BEGIN DEFERRED` ＋ **finally 復原**），統一「readiness 當交易內第一個查詢」為三 engine 的快照錨點，並補三 driver 的語句順序斷言、sqlite 的唯讀強制測試，明列 mysql 受 I-054 限制只有語句測試自動化；②快照內的查詢失敗仍會被既有 fail-open 吞掉（`evaluation.py:2467`／`:2498` 逐 symbol 轉 warning 後 `continue`，會發布一份少了 context 的 bundle）→ 兩個 loader 加 `strict` 參數、Stage 0 一律 `strict=True` 讓六個 fail-open 點原樣拋出，`rollback` 放 `finally`，失敗必須非零結束且正式目錄不存在，補四種注入失敗測試與「合法零筆仍應成功」對照組 |
| v17 | ①mysql 的快照仍靠**可被改掉的預設 isolation**（`START TRANSACTION READ ONLY` 只設存取模式；session／server 落在 `READ COMMITTED` 時每次 consistent read 都取新快照，同步期間的 commit 照樣混入且**不會報錯**，`db.py:20` 也沒固定 isolation）→ 定案先用 SQLAlchemy 的 `execution_options(isolation_level="REPEATABLE READ")` 明確設定再 `START TRANSACTION READ ONLY`，⛔ 不自己 `SET SESSION …` 手動復原（會跟著連線污染 pool）、⛔ 不改全域 engine，補「session 先設成 `READ COMMITTED`，Stage 0 必須主動切回」的測試；②strict 的分支數量與處理方式全文不一致（詳細段寫六個、他處仍寫「四處」「四種」；而 `dataset range missing` 兩條**沒有原始例外可 bare `raise`**）→ 列出六分支矩陣並標明其中兩條要**主動 `raise ValueError`**、其餘四條 bare `raise`，測試補到八條（六分支 ＋ readiness ＋ candles），全文數量統一改為「四-B 表列的六個分支」 |
| v18 | 失敗清理與既有 bundle 的保留契約**互斥**（v17 無條件要求「失敗後正式目錄不得存在」，但「重複產生」明定正式目錄可以原本就存在且**不得覆蓋**——照 v17 實作會刪掉原本有效的基準 bundle，也就是 I-074 要長期保留的證據）→ 改成依**執行前狀態**分流（原本不存在 → 仍不存在；原本已存在 → **逐位元不變且仍通過正式 loader**），明定清理只能刪本次建立的 temp、⛔ 任何路徑都不得碰既有正式目錄，失敗測試加兩組「預先放置有效同 ID bundle」情境，並把 **Stage 0 納入原子發布契約與總表**（原文只寫 Stage 1／2） |
| v19 | Stage 0 的「整包原子發布」沒真的定案（v18 只說同受契約約束，而契約寫的是**逐檔** `os.replace`；多檔 bundle 照這樣做會讓別的程序看到半成品正式目錄、失敗後正式目錄仍存在、且「先 `exists()` 再發布」有 TOCTOU）→ 新增七-B：staging 目錄放在同一 filesystem 且子目錄名正好是 `bundle_id`（滿足三方相等契約）、fsync 後先用正式 loader 驗 staging、以 **`os.mkdir` 原子 claim** 決定勝負、勝方一次 `os.rename` 整包發布、敗方改用正式 loader 驗既有那份（相同 no-op／不同中止）、`finally` 只刪本次 staging；明寫 POSIX rename 會蓋掉既有空目錄所以只對自己剛建的空目錄 rename、claim 與 rename 之間崩潰會留空目錄並在下次 fail-closed（⛔ 不自動刪正式路徑），四-B 分流表補「空／損壞」列，測試補四條 |
| v20 | ①`os.mkdir` claim **不是**整包原子發布（先把空的正式目錄公開再 rename 蓋掉：reader 看得到空目錄、輸家只能在空目錄上中止而不是 no-op、rename 前崩潰就留下空的正式目錄）→ 改用 Linux 的 **`renameat2(RENAME_NOREPLACE)`**（`ctypes` 呼叫 `syscall`），正式路徑一次完整出現、全程不建立空目錄，`EEXIST` 才驗既有那份；補檔案系統 probe 與 **`O_EXCL` 發布鎖 fallback**（明說只在所有 producer 遵守同一把鎖時成立、殘留鎖 fail-closed）、發布後 fsync `<baselines>`、以及 rename 呼叫前的 deterministic barrier 測試；②競爭輸家比「逐檔 hash」會把允許 volatile 的 `manifest.json`／`manifest.sha256` 算進去，**同一份 payload 重產會被判成不同而中止** → 改比 manifest 內那份「六份 payload 的完整 SHA-256 mapping」，⛔ 不比 manifest bytes，兩份都先通過正式 loader；完整 hash 順帶抓 8-hex `bundle_id` 碰撞 |
| v21 | ①`O_EXCL` fallback **沒定義正常競爭下怎麼取得鎖**（把「鎖已存在」一律當殘留鎖中止，兩個正常 producer 併發時第二個看到有效鎖也直接中止，達不到「一方 no-op」；且它的 no-clobber 只在合作者之間成立）→ **移除弱化 fallback**，probe 不通過即 fail-closed，訊息給出「產在支援的路徑再搬進版控」的補救；②rename 成功後 parent fsync 失敗**無處可歸**（回一般失敗違反「失敗後仍不存在」，刪掉又違反不碰正式路徑）→ 定義 **rename 成功＝commit point**，之後 fsync 失敗改回**專屬非零碼**「已發布且 loader-valid、durability 未確認」，⛔ 不刪正式路徑，重跑走 no-op ＋ 重新 fsync，四-B 與失敗行為段各補交叉說明；③raw syscall 的平台守門未閉合 → 優先用 **libc 的 `renameat2` symbol**，找不到才退回 `syscall()` 且**限定架構 allowlist**、未知架構 fail-closed，errno 用 `ctypes.get_errno()`，probe 明確驗「目的不存在→成功／已存在→`EEXIST` 且既有目的不變」並在所有路徑清掉 probe 檔 |
| v22 | ①**no-op 分支其實不會重新 fsync**（⑤ 直接「刪 staging、正常結束」，⑥ 只寫「發布成功後」——重跑一份 durability 未確認的 bundle 走的正是 no-op 這條）→ 改成**兩條成功路徑都要在回傳前 fsync `<baselines>`**，no-op 路徑的 fsync 失敗回同一個專屬狀態「既有 bundle 有效、durability 未確認」且⛔ 完全不修改正式目錄，commit point 分流表與測試各補一列；②「先產在支援路徑再搬進版控」**不是有效補救**（跨 filesystem 時 `renameat2` 回 `EXDEV`、`mv` 退化成 copy＋delete，正式路徑又暴露半成品且失去 no-clobber）→ 補救改為「讓最終的 `python/baselines/` 本身落在支援的 filesystem（搬移或重新掛載後重跑）」，另聲明「從別處匯入」需要另一套同樣原子的流程且本計畫不做，並明訂 staging 與正式路徑不同 fs（`EXDEV`）一律 fail-closed、⛔ 不得改用 copy；③probe 形狀不對 → 改成**目錄** rename 的 A／B 兩組（不存在的 destination → 成功；已存在 → `EEXIST` 且 source 與 destination 都不變），所有 probe 目錄在每條路徑都要清掉 |
| **v23** | 使用者確認 v22 後，裁決 4 項實作範圍問題：①`replay_bundle.py` 單檔裝不下 canonical／日曆／provenance／發布／loader 五塊 → 改成 `replay_bundle/` package（`__init__.py` 提供穩定公開 API、`evaluation.py` 只做流程協調、⛔ package 不反向 import `evaluation.py`），**資料 contract 一律不變**，受影響檔案清單與 v13 的 provenance 敘述同步更新；②`rr_decoupling_candidate` 是 I-074 Stage 0 才補的欄位，本筆只消費 → Stage 1 對「缺欄位／`null`／非嚴格 boolean」**在發布 after artifact 與 cohort manifest 之前**中止並指名 I-074 Stage 0，Stage 2 載入時再驗一次，⚠️ 但**欄位完整而全列為 `false` 的空 cohort 是合法結果**，⛔ 不得與欄位缺失混為一談；③腳本層測試不只兩個新檔 → 明列 `scripts/lib/replay-args.sh`（兩支腳本共用的 argv builder ＋ 所有權驗證）、`scripts/test-replay-args.sh`、版控 argv fixture（**token 序列，⛔ 非 `eval` 字串**；動態值用固定測試值或 placeholder），並由 `python/scripts/test.sh` 在 pytest 之前呼叫，避免變成沒人跑的手動項目；④自動化／手動界線寫成十二-B 表，⛔ 文件與驗收報告不得宣稱 CI 已涵蓋 PG／MySQL 的實機快照 |

##### 十五、review 修正（2026-09-10，v23 實作後第一輪 review）

review 抓到 10 項（4 高 5 中 1 低），全部已修並補上回歸測試。⚠️ **其中兩項是自動化測試
完全看不到的**——它們只在「照文件實際跑三條指令」時才會現形，所以修法一併補了
`REPLAY_DRY_RUN=1`（印出實際 `docker run` argv）與對應的 shell 斷言。

| # | 問題 | 修法 |
|---|---|---|
| 1（高） | **Stage 1 實際跑的是 before 版本**：離線腳本不分 stage 一律建 `--before-ref` 的 worktree 並掛載它。舊版根本沒有 bundle CLI；就算跑得動，也會把 before 結果標成 after artifact | 依 stage 選 source ref（Stage 1＝`AFTER_REF`，預設 `HEAD`；Stage 2＝`--before-ref`），各自推導 `base_commit` 與 `tooling_patch_sha256` |
| 2（高） | **離線容器看不到 bundle，Stage 2 也看不到 Stage 1 的 artifact**：只掛了 before worktree 的 `python/` 與 output-dir。bundle 是之後才進版控的，舊 worktree 不會有它 | bundle 與兩份 artifact 各自**唯讀掛載在與 host 相同的絕對路徑**上（參數不需改寫，也就不會改寫錯） |
| 3（高） | **argparse 縮寫繞過 provenance 守門**：shell 只擋完整名稱與 `--name=value`，而 `--image-d` 會被 argparse 展開；官方注入值排在使用者參數之前，argparse 取最後一個 → 使用者的值贏 | Python 端 `allow_abbrev=False`；shell 端改成**前綴比對**（任何 `--` 開頭、長度 ≥3 且是受保護名稱前綴者一律拒絕）。兩層都要有 |
| 4（高） | **provenance 與計畫書不符**：`runner_sha256` 永遠是 `null`（Stage 0 沒傳、Stage 1／2 寫死 `None`）；且 provenance 建在 replay **之前**，lazy import 的模組沒進 `project_modules_sha256` | 新增腳本注入的 `--runner-sha256`（腳本本身 ＋ `replay-args.sh` 的指紋，⛔ 缺它即中止）；provenance 改在 replay／擷取**之後**才建 |
| 5（中） | **loader 沒驗 `trading_calendar.json` 的 schema**，也沒驗 `replay_config` 與 manifest 的一致性、`content_hash8`／`symbols_hash8` 是否等於重算值。測試 fixture 只放一天卻能通過正式 loader | loader 補上日曆完整 schema ＋ canonical bytes 比對、replay_config 封閉欄位 ＋ 四個 mirrored 欄位一致性、兩個 hash 欄位的重算比對；fixture 改用**真的建出來的完整年度日曆** |
| 6（中） | **Stage 2 沒在 replay 前完整驗 artifact**：只驗 `schema_version` 與 `kind`，其餘要等 before replay 跑完（約 3.7 小時）才發現 | 新增 `validate_after_artifact` / `validate_cohort_manifest`（封閉欄位、型別、唯一性、候選欄位守門），連同 ③ 全部移到 replay 之前；①② 依賴 replay 輸出，維持在後 |
| 7（中） | **SQLite 狀態復原失敗被靜默吞掉**：`query_only` 復原失敗的連線仍回到 pool，後續**正常寫入會突然變成唯讀** | 收尾改成逐項記錄失敗；有失敗即 `invalidate()` 丟棄該連線並 raise，⚠️ 但已有例外在傳時只記 log ⛔ 不蓋掉原始成因 |
| 8（中） | **MySQL 手動驗證跑不起來**：Stage 0 腳本把 `DATABASE_DRIVER` 寫死成 postgres；且文件沒有可控制「candles 取完、chip 未取」時點的 barrier | `DB_DRIVER` 可覆寫；新增 `SR_REPLAY_SNAPSHOT_PAUSE_SECONDS`（預設 0＝不生效，只給手動並發驗證用），文件補進手動步驟 |
| 9（中） | **計畫要求的測試沒落地**：`fetch_candles(as_of)` 只有 sqlite 有邊界測試；「結構性離線」只 grep 腳本內容，所以抓不到 #1／#2 | 三個 engine 各驗實際送出的 SQL 與 `as_of_cutoff`（⚠️ PG／MySQL 仍**不能**在容器內真的跑，這一點不改）；離線腳本改用 `REPLAY_DRY_RUN=1` 斷言實際 argv 的 stage↔版本、bundle／artifact 掛載、`--network none`、無 DB 環境變數 |
| 10（低） | TWSE 同日期**同分類**的重複列被靜默折疊 | 重複日期一律中止，分類相同也不放行 |

⚠️ **#9 帶出的教訓值得留著**：#1 與 #2 都是「照文件實際跑」才會現形的錯誤，而原本的測試
只 grep 腳本**內容**。**驗腳本要驗它實際組出來的指令**，不是驗它的原始碼裡有沒有某個字串。

##### 十六、review 修正（2026-09-10，第二輪 review）

第二輪抓到 4 項（1 高 2 中 1 低），全部已修。

| # | 問題 | 修法 |
|---|---|---|
| 1（高） | **重複 `--before-ref` 會讓實際執行的版本與報告宣稱的版本不同**：腳本的 `replay_args_value_of` 取**第一個**值（拿它 checkout／掛載），argparse 取**最後一個**值（拿它寫進 comparison artifact）。實測 `--before-ref A --before-ref B` → 實際跑 A、報告寫 B，正好破壞本筆要保護的版本身分 | 新增 `replay_args_reject_duplicates`：`--before-ref`／`--bundle`／`--output-dir`／`--after-artifact`／`--cohort-manifest` 一律不接受重複，且排在任何取值之前 |
| 2（中） | **artifact 的型別驗證不完整**：`row_key()` 用 `str()` 硬轉，`timeframe: 123` 會悄悄變成 `"123"`；top-level 的 `timeframe`／`replay_scope`／`generated_at`／`provenance` 沒驗型別，也沒與 bundle manifest 比對 | `row_key()` 改成嚴格型別（非空字串，⛔ 不轉型）；`validate_after_artifact`／`validate_cohort_manifest` 補齊 top-level 型別與 SHA-256 形狀；新增 `assert_matches_bundle` 比對 `bundle_id`／`timeframe`／`replay_scope` |
| 3（中） | **結構性離線只驗 argv，沒有真的跑完 Stage 1／2**：證明不了 CLI 在 worktree 裡啟動得起來、bundle 與 artifact 在容器內載入得了、無網路無 DB 下跑得完 | 新增 **`scripts/smoke-replay-offline.sh`**（小型 fixture、秒級）：產一份合法 bundle → 把「目前工作樹 ＋ smoke 專用的 `rr_decoupling_candidate`」做成 tooling patch → 用官方腳本真的跑完 Stage 1／2 → 抽驗 artifact 自洽。⚠️ 預設不跑（要 docker build ＋ 兩次容器啟動 ＋ 兩個 worktree），`REPLAY_SMOKE=1` 才跑。⚠️ **「注入假 candidate」是當時的權宜做法，因為產品端還沒有那個欄位；I-074 Stage 0 完成後它會被移除**（連同「非空 cohort」斷言），理由見 I-074 Stage 0 計畫書四-B——⛔ 本列是歷史紀錄，不是現況操作說明 |
| 4（低） | `runner_sha256` 是第 5 個受保護參數，但主計畫的一覽與測試矩陣仍只列 4 個；Python 的重複參數測試也漏了它 | 三處全部補齊 |

**實際執行結果**（2026-09-10，`scripts/smoke-replay-offline.sh`）：

```
bundle b1_20260901_1d_… → Stage 1（after）→ after_artifact 35 列、cohort 5 列
                        → Stage 2（before）→ comparison 5 列、report 5 列
```

⚠️ **smoke 的 cohort 刻意用「決定性子集」而不是真的 predicate**：真的 predicate 在那份
合成資料上命中 0 列，cohort 與 comparison 都會是空的，**Stage 2 的比較路徑等於沒被走到**。
⛔ 這不能拿來推論任何命中率——它只證明管線走得完。

##### 十七、review 修正（2026-09-10，第三輪 review）

第三輪抓到 2 項（皆低），全部已修。⚠️ 兩項都是**回歸測試與文件的缺口**，不是功能漏洞——
實作本身早已保護五個參數，但「測試沒涵蓋、文件沒對齊」本身就是下一次改壞的入口。

| # | 問題 | 修法 |
|---|---|---|
| 1（低） | 測試矩陣與文件仍未對齊：spoof 迴圈漏 `--runner-sha256`；重複參數測試只涵蓋 3 個（漏 `--after-artifact`／`--cohort-manifest`）；`replay-args.sh` 的註解與本檔 1516 行仍寫「四個」 | spoof 迴圈補到五個；重複參數測試補到五個（Stage 2 的兩個用完整 Stage 2 argv 測）；兩處「四個」改成五個並註明是哪一輪補的 |
| 2（低） | **smoke 的 tooling patch 表達不出刪檔**：`tar` 疊在 HEAD worktree 上沒有刪除語意，工作樹刪掉的檔案在 scratch 會保留 HEAD 舊版 → smoke 可能跑在一份**現實中不存在的程式碼組合**上而給出綠燈 | ①同步改成 `rm -rf $SCRATCH/python` 之後再複製（精確的刪除語意）；②新增 **patch 忠實度守門**：把 patch 套到一份乾淨的 HEAD worktree，逐檔 hash 必須與 scratch 完全一致 |

⚠️ **忠實度守門的檔案清單交給 git 決定**（`ls-files --cached --others --exclude-standard`）：
那正好是「patch 帶得走的東西」。自己維護排除清單追不完——實測第一版就被
`.pytest_cache/` 與 `backtest/__init__.pyc` 這類 gitignore 產物擋下來過。

**實測刪除語意**：把一個受追蹤檔暫時移出工作樹再跑 smoke → 檔案數 140 → 139，
忠實度守門通過（舊的 tar 疊加版本在這裡會報「多了那個檔案」）。

##### 十八、Stage 0 正式執行（2026-09-10，D 日）

**已產出正式 bundle**：`b1_20260901_1d_74350966_5e92c731`，落在
`python/baselines/<bundle_id>/`，**4.7 MB**（計畫書估約 4.9 MB）。

| 項目 | 實際值 |
|---|---|
| 指令 | `scripts/run-evaluation.sh --as-of 2026-09-01 --symbols <11 檔> --emit-bundle /app/baselines --limit 1500` |
| 資料來源 | **live postgres，唯讀**（`write_db=0`；Stage 0 本身也擋 `WRITE_DB=1`）。與 2026-09-01 baseline 同源（`_cohort.source`） |
| symbols | `0050,00830,00947,00981A,2330,2399,2454,2478,3630,5490,6243`——由 `replay_cohort_2026-09-01.json` 的 `runs[*].rows[*].symbol` 實際取出 |
| readiness | `expected_latest=2026-09-01`、`market_latest=2026-09-10` → 通過 |
| 交易日曆 | `online`，涵蓋 `[2025, 2026]`（跨年涵蓋如規格要求） |
| provenance | image digest、`runner_sha256`、31 個專案模組 hash、python 3.11.16、runtime settings（⛔ 無 DSN） |
| chip／governance | 11／11 檔都有列（無「合法零筆」情境） |

**as-of 上界確實生效**——每一檔的最後一根都正好落在 **2026-09-01**（沒有它會是擷取當天的
2026-09-10），含兩檔短歷史標的：

```
0050/00830/2330/2399/2454/2478/3630/5490/6243  各 1500 根，末根 2026-09-01
00947   541 根（2024-06-12 起）、00981A  311 根（2025-05-27 起），末根同為 2026-09-01
```

**同日重跑驗證 no-op**：同一條指令再跑一次 → `published: false`、`bundle_id` 相同、
既有目錄**無任何檔案被改動**。⚠️ 這證明的是「同輸入 → 同 bundle ID」與重複產生的
no-op 路徑，**⛔ 不能替代跨日驗收**——當日資料本來就沒變。

⚠️ **與 I-074 記錄的可用根數有 1 根差異**：I-074 寫「11 檔的可用上限是 310～4,882 根」，
本次 `00981A` 在同一個 as-of 下是 **311** 根。差異落在誤差範圍內（可能是當時的量測日期或
事後補資料），⛔ 不影響本 bundle——**從現在起輸入以 bundle 為準**，那正是本筆要達成的事。

**尚未完成**：D+1（2026-09-11）載入同一份 bundle 重跑，確認輸入指紋與逐列結果相同。
bundle 依裁決在跨日驗收通過後才納入版控。

**⛔ 本筆的變更不上 live（2026-09-10 裁決）**——live runtime 用不到 `replay_bundle/`、
Stage 0／1／2 或那兩支腳本，而 Stage 0 讀 live 與 D+1 的離線 replay **都不需要部署**
（前者由 `run-evaluation.sh` 自己 build image 並掛 repo 的 `python/`，後者是 `--network none`
且無 DB 環境變數）。上線唯一會帶進 live 的行為差異是 `_load_db_sources` 的**重複 symbol
去重**（方向是修正——下游的 `_allocate_replay_quota` 與 `_decision_replay_rows` 本來就去重）。

⚠️ **因此 live 從 2026-09-10 起與 repo 有 2 個 runtime 檔的漂移**（`python/db.py`、
`evaluation.py`；此前為零漂移）。等下次有實際需要上 live 的異動時一起走 deploy 程序。
比對漂移的方法見 [`development-workflow.md`](./development-workflow.md)
「live 現在跑的是哪一版程式碼」。

##### 十九、D+1 跨日驗收（2026-09-11）與「還原係數會漂移」的實證

**條件 1 成立 ✅**（cohort identity 可重現）。同一條 Stage 0 指令在 D 日與 D+1 各跑一次：
**列身分集合完全相同**（`(symbol, timestamp)` 共 **14352** 列）。⚠️ 這是真正的跨日證據——
沒有 as-of 上界的話，今天會多抓 2026-09-02～09-11 的列。

**輸入指紋跨日不變 ✅**（條件 2 的前半）。D 日產的 bundle 在 D+1 離線載入，`bundle_id`、
`content_hash8`、`symbols_hash8` 與六份 payload 的逐檔 SHA-256 全部相同，
`captured_at` 仍是 D 日的時間戳。

#### ⚠️ 但 D+1 重跑產出的是**不同的 bundle**——而這正是本筆存在的理由

| | D 日（2026-09-10 17:54） | D+1（2026-09-11 10:30） |
|---|---|---|
| `bundle_id` | `b1_20260901_1d_74350966_5e92c731` | `b1_20260901_1d_74350966_`**`5d7ecb10`** |
| 變動的 payload | — | **只有 `candles.json.gz`** |
| 變動範圍 | — | `2330`（1500 列）＋ `00981A`（311 列）＝ **1811 / 14352 列** |
| 變動欄位 | — | `adj_factor` ＋ `open`／`high`／`low`／`close`；⚠️ **`volume`／`vol_factor` 完全沒動** |

**成因**：今天 06:30 的 `corporate_action_sync` 重算了還原係數。兩檔都有
`event_date=2026-09-16` 的 `DIVIDEND_CASH` 且 `volume_factor=1.0`——**現金股利只動價不動量**
（見 `db.py` 的 `fetch_candles` 說明），與觀察到的欄位分佈完全吻合。
實例：`00981A` 的 `adj_factor` **0.9399416722 → 0.9399213805**。

⚠️ **計畫書 v22 預言的情況在 24 小時內真的發生了**。當時寫的是：

> `--as-of` 固定的是**列範圍**，指紋能告訴你內容變了——但當 DB 歷史被修正、**還原係數更新**
> 或 model bundle 被替換時，**能做的只有中止，沒有任何機制讓你重新執行原來那份輸入。**

**沒有凍結 bundle 的話，[I-074](#i-074lifecycle-engine-的-rr-解耦decision-replay-已跑但一次都沒觸發到)
的 Stage 1 與 Stage 2 只要跨一天執行，就會用到不同的還原係數，而且不會有任何東西報錯。**
這是 I-100 存在理由的實證，比任何單元測試都有力。

#### 由此得到的兩條硬性結論

1. ⛔ **bundle 一旦選定就不得重產**：每過一天還原係數還會再變，重產出來的是**另一份**
   輸入。要換 bundle 等於整個 I-074 重來。
2. ⚠️ **任何「跨日比對 candles」的驗證都要先假設還原價會動**。⛔ 不能把
   「今天的還原價」與「昨天算出來的結果」直接相比而不檢查係數。

#### bundle 收斂決策（2026-09-11）

**留 `b1_20260901_1d_74350966_5d7ecb10`（D+1 這份），刪除 D 日那份。**
理由是 D 日那份的還原係數已被上游判定為過期；⚠️ **刪除不可逆**——係數已變，
那份再也產不出來，所以上表的差異證據先記在這裡。

⚠️ **跨日驗收的基準日因此重設**：選定的 bundle 是 2026-09-11 產的，
「逐列結果跨日相同」要以**今天為 D 日**、**2026-09-12 為 D+1**。

**尚未完成**：條件 2 的後半（逐列結果跨日相同）與條件 3 的正式實測。

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
P33/P67 是拿 **SMA 基準**量的，而 runtime 的自適應 builder 用 **Wilder**——
**門檻與 runtime 不同源**。目前該旗標是 `False`，所以還沒發作。

⛔ **本段原本接著寫「Wilder 系統性高 17～42%，打開旗標後分類就會系統性偏向高波動」，
2026-09-10 量測後撤下**（上表 5 檔是觀察值，不能外推成母體特性）。
現在的說法是：不同源**可能造成系統性分桶偏差，方向與幅度待合格母體量測**。
數據與限制見 [I-107](#i-107evaluationselection-用-tr-sma14runtime-用-wilder-atr14凍結門檻與-runtime-不同源)
的「步驟 1 量測（2026-09-10 執行）」。

**⛔ 這需要裁決，不是我可以逕行決定的**（三個選項互斥、都會動到已凍結的量）：
①統一用 Wilder ATR(14)／②統一用 TR SMA(14)／③承認是兩個指標。

⚠️ **三個選項的權威定義在 [I-107](#i-107evaluationselection-用-tr-sma14runtime-用-wilder-atr14凍結門檻與-runtime-不同源)
的「待決策：canonical formula 三選一」，本筆不再複述**（2026-09-10 review 訂正——
這裡原本維護第二份定義，其中選項 ③ 寫成「明確寫出哪一個在分桶、哪一個在門檻」，
⛔ 會被讀成分桶與門檻可採不同公式；實際規則是**先選 bucket authority、門檻必須與它同源、
另一個指標僅供獨立觀察**。兩份定義各自漂移正是這次要消滅的問題）。

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
| 狀態 | 待決策（2026-09-10：步驟 1 已執行，但**量到的母體不合格**——池外資料變舊，合格母體塌縮成 125 檔（2026-08-17 的同一套規則當時得到 319 檔），見下方「步驟 1 量測（2026-09-10 執行）」。裁決前置因此多兩步：**先回補池外日 K，再定義母體是固定 cohort 還是動態規則**） |
| 嚴重度 | 中（**目前不發作**——`SR_SCORING_ADAPTIVE_ZONE_BUILDERS_ENABLED` 在 live 為 `False`。確定的是**公式與門檻不同源**；打開後**可能造成系統性分桶偏差，但方向與幅度待合格母體量測**——⛔ 2026-09-10 前寫的「一旦打開就會系統性偏向高波動」是未經證實的斷言，見下方「步驟 1 量測」） |
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

`LOW/HIGH_VOLATILITY_THRESHOLD` 的 P33/P67 是 **2026-08-17** 拿 **TR SMA(14)** 基準
對當時合格的 319 檔量的（`zone_builder.py:59-70` 的 `VOLATILITY_THRESHOLD_PROVENANCE`），
而 runtime 的自適應 builder 用 **Wilder**，**門檻與 runtime 因此不同源**。

⛔ **本段原本接著寫「Wilder 系統性高 17～42%，打開旗標後分類會系統性偏向高波動」，
那是拿上表 5 檔外推的，2026-09-10 量測後撤下。** 目前站得住的只有「不同源」；
125 檔實測顯示 **24/125 的 Wilder 反而較低**、凍結門檻下**只有 2 檔換桶且都往 LOW**。
17～42% 是上表那 5 檔的觀察值，不是母體特性。

**因此風險改寫為**：不同源**可能造成系統性分桶偏差，方向與幅度待合格母體量測**。
完整數據與限制見下方「步驟 1 量測（2026-09-10 執行）」。

⚠️ **維度有三個，不是「14 或 60」的選擇題**：ATR period 多少、方法是 SMA 還是 Wilder、
給多少根 warm-up／lookback。單把 evaluation 的 period 改成 60 會得到 **`TR SMA(60)`**，
仍然不等於 runtime 的 Wilder ATR(14)。

#### 待決策：canonical formula 三選一

1. **統一用 Wilder ATR(14)**——與 Go `CalcATR`、與 runtime 一致；**凍結門檻必須重測**。
2. **統一用 TR SMA(14)**——與凍結門檻、選池、既有基準一致；runtime 要改，
   且與 Go 端的 ATR 定義分家。
3. **承認是兩個指標**——分別命名（`atr_pct_sma14` / `atr_pct_wilder14`）、各自記
   provenance，並**先選定哪一個是 bucket authority**；**門檻必須與該 authority 同源**，
   另一個指標只能作**獨立命名／觀察**用途。
   ⛔ **不要讀成「分桶與門檻可以選不同公式」**（2026-09-10 review 訂正——原文寫
   「明確寫出哪一個在分桶、哪一個在門檻」，容易被讀成兩者可以分開挑；
   分桶與門檻不同源正是本筆要解的問題本身）。
   authority 定了之後，門檻要不要重測與 `universe_version` 要不要升版，
   由下方步驟 4 的同源判準自動決定。

#### 決策前的必要量測與步驟

1. 先量測 **SMA14 與 Wilder14 在「資料新鮮、且套用既定資格規則的全市場股票母體」上的
   分佈與 bucket 差異**（不能只看 5 檔）。
   ⛔ **不要寫成「319 檔」**——319 是 **2026-08-17 那次量測的歷史規模**，不是驗收筆數；
   只有明確選定「固定 cohort」時 319 才是必要筆數（見下方「因此原步驟 1 之前多兩步」）。
2. 選定 canonical formula。
3. 若公式改變：重算 P33/P67 → 升 `universe_version` → 更新 135 列的
   `bucket_edge_low/high` → 與 T-003「bucket 邊界必須凍結」對齊。
4. **完成前 `SR_SCORING_ADAPTIVE_ZONE_BUILDERS_ENABLED` 維持關閉。**

#### 步驟 1 量測（2026-09-10 執行）

⚠️ **這次量測沒有完成步驟 1**——步驟 1 要的是「資料新鮮、且套用既定資格規則的全市場
股票母體」（同一套規則在 2026-08-17 得到 **319 檔**），實際只量到 **125 檔**。
兩件事要分開讀：下方「量到什麼」在它自己的母體上可信，
但「能不能拿來裁決 canonical formula」的答案是**不能**。

##### 母體塌縮：同一套規則從 319 檔（2026-08-17）掉到 125 檔，成因是 I-075

`stock_symbols` 裡 503 檔上市股票，依日 K 最後一天分群：

| 是否在選池 | 日 K 最後一天 | 檔數 |
|---|---|---|
| ❌ 池外 | **2026-08-12** | **364** |
| ✅ 池內 | 2026-09-08 | 126 |
| ❌ 池外 | 2026-08-05 | 11 |
| ❌ 池外 | 2026-08-11 / 2026-04-12 | 各 1 |

503 檔裡 **377 檔被判 `stale_candle`**，只剩 125 檔通過 `evaluate_exclusion`，
而且**這 125 檔全部都在選池內**——母體塌縮成選池本身。
（⚠️ 資格規則沒變，變的是資料新鮮度；**319 是 2026-08-17 套同一套規則的結果，不是固定筆數**。）
這正是
[I-075](#i-075重跑選池會因池外資料變舊而靜默通過)，回補成本見該筆。

⚠️ **本次是 I-075 第一次擋到「重跑選池」以外的用途**——它原本只描述選池重跑，
實際上**任何需要全市場母體的量測都會被擋**，包含門檻重測與本筆的步驟 1。

##### 量測條件

`1d`／60 個市場交易日／**2026-06-16 ～ 2026-09-09**／掃描 856 檔（股票 503、ETF 353）。

兩種公式走**同一次 DB 讀取、同一份 60 根切片**——刻意不跑兩次報告再比對，那會把資料滾動
混進公式差異（與 I-075 同一類坑）。公式一律 import 既有實作，不另寫一份：
SMA14 用 `evaluation._atr_pct`；Wilder14 照 `scoring._adaptive_zone_builder_profile`
的形狀呼叫 `indicators.calc_atr`，含 `last_close > 0` 與 `atr > 0` 兩個守門。

##### 對照組：本筆「現象」表那 5 檔

| 標的 | SMA14（09-09） | Wilder14（09-09） | 比值 | 原表（09-02）比值 |
|---|---|---|---|---|
| `0050` | 1.511% | 1.716% | 1.135 | 1.243 |
| `2330` | 1.623% | 1.808% | 1.114 | 1.192 |
| `2478` | 5.977% | 7.271% | 1.217 | 1.184 |
| `5490` | 2.293% | 3.489% | 1.521 | 1.424 |
| `6243` | 3.735% | 5.695% | 1.525 | 1.174 |

方向與量級吻合（資料已滾動 8 天，本來就不會逐位相等）。**Wilder 這一側沒有抄錯。**

##### ⛔ 訂正：「Wilder 系統性高 17～42%」在 125 檔母體上沒有重現

「現象」段那 5 檔全落在分佈的偏高側，不是母體的代表值：

| | P10 | P33 | P50 | P67 | P90 |
|---|---|---|---|---|---|
| `atr_pct` SMA14 | 1.75% | 2.69% | 3.41% | 4.10% | 6.07% |
| `atr_pct` Wilder14 | 2.12% | 3.22% | 3.95% | 4.60% | 6.10% |
| **Wilder/SMA 比值** | **0.963** | 1.076 | 1.130 | 1.196 | 1.327 |

比值全距 **0.754～1.562**，**24/125（19.2%）的 Wilder 比 SMA 低**。

⚠️ **這不等於原敘述已被推翻**——原敘述講的是回補完整的全市場母體，本次量的是 125 檔選池。
在合格母體上是否成立**目前無法回答**，不要把本節當成反面結論。

##### 換桶影響遠小於預期，原因是 `max()` 基準

**情境 a——凍結門檻 ＋ Wilder**，也就是直接打開
`SR_SCORING_ADAPTIVE_ZONE_BUILDERS_ENABLED` 的形狀（runtime 走
`pipeline.py:50-51` → `resolve_zone_builder_config_for_profile`
→ `volatility_bucket_from_profile`，同樣是 `max()` 基準配凍結門檻）。

**只有 2/125（1.6%）換桶，而且兩檔都往低波動走**：

| 標的 | 變化 | SMA14 | Wilder14 | `average_range_pct` |
|---|---|---|---|---|
| `2606 裕民` | NORMAL → **LOW** | 5.126% | 3.956% | 3.236% |
| `6547 高端疫苗` | NORMAL → **LOW** | 4.898% | 4.149% | 3.964% |

**情境 b——Wilder ＋ 重測 P33/P67**（隔離掉母體漂移：兩側都用今天同一母體重取分位數）：
**4/125（3.2%）**，2 上 2 下——`3035 智原`、`2485 兆赫` NORMAL → HIGH；
`2615 萬海`、`2637 慧洋-KY` HIGH → NORMAL。其中 `3035` 與 `2485` 的 basis 值
**在兩種公式下完全相同**，它們的移動純粹來自切點位移，與 ATR 公式無關。

⛔ **不要引用「現行 live 分桶 → Wilder 重測後分桶」的 52/125**——那把母體漂移與
公式變化混在一起，回答不了公式本身的影響。

**為什麼影響這麼小**：`bucket_basis = max(atr_pct, average_range_pct)`，而
**99/125（79.2%）的 basis 就是 `average_range_pct`**。對這 79% 來說，ATR 用哪個公式
對分桶毫無影響。**這是原本三選一的討論沒有納入的因素**，也是重新評估嚴重度時要帶上的前提。

切點對照：

| | P33 | P67 |
|---|---|---|
| 凍結常數 | 0.046089927430152715 | 0.06278197721225691 |
| 今天重測（SMA） | 0.038855480706969490 | 0.05242555539723416 |
| 今天重測（Wilder） | 0.038855480706969490 | 0.050221561858518386 |

P33 兩者完全相同，同樣是因為該位置的 basis 是 `average_range_pct`。

##### 因此原步驟 1 之前多兩步：回補與母體定義（2026-09-10 定案）

0. **回補池外標的的日 K**（2026-09-10 的缺口是約 377 檔 × 約 19 個交易日）。
1. **先決定母體是固定 cohort 還是動態規則**，⛔ **不要寫成「讓 319 檔母體重新可量」**
   （2026-09-10 review 訂正——回補後重新套用流動性、上市狀態與資料完整度規則，
   合格數量**不保證仍是 319**）：

   | 選項 | 內容 | 適用時機 |
   |---|---|---|
   | **固定 cohort** | 釘住 2026-08-17 那批原始 319 檔，只比較公式差異。**此時 319 才是必要筆數** | 只想隔離「公式」這一個變因時 |
   | **動態母體** | 回補後重新套用相同資格規則，檔數可以不是 319 | **要重測「當下母體」的 P33/P67 時用這個** |

   本筆步驟 3 的目的是重測當下母體的分位數，因此**預設走動態母體**。
   **驗收條件是「母體規則與資料新鮮度」，⛔ 不是「恰好 319 檔」。**
2. 在該母體上重跑本節的量測。
3. 裁決 canonical formula。
4. **依裁決結果分三支**（對應「待決策」段的三個選項，⛔ 三支都要有交代，不能只寫兩支）：

   | 裁決 | 門檻要不要重測 | `universe_version` | bucket authority |
   |---|---|---|---|
   | **① 統一 Wilder ATR(14)**（公式改變） | **要**——以該公式與同一 `max(atr_pct, average_range_pct)` 基準重算 P33/P67，更新凍結門檻與 `VOLATILITY_THRESHOLD_PROVENANCE` | **升版**，並同步 `bucket_edge_low/high` 與 `bucket_hint` | Wilder，evaluation 與 runtime 同源 |
   | **② 統一 TR SMA(14)**（維持現狀） | ⛔ **不是本筆的必要關閉條件** | 「要不要因母體漂移升版」是**另一個獨立決策**，需另行裁決，不能當成本筆的附帶結果 | SMA14；**runtime 的 `_adaptive_zone_builder_profile` 要改成 SMA14** 才算同源 |
   | **③ 承認是兩個指標**（`atr_pct_sma14` / `atr_pct_wilder14`） | **先決定 bucket authority，再套 ① 或 ②** | 同左 | ⛔ **必須明寫哪一個是 bucket authority**——這正是原選項 3 沒寫完的部分 |

   ⚠️ **判準統一是「裁決後的 bucket authority 與 v2 門檻的 provenance 是否同源」**
   （`VOLATILITY_THRESHOLD_PROVENANCE` 記的基準是 **TR SMA(14)**）：

   * **不同源**（authority ＝ Wilder）→ **重測門檻與升版是必要動作，⛔ 不能選「不升版」**；
   * **同源**（authority ＝ SMA14）→ 重測不是必要動作，「要不要因母體漂移升版」另行裁決，
     此時升版與不升版**才都是合法結果**。

   ⚠️ **選項 ③ 不是「待定」，而是「先決定 bucket authority」**——一旦決定，
   門檻要不要重測與 `universe_version` 要不要升版，就依上面的同源判準處理。
   ⛔ authority 沒定之前這一支無法執行；若不打算定，就先明確撤銷選項 ③。

   ⚠️ 更新池資料時注意筆數：`evaluation_universe` 是 135 筆 `active`，
   但不指定 symbols 的路徑實際只跑到 134 檔（`2867` 被 `is_listed=false` 濾掉，見
   [I-113](#i-1132867-是-active-選池成員但-is_listedfalse未指定-symbols-的全市場路徑會靜默排除它)）。
5. 完成後才跑 [`todo.md`](./todo.md) T-003 P2 的 coarse sweep（三選一的完整比較與裁決見那裡）。

⚠️ **P33/P67 只保證「量測母體」本身近似三等分**——固定的 evaluation pool 是人工決策的子集，
不是量測母體，它的分佈只能**預期較均衡**，**不保證精確三等分**。

⚠️ **替代路線，尚未採用**：若回補成本不可接受，另一個選項是把 canonical 母體的定義
從「全市場流動性合格股票」（2026-08-17 套此規則得到 319 檔）改成「選池」。那會讓步驟 1 立刻可執行（本節就是），
但**等於換掉凍結門檻的母體，屬於 contract 變更**，要另外裁決，不能當成省事的預設。

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

---

### I-113：`2867` 是 `active` 選池成員但 `is_listed=false`，未指定 symbols 的全市場路徑會靜默排除它

| 欄位 | 內容 |
|---|---|
| 狀態 | 待決策（**不知道是刻意保留還是漏收**，見下方「尚未查證」） |
| 嚴重度 | 中（**只影響不指定 symbols 的路徑**；那些路徑宣稱 135 檔、實際 134 檔，而且**差在哪不會有任何訊息**） |
| 分類 | 資料一致性 / 評估標的池 |
| 建立日期 | 2026-09-10 |
| 來源 | [I-107](#i-107evaluationselection-用-tr-sma14runtime-用-wilder-atr14凍結門檻與-runtime-不同源) 步驟 1 量測時池內成員數對不上（135 vs 134）|

#### 事實（2026-09-10 對 live 唯讀查證）

| 欄位 | 值 |
|---|---|
| `evaluation_universe.active` | `true` |
| `evaluation_universe.universe_role` | `primary` |
| `stock_symbols.security_type` | 股票 |
| **`stock_symbols.is_listed`** | **`false`** |
| 日 K 最後一天 | **2026-08-18** |

`evaluation_universe` 仍是 135 筆 `active=true`；符合這個形狀的只有 `2867` 一筆。

#### 影響範圍：只有「不指定 symbols」的路徑，⛔ 不是所有 Python 分析路徑

⛔ **本筆初版寫「靜默從所有 Python 分析路徑消失」是錯的**，2026-09-10 review 訂正。
`is_listed` 這道濾網只在**產生候選清單**的那一段，不在**分析**那一段：

| 路徑 | 會不會排除 `2867` |
|---|---|
| `selection_report.py:717` → `db.fetch_symbol_universe()`（不帶 `symbols`） | ✅ **會**——`WHERE is_listed = :listed`（`python/db.py:191-192`，`listed=True`） |
| `evaluation.py:77` `_load_db_sources` → `fetch_candles(symbol, …)` | ❌ **不會**——逐檔直接取 K 棒，**完全不檢查 `is_listed`** |

全 repo（排除 tests）只有 `selection_report.py:717` 呼叫 `fetch_symbol_universe`。
所以 `run_evaluation` / `run_builder_sweep` 只要 `--symbols` 裡帶了 `2867`，
**它照樣會被載入並算出 profile**——用的是停在 2026-08-18 的 K 棒
（`_volatility_profiles` 本身沒有 stale 判定，那是 `selection_report` 才有的邏輯）。

**問題出在哪**：`2867` 在**不指定 symbols 的全市場 selection／measurement 路徑**上
**從一開始就沒有進入母體**——不是算不出 profile 被排除，是根本沒被載入，因此
**不會出現在任何排除原因統計裡**，也沒有 warning。這類路徑「池有 135 檔」的敘述
與實際跑到的 134 檔之間，沒有東西對得起來。

`066_evaluation_universe.sql:29` 的註解寫「`false` ＝ 保留紀錄但不再納入每日維護」——
一檔已下市的標的仍掛 `active=true`，與那句話的語意對不上。

#### 尚未查證（所以狀態是待決策而非待修復）

* **`is_listed` 是怎麼變成 `false` 的**。`todo.md` T-071 明訂**不驅動 `is_listed`**，
  所以來源不是它；沒有進一步追。
* **是否刻意保留在池內**。入池/退池的歷史本身是研究紀錄（同上註解），
  所以「下市了還留著紀錄」可能是預期行為，只是 `active` 該不該同時轉 `false` 沒有定義。

⛔ **在這兩件事查清楚之前不要直接改資料**——把 `active` 改成 `false` 會改動選池成員數，
那是 T-040 的人工決策範圍。

#### 關閉條件

任一即可：

1. 定義出「池內成員下市時 `active` 要不要跟著轉 `false`」的規則，
   寫進 [`database-schema.md`](./database-schema.md) 的 `evaluation_universe` 章節，
   並依該規則處理 `2867`；或
2. 明確決定維持現狀，但讓落差**可見**——例如讀池成員的路徑在
   `active` 與 `is_listed` 不一致時發 warning，而不是靜默少一檔。

無論走哪一條，「池 135 檔」這個數字在文件與報告裡都要能對得上實際參與數。

⚠️ **順帶**：`2867` 也是 I-105（已收斂）的來源標的——當時是它跨月當天在 live 首次
`partial`。兩者是不同的問題，這裡只記關聯，不要當成同一筆。

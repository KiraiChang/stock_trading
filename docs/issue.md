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
- **下一個新編號從 `I-117` 起算。**（⚠️ **`I-114` 是跳號，⛔ 不得再發用**——2026-09-17 發 I-115 時誤把本行索引文字裡的 `I-114` 當成已發出的條目，於是直接跳到 I-115；依「編號只增不重用」，I-114 就此列為**已跳過**。**I-115 / I-116 於 2026-09-17 發出**——I-115 由 I-074 Stage 1 正式執行時踩到（Stage 1 強制要求 `--before-ref` 卻不使用它，六步程序漏寫）；I-116 由同一次執行的 `InconsistentVersionWarning` 查出（凍結 bundle 的可重現性只靠 image 還在）。**I-113 於 2026-09-10 發出**——由 I-107 步驟 1 量測時發現 cohort 母體對不上（`evaluation_universe` 135 筆 `active` vs full-market report 可辨識的 134 檔，⚠️ **兩者是不同的母體定義**且沒有對帳）。**I-104 / I-105 / I-112 於 2026-09-09 收斂**——三筆的現況都歸檔在 `architecture.md`：I-104 的「**外來錯誤必須先分類，不得把原始錯誤寫進使用者可見欄位**」與**合法形式表**在「寫入失敗的一致性契約」；I-105 的「`verification_unavailable` 一定要帶得出成因」在「日 K 缺漏偵測」；I-112 的 `<stage>_failed:N (symbol:reason, …)` 在「逐檔失敗要帶得出哪一檔、哪個階段、什麼類別」。**編號都不回收。** **I-112 於 2026-09-08 發出**——`todo.md` T-070（已收斂）的 live 觀察時發現 `corporate_action_sync` 的逐檔失敗不寫進 `error`。**I-109 / I-110 / I-111 於 2026-09-07 發出、2026-09-08 全部修復並收斂**——三筆都由 `todo.md` T-071 的實作與 review 分出：I-109 是 `parseROCDate` 收下不存在的民國年（現況歸檔在 `architecture.md`「民國日期的解析是嚴格的」）；I-110 是前端 job 清單漂移（歸檔在 `development-workflow.md`「新增排程還要同步前端的 job 清單」，並由 `scripts/check-job-names.sh` 擋住）；I-111 是 SQLite 的 `busy_timeout` 保護不到 deferred transaction 的升級（歸檔在 `database-schema.md` 的 CAS 契約，改用 `BEGIN IMMEDIATE`）。**編號都不回收。** **I-108 於 2026-09-04 發出**——由 I-106 計畫書的非有限值實測分出；**I-106 / I-107 於 2026-09-03 發出**——I-107 由 I-106 的 review 分出（TR SMA(14) 與 Wilder ATR(14) 的公式分歧）；I-106 來自 T-040 regression baseline 實跑——T-040 regression baseline 實跑時發現 `atr_pct` 的窗口與註解不符、且 evaluation 與 runtime 用的是兩個不同的 ATR 演算法；**I-103 / I-104 / I-105 於 2026-09-02 發出**——I-103 由 I-102 計畫書 review 分出（Yahoo 批次路徑給不出逐檔寫入失敗）；I-104 由 I-102 實作 review 分出（其餘排程與 job 紀錄仍直接寫入原始錯誤）；I-105 來自 `2867` 跨月當天 live 首次 `partial`（`verification_unavailable` 的成因被丟棄）；I-101 / I-102 於 2026-09-01 發出——前者來自 live 的 indicator upsert 溢位、**已於同日修復並收斂**（未完成的 live 部署由 `todo.md` T-069 承接，**該筆已於 2026-09-02 部署驗收完成並收斂**），後者由它的 review 分出、**2026-09-02 實作部署完成並收斂**（現況規格歸檔在 `architecture.md`「寫入失敗的一致性契約」與 `api-reference.md` 的兩條端點，未完成的執行期觀察由 `todo.md` T-070 承接）；I-100 於 2026-09-01 發出，由 `todo.md` T-068 同日改列——**T-068 編號不回收**；**I-099 於 2026-08-31 發出後同日作廢**——誤把 `deploy.sh` 的保守預設當成與 live 的衝突，實際上該檔是範本、所有開關一律預設 `false` 是既有慣例；**編號不回收**；I-098 於 2026-08-31 由 I-096 的 review 發現分出；I-081～I-083 於 2026-08-21 發出（**I-081 / I-082 於 2026-08-27 隨 `todo.md` T-055 收斂**），I-084～I-087 於 2026-08-24 發出，I-088～I-092 於 2026-08-25 發出（**I-091 於 2026-08-28 收斂**），I-093 / I-094 於 2026-08-26 發出（I-093 已於同日收斂，**I-094 於 2026-08-28 收斂**），I-095～I-097 於 2026-08-27 發出，其中 **I-097 於同日改列 `todo.md` T-064**——編號**不回收**。）
  **發出新編號時記得把這一行一起往前推**——上一次就是漏了這步，I-089 發出去之後
  這裡還寫著「從 I-089 起算」，差一點又重用一次（I-070 已經發生過）。
  **現存條目**裡最大的是 I-116（⚠️ 本行下方的歷史索引仍看得到更大的編號，那是紀錄不是條目）。I-109～I-111 於 2026-09-08 收斂、I-104／I-105／I-112 於 2026-09-09 收斂，I-113 於 2026-09-10 發出，I-115 / I-116 於 2026-09-17 發出（**`I-114` 跳號、⛔ 不得再發用**），**下一個可用的是 I-117**；I-102 已於 2026-09-02 收斂、編號不回收——I-096 / I-098
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
  本檔現有的 I-100 / I-103 / I-106 / I-107 / I-108 / I-113 / I-115 / I-116、已收斂的 I-102 / I-104 / I-105 / I-109～I-112、
  已跳號的 I-114，以及下一個可用的 I-117），
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

##### 2026-09-17 回補成本重算（live 唯讀量測）

⚠️ **成本比 2026-08-18 記錄的那次少一半**——當時要補 706 檔，現在是 **377 檔**：

| 日 K 最後一天 | 檔數 | 位置 |
|---|---|---|
| 2026-09-15 | 126 | ✅ 池內（新鮮） |
| **2026-08-12** | **364** | ❌ 池外 |
| 2026-08-11 | 1 | ❌ 池外 |
| 2026-08-05 | 11 | ❌ 池外 |
| 2026-04-12 | 1 | ❌ 池外 |

（母體＝`stock_symbols` 裡 `is_listed AND security_type='股票'` 共 503 檔。）

| 項目 | 值 |
|---|---|
| 要回補的檔數 | **377** |
| 落後天數 | 約 **36 天**（2026-08-12 → 09-17），約 24 個交易日 |
| 速率 | FinMind **5 req/min**，每檔一次呼叫取一段區間 |
| **預估耗時** | **約 75 分鐘**（377 ÷ 5；對照 2026-08-18 的 706 檔／2.4 小時，模型一致） |

⛔ **現階段沒有更快的路徑**：`T-043`（盤後用 Yahoo 批次補日 K）**仍是「待規劃」**，
現有的 `yahoo_quote.go` 是**盤中報價**批次（40 檔／次），⛔ 不是歷史日 K 回補。

⚠️ **這 75 分鐘是 I-107 遷移的唯一硬前置**——公式已裁決為 Wilder ATR(14)，
但重測 P33/P67 需要「資料新鮮的動態全市場母體」，而那正是本筆擋住的東西。

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
| 狀態 | **Stage 1 已完成／待 Stage 2**（⚠️ **2026-09-18**：D／D+1／仲裁**全部跑完，`outcome = MATCH`（rc=0）**，證據已封存到 `python/baselines/i074_stage1/`。**候選數 156 > 0 → ⛔ 排除分支 A**，必須跑 Stage 2 才分得出 B／C。見下方「Stage 1 正式執行結果」）。處置＝**只執行一次有界定向驗證，零命中即收斂成已知限制**，步驟與判準見下方「處置（2026-09-01 定案）」與「關閉條件（2026-09-01 改為單一決策樹）」。**在決策樹的某一個分支被走完之前不得移除本筆** |
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
`run_decision_replay()`（`evaluation.py`）的資料來源是 `_load_db_sources()`——
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
| **0** | 完成可重現性前置（I-100）與診斷欄位，**並產生封存凍結輸入 bundle**（此時才讀 DB）。⚠️ **I-100 的工具已於 2026-09-10 實作完成**（`--as-of`／`--emit-bundle`／`scripts/run-replay-offline.sh`，用法見 [`development-workflow.md`](./development-workflow.md)）。⚠️ **本階段已於 2026-09-14 全部完成**（2026-09-16 更新）：診斷欄位已實作並 review 通過，bundle `b1_20260901_1d_74350966_5d7ecb10` 已進版控 | ✅ 可重跑的 replay 路徑 ＋ 新欄位 ＋ 已封存的 bundle |
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

  ⚠️ **上表是「一般 Stage 2 路徑」限定**（2026-09-22 加註）：**I-074 的 counterfactual 路徑
  （`--i074-counterfactual`）⛔ 不適用**——那條路徑⛔ 移除集合相等檢查、⛔ 不產
  `candidate_mismatch.json`、terminal outcome **恆為 0**，B／C 改由 comparison 的 156 列判讀。
  見「I-074 Stage 2 計畫書」（⚠️ 現行版）的「二、①」與「二、④」。

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

##### ①②實際執行結果（2026-09-17，上表的估算已由實測取代）

步驟 ① pin 與 ② capacity probe 都已完成（rc=0）。
⚠️ **本節是 2026-09-17 當下的紀錄**——③ 當時尚未執行，
**已由 2026-09-18 的「Stage 1 正式執行結果」取代**（D／D+1／仲裁全部跑完、`MATCH`）。

| 項目 | 值 | 判讀 |
|---|---|---|
| image ID（六個角色共用） | `sha256:d66030dca485…` | 已釘死；identity 已建立，後續 pin 走 no-op、⛔ 不 build |
| probe `base_commit` | `745b74332dc2…` | ⚠️ 若 ③ 之前又上版，D／D+1 會是新的 commit——**不影響**（見 `development-workflow.md`） |
| 峰值 RSS | **265.2 MiB** | cgroup 上限 396.0 MiB，**餘裕 130.8 MiB** |
| host low watermark | 343.0 MiB | 峰值低於它，⛔ 不會觸發 host OOM killer |
| 速度 | 200 列／165.3 秒 ＝ **0.826 秒／列** | 比舊估的 0.85 **快 2.8%** |
| 全量推算 | **3.08 小時／趟**；D＋D+1 共 **6.16 小時** | 落在上表 3.7 小時保守上界內 |

⛔ **上面那句「容量結論：③ 跑得動」已被 2026-09-17 的實跑推翻**——見下方
「③ 第一次執行：OOM 失敗」。⚠️ **probe 的峰值⛔ 不能外推到全量。**

⚠️ 執行時踩到 [I-115](#i-115stage-1-強制要求---before-ref但那一趟根本不用它六步程序漏寫正式執行第一步就失敗)：
六步程序的 ②③ 漏寫必填的 `--before-ref`，文件已補 `ecbc141^`。

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
| **decision primary zone** | ⚠️ **已經有了，只是 replay 取錯顆** | `build_decision_summary()` 早就輸出 `decision_summary["primary_zone"]`（`decision_engine.py:2982`，來源是 `_pick_primary_zone()`）；`evaluation.py` 卻用 `_historical_zone_score_summary` 的排序第一筆（`:773`）。⚠️ **2026-09-11 修正：「不需要動 `decision_engine.py`」不成立**——`_decision_summary_zone()`（`:112`）**沒有 `relative_volume`**，直接切過去會讓 `_volume_strength_bucket()` 靜默退化成 unavailable，所以要在那裡補一個欄位；且 `evaluation.py` 有**三個** consumer 必須一起切（replay row `:1519`／`daily_confirmation_context` `:1009`／`daily_confirmation_outcome` `:1155`） |
| `price_follow_through_state` / `momentum_confirmation_state` | ✅ 已在 replay row | `evaluation.py:1034-1035` 的 `daily_price_follow_through` / `daily_momentum_confirmation` |
| `rr_gate.qualified` | ✅ 已在 replay row | `_decision_fields_from_summary`（`:804`） |
| `event_signal` / `structure_state` | 需補 | 自 decision summary 帶出 |
| `clear_zone_breakout` | ❌ **拿不到** | `resolve_lifecycle()` 內的區域變數（`lifecycle_engine.py:172`），從不回傳。**由 lifecycle 層新增輸出**（診斷用） |
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

⚠️ **`setup_rr_qualified` ⛔ 不是對外的 `rr_gate.qualified`**（2026-09-11 Stage 0 review
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

⚠️ **上面的行號是對 `ecbc141^` 查證的**（2026-09-01）。Stage 2 計畫書（v18 起）把 before 基準
改成 **`e1cbbbd` ＋ counterfactual patch**，⚠️ **等價式與這條前提在新基準下反而更直接成立**
——兩側是**同一個 base**，唯一差異就是 patch 把 `and setup_rr_qualified` 加回
`CONTINUATION`，呼叫順序⛔ 不可能不同。⛔ **但那仍是要在 ② 實際查證的事，⛔ 不是可略過的推論**
（⚠️ 教訓見「⛔ 阻擋 v1 的事實」：v1 就是把「函式本體相同」誤當成「逐列輸出相同」）。

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

⛔ **上面這句已被 Stage 2 計畫書取代**（v18 起，2026-09-22）：那是 before 基準為 `ecbc141^`
（無 RR 條件的舊版）時的模型。改用 **`e1cbbbd` ＋ counterfactual patch** 之後，
before 的 `CONTINUATION` **含 RR**，候選集合**預期恆為空**——⚠️ **那是正確行為，⛔ 不是分支 C**。
現行模型見「I-074 Stage 2 計畫書」（⚠️ 現行版）的「一、v3 比較模型」與「二、①」。

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
| `evaluation.py` | **只匯出，不自行重算**；⚠️ 例外是 **no-zone 列的合法缺席 fallback**（契約見 [`sr-zone-scoring.md`](./sr-zone-scoring.md) 的九欄位 schema），那是 serialization 層填預設值，⛔ 不是重算上游判斷 | 任何在 replay 端重算的版本都會重蹈偽陽性 |

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
  contract**（九個診斷欄位，見 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「九個診斷欄位的完整 schema」），否則 Stage 2 的 validator 與逐列比較
  都無從比起（⚠️ **第五道集合檢查已由 v18 移出 counterfactual 路徑**）。**before tooling 的完整輸出責任延到 Stage 2 計畫定義**，
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

#### Stage 0 計畫書（v1～v17，2026-09-16 收斂）

⚠️ **計畫書內容已移除**——17 個版次的完整規格與修訂紀錄（524 行）在 review 通過後收斂。
現況規格已歸檔到 [`sr-zone-scoring.md`](./sr-zone-scoring.md)：

| 主題 | 歸檔位置 |
|---|---|
| 九個診斷欄位的完整 schema ＋ no-zone fallback mapping | 「九個診斷欄位的完整 schema」 |
| Stage 2 的**第五道**集合檢查 | 「Stage 2 的第五道集合檢查」 |
| `candidate_mismatch.json` 的完整 schema | 「`candidate_mismatch.json` 的完整 schema」 |
| 兩種 terminal outcome ＋ 結束碼 4 | 「Stage 2 有兩種 terminal outcome」 |

⚠️ **計次裁決⛔ 沒有跟著刪**——它是尚未執行的政策，已移到本筆的「正式 scan 的計次裁決」。

#### Stage 0 實作結果（2026-09-14）

**已依 v17 計畫書實作完成，2026-09-14 review 已確認實作方向、⛔ 無高／中嚴重度問題。**
（v16 的 review 抓到 3 中 1 低，修正內容見修訂表 v17 與下方各列的 ⚠️ 標註。）

⚠️ **review 通過的是 Stage 0，⛔ 不是本筆**：I-074 當時的狀態是「待執行」
（⚠️ **截至 2026-09-16**；Stage 1 已於 2026-09-18 執行完畢並 `MATCH`）——決策樹的三個分支
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

⛔ **截至 2026-09-16 尚未執行**（⚠️ **Stage 1 已於 2026-09-18 完成**，見上方
「Stage 1 正式執行結果」；此處保留當時敘述）：正式 Stage 1／2（那是下一階段，且依已確認的計次裁決，D／D+1 兩趟 after
合為一次）。

#### Stage 1 計畫書（v1～v25，2026-09-16 收斂）

⚠️ **計畫書內容已移除**——25 個版次的完整規格與修訂紀錄（845 行）在 review 通過後收斂。
現況規格已歸檔：

| 主題 | 歸檔位置 |
|---|---|
| preflight／crossday／capacity probe／evidence 四個契約 | [`sr-zone-scoring.md`](./sr-zone-scoring.md)「I-074 Stage 1 的四個永久契約」 |
| provenance 的欄位集合與型別（含 `build_provenance()` 的求值順序陷阱） | 同上「provenance 的欄位集合與型別」 |
| SHA 與 canonical 的精確語意、`load_canonical_evidence_artifact()` | 同上「SHA 與 canonical 的精確語意」 |
| 六步執行程序、結束碼表、三個位置分離 | [`development-workflow.md`](./development-workflow.md)「I-074 Stage 1 的正式執行程序」 |
| finalizer 的 CLI matrix、`bundle_id` 的四類比對時點 | 同上「finalizer 的 CLI matrix」 |
| 寫多輪計畫書的兩條守則（42 個版次換來的） | 同上「寫多輪計畫書的兩條守則」 |

#### Stage 1 實作結果（2026-09-15）

**已依 v25 計畫書完成程式與自動化測試；2026-09-15～16 經多輪 review，逐輪修正如下。**
⚠️ 這裡**刻意不寫輪數與表數的摘要**——每次 re-review 都會讓那個數字漂掉；
實際輪次以本節末各張「第 N 輪 review 的修正」表為準。
⛔ **本輪只交付程式與測試：正式的 D／D+1 兩趟 replay 尚未執行，本筆在那之前不得關閉。**

| 檔案 | 內容 |
|---|---|
| `replay_bundle/provenance.py` | `PROVENANCE_FIELDS`／`PROVENANCE_ROLES` 單一真相源 ＋ `validate_provenance(role=)`（四種 role 的 nullability、10 欄精確型別、`runtime_settings` 是「⊆ 五個」） |
| `replay_bundle/artifacts.py` | `validate_replay_errors()`（從 `evaluation.py` 搬來公開化）、`EvidenceLoad`、`load_canonical_evidence_artifact()`（`.json` 用 SHA 對照⛔ 不重讀；`.json.gz` 驗 round-trip byte-identical） |
| `replay_bundle/publish.py` | `EXIT_CROSSDAY_MISMATCH = 5`（與其他 exit constants 同一落點） |
| `replay_bundle/run_identity.py`（**新增**） | 封閉 schema、兩格式載入、原子建立、**依「檔案存不存在」分流**的重入、commit-point 三段（含 no-op 的重新 fsync ＝ durability 的唯一修復路徑） |
| `replay_bundle/i074_preflight.py`（**新增**） | pre 四項／post 兩項；⚠️ **台北日期**換算（末根 epoch 的 UTC 日期是 08-31） |
| `replay_bundle/crossday.py`（**新增**） | 輸入有效性（擋假跨日）、argv 正規化、`outcome` 真值表、**來源綁定 validator**、獨立 CLI ＋ exit 5 控制流 |
| `replay_bundle/probe.py`（**新增**） | 三份 artifact 的封閉 schema 與欄位間不變條件（`row_count == len(rows) == sum(quota) == 200`、量測⛔ 不得寫 0 佔位） |
| `replay_bundle/evidence.py`（**新增**） | manifest、**六項全圖驗證**、staging ＋ 整包 `rename_noreplace`、durability 三段分流、`--recover-durability`（**依 crossday `outcome` 還原終端結果**）、CLI |
| `evaluation.py` | `--i074-preflight`／`--i074-capacity-probe` 兩個 opt-in、stage 限定與互斥、`_replay_from_bundle()` 的 `quota_override`／`on_context` |
| `python/scripts/_i074_bootstrap.py`（**新增**） | host 端的最小 package context（⚠️ 兩支 host 腳本共用，⛔ 不各寫一份） |
| `python/scripts/validate-i074-run-identity.py`（**新增**） | host 端守門；支援 `.json`／`.json.gz`；**可選 `--bundle`**（自己跑 `load_bundle()` 取 ID） |
| `python/scripts/ensure-i074-run-identity.py`（**新增**） | identity 的建立／`--peek`／`--print-path`；⚠️ bundle_id 一律經 `load_bundle()` 取得 |
| `python/scripts/write-i074-probe-measurement.py`（**新增**） | 把 shell 量到的 peak／host low／cgroup limit 寫成 measurement ＋ completion；⚠️ provenance **沿用 computation 的**（三份必須來自同一次執行），⛔ shell 不自己組 JSON |
| `scripts/pin-replay-image.sh`（**新增**） | 唯一 build／pin 入口；stdout 只印 image ID；⚠️ **`--no-identity`** 供非 I-074 的一般 Stage 1／2 用 |
| `scripts/compare-replay-crossday.sh`（**新增**） | same-path 唯讀掛載 ＋ 注入 `--run-identity`；以 image ID 執行；exit 5 原樣傳出 |
| `scripts/finalize-evidence.sh`（**新增**） | normal／recovery 兩模式的 CLI matrix；recovery ⛔ 不掛載外部 identity |
| `scripts/run-i074-stage1.sh`（**新增**） | orchestrator：捕捉 crossday rc → 只接受 0／5 → 跑 finalizer → **rc 3 優先於原始 0／5** |
| `scripts/run-replay-offline.sh` | `REPLAY_IMAGE_ID` 釘死、**一律以 image ID 執行**（⛔ 不用 tag）、I-074 模式的 identity 守門、**`MEASURE_PEAK=1` 的 peak 落檔**（⚠️ 前景執行以保留即時串流；⛔ 讀不到即 fail-closed，不寫 0 佔位、也不寫 completion） |
| `scripts/test-replay-args.sh` | 新增 12-B：host validator 的 7 條（含 `python3 -S` ＋ `--bundle`） |

**新增測試**（⚠️ 數字是**歷次 review 修正後的最終值**）：`test_run_identity.py`（29）、
`test_crossday.py`（43）、`test_i074_preflight.py`（32）、
**`test_replay_evidence.py`**（25），共 **129 條**；`test-replay-args.sh` 的斷言由原本的
62 條增為 **103 條**（host validator、comparator／finalizer／Stage 1 的 argv 與 mount、
pin 四分支、orchestrator 的 0／5／3 仲裁）。

⚠️ **檔名是 `test_replay_evidence.py`，⛔ 不是 `test_evidence.py`**——後者是既有的
SHAP evidence regression（第一輪 review 發現它被覆寫後已還原，見下方修正表中 9）。

**新增 argv fixture**：`stage1_argv.json`／`comparator_argv.json`／`finalizer_argv.json`
（⚠️ 依十三-C 的 CLI matrix 建立，⛔ 不是由實作者自選 argv 再凍結；動態值用 placeholder，
shell 測試把實際輸出正規化後**逐 token 比對**）。

**驗證**（⚠️ 下表是 **2026-09-15 初版實作當時**的結果；歷次 review 修正後的最終值見
本節末的「最終驗證」）：

| 層 | 結果 |
|---|---|
| `python/scripts/test.sh` | **1215 passed, 1 skipped**（v17 時是 1090） |
| `scripts/test-replay-args.sh` | 全過（由 `python/scripts/test.sh` 自動先跑） |
| `scripts/smoke-replay-offline.sh` | 全過，`after rows=35 cohort=0 comparison=0` |

⚠️ **實作過程中的實測佐證**（都寫進了對應模組的註解）：

* `python3 -S`（site-packages ⛔ 不在 `sys.path`）下
  `canonical → publish → calendar → artifacts → bundle → run_identity` **全鏈載入並跑完
  `load_bundle()`**——dependency-light 契約成立；
* ⚠️ **`replay_bundle/calendar.py` 與標準庫的 `calendar` 同名**，bootstrap 必須用前綴註冊；
* host validator 的五種情境全對（含「失敗時 stdout ⛔ 無輸出」）；
* 末根 epoch `1788192000` → 台北 `2026-09-01`（UTC 是 08-31）。

⚠️ **一條測試紅了但成因是 fixture**：用 `"1"*64` 當 SHA 時，純數字的 `.upper()` 不會變，
所以「⛔ 大寫 hex」那條檢查測不到。改成含字母的 SHA 後通過——**實作本身沒有問題**。

⚠️ **12-C（recovery 的六欄執行身分 tamper）由 pytest 涵蓋**（`test_replay_evidence.py` 的
`test_recovery_rejects_execution_identity_drift`），⛔ 不在 shell 層重複一份：
它要斷言的是「**在 `fsync_dir` 被呼叫之前**中止」，那要 spy 才驗得到。
⚠️ 該測試**改的是本次 recovery 這一側**、archived evidence 一個位元都不動——
⛔ 改 archived 端會先被 manifest／image 一致性守門攔下，就證明不了這個分支生效。

##### 第一輪 review 的修正（2026-09-15）

| # | 問題 | 修正 |
|---|---|---|
| 高 1 | **recovery 固定回 0**——「crossday mismatch（5）→ parent fsync 失敗（3 蓋掉 5）→ recovery 回 0」會讓 **mismatch 的 5 永遠消失** | `main()` 依 `outcome` 還原：`MATCH` → 0、其餘 → 5；⚠️ normal finalization 仍回 0（那時 crossday 的 rc 在 orchestrator 手上） |
| 高 2 | **normal finalizer 完全沒用外部 identity**——host 驗 A、Python 只信 archived 的 B，兩者從沒比過 | `run_evidence()` 載入外部那份，與要歸檔的逐欄比對 |
| 高 3 | **cohort 只比 SHA**，沒走完整 validator | 補 `validate_cohort_manifest()`、provenance 驗證，以及「cohort keys ＝ after 的候選列」 |
| 高 4 | **durable validator 沒驗「有效跨日」**——finalizer／recovery 只呼叫 `validate_crossday()`，重新封裝一套內部自洽的來源就能放行同一天 | `validate_crossday()` 開頭自己呼叫 `assert_valid_crossday_inputs()` |
| 高 5 | **shell 測試失敗仍回 0**——`$fails` 只在 I-100 段落結束處檢查一次，新增的 I-074 測試在那之後 | 結尾再檢查一次 |
| 中 6 | manifest 的 `bundle_id` 沒綁 archived identity；`generated_at` 沒驗時區 | 兩者都補 |
| 中 7 | identity no-op 的 **file** fsync 失敗回 exit 1（parent 才回 3） | 兩者都轉成 `DurabilityUnconfirmed`；測試也從 `(DurabilityUnconfirmed, OSError)` 收緊 |
| 中 8 | **重新引入路徑覆寫後門**——三支 shell 的 `I074_IDENTITY` 與 `ensure` 的 `--path`，等於把 v25 移除的 `I074_RUN_IDENTITY` 換名字加回來 | 全部移除，測試改覆寫 `XDG_DATA_HOME` |
| **中 9** | ⚠️ **覆寫了既有的 `tests/test_evidence.py`**（原本 7 條 SHAP additivity／降級／top-N regression） | `git checkout` 還原，新測試改名 `test_replay_evidence.py` |
| 低 10 | capacity probe 的 `trap` 覆蓋了 worktree cleanup，每次 probe 都可能留下 detached worktree | `cleanup_peak()` 串接原本的 `cleanup` |
| 低 11 | 兩個 I-074 flag 重複不會被拒絕（argparse 的 `store_true` 靜默接受） | 新增 `_reject_duplicate_i074_flags()` |

⚠️ **中 9 值得單獨記**：它是用 `cat >` 覆寫既有檔案造成的，而**完整測試套件仍然「通過」**
——被刪掉的 7 條被新增的 16 條取代，總數還是增加，所以沒有任何訊號。
⛔ **教訓：新增測試檔前要先確認該路徑是不是既有檔案**（`git status` 顯示 `M` 而非 `??` 就是警訊）。

##### 第二輪 review 的修正（2026-09-16）

| # | 問題 | 修正 |
|---|---|---|
| 高 1 | **orchestrator 沒把本次 comparator 的輸入／輸出綁到 finalizer**——可以「比較本次 D／D+1、卻封存另一組合法但**舊**的產物」，最終 exit code 與 evidence 包⛔ 不是同一次比較 | D／D+1／crossday 三份由 orchestrator **自己綁定**，使用者以 `--source` 覆寫即拒絕 |
| 高 2 | **全圖驗證用 `.get("provenance")` 推測欄位位置**，整份漏掉 crossday 的 `comparator_provenance`；manifest 的 `finalizer_provenance` 也沒驗 image | 新增 `PROVENANCE_LOCATIONS` **逐一列舉**（含 role），manifest validator 另驗 finalizer 的 image |
| 中 3 | 外部 identity 比對後，archived source **又被讀了一次**（兩次讀取之間可被替換） | 比對點移進 `finalize_evidence()`，用**實際要封存的那一份** |
| 中 4 | phase-A 的 config 載入是 **incidental import**，⛔ 不是可執行保證 | 階段 A **明確 `import config`** 並以 `config_module=` 傳入 builder |
| 中 5 | shell 測試排在 build **之前**——乾淨環境第一次執行會**整段跳過**；`finalizer_argv.json` 沒人讀取；orchestrator 仲裁沒測；comparator 的 abbreviation 測試是 **false pass**（缺參數也會 SystemExit） | 測試移到 build 之後；新增 finalizer argv／模式衝突 3 條、orchestrator 仲裁與綁定 10 條；abbreviation 改成「完整合法 argv 只換縮寫」 |
| 低 6 | v25 標題仍寫「待確認」；`sr-zone-scoring.md`／`development-workflow.md` 未補 Stage 1 程序 | 標題更正；補「I-074 Stage 1 的正式執行程序」（六步 ＋ 結束碼表）與「四個永久契約」 |

⚠️ **中 5 的閉環一補上就抓到一個真實 bug**：`finalize-evidence.sh` 沿用了
`replay_args_reject_injected`，而 **`--source` 正好是 `--source-root` 的前綴**——
於是 finalizer 的 `--source` **完全不能用**。⚠️ 那與 `--run-identity` vs `--run-id`
是**同一類錯**，而且因為 finalizer 從未被端到端執行過，⛔ 之前沒有任何測試抓得到。
修正：finalizer 自己做**精確比對**的所有權檢查，⛔ 不共用前綴清單。

##### 第三輪 review 的修正（2026-09-16）

| # | 問題 | 修正 |
|---|---|---|
| 中 1 | **orchestrator 太晚拒絕覆寫**——先跑 comparator、之後才檢查。錯誤呼叫會**先產出 crossday artifact**；comparator 若回 5，流程又因覆寫回 1 → **跳過 finalizer**，⛔ 掩蓋 mismatch，直接違反「mismatch 必須封存」 | 檢查移到 comparator **之前**（步驟 ⓪），並補測試斷言 **comparator 完全未被呼叫** |
| 中 2 | shell 測試硬編 `stock-trading-python-test:latest`——用 `PY_IMAGE=…` 時剛建好的 image ⛔ 不會被採用，預設 tag 不存在就整段 skip | 統一 `${PY_IMAGE:-…}`；`test.sh` 傳 **`IMAGE_REQUIRED=1`**，那時找不到 image ⛔ **一律 fail**、不得 skip |
| 中 3 | 測試閉環只完成一部分：`recovery_argv` 沒人讀、pin 三分支沒測、本輪修正沒有 regression | 補 **recovery_argv** 逐 token 比對、**pin 四分支**（含 stdout contract 與 `--no-identity`）、以及四條 pytest regression（comparator／finalizer 的 provenance image、**archived identity 只載入一次**、**builder 收到 `config_module`**） |
| 低 4 | 操作文件的 pin 步驟只印 ID，後續步驟卻沒設 `REPLAY_IMAGE_ID`（兩支腳本都強制要求） | 第一步改成 `export REPLAY_IMAGE_ID="$(scripts/pin-replay-image.sh <bundle>)"`，後面五個角色自然沿用 |

⚠️ **中 3 的 regression 要能抓到「先比對、再重讀」的舊實作**，所以
`test_archived_identity_is_loaded_exactly_once` 用**計數**斷言，⛔ 不是只比兩份內容不同
——後者在新舊實作下都會通過。

##### 第四輪 review 的修正（2026-09-16）

| # | 問題 | 修正 |
|---|---|---|
| 中 1 | **「只載入一次」測試仍是假綠燈**——它直接呼叫 `finalize_evidence()` 且 `external_identity=None`，**完全繞過** `run_evidence()` 那段（舊實作的多餘讀取正發生在那裡），新舊實作都只會計數一次 | 改走 **`run_evidence(argv)`**，external 用**另一個內容相同**的檔案，同時計數 `load_run_identity` 與 `_load_all` 對 archived 的載入 |
| 中 2 | **`export VAR="$(cmd)"` 會掩蓋非零結束碼**——pin 回 3（durability 未確認）時整行仍是 0，操作者會以為成功；且環境變數⛔ 不跨 session，D+1 隔天開新 shell 會缺 `REPLAY_IMAGE_ID` | 文件拆成兩行（`VAR="$(…)"` ＋ `export VAR`）；D+1 明訂**重新執行那兩行**（既有 identity 會走 no-op，取得同一個 ID） |
| 低 3 | 兩個宣稱沒被斷言：`--no-identity` 沒驗「真的有 build」（誤刪 build 也會過）；finalizer 測試從 `python` token 才比 argv，**Docker mounts 全被丟掉** | `--no-identity` 加 build log 斷言；補「normal 的 identity 是 same-path `:ro`」與「**recovery ⛔ 未掛載外部 identity**」 |

⚠️ **中 1 的修正做了反向驗證**（2026-09-16）：把舊實作注回 `run_evidence()` 後，該測試以
「archived identity 被載入 **2** 次（`['run_identity_load', 'evidence_load']`）」紅掉；
還原後通過。⛔ 沒有這一步，「測試能抓到舊實作」就只是宣稱。

⚠️ **`export VAR="$(cmd)"` 的實測**：`export V="$(exit 3)"` → **rc=0**；
拆開寫 `V="$(exit 3)"` → rc=3。

##### 第五輪 review 的修正（2026-09-16）

| # | 問題 | 修正 |
|---|---|---|
| 中 1 | **「拆成兩行」本身還不夠**——貼進**沒有 `set -e`** 的 shell 時，第一行回 3 之後第二行照樣執行，整段仍以 `export` 的 0 結束，durability code ⛔ 還是被掩蓋 | 文件改用 `if REPLAY_IMAGE_ID="$(…)"; then export …; else 報錯並停 fi`（⚠️ 互動 shell 也安全），或在腳本裡先 `set -e`。**實測**：`if V="$(exit 3)"` 能正確走 else 並拿到 rc=3 |
| 中 1-b（第六輪） | **`else` 只印訊息 → 整個 `if` 仍回 0**（`echo` 是成功的指令），流程照樣繼續、原始的 3 也不再是終端碼 | else 捕捉 `pin_rc=$?` 並 **`exit "$pin_rc"`**；文件標明正式流程要寫成**帶 `set -euo pipefail` 的腳本**，互動 shell 則是**人工停止點**（⛔ 不宣稱它會自動停）。實測：`if V="$(exit 3)"; then :; else echo x; fi` → **rc=0** |
| 低 2 | **recovery 的負向斷言只比一個精確字串** `$FIN_ID:$FIN_ID:ro`——回歸成 **RW mount** 或掛到**另一個 container path** 時仍會通過，而那時 recovery 已經依賴外部 identity | 改成斷言**整份指令完全不含該路徑**（`grep -qF`），fixture 另外確保 python argv 沒有 `--run-identity` |

⛔ **本輪尚未完成**：**正式的 D／D+1 兩趟 replay**——那本來就是下一步，
且依已確認的計次裁決合為一次。⛔ 本筆在那之前不得關閉。

##### 第七輪起的 review 修正（2026-09-16）

| 輪 | 問題 | 修正 |
|---|---|---|
| 七（1 低） | D+1 步驟仍寫「重新執行①的**那兩行**」，但①已改成完整的 guarded `if/else/exit` 區塊——照字面只重跑 assignment ＋ export 會**把剛修掉的結束碼問題帶回來** | 改成「重新執行步驟①的**完整 guarded pin 區塊**」，並註明⛔ 不是只重跑兩行 |
| 八（2 中 1 低） | 文件狀態沒跟上事實：I-100 仍寫「待正式 bundle／Stage 1 跑不動」、I-074 的四階段總表仍寫 Stage 0「還缺診斷欄位與 bundle」、Stage 1 實作結果仍寫「待再次 review」且測試檔名是舊的 `test_evidence.py` | 三處全部對齊現況；測試數字由第一輪基準更新為最終值 |
| 九（1 中 3 低） | 跨日策略殘留「D 日產 bundle」（兩處）；review 輪次與表數不符；未執行警語重複；驗證數字停在初版 | 本表所列 |
| 十（1 中 2 低） | 正式測試入口出現**偶發失敗**：`test-replay-args.sh` 的 comparator identity `:ro` 掛載斷言在完整 `python/scripts/test.sh` 中失敗、單獨重跑又全過；review 輪次仍有四種說法；`**……共 **8 輪** review……**` 巢狀粗體 | 補失敗時的診斷輸出並修掉調查中發現的兩個缺陷（⚠️ **根因仍未證實**，見下）；拿掉所有輪數摘要數字；解掉巢狀粗體 |
| 十一（2 中） | 第十輪把根因寫成定論，但當時是「argv 通過、只有 identity mount 紅」，與該推論矛盾（argv 比對不涵蓋 mount）；`replay_args_abs_path()` 只驗父目錄，檔名打錯仍會走到 `docker -v` | 文件改記成「調查時另外發現的兩個缺陷」並標明根因未證實；由各 caller 在 Docker 前以 `-f` 驗證必要輸入檔 |

⚠️ **第九輪順帶抓到一個真實 bug**：smoke 在**工作區乾淨時會失敗**——
`git diff --binary HEAD` 產出空 patch，而 `git apply` 對空輸入報
`error: unrecognized input`。⚠️ 那不是失敗，是「這一輪沒有 tooling 變更」。
⛔ **之前沒發現，是因為工作區一直有未 commit 的改動**；commit 之後才暴露。
`run-replay-offline.sh` 本來就用 `if [ -n "$patch_file" ]` 處理這件事（空 patch 時
`tooling_patch_sha256` 是空字串的 SHA-256 `e3b0c442…`），**smoke 現在與它一致**。

⚠️ **第十輪的偶發失敗：根因⛔ 尚未證實，⛔ 不得標成「已修」**。

當時的實際輸出是「comparator argv 通過、D／D1 mount 通過、**只有 identity mount 失敗**」，
而那一輪的輸出**沒有被保存**。⛔ 不能宣稱已證實是記憶體競爭或 ensure 失敗所致：

* 一度推論「`ensure` 失敗 → `CMP_OUT` 為空 → 下游整排失敗」，但**那個推論是錯的**——
  argv 比對只取 `sed -n '/^python$/,$p'`，而 `-v` 掛載排在 `python` **之前**
  （實測：identity mount 在第 22 行、`python` 在第 28 行），所以 argv 比對**根本不涵蓋
  mount**。「argv 通過、單一 mount 紅」因此完全可以並存，⛔ 不能用來反推 `CMP_OUT` 為空。
* 有效路徑下，新 helper 與舊算法的結果**逐字相同**，mount 比對本身也沒有改動。

#### ✅ 根因已定位並修正（2026-09-18）：`printf | grep -q` 在 `pipefail` 下的 SIGPIPE 競爭

⚠️ 這條斷言先後非決定性失敗**兩次**（第一次 comparator、第二次 finalizer，都是 identity
那一份、都在完整 `python/scripts/test.sh` 裡、單獨重跑都通過）。根因是 **shell 寫法**，
⛔ 與 identity、docker、記憶體壓力**全都無關**：

```bash id="i074_sigpipe_race_001"
# ⛔ 舊寫法
if printf '%s\n' "$CMP_OUT" | grep -qx -- "$target:$target:ro"; then
```

腳本開頭是 `set -euo pipefail`。**`grep -q` 一找到匹配就立刻退出**，此時 `printf` 若還沒把
剩下的輸出寫完就會收到 **SIGPIPE**，於是 pipeline 的結束碼變成非零——
⚠️ **即使字串明明匹配成功**，`if` 仍走 else 分支。

⚠️ **這就是它「非決定性」的來源**：觸發與否取決於 `printf` 寫入與 `grep` 退出的時序，
輸出愈大愈容易撞上。也解釋了先前所有觀察：
argv 逐 token 比對用的是 `[ "$A" = "$B" ]`（⛔ 沒有 pipe）所以從不失敗；
單獨重跑時序不同所以通過；診斷印出來的「實際 mount」永遠是對的——**因為它本來就是對的**。

**修正**：改用 here-string，⛔ 不經 pipe。

```bash id="i074_sigpipe_fix_001"
# ✅ 新寫法
if grep -qx -- "$target:$target:ro" <<< "$CMP_OUT"; then
```

**反向驗證**（`scripts/test-doc-refs.sh`，同型寫法）：

| 版本 | 20 次執行的失敗次數 |
|---|---|
| `printf \| grep -q`（舊） | **4 次**（20%） |
| `grep -q <<<`（新） | **0 次** |

⚠️ **⛔ 這不只是測試的問題**——任何 `set -o pipefail` 的腳本裡，
`大量輸出 | grep -q` 都有同樣的競爭。`scripts/test-replay-args.sh` 已全部改掉：**6 處斷言**（2026-09-21 補上漏網的
`--i074-preflight` 那條）、**3 處 fail 分支的診斷輸出**，以及 **2 處 `| head -1`**
——⚠️ `head` 同樣**會提早退出**，⛔ 不要只盯著 `grep -q`。
剩下的 `sed`／`awk`／`wc` 都會讀完輸入，⛔ 不具這個風險。

##### 調查時另外發現並修正的兩個缺陷（⚠️ 與上述偶發失敗的因果**未經證實**）

1. **測試吞掉了前置步驟的失敗**。comparator／Stage 1／finalizer 三段在跑被測腳本前，
   都會先呼叫 `ensure-i074-run-identity.py` 建立 run identity，而那三個呼叫**都寫成
   `>/dev/null 2>&1` 且不檢查 rc**。它一失敗，identity 就不存在、被測腳本整支 `exit 1`，
   而真正的錯誤訊息完全看不到。
   → 改用 `ensure_identity()`：檢查 rc、印出 stderr、明說「以下斷言的失敗都源自這裡」，
   並額外確認 identity 檔真的產出來了。
2. **路徑轉絕對值會靜默退化**。`$(cd "$(dirname "$p")" && pwd)/$(basename "$p")` 在目錄
   不存在時，command substitution 只是回空字串，於是算出 `/run_identity.json` 這種
   **看起來完全合法**的絕對路徑，一路傳進 `-v` 掛載與容器 argv。
   → 新增 `replay_args_abs_path()`（`scripts/lib/replay-args.sh`）改為硬失敗，
   `compare-replay-crossday.sh` 與 `finalize-evidence.sh` 全面改用它。

⚠️ **兩項都做了反向驗證**：把 `replay_args_abs_path()` 改回舊實作後，
`--d` 指向不存在的目錄竟然 **rc=0 且照樣印出 docker 指令**（證實會掛 `/d.json`）；
把 comparator 的 bundle 換成壞路徑後，輸出第一行就是根因＋traceback，
後續失敗被明確標註來源。

##### 最終驗證（2026-09-16，歷次 review 修正後）

| 層 | 結果 |
|---|---|
| `python/scripts/test.sh` | **1219 passed, 1 skipped** |
| `scripts/test-replay-args.sh` | **103 條斷言**全過（由 `python/scripts/test.sh` 以 `IMAGE_REQUIRED=1` 自動先跑） |
| `scripts/smoke-replay-offline.sh` | 全過，`after rows=35 cohort=0 comparison=0`（⚠️ **工作區乾淨時亦然**——空 patch 的處理見上） |

##### ✅ Stage 1 正式執行結果（2026-09-17～18）

| 步驟 | 時間 | 結果 |
|---|---|---|
| ① pin | 2026-09-17 | image `sha256:d66030dca485…` 釘死，identity 建立 |
| ② probe | 2026-09-17 | 200 列／165 秒；⚠️ 峰值 265 MiB **但那個數字⛔ 不能外推**（見下方 probe 缺陷） |
| ③ D（第一次） | 2026-09-17 | ⛔ **OOM 失敗**，183 分鐘後 SIGKILL、零產出（見下節） |
| ③ D（重跑） | 2026-09-17 16:42→19:42 | ✅ **180 分鐘跑完**，13,417 列、候選 **156** |
| ④ D+1 | 2026-09-18 09:16→12:18 | ✅ **182 分鐘**，13,417 列、候選 **156** |
| ⑤ 仲裁 | 2026-09-18 | ✅ **rc=0 `MATCH`** |

**跨日比對結果**：

```text id="i074_stage1_match_001"
outcome                 MATCH
d_only                  0 筆     ← D 有而 D+1 沒有的列
d1_only                 0 筆     ← 反向
provenance_differences  0 筆     ← 正規化 argv 的 --output-dir 後，10 欄逐欄無差異
```

⚠️ **候選集合也逐 key 相同**（⛔ 不只是數量相同）。

⚠️ **「10 欄逐欄相同」要講精確**（⛔ 不要簡化成「只有 generated_at 不同」）：

| 項目 | 狀態 |
|---|---|
| 頂層 `generated_at` | **不同**——⚠️ 它是 artifact 的**頂層欄位**，⛔ **不是** provenance 的一欄 |
| `argv` 裡的 `--output-dir` | **不同**（兩趟本來就要用不同的全新目錄） |
| **10 個 provenance 欄位** | ⚠️ **把 `argv` 的 `--output-dir` 正規化之後**，逐欄**無差異** |

⛔ 所以⛔ 不能說「provenance 完全相同」——`provenance_differences()` 是**先正規化
`--output-dir`**，其餘才全等；這正是它為什麼要有那段正規化邏輯。

**證據位置**：`python/baselines/i074_stage1/`，**10 檔 14 MB**（9 個 `.json.gz` ＋
`evidence_manifest.json`），整包一次 rename 發布。

##### ⛔ 分支 A 已排除——**必須跑 Stage 2**

候選數 **156 > 0**，且**跨日穩定重現**。所以：

* ⛔ **走不到「零命中即收斂成已知限制」**那條路；
* 需要 Stage 2（before 全掃）才分得出 **B**（如預期翻轉）或 **C**（不符預期／集合不一致）；
* Stage 2 的前置盤點與那個**謂詞不對稱**的設計題見下方「Stage 2 的前置盤點」。

##### ⚠️ comparator 也有容量問題（2026-09-18 實測，⛔ 尚未解決）

⑤ 的 comparator 要**同時載入 D 與 D+1 兩份 78 MB artifact**（`build_crossday()` 要算
`d_only`／`d1_only` 差集，兩邊的 rows 必須同時在場），實測**單份載入 +365 MiB、兩份約 730 MiB**，
而 mem-guard 在常態下只給得出 **531m**——⛔ 必然 OOM。

⚠️ 這次是**停掉全部 7 個常駐 container**（釋放約 490 MB，available 670→1158 MB）
再以 `MEM=900m` 跑過的，跑完立刻還原。⛔ **那是權宜之計，⛔ 不是解法**：

* ⚠️ Stage 2 會撞上**同一道牆**（它也要載入兩份全量 artifact 逐列比對）；
* ⛔ 停 live 服務換記憶體⛔ 不能變成常態程序；
* 真正的解法是讓比對走**串流**——⚠️ 但那與 Stage 1 的「串流寫出」是**不同的問題**
  （這次是**讀入**），⛔ 不能沿用同一個修法。

##### ③ 第一次執行：**OOM 失敗**（2026-09-17，⛔ 沒有產出任何 artifact）

| 事實 | 值 |
|---|---|
| 結束碼 | **137**（128+9 ＝ SIGKILL） |
| `dmesg` | `OOM killed process 1 (python) total-vm:1028184kB, **anon-rss:503752kB**` |
| 被殺時 RSS | **491.9 MiB** —— 撞上 mem-guard 給的 cgroup 上限 **504m** |
| 產出 | ⛔ **空的**——`$RUN_DIR/d/` 一個檔案都沒有 |
| 已耗時 | 約 2.5 小時（⚠️ 全部作廢） |

⚠️ **這是 container 內的 cgroup OOM**（`process 1 (python)`），⛔ 不是 host OOM killer
砍掉呼叫端——⚠️ 兩者的處置方向相反，⛔ 不要混為一談。

**根因（2026-09-17 實測定位並修正）：⛔ 不是 replay 過程累積，是最後一步整份 JSON 一次成形。**

⛔ **本節初稿把成因寫成「`_replay_from_bundle()` 把每列累積在記憶體、峰值隨列數線性成長」，
那是錯的**，並據此外推「13,417 列需要約 1.07 GB」——兩者都被實測推翻：

| 證據 | 內容 |
|---|---|
| D 那趟跑了 **183 分鐘**才 OOM | 全量預估 185 分鐘——**幾乎跑完了才爆**，⛔ 不是中途 |
| 過程中 RSS **穩定** | 另跑一趟量曲線（`MEM=350m`、29 分鐘、~2,100 列），RSS 一路在 **190～205 MiB**，斜率為負 |
| 尾段序列化的實測增量 | 13,417 列（每列實測 6,898 bytes）→ JSON 88.3 MiB，而 `canonical_json_bytes()` 會讓 **str 與 bytes 兩份完整拷貝同時存在** |

**獨立行程量測（同一份資料、同一個 SHA）**：

| 寫法 | baseline | 峰值 | 序列化增量 |
|---|---|---|---|
| `canonical_json_bytes()`（舊） | 47 MiB | 398 MiB | **+351 MiB** |
| `canonical_json_chunks()` 串流（新） | 47 MiB | 49 MiB | **+2 MiB** |

⚠️ 對得起來：基礎 RSS 195 ＋ 351 ＝ **546 MiB**，而 mem-guard 給的上限是 **504m**
——必然 OOM，被殺時的 492 MiB 正是爬升途中。修正後是 195 ＋ 2 ＝ **197 MiB**。

**⛔ probe 為什麼測不到**：`PROBE_QUOTA` 固定 200，而峰值來自**尾段序列化**、
規模隨列數走。200 列的序列化增量只有約 5 MiB，⛔ 完全反映不出 13,417 列的 351 MiB。
⚠️ 它叫 capacity probe，卻回答不了「全量跑不跑得動」——**這個缺陷仍然存在**，
⛔ 修好 OOM ⛔ 不等於修好 probe。

#### 修正（2026-09-17）

| 檔案 | 改動 |
|---|---|
| `replay_bundle/canonical.py` | 新增 `canonical_json_chunks()`——用 `JSONEncoder.iterencode()` 分段吐出，⛔ 不讓整份 JSON 一次成形。⚠️ 與 `canonical_json_bytes()` **共用 `_ENCODER_KWARGS`**，⛔ 不各寫一份 |
| `replay_bundle/artifacts.py` | 新增 `write_canonical_atomic()`：串流寫入 ＋ `hashlib` 增量算 SHA ＋ 64 KiB 緩衝（19.6 萬個片段逐片 write 會讓系統呼叫爆掉）。`publish_artifacts()` 改走它 |
| `evaluation.py` Stage 1 | `after_sha` 改由串流寫入回傳——⛔ 不再「先 `canonical_json_bytes()` 算 SHA、`publish` 時再序列化一次」 |
| `evaluation.py` Stage 2 | 同樣改串流；原本「序列化兩次比 hash」的雙重確認改成**讀回磁碟重算**（`sha256_file`），驗的是真正落地的 bytes，比原本更強，且排在 `report.json` **之前** |

⚠️ **bytes 逐字相同是整套契約的根**，由三支測試釘住（含 NaN／Inf 兩條路徑都要擋、
temp 檔不得殘留）。**反向驗證**：把 chunks 的 `sort_keys` 改掉 → byte-identity 測試變紅、
並連帶抓到 Stage 2 的 SHA 不符；把 cohort manifest 改成先發布 → 順序測試變紅。

⚠️ 順序契約的測試也改了觀察點：原本 spy `write_atomic`，現在 spy **`os.replace`**——
after artifact 走 `write_canonical_atomic()`、manifest 走 `publish_artifacts()`，
但兩條路徑最後都以 `os.replace` 原子發布。盯發布動作本身，換實作也不會讓守衛失效。

⛔ **本筆在容量問題解決前無法往下走**——④⑤ 都建立在「③ 有 artifact」之上。
⚠️ **這段是 2026-09-17 OOM 當下的判斷**；容量問題已於同日修正，
**D／D+1／⑤ 已於 2026-09-18 全部完成**，⛔ 不要再依本段暫停 ③。

###### ⚠️ 這一次⛔ 不適用「D 之後不得 commit」的凍結窗口

⛔ **否則會形成死結**：操作文件要求 D 跑完到仲裁結束之間⛔ 不得 commit（`base_commit`
一漂跨日就判 mismatch），但這次**根本沒有 artifact 可以仲裁**，而要解決 OOM
**一定得改程式**。兩條規則同時成立就卡死了。明訂如下：

| 條款 | 內容 |
|---|---|
| **無 artifact 的 OOM ⛔ 不算一次正式 scan** | 「只執行一次有界定向驗證」的計次⛔ 不因本次消耗——⚠️ 它連 artifact 都沒產出，⛔ 沒有任何可判讀的結果 |
| **凍結窗口解除** | 本次 D 作廢，⛔ 沒有任何東西需要與它對齊；修復期間**可以正常 commit** |
| **重新開始** | 容量問題修好後，D／D+1 **兩趟都要重跑**，凍結窗口從**新的 D** 開始重新計算 |
| ~~**步驟 ③ 暫停**~~ | ⚠️ **2026-09-18 解除**——OOM 已修、D／D+1 已跑完；此列保留為當時的處置紀錄 |

#### I-074 Stage 2 計畫書 v25（2026-09-22，✅ **已確認**）

✅ **使用者於 2026-09-22 確認 v25 並指示執行 ②**（見「② 執行結果」）。
⚠️ 後續步驟仍依「八、執行順序」逐步進行，⛔ 未經確認⛔ 不得跳步。

⚠️ **v25 收掉兩項殘留**：

| # | 項目 | 處置 |
|---|---|---|
| 1 | ⛔ **後段「測試、風險與歸檔」仍要求 before／after candidate 集合等價**，且用 `ecbc141^` 的 before 展開式——⚠️ 它**沒有標成歷史**，⛔ 實作者可能照著新增錯誤測試 | ⚠️ 直接改寫成現行模型的四項驗證，⛔ 不是加註 |
| 2 | ⚠️ 正向矩陣寫「四種目標 ＋ 兩種對照」，但表只有一種對照，後面又另列四種非目標 | ⚠️ 併成**單一九列矩陣**，⛔ 避免實作時漏測 |

#### （承上）Stage 2 計畫書 v24 的缺口修補

⛔ **v24 修掉 guard 的目標案例分類——⚠️ 這是本計畫第三次犯同一個錯。**

⚠️ v23 的 guard 把案例分成「RR 不合格 ＋ 價格證據成立 ＝ 目標」，
⛔ **那個條件⛔ 不完整**——⚠️ **本文件「二、①」早就記過一次**：candidate 還需要
`event_signal == CLOSE_RECLAIM`、**高優先失敗分支未命中**
（`active_bearish_states`／`SUPPORT_RECLAIM_INVALIDATED`／`BREAKDOWN`）。

⛔ **後果是實質的**：一筆「價格證據成立 ＋ RR 不合格、但被 `BREAKDOWN` 高優先分支吃掉」的列
會被錯分成**目標案例**，allowlist 隨即允許它的 lifecycle／market／action 改變
——⚠️ **於是一個破壞 lifecycle 優先序的錯誤 patch 也可能通過 guard。**

✅ **v24 的裁決：目標案例⛔ 不再重組近似 predicate，直接用權威欄位**：

```text id="i074_guard_target_def_001"
目標案例 ＝ **原始 e1cbbbd（formal after）** 的
           semantic_pipeline.rr_decoupling_candidate == true
非目標案例 ＝ 同一欄位為 false
```

⚠️ 這與「Stage 2 ⛔ 不得重算 predicate」是同一條紀律——⛔ **只消費、⛔ 不重組**。

#### （承上）Stage 2 計畫書 v23 的缺口修補

⚠️ **v23 修掉 v22 review 的兩個測試／值域缺口與一項文件清理。**

| # | 項目 | v23 的處置 |
|---|---|---|
| 1 | ⛔ **semantic reason code 的穩定值寫錯**——`RR_NOT_QUALIFIED` 只是 `decision_engine.py:1029` 的 `_decision_semantic_pipeline()` 內的 fallback，`_rr_gate()` 實際產出的是 `RR_INSUFFICIENT`／`RR_UNAVAILABLE`／`NO_PRIMARY_ZONE`／`RR_QUALIFIED` | ⚠️ 契約改成**「移除 `PRICE_UPSIDE_FOLLOW_THROUGH` 後逐項相同（含順序）」**，⛔ **不硬編任何 RR code** |
| 2 | ⚠️ differential guard **缺負向防竄改測試**——只驗「實際 patch 跑得過」⛔ 證明不了 guard 會擋 | ⚠️ 補**表格驅動**的正負向矩陣（四種目標形狀 ＋ 六類 tamper），見「六、3」 |
| 3 | ⚠️ 兩處現行模型交叉引用仍指向舊版號 | ⚠️ 改成**不帶版號**的指向——⛔ 每次改版都要回頭修版號本身就是會再壞的設計 |

⚠️ **v23 另有一項實測發現（⛔ 影響 guard 的可驗證範圍）**：掃 156 筆候選的 row 層
`rr_gate.reason_code`，結果是 **`RR_UNAVAILABLE` 106／`RR_INSUFFICIENT` 24／
`EXECUTION_RR_INSUFFICIENT` 14／`RR_QUALIFIED` 12**——⚠️ **出現 `EXECUTION_*` 與
`qualified=True`，證明 row 裡那顆是 execution gate**（`:2779` 覆寫後的），
⛔ **不是 semantic pipeline 用的 setup gate**。
⚠️ 所以 **artifact 上⛔ 無法回推當初 append 的是哪一個 RR code**——
⛔ 這正是「⛔ 不得硬編單一 code、改用逐項相同」的第二個理由。

#### （承上）Stage 2 計畫書 v22 的缺口修補

⚠️ **v22 修掉 v21 review 的一個阻擋點與三項文件契約缺口。**

| # | 項目 | v22 的處置 |
|---|---|---|
| 1 | ⛔ **guard allowlist 漏三個必然改變的欄位**——`semantic_pipeline` 的 `rr_decoupling_candidate`／`bias_state`／`reason_codes`（`decision_engine.py:1117` 的 `_decision_semantic_pipeline()` 實際輸出），⚠️ **依 v21 規則合法 patch 仍會被判失敗** | ⚠️ 三欄補進 allowlist 並逐欄寫期望轉換；⚠️ **另把 guard 的方向寫死**：`patched e1cbbbd`（formal before）→ `原始 e1cbbbd`（formal after） |
| 2 | ⚠️ AVOID／非 AVOID 的**分類來源**還用 `market_action`——⛔ 它⛔ 不在 replay row 裡 | ⚠️ 改用 `after.action_state`（`AVOID`／`HOLD`）當唯一 classifier，交叉斷言 `position_action_condition.state` |
| 3 | ⚠️ failed-attempt record 的**重跑守門時機**未定義 | ⚠️ 補「**replay 之前**查同 run identity ＋ 同 counterfactual SHA 的失敗紀錄、命中即中止」與「record 自身發布失敗的 recovery 由 ③ 定義」 |
| 4 | ⚠️ 兩處 provenance 舊措辭 ＋ 兩處 Markdown 表格錯位（三欄內容塞進兩欄、孤兒表格列） | ⚠️ 全部改成 **Stage 2 evidence manifest** 並修好表格 |

⚠️ **`reason_codes` 的期望轉換是查證來的，⛔ 不是推測**：
`lifecycle_engine.py:197` 的 `resolve_lifecycle()` **只在 `CONTINUATION` 分支**
append `PRICE_UPSIDE_FOLLOW_THROUGH`，
`CONFIRMED`／`TESTING` 分支⛔ 不 append；⚠️ 而 `RR_NOT_QUALIFIED`／`MARKET_ACTION_AVOID`
兩側條件相同⛔ 不會變——**所以差異恰好只有那一個 code**。

#### （承上）Stage 2 計畫書 v21 的缺口修補

⚠️ **v21 收掉 v20 review 的兩個阻擋點、兩個契約缺口與五項文件殘留。**
⚠️ **本輪多了一項實測**：直接掃過已封存的 D+1 artifact（13,417 列全掃，串流讀取），
確認 156 筆候選在 **after 側**的實際分佈——⚠️ 這讓判讀矩陣⛔ 不再是推導，而是對帳：

| 筆數 | `market_bias` | `action_state` | `position_action_condition.state` | `final_entry_state` | `lifecycle_phase` |
|---|---|---|---|---|---|
| **140** | `BULLISH_CONTINUATION` | `HOLD` | `HOLD` | `BLOCKED` | `CONTINUATION` |
| **16** | `BEARISH_BIAS` | `AVOID` | `AVOID` | `BLOCKED` | `CONTINUATION` |

| # | 阻擋點／缺口 | v21 的處置 |
|---|---|---|
| 1 | ⛔ **決策樹仍把 `market_state` 當共同必要條件，且無條件要求 `market_bias` 翻成 `BULLISH_CONTINUATION`**——⚠️ 那會**誤殺上表那 16 筆 AVOID 候選** | ⚠️ 後段收斂成**唯一的 artifact 判讀矩陣**（分 AVOID／非 AVOID 兩類），見決策樹該節 |
| 2 | ⛔ **differential guard 的允許清單仍不完整**（漏 semantic `market_state`、`bias_label`、`bias_reason_codes`、`authority_reason_codes`、`position_gate_state`、`market_bias_label`），⚠️ 又**錯誤地允許** `position_action_condition.reason_codes` 改變 | ⚠️ 改成**精確 JSON path allowlist ＋ 逐欄期望轉換 ＋「未列出的決策欄位全部相同」**，見「六、3」 |
| 3 | ⚠️ flag 與 SHA **只有 shell 層 truth table，缺 Python 層成對守門** | ⚠️ 補 `i074_counterfactual ⇔ counterfactual_patch_sha256` 的五條 CLI 斷言，見「二、④」 |
| 4 | ⚠️ patch 失效的**事故紀錄沒有持久化契約**——與「⛔ 正式 archive 不存在」並存時無處可放 | ⚠️ 明訂 **failed-attempt record** 為步驟 ③ 的必備輸出，⛔ **不是成功 evidence archive**，見「二、①」 |

⚠️ **另清掉五項文件殘留**：`I074_MODE`「待裁定」的孤兒表格列、flag 與 SHA 的同步對象混寫、
兩處「counterfactual SHA 進 provenance」（應為 evidence manifest）、指向 v18／v19 的現行模型交叉引用。

#### （承上）Stage 2 計畫書 v20 的缺口修補

⚠️ **v20 修掉 v19 review 的四個實作阻擋點**（⛔ 三項是「計畫要求的東西在凍結產物／現行機制裡
做不到」，⛔ 不是措辭）：

| # | 阻擋點 | v20 的處置 |
|---|---|---|
| 1 | ⛔ **正式 artifact ⛔ 沒有 `market_state` 與 `entry_permission_state`**——凍結 row 只有 `market_bias`／`final_entry_state`／`lifecycle_phase`／`action_state`／`position_action_condition`——見 `evaluation.py:834` 的 `_decision_fields_from_summary()`，而 after artifact **已凍結、⛔ 不能補欄位** | ⚠️ B／C 判讀改用**實有欄位**，`market_state`／`entry_permission_state` 降為**推導含意**，見「二、①之二」 |
| 2 | ⚠️ 模式旗標與 patch 的**所有權／啟用關係尚未封閉** | ⚠️ 補 **truth table**：flag 是使用者 opt-in（⛔ 不屬 `SCRIPT_INJECTED_ARGS`）、`--counterfactual-patch-sha256` 由 runner 注入；✅ **`I074_MODE` 納入**，見「二、④」 |
| 3 | ⛔ **⛔ 不可用「中繼 commit」切分兩份 patch**——會動到 detached worktree 的 HEAD，與 `replay_args_prepare_worktree()`（`replay-args.sh:148`）的「HEAD 必須等於解析出的 OID」契約衝突 | ⚠️ 改用 **`git write-tree` 的中繼 tree**，HEAD 全程不動；⛔ 並撤回「交換順序合成 hash 必然不同」的錯誤假設，見「三之一之二」 |
| 4 | ⚠️ differential guard 的**允許差異集合太窄**——合法翻轉還會連帶改 `market_bias`——`decision_engine.py:1318` 的 `_market_bias()` 直接回傳 `bias_state`與 reason codes，而 `compare_rows()`（`artifacts.py:439`）**⛔ 不挑欄位比** | ⚠️ 改成明確的 **comparison projection ＋ lifecycle 下游依賴閉包**，見「六、3」 |

**⚠️ 一併採納的裁決建議**：`I074_MODE` **納入** counterfactual flag（否則正式反事實執行反而
繞過既有 run identity 守門）；⛔ **不採中繼 commit**、改用中繼 tree；
⚠️ **patch 失效不計正式 scan 成立，但⛔ 不得靜默重跑**——必須保存**當次的兩份 patch SHA、
有界診斷與事故紀錄**（見「二、①」計次列）。

#### （承上）Stage 2 計畫書 v19 的缺口修補

⚠️ **v19 補上 v18 review 抓到的兩個高風險實作缺口**（⛔ 兩者都是「文件宣稱的契約在現行程式裡
做不到」，⛔ 不是措辭問題）：

| # | 缺口 | v19 的處置 |
|---|---|---|
| 1 | ⛔ **CLI 分不出「一般 Stage 2」與「I-074 counterfactual」**——第五道檢查對所有 Stage 2 無條件生效（`evaluation.py:2980` 的 `run_bundle_stage()` 內），直接改就會讓 v18 宣稱保留的 rc=4 **變成不可達** | ⚠️ 新增明示 opt-in **`--i074-counterfactual`**，見「二、④」 |
| 2 | ⛔ **兩份 patch 的獨立 SHA 現行 runner 算不出來**——只有一個 `TOOLING_PATCH`（`run-replay-offline.sh:73`）、一次套用與一個合併 SHA（`:166`）、一個 `--tooling-patch-sha256`（`replay-args.sh:64` 的 `replay_args_offline()`） | ⚠️ 定義**兩個輸入 ＋ 固定順序 ＋ 增量 diff SHA ＋ 合成 hash**，見「三之一之二」；⛔ v18 的「既有機制、不新增參數」**已改掉** |
| 3 | ⚠️ 「唯一產品語意變因」只靠「patch 前後測試全綠」⛔ 證明不了——patch 會一併改測試與 fixture | ⚠️ 補 **differential guard**，見「六、3」 |
| 4 | ⚠️ 前段「Stage 2 有兩種 terminal outcome」讀起來仍像現行規則 | ⚠️ 已就地加註「一般路徑限定」，見該表下方 |

#### （承上）Stage 2 計畫書 v18 的整合內容

⛔ **v16 雖經使用者確認，但那份的 before 基準是 `ecbc141^`——已被證明不可用**
（`ecbc141^` 與正式 after base `e1cbbbd` 相隔 **95 個 commit**，差異無法歸因於 RR 解耦，
見「② before tooling／反事實謂詞設計 v2」）。**切換基準是重大計畫變更**，
⛔ **必須整份重新確認**，⛔ 不能只在後方追加一節。

⚠️ **v18 把 ②v2 的「v3 比較模型」整合回整份計畫**——v17 只列出待修清單，
⛔ **那不是可確認的計畫**。整合後的四條主幹：

| # | v3 模型 | 已同步的節 |
|---|---|---|
| 1 | before ＝ **`e1cbbbd` ＋ counterfactual patch**，⛔ 不是 `ecbc141^` | 一、三、五、六、八 |
| 2 | **after 已封存的 156 keys 是唯一 cohort**；before 仍**全量 13,417 列** replay | 一、二③、六 |
| 3 | **保留全量 key 一致性守門**；⛔ **移除 before／after candidate 集合相等要求** | 二①、五、六、決策樹 |
| 4 | 156 列**全部**產 comparison，再依欄位判 B／C；`candidate_mismatch.json` 與 `EXIT_CANDIDATE_MISMATCH = 4` ⛔ **不適用這條 counterfactual 路徑** | 二①、六、決策樹 |

⚠️ **v18 的範圍縮減（⛔ 不是省事，是重新盤點的結果）**：第 3／4 條移除了
「最壞 13,417 列 candidate mismatch」這個觸發條件，因此**只為它而生的設計一併移除**——
**第二趟讀取、disk-backed spool、串流 mismatch validator、第二趟的 TOCTOU 防護、
`P_C` 磁碟情境、測試矩陣 a～p**。
⚠️ **⛔ 沒有移除的是**：一趟串流 loader、before source artifact、
單一 evidence archive 原子發布、durability 分流與 rc=3 recovery、ENOSPC 事前檢查、memory harness。

| 節 | v18 的同步結果 |
|---|---|
| 一、目標與不做範圍 | ✅ 改成 `e1cbbbd` ＋ counterfactual patch；「⛔ 不改任一版本判定」改成「after ⛔ 不動、before **只有 setup RR 這一個產品語意變因**」 |
| 二、① 謂詞 | ✅ v3 裁決取代 v2 的「before 算完整反事實」；第五道集合檢查改成 `before_candidates == ∅` |
| 二、③ 容量 ／ 二之二 磁碟 | ✅ 兩趟改一趟、spool 移除、公式改 `P_B ＋ M_safety` |
| 三、受影響檔案 | ✅ patch 改成 counterfactual（⛔ 不再是「替 `ecbc141^` 補 CLI／九欄」）；spool 模組移除 |
| 四、contract | ✅ 兩種 patch 分開；`candidate_mismatch` 的介面改動移出範圍 |
| 五、風險 ／ 六、測試矩陣 | ✅ 重寫：移除 mismatch／spool 條目，補「patch 語意越界」與 `before_candidates == ∅` |
| 決策樹 B／C | ✅ 「candidate 集合不一致 ＝ 分支 C」**移除**——before 預期恆空 |
| 八、執行順序 | ✅ 既有測試改驗 `e1cbbbd`；sizing／harness 改單路徑 |


⛔ **以下 v15～v2 的修訂紀錄是歷史，⛔ 不是現行契約**：其中關於 **disk-backed spool、
第二趟讀取、`P_C`、rc=4 終端碼**的裁定，⚠️ **已由 v18 隨 v3 比較模型整批移除**
（理由見「二、③」）。⚠️ 保留它們是為了說明**為什麼曾經需要**，⛔ 不是要照著做。

⚠️ **v15 是文件一致性收尾**（⛔ 無架構變更）：spool 清理時機**唯一裁定為
「archive rename ＋ parent fsync 成功後才清」**（⛔ 不再是「兩者都可以」——測試 o 與
`P_C` 容量情境需要唯一行為）、移除空表頭、測試 l 改成 archive 層措辭、
兩處舊指向改為直接陳述。

⚠️ **v14 把 v13 混進來的方案 A 拆掉**：v13 為了解釋「commit 前保留 spool」，寫成
「spool 是 recovery 唯一的 before／after 來源」——⛔ **那等於要求 spool 具備獨立的
hash／fsync／durability／防竄改契約，正是已拒絕的方案 A**。現在回到**純方案 B**：
spool 只是暫存、⛔ **不是 recovery 輸入**，recovery 一律用**正式 archive 內的
before source artifact ＋ manifest SHA ＋ 已封存的 Stage 1 after artifact**，
而 evidence contract **必須釘住 Stage 1 after artifact 的 SHA 與位置**。

⚠️ **v13 修掉 spool 清理時機的矛盾**：v12 的流程在 **commit 之前**就清 spool，
但 durability 表與測試 o 又定義「archive 已 durable 後清理失敗仍回 0／4」——**那條路徑
依流程不可能發生**。當時明訂 spool 在**正式 archive 之外**、commit 之前保留、rename ＋ parent fsync 都成功後才清。
⛔ **但 v13 給的理由（「它是 recovery 唯一的 before／after 來源」）已被 v14 推翻**
——那等於把方案 A 混進來；⚠️ **本段是歷史說明，⛔ 不是現行契約**。
另把殘留的單檔語意與 `M` 的舊名稱一併改掉。

⚠️ **v12 統一 commit point 的層級**：v11 只改了原子邊界那一節，**前面的 spool 流程、
durability 分流與測試 m～o 仍是「`candidate_mismatch.json` 單檔 commit」的舊語意**
——同一份文件裡兩套規則。現在一律以 **evidence archive 層**為準：staging 內的單檔換名
⛔ **不是** terminal commit，staging 內任何失敗都回一般失敗且⛔ 無正式 archive。
另訂正磁碟公式（`max(P_B, P_C) ＋ M_safety`，⛔ 不是 `3 × S ＋ M`——那會重複計算），
並把 before source artifact **裁定為 Stage 2 evidence artifact**（⛔ 不改既有 replay schema）。

⚠️ **v11 補掉方案 B 自己製造的第二個 rc=3**：before source artifact 若先發布，
它的 fsync 也可能失敗，而那時 **B／C 還沒判定**——⛔ recovery **不能固定回 rc=4**。
處置是**單一 evidence archive 一次性原子發布**，recovery 從 manifest 讀回原 terminal outcome。
另把**謂詞設計提前到 evidence contract 之前**（schema 依賴它），harness 矩陣補上 **B／C 兩條路徑**。

⚠️ **v10 裁決採方案 B 並把 evidence contract 提前**：before source artifact 會決定發布順序、
schema、SHA 關係、B／C 兩分支的輸出集合、orphan 判定與磁碟模型，**所以 evidence 計畫書
必須排在串流／發布實作之前**（執行順序 ②）。另把磁碟門檻標明**要由含該 artifact 的
harness 重新證明**（⚠️ 當時的 `3 × S ＋ M` 公式已於 v12 改掉），並要求 before source artifact 的**定位二選一、⛔ 不能只留名稱**。

⚠️ **v9 揭露一個阻擋點並訂正 M 的時序**：①實查 `evaluation.py` 確認
**Stage 2 從未單獨發布 before rows**，因此 rc=3 的 recovery ⛔ **無法同時做到
「不重跑」與「重驗來源」**——列出三個方案待裁決；②`M` 的「實測決定」與「實作前定案」
互相矛盾，改成「sizing harness → 實測 → 裁定固定 bytes → 確認後才實作 preflight」。

⚠️ **v8 原擬**讓 rc=3 有可執行出口：訂 recovery 契約（⛔ 禁止重新 replay／重建、
重驗用同一套串流驗證、成功後恢復 rc=4、再失敗的退出碼與保留政策、必須有 recovery 測試），
並把磁碟公式固定成 `3 × S ＋ M`（⚠️ **該公式已於 v12 改掉**——把峰值當 `M` 會重複計算 `3 × S`）。
⛔ **但 v9 證明那還不成立**——沒有可獨立驗證的 before 來源，「不重跑」與「重驗來源」
無法同時做到；`M` 的決定時序也矛盾。⚠️ **本段是歷史說明，⛔ 不是現行契約。**

⚠️ **v7 補上 commit point 之後的終端狀態分流**：`os.replace` 成功但 `fsync_dir` 失敗時
**保留正式檔並回 `EXIT_DURABILITY_UNCONFIRMED=3`**（沿用 Stage 1 finalizer 的既有設計），
spool 清理失敗⛔ 不得遮蔽有效的 rc=4；另明訂 **temp validator 本身也必須串流**
（否則整份載回記憶體會抵銷 spool），並把磁碟門檻具體化為 `3 × S ＋ margin`。

⚠️ **v6 補上 spool 的完整流程契約**：`build → validate → publish → raise` 在改成 spool
之後要重新定義（⚠️ **validator 必須驗實際 temp bytes**、⛔ 正式檔發布後才可回 rc=4），
並補磁碟容量事前檢查與 **orphan spool／temp 的中止政策**。

⚠️ **v5 補掉一個高風險容量缺口並裁決峰值方案**：①第二趟在**大型 mismatch**（最壞 13,417 筆）
會重新 OOM（實測 535 MiB ＋ before rows），改用 **disk-backed spool**；
②峰值驗收**裁決走 memory harness**（⛔ 不新增 limited-quota probe——200 列 probe 量不到全量
峰值是本筆自己的教訓），且 harness 必須涵蓋 **worst-case 全量 mismatch**。

⚠️ **v4 修掉 v3 的兩個實質錯誤**：①測試矩陣誤用了 crossday 的案例（candidate_mismatch 沒有 d_only_rows，而全量 key 差異會先被集合檢查②擋下）；②峰值驗收有雞生蛋問題（沒 patch 跑不起 Stage 2、跑了就是正式 scan），已列出兩個方案待裁決。evidence 計畫也改成純外部前置。

⚠️ **v3 的範圍比 v2 小**：移除 `crossday.py` 改造（Stage 1 已完成，⛔ 不是 Stage 2 前置）、
把 Stage 2 evidence contract 拆成獨立計畫書、mismatch 證據改兩趟串流。

⚠️ **v2 訂正了 v1 的兩個高風險錯誤**：①容量改錯執行路徑（v1 量的是 `crossday.py`，而 Stage 2 真正跑的是 `evaluation.py`）；②candidate 謂詞方案不完整且**違反「Stage 2 ⛔ 不得重算 predicate」的 durable contract**。另補 tooling patch 的可重建性與 mismatch 契約的測試缺口。

⚠️ 前置盤點（下一節）是本計畫書的事實基礎，⛔ 不要只讀其中一邊。

##### 一、目標與⛔ 不做的範圍

**目標**：以**正式 after base `e1cbbbd` ＋ counterfactual patch**（把 setup RR 條件加回
`CONTINUATION`）當 before 版，跑完同一份凍結 bundle 的**全量 13,417 列**，
與 Stage 1 已封存的 after 結果在 **after cohort 的 156 keys** 上逐列對照，
判定 I-074 決策樹的 **B**（如預期翻轉）或 **C**（不符預期）。

⚠️ **before 基準是 `e1cbbbd`，⛔ 不是 `ecbc141^`**——後者與正式 after base 相隔 95 個 commit，
差異無法歸因於 RR 解耦（見「② before tooling／反事實謂詞設計 v2」）。

**v3 比較模型（⚠️ 本計畫書全份適用）**：

| 項目 | 做法 |
|---|---|
| ⚠️ **啟用方式（v19）** | ⚠️ **明示 `--i074-counterfactual`**，⛔ 預設關閉——⛔ **一般 Stage 2 ⛔ 不受本模型影響**，見「二、④」 |
| before replay 範圍 | ⚠️ **全量 13,417 列**——⛔ 不能只跑 156 列，否則事件狀態不連續 |
| 守門 | ⚠️ **全量 13,417 keys 兩側一致**（既有的 `assert_same_keys`，⛔ **保留**） |
| cohort | ⚠️ **after 已封存的 156 keys 是唯一 cohort** |
| 逐列比較 | 156 keys **全部**產 comparison 輸出，再依欄位結果判 B／C |
| before 的 candidate 集合 | ⚠️ **預期恆為空**——⛔ **不再要求兩側相同**（那是 RR 條件存在時的正確行為） |
| 第五道集合檢查／`candidate_mismatch.json`／rc=4 | ⛔ **不適用本路徑**，重新裁定見「二、①」 |

| ⛔ 不做 | 理由 |
|---|---|
| ⛔ 不改 **after 側**的任何東西 | after 直接用已封存的 Stage 1 artifact，⛔ 一個位元都不動 |
| ⚠️ before 側**只做一項刻意的產品語意變更** | ⚠️ **唯一產品語意變因是 setup RR 條件**——⛔ 實作會跨 lifecycle 參數、呼叫端、判定式與測試（見「② v2」的 patch 範圍），但⛔ **不得順手改其他判定** |
| ⛔ 不重產 bundle | 見 `development-workflow.md`「凍結 bundle 一旦選定就不得重產」 |
| ⛔ 不動 Stage 1 已封存的證據 | `python/baselines/i074_stage1/` 是終態 |
| ⛔ 不改 canonical JSON 的位元組輸出 | 所有既有 SHA 與 artifact 契約的根 |
| ⛔ 不為了省記憶體停 live container | 那是 2026-09-18 的權宜之計，⛔ 不得變成程序 |
| ⛔ 不改 `crossday.py` | Stage 1 的 comparator，⛔ 不是 Stage 2 前置（理由見下方「⛔ `crossday.py` 的改造不在本計畫範圍」） |

##### 二、四個必須先解的子問題

**① candidate 謂詞在兩版⛔ 不對稱（設計題，⚠️ 本計畫書的第一決策點）**

after 的定義是 `lifecycle_phase == "CONTINUATION" and not rr_qualified`，
但 before 的 CONTINUATION 條件**本身就含 `rr_qualified`**
（`ecbc141^` 的 `decision_engine.py` 第 946-975 行；⚠️ **v18 的 `e1cbbbd` ＋ counterfactual
patch 同樣如此——那正是 patch 要恢復的東西**），原樣搬過去**恆為 `False`**
——於是 before 候選集合必然是空的、after 有 156。
⚠️ **v2 當時認為那「會被誤判成分支 C」；v3／v18 的裁決是⛔ 不再要求兩側集合相等**，
所以**恆空是預期結果**，⛔ 不是誤判來源（見下方 v3 裁決）。

⛔ **v1 建議的「三項價格證據 ＋ `not rr_qualified`」有兩個問題，v2 撤回**：

1. ⛔ **條件不完整**。`resolve_lifecycle()` 的判定順序即優先序，candidate 還需要
   **高優先失敗分支未命中**（`active_bearish_states`／`SUPPORT_RECLAIM_INVALIDATED`／
   `BREAKDOWN`）**且** `event_signal == "CLOSE_RECLAIM"`。⚠️ 只看三項價格證據
   會把被高優先分支吃掉的列也算進來。
2. ⛔ **違反 durable contract**。`sr-zone-scoring.md` 明訂：
   「**Stage 2 ⛔ 不得重算 predicate**——③ 依 after artifact 既有的欄位」、
   「本工具**只消費、⛔ 不重算也不用近似欄位反推**」。
   在 after 側用新謂詞重算，正是那條禁止的事。

**v3 裁決**（⚠️ **待使用者確認**；⛔ **取代 v2 的「before 側自己算完整反事實」**）：

⚠️ **v2 要 before 側另算一套反事實謂詞，目的只是讓兩側集合可以相等。
v3 ⛔ 不要求相等**——於是那套反事實謂詞**整個不需要**：

| 側 | 做法 |
|---|---|
| **after** | ⛔ **不重算**，直接用已封存的 `rr_decoupling_candidate`（156 列）當唯一 cohort |
| **before** | 用**同一個謂詞** `lifecycle_phase == "CONTINUATION" and not setup_rr_qualified`；⚠️ 套了 counterfactual patch 之後它**恆為 `False`** |
| **比對** | ⛔ **不比 candidate 集合**——比的是那 156 keys 的**實有欄位**（見「①之二」）：`lifecycle_phase`／`market_bias`／`action_state`／`position_action_condition.state`／`final_entry_state` |

⚠️ **這同時解掉「Stage 2 ⛔ 不得重算 predicate」那條 durable contract 的衝突**：
兩側都⛔ 不引入新謂詞，after 純消費封存欄位，before 用同一個定義。

**①之二、⛔ 判讀欄位必須是 artifact 實有的欄位（v20 新增）**

⛔ **v19 之前寫的 `market_state` 與 `entry_permission_state` ⛔ 不在凍結的 replay row 裡。**
實查 `_decision_fields_from_summary()`（`evaluation.py:834`）——它們是
`_decision_semantic_pipeline()` 的**區域變數**，⛔ 從未落進 artifact；
⚠️ 而 after artifact **已凍結**，⛔ **不得事後補欄位**（`development-workflow.md`
「凍結 bundle 一旦選定就不得重產」）。

| 決策樹寫的欄位 | artifact 實有的欄位 | 關係 |
|---|---|---|
| `lifecycle_phase` | ✅ `lifecycle_phase` | 直接證據 |
| `market_state` | ⚠️ **`market_bias`** | ⚠️ **推導**：`_market_bias()`（`decision_engine.py:1318`）直接回傳 semantic 的 `bias_state`，而 `bias_state` 由 `market_state` 決定——`BULLISH_CONTINUATION` → `BULLISH_CONTINUATION`、`BULLISH_RECOVERY` → `BULLISH_BIAS`。⛔ **`market_state` 本身⛔ 不是直接證據** |
| `action_state` | ✅ `action_state` ＋ `position_action_condition.state` | 直接證據（⚠️ 後者是 `_position_action_condition()` 複製的同一顆） |
| `entry_permission_state` | ⚠️ **`final_entry_state`** | ⚠️ **⛔ 不是同一顆**：`final_entry_state` 是 execution 層的 `final_entry_permission.state`。⛔ **semantic 的 `entry_permission_state` 在 artifact 裡不可觀測**，它「四格全不變」只能寫成**由 lifecycle 契約推導的含意**，⛔ 不是實測結論 |

**正式 B／C 判讀的欄位集合（⚠️ 唯一）**：
`lifecycle_phase`、`market_bias`、`action_state`、`position_action_condition.state`、`final_entry_state`。

⚠️ **AVOID 那兩格要注意**：`_market_bias()` 在 `market_action == "AVOID"` 時**短路回
`BEARISH_BIAS`**，⛔ 不看 `bias_state`——所以那兩格的 `market_bias` 兩側都是 `BEARISH_BIAS`
（不變），⚠️ 與決策樹「AVOID → AVOID 不變」一致。

**第五道集合檢查、`candidate_mismatch.json` 與 `EXIT_CANDIDATE_MISMATCH = 4` 的重新裁定**：

| 項目 | v3 之下的語意 |
|---|---|
| 「兩側 candidate 集合必須相等」 | ⛔ **移除**——它在 counterfactual 路徑上**必然落空**，留著只會在逐列比較**之前**中止（見 ②v2「⛔ 但『不對稱本身就是結論』行不通」） |
| 取而代之的不變條件 | ⚠️ **`before_candidates == ∅`**——非空代表 **counterfactual patch 沒有真的把 RR 加回去**（patch 成立時 `CONTINUATION` 蘊含 `setup_rr_qualified`，兩者⛔ 不可能同時成立） |
| 違反時的行為 | ⚠️ 中止並產出 **failed-attempt record**（見下方計次列）——⛔ **不產 `candidate_mismatch.json`**、⛔ **不產正式 evidence archive**：這是 **tooling 缺陷信號**，⛔ 不是產品發現，⛔ 不需要保住全差集 |
| `EXIT_CANDIDATE_MISMATCH = 4` | ⛔ **本路徑⛔ 不會合法產生 rc=4**。⚠️ 常數與既有測試**保留**（⛔ 不刪碼——它仍是 Stage 2 一般路徑的契約），但**counterfactual 執行若出現 rc=4，一律當成實作缺陷**、⛔ **不得判為分支 C** |
| 計次 | ⚠️ **提議**：patch 失效導致的中止⛔ **不計入正式 scan**——比照「preflight 與指紋檢查失敗⛔ 不計入這一次」（見「正式 scan 的計次裁決」），理由相同：**工具還沒就位，⛔ 不是驗證跑過了**。⚠️ **這是對既有計次政策的延伸，需一併確認**；⚠️ **但⛔ 不得靜默重跑**——必須產出下方的 **failed-attempt record** |

⛔ **「保存事故紀錄」與「`before_candidates != ∅` 時⛔ 無正式 archive」必須同時成立**，
所以要有**第三種產物**（⚠️ **細節屬步驟 ③ 的 Stage 2 evidence contract，但本計畫先訂死它是必備輸出**）：

| 項目 | 契約 |
|---|---|
| 名稱 | **failed-attempt record**——⛔ **⛔ 不是成功的 evidence archive**，⚠️ 命名與位置要能一眼分辨 |
| 內容 | 兩份 patch SHA ＋ 合成 hash、**有界診斷**（計數 ＋ 前 N 個 key ＋ 該列的 `lifecycle_phase`／`setup_rr_qualified`）、run identity、image ID、失敗原因 |
| 路徑與 schema | ⚠️ 由步驟 ③ 定義；⛔ **不得**放進成功 archive 的 layout，也⛔ 不得沿用其 manifest |
| durability | ⚠️ 原子發布 ＋ fsync（比照既有 finalizer）——⛔ 它是「這一趟發生過什麼」的唯一紀錄 |
| 綁定 | ⛔ **必須**與兩份 patch SHA、run identity 綁定，⛔ 否則證明不了「重跑用的是修過的 patch」 |
| 允許重跑的條件 | ⚠️ **counterfactual patch 的 SHA 必須與失敗那次不同**，且新一次仍要完整走 ⑦⑧⑨；⛔ **SHA 相同的重跑⛔ 不允許**（那只是重跑同一個 bug） |
| ⚠️ **守門時機（v22 補）** | ⚠️ **runner／preflight 必須在 replay 之前**就查找「**同 run identity ＋ 同 counterfactual patch SHA**」的失敗紀錄；⚠️ **命中即在 replay 前中止**——⛔ **不得跑完三小時才拒絕** |
| 發布失敗 | ⚠️ failed-attempt record 自身的發布或 fsync 失敗時的 **recovery 與重跑資格，由步驟 ③ 明確定義**——⛔ 不得留成未定義狀態 |

⚠️ 「三項價格證據也得到 156 筆」可以留作 **sanity check**，
⛔ **不得升格成正式 predicate**。

**② before tooling：把既有中間結果帶出來**
（⚠️ **2026-09-22 起改以 `e1cbbbd` 為 before 基準**——見「② before tooling／反事實謂詞設計 v2」。
⛔ 下面這段是以 `ecbc141^` 為基準時的分析，**規模已大幅縮小**：`e1cbbbd` 本來就有 bundle CLI
與完整九欄位，counterfactual patch 的**唯一產品語意變因是 setup RR 條件**——⛔ 但實作仍跨 lifecycle 參數、呼叫端、判定式與測試）

✅ **好消息**：before 版要的中間變數**全部已存在**且在**同一個函式內**
（`price_follow_through`／`momentum_state`／`rr_qualified`／`clear_zone_breakout`），
⛔ **不是重新實作判定**——與 Stage 0 對 after 版做的事完全對稱。

⚠️ **要對齊的是九個診斷欄位的完整 schema**（見 `sr-zone-scoring.md`），
⛔ 不是只補 `rr_decoupling_candidate` 一欄。Stage 0 已預留介面：
`validate_diagnostics(side="before")` 存在，且**刻意不驗展開式**
（展開式需要 `active_bearish_states`，光靠 row 算不出來）——
⚠️ **那一項要在 before tooling 內自己驗**，它還持有原始 `event_state_summary`。

**③ Stage 2 的容量**（⚠️ v1 改錯了路徑，v2 訂正；⚠️ **v18 依 v3 模型大幅縮小**）

⛔ **v1 把 `crossday.py` 當成 Stage 2 的執行路徑，那是錯的**——
`crossday.py` 是 **Stage 1 的 D／D+1 comparator**；Stage 2 真正跑的是 `evaluation.py`：

| 位置 | 行為 |
|---|---|
| `evaluation.py:3026` `load_artifact` | **replay 之前**就完整載入 after artifact |
| `evaluation.py:3197` `after_rows` | replay 之後 **before rows 與 after rows 同時在場**，再建兩份 by_key map |

**實測真正的 Stage 2 路徑**（用已封存的 D+1 artifact，before rows 以同量級模擬）：

| 階段 | 峰值 |
|---|---|
| 載入 after artifact | 449 MiB |
| ＋ before rows 同時在場 | **524 MiB** |
| ＋ 兩份 by_key map | 524 MiB（map 只存引用，⚠️ 增量小） |
| **建議：after 只常駐頂層 ＋ 全量 keys ＋ 156 筆候選列** | **234 MiB** |

⚠️ **524 MiB 對上 mem-guard 的 531m，只差 7 MiB**——而且這**還沒算 replay 本身**
（Stage 1 實測約 197 MiB）。⛔ **現行做法估計會超過上限**（524 ＋ replay 的 ~197，⚠️ **峰值⛔ 不能直接相加**，
這只是量級判斷）——**正式結論待改完後實測**。
改成串流後**估**約 **431 MiB**（replay 約 197 ＋ 串流後常駐 234）。
⚠️ **這是估算，⛔ 不是實測**——峰值⛔ 不能直接相加（配置器重用、GC 時機都會影響）。
**正式結論只採「改完之後，在釘死的 image 與 cgroup 下實跑 Stage 2 量到的峰值」。**

**改法：一趟串流**（⚠️ **v18 取消第二趟**）。

⚠️ **v4～v15 的第二趟讀取、disk-backed spool、串流 mismatch validator、第二趟 TOCTOU 防護、
`P_C` 情境與測試矩陣 a～p，全部是為「最壞 13,417 列 candidate mismatch」而生**。
⛔ **v3 模型移除了它的觸發條件**（⛔ 不再要求兩側 candidate 集合相等，before 預期恆空），
因此**這些設計一併移除**。⚠️ **取代它的有界診斷是 KB 量級**，⛔ 不需要 spool。

| 趟次 | 時機 | 常駐內容 |
|---|---|---|
| **唯一一趟** | **replay 之前** | 完成全量 schema／diagnostics 驗證、**增量**算 artifact raw SHA 並與 cohort manifest 比對、全量 keys（13,417 tuple）、**156 筆 cohort after rows** |

⚠️ replay 之後只用**已常駐的 156 筆 cohort rows** 做比較，
⛔ **不需要回頭重讀 after artifact**——**第二趟的 TOCTOU 風險因此一併消失**
（SHA 仍在唯一那趟與 cohort manifest 比對，⛔ 那道⛔ 不得省）。

**必須成立的不變條件**（⚠️ v18 收斂）：

| # | 不變條件 |
|---|---|
| a | ⚠️ 串流 loader 的**驗證強度⛔ 不得下降**——`validate_after_artifact()`／`validate_diagnostics()` 的每一項都要在逐列餵入下照驗 |
| b | 常駐的 156 筆 cohort rows **就是** artifact 內該 key 的**完整 row**，⛔ 不是重建、⛔ 不是摘要 |
| c | 唯一那趟增量算出的 raw SHA **等於** cohort manifest 記的值，⛔ 不符即中止 |
| d | ⚠️ **`before_candidates == ∅`**；⛔ 非空即中止並輸出有界診斷（見「二、①」） |
| e | 156 keys **全部**出現在 comparison 輸出，⛔ **不截斷、⛔ 不抽樣**（承「⛔ 不接受 aggregate 當命中證據」） |
| f | 全量 13,417 keys 兩側一致的守門⛔ **保留**——⛔ 只用 cohort 過濾會讓 before 多出來的列被靜默漏掉 |

⚠️ **發布仍走「單一 evidence archive 一次性原子發布」**（v11 裁定）：

```text id="i074_stage2_publish_flow_001"
staging 寫入 before source artifact
  → 寫入 comparison artifact ＋ report（156 列全在內）
  → 驗整份 manifest 與所有 SHA
  → **staging archive → 正式 evidence archive 的 rename**  ← ⚠️ **唯一的 commit point**
  → fsync parent dir
  → 回 **0**
```

⚠️ **staging 內的任何寫入／驗證／fsync 失敗一律回「一般失敗」**，
⛔ **正式 archive 不存在**——⛔ 不會有半成品被當成證據。

⚠️ **rename 成功⛔ 不等於落盤**。⚠️ **沿用 Stage 1 finalizer 既有的 durability 分流**
（`publish.py` 的 `EXIT_DURABILITY_UNCONFIRMED = 3`、`evidence.py` 的「commit point 之後
parent fsync 失敗⛔ 不刪除」），⚠️ **套用層級是整個 evidence archive**：

| 情況 | 正式 **archive** | 結束碼 |
|---|---|---|
| **archive rename 之前**任何失敗（staging 寫入／驗證／fsync、含 `before_candidates != ∅`） | ⛔ **不存在** | **1**（一般失敗） |
| archive rename 成功 ＋ parent dir fsync 成功 | 存在且 durable | **0**（⚠️ **B／C 由人判讀 comparison，⛔ 不是由結束碼分流**） |
| **archive rename 成功但 parent dir fsync 失敗** | ⚠️ **保留，⛔ 不刪除** | **3**（`EXIT_DURABILITY_UNCONFIRMED`）——出口見「rc=3 的 recovery 契約」 |
| archive 已 durable，**只剩輔助 temp 清理失敗** | durable | ⚠️ **仍回 0**——⛔ 不得降成一般失敗；orphan 依清理程序處理。⚠️ **此時已無 staging 可清**——`os.replace` 之後原 staging 目錄**就是**正式 archive |

⚠️ **v3 之下 terminal outcome 只有 0 一種**（⛔ 不再有 0／4 二選一），
⚠️ 這讓 manifest 的 terminal outcome 欄位與 recovery 的「恢復原結果」都退化成單一值
——⛔ **但欄位本身仍要寫，recovery 仍要讀**，⛔ 不得因為只有一個值就省略。

**④ ⛔ counterfactual 路徑必須是明示的 opt-in 模式（v19 新增，⚠️ 本計畫書的第二決策點）**

⚠️ **v18 同時要求兩件事**：counterfactual 路徑**移除**集合相等檢查；一般 Stage 2 路徑
**保留** `candidate_mismatch.json`／rc=4 且既有測試原樣保留。
⛔ **但現行 CLI 分不出這兩者**：

| 事實 | 位置 |
|---|---|
| stage 只由 `--after-artifact` ＋ `--cohort-manifest` 成對與否推導 | `scripts/lib/replay-args.sh:79` 的 `replay_args_offline_stage()` |
| 第五道集合檢查對**所有** Stage 2 執行**無條件**生效 | `evaluation.py:2980` 的 `run_bundle_stage()` 內的 `sorted(before_candidates) != sorted(after_candidates)` |

⛔ **所以直接改那段就會連一般路徑一起改掉**——v18 宣稱保留的 rc=4 會變成**不可達**，
⚠️ 那等於用文件宣稱了一條**實際上不存在**的契約。

**裁決（⚠️ 待確認）：新增 `--i074-counterfactual`，⛔ 預設關閉。**

| 規則 | 內容 |
|---|---|
| 開啟時 | 走 v3 模型：after cohort 156 keys、`before_candidates == ∅`、⛔ **無**集合相等檢查 |
| 關閉時（預設） | ⚠️ **一般 Stage 2 行為完全不變**——集合相等檢查、`candidate_mismatch.json`、rc=4 全部照舊 |
| ⛔ **禁止的實作** | ⛔ **不得從 `before_ref` 值、patch hash 非空、cohort 大小等狀態暗中推斷模式**——⚠️ 模式只能是**明示參數** |
| stage 限定 | ⚠️ **只限 Stage 2**（⚠️ 與既有兩個 flag **相反**，那兩個只限 Stage 1），用同一套 `assert_i074_flags()` 擋 |
| ⚠️ **flag** 要同步的既有機制 | `I074_FLAGS`（`evaluation.py:3316`）、`_reject_duplicate_i074_flags()`（`:3319`）、`BUNDLE_ALLOWED_ARGS`（`:3280`）、`assert_i074_flags(stage=)`（`:3350`）——⛔ **不是只加一個 `add_argument`**；⛔ **flag ⛔ 不進 `SCRIPT_INJECTED_ARGS`**（它是使用者 opt-in） |
| ⚠️ **SHA** 要同步的既有機制 | `--counterfactual-patch-sha256` 才進 `SCRIPT_INJECTED_ARGS`（`:3291`）與 `BUNDLE_ALLOWED_ARGS`——⚠️ **兩者的同步對象⛔ 不同，⛔ 不要混寫** |

**所有權與啟用的 truth table（v20 補，⛔ 這一節⛔ 不得再留「待裁定」）**：

| 參數 | 誰給 | 規則 |
|---|---|---|
| `--i074-counterfactual` | ⚠️ **使用者**傳給官方 runner | ⛔ **不屬** `SCRIPT_INJECTED_ARGS`；⛔ 預設關閉 |
| `--counterfactual-patch-sha256` | ⚠️ **runner 推導並注入** | ⛔ **使用者⛔ 不得傳**（比照 `--tooling-patch-sha256`，進 `SCRIPT_INJECTED_ARGS`；重複出現即中止） |
| `COUNTERFACTUAL_PATCH` | 使用者（環境變數） | 見下表 |
| `TOOLING_PATCH` | 使用者（環境變數） | ⚠️ **可為空** |

| flag | `COUNTERFACTUAL_PATCH` | 行為 |
|---|---|---|
| **開啟** | **存在且非空** | ✅ 走 v3 模型；⚠️ **只能是 Stage 2**；⚠️ **納入 `I074_MODE`** → ⛔ 不自動 pin、**必須**有 `REPLAY_IMAGE_ID` 與 run identity |
| **開啟** | 空／未提供 | ⛔ **中止**——⚠️ 沒有 patch 的「反事實執行」⛔ 沒有意義 |
| **關閉** | 非空 | ⛔ **中止**——⛔ 不得在一般路徑偷偷帶語意 patch |
| **關閉** | 空 | ✅ 一般 Stage 2，⚠️ 行為**逐項不變** |

✅ **`I074_MODE` 裁決：納入**——`run-replay-offline.sh:62` 的 `I074_MODE` 偵測要加上這個 flag。
⛔ **不納入的話，正式 Stage 2 反事實執行反而繞過既有的 run identity 守門**——
⚠️ 那是三趟必須跑在同一個 image 上的唯一保證。

⚠️ 這張 truth table 的 **shell 與 Python 測試都要有**（見「六、2」的 o～x）。

⛔ **Python CLI 也必須自己成對守門（v21 補）**——⛔ **不能只靠 runner**：
⚠️ runner 若漏注入 SHA，Python 端仍會進 counterfactual 路徑並產出**不完整的 evidence**。

| # | 斷言 | 行為 |
|---|---|---|
| 1 | `i074_counterfactual` 開啟但 `counterfactual_patch_sha256` **缺席** | ⛔ 中止 |
| 2 | `counterfactual_patch_sha256` 存在但 flag **關閉** | ⛔ 中止 |
| 3 | **Stage 1** 出現 `counterfactual_patch_sha256` | ⛔ 中止 |
| 4 | SHA **不是 64 位小寫 hex** | ⛔ 中止（比照 `validate_provenance()` 對既有 hash 欄位的強度） |
| 5 | 兩者都在且格式正確 | ✅ 進 counterfactual 路徑 |

⚠️ **官方 runner 與「直接呼叫 Python CLI」兩條路徑都要有測試**——
⛔ 只測 runner 等於沒測這道守門。

⚠️ **這一組必須有的測試**：argv fixture、provenance、CLI ownership（⛔ 使用者不得注入）、
重複參數、stage 限定，⚠️ **以及「關閉時一般路徑逐項不變」**。

##### rc=3 的 recovery 契約（⚠️ **Stage 2 evidence 計畫書的必備項目**）

⛔ **只寫「由 recovery 流程重新驗證並 fsync」是不夠的**——那樣 rc=3 會變成
**只有描述、沒有可執行出口**的狀態。evidence 計畫書**必須**釘死下列各項：

| 項目 | 契約 |
|---|---|
| 輸入 | ⚠️ **使用既有的正式 evidence archive**——⛔ **禁止重新 replay、⛔ 禁止重建 mismatch** |
| 驗證 | 重新做**串流**的 schema、來源與 SHA 驗證（與發布前同一套，⛔ 不得放寬） |
| ⚠️ **before 來源** | ⚠️ **使用正式 archive 內的 before source artifact**（＋ manifest SHA ＋ 已封存的 Stage 1 after artifact） |
| 成功 | ⚠️ **恢復 manifest 記錄的原本終端結果**——⚠️ **v3 之下那個值恆為 0**（⛔ 不是固定回 4；理由見下方「✅ 已避免：兩階段發布會製造第二個 rc=3」）。⛔ **仍須從 manifest 讀，⛔ 不得寫死** |
| 再失敗 | ⚠️ 明訂退出碼與保留政策——⛔ **不刪除既有的正式 evidence archive** |
| 測試 | ⚠️ evidence 計畫**必須包含對應的 recovery 測試**，⛔ 不能只在文件描述 |

###### ✅ 已由方案 B 解決：recovery 的 **before 來源**

⚠️ **v18 註記**：下表的成因裡有兩列提到 spool 與 mismatch 路徑——
⚠️ **那些在 v3 模型下已不存在**（見「二、③」）。⚠️ **但方案 B 的結論⛔ 不受影響**：
before rows 仍然只存在正式程序的記憶體，仍需要 before source artifact 才能在 rc=3 之後
不重跑而重驗。

⚠️ **此阻擋點由 evidence archive 內的 before source artifact 解決**——
⛔ 但**正式執行仍以 evidence contract 完成為前置**（執行順序 ③）。
下面保留成因，⛔ 它⛔ 不是現行的未解問題：

⚠️ 當初的矛盾是「⛔ 禁止重跑」與「重做**同強度**的來源驗證」**無法同時成立**：

| 事實 | 後果 |
|---|---|
| Stage 2 **只發布** `comparison_artifact`／`report`／`candidate_mismatch`（實查 `evaluation.py`） | ⛔ **before rows 從未被單獨發布** |
| before rows 只存在正式程序的**記憶體** | 程序結束就沒了 |
| spool 只保存**第二趟取得的 after rows** | ⛔ 不含 before |
| `finally` 會清除 spool | rc=3 之後連 spool 都不在 |
| mismatch 路徑⛔ **不產** comparison artifact | before 也不在那裡 |
| `candidate_mismatch.json` **內嵌**的 before row | ⛔ **不能當自己的獨立來源**——那是循環論證，抓不出漏列或來源被竄改 |

**裁決：採方案 B**（✅ **使用者已確認 2026-09-22**）。
理由：讓**正式的 before replay 本身**成為可獨立複核的證據，比把暫存 spool 升格成
**半永久的 recovery 狀態**清楚；代價是**必須先完成 evidence contract**，
並把新 artifact 納入容量與發布模型。

| 方案 | 做法 | 代價 |
|---|---|---|
| **A** | rc=3 時**保留並 durability-protect** 一份已驗證、含 before／after 來源的 recovery spool；成功恢復後才清除 | ⚠️ spool 從「暫存」升級成**受保護的中間狀態**，要定義它的完整性檢查與清理時機 |
| **B（採用）** | ⚠️ **commit 之前在 evidence staging 內寫入並驗證** before source artifact；⛔ **不獨立發布**，**只隨整個 archive 一次性發布** | ⚠️ 要定義**檔名、schema、在 archive 內的位置、SHA 關係、recovery 用法**——⛔ 目前完全未定義；但 before rows 本來就該是證據的一部分 |
| C | 明確**降低** recovery 的驗證範圍 | ⛔ **不建議**——違反上表「⛔ 不得放寬」那一列，等於讓 rc=3 的恢復結果不可信 |

###### ✅ 已避免：**兩階段發布**會製造第二個 rc=3

⚠️ 若 before source artifact 在 terminal artifact **之前**正式發布，
**它的 directory fsync 也可能失敗**。那個時點：

* 正式 replay **已經跑完**，⛔ 禁止重跑；
* **B／C 還沒判定**；
* ⛔ **所以 recovery 成功後⛔ 不能固定回 rc=4**——最後可能走正常比較的 **B 分支（rc=0）**。
  （⚠️ **v18 之下這個風險消失**：terminal outcome 恆為 0；⛔ **但「從 manifest 讀回」的做法照舊**。）

**處置：單一 evidence archive、一次性原子發布**（比照 Stage 1 finalizer 的整包 rename）：

```text id="i074_stage2_atomic_001"
staging 寫入 before source artifact
  → 產生 comparison／report（⚠️ v18：⛔ 已無 C 分支的 candidate mismatch）
  → 完整驗證 manifest 與**所有** SHA
  → **最後一次性發布整個 evidence archive**   ← 唯一的 commit point
  → recovery 從 **manifest 讀回原本的 terminal outcome**（⚠️ v3 之下恆為 **rc=0**）
```

⚠️ **若改採「分兩階段正式發布」**，就**必須**另外定義
「**before source 已發布、但 B／C 尚未判定**」的 **resume 模式**，
且 ⛔ **不得 replay**——⚠️ 那是額外的狀態機，⛔ 不要順手做。

⚠️ 計畫書目前雖提到「before after_artifact」，但**只有名字**——⛔ 檔名、schema、發布順序、
durability 與 recovery 用法**全部未定義**，⛔ **不能當成已解決**。

⛔ **`validate_candidate_mismatch()` 的介面改動⛔ 已移出本計畫範圍（v18）**：
v3 之下這條 counterfactual 路徑⛔ 不會產生 `candidate_mismatch.json`，
所以⛔ **不動它、⛔ 不降它的驗證強度、⛔ 也不為它改來源介面**——
⚠️ 它與既有測試**原樣保留**，仍是 Stage 2 一般路徑的契約。

⛔ **`crossday.py` 的改造⛔ 不在本計畫範圍**（v2 說「另列」卻仍把它寫進受影響檔案、
風險、測試與執行順序——那是自相矛盾，v3 整個移除）。理由：

* **Stage 1 已完成**，`crossday.py` 是它的 comparator，⛔ **不是 Stage 2 的前置**；
* 混進來只會**擴大正式執行前的變更面**，而 Stage 1 的教訓正是「變更面愈大、愈晚才發現問題」。

⚠️ 它確實也有 730 MiB 的問題（實測 OOM），**另立一筆處理**。
⚠️ 真要一起做就⛔ **不能叫「另列」**，而且必須明訂演算法——
⛔ **不是同步 `zip()`**（插入／刪除／換序會整個錯位），
而是能維持**排序交集**與**兩側差集 row** 的具體做法。

##### 二之二、⚠️ 磁碟與中止政策（⚠️ **v18 移除 spool 之後重新盤點**）

⛔ **v5～v15 的 disk-backed spool 已移除**（見「二、③」）。⚠️ **仍然要有磁碟政策**——
evidence staging 內同時存在 **before source artifact ＋ comparison ＋ report 的 temp 與正式檔**，
⚠️ before source artifact 本身就是**全量 13,417 列**的量級。

| 項目 | 政策 |
|---|---|
| **事前檢查** | ⚠️ **replay 之前**就檢查可用空間，⛔ 不要跑完三小時才因 `ENOSPC` 失去證據。**公式固定為一條**：`required = P_B ＋ M_safety`（⚠️ **v18 移除 `P_C`**——mismatch 路徑不存在了，⛔ 不再有第二條峰值）。`P_B` 由 sizing harness 實測，`M_safety` 為寫死常數；⚠️ **兩者目前都是占位符**，決定時序見下 |
| Python exception／validator 失敗 | `finally` 清除 staging 與 temp。⚠️ **含 rc=3（archive 已 rename、parent fsync 失敗）那條路徑**——⚠️ 此時**已無 staging 可清**，`os.replace` 之後原 staging 目錄**就是**正式 archive，⛔ **不得刪除** |
| **SIGKILL／主機斷電** | ⛔ **無法保證即時清除**——要定義 **orphan staging／temp 的辨識方式**與人工清理（或安全 recovery）程序 |
| ⛔ **orphan 不得被誤認為證據** | ⚠️ orphan staging ⛔ **不得**被當成正式 evidence archive——命名與位置要能一眼分辨；⚠️ **只有正式 archive 才帶 terminal outcome** |

⚠️ **`M` 的決定有時序矛盾，v9 訂正**：v8 寫「由 harness 實測決定」但「實作前定案」
——而 harness 自己排在步驟 ②，那時還沒有實測結果。⛔ 這條門檻目前**還不可落實**。
改成明確的四步：

```text id="i074_disk_margin_order_001"
① 先實作**獨立的 sizing harness**（⛔ 不含正式 preflight）
② 記錄實測的 staging temp／正式檔峰值
③ 記錄 **P_B** 基準，據此裁定 **M_safety = <固定 bytes>**
④ 更新計畫並經確認後，才實作正式 preflight
```

⛔ **在 ③ 定案之前⛔ 不得實作 preflight**——否則就是把占位符寫進程式。

##### 三、受影響檔案與資料流

| 檔案 | 改動 | 風險 |
|---|---|---|
| **`evaluation.py`** ⬅️ **最主要** | Stage 2 的 after loader 改**一趟串流**；⛔ 不再完整保留 13,417 筆 after rows，只常駐頂層 ＋ 全量 keys ＋ **156 筆 cohort rows**；移除「兩側 candidate 集合相等」那道檢查，改成 `before_candidates == ∅` | ⚠️ 這是**真正的 Stage 2 執行路徑**，v2 漏列 |
| `replay_bundle/artifacts.py` | `validate_after_artifact()`／`validate_diagnostics()` 支援逐列餵入 | ⛔ **驗證強度不得下降**；⚠️ `validate_candidate_mismatch()` ⛔ **不動**（v18 移出範圍） |
| Stage 2 streaming loader 所在模組 | 新增／調整（`replay_bundle/` 底下） | ⚠️ 要能**重新開啟**，⛔ 不是一次性 iterator；⛔ **v18 已無 spool** |
| **memory harness** | 新增（13,417 列 before rows ＋ cohort 常駐 ＋ 發布路徑） | ⚠️ 在釘死的 image／cgroup 內跑，⛔ 不是一般 CI 測試；⛔ **v18 只有一條路徑（B）**，⛔ 不再有 worst-case mismatch 情境 |
| **Stage 2 evidence contract**（含 **before source artifact** ＋ **counterfactual patch** ＋ tooling patch） | ⚠️ **外部硬性前置，排在實作之前**（執行順序 ③） | ⛔ 在它確認前，⛔ 不得決定發布順序、schema、patch 版控位置或 manifest 寫法 |
| Python 測試 | `tests/test_replay_bundle_stages.py`、`tests/test_i074_mismatch.py` | 測試矩陣見「六、2」；⚠️ `test_i074_mismatch.py` 的既有案例⛔ **原樣保留**（它釘的是一般路徑） |
| shell 測試 | `scripts/test-replay-args.sh` | Stage 2 的 argv／mount |
| **counterfactual patch** | ⚠️ 對 **`e1cbbbd`** 施加：把 setup RR 條件加回 `CONTINUATION`（跨 lifecycle 參數、呼叫端、判定式與測試） | ⚠️ **patch 本體要進版控**，⚠️ **與 tooling patch 分開記錄**，見「三之一」「三之二」 |
| `scripts/run-replay-offline.sh` | ⚠️ **兩份 patch 的輸入與固定套用順序**（⛔ v18 寫「既有機制、不新增參數」**是錯的**，見「三之一之二」） | ⛔ 只有一個 `TOOLING_PATCH`（`run-replay-offline.sh:73`），⛔ 現行做不到 |
| `scripts/lib/replay-args.sh` | ⚠️ **增量 diff SHA**（⛔ 不可兩次都對 base 取 diff）＋ 新的 `--counterfactual-patch-sha256` 注入 | ⛔ 現行 `replay-args.sh:174` 的 `replay_args_tooling_patch_sha256()` 只算合併後的一份 |
| `evaluation.py` 的 CLI parser | ⚠️ **`--i074-counterfactual` opt-in**（⛔ 預設關閉）＋ `I074_FLAGS`／`BUNDLE_ALLOWED_ARGS`／`SCRIPT_INJECTED_ARGS`／`assert_i074_flags()` 五處同步 | ⛔ **不加這個 flag 就會連一般 Stage 2 一起改掉**，見「二、④」 |
| `scripts/test-replay-args.sh` ＋ argv fixture | ⚠️ flag、兩份 patch SHA、CLI ownership、重複參數、stage 限定 | ⚠️ 目前⛔ 無 Stage 2 argv fixture（只有 stage0／stage1／comparator／finalizer） |

⛔ **v18 移除的受影響檔案**：`ecbc141^` 的 bundle CLI 移植（基準換成 `e1cbbbd`，
CLI 與九欄位本來就在）、disk-backed spool 模組、串流 mismatch validator。

**資料流**：`e1cbbbd` worktree ＋ **counterfactual patch** → 全量 13,417 列 → before rows
→ 全量 keys 守門 → `before_candidates == ∅` 守門 → 取 after cohort 的 **156 keys** 逐列對照
→ `comparison_artifact.json` ＋ `report.json` ＋ **before source artifact**，
一次性原子發布成 Stage 2 evidence archive → **rc=0**，B／C 由人判讀。

##### 三之一、⛔ **counterfactual patch ⛔ 不是 tooling patch**——兩者必須分開記錄

⚠️ 既有契約裡的 **tooling patch 是「只補輸出、⛔ 不改判定」**
（「tooling 不得改變任一版本的既有判定」）。
⛔ **但 counterfactual patch 刻意改變 lifecycle 判定**——把 setup RR 加回 `CONTINUATION`。
⚠️ **把它記成 tooling patch，等於讓 provenance 把「產品語意修改」偽裝成純 instrumentation。**

evidence contract 必須**分開記錄兩者**：

| 欄位 | 內容 | 驗證規則 |
|---|---|---|
| `counterfactual_patch` | ⚠️ **刻意恢復 RR 條件的語意 patch** | 獨立 SHA；⚠️ 要能證明「**唯一產品語意變因是 setup RR 條件**」 |
| `tooling_patch` | 若仍有**純工具／證據輸出**的改動才使用 | 獨立 SHA；⛔ **不得含任何判定變更** |

⛔ **兩者⛔ 不得合併成同一個 SHA**——否則⛔ 無從分辨「哪些差異來自 RR、哪些來自工具」。

##### 三之一之二、⛔ **兩份 patch 的套用與獨立 SHA，現行 runner ⛔ 做不到**（v19 新增）

⚠️ **v18 只寫「分開記錄」就停住了，⛔ 沒說怎麼算**。實查現行機制：

| 事實 | 位置 |
|---|---|
| 只有**一個**輸入 `TOOLING_PATCH`（`run-replay-offline.sh:73`） | ⛔ 沒有第二份 patch 的位置 |
| 只套用一次、只算**一個**合併後的 `TOOLING_PATCH_SHA256`（`run-replay-offline.sh:166`） | ⛔ 無法分離兩份 patch 的貢獻 |
| 該 SHA 的定義是「**套完之後整個 worktree 相對 base 的 `git diff --binary`**」 | `replay-args.sh:174` 的 `replay_args_tooling_patch_sha256()` |
| provenance 只有一個 `--tooling-patch-sha256` | `replay-args.sh:64` 的 `replay_args_offline()` |

⛔ **所以 v18 受影響檔案表寫的「既有機制、⛔ 不新增參數」是錯的**（v19 已改）。
⚠️ 而且照現行定義**直接算第二次，第二份 SHA 會包含第一份的差異**——那正是要避免的事。

⛔ **⛔ 不可用「中繼 commit」切分**（v20 訂正 v19）：建 commit 會**動到 detached worktree 的
HEAD**，與 `replay-args.sh:148` 的 `replay_args_prepare_worktree()` 契約直接衝突
——它在建完 worktree 後**斷言 `HEAD == 解析出的 OID`，不符即中止**。
⚠️ 而且 commit 會把 **git author、簽章、hooks 與 commit metadata** 這些⛔ 不必要的依賴
拉進本來純粹的 diff 計算。⚠️ **`git write-tree` 只固定 index 的 tree，⛔ 不碰 HEAD。**

⛔ **並撤回 v19 的一個錯誤假設**：「交換套用順序後合成 hash 必然不同」⛔ **不成立**——
⚠️ 兩份 patch 若改的是**不同檔案**，交換順序會得到**完全相同的 final tree**。
**順序只能由結構強制**：runner 固定先 counterfactual 後 tooling，
且 **evidence manifest 記 ordered components**（順序是 manifest 的一部分）；
⛔ **不得依賴「hash 不同」來偵測順序錯誤**。

**要在步驟 ② 一併裁定的項目**：

| 項目 | 內容 |
|---|---|
| 輸入 | `COUNTERFACTUAL_PATCH` 與 `TOOLING_PATCH` **兩個獨立輸入**（⚠️ 後者可為空） |
| 套用順序 | ⚠️ **固定為 counterfactual → tooling**（語意在前、instrumentation 疊在其上），⛔ 不得交換、⛔ 不得合併成一份 |
| 各自的獨立 SHA | ⚠️ **中繼 tree ＋ 增量 diff**（⛔ **v20 撤回 v19 的「中繼 commit」**，理由見下）：<br>① 套 counterfactual（`git apply --index`）→ **`git write-tree`** 得 `T1`<br>② `counterfactual_patch_sha256` ＝ `diff --binary <base> <T1>`<br>③ 套 tooling → `write-tree` 得 `T2`<br>④ `tooling_patch_sha256` ＝ `diff --binary <T1> <T2>`<br>⑤ 合成 ＝ `diff --binary <base> <T2>`<br>⚠️ **HEAD 全程維持原始 base commit**。⛔ **不可兩次都對 base 取 diff** |
| 合成後的完整 hash | ⚠️ 最終 worktree 相對 base 的**完整** diff hash ＋ 既有的 `project_modules_sha256`——⛔ **兩份增量 SHA ⛔ 不能取代它** |
| provenance 放哪 | ⚠️ **見下方「⛔ 為什麼⛔ 不加進 `PROVENANCE_FIELDS`」** |
| 測試 | spoof（宣稱值 ≠ 實際）、重複傳、漏傳、**順序交換**、空 tooling patch、SHA 不符即中止 |
| 受影響檔案 | `scripts/run-replay-offline.sh`、`scripts/lib/replay-args.sh`、`evaluation.py` 的 CLI parser 與 `SCRIPT_INJECTED_ARGS`、`scripts/test-replay-args.sh`、argv fixture |

###### ⛔ 為什麼⛔ 不加進 `PROVENANCE_FIELDS`

⚠️ `PROVENANCE_FIELDS`（`provenance.py:44`）是 **10 欄的封閉 tuple**，
而 `validate_provenance()` 做的是**精確集合相等**
（`provenance.py:200` 的 `validate_provenance()`：多欄、缺欄一律中止）。
⛔ **加第 11 欄會讓所有已封存的 Stage 1 artifact 立刻驗不過**
（`python/baselines/i074_stage1/` 存的是 10 欄），⚠️ **那會打掉 I-100 的「可獨立複核」**。

| 方案 | 裁決 |
|---|---|
| **(i) 放進 Stage 2 evidence manifest**（步驟 ③ 的新 contract） | ✅ **建議採用**——既有 replay schema ⛔ 不動；evidence manifest 本來就要新設計 |
| (ii) 擴成 11 欄 ＋ schema 版本升級 | ⛔ **不建議**——要同步 `crossday.py` 的 `provenance_differences()`、四個 role 的 nullability，以及**已封存證據的重新驗證** |

⚠️ **採 (i) 的殘留風險⛔ 不得省略不寫**：counterfactual 執行時，replay artifact 自身的
`tooling_patch_sha256` 記的是**合成工作樹的完整 diff**——
⛔ **單看那份 artifact 會誤以為只有 instrumentation**。
**緩解（硬性）**：evidence manifest 的 validator **必須**斷言
「兩份 patch 依固定順序套用後的合成 hash **等於** artifact 的 `tooling_patch_sha256`」，
且 ⛔ **counterfactual 執行在沒有對應 evidence manifest 時⛔ 不得被採信**。

##### 三之二、⚠️ tooling patch 要保存內容——⛔ 但那需要一套**新的 evidence contract**

⛔ **v2 寫「把 binary patch 加進 evidence manifest」是做不到的**：
現有的 `evidence_manifest.json` 是 **Stage 1 專用的封閉九檔 layout**，
validator 明文要求 `set(files)` **恰好等於** `ARCHIVE_PATHS`
（`evidence.py` 的 `validate_evidence_manifest()`），binary patch ⛔ 塞不進這個 schema。

⚠️ **需求本身成立**：`replay_args_tooling_patch_sha256()` 算的是當下 worktree 實際 diff 的
hash，那個值**只能驗證、⛔ 不能重建內容**。patch 一旦遺失，Stage 2 的證據就無法獨立複核
——而「可獨立複核」正是 I-100 存在的理由。

⛔ **但這是一套要另外設計的 contract，⛔ 不是本計畫書塞得下的一段**。要定義的至少有：

| 項目 | 內容 |
|---|---|
| 目錄與檔名 | Stage 2 evidence 的路徑、**封閉檔案集合** |
| patch 的形式 | raw binary entry？還是另包一份 metadata artifact？ |
| schema 與 validator | 新 manifest 的封閉欄位、完整性檢查 |
| 發布與修復 | 原子發布、durability、recovery（比照 Stage 1 的 finalizer） |
| **三方 SHA 關係** | `stored_sha256`、**實際套用後的 diff SHA**、provenance 記的 SHA，三者必須一致 |
| 對應程式 | publisher／finalizer ＋ Python 與 shell 測試 |

⚠️ **建議：本計畫書只定義「需求與驗收條件」，實作另立一份 Stage 2 evidence 計畫書。**
⛔ 在那份完成之前，Stage 2 **不得進入正式執行**——否則證據無處可放。

##### 四、contract 變化

⚠️ **要分兩層講，⛔ 不能一句「沒有格式變化」帶過**（v2 的說法與「patch 進 manifest」自相矛盾）：

| 層 | 變化 |
|---|---|
| **既有 replay artifact schema** | ⛔ **不變**——串流只改「怎麼讀」，⛔ 不改「讀出來是什麼」；canonical JSON 的位元組輸出、所有 SHA、`after_artifact`／`comparison`／`candidate_mismatch` 的 schema 全部照舊 |
| **Stage 2 evidence archive** | ⚠️ **新增一套 contract**（見「三之二」）——現有的封閉九檔 manifest ⛔ 容不下 patch |
| **before source artifact**（方案 B） | ⚠️ **裁定為 (i)：新的 Stage 2 evidence artifact**——⛔ **不改既有 replay artifact schema**。理由：v11 已裁定「單一 evidence archive 一次性發布」，它本來就在 archive 內；**schema、SHA、validator 與 archive layout 由步驟 ③ 的 evidence contract 定義** |
| **兩種 patch 的 provenance** | ⚠️ **`counterfactual_patch` 與 `tooling_patch` 分開記錄**（見「三之一」）——⛔ 不得合併成同一個 SHA |
| `validate_candidate_mismatch()` 的**來源介面** | ⛔ **不變（v18）**——這條 counterfactual 路徑⛔ 不產生 `candidate_mismatch.json` |
| **Stage 2 的結束碼** | ⚠️ **v3 之下正常路徑恆為 0**；rc=1／rc=3 照舊；⛔ **rc=4 ⛔ 不會在本路徑合法出現** |

##### 五、風險與回滾

| 風險 | 對策 |
|---|---|
| ⚠️ 串流後 after 的驗證強度下降 | ⛔ **`validate_after_artifact()`／`validate_diagnostics()` 的每一項都不得放寬**；測試見「六、2」 |
| ⚠️ 兩個 validator **各自遍歷一次 rows** | 改成 iterator 後⛔ 不能重複消費。要嘛定義**單趟複合驗證**，要嘛提供**可重新開啟的 iterator factory**——⛔ 不得因為改串流而少驗一道 |
| ⚠️ 逐列驗證漏掉整份層級的檢查 | 保留 `bundle_id`／`kind`／`schema_version` 等頂層驗證，只把 rows 那段改串流 |
| ⛔ **counterfactual patch 改到 RR 以外的語意** | ⚠️ **本輪最高風險**（它刻意改判定，⛔ 不是純 instrumentation）。對策：diff 逐行可讀、⚠️ **唯一產品語意變因是 setup RR 條件**、套用前後 `e1cbbbd` 既有測試全綠、**獨立 SHA 進 Stage 2 evidence manifest**（⛔ **不是** provenance——⛔ 不得加第 11 欄，見「三之一之二」）並與 `tooling_patch` 分開 |
| ⛔ **patch 沒真的生效**（RR 沒被加回去） | ⚠️ `before_candidates == ∅` 的守門會抓到；⛔ 非空即中止、⛔ **不得當成分支 C**；⚠️ 此中止**⛔ 不計入正式 scan**（待確認，見「二、①」） |
| ⚠️ before 側有列缺席 | 全量 13,417 keys 守門**保留**——⛔ 只用 cohort 過濾會靜默漏列 |
| ⚠️ 156 列比較結果被截斷或抽樣 | ⛔ **不截斷、⛔ 不抽樣**（不變條件 e）；承「⛔ 不接受 aggregate 當命中證據」 |
| ⚠️ before source artifact 讓磁碟或記憶體超標 | sizing harness 量 `P_B`、memory harness 驗 < 450 MiB，**兩者都在正式執行之前** |
| ⚠️ evidence archive 半成品被當成證據 | staging 內任何失敗一律 rc=1 且⛔ 無正式 archive；orphan 命名可辨識 |
| ⛔ **模式旗標缺席，一般路徑被連帶改掉** | ⚠️ **v18 的高風險缺口**：rc=4 會變成不可達。對策：`--i074-counterfactual` opt-in ＋ **測試 o「未帶 flag 時逐項不變」**；⛔ **不得靠 ref／hash 暗中推斷模式** |
| ⛔ **兩份 patch 的 SHA 互相污染** | ⚠️ 現行 SHA 定義是「整個 worktree 對 base 的 diff」，直接算兩次⛔ 一定會包含前一份。對策：**固定順序 ＋ 中繼 tree（`write-tree`）＋ 增量 diff**；⚠️ 順序由 runner 結構與 manifest 的 ordered components 強制，⛔ **不靠「hash 必然不同」**（測試 t） |
| ⚠️ counterfactual 在 replay artifact 裡看起來像純 instrumentation | ⚠️ 採方案 (i) 的**已知殘留風險**。對策：evidence manifest **必須**斷言合成 hash 等於 artifact 的 `tooling_patch_sha256`；⛔ 無 manifest 的 counterfactual 執行⛔ 不得採信 |
| **回滾** | 串流是純讀取路徑的改寫，`git revert` 即可；counterfactual patch 本來就不進主線 |

⛔ **v18 移除的風險條目**（觸發條件已不存在）：「大型 mismatch 讓第二趟重新 OOM」、
「spool 殘留」、「第二趟 TOCTOU」、「謂詞換掉後驗錯對象」（v3 兩側⛔ 不換謂詞）。

##### 六、測試與驗證策略

1. **Stage 2 峰值驗收——裁決走方案 B（memory harness）**：

   ⛔ **不走「新增 Stage 2 limited-quota probe」**：本筆自己的教訓就是
   **200 列的 probe 量不到全量峰值**（見「③ 第一次執行：OOM 失敗」），
   受限 quota 的 Stage 2 probe 同樣會**低估完整 before rows 與 evidence 發布峰值**，
   而且還要多背一套 CLI contract。

   ⚠️ **harness 必須在釘死的 image／cgroup 內**涵蓋下列全部，⛔ 少一項都不算數：

   | # | 涵蓋項目 |
   |---|---|
   | a | **13,417 筆同尺寸 before rows**（replay 產出的量級） |
   | b | **一趟**串流後常駐的 metadata、**完整 keys**、**156 筆 cohort rows** |
   | c | 正常 comparison 路徑（**156 列全產出**） |
   | d | **before source artifact ＋ comparison ＋ report 的 evidence staging 與一次性發布** |
   | e | SHA、validator 與 **temp 清理** |

   ⛔ **v18 移除 harness 的 worst-case mismatch 情境與 `P_C` 路徑**——觸發條件已不存在
   （見「二、①」）。於是磁碟只剩**一條**峰值：

   ```text id="i074_disk_formula_001"
   required = P_B ＋ M_safety
   ```

   `P_B` 是 sizing harness 實測的磁碟峰值，
   `M_safety` 是**寫死的固定餘裕**（⛔ 不得執行期解讀）。
   ⚠️ **核心實作完成後，正式 memory harness 還要再驗一次「實際生產流程的磁碟峰值
   ⛔ 沒有超出 sizing harness 的結果」**——⛔ 不能只驗記憶體。

   **acceptance 門檻：< 450 MiB**，在正式執行**之前**通過。
   ⚠️ 唯一一次正式 Stage 2 再記錄**實際全路徑峰值**，⛔ **但不為了量測而重跑**。
   ⚠️ 這是 capacity acceptance，⛔ **不適合當環境無關的一般 CI 單元測試**。
   ⚠️ 現行實測 **524 MiB（還沒算 replay）**，⛔ 不得用 `crossday.py` 的數字代替。

2. **v3 比較路徑的測試矩陣**（⚠️ **v18 取代 v4～v15 的 a～p**）：

   ⛔ **舊矩陣 a～p 測的是 candidate mismatch 的第二趟讀取與 disk spool**——
   ⚠️ v3 下那條路徑⛔ 不存在，相關測試⛔ **不新增**；
   ⚠️ `tests/test_i074_mismatch.py` 的**既有**案例⛔ **原樣保留**（它釘的是一般路徑）。

   | # | 案例 | 要驗什麼 |
   |---|---|---|
   | a | 全量 keys 兩側一致 | 守門通過，進入逐列比較 |
   | b | before **少一個／多一個／換序** key | ⛔ **fail-closed**（既有 `assert_same_keys`，⛔ **不得因為改成 cohort 比較而被繞過**） |
   | c | `before_candidates == ∅` | ⚠️ **正常路徑，⛔ 不得中止**（這正是 v2 卡住的那一條） |
   | d | `before_candidates` **非空** | ⛔ **中止** ＋ 有界診斷；⛔ **不產 `candidate_mismatch.json`**、⛔ **不回 rc=4** |
   | e | cohort 的某 key 在 before 缺席 | ⛔ fail-closed；⚠️ 要釘住**守門順序**（b 先於逐列比較） |
   | f | 156 列**全部**出現在 comparison | ⛔ 不截斷、⛔ 不抽樣（不變條件 e） |
   | g | 串流 loader 的逐列驗證 | ⚠️ 與非串流版**同強度**：`validate_after_artifact`／`validate_diagnostics` 的既有測試**全部續跑** |
   | h | artifact raw SHA 與 cohort manifest 不符 | ⛔ **fail-closed** |
   | i | validator iterator **重複消費** | ⛔ 不得少驗一道（可重新開啟的 factory 或單趟複合驗證） |
   | j | staging 內寫入／驗證／fsync 失敗 | **rc=1**、⛔ **無正式 archive**；⚠️ staging ⛔ 不得被視為證據 |
   | k | archive rename 成功、**parent dir fsync 失敗** | **rc=3**、⚠️ **保留 archive** |
   | l | archive 已 durable、**輔助 temp 清理失敗** | ⚠️ **仍回 0**——⛔ 不得降成一般失敗 |
   | m | **rc=3 的 recovery** | 用 archive 內的 before source artifact ＋ manifest SHA ＋ Stage 1 已封存 after artifact 重驗，⛔ **不 replay、⛔ 不重建**；成功後**恢復 manifest 記的 terminal outcome** |
   | n | **ENOSPC** 與 **orphan 清理** | 事前檢查要擋下；orphan ⛔ 不得被當成正式證據 |

   ⚠️ **模式旗標與兩份 patch 的測試（v19 新增）**：

   | # | 案例 | 要驗什麼 |
   |---|---|---|
   | o | ⛔ **未帶** `--i074-counterfactual` | ⚠️ **一般 Stage 2 逐項不變**：集合相等檢查、`candidate_mismatch.json`、rc=4 都還在 |
   | p | flag 帶在 **Stage 1** | ⛔ 中止（stage 限定，`assert_i074_flags()`） |
   | q | flag **重複出現** | ⛔ 中止（`_reject_duplicate_i074_flags()`） |
   | r | 使用者自行注入 script-injected 參數 | ⛔ 中止（CLI ownership） |
   | s | 兩份 patch 的宣稱 SHA 與**實際套用結果**不符 | ⛔ **fail-closed** |
   | t | **交換套用順序** | ⚠️ **runner 結構上⛔ 不提供這個入口**；測試改驗「**兩份增量 SHA 與 manifest 的 ordered components 一致**」——⛔ **不得斷言「合成 hash 必然不同」**（改不同檔案時會相同） |
   | u | `TOOLING_PATCH` 為空、只有 counterfactual | ⚠️ 兩個 SHA 都要有明確值（⛔ 不得省略欄位） |
   | v | flag 開啟但 `COUNTERFACTUAL_PATCH` 為空 | ⛔ 中止 |
   | w | flag 關閉但 `COUNTERFACTUAL_PATCH` 非空 | ⛔ 中止 |
   | x | flag 開啟時的 `I074_MODE` | ⚠️ **必須**要求 `REPLAY_IMAGE_ID`／run identity，⛔ 不自動 pin |
   | y | **Python CLI 的成對守門**（五條，見「二、④」） | ⚠️ flag 無 SHA／SHA 無 flag／Stage 1 帶 SHA／SHA 非 64 位小寫 hex 一律⛔ 中止；⚠️ **官方 runner 與直接 CLI 兩條路徑都要測** |
   | z | `before_candidates != ∅` 時的 **failed-attempt record** | ⚠️ 有產出且**可辨識**、綁住兩份 patch SHA 與 run identity；⛔ **無正式 evidence archive**；⚠️ 同 SHA 重跑⛔ 被拒 |

3. ⛔ **「唯一產品語意變因」要靠 differential guard 證明，⛔ 不是靠「測試全綠」**（v19 新增）：

   ⚠️ **counterfactual patch 本身就會改到測試與 fixture**——測試與預期**一起改**之後全綠，
   ⛔ **證明不了沒有夾帶其他語意變更**。所以要有一道**對照式**驗證：

   | # | 條件 | 要求 |
   |---|---|---|
   | a | 同一份 fixture 分別在**原始 `e1cbbbd`** 與 **patched `e1cbbbd`** 執行 | 產出逐欄對照 |
   | b | **非目標案例**——⚠️ **原始 `e1cbbbd` 的 `rr_decoupling_candidate == false`**（⛔ **不是**用「RR 不合格 ＋ 價格證據成立」自行判斷） | ⛔ **決策輸出 projection 必須完全相同** |
   | c | **RR 合格**的 `CONTINUATION` 列 | ⛔ **完全相同** |
   | d | **目標案例**——⚠️ **原始 `e1cbbbd` 的 `rr_decoupling_candidate == true`**，且 **patched 側必須為 `false`** | ⚠️ **只允許 lifecycle 的完整下游依賴閉包改變**（逐欄列於下表），其餘欄位⛔ 相同 |
   | e | 上游輸入：`setup_rr_qualified`、`event_signal`／`event_state_summary`、`structure_state`、primary zone 選擇 | ⛔ **必須完全相同**——⚠️ 不同就代表 patch 溢出到 lifecycle 之外 |

   ⛔ **guard 的 comparison projection 必須是精確的 allowlist（v21）**——⛔ 不能沿用
   `compare_rows()`（`artifacts.py:439`）的全欄位比對：它的 docstring 明寫「⛔ 不挑欄位比」，
   而**合法的 lifecycle 翻轉⛔ 本來就會連帶改動下游欄位**，全欄位比會把**合法 patch 判成不合格**。

   ⚠️ **方向必須寫死，⛔ 不得寫反**：

   ```text id="i074_guard_direction_001"
   formal before ＝ **patched** e1cbbbd（RR 已加回）
   formal after  ＝ **原始** e1cbbbd（＝正式 after 的 base，RR 已解耦）
   ```

   ⚠️ 也就是說：**guard 的「before → after」＝「patched → 原始」**，
   ⛔ **⛔ 不是「原始 → patched」**——寫反的話每一欄的期望轉換都會顛倒。

   **允許改變的 JSON path（⚠️ 窮舉，⛔ 未列出的決策欄位一律必須相同）**：

   | JSON path | 非 AVOID 的期望轉換 | AVOID 的期望 |
   |---|---|---|
   | `decision_derived_view.semantic_pipeline.lifecycle_phase` | `TESTING`／`CONFIRMED` → `CONTINUATION` | 同左 |
   | `decision_derived_view.semantic_pipeline.market_state` | `BULLISH_RECOVERY` → `BULLISH_CONTINUATION` | ⚠️ 同左（⛔ 但下游被 AVOID 短路） |
   | `decision_derived_view.semantic_pipeline.action_state` | `CONDITIONAL_HOLD`／`HOLD` → `HOLD` | ⛔ **不變**（`AVOID`） |
   | ⚠️ `decision_derived_view.semantic_pipeline.bias_state`（v22 補） | `BULLISH_BIAS` → `BULLISH_CONTINUATION` | ⛔ **不變**（`BEARISH_BIAS`） |
   | ⚠️ `decision_derived_view.semantic_pipeline.reason_codes`（v22 補，⚠️ **v23 訂正值域**） | ⚠️ **契約：把 `PRICE_UPSIDE_FOLLOW_THROUGH` 從兩側移除後，剩下的 list 必須逐項相同（⚠️ 含順序）**；方向是 **before 無、after 有**。⛔ **不得硬編任何 RR code**——`_rr_gate()` 實際產出的是 `RR_INSUFFICIENT`／`RR_UNAVAILABLE`／`NO_PRIMARY_ZONE`（`decision_engine.py:1373`），`RR_NOT_QUALIFIED` 只是 `:1104` 在 reason_code 為空時的 fallback | 同左 |
   | ⚠️ `decision_derived_view.semantic_pipeline.rr_decoupling_candidate`（v22 補） | ⚠️ **`false` → `true`**——⚠️ 這正是 `before_candidates == ∅` 的基礎，⛔ 漏列會讓合法 patch 被判失敗 | 同左 |
   | `decision_derived_view.bias_state` | `BULLISH_BIAS` → `BULLISH_CONTINUATION` | ⛔ **不變**（`BEARISH_BIAS`） |
   | `decision_derived_view.bias_label` | 隨 `bias_state`（`_market_bias_label()`） | ⛔ 不變 |
   | `decision_derived_view.bias_reason_codes` | `SEMANTIC_BULLISH_RECOVERY` → `SEMANTIC_BULLISH_CONTINUATION` | ⛔ **不變**（`MARKET_ACTION_AVOID`） |
   | `decision_derived_view.authority_reason_codes` | ⚠️ **只允許 `bias_reason_codes` 那一項的差異**——其餘來源（daily／final_entry／path／position）⛔ 必須相同 | ⛔ 不變 |
   | `decision_derived_view.position_gate_state` | 隨 `action_state`（`= semantic_pipeline["action_state"]`） | ⛔ 不變 |
   | `market_bias` ／ `market_bias_label`（top-level） | `BULLISH_BIAS` → `BULLISH_CONTINUATION` ＋ 其 label | ⛔ **不變**（`BEARISH_BIAS`） |
   | `position_action_condition.state` | 隨 `action_state` | ⛔ 不變 |

   ⛔ **`position_action_condition.reason_codes` ⛔ 不在允許清單**（v21 訂正 v20）：
   實查 `_position_action_condition()` 取的是 `derived_view["position_reason_codes"]`，
   而它的分支只看 `active_bearish_states`／`structure_state`／`primary_zone`／`rr_gate`／
   `daily_reason_codes`／`blocking_zone_ahead`——⛔ **⛔ 沒有一項是 `lifecycle_phase`**。
   ⚠️ **放寬它反而會遮住 patch 外溢。**

   | projection | 規則 |
   |---|---|
   | **非目標案例**（b／c） | ⚠️ **決策輸出 projection 完全相同**（⛔ 上表也⛔ 不得有差異）；⚠️ 分類依 **原始側的 `rr_decoupling_candidate`**，⛔ 不自行重組條件 |
   | **目標案例**（d）的上游輸入 | ⛔ **完全相同**：`setup_rr_qualified`、`event_signal`／`event_state_summary`、`structure_state`、primary zone 選擇、`rr_gate` |
   | **未列出的決策欄位** | ⛔ **一律必須相同**——⚠️ 新增任何一條 allowlist 都要先證明它**依賴 `lifecycle_phase`** |
   | ⛔ **⛔ 不比較** | provenance、`project_modules_sha256`、`generated_at` 等**必然不同**的執行資訊 |

   ⛔ **guard 自己要有負向測試（v23 補）**——⚠️ 「實際 patch 恰好跑得過」⛔ **證明不了
   guard 會擋**。⚠️ 這是「唯一產品語意變因」的核心 validator，⛔ 不能只有正向路徑。

   **表格驅動的正向矩陣**（⚠️ 九列，⛔ **全部必須 pass**：四種目標 ＋ 五種對照）：

   | # | 類別 | 形狀 | 原始側 `rr_decoupling_candidate` | 期望 |
   |---|---|---|---|---|
   | 1 | **目標** | before `TESTING` ＋ 非 AVOID | `true` | ✅ pass（⚠️ 只允許閉包內的差異） |
   | 2 | **目標** | before `CONFIRMED` ＋ 非 AVOID | `true` | ✅ pass |
   | 3 | **目標** | before `TESTING` ＋ AVOID | `true` | ✅ pass |
   | 4 | **目標** | before `CONFIRMED` ＋ AVOID | `true` | ✅ pass |
   | 5 | 對照 | **RR 合格**的 `CONTINUATION` | `false` | ✅ pass，且⛔ **輸出完全相同** |
   | 6 | 對照（⚠️ 像目標但不是） | 價格證據成立 ＋ RR 不合格，但 **`event_signal != CLOSE_RECLAIM`** | `false`（⛔ 走不到 `CONTINUATION` 分支） | ✅ pass，且⛔ **完全相同** |
   | 7 | 對照（⚠️ 像目標但不是） | **`active_bearish_states` 命中** | `false`（⚠️ **高優先失敗分支**先攔截） | ✅ pass，且⛔ **完全相同** |
   | 8 | 對照（⚠️ 像目標但不是） | **`structure_state == SUPPORT_RECLAIM_INVALIDATED`** | `false`（→ `INVALIDATED`） | ✅ pass，且⛔ **完全相同** |
   | 9 | 對照（⚠️ 像目標但不是） | **`structure_state == BREAKDOWN`** | `false`（→ `BREAKDOWN`） | ✅ pass，且⛔ **完全相同** |

   ⚠️ **6～9 是「⛔ 用近似 predicate 分類會出錯」的反例**——它們都符合
   「價格證據 ＋ RR 不合格」卻⛔ 不是 candidate；⚠️ 同時它們**證明 counterfactual patch
   ⛔ 沒有改壞 lifecycle 的優先序**。

   **每一類至少一個 tamper，⛔ 全部必須 fail-closed**：

   | # | tamper | 要驗 |
   |---|---|---|
   | a | **轉換方向寫反**（原始 → patched） | ⛔ 中止——⚠️ 同時釘死 `patched before → original after` |
   | b | allowlist 欄位出現**非預期值**（例如 `lifecycle_phase` 變成 `BREAKDOWN`） | ⛔ 中止 |
   | c | **未列出的欄位**被改（例如 `structure_state`） | ⛔ 中止 |
   | d | `position_action_condition.reason_codes` 被改 | ⛔ 中止（⚠️ 它⛔ 不在 allowlist） |
   | e | semantic `reason_codes` **多出另一個 code** | ⛔ 中止（⚠️ 逐項相同的契約，⛔ 不只看 `PRICE_UPSIDE_FOLLOW_THROUGH`） |
   | f | `rr_decoupling_candidate` **沒有 `false` → `true`** | ⛔ 中止 |

   ⚠️ **這道 guard 要排在 ⑨ 封存 patch 之前**，⛔ 不是事後補；
   ⚠️ 它用的 fixture ⛔ **不得**是 patch 一併修改的那些。

4. **兩種 patch 的可重建性**（⚠️ **分開驗**，見「三之一」與「三之一之二」）：
   `counterfactual_patch` ——斷言「套用 stored patch 後的增量 `git diff --binary` SHA」等於
   ⚠️ **Stage 2 evidence manifest** 記錄的值（⛔ **不是** provenance——⛔ 不得加第 11 欄）；
   `tooling_patch` ——若存在則同樣斷言，且⛔ **不得含任何判定變更**。
   ⚠️ **合成 hash** 才與 replay artifact 既有的 `tooling_patch_sha256` 對照。
5. **逐列驗證強度**：現有 `validate_diagnostics`／`validate_after_artifact` 的測試**全部續跑**。
   ⚠️ **另加一道 artifact 欄位存在性斷言（v20）**：B／C 判讀用到的五個欄位
   （`lifecycle_phase`／`market_bias`／`action_state`／`position_action_condition.state`／`final_entry_state`）
   **必須都在已封存的 after artifact 裡**——⛔ 判讀規則⛔ 不得引用 artifact 沒有的欄位。
6. **謂詞**：⛔ **兩側都不重算**——after 用已封存的 156 列，before 用同一個謂詞
   （套 patch 後恆 `False`），⛔ 不引入第三種定義。
7. **counterfactual patch**：套用前後 **`e1cbbbd`** 既有測試全綠
   （⚠️ **必要但⛔ 不充分**——⛔ 充分性由第 3 項的 differential guard 負責）；
   九個診斷欄位的 schema 由 `validate_diagnostics(side="before")` 驗。
8. **端到端**：`smoke-replay-offline.sh` 的 Stage 2 路徑照跑。

##### 七、完成後的歸檔位置

| 內容 | 歸檔到 |
|---|---|
| 串流比對的理由與峰值數字 | `development-workflow.md`（比照「凍結 bundle 不得重產」那兩節） |
| **Stage 2 counterfactual 的 v3 比較模型** ⚠️ **v18 取代原「mismatch streaming contract」** | `sr-zone-scoring.md`——⚠️ 它現在保存的是「兩側 candidate 集合必須相同」與 **in-memory validator 契約**，⛔ **兩者都要改**：<br>① **after 的 156 keys 是唯一 cohort、before 全量 replay**；② **全量 key 守門保留、candidate 集合相等要求移除**；③ **before candidate 預期恆空**，非空＝patch 失效；④ **一趟串流 loader，驗證強度⛔ 不得下降**；⑤ **archive 是唯一 commit point，terminal outcome 由 manifest 記錄（v3 恆為 0）**；⑥ ⚠️ **`candidate_mismatch.json`／rc=4 ⛔ 不適用 counterfactual 路徑**（一般路徑的契約⛔ 不動） |
| 九個診斷欄位的 before／after 對稱契約 | `sr-zone-scoring.md`「九個診斷欄位的完整 schema」 |
| ⚠️ **artifact 判讀矩陣**（140／16 兩類）與「哪些欄位不可觀測」 | `sr-zone-scoring.md`——⚠️ 它現在寫的是 `market_state`／`entry_permission_state`，⛔ **那兩個⛔ 不在 replay row 裡**，必須改成實有欄位並註明推導含意 |
| B／C 判定結果與證據 SHA | `issue.md` I-074，⛔ 在決策樹走完之前不得移除本筆 |

##### 八、執行順序

```text id="i074_stage2_order_001"
① 裁決採方案 B（before source artifact）              ← ✅ **已確認 2026-09-22**
② **counterfactual patch ／ v3 比較模型設計**（⛔ after 側不重算）
   ⚠️ **必須排在 evidence contract 之前**——before source artifact 的
      schema、守門條件與 validator **全都依賴這個設計**
   ⚠️ **同一步要一併裁定（v19）**：
      ⒜ `--i074-counterfactual` 的 opt-in 語意與 stage 限定（「二、④」）
      ⒝ 兩份 patch 的輸入、**固定套用順序**、增量 diff SHA 與合成 hash（「三之一之二」）
      ⒞ 兩份 SHA 記在 **Stage 2 evidence manifest**（⛔ 不動 10 欄的 `PROVENANCE_FIELDS`）
③ **Stage 2 evidence contract**（before source artifact ＋ **counterfactual patch**
   ＋ tooling patch；⚠️ **兩種 patch 分開記錄、獨立 SHA**）
   ⚠️ **必備輸出還有 failed-attempt record**（「二、①」）——⛔ 它⛔ 不是成功 archive
   計畫書 → 確認 → 實作 → review
   ⚠️ 要裁定**原子邊界**：單一 evidence archive 一次性發布，
      recovery 從 manifest 讀回原 terminal outcome（⚠️ v3 之下恆為 rc=0）
④ 實作獨立 sizing harness（⛔ 不含正式 preflight）
⑤ 實測並記錄 **P_B** 的磁碟／記憶體峰值，裁定 **M_safety = <固定 bytes>**
⑥ 更新計畫並**再次確認**
⑦ 實作：`--i074-counterfactual` opt-in、兩份 patch 的 runner／SHA 機制、
   一趟串流 loader、`before_candidates == ∅` 守門、preflight、發布、recovery
   ＋ memory harness：< 450 MiB
⑧ 測試矩陣 a～z（⚠️ 含 **o：未帶 flag 時一般路徑逐項不變**、**v／w：truth table**、**y：Python 成對守門**、**z：failed-attempt record**）
⑨ **differential guard**（「六、3」）→ 產生並封存 exact counterfactual patch，驗三方 SHA
   （＋ **`e1cbbbd`** 既有測試套用 patch 前後全綠）
⑩ **唯一一次**正式 Stage 2（約 180 分鐘）      ← ⚠️ 開跑後進入凍結窗口
⑪ 判讀 B／C 與歸檔
```

⚠️ **①～⑨ 都是分鐘～小時級且可獨立驗證**，⛔ 不要跳過直接跑 **⑩**——
Stage 1 的教訓是「跑了 183 分鐘才發現容量不夠」。
⚠️ 另外 **③ 未完成前⛔ 不得進入 ⑦**——發布順序與 schema 還沒定，實作會白做；
而 **② 未完成前⛔ 不得進入 ③**——比較模型決定 schema 與 validator。
⚠️ **⑩ 是唯一一次**：峰值驗收已裁決走 **memory harness**（步驟 ⑦），
全路徑峰值只在 **⑩** 記錄，⛔ **不作為 ⑩ 的前置**，也⛔ **不為量測而重跑**。

#### ② before tooling／反事實謂詞設計 v2（2026-09-22，⚠️ **待 review**）

⛔ **v1 的比較基準是錯的**，v2 重新裁決。以下先列證據。

##### ⛔ 阻擋 v1 的事實：`ecbc141^` 與正式 after 之間有 95 個 commit

| 事實 | 值 |
|---|---|
| before 基準（v1 設想） | `ecbc141^` |
| 正式 after artifact 綁定的 base_commit | **`e1cbbbdab44f8cf2d152e6ade9235d844f590d7f`** |
| 兩者距離 | **95 個 commit** |
| 其中動過 `lifecycle_engine.py`／`event_engine.py` 的 | 至少 **8 個**（`b445c24` T-048 C、`b113dcb` T-048 D、`fcd0ffa` I-077、`ac01775`、`b17ec59` I-096／I-098、`306dff8`、`d337da1` Stage 0…） |
| `SUPPORT_TEST_CANDIDATE` 何時引入 | `b17ec59`／`ac01775`——⚠️ **`ecbc141^` 完全沒有這個值** |

**具體的 confounding witness**（⚠️ 是正式 after 中**已存在的混淆形狀**，
⛔ **尚未真的用 `ecbc141^` replay 過該列**——要當成鐵證需補 bounded fixture）：

```text id="i074_basis_counterexample_001"
2330  as_of=2024-08-05T16:00:00+00:00
  structure_state                  SUPPORT_TEST_CANDIDATE   ← 舊版沒有這個值
  event_signal                     SUPPORT_TEST             ← 舊版會是 CLOSE_RECLAIM
  continuation_price_evidence_met  True
  clear_zone_breakout              True
  setup_rr_qualified               False
  rr_decoupling_candidate          False
```

⚠️ **條件式陳述**（⛔ 尚未用舊版 replay 該列，⛔ 不要寫成已證實的逐列結果）：
**若**舊版對該列選到相同的 primary zone／interaction，
**則** touched-only 會被命名為 `SUPPORT_RECLAIM_CANDIDATE` → `CLOSE_RECLAIM`，
於是反事實謂詞成立、形成 **before-only candidate**。
⛔ **那個差異的成因會是 `structure_state` 的語意變動，⛔ 不是 RR 解耦**
——這正是不能用 `ecbc141^` 當基準的理由。

⚠️ **`_rr_gate()` 兩版逐字相同這件事仍然成立**（AST 取出比對，各 24 行），
⛔ **但它只證明「相同輸入得到相同結果」**，⛔ **證明不了兩版實際傳入的
`primary_zone`／`entry_action_state` 相同**，更推不出 candidate 集合等價。

##### v2 裁決：**以正式 after base `e1cbbbd` 為共同基準**

| | v1（⛔ 已否決） | **v2** |
|---|---|---|
| before 基準 | `ecbc141^`（95 commit 之前） | **`e1cbbbd`**（＝正式 after 的 base） |
| 差異內容 | **95 個 commit 的所有累積變動** | ⚠️ **唯一產品語意變因是 setup RR 條件**（⛔ 實作會跨 lifecycle 參數、呼叫端、判定式與測試） |
| 差異可歸因於 RR 嗎 | ⛔ **不能** | ✅ **可以**——單一變因 |
| 需要的 patch | 整套 bundle CLI ＋ 九欄位 tooling | ⚠️ **只需 counterfactual patch**（`e1cbbbd` 已有 bundle CLI 與九欄位） |

⚠️ **counterfactual patch 的實際範圍——⛔ 不是「只改一處」**：
正式 base 的 `resolve_lifecycle()` **參數裡刻意沒有 RR**（那正是 T-044 抽離的重點），
而 `rr_qualified` 是在 **lifecycle 呼叫完成之後**才於 semantic pipeline 取得
（`decision_engine.py`：1044 呼叫 lifecycle、1055 才算 `rr_qualified`）。
所以精確恢復舊行為**至少涉及**：

| # | 改動 |
|---|---|
| 1 | 先取得 **setup RR** |
| 2 | 把它**傳進** counterfactual 的 lifecycle 路徑 |
| 3 | 在 `CONTINUATION` 條件**加回** RR |
| 4 | 更新**直接呼叫 lifecycle 的測試／fixture** |
| 5 | ⚠️ 明確處理「RR 不合格時**繼續落入** `CONFIRMED` 或 `TESTING`」——⛔ **不能事後只把 `CONTINUATION` 改成固定值** |

⚠️ 可以維持「**唯一的產品語意差異只有 RR 條件**」，
⛔ **但不得宣稱程式 diff 只有一行或一個位置**。
⚠️ 該 patch 要做成**獨立、可驗證的 patch**（⚠️ **SHA 進 Stage 2 evidence manifest**——⛔ **不是** provenance；patch 本體進 evidence。見計畫書「三之一之二」）。

⚠️ **這改變了 I-074 的驗證語意，要講清楚**：

* v1 想驗的是「**T-044 抽離前後**的行為差異」——⛔ 但那無法與 95 個 commit 的其他變動分離；
* v2 驗的是「**RR 條件本身**對 `CONTINUATION` 的影響」——⚠️ 這才是 I-074 立案時真正關心的
  （`rr_gate.qualified` 從 `CONTINUATION` 判定條件移除，影響多大）。

⛔ **若堅持用 `ecbc141^`**，結論只能寫成「所有累積版本差異」，
⛔ **不得歸因於 RR 解耦**，也⛔ **不得把集合不一致稱為 tooling asymmetry**。

##### 反事實謂詞（v2：兩版都在 `e1cbbbd` 基準上）

⚠️ v2 之下 after 側**完全不變**（用已封存的 `rr_decoupling_candidate`）；
before 側是 `e1cbbbd` ＋ counterfactual patch，其 `CONTINUATION` 恢復含 RR，
因此 **before 的 candidate 定義與 after 相同**——
⚠️ 差別只在**同一列在兩版落到不同的 `lifecycle_phase`**，那正是要量的東西。

⛔ **所以 v1 那段「before 專用的展開式」不再需要**：
兩版用同一個謂詞 `lifecycle_phase == "CONTINUATION" and not setup_rr_qualified`，
⚠️ 在 before 版它恆為 `False`（RR 已加回），在 after 版是那 156 列。

###### ⛔ 但「不對稱本身就是結論」⛔ 行不通——現行實作會在比較**之前**就 rc=4 結束

⚠️ `evaluation.py` 的**第五道集合檢查**排在逐列比較**之前**：

```python id="i074_fifth_check_blocker_001"
before_candidates = candidate_keys(rows)
after_candidates = candidate_keys(after_rows)
if sorted(before_candidates) != sorted(after_candidates):
    _publish_candidate_mismatch(...)      # → rc=4，**直接中止**
```

⚠️ v2 預期 before 恆空、after 有 156 → **必然觸發** →
⛔ **根本跑不到那 156 列的逐列比較**，也就完成不了 I-074 的原目標
（觀察 `lifecycle_phase`／`action_state`／持倉建議**怎麼翻轉**）。

**v3 的比較模型**（✅ **已整合進 Stage 2 計畫書 v18**，2026-09-22）：

| 項目 | 做法 |
|---|---|
| cohort | ⚠️ **after 已封存的 156 keys 是唯一 cohort** |
| before replay | ⚠️ **仍跑全量 13,417 列**——⛔ 不能只跑 156 列，否則事件狀態不連續 |
| 第一道驗證 | **全量 13,417 keys 兩側一致** |
| 逐列比較 | 從 before 取出 **after cohort 那 156 keys** 逐列比 |
| candidate 集合 | ⛔ **不再要求兩側相同**——⚠️ before 預期為空，**那是 RR 條件存在時的正確行為** |
| ✅ **已重新裁定（v18）** | 第五道集合檢查改成 **`before_candidates == ∅`**；`candidate_mismatch.json` 與 rc=4 ⛔ **不適用本路徑**（常數與既有測試保留，出現即視為實作缺陷）——見計畫書「二、①」 |

##### 九個診斷欄位（⚠️ v1 寫成十個，v2 訂正）

⚠️ **`lifecycle_phase` ⛔ 不在九欄內**——它是另一個既有的必驗欄位。
九欄的定義見 `replay_bundle/artifacts.py` 的 `validate_diagnostics()`：
**4 個 boolean ＋ 3 個 string ＋ `position_action_condition` ＋ `position_action`**。

⚠️ **產出位置 v1 也寫錯了**（「全部落在同一處」）：

| 欄位 | 實際產出位置 |
|---|---|
| `action_state` | semantic pipeline 內 |
| `position_action`／`position_action_condition` | ⚠️ **外層 `build_decision_summary()` 組裝**，⛔ **不在** `_decision_semantic_pipeline` 內 |

⛔ **這點必須修正，否則 patch 會從錯誤的層級取值。**
⚠️ 不過 **v2 之下這個風險大幅降低**——`e1cbbbd` 本來就有完整的九欄位輸出，
counterfactual patch ⛔ 不需要重建它們。

##### 可重現的查證紀錄

| 項目 | 值 |
|---|---|
| after base ref | `e1cbbbdab44f8cf2d152e6ade9235d844f590d7f` |
| 正式 D+1 artifact SHA | `33a6b1666487abcfc88353020afbd1e35a8cbab71b40c9ef17ea1194a8cc8b8c` |
| `_rr_gate()` 比較方式 | AST 取出兩個 ref 的函式本體逐字比對（`ecbc141^` 1261-1284 vs `e1cbbbd` 1373-1396，各 24 行） |
| ⚠️ 該比較的限制 | **只保證函式本體相同**，⛔ **不保證輸入相同、⛔ 不保證逐列輸出一致** |
| 已知的非 RR 差異 | `structure_state` 的 `SUPPORT_TEST_CANDIDATE`（`b17ec59`／`ac01775` 引入）；⚠️ **95 commit 的完整清單尚未逐一盤點**——v2 改用 `e1cbbbd` 基準後⛔ 不再需要 |

##### 驗收條件（v2）

| # | 條件 |
|---|---|
| 1 | ⚠️ **唯一產品語意變因是 setup RR 條件**——diff 必須逐行可讀，⛔ **但⛔ 不得要求「只有一處」**（見上方 patch 範圍的五項） |
| 2 | 套用 patch **前後**，`e1cbbbd` 的既有測試**全綠** |
| 3 | ⚠️ 兩份 patch 的 SHA 進 **Stage 2 evidence manifest**（⛔ **不是** 10 欄的 `PROVENANCE_FIELDS`）、**patch 本體進 evidence**（見「三之一之二」與「三之二」） |
| 4 | ⚠️ before 的 candidate 集合**預期恆為空**——⚠️ 那不是錯誤，是 RR 條件存在時的正確行為 |

⛔ **本步驟⛔ 不跑 replay**。

#### ② 執行結果（2026-09-22，⚠️ **待 review**）

✅ **計畫書 v25 已確認，② 依序執行完畢**（⛔ 本步驟沒有跑 replay）。

##### 反事實 patch 的實際範圍

| 檔案 | 真正的程式碼改動 | 性質 |
|---|---|---|
| `lifecycle_engine.py` | ⚠️ **兩行**：`resolve_lifecycle()` 新增必填參數 `setup_rr_qualified: bool`；`CONTINUATION` 分支加 `and setup_rr_qualified` | ⚠️ **唯一的產品語意變因** |
| `decision_engine.py` | ⚠️ **兩行搬移 ＋ 一行傳參**：`rr_qualified = bool((rr_gate or {}).get("qualified"))` 從呼叫**後**移到呼叫**前**，並以 `setup_rr_qualified=rr_qualified` 傳入 | ⚠️ 純取值時機搬移——`rr_gate` 在此之前就已算完，⛔ 值不受影響 |
| `tests/test_lifecycle_engine.py` | 14 個呼叫點補參數；兩支 RR 專屬測試改寫 | 測試對齊 |
| `tests/test_i074_diagnostics.py` | helper 補參數；真值表首列改 `False`；**新增一支正向測試** | 測試對齊 ＋ 防假綠 |

⚠️ **其餘都是註解與 docstring**（每一處都標了 `I-074 COUNTERFACTUAL PATCH`，⛔ 讓它⛔ 不可能被誤認為主線行為）。

##### ⚠️ 執行中發現：只翻轉真值表**⛔ 不夠**

⚠️ 套上 patch 後唯一失敗的既有測試是
`test_pipeline_candidate_truth_table[CONTINUATION-False-True]`——
⚠️ **那正是 patch 要改變的語意**（`rr_decoupling_candidate` 在 before 側變成不可達）。
⛔ **但把它改成 `False` 就收工是假綠**：⚠️ 全 `False` 有兩種成因，
①RR 條件真的加回來了（要的）、②pipeline 整個壞掉走不到 `CONTINUATION`（⛔ 不要的）。
所以另補一支**正向**測試 `test_counterfactual_continuation_requires_setup_rr()`：
同一組價格證據下，**RR 合格 → `CONTINUATION`、RR 不合格 → `CONFIRMED`**。

##### 驗收條件對照

| # | 條件 | 結果 |
|---|---|---|
| 1 | 唯一產品語意變因是 setup RR 條件 | ✅ 產品程式碼實改 **5 行**（2 ＋ 3），逐行可讀；⛔ 其餘為註解、測試與參數傳遞 |
| 2 | 套用 patch 前後 `e1cbbbd` 既有測試全綠 | ⚠️ **範圍要講清楚**：**Python pytest 全量通過**（baseline 1222 passed／1 skipped；patched 1223 passed／1 skipped），⛔ **shell 的 argv 測試本步驟未執行**（`SKIP_SHELL_TESTS=1`）。理由：patch ⛔ 沒有碰 `scripts/`，那一段兩側完全相同 |
| 3 | patch 的 SHA 與本體 | ✅ SHA 見下方「⒝」；本體與三份測試 log 暫存於 `python/baselines/i074_stage2/`（⚠️ **正式位置由步驟 ③ 的 evidence contract 決定**） |
| 4 | before 的 candidate 集合預期恆空 | ✅ 真值表四格全 `False` ＋ 等價式斷言，⚠️ **在 fixture 層已成立**；⛔ 全量證明要等 ⑩ |

##### 測試的執行身分（⚠️ **可複核**）

⚠️ **兩側走的是同一個驗證入口**——⛔ 光記「1222 → 1223」證明不了這件事：

| 項目 | baseline | patched（第 1 次） | patched（第 2 次） |
|---|---|---|---|
| tree SHA | `6bbd804e49e329dd85103aa2f510ec07760a9609`（＝`e1cbbbd^{tree}`） | ⛔ **未保存**（見下方說明） | `a1649733b910ae90f08efc5f215f31f898619f52` |
| 入口 | `python/scripts/test.sh`（**worktree 自己那份**，⛔ 不是主線的） | 同左 | 同左 |
| 參數 | ⛔ **無參數**（預設 `backtest/ tests/` 全量） | 同左 | 同左 |
| 環境 | `SKIP_SHELL_TESTS=1`、`PY_IMAGE=stock-trading-python-e1cbbbd:test` | 同左 | 同左 |
| ⚠️ **image ID**（⛔ **三次⛔ 不同**） | `sha256:c6f6cd5ef1fee48f7b8daf87b2615cd52e576301808d74be9a93d455e70c8167` | `sha256:e81b17fefe961f6118ebdae370a5a9fb9254f010422c04a401e76d4d472c436b` | `sha256:51cdac600c28d91c43970ced98db3dad9ffc624607747494b0a275611b579955` |
| 結果 | **1222 passed, 1 skipped** | ⛔ **1 failed, 1221 passed, 1 skipped** | **1223 passed, 1 skipped** |
| exit code | **0** | ⛔ **1** | **0** |
| log | `tests_baseline.log`<br>`927300e5…8141` | `tests_patched_run1_failed.log`<br>`f36743ba…4ee0` | `tests_patched_run2.log`<br>`7adf7826…8a27` |

⛔ **⛔ 不能寫成「同一顆 image」**（2026-09-22 訂正）：`python/Dockerfile` 會
`COPY` 原始碼進去，來源內容不同，最終 image 本來就不同——三次的 tag 相同但 ID 不同，
⚠️ 而 `test.sh` 每次都會重新 `docker build`。**真正相同的是**：
同一份 `Dockerfile`、同一組 base 與 dependency layer、
以及三份 log 都記到的 **Python 3.11.16 ／ pytest-9.1.1**。

⚠️ **patched 第 1 次的 tree ⛔ 沒有保存**：那次是把 patch 套上去、尚未補正向測試的中間態，
⛔ 已不可重建。⚠️ **它只作為「失敗曾經發生」的歷史 log，⛔ 不列為可重現的驗收證據**——
驗收採 baseline 與 patched 第 2 次這兩個有 tree SHA 的執行。

⚠️ **log 三份都封存在 `python/baselines/i074_stage2/`**（含**失敗那次**——
⛔ 失敗紀錄不得只留在對話裡，它正是「只翻轉真值表不夠」的證據）。
⚠️ `SKIP_SHELL_TESTS=1` 的理由：那一段驗的是 **replay 腳本的 argv 所有權**，
與反事實 patch ⛔ 無關（patch ⛔ 沒有碰 `scripts/`），且兩側完全相同。
⛔ **所以⛔ 不能說「既有測試全綠」**——⚠️ 精確的說法是
**「Python pytest 全量通過；shell argv 測試本步驟未執行」**。

⚠️ **另有一次先跑的局部驗證**（只跑 `backtest/modular/sr_scoring/tests`，998 passed／1 skipped），
⛔ **不作為驗收依據**。⚠️ **驗收採 baseline 與 patched 第 2 次**（兩者都有 tree SHA、
都可重現）；**patched 第 1 次只保留為歷史失敗紀錄**。

##### ⚠️ patch 檔的保存格式：⛔ 不得做 whitespace 正規化

⚠️ `git diff --cached --check` 會在 patch 檔報 **27 處 trailing whitespace**——
⚠️ 那是 unified diff 對**空白 context 行**使用的**單獨一個空格**前綴，⛔ **不是損壞**。
⛔ **批次 trim 會同時破壞套用能力、並改掉上表記錄的 patch SHA。**

**處置**：新增 `.gitattributes`，**逐副檔名**（`*.patch`／`*.log`，⛔ 不是整個目錄）指定
`-text`（⛔ **關掉換行正規化**——⚠️ 只寫 `-whitespace` 只關診斷，跨平台 checkout 仍可能改掉
位元組）、`-whitespace`，log 另加 `conflict-marker-size`
（⚠️ pytest 的 `=======` 分隔線剛好達到預設門檻 7）。
⛔ **⛔ 不用 `**` 涵蓋整個目錄**：之後 Stage 2 的 JSON artifact 會放進同一個目錄，
⚠️ 那些⛔ 不該跟著繞過 whitespace 檢查。
⚠️ **這只改 git 的檢查行為，⛔ 一個位元組都沒動**——patch SHA 仍是 `ef7a4cdf…4f93`。
⚠️ **正式保存路徑與格式仍由步驟 ③ 的 evidence contract 定案。**

##### ⒝ 兩份 patch 的 SHA 機制——**實測可行**

⚠️ v25 的 `git write-tree` 方案已在真實 worktree 跑過：

| 項目 | 值 |
|---|---|
| base commit | `e1cbbbdab44f8cf2d152e6ade9235d844f590d7f` |
| 中繼 tree `T1` | `a1649733b910ae90f08efc5f215f31f898619f52` |
| `counterfactual_patch_sha256` | `ef7a4cdf23dce70cda77abe8d49cdcc8e3a3829c0212b4202e747b343c4b4f93` |
| `tooling_patch_sha256`（本次為空） | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`（空字串的 SHA，⚠️ 與 smoke 既有慣例一致） |
| 合成 hash | 同 counterfactual（⚠️ tooling 為空時必然相等） |

✅ **三項關鍵性質都驗過**：
① **HEAD 全程等於 base**——⛔ 沒有中繼 commit，
⛔ 不牴觸 `replay-args.sh:148` 的 `replay_args_prepare_worktree()` HEAD 斷言；
② **stored patch 重新套用後的增量 diff SHA 等於原值**（三方一致）；
③ 空 tooling patch 的 SHA 有明確值，⛔ 不是缺欄位。

##### 仍未做（⛔ 不屬 ②）

⛔ **replay ⛔ 沒有跑**。⚠️ `--i074-counterfactual`、`--counterfactual-patch-sha256`
與 runner 的兩份 patch 輸入**都還沒實作**——它們屬於 ⑦，
⚠️ 而 schema 與 manifest 欄位要等 ③ 的 evidence contract 定案。

#### ③ Stage 2 evidence contract 計畫書 v9（2026-09-23，⚠️ **待確認**）

⚠️ **v9 收掉兩個一致性缺口**：

| # | 缺口 | v9 的處置 |
|---|---|---|
| 1 | ⛔ **`az`／`ba` 的階段歸屬與完成矩陣不一致**——`az` 是本輪 `validate_stage1_anchor_graph()` 的必要測試卻被漏掉；`ba` 需要 runner 實作卻⛔ 沒像 `ax` 標成 ⑦ | ⚠️ 本輪 ＝ `a`～`aw` ＋ `ay` ＋ **`az`**；⑦ ＝ **`ax` ＋ `ba`** |
| 2 | ⚠️ snapshot ⛔ 沒有 `manifest_payload`，第二支 helper **⛔ 無從分辨自己比的是 manifest 欄位還是 identity 欄位** | ⚠️ snapshot **保存同次讀取的 `manifest_payload`**（與其他 payload 一致） |

#### （承上）③ evidence contract 計畫書 v8 的內容

⚠️ **v8 修掉一個範圍錯置、一個信任錨缺口與一處未定義行為**：

| # | 缺口 | v8 的處置 |
|---|---|---|
| 1 | ⛔ **測試 ax 在本階段做不到**——它要驗 runner 端的凍結，⚠️ 但凍結實作屬 Stage 2 步驟 ⑦，本階段⛔ 不改 runner | ⚠️ **ax 移出本輪完成條件**，改列為 **⑦ 的驗收**；本輪矩陣＝ `a～aw`、`ay` |
| 2 | ⛔ **信任錨少了 selected members 的 bundle／image 鏈**——單檔 validator ⛔ 只驗型別與 role，⚠️ **擋不住「各檔都合法、bundle／image 卻各自不同」** | ⚠️ 明列兩條等式鏈進 `validate_stage1_anchor_graph()` |
| 3 | ⚠️ **空 `TOOLING_PATCH` 的凍結行為未定義**——⛔ 空值時沒有路徑可讀 | ⚠️ 明訂建立 **0-byte 凍結副本** |

#### （承上）③ evidence contract 計畫書 v7 的內容

⚠️ **v7 補掉兩個阻擋點**：

| # | 缺口 | v7 的處置 |
|---|---|---|
| 1 | ⛔ **preflight 與 replay 之間仍有 patch TOCTOU**——checker 驗 patch A，runner 可能套到被換掉的 patch B，⚠️ **重跑守門與 canonical 檢查雙雙被繞過** | ⚠️ 裁定 runner **一開始就把兩份 patch 凍結成私有副本**，下游一律只用凍結 bytes，見「七之三」 |
| 2 | ⛔ **`load_stage1_anchor()` 的偽碼⛔ 沒涵蓋第 6／8／9 道**——那三道是**跨檔關係**（Stage 1 是放在 `verify_evidence_graph()`，⛔ 不在 `validate_evidence_manifest()` 裡） | ⚠️ 拆成 **`load_stage1_anchor()` ＋ `validate_stage1_anchor_graph()`**，⚠️ 三個呼叫端**都要呼叫兩支**，見「七之二」 |

#### （承上）③ evidence contract 計畫書 v6 的內容

⚠️ **v6 裁決兩件 v5 留成半開放的事**：

| # | 缺口 | v6 的處置 |
|---|---|---|
| 1 | ⛔ **非 canonical 輸入 patch 只標示⛔ 不拒絕**，⚠️ 與正式 archive 的無條件要求衝突——⛔ 那種輸入會**跑完數小時 replay 才在 finalizer 被拒** | ⚠️ 改成**先查找、未命中即 rc=1 拒絕**，見「七之一」 |
| 2 | ⚠️ snapshot ⛔ 沒有 `base_commit`，checker 的 base 推導**無從取得**；⛔ 且「共用同一份 snapshot」在**跨程序**下不成立 | ⚠️ snapshot 改為**保存同一次讀取的 payload**，措辭改成**共用 helper 與 schema、每次重新載入** |

#### （承上）③ evidence contract 計畫書 v5 的內容

⚠️ **v5 修掉 v4 的一個 preflight 漏判與一個資料來源缺口**：

| # | 缺口 | v5 的處置 |
|---|---|---|
| 1 | ⛔ **checker 的比對鍵與正式契約⛔ 不是同一個值**——v4 直接雜湊 patch 檔案 bytes，⚠️ 但契約是 `sha256(git diff --binary <base> <T1>)`。⛔ **同一份變更用不同文字表示就能繞過「同一份壞 patch 不得重跑」** | ⚠️ checker 改用**與 runner 完全相同的推導**，見「七之一」 |
| 2 | ⛔ **preflight 時還沒有 Stage 2 manifest**，⚠️ 「先跑第 1～9 道」沒有宣告值可比 | ⚠️ 抽出共用的 **`load_stage1_anchor()` snapshot**，見「七之二」 |
| 3 | ⚠️ v4 摘要稱新增「七支測試」，實際 `ak`～`ar` 是**八支** | ⚠️ 訂正 |

#### （承上）③ evidence contract 計畫書 v4 的內容

⚠️ **v4 收掉 `--check-failed-record` 的 CLI 契約缺口**（⛔ v3 只給了指令名，
⚠️ **沒有定義輸入、結束碼與 runner 的呼叫義務**——那樣實作者無從下手）。

| # | 缺口 | v4 的處置 |
|---|---|---|
| 1 | ⛔ **入口⛔ 不知道「這次要比對的 patch」是什麼**；⛔ 結果也沒有回傳契約 | ⚠️ 定義完整 CLI 契約，見「七之一」 |
| 2 | ⚠️ 受影響檔案與測試矩陣沒同步新入口 | ⚠️ 補 `--check-failed-record` 與**八支**測試 |
| 3 | ⚠️ 內文已引用「v4 裁決」，標題卻還是 v3 | ⚠️ 本段即 v4，v3 降為「承上」 |

#### （承上）③ evidence contract 計畫書 v3 的內容

⚠️ **v3 修掉 v2 review 的兩個阻擋點、兩個缺口與四處文件殘留**：

| # | 缺口 | v3 的處置 |
|---|---|---|
| 1 | ⛔ **信任錨只錨了 after，graph 卻用未錨定的 cohort**；⚠️ 且 `manifest_path` 只是「例如」，⛔ 可以指向另一份同 bundle／image 的 manifest | ⚠️ 改成**封閉的 `members` mapping**（after ＋ cohort 兩份都錨）＋ **路徑寫死常數**，見「三之一」 |
| 2 | ⛔ **before source 的全量守門依賴 archive 裡⛔ 不存在的 bundle universe**——⚠️ recovery ⛔ 不讀 operational 來源，**這條守門在 recovery 時根本跑不了** | ⚠️ 改成「**等於已錨定的 Stage 1 D+1 after 全量 keys**」，13,417 降為**預期觀測值**，見「三之二 ①」 |
| 3 | ⚠️ 跨檔身分還有六條沒封閉（`before_ref`／`base_commit`／`timeframe`／provenance 逐欄／`rows_shown`／keys 排序唯一） | ⚠️ 補進「五之零」與各份 schema |
| 4 | ⚠️ failed record 只有欄位名稱，子結構與交叉綁定不完整 | ⚠️ 補**不變條件 F1～F9**（產品契約，⛔ 不是測試清單），見「七」 |

⚠️ **文件殘留四處**：recovery 的「7 檔 ＋ manifest」（會變 8 檔）、發布流程的「7 個來源」、
failed 表格被截斷的最後一列、風險表誤稱 nested `-text`「② 已加」（⚠️ **實際上是本步驟才要加**）。

#### （承上）③ evidence contract 計畫書 v2 的內容

⚠️ **v2 補齊 v1 review 的三個阻擋點與兩個缺口**（⛔ v1 宣稱「定義 layout、schema 與 recovery」，
⚠️ **但只真的定義了 manifest**）：

| # | 缺口 | v2 的處置 |
|---|---|---|
| 1 | ⛔ **四份 canonical artifact 的 schema 與跨檔 graph 沒定義**——實查 `artifacts.py` 只有 comparison／report 的 **builder，⛔ 沒有 validator** | ⚠️ 新增「三之二」逐份定義 ＋「五」定義 `verify_stage2_graph()` 的七道 |
| 2 | ⛔ **Stage 1 after 的引用沒綁到已封存的 Stage 1 evidence**——只記 path ＋ 兩個 SHA，⚠️ **任何一份合法 after artifact 都能寫出相符的 SHA** | ⚠️ 改以 **Stage 1 `evidence_manifest.json` 為信任錨**，見「三之一」 |
| 3 | ⛔ **正式保存根目錄未裁定**，且 `.gitattributes` 的 `*.patch` ⛔ **不涵蓋巢狀路徑**（實查 `patch/counterfactual.patch` 的 `text` 是 `unspecified`——⚠️ **v1 宣稱的位元組保證當時並不成立**） | ⚠️ 裁定三個根目錄 ＋ 改用 `**` 規則，見「三之零」 |
| 4 | ⚠️ failed-attempt record 還不是封閉可 recovery 的契約；⛔ 且 `<run_id>` **根本不存在**（`run_identity.py:32` 的 `RUN_IDENTITY_FIELDS` 只有 5 欄） | ⚠️ 改用 `<bundle_id>-<sha[:12]>` 命名 ＋ 封閉 schema ＋ durability 出口，見「七」 |
| 5 | ⚠️ 「三方 SHA 關係」只比對**宣告值**，⛔ 沒有證明兩份 patch 真的合成得出 `composed_sha256` | ⚠️ 裁定由 **shell finalizer 在隔離 worktree 實際重算**，並寫明各層各證明了什麼，見「四之二」 |

⚠️ **另修**：layout 是 **6 個 payload ＋ manifest ＝ 7 檔**（v1 誤寫「7 檔 ＋ manifest」）；
`raw_blob.stored_bytes` 補值域規則。

#### （承上）③ evidence contract 計畫書 v1 的內容

⚠️ **這是 Stage 2 計畫書步驟 ③ 的產物**，⛔ 不是它的一部分：Stage 2 計畫書只定義
「需要一套新的 evidence contract」與驗收條件，**實際的 layout、schema、發布與 recovery 在這裡定**。
⛔ **本計畫書確認前⛔ 不得進入步驟 ⑦**（實作），否則發布順序與 schema 還沒定、實作會白做。

##### 一、目標與⛔ 不做的範圍

**目標**：定義 **Stage 2 counterfactual 執行的證據契約**——
能原子發布、能獨立複核、能在 durability 未確認時 recovery，
並且**容得下 Stage 1 裝不進去的三樣東西**：before source artifact、兩份 patch 本體、
以及失敗時的 failed-attempt record。

| ⛔ 不做 | 理由 |
|---|---|
| ⛔ **不改 Stage 1 的 `evidence.py`** | ⚠️ **本計畫最高風險**：`ARCHIVE_LAYOUT` 是 9 檔封閉、`validate_evidence_manifest()` 與 `recover_durability()` 都做**精確集合相等**——動它會讓 `python/baselines/i074_stage1/` 那份**已封存的證據立刻驗不過** |
| ⛔ 不複製第二套原子發布原語 | `publish.py` 的 `rename_noreplace`／`fsync_dir`／`probe_no_clobber`／`DurabilityUnconfirmed` **共用**，⛔ 不 fork |
| ⛔ 不改 canonical JSON／gzip 的位元組輸出 | 所有既有 SHA 的根 |
| ⛔ 不改 10 欄的 `PROVENANCE_FIELDS` | 見 Stage 2 計畫書「三之一之二」——加第 11 欄會讓已封存的 Stage 1 artifact 驗不過 |
| ⛔ 不實作 runner 端 | `--i074-counterfactual`、兩份 patch 的輸入與 `write-tree` SHA 計算是**步驟 ⑦**；⚠️ ③ 只定**它們要產出什麼 schema** |
| ⛔ 不跑 replay | 同 ② |

##### 二、為什麼⛔ 不能重用 Stage 1 的 evidence 模組

| 事實 | 位置 | 後果 |
|---|---|---|
| `ARCHIVE_LAYOUT` 是**寫死的 9 個相對路徑** | `python/backtest/modular/sr_scoring/replay_bundle/evidence.py:62` 的 `ARCHIVE_LAYOUT` | Stage 2 的檔案集合完全不同 |
| `_MANIFEST_FIELDS` 是**封閉 7 欄** | `python/backtest/modular/sr_scoring/replay_bundle/evidence.py:89` 的 `_MANIFEST_FIELDS` | 放不下兩份 patch 的 SHA 與 ordered components |
| `finalize_evidence()` 斷言 `set(sources)` **恰好等於** 9 個路徑 | `python/backtest/modular/sr_scoring/replay_bundle/evidence.py:292` 的 `finalize_evidence()` | 多一個 before source artifact 就中止 |
| `recover_durability()` 斷言實際檔案集合**恰好等於** 9 ＋ manifest | `python/backtest/modular/sr_scoring/replay_bundle/evidence.py:412` 的 `recover_durability()` | 同上 |
| 每個 archive 項都經 `load_canonical_evidence_artifact(path, kind)` | 同上 | ⚠️ **patch 是 raw bytes、⛔ 不是 canonical JSON**，這條路走不通 |

✅ **裁決：新增 `replay_bundle/stage2_evidence.py`**，自己的 layout／manifest kind／validator／
finalizer／recovery；⚠️ **共用**發布原語、canonical 編碼與 `validate_provenance()`。

##### 三之零、⛔ 三個根目錄的裁定（v2 新增）

⛔ **`python/baselines/i074_stage2/` ⛔ 不能直接當 evidence root**：
①`rename_noreplace` 要求目標**不存在**，而它已經有 ② 的四個檔案；
②那四個檔案會違反封閉檔案集合。

| 用途 | 路徑 | 說明 |
|---|---|---|
| ② 的產物 | `python/baselines/i074_stage2/*.patch`、`*.log` | ⚠️ **留在第一層⛔ 不搬**——⛔ 它們⛔ 不受封閉集合管轄（封閉集合只管 `evidence/` 內部） |
| **成功 archive** | `python/baselines/i074_stage2/evidence/` | ⚠️ 發布時才由 `rename_noreplace` 建立；parent 是 `i074_stage2/` |
| **failed-attempt** | `python/baselines/i074_stage2/failed/<bundle_id>-<counterfactual_patch_sha256>/`（⚠️ **完整 64 碼**） | 見「七」 |

⚠️ **`.gitattributes` 必須改**（⛔ v1 的規則對巢狀無效，已實查）：

```text id="i074_stage2_gitattributes_001"
python/baselines/i074_stage2/**/*.patch -text -whitespace
python/baselines/i074_stage2/**/*.log   -text -whitespace conflict-marker-size=200
```

⚠️ 這同時涵蓋 `evidence/patch/*.patch` 與 `failed/*/patch/*.patch`。
⛔ **`.gitattributes` 要列進受影響檔案**（v1 漏了）。

##### 三、archive layout（⚠️ **封閉集合**）

```text id="i074_stage2_layout_001"
<stage2 evidence root>/
  before/before_source_artifact.json.gz     ← before 全量 13,417 列
  comparison/comparison_artifact.json.gz    ← cohort 156 列的逐列比較
  comparison/report.json.gz                 ← 人看的前 200 列
  patch/counterfactual.patch                ← ⚠️ **raw bytes，⛔ 不壓縮、⛔ 不包進 JSON**
  patch/tooling.patch                       ← ⚠️ 同上；**可為 0 bytes**
  identity/run_identity.json.gz
  evidence_manifest.json                    ← root，⛔ 不壓縮（它是索引）
```

⛔ **多一個未知檔案即中止**（比照 Stage 1）。⚠️ **6 個 payload ＋ manifest ＝ 7 檔。**

⚠️ **patch 為什麼是 raw bytes**：①`git apply` 要能直接吃；
②包進 JSON 會被 escape，**位元組就不再等於 `git diff --binary` 的輸出**，
⛔ 三方 SHA 關係當場斷掉（② 已實測那三者相等）；
③ 已用 `.gitattributes` 的 `-text` 保證 repo 端⛔ 不做換行正規化。

##### 三之一、Stage 1 after artifact 的引用：⛔ **以 Stage 1 manifest 為信任錨**

⚠️ recovery 需要「已封存的 Stage 1 after artifact」當對照的一側。**⛔ 不複製進來**
（14 MB 重複、且製造第二份真相，與「⛔ 不動 Stage 1 已封存的證據」相衝）。

⛔ **但 v1 的做法⛔ 不夠**：只記 `path` ＋ 兩個 SHA、再重讀比對，
**只能證明「指到的檔案沒變」，⛔ 證明不了「它就是正式 Stage 1 evidence 裡的那一份」**
——⚠️ **任何一份合法的 after artifact 都能在 finalize 當下被寫出相符的 SHA。**

✅ **v2 裁決：信任錨是 Stage 1 的 `evidence_manifest.json`。**

| manifest 欄位 | 內容 |
|---|---|
| `stage1_evidence.manifest_path` | ⚠️ **寫死常數** `python/baselines/i074_stage1/evidence_manifest.json`——⛔ **不是執行期參數**。⚠️ I-074 是**一次性正式驗證**，⛔ 不需要通用性；留成可指定的話，⚠️ 另一份同 bundle／同 image 的合法 manifest 也能充數 |
| `stage1_evidence.manifest_sha256` | 該檔的 SHA-256 |
| ⚠️ `stage1_evidence.members` | ⚠️ **封閉 mapping，⛔ 不是單一 member**：<br>`d1/after_artifact.json.gz` → `{artifact_sha256, stored_sha256}`<br>`d1/cohort_manifest.json.gz` → `{artifact_sha256, stored_sha256}`<br>`identity/run_identity.json.gz` → `{artifact_sha256, stored_sha256}`<br>⛔ key 集合**恰好是這三個**，⛔ 不可增減（⚠️ **v4 把 identity 也納入**——下方第 10 道要用它） |

**finalize 與 recovery 都要做的十道**：

| # | 檢查 |
|---|---|
| 1 | `manifest_path` **必須等於寫死常數**；讀它並**重算 SHA-256** 比對 `manifest_sha256` |
| 2 | ⚠️ 用 **Stage 1 既有的 `validate_evidence_manifest()`** 驗它（⛔ 不自己寫一份寬鬆版） |
| 3 | `members` 的 key 集合**恰好是** after ＋ cohort ＋ identity 三個 |
| 4 | 三份的兩個 SHA **都必須等於 Stage 1 manifest 裡對應 entry 的值** |
| 5 | ⚠️ 實際讀這三個檔並**重算**兩個 SHA，與 4 的值一致 |
| 6 | ⚠️ **Stage 1 manifest 的 `bundle_id`／`expected_image_id` 必須等於 Stage 2 的**——⛔ 否則可以拿別次執行的 evidence 來充數 |
| 7 | ⚠️ **cohort 本身要用既有的 `validate_cohort_manifest()` 驗**，⛔ 不只比 SHA |
| 8 | ⚠️ cohort 的 `after_artifact_sha256` **必須等於已錨定 after 的 `artifact_sha256`** |
| 9 | ⚠️ cohort 的 keys **必須等於該 after 的 `candidate_keys(rows)`**——⛔ 比照 Stage 1 `verify_evidence_graph()` 的同一道，⛔ 不重算 predicate |
| 10 | ⚠️ **Stage 2 的 `run_identity` 必須與已錨定的 Stage 1 archived identity 逐位元相同**——`validate_run_identity()` 驗過後**整個 object 相等**。⚠️ **理由見下** |

⛔ **為什麼是「完全相等」而⛔ 不是「bundle 與 image 相同就好」**（v4 裁決）：
⚠️ failed record 的**目錄鍵**是 `<bundle_id>-<counterfactual_patch_sha256>`，
而重跑守門的**查找鍵**原本是「完整 run identity ＋ 完整 SHA」——⛔ **兩者不一致**：
同 bundle／同 patch 但**不同 identity** 的執行會被放行，跑完卻必然撞上同一個發布目錄。
⚠️ 把 identity 釘成「必須等於 Stage 1 archived identity」之後，**identity 不再是變數**，
兩個鍵就一致了。⛔ 另一條路（把 identity digest 放進目錄名）⛔ **不採**——
⚠️ 那等於允許「換個 identity 就能重跑同一份壞 patch」，與「**必須改 patch 才可重跑**」直接衝突。
⚠️ **這一道要在 replay 之前的 preflight 就擋**，⛔ 不是跑完才發現。

⛔ **錨定不等於「驗過了」**：已錨定的 after ⛔ **不得只比 SHA 就直接餵進 graph**——
⚠️ 它仍要跑 `validate_after_artifact()`、`validate_replay_errors()`、
`validate_diagnostics(side="after")` 與 `validate_provenance(role="stage1")`；
cohort 同理跑 `validate_cohort_manifest()` 與 provenance 驗證；identity 跑 `validate_run_identity()`。
⚠️ **理由**：SHA 只證明「與 Stage 1 manifest 記的是同一份」，
⛔ 證明不了那份**內容本身**在 Stage 2 的使用情境下仍然合法。

⚠️ **路徑安全（⛔ 不得省）**：`manifest_path` 必須拒絕**絕對路徑**、含 `..` 的成分、
以及**解析後逃出 repo root** 的路徑；⚠️ 逐段檢查 symlink。
⚠️ 既有實作可參考 `scripts/check-doc-refs.py` 的 `_is_acceptable_local_file()`
（同樣的威脅模型，⛔ 不要另造一套語意）。

⛔ **已知代價**：Stage 2 archive ⛔ **不自足**——⚠️ 它與 `python/baselines/i074_stage1/`
**必須一起保存**，單獨搬走會斷鏈。⚠️ 這一條要進 `development-workflow.md`。

##### 三之二、四份 canonical artifact 的 schema（v2 新增）

⛔ **v1 只定義了 manifest，⚠️ 那樣 `verify_stage2_graph()` 沒有可實作的契約**，
recovery 也只能驗「檔案與 hash 沒變」，⛔ **證明不了內容一開始就是對的**。

⚠️ 實查現況：`artifacts.py` 有 `build_comparison_artifact()`／`build_report()`，
⛔ **但⛔ 沒有對應的 validator**（`validate_after_artifact`／`validate_cohort_manifest`／
`validate_candidate_mismatch` 都有，這兩份沒有）。**所以要新寫。**

**① `before/before_source_artifact.json.gz`**

| 項目 | 規格 |
|---|---|
| kind | ⚠️ **新 kind** `sr_zone_stage2_before_source`（⛔ 不冒用 `AFTER_KIND`——它是 after 側的語意） |
| 封閉欄位 | `schema_version`／`kind`／`bundle_id`／`before_ref`／`timeframe`／`replay_scope`／`generated_at`／`provenance`／`rows` |
| row validator | ⚠️ **沿用既有的三道**：`assert_unique_keys()`、`validate_replay_errors()`、`validate_diagnostics(side="before")` ——⛔ 不新寫一套 row schema |
| ⛔ **全量守門** | ⚠️ keys 必須**唯一**、**固定排序**，且**恰好等於已錨定的 Stage 1 D+1 after 的全量 keys**。⛔ **不得寫成「等於 bundle universe」**——⚠️ Stage 2 archive 裡**沒有** bundle manifest，而 recovery ⛔ 不讀 operational 來源，**那條守門在 recovery 時根本跑不了** |
| 列數 | ⚠️ **13,417 是本次的預期觀測值**，⛔ **不是完整性證明**——完整性由上一列的**兩側 key 集合相等**負責 |
| provenance role | `stage1`（⚠️ 它就是一次 replay 執行；⛔ 不新增 role，那會動到封閉的 `PROVENANCE_ROLES`） |
| ⚠️ 額外守門 | **`candidate_keys(rows)` 必須為空**——⚠️ 這是 `before_candidates == ∅` 在**證據層**的重複確認（⛔ 執行期擋過一次⛔ 不代表封存的這份也對） |

**② `comparison/comparison_artifact.json.gz`**

| 項目 | 規格 |
|---|---|
| kind | 沿用 `COMPARISON_KIND` |
| 封閉欄位 | `schema_version`／`kind`／`bundle_id`／`before_ref`／`after_artifact_sha256`／`generated_at`／`provenance`／`rows`（⚠️ 即 `build_comparison_artifact()` 的既有輸出） |
| 每列欄位 | `symbol`／`timeframe`／`as_of`／`differences`／`before`／`after` |
| ⛔ **來源比對** | ⚠️ **`before` 必須逐鍵等於 before source artifact 中同 key 的那一列**；**`after` 必須逐鍵等於 Stage 1 D+1 after artifact 中同 key 的那一列**——⛔ **不是「相似」，是相等** |
| ⛔ **differences 重算** | ⚠️ 用 `compare_rows()` 的同一套邏輯**重算**並比對，⛔ 不信任封存值 |
| ⛔ **cohort 一致** | ⚠️ keys **恰好等於 Stage 1 D+1 `cohort_manifest` 的 keys**（156），⛔ 不多不少 |
| `after_artifact_sha256` | 必須等於 `stage1_evidence.members["d1/after_artifact.json.gz"].artifact_sha256` |

**③ `comparison/report.json.gz`**

| 項目 | 規格 |
|---|---|
| kind | 沿用 `REPORT_KIND` |
| 封閉欄位 | `build_report()` 的既有輸出欄位 |
| `comparison_artifact_sha256` | 必須等於實際 comparison 的 `artifact_sha256` |
| ⛔ **rows 是截斷** | ⚠️ 必須是 comparison rows **依 `(symbol, timeframe, as_of)` 排序後的前 `rows_shown` 列**，⛔ 不得是抽樣或另一種排序 |
| ⛔ **統計重算** | `candidate_rows` 與 `difference_field_counts` 必須**由完整 comparison 重算**得到，⛔ 不得由截斷後的 rows 算 |

**④ `identity/run_identity.json.gz`**：沿用 Stage 1 的 `validate_run_identity()`，⛔ 不改。

##### 四、manifest schema

| 欄位 | 內容 |
|---|---|
| `schema_version` | 沿用 `ARTIFACT_SCHEMA_VERSION` |
| `kind` | `sr_zone_stage2_evidence_manifest`（⚠️ **新 kind**，⛔ 不與 Stage 1 共用） |
| `bundle_id` | 綁 archived identity，⛔ 不符即中止 |
| `expected_image_id` | 同 Stage 1 語意 |
| `files` | ⚠️ **兩種 entry 型別**，見下 |
| `patches` | `{counterfactual_patch_sha256, tooling_patch_sha256, composed_sha256, ordered_components}` |
| `stage1_evidence` | ⚠️ **以 Stage 1 manifest 為信任錨**：`manifest_path`（寫死常數）、`manifest_sha256`、**封閉的 `members` mapping**（after ＋ cohort ＋ **identity** 各 `{artifact_sha256, stored_sha256}`）——⛔ v1 的 `{path, artifact_sha256, stored_sha256}` 已作廢 |
| `terminal_outcome` | ⚠️ **v3 之下恆為 `0`**——⛔ 仍要寫、recovery ⛔ 仍要讀回，⛔ 不得寫死在程式裡 |
| `generated_at` | 同 Stage 1 |
| `finalizer_provenance` | `validate_provenance(role="finalizer")`，⛔ 仍是 10 欄 |

⚠️ **`files` 的兩種 entry**（⛔ Stage 1 只有一種）：

| 型別 | 欄位 | 用於 |
|---|---|---|
| `canonical_artifact` | `artifact_sha256`（canonical JSON payload）、`stored_sha256`（gzip 後）、`stored_bytes` | 4 個 `.json.gz` |
| ⚠️ **`raw_blob`** | `stored_sha256`、`stored_bytes`（⛔ **沒有** `artifact_sha256`——raw bytes 沒有 canonical payload 這個概念） | 2 個 `.patch` |

⚠️ **`raw_blob.stored_bytes` 的值域（v2 補）**：⛔ 必須是**非負整數**且等於實際檔案大小；
**`patch/counterfactual.patch` ⛔ 不得為 0**（⚠️ 空的反事實 patch ＝ 根本沒做反事實）；
⚠️ **只有 `patch/tooling.patch` 允許 0 bytes**。

⛔ **型別由 layout 決定、⛔ 不由 manifest 自行宣告**——否則竄改者可以把 `.json.gz`
宣告成 `raw_blob` 來跳過 canonical 驗證。

##### 四之一、`patches` 的三方 SHA 關係（⚠️ **② 已實測**）

```text id="i074_stage2_patch_sha_001"
counterfactual_patch_sha256 = sha256(git diff --binary <base> <T1>)
tooling_patch_sha256        = sha256(git diff --binary <T1> <T2>)
composed_sha256             = sha256(git diff --binary <base> <T2>)
ordered_components          = ["counterfactual", "tooling"]   ← ⚠️ 順序即套用順序
```

**validator 必須斷言的四條**：

| # | 斷言 |
|---|---|
| 1 | `files["patch/counterfactual.patch"].stored_sha256` **等於** `patches.counterfactual_patch_sha256` |
| 2 | `files["patch/tooling.patch"].stored_sha256` **等於** `patches.tooling_patch_sha256` |
| 3 | `composed_sha256` **等於** before source artifact 的 `provenance.tooling_patch_sha256` ——⚠️ **這是「⛔ 不讓語意修改偽裝成 instrumentation」的那道綁定**（見 Stage 2 計畫書「三之一之二」） |
| 4 | `ordered_components` **恰好等於** `["counterfactual", "tooling"]` ——⛔ **順序由此強制，⛔ 不靠「hash 必然不同」**（兩份 patch 改不同檔案時交換順序會得到相同 final tree） |

⚠️ **tooling patch 為空時**：`tooling_patch_sha256` 是**空字串的 SHA**
`e3b0c442…b855`（② 實測，與 smoke 既有慣例一致），且 `composed == counterfactual`。
⛔ **⛔ 不得省略該欄位或寫 null。**

##### 四之二、⛔ **上面四條只驗「宣告值一致」，⛔ 證明不了合成關係**（v2 新增）

⚠️ 誠實地講：第 1～4 條證明的是
「兩份 raw patch 的 hash 與 manifest 相符」與「`composed_sha256` 與另一個宣告值相符」。
⛔ **它們⛔ 沒有證明** `base ＋ counterfactual ＋ tooling` **真的**合成得出 `composed_sha256`
——⚠️ 那正是「⛔ 不讓語意修改偽裝成 instrumentation」所依賴的關係。

✅ **v2 裁決：由 shell finalizer 在隔離 worktree 實際重算。**

| 層 | 證明什麼 | 為什麼放這一層 |
|---|---|---|
| **`finalize-stage2-evidence.sh`** | ⚠️ **可重建性**：在 `base_commit` 開**臨時 worktree**，依 `ordered_components` 套用兩份 patch，`git write-tree` 後重算三個 SHA，⛔ **任一不符即中止、⛔ 不呼叫 Python finalizer**。⚠️ **這道同時適用成功 archive 與 failed-attempt record**（見 F8-a）——⛔ 兩條路徑都封存 patch，⛔ 就都要證明它們合成得出 `composed_sha256` | git 與 worktree 是 shell 層既有能力；⛔ 把 git 依賴塞進 `replay_bundle` 會讓 evidence 模組變得不可純資料測試 |
| **`stage2_evidence.py`** | **內部一致性**：宣告值彼此相符、與實際 bytes 相符 | ⛔ 它⛔ 不碰 git |

⚠️ **臨時 worktree 的規則**：⛔ **不得建中繼 commit**（承 Stage 2 計畫書「三之一之二」——
會牴觸 `replay_args_prepare_worktree()` 的 HEAD 斷言），用 `git write-tree`；
⚠️ 用完**必須移除**（`git worktree remove --force` ＋ `prune`）。

⛔ **若日後決定不做這一步**，文件就**必須改寫成**「這項關係僅由 runner 宣告，
⛔ evidence validator 並未驗證」——⚠️ **⛔ 不得兩者都不做卻保留「已驗證」的說法。**

##### 五之零、`verify_stage2_graph()` 的十四道（v2 新增，v3 擴充）

⚠️ 比照 Stage 1 的 `verify_evidence_graph()`——⛔ **只驗「檔案集合與 SHA」擋不住
「各檔都合法、彼此卻對不起來」**：

| # | 檢查 |
|---|---|
| 1 | 四份 canonical artifact 各自的**完整 validator**（「三之二」） |
| 2 | **Stage 1 信任錨十道**（「三之一」） |
| 3 | comparison 的每列 `before`／`after` **等於來源 row**、`differences` **重算相符** |
| 4 | comparison 的 keys **恰好等於** Stage 1 D+1 cohort 的 156 keys |
| 5 | report 的 `comparison_artifact_sha256`、截斷與統計**重算相符** |
| 6 | **所有檔案同一個 `bundle_id`**，且等於 identity 與 Stage 1 manifest 的 |
| 7 | ⚠️ **逐一列舉**每份 provenance 的位置與 role，`image_digest` 必須等於 `expected_image_id`——⛔ 不用 `.get("provenance") or …` 通用推測（Stage 1 的教訓：那樣會整份漏掉 `comparator_provenance`）。⚠️ **mapping 見下表** |
| 8 | before source 與 comparison 的 **`before_ref` 相等**，且**等於兩者的 `provenance.base_commit`** |
| 9 | before source 的 **`timeframe`／`replay_scope`** 等於已錨定的 Stage 1 D+1 after |
| 10 | ⚠️ before source 與 comparison **來自同一次 runner invocation** → 兩份 `provenance` **逐欄相等**（⛔ 不只比 `image_digest`；⚠️ 比照 Stage 1 對 probe 三份的同一道） |
| 11 | report 的 **`before_ref`／`bundle_id`** 等於 comparison |
| 12 | ⚠️ **`rows_shown` 恰好等於 `min(200, candidate_rows)`**——⛔ **只寫「取前 `rows_shown` 列」會讓 `rows_shown = 0` 的空報告合法通過** |
| 13 | comparison 的 keys **已排序且唯一** |
| 14 | ⚠️ **Stage 2 專屬**：before source 的 `candidate_keys(rows)` **必須為空** |

⚠️ **第 7 道的 provenance mapping（⛔ 必須明列，⛔ 不得通用推測）**：

| 檔案 | 欄位 | role |
|---|---|---|
| `before/before_source_artifact.json.gz` | `provenance` | `stage1` |
| `comparison/comparison_artifact.json.gz` | `provenance` | `stage1` |
| `evidence_manifest.json` | `finalizer_provenance` | `finalizer` |

⛔ **⛔ 沒有 provenance 的三個**：`comparison/report.json.gz`（⚠️ 它靠
`comparison_artifact_sha256` 綁回 comparison）、`identity/run_identity.json.gz`
（⚠️ 身分宣告，⛔ 不是執行紀錄——比照 Stage 1）、以及兩份 `.patch`（raw bytes）。

⚠️ **上表是產品契約**，⛔ 不是測試清單——「十、測試與驗證策略」的案例**引用**這裡的編號，
⛔ 反過來把規則只寫在測試名稱裡是不行的（⚠️ 實作者不會知道精確期望值）。

##### 五、發布流程與 commit point

⚠️ **單一 evidence archive、一次性原子發布**（承 Stage 2 計畫書 v11 裁定），
⛔ 流程與 Stage 1 的 `finalize_evidence()` **同形**，只是 layout 不同：

```text id="i074_stage2_finalize_001"
階段 A：讀入 6 個 payload 來源 ＋ 全部驗證（所有 project import 在此發生）
        ＋ 依信任錨重讀 Stage 1 的 after／cohort／identity 三份，執行完整十道
階段 B：才建 finalizer_provenance 與 manifest
        → 寫 staging（4 個 canonical gz ＋ 2 個 raw patch ＋ manifest）
        → 重算 project_modules_sha256 守門（⛔ 階段 B 後不得有新 import）
        → _fsync_tree(staging)
        → rename_noreplace(staging, evidence_root)   ← ⚠️ **唯一的 commit point**
        → fsync parent
        → 回 manifest 的 terminal_outcome（⚠️ v3 恆為 0）
```

| 情況 | 正式 archive | 結束碼 |
|---|---|---|
| rename 之前任何失敗 | ⛔ **不存在**（staging 清掉） | **1** |
| rename ＋ parent fsync 都成功 | 存在且 durable | **0** |
| rename 成功、parent fsync 失敗 | ⚠️ **保留** | **3**（`EXIT_DURABILITY_UNCONFIRMED`） |

##### 六、recovery 契約

⚠️ 比照 `recover_durability()`，⛔ **禁止重新 replay、⛔ 禁止重建**：

| 項目 | 規則 |
|---|---|
| 輸入 | 既有的正式 archive；⛔ 不讀 operational 來源 |
| 檔案集合 | ⛔ **恰好 6 個 payload ＋ manifest ＝ 7 檔** |
| metadata | manifest 的每一項都要**從實際檔案重算**並比對（含 raw patch 的 `stored_sha256`） |
| Stage 1 信任錨 | ⚠️ **重讀 after ＋ cohort ＋ identity 三份**、重算各自兩個 SHA，並**完整執行「三之一」的十道**（含三份的完整 validator）——⛔ 不符即中止 |
| ⚠️ **完整 graph** | ⛔ **⛔ 不得只驗檔案與 hash**：載入四份 canonical artifact 後**完整執行「五之零」的十四道**（含各自的 validator）——⚠️ **在 fsync 之前**。⛔ 少了這道，recovery 只能證明「東西沒變」，⛔ 證明不了「它原本就是對的」 |
| 執行身分 | 比對 `RECOVERY_IDENTITY_FIELDS`，⚠️ **在 fsync 之前**比 |
| 成功 | ⚠️ **回 manifest 記的 `terminal_outcome`**（v3 恆為 0）——⛔ **不得寫死** |
| 再失敗 | ⛔ **不刪除既有 archive**；回 rc=1 並要求人工介入 |

##### 七、failed-attempt record（⚠️ **⛔ 不是成功 archive**）

⚠️ `before_candidates != ∅` 時**正式 archive ⛔ 不存在**，但事故紀錄仍必須落地。

| 項目 | 契約 |
|---|---|
| 位置 | ⚠️ **與成功 archive ⛔ 不同的根目錄**：`python/baselines/i074_stage2/failed/<bundle_id>-<counterfactual_patch_sha256>/`（⚠️ **完整 64 碼，⛔ 不截斷**——⛔ 截成 12 碼會讓不同的完整 SHA 映到同一路徑）。⛔ **v1 寫的 `<run_id>` 根本不存在**——`run_identity.py:32` 的 `RUN_IDENTITY_FIELDS` 只有 `schema_version`／`kind`／`bundle_id`／`expected_image_id`／`created_at` 五欄 |
| 檔案 | `failure_record.json`（⛔ 不壓縮）＋ `patch/counterfactual.patch` ＋ `patch/tooling.patch` |
| schema | ⚠️ **封閉欄位**：`schema_version`／`kind="sr_zone_stage2_failed_attempt"`／`bundle_id`／`expected_image_id`／`run_identity`／`patches`（同「四之一」四欄）／`files`（兩份 patch 的 `raw_blob` entry）／`bounded_diagnostics`／`failure_reason`／`generated_at`／`provenance`。⛔ 多欄、缺欄、型別不符一律中止；⚠️ **canonical JSON**（⛔ 不壓縮——它要能直接讀） |
| ⚠️ **檔案集合** | ⛔ **恰好** `failure_record.json` ＋ `patch/counterfactual.patch` ＋ `patch/tooling.patch`；多一個未知檔案即中止 |
| ⚠️ **patch 重新比對** | lookup 與 recovery 都要**重讀兩份 patch 的實際 bytes、重算 SHA**，與 `files`／`patches` 比對——⛔ 不只信 record 裡的宣告值 |
| `bounded_diagnostics` | ⚠️ **有界**：`before_candidate_count`（完整計數）＋ `sample_keys`。⚠️ **`sample_keys` 的規則寫死**：依 `(symbol, timeframe, as_of)` **排序後取前 `min(N, count)` 個**、⛔ **不得重複**、每項附該列的 `lifecycle_phase`／`setup_rr_qualified`；⚠️ **N 是寫死常數⛔ 不是執行期參數**，由測試釘住。⛔ **不存全差集** |
| 發布 | ⚠️ 同樣**原子 ＋ fsync**（共用 `publish.py` 原語） |
| ⛔ 與成功 archive 的關係 | ⛔ **不得**放進成功 archive 的 layout，⛔ 不得沿用其 manifest kind |

**failed record 的不變條件（⚠️ **產品契約**，⛔ 不是測試清單）**：

| # | 不變條件 |
|---|---|
| F1 | `run_identity` **必須通過既有的 `validate_run_identity()`** |
| F2 | 頂層 `bundle_id`／`expected_image_id` **等於 embedded identity 的同名欄位** |
| ⚠️ **F2-a** | ⚠️ **embedded `run_identity` 必須完整等於「固定 Stage 1 manifest 所錨定的 identity」**（信任錨第 10 道的同一份）——⛔ **只驗 schema 與頂層欄位是不夠的**：⚠️ 一份**只有 `created_at` 不同**、其餘完全合法的 record 照樣會通過 F1～F9，**查找鍵／目錄鍵的矛盾就又回來了**。⚠️ **發布、lookup、recovery 三處都要執行這一項** |
| F3 | `provenance` 用 **`role="stage1"`**；`image_digest` 等於 `expected_image_id`、`base_commit` 等於兩份 patch 所依附的 base、**`tooling_patch_sha256` 等於 `patches.composed_sha256`** |
| F4 | ⚠️ **目錄名 `<bundle_id>-<counterfactual_patch_sha256>`（完整 64 碼）必須與 record 內容相符**——⛔ 否則改個目錄名就能繞過 lookup。⚠️ identity 已由信任錨第 10 道與 **F2-a** 釘死，**目錄鍵與查找鍵因此一致** |
| F5 | `failure_reason` 是**封閉列舉**，目前唯一值 `before_candidates_nonempty`——⛔ **不接受任意字串** |
| F6 | `bounded_diagnostics` 的**欄位集合恰好是** `{before_candidate_count, sample_keys}`——⛔ 多欄、缺欄即中止 |
| F6-a | `before_candidate_count` 是**正整數**；`sample_keys` 長度**恰好** `min(N, count)`、依 `(symbol, timeframe, as_of)` **已排序**、⛔ 不重複 |
| F6-b | ⚠️ **`sample_keys[]` 每一項的欄位集合恰好是** `{symbol, timeframe, as_of, lifecycle_phase, setup_rr_qualified}`——⛔ 缺欄或多欄都中止（⚠️ F6-a 只限制長度與排序，⛔ 擋不住欄位缺漏） |
| F6-c | `symbol`／`timeframe`／`as_of`／`lifecycle_phase` 必須是**非空字串** |
| F7 | `sample_keys[]` 每項的 `setup_rr_qualified` 必須是**嚴格 boolean**——⛔ 不接受 `0`／`1` |
| F8 | 兩份 patch 的**實際 bytes** 重算 SHA 後與 `files`／`patches` 相符 |
| F8-a | ⚠️ **合成守門同樣適用**：發布 failed record **之前**，shell 端也要在隔離 worktree 依 `ordered_components` 套用兩份 patch 並重算 `composed_sha256`（見「四之二」）——⛔ **不符即⛔ 不發布**。⚠️ **⛔ 少了這道**，failed record 可能封存「各自 SHA 都對、卻合成不出宣告 composed hash」的一組 patch |
| F9 | 檔案集合⛔ **恰好** `failure_record.json` ＋ `patch/counterfactual.patch` ＋ `patch/tooling.patch` |

**重跑守門（⚠️ 必須在 replay 之前）**：

```text id="i074_stage2_rerun_gate_001"
preflight（⛔ replay 之前）：
  掃 <stage2 root>/failed/，找「同 run identity ＋ 同 counterfactual_patch_sha256」
  ⚠️ 「同 run identity」＝ **完整 identity object 逐欄相等**，⛔ 不只比 bundle_id
  命中 → ⛔ **立即中止**（⛔ 不得跑完三小時才拒絕）
  未命中 → 繼續
```

⚠️ **lookup 與 recovery 都要完整執行 F1～F9**（⛔ 不是籠統的「schema ＋ patch SHA」）——
⚠️ 包含 **F2-a** 的 identity 綁定、F4 的目錄名相符與 F8-a 的合成守門。

⛔ **但 F8-a 需要 git，而 `stage2_evidence.py` ⛔ 不碰 git**（見「四之二」）——
⚠️ **所以 lookup 必須有對應的 shell 入口**（⛔ v3 只定義了 `--recover-failed-record`）：

```text id="i074_stage2_lookup_entry_001"
finalize-stage2-evidence.sh --check-failed-record
  ① Python：F1～F9 中⛔ 不需要 git 的各項（含 F2-a、F4）
  ② shell ：F8-a——隔離 worktree 套兩份 patch，重算 composed_sha256
  ⚠️ 兩段都過才算「這份 record 有效」；任一段失敗 → ⛔ fail-closed
```

###### 七之一、`--check-failed-record` 的 CLI 契約（v4 新增）

| 項目 | 契約 |
|---|---|
| 用法 | `finalize-stage2-evidence.sh --check-failed-record --counterfactual-patch <path>` |
| failed root | ⚠️ **寫死常數** `python/baselines/i074_stage2/failed/`——⛔ **CLI 無覆寫參數**（⚠️ 比照 `run_identity.py` 的既有慣例：測試改為直接呼叫 Python API，⛔ 不開「只給測試用」的路徑參數） |
| ⛔ **本次的比對鍵** | ⚠️ **必須與 runner 用同一套推導**（⛔ v4 初稿「直接雜湊檔案 bytes」**是錯的**）：<br>① base 取自**已驗證的 Stage 1 after `provenance.base_commit`**；<br>② 在**隔離 worktree** 套用 `--counterfactual-patch`，`git write-tree` 得 `T1`；<br>③ 比對鍵 ＝ `sha256(git diff --binary <base> <T1>)`。<br>⚠️ **為什麼不能直接雜湊檔案**：⛔ 同一份變更可以用**不同的 patch 排序、header 或文字表示**套出相同的 `T1`——那時檔案 hash 不同但 canonical diff SHA 相同，⚠️ **checker 會誤判成「沒命中」而放行**，等於繞過「同一份壞 patch ⛔ 不得重跑」 |
| ⚠️ **非 canonical 輸入的裁決** | ⚠️ 正式 archive **無條件要求** `stored patch SHA == sha256(git diff --binary <base> <T1>)`，所以⛔ **不能只標示不拒絕**（⚠️ 那種輸入會跑完**數小時 replay** 才在 finalizer 被拒）。**順序固定為**：<br>① 先用**重建出的 canonical SHA** 查找 failed records；**命中 → rc=2**；<br>② **未命中**但輸入 bytes **≠** canonical diff bytes → ⚠️ **rc=1，在 replay 之前拒絕**。<br>⛔ **⛔ 不採「用重建 bytes 取代輸入」**——⚠️ 那需要另外補完整的替換、provenance 與封存流程，⛔ 不在本計畫範圍 |
| 輸入形式 | ⚠️ **只吃 patch 路徑，⛔ 不吃 SHA**——⛔ 從介面層杜絕 spoof |
| Stage 1 identity 來源 | ⛔ **不得直接讀 `identity/run_identity.json.gz`**——⚠️ 必須先取得下方的**信任錨 snapshot** |
| 每份 record 的 base | 取自該 record 的 `provenance.base_commit`；⚠️ F3 已把它綁到 `patches.composed_sha256`，⛔ 偽造 base 會讓 F8-a 的合成對不上 |
| ⚠️ **結束碼** | **0 ＝ 無命中，放行**；**2 ＝ 命中**（⚠️ 已記錄的壞 patch）；**1 ＝ record 損壞或驗證失敗**（⛔ fail-closed）。⚠️ **2 與 1 都必須讓 runner 在 replay 之前停下**——⛔ 分開兩個碼只是為了讓人分辨「已知壞 patch」與「證據本身壞掉」 |
| ⛔ **runner 的義務** | ⚠️ 步驟 ⑦ 的 runner **必須呼叫這個入口**，⛔ **不得自行另寫一套 lookup**（⚠️ 兩套實作＝兩套語意，這正是本計畫一直在避免的） |

###### 七之二、⚠️ **共用的信任錨 snapshot**（v5 新增）

⛔ **preflight 時還沒有 Stage 2 manifest**——⚠️ 正式 finalize 的第 1～9 道是拿
manifest 裡的 `stage1_evidence` 宣告值去比對，**preflight 沒有那份宣告值可比**。
⛔ **⛔ 不能讓 preflight 與 finalizer 各自推導一份信任錨**（⚠️ 兩套推導＝兩套語意）。

✅ **裁決：共用 helper `load_stage1_anchor()` 與同一份 snapshot schema。**
⚠️ **preflight／finalizer／recovery 是不同程序，⛔ 不可能共用同一個記憶體物件**——
**每次呼叫都重新載入並完整驗證**，⛔ 不是傳遞一份既有 snapshot。

⛔ **⛔ 不能只有一支 helper**：第 1～5、7 道是**單檔**驗證，而**第 6／8／9 道是跨檔關係**
——⚠️ Stage 1 自己也是把跨檔那幾道放在 `verify_evidence_graph()`（`python/backtest/modular/sr_scoring/replay_bundle/evidence.py:184` 的 `verify_evidence_graph()`），
⛔ **`validate_evidence_manifest()` 只驗 manifest 自己的 schema**。所以拆成兩支：

```text id="i074_stage2_anchor_snapshot_001"
load_stage1_anchor() ：           ← 單次讀取 ＋ 單檔驗證（第 1～5、7 道）
  ① 從**寫死常數路徑**載入 Stage 1 evidence_manifest.json，重算 SHA          （道 1）
  ② validate_evidence_manifest()                                          （道 2）
  ③ members key 集合恰好三個；讀 after／cohort／identity，重算各自兩個 SHA
     並與 manifest entry 比對                                              （道 3、4、5）
  ④ 各自跑完整 validator：validate_after_artifact ＋ validate_replay_errors
     ＋ validate_diagnostics(side="after") ＋ validate_cohort_manifest
     ＋ validate_run_identity ＋ 各自的 validate_provenance                 （道 7）
  ⑤ 回傳 immutable snapshot：
     {manifest_sha256, ⚠️ manifest_payload,   ← 等式鏈要比的是**它的**欄位，⛔ 不是 identity 的
      members{…}, identity,
      bundle_id, expected_image_id,           ← ⚠️ 明訂**取自已驗證的 manifest_payload**
      ⚠️ after_base_commit,          ← checker 的 base 推導要用（取自已驗證的 after provenance）
      ⚠️ after_payload / cohort_payload / identity_payload}

validate_stage1_anchor_graph(snapshot, *, stage2_identity,
                             expected_bundle_id, expected_image_id) ：   ← 跨檔（第 6、8、9、10 道）
  ⑥ snapshot.bundle_id／expected_image_id == Stage 2 的                    （道 6）
  ⑦ cohort.after_artifact_sha256 == snapshot.members[after].artifact_sha256（道 8）
  ⑧ cohort keys == candidate_keys(after_payload.rows)                      （道 9）
  ⑨ stage2_identity == snapshot.identity（逐位元）                          （道 10）
  ⑩ ⚠️ **bundle／image 的兩條等式鏈**（道 6-a、6-b）——⛔ 單檔 validator 只驗型別與
     role，⛔ 擋不住「各檔都合法、bundle／image 卻各自不同」：
       manifest.bundle_id == identity.bundle_id == after.bundle_id
                          == cohort.bundle_id  == Stage 2 bundle_id
       manifest.expected_image_id == identity.expected_image_id
                                  == after.provenance.image_digest
                                  == cohort.provenance.image_digest
                                  == Stage 2 expected_image_id
```

⚠️ **三個呼叫端（preflight／finalizer／recovery）⛔ 都必須呼叫兩支**——
⚠️ **第 8、9 道⛔ 不得只留到 finalizer**：⛔ 那樣 cohort 壞掉要跑完數小時 replay 才會被發現。

⚠️ **hash、validator 與回傳值⛔ 必須全部來自同一次檔案讀取**——⛔ 不得為了省記憶體
之後再讀一次（⚠️ 兩次讀取之間檔案可能被換掉，那是 TOCTOU）。

| 使用者 | 用途 |
|---|---|
| **preflight** | ⚠️ **兩支都呼叫**（第 1～10 道）、**提供 `after_base_commit` 給 checker**、執行 failed-record lookup |
| **finalizer** | ⚠️ **兩支都重新呼叫**，用該次 snapshot 產生 Stage 2 manifest 的 `stage1_evidence` 宣告值 |
| **recovery** | ⚠️ **重跑兩支**，再與 manifest 宣告值比對 |

⚠️ 於是「第 1～9 道」在三處是**同一段程式碼**，
⛔ 差別只在 finalize／recovery 多一步「snapshot 的值 ＝ manifest 宣告值」。

###### 七之三、⛔ **兩份 patch 必須先凍結**（v7 新增）

⛔ **checker 過了⛔ 不代表 replay 套的是同一份**：checker 收路徑、重建 SHA、放行之後，
runner 若**再從原路徑讀一次**，中間被換掉就變成——

```text id="i074_stage2_patch_toctou_001"
checker 驗的是 patch A → 放行
runner 實際套的是 patch B
  → B 即使**已有 failed record** 也繞過重跑守門
  → B 若非 canonical，要跑完**數小時 replay** 才在 finalizer 被擋
```

✅ **裁決：runner 一開始就把兩份 patch 各讀一次，凍結到 runner 私有暫存目錄。**

| 規則 | 內容 |
|---|---|
| 凍結時機 | ⚠️ **runner 啟動時、preflight 之前**完成 |
| `COUNTERFACTUAL_PATCH` | ⛔ **必須是非空路徑**（⚠️ 空的反事實 patch ＝ 根本沒做反事實），讀入私有副本 |
| `TOOLING_PATCH`（有路徑） | 讀入私有副本 |
| ⚠️ `TOOLING_PATCH`（空／未提供） | ⚠️ **直接在私有目錄建立 0-byte 凍結副本**——⛔ 不是「跳過」。⚠️ 後續 SHA、套用與封存**一律用該 0-byte 副本**，於是 `tooling_patch_sha256` 自然是空字串的 SHA（② 實測的 `e3b0c442…b855`） |
| 下游來源 | ⚠️ checker、`T1` 推導、實際套用的 worktree、最終封存 **一律只用凍結副本**——⛔ **任何一處都⛔ 不得再讀原始路徑** |
| 私有目錄 | runner 自己的暫存目錄，⚠️ 用完清除；⛔ 它⛔ 不是證據，⛔ 不進 archive |
| 等價做法 | ⚠️ **或**讓 checker 建出的 worktree／`T1` **直接延續給 replay 使用**——⚠️ 兩者擇一，⛔ 不得都不做 |

⚠️ **preflight 的呼叫順序（⛔ 全部在 replay 之前）**：

| 序 | 動作 |
|---|---|
| 0 | ⚠️ **凍結兩份 patch**（「七之三」）——⛔ 之後一律只用凍結副本 |
| 1 | `load_stage1_anchor()` ＋ `validate_stage1_anchor_graph()`：⚠️ **第 1～10 道全部**（含第 8、9 道） |
| 2 | `--check-failed-record` 掃 `failed/`，逐份驗證 |
| 3 | 以「同 identity ＋ 同**完整** counterfactual SHA」判定命中 → ⚠️ **rc=2 即中止**；⚠️ **rc=1（record 損壞）同樣中止** |
| 4 | 都未命中 → 才進 replay |

⚠️ **lookup 遇到損壞或缺檔的 record**：⛔ **fail-closed**——中止並要求人工處理。
⛔ **不得「忽略後繼續」**：那等於讓一次被記錄過的壞 patch 靠「弄壞自己的 record」重新過關。

⚠️ **record 自身發布失敗怎麼算**（⛔ 不能留成未定義）：

| 情況 | 結束碼 | 重跑資格 |
|---|---|---|
| record rename 之前失敗 | **1** | ⚠️ **允許以同一 SHA 重跑**——⛔ 沒有留下任何紀錄，就不存在「已知壞 patch」；⚠️ 但報告必須標明**事故紀錄遺失** |
| record rename 成功、parent fsync 失敗 | **3** | ⛔ **視同已記錄**（lookup 讀得到）→ 同 SHA ⛔ 不得重跑 |
| record 完整發布 | **1**（⚠️ 執行仍是失敗） | ⛔ 同 SHA ⛔ 不得重跑；⚠️ 改 patch 後才可 |

⚠️ **rc=3 必須有出口**（⛔ v1 漏了，會留下永久未確認狀態）：
`finalize-stage2-evidence.sh --recover-failed-record <record 目錄>`——
**完整重驗 F1～F9（含合成守門），再 `_fsync_tree` ＋ fsync parent**，
⛔ **不重建、⛔ 不重跑 replay**。⚠️ **成功後回 `1`**（⛔ 不是 0——那一次執行本來就是失敗的），
⚠️ 並且**重跑資格仍是「⛔ 同 SHA 不得重跑」**。

⚠️ **計次**：patch 失效的中止⛔ **不計入正式 scan**（承 Stage 2 計畫書「二、①」，
比照「preflight 與指紋檢查失敗⛔ 不計入」），⛔ **但⛔ 不得靜默重跑**——
⚠️ 上面的守門就是「⛔ 不靜默」的實作。

##### 八、受影響檔案

| 檔案 | 改動 |
|---|---|
| **`replay_bundle/stage2_evidence.py`** ⬅️ **新增** | ⚠️ **`load_stage1_anchor()` ＋ `validate_stage1_anchor_graph()`（共用信任錨）**、layout、manifest builder／validator、`verify_stage2_graph()`、`finalize_stage2_evidence()`、`recover_stage2_durability()`、failed-attempt record 的 builder／publisher／lookup |
| `replay_bundle/__init__.py` | 公開 API 匯出 |
| `replay_bundle/evidence.py` | ⛔ **一行都不改** |
| `replay_bundle/publish.py` | ⛔ **不改**——原語共用 |
| **`scripts/finalize-stage2-evidence.sh`** ⬅️ **新增** | ⛔ 不動 `finalize-evidence.sh`（Stage 1 已驗收）。⚠️ **含「四之二」的隔離 worktree 重算**、**`--check-failed-record`**（preflight lookup，契約見「七之一」）與 `--recover-failed-record` |
| `replay_bundle/artifacts.py` | ⚠️ **新增** `validate_comparison_artifact()`／`validate_report()`（⛔ 目前只有 builder）；⛔ **既有 validator 一律不動** |
| **`.gitattributes`** | ⚠️ 改成 `**/*.patch`／`**/*.log`——⛔ v1 的第一層規則對 `evidence/patch/` 與 `failed/*/patch/` **無效**（已實查） |
| `scripts/lib/replay-args.sh` | ⚠️ **⛔ 本步驟不改**——兩份 patch 的**凍結**與 `write-tree` SHA 計算屬 ⑦；③ 只定契約 |
| Python 測試 | 新增 `tests/test_stage2_evidence.py` |
| shell 測試 | `scripts/test-replay-args.sh` 補新腳本的 argv 所有權 |

**資料流**：Stage 2 執行產出 before source artifact ＋ comparison ＋ report（operational）
→ `finalize-stage2-evidence.sh` 收進 staging ＋ 兩份 patch ＋ run identity
→ 全部驗證 ＋ **完整執行 after ／ cohort ／ identity 的十道信任錨** → 一次性 rename → rc=0。

##### 九、風險與回滾

| 風險 | 對策 |
|---|---|
| ⛔ **動到 Stage 1 的 evidence 模組，已封存證據驗不過** | ⚠️ **最高風險**。新模組、`evidence.py` ⛔ 一行不改；⚠️ **Stage 1 既有測試全部續跑且⛔ 不修改斷言** |
| ⚠️ Stage 2 archive ⛔ 不自足（方案 (ii)） | manifest 記相對 repo root 的 path；finalize 與 recovery 都重算 SHA；⚠️ 文件註明兩者**必須一起保存** |
| ⚠️ raw blob 繞過 canonical 驗證 | **型別由 layout 決定**，⛔ 不由 manifest 宣告；raw entry ⛔ 無 `artifact_sha256` 欄位 |
| ⚠️ patch 位元組被正規化 | `.gitattributes` 的 `-text`——⚠️ **nested 規則是本步驟才新增**（⛔ ② 加的第一層規則對 `evidence/patch/` **無效**，已實查）；⚠️ validator 直接比 `stored_sha256` |
| ⚠️ 順序錯亂而 hash 相同 | `ordered_components` 強制；⛔ 不依賴「hash 必然不同」 |
| ⚠️ failed-attempt record 與成功 archive 混淆 | 不同根目錄 ＋ 不同 kind ＋ 命名可辨識；⚠️ recovery ⛔ 不接受 failed root |
| **回滾** | ⚠️ 本輪除了新增模組與腳本，也會改 `artifacts.py`（新增兩個 validator）、`__init__.py`、`.gitattributes` 與 shell 測試——⚠️ **整輪 diff 由單一 `git revert` 回滾**；⛔ 不影響 Stage 1 任何已封存證據 |

##### 十、測試與驗證策略

| # | 案例 | 要驗什麼 |
|---|---|---|
| a | layout 多檔／缺檔 | ⛔ fail-closed |
| b | `.json.gz` 被宣告成 `raw_blob` | ⛔ 中止（型別由 layout 決定） |
| c | patch 的 `stored_sha256` 與 `patches.*` 不符 | ⛔ fail-closed |
| d | `composed_sha256` ≠ before source artifact 的 `provenance.tooling_patch_sha256` | ⛔ **中止**——這道是防偽裝的核心 |
| e | `ordered_components` 順序相反／多一項 | ⛔ 中止 |
| f | tooling patch 為 0 bytes | ✅ 通過，且 SHA ＝ 空字串 SHA、`composed == counterfactual` |
| g | Stage 1 after artifact 被換掉 | ⛔ 中止（重算 SHA 不符） |
| h | Stage 1 after artifact 檔案不存在 | ⛔ 中止，錯誤訊息要指出**兩者必須一起保存** |
| i | staging 寫入／驗證／fsync 失敗 | **rc=1**、⛔ 無正式 archive |
| j | rename 成功、parent fsync 失敗 | **rc=3**、⚠️ 保留 archive |
| k | recovery：檔案集合、metadata、SHA、執行身分四道 | 各一支 ⛔ fail-closed |
| k2 | ⚠️ **metadata 全部重算相符、但存在跨檔矛盾**（例如 comparison 的某列 `after` 與錨定 after 不符） | ⚠️ **必須由十四道 graph 抓到**——⛔ 證明 recovery ⛔ 不是只被 SHA 擋下 |
| l | recovery 成功 | ⚠️ 回 manifest 的 `terminal_outcome`，⛔ 不是寫死的 0 |
| m | failed-attempt record 的產出與可辨識性 | 位置、kind、bounded 診斷、兩份 patch 都在 |
| n | 重跑守門：同 SHA ＋ 同 run identity | ⛔ **在 replay 之前**中止 |
| o | 重跑守門：改過 patch（不同 SHA） | ✅ 放行 |
| p | record 發布的三種結果（rename 前失敗／fsync 失敗／完整） | ⚠️ 逐條對上「七」的重跑資格表 |
| q | ⚠️ **Stage 1 的既有 evidence 測試** | ⛔ **全部續跑、⛔ 不修改斷言**——這是「沒動到 Stage 1」的實證 |

⚠️ **v2 新增（對應新定義的 schema 與信任錨）**：

| # | 案例 | 要驗什麼 |
|---|---|---|
| r | before source 的 `candidate_keys(rows)` **非空** | ⛔ 中止（證據層的重複確認） |
| s | comparison 某列的 `before`／`after` 與來源 row **不符** | ⛔ 中止（逐鍵相等，⛔ 不是「相似」） |
| t | comparison 的 `differences` 被竄改 | ⛔ 中止（**重算**才抓得到） |
| u | comparison 的 keys ≠ Stage 1 D+1 cohort 的 156 | ⛔ 中止（多、少、換序各一） |
| v | report 的截斷順序錯／統計由**截斷後**的 rows 算 | ⛔ 中止 |
| w | Stage 1 信任錨**第 1～9 道全覆蓋**（⚠️ 複合規則拆成子案例）：`manifest_path` ≠ 常數／manifest SHA 不符／manifest 本身不合法／`members` key 集合被增減／**三份**任一的 SHA ≠ Stage 1 entry／重算不符／`bundle_id`／`expected_image_id` 不符／cohort validator 不過／cohort 的 `after_artifact_sha256` 或 keys 不符。⚠️ **第 10 道由 ai 負責** | ⛔ 全部 fail-closed |
| x | `manifest_path` 是絕對路徑／含 `..`／symlink 逃出 repo root | ⛔ 全部拒絕 |
| y | ⚠️ **shell finalizer 的合成重算**：換掉任一份 patch → `composed` 不符 | ⛔ **中止且⛔ 不呼叫 Python finalizer**；⚠️ **成功 archive 與 failed record 兩條路徑各一支** |
| z | `patch/counterfactual.patch` 為 **0 bytes** | ⛔ 中止；⚠️ 而 `tooling.patch` 為 0 bytes ✅ 通過 |
| aa | failed record **損壞／缺檔**時的 lookup | ⛔ **fail-closed**（⛔ 不得忽略後繼續） |
| ab | `--recover-failed-record` 成功 | ⚠️ 回 **1**（⛔ 不是 0），且同 SHA 仍⛔ 不得重跑 |
| ac | 錨定的 after／cohort **只比 SHA、跳過完整 validator** | ⛔ 中止——⚠️ 錨定⛔ 不等於驗過（見「三之一」） |
| ad | cohort 信任錨三道（`validate_cohort_manifest()` 不過／`after_artifact_sha256` ≠ 已錨定 after／keys ≠ `candidate_keys()`） | ⛔ 全部 fail-closed |
| ae | `manifest_path` ≠ 寫死常數 | ⛔ 中止 |
| af | before source 的 keys ≠ 已錨定 after 的全量 keys（多／少／換序／重複） | ⛔ 全部 fail-closed |
| ag | 「五之零」的第 **8～13** 道各一支 | ⛔ 全部 fail-closed；⚠️ **含 `rows_shown = 0` 的空報告必須被擋** |
| ag2 | ⚠️ **第 6 道**：某份 artifact 內部的 `bundle_id` 與 identity／Stage 1 manifest 不一致 | ⛔ 中止 |
| ag3 | ⚠️ **第 7 道**：provenance **位置缺漏**／**role 錯**／`image_digest` ≠ `expected_image_id` | ⛔ 各一支 fail-closed |
| ah | failed record 的不變條件 **F1～F9**（⚠️ 含 **F6-a／b／c** 的欄位集合與型別、**F8-a** 的合成守門）（identity validator／頂層對 embedded／provenance 綁定／目錄名不符／`failure_reason` 非列舉值／計數與 sample 長度／非嚴格 boolean） | ⛔ 全部 fail-closed |
| ah2 | ⚠️ **F2-a**：一份**只有 `created_at` 不同**、其餘完全合法的 record | ⛔ **必須被拒**——⚠️ 這正是查找鍵／目錄鍵矛盾的入口 |
| ah3 | `--check-failed-record` 的兩段（Python 段過但 shell 合成段不過，反之亦然） | ⛔ 任一段失敗即 fail-closed |
| ai | Stage 2 的 `run_identity` 與 Stage 1 archived identity **不完全相同** | ⛔ **在 replay 之前中止**（信任錨第 10 道）——⚠️ **⛔ 不是「lookup 不命中後放行」** |
| aj | ⚠️ **`.gitattributes` 對巢狀路徑生效** | `git check-attr` 斷言 `evidence/patch/*.patch` 與 `failed/*/patch/*.patch` 的 `text` 是 **unset**——⛔ 這一條是 v1 真正失效過的地方 |
| ak | failed root **不存在／為空** | ✅ **rc=0 放行** |
| al | records 都有效、但**完整 SHA 都不同** | ✅ **rc=0 放行** |
| am | **完整 SHA 命中** | ⚠️ **rc=2**，⛔ replay 之前拒絕 |
| an | 任一 record **損壞** | ⚠️ **rc=1**，⛔ fail-closed（⛔ 不得「跳過壞的繼續掃」） |
| ao | ⚠️ **Python 段失敗時** | ⛔ **shell 合成段⛔ 不得執行**（⚠️ 省成本是其次，重點是失敗點要唯一） |
| ap | Python 段過、**F8-a 失敗** | ⚠️ **rc=1**，⛔ 拒絕 |
| aq | **Stage 1 identity 錨定失敗**（manifest SHA／`members` 不符） | ⚠️ **rc=1**，⛔ 拒絕——⛔ 不得退而直接讀 identity 檔案 |
| ar | CLI 介面 | ⚠️ 斷言**只吃路徑、⛔ 不吃 SHA**（⛔ 從介面層杜絕 spoof） |
| as | ⚠️ **原始 bytes 不同、但套用後 `T1` 相同**的 patch | ⚠️ **仍須命中**（rc=2）——⛔ 這正是 v4 的漏判 |
| at | patch **套用失敗**（衝突／格式錯） | ⚠️ **replay 之前 rc=1**，⛔ 不得靜默放行 |
| au | 輸入不是 canonical stored patch（bytes ≠ 重建 diff）且**未命中** | ⚠️ **rc=1，replay 之前拒絕**（⛔ 不是只標示） |
| av | 輸入不是 canonical stored patch **但重建後命中** | ⚠️ **rc=2**——⛔ 命中優先於 canonical 檢查 |
| aw | `load_stage1_anchor()` 的回傳值與 hash **來自兩次不同讀取** | ⛔ **不允許**——⚠️ 斷言單次讀取（TOCTOU） |
| ax | ⚠️ **preflight 通過後把原始 patch 檔換掉** | ⚠️ replay **必須仍使用凍結版本**（或在 replay 前中止）。⛔ **⛔ 不是本輪的完成條件**——⚠️ 凍結實作在 **Stage 2 步驟 ⑦**（本階段⛔ 不改 runner），**ax 是 ⑦ 的驗收** |
| ay | preflight **⛔ 未執行第 8／9 道**（cohort 壞掉） | ⛔ **必須在 replay 之前**就被擋下 |
| az | ⚠️ after／cohort 改成**另一個合法 bundle_id 或合法 image digest**，並**同步重算檔案與 manifest SHA** | ⛔ **仍必須在 preflight 被拒**——⚠️ 這正是「各檔都合法、鏈條對不起來」的案例（道 6-a／6-b） |
| ba | `TOOLING_PATCH` **空／未提供** | ⚠️ 產生 **0-byte 凍結副本**，`tooling_patch_sha256` ＝ 空字串 SHA；⛔ **不得跳過凍結**。⛔ **⛔ 不是本輪的完成條件**——⚠️ 凍結實作在 **Stage 2 步驟 ⑦**，**`ba` 是 ⑦ 的驗收**（同 `ax`） |

⚠️ **另外**：`bounded_diagnostics` 的 N ⛔ 不得是執行期參數，**寫死**並由測試釘住。

##### 十一、完成後的歸檔位置

| 內容 | 歸檔到 |
|---|---|
| Stage 2 evidence 的 layout、manifest schema、兩種 entry 型別 | `sr-zone-scoring.md` |
| 兩份 patch 的三方 SHA 關係與 ordered components | `sr-zone-scoring.md`（⚠️ 與「⛔ 不讓語意修改偽裝成 instrumentation」寫在一起） |
| failed-attempt record 與重跑守門 | `development-workflow.md`（⚠️ 它是**操作程序**，⛔ 不是評分規格） |
| 「Stage 2 archive ⛔ 不自足、必須與 Stage 1 evidence 一起保存」 | `development-workflow.md`（比照「凍結 bundle 不得重產」） |

##### 十二、執行順序

```text id="i074_stage2_evidence_order_001"
① 本計畫書確認                                  ← ⚠️ **現在在這裡**
② 實作 stage2_evidence.py ＋ finalize 腳本
③ 測試矩陣 `a`～`aw` ＋ `ay` ＋ `az`（⚠️ **`ax` 與 `ba` 屬步驟 ⑦ 的驗收**，⛔ 不在本輪）
④ review
⑤ 回到 Stage 2 計畫書的步驟 ④（sizing harness）
```

⛔ **④ 未完成前⛔ 不得進入 Stage 2 計畫書的 ⑦**。

#### Stage 2 的前置盤點（2026-09-18，⚠️ **計畫書的材料，⛔ 不是計畫書本身**）

D 的候選數是 **156（> 0）**，所以⛔ 走不到分支 A，**Stage 2 幾乎確定要跑**。
趁 D+1（當時的步驟 ④）執行時把前置條件盤清楚：

**① 範圍是全量，⛔ 不是只跑 156 列。** `evaluation.py` 的 Stage 2 明寫
`assert_same_keys(keys, after_keys, "② before 全範圍 vs after rows")`，
理由也寫在程式裡：「⛔ 只用 after 的 cohort 過濾會讓 before 多出來的候選被**靜默漏掉**」。
所以成本與 D 同級（實測 **180 分鐘**）。

**② ⚠️ 以 `ecbc141^` 為基準時**：它底下 `replay_bundle/` **一個檔都沒有**
（`git ls-tree` 實查），也沒有 `lifecycle_engine.py`，所以要靠 `TOOLING_PATCH`
把整套 bundle CLI 套進去。
✅ **改用 `e1cbbbd` 基準後這個問題消失**——它就是正式 after 的 base，
bundle CLI 與九欄位都在，**patch 只剩 counterfactual 這一個語意變因**（⛔ 不是「一處程式碼」）。

**③ ✅ 好消息：before 版要帶出來的中間變數**全部**已經存在**`ecbc141^` 的 `decision_engine.py` 第 946-975 行——`price_follow_through`／`momentum_state`／
`rr_qualified`／`clear_zone_breakout` 都是現成的區域變數，而且全在**同一個函式內**
（不像 after 版分散在三個檔案）。⚠️ 所以 before tooling 是「**把既有中間結果帶出來**」，
⛔ **不是重新實作判定**——與 Stage 0 對 after 版做的事完全對稱，風險比預期低。

**④ ⚠️ 但有一個設計層級的問題必須在計畫書裡先解決：candidate 的謂詞在兩版⛔ 不對稱。**

```python id="i074_candidate_asymmetry_001"
# after（decision_engine.py:1140-1142）
"rr_decoupling_candidate": bool(lifecycle_phase == "CONTINUATION" and not rr_qualified)

# before（ecbc141^ decision_engine.py:958-963）——CONTINUATION 的條件**本身就含 rr_qualified**
elif event_signal == "CLOSE_RECLAIM" and (
        price_follow_through == "PRICE_UPSIDE_FOLLOW_THROUGH"
        and momentum_state == "MOMENTUM_CONFIRMED"
        and rr_qualified          # ← 正是本筆要驗的那個條件
        and clear_zone_breakout):
    lifecycle_phase = "CONTINUATION"
```

⚠️ **把 after 的謂詞原樣搬到 before，結果恆為 `False`**（CONTINUATION 蘊含 `rr_qualified`，
所以 `CONTINUATION and not rr_qualified` 永遠不成立）。於是 before 的候選集合必然是空的，
而 after 有 156——⛔ **那會讓「兩邊候選集合必須完全相同」這道檢查必然落空**，
被誤判成分支 C（tooling 不對稱），但實際上是謂詞定義的問題。

⛔ **這一點⛔ 不在本節裁決**，它是 Stage 2 計畫書要處理的第一個設計題。
✅ **已於計畫書 v18 裁決（2026-09-22）：⛔ 兩個方向都不採**——v3 比較模型⛔ **不再要求
兩側 candidate 集合相等**，於是這個設計題本身消失（見計畫書「二、①」）。
⚠️ 下表保留為當時考慮過的方向：

| 方向 | 內容 | 要注意 |
|---|---|---|
| 版本無關的謂詞 | 用「三項價格證據齊備 ＋ `not rr_qualified`」這種**兩版都算得出來**的條件當 candidate | ⚠️ 要確認它與 after 現有的 156 列**完全同集合**，否則等於換了驗證對象 |
| 兩版各自定義 | before 用「若移除 `rr_qualified` 則會變成 CONTINUATION」的反事實條件 | ⚠️ 那是在 before 版裡模擬 after 的行為，⛔ 容易變成「用實作驗實作」 |

⚠️ 無論走哪個方向，**`sr-zone-scoring.md` 的九個診斷欄位 schema 是 before 側也要對齊的契約**
（2026-09-11 已定），⛔ 不是只補 `rr_decoupling_candidate` 一欄。

#### 正式 scan 的計次裁決（2026-09-11 使用者明文確認）

⚠️ **從 Stage 0 計畫書移出**（2026-09-16 收斂計畫書時）：移出當下**尚未執行**，
⛔ 不能跟著計畫書一起刪。⚠️ **Stage 1 已於 2026-09-18 依此政策執行完畢**
（D／D+1 兩趟 ＋ 仲裁 `MATCH`，見上方「Stage 1 正式執行結果」）；
本節保留為**政策本身**，⛔ 不是待辦事項。

| 條款 | 內容 |
|---|---|
| 性質 | D／D+1 兩次跨日執行視為**同一組固定 input／code／predicate 的 deterministic replay** |
| 產物 | **D+1 驗證通過的 after artifact 直接作為 I-074 的正式 Stage 1 artifact** |
| 計次 | 這一組**只算一次**正式 scan |
| ⛔ 硬性限制 | **兩次執行期間⛔ 不得修改任何 predicate、程式碼或其他判定條件**；第二趟**只驗重現性**，⛔ 不得依其結果調整任何東西——那樣才不構成「結果導向重跑」 |
| 失敗時 | 兩趟逐列結果不一致 → **必須立案調查**，⛔ **不得以重新執行覆蓋或取代失敗結果** |
| 第三趟 | ⛔ **不執行**——三趟的 input／code／predicate 完全相同，第三趟⛔ 不產生任何新資訊，卻要多燒約 3.2 小時 |

⚠️ **I-100 關閉條件 2 要求「不同日期載入同一份 bundle 得到相同逐列結果」，那必然要用正式
bundle 跑兩趟 replay——而那兩趟同時就是 I-074 的正式 scan。**
⚠️ **preflight 與指紋檢查失敗⛔ 不計入這一次**：那是輸入還沒就位，不是驗證跑過了。

**成本**：零候選約 7～8 小時（兩趟 after）；有候選時另加一趟 before，約 11 小時。

#### 關閉條件（2026-09-01 改為單一決策樹）

結果只會落在三個分支之一。**分支 A 在 Stage 1 就判得出來**——零候選代表沒有東西可比，
**不必再跑 Stage 2**（那趟 before 全掃約 3.7 小時，省下來是實質的）。B 與 C 才需要 Stage 2：

| # | 結果 | 處置 |
|---|---|---|
| **A** | **精確候選數 ＝ 0** | 記錄實際掃描的標的、日期範圍、載入根數、eligible rows、模型 bundle 與設定，**轉為已知限制**並依下方措辭歸檔。本筆關閉 |
| **B** | **候選數 > 0，且 before/after 如預期翻轉** | 記錄**全候選**的逐列證據與下游影響（含 artifact 的 SHA-256），**驗證完成**。本筆關閉 |
| **C** | **候選數 > 0，但沒有翻轉，或下游欄位不符合下表的逐項預期**。⚠️ **v18 移除「before／after 候選集合不一致」這一種**——v3 比較模型下 before 的候選集合**預期恆為空**，那是 RR 條件存在時的**正確行為**，⛔ 不是 tooling 不對稱；⚠️ before 候選**非空**是 **counterfactual patch 失效**，⛔ **中止而非判 C**（見 Stage 2 計畫書「二、①」） | ⛔ **這是新的實作／驗證矛盾，不是零命中。本筆不得關閉**，另立新 issue 調查（編號依本檔使用說明的下一個可用值，**不要預先佔號**）。⚠️ **判讀一律依 comparison artifact 的 156 列逐列結果**——⛔ 不再有 `candidate_mismatch.json` 這條證據路徑 |

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
* ⚠️ **`market_bias`：⛔ 分兩類，⛔ 不是單一轉換**（2026-09-22 訂正，見下方判讀矩陣）。

⛔ **原本寫的「`market_state`：`BULLISH_RECOVERY` → `BULLISH_CONTINUATION`」已移除**：
①`market_state` ⛔ 不在凍結 row 裡；②**無條件要求 `market_bias` 翻成 `BULLISH_CONTINUATION`
會誤殺 AVOID 候選**。

**唯一的 artifact 判讀矩陣（2026-09-22 實測後定案）**

⚠️ 掃過已封存的 D+1 artifact（13,417 列全掃），156 筆候選在 **after 側**的實際分佈：

⚠️ **分類依據必須是 artifact 實有欄位**（2026-09-22 訂正）：

```text id="i074_avoid_classifier_001"
after.action_state == "AVOID"  → AVOID 類
after.action_state == "HOLD"   → 非 AVOID 類
```

⚠️ **交叉斷言** `position_action_condition.state` 同值（實測 156 筆全部相符）。
⛔ **`market_action` ⛔ 不得當正式 classifier**——⚠️ 它⛔ 不在 replay row 裡
（`evaluation.py:834` 的 `_decision_fields_from_summary()` 沒有匯出），
只能用來解釋內部機制或標註 differential fixture。

| 類別 | 筆數 | after `market_bias` | after `action_state`／`position_action_condition.state` | after `final_entry_state` |
|---|---|---|---|---|
| **非 AVOID** | **140** | `BULLISH_CONTINUATION` | `HOLD` | `BLOCKED` |
| **AVOID** | **16** | `BEARISH_BIAS` | `AVOID` | `BLOCKED` |

**B 分支的逐欄期望（before → after）**：

| 欄位 | 非 AVOID（140） | AVOID（16） |
|---|---|---|
| `lifecycle_phase` | `TESTING`／`CONFIRMED` → `CONTINUATION` | 同左（⚠️ **這一類唯一會變的欄位**） |
| `market_bias` | **`BULLISH_BIAS` → `BULLISH_CONTINUATION`** | ⚠️ **`BEARISH_BIAS` → `BEARISH_BIAS`（不變）**——`_market_bias()` 在 `market_action == "AVOID"` 時**短路**，⛔ 不看 `bias_state` |
| `action_state`／`position_action_condition.state` | `CONDITIONAL_HOLD` → `HOLD`（before ＝ `TESTING`）<br>`HOLD` → `HOLD`（before ＝ `CONFIRMED`） | `AVOID` → `AVOID`（不變） |
| `final_entry_state` | ⚠️ **`BLOCKED` → `BLOCKED`（不變）** | ⚠️ **`BLOCKED` → `BLOCKED`（不變）** |

⛔ **`market_state` 與 semantic `entry_permission_state` ⛔ 不得再列為 B／C 條件**——
⚠️ 它們⛔ 不在 artifact 裡，只能當**說明性的推導含意**。

⛔ **持倉欄位不屬於共同必要條件。** 它依 `market_action` 與 before lifecycle 而定，
**不得用來反過來收窄 candidate**——那會把合法的 lifecycle 翻轉排除掉。

**持倉與進場欄位的逐格預期**（`position_action_condition.state`，before → after）：

| before lifecycle | `market_action != AVOID` | `market_action == AVOID` |
|---|---|---|
| `TESTING` | **`CONDITIONAL_HOLD` → `HOLD`** | **`AVOID` → `AVOID`（不變）** |
| `CONFIRMED` | **`HOLD` → `HOLD`（不變）** | **`AVOID` → `AVOID`（不變）** |

`entry_permission_state` 在**四格全部**都是 `BLOCKED` → `BLOCKED`（不變）。
⚠️ **但這一條在 artifact 上⛔ 不可直接觀測**（2026-09-22 訂正）：`entry_permission_state` 是
semantic pipeline 的區域變數，⛔ 沒有落進凍結 row。⚠️ 它是**由 lifecycle 契約推導的含意**
（candidate 的定義本身就要求 `setup_rr_qualified = false`，而 `elif not rr_qualified:
entry_permission_state = "BLOCKED"` 排在 `CONTINUATION` 規則之前），
⛔ **不是實測結論**。artifact 上可觀測的鄰近欄位是 execution 層的 `final_entry_state`
——⚠️ **那⛔ 不是同一顆**。見 Stage 2 計畫書「二、①之二」。

⚠️ **`market_action == AVOID` 會蓋掉整條 lifecycle 對照**（2026-09-01 review 補上）。
`decision_engine.py:1079-1082` 的 `if market_action == "AVOID"` 是**最外層短路**，
直接令 `action_state = "AVOID"`、`entry_permission_state = "BLOCKED"`，
`elif lifecycle_phase == ...` 那一整串（`:1086-1094`）根本不會被評估。
`market_action` 與 `rr_gate` 一樣算在 lifecycle 之前（`:2674`），
**兩個版本對同一列會得到相同的 `market_action`**，所以 `AVOID → AVOID` 是預期結果。

⚠️ **上表有三格是「不變」，那全都是分支 B 不是分支 C。** 只有 `TESTING` ＋ 非 `AVOID`
那一格會看到持倉建議改變；⚠️ **其餘三格在 artifact 上的可觀察差異**（2026-09-22 訂正）：
非 `AVOID` 是 `lifecycle_phase` ＋ **`market_bias`**；
⚠️ **`AVOID` 兩格則只有 `lifecycle_phase`**（`market_bias` 被短路成 `BEARISH_BIAS`，兩側同值）。
⛔ `market_state` ⛔ 不在 artifact 裡，⛔ 不可用它判讀。
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
與上表自相矛盾；⚠️ **2026-09-22 再依 artifact 實有欄位訂正**）：

| 欄位 | 四格的行為 | 可觀測？ |
|---|---|---|
| `lifecycle_phase` | **四格全部會變** | ✅ artifact 直接欄位 |
| `market_bias` | ⚠️ **非 `AVOID` 兩格會變**（`BULLISH_BIAS` → `BULLISH_CONTINUATION`）；⚠️ **`AVOID` 兩格不變** | ✅ artifact 直接欄位 |
| `final_entry_state` | **四格全部不變**（`BLOCKED` → `BLOCKED`） | ✅ artifact 直接欄位 |
| 持倉建議（`action_state`／`position_action_condition.state`） | **只有 `TESTING` ＋ 非 `AVOID` 那一格會變**（持倉線沒有 RR gate，與 `test_widened_path_previously_testing_now_continuation` 一致） | ✅ artifact 直接欄位 |
| semantic `market_state` | 四格全部會變 | ⛔ **不可觀測**——推導含意 |
| semantic `entry_permission_state` | 四格全部不變 | ⛔ **不可觀測**——推導含意 |

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
  * ⚠️ **before／after 的 candidate 處理**（⛔ **2026-09-22 改寫**，依 Stage 2 計畫書的
    v3 比較模型）：⛔ **原本要求「兩側 candidate 集合等價、before 用展開式」的那一支已作廢**
    ——那是 before ＝ `ecbc141^`（底下**沒有** `lifecycle_engine.py`）時的模型。
    現行 before ＝ **`e1cbbbd` ＋ counterfactual patch**，改驗四件事：
    ① **原始 `e1cbbbd` 直接消費已封存的 `rr_decoupling_candidate`**，⛔ 不重算、⛔ 不重組；
    ② **patched before 的 candidate 全部為 `false`**（RR 已加回，⚠️ 恆空是正確行為）；
    ③ ⛔ **不做兩側 candidate 集合相等斷言**——⚠️ after 的 156 keys 是唯一 cohort；
    ④ **非目標列依原始側的 `rr_decoupling_candidate` 分類**，且兩側輸出完全相同。
  * **Stage 2 的 warm-up 連續性**：對同一個候選列，連續 warm-up 與孤立計算會得到不同的
    `event_state_summary`。
  * `lifecycle_engine.py` 既有的優先序與 RR 獨立性測試必須全數續存且不修改斷言。
* **風險**：定向挑樣造成代表性誤讀（以「不推論盛行率」的措辭處理）；I-100 未落實造成資料
  漂移（⚠️ **工具與凍結 bundle 已完成，但跨日逐列驗收仍是 blocker**——見
  [I-100](#i-100decision-replay-沒有-as-of-上界cohort-隔天就重現不了) 的關閉條件 2 與 3）；為求命中而鬆動 predicate
  ——⚠️ **護欄不是「只加回傳欄位」**（2026-09-11 修正）：正確的表述是
  **`lifecycle_engine.py` 與 `decision_engine.py` 的判定條件與優先序一行都不得改，
  而 `evaluation.py` 只允許 Stage 0／Stage 1 明列的來源切換與驗證守門**。
  ⚠️ 兩份計畫書已於 2026-09-16 收斂；**實際改了哪些檔案**見本筆的
  「Stage 0 實作結果」與「Stage 1 實作結果」兩張表（⛔ 本段不另列一套）。
* **歸檔**：驗收結論與仍需保留的限制寫進 [`sr-zone-scoring.md`](./sr-zone-scoring.md)，
  本筆經 review 確認後再移除。

---

### I-100：decision replay 沒有 as-of 上界，cohort 隔天就重現不了

| 欄位 | 內容 |
|---|---|
| 狀態 | **已完成／待 review 後移除**（⚠️ 三個關閉條件與文件要求全部成立，見本格末）（2026-09-16 更新——⚠️ 舊敘述「待正式 bundle」已不成立：`b1_20260901_1d_74350966_5d7ecb10` 於 2026-09-11 選定並進版控，見下方十八／十九）（2026-09-10 依 v23 計畫書完成程式與自動化測試，見下方「實作結果」；2026-09-01 由已知限制升級——[I-074](#i-074lifecycle-engine-的-rr-解耦decision-replay-已跑但一次都沒觸發到) 已把本筆列為硬性前置，見下方「必須做到的範圍」。**已造成一次實際後果**，見下）。✅ **三個關閉條件已全部成立**（條件 1：2026-09-11；條件 2 前半：2026-09-11、後半：**2026-09-18** 的 I-074 跨日 `MATCH`；條件 3：**2026-09-21** 正式實測，見下方十九之後）。✅ **文件要求也已滿足**（`development-workflow.md`「驗收報告必須附什麼」三項齊全）。**長期知識已於 2026-09-21 歸檔**到該文件的「凍結 bundle 一旦選定就不得重產」與「loader 的失敗行為」兩節。⚠️ **本筆狀態＝已完成／待 review 後移除**。⛔ **移除前必須先改掉指向本筆的引用**——⚠️ **不只文件，`python/backtest/modular/sr_scoring/replay_bundle/` 底下有 7 處**（`canonical.py`、`bundle.py`、`publish.py` 等的模組註解），要改指向 `development-workflow.md` 的永久章節 |
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
| **2** | Stage 1 的參數 ＋ `--after-artifact <f> --cohort-manifest <f>` | ❌ | ✅ 全候選 | **兩種終止狀態之一**：集合一致 → `comparison_artifact.json`（⛔ 不截斷）＋ `report.json`；⚠️ 集合不一致 → `candidate_mismatch.json` ＋ 專屬結束碼（見 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「Stage 2 的第五道集合檢查」） |

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
未知 `schema_version`（**受管制檔案：`manifest.json`／`trading_calendar.json`／`cohort_manifest.json`／`after_artifact.json`／`comparison_artifact.json`／**`candidate_mismatch.json`**（2026-09-11 由 I-074 Stage 0 定義 schema 與 validator 後納入；⚠️ 計畫書已收斂，schema 見 [`sr-zone-scoring.md`](./sr-zone-scoring.md)）**——⛔ 列出檔名而不是寫數量，避免再漂移）／**strict 的六個 fail-open 分支**（四-B 表）／**四道集合檢查任一不成立或鍵不唯一**／
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
| **跨日驗收** | ⛔ **D 日載入已凍結的 bundle 執行**，**D+1 日載入同一份再跑**，輸入指紋與逐列結果相同。⚠️ **⛔ 不是「D 日產 bundle」**（2026-09-16 更正）——bundle 已凍結且⛔ 不得重產，重產出來的是**另一份**輸入（見十九的實證） |

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

⚠️ **當時（2026-09-10）尚未滿足的兩項，現況如下**（2026-09-16 更新）：

1. ~~正式 bundle 尚未產出~~ → ✅ **已完成**：`b1_20260901_1d_74350966_5d7ecb10`
   於 2026-09-11 選定並**已進版控**（8 個檔案 tracked），見十八／十九；
2. **跨日驗收仍未執行**——⛔ 必須真的 D 日跑、D+1 日載入同一份再跑，同日重跑不能替代。
   ⚠️ **⛔ 不是「D 日產 bundle」**：bundle 已凍結，⛔ 不得重產（重產的是另一份輸入）。

⚠️ ~~Stage 1 目前跑不動~~ → ✅ **已可執行**（2026-09-16）：`rr_decoupling_candidate`
由 [I-074](#i-074lifecycle-engine-的-rr-解耦decision-replay-已跑但一次都沒觸發到)
Stage 0 補上並已 review 通過，Stage 1 的 preflight／crossday／probe／evidence 程式與測試
也已完成。**剩下的只有正式的 D／D+1 兩趟執行。**

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
| 3（中） | **結構性離線只驗 argv，沒有真的跑完 Stage 1／2**：證明不了 CLI 在 worktree 裡啟動得起來、bundle 與 artifact 在容器內載入得了、無網路無 DB 下跑得完 | 新增 **`scripts/smoke-replay-offline.sh`**（小型 fixture、秒級）：產一份合法 bundle → 把「目前工作樹 ＋ smoke 專用的 `rr_decoupling_candidate`」做成 tooling patch → 用官方腳本真的跑完 Stage 1／2 → 抽驗 artifact 自洽。⚠️ 預設不跑（要 docker build ＋ 兩次容器啟動 ＋ 兩個 worktree），`REPLAY_SMOKE=1` 才跑。⚠️ **「注入假 candidate」是當時的權宜做法，因為產品端還沒有那個欄位；I-074 Stage 0 完成後它會被移除**（連同「非空 cohort」斷言），理由見 I-074 Stage 0（計畫書已收斂）——⛔ 本列是歷史紀錄，不是現況操作說明 |
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
當時**原訂**以 2026-09-11 為 D 日、2026-09-12 為 D+1。
⚠️ **⛔ 那個窗口已過且未執行**（2026-09-16 更正）：實際的 D／D+1 依 I-074 的正式流程
另行安排，見 [`development-workflow.md`](./development-workflow.md)
「I-074 Stage 1 的正式執行程序」與本檔 I-074 的「正式 scan 的計次裁決」。
⚠️ 這⛔ **不影響 bundle 的有效性**——它已凍結並進版控，D／D+1 都是**載入**它，⛔ 不重產。

**✅ 條件 2 的後半已於 2026-09-18 滿足**（I-074 Stage 1 的 D／D+1 跨日仲裁
`outcome = MATCH`：`d_only`／`d1_only`／`provenance_differences` 全為 0，
候選集合逐 key 相同——見 [I-074](#i-074lifecycle-engine-的-rr-解耦decision-replay-已跑但一次都沒觸發到)
的「Stage 1 正式執行結果」）。

**✅ 條件 3 已於 2026-09-21 正式實測**（`load_bundle()` 對正式 bundle 的副本逐項竄改）：

| 情境 | 結果 |
|---|---|
| 對照組：未竄改 | ✅ 載入成功，`bundle_id` 正確 |
| 竄改 payload 內容（`candles.json.gz` 一個 byte） | **中止**：`BundleError: candles.json.gz 的 SHA-256 不符` |
| 竄改 `manifest.json` 裡記的 hash | **中止**：`BundleError: manifest.json 的 SHA-256 不符` |
| 刪掉一份 payload（`chip.json.gz`） | **中止**：`BundleError: bundle 目錄內容不符：缺 [...]` |
| 目錄 basename 與 `manifest.bundle_id` 不符 | **中止**：`BundleError: 三方相等契約不成立` |

⚠️ **⛔ 都是中止，⛔ 沒有任何一種情況「照樣輸出一份看起來可比較的報告」**——
那正是條件 3 要防的。⚠️ 測 manifest 竄改時**目錄名要保持正確**，
否則抓到的是「三方相等」那道、⛔ 不是 manifest 本身的 hash 檢查。

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

`FetchAndStoreIntradayBatch`（`backend/internal/market/fetcher.go`）的回傳值是
`(stored int, err error)`，**沒有逐檔結果**：

* 逐檔 `BulkInsert` 失敗時只 `log.Warn` 然後 `continue`（`:90-93`），**錯誤不往上傳**。
  失敗確實會**間接**反映成 `stored` 沒有增加——但那只是一個數字，
  **看不出是哪一檔失敗**。
* 而且 `stored` 沒增加有**兩種成因**：寫入失敗，或**回應裡本來就沒有這檔的 K 棒**
  （`:87-89` 直接 `continue`）。兩者被壓進同一個計數，**呼叫端分不開**。

**所以問題不是「完全沒有訊號」，而是「訊號不可用」**：知道少了幾檔，
但不知道是哪幾檔、也不知道該不該告警。

而呼叫端 `runIntradayBatch`（`internal/scheduler/scheduler.go:627-634`）**只有在整批呼叫回 error 時**才
`failed += len(batch)`。所以「批次成功、但其中幾檔沒寫進去」這個形狀，
**`job_runs` 完全看不到**。

⚠️ **live 走的就是這條路**：`YAHOO_ENABLED=true`、`FINMIND_INTRADAY_ENABLED=false`，
`runIntradayJob` 在 `HasIntradaySource()` 為真時轉給 `runIntradayBatch`（`internal/scheduler/scheduler.go:558-561`）。
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
| `evaluation.py` 的 `_atr_pct()` | **最後 14 根 true range 的算術平均** / 最後一根 close | evaluation 報表、**`selection_report.py:119`（即凍結門檻的來源）** |
| `scoring.py:216` → `backtest/indicators.py` 的 `calc_atr()` | **Wilder smoothing**：seed = `mean(tr[1:15])`，再一路平滑到第 60 根 | `_adaptive_zone_builder_profile`（runtime 自適應 builder） |

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
的「已裁決：canonical formula ＝ Wilder ATR(14)」，本筆不再複述**（2026-09-10 review 訂正——
這裡原本維護第二份定義，其中選項 ③ 寫成「明確寫出哪一個在分桶、哪一個在門檻」，
⛔ 會被讀成分桶與門檻可採不同公式；實際規則是**先選 bucket authority、門檻必須與它同源、
另一個指標僅供獨立觀察**。兩份定義各自漂移正是這次要消滅的問題）。

**錯誤 B：宣稱「pipeline 迴歸會動 `pipeline_version` 等不變量」。** 不成立——
`pipeline_version` 是 `evaluation.py:53` 的**人工常數** `DEFAULT_PIPELINE_VERSION`。
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
`pipeline_version` 是 `evaluation.py:53` 的人工常數，改壞公式而忘記升版時它不會動。

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
`volatility_bucket_from_profile()`（`zone_builder.py`）**不收門檻參數**，
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
（`evaluation.py:3403` → `_load_csv_sources` → `load_ohlcv_csv`），CSV 可以載入 `inf`。

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
`evaluation.py:53` 的人工常數，改壞公式而忘記升版時它不會動。真正的迴歸偵測在 A 層。

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
| 狀態 | **公式已裁決／遷移待執行**（2026-09-17 使用者確認：**canonical formula ＝ Wilder ATR(14)**，見下方「已裁決」）。⚠️ 剩下的是**遷移**——完整合格母體量測、重算 P33/P67、升 `universe_version`、更新 `bucket_edge_*`／`bucket_hint`，⛔ 仍被 [I-075](#i-075重跑選池會因池外資料變舊而靜默通過) 擋住（2026-09-10 量到的母體塌縮成 125 檔，見下方「步驟 1 量測」） |
| 嚴重度 | 中（**目前不發作**——`SR_SCORING_ADAPTIVE_ZONE_BUILDERS_ENABLED` 在 live 為 `False`。確定的是**公式與門檻不同源**；打開後**可能造成系統性分桶偏差，但方向與幅度待合格母體量測**——⛔ 2026-09-10 前寫的「一旦打開就會系統性偏向高波動」是未經證實的斷言，見下方「步驟 1 量測」） |
| 分類 | Python / 指標定義 / 已知限制 |
| 建立日期 | 2026-09-03 |
| 來源 | I-106 的 review——原以為只是「文件寫 60、實作是 14」，查證後發現是兩個不同演算法 |

#### 現象

| 位置 | 演算法 | 誰在用 |
|---|---|---|
| `evaluation.py` 的 `_atr_pct()` | **最後 14 根 true range 的算術平均** / 最後一根 close | evaluation 報表、**`selection_report.py:119`——即凍結門檻的來源** |
| `scoring.py:216` → `backtest/indicators.py` 的 `calc_atr()` | **Wilder smoothing**：seed = `mean(tr[1:15])`，再一路平滑到第 60 根 | `_adaptive_zone_builder_profile`（runtime 自適應 builder） |

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

#### 已裁決：canonical formula ＝ Wilder ATR(14)（2026-09-17 使用者確認）

⚠️ 下面三個選項**保留作為裁決理由的紀錄**，⛔ 不再是開放選項。

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

#### ⚠️ 結構性事實：分桶 basis 是 `max(atr_pct, average_range_pct)`，⛔ 不是 atr 單獨決定

（2026-09-17 讀碼 ＋ live 唯讀量測補上。**這⛔ 不能取代步驟 1**，母體仍不合格；
它回答的是另一個問題——**換公式的影響面有多大**。）

兩側最後都走**同一個** `zone_builder.volatility_bucket_from_profile()`：

```python id="i107_basis_001"
# zone_builder.py:401-410
values = [v for v in (atr_pct, average_range_pct) if v is not None]
basis = max(values)          # ← ⚠️ 取大者
if basis < LOW_VOLATILITY_THRESHOLD:  return "LOW_VOLATILITY"
if basis > HIGH_VOLATILITY_THRESHOLD: return "HIGH_VOLATILITY"
```

| 側 | atr 那一半 | range 那一半 |
|---|---|---|
| selection（凍結門檻來源） | `_atr_pct`＝**TR SMA(14)** | `((high-low)/close).mean()`，近 60 根 |
| runtime（自適應 builder） | `calc_atr`＝**Wilder(14)** | `((high-low)/close).mean()`，近 60 根 |

⚠️ **`average_range_pct` 兩側的算法完全相同**。所以換公式對分桶零影響的條件是：

```text id="i107_invariance_001"
average_range_pct >= max(atr_pct_sma14, atr_pct_wilder14)
```

⛔ **只比「目前的 SMA」不夠**（本節初稿寫成「`average_range_pct` 較大時零影響」，那是錯的）。
反例：`SMA = 3%`、`range = 4%`、`Wilder = 5%` —— selection 側 basis 是 4%，
換成 Wilder 後 basis 變成 **5%**，⚠️ 這一檔仍會受影響，即使它在「SMA 主導」的統計裡
被算成「range 主導」。

##### 敏感度情境（選池 135 檔，2026-09-17）——⛔ **不是實際 Wilder 影響範圍的估計**

| 情境 | basis 由 atr 決定 | 性質 |
|---|---|---|
| TR SMA(14)（現行 selection 側） | **22 / 135** | ⚠️ 這是**目前 raw SMA 的實況**，⛔ 不是「換公式後不受影響的檔數」 |
| 每檔一律 × 1.130（125 檔量測的**中位數**比值） | 47 / 135 | ⚠️ **敏感度情境**——⛔ 不是實際 Wilder，也⛔ 不是上下界 |
| 每檔一律 × 1.562（該量測的**最大**比值） | 111 / 135 | 同上 |

⛔ **這張表不能用來估計實際的 Wilder 影響範圍**：真值要用 `indicators.calc_atr`
逐檔重算（見「遷移步驟」）。⚠️ 「basis 由 atr 決定」也⛔ 不等於換桶——
還要跨過 P33/P67 門檻才會換，這是上方 125 檔實測**只有 2 檔換桶且都往 LOW** 的部分機制。

##### ⚠️ 方法與限制（⛔ 不要把這組數字當成步驟 1 的替代）

* 母體是**選池 135 檔**（資料新鮮），⛔ 不是步驟 1 要的「全市場合格母體」。
* Wilder 那兩欄是**用比值外推**的，⛔ 不是重算——SQL 難以表達 Wilder 的遞迴平滑。
  真值要用 `indicators.calc_atr` 重算。
* ⚠️ **量測走原始價，python 端走還原價**（`db.py` 預設 `adjusted=True`）。
  `range_pct` 是單日內比值，⛔ 不受還原影響；但 TR 的 `|high − prev_close|` 跨日，
  除權息造成的跳空會被當成真實波動——而 60 根切片（2026-06 ～ 09）正好涵蓋台股除權息旺季。
  **2026-09-17 review 以 `adj_factor` 複算了這一項**：adjusted 情境下 SMA 主導**仍是 22/135**，
  逐檔比較為「adjusted 低於 raw **6 檔**、相同 **129 檔**、高於 raw **0 檔**」。
  ⚠️ 所以**在這 135 檔樣本上** raw ATR 沒有低估 adjusted ATR；
  ⛔ 這是樣本觀察，**不是一般性的公式保證**。

##### 這對裁決的意義

⚠️ 上表顯示**在選池樣本上，預期會改變 basis／bucket 的比例可能偏低**。但兩件事⛔ 不可混為一談：

| | 影響面 | 重測成本 |
|---|---|---|
| 這批分析說得上話 | ✅ 可能較低（⚠️ 僅限選池 135 檔樣本、且為敏感度情境） | ❌ ⛔ 說不上話 |
| 為什麼 | 換桶要同時滿足「atr 主導」與「跨越門檻」 | **P33/P67 重測必須用完整合格母體重算所有 basis**，⛔ 不是只重測 atr 主導的那些標的 |

⛔ **「不同源」是整套 provenance contract 的問題，⛔ 不能限縮成「16～35% 的標的」**：
`VOLATILITY_THRESHOLD_PROVENANCE` 記的是拿 **SMA 側**量出來的門檻，而 runtime 用 **Wilder 側**——
契約層面它整份都不同源，與有多少檔實際換桶無關。
**門檻重測、升 `universe_version`、資料遷移的工程步驟一步都不會少。**

#### 裁決（2026-09-17 使用者確認）：**選項 1——canonical formula ＝ Wilder ATR(14)**

理由：

* runtime 的 adaptive profile 已用 Wilder；實際 ATR zone builder 也用 Wilder。
* Go 的 `CalcATR`（`backend/internal/indicator/atr.go`）同樣是 Wilder。
* 選 SMA 會讓「分桶用的 ATR」與「實際 zone width 用的 ATR」**仍是兩種語意**。
* 選項 3 最後還是要選 bucket authority；authority 若選 Wilder，
  本質上就是選項 1 再加一層欄位契約。

⚠️ **但公式決策與遷移執行要拆開**——這批分析足以支持**架構裁決**，
⛔ 不足以取代正式的門檻量測：

| # | 步驟 | 現在能不能做 |
|---|---|---|
| 1 | 決定 canonical formula ＝ Wilder ATR(14) | ✅ **可以**（架構裁決，不依賴母體） |
| 2 | I-075 回補後，以**資料新鮮的動態全市場母體**、用真正的 `calc_atr` **逐檔重算** | ❌ 被 I-075 擋住；⛔ **不可用 1.130 外推** |
| 3 | 用 `max(wilder_atr_pct, average_range_pct)` 重算 P33/P67 | ❌ 等步驟 2 |
| 4 | 升 `universe_version` | ❌ 等步驟 3 |
| 5 | 更新 provenance、`bucket_edge_low/high` 與 `bucket_hint` | ❌ 等步驟 4 |
| 6 | 完成後再跑 T-003 P2 | ❌ 等步驟 5 |

#### 遷移步驟（⚠️ **公式已定，以下是執行順序**）

⚠️ 2026-09-17 起**完整母體量測⛔ 不再是公式選擇的前置條件**——它的角色改成
**量化影響 ＋ 產生新 P33/P67 ＋ 完成遷移**。舊版「先量測才裁決」那套已移除，
⛔ **不得再引用**。

1. ✅ **已完成**：canonical formula ＝ **Wilder ATR(14)**。
2. **完整合格母體量測**——以資料新鮮的動態全市場母體，用**真正的 `indicators.calc_atr`
   逐檔重算**（⛔ **不可用 1.130 之類的比值外推**）。⛔ 仍被 I-075 擋住；
   ⛔ **不要寫成「319 檔」**——319 是 2026-08-17 那次量測的歷史規模，不是驗收筆數
   （見下方「因此原步驟 1 之前多兩步」）。
3. 用 `max(wilder_atr_pct, average_range_pct)` **重算 P33/P67**。
4. 升 `universe_version`。
5. 更新 `VOLATILITY_THRESHOLD_PROVENANCE`、135 列的 `bucket_edge_low/high` 與 `bucket_hint`，
   並與 T-003「bucket 邊界必須凍結」對齊。
6. **完成前 `SR_SCORING_ADAPTIVE_ZONE_BUILDERS_ENABLED` 維持關閉**；完成後再跑 T-003 P2。

#### 步驟 1 量測（2026-09-10 執行）

⚠️ **這次量測沒有完成步驟 1**——步驟 1 要的是「資料新鮮、且套用既定資格規則的全市場
股票母體」（同一套規則在 2026-08-17 得到 **319 檔**），實際只量到 **125 檔**。
兩件事要分開讀：下方「量到什麼」在它自己的母體上可信，
但「能不能拿來**產生新的 P33/P67**」的答案是**不能**。
⚠️ **2026-09-17 起這句的適用範圍縮小了**：canonical formula 已裁決為 Wilder ATR(14)
（⛔ 不再需要靠量測來選公式），本節資料仍**不足以支撐門檻重測與遷移**。

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
**99/125（79.2%）的 basis 就是 `average_range_pct`**。
**這是原本三選一的討論沒有納入的因素**，也是重新評估嚴重度時要帶上的前提。

⚠️ **2026-09-17 訂正**：本段原本接著寫「對這 79% 來說 ATR 用哪個公式對分桶毫無影響」，
⛔ **那句已移除**——「不受公式影響」的正確條件是

```text id="i107_invariance_002"
average_range_pct >= max(atr_pct_sma14, atr_pct_wilder14)
```

而**本節⛔ 沒有載明那 99 檔是在哪一個公式下統計的**。若只比 SMA，
`range` 介於 SMA 與 Wilder 之間的標的會被誤算進「不受影響」那一側
（反例與同類訂正見下方「結構性事實」段）。
⚠️ **上方情境 a／b 的換桶數（2/125、4/125）⛔ 不受這個缺口影響**——
那兩組是用 Wilder **逐檔實際重算**的結果，不是由這個 79% 推導出來的。

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
3. ~~裁決 canonical formula~~ ✅ **已於 2026-09-17 裁決為 Wilder ATR(14)**，
   ⛔ 這一步不再是流程的一部分。
4. **依裁決結果執行——實際走的是下表 ①**（②③ 保留為當時的分支紀錄，⛔ 不再是開放選項）：

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
   但不指定 symbols 的 full-market 路徑母體是 134 檔（`2867` 已下市，⚠️ **那是正確行為**；
   缺的是對帳證據，見
   [I-113](#i-113i-107-量測把-135-筆-active-cohort-與-134-筆-listed-eligible-cohort-混為同一母體且沒有對帳證據)）。
5. 完成後才跑 [`todo.md`](./todo.md) T-003 P2 的 coarse sweep
   （⚠️ 公式已裁決為 **Wilder ATR(14)**，T-003 那側的執行順序已同步收斂成單一路徑；
   舊的三分支只作為歷史裁決紀錄保留在本筆）。

⚠️ **P33/P67 只保證「量測母體」本身近似三等分**——固定的 evaluation pool 是人工決策的子集，
不是量測母體，它的分佈只能**預期較均衡**，**不保證精確三等分**。

⚠️ **替代路線，尚未採用**：若回補成本不可接受，另一個選項是把 canonical 母體的定義
從「全市場流動性合格股票」（2026-08-17 套此規則得到 319 檔）改成「選池」。那會讓步驟 1 立刻可執行（本節就是），
但**等於換掉凍結門檻的母體，屬於 contract 變更**，要另外裁決，不能當成省事的預設。

#### 已知限制（在**遷移**完成前成立）

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
`_volatility_profiles()`（`evaluation.py`）算的是 `(high - low) / close`，
再用 `.replace([np.inf, -np.inf], np.nan).dropna()` 清理——但 `(h - l) / inf` 的結果是
**`0.0`（有限值）**，清不掉。實測：**整段 `close` 都是 `+inf` 時
`atr_pct` 正確回 `None`，`average_range_pct` 卻是 `0.0`，bucket 被判成 `LOW_VOLATILITY`。**
「資料壞掉的標的被歸類成最穩」這個情境**確實存在，但入口是 `average_range_pct`，
不是 `_atr_pct`**。修正範圍要涵蓋兩者。

#### 可達性

* ✅ **CSV 路徑明確可達**：`evaluation.py:3403` 的 `--csv` → `_load_csv_sources`
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

### I-113：I-107 量測把 135 筆 active cohort 與 134 筆 listed-eligible cohort 混為同一母體，且沒有對帳證據

| 欄位 | 內容 |
|---|---|
| 狀態 | **待修復**（2026-09-17 使用者確認裁決：⚠️ **維持 `active`／`is_listed` 可逆契約⛔ 不自動停用**，對帳責任流採 **方案 B——`python/db.py` 新增唯讀 reconciliation 查詢**。⛔ 已查證 `active` 與 `is_listed` 不連動是明文契約、⛔ 不是漏收；待實作的是 cohort 對帳輸出） |
| 嚴重度 | 中（**只影響「把不指定 symbols 的 full-market report 拿來代表或對照 evaluation cohort、卻沒有 reconciliation」的工作流**；⛔ **full-market selection 本身沒有錯**——它建立的就是「仍上市的全市場母體」。缺的是 135 vs 134 的對帳證據，差在哪不會有任何訊息） |
| 分類 | 資料一致性 / 評估標的池 |
| 建立日期 | 2026-09-10 |
| 來源 | I-107 步驟 1 量測時，把 `evaluation_universe` 的 135 筆 `active` cohort 與 full-market report 可辨識的 134 檔當成同一個母體（135 vs 134）|

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
| `selection_report.py:717` → `db.fetch_symbol_universe()`（不帶 `symbols`） | ✅ **會**——`WHERE is_listed = :listed`（`python/db.py:374`，`listed=True`） |
| `evaluation.py` 的 `_load_db_sources()` → `fetch_candles(symbol, …)` | ❌ **不會**——逐檔直接取 K 棒，**完全不檢查 `is_listed`** |

全 repo（排除 tests）只有 `selection_report.py:717` 呼叫 `fetch_symbol_universe`。
所以 `run_evaluation` / `run_builder_sweep` 只要 `--symbols` 裡帶了 `2867`，
**它照樣會被載入並算出 profile**——用的是停在 2026-08-18 的 K 棒
（`_volatility_profiles` 本身沒有 stale 判定，那是 `selection_report` 才有的邏輯）。

**問題出在哪**：`2867` 在**不指定 symbols 的全市場路徑**上從一開始就沒有進入母體——
不是算不出 profile 被排除，是根本沒被載入，因此**不會出現在任何排除原因統計裡**，也沒有 warning。
⚠️ **那條路徑這樣做是對的**（它要的就是「仍上市的全市場母體」）；
⛔ **錯的是把它的結果拿來代表 evaluation cohort 卻不留對帳**——
「池有 135 檔」與該報告實際跑到的 134 檔之間，沒有東西對得起來。

`migrations/postgres/066_evaluation_universe.sql:29` 的註解寫「`false` ＝ 保留紀錄但不再納入每日維護」。
⚠️ **2026-09-17 訂正**：那句話只描述 `active=false` 的語意，⛔ **並沒有**宣稱
`active=true` 就一定會被納入——`active=true` 仍要再經過**本輪的 listing eligibility 過濾**
（見下方「已查證」②）。⛔ 本筆初稿寫的「與那句話的語意對不上」因此不成立。

#### 已查證（2026-09-17，讀碼 ＋ live 唯讀查詢）

原本列為「尚未查證」的兩件事都查清楚了。

**① `is_listed` 是怎麼變成 `false` 的 → TWSE ISIN 名單同步，⛔ 與 T-071 無關。**

寫入點只有一處，`store/stock_symbol_repo.go` 的 `markMissingDelisted()`：

```sql id="i113_mark_delisted_001"
UPDATE stock_symbols SET is_listed = false, updated_at = <seenAt>
WHERE is_listed = true AND last_seen_at < <seenAt>
```

同步時每個出現在官方名單的 symbol 都會把 `last_seen_at` 推到 `seenAt`，
**本次快照缺席**的就被標為下市（用浮水印取代逐一列舉的 `NOT IN`）。
⚠️ 所以它反映的是**官方名單的缺席**，⛔ 不是 T-071 的對帳結果——T-071「不驅動
`is_listed`」的約定沒有被違反。

**⚠️ 而且 `2867` 是真的下市了**，⛔ 不是資料錯誤：

| 欄位 | 值 |
|---|---|
| `last_seen_at` | 2026-08-30（最後一次出現在官方名單） |
| `delisted_date` | **2026-09-01** |
| `delisted_event_id` | 1（有正式的下市事件紀錄） |

**② 是否刻意保留在池內 → ⚠️ 是刻意的，而且是明文的可逆契約。**

⛔ **本節 2026-09-17 初稿寫成「沒有機制維護、所以不是刻意保留」，那是錯的**，
成因是**只查了寫入端（`SetActive()` 沒有自動呼叫者）卻沒查消費端**。既有契約明文寫著：

> **刻意不採「一次性把 `evaluation_universe.active` 設 false」**：那會在主檔誤判
> （例如某天清冊抓取不完整）時**靜默清掉池成員**，而重新入池是人工動作。
> 每輪重新過濾則是可逆的——主檔隔天恢復，抓取就自動恢復。
>
> —— `docs/architecture.md`「日 K 維護」的下市過濾段，
> 同一段註解也寫在 `backend/internal/scheduler/scheduler.go` 的 `dropDelistedSymbols`

| 狀態 | 語意 | 由誰維護 |
|---|---|---|
| `evaluation_universe.active` | **cohort membership**（人工／選池） | 選池流程；`SetActive()` 的唯一入口是 API `PATCH /api/v1/evaluation-universe/:symbol` |
| `stock_symbols.is_listed` | **本輪是否具備上市資格** | `stock_symbol_sync` 每日自 TWSE 清冊同步 |

⚠️ **兩者刻意不連動**：`active` 決定 **cohort membership**，
而**只有已知 `is_listed=false` 才在本輪排除**——⛔ **不是嚴格的 `active AND is_listed`**。
主檔查無或查詢失敗時一律 **fail-open 保留**（`internal/scheduler/scheduler.go:1215` 起的三態判定）：

```text id="i113_eligibility_001"
active AND ( is_listed = true  OR  主檔查無該 symbol  OR  主檔查詢失敗 )
```

⛔ **寫成 `active AND is_listed` 會誤導後續實作者把 unknown 也靜默排掉**，
而那正是契約裡「多抓一點可接受、靜默少抓不可接受」要防的事。
可逆性由 `TestEvaluationUniverseSyncResumesAfterRelisting`（`internal/scheduler/scheduler_test.go:1980`）釘住。
所以「`SetActive()` 沒有自動呼叫入口」**只能證明兩份狀態刻意不連動**，
⛔ 證明不了系統漏做退池。

#### 真正的缺口：⛔ 不是「沒有回收路徑」，是**對帳不可見**

⛔ 初稿寫的「池成員下市／停牌後沒有任何回收路徑」也不準確，兩處要訂正：

* **日 K 維護路徑本來就有每輪過濾**，而且**會計數**——`dropDelistedSymbols` 回傳
  `(保留的標的, delisted 數, 主檔查無數)`，並寫進 log（`internal/scheduler/scheduler.go:1193` 的
  `zap.Int("delisted", …)`）。三態判定（`true` 保留／`false` 過濾／**主檔查無時 fail-open 保留**）
  也都在契約裡。
* ⛔ **「停牌」不等於 `is_listed=false`**，⛔ 不要在這裡混用——`is_listed` 反映的是
  TWSE 清冊的缺席（見上方 ①）。

⚠️ **⛔ 這⛔ 不是 `selection_report.py` 的缺陷**（2026-09-17 再訂正）：
不帶 symbols 呼叫 `fetch_symbol_universe()` 時，它建立的本來就是
**「目前仍上市的全市場母體」**——把 `is_listed=false` 的 `2867` 排除**是正確行為**。

**真正出錯的是量測流程**：I-107 把 `evaluation_universe.active` 的 **135 筆 cohort membership**
直接拿去和 full-market report 裡可辨識的 **134 檔**相比，**卻沒有留下 reconciliation**。
兩者是不同的母體定義，⛔ 不該當成同一個數字。

⛔ **修法⛔ 不是改 `fetch_symbol_universe()` 的全市場母體，也⛔ 不是把 `2867` 納入 P33/P67**——
新增的 reconciliation 是**獨立旁證**，⛔ 不參與 selection、分位數或 bucket 計算。

相關的程式碼事實（**供理解兩個母體為何不同，⛔ 不是要改它**）：

```python id="i113_universe_gap_001"
# python/db.py:361-375
def fetch_symbol_universe(symbols: list[str] | None = None) -> list[dict]:
    if symbols:
        sql = text(base + " WHERE symbol IN :symbols")      # ⚠️ ⛔ 不過濾 is_listed
    else:
        sql = text(base + " WHERE is_listed = :listed")     # ← 靜默排除 2867
```

`selection_report.py:717` 走的正是 `fetch_symbol_universe()`（不帶 symbols）那一條，
所以它的母體是 **listed-eligible 的全市場**，⛔ **它從未宣稱自己在跑 135 檔 evaluation cohort**。
⚠️ **要修的是工作流**：把這份 full-market report 拿來代表或對照 evaluation cohort 時，
⛔ 不得沒有 reconciliation。⛔ 與 `active` 該不該轉 false 無關。

#### 裁決（2026-09-17 使用者確認）

**維持既有可逆契約**，⛔ 不自動把 `active` 轉成 `false`：

| 項目 | 裁決 |
|---|---|
| `2867` | ⛔ **不做單筆資料修改** |
| `universe_version` | ⛔ **不因這件事升版** |
| 重新上市 | 保留 cohort membership，下一輪自動恢復 |

要補的是**可見性**——凡是宣稱在跑「135 檔池」的路徑，都要產出這組對帳：

**固定九欄位**（2026-09-17 使用者裁決）。成功時：

```json id="i113_reconciliation_001"
{
  "cohort_source": "evaluation_universe.active",
  "pool_members_total": 135,
  "eligible_members_total": 134,
  "excluded_unlisted_count": 1,
  "excluded_unlisted_symbols": ["2867"],
  "unknown_master_count": 0,
  "unknown_master_symbols": [],
  "reconciliation_status": "ok",
  "error_category": null
}
```

查詢失敗時**照樣產出 artifact**，⛔ 不是缺席：

```json id="i113_reconciliation_002"
{
  "cohort_source": "evaluation_universe.active",
  "pool_members_total": null,
  "eligible_members_total": null,
  "excluded_unlisted_count": null,
  "excluded_unlisted_symbols": null,
  "unknown_master_count": null,
  "unknown_master_symbols": null,
  "reconciliation_status": "unavailable",
  "error_category": "database_query_failed"
}
```

⚠️ **為什麼是「產出 unavailable」而不是「直接中止」**：artifact 證明**有執行但對帳失敗**，
又⛔ 不會被讀成「沒有落差」。直接中止會把**執行失敗**與**根本沒執行**混成同一種缺席。

⚠️ **`error_category` 是獨立欄位，⛔ 不得塞進 `reconciliation_status` 字串**——
既然要求穩定的錯誤類別，它就該是可枚舉、可比對的欄位。

##### 欄位語意

| 欄位 | 語意 |
|---|---|
| `cohort_source` | cohort authority 是誰（⛔ 不要讓讀的人猜） |
| `eligible_members_total` | **已上市 ＋ 主檔查無後 fail-open 保留**的成員（⛔ 不是只有 `is_listed=true`） |
| `unknown_master_*` | 主檔查無那一態，⛔ **不得併進 `excluded_unlisted`**——兩者處置相反 |
| `reconciliation_status` | `ok`／`unavailable` |
| `error_category` | **封閉枚舉**；`ok` 時為 `null`（合法值見下表） |

##### 不變式（⛔ 兩組都要成立）

**`reconciliation_status == "ok"`**：

* 六個 counts／lists **全部非 null**；`error_category` 為 **null**。
* 每個 list 的**長度等於對應的 count**。
* `pool_members_total == eligible_members_total + excluded_unlisted_count`
* `unknown_master_count` **已包含在** `eligible_members_total` 之內。

**`reconciliation_status == "unavailable"`**：

* 六個 counts／lists **一律 null**——⛔ **不得用 `0` 或 `[]`**。
* `error_category` 必須是**封閉枚舉的合法值**——⚠️ **現階段唯一合法的非 null 值是
  `database_query_failed`**：

  | 值 | 何時用 |
  |---|---|
  | `null` | **只在** `reconciliation_status == "ok"` 時 |
  | `database_query_failed` | cohort 查詢本身失敗（連線、SQL、逾時） |

  ⛔ **要新增類別就要同步更新本表**——⛔ 不得在程式裡臨時發明字串，
  那會讓下游的分類統計靜默失準。
* ⛔ **原始 exception 只進 log／stderr，⛔ 不寫進 artifact**。

##### ⚠️ 產出 artifact ⛔ 不等於執行成功

| 情境 | scheduler（日 K 回補） | 對帳報告 |
|---|---|---|
| 主檔查詢整體失敗 | **fail-open**：維持全量回補（寧可多抓） | **fail-closed**：⛔ 不得假裝全部 eligible |

⚠️ **operational fail-open 與報告 fail-closed 是兩件不同的事，⛔ 不要混寫。**
unavailable 時：

* artifact **可以落地**，供追蹤與監控。
* ⛔ **但這次報告⛔ 不得被視為「cohort reconciliation 通過」。**
* 依賴 cohort 證據的**匯入、門檻重測、升版或正式 P2 必須中止**。
* CLI **寫完 artifact 後回傳非零 exit code**——⚠️ **「有證據產出」⛔ 不等於「執行成功」**。

⛔ **最糟的輸出是六個零值卻標成 `ok`**——那讓「沒有落差」與「沒查到」看起來一模一樣。

⚠️ **關閉條件要移除「自動 `SetActive(false)`」這個選項**——它⛔ 不是普通選項，
而是推翻既有可逆契約的另一次設計變更。

#### 關閉條件

⚠️ **單一路徑，⛔ 不是「任一即可」**（2026-09-17 訂正——舊版把「自動 `SetActive(false)`」
列為第一個合法選項，與上方正文直接矛盾）：

1. **維持 `active` 與 `is_listed` 分離**——⛔ 不修改 `2867`、⛔ 不因本項升 `universe_version`。
2. **凡是宣稱在跑 evaluation cohort 的路徑，都要輸出 membership／eligible／excluded 對帳**——
   ✅ 責任流已裁決為 **方案 B**（`python/db.py` 新增唯讀 reconciliation 查詢），
   **固定九欄位**、兩組不變式與三態處置見上方「裁決」與「對帳之前要先定 cohort 由誰提供」。
3. 行為歸檔到 [`database-schema.md`](./database-schema.md) 的 `evaluation_universe` 章節
   或 [`architecture.md`](./architecture.md)。

⛔ **「未來自動停用下市成員」⛔ 不是本筆的關閉分支**——它推翻既有可逆契約，
要另立一次 contract change。

##### ⚠️ 對帳之前要先定「cohort 由誰提供」——⛔ 目前的輸入根本拿不到

`selection_report.py:717` 呼叫 `fetch_symbol_universe()` **不帶參數**，載入的是
**全市場仍上市標的**；它⛔ **不知道 evaluation cohort 是哪 135 檔**。
所以那九個欄位現在只是**期望輸出**，⛔ 還不是可執行契約。三條可選的資料流：

| 方案 | 內容 | 代價 |
|---|---|---|
| A | **呼叫端傳入** active symbols | selection 這一側不必碰 DB，但每個入口都要記得傳 |
| B | 新增**專門讀 `evaluation_universe` 的唯讀查詢** | 單一真相源；`python/db.py` 多一個函式 |
| C | 擴充現有 `--pin-symbols` 的對帳 | 重用既有機制，⚠️ 但**只在 `--import-payload` 那條路徑上處理**（`selection_report.py:686` 產出 `missing_symbols`、`:773-775` 發 warning，整段包在 `if args.import_payload:` 裡），而且⛔ **分不出「未上市」與「主檔不存在」**——那正是 `dropDelistedSymbols` 三態裡處置相反的兩態 |

**2026-09-17 使用者確認：採 B。** ⛔ 不選 A 是因為每個呼叫端都可能漏傳或傳入過期清單；
⛔ 不選 C 是因為 `--pin-symbols` 是人工輸入，且目前分不出「未上市」與「主檔缺席」。
（A／C 保留為評估紀錄，⛔ 不再是開放選項。）

B 的形狀：

| 項目 | 內容 |
|---|---|
| cohort authority | `evaluation_universe.active` |
| 取值方式 | **單一 SQL**，`LEFT JOIN stock_symbols`——⚠️ 同一個 statement 才拿得到**一致快照** |
| `is_listed = true` | `eligible` |
| `is_listed = false` | `excluded_unlisted` |
| **無對應主檔** | `unknown_master`，⚠️ **仍計入 `eligible`**（fail-open，與 scheduler 同向） |
| 查詢失敗 | **產出 `reconciliation_status="unavailable"` 的 artifact**（counts／lists 一律 `null`、帶 `error_category`），⛔ **不得中止到沒有 artifact**，也⛔ **不得產出看似成功的零值**；CLI 仍回**非零 exit code**，下游一律中止 |
| 輸出位置 | 獨立的 `evaluation_cohort_reconciliation` 區塊 |
| 邊界 | ⛔ **不影響** full-market selection 母體、P33/P67 與 `selected_symbols` |

⚠️ **這是待實作項，⛔ 不是已完成**——本筆要到對帳輸出實際產出後才能關閉。

⚠️ **順帶**：`2867` 也是 I-105（已收斂）的來源標的——當時是它跨月當天在 live 首次
`partial`。兩者是不同的問題，這裡只記關聯，不要當成同一筆。

---

### I-115：Stage 1 強制要求 `--before-ref`，但那一趟根本不用它；六步程序漏寫，正式執行第一步就失敗

| 欄位 | 內容 |
|---|---|
| 狀態 | **已修文件／待 review**（⚠️ 文件已補齊可執行的指令；**腳本的參數契約本身尚未決定要不要改**，見「待決策」） |
| 嚴重度 | 低（不影響結果正確性，但**擋住正式執行**且錯誤訊息指不到原因） |
| 分類 | Python / SR Zone / 腳本契約 · 文件與實作不一致 |
| 建立日期 | 2026-09-17 |
| 來源 | I-074 Stage 1 正式執行步驟 ② 實際踩到 |

#### 事實

`scripts/run-replay-offline.sh` 對 `--bundle`／`--output-dir`／`--before-ref` **三者一律**
強制檢查（`:88`），但 Stage 1 的 worktree 建在 `AFTER_REF`（預設 `HEAD`）——

```bash id="i115_stage_ref_001"
# scripts/run-replay-offline.sh:98-102
if [ "$STAGE" = "1" ]; then
  SOURCE_REF="$AFTER_REF"      # ← Stage 1 用 after，--before-ref ⛔ 不參與執行
else
  SOURCE_REF="$BEFORE_REF"
fi
```

所以 Stage 1 的 `--before-ref` **只進 provenance 的 `argv`**，⛔ 不決定這一趟跑哪份程式碼。
而 `development-workflow.md` 的「I-074 Stage 1 的正式執行程序」六步裡，②（probe）與
③（D）兩條指令**都沒寫這個必填參數**，於是 2026-09-17 正式執行時：

```text id="i115_symptom_001"
=== ① pin ===        → 成功，image ID 已釘死
=== ② capacity probe ===
ERROR: 需要 --before-ref.        ← 照文件抄就跑不起來
```

⚠️ **錯誤訊息⛔ 指不到原因**：它說「需要 `--before-ref`」，但使用者照著的是一份
**宣稱完整**的執行程序，而且這個參數在 Stage 1 還不影響執行——很難想到要去翻腳本原始碼。

#### 已做的處置（2026-09-17）

`development-workflow.md` 的六步程序 ②③ 補上 `--before-ref ecbc141^`，並寫明：

* I-074 的 before 版固定是 **`ecbc141^`**（`ecbc141` ＝ T-044 Lifecycle Engine 抽離，
  `lifecycle_engine.py` 是該 commit 才新增的）。
* ⛔ **不要寫成 `HEAD^` 這種會漂移的表達式**——D 與 D+1 的 `argv` 必須逐 token 相同，
  綁固定 SHA 才拿得到同一個值。

#### 待決策（⛔ 不要順手改掉，會動到 provenance 契約）

要不要讓 Stage 1 的 `--before-ref` 變成**非必填**？兩邊都有道理：

| 主張 | 理由 |
|---|---|
| 維持必填 | Stage 1 產出的 after artifact 本來就**是為了跟某個 before 版比**；把「要比誰」記進 provenance 能讓這份 artifact 自帶比對對象，Stage 2 才有唯一版本入口 |
| 改為選填 | Stage 1 不用它卻擋住執行，且 `argv` 進 provenance 後**會成為跨日比對的一部分**——一個不影響結果的參數卻能讓 D／D+1 判 mismatch |

⚠️ 改之前要先確認：`provenance_differences()` 逐欄比對全部 10 欄，`argv` 只正規化
`--output-dir`。動 `--before-ref` 的必填性**會改變 `argv` 的形狀**，對已產出的 artifact
不相容。I-074 這一輪⛔ 不要動它——先跑完，之後再決定。

---

### I-116：凍結 bundle 的可重現性只靠「image 還在」——`requirements.txt` 是下限釘法，bundle 也沒記訓練時的套件版本

| 欄位 | 內容 |
|---|---|
| 狀態 | 待決策（⚠️ **⛔ 不影響 I-074 本輪**——image 已釘死且 D／D+1 共用，見下方「對 I-074 的影響」） |
| 嚴重度 | 中（不影響當下結果，但**侵蝕 I-100 整套凍結 bundle 的目的**） |
| 分類 | Python / SR Zone / 可重現性 |
| 建立日期 | 2026-09-17 |
| 來源 | I-074 Stage 1 正式執行時，probe 與 D 兩趟的 log 各出現 3 次 `InconsistentVersionWarning` |

#### 事實

```text id="i116_warning_001"
InconsistentVersionWarning: Trying to unpickle estimator LabelEncoder from version
1.9.0 when using version 1.9.1. This might lead to breaking code or invalid results.
```

（`LabelEncoder`／`_SigmoidCalibration`／`CalibratedClassifierCV` 各一次。）

| 事實 | 值 |
|---|---|
| 模型檔 | `model.joblib`，**凍結在 bundle 內**（`b1_20260901_1d_74350966_5d7ecb10/`） |
| 訓練時 sklearn | **1.9.0**（⚠️ **只能從 warning 反推**——bundle 自己沒記） |
| 執行時 sklearn | 1.9.1（image `sha256:d66030dca485…`） |
| `requirements.txt` | `scikit-learn>=1.4.0`、`lightgbm>=4.0.0`、`joblib>=1.3.0`——**全是下限，⛔ 無上界** |
| bundle `manifest.json` | 只有 `schema_version`，**⛔ 不記任何套件版本** |

#### 缺口在哪：偵測 ≠ 可重建

現有防線是**偵測**，而且運作正常：

* provenance 有 `pip_freeze_sha256` 與 `python_version`，跨日比對逐欄比 → 版本一漂就判 `MISMATCH`。
* `pin-replay-image.sh` 對「identity 已存在但 image 已不在本機」**fail-closed**，
  ⛔ 不得重建後換一個 ID。

⚠️ **但那兩道都只是止血**。一旦那個 image 從本機消失，`requirements.txt` 的 `>=` 會讓重建
裝到**當時最新**的版本，`pip_freeze_sha256` 必然不同——於是**這份凍結 bundle 再也產不出
可比的證據**。凍結 bundle 的整個目的是「日後還能重建同一條指令的結果」，而現在它實際上
依賴的是「那個 image 一直沒被清掉」這件事。

⚠️ 另有一層：`model.joblib` 是 pickle。sklearn 自己警告 unpickle 跨版本
「may lead to breaking code or **invalid results**」——目前沒有任何測試在驗
「同一份 model.joblib 在不同 sklearn 版本下給出相同預測」。

#### 對 I-074 的影響：⛔ 無

D／D+1／comparator／finalizer **共用同一個釘死的 image ID**，所以兩趟的 sklearn 版本必然相同，
跨日比對不受影響；warning 在 probe 與 D 都出現，是**既有且一致**的狀況。
⛔ 本輪不要為了這筆去動 image 或 requirements——那會讓已經跑掉的 probe 失去可比性。

#### 待決策

| 選項 | 代價 |
|---|---|
| `requirements.txt` 改精確釘版（`==`） | 要一併處理既有 image 與 live 的版本差；影響範圍超出 SR Zone |
| bundle `manifest.json` 增記訓練時的 `pip_freeze`／sklearn 版本 | 只解決「知道差在哪」，⛔ 不解決「裝得回去」 |
| 為 bundle 保存 image tarball（`docker save`） | 最徹底，但每份 bundle 多數百 MB |
| 維持現狀，明確接受「image 消失即不可重現」 | ⚠️ 那要寫進 `sr-zone-scoring.md` 成為**明示的已知限制**，⛔ 不能默認 |

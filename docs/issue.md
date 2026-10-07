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
- **下一個新編號從 `I-119` 起算。**（**I-118 於 2026-09-23 發出**——I-074 ③b 的 review 修正時量到 replay 腳本會洩漏 git worktree。**I-117 於 2026-09-23 發出**——由 I-074 Stage 2 開工前盤點發現（Stage 1 的 comparator／finalizer／recovery 都會整份載入兩份 after artifact，超過 mem-guard）。⚠️ **`I-114` 是跳號，⛔ 不得再發用**——2026-09-17 發 I-115 時誤把本行索引文字裡的 `I-114` 當成已發出的條目，於是直接跳到 I-115；依「編號只增不重用」，I-114 就此列為**已跳過**。**I-115 / I-116 於 2026-09-17 發出**——I-115 由 I-074 Stage 1 正式執行時踩到（Stage 1 強制要求 `--before-ref` 卻不使用它，六步程序漏寫）；I-116 由同一次執行的 `InconsistentVersionWarning` 查出（凍結 bundle 的可重現性只靠 image 還在）。**I-113 於 2026-09-10 發出**——由 I-107 步驟 1 量測時發現 cohort 母體對不上（`evaluation_universe` 135 筆 `active` vs full-market report 可辨識的 134 檔，⚠️ **兩者是不同的母體定義**且沒有對帳）。**I-104 / I-105 / I-112 於 2026-09-09 收斂**——三筆的現況都歸檔在 `architecture.md`：I-104 的「**外來錯誤必須先分類，不得把原始錯誤寫進使用者可見欄位**」與**合法形式表**在「寫入失敗的一致性契約」；I-105 的「`verification_unavailable` 一定要帶得出成因」在「日 K 缺漏偵測」；I-112 的 `<stage>_failed:N (symbol:reason, …)` 在「逐檔失敗要帶得出哪一檔、哪個階段、什麼類別」。**編號都不回收。** **I-112 於 2026-09-08 發出**——`todo.md` T-070（已收斂）的 live 觀察時發現 `corporate_action_sync` 的逐檔失敗不寫進 `error`。**I-109 / I-110 / I-111 於 2026-09-07 發出、2026-09-08 全部修復並收斂**——三筆都由 `todo.md` T-071 的實作與 review 分出：I-109 是 `parseROCDate` 收下不存在的民國年（現況歸檔在 `architecture.md`「民國日期的解析是嚴格的」）；I-110 是前端 job 清單漂移（歸檔在 `development-workflow.md`「新增排程還要同步前端的 job 清單」，並由 `scripts/check-job-names.sh` 擋住）；I-111 是 SQLite 的 `busy_timeout` 保護不到 deferred transaction 的升級（歸檔在 `database-schema.md` 的 CAS 契約，改用 `BEGIN IMMEDIATE`）。**編號都不回收。** **I-108 於 2026-09-04 發出**——由 I-106 計畫書的非有限值實測分出；**I-106 / I-107 於 2026-09-03 發出**——I-107 由 I-106 的 review 分出（TR SMA(14) 與 Wilder ATR(14) 的公式分歧）；I-106 來自 T-040 regression baseline 實跑——T-040 regression baseline 實跑時發現 `atr_pct` 的窗口與註解不符、且 evaluation 與 runtime 用的是兩個不同的 ATR 演算法；**I-103 / I-104 / I-105 於 2026-09-02 發出**——I-103 由 I-102 計畫書 review 分出（Yahoo 批次路徑給不出逐檔寫入失敗）；I-104 由 I-102 實作 review 分出（其餘排程與 job 紀錄仍直接寫入原始錯誤）；I-105 來自 `2867` 跨月當天 live 首次 `partial`（`verification_unavailable` 的成因被丟棄）；I-101 / I-102 於 2026-09-01 發出——前者來自 live 的 indicator upsert 溢位、**已於同日修復並收斂**（未完成的 live 部署由 `todo.md` T-069 承接，**該筆已於 2026-09-02 部署驗收完成並收斂**），後者由它的 review 分出、**2026-09-02 實作部署完成並收斂**（現況規格歸檔在 `architecture.md`「寫入失敗的一致性契約」與 `api-reference.md` 的兩條端點，未完成的執行期觀察由 `todo.md` T-070 承接）；I-100 於 2026-09-01 發出，由 `todo.md` T-068 同日改列——**T-068 編號不回收**；**I-099 於 2026-08-31 發出後同日作廢**——誤把 `deploy.sh` 的保守預設當成與 live 的衝突，實際上該檔是範本、所有開關一律預設 `false` 是既有慣例；**編號不回收**；I-098 於 2026-08-31 由 I-096 的 review 發現分出；I-081～I-083 於 2026-08-21 發出（**I-081 / I-082 於 2026-08-27 隨 `todo.md` T-055 收斂**），I-084～I-087 於 2026-08-24 發出，I-088～I-092 於 2026-08-25 發出（**I-091 於 2026-08-28 收斂**），I-093 / I-094 於 2026-08-26 發出（I-093 已於同日收斂，**I-094 於 2026-08-28 收斂**），I-095～I-097 於 2026-08-27 發出，其中 **I-097 於同日改列 `todo.md` T-064**——編號**不回收**。）
  **發出新編號時記得把這一行一起往前推**——上一次就是漏了這步，I-089 發出去之後
  這裡還寫著「從 I-089 起算」，差一點又重用一次（I-070 已經發生過）。
  **現存條目**裡最大的是 I-118（⚠️ 本行下方的歷史索引仍看得到更大的編號，那是紀錄不是條目）。I-109～I-111 於 2026-09-08 收斂、I-104／I-105／I-112 於 2026-09-09 收斂，I-113 於 2026-09-10 發出，I-115 / I-116 於 2026-09-17 發出（**`I-114` 跳號、⛔ 不得再發用**），I-117 / I-118 於 2026-09-23 發出（I-074 Stage 2 開工前盤點與 ③b 的 review），**下一個可用的是 I-119**；I-102 已於 2026-09-02 收斂、編號不回收——I-096 / I-098
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
  本檔現有的 I-100 / I-103 / I-106 / I-107 / I-108 / I-113 / I-115～I-118、已收斂的 I-102 / I-104 / I-105 / I-109～I-112、
  已跳號的 I-114，以及下一個可用的 I-119），
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
| 狀態 | **Stage 1 已完成／Stage 2 進行中：步驟 ④（sizing harness）✅ review 通過並 commit，步驟 ⑤（`--formal` 正式量測、裁定 `M_safety`）✅ review 通過並 commit，步驟 ⑥（Stage 2 計畫書 v29）✅ review 通過並 commit，步驟 ⑦ 的總綱 v1 ✅ review 通過並 commit，⑦a 細部計畫 v1 ✅ review 通過（三輪）並 commit，⑦a 實作 ✅ review 通過（兩輪）並 commit，⑦b 細部計畫 v1 ✅ review 通過（三輪）並 commit，⑦b 實作 ✅ review 通過（兩輪）並 commit，⑦c 細部計畫 v1 ✅ review 通過（七輪）並 commit，⑦c 實作 ✅ review 通過（四輪）並 commit，⑦d 細部計畫 v1 ✅ review 通過（六輪）並 commit，⑦d 實作（含第一輪 review 的三項修正）已 commit `f20ce3c`，⑦d 增補計畫 v11（容器記憶體改以 RSS 判定）✅ review 通過（十輪）並 commit `d1267bb`，⑦d 增補實作 ✅ review 通過（三輪）並 commit `576a8f7`，量測的有效性條件與 ⑨-1 fail-fast 計畫 v8 ✅ review 通過（七輪）並 commit `0a92d95`，實作 ✅ review 通過（兩輪）並 commit `c49fde1`，步驟 ⑧ 計畫 v4（第三輪 review 修正後）⚠️ 待確認**（⚠️ **2026-10-06**：之後依 v29 的順序 ⑧ → ⑨ → ⑨-1；`fs_peak` 的雜訊經查是 live 的排程工作，使用者裁決方案 B，見「Stage 2 量測的有效性條件與 ⑨-1 fail-fast 計畫」）（⚠️ **2026-10-02**：⑦d 見「Stage 2 步驟 ⑦d 細部計畫 v1」）（⚠️ **2026-10-01**：⑦c 見「Stage 2 步驟 ⑦c 細部計畫 v1」與「Stage 2 步驟 ⑦c 實作結果」）（⚠️ **2026-09-30**：⑦b 見「Stage 2 步驟 ⑦b 細部計畫 v1」與「Stage 2 步驟 ⑦b 實作結果」；⑦a 見「Stage 2 步驟 ⑦a 細部計畫 v1」與「Stage 2 步驟 ⑦a 實作結果」）（⚠️ **2026-09-29**：⑦ 分四包 ⑦a～⑦d 依序實作，跨包介面與新裁決見「Stage 2 步驟 ⑦ 總綱 v1」；⚠️ 其中發現 ⑩ 的 replay 執行的是 `e1cbbbd` worktree 的程式碼，⑦a 的改動改以 **tooling patch** 送進 replay）（⚠️ **2026-09-23**：開工前盤點發現 **Stage 1 釘住的 image `sha256:d66030dca485…` 已不在本機**，Stage 2 計畫書改採「兩側都在新 image 重跑」，**Stage 2 計畫書 v28 與 ③ evidence contract v12 已於 2026-09-23 確認**；**③b 已實作並 commit**；**③c 環境閘門 2026-09-23 判定 EQUIVALENT**（`envcheck/` 已發布並於 `5bae980` 進版控）；**③d（Stage 2 archive、failed record、check／recover）已於 2026-09-24 實作、✅ review 通過並 commit**（見「③d 實作結果」）；**步驟 ④ sizing harness 已於 2026-09-24 實作、2026-09-29 ✅ review 通過（四輪）並 commit**（見「Stage 2 步驟 ④ 實作結果」；可用性驗證 `P_B` 150.5～150.7 MiB，⛔ 不是 ⑤ 的正式量測）；②已於 2026-09-23 review 通過，見「I-074 Stage 2 計畫書」（⚠️ 現行版）的「二、⑤」）（⚠️ **2026-09-18**：D／D+1／仲裁**全部跑完，`outcome = MATCH`（rc=0）**，證據已封存到 `python/baselines/i074_stage1/`。**候選數 156 > 0 → ⛔ 排除分支 A**，必須跑 Stage 2 才分得出 B／C。見下方「Stage 1 正式執行結果」）。處置＝**只執行一次有界定向驗證，零命中即收斂成已知限制**，步驟與判準見下方「處置（2026-09-01 定案）」與「關閉條件（2026-09-01 改為單一決策樹）」。**在決策樹的某一個分支被走完之前不得移除本筆** |
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
  | ⛔ 上限 | 最壞 **三趟** replay；⛔ 不得再有第四趟。⚠️ **2026-09-23 改為：跑完最多五趟、物理啟動最多八趟**（Stage 1 的 image 遺失；✅ 使用者於 v27 裁決，見「正式 scan 的計次裁決」的 v26／v27 修訂） |

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
| image ID（六個角色共用） | `sha256:d66030dca485…` | 已釘死；identity 已建立，後續 pin 走 no-op、⛔ 不 build。⚠️ **2026-09-23 查證：該 image 已不在本機**（`docker image inspect` 回 `No such image`），Stage 2 的處置見「二、⑤」 |
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
| top-level `position_action` | 需補 | 另一條推導（`_decision_action()`），**⛔ 不是 B 的翻轉對象**，見下方關閉條件的警語。⚠️ **2026-09-23（Stage 2 計畫書 v28）訂正**：原本寫的「只記錄不判定」撤回——它必須兩側相同，**有差異就是分支 C** |

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
* ⚠️ **2026-09-23 立案為 [I-117](#i-117stage-1-的-comparatorfinalizerrecovery-整份載入兩份-after-artifact超過-mem-guard)**（Stage 1 的 finalizer／recovery 也有同樣的整份載入）。

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

#### Stage 2 步驟 ⑦ 總綱 v1（2026-09-29，✅ **已確認**（2026-09-30，review 通過並 commit））

⚠️ v29 決策表第 7 列把「檔名、label 的注入者、argv 與測試落點」移到 ⑦ 的計畫書。⑦ 的規模與 ③ 相當，所以先寫這份**總綱**：
只定**拆包、順序、跨包介面、新裁決與測試落點**；各包的程式設計寫在該包的**細部計畫**，確認後才實作。
⚠️ 以 v29 的不變條件、驗收條件與已裁定的機制為驗收標準；本總綱對 v29／③ 的修改一律列在「七」，並在原處加註「⚠️ ⑦ 總綱 v1」（確認前寫「待確認」；✅ 2026-09-30 確認後一律改成「✅ 2026-09-30 確認」）。

**使用者裁決（2026-09-29，⚠️ 在寫進總綱之前做成）**：

| # | 決策 | 結果 |
|---|---|---|
| 1 | 拆包 | ✅ **四包依序**：⑦a replay 側 → ⑦b supervisor＋orchestrator＋freeze record → ⑦c `--promote`＋B／C 判讀器 → ⑦d memory harness；每包「細部計畫 → 實作 → review → commit」 |
| 2 | 計畫書 | ✅ **總綱 ＋ 每包細部計畫**（⛔ 不寫一份涵蓋全部細節的大計畫書） |
| 3 | ⑦a 的程式碼怎麼送進 ⑩ 的 replay（見下方「一之一」） | ✅ **tooling patch**：⑦a 照常在 HEAD 開發與測試，再機械產生「`e1cbbbd` → HEAD、只含 `evaluation.py` ＋ `replay_bundle/`」的 tooling patch；⑩ 固定先套 counterfactual、再套 tooling（⛔ 不採「replay 改掛 HEAD 的工具層」——那要改 provenance 語意與 ③ 的 verifier） |
| 4 | tooling 的缺陷造成**假的 rc=6** 怎麼辦（failed record 的 lookup 鍵只有 identity ＋ counterfactual SHA） | ✅ **鍵⛔ 不加入 tooling SHA、風險前移**（⚠️ 第一輪 review 之後，鍵本身改成第 9 列的語意 SHA）：⑦a 以**真實的 evaluation 路徑**測 aa／ab／ac／ac2，加一支「餵進 `CounterfactualEffectCheck` 的一定是 before rows」；smoke bundle 完整走一次反事實路徑；⑨ 加 **tooling 非語意 guard**。⚠️ 真的誤判時，唯一出路是改 counterfactual 的**產品檔**（第 9 列的語意鍵才會變），連帶重做 ⑨、⑨-1、⑨-2（⛔ 不採「鍵加入 tooling SHA」——任何 tooling 改動都能解鎖同一份壞 patch；⛔ 不另加限縮範圍的真資料彩排）。⚠️ **第一輪 review 指出**：同一個理由也適用於完整 SHA 鍵本身——counterfactual 含兩個測試檔，只改測試就會換鍵，見第 9 列 |
| 5 | canonical diff 的 index 行（見下方「二」） | ✅ **改用 `--full-index`**（⛔ 不採「釘死 `core.abbrev=7`」） |
| 6 | ③ 留給 ⑦ 的測試（ax、ba，以及 n／ai／ay 的「在 replay 之前」那一層） | ✅ **歸 ⑦b**——它們測的是 orchestrator 的 preflight（原本的拆包選項文字把它們寫在 ⑦d） |
| 7 | ⑧ 的定義 | ✅ **改成矩陣完整性稽核 ＋ 全量執行**：⑦ 各包各自附上它負責的測試，⑧ 逐 id 對照、補齊缺漏後全量執行一次 |
| 8 | ba | ✅ **拆兩層**：runner 層（⑦a）照舊測「空 tooling → 0-byte 凍結副本」；orchestrator 層（⑦b）改成「tooling 必須非空、且 ＝ 複本內封存的那一份」 |
| 9 | failed record 的查找鍵（第一輪 review 之後提出） | ✅ **改用語意 SHA ＋ 照實寫殘餘**：鍵改成只涵蓋 counterfactual 的**產品檔白名單**（`decision_engine.py`、`lifecycle_engine.py`）的 canonical diff SHA（⚠️ 第二輪 review 訂正：第一輪誤寫成「排除 `tests/`」，偏離了本列核可的內容）；archive 與 failed record 仍保存完整 patch 與完整 SHA；殘餘限制照實寫明（見「二」）（⛔ 不採「維持完整 SHA ＋ 人工門檻」） |

**⑦ 總綱 v1 第一輪 review 的修正（2026-09-29）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | ⛔ **只加 `--full-index`，diff 仍不 canonical**——git config 會改變 bytes，而清掉 `GIT_*` 擋不住 system／global／local config | ✅ 先在本機重現（git 2.30.2）：只加 `--full-index` 時，`diff.noprefix`、`diff.context`、`diff.renames`、`diff.orderFile` 都改變 SHA。改成「二」的完整定義：**明確釘死所有影響 bytes 的參數**，並**在隔離的暫存 bare repo 計算**；新增「惡意 git config 下 bytes 不變」的測試。釘死參數之後，15 種惡意 config 實測全部得到同一個 SHA |
| 高 | ⛔ **failed-record 鍵可被非語意變更解鎖**——counterfactual 含兩個測試檔，只改測試註解就換鍵；與拒絕 tooling SHA 的理由是同一個漏洞，文字宣稱的保證高於實際 | ✅ 本總綱決策表第 9 列：改用語意 SHA（⚠️ 第二輪訂正為產品檔白名單）；③ 的「七」「七之一」「七之三」、F4、「二、①」的重跑條件同步；殘餘限制照實寫（只改產品檔的註解或空白，鍵仍會變） |
| 中 | ⛔ **產生器讀 HEAD，與「review 後才 commit」互相循環** | ✅ 產生器改吃**明確的 source tree OID**：stage 程式碼 → `git write-tree` → 對該 tree 產生 tooling patch → stage patch → review 整份 staged 結果 → commit → 驗 HEAD 的 tooling 路徑投影與 patch 完全相同（「二」的版控列） |
| 低 | ⛔ 「既有測試的斷言一律不改」範圍太寬 | ✅ 限定為「`e1cbbbd` 既有的產品測試」；釘住舊 patch SHA 或 canonical bytes 的契約測試（`test-replay-args.sh` 把 `--full-index` 視為非 canonical 的那一段）依新定義更新（實查：⛔ 沒有任何程式或測試釘住 `ef7a4cdf…`） |
| ⚠️ 自己發現 | ⛔ **host 的 git 2.30.2 ⛔ 不支援 `GIT_CONFIG_GLOBAL`**（2.32 才有；實測 `~/.gitconfig` 照樣被讀）；⛔ **`info/attributes` 與工作樹的 `.gitattributes` 無法以 `-c` 關掉**（實測 `diff=<driver>` 會改 hunk 標頭、`-diff` 會改成 binary）；⛔ 2.30 也⛔ 沒有 `rev-parse --path-format` | ✅ 改在**暫存 bare repo**（`objects/info/alternates` 指回來源 repo）計算——沒有工作樹、沒有 `info/`、local config 是暫存 repo 自己的；`HOME`／`XDG_CONFIG_HOME` 指向空目錄、`GIT_CONFIG_NOSYSTEM=1`、`GIT_ATTR_NOSYSTEM=1`。實測來源 repo 同時帶 `info/attributes` 與 `.gitattributes` 時，bytes 仍與乾淨環境相同 |

**⑦ 總綱 v1 第二輪 review 的修正（2026-09-30）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | ⛔ **新增的 `counterfactual_semantic_sha256` 沒有同步進 failure record 的封閉 schema**——照文字實作，合法 record 會被當成多欄拒絕 | ✅ ③「七」的 schema 列加入這一欄（hex64，⛔ 不得為空 diff 的 SHA）；builder、validator、summary 同步；F4、lookup、recovery 以**重算值**比對目錄名與宣告值；⑦a 補缺欄、多欄、格式錯誤、宣告 ≠ 重算的測試；「七」的受影響清單明列 schema 列 |
| 高 | ⛔ **語意 SHA 只是「排除 `tests/`」**——只有 exclude pathspec 時 git 先納入其他全部路徑，counterfactual 混入文件、fixture、腳本就會換鍵，仍能解鎖同一份產品變更；而且⛔ **偏離了本總綱決策表第 9 列核可的內容**（使用者核可的選項寫的是「只涵蓋 `decision_engine.py`、`lifecycle_engine.py`」，第一輪實作成排除式） | ✅ 改成**正向白名單**（兩個產品檔的完整路徑）；另加前置不變條件：完整 counterfactual 改動的檔案集合**恰好**是固定的四個檔（實查現行 patch 的 `diff --git` 正好是這四個），多一個、少一個一律拒絕；⑦a 補「多出非測試的非產品檔 → 拒絕」「只改測試 → 鍵不變」的測試 |
| 中 | ⛔ **回滾宣告過度**——「counterfactual 換格式的 commit 可以單獨 revert」不成立：⑦a 同時改共用函式、SHA、schema 與測試，⑦b～⑦d 又依賴新格式 | ✅ 「六」的回滾列改寫：⑦b 開工之前只能整包 revert ⑦a；之後要依 ⑦d → ⑦a 的反向順序 revert，並重做受影響的封存與量測 |
| 低 | ⛔ **交叉引用錯誤**：「五」說測試覆寫鎖目錄「違反決策表第 9 列」，原意是 v29 的第 9 列，但本總綱的第 9 列是語意 SHA | ✅ 改成「v29 決策表第 9 列、『八之一之二』第 9 列與 n7b」；本總綱內其餘指向自己的引用一律寫成「本總綱決策表第 N 列」，⑦ 總綱以外的加註寫成「⑦ 總綱 v1 決策表第 N 列」 |

**⑦ 總綱 v1 第三輪 review 的修正（2026-09-30）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | ⛔ **語意 SHA 缺少 shell → Python 的可信交接**——文件要求 F4、publish、lookup、recovery 都以重算值驗它，但 Python ⛔ 不碰 git，現行交接（`VerifiedComposition`，實查 `stage2_archive.py` 與 `finalize-stage2-evidence.sh` 的 `VERIFY_ARGS`）只有 patch base 與三個完整 SHA；builder 的來源、recovery 在 fsync 之前的綁定、「Python 執行 F4」都沒有定義 | ✅ 「二」新增交接列：共用函式輸出語意 SHA 並先驗四檔集合；新增只能由腳本注入的 `--verified-counterfactual-semantic-sha256`（只有 publish／recover-failed-record 必須帶，其餘模式拒絕）擴充 `VerifiedComposition`；publish 在 rename 之前、recovery 在 fsync 之前比對 record 欄位；check 拆成 Python 層（hex64、目錄名）與 shell 層（重算、四檔集合、命中判定）；F4 分兩層；同步注入參數表、CLI matrix、`patch_claims()`、summary、shell 欄位解析與 argv fixture；⑦a 補 spoof、缺參數、模式不符、欄位 ≠ 交接值、shell 驗完後替換 patch 的測試 |
| 低 | ⛔ 「七之一」比對鍵那一列段尾是 `））。` | ⚠️ 查證：括號**其實是平衡的**（前一個 `）` 關內層的「（見…」、後一個關外層的加註），直接刪一個反而會不平衡；✅ 改成以破折號取代內層括號，段尾只剩一個 `）`，⛔ 不再誤讀 |

**⑦ 總綱 v1 第四輪 review 的修正（2026-09-30）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | ⛔ **驗收矩陣與重跑規則仍以「完整 SHA」決定重跑資格**——n、o、ab、al、am、重跑資格表與 recovery 說明都還寫「同 SHA／改 patch」；照字面，「只改兩個測試檔」（完整 SHA 不同、語意 SHA 相同）會被放行，與第 9 列矛盾 | ✅ 全檔掃過現行規則並統一成**語意 SHA**：③ 的 n、o、ab、al、am 改寫，新增 **o2**（只改測試 → rc=2），am 至少兩支（完整 SHA 相同、只改測試）；③「七之三」的重跑資格表、`--recover-failed-record` 的重跑資格、兩個鍵一致性的說明；v29「六、1」的隔離規則、n12（另補只改測試的一支）、z、「八之一」的真正 repo 列。⚠️ 歷史紀錄（v29 各輪修正表、v2x 缺口表）維持原樣 |

**⑦ 總綱 v1 第五輪 review 的修正（2026-09-30）**：

| # | 問題 | 修正 |
|---|---|---|
| 中 | ⛔ **兩處現行規則仍寫「完整 SHA 不同即可重跑」**：n12 的句尾「不同 SHA 且 freeze record 夠新 → 可以進 replay」；「正式 scan 的計次裁決」的 before 重跑列「以不同的 counterfactual patch SHA 重跑」——照字面只改測試檔就能用掉唯一一次 before 重跑額度。⚠️ 第四輪的全檔重掃漏掉這兩處（搜尋條件沒涵蓋「不同 SHA」「不同的 counterfactual patch SHA」這類寫法） | ✅ n12 句尾改成「不同語意 SHA」；計次政策的 before 重跑列改成「只有白名單產品檔的 diff 改變、語意 SHA 與失敗紀錄不同、其餘 preflight 全部通過，才可進行這一趟；只改測試檔不得取得重跑資格」。以擴大後的條件再掃全檔，其餘命中都是歷史紀錄或前幾輪已加註之處 |

##### 一、目標與⛔ 不做的範圍

| 項目 | 內容 |
|---|---|
| 目標 | v29「八」⑦ 列的全部項目；③b／③d 已完成的部分（發布、三種 recovery、failed-record 的 check、共用串流讀取、信任錨、`CounterfactualEffectCheck`）⛔ 不重做 |
| ⛔ 不做 | ⑧ 的矩陣稽核、⑨ 的 guard 與封存、⑨-1／⑨-2 的正式量測、⑩、⑪ |

###### 一之一、⚠️ 為什麼要 tooling patch（2026-09-29 實查）

⚠️ ⑩ 的 replay 容器掛的是 **`e1cbbbd` worktree 的 `python/`**——`run-replay-offline.sh` 在 Stage 2 以 `BEFORE_REF` 建 worktree
（`SOURCE_REF="$BEFORE_REF"`），再以 `-v "$WORKTREE/python":/app:ro` 掛進容器。⛔ **所以 ⑦a 在 HEAD 對 `evaluation.py` 的改動，
⑩ 根本執行不到**；它需要的 `stream.py`、`stage2_archive.py`、`stage2_evidence.py` 在 `e1cbbbd` 上也不存在。v18～v29 都把這些改動
當成「改 HEAD 就生效」，⛔ 沒有處理這個落差（sizing harness 以 fixture 容器模擬 replay 的寫檔，所以也沒有撞到）。

✅ 實查 `e1cbbbd..HEAD`：`evaluation.py` 與**全部產品程式碼**都沒有差異，差異只在 `replay_bundle/`（③b／③d 加的工具）；所有測試檔
都是**新增**，`e1cbbbd` 的既有測試原封不動存在於 HEAD、並已對 HEAD 的 `replay_bundle/` 跑綠。③ 的 archive、合成守門、
`ordered_components` 與 `patch_claims()` **本來就支援非空的 tooling patch**（`compose_check` 在 tooling 非空時才套；既有 shell 測試已有
非空 tooling 的案例）——⛔ **不需要為了非空 tooling 改 ③ 的程式**，只要把 tooling patch 做出來（⚠️ 但決策 5 的 `--full-index` 另外要改 ③ 的 diff 呼叫，見「二」）。

##### 二、tooling patch 與 canonical diff 的契約（決策 3、5）

| 項目 | 規則 |
|---|---|
| 路徑集合 | `python/backtest/modular/sr_scoring/evaluation.py` 與 `python/backtest/modular/sr_scoring/replay_bundle/`（只含已追蹤檔）；⛔ 不含 tests（HEAD 的新測試依賴 worktree 裡沒有的 `python/scripts/`，放進去也會擴大「非語意」的範圍） |
| 產生器 | `scripts/make-i074-tooling-patch.sh --source-tree <40 碼 tree OID>`（⑦a；⚠️ 第一輪 review：⛔ **不讀 HEAD**，來源一律是明確的 tree）：暫存 worktree 內 `e1cbbbd` ＋ `git apply --index` counterfactual → `write-tree` 得 `T1` → 以**來源 tree** 的 tooling 路徑覆蓋 → `write-tree` 得 `T2` → 輸出 canonical 的 `T1..T2` diff → 以共用合成函式自我驗證（raw bytes 的 SHA ＝ ③「四之一」第 2 條的 `tooling_patch_sha256`） |
| 產生器的四條不變條件 | ① counterfactual 與 tooling 的路徑**交集為空**（counterfactual 只動 `decision_engine.py`、`lifecycle_engine.py` 與兩個測試檔）；② `T2` 在 tooling 路徑上 ＝ 來源 tree；③ `T2` 在其他路徑上 ＝ `T1`；④ **產品碼不變條件**：`python/` 扣掉 `baselines/`、`scripts/`、`sr_scoring/tests/`、`replay_bundle/`、`evaluation.py` 之後，`e1cbbbd` → 來源 tree **沒有差異**——⛔ 否則 ⑩ 會靜默跑到舊版 |
| canonical diff（決策 5；⚠️ 第一輪 review 補完） | **唯一定義**：`git --git-dir=<暫存 bare repo> -c core.quotePath=true -c diff.suppressBlankEmpty=false -c core.attributesFile=/dev/null diff --binary --full-index --no-ext-diff --no-textconv --no-color --src-prefix=a/ --dst-prefix=b/ -U3 --inter-hunk-context=0 --diff-algorithm=myers --no-renames --indent-heuristic -O/dev/null --no-relative <A> <B> [-- <固定 pathspec>]`；`<A>`、`<B>` 必須是 **40 碼 tree OID**（⛔ 不接受 commit、ref 或工作樹）。⛔ 理由：`--binary` 對文字檔的 `index` 行只印縮寫的 blob OID（長度隨物件數自動決定）；⛔ **只加 `--full-index` 也不夠**——實測 `diff.noprefix`、`diff.context`、`diff.renames`、`diff.orderFile`、`core.quotePath`、屬性的 `diff=<driver>`／`-diff` 都會改變 bytes |
| 計算位置與環境（⚠️ 第一輪 review 補） | 在**暫存的 bare repo** 計算：`git init --bare --template=`，`objects/info/alternates` 指向來源 repo 的 common objects 目錄（⛔ 不在複本或真正 repo 內建 alternates）——沒有工作樹與 `info/`，所以 `.gitattributes`、`info/attributes` 都讀不到（⛔ 兩者無法以 `-c` 關掉），local config 也是暫存 repo 自己的。環境：以 `env -i` 起頭，只帶 `PATH`，設 `GIT_CONFIG_NOSYSTEM=1`、`GIT_ATTR_NOSYSTEM=1`，`HOME` 與 `XDG_CONFIG_HOME` 指向空的暫存目錄（⚠️ host 是 git 2.30.2，⛔ 不支援 `GIT_CONFIG_GLOBAL`）。上一列的參數與 `-c` 是第二層防線 |
| ⚠️ 惡意 config 測試（第一輪 review 補） | fixture 同時含 rename、非 ASCII 檔名、空白 context 行、多檔案（順序）、binary 檔、會被 funcname 規則命中的行；在測試能控制的每一層放入惡意設定（來源 repo 的 `.git/config`、測試 `HOME` 的 `.gitconfig`、`info/attributes`、工作樹 `.gitattributes`、`core.attributesFile`、`GIT_CONFIG_PARAMETERS`／`GIT_CONFIG_COUNT`），逐層與全部同時，bytes 都必須 ＝ 乾淨環境的參考值 |
| ⚠️ 殘餘限制：git 版本 | 產生器、runner、finalizer 與晉升的驗證都在同一台 host 的 git（2.30.2）執行；⑨ 封存到 ⑩ 之間若升級 git，diff 輸出可能不同——⚠️ 由 runner 在 docker 之前的「raw bytes ＝ 重算的 canonical」斷言 **fail-closed**，⛔ 不會靜默通過（要重新封存） |
| 唯一的合成函式 | `scripts/lib/replay-args.sh` 抽出一支（含上面的 canonical diff 與語意鍵）；runner、`finalize-stage2-evidence.sh` 的 `compose_check` 與 `--check-failed-record`、產生器**都呼叫它**，⛔ 不各寫一份 |
| ⚠️ failed record 的查找鍵（本總綱決策表第 9 列；⚠️ 第二輪 review 改寫） | **`counterfactual_semantic_sha256`** ＝ canonical diff `<base> <T1> -- python/backtest/modular/sr_scoring/decision_engine.py python/backtest/modular/sr_scoring/lifecycle_engine.py`——**正向白名單**，⛔ 不用「排除 `tests/`」（只有 exclude pathspec 時 git 先納入其他全部路徑，counterfactual 混入文件、fixture、腳本就會換鍵）。⚠️ **前置不變條件**：完整 counterfactual 在 `<base>..<T1>` 改動的檔案集合**恰好**是固定的四個檔——兩個白名單產品檔 ＋ `python/backtest/modular/sr_scoring/tests/test_i074_diagnostics.py`、`python/backtest/modular/sr_scoring/tests/test_lifecycle_engine.py`；多一個、少一個一律**拒絕**（⛔ 不計算鍵、⛔ 不放行）；語意 diff 必須**非空**。四個檔案的集合是常數，要改就改計畫。failed record 的目錄名改成 `<bundle_id>-<counterfactual_semantic_sha256>`；**封閉 schema 加入 `counterfactual_semantic_sha256`（hex64）**，builder、validator 與 summary 同步；F4、lookup 與 recovery 一律以**由封存 patch 重算的值**比對目錄名與宣告值（⛔ 不只信宣告值；重算只能在 shell 做，交給 Python 的方式見下一列）；**完整 patch 與 `patches` 四欄照舊保存與驗證**。⚠️ **殘餘限制（照實）**：lookup 防的是「白名單產品檔的 canonical diff 逐位元相同」的重跑，⛔ **不是語意等價**——只改這兩個產品檔的註解或空白，鍵仍會改變；目前⛔ 沒有任何 failed record，改鍵不影響既有證據 |
| ⚠️ 語意 SHA 的 shell → Python 交接（第三輪 review 補） | ⚠️ 重算要用 git，只能在 shell 做；現行的可信交接（`VerifiedComposition` ＋ `--verified-*`）只有 patch base 與三個完整 SHA。契約：① **共用合成函式**另外輸出語意 SHA，並先驗四檔集合（不符即中止、⛔ 不輸出語意 SHA）；② 新增**只能由腳本注入**的 `--verified-counterfactual-semantic-sha256`（進 `STAGE2_INJECTED_ARGS`，重複或由使用者傳入即拒絕），擴充 `VerifiedComposition`——⚠️ **只有** `--publish-failed-record`、`--recover-failed-record` **必須**帶，`--finalize`、`--recover-durability`、`--check-failed-record` 帶了就拒絕（成功 archive ⛔ 不存語意 SHA）；③ **publish**：shell 從凍結的 counterfactual 重算語意 SHA 並注入 → Python builder 把它寫進 record → `check_verified_composition()` 在 **commit point（rename）之前**比對 record 實際的欄位 ＝ 交接值；④ **recovery**：`i074-stage2-patch-claims.py` 另外取出 record 宣告的語意 SHA → shell 從 record 內的實際 patch 重算並比對宣告值 → 注入 → Python 在 **fsync 之前**比對 record 欄位 ＝ 交接值；⑤ **check**：Python 段只驗 hex64 與「目錄名 ＝ 宣告值」，summary 另輸出宣告的語意 SHA；shell 段從每份 record 的實際 patch 重算語意 SHA、驗四檔集合、比對宣告值，本次輸入也由 shell 重算，**命中條件改成 `R_SEM = 本次的語意 SHA`**；⑥ **F4 分兩層**：Python 層（hex64、目錄名 ＝ 宣告值、宣告值 ＝ 交接值）與 shell 層（重算、四檔集合、重算值 ＝ 宣告值）——⛔ 不再寫「Python 執行需要 git 的重算」。同步：`STAGE2_INJECTED_ARGS`、③「七之四」的 CLI matrix、`patch_claims()`、summary、shell 的欄位解析、`stage2_finalizer_argv.json` |
| counterfactual patch 的格式 | ⑦a 把 `counterfactual_e1cbbbd.patch` 換成 `--full-index` 格式（內容不變、SHA 會變；② 實測的 `ef7a4cdf…` 從此是歷史值）；⑨ 照樣做 guard 與封存 |
| 版控與漂移（⚠️ 第一輪 review 改寫） | ⑦a 起 `python/baselines/i074_stage2/tooling_e1cbbbd.patch` 進版控。⚠️ **流程**（⑦a～⑦d 只要動到 tooling 路徑就照做，維持單一 commit、⛔ 不先 commit 未 review 的程式）：① stage 待 review 的程式碼 → ② `git write-tree` 得 proposed source tree → ③ 產生器對該 tree 產生 patch → ④ stage patch → ⑤ 使用者 review 整份 staged 結果 → ⑥ commit → ⑦ 驗 **HEAD 的 tooling 路徑投影 ＝ ② 的 tree 的投影**，且產生器對 `HEAD^{tree}` 的輸出與已 commit 的 patch 逐位元相同。**漂移測試**：產生器對「目前 index 的 tree」的輸出必須 ＝ index 中的 patch |
| 封存 | ⑨ 與 counterfactual **一起封存**；之後任一份的 bytes 或 SHA 再變，⑨-1、⑨-2 就要重跑 |
| 「⛔ 不得含判定變更」 | ⑨ 新增 **tooling 非語意 guard**：同一份 fixture、一般 Stage 2（⛔ 不帶 flag），分別在 `e1cbbbd` 與 `e1cbbbd` ＋ tooling 上跑，comparison／report 除 provenance 之外**逐位元相同**；⑨ 的 differential guard 的 patched 側**含 tooling** |
| runner 的前置守門 | docker 之前斷言**兩份** patch 的 raw bytes SHA 各自 ＝ 增量 canonical SHA（現行在 replay 之前只驗 counterfactual——tooling 不 canonical 要等 finalize 才被擋，那時已燒掉約 180 分鐘） |

##### 三、四包的範圍、順序與驗收

| 包 | 範圍 | 驗收（測試 id） |
|---|---|---|
| **⑦a replay 側** | `evaluation.py`：`--i074-counterfactual`（五處同步，「二、④」）、`--counterfactual-patch-sha256` 與 Python 成對守門五條、一趟串流 loader（`stream_after_artifact`，只常駐 cohort rows）、全量 key 守門、`CounterfactualEffectCheck`（重用）→ **rc=6 ＋ `bounded_diagnostics.json`**、rc=0 時寫 `before_source_artifact`／`comparison_artifact`／`report`（report 最後寫；檔名取自 `stage2_archive` 的 `OPERATIONAL_*` 常數；終態**恰好一種**）；`publish.py` 新增 `EXIT_COUNTERFACTUAL_INEFFECTIVE = 6`；runner／`replay-args.sh`：`COUNTERFACTUAL_PATCH`、固定順序、共用合成函式、注入 SHA、flag 納入 `I074_MODE`；`finalize-stage2-evidence.sh` 改呼叫共用函式；failed record 的查找鍵改成語意 SHA（`stage2_archive.py` 的 `failed_record_dir_name()`、封閉 schema 加欄與 builder／validator／summary、F4、`--check-failed-record`；改動檔案集合的四檔不變條件；⚠️ 第三輪 review：語意 SHA 的 shell → Python 交接——`--verified-counterfactual-semantic-sha256`、`VerifiedComposition`、`check_verified_composition()`、`patch_claims()`、summary、shell 欄位解析、CLI matrix 與 argv fixture）；產生器、tooling patch、counterfactual 換格式；Stage 2 argv fixture（`python/scripts/fixtures/stage2_argv.json`） | 「六、2」a～i、o～y、aa～ac2；決策 4 的真實路徑測試與 smoke；ba 的 runner 層；`e1cbbbd` ＋ 兩份 patch 的既有測試全綠；惡意 config 測試；語意鍵（只改測試 → 鍵不變、仍命中；只改測試的 counterfactual → 中止；⚠️ 第二輪 review：多出白名單與兩個測試檔以外的任何檔案（例如文件、fixture、腳本）→ 拒絕；failure record 的 `counterfactual_semantic_sha256` 缺欄、多欄、非 hex64、宣告值 ≠ 重算值 → 各自拒絕；⚠️ 第三輪 review：交接參數由使用者傳入（spoof）或重複 → 拒絕、應帶而缺 → 拒絕、在不該帶的模式出現 → 拒絕、record 欄位 ≠ 交接值 → 拒絕，以及 **shell 驗完之後替換 patch**（TOCTOU）→ 不發布、不 fsync；⚠️ 第四輪 review：③ 的 n、o、**o2**、ab、al、am 依語意 SHA 改寫——`--check-failed-record` 那一層在 ⑦a，「在 replay 之前」那一層在 ⑦b）；⚠️ **`e1cbbbd` 既有的產品測試，斷言一律不改**（它們會在 `e1cbbbd` ＋ patch 的 worktree 裡跑；第一輪 review 限縮範圍）——釘住舊 canonical bytes 的契約測試依新定義更新 |
| **⑦b supervisor＋orchestrator＋freeze record** | `scripts/run-i074-stage2.sh`（入口 → supervisor → 持鎖階段 → 複本內 orchestrator）；supervisor（「八之一之二」第 1～12 列）；label shim；preflight（③「七之三」第 0～7 列，含磁碟檢查與常數）；分流（依**磁碟事實**判終態——`--publish-failed-record` 成功也回 1，⛔ 不能看結束碼）；`--resume`（只重跑 finalize，ae）；freeze record 的寫入端（sizing harness 改用真實 tooling、算 `--full-index` SHA、`--formal` 且 `ok` 才寫）與驗證端；端到端結束碼實作到「複本內終態」為止，**晉升先用固定回 9 的 stub** | n1～n8、n7b、n10、n12 的 preflight 部分、ad、ae、ax、ba 的 orchestrator 層、n／ai／ay 的「在 replay 之前」 |
| **⑦c `--promote`＋判讀器** | 晉升（「八之三」的七步判定順序、8／9、信任根綁定）；`finalize-stage2-evidence.sh` 新增**唯讀**模式 `--verify-promotion-staging`（錨點取自複本、以目的地名稱驗 F4、路徑限在真正 repo 的 `i074_stage2/` 直屬下、唯讀掛載；同步 ③「七之四」的 CLI matrix 與 `stage2_finalizer_argv.json`）；把 stub 換成真正的晉升、完成端到端結束碼；`replay_bundle/stage2_verdict.py` ＋ `scripts/judge-i074-stage2.sh`——**判讀程式碼與錨點都從 `base_commit` 以 `git archive` 取出**、在 Stage 2 image 內執行（判讀規則在結構上一定是 ⑩ 之前寫好的那一版）（⚠️ **⑦c 細部計畫 v1（✅ 2026-10-01 確認）**：改成 `git clone --no-hardlinks` ＋ detached checkout `base_commit`——③ 的合成守門要在 repo 裡套 patch；驗證與判讀在同一個 Python 程序（驗證模式的 `--judge`）） | n9、n11、n12、「六、9」a～m |
| **⑦d memory harness** | `scripts/i074-stage2-acceptance.sh`：重用 sizing 的量測原語，在 repo 外的隔離複本跑 success／failure 兩條實際流程；replay 程序如何產出 13,417 列而⛔ 不必跑三小時、⑩ 實際峰值怎麼記錄（⛔ 不改 ⑩ 的 docker argv），由細部計畫定；⚠️ **只做開發驗證**，正式驗收在 ⑨-1 | 「六、1」的 a～i（開發驗證）；與 label shim 的互斥。⚠️ **⑦d 細部計畫 v1（✅ 2026-10-02 確認）**：replay 有 stub 與 full 兩種計算（full 是 ⑨-1 的量測趟）；⑩ 的實際峰值由外部唯讀的 observer 記錄（下界） |

##### 四、跨包介面

| 項目 | 規則 |
|---|---|
| 執行目錄 `<work>` | 必須**新建**、canonical path、在 repo 外、與 Docker Root Dir **同一個裝置**。layout：`repo/`（複本，detached 在 freeze record 的 `repo_head`）、`freeze/`（freeze record 與報告的副本）、`run/patches/{counterfactual,tooling}.patch`（凍結副本，取自**複本的常數路徑**，⛔ 使用者不得指定）、`run/stage2/`（runner 的 `--output-dir`，⛔ 預先不放任何東西）、`tmp/`（TMPDIR）、`bin/`（label shim）、`logs/`、`state/`（`--resume` 需要的 durable 標記，⑦b 定）。finalize 以 `--run-dir <work>/run` 呼叫，⛔ 不帶 `--source-ref`（`base_commit` 必須 ＝ 複本的 HEAD，那是信任根） |
| 公開 argv | `run-i074-stage2.sh --freeze-record <path> --work-dir <dir>`；`--resume --work-dir <dir>`；`--promote --work-dir <dir>`（⑦c）。內部階段的 argv 由 ⑦b 定；直接呼叫一律被 `/proc/locks` 的驗證擋下 |
| 檔名 | supervisor：`scripts/lib/i074-stage2-supervisor.py`（host 端、只用標準庫、**相容 Python 3.9**——host 是 3.9.2）；label shim：`scripts/lib/i074-stage2-docker-label-shim.sh`；freeze record 驗證：`python/scripts/i074_stage2_freeze_record.py`（標準庫）；磁碟檢查與常數：`python/scripts/i074_stage2_preflight.py`；晉升的 host 邏輯：`python/scripts/i074_stage2_promote.py`（⑦c）。⚠️ 入口與持鎖階段執行的**每一個檔案**都先驗內容 ＝ HEAD 中的版本 |
| supervisor ↔ orchestrator 協定 v1 | 環境變數 `I074_STAGE2_TOKEN`、`I074_STAGE2_SUPERVISOR_PID`、`I074_STAGE2_SUPERVISOR_START`、`I074_STAGE2_REAL_REPO`（必須與複本的 `origin` 相符）、`I074_STAGE2_REAL_DOCKER`、`I074_STAGE2_MODE`；傳進來的環境若已帶任何 `I074_STAGE2_*` → supervisor 拒絕；鎖檔與 sentinel 的路徑是常數（`/run/lock/i074-stage2.lock`、`/run/lock/i074-stage2.active`）；`/proc/locks` 的驗證寫在 `run-i074-stage2.sh` 內（只用 shell 與 host 標準庫）。⚠️ 協定要**跨版本穩定**——單獨執行 `--promote` 時可能是新版 supervisor 帶舊版 orchestrator |
| 環境變數的清理 | 清掉 `GIT_*`、`DOCKER_*`、`REPLAY_DRY_RUN`、`MEASURE_PEAK`、`TOOLING_PATCH`、`COUNTERFACTUAL_PATCH`、`PY_IMAGE`、`AFTER_REF`、`REPLAY_ARGS_SELFTEST`、`MEM*`、`CPUS`、`SIZING_*`、`I074_SIZING_FAULT`；固定帶 `PYTHONDONTWRITEBYTECODE=1`（⚠️ **⑦b 細部計畫 v1 第一輪 review（✅ 2026-09-30 確認）**：另清 `LD_*`、`BASH_ENV`、`ENV`、`PYTHON*`，並把 PATH 固定成 `<work>/bin:/usr/bin:/bin`，見 ⑦b「二之三之一」）。⚠️ 兩份 patch 的環境變數**只傳給 runner 的那一次呼叫**，並有測試證明 finalizer 收不到（`finalize-stage2-evidence.sh` 會對 HEAD worktree 套 `TOOLING_PATCH`） |
| label | 鍵 `i074.stage2.run`；**唯一注入點是 PATH docker shim**（依 v29 決策表第 7 列在本總綱定案；⛔ runner／finalizer 不改）：對 `run`、`create`、`container run`、`container create` 在子指令之後插入**恰好一個** label，再 `exec` 真正的 docker（I/O 透明）。拒絕：子指令之前有全域選項、出現 `--label-file*`、任何 token 含 `i074.stage2.run`（一次涵蓋 `-l`／`--label=` 等寫法）、token 不是 64 位小寫 hex 或與 sentinel 內的不符。真正的 docker 由 supervisor 解析「PATH 扣掉 shim 目錄」之後的 realpath 交給 shim，shim 發現它指向自己就拒絕（⚠️ **⑦b 細部計畫 v1 第一輪 review（✅ 2026-09-30 確認）**：呼叫者的 PATH ⛔ 不是可信來源，改成固定的 `/usr/bin/docker` ＋ 信任條件，見 ⑦b「二之三之一」與「三」#18）；supervisor 自己的 `docker ps`／`rm` 用絕對路徑。與 sizing shim **明文互斥**（兩邊各自偵測對方並拒絕）。靜態測試：⑩ 呼叫圖裡的腳本⛔ 不得以絕對路徑呼叫 docker、⛔ 不得用 `command -p`、⛔ 不得改寫 PATH。殘留檢查之前先驗 image 的 `Config.Labels` 不含這個鍵。⚠️ 已知限制：直接呼叫 `/usr/bin/docker` 攔不到 |
| 結束碼 | 常數集中在 `replay_bundle/publish.py`：6（⑦a）、8、9（⑦c）；只用標準庫的腳本各自鏡像一份，由測試斷言相等。replay 在反事實路徑只可能回 0／1／2／6（rc=4 一律是缺陷）；端到端結束碼依「八之三」；⑦b 的細部計畫給出「模式 × 失敗點」的完整表（鎖之前的各種失敗、鎖衝突、sentinel、殘留容器、128＋N、137） |
| freeze record v1 | 「八之二」加 `tooling_patch_raw_sha256`、`tooling_patch_sha256`（兩者必須相等）；兩種 counterfactual SHA 的 canonical 改用「二」的定義（`--full-index` ＋ 釘死參數 ＋ 暫存 bare repo）；寫入端歸 ⑦b，`--formal` 的 clean 清單涵蓋新模組 |

##### 五、測試落點

| 測試對象 | 落點 |
|---|---|
| `evaluation.py`、`replay_bundle/`、`python/scripts/` 的純邏輯（freeze record、磁碟檢查、晉升邏輯、判讀） | pytest（docker 內，`python/scripts/test.sh`） |
| runner／finalizer 的 argv 與 mount | `scripts/test-replay-args.sh`（既有） |
| supervisor、orchestrator、晉升的整合 | **新增 `scripts/test-i074-stage2.sh`**（host 端 shell）＋ host 端 `python3 -m unittest`（相容 3.9；host 沒有 pytest，測試 image 裡沒有 git 與 docker CLI）；由 `python/scripts/test.sh` 在 `SKIP_SHELL_TESTS` 那一段呼叫 |

⚠️ **測試模式⛔ 沒有正式環境的覆寫口**——以環境變數覆寫鎖目錄，等於一個事實上的「解除入口」（換個目錄就繞過 sentinel），
違反 v29 決策表第 9 列（固定執行帳號 ＋ 重開機才解除）、「八之一之二」第 9 列（⛔ 沒有解除入口）與 n7b 的「正式模式⛔ 不接受覆寫」。改成：測試把腳本**複製到隔離的最小 repo、以 sed 改掉常數**（鎖檔、sentinel、uid、label 鍵），並斷言與正式
檔案**只差那幾行**。⛔ 測試**絕不碰 `/run/lock`**（SIGKILL 那一支若留下正式 sentinel，就只能重開機）；⛔ **絕不建立帶正式鍵的真實容器**
（殘留會擋住 ⑩ 的啟動檢查）。

##### 六、風險與回滾

| 風險 | 對策 |
|---|---|
| tooling patch 與 HEAD 漂移，⑩ 跑到舊版工具 | 漂移測試 ＋ 產品碼不變條件；⑨ 封存之後再變就重跑 ⑨-1、⑨-2 |
| tooling patch 夾帶判定變更 | tooling 非語意 guard ＋ differential guard 的 patched 側含 tooling（⑨） |
| 改用 `--full-index` 牽動 ③ 已實作的程式 | 只影響**非空** diff；Stage 1 與 `envcheck/` 的 tooling 都是空的，已封存的證據不受影響；⑦a 的測試涵蓋 |
| git config、屬性或 git 版本讓同一份 diff 產生不同 bytes（第一輪 review） | 暫存 bare repo ＋ 隔離環境 ＋ 釘死參數；惡意 config 測試；git 版本的差異由 runner 在 docker 之前 fail-closed |
| failed record 的查找鍵被非語意變更繞過（第一輪 review） | 語意 SHA（排除測試檔）；殘餘（只改產品檔的註解或空白）照實寫明，⛔ 不宣稱語意等價 |
| ⑤ 的 `P_B` 沒有算到 tooling patch | ⑨-1、⑨-2 以真實兩份 patch 實測；超過 `P_B_BUDGET` 就走「六、1」的回退順序 |
| 包與包之間的過渡狀態 | ⑩ 在 ⑨ 之前不可能執行；⑦b 的晉升 stub 固定回 9，⛔ 不會把證據搬進真正 repo |
| 測試誤觸正式的鎖或 sentinel | 隔離 repo ＋ sed；⛔ 沒有執行期覆寫口 |
| host 只有 2 GiB | 一律用 repo 腳本、依序執行 |
| **回滾**（⚠️ 第二輪 review 改寫） | ⛔ **不保證任意 commit 單獨 revert 仍一致**——⑦a 同時改共用合成函式、canonical SHA、counterfactual 的格式、failed record 的 schema 與測試，⑦b～⑦d 又依賴新格式。規則：⑦b 開工之前，只能**整包 revert ⑦a**；⑦b 之後已有依賴，要依**反向順序** revert（⑦d → ⑦c → ⑦b → ⑦a），並重做受影響的封存與量測（⑨、⑨-1、⑨-2） |

##### 七、對已確認章節的修改（⚠️ 都加註「⑦ 總綱 v1」；✅ 2026-09-30 確認）

| 位置 | 修改 |
|---|---|
| v29「三」受影響檔案 | `evaluation.py`、streaming loader 兩列加註「經 tooling patch 進入 replay」；新增 tooling patch 與產生器一列；`replay-args.sh` 一列加共用合成函式；supervisor 與 label 注入點兩列填入檔名 |
| v29「六、1」 | 補註：⑤ 量 `P_B` 時 tooling 是空的，由 ⑨-1、⑨-2 驗 |
| v29「六、2」的 u | 屬 runner 層 |
| v29「八」 | ⑦ 列加總綱指引；⑧ 改寫；⑨ 改成封存兩份 patch、加 tooling 非語意 guard、「既有測試全綠」改成套兩份 patch 之後；⑨-1、⑨-2 改用真實的兩份 patch |
| 「八之一」 | 訂正：replay 本身（含 `evaluation.py` 與 `replay_bundle/`）是從 `e1cbbbd` worktree 執行；「執行位置」寫明兩份 patch 取自複本的常數路徑、⑩ 的 tooling ⛔ 不得為空 |
| 「八之二」 | 加 tooling 兩欄；counterfactual SHA 的 canonical 改用「二」的定義；交叉不變條件與驗證同步 |
| ③「四之一」 | canonical diff 改用「二」的定義；空 tooling 限 runner 層 |
| ③「七之三」、③ 測試表的 ba | 空 tooling 的規則限 runner 層；ba 拆兩層 |
| ⚠️ 第一輪 review：③「三之零」「三之二」的 failed 目錄、「七」的位置列、**schema 列**（第二輪補：封閉欄位加 `counterfactual_semantic_sha256`）與 F4、重跑守門、「七之一」的比對鍵、「七之三」preflight 第 5 列、「八之三」的晉升對象與步驟 3、「二、①」的重跑條件與守門時機 | 查找鍵改成語意 SHA（本總綱決策表第 9 列；第二輪訂正為產品檔白名單 ＋ 四檔不變條件） |
| ⚠️ 第四輪 review：③ 測試 n、o、o2（新增）、ab、al、am；「七之三」的重跑資格表與 recovery 的重跑資格；兩個鍵一致性的說明；v29「六、1」的隔離規則、n12、z、「八之一」的真正 repo 列 | 重跑資格一律以語意 SHA 判定 |
| ⚠️ 第五輪 review：v29「六、2」n12 的句尾、「正式 scan 的計次裁決」的 before 重跑列 | 同上（計次政策明寫「只改測試檔不得取得重跑資格」） |
| ⚠️ 第三輪 review：③「七」的 F4（分成 Python 層與 shell 層）、「七之一」的比對鍵與命中條件、「七之四」的 CLI matrix | 語意 SHA 的 shell → Python 交接（「二」的交接列） |
| ⚠️ 第一輪 review：③「四之一」、「八之二」、「六、4」、「七之一」、I-100 的 `tooling_patch_sha256` 定義 | canonical diff 改指向本總綱「二」的完整定義（⛔ 不只 `--full-index`） |

##### 八、歸檔位置

| 內容 | 歸檔到 |
|---|---|
| ⑩ 的正式執行程序（orchestrator、supervisor、`--resume`、`--promote`、判讀器、重開機的條件） | `development-workflow.md`，新增「I-074 Stage 2 的正式執行程序」；sizing 一節依 v29 改寫 |
| 反事實路徑、tooling patch 與 `--full-index` canonical、晉升、判讀規則的契約 | `sr-zone-scoring.md` |
| ⚠️ 現行主題文件裡的 canonical diff 定義（⛔ 未確認的規則不先寫進主題文件——**隨 ⑦a 實作一併改寫**） | `sr-zone-scoring.md` 的三方 SHA 區塊（`counterfactual_patch_sha256 = sha256(git diff --binary …)` 三行）；`development-workflow.md` 的 `tooling_patch_sha256` 定義列、`git apply --index` 的說明、`--check-failed-record` 的比對鍵 |
| ⚠️ 現行主題文件裡的 failed record 目錄鍵（第一輪 review；同樣**隨 ⑦a 實作一併改寫**） | `sr-zone-scoring.md` 的 failed-attempt record 路徑（`failed/<bundle_id>-<counterfactual_patch_sha256>/` 改成語意 SHA）與重跑守門的說明 |
| ⚠️ 現行主題文件裡的 `--verified-*` 交接說明（第三輪 review；同樣**隨 ⑦a 實作一併改寫**） | `sr-zone-scoring.md` 與 `development-workflow.md` 描述「patch base 與三個 SHA 以 `--verified-*` 注入」的段落——補上語意 SHA 的交接與模式限制 |
| 各包的細部計畫與實作結果 | 本筆 I-074 |

#### Stage 2 步驟 ⑦a 細部計畫 v1（2026-09-30，✅ **已確認**（2026-09-30，review 三輪後通過並 commit））

⚠️ 依「Stage 2 步驟 ⑦ 總綱 v1」（✅ 2026-09-30 review 通過並 commit）拆出的第一包「replay 側」。範圍、驗收 id 與新裁決以總綱「二」「三」為準；
本細部計畫只補**程式設計、呼叫順序、測試落點**，以及總綱沒寫到、實作時必須決定的細節（列在「三」，✅ 2026-09-30 確認）。
⛔ **細部計畫 review 通過並 commit 之前⛔ 不改任何程式**；實作完成後依總綱「二」的版控流程 stage、⛔ 不 commit，停下等 review。

**⑦a 細部計畫 v1 第一輪 review 的修正（2026-09-30）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | ⛔ **B4 把 cohort 的三種物件混成同一個名稱**——loader 回傳的 `EvidenceLoad`、它的 `.parsed`（payload）、`validate_cohort_manifest()` 回傳的 tuple keys 都寫成「cohort」，並直接傳給 `keep_keys` 與 `assert_same_keys()`；照字面實作，`validate_cohort_manifest()` 或 `keep_keys` 會收到錯誤型別 | ✅ B4「replay 之前」改成明確的 `cohort_load` → `cohort_payload = cohort_load.parsed` → `cohort_keys = validate_cohort_manifest(cohort_payload)` → `stream_after_artifact(…, keep_keys=set(cohort_keys))` → `assert_same_keys(cohort_keys, after.candidate_keys, …)`；SHA 比對讀 `cohort_payload`、comparison 用 `sorted(cohort_keys)` |
| 中 | ⛔ **after／cohort 的 provenance 沒有驗**——`stream_after_artifact()` 把 role 留給呼叫端、`validate_cohort_manifest()` 只驗它是 dict，「三」#3 卻直接信任 after 的 `provenance.base_commit` | ✅ replay 之前、「三」#3 之前，兩份都跑 `validate_provenance(…, role="stage1")`——實查 `load_stage1_anchor()` 對 cohort 與 after 用的正是 `stage1`，讀取順序也照它（cohort 先讀、after 串流）；「五」補兩支：after／cohort 的 provenance 損壞 → 在 `_replay_from_bundle()` 之前中止 |
| 測試 | ⚠️ **smoke 可能以空 cohort 通過**——現行 smoke 的合成 bundle 是單調上漲的 K 棒，⛔ 不會出現「跌破支撐再收復」，幾乎不可能產生 candidate；「rc=0、恰好三檔」只證明得了 patch 套用、CLI 啟動與空 comparison 的產出 | ✅ 使用者裁決（2026-09-30）：**調 fixture，查不通就退回**。⚠️ **已查證可行**（見 B7 的實測）：從已進版控的正式 bundle 切出 6243 最後 250 根，Stage 1 得 165 列、**6 筆候選**（與 D+1 cohort 在該期間的 6 筆逐 key 相同）；反事實程式跑同一份切片 → 候選 0、6 筆全數翻轉、cohort 以外 159 列逐列相同。B7 改用這份切片，並斷言 comparison 非空且逐列翻轉 |

**⑦a 細部計畫 v1 第二輪 review 的修正（2026-09-30）**：

| # | 問題 | 修正 |
|---|---|---|
| 中 | ⛔ **衍生 bundle 的 manifest 規則沒有定義完整**——`build_manifest()` 還要 readiness、calendar metadata、`captured_at`、provenance；⛔ 尤其不能沿用原 11 檔 bundle 的 provenance，否則等於宣稱衍生 bundle 是原 Stage 0 的 argv／image 產出的 | ✅ B7 新增「切片 fixture 的契約」表：每個欄位的來源（一律取自 `load_bundle()` 驗過的來源）、`captured_at` 的規則、smoke 專用 provenance 與**用途界線**（⛔ 不是證據）。⚠️ **以這些規則逐字重做一次（2026-09-30）**：bundle_id 仍 ＝ `b1_20260901_1d_de3ab843_a7c9ffb4`（`captured_at` 與 provenance ⛔ 不影響 ID） |
| 中 | ⛔ **「唯讀」與「有界」沒有落成可驗證的守門**——現行 smoke 產 bundle 那一步把整個 `python/` 以可寫方式掛進容器；切片若失效，完整的 11 檔 bundle 會被帶進兩趟約 136 秒的 replay | ✅ B7 新增「守門」表：`/app:ro`、只有 `/out` 可寫（既有 `make_smoke_bundle.py` 那一步一併改成 `:ro`）；輸出⛔ 不得落在來源 bundle 內；發布後重新 `load_bundle()`，斷言 ID ＝ 常數、`symbols` ＝ `["6243"]`、candles 恰好 250 列且全是 6243；smoke 在 **replay 之前**再驗一次 ID；前後比對來源的 `manifest.sha256` 與 `git status`。「五」補產生器的 pytest（正向 ＋ 輸出落在來源內的負向，負向用來源的暫存複本，⛔ 不碰真正的 bundle）。⚠️ 重做時已在 `/app:ro` 下跑通，來源的 `manifest.sha256` 與 `git status` 前後不變 |
| 低 | 「五」provenance 那一列寫「`base_commit` ≠ `base_commit`／`before_ref`」，容易誤讀 | ✅ 改成「after artifact 的 `provenance.base_commit` ≠ CLI 的 `--base-commit` 或 `--before-ref`」；B4 的 ④ 與「三」#3 同步寫明哪一個是 CLI、哪一個是 artifact |

**⑦a 細部計畫 v1 第三輪 review 的修正（2026-09-30）**：

| # | 問題 | 修正 |
|---|---|---|
| 低 | ⛔ **宣稱佔位 provenance「讓它無法冒充正式 bundle」，技術上不成立**——實查 `load_bundle()` 完全不讀 manifest 的 provenance，Stage 1 載入 bundle 後也不驗它；而這份 fixture 本來就會被 smoke 的 Stage 1 實際載入 | ✅ 「用途界線」改寫：佔位值只用來**標示** smoke／非正式身分；真正的界線由暫存目錄、⛔ 不進版控、⛔ 不進 finalize 與正式歸檔維持；⛔ 不宣稱 loader 會拒絕它 |
| 低 | ⛔ **事實錯誤**：寫「全 0 的佔位值（比照 `make_smoke_bundle.py`）」，但現行腳本的 `runner_sha256` 是 `"s" * 64`——實查它連 `validate_provenance()` 的 64 碼小寫 hex 都通不過 | ✅ 刪掉「比照」，明寫新產生器改用**合法的**全 0 hex（`image_digest` ＝ `sha256:` ＋ 64 個 0、`runner_sha256` ＝ 64 個 0）；「五」的正向測試加 `validate_provenance(…, role="stage0")`。⛔ 不順手改 `make_smoke_bundle.py`（⑦a 範圍外） |

**現況（2026-09-30 實查）**：

| 項目 | 事實 |
|---|---|
| `e1cbbbd..HEAD` 的產品碼 | `python/` 的差異只在 `replay_bundle/`（與新增的測試、`python/scripts/`、`python/baselines/`）；`evaluation.py` **無差異**——總綱「一之一」成立 |
| counterfactual patch | `counterfactual_e1cbbbd.patch` 的 `diff --git` 恰好四個檔（兩個產品檔 ＋ 兩個測試檔）；`index` 行是**縮寫**的舊格式（`ef7a4cdf…`）；`T1` ＝ `a1649733b910ae90f08efc5f215f31f898619f52` |
| ③b／③d 已完成、⑦a 重用 | `CounterfactualEffectCheck`、`build_before_source()`、`build_counterfactual_failure()`／`validate_counterfactual_failure()`、`OPERATIONAL_*`、`stream_after_artifact()`、`VerifiedComposition`／`check_verified_composition()` |
| 尚未有 | `EXIT_COUNTERFACTUAL_INEFFECTIVE = 6`；`--i074-counterfactual`；runner 的第二份 patch；語意 SHA；tooling patch 與產生器 |
| canonical diff 的現況 | **四處各算一份**：`replay_args_tooling_patch_sha256()`（worktree 對 commit 的 `git diff --binary`）、`finalize-stage2-evidence.sh` 的 `compose_check()`、`--check-failed-record` 的比對鍵、sizing harness |
| 環境 | host 是 git 2.30.2、Python 3.9.2、2 GiB RAM（測試一律用 repo 腳本、依序跑）；真正 repo 已登記 177 個 worktree（[I-118](#i-118replay-相關腳本會洩漏-git-worktree註冊與-tmp-目錄都會累積) 的既有洩漏），⑦a 的測試⛔ 不得再增加 |

##### 一、目標與⛔ 不做的範圍

| 項目 | 內容 |
|---|---|
| 目標 | 總綱「三」⑦a 列的全部項目與驗收 id |
| ⛔ 不做 | ⑦b～⑦d（supervisor、orchestrator、freeze record、sizing 改用真實 tooling、晉升、判讀器、memory harness）；⑧ 的矩陣稽核；⑨ 的封存與兩道 guard；任何正式 replay；`i074-stage2-sizing.sh`／`i074_stage2_sizing.py` ⛔ 不改（只驗它們不受影響——sizing 以集合差找 failed record，⛔ 不依賴目錄名的鍵）；一般 Stage 1／2 路徑的行為（測試 o） |
| ⚠️ 過渡狀態 | ⑦a commit 的 tooling patch **不是** ⑨ 封存的那一份（⑦b～⑦d 還會動 `replay_bundle/`，每次都依版控流程重產）；counterfactual 換格式之後 `ef7a4cdf…` 成為歷史值 |

##### 二、設計（依實作順序 B1～B8）

**B1　共用合成函式（`scripts/lib/replay-args.sh`）**

新增常數（⚠️ 唯一定義處）：`I074_CF_PRODUCT_PATHS`＝`python/backtest/modular/sr_scoring/decision_engine.py`、`python/backtest/modular/sr_scoring/lifecycle_engine.py`；
`I074_CF_FILES`＝上面兩個 ＋ `python/backtest/modular/sr_scoring/tests/test_i074_diagnostics.py`、`python/backtest/modular/sr_scoring/tests/test_lifecycle_engine.py`（排序後的四個完整路徑）。

| 函式 | 行為 |
|---|---|
| `replay_args_canonical_diff <worktree 或 git dir> <treeA> <treeB> [path...]` | stdout 輸出 raw diff bytes。`A`／`B` 必須是 40 碼、且 `cat-file -t` ＝ `tree`（⛔ 不接受 commit、ref）。在 **subshell** 內建暫存 bare repo（`git init --bare --template=`，`objects/info/alternates` 指向來源 repo 的 common `objects`），以 `env -i` 起頭、只帶 `PATH`，設 `GIT_CONFIG_NOSYSTEM=1`、`GIT_ATTR_NOSYSTEM=1`，`HOME`／`XDG_CONFIG_HOME` 指向空目錄；diff 的參數**逐字照總綱「二」的唯一定義**；subshell 的 EXIT trap 清掉暫存目錄。common dir 以 `cd` ＋ `pwd -P` 取絕對路徑（2.30 ⛔ 沒有 `--path-format`） |
| `replay_args_canonical_sha256 …` | 同上，取 SHA-256 |
| `replay_args_compose <worktree> <base_commit> <cf 檔或空字串> <tooling 檔或空字串>` | 前提：worktree 由 `replay_args_prepare_worktree()` 建在 base、HEAD ＝ base。依序 `git apply --index` cf → `write-tree` ＝ `T1`（無 cf 時 `T1` ＝ `base^{tree}`）→ tooling 非空才套 → `write-tree` ＝ `T2` → **乾淨檢查**（`git status --porcelain=v1 --untracked-files=all` 取成變數、here-string 比對：⛔ 不得有 `??`、第二欄⛔ 不得非空白——工作樹必須恰好等於 index）→ 斷言 HEAD 仍是 base → 以 `replay_args_canonical_sha256` 算 cf ＝ `(base^{tree}, T1)`、tool ＝ `(T1, T2)`、comp ＝ `(base^{tree}, T2)`。**有 cf 時**：`--name-only -z --no-renames (base^{tree}, T1)` 的集合**恰好**＝ `I074_CF_FILES`（多一個、少一個一律中止）；語意 diff ＝ `(base^{tree}, T1) -- I074_CF_PRODUCT_PATHS` **必須非空**，sem ＝ 它的 SHA。成功才在 stdout 印一行 `T1 T2 cf tool comp sem`（無 cf 時 cf、sem 印 `-`）；cf 檔存在但 0 bytes → 中止 |
| `replay_args_tooling_patch_sha256`（⚠️ **改寫**，簽章不變） | 改成呼叫 `replay_args_compose "$wt" "$base" "" "$patch"` 取 comp；⛔ 不再 `git add -A -N`（乾淨檢查改由 compose 做，而且更嚴）。Stage 1 的 `finalize-evidence.sh`、`compare-replay-crossday.sh` 與 Stage 2 finalizer 自身的 provenance 因此共用同一個定義（空 patch 仍是 `e3b0…`） |

⚠️ 套用端（`git apply`）仍讀真正 repo 的 config；被 config 扭曲時，`T1`／`T2` 的 canonical SHA 會 ≠ raw bytes 的 SHA，由 B5 runner 與既有合成守門的「raw ＝ canonical」**fail-closed** 擋下（⛔ 不會靜默通過）。

**B2　counterfactual patch 換格式**

`counterfactual_e1cbbbd.patch` ＝ `replay_args_canonical_diff <e1cbbbd^{tree}> a1649733…`（內容不變、bytes 與 SHA 會變）。驗證：新 patch 套在 `e1cbbbd` 之後 `write-tree` 仍 ＝ `a1649733…`；檔案集合 ＝ 四檔；`.gitattributes` 的 `-text` 照舊涵蓋。新 SHA 記進實作結果，`ef7a4cdf…` 標為歷史值。

**B3　語意 SHA 與 failed record（`stage2_archive.py`、claims 工具、`finalize-stage2-evidence.sh`）**

Python（⛔ 不碰 git）：

| 項目 | 改動 |
|---|---|
| 封閉 schema | `_FAILED_RECORD_FIELDS` 加 `counterfactual_semantic_sha256`；`validate_failed_record()` 驗 hex64 且 ≠ `EMPTY_SHA256`（⛔ 不得是空 diff 的 SHA） |
| 目錄名與 builder | `failed_record_dir_name(bundle_id, counterfactual_semantic_sha256)`；`build_failed_record(…, counterfactual_semantic_sha256=)` |
| 交接 | `VerifiedComposition` 加選填的 `counterfactual_semantic_sha256`（有就驗 hex64、≠ 空 diff SHA）；`check_verified_composition()` 另外比對 record 的語意欄位（交接值與實際欄位必須同時存在且相等） |
| CLI | 新注入參數 `--verified-counterfactual-semantic-sha256` 進 `STAGE2_INJECTED_ARGS`（重複即拒）；⚠️ **只有** `--publish-failed-record`、`--recover-failed-record` **必須**帶，`--finalize`、`--recover-durability`、`--check-failed-record` 帶了就拒 |
| publish | 目錄名與 record 欄位都取交接值；`verify_staging` 從磁碟重讀 staged record，驗「目錄名 ＝ 宣告值 ＝ 交接值」之後才 rename |
| recover | `verify_failed_record()` 之後、fsync 之前比對「宣告值 ＝ 交接值」 |
| check（Python 段） | F4 的 Python 層：hex64、目錄名 ＝ 宣告值；`_record_summary()` 加 `counterfactual_semantic_sha256` |
| 宣告值 | `patch_claims(mode="failed-record")` 多回宣告的語意 SHA；`i074-stage2-patch-claims.py --failed-record` 印 **5** 個 token（其餘模式維持 4 個） |

shell（`finalize-stage2-evidence.sh`）：

| 項目 | 改動 |
|---|---|
| 注入清單 | `STAGE2_INJECTED` 加新參數 |
| `compose_check()` | 改成「兩份 patch 各讀一次到私有副本 → 新 worktree → `replay_args_compose` → 與宣告值比對」，語意 SHA 放進 `COMPOSE_SEM`；⛔ 不再自己算 diff |
| publish | `VERIFY_ARGS` 加 `--verified-counterfactual-semantic-sha256 "$COMPOSE_SEM"` |
| recover-failed-record | `COMPOSE_SEM` ≠ claims 的宣告值 → rc=1、⛔ 不呼叫 Python；相等才注入 |
| check（shell 段） | 本次輸入以 `replay_args_compose`（tooling ＝ 私有 0-byte 檔）得 `CANON_KEY` 與 `INPUT_SEM`（四檔不符 → rc=1）；每份 record 做 `compose_check`，重算的 sem ≠ record 宣告的 `R_SEM` → rc=1；⚠️ **命中條件改成 `R_SEM = INPUT_SEM`**；之後才做既有的「未命中但輸入 bytes ≠ canonical → rc=1」。訊息改成「只有白名單產品檔的 diff 改變才可重跑」 |
| argv fixture | `stage2_finalizer_argv.json` 的 publish／recover-failed-record 兩組加新參數與 placeholder；測試的 `s2_normalize` 同步 |

**B4　`publish.py` 與 `evaluation.py` 的反事實路徑**

| 項目 | 改動 |
|---|---|
| 結束碼與例外 | `publish.py` 加 `EXIT_COUNTERFACTUAL_INEFFECTIVE = 6`；`stage2_archive.py` 加 `CounterfactualIneffective`（⛔ 不繼承 `ValueError`，比照 `CandidateMismatch`，帶 path）；`__init__.py` 匯出所需名稱 |
| CLI 五處同步（「二、④」） | `--i074-counterfactual`（`store_true`）進 `I074_FLAGS`（重複即拒）、`BUNDLE_ALLOWED_ARGS`、`assert_i074_flags()`（只限 Stage 2）；`--counterfactual-patch-sha256` 進 `SCRIPT_INJECTED_ARGS`、`BUNDLE_ALLOWED_ARGS`、parser（`SUPPRESS`）。⛔ 模式只由 flag 決定 |
| 成對守門（五條） | 新函式，排在 `run_bundle_stage()` **最前面**（`load_bundle()` 之前）：flag 無 SHA／SHA 無 flag／Stage 1 帶 SHA 或 flag／SHA 非 64 位小寫 hex → `CliUsageError`（rc=1）；兩者都在且格式正確才進反事實路徑 |
| 一般路徑 | 未帶 flag 時的程式碼**一行不動**（測試 o） |
| 反事實路徑 | 新函式 `_run_counterfactual_stage2()`，見下表 |
| `main()` | `CounterfactualIneffective` → `EXIT_COUNTERFACTUAL_INEFFECTIVE`（排在一般 `ValueError` 之前）；stdout 摘要加 `mode` 與 `counterfactual_patch_sha256` |

| 階段 | 動作（⚠️ 順序即契約） |
|---|---|
| replay 之前（⚠️ 第一輪 review 改寫：物件名稱與 provenance） | ① `cohort_load = load_canonical_evidence_artifact(args.cohort_manifest, COHORT_KIND)`（`EvidenceLoad`，支援 `.json`／`.json.gz`）→ `cohort_payload = cohort_load.parsed` → `cohort_keys = validate_cohort_manifest(cohort_payload)`（tuple keys）→ `validate_provenance(cohort_payload["provenance"], role="stage1")` → `assert_matches_bundle(cohort_payload, …)`；② `after = stream_after_artifact(args.after_artifact, label=…, side="after", keep_keys=set(cohort_keys))`（**一趟**讀，逐列套 `StreamRowValidator`，只常駐 keys、每列 digest 與 cohort rows）→ `validate_provenance(after.load.top["provenance"], role="stage1")` → `assert_matches_bundle(after.load.top, …)`；③ `cohort_payload["after_artifact_sha256"]` ＝ `after.load.artifact_sha256`（h）→ `assert_same_keys(cohort_keys, after.candidate_keys, …)` → `cohort_keys` 的每個 key 都在 `after.kept_rows`；④ fail-fast（「三」#3，⚠️ 排在 provenance 驗證**之後**）：CLI 的 `--before-ref` 是 40 碼、且 ＝ CLI 的 `--base-commit`（runner 注入的 worktree HEAD）＝ after artifact 的 `after.load.top["provenance"]["base_commit"]`。⚠️ 讀取順序與 role 照 `load_stage1_anchor()` 的既有寫法（cohort 先讀、after 串流；兩者都是 `stage1`） |
| replay | 沿用 `_replay_from_bundle()`、唯一性、`validate_replay_errors()`、`validate_diagnostics(side="before")`；provenance 照舊在 replay 之後建（反事實用到的模組在 package import 時就已載入，`project_modules_sha256` ⛔ 不會少記） |
| 全量 key 守門（⚠️ 先於反事實生效檢查） | `assert_same_keys(universe, after_keys)`、`assert_same_keys(keys, after_keys)`，再加**有序**相等 `keys == after_keys`（測試 b 的「換序」；全圖的全量守門本來就要求同序） |
| 反事實生效 | 以**本次 replay 的 before rows** 逐列餵 `CounterfactualEffectCheck`；不生效 → `build_counterfactual_failure()` → `validate_counterfactual_failure()` → **只**發布 `bounded_diagnostics.json` → 拋 `CounterfactualIneffective`（rc=6）；生效之後再斷言 `candidate_keys(rows) == []`（防禦，違反即 rc=1）。⛔ 永遠不走 `_publish_candidate_mismatch()`（rc=4 在本路徑不可達） |
| rc=0 的輸出 | 檔名取 `Path(OPERATIONAL_*).name`（並斷言 parent 都是 `stage2`——runner 的 `--output-dir` 就是 `<run>/stage2`）：before source（`build_before_source()`，全量、replay 順序、串流寫出）→ comparison（`sorted(cohort_keys)` 逐列 `compare_rows(before, after.kept_rows[key])`，`after_artifact_sha256` ＝ 串流 payload SHA，與 before source 共用**同一個** provenance 物件）→ 落地重讀驗 SHA → report（`report_max_rows` 取 bundle manifest）**最後**寫。⚠️ **終態恰好一種**：rc=0 ＝ 恰好三檔、rc=6 ＝ 只有 `bounded_diagnostics.json` |

**B5　runner（`scripts/run-replay-offline.sh`）**

1. `REPLAY_INJECTED_ARGS` 加 `--counterfactual-patch-sha256`（使用者傳入或縮寫即拒）。
2. 偵測 `--i074-counterfactual`（⛔ 不接受 `=value` 形式）→ 納入 `I074_MODE`（⛔ 不自動 pin、必須有 `REPLAY_IMAGE_ID` 與 identity）。
3. **建 worktree 之前**的 truth table：flag ＋ `COUNTERFACTUAL_PATCH` 空 → 1；無 flag ＋ 非空 → 1；flag 但不是 Stage 2（after／cohort 未成對）→ 1；flag 但 `I074_STAGE` ≠ 2 → 1（「三」#4）。
4. **凍結**：私有 `mktemp -d`（EXIT trap 清）；`COUNTERFACTUAL_PATCH` 必須是一般檔案且非空、讀一次；`TOOLING_PATCH` 有就讀一次、沒有就建 **0-byte** 副本（ba 的 runner 層）；之後一律只用凍結副本。
5. `replay_args_prepare_worktree()` → `replay_args_compose "$WORKTREE" "$BASE_COMMIT" <凍結 cf 或空> <凍結 tooling>`。
6. 反事實模式在 docker 之前斷言 `sha(凍結 cf) ＝ cf`、`sha(凍結 tooling) ＝ tool`（總綱「二」的 runner 前置守門；四檔集合與語意 diff 已由 compose 驗過）；一般模式只取 comp。
7. `--tooling-patch-sha256` ＝ comp；`replay_args_offline()` 加第 6 個位置參數 `<cf SHA 或空>`，非空時在 `--runner-sha256` 之後注入 `--counterfactual-patch-sha256`；其餘呼叫端同步。
8. dry-run 另印一行 `==> patches: counterfactual=<sha|-> tooling=<sha> composed=<sha>`（測試 u／ba 用）；結束碼照舊原樣傳出（含 6）。

新增 `python/scripts/fixtures/stage2_argv.json`：`stage2_counterfactual_argv`（注入的五項 ＋ `--counterfactual-patch-sha256 <CF_SHA>` ＋ 使用者參數，`--i074-counterfactual` 原樣在尾端）。

**B6　tooling patch 產生器與版控**

`scripts/make-i074-tooling-patch.sh`——常數：base ＝ `e1cbbbdab44f8cf2d152e6ade9235d844f590d7f`、counterfactual 的路徑、tooling 路徑集合（`evaluation.py`、`replay_bundle/`）、輸出路徑 `python/baselines/i074_stage2/tooling_e1cbbbd.patch`。

| 模式 | 行為 |
|---|---|
| `--source-tree <40 碼 tree>` | ⛔ 不讀 HEAD、⛔ 不讀工作樹。worktree 建在 base → `replay_args_compose`（cf ＝ **來源 tree 裡的** cf blob，tooling 空）得 `T1` → 在 index 內 `rm --cached` tooling 路徑、`read-tree --prefix=…/replay_bundle/ <來源>:…/replay_bundle`、`update-index --cacheinfo` 放入 `evaluation.py` → `write-tree` ＝ `T2` → 驗總綱的四條不變條件（① 交集為空；② `T2` 在 tooling 路徑上的 tree／blob OID ＝ 來源；③ `--name-only (T1, T2)` ⊆ tooling 路徑；④ `(base, 來源) -- python/` 扣掉 `baselines/`、`scripts/`、`sr_scoring/tests/`、`replay_bundle/`、`evaluation.py` 之後沒有差異）→ `replay_args_canonical_diff (T1, T2)` 寫到暫存檔 → **自我驗證**：新 worktree 以 `replay_args_compose(cf, 產出)` 重建，`T2'` ＝ `T2` 且 tool ＝ sha(產出) → 成功才把 bytes 印到 stdout（失敗 stdout ⛔ 無輸出） |
| `--verify <40 碼 tree>` | 對該 tree 產生，與 `<tree>:python/baselines/i074_stage2/tooling_e1cbbbd.patch` 逐位元比對；0 ＝ 相同，1 ＝ 不同或缺檔 |

所有暫時 worktree 由 EXIT trap 移除；測試斷言真正 repo 的 worktree 數量前後不變。

**版控流程**（總綱「二」；⑦a～⑦d 只要動到 tooling 路徑就照做）：`git add` 程式 → `T=$(git write-tree)` → `--source-tree "$T"` 輸出到暫存檔 → 放到輸出路徑並 `git add` → 使用者 review 整份 staged 結果 → commit →
`--verify "$(git rev-parse HEAD^{tree})"`，並比對 HEAD 與 `T` 的 tooling 路徑投影。**漂移測試**（`test-replay-args.sh`）：`--verify "$(git write-tree)"`（index 的 tree）。
⚠️ 漂移測試在開發途中（index 還沒有新 patch）**必然失敗**，是預期行為；完整驗收在 stage 之後跑。

**B7　smoke 與 `e1cbbbd` 既有測試**

| 項目 | 做法 |
|---|---|
| smoke（決策 4；⚠️ 第一輪 review 改寫） | ⚠️ **fixture 改用正式 bundle 的切片**（⛔ 不用合成的單調 K 棒——它不會產生 candidate）：新增 `python/scripts/make_counterfactual_smoke_bundle.py`，從已進版控的 `b1_20260901_1d_74350966_5d7ecb10` **唯讀**取 `6243` 最後 250 根 K 棒、該標的的 chip 與 governance、原本的 model 與 trading calendar、replay_config（只把 `dataset_from` 改成切片的第一根），以既有的 `render_payloads()`／`build_manifest()`／`emit_bundle()` 發布到 smoke 的暫存目錄（bundle_id 由內容決定，⛔ 不受 `captured_at` 影響）。`smoke-replay-offline.sh` 新增反事實段（仍只在 `REPLAY_SMOKE=1` 時跑）：以工作樹現行內容建暫存 index 的 tree → 產生器 `--source-tree` 得 tooling → 暫存 `XDG_DATA_HOME` 建 Stage 2 identity（切片 bundle ＋ smoke image）→ Stage 1：`AFTER_REF=e1cbbbd` ＋ tooling → Stage 2：`--i074-counterfactual`、`--before-ref <e1cbbbd 完整 OID>`、真正的 counterfactual ＋ 同一份 tooling → 斷言：rc=0、恰好三檔；**cohort ≥ 1**；comparison 列數 ＝ cohort、**非空**；每一列 after 的 `rr_decoupling_candidate` 為 `true`、before 為 `false`、`lifecycle_phase` 在 `differences` 內且 before ≠ `CONTINUATION`；並在 host 以 bootstrap 載入 `stream_before_source()`／`validate_comparison_artifact()`／`validate_report()` 驗內容（⛔ 不跑 finalize——切片不是錨定的 Stage 1 證據）。⚠️ 斷言用「≥ 1」而⛔ 不寫死 6：smoke image 會重新 build，套件版本可能與封存時不同 |
| ⚠️ 可行性實測（2026-09-30，第一輪 review 之後；scratchpad、`stock-trading-python-test:latest`、mem-guard 563m） | 切片 bundle `b1_20260901_1d_de3ab843_a7c9ffb4`（250 根、chip 33、governance 16）。**after（HEAD 的產品碼 ＝ `e1cbbbd`）**：165 列、候選 **6**——`2026-06-21`、`06-22`、`06-23`、`07-16`、`07-20`、`07-28`（UTC 時戳），與 D+1 cohort 中 6243 在這段期間的 6 筆逐 key 相同；136 秒。**反事實（`git archive e1cbbbd` ＋ 套 counterfactual，⛔ 不建 worktree）**：同 165 列、候選 **0**、「CONTINUATION 且 RR 不合格」0 列；6 筆翻轉——前三筆 `CONTINUATION → CONFIRMED` 且 `market_bias` 維持 `BEARISH_BIAS`（AVOID），後三筆 `CONTINUATION → TESTING／TESTING／CONFIRMED` 且 `BULLISH_CONTINUATION → BULLISH_BIAS`（非 AVOID）——判讀矩陣的四格都有；cohort 以外 159 列逐列相同；136 秒。⚠️ 這次用的是 Stage 1 模式跑反事實程式碼，只證明「這份切片會翻轉」；反事實的 Stage 2 路徑在實作之後由 smoke 本身驗 |

**切片 fixture 的契約**（⚠️ 第二輪 review 補；`make_counterfactual_smoke_bundle.py` 的常數與規則）：

| 欄位／項目 | 規則 |
|---|---|
| 來源 | 常數 `b1_20260901_1d_74350966_5d7ecb10`（目錄名必須相符），⚠️ **一律經 `load_bundle()` 讀**（manifest SHA、逐檔 SHA、目錄內容、三方相等）；⛔ 不直接解 payload 檔 |
| `symbols` | `["6243"]`（常數） |
| candles | `loaded.candles["6243"]` 的最後 250 列（payload 已依 timestamp 排序；常數 250） |
| chip／governance | `loaded.chip`／`loaded.governance` 裡 6243 的全部列 |
| replay_config | 來源的 `replay_config`，**只**把 `dataset_from` 改成第一根 K 棒的 UTC ISO 時間（`2025-08-20T16:00:00+00:00`） |
| trading calendar、model | 逐位元沿用來源 |
| `as_of`、`timeframe`、`limit`、`replay_scope`、`report_max_rows`、`readiness`、`calendar` | 從已驗證的來源 manifest 複製 |
| `captured_at` | 沿用來源 manifest 的值（切片⛔ 沒有重新擷取資料，這是資料真正的擷取時間；也讓 manifest bytes 可重現）。⚠️ 它⛔ 不影響 bundle_id |
| provenance | ⛔ **不沿用來源的**（那是原 Stage 0 的 argv 與 image）；另建 smoke fixture 專用的 `build_provenance()`：`source_root="/app"`、**合法的**全 0 佔位值（`image_digest` ＝ `sha256:` ＋ 64 個 0、`runner_sha256` ＝ 64 個 0——必須通過 `validate_provenance(…, role="stage0")`）、`base_commit`／`tooling_patch_sha256` 為 null、`argv` 記產生器名稱與它的常數（來源 ID、標的、根數）（⚠️ 第三輪 review：⛔ 不是「比照 `make_smoke_bundle.py`」——那支的 `runner_sha256` 是 `"s" * 64`，通不過驗證） |
| ⚠️ 用途界線（第三輪 review 改寫） | ⛔ **不是證據**。佔位 provenance 只用來**標示** smoke／非正式身分——⛔ **不宣稱 loader 會拒絕它**（`load_bundle()` 不讀 manifest 的 provenance，Stage 1 也不驗；smoke 的 Stage 1 本來就會載入它）。真正的界線由這幾道維持：只存在於 smoke 的 `mktemp` 暫存目錄、smoke 結束即刪；⛔ 不進版控；⛔ 不進 finalize、⛔ 不進任何正式歸檔（`python/baselines/` 底下的任何位置） |

**切片 fixture 的守門**（⚠️ 第二輪 review 補；全部 fail-closed）：

| 守門 | 規則 |
|---|---|
| 唯讀 | 產生器的容器 `/app:ro`，只有 `/out`（smoke 的 `mktemp` 目錄）可寫；既有 `make_smoke_bundle.py` 那一步也改成 `/app:ro`（它本來就只寫 `/out`） |
| 輸出位置 | 解析後的輸出目錄⛔ 不得等於、也⛔ 不得位在來源 bundle 之內——在任何寫入**之前**驗 |
| 發布後自我驗證 | 對產出重新 `load_bundle()`：bundle_id ＝ 常數 `b1_20260901_1d_de3ab843_a7c9ffb4`；manifest 的 `symbols` ＝ `["6243"]`；candles 只有 6243、恰好 250 列；⛔ 任一不符即非零結束、stdout ⛔ 無 ID |
| smoke 在 replay 之前 | 產生器印出的 ID 與實際目錄名都 ＝ 常數，否則⛔ 不跑任何 replay |
| 來源未被修改 | smoke 前後比對來源的 `manifest.sha256` 相同、`git status --porcelain -- <來源目錄>` 為空 |
| `e1cbbbd` 既有測試 | 在 `e1cbbbd` ＋ counterfactual ＋ tooling 的 worktree 跑它**自己的** `python/scripts/test.sh`（`SKIP_SHELL_TESTS=1`、專用 `PY_IMAGE` tag），記錄 tree、image ID、結果（比照 ②「測試的執行身分」）；⚠️ `e1cbbbd` 的產品測試斷言一律不改 |

**B8　驗證、反向驗證、歸檔、stage**——見「六」「八」。

##### 三、總綱沒寫到、本細部計畫補上的決定（✅ 2026-09-30 確認）

| # | 決定 | 理由 |
|---|---|---|
| 1 | runner 的「raw ＝ canonical」**只在反事實模式**強制；一般模式只記 comp | smoke 與一般 Stage 1／2 的 tooling 是 `git diff --binary HEAD` 產的，強制會打壞它們；一般模式⛔ 沒有封存 raw patch 的證據 |
| 2 | `replay_args_tooling_patch_sha256()` 改寫在共用函式上（Stage 1 腳本一併換定義） | 總綱「⛔ 不各寫一份」；空 patch 仍是 `e3b0…`，Stage 1、`envcheck/` 已封存的證據與見證趟的 E7 不受影響；非空 tooling 的一般執行 SHA 值會變（⛔ 沒有證據依賴它） |
| 3 | 反事實模式在 replay **之前**驗 CLI 的 `--before-ref`（40 碼）＝ CLI 的 `--base-commit` ＝ after artifact 的 `provenance.base_commit` | 全圖第 8 道本來就要求；⛔ 否則跑完三小時才在 finalize 被擋 |
| 4 | runner：flag 必須搭配 `I074_STAGE=2` | before 必須用 Stage 2 identity（F2-a）；明示而⛔ 不推斷 |
| 5 | 乾淨檢查改成「無 untracked、無 unstaged」，⛔ 不再 `git add -A -N` | `write-tree` 只看 index，容器掛的是工作樹——兩者必須相同 |
| 6 | 反事實路徑的 after／cohort 用 canonical loader（支援 `.json.gz`）；一般路徑維持 `load_artifact()` | ⑩ 可以直接讀錨定的 D+1；一般路徑逐項不變 |
| 7 | 四檔不變條件由共用函式執行，所以 **`--finalize`／`--recover-durability` 也會拒絕**非四檔的 counterfactual | 總綱「二」交接列 ① 的直接結果，照實寫明 |
| 8 | 產生器分 `--source-tree`（stdout）與 `--verify`（比對 tree 內的 patch）兩個模式 | 漂移測試與 commit 之後的檢查共用同一個入口 |

##### 四、受影響檔案與資料流

| 檔案 | 改動 |
|---|---|
| `scripts/lib/replay-args.sh` | 常數、canonical diff、compose、`replay_args_tooling_patch_sha256()` 改寫、`replay_args_offline()` 第 6 參數、注入清單 |
| `scripts/run-replay-offline.sh` | flag、truth table、凍結、compose、前置守門、注入、dry-run 摘要 |
| `scripts/finalize-stage2-evidence.sh` | `compose_check()` 改用共用函式、語意 SHA 的交接、check 以語意 SHA 命中、注入清單 |
| `scripts/make-i074-tooling-patch.sh` | **新增** |
| `scripts/smoke-replay-offline.sh` | 反事實段 |
| `python/scripts/make_counterfactual_smoke_bundle.py` | **新增**（第一輪 review）：正式 bundle 的切片 fixture（契約與守門見 B7；第二輪 review 補） |
| pytest：`tests/test_counterfactual_smoke_fixture.py` | **新增**（第二輪 review） |
| `scripts/test-replay-args.sh` | 見「五」 |
| `python/backtest/modular/sr_scoring/evaluation.py` | CLI 五處、成對守門、`_run_counterfactual_stage2()`、`main()` 的 rc=6 |
| `replay_bundle/publish.py`、`replay_bundle/stage2_archive.py`、`replay_bundle/__init__.py` | 結束碼 6、例外、語意 SHA 欄位與交接、匯出 |
| `python/scripts/i074-stage2-patch-claims.py` | `--failed-record` 印 5 個 token |
| `python/scripts/fixtures/stage2_argv.json`（**新增**）、`stage2_finalizer_argv.json` | argv fixture |
| `python/baselines/i074_stage2/counterfactual_e1cbbbd.patch`（換格式）、`tooling_e1cbbbd.patch`（**新增**，進版控） | 兩份 patch |
| pytest：`tests/test_replay_counterfactual.py`（**新增**）、`test_replay_stage2_archive.py`、`test_replay_bundle_cli.py` | 見「五」 |
| `docs/issue.md`、`docs/sr-zone-scoring.md`、`docs/development-workflow.md` | 實作結果與歸檔 |

**資料流**（⑩ 的 replay，⑦a 負責的那一段）：凍結兩份 patch → `e1cbbbd` worktree ＋ `replay_args_compose`（cf → `T1` → tooling → `T2`、三個 canonical SHA ＋ 語意 SHA、四檔集合）→ docker 之前的 raw ＝ canonical → 容器內 `evaluation.py`（經 tooling patch 是 HEAD 的版本）：成對守門 → 串流 after ＋ cohort → replay → 全量 key 守門 → `CounterfactualEffectCheck` → rc=6（`bounded_diagnostics.json`）或 rc=0（before source → comparison → report）。

##### 五、測試（id → 落點）

| id | 內容 | 落點 |
|---|---|---|
| 「六、2」a～f | 全量 keys、before 少／多／**換序**、`∅` 是正常路徑、非空 → rc=6（⛔ 沒有 `candidate_mismatch.json`、⛔ 不是 rc=4）、cohort key 缺席且守門先於逐列比較、cohort 列全部出現在 comparison | pytest（真實的 `run_bundle_stage()` ＋ `stub_replay`） |
| g、i | after 的每一類缺陷（envelope、非 object、重複 key、flag 型別、replay error、診斷欄位）在反事實模式都於 replay **之前**被擋；after 只被讀一次（spy） | pytest |
| h | cohort 的 `after_artifact_sha256` ≠ 串流 SHA → replay 之前中止 | pytest |
| ⚠️ provenance（第一輪 review） | after 的 provenance 損壞（缺欄、多欄、型別錯、role 不符）→ 在 `_replay_from_bundle()` 之前中止；cohort 的 provenance 損壞 → 同上（以 spy 斷言 replay 未被呼叫）；另一支：after artifact 的 `provenance.base_commit` 合法、但 ≠ CLI 的 `--base-commit` 或 `--before-ref` → 中止（「三」#3）；另一支：`--before-ref` 不是 40 碼 → 中止 | pytest |
| aa、ab、ac、ac2 ＋ 決策 4 | 以真實路徑產生四種形狀；⚠️ **「餵進去的一定是 before rows」**：after 有候選、before 已加回 RR → 必須 rc=0（誤餵 after 會回 6） | pytest |
| 終態 | rc=0 恰好三檔且 report 最後寫；rc=6 只有中繼檔且通過 `validate_counterfactual_failure()`；檔名 ＝ `OPERATIONAL_*` | pytest |
| o | 未帶 flag：集合相等檢查、`candidate_mismatch.json`、rc=4 照舊（既有測試全跑 ＋ 一支明示） | pytest |
| p、q、r、y | Stage 1 帶 flag／SHA、flag 重複、SHA 重複、成對守門五條——`run_bundle_stage()` 與直接 `main()` 兩條路徑；runner 端另測 | pytest ＋ shell |
| s、t、u、v、w、x、ba（runner 層） | spoof、增量 SHA 與順序、空 tooling 的明確值與 0-byte 凍結、truth table、`I074_MODE`、raw ≠ canonical（cf 與 tooling 各一）在 docker 之前擋 | shell（隔離 repo） |
| 結束碼 6 | 常數 ＝ 6；`main()` 回 6；runner 原樣傳出（fake docker） | pytest ＋ shell |
| canonical／惡意 config | fixture 含 rename、非 ASCII 檔名、空白 context 行、多檔順序、binary、funcname 會命中的行；每一層（來源 repo 的 config、測試 `HOME` 的 `.gitconfig`、`info/attributes`、tree 內的 `.gitattributes`、`core.attributesFile`、`GIT_CONFIG_PARAMETERS`／`GIT_CONFIG_COUNT`）單獨與全部同時 → bytes ＝ 乾淨環境的參考值；參數給 commit 而非 tree → 拒絕 | shell |
| 語意鍵 | 只改測試 → sem 不變；多一個非白名單檔（文件、fixture、腳本）、少一檔、只改測試的 counterfactual → compose 拒絕；sem 非空 | shell |
| ③ n、o、o2、am（兩支）、al | `--check-failed-record` 以語意 SHA 命中／放行 | shell（fake docker ＋ 真 git） |
| ③ ab | `--recover-failed-record` 成功回 1 之後，同語意 SHA 的 check 仍回 2 | shell |
| failure record 的語意欄位 | 缺欄、多欄、非 hex64、空 diff 的 SHA、目錄名 ≠ 宣告值、宣告值 ≠ 交接值（publish 與 recover 各一）→ 各自拒絕 | pytest |
| 交接參數 | spoof（shell 與 Python）、重複、應帶而缺、在不該帶的模式出現 → 拒絕；⚠️ **shell 驗完之後替換 patch**（TOCTOU）→ 不發布、不 fsync | pytest ＋ shell |
| 既有 ③d 的 shell 測試 | fixture repo 改成四檔的版面；「非 canonical」的範例改成預設縮寫 `index` 的 `git diff --binary`（`--full-index` 那一版現在才是 canonical） | shell |
| argv fixture | Stage 2 反事實 argv（真正 repo、真正的兩份 patch、dry-run）與 finalizer 的兩組更新 | shell |
| 產生器 | 四條不變條件各一支違反 → 拒絕（隔離 repo，以「複製腳本 ＋ sed 常數」並斷言與正式檔案只差常數行）；`--verify` 的漂移測試；worktree 數量不變 | shell |
| sizing | `test_i074_stage2_sizing.py` 全過（⛔ 不改） | pytest |
| ⚠️ 切片 fixture（第二輪 review） | 正向：對真正的來源（唯讀使用）產生到 `tmp_path`，ID ＝ 常數、`symbols` ＝ `["6243"]`、恰好 250 列、provenance 是佔位值而⛔ 不是來源的、且通過 `validate_provenance(…, role="stage0")`、`captured_at` ＝ 來源的；負向：輸出目錄位在來源之內（用**來源的暫存複本**，⛔ 不碰真正的 bundle）→ 寫入之前就中止、複本⛔ 未被修改；來源目錄名不符 → 中止 | pytest（`test_counterfactual_smoke_fixture.py`） |
| ⚠️ smoke（非 stub；第一輪 review） | 正式 bundle 切片的真實產品資料流：after 有候選 → 反事實 before 消掉 → comparison 非空且逐列翻轉（B7） | `REPLAY_SMOKE=1` 的 `smoke-replay-offline.sh` |

⚠️ **各層各自保護什麼**（第一輪 review）：pytest 的 `stub_replay` 驗 `evaluation.py` 反事實路徑的**邏輯與順序**（守門、終態、rc），⛔ 不經過產品資料流；smoke 驗**真實產品資料流 ＋ 兩份 patch 的套用路徑**（`e1cbbbd` worktree、容器內 CLI、非空翻轉）；⑨ 的 differential guard 驗「唯一語意變因」。

⚠️ ③ 的 n、o、o2、ab、al、am 的「在 replay 之前」那一層、ax、ba 的 orchestrator 層、z 屬 ⑦b（總綱「三」）。

##### 六、驗證

| # | 項目 |
|---|---|
| 1 | `python/scripts/test.sh`（完整，含 `test-replay-args.sh` 與 doc-refs）全綠，依序執行 |
| 2 | 反向驗證（逐項注回、確認變紅、還原後重跑）：拿掉有序 key 守門、改成餵 after rows、拿掉成對守門任一條、canonical 參數拿掉一個、四檔檢查拿掉、check 改回以完整 SHA 命中、runner 拿掉 raw ＝ canonical、產生器拿掉自我驗證 |
| 3 | `REPLAY_SMOKE=1` 跑一次 smoke（含反事實段；⚠️ 反事實段預計多約 5 分鐘——兩趟 replay 各約 136 秒）：cohort ≥ 1、comparison 非空且逐列翻轉 |
| 4 | `e1cbbbd` ＋ 兩份 patch 的 worktree 跑它自己的 `test.sh`，記錄 tree、image、結果 |
| 5 | counterfactual 換格式：重算 `T1` 仍 ＝ `a1649733…`；記下新 SHA |
| 6 | 真正 repo 的 worktree 登記數、`python/baselines/`（兩份 patch 以外）前後不變 |
| 7 | stage 之後 `make-i074-tooling-patch.sh --verify "$(git write-tree)"` 回 0；commit 之後以 `HEAD^{tree}` 再驗一次並比對投影 |

##### 七、風險與回滾

| 風險 | 對策 |
|---|---|
| 改寫 `replay_args_tooling_patch_sha256()` 影響 Stage 1 腳本 | 空 patch 的值不變；既有 Stage 1 測試與 E7 的測試全跑 |
| 一般 Stage 2 被連帶改到 | 一般路徑的程式碼不動；o 與既有測試全跑；⑨ 另有 tooling 非語意 guard |
| 語意 SHA 的交接漏接 | 每個模式的必帶／拒帶各一支；TOCTOU 測試 |
| canonical 定義與 git 版本綁定 | 產生器、runner、finalizer 都在同一台 host；版本變了由 runner 在 docker 之前 fail-closed（總綱「二」的殘餘限制） |
| 漂移測試在開發途中必然失敗 | 預期行為；stage 之後才跑完整驗收 |
| **回滾** | ⑦b 開工之前只能**整包 revert ⑦a**（總綱「六」）；counterfactual 換格式的 bytes 在 git 歷史裡，revert 即還原 |

##### 八、完成後的歸檔位置

| 內容 | 歸檔到 |
|---|---|
| 三方 SHA 改成 canonical 定義與 tree-to-tree；failed record 路徑改語意 SHA、schema 加欄、重跑守門；`--verified-*` 交接補語意 SHA；反事實路徑（opt-in、rc=6、operational 三檔、終態恰好一種）與 tooling patch 的契約 | `sr-zone-scoring.md` |
| `tooling_patch_sha256` 的定義列、`git apply --index` 的說明、`--check-failed-record` 的比對鍵與模式限制、tooling patch 的版控流程與漂移測試 | `development-workflow.md` |
| ⑦a 實作結果（新的 counterfactual SHA、`e1cbbbd` 測試結果、smoke、反向驗證、與計畫的差異） | 本筆 I-074 |

#### Stage 2 步驟 ⑦a 實作結果（2026-09-30，✅ **review 通過**（兩輪修正後）並 commit `9e29658`）

✅ 依「Stage 2 步驟 ⑦a 細部計畫 v1」（三輪 review 後確認）完成 B1～B8。⛔ **沒有跑任何正式 replay、沒有 commit**；
依總綱「二」的版控流程把程式與 tooling patch **stage**，停在 review（commit 之後要做的 `--verify` 見下方「review 之後」）。

**⑦a 實作第一輪 review 的修正（2026-09-30）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | ⛔ **smoke 與「結束碼 6」測試以「執行前後全 repo worktree 清單的差集」直接 `worktree remove --force` ＋ `rm -rf`**——smoke 跑約五分鐘，期間使用者或其他程序建立的 worktree 也會出現在差集裡而被誤刪 | ✅ 新增 `scripts/lib/replay-args.sh` 的 `replay_args_remove_worktrees_under()`：**只移除登記路徑解析後確實位於指定目錄之內**的 worktree，範圍外一律不動；runner 以**本次專屬、新建**的 `TMPDIR` 執行（它的 worktree 來自 `mktemp -d`），只清那個目錄；smoke 的 EXIT trap 也呼叫它（反事實段中途失敗時）。⛔ 兩處的差集寫法都已移除。測試：fake docker 在 runner **執行途中**另建一個 `TMPDIR` 以外的 worktree（模擬同時段的其他程序）→ 清理之後本次的登記清空、那個無關 worktree 仍在；另驗「專屬 `TMPDIR` 底下確實有登記」（⛔ 不是空測） |
| 中 | ⛔ **來源 bundle 一開始就 dirty 時 smoke 仍會通過**——只比較前後 `git status` 相等，⛔ 不符計畫的「`git status` 為空」 | ✅ 任何 replay 之前就要求來源的 `git status` **為空**（否則⛔ 不跑反事實的 replay），結束時再要求一次為空（另驗 `manifest.sha256` 前後相同） |
| 低 | `test_replay_bundle_cli.py` 的 EOF 多一個空行，`git diff --check` 失敗 | ✅ 刪除；全部 staged 內容 `git diff --cached --check` 通過 |

**⑦a 實作第二輪 review 的修正（2026-09-30）**：

| # | 問題 | 修正 |
|---|---|---|
| 中 | ⛔ **`git status` 執行失敗會被當成「乾淨」**——smoke 用 `[ -z "$(git … status …)" ]`，status 失敗且沒有 stdout 時條件照樣成立（開始與結束兩處） | ✅ 新增 `scripts/lib/replay-args.sh` 的 `replay_args_path_clean()`：0＝乾淨、1＝有改動或 untracked、**2＝`git status` 本身失敗**（⛔ 不當成乾淨）；smoke 把來源檢查移到**最前面**（`docker build` 與任何 replay 之前），不通過就整支中止；結束時也用它。測試：四種結果各一支；再以假 `git`（只攔 `status`：失敗／回傳 dirty，其餘交給真的 git）跑**真正的** smoke，兩種都斷言中止且 docker **一次都沒被呼叫**——⛔ 不必動到真正的 bundle |
| 低 | ⛔ **worktree 清理函式吞掉 git 的失敗**——`worktree list` 放在 process substitution 裡，失敗時只收到空輸入、仍回 0；`worktree remove` 後接 `\|\| true`，失敗也照樣 `rm -rf`，留下 stale 登記卻回報成功 | ✅ `replay_args_remove_worktrees_under()` 先把清單取成變數並檢查結束碼（失敗 → 1）；`worktree remove` 失敗 → 回 1、⛔ 不刪實體目錄；呼叫端（smoke 主流程、「結束碼 6」測試）檢查回傳值。測試：不存在的 repo（list 失敗）→ 非零；範圍內的 worktree 被 `git worktree lock`（remove 失敗）→ 非零、實體目錄與登記都還在；解鎖之後 → 0 且已移除（對照組） |

反向驗證：把兩支函式改回 fail-open 的寫法 → 4 支新測試變紅（`status` 失敗被當成乾淨、smoke 沒擋下而呼叫了 docker、list 失敗回 0、remove 失敗回 0），還原後重跑通過。

| 檔案 | 內容 |
|---|---|
| `scripts/lib/replay-args.sh` | `I074_CF_PRODUCT_PATHS`／`I074_CF_FILES` 常數；`replay_args_canonical_diff()`／`replay_args_canonical_sha256()`（暫存 bare repo ＋ `env -i` ＋ 釘死參數，只收 40 碼 tree OID）；**唯一的合成函式** `replay_args_compose()`（前提檢查 HEAD 與 index ＝ base、固定順序、`write-tree`、工作樹 ＝ index、三個 canonical SHA ＋ 語意 SHA、四檔集合）；`replay_args_tooling_patch_sha256()` 改寫在它上面；`replay_args_offline()` 第 6 參數；注入清單加 `--counterfactual-patch-sha256` |
| `scripts/run-replay-offline.sh` | `--i074-counterfactual` 納入 `I074_MODE`；truth table（含 `I074_STAGE=2`、拒絕 `=value`）；兩份 patch 凍結（空 tooling → 0-byte 副本、symlink 拒絕）；compose；docker 之前的 raw ＝ canonical；注入；dry-run 的 `==> patches:` 摘要 |
| `scripts/finalize-stage2-evidence.sh` | `compose_check()` 改呼叫共用函式並輸出 `COMPOSE_SEM`；語意 SHA 的交接（publish 注入、recover 先比對宣告值）；check 以語意 SHA 命中、每份 record 由 shell 重算語意 SHA |
| `scripts/make-i074-tooling-patch.sh`（**新增**） | `--source-tree`／`--verify`；四條不變條件（`mk_check_invariants()`，被 source 時不執行 main）；自我驗證 |
| `scripts/smoke-replay-offline.sh` | 反事實段（正式 bundle 切片）；既有 bundle 產生改 `/app:ro` |
| `evaluation.py` | CLI 五處；`assert_counterfactual_args()`（成對守門，`load_bundle()` 之前）；`_run_counterfactual_stage2()`；`main()` 的 rc=6；一般路徑一行未改 |
| `replay_bundle/publish.py`、`stage2_archive.py`、`__init__.py` | `EXIT_COUNTERFACTUAL_INEFFECTIVE = 6`；`CounterfactualIneffective`；`validate_semantic_sha256()`；failed record 的語意欄位、目錄鍵與交接（`VerifiedComposition`、`check_verified_composition()`、CLI 的 `--verified-counterfactual-semantic-sha256`、`patch_claims()`、summary）；匯出 `OPERATIONAL_*` 等 |
| `python/scripts/i074-stage2-patch-claims.py` | `--failed-record` 印 5 個 token |
| `python/scripts/make_counterfactual_smoke_bundle.py`（**新增**） | 切片 fixture（契約與守門見 B7） |
| fixtures | `stage2_argv.json`（**新增**）、`stage2_finalizer_argv.json`（publish／recover 兩組加交接參數） |
| `python/baselines/i074_stage2/` | `counterfactual_e1cbbbd.patch` 換成 canonical 格式：**`fa7f5ba840a3106978ad1ea1661eae7c97cb77710db2a7e042bb6f1b3a6fea14`**（舊格式 `ef7a4cdf…` 為歷史值；只有 `index` 行不同，套回 `e1cbbbd` 仍得 `T1` ＝ `a1649733…`）；`tooling_e1cbbbd.patch`（**新增**）：**`353c69a0370bc2a8e6ff83a9987e99d4b0a6119213ef1ce39094b187cb16b8dc`**，224,068 bytes，只含 `evaluation.py` 與 `replay_bundle/` 的 10 個檔 |
| 測試 | 新增 `test_replay_counterfactual.py`、`test_counterfactual_smoke_fixture.py`；改 `test_replay_stage2_archive.py`、`test_replay_bundle_cli.py`、`scripts/test-replay-args.sh` |
| 文件 | `sr-zone-scoring.md`、`development-workflow.md`（見「歸檔」）；本筆 8 處因程式位移而漂移的行號引用（其中兩列加註 ⑦a 的現況） |

**⑩ 會用到的兩份 patch 的合成結果**（以 `replay_args_compose()` 在 `e1cbbbd` 實測）：`T1` ＝ `a1649733…`、`T2` ＝ `67ef270a…`；
counterfactual `fa7f5ba8…`、tooling `353c69a0…`、composed `16781839cb755b127c10ba1bb274f1ff52ca227739931f6c500d4173fa792423`、
語意 SHA `5b851e1eef4081af5ba6d4ad6d5ffcfaa3344b051d9594f6d2578853404ec9e8`。⚠️ ⑦b～⑦d 還會動 `replay_bundle/`，tooling 與 composed
到 ⑨ 封存時才定案。

**驗證**：

| # | 項目 | 結果 |
|---|---|---|
| 1 | `python/scripts/test.sh`（完整；stage 之後） | ✅ **1644 passed, 1 skipped**（③d 時 1509）；doc-refs 0 個問題；`test-replay-args.sh` 全過；245 秒 |
| 2 | 反向驗證（逐項注回、確認變紅、從 index 還原） | 見下表，全部如預期 |
| 3 | `REPLAY_SMOKE=1` 的 smoke | ✅ 全過（293 秒）：切片 `b1_20260901_1d_de3ab843_a7c9ffb4`；反事實 Stage 2 回 0；**before 165 列、cohort 6、comparison 6**、逐列翻轉；來源 bundle 未被修改 |
| 4 | `e1cbbbd` ＋ 兩份 patch 的既有測試（以 `replay_args_compose()` 建的 worktree 跑它**自己的** `test.sh`、`SKIP_SHELL_TESTS=1`、`PY_IMAGE=stock-trading-python-e1cbbbd:test`） | ✅ **1223 passed, 1 skipped**（與 ② 套 counterfactual 之後相同）；tree `67ef270af2ebcd4aabd44584f5d70da1b71a81a1`（＝ `T2`）、image `sha256:019f6be136f79aa8d26a2e7bac27ce968f5d1f4329c0365c1c257e3c164d2b26`、Python 3.11.16／pytest-9.1.1。⚠️ 第一次執行在 build 之前就停在 `e1cbbbd` 版 doc-refs 工具的自我測試「同名歧義」——那一行是 `printf … \| grep -q`（pipefail 下的 SIGPIPE 競爭，`70d7571` 已改成 here-string），輸出其實含有預期字樣；兩份 patch 都⛔ 沒碰 `scripts/`，重跑即通過（doc-refs 35／35、文件引用 0 個問題） |
| 5 | counterfactual 換格式 | ✅ 套回 `e1cbbbd` 的 `write-tree` 仍 ＝ `a1649733…`；四檔不變 |
| 6 | 真正 repo 的 worktree 登記 | 完整 `test.sh` 每跑一次 ＋4，**與 I-118 記錄的既有增量相同**——新測試 0 增加（新增的「結束碼 6」測試與 smoke 反事實段會走到 `exec docker run`，各自以專屬 `TMPDIR` 執行、只收掉那個目錄底下的登記並斷言——第一輪 review 改寫）；smoke 既有的 Stage 1／2 仍各留 1 個（I-118 既有） |
| 7 | 漂移 | ✅ stage 之後 `--verify "$(git write-tree)"` 回 0 |

**反向驗證**：

| # | 注回的回歸 | 變紅的測試 |
|---|---|---|
| R1 | 拿掉「before 與 after 同序」 | `test_key_guard_runs_before_the_effect_check[reorder]` |
| R2 | 改成把 after 的 cohort 列餵給 `CounterfactualEffectCheck` | 7 支（含「餵的一定是 before rows」與成功路徑） |
| R3 | 拿掉成對守門「有 SHA 沒 flag」 | 2 支（`run_bundle_stage()` 與直接 `main()` 各一） |
| R4 | canonical 定義拿掉 `--full-index` | 9 項 shell（非 canonical 的偵測 5 項、真正 repo 的 argv 2 項、參考值形狀、漂移） |
| R5 | 拿掉四檔檢查 | finalize、check、runner 三處「多出非白名單檔」（⚠️ 「只改測試檔」那支仍綠：語意 diff 為空的檢查另外擋下） |
| R6 | check 改回以完整 SHA 命中 | `o2／am：只改測試檔 → 仍命中` |
| R7 | runner 拿掉反事實的 raw ＝ canonical | `runner 前置守門：反事實 patch 的 raw bytes ≠ canonical` |
| R8 | 拿掉產生器的自我驗證 | `自我驗證：產出寫出之後被改動` |
| R9 | 拿掉語意 SHA 交接值與 record 欄位的比對 | recover 那支（⚠️ publish 那支仍綠：F4 的目錄名檢查另外擋下） |
| R10 | 拿掉 after 的 provenance 驗證 | 3 支（⚠️ `null-base` 仍綠：版本守門另外擋下） |

**⚠️ 與計畫的差異**：

| # | 差異 | 理由 |
|---|---|---|
| 1 | `replay_args_compose()` 另加前提檢查：HEAD ＝ base、**index 的 tree ＝ base 的 tree** | 計畫只寫「worktree 由 `replay_args_prepare_worktree()` 建」；多一道就擋得下被重用的髒 worktree |
| 2 | 產生器的不變條件抽成 `mk_check_invariants()`，腳本被 source 時只定義函式 | ②③ 在正常流程下由構造保證、造不出違反的輸入；測試以刻意做壞的 tree 直接呼叫它，才驗得到每一條（⛔ 不是執行期覆寫口——正式執行只有 `--source-tree`／`--verify`） |
| 3 | 新增「產生器自我驗證」的竄改測試 | 反向驗證 R8 時發現：拿掉自我驗證⛔ 沒有任何測試變紅（正常產出本來就正確）；以複本注入一行竄改補上 |
| 4 | 惡意 config 的參考值檢查改看**文字檔**的 index 行 | 反向驗證 R4 時發現：`--binary` 對 binary 檔本來就印完整 OID，原本的斷言被 `bin.dat` 滿足而空測 |
| 5 | 「結束碼 6」測試與 smoke 反事實段各自收掉 `exec docker run` 留下的 worktree 登記（⚠️ 第一輪 review：改成只清專屬 `TMPDIR` 底下的，見上方修正表） | 計畫要求新測試⛔ 不得增加 I-118 的洩漏；既有的「結束碼 4」測試與 smoke 既有段落⛔ 沒有順手改（屬 I-118） |
| 6 | 反事實測試的 Stage 1 fixture 要帶非空 argv | 反事實路徑以 `validate_provenance(role="stage1")` 驗 after／cohort，而 provenance 的 `argv` 必須非空——正是第一輪 review 要求的守門在生效 |
| 7 | `evaluation.py` 的 stdout 摘要另帶 `before_source_artifact_sha256` | 讓 orchestrator 不必重讀 77 MB 的檔案就能記下它 |

**review 之後**（commit 由使用者決定）：commit 後跑 `scripts/make-i074-tooling-patch.sh --verify "$(git rev-parse 'HEAD^{tree}')"`，
並比對 HEAD 與 stage 時的 source tree 在 `evaluation.py`、`replay_bundle/` 的物件 OID 相同。
✅ **2026-09-30 已執行**（commit `9e29658` 之後）：`HEAD^{tree}` ＝ `8b1928cd30f04e4b036ec494b14d7f4de04d1d07`（＝ stage 時的 tree）；`--verify` 通過（對該 tree
重新產生的 tooling patch 與已 commit 的逐位元相同，`353c69a0…`）；`evaluation.py`（`6eefa111…`）與 `replay_bundle/`（`ae99cd3d…`）的 OID 與 stage 時相同。

**歸檔**（⚠️ 依 CLAUDE.md，本筆的計畫與結果保留到 review 確認後才收斂）：契約寫進
[`sr-zone-scoring.md`](./sr-zone-scoring.md)「I-074 Stage 2 的正式證據契約」（canonical diff 的唯一定義、語意 SHA、交接）與新增的
「I-074 Stage 2 的反事實 replay 路徑與 tooling patch」；操作程序寫進 [`development-workflow.md`](./development-workflow.md)
（參數所有權表、`tooling_patch_sha256` 的定義、check 的比對鍵與重跑資格，以及新增的「I-074 Stage 2 的 tooling patch」）。

#### Stage 2 步驟 ⑦b 細部計畫 v1（2026-09-30，✅ **已確認**（2026-09-30，review 三輪後通過並 commit））

⚠️ 依「Stage 2 步驟 ⑦ 總綱 v1」（✅ 2026-09-30 確認）拆出的第二包「supervisor＋orchestrator＋freeze record」。範圍、驗收 id 與已裁定的機制以
總綱「三」「四」「五」、v29「八之一」「八之一之二」「八之二」與 ③「七之三」為準；本細部計畫只補**程式設計、呼叫順序、argv、`state/`、
結束碼表與測試落點**，以及總綱沒寫到、實作時必須決定的細節（列在「三」，✅ 2026-09-30 確認）。
⚠️ ⑦b **⛔ 不動 tooling 路徑**（`evaluation.py`、`replay_bundle/`）——⑦a commit 的 tooling patch（`353c69a0…`）與漂移測試不受影響。

**現況（2026-09-30 實查）**：

- ⑦a 已 commit（`9e29658`）；兩份 patch 已進版控：counterfactual 15,954 bytes（`fa7f5ba8…`）、tooling 224,068 bytes（`353c69a0…`）。
- sizing harness 仍以**空的** tooling 量測：patched worktree 只套 counterfactual，快照與 `freeze_patches()` 都寫 0-byte 的 `tooling.patch`，
  fixture 的 provenance 以 `sha256(counterfactual patch)` 充當 `tooling_patch_sha256`（只有 tooling 為空時才等於合成 SHA）；
  報告的 `meta` 有 run id、mode、`repo_head`、`clone_head`、三個腳本 SHA、image、identity SHA（非 formal 時可缺）與 counterfactual 的 raw SHA，⛔ 沒有 tooling。
- ⛔ **`--check-failed-record` 比對不到目前的 Stage 2 identity**：它的 Python 段 `check_failed_records()` 以 `_load_trust_anchors(python_root, None)`
  載入信任錨——`stage2_identity` 為 `None` 時直接拿 **envcheck 封存的** identity 當本次的 identity。所以 ③ 測試表的 ai（兩份 identity
  只差 `created_at` 也要在 replay 之前中止）⛔ 不能靠它，preflight 第 3 步要另外以**目前 XDG 的** identity 呼叫（「三」#1）。
- `python/scripts/_i074_bootstrap.py` 已能在 host（Python 3.9.2、沒有 pandas）載入 `canonical`、`run_identity`、`stage2_evidence`、`stage2_archive` 等 dependency-light 模組。
- `replay_bundle/publish.py` 還沒有 8、9（總綱把它們排在 ⑦c）。
- `/run/lock` 是 1777、5 MiB 的 tmpfs，目前沒有 `i074-stage2.*`；host 2 GiB RAM、git 2.30.2。

**執行步驟**：

- **步驟 A（本次，只動文件）**：本細部計畫寫進本筆；⑦a 實作結果、狀態列與 v29「八」的標記改成 ✅。⛔ 不改程式，停下等 review／commit。
- **步驟 B（本計畫 review 通過並 commit 之後）**：依「二」實作 → 「六」驗證 → 歸檔 → stage → ⛔ 不 commit，停下等 review。
  ⚠️ 不動 tooling 路徑，所以版控流程只需在 stage 之後跑漂移測試，確認 tooling patch 仍與 index 的 tree 一致。

**⑦b 細部計畫 v1 第一輪 review 的修正（2026-09-30）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | ⛔ **git／Docker 的信任根受呼叫者的 PATH 控制**：supervisor 取「PATH 上第一個可執行的 docker」、只排除已知的 shim 與 `/dev/shm`；入口的 HEAD 驗證與 clone 也用 PATH 上的 git。✅ 實查這台 host 的 PATH 是 `~/.local/bin`（兩次）、`/usr/local/bin` 排在 `/usr/bin` 之前——那裡的假 docker 能偽造 `image inspect`／`docker ps`，之後還會成為 shim 轉交的「真正 docker」 | ✅ 新增「二之三之一、信任根」：固定 `/usr/bin/git`、`/usr/bin/docker`、`/usr/bin/python3`、`/bin/bash` 與 `TRUSTED_PATH=/usr/bin:/bin`；入口角色的第一個動作就是把 PATH 設成這個常數；supervisor 在取鎖之前驗四個程式的 realpath 與每一層目錄都是 root 擁有、⛔ group／other 可寫，之後一律用絕對路徑；workload 的 PATH ＝ `<work>/bin:$TRUSTED_PATH`（⛔ 不再沿用呼叫者 PATH 的其餘部分）；持鎖階段與複本逐一驗 `command -v`。同一個理由，環境清理另加 `LD_*`、`BASH_ENV`、`ENV`、`PYTHON*`。取代總綱「四」label 列的「PATH 扣掉 shim 目錄」（總綱加註）；補「PATH 前置的 fake git／docker／python3 ⛔ 從未被執行」與「信任條件不符 → 1／8、沒有取鎖」的測試（「三」#18） |
| 中 | ⛔ **replay 之後的 worktree 清理是 fail-open**：寫「失敗只記錄」之後仍 finalize／publish，與 ⑦a 已修正的清理契約（失敗要回報）不一致 | ✅ 改成 **replay 開始之後 orchestrator ⛔ 不做任何 worktree 清理**，並定義殘留與收斂：殘留只可能是**複本的** `.git` 裡、登記路徑位於 `<work>/tmp` 底下的 worktree（runner 的 `exec docker run` 留下的那一個，屬於 `P_B` 已計入的 `replay_worktree`；finalizer 自己建的由它自己的契約處理）；⛔ 不影響真正 repo、⛔ 不影響複本完整性（`git status` 只看主工作樹）；收斂：⑦c 的 `--promote` 步驟 2 `git worktree prune`，以及晉升結果 commit 之後整個 `<work>` 刪除。replay **之前**唯一的清理（preflight 第 2 步的暫時 worktree）改成**失敗即 1**（replay 之前、⛔ 不計入正式 scan）。補故障注入測試（fake git 讓 `worktree remove` 失敗）與「殘留只在 `<work>/tmp`、真正 repo 的登記數不變」的斷言（「三」#19） |
| 中 | ⛔ **`state/` 宣稱封閉卻只列概略欄位**：沒有 schema 名稱、精確鍵集合、型別與跨檔不變條件；`--resume` 沒寫要把目前的 `REPLAY_IMAGE_ID`、XDG identity 的 SHA 與 `preflight.json` 重新比對 | ✅ 「二之八」改寫成五份完整的封閉 schema（schema 名稱、鍵集合、型別／格式）＋ 以 SHA 串起來的跨檔鏈（`preflight` 綁 `run`、`replay_started` 綁 `preflight`、`replay_done` 綁 `replay_started`、`attempt` 綁 `replay_done`）＋ 與 freeze record 的交叉條件；`--resume` 與 replay 之後的檢查點都重新比對**目前的** `REPLAY_IMAGE_ID` 與 XDG identity 的 SHA；補「resume 時 image 改變」「identity 只改 `created_at`」都在 finalizer 之前中止的測試，以及每份 state 的缺欄、多欄、型別、非 canonical、鏈不符各一支 |
| 低 | ⛔ **入口 cleanliness 的文字互相矛盾**：資料流寫「真正 repo 的工作樹必須 ＝ HEAD」，詳細設計只驗入口清單的檔案，測試只寫「入口 dirty」 | ✅ 統一成「**入口清單的投影 ＝ HEAD**」（v29「八之一」的原契約：驗入口與持鎖階段會執行的檔案，⛔ 不凍結整份真正 repo）；資料流、「二之六」、E3 同步；測試改成**逐一**竄改清單內的每一個檔案（改內容、以 `git rm --cached` 變成未追蹤，各一支）→ 1／8 且沒有取鎖；另加對照組：清單以外的檔案 dirty → 照常通過 |

**⑦b 細部計畫 v1 第二輪 review 的修正（2026-09-30）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | ⛔ **入口還沒有可信的 bootstrap**：第一輪寫「入口啟動之後才固定 PATH」，但 shell 直譯器、`BASH_ENV`、`LD_PRELOAD` 在第一行執行之前就可能生效；supervisor 才清 `LD_*` 也太晚（Python 已經啟動） | ✅ 先重現（2026-09-30）：`#!/bin/bash -p` 直接執行時 `BASH_ENV` ⛔ 沒有被執行、`$-` 含 `p`；改用 `bash <script>` 啟動時 `BASH_ENV` 被執行、`$-` 不含 `p`。改成：正式命令一律**直接執行**腳本，shebang 固定為 `#!/bin/bash -p`（⛔ `#!/usr/bin/env bash`）；入口角色在任何外部指令之前、只用 bash 內建：驗 `$-` 含 `p`（不含 → 1／8，代表不是以正式命令啟動）→ `unset` `BASH_ENV`、`ENV` 與全部 `LD_*`、`PYTHON*` → `PATH=$TRUSTED_PATH` → 才 `exec /usr/bin/python3 -I`；supervisor 以 `/bin/bash -p` 啟動持鎖階段、持鎖階段以 `/bin/bash -p` exec 複本，兩者也驗 `$-`。⚠️ **照實界定保證**：無特權的 shell 流程⛔ 防不了惡意呼叫者以 `LD_PRELOAD`（或換掉直譯器）污染**第一個程序**；環境清理保證的是「⛔ 不把污染傳入 supervisor 與之後的 workload」，⛔ 不宣稱完整的信任根（「二之三之一」的已知限制改寫）。測試補 fake PATH ＋ `BASH_ENV`（「三」#20） |
| 中 | ⛔ **`git worktree prune` 收不掉第一輪描述的殘留**：runner 留下的是**實體目錄仍存在**的 worktree，而 prune 只移除實體目錄已消失的登記 | ✅ 先重現（git 2.30.2）：兩個 worktree 各刪掉一個的實體目錄後 `prune`，只有目錄已消失的那一筆被移除。改成照實寫：殘留（實體目錄與登記）**一直保留到整個 `<work>` 被刪除**（晉升結果 commit 之後），刪掉「⑦c 的 prune 能收斂它」的宣稱（「二之七」、「三」#19）。⚠️ 附帶發現：「八之三」的「崩潰留下的 worktree 登記由下一次 `--promote` 的 `git worktree prune`（複本）清掉」有同一個前提問題（被 SIGKILL 時實體目錄通常還在），留給 ⑦c 細部計畫處理——⑦b ⛔ 不改「八之三」 |
| 低 | ⛔ **`REPLAY_IMAGE_ID` 只要求存在，沒有在第一次 Docker 呼叫之前驗格式**：option-like 或非 immutable 的值會進入 docker argv | ✅ 入口在 bash 內建的守門之後、任何 docker 呼叫之前驗 `^sha256:[0-9a-f]{64}$`（不符 → 1／8），supervisor 再驗一次；補空值、`--help`、`-f`、`<name>:<tag>`、大寫 hex、63 位、尾端空白或換行的案例——都回 1／8 且 docker ⛔ 一次都沒被呼叫（「三」#21） |
| 執行注意 | `git status` 是 `MM docs/issue.md`：上一版計畫已 staged、這一輪的修正還沒 | ✅ 照實記下；commit 之前需要重新 `git add docs/issue.md`（本輪⛔ 不代為 stage） |
| 執行注意 | review 環境實測 `/usr/bin/git` 等檔案與上層目錄的 owner UID 是 65534，不是 0 | ✅ 在正式執行帳號重驗（2026-09-30，本 session：`dev` uid 1001、`/proc/self/uid_map` ＝ `0 0 4294967295` 即初始 user namespace）：四個程式、`/usr`、`/usr/bin`、`/bin`、`/` 的 owner uid 都是 **0**——65534 是 review 環境的 user namespace 映射（overflow uid）。為了讓這個前提由程式守住而⛔ 不靠文件，supervisor 的信任根檢查**先驗 `uid_map` 是初始 user namespace**，不是就 1／8（在映射過的 namespace 裡 owner 不可信）；「六」加一步：實作前在正式帳號的登入 shell 再驗一次，不符就停下改計畫 |

**⑦b 細部計畫 v1 第三輪 review 的修正（2026-09-30）**：

| # | 問題 | 修正 |
|---|---|---|
| 低 | ⛔ 「二之三」argv 表的 supervisor 列仍寫 `bash <script> --internal-locked-stage …`，與第二輪定案的 `/bin/bash -p` 與 `$-` 守門衝突；結束碼表的 S6 仍寫「真正的 docker 解析不到或指向 shim」，與 S0（docker 已固定並驗信任條件）的責任重疊 | ✅ argv 表改成 `/bin/bash -p <script> …`（入口那一步也寫明 `exec /usr/bin/python3 -I`）；S6 只保留 `image inspect` 失敗與 labels 含鍵。全節另以 `bash <` 搜尋：其餘命中都是在描述「非正式的 `bash <script>` 啟動會被拒絕」 |

##### 一、目標與⛔ 不做

| 項目 | 內容 |
|---|---|
| 目標 | 總綱「三」⑦b 列的全部項目與驗收 id：`scripts/run-i074-stage2.sh`（入口 → supervisor → 持鎖階段 → 複本內 orchestrator）、supervisor（「八之一之二」第 1～12 列）、label shim、preflight 0～7（含磁碟檢查與常數）、依**磁碟事實**分流、`--resume`、freeze record 的寫入端與驗證端、端到端結束碼到「複本內終態」為止、晉升 stub（固定回 9） |
| ⛔ 不做 | 真正的晉升、`--verify-promotion-staging`、判讀器（⑦c）；memory harness（⑦d）；`evaluation.py`、`replay_bundle/` 的任何改動；⑧～⑪；**任何會用到真正 `/run/lock` 的執行**——⑦b 的驗證全部在隔離環境，第一次正式使用是 ⑩ |
| ⚠️ 過渡狀態 | ⑦b commit 之後，完整流程到「複本內終態」之後一律回 **9**（stub），`--promote` 也一律 **9**——⛔ 不會把任何證據搬進真正 repo。sizing harness 改用真實 tooling 之後，⑤ 的 `P_B`（空 tooling）成為歷史值，由 ⑨-2 重量 |

##### 二、設計

###### 二之一、檔案與角色

| 檔案 | 執行位置 | 角色 |
|---|---|---|
| `scripts/run-i074-stage2.sh`（新增） | host `/bin/bash -p`（絕對 shebang；⚠️ 第二輪 review） | 同一個檔案三種角色，依第一個參數分派：**入口**（公開 argv）、**持鎖階段**（`--internal-locked-stage`）、**複本內 orchestrator**（`--internal-in-clone`）；`/proc/locks` 的驗證寫在這裡（shell ＋ 內嵌、只用標準庫的 `python3`，⛔ 不 import repo 模組） |
| `scripts/lib/i074-stage2-supervisor.py`（新增） | host python3（**3.9 相容、只用標準庫**，`prctl` 走 `ctypes`） | 「八之一之二」第 1～12 列 |
| `scripts/lib/i074-stage2-docker-label-shim.sh`（新增） | host bash | 唯一的 label 注入點 |
| `python/scripts/i074_stage2_freeze_record.py`（新增） | host python3（3.9；標準庫 ＋ `_i074_bootstrap` 載入的 `canonical`；需要時呼叫 `git`） | freeze record 的 `build`（sizing 用）、`check-basic`（持鎖階段）、`check-full`（複本內 preflight 第 2 步） |
| `python/scripts/i074_stage2_preflight.py`（新增） | host 子指令（3.9、標準庫）＋ 一個容器子指令 | 常數 `P_B_BUDGET`、`M_SAFETY`、`REQUIRED_BYTES`；`disk`（host）；`anchors`（Stage 2 image 內，lazy import）；`state-write`／`state-check`（host，`state/` 的封閉 schema 與 durable 寫入） |
| `scripts/i074-stage2-sizing.sh`、`python/scripts/i074_stage2_sizing.py`、`scripts/lib/i074-sizing-docker-shim.sh`（修改） | — | 真實 tooling、freeze record 寫入、與 label shim 互斥（「二之五」「二之九」） |
| `scripts/test-i074-stage2.sh`、`scripts/tests/test_i074_stage2_host.py`（新增） | host | 見「五」；由 `python/scripts/test.sh` 的 `SKIP_SHELL_TESTS` 區段呼叫 |

###### 二之二、資料流

```text id="i074_stage2_7b_flow_001"
入口（真正 repo；⚠️ 只驗入口清單的投影 ＝ HEAD，⛔ 不凍結整份工作樹）
  直接執行（#!/bin/bash -p）→ 只用 builtin：$- 含 p、unset BASH_ENV／ENV／LD_*／PYTHON*、PATH＝TRUSTED_PATH
  → REPLAY_IMAGE_ID 格式 → 公開 argv → 入口清單的投影 ＝ HEAD → work 目錄規則 → exec /usr/bin/python3 -I supervisor
supervisor（同上）
  四個系統程式的信任條件 → 驗自身 → 拒絕帶 I074_STAGE2_* 的環境 → uid → 取鎖 → 沒有舊 sentinel
  → image 的 Config.Labels 沒有鍵 → 殘留容器（docker ps 成功且為空）→ 建 sentinel → subreaper
  → 以 /bin/bash -p 啟動持鎖階段（清理過的環境 ＋ 協定 v1 ＋ PATH=<work>/bin:TRUSTED_PATH，parent-death TERM）→ 等待 → 正常釋放
持鎖階段（同上）
  重驗檔案 → run：mkdir <work> → 複製 freeze record 與報告到 <work>/freeze/ → 驗副本的基本欄位
               → repo_head 存在於真正 repo → clone --no-hardlinks → detached checkout repo_head
           resume／promote：<work> 的 layout 與 state/run.json
  → exec /bin/bash -p <work>/repo/scripts/run-i074-stage2.sh --internal-in-clone <mode> --work-dir <work>
複本內 orchestrator（repo_head 版）
  /proc/locks → preflight 0（完整性）→ state/run.json → 安裝 label shim
  → 1 凍結 patch → 2 freeze record 完整驗證 → 3 信任錨（目前的 identity）
  → 4 真正 repo 的兩條檢查 ＋ --check-failed-record（5：命中 → 2）→ 6 磁碟 → state/preflight.json
  → 7 replay（背景 ＋ wait）→ 輸出形狀 → state/replay_done.json
  → 檢查點 → finalize（rc 0）／publish-failed-record（rc 6）→ 依磁碟事實判終態
  → 有終態：檢查點 → 晉升 stub（9）；沒有終態且回 1：state/attempt.json → 1（可以 --resume）
```

###### 二之三、argv

| 角色 | argv | 規則 |
|---|---|---|
| 入口 | `run-i074-stage2.sh --freeze-record <path> --work-dir <dir>`；`--resume --work-dir <dir>`；`--promote --work-dir <dir>` | 旗標重複、未知參數、`--x=value` 寫法、缺值一律拒絕；⛔ 沒有 `--repo-head`、⛔ 沒有任何路徑覆寫（ad）；三種模式都必須有 `REPLAY_IMAGE_ID`（「三」#15），而且在任何 docker 呼叫之前驗 `^sha256:[0-9a-f]{64}$`（⚠️ 第二輪 review，「三」#21）。run：`<dir>` ⛔ 不得存在（含 symlink）、canonical 之後在真正 repo 之外、⛔ 不是 repo 的上層、上層目錄存在；resume／promote：`<dir>` 必須存在 |
| supervisor | `i074-stage2-supervisor.py {run\|resume\|promote} --work-dir <canonical dir> [--freeze-record <path>]` | 由入口 `exec /usr/bin/python3 -I`；持鎖階段的 argv 由 supervisor **自己組**（`/bin/bash -p <supervisor 所在 repo>/scripts/run-i074-stage2.sh --internal-locked-stage <mode> …`；⚠️ 第三輪 review：固定 `/bin/bash -p`，持鎖階段以 `$-` 含 `p` 守門），⛔ 不接受任意指令 |
| 持鎖階段 | `--internal-locked-stage {run\|resume\|promote} --work-dir <dir> [--freeze-record <path>]` | 協定變數缺漏或格式不符、`/proc/locks` 驗證不過 → 立即中止 |
| 複本內 | `--internal-in-clone {run\|resume\|promote} --work-dir <dir>` | 同上；另驗自身的 canonical path ＝ `<work>/repo/scripts/run-i074-stage2.sh` |

協定 v1（總綱「四」）：`I074_STAGE2_TOKEN`、`I074_STAGE2_SUPERVISOR_PID`、`I074_STAGE2_SUPERVISOR_START`、`I074_STAGE2_REAL_REPO`、
`I074_STAGE2_REAL_DOCKER`、`I074_STAGE2_MODE`（`orchestrator`／`promote`；argv 的 `run`／`resume` 對應 `orchestrator`）。
⚠️ 名稱或語意之後要改就是協定 v2，另寫計畫（單獨執行 `--promote` 時可能是新版 supervisor 帶舊版複本）。

###### 二之三之一、信任根：固定的系統程式與 PATH（⚠️ 第一輪 review 新增）

| 項目 | 規則 |
|---|---|
| 常數 | `TRUSTED_PATH=/usr/bin:/bin`；`GIT=/usr/bin/git`、`DOCKER=/usr/bin/docker`、`PYTHON=/usr/bin/python3`、`BASH=/bin/bash`；信任的 owner uid ＝ 0。`run-i074-stage2.sh`、supervisor、label shim 各自鏡像，測試斷言三處相等。2026-09-30 實查：四者的 realpath 都是 root 擁有、`0755` 的 regular file（`python3` 是指向 `python3.9` 的 symlink），`/usr`、`/usr/bin`、`/bin` 都是 root、`0755`；使用者的 PATH 是 `~/.local/bin:~/.local/bin:/usr/local/bin:/usr/bin:/bin:/usr/games`——前三項都排在 `/usr/bin` 之前。⚠️ 第二輪 review 補：以上是在初始 user namespace（`/proc/self/uid_map` ＝ `0 0 4294967295`）以 `stat -c %u` 取得的數字 uid（0）；映射過的 namespace 會看到 65534（review 環境就是） |
| 入口（⚠️ 第二輪 review 改寫） | **正式命令一律直接執行腳本**，shebang 固定為 `#!/bin/bash -p`（⛔ `#!/usr/bin/env bash`：那會先依呼叫者的 PATH 找 bash）——`-p` 讓 bash ⛔ 不處理 `BASH_ENV`／`ENV`、⛔ 不匯入環境裡的函式、⛔ 採用 `SHELLOPTS`／`BASHOPTS`。依第一個參數分派（bash 內建的 `case`）之後，入口角色在**任何外部指令之前、只用 bash 內建**依序：驗 `$-` 含 `p`（不含 → 1／8：代表是以 `bash <script>` 之類的方式啟動，`BASH_ENV` 可能已經執行）→ `unset BASH_ENV ENV`，並以 `compgen -e` 列舉、`unset` 全部 `LD_*`、`PYTHON*` → `PATH=$TRUSTED_PATH` → 驗 `REPLAY_IMAGE_ID` 的格式；之後所有外部指令（git、sha256sum、realpath…）都從固定的 PATH 解析；以 `exec /usr/bin/python3 -I <supervisor>` 啟動 supervisor（`-I`：再忽略一次 `PYTHON*` 與 user site-packages） |
| supervisor | **取鎖之前**先驗 `/proc/self/uid_map` 恰好是初始 user namespace 的 `0 0 4294967295`（⚠️ 第二輪 review：映射過的 namespace 裡 owner 不可信）→ 再驗四個常數：realpath 是 regular file、owner ＝ 信任 uid、⛔ group／other 可寫、可執行；realpath 的每一層上層目錄 owner ＝ 信任 uid、⛔ group／other 可寫——任一不符 → 1／8。之後 supervisor 自己的 git、docker 一律用這些絕對路徑；`I074_STAGE2_REAL_DOCKER` ＝ 驗過的 `/usr/bin/docker` 的 realpath；持鎖階段以 `/bin/bash -p` 啟動（持鎖階段再以 `/bin/bash -p` exec 複本內的 orchestrator；兩者都驗 `$-` 含 `p`），環境的 `PATH=<work>/bin:$TRUSTED_PATH`（⛔ 不再沿用呼叫者 PATH 的其餘部分）。取代「PATH 扣掉 shim 目錄之後第一個可執行的 docker」 |
| 環境清理（追加） | 同一個理由另清 `LD_*`、`BASH_ENV`、`ENV`、`PYTHON*`（之後再設 `PYTHONDONTWRITEBYTECODE=1`）——它們同樣能讓固定路徑的程式載入呼叫者指定的程式碼。⚠️ 第二輪 review：**入口在 exec Python 之前就以 bash 內建清掉**（見「入口」列），supervisor 的清理是第二道；下游腳本的 `#!/usr/bin/env bash` 因 PATH 已固定而解析到 `/bin/bash`（本機沒有 `/usr/bin/bash`，`<work>/bin` 只有 `docker`） |
| 持鎖階段與複本 | 開頭驗 `PATH` 恰好 ＝ `<work>/bin:$TRUSTED_PATH`、`command -v git` ＝ `/usr/bin/git`、`command -v python3` ＝ `/usr/bin/python3`；複本另驗 `<work>/bin` 底下**恰好只有** `docker`（＝ 複本的 shim），第一次呼叫 docker 之前驗 `command -v docker` ＝ `<work>/bin/docker` |
| label shim | `I074_STAGE2_REAL_DOCKER` 除了「二之五」的規則，另驗同一組信任條件 |
| 已知限制（⚠️ 第二輪 review 改寫：照實界定） | ⛔ **這不是完整的信任根**。無特權的 shell 流程⛔ 防不了惡意的呼叫者以 `LD_PRELOAD` 污染**第一個程序**（`/bin/bash` 本身在第一行之前就已載入），也防不了呼叫者不用正式命令、改用別的直譯器執行腳本內容（只有 `bash <script>` 這種能被 `$-` 偵測到）。實際保證的是：①正式命令的直譯器是固定的 `/bin/bash -p`；②呼叫者環境裡的 `PATH`、`BASH_ENV`、`ENV`、`LD_*`、`PYTHON*` ⛔ 不會傳進 supervisor 與之後的 workload；③之後執行的 git、docker、python3、bash 都是固定路徑、root 擁有且別人不可寫的那一份。防的是 PATH 被遮蔽之類的**誤用**（本機實際的 PATH 就是這樣），⛔ 不是惡意的同帳號使用者，更防不了 root。host 升級套件或把 docker 移到別處 → fail-closed（1／8），要改常數就是改計畫 |

###### 二之四、supervisor（「八之一之二」逐列）

| 列 | 實作 |
|---|---|
| 信任根 | 「二之三之一」的四個系統程式，⚠️ 排在一切之前（第一輪 review）；不符 → 1／8 |
| 自身 | `/usr/bin/git -C <所在 repo> show HEAD:scripts/lib/i074-stage2-supervisor.py` 的 SHA-256 ＝ 檔案內容的 SHA-256（`GIT_*` 已清掉）；不符或未追蹤 → 1／8 |
| 環境 | 傳入的環境帶任何 `I074_STAGE2_*` → 1／8 |
| 1 帳號 | 常數 `RUN_UID = 1001`；`os.getuid()` 不符 → 1／8 |
| 2 鎖 | 常數 `LOCK_PATH = "/run/lock/i074-stage2.lock"`；`os.open(O_RDWR\|O_CREAT\|O_NOFOLLOW\|O_CLOEXEC, 0o600)` → `fstat`：regular、owner ＝ `RUN_UID`、mode 恰好 `0600`、`st_nlink == 1` → `lstat` 與 `fstat` 同一個 `st_dev`＋`st_ino` → `fcntl.flock(LOCK_EX\|LOCK_NB)`；屬性不符 → 1／8、拿不到鎖 → 1／8；⛔ 永不 unlink；fd 是 `O_CLOEXEC`，子程序拿不到 |
| 3 sentinel | 常數 `SENTINEL_PATH = "/run/lock/i074-stage2.active"`。`lstat` 顯示存在（任何型別）→ 印出內容（有界讀取；讀不懂照樣拒絕）與「**需要重開機**」→ 1／9；`lstat` 的其他錯誤 → 1／8。建立：`O_WRONLY\|O_CREAT\|O_EXCL\|O_NOFOLLOW\|O_CLOEXEC`、`0600` → `fstat` 驗屬性 → 寫 canonical JSON → fsync 檔案與 `/run/lock`；**fd 保留到釋放**。**收回**（workload child 成功啟動之前的可控失敗：寫入、fsync、屬性、subreaper、`Popen`）：①曾產生 child 就先 reap（`subprocess.Popen` 在 exec 失敗時會自己等 child 再拋例外），並確認沒有活著的後代、`docker ps -a --filter label=<鍵>=<token>` 成功且為空；②保存的 fd 的 `fstat` 與路徑的 `lstat` 是同一個 inode；③才 unlink、fsync 目錄、放鎖 → 1／8。任一步不成立 → sentinel 留下 → 1／9 |
| 4 schema | `{schema: "i074_stage2_active_run/v1", token, boot_id, uid, supervisor_pid, supervisor_start_time, mode}`；`validate_sentinel()` 在寫入之前驗自己的輸出：嚴格 int（⛔ `bool`）、token hex64、`boot_id` 小寫 UUID 且 ＝ 目前的 `/proc/sys/kernel/random/boot_id`、uid ＝ `RUN_UID`、pid > 0、starttime（`/proc/self/stat` 第 22 欄）> 0、mode ∈ {`orchestrator`, `promote`}；多欄／缺欄拒絕 |
| 5 範圍 | `prctl(PR_SET_CHILD_SUBREAPER, 1)`；後代 ＝ 以 `/proc/*/stat` 的 ppid 建樹、從 supervisor 的 pid 往下；等待迴圈同時 `waitpid(-1, WNOHANG)` 回收回到自己底下的孤兒 |
| 6 label | 鍵常數 `LABEL_KEY = "i074.stage2.run"`；token ＝ `secrets.token_hex(32)`，只經 sentinel 與協定變數流出 |
| 7 啟動檢查 | ①真正的 docker ＝ 常數 `/usr/bin/docker`（信任條件已在取鎖之前驗過；⚠️ 第一輪 review：⛔ 不從呼叫者的 PATH 解析）；②`REPLAY_IMAGE_ID` 再驗一次 `^sha256:[0-9a-f]{64}$`（⚠️ 第二輪 review）→ `docker image inspect --format '{{json .Config.Labels}}' $REPLAY_IMAGE_ID`：失敗、或 labels 含 `LABEL_KEY` → 1／8；③`docker ps -a -q --filter label=<鍵>`：失敗或非空 → 1／8。⚠️ 三步都在建 sentinel **之前**，失敗不留 sentinel、排除原因後可直接重跑 |
| 8 正常釋放 | workload child 結束、或自己收到 TERM／INT／HUP：`docker ps -a -q --filter label=<鍵>=<token>` → `docker rm -f` → 對後代送 TERM → 最多等 20 秒（持續回收）→ 仍有就 KILL → 最多 5 秒 → **確認**沒有活著的後代（zombie 不算，且已回收）＋ 容器查詢成功且為空 → sentinel 的 inode 比對 → unlink、fsync 目錄 → 關閉 lock fd（⚠️ ⑦b 實作第一輪 review：unlink 之後到目錄 fsync 成功之前**⛔ 放鎖**——fsync 失敗就持鎖、每 30 秒重試並回報；清理途中的任何例外也等同「清不空」，⛔ 讓 supervisor 結束、連帶放鎖；收回同樣）。結束碼：child 的結束碼（child 被訊號 N 結束 → 128＋N）；supervisor 自己收到訊號 N → 128＋N。清不空或 inode 不符 → ⛔ 不刪、⛔ 不放鎖，每 30 秒重試並回報（「三」#10） |
| 9 SIGKILL | ⛔ 沒有任何解除入口：沒有能刪 sentinel 的 CLI、沒有覆寫路徑的環境變數或參數 |
| 10 持鎖驗證 | 寫在 `run-i074-stage2.sh`：`/proc/locks` 有一列 `FLOCK`、`MAJ:MIN:INO` ＝ 鎖檔的（`os.major`／`os.minor` 取自 `stat`）、pid ＝ `I074_STAGE2_SUPERVISOR_PID`；該 pid 的 starttime ＝ `I074_STAGE2_SUPERVISOR_START`；它在自己的 ppid 鏈上；sentinel 存在，且 token、pid、starttime、`boot_id`、mode 與協定變數相同。⛔ 不用「再 flock 一次」 |
| 11 中斷 | child 以 `preexec_fn` 設 `PR_SET_PDEATHSIG=SIGTERM`（設完再確認 `getppid()` 仍是 supervisor）並恢復預設的訊號處置；orchestrator 的長步驟一律背景 ＋ `wait`（「二之七」） |
| 12 撤回的做法 | 一律⛔ 不實作 |

**環境清理**（總綱「四」）在 supervisor 做一次：刪 `GIT_*`、`DOCKER_*`、`REPLAY_DRY_RUN`、`MEASURE_PEAK`、`TOOLING_PATCH`、`COUNTERFACTUAL_PATCH`、
`PY_IMAGE`、`AFTER_REF`、`REPLAY_ARGS_SELFTEST`、`MEM*`、`CPUS`、`SIZING_*`、`I074_SIZING_FAULT`，以及（第一輪 review 追加）`LD_*`、`BASH_ENV`、`ENV`、`PYTHON*`；
設 `PYTHONDONTWRITEBYTECODE=1`、協定 v1、`PATH=<work>/bin:$TRUSTED_PATH`；`REPLAY_IMAGE_ID`、`XDG_DATA_HOME`、`HOME` 照傳。兩份 patch 的環境變數由 orchestrator **只**加在 runner 那一次呼叫上。

###### 二之五、label shim 與 sizing shim 的互斥

| 項目 | 規則 |
|---|---|
| 安裝 | 複本內 orchestrator 在 preflight 0 通過之後，把**複本的** `scripts/lib/i074-stage2-docker-label-shim.sh`（shebang 同樣固定為 `#!/bin/bash -p`）複製成 `<work>/bin/docker`（`0555`，「三」#5）；之後每個檢查點都驗它的內容 ＝ `repo_head` 中的版本、`<work>/bin` 底下恰好只有它；第一次呼叫 docker 之前驗 `command -v docker` ＝ `<work>/bin/docker` |
| 注入 | 第一個參數是 `run`／`create`，或前兩個是 `container run`／`container create` → 在子指令之後插入 `--label i074.stage2.run=<token>`（恰好一個）再 `exec "$I074_STAGE2_REAL_DOCKER"`；其餘子指令原樣 `exec`（I/O 透明，shim ⛔ 不寫 stdout） |
| 拒絕（結束碼 125、⛔ 不執行 docker） | 第一個參數以 `-` 開頭（子指令之前的全域選項，**任何子指令**都拒絕，「三」#6）；run／create 的任一 token 含 `i074.stage2.run` 或以 `--label-file` 開頭；`I074_STAGE2_TOKEN` 不是 hex64，或 ≠ sentinel（常數路徑，以 bash 正規式從 canonical JSON 取 `token`）；`I074_STAGE2_REAL_DOCKER` 不是絕對路徑、不可執行、realpath 是 shim 自己或 sizing shim、不符合「二之三之一」的信任條件；環境帶任何 `SIZING_*` |
| sizing 那一邊 | sizing shim：環境帶任何 `I074_STAGE2_*`、或 `SIZING_REAL_DOCKER` 的 realpath 是 label shim → 125（寫 `shim-errors.log`、⛔ 不執行）；sizing harness：啟動時環境帶 `I074_STAGE2_*`、或 PATH 上的 docker 是 label shim → 中止 |
| 靜態規則（測試） | ⑩ 呼叫圖（`run-i074-stage2.sh`、`run-replay-offline.sh`、`finalize-stage2-evidence.sh`、`scripts/lib/replay-args.sh`、`scripts/lib/mem-guard.sh`）：⛔ 以絕對路徑或變數呼叫 docker、⛔ `command -p`、⛔ 指派或 export `PATH`——唯一例外是 `run-i074-stage2.sh` 入口角色的第一個動作 `PATH=$TRUSTED_PATH`（測試斷言恰好一處、值是常數）；supervisor 與 shim 本身不在清單內。⚠️ 第二輪 review 追加：`run-i074-stage2.sh` 與 shim 的第一行恰好是 `#!/bin/bash -p` |

###### 二之六、持鎖階段

- **入口清單**（入口與持鎖階段會執行的每一個檔案，總綱「四」的檔名列）：`scripts/run-i074-stage2.sh`、`scripts/lib/i074-stage2-supervisor.py`、
  `python/scripts/i074_stage2_freeze_record.py`、`python/scripts/i074_stage2_preflight.py`（`P_B_BUDGET` 的唯一定義，freeze record 模組 import 它）、
  `python/scripts/_i074_bootstrap.py` 與它載入的 `replay_bundle/canonical.py`——每一個都必須已追蹤且內容 ＝ HEAD 中的版本。
  ⚠️ 這就是入口的 cleanliness 的**全部**（第一輪 review 統一用詞）：「入口清單的投影 ＝ HEAD」，⛔ 不凍結整份真正 repo——清單以外的檔案 dirty 不影響（v29「八之一」：真正 repo 照常開發）。
- 開頭：協定變數齊全且格式正確 → PATH 與 `command -v`（「二之三之一」）→ `/proc/locks` 驗證 → `I074_STAGE2_REAL_REPO` ＝ 自己所在 repo 的 canonical path → 重驗入口清單的檔案 ＝ HEAD。
- **run**：`mkdir <work>`（已存在即失敗——入口檢查之後的競爭）→ 建 `freeze/`、`logs/`、`state/`、`tmp/`、`bin/`、`run/` →
  把 `--freeze-record` 與**同目錄的** `sizing_report.json` 各讀一次、寫進 `<work>/freeze/`（「三」#4：先複製、再驗副本）→
  `i074_stage2_freeze_record.py check-basic --record <副本>`（stdout 只印 `repo_head`）→ `git -C <真正 repo> cat-file -e <repo_head>^{commit}` →
  `git clone -q --no-hardlinks --no-checkout --template= <真正 repo> <work>/repo` → `git -C <work>/repo -c core.hooksPath=/dev/null checkout -q --detach <repo_head>` →
  HEAD ＝ `repo_head`、沒有 alternates、`origin` ＝ 真正 repo → exec 複本內 orchestrator。
- **resume／promote**：`<work>/repo`、`<work>/freeze/`、`<work>/state/run.json` 存在，且 `run.json` 記的 `work_dir` ＝ canonical `<work>`；freeze record 副本以 HEAD 版的 `check-basic` 驗過，它的 `repo_head` 與副本的 SHA ＝ `run.json` 的記錄值，`repo_head` 存在於真正 repo → exec。
- ⚠️ **exec 之前**（⑦b 實作第一輪 review（中），三種模式都做）：複本的 HEAD ＝ 凍結的 `repo_head`，且要執行的 `scripts/run-i074-stage2.sh` 內容 ＝ **真正 repo 的物件**裡 `repo_head` 的 blob——⛔ 讓複本的 orchestrator「先執行、再自己驗自己」（沿用持久化複本的 `--resume`／`--promote` 尤其需要；⑦c 的晉升會操作真正 repo）。不符 → run／resume 1、promote 9。
- ⛔ 不做任何 preflight、replay 或發布。

###### 二之七、複本內 orchestrator

**preflight**（③「七之三」0～7；⚠️ 任一步失敗都⛔ 不執行後面各步，測試以 spy 斷言）。orchestrator 一開始 `export TMPDIR=<work>/tmp`——
所有暫存目錄與 worktree（含 runner、finalizer 自己建的）都落在 `<work>/tmp`（sizing 的 L3）：

| 序 | 動作 | 失敗 |
|---|---|---|
| — | 協定變數、PATH 與 `command -v`（「二之三之一」）、`/proc/locks`、`I074_STAGE2_MODE` 與模式一致 | 1（promote：9） |
| 0 | **只用 git、shell 與內嵌標準庫**：自身 canonical path ＝ `<work>/repo/scripts/run-i074-stage2.sh`；`.git/objects/info/alternates` 不存在；`origin` ＝ `I074_STAGE2_REAL_REPO`；`repo_head` 以內嵌的 `python3 -c`（`json` ＋ oid40 正規式）從 freeze record 副本取出；HEAD ＝ `repo_head`；`git status --porcelain --untracked-files=no` 為空；`git status --porcelain --untracked-files=all --ignored` 在 `python/baselines/i074_stage2/` 以外沒有任何項目（「三」#8）；自身內容 ＝ `git show <repo_head>:scripts/run-i074-stage2.sh`；（第一次以外）freeze record 與報告副本的 SHA ＝ `state/run.json`、`<work>/bin/docker` ＝ 複本的 shim | 1（promote：9） |
| — | run：寫 `state/run.json`（exclusive）→ 安裝 label shim | 1 |
| 1 | 凍結：複本的 `python/baselines/i074_stage2/counterfactual_e1cbbbd.patch`、`tooling_e1cbbbd.patch` 各讀一次 → `<work>/run/patches/{counterfactual,tooling}.patch`；兩份的 SHA ＝ `git cat-file blob <repo_head>:<路徑>` 的 SHA；**tooling 必須非空**（ba 的 orchestrator 層）；`<work>/run/stage2` ⛔ 不得存在 | 1 |
| 2 | `replay_args_prepare_worktree`（`TMPDIR=<work>/tmp`，base ＝ freeze record 的 `base_commit`）→ `replay_args_compose <wt> <base> <凍結 cf> <凍結 tooling>` → `i074_stage2_freeze_record.py check-full`（「二之九」）→ 移除暫時的 worktree（⚠️ 第一輪 review：這是 replay 之前唯一的清理，**失敗即 1**；⚠️ ⑦b 實作第一輪 review：合成或 `check-full` 失敗時**也**移除） | 1 |
| 3 | 經 shim `docker run`（`--rm --network none --read-only --user <uid:gid>`、mem-guard 下修的 `--memory`、`-v <複本>/python:/app:ro`、identity 唯讀掛載）執行 `i074_stage2_preflight.py anchors`：`load_run_identity(<XDG 的 Stage 2 identity>)` → `expected_image_id` ＝ `REPLAY_IMAGE_ID` → `stage2_archive._load_trust_anchors(python_root, identity)`（第 1～10 道 ＋ E 系列，**以目前的 identity**——ai、ay）→ stdout 一行 JSON：`bundle_id`、`after_base_commit`，以及 after／cohort 的 repo 相對路徑（取自 `stage2_evidence` 的常數，「三」#13）。orchestrator 再驗 `after_base_commit` ＝ `base_commit`、bundle 目錄存在於複本 | 1 |
| 4 | 真正 repo（`git --no-optional-locks`）：`status --porcelain --untracked-files=all --ignored -- python/baselines/i074_stage2/` 為空；HEAD 的 `ls-tree -r -- python/baselines/i074_stage2/failed/` 每一列都逐位元出現在複本 `repo_head` 的同一個輸出裡 → `finalize-stage2-evidence.sh --check-failed-record --counterfactual-patch <凍結 cf>` | 1 |
| 5 | check 回 **2**（同語意 SHA 的已記錄壞 patch）→ 結束碼 **2**；回 1 → 1 | 2／1 |
| 6 | `docker info --format '{{.DockerRootDir}}'`（經 shim；失敗或空 → 1）→ `i074_stage2_preflight.py disk --location run=<work>/run --location tmp=<work>/tmp --location baselines=<複本>/python/baselines/i074_stage2 --location git=<複本>/.git --docker-root <dir>`：全部 `st_dev` 相同，`statvfs(<work>/run)` 的 `f_bavail × f_frsize ≥ REQUIRED_BYTES` | 1 |
| — | 寫 `state/preflight.json` | 1 |
| 7 | replay（下一段） | — |

**replay 與分流**：

1. 寫 `state/replay_started.json`（exclusive；含 replay argv）。
2. `env I074_STAGE=2 COUNTERFACTUAL_PATCH=<凍結 cf> TOOLING_PATCH=<凍結 tooling> <複本>/scripts/run-replay-offline.sh --bundle <複本>/python/baselines/<bundle_id> --output-dir <work>/run/stage2 --before-ref <base_commit> --after-artifact <複本>/python/<after> --cohort-manifest <複本>/python/<cohort> --i074-counterfactual`——⚠️ 兩份 patch 的環境變數**只**出現在這一次呼叫；背景執行 ＋ `wait`；輸出導到 `<work>/logs/`。
3. **輸出形狀**（`<work>/run/stage2` 的檔案集合；檔名鏡像 `stage2_archive` 的 `OPERATIONAL_*`，由測試斷言相等）：rc 0 ⇒ 恰好 before source、comparison、report 三檔；rc 6 ⇒ 恰好 `bounded_diagnostics.json`；其他結束碼或形狀不符 → **1**（⛔ 不寫 attempt，⛔ 不能 `--resume`）。符合 → 寫 `state/replay_done.json`（「二之八」）。
   ⚠️ **replay 開始之後 orchestrator ⛔ 不做任何 worktree 清理**（第一輪 review，「三」#19）：殘留只可能是**複本的** `.git` 裡、登記路徑位於 `<work>/tmp` 底下的
   worktree（runner 的 `exec docker run` 留下的那一個，屬 `P_B` 已計入的 `replay_worktree`；finalizer 自己建的由它自己的契約處理）；⛔ 不影響真正 repo、
   ⛔ 不影響複本完整性（`git status` 只看主工作樹）。⚠️ **殘留（實體目錄與登記）一直保留到整個 `<work>` 被刪除**（晉升結果 commit 之後）——
   ⛔ `git worktree prune` 收不掉它：prune 只移除實體目錄已消失的登記（第二輪 review；2026-09-30 以 git 2.30.2 重現）。
4. **檢查點**（`/proc/locks` → preflight 0 的各項 → **目前的** `REPLAY_IMAGE_ID` 與 XDG identity 檔的 SHA ＝ `preflight.json` 的記錄值，⚠️ 第一輪 review）→ 不符 → **1**、⛔ 不發布（replay 開始之後的不符：停下、另立 issue；⛔ 不寫 attempt）。
5. rc 0 → `finalize-stage2-evidence.sh --finalize --run-dir <work>/run`（⛔ 不帶 `--source-ref`）；rc 6 → `--publish-failed-record --run-dir <work>/run`；背景 ＋ `wait`。
6. **依磁碟事實判終態**（⛔ 不看結束碼——`--publish-failed-record` 完整發布也回 1）：rc 0 的路徑看複本的 `python/baselines/i074_stage2/evidence/`（真實目錄、非 symlink）；rc 6 的路徑看 `failed/<bundle_id>-<語意 SHA>/`（語意 SHA 取自第 2 步的 compose）。
   - 有終態 → 檢查點 → **晉升 stub**：印「晉升尚未實作（⑦c）」→ **9**。
   - 沒有終態、finalize／publish 回 **1** → 寫 `state/attempt.json` → **1**（可以 `--resume`）。
   - 沒有終態、其他結束碼（含 3）→ **1**（矛盾；⛔ 不寫 attempt）。

**中斷**：`trap` TERM／INT／HUP → 對進行中的步驟送 TERM、最多等 5 秒 → 經 shim `docker ps -a -q --filter label=i074.stage2.run=<token>` → `docker rm -f` → 結束 128＋N；
更徹底的清理（KILL、孤兒、容器）由 supervisor 的第 8 列負責。

###### 二之八、`state/`、`--resume`、`--promote`

⚠️ **第一輪 review 改寫**：五份 state 都是**封閉 schema**——canonical JSON（以 `canonical_json_bytes` 序列化，讀取時 raw 必須 ＝ 重新序列化的結果）、頂層 object、
鍵集合**恰好**如下，多欄、缺欄、型別不符（含以 `bool` 冒充整數）一律拒絕。格式：**hex64**、**oid40** 同「八之二」；**image** ＝ `sha256:` ＋ hex64；
**abs** ＝ 以 `/` 開頭、`realpath` 之後不變、不含控制字元的字串；**name** ＝ 非空、不含 `/` 與控制字元的字串。寫入：同目錄 tmp → fsync → `rename` → fsync 目錄
（`run.json` 與 `replay_started.json` 以 exclusive create 建立）。讀取一律經 `i074_stage2_preflight.py state-check --require <名稱,…>`（⚠️ ⑦b 實作第一輪 review 訂正：原本誤寫成 `--kind`）：驗 schema，並以前一份的**檔案 SHA** 驗鏈。

| 檔案 | `schema` | 其餘的鍵（型別） | 跨檔不變條件 |
|---|---|---|---|
| `run.json` | `i074_stage2_orch_run/v1` | `work_dir`（abs）、`real_repo`（abs）、`repo_head`（oid40）、`freeze_record_sha256`（hex64）、`report_sha256`（hex64） | `work_dir` ＝ canonical `<work>`；`real_repo` ＝ `I074_STAGE2_REAL_REPO`；`repo_head` ＝ freeze record 的 `repo_head` ＝ 複本 HEAD；`freeze_record_sha256` ＝ `<work>/freeze/freeze_record.json` 的 SHA；`report_sha256` ＝ 報告副本的 SHA ＝ freeze record 的 `report_sha256` |
| `preflight.json` | `i074_stage2_orch_preflight/v1` | `run_sha256`、`counterfactual_raw_sha256`、`tooling_raw_sha256`、`counterfactual_sha256`、`tooling_sha256`、`composed_sha256`、`counterfactual_semantic_sha256`、`identity_sha256`（hex64）；`base_commit`（oid40）；`image_id`（image）；`bundle_id`（name）；`after_artifact`、`cohort_manifest`（`python/` 開頭、不含 `..` 的 repo 相對路徑） | `run_sha256` ＝ `run.json` 的 SHA；`base_commit` ＝ freeze record 的；兩組 raw ＝ canonical，`tooling_raw_sha256` ≠ 空字串的 SHA；patch 的 raw／canonical、`image_id`、`identity_sha256` 與 freeze record 逐欄相等；`bundle_id`、兩個路徑 ＝ `anchors` 的輸出 |
| `replay_started.json` | `i074_stage2_orch_replay_started/v1` | `preflight_sha256`（hex64）、`argv`（非空的 string list） | `preflight_sha256` ＝ `preflight.json` 的 SHA；`argv` ＝ 由 `preflight.json` 與常數重組的 replay argv（逐項相等） |
| `replay_done.json` | `i074_stage2_orch_replay_done/v1` | `replay_started_sha256`（hex64）、`rc`（int，∈ {0, 6}）、`outputs`（object，值 hex64） | `replay_started_sha256` ＝ `replay_started.json` 的 SHA；`outputs` 的鍵：rc 0 ⇒ 恰好三個 operational 檔名、rc 6 ⇒ 恰好 `bounded_diagnostics.json` |
| `attempt.json` | `i074_stage2_orch_attempt/v1` | `replay_done_sha256`（hex64）、`kind`（`finalize`／`publish_failed_record`）、`rc`（int，恰好 1）、`count`（int，≥ 1） | `replay_done_sha256` ＝ `replay_done.json` 的 SHA；`kind` 與 `replay_done.rc` 對應（0 ↔ `finalize`、6 ↔ `publish_failed_record`）；每次重寫 `count` 加一 |

- **`--resume`**（ae；「三」#7）：協定 → 信任根 → `/proc/locks` → preflight 0 → 五份 state 都存在、各自通過 schema、鏈與交叉條件全部成立 →
  ⚠️ **目前的** `REPLAY_IMAGE_ID` ＝ `preflight.image_id`；**目前 XDG identity 檔**的 SHA ＝ `preflight.identity_sha256`（只改 `created_at` 也不符），且它的
  `expected_image_id` ＝ 同一個 image（第一輪 review）→ 兩份凍結 patch 的 SHA ＝ `preflight` 的 raw SHA、全部輸出檔的 SHA ＝ `replay_done.outputs` →
  已有終態就直接走檢查點 → stub；否則依 `replay_done.rc` 重跑 finalize 或 publish，分流同上。
  ⛔ replay 一律不被呼叫；任一項不符、或缺任何一份 state（例如 replay 中斷、replay 之後的檢查點不符、收到訊號）→ **1**、⛔ finalizer 未被呼叫，
  提示改用 `--promote`（⑦c 由它依磁碟事實裁決）。
- **`--promote`**（⑦b）：協定 → 信任根 → `/proc/locks` → preflight 0（＝「八之三」步驟 1）→ stub **9**；任一不符 → **9**。⑦c 把 stub 換成「八之三」的判定順序。

###### 二之九、freeze record

**寫入端（sizing harness）**：

- 真實 tooling：`TOOL_PATCH=<sizing 複本>/python/baselines/i074_stage2/tooling_e1cbbbd.patch`（必須非空）。patched worktree 改以
  `replay_args_compose "$wt" "$BASE" "$CF_PATCH" "$TOOL_PATCH"` 建立，第一次呼叫時斷言兩份 raw SHA 各自 ＝ 增量 canonical SHA；
  快照與 `freeze_patches()` 放真實 tooling；fixture 改收 `--composed-sha256`（host 算好的合成 SHA）當 provenance 的 `tooling_patch_sha256`（「三」#16）。
- `meta` 新增 `tooling_sha256`（raw）、`counterfactual_canonical_sha256`、`tooling_canonical_sha256`、`composed_sha256`、`counterfactual_semantic_sha256`；
  `--formal` 時 identity 檔必須存在；`--formal` 的「檔案 ＝ HEAD」清單加入 `i074_stage2_freeze_record.py`、`i074_stage2_preflight.py`
  （其餘被它們載入的檔案已在既有的「`scripts/`、`python/` 必須 clean」之內）。
- 步驟 6：`helper report` 之後、任何檔案複製到 `<work>` **之前**，`--formal` 時呼叫 `i074_stage2_freeze_record.py build`：只有報告的
  `mode = formal`、`status = ok` 且 `P_B ≤ P_B_BUDGET` 才在 S 寫 `freeze_record.json`（先以驗證端的同一組函式驗自己的輸出），否則印出原因、⛔ 不寫（「三」#9）；
  三個腳本 SHA 以 `git show <repo_head>:<path>` 的**內容**計算（⛔ 不是 blob OID）並比對 `meta`；`base_commit` 必須 ＝ 常數（`e1cbbbd` 的完整 OID）。
  複製到 `<work>` 之後對兩個副本再驗一次，不符就刪掉 freeze record 並中止。validation 模式、`assumption_violated`、任何失敗路徑都⛔ 不寫（n10）。

**驗證端（`i074_stage2_freeze_record.py`）**：

- `check-basic --record <p>`：canonical（raw ＝ `canonical_json_bytes(parsed)`）＋「八之二」封閉 schema 與逐欄規則（含兩組 raw ＝ canonical、
  tooling raw ≠ 空字串的 SHA、`clone_head` ＝ `repo_head`、`base_commit` ＝ 常數、`p_b_bytes ≤ P_B_BUDGET`）。
- `check-full --record --report --clone --frozen-counterfactual --frozen-tooling --counterfactual-canonical --tooling-canonical --identity --image`：上一項 ＋
  `report_sha256` ＋ 交叉不變條件（報告的 `mode`、`status`、`P_B` 與 `meta` 的 run id、`repo_head`、`clone_head`、三個腳本 SHA、image、identity SHA、
  兩個 raw SHA 逐欄相等）＋ 三個腳本 SHA ＝ 複本中檔案內容的 SHA ＋ 兩份凍結 patch 的 raw SHA 與 compose 的增量 SHA ＝ record ＋
  identity 檔的 SHA ＝ record、identity 的 `expected_image_id` ＝ record ＝ `REPLAY_IMAGE_ID`。

###### 二之十、端到端結束碼（模式 × 失敗點；⑦b 的範圍）

| # | 失敗點 | 位置 | run／resume | promote | sentinel |
|---|---|---|---|---|---|
| E0 | 不是以正式命令啟動（`$-` 不含 `p`；⚠️ 第二輪 review） | 入口 | 1 | 8 | 不建立 |
| E1 | 用法錯誤（未知參數、缺值、重複、`=value`、由外部傳入內部旗標、裸 `--repo-head`、路徑覆寫）；`REPLAY_IMAGE_ID` 缺少或不符 `^sha256:[0-9a-f]{64}$`（第二輪 review） | 入口 | 1 | 8 | 不建立 |
| E2 | 環境已帶 `I074_STAGE2_*` | 入口／supervisor | 1 | 8 | 不建立 |
| E3 | 入口清單的投影 ≠ HEAD（任一檔內容不同或未追蹤；清單以外的檔案不算） | 入口 | 1 | 8 | 不建立 |
| E4 | work 目錄規則不符 | 入口 | 1 | 8 | 不建立 |
| S0 | 不是初始 user namespace（第二輪 review）、或四個系統程式的信任條件不符（第一輪 review） | supervisor（取鎖之前） | 1 | 8 | 不建立 |
| S1 | supervisor 自身 ≠ HEAD | supervisor | 1 | 8 | 不建立 |
| S2 | uid ≠ 1001 | supervisor | 1 | 8 | 不建立 |
| S3 | 鎖檔屬性不符（symlink、非 regular、owner、mode、link count、開啟後 inode 被換） | supervisor | 1 | 8 | 不建立 |
| S4 | 鎖衝突 | supervisor | 1 | 8 | 不建立（不是自己的，⛔ 不動） |
| S5 | sentinel 已存在（含讀不懂） | supervisor | 1 | 9 | 不動；提示需要重開機 |
| S6 | `docker image inspect` 失敗、或 image 的 labels 含鍵（⚠️ 第三輪 review：docker 本身的信任已歸 S0，這一列只剩 image） | supervisor | 1 | 8 | 不建立 |
| S7 | 有殘留容器或 `docker ps` 失敗 | supervisor | 1 | 8 | 不建立 |
| S8 | sentinel 建立之後、workload child 成功啟動之前的可控失敗 → 收回成功 | supervisor | 1 | 8 | 刪除 |
| S9 | 同上但收回不成立（inode 被換、後代或容器清不掉） | supervisor | 1 | 9 | 留下；需要重開機 |
| L1 | 持鎖階段：重驗檔案、freeze record 基本欄位、`repo_head` 不在真正 repo、`mkdir`／clone／checkout 失敗、resume／promote 的 layout 不符 | 持鎖階段 | 1 | 8 | 正常釋放 |
| L2 | ⚠️ ⑦b 實作第一輪 review：持鎖階段在 exec 之前驗複本——HEAD ≠ `repo_head`、orchestrator ≠ 真正 repo 物件裡的 blob、resume／promote 的 freeze record 副本與 `run.json` 不符 | 持鎖階段 | 1 | 9 | 正常釋放 |
| O1 | 協定變數或 `/proc/locks` 驗證失敗 | 複本 | 1 | 9 | 正常釋放 |
| O2 | preflight 0（完整性） | 複本 | 1 | 9 | 正常釋放 |
| O3 | preflight 1～4、6 失敗（含第 2 步的暫時 worktree 移除失敗）；check 回 1 | 複本 | 1 | — | 正常釋放 |
| O4 | lookup 命中 | 複本 | **2** | — | 正常釋放 |
| O5 | replay 的結束碼 ∉ {0, 6}、或輸出形狀不符 | 複本 | 1（⛔ 不能 resume） | — | 正常釋放 |
| O6 | replay 之後的檢查點不符（含 image 或 identity 與 `preflight.json` 不符） | 複本 | 1（⛔ 不能 resume；另立 issue） | 9 | 正常釋放 |
| O7 | finalize／publish 回 1 且沒有終態 | 複本 | 1（寫 attempt，可 resume） | — | 正常釋放 |
| O8 | 已有終態（finalize 0／3、publish 1／3）→ 晉升（⚠️ **⑦c 細部計畫 v1（✅ 2026-10-01 確認）**：stub 換成真正的晉升，見 ⑦c「二之五」） | 複本 | 晉升的結束碼 **0**／**6**／**3**／**8**／**9** | 同左 | 正常釋放 |
| O9 | resume 的 state 不符（缺任何一份、schema 或鏈不符、image 或 identity 與 `preflight.json` 不符、凍結 patch 或輸出的 SHA 變了） | 複本 | 1（finalizer 未被呼叫） | — | 正常釋放 |
| G1 | orchestrator 或 supervisor 攔截到訊號 N | 任一 | 128＋N | 128＋N | 正常釋放 |
| G2 | supervisor 被 SIGKILL（含 OOM killer） | — | 137 | 137 | 留下；需要重開機 |
| G3 | 正常釋放清不空（容器、後代、inode） | supervisor | ⛔ 不結束，持續回報 | 同左 | 留下且持鎖 |

⚠️ 0、6、3 與晉升本身的 8 要到 ⑦c 才可達（⑦b 的終態一律經 stub 回 9）。結束碼常數由 supervisor 與 `run-i074-stage2.sh` 各自鏡像
（1、2、8、9），測試釘住字面值並斷言兩邊相等（「三」#2）。

##### 三、總綱沒寫到、本計畫補上的決定（✅ 2026-09-30 確認）

| # | 決定 | 理由 |
|---|---|---|
| 1 | preflight 第 3 步是 `i074_stage2_preflight.py anchors`：在 Stage 2 image 內以**目前 XDG 的** identity 呼叫 `stage2_archive._load_trust_anchors()`；⛔ 不擴充 ③ 的 `--check-failed-record` | check 模式拿 envcheck 封存的 identity 當本次的 identity，驗不到 ai；擴充它要改 ③「七之四」的 CLI matrix 與 `replay_bundle/`（tooling 路徑，得重產 tooling patch）。直接呼叫同一個 `_load_trust_anchors()`，推導只有一份。放在 Stage 2 image 內（⛔ 不用 host 的 bootstrap）：與 finalizer 的 Python 段同一個直譯器與 image，shell 整合測試也能以 fake docker 控制它 |
| 2 | 8、9 在 ⑦b 只由 supervisor 與 orchestrator **鏡像字面值**（測試釘住）；`publish.py` 的常數與跨檔相等測試照總綱留在 ⑦c | ⑦b 因此完全不動 tooling 路徑，tooling patch 與 ⑦a 的封存不受影響 |
| 3 | freeze record 的 canonical JSON 以 `_i074_bootstrap` 載入 `canonical.py`（host 可用、單一定義）；sizing helper 既有的 `canonical_dumps` 不動 | 八之二規定「與證據相同的 canonical JSON」，⛔ 不再多一份副本 |
| 4 | 持鎖階段**先把 freeze record 與報告複製進 `<work>/freeze/`、再驗副本**（「八之一」寫的是先驗基本欄位再複製） | 先驗再複製有 TOCTOU：驗到的與之後使用的可能不是同一份 |
| 5 | label shim 由**複本內** orchestrator 從複本安裝（`repo_head` 版），PATH 由 supervisor 設；第一次呼叫 docker 之前驗 `command -v docker` | shim 屬於 ⑩ 的呼叫圖，應與其餘程式碼一起固定在複本的 OID；supervisor 啟動時複本還不存在 |
| 6 | shim 對**任何子指令**都拒絕前置的全域選項；「token 含鍵」只對 run／create 拒絕 | 只在 run／create 檢查時，`docker -H x run` 的第一個參數不是 `run`、會被原樣放行而建立沒有 label 的容器；orchestrator 中斷時的 `docker ps --filter label=<鍵>=<token>` 需要用到鍵 |
| 7 | `--resume` 只接受「finalize／publish 回 1 且磁碟上沒有終態」（`state/attempt.json`）；replay 中斷、replay 之後的檢查點不符、訊號都⛔ 不能 resume | ae 的範圍就是「finalize 回 1 之後重跑」；其餘情況「八之三」規定走 `--promote` 依磁碟事實裁決（⑦b 為 stub → 9，交人工） |
| 8 | 真正 repo 的「`i074_stage2/` 底下沒有未追蹤項目」與複本的「`i074_stage2/` 以外沒有未追蹤檔」都把 **ignored** 檔算進去 | 被 ignore 的 failed record 同樣進不了下一個複本；複本裡多出的 `__pycache__` 等也代表有程式在複本內寫入 |
| 9 | `--formal` sizing 得到 `assumption_violated` 或 `P_B > P_B_BUDGET` 時⛔ 不寫 freeze record，但結束碼維持 0 | 量測本身有效；缺少 freeze record 本身就擋住 ⑩（入口只接受 freeze record） |
| 10 | 正常釋放清不空時，supervisor **不結束**：持鎖、留 sentinel、每 30 秒重試並回報，直到清空（或操作者 SIGKILL，走重開機） | 第 8 列「⛔ 不刪 sentinel、⛔ 不放鎖，持續回報、等人工處理」；結束 supervisor 會連帶放鎖 |
| 11 | n7b 的 deterministic barrier：fake runner 停在「建立容器之前」→ SIGKILL 舊 supervisor（137）→ 新 supervisor 被 sentinel 擋下（1，建立複本、preflight、replay 都未被呼叫）→ 放開 barrier：fake runner（忽略 TERM，模擬「舊流程沒有停下」）經 shim 建立容器，並以**同一組協定變數**呼叫複本 orchestrator 的 `--internal-in-clone resume`（下一個檢查點）→ `/proc/locks` 驗證失敗、finalize 未被呼叫。之後模擬重開機（清空隔離的鎖目錄）：fake docker 裡仍有這個帶鍵的容器 → 新 supervisor 回 1；移除之後放行 | 舊 orchestrator 本身會因 parent-death TERM 結束，無法讓它「繼續走到檢查點」；改由存活的後代以同一組協定走同一個檢查點函式，驗到的是同一段程式 |
| 12 | 測試的隔離做法：`sed` 改的常數除了鎖檔、sentinel、uid、label 鍵，**另加** `i074_stage2_freeze_record.py` 的 base OID（合成 repo 沒有 `e1cbbbd`）與信任根常數（四個程式路徑、`TRUSTED_PATH`、信任的 owner uid——測試的 fake 是 dev 擁有；⚠️ 第一輪 review 追加），並斷言與正式檔案只差這幾行；n5 的磁碟拒絕以 fake `docker info`（失敗、或回 `/dev/shm` 底下的目錄）觸發，⛔ 不 sed 預算；第一次執行時的 preflight 0 失敗以 **fake `git` 包裝**（放在 sed 過的 `TRUSTED_PATH` 目錄，其餘轉交真的 git）在 clone／checkout 之後改動複本來觸發 | 合成 repo 才能讓每個情境幾秒內跑完；fake 只存在測試環境，⛔ 不是正式的覆寫口 |
| 13 | replay 的 bundle、after、cohort 路徑一律取自 `anchors` 的輸出（`stage2_evidence` 的常數與已驗證 manifest 的 `bundle_id`）；shell ⛔ 不再寫一份 | 單一來源；也是 ad「⛔ 由使用者覆寫階段一的輸出」的結構性保證 |
| 14 | `state/` 的五份封閉 schema 與 SHA 鏈（「二之八」；⚠️ 第一輪 review 補完） | 總綱把 `state/` 的內容留給 ⑦b 定 |
| 15 | 入口在三種模式都要求 `REPLAY_IMAGE_ID` | supervisor 在殘留檢查之前要驗 image 的 `Config.Labels`（總綱「四」的 label 列），需要知道是哪一個 image |
| 16 | sizing 的 fixture 改收 host 算好的合成 SHA（取代 `sha256(counterfactual patch)`） | tooling 非空之後兩者不再相等，finalize 的合成守門會擋下 |
| 17 | freeze record 的驗證分 `check-basic`（持鎖階段，真正 repo 的 HEAD 版）與 `check-full`（複本內，`repo_head` 版），規則函式同一份 | 對應「八之二」兩個驗證位置；持鎖階段還沒有複本，驗不了交叉條件 |
| 18 | ⚠️ 第一輪 review：信任根固定成 `/usr/bin/git`、`/usr/bin/docker`、`/usr/bin/python3`、`/bin/bash` 與 `TRUSTED_PATH=/usr/bin:/bin`，supervisor 在取鎖之前驗 root 擁有、⛔ 別人可寫（⚠️ 第二輪 review：先驗是初始 user namespace）；取代總綱「四」的「PATH 扣掉 shim 目錄之後的 realpath」 | 呼叫者的 PATH（本機是 `~/.local/bin`、`/usr/local/bin` 在前）能讓假 docker 偽造 inspect／ps 並成為 shim 轉交的對象，假 git 能偽造 HEAD 驗證；固定路徑之後，要偽造就得是 root |
| 19 | ⚠️ 第一輪 review：replay 開始之後 orchestrator ⛔ 不清 worktree；replay 之前的暫時 worktree 移除失敗即 1。⚠️ 第二輪 review 訂正：殘留一直保留到整個 `<work>` 被刪除，⛔ 不宣稱 prune 能收斂 | 清理若 fail-open 會違反清理契約；若 fail-closed，一個與證據無關的登記就能讓三小時的 replay 作廢。不清理就沒有失敗點，殘留限定在可拋棄的複本（prune 只清實體目錄已消失的登記，收不掉它） |
| 20 | ⚠️ 第二輪 review：正式命令直接執行、shebang `#!/bin/bash -p`；入口在任何外部指令之前以 bash 內建驗 `$-`、清 `BASH_ENV`／`ENV`／`LD_*`／`PYTHON*`、固定 PATH，再 `exec /usr/bin/python3 -I`；保證範圍照實寫成「⛔ 把污染傳入後續 workload」，⛔ 不宣稱完整信任根 | `bash <script>` 會在第一行之前執行 `BASH_ENV`；`LD_PRELOAD` 在直譯器載入時就生效，無特權的流程只能不讓它往下傳 |
| 21 | ⚠️ 第二輪 review：`REPLAY_IMAGE_ID` 在入口與 supervisor 各驗一次 `^sha256:[0-9a-f]{64}$`，在任何 docker 呼叫之前 | ⛔ 讓 option-like（`--help`）或可移動的 tag 進入 docker argv |

##### 四、受影響檔案

| 檔案 | 改動 |
|---|---|
| `scripts/run-i074-stage2.sh` | **新增**（入口、持鎖階段、複本內 orchestrator、`/proc/locks` 驗證、晉升 stub） |
| `scripts/lib/i074-stage2-supervisor.py` | **新增** |
| `scripts/lib/i074-stage2-docker-label-shim.sh` | **新增** |
| `python/scripts/i074_stage2_freeze_record.py`、`python/scripts/i074_stage2_preflight.py` | **新增** |
| `scripts/i074-stage2-sizing.sh` | 真實 tooling、compose、meta 新欄、formal 清單、freeze record、與 label shim 互斥 |
| `python/scripts/i074_stage2_sizing.py` | fixture 的 `--composed-sha256` |
| `scripts/lib/i074-sizing-docker-shim.sh` | 拒絕 `I074_STAGE2_*` 與 label shim |
| `scripts/test-i074-stage2.sh`、`scripts/tests/test_i074_stage2_host.py` | **新增** |
| `python/backtest/modular/sr_scoring/tests/test_i074_stage2_preflight.py`、`test_i074_stage2_freeze_record.py` | **新增**（pytest） |
| `python/backtest/modular/sr_scoring/tests/test_i074_stage2_sizing.py`、`scripts/test-replay-args.sh` | fixture 參數、formal 清單 |
| `python/scripts/test.sh` | `SKIP_SHELL_TESTS` 區段呼叫 `scripts/test-i074-stage2.sh` |
| `docs/issue.md`、`docs/development-workflow.md` | 計畫書、實作結果、歸檔 |

⛔ 不改：`evaluation.py`、`replay_bundle/`、`run-replay-offline.sh`、`finalize-stage2-evidence.sh`、`scripts/lib/replay-args.sh`、兩份 patch。

##### 五、測試（id → 落點）

落點：**pytest**（docker 內，純邏輯）；**host unittest**（`scripts/tests/test_i074_stage2_host.py`，Python 3.9：supervisor 的內部函式、需要 git 的 helper、
host 模組在 3.9 可 import）；**shell**（`scripts/test-i074-stage2.sh`：合成 repo ＋ sed 過常數的正式腳本 ＋ fake docker／runner／finalizer／git）。

| id | 內容 | 落點 |
|---|---|---|
| n1、n2、n6；n4 的 stat／statvfs 失敗 | `== REQUIRED` 通過、`−1` 拒絕；`f_bavail` 不足但 `f_bfree` 足夠 → 拒絕；stat／statvfs 丟例外 → 拒絕；三個常數與算術 | pytest（注入 stat／statvfs） |
| n3 | 任一位置或 Docker Root Dir 的 `st_dev` 不同 → 拒絕 | pytest ＋ shell（fake `docker info` 回 `/dev/shm` 底下的目錄） |
| n4（Docker Root Dir 取不到）、n5 | fake `docker info` 失敗或回空、以及 n3 的 shell 那一支：結束碼 1、runner／finalize 的 spy ⛔ 未被呼叫、`state/preflight.json` 不存在 | shell |
| n7 | 入口清單的**每一個**檔案逐一竄改（改內容、`git rm --cached` 成未追蹤，各一支）→ 1／8 且沒有取鎖（獨立 fd 拿得到、沒有 sentinel），對照組：清單以外的檔案 dirty → 照常通過（⚠️ 第一輪 review）；freeze record 違反「八之二」的每一條（pytest 逐條，shell 兩支）；HEAD ≠ `repo_head`、已追蹤檔被改、`i074_stage2/` 以外有未追蹤檔（含 ignored）、有 alternates、orchestrator 自身內容 ≠ `repo_head` 版（fake git 在 checkout 之後改動）；orchestrator 不在複本內執行（fake runner 以協定變數呼叫真正 repo 那一份）；真正 repo 的 `i074_stage2/` 有未追蹤項目 → 各自 1；⚠️ 完整性不符的各支：`anchors`、`--check-failed-record`、runner **都未被呼叫**；入口⛔ 接受 `--repo-head` | pytest ＋ shell |
| n7b | 「八之一之二」逐列：獨立 fd 探測執行中拿不到、正常結束後立刻拿得到；不同 `XDG_DATA_HOME`、不同 repo 同時啟動 → 後到的 1；非固定帳號（sed uid）→ 1／8；鎖檔與 sentinel 屬性（symlink、非 regular、mode、link count、開啟後 inode 被換）；鎖檔從未被 unlink；外部 helper 不延長鎖；`setsid` 的孤兒回到 subreaper、它消失之前不放鎖；TERM → 移除容器 → TERM／KILL → 兩者都消失之前拿不到鎖；清不空 → 不刪、不放鎖（持續回報）；SIGKILL → 137、sentinel 留下、新 supervisor 1（promote 9）且提示重開機；deterministic barrier（「三」#11）；sentinel 讀不懂 → 拒絕；沒有解除入口；sentinel schema 每欄各一支；模擬重開機後有／無殘留容器；啟動檢查失敗不留 sentinel、排除後直接重跑成功；收回（fsync 失敗、`fork` 成功而 exec 失敗）；刪除前 inode 被換（收回與正常釋放各一支）；直接執行複本 orchestrator、或傳入正確鎖檔但沒有持鎖的 pid → 在完整性檢查之前中止；label（每個 run／create 恰好一個、自帶鍵拒絕、`docker ps` 失敗 fail-closed、image labels 含鍵）；supervisor 自身 dirty；結束碼（以實際 argv：鎖衝突 1／8、sentinel 1／9、可攔截訊號 128＋N） | host unittest（屬性、schema、收回與 inode 的故障注入）＋ shell（其餘） |
| n8 | fake runner 的副作用在 replay 之後改動複本（已追蹤檔、freeze record 副本、`<work>/bin/docker`）→ 檢查點不符、finalize／publish 未被呼叫、⛔ 沒有 attempt；`--promote` 之前改動 → 9、stub 未到達 | shell |
| n10 | `build`：formal ＋ ok ＋ ≤ 預算 → 通過封閉 schema 與交叉條件；validation、`assumption_violated`、`P_B > P_B_BUDGET` → 不寫；腳本 SHA ＝ 內容 SHA 且 ≠ blob OID；`--formal` 時新模組 dirty → harness 中止 | pytest ＋ host unittest（git）＋ `test-replay-args.sh`（sizing 段） |
| n12（preflight 部分） | 真正 repo 有未追蹤（或 ignored）的 failed record → 1；真正 repo HEAD 的 `failed/` ⊄ 複本（舊的 freeze record）→ 1；check 回 2 → 2 且 runner 未被呼叫；回 0 → 進 replay | shell |
| ad | rc 0 ＋ 三檔 → finalize；rc 6 ＋ 中繼檔 → publish；rc 1、2、4、137 → 都不呼叫、1；rc 0 但多一檔或少一檔、rc 6 但另有 comparison → 1；入口帶 `--after-artifact`、`--bundle`、`--output-dir`、`--before-ref` → 1 且沒有建複本 | shell |
| ae | finalize 回 1 且沒有終態 → 1 ＋ attempt；`--resume` → runner 未被呼叫、finalize 以同一個 `--run-dir` 呼叫、凍結 patch 與輸出的 SHA 不變 → 第二次建立 `evidence/` → 9；沒有 attempt 就 resume → 1；輸出被改 → 1；publish 回 1 但 record 已在磁碟 → 9（⛔ 不寫 attempt）；publish 回 1 且沒有 record → attempt → resume 再 publish；finalize 回 3 ＋ `evidence/` 存在 → 9 | shell |
| ax | fake `anchors` 容器在凍結之後改掉複本常數路徑上的 counterfactual → runner 收到的是凍結副本（路徑與 SHA 皆為原值）→ 之後的檢查點因複本 dirty 中止 | shell |
| ba（orchestrator 層） | `repo_head` 的 tooling patch 是 0 bytes → 1、runner 未被呼叫 | shell |
| n／ai／ay（「在 replay 之前」） | n：check 回 2 → 2、runner 未被呼叫；ai、ay：`anchors` 以目前的 identity 呼叫 `_load_trust_anchors()`（spy）、identity 只差 `created_at` → 拒絕、cohort 被竄改的 python root → 拒絕；shell：`anchors` 失敗 → 1、check 與 runner 未被呼叫 | pytest ＋ shell |
| 信任根（第一輪 review） | 呼叫者 PATH 的前面放 fake `git`／`docker`／`python3`（記錄每一次呼叫）→ 完整的 fake 流程照常、這些 fake ⛔ 從未被執行；四個程式任一信任條件不符（非 regular、owner 不是信任 uid、group／other 可寫、某一層目錄可寫）→ 1／8、沒有取鎖；持鎖階段或複本的 `PATH`、`command -v` 不符、`<work>/bin` 多一個檔案 → 中止；shim 的 `I074_STAGE2_REAL_DOCKER` 不符信任條件 → 125；⚠️ 第二輪 review 追加：`BASH_ENV`／`ENV` 指向會寫標記檔的腳本 ＋ fake PATH，以正式命令直接執行 → 標記檔⛔ 不存在、流程照常；同樣的環境改用 `bash <script>` 啟動 → 1／8、supervisor ⛔ 沒有啟動（沒有取鎖、沒有 sentinel、fake docker 沒有任何呼叫）；`PYTHONPATH` 放會寫標記檔的 `sitecustomize.py` → 標記檔⛔ 不存在；`LD_PRELOAD`、`BASH_ENV`、`PYTHON*` 在 supervisor 與全部下游的 spy 裡都⛔ 看不到；`uid_map` 不是初始 namespace（unittest 注入）→ 1／8；`REPLAY_IMAGE_ID` 為空、`--help`、`-f`、`<name>:<tag>`、大寫 hex、63 位、尾端空白或換行 → 1／8 且 docker ⛔ 一次都沒被呼叫；兩個腳本的 shebang 恰好是 `#!/bin/bash -p` | host unittest ＋ shell |
| worktree（第一輪 review） | fake git 讓 preflight 第 2 步的 `worktree remove` 失敗 → 1、`state/preflight.json` 不存在、runner 未被呼叫；完整的 fake 流程結束後，複本的登記（與實體目錄）只在 `<work>/tmp` 底下、真正 repo 的登記數不變；replay 開始之後 orchestrator ⛔ 沒有呼叫任何 `worktree remove`／`prune`（fake git 記錄） | shell |
| state（第一輪 review） | 五份 state 各自：缺欄、多欄、型別（含 `bool`）、非 canonical、鏈的 SHA 不符、交叉條件不符 → `state-check` 拒絕；`--resume` 時 `REPLAY_IMAGE_ID` 改變、XDG identity 只改 `created_at`、凍結 patch 或輸出被改、任何一份 state 被刪 → 1 且 finalizer ⛔ 未被呼叫；第一次執行時 replay 期間改掉 identity → 檢查點中止、finalizer 未被呼叫 | pytest ＋ shell |
| 環境 | 外層先設 `GIT_*`、`DOCKER_*`、`TOOLING_PATCH`、`COUNTERFACTUAL_PATCH`、`MEM`、`SIZING_*`、`LD_PRELOAD`、`BASH_ENV`、`PYTHONPATH` 等：runner spy 只看到凍結副本的兩個 patch 路徑；finalize 與 check 的 spy **收不到**兩份 patch 的變數；其餘清單內的變數誰都收不到 | shell |
| label shim | 四種子指令恰好插入一個 label；其他子指令原樣放行（stdout、stderr、結束碼透明）；全域選項（含 `-H … run`）、`--label-file`、`--label-file=…`、含鍵的 token（`-l`、`--label=`、值內含）、token 格式錯或 ≠ sentinel、real docker 是自己或 sizing shim、帶 `SIZING_*` → 125 且真正的 docker 未被呼叫；sizing shim 與 sizing harness 拒絕 `I074_STAGE2_*` 與 label shim | shell |
| 靜態 | ⑩ 呼叫圖的三條規則；orchestrator 鏡像的 `OPERATIONAL_*`／`FROZEN_*` 名稱 ＝ `stage2_archive`（host bootstrap 載入比對）；兩處結束碼常數相等且為字面值；sed 過的副本與正式檔案只差約定的那幾行 | shell |
| 隔離 | 測試前後 `/run/lock/i074-stage2.*` 的狀態不變（都不存在）；真正 repo 的 worktree 登記數不變；⛔ 沒有任何帶正式鍵的 docker 呼叫（fake docker 記錄） | shell |
| host 3.9 | supervisor、`i074_stage2_freeze_record.py`、`i074_stage2_preflight.py` 以 host 的 python3 `py_compile` 並 import | host unittest |

##### 六、驗證

0. ⚠️ 第二輪 review：**實作開始之前**，在正式執行帳號（`dev`）的登入 shell 重驗 `/proc/self/uid_map` ＝ `0 0 4294967295`、四個系統程式與上層目錄的 owner uid ＝ 0、`/usr/bin/bash` 不存在（`#!/usr/bin/env bash` 解析到 `/bin/bash`）；任一不符 → 停下改計畫（2026-09-30 已在本 session 實測全部成立）。
1. `python/scripts/test.sh` 完整執行（含 `test-replay-args.sh`、`test-i074-stage2.sh`、doc-refs、pytest），依序執行。
2. 反向驗證（逐項注回、確認變紅、還原）：拿掉 `/proc/locks` 的祖先鏈檢查；shim 只在 run／create 檢查全域選項；拿掉 tooling 非空檢查；改成看結束碼判終態；
   檢查點跳過 preflight 0；`anchors` 改傳 `None` 當 identity（ai 必須變紅）；supervisor 不清 `TOOLING_PATCH`；sentinel 不比 inode 就 unlink；
   後代還在就放鎖；freeze record 在 validation 模式也寫；⚠️ 第一輪 review 追加：supervisor 改回從 PATH 取 docker（PATH 前置 fake 的那一支必須變紅）；
   `--resume` 不比 identity（只改 `created_at` 的那一支必須變紅）；`state-check` 不驗鏈；replay 之後加回「失敗只記錄」的清理（「沒有 `worktree remove`」的斷言必須變紅）；⚠️ 第二輪 review 追加：拿掉 `$-` 含 `p` 的檢查（`bash <script>` 那一支必須變紅）；入口不清 `LD_*`／`PYTHON*` 就 exec（下游 spy 那一支必須變紅）；拿掉 `REPLAY_IMAGE_ID` 的格式檢查（`--help` 那一支必須變紅）。
3. sizing **validation 模式**實跑一次（`REPLAY_IMAGE_ID` ＝ Stage 2 image，約 6 分鐘）：真實 tooling 的 compose、fixture 的合成 SHA、finalize／publish／check 的
   預期結束碼全部成立；記錄 `P_B`（⚠️ 開發觀察值，⛔ 不是 ⑨-2）。
4. stage 之後跑 `scripts/make-i074-tooling-patch.sh --verify "$(git write-tree)"` ＝ 0（tooling patch 未變）。
5. 真正 repo 的 worktree 登記數、`python/baselines/`、`/run/lock` 前後不變（複本內的殘留依「二之七」只可能在 `<work>/tmp` 底下，⛔ 不在此列）。
6. commit 之後（使用者同意時）：`--formal` sizing 實跑一次，確認寫出的 freeze record 通過 `check-basic`；記下 SHA 與 `P_B`（⚠️ 同樣只是開發驗證，
   ⑩ 用的 freeze record 一律來自 ⑨-2）。

##### 七、風險與回滾

| 風險 | 對策 |
|---|---|
| supervisor 的缺陷在正式執行時留下真正的 sentinel（只能重開機，會中斷 live 服務） | 「八之一之二」每一列都有隔離測試；⑦b 不在真正 `/run/lock` 執行任何東西；第一次正式使用是 ⑩，之前還有 ⑦c、⑦d 的測試與 ⑧ 的全量執行 |
| host 是 Python 3.9，容器是 3.11 | host 模組以 host python3 編譯、import 並跑 unittest；pytest 另在容器跑 |
| 合成 repo 與正式環境有落差 | 測試斷言 sed 過的副本只差約定的常數；需要真實資料的部分（`anchors`、compose、`--check-failed-record`）由 pytest 與 ⑦a 的測試對真正 repo 驗 |
| sizing 改用真實 tooling 之後的量測語意 | 「六」第 3 步實跑；⑨-2 才是正式值 |
| `<work>/logs/` 不在 `P_B` 的量測範圍（sizing 的步驟輸出寫在 tmpfs） | 內容是 KB～MB 級、由 `M_safety` 吸收；⑨-1 量的是實際流程 |
| 信任根固定在 `/usr/bin`、`/bin`（第一輪 review） | host 升級套件或搬動 docker → supervisor fail-closed（1／8、不取鎖），改常數就是改計畫 |
| 回滾 | ⑦b 只新增檔案並修改 sizing 與測試；⑦a ⛔ 不依賴 ⑦b，可以單獨 revert ⑦b；⑦c／⑦d 開工之後依總綱「六」的反向順序 |

##### 八、歸檔（實作後；review 前保留本筆的計畫內容）

- `development-workflow.md`：新增「I-074 Stage 2 的正式執行程序」——入口 → supervisor → 持鎖階段 → 複本、公開 argv、preflight 0～7、`state/` 與 `--resume`、
  「模式 × 失敗點」結束碼表、重開機才解除的條件、label shim 與 sizing shim 的互斥、freeze record 的寫入與驗證、測試的隔離規則；sizing 一節改寫
  （真實 tooling、freeze record、`--formal` 清單）。
- `issue.md`：⑦b 實作結果（與計畫的差異、反向驗證、sizing 實跑的觀察值）。

#### Stage 2 步驟 ⑦b 實作結果（2026-09-30，✅ **review 通過**（兩輪修正後）並 commit `ce17ab2`）

✅ 依「Stage 2 步驟 ⑦b 細部計畫 v1」（三輪 review 後確認）完成「二」的全部設計。⛔ **沒有在真正的 `/run/lock` 執行任何東西、沒有跑正式 replay、
沒有 commit**；程式與文件已 stage，停在 review。⚠️ ⑦b 沒有動 tooling 路徑（`evaluation.py`、`replay_bundle/`）：stage 之後
`scripts/make-i074-tooling-patch.sh --verify "$(git write-tree)"` 通過，tooling patch 仍是 `353c69a0…`。

**⑦b 實作第二輪 review 的修正（2026-10-01）**：

| # | 問題 | 修正 |
|---|---|---|
| 中 | ⛔ **deterministic-barrier 測試是紅的、沒有真正建立殘留容器**：review 的執行出現 3 項失敗（舊流程建立的容器沒有 label、模擬重開機之後沒有殘留容器可攔、`--promote` 回 9 而非 8）；review 推測原因是 helper 以一般 bash 執行、label shim 要求 privileged mode | ⚠️ 查證：「一般 bash 呼叫 `docker`」⛔ 會讓 shim 失去 `-p`——以名字呼叫時執行的是 shim 檔案本身，由它的 `#!/bin/bash -p` 啟動（實測 `$-` ＝ `hpB`），而且這三項在我的執行都通過。✅ **真正的原因是時序競爭，已重現**：supervisor 被 SIGKILL 之後，舊 orchestrator 的 `on_signal` 最多等 5 秒（runner 忽略 TERM）、再以 label 移除本趟的容器——barrier 若在這段期間放開，runner 剛建立的容器就被它刪掉。把「放開 barrier」移到 SIGKILL 之後立刻執行的暫時變體，**穩定重現同樣的 3 項失敗**。修正：fake runner 記下父程序（複本內的 orchestrator）的 pid，測試**先等舊 orchestrator 確實結束才放開 barrier**（新增一項斷言）；helper 的 `docker create` 失敗就記下結束碼並中止，測試另斷言它成功——fixture 的設定失敗⛔ 再被後面的步驟掩蓋 |
| 低 | ⛔ **sentinel 的 fd 關閉失敗之後會卡在錯誤的重試分支**：`removal_pending` 設成真之後 `os.close()` 拋錯，`release()` 回到完整清理 → inode 比對（路徑已不在）永遠失敗 → 再也不重試目錄 fsync | ✅ 兩道：①unlink 之後關 fd 改成 best-effort（失敗只回報，⛔ 妨礙狀態轉移）；②`release()` 在 `removal_pending` 時直接進 `finish_removal()`、⛔ 回頭重做清理與 inode 比對（收回本來就這樣）。unittest 補「關 fd 失敗 ＋ 目錄 fsync 失敗一次」的釋放與收回各一支：重試期間持鎖、最後刪除 durable 並放鎖。反向驗證：兩道都拿掉 → 新測試失敗（狀態一直是「sentinel 已刪、鎖持有」，正是 review 描述的卡住）；⚠️ 第一次注回時那支測試**卡住而⛔ 失敗**（`release()` 依設計忽略 TERM，`timeout` 也停不了它）——把觀察用的 `sleep` 替身改成重試超過 10 次就失敗之後，注回才是乾淨的紅 |
| 低 | ⛔ 驗證紀錄與 staged 版本不符（host unittest 24 項、整合測試全部通過） | ✅ 重跑之後更新（下方「驗證」與「檔案」兩表） |

**⑦b 實作第一輪 review 的修正（2026-10-01）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | ⛔ **sentinel 刪除後的目錄 fsync 失敗時仍會放鎖**：`remove_sentinel()` 先 unlink 再 fsync，fsync 的例外一路離開 `release()`／`rollback()`，由 `main()` 的 finally 關掉 lock fd——結果是「沒有 sentinel、鎖也放了」，違反「刪 sentinel、fsync 目錄 → 放鎖」與 G3 | ✅ 先寫測試重現（4 支全部以例外失敗）再修：`remove_sentinel()` 只負責 inode 比對 ＋ unlink；新增 `finish_removal()`——目錄 fsync 成功才算刪掉，失敗就**持鎖**、每 30 秒重試並回報；`release()` 把清理途中的**任何**例外都當成「清不空」（持鎖、留 sentinel、重試，⛔ 讓例外離開）；`rollback()` 同樣先 `finish_removal()` 才放鎖；`main()` 的 finally 在 `removal_pending` 時先把 fsync 做完；等待 child 途中的非預期例外也照樣走正常釋放。host unittest 補四支（釋放與收回的「刪除後 fsync 失敗」、清理途中的任意 OSError、經 `main()` 的整條路徑），斷言 supervisor ⛔ 結束、重試期間鎖⛔ 可取得 |
| 中 | ⛔ **resume／promote 先執行複本的 orchestrator，才由它自己驗自己**（循環自證）：持鎖階段只確認檔案存在 | ✅ 持鎖階段（真正 repo 的 HEAD 版）在 **exec 之前**、三種模式都驗：複本 HEAD ＝ 凍結的 `repo_head`，且 `scripts/run-i074-stage2.sh` 的內容 ＝ **真正 repo 的物件**裡 `repo_head` 的 blob；resume／promote 另驗 freeze record 副本（HEAD 版 `check-basic`）的 `repo_head` 與副本 SHA ＝ `state/run.json`、`repo_head` 存在於真正 repo。不符 → run／resume 1、promote **9**（複本完整性，同「八之三」步驟 1；結束碼表加 L2）。測試：被改過的複本 orchestrator（`assume-unchanged` 掩蓋、第 2 行會寫標記檔）在第一次執行、`--resume`、`--promote` 三種情況都⛔ 被執行 |
| 低 | ⛔ preflight 2 的合成或 `check-full` 失敗時留下暫時 worktree | ✅ 改成成功或失敗都移除（移除失敗本身也是 1）；測試：identity 不符讓 `check-full` 失敗 → 1，複本只剩主 worktree、`<work>/tmp` 沒有 `compose.*` |
| 低 | 文件與介面不符：計畫寫 `state-check --kind`（實際是 `--require`）；`development-workflow.md` 寫「環境清理後只帶協定 v1」（實際是清單刪除，`HOME`、`XDG_DATA_HOME`、`REPLAY_IMAGE_ID` 與其他不在清單上的變數照傳） | ✅ 兩處照實改寫；另把上面三項修正同步進計畫「二之四」第 8 列、「二之六」、「二之七」第 2 步、「二之十」（L2）與 `development-workflow.md` |
| ⚠️ 自己發現 | host unittest 加了在本程序呼叫 `release()` 的測試之後，`ForkedPaths` 的 setsid 孤兒測試從 0.5 秒變成 20 秒 | 查證：`release()` 依設計把 TERM／INT／HUP 設成 SIG_IGN，在本程序呼叫它的測試把這個狀態留給之後 fork 的子程序（孤兒繼承「忽略 TERM」，只能等 KILL 升級）。⛔ 不是產品缺陷（正式 supervisor 釋放之後就結束；workload child 由 `preexec_fn` 恢復預設處置）；測試的 setUp／tearDown 改成保存並還原訊號處置 |

反向驗證（修正逐項注回 → 新測試變紅 → 還原）：V1a「釋放時刪掉後直接 fsync」、V1b「收回時刪掉後直接 fsync」、V1c「釋放不攔清理途中的例外」、
V2「持鎖階段 exec 之前不比 blob」、V3「preflight 2 只在成功時移除」——五項都在預期的測試變紅。

**檔案**：

| 檔案 | 內容 |
|---|---|
| `scripts/run-i074-stage2.sh`（新增） | 入口（bash 內建的守門、公開 argv、入口清單的投影 ＝ HEAD、work 目錄規則）、持鎖階段（先複製再驗 freeze record、clone）、複本內 orchestrator（`/proc/locks`、preflight 0～7、replay、輸出形狀、檢查點、依磁碟事實判終態、`--resume`、晉升 stub）、中斷處理 |
| `scripts/lib/i074-stage2-supervisor.py`（新增） | 信任根（初始 user namespace、四個系統程式與上層目錄）、自身 ＝ HEAD、環境、uid、鎖、sentinel（封閉 schema、inode 比對）、image labels、殘留容器、subreaper、parent-death TERM、正常釋放（容器 → TERM／KILL → 確認 → inode → 刪 → 放鎖）、收回、清不空時持續回報 |
| `scripts/lib/i074-stage2-docker-label-shim.sh`（新增） | 唯一的 label 注入點與拒絕規則、信任條件、與 sizing shim 互斥 |
| `python/scripts/i074_stage2_preflight.py`（新增） | 常數（唯一定義）、`disk`、`anchors`（容器內）、五份 state 的封閉 schema／SHA 鏈／交叉條件、`replay_argv()`（replay argv 唯一的組裝處） |
| `python/scripts/i074_stage2_freeze_record.py`（新增） | `build`／`check-pair`／`check-basic`／`check-full`；canonical JSON 經 `_i074_bootstrap` 取自 `replay_bundle/canonical.py` |
| `scripts/i074-stage2-sizing.sh`、`python/scripts/i074_stage2_sizing.py`、`scripts/lib/i074-sizing-docker-shim.sh` | 真實 tooling、量測窗口之外合成一次並斷言 raw ＝ canonical、patched worktree 每次都必須得到同一個合成結果、fixture 的 `--composed-sha256`、`meta` 新欄、freeze record 寫入、`--formal` 清單、與 label shim 互斥 |
| 測試 | `scripts/test-i074-stage2.sh`（新增，184 項）、`scripts/tests/test_i074_stage2_host.py`（新增，29 項）、`test_i074_stage2_preflight.py`、`test_i074_stage2_freeze_record.py`（新增，合計 74 項）、`test_i074_stage2_sizing.py`、`test-replay-args.sh`（sizing 的 `--formal` 清單兩支）、`python/scripts/test.sh`（在 `SKIP_SHELL_TESTS` 區段呼叫新腳本） |
| 文件 | `development-workflow.md`：新增「I-074 Stage 2 的正式執行程序」、sizing 一節補 ⑦b 的改動、腳本層測試表加一列、⑦a 的 tooling patch 一節標成 review 通過 |

**驗證**：

| 項目 | 結果 |
|---|---|
| 「六」第 0 步（實作前） | ✅ `dev`（uid 1001）、`/proc/self/uid_map` ＝ `0 0 4294967295`；`/usr/bin/git`、`/usr/bin/docker`、`/usr/bin/python3.9`、`/bin/bash`、`/usr`、`/usr/bin`、`/bin`、`/` 的 owner uid 都是 0、⛔ group／other 可寫；`/usr/bin/bash` 不存在 |
| `python/scripts/test.sh` 完整執行（依序） | ✅（⚠️ 2026-10-01 實作第二輪 review 之後重跑，以這一次為準）398 秒；doc-refs 0 問題；`test-replay-args.sh` 全部通過（含 tooling patch 漂移測試）；`test-i074-stage2.sh` 184 項全部通過（含 host unittest 29 項）；pytest **1718 passed、1 skipped**（⑦a 為 1644 ＋ 本包新增 74） |
| `anchors` 以真實資料實跑 | ✅ Stage 2 image（`sha256:2a90ad1c…`）、`--network none --read-only`、`/app:ro`、目前 XDG 的 identity：約 23 秒、rc 0，輸出 `bundle_id` ＝ `b1_20260901_1d_74350966_5d7ecb10`、`after_base_commit` ＝ `e1cbbbd…`；該 image 的 `Config.Labels` 只有 compose 的三個鍵，⛔ 沒有 `i074.stage2.run` |
| sizing **validation 模式**實跑（「六」第 3 步） | ✅ `status: ok`、**`P_B` ＝ 159,784,960 bytes（152.4 MiB）**（⑤ 的空 tooling 為 150.8 MiB；預算 160 MiB，餘 7.6 MiB）；三條路徑 132.7／152.4／67.9 MiB；十個步驟的結束碼全部符合預期（finalize 以真實 tooling 的合成 SHA 回 0、publish 1、check 2、recover 1）；兩份 patch 的 raw ＝ canonical（`fa7f5ba8…`、`353c69a0…`）、合成 `16781839…`、語意 `5b851e1e…`；記憶體峰值最高 399.6 MiB（success 的 fixture），證據層程序最高 374.1 MiB；沒有殘留容器；validation 模式⛔ 沒有寫 freeze record。⚠️ 第一次實跑在 witness 的自我檢查被擋下（rc 1、⛔ 不宣稱 `P_B`）——原因是我在量測窗口中編輯了真正 repo 的 `docs/development-workflow.md`，harness 正確地 fail-closed；刪掉該執行目錄、量測期間不動 repo 之後重跑即通過（⚠️ 這是開發觀察值，⛔ 不是 ⑨-2） |
| 隔離 | ✅ 測試前後 `/run/lock` 只有 `subsys`（⛔ 沒有任何 `i074-stage2.*`）；真正 repo 的 worktree 登記數不變（245，I-118 的既有洩漏）；fake docker ⛔ 沒有收到任何帶正式鍵 `i074.stage2.run=` 的參數 |

**反向驗證**（逐項注回、跑對應測試、確認變紅且紅在預期的那一項、逐位元還原）：

| # | 注回的缺陷 | 結果 |
|---|---|---|
| R1 | `/proc/locks` 驗證拿掉 ppid 鏈 | ✅ 紅：「協定變數完全正確、supervisor 活著且持鎖，但呼叫者不是它的後代」 |
| R2 | shim 只擋 `-H`／`--host` 的全域選項 | ✅ 紅：「全域選項在任何子指令之前（`--config ps`）」 |
| R3 | 拿掉 tooling 非空檢查 | ✅ 紅：ba |
| R4 | 改成看結束碼判終態 | ✅ 紅：publish 回 1 但 record 已在磁碟、finalize 回 3 但沒有終態 |
| R5 | 檢查點跳過 preflight 0 | ✅ 紅：ax 的後半、n8 的 `<work>/bin` 兩支 |
| R6 | `anchors` 改傳 `None` 當 identity | ✅ 紅：spy 與 ai（pytest） |
| R7 | supervisor 不清 `TOOLING_PATCH` | ✅ 紅：「finalizer 與 check ⛔ 收不到兩份 patch 的變數」與 unittest |
| R8 | sentinel 不比 inode 就 unlink | ✅ 紅：unittest 的收回與正常釋放 |
| R9 | 後代還在就放鎖 | ✅ 紅：setsid 的孤兒 |
| R10 | freeze record 在 validation 模式也寫 | ✅ 紅：n10（pytest） |
| R11 | supervisor 改回從呼叫者的 PATH 取 docker | ⚠️ 第一次**沒有變紅**——入口已把 PATH 固定，呼叫者的 PATH 根本到不了 supervisor，那一層沒有被單獨測到。✅ 補一支「繞過入口、以呼叫者的 PATH 直接啟動 supervisor」之後重驗變紅 |
| R12 | `--resume` 不比 identity | ✅ 紅：resume 與第一次執行的「只改 `created_at`」兩支 |
| R13 | `state-check` 不驗鏈 | ✅ 紅：pytest 的鏈 |
| R14 | replay 之後加回「失敗只記錄」的清理 | ⚠️ 第一次**沒有變紅**——fake runner 不留 worktree，清理沒有東西可刪、就不會出現 `worktree remove`。✅ fake runner 改成像真正的 runner 一樣在 `<work>/tmp` 留下 worktree，並斷言它被保留；重驗變紅 |
| R15 | 拿掉 `$-` 含 `p` 的檢查 | ✅ 紅：`bash <script>` |
| R16 | 入口不清 `LD_*`／`PYTHON*` 就 exec | ✅ 紅：「LD_PRELOAD 只影響第一個程序」 |
| R17 | 拿掉入口的 `REPLAY_IMAGE_ID` 格式檢查 | ✅ 紅：「入口就擋下」的各支（supervisor 那一層另由 unittest 測） |

**與計畫的差異**：

| # | 差異 | 理由 |
|---|---|---|
| 1 | supervisor 的 `main()` 在**每一條回傳路徑**都明確關閉 lock fd（收回在 unlink ＋ fsync 目錄之後才放鎖） | 計畫的「③…再放鎖」原本靠程序結束隱含達成；host unittest 在同一個程序裡重跑時抓到鎖沒放 |
| 2 | 信任條件：檔案 owner 接受「信任 uid **或 root**」；上層目錄另接受「root 擁有且設了 sticky bit」的共用目錄 | 正式環境兩者都是 0、`/usr/bin` 的上層沒有 sticky 目錄，行為不變；測試把信任 uid 改成 `dev` 時，系統的 `/usr/bin/git` 仍是 root 擁有，測試目錄也在 `/tmp` 底下 |
| 3 | 持鎖階段 exec 之前先驗複本裡確實有 `scripts/run-i074-stage2.sh` | 複本 HEAD 被移到沒有它的 commit 時原本會以 bash 的 127 結束；改成照結束碼表 fail-closed（1／8） |
| 4 | 入口另清 `GIT_*`；supervisor 另清 `SHELLOPTS`、`BASHOPTS`、`CDPATH`、`GLOBIGNORE` | 入口在 supervisor 之前就要用 git 驗入口清單；下游 `#!/usr/bin/env bash` 的腳本⛔ 是 `-p` |
| 5 | sizing：在量測窗口之外先合成一次、記下結果，之後每個 patched worktree 都必須得到同一個合成結果 | 計畫寫「第一次呼叫時斷言」，但步驟 0 的 worktree 在 command substitution 裡建立、全域變數傳不回來 |
| 6 | `--promote`（⑦b）在 stub 之前另跑一次 `state-check --require run` | 檢查點的同一個函式；⛔ 改變結束碼（任一不符 → 9） |
| 7 | state 的寫入分成 `state-write-run`／`-preflight`／`-replay-started`／`-replay-done`／`-attempt` 五個子指令；freeze record 另加 `check-pair` | 值由 Python 端自己計算與驗證（⛔ 在 shell 組 JSON）；`check-pair` 就是計畫「複製到 `<work>` 之後對兩個副本再驗一次」 |
| 8 | 測試另補：supervisor 那一層的 PATH（R11）、fake runner 留下 worktree（R14）、入口那一層的 `REPLAY_IMAGE_ID` 與 `LD_PRELOAD`（R16、R17）的專屬斷言 | 反向驗證發現原本的斷言被另一道防線遮住 |
| 9 | ⚠️ 附帶紀錄（⛔ 不在 ⑦b 處理）：計畫第二輪 review 記下的「八之三」`git worktree prune` 前提問題，⑦c 的細部計畫要處理 | 同上一輪紀錄 |

**review 之後**（commit 由使用者決定）：commit 後再跑一次 `scripts/make-i074-tooling-patch.sh --verify "$(git rev-parse 'HEAD^{tree}')"`
（⑦b 沒有動 tooling 路徑，預期仍通過）——✅ **2026-10-01 已執行**（commit `ce17ab2` 之後）：`HEAD^{tree}` ＝ `a817ec516ec559916d746a2b9df5b86a3671e3cf`
（＝ stage 時的 tree），`--verify` 通過（tooling patch 仍是 `353c69a0…`），`evaluation.py`（`6eefa111…`）與 `replay_bundle/`（`ae99cd3d…`）的 OID 與 ⑦a 相同；「六」第 6 步的 `--formal` sizing（約 6 分鐘，確認寫出的 freeze record 通過 `check-basic`）
在使用者同意時才跑——⚠️ 它只是開發驗證，⑩ 用的 freeze record 一律來自 ⑨-2。

**歸檔**（⚠️ 依 CLAUDE.md，本筆的計畫與結果保留到 review 確認後才收斂）：操作程序寫進
[`development-workflow.md`](./development-workflow.md) 新增的「I-074 Stage 2 的正式執行程序」，sizing 一節補 ⑦b 的改動，腳本層測試表加一列。

#### Stage 2 步驟 ⑦c 細部計畫 v1（2026-10-01，✅ **已確認**（2026-10-01，review 七輪後通過並 commit `4b3aa10`））

⚠️ 依「Stage 2 步驟 ⑦ 總綱 v1」（✅ 2026-09-30 確認）拆出的第三包「`--promote`＋B／C 判讀器」。範圍、驗收 id 與已裁定的機制以總綱「三」「四」、
v29「八之三」與「六、9」為準；本細部計畫只補**程式設計、呼叫順序、介面、結束碼與測試落點**，以及總綱沒寫到、實作時必須決定的細節（列在「三」，⚠️ 待確認）。
⚠️ ⑦c **會動 tooling 路徑**（`replay_bundle/` 的 `publish.py`、`stage2_archive.py` 與新增的 `stage2_verdict.py`），所以照總綱「二」的版控流程重產 tooling patch。

**現況（2026-10-01 實查）**：

- ⑦b 已 commit（`ce17ab2`）：晉升是固定回 9 的 stub；supervisor 與 orchestrator 各自**鏡像** 1／2／8／9 的字面值，`replay_bundle/publish.py`
  只有 1、3、4、5、6、7（總綱把 8、9 留給 ⑦c）。
- ③ 的完整 verifier：`verify_stage2_graph()`（十六道、封閉檔案集合、每個成員重算）——⛔ **沒有**驗 `finalizer_provenance.base_commit` 的
  信任根綁定（「八之三」第九輪已指出）；`verify_failed_record(…, dir_name=)` 已支援以**另一個名稱**驗 F4 的 Python 層。兩者都以
  `_load_trust_anchors(python_root, …)` 從某個 python root 取錨點。
- `finalize-stage2-evidence.sh` 的七種模式全部從**自己 repo 內的常數路徑**取證據，⛔ 沒有能驗「真正 repo 裡某個 staging」的入口——
  「八之三」的「驗證用的錨點」列預告了要補一個唯讀入口。
- `replay_bundle/publish.py` 是 dependency-light（`_i074_bootstrap` 的模組清單內），host 可以直接載入它的 `rename_noreplace()`
  （`renameat2(RENAME_NOREPLACE)`）、`fsync_dir()`、`fsync_file()`、`remove_tree()`。③ 的 writer 以 `.<目的地名>.staging-<16 hex>` 建
  staging、`probe_no_clobber()` 以 `.probe-<16 hex>-{a,b}-{src,dst}` 建 probe——這兩種是「可辨識的 ③ orphan」。
- comparison 的每一列是 `{symbol, timeframe, as_of, differences, before, after}`（`compare_rows()` 全欄位比較）。
- 真正 repo 的 `python/baselines/i074_stage2/` 目前只有兩份 patch、`envcheck/` 與三個 log；⛔ 沒有 `evidence/`、⛔ 沒有 `failed/`。
- ⑦b 第二輪 review 記下：「八之三」寫的「崩潰留下的 worktree 登記由下一次 `--promote` 的 `git worktree prune` 清掉」前提不成立
  （prune 只清實體目錄已消失的登記），留給本包處理。

**執行步驟**：

- **步驟 A（本次，只動文件）**：本細部計畫寫進本筆；⑦b 實作結果、狀態列與 v29「八」的標記改成 ✅。⛔ 不改程式，停下等 review／commit。
- **步驟 B（本計畫 review 通過並 commit 之後）**：依「二」實作 → 「六」驗證 → 歸檔 → 依總綱「二」的版控流程 stage（程式 → `git write-tree`
  → 產生 tooling patch → stage patch）→ ⛔ 不 commit，停下等 review。

**⑦c 細部計畫 v1 第七輪 review 的修正（2026-10-01）**：

| # | 問題 | 修正 |
|---|---|---|
| 中低 | ⛔ 第 5b 步失敗時宣稱「6a ⛔ 不 fsync」，但第 4 步（來源的 durability）在第 5b 步**之前**就已完成——recovery 測試若以 spy 斷言整趟零 fsync，必然與流程衝突；第六輪的修正表、第 5b 步、「五」的測試列都有同樣的敘述 | ✅ 統一改成：第 4 步的來源 fsync 照常執行；第 5b 步失敗之後，6a ⛔ 不呼叫驗證模式、⛔ 不 fsync **目的地**；測試的 spy 以 `fstat` 的 (`st_dev`, `st_ino`) 對照來源與目的地的 inventory，分辨被 fsync 的 fd 屬於哪一邊，斷言**目的地**沒有被 fsync（⛔ 不斷言整趟零 fsync）。⚠️ 自己再掃全節的「⛔ 不 fsync」：第四輪的 6a 收尾重算 barrier 有同一個問題，一併改；n11 的 3b（第 4 步之前就失敗）整趟確實沒有 fsync，改成明寫理由 |

**⑦c 細部計畫 v1 第六輪 review 的修正（2026-10-01）**：

| # | 問題 | 修正 |
|---|---|---|
| 中 | ⛔ **ignore 守門只在 6b，6a 的既有目的地仍可能被忽略**：6b 已 rename、parent fsync 失敗回 3 → 兩次 `--promote` 之間 `.gitignore` 被改成過廣規則 → 重跑走 6a，驗證與 fsync 都成功、回 0／6，目的地卻被 git 忽略、進不了版控；文件卻宣稱啟動前失效的規則都會被擋下 | ✅ 守門拆兩組：**目的地那組**（成員路徑全部不被忽略）提升為第 **5b** 步、在 6a／6b 分支**之前**共用；**staging 那組**（全部被忽略）維持 6b 專屬、建 staging 之前。⚠️ 目的地那組同樣用 `--no-index`：目的地已被追蹤時，過廣規則照樣回 9——刻意從嚴（過廣規則會讓之後的 failed record 進不了版控，交人工修規則）。操作契約③、決策 #21、風險表同步。補 recovery 測試：pytest 走完整序列（6b rename 之後注入 parent fsync 失敗 → 3 → 規則變成過廣 → 重跑走 6a → 9、⛔ 不呼叫驗證模式、⛔ 不 fsync 目的地（第 4 步的來源 fsync 照常；⚠️ 第七輪 review 訂正）→ 規則修好 → 0／6）；host 真 git 以「目的地已與來源逐位元相同」的 recovery 狀態做同一件事 |

**⑦c 細部計畫 v1 第五輪 review 的修正（2026-10-01）**：

| # | 問題 | 修正 |
|---|---|---|
| 中 | ⛔ **staging 的 ignore 規則只有靜態測試、沒有執行期守門**：入口清單（⑦b「二之六」）⛔ 不含 `.gitignore`，⑩ 開始之前就刪掉或改壞規則照樣能進入晉升；操作契約只禁止「執行期間」修改；6b 直接建 staging 並假設它會被忽略 | ✅ 6b 在建 staging **之前**加 **ignore 守門**（⚠️ 第六輪 review：目的地那組提升為 6a／6b 共用的第 5b 步）：以受信任的 `git check-ignore --no-index -z --stdin` 查兩組 probe——staging 之下每個成員的路徑必須**全部**被忽略、目的地之下同一組路徑必須**全部不**被忽略；不符、git 失敗、逾時 → **9** 且⛔ 不建 staging。⚠️ 先以 git 2.30.2 實測決定兩個細節：①⛔ 不加 `-v`——`-v` 連否定規則（`!.promote-staging-*`）的命中也回 0；②probe 用**成員路徑**而⛔ 不只用 staging 目錄本身——只對目錄生效的規則（`….promote-staging-*/`）對還不存在的目錄查不到，對成員路徑才查得到（`git add -A` 看的也是成員）。補測試：啟動前刪除規則、改成過廣規則（兩種）、子目錄的否定規則 → 9 且沒有 staging；只對目錄生效的規則 → 照常晉升（對照組）；git 回 128、逾時、輸出集合不符 → 9。⚠️ 測試 image ⛔ 沒有 git，真 git 的案例放在 host 的 `scripts/test-i074-stage2.sh` |
| 低 | ⛔ dirfd 的「非負整數」驗證若用 `isinstance(fd, int)`，`True`／`False` 會被當成 fd 1／0 | ✅ 契約明寫 `type(fd) is int and fd >= 0`；測試補兩個 fd 參數各自傳 `True`／`False` → 在 syscall 之前拋 `ValueError` |

**⑦c 細部計畫 v1 第四輪 review 的修正（2026-10-01）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | ⛔ **操作契約仍允許破壞 active staging 與目的地**：契約只禁止搬移、刪除、替換目錄，並明說目錄內其他檔案可以修改——外部程序在驗證通過之後、rename 之前**原地改寫** staging 的成員時，inode 與父目錄鏈都不變，鏈檢查照樣通過，晉升回 0／6 但目的地已不是剛驗過的內容；6a 的既有目的地有同一個窗口。另外契約字面上也禁止了晉升自己對 `.promote-staging-*` 的清理與 rename | ✅ 契約重寫（「二之三」）：**對象**是晉升程序以外的所有程序——晉升依流程建立、寫入、清理與 rename staging 是唯一例外；**禁止**：①搬移、刪除或替換那幾個目錄本身；②修改、刪除、搬移或替換 active staging 與本次 target 目的地的**任何成員**；③移除或改寫 `.gitignore` 的 staging 規則、以 `git add -f` 把 staging 加進 index；其他一般編輯、`git add -A` 與 commit 照常。⚠️ 另加偵測（契約之外）：**收尾重算**——6b 在 rename 之前、6a 在驗證之後，經持有的 fd 重算每檔 SHA，與比對時的值不同 → **9**（6b 不發布、清掉 staging），把「原地改寫」從「偵測不到」變成 9；重算之後的修改照實寫成晉升之後的編輯。補 barrier 測試（6b、6a 各一支）與 `.gitignore` 規則的 `git check-ignore` 測試 |
| 中 | ⛔ **`rename_noreplace_at()` 的名稱規則與舊 wrapper 衝突**：計畫寫「`_at()` 只接受單一 component」又寫「舊的 `rename_noreplace()` 改成以 `AT_FDCWD` 呼叫它」，但五個正式呼叫點（`bundle.py`、`evidence.py`、`run_identity.py`、`stage2_evidence.py`、`publish.py` 的 probe）都傳多層路徑——舊 API 會壞掉 | ✅ 查證屬實。拆三層：私有 `_renameat2(src_dir_fd, src_path, dst_dir_fd, dst_path)` 只負責 syscall 與 errno 對應；`rename_noreplace_at()` 驗兩個名稱都是單一 component、兩個 fd 都是非負整數之後呼叫它；`rename_noreplace()` 以 `AT_FDCWD` 與既有的完整路徑**直接**呼叫它。⚠️ 附帶：`ctypes.c_char_p` 會在 NUL 截斷路徑（2026-10-01 實測：`a\x00ignored` 會把 `a` rename 掉），私有函式對含 NUL 的路徑拋 `ValueError`——這是舊 API 唯一的行為差異，只影響本來就會被誤處理的輸入。測試保留既有的絕對／多層路徑案例，另驗 `_at()` 拒絕 `/`、`.`、`..`、空字串、NUL、負數 fd（且⛔ 不呼叫 syscall） |

**⑦c 細部計畫 v1 第三輪 review 的修正（2026-10-01）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | ⛔ **「不會寫到樹外」的宣稱不成立**：父目錄在持有 fd 之後被搬走（例如 `mv failed <repo 外> && ln -s <marker> failed`）時，rename ⛔ 不跟隨新的 symlink，但會寫進持有的 fd 指向的舊目錄——它被搬到 repo 外，record 就落在 repo 外；事後的鏈檢查只能回 9，⛔ 無法撤銷已發生的寫入 | ✅ 改成照實的邊界：fd-anchored 保證的是「⛔ 不跟隨替換後的 symlink」；已持有的父目錄被搬走時，record 可能寫進該 inode 的新位置（可能在 repo 外），事後偵測 → **9**。新增**操作契約**（「二之三」）：⑩ 的任何一趟（完整流程、`--resume`、`--promote`）執行期間，⛔ 不得搬移、刪除或替換真正 repo 的 `python/`、`python/baselines/`、`i074_stage2/`、`failed/` 與 `.promote-staging-*`；其他編輯與 commit 照常——同步「二之六」（v29「八之一」的「真正 repo」列）、決策 #15 與新增的 #18、風險表、「八」的歸檔。barrier 測試拆開斷言：symlink 指向的 marker 完全不變；被搬走的舊目錄（repo 內、repo 外各一支）照實斷言會收到 record；`rm -rf` 之後換成 symlink 的那一支 rename 失敗、staging 已清 |
| 中 | ⛔ 決策 #12（「建立失敗 → 8」）與 n11（「複製 I/O 錯誤 → 8」、`rename_noreplace`）仍是 v1 的規則，與第二輪的分類相反 | ✅ 兩處改成依「結束碼的分類」：目的端的建立與寫入只有 `ENOSPC`／`EDQUOT`／`EIO` → 8；來源錯誤、完整性漂移與其他 errno → 9；名稱改成 `rename_noreplace_at()`（風險表同步） |
| 低 | ⛔ 修正摘要寫「十個」reason code，表中實際是十一個；`ROW_SHAPE_INVALID` 只寫「缺下列任一欄位」 | ✅ 改成十一個；`ROW_SHAPE_INVALID` 直接列出 before／after 的必備欄位與型別（`null` 也算不符） |

**⑦c 細部計畫 v1 第二輪 review 的修正（2026-10-01）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | ⛔ **`O_NOFOLLOW` 只保護最後一層**：第 4 步與 6b 用完整路徑；⑩ 期間真正 repo 明定可以照常編輯（v29「八之一」），`failed/` 若被換成指向樹外的 symlink，`rename_noreplace(staging, failed/<name>)` 會跟隨中間層，把正式 record 寫到 repo 外 | ✅ 先重現（2026-10-01，scratchpad）：`failed/` 換成指向樹外的 symlink 之後，完整路徑的 `O_CREAT \| O_NOFOLLOW` 與 `renameat2` 都落在樹外；對照組（逐層持有 fd 之後才被換掉）symlink 指向的目錄不變、record 落在被搬走的原目錄、rename 之後重開得到 `ELOOP`。修正：晉升的所有走訪與寫入改成 **fd-anchored**（「二之三」的「路徑錨定」）——真正 repo 根與複本根各開一次 fd，之下逐層 `openat(…, O_DIRECTORY \| O_NOFOLLOW)` 並持有；stat、mkdir、開檔、複製、刪除、`statvfs`、rename 全部用 `dir_fd`；`publish.py` 新增 `rename_noreplace_at()`（來源與目的各帶 dirfd 與單一名稱）；`failed/` 開一次、之後只用它的 fd；**rename 前後各做一次鏈檢查**（從根路徑重走，每層的裝置與 inode ＝ 持有的 fd；rename 之後目的地名稱必須解析到 staging 的 inode）；一般檔案另要求 `st_nlink` ＝ 1。⚠️ 驗證模式（docker 唯讀掛載）只能吃路徑，改以「輸出的 `manifest_sha256`／`record_sha256` ＝ 本程序經 fd 寫入或讀到的那份 bytes 的 SHA」綁定（封閉 layout ＋ 逐項 SHA 讓兩者等價）。⚠️ 照實的界線寫在「二之三」：rename 只能以名稱指定來源、父目錄被搬走時 record 落在原目錄（→ 9，交人工）、repo 根以上不在保證內。測試補 `failed/` 與中間層指向樹外、驗證之後 rename 之前替換 parent 的 barrier、hardlink——symlink 指向的 marker 與 inode 都完全不變（⚠️ 第三輪 review 訂正：被搬走的舊目錄可能收到 record，「不會寫到樹外」撤回） |
| 中 | ⛔ **漂移與 durability 失敗的結束碼沒拆開**：第 4 步把所有 `OSError` 映成 3，但 inventory 之後被刪、換成 symlink、替換 parent 會回 `ENOENT`／`ELOOP`／`ENOTDIR`——那是完整性漂移，⛔ 不是 durability 未確認 | ✅ 「二之三」新增結束碼分類：inventory 之後任何 open／`fstat` 錯誤或不符、鏈檢查不符、SHA 綁定不符 → **9**；只有在已核對的 fd 上 `fsync()` 本身失敗 → **3**；**8** 只留給目的端的容量與寫入 I/O（空間預檢、在已核對的 staging fd 上的 `ENOSPC`／`EDQUOT`／`EIO`、rename 的 `EEXIST`／`ENOTEMPTY`）；6b 的來源側錯誤 → 9。⚠️ 附帶收緊：rename 的 `EXDEV`／`EINVAL`／`ENOSYS` 改成 9（v1 籠統寫「`rename_noreplace` 失敗 → 8」，但重跑解決不了它們）。每一類各補測試 |
| 低 | ⛔ argv fixture 仍寫「兩組」 | ✅ 改成三組：evidence 驗證、failed 驗證、evidence ＋ `--judge` |
| 低 | ⛔ 判讀輸出沒有封閉 schema（`c_rows` 是數量還是陣列、逐列欄位、`class` 值域、reason code、排序都沒定） | ✅ 「二之四」釘死完整 key set、型別、列的順序（＝ comparison 已驗的排序）、`class` 值域與十一個穩定的 reason code（⚠️ 第三輪 review 訂正：原本寫十個）；`c_rows` 改名 `c_row_count`；0 列 → 拒絕判讀；驗證模式輸出的 JSON 也一併封閉（「二之二」） |

**⑦c 細部計畫 v1 第一輪 review 的修正（2026-10-01）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | ⛔ **驗證與判讀之間有 TOCTOU**：步驟 3 完整驗證、步驟 4 只重驗 manifest、base 與 comparison 的 SHA——兩步之間改的若是 comparison 以外的成員，判讀仍會成功，違反 m 的「任一成員 bytes 被改都拒絕」，決策 #9 的「綁定到剛驗過的那一份」也不成立（容器的唯讀掛載擋不住 host 同時修改來源） | ✅ 改成**驗證與判讀在同一個 Python 程序、用同一份讀進記憶體的資料**：驗證模式加 `--judge`（只限 `--target evidence`），`verify_stage2_graph()` 本來就「每個成員只讀一次」，它回傳的 `Stage2Verified` 加上已驗過的 comparison 物件，判讀直接吃這個物件——驗與判之間⛔ 沒有第二次讀取。⚠️ ⛔ 不採 review 建議的私有 snapshot：snapshot 之後仍是「先驗、再由另一個程序讀」，同一帳號的程序照樣能在兩步之間改它；同程序判讀連那個窗口都沒有。殘餘的跨程序交接只剩 shell 合成守門 → Python 段，由既有的 `check_verified_composition()`（Python 以自己讀到的 patch bytes 比對 shell 驗過的 SHA）與 `verify_stage2_graph()` 的全成員重算綁住。`stage2_verdict.py` 因此只剩純函式（⛔ 沒有自己的 CLI）。測試補：判讀路徑中 comparison 只被讀一次、判讀的物件就是驗證回傳的那一個（spy）；shell 段之後、Python 段之前改掉 comparison 以外的成員 → 拒絕、⛔ 不輸出 B／C |
| 中 | ⛔ **晉升的 staging 會被真正 repo 的 `git add -A` 看見**：6b 的 staging 建在真正工作樹內，`.gitignore` 沒有排除它，而 ⑩ 期間真正 repo 照常編輯與 commit——複製途中執行 `git add -A` 可能把半成品放進 index；n9 只斷言最終 bytes 抓不到 | ✅ `.gitignore` 精準加 `python/baselines/i074_stage2/.promote-staging-*`（正式目的地 `evidence/`、`failed/` ⛔ 不 ignore——發布之後要進版控）；補 barrier 測試：staging 已建好、rename 之前（fake 驗證模式那一刻）在真正 repo 執行 `git add -A`、`git status`，staging ⛔ 不進入 index、⛔ 不出現在未追蹤清單。⚠️ ⑦b 的 preflight 以 `--ignored` 檢查真正 repo 的 `i074_stage2/`，所以留下的 orphan staging 照樣擋住下一次 ⑩，直到 `--promote` 的第 2 步清掉 |
| 中 | ⛔ **第 4 步在完整驗證之前就遞迴 fsync 來源，而且會跟隨 symlink**：既有的 `_fsync_tree()` 用 `Path.is_file()`／`is_dir()`；複本的終態若被換成指向樹外的 symlink，會在 verifier 拒絕之前就開啟並 fsync 樹外的檔案 | ✅ 新增「**封閉 inventory**」（第 3 步之後、identity 的綁定與第 4 步之前——讀 identity 也只走它）：以 `os.scandir(…)`／`os.lstat()` 逐層列舉、⛔ 不跟隨 symlink，**只接受一般檔案與目錄**——symlink、FIFO、socket、device 一律 **9**，且⛔ 不開啟、⛔ 不 fsync 它的目標；fsync 以 `O_RDONLY \| O_NOFOLLOW`（目錄另加 `O_DIRECTORY`）開啟，`fstat` 必須與 inventory 的裝置、inode、型別相同。6b 的複製與 6a 的比對都用同一套 inventory（目的地同樣規則）。⛔ 不重用 `_fsync_tree()`。測試補 symlink 指向樹外（樹外的檔案⛔ 不被開啟——spy）、FIFO、socket、目的地含 symlink。⚠️ 第二輪 review 改成 fd-anchored（完整路徑的 `O_NOFOLLOW` 只保護最後一層） |
| 中低 | ⛔ **判讀矩陣把兩個欄位混寫**：「`action_state` `TESTING`：…、`CONFIRMED`：…」——`TESTING`／`CONFIRMED` 是 `lifecycle_phase`；另有「⛔ 可達 → 拒絕」應為「不可達」 | ✅ 改成依 `before.lifecycle_phase` 分寫兩側的 `action_state`；AVOID 類同樣分寫。⚠️ 自己再掃全節：還有約二十處「⛔ ＋ 動詞」省略了「不」（例如「⛔ 覆寫」「⛔ 多數決」「⛔ 用 `git archive`」「判讀器⛔ 取鎖」），照字面會讀成相反的意思——全部補成「⛔ 不…」 |

##### 一、目標與⛔ 不做

| 項目 | 內容 |
|---|---|
| 目標 | 總綱「三」⑦c 列：晉升（「八之三」的七步判定順序、8／9、信任根綁定）、`finalize-stage2-evidence.sh` 的唯讀模式 `--verify-promotion-staging`（同步 ③「七之四」的 CLI matrix 與 `stage2_finalizer_argv.json`）、把 stub 換成真正的晉升並完成端到端結束碼、`replay_bundle/stage2_verdict.py` ＋ `scripts/judge-i074-stage2.sh`；`publish.py` 加 8、9 並由測試斷言 supervisor 與 orchestrator 的鏡像相等（⑦b「三」#2）。驗收：n9、n11、n12（晉升那一層）、「六、9」a～m |
| ⛔ 不做 | memory harness（⑦d）；⑧～⑪；**任何真正的晉升或判讀**（真正 repo 還沒有任何 Stage 2 終態；⑦c 的驗證全部在合成環境）；③ 既有七種模式的行為（只加一種模式） |
| ⚠️ 過渡狀態 | ⑦c commit 之後，⑩ 的程式端到端完整（入口 → … → 晉升 → 判讀）；還差 ⑦d、⑧ 的稽核與 ⑨ 的封存。tooling patch 隨本包重產（⑨ 之前還會再變） |

##### 二、設計

###### 二之一、檔案與角色

| 檔案 | 改動 |
|---|---|
| `replay_bundle/publish.py` | 加 `EXIT_PROMOTION_FAILED = 8`、`EXIT_PROMOTION_BLOCKED = 9`（唯一定義；supervisor、`run-i074-stage2.sh`、晉升模組的值由測試斷言相等）；⚠️ 第二輪 review 新增、第四輪拆成三層：私有 `_renameat2(src_dir_fd, src_path, dst_dir_fd, dst_path)`（syscall 與 errno 對應，原本在 `rename_noreplace()` 內；路徑含 NUL → `ValueError`）；`rename_noreplace_at(src_dir_fd, src_name, dst_dir_fd, dst_name)`（兩個名稱都必須是單一 component、兩個 fd 都必須 `type(fd) is int and fd >= 0`——`bool` ⛔ 不算，否則 `True`／`False` 會被當成 fd 1／0（第五輪 review）——才呼叫私有函式）；既有的 `rename_noreplace(src, dst)` 以 `AT_FDCWD` 與完整路徑**直接**呼叫私有函式（多層路徑照舊；唯一的行為差異是含 NUL 的路徑改成拋錯）。`_load_renameat2()` 回傳的 callable 改成帶兩個 dirfd。⚠️ **實作差異（2026-10-01，見「⑦c 實作結果」的「與計畫的差異」#1）**：`_load_renameat2()` **維持兩參數**（`AT_FDCWD`、完整路徑）——⑩ 的 replay 跑在 `e1cbbbd` ＋ tooling patch 上，`e1cbbbd` 自己的 `test_exdev_is_fail_closed` 以兩參數的 fake 替換它，改成帶 dirfd 會讓那支測試失敗（實測）；dirfd 版改由新的 `_load_renameat2_at()` 提供，「私有 `_renameat2()`」實作成兩個公開函式共用的 `_encode_rename_paths()`（NUL）與 `_raise_for_rename()`（errno 對應），三層的行為不變 |
| `replay_bundle/stage2_archive.py` | 新增 `verify_promotion_target()` 與 CLI 模式 `--verify-promotion-staging`（含 `--judge`）；`Stage2Verified` 加上已驗過的 comparison 物件；`patch_claims()` 新增 `promotion` 模式 |
| `python/scripts/i074-stage2-patch-claims.py` | 新增 `--promotion <path> --target <t>`（evidence 印 4 個 token、failed 印 5 個） |
| `scripts/finalize-stage2-evidence.sh` | 新增唯讀模式 `--verify-promotion-staging <path> --target <t> [--judge]`（「二之二」） |
| `python/scripts/i074_stage2_promote.py`（新增） | 晉升的 host 邏輯（「八之三」步驟 2～7；Python 3.9、標準庫 ＋ `_i074_bootstrap` 載入的 `publish`／`canonical`）；fd-anchored 的走訪、inventory、複製、刪除（`remove_tree_at()`）與鏈檢查（「二之三」的「路徑錨定」） |
| `scripts/run-i074-stage2.sh` | stub 換成晉升；`--promote`／`--resume` 與完整流程都經同一個 `promote()` |
| `replay_bundle/stage2_verdict.py`（新增） | B／C 判讀規則（純函式；⛔ 沒有自己的 CLI——由驗證模式的 `--judge` 在同一個程序呼叫，「二之四」） |
| `.gitignore` | 加 `python/baselines/i074_stage2/.promote-staging-*`（第一輪 review） |
| `scripts/judge-i074-stage2.sh`（新增） | 判讀器的入口（「二之四」） |
| `python/scripts/fixtures/stage2_finalizer_argv.json` | 新增三組：`verify_promotion_staging_evidence_argv`、`verify_promotion_staging_failed_argv`、`verify_promotion_staging_evidence_judge_argv`（第二輪 review 訂正：原本寫兩組） |
| `python/baselines/i074_stage2/tooling_e1cbbbd.patch` | 依版控流程重產 |
| 測試 | 見「五」 |

###### 二之二、唯讀的驗證入口 `--verify-promotion-staging`

| 項目 | 規則 |
|---|---|
| 用法 | `REPLAY_IMAGE_ID=… <某個複本>/scripts/finalize-stage2-evidence.sh --verify-promotion-staging <path> --target <evidence\|failed/<bundle_id>-<語意 SHA>> [--judge]`；⛔ 沒有其他參數、⛔ 不接受 `--run-dir`／`--source-ref`；`--judge` 只限 `--target evidence`（第一輪 review） |
| 真正 repo | 由**本腳本所在 repo 的 `origin`** 推導（`git -C <REPO_ROOT> remote get-url origin` 的 canonical path；⛔ 沒有路徑參數）。⑩ 的複本與判讀器的暫存複本都是 clone 出來的，`origin` 就是真正 repo |
| `path` 的規則 | canonical、⛔ 不得是 symlink、是目錄，而且**只接受兩種**：① staging ＝ `<真正 repo>/python/baselines/i074_stage2/.promote-staging-<16 hex>`（直屬）；② 目的地本身 ＝ `<真正 repo>/python/baselines/i074_stage2/<target>`（6a 與判讀器用）。其他一律回 1（含複本內的路徑） |
| `target` | 封閉：`evidence`，或 `failed/<name>`（`name` 符合 `_FAILED_DIR_RE`） |
| 錨點 | **一律取自本腳本所在的 repo**（⑩：複本、釘在 `repo_head`；判讀器：暫存複本、釘在 `base_commit`）：Stage 1 錨點、`envcheck/`；identity 用 `path` 內封存的那一份（比照 recovery 模式） |
| 掛載 | `path` 與兩個錨點目錄**唯讀**；⛔ 沒有任何可寫的證據目錄；⛔ 不 fsync、⛔ 不寫入 |
| 合成守門（shell 層） | `i074-stage2-patch-claims.py --promotion <path> --target <t>` 取宣告值 → 私有副本 → 新 worktree（本 repo、`TMPDIR`）→ `replay_args_compose()` → 三個增量 SHA 與宣告值逐一相等；failed record 另驗重算的語意 SHA ＝ 宣告值 **＝ `target` 名稱裡的 SHA**（以目的地名稱驗 F4 的 shell 層）。⛔ 不符 → 1、⛔ 不呼叫 Python |
| Python 段 | `verify_promotion_target(python_root, path, target, verified)`：evidence → `_load_trust_anchors(python_root, None)` → `verify_stage2_graph(path, …)` → `check_verified_composition()` → ⚠️ **信任根的綁定**：`finalizer_provenance.base_commit` ＝ 注入的 `--base-commit`（本腳本由 `--source-ref HEAD` 算出，就是本 repo 的 HEAD）；failed → `verify_failed_record(path, …, dir_name=<target 的末段>)`（以目的地名稱驗 F4 的 Python 層）→ `check_verified_composition(semantic=…)`。identity 的 `expected_image_id` ＝ `--image-digest`。⚠️ **`--judge`（第一輪 review）**：驗證通過之後，在**同一個程序**以 `Stage2Verified` 帶回的、`verify_stage2_graph()` 剛驗過的 comparison 物件呼叫 `stage2_verdict.judge_comparison()`——驗與判之間⛔ 沒有第二次讀取 |
| 輸出、結束碼 | **0** ＝ 有效，stdout 只印一行 canonical JSON，⚠️ **封閉 key set**（第二輪 review）：evidence 恰好 `kind`（＝ `"evidence"`）、`target`、`manifest_sha256`、`base_commit`、`identity_sha256`，帶 `--judge` 時再加 `verdict`（「二之四」的封閉 schema）；failed 恰好 `kind`（＝ `"failed_record"`）、`target`、`record_sha256`、`base_commit`、`identity_sha256`；兩個 SHA 都是**該檔案 bytes** 的 SHA-256（晉升據此綁定，「二之三」）；**1** ＝ 無效或用法錯誤（⛔ 沒有 3：它不寫任何東西） |
| 同步 | `STAGE2_MODES`、`build_stage2_parser()`、`run_stage2()`（本模式必須帶四個 `--verified-*`；failed 另帶語意 SHA、evidence 帶了就拒；`--judge` 只接受 evidence）、③「七之四」的 CLI matrix（加註）、argv fixture 三組（evidence、failed、evidence ＋ `--judge`；第二輪 review 訂正）、`s2_normalize` 的 placeholder |

###### 二之三、晉升（`--promote`；「八之三」的判定順序）

`run-i074-stage2.sh` 的 `promote()`：`/proc/locks` → preflight 0（⑦b 既有）→ `state-check --require run,preflight` → 背景執行
`python3 -B <複本>/python/scripts/i074_stage2_promote.py --work-dir <work> --real-repo <I074_STAGE2_REAL_REPO>` → 結束碼原樣成為端到端結束碼
（不是 0／3／6／8／9 → 9）。完整流程（終態已在磁碟）、`--resume`（終態已在磁碟）與 `--promote` 三處都走它。

**路徑錨定（⚠️ 第二輪 review）**：完整路徑的 `O_NOFOLLOW` 只保護最後一層，中間的 `python/`、`baselines/`、`i074_stage2/`、`failed/` 被換成
symlink 時照樣會被跟隨（2026-10-01 實測），而 ⑩ 期間真正 repo 可以照常編輯。所以晉升的每一個檔案系統操作都**錨定在持有的 fd 上**：

| 項目 | 規則 |
|---|---|
| 根 | 真正 repo 根（supervisor 已驗的 canonical 路徑）與複本根，各以 `os.open(<路徑>, O_RDONLY \| O_DIRECTORY \| O_NOFOLLOW \| O_CLOEXEC)` 開**一次**。⚠️ 根以上的路徑⛔ 不在保證內（⑦b 的入口與 supervisor 已驗 canonical 與 owner；⑩ 期間「照常編輯」的範圍是 repo 之內） |
| 逐層 | 根之下 `python`、`baselines`、`i074_stage2`、`failed`、終態目錄、staging 都以 `os.open(<單一名稱>, O_RDONLY \| O_DIRECTORY \| O_NOFOLLOW \| O_CLOEXEC, dir_fd=<上一層>)` 開啟並**持有**；名稱必須是單一 component（含 `/`、等於 `.` 或 `..` → **9**） |
| 操作 | stat（`follow_symlinks=False`）、`mkdir`、開檔、`scandir`、刪除（`remove_tree_at()`：`unlinkat`／`rmdir` 加 `dir_fd`、⛔ 不跟隨 symlink）、`statvfs`、rename（`rename_noreplace_at()`）**全部用 `dir_fd`**；⛔ 不以完整路徑做任何寫入或 fsync |
| inventory | 每一項記相對路徑、型別、`st_dev`／`st_ino`，一般檔案另記 `st_size`；一般檔案的 `st_nlink` 必須 ＝ 1（hardlink 會把讀取與 fsync 帶到樹外的 inode）。之後每次開啟都以 `fstat` 核對 |
| 鏈檢查 | 從根路徑重走一遍：根與每一層的 (`st_dev`, `st_ino`) ＝ 持有的 fd；rename **之前**另驗 staging 名稱解析到 staging fd 的 inode，rename **之後**另驗目的地名稱解析到同一個 inode。不符 → **9** |
| 驗證模式的綁定 | 驗證模式在 docker 內以唯讀掛載執行，只能吃路徑——所以它輸出的 `manifest_sha256`（evidence）／`record_sha256`（failed）必須 ＝ 本程序**經 fd** 寫入（6b）或讀到（6a）的那份 manifest／record bytes 的 SHA。evidence 是封閉 7 檔、manifest 逐項記 SHA；failed record 有 F9 的封閉檔案集合與 F8 的逐項 SHA——「同一份 manifest／record 且驗證通過」因此等於成員逐位元相同。不符 → **9** |
| ⚠️ 照實的界線 | ①Linux 沒有「以 fd 指定 rename 來源」的呼叫，來源只能以名稱指定——rename 前後的鏈檢查把被換掉的情況變成 **9**，⛔ 不能預防。②父目錄在持有 fd 之後被**搬走**（例如 `mv failed <別處> && ln -s <X> failed`）時，rename ⛔ 不跟隨新的 symlink（`X` 不變），但會寫進持有的 fd 指向的那個目錄——它現在在哪裡，record 就落在哪裡；⚠️ **被搬到 repo 外，就是寫到 repo 外**（第三輪 review 訂正）。rename 之後的鏈檢查只能**事後**發現 → **9**、交人工，⛔ 無法撤銷已發生的寫入（2026-10-01 實測：重開得到 `ELOOP`）；所以由下一列的操作契約禁止這類操作。③staging（6b）與既有目的地（6a）的成員在驗證之後被**原地改寫**時，inode 與父目錄鏈都不變，鏈檢查抓不到——由下一列的操作契約禁止，並由**收尾重算**偵測（6b：rename 之前；6a：驗證之後；經持有的 fd 重算每檔 SHA，與比對時的值不同 → **9**，6b 不發布、清掉 staging；⚠️ 第四輪 review）。重算之後到 rename（6b）或結束碼回傳之間、以及晉升之後的修改，⛔ 不在晉升的保證內——判讀器與再跑 `--promote`（6a）都會做完整驗證，commit 前的 review 也看得到 |
| ⚠️ 操作契約（第三輪 review；第四輪重寫） | **對象**：晉升程序（`i074_stage2_promote.py`）以外的**所有程序**——含操作者自己的 shell、編輯器、IDE 與 git；晉升依「二之三」建立、寫入、清理與 rename staging、寫入目的地，是唯一例外。**期間**：⑩ 的任何一趟（完整流程、`--resume`、`--promote`——前兩者在終態落地之後會自動晉升）執行期間。**禁止**：①搬移、刪除或替換真正 repo 的 `python/`、`python/baselines/`、`python/baselines/i074_stage2/`、`…/i074_stage2/failed/` 這幾個目錄本身（含 `mv`、`rm -rf` 之後重建、換成 symlink、會刪掉它們的 `git clean`／`git stash -u`）；②修改、刪除、搬移或替換 active staging（`.promote-staging-*`）與本次 target 目的地（`evidence/` 或 `failed/<本次的名稱>/`）的**任何成員**，含目錄本身；③移除或改寫 `.gitignore` 的 `python/baselines/i074_stage2/.promote-staging-*` 規則、以 `git add -f` 把 staging 加進 index。**照常**：其他檔案的一般編輯、`git add -A` 與 commit（含上述目錄之內、不屬於②的其他檔案）。**違反時（照實）**：①symlink 指向的目標⛔ 不會被寫入，record 可能落在被搬走的目錄（可能在 repo 外），鏈檢查 → **9**；②收尾重算之前的改寫 → **9**，之後的改寫晉升⛔ 無法偵測（見上一列③）；③在守門之前就已失效的規則由 ignore 守門擋下 → **9**：目的地那組 6a／6b 都查（第 5b 步，第六輪 review），staging 那組只在 6b 建 staging 之前查（第五輪 review）；守門之後才改壞規則或 `git add -f` staging，晉升⛔ 無法偵測，staging 可能被 commit——commit 前的 review 看得到 |

**結束碼的分類（⚠️ 第二輪 review）**——下表的「不通過」欄都依這個分類：

| 結束碼 | 只用在 |
|---|---|
| **9** | inventory 之後的任何完整性漂移：open／`openat`／`fstat` 的錯誤（`ENOENT`、`ELOOP`、`ENOTDIR`…）、型別或 `st_dev`／`st_ino`／`st_nlink` 不符、鏈檢查不符、收尾重算不符（第四輪 review）、ignore 守門不通過（第五輪 review）、驗證不通過、驗證模式輸出的 SHA 不符、6b 的來源側任何錯誤（開檔、`fstat`、讀到的 SHA ≠ inventory）、目的端不在 8 的 errno 清單內的錯誤、`rename_noreplace_at` 的 `EEXIST`／`ENOTEMPTY` 以外的錯誤（含 `EXDEV`、`EINVAL`、`ENOSYS`——重跑解決不了） |
| **3** | 只有在已核對的 fd 上 `fsync()` 本身失敗：第 4 步（來源）、6a（目的地）、6b 在 rename **之後**的 parent |
| **8** | 只有目的端的容量與寫入 I/O：`statvfs` 空間不足；staging 與 `failed/` 的建立、寫入、rename 之前的 fsync 在已核對的 fd 上回 `ENOSPC`／`EDQUOT`／`EIO`；`rename_noreplace_at` 的 `EEXIST`／`ENOTEMPTY`（重跑時走 6a） |

| 步 | 實作（`i074_stage2_promote.py`，除第 1 步） | 不通過 |
|---|---|---|
| 1 | （shell）`/proc/locks`、preflight 0、`state-check --require run,preflight`——沿用 ⑦b 已實作、只用 git、shell 與內嵌標準庫的那一段（⛔ 不重寫） | **9** |
| 2 | 複本 `git worktree prune`；真正 repo 的 `i074_stage2/` **直屬**下符合 `^\.promote-staging-[0-9a-f]{16}$` 的目錄以 fd-anchored 的 `remove_tree_at(<i074_stage2 fd>, <名稱>)` 刪除（⛔ 不用路徑版的 `remove_tree()`；第二輪 review）（⛔ 其他一律不動）。⚠️ 實體目錄還在的 worktree 登記 prune 收不掉，保留到 `<work>` 刪除（「三」#5） | **9** |
| 3 | **解析本次的唯一終態**：複本 `git status --porcelain=v1 -z --untracked-files=all --ignored -- python/baselines/i074_stage2/`，先排除可辨識的 ③ orphan（`i074_stage2/` 或 `failed/` 直屬下的 `.<name>.staging-<16 hex>`、`.probe-<16 hex>-…`），再依頂層分組：`evidence/…` → evidence；`failed/<name>/…` → failed/<name>；其他 → 不認得。必須**恰好一組**；failed 的 `name` 必須 ＝ `<preflight.bundle_id>-<preflight.counterfactual_semantic_sha256>`；`replay_done.json` 存在時，evidence 對應 rc 0、failed 對應 rc 6 | 零組、多組、不認得、名稱不符、讀取錯誤 → **9** |
| 3b | ⚠️ **封閉 inventory**（第一輪 review；第二輪改成 fd-anchored）：從複本根的 fd 逐層開到終態目錄，再以 `os.scandir(<目錄 fd>)` 與 `os.stat(<名稱>, dir_fd=<目錄 fd>, follow_symlinks=False)` 遞迴列舉、⛔ 不跟隨 symlink，**只接受一般檔案（`st_nlink` ＝ 1）與目錄**；symlink、FIFO、socket、device、hardlink 一律拒絕，且⛔ 不開啟、⛔ 不 fsync 它的目標。之後的 identity 讀取、fsync、複製與比對**都只用這份 inventory 與持有的 fd**（⛔ 不重用會跟隨 symlink 的 `_fsync_tree()`） | **9** |
| 3c | **identity 的綁定**（「八之三」的「驗證用的錨點」列）：依 inventory 以 `os.open(<名稱>, O_RDONLY \| O_NOFOLLOW, dir_fd=<目錄 fd>)` 開啟、`fstat` 與 inventory 相同（不符 → 9），讀終態封存的 identity（evidence 的 `identity/run_identity.json.gz` 解壓後的 bytes；failed record 的 `run_identity` 以 canonical JSON 序列化）的 SHA-256 ＝ `preflight.identity_sha256` | **9** |
| 4 | **來源的 durability**：依 inventory 以 `os.open(<名稱>, O_RDONLY \| O_NOFOLLOW [\| O_DIRECTORY], dir_fd=<目錄 fd>)` 開啟每個檔案與目錄，`fstat` 的型別、`st_dev`／`st_ino`（一般檔案另驗 `st_nlink` ＝ 1）必須與 inventory 相同 → `fsync()`；最後 fsync 持有的 `i074_stage2/`（failed 另加 `failed/`）的 fd | open／`fstat` 的任何錯誤（`ENOENT`、`ELOOP`、`ENOTDIR`…）或不符 → **9**（完整性漂移）；只有 `fsync()` 本身失敗 → **3** |
| 5 | 從真正 repo 根的 fd 逐層開到 `i074_stage2/` 並持有；failed 另開 `failed/` 並持有（不存在 → 6b 時建立；存在但是 symlink 或不是目錄 → 9）；`os.stat(<target 名稱>, dir_fd=<parent fd>, follow_symlinks=False)`：不存在 → 6b；是目錄 → 6a | 其他型別或任何錯誤 → **9** |
| 5b | ⚠️ **目的地的 ignore 守門**（第六輪 review；6a／6b 共用，在分支**之前**）：以 `/usr/bin/git -C <真正 repo> check-ignore --no-index -z --stdin` 查 `<target 相對路徑>/<inventory 每個一般檔案的相對路徑>`，必須**全部不**被忽略（git 回 1，且輸出為空）。⛔ 不加 `-v`、以成員路徑查（理由同 6b）。⚠️ `--no-index`：目的地已被追蹤時，過廣規則照樣不通過——刻意從嚴（過廣規則會讓之後的 failed record 進不了版控） | 有任何一筆被忽略、git 的其他結束碼（例如 128）、逾時、輸出無法解析 → **9**；6a ⛔ 不呼叫驗證模式、⛔ 不 fsync 目的地，6b ⛔ 不建 staging（第 4 步的來源 fsync 在這之前已照常完成；⚠️ 第七輪 review 訂正） |
| 6a | 目的地從 parent fd 以同一套規則建 inventory（symlink、特殊檔案、`st_nlink` ≠ 1 → **9**）→ 與來源逐位元比對（相對路徑集合、型別、每檔 SHA-256，兩邊都經 fd 讀）→ `--verify-promotion-staging <目的地> --target <t>`，輸出的 `manifest_sha256`／`record_sha256` ＝ 本程序經 fd 讀到的 bytes 的 SHA → **收尾重算**（經 inventory 的 fd 重算每檔 SHA ＝ 比對時的值；第四輪 review）→ 鏈檢查 → 依 inventory 的 fd fsync 目的地的每個檔案、每個目錄與 parent（⚠️ **⑦c 實作第一輪 review（2026-10-01）**：failed record 另 fsync `i074_stage2/`——`failed/` 自己的目錄項目在那裡） | 比對、驗證、SHA 綁定、收尾重算、鏈檢查不符或任何開檔錯誤 → **9**（⛔ 不覆寫）；只有 `fsync()` 本身失敗 → **3** |
| 6b | ⚠️ **staging 的 ignore 守門**（第五輪 review；建 staging **之前**；目的地那組在第 5b 步）：先產生本次的 staging 名稱 → 以同一個命令查 `<staging 相對路徑>/<inventory 每個一般檔案的相對路徑>`，必須**全部**被忽略（git 回 0，且 NUL 分隔的輸出集合 ＝ 輸入集合）；不符、git 的其他結束碼（例如 128）、逾時、輸出無法解析 → **9**，⛔ 不建 staging。⛔ 不加 `-v`（`-v` 連否定規則的命中也回 0）；⛔ 不只查 staging 目錄本身（只對目錄生效的規則對還不存在的目錄查不到；2026-10-01 以 git 2.30.2 實測兩者）→ 空間：`os.statvfs(<i074_stage2 fd>)` 的 `f_bavail × f_frsize` ≥ 來源 allocated bytes（`st_blocks × 512`，含目錄）的 **2 倍** → `os.mkdir(.promote-staging-<16 hex>, dir_fd=<i074_stage2 fd>)`（⚠️ 這個名稱由 `.gitignore` 排除，第一輪 review）並持有它的 fd → 依 inventory 逐項：來源以 `O_RDONLY \| O_NOFOLLOW` 經 fd 開啟並以 `fstat` 核對，目的以 `mkdir(…, dir_fd=)`／`os.open(…, O_WRONLY \| O_CREAT \| O_EXCL \| O_NOFOLLOW, dir_fd=)` 建立 → 逐位元複製、比對（每檔 SHA ＝ 來源經 fd 讀到的值）→ `--verify-promotion-staging <staging> --target <t>`，輸出的 SHA ＝ 本程序寫入的 manifest／record bytes 的 SHA → fsync staging 的每個檔案與目錄 → failed 時 `failed/` 不存在就 `mkdir(…, dir_fd=<i074_stage2 fd>)`、開啟並持有、fsync `i074_stage2/`（⚠️ **⑦c 實作第一輪 review（2026-10-01）**：**不論新建或既有都 fsync**——上一次可能建了 `failed/` 卻在 fsync 失敗後回 8，重跑時跳過這一步就會在目錄項目未落盤時回 6） → **收尾重算**（經 staging 的 fd 重算每檔 SHA ＝ 複製時的值；第四輪 review）→ 鏈檢查（含 staging 名稱 → staging 的 inode）→ `rename_noreplace_at(<i074_stage2 fd>, <staging 名稱>, <parent fd>, <target 名稱>)` → 鏈檢查（含目的地名稱 → staging 的 inode）→ fsync parent 的 fd（⚠️ **⑦c 實作第一輪 review（2026-10-01）**：跨目錄 rename 的兩個 parent——failed record 另 fsync `i074_stage2/`；任一失敗 → **3**） | 依上方「結束碼的分類」：**8** ＝ 空間不足、目的端在已核對的 fd 上回 `ENOSPC`／`EDQUOT`／`EIO`、rename 的 `EEXIST`／`ENOTEMPTY`（重跑時走 6a）；**9** ＝ 來源側的任何錯誤或不符、目的端的其他 errno、staging 與來源逐位元相同卻驗不過、SHA 綁定、收尾重算或鏈檢查不符、rename 的其他錯誤；**3** ＝ rename 之後 parent 的 `fsync()` 失敗。⚠️ rename 之前的任何失敗都以 `remove_tree_at()` 清掉本次的 staging（⚠️ **⑦c 實作第一輪 review（2026-10-01）**：名稱目前解析到的⛔ 不是建立時的 inode 就⛔ 不刪、保留現場、**9**；⚠️ **⑦c 實作第二輪 review（2026-10-02）**：名稱已不存在（staging 被搬走或刪掉）同樣 **9**——⛔ 不等於已清除，⛔ 不沿用代表「可安全重跑」的 8；第 2 步清孤兒時的 ENOENT 仍是 no-op；⚠️ **⑦c 實作第三輪 review（2026-10-02）**：核對⛔ 不只在清理之前——stat 與開啟之間（開啟的 `ENOENT`／`ELOOP`／`ENOTDIR`、開啟之後 inode 不符）、遞迴之後 rmdir 之前（再以 expect 核對一次）、rmdir 本身（`ENOENT`／`ENOTEMPTY`；⚠️ **第四輪 review（2026-10-02）**補 `ENOTDIR`／`ELOOP`——換成 symlink 或一般檔案）都是 **9**；最後一次核對之後才換成空目錄時 rmdir 會刪掉那個空目錄（照實的界線），以仍開著的 fd 的 `st_nlink` ≠ 0 事後發現 → **9**）；rename 之後鏈檢查不符時 staging 已不在原名稱下，⛔ 不追著刪 |
| 7 | 成功 | evidence **0**、failed record **6** |

未列出的任何例外一律 **9**（⛔ 不猜）。⚠️ `--promote` ⛔ 不呼叫 ③ 的 recovery、replay、finalize、publish（spy 斷言）。
⚠️ 會寫到哪裡（照實）：真正 repo 的 `i074_stage2/` 下的 staging、目的地與（必要時）`failed/`；複本的 `.git`（驗證模式的合成守門 worktree 與 object）；
`<work>/tmp`。⛔ 不寫真正 repo 的 `.git`；⛔ 不改複本內的終態（只 fsync）。

###### 二之四、B／C 判讀器

**`replay_bundle/stage2_verdict.py`**（規則唯一的定義處；「關閉條件」的判讀矩陣 ＋ v26 補充的差異表）：

| 檢查（逐列，任一不符 → 該列 **C**，理由逐條記下） | 對應 |
|---|---|
| `differences` ⊆ 允許清單 {`lifecycle_phase`、`market_bias`、`action_state`、`position_action_condition`、`rr_decoupling_candidate`}；含 top-level `position_action` → C；含清單以外的任何欄位 → C | j、k |
| `position_action_condition` 在 `differences` 裡時，兩側物件**只有 `state` 不同** | j |
| `rr_decoupling_candidate`：after ＝ `true`、before ＝ `false` | 共同必要條件 |
| 分類只看 **after 的 `action_state`**：`AVOID` → AVOID 類；`HOLD` → 非 AVOID 類；其他 → C | 判讀矩陣的分類依據 |
| 兩側各自 `position_action_condition.state` ＝ `action_state` | i |
| `lifecycle_phase`：before ∈ {`TESTING`, `CONFIRMED`}、after ＝ `CONTINUATION` | e |
| 非 AVOID：`market_bias` `BULLISH_BIAS` → `BULLISH_CONTINUATION`；`before.lifecycle_phase` ＝ `TESTING` 時 `before.action_state` `CONDITIONAL_HOLD` → `after.action_state` `HOLD`；`before.lifecycle_phase` ＝ `CONFIRMED` 時 `before.action_state` `HOLD` → `after.action_state` `HOLD`（⚠️ 第一輪 review 訂正：原本把兩個欄位混寫） | f、a～b |
| AVOID：`market_bias` `BEARISH_BIAS` → `BEARISH_BIAS`；`before.lifecycle_phase` 是 `TESTING` 或 `CONFIRMED` 都一樣：`before.action_state` `AVOID` → `after.action_state` `AVOID` | g、c～d |
| `final_entry_state`：兩側都是 `BLOCKED` | h |

整體：**任一列是 C → 整體 C**（⛔ 不以多數決；l）；全部是 B → B。

**輸出的封閉 schema（⚠️ 第二輪 review）**——`judge_comparison()` 回傳、驗證模式 `--judge` 放進 `verdict` 欄位，以 canonical JSON 輸出；多欄、缺欄、型別或值域不符都是缺陷（測試逐鍵斷言）：

| 欄位 | 型別與值域 |
|---|---|
| 頂層 | 恰好五個欄位：`schema`、`verdict`、`row_count`、`c_row_count`、`rows` |
| `schema` | 固定字串 `"i074_stage2_verdict/v1"` |
| `verdict` | `"B"` 或 `"C"` |
| `row_count` | 整數 ≥ 1，＝ `rows` 的長度。comparison 為 0 列 → **拒絕判讀**（驗證模式回 1），⛔ 不以空集合判 B |
| `c_row_count` | 整數，＝ `rows` 中 `verdict` 為 `"C"` 的列數（⚠️ 取代 v1 含糊的 `c_rows`） |
| `rows` | 陣列，順序 ＝ comparison 的列順序（`validate_comparison_artifact()` 已驗依 (`symbol`, `timeframe`, `as_of`) 排序且唯一）；⛔ 不重排 |
| `rows[]` | 恰好六個欄位：`symbol`、`timeframe`、`as_of`（字串，照抄 comparison）；`class`（`"NON_AVOID"`／`"AVOID"`／`"UNCLASSIFIED"`，只看 after 的 `action_state`：`HOLD`／`AVOID`／其他）；`verdict`（`"B"`／`"C"`）；`reasons`（reason code 的陣列，字典序、去重；`verdict` ＝ `"B"` ⇔ 空陣列） |

reason code（封閉集合；每項檢查固定對應一個 code，⛔ 不輸出自由文字）：

| code | 觸發 |
|---|---|
| `ROW_SHAPE_INVALID` | before 與 after **各自**必須有：`lifecycle_phase`、`market_bias`、`action_state`、`final_entry_state`（`str`）、`rr_decoupling_candidate`（`bool`）、`position_action_condition`（`dict`，含 `str` 欄位 `state`）——任一缺欄、`null` 或型別不符（以 `type(x) is …` 判斷，`bool` ⛔ 不算 `int`）即命中；出現時 `reasons` **只有**這一個 code、⛔ 不再做其他檢查。⛔ 不要求 `position_action` 存在（它只經 `differences` 檢查）；`differences`、key 欄位與兩側 key set 相同已由 `validate_comparison_artifact()` 驗過（第三輪 review 補列） |
| `POSITION_ACTION_CHANGED` | `differences` 含 top-level `position_action` |
| `DIFF_FIELD_NOT_ALLOWED` | `differences` 含允許清單與 `position_action` 以外的欄位 |
| `POSITION_ACTION_CONDITION_NON_STATE_DIFF` | `position_action_condition` 兩側除了 `state` 還有其他差異 |
| `RR_DECOUPLING_NOT_FALSE_TO_TRUE` | `rr_decoupling_candidate` 不是 before `false`、after `true` |
| `ACTION_STATE_UNCLASSIFIED` | after 的 `action_state` 不是 `HOLD`／`AVOID`（`class` ＝ `"UNCLASSIFIED"`；⛔ 不再做下面兩項依分類的檢查） |
| `CONDITION_STATE_MISMATCH` | 任一側的 `position_action_condition.state` ≠ 該側的 `action_state` |
| `LIFECYCLE_PHASE_UNEXPECTED` | before ∉ {`TESTING`, `CONFIRMED`} 或 after ≠ `CONTINUATION` |
| `MARKET_BIAS_UNEXPECTED` | `market_bias` 的轉換不符該分類 |
| `ACTION_STATE_TRANSITION_UNEXPECTED` | `action_state` 的轉換不符該分類與 `before.lifecycle_phase`（before 不是 `TESTING`／`CONFIRMED` 時只記上一項、⛔ 不重複記這一項） |
| `FINAL_ENTRY_STATE_NOT_BLOCKED` | 任一側的 `final_entry_state` ≠ `BLOCKED` |

除了 `ROW_SHAPE_INVALID` 與 `ACTION_STATE_UNCLASSIFIED` 註明的略過，每一列的檢查**全部執行**、命中的 code 全部記下（⛔ 不在第一個不符就停）。
⚠️ 第一輪 review：`stage2_verdict.py` 只有純函式 `judge_row()`／`judge_comparison()`；⛔ 沒有自己的 CLI、⛔ 不自己讀檔——一律由驗證模式的 `--judge`
在**同一個程序**、以剛驗過的 comparison 物件呼叫（「三」#9）。

**`scripts/judge-i074-stage2.sh`**（`REPLAY_IMAGE_ID=… scripts/judge-i074-stage2.sh`；⛔ 沒有參數）：

| 序 | 動作 | 不通過 |
|---|---|---|
| 0 | 與 ⑩ 的入口同一套 bootstrap：`#!/bin/bash -p`、bash 內建的守門（`$-` 含 `p`、清 `BASH_ENV`／`ENV`／`LD_*`／`PYTHON*`／`GIT_*`、`PATH=/usr/bin:/bin`）、`REPLAY_IMAGE_ID` 格式；本腳本的內容 ＝ HEAD | 1 |
| 1 | 讀真正 repo 的 `python/baselines/i074_stage2/evidence/evidence_manifest.json`（內嵌標準庫）取 `finalizer_provenance.base_commit`（oid40） | 1 |
| 2 | `git cat-file -e <base>^{commit}`（不可達 → 拒絕，m）→ `git clone --no-hardlinks --no-checkout --template=` 到暫存目錄 → detached checkout `<base>`；HEAD ＝ base（「三」#1） | 1 |
| 3 | 暫存複本的 `finalize-stage2-evidence.sh --verify-promotion-staging <真正 repo>/…/evidence --target evidence --judge`——錨點取自 `base_commit` 的樹（⛔ 不讀真正 repo 的工作樹或目前的 HEAD）；信任根的綁定（manifest 的 `base_commit` ＝ 暫存複本的 HEAD ＝ 第 1 步讀到的值，第 1、3 步之間 manifest 若被換掉就在這裡擋下）；⚠️ 驗證與判讀在**同一個 Python 程序**（第一輪 review） | 1（⛔ 不輸出 B／C） |
| 4 | stdout：驗證模式的 JSON（含 `verdict`、`base_commit`、manifest SHA）；清掉暫存複本。**結束碼 0 ＝ 已判讀（B 與 C 都是 0）、1 ＝ 拒絕判讀**（「三」#8） | — |

###### 二之五、端到端結束碼（⑩ 完整）

⑦b「二之十」的表沿用；**O8** 改成「已有終態 → 晉升 → 晉升的結束碼」：**0**（成功 archive 已在真正 repo durable）、**6**（failed record 已在真正
repo durable）、**3**（某一層 durability 未確認 → 重跑 `--promote`）、**8**（可以安全重跑 `--promote`）、**9**（需要人工判斷）。其餘列不變。

###### 二之六、對已確認章節的修改（⚠️ 都加註「⑦c 細部計畫 v1」）

| 位置 | 修改 |
|---|---|
| v29「八之三」的「會寫到哪裡」列與 n11 的最後一句 | 「崩潰留下的 worktree 登記由下一次 `--promote` 的 `git worktree prune` 清掉」改成照實：prune 只清實體目錄已消失的登記；實體目錄還在的保留到 `<work>` 刪除，⛔ 不影響冪等（n11 改成「下一次 `--promote` 照樣成功」） |
| v29「八之三」的「驗證用的錨點」列 | 補「唯讀入口 ＝ `--verify-promotion-staging`」與 identity 綁定的做法（「二之三」第 3c 步） |
| v29「八之三」判定順序的第 3、4 步之間（⚠️ 第一輪 review） | 插入「封閉 inventory」（「二之三」第 3b 步：`lstat`、⛔ 不跟隨 symlink，只接受一般檔案與目錄；不符 → **9**）；第 4 步與 6a／6b 的 fsync、複製、比對都依它；⚠️ 第二輪 review：所有操作錨定在持有的 fd 上、rename 前後做鏈檢查（「二之三」的「路徑錨定」） |
| v29「八之三」判定順序第 4、6a、6b 列的「不通過」（⚠️ 第二輪 review） | 依「二之三」的「結束碼的分類」細分：第 4 步的開檔與 `fstat` 錯誤由 3 改成 **9**、只有 `fsync()` 失敗是 3；6b 的 8 限縮為目的端的容量與寫入 I/O 及 rename 的 `EEXIST`／`ENOTEMPTY`，rename 的其他錯誤（含 `EXDEV`、`EINVAL`、`ENOSYS`）與來源側錯誤改成 **9** |
| v29「八之三」判定順序的 6a、6b 與第 5 步之後（⚠️ 第四～六輪 review） | 6a 在驗證之後、6b 在 rename 之前加「收尾重算」（第四輪）；第 5 步之後插入 6a／6b 共用的第 5b 步「目的地的 ignore 守門」（第六輪）；6b 開頭加「staging 的 ignore 守門」（第五輪）；不通過一律 **9** |
| v29「八之一」的「真正 repo」列（⚠️ 第三輪 review；第四輪重寫） | 補例外：⑩ 的任何一趟執行期間，晉升程序以外的程序⛔ 不得搬移、刪除或替換 `python/`、`python/baselines/`、`i074_stage2/`、`failed/` 本身，⛔ 不得修改、刪除、搬移或替換 active staging 與本次目的地的任何成員，⛔ 不得移除或改寫 `.gitignore` 的 staging 規則、⛔ 不得 `git add -f` staging（「二之三」的操作契約）；其他編輯、`git add -A` 與 commit 照常 |
| ⑦ 總綱「三」的 ⑦c 列、v29「八之三」的「真正 repo 的錨點」列 | 「以 `git archive` 取出」改成「`git clone --no-hardlinks` ＋ detached checkout」（「三」#1）；判讀與驗證在同一個程序（「三」#9） |
| ③「七之四」的 CLI matrix | 加第八種模式 |
| ⑦b「二之十」的 O8 列 | 同「二之五」 |

##### 三、總綱沒寫到、本計畫補上的決定（⚠️ 待確認）

| # | 決定 | 理由 |
|---|---|---|
| 1 | 判讀器從 `base_commit` 取程式碼與錨點的方式改成「`git clone --no-hardlinks` ＋ detached checkout `base_commit`」，⛔ 不用 `git archive` | 內容與 `git archive` 相同（同一個 tree），但多了 git 物件——③ 的完整 verifier 含 shell 的合成守門，需要在 repo 裡套 patch；也讓判讀器與晉升呼叫**同一個**驗證入口（「二之二」），⛔ 不再寫一份 |
| 2 | `--verify-promotion-staging` 由本腳本所在 repo 的 `origin` 推導真正 repo，`path` 只接受 staging 與目的地兩種；只讀、結束碼只有 0／1 | 總綱「路徑限在真正 repo 的 `i074_stage2/` 直屬下」；開路徑參數就等於開覆寫口（⛔ 不開） |
| 3 | 信任根綁定的三者相等拆兩處：「複本 HEAD ＝ freeze record 的 `repo_head`」由 preflight 0（⑦b）驗；「`finalizer_provenance.base_commit` ＝ 複本 HEAD」由驗證模式驗（注入的 `--base-commit` 就是本 repo 的 HEAD） | 每一處都只用自己手上可信的值；判讀器的暫存複本同理（HEAD ＝ base_commit） |
| 4 | identity 的綁定以「終態封存的 identity 的 canonical bytes 的 SHA ＝ `preflight.identity_sha256`」實作（第 3c 步） | XDG 的 identity 檔本身就是 canonical JSON；比 SHA ⛔ 不需要再讀 XDG |
| 5 | 第 2 步只做 `git worktree prune`；實體目錄還在的登記保留到 `<work>` 刪除；同步訂正「八之三」與 n11 的文字（「二之六」） | ⑦b 第二輪 review 重現：prune 收不掉實體目錄還在的登記；它們⛔ 不影響冪等（每次都新建 `mktemp` 路徑） |
| 6 | 晉升自己的 staging 名稱 `.promote-staging-<16 hex>`，直屬真正 repo 的 `i074_stage2/`；第 2 步只清這一種；第 3 步只**忽略**（⛔ 不刪除）複本內 ③ 的 orphan | 「⛔ 不動其他東西」「⛔ 不改複本內的終態」；兩種 orphan 位在不同的 repo、名稱可辨識 |
| 7 | 8、9 搬進 `publish.py`；晉升模組經 `_i074_bootstrap` 載入它；supervisor 與 orchestrator 維持鏡像（它們不 import repo 模組），由測試斷言相等 | ⑦b「三」#2 的後半 |
| 8 | 判讀器的結束碼：0 ＝ 已判讀（B 或 C 寫在 JSON）、1 ＝ 拒絕判讀；⛔ 不把 B／C 編進結束碼 | B 與 C 都是合法的判讀結果，⛔ 不是錯誤 |
| 9 | ⚠️ 第一輪 review 改寫：**驗證與判讀在同一個 Python 程序**——判讀直接吃 `verify_stage2_graph()` 剛驗過、留在記憶體的 comparison 物件（驗證模式的 `--judge`）；⛔ 不採「兩個程序以 manifest SHA 綁定」、⛔ 不採私有 snapshot | 兩個程序之間（即使 snapshot）仍有「驗完、再讀」的窗口；同程序判讀沒有第二次讀取 |
| 10 | 判讀器⛔ 不取鎖（它唯讀、在晉升之後由人執行），但用 ⑩ 入口同一套 bootstrap 與「本腳本 ＝ HEAD」 | ⛔ 不讓遮蔽的 PATH 或改過的腳本偽造判讀；⛔ 不為唯讀步驟引入 sentinel |
| 11 | 判讀規則比矩陣多驗三件（before 的 `rr_decoupling_candidate` 為 `false`、after 的 `action_state` 必須在分類裡、兩側 `position_action_condition.state` ＝ `action_state`），任一不符 → C | 矩陣的共同必要條件與交叉斷言；⛔ 不讓矩陣外的形狀默默落到 B |
| 12 | 6b 在 `failed/` 不存在時建立它並 fsync `i074_stage2/`；建立或這次 fsync 在已核對的 fd 上回 `ENOSPC`／`EDQUOT`／`EIO` → 8，其他錯誤（含 `failed` 已存在但是 symlink 或不是目錄）→ 9（⚠️ 第三輪 review：同步「二之三」的結束碼分類） | 真正 repo 目前沒有 `failed/`；它是目的地 parent 的一部分 |
| 13 | ⚠️ 第一輪 review：`.gitignore` 精準排除 `python/baselines/i074_stage2/.promote-staging-*`；正式目的地⛔ 不排除 | ⑩ 期間真正 repo 照常 commit，`git add -A` ⛔ 不得收進半成品；發布之後的終態要進版控 |
| 14 | ⚠️ 第一輪 review：晉升的 fsync、複製、比對全部基於 `lstat` 的封閉 inventory 與 `O_NOFOLLOW`，只接受一般檔案與目錄（第 3b 步；第二輪改成 fd-anchored，#15） | 完整驗證在第 6 步才執行，第 3c 步的 identity 讀取與第 4 步的 fsync 之前⛔ 不能讓被竄改成 symlink 的終態把晉升帶到樹外 |
| 15 | ⚠️ 第二輪 review：晉升的所有檔案系統操作錨定在持有的 fd 上（`openat` 逐層 `O_DIRECTORY \| O_NOFOLLOW`、`dir_fd`、`rename_noreplace_at()`），rename 前後各做一次鏈檢查，一般檔案 `st_nlink` ＝ 1；驗證模式（只能吃路徑）以輸出的 manifest／record SHA 綁定到經 fd 寫入或讀到的 bytes；照實的界線見「二之三」 | 完整路徑的 `O_NOFOLLOW` 只保護最後一層（2026-10-01 重現）；⑩ 期間真正 repo 照常編輯。保證的只有「⛔ 不跟隨替換後的 symlink」；rename 來源被換掉、父目錄被搬走（可能搬到 repo 外，record 跟著落在那裡）⛔ 無法預防，只能事後以鏈檢查變成 9——所以另訂操作契約（#18；⚠️ 第三輪 review 撤回「不會寫到樹外」） |
| 16 | ⚠️ 第二輪 review：結束碼依「二之三」的分類——漂移一律 9、只有已核對 fd 上的 `fsync()` 失敗才是 3、8 只留給目的端的容量與寫入 I/O；rename 的 `EXDEV`／`EINVAL`／`ENOSYS` 改成 9 | 3 與 8 的語意是「重跑 `--promote` 就好」；漂移與不支援的檔案系統重跑解決不了，必須交人工 |
| 17 | ⚠️ 第二輪 review：判讀輸出（「二之四」）與驗證模式的輸出（「二之二」）都是封閉 schema、穩定的 reason code、列順序 ＝ comparison 的順序 | 判讀結果是正式紀錄；⛔ 不讓它的形狀隨實作細節漂移 |
| 18 | ⚠️ 第三輪 review、第四輪重寫：操作契約——對象是晉升程序以外的所有程序（晉升自己的建立、清理與 rename 是唯一例外）；⑩ 的任何一趟執行期間⛔ 不得搬移、刪除或替換那幾個目錄本身、⛔ 不得改動 active staging 與本次目的地的任何成員、⛔ 不得動 `.gitignore` 的 staging 規則或 `git add -f` staging；其他編輯、`git add -A` 與 commit 照常（「二之三」） | fd-anchored 擋得住 symlink 替換、擋不住「已持有的目錄被搬走」與「成員被原地改寫」；這些對象在 ⑩ 期間本來就沒有被外部改動的理由，禁止的代價小；違反時由鏈檢查與收尾重算回 9（照實寫明偵測不到的部分） |
| 19 | ⚠️ 第四輪 review：收尾重算——6b 在 rename 之前、6a 在驗證之後，經持有的 fd 重算每檔 SHA，與比對時的值不同 → 9 | 驗證模式在 docker 內以路徑讀取，它通過之後的原地改寫鏈檢查抓不到；多讀一次終態的代價遠小於把未驗過的內容發布成 0／6 |
| 20 | ⚠️ 第四輪 review：`publish.py` 的 rename 拆三層（⚠️ 實作差異見「二之一」：私有層實作成 `_encode_rename_paths()`／`_raise_for_rename()` ＋ 兩個 loader；計畫原文：私有 `_renameat2()`、`rename_noreplace_at()`、`rename_noreplace()`）；路徑含 NUL 一律拋錯；`_at()` 的 fd 以 `type(fd) is int and fd >= 0` 檢查（第五輪 review：`bool` 是 `int` 的子類） | 單一 component 的規則只屬於 `_at()`；既有的五個呼叫點傳多層路徑、⛔ 不能被新規則打壞；`c_char_p` 在 NUL 截斷是既有的潛在誤 rename（實測） |
| 21 | ⚠️ 第五輪 review、第六輪拆組：ignore 守門分兩組——**目的地那組**（全部不被忽略）是 6a／6b 共用的第 5b 步，**staging 那組**（全部被忽略）只在 6b 建 staging 之前；兩組都用 `check-ignore --no-index -z --stdin`、⛔ 不加 `-v`、以成員路徑查；不通過 → 9。⛔ 不另外加進 ⑦b 的 preflight | 入口清單⛔ 不含 `.gitignore`，啟動前就失效的規則只有執行期才擋得到；6a 是 3 之後的 recovery 路徑——兩次 `--promote` 之間規則被改壞時，只查 6b 會讓 6a 回 0／6 而目的地進不了版控（第六輪 review）。晉升時才失敗的代價是終態留在複本、修好規則之後重跑 `--promote` 即可（冪等），⛔ 不需要為了提早幾小時發現而改動已確認的 ⑦b preflight |

##### 四、受影響檔案

見「二之一」；另有 `docs/issue.md`、`docs/development-workflow.md`（⑩ 程序：晉升與判讀）、`docs/sr-zone-scoring.md`（判讀器的規則與晉升契約）。
⛔ 不改：`evaluation.py`、`run-replay-offline.sh`、supervisor、label shim、sizing。

##### 五、測試（id → 落點）

| id | 內容 | 落點 |
|---|---|---|
| 「六、9」a～d | 四格（before `TESTING`／`CONFIRMED` × 非 AVOID／AVOID）各一列合成 row → B | pytest（`test_i074_stage2_verdict.py`） |
| e～k | 各一支（lifecycle 沒翻轉、非 AVOID 的 `market_bias` 沒翻、AVOID 的 `market_bias` 變了、`final_entry_state` 變了、`action_state` 與 `position_action_condition.state` 不一致、允許清單以外的差異、top-level `position_action` 有變）→ C；k 另驗 `position_action` 相同時不影響 B；另加 `position_action_condition` 的非 `state` 欄位不同、before 的 `rr_decoupling_candidate` 為 `true`、after 的 `action_state` 不在分類裡 → C | pytest |
| l | 多列中一列 C → 整體 C | pytest |
| 判讀輸出的 schema（第二輪 review） | 頂層與 `rows[]` 的 key set 逐鍵相等、型別與值域；e～k 每一支斷言**完整的** `reasons` 陣列（⛔ 不只斷言含某個 code）；多項不符同時命中時 code 全部記下且字典序；`ROW_SHAPE_INVALID` 只有它一個；`UNCLASSIFIED` 不再記依分類的 code；列順序 ＝ comparison；`c_row_count`；0 列 → 拒絕；固定 fixture 的 canonical bytes 與 golden 逐位元相同 | pytest |
| m | 驗證模式：成員 bytes 被改、manifest 驗不過、`finalizer_provenance.base_commit` ≠ 本 repo HEAD → 拒絕（pytest，`verify_promotion_target()`）；判讀器：`base_commit` 不可達、該 commit 裡缺錨點、驗證模式不通過 → 1 且⛔ 不輸出 B／C；⚠️ 第一輪 review：判讀路徑中 comparison 只被讀一次、判讀的物件就是 `verify_stage2_graph()` 回傳的那一個（spy），shell 段之後、Python 段之前改掉 comparison 以外的成員 → 拒絕；對照組：真正 repo 工作樹或目前 HEAD 的錨點被改 → 照樣判讀（shell） | pytest ＋ shell |
| 驗證模式 | 路徑規則（真正 repo 外、複本內、symlink、非直屬、名稱不符、target 不符）→ 1；`--verified-*` 由外部傳入、缺、模式不符 → 1；唯讀掛載；argv 與 fixture 逐 token 相同；failed 的 `target` 名稱裡的 SHA ≠ 重算的語意 SHA → 1；⚠️ `--judge`（第一輪 review）：搭 failed target → 1、重複 → 1、驗證不通過時⛔ 不呼叫判讀函式（spy）、帶 `--judge` 的 argv 與 fixture 逐 token 相同；輸出 JSON 的 key set 封閉（evidence、failed、evidence ＋ `--judge` 各一支，第二輪 review） | `test-replay-args.sh`（dry-run 與 fake docker）＋ pytest |
| n11 | 「八之三」每一步至少一支：1 → 9（⑦b 既有）；3 的零組／多組（`evidence/` 與本次的 failed record 同時存在）／不認得／名稱不符 → 9；3b 終態含 symlink（指向樹外的檔案：該檔案⛔ 不被開啟——spy；identity 檔本身換成這種 symlink 也一樣）、FIFO、socket → 9、整趟⛔ 沒有任何 fsync（在第 4 步之前就失敗）；3c identity 不符 → 9；4 fsync 失敗 → 3 且⛔ 不建 staging；6a 相同 → 0／6 只補 fsync、不同 → 9 ⛔ 不覆寫、目的地含 symlink → 9；6b 空間不足 → 8（釋出後重跑成功）、目的端寫入注入 `ENOSPC`／`EDQUOT`／`EIO` → 8、來源側讀取錯誤 → 9（其餘 errno 見下方「結束碼的分類」列；⚠️ 第三輪 review 同步）、`rename_noreplace_at()` 遇到 `EEXIST` → 8 而重跑走 6a、parent fsync 失敗 → 3 而重跑成功、staging 驗不過（終態發布之後才竄改來源，evidence 與 failed 各一支）→ 9 且目的地不存在、staging 已清；只竄改 `finalizer_provenance.base_commit` → 9；6a 的來源與目的地都改成同一份錯誤的 `base_commit` → 只有信任根綁定讓它 9；可辨識的 orphan staging 被清、其他檔案未被動；③ 的 recovery、replay、finalize、publish 在 `--promote` 中都⛔ 不被呼叫；真正 repo 的 `.git` 與複本內終態的 inventory 不變（除 fsync）；finalize 回 3 之後直接 `--promote` → 0；兩個執行目錄同時 `--promote` → 後到的 8（⑦b 的鎖測試沿用） | pytest（`test_i074_stage2_promote.py`，以注入的驗證函式與故障）＋ shell（整合，fake finalizer 的驗證模式） |
| 路徑錨定（第二輪 review；第三輪拆開斷言） | 以 symlink 指向的「marker 目錄」做對照，每支都斷言 marker 的 inventory（路徑、bytes、inode、mtime）前後完全相同：`failed/` 是指向 marker 的 symlink → 9；中間層（`python/baselines`、`i074_stage2`）換成指向 marker 的 symlink → 9；**barrier**（驗證之後、rename 之前）分三支：(i) 把 `failed/` 搬到 repo 內的別名、(ii) 搬到 repo 外（同一個檔案系統）、(iii) `rm -rf`——之後都換成指向 marker 的 symlink：三支都 → 9、marker 不變；(i)(ii) **照實斷言 record 落在被搬走的舊目錄**（(ii) 就是 repo 外，即操作契約禁止的情況）；(iii) 的 rename 失敗（`ENOENT`）、⛔ 沒有 record、staging 已清；`i074_stage2/` 另做一支 (i)；barrier：rename 之前把 staging 換成另一個目錄 → 9、被換進來的目錄⛔ 沒有被 rename；終態有 `st_nlink` ＝ 2 的檔案（另一個 link 在 marker 裡）→ 9，且 marker 的 inode ⛔ 不曾被開啟或 fsync（spy 記下每個被 fsync 的 fd 的 `fstat`）；名稱含 `/` 或 `..` → 9 | pytest（`test_i074_stage2_promote.py`） |
| rename 的三層（第四輪 review） | 既有 `test_replay_bundle_publish.py` 的絕對／多層路徑案例（probe A／B、既有目錄 → `FileExistsError`、`EXDEV` fail-closed）全部保留並通過——`test_exdev_is_fail_closed` 的 fake loader 只改成新的四參數簽章（⚠️ 實作差異：⛔ 不改，維持兩參數、與 `e1cbbbd` 的同名測試逐位元相同，見「二之一」）；`rename_noreplace_at()`：相對於 dirfd 成功（cwd 在別處）、`EEXIST` → `FileExistsError`、`EXDEV` 與舊 API 同一套對應；名稱是 `/` 開頭或含 `/`、`.`、`..`、空字串、含 NUL、fd 為負數（含 `AT_FDCWD`）、`src_dir_fd` 與 `dst_dir_fd` 各自傳 `True`／`False`（第五輪 review）→ `ValueError` 且⛔ 不呼叫 syscall（spy）；`rename_noreplace()` 的路徑含 NUL → `ValueError`、⛔ 不呼叫 syscall | pytest（`test_replay_bundle_publish.py`） |
| ignore 守門（第五輪 review） | host、真 git（`scripts/test-i074-stage2.sh`，fake finalizer 的驗證模式）：⑩ 開始之前把真正 repo 的 staging 規則刪掉 → `--promote` 回 9、`i074_stage2/` 下⛔ 沒有任何 `.promote-staging-*`、目的地不存在；改成過廣規則（`python/baselines/i074_stage2/*`、`python/baselines/`）→ 9、⛔ 沒有 staging；`i074_stage2/.gitignore` 加否定規則 `!.promote-staging-*` → 9；對照組：只對目錄生效的 `python/baselines/i074_stage2/.promote-staging-*/` → 照常晉升；修好規則之後重跑 `--promote` → 0／6。⚠️ 第六輪 review 的 recovery（host、真 git）：目的地已與來源逐位元相同（6b rename 之後、parent fsync 之前的狀態）＋過廣規則 → `--promote` 走 6a 回 9、目的地未被動；修好規則 → 0／6。pytest（注入 git runner；⚠️ 測試 image ⛔ 沒有 git）：git 回 128、逾時、輸出多一筆／少一筆／不在輸入集合 → 9 且⛔ 不呼叫 `mkdir`（spy）；第 5b 步有任何一筆被忽略 → 9，6b ⛔ 不呼叫 `mkdir`、6a ⛔ 不呼叫驗證模式、⛔ 不 fsync 目的地（spy 以 `fstat` 的 (`st_dev`, `st_ino`) 對照兩邊的 inventory，分辨被 fsync 的 fd 屬於來源或目的地；第 4 步的來源 fsync 照常；⚠️ 第七輪 review）；⚠️ 第六輪 review 的完整序列：6b 在 rename 之後注入 parent `fsync()` 失敗 → 3 → runner 改成回報目的地被忽略 → 重跑走 6a → 9（來源照常 fsync、目的地⛔ 沒有被 fsync）→ runner 恢復 → 重跑 → 0／6；argv 恰好是 `check-ignore --no-index -z --stdin`、⛔ 沒有 `-v`；probe 是成員路徑 | shell ＋ pytest |
| 操作契約與收尾重算（第四輪 review） | barrier：6b 的驗證模式通過之後、rename 之前原地改寫 staging 的一個非 manifest 成員（inode 不變）→ 9、目的地不存在、staging 已清；barrier：6a 的驗證模式通過之後原地改寫目的地的一個成員 → 9、⛔ 不 fsync 目的地（同一個 spy 分辨來源與目的地；⚠️ 第七輪 review）也⛔ 不回 0／6；對照組：同樣的改寫發生在收尾重算之後（fake 在重算之後才動）→ 照實斷言晉升回 0／6 而判讀器拒絕；`git check-ignore` 對 `python/baselines/i074_stage2/.promote-staging-0123456789abcdef` 命中、對 `…/evidence/x` 與 `…/failed/x` ⛔ 不命中 | pytest ＋ shell（`test-replay-args.sh`） |
| 結束碼的分類（第二輪 review） | 第 4 步：inventory 之後檔案被刪（`ENOENT`）、換成 symlink（`ELOOP`）、parent 換成檔案（`ENOTDIR`）、inode 被換 → 9；只注入 `fsync()` 失敗 → 3。6a：開檔錯誤 → 9、只有 `fsync()` 失敗 → 3。6b：複製途中來源被換或內容變了 → 9；staging 寫入注入 `ENOSPC`／`EDQUOT`／`EIO` → 8、注入 `EACCES` → 9；rename 注入 `EXDEV`／`EINVAL` → 9、`EEXIST` → 8；驗證模式輸出的 SHA 與本程序寫入的不同（fake）→ 9；rename 之後 parent 的 `fsync()` 失敗 → 3 | pytest |
| n9 | fake ⑩ 執行期間在真正 repo：(i) commit 一般檔案、(ii) 工作樹改錨點、(iii) commit 改錨點——晉升的證據與對照組逐位元相同；(ii)(iii) 的判讀器照樣判讀 | shell |
| staging 與 `git add -A`（第一輪 review） | staging 已建好、rename 之前（fake 驗證模式那一刻）在真正 repo 執行 `git add -A` 與 `git status`：staging ⛔ 不進 index、⛔ 不出現在未追蹤清單；發布之後的目的地照常出現在未追蹤清單 | shell |
| n12（晉升那一層） | failed record 晉升之後還沒 commit → 下一次 ⑩ 的 preflight 中止（⑦b 的 preflight 檢查 ＋ 本包真正的晉升）；commit 之後換新的 freeze record → 通過 | shell |
| 結束碼常數 | `publish.py` 的 8、9 ＝ supervisor 與 `run-i074-stage2.sh` 的鏡像 ＝ 晉升模組使用的值 | shell（`mirror_constants`）＋ pytest |
| 判讀器的 bootstrap | `bash <script>` → 1、`REPLAY_IMAGE_ID` 格式錯 → 1、腳本 ≠ HEAD → 1、呼叫者 PATH 前置的 fake 從未被執行 | shell |
| `patch_claims` 的 promotion 模式 | evidence 4 個 token、failed 5 個 token；路徑規則 | pytest |

##### 六、驗證

1. `python/scripts/test.sh` 完整執行（依序）。
2. 反向驗證（逐項注回 → 對應測試變紅 → 還原）：拿掉信任根綁定；驗證模式改成接受任意路徑；6a 比對不符改成覆寫；6b 失敗時不清 staging；
   第 3 步改成取第一組（不驗唯一）；判讀器改成多數決；判讀器拿掉允許清單；判讀器改讀真正 repo 工作樹的錨點；⚠️ 第一輪 review 追加：判讀改成驗證之後重新讀 comparison（read-once 的 spy 必須變紅）；第 4 步改用會跟隨 symlink 的 `_fsync_tree()`（symlink 那一支必須變紅）；拿掉 `.gitignore` 的 staging 規則（barrier 那一支必須變紅）；⚠️ 第二輪 review 追加：6b 改回完整路徑的 rename（`failed/` 換成 symlink 的那一支必須變紅）；拿掉 rename 之後的鏈檢查（搬走 parent 的 barrier 必須變紅）；拿掉 `st_nlink` 檢查（hardlink 那一支必須變紅）；第 4 步把開檔錯誤改回 3（`ENOENT` 那一支必須變紅）；拿掉驗證模式輸出的 SHA 綁定（fake 回不同 SHA 的那一支必須變紅）；`reasons` 改成第一個不符就停（多項命中的那一支必須變紅）；⚠️ 第四輪 review 追加：拿掉收尾重算（原地改寫的兩支 barrier 必須變紅）；`rename_noreplace_at()` 拿掉名稱檢查（拒絕 `/` 與 `..` 的那幾支必須變紅）；`rename_noreplace()` 改成經 `_at()`（既有多層路徑的測試必須變紅）；⚠️ 第五輪 review 追加：拿掉 ignore 守門（刪除規則的那一支必須變紅）；守門改成加 `-v` 只看結束碼（否定規則的那一支必須變紅）；守門改成只查 staging 目錄本身（只對目錄生效的對照組必須變紅）；fd 檢查改成 `isinstance(fd, int)`（`True`／`False` 那幾支必須變紅）；⚠️ 第六輪 review 追加：目的地那組守門改回只在 6b 查（recovery 序列與 host 的 recovery 那兩支必須變紅）。
3. 版控流程：stage 程式 → `git write-tree` → `scripts/make-i074-tooling-patch.sh --source-tree <T>` → stage patch → `--verify "$(git write-tree)"` ＝ 0；
   `e1cbbbd` ＋ counterfactual ＋ 新 tooling 的 worktree 跑它自己的 `python/scripts/test.sh`（`SKIP_SHELL_TESTS=1`），記錄 tree 與結果（比照 ⑦a）；
   `REPLAY_SMOKE=1` 的 smoke（含反事實段）跑一次。
4. 隔離：`/run/lock`、真正 repo 的 worktree 登記數與 `python/baselines/` 前後不變。

##### 七、風險與回滾

| 風險 | 對策 |
|---|---|
| 晉升寫錯地方或覆寫既有證據 | 只寫 `i074_stage2/` 下的 staging 與目的地；`rename_noreplace_at()`；6a ⛔ 不覆寫；路徑規則由驗證模式再擋一次；封閉 inventory；fd-anchored 的走訪與 `rename_noreplace_at()`、rename 前後的鏈檢查（第二輪 review）；staging 由 `.gitignore` 排除；測試斷言真正 repo 的 `.git` 與其他檔案不變 |
| rename 只能以名稱指定來源；父目錄在持有 fd 之後被搬走 | fd-anchored 只保證⛔ 不跟隨替換後的 symlink；父目錄被搬到 repo 外時 record 會跟著落在那裡（⛔ 無法撤銷）——操作契約（「三」#18）禁止這類操作，rename 前後的鏈檢查把違反變成 **9**、交人工；照實寫在「二之三」的界線，⛔ 不宣稱能預防（⚠️ 第三輪 review 訂正） |
| staging 或目的地的成員在驗證之後被原地改寫 | 操作契約禁止；收尾重算在 rename（6b）之前、驗證（6a）之後偵測 → **9**；重算之後的改寫照實寫成晉升⛔ 無法偵測，由判讀器、再跑 `--promote` 與 commit 前的 review 把關（⚠️ 第四輪 review） |
| `.gitignore` 的規則在 ⑩ 開始之前、或兩次 `--promote` 之間失效 | 目的地守門（第 5b 步，6a／6b 共用）與 staging 守門（6b）回 **9**；終態仍在複本或目的地，修好規則之後重跑 `--promote`（⚠️ 第五、六輪 review） |
| 改 `publish.py` 的 rename 打壞既有發布路徑 | 三層拆分、既有呼叫點與測試不動；唯一的行為差異（含 NUL 的路徑拋錯）有專屬測試 |
| 判讀規則寫錯（判準在看到結果之前就寫死） | 「六、9」a～m ＋ 本包補的三件；規則只有一份（`stage2_verdict.py`）；判讀器在結構上只能執行 `base_commit` 那一版 |
| tooling patch 因 `replay_bundle/` 的新增而變大 | 版控流程 ＋ 漂移測試；`e1cbbbd` ＋ 新 tooling 的既有測試全綠；smoke；⑨ 的 tooling 非語意 guard 仍會再驗 |
| 回滾 | ⑦c 之後依總綱「六」的反向順序（⑦d → ⑦c → ⑦b → ⑦a）；單獨 revert ⑦c 會回到 stub（9），tooling patch 要跟著重產 |

##### 八、歸檔（實作後；review 前保留本筆的計畫內容）

- `development-workflow.md`：「I-074 Stage 2 的正式執行程序」補晉升（步驟與結束碼、會寫到哪裡、重跑的判斷、⚠️ 執行期間的操作契約）與判讀器的操作程序。
- `sr-zone-scoring.md`：B／C 判讀器的規則（與判讀矩陣寫在一起，v29「七」的歸檔列）與晉升契約的現況。
- `issue.md`：⑦c 實作結果（與計畫的差異、反向驗證、新的 tooling patch SHA 與 `e1cbbbd` 的測試結果）。

#### Stage 2 步驟 ⑦c 實作結果（2026-10-01，✅ **review 通過**（四輪修正後）並 commit `5a03777`）

✅ 依「Stage 2 步驟 ⑦c 細部計畫 v1」（七輪 review 後確認並 commit `4b3aa10`）完成「二」的全部設計。⛔ **沒有在真正的 `/run/lock`
執行任何東西、沒有跑正式 replay、沒有晉升任何真正的證據、沒有 commit**；程式與文件已依總綱「二」的版控流程 stage（程式 → `git write-tree`
→ 產生 tooling patch → stage patch → `--verify`），停在 review。⚠️ ⑦c 動到 tooling 路徑（`replay_bundle/` 的 `publish.py`、`stage2_archive.py`、
新增的 `stage2_verdict.py`），tooling patch 已重產：**`5ff9a90f…`**（247,656 bytes；⑦b 為 `353c69a0…`）。

**⑦c 實作第一輪 review 的修正（2026-10-01）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | **`failed/` 的 parent fsync 失敗之後，重跑可能誤回 6（durable）**：只有新建 `failed/` 時才 fsync `i074_stage2/`；那次 fsync 失敗 → 8、`failed/` 留在磁碟 → 重跑看到 `failed/` 已存在就跳過，rename 之後只 fsync `failed/` 就回 6——`failed/` 在 `i074_stage2/` 裡的目錄項目從未確認落盤（review 以故障注入重現：`first_rc 8`、`second_rc 6`、`real_i074_fsynced_on_retry false`） | ✅ 先寫測試、確認紅（同一個重現）再修。⚠️ 這是**計畫本身的缺口**（「二之三」6b 原文就只在新建時 fsync），計畫的 6a、6b 列與 v29「八之三」的加註都已更新：①failed record 不論 `failed/` 新建或既有，**rename 之前都 fsync `i074_stage2/`**；②跨目錄 rename 之後**兩個 parent**（`failed/` 與 `i074_stage2/`）都 fsync，任一失敗 → 3；③6a 的 failed record 也另 fsync `i074_stage2/`（同一個缺口在 6a 的那一面）。測試補三支：「建立 `failed/` 之後 parent fsync 失敗 → 8 → 重跑在 **rename 之前**重新 fsync `i074_stage2/` → 6」（斷言順序）、rename 之後任一 parent 的 fsync 失敗 → 3、6a 的 failed record fsync `i074_stage2/`。反向驗證三項各自變紅（下表 R1～R3）——⚠️ 第一版測試只斷言「有 fsync」，R1 單獨注回時被 ② 的 rename 之後那一次 fsync 遮住而沒紅，改成斷言「在 rename 之前」之後才紅 |
| 中 | **staging 被換掉時，錯誤清理會刪掉換進來的目錄**：`finally` 只依名稱呼叫 `remove_tree_at()`，⛔ 沒有確認名稱目前解析到的仍是本程序建立的 inode——既有測試刻意製造了這條競爭並要求回 9，卻只斷言它沒有被 rename | ✅ 建立 staging 時記下它的 (dev, ino)；清理時 `remove_tree_at(…, expect=…)` 先比對，⛔ 不符就拋 `StagingReplaced`：⛔ 不刪任何東西、保留現場、結束碼 **9**（原本的結束原因附在訊息裡）。既有那支 barrier 測試補斷言：換進來的目錄 inode 與內容完整保留（修正前紅、修正後綠；反向驗證 R4） |

**⑦c 實作第二輪 review 的修正（2026-10-02）**：

| # | 問題 | 修正 |
|---|---|---|
| 中 | **staging 被搬走、同名路徑不存在時，仍回 8**：`remove_tree_at()` 對 `FileNotFoundError` 一律直接返回（即使呼叫端給了 `expect`），`_cleanup_staging()` 因此分不出「已清除」與「本程序建立的 staging 被搬走」——staging 在寫入途中被搬走、寫入再回 `ENOSPC` 時，沿用代表「可安全重跑」的 8，留下下落不明的 staging，也與 `development-workflow.md` 6b 列的「名稱已被換掉 → 不刪、9」不一致（review 以故障注入重現：`rc 8`、`active_name_missing true`、`moved_staging_preserved true`、`destination_absent true`）。⚠️ 這是第一輪「錯誤清理只刪自己的 inode」修正的缺口：只處理了「名稱指向別的 inode」，漏了「名稱不見了」 | ✅ 先寫測試、確認紅（搬走、刪掉兩種都是 `assert 8 == 9`）再修：`expect` 為 None（第 2 步清孤兒、遞迴刪子項目）時 ENOENT 維持冪等 no-op；給了 `expect`（本次的 staging）時 ENOENT 拋 `StagingReplaced` → ⛔ 不刪、保留現場、**9**（原本的結束原因附在訊息裡）。測試補兩支：第一次寫入時把 staging 搬走（或刪掉）並注入 `ENOSPC` → 9、搬走的目錄 inode 與內容完整保留、目的地不存在；`remove_tree_at()` 的 ENOENT 兩種語意。反向驗證 R5。計畫「二之三」6b 列、v29「八之三」的加註、`development-workflow.md` 6b 列、`sr-zone-scoring.md`「錯誤清理只刪自己的 inode」都已同步 |

**⑦c 實作第三輪 review 的修正（2026-10-02）**：

| # | 問題 | 修正 |
|---|---|---|
| 中 | **第二輪的修正只涵蓋「第一次 stat 時名稱已消失」**：stat 成功之後、開啟之前被搬走 → 開啟拋 `FileNotFoundError`；被換掉 → `ESTALE`；遞迴之後 rmdir 之前消失 → rmdir 的 `ENOENT`、被換成空目錄 → 直接刪掉換進來的目錄——都被清理端當成一般清理錯誤（或根本沒發現），只印訊息、維持原本的 8，違反「名稱消失或換掉 → 9」。第二輪的測試在 `_write` 時就搬走，清理開始時名稱早已不存在，沒有涵蓋這些窗口 | ✅ 先補 seam（`_lstat_at`、`_rmdir`，行為不變）與測試、確認紅（第三輪當時的九個變體全部 `assert 8 == 9`，既有 96 支照樣綠；第四輪再補五個，見下表）再修：有 `expect` 的清理改走 `_remove_own_staging()`——①清理之前核對；②開啟的 `ENOENT`／`ELOOP`／`ENOTDIR`、開啟之後 inode 不符 → `StagingReplaced`；③遞迴之後、rmdir 之前**再以 expect 核對一次**；④rmdir 的 `ENOENT`／`ENOTEMPTY` → `StagingReplaced`（⚠️ 漏了 `ENOTDIR`，第四輪 review 補上 `ENOTDIR`／`ELOOP`，見下表）；⑤（review 建議之外）照實的界線：rmdir 只能以名稱指定，最後一次核對之後才換成**空**目錄時 rmdir 會刪掉它（沒有內容遺失），所以 rmdir 之後以仍開著的 fd 的 `st_nlink` ≠ 0 發現本程序的 staging 還連在別處 → 9（host 的 ext4 與測試容器的 overlayfs 實測：刪掉的是自己時為 0、自己被搬走而刪掉換進來的空目錄時為 2）。`expect=None`（第 2 步清孤兒、遞迴刪子項目）的行為⛔ 不變。測試補三組九支（第四輪補到十四支；決定性的競爭：在 `_open_dir`／第二次 `_lstat_at`／`_rmdir` 被呼叫時搬走或換掉）：stat 與開啟之間（搬走／換成有內容的目錄／換成 symlink）、遞迴完成之後 rmdir 之前（搬走／有內容／空目錄——換進來的連空目錄也⛔ 不刪）、最後一次核對之後（搬走／有內容 → rmdir 的 errno；空目錄 → `st_nlink`；第四輪補 symlink／一般檔案 → `ENOTDIR`），全部斷言 9、搬走的 staging 的 inode 與內容完整保留、marker 不變、目的地不存在。反向驗證 R6～R10。⚠️ ⛔ 不在本輪範圍：staging **內部**的子項目在清理途中被換掉仍是一般清理錯誤（`ESTALE`，維持原本的結束碼）——它們在 `.promote-staging-*` 的名稱空間內，下一次 `--promote` 的第 2 步會整個清掉 |

**⑦c 實作第四輪 review 的修正（2026-10-02）**：

| # | 問題 | 修正 |
|---|---|---|
| 中 | **最後一次核對之後換成 symlink 或一般檔案，rmdir 回 `ENOTDIR`，仍回 8**：第三輪只把 rmdir 的 `ENOENT`／`ENOTEMPTY`／`EEXIST` 轉成 `StagingReplaced`，`ENOTDIR` 落入一般清理錯誤、維持原本的 8，與「清理途中任何時點被替換 → 9」不一致；最後窗口的測試只涵蓋搬走／有內容的目錄／空目錄，所以 105 支全綠仍抓不到 | ✅ 先實測：host 的 ext4 與測試容器的 overlayfs 上，以 `dir_fd` 對 symlink 與一般檔案 rmdir 都是 `ENOTDIR`（symlink 指向的目錄⛔ 沒有被動到）。先補測試、確認紅（最後窗口的 symlink、一般檔案兩支 `assert 8 == 9`）再修：rmdir 的替換型 errno 加入 `ENOTDIR` 與 `ELOOP`（與開啟階段的分類一致；`ELOOP` 在單一 component ＋ `dir_fd` 的 rmdir 實務上不會出現，⛔ 沒有決定性的測試，只為分類一致）。測試補五支：最後窗口的 symlink、一般檔案（review 要求；斷言 9、symlink 的目標與 inode 及 marker ⛔ 不被改動、搬走的 staging 保留、目的地不存在），另外為了三個窗口的變體一致，stat 與開啟之間補一般檔案、rmdir 之前的核對補 symlink 與一般檔案（這三支在第三輪的程式上就是綠的——那兩個窗口本來就以 inode 核對，不看類型）。反向驗證 R11 |

**實作途中自己發現、已修正**：

| # | 發現 | 處置 |
|---|---|---|
| 1 | 6a 的驗證模式綁定拿**目的地** inventory 的 SHA，而只有來源在第 4 步算過 SHA——每一次 6a 都會因綁定不符回 9 | 第一次跑測試之前讀碼發現；改成來源的 SHA（目的地剛與來源逐位元比對過，兩者相同） |
| 2 | `NoClobberUnsupported`（`EXDEV`／`EINVAL`／`ENOSYS`）是 `RuntimeError` 的子類，6b 只攔 `OSError`／`ValueError`——它會一路漏到 `main()` 的 catch-all（結果一樣是 9，但沒有照「結束碼的分類」明確歸類） | pytest 抓到；6b 明確攔 `PublishError` → 9 |
| 3 | ⚠️ **`e1cbbbd` 自己的 `test_exdev_is_fail_closed` 在新 tooling 下失敗**：計畫要把私有的 `_load_renameat2()` 改成帶 dirfd，但 ⑩ 的 replay 跑在 `e1cbbbd` ＋ tooling patch 上，`e1cbbbd` 的那支測試以兩參數的 fake 替換它；計畫同時要求 `e1cbbbd` 的既有測試全綠、斷言一律不改——兩條互相矛盾 | 實跑抓到（1 failed、1222 passed）。改成 `_load_renameat2()` **維持兩參數**、dirfd 版另由 `_load_renameat2_at()` 提供（下方「與計畫的差異」#1；計畫的「二之一」、決策 #20、「五」的對應列都已加註）；HEAD 的 `test_exdev_is_fail_closed` 也還原成與 `e1cbbbd` 逐位元相同；重跑 1223 passed |
| 4 | 反向驗證抓到兩支測試有遮蔽：①ignore 守門的各支不獨立——前一支失敗（P20）留下已晉升的目的地，後一支（只對目錄生效的對照組）改走 6a、根本沒查 staging 守門，P21 因此注不紅；②「本 repo（複本）內的路徑 → 1」借用了前面測試改過的 `evidence/`，它的合成守門本來就過不了，擋下它的⛔ 不是路徑規則 | ①每一支 ignore 守門的情境都先清掉真正 repo 的終態；②改用本 repo 內一份**合法**、名稱也對的 staging；兩者重做反向驗證都變紅（下表 C、D） |
| 5 | 測試自身：「inode 被換」的那支以「刪掉再寫」造新檔，檔案系統立刻重用同一個 inode 號碼，漂移因此看不出來 | 測試改成先建新檔再 `os.replace`（保證是不同的 inode） |

**檔案**：

| 檔案 | 內容 |
|---|---|
| `replay_bundle/publish.py` | `EXIT_PROMOTION_FAILED = 8`、`EXIT_PROMOTION_BLOCKED = 9`（唯一定義）；rename 三層：共用的私有 `_encode_rename_paths()`（路徑含 NUL → `ValueError`）與 `_raise_for_rename()`（errno 對應）、`rename_noreplace_at()`（單一 component 的名稱、`type(fd) is int and fd >= 0`；經 `_load_renameat2_at()`）、`rename_noreplace()`（完整路徑；經兩參數的 `_load_renameat2()`，既有五個呼叫點不變） |
| `replay_bundle/stage2_verdict.py`（新增） | B／C 判讀規則：`judge_row()`／`judge_comparison()`、封閉的輸出 schema、十一個 reason code |
| `replay_bundle/stage2_archive.py` | `Stage2Verified` 加 `comparison` 與 `manifest_sha256`（同一次讀取）；`_verify_failed_record_load()`（record 檔案的 SHA 取自同一次讀取，`verify_failed_record()` 介面不變）；`parse_promotion_target()`、`verify_promotion_target()`（信任根的綁定、`--judge`）；CLI 模式 `--verify-promotion-staging`（`--target`、`--judge`；三者重複即拒）；`patch_claims(mode="promotion")`；驗證模式的 stdout 是一行 canonical JSON |
| `python/scripts/i074-stage2-patch-claims.py` | `--promotion <path> --target <t>`（evidence 4 個 token、failed 5 個） |
| `scripts/finalize-stage2-evidence.sh` | 第八種模式 `--verify-promotion-staging`：真正 repo 由 origin 推導、兩種路徑、全部 `:ro`、⛔ 不接受 `--run-dir`／`--source-ref`／`TOOLING_PATCH`、failed 的語意 SHA ＝ target 名稱、結束碼只有 0／1（EXIT trap 收斂） |
| `python/scripts/i074_stage2_promote.py`（新增） | 晉升的第 2～7 步：fd 錨定、封閉 inventory、identity 的綁定、兩組 ignore 守門、驗證模式的綁定、收尾重算、鏈檢查、結束碼的分類；錯誤清理以 (dev, ino) 在每個時點確認只刪本次的 staging（實作第一～三輪 review） |
| `scripts/run-i074-stage2.sh` | stub 換成 `promote()`（第 1 步 ＋ 背景執行晉升模組；未定義的結束碼視為 9）；檔頭與結束碼說明 |
| `scripts/judge-i074-stage2.sh`（新增） | 判讀器的入口 |
| `.gitignore` | `python/baselines/i074_stage2/.promote-staging-*` |
| `python/scripts/fixtures/stage2_finalizer_argv.json` | 驗證模式三組 ＋ 兩個 placeholder |
| `python/baselines/i074_stage2/tooling_e1cbbbd.patch` | 依版控流程重產（`5ff9a90f…`） |
| 測試 | pytest：`test_i074_stage2_promote.py`（新增，110 項）、`test_i074_stage2_verdict.py`（新增，43 項）、`test_replay_stage2_archive.py`（驗證模式、`--judge`、promotion claims、CLI）、`test_replay_bundle_publish.py`（rename 三層）；shell：`test-replay-args.sh`（驗證模式 35 項、`.gitignore` 靜態 2 項）、`test-i074-stage2.sh`（晉升整合、真 git 的 ignore 守門、`git add -A` barrier、n9、n12 的晉升那一層、判讀器；原本預期 stub 9 的各支改成 0／6） |
| 文件 | `development-workflow.md`（CLI matrix、正式執行程序的晉升與判讀器、操作契約、⑦b／sizing 的狀態）、`sr-zone-scoring.md`（新增「I-074 Stage 2 的晉升與 B／C 判讀器」、⑦a 一節標成 review 通過）、本筆（已確認章節依「二之六」加註、計畫書標題與狀態列） |

**驗證**：

| 項目 | 結果 |
|---|---|
| `python/scripts/test.sh` 完整執行（依序；stage 之後） | ✅（⚠️ 實作第四輪 review 之後重跑，以這一次為準）391 秒；doc-refs 45／45、文件引用 0 個問題；`test-replay-args.sh` **379 項**全部通過（含 tooling patch 漂移測試：index 的 tree 重新產生的結果 ＝ index 中的 patch）；`test-i074-stage2.sh` **222 項**全部通過（含 host unittest 29 項）；pytest **1945 passed、1 skipped**（⑦b 為 1718；第一輪 review 修正補 4 支、第二輪補 3 支、第三輪補 9 支、第四輪補 5 支） |
| tooling patch | `5ff9a90fe33113ee0dcece3d121910daeadc3a3d5eeb6135f4b7939eeccf0b98`（247,656 bytes）；`--verify "$(git write-tree)"` ＝ 0；tooling 路徑上只有 `replay_bundle/` 的 `publish.py`、`stage2_archive.py`、`stage2_verdict.py` 有差異（`evaluation.py` 不變）；counterfactual 的 SHA（`fa7f5ba8…`）、T1（`a1649733…`）與語意 SHA（`5b851e1e…`）都不變，合成 SHA 變成 `ac49d36f…` |
| `e1cbbbd` ＋ counterfactual ＋ 新 tooling 的既有測試 | ✅ **1223 passed、1 skipped**（與 ⑦a 相同）；以 `replay_args_compose()` 建的 worktree 跑它自己的 `python/scripts/test.sh`（`SKIP_SHELL_TESTS=1`、`PY_IMAGE=stock-trading-python-e1cbbbd:test`）；tree `d7288222600d5637a457359b71991d5457ca7a3d`（＝ `T2`）、image `sha256:9fcb956ed0a903e11fd92ce4cb962b6615cd92a19b292421b42c0975753457c5`；doc-refs 35／35、文件引用 0 個問題。⚠️ 第一次執行 1 failed（上表 #3） |
| `REPLAY_SMOKE=1` 的 smoke（含反事實段） | ✅ 301 秒；切片 `b1_20260901_1d_de3ab843_a7c9ffb4`、tooling 由工作樹現行內容產生（247,656 bytes）；反事實 Stage 2 回 0；**before 165 列、cohort 6、comparison 6**、逐列翻轉；來源 bundle 未被修改 |
| 隔離 | ✅ `/run/lock` ⛔ 沒有任何 `i074-stage2.*`；`python/baselines/` 只有 tooling patch 改變；`test-i074-stage2.sh` 自己斷言真正 repo 的 worktree 登記數不變。⚠️ 完整 `test.sh` 每跑一次 ＋4（以時間戳對照：來自 ⑦a 以前的「結束碼 4 原樣傳出」、comparator、Stage 1 argv 三段——runner 走到 `exec docker run`、EXIT trap 不執行，[I-118](#i-118replay-相關腳本會洩漏-git-worktree註冊與-tmp-目錄都會累積) 記錄的既有增量），**⑦c 的測試 0 增加**；smoke 的一般段同樣留下 2 筆（實體目錄還在的 `/tmp` 暫時 worktree，已以 `git worktree remove` 逐一移除） |

**反向驗證**（逐項注回 → 對應測試變紅且紅在預期的那幾支 → 逐位元還原、比 SHA）：

| # | 注回的缺陷（計畫「六」第 2 項） | 結果 |
|---|---|---|
| P1 | 拿掉信任根綁定 | ✅ 紅：`trust_root`、CLI 回 1 那支 |
| P2 | 6a 比對不符改成覆寫（拿掉集合比對與第一次逐位元比對） | ✅ 紅：6a 的 bytes／extra／missing |
| P3 | 6b 失敗時不清 staging | ✅ 紅：6b 的 12 支 |
| P4 | 第 3 步改成取第一組 | ✅ 紅：兩組 |
| P5 | 判讀器改成多數決 | ✅ 紅：l、多數決、golden |
| P6 | 判讀器拿掉允許清單 | ✅ 紅：h、j、多項命中、golden |
| P7 | 判讀改成驗證之後重新讀 comparison | ✅ 紅：read-once 的 spy |
| P8 | inventory 與第 4 步改成會跟隨 symlink（⚠️ 只換第 4 步注不紅：3b 的 inventory 先擋下 symlink，所以連 inventory 一起改） | ✅ 紅：symlink 指向樹外的兩支 |
| P9（Run A） | 拿掉 `.gitignore` 的 staging 規則（合成 repo 的 `.gitignore` 改成空檔）＋ 關掉 staging 守門 | ✅ 紅：`git add -A` barrier 等 3 支 |
| P10 | 6b 改回完整路徑的 rename | ✅ 紅：barrier 的 repo／outside／rmrf 與 `i074_stage2` 那支 |
| P11 | 拿掉 rename 之後的鏈檢查 | ✅ 紅：barrier（repo、outside、`i074_stage2`）與鏈檢查之後換 staging 那支 |
| P12 | 拿掉 `st_nlink` 檢查 | ✅ 紅：hardlink |
| P13 | 第 4 步把開檔錯誤改回 3 | ✅ 紅：`ENOENT`／`ELOOP`／`ENOTDIR` |
| P14 | 拿掉驗證模式輸出的 SHA 綁定 | ✅ 紅：綁定不符那支 |
| P15 | `reasons` 改成第一個不符就停 | ✅ 紅：h、多項命中、golden |
| P16 | 拿掉收尾重算（6a、6b） | ✅ 紅：兩支原地改寫的 barrier |
| P17 | `rename_noreplace_at()` 拿掉名稱檢查 | ✅ 紅：12 支名稱規則 |
| P18 | `rename_noreplace()` 改成經 `_at()` | ✅ 紅：既有多層路徑的 19 支 |
| P19 | 拿掉兩組 ignore 守門 | ✅ 紅：5b、6b 的三支 |
| P20（Run B） | 守門加 `-v`、只看結束碼 | ✅ 紅：子目錄的否定規則（真 git） |
| P21（Run D） | 守門改成只查 staging 目錄本身 | ✅ 紅：只對目錄生效的對照組（⚠️ 第一次與 P20 合在 Run B 時被遮蔽，見上表 #4） |
| P22 | fd 檢查改成 `isinstance` | ✅ 紅：`True`／`False` 四支 |
| P23（＋ Run B） | 目的地那組守門改回只在 6b 查 | ✅ 紅：pytest 的 recovery 序列與 host 真 git 的 recovery |
| P24（Run B） | 判讀器改用真正 repo 的驗證入口（⛔ 不用暫存複本） | ✅ 紅：判讀器 n9（真正 repo 的 HEAD 已改壞驗證入口） |
| P25（Run C） | 驗證模式改成接受任意路徑 | ✅ 紅：真正 repo 外、本 repo 內（合法內容）、非直屬、名稱不符、目的地名稱 ≠ target（⚠️ 第一次「本 repo 內」那支沒紅，見上表 #4） |
| R1（review） | failed record 只在新建 `failed/` 時 fsync `i074_stage2/`（還原成 review 前） | ✅ 紅：「parent fsync 失敗 → 8 → 重跑在 rename 之前 fsync」 |
| R2（review） | rename 之後只 fsync 目的地的 parent | ✅ 紅：兩個 parent 的 `i074` 那一支 |
| R3（review） | 6a 只 fsync 目的地的 parent | ✅ 紅：6a 的 failed record |
| R4（review） | 錯誤清理不比對 inode | ✅ 紅：staging 在驗證時被換掉的 barrier（換進來的目錄被刪） |
| R5（第二輪 review） | 錯誤清理的 ENOENT 改回一律 no-op（給了 `expect` 也當成已清除） | ✅ 紅：搬走、刪掉兩支與 `remove_tree_at()` 的 ENOENT 語意那支（3 支）；同一次執行裡第一輪的兩支換 staging barrier 與 6b 寫入錯誤 4 支（`ENOSPC`／`EDQUOT`／`EIO` 仍 8、`EACCES` 9）照樣綠——修正⛔ 不會把正常的 8 變成 9 |
| R6（第三輪 review） | 開啟的 `ENOENT`／`ELOOP`／`ENOTDIR` ⛔ 不轉成 `StagingReplaced` | ✅ 紅：stat 與開啟之間的「搬走」「symlink」 |
| R7（第三輪 review） | 開啟之後 inode 不符改回一般的 `ESTALE` | ✅ 紅：stat 與開啟之間的「有內容的目錄」 |
| R8（第三輪 review） | rmdir 之前照樣 lstat、但⛔ 不比對結果 | ✅ 紅：遞迴之後 rmdir 之前的「空目錄」（換進來的空目錄被刪）；「搬走」「有內容」仍由 rmdir 的 errno 擋下（兩道互補）。⚠️ 第一版直接拿掉核對、三支都紅——但那是測試的觸發點（第二次 `_lstat_at`）跟著消失，⛔ 不能證明核對本身有作用；第二版只留呼叫、⛔ 不比對，「搬走」又因為注入的 lstat 自己拋 `ENOENT` 而紅；第三版吞掉那個例外之後才只紅在「空目錄」 |
| R9（第三輪 review） | rmdir 的 `ENOENT`／`ENOTEMPTY` ⛔ 不轉成 `StagingReplaced` | ✅ 紅：最後一次核對之後的「搬走」「有內容」 |
| R11（第四輪 review） | rmdir 的 `ENOTDIR`／`ELOOP` ⛔ 不轉成 `StagingReplaced`（還原成第三輪） | ✅ 紅：最後一次核對之後的「symlink」「一般檔案」；同一次執行的其他 23 支照樣綠 |
| R10（第三輪 review） | 拿掉 rmdir 之後的 `st_nlink` 檢查 | ✅ 紅：最後一次核對之後的「空目錄」 |

**與計畫的差異**：

| # | 差異 | 理由 |
|---|---|---|
| 1 | ⚠️ `publish.py`：`_load_renameat2()` **維持兩參數**（`AT_FDCWD`、完整路徑），dirfd 版是新的 `_load_renameat2_at()`；計畫的「私有 `_renameat2()`」實作成兩個公開函式共用的 `_encode_rename_paths()`（NUL）與 `_raise_for_rename()`（errno 對應）。HEAD 的 `test_exdev_is_fail_closed` ⛔ 不改（與 `e1cbbbd` 逐位元相同） | 上表 #3：計畫的兩條要求互相矛盾，選擇保住「`e1cbbbd` 的既有測試全綠、斷言不改」；三層的行為與測試不變。計畫的「二之一」、決策 #20 與「五」的對應列已加註 |
| 2 | inventory 以 `os.listdir(<目錄 fd>)` ＋ `os.stat(<名稱>, dir_fd=…, follow_symlinks=False)` 列舉（計畫寫 `os.scandir(<目錄 fd>)`） | 語意相同（⛔ 不跟隨 symlink）；`listdir(fd)` 每次都從頭讀，⛔ 不受同一個 fd 的目錄位置影響 |
| 3 | 晉升另加三道比對：複本 `git status` 列出的檔案 ＝ inventory 的檔案；驗證模式輸出的 `identity_sha256` ＝ `preflight.identity_sha256`、`base_commit` ＝ `state/run.json` 的 `repo_head` | 都是 fail-closed 的交叉核對（不符 → 9），⛔ 不放寬任何一道 |
| 4 | 一般檔案以 `O_NONBLOCK` 開啟 | inventory 之後被換成 FIFO 時開檔⛔ 不會掛住，之後的 `fstat` 照樣判 9；對一般檔案沒有作用 |
| 5 | 晉升模組有四個只給 pytest 用的掛勾（`after_inventory`、`before_copy`、`after_final_rehash`、`before_rename`）與可注入的 git／驗證模式／state 讀取 | barrier 與故障注入需要確定的時點；⛔ 沒有任何 CLI 或環境變數的覆寫口 |
| 6 | 驗證模式（shell）另拒絕 `TOOLING_PATCH`，並以 EXIT trap 把 `set -e` 漏出的其他結束碼收斂成 1 | 驗的程式碼必須是 HEAD 本身；結束碼只有 0／1 |
| 7 | 新建的 `failed/` 權限 `0o755`；staging 的目錄與檔案沿用來源的權限位元 | 計畫沒寫；git 只追蹤執行位元 |

**review 之後**（commit 由使用者決定）：commit 後再跑 `scripts/make-i074-tooling-patch.sh --verify "$(git rev-parse 'HEAD^{tree}')"`，並比對 HEAD 與
stage 時的 tree 在 `evaluation.py`、`replay_bundle/` 的物件 OID 相同——✅ **2026-10-02 已執行**（commit `5a03777` 之後）：`HEAD^{tree}` ＝ `48385e1dae5b42e07884a94d34ee8bb96277c182`（＝ 最後一次 stage 的 tree），`--verify` 通過（tooling patch 仍是 `5ff9a90f…`），`evaluation.py`（`6eefa111…`）與 `replay_bundle/`（`c5de32b0…`）的 OID 與 stage 時相同。⚠️ tooling patch 已改變：⑨-2 的正式 sizing（與它寫出的 freeze record）
必須以包含本包的 commit 為 `repo_head`。

**歸檔**（⚠️ 依 CLAUDE.md，本筆的計畫與結果保留到 review 確認後才收斂）：操作程序寫進 [`development-workflow.md`](./development-workflow.md)
「I-074 Stage 2 的正式執行程序」與 CLI matrix；契約與判讀規則寫進 [`sr-zone-scoring.md`](./sr-zone-scoring.md) 新增的「I-074 Stage 2 的晉升與 B／C 判讀器」。

#### Stage 2 步驟 ⑦d 細部計畫 v1（2026-10-02，✅ **已確認**（2026-10-02，review 六輪後通過並 commit `9af7942`））

⚠️ 依「Stage 2 步驟 ⑦ 總綱 v1」（✅ 2026-09-30 確認）拆出的第四包「memory harness」。範圍與驗收以總綱「三」的 ⑦d 列、v29「六、1」與
「八」的 ⑨-1 為準；總綱明定由本細部計畫決定的兩件事——**replay 程序如何產出 13,417 列而⛔ 不必跑三小時**、**⑩ 的實際峰值怎麼記錄
（⛔ 不改 ⑩ 的 docker argv）**——寫在「二之二」「二之六」，連同其他總綱沒寫到的細節列在「三」（⚠️ 待確認）。
⚠️ ⑦d **⛔ 不動 tooling 路徑**（`evaluation.py`、`replay_bundle/`）——tooling patch 維持 `5ff9a90f…`，漂移測試照跑。

**現況（2026-10-02 實查）**：

- ⑦c 已 commit（`5a03777`）：⑩ 的程式端到端完整（入口 → supervisor → 持鎖階段 → 複本內 orchestrator → replay → finalize／publish →
  晉升）；判讀器可用。⑦c 的 tooling patch 是 `5ff9a90f…`（247,656 bytes）；⑦b 的 sizing validation（真實 tooling、`353c69a0…`）量到
  **`P_B` 152.4 MiB**（預算 160 MiB），之後 ⑦c 讓 tooling 與 HEAD 都變大（HEAD 的已追蹤檔 34.4 → 34.8 MiB），**還沒有重量**。
- replay 程序：`_run_counterfactual_stage2()` → `_replay_from_bundle()`（`load_bundle`、11 檔 candles 的 DataFrame、`load_model`、quota、
  universe）→ **`_decision_replay_rows()`——唯一需要約 180 分鐘的計算**（11 檔、13,417 列，回傳整份 list）→ row-level 守門、after 的一趟
  串流、全量 key 守門、`CounterfactualEffectCheck`、三檔輸出（或 rc=6 的中繼檔）。`main()` 以 `sys.argv[1:]` 當 argv（也寫進 provenance）；
  `_replay_from_bundle()` 以**模組全域名稱**呼叫 `_decision_replay_rows`。
- 記憶體的歷史實測：「③ 第一次執行」的計算過程 RSS 穩定在 190～205 MiB（`MEM=350m`、29 分鐘、約 2,100 列）、全量結束時（含整份 rows）
  約 195 MiB；③c 的 after' 見證趟（完整計算、178 分鐘、mem-guard 549m）峰值**約 315 MiB——每 5 分鐘取樣，是下界**。兩者都⛔ 不是
  Stage 2 反事實路徑的整個程序，也都⛔ 沒有以容器內的 wrapper 量到精確峰值。
- **計次政策**（「正式 scan 的計次裁決」v26／v27，✅ 使用者裁決）：跑完並留下 artifact 的 replay 最多 **5 趟**（D、D+1、after'、before、
  before 重跑）、物理啟動最多 **8 趟**——⛔ 沒有給量測用的完整 replay 留任何額度。
- **v29「八之一」的容量列**（✅ 已確認）：`P_B` 的起訖是「replay 開始 → 複本內發布完成」；晉升有自己的空間預檢（來源 allocated bytes 的
  2 倍）、失敗回 8 可以重試、⛔ 不會失去 replay，所以⛔ 不算進 `P_B`。
- sizing harness（④、⑦b）：以 **fixture 容器**模擬 replay 的寫檔（⛔ 不跑 runner 與 `evaluation.py`）；shim 一律加 `--read-only`；
  helper 的 phase／role 是 sizing 專用的封閉列舉、`load_invocations()` 要求 `SizeRw == 0`、`build_report()` 寫死 sizing 的步驟；
  `P_B_BUDGET`（167,772,160）唯一定義在 `i074_stage2_preflight.py`；450 MiB 的門檻在程式裡還沒有定義。
- ⑩ 的 preflight（⑦b）在容器內跑兩個程序：`anchors`（`i074_stage2_preflight.py anchors`，只做 `_load_trust_anchors()`）與
  `--check-failed-record`（`check_failed_records()`：同樣先 `_load_trust_anchors()`，再逐份驗 failed record）。
- 晉升（⑦c）：`i074_stage2_promote.py` 是 host 程序（⛔ 不在 cgroup 裡），複製與 hash 以 1 MiB 分塊、只有 identity／record 那一個小檔整份讀；
  它從 state 只讀 `preflight` 的 `bundle_id`、`counterfactual_semantic_sha256`、`identity_sha256`，`replay_done.rc` 與 `run.repo_head`；
  驗證模式由複本的 `finalize-stage2-evidence.sh --verify-promotion-staging` 在容器內執行（經 PATH 上的 docker），路徑必須在**真正 repo**
  （複本的 `origin`）內。`Promoter` 的建構子可以注入 `load_states`（⑦c 為 pytest 留的；⛔ 沒有 CLI 或環境變數的覆寫口）。
- ⑩ 的環境：supervisor 以 `clean_env()`（`ENV_DROP_PREFIXES`／`ENV_DROP_NAMES`）清理、PATH 固定成 `<work>/bin:/usr/bin:/bin`、
  帶 `PYTHONDONTWRITEBYTECODE=1`；兩份 patch 的環境變數只給 runner 那一次。
- host：cgroup v1（cgroupfs driver）、Docker Root Dir `/var/lib/docker`；容器的 `memory/docker/<id>/memory.max_usage_in_bytes`
  **uid dev 從 host 讀得到**，但容器一結束 cgroup 就被移除。`run-evaluation.sh` 記過實測教訓：從外面輪詢會**系統性漏掉尾段峰值**。
- ⚠️ **自己發現（sizing 模型的缺口）**：runner 把兩份 patch 凍結在私有的 `mktemp -d`（`TMPDIR` 底下），清理靠 EXIT trap——但它以
  `exec docker run` 結束，trap ⛔ 不會執行，**凍結副本留在 L3**（約 0.25 MiB，與 I-118 的 worktree 同一個成因）。sizing 以 fixture 取代
  runner，三種量法都⛔ 沒有看到它；⑤／⑦b 的 `P_B` 因此少算這一份。

**⑦d 細部計畫 v1 第六輪 review 的修正（2026-10-02）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | **nonce 不是「本次 snapshot pass 建的」的可信證明**：手動呼叫者可以讀或建 `BOOTSTRAP`、再給相同的環境 nonce，re-exec 分辨不出來，「手動設定的 `SIZING_SNAPSHOT` 一律⛔ 不刪」因此不成立；而且先比 inode 再 `rm -rf` 仍是 check-then-delete | ✅ 採 review 提出的**保守方案**：**bootstrap 階段（snapshot pass，以及 re-exec pass 完整驗證通過之前）⛔ 不刪任何東西**——失敗或收到訊號只印出 S 的位置、結束碼 1／130／143，接受少量 tmpfs 殘留；S 的刪除只發生在 re-exec 完整驗證通過之後的正式流程（六步清理與正常結束，沿用 sizing 既有的做法）。nonce 與 `BOOTSTRAP` 一併拿掉。⚠️ 沒有採推薦的「繼承 fd 當清理權限」：實測 bash 的 `exec 7<目錄` 可以跨 `exec` 繼承（2026-10-02），但 fd 錨定的刪除要 `unlinkat`／`rmdir(dir_fd)`，只能以 Python 寫、又⛔ 不能取自活路徑，兩個入口都要各內嵌一份；最後以名稱 `rmdir` S 本身仍有 ⑦c 記過的殘餘競爭；手動呼叫者同樣可以自己開 fd 交棒——為了清掉偶發失敗留下的約 100 KB tmpfs，不值得。⚠️ 照實寫明：手動準備、而且**通過完整驗證**的快照（內容就是一份合法的快照）與正常交棒⛔ 無法分辨，之後的正式流程結束時會照常清掉它（「二之三」的「bootstrap 的失敗與中斷」列）。測試補「既有目錄 ＋ 完整的合法內容 ＋ 之後的驗證失敗」⛔ 不刪（ac18） |
| 低 | ac19 把 validation（dirty）與 freeze record（只限 formal ＋ clean ＋ ok）寫在同一條 | ✅ 拆成 ac19（validation、逐檔修改：只驗 `harness_manifest` 與報告 SHA，並斷言⛔ 沒有 freeze record）與 ac19b（clean 的 formal fixture：新欄位存在、freeze record 照常寫出並通過 `check-pair`） |

**⑦d 細部計畫 v1 第五輪 review 的修正（2026-10-02）**：

| # | 問題 | 修正 |
|---|---|---|
| 中 | **snapshot bootstrap 的失敗清理沒有定義**：共用原語負責 S 與六步清理，但 snapshot pass 必須先建 S、複製共用原語再 re-exec——複製或寫 `MANIFEST` 失敗、`--formal` 的前後驗證失敗、re-exec 一開始的驗證失敗、bootstrap 期間收到 INT／TERM，全都發生在共用原語與正式 trap 生效之前；ac18 也只驗拒絕、⛔ 沒有驗 S 被清掉 | ✅ 「二之三」新增「bootstrap 的失敗與中斷」列：**兩個入口各自負責 minimal bootstrap**（⛔ 不 source 任何活路徑的檔案）——以 exclusive 的 `mkdir -m 700` 建 S、**立刻**裝 bootstrap trap，記下 S 的 (dev, ino) 與一個 nonce（寫進 `<S>/BOOTSTRAP`）；`exec` 成功之前任何失敗或 INT／TERM → 只在 S 仍是本次建立的那個 inode、owner ＝ 自己、realpath ＝ 預期路徑、tmpfs 時才 `rm -rf`（否則⛔ 不刪、印出位置）；re-exec pass 的**第一個動作**就是接管 trap，再做 realpath／tmpfs／owner／mode／nonce／`MANIFEST` 的驗證——驗證失敗時只有在 S 帶著本次的 nonce（`SIZING_SNAPSHOT_NONCE` ＝ `<S>/BOOTSTRAP`）才刪，手動設定的 `SIZING_SNAPSHOT` 一律⛔ 不刪；驗證通過才 source 快照裡的共用原語、換成六步清理。共用原語只負責 re-exec 之後的正式流程。ac18 補 copy、`MANIFEST`、`--formal` 的後驗、re-exec 的驗證、INT／TERM 各自失敗之後⛔ 沒有殘留的 S。⚠️ 第六輪 review 訂正：nonce ⛔ 不是可信的證明、inode 比對之後再刪仍是 check-then-delete——改成 bootstrap 階段⛔ 不刪任何東西 |
| 低 | **報告沒有綁住完整的快照清單**：只列 harness／shim／helper／launcher 的 SHA，漏了 `i074-stage2-measure.sh`、sizing 的 `mem-guard.sh` 與 `MANIFEST` 本身；validation 允許 dirty，單看報告無法確定實際執行的檔案集合 | ✅ 兩種報告都加**封閉的 `harness_manifest`**（相對路徑 → SHA-256，集合恰好是本 profile 的清單 ①）與 `harness_manifest_sha256`（它的 canonical SHA），原始 `MANIFEST` 隨原始量測保存；freeze record 已綁整份報告的 SHA、而且只讀報告的特定欄位（實查 `cross_check_report()`），⛔ 不擴充它的 schema。測試逐一改動清單上的每個檔案，斷言報告的值跟著變（ac19） |

**⑦d 細部計畫 v1 第四輪 review 的修正（2026-10-02）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | **快照清單不完整，sizing 仍無法完全脫離活路徑**：清單只有 acceptance 的五個檔案，卻要求 sizing 也 `exec` 快照版本——漏了 `scripts/i074-stage2-sizing.sh` 本身、它直接 source 的 `scripts/lib/mem-guard.sh`（實查第 42 行），而 `i074_stage2_freeze_record.py` 在量測結束後仍從 `REPO_ROOT` 執行（實查第 456 行）——指原始 repo 就仍有 TOCTOU，指快照則快照裡沒有它與它載入的 `i074_stage2_preflight.py` | ✅ 改成 **profile 各自的清單**（「二之三」新增的「快照與來源的清單」表）：逐一列出從快照執行的檔案（sizing 加入主腳本與 `mem-guard.sh`）、從工作複本（HEAD）執行的正式程式、原始 repo 只用於哪些 git／inventory 驗證、快照前後要驗的檔案，以及 snapshot pass 與 re-exec pass 的判別（⛔ 不遞迴）。`i074_stage2_freeze_record.py` 的 `build`／`check-pair` 改從**工作複本**執行、`--repo <工作複本>`（它只以 git 物件讀 `repo_head` 中的檔案內容，工作複本有同一份物件；它載入的 `i074_stage2_preflight.py` 也是工作複本的）。實查：sizing helper 在 host 只用標準庫、`anchor-base` 與 `replay-args.sh` 本來就取自工作複本，⛔ 沒有其他漏網的活路徑 |
| 低 | ac17 仍要求「shim 掛載的 launcher ＝ 啟動 harness 的 repo 的檔案」，與第三輪的快照衝突 | ✅ 改成：快照的內容取自啟動 harness 的 repo；shim 實際掛載的一定是 `<S>/harness/…`；快照的 SHA ＝ 當次凍結的版本；工作複本⛔ 不是 launcher 的來源 |
| 低 | 「七」的風險表仍寫「最大單一程序 RSS ＋ 取樣群組總和一起守門」 | ✅ 改成：最大單一程序 RSS 是正式門檻；取樣群組總和只提供單向超標警報，低於 450 MiB ⛔ 不構成通過的證據 |

**⑦d 細部計畫 v1 第三輪 review 的修正（2026-10-02）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | **harness 自己的程式沒有凍結（TOCTOU）**：`--formal` 只在開始時驗 harness 檔案 ＝ HEAD，之後 helper、shim、launcher 一直從啟動 harness 的工作 repo 讀（launcher 甚至以活路徑 bind mount），而 full 排在最後、整趟可能三小時——中途被改，success、failure、full 可能跑不同版本，報告卻記錄開始時的 SHA，「改了又還原」也抓不到；bash 本身也是邊讀邊執行腳本 | ✅ **步驟 0 最前面建快照**（「二之三」的「harness 的快照」列）：建 S 之後把 harness 自己的檔案複製到 `<S>/harness/`（tmpfs、`0700`），`--formal` 在複製**之前**驗原檔 ＝ HEAD、複製**之後**再驗快照 ＝ HEAD（抓到兩者之間的修改），validation 則把工作樹的版本凍結成快照；主腳本隨即 `exec` 快照裡的自己，之後共用原語、helper、shim 一律從快照載入，launcher 掛載快照的路徑，報告記錄快照的 SHA-256。sizing 共用同一套原語、⑨-2 的 `--formal` 有同樣的風險，所以**一併沿用**（「三」#22）。測試在快照建立之後修改與刪除原始的 launcher、helper、shim，斷言執行與報告都用快照（ac18） |
| 中 | **取樣的程序群組 RSS 總和是下界，不能當成通過的證明** | ✅ host 端的**門檻**只用 `RUSAGE_CHILDREN` 的**最大單一程序 RSS**（每個已 wait 的程序各自的高水位，精確）——v26 的契約本來就是「**每一個程序**各自 < 450 MiB」；取樣的群組總和降為**單向警報**：觀察到 ≥ 450 MiB → `threshold_exceeded`，< 450 MiB 只記成「未觀察到超標」，⛔ 不當成通過的證據。⚠️ 照實寫明：整個 host 程序群組的實際總峰值⛔ 不在契約內、也量不到（⛔ 沒有可寫的 cgroup）（「二之三」「二之五」、「三」#21） |
| 低 | 表格與測試沒有同步：「四」的 helper 漏列 `host-run`；ac6 只寫「容器與 promoter 各一」；ac16 沒有驗「同時存活的子程序」確實相加 | ✅ 「四」補 `host-run`；ac6 分別釘住容器 cgroup 峰值、host 最大單一程序 RSS 的邊界，以及群組總和的單向警報；ac16 補兩個同時存活、各自配置已知大小記憶體的子程序 |

**⑦d 細部計畫 v1 第二輪 review 的修正（2026-10-02）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | **「只換 counterfactual 時⛔ 不需要重跑」直接違反 v29**：v29「八」的 ⑨-1 明定兩份 exact patch 任一份的 bytes 或 SHA 再變都要重跑；counterfactual 也影響 runner 合成的 worktree 與凍結副本的磁碟量、stub 兩條路徑的正式組合，以及 ⑩ 真正執行的程式 | ✅ 刪掉那一句（「二之七」計次列、「七」的量測趟列）：**兩份 patch 任一份再變，⑨-1 一律整個重跑（含量測趟）**；量測趟的額度只有 1 趟，要重跑時額度另行裁決（⛔ 不自動取得）——所以 ⑨-1 照 v29 的既有順序只在 ⑨ 封存之後執行。⛔ 不另立「沿用三小時結果」的拆分契約。「二之二」full 模式少了 counterfactual 的那段說明也限縮成「只就本次封存的 patch 而言」 |
| 中 | **commit 前的 validation 找不到新的 launcher**：shim 從工作複本掛載 `python/scripts/i074_stage2_replay_stub.py`，但工作複本是 `git clone` 的 HEAD，本包新增的檔案在 commit 之前不在裡面 | ✅ 比照 sizing（它的 shim、helper、fixture 本來就取自啟動 harness 的 repo）：**harness 自己的檔案**（acceptance 腳本、共用原語、shim、helper、launcher）一律取自**啟動 harness 的 repo**——validation 允許工作樹的版本、報告記錄 SHA-256，`--formal` 由守門驗它們 ＝ HEAD；**⑩ 的正式程式**（runner、finalizer、promoter、`replay_argv()`、`clean_env()`、`P_B_BUDGET`）一律取自**工作複本**（HEAD），helper 以明確的 `--clone` 路徑載入它們（「二之三」第一張表的「檔案的來源」列、「三」#11、#20）。⚠️ 第三輪 review：harness 自己的檔案再改成步驟 0 凍結的快照（⛔ 不用活路徑） |
| 中 | **promoter 的記憶體量測漏掉子程序**：只取 `RUSAGE_SELF`，但 promoter 會啟動 git、驗證模式的 shell 與 Docker client | ✅ 改成**每一步**都經 `host-run` 子指令執行（晉升不再特例）：記 host 端程序樹的**最大單一程序 RSS**（`RUSAGE_CHILDREN`——所有已 wait 的子孫裡最大的一個，含 promoter 自己在內）與每 0.1 秒取樣的**程序群組 RSS 總和**（下界），兩者都 < 450 MiB；照實標示 RSS ⛔ 不含 page cache、與容器的 cgroup 峰值是不同的量法，而且 host 端⛔ 沒有可寫的 cgroup，程序樹的精確峰值量不到（「二之五」、「三」#21）。⚠️ 第三輪 review 訂正：門檻只用最大單一程序 RSS，群組總和改成單向警報 |

**⑦d 細部計畫 v1 第一輪 review 的修正（2026-10-02）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | **記憶體驗收沒有涵蓋真正的 replay 計算**：launcher 換掉 `_decision_replay_rows()` 之後，計算工作集（pandas、zone 計算、模型推論）完全沒有被執行，載入等量的 JSON 證明不了它是計算工作集的保守上界；計畫與 v29 卻仍把結果稱為完整 replay 程序的 < 450 MiB 正式驗收 | ✅ **使用者裁決（2026-10-02）：⑨-1 加一趟量測趟**——⚠️ 原本的建議「⑨-1 跑一次未 patch 的完整 replay」與已確認的計次上限（v27：跑完 5 趟、物理啟動 8 趟）衝突，所以先請使用者裁決。做法（「二之二」）：⑨-1 以 `e1cbbbd` ＋ tooling、**⛔ 不套 counterfactual** 跑一次完整計算（算出的列與錨定的 D+1 相同，⛔ 不會在 ⑩ 之前得到任何 B／C 資訊），launcher 在真實計算之後才套 success 的合成、驅動反事實尾段——同一個程序涵蓋計算與尾段，以容器內的 wrapper 量精確峰值；計次政策另立一格「⑨-1 量測趟」（「二之七」）。快速 launcher 保留，只作 ⑦d 的開發驗證與兩條磁碟路徑；`--formal` 必須帶完整計算（「三」#19） |
| 高 | **磁碟路徑漏了 ⑦c 的晉升**：⑩ 是 replay → finalize／publish → 終態 → 晉升，晉升會在真正 repo 的 staging 再複製一份終態；餘裕不大，不能省略 | ⚠️ 依規格查證：v29「八之一」的容量列（✅ 已確認）明定晉升⛔ 不算進 `P_B`（自己的空間預檢、失敗回 8 可重試、⛔ 不失去 replay）——把它納入 `P_B` 等於改已確認的決策，所以先請使用者裁決。✅ **使用者裁決（2026-10-02）：實跑晉升、`P_B` 的定義不變**——acceptance 以兩層複本（真正 repo → 模擬的真正 repo → 工作複本）實際跑一次隔離的晉升（success 與 failure 各一）：promoter 自身的記憶體與驗證模式容器的記憶體都**實量**並納入 < 450 MiB（取代原本的代量 #7）；晉升的 staging 磁碟另列為**資訊值**（`P_path ＋ 晉升`），⛔ 不與 `P_B_BUDGET` 比較（「二之三」「二之五」） |
| 中 | **`runner_frozen_patches` 只在 acceptance 計入**：已確認是正式路徑的實際占用，⑨-2 的 sizing 若繼續省略就是已知的低估 | ✅ acceptance 與 sizing **兩邊都計入**（sizing 依路徑以步驟 0 實建同形狀的目錄量）；⑤ 的數字保留為歷史紀錄並註明當時的模型沒有這一項（「三」#15、「二之七」） |
| 低 | 「sizing 的輸出逐位元不變」不成立——改了 harness、shim、helper，報告記錄的來源 SHA 一定會變（何況 accounted 也要加一項） | ✅ 改寫成：**既有的 CLI、改寫後的 argv 與 profile 未設定時的行為不變；accounted 依「三」#15 多一個組成；來源檔的 SHA 隨本次異動正常更新** |
| 低 | observer 沒有標示觀測的完整度——短命容器可能完全沒被取樣，沒看到 replay 容器的報告也會被誤認為完整 | ✅ 報告加 `replay_seen`、`observation_complete`、`missing_expected_containers`；靜態測試釘住 observer 的 label 鍵 ＝ label shim 與 orchestrator 的常數（「二之六」） |

##### 一、目標與⛔ 不做

| 項目 | 內容 |
|---|---|
| 目標 | ① `scripts/i074-stage2-acceptance.sh`：在 repo 外的隔離複本，以**真實的 runner、`evaluation.py`、finalizer 與晉升**跑 success／failure 兩條實際流程，量 v29「六、1」：replay 程序（a～c；⚠️ 計算工作集由 ⑨-1 的量測趟涵蓋）與 d～i 各程序、以及晉升（promoter 與驗證模式）的記憶體峰值各自 **< 450 MiB**；兩條流程的磁碟峰值各自 **≤ `P_B_BUDGET`**（量法與 `P_B` 相同；晉升另列資訊值）；② ⑩ 的實際峰值紀錄（observer，⛔ 不改 ⑩ 的任何程式與 docker argv）；③ 與 label shim 互斥；④ sizing 的 accounted 補上 `runner_frozen_patches` |
| ⚠️ 只做開發驗證 | ⑦d 只以 validation 模式、快速 launcher 實跑；**⛔ 不在真資料上跑完整計算**（那就是 ⑨-1 唯一的量測趟）；**正式驗收是 ⑨-1**（用 ⑨ 封存的兩份 patch，`--formal` ＋ 完整計算） |
| ⛔ 不做 | ⑨ 的 guard 與封存、⑨-1、⑨-2、⑩、⑪；witness 路徑（③c 已完成，v29「六、1」：⛔ 不重量）；⛔ 不改 `evaluation.py`、`replay_bundle/`、`run-replay-offline.sh`、`finalize-stage2-evidence.sh`、`run-i074-stage2.sh`、supervisor、label shim、`i074_stage2_preflight.py`、`i074_stage2_promote.py`；⛔ 不改 `P_B` 的定義、`P_B_BUDGET`、`M_safety`（開發驗證若超過預算，照實回報、依 v29「六、1」的回退順序由使用者決定） |

##### 二、設計

###### 二之一、檔案與角色

| 檔案 | 執行環境 | 角色 |
|---|---|---|
| `scripts/i074-stage2-acceptance.sh`（新增） | host bash | 編排：模式、守門、兩層複本、兩條流程、兩次晉升、只量記憶體的程序、twin、報告（「二之三」） |
| `scripts/lib/i074-stage2-measure.sh`（新增，**自 sizing 抽出**） | host bash（被 source） | 共用的 shell 原語（⚠️ 第五輪 review：只負責 **re-exec 之後**的正式流程；建 S 與快照由兩個入口自己的 bootstrap 負責）：process group 的收尾（TERM → KILL → 再確認）、以 CID 清容器、失敗時的六步清理、`begin_phase`／`end_phase`／`step`、work 目錄防護、與 label shim 互斥的啟動守門 |
| `scripts/i074-stage2-sizing.sh`（修改） | host bash | 開頭建快照並 `exec` 快照版本、改成 source 上一列；`i074_stage2_freeze_record.py` 改從工作複本執行（「二之三」的清單）；步驟 0 另實建 runner 凍結副本的同形狀目錄（「三」#15） |
| `scripts/lib/i074-sizing-docker-shim.sh`（修改） | host bash | `SIZING_PROFILE=acceptance`：⛔ 不加 `--read-only`；role `replay` 把容器指令換成 launcher、完整計算時把 `/app` 的來源換成不套 counterfactual 的 worktree（「二之四」）；profile 未設定 ＝ sizing |
| `python/scripts/i074_stage2_sizing.py`（修改） | host 3.9 ／ 容器 3.11 | 量測原語以 profile 參數化封閉列舉；sizing 的 accounted 加 `runner_frozen_patches`；新增 `acceptance-report`、`replay-argv`、`clean-env`、`promote-measure`、`host-run`、`observe` 子指令 |
| `python/scripts/i074_stage2_replay_stub.py`（新增） | 容器（Stage 2 image、3.11） | replay 的 launcher（「二之二」）；⛔ 不在 tooling 路徑 |

###### 二之二、replay 程序的兩種計算模式（「三」#1、#19）

| 模式 | 用在哪裡 | `_decision_replay_rows()` | 涵蓋 |
|---|---|---|---|
| **stub**（快速，預設） | ⑦d 的開發驗證；兩條磁碟路徑（success、failure）在任何模式都用它 | 換成「讀錨定的 D+1 after、依原順序收成 list」 | rows（13,417 列、整份常駐）、尾段、runner、mem-guard、檔案大小——⚠️ ⛔ 不含計算工作集 |
| **full**（⑨-1 的量測趟，約 3 小時） | 只在 `--replay-compute full`；`--formal` 必須帶它 | **真的執行**（`e1cbbbd` ＋ tooling、⛔ 不套 counterfactual），之後才套 success 的合成 | 計算工作集 ＋ 整份 rows ＋ success 尾段，**同一個程序**、容器內 wrapper 的精確峰值 |

| 項目 | 規則 |
|---|---|
| launcher | `python/scripts/i074_stage2_replay_stub.py`，只在 acceptance 的 replay 容器內執行，由 shim 以唯讀 bind mount 放在 `/acceptance/replay_stub.py`——⚠️ 取自**啟動 harness 的 repo** 在步驟 0 建的**快照**（第二輪 review：工作複本是 HEAD，commit 之前沒有它；第三輪 review：⛔ 不掛活路徑，見「二之三」的「harness 的快照」列）。`sys.path.insert(0, "/app")` → `import backtest.modular.sr_scoring.evaluation as ev` → **只**指派 `ev._decision_replay_rows`（⛔ 不動任何其他屬性）→ `sys.argv = ["evaluation", *argv]`（argv 逐 token ＝ runner 組出的容器參數，所以 provenance 的 `argv` 與 ⑩ 相同）→ `ev.main()`，結束碼照 `main()`（0／6／1） |
| 模式與計算 | 容器環境變數 `I074_ACCEPTANCE_REPLAY`（`success`／`failure`）與 `I074_ACCEPTANCE_COMPUTE`（`stub`／`full`），由 shim 依 harness 的 `SIZING_REPLAY_MODE`／`SIZING_REPLAY_COMPUTE` 注入；缺少或其他值 → rc=1、⛔ 沒有輸出；`full` 只接受 `success` |
| stub 的列 | argv 裡的 `--after-artifact`、`--cohort-manifest`（各恰好一次，否則 rc=1）——也就是 ⑩ 用的錨定 D+1 after 與 cohort；以 `stream_canonical_artifact()` 逐列讀（canonical 與 SHA 照驗），依原順序收成 **list**；呼叫參數全部忽略 |
| full 的列 | 以原本的呼叫參數呼叫**原本的** `_decision_replay_rows()`；⚠️ 回傳之後先斷言 cohort 的 156 列全部是候選（`CONTINUATION`、`setup_rr_qualified=false`、flag 為 true）——證明容器裡的程式碼⛔ 沒有套 counterfactual（否則 rc=1）；再套 success 的合成 |
| success 的合成 | cohort 的 156 列改成 `lifecycle_phase="TESTING"`、`rr_decoupling_candidate=False`（與 sizing fixture 相同），其餘逐列不變 → 反事實生效 → rc=0、恰好三檔 |
| failure | 逐列不變 → 156 列候選 → `rr_not_restored` → rc=6、只有 `bounded_diagnostics.json` |
| full 的程式碼來源 | 步驟 0（量測窗口之外）以 `replay_args_compose(<wt>, <base>, "", <tooling>)` 建 `e1cbbbd` ＋ tooling 的 worktree（`<work>/nocf`）；shim 只在 `full` 把 runner 的 `-v <runner 的 worktree>/python:/app:ro` 換成 `-v <work>/nocf/python:/app:ro`（恰好一個，否則 125）。runner 本身照常凍結、合成、注入（provenance 照 ⑩），只有容器內執行的程式碼少了 counterfactual 的兩個產品檔改動 |
| 仍然是真的 | `load_bundle()`、11 檔的 DataFrame、`load_model()`、quota、universe、`keys`、`validate_replay_errors`、`validate_diagnostics`、provenance、after 的一趟串流、全量 key 守門（有序）、`CounterfactualEffectCheck`、before source 的串流寫入、`before_by_key`、comparison、落地重讀、report；runner 的凍結、worktree、合成、注入參數與 mem-guard |
| ⚠️ 照實的殘餘 | stub：計算工作集⛔ 沒有被執行（由 full 涵蓋）；rows 由逐列 JSON 解析而來（鍵與字串⛔ 不共用物件），佔用預期 ≥ 真正的 rows。full：**本次封存的** counterfactual 只改 `decision_engine.py`、`lifecycle_engine.py` 裡 RR 的判定，計算的資料量與結構相同——記憶體上視為等價、照實寫明（⚠️ 只就本次的 patch 而言，⛔ 不推廣到未來的 patch：任一份 patch 再變就整個重跑 ⑨-1，「二之七」）；failure 路徑的計算與 success 相同、尾段更輕，⛔ 不再另跑一趟完整計算 |

###### 二之三、acceptance harness 的流程

⚠️ **⑦d 增補（✅ 2026-10-05 確認）**：bootstrap 經 python3 啟動器設 subreaper 再 exec 快照裡的主腳本；建 work 目錄之後先跑 `measure_subreaper_guard`（行為驗證）與能力檢查（cgroup v1 的 `total_rss`），之後才 clone；每一步之後 `check-step` 與收養檢查（`reap-adopted --check-only`），正常結束之前再檢查一次。

```text id="i074_stage2_acceptance_flow_001"
REPLAY_IMAGE_ID=sha256:… scripts/i074-stage2-acceptance.sh --work-dir <repo 外、尚不存在的目錄> [--replay-compute stub|full] [--formal]

0. 準備（守門與做法同 sizing 的步驟 0；profile＝acceptance）
   ⚠️ 最前面（第三輪 review）：建 S → harness 自己的檔案複製到 <S>/harness/（--formal：複製前驗原檔 ＝ HEAD、複製後驗快照 ＝ HEAD）→ exec <S>/harness/ 裡的主腳本；
   之後的每一行都在快照裡執行
   守門：REPLAY_IMAGE_ID 必填、⛔ 不得與 PY_IMAGE 並存；環境帶 I074_STAGE2_* → 中止；PATH 上的 docker 是 label shim 或 /dev/shm 下的 shim → 中止；
         work 目錄以 canonical path 判斷、⛔ 不得在 repo 內、⛔ 不得已存在；--formal：必須 --replay-compute full、scripts/、python/、.gitattributes clean、
         「四」的 harness 檔案 ＝ HEAD、fd 1／2 ⛔ 不得導到 L0 上的一般檔案
   S＝/dev/shm/i074-accept-<run id>/（必須是 tmpfs）
   兩層複本：git clone --no-hardlinks <repo> <work>/real（模擬的真正 repo）→ git clone --no-hardlinks <work>/real <work>/repo（工作複本；origin＝<work>/real）
   L1＝<work>/runs、L2＝工作複本的 python/baselines/i074_stage2/、L3＝<work>/tmp、L4＝Docker Root Dir（只驗 st_dev ＝ L0）、L5＝工作複本的 .git、
   L6＝<work>/real 的 python/baselines/i074_stage2/（晉升的 staging 與目的地，只在晉升的窗口量）
   記 Stage 2 identity 檔的 SHA-256（結束時再驗一次）
   兩份 patch：工作複本的常數路徑（與 ⑩ 相同）；量測窗口之外以 replay_args_compose() 合成一次，raw ＝ 增量 canonical
   anchor 資訊（bundle_id、base_commit、after／cohort 路徑）：helper 在 host 取（⛔ 不在量測窗口內）
   replay argv：`replay-argv` 子指令呼叫 i074_stage2_preflight.replay_argv()（唯一的組裝處），再把 <W>/repo、<W>/run/stage2 對應到工作複本與本路徑的 run 目錄
   步驟 0 的實建量測（accounted 用）：HEAD worktree、base＋兩份 patch 的 worktree、patch 快照、runner 凍結副本的同形狀目錄、probe
   full 模式另建 <work>/nocf（e1cbbbd ＋ tooling；在任何 baseline 之前，所以⛔ 不計入 P_path）
1. preflight（只量記憶體）：--check-failed-record（預期 0：還沒有 failed record）
2. success：begin_phase → 凍結兩份 patch 到 runs/success/patches/ → replay＝runner（stub、success，預期 0）→ --finalize（預期 0）→ end_phase（archive＝evidence/）
3. promote_success：begin（L6 ＋ L0 取樣）→ promote-measure（evidence，預期 0）→ end
4. failure：記 failed/ 的集合 → begin_phase → 凍結 → replay＝runner（stub、failure，預期 6）→ --publish-failed-record（預期 1）→ record＝集合差（恰好一筆且 ＝ CLI 的 published）→ end_phase
5. 工作複本的 evidence/ 暫時搬到 <work>/aside/（同一個檔案系統的 rename；晉升的第 3 步要求終態恰好一組）
6. promote_failure：begin → promote-measure（failed record，預期 6）→ end → evidence/ 搬回
7. memory_only：--check-failed-record（預期 2：命中本次 record）→ --recover-durability（0）→ --recover-failed-record <record>（1）→ --recover-envcheck（0）
   → full 模式另加：replay＝runner（full、success、輸出到 runs/full/，預期 0）
8. metadata twin（所有量測窗口結束之後）
9. 報告：acceptance-report → 本次 run id 的容器歸零、Stage 2 identity 未變 → 報告與原始量測複製到 <work>，清 S
```

| 項目 | 規則 |
|---|---|
| 檔案的來源（第二輪 review） | **harness 自己的檔案**取自啟動 harness 的 repo、**經下一列的快照**（validation 凍結工作樹的版本、報告記 SHA-256；`--formal` 驗 ＝ HEAD）；**⑩ 的正式程式**一律取自工作複本（HEAD），helper 以明確的 `--clone <工作複本>` 載入；逐檔的歸屬見本節最後的「快照與來源的清單」（第四輪 review） |
| harness 的快照（第三輪 review） | 主腳本的開頭只做最少的事（⚠️ bash 是邊讀邊執行，所以要盡早換掉）：解析參數 → 建 S（`/dev/shm/i074-accept-<run id>/`，`0700`、必須是 tmpfs）→ 把本 profile 清單上的檔案依相對路徑複製到 `<S>/harness/`、寫 `<S>/harness/MANIFEST`（相對路徑與 SHA-256）；`--formal` 在複製**之前**驗原檔的內容 ＝ HEAD 的 blob、複製**之後**再驗快照 ＝ HEAD 的 blob（兩者之間的修改也抓得到）→ `exec /bin/bash <S>/harness/scripts/i074-stage2-acceptance.sh <原參數>`，以 `SIZING_SNAPSHOT=<S>`、`SIZING_ORIGIN_REPO=<啟動 harness 的 repo>` 交棒。快照裡的主腳本先驗自己的 realpath ＝ `<S>/harness/…`、S 是 tmpfs 且 owner ＝ 自己、mode `0700`，之後共用原語、helper、shim 一律從快照載入，launcher 以快照的路徑掛載；報告記錄快照的 SHA-256。⚠️ sizing 的主腳本一併沿用（「三」#22） |
| replay 那一步 | `<工作複本>/scripts/run-replay-offline.sh <replay argv>`，環境 ＝「環境」列 ＋ `I074_STAGE=2`、`COUNTERFACTUAL_PATCH`／`TOOLING_PATCH`（本路徑的凍結副本）、`SIZING_ROLE=replay`、`SIZING_REPLAY_MODE`、`SIZING_REPLAY_COMPUTE`；⛔ 不設 `MEM`（與 ⑩ 一樣用 runner 的預設 700m 經 mem-guard 下修，報告記錄實際上限） |
| 晉升那一步（`promote-measure`） | host 的 python3 以 `Promoter(<work>, <work>/real, load_states=<harness 組的 state>)` 執行晉升的第 2～7 步（git 與驗證模式都是真的；驗證模式的容器經 shim、role `promotion`）；state 由本次的實際值組出：`bundle_id`、語意 SHA、`identity_sha256`（以 `i074_stage2_preflight` 寫 `preflight.json` 時的同一個算法）、`repo_head`（工作複本的 HEAD）、`replay_done.rc`（0／6）——⛔ 不寫任何 state 檔。記憶體與其他步驟一樣由 `host-run` 量（下一列）。⚠️ 晉升的第 1 步（`/proc/locks`、preflight 0、state 檔的驗證）屬 orchestrator，⛔ 不在 acceptance 內 |
| host 端的程序樹（第二輪 review） | **每一步**都以 `helper host-run --state <S> --step <名稱> -- <指令>` 執行（它是該步驟 process group 的 leader）：等指令結束後記 `RUSAGE_CHILDREN` 的 `ru_maxrss`（所有已 wait 的子孫裡最大的**單一**程序，晉升那一步含 promoter 自己——每個程序各自的高水位，**精確**，是門檻）；另每 0.1 秒把同一個 process group 的 `/proc/<pid>/status` 的 `VmRSS` 加總、取最大值（取樣，是**下界**，只作單向警報，第三輪 review）；結束碼原樣傳回；容器裡的程序屬於 docker daemon，⛔ 不在這棵樹裡（由 cgroup 峰值另量） |
| 環境 | 每一步都以 `env -i` 執行：`clean-env` 子指令載入 supervisor 的 `clean_env()`（**唯一定義**，⛔ 不另抄清單）處理 harness 收到的環境，再加 `PATH=<S>/bin:/usr/bin:/bin`（`<S>/bin/docker` 是 shim；⑩ 是 `<work>/bin:/usr/bin:/bin`）、`PYTHONDONTWRITEBYTECODE=1`、`TMPDIR=<L3>`、`REPLAY_IMAGE_ID` 與 shim 需要的 `SIZING_*`。⚠️ 兩份 patch 的變數**只**給 replay 那一步（總綱「四」：finalizer 會對 HEAD worktree 套 `TOOLING_PATCH`） |
| bootstrap 的失敗與中斷（第五輪 review；第六輪改寫） | **兩個入口各自**的 minimal bootstrap（⛔ 不 source 任何活路徑的檔案）：以 exclusive 的 `mkdir -m 700` 建 S（已存在 → 中止、⛔ 不碰）→ **立刻**裝 bootstrap trap（EXIT／INT／TERM）→ 驗 tmpfs、owner、realpath → 複製、寫 `MANIFEST`、`--formal` 的前後驗證 → `exec`。re-exec pass 的**第一個動作**是裝自己的 bootstrap trap，再做「快照與來源的清單」④ 的完整驗證。⚠️ **bootstrap 階段（snapshot pass，以及 re-exec pass 完整驗證通過之前）⛔ 不刪任何東西**：任何失敗或訊號只把 S 的位置印到 stderr（S 在 tmpfs、只有快照的幾個檔案）、結束碼 1（訊號是 130／143）。完整驗證通過之後才 source 快照裡的共用原語、把 trap 換成下一列的六步清理——S 的刪除只發生在這之後（六步清理與正常結束，沿用 sizing 既有的做法）。⚠️ 照實的界線：手動設定 `SIZING_SNAPSHOT`、而且內容**通過完整驗證**的目錄（就是一份合法的快照），與正常交棒⛔ 無法分辨，之後的正式流程結束時會照常清掉它 |
| 失敗與中斷（re-exec 之後） | 沿用 sizing 的六步清理（共用原語）：停取樣 → 以 CID 移除容器 → 收掉步驟的 process group → 再清一次 → 窗口標 aborted → S 複製到 `<work>/raw-failed/`、`failure_summary.json`（⛔ 不宣稱任何峰值）→ 全部成功才清 S |
| 演練用的故障注入 | 沿用 `I074_SIZING_FAULT`（共用原語讀它；`--formal` 一律拒絕） |

**快照與來源的清單**（第四輪 review；profile 各自、寫死在各自的主腳本，測試釘住）：

| 類別 | sizing | acceptance |
|---|---|---|
| ① 從快照（`<S>/harness/`）執行；`--formal` 在快照前後都驗 ＝ HEAD | `scripts/i074-stage2-sizing.sh`、`scripts/lib/i074-stage2-measure.sh`、`scripts/lib/i074-sizing-docker-shim.sh`、`scripts/lib/mem-guard.sh`（fixture 的記憶體上限）、`python/scripts/i074_stage2_sizing.py`（host 與 fixture 容器的掛載） | `scripts/i074-stage2-acceptance.sh`、`scripts/lib/i074-stage2-measure.sh`、`scripts/lib/i074-sizing-docker-shim.sh`、`python/scripts/i074_stage2_sizing.py`、`python/scripts/i074_stage2_replay_stub.py`（launcher 的掛載）|
| ② 從工作複本（`<work>/repo`，HEAD）執行的正式程式 | `finalize-stage2-evidence.sh` 與它載入的一切、`scripts/lib/replay-args.sh`、`anchor-base` 讀的 `python/`、fixture 容器的 `/app`、**`i074_stage2_freeze_record.py`**（`build`／`check-pair`，`--repo <工作複本>`；它載入的 `i074_stage2_preflight.py` 也是工作複本的） | `run-replay-offline.sh`、`finalize-stage2-evidence.sh` 與它們載入的一切（含 runner 自己 source 的 `mem-guard.sh`）、`i074_stage2_preflight.py`（`replay_argv()`、`P_B_BUDGET`、identity 的算法）、`i074_stage2_promote.py`、`scripts/lib/i074-stage2-supervisor.py`（`clean_env()`） |
| ③ 原始 repo（`SIZING_ORIGIN_REPO`）只用於 | `git clone` 的來源；`--formal` 的 clean 檢查（`scripts/`、`python/`、`.gitattributes`）；快照前後與 HEAD 的比對；`repo_head`／`repo_dirty` 的紀錄；inventory 自我檢查（⛔ 不得改變）——⛔ 快照之後⛔ 不執行、⛔ 不 source 它的任何檔案 | 同左（`git clone` 是 原始 repo → `<work>/real` → `<work>/repo`）；另斷言工作複本的 HEAD ＝ 記下的 `repo_head` |
| ④ 兩個 pass 的判別 | **snapshot pass**：`SIZING_SNAPSHOT` 未設定——只做「harness 的快照」列的事，然後 `exec`；**re-exec pass**：`SIZING_SNAPSHOT` 已設定——自己的 realpath 必須 ＝ `$SIZING_SNAPSHOT/harness/<自己的相對路徑>`、S 是 tmpfs 且 owner ＝ 自己、mode `0700`、`MANIFEST` 的檔案集合**恰好**是本 profile 的 ① 且每個 SHA 相符（`--formal` 再驗一次 ＝ HEAD），任一不符就中止（⛔ 不刪任何東西，見「bootstrap 的失敗與中斷」列）——⛔ 不再建快照、⛔ 不遞迴；`SIZING_SNAPSHOT` 已設定但 realpath ⛔ 不在它底下 → 中止（⛔ 不能以環境變數跳過快照） | 同左 |

###### 二之四、shim 與量測原語的 profile（含與 label shim 的互斥）

| 項目 | 規則 |
|---|---|
| profile 的來源 | harness 以 `SIZING_PROFILE` 交給 shim 與 helper：`acceptance`，或未設定（＝ sizing）；其他值 → shim 125、helper 拒絕。索引記錄 profile，報告驗它與本次 harness 相同 |
| sizing（未設定） | 既有的 CLI、改寫後的 argv 與判定不變：`--read-only`、phase ∈ {witness, success, failure, memory_only}、role ∈ {fixture, finalizer, recovery, check}、`SizeRw` 必須是 0；⚠️ accounted 依「三」#15 多 `runner_frozen_patches`；來源檔的 SHA 隨本次異動正常更新 |
| acceptance 的封閉列舉 | phase（依序）：`preflight`、`success`、`promote_success`、`failure`、`promote_failure`、`memory_only`（磁碟路徑只有 success、failure；晉升兩個是資訊值的窗口）；role：`check`、`replay`、`finalizer`、`promotion`、`recovery`；預期順序寫死：preflight ＝ (check)、success ＝ (replay, finalizer)、promote_success ＝ (promotion)、failure ＝ (replay, finalizer)、promote_failure ＝ (promotion)、memory_only ＝ (check, recovery, recovery, recovery)，full 模式再加 (replay) |
| acceptance 的改寫 | 通用改寫照舊（`--cidfile`、`--name`、`/peak` 的 mount、cgroup 峰值 wrapper、拿掉 `--rm`），但 **⛔ 不加 `--read-only`**（v29「六、1」差異列：⑩ 不加）；role `replay` 另外：容器指令開頭必須恰好是 `python -m backtest.modular.sr_scoring.evaluation`（否則 125、⛔ 不執行）→ 換成 `python /acceptance/replay_stub.py`，其後的參數逐 token 不變，加 `-v <S>/harness/python/scripts/i074_stage2_replay_stub.py:/acceptance/replay_stub.py:ro`（快照的路徑，由 harness 以 `SIZING_REPLAY_STUB` 交給 shim）與兩個模式變數；`full` 另換 `/app` 的來源（「二之二」）。twin 用同一份改寫後的 spec。⚠️ **⑦d 增補（✅ 2026-10-05 確認）**：每一個 role 的容器指令改成 `python -I /acceptance/rss_wrapper.py <原指令>`（掛載快照裡的 wrapper），⛔ 不再是 `sh -c` 的 cgroup 峰值 wrapper |
| `SizeRw` | acceptance：**照實計入**容器足跡（終止值；⛔ 不要求 0），報告另列各容器的 `SizeRw` 總和為 `read_only_gap`，與 sizing 的 0 對照（④「五」的 `--read-only` 落差） |
| 互斥（沿用 ⑦b「二之五」，⛔ 不改 label shim） | acceptance 的所有環境變數都在 `SIZING_*` 命名空間（容器內的 `I074_ACCEPTANCE_*` 只出現在 `docker run` 的 `-e`），所以 label shim 既有的「環境帶 `SIZING_*` → 125」已涵蓋；shim 既有的「環境帶 `I074_STAGE2_*` 或 `SIZING_REAL_DOCKER` 是 label shim → 125」與 harness 的啟動守門（共用原語）同樣適用於 acceptance |

###### 二之五、報告、門檻與結束碼

| 項目 | 規則 |
|---|---|
| 磁碟（門檻） | success、failure 各自 `P_path = max(dirs_peak, fs_peak, accounted)`——沿用 `phase_peaks()`、`combine_footprints()`、twin 的 `adopted_metadata()`、log 上界（⛔ 不另寫一份）。accounted 的組成同 sizing 的 success／failure：replay worktree（base＋兩份 patch）、合成守門的 worktree、HEAD worktree、patch 快照、**`runner_frozen_patches`**、run 目錄、archive／record、probe、容器足跡（含 `SizeRw`）。⚠️ `P_B` 的起訖⛔ 不變（v29「八之一」） |
| 磁碟（晉升，資訊值） | 兩次晉升各自的窗口：L6 的目錄取樣 ＋ L0 的取樣 ＋ 會計（staging ＝ 終態的 allocated bytes ＋ 每個目錄 1 個 block；rename ⛔ 不複製）→ `P_promotion`；報告另列 `P_path ＋ P_promotion`（⚠️ 偏保守：晉升開始時 L3 的 worktree 仍在）——⛔ 不與 `P_B_BUDGET` 比較；晉升的空間另由它自己的預檢（2 倍）把關 |
| 記憶體 | 容器程序：每一個 invocation 的 cgroup 峰值（wrapper 在容器內、退出前讀 v1 `memory.max_usage_in_bytes` → v2 `memory.peak`；含 page cache，與 ④⑤ 相同）；**host 端**（第二輪 review）：每一步的程序樹由 `host-run` 記**最大單一程序 RSS**（精確）與**取樣的程序群組 RSS 總和**（下界）——⚠️ RSS ⛔ 不含 page cache，與 cgroup 峰值是不同的量法；host 端⛔ 沒有可寫的 cgroup（⑦b 實查），**整個程序群組的實際總峰值量不到、也⛔ 不在契約內**（第三輪 review），照實標示；任一數字讀不到或為 0 → fail-closed。⚠️ **⑦d 增補（✅ 2026-10-05 確認）**：容器改以 wrapper 的精確閘（`max_single_rss_bytes`）＋ 取樣的 `total_rss` 判定，cgroup 峰值只列資訊值；host 端的 `host-run` 是 subreaper、收到 `ECHILD`、計入 `RUSAGE_SELF`，群組取樣改成 ppid 鏈 |
| 門檻 | **每一個容器程序的 cgroup 峰值、每一步 host 端的最大單一程序 RSS，都 < 450 MiB**（v26：「每一個程序各自」；471,859,200 bytes，嚴格小於；常數 `ACCEPTANCE_MEMORY_LIMIT` 唯一定義在 helper，測試釘死）；**success、failure 的 `P_path` ≤ `P_B_BUDGET`**（`import` 自 `i074_stage2_preflight.py`，⛔ 不抄數字）。⚠️ **單向警報**（第三輪 review）：取樣的程序群組 RSS 總和觀察到 ≥ 450 MiB → `threshold_exceeded`；< 450 MiB 只記成「未觀察到超標」，⛔ 不當成通過的證據。⚠️ **⑦d 增補（✅ 2026-10-05 確認）**：容器那一半改成「`max_single_rss_bytes` ≥ 450 MiB 或 `rss_peak_sampled_bytes` ≥ 450 MiB → 超標」 |
| 對照表（v29「六、1」） | a～c ＝ replay（stub 兩次；⚠️ 計算工作集 ＝ full 的那一次）；d、e、f ＝ `--finalize`；failure 的發布 ＝ `--publish-failed-record`；g ＝ `--recover-durability`、`--recover-failed-record`；h ＝ preflight 的 `--check-failed-record`（⑩ 的真實程式；`anchors` 以它代量，「三」#6）；i ＝ `--recover-envcheck`；晉升 ＝ promoter 的程序樹（兩次，`host-run`）＋ 驗證模式的容器（兩次） |
| status | `ok`（全部門檻成立、且單向警報沒有觸發）／`threshold_exceeded`（報告**照寫**，逐項列出違反與警報）；任何量測缺失、自我檢查不過、結束碼不符、索引不完整 → ⛔ 不產報告（fail-closed）；⚠️ 報告的 `replay_compute`（`stub`／`full`）照實記錄——stub 的報告明示「計算工作集未涵蓋」，⛔ 不得當成 ⑨-1 的正式驗收。⚠️ **⑦d 增補（✅ 2026-10-05 確認）**：另要求能力檢查與每一步的逐步檢查都通過；契約與範圍見增補計畫的「驗收語意」 |
| 結束碼 | **0** ＝ 報告已寫且 `ok`；**2** ＝ 報告已寫但 `threshold_exceeded`；**1** ＝ harness 失敗（⛔ 不產報告）（「三」#8） |
| 報告內容 | schema `i074_stage2_acceptance_report_v1`（canonical）：模式、`replay_compute`、來源（HEAD、dirty、**封閉的 `harness_manifest`**——快照清單 ① 的相對路徑 → SHA-256，與它的 canonical SHA `harness_manifest_sha256`（第五輪 review；原始 `MANIFEST` 隨原始量測保存；sizing 的報告同樣加這兩欄）；`--formal` 另有 image、identity）、兩份 patch 的 raw／canonical SHA 與合成 SHA、語意 SHA（⑨-1 拿來與 ⑨ 的封存值比對）、mem-guard 實際上限、兩條路徑的量法與組成、晉升的資訊值、每個程序的峰值與門檻、`read_only_gap`、host `MemAvailable` 低點、notes（殘餘、`anchors` 的代量）；另有文字版。⚠️ **⑦d 增補（✅ 2026-10-05 確認）**：schema 升成 `i074_stage2_acceptance_report_v2`（`contract`、`out_of_contract`、`auto_reap_detection`、`memory_measures` 四個封閉欄位、結構化的 `violations`，寫出前以 `validate_acceptance_report_v2()` 驗證） |

###### 二之六、⑩ 的實際峰值紀錄：observer（「三」#2）

⚠️ ⛔ 不改 ⑩ 的 docker argv，就**拿不到精確的容器峰值**：精確值只能在容器內、退出前讀（sizing 與 acceptance 的 wrapper 就是改了指令），
容器一結束 cgroup 就被移除。v29 只要求 ⑩ **額外記錄**（⛔ 不作為前置、⛔ 不為量測而重跑、⛔ 不反向決定是否執行 ⑩），所以用一個
**外部、唯讀**的 observer 記下界與取樣值，並照實標示：

| 項目 | 規則 |
|---|---|
| 入口 | `python3 python/scripts/i074_stage2_sizing.py observe --work-dir <⑩ 的 --work-dir（可尚未存在）> --out <repo 外、尚不存在、⛔ 不在 --work-dir 內的目錄>`。操作者在 ⑩ 開始**之前**於另一個終端機啟動，⑩ 結束之後以 Ctrl-C（INT／TERM）停止；⛔ 不是 ⑩ 的一部分（supervisor 外部的程序：⛔ 不延長鎖、⛔ 不在 ⑩ 的呼叫圖內） |
| 唯讀 | ⛔ 不寫 `--work-dir`、⛔ 不寫任何 repo；docker 只用 `ps` 與 `inspect`；執行中的狀態在 `/dev/shm/i074-observe-<run id>/`（tmpfs），停止時才寫 `--out` |
| 守門 | 環境帶 `I074_STAGE2_*` 或 `SIZING_*`、PATH 上的 docker 是 label shim 或 sizing shim → 拒絕（經 shim 看到的⛔ 不是 ⑩ 的真實狀態） |
| 記憶體 | 每 2 秒 `docker ps -a -q --no-trunc --filter label=<鍵>`（鍵 ＝ `i074.stage2.run`；只用鍵：同一時間只會有一趟 ⑩）；新出現的容器 `docker inspect` 一次，依指令的前幾個 token 辨識角色（`anchors`、`lookup`、`replay`、`finalize`、`publish`、`promotion_verify`）；每 0.5 秒讀每個已知容器的 cgroup high-water mark（v1 `memory/docker/<id>/memory.max_usage_in_bytes`，v2 的 `memory.peak` 候選路徑）→ 每個容器保留讀到的最大值、讀取次數、最後一次讀到與容器消失的時間。⚠️ **⑦d 增補（✅ 2026-10-05 確認）**：另讀 v1 `memory.stat` 的 `total_rss`（取樣的最大值），讀不到 → `rss_unavailable_containers`、⛔ 不完整 |
| ⚠️ 照實的界線 | 讀到的是**最後一次讀取之前**的 high-water mark——最後一次讀取之後、容器結束之前的尾段峰值**可能漏記**（`run-evaluation.sh` 的教訓），所以一律標成**下界**；cgroup 讀不到 → `unavailable`、⛔ 不寫 0 |
| 完整度（第一輪 review） | `replay_seen`（看過 replay 容器且至少讀到一次 cgroup）；`missing_expected_containers`（預期角色：`anchors`、`lookup`、`replay`、`finalize` 或 `publish`、`promotion_verify`——⛔ 沒有看到的逐一列出；⑩ 中途結束時照實列出）；`observation_complete` ＝ 上面兩者都成立、⛔ 沒有任何 `unavailable`、observer 在第一個容器出現之前已開始且在最後一個容器消失之後才停止。⛔ 不完整的報告照樣寫出、但頂層標明 `observation_complete=false` |
| 磁碟 | 啟動時記 L0（`statvfs` 已用量）的 baseline；每 0.5 秒取 L0；`--work-dir` 出現之後每 10 秒以 `allocated_tree()` 量整棵 tree——兩者都是**取樣值**（live 服務的寫入會墊高或壓低 L0），⛔ 不是 `P_B` 的量法、⛔ 不與 `P_B_BUDGET` 比較 |
| 報告 | `observation.json`（canonical，schema `i074_stage2_observation_v1`；每個數字都帶 `kind`：`observed_lower_bound`／`sampled`）＋ `observation.txt`；另記 host `MemAvailable` 低點；結果只**額外記錄**進本筆 |
| 鍵的漂移（第一輪 review） | 靜態測試：observer 的鍵 ＝ label shim 的常數 ＝ `run-i074-stage2.sh` 的 `I074_LABEL_KEY` |

###### 二之七、對已確認章節的修改（⚠️ 實作時都加註「⑦d 細部計畫 v1」）

| 位置 | 修改 |
|---|---|
| 「正式 scan 的計次裁決」v26／v27 的表 | ✅ **使用者裁決（2026-10-02）**：新增一格「**⑨-1 量測趟**」——性質：容量量測，`e1cbbbd` ＋ tooling、⛔ 不套 counterfactual（算出的列與錨定的 D+1 相同，⛔ 不產生任何新的判讀資訊；輸出只留在 acceptance 的複本，⛔ 不得當成證據）；計次：⛔ 不計入正式 scan；上限：**1 趟、崩潰重試 1 次**，另計、⛔ 不佔上限 ①②；⚠️ 第二輪 review：兩份 patch 任一份的 bytes 或 SHA 再變，⑨-1 一律整個重跑（含量測趟，v29「八」），量測趟的額度要重跑時另行裁決（⛔ 不自動取得）——所以 ⑨-1 只在 ⑨ 封存之後執行 |
| v29「六、1」 | 覆蓋表下方：計算工作集由 ⑨-1 量測趟涵蓋（「二之二」）；h 列：⑩ 的 preflight 已實作，h 量真實的 `--check-failed-record`、`anchors` 以它代量；晉升（promoter、驗證模式）的記憶體也納入 < 450 MiB，晉升的磁碟另列資訊值、`P_B` 的起訖⛔ 不變；「實作」列：沿用 sizing 的量測原語（共用 shell 與 profile）；「差異」列：acceptance ⛔ 不加 `--read-only`、`SizeRw` 照實計入並列 `read_only_gap` |
| v29「八」 | ⑨-1：入口與結束碼（`scripts/i074-stage2-acceptance.sh --formal --replay-compute full`：0／2／1）、量測趟（上一列）、報告的 patch SHA 由 ⑨-1 的程序與 ⑨ 的封存值比對；⑩：observer（「二之六」，下界、⛔ 不作前置） |
| v29「八之一」容量列 | 補註：⑦d 的 acceptance 實跑晉升，記憶體納入門檻、磁碟只列資訊值（使用者裁決 2026-10-02，本列的規則⛔ 不變） |
| 總綱「三」⑦d 列 | 本計畫的決定（「二之二」「二之六」與計次的新格） |
| ④ 計畫書「二」的已知寫入清單與「三」的 docker shim 表 | `runner_frozen_patches`；shim 的 profile |
| ⑤ 的結果 | 註明當時的 accounted ⛔ 沒有 `runner_frozen_patches`（數字保留為歷史紀錄） |

##### 三、總綱沒寫到、本計畫補上的決定（⚠️ 待確認）

| # | 決定 | 理由 |
|---|---|---|
| 1 | replay 有兩種計算模式：stub（只替換 `_decision_replay_rows()`，快速）與 full（真實計算、⛔ 不套 counterfactual、之後才套 success 的合成）（「二之二」） | ✅ **使用者裁決（2026-10-02）**。stub 讓開發驗證與兩條磁碟路徑在數分鐘內完成；full 讓 ⑨-1 在同一個程序量到計算工作集 ＋ 尾段的精確峰值，而且⛔ 不會在 ⑩ 之前產生任何 B／C 資訊。⛔ 不採「套 counterfactual 的完整 replay」（會提前得到 before 的結果） |
| 2 | ⑩ 的實際峰值由**外部唯讀 observer** 記錄：記憶體是 cgroup high-water mark 的**下界**、磁碟是取樣值，報告標明完整度（「二之六」） | ⛔ 不改 argv 就拿不到精確值；v29 只要求額外記錄。⛔ 不採「讓 `evaluation.py` 自己印 `ru_maxrss`」——要動 tooling 路徑、重產 tooling patch，指標也與 cgroup 峰值不同 |
| 3 | sizing 的共用 shell 原語抽成 `scripts/lib/i074-stage2-measure.sh`，sizing 改 source 它 | 總綱：「⛔ 不另寫一份」。sizing 的既有測試全部照跑（只有 #15 帶來的 accounted 變化要更新期望值），另實跑一次 sizing validation |
| 4 | 量測原語留在 `i074_stage2_sizing.py`，以 profile 參數化封閉列舉；acceptance 的報告是新函式 | 同上 |
| 5 | acceptance ⛔ 不加 `--read-only`，`SizeRw` 照實計入並列 `read_only_gap` | v29「六、1」差異列 |
| 6 | h 量 ⑩ preflight 的真實程式 `--check-failed-record`；`anchors` 以它**代量** | `anchors` 只做 `_load_trust_anchors()`（另傳入目前的 identity），與 `check_failed_records()` 的第一步相同、之後⛔ 沒有任何讀取；直接量 `anchors` 要在 harness 複製 orchestrator 的 docker argv（會漂移） |
| 7 | 晉升**實跑**（success、failure 各一）：promoter 與驗證模式的記憶體納入門檻；晉升的磁碟只列資訊值，`P_B` 的定義⛔ 不變 | ✅ **使用者裁決（2026-10-02）**。取代 v1 的「以 recovery 代量驗證模式」；v29「八之一」容量列的理由（自己的空間預檢、失敗可重試）照舊成立 |
| 8 | 結束碼 0（ok）／2（`threshold_exceeded`，報告照寫）／1（harness 失敗，⛔ 不產報告） | ⑨-1 要能機械判定；超過門檻仍要保留完整數字給回退決策 |
| 9 | 順序 preflight → success → 晉升 success → failure →（evidence 暫時搬開）晉升 failure →（搬回）→ memory_only →（full）量測趟 | 對齊 ⑩ 的順序；failure 排在 success 之後（v29：反過來的話，複本內的 record 會擋住 success）；晉升的第 3 步要求終態恰好一組，所以第二次晉升之前把 evidence/ 搬開；量測趟最後跑，失敗時前面的結果已經完整 |
| 10 | ⛔ 不跑 witness 路徑 | v29「六、1」：witness 已在 ③c 完成 |
| 11 | `--formal`：必須 `--replay-compute full`、clean、harness 自己的檔案（「四」的清單）＝ HEAD；報告記錄兩份 patch 的 raw／canonical SHA，**與 ⑨ 封存值的比對由 ⑨-1 的程序做** | ⑨ 的封存形式還沒定；⛔ 不在 ⑦d 預先綁定 |
| 12 | 每一步的環境 ＝ supervisor 的 `clean_env()` ＋ 固定 PATH ＋ 量測變數；兩份 patch 只給 replay 那一步 | 與 ⑩ 相同的環境；`clean_env()` 是唯一定義 |
| 13 | replay argv 以 `replay_argv()`（唯一的組裝處）產生，再對應兩個路徑前綴 | ⛔ 不各寫一份 |
| 14 | 記憶體門檻嚴格 `<` 450 MiB（471,859,200 bytes）、磁碟 `≤ P_B_BUDGET` | v29 的原文（「< 450 MiB」、「≤ `P_B_BUDGET`」） |
| 15 | `runner_frozen_patches` 在 acceptance 與 **sizing 都計入**；⑤ 的數字保留為歷史紀錄 | 第一輪 review：既然已確認是正式路徑的實際占用，⑨-2 繼續省略就是已知的低估；⛔ 不為了與 ⑤ 的可比性維持錯誤的模型 |
| 16 | 計次政策新增「⑨-1 量測趟」一格（「二之七」第一列） | ✅ **使用者裁決（2026-10-02）**；量測趟⛔ 不套 counterfactual，所以⛔ 不是 Stage 2 scan、也⛔ 不產生判讀資訊 |
| 17 | 晉升以 `Promoter` 的建構子注入 `load_states`（harness 組 state、⛔ 不寫 state 檔）；第 1 步（`/proc/locks`、preflight 0、state 檔的驗證）⛔ 不在 acceptance 內 | acceptance ⛔ 不能跑 supervisor 與 orchestrator（鎖、sentinel、label shim）；第 1 步只讀小檔，記憶體可忽略、照實寫明 |
| 18 | 兩層複本：`<work>/real`（模擬的真正 repo）→ `<work>/repo`（工作複本，origin ＝ `<work>/real`） | 驗證模式的路徑必須在 origin 內；⛔ 不碰真正 repo |
| 19 | full 只在 ⑨-1；⑦d ⛔ 不在真資料上跑 full（只以 pytest 與 fake docker 驗它的機制） | 量測趟只有 1 趟的額度 |
| 20 | harness 自己的檔案取自啟動 harness 的 repo、⑩ 的正式程式取自工作複本（「二之三」） | 第二輪 review：commit 之前工作複本沒有本包的新檔；sizing 本來就這樣做；正式程式從工作複本取，`--formal` 時它就是 HEAD |
| 21 | 每一步經 `host-run` 量 host 端程序樹：**最大單一程序 RSS 是門檻**，取樣的群組總和是**單向警報** | 第二輪 review：promoter 的子程序（git、shell、docker client）`RUSAGE_SELF` 看不到，一律套用、晉升⛔ 不特例；第三輪 review：下界⛔ 不能證明通過，而 v26 的契約本來就是每一個程序各自 |
| 22 | harness 自己的檔案在步驟 0 凍結成 S 裡的快照，主腳本 `exec` 快照版本；**sizing 一併沿用**；清單 profile 各自（「二之三」的「快照與來源的清單」），sizing 的 freeze record 改從工作複本執行 | 第三輪 review（TOCTOU）；sizing 共用同一套原語，⑨-2 的 `--formal` 有同樣的風險——⚠️ 本項的 sizing 部分是本計畫自己延伸的，請一併確認 |
| 23 | bootstrap（建 S、快照、`exec`）由兩個入口各自負責；**bootstrap 階段⛔ 不刪任何東西**（只印出 S 的位置），S 的刪除只在完整驗證通過之後的正式流程；共用原語只負責 re-exec 之後 | 第五輪 review：bootstrap 的失敗發生在共用原語與正式 trap 生效之前；第六輪 review：nonce ⛔ 不是可信的證明、check-then-delete ⛔ 不是原子的——採保守方案（推薦的 fd 方案⛔ 不採的理由見第六輪的修正表） |
| 24 | 兩種報告都加封閉的 `harness_manifest` 與 `harness_manifest_sha256`；freeze record 的 schema ⛔ 不擴充 | 第五輪 review：validation 允許 dirty，報告要能單獨說明實際執行的檔案集合；freeze record 已綁整份報告的 SHA |

##### 四、受影響檔案

| 檔案 | 改動 |
|---|---|
| `scripts/i074-stage2-acceptance.sh`（新增） | 「二之三」（含自己的 minimal bootstrap） |
| `scripts/lib/i074-stage2-measure.sh`（新增） | 自 `i074-stage2-sizing.sh` 抽出的共用原語 |
| `scripts/i074-stage2-sizing.sh` | 開頭的 minimal bootstrap 建快照並 `exec` 快照版本（「三」#22、#23）；報告加 `harness_manifest`（#24）；source 共用原語；freeze record 改從工作複本執行；步驟 0 實建 runner 凍結副本的同形狀目錄；`--formal` 的檔案清單改成「快照與來源的清單」的 ① |
| `scripts/lib/i074-sizing-docker-shim.sh` | `SIZING_PROFILE`、replay 的改寫與 full 的 `/app` 來源（「二之四」） |
| `python/scripts/i074_stage2_sizing.py` | profile、sizing 的 `runner_frozen_patches`、`acceptance-report`、`replay-argv`、`clean-env`、`promote-measure`、`host-run`、`observe`、`ACCEPTANCE_MEMORY_LIMIT` |
| `python/scripts/i074_stage2_replay_stub.py`（新增） | launcher |
| `python/backtest/modular/sr_scoring/tests/test_i074_stage2_acceptance.py`（新增） | launcher（stub、full）、profile、報告、argv 對應、晉升的 state 組裝、observer 的核心邏輯 |
| `python/backtest/modular/sr_scoring/tests/test_i074_stage2_sizing.py` | `runner_frozen_patches` 的期望值 |
| `scripts/test-replay-args.sh` | shim 的 acceptance profile、harness 的守門與環境、共用原語的回歸、sizing 的新組成 |
| `scripts/test-i074-stage2.sh`、`scripts/tests/test_i074_stage2_host.py` | label shim 拒絕 `SIZING_PROFILE`；鍵的靜態測試；`clean-env`、`promote-measure` 的 state 組裝與 observer 在 host 3.9 的 unittest |
| `docs/issue.md`、`docs/development-workflow.md`、`docs/sr-zone-scoring.md` | 計畫書、實作結果、歸檔 |
| ⛔ 不改 | `evaluation.py`、`replay_bundle/`（tooling patch 不變）、`run-replay-offline.sh`、`finalize-stage2-evidence.sh`、`run-i074-stage2.sh`、supervisor、label shim、`i074_stage2_preflight.py`、`i074_stage2_promote.py`（只讀前兩個 Python 模組的常數與函式，以及 supervisor 的 `clean_env()`） |

`--formal` 要驗 ＝ HEAD 的檔案：「二之三」的「快照與來源的清單」的 ①（profile 各自，快照的前後各一次）；② 的正式程式取自工作複本（它就是 HEAD）；「scripts/、python/、.gitattributes clean」照舊。

##### 五、測試（id → 落點）

| id | 內容 | 落點 |
|---|---|---|
| ac1 | launcher 只替換 `_decision_replay_rows`（替換前後 module `__dict__` 只差這一個名稱）；`sys.argv[1:]` ＝ 原參數 → provenance 與不經 launcher 的對照組逐欄相同（`argv` 也相同） | pytest（⑦a 的反事實 fixture） |
| ac2 | stub：success → rc=0、恰好三檔、rows 數 ＝ after 列數、只有 cohort 列被改；failure → rc=6、只有 `bounded_diagnostics.json`、`rr_not_restored` | pytest |
| ac3 | `--after-artifact`／`--cohort-manifest` 缺少或重複、`I074_ACCEPTANCE_REPLAY`／`I074_ACCEPTANCE_COMPUTE` 缺少或非法、`full` 配 `failure` → rc=1、⛔ 沒有輸出 | pytest |
| ac3b | full：真的呼叫原本的 `_decision_replay_rows()`（spy）、之後才合成；回傳的 cohort 列⛔ 不是候選（模擬已套 counterfactual）→ rc=1、⛔ 沒有輸出 | pytest |
| ac4 | shim 的 acceptance profile：⛔ 沒有 `--read-only`；replay 的指令恰好換成 launcher（加唯讀 mount 與兩個模式變數），其餘 option 與參數逐 token 不變；replay 指令開頭不符 → 125、⛔ 不執行；`full` 恰好換掉一個 `/app` mount（零個或兩個 → 125）；非 replay 的 role 只做通用改寫；未知 profile → 125；profile 未設定時與 ⑦c 版本的改寫結果相同（既有測試不改） | shell（fake docker） |
| ac5 | profile 的封閉列舉與預期順序（含 full 多出的 replay）；acceptance 的 `SizeRw` 非 0 照實計入、sizing 仍要求 0；索引的 profile 與本次不符 → fail-closed | pytest |
| ac6 | 門檻的邊界：容器的 cgroup 峰值、host 端的最大單一程序 RSS 各自 ＝ 471,859,200 → 不通過、－1 → 通過；取樣的群組總和 ≥ 471,859,200 → `threshold_exceeded`、＜ 471,859,200 → ⛔ 不影響 status、只記「未觀察到超標」；`P_path` ＝ `P_B_BUDGET` → 通過、＋1 → 不通過；`P_B_BUDGET` 與 `i074_stage2_preflight.P_B_BUDGET` 是同一個物件；晉升的資訊值⛔ 不影響 status；量測缺失或為 0 → ⛔ 不產報告；`threshold_exceeded` → 報告照寫、逐項列出、harness 結束碼 2；stub 的報告標明「計算工作集未涵蓋」 | pytest ＋ shell。⚠️ **⑦d 增補（✅ 2026-10-05 確認）**：容器那一半改成精確閘與偵測器的邊界，cgroup 峰值 ＝ 門檻 → ⛔ 不影響 status（資訊值） |
| ac7 | harness 的守門：work 目錄（repo 內、parent symlink 指回 repo、已存在）、缺 `REPLAY_IMAGE_ID`、`--formal` 未帶 `--replay-compute full`、`--formal` 時 dirty 或清單內的檔案 ≠ HEAD、fd 導到 L0 的一般檔案 → 拒絕 | shell（隔離 repo） |
| ac8 | 與 label shim 互斥：harness 在環境帶 `I074_STAGE2_*`、或 PATH 上的 docker 是 label shim 時中止；acceptance profile 的 shim 在同樣條件下 125；label shim 在環境帶 `SIZING_PROFILE=acceptance` 時 125 | shell |
| ac9 | 環境：replay 那一步 ＝ `clean_env()` 的結果 ＋ 固定 PATH ＋ 量測變數 ＋ 兩份 patch；其他步驟⛔ 不得收到兩份 patch 的變數（fake 入口錄下環境）；`clean-env` 與 supervisor 的 `clean_env()` 對同一份輸入結果相同 | shell ＋ host unittest |
| ac10 | `replay-argv`：與 `replay_argv()` 的輸出只差兩個路徑前綴，其餘逐 token 相同 | pytest |
| ac11 | 共用原語：sizing 的既有測試（④ 的 a～a13、b～b3 與中止點演練）全部照跑（只更新 #15 帶來的期望值）；acceptance 在 twin 那一步注入故障 → `raw-failed/` 與 `failure_summary.json`、⛔ 不產報告、S 已清 | shell |
| ac12 | observer：fake docker ＋ 可覆寫的 cgroup 根目錄——high-water 取最大、容器出現與消失、cgroup 讀不到 → `unavailable`（⛔ 不寫 0）、`--out` 在 repo 或 `--work-dir` 內 → 拒絕、環境帶 `I074_STAGE2_*`／`SIZING_*` 或 docker 是 shim → 拒絕、收到 INT 之後寫出報告、`--work-dir` 的 inventory ⛔ 不變 | pytest ＋ host unittest（3.9） |
| ac12b | observer 的完整度：⛔ 沒有看到 replay → `replay_seen=false`、`observation_complete=false`；少一個預期角色 → 列進 `missing_expected_containers`；有 `unavailable` → 不完整；全部看到 → 完整；鍵的靜態測試（observer ＝ label shim ＝ orchestrator） | pytest ＋ shell |
| ac13 | 晉升的量測：state 組裝的五個欄位與 orchestrator 寫 `preflight.json`／`replay_done.json`／`run.json` 時對同一組輸入算出的值相同；`promote-measure` 在合成的兩層複本上 success → 0、failure → 6，目的地落在 `<work>/real`、真正 repo ⛔ 不變；promoter 的程序樹由 `host-run` 量到（含 git 與驗證模式的子程序）；evidence/ 搬開之後才晉升 failure、之後搬回 | pytest（合成 repo）＋ host unittest |
| ac14 | sizing 的 `runner_frozen_patches`：witness（0-byte 的 tooling）與 success／failure（兩份 patch）的形狀各自實建量測、計入 accounted；`--formal` 的 freeze record 照常寫出、通過 `check-pair` | pytest ＋ shell |
| ac16 | `host-run`：子孫的最大單一 RSS 取自 `RUSAGE_CHILDREN`（以配置已知大小記憶體的子程序與孫程序驗證，孫程序較大時取到孫程序）；兩個**同時存活**、各自配置已知大小的子程序 → 群組總和的取樣 ≥ 兩者之和（扣掉取樣容差）、最大單一 ＝ 較大的那一個；取樣只算同一個 process group；結束碼原樣傳回；收到 TERM 時連同子程序結束；讀不到 → fail-closed | host unittest（3.9）。⚠️ **⑦d 增補（✅ 2026-10-05 確認）**：「取樣只算同一個 process group」改成「以 ppid 鏈追到的全部子孫（含 setsid 的）」；新增的 ac20～ac28 見增補計畫的「五」 |
| ac17 | 檔案的來源（第四輪 review 改寫）：快照的內容取自啟動 harness 的 repo；shim 實際掛載的一定是 `<S>/harness/…`；快照的 SHA ＝ 當次凍結的版本；工作複本⛔ 不是 launcher 的來源；validation 以修改過的工作樹 launcher 執行 → 快照與報告記的是它的 SHA；`--formal` 時 launcher 與 HEAD 不同 → 拒絕；helper 載入的 `replay_argv()`、`clean_env()`、`P_B_BUDGET` 來自 `--clone` | shell ＋ pytest |
| ac18 | 快照（第三輪 review）：快照建立之後（fake docker 第一次被呼叫時）修改與刪除原始的 launcher、helper、shim → 後續的步驟、launcher 的掛載來源與報告的 SHA 都是快照的；`--formal` 在複製前後任一次與 HEAD 不同 → 拒絕；快照裡的主腳本在 realpath 不符、S 不是 tmpfs 或 mode 不是 `0700` 時中止；`MANIFEST` 多一個、少一個或 SHA 不符 → 中止；手動設定 `SIZING_SNAPSHOT` 但腳本⛔ 不在它底下 → 中止、⛔ 沒有建任何快照；sizing 同樣的一支，另驗快照之後修改原始 repo 的 `mem-guard.sh`、`i074_stage2_freeze_record.py`、`i074_stage2_preflight.py` ⛔ 不影響本次（freeze record 由工作複本寫出）。⚠️ 第五輪 review（第六輪改寫）：bootstrap 的失敗——複製失敗、寫 `MANIFEST` 失敗、`--formal` 的後驗失敗（以 PATH 上包裝的 `cp` 在複製之後改掉原檔，`--formal` ⛔ 不接受故障注入）、re-exec 的 `MANIFEST` 驗證失敗、bootstrap 期間收到 INT 與 TERM（validation 的故障注入讓 bootstrap 停在建好 S 之後）——每一支都斷言結束碼（1／130／143）、S 的位置印在 stderr、**S 與裡面已寫的檔案⛔ 沒有被刪**（測試自己收拾）；手動設定 `SIZING_SNAPSHOT` 指向既有目錄（只有部分內容；另一支：**內容完整合法、之後的某一道驗證才失敗**）→ 中止、該目錄的內容與 inode ⛔ 不變 | shell |
| ac19 | 報告綁住完整的快照清單（第五輪 review；第六輪拆開）：validation（dirty）下逐一改動清單 ① 的每一個檔案（兩種 profile 各自）→ 報告的 `harness_manifest` 對應那一筆與 `harness_manifest_sha256` 跟著變、其餘不變；`harness_manifest` 的鍵集合恰好 ＝ ①；與保存的 `MANIFEST`、快照檔案的實際 SHA 一致；sizing ⛔ 沒有寫出 freeze record | shell ＋ pytest |
| ac19b | clean 的 formal fixture（sizing）：報告有 `harness_manifest` 與 `harness_manifest_sha256`、freeze record 照常寫出並通過 `check-pair`（freeze record 的 schema ⛔ 不變） | shell ＋ pytest |
| ac15 | 隔離：真正 repo 的 inventory 與 Stage 2 identity 檔不變（harness 自己斷言）；`/run/lock` ⛔ 沒有新項目；⛔ 沒有帶 `i074.stage2.run` 的容器；本包的測試⛔ 不增加真正 repo 的 worktree 登記 | shell、實跑 |
| 「六、1」a～i ＋ 晉升 | 開發驗證的實跑（「六」第 3 步，stub）：報告的對照表逐項有數字；計算工作集留待 ⑨-1 的量測趟 | 實跑 |

⚠️ 測試⛔ 絕不碰 `/run/lock`、⛔ 絕不建立帶正式鍵（`i074.stage2.run`）的真實容器：observer 的真 Docker 試跑用以 sed 換掉鍵的副本（比照 ⑦b「五」）。

##### 六、驗證

1. `python/scripts/test.sh` 完整執行（依序；含 `test-replay-args.sh`、`test-i074-stage2.sh`、host unittest、doc-refs）。
2. 反向驗證（逐項注回 → 對應測試變紅 → 逐位元還原）：launcher 多改一個屬性或改 argv；full 不呼叫原本的計算、或⛔ 不檢查 cohort 是否仍是候選；
   acceptance 加回 `--read-only`；`SizeRw` 不計入；記憶體門檻改成 `≤`；磁碟門檻改成 `<`；拿掉 `runner_frozen_patches`（acceptance 與 sizing 各一）；
   晉升的資訊值混進門檻；兩份 patch 的變數傳給 finalizer；拿掉 identity 的結束檢查；observer 讀不到時寫 0、或沒看到 replay 卻標完整；
   拿掉 acceptance 的互斥守門；`host-run` 改成只看 `RUSAGE_SELF`；shim 改從工作複本或活路徑掛載 launcher；拿掉 `exec` 快照（改回直接執行原檔）；群組總和改回當成通過門檻；bootstrap 失敗時改成 `rm -rf` S（手動指定的完整目錄被刪）；拿掉 bootstrap trap（訊號時⛔ 沒有印出位置、結束碼不符）；報告的 `harness_manifest` 漏掉共用原語。
3. acceptance **validation 模式、stub** 實跑一次（真 Stage 2 image、真 bundle 與錨定的 D+1、兩次實際晉升，預估 15～20 分鐘）：各步驟結束碼符合「二之三」、
   自我檢查通過；記下兩條路徑的 `P_path`、晉升的資訊值、各程序的峰值（含 promoter 與驗證模式）、`read_only_gap`、`runner_frozen_patches`。
   ⚠️ 若 `threshold_exceeded`：照實回報，依 v29「六、1」的回退順序由使用者決定（⛔ 不在 ⑦d 內改預算）。
4. sizing **validation 模式**實跑一次（抽出共用原語、加上 `runner_frozen_patches` 之後，約 6 分鐘）：取得 ⑦c tooling 下的 `P_B`。
5. observer 的真 Docker 試跑（以 sed 換掉鍵的副本）：短命容器配置已知大小的記憶體 → high-water 下界 ≥ 配置量、完整度欄位符合實際。
6. 隔離：`/run/lock`、真正 repo 的 inventory、worktree 登記數、`python/baselines/`。
7. `scripts/make-i074-tooling-patch.sh --verify "$(git write-tree)"`（⑦d 不動 tooling 路徑，預期不變）。
8. ⛔ 不在 ⑦d 執行：full 模式的真資料實跑（⑨-1 的量測趟）、`--formal`。

##### 七、風險與回滾

| 風險 | 對策 |
|---|---|
| stub 低估計算工作集 | 報告明示；正式驗收（⑨-1）必須帶 full；⑩ 的 observer 另記下界 |
| full 的量測趟只有 1 趟、約 3 小時 | 最後才跑（前面的結果已完整）；崩潰重試 1 次；⑦d ⛔ 不在真資料上用掉它；⑨-1 只在 ⑨ 封存之後執行——之後兩份 patch 任一份再變就整個重跑 ⑨-1，量測趟的額度另行裁決 |
| full 的程式碼少了 counterfactual 的兩個產品檔改動 | 本次封存的 patch 只改 RR 判定的分支，資料量與結構相同；照實寫明，⛔ 不推廣到未來的 patch |
| 抽出共用原語改壞 sizing（⑨-2 要用它） | sizing 的既有測試照跑 ＋ validation 實跑；回滾時一起 revert |
| 開發驗證超過 `P_B_BUDGET`（⑦c 讓 tooling 與 HEAD 變大，`runner_frozen_patches` 也是新計入） | 照實回報；v29 的回退順序（更新預算 → 重跑 ⑤ → ⑥）由使用者決定 |
| host 端程序樹的精確峰值量不到（⛔ 沒有可寫的 cgroup） | 最大單一程序 RSS（精確）是正式門檻；取樣的群組總和（下界）只提供單向的超標警報，低於 450 MiB ⛔ 不構成通過的證據；照實標示 |
| acceptance 以建構子注入 state 跑晉升，與 ⑩ 的第 1 步不同 | 第 1 步只讀小檔；照實寫明；晉升的第 2～7 步、git 與驗證模式都是真的 |
| observer 拖累 ⑩（2 GiB host） | 只做 `ps`、`inspect`、讀檔與每 10 秒一次的 tree 掃描；可以隨時停掉；⛔ 不碰 ⑩ 的任何檔案或容器 |
| acceptance 誤寫真正 repo | 兩層複本；全部在工作複本執行（路徑常數由腳本位置推導），晉升的目的地是 `<work>/real`；inventory 自我檢查含真正 repo |
| host 只有 2 GiB | 依序執行；每個容器經 mem-guard |
| 回滾 | ⑦d 只新增檔案、重構 sizing 與測試；⑦a～⑦c ⛔ 不依賴它，可以單獨 revert ⑦d（sizing 的 accounted 一起回到舊模型） |

##### 八、歸檔（實作後；review 前保留本筆的計畫內容）

- `development-workflow.md`：acceptance harness 的操作程序（模式、兩種計算、輸出、結束碼、門檻、⑨-1 怎麼用它）、⑩ 期間 observer 的操作；sizing 一節補共用原語、profile 與 `runner_frozen_patches`。
- `sr-zone-scoring.md`：容量驗收的現況規格（涵蓋項目、兩種計算模式與殘餘、晉升的量法、`anchors` 的代量、observer 是下界與完整度）。
- `issue.md`：⑦d 實作結果（開發驗證的數字、反向驗證、與計畫的差異）。

#### Stage 2 步驟 ⑦d 實作結果（2026-10-02，⚠️ **待 review**）

✅ 依「Stage 2 步驟 ⑦d 細部計畫 v1」（六輪 review 後確認並 commit `9af7942`）完成「二」的設計。⛔ **沒有在真正的 `/run/lock` 執行任何東西、
沒有跑完整計算（那是 ⑨-1 唯一的量測趟）、沒有跑 `--formal`、沒有 commit**；程式與文件已 stage，停在 review。⚠️ ⑦d ⛔ 沒有動
tooling 路徑（`evaluation.py`、`replay_bundle/`）：tooling patch 仍是 `5ff9a90f…`（stage 之後 `--verify` 通過）。

**實作途中發現、已修正**：

| # | 發現 | 處置 |
|---|---|---|
| 1 | ⚠️ **stub 的 replay 在 mem-guard 的 444m 下被 OOM**（開發驗證第二次實跑：容器結束碼 137、`anon-rss 442192kB`）：逐列 JSON 解析的列各自持有鍵與字串的物件，比真正計算產出的列大得多 | 先在 Stage 2 image 實測 13,417 列常駐的增量：逐列解析 **＋324 MiB**、以 `sys.intern` 共用鍵與字串值 **＋141 MiB**；③c 見證趟的完整計算（含整份 rows）峰值約 315 MiB、import 本身約 170 MiB，與後者相符。launcher 的 stub 改成共用（`compact()`）；計畫「二之二」的「stub 的列佔用預期 ≥ 真正的列（保守方向）」因此不成立——見「與計畫的差異」#1 |
| 2 | ⚠️ **`--recover-envcheck` 在 acceptance 裡必然失敗**（開發驗證第三次實跑）：封存的 `envcheck/` 是 ③c 當時的 HEAD 發布的，recovery 要求執行身分的 `base_commit` ＝ 封存的 `finalizer_provenance.base_commit`（sizing 沒撞到，因為它的 witness 路徑先以目前的 HEAD 重新發布了 `envcheck/`） | i 改成：把封存的 `envcheck/` 暫時搬開 → 以封存的見證輸出（`envcheck/witness/*.json.gz`，gzip 解壓即原本的 canonical bytes）跑 `--envcheck` 重新發布（E3a 全量比對 ＋ 發布——就是 ③c 執行過的程式）→ `--recover-envcheck` → 搬回原本的。見「與計畫的差異」#2 |
| 3 | `replay_argv()` 的第一個 token 就是 runner 本身，harness 又在前面加了一次 runner（開發驗證第一次實跑：`evaluation: error: unrecognized arguments …/run-replay-offline.sh`） | 照原樣執行 `replay_argv()`，並斷言第一個 token ＝ 工作複本的 runner |
| 4 | sizing 的 `anchor-base` 從 helper **自己的目錄** import `_i074_bootstrap`——快照裡沒有它，sizing 一改成從快照執行就會壞 | 改成從 `--python-root`（工作複本）載入（「快照與來源的清單」②）；`acceptance-anchors` 同一個做法 |
| 5 | sizing 既有的「`--formal` 時 stdout 導到 L0 → 拒絕」那支測試，在 bootstrap 加上之後改由「清單 ① 未進版控」先擋下——測試仍綠、但⛔ 不再測到 fd 的守門 | 那支之前先把 shim 重新納入版控，並斷言 stderr 是 fd 守門的訊息；「python/ 有未 commit 的變更」那支改斷言 bootstrap 的「與 HEAD 的內容不同」（helper 在清單 ① 內） |
| 6 | 測試自身：非互動 shell 的背景工作一開始就 ignore SIGINT（bash 也無法 trap 進場時被 ignore 的訊號），INT 那支一直送不進去；`ac_kept` 會把啟動訊息裡的 `S=…` 誤認為「保留」 | 以 python 把 SIGINT 還原成預設、`setsid` 之後再 exec harness；`ac_kept` 只認「保留」的訊息，另斷言啟動時的 S 已不存在 |

**與計畫的差異**：

| # | 差異 | 理由 |
|---|---|---|
| 1 | stub 的列以 `sys.intern` 共用鍵與字串值（計畫寫「逐列 JSON 解析，佔用預期 ≥ 真正的列，保守方向」） | 上表 #1：未共用的 stub 在 ⑩ 同樣的 mem-guard 上限下就 OOM，跑不完兩條磁碟路徑；共用之後的增量（＋141 MiB）與 ③c 的完整計算相符。stub 本來就⛔ 不是正式驗收（`--formal` 必須 full），報告照樣標明「計算工作集未涵蓋」 |
| 2 | i 量 `--envcheck`（重新發布）＋ `--recover-envcheck`，memory_only 的預期角色多一個 `finalizer`（計畫只寫 `--recover-envcheck`） | 上表 #2 |
| 3 | ac9（環境的契約）：`host-run` 記錄每一步的環境變數**名稱**（⛔ 不記值）與指令，報告以 supervisor 的 `ENV_DROP_*`（唯一定義）逐步驗——違反即 fail-closed（計畫寫「fake 入口錄下環境」） | 實際的每一步都被驗到（開發驗證的實跑本身就是證據），⛔ 不需要另建一套假的正式程式 |
| 4 | ac11 的中止點用 `prepare`（work 目錄建好之後立刻中止；`twins` 照樣可用） | 在 twin 那一步中止要先跑完整個流程（約 7 分鐘），⛔ 不適合放進每次的測試 |
| 5 | shim 的 launcher 路徑由 `SIZING_STATE` 推導（`<S>/harness/python/scripts/i074_stage2_replay_stub.py`；計畫寫「由 harness 以 `SIZING_REPLAY_STUB` 交給 shim」） | 更嚴：shim ⛔ 不接受快照以外的來源 |
| 6 | acceptance 不論模式都驗「② 的正式程式在 HEAD 裡」 | 正式程式本來就取自工作複本（HEAD），HEAD 裡沒有就跑不了，提早擋下 |

**檔案**：

| 檔案 | 內容 |
|---|---|
| `scripts/i074-stage2-acceptance.sh`（新增） | 「二之三」的流程：minimal bootstrap（快照 ＋ re-exec）、守門、兩層複本、preflight → success → 晉升 → failure → 晉升 → memory_only（含 i 的重新發布）→（full）量測趟、twin、報告；故障注入 `prepare` |
| `scripts/lib/i074-stage2-measure.sh`（新增） | 自 sizing 抽出的共用原語：六步清理、process group、CID 清理、量測窗口、啟動守門、work 目錄防護、`--formal` 的 fd 守門 |
| `scripts/i074-stage2-sizing.sh` | 同一份 bootstrap（兩個入口除了四個常數之外逐字相同）；source 共用原語；freeze record 改從工作複本執行；`runner_frozen_patches` 的實建量測 |
| `scripts/lib/i074-sizing-docker-shim.sh` | `SIZING_PROFILE`（acceptance：⛔ 不加 `--read-only`、replay 換成快照裡的 launcher、full 換 `/app` 的來源；未設定 ＝ sizing） |
| `python/scripts/i074_stage2_sizing.py` | profile 參數化的封閉列舉、`SNAPSHOT_FILES`、`ACCEPTANCE_MEMORY_LIMIT`、`load_harness_manifest()`；sizing 的 accounted 加 `runner_frozen_patches`、報告加 `harness_manifest`；新子指令 `acceptance-anchors`、`replay-argv`、`clean-env`、`promote-measure`、`host-run`、`acceptance-report`、`observe`；`anchor-base` 改從 `--python-root` 載入 `_i074_bootstrap` |
| `python/scripts/i074_stage2_replay_stub.py`（新增） | launcher（stub／full、success／failure） |
| 測試 | pytest：`test_i074_stage2_acceptance.py`（新增，47 項）、`test_i074_stage2_sizing.py`（fixture 補兩個組成與 MANIFEST）；shell：`test-replay-args.sh`（⑦d 一節 39 項、sizing 兩支的修正）、`test-i074-stage2.sh`（label shim 拒絕 `SIZING_PROFILE`）；host unittest：`AcceptanceHost`（8 項：clean-env、晉升的 state、host-run 三支、observer、鍵的單一常數、快照清單與 bootstrap 的一致） |
| 文件 | `development-workflow.md`（sizing 一節的 ⑦d 改動、新增 acceptance harness 與 observer 的操作程序）、`sr-zone-scoring.md`（新增「I-074 Stage 2 的容量驗收」）、本筆（已確認章節依「二之七」加註、計畫書標題與狀態列） |

**開發驗證的實跑**（validation、stub；第四次——前三次分別抓到上表的 #3、#1、#2）：

| 項目 | 結果 |
|---|---|
| 執行 | 2026-10-02 07:27～07:34Z（約 6.5 分鐘），`~/i074_stage2_acceptance/dev4-20261002T072722Z/`；HEAD `9af7942`（工作複本）、harness 為工作樹的快照；結束碼 0、`status: ok` |
| 磁碟（門檻 ≤ 160 MiB） | **success `P_path` 153.5 MiB**（目錄 152.9／檔案系統 153.3／會計 153.5；HEAD worktree 37.3、replay 與合成守門 worktree 各 15.2、run 目錄 78.0、`evidence/` 7.1、`runner_frozen_patches` 0.3、容器 0.3）；failure 69.0 MiB；`read_only_gap` 0 |
| 晉升（資訊值） | `P_promotion` success 59.8／failure 53.1 MiB（含驗證模式在 L3 建的 worktree）；`P_path ＋ P_promotion` 213.3／122.1 MiB |
| 記憶體：容器（門檻 < 450 MiB） | preflight lookup 318.4、replay（stub）success 413.8／failure 409.9、`--finalize` 359.9、`--publish-failed-record` 332.0、晉升的驗證模式 302.8／287.3、lookup 命中 301.5、`--recover-durability` 336.1、`--recover-failed-record` 392.5、`--envcheck` 418.6、`--recover-envcheck` 318.0 MiB（cgroup 峰值、含 page cache；replay 的上限是 mem-guard 下修後的 444m） |
| 記憶體：host 端 | 每一步的最大單一程序 RSS 47.9～48.8 MiB；程序群組的取樣總和 69.3～97.6 MiB（晉升最高）；`MemAvailable` 低點 247.3 MiB |
| 環境的契約 | 12 步全部通過（兩份 patch 的變數只出現在兩個 replay 的指令） |

**驗證**：

| 項目 | 結果 |
|---|---|
| `python/scripts/test.sh` 完整執行（依序） | ✅ 417 秒；pytest **1992 passed、1 skipped**（⑦c 為 1945；本包新增 47）；`test-replay-args.sh` **418 項**（⑦d 一節 39 項）、`test-i074-stage2.sh` **223 項**（含 host unittest 37 項，本包新增 8）全部通過；doc-refs 45／45、文件引用 0 個問題。真正 repo 的 worktree 登記數 ＋4（[I-118](#i-118replay-相關腳本會洩漏-git-worktree註冊與-tmp-目錄都會累積) 既有的增量，與 ⑦c 相同；本包的測試 0 增加） |
| sizing 的 validation 實跑（「六」第 4 步：抽出共用原語、快照、`runner_frozen_patches` 之後） | ✅ 2026-10-02 07:48～07:53Z（約 5 分鐘），`~/i074_stage2_sizing/validation-7d-20261002T074815Z/`；`status: ok`、**`P_B` ＝ 160,956,416 bytes（153.5 MiB）**——⑦c 的 tooling（`5ff9a90f…`）與 `runner_frozen_patches`（witness 4,096、success／failure 270,336 bytes）都已計入，預算 160 MiB 還剩 6.8 MiB；三條路徑 133.4／153.5／68.9 MiB；十個程序的 cgroup 峰值 297.9～409.1 MiB；報告的 `harness_manifest` 恰好是清單 ①、`harness_sha256` ＝ 其中主腳本那一筆 |
| observer 的真 Docker 試跑（「六」第 5 步；以 sed 換掉鍵的副本、⛔ 不建立帶正式鍵的容器） | ✅ 一個配置 150 MiB、存活 6 秒、指令含 `backtest.modular.sr_scoring.evaluation` 的容器：high-water 下界 164,278,272 bytes（≥ 配置量）、讀 11 次、角色 `replay`、消失時間有記；`replay_seen = true`、`observation_complete = false`、`missing_expected_containers` ＝ 其他四個角色（符合實際）；狀態目錄已清 |
| tooling patch | ✅ ⑦d ⛔ 沒有動 tooling 路徑：stage 之後 `--verify "$(git write-tree)"` 通過（仍是 `5ff9a90f…`） |
| 隔離 | ✅ `/run/lock` ⛔ 沒有任何 `i074-stage2.*`；⛔ 沒有帶 `i074.stage2.run` 的容器；真正 repo 的 inventory 由每一次實跑的自我檢查驗過；測試與實跑留下的 `/dev/shm/i074-*` 全部已清；`<work>` 都在 repo 外（`~/i074_stage2_acceptance/dev4-…`、`~/i074_stage2_sizing/validation-7d-…` 保留作紀錄，dev1～dev3 已刪） |

**反向驗證**（逐項注回 → 對應測試變紅 → 逐位元還原、比 SHA；pytest 那一層用既有的 harness，shell 那一層把 `test-replay-args.sh` 的 ⑦d 一節抽成獨立的腳本跑）：

| # | 注回的缺陷 | 結果 |
|---|---|---|
| D1 | launcher 多改一個屬性 | ✅ 紅：ac1（module 的屬性只差 `_decision_replay_rows`） |
| D2 | launcher 改了 argv | ✅ 紅：ac1（provenance 與對照組不同） |
| D3 | full ⛔ 不呼叫原本的計算 | ✅ 紅：ac3b 兩支 |
| D4 | full ⛔ 不檢查 cohort 是否仍是候選 | ✅ 紅：ac3b（套了 counterfactual 的列） |
| D5 | acceptance 加回 `--read-only` | ✅ 紅：ac4 的 replay 與其他 role 兩支 |
| D6 | `SizeRw` 不計入容器足跡 | ✅ 紅：ac6（容器足跡） |
| D7 | 記憶體門檻改成 `≤` | ✅ 紅：ac6 的邊界（峰值 ＝ 471,859,200） |
| D8 | 磁碟門檻改成 `<` | ✅ 紅：ac6 的磁碟邊界 |
| D9 | 拿掉 `runner_frozen_patches`（acceptance 與 sizing） | ✅ 紅：ac6、ac14 |
| D10 | 兩份 patch 的變數可以給 replay 以外的步驟 | ✅ 紅：ac9 的 finalize |
| D12 | observer 讀不到 cgroup 時寫 0 | ✅ 紅：ac12 的 `unavailable` |
| D13 | observer 沒看到 replay 卻標完整（預期角色與完整度都拿掉 replay） | ✅ 紅：ac12b。⚠️ 第一版只拿掉 `replay_seen` 那一個條件，沒有紅——`replay` 同時在預期角色裡，兩道互相涵蓋，那樣的注入⛔ 不會改變行為 |
| D14 | 拿掉 acceptance 的互斥守門 | ✅ 紅：ac8 兩支 |
| D15 | `host-run` 只看 `RUSAGE_SELF` | ✅ 紅：host unittest 兩支 |
| D16 | shim 改從工作複本掛載 launcher | ✅ 紅：ac4 的 replay |
| D17 | acceptance 的 helper 改從活路徑載入（⛔ 不用快照） | ✅ 紅：ac11／ac18。⚠️ 第一次沒紅：測試把 `raise SystemExit(99)` 加在 helper 的**結尾**，在 `raise SystemExit(main())` 之後永遠執行不到——改成插在開頭之後才紅（測試已修正） |
| D17b | sizing 的 `mem-guard.sh` 改從活路徑 source | ✅ 紅：ac18 的 sizing（結束碼 99） |
| D19 | bootstrap 失敗時刪掉 S | ✅ 紅：`--formal` 的後驗、兩種 bootstrap 失敗、INT、TERM 五支 |
| D19b | re-exec 的驗證失敗時刪掉 S | ✅ 紅：手動設定的三支 |
| D21 | 快照清單漏掉共用原語 | ✅ 紅：host unittest 的清單一致性 |

⚠️ 計畫「六」第 2 項的「拿掉 identity 的結束檢查」⛔ 沒有決定性的測試（要在實跑途中改 identity 檔），只以程式碼與實跑的結束檢查（未改動 → 通過）為證；照實列出。

**實作第一輪 review 的修正**（2026-10-02；三項都先寫測試、確認紅之後才修）：

| # | review 的發現 | 查證 | 修正 |
|---|---|---|---|
| 1 | 中：`--formal` 用的來源沒有綁到同一個 immutable commit——快照先對當時的 HEAD 驗、之後才重新讀 `repo_head`，sizing 更晚才 clone，報告也沒驗 `clone_head ＝ repo_head`；HEAD 在期間移動時，可能拿舊快照量新 clone，freeze record 也可能指向未被實測的 commit | ✅ 成立：bootstrap 兩個 pass 都對「會移動的」`HEAD` 驗；acceptance 在快照之後才 `rev-parse HEAD`、clone 取的是 clone 當下的 HEAD（只有 acceptance 的報告驗了 clone ＝ repo）；sizing 的報告⛔ 沒驗，只靠 `--formal` 時 freeze record 的自我驗證間接擋下 | bootstrap 的 snapshot pass **只解析一次** `HEAD^{commit}`（`BOOT_HEAD`），`--formal` 的前驗與後驗都對它的 blob；以 `SIZING_BOOT_HEAD` 交給 re-exec pass（驗它是 40 碼、而且是啟動 repo 裡的 commit；`--formal` 再對它驗一次快照）。主體：`repo_head` ＝ `BOOT_HEAD`（⛔ 不再讀 HEAD）；acceptance 的兩層複本與 sizing 的工作複本都在 clone 之後 `checkout --detach BOOT_HEAD`（⑩ 的工作複本同樣 detached 在 `repo_head`）並斷言；②的正式程式、freeze record 依賴的兩個模組改驗「在 `BOOT_HEAD` 裡」；`--formal` 在 clean 檢查之後要求啟動 repo 的 HEAD 仍是 `BOOT_HEAD`（否則在建 work 目錄之前拒絕）；sizing 的 `build_report()` 也要求 `repo_head`、`clone_head` 都存在且相等。freeze record 的 `repo_head` 取自報告，因此是同一個 OID |
| 2 | 中：計畫要求 acceptance 的報告收錄「mem-guard 實際上限」，實作只有固定的 450 MiB 門檻，正式報告無法自行證明當次的實際限制 | ✅ 成立：容器的 `--memory` 只存在封存的 argv 裡，sidecar 與報告都沒記 | shim 在 `docker rm` 之前多讀一次 `docker inspect` 的 `HostConfig.Memory`／`MemorySwap`（daemon 實際套用的），sidecar 記成 `memory_limit_bytes`／`memory_swap_limit_bytes`（兩個 profile 都記；讀不到 → 量測失敗、⛔ 不寫 0）。acceptance 的報告逐 invocation 驗：上限 > 0、各自 ＝ 封存的 argv（index；hash 綁住）裡**恰好一個** `--memory`（或 `-m`）與 `--memory-swap` 依 docker 的寫法（1024 進位）解析出的 bytes、而且 `MemorySwap ＝ Memory`——任一不成立 → ⛔ 不產報告（結束碼 1）。記憶體列加 `memory_limit_bytes`，`limits` 加 `container_memory_limit_bytes`（排序後的相異值），文字版每列印「／上限」；notes 分兩類逐一列出容器——上限不高於門檻的，判定實質是「在這個上限內、⛔ 不用 swap、以預期的結束碼跑完」；高於門檻的，峰值含 page cache、會隨當次的上限上升（⚠️ 第一版只看「最低的上限」就概括全部容器，本輪的實跑看到上限因容器而異之後改成逐一列出） |
| 3 | 低：re-exec 只封閉檢查 `$S/harness` 裡的 regular files，⛔ 沒有確認 S 根目錄恰好只有 `harness`，也⛔ 沒拒絕額外的空目錄或特殊檔案，通過之後卻會遞迴刪除整個 S——超出已裁決的「合法手動快照本身可被清除」界線 | ✅ 成立（原本是 `find "$S/harness" -type f`） | re-exec pass 在讀任何檔案內容**之前**（FIFO 之類的特殊檔案⛔ 不會被讀到而卡住），以 `find "$S" -mindepth 1 -printf '%y %P\0'` 排序後的 SHA 比對「恰好 `harness/`、清單 ① 的檔案、它們的上層目錄、`MANIFEST`」（NUL 分隔：檔名含換行也⛔ 不會混淆）——多任何檔案、目錄、symlink 或特殊檔案都拒絕（bootstrap 的 trap：⛔ 不刪任何東西）；`find` 讀不了任何目錄也拒絕。取代原本只看 regular file 的集合檢查 |

**與計畫的差異（本輪新增）**：

| # | 差異 | 理由 |
|---|---|---|
| 7 | 容器的 `MemorySwap ≠ Memory`（可以用 swap）→ fail-closed（計畫只要求記錄上限） | 可以用 swap 時 cgroup v1 的 `memory.max_usage_in_bytes` 不含被換出的部分，峰值會低估、門檻判定失去意義；⑩ 的所有容器（runner 的 `MEMSWAP` 預設 ＝ `MEM`、finalizer 與驗證模式明寫 `--memory-swap` ＝ `--memory`）現況⛔ 不受影響 |
| 8 | acceptance 的「模擬的真正 repo」（`<work>/real`）也 detached 在 `repo_head`（計畫只寫「兩層複本」） | 兩層綁同一個 OID；promoter ⛔ 不讀真正 repo 的 HEAD 或分支，晉升的行為⛔ 不受影響 |

**本輪的測試**：pytest——`test_i074_stage2_acceptance.py` ＋36（docker 的大小寫法 18、argv 的 `--memory` 解析與拒絕 9、報告逐列記錄 1、fail-closed 7、sidecar 1）、`test_i074_stage2_sizing.py` ＋3（`clone_head ≠ repo_head`、缺 `repo_head`、缺 `clone_head`）；shell——`test-replay-args.sh` 的 sizing shim ＋3（sidecar 記上限、多出 token、空字串）、⑦d 一節 ＋13（HEAD 在 bootstrap 之後移動：acceptance 與 sizing 的綁定、兩個入口的 `--formal` 拒絕與對照組；S 的形狀：根目錄的兄弟檔案、根目錄的額外目錄、`harness/` 裡的空目錄、FIFO、symlink、`MANIFEST` 換成 FIFO；`SIZING_BOOT_HEAD` 不是 commit；合法手動快照的對照組）。既有的手動快照兩支改成同時斷言**實際的失敗原因**（之前只斷言「失敗且⛔ 不刪」——re-exec 多了 `SIZING_BOOT_HEAD` 的驗證之後，沒帶它的舊測試會在別的地方失敗而空過）。

**本輪的實跑**（validation；⛔ 不是正式量測）：

| 項目 | 結果 |
|---|---|
| acceptance（stub） | ✅ 2026-10-02 08:48～08:55Z，`~/i074_stage2_acceptance/dev5-r1-20261002T084817Z/`；結束碼 0、`status: ok`；`repo_head` ＝ `clone_head` ＝ 兩層複本的 HEAD ＝ `9af7942`（detached）；success `P_path` 153.5、failure 69.0 MiB；晉升 59.9／53.1 MiB；十二個容器的上限**各自不同**（402～532 MiB，argv ＝ inspect、swap ＝ memory 全部通過）；host 最大單一 48.7～51.8 MiB、群組取樣 71.4～98.4 MiB；`MemAvailable` 低點 285.3 MiB。⚠️ 報告是改 notes 之前的版本產的（只差 notes 那兩則）——以本輪最終的 helper 對同一份 `raw/` 重產：頂層只有 `notes` 不同（原本一則概括全部，重產後兩則恰好列出 #1 與 #2～#12），其餘欄位全部相同 |
| sizing | ✅ 2026-10-02 08:56～09:02Z，`~/i074_stage2_sizing/validation-7d-r1-20261002T085639Z/`；`status: ok`、**`P_B` ＝ 160,960,512 bytes（153.5 MiB）**（⑦d 初版的實跑 160,956,416，＋4 KiB）；`repo_head` ＝ `clone_head` ＝ `9af7942`、工作複本 detached；十個 sidecar 都記下上限（458～508 MiB） |

⚠️ **本輪實跑的新發現**（✅ 2026-10-05 裁決：方向 (b)，另記 `total_rss` 並以它判定，再加一道精確的單一程序 RSS 閘——計畫見下方「Stage 2 步驟 ⑦d 增補計畫：容器記憶體改以 RSS 判定」）：mem-guard 每一次都依當下的 `MemAvailable` 下修 `--memory`，**同一趟裡各容器的上限就差到 130 MiB**；
而門檻比的是**含 page cache** 的 cgroup 峰值（⚠️ 更正：這個量法是 ⑦d 細部計畫「二之五」沿用 ④⑤ 定的，v29「六、1」只寫門檻 < 450 MiB），page cache 要逼近上限才被回收，所以**峰值會隨上限上升**：
同一個 stub replay（success）在 ⑦d 初版的實跑是上限 444m → 峰值 413.8 MiB，本輪是上限 466 MiB → **峰值 449.4 MiB（471,228,416 bytes；距門檻
471,859,200 只剩 630,784 bytes）**。也就是說，⑨-1 的正式驗收（full）可能只因為當次 mem-guard 給的上限較高、page cache 累積較多而得到
`threshold_exceeded`（偏保守的方向，⛔ 不會誤判通過；但結果與當下的 host 狀態有關）；反過來，上限不高於門檻的容器，門檻判定本身是空的
（實質是「在上限內跑完」）。可能的方向（⛔ 未決定）：(a) 維持 v29 的定義，照實接受；(b) acceptance 另記 cgroup 的 `memory.stat`
（`total_rss` 等不含 page cache 的量）並以它判定；(c) acceptance 以固定的 `MEM`（不高於門檻）執行。任何一個都會動到 v29「六、1」已確認的
量法或 ⑦d 的計畫，需另行裁決。

**本輪的反向驗證**（逐項注回 → 對應測試變紅 → 逐位元還原、比 SHA；pytest 那一層用既有的 harness，shell 那一層把 ⑦d 一節與 sizing shim 的 a1～a4 抽成獨立的腳本跑）：

| # | 注回的缺陷 | 結果 |
|---|---|---|
| R1 | sizing 的報告⛔ 不驗 `clone_head ＝ repo_head` | ✅ 紅：`clone_head ≠ repo_head` 那一支 |
| R2 | 容器可以用 swap 也接受 | ✅ 紅：fail-closed 的 swap |
| R3 | ⛔ 不比對封存的 argv 與 daemon 實際套用的上限 | ✅ 紅：fail-closed 的「≠」 |
| R4 | 上限 0（沒有上限）也接受 | ✅ 紅：fail-closed 的「沒有記憶體上限」 |
| R5 | 報告⛔ 不記錄每個容器的上限 | ✅ 紅：逐列記錄 |
| R6 | sidecar 把讀不到的上限當成 ok | ✅ 紅：sidecar 那一支 |
| R7 | acceptance 的 `repo_head` 改回讀 HEAD | ✅ 紅：acceptance 的綁定 |
| R8 | acceptance 的兩層複本⛔ 不 checkout bootstrap 的 OID | ✅ 紅：acceptance 的綁定 |
| R9 | sizing 的 `repo_head` 改回讀 HEAD | ✅ 紅：sizing 的綁定 |
| R10 | sizing 的工作複本⛔ 不 checkout bootstrap 的 OID | ✅ 紅：sizing 的綁定 |
| R11 | `--formal` ⛔ 不驗 HEAD 仍是 bootstrap 的 OID（兩個入口） | ✅ 紅：兩個入口的 `--formal` 各一支 |
| R12 | 拿掉 S 的形狀檢查 | ✅ 紅：手動快照的 extra、sibling、rootdir、emptydir、fifo、symlink、manifest-fifo 七支 |
| R13 | 形狀檢查放到讀 `MANIFEST` 之後 | ✅ 紅：manifest-fifo（讀 FIFO 卡住、由 timeout 收掉，結束碼 137） |
| R14 | ⛔ 不驗 `SIZING_BOOT_HEAD` | ✅ 紅：手動快照的 head |
| R15 | shim ⛔ 不把 inspect 讀到的上限交給 sidecar（改成固定值） | ✅ 紅：sidecar 的「多出 token」「空字串」兩支。⚠️ 反向驗證腳本第一次把預期的名稱寫成 pass 訊息的字樣而判成「不符」——紅的確實是這兩支，改正名稱後重跑一次確認 |

⚠️ 測試自身的修正：sizing shim 的 fake docker 原本用 `${FAKE_MEMORY:-…}`，空字串會被換成預設值，「空的上限」那一支因此沒有打到（第一次完整執行時唯一的失敗）——改成 `${FAKE_MEMORY-…}`。

**本輪的驗證**：`python/scripts/test.sh` 完整執行（依序）✅——pytest **2031 passed、1 skipped**（本輪 ＋39）；`test-replay-args.sh` **434 項**（本輪 ＋16：sizing shim 3、⑦d 一節 13）、`test-i074-stage2.sh` **223 項**（含 host unittest 37 項，兩份 bootstrap 除常數之外逐字相同的那一支照樣通過）全部通過；doc-refs 45／45、文件引用 0 個問題。真正 repo 的 worktree 登記數 ＋4（[I-118](#i-118replay-相關腳本會洩漏-git-worktree註冊與-tmp-目錄都會累積) 既有的增量；本輪的測試 0 增加）；`/dev/shm/i074-*`、`/run/lock` 的 `i074-stage2.*`、帶 `i074.stage2.run` 的容器都沒有殘留。tooling 路徑⛔ 沒有動：stage 之後 `--verify "$(git write-tree)"` 通過（仍是 `5ff9a90f…`）。

**review 之後**（commit 由使用者決定）：⑦d ⛔ 沒有動 tooling 路徑——commit 後再跑一次 `scripts/make-i074-tooling-patch.sh --verify "$(git rev-parse 'HEAD^{tree}')"`
（預期不變）。⚠️ sizing 的 harness 已改變：⑨-2 的正式 sizing 必須以包含本包的 commit 為 `repo_head`。

**歸檔**（⚠️ 依 CLAUDE.md，本筆的計畫與結果保留到 review 確認後才收斂）：操作程序寫進 [`development-workflow.md`](./development-workflow.md)
（sizing 一節的 ⑦d 改動、新增「I-074 Stage 2 的 memory／disk acceptance harness 與 ⑩ 的 observer」），現況規格寫進
[`sr-zone-scoring.md`](./sr-zone-scoring.md) 新增的「I-074 Stage 2 的容量驗收」。

#### Stage 2 步驟 ⑦d 增補計畫：容器記憶體改以 RSS 判定（2026-10-05，v11，✅ **已確認**（2026-10-05，review 十輪後通過並 commit `d1267bb`））

⚠️ 來源：⑦d 實作第一輪 review 之後的實跑發現（見上方「⑦d 實作結果」的「本輪實跑的新發現」）——容器的 cgroup 峰值含 page cache，
page cache 要逼近上限才被回收，所以**峰值會隨 mem-guard 當次給的上限上升**（同一個 stub replay：上限 444m → 413.8 MiB、466 MiB → 449.4 MiB）。
使用者 2026-10-05 的裁決：

1. **另記 `memory.stat` 的 `total_rss`（不含 page cache）並以它判定**；cgroup v1 沒有 `total_rss` 的 high-water mark、只能取樣（下界），所以再加一道
   精確的閘（`getrusage`），兩道都 < 450 MiB 才算通過。
2. **契約縮小**（計畫第三輪 review 之後）：驗收的對象改成「**經正常 wait 鏈保存 resource usage 的每一個程序**各自 < 450 MiB」——被 kernel 自動回收的子孫
   （parent 把 `SIGCHLD` 設成 `SIG_IGN` 或用 `SA_NOCLDWAIT`）**在契約外**；v29「六、1」、報告 `status` 的定義與歸檔文件同步修改（「二」⑨）。
3. **commit 邊界**：在 `f20ce3c`（⑦d 實作，已 commit）之上**另立增補 commit**——本計畫確認後一個（只動文件）、實作 review 後一個；⛔ 不 amend `f20ce3c`。

**驗收語意**（v11）：

- **契約**：經正常 wait 鏈保存 resource usage 的每一個程序——wrapper（或 `host-run`）自己、leader，以及被收掉的子孫（含被 PID 1／subreaper 收養、再由它收掉的孤兒）——
  各自的 RSS high-water < 450 MiB。**通過的證明**是精確的閘 `max(RUSAGE_SELF, RUSAGE_CHILDREN)`，前提是 leader 結束後收到 `ECHILD`。
- **契約外**：被自動回收的子孫（`getrusage(2)`、`wait(2)`：resource usage 被丟棄，最上層照樣收到 `ECHILD`）。⚠️ 契約外⛔ 不等於放任：`SIGCHLD=SIG_IGN`
  在執行期取樣偵測、偵測到即 fail-closed；本 repo 的程式靜態禁止設定它與 `SA_NOCLDWAIT`（「二」⑧）。報告明列這個範圍。
- **單向偵測器**：取樣的 `total_rss` 觀察到 ≥ 450 MiB 就判超標；沒觀察到⛔ 不是通過的證據。
- **`status = ok`** ＝ 能力檢查與每一步的逐步檢查都通過、契約內每一個程序的精確閘都 < 450 MiB、偵測器沒有觸發（以及既有的磁碟門檻與單向警報）。
- **⛔ 不在 host 留下程序**（v5，結構性的保證）：harness 自己是 subreaper（「二」④），任何步驟留下的程序——含 `host-run` 被 KILL、或在 TERM 的寬限期間才 fork、
  才 `setsid()` 的——都會被 harness 收養、隨時可列舉並清掉；清不掉 → 保留 S、列出 PID。⚠️ 前提是 **harness 自己沒有被 KILL**（host 當機或
  harness 被 `kill -9` 時沒有任何程式能收尾）；另有一個極短的 PID 重用競態（「二」④），兩者照實標示。

⚠️ 確認之前⛔ 不改任何程式。

##### 一、目標與⛔ 不做

| 項目 | 內容 |
|---|---|
| 目標 | acceptance profile 的**容器程序**改以 RSS 判定（上方「驗收語意」）；原本含 page cache 的 cgroup 峰值照樣記錄，改成**資訊值**（⛔ 不再產生違反）。host 端的 `host-run` 補上同一個缺口；harness 自己成為 subreaper，`host-run` 以任何方式結束（正常、逾時、被 TERM、被 KILL）留下的子孫都由 harness 收養並清掉（「二」④；前提與殘餘見上方「驗收語意」）。**提早失敗**：量測開始之前先驗一次 cgroup v1 的能力，每一步結束後立刻檢查這一步的量測（「二」⑦）。⑩ 的 observer 另記 `total_rss` 的取樣峰值（下界）。報告升成 `i074_stage2_acceptance_report_v2` |
| ⛔ 不做 | ⛔ 不新增「整個容器 aggregate RSS < 450 MiB」的契約；⛔ 不宣稱涵蓋被自動回收的子孫；⛔ 不改 sizing profile（wrapper、sidecar 的既有欄位、報告與 ④⑤ 的數字都不變）；⛔ 不改 ⑩ 的任何程式、docker argv、`run-replay-offline.sh` 的 `MEASURE_PEAK`；⛔ 不改門檻 450 MiB、`P_B` 與磁碟的量法；⛔ 不支援 cgroup v2 的判定；⛔ 不 amend `f20ce3c`。第一輪 review 的上限驗證（argv ＝ inspect、`MemorySwap ＝ Memory`）照舊——被換出的匿名頁同樣不在 `total_rss` 裡 |

##### 二、設計

**① 容器內的 wrapper**（新檔 `python/scripts/i074_stage2_rss_wrapper.py`，只給 acceptance profile；⚠️ 3.9 相容，host 的測試用 python3——host 的 `python` 是 2.7）：

| 項目 | 規則 |
|---|---|
| 掛載與指令 | shim 以唯讀掛載 `<S>/harness/python/scripts/i074_stage2_rss_wrapper.py:/acceptance/rss_wrapper.py:ro`（**快照**的路徑，由 S 推導、⛔ 不接受其他來源），容器指令改成 `python -I /acceptance/rss_wrapper.py <原指令逐 token>`（取代 sizing 的 `sh -c "$PEAK_WRAPPER"`）；replay 的 launcher 改寫照舊，外面再包這一層。wrapper 是容器的 **PID 1** |
| 收養 | 是 PID 1 → 孤兒本來就由它收養；⛔ 不是 PID 1（host 上的測試）→ 先以 `prctl(PR_SET_CHILD_SUBREAPER)` 設成 subreaper，失敗 → 記錯誤（fail-closed）。記下 `reaper`（`pid1`／`subreaper`） |
| 執行 | `subprocess.Popen(原指令)`（stdin／stdout／stderr 原樣繼承：**I/O 透明**，wrapper 自己⛔ 不寫 stdout／stderr，錯誤只寫 `/peak/rss.json`）；等 leader 結束 → 以 `waitpid(-1, WNOHANG)` 收掉其餘子孫，直到 `ECHILD`；寬限 **10 秒**內收斂不了 → 記錯誤（fail-closed）、⛔ 不殺它們（wrapper 結束時 docker 會清掉整個容器，⛔ 不會留在 host） |
| 精確的閘 | 收斂到 `ECHILD` 之後才讀：`self_max_rss_bytes` ＝ `RUSAGE_SELF.ru_maxrss`、`children_max_rss_bytes` ＝ `RUSAGE_CHILDREN.ru_maxrss`、`max_single_rss_bytes` ＝ 兩者取大。⚠️ `ru_maxrss` 含 file-backed 的常駐頁（共用函式庫、mmap）——比匿名記憶體偏保守 |
| 單向偵測器 | 背景 thread 每 **50 ms** 讀 `/sys/fs/cgroup/memory/memory.stat` 的 `total_rss`（v1；容器內看到的就是自己的 cgroup，與既有 wrapper 讀 `max_usage_in_bytes` 同一個位置；v1 的 `total_rss` ＝ 匿名頁 ＋ swap cache，⛔ 不是一般意義的完整 RSS）；保留最大值與次數；收斂之後再讀最後一次。讀不到 → 記錯誤（⛔ 不改讀 v2） |
| 自動回收的偵測 | 同一個 thread 每次取樣也掃容器的 `/proc/[0-9]*/status`（容器有自己的 PID namespace，看到的只有容器內的程序），任一程序的 `SigIgn` 含 `SIGCHLD` → `auto_reap_detected=true`（fail-closed）。⚠️ 取樣：存活不到 50 ms 的 parent 可能漏掉；`SA_NOCLDWAIT` 看不到——兩者都屬契約外（上方「驗收語意」） |
| cgroup 峰值（資訊值） | 收斂之後照舊讀 v1 `memory.max_usage_in_bytes` → v2 `memory.peak`，寫進 `/peak/peak`（格式與 sizing 相同） |
| 結束碼 | leader 的結束碼原樣傳回；leader 被訊號結束 → `128 + 訊號編號`（與 `sh` 相同，例如 OOM 的 137）；量測失敗⛔ 不改結束碼（由「二」⑦ 的逐步檢查立刻 fail-closed） |
| 覆寫 | 沿用 `SIZING_CGROUP_ROOT`／`SIZING_PEAK_DIR`（只給 host 上的測試）；寬限、間隔與 `/proc` 的位置是函式參數，測試以參數覆寫（⛔ 不開環境變數） |

**② `rss.json` 的封閉 schema**（canonical；sidecar 逐條驗，任一不符 → 量測失敗、⛔ 不寫 0）：

| 鍵 | 規則 |
|---|---|
| （整體） | 鍵集合**恰好**是下列十二個（⛔ 不得缺欄、⛔ 不得多欄）；整數欄位必須是 `int` 且⛔ 不是 `bool`；布林欄位必須是 `bool` |
| `schema` | ＝ `i074_stage2_rss_v1` |
| `rss_source` | 封閉 enum：只有 `v1:total_rss` |
| `rss_interval_ms` | ＝ 50（整數，⛔ 不用浮點比較） |
| `rss_samples` | 整數 > 0 |
| `rss_peak_sampled_bytes` | 整數 > 0 |
| `self_max_rss_bytes`、`children_max_rss_bytes` | 整數 > 0 |
| `max_single_rss_bytes` | 整數 ＝ `max(self_max_rss_bytes, children_max_rss_bytes)` |
| `reaper` | 封閉 enum `pid1`／`subreaper`；容器內必須是 `pid1`（sidecar 驗） |
| `all_descendants_reaped` | 必須是 `true` |
| `auto_reap_detected` | 必須是 `false` |
| `errors` | 必須是**空的**字串陣列（wrapper 有錯誤時照樣寫出、非空 → 量測失敗） |

**③ sidecar 與報告**：

| 項目 | 規則 |
|---|---|
| sidecar | 依 index 的 `profile`：acceptance 才讀 `rss.json`、依 ② 驗；sizing ⛔ 不讀也⛔ 不要求 |
| 門檻 | 每一個容器：**`max_single_rss_bytes` ≥ 450 MiB 或 `rss_peak_sampled_bytes` ≥ 450 MiB → `threshold_exceeded`**（前者是精確閘、後者是單向偵測器；兩者都 < 450 MiB 才通過）。含 page cache 的 cgroup 峰值⛔ 不再產生違反 |
| 報告 schema | 升成 **`i074_stage2_acceptance_report_v2`**；頂層新增三個**封閉**的欄位：`contract`（字串，＝ `per_process_rss_within_wait_chain`）、`out_of_contract`（陣列，恰好 ＝ `["descendants_auto_reaped_by_kernel"]`）、`auto_reap_detection`（物件，恰好 ＝ `{"sigchld_sig_ign": "sampled_fail_closed", "sa_nocldwait": "unobservable"}`）。`violations` 改成**結構化**的陣列（每筆恰好 `kind`、`subject`、`value`、`limit`；`kind` 是封閉 enum：`container_max_single_rss`、`container_rss_sampled`、`host_max_single_rss`、`host_group_rss_sampled`、`disk_p_path`）。寫出之前以 `validate_acceptance_report_v2()` 驗證整份報告，不符 → ⛔ 不寫報告（harness 失敗）：① 頂層與重要巢狀列（`limits`、`paths.*`、`promotion.*`、`memory[]`、`host[]`、`violations[]`）的**封閉鍵集合與型別**（整數排除 `bool`）、三個固定值、`status` 的 enum；② **衍生欄位的內部一致性**：`memory[]` 與 `host[]` 的 `max_single_rss_bytes` ＝ `max(self_max_rss_bytes, children_max_rss_bytes)`；`paths.*` 的 `accounted` ＝ `accounted_parts` 的總和、`P_path` ＝ `max(peaks.dirs_peak, peaks.fs_peak, accounted)`；`promotion.*` 的 `P_promotion` ＝ `max(peaks.dirs_peak, peaks.fs_peak, accounted)`、`P_path_plus_promotion` ＝ 對應路徑的 `P_path` ＋ `P_promotion`；`memory[].below_limit` ＝（精確閘與取樣都 < 門檻）、`host[].group_alarm` ＝（群組取樣 ≥ 門檻）；`limits.memory_bytes` ＝ `ACCEPTANCE_MEMORY_LIMIT`、**`limits.P_B_BUDGET` ＝ 正式常數**（validator 的參數 `p_b_budget`，呼叫端一律傳 `i074_stage2_preflight.P_B_BUDGET`——與產出端相同的來源；⛔ 不接受報告自己的值當基準）、`limits.container_memory_limit_bytes` ＝ 記憶體列上限的排序相異值；③ **由列重新推導門檻結果**——產出端與 validator **共用唯一的** `derive_acceptance_violations(report)`（⛔ 不寫兩份門檻邏輯）：`memory[]` 的精確閘與偵測器、`host[]` 的最大單一與群組取樣、`paths.*` 的 `P_path` 對 `limits.P_B_BUDGET`——推導出的違反必須與 `violations` **逐筆相等**（順序依列的順序）；④ `status = ok` ⟺ `violations` 為空。⚠️ 它驗的是**報告本身的一致性**；⛔ 不能驗證數字與原始量測（`raw/`）相符——那由產出端的流程保證。⚠️ v1 是 ⑦d 開發驗證的舊語意（dev4、dev5 兩份，判定量是含 page cache 的 cgroup 峰值），⛔ 不得當成 v2 的證據；現在⛔ 沒有任何程式讀 acceptance 報告，日後若有（例如 ⑨-1 的判讀），只接受 v2（以同一個驗證函式） |
| 記憶體列 | `max_single_rss_bytes`（`exact_within_wait_chain`）、`self_max_rss_bytes`、`children_max_rss_bytes`、`rss_peak_sampled_bytes`（`sampled_lower_bound`）與次數、`cgroup_peak_bytes`（`informational_includes_page_cache`）、`memory_limit_bytes`、`reaper` |
| notes | ① 契約與範圍（上方「驗收語意」）；② `ru_maxrss` 含 file-backed 的常駐頁、wrapper 自己（約 10 MiB）也計入——偏保守；③ 上限**嚴格低於**門檻的容器（逐一列出）：匿名頁受上限限制，`total_rss` 偵測器不可能觸發；⚠️ 精確的閘仍逐一比較（file-backed 頁可能記在別的 cgroup，⛔ 不受這個上限限制）；④ cgroup 峰值含 page cache、會隨當次上限上升，只列資訊值。取代第一輪 review 加的兩則 notes |
| 文字版 | 開頭印契約與範圍；每個容器一列：最大單一程序（精確）、RSS 取樣峰值（次數）、cgroup（資訊值）、上限 |

**④ host 端：`host-run` 與 harness 的收養**：

| 項目 | 規則 |
|---|---|
| `host-run` 的收養與精確閘 | spawn 之前先以 `prctl(PR_SET_CHILD_SUBREAPER)` 設成 subreaper（失敗 → fail-closed）；leader 結束後以 `waitpid(-1, WNOHANG)` 收到 `ECHILD`（寬限 10 秒）；`max_single_rss_bytes` ＝ `RUSAGE_SELF` 與 `RUSAGE_CHILDREN` 取大（`host-run` 自己是 Python、約 20 MiB，偏保守） |
| 取樣 | 每 **100 ms** 掃 `/proc`，以 ppid 鏈追到 `host-run` 的全部子孫（⛔ 不再只看原本的 process group——`setsid` 的子孫會逃出 group，但仍是 subreaper 的後代）：RSS 總和（單向警報）、`SigIgn` 含 `SIGCHLD` → `auto_reap_detected` |
| `host-run` 自己的清理 | **逾時**（寬限內收斂不了）或**收到 TERM／INT／HUP**：先記量測失敗（`all_descendants_reaped=false` 或 `errors` 記中斷），再進清理階段——**只對 ppid ＝ `host-run` 的直接子程序**送訊號（⚠️ 只有 `host-run` 能收它們，收之前 PID ⛔ 不會被重用，所以這一段沒有 PID 重用的競態）：TERM → 等 5 秒 → KILL → 收；孫程序在 parent 死後被收養成直接子程序，下一輪再處理；直到 `ECHILD`，整段上限 **15 秒**（< `stop_step_group` 升級 KILL 之前的 20 秒，所以 harness 的 TERM 通常等得到它清完）。記 `cleanup_complete` 與 `leftover_pids`；清不乾淨 → 結束碼 **70**。被 TERM／INT／HUP 時清理完各以 **143／130／129** 結束 |
| **harness 是 subreaper**（結構性的收養） | bootstrap 的 snapshot pass 以 `exec python3 -c <設 PR_SET_CHILD_SUBREAPER，再 execv /bin/bash>` 進入快照裡的主腳本（subreaper 屬性跨 `execve` 保留；bootstrap 區塊在兩個入口照舊逐字相同）。共用原語新增 `measure_subreaper_guard`，在六步清理的 trap 裝好之後**以行為驗證**：
① **啟動**：`GUARD_SHELL_PID="$( ( python3 -c … </dev/null >/dev/null 2>&1 & echo "$!" ) )"`——子 shell **直接**背景啟動 Python（⛔ 不經外部的 `setsid` 指令：util-linux 的 `setsid` 在呼叫者是 process group leader 時會自己 fork，`$!` 就成了短命的 parent），以 command substitution 回報 `$!` 後立刻結束（**封閉的通道**：恰好一行十進位整數，否則視為沒有）；測試程序的 stdio 全部導走（⛔ 不會撐住 command substitution 的 pipe）。所以 `$!` 在結構上就是測試程序本身的 PID。
② **測試程序自己回報身分**：一啟動先 `os.setsid()`（脫離 harness 的 process group，模擬逃出 group 的子孫）——**失敗就立刻結束**（它⛔ 沒有任何子程序，⛔ 不留後代、⛔ 不寫身分的 JSON）；成功才讀自己的 `/proc/self/stat`，把 PID 與 starttime 以暫存檔 ＋ rename 寫進 `<S>/guard-probe.json`（鍵恰好 `pid`、`starttime`，整數），然後睡 5 秒結束（本身有時限）；讀不到自己的 stat → ⛔ 不寫身分的 JSON、立刻結束。**失敗的狀態檔**：上面兩個失敗出口在結束之前，以暫存檔 ＋ rename 寫 `<S>/guard-probe-status.json`（canonical，鍵恰好 `schema`（＝ `i074_stage2_guard_probe_status_v1`）、`pid`（`os.getpid()`）、`stage`（封閉 enum：`setsid`、`self_stat`）、`errno`（`errno.errorcode` 的名稱，例如 `EPERM`））；寫不了就照樣結束。⚠️ 它**只供診斷與測試**：⛔ 不是身分來源（③ ⛔ 不讀它、⛔ 不依它送任何訊號），guard 中止時只把它的內容原樣印進訊息。
③ **信任身分的條件**（三者都成立）：`guard-probe.json` 在 2 秒內出現且格式正確；它的 `pid` ＝ `GUARD_SHELL_PID`；`/proc/<pid>/stat` 的 starttime ＝ 它的 `starttime`。成立 → 記成 `GUARD_PROBE`，驗它的 ppid ＝ harness 的 PID，不符 → 中止（⛔ 不能以手動 re-exec 跳過）。
④ **收尾**（所有出口）：身分可信 → `helper kill-pinned`（通過、ppid 不符、guard 期間收到訊號——`on_failure` 看到 `GUARD_PROBE` 就處理）；**身分不可信**（沒有 JSON、格式不符、兩個 PID 不相等、starttime 不符）→ ⛔ 不呼叫 `kill-pinned`、⛔ 不送任何訊號，只以 `GUARD_SHELL_PID` 等最多 6 秒、確認 `/proc/<pid>` 已消失或是 zombie，然後**中止**（subreaper 沒有被驗證）；`GUARD_SHELL_PID` 也沒有、或 6 秒後仍在 → 保留 S、列出能列的 PID。`kill-pinned` 結束碼 ≠ 0 → 保留 S、列出 PID |
| 收養的程序怎麼清（`helper reap-adopted`） | **先釘住 harness 自己的身分**：參數帶 harness 的 PID 與 starttime（bash 以 `/proc/$$/stat` 取得），helper 啟動時驗 `/proc/<parent>/stat` 的 starttime 相符、而且 `os.getppid()` ＝ parent（helper 由 harness 同步呼叫，是它的直接子程序）——不符 → 結束碼 2、⛔ 不送任何訊號；**每一輪列舉之前、每一個訊號之前**都再驗 `os.getppid()` ＝ parent（harness 一死，helper 立刻被別的程序收養、`getppid()` 跟著變；只要它還等於 parent，那個 PID 就仍是原本的 harness、⛔ 不可能已被重用），不符 → 立刻停止送訊號、結束碼 2。之後列舉 ppid ＝ harness 的程序（排除 helper 自己與 `--exclude` 列的已知子程序，例如取樣器）；每一筆以 `/proc/<pid>/stat` 的 starttime 釘住，**送每一個訊號之前**都再確認一次：TERM → 5 秒 → KILL → 5 秒；孫程序被收養上來就下一輪處理，整段上限 30 秒。⚠️ harness 是 bash，會在 `SIGCHLD` 時自行收掉死去的子程序——確認 starttime 與送訊號之間仍有極短的 PID 重用競態（kernel 4.19 ⛔ 沒有 pidfd：`pidfd_open` 回 ENOSYS，已實測），照實標示 |
| 什麼時候清 | ① 每一步結束後（共用的 `run_in_group`，在既有的「group 已無成員」檢查之後）：`reap-adopted --check-only`——有任何被收養的程序 ＝ 這一步把程序留在 host → `on_failure`；② `on_failure` 的步驟收尾（`stop_step_group`）之後、第二次容器清理之前：`reap-adopted`；③ 正常結束之前再 `--check-only` 一次 |
| `reap-adopted` 的封閉契約 | 參數 `--state <S> --parent <pid> --parent-starttime <整數> [--exclude <pid> …] [--check-only]`；結束碼：**0** ＝ 沒有被收養的程序（或全部清掉）、**1** ＝ 有（`--check-only`）或清完仍有殘留、**2** ＝ helper 自己失敗（任何例外、`/proc` 讀不到、檔案寫不了、parent 的身分不符或在執行期間改變）。殘留時寫 `<S>/leftover-pids.json`（canonical、write 到暫存檔再 rename；鍵恰好 `schema`（＝ `i074_stage2_leftover_pids_v1`）、`parent`（整數）、`processes`（依 pid 排序、pid 不重複的陣列，每筆恰好 `pid`、`starttime`（整數）與 `cmdline`（字串）））——只給操作者看，⛔ 沒有程式讀回它。`on_failure`：`reap-adopted` 的結束碼 ≠ 0（含 2）→ **保留 S**、印出 `leftover-pids.json` 或「helper 失敗」（與「步驟的 process group 沒有確實結束」同一個處置） |
| `kill-pinned` 的封閉契約 | 參數 `--pid <整數> --starttime <整數>`；只送訊號給**同時符合 PID 與 starttime** 的程序，**送每一個訊號之前**都再讀一次 `/proc/<pid>/stat`：`/proc/<pid>` 不在、starttime 不符（已不是它）、或 state 是 `Z`／`X`（已死、只等收屍）→ 視為已消失、⛔ 不送訊號；否則 TERM → 最多 2 秒 → KILL → 最多 2 秒。結束碼：**0** ＝ 已消失（含一開始就不在、身分不符）、**1** ＝ KILL 之後仍存活（同一個 starttime、⛔ 不是 zombie）、**2** ＝ helper 自己失敗（參數不合法、`/proc` 讀取發生 ENOENT 以外的錯誤）——**第一個訊號之前**失敗 → ⛔ 不送任何訊號；**TERM 之後**才失敗 → ⛔ 不再送後續的訊號（⛔ 不升級 KILL）、回 2。**輸出契約**：參數合法時，stdout 恰好一行 canonical JSON，鍵恰好 `schema`（＝ `i074_stage2_kill_pinned_v1`）、`pid`、`starttime`（整數）、`result`（封閉 enum：`already_gone`、`identity_mismatch`、`terminated_by_term`、`terminated_by_kill`、`alive_after_kill`、`error_before_signal`、`error_after_term`）、`signals_sent`（依序、只會是 `[]`、`["TERM"]`、`["TERM","KILL"]`）、`error`（字串或 `null`）；`result` 與結束碼一一對應（前四個 → 0、`alive_after_kill` → 1、兩個 `error_*` → 2）。參數不合法 → 結束碼 2、stdout ⛔ 沒有任何輸出（錯誤在 stderr）。呼叫端（guard 與 `on_failure`）把這一行原樣附加到 `<S>/kill-pinned.jsonl`，隨 S 複製進 `raw/`／`raw-failed/`；`on_failure` 的失敗摘要另列這個檔的路徑。⚠️ `reap-adopted` 判斷「仍存活」也同樣把 `Z`／`X` 視為已死 |
| 紀錄的封閉 schema | `<S>/host/<step>.json`（canonical），鍵集合**恰好**是下列十七個（⛔ 不得缺欄、⛔ 不得多欄）；整數排除 `bool`、布林必須是 `bool`：`schema`（＝ `i074_stage2_host_run_v1`）；`step`（字串，＝ 檔名、而且是報告預期的步驟）；`rc`（整數 0～255，＝ `rc.tsv` 記的實際結束碼）；`self_max_rss_bytes`、`children_max_rss_bytes`（整數 > 0）；`max_single_rss_bytes`（＝ 兩者取大）；`group_rss_peak_sampled_bytes`（整數 > 0，含 `host-run` 自己）；`group_samples`（整數 > 0）；`group_interval_ms`（＝ 100）；`reaper`（＝ `subreaper`）；`all_descendants_reaped`、`auto_reap_detected`、`cleanup_complete`（布林）；`leftover_pids`（整數陣列）；`errors`（字串陣列）；`env_keys`（排序、不重複的字串陣列）；`cmd`（非空的字串陣列）。**有效的量測**另要求 `all_descendants_reaped=true`、`auto_reap_detected=false`、`cleanup_complete=true`、`leftover_pids=[]`、`errors=[]`——任一不符 → 逐步檢查與報告都 fail-closed |

**⑤ observer**（⑩，外部唯讀）：每 0.5 秒讀 cgroup high-water 的同一個迴圈，另讀 v1 `memory/docker/<id>/memory.stat` 的 `total_rss`（⛔ 不讀 v2），每個容器另記 `rss_peak_bytes`（下界）與讀取次數；讀不到 → 列進 `rss_unavailable_containers`，`observation_complete` 另要求它是空的。

**⑥ 快照清單**：新 wrapper 加進 acceptance 的「清單 ①」（`I074_BOOT_FILES` 與 helper 的 `SNAPSHOT_FILES["acceptance"]`；既有的 host unittest 釘住兩者一致），報告的 `harness_manifest` 因此也綁住它。sizing 的清單⛔ 不變。

**⑦ 提早失敗**：

| 項目 | 規則 |
|---|---|
| 能力檢查 | 建 work 目錄**之後**、clone 之前（六步清理的 trap 已裝好）：以**真正的** docker（⛔ 不經 shim、⛔ 不進 invocation 索引）、**同一個 image**、同一種掛載跑一次 `python -I /acceptance/rss_wrapper.py python -c pass`（`--rm --network none --user <uid:gid> --cidfile <S>/cid/probe.cid --name i074sz-<run id>-probe`）——cidfile 與名稱都在既有 `cleanup_containers` 的範圍內；docker client 以 `setsid timeout -s KILL 120` 在背景執行、harness `wait` 它並把它記成 `STEP_PID`（被中斷時 `on_failure` 照常結束它的 process group、再以 cidfile／名稱清容器）。`rss.json` 依 ② 驗（含 `rss_source = v1:total_rss`、`reaper = pid1`）；不符、逾時、docker 失敗 → `on_failure`（容器清理、`raw-failed`、摘要），⛔ 不 clone、⛔ 不開始任何量測。結果記進 meta（`rss_capability`）。演練用的故障注入 `probe-timeout`（把逾時縮成 2 秒；`--formal` 照舊拒絕所有故障注入） |
| 逐步檢查 | `step()` 在比對完結束碼之後立刻 `helper check-step --state "$S" --step <名>`：驗**這一步**的 host 紀錄（④ 的封閉 schema 與有效條件，含 `rc` ＝ `rc.tsv`）與目前為止所有 sidecar（status 與 ② 的 schema）；任一不符 → `on_failure`，⛔ 不再執行之後的步驟（⑨-1 的量測趟排在最後，前面任何一步的量測失敗都⛔ 不會讓它開跑） |

**⑧ 自動回收的前提怎麼守**：

| 層 | 做法 |
|---|---|
| 執行期（容器與 host） | ① 的 wrapper 與 ④ 的 `host-run` 每次取樣都看每一個子孫的 `SigIgn`；含 `SIGCHLD` → fail-closed |
| 靜態（本 repo 的程式） | 測試以**語意**掃 `python/`、`scripts/`（測試除外）。Python（AST）：先解析每個檔的 `signal` 名稱來源——`import signal`、`import signal as X`、`from signal import signal／SIGCHLD／SIGCLD／SIG_IGN／Signals [as Y]`——再找呼叫目標解析成 `signal.signal`、第一個參數解析成 `SIGCHLD`／`SIGCLD`（含 `Signals.SIGCHLD` 與整數字面值 17）、處理函式（位置或 `handler=`）解析成 `SIG_IGN`（含 `Handlers.SIG_IGN`）的呼叫；任何名為 `SA_NOCLDWAIT` 的識別字或字串。Shell：`trap ''`／`trap ""`／`trap -- ''` 的對象含 `CHLD`／`SIGCHLD`。⛔ 不禁止「提到 `SIGCHLD`」（偵測器要用它算 `SigIgn` 的位元）。⚠️ 動態寫法（`getattr`、`exec`、以 ctypes 直接呼叫 `sigaction`）⛔ 不在靜態檢查內——由執行期偵測涵蓋 `SIG_IGN`。現況 0 處（2026-10-05 實查） |
| 契約外（照實標示） | `SA_NOCLDWAIT` 與存活不到一個取樣間隔的 `SIG_IGN` parent：報告的 `out_of_contract` |

**⑨ 對已確認章節的修改**（本計畫確認後、隨計畫書 commit 加註；⚠️ 都標「⑦d 增補（2026-10-05 確認）」）：

| 章節 | 修改 |
|---|---|
| v29「六、1」 | 「replay 程序（涵蓋 a～c）及 d～i 各獨立程序，皆各自 < 450 MiB」與「acceptance 門檻：< 450 MiB」兩處加註：判定量是 RSS（精確閘 ＋ 單向偵測器），契約限於經正常 wait 鏈保存 resource usage 的程序，被自動回收的子孫在契約外；含 page cache 的 cgroup 峰值只列資訊值 |
| ⑦d 細部計畫「二之三」 | 流程加能力檢查（建 work 目錄之後、clone 之前）與每一步之後的逐步檢查 |
| ⑦d 細部計畫「二之四」 | acceptance 的改寫：容器指令改用 wrapper（⛔ 不再是 `sh -c` 的 cgroup 峰值 wrapper） |
| ⑦d 細部計畫「二之五」 | 記憶體、門檻、`status`、報告內容四列改成本增補；報告 schema 改 v2 |
| ⑦d 細部計畫「二之六」 | observer 另記 `total_rss` |
| ⑦d 細部計畫「五」 | 加 ac20～ac27、ac24b、ac24c；**既有的 ac6 改寫**：容器那一半改成精確閘與偵測器的邊界（各自 ＝ 471,859,200 → 超標、−1 → 通過），cgroup 峰值 ＝ 471,859,200 → ⛔ 不影響 status（資訊值）；host 那一半、磁碟與其餘照舊。**既有的 ac16 改寫**：「取樣只算同一個 process group」改成「以 ppid 鏈追到的全部子孫（含 `setsid` 的）」，其餘照舊；兩者的測試（`test_ac6_container_and_host_boundaries`、`test_host_run_*`）同步改，⛔ 不留過期的斷言 |
| ⑦d 細部計畫「二之三」（bootstrap） | snapshot pass 以 python3 啟動器設 subreaper 再 exec 快照裡的主腳本；共用原語加 `measure_subreaper_guard` 與 `reap-adopted` 的三個時機 |
| ⑦d 實作結果 | 「本輪實跑的新發現」那一段的「v29「六、1」」出處已更正（本計畫 v1 起） |

##### 三、本增補補上的決定（⚠️ 待確認）

| # | 決定 | 理由 |
|---|---|---|
| 1 | wrapper 是獨立的 Python 檔、以快照路徑唯讀掛載（⛔ 不寫成 shim 裡的 inline 字串） | pytest 能直接測；綁進 `harness_manifest`；與 replay launcher 同一個做法 |
| 2 | 取樣間隔：容器 50 ms、host 100 ms | 它們只是偵測器，間隔⛔ 不影響通過的證明；host 要掃整個 `/proc`，間隔放寬 |
| 3 | 通過的證明 ＝ `max(RUSAGE_SELF, RUSAGE_CHILDREN)`，前提是 PID 1／subreaper 收到 `ECHILD` | 契約（縮小後）的直接量法 |
| 4 | 容器內：leader 結束後 10 秒內收斂不到 `ECHILD` → fail-closed、⛔ 不殺 | 存活的子孫的 high-water 讀不到；wrapper 結束時 docker 會清掉整個容器 |
| 5 | 只接受 cgroup v1 的 `total_rss`（v2 的 `anon` ⛔ 不讀；`rss_source` 封閉 enum） | v2 的 `anon` 與 v1 的 `total_rss` 不同義，而且只有 fake 測試；本 host 是 v1。移到 v2 host 時再以實機驗證另行裁決 |
| 6 | 含 page cache 的 cgroup 峰值降為資訊值 | 使用者的裁決；保留它才看得到與 ④⑤ 的對照 |
| 7 | observer 也另記 `total_rss` 的取樣峰值 | 與 acceptance 的偵測器同一種量；⑩ 只額外記錄，⛔ 不作為前置（v29「八」） |
| 8 | sizing 的**量法**完全不變（wrapper、sidecar 的既有欄位、報告與數字）；⚠️ 共用的 bootstrap（subreaper）、`run_in_group` 的收養檢查與 `on_failure` 的 `reap-adopted` 也套用到 sizing（「三」#15、#16） | ⑤ 的數字與 ⑨-2 的正式 sizing 以同一種量法比較；清理的強化對 sizing 同樣成立 |
| 9 | `host-run` 補上 subreaper、收到 `ECHILD`、計入 `RUSAGE_SELF`、以 ppid 鏈取樣 | 與容器的精確閘是同一個缺口 |
| 10 | 契約縮小（使用者的裁決）；`SIGCHLD=SIG_IGN` 執行期偵測、本 repo 語意靜態禁止 | 要涵蓋任意子孫需要觀察每一個 process exit（ptrace／taskstats），⛔ 不在本增補 |
| 11 | 能力檢查在建 work 目錄之後、clone 之前；生命週期交給既有的六步清理（cidfile、名稱、`STEP_PID`） | 失敗時沿用 `raw-failed` 與摘要的既有處置；⛔ 不另寫一套清理 |
| 12 | `host-run` 的程序內清理只對直接子程序送訊號；harness 的 `reap-adopted` 以 starttime 釘住 PID | 前者沒有 PID 重用的競態；kernel 4.19 ⛔ 沒有 pidfd，後者是能做到的最小競態 |
| 13 | 報告 schema 升成 v2、頂層記 `contract` 與 `out_of_contract` | 判定語意變了；dev4、dev5 的 v1 報告是舊語意的開發紀錄 |
| 14 | 另立增補 commit（使用者的裁決）：計畫書 commit（只動文件）→ 實作 commit | ⛔ 不 amend 已 commit 的 `f20ce3c` |
| 15 | **harness 自己是 subreaper**（bootstrap 以 python3 啟動器設定、以行為驗證），取代 v4 的 `.live` 快照 | 快照有漏失窗口（最後一次快照之後、TERM 寬限期間才 fork 的程序）；收養是結構性的，隨時可列舉。⚠️ 動到已 commit 的 bootstrap 區塊（兩個入口照舊逐字相同） |
| 16 | 每一步之後檢查「harness 有沒有被收養的程序」，有就 `on_failure`（sizing 也套用——共用原語） | 步驟把程序留在 host 本身就是 harness 的失敗；sizing 的步驟本來就⛔ 不該留程序 |
| 17 | 報告 v2 的三個新欄位是封閉的固定值；`violations` 結構化；驗證函式驗封閉 schema 並由列重新推導門檻結果 | 日後的讀取端（例如 ⑨-1 的判讀）用同一個函式；它保證報告本身一致，⛔ 不保證數字與原始量測相符 |
| 18 | `reap-adopted` 以 `getppid()` 確認 parent 仍是原本的 harness（每一輪、每一個訊號之前） | helper 是 harness 的直接子程序：`getppid()` 等於 parent 的期間，那個 PID ⛔ 不可能已被重用；不必等 pidfd |
| 19 | guard 的測試程序由子 shell 直接背景啟動、自己 `os.setsid()` 並回報 PID 與 starttime，而且必須 ＝ 子 shell 以封閉通道回報的 `$!`；身分不可信 → ⛔ 不送訊號、等它自己結束、中止 | `$!` 在結構上就是測試程序；沒有可信的身分就沒有安全的訊號；兩個獨立來源一致才信；測試程序本身最多活 5 秒 |
| 20 | 被訊號中斷時 harness 的外部結束碼維持 **1**（既有契約），原始訊號的結束碼記在 `failure_summary.json` | 不遷移既有的操作契約（`development-workflow.md` 的「1 ＝ harness 失敗」）；`host-run` 自己的 143／130／129 是步驟層的結束碼，與此無關 |

##### 四、受影響檔案與資料流

| 檔案 | 改動 |
|---|---|
| `python/scripts/i074_stage2_rss_wrapper.py`（新增） | 「二」① |
| `scripts/lib/i074-sizing-docker-shim.sh` | acceptance profile：掛載 wrapper、容器指令改用它；sizing 照舊 |
| `scripts/i074-stage2-acceptance.sh`、`scripts/i074-stage2-sizing.sh` | bootstrap 區塊的 exec 改經 python3 啟動器（subreaper；兩個入口逐字相同）；acceptance：`I074_BOOT_FILES` 加新檔、能力檢查、`step()` 的逐步檢查、故障注入 `probe-timeout`；sizing：故障注入 `orphan-setsid`（步驟留下一個 `setsid` 的程序）；兩個入口都接受故障注入 `subreaper-stall`（guard 記下測試程序之後停住）、`guard-noident`（測試程序⛔ 不回報身分）、`guard-badident`（回報錯的 PID）與 `guard-pgleader`（測試程序先自成 process group leader，讓 `os.setsid()` 失敗）；`--formal` 照舊拒絕所有故障注入 |
| `scripts/lib/i074-stage2-measure.sh` | `measure_subreaper_guard`（含 `GUARD_PROBE` 在所有出口的收尾）；`run_in_group` 之後的收養檢查；`on_failure` 的 `reap-adopted` 與「結束碼 ≠ 0 → 保留 S」；正常結束前的收養檢查 |
| `python/scripts/i074_stage2_sizing.py` | `SNAPSHOT_FILES["acceptance"]`、兩份 schema 的驗證、sidecar、報告 v2（門檻、記憶體列、三個封閉欄位、`validate_acceptance_report_v2()`、notes、文字版）、`derive_acceptance_violations()`、`host-run`（④）、`reap-adopted`、`kill-pinned`、`check-step`、能力檢查的驗證、observer（⑤） |
| 測試 | pytest：wrapper、兩份 schema、sidecar、報告 v2、observer、靜態掃描、既有的 ac6 改寫；shell：`test-replay-args.sh` 的 ac4、能力檢查、逐步檢查、harness 的收養（ac28）；host unittest：`host-run`（含既有的 ac16 改寫）與 `reap-adopted` |
| 文件 | 本筆（「二」⑨）、`sr-zone-scoring.md`「I-074 Stage 2 的容量驗收」、`development-workflow.md` 的 acceptance 一節 |

資料流：容器內 wrapper → `<S>/peak/<ID>/{peak,rss.json}` → shim 結束時的 sidecar（`<S>/containers/<ID>.json`）→ 逐步檢查 → `acceptance-report`（v2）；host 端：`host-run` → `<S>/host/<step>.json` → 逐步檢查 → 報告；收養：harness（subreaper）→ 每一步之後與 `on_failure` 的 `reap-adopted`（殘留時 `<S>/leftover-pids.json`）。

##### 五、測試（id → 落點）

| id | 內容 | 落點 |
|---|---|---|
| ac20 | wrapper：fake `memory.stat` 在執行期間變化 → 取樣峰值 ＝ 其中的最大值、次數 > 0；leader 配置 N MiB → `children_max_rss_bytes` ≥ N MiB；**wrapper 自己膨脹**（在 wrapper 的程序裡先配置 N MiB 再執行）→ `self_max_rss_bytes`、`max_single_rss_bytes` ≥ N MiB；**未被 wait 的子孫**（leader 啟動一個配置 N MiB 的孫程序、⛔ 不 wait 就結束）→ 仍計入；**存活的孤兒**（孫程序活過寬限）→ `all_descendants_reaped=false` 與錯誤、結束碼照傳；孤兒在寬限內結束 → 收掉並計入；**反例：parent 把 `SIGCHLD` 設成 `SIG_IGN`、它的子程序配置 N MiB** → `children_max_rss_bytes` < N MiB（被自動回收、確實⛔ 不在精確閘裡）**而且** `auto_reap_detected=true`（⛔ 不會被當成通過）；結束碼原樣（0、3）與訊號（KILL → 137）；stdout／stderr 逐 byte 透明；`memory.stat` 讀不到 → 錯誤、結束碼照傳；只有 v2 的 `memory.stat` → 錯誤（⛔ 不改讀 `anon`）；⛔ 不是 PID 1 → `reaper=subreaper` | pytest（3.9 相容另在 host unittest 跑一支） |
| ac21 | `rss.json` 的封閉 schema：逐欄竄改各一支——缺欄、多欄、`bool` 當整數、整數當布林、0、`rss_interval_ms` ≠ 50、未知的 `rss_source`、`reaper=subreaper`、`errors` 非空、`max_single_rss_bytes` ≠ 兩者取大、`all_descendants_reaped=false`、`auto_reap_detected=true` → 量測失敗；acceptance 讀、sizing ⛔ 不讀 | pytest |
| ac22 | 報告 v2：精確閘的邊界（＝ 450 MiB → 超標、−1 → 通過）；偵測器的邊界（取樣 ＝ 450 MiB → 超標、−1 → 通過）；cgroup 峰值 ≥ 450 MiB、RSS 都低 → `ok`；schema 名稱；`violations` 是結構化的；`validate_acceptance_report_v2()` 的竄改——頂層與巢狀列（`memory[]`、`host[]`、`paths.*`、`promotion.*`、`limits`、`violations[]`）的缺欄、多欄、型別錯誤（含 `bool` 冒充整數）、三個固定欄位的未知值、`status` 不在 enum；**語意交叉**：`status = ok` 但 `violations` 非空、`threshold_exceeded` 但 `violations` 為空、把某一列的 `max_single_rss_bytes` 改成 ≥ 門檻而 `violations` 沒有對應的一筆、刪掉一筆 `violations`、`P_path` 改成 > `P_B_BUDGET` → 全部拒絕；**把 `limits.P_B_BUDGET` 調高、移除磁碟的那一筆違反、其餘欄位保持一致** → 拒絕（它必須等於正式常數）；**衍生欄位的竄改**：`self_max_rss_bytes` 改成 500 MiB 同時把 `max_single_rss_bytes` 壓低（容器與 host 各一支）、`accounted` ≠ `accounted_parts` 的總和、`P_path` ≠ 三者取大、`P_promotion` 或 `P_path_plus_promotion` 不符、`below_limit`／`group_alarm` 翻轉、`container_memory_limit_bytes` 與列不符 → 全部拒絕；產出端與 validator 呼叫的是同一個 `derive_acceptance_violations()`（以 monkeypatch 改它，兩邊一起變）；notes 只列上限**嚴格低於**門檻的容器（上限 ＝ 門檻的容器⛔ 不列） | pytest |
| ac23 | observer：`total_rss` 取樣峰值、讀不到 → `rss_unavailable_containers` 且⛔ 不完整 | pytest |
| ac24 | `host-run`：未被 wait 的孫程序配置 N MiB → 計入；`RUSAGE_SELF` 計入；`setsid` 的子孫計入取樣；**存活、而且會 `setsid()` 的子孫**活過寬限 → `all_descendants_reaped=false`、清理階段收掉它、`cleanup_complete=true`；**忽略 TERM 的子孫** → 升級 KILL；**`host-run` 收到 TERM／INT／HUP**（參數化三支；有 `setsid` 的子孫在跑）→ 清理後各以 143／130／129 結束、子孫已不在；**`host-run` 自己膨脹**（以一個先配置 N MiB、再在自己的程序裡呼叫 `host_run()` 的 driver 執行）→ `self_max_rss_bytes` 與 `group_rss_peak_sampled_bytes` 都 ≥ N MiB（群組取樣含 `host-run` 自己）；`SIGCHLD=SIG_IGN` 的子孫 → `auto_reap_detected=true`；清理收不乾淨（把送訊號換成 no-op）→ 結束碼 70 | host unittest |
| ac24b | 收養（測試以一個設成 subreaper 的 Python 程序扮演 harness）：**`host-run` 被 KILL**、留下 `setsid` 的子孫 → 被收養、`reap-adopted` 收掉它；**TERM 寬限期間才 fork**（子程序收到 TERM 時 fork 一個 `setsid` 的程序再結束、`host-run` 隨後被 KILL）→ 新程序同樣被收養並收掉；**PID 重用的守門**：列舉之後、送訊號之前 starttime 變了（monkeypatch）→ ⛔ 不送訊號；**parent 的身分**：`--parent-starttime` 不符 → 2、⛔ 不送任何訊號；**helper 執行期間 parent 消失**（扮演 harness 的程序在第一輪 TERM 之後被 KILL）→ helper 偵測到 `getppid()` 改變、立刻停止（剩下的目標⛔ 沒有再收到訊號、仍存活，由測試自己收掉）、2；`--check-only` 有收養的程序 → 1；清完仍有殘留（把送訊號換成 no-op）→ 1、`leftover-pids.json` 符合封閉格式；`/proc` 讀不到 → 2 | host unittest |
| ac28b | `kill-pinned`（每一支都斷言 stdout 那一行 JSON 的封閉鍵集合、`result`、`signals_sent` 與結束碼的對應）：一開始就不在 → 0；身分不符（PID 屬於另一個存活的無關程序、starttime 不同）→ 0 而且⛔ 不送訊號（該程序仍存活）；忽略 TERM 的程序 → 升級 KILL → 0；KILL 之後已成 zombie（扮演 parent 的程序故意不收）→ 視為已消失 → 0；KILL 之後仍存活（把送訊號換成 no-op）→ 1；第一次讀 `/proc` 就發生 EACCES 等錯誤（monkeypatch）→ 2 而且⛔ 不送訊號；**TERM 之後、KILL 之前**讀 `/proc` 失敗（monkeypatch 第二次讀取）→ 2、`result=error_after_term`、`signals_sent=["TERM"]`、⛔ 沒有升級 KILL；參數不合法 → 2、stdout 是空的；guard 與 `on_failure` 把那一行附加到 `<S>/kill-pinned.jsonl`（shell 測試斷言內容） | host unittest |
| ac24c | host 紀錄的封閉 schema：逐欄竄改（缺欄、多欄、`bool` 當整數、整數當布林、`step` ≠ 檔名、`rc` ≠ `rc.tsv`、`max_single_rss_bytes` ≠ 取大、`group_interval_ms` ≠ 100、`env_keys` 未排序、`cmd` 為空、各有效條件不符）→ 逐步檢查與報告 fail-closed | pytest |
| ac25 | 能力檢查：fake docker 產出只有 v2 的 `rss.json`、`reaper=subreaper`、或 schema 不符 → `on_failure`、⛔ 沒有 clone；**卡住的 docker client**（`probe-timeout`）→ 被 KILL、以 cidfile 清掉容器；**被 TERM**（卡住期間）→ harness 結束碼 **1**（既有契約）、`failure_summary.json` 記原始訊號的結束碼 143、容器已清；容器清不掉 → 保留 S、列出 CID | shell（隔離的最小 repo） |
| ac26 | 逐步檢查：第一步之後放一份量測失敗的 sidecar（或 host 紀錄的有效條件不符）→ 立刻 `on_failure`、之後的步驟⛔ 沒有執行 | shell（隔離的最小 repo）＋ pytest（`check-step` 本身） |
| ac28 | harness 的收養：經 bootstrap 啟動 → `measure_subreaper_guard` 通過、**測試程序已不在**；手動 re-exec（⛔ 不經 python3 啟動器、快照合法）→ 在 guard 中止、**測試程序已不在**（guard 的錯誤訊息印出它的 PID 與 starttime，測試據此確認）；guard 期間收到 TERM（故障注入 `subreaper-stall`：記下測試程序之後停住）→ `on_failure` 收掉它、harness 結束碼 **1**、`failure_summary.json` 記 143；**測試程序回報不了身分**（故障注入 `guard-noident`：測試程序⛔ 不寫 `guard-probe.json`）→ ⛔ 不送訊號、等它自己結束（5 秒內）、確認消失 → 中止；**JSON 的 PID ≠ `$!`**（故障注入 `guard-badident`：測試程序寫入自己的 PID ＋ 1）→ ⛔ 沒有呼叫 `kill-pinned`（`kill-pinned.jsonl` ⛔ 不存在）、只以 `$!` 等它自己結束、確認消失 → 中止；**正常情況下 `$!` ＝ JSON 的 `pid`**（正面的一支）；**process group leader 的情境**（故障注入 `guard-pgleader`：測試程序先 `os.setpgid(0, 0)` 自成 process group leader，再呼叫 `os.setsid()`——確定以 EPERM 失敗；⛔ 不靠 `set -m`，那要在實際執行 `python3 … &` 的內層子 shell 才生效、容易打不到）→ 測試程序立刻結束、⛔ 沒有任何後代、⛔ 不寫身分的 JSON → 走身分不可信的路徑 → 中止、⛔ 沒有殘留；另斷言故障注入**真的走到 EPERM**：`guard-probe-status.json` 存在且恰好是 `stage=setsid`、`errno=EPERM`、`pid` ＝ `$!`（guard 的訊息印出 `$!` 與狀態檔的內容），而且⛔ 沒有呼叫 `kill-pinned`（`kill-pinned.jsonl` ⛔ 不存在）；**狀態檔⛔ 不是身分來源**：故障注入 `guard-badident` 的情境另放一份偽造的狀態檔 → guard 的判斷與收尾⛔ 不變；sizing 的故障注入 `orphan-setsid`（步驟留下一個 `setsid` 的 `sleep`）→ 收養檢查觸發 `on_failure`、`sleep` 已不在；`reap-adopted` 結束碼 ≠ 0（以 fake 的 helper）→ `on_failure` 保留 S、印出殘留；兩個入口的 bootstrap 區塊照舊逐字相同 | shell（隔離的最小 repo）＋ host unittest（既有的一致性） |
| ac27 | 靜態（語意）：`python/`、`scripts/`（測試除外）⛔ 沒有把 `SIGCHLD`／`SIGCLD` 設成 `SIG_IGN`、⛔ 沒有 `SA_NOCLDWAIT`、⛔ 沒有 `trap '' … CHLD`。對照組（暫存檔，逐一植入 → 必須被抓到）：`signal.signal(signal.SIGCHLD, signal.SIG_IGN)`、`import signal as sig; sig.signal(sig.SIGCHLD, sig.SIG_IGN)`、`from signal import signal, SIGCHLD, SIG_IGN; signal(SIGCHLD, SIG_IGN)`、`from signal import signal as s, SIGCHLD as C, SIG_IGN as I; s(C, I)`、`signal.signal(17, signal.SIG_IGN)`、`signal.signal(signal.Signals.SIGCHLD, handler=signal.SIG_IGN)`、`SA_NOCLDWAIT`、`trap '' CHLD`、`trap -- "" SIGCHLD`；⛔ 不該抓的：只提到 `SIGCHLD`、`signal.signal(signal.SIGCHLD, signal.SIG_DFL)` | pytest |
| ac4（擴充） | acceptance 每個 role 的指令都是 `python -I /acceptance/rss_wrapper.py <原指令>`、掛載的是快照裡的檔；sizing 照舊是 `sh -c`（⛔ 不掛 wrapper） | shell |
| ac19（既有） | 快照清單與 `SNAPSHOT_FILES` 一致、`harness_manifest` 涵蓋新檔 | host unittest、pytest |

反向驗證（逐項注回、確認變紅、還原）：門檻改回 cgroup 峰值；拿掉精確的閘；拿掉偵測器；門檻的邏輯寫反；精確閘不含 `RUSAGE_SELF`；不收孤兒（leader 結束就讀）；存活的子孫不視為錯誤；拿掉 `SigIgn` 的偵測；取樣讀不到時寫 0；改讀 v2 的 `anon`；兩份 schema 放寬（多欄、`bool`、交叉值）；notes 用「不高於」；報告 schema 留在 v1；結束碼不傳回訊號；wrapper 寫 stdout；shim 改從工作複本掛載 wrapper；observer 讀不到 `total_rss` 仍標完整；`host-run` 不收孤兒、只看 process group、逾時不清理、被 TERM 不清理、清不乾淨卻回 0；bootstrap 不設 subreaper；拿掉 `measure_subreaper_guard`；拿掉每一步之後的收養檢查；`reap-adopted` 不驗 starttime；`reap-adopted` 不驗 parent 的身分（或只在開頭驗一次）；guard 的測試程序在失敗路徑⛔ 沒有收掉；拿不到身分或兩個 PID 不相等時仍送訊號；guard 改回外部的 `setsid` 指令；`kill-pinned` 的 `result` 與結束碼不對應；validator 以報告自己的 `P_B_BUDGET` 當基準；`kill-pinned` 在 TERM 之後讀取失敗仍升級 KILL；`kill-pinned` 不驗 starttime、把 zombie 當成存活；validator 不重新推導門檻結果、不驗衍生欄位、產出端另寫一份門檻邏輯；`reap-adopted` 失敗（結束碼 2）仍刪 S；報告不經 `validate_acceptance_report_v2()`；靜態掃描不解析 alias；拿掉能力檢查；能力檢查逾時不清容器；拿掉逐步檢查；靜態掃描漏掉 `SIG_IGN`。

##### 六、驗證

1. `python/scripts/test.sh` 完整執行（依序）全綠。
2. 反向驗證（上表）。
3. acceptance 開發驗證的實跑（stub）一次：subreaper 的 guard 通過；每一步之後的收養檢查都是 0；能力檢查通過（`rss_capability`）；每個容器的最大單一程序、wrapper 自己、RSS 取樣峰值與次數、cgroup（資訊值）、上限、`reaper=pid1` 都有數字；每一步的 host 紀錄有效（`all_descendants_reaped=true`、`cleanup_complete=true`）；報告是 v2；與 dev5 的 cgroup 峰值對照。
4. sizing 的 validation 實跑一次（bootstrap、`run_in_group`、`on_failure` 是共用原語；量法與數字預期不變）。
5. stage 之後 `--verify "$(git write-tree)"`（tooling 路徑⛔ 不動）、`git diff --cached --check`、`check-doc-refs.py`，以及「⛔ 不…」用語的掃描。

##### 七、風險與回滾

| 風險 | 對策 |
|---|---|
| 取樣的 `total_rss` 漏掉尖峰 | 它只是偵測器；通過由精確的閘證明 |
| 契約外的自動回收 | 契約已縮小（使用者的裁決）；`SIG_IGN` 執行期偵測、本 repo 靜態禁止；報告的 `out_of_contract` 照實列出 |
| wrapper 改變被量的程序（多一層 Python、約 10 MiB、50 ms 一次的讀檔與 `/proc` 掃描、PID 1 的收養職責） | 方向偏保守；結束碼與 I/O 由測試釘住 |
| 清理殺到不該殺的程序 | `host-run` 只對直接子程序送訊號（PID ⛔ 不會被重用）；`reap-adopted` 先釘住 harness 自己（starttime ＋ 每一步的 `getppid()`），只動 ppid ＝ harness 的程序、以 starttime 釘住，⛔ 不以名稱或指令比對（⛔ 不用 `pkill -f`）；確認與送訊號之間仍有極短的競態（⛔ 沒有 pidfd），照實標示；guard 的測試程序以 PID ＋ starttime 釘住、最多活 5 秒 |
| harness 自己被 KILL、或 host 當機 | 照實標示的前提（「驗收語意」）：沒有任何程式能收尾；`on_failure` 本身也是 harness 的一部分 |
| 改到已 commit 的 bootstrap | 兩個入口逐字相同的測試照跑；subreaper 以行為驗證；bootstrap 的既有測試（快照、re-exec、⛔ 不刪）全部重跑 |
| 換到 cgroup v2 的 host | 能力檢查在 clone 之前就中止，需另行裁決 |
| 判定量改了，⑨-1 的意義跟著改 | 「二」⑨ 逐一加註；報告升 v2；sizing 的量法不變 |
| 回滾 | 增補的實作是一個獨立的 commit（`f20ce3c` 之上）：`git revert` 它即回到 ⑦d 已 commit 的狀態；計畫書 commit 只動文件 |

##### 八、歸檔（實作後；review 前保留本計畫）

`sr-zone-scoring.md`「I-074 Stage 2 的容量驗收」改寫「對象與門檻」與「容器的記憶體上限」兩點並加契約與範圍；`development-workflow.md` acceptance 一節的「門檻」「容器的記憶體上限」兩列、能力檢查與 `host-run` 的清理；本筆的 ⑦d 實作結果另加「增補的實作結果」。

##### 增補計畫第一輪 review 的修正（2026-10-05）

| 嚴重度 | 發現 | 查證 | 修正（v2） |
|---|---|---|---|
| 高 | 兩道 RSS 閘仍無法證明「每個程序／整個容器都低於 450 MiB」：`RUSAGE_CHILDREN` 只涵蓋已結束且已 wait 的子孫、是最大的那一個而⛔ 不是整棵樹，wrapper 自己⛔ 不在內；計畫又允許結束時仍存活的孤兒只靠 50 ms 的下界涵蓋 → 可能假通過 | ✅ 成立（`getrusage(2)`）。另查到 host 端 `host-run` 有同一個缺口 | 語意釘死為 v29 的「每一個程序各自」：通過的證明 ＝ `max(RUSAGE_SELF, RUSAGE_CHILDREN)`，前提是 wrapper 是 PID 1（或 subreaper）並收到 `ECHILD`，收斂不了 → fail-closed；取樣的 `total_rss` 改成單向偵測器；⛔ 不新增容器總和的契約；`host-run` 一併補上（「三」#9）；測試補 wrapper 自身膨脹、存活的孤兒、未被 wait 的子孫、收斂不到 `ECHILD` |
| 中 | cgroup v2 的 `anon` 只有 fake 測試，卻能產生正式的通過結果 | ✅ 成立（v1 `total_rss` 是匿名頁 ＋ swap cache，v2 `anon` 是匿名映射，⛔ 不同義） | 只接受 `v1:total_rss`（封閉 enum），wrapper ⛔ 不改讀 v2；移到 v2 host 時以實機驗證另行裁決（「三」#5） |
| 低 | `rss.json` 的「格式不符」沒有定義成封閉 schema | ✅ 成立 | 「二」② 的封閉 schema（恰好十一個鍵、整數排除 `bool`、峰值與次數 > 0、`rss_interval_ms` ＝ 50、`rss_source` 封閉 enum、`errors` 為空、`max_single_rss_bytes` ＝ 兩者取大、`all_descendants_reaped=true`），ac21 逐欄竄改 |
| 低 | 上限等於門檻時，⛔ 不能宣稱兩道閘必然通過（判定是嚴格 <） | ✅ 成立；另查到更廣的一層：`ru_maxrss` 含 file-backed 頁，可能記在別的 cgroup，**即使上限嚴格低於門檻，精確的閘也⛔ 不受上限保證** | notes 改成只列上限**嚴格低於**門檻的容器，而且只宣稱 `total_rss` 偵測器不可能觸發；精確的閘仍逐一比較 |

##### 增補計畫第二輪 review 的修正（2026-10-05）

| 嚴重度 | 發現 | 查證 | 修正（v3） |
|---|---|---|---|
| 高 | 收到 `ECHILD` ⛔ 不等於所有子孫的 RSS 都已納入：中間的 parent 把 `SIGCHLD` 設成 `SIG_IGN` 或用 `SA_NOCLDWAIT` 時，子程序被 kernel 自動回收、resource usage 被丟棄，最上層照樣收到 `ECHILD` | ✅ 成立（`getrusage(2)`） | 採 review 的第一個選項：精確閘的範圍**明訂**為「wrapper、leader 與經正常 wait 鏈保存 resource usage 的子孫」；`SIGCHLD=SIG_IGN` 在容器與 host 都以取樣偵測（`SigIgn`）、偵測到即 fail-closed（`auto_reap_detected` 進兩份 schema）；本 repo 的程式以靜態測試禁止（ac27，現況 0 處）；`SA_NOCLDWAIT` 列為殘餘（「二」⑧）；ac20 加反例：`SIG_IGN` 的 parent 底下的配置確實⛔ 不在精確閘裡，而且被偵測、⛔ 不會被當成完整的證明 |
| 中 | `host-run` 逾時之後可能留下存活的 host 程序：容器的 wrapper ⛔ 不殺是安全的（docker 會清），host 上沒有這個保證；`setsid` 的子孫會逃出原本的 process group，外層的 TERM／KILL 只看那個 group | ✅ 成立（`scripts/lib/i074-stage2-measure.sh` 的 `stop_step_group` 只對原 group） | 「二」④：先標 `all_descendants_reaped=false`（量測必然失敗）→ 獨立的清理階段（ppid 鏈追到的全部子孫：TERM → 5 秒 → KILL → 收）→ 記 `cleanup_complete`、`leftover_pids`；清不乾淨 → 結束碼 70、`<S>/host-leftover-pids`，`on_failure` 保留 S 並列出 PID；群組取樣也改成 ppid 鏈；ac24 加會 `setsid()` 的存活子孫與忽略 TERM 的子孫 |
| 中 | v1-only 的不相容太晚才被發現：wrapper 讀不到 v1 只記錯誤、結束碼照傳，要到最後的報告才中止——v2 host 上可能白跑完整套（含約 3 小時的量測趟） | ✅ 成立 | 「二」⑦：建 work 目錄之前以同一個 image、同一種掛載跑一次能力檢查（⛔ 不經 shim、⛔ 不進索引）；每一步之後立刻 `check-step`（這一步的 host 紀錄 ＋ 目前為止的 sidecar），第一個不符就 `on_failure`；ac25、ac26 |
| 低 | `host-run` 的紀錄缺少封閉 schema | ✅ 成立 | 「二」④ 的封閉 schema（完整鍵集合、整數排除 `bool`、`max_single_rss_bytes` ＝ 兩者取大、`reaper=subreaper` 與各有效條件）；ac24 的逐欄竄改 |

##### 增補計畫第三輪 review 的修正（2026-10-05）

| 嚴重度 | 發現 | 查證 | 修正（v4） |
|---|---|---|---|
| 高 | 正式的 `ok` 仍無法證明「每一個程序」都 < 450 MiB：精確閘只涵蓋正常 wait 鏈，`SIG_IGN` 是取樣偵測、`SA_NOCLDWAIT` 看不到——只在 notes 標殘餘⛔ 不能把可能的假通過變成有效驗收；必須在「縮小契約」與「證明不了就⛔ 不給 ok」二選一 | ✅ 成立 | **使用者裁決：縮小契約**——「經正常 wait 鏈保存 resource usage 的每一個程序」；被自動回收的子孫在契約外；v29「六、1」、報告 `status`、歸檔文件同步修改（「二」⑨）；報告頂層記 `contract` 與 `out_of_contract` |
| 中 | `host-run` 的清理只處理正常逾時，被 TERM、KILL 或異常結束時來不及清；外層只清原 process group；以裸 PID 送訊號有 PID 重用的競態 | ✅ 成立；kernel 4.19 ⛔ 沒有 pidfd（`pidfd_open` 回 ENOSYS，實測） | 程序內：逾時與 TERM／INT／HUP 都進清理階段，**只對直接子程序**送訊號（收之前 PID ⛔ 不會被重用）、一輪一輪收；外層：`host-run` 每次取樣寫 `<S>/host/<step>.live`（`pid, starttime`），`on_failure` 以 `host-reap` 先比 starttime 再送訊號；收不掉 → 保留 S、列出 PID；ac24（TERM）、ac24b（KILL、PID 重用的守門） |
| 中 | 能力檢查的容器生命週期沒有納管（cidfile、唯一名稱、逾時、中斷清理） | ✅ 成立（既有清理只認 `<S>/cid/*.cid` 與 `i074sz-<run id>-*`） | 能力檢查改在建 work 目錄之後、clone 之前，六步清理的 trap 已裝好；`--cidfile <S>/cid/probe.cid`、`--name i074sz-<run id>-probe`（都在既有清理的範圍內）、`setsid timeout -s KILL 120` 背景執行並記成 `STEP_PID`；故障注入 `probe-timeout`；ac25 補卡住、被 TERM、清不掉 |
| 中 | commit 與回滾的邊界與事實不符：文件寫「併進尚未 commit 的 ⑦d」，但 HEAD 已是 `f20ce3c`（⑦d 實作） | ✅ 成立 | **使用者裁決：另立增補 commit**（計畫書 → 實作）、⛔ 不 amend；回滾單位 ＝ 增補的實作 commit（`git revert`）；狀態列與「現在在這裡」同步 |
| 低 | host 紀錄還⛔ 不是真正的封閉 schema（`step`、`rc`、三個布林、`env_keys`、`cmd` 的型別與交叉條件；`step` ＝ 檔名、`rc` ＝ 實際結果） | ✅ 成立 | 「二」④ 逐欄定義（十七個鍵、型別、交叉條件、有效條件）；ac24c 逐欄竄改 |
| 低 | 門檻的文字邏輯寫反（寫成「`max_single` < 450 或 `sampled` ≥ 450 → 超標」） | ✅ 成立 | 改成「`max_single_rss_bytes` ≥ 450 MiB 或 `rss_peak_sampled_bytes` ≥ 450 MiB → `threshold_exceeded`」；反向驗證加「邏輯寫反」 |
| 低 | ac27 的字面掃描與偵測器本身衝突（偵測器要用 `SIGCHLD` 算 `SigIgn` 的位元） | ✅ 成立 | 改成語意檢查（AST 找「設成 `SIG_IGN`」與 `SA_NOCLDWAIT`、shell 找 `trap ''`）；對照組：只提到 `SIGCHLD` → 通過 |
| 低 | 改了報告結構與判定語意卻沿用 v1 schema；已經有一份舊語意的開發報告 | ✅ 成立（dev4、dev5 兩份） | 升成 `i074_stage2_acceptance_report_v2`；v1 明定為舊語意的開發紀錄；現在沒有任何程式讀它，日後的讀取端只接受 v2 |

##### 增補計畫第四輪 review 的修正（2026-10-05）

| 嚴重度 | 發現 | 查證 | 修正（v5） |
|---|---|---|---|
| 高 | 宣稱「被 KILL 也⛔ 不留下子孫」與設計本身矛盾：`.live` 快照之後才產生、或在 TERM 的 5 秒寬限期間才 fork／`setsid()` 的程序不在快照裡，parent 被 KILL 後改由 init 收養，`host-reap` 找不到 | ✅ 成立 | 採 review 的第二個選項（結構性的收養）：**harness 自己是 subreaper**——bootstrap 以 python3 啟動器設 `PR_SET_CHILD_SUBREAPER` 再 exec bash（跨 `execve` 保留），以行為驗證；`host-run` 以任何方式結束留下的程序都被 harness 收養、隨時可列舉；`.live` 與 `host-reap` 移除，改成 `reap-adopted`（每一步之後檢查、`on_failure` 清理、正常結束前再檢查）。前提照實寫明：harness 自己沒有被 KILL；殘餘：極短的 PID 重用競態（無 pidfd）。ac24b 補「`host-run` 被 KILL」「TERM 寬限期間才 fork」兩個反例；`host-run` 的清理上限 15 秒 < `stop_step_group` 的 20 秒 |
| 中 | `.live`／`host-reap` 是會送訊號的控制介面，卻沒有封閉契約（格式、atomic write 失敗、正常結束後的處置、malformed、helper 自身失敗時 `on_failure` 怎麼 fail-closed、`host-leftover-pids` 的格式） | ✅ 成立 | `.live` 與 `host-leftover-pids` 隨 v5 的設計移除；新的 `reap-adopted` 定義封閉契約：參數、結束碼 0／1／2、`leftover-pids.json` 的封閉格式（只給操作者看、⛔ 沒有程式讀回）、`on_failure` 對任何 ≠ 0 的結束碼都保留 S |
| 中 | 既有的 ac6（cgroup 峰值達門檻即失敗）與 ac16（取樣只算同一個 process group）沒有列進修改清單，會與 v4 直接衝突 | ✅ 成立（`test_ac6_container_and_host_boundaries`、`test_host_run_*`） | 「二」⑨ 明列 ac6、ac16 的改寫與對應測試的同步修改，⛔ 不留過期的斷言 |
| 低 | 報告 v2 的新欄位沒有明確 schema（`out_of_contract` 的型別與值集合） | ✅ 成立 | 三個封閉的固定值（`contract` 字串、`out_of_contract` 陣列、`auto_reap_detection` 物件）；寫出前以 `validate_acceptance_report_v2()` 自我驗證；ac22 補缺欄、多欄、型別錯誤、未知值 |
| 低 | ac27 的 AST 規則會漏掉 alias（`import signal as sig`、`from signal import …`） | ✅ 成立 | 靜態檢查先解析 `signal` 名稱的來源（含 `as`、from-import、`Signals.SIGCHLD`、整數 17、`handler=`）；ac27 補對照組；動態寫法照實列為⛔ 不在靜態檢查內 |

##### 增補計畫第五輪 review 的修正（2026-10-05）

| 嚴重度 | 發現 | 查證 | 修正（v6） |
|---|---|---|---|
| 中高 | `reap-adopted` 沒有釘住 harness 自己的身分：`--parent` 是裸 PID；harness 在 helper 執行的 30 秒內被 KILL、PID 又被重用時，可能列舉並殺掉新程序的子程序——⛔ 不只是「無法收尾」，而是可能誤殺 | ✅ 成立 | 參數加 `--parent-starttime`，啟動時驗；每一輪列舉與每一個訊號之前驗 `os.getppid()` ＝ parent（helper 是 harness 的直接子程序，`getppid()` 還等於 parent 的期間，那個 PID ⛔ 不可能已被重用），不符立刻停止、結束碼 2；ac24b 補「starttime 不符」「執行期間 parent 消失」 |
| 中 | `measure_subreaper_guard` 的失敗路徑可能留下測試用的 `setsid sleep`：手動 re-exec 的反例正是「⛔ 不是 subreaper」，那時它已被 init 收養，`reap-adopted` 找不到 | ✅ 成立 | 測試程序改成 `sleep 5`（本身有時限），記下 PID 與 starttime；通過、不符、讀不到、guard 期間的訊號——所有出口都以 `kill-pinned`（比對 starttime）收掉；ac28 斷言兩條路徑之後測試程序都已不在，另加 guard 期間收到 TERM（故障注入 `subreaper-stall`） |
| 中 | report v2 的 validator 還⛔ 不足以當日後讀取端的共同守門：`status = ok` 搭配非空的 `violations`、或記憶體列已達門檻的竄改報告仍可能被接受 | ✅ 成立 | `violations` 結構化（封閉的 `kind`）；validator 驗頂層與重要巢狀列的封閉鍵集合與型別，**由列重新推導**門檻結果、要求與 `violations` 逐筆相等，並驗 `status` ⟺ `violations` 是否為空；照實寫明它驗報告本身的一致性、⛔ 不驗與原始量測相符；ac22 補語意交叉的竄改 |
| 低 | `host-run` 承諾 TERM／INT／HUP 都清理，ac24 只測 TERM | ✅ 成立 | ac24 參數化三種訊號，結束碼 143／130／129 |
| 低 | 章節內仍標「驗收語意（v4）」 | ✅ 成立 | 改成 v6 |

##### 增補計畫第六輪 review 的修正（2026-10-05）

| 嚴重度 | 發現 | 查證 | 修正（v7） |
|---|---|---|---|
| 中 | guard 在「讀不到 `/proc`」時無法安全執行 `kill-pinned`（手上沒有可信的 starttime）；`kill-pinned` 也沒有封閉契約 | ✅ 成立；另查到一層：被 KILL、還沒被收的程序 `/proc/<pid>` 仍在、starttime 相同（state `Z`），⛔ 不能當成「仍存活」 | 測試程序**自己回報** PID 與 starttime（`guard-probe.json`）；拿不到身分 → ⛔ 不送訊號、等最多 6 秒確認消失，確認不了 → 保留 S；`kill-pinned` 的封閉契約（參數、0／1／2、已消失、身分不符、`Z`／`X` 視為已死、忽略 TERM、KILL 後仍存活、`/proc` 失敗），`reap-adopted` 同樣把 `Z`／`X` 視為已死；ac28b 逐一測，ac28 補故障注入 `guard-noident` |
| 中 | report validator 沒有明訂衍生欄位的內部一致性：把 `self_max_rss_bytes` 改成 500 MiB、同時壓低 `max_single_rss_bytes`，仍可能「`violations` 與 `status` 相符」卻內部矛盾 | ✅ 成立 | validator 驗衍生欄位（`max_single` ＝ 兩者取大〔容器與 host〕、`accounted` ＝ 組成總和、`P_path` ＝ 三者取大、晉升兩欄、`below_limit`、`group_alarm`、`limits` 的兩個值）；產出端與 validator 共用唯一的 `derive_acceptance_violations()`；ac22 補衍生欄位的竄改 |
| 中 | ac28 寫 TERM 之後回 143，與既有契約衝突（`on_failure` 固定 `exit 1`、操作文件定義 1 ＝ harness 失敗） | ✅ 成立；ac25 也有同一個寫法 | 維持外部結束碼 1（「三」#20），ac25、ac28 改成斷言 `failure_summary.json` 記原始訊號的 143 |
| 低 | host 紀錄宣稱群組取樣含 `host-run` 自己，ac24 卻沒有測 | ✅ 成立 | ac24 補「`host-run` 自己膨脹」：driver 先配置 N MiB 再在自己的程序裡呼叫 `host_run()`，`self_max_rss_bytes` 與群組取樣都必須反映 |

##### 增補計畫第七輪 review 的修正（2026-10-05）

| 嚴重度 | 發現 | 查證 | 修正（v8） |
|---|---|---|---|
| 中 | validator 沒有釘死固定的 `P_B_BUDGET`：同時調高報告裡的預算並移除磁碟的違反，整份報告仍可能自洽 | ✅ 成立 | `limits.P_B_BUDGET` ＝ 正式常數（validator 的參數，呼叫端傳 `i074_stage2_preflight.P_B_BUDGET`）；ac22 補「預算調高、其餘一致」→ 拒絕 |
| 中 | guard 的兩個 PID 來源（測試程序寫的 JSON、子 shell 的 `$!`）沒有相等契約，`$!` 怎麼傳回 harness 也沒定義；JSON 合法但 PID 錯時可能對錯的目標 `kill-pinned` | ✅ 成立 | `$!` 以 command substitution 回報（恰好一行整數；測試程序的 stdio 導走）；身分可信 ⟺ JSON 出現且格式正確、`pid` ＝ `$!`、starttime 相符；否則⛔ 不呼叫 `kill-pinned`、只以 `$!` 限時等待、中止；ac28 補故障注入 `guard-badident` |
| 低 | `kill-pinned` 的「rc=2 ⛔ 不送訊號」說得太強：`/proc` 錯誤可能發生在 TERM 已送出之後 | ✅ 成立 | 改成「第一個訊號之前失敗 → ⛔ 不送任何訊號；TERM 之後才失敗 → ⛔ 不再送後續的訊號、回 2」；ac28b 補「TERM 之後、KILL 之前 `/proc` 失敗」 |

##### 增補計畫第八輪 review 的修正（2026-10-05）

| 嚴重度 | 發現 | 查證 | 修正（v9） |
|---|---|---|---|
| 中 | guard 的 `$!` ⛔ 不保證是實際的 Python 測試程序：外部的 `setsid python3 … &` 在呼叫者是 process group leader 時，util-linux 的 `setsid` 會自己 fork，`$!` 是短命的 parent、JSON 是 Python 子程序——只等 `$!` 消失就可能在測試程序還活著時判定收尾完成；`guard-badident` 只是人工寫入 PID ＋ 1，沒有涵蓋真實的 fork | ✅ 成立（util-linux `setsid(1)`：呼叫者已是 process group leader 時 fork） | 子 shell 直接背景啟動 Python、由它自己 `os.setsid()`（失敗就立刻結束、⛔ 不留後代、⛔ 不寫 JSON）——`$!` 在結構上就是測試程序；ac28 補「正常情況下 `$!` ＝ JSON 的 `pid`」與 process group leader 的情境（`set -m`） |
| 低 | `kill-pinned` 晚期失敗的紀錄沒有輸出契約（「結果記成…」沒有說記在哪裡） | ✅ 成立 | stdout 恰好一行 canonical JSON（封閉鍵集合、`result` 封閉 enum 並與結束碼一一對應、`signals_sent`）；參數不合法 → stdout 空；呼叫端附加到 `<S>/kill-pinned.jsonl`、隨 S 複製、失敗摘要列出路徑；ac28b 每一支都斷言這一行 |

##### 增補計畫第九輪 review 的修正（2026-10-05）

| 嚴重度 | 發現 | 查證 | 修正（v10） |
|---|---|---|---|
| 低 | ac28 的 `set -m` 測試位置不夠明確：只在外層 shell 開 job control 時，command substitution 內會關掉它，背景的 Python ⛔ 不會成為 process group leader、`os.setsid()` 照樣成功，打不到 EPERM 的分支 | ✅ 成立（`set -m` 必須在實際執行 `python3 … &` 的內層子 shell 啟用才有作用） | 改用確定的故障注入 `guard-pgleader`：測試程序先 `os.setpgid(0, 0)` 再 `os.setsid()`，必然 EPERM；另斷言真的走到這個分支（專用的結束碼）、⛔ 沒有 JSON、⛔ 沒有殘留 |

##### 增補計畫第十輪 review 的修正（2026-10-05）

| 嚴重度 | 發現 | 查證 | 修正（v11） |
|---|---|---|---|
| 低 | 「專用結束碼」沒有可觀察的傳遞通道：測試程序的 stdio 導走、`os.setsid()` 失敗時⛔ 不寫 JSON，資料流只有 `$!` 與 `/proc` 的存活輪詢；測試程序被收養之後它的結束碼也⛔ 不屬於 harness 的工作 | ✅ 成立 | 改成**只供診斷與測試**的失敗狀態檔 `<S>/guard-probe-status.json`（atomic、封閉：`schema`、`pid`、`stage`（`setsid`／`self_stat`）、`errno` 的名稱）；⛔ 不是身分來源（③ ⛔ 不讀它、⛔ 不依它送訊號）；ac28 以它斷言真的走到 EPERM，另以偽造的狀態檔證明它⛔ 不會影響 guard 的判斷 |

#### Stage 2 步驟 ⑦d 增補的實作結果（2026-10-05，✅ **review 通過**（三輪，2026-10-06）並 commit `576a8f7`）

✅ 依「Stage 2 步驟 ⑦d 增補計畫：容器記憶體改以 RSS 判定」（v11，review 十輪後確認並 commit `d1267bb`）完成「二」的設計。⛔ **沒有跑 `--formal`、
沒有跑完整計算、沒有 commit**；程式與文件已 stage，停在 review。⚠️ 增補⛔ 沒有動 tooling 路徑（`evaluation.py`、`replay_bundle/`）。

**做了什麼**（對照計畫「二」）：

| 「二」 | 實作 |
|---|---|
| ① 容器內的 wrapper | 新檔 `python/scripts/i074_stage2_rss_wrapper.py`：PID 1（⛔ 不是時 `prctl(PR_SET_CHILD_SUBREAPER)`）、leader 結束後收到 `ECHILD`（寬限 10 秒）、`max(RUSAGE_SELF, RUSAGE_CHILDREN)`；背景 thread 每 50 ms 讀 v1 `total_rss` 並掃子孫的 `SigIgn`；結束碼原樣（訊號 → 128 ＋ N）；I/O 透明；`/peak/peak`（資訊值）與 `/peak/rss.json` |
| ② `rss.json` 的封閉 schema | helper 的 `rss_record_problems()`（十二個鍵、整數排除 `bool`、交叉條件、有效條件、容器內 `reaper=pid1`）；常數與 wrapper 相同（測試釘住） |
| ③ sidecar 與報告 | sidecar 依 index 的 profile 讀 `rss.json`；報告 v2：記憶體列、四個封閉的固定欄位、結構化的 `violations`、唯一的 `derive_acceptance_violations()`、寫出前的 `validate_acceptance_report_v2()`（封閉 schema、衍生欄位、由列重新推導、`P_B_BUDGET` ＝ 正式常數、`status`）、notes 與文字版 |
| ④ host 端 | `host_run()` 改寫（subreaper、收到 `ECHILD`、`RUSAGE_SELF`、ppid 鏈的取樣與 `SigIgn`、逾時與 TERM／INT／HUP 的清理只對直接子程序、清不乾淨 → 70、十七個鍵的紀錄）、`host_record_problems()`、`reap-adopted`（parent 的身分：starttime ＋ 每一輪與每一個訊號之前的 `getppid()`；目標以 starttime 釘住；Z／X 視為已死；0／1／2 與 `leftover-pids.json`）、`kill-pinned`（封閉的輸出契約）；bootstrap 的 python3 啟動器（兩個入口逐字相同）、共用原語的 `measure_subreaper_guard`（測試程序自己 `os.setsid()`、自己回報身分、與 `$!` 相等才信；失敗出口的狀態檔只供診斷）、`measure_check_adopted`（`run_in_group` 之後、正常結束之前）、`on_failure` 的 `kill-pinned`／`reap-adopted` 與「≠ 0 → 保留 S」、失敗摘要列出 `kill-pinned.jsonl` 與 `leftover-pids.json` |
| ⑤ observer | 另讀 v1 `memory.stat` 的 `total_rss`（`rss_peak_bytes`、`rss_reads`），讀不到 → `rss_unavailable_containers`、⛔ 不完整 |
| ⑥ 快照清單 | acceptance 的 `I074_BOOT_FILES` 與 `SNAPSHOT_FILES["acceptance"]` 加 wrapper；shim 對 acceptance 的每一個 role 掛載快照裡的 wrapper、指令改用它（sizing ⛔ 不變） |
| ⑦ 提早失敗 | acceptance：建 work 目錄之後 guard、能力檢查（`probe_capability`：真正的 docker、cidfile 與名稱在既有清理的範圍內、`STEP_PID`、120 秒）才 clone；`step()` 之後 `helper check-step`。故障注入另加 `probe-timeout`、`subreaper-stall`、`guard-noident`、`guard-badident`、`guard-pgleader`；sizing 另加 `orphan-setsid` |
| ⑧ 靜態檢查 | helper 的 `auto_reap_violations()`（AST，解析 alias 與 from-import）與 `scan_auto_reaping()`；現況 0 處 |
| ⑨ 已確認章節的加註 | v29「六、1」、⑦d 細部計畫「二之三」「二之四」「二之五」（四列）「二之六」「五」（ac6、ac16），共 10 處，都標「⑦d 增補（✅ 2026-10-05 確認）」 |

**實作途中發現**：

| # | 發現 | 處置 |
|---|---|---|
| 1 | ⚠️ **fork 之後、exec 之前，子程序與 parent 共用常駐頁，它的 `ru_maxrss` 含 parent 當時的 RSS**（實測：wrapper 先配置 96 MiB，leader 只跑 `pass`，leader 的 high-water 仍是約 112 MB） | ⛔ 不影響精確閘：繼承的部分⛔ 不會超過 parent 自己的 high-water（parent 已計入），取大的結果不變、偏保守。報告的 notes ② 照實寫明；ac20 改成斷言「children ≤ self ＋ 8 MiB」（原本寫的「children < 96 MiB」是錯的預期） |
| 2 | pytest 的測試容器只掛了 `python/`——ac27 掃 `scripts/` 的那一半在容器裡會被**跳過**（第一版是 `pytest.skip`，等於沒掃） | 掃描移進 helper（`scan_auto_reaping()`）：pytest 掃 `python/`（並斷言⛔ 不是空掃描），host unittest 以同一個函式掃 `scripts/` 與 `python/` |
| 3 | 「任何含 `SA_NOCLDWAIT` 的字串」太寬：wrapper 的 docstring 與報告的 notes 會提到它（契約外的殘餘），字串本身也設不了旗標 | 只禁止名為 `SA_NOCLDWAIT` 的識別字；對照組加「說明文字提到它 → ⛔ 不算」（差異 #1） |

**與計畫的差異**：

| # | 差異 | 理由 |
|---|---|---|
| 1 | 靜態檢查⛔ 不把含 `SA_NOCLDWAIT` 的字串算違規（計畫寫「識別字或字串」） | 上表 #3 |
| 2 | ac27 的 `scripts/` 那一半在 host unittest（計畫寫 pytest） | 上表 #2；同一個函式 |
| 3 | 記憶體列的量法標記改成頂層一個封閉的固定物件 `memory_measures`（計畫寫在每一列標 `exact_within_wait_chain` 等） | 只寫一次、validator 驗固定值；每一列的鍵集合因此更小 |
| 4 | ac24b 的「helper 執行期間 parent 消失」以 fork 出的子程序裡 monkeypatch `os.getppid()` 實現（計畫寫「扮演 harness 的程序在第一輪 TERM 之後被 KILL」） | 決定性：對 helper 而言兩者都是「`getppid()` 改變」；真的殺掉 parent 時結束碼收不到 |
| 5 | ac28 的「偽造的狀態檔⛔ 不影響判斷」在 helper 那一層測（`guard_identity()` 在偽造的狀態檔存在時結果不變；計畫寫 shell） | 判斷全在 helper；shell 只把狀態檔原樣印出 |
| 6 | ac26 的 shell 那一半：自 `acceptance.sh` 抽出 `step()` 實際執行（stub 的 `run_in_group`／`helper`／`on_failure`） | 隔離的最小 repo 走不到步驟（要有 Stage 1 的錨點）；抽出的是檔案裡的實際函式 |

**檔案**：`python/scripts/i074_stage2_rss_wrapper.py`（新增）、`python/scripts/i074_stage2_sizing.py`、`scripts/lib/i074-sizing-docker-shim.sh`、`scripts/lib/i074-stage2-measure.sh`、`scripts/i074-stage2-acceptance.sh`、`scripts/i074-stage2-sizing.sh`；測試：`test_i074_stage2_acceptance.py`（ac20～ac28 的 pytest 部分、既有 ac6 與 fixture 的改寫）、`scripts/tests/test_i074_stage2_host.py`（`RssAddendumHost`：ac20 的 3.9、ac24、ac24b、ac27 的 `scripts/`、ac28b）、`scripts/test-replay-args.sh`（ac4、ac25、ac26、ac28、sizing 的 `orphan-setsid`、fake docker 的能力檢查）；文件：本筆、`sr-zone-scoring.md`、`development-workflow.md`。

**開發驗證的實跑**（validation、stub）：

| 項目 | 結果 |
|---|---|
| acceptance | ✅ 2026-10-05 07:49～07:56Z，`~/i074_stage2_acceptance/dev6-rss-20261005T074904Z/`；結束碼 0、`status: ok`、報告 v2；guard 通過（`kill-pinned.jsonl`：測試程序 `terminated_by_term`）、`rss_capability = v1:total_rss`、`repo_head` ＝ `clone_head` ＝ `d1267bb`；success `P_path` 161,267,712 bytes（153.8 MiB）、failure 72,630,272 bytes；十二個容器的**精確閘** 245.5～363.5 MiB（replay：success 336.7、failure 363.5）、**取樣的 `total_rss`** 228.4～347.6 MiB（每個容器 439～838 次）、cgroup（資訊值）285.2～417.0 MiB——⚠️ success 的 finalizer 的 cgroup 峰值 417.0 MiB 剛好等於它當次的上限（page cache 逼近上限才回收，正是改 RSS 的原因），它的精確閘只有 297.7 MiB；十二步的 host 紀錄全部有效（`all_descendants_reaped`、`cleanup_complete`），最大單一 49.2～51.7 MiB、群組取樣 73.7～106.4 MiB；`MemAvailable` 低點 206.6 MiB |
| sizing（兩次） | ✅ 第一次 2026-10-05 08:59～09:05Z（`~/i074_stage2_sizing/validation-7d-rss-20261005T085906Z/`）：`status: ok`、guard 通過、`repo_head` ＝ `clone_head` ＝ `d1267bb`，但 **`P_B` ＝ 167,575,552 bytes（距 `P_B_BUDGET` 只剩 196,608 bytes）**——拆開來看，`dirs_peak` 與 `accounted` 只比第一輪 review 的實跑多約 0.3 MiB（`code_worktree`：HEAD 多了已 commit 的 ⑦d 程式），整個跳升來自 **`fs_peak`**（L0 的 `statvfs` 已用量，catch-all）：witness 與 success 兩個窗口比 `dirs_peak` 多 3.93／6.89 MiB（success 窗口裡超過 3 MiB 的樣本持續 68.7 秒），最後的 failure 窗口回到 0.46 MiB。同一份程式的 acceptance（dev6，使用者清磁碟之前）與之前所有實跑都只多 0.25～0.43 MiB。第二次 09:06～09:12Z（`…/validation-7d-rss2-20261005T090642Z/`）：`status: ok`、**`P_B` ＝ 161,882,112 bytes（154.4 MiB，餘裕 5.6 MiB）**、`fs_peak` 多出的量回到 0.27／1.46／0.27 MiB；十個程序的 cgroup 峰值 273.1～342.0 MiB（sizing 的量法⛔ 不變） |

⚠️ **本次實跑的觀察（本輪⛔ 沒有處置，待裁決）**：sizing 的 `P_path = max(dirs_peak, fs_peak, accounted)` 裡的 `fs_peak` 是整個根檔案系統的已用量，
**量測窗口期間 host 上任何其他寫入都會算進來**——第一次 sizing 在使用者剛清完磁碟之後跑，`fs_peak` 被墊高了 7 MiB，`P_B` 只差 196,608 bytes 就超過
`P_B_BUDGET`；重跑一次就回到正常。換句話說，⑨-2 的正式 sizing（`--formal`）可能只因為當下 host 的背景寫入就超過預算、走 v29「六、1」的回退順序。
`P_B` 的量法是 v29 確認過的，本增補⛔ 不改；是否要在正式 sizing 之前另外約束 host 的狀態（例如量測期間停掉其他寫入）、或接受重跑，需另行裁決。
⚠️ **2026-10-06 查證與裁決**：成因⛔ 不是「剛清完磁碟」，而是 live 的 `sr_analysis` 排程（平日台北時間 17:00，與量測窗口重疊）；同樣的雜訊也會影響 ⑨-1。使用者裁決方案 B（有效性條件 ＋ ⑨-1 的 fail-fast ＋ 作業規則），見「Stage 2 量測的有效性條件與 ⑨-1 fail-fast 計畫」。

**本輪的驗證**：`python/scripts/test.sh` 完整執行（依序）✅——pytest **2150 passed、1 skipped**（本輪 ＋119）；`test-replay-args.sh` **452 項**（本輪 ＋18）、`test-i074-stage2.sh` **223 項**（含 host unittest 56 項，本輪 ＋19：`RssAddendumHost` 19 項）全部通過；doc-refs 45／45、文件引用 0 個問題。⚠️ 第一次完整執行在 `test-i074-stage2.sh` 的 ⑩ 完整流程失敗——**根檔案系統只剩約 556 MiB**，preflight 6 的磁碟檢查（需要 1,241,513,984 bytes）照設計擋下（環境，⛔ 不是程式）；使用者清出空間之後重跑全綠。真正 repo 的 worktree 登記數 ＋4（[I-118](#i-118replay-相關腳本會洩漏-git-worktree註冊與-tmp-目錄都會累積) 既有的增量）；`/dev/shm/i074-*`、`/run/lock` 的 `i074-stage2.*`、帶 `i074.stage2.run` 的容器都沒有殘留。tooling 路徑⛔ 沒有動：stage 之後 `--verify "$(git write-tree)"` 通過。

**本輪的反向驗證**（逐項注回 → 對應測試變紅 → 逐位元還原、比 SHA；pytest 23 項、host unittest 13 項、shell 11 項，共 47 項）：✅ **47／47 紅在預期的那幾支**。
⚠️ 第一次跑完是 45／47，兩項是**測試本身的缺陷**，修正後重跑那兩項才紅：

| # | 注回的缺陷 | 第一次 | 原因與修正 |
|---|---|---|---|
| H8 | `reap-adopted` 不驗 starttime | ⚠️ 沒有紅 | 測試的目標程序忽略 TERM：缺陷讓 TERM 照樣送出，但 starttime 一直「變」使它被當成已消失、KILL 永遠輪不到——目標活下來，測不出差別。改成⛔ 不忽略 TERM 的目標（一個 TERM 就會結束），之後紅 |
| S3 | 拿掉 `measure_subreaper_guard` | ⚠️ 沒有在預期的那一支紅 | 手動 re-exec 那一支以 `grep … \| head \| cut` 取 PID，⛔ 沒有 `\|\| true`——訊息不在時 `pipefail` ＋ `set -e` 讓整段測試中止，而⛔ 不是讓那一支失敗。補上之後紅（反向驗證抓到的測試缺陷） |

其餘 45 項（摘要）：報告的門檻（改回 cgroup 峰值、拿掉精確閘或偵測器、邏輯寫反、notes 用「不高於」、schema 留在 v1、產出端不經 validator、validator 不重新推導／不驗衍生欄位／以報告自己的預算為基準、產出端另寫一份門檻邏輯）；wrapper（不含 `RUSAGE_SELF`、不收孤兒、存活的子孫當成沒事、拿掉 `SigIgn` 偵測、讀不到 `total_rss` 當成 0、改讀 v2、結束碼不傳回訊號、寫 stdout）；兩份 schema 放寬；observer 讀不到 `total_rss` 仍標完整；靜態掃描不解析 alias；`host-run`（不含 `RUSAGE_SELF`、拿掉 `SigIgn`、不收孤兒、取樣只看直接子程序、逾時不清理、被 TERM 不清理、清不乾淨卻回 0）；`reap-adopted` 只在開頭驗 parent；`kill-pinned`（不驗 starttime、把 zombie 當成存活、TERM 之後讀取失敗當成已消失、`result` 與結束碼不對應）；shim 從工作複本掛載 wrapper；bootstrap 不設 subreaper；拿掉每一步之後的收養檢查；`reap-adopted` 失敗仍刪 S；guard 在 ppid 不符時沒有收掉測試程序；身分不可信仍送訊號；guard 改回外部的 `setsid`；拿掉能力檢查；能力檢查逾時之後不清容器；拿掉逐步檢查——全部紅在預期的那幾支。

**增補實作第一輪 review 的修正**（2026-10-05；兩項都先寫測試、確認紅之後才修）：

| # | review 的發現 | 查證 | 修正 |
|---|---|---|---|
| 1 | 中：`host-run` 寫進紀錄的 `rc` 可能與實際的結束碼不同——先把 leader 的結束碼寫進紀錄，之後才決定回傳 70 或 128 ＋ 訊號；TERM／INT／HUP 實際回 143／130／129 但紀錄可能是 0，清不乾淨實際回 70 但紀錄是 leader 的結束碼；測試只驗回傳與清理結果 | ✅ 成立（讀碼；新斷言在修正前紅） | 先算出**唯一的** `final_rc`（清不乾淨 70 ＞ 訊號 128 ＋ N ＞ leader 的結束碼），紀錄與回傳同一個值；ac24 的三種訊號與清不乾淨那一支都斷言 `rec["rc"]` ＝ 實際結束碼、`host_record_problems()` 沒有 `rc` 那一條（量測仍因中斷或清不乾淨而無效） |
| 2 | 中：report v2 的 validator 還⛔ 不是真正的封閉型別驗證——`container_memory_limit_bytes` 只有在已經是 list 時才比對（換成字串直接跳過、回傳 `[]`）；`peaks` 型別錯時仍呼叫 `.get()`、拋 `AttributeError` 而⛔ 不是回傳問題 | ✅ 成立（實測：`"not-a-list"` → `[]`；`peaks=[]` → `AttributeError`） | validator 改寫：每一節**先**驗完型別（`_typed()`：int／pos／nonneg／bool／str，`bool` ⛔ 不得冒充整數；`_peaks_ok()` 含 `dirs_peak_by_location`；`container_memory_limit_bytes` 必須是正整數陣列；字串欄位、violations 各欄），型別不符就跳過那一節的交叉運算；最外層另有後援——任何未預期的例外都轉成問題、⛔ 不拋出。新測試：**對報告裡每一個節點逐一換成錯的型別**（ok 與 violating 兩份報告，各數百個組合）→ 一律回傳問題、⛔ 不拋例外；另加 review 點名的十二種具體竄改與 violations 各欄 |

本輪的反向驗證：B1′（review 描述的原缺陷：⛔ 不驗型別、只在是 list 時才比對）→ 紅在新增的三支；B2（拿掉後援、`peaks` 型別錯仍做交叉運算）→ 紅；B4（紀錄寫 leader 的結束碼）→ 紅在 ac24 的兩支。⚠️ B1（只拿掉型別檢查、保留無條件的比對）與 B3（拿掉 violations 各欄的型別檢查）沒有紅——那兩道與後面的檢查重疊（無條件的相等比對、由列重新推導的違反）：拿掉其中一道，另一道照樣擋下，⛔ 不是缺口，照實記錄。驗證：`python/scripts/test.sh` 完整執行見下一行。

本輪的驗證：`python/scripts/test.sh` 完整執行——shell 那幾層全部通過（`test-replay-args.sh` 452 項、`test-i074-stage2.sh` 223 項（含 host unittest 56 項）、doc-refs 45／45、文件引用 0 個問題）；pytest 那一層在 65% 被 kernel 的 OOM killer 收掉（`anon-rss` 403 MB）：這一次 mem-guard 依當下的 `MemAvailable` 只給 409m（前一次是 442m）。實測本輪新增的測試⛔ 沒有墊高記憶體（單獨跑的峰值 195.5～201.3 MiB，與既有的同一個量級），是整個 pytest 程序累積的峰值碰到較低的上限；依專案的做法（⛔ 不調高 `MEM`），把同一組 pytest **分三段依序跑完**：539 ＋ 663 ＋ 963 ＝ **2165 passed、1 skipped**（＝ 前一次的 2150 ＋ 本輪新增的 15）。

**增補實作第二輪 review 的修正**（2026-10-06；先寫測試、確認紅之後才修）：

| # | review 的發現 | 查證 | 修正 |
|---|---|---|---|
| 1 | 中：`host-run` 在決定 `final_rc` 之後、handler 復原之前收到的 TERM／INT／HUP 會被靜默吞掉——訊號落在序列化或 `write_exclusive()` 期間時，handler 只追加 `received`，⛔ 不會重算 `final_rc` 或補上錯誤（review 以注入式診斷穩定重現：回傳 0、紀錄 0、`errors` 為空） | ✅ 成立（新增的測試 B 以同一個注入點在修正前紅：回傳 0） | 定義**訊號的 linearization point（L）**：決定 `final_rc` 之前先以 `pthread_sigmask` 擋住 TERM／INT／HUP（CPython 在回傳之前會先跑完已送達的 handler），再以 `sigtimedwait` 取走已 pending 的——**L 之前送達的一律反映到錯誤、紀錄的 `rc` 與回傳碼**；L 之後（序列化與寫入期間）送達的留在 pending，紀錄寫完再取：紀錄是 exclusive create、已經定案、⛔ 不能改，回傳碼照樣改成 128 ＋ N、stderr 照實說明，紀錄的 `rc` 與實際結束碼因此不符 → check-step 與報告擋下（fail-closed，⛔ 不宣稱成功）；最後才復原 handler、解除阻擋。新增三支決定性的測試：A（L 之前送 TERM → 143、紀錄 143、錯誤記下）、A2（擋住之後才送、pending 的 HUP → L 取走 → 129）、B（review 的注入點：寫紀錄的入口送 TERM → 回傳 143、紀錄 0、`host_record_problems()` 抓到 rc 不符） |

本輪的反向驗證：C1（L ⛔ 不擋訊號——即原本的競態）→ 紅在 B；C2（紀錄寫出之後⛔ 不再取 pending 的訊號——晚到的 TERM 在解除阻擋時以預設動作殺掉程序）→ 紅在 B；C3（L 收到的訊號⛔ 不記進錯誤）→ 紅在 A、A2 與既有的三種訊號那一支。本輪的驗證見下一行。

本輪的驗證：`python/scripts/test.sh` 完整執行——shell 那幾層全部通過（`test-replay-args.sh` 452 項、`test-i074-stage2.sh` 223 項（含 host unittest **59** 項：本輪 ＋3）、doc-refs 45／45、文件引用 0 個問題）；pytest 那一層同樣在 `test_replay_envcheck`（65%）被 OOM killer 收掉（mem-guard 這次只給 419m，`anon-rss` 415 MB；與第一輪 review 的情況相同），同一組 pytest 分三段依序跑完：539 ＋ 663 ＋ 963 ＝ **2165 passed、1 skipped**（本輪⛔ 沒有改 pytest）。

**增補實作第三輪 review 的修正**（2026-10-06；先寫測試、確認紅之後才修）：

| # | review 的發現 | 查證 | 修正 |
|---|---|---|---|
| 1 | 中：晚到的訊號蓋過「清不乾淨 → 70」的優先序——`final_rc` 明定清不乾淨 70 ＞ 訊號，但第二輪新增的「紀錄寫出之後」分支遇到晚到的訊號就無條件回 128 ＋ N（review 以決定性注入同時製造清理失敗與寫入期間的 TERM：回傳 143、紀錄 70、`cleanup_complete` false、`leftover_pids` `[999999]`）；failure summary 因此把主因記成訊號 143，⛔ 不是更重要的「host 子程序清不乾淨」。第二輪的測試只涵蓋單獨的晚到 TERM | ✅ 成立（新增的測試在修正前紅：`143 != 70`） | 晚到的分支套用**同一個優先序**：有殘留 → 結束碼維持 `final_rc`（70，與紀錄一致），stderr 照樣記下晚到的訊號、說明清不乾淨優先；沒有殘留 → 照第二輪回 128 ＋ N（紀錄與實際不符 → fail-closed）。新增測試「清理失敗 ＋ 寫紀錄期間的 TERM」：`_reap_nohang` 永遠收不到 `ECHILD`、`_cleanup_direct_children` 回 `[999999]`、在寫 host 紀錄的入口送真正的 TERM → 斷言回傳 70、紀錄 70、`cleanup_complete` false、`leftover_pids` `[999999]`、stderr 記下晚到的訊號、`host_record_problems()` 沒有 rc 不符那一條 |

本輪的反向驗證：D1（晚到的分支無條件回 128 ＋ N——即原缺陷）→ 紅在新增的那一支；D2（晚到的分支一律維持紀錄的 rc——吞掉晚到的訊號）→ 紅在第二輪的 B（單獨的晚到 TERM），證明修正⛔ 沒有把第二輪的守門拿掉。兩項的檔案都逐位元還原（SHA 前後相同）。

本輪的驗證：`python/scripts/test.sh` 完整執行（stage 之後；545 秒）全綠——`test-replay-args.sh` 452 項、`test-i074-stage2.sh` 223 項（含 host unittest **60** 項：本輪 ＋1）、doc-refs 45／45、文件引用 0 個問題；pytest 這一次⛔ 沒有被 OOM killer 收掉，一次跑完 **2165 passed、1 skipped**（本輪⛔ 沒有改 pytest）。

**review 之後**（commit 由使用者決定）：增補⛔ 沒有動 tooling 路徑——commit 後再跑一次 `scripts/make-i074-tooling-patch.sh --verify "$(git rev-parse 'HEAD^{tree}')"`。✅ 2026-10-06 已跑（`576a8f7`）：通過，tooling 路徑與 `d1267bb` 無差異。
⚠️ ⑨-1 的正式驗收必須以包含本增補的 commit 為 `repo_head`；`--formal` 的報告是 v2。

**歸檔**（⚠️ 依 CLAUDE.md，本筆的計畫與結果保留到 review 確認後才收斂）：現況規格寫進 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「I-074 Stage 2 的容量驗收」
（對象與門檻、契約與範圍、容器的記憶體上限、報告 v2、⛔ 不在 host 留下程序、observer 的 `total_rss`），操作程序寫進
[`development-workflow.md`](./development-workflow.md)（acceptance 一節的門檻、能力檢查、逐步檢查與收養、故障注入、observer 的 `total_rss`；sizing 一節的共用原語）。

#### Stage 2 量測的有效性條件與 ⑨-1 fail-fast 計畫（v8，2026-10-06，✅ **已確認**（review 七輪後 commit `0a92d95`））

⚠️ **緣起**：「⑦d 增補的實作結果」記下的 `fs_peak` 觀察（第一次 sizing 的 `P_B` 只差 196,608 bytes 就超過 `P_B_BUDGET`）。2026-10-06 查證：
⛔ **不是**原本推測的「剛清完磁碟」，而是 **live 的排程工作**——live（postgres、backend、worker…）與量測共用同一個根檔案系統，backend 的
`SR_ANALYSIS_CRON=0 17 * * 1-5`（`TZ=Asia/Taipei`）在 2026-10-05 09:00:00Z 啟動、09:02:02Z 記下 `sr analysis done`；量測窗口換算成
UTC：witness 08:59:25–09:00:41（未解釋的用量從約 08:59:56 起上升）、success 09:00:42–09:02:27（整段以約 60 KiB/s 上升，工作結束後變平）、
failure 09:02:28 之後（平的）；重跑在 09:07 之後，乾淨。盤中、沒有批次工作時實測根檔案系統 2 分鐘的漂移只有 0.7 KiB/s。
歷次實跑 24 個磁碟窗口（sizing 6 次 × 3、acceptance 3 次 × 2）逐一對照：21 個 `fs_peak − accounted` 在 −4.3～−0.06 MiB（會計上界成立，
`P_path` ＝ `accounted`），只有那一次的 witness、success（＋3.4、＋6.0 MiB）與重跑的 success（＋684,032 bytes：t＝15.5 秒時一次約 1 MiB 的外部寫入）超出
（review 另行重算 9 份報告、24 個窗口，結果相同）。
⚠️ 影響 ⑨-1 **也**影響 ⑨-2：兩者的磁碟路徑都用 `P_B` 的量法，餘裕只有 5.6 MiB（sizing）／6.2 MiB（acceptance 的 success）；而 ⑨-1 的磁碟窗口在前、
約 3 小時的量測趟在後、門檻到最後才判定——雜訊造成的超標要等量測趟跑完才知道（rc＝2、唯一的額度已用掉）。
✅ **使用者裁決（2026-10-06）：方案 B**（⛔ 不採「只靠作業規則」的 A、⛔ 不採改 `P_path` 定義的 C、⛔ 不採「超標時允許重跑」）。

##### 一、目標與⛔ 不做

| 項目 | 內容 |
|---|---|
| 目標 | ① **有效性條件**（含第一輪 review 補的**模糊區**）：量測無效就⛔ 沒有判定、可以重跑；sizing 與 acceptance 共用；② **⑨-1 的 fail-fast**：`--replay-compute full` 時，量測趟**之前**以共用的 collector 判定已量到的全部門檻與有效性，結果寫成封閉契約的 `precheck.json`（第一輪 review），讀取端以**受信任 repo** 在 `repo_head` 的程式與常數、從原始量測重算（第二、三輪 review），不過就⛔ 不跑量測趟（額度⛔ 不消耗）；跑完以封閉的 **raw manifest** 為原始量測留下事後比對的錨（第五輪 review）；③ **作業規則**：正式量測避開 live 的批次時段、量測期間⛔ 不做其他會寫根檔案系統的事 |
| ⛔ 不做 | ⛔ 不改 `P_path = max(dirs_peak, fs_peak, accounted)`、`P_B`、`P_B_BUDGET`、`M_safety` 的定義與值；⛔ 不改 sizing 報告（v1）與 acceptance 報告 v2 的 schema 與語意——`derive_acceptance_violations()` 的輸出⛔ 不變、v2 只在沒有任何有效性問題時寫出（第三輪 review）；⛔ 不改 freeze record（`check-pair`）、preflight、⑩ 的 observer；⛔ 不碰 live（⛔ 不停排程，只避開時段）；晉升的窗口（資訊值）⛔ 不套有效性條件；⛔ 不動 tooling 路徑 |

##### 二、設計

| 項目 | 規則 |
|---|---|
| 判定的基準 `P_basis` | 每條磁碟路徑另算 `P_basis = max(dirs_peak, accounted)`：`dirs_peak` 只量流程自己的位置（L1、L2、L3、L5），`accounted` 是已知寫入的會計上界——兩者都⛔ 不受 host 其他程序影響。`fs_peak` 是**整個檔案系統已用量的淨變化**（`statvfs`），其他程序的寫入會墊高它、刪除會壓低它 |
| 有效性條件 ①：未解釋的淨成長 | `fs_unexplained = fs_peak − P_basis` 必須 **≤ 1 MiB**（`FS_UNEXPLAINED_TOLERANCE = 1,048,576`）。⚠️ 它偵測的是**淨的**未解釋正成長（第一輪 review 訂正）：外部的刪除可能抵銷外部的寫入或流程漏算的寫入，所以⛔ 不能證明量測期間沒有外部 I/O、也⛔ 不保證抓得到每一筆漏算——作業規則用來降低抵銷的機會 |
| 有效性條件 ②：模糊區（第一輪 review） | **容差⛔ 不得決定結果**：`P_path > P_B_BUDGET` 但 `P_basis ≤ P_B_BUDGET`——超標完全由容許範圍內的 `fs_unexplained` 造成——→ **無效**（`ambiguous_disk_exceed`），可以重跑。只有 `P_basis` 本身超過預算，才可能形成不可重跑的正式超標。sizing（witness、success、failure）與 acceptance（success、failure）都套；sizing 的 `build_report()` 因此多一個參數 `p_b_budget`（呼叫端一律傳 `i074_stage2_preflight.P_B_BUDGET`，與 acceptance 相同）。⚠️ **模糊區只是有效性問題，⛔ 不是違反**（第二輪 review）。⚠️ 報告 v2 的 `derive_acceptance_violations()` ⛔ 不改，磁碟仍是 `P_path > P_B_BUDGET`（第三輪 review）：v2 只在沒有任何有效性問題時寫出，而沒有有效性問題時 `P_path > 預算` ⟺ `P_basis > 預算`（否則就落在模糊區）。以 `P_basis` 判、記 `P_basis` 的只有 `precheck.json` 的 `disk_p_basis`（見下）。freeze record 的「`P_B > P_B_BUDGET` ⛔ 不寫」照舊（見下一列 sizing 的部分） |
| 確定的違反優先（第二輪 review；取代 v2 的「有效性優先」） | **確定的違反**＝與磁碟雜訊無關的違反：記憶體的每一類（容器的精確閘與偵測器、host 的最大單一與群組取樣），以及 `P_basis > P_B_BUDGET` 的磁碟超標。判定的優先序：① 有確定的違反 → **threshold_exceeded**（⛔ 不得重跑），同一次的有效性問題照樣記錄；② 沒有確定的違反、但有有效性問題（① 或 ②）→ **invalid**（可以重跑）；③ 都沒有 → ok。**報告維持既有語意**（第三輪 review；⛔ 不升 v3）：acceptance 報告 v2 **只在沒有任何有效性問題時**寫出——有有效性問題（不論有沒有確定的違反）→ `SizingError`、⛔ 不產報告（fail-closed，原始量測保存到 `raw-failed/`）；**混合狀態**（確定的違反 ＋ 有效性問題）的判定**只由 `precheck.json` 表達**。full 模式的最後報告結構上⛔ 不會遇到有效性問題（磁碟窗口在 precheck 之前就結束，precheck 不是 ok 就⛔ 不跑量測趟）——最後報告若出現有效性問題就是 harness 的缺陷 → fail-closed；stub 模式只供開發驗證（⛔ 不是判定），遇到混合狀態同樣⛔ 不產報告。`validate_acceptance_report_v2()` 多驗「沒有任何有效性問題」（由 `paths` 與程式的容差推導）——只是**收窄**：既有的有效報告（dev6）照樣通過，舊版 validator 對新報告的結論不變。sizing 報告 v1 沒有逐項的違反清單、判定是 freeze record 的 `P_B ≤ P_B_BUDGET`，所以混合狀態照常寫出：確定的違反必然 `P_B` ＞ 預算，v1 讀取端的結論就是超標，語意不變；只有「沒有確定的違反、但有有效性問題」才拋 `SizingError`；文字版與 stderr 列出 `P_basis` 與有效性問題 |
| 判定與重跑的紀律 | 只有 **invalid** 可以重跑；**第一個不是 invalid 的結果就是判定**——threshold_exceeded（sizing：寫出的報告 `P_B > P_B_BUDGET`；acceptance：報告 rc＝2、或 `precheck-verdict` 判讀為 `threshold_exceeded`）⛔ 不得重跑，照 v29「六、1」的回退順序。每一次嘗試（含 invalid 的）都記進本筆 |
| 為什麼是 1 MiB | 正常的 21 個窗口最高 −0.06 MiB；重跑那一次的外部寫入 684,032 bytes；兩邊的預算餘裕 ≥ 5.6 MiB；模糊區規則讓這 1 MiB ⛔ 不會單獨造成不可重跑的超標 |
| 共用的 collector（第一輪 review） | 把 `build_acceptance_report()` 的蒐集部分抽成 `collect_acceptance_measurements(state, *, stage, p_b_budget, …)`，`stage` ∈ {`before_full`, `complete`}，回傳**內部的量測快照**（meta、manifest、limits、paths、promotion、memory、host）——⛔ **不是** `i074_stage2_acceptance_report_v2`：沒有 schema、contract、status、violations 等頂層鍵，v2 的 validator 一定拒絕它。`before_full` 的預期步驟與 invocation 是 full 模式正式集合的**精確前綴**（＝ 少了最後的 `replay_full`；程式內以「正式集合[:len(前綴)] ＝ 前綴」斷言，不符即拋錯）。⚠️ collector 讀的每一個集合（`rc.tsv` 的步驟、`index/`、`containers/`、`twins/`、lifecycle event、`host/`、`phases/`）都必須**恰好**等於預期的集合——⛔ 不靜默忽略多出的資料（第二輪 review）；唯一的例外是下面「讀取端」經過驗證、明確列舉的量測趟 suffix。`build_acceptance_report()` ＝ collector（`complete`）＋ 有效性 ＋ v2 包裝（輸出⛔ 不變）；precheck ＝ collector（`before_full`）＋ 有效性 ＋ `derive_acceptance_violations()`（同一個函式，吃快照的 limits／memory／host／paths） |
| `precheck.json` 的契約（第一輪 review） | schema `i074_stage2_acceptance_precheck_v1`（canonical JSON），**封閉**的頂層鍵：`schema`、`status`、`full_trip`、`identity`（`run_id`、`mode`、`replay_compute`、`image`、`repo_head`、`clone_head`、`harness_manifest_sha256`）、`limits`（`memory_bytes`、`P_B_BUDGET`、`fs_unexplained_tolerance_bytes`）、`steps`（涵蓋的步驟名，＝ 前綴）、`sequences`（涵蓋的 invocation，＝ 1..n）、`disk`（success、failure 各恰好 `P_path`、`dirs_peak`、`fs_peak`、`accounted`、`P_basis`、`fs_unexplained`）、`validity_problems`（每筆恰好 `kind` ∈ {`fs_unexplained`, `ambiguous_disk_exceed`}、`subject`、`value`、`limit`）、`violations`（每筆恰好 `kind`、`subject`、`value`、`limit`；記憶體那幾類與報告 v2 的 enum 與推導相同——共用 `derive_acceptance_violations()` 拆出的記憶體段；磁碟一律是 **`disk_p_basis`**：`P_basis > P_B_BUDGET`、`value` 記 `P_basis`，裁決值就是證據值（第三輪 review），⛔ 不出現 `disk_p_path`）。**優先序**（第二輪 review；取代 v2 的完全互斥）：`violations`（只含確定的違反）非空 ⟺ `status = threshold_exceeded`，此時 `validity_problems` **可以**非空；否則 `validity_problems` 非空 ⟺ `status = invalid`；否則 `ok`。`full_trip` ＝ `allowed`（ok）／`not_started`（其他）。**寫入**：`<S>/precheck.json`，`write_exclusive`（`O_CREAT \| O_EXCL`，⛔ 不覆寫）；三種結果都寫，隨 S 複製到 `raw/`（繼續）或 `raw-failed/`（中止） |
| `precheck.json` 的讀取端（第三輪 review 重寫） | `helper precheck-verdict --raw <raw 或 raw-failed 目錄> --repo <受信任的 repo> --expected-repo-head <40 碼 OID>`（三個參數都必填；第四輪 review）。⚠️ **offline 的讀取端⛔ 不執行、⛔ 不 import 原始量測目錄裡的任何程式**——快照只當成要比對的 bytes（同一個 artifact 裡的模組、`MANIFEST`、`identity` 與 `precheck.json` 只能證明彼此一致，⛔ 不是信任根）：⓪ **外部的 commit 錨點**（第四輪 review）：`--expected-repo-head` 由呼叫端提供、⛔ 不取自 artifact——必須是 40 碼小寫 hex，且 `git -C <repo> rev-parse --verify <expected>^{commit}` ＝ 它自己（⛔ 不接受 ref、縮寫或 tag）；① `precheck.json` 只當資料解析，`identity.repo_head` 必須 ＝ `expected_repo_head`——**不符就在取出或執行任何程式之前拒絕**；② 快照的每一個檔案（`MANIFEST` 列出的相對路徑，⛔ 不接受 `..`、絕對路徑或重複）逐位元 ＝ `git show <expected_repo_head>:<路徑>`，`MANIFEST` 的 canonical SHA ＝ `identity.harness_manifest_sha256`；③ 一律從 **`expected_repo_head` 的 git object** 取出到私有的暫存目錄（⛔ 不由 artifact 的任何欄位選擇 commit），以 `python3 -I` 執行取出來的 helper 的 `precheck-recompute`（工作樹版本**只執行固定的驗錨與取出 frontend**，⛔ 不得呼叫工作樹的 `precheck-recompute`／collector（第七輪 review）；取出的是受信任的程式；它驗 `MANIFEST` 的路徑集合 ＝ 自己的 `SNAPSHOT_FILES`、從取出來的 preflight／supervisor 載入常數）；④ **形狀**：A＝狀態恰好是前綴（precheck 之後中止，`raw-failed/`）→ collector（`before_full`）直接重算（它本身就驗前綴每一項的內容）；B＝前綴 ＋ 量測趟的 suffix（成功的 `raw/`）→ **先以 collector（`complete`）驗完整狀態的全部內容**（`rc.tsv` 的預期與實際、sidecar 的狀態與 RSS 欄位、twin 的狀態與 spec hash、host 紀錄的 schema／清理／RSS、lifecycle event 的完整與先後、invocation 的 spec hash——與最後報告用的是同一套檢查），再驗 suffix 恰好是唯一合法的那一組（`rc.tsv` 在前綴之後恰好一行 `replay_full`、`index/`／`containers/`／`twins/` 恰好多出 sequence n＋1 且它是 full 模式正式集合的最後一個 invocation、lifecycle event 恰好多出它的 `create_begin` 與 `rm_done`、`host/` 恰好多出 `replay_full.json`）且 `precheck.json` 的 `status` 是 ok，**最後**才明確投影掉 suffix、以 collector（`before_full`）重算；多出、缺少、重排或內容不合法 → 拒絕；⑤ 重算的結果與檔案**逐位元相同**（canonical）——記憶體列的違反由這一步證明。全部通過才印出 `status`、rc＝0；任何一項不符、缺檔、空檔、截斷、⛔ 不是 canonical、`expected_repo_head` 不是受信任 repo 的 commit、`identity.repo_head` ≠ 它、快照 ≠ 它的內容 → rc＝1「無法判讀」。⚠️ 照實的界線：它證明**程式與常數**來自受信任 repo 中由呼叫端指定的 `expected_repo_head`、`precheck.json` 是原始量測在那份程式下的確定結果；⛔ **不證明原始量測本身沒被竄改**（量測資料沒有外部的錨）——所以 ⑨-1 的程序在跑完當下以下一列的 **raw manifest** 為原始量測建立事後比對的錨，記進本筆（之後 commit）。⚠️ **`expected_repo_head` 的來源**（第四輪 review）：⑨-1 開跑之前在真正 repo 以 `git rev-parse HEAD` 取得（`--formal` 已守 `scripts/`、`python/`、`.gitattributes` 與 HEAD 相同、HEAD ⛔ 沒有在 bootstrap 之後移動，而快照清單的檔案都在這些路徑內，所以快照 ＝ 錨點；`docs/` 等其他路徑的未 commit 變更⛔ 不影響判讀——第五輪 review 訂正，⛔ 不另加「整棵工作樹 clean」的前置條件），開跑前就記下並回報（量測期間⛔ 不改真正 repo，跑完才寫進本筆）——⛔ 不從跑完的 artifact 讀回；歸檔時一併記錄這個值與它的來源（freeze record 在 ⑨-2 才產生，⑨-1 用不到它）。判讀不了的只有**快照清單裡的檔案**與錨點不同的執行（例如快照清單內有未 commit 變更的開發執行），照實；其他路徑的未 commit 變更⛔ 不影響（第五輪 review）。⚠️ ⑨-1 的程序**只認 `precheck-verdict` 的輸出**；無法判讀⛔ 不得當成可重跑，交人工查明（原始量測照樣保存）。量測趟途中失敗（狀態介於 A、B 之間）是量測趟的崩潰，依計次政策的「崩潰重試 1 次」處理，⛔ 不經 precheck 的判讀 |
| raw manifest（事後比對的錨；第五輪 review） | **對外**：`helper raw-manifest --raw <raw 或 raw-failed 目錄> --repo <受信任的 repo> --expected-repo-head <40 碼 OID> (--out <raw 目錄之外的新路徑> | --check-sha256 <記下的值> [--out <新路徑>])`。⚠️ **雙層、與 `precheck-verdict` 同一個信任模型**（第六輪 review）：對外的命令只做錨點的驗證（40 碼小寫 hex、`rev-parse --verify <expected>^{commit}` ＝ 它自己）、從 **`expected_repo_head` 的 git object** 取出 helper 到私有的暫存目錄、以 `python3 -I` 執行取出那一版的內部命令 `raw-manifest-recompute`；工作樹版本**只執行固定的驗錨與取出 frontend**，⛔ 不得呼叫工作樹的 `raw-manifest-recompute`／manifest 產生演算法（第七輪 review）、⛔ 不從原始量測目錄載入或執行任何程式。產生（⑨-1 跑完當下）與日後的檢查都走這一個入口，所以兩次用的一定是同一個 commit 的演算法。兩個對外命令共用同一個「驗錨點 → 取出受信任的 helper → `python3 -I` 執行」的前端函式。**產出**：canonical JSON（`canonical_dumps()`：鍵排序、`separators=(",", ":")`、UTF-8、`ensure_ascii=False`、結尾⛔ 不加換行）的 `i074_stage2_raw_manifest_v1`，封閉的頂層鍵恰好是 `schema`、`expected_repo_head`、`root_name`（`raw` 或 `raw-failed`）、`file_count`、`dir_count`、`files`、`dirs`。**走訪**：`--raw` 本身必須是目錄（`lstat`，⛔ 不是 symlink）；往下逐項 `lstat`、⛔ 不跟隨 symlink：一般檔案 → `files` 一列，恰好 `path`、`size`（`st_size`）、`sha256`（內容）；目錄 → `dirs` 一列（**含空目錄**；根目錄本身⛔ 不列）；symlink、FIFO、socket、裝置檔 → 拒絕、⛔ 不產 manifest。**路徑**：相對於 `--raw` 的 POSIX 路徑（`/` 分隔），每一段必須是合法 UTF-8（無法解碼 → 拒絕），⛔ 不得是空字串、`.` 或 `..`；`files` 與 `dirs` 各自依路徑的 UTF-8 bytes 遞增排序（與 code point 的順序相同）、⛔ 不得重複；`file_count`／`dir_count` ＝ 兩個陣列的長度。**輸出**（第六輪 review）：`--out` 以 exclusive create 寫（`O_CREAT \| O_EXCL`；路徑已存在——含一般檔案、symlink、懸空的 symlink——→ 拒絕、⛔ 不覆寫、原檔⛔ 不變）；父目錄必須已存在，解析之後落在 `--raw` 之內 → 拒絕（避免自我雜湊）；寫入途中失敗 → 刪掉本次建立的那個檔案。產生模式必須帶 `--out`。**檢查**：帶 `--check-sha256` 時先在記憶體重建、比對 SHA-256：相同 → rc＝0，有 `--out` 才以同樣的 exclusive 規則寫出；⛔ 不同 → rc＝1、⛔ 不寫任何檔案——新增、刪除、改名、改一個 byte、多或少一個空目錄都會讓重新產生的 bytes 不同，換成 symlink 或特殊檔案則直接拒絕。⚠️ **錨是記進本筆的 SHA-256**，manifest 檔本身只方便查看。**產生與日後重建**都經上述的雙層入口（工作樹版本只執行驗錨與取出的 frontend，⛔ 不呼叫工作樹的產生演算法）；schema 名稱固定演算法，演算法要改就換 schema 名。**本筆記錄**：manifest 的 SHA-256、`file_count`、`dir_count`、`expected_repo_head` 與它的來源、產生與檢查用的完整指令；work 目錄頂層在 raw 目錄之外的檔案（`acceptance_report.json`／`.txt` 或 `failure_summary.json`）另記各自的 SHA-256 |
| 權威常數的來源（第二輪 review） | collector、validator、precheck 與 verdict 需要的 `P_B_BUDGET`（唯一定義在 `i074_stage2_preflight.py`）與 `ENV_DROP_NAMES`／`ENV_DROP_PREFIXES`（`scripts/lib/i074-stage2-supervisor.py`）**live 一律取自 harness 的快照**（offline 的讀取端見上一列，⛔ 不從原始量測目錄載入）：sizing 的快照清單（`SNAPSHOT_FILES["sizing"]` 與 `scripts/i074-stage2-sizing.sh` 的 `I074_BOOT_FILES`）加入 `python/scripts/i074_stage2_preflight.py`；acceptance 的（`SNAPSHOT_FILES["acceptance"]` 與 `scripts/i074-stage2-acceptance.sh` 的 `I074_BOOT_FILES`）加入它與 `scripts/lib/i074-stage2-supervisor.py`。新增 `_load_frozen(<S 的快照目錄>, <相對路徑>)`：先驗該檔的 SHA-256 ＝ `MANIFEST` 的那一行，才以 `importlib` 載入（兩個模組在模組層級只 import 標準函式庫，載入⛔ 沒有副作用）——只給 live 用：S 的快照由 bootstrap 從受信任的 repo 建立並驗證過。live 的 `report`（sizing）、`acceptance-precheck`、`acceptance-report` 另外斷言快照裡的這兩個檔案與工作複本的**逐位元相同**（步驟實際執行的是工作複本的 `clean_env()`，freeze record 也從工作複本 import `P_B_BUDGET`）——不同即 fail-closed。⛔ 不讀活路徑、⛔ 不複製常數。函式層一律以參數接收常數（測試直接傳），只有 CLI 層載入快照。`--formal` 既有的「清單內的檔案 ＝ HEAD」與 S 的形狀檢查依清單運作，自動涵蓋新加的兩個檔案 |
| ⑨-1 的 fail-fast | acceptance 的 full 模式流程：…`recover_envcheck` → **metadata twin（量測趟之前的 invocation）** → **`helper acceptance-precheck`**（寫 `precheck.json`；結束碼 0 ok／2 threshold_exceeded／3 invalid／其他 ＝ 錯誤）→ 0 才跑量測趟（`replay_full`）→ **metadata twin（只有量測趟那一個）** → 報告。2、3 與錯誤都走 `on_failure`（harness rc＝1；訊息寫明「量測趟未執行、額度未消耗」與哪一種）；判讀一律經 `precheck-verdict`。stub 模式⛔ 不跑 precheck（流程與結束碼照舊：0／2／1） |
| metadata twin 分兩次 | twin 本來在「所有量測窗口結束之後」才建；量測趟本身⛔ 沒有磁碟窗口，所以把 twin 提到量測趟之前⛔ 不影響任何窗口。`run_twins()` 改成**只處理還沒有 twin 結果的 invocation**（已有的跳過；每一份仍以 `write_exclusive` 寫；`load_invocations()` 的完整性檢查照舊要求每個 sequence 恰好一份）；sizing 只呼叫一次，行為不變 |
| 作業規則（歸檔到 `development-workflow.md`） | 正式量測（⑨-1、⑨-2）開始之前，以 `docker inspect` **只看排程鍵**（`*_CRON`，⛔ 不印其他環境變數）確認 live 的批次時段（現行：台北時間平日 06:30、16:00、17:00、22:00，每日 07:00），讓磁碟窗口（⑨-1 約前 15 分鐘、⑨-2 約 6 分鐘）避開它們與它們的執行期間；量測期間⛔ 不跑測試、⛔ 不 build image、⛔ 不做其他會寫根檔案系統的工作。⚠️ 這是降低無效與抵銷機率的作業規則，⛔ 不是保證 |

##### 三、待確認的決定

| # | 決定 | 理由 |
|---|---|---|
| 1 | 容差 1 MiB，以常數寫死（⛔ 不開參數）；搭配模糊區規則 | 可調就可以事後挑；模糊區讓容差⛔ 不會單獨決定結果（第一輪 review：補上之後可接受） |
| 2 | precheck 的超標、無效與錯誤都讓 harness 以 rc＝1 結束，由封閉契約的 `precheck.json` 經 `precheck-verdict`（`--repo` ＋ `--expected-repo-head`）區分 | 結束碼 2 一直代表「報告已寫」（第一輪 review：補上封閉契約之後可接受；第三輪 review：讀取端改成受信任 repo、形狀 B 先驗全部內容） |
| 3 | 有效性條件只套磁碟路徑；晉升的窗口只列資訊值 | 晉升的磁碟⛔ 不與預算比較（v29「八之一」容量列）（第一輪 review：同意） |
| 4 | 歷史紀錄⛔ 不追溯改寫；只在本筆照實記下「以新條件重算，只有 10/05 第一次 sizing 會被判無效」 | ⑤ 的正式量測三條路徑都是負值（第一輪 review：同意，並已重算確認） |
| 5 | ~~有效性優先~~ → **確定的違反優先，其次有效性問題**（第二輪 review 不同意 v2 的做法，改成這樣）；模糊區只是有效性問題 | 磁碟窗口無效⛔ 不會讓同一次量到的精確 RSS 或 `P_basis` 失效；v2 的做法等於用無關的磁碟雜訊取得重跑資格（第三輪 review：同意） |
| 6 | acceptance 報告 v2 **維持原語意**：只在沒有任何有效性問題時寫出，混合狀態只由 `precheck.json` 表達（第三輪 review 的兩個選項之二；⛔ 不升 v3） | full 模式的最後報告結構上不會遇到混合狀態；stub 只供開發驗證；⛔ 不需要第二份 schema。sizing 報告 v1 的判定是 `P_B ≤ 預算`，混合狀態照寫⛔ 不改變它的語意 |
| 7 | offline 的 `precheck-verdict` 需要受信任的 repo（`--repo`）與**外部的 commit 錨點**（`--expected-repo-head`，第四輪 review），只支援快照 ＝ 該 commit 的執行 | ⛔ 不執行原始量測目錄裡的程式，也⛔ 不讓 artifact 選擇要執行哪個 commit；⑨-1 是 `--formal`（快照 ＝ HEAD），錨點是開跑前記下的 HEAD |
| 8 | 原始量測的事後錨點是封閉的 raw manifest（`i074_stage2_raw_manifest_v1`）的 SHA-256，記進本筆並 commit（第五輪 review） | 演算法封閉在 schema 裡、產生與日後重建都經 `--repo` ＋ `--expected-repo-head` 從 git object 取出的同一個 helper 執行（第六輪 review），算出的值才確定與當時同義；它只能偵測**之後**的改動，⛔ 不證明跑完當下之前的狀態 |

##### 四、受影響檔案與資料流

| 檔案 | 改動 |
|---|---|
| `python/scripts/i074_stage2_sizing.py` | `FS_UNEXPLAINED_TOLERANCE`、`disk_validity_problems(paths, p_b_budget)`（① 與 ②）、判定的優先序；`build_report()` 加 `p_b_budget`；`derive_acceptance_violations()` 拆出記憶體段共用（輸出⛔ 不變）、precheck 的 `disk_p_basis`；`collect_acceptance_measurements()`（從 `build_acceptance_report()` 抽出；每個集合恰好等於預期）；`build_acceptance_report()` 改用它；`validate_acceptance_report_v2()` 多驗「沒有任何有效性問題」；precheck 的 builder、`validate_acceptance_precheck_v1()`、形狀 B 的完整驗證與 suffix 投影；`run_twins()` 跳過已有結果；`SNAPSHOT_FILES` 兩個 profile 各加檔案、`_load_frozen()`；子指令 `acceptance-precheck`、`precheck-verdict`（外部 commit 錨點、git 比對與取出、⛔ 不 import 原始量測裡的程式）、`raw-manifest`（對外：驗錨點、從 git object 取出 helper）與 `raw-manifest-recompute`（內部）——`precheck-verdict` 與 `raw-manifest` 共用「驗錨點 → 取出受信任的 helper → `python3 -I` 執行」的前端函式、`precheck-recompute`（由 `precheck-verdict` 以取出的受信任版本執行）；`report`、`acceptance-report` 改從快照載入常數（並與工作複本比對）；兩份文字版列出 `P_basis`、`fs_unexplained` 與有效性問題 |
| `scripts/i074-stage2-acceptance.sh` | `I074_BOOT_FILES` 加 preflight 與 supervisor；full 模式的順序（twin → precheck → 量測趟 → twin）；precheck 結束碼對應 `on_failure` 的訊息 |
| `scripts/i074-stage2-sizing.sh` | `I074_BOOT_FILES` 加 preflight（第二輪 review：v2 寫的「⛔ 不改」不成立——快照裡沒有 preflight，就無法安全取得唯一真相源） |
| 測試 | `test_i074_stage2_sizing.py`、`test_i074_stage2_acceptance.py`、`scripts/test-replay-args.sh`（見「六」；隔離的最小 repo 與快照清單相關的既有測試依新清單更新） |
| 文件 | `docs/sr-zone-scoring.md`（容量驗收：有效性條件、模糊區、確定的違反優先、判定與重跑的紀律、`precheck.json` 與 raw manifest 的 schema 與不變條件、offline 讀取端的信任模型、權威常數取自快照）、`docs/development-workflow.md`（sizing 與 acceptance 兩節：有效性條件、full 模式的順序、`precheck-verdict` 的判讀、作業規則）、本筆 |

資料流：取樣（不變）→ `phase_peaks()`（不變）→ `paths`（`P_path` 不變）→ **有效性條件 ① ②**（新）→ 門檻推導（不變）→ 報告。full 模式另在量測趟之前多一次「twin → collector（`before_full`）→ 有效性 → 同一個門檻推導 → `precheck.json`」。

##### 五、風險與回滾

| 風險 | 對策 |
|---|---|
| 1 MiB 太緊，正常量測也被判無效 | 歷史 24 個窗口只有雜訊那一次超出；無效只花重跑的時間（sizing 約 6 分鐘、acceptance 量測趟之前約 15 分鐘），⛔ 不消耗量測趟 |
| 外部刪除抵銷了外部寫入或漏算 | 照實寫明有效性條件只偵測**淨**成長；作業規則降低機會；會計上界與目錄取樣仍是 `P_path` 的主要來源 |
| 流程真的有漏算的寫入，之後每次都無效 | 那正是要查明的缺陷（會計模型宣稱是上界）；照實回報，⛔ 不放寬容差 |
| precheck 與最後的報告推導不一致 | 共用 collector 與 `derive_acceptance_violations()` 的記憶體段；測試證明同一份沒有有效性問題的狀態加上通過的量測趟之後，既有的違反逐筆對應保留，量測趟自己超標只會**新增**違反 |
| `precheck.json` 損壞或被改 | 封閉契約 ＋ 身分綁定 ＋ 從原始量測重算逐位元比對；無法判讀⛔ 不得當成可重跑 |
| offline 判讀需要受信任的 repo 與外部的 commit 錨點 | ⑨-1 用的 commit 一定在 repo 裡、開跑前就記下；只有快照清單內的檔案與錨點不同的執行判讀不了（照實，決定 #7） |
| 原始量測本身被一致竄改 | 照實⛔ 不在 verdict 的保證內；⑨-1 跑完當下以 raw manifest 建立錨：manifest 的 SHA-256、檔案數、目錄數、`expected_repo_head` 與它的來源、指令記進本筆並 commit（決定 #8）——只偵測**之後**的改動 |
| 快照多了兩個檔案 | `--formal` 的「清單內的檔案 ＝ HEAD」與 S 的形狀檢查依清單自動涵蓋；開發時改了 preflight／supervisor 卻沒 commit，live 的「快照 ＝ 工作複本」檢查會中止（照實；正式量測一律 clean） |
| twin 分兩次出錯（重複或漏建） | `run_twins()` 只補缺的；完整性檢查照舊 |
| 回滾 | `git revert` 本計畫的實作 commit 即可；`P_path` 與兩份報告的 schema 都沒變，既有紀錄與 freeze record 不受影響 |

##### 六、測試與驗證

| 層 | 內容 |
|---|---|
| pytest：有效性 | ① 的邊界（＝ 1 MiB 有效、＋1 byte 無效、負值有效）；② 的邊界：`P_basis` ＝ 預算且 `fs_peak` ＝ 預算 ＋1（容差內）→ 無效（`ambiguous_disk_exceed`）、`P_basis` ＝ 預算 ＋1 → 有效且超標（acceptance 報告 rc＝2；sizing 報告寫出、freeze record 照舊拒寫）、`P_path` ＝ 預算 → 有效且⛔ 沒有超標；sizing 與 acceptance 的 builder 各自拋錯、訊息含路徑與 bytes；晉升的窗口超出⛔ 不影響；**優先序**（第二輪 review）：記憶體超標 ＋ 磁碟無效 → precheck 是 threshold_exceeded、stub 的報告⛔ 不產（rc＝1）；`P_basis` 超標 ＋ `fs_unexplained` 超限 → precheck 是 threshold_exceeded（`disk_p_basis`、`value` ＝ `P_basis`）、acceptance 報告⛔ 不產、sizing 報告寫出且 freeze record 拒寫；只有模糊區 → invalid（模糊區⛔ 不出現在 `violations`）；**v2 語意不變**（第三輪 review）：`derive_acceptance_violations()` 的輸出對既有報告逐位元不變、`validate_acceptance_report_v2()` 抓到有任何有效性問題的報告、dev6 照樣通過（封閉型別的 fuzz 測試照舊通過） |
| pytest：collector 與 precheck | 快照⛔ 不能通過 v2 的 validator；`before_full` 的步驟與 invocation 是精確前綴（多一步、少一步、量測趟已存在 → 拋錯）；每一個集合恰好等於預期（多一個 phase 目錄、多一份 host 紀錄、多一份 twin、不認得的 lifecycle event → 拋錯）；**保留性**：同一份沒有有效性問題的狀態，precheck 的違反與加上通過的量測趟之後報告的違反**逐筆對應**（記憶體逐筆相同；磁碟的 `disk_p_basis` ⟺ `disk_p_path`、`subject` 相同——沒有有效性問題時兩個條件等價），量測趟超標時報告只多出它自己的那幾筆；四種結果（ok、記憶體超標、磁碟超標、無效）寫出的 `precheck.json` 通過讀取端 |
| pytest：`precheck.json` 的讀取端 | 封閉 schema 對每一個節點逐一換成錯的型別 → 一律回問題、⛔ 不拋例外；優先序的每一種錯配（例如 `invalid` 但有違反、`invalid` 但 `validity_problems` 空、`ok` 但有違反或有效性問題、`full_trip` 不符）；**形狀**（第二輪 review）：完整成功的狀態經合法 suffix 投影後可重算、且 `status` 必須是 ok，suffix 多出、缺少或重排任何一項（多一個 invocation、少了 twin、`replay_full` 不在最後、多一個 host 紀錄、多一個 phase 目錄）→ 一律拒絕；**形狀 B 的內容**（第三輪 review）：集合都對、但 `replay_full` 的預期／實際結束碼不符、sidecar 是 `measure_failed` 或缺 RSS 欄位、twin 是 `measure_failed` 或 spec hash 不符、host 紀錄 schema 不符或 `cleanup_complete` 為 false、lifecycle event 少了 `rm_done` 或先後顛倒 → 各自無法判讀；**信任根**（第三輪 review）：在原始量測的快照裡放一份**與 `MANIFEST`、`identity` 一致**、但 import 時會寫出標記檔的 preflight → 無法判讀（≠ `expected_repo_head` 的內容），且標記檔⛔ 沒有出現（⛔ 沒有被執行）；`expected_repo_head` 不在受信任的 repo、快照與它差一個 byte → 無法判讀；**外部錨點**（第四輪 review）：受信任的 repo 同時有正常的 commit C1 與另一個 commit C2（C2 的 `precheck-recompute` 入口一被呼叫就寫標記檔），artifact 與 C2 完全一致（快照 ＝ C2、`identity.repo_head` ＝ C2）、`--expected-repo-head` ＝ C1 → 在取出或執行任何程式之前拒絕、標記檔⛔ 沒有出現；另外讓**目前工作樹**的 `precheck-recompute` 入口也寫標記檔，`--expected-repo-head` ＝ C1 的正常判讀⛔ 不得觸發它（第七輪 review：標記一律放在 recompute／collector 的入口，⛔ 不放在模組載入或對外的 frontend——工作樹的 frontend 本來就會執行）；`--expected-repo-head` 是 ref（`HEAD`）、縮寫、tag 或不存在的 OID → 拒絕；缺少它 → 用法錯誤；live 的快照與工作複本不同 → fail-closed；身分綁定（`run_id`、`repo_head`、`harness_manifest_sha256` 各改一個）；重算不符（改檔案裡的一個數字、或改原始量測的一個樣本）；缺檔、空檔、截斷、⛔ 不是 canonical → 無法判讀；第二次寫入 → 失敗（exclusive） |
| pytest：raw manifest（第五輪 review） | 同一份目錄產生兩次逐位元相同；新增、刪除、改名、改一個 byte、多一個或少一個空目錄 → `--check-sha256` 不符（rc＝1）；symlink（指向檔案、指向目錄、懸空）、FIFO → 拒絕、⛔ 不產 manifest；`--raw` 本身是 symlink → 拒絕；非 UTF-8 的檔名 → 拒絕；`--out` 在 `--raw` 之內（含經 symlink 解析之後）→ 拒絕；手改 manifest 的 `files` 順序或多一個空白 → 與重新產生的 bytes 不同；含非 ASCII 檔名的固定案例釘住排序與 canonical bytes；缺參數 → 用法錯誤；**信任的執行路徑**（第六輪 review）：目前工作樹與另一個 commit C2 的 `raw-manifest-recompute`／manifest 產生演算法的入口都會寫標記檔（第七輪 review：標記放在演算法的入口，⛔ 不放在模組載入或對外的 frontend——工作樹的 frontend 本來就會執行），`--expected-repo-head` ＝ C1 → 只執行從 C1 取出的版本、兩個標記檔都⛔ 沒有出現，產出 ＝ C1 的演算法；缺 `--repo` → 用法錯誤；錨點是 ref、縮寫或不存在 → 拒絕；**輸出**（第六輪 review）：`--out` 已存在（一般檔案、指向檔案或目錄的 symlink、懸空的 symlink）→ 拒絕且原檔與 symlink 的目標都⛔ 沒有被改動；`--check-sha256` 不符 → rc＝1、`--out` 的路徑⛔ 沒有被建立；相符且帶 `--out` → exclusive 寫出、內容 ＝ 產生模式的輸出；產生模式缺 `--out` → 用法錯誤 |
| pytest：twin | `run_twins()` 第二次呼叫只建新的 invocation、已有的⛔ 不重建 |
| shell（fake docker） | full 模式：twin 與 precheck 的順序（fake docker 的紀錄：量測趟之前的 twin 都在 `replay_full` 之前、量測趟的 twin 在它之後）、成功的 `raw/` 經 `precheck-verdict --raw raw --repo <隔離的 repo> --expected-repo-head <隔離 repo 的 HEAD>`（形狀 B）判讀為 `ok`；⚠️ 本列每一個 `precheck-verdict` 的呼叫都帶齊三個參數（第四輪 review：⛔ 不寫只有 `--raw` 的範例，免得測試變成 fail-open）；兩個 harness 的快照含新加的檔案、`--formal` 時它們與 HEAD 不同 → 拒絕；precheck 判定超標（fake docker 讓 success 的 replay 回報超標的 RSS）→ rc＝1、`precheck-verdict --raw raw-failed --repo <隔離的 repo> --expected-repo-head <隔離 repo 的 HEAD>` 印 `threshold_exceeded`、**`replay_full` 沒有被執行**；precheck 判定無效（新的故障注入 `fs-noise`：success 窗口期間在量測範圍外寫 2 MiB、窗口結束後刪除）→ rc＝1、同一個指令（三個參數）印 `invalid`、量測趟沒有執行；改掉 `raw-failed/precheck.json` 的一個 byte → 同一個指令無法判讀；sizing 的 `fs-noise` → rc＝1、訊息含 `fs_unexplained`；`--formal` ⛔ 不接受 `fs-noise`（既有的「formal ⛔ 不接受故障注入」） |
| 反向驗證 | 拿掉 ①（sizing／acceptance／validator 各一）、`≤` 改 `<`、拿掉 ②（模糊區）、優先序反過來（有效性問題蓋過確定的違反）、模糊區算成違反、磁碟條件改回 `P_path`、晉升也套條件、precheck 不擋量測趟、precheck 放到量測趟之後、`precheck-verdict` 不重算、投影接受多出的項目、形狀 B 不先驗完整內容、offline 改從原始量測目錄 import、不比對快照與 `expected_repo_head`、改用 artifact 的 `identity.repo_head` 選擇要取出的 commit、錨點接受 ref 或縮寫、raw manifest 跟隨 symlink、漏列空目錄、`--out` 可以落在 raw 之內、`--check-sha256` ⛔ 不比對、對外的 frontend（`raw-manifest`、`precheck-verdict`）直接呼叫本地的 recompute、⛔ 沒有切換到 expected commit、`--out` 覆寫既有檔案、檢查不符仍寫出、precheck 的磁碟改記 `P_path`、v2 允許有效性問題、不驗身分綁定、不驗凍結模組與 `MANIFEST`、常數改讀工作複本、優先序拿掉一條、collector 忽略多出的集合成員、`run_twins()` 不跳過已有的——各自紅在預期的那幾支，逐位元還原 |
| 歷史資料的重算（唯讀） | 以新條件重算現有 9 份報告：預期只有 `validation-7d-rss-20261005T085906Z` 被判無效（① 超出；② ⛔ 沒有任何一份落入） |
| 實跑（dev，避開 live 批次時段） | acceptance stub 一趟、sizing validation 一趟；⛔ 不跑 full、⛔ 不跑 `--formal` |
| 完整測試 | `python/scripts/test.sh`（依序；pytest 若被 OOM 收掉就分段）、tooling patch `--verify`、`check-doc-refs`、禁止標記的否定詞掃描 |

##### 七、歸檔（實作後；review 前保留本計畫）

- `sr-zone-scoring.md`「I-074 Stage 2 的容量驗收」：有效性條件（① 淨成長、② 模糊區）、確定的違反優先、判定與重跑的紀律、⑨-1 的 fail-fast、權威常數取自快照，以及兩份耐久的契約——`i074_stage2_acceptance_precheck_v1` 與 **`i074_stage2_raw_manifest_v1`** 的 schema 與不變條件、offline 讀取端的信任模型（外部 commit 錨點、⛔ 不執行原始量測裡的程式；第六輪 review：契約歸這裡，移除本筆的計畫之後才不會只剩操作描述）。
- `development-workflow.md`：sizing 一節（有效性條件、作業規則）、acceptance 一節（full 模式的順序、`precheck-verdict` 的判讀與三個參數、外部 commit 錨點的來源、raw manifest 的產生與檢查的**操作指令**）。
- 本筆：實作結果、歷史資料重算、實跑、與計畫的差異；「⑦d 增補的實作結果」的觀察段落改指本計畫。

##### 第一輪 review 的修正（2026-10-06）

| 嚴重度 | review 的發現 | 查證 | 修正 |
|---|---|---|---|
| 中 | **1 MiB 容差可能形成不可重跑的假超標**：`P_basis` ＝ 預算、`fs_peak` ＝ 預算 ＋1 時量測有效，但門檻推導（`derive_acceptance_violations()` 以 `P_path > 預算` 判違反）會產生 `disk_p_path`，而「第一個有效的結果⛔ 不得重跑」 | ✅ 成立（讀碼：`P_path` 含 `fs_peak`） | 「二」新增**模糊區**：`P_path > 預算` 但 `P_basis ≤ 預算` → 無效、可以重跑；只有 `P_basis` 本身超過預算才可能是正式超標。`P_path`、`P_B_BUDGET` 與門檻推導都⛔ 不改（寫出的報告裡兩者必然一致）（⚠️ 第二輪 review 之後：門檻推導的磁碟條件改成 `P_basis`，見第二輪的表）；sizing 的 builder 因此帶 `p_b_budget` |
| 中 | **`precheck.json` 是唯一的裁決證據，契約卻沒封閉**：schema、互斥規則、身分綁定、寫入方式、讀取端與損壞／竄改測試都沒定義 | ✅ 成立 | 「二」新增 `i074_stage2_acceptance_precheck_v1` 的封閉契約（鍵、型別、互斥、身分、exclusive 寫入）與讀取端 `precheck-verdict`（封閉驗證 ＋ 身分綁定 ＋ 從原始量測重算逐位元比對）；⑨-1 只認它的輸出，無法判讀⛔ 不得當成可重跑；「六」補對應測試 |
| 中 | **`before_full=True` ⛔ 不應產生可被當成完整 v2 報告的物件**；現行 builder 在 full 模式要求量測趟的步驟與紀錄全部存在 | ✅ 成立（讀碼） | 改成抽出共用的 collector：precheck 拿到的是**內部的量測快照**，⛔ 不是 v2 報告（v2 的 validator 一定拒絕）；`before_full` 的集合是正式集合的精確前綴（程式內斷言）；測試證明加上通過的量測趟之後既有違反完整保留、量測趟超標只新增違反 |
| 補充 | 「漏算超過 1 MiB 保證會被擋」說得太滿：`fs_peak` 是整個檔案系統的淨變化，外部刪除可能抵銷 | ✅ 成立 | 改成「偵測**淨**未解釋的正成長」，照實寫明⛔ 不能證明沒有外部 I/O；作業規則用來降低抵銷的機會 |

另外新增決定 #5（有效性優先）：補模糊區之後，「無效」與「超標」可能出現在同一次，需要明定先後。⚠️ 第二輪 review 不同意，改成「確定的違反優先」（見下表）。

##### 第二輪 review 的修正（2026-10-06）

| 嚴重度 | review 的發現 | 查證 | 修正 |
|---|---|---|---|
| 高 | **決定 #5「有效性優先」會丟棄已證實的獨立超標**：磁碟窗口無效⛔ 不會讓同一次量到的精確 RSS 失效；允許重跑等於用無關的磁碟雜訊取得重跑資格。`P_basis` ＞ 預算同理，⛔ 不受外部雜訊影響 | ✅ 成立 | 改成**確定的違反優先**：記憶體的每一類與 `P_basis` 超標 → threshold_exceeded（⛔ 不得重跑），否則有效性問題 → invalid；模糊區只是有效性問題。`precheck.json` 的完全互斥改成優先序（`threshold_exceeded` 可以帶非空的 `validity_problems`）；門檻推導的磁碟條件改成 `P_basis`；報告 v2 在確定的違反時照寫（有效性問題由 `paths` 推得），validator 要求「有有效性問題 ⟹ threshold_exceeded」 |
| 中 | **成功後的 `raw/` 無法依設計重算 precheck**：collector 的 `before_full` 只接受精確前綴，成功的 `raw/` 一定含 `replay_full` | ✅ 成立 | 讀取端明定兩種形狀：A（恰好前綴）直接重算；B（前綴 ＋ 唯一合法的量測趟 suffix）先逐項驗 suffix、`status` 必須是 ok，再明確投影掉。collector 的每一個集合都必須恰好等於預期，⛔ 不靜默忽略多出的資料 |
| 中 | **重算缺少兩個權威來源，「sizing 的 shell ⛔ 不改」不可行**：`P_B_BUDGET` 在 preflight、`ENV_DROP_*` 在 supervisor，兩個 harness 的快照清單都沒有它們；`precheck-verdict` 只收 `--raw` | ✅ 成立（讀碼：現行 `acceptance-report` 從 `--clone` 載入；`I074_BOOT_FILES` 與 `SNAPSHOT_FILES` 都沒有這兩個檔案） | 兩個 profile 的快照清單加入它們（sizing：preflight；acceptance：preflight 與 supervisor），`scripts/i074-stage2-sizing.sh` 因此要改；`_load_frozen()` 先驗 `MANIFEST` 才載入；live 另外斷言快照與工作複本逐位元相同；`precheck-verdict` 只用 `--raw` 裡凍結的版本並驗 `MANIFEST` 與 `identity`；⛔ 不讀活路徑、⛔ 不複製常數 |

裁決：#1（1 MiB ＋ 模糊區）、#2（rc＝1 ＋ `precheck-verdict`）方向可接受；#3、#4 同意；#5 改成「確定的違反優先，其次有效性問題」。
⚠️ 上表第一列「門檻推導的磁碟條件改成 `P_basis`；報告 v2 在確定的違反時照寫」與第三列「`precheck-verdict` 只用 `--raw` 裡凍結的版本」已被第三輪 review 推翻，見下表。

##### 第三輪 review 的修正（2026-10-06）

| 嚴重度 | review 的發現 | 查證 | 修正 |
|---|---|---|---|
| 高 | **offline 的 verdict ⛔ 不應從 `raw/` 執行 Python 模組**：模組、`MANIFEST`、`identity` 與 `precheck.json` 在同一個 artifact 裡，只能證明彼此一致，⛔ 不是外部信任根；一致地竄改整組仍可能通過，而且 verifier 會直接執行其中的任意程式 | ✅ 成立 | `precheck-verdict` 加 `--repo <受信任的 repo>`：快照只當 bytes，逐位元比對 `git show <repo_head>:<路徑>`，再從 **git object** 取出受信任的 helper 與常數模組、以 `python3 -I` 執行它的 `precheck-recompute`；offline ⛔ 不 import 原始量測目錄裡的任何程式。live 照舊載入 S 的快照（bootstrap 從受信任的 repo 建立並驗證過）。照實寫明它⛔ 不證明原始量測本身沒被竄改，⑨-1 跑完當下把 `precheck.json` 的 SHA-256 與原始量測的檔案摘要記進本筆並 commit（決定 #7） |
| 中 | **形狀 B 只驗 suffix 的集合，⛔ 沒有驗內容**：檔名都在但內容損壞的 suffix 可能被投影掉，最後仍得到 ok | ✅ 成立 | 形狀 B 先以 collector（`complete`）驗完整狀態的全部內容（與最後報告同一套檢查），才驗 suffix 集合、`status` 是 ok，最後才投影；「六」補 rc、sidecar、twin、host、event 各自內容損壞的測試 |
| 中 | **同名報告 v2 的門檻語意被改變**：`disk_p_path` 的條件改成 `P_basis`、v2 又能在有有效性問題時寫出——新舊 validator 結論可能不同、JSON 沒有結構化的有效性欄位、裁決值（`P_basis`）與記錄值（`P_path`）不一致 | ✅ 成立 | 採 review 的第二個選項（決定 #6）：**v2 維持原語意**——`derive_acceptance_violations()` 的輸出⛔ 不變，v2 只在沒有任何有效性問題時寫出（validator 的新檢查只是收窄），混合狀態只由 `precheck.json` 表達；`precheck.json` 的磁碟違反是 `disk_p_basis`、`value` 記 `P_basis`。sizing 報告 v1 的判定本來就是 `P_B ≤ 預算`，混合狀態照寫⛔ 不改變語意 |

裁決：#1、#3、#4、#5 同意；#2 方向同意，須修正前兩項（已修正）。
⚠️ 上表第一列的「從 git object 取出」仍由 artifact 的 `identity.repo_head` 決定 commit——第四輪 review 補上外部錨點，見下表。

##### 第四輪 review 的修正（2026-10-06）

| 嚴重度 | review 的發現 | 查證 | 修正 |
|---|---|---|---|
| 高 | **`--repo` 仍⛔ 不是完整的 commit 信任根**：`identity.repo_head` 取自待驗證的 artifact，讀取端只要求它是受信任 repo 裡的 commit——artifact 改指物件庫裡的另一個 commit、同步換掉快照與量測資料，verifier 仍會從那個 commit 取出 helper 並執行；`python3 -I` 只隔離環境，⛔ 不讓腳本本身變可信 | ✅ 成立 | 新增必填的 `--expected-repo-head <40 碼 OID>`（呼叫端提供、⛔ 不取自 artifact）：先驗它是受信任 repo 的 commit 且 `rev-parse` ＝ 它自己，再要求 `identity.repo_head` ＝ 它，**不符就在取出或執行任何程式之前拒絕**；取出一律從它（⛔ 不由 artifact 的欄位選 commit）。⑨-1 的錨點是開跑前在真正 repo 記下的 HEAD，歸檔時一併記錄值與來源。測試補 C1／C2（C2 一執行就寫標記檔）：artifact 指向 C2、錨點是 C1 → 執行前拒絕、標記檔⛔ 沒有出現 |
| 低 | **測試矩陣的 CLI 範例漏掉必要參數**：前段帶 `--repo`，threshold／invalid 的案例卻只有 `--raw raw-failed` | ✅ 成立 | 「六」的 shell 列每一個呼叫都帶齊 `--raw`、`--repo`、`--expected-repo-head`，並加註⛔ 不寫只有 `--raw` 的範例 |

裁決：#1～#6 可以接受；#7 補上外部 commit 綁定（已補）。

##### 第五輪 review 的修正（2026-10-06）

| 嚴重度 | review 的發現 | 查證 | 修正 |
|---|---|---|---|
| 中 | **原始量測的事後錨點沒有形成可重現的契約**：「每個檔案的 SHA-256 清單摘要」沒有定義涵蓋範圍（空目錄、symlink、特殊檔案）、路徑的編碼與排序、清單格式與摘要演算法、新增／刪除／改名如何被偵測、日後用哪個固定指令重建——它是補足「verdict ⛔ 不證明原始量測未遭竄改」唯一的外部錨點 | ✅ 成立 | 「二」新增 **raw manifest**（`i074_stage2_raw_manifest_v1`）：封閉的鍵、canonical JSON、`lstat` 走訪⛔ 不跟隨 symlink、含空目錄、symlink／特殊檔案／非 UTF-8／非法路徑一律拒絕、依 UTF-8 bytes 排序、輸出必須在 raw 之外；`--check-sha256` 重建比對；日後以 `expected_repo_head` 的 helper 重建；本筆記錄 SHA-256、檔案數、目錄數、錨點與來源、完整指令（決定 #8）。「六」補新增、刪除、改名、改一個 byte、空目錄、symlink、FIFO、排序的測試 |
| 低 | **dirty 的描述比現行守門更廣**：計畫寫「工作樹 clean」、dirty 的開發執行判讀不了，但 `--formal` 只守 `scripts/`、`python/`、`.gitattributes`（`scripts/i074-stage2-acceptance.sh` 的 `--formal` 守門：`git status --porcelain -- scripts python .gitattributes`）；只有快照相關的檔案與錨點不同才真的判讀不了 | ✅ 成立（讀碼） | 改成精確的描述：快照清單的檔案都在 `--formal` 守的路徑內，所以快照 ＝ 錨點；判讀不了的只有快照清單內的檔案與錨點不同的執行，`docs/` 等其他路徑的未 commit 變更⛔ 不影響；⛔ 不另加「整棵工作樹 clean」的人工前置條件 |

##### 第六輪 review 的修正（2026-10-06）

| 嚴重度 | review 的發現 | 查證 | 修正 |
|---|---|---|---|
| 中 | **日後重建「執行 expected commit 的 helper」只有文字要求，CLI 沒有可信的執行路徑**：`raw-manifest` 沒有 `--repo`、也沒有從 git object 取出並啟動那一版的機制；HEAD 移動之後直接執行目前工作樹的 helper，證明不了用的是 expected commit 的演算法 | ✅ 成立 | 改成與 `precheck-verdict` 相同的雙層設計：對外的 `raw-manifest` 加必填的 `--repo`，驗完整的錨點 OID、從它的 git object 取出 helper、以 `python3 -I` 執行內部的 `raw-manifest-recompute`；工作樹版本只執行驗錨與取出的 frontend、⛔ 不呼叫工作樹的 `raw-manifest-recompute`（第七輪 review 訂正措辭）、⛔ 不從原始量測目錄載入程式；產生與檢查走同一個入口；兩個對外命令共用前端函式。「六」補 C1／C2（工作樹與 C2 的演算法都會寫標記檔，指定 C1 時只執行 C1） |
| 低 | **`--out` 的寫入與檢查模式的行為沒定義**：是否覆寫、是否 atomic／exclusive、檢查不符時是否仍寫出 | ✅ 成立 | exclusive create（已存在、含 symlink → 拒絕、⛔ 不覆寫）、父目錄必須已存在、寫入失敗刪掉本次建立的檔案；檢查模式先在記憶體重建比對，⛔ 不符 → rc＝1、⛔ 不寫任何檔案，相符且帶 `--out` 才以同樣規則寫出。「六」補既有輸出、輸出是 symlink、檢查失敗⛔ 不建檔的測試 |
| 低 | **耐久文件的歸檔清單漏列 raw manifest 的契約** | ✅ 成立 | 「七」：`i074_stage2_raw_manifest_v1`（與 `precheck.json`）的 schema、不變條件與 offline 讀取端的信任模型歸到 `sr-zone-scoring.md`；操作指令留在 `development-workflow.md`；「四」的文件列同步 |

##### 第七輪 review 的修正（2026-10-06）

| 嚴重度 | review 的發現 | 查證 | 修正 |
|---|---|---|---|
| 低 | **文字矛盾**：raw manifest 列正確寫「⛔ 不執行目前工作樹的 helper 演算法」，第六輪的表卻寫「⛔ 不執行工作樹的 helper」——對外命令本身就由工作樹的 helper 提供 frontend，後者字面上不可能成立；測試的標記若放在模組載入或 frontend，工作樹的版本一定會觸發；反向驗證的描述也不精確 | ✅ 成立 | 統一成「工作樹版本只執行固定的驗錨與取出 frontend，⛔ 不得呼叫工作樹的 recompute／產生演算法」——`precheck-verdict`（`precheck-recompute`／collector）與 `raw-manifest`（`raw-manifest-recompute`／manifest 產生演算法）兩處、第六輪的表一併訂正；「六」的標記一律放在 recompute／演算法的入口，`precheck-verdict` 也補「工作樹的 recompute 入口寫標記檔、正常判讀⛔ 不得觸發」；反向驗證改成「對外的 frontend 直接呼叫本地的 recompute、⛔ 沒有切換到 expected commit」 |

#### Stage 2 量測的有效性條件與 ⑨-1 fail-fast 的實作結果（2026-10-06，✅ **review 通過**（兩輪）並 commit `c49fde1`）

✅ 依「Stage 2 量測的有效性條件與 ⑨-1 fail-fast 計畫」（v8，review 七輪後確認並 commit `0a92d95`）完成「二」的設計。⛔ **沒有跑 `--formal`、
沒有跑完整計算、沒有 commit**；程式與文件已 stage，停在 review。⚠️ ⛔ 沒有動 tooling 路徑（`evaluation.py`、`replay_bundle/`）。

| 項目 | 結果 |
|---|---|
| 有效性條件 | `disk_numbers()`（`P_basis`、`fs_unexplained`）、`disk_validity_problems()`（① ②）、`disk_basis_violations()`（`disk_p_basis`）。sizing：`build_report(state, *, p_b_budget)`——確定的超標照常寫報告、只有有效性問題 → `SizingError`（量測無效）；文字版逐路徑列出 `P_basis`、`fs_unexplained` 與有效性問題。acceptance：任何有效性問題 → `SizingError`（v2 只在沒有有效性問題時寫出）；`validate_acceptance_report_v2()` 多驗「沒有任何有效性問題」；`derive_acceptance_violations()` 拆出 `derive_memory_violations()`，輸出⛔ 不變 |
| 共用的 collector | `collect_acceptance_measurements(state, *, stage)` ＋ `acceptance_expected()`（`before_full` ＝ 正式集合的精確前綴，程式內斷言）；`phases/`、`host/` 的成員與 lifecycle event 的種類都必須恰好等於預期；`build_acceptance_report()` 以它重寫——**以 dev6 的原始量測重建的報告 v2 與當時的報告逐位元相同**（13,343 bytes），新的 validator 對它也⛔ 沒有問題 |
| precheck | `precheck_from_snapshot()`、`build_acceptance_precheck()`（寫出之前自我驗證）、`validate_acceptance_precheck_v1()`、`precheck_text()`；CLI `acceptance-precheck --state --clone`（`write_exclusive` `<S>/precheck.json`；結束碼 0／2／3） |
| full 模式的順序 | `scripts/i074-stage2-acceptance.sh`：…`recover_envcheck` → twin → `acceptance-precheck`（2、3、其他 → `on_failure`，訊息寫明「量測趟未執行、額度未消耗」）→ 量測趟 → twin（`run_twins()` 只補還沒有結果的）→ 報告 |
| offline 的讀取端 | 共用的 frontend `verify_anchor()`／`git_blob()`／`run_trusted()`（`GIT_NO_REPLACE_OBJECTS=1`；只取出 helper、preflight、supervisor 三個檔案到私有的暫存目錄，以 `python3 -I` 執行）；`precheck_verdict()`（`identity.repo_head` ＝ 錨點、快照逐位元 ＝ 錨點 commit，之後才執行）；內部的 `precheck_recompute()`（形狀 A／B）、`verify_full_suffix()`、`project_before_full()`；CLI `precheck-verdict`、`precheck-recompute` |
| raw manifest | `build_raw_manifest()`、`write_new_file()`、`raw_manifest_main()`；CLI `raw-manifest`（對外）、`raw-manifest-recompute`（內部） |
| 權威常數 | `SNAPSHOT_FILES` 與兩個 `I074_BOOT_FILES` 加檔（sizing：preflight；acceptance：preflight、supervisor）；`_load_frozen()`、`live_constants()`；`report`（多了 `--clone`）、`acceptance-report`、`acceptance-precheck` 改用它 |
| 故障注入 `fs-noise` | 共用原語 `measure_fs_noise_begin`／`measure_fs_noise_end`（success 窗口：work 目錄的父目錄寫 2 MiB、窗口結束後刪除；`on_failure` 也清掉）；兩個入口的封閉清單加入它；`--formal` 照舊拒絕任何故障注入 |
| failure summary | 多一個 `precheck` 欄位（`raw-failed/precheck.json` 的位置；判讀一律經 `precheck-verdict`） |

**與計畫的差異**（⚠️ 待 review 確認）：

| # | 計畫 | 實作 | 理由 |
|---|---|---|---|
| 1 | 「六」的 shell 列以 fake docker 跑 full 模式的端到端（順序、precheck 擋下量測趟、`raw/`／`raw-failed/` 經 `precheck-verdict` 判讀、`fs-noise`） | 順序與擋法改成**抽出 `acceptance.sh` 的實際片段 ＋ stub** 執行（與 ac26 同一種做法：full 0／2／3／5 與 stub 五種情境）；形狀 A／B 的判讀改在 pytest（`precheck_recompute()`）與 host unittest（真的 git、隔離的 repo、C1／C2）驗；`fs-noise` 的端到端改由 dev 實跑驗（見下方「實跑」） | 現有的 fake docker 沒有任何一支能把 acceptance 或 sizing 跑完整個流程（全部在中途的故障點中止）；為了這幾項另寫一套能扮演 runner、finalizer、晉升的 fake docker，成本與風險都高，而上述三層合起來涵蓋同樣的性質 |
| 2 | —— | 內部命令（`precheck-recompute`、`raw-manifest-recompute`）多了 `--trusted-root`，並要求自己的檔案路徑 ＝ 取出的那一份（否則拒絕） | 讓「⛔ 不得直接呼叫本地的 recompute」在程式內也成立 |
| 3 | —— | git 呼叫一律帶 `GIT_NO_REPLACE_OBJECTS=1` | replace refs 可以換掉錨點 commit 的內容 |
| 4 | —— | failure summary 多一個 `precheck` 欄位 | 只是位置，方便找到；判讀照樣只認 `precheck-verdict` |
| 5 | collector 的 `host/` 完整性 | 逐步讀取時照舊先報「缺少 … 的 host 端量測」，多出的成員在讀完之後才以集合比對 | 保留既有的錯誤訊息（既有測試釘住它） |
| 6 | 實跑：acceptance stub 一趟、sizing validation 一趟 | 另加 sizing ＋ `fs-noise` 一趟（取代 #1 的 fake docker 端到端）；⛔ 沒有跑 acceptance ＋ `fs-noise` | 根檔案系統只剩 2.9 GB（四趟約需 1.6 GB）；acceptance 與 sizing 共用同一套量測原語與 `disk_validity_problems()`，acceptance 的 builder 那一道由 pytest 與反向驗證 V2 涵蓋 |

**歷史資料的重算**（唯讀；以新條件重算現有 9 份報告、24 個窗口）：只有 `validation-7d-rss-20261005T085906Z` 被判無效（witness ＋3,584,000、success ＋6,311,936 bytes，都是 ①）；
其餘 23 個窗口的 `fs_unexplained` 最大 ＋684,032 bytes（`validation-7d-rss2-20261005T090642Z` 的 success）；⛔ 沒有任何一份落入模糊區、⛔ 沒有確定的超標。
⑤ 的正式量測（`formal-20260929T020903Z`）三條路徑都是負值，不受影響（決定 #4：歷史紀錄⛔ 不追溯改寫）。

**反向驗證**（逐項注回、確認變紅、逐位元還原；產品檔的 SHA 前後相同）：32 項中 30 項紅在預期的那幾支、2 項是照實的重疊——

| # | 注回的缺陷 | 結果 |
|---|---|---|
| V1～V3 | 拿掉有效性條件（sizing builder／acceptance builder／validator 各一） | 紅：sizing 的邊界與模糊區、acceptance 的「v2 只在沒有有效性問題時寫出」、validator 的那一支（acceptance builder 那一支的測試改成釘住 builder 自己的訊息——否則 validator 的訊息也含同一段文字，兩道重疊看不出來） |
| V4、V5 | 容差的 `≤` 改成 `<`；拿掉模糊區 | 紅：sizing 與 acceptance 的邊界／模糊區、precheck 的 ambiguous |
| V6、V6b | 優先序反過來（precheck／sizing） | 紅：precheck 的 memory＋noise、basis＋noise；sizing 的「確定的超標照寫報告」 |
| V7、V8 | 模糊區算成違反；precheck 的磁碟違反改記 `P_path` | 紅：precheck 的 ambiguous；basis＋noise（`value` ≠ `P_basis`） |
| V9 | 晉升的窗口也套有效性條件 | 紅：`test_validity_ignores_the_promotion_windows` |
| V10、V11 | precheck ⛔ 不擋量測趟；precheck 放到量測趟之後 | 紅：shell 片段的 full 2／3／5；full 0／2／3／5 |
| V12～V14 | 讀取端⛔ 不比對重算的結果；投影⛔ 不驗拿掉的恰好是 suffix；形狀 B ⛔ 不先驗完整內容 | 紅：A／B 的原始量測被改；投影；suffix 的 rc、sidecar（失敗、缺 RSS 欄位）、twin（失敗、spec hash）、host 紀錄 |
| V16～V18、V23 | ⛔ 不比對快照與錨點；改用 artifact 的 `repo_head` 選 commit；錨點接受 ref／縮寫／tag；對外的 frontend 直接呼叫本地的 recompute | 紅：原始量測裡的 preflight 被執行、C1／C2、raw manifest 接受 tag、工作樹的標記檔出現 |
| V19～V22、V24、V25 | raw manifest 跟隨 symlink、漏列空目錄、`--out` 可以落在 raw 之內、⛔ 不比對 `--check-sha256`、覆寫既有檔案、檢查不符仍寫出 | 紅：各自的那幾支 |
| V27～V31 | live ⛔ 不比對快照與工作複本；validator 拿掉優先序；collector 忽略多出的成員；`run_twins()` ⛔ 不跳過已有的；⛔ 不驗 event 的種類 | 紅：各自的那幾支 |
| V15（照實的重疊） | offline 改從原始量測目錄執行 recompute | ⛔ 沒有紅：在這之前已驗快照逐位元 ＝ 錨點 commit，從原始量測目錄執行與從 git object 執行的 bytes 相同——由 V16 那一道保證 |
| V26（照實的重疊） | ⛔ 不驗身分綁定（`identity` ＝ 同一個目錄的 `meta.tsv`／`MANIFEST`） | ⛔ 沒有紅：重算的 `precheck.json` 的 `identity` 也取自 `meta.tsv` 與 `MANIFEST`，改動任一個都會讓逐位元比對失敗——身分綁定只是更早、更清楚的失敗訊息 |

本輪的驗證：`python/scripts/test.sh` 完整執行（stage 之後；依序）全綠——doc-refs 45／45、文件引用 0 個問題；`test-replay-args.sh` **460 項**（本輪 ＋8：fail-fast 的五種情境、兩個 harness 的 `--formal` ⛔ 不接受 `fs-noise`、`fs-noise` 是認得的故障）；`test-i074-stage2.sh` **223 項**（含 host unittest **67** 項：本輪 ＋7——offline 讀取端的信任模型 6 支、`live_constants()` 1 支）；pytest **2266 passed、1 skipped**（本輪 ＋101；這一次⛔ 沒有被 OOM killer 收掉）。tooling 路徑⛔ 沒有動：漂移測試（index 的 tree）通過。

**實跑**（dev；⛔ 不跑 `--formal`、⛔ 不跑 full；2026-10-06 08:30～08:50Z ＝ 台北 16:30～16:50，在 16:00 backfill（16:00～16:25）與 17:00 `sr_analysis` 之間；
`repo_head` ＝ `clone_head` ＝ `0a92d95`，harness 由工作樹的快照執行）：

| 實跑 | 結果 |
|---|---|
| acceptance stub（`~/i074_stage2_acceptance/dev7-validity-20261006T080114Z/`） | ✅ 結束碼 0、`status: ok`、報告 v2；快照 8 個檔案（含 preflight、supervisor，`live_constants()` 與工作複本逐位元相同）；success `P_path` 161,513,472 bytes（`P_basis` 同值、`fs_unexplained` −225,280、預算餘裕 6,258,688）、failure 72,867,840（`fs_unexplained` −49,152）；十二個容器的精確閘 260.6～352.8 MiB；`MemAvailable` 低點 194.3 MiB |
| sizing validation（`~/i074_stage2_sizing/validation-validity-20261006T080114Z/`） | ✅ 結束碼 0、`status: ok`；快照 6 個檔案（含 preflight）；`P_B` ＝ 161,501,184 bytes（預算餘裕 6,270,976）；三條路徑的 `fs_unexplained` −110,592／−458,752／−548,864，⛔ 沒有有效性問題；文字版列出 `P_basis` 與 `fs_unexplained` |
| sizing ＋ `fs-noise`（`~/i074_stage2_sizing/validation-validity-noise-20261006T080114Z/`） | ✅ 照預期被判**無效**：結束碼 1、在 report 階段中止、⛔ 不產報告，`量測無效（有效性條件；⛔ 不是判定，可以重跑）：success：fs_unexplained（未解釋的淨成長） 1613824 bytes ＞ 1048576`（寫入的 2 MiB 減去會計上界原本的餘裕）；量測範圍外的暫存檔已刪除；`failure_summary.json` 的 `failed_stage` ＝ report |

三趟之後 `/dev/shm/i074-*`、帶 `i074sz-` 名稱的容器都⛔ 沒有殘留。⚠️ 根檔案系統剩 1.8 GB（96%）：三個實跑目錄共約 1.2 GB，⛔ 沒有刪除
（是否清理既有的 dev 實跑目錄由使用者決定）。

**實作第一輪 review 的修正**（2026-10-06；先寫測試、確認紅之後才修）：

| 嚴重度 | review 的發現 | 查證 | 修正 |
|---|---|---|---|
| 高 | **offline verifier 的 git 信任根可以被環境變數／PATH 繞過**：`_git_env()` 完整繼承 `os.environ`，`verify_anchor()`、`git_blob()` 呼叫 PATH 上的 `git`——`GIT_DIR`、`GIT_OBJECT_DIRECTORY` 能讓 `--repo` 被忽略，PATH 上的假 git 能偽造錨點並回傳任意的 Python，接著在 `run_trusted()` 被執行；`GIT_NO_REPLACE_OBJECTS=1` 只防 replace refs | ✅ 成立（讀碼；實測 `GIT_DIR=<真正 repo> /usr/bin/git -C /tmp rev-parse HEAD` 成功、拿掉 `GIT_DIR` 則 rc 128）；另外發現取出的 helper 那個 python 子程序也繼承整個環境（含 `LD_*`） | git 固定是 **`/usr/bin/git`**（不存在就拒絕、⛔ 不從 PATH 找）；新增 `offline_child_env()`——git 與取出的 helper 都以最小化的 allowlist 環境執行（`PATH=/usr/bin:/bin`、空的 `HOME`／`XDG_CONFIG_HOME`、`LANG`／`LC_ALL=C.UTF-8`、`GIT_CONFIG_NOSYSTEM`、`GIT_NO_REPLACE_OBJECTS`、`GIT_TERMINAL_PROMPT=0`；git 另加 `GIT_CEILING_DIRECTORIES` ＝ `--repo` 的上一層，讓 `--repo` 必須是 repo 的根目錄），⛔ 不繼承呼叫端的 `GIT_*`、`LD_*`、`PYTHON*`、`PATH`。host unittest 補兩支（修正前一紅一錯）：PATH 上的假 git（一執行就寫標記檔）⛔ 沒有被執行、`precheck-verdict` 與 `raw-manifest` 照常；`GIT_DIR`／`GIT_OBJECT_DIRECTORY`／`GIT_ALTERNATE_OBJECT_DIRECTORIES` 指向另一個空的 repo 時照常判讀 `ok`；`--repo` 不是 repo 而 `GIT_DIR` 指向受信任的 repo → 拒絕；`--repo` 是 repo 的子目錄 → 拒絕（⛔ 不往上找）；以 spy 證明每一個子程序的 argv 與環境（`/usr/bin/git`、`PATH` 固定、⛔ 沒有 `LD_*`／`PYTHON*`、`GIT_*` 只有 allowlist） |
| 低 | **`ambiguous_disk_exceed` 的定義與實作不一致**：計畫與耐久文件寫「超標由**容許範圍內**的 `fs_unexplained` 造成」，但實作⛔ 沒有限制 `fs_unexplained` ≤ 1 MiB——`P_basis` ＝ 預算、`P_path` ＝ 預算 ＋1 MiB ＋1 時同時記下 ① 與模糊區 | ✅ 成立（讀碼；最終判定都是 invalid，但原因分類不精確） | 依計畫補上限：模糊區另要求 `fs_unexplained` ≤ 容差，超過時只記 ①。補重疊邊界的測試（修正前紅）：`disk_validity_problems()` 在 `fs_unexplained` ＝ 容差／容差 ＋1 的兩個邊界、sizing builder 的訊息⛔ 沒有「模糊區」、precheck 的 overlap 只有 `fs_unexplained`，以及 validator 拒絕「同一條路徑又記成模糊區」的 `precheck.json`；耐久文件同步補「且 `fs_unexplained` ≤ 容差」 |

本輪的反向驗證：R1（模糊區⛔ 不限容差——原缺陷）→ 紅在 sizing 的重疊邊界、precheck 的 overlap、validator 的 overlap；R3（子程序改回繼承呼叫端的環境）→ 紅在兩支 host；R4（拿掉 `GIT_CEILING_DIRECTORIES`）→ 紅在子目錄那一段；R5（取出的 helper ⛔ 不帶最小化的環境）→ 紅在 spy 那一支。⚠️ R2（git 改回從 PATH 找）只紅在 spy 那一支（argv 是 `git`），假 git 的行為測試⛔ 沒有紅——子程序的 `PATH` 已經是最小化的 `/usr/bin:/bin`，Python 以子程序環境的 `PATH` 找執行檔，兩道防護重疊；R2b（兩道同時拿掉）→ 兩支都紅。檔案都逐位元還原。

本輪的驗證：`python/scripts/test.sh` 完整執行（stage 之後）——shell 那幾層全部通過（doc-refs 45／45、文件引用 0 個問題、`test-replay-args.sh` 460 項、`test-i074-stage2.sh` 223 項（含 host unittest **69** 項：本輪 ＋2））；pytest 那一層在 `test_replay_envcheck`（67%）被 OOM killer 收掉（mem-guard 這次只給 390m；與 ⑦d 增補時的情況相同），依專案的做法（⛔ 不調高 `MEM`）把同一組 pytest 分三段依序跑完：539 ＋ 767 ＋ 963 ＝ **2269 passed、1 skipped**（本輪 ＋3）。

**歸檔**（⚠️ 依 CLAUDE.md，本筆的計畫與結果保留到 review 確認後才收斂）：現況規格寫進 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「I-074 Stage 2 的容量驗收」
（量測的有效性條件、⑨-1 的 fail-fast、`precheck.json`、offline 的讀取端與信任模型、raw manifest、權威常數取自快照），操作程序寫進
[`development-workflow.md`](./development-workflow.md)（acceptance 一節的 ⑨-1 指令、有效性條件、fail-fast、precheck 的判讀、raw manifest、作業規則、故障注入；sizing 一節的有效性條件）。

#### Stage 2 步驟 ⑧ 計畫 v4（2026-10-07，⚠️ **待確認**）

⚠️ **緣起**：v29「八」的 ⑧ 原本是「測試矩陣」；⑦ 總綱 v1 改成**矩陣完整性稽核 ＋ 全量執行**——⑦ 各包已各自附上它負責的測試，
⑧ 逐 id 對照 a～z、aa～ai、n1～n12、n7b、B／C 的 a～m 與 ③ 的測試表，補齊缺漏後全量執行一次。本計畫補**稽核的範圍、方法、判準、
確認點、全量執行的命令與殘留的驗收條件**。v2～v4 依第一～三輪 review 改寫（修正對照見本節最後）。

##### 一、目標與⛔ 不做

| 項目 | 內容 |
|---|---|
| 目標 | ① **稽核**：「二」的每一個 id 展開成原子案例（subcase），各自對到實際的測試並判定；② **補齊**：部分與缺漏補測試；③ **測試衛生**：補上 2026-10-07 發現的程序洩漏（「四」）；④ **全量執行**一次（「五」） |
| ⛔ 不做 | ⑨ 的 differential guard、tooling 非語意 guard、兩份 patch 的封存、`e1cbbbd` ＋ 兩份 patch 的既有測試；⑨-1、⑨-2、⑩；⛔ 不改產品程式——稽核或補測試時發現產品與規格不符，記進本筆、回報裁決（照 CLAUDE.md 先查規格），需要改產品才補得起來的缺漏同樣先回報；⛔ 不動 tooling 路徑（`evaluation.py`、`replay_bundle/`）；⛔ 不動測試的入口與框架（`python/scripts/test.sh`、`scripts/test-replay-args.sh`／`scripts/test-i074-stage2.sh` 的框架部分）——發現測試沒有被入口執行、而修正需要動它們時，停在確認點 ② 另行確認是否擴大範圍；⛔ 不清理、⛔ 不 prune 開跑之前既有的 worktree 登記（I-118 的範圍） |

##### 二、稽核的範圍與粒度

| namespace | id | 規格來源 |
|---|---|---|
| v29「六、2」 | a～n、o～z、aa～ai（含 ac2）、n1～n12、n7b | 「I-074 Stage 2 計畫書 v29」「六、2」（含後續各計畫的加註） |
| v29「六、9」 | a～m（B／C 判讀器） | 「六、9」與「Stage 2 步驟 ⑦c 細部計畫 v1」「二之四」 |
| ③「十」 | a～z、aa～az、ba～bk，含 k2、o2、ag2、ag3、ah2～ah5、bd2～bd4、be2、bj0 | 「③ Stage 2 evidence contract 計畫書」「十」（v12 為準，含 ⑦ 總綱 v1 的加註） |
| ⛔ 不在範圍 | v29「六、1」a～i（容量；開發驗證在 ⑦d、正式驗收是 ⑨-1）、「六、3」（differential guard，⑨）、⑦a～⑦d 與有效性條件計畫各自補的測試列——稽核表只在它們剛好承接上述 id 時引用 | —— |

**粒度**（第一輪 review）：稽核表的一列是 **namespace ＋ id ＋ subcase**——規格寫「各一支」「每一列至少一支」「逐條」或以頓號列舉多個情境的
id（例如 v29 的 n7、n7b、n11，③ 的 w、ah、bd），每一個情境展開成一個 subcase；單一性質的 id 只有一個 subcase。彙總同時列**頂層 id 數**
（約 140）與**展開後的 subcase 數**。跨 namespace 同名的 id（例如 n、z、ad～ai）一律帶 namespace。以每個 id 的**最新**規格為準：
後續計畫的改寫照實引用；**移交到 ⑦ 的案例**（例如 ③ 的 ax、ba，n／ai／ay 的「在 replay 之前」那一層）⛔ 不是「不適用」——照樣稽核現在的
測試、判定涵蓋／部分／缺漏；「➖ 不適用」只留給**真正撤回、已不存在**的契約（寫明出處）。

##### 三、稽核的方法

1. **起點**：各包的「五、測試（id → 落點）」與 ③b、③d 實作結果的落點表——逐 subcase 找到**現在**實際存在的測試：pytest 寫成
   `檔案::測試名[參數]`、shell 寫成 `scripts/…sh` 的 pass 標籤、host unittest 寫成 `類別.方法`。
2. **讀測試本體、判定**（⛔ 不能只看名稱）——每一列記：規格來源、落點、**具體的斷言**、判定：
   - ✅ **涵蓋**：斷言驗到規格寫的性質。例如「replay ⛔ 未被呼叫」要有 spy 的斷言（只驗結束碼不算）；「串流、只讀一次」要有計數或 spy。
   - ⚠️ **部分**：測試存在，但少驗某個性質——寫明少什麼。
   - ❌ **缺漏**：找不到測試。
   - ➖ **不適用**：規格已撤回或已不存在——寫明出處。
3. **確認測試真的會跑**：在 `python/scripts/test.sh` 的執行範圍內、⛔ 沒有被 skip——查明 pytest 的「1 skipped」是哪一支、為什麼；
   shell 需要 image 的段落在 `IMAGE_REQUIRED=1` 下⛔ 不得靜默 skip。
4. **抽樣反向驗證**：總綱點名的 o、v、w、y、z、ab（v29「六、2」）各注回一次缺陷，確認既有測試仍然紅在預期的那幾支；其餘以各包當時的
   反向驗證紀錄為準（稽核表列出出處）。⚠️ **一律在隔離環境注回**（第一輪 review）——正式工作樹與 index 全程只讀：
   - **隔離環境（共用的建法，「五」的 smoke 也用它）**：綁定**當時 staged 的 tree**、**保留歷史物件**（既有的 shell 測試會在 `e1cbbbd`
     建 worktree）的 clone。以下面的 bash 腳本建立（第三輪 review：固定的 author／committer、⛔ 不執行任何 hook、以 `commit-tree` 建暫存
     commit、⛔ 不依賴 host 的全域 git 設定）；它斷言 clone 的 tree ＝ staged tree、工作樹乾淨、`e1cbbbd` 在。⛔ 不用 `git worktree add`
     （會增加 I-118 的登記）；之後在 clone 裡建的 worktree 都登記在 clone 自己的 `.git`。2026-10-07 實際執行過：tree 相符、正式 repo 的
     index、status 與 worktree 登記都沒變、目錄已存在時拒絕、約 158 MB。

```bash
#!/usr/bin/env bash
# 綁定 staged tree 的隔離 clone（「三」第 4 步）。用法：bash mk_clone.sh <scratchpad 裡尚不存在的目錄>
set -euo pipefail
REPO=/home/dev/workspace/stock_trading            # 真正 repo：只讀（write-tree 只把 index 寫成 tree 物件）
DEST="$1"
[ ! -e "$DEST" ] || { echo "目錄已存在：$DEST" >&2; exit 1; }
T=$(git -C "$REPO" write-tree)
git -c core.hooksPath=/dev/null clone -q --no-hardlinks --template= "$REPO" "$DEST"
git -C "$DEST" ls-files -z | (cd "$DEST" && xargs -0 -r rm -f --)
git -C "$REPO" archive "$T" | tar -x -C "$DEST"
git -C "$DEST" -c core.hooksPath=/dev/null add -A
T2=$(git -C "$DEST" write-tree)
[ "$T2" = "$T" ] || { echo "隔離 clone 的 tree $T2 ≠ staged tree $T" >&2; exit 1; }
C=$(GIT_AUTHOR_NAME=i074-step8 GIT_AUTHOR_EMAIL=i074-step8@invalid GIT_COMMITTER_NAME=i074-step8 \
    GIT_COMMITTER_EMAIL=i074-step8@invalid git -C "$DEST" commit-tree "$T2" -p HEAD -m "i074 step8: staged tree $T")
git -C "$DEST" update-ref HEAD "$C"
[ "$(git -C "$DEST" rev-parse 'HEAD^{tree}')" = "$T" ] || { echo "HEAD 的 tree ≠ $T" >&2; exit 1; }
[ -z "$(git -C "$DEST" status --porcelain=v1 --untracked-files=all)" ] || { echo "隔離 clone 不乾淨" >&2; exit 1; }
git -C "$DEST" cat-file -e e1cbbbdab44f8cf2d152e6ade9235d844f590d7f^{commit}
echo "$T"
```

   - **執行**：注回與測試都在 clone 裡。pytest 寫出**精確的 node id**、以 `SKIP_SHELL_TESTS=1 PY_IMAGE=<專屬 tag> python/scripts/test.sh -q <node id …>`
     執行（⛔ 不覆寫共用的測試 image）；shell 的 id **沒有依 id 執行單一段落的公開入口**——一律在 clone 裡跑**完整的官方腳本**
     （`IMAGE_REQUIRED=1 PY_IMAGE=<專屬 tag> scripts/test-replay-args.sh`、`scripts/test-i074-stage2.sh`），再看預期的那幾支有沒有紅；
     若必須新增 selector，屬測試框架的改動，停在確認點 ② 裁決。
   - **正式工作樹的守門**：開始前與結束後各記一次 `git write-tree`（index 的 tree）、`git status --porcelain=v2 --untracked-files=all`、
     工作樹每個檔案的 mode 與 SHA-256（含未追蹤檔、symlink 記目標）——三者必須完全相同；中斷或 OOM 時也一樣（注回從來⛔ 不在正式工作樹）。
   - **收尾**：專屬的 `PY_IMAGE` tag 在成功、失敗、中斷三條路徑都以 trap 刪除（刪除前記下它的 image ID），結束後驗證 tag 已不存在；
     刪掉 scratchpad 裡 clone 自己的目錄。
5. **稽核表**寫進本筆的「⑧ 稽核結果」：彙總各判定的數量（頂層 id 與 subcase 各一份）與缺漏清單。

##### 四、補齊與測試衛生（稽核之後）

| 項目 | 規則 |
|---|---|
| 部分與缺漏 | 先寫測試，再在隔離環境注回對應的缺陷確認它會紅（「三」第 4 步的做法）；只動測試檔與測試 fixture |
| 補的時候發現產品不符規格 | ⛔ 不修；記進本筆、回報裁決 |
| 發現測試沒被入口執行 | 修正若要動 `python/scripts/test.sh` 或 shell 的框架 → 停在確認點 ②，另行確認是否擴大範圍 |
| ⚠️ 測試衛生（2026-10-07 發現） | `scripts/tests/test_i074_stage2_host.py` 的 `RssAddendumHost` 以 `setsid sh -c` 起「存活的子孫」（`MARKER_LOOP`），清理只靠產品（`host-run`）與 `self.pids`（只記到 `host-run` 自己）——產品的清理失效時（例如反向驗證注入的缺陷）就洩漏：2026-10-07 收掉 6 個（10/05 兩次反向驗證留下、存活約 41 小時）。補法（第一輪 review）：① 每支測試一個**唯一的 cleanup token**，測試起的**每一個**長存程序的 argv 都明確帶它；② 獨立的、冪等的 helper（⛔ 不呼叫被測的產品函式）：只看同一個 UID、排除測試程序自己與它的祖先，掃 `/proc` 找 argv 含 token 的程序、以 PID ＋ starttime 釘住 → TERM → 等待 → KILL → **重新掃描**，直到固定點（掃不到任何候選）或逾時；送任何訊號之前都重新比對 PID ＋ starttime（⛔ 不對已被重用的 PID 送訊號）；③ **`/proc` 的正常競爭**（第二輪 review）：列舉之後、讀 `stat`／`cmdline` 之前程序就結束（`ENOENT`，且 `/proc/<pid>` 已不存在）→ 視為該候選已消失；只有 **PID 仍存在**但身分讀不到、解析失敗或確認不了 starttime，以及逾時、候選仍存活 → **測試失敗**（⛔ 不吞掉）；④ 測試本體在斷言之後**手動呼叫**一次並斷言「⛔ 沒有存活的候選」，`addCleanup` 再呼叫一次當保險；⑤ 另補一支：以 driver 讓產品的清理失效（monkeypatch 成 no-op）→ helper 仍收得掉、測試斷言收掉之後⛔ 沒有存活 |
| ⚠️ 確認點 ② | 稽核表與補法（預計新增或修改的測試清單）寫好之後**先停下來確認**，確認之後才開始補 |

##### 五、全量執行（命令封閉）

| 步驟 | 命令與規則 |
|---|---|
| 1 | 全部 stage 之後：`python/scripts/test.sh`（doc-refs、`check-doc-refs`、`test-replay-args.sh`（`IMAGE_REQUIRED=1`）、`test-i074-stage2.sh`、pytest） |
| 2：OOM 的退路 | ⚠️ **⛔ 不自動啟用**（第三輪 review）：目前的入口（`python/scripts/test.sh` 以不具名的 `docker run --rm` 跑 pytest）取不到 container 的身分，這台 kernel 的 OOM 訊息也⛔ 不帶 memcg——下面的條件只是**推論**、⛔ 不是身分綁定。所以 pytest 那一層以 rc＝137 結束時：**中止並回報**（附游標之後新增的 kernel 訊息與步驟 1 的完整輸出），由你裁決是否以分段補跑；要讓入口提供 container 身分（例如 `--cidfile`）屬測試入口的改動，⛔ 不在本計畫——要做就在確認點 ② 另行提出。一般的測試失敗⛔ 不得以分段掩蓋。回報時附上的證據：證據（第二輪 review）：步驟 1 開跑之前記下 kernel log 的游標（`dmesg` 最後一行的單調時間戳 `[秒.微秒]`；⛔ 不用 `dmesg -T`——它換算的時間⛔ 不可信）；pytest 那一層以 rc＝137 結束之後，只看游標之後新增的行：必須至少一行 `OOM killed process`／`Out of memory`，它的程序名稱是 Python 的程序（`pytest`、`python*`；⛔ 不寫死某一個名稱——既有的紀錄是 `(python)`、本機近期是 `(pytest)`），且 `anon-rss` 落在 `test.sh` 印出的 `mem=` 上限的 80%～100%（對上那一個容器）。游標那一行已不在 ring buffer（被沖掉）、`dmesg` 讀不到、或找不到符合的行，都照實寫明。**經你同意之後**才分段：⛔ 不調高 `MEM`，以 `SKIP_SHELL_TESTS=1 python/scripts/test.sh -q -ra $s1`（`$s2`、`$s3` 同理）依序跑三段（shell 那幾層已在步驟 1 跑完；glob 在 `python/` 底下展開、＝ 容器的 `/app`，見下方的命令）——**shard 1** ＝ `backtest/modular/tests tests backtest/modular/sr_scoring/tests/test_[a-h]*.py`、**shard 2** ＝ `backtest/modular/sr_scoring/tests/test_i*.py`、**shard 3** ＝ `backtest/modular/sr_scoring/tests/test_[j-z]*.py`；每段記 passed、failed、errors、skipped、xfailed、xpassed、deselected（⛔ 不能只加總 passed）。**shard 的證明**（跑之前）：見下方「收集的比對」 |
| 3：smoke（決定 #4） | ⚠️ **在「三」第 4 步的 staged-tree 隔離 clone 裡**執行（第二輪 review：`git diff --quiet` 抓不到未追蹤與 ignored 的檔案，而 smoke 以 `COPY . .` 建 image、以暫存 index `git add -A python` 組 tooling patch——clone 只有 staged 的內容）：`PY_IMAGE=<smoke 專屬 tag> scripts/smoke-replay-offline.sh`（⛔ 不經 `REPLAY_SMOKE=1 python/scripts/test.sh`——那會把整套重跑）。⚠️ smoke 會以 clone 的內容**重新 build** 這個 tag：記下它的 image ID，照實寫成「同一個 staged tree、另一個 tag 重新 build」（⛔ 不宣稱與步驟 1 是同一個 image）；tag 依「三」第 4 步的收尾刪除。跑之前確認磁碟空間 |
| 4 | `scripts/make-i074-tooling-patch.sh --verify "$(git write-tree)"`（以 **staged tree** 驗；⛔ 不是 HEAD 或工作樹）——預期⛔ 沒有變化 |
| 5：最後一次文件修改之後 | `python3 -B scripts/check-doc-refs.py` 必須是「問題 0」（步驟 1 已跑過一次；這一次是文件改完之後的最終守門）；`git diff --cached --check` 乾淨 |
| 6：禁止標記的否定詞掃描 | 見下方「否定詞掃描」的完整程式；成功條件：結束碼 0 |

**shard 的證明**（分段之前；存成檔案、⚠️ **以 bash 執行**——zsh 沒有 `shopt`；`set -euo pipefail`、暫存檔在 `mktemp -d` 且由 trap 清除、⛔ 不在 repo 留檔；入口失敗或抽不到 node id 都失敗）。2026-10-07 實際執行過：完整入口 2270 個 node id、三段 540／767／963，兩兩互斥、聯集成立；入口失敗（不存在的路徑）與 glob 沒有符合都以結束碼 1 失敗：

```bash
#!/usr/bin/env bash
# shard 的證明（「五」步驟 2）：三段兩兩互斥、聯集 ＝ 完整入口。⚠️ 以 bash 執行。用法：bash shard_proof.sh <repo 或隔離 clone>
set -euo pipefail
cd "$1"
W=$(mktemp -d); trap 'rm -rf -- "$W"' EXIT
collect() {  # $1＝輸出檔；其餘＝pytest 的路徑。入口失敗或抽不到任何 node id → 失敗（⛔ 不放行）
  local out="$1"; shift
  if ! SKIP_SHELL_TESTS=1 python/scripts/test.sh --collect-only -q "$@" > "$W/raw" 2> "$W/err"; then
    tail -20 "$W/err" >&2; echo "收集失敗：$*" >&2; return 1
  fi
  grep -E '^(backtest|tests)/[^[:space:]]*::' "$W/raw" | LC_ALL=C sort > "$out"
}
# glob 在 python/ 底下展開（＝ 容器的 /app）；沒有符合 → failglob 讓它失敗。路徑⛔ 不含空白，展開後不加引號傳入
s1=$(cd python && shopt -s failglob && echo backtest/modular/tests tests backtest/modular/sr_scoring/tests/test_[a-h]*.py)
s2=$(cd python && shopt -s failglob && echo backtest/modular/sr_scoring/tests/test_i*.py)
s3=$(cd python && shopt -s failglob && echo backtest/modular/sr_scoring/tests/test_[j-z]*.py)
collect "$W/full" backtest/ tests/
collect "$W/s1" $s1
collect "$W/s2" $s2
collect "$W/s3" $s3
wc -l < "$W/full" | xargs echo "完整入口："; for s in s1 s2 s3; do wc -l < "$W/$s" | xargs echo "$s："; done
[ -z "$(LC_ALL=C comm -12 "$W/s1" "$W/s2")$(LC_ALL=C comm -12 "$W/s1" "$W/s3")$(LC_ALL=C comm -12 "$W/s2" "$W/s3")" ] \
  || { echo "shard 之間有重疊" >&2; exit 1; }
LC_ALL=C sort -m "$W/s1" "$W/s2" "$W/s3" | cmp -s - "$W/full" || { echo "shard 的聯集 ≠ 完整入口" >&2; exit 1; }
echo "shard 證明：成立"
```

**否定詞掃描**（第三輪 review：由 Python 自己呼叫 `git diff --cached -U0` 並檢查結束碼——⛔ 不會在 git 失敗時讀到空輸入而放行；存成檔案，在 repo 根目錄以 `python3 <檔案>` 執行；結束碼 0 ＝ 沒有命中、1 ＝ 有命中、2 ＝ git 失敗）。2026-10-07 實際執行過三種情況：

```python
"""禁止標記（U+26D4）的否定詞掃描：只看 `git diff --cached -U0` 的新增行。結束碼：0 ＝ 沒有命中、1 ＝ 有命中、2 ＝ git 失敗。"""
import re
import subprocess
import sys

proc = subprocess.run(["git", "diff", "--cached", "-U0"], capture_output=True)
if proc.returncode != 0:
    sys.stderr.write(proc.stderr.decode("utf-8", "replace"))
    sys.exit(2)
text = proc.stdout.decode("utf-8")
neg = re.compile(r"(?:\*\*)?(?:不|沒|無|非|別|未|勿|禁|排除分支)")
hits = [line for line in text.splitlines() if line.startswith("+") and not line.startswith("+++")
        for m in re.finditer(r"\u26d4\s*(\S{0,4})", line) if not neg.match(m.group(1))]
for line in hits:
    print(line)
sys.exit(1 if hits else 0)
```

##### 六、殘留的驗收條件（第一輪 review 改寫）

| 對象 | 條件 |
|---|---|
| git worktree | 全量執行前後各存一份完整的 `git worktree list --porcelain`（⛔ 不能只記數量）；**本輪新增的測試⛔ 不得增加** I-118 既有以外的登記；既知的增量照實記錄（步驟 1 的 `test-replay-args.sh` 跑一次 ＋4）；smoke 與反向驗證都在隔離 clone 裡，它們的 worktree 登記在 clone 自己的 `.git`，真正 repo 預期⛔ 沒有增量；⛔ 不清理、⛔ 不 prune 開跑之前既有的登記 |
| `/run/lock` | 固定的鎖檔 `/run/lock/i074-stage2.lock` 依契約永久保留（⛔ 不 unlink）——若存在，前後的 owner、mode、inode 不變，且以 `/proc/locks` 確認⛔ 沒有人持有它（⛔ 不對它 `flock`）；sentinel `/run/lock/i074-stage2.active` 必須**不存在** |
| 本輪零新增 | `/dev/shm/i074-*`；本 session 的測試程序（含「四」的 token 掃描）；**容器**：前後各存一份完整的 `docker ps -aq --no-trunc`（含已停止的），結束後逐一審核新增的 ID——smoke 有不具名、⛔ 沒有那個 label 的 `docker run --rm` 容器，所以⛔ 不能只看 `i074.stage2.run` 與 `i074sz-` 兩種命名（第二輪 review）；**image tag**：反向驗證與 smoke 的專屬 tag 結束後必須不存在（刪除前記下 image ID）——前後比對，⛔ 不得新增任何一個 |
| smoke | ⛔ 不宣稱「腳本自己全部清理」；它在隔離 clone 裡跑，殘留以 clone 為範圍檢查後連同 clone 刪除 |

##### 七、受影響檔案

| 檔案 | 改動 |
|---|---|
| `scripts/tests/test_i074_stage2_host.py` | 「四」的 cleanup helper、token 與它的測試 |
| 其他測試檔 | 依稽核結果補齊——清單在確認點 ② 列出、確認之後才動 |
| `docs/issue.md` | 本計畫、「⑧ 稽核結果」、補齊與反向驗證、全量執行的結果 |
| `docs/development-workflow.md` | I-074 Stage 2 一節加**測試類別與入口的索引**（哪一類 id 在哪個測試檔或 shell 段落、由哪個入口執行；⛔ 不放易漂移的完整方法名清單） |
| ⛔ 不動 | 產品程式、tooling 路徑、測試的入口與框架（需要時停在確認點 ②） |

##### 八、確認點

① 本計畫；② 稽核結果與補法（「四」）；③ 補齊與全量執行完成後 stage、⛔ 不 commit，停下來 review。

##### 九、風險

| 風險 | 對策 |
|---|---|
| 稽核量大（約 140 個 id、展開後更多），判定失準 | 每一列寫斷言的依據；「三」第 4 步的抽樣反向驗證；review 可以逐列抽查 |
| 「部分」的判準過寬或過嚴 | 以規格寫明的性質為準（⛔ 不自行加碼）；判定為部分時寫明少驗的那一點，由確認點 ② 一起裁決 |
| 反向驗證污染正式工作樹 | 一律在綁定 staged tree 的隔離 clone 注回；正式工作樹與 index 前後三項比對 |
| 隔離 clone 與正式 repo 的差異（例如 ignored 的本機檔案） | clone 只有 staged tree 的內容，正是要驗的；需要本機狀態的測試（若有）在稽核時照實標出 |
| 補測試需要改產品或測試框架 | ⛔ 不在 ⑧ 改；停在確認點 ② 回報 |
| 全量執行 OOM | 分段⛔ 不自動啟用：中止並回報證據，由你裁決；同意之後先跑 shard 的證明 |
| smoke 佔磁碟 | 跑之前確認空間；在隔離 clone 裡跑，結束後連同 clone 與專屬 tag 刪除 |

##### 十、決定（第一輪 review 的裁決）

| # | 決定 |
|---|---|
| 1 | 稽核範圍是「二」的三個 namespace；**移交到 ⑦ 的案例照樣稽核實際的涵蓋**，⛔ 不標不適用 |
| 2 | 以「性質是否被斷言」判定，並**展開成原子 subcase** |
| 3 | 稽核結果與補法先停下確認（確認點 ②） |
| 4 | 加跑 smoke：在 staged-tree 隔離 clone 裡直接執行 `scripts/smoke-replay-offline.sh`（專屬 tag、照實記錄重新 build 的 image ID） |
| 5 | `development-workflow.md` 只放穩定的測試類別與入口的索引 |
| 6 | 測試衛生納入 ⑧：獨立 helper、固定點重掃、失敗即紅 |

##### 第一輪 review 的修正（2026-10-07）

| 嚴重度 | review 的發現 | 查證 | 修正 |
|---|---|---|---|
| 高 | **殘留的驗收條件與 I-118 的已知現況矛盾**：v1 要求全量執行前後⛔ 沒有 worktree 殘留、smoke「腳本自己清理」，但 I-118 已確認 `test-replay-args.sh` 每跑一次 ＋4、smoke 的一般路徑 ＋2，repo 目前已有 377 筆登記；鎖檔依契約永久保留、只有 sentinel 應該消失 | ✅ 成立（`git worktree list` 377 筆；I-118 的事實段；supervisor 只 unlink sentinel） | 「六」改寫：worktree 存完整 inventory、本輪新增的測試⛔ 不得增加既有以外的增量、既知增量照實記錄、⛔ 不清理或 prune 既有登記；鎖檔若存在只驗屬性與⛔ 沒有人持有（`/proc/locks`）、sentinel 必須不存在；`/dev/shm`、測試程序與容器才要求零新增；smoke ⛔ 不再宣稱自己全部清理 |
| 高 | **反向驗證⛔ 不得直接在 staged 的工作樹注回缺陷**：單檔 SHA 相同證明不了 index、mode、symlink、未追蹤檔沒有漂移，中斷或 OOM 時也可能沒還原 | ✅ 成立 | 「三」第 4 步：一律在綁定 `git write-tree` 的隔離環境（`git archive` ＋ `git init`，驗 tree ＝ staged tree；⛔ 不用 `git worktree add`）注回；專屬的 `PY_IMAGE` tag；正式工作樹與 index 全程只讀，前後比對 index tree、`git status --porcelain=v2`、每個檔案的 mode 與 SHA |
| 中 | **一個 id 一列的粒度不足**：n7b、n7、n11、③ 的 w／ah／bd 等一個 id 內含大量「各一支」，部分存在就可能誤標涵蓋；「移交到 ⑦」⛔ 不等於不適用 | ✅ 成立 | 「二」：稽核表以 namespace ＋ id ＋ subcase 為一列，同時統計頂層 id 與 subcase；移交到 ⑦ 的案例照樣稽核；「➖」只留給真正撤回的契約 |
| 中 | **程序洩漏的 teardown 描述仍可能漏抓**：掃描期間仍可能 fork、子孫不一定帶暫存路徑、cleanup 失敗被吞掉仍會假綠、teardown 之後原測試難以斷言效果 | ✅ 成立（現行 cleanup 只殺 `self.pids`） | 「四」：唯一 token 帶在每一個長存程序的 argv、同 UID、PID ＋ starttime、排除自身與祖先、TERM → KILL → 重掃到固定點、逾時或讀不到身分即測試失敗、獨立於產品函式、冪等 helper 在測試內手動呼叫並斷言、teardown 再呼叫一次 |
| 中 | **OOM 的退路與 smoke 的命令沒有封閉**：`REPLAY_SMOKE=1 python/scripts/test.sh` 會把整套重跑；分段沒寫死、可能掩蓋一般失敗、只加總 passed | ✅ 成立（讀 `python/scripts/test.sh`） | 「五」：smoke 直接執行 `scripts/smoke-replay-offline.sh`（同一個 image；之前斷言工作樹 ＝ index）；分段只在 rc＝137 ＋ OOM 證據時使用、三段寫死且以 `--collect-only` 證明互斥與聯集、`SKIP_SHELL_TESTS=1`、記錄 skipped／xfailed／xpassed／deselected |
| 中 | **命令與受影響檔案沒有封閉**：tooling `--verify` 要以 staged tree；否定詞掃描沒寫工具、pattern、範圍與成功條件；`check-doc-refs` 的最後一次要說明用途；要動測試入口時的範圍 | ✅ 成立 | 「五」寫死每一個命令；「七」新增受影響檔案表；「一」「四」：要動 `python/scripts/test.sh` 或 shell 框架時停在確認點 ② |

裁決：#1～#6 依 review 的意見修正後同意（見「十」）。

##### 第二輪 review 的修正（2026-10-07）

| 嚴重度 | review 的發現 | 查證 | 修正 |
|---|---|---|---|
| 高 | **反向驗證的隔離 repo 無法保證既有 shell 測試可執行**：`git archive` ＋ `git init` 沒有歷史，而 `scripts/test-replay-args.sh` 的 ⑦a 段會在 `e1cbbbd` 建 worktree；兩支 shell 腳本也沒有依 id 執行單一段落的公開入口 | ✅ 成立（讀碼） | 「三」第 4 步：`git clone --no-hardlinks`（保留歷史）→ 刪掉已追蹤檔 → `git archive "$T"` 覆蓋 → `git add -A` → 暫存 commit → 斷言 `HEAD^{tree}` ＝ `T`；pytest 寫精確 node id；shell 在 clone 裡跑完整的官方腳本，要新增 selector 時停在確認點 ② |
| 高 | **smoke 的前置條件證明不了「工作樹 ＝ index」**：`git diff --quiet` 抓不到未追蹤與 ignored 的檔案，smoke 卻以 `COPY . .` 建 image、以暫存 index `git add -A python` 組 tooling patch；smoke 自己會重新 build tag，「同一個 image」的說法不精確 | ✅ 成立（`python/Dockerfile` 的 `COPY . .`、`.dockerignore` 只排除少數樣式；smoke 第 66 行 `docker build`） | 「五」步驟 3：smoke 改在 staged-tree 隔離 clone 裡跑（只有 staged 的內容）、專屬 tag，照實寫成「同一個 staged tree、另一個 tag 重新 build」並記下 image ID |
| 中 | **OOM 退路的證據條件可能同時誤判**：寫死 `(pytest)`，但既有紀錄是 `(python)`；沒限定本次新增，舊訊息可能被誤認；`dmesg -T` 的時間不可信、ring buffer 可能沖掉 | ✅ 成立（本筆既有的紀錄是 `(python)`） | 「五」步驟 2：開跑前記 kernel log 游標（單調時間戳）、只看之後新增的行、程序名稱是 Python 的程序（⛔ 不寫死某一個）、`anon-rss` 對上 `mem=` 上限；游標被沖掉或證據不足 → 中止回報、⛔ 不分段 |
| 中 | **殘留的驗收漏掉 smoke 的容器與專屬 image tag**：smoke 有不具名、沒有 label 的 `docker run --rm` 容器；專屬 `PY_IMAGE` tag 沒有規定刪除 | ✅ 成立（smoke 第 72 行起） | 「六」：容器改成前後完整的 `docker ps -aq --no-trunc` 並審核每個新增 ID；專屬 tag 在成功、失敗、中斷都以 trap 刪除、記下 image ID、結束後驗證不存在 |
| 中 | **cleanup helper 把 `/proc` 的正常消失競爭當成失敗** | ✅ 成立 | 「四」：讀取失敗且 `/proc/<pid>` 已不存在 → 視為已消失；只有 PID 仍存在而身分讀不到、解析失敗、確認不了 starttime 才失敗；送訊號之前重新比對 PID ＋ starttime |
| 中 | **命令仍未完全封閉**：否定詞掃描只寫「與前幾包相同的那段 Python」，pattern 用了全形 `｜`、`(**)?` 也⛔ 不是可執行的 regex；collect-only 沒寫如何透過同一個 image 執行與比對 | ✅ 成立 | 「五」：寫出可直接複製的完整命令——否定詞掃描（禁止標記 ＝ U+26D4、UTF-8、真正的 regex、結束碼 0／1）與收集的比對（官方入口 `SKIP_SHELL_TESTS=1 python/scripts/test.sh --collect-only -q`、抽 node id、`LC_ALL=C sort`、兩兩 `comm -12` 為空、`sort -m` 的聯集 `cmp` 完整入口） |

##### 第三輪 review 的修正（2026-10-07）

| 嚴重度 | review 的發現 | 查證 | 修正 |
|---|---|---|---|
| 中 | **兩段「封閉命令」仍可能 fail-open**：`collect()` 是 pipeline 卻沒有 `pipefail`、又丟掉 stderr，入口失敗時可能只回傳最後一個 `sort` 的成功；否定詞掃描在 `git diff` 失敗時讀到空輸入也回 0；收集命令用了 bash 專屬的 `shopt` 卻沒有明定以 bash 執行（zsh 實測 `command not found: shopt`）；輸出檔留在 repo 根目錄 | ✅ 成立 | 兩段都改成完整的程式並實際執行過：shard 的證明是 bash 腳本（`set -euo pipefail`、`mktemp -d` ＋ trap、入口失敗時印出 stderr 並失敗、抽不到 node id 也失敗；正常、入口失敗、glob 沒有符合三種情況都驗過）；否定詞掃描由 Python 自己呼叫 `git diff` 並檢查結束碼（0／1／2 三種情況都驗過） |
| 中 | **OOM 證據仍無法決定性地綁定失敗的 pytest container**：`anon-rss` 是被殺程序的、⛔ 不是 container 的總用量；live 上另一個 Python 程序同時 OOM 也可能碰巧符合——「對上那一個容器」仍是推論 | ✅ 成立（入口以不具名的 `docker run --rm` 執行；這台 kernel 的 OOM 訊息⛔ 不帶 memcg） | 「五」步驟 2：分段⛔ 不自動啟用——pytest 以 137 結束時中止並回報證據，由你裁決；擴充入口取得 container 身分屬入口的改動，要做就在確認點 ② 另行提出 |
| 低 | **隔離 clone 的暫存 commit 依賴 host 的全域 git 設定**：沒有指定 `user.email`，global／template 的 hook 也可能介入；刪除已追蹤檔沒有處理空輸入與 `-` 開頭的路徑 | ✅ 成立 | 「三」第 4 步改成完整的 bash 腳本：`clone --template=` ＋ `core.hooksPath=/dev/null`、以環境變數固定 author／committer、`commit-tree` ＋ `update-ref` 建暫存 commit、`xargs -0 -r rm -f --`；實際執行過 |
#### I-074 Stage 2 計畫書 v29（2026-09-29，步驟 ⑥，✅ **已確認**（2026-09-29，review 通過並 commit））

⚠️ **v29 是執行順序的 ⑥「更新計畫並再次確認」**：併入 ⑤ 的裁定（`P_B` 與 `M_safety`），並把計畫書與 ③b～⑤ 之後的
程式碼現況逐項對照。v28 的其餘內容不變；⛔ **v29 確認前，⛔ 不得進入 ⑦**（「二之二」：更新計畫並經確認後，才實作
正式 preflight）。

**使用者裁決（2026-09-29，⚠️ 在寫進計畫書之前做成）**：

| # | 決策 | 結果 |
|---|---|---|
| 1 | preflight 裡的 `P_B` 怎麼進程式 | ✅ **寫死預算常數 `P_B_BUDGET` = 167,772,160 bytes（160 MiB）**——⛔ 不寫死實測值、⛔ 門檻⛔ 不取自執行期讀到的報告（⚠️ 第三輪 review 加的 freeze record 是確認重跑的**信任錨**，其中的 `P_B` 只用來驗 ≤ `P_B_BUDGET`，⛔ 不當門檻，見「八之一」） |
| 2 | ⑤ 的確認重跑 | ✅ **只在 ⑩ 之前一次**（⑨ 之後的最後一次 commit 之後；⚠️ 第五輪：⑩ 的隔離複本就釘在這一次的 OID——freeze record 的 `repo_head`——真正 repo ⛔ 不再凍結，見「八之一」） |
| 3 | 殘留的「⚠️ 待確認」標記 | ✅ **全部改成「已隨 v28／③ v12 確認（2026-09-23）」**——依據是 v28 的裁決表，以及 ③b／③d 已照著實作並通過 review |
| 4 | ⑩ 執行期間「工具版本與 HEAD worktree 不被改動」的保證方式（第五輪 review 之後提出） | ✅ **隔離複本執行**（2026-09-29）：⑩ 在 repo 外、釘在 freeze record OID 的 `git clone --no-hardlinks` 複本執行，證據經「晉升」進真正 repo；⛔ 撤回第二～四輪的凍結檢查機制（⛔ 不採「程序性凍結加最小保護」、⛔ 不採「補完整套機制」） |
| 5 | supervisor 被 SIGKILL 時是否保證「唯一一次正式趟」（第十三輪 review 之後提出） | ✅ **保證：加 active-run sentinel**（2026-09-29）——supervisor 被 SIGKILL 時 sentinel 留下，新的 supervisor 一律 fail-closed，待人工確認殘留已清除才解除（⛔ 不採「明定 SIGKILL 下不保證」）。⚠️ **第十五輪由第 9 列取代**：⛔ 沒有解除入口，sentinel 只在正常釋放或重開機時消失——「待人工確認後解除」撤回 |
| 6 | 鎖與串行化的範圍 | ✅ **整台 host**（2026-09-29）：鎖、sentinel 與殘留檢查都是 host 層級（⛔ 不以 repo 為範圍）。⚠️ 第十四輪：原本放在 XDG 路徑，只能保證「同一帳號、同一 XDG root」——改放 **`/run/lock`**（1777 的 tmpfs，任何帳號都能建檔）才是真正的整台 host。⚠️ **第十五輪由第 9 列取代**：範圍降為固定的執行帳號 |
| 7 | supervisor 的檔名、label 的注入者、argv 與測試落點等實作細節 | ✅ **移到 ⑦ 的實作計畫書**（2026-09-29）：v29 只定**不變條件、驗收條件與已裁定的機制**，⑦ 的計畫書以它們為驗收標準——符合 ⑥ 開始時確認的範圍（⑥ 只更新 Stage 2 計畫書，⑦ 的實作計畫書進 ⑦ 時另寫） |
| 8 | supervisor 被 SIGKILL 之後，sentinel 靠什麼解除（第十四輪 review 之後提出；⚠️ 這台 host 同時跑著 live 服務） | ✅ **token 繼承 ＋ 掃 `/proc`**（2026-09-29）：所有後代繼承 supervisor 產生的 token；解除入口取同一把鎖、嚴格解析 sentinel，確認 supervisor（pid ＋ starttime ＋ boot ID）已不在、同帳號沒有任何帶 token 的程序、沒有 Stage 2 label 的容器，三者都成立才解除；重開過機則免掃程序（⛔ 不採「只接受重開機」——會中斷 live 服務）。⚠️ **第十五輪由第 9 列取代**（單次列舉 `/proc` 有 fork／exit 的 TOCTOU） |
| 9 | 沒有 root 就無法結構性地保證「SIGKILL 下的唯一正式趟」與「跨帳號的鎖」，要怎麼處理（第十五輪 review 之後提出；⚠️ 實查：`dev` 不能建立 cgroup、沒有 systemd user manager、`sudo` 需要密碼） | ✅ **固定執行帳號 ＋ 重開機才解除**（2026-09-29）：範圍是固定的執行帳號 `dev`；supervisor 被 SIGKILL 之後⛔ 沒有解除入口，sentinel（tmpfs）只在正常釋放或重開機時消失；使用者接受重開機會中斷 live 服務（⛔ 不採「一次性 root 設定」、⛔ 不採「降級成人工確認解除」） |

| # | 缺口／漂移 | v29 的處置 | 同步的節 |
|---|---|---|---|
| 1 | ⛔ **preflight 的呼叫順序⛔ 沒有磁碟檢查**：v28 的公式 `required = P_B ＋ M_safety` 只寫在「二之二」，⛔ 沒寫它在哪一步、量哪個檔案系統、`P_B` 怎麼進程式 | ⚠️ 補成 preflight 的一步，規格見下方「磁碟檢查（v29）」；③ evidence contract「七之三」的呼叫順序同步加一列 | 二之二、六 1、六 2（n1～n6）、③ 的七之三 |
| 2 | ⚠️ **`P_B` 會隨 HEAD 變大**：`P_B` 含一份 HEAD 的完整 worktree（⑤ 實測 36.0 MiB），⑦～⑨ 的每一次 commit 都讓它變大；寫死實測值又沿用「實際峰值 > `P_B` 就回退」會幾乎必然觸發，而更新常數本身又是一次 commit（循環） | ⚠️ 改成**預算常數**：`P_B_BUDGET` = 160 MiB（比 ⑤ 的實測高 9.2 MiB，約 6%），吸收 HEAD 變大、實際 before source 與合成品的尺寸差、量測波動（④～⑤ 共六次 ≤ 0.3 MiB）；回退的比較對象改成預算 | 二之二、六 1、八 |
| 3 | ⛔ **「二之二」要求「見證趟之前、before 正式趟之前各檢查一次」，與時序自相矛盾**：見證趟（③c）排在 ⑤ 之前，而 v9 規定「`M_safety` 定案（＝⑤）之前⛔ 不得實作 preflight」——見證趟之前那一次**⛔ 不可能做到，實際也沒有做** | ⚠️ 照實訂正：③c 已於 2026-09-23 成功完成（`envcheck/` 已 durable），⛔ 不補做；**只剩 ⑩ 之前那一次** | 二之二 |
| 4 | ④ 計畫書「七」指定「五」的 ⑦ 回退順序在 ⑥ 併入「六、1」 | ✅ 併入，比較對象改成 `P_B_BUDGET`（第 2 列） | 六 1、④ 計畫書的五、七 |
| 5 | ⚠️ **⑤ 的記憶體只涵蓋證據層程序**，⛔ 不含 replay 程序；「六、1」的「現行實測 524 MiB」容易被誤讀成 ⑤ 之後已過時 | ⚠️ 「六、1」補註：⑤ 量到的是 d～i 裡的證據層程序（最高 342.0 MiB，sizing 階段的觀察值）；**正式驗收是 ⑨-1 的 memory／disk acceptance（⑦ 只做 memory harness 的實作與開發驗證）：replay 程序（涵蓋 a～c）及 d～i 各獨立程序，皆各自 < 450 MiB**；524 MiB 是 replay 程序改成串流之前的峰值 | 六 1 |
| 6 | ⚠️ **「八」列給 ⑦ 的項目有一部分已在 ③ 做完**（發布、recovery、failed-record 的 check、共用串流讀取、信任錨、「①之三」的 `CounterfactualEffectCheck`） | ⚠️ ⑦ 的範圍改成**實際剩下的項目**（見「八」）；⚠️ replay 端的「①之三」守門**重用** ③d 的 `CounterfactualEffectCheck`，⛔ 不另寫一份（兩套推導＝兩套語意） | 八 |
| 7 | ⚠️ **行號漂移**（③b 改動之後；都是「識別符緊鄰行號」的格式，但檢查容許 ±15 行，所以⛔ 沒被抓到） | ✅ 更新：`replay_args_offline()` 64 → 65、`replay_args_prepare_worktree()` 148 → 154、`I074_MODE` 62 → 65、`TOOLING_PATCH` 73 → 86、`TOOLING_PATCH_SHA256` 166 → 180、`validate_provenance()` 200 → 209；`evaluation.py`／`decision_engine.py` 的引用仍成立 | 二④、三、三之一之二、v19／v20 的紀錄、② 執行結果的「⒝ 兩份 patch 的 SHA 機制」 |
| 8 | ⚠️ v26～v27、③ v10、計次裁決節與關閉條件還留著 22 處「⚠️ 待確認」（⚠️ 初版漏了關閉條件的「v26 補充」，review 指出） | ✅ 依決策 3 改成已確認 | 各處 |

**磁碟檢查（v29）**：

| 項目 | 規格 |
|---|---|
| 常數 | `P_B_BUDGET` = **167,772,160 bytes（160 MiB）**、`M_safety` = **1,073,741,824 bytes（1 GiB）**，兩者都寫死在程式裡、由測試釘住值，⛔ 不開 CLI、⛔ 不得執行期解讀；`required` = **1,241,513,984 bytes（1,184 MiB）** |
| 位置 | **複本內** orchestrator 的 preflight（複本已建立之後），**failed-record lookup 之後、replay 之前**——離實際用到空間的時間最近 |
| 量法 | `statvfs` 的 `f_bavail × f_frsize`（一般使用者可用的空間，⛔ 不用 `f_bfree`） |
| 前提（fail-closed） | run 目錄、worktree 暫存目錄、**複本的** `python/baselines/i074_stage2/`、**複本的** `.git`、Docker Root Dir 必須在**同一個裝置**（`st_dev` 相同）——`P_B` 是在「全部同一個檔案系統」下量的（⑤：L0～L5 與 Docker Root Dir 同一個裝置）；⛔ 不同就中止，要回頭重新 sizing |
| 失敗 | 結束碼 1、⛔ 在 replay 之前中止；比照「preflight 與指紋檢查失敗⛔ 不計入這一次」，⛔ 不計入正式 scan |
| 預算的守門 | ⚠️ **⑨ 之後的正式 memory／disk acceptance**（用封存的 exact patch SHA，見「八」的 ⑨-1）量到的實際流程磁碟峰值（⚠️ **量法必須與 `P_B` 相同**，見「六、1」）、以及 ⑩ 之前的 `--formal` 確認重跑（⑨-2），**都必須 ≤ `P_B_BUDGET`**（重跑還要 `status = "ok"`）；⛔ 超過就走「六、1」的回退順序 |
| ⚠️ 隔離執行 | ⑩ 在釘住 freeze record OID 的隔離複本執行，真正 repo ⛔ 不凍結；複本完整性、晉升與撤回的項目見「八之一」～「八之三」 |
| 測試 | 「六、2」的 n1～n12（進入複本、複本完整性、真正 repo 不凍結、freeze record、晉升與 failed record 的重跑見 n7～n12）、「六、9」的 m |
| ⚠️ 見證趟 | ⛔ 不適用（③c 已完成，見上表第 3 列） |

**v29 第一輪 review 的修正（2026-09-29）**：

| # | 問題 | 修正 |
|---|---|---|
| 中 | ⛔ **確認重跑之後允許 `docs/` commit，會破壞預算的證明**——`P_B` 含完整的 HEAD worktree，文件也在裡面；確認重跑若得到 159.9 MiB，之後 0.2 MiB 的文件 commit 就讓實際峰值超過預算，卻依規則不重跑 | ✅ 採「禁止任何 commit」：從確認重跑到**窗口 B 結束**，HEAD 凍結（⚠️ 延伸到窗口 B 結束，是因為 finalizer 在 finalize 當下才從 HEAD 開 worktree）；期間的紀錄只寫在工作樹、窗口 B 結束後才 commit；⑩ 的執行紀錄以 `git rev-parse HEAD` 對照確認重跑報告的 `repo_head`（「八之一」「八」） |
| 中 | ⛔ **⑦ 的「實際流程磁碟峰值」沒有定義量法**，也沒有 preflight 磁碟檢查的專屬測試 | ✅ 「六、1」寫死：重用 `i074_stage2_sizing.py` 的量測原語，路徑起訖、L0～L5、allocated bytes、baseline 增量、`max(dirs_peak, fs_peak, accounted)`、取樣與容器足跡模型全部沿用 ④「二」，⛔ 不得改用 `du` 或只看輸出目錄；「六、2」補 n1～n6（邊界 `==`／`−1`、`f_bavail` 與 `f_bfree`、`st_dev` 不同、讀取失敗 fail-closed、拒絕時 replay 未被呼叫、常數與算術釘死） |
| 中 | ⛔ **現行公式與回退規則同時留著 `P_B` 與 `P_B_BUDGET` 兩套語意**（兩者差 9,670,656 bytes）；④「五」的回退內容仍寫「更新 `P_B`／`M_safety`」 | ✅ 現行政策一律改成 `P_B_BUDGET`：「二之二」的事前檢查與公式、「六、1」的公式區塊與 memory harness 的比較對象、④「五」的回退內容（更新 `P_B_BUDGET`，`M_safety` 原則上維持）；⑤ 的量測區保留 `P_B` 的歷史算式並加註現行公式。⚠️ `development-workflow.md` 的 sizing 一節仍是 ④ 的規則，**v29 確認後、隨 ⑦ 的歸檔改寫**（⛔ 未確認的規則不先寫進主題文件） |
| 低 | ⛔ 宣稱已清完「待確認」，關閉條件的「v26 補充」仍留著一處 | ✅ 改成已確認；第 8 列的數量訂正為 22 |

**v29 第二輪 review 的修正（2026-09-29）**：

| # | 問題 | 修正 |
|---|---|---|
| 中 | ⛔ **凍結只鎖 HEAD、⛔ 沒鎖實際執行的檔案**：期間仍可修改未提交的 `python/`、`scripts/`、`.gitattributes`，runner／orchestrator／finalizer 會在 HEAD 不變的情況下執行到 dirty 內容；HEAD 不符也只記為「程序偏離」，與容量證明不一致 | ✅ 「八之一」改寫成表：除了⛔ 任何 commit，`python/`、`scripts/`、`.gitattributes` 也⛔ 不得有未提交的修改或未追蹤檔；orchestrator 以明示參數接收確認重跑的 `repo_head`（⚠️ **第三輪改成 freeze record**，見下表），在 **preflight、finalize／publish 之前、recovery 之前（recovery 一律經 orchestrator）、窗口 B 結束時**各驗一次。replay 之前不符 → fail-closed（⛔ 不計入正式 scan）；**replay 開始之後不符 → 該趟證據⛔ 不得進入 B／C 判讀**，能否重跑⛔ 不預先放寬（現行計次只允許 patch 失效後重跑），停下另立 issue。已知限制照實寫明（檢查點之間改了又還原驗不出來；ignored 檔不在範圍內）。③「七之三」的 preflight 順序加「凍結檢查」一列；「六、2」補 n7、n8 |
| 中 | ⛔ **memory harness 的 failure 路徑會污染正式的 failed-attempt 守門**：實際走 rc=6 → `--publish-failed-record`，而量測位置直接列出正式的 `python/baselines/i074_stage2/`——failed record 以正式 identity ＋ patch SHA 發布後，會永久擋住同 SHA 的 ⑩ | ✅ 「六、1」的量法表加「隔離」：在 repo 外的 `git clone --no-hardlinks` 複本執行（沿用 ④），用複本自己的 baseline；⚠️ 仍用正式的 identity 與 patch SHA（⛔ 不另造 harness 專用 SHA，否則驗不到完整組合）；failure 路徑排在 success 之後或各用獨立複本；結束後斷言真正 repo 的完整 inventory 與 Stage 2 identity 檔⛔ 完全未變 |
| 低 | 「a～i 每個程序」用詞不精確——a～c 是 replay 程序內的資料量與路徑條件，⛔ 不是三個獨立程序 | ✅ 改成「replay 程序（涵蓋 a～c）及 d～i 各獨立程序，皆各自 < 450 MiB」（「六、1」、第 5 列、「八」） |

**v29 第三輪 review 的修正（2026-09-29）**：

| # | 問題 | 修正 |
|---|---|---|
| 中 | ⛔ **凍結用的 `repo_head` 仍是可自行宣告的值**：操作者抄入、orchestrator 只比對「HEAD ＝ 參數」；誤抄成目前的 HEAD 時，中間有 commit 也會通過 | ✅ 改成 **freeze record**：`--formal` sizing 成功時輸出 canonical 的 `freeze_record.json`（`mode`、`status`、`P_B`、`repo_head`、報告 SHA、harness／shim／helper 與 counterfactual patch 的 SHA、image 與 identity 檔的 SHA）；orchestrator 只接受它的**路徑**、逐項驗證，並在每個檢查點確認它沒被換掉；⛔ 不再有裸 `repo_head` 的入口。門檻仍是寫死的 `P_B_BUDGET`，freeze record 只是信任錨（決策 1 那一列的文字同步寫清楚這個區別）。測試 n7 擴充、新增 n10 |
| 中 | ⛔ **memory harness 與 exact patch 的順序沒有封閉**：harness 要用正式 patch SHA，卻排在 ⑦，而 exact patch 到 ⑨ 才封存——⑦ 先驗收、⑨ 再改 patch，驗到的就不是正式 SHA | ✅ 「八」改成：⑦ 只做 memory harness 的**實作與開發驗證**；⑨ 封存 exact patch；**⑨-1 正式 memory／disk acceptance**（用封存的 exact patch SHA）；**⑨-2 ⑤ 的確認重跑**（產出 freeze record，從此凍結）。exact patch 的 bytes 或 SHA 之後只要再變，⑨-1 就要重跑；「六、1」的回退順序與「磁碟檢查」表同步改指 ⑨-1／⑨-2 |
| 低 | ⛔ **第四個檢查點（窗口 B 結束時）沒有測試** | ✅ 「八之一」加**窗口結束紀錄**（run 目錄、⛔ 不進 archive；`ok`／`violated`——⚠️ 第四輪改成 durable 契約，見下一表），**B／C 判讀器必須先驗到綁定該 archive 的 `ok` 紀錄才判讀**——「不得進入 B／C 判讀」由程式強制；新增 n9（發布後才製造 HEAD 或工作樹差異，成功 archive／failed record／recovery 之後三種終態各至少一支：紀錄 `violated`、artifact ⛔ 未被刪改、判讀器拒絕）與判讀器的測試 m |

**v29 第四輪 review 的修正（2026-09-29）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | ⛔ **窗口結束紀錄成為 B／C 的必要證據，卻沒有 durability／recovery 契約**：只放在 run 目錄、⛔ 沒有原子寫入、fsync、失敗的結束碼、recovery 與保存位置——唯一一次正式 replay 成功、archive 也已 durable，可能因最後一個小檔寫不成而永久無法判讀；而且要求綁定的「manifest SHA」在 failed record ⛔ 不存在 | ✅ 新增「八之三」：永久位置 `python/baselines/i074_stage2/window_close/`（第五個根目錄，③「三之零」同步）；封閉 schema，`terminal_kind` 分 `success_archive`（綁 `evidence_manifest.json`）與 `failed_attempt`（綁 `failure_record.json`）；沿用 ③ 的發布原語做原子發布與 fsync；rename 之前失敗 rc=1、parent fsync 失敗 rc=3；`--recover-window-close` 只重驗既有 artifact、freeze record 與工作樹後補寫，⛔ 不 replay；**窗口 B 的終點改成「窗口結束紀錄已 durable」**；B／C gate 只適用 `success_archive`。測試 n9、n12、m 同步 |
| 中 | ⛔ **全文仍保留「正式驗收在 ⑦」的說法**，與 ⑨-1 相反；⑧ 還寫判讀器的 a～l | ✅ 第 5 列、「六、1」、「八」的末段（⑦ 只做實作與開發驗證、⑨-1 才是正式 capacity acceptance、⑩ 只額外記錄實際峰值、⛔ 不反向決定是否執行 ⑩）、⑧ 改成 a～m；④ 計畫書兩處與 ⑤ 的規則表加註 |
| 中 | ⛔ **freeze record 寫「封閉 schema」又寫「至少含」**；counterfactual patch 的 SHA 分不清是 raw bytes 還是套用後的 diff；腳本 SHA 可能被理解成 git blob OID | ✅ 新增「八之二」：完整欄位表（型別、hex64／oid40、值域）、交叉不變條件、unknown field 拒絕；**兩種 patch SHA 都存**且必須相等（③「四之一」第 1 條：封存的 patch bytes 就是 canonical diff）；腳本 SHA 明訂為「HEAD 中檔案內容的 SHA-256」、⛔ 不是 blob OID。測試 n7、n10 同步 |
| ⚠️ 自己發現 | ⛔ **第二輪的凍結範圍與 ⑩ 自己的輸出衝突**：規則寫「`python/` 不得有任何未追蹤檔」，但 ⑩ 本來就會把證據寫進 `python/baselines/i074_stage2/`（`evidence/`、`failed/`，以及本輪的 `window_close/`）——發布之後的檢查點會**必然不符**，規則無法執行 | ✅ 「八之一」的「禁止」列改成三條：(a) 已追蹤檔一律不得修改（含 `envcheck/`）；(b) `python/baselines/i074_stage2/` 以外不得有未追蹤檔；(c) 該目錄底下的未追蹤項目恰好是本趟預期的新增（依檢查點而定），多出任何其他項目即不符；run 目錄⛔ 不得在 `python/`、`scripts/` 底下。新增測試 n11 |
| ⚠️ 自己發現 | ⛔ **②（發布之前）不符時沒有終態 artifact**，窗口結束紀錄無從綁定；而且只要事後恢復 clean、再重跑 finalize，② 就會通過並發布——等於繞過「replay 期間曾經不乾淨」 | ✅ ② 不符時在 run 目錄（與它否決的 operational 輸出放在一起）寫 durable 的 **`freeze_violation` 標記**，orchestrator 之後任何 finalize／publish 重試一律拒絕；⛔ 不寫窗口結束紀錄（`violations` 的 checkpoint 因此⛔ 沒有 `pre_publish`）。測試 n8 同步 |

**v29 第五輪 review 的處置（2026-09-29）**：

⚠️ 第五輪 review 指出窗口結束紀錄的四個缺口——①`--recover-window-close` 用**事後**的狀態重算，會洗掉當下的 violation（或因逐位元
不一致而永久無法 recovery）；②紀錄可能綁定 durability 未確認、crash 後消失的終態；③`freeze_violation` 沒有 durable 契約；④freeze
record 原件沒有永久保存。四項都屬實。⚠️ 但它們都是同一個模式的結果：⑩ 的工具從**真正 repo 的工作樹**執行，只能事後偵測，每補一層
就多一個狀態機缺口——照建議補完，等於在 ⑥ 裡設計一套規模與 ③ 相當的新 evidence contract。於是先請使用者裁決方向（決策表第 4 列）：
✅ **隔離複本執行**。

| 項目 | 處置 |
|---|---|
| 設計 | ⑩ 在 repo 外、釘在 freeze record OID 的 `git clone --no-hardlinks` 複本執行（與 ④、⑨-1 同一套做法，也正是 `P_B` 量測的條件）；真正 repo ⛔ 不再凍結；證據經「**晉升**」（逐位元複製與比對、`rename_noreplace`、fsync，有自己的 rc 與 `--recover-promotion`——⚠️ 第八輪已撤回 `--recover-promotion`，改成單一、冪等的 `--promote`）進真正 repo——「八之一」「八之三」 |
| 第五輪的 ①② | ⛔ **前提消失**——窗口結束紀錄與它的 recovery 整個撤回；⛔ 沒有「重算歷史結果」的步驟，也⛔ 沒有綁定未 durable 終態的紀錄（晉升只接受複本內**已 durable** 的終態） |
| 第五輪的 ③ | ⛔ **前提消失**——`freeze_violation` 撤回：複本歸 orchestrator 專用、完整性檢查不符就⛔ 不發布、⛔ 不晉升，證據進不了真正 repo；⛔ 沒有「恢復 clean 之後重跑 finalize」的繞過路徑（重跑 finalize 同樣要過完整性檢查，而複本的 HEAD 與已追蹤檔被改過就過不了） |
| 第五輪的 ④ | ⛔ **前提消失**——freeze record ⛔ 不進證據鏈，⑩ 之後沒有步驟需要回頭驗它；它只決定複本釘哪個 OID，⑩ 的執行紀錄記下它的 SHA 與 `repo_head`（「八之二」） |
| 新增的規則 | 已晉升但還沒 commit 的 failed record 會讓下一次 ⑩ 的 lookup 看不到——複本內的 preflight 驗「真正 repo 的 `python/baselines/i074_stage2/` 底下⛔ 沒有未追蹤項目」；⚠️ **自己發現**：拿**舊的** freeze record（OID 早於 failed record 的 commit）同樣會讓 lookup 看不到——另驗「真正 repo HEAD 的 `failed/` 全部出現在複本的 `repo_head` 裡」（「八之一」）；晉升之前驗真正 repo 與複本的 Stage 1 錨點、`envcheck/` 逐位元相同（「八之三」；⚠️ **第六輪撤回**——那一步本身有 TOCTOU，改成晉升只用複本的錨點做完整驗證） |
| 撤回（⚠️ 第二～四輪修正表裡的對應處置因此作廢，表格保留為歷史） | 真正 repo 的 HEAD 凍結區間、工作樹範圍 (a)～(c)、四個檢查點、窗口結束紀錄（原「八之三」整節）、`freeze_violation`、第五個根目錄 `window_close/`（③「三之零」同步移除）、B／C 判讀器對窗口結束紀錄的前置條件 |
| 保留 | freeze record（「八之二」，改成「複本釘哪個 OID」的依據）、⑨-1／⑨-2 的順序、「六、1」的量法與隔離、n1～n6 |
| 測試 | n7～n12 與 m 依新設計改寫：進入複本與 preflight、複本完整性、**真正 repo 不凍結**（⑩ 期間在真正 repo commit 與編輯，產出的證據與對照組逐位元相同）、freeze record、晉升、failed record 的晉升與重跑、判讀前的完整驗證 |

**v29 第六輪 review 的修正（2026-09-29）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | ⛔ **③「七之三」的 preflight 表先跑信任錨與 failed-record checker，最後才驗複本完整性**（與總覽的順序相反）——複本裡的 orchestrator 或 Python validator 若被改過，會先被執行 | ✅ 表格重排：**第 0 列**是複本自身完整性，⛔ 只用 git 與 shell、⛔ 不 import repo 內的 Python 模組（`repo_head` 以 host 的標準庫解析），不符即中止、之後各列一律不執行；接著才是凍結 patch、freeze record 的完整驗證、信任錨、failed-record、磁碟。完整性另補「`python/baselines/i074_stage2/` 以外⛔ 沒有未追蹤檔」與「orchestrator 自身的內容 ＝ `repo_head` 中的版本」（「八之一」）。n7 改成斷言信任錨、failed-record checker、replay **都未被呼叫** |
| 高 | ⛔ **晉升只證明「目的地 ＝ 來源」**：複本內的 `evidence/`、`failed/` 是未追蹤的產物、⛔ 不受完整性檢查保護；終態在 finalizer 驗完之後、晉升之前損壞，晉升會忠實複製同一份壞資料 | ✅ 晉升在 rename **之前**對 staging 做與 recovery 相同強度的完整驗證（成功 archive：`verify_stage2_graph()` 全部各道 ＋ shell 端的合成守門；failed record：F1～F10），錨點一律取自複本；`--recover-promotion` 也改成先對目的地完整驗證。n11 補「終態 durable 之後、晉升之前竄改來源」（成功 archive 與 failed record 各一支：rc=8、目的地⛔ 不存在） |
| 中 | ⛔ **真正 repo 不凍結，與晉升的錨點前置檢查之間有 TOCTOU** | ✅ **晉升⛔ 不讀真正 repo 的錨點**（撤回第五輪的比對步驟）：archive 是否有效只取決於複本裡釘住的錨點，真正 repo 怎麼改都影響不了它。晉升之後真正 repo 的錨點若與 git 不同，是 repo 端的問題——它們是已 commit 的終態證據、archive 以 SHA 綁定，identity 封存在 archive 內（實查 `STAGE2_LAYOUT` 含 `identity/run_identity.json.gz`），一律從 git 還原、⛔ 不動 archive（⚠️ **第七輪改成判讀器直接從 `base_commit` 取錨點**，⛔ 不再需要還原），所以⛔ 不會出現「archive 已發布卻永久無法使用」。⚠️ 沒有採用 review 建議的「晉升期間凍結保護集合」——它只縮短 TOCTOU 的窗口，而這個做法讓窗口⛔ 不存在。n9 補「⑩ 期間改真正 repo 的錨點」：晉升不受影響；判讀器在錨點被改的期間拒絕，從 git 還原之後接受 |
| 中 | ⛔ **failed record 晉升成功與晉升操作失敗共用 rc=1**；`--recover-promotion` 的成功碼沒有定 | ✅ 兩者都做：晉升操作的錯誤一律 **`EXIT_PROMOTION_FAILED = 8`**（現有結束碼用到 1～5 與 7——2 是 `--check-failed-record` 的命中——6 已規劃給反事實失效）；成功才恢復終態原本的 0／1；`--recover-promotion` 成功同樣恢復 0／1。⚠️ 另外 preflight 中止也是 rc=1，所以 orchestrator 每次結束都寫 **`result.json`**（封閉 schema：`terminal_kind`、`promotion`、`destination`、`exit_code`），呼叫端⛔ 不得單看 rc=1 判定 failed record 已晉升。⚠️ **第七輪撤回 `result.json`**，改成端到端結束碼 ＋ 唯讀的 `--status`（⚠️ **第八輪再撤回 `--status`／`--recover-promotion`**，改成單一、冪等的 `--promote`）。「四」的結束碼列同步 |

**v29 第七輪 review 的修正（2026-09-29）**：

| # | 問題 | 修正 |
|---|---|---|
| 中 | ⛔ **`result.json` 成為 rc=1 的判讀依據，卻沒有自己的寫入與 recovery 契約**——最危險的形狀：晉升的 rename 與 parent fsync 都成功、摘要寫出之前中斷，一般的重試因目的地已存在而回 8 | ✅ **撤回 `result.json`**（補契約等於再加一套會過期的狀態）：①**端到端結束碼各自唯一**——0 ＝ 成功 archive 已在真正 repo durable、**6** ＝ failed record 已在真正 repo durable（與 1 分開）、2 ＝ lookup 命中、1 ＝ 還沒有任何終態、3 ＝ durability 未確認、8 ＝ 晉升失敗；`finalize-stage2-evidence.sh` 各模式的結束碼⛔ 不變（另一層）。②**唯讀的 `--status`**：每次從磁碟事實重新推導狀態，⛔ 不讀摘要檔，所以⛔ 沒有「讀到舊摘要」的問題；review 指出的形狀落在 **P4**，唯一的下一步是 `--recover-promotion`（回 0／6）。n11 補這一支。⚠️ **第八輪撤回 ②**，改成單一、冪等的 `--promote`（見第八輪表） |
| 中 | ⛔ **rc=8 的所有失敗被統稱為「目的地不存在、可重試」**，與「目的地已存在」等分支矛盾；各類失敗的重試資格其實不同 | ✅ （⚠️ **第八輪撤回，改成 `--promote` 的判定順序與結束碼 8／9**）「八之三」新增**狀態矩陣** P0～P5：逐類定義判定方式、目的地狀態、可否重試與**唯一的下一步**（`stop`／`none`／`promote`／`recover-promotion`）；「複本內終態的 rc=3」與「晉升失敗」都有明確的落點。⚠️ fsync 是否成功無法事後從磁碟得知，所以 `--promote` **一律先跑 ③ 對應的 recovery**（冪等），⛔ 不靠「記得上次的結束碼」。n11 依矩陣逐狀態改寫 |
| 中 | ⛔ **真正 repo 的錨點只測工作樹修改，⛔ 沒涵蓋「執行期間 commit 改掉錨點」**；第六輪寫的「`git checkout <OID>` 還原」沒有定義 OID，自動 checkout 也可能覆蓋使用者的修改 | ✅ 採 review 的第二個選項，而且⛔ 不必改 archive 的 schema：成功 archive 的 manifest 本來就永久記錄 **`finalizer_provenance.base_commit`**——finalizer 程式碼的來源 commit，也就是複本釘住的 OID（實查 `finalize-stage2-evidence.sh`）。**B／C 判讀器從這個 commit 的 git 物件取出 Stage 1 錨點與 `envcheck/`**，⛔ 不讀工作樹、⛔ 不讀目前的 HEAD——真正 repo 之後怎麼改或 commit 都影響不了判讀；撤回「`git checkout` 還原」。唯一的前提：`base_commit` 必須一直可達（⛔ 不得改寫會讓它消失的歷史；不可達時 fail-closed）。n9 補 (iii)「commit 改掉錨點」、m 補「`base_commit` 不可達」與「工作樹被改仍照樣接受」的對照組 |

**v29 第八輪 review 的修正（2026-09-29）**：

⚠️ 三項都指向第七輪新加的 `--status`／`--recover-promotion`／P0～P5 狀態機。⚠️ 與第五輪同一個教訓：⛔ 不在它上面再補一層，改成**收斂成
單一、冪等的 `--promote`**——崩潰或失敗之後，唯一的下一步都由它的結束碼決定。

| # | 問題 | 修正 |
|---|---|---|
| 高 | ⛔ **`--promote` 先跑 ③ 的 recovery，但 failed record 的 `--recover-failed-record` 成功與一般失敗都回 1**（實查 `stage2_archive.py`：成功回 1 並在 stdout 印 JSON；validator、參數、I/O 失敗也回 1），orchestrator 分不出能不能繼續 | ✅ `--promote` **⛔ 不再呼叫 ③ 的 recovery 模式**：它只是為了確保來源已 durable，而完整驗證本來就在與來源逐位元相同的 staging 上做——改成 `--promote` **自己 fsync 來源**（每個檔案、每個目錄與 parent；失敗 → 3）。⛔ 不需要解讀 rc=1，也⛔ 不需要為 stdout 定新契約。n11 補「來源 fsync 失敗 → 3、staging ⛔ 未被建立」與「③ 的 recovery 模式⛔ 未被呼叫」 |
| 中 | ⛔ **`--status` 宣稱唯讀，但完整驗證的 shell 合成守門會建 `mktemp`、登記 worktree、`git apply --index`／`write-tree`** | ✅ **撤回 `--status`**（連同 `--recover-promotion`），所以⛔ 不再有「唯讀」的宣稱；`--promote` 會寫到哪裡照實寫明——真正 repo 的 staging 與目的地、**複本的** `.git`（worktree 登記與 object）、執行目錄的暫存目錄，⛔ 不寫真正 repo 的 `.git`；崩潰留下的 worktree 登記由下一次 `--promote` 的 `git worktree prune` 清掉。n11 補「合成守門中途被殺之後重跑成功」與「真正 repo 的 `.git` inventory 不變」 |
| 中 | ⛔ **P0～P5 沒有分類優先序**，條件會重疊或漏接（完整性不符又目的地不同、來源不存在但目的地存在、多個終態、I/O 錯誤），也沒定 `--status` 的結束碼 | ✅ 矩陣換成 **`--promote` 的判定順序**（七步，依序執行、前一步沒過就不做後一步）：完整性 → prune 與清 orphan → **解析本次的唯一終態**（恰好一個、零個或多個或不認得 → 9）→ 來源 fsync → 目的地是否存在 → 6a 已存在（逐位元 ＋ 完整驗證 ＋ fsync，否則 9）／6b 不存在（空間、複製、完整驗證、rename、fsync）→ 成功；⚠️ **未列出的任何例外一律 9**。結束碼分成 **8**（可以重跑 `--promote`，目的地⛔ 不存在）與新增的 **9**（`EXIT_PROMOTION_BLOCKED`：需要人工判斷、⛔ 不得自動重跑）；另以 lock 檔互斥，避免兩個 `--promote` 互相把對方的 staging 當成 orphan |

**v29 第九輪 review 的修正（2026-09-29）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | ⛔ **lock 放在「執行目錄」，無法真正互斥**——兩個執行目錄就是兩把鎖，仍可能同時操作真正 repo 的同一個目的地，甚至在步驟 2 互清 staging | ✅ 改成對**真正 repo 的 `python/baselines/i074_stage2/` 目錄本身**做 `flock`（canonical path 開啟、鎖在 inode 上）：不同執行目錄、不同路徑寫法都搶同一把鎖，⛔ 不另建 lock 檔、⛔ 不寫真正 repo 的 `.git`。⚠️ 鎖整個 parent 而⛔ 不是單一目的地（review 建議以「repo ＋ 目的地」為 key）——步驟 2 清 orphan 的範圍是整個 parent，只鎖單一目的地仍會讓兩個不同目的地的晉升互清 staging。n11 改用**兩個不同執行目錄**驗證，並補「不同路徑寫法」一支 |
| 中 | ⛔ **B／C 的信任根 `finalizer_provenance.base_commit` 沒有在晉升時釘住**——③ 的 `verify_stage2_graph()` 只驗 schema 與 image 鏈，⛔ 擋不住「finalize 之後、晉升之前竄改來源」 | ✅ 「八之三」新增「信任根的綁定」：成功 archive 在 6a／6b 的完整驗證另驗 `finalizer_provenance.base_commit` ＝ freeze record 的 `repo_head` ＝ 複本 `HEAD`，不相等 → 9；failed record ⛔ 不進 B／C、⛔ 不適用。n11 補「只竄改 `base_commit`」的 6a／6b 各一支 |
| 中 | ⛔ **rc=8 寫「目的地不存在」，但拿不到 lock、`rename_noreplace` 遇到 `EEXIST` 時目的地可能已經存在** | ✅ rc=8 改成「**本次呼叫沒有確認晉升完成，但可以安全重跑**」，⛔ 不承諾目的地不存在；重跑時由步驟 5／6a 依磁碟事實裁決 |

**v29 第十輪 review 的修正（2026-09-29）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | ⛔ **鎖只涵蓋晉升，⛔ 沒守住「唯一一次正式 Stage 2」**——兩個 orchestrator 可以各自通過 preflight、在不同複本跑完三小時的 replay，到晉升才被序列化；兩趟若一個成功、一個失敗，目的地不同，甚至可能都被晉升 | ✅ **同一把鎖提前到真正 repo 的 bootstrap**：驗完自身內容就取得（在建立複本、preflight、replay **之前**），鎖的 fd ⛔ 不設 close-on-exec、re-exec 之後由複本內的 orchestrator 一路持有到晉升結束；複本內的 orchestrator 先驗證自己確實持有這把鎖（⚠️ **第十一輪撤回 fd 繼承與「再 flock 一次」的驗證**，改成持鎖的 wrapper ＋ `/proc/locks`，見第十一輪表）。拿不到 → 結束碼 1、⛔ 不建複本、⛔ 不計入正式 scan。流程內的 `--promote` 沿用它，單獨執行的 `--promote` 才自己取得。⚠️ 鎖只防並行，跨時間的次數仍由計次政策與 failed-record lookup 管。新增 n7b（兩個執行目錄同時啟動：後到的 replay 等⛔ 都未被呼叫） |
| 中 | ⛔ **6a 的 `base_commit` 測試沒有隔離到新守門**——6a 先逐位元比對，只竄改目的地會先因不同而失敗 | ✅ n11 的 6a 那一支改成：來源與既有目的地都改成**同一份** canonical 但錯誤的 `base_commit`，讓逐位元比對與 ③ 的各道都通過，只剩「信任根的綁定」使它回 9 |
| 低 | ⛔ **窗口 B 的兩處仍寫「含任何 rc=3 的 recovery」**；第五輪表的摘要提到 `--recover-promotion` 卻沒有撤回註記 | ✅ 兩處改成「rc=3 之後重跑 `--promote`，直到回 0／6」；第五輪表的「設計」列補註第八輪已撤回 |

**v29 第十一輪 review 的修正（2026-09-29）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | ⛔ **「在繼承的 fd 上再 flock 一次成功」證明不了 bootstrap 原本就持有鎖**——Linux 的 `flock` 對同一個 fd 再鎖一次必定成功，傳入一個未鎖的目錄 fd 也會當場取得；而且 fd 全程不設 close-on-exec，會被 Git、Docker、Python 等子程序繼承，子程序比 orchestrator 活得久就會延長鎖 | ✅ 採 review 較穩妥的那一個選項：**鎖由持鎖的 wrapper 擁有、⛔ 不交給子程序**——`flock -n -o <目錄> setpriv --pdeathsig TERM -- <持鎖階段>`：`flock(1)` 行程持鎖、`-o` 讓命令⛔ 不繼承 lock fd、`--pdeathsig TERM` 讓 wrapper 消失時子程序收到 TERM；wrapper 等待並原樣回傳結束碼。持鎖的驗證改讀 **`/proc/locks`**：該目錄的 `st_dev` ＋ `st_ino` 上有 `FLOCK`、持有者 pid 是自己祖先鏈上的 wrapper。撤回第十輪的 fd 繼承。⚠️ **2026-09-29 先在 scratchpad 實測四件事都成立**：執行中獨立 fd 拿不到鎖、`/proc/locks` 的持有者 pid 就是 `flock` 行程；wrapper 結束後即使留下 `setsid` 的背景程序，獨立 fd 立刻拿得到；wrapper 被 KILL 時子程序收到 TERM；只傳入未持鎖的 fd 時 `/proc/locks` 查不到持有者。n7b 依 review 補齊四支（執行中拿不到、結束後立刻拿得到、後代程序不延長、未持鎖的 fd 不被誤判），另加 wrapper 被 KILL 一支。⚠️ **第十二輪改成 supervisor 持鎖**——wrapper 被 KILL 時鎖立刻釋放、本趟的後代卻還活著（見第十二輪表） |

**v29 第十二輪 review 的修正（2026-09-29）**：

| # | 問題 | 修正 |
|---|---|---|
| 高 | ⛔ **wrapper 被殺後鎖先釋放，但 replay／Docker 後代可能繼續執行**——parent-death signal 只送給直接子程序；bash 在等前景命令時會延後 TERM trap。第二個正式 orchestrator 已能取得鎖，上一趟的 replay 卻還活著，仍違反唯一正式趟。n7b 甚至把「還在跑的 docker CLI 不延長鎖」列成期望 | ✅ ⚠️ 先在 scratchpad **實測重現**：wrapper 被 KILL 後鎖立刻釋放、bash orchestrator 與前景 `sleep` 仍在跑、trap 等前景命令結束才執行。改成**專用 supervisor 持鎖**：以新的 session 啟動子程序，**本趟 ＝ session 成員 ＋ 帶本趟 label 的容器**；orchestrator 結束或 supervisor 收到 TERM／INT／HUP 時，先依 label 移除容器、再 TERM／KILL session 成員，**確認兩者都清空之後才放鎖**，清不空就⛔ 不放。區分「無關的 detached helper」（自己 `setsid` 另開 session，⛔ 不延長鎖）與「本趟程序」（必須全部停止）。⚠️ supervisor 被 **SIGKILL** 無法處理（鎖一定立刻釋放），照實寫成已知限制並加三道保險：orchestrator 的長步驟一律背景 ＋ `wait`（trap 立即生效）、舊 orchestrator 在發布／晉升前以 `/proc/locks` 驗證 supervisor 仍持鎖、新 supervisor 查到任何 Stage 2 label 的容器就中止。n7b 改成斷言「本趟的程序與容器都消失之後」獨立 fd 才拿得到鎖。⚠️ **第十三輪**：以 session 定義本趟與「`setsid` 即無關」撤回（改成 child subreaper），容器掃描的保險改成 active-run sentinel，鎖改到 host 層級（見第十三輪表） |
| 中 | ⛔ **同一條 `flock -n` 無法同時產生衝突時的 rc=1 與 rc=8** | ✅ 衝突時的結束碼由 supervisor 依模式決定：完整 orchestrator 回 **1**、`--promote` 回 **8**；若實作改用 `flock(1)`，前者用預設、後者用 `-E 8`（實測預設回 1、`-E 8` 回 8）。n7b 補以實際 argv 啟動的兩支結束碼測試；另補 orchestrator 被訊號中斷時 supervisor 清空後回 **128＋N**（端到端結束碼同步） |

**v29 第十三輪 review 的修正（2026-09-29）**：

⚠️ 先請使用者裁決三件事（決策表第 5～7 列）：SIGKILL 下**保證**唯一正式趟（加 sentinel）、串行化範圍是**整台 host**、實作細節**移到 ⑦ 的計畫書**（v29 只定不變條件與已裁定的機制——這幾輪 review 要的檔名、注入者、argv、測試落點都是 ⑦ 計畫書層級的內容，⑥ 開始時已確認 ⑦ 的實作計畫書另寫）。

| # | 問題 | 修正 |
|---|---|---|
| 高 | ⛔ **SIGKILL 路徑仍有 TOCTOU**：舊 supervisor 被 SIGKILL → 新 supervisor 取鎖、掃描當下沒有 label 容器而通過 → 舊 orchestrator 這時才建立 replay 容器 → 兩趟並行 | ✅ **active-run sentinel**（決策表第 5 列）：取鎖之後、啟動任何子程序之前以 `O_CREAT \| O_EXCL` 建立並 fsync；只有確認本趟的程序與容器都清空之後才刪除、再放鎖。supervisor 被 SIGKILL 時 sentinel 留下，新的 supervisor 一律 fail-closed（完整 orchestrator 回 1、`--promote` 回 9），待人工確認之後由條件式的解除入口刪除——第二趟在啟動任何東西**之前**就被擋下，⛔ 不依賴掃描時機。n7b 加 **deterministic barrier** 重現 review 描述的順序 |
| 高 | ⛔ **任何後代都能用 `setsid` 逃出本趟的定義** | ✅ supervisor 設為 **child subreaper**：本趟 ＝ supervisor 的**所有後代** ＋ 本趟 label 的容器；後代即使 `setsid`、父程序也結束，孤兒仍回到 supervisor 底下（2026-09-29 在 scratchpad 實測：`setsid` 之後、父程序已結束的孫程序，ppid 回到 subreaper）。撤回「`setsid` 即無關」；「無關的 helper」只指 supervisor 之外啟動的程序。⚠️ 比 review 建議的「正式後代不得 `setsid`」更強——⛔ 不必靠規範約束所有工具。n7b 的 detached helper 改從 supervisor 外部啟動，另加「正式後代 `setsid` 逃不掉」一支 |
| 中高 | ⛔ **Docker label 是 SIGKILL 保險的核心，契約卻未定案**（兩種注入方式並列、`<run id>` 指什麼不明、`docker ps` 失敗怎麼辦） | ✅ 「八之一」寫成不變條件：token 由 supervisor **每次執行**產生、⛔ 不可注入或覆寫；每個 `docker run`／`docker create` **恰好**一個 `i074.stage2.run=<token>`；使用者自帶同一個鍵 → 拒絕；清理用完整 `key=value`、殘留檢查用鍵（host 層級）；`docker ps` 失敗 → fail-closed；**注入點只能有一個**。⚠️ 注入點是哪一個（shim 或修改 runner／finalizer）依決策表第 7 列在 ⑦ 的計畫書定。n7b 補 missing／duplicate／spoof 與 `docker ps` 失敗 |
| 中 | ⛔ **受影響檔案與資料流沒有同步 supervisor** | ✅ 「三」補 supervisor 與 label 注入點兩列（檔名與測試落點依決策表第 7 列在 ⑦ 定）；「八之一」的「進入複本」寫出完整資料流（入口 → supervisor → 持鎖階段 → 複本 → `exec` 複本內的 orchestrator）；supervisor **自己也先驗檔案內容 ＝ HEAD**，⛔ 不執行未 commit 或與預期不同的版本；「八」的 ⑦ 摘要同步 |
| 低 | 端到端結束碼的 128＋N 沒有區分可攔截與不可攔截的訊號 | ✅ 補註：只適用可攔截的訊號；supervisor 自己被 SIGKILL 時由作業系統直接回 **137**、⛔ 沒有完成清理（sentinel 留下） |
| ⚠️ 連帶 | 串行化範圍裁定為整台 host，而第十二輪的鎖在真正 repo 的目錄 | ✅ 鎖改到 Stage 2 identity 所在的 XDG 目錄（host 層級），晉升也因此全 host 串行化；「八之三」的「冪等與互斥」同步 |

**v29 第十四輪 review 的修正（2026-09-29）**：

⚠️ 解除條件的做法先請使用者裁決（決策表第 8 列：token 繼承 ＋ 掃 `/proc`；⛔ 不採「只接受重開機」——這台 host 同時跑著 live 服務）。

| # | 問題 | 修正 |
|---|---|---|
| 高 | ⛔ **sentinel 的解除入口仍可能過早解除**——supervisor 被 SIGKILL 之後 subreaper 也消失，後代被 init 收養；停在「建立容器之前」的舊 orchestrator 在「supervisor 不在 ＋ 沒有 label 容器」的條件下看不到，sentinel 被解除後它才建立容器 | ✅ （⚠️ **第十五輪撤回**：`/proc` 掃描有 fork／exit 的 TOCTOU，改成「重開機才解除」，見第十五輪表）「八之一之二」第 9 列重寫：解除入口**先取得同一把鎖**；**嚴格解析** sentinel（第 4 列的封閉 schema；空檔、部分寫入、非 canonical、schema 不符一律⛔ 不自動解除）；supervisor 是否還在以 **pid ＋ starttime ＋ boot ID** 判斷（⛔ 不只看 pid）；**所有後代繼承 supervisor 產生的 token**（環境變數），解除時掃描同 `uid` 的 `/proc/*/environ`，仍有帶 token 的程序就⛔ 不解除（讀不到 environ 也 fail-closed）；再確認 Stage 2 label 的容器為空。⑩ 的程式路徑⛔ 不得以乾淨的環境啟動子程序。n7b 補 **deterministic barrier 的解除路徑**：舊程序停在建容器之前 → SIGKILL → 解除必須失敗 → 舊程序消失之後才能解除；另補 pid 重用、sentinel 損壞、讀不到 environ、boot ID 不同 |
| 高 | ⛔ **XDG 路徑不能提供「整台 host」互斥**——不同帳號、不同 `XDG_DATA_HOME` 會拿到不同的鎖；現有測試本來就會覆寫 `XDG_DATA_HOME` | ✅ （⚠️ **第十五輪**：「不同帳號也互斥」撤回、範圍降為固定執行帳號，見第十五輪表）鎖與 sentinel 改放 **`/run/lock`**（2026-09-29 實查：1777 的 tmpfs，任何帳號都能建檔、⛔ 不受 XDG 影響）——這是實現使用者已經裁決的「整台 host」，⛔ 不改裁決。sentinel 因此隨開機週期消失，與「重開機之後舊程序都不在」一致；重開機之後殘留的容器由啟動檢查擋下。n7b 補「不同 `XDG_DATA_HOME`」（另一支：不同 repo）仍互斥 |
| 低 | ⛔ **「八之一」的表格被實體換行切斷**（長內容塞在單一儲存格） | ✅ 移成表格外的獨立小節「**八之一之二**」，每條規則各一列、單一實體行；「八之一」的表格只留一列指引 |

**v29 第十五輪 review 的修正（2026-09-29）**：

⚠️ 兩個高風險項目都需要 host 層級的設施（專用 cgroup、root 預先建立的鎖檔）。先實查（2026-09-29）：`dev` 所在的 cgroup 屬
`system.slice/ssh.service`、各階層都不可寫；`dev` 沒有 systemd user manager（linger 名單只有另一個帳號，`systemd-run --user` 連不上
bus）；`sudo` 需要密碼——⛔ 沒有 root 的一次性設定就做不到。於是請使用者裁決（決策表第 9 列）：✅ **固定執行帳號 ＋ 重開機才解除**。

| # | 問題 | 修正 |
|---|---|---|
| 高 | ⛔ **掃描 `/proc/*/environ` 有 fork／exit 的 TOCTOU**：列舉之後才 fork 出的子程序不在清單上、而 parent 在被讀到之前就結束——多掃幾次只能降低機率，⛔ 不是結構性保證；環境變數也可能在後續 exec 時遺失 | ✅ 撤回 token 繼承、`/proc` 掃描與整個解除入口：supervisor 被 SIGKILL（含 OOM killer）之後**⛔ 沒有解除入口**，sentinel 放在 tmpfs 的 `/run/lock`，**只在正常釋放或重開機時消失**；重開機之後由啟動檢查確認沒有殘留的 Stage 2 容器才放行。操作者⛔ 不得手動刪除 sentinel。本趟的範圍回到「supervisor 的所有後代（subreaper）＋ label 容器」，只在 supervisor 活著時使用。n7b 刪掉掃描相關的測試，改成「沒有任何解除入口」與「模擬重開機之後」 |
| 高 | ⛔ **`/run/lock` 是 1777，⛔ 不代表固定的鎖檔能跨帳號安全共用**（umask 可能讓鎖檔變成 0600、sticky bit 下別的帳號刪不掉也修不了、可預測路徑要防 symlink 與 inode 被換掉）；「不同帳號也互斥」卻被列成選測 | ✅ 範圍降為**固定的執行帳號**（本 host：`dev`；非此帳號 → 1／8），撤回「不同帳號也互斥」。鎖檔以 `O_NOFOLLOW`、mode `0600` 開啟，`fstat` 驗 regular file、owner、mode、link count ＝ 1，並以 `lstat` 確認路徑仍是同一個 inode；**永不 unlink**。sentinel 同樣以 `O_EXCL \| O_NOFOLLOW`、`0600` 建立並驗屬性。⚠️ 因為⛔ 沒有解除工具，review 提到的「解除工具要由原 UID 執行」⛔ 不再適用。n7b 補各項屬性的 fail-closed 與「非固定帳號」 |
| 中 | ⛔ **sentinel 的封閉 schema 缺欄位型別與範圍** | ✅ 「八之一之二」第 4 列補齊：`token` 為 64 位小寫 hex；`boot_id` 為小寫 UUID 且必須等於目前的 boot ID；`uid`、`supervisor_pid`、`supervisor_start_time` 為嚴格整數（⛔ 不接受 `bool`；`uid >= 0`，其餘 `> 0`），`uid` 必須等於執行帳號；`mode` 為封閉列舉；所有字串都受正規式或列舉限制，因此⛔ 不可能含 NUL 或控制字元。⚠️ schema 只用於寫入與診斷——sentinel 一旦存在就拒絕，⛔ 不因內容讀不懂而放行。n7b 補每個欄位一支 |

**v29 第十六輪 review 的修正（2026-09-29）**：

| # | 問題 | 修正 |
|---|---|---|
| 中 | ⛔ **殘留容器檢查失敗之後，sentinel 的處置沒有定義**：順序是「建 sentinel → 查殘留容器」，查到殘留或 `docker ps` 暫時失敗就回 1／8——這時還沒啟動任何子程序，卻留下 sentinel，之後排除原因也只能重開機，rc=8 的「排除原因後重跑」失去意義 | ✅ 兩個做法都採用：①**順序對調**——取鎖、確認沒有舊 sentinel 之後，先查殘留容器（`docker ps` 成功且為空），**確認為空才建立 sentinel**（「八之一之二」第 7、3 列）；②**收回規則**——sentinel 建立之後、**第一個子程序啟動之前**的任何可控失敗，supervisor 都刪掉自己建的那一份、fsync 目錄、再放鎖；只有 SIGKILL 或刪除本身失敗才會留下（第 3 列；第 9 列同步成「三種情況消失」）。n7b 補：有殘留容器、`docker ps` 失敗都⛔ 不留下新的 sentinel；排除原因後直接重跑成功、⛔ 不需要重開機；第一個子程序之前的注入失敗會收回 sentinel |
| 低 | ⛔ **決策表第 5 列仍寫「待人工確認殘留已清除才解除」** | ✅ 加註「第十五輪由第 9 列取代：⛔ 沒有解除入口，只在正常釋放或重開機時消失」 |
| 低 | ⛔ **決策表的編號順序是 6、8、9、7** | ✅ 重排成 6、7、8、9（內容不變） |

**v29 第十七輪 review 的修正（2026-09-29）**：

| # | 問題 | 修正 |
|---|---|---|
| 中 | ⛔ **第 9 列自相矛盾**：前段允許第 3 列的收回，後段又寫「任何沒走完第 8 列的結束 → sentinel 留下」——收回本來就不走第 8 列 | ✅ 改成「**除第 3 列明定的安全收回之外**，supervisor 被 SIGKILL（含 OOM killer），或任何已啟動本趟 workload child、卻沒完成第 8 列清理的結束 → sentinel 留下」 |
| 中 | ⛔ **兩處總流程仍是「取鎖 → 建 sentinel」的舊順序**（「八」的 ⑦ 摘要、「八之一」的「進入複本」）——實作者最可能照著做 | ✅ 兩處都同步成「驗執行帳號 → 取鎖 → 確認⛔ 沒有舊 sentinel → 查殘留容器（`docker ps` 成功且為空）→ 建 sentinel → 設 subreaper → 啟動持鎖階段（workload child）」 |
| 中 | ⛔ **收回的邊界與「自己的 sentinel」不夠精確**：同時寫「第一個子程序啟動之前」與「啟動子程序失敗」，而殘留檢查的 `docker ps` 本身也是子程序 | ✅ 邊界釘成「**持鎖階段的 workload child 成功啟動（`exec` 成功）之前**」（`docker ps` 在建立 sentinel 之前就結束，⛔ 不算）；曾產生過 child 就先 reap，並確認沒有活著的後代、本趟 label 的容器為空；刪除前以**建立時保存的 sentinel fd** `fstat`、對路徑 `lstat`，確認是同一個 `st_dev` ＋ `st_ino`，不符 → ⛔ 不 unlink（第 3 列；⚠️ 第 8 列的正常釋放也套用同一個檢查）。n7b 補「`fork` 成功而 `exec` 失敗」與「刪除前路徑被換掉」（收回與正常釋放各一支） |

**v29 第十八輪 review 的修正（2026-09-29）**：✅ review 結論「實質設計已收斂」，只修三處文字殘留——「八之一之二」第 3 列的「啟動任何子程序之前」與第 9 列②的「第一個子程序啟動之前」都改成「**持鎖階段的 workload child 成功啟動之前**」（殘留檢查的 `docker ps` 本身也是子程序，舊寫法與第十七輪釘住的邊界不一致）；小節標題更新成「第十～十七輪」。

#### （承上）I-074 Stage 2 計畫書 v28（2026-09-23，✅ **已確認**）的內容

✅ **使用者於 2026-09-23 確認 v28 與 ③ evidence contract v12，並指示開始 ③a／③b 的實作**。

⚠️ **v28 修掉 v27 review 的兩個中等問題與一項文字殘留**，並記錄同一輪 review 的裁決。
v27 的五個阻擋點經 review **確認已實質修正**；其餘內容不變。
（確認前的規則：⛔ v28 與 ③ v12 一起確認前，⛔ 不得進入 ③ 的實作——✅ 已於 2026-09-23 滿足。）

| # | 問題 | v28 的處置 | 同步的節 |
|---|---|---|---|
| 1 | ⚠️ **top-level `position_action` 仍保留「只記錄、不影響判定」的例外**——但它由 `_decision_action()` 在 lifecycle **之前**算出，`_final_action_from_entry()` 只原樣帶回，counterfactual patch⛔ 沒碰這條路徑 | ⚠️ **撤回例外**：它⛔ 不是必要的翻轉欄位（B 不要求它改變），⚠️ 但**有差異就判 C**；已先對照 `decision_engine.py` 查證上述推導路徑 | 關閉條件、六 9 |
| 2 | ⚠️ **envcheck 沒定義 key 集合不一致怎麼落證據**——`rows_compared` 是交集還是聯集、單側 key 算不算 `row_mismatch_count`、sample 怎麼標示缺在哪一側 | ⚠️ ③ v12 補成封閉契約：`key_mismatch_*`（sample 帶 `side`）、`rows_compared`＝交集、`row_mismatch_count` 只計交集內 bytes 不同的 key，外加兩條計數不變條件；三種形狀各一支測試 | ③ v12 的三之三、十 |
| 3 | ⚠️ **rc=6 的舊敘述**：決策表、事故紀錄前提、測試 z、failed record 開頭仍只寫 `before_candidates ≠ ∅`；`bounded_diagnostics` 那一列還混著作廢的 v10 schema | ⚠️ 統一改成「**任一反事實生效條件不成立**」並列出兩種原因；v10 的舊 schema 移出現行契約表 | v26 決策表、二①、六 2、③ v12 的七 |

**同一輪 review 的裁決（2026-09-23）**：

| 項目 | 結果 | 附帶條件 |
|---|---|---|
| ② 的 review | ✅ **通過**（見「② 的 review 結論」） | —— |
| v26 決策 #1：NOT_EQUIVALENT 的處置 | ✅ 同意 | 封存 `envcheck/`、停在 before 之前、另立 issue；⛔ 不得以 after' 取代 D+1 |
| v26 決策 #3：image 保存方式 | ✅ 同意 | ⚠️ **正式操作文件要寫死 tarball 的保存位置與驗 SHA 的步驟**（見「二、⑤」與「七」） |
| v26 決策 #4：見證證據的形式 | ✅ 方向同意 | ⚠️ 要先補上 key 集合不一致的 schema——**已於 ③ v12 補上** |
| v26 決策 #5：執行順序 | ✅ 同意 | 先完成 envcheck 的最小實作與環境閘門，EQUIVALENT 才繼續 ③d |
| v26 決策 #6：結束碼 6／7 | ✅ 同意 | 兩者分別代表「工具的反事實失效」與「環境不等價」，⛔ 不得混進一般的 rc=1 |
| N ＝ 20 | ✅ 同意 | 寫死的共用常數、⛔ 不開 CLI；完整計數照樣保留；failed record 與 envcheck 共用上限、sample 欄位各自封閉；⛔ 不得套用到 B／C 的正式證據（見 ③ v12「七」） |
| 「判讀欄位以外出現差異一律判 C」 | ✅ 同意，⚠️ 採**字面上的 fail-closed**（含 `position_action`，見上表第 1 列） | —— |

#### （承上）Stage 2 計畫書 v27（2026-09-23）的內容

⚠️ **v27 修掉 v26 review 的五個阻擋點**（⚠️ 其中兩點連同 ③ evidence contract v11 一起改）。
v26 的其餘內容不變；⛔ **v27 與 ③ v11 一起確認前，⛔ 不得進入 ③ 的實作**。

| # | 阻擋點 | v27 的處置 | 同步的節 |
|---|---|---|---|
| 1 | ⛔ **NOT_EQUIVALENT 的證據無法發布、也無法 recovery**——E3 要求重算結果「必須是 EQUIVALENT」，envcheck 模式卻要在 NOT_EQUIVALENT 時照樣發布並回 rc=7；`envcheck/evidence_manifest.json` 也沒有 schema | ⚠️ E3 拆成 **E3a（archive 自洽，接受兩種結果）** 與 **E3b（Stage 2 使用資格，只接受 EQUIVALENT）**；補上 envcheck manifest 的封閉 schema，outcome **以 equivalence artifact 為唯一來源** | ③ v11 的三之三、五之零、五、六、七之二、七之四、十 |
| 2 | ⛔ **新加的兩種 rc=6 失敗寫不進 failed record**——`failure_reason` 只有 `before_candidates_nonempty`、計數必須是正整數、sample 沒有 `rr_decoupling_candidate`，flag 假綠時的計數卻可以是 0 | ⚠️ ✅ **使用者裁決（2026-09-23）：兩種原因，靠檢查順序互斥**——先驗 flag 一致性（`candidate_flag_inconsistent`），一致才驗候選集合（`rr_not_restored`）；`bounded_diagnostics` 改成依原因分流的封閉 union | 二①之三、二③、六 2、③ v11 的七 |
| 3 | ⛔ **串流驗證與「既有 validator 一律不動」互相矛盾** | ⚠️ 抽出 **row-level 共用原語**，整批與串流兩種 validator 都呼叫同一套；**公開 contract 與 Stage 1 行為不變、允許內部重構**；Stage 1 既有測試⛔ 不修改斷言並全數續跑 | 二①之三、三、五、③ v11 的五之一、八、九 |
| 4 | ⛔ **「最壞 5 趟」與崩潰重試互相衝突**——把 D、D+1 也算進去，最多可以啟動 8 趟 | ⚠️ ✅ **使用者裁決（2026-09-23）：跑完最多 5 趟、物理啟動最多 8 趟**（含 D、D+1），每一格最多因崩潰重試 1 次 | 計次裁決、寫死的範圍 |
| 5 | ⛔ **凍結窗口沒有涵蓋 rc=7 與 rc=3 recovery** | ⚠️ A 窗口結束於 `envcheck/` 已 durable 且結束碼是 0 或 7；B 窗口結束於成功 archive 或 failed record 已 durable；⚠️ **任何 rc=3 都延續到對應的 recovery 完成** | 八之一 |

⚠️ **v26 決策點的狀態**：#2（次數上限）已由上表第 4 列裁決；其餘五項（NOT_EQUIVALENT 的處置、
image 保存方式、見證證據的形式、執行順序、新結束碼）**仍待確認**——✅ 已隨 v28／③ v12 確認（2026-09-23），見 v28 的裁決表。

#### （承上）Stage 2 計畫書 v26（2026-09-23）的內容

⚠️ **v26 由 2026-09-23 的開工前盤點觸發**：計畫書與程式碼逐項對照。
v25 已確認的內容，**除下表列出的各節外一律不變**。
⛔ **v26 確認前，⛔ 不得進入 ③ 的實作**。

✅ **使用者裁決（2026-09-23）**：Stage 1 釘住的 image 已不在本機 → **兩側都在新 image 重跑**
（⛔ 不採「比對環境指紋就放行」，也⛔ 不採「接受環境差異直接跑」）。見「二、⑤」。

| # | 缺口 | v26 的處置 | 同步的節 |
|---|---|---|---|
| 1 | ⛔ **Stage 1 釘住的 image `sha256:d66030dca485…` 已不在本機**——`pin-replay-image.sh` 遇到「identity 還在、image 已不在」會 fail-closed，`I074_MODE` 的 Stage 2 跑不起來；③ v9 的「Stage 2 identity 與 Stage 1 逐位元相同」無法成立；「唯一產品語意變因」的前提（同一個執行環境）也失去保證 | ⚠️ 新增「二、⑤」：新 image（**專用 tag ＋ repo 外 tarball**）、**依 stage 固定推導的 identity**、**after' 見證趟**、**環境等價閘門** | 一、二⑤、三、四、五、六、八 |
| 2 | ⛔ **發布模型與 ③ v9 互相矛盾**——本計畫書寫「replay 程序自己寫 staging 並 rename」，③ v9 寫「replay 只產 operational 輸出、由 shell finalizer 收進 archive」，而且 ③ ⛔ 沒有回頭同步本計畫書 | ⚠️ 本計畫書改成**兩段式**，並補上 orchestrator、凍結 patch 的生命週期、`before_candidates ≠ ∅` 的專屬結束碼 | 二③、二之二、三、四、八 |
| 3 | ⛔ **finalizer／recovery／preflight 的記憶體沒有納入設計**——它們都是獨立程序，整份載入 after（實測單份約 +365～449 MiB）加上同量級的 before source，會撞上 crossday comparator 的同一道牆（約 730 MiB） | ⚠️ harness 範圍擴大到所有會讀全量 artifact 的程序；記憶體模型由 ③ evidence contract（⚠️ 現行版）的「五之一」定義；原本說「另立一筆」的 comparator 問題立為 [I-117](#i-117stage-1-的-comparatorfinalizerrecovery-整份載入兩份-after-artifact超過-mem-guard) | 二③、六 1 |
| 4 | ⚠️ **`before_candidates == ∅` 可能假綠**——`validate_diagnostics(side="before")` 刻意不驗等價式（`ecbc141^` 時代的理由），flag 若壞成恆 `False` 而 RR 沒加回去，守門會空洞通過 | ⚠️ before 側改驗**直接不變條件 ＋ 等價式**（⛔ 不改既有 validator） | 二①、二②、六 2 |
| 5 | ⚠️ v25 已確認，**內文仍殘留 4 處「待確認」**；「patch 失效不計入正式 scan」是政策延伸，卻沒寫進計次裁決節，且與「⛔ 不得再有第四趟」衝突 | ⚠️ 殘留改成「已隨 v25 確認」；**計次上限改寫進「正式 scan 的計次裁決」**（✅ 已隨 v28／③ v12 確認（2026-09-23）） | 二①、五、計次裁決 |
| 6 | ⚠️ **B／C 只寫「由人判讀」**——判準等於看到結果之後才定 | ⚠️ ⑩ 之前先實作並測過 **B／C 判讀器** | 三、六、八、關閉條件 |
| 7 | ⚠️ **differential guard 沒有落點**——檔案位置與執行機制都不在受影響檔案表 | ⚠️ 補進受影響檔案表與測試 | 三、六 3 |
| 8 | ⚠️ **Stage 2 的凍結窗口未定義範圍** | ⚠️ 新增「八之一」 | 八之一 |

⚠️ **v26 的待確認決策點**（⛔ 確認前⛔ 不得實作；✅ 已隨 v28／③ v12 確認（2026-09-23），見 v28 的裁決表）：

| # | 決策 | 建議 |
|---|---|---|
| 1 | 環境等價判定為 **NOT_EQUIVALENT** 時 | 停在 before 之前、封存見證證據、另立 issue；⛔ **不得改用 after' 取代 D+1**（那等於看到結果之後換掉正式 Stage 1 artifact） |
| 2 | 物理 replay 的上限 | **最壞 5 趟**（見「正式 scan 的計次裁決」的 v26 修訂）。⚠️ **v27 訂正**：這一格與崩潰重試衝突；✅ 使用者裁決為「**跑完最多 5 趟、物理啟動最多 8 趟**」 |
| 3 | image 保存方式 | **專用 tag（必要）＋ `docker save` tarball 放在 repo 外**（記 SHA 與還原程序） |
| 4 | 見證證據的形式 | **獨立的小 archive**（`python/baselines/i074_stage2/envcheck/`），Stage 2 archive **以 manifest 錨定**、⛔ 不複製成員（比照 Stage 1 信任錨） |
| 5 | 執行順序 | **先過環境閘門，再做 ③ 的其餘部分**（見「八、執行順序」） |
| 6 | 新結束碼 | `EXIT_COUNTERFACTUAL_INEFFECTIVE = 6`（⚠️ v28 訂正：**任一反事實生效條件不成立**——`candidate_flag_inconsistent` 或 `rr_not_restored`，見「①之三」；⛔ 不只 `before_candidates ≠ ∅`）、`EXIT_ENV_NOT_EQUIVALENT = 7`（見證不等價） |

⚠️ **② 的 review 狀態**：v26 當時兩節都是「待 review」；✅ **2026-09-23 已通過**（見「② 的 review 結論」）。

#### （承上）Stage 2 計畫書 v25（2026-09-22，✅ **已確認**）的收尾

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
| 3 | ⛔ **⛔ 不可用「中繼 commit」切分兩份 patch**——會動到 detached worktree 的 HEAD，與 `replay_args_prepare_worktree()`（`replay-args.sh:154`）的「HEAD 必須等於解析出的 OID」契約衝突 | ⚠️ 改用 **`git write-tree` 的中繼 tree**，HEAD 全程不動；⛔ 並撤回「交換順序合成 hash 必然不同」的錯誤假設，見「三之一之二」 |
| 4 | ⚠️ differential guard 的**允許差異集合太窄**——合法翻轉還會連帶改 `market_bias`——`decision_engine.py:1318` 的 `_market_bias()` 直接回傳 `bias_state`與 reason codes，而 `artifacts.py` 的 `compare_rows()`**⛔ 不挑欄位比** | ⚠️ 改成明確的 **comparison projection ＋ lifecycle 下游依賴閉包**，見「六、3」 |

**⚠️ 一併採納的裁決建議**：`I074_MODE` **納入** counterfactual flag（否則正式反事實執行反而
繞過既有 run identity 守門）；⛔ **不採中繼 commit**、改用中繼 tree；
⚠️ **patch 失效不計正式 scan 成立，但⛔ 不得靜默重跑**——必須保存**當次的兩份 patch SHA、
有界診斷與事故紀錄**（見「二、①」計次列）。

#### （承上）Stage 2 計畫書 v19 的缺口修補

⚠️ **v19 補上 v18 review 抓到的兩個高風險實作缺口**（⛔ 兩者都是「文件宣稱的契約在現行程式裡
做不到」，⛔ 不是措辭問題）：

| # | 缺口 | v19 的處置 |
|---|---|---|
| 1 | ⛔ **CLI 分不出「一般 Stage 2」與「I-074 counterfactual」**——第五道檢查對所有 Stage 2 無條件生效（`evaluation.py:3183` 的 `run_bundle_stage()` 內），直接改就會讓 v18 宣稱保留的 rc=4 **變成不可達** | ⚠️ 新增明示 opt-in **`--i074-counterfactual`**，見「二、④」 |
| 2 | ⛔ **兩份 patch 的獨立 SHA 現行 runner 算不出來**——只有一個 `TOOLING_PATCH`（`run-replay-offline.sh:86`）、一次套用與一個合併 SHA（`:180`）、一個 `--tooling-patch-sha256`（`replay-args.sh:65` 的 `replay_args_offline()`） | ⚠️ 定義**兩個輸入 ＋ 固定順序 ＋ 增量 diff SHA ＋ 合成 hash**，見「三之一之二」；⛔ v18 的「既有機制、不新增參數」**已改掉** |
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

⚠️ **v26 前提（✅ 已隨 v28／③ v12 確認（2026-09-23））**：Stage 1 釘住的 image 已不在本機，所以 before 必須跑在**新 image**；
⚠️ 已封存的 after 要能繼續當對照的一側，**先要證明新 image 與舊 image 對這份 bundle 等價**
——由同一個新 image 上的 **after' 見證趟**與 Stage 1 D+1 逐列比對。⛔ 不等價就停，見「二、⑤」。

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
| ⚠️ **執行環境（v26）** | before 與 **after' 見證趟**都跑在**同一個新 image**；比對的 after 側仍是**已封存的 D+1**，由**環境等價閘門**證明兩個 image 對這份 bundle 等價（見「二、⑤」） |

| ⛔ 不做 | 理由 |
|---|---|
| ⛔ 不改 **after 側**的任何東西 | after 直接用已封存的 Stage 1 artifact，⛔ 一個位元都不動 |
| ⚠️ before 側**只做一項刻意的產品語意變更** | ⚠️ **唯一產品語意變因是 setup RR 條件**——⛔ 實作會跨 lifecycle 參數、呼叫端、判定式與測試（見「② v2」的 patch 範圍），但⛔ **不得順手改其他判定** |
| ⛔ 不重產 bundle | 見 `development-workflow.md`「凍結 bundle 一旦選定就不得重產」 |
| ⛔ 不動 Stage 1 已封存的證據 | `python/baselines/i074_stage1/` 是終態 |
| ⛔ 不改 canonical JSON 的位元組輸出 | 所有既有 SHA 與 artifact 契約的根 |
| ⛔ 不為了省記憶體停 live container | 那是 2026-09-18 的權宜之計，⛔ 不得變成程序 |
| ⛔ 不改 `crossday.py` | Stage 1 的 comparator，⛔ 不是 Stage 2 前置（理由見下方「⛔ `crossday.py` 的改造不在本計畫範圍」）。⚠️ **v26 的環境等價比對是新模組**，⛔ 不改它 |
| ⛔ **不得以 after' 取代 D+1**（v26） | after' 只是**環境見證**，⛔ 不是新的正式 Stage 1 artifact；正式的 after 側仍是 `python/baselines/i074_stage1/` 的 D+1 |

##### 二、必須先解的子問題（v19 起四個，⚠️ v26 新增第五個）

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

**v3 裁決**（✅ **已隨 v25 確認**，2026-09-22——⚠️ v26 把原本殘留的「待使用者確認」改掉；⛔ **取代 v2 的「before 側自己算完整反事實」**）：

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
| 違反時的行為 | ⚠️ 中止並產出 **failed-attempt record**（見下方計次列）——⛔ **不產 `candidate_mismatch.json`**、⛔ **不產正式 evidence archive**：這是 **tooling 缺陷信號**，⛔ 不是產品發現，⛔ 不需要保住全差集。⚠️ **v26（✅ 已隨 v28／③ v12 確認（2026-09-23））**：replay 以**專屬結束碼** `EXIT_COUNTERFACTUAL_INEFFECTIVE = 6` 結束，並在 operational 目錄留下 **bounded diagnostics 中繼檔**——⛔ 否則它與其他失敗一樣是 rc=1，orchestrator **分不出要不要發布 failed record** |
| `EXIT_CANDIDATE_MISMATCH = 4` | ⛔ **本路徑⛔ 不會合法產生 rc=4**。⚠️ 常數與既有測試**保留**（⛔ 不刪碼——它仍是 Stage 2 一般路徑的契約），但**counterfactual 執行若出現 rc=4，一律當成實作缺陷**、⛔ **不得判為分支 C** |
| 計次 | ✅ **已隨 v25 確認**：patch 失效導致的中止⛔ **不計入正式 scan**——比照「preflight 與指紋檢查失敗⛔ 不計入這一次」（見「正式 scan 的計次裁決」），理由相同：**工具還沒就位，⛔ 不是驗證跑過了**；⚠️ **但⛔ 不得靜默重跑**——必須產出下方的 **failed-attempt record**。⚠️ **v26（✅ 已隨 v28／③ v12 確認（2026-09-23））**：這條延伸與重跑次數的上限**改寫進「正式 scan 的計次裁決」**——⛔ 原本只寫在這裡，而那一節仍寫「⛔ 不得再有第四趟」，兩者互相衝突 |

⛔ **「保存事故紀錄」與「反事實生效條件不成立時⛔ 無正式 archive」必須同時成立**
（⚠️ v28 訂正：條件是「①之三」的檢查順序得出 `candidate_flag_inconsistent` 或 `rr_not_restored`，⛔ 不只 `before_candidates != ∅`），
所以要有**第三種產物**（⚠️ **細節屬步驟 ③ 的 Stage 2 evidence contract，但本計畫先訂死它是必備輸出**）：

| 項目 | 契約 |
|---|---|
| 名稱 | **failed-attempt record**——⛔ **⛔ 不是成功的 evidence archive**，⚠️ 命名與位置要能一眼分辨 |
| 內容 | 兩份 patch SHA ＋ 合成 hash、**有界診斷**（計數 ＋ 前 N 個 key ＋ 該列的 `lifecycle_phase`／`setup_rr_qualified`）、run identity、image ID、失敗原因。⚠️ **v27**：失敗原因有兩種、有界診斷依原因分流，sample 另加 `rr_decoupling_candidate`（見「①之三」與 ③ evidence contract「七」） |
| 路徑與 schema | ⚠️ 由步驟 ③ 定義；⛔ **不得**放進成功 archive 的 layout，也⛔ 不得沿用其 manifest |
| durability | ⚠️ 原子發布 ＋ fsync（比照既有 finalizer）——⛔ 它是「這一趟發生過什麼」的唯一紀錄 |
| 綁定 | ⛔ **必須**與兩份 patch SHA、run identity 綁定，⛔ 否則證明不了「重跑用的是修過的 patch」 |
| 允許重跑的條件 | ⚠️ **counterfactual patch 的 SHA 必須與失敗那次不同**，且新一次仍要完整走 ⑦⑧⑨；⛔ **SHA 相同的重跑⛔ 不允許**（那只是重跑同一個 bug）。⚠️ **⑦ 總綱 v1 第一輪 review（✅ 2026-09-30 確認）**：「SHA」改指**語意 SHA**——完整 SHA 含兩個測試檔，只改測試就會換鍵；⚠️ 殘餘：只改產品檔的註解或空白，語意 SHA 仍會變（⛔ 不是語意等價） |
| ⚠️ **守門時機（v22 補）** | ⚠️ **runner／preflight 必須在 replay 之前**就查找「**同 run identity ＋ 同 counterfactual patch SHA**」的失敗紀錄；⚠️ **命中即在 replay 前中止**——⛔ **不得跑完三小時才拒絕**。⚠️ **⑦ 總綱 v1 第一輪 review（✅ 2026-09-30 確認）**：查找鍵改成**語意 SHA** `counterfactual_semantic_sha256`（counterfactual 限於兩個產品檔白名單的 canonical diff，見「⑦ 總綱 v1」的「二」） |
| 發布失敗 | ⚠️ failed-attempt record 自身的發布或 fsync 失敗時的 **recovery 與重跑資格，由步驟 ③ 明確定義**——⛔ 不得留成未定義狀態 |

⚠️ 「三項價格證據也得到 156 筆」可以留作 **sanity check**，
⛔ **不得升格成正式 predicate**。

**①之三、⛔ `before_candidates == ∅` 單獨成立⛔ 證明不了 RR 真的加回去（v26 新增，✅ 已隨 v28／③ v12 確認（2026-09-23））**

⚠️ 這道守門只看 flag。flag 若因實作缺陷**恆為 `False`**、而 RR 其實**沒有**加回 `CONTINUATION`，
before 的 156 列仍會是 `CONTINUATION`，守門卻**空洞地通過**——最後在 comparison 看到「沒有翻轉」，
被判成**分支 C**，把工具缺陷記成產品發現。

⚠️ 而既有的 `validate_diagnostics(side="before")` **刻意不驗等價式**——它的理由是 `ecbc141^` 時代
「展開式需要 `active_bearish_states`」。⚠️ **v3 模型下那個理由已不存在**：before 也跑在 `e1cbbbd` 上，
flag 由**同一段程式**以同一個定義組出。所以 before 側（執行期守門與 before source validator）**必須另外驗兩件事**：

| # | 不變條件 | 抓得到什麼 |
|---|---|---|
| 1 | ⚠️ **沒有任何一列** `lifecycle_phase == "CONTINUATION"` 且 `setup_rr_qualified == false` | **RR 沒加回去**——⚠️ 這才是「patch 生效」的直接證據，⛔ 不依賴 flag |
| 2 | ⚠️ 每一列 `rr_decoupling_candidate == (lifecycle_phase == "CONTINUATION" and not setup_rr_qualified)` | **flag 的計算壞掉** |

⛔ **既有 `validate_diagnostics()` 的公開行為不改**（它是 Stage 0 已驗收的契約：`side="before"` 仍不驗等價式）。
⚠️ **v27**：允許把它內部重構成共用的 row-level 原語（見「三」的 `artifacts.py` 列），⛔ 但對外的判定結果一律不變。
兩條新檢查寫在 Stage 2 專屬的程式裡（執行期在 counterfactual 路徑、證據層在 `stage2_evidence.py`）。

⚠️ **v27：檢查順序與失敗原因**（✅ 使用者裁決 2026-09-23：兩種原因，靠檢查順序互斥）。
⚠️ 關鍵事實：**只要 flag 與等價式一致，「`before_candidates ≠ ∅`」與「有列是 `CONTINUATION` 且
`setup_rr_qualified == false`」就是同一組列**——所以固定順序之後只剩兩種互斥的原因：

| 順序 | 檢查（對全量 13,417 列） | 不成立時的 `failure_reason` | 計數 |
|---|---|---|---|
| 1 | 每一列 `rr_decoupling_candidate == (lifecycle_phase == "CONTINUATION" and not setup_rr_qualified)`（上表第 2 條） | `candidate_flag_inconsistent` | 不一致的列數（正整數） |
| 2 | 第 1 條成立之後，`before_candidates == ∅`（⚠️ 此時等同上表第 1 條） | `rr_not_restored` | 候選列數（正整數） |

⚠️ **兩種原因都是中止、結束碼 6、failed-attempt record**；⚠️ 同時違反時**一律是 `candidate_flag_inconsistent`**
——flag 一旦不可信，由它算出的候選集合也不可信，⛔ 不能當第二種原因的證據。
⚠️ record 的 schema 見 ③ evidence contract（⚠️ 現行版）的「七」。

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
⚠️ **v26 更正**：上面兩句是 `ecbc141^` 基準時的分析。v3 模型下 before 與 after 用**同一個定義**，
⛔ 不再有「展開式」；before 側改驗「①之三」的兩條不變條件（⛔ 仍不改 `validate_diagnostics()`）。

**③ Stage 2 的容量**（⚠️ v1 改錯了路徑，v2 訂正；⚠️ **v18 依 v3 模型大幅縮小**）

⛔ **v1 把 `crossday.py` 當成 Stage 2 的執行路徑，那是錯的**——
`crossday.py` 是 **Stage 1 的 D／D+1 comparator**；Stage 2 真正跑的是 `evaluation.py`：

| 位置 | 行為 |
|---|---|
| `evaluation.py:3232` `load_artifact` | **replay 之前**就完整載入 after artifact |
| `evaluation.py:3403` `after_rows` | replay 之後 **before rows 與 after rows 同時在場**，再建兩份 by_key map |

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

⚠️ **v26 補：上面只算了 replay 程序**。改成兩段式發布（見下方）之後，**finalizer、recovery、
preflight 與環境等價比對都是獨立程序**，各自都要讀全量 artifact：finalizer 至少要讀已錨定的 D+1 after
（單份整份載入實測約 +365～449 MiB）與同量級的 before source——⛔ 照 Stage 1 的整份載入寫法，
必然撞上 crossday comparator 的同一道牆（約 730 MiB，見 [I-117](#i-117stage-1-的-comparatorfinalizerrecovery-整份載入兩份-after-artifact超過-mem-guard)）。
⚠️ 這些程序的記憶體模型由 ③ evidence contract（⚠️ 現行版）的「五之一」定義，
並納入「六、1」的 harness；⛔ 不得靠停 live container 換記憶體。

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

⚠️ **發布仍是「單一 evidence archive 一次性原子發布」**（v11 裁定），
⚠️ **但 v26 改成兩段式**（✅ 已隨 v28／③ v12 確認（2026-09-23））：v11～v25 寫的是「replay 程序自己寫 staging 並 rename」，
⛔ 而 ③ evidence contract（v2 起）已裁定**由 shell 的 `finalize-stage2-evidence.sh` 收進 archive**
——兩份 patch 的合成守門需要 git 與隔離 worktree，⛔ 不能放進 Python 的 evidence 模組（見 ③「四之二」）。
⛔ ③ 當時⛔ 沒有回頭同步本計畫書，兩份文件是兩套模型；v26 統一成下面這一套：

```text id="i074_stage2_publish_flow_001"
階段一：replay（evaluation.py，容器內，⚠️ --i074-counterfactual）
  串流載入 after → 全量 replay → 全量 key 守門
  → before_candidates == ∅ ＋「①之三」兩條不變條件
  → 寫 operational 輸出：before source artifact ＋ comparison ＋ report（156 列全在內）
  → 回 0（⚠️ 此時⛔ 還沒有任何正式證據）
階段二：finalize（finalize-stage2-evidence.sh，⚠️ 由 orchestrator 綁定階段一的輸出）
  隔離 worktree 依 ordered_components 套兩份凍結 patch、重算三個 SHA
  → Python finalizer 完整驗證（信任錨 ＋ 環境見證錨 ＋ 全圖）
  → staging → **rename 成正式 evidence archive**  ← ⚠️ **唯一的 commit point**
  → fsync parent → 回 manifest 的 terminal_outcome（v3 恆為 0）
```

⚠️ **staging 內的任何寫入／驗證／fsync 失敗一律回「一般失敗」**，
⛔ **正式 archive 不存在**——⛔ 不會有半成品被當成證據。

⚠️ **rename 成功⛔ 不等於落盤**。⚠️ **沿用 Stage 1 finalizer 既有的 durability 分流**
（`publish.py` 的 `EXIT_DURABILITY_UNCONFIRMED = 3`、`evidence.py` 的「commit point 之後
parent fsync 失敗⛔ 不刪除」），⚠️ **套用層級是整個 evidence archive**：

| 情況 | 正式 **archive** | 結束碼 | 產生者 |
|---|---|---|---|
| **階段一**任何失敗（載入、replay、守門、operational 寫入） | ⛔ **不存在** | **1** | replay |
| **階段一**：「①之三」的檢查順序得出 `candidate_flag_inconsistent` 或 `rr_not_restored`（⚠️ `before_candidates ≠ ∅` 屬於後者） | ⛔ **不存在**——改由 orchestrator 發布 **failed-attempt record** | ⚠️ **6**（`EXIT_COUNTERFACTUAL_INEFFECTIVE`，✅ 已隨 v28／③ v12 確認（2026-09-23）） | replay |
| **階段二**：rename 之前任何失敗 | ⛔ **不存在**（staging 清掉）；⚠️ **operational 輸出與凍結 patch 保留**，可以**只重跑 finalize**（⛔ 不重跑 replay） | **1** | finalizer |
| rename 成功 ＋ parent dir fsync 成功 | 存在且 durable | **0**（⚠️ **B／C 由判讀器對 comparison 判定，⛔ 不是由結束碼分流**） | finalizer |
| **rename 成功但 parent dir fsync 失敗** | ⚠️ **保留，⛔ 不刪除** | **3**（`EXIT_DURABILITY_UNCONFIRMED`）——出口見「rc=3 的 recovery 契約」 | finalizer |
| archive 已 durable，**只剩輔助 temp 清理失敗** | durable | ⚠️ **仍回 0**——⛔ 不得降成一般失敗；orphan 依清理程序處理。⚠️ **此時已無 staging 可清**——`os.replace` 之後原 staging 目錄**就是**正式 archive | finalizer |

⚠️ **為什麼 finalize 失敗可以只重跑 finalize**：finalize 對同一份 operational 輸入是決定性的，
而且每一次都**完整重驗**（信任錨、全圖、合成守門）——⛔ 它⛔ 不是新的 scan。
⚠️ 但**輸入必須是 orchestrator 綁定的那一份**（⛔ 使用者不得以其他路徑覆寫——Stage 1 第二輪 review 高 1 的同一個教訓）。

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
| 第五道集合檢查對**所有** Stage 2 執行**無條件**生效 | `evaluation.py:3183` 的 `run_bundle_stage()` 內的 `sorted(before_candidates) != sorted(after_candidates)` |

⛔ **所以直接改那段就會連一般路徑一起改掉**——v18 宣稱保留的 rc=4 會變成**不可達**，
⚠️ 那等於用文件宣稱了一條**實際上不存在**的契約。

**裁決（✅ 已隨 v25 確認）：新增 `--i074-counterfactual`，⛔ 預設關閉。**

| 規則 | 內容 |
|---|---|
| 開啟時 | 走 v3 模型：after cohort 156 keys、`before_candidates == ∅`、⛔ **無**集合相等檢查 |
| 關閉時（預設） | ⚠️ **一般 Stage 2 行為完全不變**——集合相等檢查、`candidate_mismatch.json`、rc=4 全部照舊 |
| ⛔ **禁止的實作** | ⛔ **不得從 `before_ref` 值、patch hash 非空、cohort 大小等狀態暗中推斷模式**——⚠️ 模式只能是**明示參數** |
| stage 限定 | ⚠️ **只限 Stage 2**（⚠️ 與既有兩個 flag **相反**，那兩個只限 Stage 1），用同一套 `assert_i074_flags()` 擋 |
| ⚠️ **flag** 要同步的既有機制 | `I074_FLAGS`（`evaluation.py:3527`）、`_reject_duplicate_i074_flags()`（`:3530`）、`BUNDLE_ALLOWED_ARGS`（`:3485`）、`assert_i074_flags(stage=)`（`:3561`）——⛔ **不是只加一個 `add_argument`**；⛔ **flag ⛔ 不進 `SCRIPT_INJECTED_ARGS`**（它是使用者 opt-in） |
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

✅ **`I074_MODE` 裁決：納入**——`run-replay-offline.sh:65` 的 `I074_MODE` 偵測要加上這個 flag。
⛔ **不納入的話，正式 Stage 2 反事實執行反而繞過既有的 run identity 守門**——
⚠️ 那是三趟必須跑在同一個 image 上的唯一保證。
⚠️ **v26**：Stage 1 的 image 已遺失，「同一個 image」改成**after' 見證趟與 before 共用的新 image**，
由**唯一固定的 Stage 2 identity** 保證（見「二、⑤」）——⚠️ 納入 `I074_MODE` 的理由不變。

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

**⑤ ⛔ 執行環境遺失：Stage 1 釘住的 image 已不在本機（v26 新增，⚠️ 本計畫書的第三決策點）**

| 事實 | 值 |
|---|---|
| Stage 1 綁定的 image | `sha256:d66030dca485…`——`python/baselines/i074_stage1/` 的 manifest（`expected_image_id`）、identity 與所有 provenance 都記這個值 |
| 2026-09-23 查證 | `docker image inspect` 回 `No such image`；repo 外也找不到 `docker save` 的備份 |
| XDG identity 檔 | **仍在**（`~/.local/share/stock_trading/i074_stage1/run_identity.json`） |
| `pin-replay-image.sh` 的行為 | identity 還在、image 已不在 → **fail-closed**（⛔ 不得重建後換 ID）。⚠️ **這是正確行為**，但它讓 `I074_MODE` 的 Stage 2 **跑不起來** |
| identity 路徑 | `default_run_identity_path()`（`run_identity.py`）**寫死 `i074_stage1`**——Stage 2 要換新 identity 就必須改 |
| ⚠️ 可能成因（⛔ **未查證是哪個操作刪的**） | pin 用的 tag 是 `stock-trading-python-test:latest`（`pin-replay-image.sh` 的 `IMAGE=`），與 `python/scripts/test.sh`、`smoke-replay-offline.sh`、`run-evaluation.sh`、`build-selection-report.sh`、`verify-regression-baseline.sh` **共用**——任何一次重新 build 都會讓釘住的 ID 失去 tag，之後被 prune 清掉 |

⛔ **不修這個流程，新 pin 的 image 也會再丟一次**——③／⑦ 實作期間 `test.sh` 會跑很多次。

**影響**：

* ③ v9 的「Stage 2 identity 與 Stage 1 archived identity 逐位元相同」（信任錨第 10 道、F2-a）**無法成立**；
* 「唯一產品語意變因是 setup RR 條件」的前提是**同一個執行環境**——`requirements.txt` 是下限釘法
  （[I-116](#i-116凍結-bundle-的可重現性只靠image-還在requirementstxt-是下限釘法bundle-也沒記訓練時的套件版本)），
  重建的 image 會裝到不同版本；`model.joblib` 是 pickle，sklearn 版本不同就可能給出不同的預測。

✅ **使用者裁決（2026-09-23）：兩側都在新 image 重跑**，做法如下（細節✅ 已隨 v28／③ v12 確認（2026-09-23））：

| 項目 | 做法 |
|---|---|
| 新 image | ⚠️ **專用 tag**（例如 `stock-trading-python-replay:i074-stage2`，⛔ 不與任何 build 腳本共用）；pin 之後立刻 `docker save` 成 tarball，**放在 repo 外**，記下 tarball 的 SHA-256 與還原程序（⚠️ `docker load` 之後 image ID 不變）。⚠️ **v28（review 附帶條件）**：正式操作文件要**寫死保存位置與驗 SHA 的步驟**——建議位置與 identity 同一個 XDG 基底（`…/stock_trading/i074_stage2/images/`，⛔ 不放 `/tmp`），還原前先驗 tarball 的 SHA-256、`docker load` 之後再驗 image ID 等於 identity 的 `expected_image_id`，任一不符即中止 |
| 套件清單 | ⚠️ 記錄**完整的 distributions 清單**（⛔ 不只 `pip_freeze_sha256`），由環境等價比對**在同一個 image 內**產出，並與 hash 交叉驗證——⚠️ 部分補上 I-116「偵測 ≠ 可重建」的缺口 |
| 💡 build 的建議 | 先在容器內算 `stock_trading-python-server:latest`（與遺失的 image 同一份 Dockerfile、只晚 16 分鐘 build）的 `pip_freeze_sha256`；若等於 Stage 1 provenance 的 `7a39573e…`，就用它的清單**精確釘版**來 build 新 image，提高等價的機會。⚠️ **這只提高機率，⛔ 不取代下面的見證趟**。✅ **2026-09-23 實測相等，使用者裁決改為直接採用該 image**（`pin-replay-image.sh --stage 2 --adopt-image`，⛔ 不重新 build），見「③c 準備」 |
| identity | ⚠️ **依 stage 固定推導**：`…/stock_trading/i074_stage2/run_identity.json`（⛔ 不開放任意路徑覆寫，沿用「測試改覆寫 `XDG_DATA_HOME`」的慣例）；`bundle_id` 與 Stage 1 相同、`expected_image_id` 是新 image。⚠️ **建立一次，after' 與 before 都用它**——identity 仍然⛔ 不是變數 |
| **after' 見證趟** | 新 image ＋ **原始 `e1cbbbd`** ＋ 空的 tooling patch，**Stage 1 模式**（含 `--i074-preflight`）跑同一份 bundle，產出 after' 與 cohort'。⚠️ **它只是見證**：⛔ 不是新的正式 Stage 1 scan，⛔ 結果不得取代 D+1 |
| 環境等價判定 | 見下表，**事前寫死** |
| 分流 | **EQUIVALENT** → 才進 before 正式趟；**NOT_EQUIVALENT** → 在 before **之前**停止、封存見證證據、另立 issue（結束碼 `EXIT_ENV_NOT_EQUIVALENT = 7`，✅ 已隨 v28／③ v12 確認（2026-09-23））；⛔ **不得改用 after' 取代 D+1**——那等於看到結果之後換掉正式 Stage 1 artifact |
| 證據 | ⚠️ **獨立的小 archive** `python/baselines/i074_stage2/envcheck/`，比對一完成就原子發布（⛔ 不讓 operational 檔案在後續實作期間躺好幾天）；Stage 2 archive **以它的 manifest 為第二個信任錨**（⛔ 不複製成員）——規格見 ③ evidence contract v10「三之三」 |

**環境等價判定**（⚠️ **事前寫死**，⛔ 不得看到結果再調）：

| # | 條件 | 規則 |
|---|---|---|
| 1 | 全量 rows | 兩側的 13,417 個 key 集合相同，且**逐 key 的 canonical row bytes 完全相同**（⛔ 不挑欄位） |
| 2 | cohort | cohort' 的 keys **等於** Stage 1 D+1 cohort 的 156 keys |
| 3 | 必須相同的 provenance 欄位 | `base_commit`、`tooling_patch_sha256`、`project_modules_sha256`、`runtime_settings` |
| 4 | 允許不同、但逐欄記錄 | `image_digest`、`pip_freeze_sha256`、`python_version`、`runner_sha256`、`argv`、`source_root`（⚠️ 與第 3 列合起來**恰好是 10 欄**，⛔ 不留未分類的欄位） |
| 5 | 結果 | 1～3 全部成立 → **EQUIVALENT**；任一不成立 → **NOT_EQUIVALENT** |

⚠️ **為什麼這樣就能繼續用封存的 D+1**：兩個 image 對這份 bundle 的每一列輸出**逐位元相同**，
環境差異對本次驗證就⛔ 沒有可觀測的影響；before 與 after' 又跑在同一個新 image，
所以 before 與 D+1 之間的差異仍然**只能**來自 counterfactual patch。

⚠️ **比對必須串流**：⛔ 不得兩份整份載入（那正是 I-117 的 730 MiB）；⛔ 也不改 `crossday.py`
——它的 `outcome` 要求 provenance 無差異，語意與本判定不同。新增比對模組，記憶體目標 < 450 MiB。

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

⚠️ **v26**：兩段式之下 rc=3 只會由**階段二的 finalizer** 產生，recovery 走
`finalize-stage2-evidence.sh` 的 **recover-durability** 模式（CLI matrix 見 ③ evidence contract（⚠️ 現行版）的「七之四」），
⚠️ 並且**重做 shell 端的合成守門**（與 failed record 的 recovery 對稱）。

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

⚠️ 它確實也有 730 MiB 的問題（實測 OOM），**另立一筆處理**——⚠️ 2026-09-23 才補立為 [I-117](#i-117stage-1-的-comparatorfinalizerrecovery-整份載入兩份-after-artifact超過-mem-guard)。
⚠️ 真要一起做就⛔ **不能叫「另列」**，而且必須明訂演算法——
⛔ **不是同步 `zip()`**（插入／刪除／換序會整個錯位），
而是能維持**排序交集**與**兩側差集 row** 的具體做法。

##### 二之二、⚠️ 磁碟與中止政策（⚠️ **v18 移除 spool 之後重新盤點**）

⛔ **v5～v15 的 disk-backed spool 已移除**（見「二、③」）。⚠️ **仍然要有磁碟政策**——
evidence staging 內同時存在 **before source artifact ＋ comparison ＋ report 的 temp 與正式檔**，
⚠️ before source artifact 本身就是**全量 13,417 列**的量級。

⚠️ **v26 補（兩段式與見證趟之後，`P_B` 要涵蓋的東西變多了）**：

| 同時存在的內容 | 量級 |
|---|---|
| 階段一的 operational 輸出（before source ＋ comparison ＋ report，**未壓縮 JSON**） | before source 約 88 MiB（與 after 同量級） |
| 階段二的 staging（同樣內容的 **gzip** ＋ 兩份 patch ＋ manifest） | 約 7 MiB 量級 |
| 見證趟的 operational 輸出（after' ＋ cohort'）與 `envcheck/` 小 archive | after' 約 88 MiB；archive 約 7 MiB |
| ⚠️ **新 image 的 tarball**（repo 外） | 數百 MiB～GiB 量級——⚠️ **不在 `P_B` 內**，但 pin 步驟要自己做事前檢查 |

⚠️ 所以「事前檢查」要在**兩個時點**各做一次：見證趟之前、before 正式趟之前；公式仍是
~~`P_B ＋ M_safety`~~ → ⚠️ **v29：`P_B_BUDGET ＋ M_safety`**（`P_B_BUDGET` 是寫死的預算；sizing harness 依上表**實測**的 `P_B` 必須 ≤ 它，⛔ 不用上表的估計值代入）。
⚠️ **v29 訂正**：見證趟之前那一次**⛔ 不可能做到、實際也沒有做**——③c 排在 ⑤ 之前，而下方的時序規定「⑤ 定案之前⛔ 不得實作
preflight」。③c 已於 2026-09-23 成功完成，⛔ 不補做；**只剩 ⑩ 之前那一次**，公式裡的 `P_B` 用預算 `P_B_BUDGET`（見 v29）。

| 項目 | 政策 |
|---|---|
| **事前檢查** | ⚠️ **replay 之前**就檢查可用空間，⛔ 不要跑完三小時才因 `ENOSPC` 失去證據。**公式固定為一條**：⚠️ **v29：`required = P_B_BUDGET ＋ M_safety`**（⚠️ **v18 移除 `P_C`**——mismatch 路徑不存在了，⛔ 不再有第二條峰值）。`P_B` 由 sizing harness 實測（⚠️ 只用來驗證 ≤ `P_B_BUDGET`，⛔ 不直接進公式），`M_safety` 為寫死常數；⚠️ ~~兩者目前都是占位符~~ → ⑤（2026-09-29，✅ review 通過）實測 `P_B` = 158,101,504 bytes（150.8 MiB）、裁定 `M_safety` = 1,073,741,824 bytes（1 GiB，固定）；⚠️ **v29（⑥）：程式裡寫死的是預算 `P_B_BUDGET` = 167,772,160 bytes（160 MiB）**，`required` = **1,241,513,984 bytes**——位置、量法與前提見 v29「磁碟檢查」；決定時序見下 |
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
⚠️ **v29 對照**：上面的 ①② ＝ 執行順序的 ④（✅ 2026-09-29）、③ ＝ ⑤（✅ 2026-09-29）、④ ＝ ⑥（v29，✅ 2026-09-29 確認）；preflight 在 ⑦b 實作（見「Stage 2 步驟 ⑦ 總綱 v1」）。

##### 三、受影響檔案與資料流

| 檔案 | 改動 | 風險 |
|---|---|---|
| **`evaluation.py`** ⬅️ **最主要** | Stage 2 的 after loader 改**一趟串流**；⛔ 不再完整保留 13,417 筆 after rows，只常駐頂層 ＋ 全量 keys ＋ **156 筆 cohort rows**；移除「兩側 candidate 集合相等」那道檢查，改成 `before_candidates == ∅` | ⚠️ 這是**真正的 Stage 2 執行路徑**，v2 漏列。⚠️ **⑦ 總綱 v1（✅ 2026-09-30 確認）**：改動**在 HEAD 實作與測試**，但 ⑩ 的 replay 執行的是 `e1cbbbd` worktree，所以要**經 tooling patch 進入 replay**（見下方 tooling patch 一列） |
| `replay_bundle/artifacts.py` | ⚠️ **v27 改寫**：抽出 **row-level 共用原語**（單列的 key、候選 flag、replay errors、九欄位與交叉一致性），既有的**整批 validator** 與新的**串流 validator** 都呼叫同一套，**單趟組合**逐列套用。⚠️ 現況：`validate_diagnostics()`／`validate_replay_errors()`／`validate_candidate_flags()` 已接受 `Iterable`，但各自讀一遍 rows；`validate_after_artifact()` 要吃整份 dict——所以要抽的是「單列檢查」與「單趟組合」 | ⛔ **驗證強度不得下降**；⚠️ **公開 contract 與 Stage 1 行為不變、允許內部重構**；⚠️ **Stage 1／Stage 0 既有測試⛔ 不修改斷言、全數續跑**；⚠️ `validate_candidate_mismatch()` ⛔ **不動**（v18 移出範圍） |
| Stage 2 streaming loader 所在模組 | 新增／調整（`replay_bundle/` 底下） | ⚠️ 要能**重新開啟**，⛔ 不是一次性 iterator；⛔ **v18 已無 spool**。⚠️ **⑦ 總綱 v1（✅ 2026-09-30 確認）**：`replay_bundle/` 同樣經 tooling patch 進入 replay |
| ⚠️ **tooling patch 與產生器**（**⑦ 總綱 v1 新增，✅ 2026-09-30 確認**） | `python/baselines/i074_stage2/tooling_e1cbbbd.patch`（進版控）＋ `scripts/make-i074-tooling-patch.sh`：範圍限 `evaluation.py` ＋ `replay_bundle/`，由 `e1cbbbd`＋counterfactual 的 `T1` 產生 canonical 的 `T1..T2` diff；⚠️ 漂移測試、產品碼不變條件、⑨ 與 counterfactual 一起封存 | ⛔ 不得含判定變更——由 ⑨ 的 tooling 非語意 guard 與 differential guard 驗 |
| **memory harness** | 新增（13,417 列 before rows ＋ cohort 常駐 ＋ 發布路徑） | ⚠️ 在釘死的 image／cgroup 內跑，⛔ 不是一般 CI 測試；⛔ **v18 只有一條路徑（B）**，⛔ 不再有 worst-case mismatch 情境 |
| **Stage 2 evidence contract**（含 **before source artifact** ＋ **counterfactual patch** ＋ tooling patch） | ⚠️ **外部硬性前置，排在實作之前**（執行順序 ③） | ⛔ 在它確認前，⛔ 不得決定發布順序、schema、patch 版控位置或 manifest 寫法 |
| Python 測試 | `tests/test_replay_bundle_stages.py`、`tests/test_i074_mismatch.py` | 測試矩陣見「六、2」；⚠️ `test_i074_mismatch.py` 的既有案例⛔ **原樣保留**（它釘的是一般路徑） |
| shell 測試 | `scripts/test-replay-args.sh` | Stage 2 的 argv／mount |
| **counterfactual patch** | ⚠️ 對 **`e1cbbbd`** 施加：把 setup RR 條件加回 `CONTINUATION`（跨 lifecycle 參數、呼叫端、判定式與測試） | ⚠️ **patch 本體要進版控**，⚠️ **與 tooling patch 分開記錄**，見「三之一」「三之二」 |
| `scripts/run-replay-offline.sh` | ⚠️ **兩份 patch 的輸入與固定套用順序**（⛔ v18 寫「既有機制、不新增參數」**是錯的**，見「三之一之二」） | ⛔ 只有一個 `TOOLING_PATCH`（`run-replay-offline.sh:86`），⛔ 現行做不到 |
| `scripts/lib/replay-args.sh` | ⚠️ **增量 diff SHA**（⛔ 不可兩次都對 base 取 diff）＋ 新的 `--counterfactual-patch-sha256` 注入 | ⛔ 現行 `replay-args.sh:409` 的 `replay_args_tooling_patch_sha256()` 只算合併後的一份。⚠️ **⑦ 總綱 v1（✅ 2026-09-30 確認）**：抽出**唯一的共用合成函式**（tree-to-tree、「⑦ 總綱 v1」「二」的 canonical 定義與語意鍵），runner、`finalize-stage2-evidence.sh` 的 `compose_check`／`--check-failed-record`、tooling patch 產生器都呼叫它 |
| `evaluation.py` 的 CLI parser | ⚠️ **`--i074-counterfactual` opt-in**（⛔ 預設關閉）＋ `I074_FLAGS`／`BUNDLE_ALLOWED_ARGS`／`SCRIPT_INJECTED_ARGS`／`assert_i074_flags()` 五處同步 | ⛔ **不加這個 flag 就會連一般 Stage 2 一起改掉**，見「二、④」 |
| `scripts/test-replay-args.sh` ＋ argv fixture | ⚠️ flag、兩份 patch SHA、CLI ownership、重複參數、stage 限定 | ⚠️ 目前⛔ 無 Stage 2 argv fixture（只有 stage0／stage1／comparator／finalizer） |
| ⚠️ **`scripts/run-i074-stage2.sh`**（v26 新增，屬 ⑦） | **orchestrator**：凍結兩份 patch → preflight → replay → 依結束碼分流（0 → finalize；6 → 發布 failed-attempt record；其他 → 停）；**自己綁定**階段一的輸出交給 finalizer，⛔ 使用者不得以其他路徑覆寫；⚠️ **凍結 patch 放在 run 目錄底下**，保留到 finalize rc=0 或 failed record 發布完成才清，finalize 重試沿用同一份 | ⛔ 沒有它，「比較本次、封存另一份」的 Stage 1 第二輪 review 高 1 會在 Stage 2 重演 |
| ⚠️ **supervisor**（v29 第十二、十三輪新增，屬 ⑦；⚠️ 檔名與測試落點在 ⑦ 的計畫書定——⚠️ **⑦ 總綱 v1（✅ 2026-09-30 確認）**：`scripts/lib/i074-stage2-supervisor.py`，host 端標準庫、相容 Python 3.9；測試在 `scripts/test-i074-stage2.sh` ＋ host 端 `unittest`） | host 端、只用標準庫：驗自身內容 ＝ HEAD、host 層級的鎖、active-run sentinel、child subreaper、以 parent-death signal 啟動子程序、依 label 與 pid 樹清空本趟、釋放條件、衝突時依模式回 1／8／9；固定執行帳號、鎖檔與 sentinel 的屬性驗證；⛔ 沒有解除入口（「八之一之二」） | ⚠️ 它是「唯一一次正式趟」的並行守門——n7b 的每一支都要有 |
| ⚠️ **容器 label 的注入點**（v29 第十三輪，屬 ⑦；⚠️ docker shim 或修改 runner／finalizer 二擇一，在 ⑦ 的計畫書定——⚠️ **⑦ 總綱 v1（✅ 2026-09-30 確認）**：採 **PATH docker shim** `scripts/lib/i074-stage2-docker-label-shim.sh`，runner／finalizer ⛔ 不改） | ⑩ 的每一個 `docker run`／`docker create` 恰好帶一個 `i074.stage2.run=<token>`；拒絕使用者自帶同一個鍵 | ⛔ 不得有兩個注入點並存；argv 測試見 n7b |
| ⚠️ **stage-scoped identity 與專用 tag**（v26 新增，屬 ③b） | `run_identity.py` 的 `default_run_identity_path()` 依 stage 固定推導；`pin-replay-image.sh`、`ensure-i074-run-identity.py`、`validate-i074-run-identity.py`、`run-replay-offline.sh` 跟著支援 Stage 2 identity；pin 改用**專用 tag** ＋ `docker save` | ⚠️ **Stage 1 的 identity 與行為⛔ 不得改變**（它的 fail-closed 是正確的） |
| ⚠️ **環境等價比對模組**（v26 新增，屬 ③b；建議 `replay_bundle/envcheck.py`） | 串流比對 after' 與 D+1、產出 equivalence artifact、`EXIT_ENV_NOT_EQUIVALENT = 7`；⛔ 不改 `crossday.py` | ⚠️ 與 Stage 2 的一趟串流 loader **共用同一套串流讀取**，⛔ 不各寫一份 |
| ⚠️ **B／C 判讀器**（v26 新增，⑩ 之前；建議 `replay_bundle/stage2_verdict.py`） | 讀**已封存的** comparison artifact，依「關閉條件」的判讀矩陣逐列判 B／C，輸出每列的判定與理由；⚠️ **判準在 ⑩ 之前就寫成程式並測過** | ⛔ 否則判準等於看到結果之後才定 |
| ⚠️ **differential guard**（v26 補落點，屬 ⑨；建議 `python/scripts/i074-differential-guard.py` ＋ 對應測試） | 在**原始 `e1cbbbd`** 與 **patched `e1cbbbd`** 兩個 worktree 各跑同一份 fixture、各自輸出 decision summary，再由第三個程序依「六、3」的 allowlist 比對 | ⚠️ fixture ⛔ 不得是 patch 一併修改的那些 |

⛔ **v18 移除的受影響檔案**：`ecbc141^` 的 bundle CLI 移植（基準換成 `e1cbbbd`，
CLI 與九欄位本來就在）、disk-backed spool 模組、串流 mismatch validator。

**資料流**（⚠️ v26 改寫）：

1. **環境閘門**：新 image ＋ 原始 `e1cbbbd` → after' 全量 replay → 與 D+1 串流比對
   → `envcheck/` 小 archive（EQUIVALENT 才繼續）；
2. **階段一**：新 image ＋ `e1cbbbd` worktree ＋ **counterfactual patch** → 全量 13,417 列 → before rows
   → 全量 keys 守門 → `before_candidates == ∅` ＋「①之三」守門 → 取 after cohort 的 **156 keys** 逐列對照
   → operational 的 before source artifact ＋ comparison ＋ report；
3. **階段二**：orchestrator 綁定階段一的輸出 → `finalize-stage2-evidence.sh` 合成守門 ＋ 完整驗證
   → 一次性原子發布成 Stage 2 evidence archive → **rc=0**；
4. **判讀**：B／C 判讀器讀封存的 comparison artifact 判定。

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
| 只有**一個**輸入 `TOOLING_PATCH`（`run-replay-offline.sh:86`） | ⛔ 沒有第二份 patch 的位置 |
| 只套用一次、只算**一個**合併後的 `TOOLING_PATCH_SHA256`（`run-replay-offline.sh:238`；⚠️ ⑦a 起已改成兩份 patch 與三個增量 SHA，見「⑦a 細部計畫」B5） | ⛔ 無法分離兩份 patch 的貢獻 |
| 該 SHA 的定義是「**套完之後整個 worktree 相對 base 的 `git diff --binary`**」 | `replay-args.sh:409` 的 `replay_args_tooling_patch_sha256()`（⚠️ ⑦a 起改成 canonical diff，見「⑦a 細部計畫」B1） |
| provenance 只有一個 `--tooling-patch-sha256` | `replay-args.sh:65` 的 `replay_args_offline()` |

⛔ **所以 v18 受影響檔案表寫的「既有機制、⛔ 不新增參數」是錯的**（v19 已改）。
⚠️ 而且照現行定義**直接算第二次，第二份 SHA 會包含第一份的差異**——那正是要避免的事。

⛔ **⛔ 不可用「中繼 commit」切分**（v20 訂正 v19）：建 commit 會**動到 detached worktree 的
HEAD**，與 `replay-args.sh:154` 的 `replay_args_prepare_worktree()` 契約直接衝突
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
（`provenance.py:209` 的 `validate_provenance()`：多欄、缺欄一律中止）。
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
| **Stage 2 的結束碼** | ⚠️ **v3 之下正常路徑恆為 0**；rc=1／rc=3 照舊；⛔ **rc=4 ⛔ 不會在本路徑合法出現**。⚠️ **v26（✅ 已隨 v28／③ v12 確認（2026-09-23））**：新增 **6**（`EXIT_COUNTERFACTUAL_INEFFECTIVE`，階段一的 patch 失效）；rc=3 改由**階段二的 finalizer** 產生（見「二、③」的兩段式表）  ⚠️ **v29（第六輪）**：新增 **8**（`EXIT_PROMOTION_FAILED`，晉升操作的錯誤；⛔ 不與終態的 rc=1 共用），並定義 orchestrator 的**端到端結束碼**（0／6／2／1／3／8／9；9 ＝ `EXIT_PROMOTION_BLOCKED`）與單一、冪等的 `--promote`（「八之三」；⚠️ 第七輪撤回 `result.json`、第八輪撤回 `--status`／`--recover-promotion`） |
| ⚠️ **環境見證**（v26） | ⚠️ **新增 equivalence artifact 與 `envcheck/` 小 archive**（schema 由 ③ evidence contract（⚠️ 現行版）的「三之三」定義）；新增結束碼 **7**（`EXIT_ENV_NOT_EQUIVALENT`）。⛔ 既有 replay artifact 的 schema **不變**——after' 就是一份普通的 after artifact |
| ⚠️ **run identity**（v26） | ⚠️ 路徑改成**依 stage 固定推導**，新增 Stage 2 的 identity 檔；⛔ **Stage 1 的 identity 檔、schema 與 pin 行為一律不變** |

##### 五、風險與回滾

| 風險 | 對策 |
|---|---|
| ⚠️ 串流後 after 的驗證強度下降 | ⛔ **`validate_after_artifact()`／`validate_diagnostics()` 的每一項都不得放寬**；測試見「六、2」 |
| ⚠️ 兩個 validator **各自遍歷一次 rows** | 改成 iterator 後⛔ 不能重複消費。要嘛定義**單趟複合驗證**，要嘛提供**可重新開啟的 iterator factory**——⛔ 不得因為改串流而少驗一道。⚠️ **v27 裁定：單趟複合驗證**，建立在 row-level 共用原語上（見「三」） |
| ⚠️ **重構 `artifacts.py` 的 validator 改變了 Stage 1 行為**（v27） | ⚠️ 公開函式的簽章、例外型別與錯誤判定一律不變；⚠️ Stage 0／Stage 1 既有測試⛔ 不修改斷言、全數續跑；⚠️ 另對已封存的 `python/baselines/i074_stage1/` 跑一次既有 validator，結論必須與重構前相同 |
| ⚠️ 逐列驗證漏掉整份層級的檢查 | 保留 `bundle_id`／`kind`／`schema_version` 等頂層驗證，只把 rows 那段改串流 |
| ⛔ **counterfactual patch 改到 RR 以外的語意** | ⚠️ **本輪最高風險**（它刻意改判定，⛔ 不是純 instrumentation）。對策：diff 逐行可讀、⚠️ **唯一產品語意變因是 setup RR 條件**、套用前後 `e1cbbbd` 既有測試全綠、**獨立 SHA 進 Stage 2 evidence manifest**（⛔ **不是** provenance——⛔ 不得加第 11 欄，見「三之一之二」）並與 `tooling_patch` 分開 |
| ⛔ **patch 沒真的生效**（RR 沒被加回去） | ⚠️ `before_candidates == ∅` 的守門會抓到；⛔ 非空即中止、⛔ **不得當成分支 C**；⚠️ 此中止**⛔ 不計入正式 scan**（✅ 已隨 v25 確認；重跑上限見「正式 scan 的計次裁決」的 v26 修訂）。⚠️ **v26**：flag 本身壞掉時這道守門會空洞通過，所以另加「①之三」的兩條不變條件 |
| ⚠️ before 側有列缺席 | 全量 13,417 keys 守門**保留**——⛔ 只用 cohort 過濾會靜默漏列 |
| ⚠️ 156 列比較結果被截斷或抽樣 | ⛔ **不截斷、⛔ 不抽樣**（不變條件 e）；承「⛔ 不接受 aggregate 當命中證據」 |
| ⚠️ before source artifact 讓磁碟或記憶體超標 | sizing harness 量 `P_B`、memory harness 驗 < 450 MiB，**兩者都在正式執行之前** |
| ⚠️ evidence archive 半成品被當成證據 | staging 內任何失敗一律 rc=1 且⛔ 無正式 archive；orphan 命名可辨識 |
| ⛔ **模式旗標缺席，一般路徑被連帶改掉** | ⚠️ **v18 的高風險缺口**：rc=4 會變成不可達。對策：`--i074-counterfactual` opt-in ＋ **測試 o「未帶 flag 時逐項不變」**；⛔ **不得靠 ref／hash 暗中推斷模式** |
| ⛔ **兩份 patch 的 SHA 互相污染** | ⚠️ 現行 SHA 定義是「整個 worktree 對 base 的 diff」，直接算兩次⛔ 一定會包含前一份。對策：**固定順序 ＋ 中繼 tree（`write-tree`）＋ 增量 diff**；⚠️ 順序由 runner 結構與 manifest 的 ordered components 強制，⛔ **不靠「hash 必然不同」**（測試 t） |
| ⚠️ counterfactual 在 replay artifact 裡看起來像純 instrumentation | ⚠️ 採方案 (i) 的**已知殘留風險**。對策：evidence manifest **必須**斷言合成 hash 等於 artifact 的 `tooling_patch_sha256`；⛔ 無 manifest 的 counterfactual 執行⛔ 不得採信 |
| ⛔ **新 pin 的 image 又被清掉**（v26） | ⚠️ 釘住的 ID 失去 tag 就會被 prune 清掉——Stage 1 就是這樣丟的（成因未查證，但 tag 共用是已知風險）。對策：**專用 tag**（⛔ 不與任何 build 腳本共用）＋ **repo 外 tarball**（記 SHA 與還原程序） |
| ⛔ **新 image 與舊 image 不等價**（v26） | ⚠️ 事前寫死的**環境等價判定**；⛔ 不等價就停在 before 之前、另立 issue；⛔ **不得改用 after' 取代 D+1**，也⛔ 不得放寬判定規則後重比 |
| ⚠️ **finalizer／recovery／preflight 整份載入而 OOM**（v26） | ⚠️ 記憶體模型由 ③ evidence contract（⚠️ 現行版）的「五之一」定義（串流、只常駐 keys 與 156 筆 cohort rows），並納入「六、1」的 harness；⛔ 不得停 live container |
| ⚠️ **orchestrator 綁錯輸出**（v26） | ⚠️ 階段一的輸出由 orchestrator **自己綁定**，⛔ 使用者不得覆寫；finalize 失敗只重跑 finalize、⛔ 不重跑 replay |
| **回滾** | 串流是純讀取路徑的改寫，`git revert` 即可；counterfactual patch 本來就不進主線。⚠️ **v26**：stage-scoped identity 與專用 tag 的改動同樣 `git revert` 即可，⛔ Stage 1 的 identity 與已封存證據不受影響 |

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
   | d | **before source artifact ＋ comparison ＋ report 的 evidence staging 與一次性發布**（⚠️ **v26：屬階段二的 finalizer 程序**） |
   | e | SHA、validator 與 **temp 清理** |
   | ⚠️ f | **v26：finalizer 程序**——串流讀已錨定的 D+1 after、before source、comparison，完成信任錨 ＋ 環境見證錨 ＋ 全圖驗證 |
   | ⚠️ g | **v26：recovery 程序**（成功 archive 與 failed record 各一） |
   | ⚠️ h | **v26：preflight 程序**（信任錨兩支 ＋ failed-record lookup） |
   | ⚠️ i | **v26：環境等價比對程序**（after' 與 D+1 兩份全量 artifact） |

   ⚠️ **⑦d 細部計畫 v1（✅ 2026-10-02 確認）**：a～c 的**計算工作集**由 ⑨-1 的量測趟（`--replay-compute full`，⛔ 不套 counterfactual）涵蓋——stub 只用於開發驗證與兩條磁碟路徑；h 量 ⑩ preflight 的真實程式 `--check-failed-record`（`anchors` 以它代量）；i 先以封存的見證輸出重新發布 envcheck（`--envcheck`）、再量 `--recover-envcheck`；晉升（promoter 的 host 程序樹與驗證模式的容器）也納入 < 450 MiB；host 端以**最大單一程序 RSS**為門檻、程序群組的取樣總和只作單向警報。

   ⚠️ **v26：門檻對每一個程序各自成立**——⛔ 不是只量 replay。
   ⛔ **Stage 1 的 comparator 就是只量了 replay、沒量 comparator，才在正式執行時撞到 730 MiB**
   （見 [I-117](#i-117stage-1-的-comparatorfinalizerrecovery-整份載入兩份-after-artifact超過-mem-guard)）。

   ⛔ **v18 移除 harness 的 worst-case mismatch 情境與 `P_C` 路徑**——觸發條件已不存在
   （見「二、①」）。於是磁碟只剩**一條**峰值：

   ```text id="i074_disk_formula_001"
   required = P_B_BUDGET ＋ M_safety        ← ⚠️ v29；v18～v28 寫的是 P_B ＋ M_safety
   ```

   `P_B_BUDGET` 是**寫死的預算**，sizing harness 實測的磁碟峰值 `P_B` 必須 ≤ 它；
   `M_safety` 是**寫死的固定餘裕**（⛔ 不得執行期解讀）。
   ⚠️ **v29（⑥）**：`M_safety` = 1,073,741,824 bytes（1 GiB）；程式裡的 `P_B` 是**預算** `P_B_BUDGET` = 167,772,160 bytes
   （160 MiB，⑤ 實測 158,101,504 bytes），`required` = 1,241,513,984 bytes。⚠️ **回退順序（併入 ④ 計畫書「五」，比較對象改成預算）**：
   ⑨ 之後的正式 memory／disk acceptance（⑨-1）量到的實際流程磁碟峰值、或 ⑩ 之前的 `--formal` 確認重跑（⑨-2），**任一 > `P_B_BUDGET`**（重跑另需
   `status = "ok"`）→ ⛔ 不得進入 ⑩ → 更新 `P_B_BUDGET`（`M_safety` 維持，除非另行裁定）→ 重跑 ⑤ → 回到 ⑥ 確認。
   ⚠️ memory harness 再驗磁碟峰值時，要一併確認 ④ 計畫書「五」的 `--read-only` 落差（⑩ 不加 `--read-only`，寫到別處就是
   sizing 沒量到的用量）。
   ⚠️ **⑦ 總綱 v1（2026-09-29，✅ 2026-09-30 確認）**：⑤ 量 `P_B` 時 **tooling patch 是空的**（`i074-stage2-sizing.sh` 凍結 tooling 時
   一律 `: > tooling.patch`，建的是 0-byte 檔），而 ⑩ 要套非空的 tooling patch（`evaluation.py` ＋ `replay_bundle/`）——worktree 會變大。差距**⛔ 不回頭重跑 ⑤**，
   由 ⑨-1（兩份封存 patch 的實際流程）與 ⑨-2（真實兩份 patch 的 `--formal` 重跑）驗 ≤ `P_B_BUDGET`；⚠️ 預算對 ⑤ 的實測只剩約 9.2 MiB 餘裕，
   超過就走上面的回退順序。
   ⚠️ **核心實作完成後，正式 memory harness 還要再驗一次「實際生產流程的磁碟峰值
   ⛔ 沒有超出 ~~sizing harness 的結果~~ `P_B_BUDGET`（v29）」**——⛔ 不能只驗記憶體。
   ⚠️ **v29（review 修正）：實際流程磁碟峰值的量法必須與 `P_B` 相同**，否則兩個數字⛔ 不能比較——⛔ 不得改用 `du`、
   ⛔ 不得只看輸出目錄：

   | 項目 | 規則（全部沿用 ④ 計畫書「二、`P_B` 的定義與量法」） |
   |---|---|
   | 實作 | ⚠️ **重用** `python/scripts/i074_stage2_sizing.py` 的量測原語與報告邏輯（allocated bytes、取樣、inventory、`P_path` 的計算），⛔ 不另寫一份。⚠️ **⑦d 細部計畫 v1（✅ 2026-10-02 確認）**：sizing 與 acceptance 共用 `scripts/lib/i074-stage2-measure.sh` 與 helper 的 profile（⛔ 不另寫一份） |
   | ⚠️ 隔離（第二輪 review） | 在 **repo 外的 `git clone --no-hardlinks` 複本**執行（沿用 ④），用**複本自己的** `python/baselines/i074_stage2/`——⛔ 不得碰真正的 baseline（failed record 一旦以正式 identity ＋ patch SHA 發布，就會永久擋住同 SHA 的 ⑩；⚠️ **⑦ 總綱 v1 第四輪 review（✅ 2026-09-30 確認）**：擋住的是**同語意 SHA**）。⚠️ **用正式的 Stage 2 identity 與 counterfactual patch SHA**（才驗得到 identity 與 failed-record 守門的完整組合，⛔ 不另造 harness 專用 SHA）；⚠️ failure 路徑放在 success 之後、或各用獨立的複本（⛔ 否則複本內的 failed record 會擋住 success 路徑）；結束後斷言**真正 repo 的完整 inventory（含 `python/baselines/i074_stage2/`）與 Stage 2 identity 檔⛔ 完全未變**（沿用 ④ 的自我檢查） |
   | 路徑與起訖 | **success**：before 的 replay 開始 → `--finalize` 發布完成；**failure**：before 的 replay 開始（回 6）→ `--publish-failed-record` 發布完成——兩條各自量、各自 ≤ `P_B_BUDGET`（witness 已在 ③c 完成，⛔ 不重量）。⚠️ **⑦d 細部計畫 v1（✅ 2026-10-02 確認）**：晉升（success、failure 各一）另以資訊值的窗口量，⛔ 不改 `P_B` 的起訖（「八之一」容量列） |
   | 位置 | L0（根檔案系統的 `statvfs` 已用量，catch-all）、L1（orchestrator 的 run 目錄）、L2（**複本的** `python/baselines/i074_stage2/`）、L3（worktree 暫存目錄）、L4（Docker Root Dir，只驗 `st_dev`）、L5（**複本的** `.git`）；路徑開始前各記一次 baseline 與 `st_dev` |
   | 單位與增量 | allocated bytes（`st_blocks × 512`，含目錄）；每個位置取 Σ max(current − baseline, 0) |
   | 峰值 | `P_path = max(dirs_peak, fs_peak, accounted)`；取樣間隔與 sizing 相同；容器足跡依同一模型（`SizeRw`、json-file log 上界、metadata 採用值） |
   | ⚠️ 差異 | ⑩ 不加 `--read-only`——`SizeRw` 不再保證是 0，**照實計入** accounted，並與 sizing 的 0 對照（即 ④「五」的 `--read-only` 落差）。⚠️ **⑦d 細部計畫 v1（✅ 2026-10-02 確認）**：acceptance ⛔ 不加 `--read-only`、`SizeRw` 照實計入並列 `read_only_gap` |

   **acceptance 門檻：< 450 MiB**，在正式執行**之前**通過。
   ⚠️ **⑦d 增補（✅ 2026-10-05 確認）**：判定量是 RSS——精確閘（每一個程序的 `max(RUSAGE_SELF, RUSAGE_CHILDREN)`）＋ 單向偵測器（取樣的 cgroup v1 `total_rss`）；契約限於**經正常 wait 鏈保存 resource usage 的程序**，被 kernel 自動回收的子孫在契約外（`SIGCHLD=SIG_IGN` 偵測到即 fail-closed）；含 page cache 的 cgroup 峰值只列資訊值。見「Stage 2 步驟 ⑦d 增補計畫：容器記憶體改以 RSS 判定」。
   ⚠️ 唯一一次正式 Stage 2 再記錄**實際全路徑峰值**，⛔ **但不為了量測而重跑**。
   ⚠️ 這是 capacity acceptance，⛔ **不適合當環境無關的一般 CI 單元測試**。
   ⚠️ 現行實測 **524 MiB（還沒算 replay）**，⛔ 不得用 `crossday.py` 的數字代替。
   ⚠️ **v29 註**：524 MiB 是 **replay 程序**（`evaluation.py` 的 Stage 2 路徑）改成串流之前的峰值（「二、③」的實測表）。
   ⑤ 的 sizing 量到的是 d～i 裡的**證據層程序**（`--envcheck`／`--finalize`／`--publish-failed-record`、三種 recovery、
   `--check-failed-record` 的 Python 段；最高 342.0 MiB）——⚠️ 那只是 sizing 階段的**觀察值**，⛔ 不是本項的驗收；
   ⛔ 也⛔ 不含 replay 程序。正式驗收是 **⑨-1 的 memory／disk acceptance**（⑦ 只做 memory harness 的實作與開發驗證）：**replay 程序（涵蓋 a～c）及 d～i 各獨立程序，皆各自 < 450 MiB**。

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
   | ⚠️ n1 | **v29：邊界**——可用空間 `== required` | ✅ 通過；`required − 1` → ⛔ 拒絕 |
   | ⚠️ n2 | **v29**：`f_bavail` 不足、但 `f_bfree` 足夠（root 保留區） | ⛔ **仍拒絕**（量法只用 `f_bavail × f_frsize`） |
   | ⚠️ n3 | **v29**：任一相關位置（run 目錄、worktree 暫存目錄、`python/baselines/i074_stage2/`、`.git`）或 Docker Root Dir 的 `st_dev` 不同 | ⛔ 拒絕（`P_B` 的單一檔案系統前提不成立） |
   | ⚠️ n4 | **v29**：Docker Root Dir 取不到、`statvfs`／`stat` 失敗 | ⛔ **fail-closed**（⛔ 不得當成「空間足夠」或「同一個裝置」） |
   | ⚠️ n5 | **v29**：n1～n4 任一拒絕 | ⚠️ **replay ⛔ 未被呼叫**（以 spy／fake 斷言），結束碼 1、⛔ 不計入正式 scan |
   | ⚠️ n6 | **v29**：常數 | `P_B_BUDGET` = 167,772,160、`M_safety` = 1,073,741,824、`required` = 1,241,513,984——三個值與算術都由測試釘死 |
   | ⚠️ n7 | **v29（第二～六輪 review）：進入複本與 preflight**——各一支：真正 repo 的 orchestrator 檔案 dirty（bootstrap）；freeze record 違反「八之二」的任一條；複本 `HEAD` ≠ `repo_head`；複本的已追蹤檔被修改；複本在 `python/baselines/i074_stage2/` 以外有未追蹤檔；複本有 alternates；orchestrator ⛔ 不在複本內執行、或自身內容 ≠ `repo_head` 中的版本；真正 repo 的 `python/baselines/i074_stage2/` 底下有未追蹤項目 | ⛔ 中止、結束碼 1；⚠️ **複本完整性不符的各支：信任錨的載入與驗證、failed-record checker、replay 一律⛔ 未被呼叫**（以 spy 斷言——⛔ 不能只斷言 replay）；⚠️ 另驗⛔ **沒有**接受裸 `repo_head` 的入口 |
   | ⚠️ n7b | **v29（第十～十七輪）：唯一一次的並行守門**（「八之一之二」的每一列至少一支；⚠️ 測試以隔離的鎖目錄執行，⛔ 不碰真正的 `/run/lock`——覆寫方式在 ⑦ 定，正式模式⛔ 不接受覆寫） | ⚠️ 以**另一個獨立的 fd**（非阻塞 `flock`）探測：orchestrator 執行期間**拿不到**鎖；**正常結束**時確認已無活著的後代、本趟 label 的容器已不在，才刪 sentinel、放鎖，之後立刻拿得到；⚠️ **不同 `XDG_DATA_HOME`**（另一支：不同 repo）同時啟動 → 仍搶同一把、後到的回 **1**；⚠️ **非固定帳號**執行 → **1**／**8**；⚠️ **鎖檔與 sentinel 的屬性**：預先建成 symlink、非 regular file、mode 不是 `0600`、link count ≠ 1、開啟之後路徑被換成別的 inode → 各自 fail-closed；鎖檔⛔ 從未被 unlink；⚠️ **無關的 helper**（supervisor **外部**啟動）⛔ 不延長鎖；⚠️ 正式後代 `setsid`、父程序又已結束 → 回到 subreaper，**它消失之前**⛔ 不放鎖；本趟的程序與容器：supervisor 收到 TERM → 移除容器 → TERM／KILL → **兩者都確認消失之前**獨立 fd 拿不到鎖；清不空 → ⛔ 不刪 sentinel、⛔ 不放鎖；⚠️ **supervisor 被 SIGKILL**：結束碼 **137**、sentinel 留下；之後的新 supervisor 回 **1**（`--promote` 模式回 **9**），建立複本、preflight、replay **⛔ 都未被呼叫**，並提示需要重開機；⚠️ **deterministic barrier**：讓舊 orchestrator 停在「建立 replay 容器之前」→ SIGKILL 舊 supervisor → 啟動新 supervisor（被 sentinel 擋下）→ 放開 barrier 讓舊流程建立容器 → 新流程⛔ 始終沒有進 replay，舊流程在發布／晉升前的 `/proc/locks` 驗證失敗而⛔ 不發布；⚠️ sentinel 內容**讀不懂**（空檔、部分寫入、schema 不符）→ 照樣拒絕；⚠️ **沒有任何解除入口**（⛔ 沒有能刪 sentinel 的 CLI）；sentinel 的 schema：每個欄位的型別與範圍各一支（`bool` 冒充整數、負的 uid、pid 為 0、大寫或格式錯的 boot ID、非 64 位小寫 hex 的 token、多欄／缺欄）；⚠️ **模擬重開機**（清空隔離的鎖目錄）之後：沒有殘留容器 → 放行；仍有帶 `i074.stage2.run` 的容器 → **1**／**8**；⚠️ **啟動檢查失敗⛔ 不留下 sentinel**（第十六輪）：有殘留容器 → 1／8、本次⛔ 沒有留下新的 sentinel；`docker ps` 失敗 → 同上；移除殘留容器、`docker ps` 恢復之後**直接重跑成功**、⛔ 不需要重開機；sentinel 建立之後、workload child 成功啟動之前注入失敗（例如 fsync 失敗；另一支：`fork` 成功而 `exec` 失敗——先 reap、確認沒有後代與本趟容器）→ supervisor 刪掉自己的 sentinel、放鎖，之後直接重跑成功；⚠️ **刪除前路徑被換成別的 inode**（收回與第 8 列的正常釋放各一支）→ ⛔ 不 unlink、⛔ 不放鎖，被換上的那個檔案⛔ 未被刪；⚠️ **直接執行複本內的 orchestrator**、或**傳入正確鎖檔但沒有持鎖的 fd** → `/proc/locks` 驗證失敗、在完整性檢查之前中止；⚠️ **label**：每個 `docker run`／`docker create` 恰好一個正確的 label；使用者自帶同一個鍵（重複、偽造 token）→ 拒絕；啟動前的 `docker ps` 失敗 → ⛔ fail-closed；⚠️ **supervisor 自身的完整性**：dirty 或與 HEAD 不同 → 中止；⚠️ **結束碼**：以實際的 argv 啟動，鎖衝突時完整 orchestrator 回 **1**、`--promote` 回 **8**，sentinel 存在時分別回 **1**／**9**；可攔截的訊號 → supervisor 清空後回 **128＋N** |
   | ⚠️ n8 | **v29（第五輪）**：finalize／publish／recovery／晉升之前的複本完整性不符（含 freeze record 副本被換掉） | ⛔ 中止、⛔ 不發布、⛔ 不晉升 |
   | ⚠️ n9 | **v29（第五～七輪）：真正 repo 不凍結**——⑩（以 fake replay）執行期間，在真正 repo：**(i)** 編輯並 commit 一般的 `python/`、`scripts/`、`docs/` 檔案；**(ii)** 在工作樹改掉 Stage 1 錨點與 `envcheck/`；**(iii)** **commit** 改掉 Stage 1 錨點與 `envcheck/` | 三支都：執行到的程式碼、finalizer 的 HEAD worktree、產出與晉升的證據，與「不動真正 repo」的對照組**逐位元相同**；⚠️ (ii)(iii) 的 B／C 判讀器**照樣接受**（錨點取自 archive 的 `base_commit`，⛔ 不讀工作樹或目前的 HEAD）；archive 全程⛔ 未被修改 |
   | ⚠️ n10 | **v29（第三～五輪 review）：sizing harness 的 freeze record** | `--formal` 且 `status = ok` → 寫出符合「八之二」封閉 schema 的 canonical freeze record，交叉不變條件全部成立（含 `counterfactual_patch_raw_sha256` ＝ `counterfactual_patch_sha256`、腳本 SHA 是 `repo_head` 中檔案內容的 SHA 而⛔ 不是 blob OID）；validation 模式、`assumption_violated`、任何失敗路徑 → ⛔ 不寫 |
   | ⚠️ n11 | **v29（第五～十輪）：`--promote`**（「八之三」的判定順序，每一步至少一支） | 步驟 1 複本完整性不符 → **9**，⚠️ 後面各步與任何 verifier ⛔ 未被呼叫；步驟 3 零個終態、`evidence/` 與本次 failed record 同時存在、不認得的未追蹤項目 → 各 **9**；步驟 4 來源 fsync 失敗 → **3**、⚠️ staging ⛔ 未被建立；⚠️ **rename ＋ parent fsync 都成功、但在結束之前被殺** → 重跑 `--promote` 走 6a、回 **0**／**6**；6a 目的地與來源不同 → **9**、⛔ 不覆寫；6b 空間不足 → **8**，釋出空間後重跑成功；複製的 I/O 錯誤 → **8**；⚠️ **終態發布之後、晉升之前竄改複本內的來源**（成功 archive 與 failed record 各一支）→ staging 驗不過 → **9**、目的地⛔ 不存在、staging 清掉；`rename_noreplace` 遇到 `EEXIST` → **8**，重跑走 6a；parent fsync 失敗 → **3**，重跑成功；finalize 的 rc=3 之後直接 `--promote` → 步驟 4 收尾後成功；⚠️ **兩個不同執行目錄**（兩個複本）的 `--promote` 同時對同一個真正 repo 執行 → 後到的拿不到鎖回 **8**、先到的⛔ 不受影響、它的 staging ⛔ 未被清掉（另一支：以 symlink 等不同路徑寫法指向同一個真正 repo，同樣互斥）；⚠️ **只竄改來源 archive 的 `finalizer_provenance.base_commit`** → 6b 的 **9**、目的地⛔ 不存在；⚠️ 6a 那一支：把**來源與既有目的地都改成同一份** canonical 但錯誤的 `base_commit`（逐位元比對因此通過、③ 的各道也通過）→ 只有「信任根的綁定」讓它回 **9**、⛔ 不覆寫（⛔ 只改目的地的話，會先因逐位元不同而失敗，證明不了新守門有執行）；可辨識的 orphan staging 被清掉、其他檔案⛔ 未被動；⚠️ **③ 的 recovery 模式、replay、finalize、publish 在 `--promote` 中⛔ 都未被呼叫**；⚠️ 真正 repo 的 `.git` 與複本內的終態 inventory ⛔ 不變（除 fsync）；合成守門中途被殺之後，下一次 `--promote` 的 `git worktree prune` 收掉殘留的登記並成功（⚠️ **⑦c 細部計畫 v1（✅ 2026-10-01 確認）**：最後一句照實改成「下一次 `--promote` 照樣成功」——prune 只清實體目錄已消失的登記，實體目錄還在的保留到 `<work>` 刪除、⛔ 不影響冪等；「複製的 I/O 錯誤 → 8」細分成 `ENOSPC`／`EDQUOT`／`EIO` → 8、來源側與其他 errno → 9；另見 ⑦c「五」的路徑錨定、結束碼的分類、ignore 守門、操作契約與收尾重算各列） |
   | ⚠️ n12 | **v29（第五輪）：failed record 的晉升與重跑** | 晉升之後還沒 commit → 下一次 ⑩ 的 preflight 中止；commit 之後，用**新的** freeze record → 新複本的 lookup 讀得到它、同**語意 SHA** 被擋（rc=2；⚠️ **⑦ 總綱 v1 第四輪 review（✅ 2026-09-30 確認）**：另一支：只改測試檔的 counterfactual 同樣被擋）；⚠️ 用**舊的** freeze record（OID 早於那筆 commit）→ preflight 中止（真正 repo HEAD 的 `failed/` ⊄ 複本）；**不同語意 SHA** 且 freeze record 夠新 → 可以進 replay（⚠️ **⑦ 總綱 v1 第五輪 review（✅ 2026-09-30 確認）**：原本寫「不同 SHA」；只改測試檔的語意 SHA 相同，⛔ 不得進 replay） |

   ⚠️ **模式旗標與兩份 patch 的測試（v19 新增）**：

   | # | 案例 | 要驗什麼 |
   |---|---|---|
   | o | ⛔ **未帶** `--i074-counterfactual` | ⚠️ **一般 Stage 2 逐項不變**：集合相等檢查、`candidate_mismatch.json`、rc=4 都還在 |
   | p | flag 帶在 **Stage 1** | ⛔ 中止（stage 限定，`assert_i074_flags()`） |
   | q | flag **重複出現** | ⛔ 中止（`_reject_duplicate_i074_flags()`） |
   | r | 使用者自行注入 script-injected 參數 | ⛔ 中止（CLI ownership） |
   | s | 兩份 patch 的宣稱 SHA 與**實際套用結果**不符 | ⛔ **fail-closed** |
   | t | **交換套用順序** | ⚠️ **runner 結構上⛔ 不提供這個入口**；測試改驗「**兩份增量 SHA 與 manifest 的 ordered components 一致**」——⛔ **不得斷言「合成 hash 必然不同」**（改不同檔案時會相同） |
   | u | `TOOLING_PATCH` 為空、只有 counterfactual | ⚠️ 兩個 SHA 都要有明確值（⛔ 不得省略欄位）。⚠️ **⑦ 總綱 v1（✅ 2026-09-30 確認）**：屬 runner 層；⑩ 的 orchestrator 要求 tooling 非空（③ 測試表的 ba 拆兩層） |
   | v | flag 開啟但 `COUNTERFACTUAL_PATCH` 為空 | ⛔ 中止 |
   | w | flag 關閉但 `COUNTERFACTUAL_PATCH` 非空 | ⛔ 中止 |
   | x | flag 開啟時的 `I074_MODE` | ⚠️ **必須**要求 `REPLAY_IMAGE_ID`／run identity，⛔ 不自動 pin |
   | y | **Python CLI 的成對守門**（五條，見「二、④」） | ⚠️ flag 無 SHA／SHA 無 flag／Stage 1 帶 SHA／SHA 非 64 位小寫 hex 一律⛔ 中止；⚠️ **官方 runner 與直接 CLI 兩條路徑都要測** |
   | z | **任一反事實生效條件不成立**時的 **failed-attempt record**（⚠️ v28 訂正：兩種原因 `candidate_flag_inconsistent`／`rr_not_restored` 各一支；⛔ 不只 `before_candidates != ∅`） | ⚠️ 有產出且**可辨識**、綁住兩份 patch SHA 與 run identity；⛔ **無正式 evidence archive**；⚠️ 同**語意 SHA** 重跑⛔ 被拒（⚠️ **⑦ 總綱 v1 第四輪 review（✅ 2026-09-30 確認）**：原本寫「同 SHA」） |

   ⚠️ **v26 新增（✅ 已隨 v28／③ v12 確認（2026-09-23））**：

   | # | 案例 | 要驗什麼 |
   |---|---|---|
   | aa | flag 一致、`before_candidates != ∅` | ⚠️ 結束碼是 **6**（⛔ 不是 1），`failure_reason` 是 **`rr_not_restored`**，且 operational 目錄有 bounded diagnostics 中繼檔 |
   | ab | ⚠️ **flag 恆為 `false`、但有列是 `CONTINUATION` 且 `setup_rr_qualified == false`** | ⛔ **必須中止（rc=6）**，`failure_reason` 是 **`candidate_flag_inconsistent`**——⚠️ 這正是 `before_candidates == ∅` 會假綠的形狀 |
   | ac | flag 與等價式不符（其他形狀，例如 flag 為 `true` 但該列不是 `CONTINUATION`） | ⛔ 中止（rc=6），`candidate_flag_inconsistent` |
   | ⚠️ ac2 | **v27：兩種同時發生**（有列 flag 不一致，另有列 flag 一致且是候選） | ⚠️ 一律是 **`candidate_flag_inconsistent`**（檢查順序決定優先序） |
   | ad | orchestrator 的分流 | 0 → finalize；6 → 發布 failed record；其他 → 停；⚠️ **使用者以其他路徑覆寫階段一的輸出 → 在 replay 之前拒絕** |
   | ae | finalize 回 1 之後重跑 | ⚠️ **沿用同一份凍結 patch 與 operational 輸出**，⛔ replay ⛔ 不被呼叫 |
   | af | Stage 2 identity 的路徑 | ⚠️ 依 stage 固定推導；⛔ **Stage 1 的 identity 路徑與 pin 的 fail-closed 行為逐項不變** |
   | ag | 專用 tag | ⚠️ pin 用的 tag ⛔ 不等於任何 build 腳本的預設 tag；`python/scripts/test.sh` 跑完之後，釘住的 image **仍帶著專用 tag** |
   | ah | 環境等價判定 | 全相同 → EQUIVALENT（rc=0）；「二、⑤」判定表的第 1～3 條**各一支**不成立 → NOT_EQUIVALENT（rc=7）；第 4 條的欄位不同 ⛔ **不得**翻轉結果 |
   | ai | 環境等價比對的讀取方式 | ⚠️ 串流——⛔ 不得把兩份 rows 同時整份載入（以計數或 spy 斷言） |

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
   `artifacts.py` 的 `compare_rows()`的全欄位比對：它的 docstring 明寫「⛔ 不挑欄位比」，
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

   ⚠️ **執行機制（v26 補落點）**：兩個 worktree（原始 `e1cbbbd`、patched `e1cbbbd`）**各自以獨立程序**
   跑同一份 fixture、各自輸出 decision summary，再由**第三個程序**依上面的 allowlist 比對
   ——⛔ 不在同一個程序內 import 兩份同名模組。檔案位置見「三、受影響檔案」。

4. **兩種 patch 的可重建性**（⚠️ **分開驗**，見「三之一」與「三之一之二」）：
   `counterfactual_patch` ——斷言「套用 stored patch 後的增量 `git diff --binary` SHA」等於
   ⚠️ **Stage 2 evidence manifest** 記錄的值（⛔ **不是** provenance——⛔ 不得加第 11 欄）；
   `tooling_patch` ——若存在則同樣斷言，且⛔ **不得含任何判定變更**。
   ⚠️ **合成 hash** 才與 replay artifact 既有的 `tooling_patch_sha256` 對照。
   ⚠️ **⑦ 總綱 v1（✅ 2026-09-30 確認）**：三個 SHA 一律改用「⑦ 總綱 v1」「二」的 canonical diff（`--full-index` ＋ 全部釘死的參數、在隔離的暫存 bare repo 計算；⚠️ 第一輪 review：只加 `--full-index` 不夠）、tree-to-tree 計算（見 ③「四之一」與「⑦ 總綱 v1」的「二」）。
5. **逐列驗證強度**：現有 `validate_diagnostics`／`validate_after_artifact` 的測試**全部續跑**。
   ⚠️ **另加一道 artifact 欄位存在性斷言（v20）**：B／C 判讀用到的五個欄位
   （`lifecycle_phase`／`market_bias`／`action_state`／`position_action_condition.state`／`final_entry_state`）
   **必須都在已封存的 after artifact 裡**——⛔ 判讀規則⛔ 不得引用 artifact 沒有的欄位。
6. **謂詞**：⛔ **兩側都不重算**——after 用已封存的 156 列，before 用同一個謂詞
   （套 patch 後恆 `False`），⛔ 不引入第三種定義。
7. **counterfactual patch**：套用前後 **`e1cbbbd`** 既有測試全綠
   （⚠️ **必要但⛔ 不充分**——⛔ 充分性由第 3 項的 differential guard 負責）；
   九個診斷欄位的 schema 由 `validate_diagnostics(side="before")` 驗，
   ⚠️ **v26：再加「①之三」的兩條不變條件**（`validate_diagnostics(side="before")` 刻意不驗等價式）。
8. **端到端**：`smoke-replay-offline.sh` 的 Stage 2 路徑照跑。
9. ⚠️ **B／C 判讀器（v26 新增，✅ 已隨 v28／③ v12 確認（2026-09-23）；⛔ 必須在 ⑩ 之前完成並測過）**：
   判讀器只讀**已封存的** comparison artifact，逐列套用「關閉條件」的判讀矩陣，輸出每一列的
   B／C 與理由。⚠️ 測試用**合成的 comparison rows**（⛔ 不得用正式結果）：
   ⚠️ **v29（第五輪）**：判讀器讀真正 repo 中**已晉升**的成功 archive，判讀前先以 ③ 的完整 verifier 驗過，驗不過就拒絕（「八之三」；測試 m）。

   | # | 案例 | 期望 |
   |---|---|---|
   | a～d | 四格（before `TESTING`／`CONFIRMED` × 非 AVOID／AVOID）各一列，逐欄符合矩陣 | ✅ **B** |
   | e | 任一格的 `lifecycle_phase` 沒翻轉 | ⛔ **C** |
   | f | 非 AVOID 的 `market_bias` 沒有 `BULLISH_BIAS` → `BULLISH_CONTINUATION` | ⛔ **C** |
   | g | AVOID 的 `market_bias` 有變 | ⛔ **C** |
   | h | `final_entry_state` 有變 | ⛔ **C** |
   | i | `action_state` 與 `position_action_condition.state` 不一致 | ⛔ **C** |
   | j | ⚠️ **判讀欄位以外出現非預期差異** | ⛔ **C**——規則見「關閉條件」的 v26 補充 |
   | k | top-level `position_action` 有變 | ⛔ **C**（⚠️ v28 訂正：撤回 v26 的「只記錄、不影響判定」）；⚠️ 另驗它維持相同時不影響 B（a～d 的合成列本來就要求它相同） |
   | l | 156 列之中任一列是 C | ⚠️ **整體判為 C**（⛔ 不以多數決） |
   | ⚠️ m | **v29（第三～七輪 review）**：判讀前以 ③ 的完整 verifier 驗已晉升的 archive，錨點取自 manifest 的 `finalizer_provenance.base_commit`——任一成員 bytes 被改、manifest 驗不過、`base_commit` 不可達或該 commit 裡缺錨點 | ⛔ **拒絕判讀**（⛔ 不輸出 B／C）；⚠️ 對照組：工作樹或目前 HEAD 的錨點被改過 → **照樣接受**（n9） |

##### 七、完成後的歸檔位置

| 內容 | 歸檔到 |
|---|---|
| 串流比對的理由與峰值數字 | `development-workflow.md`（比照「凍結 bundle 不得重產」那兩節） |
| **Stage 2 counterfactual 的 v3 比較模型** ⚠️ **v18 取代原「mismatch streaming contract」** | `sr-zone-scoring.md`——⚠️ 它現在保存的是「兩側 candidate 集合必須相同」與 **in-memory validator 契約**，⛔ **兩者都要改**：<br>① **after 的 156 keys 是唯一 cohort、before 全量 replay**；② **全量 key 守門保留、candidate 集合相等要求移除**；③ **before candidate 預期恆空**，非空＝patch 失效；④ **一趟串流 loader，驗證強度⛔ 不得下降**；⑤ **archive 是唯一 commit point，terminal outcome 由 manifest 記錄（v3 恆為 0）**；⑥ ⚠️ **`candidate_mismatch.json`／rc=4 ⛔ 不適用 counterfactual 路徑**（一般路徑的契約⛔ 不動） |
| 九個診斷欄位的 before／after 對稱契約 | `sr-zone-scoring.md`「九個診斷欄位的完整 schema」 |
| ⚠️ **artifact 判讀矩陣**（140／16 兩類）與「哪些欄位不可觀測」 | `sr-zone-scoring.md`——⚠️ 它現在寫的是 `market_state`／`entry_permission_state`，⛔ **那兩個⛔ 不在 replay row 裡**，必須改成實有欄位並註明推導含意 |
| B／C 判定結果與證據 SHA | `issue.md` I-074，⛔ 在決策樹走完之前不得移除本筆 |
| ⚠️ **環境見證**（v26）：image 遺失的經過、環境等價判定規則、專用 tag 與 tarball 的保存與還原程序、stage-scoped identity | `development-workflow.md`（⚠️ 與「I-074 Stage 1 的正式執行程序」並列，補一節 Stage 2 的正式執行程序）。⚠️ **v28**：該節要**寫死 tarball 的保存位置與驗 SHA 的步驟**（review 附帶條件） |
| ⚠️ **B／C 判讀器的規則**（v26） | `sr-zone-scoring.md`（與判讀矩陣寫在一起） |

##### 八、執行順序

⚠️ **v26 重排（✅ 已隨 v28／③ v12 確認（2026-09-23））**：插入 **③a～③d**，把**環境閘門提前到 ③ 的大部分實作之前**。
理由：閘門若判 NOT_EQUIVALENT，Stage 2 整套做法都要重想，③ 的其餘實作會白做；
而閘門本身只需要一小包前置（identity、專用 tag、串流讀取、比對模組與它的小 archive）。
⚠️ **①～⑪ 的編號與含義不變**，其他節引用的「步驟 ⑦」「⑩」等仍然有效。

```text id="i074_stage2_order_001"
① 裁決採方案 B（before source artifact）              ← ✅ **已確認 2026-09-22**
② **counterfactual patch ／ v3 比較模型設計**（⛔ after 側不重算）  ← ✅ 已執行，✅ **2026-09-23 review 通過**
   ⚠️ **必須排在 evidence contract 之前**——before source artifact 的
      schema、守門條件與 validator **全都依賴這個設計**
   ⚠️ **同一步要一併裁定（v19）**：
      ⒜ `--i074-counterfactual` 的 opt-in 語意與 stage 限定（「二、④」）
      ⒝ 兩份 patch 的輸入、**固定套用順序**、增量 diff SHA 與合成 hash（「三之一之二」）
      ⒞ 兩份 SHA 記在 **Stage 2 evidence manifest**（⛔ 不動 10 欄的 `PROVENANCE_FIELDS`）
③ **Stage 2 evidence contract**（before source artifact ＋ **counterfactual patch**
   ＋ tooling patch；⚠️ **兩種 patch 分開記錄、獨立 SHA**）
   ⚠️ **必備輸出還有 failed-attempt record**（「二、①」）——⛔ 它⛔ 不是成功 archive
   ⚠️ 要裁定**原子邊界**：單一 evidence archive 一次性發布，
      recovery 從 manifest 讀回原 terminal outcome（⚠️ v3 之下恆為 rc=0）
   ③a ③ evidence contract 現行版確認（⚠️ 連同本計畫書現行版；② 的 review 已於 2026-09-23 通過）  ← ✅ 2026-09-23
   ③b 實作第一包 → review：   ← ✅ 2026-09-23 實作完成、✅ review 通過
        stage-scoped identity ＋ 專用 tag ＋ tarball 的 pin 流程、共用的串流讀取、
        ⚠️ `artifacts.py` 的 row-level 共用原語（v27）、
        可共用的 archive 核心、環境等價比對模組 ＋ `envcheck/` 小 archive
   ③c **環境閘門**：pin 新 image ＋ tarball → after' 見證趟（約 180 分鐘）
        → 環境等價比對 → 發布 `envcheck/`（0 或 7）  ← ⚠️ 凍結窗口 A（見「八之一」）
        ✅ 2026-09-23：**EQUIVALENT（rc=0）**，凍結窗口 A 已結束（見「③c 執行結果」）
        └ NOT_EQUIVALENT → ⛔ 停止，另立 issue；⛔ 不進 ③d
   ③d 實作 ③ 的其餘部分（Stage 2 archive、failed record、check／recover）→ review   ← ✅ 2026-09-24 實作完成、✅ review 通過
④ 實作獨立 sizing harness（⛔ 不含正式 preflight）   ← ✅ 2026-09-24 實作完成、✅ 2026-09-29 review 通過（四輪）並 commit（計畫書 v7 ＋ 差異 1、2 ✅ 已確認；review 修正見「Stage 2 步驟 ④ 實作結果」）
⑤ 實測並記錄 **P_B** 的磁碟／記憶體峰值，裁定 **M_safety = <固定 bytes>**   ← ✅ 2026-09-29 正式量測完成（`P_B` 150.8 MiB、`M_safety` 1 GiB），✅ review 通過並 commit（見「Stage 2 步驟 ⑤」）
⑥ 更新計畫並**再次確認**   ← ✅ 2026-09-29：Stage 2 計畫書 v29 review 通過並 commit
⑦ 實作（⚠️ v29 依現況重列——發布、三種 recovery、failed-record 的 check、共用串流讀取、信任錨、
   「①之三」的 `CounterfactualEffectCheck` 已在 ③b／③d 完成，⛔ 不重做）：
   `evaluation.py` 的 `--i074-counterfactual` opt-in ＋ SHA 注入 ＋ Python 成對守門、一趟串流 loader、
   `before_candidates == ∅` ＋「①之三」守門（⚠️ **重用** `CounterfactualEffectCheck`）、結束碼 6 ＋ 有界診斷中繼檔、
   operational 輸出；runner 的兩份 patch 與增量 SHA、Stage 2 的 argv fixture；
   orchestrator `run-i074-stage2.sh`（⚠️ 第十三輪同步：入口自我驗證 → **supervisor**（驗自身 → 驗執行帳號 → 取鎖 → 確認⛔ 沒有舊 sentinel → 查殘留容器（`docker ps` 成功且為空）→ 建 sentinel → 設 subreaper → 啟動持鎖階段（workload child）；單一的容器 label 注入點；⚠️ 第十七輪同步「八之一之二」第 7、3 列的順序）→ 持鎖階段建立隔離複本 → `exec` 複本內的 orchestrator；複本內：凍結 patch → preflight：
   freeze record ＋ 複本完整性 ＋ 信任錨 ＋ 環境見證錨 ＋ failed-record lookup ＋ **磁碟檢查** → replay → 分流 → finalize／publish
   → **晉升**（單一、冪等的 `--promote` 與端到端結束碼；複本內終態的 rc=3 也由它收尾，⑩ 的流程⛔ 不呼叫 ③ 的 recovery 模式），見「八之一」～「八之三」）；
   B／C 判讀器（⚠️ 判讀前以 ③ 的完整 verifier 驗已晉升的 archive）；memory harness 的**實作與開發驗證**（repo 外的隔離複本；
   ⚠️ **正式驗收⛔ 不在 ⑦**——要用 ⑨ 封存的 exact patch SHA，見 ⑨-1）；sizing harness 在 `--formal` 成功時輸出 **freeze record**；
   ③ 留給 ⑦ 的測試（ax、ba，以及 n／ai／ay 的「在 replay 之前」那一層）
   ⚠️ **⑦ 總綱 v1（2026-09-29，✅ 2026-09-30 確認）**：⑦ 分四包依序實作——⑦a replay 側 → ⑦b supervisor＋orchestrator＋freeze record
   → ⑦c `--promote`＋B／C 判讀器 → ⑦d memory harness，每包「細部計畫 → 實作 → review → commit」；
   ⚠️ `evaluation.py` 與 `replay_bundle/` 的改動**經 tooling patch 進入 replay**（⑩ 的 replay 執行的是 `e1cbbbd` worktree）；
   各包範圍、跨包介面與測試落點見「Stage 2 步驟 ⑦ 總綱 v1」   ← ✅ ⑦ 總綱 v1 已確認（2026-09-30）；✅ ⑦a 細部計畫 v1 已確認（2026-09-30）；✅ ⑦a 實作 review 通過並 commit（2026-09-30）；✅ ⑦b 細部計畫 v1 已確認（2026-09-30）；✅ ⑦b 實作 review 通過並 commit（2026-10-01）；✅ ⑦c 細部計畫 v1 已確認（2026-10-01）；✅ ⑦c 實作 review 通過並 commit（2026-10-02）；✅ ⑦d 細部計畫 v1 已確認（2026-10-02，commit `9af7942`）；⑦d 實作已 commit（`f20ce3c`，含第一輪 review 的三項修正；見「Stage 2 步驟 ⑦d 實作結果」）；✅ ⑦d 增補計畫 v11 已確認（2026-10-05，commit `d1267bb`）；✅ ⑦d 增補實作 review 通過並 commit（2026-10-06，`576a8f7`；見「Stage 2 步驟 ⑦d 增補的實作結果」）；✅ 量測的有效性條件與 ⑨-1 fail-fast 計畫 v8 已確認（2026-10-06，`0a92d95`）；✅ 它的實作 review 通過並 commit（2026-10-07，`c49fde1`；見「Stage 2 量測的有效性條件與 ⑨-1 fail-fast 的實作結果」）
⑧ 測試矩陣 a～z ＋ aa～ai（⚠️ 含 **o：未帶 flag 時一般路徑逐項不變**、**v／w：truth table**、**y：Python 成對守門**、**z：failed-attempt record**、**ab：flag 假綠**）＋ B／C 判讀器的 a～m
   ⚠️ **⑦ 總綱 v1（✅ 2026-09-30 確認）**：⑦ 各包已各自附上它負責的測試；⑧ 改成**矩陣完整性稽核 ＋ 全量執行**——逐 id 對照
   a～z、aa～ai、n1～n12、n7b、B／C 的 a～m 與 ③ 的測試表，補齊缺漏後全量執行一次
   ← ⚠️ 現在在這裡：⑧ 計畫 v4 待確認（見「Stage 2 步驟 ⑧ 計畫」）
⑨ **differential guard**（「六、3」）→ 產生並封存 exact counterfactual patch，驗三方 SHA
   （＋ **`e1cbbbd`** 既有測試套用 patch 前後全綠）
   ⚠️ **⑦ 總綱 v1（✅ 2026-09-30 確認）**：⑨ 改成**兩份 patch 一起封存**（counterfactual ＋ tooling）；「既有測試全綠」改成
   「套 counterfactual ＋ tooling 之後全綠」；differential guard 的 patched 側含 tooling；另加 **tooling 非語意 guard**
   （同一份 fixture、一般 Stage 2（不帶 flag），`e1cbbbd` 與 `e1cbbbd`＋tooling 的 comparison／report 除 provenance 之外逐位元相同）
   ⚠️ **v29（第三輪 review 改寫）：⑨ 之後依序**——
   ⑨-1 **正式 memory／disk acceptance**：⑦ 實作的 memory harness，用⑨ **封存的 exact counterfactual patch SHA**、在 repo 外的
        隔離複本執行：replay 程序及 d～i 各程序各自 < 450 MiB，success／failure 兩條實際流程磁碟峰值各自 ≤ `P_B_BUDGET`。
        ⚠️ exact patch 的 bytes 或 SHA **之後只要再變，⑨-1 就要重跑**（容量與 failed-record 組合驗到的必須是正式 SHA）；
        結果記進本筆並 commit（⑨-2 之後就不能 commit 了）
        ⚠️ **⑦ 總綱 v1（✅ 2026-09-30 確認）**：「exact patch」指**兩份封存 patch**（counterfactual ＋ tooling），任一份的 bytes 或 SHA 再變都要重跑
        ⚠️ **⑦d 細部計畫 v1（✅ 2026-10-02 確認）**：入口 `scripts/i074-stage2-acceptance.sh --formal --replay-compute full`（結束碼 0 ok／2 threshold_exceeded／1 harness 失敗）；
        `full` 是唯一的量測趟（「正式 scan 的計次裁決」的新格）；報告的兩份 patch SHA 由 ⑨-1 的程序與 ⑨ 的封存值比對
   ⑨-2 **⑤ 的確認重跑**：⑨-1 之後的最後一次 commit 之後，以 ⑩ 要用的 HEAD 跑一次 `--formal` sizing（約 6 分鐘），
        產出 **freeze record**；`status = "ok"` 且 `P_B ≤ P_B_BUDGET` 才進 ⑩；⛔ 不符就走「六、1」的回退順序。
        ⚠️ **⑦ 總綱 v1（✅ 2026-09-30 確認）**：這一次 sizing 用**真實的兩份 patch**（⑤ 量的 `P_B` 是空 tooling，差距由這一次實測驗證）
        ⚠️ ⑩ 的隔離複本就釘在這一次的 OID（freeze record 的 `repo_head`）；之後真正 repo 可以照常 commit（見「八之一」）
⑩ **唯一一次**正式 Stage 2：before 全量 replay（約 180 分鐘）＋ finalize ＋ **晉升**（⚠️ v29：在隔離複本執行，見「八之一」）   ← ⚠️ 凍結窗口 B（v29：約束對象是複本）
   ⚠️ **⑦d 細部計畫 v1（✅ 2026-10-02 確認）**：⑩ 期間由外部唯讀的 observer 額外記錄實際峰值（記憶體是 cgroup high-water mark 的下界、磁碟是取樣值；⛔ 不作為前置）
⑪ B／C 判讀器判定 ＋ 歸檔
```

⚠️ **①～⑨ 都是分鐘～小時級且可獨立驗證**（⚠️ v26 例外：③c 的見證趟約 180 分鐘），
⛔ 不要跳過直接跑 **⑩**——Stage 1 的教訓是「跑了 183 分鐘才發現容量不夠」。
⚠️ 另外 **③ 未完成前⛔ 不得進入 ⑦**——發布順序與 schema 還沒定，實作會白做；
而 **② 未完成前⛔ 不得進入 ③**——比較模型決定 schema 與 validator。
⚠️ **⑩ 是唯一一次**：峰值驗收已裁決走 **memory harness**——⚠️ **v29（第四輪）**：⑦ 只做它的實作與開發驗證，
**正式 capacity acceptance 是 ⑨-1**（用封存的 exact patch SHA）；⑩ 只**額外記錄**正式資料執行的實際峰值，
⛔ **不作為 ⑩ 的前置**、⛔ 不用該次結果反向決定是否執行 ⑩，也⛔ **不為量測而重跑**。

##### 八之一、凍結窗口（v26 新增，✅ 已隨 v28／③ v12 確認（2026-09-23））

⚠️ Stage 1 的凍結窗口是為了「D／D+1 跨日時 `base_commit` 不得漂移」。Stage 2 的兩次 replay
都跑在 `e1cbbbd` worktree 上，**產品程式碼不受 HEAD 影響**；⚠️ 但 runner、比對模組與 finalizer
都從 HEAD 執行，⛔ 執行途中被換掉就證明不了「這份證據是哪一版工具產生的」。

⚠️ **⑦ 總綱 v1 訂正（2026-09-29，✅ 2026-09-30 確認）**：上一句的「runner、比對模組（環境等價比對）與 finalizer 從 HEAD 執行」成立，⛔ 但**漏了 replay 本身**——replay 容器掛的是 `e1cbbbd` worktree 的 `python/`（`run-replay-offline.sh` 的 `SOURCE_REF`與 `-v "$WORKTREE/python":/app:ro`），所以 **`evaluation.py` 與 `replay_bundle/`（含反事實比較、串流 loader、`CounterfactualEffectCheck`）也是從 worktree 執行**，⛔ 不是從 HEAD。⑦ 在 HEAD 對它們的改動因此**經 tooling patch 進入 replay**（套用順序 counterfactual → tooling），見「Stage 2 步驟 ⑦ 總綱 v1」的「二」。

| 窗口 | 起點 | 終點 | ⛔ 期間不得 commit |
|---|---|---|---|
| **A**（環境閘門） | ③c 的 after' 見證趟開跑 | ⚠️ **v27**：`envcheck/` 已 durable，且結束碼是 **0 或 7**；⚠️ 若先回 rc=3，**延續到 `--recover-envcheck` 還原成 0 或 7** | 任何動到 `python/`、`scripts/`、`.gitattributes` 的變更 |
| **B**（正式 Stage 2） | ⑩ 的 before 開跑 | ⚠️ **v29 第五輪：約束對象改成隔離複本，終點是晉升完成**（rc=3 之後重跑 `--promote`，直到回 0／6——第十輪訂正，⑩ ⛔ 不呼叫 ③ 的 recovery 模式；見下方 v29 的表，取代下一句）。⚠️ **v27**：成功 archive **或** failed-attempt record 其中之一已 durable；⚠️ 任何 rc=3 都**延續到對應的 recovery 完成**（`--recover-durability` 或 `--recover-failed-record`） | ⚠️ v29：⛔ 不再限制真正 repo；⛔ 不得修改**複本** |

⛔ **v26 的終點只寫了 rc=0**：NOT_EQUIVALENT 的合法終態（durable ＋ rc=7）與 rc=3 之後的 recovery 都落在窗口之外，
⚠️ 而 recovery 同樣從 HEAD 執行、同樣要驗證「這份證據是哪一版工具產生的」——v27 補上。

⚠️ **兩個窗口之間可以正常 commit**（③d～⑨ 的實作就在這段）——⚠️ 所以 after' 與 before 的
`runner_sha256` **本來就可能不同**：③ evidence contract 的全圖只要求「before source 與 comparison 來自同一次執行、
provenance 逐欄相等」，⛔ 不要求與 after' 相同，⛔ 那不是缺陷。
⛔ **但 image 在兩個窗口必須相同**——由 Stage 2 identity 保證，並由 ③ evidence contract 的環境見證錨驗證。

⚠️ **v29 第五輪改寫（2026-09-29，✅ 使用者裁決：⑩ 在隔離複本執行）**——第二～四輪疊上去的「凍結檢查」機制（真正 repo 的
HEAD 凍結區間、工作樹範圍 (a)～(c)、四個檢查點、窗口結束紀錄、`freeze_violation` 標記、第五個根目錄）**整套撤回**：⑩ 的工具
直接從真正 repo 的工作樹執行時，只能**事後偵測**有沒有人改過，每補一層就多一個狀態機缺口（第五輪 review 又指出 recovery 會
改寫歷史結果、紀錄綁定未 durable 的終態、標記沒有契約、freeze record 原件沒有保存）。改成從**結構上排除**：

| 項目 | 規則 |
|---|---|
| 執行位置 | ⑩ 在 **repo 外的隔離複本**執行：`git clone --no-hardlinks <真正 repo>` 建在 repo 外的執行目錄，detached checkout 到 **freeze record 的 `repo_head`**（「八之二」）。replay、finalize、recovery 全部用**複本內**的腳本——正式腳本的路徑常數由腳本位置推導（`finalize-stage2-evidence.sh`、`run-replay-offline.sh` 都是），證據只會寫進**複本的** `python/baselines/i074_stage2/`。與 ④ 的 sizing、⑨-1 的 acceptance **同一套做法——`P_B` 量的正是這個條件**。⚠️ **⑦ 總綱 v1（✅ 2026-09-30 確認）**：兩份 patch 一律取自**複本內的常數路徑**（`python/baselines/i074_stage2/counterfactual_e1cbbbd.patch`、`tooling_e1cbbbd.patch`），由 orchestrator 凍結到 `<work>/run/patches/`，⛔ 使用者不得指定；⑩ 的 tooling ⛔ 不得為空 |
| 進入複本（第十三輪同步資料流） | **入口 → supervisor → 持鎖階段 → 複本**：操作者從真正 repo 執行 orchestrator 的入口 → 入口驗自己的檔案內容 ＝ 真正 repo 的 HEAD 中的版本（⛔ dirty 即中止）→ 交給 **supervisor**（「八之一之二」；⚠️ supervisor 自己也先驗它的檔案內容 ＝ HEAD 中的版本，⛔ 不執行未 commit 或與預期版本不同的 supervisor）→ supervisor 驗執行帳號 → 取鎖 → 確認⛔ 沒有舊 sentinel → 查殘留容器（`docker ps` 成功且為空）→ 建 sentinel → 設 subreaper → 啟動持鎖階段（workload child）（⚠️ 第十七輪同步「八之一之二」第 7、3 列的順序）→ 持鎖階段**再驗一次**自身內容 → 驗 freeze record 的基本欄位 → 建立複本 → 把 freeze record 與同目錄的報告**複製進執行目錄**（之後一律用副本，比照凍結 patch）→ **`exec` 複本內的 orchestrator**（pid 不變、仍是 supervisor 的後代）。⛔ 入口與持鎖階段都不做任何 preflight、replay 或發布。⚠️ 各支程式的檔名與 argv 在 ⑦ 的計畫書定（決策表第 7 列） |
| ⚠️ 鎖、sentinel 與本趟的範圍 | 見獨立小節「**八之一之二**」（第十四輪從表格移出：原本塞在單一儲存格、被實體換行切斷） |
| 複本完整性（fail-closed） | 複本內的 orchestrator 在 preflight，以及每次呼叫 finalize／publish／recovery／晉升之前驗：自己位於該複本（canonical path）；⛔ 沒有 alternates（`.git/objects/info/alternates` 不存在）；複本 `HEAD` ＝ freeze record 的 `repo_head`；已追蹤檔⛔ 沒有修改（`git -C <複本> status --porcelain --untracked-files=no` 為空）；⚠️ `python/baselines/i074_stage2/` **以外**⛔ 沒有未追蹤檔（第六輪補）；⚠️ orchestrator **自身的內容** ＝ `git show <repo_head>:scripts/run-i074-stage2.sh`（第六輪補）；freeze record 副本的 SHA-256 與 preflight 記下的相同。⚠️ **順序（第六輪訂正）**：上述檢查**⛔ 只用 git 與 shell、⛔ 不 import 任何 repo 內的 Python 模組**（`repo_head` 以 host 的標準庫解析），而且排在 preflight 的**最前面**——⛔ 在它通過之前，⛔ 不得執行信任錨、failed-record checker 或任何 repo 內的 validator（它們本身可能就是被改過的程式）。⚠️ 複本內新產生的 `evidence/`、`failed/` 是未追蹤的產物、⛔ 不受這項檢查保護——由晉升前的完整驗證負責（「八之三」）。⚠️ 複本歸 orchestrator 專用，這些檢查防的是誤操作。**preflight 不符** → 結束碼 1、⛔ 不進 replay、⛔ 不計入正式 scan；**replay 開始之後不符** → ⛔ 不發布、⛔ 不晉升，停下另立 issue（能否重跑⛔ 不預先放寬——現行計次只允許 patch 失效後重跑） |
| 真正 repo | ⚠️ ⑩ 期間**不再凍結**：可以照常 commit 與編輯——⑩ 用的程式碼與 HEAD worktree 都固定在複本的 OID；判讀器的錨點也取自 archive 記錄的 `base_commit`、⛔ 不讀工作樹（「八之三」），所以連 Stage 1 錨點與 `envcheck/` 的改動都影響不了已發布的證據（⚠️ 但它們是終態證據，改動本身另立 issue；⛔ 不得改寫會讓 `base_commit` 不可達的歷史）。⚠️ **唯一的例外：已晉升但還沒 commit 的 failed record**——下一次 ⑩ 的複本只 clone 得到已 commit 的內容，lookup 會看不到它。所以複本內的 preflight 另驗兩條：①「**真正 repo 的 `python/baselines/i074_stage2/` 底下⛔ 沒有未追蹤項目**」，有就中止（要求先 commit）；② **真正 repo 的 HEAD 裡 `failed/` 底下的每一筆紀錄，都必須出現在複本的 `repo_head` 裡**（`git ls-tree -r` 比對，逐位元相同），否則中止——⛔ 否則拿**舊的** freeze record（它的 OID 早於那筆 failed record 的 commit）就能讓新複本的 lookup 看不到它，同 SHA 繞過守門（⚠️ **⑦ 總綱 v1 第四輪 review（✅ 2026-09-30 確認）**：指同語意 SHA）。⚠️ **⑦c 細部計畫 v1（✅ 2026-10-01 確認）**（第三、四輪 review）：⑩ 的任何一趟執行期間另有**操作契約**——晉升程序以外的程序⛔ 不得搬移、刪除或替換 `python/`、`python/baselines/`、`i074_stage2/`、`failed/` 本身，⛔ 不得改動 active staging 與本次目的地的任何成員，⛔ 不得移除或改寫 `.gitignore` 的 staging 規則、⛔ 不得 `git add -f` staging；其他編輯、`git add -A` 與 commit 照常（⑦c「二之三」） |
| 窗口 B（改） | 約束對象從真正 repo 改成**複本**：從建立複本到晉升完成（rc=3 之後重跑 `--promote`，直到回 0／6——第十輪訂正），⛔ 不得修改複本——由上面的完整性檢查把關 |
| 證據進入真正 repo | 只能經「**晉升**」（「八之三」）；B／C 判讀器讀**已晉升**的成功 archive，並在判讀前以 ③ 的完整 verifier 驗過 |
| 容量 | 複本在 preflight **之前**建立，磁碟檢查量的是建立之後的可用空間；`P_B` 的起訖⛔ 不變（replay 開始 → 複本內發布完成）。晉升另有自己的空間預檢（「八之三」）——晉升失敗時證據仍在複本、可以重試，⛔ 不會失去三小時的 replay，所以⛔ 不算進 `P_B`。⚠️ **⑦d 細部計畫 v1（✅ 2026-10-02 確認）**：acceptance 實跑晉升，記憶體納入 < 450 MiB、磁碟只列資訊值（使用者裁決 2026-10-02；本列的規則⛔ 不變） |

##### 八之一之二、鎖、active-run sentinel 與本趟的範圍（v29 第十～十七輪；✅ 已隨 v29 確認（2026-09-29））

⚠️ 第十四輪從「八之一」的表格移出來成為獨立小節。⚠️ 這裡只定**不變條件與已裁定的機制**；各支程式的檔名、argv 與測試落點在
⑦ 的計畫書定（決策表第 7 列）。⚠️ 鎖與 sentinel 防的是**並行**；跨時間的次數仍由計次政策與 failed-record lookup 管。
⚠️ **第十五輪（✅ 使用者裁決，決策表第 9 列）**：範圍是**固定的執行帳號**、supervisor 被 SIGKILL 之後**只能靠重開機解除**——這台
host 上 `dev` 不能建立 cgroup（所在的 cgroup 屬 `system.slice/ssh.service`、不可寫）、沒有 systemd user manager（沒有 linger）、
`sudo` 需要密碼（2026-09-29 實查），⛔ 沒有 root 設定就做不到結構性的程序追蹤與跨帳號的穩定鎖物件。依賴 Linux（`/proc/locks`、
parent-death signal、child subreaper、`/run/lock`）。

| # | 項目 | 規則 |
|---|---|---|
| 1 | 執行帳號與位置（第十五輪） | ⚠️ **只允許固定的執行帳號**（本 host：`dev`，uid 1001；⑦ 以常數寫死並由測試釘住）——其他帳號執行 supervisor → 完整 orchestrator 模式回 **1**、`--promote` 模式回 **8**。鎖檔 **`/run/lock/i074-stage2.lock`**、sentinel **`/run/lock/i074-stage2.active`**：`/run/lock` 是 1777 的 tmpfs（2026-09-29 實查），⛔ 不受 `XDG_DATA_HOME` 影響，所以同一帳號的**不同執行目錄、不同 XDG root、不同 repo** 都搶同一把；兩者都**隨開機週期消失**。⚠️ 第十四輪宣稱的「不同帳號也互斥」**撤回**（第一個帳號建立的檔案，其他帳號可能開不了、sticky bit 下也刪不掉或修不了） |
| 2 | 鎖 | 專用的 supervisor（host 端、只用標準庫；⚠️ 先驗自己的檔案內容 ＝ HEAD 中的版本）以 `O_RDWR \| O_CREAT \| O_NOFOLLOW \| O_CLOEXEC`、mode `0600` 開啟鎖檔，**開啟之後以 `fstat` 驗**：regular file、owner ＝ 執行帳號、mode 恰好 `0600`、link count ＝ 1，並以 `lstat` 確認路徑仍指向**同一個** `st_dev` ＋ `st_ino`（⛔ 中途被換掉即中止）；⛔ **永不 unlink** 鎖檔；lock fd ⛔ 不傳給任何子程序。任一不符（例如被預先建成 symlink、非 regular file、owner 不對）→ ⛔ fail-closed（1／8）。非阻塞 `flock` 拿不到 → 完整 orchestrator 模式回 **1**、`--promote` 模式回 **8**（若實作改用 `flock(1)`：前者用預設、後者用 `-E 8`，2026-09-29 實測） |
| 3 | 建立 sentinel（第十六輪訂正順序） | ⚠️ 取得鎖、確認⛔ 沒有舊 sentinel、**第 7 列的殘留容器檢查通過之後**、**持鎖階段的 workload child 成功啟動之前**，以 `O_WRONLY \| O_CREAT \| O_EXCL \| O_NOFOLLOW \| O_CLOEXEC`、mode `0600` 建立 sentinel，`fstat` 驗 regular file、owner、mode、link count ＝ 1，寫入 canonical JSON、fsync 檔案與目錄。**已存在** → ⛔ fail-closed：完整 orchestrator 模式回 **1**（⛔ 不進 replay、⛔ 不計入正式 scan）、`--promote` 模式回 **9**，印出內容（讀不懂也照樣拒絕）與「**需要重開機**」。⚠️ **自己建的 sentinel 的收回（第十六輪；第十七輪釘住邊界與刪除前的檢查）**：邊界是「**持鎖階段的 workload child 成功啟動（`exec` 成功）之前**」——workload child 是第一個會進入正式持鎖流程、可能產生 Stage 2 狀態的子程序；殘留容器檢查的 `docker ps` 在建立 sentinel **之前**就已結束，⛔ 不算。這段期間的任何可控失敗（寫入或 fsync 失敗、屬性驗證失敗、設定 subreaper 失敗、workload child 啟動失敗等），supervisor 依序：①若曾產生過 child（例如 `fork` 成功而 `exec` 失敗），先 **reap**，並確認⛔ 沒有活著的後代、`docker ps -a --filter label=i074.stage2.run=<token>` **成功且為空**；②以**建立時保存的 sentinel fd** 做 `fstat`、對路徑做 `lstat`，確認兩者是**同一個** `st_dev` ＋ `st_ino`——不符（路徑被換成別的 inode）→ ⛔ fail-closed、⛔ **不 unlink**；③都成立才刪掉這一份、fsync 目錄、再放鎖——⛔ 不需要重開機。任一步無法完成 → sentinel 留下（等同第 9 列） |
| 4 | sentinel 的封閉 schema（第十五輪補型別） | canonical JSON，鍵集合**恰好**如下，多欄、缺欄、型別不符一律拒絕：`schema`（string，恰好 `"i074_stage2_active_run/v1"`）；`token`（string，恰好 64 個小寫十六進位字元，supervisor 每次執行以密碼學亂數產生）；`boot_id`（string，小寫 UUID `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`，⚠️ 必須 ＝ 目前的 `/proc/sys/kernel/random/boot_id`）；`uid`（嚴格整數、⛔ 不接受 `bool`、`>= 0`，⚠️ 必須 ＝ 執行帳號的 uid）；`supervisor_pid`（嚴格整數、⛔ 不接受 `bool`、`> 0`）；`supervisor_start_time`（嚴格整數、⛔ 不接受 `bool`、`> 0`，取自 `/proc/<pid>/stat` 的 starttime）；`mode`（string，恰好 `orchestrator` 或 `promote`）。⚠️ 所有字串都受上面的正規式或列舉限制，因此⛔ 不可能含 NUL 或控制字元。⚠️ schema 只用於寫入與診斷——sentinel **一旦存在就拒絕**，⛔ 不因為內容讀不懂或過期而放行 |
| 5 | 本趟的範圍（第十五輪簡化） | 本趟 ＝ **supervisor 的所有後代** ＋ **帶本趟 token label 的容器**。supervisor 設為 **child subreaper**（`PR_SET_CHILD_SUBREAPER`）——supervisor 還活著時，後代即使 `setsid`、父程序也結束，孤兒仍回到它底下（2026-09-29 實測）。「無關的 helper」只指 supervisor **之外**啟動的程序。⚠️ 第十四輪的「環境變數 token 繼承 ＋ 掃 `/proc`」**撤回**——單次列舉 `/proc` 有 fork／exit 的 TOCTOU（列舉之後才 fork 出的子程序、而 parent 在被讀到之前就結束），證明不了歸零；supervisor 死後改由重開機處理（第 9 列），⛔ 不需要它 |
| 6 | 容器 label | token 由 supervisor 每次執行產生（⛔ 不可由使用者注入或覆寫）：⑩ 的**每一個** `docker run`／`docker create` **恰好**帶一個 `i074.stage2.run=<token>`；使用者自帶同一個鍵（重複、偽造）→ ⛔ 拒絕；清理用完整的 `key=value`，殘留檢查用**鍵**；`docker ps` 查詢失敗 → ⛔ fail-closed（⛔ 不得當成「沒有殘留」）。⚠️ **注入點只能有一個**（docker shim 或修改 runner／finalizer 二擇一，⛔ 不得並存），由 ⑦ 的計畫書定 |
| 7 | 新 supervisor 的啟動檢查（第十六輪訂正順序） | 驗執行帳號 → 取鎖（第 2 列）→ 確認⛔ 沒有舊 sentinel → ⚠️ **查殘留容器**：`docker ps -a --filter label=i074.stage2.run` **成功且為空**（任何帶這個鍵的容器，執行中或已停止；含重開機之後殘留的）→ 才建立自己的 sentinel（第 3 列）。⚠️ 有殘留容器、或 `docker ps` 失敗 → 完整 orchestrator 回 **1**、`--promote` 回 **8**，⛔ 不進 replay——此時**還沒建立 sentinel**，排除原因之後可以直接重跑、⛔ 不需要重開機（第十五輪的順序是「先建 sentinel 再查」，失敗時會留下 sentinel、只能重開機，rc=8 的「排除原因後重跑」因此失去意義——撤回） |
| 8 | 正常釋放 | orchestrator 結束、或 supervisor 收到 TERM／INT／HUP 時：依 label 移除本趟的容器 → 對所有後代送 TERM、逾時升級 KILL（依 pid 樹，⛔ 不用 `pkill -f`）→ **確認已無活著的後代（zombie 不算）、`docker ps -a --filter label=i074.stage2.run=<token>` 成功且為空** → ⚠️ 以建立時保存的 sentinel fd 做 `fstat`、對路徑做 `lstat`，確認是**同一個** `st_dev` ＋ `st_ino`（不符 → ⛔ 不 unlink、⛔ 不放鎖；第十七輪）→ 刪 sentinel、fsync 目錄 → 放鎖。清不空 → ⛔ 不刪 sentinel、⛔ 不放鎖，持續回報、等人工處理 |
| 9 | ⚠️ supervisor 被 SIGKILL 之後（第十五輪，✅ 使用者裁決：重開機才解除） | ⚠️ **沒有解除入口**。sentinel 只會在三種情況消失：①第 8 列的正常釋放；②第 3 列的「持鎖階段的 workload child 成功啟動之前的可控失敗」由 supervisor 收回自己建的那一份；③**重開機**（`/run/lock` 是 tmpfs）。**除第 3 列明定的安全收回之外**，supervisor 被 SIGKILL（⚠️ 含這台 2 GiB host 上可能發生的 **OOM killer**），或任何**已啟動本趟 workload child、卻沒完成第 8 列清理**的結束 → sentinel 留下（第十七輪訂正：原本寫「任何沒走完第 8 列的結束」，與第 3 列的收回互相矛盾）→ 新的 supervisor 一律 fail-closed（1／9），**直到重開機**；重開機之後由第 7 列確認沒有殘留的 Stage 2 容器才放行。⛔ **操作者⛔ 不得手動刪除 sentinel**（它是唯一一道跨 supervisor 存活的並行守門）。⚠️ 代價：重開機會中斷這台 host 上的 live 服務（使用者已知並接受）。⚠️ 第十三、十四輪的解除入口與條件（「supervisor 不在 ＋ 沒有容器」、token 掃描）**撤回** |
| 10 | 持鎖的驗證 | 複本內的 orchestrator 在完整性檢查之前與每個檢查點讀 `/proc/locks`，驗「鎖檔（`st_dev` ＋ `st_ino`）上有 `FLOCK`、持有者 pid 是自己的 supervisor」，⛔ 不符即中止（supervisor 不在了就⛔ 不發布、⛔ 不晉升）；⛔ **不用「在 fd 上再 flock 一次成功」** |
| 11 | orchestrator 的中斷處理 | supervisor 以 parent-death signal（TERM）啟動子程序；orchestrator 的長步驟**一律背景執行 ＋ `wait`**（bash 要等前景命令結束才處理 trap，2026-09-29 實測），中斷時依 label 移除容器、結束自己的後代 |
| 12 | 撤回的做法 | 第十輪的 fd 繼承；第十一輪的 `flock(1)` wrapper；第十二輪以 session 定義本趟與「`setsid` 即無關」；第十三輪的 XDG 位置與「supervisor 不在 ＋ 沒有容器」的解除條件；第十四輪的「不同帳號也互斥」、token 繼承、`/proc` 掃描與解除入口 |

##### 八之二、freeze record 的封閉 schema（v29 第三輪新增、第四輪寫成封閉契約、第五輪改為「複本釘哪個 OID」的依據；✅ 已隨 v29 確認（2026-09-29）；⚠️ ⑦ 總綱 v1 加 tooling 兩欄（✅ 2026-09-30 確認））

由 `--formal` sizing 在 `status = "ok"` 時寫出 `<work>/freeze_record.json`（validation 模式、`assumption_violated`、任何失敗路徑⛔ 不寫）。
它決定 ⑩ 的**複本釘在哪個 OID**，並證明那個 OID 已經過確認重跑；⚠️ 門檻仍是寫死的 `P_B_BUDGET`，其中的 `p_b_bytes` 只用來驗
≤ 預算、⛔ 不當門檻。⚠️ 它⛔ **不進證據鏈**——⑩ 之後沒有任何步驟需要回頭驗它；⑩ 的執行紀錄（本筆）記下它的 SHA-256 與
`repo_head`，原件隨執行目錄保存到晉升結果 commit 之後。

編碼：與證據相同的 **canonical JSON**（`replay_bundle/canonical.py`）；⚠️ **鍵集合必須恰好等於下表**——多欄、缺欄、型別不符
（含以 `bool` 冒充整數）一律拒絕。**hex64** ＝ 恰好 64 個字元的小寫十六進位（`^[0-9a-f]{64}$`），**oid40** ＝ 恰好 40 個字元的
小寫十六進位。

| 欄位 | 型別 | 規則 |
|---|---|---|
| `schema` | string | 恰好 `"i074_stage2_freeze_record/v1"` |
| `mode` | string | 恰好 `"formal"` |
| `status` | string | 恰好 `"ok"` |
| `run_id` | string | sizing 的 run id（`^[0-9]{8}T[0-9]{6}Z-[0-9]+$`） |
| `repo_head` | oid40 | 確認重跑時真正 repo 的 HEAD commit OID——⑩ 的複本就 checkout 這一個 |
| `clone_head` | oid40 | sizing 複本的 HEAD；⚠️ 必須 ＝ `repo_head` |
| `base_commit` | oid40 | 恰好是 `e1cbbbd` 的完整 OID |
| `p_b_bytes` | int | > 0；⚠️ 必須 ＝ 報告的 `P_B`，且 ≤ `P_B_BUDGET` |
| `report_sha256` | hex64 | 同目錄 `sizing_report.json` **檔案內容**的 SHA-256 |
| `harness_sha256`／`shim_sha256`／`helper_sha256` | hex64 | `scripts/i074-stage2-sizing.sh`、`scripts/lib/i074-sizing-docker-shim.sh`、`python/scripts/i074_stage2_sizing.py` 在 **`repo_head` 中的檔案內容**的 SHA-256（⛔ **不是** git blob OID——blob OID 有 `blob <len>\0` 前綴） |
| `counterfactual_patch_raw_sha256` | hex64 | sizing 使用的 counterfactual patch 檔**原始 bytes** 的 SHA-256 |
| `counterfactual_patch_sha256` | hex64 | 在 `base_commit` 上 `git apply --index` → `git write-tree` 得 `T1`，`git diff --binary <base> <T1>` 的 SHA-256（③「四之一」的定義）；⚠️ 必須 ＝ `counterfactual_patch_raw_sha256`（③「四之一」第 1 條：封存的 patch bytes 就是 canonical diff）。⚠️ **⑦ 總綱 v1（✅ 2026-09-30 確認）**：canonical diff 改為「⑦ 總綱 v1」「二」的 canonical diff（`--full-index` ＋ 全部釘死的參數、在隔離的暫存 bare repo 計算；⚠️ 第一輪 review：只加 `--full-index` 不夠），tree-to-tree 計算 |
| ⚠️ `tooling_patch_raw_sha256`（**⑦ 總綱 v1 新增，✅ 2026-09-30 確認**） | hex64 | sizing 使用的 tooling patch 檔**原始 bytes** 的 SHA-256；⚠️ ⑩ 的 tooling ⛔ 不得為空，所以⛔ 不得等於空字串的 SHA |
| ⚠️ `tooling_patch_sha256`（**⑦ 總綱 v1 新增，✅ 2026-09-30 確認**） | hex64 | 在 `T1` 上 `git apply --index` tooling → `git write-tree` 得 `T2`，canonical `diff <T1> <T2>` 的 SHA-256（③「四之一」第 2 條）；⚠️ 必須 ＝ `tooling_patch_raw_sha256` |
| `image_id` | string | `sha256:` ＋ hex64；必須 ＝ Stage 2 identity 的 `expected_image_id` |
| `identity_sha256` | hex64 | Stage 2 identity 檔內容的 SHA-256 |

**交叉不變條件**：報告的 `mode`、`status`、`run_id`、`repo_head`、`clone_head`、三個腳本 SHA、image、identity SHA、counterfactual
raw SHA 都要與 freeze record **逐欄相等**（報告由 `report_sha256` 綁定）。⚠️ **⑦ 總綱 v1（✅ 2026-09-30 確認）**：報告同步記錄 tooling raw SHA，並納入逐欄相等。

**驗證**（⛔ 任一不符即中止、replay ⛔ 未被呼叫、結束碼 1、⛔ 不計入正式 scan）：
真正 repo 的 orchestrator（進入複本之前）驗 schema、`mode`、`status`、`p_b_bytes ≤ P_B_BUDGET`，以及 `repo_head` 存在於真正 repo；
複本內的 orchestrator（preflight）驗上表全部與交叉不變條件、報告 SHA、複本 `HEAD` ＝ `repo_head`、三個腳本 SHA ＝ 複本中對應檔案
內容的 SHA、兩個 counterfactual SHA ＝ 本次凍結的 patch（raw bytes 的 SHA ＝ runner 由 `T1` 算出的增量 diff SHA）、⚠️ 兩個 tooling SHA ＝ 本次凍結的 tooling patch（raw ＝ `T1..T2` 的增量 diff SHA；⑦ 總綱 v1，✅ 2026-09-30 確認）、image 與 identity
檔 SHA ＝ 目前的 Stage 2 identity。⛔ **沒有接受裸 `repo_head` 的入口**。

##### 八之三、晉升（promotion）契約（v29 第五輪新增、第六輪補完整驗證、第七輪補結束碼、第八輪收斂成單一冪等入口；✅ 已隨 v29 確認（2026-09-29））

把複本內已發布的終態搬進真正 repo 的唯一途徑。⚠️ **第六輪訂正**：複本內的 `evidence/`、`failed/` 是**未追蹤**的產物，⛔ 不受
複本完整性檢查保護——所以晉升在 rename **之前**對 staging 做與 recovery 相同強度的完整驗證。⚠️ **第八輪收斂**：第七輪的唯讀
`--status`、`--recover-promotion` 與 P0～P5 矩陣**撤回**（`--status` 的完整驗證其實會寫複本的 git metadata，⛔ 不是唯讀；矩陣的
條件會重疊或漏接），改成**單一、冪等的 `--promote`**：任何失敗或崩潰之後，唯一的下一步都是依結束碼決定「重跑它」或「停下」。
⚠️ `--promote` **⛔ 不呼叫 ③ 的 recovery 模式**——failed record 的 `--recover-failed-record` 成功與一般失敗都回 1（實查
`stage2_archive.py` 的 `run_stage2()`／`main()`），分不出來；來源的 durability 改由 `--promote` 自己 fsync，完整驗證則在逐位元相同的
staging 上做。

| 項目 | 規格 |
|---|---|
| 對象 | 複本內**本次**的終態：成功 archive（`evidence/`）或 failed record（`failed/<bundle_id>-<本次凍結的 counterfactual SHA>/`；⚠️ **⑦ 總綱 v1 第一輪 review（✅ 2026-09-30 確認）**：目錄鍵改用本次凍結 counterfactual 的**語意 SHA**） |
| 目的地 | 真正 repo 的 `python/baselines/i074_stage2/` 下**同一個相對路徑**；⛔ 不覆寫、⛔ 不合併 |
| 冪等與互斥 | 在任何時點重跑都安全：目的地不存在才建立（`rename_noreplace`）；已存在且與來源完全相同、完整驗證通過，只補 fsync；其他情況⛔ 不動它。⚠️ **互斥**：與 ⑩ 的 bootstrap **同一把鎖、同一個 supervisor、同一個 sentinel**（「八之一之二」）——鎖在 `/run/lock`、只允許固定的執行帳號（第十四、十五輪），所以同一帳號的**不同執行目錄、不同路徑寫法、不同 repo**都搶同一把。⚠️ 鎖的是整個 parent、⛔ 不是單一目的地——步驟 2 清 orphan 的範圍就是整個 parent，只鎖單一目的地仍會讓兩個不同目的地的晉升互清 staging。⑩ 的流程裡，鎖從 bootstrap（建立複本**之前**）持有到晉升結束，流程內的晉升沿用它；**單獨執行**的 `--promote`（例如 rc=3／8 之後重跑）同樣由 supervisor 取得鎖（`--promote` 模式）。拿不到鎖 → **8**；該目錄不存在或開不起來 → **9** |
| ⚠️ 會寫到哪裡（第八輪照實寫明） | 真正 repo 的 `python/baselines/i074_stage2/` 下的 staging 與目的地；**複本的** `.git`（shell 端合成守門的 worktree 登記與 `git apply --index`／`git write-tree` 寫的 object，結束時清掉 worktree）；執行目錄的暫存目錄。⛔ **不寫真正 repo 的 `.git`**；⛔ 不改複本內的終態（只 fsync）。崩潰留下的 worktree 登記由下一次 `--promote` 的 `git worktree prune`（複本）清掉（⚠️ **⑦c 細部計畫 v1（✅ 2026-10-01 確認）**：最後一句照實改寫——prune 只清實體目錄已消失的登記，實體目錄還在的保留到 `<work>` 刪除、⛔ 不影響冪等；staging 名稱 `.promote-staging-<16 hex>`，由 `.gitignore` 排除） |
| ⚠️ 驗證用的錨點 | **一律取自複本（釘住的 OID）**：Stage 1 錨點（`python/baselines/i074_stage1/`）與 `envcheck/` 讀複本內的那一份；identity 用 archive 封存的那一份（`identity/run_identity.json.gz`），並驗它等於複本 preflight 時的 Stage 2 identity；兩份 patch 用凍結副本。⚠️ ③ 的 verifier 若只接受它自己 repo 內的路徑，⑦ 補一個**只讀**、可指定錨點根目錄的驗證入口（⛔ 不放寬強度）（⚠️ **⑦c 細部計畫 v1（✅ 2026-10-01 確認）**：唯讀入口 ＝ `finalize-stage2-evidence.sh --verify-promotion-staging`——真正 repo 由複本的 `origin` 推導、只接受 staging 與目的地兩種路徑、全部唯讀掛載；identity 的綁定以「終態封存的 identity 的 canonical bytes 的 SHA ＝ `preflight.identity_sha256`」實作，⑦c「二之三」第 3c 步） |
| ⚠️ 信任根的綁定（第九輪） | ⚠️ 成功 archive 在 6a 與 6b 的完整驗證**另驗三者相等**：`evidence_manifest.json` 的 `finalizer_provenance.base_commit` ＝ freeze record 的 `repo_head` ＝ 複本的 `HEAD`。⚠️ 它是 B／C 判讀器取錨點的信任根（下一列），而 ③ 的 `verify_stage2_graph()` 只驗 finalizer provenance 的 schema 與 image 鏈、⛔ 沒有這一條（實查 `stage2_archive.py` 的 `verify_stage2_graph()`）；官方 finalizer 正常執行時由 `--source-ref` 算出這個值，但那⛔ 擋不住「finalize 之後、晉升之前竄改來源」——所以由 `--promote` 補。不相等 → **9**。⚠️ failed record ⛔ 不進 B／C，⛔ 不適用這一條 |
| ⚠️ 真正 repo 的錨點（第六、七輪） | ⛔ **晉升與判讀都⛔ 不讀真正 repo 工作樹裡的錨點**。成功 archive 的 manifest 永久記錄 `finalizer_provenance.base_commit`——finalizer 程式碼的來源 commit，也就是複本釘住的 OID（實查 `finalize-stage2-evidence.sh`：`--base-commit` 就是它）。⚠️ **B／C 判讀器從這個 commit 的 git 物件取出 Stage 1 錨點與 `envcheck/`**（例如 `git archive <base_commit> -- <路徑>` 解到暫存目錄），⛔ 不讀工作樹——所以真正 repo 之後不論改工作樹或 **commit** 改掉錨點，都⛔ 影響不了已發布的 archive 與判讀（第六輪寫的「`git checkout <OID>` 還原」撤回：OID 沒有定義，自動 checkout 也可能覆蓋使用者的修改）。⚠️ 唯一的前提：**`base_commit` 必須一直可達**——⛔ 不得改寫會讓它消失的歷史；它不可達時判讀器 fail-closed。⚠️ 錨點是已 commit 的終態證據，改動它本身仍是另一件事故，另立 issue（⚠️ **⑦c 細部計畫 v1（✅ 2026-10-01 確認）**：取錨點的方式改成「`git clone --no-hardlinks` ＋ detached checkout `base_commit`」、⛔ 不用 `git archive`；驗證與判讀在同一個 Python 程序） |
| 判讀 | ⚠️ B／C 判讀器讀**真正 repo 中已晉升**的成功 archive，判讀前以 ③ 的完整 verifier 驗過（錨點取自 `base_commit`——它在晉升時已驗等於複本釘住的 OID，見「信任根的綁定」）；驗不過就拒絕 |
| 晉升之後 | 複本與執行目錄保留到晉升結果 **commit 之後**才可清除；failed record 晉升之後**必須 commit，才能開始下一次 ⑩**（「八之一」的真正 repo 例外） |
| ⑨-1 | ⚠️ acceptance ⛔ **不晉升**（它的終態留在自己的複本，只驗容量與記憶體） |

**`--promote` 的判定順序**（⚠️ 依序執行；⛔ 前一步沒通過就⛔ 不做後一步；未列出的任何例外一律 **9**，⛔ 不猜）：

| 步 | 動作 | 不通過時 |
|---|---|---|
| 1 | 複本完整性（「八之一」：⛔ 只用 git 與 shell、⛔ 不 import repo 內的模組） | **9** |
| 2 | 複本的 `git worktree prune`；清掉目的地 parent 下**可辨識**的 orphan staging（⛔ 不動其他東西） | **9** |
| 3 | **解析本次的唯一終態**：複本的 `python/baselines/i074_stage2/` 底下的**未追蹤**項目（可辨識的 orphan staging 除外）必須**恰好一個**，而且是 `evidence/` 或 `failed/<bundle_id>-<本次凍結的 counterfactual SHA>/`（⚠️ **⑦ 總綱 v1 第一輪 review（✅ 2026-09-30 確認）**：語意 SHA） | 零個、多個（例如 `evidence/` 與本次的 failed record 同時存在）、不認得的項目、讀取錯誤 → **9** |
| 4 | **來源的 durability**：fsync 終態的每個檔案、每個目錄與它的 parent | **3**（重跑 `--promote`） |
| 5 | 目的地是否存在（`lstat`） | 讀取錯誤 → **9** |
| 6a | **目的地已存在**：與來源逐位元比對（檔案清單、目錄結構、每檔 SHA-256）→ 對目的地做**完整驗證**（成功 archive 含「信任根的綁定」）→ fsync 目的地的每個檔案、每個目錄與 parent | 比對或驗證不符 → **9**（⛔ 不覆寫）；fsync 失敗 → **3** |
| 6b | **目的地不存在**：同一個裝置、空間預檢（`f_bavail × f_frsize` ≥ 來源 allocated bytes 的 2 倍）→ 建 staging、逐檔**逐位元複製**、比對 → 對 staging 做**完整驗證**（成功 archive：`verify_stage2_graph()` 全部各道 ＋ shell 端的合成守門 ＋「信任根的綁定」；failed record：F1～F10）→ fsync staging → `rename_noreplace` → parent fsync | 空間不足、複製／比對的 I/O 錯誤、`rename_noreplace` 失敗（含 `EEXIST`——重跑時會走 6a）→ **8**；⚠️ staging 與來源**逐位元相同卻驗不過**（代表來源本身無效）→ **9**；stat 錯誤 → **9**；parent fsync 失敗 → **3**。任何非 0／6 的結束都清掉本次的 staging |
| 7 | 成功 | 成功 archive **0**、failed record **6** |

⚠️ **⑦c 細部計畫 v1（✅ 2026-10-01 確認）**對本表的修改（實作見 ⑦c「二之三」）：第 3、4 步之間插入**封閉 inventory**（3b：`lstat`、⛔ 不跟隨 symlink，只接受一般檔案（`st_nlink` ＝ 1）與目錄）與 **identity 的綁定**（3c）；所有操作錨定在持有的 fd 上、rename 前後各做一次鏈檢查；第 5 步之後插入 6a／6b 共用的第 5b 步「目的地的 ignore 守門」、6b 開頭加「staging 的 ignore 守門」；6a 在驗證之後、6b 在 rename 之前加「收尾重算」；⚠️ ⑦c 實作第一輪 review：failed record 在 rename 之前一律 fsync `i074_stage2/`、rename 之後兩個 parent 都 fsync、6a 另 fsync `i074_stage2/`，錯誤清理只刪本次建立的那一個 inode（第二、三輪 review：名稱已不存在也回 9，清理途中每個時點都核對）；「不通過」欄依「結束碼的分類」細分——第 4 步的開檔與 `fstat` 錯誤由 3 改成 **9**（只有 `fsync()` 本身失敗是 3）、6b 的 8 限縮為目的端的容量與寫入 I/O 及 rename 的 `EEXIST`／`ENOTEMPTY`，rename 的其他錯誤（含 `EXDEV`、`EINVAL`、`ENOSYS`）與來源側錯誤改成 **9**。

⚠️ **端到端結束碼（orchestrator；`--promote` 同一套）**：**0** ＝ 成功 archive 已在真正 repo durable；**6** ＝ 反事實失效、failed record
已在真正 repo durable；**2** ＝ preflight 的 lookup 命中；**1** ＝ 還沒有任何終態（preflight、replay、finalize／publish 在發布之前失敗），
⛔ 沒有證據；**3** ＝ 某一層的 durability 未確認 → **重跑 `--promote`**；**8**（`EXIT_PROMOTION_FAILED`）＝ **本次呼叫沒有確認晉升完成，但可以安全重跑 `--promote`**
（先排除原因，例如釋出空間、等另一個 `--promote` 結束）——⚠️ **⛔ 不承諾目的地不存在**（第九輪訂正：拿不到鎖、`rename_noreplace` 遇到
`EEXIST` 時，目的地可能已經存在或即將存在），重跑時由步驟 5／6a 依磁碟事實裁決；**128 ＋ N** ＝ orchestrator 被訊號 N 中斷（supervisor 清空本趟之後原樣回傳；⚠️ **只適用 orchestrator 或 supervisor 攔截得到的訊號**——supervisor 自己被 **SIGKILL** 時由作業系統直接回 **137**，⛔ **沒有完成清理**，sentinel 留下、新的 supervisor 一律 fail-closed，**直到重開機**，見「八之一之二」第 9 列）→ 執行 `--promote`，由它的判定順序裁決（沒有終態 → 9，交人工依計次政策處理）；**9**（`EXIT_PROMOTION_BLOCKED`）＝ **需要人工判斷**：⛔ 不得自動重跑，停下、另立
issue——確認原因只是暫時性的 I/O 問題之後，才可以再執行 `--promote`（它是冪等的，⛔ 不會覆寫）。⚠️ 複本內終態的 rc=3（finalize
的 parent fsync 失敗）也由 `--promote` 的步驟 4 收尾，⑩ 的流程⛔ 不呼叫 ③ 的 recovery 模式。⚠️ `finalize-stage2-evidence.sh` 各模式的
結束碼（③，例如 failed record 發布完成回 1）**⛔ 不變**——那是另一層。現有結束碼：1～5、7 已用（2 是 `--check-failed-record` 的命中），
6 已規劃給反事實失效，8、9 新增。

#### ② before tooling／反事實謂詞設計 v2（2026-09-22；✅ **2026-09-23 review 通過**，見「② 的 review 結論」）

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

#### ② 執行結果（2026-09-22；✅ **2026-09-23 review 通過**，見「② 的 review 結論」）

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
⛔ 不牴觸 `replay-args.sh:154` 的 `replay_args_prepare_worktree()` HEAD 斷言；
② **stored patch 重新套用後的增量 diff SHA 等於原值**（三方一致）；
③ 空 tooling patch 的 SHA 有明確值，⛔ 不是缺欄位。

##### ② 的 review 結論（2026-09-23，✅ **通過**）

✅ **步驟 ② 在其既定範圍內通過 review**。review 端**實際複核**的事實：

| 項目 | 結果 |
|---|---|
| 套用 | patch **乾淨套到** `e1cbbbdab44f8cf2d152e6ade9235d844f590d7f`；套用期間 HEAD 仍是原 base commit |
| tree 與 SHA | base tree `6bbd804e49e329dd85103aa2f510ec07760a9609`；**重建的 `T1`** `a1649733b910ae90f08efc5f215f31f898619f52`；canonical diff SHA-256 `ef7a4cdf23dce70cda77abe8d49cdcc8e3a3829c0212b4202e747b343c4b4f93`——三者都與上文一致 |
| 範圍 | patch **只動 4 個預期檔案**；所有 `resolve_lifecycle()` 呼叫點都已補參數 |
| 語意 | **產品語意變因只有「把 setup RR 加回 `CONTINUATION`」**；`rr_qualified` 的取值搬移⛔ 不改變 `rr_gate` 的值 |
| 正向測試 | `test_counterfactual_continuation_requires_setup_rr()` 確實跑**真實的 semantic pipeline**，⛔ 不是直接偽造 lifecycle 結果 |
| 三份 log | SHA 與文件一致：baseline 1222 passed／1 skipped；patched 第 1 次是**預期中**的一筆真值表失敗；patched 第 2 次 1223 passed／1 skipped |
| 揭露 | 文件正確揭露：shell argv 測試未執行、patched 第 1 次的 tree 未保存且⛔ 不作正式驗收證據、本步驟⛔ 沒有跑 replay |

⚠️ **非阻擋限制**：log 只能獨立證實 Docker image ID 的**前 12 碼**；上表「測試的執行身分」裡的完整 64 碼是另行記錄的，
⛔ 無法只憑 log 完整重算。⚠️ image ID ⛔ 不是步驟 ② 的正式驗收錨，⛔ 不影響本次通過。

##### 仍未做（⛔ 不屬 ②）

⛔ **replay ⛔ 沒有跑**。⚠️ `--i074-counterfactual`、`--counterfactual-patch-sha256`
與 runner 的兩份 patch 輸入**都還沒實作**——它們屬於 ⑦，
⚠️ 而 schema 與 manifest 欄位要等 ③ 的 evidence contract 定案。

#### ③ Stage 2 evidence contract 計畫書 v12（2026-09-23，✅ **已確認**）

✅ **使用者於 2026-09-23 與 Stage 2 計畫書 v28 一起確認**。

⚠️ **v12 隨 Stage 2 計畫書 v28 修掉 v11 review 的兩項**：

| # | 問題 | v12 的處置 | 同步的節 |
|---|---|---|---|
| 1 | ⚠️ **envcheck 沒定義 key 集合不一致的證據形式**——`rows_compared` 的語意、單側 key 算不算 `row_mismatch_count`、sample 如何標示缺在哪一側，都有兩種以上合法解讀 | ⚠️ 新增 `reference_key_count`／`witness_key_count`／`key_mismatch_count`／`key_mismatch_sample_keys`（每項帶 `side` ∈ {`REFERENCE_ONLY`, `WITNESS_ONLY`}）；寫死 `rows_compared`＝**交集**、`row_mismatch_count` **只計交集內 bytes 不同的 key**；加兩條計數不變條件，並把 `outcome` 的判定式寫成封閉條件 | 三之三、十（`be2`） |
| 2 | ⚠️ **rc=6 的舊敘述與作廢 schema 殘留**——failed record 開頭只寫 `before_candidates != ∅`；`bounded_diagnostics` 那一列混著 v10 的單一 schema | ⚠️ 開頭改成「任一反事實生效條件不成立」；`bounded_diagnostics` 那一列只留現行契約（以 F5～F7、F10 為準），並寫入 N＝20 的使用條件 | 七 |

#### （承上）③ evidence contract 計畫書 v11（2026-09-23）的內容

⚠️ **v11 隨 Stage 2 計畫書 v27 修掉 v10 review 的三個阻擋點**（另外兩點是次數上限與凍結窗口，只改 Stage 2 計畫書）：

| # | 阻擋點 | v11 的處置 | 同步的節 |
|---|---|---|---|
| 1 | ⛔ **NOT_EQUIVALENT 的 archive 無法通過自己的發布與 recovery**——E3 要求重算結果「必須是 EQUIVALENT」；envcheck manifest ⛔ 沒有 schema，recovery 的 outcome 也沒說取自哪裡 | ⚠️ E3 拆成 **E3a／E3b** 並寫明各呼叫端做哪一層；補 **envcheck manifest 的封閉 schema**；outcome **以 equivalence artifact 為唯一來源**，manifest 的 `terminal_outcome` 只是推導值、三者必須一致 | 三之三、五之零、五、六、七之二、七之四、十 |
| 2 | ⛔ **failed record 的 schema 容不下新的兩種 rc=6**——只有一種 `failure_reason`、計數必須是正整數、sample 沒有 `rr_decoupling_candidate` | ⚠️ `failure_reason` 改成兩種（`candidate_flag_inconsistent`／`rr_not_restored`），**優先序由檢查順序決定**；`bounded_diagnostics` 改成依原因分流的**封閉 union**；新增 F10（sample 必須真的違反它宣稱的那一條） | 七、七之四、十 |
| 3 | ⛔ **串流驗證與「既有 validator 一律不動」互相矛盾** | ⚠️ 改成「**公開 contract 不動、允許內部重構成共用的 row-level 原語**」 | 五之一、八、九、十 |

#### （承上）③ evidence contract 計畫書 v10（2026-09-23）的內容

⚠️ **v10 隨 Stage 2 計畫書 v26 改寫**（觸發：2026-09-23 開工前盤點，見 v26 的修訂表）。
⛔ **v10 與 v26 一起確認前，⛔ 不得實作**。

| # | 缺口 | v10 的處置 | 同步的節 |
|---|---|---|---|
| 1 | ⛔ **Stage 1 釘住的 image 已不在本機**——信任錨第 10 道與 F2-a 要求「Stage 2 identity 與 Stage 1 archived identity 逐位元相同」，⛔ 已不可能成立 | ⚠️ 改成「**等於唯一固定的 Stage 2 identity**」；Stage 2 的 image 改由**環境見證錨**與 Stage 1 串接 | 三之一、七、七之一、七之二 |
| 2 | ⚠️ **環境見證證據沒有位置** | ⚠️ 新增「**三之三：環境見證錨**」——`envcheck/` 是**獨立的小 archive**，Stage 2 archive **以它的 manifest 錨定**、⛔ 不複製成員（layout 仍是 7 檔） | 三之零、三之三、四、五之零、五、六 |
| 3 | ⛔ **finalizer／recovery／preflight 的記憶體沒有納入設計**——`load_stage1_anchor()` 回傳整份 `after_payload`，finalizer 還要整份讀 before source，⛔ 必然撞上 730 MiB 的牆 | ⚠️ 新增「**五之一：記憶體模型**」——串流、單次讀取就同時完成 hash 與驗證，只常駐 keys、每列 digest 與 156 筆 cohort rows | 五之一、七之二、十 |
| 4 | ⛔ **finalizer 的 CLI matrix 不完整**——只定義了 `--check-failed-record` 與 `--recover-failed-record`，**成功 archive 的正常發布、durability recovery、failed record 的發布**三個入口都沒有；⚠️ 沒有 matrix 就定不出 argv fixture | ⚠️ 新增「**七之四：finalizer 的 CLI matrix**」（五種模式） | 七之四、八、十 |
| 5 | ⛔ **凍結 patch 的生命週期自相矛盾**——「最終封存只能用凍結副本」與「私有目錄用完就清」同時成立時，另一個步驟的 finalizer 拿不到副本，finalize 重試也拿不到 | ⚠️ 凍結副本放在 **run 目錄底下**，保留到 finalize rc=0 或 failed record 發布完成 | 七之三 |
| 6 | ⚠️ **before source 只驗 `candidate_keys == ∅`，flag 壞掉時會假綠** | ⚠️ 全圖新增第 15 道（Stage 2 計畫書「①之三」的兩條不變條件） | 三之二、五之零、十 |
| 7 | ⚠️ **測試矩陣的階段歸屬**：`n`／`ai`／`ay` 驗的是「runner 在 replay 之前擋下」，⚠️ runner 屬 ⑦——與 v8／v9 修掉的 `ax`／`ba` 是同一類錯 | ⚠️ 拆成兩層：本輪驗**入口／函式的結束碼**，⑦ 驗**runner 在 replay 之前呼叫** | 十、十二 |
| 8 | ⚠️ 成功 archive 的 recovery ⛔ 沒寫要不要重做合成守門（failed record 的有寫） | ⚠️ **要重做**，兩者對稱 | 六 |
| 9 | ⚠️ `bounded_diagnostics` 的 N ⛔ 沒有值；report 的 200 ⛔ 沒寫來源 | ⚠️ **N ＝ 20**（⚠️ v10 當時待確認；✅ v28 已確認）；200 註明取自 bundle 的 `report_max_rows`（實查＝200） | 七、五之零 |

#### （承上）③ evidence contract 計畫書 v9 的內容

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
⚠️ **v10 再加一樣**：Stage 1 的 image 遺失後，新 image 與舊 image 等價的**環境見證**
（`envcheck/` 小 archive，見「三之三」）。

| ⛔ 不做 | 理由 |
|---|---|
| ⛔ **不改 Stage 1 的 `evidence.py`** | ⚠️ **本計畫最高風險**：`ARCHIVE_LAYOUT` 是 9 檔封閉、`validate_evidence_manifest()` 與 `recover_durability()` 都做**精確集合相等**——動它會讓 `python/baselines/i074_stage1/` 那份**已封存的證據立刻驗不過** |
| ⛔ 不複製第二套原子發布原語 | `publish.py` 的 `rename_noreplace`／`fsync_dir`／`probe_no_clobber`／`DurabilityUnconfirmed` **共用**，⛔ 不 fork |
| ⛔ 不改 canonical JSON／gzip 的位元組輸出 | 所有既有 SHA 的根 |
| ⛔ 不改 10 欄的 `PROVENANCE_FIELDS` | 見 Stage 2 計畫書「三之一之二」——加第 11 欄會讓已封存的 Stage 1 artifact 驗不過 |
| ⛔ 不實作 runner 端 | `--i074-counterfactual`、兩份 patch 的輸入與 `write-tree` SHA 計算、orchestrator 是**步驟 ⑦**；⚠️ ③ 只定**它們要產出什麼 schema**。⚠️ **v10 例外（③b）**：stage-scoped identity、專用 tag 的 pin 流程與環境等價比對——它們是**環境閘門**的前置，⛔ 等不到 ⑦ |
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
| **failed-attempt** | `python/baselines/i074_stage2/failed/<bundle_id>-<counterfactual_patch_sha256>/`（⚠️ **完整 64 碼**；⚠️ **⑦ 總綱 v1 第一輪 review（✅ 2026-09-30 確認）**：改成 `<bundle_id>-<counterfactual_semantic_sha256>`） | 見「七」 |
| ⚠️ **環境見證**（v10） | `python/baselines/i074_stage2/envcheck/` | ⚠️ 獨立的小 archive，閘門一過就發布；Stage 2 archive **以它的 manifest 錨定**。見「三之三」 |

⚠️ 所以「三個根目錄」在 v10 變成**四個**（⛔ 標題保留原名，避免斷掉其他節的引用）。

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
| 6 | ⚠️ **Stage 1 manifest 的 `bundle_id` 必須等於 Stage 2 的**——⛔ 否則可以拿別次執行的 evidence 來充數。⚠️ **v10 改寫 image 那一半**：Stage 1 的 `expected_image_id` 只要求**與 Stage 1 自己的 identity 與 provenance 一致**（Stage 1 內部的鏈，見「七之二」⑩）；⛔ **不再要求等於 Stage 2 的**——Stage 2 的 image 改由「三之三」的環境見證錨串接 |
| 7 | ⚠️ **cohort 本身要用既有的 `validate_cohort_manifest()` 驗**，⛔ 不只比 SHA |
| 8 | ⚠️ cohort 的 `after_artifact_sha256` **必須等於已錨定 after 的 `artifact_sha256`** |
| 9 | ⚠️ cohort 的 keys **必須等於該 after 的 `candidate_keys(rows)`**——⛔ 比照 Stage 1 `verify_evidence_graph()` 的同一道，⛔ 不重算 predicate |
| 10 | ⚠️ **v10 改寫**：Stage 2 的 `run_identity` 必須**逐位元等於唯一固定的 Stage 2 identity**——也就是「三之三」環境見證 archive 所封存的那一份（`validate_run_identity()` 驗過後**整個 object 相等**），且其 `bundle_id` 等於 Stage 1 的。⛔ v4～v9 的「等於 Stage 1 archived identity」**已不可能成立**（Stage 1 的 image 已不在本機，見 Stage 2 計畫書「二、⑤」）。⚠️ **理由見下** |

⛔ **為什麼是「完全相等」而⛔ 不是「bundle 與 image 相同就好」**（v4 裁決）：
⚠️ failed record 的**目錄鍵**是 `<bundle_id>-<counterfactual_patch_sha256>`（⚠️ **⑦ 總綱 v1 第一輪 review（✅ 2026-09-30 確認）**：改成語意 SHA；下面「identity 釘死之後兩個鍵一致」的推理不變），
而重跑守門的**查找鍵**原本是「完整 run identity ＋ 完整 SHA」——⛔ **兩者不一致**：
同 bundle／同 patch 但**不同 identity** 的執行會被放行，跑完卻必然撞上同一個發布目錄。
⚠️ 把 identity 釘成「必須等於 Stage 1 archived identity」之後，**identity 不再是變數**，
兩個鍵就一致了。⚠️ **v10**：釘住的對象改成**唯一固定的 Stage 2 identity**（建立一次、封存在
`envcheck/`、after' 與 before 共用），⚠️ **identity 仍然⛔ 不是變數**，這段論證照樣成立。⛔ 另一條路（把 identity digest 放進目錄名）⛔ **不採**——
⚠️ 那等於允許「換個 identity 就能重跑同一份壞 patch」，與「**必須改 patch 才可重跑**」直接衝突（⚠️ **⑦ 總綱 v1 第四輪 review（✅ 2026-09-30 確認）**：「改 patch」指改白名單產品檔、使語意 SHA 改變）。
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
⚠️ **v10**：還要加上 `python/baselines/i074_stage2/envcheck/`——**三者必須一起保存**。

##### 三之三、環境見證錨（v10 新增，✅ 已隨 v28／③ v12 確認（2026-09-23））

⚠️ Stage 1 的 image 已不在本機，Stage 2 改在新 image 上跑（Stage 2 計畫書「二、⑤」）。
**after' 見證趟**與環境等價比對的結果，發布成**獨立的小 archive**，Stage 2 archive 再以它的
manifest 為**第二個信任錨**——⛔ 不複製成員（比照「三之一」⛔ 不複製 Stage 1 after 的理由）。

**`envcheck/` 的封閉 layout**：

```text id="i074_stage2_envcheck_layout_001"
python/baselines/i074_stage2/envcheck/
  witness/after_artifact.json.gz      ← after'：新 image ＋ 原始 e1cbbbd 的全量 replay
  witness/cohort_manifest.json.gz     ← cohort'
  equivalence/equivalence.json.gz     ← 環境等價判定（新 kind）
  identity/run_identity.json.gz       ← ⚠️ 唯一固定的 Stage 2 identity
  evidence_manifest.json              ← root，⛔ 不壓縮
```

⛔ **多一個未知檔案即中止**。⚠️ **4 個 payload ＋ manifest ＝ 5 檔**。

**`equivalence/equivalence.json.gz` 的 schema**（kind `sr_zone_stage2_env_equivalence`）：

| 欄位 | 內容 |
|---|---|
| `schema_version`／`kind`／`bundle_id`／`generated_at` | 同其他 artifact |
| `reference` | Stage 1 D+1 after 的 `artifact_sha256` ＋ Stage 1 manifest 的 SHA（⚠️ 與「三之一」同一個錨） |
| `witness` | after' 與 cohort' 的 `artifact_sha256` |
| ⚠️ `reference_key_count` ／ `witness_key_count`（v12） | 兩側各自的全量 key 數（⚠️ 預期觀測值都是 13,417） |
| ⚠️ `key_mismatch_count`（v12） | ⚠️ **只出現在其中一側的 key 數**（兩側差集的大小總和） |
| ⚠️ `key_mismatch_sample_keys`（v12） | 依 `(symbol, timeframe, as_of)` 排序後的前 `min(N, key_mismatch_count)` 項；⚠️ 每一項的欄位集合**恰好是** `{symbol, timeframe, as_of, side}`，`side` ∈ {`REFERENCE_ONLY`, `WITNESS_ONLY`}（⛔ 封閉列舉；reference ＝ Stage 1 D+1、witness ＝ after'）；⛔ 不重複 |
| `rows_compared` | ⚠️ **v12 訂定：兩側都有的 key 數（交集）**——⛔ 不是聯集、⛔ 不是任一側的筆數；⛔ 也不是完整性證明（完整性由 `key_mismatch_count == 0` 負責） |
| `row_mismatch_count` ／ `mismatch_sample_keys` | ⚠️ **v12 訂定：只計兩側都有、但 canonical row bytes 不同的 key**（⛔ 不含只出現在一側的 key）；sample 依 key 排序取前 `min(N, count)` 個，每一項**恰好是** `{symbol, timeframe, as_of}`（N 同「七」） |
| ⚠️ 計數不變條件（v12） | `reference_key_count == rows_compared ＋ REFERENCE_ONLY 的數量`、`witness_key_count == rows_compared ＋ WITNESS_ONLY 的數量`——⚠️ 兩條都要由 validator 重算，⛔ 不信任封存值 |
| `cohort_equal` | cohort' 的 keys 是否等於 Stage 1 D+1 cohort |
| `provenance_required_equal` | ⚠️ 恰好四欄：`base_commit`／`tooling_patch_sha256`／`project_modules_sha256`／`runtime_settings`，各自是否相等 |
| `provenance_allowed_differences` | ⚠️ 恰好六欄：`image_digest`／`pip_freeze_sha256`／`python_version`／`runner_sha256`／`argv`／`source_root`，各自記兩側的值（⚠️ 與上一列合起來**恰好 10 欄**） |
| `witness_distributions` | ⚠️ 新 image 的**完整** distributions 清單（排序、去重），⚠️ 其 `pip_freeze_sha256()` 必須等於 after' provenance 記的值——⛔ 清單與 hash 對不上即中止 |
| `outcome` | `EQUIVALENT`／`NOT_EQUIVALENT`——⚠️ **validator 由上面各欄重算**，⛔ 不信任封存值。⚠️ **v12**：`EQUIVALENT` ⇔ `key_mismatch_count == 0` ＋ `row_mismatch_count == 0` ＋ `cohort_equal` ＋ 四個必須相同的 provenance 欄位都相同 |
| `comparator_provenance` | `validate_provenance(role="comparator")`（⚠️ 沿用既有 role，⛔ 不新增） |

**`envcheck/evidence_manifest.json` 的封閉 schema**（v11 新增）：

| 欄位 | 內容 |
|---|---|
| `schema_version` | 沿用 `ARTIFACT_SCHEMA_VERSION` |
| `kind` | `sr_zone_stage2_envcheck_manifest`（⚠️ **新 kind**，⛔ 不與 Stage 1 或 Stage 2 archive 的 manifest 共用） |
| `bundle_id` | 等於 Stage 1 manifest 與 envcheck 各檔的 |
| `expected_image_id` | ⚠️ **新 image**，等於 envcheck identity 的 `expected_image_id` |
| `files` | ⛔ key **恰好**是 layout 的四個 payload，全部是 `canonical_artifact` entry（`artifact_sha256`、`stored_sha256`、`stored_bytes`）；⛔ 沒有 `raw_blob` |
| `stage1_evidence` | ⚠️ **與 Stage 2 manifest 同一個欄位定義**（「四」），⛔ 不另訂一份：`manifest_path` 寫死常數、`manifest_sha256`、三個 `members` |
| `terminal_outcome` | ⚠️ **0 或 7**——⛔ **不是獨立來源**，見下方「outcome 的唯一來源」 |
| `generated_at` | 同其他 manifest |
| `finalizer_provenance` | `validate_provenance(role="finalizer")`，`image_digest` 等於 `expected_image_id` |

⛔ 多欄、缺欄、型別不符一律中止。

**outcome 的唯一來源**（v11 新增）：⚠️ **equivalence artifact 的 `outcome`**，而且它本身要由 E3a 重算驗證。
manifest 的 `terminal_outcome` 只是推導值：`EQUIVALENT` → 0、`NOT_EQUIVALENT` → 7。
⚠️ 發布與 recovery 都要求**三者一致**：重算的 outcome ＝ equivalence 封存的 `outcome` ＝ 由 `terminal_outcome` 反推的值；
⛔ 任一不一致即中止（rc=1）。recover-envcheck 成功時回 `terminal_outcome`。

**錨定規則**（⚠️ 與「三之一」同樣⛔ 不得只比 SHA；各呼叫端做哪幾條見下方的呼叫端表）：

| # | 檢查 |
|---|---|
| E1 | `envcheck/` 的 manifest 路徑**寫死常數**、重算 SHA、封閉 layout、每個成員重算兩個 SHA |
| E2 | after'／cohort' 跑完整 validator（同「三之一」對 Stage 1 after 的那一組），identity 跑 `validate_run_identity()` |
| E3a | ⚠️ **archive 自洽**（v11 由 E3 拆出）：**重新串流比對** after' 與 Stage 1 D+1——逐 key canonical row bytes、cohort、四個必須相同的 provenance 欄位——重算出 outcome，⚠️ 必須符合上方「outcome 的唯一來源」的三者一致。⚠️ **`EQUIVALENT` 與 `NOT_EQUIVALENT` 都合法** |
| E3b | ⚠️ **Stage 2 使用資格**（v11 由 E3 拆出）：E3a 通過，**且** outcome 是 `EQUIVALENT`。⛔ NOT_EQUIVALENT 的 archive 是合法證據，⛔ 但**不得**拿來放行 Stage 2 |
| E4 | `witness_distributions` 的 hash 等於 after' 的 `pip_freeze_sha256` |
| E5 | ⚠️ **image 鏈**：envcheck identity 的 `expected_image_id` ＝ after'／cohort' 的 `provenance.image_digest` ＝ `comparator_provenance.image_digest` ＝ Stage 2 的 `expected_image_id` |
| E6 | ⚠️ **bundle 鏈**：envcheck 各檔的 `bundle_id` ＝ Stage 1 manifest 的 ＝ Stage 2 的 |
| E7 | ⚠️ after' 的 `provenance.base_commit` ＝ Stage 1 after 的 `base_commit`（＝ `e1cbbbd`），`tooling_patch_sha256` ＝ 空字串的 SHA |

⚠️ **E3a 是每個呼叫端都要重做的全量比對**——⚠️ 所以它必須串流（見「五之一」），
⛔ 不得為了省時間只比 `row_mismatch_count`。

⛔ **v10 把兩層寫成同一條**（「重算的 `outcome` 必須是 `EQUIVALENT`」），結果 NOT_EQUIVALENT 的 archive
**連自己的發布與 recovery 都過不了**，⛔ 與「NOT_EQUIVALENT 也要封存證據、回 rc=7」直接矛盾。v11 拆開：

| 呼叫端 | 做哪幾條 | 結果 |
|---|---|---|
| `--envcheck`（發布，對 staging 驗） | E1、E2、**E3a**、E4～E7 | **0**（EQUIVALENT）或 **7**（NOT_EQUIVALENT）；證據兩種都發布 |
| `--recover-envcheck` | E1、E2、**E3a**、E4～E7 | 回 manifest 的 `terminal_outcome`（0 或 7） |
| Stage 2 **preflight**（before 之前） | E1～E7，含 **E3a ＋ E3b** | ⛔ NOT_EQUIVALENT 即在 replay 之前中止 |
| Stage 2 `--finalize` | 同上 | ⛔ 同上 |
| Stage 2 `--recover-durability` | 同上 | ⛔ 同上 |

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
| ⚠️ 額外守門 | **`candidate_keys(rows)` 必須為空**——⚠️ 這是 `before_candidates == ∅` 在**證據層**的重複確認（⛔ 執行期擋過一次⛔ 不代表封存的這份也對）。⚠️ **v10**：再加 Stage 2 計畫書「①之三」的兩條（⛔ 沒有任何一列是 `CONTINUATION` 且 `setup_rr_qualified == false`；每列 flag 等於等價式）——⛔ 只看 flag 會在 flag 壞掉時假綠 |

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
| ⚠️ `environment_witness`（v10） | ⚠️ **以 `envcheck/` 的 manifest 為第二個信任錨**：`manifest_path`（寫死常數 `python/baselines/i074_stage2/envcheck/evidence_manifest.json`）、`manifest_sha256`、**封閉的 `members` mapping**（四個 payload 各 `{artifact_sha256, stored_sha256}`）——規則見「三之三」 |
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

⚠️ **⑦ 總綱 v1（2026-09-29，✅ 2026-09-30 確認）：canonical diff 改用 `--full-index`**。上面三條的 `git diff --binary` 一律改成
`git diff --binary --full-index --no-ext-diff --no-textconv --no-color`（清掉 `GIT_*`），三個 SHA 一律 **tree-to-tree** 計算。
⚠️ **第一輪 review 補完**：上一行仍不夠——git config 與屬性（`diff.noprefix`、`diff.context`、`diff.renames`、`diff.orderFile`、`core.quotePath`、
`diff=<driver>`／`-diff`）都會改變 bytes（2026-09-29 實測）。**唯一定義見「⑦ 總綱 v1」的「二」**：釘死全部影響 bytes 的參數、
在只借物件的暫存 bare repo 計算、隔離環境，並有惡意 config 測試。
⛔ 理由：`--binary` 對文字檔的 `index` 行只印**縮寫**的 blob OID，長度隨 repo 的物件數自動決定（目前 7 碼，前綴有歧義時也會變長）——
同一份 patch 在真正 repo（產生器）與複本（runner、finalizer）算出的 SHA 可能不同。⚠️ 影響範圍只有**非空** diff：Stage 1 與
`envcheck/` 的 tooling 都是空的，已封存的證據不受影響；`counterfactual_e1cbbbd.patch` 由 ⑦a 換成 `--full-index` 格式（內容不變、
SHA 會變，② 實測的 `ef7a4cdf…` 從此是歷史值）。runner、`compose_check`、`--check-failed-record`、tooling patch 產生器共用**同一支**
合成函式（`scripts/lib/replay-args.sh`）；`test-replay-args.sh` 原本把 `--full-index` 視為非 canonical 的斷言同步改掉。

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
⚠️ **⑦ 總綱 v1（✅ 2026-09-30 確認）**：空 tooling 仍是 runner 層的合法輸入（一般路徑、測試），⚠️ **但 ⑩ 的 tooling ⛔ 不得為空**——`evaluation.py` 與
`replay_bundle/` 的反事實路徑要經它進入 replay（見 Stage 2 計畫書「八之一」的訂正與「⑦ 總綱 v1」）。

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

##### 五之零、`verify_stage2_graph()` 的十四道（v2 新增，v3 擴充；⚠️ v10 再加兩道，共十六道）

⚠️ 比照 Stage 1 的 `verify_evidence_graph()`——⛔ **只驗「檔案集合與 SHA」擋不住
「各檔都合法、彼此卻對不起來」**：

| # | 檢查 |
|---|---|
| 1 | 四份 canonical artifact 各自的**完整 validator**（「三之二」） |
| 2 | **Stage 1 信任錨十道**（「三之一」） |
| 3 | comparison 的每列 `before`／`after` **等於來源 row**、`differences` **重算相符** |
| 4 | comparison 的 keys **恰好等於** Stage 1 D+1 cohort 的 156 keys |
| 5 | report 的 `comparison_artifact_sha256`、截斷與統計**重算相符** |
| 6 | **所有檔案同一個 `bundle_id`**，且等於 identity、Stage 1 manifest 與 ⚠️ **`envcheck/` manifest（v10）**的 |
| 7 | ⚠️ **逐一列舉**每份 provenance 的位置與 role，`image_digest` 必須等於 `expected_image_id`——⛔ 不用 `.get("provenance") or …` 通用推測（Stage 1 的教訓：那樣會整份漏掉 `comparator_provenance`）。⚠️ **mapping 見下表** |
| 8 | before source 與 comparison 的 **`before_ref` 相等**，且**等於兩者的 `provenance.base_commit`** |
| 9 | before source 的 **`timeframe`／`replay_scope`** 等於已錨定的 Stage 1 D+1 after |
| 10 | ⚠️ before source 與 comparison **來自同一次 runner invocation** → 兩份 `provenance` **逐欄相等**（⛔ 不只比 `image_digest`；⚠️ 比照 Stage 1 對 probe 三份的同一道） |
| 11 | report 的 **`before_ref`／`bundle_id`** 等於 comparison |
| 12 | ⚠️ **`rows_shown` 恰好等於 `min(200, candidate_rows)`**——⛔ **只寫「取前 `rows_shown` 列」會讓 `rows_shown = 0` 的空報告合法通過**。⚠️ **v10 註**：200 是 bundle manifest 的 `report_max_rows`（實查＝200，`build_report()` 就是用它截斷）；Stage 2 archive 不含 bundle manifest，所以 validator 寫死常數 200，⚠️ 並由測試斷言它等於該 bundle 的值 |
| 13 | comparison 的 keys **已排序且唯一** |
| 14 | ⚠️ **Stage 2 專屬**：before source 的 `candidate_keys(rows)` **必須為空** |
| ⚠️ 15 | ⚠️ **v10**：before source ⛔ **沒有任何一列** `lifecycle_phase == "CONTINUATION"` 且 `setup_rr_qualified == false`，且每列 `rr_decoupling_candidate` **等於**等價式（Stage 2 計畫書「①之三」） |
| ⚠️ 16 | ⚠️ **v10**：**環境見證錨**（「三之三」的 E1～E7，⚠️ v11：**含 E3a ＋ E3b**——只接受 EQUIVALENT）成立，且 before source、comparison 的 `provenance.image_digest` ＝ manifest 的 `expected_image_id` ＝ envcheck identity 的 `expected_image_id`；Stage 2 archive 的 `identity/run_identity.json.gz` **逐位元等於** envcheck 的那一份 |

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

##### 五之一、記憶體模型（v10 新增，✅ 已隨 v28／③ v12 確認（2026-09-23））

⛔ **v9 沒有記憶體模型**：`load_stage1_anchor()` 回傳整份 `after_payload`（整份載入實測約
+365～449 MiB），finalizer 還要整份讀 before source（同量級），v10 又多了 after'——
⛔ 照 Stage 1 的整份載入寫法，**必然**撞上 crossday comparator 的同一道牆
（約 730 MiB，見 [I-117](#i-117stage-1-的-comparatorfinalizerrecovery-整份載入兩份-after-artifact超過-mem-guard)），
而 mem-guard 常態只給 531m；⛔ Stage 2 計畫書禁止停 live container 換記憶體。

| 規則 | 內容 |
|---|---|
| 讀取方式 | ⚠️ **全量 artifact（Stage 1 D+1 after、after'、before source）一律串流**；⚠️ 每份**只讀一次**，同一趟內同時完成**增量 SHA**、schema／diagnostics 驗證與下列摘要——⛔ 不得「先算 hash、再讀一次驗證」（TOCTOU，見「七之二」） |
| 常駐內容 | ⚠️ 只有：頂層欄位、**全量 keys**、**每列 canonical bytes 的 digest**、**156 筆 cohort rows 的完整 row**（comparison 的逐鍵相等要用） |
| 跨檔比對 | 全量 keys 相等、before／after' 與 D+1 的逐列相等，一律用**每列 digest** 比；⛔ 不得同時持有兩份全量 rows |
| `load_stage1_anchor()` 的回傳 | ⚠️ **改成上面的摘要**（`after_keys`、`after_row_digests`、`cohort_rows`、`after_base_commit`…），⛔ **不回傳整份 `after_payload`**；⚠️ `manifest_payload`、`cohort_payload`、`identity_payload` 都是 KB 級，照舊保留 |
| 串流讀取的實作 | ⚠️ **與 Stage 2 計畫書「二、③」的一趟串流 loader、環境等價比對共用同一套**，⛔ 不各寫一份 |
| 驗收 | 用 Stage 2 計畫書「六、1」的 harness 實測，**每一個程序**（preflight、finalizer、recovery、環境等價比對）各自 **< 450 MiB**；⛔ 不得用停 container 換記憶體的方式通過 |

⚠️ **驗證強度⛔ 不得下降**：`validate_after_artifact()`、`validate_replay_errors()`、
`validate_diagnostics()` 的每一項都要在逐列餵入下照驗（承 Stage 2 計畫書「二、③」的不變條件 a）。
⚠️ **v11 的做法**：抽出 **row-level 共用原語**，整批 validator 與串流 validator 都呼叫同一套
——⛔ 不另寫一份「串流版」規則（兩套實作＝兩套語意）。⚠️ **既有 validator 的公開 contract 與 Stage 1 行為不變，
允許內部重構**；Stage 0／Stage 1 既有測試⛔ 不修改斷言、全數續跑。

##### 五、發布流程與 commit point

⚠️ **單一 evidence archive、一次性原子發布**（承 Stage 2 計畫書 v11 裁定），
⛔ 流程與 Stage 1 的 `finalize_evidence()` **同形**，只是 layout 不同：

```text id="i074_stage2_finalize_001"
階段 A：讀入 6 個 payload 來源 ＋ 全部驗證（所有 project import 在此發生）
        ＋ 依信任錨重讀 Stage 1 的 after／cohort／identity 三份，執行完整十道
        ＋ ⚠️ v10：依環境見證錨重讀 envcheck，執行 E1～E7（⚠️ v11：含 E3a ＋ E3b）
        ⚠️ v10：以上**一律串流**（見「五之一」），⛔ 不得整份載入任何全量 artifact
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
| ⚠️ **環境見證錨**（v10） | ⚠️ **重做「三之三」的 E1～E7**（⚠️ v11：含 E3a 的全量串流重比 ＋ E3b）——⛔ 不符即中止 |
| ⚠️ **完整 graph** | ⛔ **⛔ 不得只驗檔案與 hash**：載入四份 canonical artifact 後**完整執行「五之零」的各道**（⚠️ v10 起共十六道，含各自的 validator）——⚠️ **在 fsync 之前**。⛔ 少了這道，recovery 只能證明「東西沒變」，⛔ 證明不了「它原本就是對的」 |
| ⚠️ **合成守門**（v10） | ⚠️ **由 shell 入口在隔離 worktree 重做「四之二」的合成重算**——與 failed record 的 recovery 對稱（v9 只對 failed record 寫了這一條） |
| 執行身分 | 比對 `RECOVERY_IDENTITY_FIELDS`，⚠️ **在 fsync 之前**比 |
| 成功 | ⚠️ **回 manifest 記的 `terminal_outcome`**（v3 恆為 0）——⛔ **不得寫死** |
| 再失敗 | ⛔ **不刪除既有 archive**；回 rc=1 並要求人工介入 |

##### 七、failed-attempt record（⚠️ **⛔ 不是成功 archive**）

⚠️ **任一反事實生效條件不成立時**（⚠️ v12 訂正：Stage 2 計畫書「①之三」的檢查順序得出
`candidate_flag_inconsistent` 或 `rr_not_restored`；⛔ 不只 `before_candidates != ∅`），
**正式 archive ⛔ 不存在**，但事故紀錄仍必須落地。

| 項目 | 契約 |
|---|---|
| 位置 | ⚠️ **與成功 archive ⛔ 不同的根目錄**：`python/baselines/i074_stage2/failed/<bundle_id>-<counterfactual_patch_sha256>/`（⚠️ **完整 64 碼，⛔ 不截斷**——⛔ 截成 12 碼會讓不同的完整 SHA 映到同一路徑）。⚠️ **⑦ 總綱 v1 第一輪 review（✅ 2026-09-30 確認）**：目錄名改成 `<bundle_id>-<counterfactual_semantic_sha256>`（同樣完整 64 碼）；record 另加 `counterfactual_semantic_sha256` 一欄（封閉 schema 同步，見下方 schema 列），`patches` 四欄與兩份完整 patch 照舊。⛔ **v1 寫的 `<run_id>` 根本不存在**——`run_identity.py:32` 的 `RUN_IDENTITY_FIELDS` 只有 `schema_version`／`kind`／`bundle_id`／`expected_image_id`／`created_at` 五欄 |
| 檔案 | `failure_record.json`（⛔ 不壓縮）＋ `patch/counterfactual.patch` ＋ `patch/tooling.patch` |
| schema | ⚠️ **封閉欄位**：`schema_version`／`kind="sr_zone_stage2_failed_attempt"`／`bundle_id`／`expected_image_id`／`run_identity`／`patches`（同「四之一」四欄）／`files`（兩份 patch 的 `raw_blob` entry）／`bounded_diagnostics`／`failure_reason`／`generated_at`／`provenance`（⚠️ **⑦ 總綱 v1 第二輪 review（✅ 2026-09-30 確認）**：再加 **`counterfactual_semantic_sha256`**（hex64，⛔ 不得為空 diff 的 SHA），必須 ＝ 由封存的 counterfactual patch 重算的語意 SHA，並 ＝ 目錄名的後段）。⛔ 多欄、缺欄、型別不符一律中止；⚠️ **canonical JSON**（⛔ 不壓縮——它要能直接讀） |
| ⚠️ **檔案集合** | ⛔ **恰好** `failure_record.json` ＋ `patch/counterfactual.patch` ＋ `patch/tooling.patch`；多一個未知檔案即中止 |
| ⚠️ **patch 重新比對** | lookup 與 recovery 都要**重讀兩份 patch 的實際 bytes、重算 SHA**，與 `files`／`patches` 比對——⛔ 不只信 record 裡的宣告值 |
| `bounded_diagnostics` | ⚠️ **依 `failure_reason` 分流的封閉 union**——欄位集合、型別與 sample 規則以下方 **F5～F7、F10** 為準（⚠️ v12：v10 的單一 schema 已從本列移除，改版經過見 v11 修訂表第 2 列）。⚠️ **`sample_keys` 的規則寫死**：依 `(symbol, timeframe, as_of)` **排序後取前 `min(N, count)` 個**、⛔ **不得重複**。⚠️ **N ＝ 20**（✅ review 同意，2026-09-23）：**寫死的共用常數**，⛔ 不開 CLI 參數；⚠️ **完整計數照樣保留**，N 只限制 sample 的數量；failed record 與環境等價判定**共用這個上限**，⚠️ 但 sample 的欄位集合**各自封閉**；⛔ **N 不得套用到 B／C 的正式證據**——156 列 cohort 全部保存、全部判讀。⛔ **不存全差集** |
| 發布 | ⚠️ 同樣**原子 ＋ fsync**（共用 `publish.py` 原語） |
| ⛔ 與成功 archive 的關係 | ⛔ **不得**放進成功 archive 的 layout，⛔ 不得沿用其 manifest kind |

**failed record 的不變條件（⚠️ **產品契約**，⛔ 不是測試清單）**：

| # | 不變條件 |
|---|---|
| F1 | `run_identity` **必須通過既有的 `validate_run_identity()`** |
| F2 | 頂層 `bundle_id`／`expected_image_id` **等於 embedded identity 的同名欄位** |
| ⚠️ **F2-a** | ⚠️ **embedded `run_identity` 必須完整等於「唯一固定的 Stage 2 identity」**（⚠️ v10 改寫：即環境見證錨封存、信任錨第 10 道比對的同一份；⛔ v9 的「固定 Stage 1 manifest 所錨定的 identity」已不可能成立）——⛔ **只驗 schema 與頂層欄位是不夠的**：⚠️ 一份**只有 `created_at` 不同**、其餘完全合法的 record 照樣會通過 F1～F10，**查找鍵／目錄鍵的矛盾就又回來了**。⚠️ **發布、lookup、recovery 三處都要執行這一項** |
| F3 | `provenance` 用 **`role="stage1"`**；`image_digest` 等於 `expected_image_id`、`base_commit` 等於兩份 patch 所依附的 base、**`tooling_patch_sha256` 等於 `patches.composed_sha256`** |
| F4 | ⚠️ **目錄名 `<bundle_id>-<counterfactual_patch_sha256>`（完整 64 碼）必須與 record 內容相符**——⛔ 否則改個目錄名就能繞過 lookup。⚠️ identity 已由信任錨第 10 道與 **F2-a** 釘死，**目錄鍵與查找鍵因此一致**。⚠️ **⑦ 總綱 v1 第一輪 review（✅ 2026-09-30 確認）**：目錄名改用 `counterfactual_semantic_sha256`，而且它必須 ＝ 由封存的 counterfactual patch **重算**的語意 SHA（F8-a 的合成守門一併重算；⛔ 不只信宣告值）；lookup 與 recovery 都以重算值比對目錄名與欄位。⚠️ **第三輪 review：F4 分兩層**——Python 層驗 hex64、目錄名 ＝ 宣告值、宣告值 ＝ 交接值（`--verified-counterfactual-semantic-sha256`）；shell 層從實際 patch 重算、驗四檔集合、重算值 ＝ 宣告值（Python ⛔ 不碰 git），見「⑦ 總綱 v1」「二」的交接列 |
| F5 | `failure_reason` 是**封閉列舉**——⚠️ **v11：恰好兩個值** `candidate_flag_inconsistent`、`rr_not_restored`（⛔ v10 的唯一值 `before_candidates_nonempty` 已併入後者）；⛔ **不接受任意字串**。⚠️ 兩者的優先序由 Stage 2 計畫書「①之三」的**檢查順序**決定，⛔ record 不另存「同時違反的其他原因」 |
| F6 | ⚠️ **v11：`bounded_diagnostics` 是依 `failure_reason` 分流的封閉 union**，欄位集合**恰好是**：<br>`candidate_flag_inconsistent` → `{inconsistent_row_count, sample_keys}`<br>`rr_not_restored` → `{before_candidate_count, sample_keys}`<br>⛔ 多欄、缺欄、欄位與原因不相配即中止 |
| F6-a | 計數欄位（依原因二選一）是**正整數**；`sample_keys` 長度**恰好** `min(N, count)`、依 `(symbol, timeframe, as_of)` **已排序**、⛔ 不重複 |
| F6-b | ⚠️ **`sample_keys[]` 每一項的欄位集合恰好是** `{symbol, timeframe, as_of, lifecycle_phase, setup_rr_qualified, rr_decoupling_candidate}`（⚠️ v11 加上最後一欄，兩種原因共用）——⛔ 缺欄或多欄都中止（⚠️ F6-a 只限制長度與排序，⛔ 擋不住欄位缺漏） |
| F6-c | `symbol`／`timeframe`／`as_of`／`lifecycle_phase` 必須是**非空字串** |
| F7 | `sample_keys[]` 每項的 `setup_rr_qualified` 與 `rr_decoupling_candidate` 必須是**嚴格 boolean**——⛔ 不接受 `0`／`1` |
| ⚠️ F10 | ⚠️ **v11：sample 必須真的違反它宣稱的那一條**——⛔ 否則 record 可以宣稱一種原因、卻放著毫不相干的列：<br>`candidate_flag_inconsistent` → 每一項 `rr_decoupling_candidate != (lifecycle_phase == "CONTINUATION" and not setup_rr_qualified)`<br>`rr_not_restored` → 每一項 `lifecycle_phase == "CONTINUATION"`、`setup_rr_qualified == false`、`rr_decoupling_candidate == true` |
| F8 | 兩份 patch 的**實際 bytes** 重算 SHA 後與 `files`／`patches` 相符 |
| F8-a | ⚠️ **合成守門同樣適用**：發布 failed record **之前**，shell 端也要在隔離 worktree 依 `ordered_components` 套用兩份 patch 並重算 `composed_sha256`（見「四之二」）——⛔ **不符即⛔ 不發布**。⚠️ **⛔ 少了這道**，failed record 可能封存「各自 SHA 都對、卻合成不出宣告 composed hash」的一組 patch |
| F9 | 檔案集合⛔ **恰好** `failure_record.json` ＋ `patch/counterfactual.patch` ＋ `patch/tooling.patch` |

**重跑守門（⚠️ 必須在 replay 之前）**：

```text id="i074_stage2_rerun_gate_001"
preflight（⛔ replay 之前）：
  掃 <stage2 root>/failed/，找「同 run identity ＋ 同 counterfactual_patch_sha256」
  ⚠️ ⑦ 總綱 v1 第一輪 review（✅ 2026-09-30 確認）：比對改用 counterfactual_semantic_sha256（兩個產品檔白名單的 canonical diff）
  ⚠️ 「同 run identity」＝ **完整 identity object 逐欄相等**，⛔ 不只比 bundle_id
  命中 → ⛔ **立即中止**（⛔ 不得跑完三小時才拒絕）
  未命中 → 繼續
```

⚠️ **lookup 與 recovery 都要完整執行 F1～F10**（⛔ 不是籠統的「schema ＋ patch SHA」）——
⚠️ 包含 **F2-a** 的 identity 綁定、F4 的目錄名相符與 F8-a 的合成守門。

⛔ **但 F8-a 需要 git，而 `stage2_evidence.py` ⛔ 不碰 git**（見「四之二」）——
⚠️ **所以 lookup 必須有對應的 shell 入口**（⛔ v3 只定義了 `--recover-failed-record`）：

```text id="i074_stage2_lookup_entry_001"
finalize-stage2-evidence.sh --check-failed-record
  ① Python：F1～F10 中⛔ 不需要 git 的各項（含 F2-a、F4）
  ② shell ：F8-a——隔離 worktree 套兩份 patch，重算 composed_sha256
  ⚠️ 兩段都過才算「這份 record 有效」；任一段失敗 → ⛔ fail-closed
```

###### 七之一、`--check-failed-record` 的 CLI 契約（v4 新增）

| 項目 | 契約 |
|---|---|
| 用法 | `finalize-stage2-evidence.sh --check-failed-record --counterfactual-patch <path>` |
| failed root | ⚠️ **寫死常數** `python/baselines/i074_stage2/failed/`——⛔ **CLI 無覆寫參數**（⚠️ 比照 `run_identity.py` 的既有慣例：測試改為直接呼叫 Python API，⛔ 不開「只給測試用」的路徑參數） |
| ⛔ **本次的比對鍵** | ⚠️ **必須與 runner 用同一套推導**（⛔ v4 初稿「直接雜湊檔案 bytes」**是錯的**）：<br>① base 取自**已驗證的 Stage 1 after `provenance.base_commit`**；<br>② 在**隔離 worktree** 套用 `--counterfactual-patch`，`git write-tree` 得 `T1`；<br>③ 比對鍵 ＝ `sha256(git diff --binary <base> <T1>)`（⚠️ **⑦ 總綱 v1（✅ 2026-09-30 確認）**：改用「⑦ 總綱 v1」「二」的 canonical diff（`--full-index` ＋ 全部釘死的參數、在隔離的暫存 bare repo 計算；⚠️ 第一輪 review：只加 `--full-index` 不夠），與 runner 呼叫同一支共用合成函式；⚠️ 第一輪 review：比對鍵改成產品檔白名單的**語意 SHA**——見「⑦ 總綱 v1」的「二」；⚠️ 第三輪 review：本次輸入與每份 record 的語意 SHA 都由 **shell** 從實際 patch 重算並驗四檔集合，Python 段只驗 hex64 與「目錄名 ＝ 宣告值」）。<br>⚠️ **為什麼不能直接雜湊檔案**：⛔ 同一份變更可以用**不同的 patch 排序、header 或文字表示**套出相同的 `T1`——那時檔案 hash 不同但 canonical diff SHA 相同，⚠️ **checker 會誤判成「沒命中」而放行**，等於繞過「同一份壞 patch ⛔ 不得重跑」 |
| ⚠️ **非 canonical 輸入的裁決** | ⚠️ 正式 archive **無條件要求** `stored patch SHA == sha256(git diff --binary <base> <T1>)`，所以⛔ **不能只標示不拒絕**（⚠️ 那種輸入會跑完**數小時 replay** 才在 finalizer 被拒）。**順序固定為**：<br>① 先用**重建出的 canonical SHA** 查找 failed records；**命中 → rc=2**；<br>② **未命中**但輸入 bytes **≠** canonical diff bytes → ⚠️ **rc=1，在 replay 之前拒絕**。<br>⛔ **⛔ 不採「用重建 bytes 取代輸入」**——⚠️ 那需要另外補完整的替換、provenance 與封存流程，⛔ 不在本計畫範圍 |
| 輸入形式 | ⚠️ **只吃 patch 路徑，⛔ 不吃 SHA**——⛔ 從介面層杜絕 spoof |
| Stage 1 identity 來源 | ⛔ **不得直接讀 `identity/run_identity.json.gz`**——⚠️ 必須先取得下方的**信任錨 snapshot**。⚠️ **v10**：命中判定用的「同 identity」是**唯一固定的 Stage 2 identity**，取自**已驗證的環境見證錨**（「三之三」），⛔ 同樣不得直接讀 identity 檔 |
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
      ⚠️ v10：after_keys / after_row_digests / after_candidate_keys / cohort_rows（156 筆完整 row）
             ← ⛔ 不再回傳整份 after_payload（見「五之一」）
      cohort_payload / identity_payload}

validate_stage1_anchor_graph(snapshot, *, stage2_identity, envcheck_identity,
                             expected_bundle_id) ：   ← 跨檔（第 6、8、9、10 道）
  ⑥ snapshot.bundle_id == Stage 2 的                                        （道 6）
     ⚠️ v10：⛔ 不再比 image——Stage 2 的 image 由「三之三」的 E5 串接
  ⑦ cohort.after_artifact_sha256 == snapshot.members[after].artifact_sha256（道 8）
  ⑧ cohort keys == snapshot.after_candidate_keys（串流時收集）              （道 9）
  ⑨ ⚠️ v10：stage2_identity == envcheck_identity（逐位元），
     且 stage2_identity.bundle_id == snapshot.bundle_id                     （道 10）
     ⛔ v4～v9 的「== snapshot.identity（Stage 1 archived identity）」已不可能成立
  ⑩ ⚠️ **bundle／image 的兩條等式鏈**（道 6-a、6-b）——⛔ 單檔 validator 只驗型別與
     role，⛔ 擋不住「各檔都合法、bundle／image 卻各自不同」：
       manifest.bundle_id == identity.bundle_id == after.bundle_id
                          == cohort.bundle_id  == Stage 2 bundle_id
       manifest.expected_image_id == identity.expected_image_id
                                  == after.provenance.image_digest
                                  == cohort.provenance.image_digest
       ⚠️ v10：image 鏈**只在 Stage 1 內部成立**，⛔ 不再延伸到 Stage 2 expected_image_id
```

⚠️ **三個呼叫端（preflight／finalizer／recovery）⛔ 都必須呼叫兩支**——
⚠️ **第 8、9 道⛔ 不得只留到 finalizer**：⛔ 那樣 cohort 壞掉要跑完數小時 replay 才會被發現。

⚠️ **hash、validator 與回傳值⛔ 必須全部來自同一次檔案讀取**——⛔ 不得為了省記憶體
之後再讀一次（⚠️ 兩次讀取之間檔案可能被換掉，那是 TOCTOU）。

| 使用者 | 用途 |
|---|---|
| **preflight** | ⚠️ **兩支都呼叫**（第 1～10 道）、⚠️ **v10：再做環境見證錨的 E1～E7**（⚠️ v11：含 E3a ＋ E3b）、**提供 `after_base_commit` 給 checker**、執行 failed-record lookup |
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
| ⚠️ `TOOLING_PATCH`（空／未提供） | ⚠️ **直接在私有目錄建立 0-byte 凍結副本**——⛔ 不是「跳過」。⚠️ 後續 SHA、套用與封存**一律用該 0-byte 副本**，於是 `tooling_patch_sha256` 自然是空字串的 SHA（② 實測的 `e3b0c442…b855`）。⚠️ **⑦ 總綱 v1（✅ 2026-09-30 確認）**：這一列限 **runner 層**；⑩ 的 orchestrator 從複本的常數路徑凍結 tooling patch，⛔ **不得為空**、且必須等於複本內封存的那一份（ba 拆兩層） |
| 下游來源 | ⚠️ checker、`T1` 推導、實際套用的 worktree、最終封存 **一律只用凍結副本**——⛔ **任何一處都⛔ 不得再讀原始路徑** |
| 私有目錄 | ⚠️ **v10 改寫**：放在 **orchestrator 的 run 目錄底下**，⚠️ **保留到 finalize rc=0 或 failed-attempt record 發布完成才清**——finalize 是另一個步驟，要從這裡取得要封存的 patch bytes；finalize 失敗重試也沿用同一份。⛔ 它本身⛔ 不是證據，⛔ 不進 archive（archive 裡的是它的逐位元副本）。⛔ v7～v9 寫的「用完清除」會讓 finalizer 拿不到凍結副本，只能回頭讀原始路徑，⛔ 違反上一列 |
| 等價做法 | ⚠️ **或**讓 checker 建出的 worktree／`T1` **直接延續給 replay 使用**——⚠️ 兩者擇一，⛔ 不得都不做 |

⚠️ **preflight 的呼叫順序（⛔ 全部在 replay 之前）**：

| 序 | 動作 |
|---|---|
| ⚠️ 0 | **複本自身完整性（Stage 2 計畫書 v29 補；第六輪移到最前面）**：⛔ **只用 git 與 shell、⛔ 不 import 任何 repo 內的 Python 模組**——位於複本內、⛔ 沒有 alternates、複本 `HEAD` ＝ freeze record 副本的 `repo_head`、已追蹤檔⛔ 沒有修改、`python/baselines/i074_stage2/` 以外⛔ 沒有未追蹤檔、orchestrator 自身的內容 ＝ `repo_head` 中的版本——⛔ 不符即中止（rc=1、⛔ 不計入正式 scan），⚠️ **之後各列一律⛔ 不執行**；規格見 Stage 2 計畫書「八之一」 |
| 1 | ⚠️ **凍結兩份 patch**（「七之三」）——⛔ 之後一律只用凍結副本 |
| ⚠️ 2 | **freeze record 的完整驗證（Stage 2 計畫書 v29 補）**：「八之二」的封閉 schema、交叉不變條件、腳本與 counterfactual 的 SHA、image／identity——此時複本已驗過，才開始用 repo 內的模組；⛔ 不符即中止（rc=1、⛔ 不計入正式 scan） |
| 3 | `load_stage1_anchor()` ＋ `validate_stage1_anchor_graph()`：⚠️ **第 1～10 道全部**（含第 8、9 道） |
| 4 | `--check-failed-record` 掃 `failed/`，逐份驗證；⚠️ **v29**：另驗真正 repo 的 `python/baselines/i074_stage2/` 底下⛔ 沒有未追蹤項目，且真正 repo HEAD 的 `failed/` 全部出現在複本裡（Stage 2 計畫書「八之一」） |
| 5 | 以「同 identity ＋ 同**完整** counterfactual SHA」判定命中 → ⚠️ **rc=2 即中止**；⚠️ **rc=1（record 損壞）同樣中止**。⚠️ **⑦ 總綱 v1 第一輪 review（✅ 2026-09-30 確認）**：「完整 SHA」改成**語意 SHA**（⑦ 總綱 v1 決策表第 9 列） |
| ⚠️ 6 | **磁碟檢查（Stage 2 計畫書 v29 補）**：`f_bavail × f_frsize` ≥ `P_B_BUDGET ＋ M_safety`（1,241,513,984 bytes），且相關位置與 Docker Root Dir 同一個裝置——⛔ 不符即中止（rc=1、⛔ 不計入正式 scan）；規格見 Stage 2 計畫書 v29「磁碟檢查」 |
| 7 | 都通過 → 才進 replay |

⚠️ **lookup 遇到損壞或缺檔的 record**：⛔ **fail-closed**——中止並要求人工處理。
⛔ **不得「忽略後繼續」**：那等於讓一次被記錄過的壞 patch 靠「弄壞自己的 record」重新過關。

⚠️ **record 自身發布失敗怎麼算**（⛔ 不能留成未定義）：

| 情況 | 結束碼 | 重跑資格 |
|---|---|---|
| record rename 之前失敗 | **1** | ⚠️ **允許以同一語意 SHA 重跑**（⚠️ **⑦ 總綱 v1 第四輪 review（✅ 2026-09-30 確認）**：原本寫「同一 SHA」）——⛔ 沒有留下任何紀錄，就不存在「已知壞 patch」；⚠️ 但報告必須標明**事故紀錄遺失** |
| record rename 成功、parent fsync 失敗 | **3** | ⛔ **視同已記錄**（lookup 讀得到）→ 同**語意 SHA** ⛔ 不得重跑 |
| record 完整發布 | **1**（⚠️ 執行仍是失敗） | ⛔ 同**語意 SHA** ⛔ 不得重跑；⚠️ 改了白名單產品檔、語意 SHA 不同才可（⚠️ **⑦ 總綱 v1 第四輪 review（✅ 2026-09-30 確認）**：原本寫「改 patch 後才可」——只改測試檔⛔ 不算） |

⚠️ **rc=3 必須有出口**（⛔ v1 漏了，會留下永久未確認狀態）：
`finalize-stage2-evidence.sh --recover-failed-record <record 目錄>`——
**完整重驗 F1～F10（含合成守門），再 `_fsync_tree` ＋ fsync parent**，
⛔ **不重建、⛔ 不重跑 replay**。⚠️ **成功後回 `1`**（⛔ 不是 0——那一次執行本來就是失敗的），
⚠️ 並且**重跑資格仍是「⛔ 同語意 SHA 不得重跑」**（⚠️ **⑦ 總綱 v1 第四輪 review（✅ 2026-09-30 確認）**：原本寫「同 SHA」）。

⚠️ **計次**：patch 失效的中止⛔ **不計入正式 scan**（承 Stage 2 計畫書「二、①」，
比照「preflight 與指紋檢查失敗⛔ 不計入」），⛔ **但⛔ 不得靜默重跑**——
⚠️ 上面的守門就是「⛔ 不靜默」的實作。

###### 七之四、`finalize-stage2-evidence.sh` 的 CLI matrix（v10 新增，✅ 已隨 v28／③ v12 確認（2026-09-23））

⛔ **v9 只定義了 `--check-failed-record` 與 `--recover-failed-record`**，成功 archive 的正常發布、
它的 durability recovery、failed record 的發布三個入口都沒有——⚠️ 沒有 matrix 就定不出 argv fixture，
而 Stage 1 的教訓是 **fixture 要由計畫定義**，⛔ 不能由實作者自選 argv 再凍結。

⚠️ **共通規則**：模式旗標**互斥**、重複即中止；⚠️ 所有路徑一律**寫死常數或由 orchestrator 的 run 目錄
固定推導**，⛔ 使用者不得逐檔以 `--source` 覆寫（Stage 1 第二輪 review 高 1）；Python 段在
**Stage 2 identity 的 image** 內執行，`finalizer_provenance.image_digest` 必須等於它；
合成守門在 host 的隔離 worktree 執行、⛔ 不建中繼 commit。

| 模式 | 用法 | 輸入（⚠️ 皆由 run 目錄固定推導） | 做什麼 | 結束碼 | 屬於 |
|---|---|---|---|---|---|
| **envcheck** | `--envcheck --run-dir <閘門的 run 目錄>` | `witness/after_artifact.json`、`witness/cohort_manifest.json`；Stage 2 identity | 串流比對 after' 與 D+1 → 產出 equivalence → 驗 E1、E2、**E3a**、E4～E7（⚠️ v11：⛔ 不做 E3b）→ 原子發布 `envcheck/` | **0**＝EQUIVALENT；**7**＝NOT_EQUIVALENT（⚠️ **證據照樣發布**）；1；3 | ③b |
| **recover-envcheck** | `--recover-envcheck` | 既有 `envcheck/`（寫死常數） | 完整重驗 E1、E2、**E3a**（含全量重比）、E4～E7 → fsync；⛔ 不重跑 replay | ⚠️ v11：「outcome 的唯一來源」三者一致時回 manifest 的 `terminal_outcome`（0／7）；否則 1 | ③b |
| **normal** | `--finalize --run-dir <Stage 2 的 run 目錄>` | `stage2/before_source_artifact.json`、`stage2/comparison_artifact.json`、`stage2/report.json`、`patches/counterfactual.patch`、`patches/tooling.patch`（凍結副本） | 合成守門 → 信任錨 ＋ 環境見證錨 ＋ 全圖 → 發布成功 archive | 0／1／3（「五」） | ③d |
| **recover-durability** | `--recover-durability` | 既有成功 archive（寫死常數） | 「六」——含合成守門重做 | manifest 的 `terminal_outcome`／1 | ③d |
| **publish-failed-record** | `--publish-failed-record --run-dir <Stage 2 的 run 目錄>` | `stage2/bounded_diagnostics.json`（⚠️ replay 在 rc=6 時寫出的中繼檔：`failure_reason` ＋ 依 F6 分流的欄位；⚠️ v11：finalizer 以 F5～F7、F10 驗它）＋ 凍結 patch | F8-a 合成守門 → F1～F10 → 發布 failed record（⚠️ **⑦ 總綱 v1 第三輪 review（✅ 2026-09-30 確認）**：shell 另外重算語意 SHA 並以 `--verified-counterfactual-semantic-sha256` 注入，Python 在 rename 之前比對 record 欄位） | ⚠️ **1**（完整發布——那次執行本來就是失敗）／3（rename 成功、fsync 失敗）；見「七之三」的重跑資格表 | ③d |
| **check-failed-record** | 見「七之一」 | 同左 | 同左（⚠️ **⑦ 總綱 v1 第三輪 review（✅ 2026-09-30 確認）**：Python 段只驗語意 SHA 的 hex64 與目錄名，shell 段重算並以語意 SHA 判定命中；⛔ 不接受 `--verified-counterfactual-semantic-sha256`） | 0／2／1 | ③d |
| **recover-failed-record** | 見「七之三」 | 同左 | 同左（⚠️ **⑦ 總綱 v1 第三輪 review（✅ 2026-09-30 確認）**：shell 從 record 的實際 patch 重算語意 SHA、比對宣告值後注入，Python 在 fsync 之前比對 record 欄位） | 1 | ③d |
| **verify-promotion-staging**（⚠️ **⑦c 細部計畫 v1（✅ 2026-10-01 確認）**） | `--verify-promotion-staging <path> --target <evidence\|failed/<name>> [--judge]` | ⚠️ ⛔ 不由 run 目錄推導：真正 repo（＝ 本 repo 的 `origin`）的 `i074_stage2/.promote-staging-<16 hex>`（直屬）或目的地本身；錨點取自本 repo | 合成守門（failed 另驗語意 SHA ＝ target 名稱）→ evidence 的全圖 ＋ 信任根的綁定／failed record 的 F1～F10（F4 以目的地名稱驗）；⚠️ **唯讀**（全部 `:ro`、⛔ 不 fsync、⛔ 不寫入）；`--judge`（只限 evidence）在同一個程序判讀 | **0** 有效（一行 canonical JSON）／**1** | ⑦c |

⚠️ **argv fixture**（⚠️ **⑦c 細部計畫 v1（✅ 2026-10-01 確認）**：再加驗證模式的三組——evidence、failed、evidence ＋ `--judge`）：新增 `python/scripts/fixtures/stage2_finalizer_argv.json`，**七種模式各一組**
token 序列（⛔ 非 `eval` 字串；動態值用 placeholder），由 `scripts/test-replay-args.sh` 逐 token 比對，
並驗 mount（⚠️ 用 here-string，⛔ 不用 `| grep -q`——`pipefail` 下的 SIGPIPE 競爭）。

##### 八、受影響檔案

| 檔案 | 改動 |
|---|---|
| **`replay_bundle/stage2_evidence.py`** ⬅️ **新增** | ⚠️ **`load_stage1_anchor()` ＋ `validate_stage1_anchor_graph()`（共用信任錨）**、layout、manifest builder／validator、`verify_stage2_graph()`、`finalize_stage2_evidence()`、`recover_stage2_durability()`、failed-attempt record 的 builder／publisher／lookup。⚠️ **v10**：再加**可共用的封閉 layout archive 核心**（`envcheck/` 與成功 archive 共用，③b）、**環境見證錨**、「五之一」的串流版信任錨 |
| ⚠️ **串流讀取**（v10，③b；建議 `replay_bundle/stream.py`） | 一次讀取同時完成增量 SHA、逐列驗證與摘要；⚠️ Stage 2 的一趟串流 loader、環境等價比對、finalizer／recovery／preflight **共用**，⛔ 不各寫一份 |
| ⚠️ **環境等價比對**（v10，③b；建議 `replay_bundle/envcheck.py`） | equivalence artifact 的 builder／validator、`EXIT_ENV_NOT_EQUIVALENT = 7`（⚠️ 與其他結束碼常數同一落點：`publish.py`） |
| ⚠️ **stage-scoped identity 與專用 tag**（v10，③b） | `run_identity.py` 的 `default_run_identity_path()` 依 stage 固定推導；`pin-replay-image.sh` 改用**專用 tag** ＋ `docker save` tarball；`ensure-i074-run-identity.py`、`validate-i074-run-identity.py`、`run-replay-offline.sh` 支援 Stage 2 identity——⚠️ **Stage 1 的路徑與行為逐項不變** |
| ⚠️ `replay_bundle/publish.py`（v10） | ⚠️ **只新增兩個結束碼常數**（6、7），⛔ 原語不改 |
| `replay_bundle/__init__.py` | 公開 API 匯出 |
| `replay_bundle/evidence.py` | ⛔ **一行都不改** |
| `replay_bundle/publish.py` | ⛔ **原語不改**——共用（⚠️ v10 只新增結束碼常數，見上） |
| **`scripts/finalize-stage2-evidence.sh`** ⬅️ **新增** | ⛔ 不動 `finalize-evidence.sh`（Stage 1 已驗收）。⚠️ **含「四之二」的隔離 worktree 重算**；⚠️ **v10：七種模式見「七之四」的 CLI matrix** |
| `replay_bundle/artifacts.py` | ⚠️ **新增** `validate_comparison_artifact()`／`validate_report()`（⛔ 目前只有 builder）；⚠️ **v11 改寫**：v10 寫的「既有 validator 一律不動」與「五之一」的串流要求**互相矛盾**——改成「**既有 validator 的公開 contract 與 Stage 1 行為不變，允許內部重構成 row-level 共用原語**」，Stage 0／Stage 1 既有測試⛔ 不修改斷言 |
| **`.gitattributes`** | ⚠️ 改成 `**/*.patch`／`**/*.log`——⛔ v1 的第一層規則對 `evidence/patch/` 與 `failed/*/patch/` **無效**（已實查） |
| `scripts/lib/replay-args.sh` | ⚠️ **⛔ 本步驟不改**——兩份 patch 的**凍結**與 `write-tree` SHA 計算屬 ⑦；③ 只定契約 |
| Python 測試 | 新增 `tests/test_stage2_evidence.py`；⚠️ **v10**：另加 `tests/test_envcheck.py`（環境等價比對）與串流讀取的測試 |
| shell 測試 | `scripts/test-replay-args.sh` 補新腳本的 argv 所有權；⚠️ **v10**：新增 `stage2_finalizer_argv.json`（七種模式） |

**資料流**（⚠️ v10 改寫）：

1. **環境閘門**（③c）：after' 見證趟的 operational 輸出 → `finalize-stage2-evidence.sh --envcheck`
   → 串流比對 ＋ E1、E2、E3a、E4～E7 → 發布 `envcheck/`（0 或 7）；
2. **正式 Stage 2**（⑩）：before source artifact ＋ comparison ＋ report（operational）
   → `finalize-stage2-evidence.sh --finalize` 收進 staging ＋ 兩份凍結 patch ＋ Stage 2 identity
   → 合成守門 ＋ **Stage 1 信任錨十道** ＋ **環境見證錨** ＋ 全圖 → 一次性 rename → rc=0；
3. patch 失效（rc=6）→ `--publish-failed-record`。

##### 九、風險與回滾

| 風險 | 對策 |
|---|---|
| ⚠️ **重構 `artifacts.py` 的 validator 改變 Stage 1 行為**（v11） | ⚠️ 公開簽章、例外型別與判定結果一律不變；Stage 0／Stage 1 既有測試⛔ 不修改斷言；⚠️ 另對已封存的 Stage 1 evidence 重跑既有 validator，結論必須與重構前相同 |
| ⛔ **動到 Stage 1 的 evidence 模組，已封存證據驗不過** | ⚠️ **最高風險**。新模組、`evidence.py` ⛔ 一行不改；⚠️ **Stage 1 既有測試全部續跑且⛔ 不修改斷言** |
| ⚠️ Stage 2 archive ⛔ 不自足（方案 (ii)） | manifest 記相對 repo root 的 path；finalize 與 recovery 都重算 SHA；⚠️ 文件註明兩者**必須一起保存** |
| ⚠️ raw blob 繞過 canonical 驗證 | **型別由 layout 決定**，⛔ 不由 manifest 宣告；raw entry ⛔ 無 `artifact_sha256` 欄位 |
| ⚠️ patch 位元組被正規化 | `.gitattributes` 的 `-text`——⚠️ **nested 規則是本步驟才新增**（⛔ ② 加的第一層規則對 `evidence/patch/` **無效**，已實查）；⚠️ validator 直接比 `stored_sha256` |
| ⚠️ 順序錯亂而 hash 相同 | `ordered_components` 強制；⛔ 不依賴「hash 必然不同」 |
| ⚠️ failed-attempt record 與成功 archive 混淆 | 不同根目錄 ＋ 不同 kind ＋ 命名可辨識；⚠️ recovery ⛔ 不接受 failed root |
| ⛔ **finalizer／recovery／preflight OOM**（v10） | 「五之一」的串流模型 ＋ 每個程序各自 < 450 MiB 的 harness；⛔ 不得停 live container |
| ⛔ **環境見證被當成通過、其實沒重比**（v10） | ⚠️ **v11**：每個呼叫端都重做 **E3a** 的全量串流比對（重算的 outcome 必須與封存值、manifest 三者一致）；Stage 2 的 preflight、finalize、recovery 另做 **E3b**（只接受 `EQUIVALENT`）；⛔ 不信任封存值 |
| ⚠️ **Stage 2 identity 被換掉**（v10） | 信任錨第 10 道、F2-a、E5 都要求逐位元等於 `envcheck/` 封存的那一份；⚠️ identity 路徑依 stage 固定推導、⛔ 不開放覆寫 |
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
| k2 | ⚠️ **metadata 全部重算相符、但存在跨檔矛盾**（例如 comparison 的某列 `after` 與錨定 after 不符） | ⚠️ **必須由「五之零」的 graph 抓到**（v10 起十六道）——⛔ 證明 recovery ⛔ 不是只被 SHA 擋下 |
| l | recovery 成功 | ⚠️ 回 manifest 的 `terminal_outcome`，⛔ 不是寫死的 0 |
| m | failed-attempt record 的產出與可辨識性 | 位置、kind、bounded 診斷、兩份 patch 都在 |
| n | 重跑守門：同**語意 SHA** ＋ 同 run identity（⚠️ **⑦ 總綱 v1 第四輪 review（✅ 2026-09-30 確認）**：原本寫「同 SHA」） | ⛔ **在 replay 之前**中止（rc=2）。⚠️ **v10 拆層**：本輪驗 `--check-failed-record` 回 **2**；「runner 在 replay 之前呼叫它」是 **⑦ 的驗收**（同 `ax`） |
| o | 重跑守門：白名單產品檔的 diff 改變、**語意 SHA 不同**（⚠️ **⑦ 總綱 v1 第四輪 review（✅ 2026-09-30 確認）**：原本寫「改過 patch（不同 SHA）」——只改測試檔時完整 SHA 也不同，⛔ 不得放行，見 o2） | ✅ 放行 |
| ⚠️ o2 | **第四輪新增**：只改兩個測試檔——完整 SHA 不同、**語意 SHA 相同** | ⚠️ **rc=2**，⛔ replay 之前拒絕 |
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
| w | Stage 1 信任錨**第 1～9 道全覆蓋**（⚠️ 複合規則拆成子案例）：`manifest_path` ≠ 常數／manifest SHA 不符／manifest 本身不合法／`members` key 集合被增減／**三份**任一的 SHA ≠ Stage 1 entry／重算不符／`bundle_id` 不符（⚠️ v10：`expected_image_id` 只驗 Stage 1 內部的鏈）／cohort validator 不過／cohort 的 `after_artifact_sha256` 或 keys 不符。⚠️ **第 10 道由 ai 負責** | ⛔ 全部 fail-closed |
| x | `manifest_path` 是絕對路徑／含 `..`／symlink 逃出 repo root | ⛔ 全部拒絕 |
| y | ⚠️ **shell finalizer 的合成重算**：換掉任一份 patch → `composed` 不符 | ⛔ **中止且⛔ 不呼叫 Python finalizer**；⚠️ **成功 archive 與 failed record 兩條路徑各一支** |
| z | `patch/counterfactual.patch` 為 **0 bytes** | ⛔ 中止；⚠️ 而 `tooling.patch` 為 0 bytes ✅ 通過 |
| aa | failed record **損壞／缺檔**時的 lookup | ⛔ **fail-closed**（⛔ 不得忽略後繼續） |
| ab | `--recover-failed-record` 成功 | ⚠️ 回 **1**（⛔ 不是 0），且同**語意 SHA** 仍⛔ 不得重跑（⚠️ **⑦ 總綱 v1 第四輪 review（✅ 2026-09-30 確認）**：原本寫「同 SHA」） |
| ac | 錨定的 after／cohort **只比 SHA、跳過完整 validator** | ⛔ 中止——⚠️ 錨定⛔ 不等於驗過（見「三之一」） |
| ad | cohort 信任錨三道（`validate_cohort_manifest()` 不過／`after_artifact_sha256` ≠ 已錨定 after／keys ≠ `candidate_keys()`） | ⛔ 全部 fail-closed |
| ae | `manifest_path` ≠ 寫死常數 | ⛔ 中止 |
| af | before source 的 keys ≠ 已錨定 after 的全量 keys（多／少／換序／重複） | ⛔ 全部 fail-closed |
| ag | 「五之零」的第 **8～13** 道各一支 | ⛔ 全部 fail-closed；⚠️ **含 `rows_shown = 0` 的空報告必須被擋** |
| ag2 | ⚠️ **第 6 道**：某份 artifact 內部的 `bundle_id` 與 identity／Stage 1 manifest 不一致 | ⛔ 中止 |
| ag3 | ⚠️ **第 7 道**：provenance **位置缺漏**／**role 錯**／`image_digest` ≠ `expected_image_id` | ⛔ 各一支 fail-closed |
| ah | failed record 的不變條件 **F1～F10**（⚠️ 含 **F6-a／b／c** 的欄位集合與型別、**F8-a** 的合成守門）（identity validator／頂層對 embedded／provenance 綁定／目錄名不符／`failure_reason` 非列舉值／計數與 sample 長度／非嚴格 boolean） | ⛔ 全部 fail-closed |
| ⚠️ ah4 | **v11：`bounded_diagnostics` 的 union**——兩種原因各自的合法形狀；原因與欄位不相配（例如 `rr_not_restored` 卻帶 `inconsistent_row_count`）；計數為 0；sample 缺 `rr_decoupling_candidate`；`failure_reason` 是舊值 `before_candidates_nonempty` | 合法形狀 ✅；其餘 ⛔ 全部 fail-closed |
| ⚠️ ah5 | **v11：F10**——sample 沒有違反它宣稱的那一條（兩種原因各一支） | ⛔ fail-closed |
| ah2 | ⚠️ **F2-a**：一份**只有 `created_at` 不同**、其餘完全合法的 record | ⛔ **必須被拒**——⚠️ 這正是查找鍵／目錄鍵矛盾的入口 |
| ah3 | `--check-failed-record` 的兩段（Python 段過但 shell 合成段不過，反之亦然） | ⛔ 任一段失敗即 fail-closed |
| ai | ⚠️ **v10 改寫**：Stage 2 的 `run_identity` 與 **`envcheck/` 封存的 Stage 2 identity** **不完全相同**（含只有 `created_at` 不同） | ⛔ 中止（信任錨第 10 道）——⚠️ **⛔ 不是「lookup 不命中後放行」**。⚠️ **v10 拆層**：本輪驗 `validate_stage1_anchor_graph()` 拒絕；「在 replay 之前」是 **⑦ 的驗收** |
| aj | ⚠️ **`.gitattributes` 對巢狀路徑生效** | `git check-attr` 斷言 `evidence/patch/*.patch` 與 `failed/*/patch/*.patch` 的 `text` 是 **unset**——⛔ 這一條是 v1 真正失效過的地方 |
| ak | failed root **不存在／為空** | ✅ **rc=0 放行** |
| al | records 都有效、但**語意 SHA 都不同** | ✅ **rc=0 放行**（⚠️ **⑦ 總綱 v1 第四輪 review（✅ 2026-09-30 確認）**：原本寫「完整 SHA 都不同」——只改測試時完整 SHA 不同卻應命中） |
| am | **語意 SHA 命中** | ⚠️ **rc=2**，⛔ replay 之前拒絕；⚠️ **⑦ 總綱 v1 第四輪 review（✅ 2026-09-30 確認）**：至少兩支——完整 SHA 也相同、完整 SHA 不同但只改測試 |
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
| ay | preflight **⛔ 未執行第 8／9 道**（cohort 壞掉） | ⛔ **必須在 replay 之前**就被擋下。⚠️ **v10 拆層**：本輪驗「兩支 helper 合起來涵蓋第 8、9 道並拒絕」；「preflight 入口在 replay 之前呼叫兩支」是 **⑦ 的驗收** |
| az | ⚠️ after／cohort 改成**另一個合法 bundle_id 或合法 image digest**，並**同步重算檔案與 manifest SHA** | ⛔ **仍必須在 preflight 被拒**——⚠️ 這正是「各檔都合法、鏈條對不起來」的案例（道 6-a／6-b） |
| ba | `TOOLING_PATCH` **空／未提供** | ⚠️ 產生 **0-byte 凍結副本**，`tooling_patch_sha256` ＝ 空字串 SHA；⛔ **不得跳過凍結**。⛔ **⛔ 不是本輪的完成條件**——⚠️ 凍結實作在 **Stage 2 步驟 ⑦**，**`ba` 是 ⑦ 的驗收**（同 `ax`）。⚠️ **⑦ 總綱 v1（✅ 2026-09-30 確認）拆兩層**：**runner 層（⑦a）**照本列；**orchestrator 層（⑦b）**改成「tooling 必須非空、且 ＝ 複本內封存的那一份，否則在 replay 之前中止」 |

⚠️ **v10 新增**：

| # | 案例 | 要驗什麼 |
|---|---|---|
| bb | before source 有一列 `CONTINUATION` 且 `setup_rr_qualified == false`、但 flag 為 `false` | ⛔ **中止**（全圖第 15 道）——⚠️ `candidate_keys == ∅` 單獨會放行它 |
| bc | before source 某列 flag 與等價式不符 | ⛔ 中止（第 15 道） |
| bd | 環境見證錨 E1～E7 **各一支**（含：equivalence 封存 `EQUIVALENT` 但重比不相等；`witness_distributions` 與 hash 不符；image 鏈或 bundle 鏈斷掉；after' 的 `base_commit` 不是 `e1cbbbd`） | ⛔ 全部 fail-closed |
| ⚠️ bd2 | **v11：合法的 NOT_EQUIVALENT archive** | ⚠️ `--envcheck` 回 **7** 且證據已發布；`--recover-envcheck` 回 **7**；⚠️ Stage 2 的 preflight、`--finalize`、`--recover-durability` ⛔ **都被 E3b 擋下** |
| ⚠️ bd3 | **v11：outcome 三者不一致**（重算值、equivalence 封存值、manifest 的 `terminal_outcome` 任兩者不符，各一支） | ⛔ 發布與 recovery 都 fail-closed（rc=1） |
| ⚠️ bd4 | **v11：envcheck manifest 的封閉 schema**（多欄／缺欄／kind 錯／`files` 多或少一個 key／出現 `raw_blob`／`finalizer_provenance` 的 image 不等於 `expected_image_id`） | ⛔ 全部 fail-closed |
| ⚠️ be2 | **v12：key 集合不一致的三種形狀**——① 只在 reference 側的 key；② 只在 witness 側的 key；③ key 相同、canonical bytes 不同 | 三種都是 NOT_EQUIVALENT（7）；①② 進 `key_mismatch_*` 且 `side` 正確、⛔ **不計入** `row_mismatch_count`；③ 進 `row_mismatch_*`；⚠️ 計數不變條件與 `rows_compared`（交集）三種情形都要成立 |
| be | 環境等價判定：全相同／判定表第 1～3 條各自不成立／第 4 條的欄位不同 | EQUIVALENT（0）／NOT_EQUIVALENT（7，⚠️ **證據照樣發布**）／⚠️ 第 4 條⛔ **不得**翻轉結果 |
| bf | ⚠️ **串流**：finalizer、recovery、preflight、環境等價比對 | ⚠️ 以計數或 spy 斷言**每份全量 artifact 只讀一次**、⛔ 沒有同時持有兩份全量 rows |
| bg | CLI matrix：七種模式的 argv 逐 token、互斥、重複、使用者以 `--source` 覆寫 | 依「七之四」；覆寫與互斥違反 ⛔ 全部中止 |
| bh | `--publish-failed-record` | F8-a 合成守門不過 ⛔ 不發布；完整發布回 **1**；rename 成功、fsync 失敗回 **3** |
| bi | `--recover-durability` 重做合成守門 | 換掉 archive 內任一份 patch（同步改 manifest）⛔ 必須被合成守門擋下 |
| ⚠️ bj0 | **v11：row-level 原語** | ⚠️ 同一組壞列分別餵給整批 validator 與串流 validator，**被擋下的列與錯誤判定必須相同**；⚠️ Stage 0／Stage 1 既有 validator 測試⛔ 不修改斷言、全數續跑 |
| bj | report 的 200 | ⚠️ 斷言它等於 bundle manifest 的 `report_max_rows` |
| bk | N | ⚠️ 斷言 N ＝ 20，且**三種 sample**——failed record 的 `sample_keys`、envcheck 的 `mismatch_sample_keys` 與 `key_mismatch_sample_keys`（⚠️ v12 補上第三種）——都用**同一個常數**；⚠️ 三者的完整計數（`inconsistent_row_count`／`before_candidate_count`、`row_mismatch_count`、`key_mismatch_count`）照樣保留，⛔ 不受 N 截斷 |

⚠️ **另外**：`bounded_diagnostics` 的 N ⛔ 不得是執行期參數，**寫死**並由測試釘住（⚠️ N ＝ 20：v10 當時待確認，✅ 已於 2026-09-23 隨 Stage 2 計畫書 v28 的 review 確認）。

##### 十一、完成後的歸檔位置

| 內容 | 歸檔到 |
|---|---|
| Stage 2 evidence 的 layout、manifest schema、兩種 entry 型別 | `sr-zone-scoring.md` |
| 兩份 patch 的三方 SHA 關係與 ordered components | `sr-zone-scoring.md`（⚠️ 與「⛔ 不讓語意修改偽裝成 instrumentation」寫在一起） |
| failed-attempt record 與重跑守門 | `development-workflow.md`（⚠️ 它是**操作程序**，⛔ 不是評分規格） |
| 「Stage 2 archive ⛔ 不自足、必須與 Stage 1 evidence 一起保存」 | `development-workflow.md`（比照「凍結 bundle 不得重產」）。⚠️ **v10**：改成「與 Stage 1 evidence **及 `envcheck/`** 三者一起保存」 |
| ⚠️ **環境見證**（v10）：`envcheck/` 的 layout、equivalence schema、E1～E7 | `sr-zone-scoring.md`（與信任錨寫在一起） |
| ⚠️ **finalizer 的 CLI matrix 與記憶體模型**（v10） | `development-workflow.md`（操作程序） |

##### 十二、執行順序

⚠️ **v10 依 Stage 2 計畫書 v26 的 ③a～③d 重排**（✅ 已隨 v28／③ v12 確認（2026-09-23））：

```text id="i074_stage2_evidence_order_001"
① 本計畫書與 Stage 2 計畫書的現行版確認（② 的 review 已於 2026-09-23 通過）   ← ✅ 2026-09-23
② 第一包（Stage 2 的 ③b）：   ← ✅ 2026-09-23 實作完成、✅ review 通過（見「③b 實作結果」）
     共用的封閉 layout archive 核心、串流讀取、⚠️ `artifacts.py` 的 row-level 共用原語（v11）、
     環境等價比對、envcheck manifest（v11）、stage-scoped identity ＋ 專用 tag ＋ tarball、
     finalize 腳本的 envcheck／recover-envcheck 兩種模式
     → 測試：`bd`～`bd4`（能在本包驗的部分）、`be`、`bf`（環境等價比對的部分）、`bg`（兩種模式）、`bj0`
     → review
③ 環境閘門（Stage 2 的 ③c）：pin → after' 見證趟 → `--envcheck`   ← ✅ 2026-09-23 EQUIVALENT（rc=0）
     └ rc=7（NOT_EQUIVALENT）→ ⛔ 停止，另立 issue；⛔ 不進 ④
④ 第二包（Stage 2 的 ③d）：stage2_evidence.py 的其餘部分 ＋ finalize 腳本的其餘五種模式   ← ✅ 2026-09-24 實作完成、✅ review 通過（見「③d 實作結果」）
     → 測試矩陣 `a`～`aw` ＋ `ay` ＋ `az` ＋ `ah4`／`ah5` ＋ `bb`～`bk`（⚠️ `bd2` 的「Stage 2 被 E3b 擋下」那一半在本包驗）
       （⚠️ **`ax`、`ba` 與 `n`／`ai`／`ay` 的「在 replay 之前」那一層屬 Stage 2 步驟 ⑦ 的驗收**，⛔ 不在本輪）
     → review
⑤ 回到 Stage 2 計畫書的步驟 ④（sizing harness）
```

⛔ **④ 未完成前⛔ 不得進入 Stage 2 計畫書的 ⑦**；⛔ **③ 判 NOT_EQUIVALENT 就⛔ 不進 ④**。

#### ③b 實作結果（2026-09-23，✅ **review 通過**）

✅ **依 Stage 2 計畫書 v28 與 ③ evidence contract v12 完成第一包**（環境閘門的前置）。
⛔ **本輪沒有跑任何 replay、沒有 pin 新 image、沒有 commit**——那些屬 ③c，要等 review 通過。

| 檔案 | 內容 |
|---|---|
| `replay_bundle/artifacts.py` | 抽出 **row-level 共用原語**：`check_row_object()`／`check_candidate_flag()`／`replay_error_messages()` ＋ `raise_replay_errors()`／`check_diagnostics_row()`，以及 `validate_after_envelope()`；既有整批 validator 改為呼叫它們（⚠️ 以 `git diff -w` 確認規則本體**一字未改**）；新增單趟組合的 `StreamRowValidator` |
| `replay_bundle/stream.py`（**新增**） | `stream_canonical_artifact()`：一趟讀取完成 raw／payload SHA、**逐欄逐列重新編碼比 SHA**（canonical 檢查）、`.json.gz` 的 **round-trip 串流比 SHA**、schema／kind；可在同一趟寫出 canonical gzip 副本（`copy_to`）。`stream_after_artifact()`：只常駐 keys、每列 digest 與指定的完整列 |
| `replay_bundle/stage2_evidence.py`（**新增**，本包只放共用部分） | 串流版 `load_stage1_anchor()` ＋ `validate_stage1_anchor_graph()`（道 10 已改寫）、`stage1_evidence` 的 builder／validator、`ClosedArchiveWriter`（staging → **對 staging 跑完整驗證** → `rename_noreplace` → fsync parent）、`resolve_repo_path()`、`DIAGNOSTIC_SAMPLE_LIMIT = 20` |
| `replay_bundle/envcheck.py`（**新增**） | 環境等價判定、equivalence／envcheck manifest 的封閉 schema、E1～E7（E3a／E3b）、`publish_envcheck()`、`recover_envcheck()`、`verify_envcheck_for_stage2()`、CLI（`--envcheck`／`--recover-envcheck`） |
| `replay_bundle/provenance.py` | 抽出 `installed_distributions()`，`pip_freeze_sha256()` 改為它的 SHA（⛔ 兩者不各自推導） |
| `replay_bundle/run_identity.py`、`python/scripts/ensure-i074-run-identity.py` | identity 路徑**依 stage 固定推導**（封閉列舉 1／2）；Stage 1 的推導結果不變 |
| `replay_bundle/publish.py` | 新增 `EXIT_ENV_NOT_EQUIVALENT = 7`（⚠️ 結束碼 6 屬 ⑦，本輪未加） |
| `scripts/pin-replay-image.sh` | `--stage 2`：**專用 tag**、⛔ 不接受 `PY_IMAGE`、Stage 2 identity、`docker save` tarball ＋ `.sha256`；Stage 1 行為逐項不變 |
| `scripts/restore-replay-image.sh`（**新增**） | 先驗 tarball SHA → `docker load` → 再驗 image ID → 重新掛上專用 tag |
| `scripts/run-replay-offline.sh` | `I074_STAGE`（封閉列舉 1／2）選 identity；⛔ 只在 I-074 正式流程有意義 |
| `scripts/finalize-stage2-evidence.sh`（**新增**） | CLI matrix 的 `--envcheck`／`--recover-envcheck` 兩種模式（其餘五種屬 ③d） |
| `python/scripts/fixtures/stage2_finalizer_argv.json`（**新增**） | 兩種模式的 argv token 序列 |

**測試**：新增 `test_replay_stream.py`、`test_replay_row_primitives.py`（bj0）、`test_replay_envcheck.py`
（bd～bd4、be／be2、bf、bk，以及信任錨、CLI matrix、stage-scoped identity、**對真正封存的 Stage 1
evidence 跑串流信任錨**），共 139 條；`scripts/test-replay-args.sh` 新增 Stage 2 段落（finalizer
兩種模式的 argv／mount／模式衝突、`pin --stage 2`、restore、`I074_STAGE`）。

| 層 | 結果 |
|---|---|
| `python/scripts/test.sh`（完整） | **1361 passed, 1 skipped**（② 的 baseline 是 1222）；doc-refs 與 `test-replay-args.sh` 全過（184 項 ok） |
| 反向驗證（把回歸注回產品程式，確認測試會紅） | ① 拿掉 E3b → `be2` 三支紅；② `row_mismatch` 誤計單側 key → `be2`／`bk` 紅；③ 拿掉 gzip round-trip 檢查 → 四支非 canonical gzip 測試紅；④ 拿掉 E7 的 base_commit 檢查 → E7 測試紅；⑤ 串流 validator 漏掉 diagnostics → bj0 七支紅。全部還原後重跑通過 |

**真實資料實測**（host，2026-09-23）：

| 項目 | 結果 |
|---|---|
| 分塊重新壓縮 D+1 是否與封存檔逐位元相同 | ✅ 相同（stored `8cab461c…`、payload `33a6b166…` 都與 Stage 1 manifest 一致），2.2 秒 |
| 串流驗證 D+1（13,417 列、156 候選） | 8.7 秒，峰值 RSS **135 MiB**（整份載入約 449 MiB） |
| 串流信任錨 `load_stage1_anchor()` ＋ 跨檔關係 | 9.3 秒，峰值 RSS **138 MiB**；156 列 cohort rows 全部保留 |

**⚠️ 與計畫的差異（✅ review 同意）**：

| # | 差異 | 理由 |
|---|---|---|
| 1 | ⚠️ **判定表第 3 條的 `base_commit`／`tooling_patch_sha256` 不符 → 結束碼 1、⛔ 不發布**，⛔ 不是計畫寫的 NOT_EQUIVALENT（7）。✅ **review（2026-09-23）同意這個方向**，但要求**在 replay 之前就擋**——見下方「第一輪 review 的修正」高 1 | 這兩欄同時也是 **E7**（after' 必須是原始 `e1cbbbd`、⛔ 不套 patch）——E7 先擋。跑錯版本的那一趟⛔ 不是環境見證；若當成 NOT_EQUIVALENT 發布，會依計次政策（after' 不得重跑）**永久擋住 Stage 2**。`project_modules_sha256`／`runtime_settings` 不符仍是 NOT_EQUIVALENT（7）。測試 `be` 已依此拆成兩組 |
| 2 | ~~recover-envcheck 的 shell argv 測試只在 repo 內還沒有真正的 `envcheck/` 時才跑~~ | ⛔ **已由第一輪 review 推翻並修正**（中 3）：改在隔離的最小 repo 裡跑，⛔ 不再因正式證據存在而 skip |
| 3 | 發布時對 staging 跑**完整**的 E1～E7（與 recovery 同一段程式） | 比「只驗記憶體物件」強；代價是多串流讀一次 staging 內的副本 |

##### ③b 第一輪 review 的修正（2026-09-23）

| # | 問題 | 修正 |
|---|---|---|
| 高 1 | ⛔ **E7 只在 replay 完成後才檢查**——runner 在 Docker 之前就算出了 `BASE_COMMIT` 與 `TOOLING_PATCH_SHA256` 卻沒驗，錯設 `AFTER_REF` 或殘留 `TOOLING_PATCH` 要燒完約 180 分鐘才在 envcheck 被拒，而 after' 只有一趟 | ✅ 新增 `python/scripts/check-i074-witness-run.py`（host 端、用**同一套** Stage 1 信任錨）；`run-replay-offline.sh` 在 **Stage 2 identity ＋ Stage 1 模式**（＝見證趟）時，於 worktree 建好、兩個值算完之後、**Docker 之前**呼叫它。envcheck 發布與 recovery 的 E7 照舊保留（⛔ 提早擋不取代正式那一道）。測試分兩種跑法：⚠️ **兩個錯誤案例**（錯的 `AFTER_REF`、殘留的 patch）以 fake docker 跑**非 dry-run**，斷言中止且 fake docker 的 log 裡⛔ 沒有 `run`；⚠️ **放行案例**（正確的 base、無 patch）改用 **dry-run**，斷言輸出裡出現 `docker run`——真的走到 `exec docker run` 時 runner 的 EXIT trap 不會執行、worktree 會留在 `/tmp`（見 [I-118](#i-118replay-相關腳本會洩漏-git-worktree註冊與-tmp-目錄都會累積)） |
| 中 2 | ⛔ **`.sha256` 沒有綁死實際的 tarball**——`sha256sum -c` 驗的是 sidecar 裡寫的那個檔名，sidecar 改指向同目錄的合法 decoy 就能讓被改過的 tar 照樣被 load | ✅ 新增 `scripts/lib/image-tarball.sh` 的 `image_tarball_verify()`（pin 與 restore 共用）：sidecar **恰好一行**、檔名**恰好是 `<hex>.tar`**、讀出 digest **對 tar 本身重算**。測試補「tar 被改＋sidecar 指向合法 decoy」（pin 與 restore 各一）與「sidecar 兩行」 |
| 中 3 | ⛔ **正式 `envcheck/` 產生後，shell 的 recovery 測試會永久 skip** | ✅ finalizer 的 shell 測試**全部**改在隔離的最小 repo（`git init` ＋ 工作樹現行的腳本與模組）裡跑，⛔ 完全不碰真正的 repo；並斷言指令裡⛔ 沒有真正 repo 的 baselines 路徑 |
| 低 4 | ⚠️ 新增的 shell CLI 仍靜默接受部分重複／多餘參數 | ✅ `finalize-stage2-evidence.sh` 的 `--source-ref` 重複、`pin-replay-image.sh` 的 `--stage` 重複與 bundle 之後的多餘參數（含 `--no-identity` 之後）一律中止；各補 shell 測試 |

⚠️ **③c 之前要注意**：`finalize-stage2-evidence.sh` 跑的是 **`--source-ref`（預設 HEAD）worktree 裡的程式**
——③b 必須**先 commit** 才會被正式執行用到（與 Stage 1 finalizer 相同）。

**歸檔**（✅ review 通過）：操作程序寫進 [`development-workflow.md`](./development-workflow.md)
「I-074 Stage 2 的環境見證程序」（含 tarball 的保存位置與驗 SHA 步驟）；契約寫進
[`sr-zone-scoring.md`](./sr-zone-scoring.md)「I-074 Stage 2 的環境見證契約」。

#### ③c 準備：Stage 2 image 的來源（2026-09-23，✅ `--adopt-image` review 通過）

⚠️ **③c 開始前的實測**（唯讀；在容器內用 `provenance.py` 的同一套計算）：

| image | 建立時間 | `pip_freeze_sha256` | 與 Stage 1（`7a39573e…`） |
|---|---|---|---|
| `stock_trading-python-server:latest`（`sha256:2a90ad1c1dd8…`） | 2026-09-17 10:05（遺失的 image 之後 16 分鐘，同一份 Dockerfile） | `7a39573e…` | ✅ **完全相同**（Python 3.11.16、sklearn 1.9.1、pandas 3.0.5） |
| `stock-trading-python-test:latest` | 2026-09-23（本輪測試重新 build） | `dd7d34c7…` | ❌ 不同（例如 pandas 3.0.6）——⚠️ **layer cache 已更新**，照原設計 `pin --stage 2` 重新 build 也會裝到這一組 |

⚠️ **所以照原設計重新 build，見證趟判成 NOT_EQUIVALENT 的風險明顯較高**，而 after' 只有一趟。

✅ **使用者裁決（2026-09-23）**：**採用 python-server image**，⛔ 不重新 build；commit 由使用者自己做。

**實作**（✅ review 通過）：

| 檔案 | 內容 |
|---|---|
| `scripts/pin-replay-image.sh` | 新增 `--adopt-image <完整 image ID>`：只限 `--stage 2`、⛔ 不收 tag、重複即中止；identity **還不存在**時：確認 image 在本機 → **在該 image 內**算 `{pip_freeze_sha256, python_version}` → 必須**逐字等於** Stage 1 信任錨的 → 才掛上專用 tag、建立 Stage 2 identity、存 tarball；identity **已存在**時只接受指向同一個 image（no-op），⛔ 不得藉此換掉已釘死的 image |
| `python/scripts/print-i074-environment.py`（**新增**） | `--stage1`：取自**已驗證的** Stage 1 信任錨（`load_stage1_anchor()` 的 after provenance）；`--current`：該環境自己的——與 `build_provenance()` 同一個來源。stdout 只印一行 canonical JSON |
| `python/scripts/_i074_bootstrap.py` | `load_replay_bundle()` 可指定只載入部分模組（`--current` 只要 `canonical`／`provenance`） |
| `scripts/test-replay-args.sh` | `--adopt-image` 的八條：非 Stage 2、tag、重複、找不到 image、環境不同（⛔ 沒有 tag／identity／tarball）、環境相同（⛔ 不 build、tag、identity、tarball 可驗證）、identity 已釘死時換 image（拒絕）與同一個（no-op） |

實測：`print-i074-environment.py --current` 在 python-server image 內的輸出與 `--stage1` **逐字相同**，
在今天的測試 image 內則不同（`dd7d34c7…`）。

**③c 的接續步驟**（commit 之後）：

```text id="i074_stage2_3c_steps_001"
① pin      scripts/pin-replay-image.sh --stage 2 \
             --adopt-image sha256:2a90ad1c1dd801d59373988a4d84afe1437cbee62ccab3709b06cfa8f7da5027 <bundle>
             （guarded 寫法見 development-workflow.md「I-074 Stage 2 的環境見證程序」）
② 見證趟  AFTER_REF=e1cbbbd I074_STAGE=2 scripts/run-replay-offline.sh … --i074-preflight  （約 180 分鐘；凍結窗口 A 開始）
③ 判定    scripts/finalize-stage2-evidence.sh --envcheck --run-dir <run 目錄>
```

#### ③c 執行結果（2026-09-23，✅ **EQUIVALENT**）

| 步驟 | 時間 | 結果 |
|---|---|---|
| ① pin | 2026-09-23 16:31 | ✅ `pin-replay-image.sh --stage 2 --adopt-image sha256:2a90ad1c…`：在該 image 內比對 `{pip_freeze_sha256: 7a39573e…, python_version: 3.11.16}` **逐字等於** Stage 1 → 掛上專用 tag、建立 Stage 2 identity（`created_at` 2026-09-23T08:31:33Z）、tarball `…/i074_stage2/images/2a90ad1c….tar`（832,766,976 bytes，SHA-256 `a3c2d3a8c1c57f75c61a3faa12fe55ed1aefb290b8ce19ccacc832d7741ca583`，以 `image_tarball_verify()` 重驗通過） |
| ② 見證趟 | 16:33:47 → 19:32:10（**178 分鐘**） | ✅ rc=0；HEAD `3d37691`、程式碼 `e1cbbbd`、⛔ 無 patch（E7 前置守門通過）；13,417 列、候選 **156**；after' `f5d6d9ea…`（77,904,262 bytes）；mem-guard 上限 549m，**峰值約 315 MiB**（每 5 分鐘取樣）；三次 `InconsistentVersionWarning`（sklearn 1.9.0 → 1.9.1）與 Stage 1 當時相同 |
| ③ envcheck | 19:36 | ✅ **rc=0，EQUIVALENT**；`python/baselines/i074_stage2/envcheck/` 已發布（5 檔），✅ 於 `5bae980` 進版控 |

**環境等價判定的內容**（`equivalence/equivalence.json.gz`）：

| 項目 | 值 |
|---|---|
| key 集合 | reference 13,417 ＝ witness 13,417；`rows_compared` 13,417；`key_mismatch_count` **0** |
| 逐列 | `row_mismatch_count` **0**——⚠️ **13,417 列的 canonical row bytes 全部逐位元相同** |
| cohort | `cohort_equal` **true**（156 keys） |
| 必須相同的四欄 | `base_commit`／`tooling_patch_sha256`／`project_modules_sha256`／`runtime_settings` **全部相同** |
| 允許不同的六欄 | `image_digest`（`d66030dc…` → `2a90ad1c…`）、`runner_sha256`、`argv` 不同——⚠️ raw argv 有三個 token 不同：`--image-digest` 與 `--runner-sha256` 的值（反映前述兩欄），以及 `--output-dir`；`pip_freeze_sha256`、`python_version`、`source_root` 相同 |
| 信任錨 | reference after `33a6b166…`、Stage 1 manifest `485fb601…`；witness after `f5d6d9ea…`、cohort' `d5768e8f…` |

**獨立複核**：在 host 以 `verify_envcheck_for_stage2()` 對已發布的 `envcheck/` 重跑 **E1～E7 ＋ E3b**，通過（17.9 秒）。

⚠️ **這代表什麼**：新 image（`2a90ad1c…`）與遺失的 Stage 1 image 對這份 bundle 的每一列輸出**逐位元相同**
——封存的 D+1 可以繼續當 after 側，before 正式趟（⑩）與 D+1 之間的差異只能來自 counterfactual patch。
⚠️ **凍結窗口 A 已結束**（`envcheck/` durable、rc=0）。

⚠️ **待辦**：
* ~~`python/baselines/i074_stage2/envcheck/` 要進版控~~——✅ 已於 `5bae980` 進版控（與 `python/baselines/i074_stage1/` 必須一起保存）；
* 見證趟的 runner 以 `exec docker run` 結束，留下一個 detached worktree（I-118 的同一個成因），已手動移除；
* ~~下一步：**③d**（Stage 2 archive、failed record、check／recover 的其餘部分）~~——✅ 2026-09-24 已實作，見「③d 實作結果」。

#### ③d 實作結果（2026-09-24，✅ **review 通過**）

✅ **依 Stage 2 計畫書 v28 與 ③ evidence contract v12 完成第二包**（「十二、執行順序」的 ④）。
⛔ **本輪沒有跑任何正式 replay、沒有動 `python/baselines/` 的任何檔案、沒有 commit**；真實規模的實測與
端到端都在 scratchpad 的複本上做（見下）。

| 檔案 | 內容 |
|---|---|
| `replay_bundle/stage2_archive.py`（**新增**） | 成功 archive：封閉 layout（7 檔）、`raw_blob` entry、manifest builder／validator、before source 的串流 validator（`stream_before_source()`）、**`CounterfactualEffectCheck`**（「①之三」的檢查順序，⚠️ 執行期與證據層共用）、`verify_stage2_graph()`（十六道）、`finalize_stage2_evidence()`、`recover_stage2_durability()`；failed record：中繼檔 schema、`validate_bounded_diagnostics()`（F5～F7、F10）、builder／validator／`verify_failed_record()`（F1～F10 不含 git 的部分）、`publish_failed_record()`、`check_failed_records()`、`recover_failed_record()`；合成守門的宣告值 `patch_claims()`；CLI（五種模式） |
| `replay_bundle/artifacts.py` | 新增 `validate_comparison_artifact()`（每列必須恰好等於 `compare_rows()` 的重算、keys 排序唯一）與 `validate_report()`（截斷與統計由完整 comparison 重算、`rows_shown` ＝ `min(上限, candidate_rows)`）；⛔ 既有函式一行未改 |
| `replay_bundle/stage2_evidence.py` | `ClosedArchiveWriter.add_raw()`（raw patch）；`load_stage1_anchor()` 在成員缺檔時指出「必須一起保存」（測試 h 抓到的缺口） |
| `replay_bundle/envcheck.py` | `EnvcheckResult` 加 `manifest_sha256`（Stage 2 manifest 的 `environment_witness` 以它錨定，與 manifest 內容來自同一次讀取） |
| `replay_bundle/__init__.py` | 公開 API 匯出 |
| `scripts/finalize-stage2-evidence.sh` | 其餘五種模式；host 端**合成守門**（隔離 worktree、`git apply --index` ＋ `git write-tree`、⛔ 不建中繼 commit、斷言 HEAD 不動）；check 的決策（Python 段先過 → 本次 canonical 比對鍵 → 每份 record 的 F8-a → 命中 2／非 canonical 1／放行 0）；所有暫時 worktree 由 EXIT trap 清掉 |
| `python/scripts/i074-stage2-patch-claims.py`（**新增**）、`_i074_bootstrap.py` | host 端取合成守門的宣告值（只讀小檔）；bootstrap 加 `STAGE2_ARCHIVE_MODULES` |
| `python/scripts/fixtures/stage2_finalizer_argv.json` | **七種模式各一組** argv |
| `.gitattributes` | `python/baselines/i074_stage2/**/*.patch`／`**/*.log`（⛔ 原本的第一層規則對巢狀無效） |

**測試**：新增 `test_replay_stage2_archive.py`（**148 條**，含第一輪 review 修正補的 13 條）；`scripts/test-replay-args.sh` 新增 ③d 段落
（**真的 git** 在隔離 repo 做合成守門：argv／mount、y、bh、bi、F8-a recovery、bg、ar；check 的決策以
fake docker 回傳 Python 段輸出：ak、al、am、as／av、au、at、ap、ao；結尾斷言隔離 repo 的 worktree 歸零）
與 aj（`git check-attr`）。

| 層 | 結果 |
|---|---|
| `python/scripts/test.sh`（完整，第一輪 review 修正後） | **1509 passed, 1 skipped**（③c 時 1361，＋148）；doc-refs 無問題；`test-replay-args.sh` 全過（260 項 ok） |

| 測試矩陣 | 位置 |
|---|---|
| a～m、p、r～w、z、aa、ab、ac、ad、af、ag、ag2、ag3、ah、ah2、ah4、ah5、ai（本輪層）、ak、al（Python 段）、an、aw、ay（本輪層）、az、bb、bc、bd2（Stage 2 那一半）、bf、bj、bk、bg（Python 端）；x、ae 的路徑安全與寫死常數沿用 ③b 的 `test_replay_envcheck.py` | pytest |
| n（本輪層）、o、y、ah3、aj、ak、al、am、ao、ap、aq、ar、as、at、au、av、bg、bh、bi | shell |
| q | 全套既有測試續跑、⛔ 不改斷言 |
| ⛔ ax、ba，以及 n／ai／ay 的「在 replay 之前」那一層 | Stage 2 步驟 ⑦ |

**反向驗證**（把回歸注回產品程式，確認測試會紅；全部還原後重跑通過）：Python 端——拿掉合成 hash 與
before provenance 的綁定、拿掉「①之三」、拿掉 F2-a、拿掉 F10、拿掉 comparison `before` 的來源比對、
把 E3b 關掉、拿掉 before 全量 keys 守門，七項都有測試變紅；shell 端——拿掉合成 SHA 的比對（y、bi、bh、
F8-a recovery、ap 變紅）、把「非 canonical」排到「命中」之前（as／av 變紅）。

**真實規模實測**（2026-09-24，Stage 2 image `2a90ad1c…`，mem-guard 527m；在 scratchpad 複製
`i074_stage1/` 與 `envcheck/`，以 D+1 的 13,417 列合成一份 before source——156 列候選把 RR 加回去——
與對應的 comparison／report，反事實 patch 用真正的 `counterfactual_e1cbbbd.patch`）：

| 程序 | 耗時 | 峰值 RSS |
|---|---|---|
| `finalize_stage2_evidence()` | 37.0 秒 | **299 MiB** |
| `recover_stage2_durability()` | 26.9 秒 | **314 MiB** |
| `check_failed_records()` | 18.0 秒 | **274 MiB** |

⚠️ 都 < 450 MiB（「五之一」）。⚠️ 這**不是** Stage 2 計畫書步驟 ④ 的 sizing harness（那一步仍要做），
只是確認 ③d 的程式在真實規模下跑得完、沒有整份載入。

**端到端**（同一份合成資料；在 scratchpad 以 `git clone --shared` 複本 ＋ 工作樹現行變更，用**真正的
shell 入口**與 Stage 2 image）：`--finalize` rc=0（42 秒，合成守門對 `e1cbbbd` 重建出 `ef7a4cdf…`）→
`--recover-durability` rc=0 → `--check-failed-record` rc=0 → `--publish-failed-record`（156 列候選的
`rr_not_restored`）rc=**1** → 同一份 patch 再 `--check-failed-record` rc=**2** → `--recover-failed-record`
rc=**1**；複本的 worktree 全部清掉。⚠️ 那份複本裡的 commit ⛔ 不在真正的 repo。

**⚠️ 與計畫的差異（✅ review 同意）**：

| # | 差異 | 理由 |
|---|---|---|
| 1 | ③d 的程式放在**新檔 `stage2_archive.py`**，⛔ 不是計畫寫的 `stage2_evidence.py` | `envcheck.py`（③b）import `stage2_evidence.py`，而 ③d 要用 `envcheck.py` 的 E1～E7——放同一檔會循環 import；延到函式內 import 又會踩到「階段 B 之後⛔ 不得有新的 project import」。測試檔同理命名為 `test_replay_stage2_archive.py`（比照 ③b 的 `test_replay_*` 慣例） |
| 2 | ⚠️ **更嚴**：全圖第 8 道多一個等式——`before_ref` ＝ provenance 的 `base_commit` ＝ **Stage 1 after 的 base**；F3 的「兩份 patch 所依附的 base」同樣釘成它 | v3 模型下 before ＝ `e1cbbbd` ＋ 反事實 patch；check 的比對鍵用的也是這個 base（「七之一」）。⛔ 沒有這一條的話，一份跑在別的 base 上的 before 會通過全圖、卻和查找鍵用不同的 base |
| 3 | ⚠️ **補訂**：replay 在結束碼 6 時寫出的中繼檔 `stage2/bounded_diagnostics.json` 除了 `failure_reason` ＋ 分流欄位，還帶 `schema_version`／`kind`（`sr_zone_stage2_counterfactual_failure`）／`bundle_id`／`generated_at`／**`provenance`** | F3 要求 record 的 provenance 是**那一次 replay** 的（`tooling_patch_sha256` ＝ 合成 hash）；finalizer 自己的 provenance 記的是 finalizer 的 argv 與 base，⛔ 不能代替。這份中繼檔由 ⑦ 產出，builder 已放在 `stage2_archive.build_counterfactual_failure()` |
| 4 | 合成守門的**宣告值**由 host 端小工具 `i074-stage2-patch-claims.py` 取出；`--finalize` 的 composed 宣告取自 **comparison 的 provenance**（⛔ 不讀 77 MB 的 before source） | shell 要在呼叫 Python 之前知道宣告值才能「⛔ 不符即不呼叫 Python」。全圖第 10 道要求 before source 與 comparison 的 provenance 逐欄相等、「四之一」第 3 條要求 composed ＝ before 的 provenance，所以兩者等價 |
| 5 | ⚠️ **更嚴**：`validate_patches()` 另外要求「tooling 為空 ⇒ composed ＝ counterfactual」 | 空 tooling 時 T2 ＝ T1，這是必然成立的關係（「四之一」的註記）；多一道就少一種自相矛盾的宣告 |
| 6 | ⚠️ **更嚴**：`failed/` 底下任何認不得的項目（含 `.…staging-…` 殘骸、非目錄）都讓 lookup fail-closed | 計畫只寫「損壞或缺檔的 record」；認不得的項目同樣可能是發布中斷的紀錄，⛔ 忽略它就等於「跳過壞的繼續掃」 |
| 7 | `--check-failed-record` 的 Python 段在**容器內**跑、`i074_stage2` **唯讀**掛載，輸出 `{base_commit, records}` 給 shell 做 F8-a 與決策；使用者的 patch **只在 host** 使用（⛔ 不進容器） | 計畫只寫「Python 段／shell 段」；這樣 Python 段與其他模式同一個 image，且 lookup 不可能寫入證據 |
| 8 | 結束碼 6（`EXIT_COUNTERFACTUAL_INEFFECTIVE`）**仍未加入** `publish.py` | 它由 replay 產生，屬 ⑦（同 ③b 的處理） |

##### ③d 第一輪 review 的修正（2026-09-24）

| # | 問題 | 修正 |
|---|---|---|
| 高 1 | ⛔ **合成守門與實際封存之間有跨程序 TOCTOU**：shell 在 Docker 之前驗 A 的合成關係，Python 之後**重新讀** operational artifact 與 patch；兩段之間來源被一致地換成 B，shell 證明的是 A、Python 驗並封存的卻是 B，而 Python ⛔ 不碰 git、只能驗 B 的宣告值彼此一致（`:ro` 只擋容器寫入，⛔ 凍結不了 host）。影響 `--finalize`、`--publish-failed-record` 與兩種 recovery | ✅ 新增專屬交接 `VerifiedComposition`：shell 把合成守門驗過的 **patch base 與三個 SHA** 以 `--verified-patch-base`／`--verified-counterfactual-sha256`／`--verified-tooling-sha256`／`--verified-composed-sha256` 注入（⛔ 不沿用 `--base-commit`——那是 finalizer 程式碼的來源 commit）；Python 以**實際要封存或要 fsync 的內容**（staging 的全圖結果、archive、record）比對，相符才允許 commit point（`check_verified_composition()`）。四種模式**必須**帶、check ⛔ 不接受；使用者⛔ 不得自帶（兩側的注入清單都加上）。shell 端另外先把兩份 patch 各讀一次到私有目錄，SHA 與 `git apply` 都用副本（⛔ 不讓 shell 內部的兩次讀取也留縫）。argv fixture 的四種模式同步補上。測試：pytest 的 `test_toctou_*`（四種入口各一；B 本身一致、**不帶交接值時 Python 單獨擋不下**當對照組）、shell 的「合成守門之後、Docker 之前換檔」（fake docker 在 `run` 的第一步換掉 tooling patch，斷言交出去的仍是 A 的值）；另在 scratchpad 的 clone 複本用**真的 Docker** 重現同一情境：rc=1、⛔ 沒有 archive、⛔ 沒有 staging 殘骸 |
| 中 2 | ⛔ **`bounded_diagnostics` 的輸出有界、計算過程卻無界**：`CounterfactualEffectCheck` 保留所有違規列，最後才排序截成 20 筆——「大量列同時違規」時記憶體隨違規列數成長，與串流記憶體模型不符；299／314 MiB 的實測只涵蓋成功路徑 | ✅ 改成兩個完整計數器 ＋ 各自只保留排序最前面 20 筆的 `BoundedSample`（插入後立即丟掉最大的；以 `(key, 序號)` 排序，⛔ 不拿 sample dict 比大小）。測試：1,000 筆**反向**輸入（每筆新來的都比 buffer 裡的小），兩種原因各一，斷言 buffer 任何時刻 ≤ 20、sample 是最小的 20 筆、完整計數＝1,000；另以打亂順序的 500 筆對照完整排序 |

反向驗證：拿掉 finalize 的交接比對（`test_toctou_finalize_*` 變紅）、把 buffer 改回無界（四支變紅）、shell 不送
`--verified-*`（四種模式的 argv 比對與兩支交接測試變紅），全部還原後重跑通過。

**歸檔**（✅ review 通過）：契約寫進 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「I-074 Stage 2 的正式證據契約」；
操作程序（CLI matrix、合成守門、check 的順序、failed record 的重跑資格、三者一起保存、記憶體）寫進
[`development-workflow.md`](./development-workflow.md)「I-074 Stage 2 的正式證據與 failed record 程序」。

⚠️ **待辦**：
* ~~review ③d（含上表的差異 2～7）~~——✅ 2026-09-24 review 通過並 commit；下一步回到 Stage 2 計畫書的步驟 ④（sizing harness），⑦ 之前⛔ 不得正式執行 Stage 2；
* [I-118](#i-118replay-相關腳本會洩漏-git-worktree註冊與-tmp-目錄都會累積) 的既有洩漏仍在：本輪反覆跑常態 shell 測試，真正的 repo 的 worktree 註冊由 33 累積到 89；每一輪完整 `test.sh` 都量得 **＋4**（例如 85 → 89），與 I-118 記錄的每輪增量相同——③d 沒有加重（它的測試全在隔離 repo 裡跑，並已斷言那邊歸零）。⛔ 本輪沒有清理，留待 I-118 處理或由使用者決定。

#### Stage 2 步驟 ④：sizing harness 計畫書 v7（2026-09-24，✅ **已確認**）

⚠️ 定位：Stage 2 計畫書「八、執行順序」的 **④ 實作獨立 sizing harness（⛔ 不含正式 preflight）**，對應「二之二」
磁碟時序的第 ① 步。**⑤ 才正式實測並記錄 `P_B`、裁定 `M_safety`**；⛔ 在 ⑤ 定案之前⛔ 不得實作 preflight。

**⚠️ 實作前發現的差異（2026-09-24，✅ **已確認**——正式驗收以測試 c2 與 ④ 的可用性執行為準）**：

| # | 計畫寫的 | 實際 | 改法 |
|---|---|---|---|
| 差異 1 | 「三、資料流」：fixture 以 `publish_artifacts()` 寫出見證路徑的 after'（13,417 列）與成功路徑的 before source（13,417 列） | ⛔ **做不到**：`publish_artifacts()` → `write_canonical_atomic()` → `canonical_json_chunks()` 要的是**完整的物件**，13,417 列 rows 必須整份常駐（D+1 整份載入實測約 449 MiB）。**證據**（2026-09-24，Stage 2 image、唯讀 rootfs、mem-guard 下修為 **417m**，具名容器事後 inspect）：`State.OOMKilled = true`、`ExitCode = 137`，連讀完 rows、印出第一行都沒做到。⚠️ 訂正：同日稍早記的「371m、rc=1」⛔ **不是證據**——那個 rc 是管線最後的 `grep` 的結束碼。mem-guard 的上限隨 host 可用記憶體浮動（本輪看過 371m～557m），⛔ 不能靠運氣 | 兩個**全量**檔改由 fixture 專用的**逐列串流 writer** 寫出（契約見下表）；**小檔**（cohort'、comparison、report、`bounded_diagnostics.json`）照舊用 `publish_artifacts()`，且**指標檔最後發布**。正式 artifact 的 schema、canonical bytes 與 validator ⛔ 都不改 |

**fixture 串流 writer 的契約**（⚠️ 磁碟語意與 `write_canonical_atomic()` 逐項相同）：

| 項目 | 契約 |
|---|---|
| 寫入序列 | 同目錄 temp `.{name}.{12 hex}.tmp` → 64 KiB 分塊寫入 → `flush` ＋ `fsync` → `os.replace` → fsync 目錄 |
| 內容 | canonical bytes：頂層以 `canonical_json_bytes()` 編碼、在 `"rows":[]` 的位置插入逐列的 `canonical_json_bytes(row)`（以 `,` 分隔）；⚠️ 斷言 `"rows":[]` 在頂層編碼裡**恰好出現一次** |
| 輸入 | rows 是**一次性的 iterator**（⛔ 不得 `list(rows)`）；⚠️ 常駐上限 **3 列**（佇列 1 ＋ 讀取端手上 1 ＋ 寫出端正在編碼的 1——④ 第一輪 review 訂正，原寫「只常駐一列」）；成功路徑另留 156 列 cohort 供 comparison |
| SHA | 寫入時**增量**計算 SHA-256 並回傳（與 `write_canonical_atomic()` 相同） |
| 失敗：`os.replace` **之前** | ⚠️ 任一列編碼失敗（含非有限數值）、iterator 中途拋錯、`write`、`flush` 或 **temp 的** `fsync` 失敗 → **`finally` 刪除 temp**，⛔ 不得出現正式檔（與 `write_canonical_atomic()` 相同） |
| 失敗：`os.replace` **之後** | ⚠️ **目錄的** `fsync` 失敗時 temp 已經被 rename——**正式檔可能已存在**、只是 durability 未確認（⛔ 此時已無 temp 可刪，⛔ 不宣稱 `finally` 能處理；與 `write_canonical_atomic()` 的語意相同）。fixture 以非零結束碼中止、⛔ **不得發布後續的指標檔**（cohort'／report），harness 因結束碼不符而 **fail-closed** |
| 發布順序 | witness：先寫 after'（⚠️ 必須**完整成功**、含目錄 fsync），**比對回傳的 SHA ＝ cohort' 的 `after_artifact_sha256`**，一致才以 `publish_artifacts()` 發布 cohort'（指標檔）——⛔ 不只依賴稍後的 `--envcheck` 才發現不一致；success：before source → comparison → report（report 是指標檔，最後；前一步任何失敗都⛔ 不寫後面的）；failure：只有 `bounded_diagnostics.json` |

⚠️ **對 `P_B` 的影響：無**——temp 的完整大小、64 KiB buffer、rename 時序與目錄操作都不變；改變的只有 fixture 程序
自己的常駐記憶體（它本來就⛔ 不列入程序峰值）。

**v7 修訂表**（v6 的 review，2026-09-24；✅ 2026-09-24 確認）：

| # | v6 的問題 | v7 的改法 | 改到哪 |
|---|---|---|---|
| 中 | ⛔ **規格矛盾**：索引記的是完整的 `docker run` argv、twin 用 `docker create`，而正規化只允許替換 `--cidfile`／`--name` 的值——兩者必然分別含 `run` 與 `create`，SHA 不可能相同，**每個 twin 都會 fail-closed** | 改成對 **container spec** 取 hash：`container_spec_argv` ＝ 排除 docker 執行檔與 operation（`run`／`create`）之後的 options、image、容器指令與參數；`--cidfile`／`--name` 的值仍換成佔位字。測試：同一份 spec 的 run 與 create hash 相等；除 operation、`--cidfile`、`--name` 以外，任一 option、image、指令或參數改變都讓 hash 不同 | 二、六 |
| 補 1 | 在允許位置內建檔會改到**父目錄**的 mtime，父目錄若不在 allowlist 會被誤判 | 每條路徑記 inventory **之前**，先建好 L1～L5 的根目錄與必要的父目錄；「允許位置」明訂為**根目錄本身及其下所有項目**——允許位置以外的目錄（例如 `<work>`、`<work>/runs`）只有在它**直接底下**的項目增減時 mtime 才會變，那正是要抓的 | 二、三 |
| 補 2 | 容器「生命週期不重疊」用 wall clock 證明，校時會造成誤判 | 改用 **lifecycle event sequence**：shim 在 S 以 `flock` 保護的單調計數器記錄每個容器的 `create_begin` 與 `rm_done` 事件（另附 `CLOCK_MONOTONIC` 時間戳，只作輔助）；同一路徑內前一個容器的 `rm_done` 事件號 < 下一個的 `create_begin` 才算不重疊；⚠️ **任一事件缺失或無法證明順序 → 一律相加** | 二、六 |

**v6 修訂表**（v5 的 review，2026-09-24；⚠️ argv 正規化與不重疊的證明已由 v7 再改寫）：

| # | v5 的問題 | v6 的改法 | 改到哪 |
|---|---|---|---|
| 中 1 | 宣稱 `measurement_overhead_bytes = 0`，但 shim 加的 `--name`、`--cidfile`、`--read-only`、`/peak` mount 與 memory wrapper 的 argv **會寫進 Docker Root Dir 的 metadata**——L0 上仍有儀器造成的開銷 | 分成兩項、分開記錄：**`host_measurement_artifact_bytes = 0`**（harness 的 host 端量測檔全在 S）；**`docker_instrumentation_overhead = "included_unseparated"`**（儀器造成的 metadata 增量**保守地含在**容器 metadata 的採用值裡，⛔ 不從 `P_B` 扣除） | 二 |
| 中 2 | twin 延後到窗口後建，但沒定義每一筆 invocation 回填到哪一條 `P_path`、同一路徑多個容器怎麼合併、缺漏／重複／亂序怎麼拒絕；完整 argv 裡的 `--cidfile` 也不能原樣重播 | 每筆 invocation 在**執行前**寫一份不可變的索引（`sequence`、`phase`、`role`、`included_in_disk_path`、正規化 argv 的 SHA-256、sidecar 路徑）；各路徑記錄起訖 sequence；步驟 4 一律 `phase = "memory_only"`、`included_in_disk_path = false`；每個預期的 sequence **恰好**一份索引、一份 command sidecar、一份 twin 結果，缺失／重複／未知 phase／SHA 不一致一律 fail-closed。同一路徑內的容器足跡：生命週期**不重疊**（前一個已刪除才建下一個，由 sidecar 的時間戳證明）取最大，否則相加。twin 只替換 `--cidfile` 與 `--name` 的值（**等長、唯一、事前⛔ 不存在**），其餘 token 逐字不變，正規化規則寫死（見「二」） | 二、三、六 |
| 中 3 | fail-closed 時宣稱「保留原始量測檔」，但原始量測全在 tmpfs 的 S，正常流程到最後才複製；中途失敗若 EXIT trap 直接清 S，診斷證據就消失 | 明訂失敗時的清理順序：停取樣 → 以 CID 強制移除容器 → 窗口標 `aborted`（⛔ 不再計算 `P_path`）→ 複製 S 到 `<work>/raw-failed/` → 寫**不宣稱 `P_B`** 的 `failure_summary.json`（失敗階段、原指令結束碼）→ 複製成功才清 S；複製也失敗就**保留 S** 並把位置印到 stderr | 三、六 |
| 補 | 自我檢查只看「新建／修改」，⛔ 沒看刪除與型別改變；以結束時的 `git status` 為準也不夠 | 改成**完整 inventory 比較**（路徑、型別、allocated bytes、mtime、inode）：baseline 已存在的項目被**刪除或改變型別**——在任何位置都 fail-closed（會壓低 L0 的 delta）；新建／修改只允許在 L1、L2、L3、L5 | 二 |

**v5 修訂表**（v4 的 review，2026-09-24；⚠️ 「`measurement_overhead_bytes` 的設計值是 0」已由 v6 分成兩項）：

| # | v4 的問題 | v5 的改法 | 改到哪 |
|---|---|---|---|
| 中高 1 | host 端的 identity validator 與宣告值小工具經 `exec_module()` 載入整組模組，**會在複本的 source tree 寫 `__pycache__`**（`PYTHONDONTWRITEBYTECODE=1` 只設在 Docker 參數）——不在任何量測位置、也不在已知寫入清單 | ⚠️ 2026-09-24 實查確認：③d 端到端用的 clone 已多出 `python/scripts/__pycache__/` 與 `replay_bundle/__pycache__/`。**裁決：禁止寫入**——harness 以 `PYTHONDONTWRITEBYTECODE=1` 的 host 環境呼叫所有正式入口，全部模式跑完後斷言複本 source tree ⛔ 沒有新的 `__pycache__`／`.pyc`；⚠️ ⑦ 的正式 orchestrator 必須沿用，否則 ⑩ 會多寫、觸發「五」的回退 | 二、三、五、六 |
| 中 2 | harness 自己的寫入（cidfile、sidecar、取樣原始值、`MemAvailable` 取樣、twin 的 cidfile）沒定義納入或排除；twin 還在 baseline 之後建——`fs_peak` 混進 harness 開銷、`accounted` 又沒涵蓋 | harness 的狀態**全部**放 host tmpfs `/dev/shm/i074-sizing-<run id>/`（S；實查：tmpfs、1.0 GB、可寫），⛔ 不落 L0；metadata twin 移到**所有量測窗口結束後**才依 shim 記下的完整 argv 重建；報告與原始量測最後才寫 `<work>`。`measurement_overhead_bytes` 的設計值是 0，窗口內 L0 上的 harness 寫入一律視為缺陷：每條路徑結束後**自我檢查**，不過即 fail-closed | 二、三、六 |
| 中 3 | 取樣用 block 用量，會計卻混用「檔案大小」「實際大小」與內容長度 | ⚠️ **一律 allocated bytes**（`du --block-size=1` 或 `st_blocks × 512`），目錄 blocks 一併計入；operational 輸出與正式 archive 量**整棵 tree**，⛔ 不逐檔 `st_size` 相加；內容長度只用於讀不到的 docker log 與 metadata，且以 block 上捨；`probe_no_clobber()` 改為步驟 0 **實建同形狀**量測；會 rename 的目錄每個加 1 block。新增測試 d0（1 byte 多檔、多層空目錄、非整 block、temp → rename、index 與 `index.lock` 並存），並斷言 `st_size` 相加的算法會低估 | 二、六 |

**v4 修訂表**（v3 的 review，2026-09-24；⚠️ metadata twin 的時點與已知寫入清單的單位已由 v5 再改寫）：

| # | v3 的問題 | v4 的改法 | 改到哪 |
|---|---|---|---|
| 高 1 | 可寫的容器 `/tmp`（bind mount）仍可能「兩次取樣之間寫入再刪除」，會計表只寫了「容器 `/tmp` 的峰值」卻⛔ 沒有可證明的來源 | ⚠️ **⛔ 不提供可寫的 `/tmp`**：容器的 root filesystem 與 `/tmp` 一律唯讀，只有正式模式本來就需要的 bind mount 可寫（L2 的 `i074_stage2/`、fixture 的 run 目錄、只寫一個數字的 `/peak`）。2026-09-24 實查：完全唯讀下 `config`、`stage2_archive`、`envcheck`、sklearn、pandas、joblib 全部 import 成功，而 `tempfile.gettempdir()` **直接拋 `FileNotFoundError`**——任何需要暫存磁碟的路徑都會**大聲失敗**，所以「全部正式模式成功」就證明不需要；`replay_bundle/` 與 `config.py` 也沒有任何 tempfile 用法。可寫的 bind mount 內的寫入改由**程式碼盤點的已知寫入清單**逐項給上界（見「二」） | 二、三、六 |
| 中 2 | 用 `sleep` 容器量 metadata 證明不了正式容器的上界（argv、mounts、環境、名稱都不同） | 對**每一次**正式 invocation，shim 以**完全相同**的 options、mounts、image、環境與指令 `docker create` 三個 twin（⛔ 不執行）量 L0 的已用量差；採用值 ＝ max(三次原始值取 64 KiB 上捨, 2 × `docker inspect` JSON 長度取 4 KiB 上捨) ＋ 64 KiB（啟動時才建立的 `hosts`／`hostname`／`resolv.conf` 等）；原始值與採用值都記錄 | 二、六 |
| 中 3 | json-file 的 log 上界沒有守前提：driver 可能不是 json-file、可能有 rotation | 每個容器結束後讀它**實際的** `HostConfig.LogConfig`（並記錄 daemon 的預設）：⚠️ 必須是 `json-file` 且⛔ 沒有 `max-size`／`max-file`，否則 fail-closed（2026-09-24 實查：daemon 預設與容器實際都是 `json-file`、`Config` 為空）。上界公式涵蓋 stdout／stderr 分流、無結尾換行、JSON 跳脫（逐 byte 最壞 6 倍）與 16 KiB 的長行分段 | 二、六 |
| 低 4 | ⑦ 若發現實際磁碟峰值超過 `P_B`，回退順序沒寫死 | 明訂：⑦ 的實際流程峰值 > ⑤ 裁定的 `P_B` → ⛔ 不得進入 ⑩；更新 `P_B`／`M_safety`、重跑 ⑤、回到 ⑥ 確認（⑥ 時併入 Stage 2 計畫書「六、1」） | 五、七 |
| 補 | ⚠️ v3 **自己漏列**的兩類寫入（實作前自查） | ① git 在複本 `.git` 裡的寫入（每個 worktree 的 admin 目錄與 index、`write-tree` 的物件，實測約 128 KiB；更新 index 時的 `index.lock` 暫存）——新增量測位置 **L5**；② `probe_no_clobber()` 在 L2 的暫態寫入（同一時間最多 3 個目錄 ＋ 3 個 marker 檔） | 二 |

**v3 修訂表**（v2 的 review，2026-09-24；⚠️ 高 1 的「可寫 `/tmp`」與容器 metadata 的量法已由 v4 再改寫）：

| # | v2 的問題 | v3 的改法 | 改到哪 |
|---|---|---|---|
| 高 1 | `SizeRw` 是容器**結束時**的值，⛔ 不是 writable layer 的峰值——程序若在 layer 寫入後刪除，結束後的 inspect 與 0.2 秒取樣都可能漏掉，會計法因此稱不上上界 | 改成**可證明的封閉寫入模型**：shim 加 `--read-only`，容器的 `/tmp` bind mount 到 L3 底下的**每容器目錄**（⛔ 不用 tmpfs），`/peak`、Stage 2 archive、run 目錄維持既有 bind mount——**全部模式在這個條件下跑完，就證明沒有未列管的 layer 寫入**（2026-09-24 實查：Stage 2 image 在 `--read-only` 下 import 全部模組成功、寫 `/` 得到 EROFS）。`SizeRw` 降級為**終止值／交叉檢查**（期望 0） | 二、三、六 |
| 高 2 | 同檔案系統守門只驗 L1～L3 與 L0，⛔ 沒驗 Docker Root Dir——data root 可以被 daemon 設定搬到別的 mount，那時 L0 的 `statvfs("/")` 捕捉不到 Docker 用量 | 執行時由 daemon 取得 `DockerRootDir`（⛔ 不寫死 `/var/lib/docker`），列為 **L4**：canonical path ＋ `st_dev`。**與 L0 不同檔案系統 → fail-closed**（單一 `required = P_B ＋ M_safety` 只對一個檔案系統有意義；兩個檔案系統要改 preflight 設計，屬計畫變更）。2026-09-24 實查：`/var/lib/docker`、同一個 device；非 root 可以 `stat`／`statvfs` | 二、六 |
| 中 3 | `P_failure ≤ P_success` 被當成硬守門、⛔ 不產報告——但 `P_B` 已取三者最大，單一公式⛔ 不依賴它；硬擋反而丟掉最需要的反例數據 | 降為**預期假設／診斷**：不成立時照樣產出帶完整數字的 canonical 報告，`status = "assumption_violated"`，⛔ 不得進入 ⑤ 的正式裁定、回頭 review 計畫；`P_B` 仍取三者最大 | 二、六 |
| 中 4 | 資料流漏了兩個正式 CLI 的必填參數 | check 用**步驟 3 凍結的那一份** counterfactual patch；recover 用步驟 3 **實際發布**的 record 目錄——由「發布前後 `failed/` 的集合差恰好一筆」取得，並必須等於發布 CLI 輸出的 `published`，⛔ 不得用「排序第一個」之類的方式猜 | 三、六 |
| 中 5 | shim 沒有釘 I/O 透明（`--check-failed-record` 直接把 `docker run` 的 stdout 當 JSON 解析）；中斷清理只寫「EXIT trap 移除具名容器」，容器還在跑時一般 `docker rm` 會失敗 | shim **自己⛔ 不向 stdout／stderr 寫任何 byte**，計量結果只寫 sidecar 檔；`docker logs` 只導進計數、⛔ 不重新輸出；以 `--cidfile` 記錄本次實際建立的容器 ID，清理只對已記錄的 CID `docker rm -f`；inspect／logs／rm 任一步失敗 → sidecar 標「量測失敗」並保存原指令結束碼，report fail-closed | 三、五、六 |

**v2 修訂表**（v1 的 review，2026-09-24；⚠️ 高 1 與中 4 已由 v3 再改寫）：

| # | v1 的問題 | v2 的改法 | 改到哪 |
|---|---|---|---|
| 高 1 | `P_B` 定義為同一檔案系統的全部新增用量，卻**先假定**容器 writable layer 與 log 只有 KB 而排除——`required = P_B ＋ M_safety` 就不是真正的上界 | 兩層都納入：① **根檔案系統已用量差**（`statvfs`）當 catch-all 取樣；② 每個容器的 writable layer（`docker inspect --size` 的 `SizeRw`）與 json-file log（⚠️ 非 root **讀不到** log 檔，2026-09-24 實查——改由 `docker logs` 的內容算**上界**）明列進會計上界。「只有 KB」只能是報告的結論，⛔ 不是前提 | 二、三 |
| 高 2 | 上游的 `P_B` 還涵蓋見證路徑（after'／cohort'、`envcheck/`），preflight 也要在見證趟之前做一次；v1 只量 replay → finalize | 公式明訂 **`P_B = max(P_witness, P_success, P_failure)`**，三條路徑用**同一套**量法實測；見證路徑的 operational 輸出取自已封存的真正 after'／cohort' | 二、三 |
| 高 3 | 取樣的是**絕對** `du`——複本的 `python/baselines/i074_stage2/` 已有 `envcheck/` 等既有內容，會被算進 `P_B` | 每條路徑開始前對每個**互不重疊**的量測位置記錄 `baseline_bytes`；`delta(t) = Σ max(current(t) − baseline, 0)`、`sample_peak = max(delta(t))`；報告保留每個位置的 baseline、peak、delta 與 device；會計法的組成路徑也明定互不重疊 | 二 |
| 中 4 | failure 路徑只宣稱「必然較小」 | 納入 `max()`；另加斷言 **`P_failure ≤ P_success`**（「二之二」只留一條公式的前提），⛔ 不成立即 fail-closed、要回頭改計畫；報告自我檢查每條 `P_path ≤ P_B` | 二、六 |
| 中 5 | wrapper 只寫 cgroup v1 | 與 `run-replay-offline.sh` 相同的 **v1 → v2 fallback**（`memory.max_usage_in_bytes` → `memory.peak`）；兩者皆缺、值為 0 → fail-closed；保留原結束碼——計畫、shim、測試同步釘住 | 三、六 |
| 中 6 | 只警告 dirty worktree，無法辨識實際用了哪一版 harness；work 目錄防護只做字串判斷 | 分兩種模式：**④ 可用性驗證**允許未 commit，報告記錄 harness／shim／fixture 的 SHA-256；**⑤ 正式量測**（`--formal`）要求相關 tracked 檔案 clean，記錄 source commit OID、image identity 與各腳本 SHA-256。work 目錄以 **canonical path** 判斷，補「parent symlink 指回 repo」的拒絕測試 | 三、六 |

##### 一、目標與⛔ 不做的範圍

**目標**：一支可重複執行、在釘死的 Stage 2 image 與 mem-guard cgroup 內跑的 harness，量出：

1. **`P_B = max(P_witness, P_success, P_failure)`**：三條路徑各自從 replay 開始、到發布完成為止，**新增**的磁碟用量峰值
   （repo、run 目錄、`/tmp` 與 Docker Root Dir 必須在同一個檔案系統，執行時驗證）；
2. 階段二（證據層）**每一個程序**的記憶體峰值，供 ⑤ 對照 < 450 MiB（「六、1」的 d～i）。

| ⛔ 不做 | 理由 |
|---|---|
| ⛔ 不實作正式 preflight（可用空間檢查、`M_safety` 常數） | 「二之二」：`M_safety` 未定案前寫進程式就是把占位符寫進程式 |
| ⛔ 不裁定 `M_safety` | 屬 ⑤ |
| ⛔ 不量 replay 程序本身的**記憶體**（「六、1」的 a～c） | 那條路徑（`evaluation.py` 的串流 loader 與 `--i074-counterfactual`）是 ⑦ 才實作；⑦ 的 memory harness 量它（⚠️ Stage 2 計畫書 v29：正式驗收在 ⑨-1），並再驗「實際流程的磁碟峰值⛔ 沒有超出 sizing harness 的結果」（回退規則見「五」）。⚠️ replay 的**磁碟**足跡（worktree ＋ operational 輸出）照樣算進 `P_B` |
| ⛔ 不改任何正式入口 | `finalize-stage2-evidence.sh`、`run-replay-offline.sh`、`replay_bundle/*`、`evaluation.py` 一行不動——harness 只從外面呼叫它們 |
| ⛔ 不支援 Docker Root Dir 在另一個檔案系統、⛔ 不支援 json-file 以外的 log driver 或 log rotation | 遇到即 fail-closed（單一公式只對一個檔案系統、一個已定義的 log 模型成立） |
| ⛔ 不碰真正的 `python/baselines/` 與 live | 全部在 repo 外的 `git clone --no-hardlinks` 複本裡跑（見「三」；⚠️ 差異 2：原寫 `--shared`） |

##### 二、`P_B` 的定義與量法

**三條路徑**（每條各自量，`P_B` 取最大）：

| 路徑 | 從 → 到 | 磁碟上新增的東西 |
|---|---|---|
| **witness** | 見證趟的 replay 開始 → `--envcheck` 發布完成 | replay worktree（`e1cbbbd`、⛔ 無 patch）、after'／cohort' 的 operational 輸出（含寫入 temp）、finalize 的程式碼 worktree、`envcheck/` 的 staging（→ rename 成正式，同一份 bytes）、git 寫入、容器足跡 |
| **success** | before 正式趟的 replay 開始 → `--finalize` 發布完成 | replay worktree（`e1cbbbd` ＋ 兩份 patch）、before source／comparison／report（含 temp）、凍結 patch、finalize 的程式碼 worktree、合成守門的 worktree 與 patch 快照、`evidence/` 的 staging、git 寫入、容器足跡 |
| **failure** | before 正式趟的 replay 開始（回 6）→ `--publish-failed-record` 發布完成 | replay worktree、`bounded_diagnostics.json`、凍結 patch、finalize 的兩種 worktree 與快照、`failed/<…>/` 的 staging、git 寫入、容器足跡 |

⚠️ replay worktree：現行 runner 以 `exec docker run` 結束，worktree **不會被清掉**（I-118）——每條路徑都以最壞情況計，
**一直存在到該路徑結束**。⚠️ 見證路徑的 operational 輸出取自**已封存的真正 after'／cohort'**（③c 的產物），⛔ 不用估計值。

**量測位置**（⚠️ **互不重疊**；每條路徑開始前各記一次 `baseline` 與 `st_dev`）：

| 位置 | 內容 |
|---|---|
| L0 根檔案系統 | `statvfs` 的已用量——⚠️ **catch-all**（Docker 的 log 與容器 metadata、任何沒列到的寫入） |
| L1 run 根目錄 | 三條路徑各自的 run 目錄（`<work>/runs/{witness,success,failure}`） |
| L2 複本的 `python/baselines/i074_stage2/` | `envcheck/`、`evidence/`、`failed/`、它們的 staging 與 `probe_no_clobber()` 的暫態目錄 |
| L3 harness 的 `TMPDIR` | replay worktree、正式腳本 `mktemp -d` 建的所有 worktree 與快照（⚠️ v5：`/peak` 已移到 S） |
| L4 Docker Root Dir | 執行時由 `docker info` 取得、取 canonical path；⚠️ 只驗 `st_dev` 必須等於 L0（非 root 讀不了它的內容，用量由 L0 涵蓋） |
| L5 複本的 `.git` | 各 worktree 的 admin 目錄（index、HEAD、logs）、`git apply --index` 與 `git write-tree` 寫的物件、`index.lock` |
| ⚠️ S harness 的狀態根目錄（v5） | **host 的 tmpfs** `/dev/shm/i074-sizing-<run id>/`（2026-09-24 實查：tmpfs、1.0 GB、可寫）——cidfile、shim 的 sidecar、每容器的 `/peak`、取樣的 rolling 狀態與原始值、`MemAvailable` 取樣、harness 自己的 console log、baseline marker。⚠️ **⛔ 不在 L0**：harness 的量測開銷⛔ 不混進 `P_B`（見下方「harness 開銷」） |

**封閉寫入模型**（v4，高 1）：shim 讓每個容器以 **`--read-only`** 執行，**⛔ 不提供可寫的 `/tmp`**；唯一可寫的是
正式模式本來就有的 bind mount——L2（check 模式連它也唯讀）、fixture 容器的 run 目錄（L1）、以及 `/peak`（v5：在 S
的 tmpfs，只寫一個數字，⛔ 不佔磁碟）。`/dev/shm` 是 Docker 預設的 tmpfs，屬**記憶體**（計入 cgroup 峰值），⛔ 不佔磁碟。**正式模式全部在這個
條件下跑完，就證明程序沒有其他暫存磁碟需求**（需要的話 `tempfile` 會直接失敗，見 v4 修訂表高 1）；`SizeRw` 仍記錄，
定位為**終止值／交叉檢查**（期望 0，非 0 即 fail-closed）。

**host 端的 Python**（v5，中高 1）：正式腳本在 host 直接執行 identity validator 與合成守門的宣告值小工具，兩者經
`exec_module()` 載入整組 `replay_bundle` 模組——⚠️ ③d 端到端用的 clone 實查已經多出 `python/scripts/__pycache__/` 與
`replay_bundle/__pycache__/`（`PYTHONDONTWRITEBYTECODE=1` 只設在 Docker 參數裡）。這些位置⛔ 不在任何量測位置內。
**裁決：禁止寫入**——harness 以 `PYTHONDONTWRITEBYTECODE=1` 的 host 環境呼叫所有正式入口（它自己的 host Python 也一樣），
並在全部模式跑完後斷言複本的 source tree ⛔ 沒有任何新的 `__pycache__`／`.pyc`。⚠️ ⑦ 的正式 orchestrator 必須沿用
同一個環境；⛔ 沒有沿用的話 ⑩ 會多寫，觸發「五」的回退。

**harness 開銷**（v5，中 2）：harness 自己的寫入**全部**放在 S（tmpfs），⛔ 不落在 L0；量測窗口內⛔ 不寫 `<work>`；
metadata twin 移到**所有量測窗口結束之後**才建（見「三」的步驟 5）；canonical 報告與要保存的原始量測在最後才複製到
`<work>`。開銷分兩項記錄（v6）：

| 報告欄位 | 值 | 意思 |
|---|---|---|
| `host_measurement_artifact_bytes` | **0**（設計值） | harness 的 host 端量測檔（cidfile、sidecar、取樣、log）全在 S；量測窗口內落在 L0 上的 harness 寫入一律是缺陷——每條路徑結束後自我檢查，⛔ 不默默混進 `P_B` |
| `docker_instrumentation_overhead` | **`"included_unseparated"`** | shim 加的 `--name`、`--cidfile`、`--read-only`、`/peak` mount 與 memory wrapper 的 argv 會寫進 Docker Root Dir 的 metadata——⚠️ 它們**保守地含在**容器 metadata 的採用值裡（twin 用的就是改寫後的 argv），⛔ 不分離、⛔ 不從 `P_B` 扣除 |

**自我檢查**（v6 改為完整 inventory 比較）：每條路徑開始前，⚠️ **先建好 L1～L5 的根目錄與必要的父目錄**（v7：
否則在允許位置內建第一個檔案，就會改到不在 allowlist 的父目錄的 mtime），再對複本整棵 tree（含 `.git`）、`<work>` 與
真正的 repo（含 `.git`）各記一份 inventory（路徑、型別、allocated bytes、mtime、inode）；路徑結束後再記一次比對。
⚠️ 「允許位置」明訂為**根目錄本身及其下所有項目**：

| 變化 | 允許嗎 |
|---|---|
| 新建、修改 | 只允許在 L1、L2、L3、L5 |
| baseline 已存在的項目被**刪除**或**改變型別** | ⛔ 任何位置都不允許——刪除會壓低 L0 的 delta，讓 `P_path` 被低估而不被指出 |
| 複本 source tree 出現 `__pycache__`／`.pyc` | ⛔ 不允許（見上方「host 端的 Python」） |

⚠️ 量測期間請勿操作真正的 repo——它的 inventory 有任何變化都會 fail-closed（寧可重量，⛔ 不猜是誰改的）。

**會計的單位**（v5，中 3）：⚠️ **所有落在檔案系統上的組成一律用 allocated bytes**——`du -sx --block-size=1`
（或 `st_blocks × 512`），**目錄本身與所有子目錄的 blocks 一併計入**；operational 輸出與正式 archive 直接量**整棵 tree**，
⛔ 不逐檔 `st_size` 相加（1 byte 的檔案至少占一個 block、目錄也占 block、sparse／壓縮檔的 apparent size 與 allocated size
不同）。**內容長度只用在還沒落盤、讀不到的東西**（docker log、容器 metadata）的保守推導，且推導結果以 block 上捨。

**已知寫入清單**（程式碼盤點；會計上界的每一項；⚠️ 單位全部是 allocated bytes）：

| 寫入 | 位置 | 上界的來源 |
|---|---|---|
| replay worktree 的 checkout | L3 | 步驟 0 實建同一個 worktree（`e1cbbbd` ＋ 兩份 patch）量整棵 tree |
| operational 輸出（全量檔：串流 writer；小檔：`publish_artifacts()`——兩者都是同目錄 temp → `os.replace`，見差異 1） | L1 | 發布完成後量整棵 run 目錄 tree ＋ **每個目錄 1 個 block**（temp 的目錄項暫時多一筆；⛔ 不依賴「目錄不縮」這種檔案系統特性）。⚠️ 依序寫、temp 就地變成正式檔，⛔ 不會與自己的正式檔並存 |
| 凍結 patch、`bounded_diagnostics.json` | L1 | 同上（在同一棵 run 目錄 tree 內） |
| ⚠️ **⑦d 細部計畫 v1（✅ 2026-10-02 確認）**：runner 的凍結副本（runner 以 `exec docker run` 結束、EXIT trap ⛔ 不執行） | L3 | 步驟 0 實建同形狀的目錄（`runner_frozen_patches`；④ 的 fixture 取代了 runner，三種量法都沒看到它） |
| finalize 的程式碼 worktree（HEAD） | L3 | 步驟 0 實建同一個 worktree 量整棵 tree |
| 合成守門的 worktree（`e1cbbbd` ＋ patch）與 patch 快照 | L3 | 同上；快照目錄以實建同形狀的目錄量 |
| archive 的 staging（含 before source 的串流副本）→ 正式 archive | L2 | 發布完成後量**整棵**正式 archive tree（含 manifest 與各層目錄）＋ 每個目錄 1 個 block（rename，⛔ 不複製） |
| `probe_no_clobber()` | L2 | 步驟 0 在 L2 的檔案系統上**實建同形狀**的結構（3 個目錄 ＋ 3 個內容 1～3 bytes 的 marker 檔）量 allocated bytes，每次發布計一份 |
| git 的寫入（每個同時存在的 worktree） | L5 | 步驟 0 實建時量到的 `.git` 增量 ＋ **一份 index 的 allocated bytes**（`index.lock` 與 index 同時存在的那一刻） |
| 容器足跡 | L4（＝ L0） | 見下表 |

⚠️ `/peak` 在 S（tmpfs），⛔ 不佔磁碟，v5 起不再列入。

**容器足跡**：

| 成分 | 量法 | 為什麼是上界 |
|---|---|---|
| writable layer | `--read-only` ⇒ 恆為 0；`SizeRw` 交叉檢查 | 封閉寫入模型 |
| json-file log | ⚠️ 先驗容器實際的 `HostConfig.LogConfig`：`Type` 必須是 `json-file`、`Config` ⛔ 不得有 `max-size`／`max-file`（否則 fail-closed）。上界：`docker logs` 的 stdout 與 stderr **分流**導進計數（⛔ 不重新輸出），逐行（含無結尾換行的最後一段）切成 ≤ 16 KiB 的片段，每段 `6 × 長度 ＋ 128` bytes（JSON 逐 byte 跳脫的最壞倍數 ＋ `log`／`stream`／`time` 欄位與換行的固定開銷），總和取 4 KiB 上捨 | log 由 daemon **只增不減**地寫到容器刪除為止（⛔ 沒有 rotation），結束時的大小就是峰值 |
| 容器 metadata（`config.v2.json`、`hostconfig.json`、`hosts` 等） | shim 在每一次正式 invocation 把**改寫後的完整 argv** 記到 S 的索引；⚠️ **所有量測窗口結束後**（v5），harness 以**完全相同**的 options、mounts、image、環境與指令 `docker create` 三個 twin（⛔ 不執行；`--cidfile`／`--name` 的值依下方正規化規則替換成等長、唯一的值），量每次建立前後 L0 的已用量差後 `docker rm`（等 removal 完成）；**採用值 ＝ max(三次原始值的最大值取 64 KiB 上捨, 2 × `docker inspect` JSON 長度取 4 KiB 上捨) ＋ 64 KiB**（啟動時才建立的 `hosts`／`hostname`／`resolv.conf` 與目錄項）；原始值與採用值都寫進 sidecar | metadata 是容器的組態本身——同一組參數的大小固定、存活期間不變；L0 的干擾只會墊高原始值，`inspect` 長度的下限擋住被其他程序的刪除壓低 |

**三種量法，取大者**：

| 量法 | 定義 |
|---|---|
| 目錄取樣 | 每 0.2 秒同步量 L1、L2、L3、L5（`du -sx --block-size=1`，⚠️ **block 用量**，⛔ 不是 apparent size）：`delta_dirs(t) = Σ max(current(t) − baseline, 0)`；`dirs_peak = max(delta_dirs(t))` |
| 檔案系統取樣 | 同一個取樣點量 L0：`delta_fs(t) = used(t) − used_baseline`；`fs_peak = max(delta_fs(t))`（其他程序的寫入只會**墊高**它；刪除則可能壓低，所以⛔ 不單獨使用） |
| 會計上界 | 上面「已知寫入清單」逐項相加（組成路徑互不重疊）——⚠️ 這是 `P_path` 的**上界來源**；兩種取樣是交叉檢查與 catch-all |

**invocation 索引**（v6，中 2；v7 改為 container spec 的 hash）：shim 在每一次 `docker run` **執行前**，以 exclusive
create 寫一份不可變的 `<S>/index/<sequence>.json`：

```json
{"sequence": 7, "phase": "success", "role": "finalizer", "included_in_disk_path": true,
 "container_spec_sha256": "…", "argv": ["…"], "sidecar_path": "<S>/containers/o0070.json"}
```

| 規則 | 內容 |
|---|---|
| `phase`／`role` 的來源 | harness 在呼叫前以環境變數交給 shim（⛔ shim 不自己推論）；`phase` ∈ {`witness`, `success`, `failure`, `memory_only`}、`role` ∈ {`fixture`, `finalizer`, `recovery`, `check`}——封閉列舉 |
| 各路徑的範圍 | harness 在每條路徑開始與結束時記錄 sequence 的起訖；預期的 invocation 寫死：witness ＝ fixture ＋ `--envcheck`、success ＝ fixture ＋ `--finalize`、failure ＝ fixture ＋ `--publish-failed-record`、memory_only ＝ 步驟 4 的四個 |
| memory-only | 步驟 4 一律 `phase = "memory_only"`、`included_in_disk_path = false`，⛔ 不得計入任何 `P_path`；`included_in_disk_path` 必須與 `phase` 相符 |
| 完整性 | 每個預期的 sequence **恰好**一份索引、一份 command sidecar（`<S>/containers/<ID>.json`）、一份 twin 結果；缺失、重複、未知 phase／role、sequence 不連續、索引與 sidecar 或 twin 的 `container_spec_sha256` 不一致 → fail-closed |
| 同一路徑內的合併（v7） | shim 在 S 以 `flock` 保護的單調計數器記錄 **lifecycle event**：每個容器的 `create_begin`（`docker run` 之前）與 `rm_done`（`docker rm` 回傳之後）各取一個事件號，另附 `CLOCK_MONOTONIC` 時間戳（⚠️ 只作輔助，⛔ 不作判定）。同一路徑內依 sequence 排列，**前一個的 `rm_done` 事件號 < 下一個的 `create_begin`** 才算不重疊 → 容器足跡取**最大**；⚠️ **任一事件缺失、事件號重複或無法證明順序 → 一律相加** |
| container spec 的 hash（v7） | `container_spec_argv` ＝ 從 docker 指令列**排除 docker 執行檔與 operation token**（`run`／`create`）後剩下的 options、image、容器指令與參數；其中 `--cidfile` 與 `--name` 的**值**換成固定的 `<CIDFILE>`、`<NAME>`，其餘 token 逐字保留，以 NUL 串接後取 SHA-256（索引欄位名 `container_spec_sha256`）。twin 以 `docker create <同一份 spec>` 建立，它的 spec hash 必須與索引的相同。⚠️ 正式參數裡若出現 `docker create` 不接受的 option（run 專屬），twin 建不起來 → fail-closed、回頭改計畫（現行正式入口只用 `--network`／`--user`／`--cpus`／`--memory`／`--memory-swap`／`--pids-limit`／`-e`／`-v`／`-w`，`--rm` 由 shim 拿掉） |
| twin 的 cidfile／name | 容器 ID 形如 `<kind><sequence 三位><k>`（正式 `o…0`、twin `t…1`～`t…3`），名稱 `i074sz-<run id>-<ID>`、cidfile `<S>/cid/<ID>.cid`——**與正式那一份等長**、各自唯一；建立前斷言 cidfile ⛔ 不存在（Docker 遇到既有的 cidfile 會失敗），twin 刪除後一併刪 cidfile |

`P_path = max(dirs_peak, fs_peak, accounted)`；`P_B = max(P_witness, P_success, P_failure)`。

**判定**：

| 情況 | 結果 |
|---|---|
| 任一位置、組成、容器足跡或程序峰值量不到（⛔ 不得寫成 0）；shim sidecar 標「量測失敗」；`SizeRw ≠ 0`；log driver 不是 `json-file` 或有 rotation；L1～L5 任一與 L0 的 `st_dev` 不同、或 S ⛔ 不是 tmpfs；任一正式入口的結束碼不符預期；`--read-only` 下任一模式失敗；⚠️ **自我檢查**不過（v6：完整 inventory 比較，見「二」）；⚠️ invocation 索引不完整或不一致（見「二」）；harness 的 stdout／stderr 是 L0 上的一般檔案（`--formal` 時） | ⛔ **fail-closed**：⛔ 不產報告；原始量測依「三」的失敗清理順序保存到 `<work>/raw-failed/`（或保留 S） |
| `P_failure ≤ P_success` 不成立（⚠️ 只是**預期假設**，⛔ 不是單一公式的前提） | **照樣產出**完整報告，`status = "assumption_violated"`；⛔ 不得進入 ⑤ 的正式裁定，回頭 review 計畫；`P_B` 仍取三者最大 |
| 其餘 | `status = "ok"`；報告自我檢查每條 `P_path ≤ P_B` |

##### 三、資料流

```text id="i074_stage2_sizing_flow_001"
0. 準備   模式：④ 可用性驗證（預設）或 ⑤ 正式量測（--formal：相關 tracked 檔案必須 clean）
          建 S＝/dev/shm/i074-sizing-<run id>/（必須是 tmpfs）；harness 的所有狀態與 console log 都寫在 S
          work 目錄以 canonical path 判斷，⛔ 不得在 repo 內（含 parent symlink 指回 repo）、⛔ 不得已存在
          git clone --no-hardlinks <repo> <work>/repo（HEAD；⚠️ 差異 2：原寫 --shared）；REPLAY_IMAGE_ID 必須是 Stage 2 identity 的 image
          取得 DockerRootDir（L4）並驗 st_dev；記錄 daemon 的預設 logging driver
          把複本已封存的 envcheck/ 複製到 <work>/cache/（不在量測位置內），再從複本移除 envcheck/
          ——見證路徑要從「還沒有 envcheck/」的狀態開始
          實建兩種 worktree（HEAD；e1cbbbd ＋ 兩份 patch ＋ write-tree）量整棵 tree 與 .git 增量（allocated），量完移除
          在 L2 的檔案系統上實建 probe_no_clobber() 同形狀的結構量 allocated bytes，量完移除
          建好 <work>/tmp（L3）與 <work>/runs；⚠️ 之後所有 host 端指令都帶 PYTHONDONTWRITEBYTECODE=1
1. witness  建好本路徑的 L1 根（runs/witness/）→ 記 baseline（各位置、inventory ＋ S 內的 marker）→ 在 TMPDIR 建 replay worktree（e1cbbbd）→ fixture 容器（唯讀
            rootfs）把 cache 的 after' 以**串流 writer**（差異 1）寫進 runs/witness/witness/，回傳的 SHA 必須等於
            cohort' 的 after_artifact_sha256，一致才以 publish_artifacts() 發布 cohort'（指標檔）→ 正式 --envcheck
            （期望 0）→ 記 P_witness → 自我檢查
2. success  建好 runs/success/ → 記 baseline → replay worktree（e1cbbbd ＋ git apply --index 兩份 patch）→ fixture 容器以已錨定的 D+1 合成
            13,417 列 before source（156 列候選把 RR 加回去；**串流 writer**，差異 1），再以 publish_artifacts()
            寫 comparison、report（report 是指標檔，最後）到 runs/success/stage2/ → 凍結 patch → 正式 --finalize（期望 0）→ 記 P_success → 自我檢查
3. failure  建好 runs/failure/ → 記 baseline 與 failed/ 的項目集合 → replay worktree（同上）→ fixture 容器寫 bounded_diagnostics.json
            （156 列 rr_not_restored）→ 凍結 patch（runs/failure/patches/）→ 正式 --publish-failed-record（期望 1）
            → 記 P_failure → 自我檢查；RECORD ＝ 發布後 failed/ 的集合差（⚠️ 必須恰好一筆），且必須等於 CLI 輸出的 "published"
4. 其餘程序（只量記憶體與耗時；⛔ 不在任何 P_path 的窗口內）：
            --recover-envcheck（期望 0；E3a 全量重比＝「環境等價比對程序」）
            --recover-durability（期望 0）
            --check-failed-record --counterfactual-patch runs/failure/patches/counterfactual.patch（期望 2：命中 RECORD）
            --recover-failed-record RECORD（期望 1）
5. metadata twin：對 S 裡記下的每一個正式 invocation 的完整 argv 各建三個 twin 量 metadata（v5：移出量測窗口）
6. 報告     全部量測結束後才寫 <work>/sizing_report.json（canonical）＋ sizing_report.txt，並把 S 的原始量測複製到
            <work>/raw/；⛔ 不寫進 repo（⑤ 轉錄到本筆）；最後清掉 S
```

**中途失敗或被中斷時的清理順序**（v6，中 3；⚠️ 全部發生在窗口終止之後，⛔ 不污染 `P_B`）：

| 序 | 動作 |
|---|---|
| 1 | 停止 sampler |
| 2 | 以 **CID** 強制移除本次的所有容器（含 twin） |
| 3 | 把進行中的量測窗口標成 `aborted`——⛔ 不再計算任何 `P_path` |
| 4 | 把 S 複製到 `<work>/raw-failed/` |
| 5 | 寫 `<work>/failure_summary.json`：失敗的階段、原指令的結束碼、已完成的路徑——⚠️ **⛔ 不宣稱 `P_B`** |
| 6 | 複製成功才清 S；⚠️ 複製失敗就**保留 S**，並把它的位置印到 stderr |

⚠️ 所有正式入口都以 `TMPDIR=<work>/tmp PYTHONDONTWRITEBYTECODE=1 PATH=<shim>:$PATH <work>/repo/scripts/finalize-stage2-evidence.sh …` 執行——
路徑常數由腳本位置推導，從複本執行就只會寫複本。fixture 容器同樣經 shim（唯讀 rootfs），但它只是模擬 replay 的寫檔，
⛔ 它的記憶體⛔ 不列入程序峰值。

**docker shim**（正式腳本⛔ 不改；只改寫 `docker run`，其餘子指令原樣 exec 真正的 docker）：

| 項目 | 契約 |
|---|---|
| 改寫 | 在 image 前加 `--cidfile <S>/cid/<ID>.cid`、`--name i074sz-<run id>-<ID>`（`<ID>` 的格式見「二」的 invocation 索引）、`--read-only`、`-v <S 內的 peak 目錄>:/peak`，拿掉 `--rm`（⛔ **不**加任何可寫的 `/tmp`）；容器指令包進與 `run-replay-offline.sh` 的 `MEASURE_PEAK` **同一套** wrapper：指令結束後、退出前讀 cgroup 峰值——**v1 `memory.max_usage_in_bytes` → v2 `memory.peak`**——並保留原結束碼；原本的 option 與容器指令**逐 token 不變**。⚠️ **⑦d 細部計畫 v1（✅ 2026-10-02 確認）**：`SIZING_PROFILE=acceptance` ⛔ 不加 `--read-only`、role `replay` 換成 launcher（profile 未設定時行為不變） |
| 記錄 | **執行前**以 exclusive create 寫不可變的 invocation 索引（含改寫後的完整 argv 與正規化 SHA，見「二」）——metadata twin 由 harness 在**所有量測窗口結束後**依它重建；⚠️ shim ⛔ 不在窗口內建 twin |
| ⚠️ I/O 透明 | shim **自己⛔ 不向 stdout／stderr 寫任何 byte**——容器的 stdout／stderr 原樣直通（`--check-failed-record` 要把 stdout 當 JSON 解析）；計量結果只寫 sidecar 檔（`<S>/containers/<ID>.json`，tmpfs）；`docker logs` 只導進計數程序，⛔ 不重新輸出 |
| 結束之後 | 讀 `SizeRw`、實際的 `LogConfig`、log 上界、cgroup 峰值 → 寫 sidecar → `docker rm`；**傳回原指令的結束碼** |
| 失敗 | inspect、logs、rm 任一步失敗（twin 的 create／rm 失敗由 harness 在步驟 5 記錄）→ sidecar 標「量測失敗」並保存原結束碼（shim 仍傳回原結束碼），report fail-closed |
| 中斷 | shim 與 harness 都 trap `INT`／`TERM`／`EXIT`：對 **`--cidfile` 記錄到的 CID**（含 twin）`docker rm -f`（⛔ 不以名稱猜；容器還在跑也移除得掉） |

host 的 `MemAvailable` 低點另外每 2 秒取樣。⚠️ cgroup 峰值讀不到或為 0 → fail-closed。⚠️ 這個數字含 page cache
（與 Stage 1 probe 的量法一致，⛔ 不另換指標）。

⚠️ **「六、1」的 h（preflight）**：正式 preflight 入口屬 ⑦；本 harness 以 `--check-failed-record` 的 Python 段
代量（它同樣載入兩個信任錨並掃 `failed/`），報告裡明確標示「代量」。

**報告的來源紀錄**：

| 模式 | 要求 | 記錄 |
|---|---|---|
| ④ 可用性驗證（預設） | 允許未 commit | 複本的 HEAD OID、工作樹是否 dirty，以及**實際執行的** harness、shim、fixture 與正式入口的 SHA-256 |
| ⑤ 正式量測（`--formal`） | `scripts/`、`python/`、`.gitattributes` 的 tracked 檔案⛔ 不得有未 commit 變更（`git status --porcelain`）；harness 自己的檔案必須已進版控，且 SHA-256 等於 HEAD 的 blob | 上欄全部 ＋ source commit OID、image ID 與 Stage 2 identity 檔的 SHA-256、mem-guard 上限、DockerRootDir、logging driver |

##### 四、受影響檔案

| 檔案 | 改動 |
|---|---|
| `scripts/i074-stage2-sizing.sh`（**新增**） | 編排：模式、S（tmpfs）與 work 目錄防護、複本、L4／logging 檢查、兩種 worktree 與 probe 結構的實建量測、TMPDIR／PATH／`PYTHONDONTWRITEBYTECODE` 導向、取樣、呼叫正式入口、record 目錄的集合差、自我檢查、窗口後的 metadata twin、CID 清理、收集結果；⛔ 不含可用空間檢查 |
| `scripts/lib/i074-sizing-docker-shim.sh`（**新增**） | docker shim（見上表） |
| `python/scripts/i074_stage2_sizing.py`（**新增**） | `fixture`（容器內：見證路徑的 after'／cohort'、成功路徑的 operational 輸出、失敗路徑的中繼檔）、`logbound`（host：json-file log 上界，stdlib）、`allocated`（host：整棵 tree 的 allocated bytes，含目錄）與 `report`（host：三種量法、判定、canonical 報告） |
| `scripts/test-replay-args.sh` | shim 的改寫、I/O 透明、wrapper、metadata twin、log 模型守門、中斷清理；harness 的模式與 work 目錄防護 |
| `python/backtest/modular/sr_scoring/tests/test_i074_stage2_sizing.py`（**新增**） | fixture 產出通過**正式**的 envcheck／finalize／publish-failed-record；log 上界；report 的公式與判定 |
| `docs/development-workflow.md` | harness 的操作程序 |
| ⛔ `finalize-stage2-evidence.sh`、`run-replay-offline.sh`、`replay_bundle/*`、`evaluation.py` | ⛔ 一行不改 |

##### 五、風險與回滾

| 風險 | 對策 |
|---|---|
| 取樣漏掉短暫峰值 | 封閉寫入模型：唯讀 rootfs、⛔ 無可寫 `/tmp`，可寫的 bind mount 內只有「已知寫入清單」的寫入，逐項給上界；兩種取樣是交叉檢查與 catch-all |
| 其他程序的寫入干擾 L0 | 只會墊高（保守）；報告並列三種量法，差距過大時標示，⑤ 判讀 |
| ⚠️ harness 自己的寫入混進 `P_B`（v5） | 全部放 S（tmpfs）；twin 移出窗口；報告最後才寫；每條路徑結束後自我檢查 |
| ⚠️ host 端 Python 寫 bytecode（v5） | `PYTHONDONTWRITEBYTECODE=1` ＋ 自我檢查；⑦ 的 orchestrator 必須沿用（⛔ 沒沿用就走下一列之後的回退） |
| ⚠️ apparent size 低估實際占用（v5） | 會計一律 allocated bytes、量整棵 tree（含目錄）；推導值以 block 上捨 |
| 合成的 before source 與 ⑦ 實際產出的尺寸不同 | 反事實只改 156 列的少數欄位，量級相同；報告並列 D+1 after 的大小對照。見證路徑用的是真正的 after'，⛔ 沒有這個落差 |
| `--read-only` 與 ⑩ 的實際執行條件不同 | ⚠️ 只**縮小**可寫範圍——⑩（不加 `--read-only`）若寫到別處，就是 harness 沒量到的用量 |
| ⚠️ **⑦ 的實際流程峰值超過 ⑤ 裁定的 `P_B`**（v4，低 4；✅ 已於 ⑥ 併入 Stage 2 計畫書「六、1」，比較對象改成預算 `P_B_BUDGET`） | **寫死的回退順序**：⛔ 不得進入 ⑩ → ~~更新 `P_B`／`M_safety`~~ ⚠️ **v29：更新 `P_B_BUDGET`（`M_safety` 原則上維持）** → 重跑 ⑤ → 回到 ⑥ 確認。⚠️ ⑦ 的 memory harness 再驗實際磁碟峰值時（⚠️ v29：正式驗收在 ⑨-1），要一併確認上一列的 `--read-only` 落差 |
| 誤寫到真正的證據目錄 | 正式腳本的路徑常數由腳本位置推導——從複本執行只會寫複本；canonical path 的 work 目錄防護；測試斷言 |
| shim 拿掉 `--rm` 後中途被中斷、容器殘留 | `--cidfile` 記錄的 CID（含 twin）一律 `docker rm -f`（shim 與 harness 兩層 trap） |
| shim 汙染 stdout | shim 本身⛔ 不寫 stdout／stderr；測試逐 byte 比對 |
| host 只有 2 GiB | 程序依序執行、各自經 mem-guard；harness 本身只跑 `du`、`statvfs` 與 shell |
| **回滾** | 全部是新檔（＋ shell 測試段落），單一 `git revert` |

##### 六、測試與驗證

| # | 案例 |
|---|---|
| a | shim 的改寫：`docker run OPTS IMAGE CMD…` → `OPTS'（加 --cidfile、--name、--read-only、-v /peak，⛔ --rm、⛔ 可寫 /tmp）IMAGE sh -c <wrapper> _ CMD…`，OPTS 與 CMD 逐 token 不變；非 `run` 原樣傳遞；容器的結束碼（0、非 0）原樣傳出 |
| a2 | wrapper：cgroup v1 存在 → 讀 v1；v1 缺、v2 存在 → 讀 v2；兩者皆缺 → peak 檔空 → report fail-closed；值為 0 → fail-closed；原指令的結束碼保留（⚠️ 以可覆寫的 cgroup 根目錄在 host 上測，並斷言候選路徑與 `run-replay-offline.sh` 的相同） |
| a3 | ⚠️ **I/O 透明**：經 shim 的 stdout 與未包裝的 `docker run` **逐 byte 相同**；應用程式的 stderr ⛔ 不被重複；在隔離 repo 經 shim 跑 `--check-failed-record`，JSON 能被正式 shell 段解析且決策正確 |
| a4 | 足跡與失敗：sidecar 記錄 `SizeRw`、實際 `LogConfig`、log 上界、cgroup 峰值與原結束碼（metadata 的原始值與採用值由步驟 5 記錄）；inspect／logs／rm 任一步失敗 → sidecar 標「量測失敗」、shim 仍傳回原結束碼、report fail-closed；twin 的 create／rm 失敗 → report fail-closed；`SizeRw ≠ 0` → report fail-closed |
| a5 | ⚠️ **中斷**：容器仍在執行時對 harness 送 `SIGINT`／`SIGTERM` → cidfile 記錄的容器被 `docker rm -f`（fake docker 斷言呼叫與 CID；④ 的驗證執行再以真的 Docker 做一次，斷言 `docker ps -a` 沒有本次 run id 的容器） |
| a6 | ⚠️ **封閉寫入**（v4，高 1）：經 shim 的容器寫 `/tmp` 或 `/` 都失敗（EROFS）、`tempfile.gettempdir()` 失敗——證明⛔ 沒有可寫的暫存位置 |
| a7 | ⚠️ **metadata**（v4，中 2）：以真的 Docker 建長 argv（≥ 32 KiB）與長 mount path 的 invocation → 採用值 ≥ 三次原始值的最大值、且 ≥ 2 × `inspect` 長度（採用值隨 argv 變大）；一般長度的 invocation 同樣成立 |
| a8 | ⚠️ **log 模型**（v4，中 3）：`LogConfig.Type` 不是 `json-file` → fail-closed；有 `max-size` 或 `max-file` → fail-closed；上界 ≥ 以 json-file 格式實際編碼的長度（參考編碼器）——涵蓋 stdout 與 stderr、`"`、`\`、控制字元、非 ASCII、無結尾換行、> 16 KiB 的單行 |
| a9 | ⚠️ **bytecode**（v5，中高 1）：在全新的 clone 經 harness 跑完全部模式 → 複本 source tree ⛔ 沒有任何新的 `__pycache__`／`.pyc`；harness 呼叫正式入口的環境帶 `PYTHONDONTWRITEBYTECODE=1`（fake 入口斷言） |
| a10 | ⚠️ **harness 開銷**（v5，中 2）：cidfile、sidecar、`/peak`、取樣狀態、console log 的路徑全部在 S 之下且 S 是 tmpfs；量測窗口內 shim ⛔ 不建 twin；`--formal` 時 stdout 被導到 L0 上的一般檔案 → 拒絕；報告的 `host_measurement_artifact_bytes = 0`、`docker_instrumentation_overhead = "included_unseparated"` |
| a11 | ⚠️ **invocation 索引**（v6，中 2）：順序打亂、缺 sidecar、重複 sidecar、重複索引、未知 phase／role、memory-only 被標成 `included_in_disk_path = true`、索引與 sidecar 的 SHA 不一致、twin 的 cidfile 已存在 → 全部 fail-closed；twin 的 cidfile／name 與正式的等長 |
| a11b | ⚠️ **container spec hash**（v7）：同一份 spec 的 `docker run` 與 `docker create` 的 hash **相等**；只改 operation、`--cidfile` 或 `--name` 的值 → 相等；改任一 option、option 的值、image、容器指令或任一參數（含順序）→ **不同** |
| a11c | ⚠️ **不重疊的證明**（v7）：前一個的 `rm_done` 事件號 < 下一個的 `create_begin` → 取最大；事件號交錯、缺一個事件、事件號重複 → 相加；`CLOCK_MONOTONIC` 時間戳與事件號矛盾時仍以事件號為準（時間戳⛔ 不作判定） |
| a12 | ⚠️ **中止時的原始量測**（v6，中 3）：在 witness 路徑中、正式入口失敗、metadata twin 失敗、以及 `SIGTERM` 四個中止點——`<work>/raw-failed/` 與 `failure_summary.json` 都存在（⛔ 不含 `P_B`）、本次容器全部被移除、S 已清；模擬複製失敗 → S 被保留且位置印在 stderr |
| a13 | ⚠️ **inventory**（v6，補）：baseline 已存在的檔案被刪除、目錄換成檔案（型別改變）、允許位置以外新建或修改 → fail-closed；允許位置內的新建與修改（含第一次在允許的根目錄內建檔）→ 通過；⚠️ v7：允許根目錄**事先建好**時，它的父目錄 mtime ⛔ 不變 |
| b | harness 拒絕：work 目錄在 repo 內、**parent symlink 指回 repo**、已存在；沒有 `REPLAY_IMAGE_ID`；`--formal` 時相關 tracked 檔案 dirty、或 harness 檔案未進版控／SHA 不等於 HEAD |
| b2 | L4：Docker Root Dir 與 L0 同一個 device → 通過；不同 device → fail-closed（以注入的 `st_dev` 測 report 的判定；harness 以 daemon 回報的路徑取值、⛔ 不寫死） |
| b3 | record 目錄：發布前後 `failed/` 的集合差恰好一筆且等於 CLI 的 `published` → 通過；零筆、兩筆、或與 `published` 不符 → fail-closed |
| c | fixture：在測試用的小型信任錨上合成 → **正式**的 `publish_envcheck()`（見證）、`finalize_stage2_evidence()`（成功）、`publish_failed_record()`（失敗）都通過（證明合成的輸入是合法的 operational 輸出）；⚠️ 串流 writer 寫出的全量檔與 `canonical_json_bytes()` 對同一份內容的輸出**逐位元相同** |
| c2 | ⚠️ **串流 writer**（差異 1）：空列、單列、多列，以及 Unicode、跳脫字元（`"`、`\\`、控制字元）與巢狀值 → 與 `canonical_json_bytes()` 逐位元相同；rows 以**一次性 generator** 餵入——⚠️ 因為有 64 KiB buffer，用**總量遠超過 64 KiB** 的 generator，在它**尚未耗盡時**斷言 temp 已經成長（小型 rows 在 buffer flush 前維持 0 bytes 是正常行為，⛔ 不以它判定），實作若偷偷 `list(rows)` 就會在耗盡前看不到任何寫入；非有限數值、iterator 中途拋錯 → ⛔ 沒有正式檔、⛔ 沒有殘留 temp；⚠️ **故障注入**：`os.replace` 之前的 `write`／temp `fsync` 失敗 → ⛔ 沒有正式檔、⛔ 沒有 temp；`os.replace` 之後的**目錄** `fsync` 失敗 → fixture 非零結束、cohort'／report ⛔ 未發布（正式資料檔可以存在）；回傳的 SHA ＝ 正式檔的實際 SHA ＝ cohort' 引用的 SHA，⛔ 不一致時 cohort' ⛔ 不發布 |
| d0 | ⚠️ **allocated bytes**（v5，中 3）：以真的檔案系統建出下列形狀，逐步量實際 allocated 的峰值，斷言會計值 ≥ 峰值：多個 1 byte 的檔案、多層空目錄、非整 block 的大小、temp → rename 的依序寫入、index 與 `index.lock` 同時存在；⚠️ 並斷言以 `st_size` 相加的算法會**低估**（證明測試有鑑別力） |
| d | report：`delta` 以各位置的 baseline 相減、逐位置 clamp 為 0；三種量法取大；`P_B` 取三條路徑的最大；已知寫入清單逐項相加（含 probe、git、容器足跡；⛔ 不含在 S 的 `/peak`）；任一位置／組成／足跡／程序峰值缺失或為 0、`st_dev` 不一致 → fail-closed、⛔ 不產報告；`P_failure > P_success` → **照樣產出**、`status = "assumption_violated"` |
| e | 驗證執行（④）：在 host 以真實資料完整跑一次，**全部在唯讀 rootfs、⛔ 無可寫 `/tmp`、host 端⛔ 不寫 bytecode** 下完成、自我檢查通過、所有程序結束碼符合預期（envcheck 0、finalize 0、publish 1、recover-envcheck 0、recovery 0、check 2、recover-failed 1）；另做一次 a5 的真 Docker 中斷演練；數字只當「harness 可用」的證據，**正式紀錄與 `M_safety` 裁定在 ⑤（`--formal`）** |

##### 七、完成後的歸檔

操作程序寫進 [`development-workflow.md`](./development-workflow.md)（I-074 Stage 2 各節之後）；
實作與驗證結果寫在本筆「④ 實作結果」；`P_B`、各程序峰值與 `M_safety` 的裁定在 ⑤ 寫進本筆與「二之二」；
⚠️ 「五」的 ⑦ 回退順序在 ⑥ 併入 Stage 2 計畫書「六、1」（✅ v29）。

#### Stage 2 步驟 ④ 實作結果（2026-09-24，✅ **review 通過**——2026-09-29，四輪——並 commit）

✅ **依計畫書 v7 ＋ 差異 1 實作 sizing harness**。⛔ 本輪沒有跑任何正式 replay、沒有動 `python/baselines/`、沒有 commit；
所有實跑都在 repo 外的 `~/i074_stage2_sizing/` 底下。

| 檔案 | 內容 |
|---|---|
| `scripts/i074-stage2-sizing.sh`（**新增**） | 編排：模式（④ 可用性驗證／⑤ `--formal`）、work 目錄防護（canonical path）、S（tmpfs）、`git clone --no-hardlinks` 複本、L4 與 logging 檢查、步驟 0 的實建量測、三條路徑（baseline → 取樣 → fixture → 正式入口 → 自我檢查）、memory-only、窗口後的 metadata twin、報告；失敗清理依計畫的六步；每個步驟以 `setsid` 背景執行再 `wait`，中斷時對整個 process group 收尾並等它真的結束 |
| `scripts/lib/i074-sizing-docker-shim.sh`（**新增**） | docker shim：只改寫 `docker run`（`--cidfile`／`--name`／`--read-only`／`/peak`、⛔ `--rm`、⛔ 可寫 `/tmp`、與 runner 同一套 v1 → v2 的 peak wrapper），執行前寫不可變索引、`create_begin`／`rm_done` 事件，結束後讀 `SizeRw`、`LogConfig`、log 上界、cgroup 峰值寫 sidecar；自己⛔ 不向 stdout／stderr 寫任何 byte；中斷時以 CID `docker rm -f` |
| `python/scripts/i074_stage2_sizing.py`（**新增**） | 容器內的 `fixture`（串流 writer：差異 1）；host 的 allocated bytes、inventory 比較、container spec hash、索引／事件／sidecar、log 上界、metadata twin、取樣、record 目錄、報告與失敗摘要（⚠️ host 端只用標準庫） |
| `python/backtest/modular/sr_scoring/tests/test_i074_stage2_sizing.py`（**新增**） | 64 條（含第一輪 review 補的 8 條、第二輪的 3 條、第三輪的 1 條） |
| `scripts/test-replay-args.sh` | sizing 的 shim／wrapper／I/O 透明／中斷／封閉寫入／metadata twin／參數檢查兩段 |

**⚠️ 實作中發現的差異（✅ review 同意，2026-09-24）**：

| # | 計畫寫的 | 實際 | 理由 |
|---|---|---|---|
| 差異 2 | 「三、資料流」：`git clone --shared` | ⚠️ **`git clone --no-hardlinks`**（完整複製 164 MB 的物件、⛔ 不共用 inode、⛔ 沒有 alternates） | **第一次實跑就被自我檢查擋下**（witness 路徑）：真正 repo 的 `.git/objects/pack/pack-346a….pack` 的 **mtime 被改了**（大小、inode 不變，link 數 1）。原因是 git 的 freshen：`git write-tree`／`git apply --index` 寫到**已存在**的物件時會 touch 含有它的 pack，而 `--shared` 的 alternates 指向真正 repo 的物件庫。改用 `--no-hardlinks` 之後 git 的寫入全落在 L5。⚠️ ⑩ 的正式流程一樣會 freshen 真正 repo 的 pack——只改 mtime、⛔ 不增加用量 |
| 補 1 | 「六」的 a12：四個中止點的測試 | 以**真的 Docker 演練**完成（見下表）；「metadata twin 失敗」與「複製失敗」沒有自然的觸發方式，新增**演練用的故障注入** `I074_SIZING_FAULT=twins／copy`（`--formal` 一律拒絕） | 中止路徑要在真實的 clone、容器與 tmpfs 上才有意義；完整 harness 一次約 6～7 分鐘，⛔ 不適合放進常態的 `test.sh` |
| 補 2 | container spec 的正規化 | 只正規化 **image 之前**的 `--cidfile`／`--name`（容器指令裡同名的參數⛔ 不動；測試 a11b 覆蓋） | 否則容器指令改了 `--name` 參數時 hash 不會變，違反 a11b |
| 補 3 | 中斷處理 | 步驟以 `setsid` 背景執行；中斷時先以 CID 移除容器、再對整個 process group 送 `TERM`，並**等 group 內所有程序結束**才複製 S | 真 Docker 演練時發現：只 `wait` 得到 group leader，shim（孫程序）還在補寫 sidecar，S 就被複製並刪掉了 |

**測試**：pytest 60 條（c、c2、a8、d0、a13、a11、a11b、a11c、b2、b3、d，以及第一輪 review 補的 twin CID 收尾、常駐上限）；`test-replay-args.sh` 新增 sizing 兩段
（a、a2、a3、a4、a5、a6、a7、a8、b，其中 a6、a7 以真的 Docker）。

| 層 | 結果 |
|---|---|
| `python/scripts/test.sh`（完整，第一輪 review 修正後） | **1569 passed, 1 skipped**（③d 後 1509，＋60）；doc-refs 無問題；`test-replay-args.sh` 全過（293 項 ok） |
| 殘留 | 全部執行與演練之後：`docker ps -a --filter name=i074sz-` 為 0、`/dev/shm` ⛔ 沒有 `i074-sizing-*`；真正 repo 的 worktree 仍是 I-118 的每輪 ＋4（本輪 ⛔ 沒有加重） |

**反向驗證**（把回歸注回產品程式，確認測試會紅；全部還原後重跑通過）：串流 writer 偷偷 `list(rows)`、inventory
不看刪除、容器足跡一律取最大、log 上界不乘跳脫倍數（⚠️ 第一次沒被抓到——補了「每個 byte 都要跳脫」的案例後才變紅）、
容器指令裡的 `--name` 也被正規化、`os.replace` 之前失敗不刪 temp——六項都有測試變紅。

**中止點演練**（真的 Docker；每一項都斷言本次 run id 的容器歸零）：

| 中止點 | 結果 |
|---|---|
| witness 路徑內（第一次實跑，自我檢查擋下 pack 的 mtime） | `raw-failed/`、`failure_summary.json`（witness＝aborted、⛔ 無 `P_B`）、S 已清 |
| 正式入口失敗（以不符 Stage 2 identity 的 image 執行，`--envcheck` 回 1） | 同上 |
| `SIGTERM`（success 路徑的 `--finalize` 容器正在執行） | 容器被移除；o0040 的 sidecar 標「被 TERM 中斷」；summary：witness＝complete、success＝aborted |
| metadata twin（`I074_SIZING_FAULT=twins`） | 三條路徑都 complete 之後在步驟 5 中止：`raw-failed/`、summary（⛔ 無 `P_B`）、S 已清 |
| 複製失敗（`I074_SIZING_FAULT=copy`） | ⛔ 沒有 `raw-failed/`；**S 被保留**且位置印在 stderr |
| 失敗摘要寫不出來（`I074_SIZING_FAULT=summary`，review 修正後） | `raw-failed/` 已複製、摘要沒寫成 → **S 被保留**，並回報「複製（1）或失敗摘要（0）」 |

**可用性驗證執行**（2026-09-24 09:01Z，第一輪 review 修正後；HEAD `67fd060`＋工作樹的 harness，mode＝validation，
Stage 2 image，mem-guard 496m（fixture；08:33Z 那次是 524m）；⚠️ **數字只證明 harness 可用，⛔ 不是 ⑤ 的正式量測**——正式紀錄與
`M_safety` 的裁定在 ⑤ 以 `--formal` 重跑）：

| 路徑 | `P_path` | 目錄取樣峰值 | 檔案系統取樣峰值 | 會計上界 | 主要組成（會計） |
|---|---|---|---|---|---|
| witness | 132.2 MiB | 131.5 MiB | 132.2 MiB | 131.9 MiB | after'／cohort' 的 run 目錄 74.3、HEAD worktree 35.8、replay worktree 15.0、`envcheck/` 6.6、容器 0.2 |
| **success** | **150.5 MiB** | 150.0 MiB | ⚠️ 134.3 MiB | 150.5 MiB | before source 等的 run 目錄 77.8、HEAD worktree 35.8、replay 與合成守門 worktree 各 15.0、`evidence/` 6.8、容器 0.2 |
| failure | 66.1 MiB | 65.5 MiB | 65.7 MiB | 66.1 MiB | HEAD worktree 35.8、兩個 worktree 各 15.0、record 與中繼檔 < 0.2、容器 0.3 |

`P_B = 157,851,648 bytes（150.5 MiB）`，`status = "ok"`。⚠️ success 的**檔案系統取樣峰值比目錄取樣低 16 MiB**——量測期間有
其他程序在釋放空間；witness 則反過來（其他程序的寫入把它墊高到略大於會計上界）。這正是計畫規定「檔案系統取樣⛔ 不單獨
使用、三種量法取大」的理由。修正前的另一次執行（08:33Z）是 `P_B` 150.7 MiB，兩次差 0.2 MiB。
容器足跡：`SizeRw` 全為 0、log 上界 4～8 KiB、metadata 採用值 192～256 KiB（twin 原始值約 124～132 KiB）。

記憶體（cgroup 峰值，含 page cache；⚠️ 報告裡的結束碼是**容器內 Python 段**的——check 的 Python 段回 0，shell 段才依命中回 2）：

| 程序 | 峰值（09:01Z）| 峰值（08:33Z） |
|---|---|---|
| `--envcheck`（witness） | 373.2 MiB | 376.0 MiB |
| `--finalize`（success） | 332.9 MiB | 360.9 MiB |
| `--publish-failed-record`（failure） | 328.7 MiB | 335.4 MiB |
| `--recover-envcheck`（環境等價比對程序） | 305.3 MiB | 303.1 MiB |
| `--recover-durability` | 358.8 MiB | 315.6 MiB |
| `--check-failed-record` 的 Python 段（⚠️ 代量 preflight） | 300.5 MiB | 323.8 MiB |
| `--recover-failed-record` | 316.4 MiB | 332.3 MiB |
| fixture（⛔ 不列入程序峰值） | 329.2～340.0 MiB | 330.0～359.1 MiB |

host `MemAvailable` 低點 341.8 MiB（09:01Z）／374.0 MiB（08:33Z）。⚠️ cgroup 峰值含 page cache、隨 host 狀態浮動約 ±40 MiB。

##### ④ 第一輪 review 的修正（2026-09-24）

✅ **review 裁決**：差異 2（`--no-hardlinks`）與補 1～3 **同意**；差異 1 的實作正確，只需處理常駐列數的契約差異。

| # | 問題 | 修正 |
|---|---|---|
| 中高 | ⛔ **twin 的 `docker rm` 失敗時反而遺失 CID**：`run_twins()` 不論 rm 成敗都先刪 cidfile，容器還在、外層的失敗清理卻找不到 CID | ✅ 只有普通 `docker rm` **成功**才刪 cidfile；失敗時保留 cidfile（內容是實際 CID）、twin 結果記錄 `twin_containers` 與失敗原因。外層的失敗清理改由 `cleanup-cids` 對 S 裡**每一份** cidfile `docker rm -f`（helper 失敗時退回 bash 迴圈），移除成功才刪 cidfile、移除不了的 CID 回報（⛔ 不吞掉）。測試：fake docker 的 rm 失敗 → cidfile 保留 → `cleanup-cids` 呼叫 `rm -f <CID>` 並清掉 cidfile；`rm -f` 也失敗 → 回報該 CID、cidfile 保留 |
| 中 | ⛔ **生命週期事件反序仍會採用 max**：只驗相鄰容器 `前.rm_done < 後.create_begin`，⛔ 沒驗每個容器自己的 `create_begin < rm_done`（review 的反例 `o0010: create=4, rm=1／o0020: create=3, rm=5` 被判成不重疊、採用 300） | ✅ 先驗每個容器 `create_begin`、`rm_done` **各恰好一筆**且 **`create_begin < rm_done`**，再驗相鄰；事件號重複、未知事件、多一筆都走**相加**。測試補：同一容器反序（→ 400）、未知事件、多一筆 `create_begin` |
| 中 | ⛔ **inventory 的 L1 放行整個 `<work>/runs`**，改到別條路徑的 run 也會被當合法 | ✅ `end_phase()` 動態傳 `<L1>/<PHASE>`、L2、L3、L5。測試：本路徑 run 內新建通過、改到 sibling run（witness）被擋下 |
| 低 | ⛔ 失敗摘要寫不出來時仍清掉 S，並印出「摘要已存在」 | ✅ **原始量測的複製與失敗摘要都成功**才清 S；任一沒成功就保留 S 並回報兩者各自的狀態。演練用的故障注入加 `summary` |
| 低 | 差異 1 寫「只常駐一列」，實作是 `Queue(maxsize=64)` | ✅ 改成 `maxsize=1`；文件訂正為**常駐上限 3 列**（佇列 1 ＋ 讀取端手上 1 ＋ 寫出端正在編碼的 1）。測試：500 列的來源，任何時刻「已產生、未取走」≤ 2 |

反向驗證：把 twin 改回原本的寫法（rm 失敗仍刪 cidfile）、拿掉 `create_begin < rm_done`、佇列改回 64 列——各自有測試變紅；
還原後通過。⚠️ 我第一次對 twin 注入的突變沒被抓到——那是突變本身沒生效（修正後的 else 分支會補寫 cidfile），改用原本的寫法當突變才正確。

##### ④ 第二輪 review 的修正（2026-09-24）

| # | 問題 | 修正 |
|---|---|---|
| 中高 | ⛔ **`docker rm -f` 仍失敗時，外層最後照樣刪掉 S**：fallback 對所有失敗 `\|\| true`，清 S 的條件只看複製與摘要——容器殘留、S 卻沒了（只剩 `raw-failed/` 裡的 cidfile 副本） | ✅ 抽出 `cleanup_containers()`：helper 失敗時 fallback **逐一累積**移除不了的 CID（`rm -f` 失敗且 `inspect` 仍看得到才算殘留），另以名稱**偵測**本次 run id 的容器（⛔ 只偵測、⛔ 不用來移除）；中斷時先清一次、process group 結束後**再清一次（這一次才算數）**；⚠️ **`cleaned`、`copied`、`summarized` 三者都成功才清 S**；殘留的 CID 印到 stderr 並寫進 `failure_summary.json` 的 `leftover_containers`。成功路徑也在寫出任何報告**之前**確認本次的容器一個都不剩。演練用的故障注入加 `cleanup`。測試（shell）：fake docker 的 `rm` 一律失敗、`inspect` 顯示容器仍在 → S 與 cidfile 保留、摘要與 stderr 列出 CID、helper 與 fallback 都試過 `rm -f` |
| 中 | ⛔ **未知容器 ID 的 lifecycle event 被忽略**，仍採用 `max_non_overlapping`（review 以 `o9990/create_begin` 重現） | ✅ 報告層先驗**所有 event 的容器 ID 都屬於 invocation 索引**，⛔ 不認得即 fail-closed；之後才依路徑過濾事件、交給 `combine_footprints()`。測試：加一筆 `o9990` 的事件 → 報告 fail-closed |
| 文件 | `development-workflow.md` 還寫「差異 2 待確認」；計畫書正文仍寫 `git clone --shared`；`fixture_witness` 的註解仍寫「常駐只有一列」 | ✅ 全部訂正（正文註記「差異 2：原寫 `--shared`」） |

反向驗證：清 S 的條件拿掉 `cleaned`（shell 的 a12 變紅）、報告不驗未知容器 ID（pytest 變紅）；還原後通過。
修正後以真的 Docker 重做 `SIGTERM` 演練（`--finalize` 容器執行中；容器歸零、`leftover_containers` 為空、S 已清）與一次完整的
可用性驗證執行（09:31Z；`status = "ok"`、`P_B` **150.6 MiB**——witness 131.9／success 150.6／failure 66.0 MiB，與前兩次差 ≤ 0.2 MiB）。
`python/scripts/test.sh`：**1572 passed, 1 skipped**（pytest 的 sizing 測試 63 條）；`test-replay-args.sh` 全過（294 項 ok）。

##### ④ 第三輪 review 的修正（2026-09-24）

| # | 問題 | 修正 |
|---|---|---|
| 中高 | ⛔ **process group 等 20 秒後仍存活，照樣刪 S**：`stop_step_group()` 逾時就返回，清 S 的條件⛔ 不含 group 是否真的結束——仍存活的程序可能還在寫 S | ✅ `TERM` 等 20 秒、仍有成員就**升級 `KILL`** 再等 5 秒；仍有成員則回報 PGID 與成員並回傳失敗，`on_failure()` 記 `group_stopped=0`。⚠️ **清 S 的條件改成 `group_stopped`、`cleaned`、`copied`、`summarized` 四者都成功**，任一沒成功就保留 S 並回報四者各自的狀態。演練用的故障注入加 `stuck`（放一個 `trap "" TERM` 的步驟）與 `group-alive`（讓 group 在 `KILL` 之後仍被視為存活）。⚠️ **`stuck` 的測試另外抓到一個原本就有的 bug**：舊的收尾在輪詢之前先 `wait` leader，leader 忽略 `TERM` 時 `wait` 會一直卡到它自己結束（實測整整等了當時注入的 `sleep 300`；注入的步驟現改為 60 秒，退化時測試約 1 分鐘就變紅），逾時與升級 `KILL` 根本輪不到。✅ 改成**不先 `wait`**、輪詢整個 group，成員檢查**排除 zombie**（leader 在被回收之前就是 zombie，已不能寫任何東西），group 結束後才 `wait` 回收 leader；⚠️ host 的 `ps` 本身失敗 ⇒ **當作仍存活**（⛔ 查不到不能當作已結束）。測試（shell）：`stuck` → 升級 `KILL`、**30 秒內**結束、以 harness 印出的 PGID 確認 group 已結束、S 照常清掉；`group-alive` → S 保留、訊息含「步驟結束（0）」；`ps` 一律失敗 ＋ `stuck` → S 保留 |
| 中 | ⛔ **成功路徑的 `docker ps` 失敗被當成沒有殘留**：原本是 `[ -z "$(docker ps …)" ]`，`docker ps` 失敗時命令替換得到空字串、被判成「沒有容器」 | ✅ 抽出 `ensure_no_run_containers()`：`docker ps` 的結束碼非 0 就走 `on_failure`（⛔ 查不到不能當作沒有殘留）；有本次的容器也走 `on_failure`。演練用的故障注入加 `final-check`（只跑這道檢查）。測試（shell）：fake docker 的 `ps` 回 1 → 結束碼 1、`failure_summary.json` 記「docker ps 失敗」、⛔ 沒有報告；對照組 `ps` 回 0 → 通過 |
| 中低 | ⛔ **依路徑過濾事件之後，跨路徑的重複事件號就看不到了**：兩條路徑各用同一個事件號，各自的檢查都通過 | ✅ `build_report()` 在未知容器 ID 的檢查之後、**過濾之前**先驗**全域**事件號唯一（全域計數器是一個，重複＝計數器損壞），⛔ 重複即 fail-closed。測試：把另一條路徑的事件改成同一個事件號 → 報告 fail-closed |

⚠️ 修正中的一個事故：我第一版的 shell 測試用 `pkill -f "exec sleep 300"` 收尾，**以指令字串比對殺到了執行測試的 shell 本身**
（它的命令列含同一段字串），測試中斷、留下一個 `sleep 300` 與一份 S（已手動清掉；那一輪測試自己的 `/tmp/tmp.*` 暫存目錄因權限沒有刪，留待手動清理）。✅ 測試改成**只依 PGID**：harness
在 stderr 印出 process group，測試以 `pgrep -g` 確認、以 `kill -- -<PGID>` 收尾；a5 原本的 `pkill -f "sleep 30"`
同樣改成 `setsid` 後對該 group 收尾。⛔ 測試裡不再有 `pkill -f`／`pgrep -f`。

反向驗證（把回歸注回產品程式，確認會紅；全部還原後以雜湊比對確認與修正版相同）：清 S 的條件拿掉 `group_stopped`
（`group-alive` → S 被清掉）、`docker ps` 不檢查結束碼（`final-check` ＋ `ps` 回 1 → 結束碼 0）、`ps` 失敗當作 group 已結束
（`stuck` ＋ `ps` 失敗 → S 被清掉）、收尾前先 `wait` leader（`stuck` → 61 秒、⛔ 沒有升級 `KILL`）、`build_report()` 拿掉全域事件號
檢查（pytest 的跨路徑重複事件號那一條變紅）——五項都會被測試抓到。
修正後以真的 Docker 重做 `SIGTERM` 演練（10:22Z，`--finalize` 容器執行中；收尾 2 秒、⛔ 沒有升級 `KILL`、容器歸零、`leftover_containers` 為空、S 已清，o0040 的 sidecar 標「被 TERM 中斷」——孫程序的補寫仍在複製 S 之前完成）與一次完整的可用性驗證執行（10:22Z；`status = "ok"`、`P_B` **150.5 MiB**——witness 131.9／success 150.5／failure 66.0 MiB，與前幾次差 ≤ 0.2 MiB；成功路徑結尾以真的 `docker ps` 確認沒有殘留）。
`python/scripts/test.sh`：**1573 passed, 1 skipped**（pytest 的 sizing 測試 64 條）；`test-replay-args.sh` 全過（299 項 ok，
第三輪 ＋5）。worktree 仍是 I-118 的既有模式（完整 `test.sh` 一輪 ＋8，其中 `test-replay-args.sh` 佔 ＋4），本輪⛔ 沒有加重。

##### ④ 第四輪 review 的修正（2026-09-29）

| # | 問題 | 修正 |
|---|---|---|
| 中高 | ⛔ **leader 自行退出後，同 group 的殘留子程序沒有人收尾**：`step()` 在 `wait "$STEP_PID"` 之後立刻清空 `STEP_PID`——結束碼不符時 `on_failure()` 看不到 PGID、⛔ 不呼叫 `stop_step_group()`；結束碼相符時直接往下走。殘留程序可能還在寫 S，失敗流程卻照樣複製並刪掉 S（review 以隔離的 process group 重現：`leader=3 rc=7 remaining=4:sleep`） | ✅ 抽出 `run_in_group()`（`step()` 改用它）：`wait` 之後**保留 `STEP_PID`**；結束碼不符 → 交給 `on_failure()` 以同一個 PGID 收尾；結束碼相符但 group **仍有非 zombie 成員** → 也視為失敗（階段記「leader 已結束（結束碼 N）但 process group P 仍有成員」）並收尾；確認 group 已空之後才清空 `STEP_PID`。`stop_step_group()` 一開始就印出 PGID 與當下的成員。演練用的故障注入加 `orphan-ok`／`orphan-bad`（leader 以 0／7 退出、背景 `sleep 60` 留在 group 裡）。測試（shell）：兩種都 → 結束碼 1、收尾時列得出殘留成員、事後 group 已空、`failure_summary.json` 的階段與結束碼正確 |

反向驗證：`wait` 之後立刻清空 `STEP_PID`（原本的寫法；`orphan-bad` → ⛔ 沒有收尾、殘留程序仍存活、S 照樣被清）、結束碼相符時不檢查
group（`orphan-ok` → 結束碼 0）——兩項都會被測試抓到；還原後以雜湊比對確認與修正版相同。
修正後以真的 Docker 跑一次完整的可用性驗證（2026-09-29 01:45Z；每個真實步驟結束時 group 都已空、⛔ 沒有誤判；`status = "ok"`、`P_B` **150.6 MiB**——witness 131.9／success 150.6／failure 66.1 MiB）與 `SIGTERM` 演練（01:52Z，`--finalize` 容器執行中；收尾時 group 有 4 個成員、2 秒內全部結束，容器歸零、S 已清，o0040 的 sidecar 標「被 TERM 中斷」）。
`python/scripts/test.sh`：**1573 passed, 1 skipped**；`test-replay-args.sh` 全過（301 項 ok，第四輪 ＋2）；doc-refs 無問題。

**歸檔**（✅ 已完成）：操作程序寫進 [`development-workflow.md`](./development-workflow.md)「I-074 Stage 2 的 sizing
harness（步驟 ④）」。

✅ review ④（含差異 2 與補 1～3）已於 2026-09-29 通過並 commit。⑤ 的正式量測與 `M_safety` 的裁定見「Stage 2 步驟 ⑤」。

#### Stage 2 步驟 ⑤：`--formal` 正式量測與 `M_safety` 的裁定（2026-09-29 完成，✅ **review 通過**並 commit）

**事前寫死的規則**（✅ 2026-09-29 使用者裁定；⚠️ 在看到正式數字**之前**定案——計畫書只寫「`M_safety` 為寫死的固定 bytes、
據 `P_B` 裁定」，沒有公式，也沒寫 ⑤ 能不能重跑）：

| 項目 | 規則 |
|---|---|
| **`M_safety`** | **固定 1 GiB（1,073,741,824 bytes）**，⛔ 不隨 `P_B` 縮放、⛔ 不得執行期解讀。理由：⑩ 約 180 分鐘期間，同一個檔案系統上的 live 服務（postgres、backend、worker、docker log；DockerRootDir 也在 `/`）會持續寫入，另有 `--read-only` 落差、合成 before source 與實際產出的尺寸差——這幾項都⛔ 不在 `P_B` 裡；④ 的 `P_B` 約 151 MiB，1 GiB 約 6.8 倍。裁定當時 `/` 可用 6,787 MiB（約 6.6 GiB；檔案系統 40,188 MiB、使用率 83%）——⚠️ 這是**人工執行 `df -B1M --output=target,size,used,avail,pcent /` 的當時觀察值**（刪除 ④ 的舊 work 目錄之前），⛔ 沒有收進正式 artifact 或 raw |
| 重跑 | **第一次 `status = "ok"` 的 `--formal` 就是正式紀錄**，⛔ 不為了數字重跑。harness 或環境失敗（沒有產出報告）時，查明原因後可以重跑，**每次嘗試都記在本節**並保留 work 目錄。`assumption_violated` 依 ④ 計畫書停下、回頭 review 計畫 |
| 記憶體峰值 | 各程序的 cgroup 峰值只**記錄並對照 < 450 MiB**；有程序 ≥ 450 MiB 時標示出來，⛔ 不擋 `M_safety` 的裁定，留到 ⑥ 一併決定（正式的記憶體驗收在 ⑦ 的 memory harness；⚠️ ⑥ 的 v29 改成 ⑨-1） |
| 紀錄 | 報告留在 repo 外的 `<work>/`，⛔ 不進 repo；數字轉錄到本節與「二之二」（④ 計畫書「七」） |

**正式量測**（第一次就 `status = "ok"`，⛔ 沒有重跑）：

| 項目 | 值 |
|---|---|
| 執行 | 2026-09-29 02:09Z，`--formal`，run id `20260929T020903Z-583233`，約 5 分半（console 02:09:03～02:14:32）；work：`~/i074_stage2_sizing/formal-20260929T020903Z/`（repo 外） |
| 來源 | HEAD `423c932`（clone 的 HEAD 相同）；`scripts/`、`python/`、`.gitattributes` clean、harness 三個檔案等於 HEAD（⚠️ `repo_dirty = true` 只來自 `docs/` 的未 commit 變更——`--formal` 只要求前三者 clean） |
| image／identity | `sha256:2a90ad1c…5027`；Stage 2 identity 檔 SHA-256 `a02bf3bd…`；base `e1cbbbd`、counterfactual patch `ef7a4cdf…` |
| 環境 | DockerRootDir `/var/lib/docker`（與 L0～L5 同一個裝置）、logging driver `json-file`；fixture 的 mem-guard 447m；S 在 tmpfs |
| harness | harness `37ec8be9…`、shim `cdee9615…`、helper `96bec62a…` |

| 路徑 | `P_path` | 目錄取樣峰值 | 檔案系統取樣峰值 | 會計上界 | 主要組成（會計） |
|---|---|---|---|---|---|
| witness | 132.1 MiB | 131.6 MiB | 131.9 MiB | 132.1 MiB | run 目錄 74.3、HEAD worktree 36.0、replay worktree 15.0、`envcheck/` 6.6、容器 0.2 |
| **success** | **150.8 MiB** | 150.2 MiB | 150.4 MiB | 150.8 MiB | run 目錄 77.8、HEAD worktree 36.0、replay 與合成守門 worktree 各 15.0、`evidence/` 6.8、容器 0.3 |
| failure | 66.2 MiB | 65.7 MiB | 65.9 MiB | 66.2 MiB | HEAD worktree 36.0、兩個 worktree 各 15.0、record 與中繼檔 0.1、容器 0.2 |

**`P_B = 158,101,504 bytes（150.8 MiB）`**，`status = "ok"`（failure ≤ success 的預期假設成立）。三種量法在每條路徑都相差 ≤ 0.61 MiB（最大為 success 的 634,880 bytes；witness 462,848、failure 552,960 bytes） ⚠️ **⑦d 細部計畫 v1（✅ 2026-10-02 確認）**：當時的 accounted ⛔ 沒有 `runner_frozen_patches`（⑦d 起計入）——數字保留為歷史紀錄。
（這次⛔ 沒有其他程序的明顯干擾）；容器合併規則三條路徑都是 `max_non_overlapping`。和 ④ 的五次可用性驗證（150.5～150.7 MiB）相比
多 0.1～0.3 MiB，主要來自 HEAD worktree 35.8 → 36.0 MiB（④ 的 commit 讓 HEAD 變大）。
容器足跡：10 個容器的 `SizeRw` 全為 0、log 上界 4～8 KiB、metadata 採用值 192～256 KiB（twin 原始值 124～136 KiB）；
10 個程序的結束碼全部符合預期；結束後本次的容器歸零、S 已清。

記憶體（cgroup 峰值，含 page cache；對照 < 450 MiB——✅ **全部低於**，最高 342.0 MiB；⚠️ 只有證據層程序，⛔ 不含 replay，見下方第 3 點）：

| 程序 | 峰值 |
|---|---|
| `--envcheck`（witness） | 342.0 MiB |
| `--finalize`（success） | 341.0 MiB |
| `--publish-failed-record`（failure，預期 rc=1） | 282.2 MiB |
| `--recover-envcheck`（環境等價比對程序） | 294.7 MiB |
| `--recover-durability` | 332.2 MiB |
| `--check-failed-record` 的 Python 段（⚠️ 代量 preflight） | 302.3 MiB |
| `--recover-failed-record`（預期 rc=1） | 311.4 MiB |
| fixture（⛔ 不列入程序峰值） | 301.1～320.5 MiB |

host `MemAvailable` 低點 282.9 MiB。⚠️ cgroup 峰值含 page cache、隨 host 狀態浮動（④ 各次 ±40 MiB），⛔ 不是 ⑦ 的正式記憶體驗收。

**裁定**（依上面事前寫死的規則，✅ review 通過）：

```text id="i074_stage2_msafety_001"
P_B       = 158,101,504 bytes（150.8 MiB）     ← ⑤ 正式量測
M_safety  = 1,073,741,824 bytes（1 GiB）       ← 固定常數
required  = P_B ＋ M_safety = 1,231,843,328 bytes（約 1,174.8 MiB）
```

⚠️ 上面是 ⑤ 當時的算式（歷史紀錄）。**現行公式以 ⑥ 的 Stage 2 計畫書 v29 為準**：`required = P_B_BUDGET ＋ M_safety` = 1,241,513,984 bytes。

量測後 `/` 可用 8,301,604,864 bytes（約 7.7 GiB，使用率 80%）——⚠️ 這是量測結束後**人工執行 `df -B1 --output=avail /` 與
`df -B1M --output=avail,pcent /` 的當時觀察值**，⛔ 沒有收進正式 artifact 或 raw，只是當下的參考；**preflight 在見證趟之前、before
正式趟之前各檢查一次**（「二之二」）；見證趟已在 ③c 跑完，所以實際只剩 ⑩ 之前那一次。

⚠️ **提給 ⑥ 的事項**（⛔ 本步驟不改計畫；✅ 已由 ⑥ 的 Stage 2 計畫書 v29 處理——第 1 項改成預算 `P_B_BUDGET`）：

1. **HEAD worktree 會隨 commit 變大**：`P_B` 含一份 HEAD 的完整 worktree（36.0 MiB），⑦ 會 commit runner、orchestrator、
   測試與文件，HEAD 幾乎一定再變大。④ 計畫書「五」寫死的回退規則是「⑦ 的實際流程峰值 > ⑤ 裁定的 `P_B` → ⛔ 不得進入 ⑩、
   更新 `P_B`／`M_safety`、重跑 ⑤、回到 ⑥」——⛔ 沒有容差，所以**⑦ 之後幾乎一定要重跑一次 ⑤**（約 5～7 分鐘）。⑥ 要決定：
   接受「⑦ 之後照規則重跑 ⑤」，或另訂容差。
2. **`M_safety` 與 `P_B` 的關係**：`M_safety` 是固定常數、⛔ 不隨 `P_B` 重算——⑤ 重跑時只更新 `P_B`，`M_safety` 維持
   1 GiB，除非 ⑥ 另行裁定。
3. **⑤ 的記憶體只涵蓋證據層程序**（③ 的各入口），⛔ **不含 replay 程序本身**——「六、1」的「現行實測 524 MiB」是
   `evaluation.py` 的 Stage 2 路徑在改成串流之前的峰值（見「二、③」的實測表），一趟串流 loader 屬 ⑦，由 ⑦ 的
   memory harness 驗收。⚠️ 所以上表「全部低於 450 MiB」⛔ 不代表 replay 已經過關。

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

##### v26／v27 修訂：Stage 2 的計次與上限（2026-09-23，v26 ✅ 已隨 v28／③ v12 確認（2026-09-23）；上限的定法 ✅ 已於 v27 裁決）

⚠️ **觸發**：Stage 1 釘住的 image 已不在本機，使用者裁決「兩側都在新 image 重跑」
（見 Stage 2 計畫書「二、⑤」）。原本「⛔ 最壞三趟、不得再有第四趟」是以「三趟共用同一個 image」
為前提，⚠️ **那個前提已經不在**。另外「patch 失效不計入正式 scan」已隨 v25 確認，
卻只寫在 Stage 2 計畫書「二、①」，⛔ 與本節的上限互相衝突——一併寫進來：

| 趟次 | 性質 | 計次 | 次數上限 |
|---|---|---|---|
| D、D+1 | Stage 1 正式 scan（已完成） | 合為**一次** | ——（已執行） |
| **after' 見證趟** | ⚠️ **環境見證**，⛔ 不是新的 Stage 1 scan；結果⛔ 不得取代 D+1 | ⛔ **不計入正式 scan** | ⚠️ **1 趟**，⛔ 不得重跑——判 NOT_EQUIVALENT 就停（⛔ 不得換 image 或放寬判定後重比） |
| **before 正式趟** | Stage 2 的**唯一一次**正式 scan | 計為一次 | 1 趟 |
| before 重跑 | ⚠️ **只限** patch 失效（結束碼 6，已發布 failed-attempt record）之後，以**不同的 counterfactual patch SHA** 重跑。⚠️ **⑦ 總綱 v1 第五輪 review（✅ 2026-09-30 確認）**：改成：**只有白名單產品檔的 diff 改變、`counterfactual_semantic_sha256` 與失敗紀錄不同，且其餘 preflight 全部通過**，才可進行這一趟；⛔ **只改測試檔不得取得重跑資格**（完整 SHA 會變、語意 SHA 不變） | ⛔ 失效那一趟不計入 | ⚠️ **最多再 1 趟** |
| **⑨-1 量測趟**（⚠️ **⑦d 細部計畫 v1（✅ 2026-10-02 確認）**，使用者裁決 2026-10-02） | ⚠️ 容量量測：`e1cbbbd` ＋ tooling、⛔ 不套 counterfactual（算出的列與錨定的 D+1 相同，⛔ 不產生任何新的判讀資訊；輸出只留在 acceptance 的複本，⛔ 不得當成證據） | ⛔ 不計入正式 scan | **1 趟、崩潰重試 1 次**，另計、⛔ 不佔上限 ①②；兩份 patch 任一份的 bytes 或 SHA 再變，⑨-1 一律整個重跑（含量測趟），額度另行裁決（⛔ 不自動取得） |
| ⛔ 上限 | ⚠️ **兩個上限同時成立**（✅ 使用者裁決，v27）：<br>① **跑完並留下 artifact 的 replay 最多 5 趟**（D、D+1、after'、before、before 重跑）；<br>② **物理啟動最多 8 趟**（上面 5 趟 ＋ after'、before、before 重跑三格各 1 次崩潰重試；⚠️ **含已完成的 D、D+1**）。⛔ 任一上限到頂就不得再啟動 | | |

⛔ **v26 原本寫「最壞五趟」又允許「崩潰不佔次數」**，兩條合起來實際可以啟動 8 趟——⚠️ v27 把兩種計數分開寫明。

⚠️ **沒有產出任何 artifact 的崩潰**（比照 2026-09-17 的 OOM）：⛔ 不計入正式 scan、⛔ 不佔上限 ①，
⚠️ **但佔上限 ②**；必須留事故紀錄，而且**每一格最多因崩潰重試 1 次**——第二次崩潰就停止、另立 issue
（⛔ 不能以「沒產出」為由無限重試）。⚠️ D、D+1 已完成，⛔ 不再有崩潰重試的額度。
⚠️ **finalize 失敗**（階段二）只重跑 finalize，⛔ **不是** replay，⛔ 兩個上限都不佔。

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
（`:2694` / `:2812` / `:2941`，⚠️ 2026-09-23 依現況訂正行號），**與 `action_state` 不是同一個東西**。
⚠️ **v28 訂正**：v26 以前寫「top-level `position_action` 可以一併記錄供觀察」——⛔ **那個例外撤回**。
它⛔ 不是 B 的翻轉對象（⛔ 不得拿它判定「如預期翻轉」），⚠️ 但它**必須兩側相同**：
`_decision_action()` 在 lifecycle **之前**就算出它（`:2694`，早於 `:2738` 建 derived view、也就是
semantic pipeline 呼叫 lifecycle 之前），`_final_action_from_entry()` 的每個 return 都**原樣帶回**它；
counterfactual patch⛔ 沒碰這條路徑，differential guard 也要求未列出的欄位完全相同。
⚠️ **所以它一旦有差異，就是非預期外溢 → 分支 C**（見下方判讀器的差異表）。

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

⚠️ **v26 補充（2026-09-23，✅ 已隨 v28／③ v12 確認（2026-09-23））：判準要在 ⑩ 之前寫成程式**。原文只寫「由人判讀」，
⛔ 那等於判準在看到結果之後才落地。改成 **B／C 判讀器**（見 Stage 2 計畫書「三」與「六、9」）：
讀已封存的 comparison artifact，逐列套用上面的矩陣。⚠️ 另外要事前寫死
**「判讀欄位以外出現差異」算什麼**——`compare_rows()` 比的是全欄位，`differences` 會列出所有不同的欄位：

| comparison row 的 `differences` 內容 | 判定 |
|---|---|
| 只含 `lifecycle_phase`、`market_bias`、`action_state`、`position_action_condition`（⚠️ 且該物件**只有 `state` 不同**）、`rr_decoupling_candidate` | 依上面的矩陣判 B／C |
| 另含 top-level `position_action` | ⛔ **分支 C**（⚠️ v28 訂正：撤回 v26 的「只記錄」）——它算在 lifecycle 之前、counterfactual patch 沒碰它的推導路徑，有差異就是非預期外溢。⚠️ B **不要求它改變**，而是要求它**維持相同** |
| **含上面以外的任何欄位** | ⛔ **分支 C**——⚠️ 那代表 counterfactual 的影響溢出到 lifecycle 下游閉包之外（differential guard 在 fixture 層驗過的事，在實際資料上被推翻） |

⚠️ 允許清單**要在 ⑦ 實作判讀器之前**，以 `_decision_fields_from_summary()` 與 replay row 的實際欄位逐一核對；
⛔ 核對後若要增列，必須先證明該欄位**依賴 `lifecycle_phase`**（與 differential guard 的 allowlist 同一條紀律）。
⚠️ **156 列之中任一列是 C，整體就是 C**（⛔ 不以多數決）。

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
| `tooling_patch_sha256` | **腳本**：worktree 實際 `git diff --binary <base_commit>` 的輸出 hash，且⛔ **新增檔案必須含在內**（見下方套用方式）。⚠️ **I-074 Stage 2 ⑦ 總綱 v1（✅ 2026-09-30 確認）**：⑦a 起改由共用合成函式以 tree-to-tree、「⑦ 總綱 v1」「二」的 canonical diff（`--full-index` ＋ 全部釘死的參數、在隔離的暫存 bare repo 計算；⚠️ 第一輪 review：只加 `--full-index` 不夠）計算；空 patch 的值（空字串的 SHA）⛔ 不變，已封存的 Stage 1 證據不受影響 |
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
| 狀態 | 待決策（⚠️ **2026-09-23 已實際發生**：I-074 Stage 1 釘住的 image 已不在本機——⛔ 原本寫的「不影響 I-074 本輪」**已不成立**，見下方「對 I-074 的影響」） |
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

#### 對 I-074 的影響：~~⛔ 無~~ ⚠️ **2026-09-23 起：已實際發生**

~~D／D+1／comparator／finalizer **共用同一個釘死的 image ID**，所以兩趟的 sklearn 版本必然相同，
跨日比對不受影響；warning 在 probe 與 D 都出現，是**既有且一致**的狀況。
⛔ 本輪不要為了這筆去動 image 或 requirements——那會讓已經跑掉的 probe 失去可比性。~~
（⚠️ 上面是 2026-09-17 的判斷。**Stage 1 本身仍不受影響**——D／D+1 確實共用同一個 image，
證據已封存；受影響的是**還沒跑的 Stage 2**。）

| 事實（2026-09-23 查證） | 值 |
|---|---|
| Stage 1 釘住的 image | `sha256:d66030dca485…`——`docker image inspect` 回 `No such image`；repo 外也找不到 `docker save` 的備份 |
| 後果 | `pin-replay-image.sh` 依設計 fail-closed，I-074 Stage 2 的 `I074_MODE` 路徑跑不起來；本筆「image 消失即不可重現」的風險**成真** |
| ⚠️ 可能成因（⛔ **未查證是哪個操作刪的**） | pin 用的 tag `stock-trading-python-test:latest` 與 `python/scripts/test.sh` 等 5 支腳本**共用**，任何一次重新 build 都會讓釘住的 ID 失去 tag，之後被 prune 清掉 |

⚠️ **I-074 的處置**：使用者裁決「兩側都在新 image 重跑」，並以 after' 見證趟證明新舊 image 對該 bundle 等價
——見 [I-074](#i-074lifecycle-engine-的-rr-解耦decision-replay-已跑但一次都沒觸發到) 的 Stage 2 計畫書（⚠️ 現行版）「二、⑤」。
⚠️ 那一輪會順帶做到本筆「待決策」表的兩項：**專用 tag ＋ `docker save` tarball**（第三列的做法，只針對該次執行），
以及**記錄完整的 distributions 清單**（比第二列更進一步：清單與 hash 交叉驗證）。
⛔ **本筆的通則問題**（`requirements.txt` 的下限釘法、其他 bundle 的保存方式）**仍待決策**，⛔ 不因 I-074 的處置而關閉。

#### 待決策

| 選項 | 代價 |
|---|---|
| `requirements.txt` 改精確釘版（`==`） | 要一併處理既有 image 與 live 的版本差；影響範圍超出 SR Zone |
| bundle `manifest.json` 增記訓練時的 `pip_freeze`／sklearn 版本 | 只解決「知道差在哪」，⛔ 不解決「裝得回去」 |
| 為 bundle 保存 image tarball（`docker save`） | 最徹底，但每份 bundle 多數百 MB |
| 維持現狀，明確接受「image 消失即不可重現」 | ⚠️ 那要寫進 `sr-zone-scoring.md` 成為**明示的已知限制**，⛔ 不能默認 |

---

### I-117：Stage 1 的 comparator／finalizer／recovery 整份載入兩份 after artifact，超過 mem-guard

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復（⚠️ **已知限制**：Stage 1 已於 2026-09-18 靠權宜之計跑完，證據已 durable；本筆影響的是**日後重跑**這三條路徑） |
| 嚴重度 | 中（正式證據不受影響，但 recovery 路徑在常態記憶體下**跑不動**） |
| 分類 | Python / SR Zone / 容量 |
| 建立日期 | 2026-09-23 |
| 來源 | I-074 Stage 2 計畫書原本寫「comparator 的 730 MiB 問題**另立一筆處理**」，但一直沒有立案；2026-09-23 開工前盤點時補上 |

#### 事實

| 事實 | 值 |
|---|---|
| comparator | `build_crossday()`（`crossday.py`）要算 `d_only`／`d1_only` 差集，D 與 D+1 兩份 after artifact 必須**同時整份在場**；實測單份載入約 +365 MiB、兩份約 **730 MiB** |
| finalizer ／ recovery | Stage 1 的 `finalize_evidence()` 與 `recover_durability()`（`evidence.py`）都經 `_load_all()` **整份載入 9 個 archive 成員**——其中就有同樣兩份 after artifact |
| mem-guard 常態給得出的上限 | **531m**（⛔ 必然 OOM） |
| 2026-09-18 怎麼跑過的 | **停掉全部 7 個常駐 container**（釋放約 490 MB）再以 `MEM=900m` 執行，跑完立刻還原——⚠️ **權宜之計**，⛔ 不得變成程序 |

#### 影響

* 若 Stage 1 的 evidence 日後需要 `--recover-durability`，或要重跑 comparator 複核，
  ⛔ 在常態記憶體下**跑不動**，只能再停一次 live 服務；
* ⚠️ **同一道牆會出現在 I-074 Stage 2**：finalizer／recovery／preflight 與環境等價比對都要讀全量 artifact
  ——那邊已在 Stage 2 計畫書與 ③ evidence contract（⚠️ 現行版）的「五之一」改成串流設計，
  ⛔ **但那是新模組，⛔ 不會順帶修好 Stage 1 的這三條路徑**。

#### 處置方向（⚠️ 待規劃）

* 讓 comparator 與 finalizer／recovery 的全量比對改走**串流**（每列 digest ＋ 排序交集），
  ⛔ **不是同步 `zip()`**（插入／刪除／換序會整個錯位）；
* ⚠️ 可以沿用 I-074 Stage 2 的串流讀取模組（③b 產出），⛔ 不另寫一份；
* ⛔ **不得改變 Stage 1 已封存證據的驗證結果**——修完後，對 `python/baselines/i074_stage1/` 跑
  recovery 的驗證結論必須與現在相同（只差記憶體用量）。

---

### I-118：replay 相關腳本會洩漏 git worktree（註冊與 `/tmp` 目錄都會累積）

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 低（不影響任何證據或判定；會讓 `.git/worktrees` 與 `/tmp` 持續累積） |
| 分類 | Scripts / 測試衛生 |
| 建立日期 | 2026-09-23 |
| 來源 | I-074 ③b 第一輪 review 修正時，量測 `scripts/test-replay-args.sh` 前後的 worktree 數量 |

#### 事實（2026-09-23 實測）

跑一次 `IMAGE_REQUIRED=1 scripts/test-replay-args.sh`：`git worktree list` 由 **18 → 22**，`/tmp/tmp.*` 多 1 個；
當時 repo 已累積 18 筆 worktree 註冊，其中多筆是 `prunable`（目錄已不在、註冊還在）。

| 成因 | 位置 | 後果 |
|---|---|---|
| cleanup 只 `rm -rf "$WORKTREE"`，⛔ 沒有 `git worktree remove` | `scripts/finalize-evidence.sh`、`scripts/compare-replay-crossday.sh` 的 `trap` | 目錄刪了，**註冊留在 `.git/worktrees`**（變成 prunable） |
| 最後一步是 `exec docker run` | `scripts/run-replay-offline.sh`、`scripts/finalize-evidence.sh`、`scripts/compare-replay-crossday.sh` 等 | `exec` 取代了 shell，**EXIT trap 不會執行**——worktree 目錄與註冊都留下。⚠️ 正式執行時容器還在用這個 worktree，⛔ 不能在 `exec` 之前清；但執行完也沒有人清 |
| 測試以 fake docker 跑到 `exec docker run` | `scripts/test-replay-args.sh`（例如「結束碼 4 原樣傳出」） | 每跑一次常態測試就多一個 |

⚠️ **I-074 ③b 新增的部分已避開**：`finalize-stage2-evidence.sh` 用 `git worktree remove --force` ＋ 前景執行
（⛔ 不 `exec`）；它的 shell 測試在隔離的暫存 repo 裡跑；見證趟 E7 守門的放行案例改用 dry-run。

#### 處置方向（⚠️ 待規劃）

* cleanup 一律 `git worktree remove --force` ＋ `rm -rf`（比照 `run-replay-offline.sh` 既有的 `cleanup()`）；
* `exec docker run` 改為前景執行並保留 trap，原樣傳出結束碼（比照 `finalize-stage2-evidence.sh`）；
  ⚠️ 要確認不影響即時串流與結束碼的既有契約（Stage 1 的 exit 4／5 傳遞測試）；
* 測試結束時斷言 worktree 數量**沒有增加**，⛔ 不靠人工 `git worktree prune`。

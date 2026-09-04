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
- **下一個新編號從 `I-109` 起算。**（**I-108 於 2026-09-04 發出**——由 I-106 計畫書的非有限值實測分出；**I-106 / I-107 於 2026-09-03 發出**——I-107 由 I-106 的 review 分出（TR SMA(14) 與 Wilder ATR(14) 的公式分歧）；I-106 來自 T-040 regression baseline 實跑——T-040 regression baseline 實跑時發現 `atr_pct` 的窗口與註解不符、且 evaluation 與 runtime 用的是兩個不同的 ATR 演算法；**I-103 / I-104 / I-105 於 2026-09-02 發出**——I-103 由 I-102 計畫書 review 分出（Yahoo 批次路徑給不出逐檔寫入失敗）；I-104 由 I-102 實作 review 分出（其餘排程與 job 紀錄仍直接寫入原始錯誤）；I-105 來自 `2867` 跨月當天 live 首次 `partial`（`verification_unavailable` 的成因被丟棄）；I-101 / I-102 於 2026-09-01 發出——前者來自 live 的 indicator upsert 溢位、**已於同日修復並收斂**（未完成的 live 部署由 `todo.md` T-069 承接，**該筆已於 2026-09-02 部署驗收完成並收斂**），後者由它的 review 分出、**2026-09-02 實作部署完成並收斂**（現況規格歸檔在 `architecture.md`「寫入失敗的一致性契約」與 `api-reference.md` 的兩條端點，未完成的執行期觀察由 `todo.md` T-070 承接）；I-100 於 2026-09-01 發出，由 `todo.md` T-068 同日改列——**T-068 編號不回收**；**I-099 於 2026-08-31 發出後同日作廢**——誤把 `deploy.sh` 的保守預設當成與 live 的衝突，實際上該檔是範本、所有開關一律預設 `false` 是既有慣例；**編號不回收**；I-098 於 2026-08-31 由 I-096 的 review 發現分出；I-081～I-083 於 2026-08-21 發出（**I-081 / I-082 於 2026-08-27 隨 `todo.md` T-055 收斂**），I-084～I-087 於 2026-08-24 發出，I-088～I-092 於 2026-08-25 發出（**I-091 於 2026-08-28 收斂**），I-093 / I-094 於 2026-08-26 發出（I-093 已於同日收斂，**I-094 於 2026-08-28 收斂**），I-095～I-097 於 2026-08-27 發出，其中 **I-097 於同日改列 `todo.md` T-064**——編號**不回收**。）
  **發出新編號時記得把這一行一起往前推**——上一次就是漏了這步，I-089 發出去之後
  這裡還寫著「從 I-089 起算」，差一點又重用一次（I-070 已經發生過）。
  檔案裡看得到的最大是 I-108（2026-09-04 發出，**下一個可用的是 I-109**；I-102 已於同日收斂、編號不回收——I-096 / I-098
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
  本檔現有的 I-100 / I-103 / I-104 / I-105 / I-106 / I-107 / I-108、已收斂的 I-102 與下一個可用的 I-109），
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

### I-105：`verification_unavailable` 的成因被丟棄，live 的 `partial` 無法診斷

| 欄位 | 內容 |
|---|---|
| 狀態 | **已實作／待 review（2026-09-02），尚未部署** |
| 嚴重度 | 中（**不影響判定正確性**——三種結論仍分得開；但 `partial` 出現時查不出為什麼，等於「壞了卻不在明處」） |
| 分類 | Go / 排程 / 市場資料 / 可觀測性 |
| 發現日期 | 2026-09-02 |
| 來源 | `2867` 跨月當天 live 首次出現 `partial`（原記於 `todo.md` T-067 的觀察，該筆已收斂） |

#### 現象

`candle_gap_detection.go:535-539`：

```go
traded, err := s.exchangeReference.StockTradedDates(ctx, sc.symbol, sc.market, mc.year, mc.month)
if err != nil {
    unavailable++
    mergeAttempt(attempts, sc.symbol, now, store.VerificationUnavailable, false)
    continue        // ← err 到此為止，沒有任何 log
}
```

**`err` 被完全丟棄。** 而 `market/exchange_reference.go` 產生的錯誤其實很具體：

* `verification unavailable: twse stat=%q (symbol=%s)`（`:526`）
* `... %s status %d`（`:253`）、`... %s decode: %v`（`:263`）、`... request failed: %v`（`:247`）
* `... 未知市場別 %q（symbol=%s）`（`:501`）

**診斷資訊存在，但在那一行被扔掉**，最後只剩 `job_runs.error` 的
`verification_unavailable: 1 筆驗不了` 與 log 的 `unavailable=1` 兩個計數。

⚠️ 這與 [`architecture.md`](./architecture.md)「三種結論必須分得開」的宣示相牴觸：
那節寫「所有讀不懂的回應一律 `verification_unavailable`，**不猜測、壞在明處**」——
前半做到了，**後半沒有**。

#### 2026-09-02 的實例（live 首次 `partial`）

| 觀察 | 值 |
|---|---|
| `job_runs` | `partial`、135/0、16:25:03→16:25:10（6.41 秒）、`error` ＝ `verification_unavailable: 1 筆驗不了` |
| log | `pool=135 candidates=9 gap=0 unavailable=1 deferred=0 breaker_skipped=0` |
| `candle_verification_state` | `2867` 由 `verified` 轉為 **`unavailable`**，`consecutive_failures` **＝ 0** |

**候選數 9 與跨月的預測完全吻合**（視窗 08-19～09-01，`2867` 日 K 止於 08-19，
缺 08-20/21/24/25/26/27/28/31 ＋ 09-01，橫跨 8、9 兩個月）。

⚠️ **`consecutive_failures = 0` 不是 bug，它是證據**：依合併規則
（`candle_gap_detection.go:686-694`、`store/candle_verification_repo.go:55-56`）
「有任何成功 → 歸零」，而 `last_verified_at` 也更新成 16:25:08。
**所以兩個月份群組裡確實有一組成功、一組失敗。**

**但查不出是哪一組失敗、更查不出為什麼。** 情況證據指向 9 月那組
（前三個交易日只有 8 月、全部 `verified`，今天多了 9 月才出現 unavailable），
⛔ **那是推論不是實測**——而這正是本筆要修的東西。

#### 影響

* live 出現 `partial` 時無法判斷是暫時性（限流、逾時）還是結構性（端點格式改變、
  市場別判斷錯誤），也就無法決定要不要處置。
* 跨月是**每個月月初都會發生**的常態，這個形狀會反覆出現。

#### 修復方式（2026-09-02 實作）

在 `unavailable++` 之前記一筆 **Warn** log，帶 `symbol` / `market` / `year` / `month` /
`missing_dates` / `error`。

**只加 log、不動 `job_runs.error`**，理由有二：

1. **log 檔是持久化的、足以承接這個用途**——app log 鏡射到每日輪替的檔案
   （bind mount `./logs/backend`、保留 14 天），**容器重建後仍在**，
   實測可回溯到 2026-08-20。詳見 [`architecture.md`](./architecture.md) 的同一節。
2. **要把 reason 併進 `job_runs.error` 就得先過安全分類器**——那是使用者可見面
   （`GET /scheduler/status` → 前端 `Scheduler.svelte:227` 原樣渲染），
   端點錯誤可能帶 URL 與回應片段。分類器目前在 scheduler 套件，而
   [I-104](#i-104其餘排程與-job-紀錄仍直接寫入原始錯誤可從前端外洩連線細節)
   正要決定它該抽到哪裡。**在那筆定案前不動 `job_runs.error`。**

⚠️ **記 Warn 不是 Error**：驗不了是三種結論之一、預期會發生，不是不變式被違反。

**測試**（`scheduler/gap_unavailable_log_test.go`）四支：

| 測試 | 情境 | 斷言 |
|---|---|---|
| `LogsUnavailableCause` | 單一月份失敗 | 記下 `symbol` / `market` / 年月 / **原始 cause** |
| `LogsEachFailedMonth` | **兩個月份都失敗** | `unavailable == 2`，**各記一筆共兩筆**，8 月與 9 月都出現 |
| `CrossMonthOneSucceedsOneFails` | **8 月成功、9 月失敗**（＝2026-09-02 live 的形狀） | `unavailable == 1`；**只記一筆且 `month` 為 `September`**；`missing_dates == 1`；coalesce 契約（`anySuccess` 與 `anyUnavailable` 皆 true、`result` 取最嚴重為 `unavailable`） |
| `DoesNotLogWhenSuccessful` | 核對成功 | `unavailable == 0`，**不得記這行** |

⚠️ **兩種跨月情境要分開驗，不能只驗一種**（2026-09-02 review 修正——前一版只寫
「跨月時兩個月份各記一次」，而當時的 stub 只能按 symbol 回錯，**做不出一成一敗**，
於是「能不能指出是哪一個月失敗」根本沒被驗到，兩筆 Warn 蓋住了整個問題）。
為此在 `gapReferenceStub` 補了 `tradedErrByMonth`（key 為 symbol＋year＋month，
優先於既有的 `tradedErr`，沒設就退回原行為，既有使用者不受影響）。

⚠️ **這件事值得測試的理由**：計數照樣會加、`job_runs` 照樣收 `partial`，
所以「有沒有記成因」不會讓任何既有斷言變紅——**漏掉就是靜默漏掉**。
實作完成後把 log 那段暫時移除重跑，**三支相關測試全部變紅**，確認它們真的擋得住。

#### 關閉條件

live 再次出現 `verification_unavailable` 時，**能從 log 判斷出成因類別**
（哪一檔、哪一個年月、哪一種失敗）。

✅ **已部署**（2026-09-03 10:59:53，image build 10:59:52、HEAD `cdf29b5`、工作樹乾淨）。
容器內 binary 含 `candle gap verification unavailable`；啟動無誤（migrations v75、scheduler started）。

⬜ **但 live 尚未產生證據**——`candle_gap_detection` 每日 **16:25（local）** 才跑，
部署後到核實時只跑過 1 次 `intraday`。**部署完成 ≠ 關閉條件達成**：
要等下一次真的出現 `verification_unavailable`，且 log 裡有對應的
`candle gap verification unavailable`（含 symbol／year／month／missing_dates／error）才算關閉。

---

### I-104：其餘排程與 job 紀錄仍直接寫入原始錯誤，可從前端外洩連線細節

| 欄位 | 內容 |
|---|---|
| 狀態 | **已實作／待 review（2026-09-02），尚未部署** |
| 嚴重度 | 中（**不影響 runtime，但會把 DSN／主機位址／SQL 片段顯示在畫面上**，而且持久化後每次查詢都再洩一次） |
| 分類 | Go / 排程 / 安全 / 可觀測性 |
| 發現日期 | 2026-09-02 |
| 來源 | I-102 實作 review——該筆只涵蓋四個 `Evaluate` 路徑，其餘同類旁路留在這裡 |

#### 現象

I-102 定下的規則是「寫進使用者可見的 error 欄位前，一律過封閉值域的 reason code 分類器」
（`scheduler.safeJobErrorReason`），並已套用到它自己的四條路徑。
**其餘排程與 job 紀錄沒有跟上**，仍直接寫 `err.Error()`。

⚠️ **這些欄位都到得了畫面**：

| 路徑 | 前端渲染處 |
|---|---|
| `job_runs.error` → `GET /scheduler/status` | `Scheduler.svelte:227` 的 `{job.error}` |
| `sr_evaluation_jobs` / 訓練 job 的 error | `SRZones.svelte` **6 處**（`:1395`、`:1442`、`:1610`、`:2043` 等） |

**原始 driver 錯誤常帶 DSN、主機位址、連線字串或 SQL 片段**，寫進去等於顯示在畫面上；
`job_runs` 還保留 30 天，之後每次查詢都再洩一次。

#### 盤點（2026-09-02）

**`scheduler.go`——19 處**，分佈於 7 個 job：

| 函式 | 行 |
|---|---|
| `runChipDailySync` | `:640`、`:651` |
| `runStockSymbolSync` | `:679` |
| `runSRZoneVerification` | `:704`、`:714` |
| `runSREvaluation` | `:733`、`:761`、`:781`、`:791`、`:794`、`:814` |
| `runEvaluationUniverseSync` | `:1072`、`:1117` |
| `RunCorporateActionSync` | `:1303`、`:1315`、`:1341`、`:1347` |
| `runSRAnalysisOwned` | `:1504`、`:1531` |

**`candle_gap_detection.go`——2 處**：`:147`、`:179` 寫成
`"verification_unavailable: " + calErr.Error()`。
⚠️ **前綴是安全的、後面接的原文不是**——這一支正是「用可辨識前綴」慣例的來源，
卻同樣把 cause 原文接在後面。

**handler——2 處**：`api/handler/sr_regression_results.go:173`、
`api/handler/sr_zones.go:830` 的 `MarkFailed(ctx, jobID, err.Error())`。

⚠️ **`:1117`、`:1531`、`:1341` 那幾處是 `symbol + ": " + err.Error()` 或
`append(errParts, err.Error())` 的形式**，比單純的 `finishRun(..., err.Error())` 更容易漏掉——
搜尋 `err.Error()` 抓得到，但只看 `finishRun` 的呼叫點抓不到。

#### 為什麼當初沒有併進 I-102（已收斂）

I-102 的範圍是「四個 `Evaluate` 路徑的寫入失敗語意」。把 7 個 job ＋ 2 個 handler
一起改會讓那筆的受影響面翻倍，而且**驗收方式不同**——本筆純粹是輸出脫敏，
沒有行為語意的取捨要裁決。

#### 修復方式（2026-09-02 實作）

**① 分類器抽成 `internal/joberr` 套件。** 原本在 scheduler，但 handler 也要用，
讓 handler 依賴 scheduler 是錯的方向；放 store 則不屬於它的資料存取職責。
scheduler 保留 `type reasonCode = joberr.Reason` 的別名，既有程式碼不必全改。

對外三個函式：`Classify(err) Reason`、`Summary(stage, err)`、`SummaryFor(stage, symbol, err)`。
值域也補齊了（新增 `not_found` / `upstream_error`，並讓 `context.Canceled` 也歸 `timeout`）。

**② 25 處全部替換**：`scheduler.go` 19 處（含 4 處字串拼接形式）、
`candle_gap_detection.go` 2 處、handler 的 `MarkFailed` 2 處，
**外加立案時漏掉的 2 處**——`market.go:93` 與 `chip.go:317` 把逐檔失敗寫進
`failures` 陣列，那會被持久化並由前端 `Backfill.svelte:294` 原樣渲染。
⚠️ **它們走的是 `UpdateProgress` 不是 `MarkFailed`**，所以最初只掃 `MarkFailed`
與 `finishRun` 的盤點沒抓到。

⛔ **明確不在範圍**（2026-09-02 逐處查證）：`ShouldBindJSON` 的 400 回應
（`auth.go` / `watchlist.go` / `backtest.go` / `chip.go:232`）描述的是呼叫端自己的
payload，不碰 driver，保留原文對呼叫端才有用；**`position.go` 的兩個 `ApplyEvent` handler**
對 `ErrPositionVersionConflict` / `ErrPositionInvalidEvent` 的回應是 `errors.Is` 比對的
sentinel domain error，安全由建構方式保證。
⚠️ **這裡刻意不寫行號**——同一種輸出在兩個 handler 各有一對，用行號描述容易失準
（前一版只列了其中一對）。

**③ 新增 `joberr.SafeMessenger`**——這是實作時才浮現的問題：
「市場層級對照源陳舊: source_as_of=… 落後 N 個交易日（門檻 M）」是**我們自己組的訊息**，
只含日期與數字，把它壓成 `internal_error` 是**資訊淨損失、零安全收益**。
所以讓實作該介面的錯誤原文通過（`joberr.Describe`），並在介面註解寫死使用條件：
**訊息必須由本專案完整組出、不含任何來自外部系統的字串**。
`staleSourceError` 是第一個使用者。

⚠️ **既有測試 `TestRunSREvaluationFailsWhenWatchlistFallbackErrors` 原本斷言
`errMsg == expectedErr.Error()`**——那等於把外洩釘死。已改成斷言 `stage:reason` 形式
且不含原文。

#### 測試

* `joberr/joberr_test.go`——10 種已知成因的分類、未知一律 `internal_error`、
  **四個對外函式對三種帶敏感標記的 cause 都不外洩**、`SafeMessenger` 的通過與邊界。
* `scheduler/job_error_leak_test.go`——**逐 call-site 端到端**，分 A／B／C1～C4／D 七組：

  | 組 | 涵蓋 |
  |---|---|
  | A. 取清單失敗的早退（8 處） | `pre_market` / `daily_close` / `chip_daily_sync` / `sr_evaluation` / `sr_analysis` / `sr_zone_verify` / `evaluation_universe_sync` / `stock_symbol_sync` |
  | B. **逐檔失敗**（`SummaryFor`，4 處） | `sr_analysis` / `evaluation_universe_sync` / `chip_daily_sync` / `sr_zone_verify`——四處都**讓清單成功、逐檔才失敗**，真的執行到那一行並斷言 `stage:symbol:reason` |
  | C1. `corporate_action_sync`（2 處） | **早退**（列標的失敗）＋ **`errParts`**（標的成功、watchlist 失敗，走 `corporate_action_watchlist:` 那一行） |
  | C2. `sr_evaluation` 其餘替換點（3 個案例／**4 個替換點**） | **job create 失敗**／**`MarkDone` 失敗**（上游回 200、MarkDone 才失敗）／**上游失敗**（同時斷言 `job_runs.error` 與 `MarkFailed` 兩個欄位） |
  | C3. `corporate_action_sync` 的 `SyncSplits` 早退 | 注入會失敗的 split source，走 `corporate_action(splits)` 那一行 |
  | C4. `candle_gap_detection`（2 處） | **日曆取不到**（`calErr`）／**對照源日期取不到**，並斷言仍保留 `verification_unavailable: ` 前綴 |
  | D. tally 摘要與 `safeJobErrorSummary` | 形式與不外洩 |

  ⛔ **`SyncPerSymbolEvents` 回 `syncErr` 的那一格沒有測試**：依實作註解它只在
  **ctx 逾時／取消**時發生，而 `RunCorporateActionSync()` 自己建 context、沒有注入點。
  該分支的 cause 是 ctx sentinel（`context.DeadlineExceeded` / `Canceled`），
  本來就不帶外部字串，由分類器測試與程式碼審查承接。

  ⚠️ **每個案例都要真的走到「這次改動的那一行」**——第一版所有案例共用同一個
  failing watchlist，於是多數只觸發早退分支，**逐檔那條根本沒被執行、測試卻是綠的**。
  ⚠️ **只測分類器抓不到呼叫點漏接**——I-102 實作時就是這樣漏掉 `pre_market` 那一行。

  **mutation check**：把 8 條 call-site 分別改回 `err.Error()` 重跑
  （`sr_analysis` / `stock_symbol_sync` / `universe_sync` / `chip_sync` / `zone_verify` /
  `corporate_action_watchlist` / `corporate_action`（splits）/ `candle_gap` 日曆），
  加上後補的 `sr_evaluation` 的 `MarkDone`（`scheduler.go:816`）共 9 條，
  **每一條都確實讓對應子測試變紅**。

#### 關閉條件

1. ~~搜尋結果裡沒有任何一處 `err.Error()` 會流進使用者可見的 error 欄位。~~
   ✅ **已達成**——`scheduler.go` 內 0 處（`zap.Error` 不在此限）。
2. 逐 call-site 注入帶敏感標記的錯誤，斷言 error 欄位不含該標記。
   ✅ **本筆 21 個替換點中 19 個已有實際執行到該行的測試**，逐處對照見下表。

   | 檔案 | 替換點 | 測試 |
   |---|---|---|
   | `scheduler.go` | `chip_daily_sync` 早退 ＋ 逐檔 | ✅ ✅ |
   | | `stock_symbol_sync` 早退 | ✅ |
   | | `sr_zone_verify` 早退 ＋ 逐檔 | ✅ ✅ |
   | | `sr_evaluation` ×6：早退／symbols marshal／job create／上游失敗的 `MarkFailed` ＋ `finishRun`／`MarkDone` 失敗 | ✅ 早退、job create、上游失敗兩處、`MarkDone`（共 5 處）；⬜ symbols marshal（見下） |
   | | `evaluation_universe_sync` 早退 ＋ 逐檔 | ✅ ✅ |
   | | `corporate_action_sync`：`SyncSplits` 早退／列標的早退／`errParts` watchlist／`errParts` syncErr | ✅ ✅ ✅；⬜ syncErr（見下） |
   | | `sr_analysis` 早退 ＋ 逐檔 | ✅ ✅ |
   | `candle_gap_detection.go` | 日曆取不到 ＋ 對照源陳舊 | ✅ ✅ |

   ⛔ **兩個未覆蓋的分支，各有明確理由**：
   * **`json.Marshal([]string)` 失敗**（`scheduler.go:759`）——輸入是 `[]string`，
     **在現行型別下不可能失敗**，沒有注入點。
   * **`SyncPerSymbolEvents` 回 `syncErr`**——依實作註解只在 **ctx 逾時／取消**時發生，
     而 `RunCorporateActionSync()` 自己建 context。cause 是 ctx sentinel，不帶外部字串。

   ⚠️ **report marshal（`scheduler.go:800`）不是本筆的替換點**——它失敗時只把
   `reportJSON` 設成 `"null"`，**不寫進任何 job 欄位**，沒有外洩面。
   前一版把它列進表格、又漏掉真正未覆蓋的 `MarkDone`，兩個錯誤剛好互相抵銷成
   「19/21」；補上 `MarkDone` 測試後才是真的 19/21。

   ⚠️ **`pre_market` 與 `daily_close` 不計入本筆**——那兩處是 I-102 改的，
   本筆的測試順帶涵蓋它們，但**不能拿來充本筆的覆蓋率**（前一版的「14 個」就是這樣算出來的）。
   ⚠️ **前一版聲稱「`stock_symbol_sync` 與 `corporate_action_sync` 注入不了 stub」是錯的**
   ——`NewStockSymbolSyncer` 收的是 `StockSymbolSource` **介面**，
   而 corporate action 早就有可注入的 stub 組合（`scheduler_test.go` 的
   `TestRunCorporateActionSyncFailsWhenSymbolListUnavailable`）。兩者現在都有測試。

   ✅ **handler 側四處也都有 call-site 測試**（`api/handler/job_error_leak_test.go`
   ＋ 既有的 `TestMarketBackfill…`）：

   | 呼叫點 | 測試方式 |
   |---|---|
   | `market.go` 的 `failures` | 既有的實際回補流程測試 |
   | `chip.go` 的 `failures` | 實際跑 `runSync()`，來源回帶憑證的錯誤，斷言 `failures` 只含裸 reason code |
   | `sr_regression_results.go` 的 `MarkFailed` | 實際跑 `runEvaluationJob()`，上游用回 500 的 `httptest` server |
   | `sr_zones.go` 的 `MarkFailed` | 實際跑 `runTrainJob()`，同上 |

   ⛔ **不要用分類器測試代替 call-site 測試**（那正是本筆第一版犯的錯）。
   **mutation check**：把 `chip.go` 那行改回 `err.Error()`，該測試確實變紅並印出完整的
   外洩內容。
3. ⬜ **部署並在 live 觀察一次**。⚠️ **合法形式依欄位而不同，不要用單一格式判**：

   | 欄位 | 合法形式 |
   |---|---|
   | `job_runs.error` | `stage:reason`、`stage:symbol:reason`、經核准的 `SafeMessenger` 描述（目前只有「市場層級對照源陳舊…」）、以及**既有的固定安全文字**（例如「部分標的回補失敗，詳見 log」「N 檔未處理」） |
   | job 紀錄的 error（`MarkFailed`） | `stage:reason` |
   | `failures[].error`（backfill／chip） | **裸 reason code**（例如 `conn_refused`）——symbol 已在同一列的 `symbol` 欄，不重複 |

   **判準是「不含原始錯誤文字」，不是「符合某個格式」。**

   ✅ **已部署**（2026-09-03 10:59:53，同上）。容器內 binary 含
   `serialization_failure` / `internal_error` / `corporate_action(splits)` /
   `corporate_action_watchlist`，且**已無**舊的 `err.Error()` 拼接字串。

   ⬜ **但 live 尚未有失敗路徑可驗**——部署後只跑過 1 次 `intraday`（success，error 空）。
   本條要等任一排程真的失敗、且 error 欄符合上表形式才算達成。**不要為了驗證去製造失敗。**

   ℹ️ **附帶查證**（2026-09-03 06:30 的 `corporate_action_sync` `partial`）：
   `symbols_failed=1` 但 `error` 為空，**這是既有設計不是迴歸**——逐檔失敗只計數，
   成因記在 log（`adjuster.go:307`：`2867` dividends HTTP 404），
   `errParts` 只承接整體性錯誤（syncErr／skipped／watchlistErr）。該輪為部署前的 binary。

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
很可能只是在比對自己產生的空氣（I-104 已經踩過一次）。

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
會連帶改動 `NaN` 與 `0` 的既有行為、以及 I-106 的情境 1／3／6／8 fixture——
那是另一件事，不在本筆範圍。

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

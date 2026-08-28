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
- **下一個新編號從 `I-098` 起算。**（I-081～I-083 於 2026-08-21 發出（**I-081 / I-082 於 2026-08-27 隨 `todo.md` T-055 收斂**），I-084～I-087 於 2026-08-24 發出，I-088～I-092 於 2026-08-25 發出（**I-091 於 2026-08-28 收斂**），I-093 / I-094 於 2026-08-26 發出（I-093 已於同日收斂，**I-094 於 2026-08-28 收斂**），I-095～I-097 於 2026-08-27 發出，其中 **I-097 於同日改列 `todo.md` T-064**——編號**不回收**。）
  **發出新編號時記得把這一行一起往前推**——上一次就是漏了這步，I-089 發出去之後
  這裡還寫著「從 I-089 起算」，差一點又重用一次（I-070 已經發生過）。
  檔案裡看得到的最大是 I-096，但被移除的條目
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
  I-081～I-084 / I-086～I-090 / I-093 與下一個可用的 I-098），那是預期的，不是殘留。

---

### I-085：`yahoo.rate_limit` 同時節流除權息逐檔同步，但 config 註解只把它寫成「盤中」設定

| 欄位 | 內容 |
|---|---|
| 狀態 | **已實作／待 review**（2026-08-24 改完註解） |
| 嚴重度 | 低（行為正確且是刻意設計，**只有文件沒寫**；但改錯會直接拖慢公司行動同步） |
| 分類 | 文件 / Go / 設定 / 還原股價 |
| 發現日期 | 2026-08-24 |
| 來源 | 公司行動同步分片改造的 dev 驗收（2026-08-24，想用 stub 取代 Yahoo 時發現） |

`backend/config.yaml` 的 `yahoo:` 整段前言寫的是
「Yahoo 股市盤中資料源（非官方 API），作為 Tier-1 批次盤中源」，
`rate_limit` 的註解是「每分鐘請求上限（批次計為一次）」——讀起來像**只在盤中生效**，
而且 `enabled: false`（目前的預設）看起來等於「這段設定沒在用」。

**實際上 `rate_limit` 一直在用，而且是被除權息同步用掉的。**
`cmd/server/main.go:152` 無條件建立 `NewYahooDividendClient(cfg.Yahoo.RateLimit, log)`，
它與盤中報價客戶端共用同一個節流器（`sharedYahooLimiter`，理由見
`yahoo_dividend.go:42`——兩者打同一個 host，各自節流會讓實際速率加倍）。
`enabled` 只控制盤中那個客戶端要不要建立，管不到除權息。

**為什麼值得記**：這個數字直接進了 `corporate_action.timeout_sec` 的預算算術。逐檔同步每檔要打 Yahoo（除權息）
與 FinMind（減資），節奏由較慢的一邊決定；目前 FinMind 5/min 主導（每檔約 12 秒），
但**把 `yahoo.rate_limit` 調到 5 以下，主導權就換邊**，`corporate_action_sync`
的 45 分鐘預算會跟著失準，而調的人以為自己只動了盤中設定。

**要改的是註解，不是程式碼**：共用節流器是刻意的，且 `enabled` 只管盤中客戶端也合理。
`config.yaml` 的 `yahoo:` 段要寫明「`rate_limit` 同時節流除權息逐檔同步（`corporate_action_sync`），
與 `enabled` 無關」，並交叉引用 [`architecture.md`](./architecture.md) 的公司行動同步段。

**修復內容（2026-08-24）**：

* `backend/config.yaml` 的 `yahoo:` 段前言寫明「兩個客戶端、兩個端點、共用一個節流器」，
  並逐項標註 `base_url`／`enabled`／`batch_size` **只作用於盤中**，`rate_limit` 兩邊都吃。
* `rate_limit` 的註解補上它與 `corporate_action.timeout_sec` 的關係：調到 5 以下會換成
  Yahoo 主導逐檔節奏，45 分鐘預算失準。
* `docker-compose.yml` 的 `YAHOO_ENABLED` / `YAHOO_RATE_LIMIT` 補同樣的提醒
  （dev compose 沒有透傳這兩個，不需要改）。
* [`architecture.md`](./architecture.md) 公司行動同步那節補一段「Yahoo 那半的速率來自
  `yahoo.rate_limit`，與 `enabled` 無關」。

程式碼未動——共用節流器與 `enabled` 只管盤中客戶端都是刻意設計。

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

### I-080：`/sr-zones/event-timeline` 仍以 `zone_key` 摺疊，同一個 zone 會被顯示成多條鏈

| 欄位 | 內容 |
|---|---|
| 狀態 | **已修復／待 review**（2026-08-20，修法見 `todo.md` T-051 的實作結果） |
| 嚴重度 | 中（誤導：使用者看到的鏈數與實際身分不符，且沒有任何提示） |
| 分類 | Go / SR Zone / 事件鏈 / 對外 API |
| 發現日期 | 2026-08-20 |
| 來源 | T-048 全案 review |

T-048 立案要解的問題是「zone 邊界每次由 ATR 重算，事件鏈的身分綁在浮點邊界上會分裂」。
身分層（`zone_instances` / `event_instances`）已經解掉了它——**但唯一會把事件鏈顯示給人看的
端點沒有改**：`GET /sr-zones/event-timeline` 走的是 `analysis.BuildEventTimeline`，
仍以 `(zone_key, event_family)` 把 `market_event_states` 的快照摺疊成鏈
（`event_timeline.go` 的型別註解就寫著這個鍵）。

**實測落差**（同一份 84 次分析的資料）：

| symbol | `event_instances`（身分層真鏈） | timeline 端點的 `(zone_key, family)` 組合 |
|---|---|---|
| 2330 | 28 | 31 |
| 3105 | 38 | 35 |
| 6182 | 37 | 33 |
| 8150 | 25 | 21 |

**雙向都對不上**：timeline 多出來的是被 key 漂移拆開的鏈（102 個身分漂移過 key，
見 [I-079](#i-079zone_key_aliases-每身分-8-筆上限已有-23-個身分撞頂)）；
身分層多出來的是身分終止後的重生鏈（`seq > 1` 共 10 條）。

**影響面**：T-041 的前端 timeline 正是要接這個端點，接上去等於把 T-048 修好的分裂
原封不動顯示給使用者。發現當時前端還沒有引用它（`frontend/src` 沒有 `event-timeline`
的呼叫），所以趕在接上之前修掉。

**後續**：前端已於 2026-08-21 接上（T-041 的 Event Timeline 面向），讀到的是修好之後的
身分層鏈；顯示端的判讀規則見
[`sr-zone-scoring.md`](./sr-zone-scoring.md)「前端 Event Timeline 的判讀規則（現況）」。

---

### I-077：同一個交易日重複分析會讓事件提早老化到 `EXPIRED`

| 欄位 | 內容 |
|---|---|
| 狀態 | **已修復／待 review**（2026-08-20，修復方式與實測見下方「修法定案」與「實作結果」） |
| 嚴重度 | 中（不影響既有單日流程，但一天多打幾次分析就會改變事件狀態與下游 Market State） |
| 分類 | Python / SR Zone / 事件生命週期 |
| 發現日期 | 2026-08-19 |
| 來源 | T-048 階段 C 修法後的 as-of 階梯驗收，重跑第七階時發現 |

`_normalize_previous_event_state`（`python/backtest/modular/sr_scoring/event_engine.py`）
把「被 carry 一次」直接當成「多存活一根 K 棒」：

```python
age_bars = int(state.get("age_bars") or 0) + 1
expired = state_name != LIFECYCLE_EXPIRED and age_bars >= expires_after
```

計數單位其實是**分析次數**，不是 K 棒數。一個交易日只分析一次時兩者相等，這也是
T-045 當初的隱含前提；但 `POST /sr-zones` 沒有任何「同一天只算一次」的限制。

2026-08-19 的階梯實測：第七階（2026-08-18）跑完後有兩條 `SUPPORT_RECLAIM` 鏈是
`CONFIRMED`／active；**candles 一根都沒變**，只是把同一階再打一次，兩條就同時
`CONFIRMED → EXPIRED`，`event_transitions` 多出兩筆。下游 `market_state_from_event_states`
只看 active 事件，所以這會實際改變 Market State。

**不是 T-048 造成的**：老化規則屬於 T-045 的事件鏈，階段 C 只是把它的結果存進
`event_instances`／`event_transitions`，於是這個原本只存在於記憶體摺疊裡的行為
第一次留下可查的痕跡。

**修法方向**（尚未決定，需與 T-049 一起看）：讓老化以「最新 K 棒的 timestamp 是否推進」
為準，而不是以分析次數為準——例如把上次分析的最後一根 K 棒時間存進 state，
carry 時只有時間推進才 +1。這會動到事件狀態的推導，屬於下游決策行為，
依 T-048「不改任何下游決策邏輯」的界線不在該筆範圍內。

**承接**：todo.md T-049 規劃時必須一起處理或明確排除本 issue；T-049 會讓 Market State
與所有下游改讀同一套 state，若不先定義老化單位，重複分析造成的 `EXPIRED` 會被放大成
Bias／entry 的可見差異。

**但決策點比 T-049 更早：T-052 上線前就要定。** 分析排程一上線，同一個交易日會出現
「排程跑一次＋人工再點一次」，`age_bars` 一天前進 2——而 T-052 累積的正是 I-074 / T-049
要用的驗證母體，**老化單位沒定就先開排程，等於一邊累積一邊污染**。

---

#### 修法定案（**已實作／待 review**，2026-08-20）

**採 A ＋ B**：A 修老化單位（正確性），B 在 T-052 加同日守衛（**只為省資源，不是正確性依賴**）。

##### 盤點推翻了上面「修法方向」的前提

上面寫「把上次分析的最後一根 K 棒時間**存進 state**」——那繞遠了。實際盤點：

| 事實 | 位置 |
|---|---|
| Python 早就有「這次的 K 棒時間」 | `pipeline.py:89` 的 `analyzed_at = frame.index[-1]`，是**資料日期不是 wall clock** |
| Go 早就有「上次的 K 棒時間」 | `GetLatestMarketEventStates` 撈回的每一列都帶 `analyzed_at`，而 Go 的 `analyzed_at` 是從 Python 回應解析的（`client.go:79`）——整條鏈都是 K 棒時間 |
| 唯一的洞 | `scoreZonesPreviousEventState`（`client.go:961`）是**手工白名單** struct，`state_json` 不整包轉發，所以 Python 現在拿不到上次的時間 |

**與階段 E 的 `zone_uid` 漏欄位是同一個陷阱**：Go 往 Python 送、往前端回，兩個方向都是手工白名單。
所以不必在每一列 state 重複存同一個純量，**送一個 request 層級的純量即可**。

##### 修改目標與不做的範圍

* **目標**：`age_bars` 只在最新 K 棒真的推進時 +1；同一根 K 棒重複分析不再老化。
* **不做**：不改 `expires_after_bars` 的任何門檻值、不改 family lifecycle 規則、
  不改事件偵測、不改 `_zone_key()`、不動身分層四張表。
* **不做**：不改 `reuse_existing=true` 那條 provider 路徑（它不送 previous states）。

##### 受影響檔案與資料流

```text
Go  analysis/client.go
     ├─ scoreZonesRequest ＋ PreviousAnalyzedAt（request 層級純量）
     └─ 取值：previousEventStates[0].AnalyzedAt
        （該查詢用 analysis_id = (SELECT … LIMIT 1)，整批同一次分析，取 [0] 安全）
Python http_server.py        /sr-zones request model ＋ previous_analyzed_at
       scoring.py → pipeline.py → decision_engine.py   往下傳這個純量
       event_engine.py
         ├─ build_event_state_summary(..., previous_analyzed_at=None)
         └─ _normalize_previous_event_state(state, bar_advanced)
              age_bars += 1 只在 bar_advanced 時
```

**為什麼是 Go 送「上次的時間」而不是 Go 直接算 `bar_advanced`**：Go 在呼叫前不知道這次的
`analyzed_at`（它由 Python 從 frame 算出）。Go 若改用自己 DB 的最新 candle ts 去比，
就會出現第二個「這次的 K 棒是哪一根」的判準，而 limit／還原係數都可能讓兩邊不一致。
**「這次分析站在哪根 K 棒」的 authority 留在 Python 一份。**

##### 資料 contract 變化

| 變更 | 型態 | 相容性 |
|---|---|---|
| `/sr-zones` request ＋ `previous_analyzed_at`（RFC3339，可省略） | 純新增可選欄位 | 舊呼叫端不送＝維持現行行為 |
| `/sr-zones` response | **不變** | — |
| DB schema | **不變** | 不新增欄位、不 migration |

##### 缺值與邊界

* **缺 `previous_analyzed_at`**（舊呼叫端、沒有 previous states、evaluation/replay 未帶）
  → `bar_advanced = True`，**完全等於現行行為**。既有資料與既有呼叫端不受影響。
* **時間沒有前進反而倒退**（as-of 回放、資料修正）→ 視為未推進，不老化。保守側。

##### 主要風險與回滾

| 風險 | 對策 |
|---|---|
| 這是**決策可見改變**，照規矩該做 replay 分佈驗證，但母體正是 T-052 要產的（雞生蛋） | 影響面可窮舉：只在「兩次分析共用同一根最新 K 棒」時生效，而該情況今天的行為**可證明是錯的**（見上方實測）。因此改用**冪等性**驗收，不用分佈比較 |
| 純量從 Go 一路傳到 `event_engine`，中間任一層漏傳就靜默退回舊行為 | 缺值語意刻意設計成「等於現行行為」，所以漏傳不會壞資料——但也因此**不會報錯**。以端到端冪等測試把守，不只靠單元測試 |
| 動到 `pipeline.py` / `scoring.py` / `decision_engine.py` 這條決策核心路徑（階段 E 曾刻意迴避） | 這三層只是**傳遞純量**，不改任何判斷；判斷只發生在 `_normalize_previous_event_state` 一處 |
| 回滾 | 純新增可選欄位 ＋ 一個條件式，無 migration。`git revert` 即可 |

##### 測試與驗證策略

* **單元（Python）**：`bar_advanced=False` 時 `age_bars` 不動、且不會提早轉 `EXPIRED`；
  `True` 時行為與現行逐項相同；缺值時走 `True`。
* **單元（Go）**：`previousEventStates` 為空時不送該欄位；非空時送的是那批 states 的
  `analyzed_at`。
* **端到端（現有 dev 階梯）**兩條門檻：
  1. **冪等性（紅燈變綠燈）**：同一階連跑兩次，`market_event_states` 逐欄相同。
     **今天會不同**——這是本 issue 的直接證據，修完必須相同。
  2. **回歸**：四檔 21 階階梯。**判準不是「四檔都逐欄相同」**——計畫書初版寫錯了，
     實測推翻：階梯是按**交易日**切的，但個股不一定每天都有 K 棒，於是有幾階是
     「同一根 K 棒被連續分析」，那正是本 issue 的情境。實測分布：

     | symbol | 分析次數 | 相異 K 棒 | 同棒重複階數 |
     |---|---|---|---|
     | 2330 | 21 | 21 | 0 |
     | 3105 | 21 | 21 | 0 |
     | 6182 | 21 | 18 | **3** |
     | 8150 | 21 | 18 | **3** |

     **baseline 本身就含有 I-077 的錯誤老化**，所以正確判準是：
     * 2330／3105：決策**逐欄相同**（全是相異 K 棒，行為不該改變）。
     * 6182／8150：差異**必須侷限在同棒那幾階**，且成因可歸因到老化欄位。
       這裡有差異是**修法生效**，不是回歸失敗。
     * 身分層（`zone_instances` / `zone_relations` / alias）與老化無關，數字應逐項重現；
       事件鏈（`event_instances` / `event_transitions`）會因 6182／8150 的狀態改變而變動。
* **回歸套件**：`backend/scripts/test.sh` 與 `python/scripts/test.sh` 全綠。

##### 完成後歸檔（**已完成**）

* 老化單位改為「K 棒推進」、`previous_analyzed_at` 的來源與缺值／時間倒退語意 →
  [`sr-zone-scoring.md`](./sr-zone-scoring.md)「Aging → `EXPIRED`」那段。
  **原本計畫寫要歸檔到 `api-reference.md`，那是錯的**：該文件只涵蓋 Go 對外 API，
  Go↔Python 的 `/sr-zones` request contract 一直記在 `sr-zone-scoring.md`。
* T-052 的同日守衛定位（省資源，非正確性依賴）→ 留在 todo.md T-052。

##### 實作結果（2026-08-20）

| 門檻 | 結果 |
|---|---|
| 單元（Python） | 599 passed / 1 skipped（+4：同棒不老化、推進照舊老化、缺值＝舊行為、`_bar_advanced_since` 六種邊界） |
| 單元（Go） | 全綠（+2：有 previous states 時送出、無 previous states 時 `omitempty` 整個消失） |
| **冪等性** | 同一根 K 棒（`2026-08-12`）再打一次：事件狀態 **33 vs 33 筆、逐欄 0 差異**，含 `age_bars`。且非空跑——該次有 **26 筆 carried、`age_bars` 4~19**，舊碼會全部 +1 |
| 回歸：2330／3105 | 決策與事件狀態**逐欄相同**（21 階全是相異 K 棒） |
| 回歸：6182／8150 | 有差異，且**全部 6 筆都落在同棒階；K 棒推進階 0 筆差異** |
| 身分層 | 329 / 57 / 685 逐項重現，`zone_uid` 1282/1282 |
| 六條門檻 ＋ D4 | 全部 0 |

**差異的成因逐筆對得上**：

* **8150**：只有 `age_bars 18 → 17`。那些事件早已 EXPIRED，所以事件狀態 CSV 逐欄相同
  （`age_bars` 在 `state_json` 裡，不在該 CSV 的欄位集），差異只出現在 `decision_summary`。
* **6182**：`active_event_types` 多出數個 `INTRADAY_RECLAIM`——**本來被提早老化掉的 reclaim
  事件現在正確地留在 active**，正是本 issue 原始紀錄的情境。`event_transitions` 因此由 250
  降到 246，少的 4 筆就是不再發生的提早 `EXPIRED` 轉換。
* **頂層決策欄位沒有變**：6182 的差異欄位是 `decision_summary` / `price_path_json` /
  `decision_derived_view_json`，`market_bias` / `entry_permission_state` /
  `position_action` / `event_market_state` 在這個母體裡都沒被翻掉。

**驗收判準本身在實作中被修正過一次**：計畫書初版假設「階梯每階 K 棒都推進」，實測發現
6182／8150 各有 3 階是同一根 K 棒重複分析——**baseline 本身就含有本 issue 的錯誤老化**。
修正後的判準見上方「測試與驗證策略」。

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

### I-092：`sr_analysis` 的 `symbols_total` 是「扣掉跳過後的實際處理數」，與其他排程相反，且與自己的 log 不一致

| 欄位 | 內容 |
|---|---|
| 狀態 | **已實作／待 review**（2026-08-26 修完，**計畫書保留供 review**） |
| 嚴重度 | 中（數字本身正確，但同一個欄位在不同 job 代表不同東西，排程頁無法橫向比較） |
| 分類 | Go / Scheduler / API / 可觀測性 |
| 發現日期 | 2026-08-25 |
| 來源 | SR zone 分析排程的 live 驗證（2026-08-25 唯讀盤點） |

#### 現象

`runSRAnalysis` 把**扣掉跳過之後**的數字寫進 `job_runs`，但同一輪的完成 log 印的是
**清單原本的大小**（`scheduler/scheduler.go:1292`、`:1294`、`:1296`）：

```go
total := len(symbols) - skipped                     // :1292 → 寫進 job_runs
s.log.Info("sr analysis done", …,
    zap.Int("total", len(symbols)), …)              // :1294 → log 印 11
s.finishRun(ctx, runID, jobName, total, failed, …)  // :1296 → job_runs 記 10
```

2026-08-21 那輪的實例（`0050` 因「已分析過今日 K 棒」被跳過）：

```
log:      {"msg":"sr analysis done","total":11,"analyzed":10,"skipped":1,"failed":0}
job_runs: symbols_total = 10
```

**同一輪、同一個名字叫 `total` 的東西，兩處是不同的值。** 看排程頁的人會問
「watchlist 明明 11 檔，為什麼顯示 10」，而畫面上沒有任何欄位解釋得了。

#### 這是三支排程裡唯一的例外

| Job | `symbols_total` 的語意 | 跳過／未處理怎麼算 |
|---|---|---|
| `corporate_action_sync` | **當日名單大小**（`len(symbols)`，`:1173`） | 併進 `symbols_failed`（刻意，見下） |
| `evaluation_universe_sync` | **池大小**（`poolSize`，T-062 定案） | 不計入失敗，只記進 log 的 `skipped` |
| `sr_analysis` / `sr_analysis_chip` | **名單大小 − 跳過** ← 唯一的例外 | 不計入失敗 |

前兩者的分母都是「本輪該做的清單有多大」，只有 SR 分析的分母會浮動。

**注意不要順手把第三欄也統一掉**：`corporate_action_sync` 把未處理併進 `failed`
是刻意的，因為那邊的「沒輪到」是**逾時導致該做而沒做**；SR 分析與評估池的「跳過」
是**判定過後確認不需要做**。兩者語意不同，現況是對的
（見 [`api-reference.md`](./api-reference.md) 的「`symbols_total` / `symbols_failed`
的單位是標的數」）。本筆只統一分母。

#### 為什麼統一成「清單大小」而不是「實際處理數」

`symbols_total` 在 [`architecture.md`](./architecture.md) 與
[`api-reference.md`](./api-reference.md) 裡是被當成「這輪的清單有多大」在讀的，
維運用它判斷「窗口有沒有被截斷」「池有多大」。改成實際處理數會讓數字每天浮動
（11 / 10 / 11 …），而浮動的原因看不見——那正是這筆要解的問題，不能用它當解法。

---

#### 修改計畫書

**修改目標**：`sr_analysis` / `sr_analysis_chip` 的 `job_runs.symbols_total`
改為 `len(symbols)`（watchlist 大小），跳過數改由 log 呈現，與另外兩支排程一致。

**不做的範圍**

* **不動 `symbols_failed` 的語意**，也不把跳過併進失敗（理由見上表註記）。
* 不動 `corporate_action_sync` 與 `evaluation_universe_sync`——它們的分母已經是清單大小。
* 不改 `/scheduler/status` 的欄位或 JSON 形狀，不改前端。
* 不改跳過的判定邏輯（`srAnalysisSkipReason` 一行不動）。

**受影響檔案與資料流**

| 檔案 | 變更 |
|---|---|
| `scheduler/scheduler.go` | `runSRAnalysis`：`finishRun` 改傳 `len(symbols)`；完成 log 的欄位改成與 `evaluation_universe_sync` 同名的 `total` / `analyzed` / `skipped` / `failed`，避免兩支排程的 log 欄位各叫各的 |
| `scheduler/scheduler_test.go` | 補測試（見下） |
| `docs/api-reference.md` | 在 `symbols_total` 那節寫明「分母一律是本輪的清單大小，跳過不扣」，並點名三支排程 |

資料流不變，只有寫進 `job_runs` 的那一個整數變了。

**狀態推導的變化**

`finishRun` 依 `failed >= total` 推導 `failed`／`partial`／`success`。分母變大之後，
**「全部跳過只剩一檔且那檔失敗」從 `failed` 變成 `partial`**——這是這次改動唯一會
改到狀態字串的情境。`partial` 在這裡是對的：那輪確實有跑，只是跑得不完整。

**主要風險與回滾**

* 風險很低：只影響一個統計數字與其推導出的狀態字串，不影響任何分析行為。
* **要注意歷史資料不會回填**：改動前後的 `job_runs` 列語意不同，跨 8/25 比較
  `sr_analysis` 的 `symbols_total` 會看到一個階梯。這點要寫進 `api-reference.md`。
* 回滾：把那一個參數改回 `total` 即可。

**測試與驗證策略**

* `scheduler` 測試：stub 讓 watchlist 回 3 檔、其中 1 檔跳過，斷言
  `job_runs.symbols_total == 3`（不是 2）、`symbols_failed == 0`、狀態 `success`。
* 邊界：全部跳過時 `symbols_total == 清單大小`、`failed == 0`、狀態 `success`。
* 邊界：只剩一檔沒被跳過且該檔失敗時，狀態是 `partial` 而不是 `failed`
  （把上面那個狀態變化釘住，否則日後會被當成迴歸改回去）。
* dev 驗收非必要：這是純計數改動，單元測試涵蓋得完整。

**完成後的歸檔位置**

[`api-reference.md`](./api-reference.md) 的
「`symbols_total` / `symbols_failed` 的單位是**標的數**」那一節——補上
「分母是本輪的清單大小，跳過不扣」的通則、三支排程的對照，以及 8/25 前後的階梯。

---

#### 修復方式（2026-08-26）

照計畫書實作，**沒有偏離**。程式碼的實質改動只有一行。

| 檔案 | 變更 |
|---|---|
| `scheduler/scheduler.go` | `runSRAnalysis` 的 `finishRun` 改傳 `len(symbols)`；`analyzed` 改由 `len(symbols) - skipped - failed` 算（同值，只是不再借用被移除的 `total` 變數）。加註解說明三支排程的共用通則，以及為什麼跳過仍不計入 `symbols_failed` |
| `scheduler/scheduler_test.go` | candle stub 加 `perSymbol`（預設 nil，既有測試行為不變）；更新既有的全跳過測試；新增 3 支 |
| `docs/api-reference.md` | `symbols_total` 那節補通則、三支排程對照表、歷史階梯警告、狀態字串變化 |

**log 欄位維持原名，未改**——這點與計畫書的字面敘述不同，理由見下。

**驗證結果**：

* `backend/scripts/test.sh`（vet → test → build）全數通過。
* **負向對照**：把分母改回 `len(symbols) - skipped` 重跑，**4 支測試全部失敗**，
  其中 `TestSRAnalysisAllAttemptedFailedIsPartialNotFailed` 得到
  `status:failed symbolsTotal:1`——正好呈現舊慣例下的那個狀態字串。已還原。
* 新測試涵蓋計畫書列的三個情境：3 檔跳 1（`total=3` 非 2）、全部跳過（`total=3`、`success`）、
  只剩一檔且失敗（**`partial` 而非 `failed`**）。

#### 實作時發現的兩件事

**1. 既有測試的註解寫著一個不成立的理由。**
`TestSRAnalysisSkipsWhenLatestCandleIsNotToday` 原註解是
「job_runs 的 total 要把跳過的扣掉，**否則每個假日都會看到一筆 failed**」。
實際推導 `finishRunDegraded`：`failed` 分支要 `total > 0 && failed >= total`，
而假日是 `failed = 0`，**兩種分母算出來都是 `success`**（`total=1/failed=0` 與
`total=0/failed=0` 同解）。那個理由不成立，已在測試註解裡更正並留下推導。

**2. 計畫書的 log 欄位那句是錯的，沒有照著做。**
計畫書寫「完成 log 的欄位改成與 `evaluation_universe_sync` **同名的**
`total` / `analyzed` / `skipped` / `failed`」——但 `evaluation_universe_sync` 的欄位其實是
`pool` / `attempted` / `skipped` / `failed`，兩者並不同名，那句話自相矛盾。

而且 `attempted` 與 `analyzed` **是不同的量**：前者含失敗（送出請求數），
後者不含（成功數）。硬改成同名會讓兩個不同的數字共用一個名字，比現況更糟。

**所以維持各自的欄位名，只在 `api-reference.md` 寫明「兩者的第一個欄位都是清單大小」。**
這是與計畫書的字面偏離，理由如上；實質目標（分母統一、log 與 job_runs 不再打架）
完全達成——改完之後 `sr_analysis` 的 log `total` 與 `job_runs.symbols_total` 就是同一個數。

---

### I-095：zone 角色翻轉只記在身分層，事件層沒有任何「壓力被突破」的紀錄

| 欄位 | 內容 |
|---|---|
| 狀態 | **待決策**（要不要讓 role flip 也產生事件，是設計取捨不是明確的 bug） |
| 嚴重度 | 低（**不影響任何決策**——該事件本來就是 `decision_visible=false`；影響的是「事實累積」的完整性） |
| 分類 | Python / SR Zone / 事件層 / 身分層 |
| 發現日期 | 2026-08-26 |
| 來源 | 0050 `2026-08-26` 分析內容的逐項核實（`analysis_id=117`） |

#### 現象

`0050` 在 2026-08-26 把 `104.44～105.06`（zone `f2f1ab63`，`recent_pivot`）
**從壓力翻成支撐**——身分層記得清清楚楚：

| seq | role | state | started | ended | end_reason |
|---|---|---|---|---|---|
| 1 | RESISTANCE | INVALIDATED | 2026-08-20 18:22 | **2026-08-26 17:01:12** | **`ROLE_FLIPPED`** |
| 2 | SUPPORT | ACTIVE | **2026-08-26 17:01:12** | — | — |

⚠️ **這裡的 `SUPPORT / ACTIVE` 是 role incarnation；畫面事件鏈上的 `CONFIRMED`
是另一層狀態**（2026-08-27 再核實）。`ROLE_FLIPPED` transition 指向 seq 2 的新
incarnation UID，且這個 zone 在 seq 1 只有 `RESISTANCE`，**不存在可被復活的舊
terminal SUPPORT incarnation**。同時出現的 `SUPPORT_RECLAIM / CONFIRMED` 與
`SUPPORT_RETEST / CONFIRMED` 也都是當下新開的 event chain（各自 seq 1），不是舊鏈復活。
所以「新 SUPPORT 一世有正確建立」是已驗證的正常行為，不是本筆的問題；本筆只問
「角色翻轉所代表的壓力突破，是否也要在事件事實層留一筆紀錄」。

價格也支持這個判斷：當日開 104.10、高 106.05、**收 105.90**，站上帶頂 105.06。

**但事件層對這件事沒有 breakout 紀錄**：該分析的 23 筆 event-state snapshots 裡
**沒有任何 `RESISTANCE_BREAKOUT`**；同日 22:00 的第二次 0050 分析（`analysis_id=128`）
也同樣是 0，0050 全歷史的 `RESISTANCE_BREAKOUT` event chain 亦為 0。

⚠️ 前一版寫「同日其他標的共產生 9 筆」不夠精確（2026-08-27 再核實）：

* 9 筆是單輪 `market_event_states` 的 snapshots（`2478` 2、`3630` 1、`5490` 4、
  `6243` 2），**包含 carry-forward，不能全叫做當輪新產生**。
* 單輪真正新增的 `market_event_detections` 是 2 筆，分別在 `3630`、`5490`；
  同日第二輪亦各 1 筆。
* 這兩個有新 detection 的對照案例，其 `event_sequence_json` 與
  `decision_derived_view_json` 都沒有 `RESISTANCE_BREAKOUT`，證明
  **shadow event 的建立與 Decision 隔離機制本身正常**；缺的是 0050 這種 role-flip
  breakout 根本沒有建立事件，不是「事件有建立但被 Decision 吃掉」。

#### 成因：事件是依「當前 role」分派的，翻轉後就走不到壓力側

`detect_market_events`（`event_engine.py:614`）：

```python
for z in zone_scores:
    if z.role == ZoneType.RESISTANCE.value:
        events.extend(_resistance_zone_events(...))   # RESISTANCE_BREAKOUT 在這裡
        continue
    if z.role != ZoneType.SUPPORT.value:
        continue
    ...                                                # 支撐側的三個分支
```

zone builder 在這根 K 棒已經把它歸類成 **SUPPORT**（價格收在帶頂之上），
所以它走支撐分支、產出 `INTRADAY_RECLAIM` ＋ `SUPPORT_RETEST_HELD`，
**`_resistance_zone_events` 對它一次都沒被呼叫**。

換句話說：**「壓力被突破」正是它翻成支撐的原因，而那個原因讓它錯過了記錄突破的分支。**

另一個佐證：`0050` 當日最近的壓力是 `107.18～107.82`，而最高只到 106.05，
**沒碰到任何仍是壓力的 zone**，所以其他 zone 也不會補上這筆。

#### 核實與更正（2026-08-27）

本筆原本寫「同日那 9 筆就是『碰到壓力但還沒翻轉』」——**那句話是錯的**，逐筆查過之後：

| **當輪的 9 筆 `RESISTANCE_BREAKOUT` state snapshot**（每輪各一份，兩輪共 18 列） | 筆數 |
|---|---|
| **當天新生成的事件**（`age_bars=0`、`carried_from_previous=false`） | **2**（`3630` / `5490` 各一） |
| carry 進來的既有事件（`age_bars` 1～3，多數已 `EXPIRED`） | 7 |

⚠️ **「9 筆」是當輪的 state snapshot 數，不是「當天新增 9 個事件」**——
`market_event_states` 每輪都會把所有仍在追蹤的事件寫一份，所以同一個事件會在
17:00 與 22:00 各留一列。

而且其中 `2478` 的 `120.637~121.363` **今天確實翻轉了**（`ROLE_FLIPPED`），
一度看起來像本筆的反例。追事件史才確認不是——那筆是 **08-24 新生成、carry 到今天已過期**的殘留：

```
08-24 17:02  CANDIDATE  age 0  carried=false   ← 新生成（當天 SUPPORT→RESISTANCE 翻轉）
08-25 17:02  CANDIDATE  age 1  carried=true
08-26 17:02  EXPIRED    age 2  carried=true    ← 今天 RESISTANCE→SUPPORT 翻轉，仍無新事件
```

**核實後結論反而更強**：2026-08-26 有**兩個** zone 發生 RESISTANCE→SUPPORT 翻轉——
`0050` 的 `f2f1ab63` 與 `2478` 的 `120.637~121.363`——**兩個都沒有產生新的
`RESISTANCE_BREAKOUT`**。原本只有一個樣本，現在有兩個獨立樣本。

⚠️ **順帶發現一個不對稱**（尚未查明是否為預期）：`2478` 在 08-24 的
**SUPPORT→RESISTANCE** 翻轉當下**有**新生成的 `RESISTANCE_BREAKOUT`（age 0、
`PENDING_CLOSE_CONFIRMATION`，intrabar 突破未收上）。也就是說**兩個方向的翻轉行為不同**：
翻成壓力時會產生事件，翻成支撐時不會。這是因為前者翻轉後 zone **仍是壓力**、走得到壓力分支，
後者翻轉後變成支撐、走不到。處理方向若選 B，這個不對稱要一併考慮。

#### 為什麼這仍然值得記

`RESISTANCE_BREAKOUT` 是 `decision_visible=false` 的**只寫不讀事實層事件**
（見 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「事件的決策可見性」），
它存在的**唯一目的就是累積事實**，供日後的分佈分析與 replay 使用。

而「壓力被站上」這件事：

* **身分層有**（`zone_role_incarnations` 的 `ROLE_FLIPPED`）。
* **事件層沒有**。

於是兩層對「今天有沒有發生突破」給出不同答案。要用事件層做母體統計時
（例如「突破後 N 根的表現」），**翻轉型的突破會整批缺席**——
而那可能正是最值得看的一類，因為它是唯一「突破成功到足以改變角色」的樣本。

**目前不影響任何決策**：該事件不進決策，`0050` 當日的
`market_bias` / `entry_permission_state` / `position_action` 都由可見事件推導，
已逐項核實無誤（`active` 桶 shadow 洩漏 0、`event_sequence_json` 無 shadow 名字）。

#### 處理方向（**擇一，未定案**）

**A. 維持現狀，只補文件。** 在 `sr-zone-scoring.md` 寫明
「`RESISTANCE_BREAKOUT` 不涵蓋翻轉當下的突破，翻轉請看 `zone_role_incarnations`」。

* 成本最低，且**不動任何會產生事件的程式碼**——事件層一旦多出事實，
  即使 `decision_visible=false`，也會經 carry-forward 進入下一次分析的 `states`，
  要重新確認四個過濾點都擋得住。
* 代價：做事件層統計的人必須自己去 join 身分層，而那件事很容易被忘記。

**B. 翻轉時補發一筆事件。** 在角色翻轉的路徑上補一筆
`RESISTANCE_BREAKOUT`（或新的型別如 `ROLE_FLIP_BREAKOUT`），維持 `decision_visible=false`。

* 讓「突破」這個事實在事件層完整。
* ⚠️ **新事件型別要走完整的隔離檢查**：四個過濾點、
  `EVENT_TYPE_META` 的 `decision_visible`、身分層寫入、前端標記，
  缺一個就會經方向桶或位置型讀者改到決策（見「事件的決策可見性」的三類讀法）。
* ⚠️ 若沿用既有的 `RESISTANCE_BREAKOUT` 型別，要注意它的觸發條件含**量能門檻**
  （`relative_volume >= HIGH_VOLUME_BREAKDOWN_THRESHOLD` 或 `volume_confirmation == FAILED`），
  而角色翻轉**沒有**這個條件——兩者的語意會被混在同一個名字下。

**傾向 A**：這是事實完整性問題，不是正確性問題；而 B 要動的是「產生事件」這條路徑，
風險與收益不成比例。**但如果日後真的要用事件層做突破後表現的統計，就必須先解掉這筆**，
否則母體會系統性地少掉最強的那一類樣本。

**相關**：本次核實的另外兩項已分別立案——I-096（touch 被命名並解讀成 reclaim）
與 [`todo.md`](./todo.md) T-064（中文標籤 SSOT／呈現契約待整理，**2026-08-27 由 I-097 改列**）。

---

### I-096：`structure_state` 把單純碰觸命名成「收復候選」，並會回饋 Lifecycle／Decision

| 欄位 | 內容 |
|---|---|
| 狀態 | **待決策**（2026-08-27 診斷後從「待修復」下修——見下方診斷結果） |
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

**相關**：[`sr-zone-scoring.md`](./sr-zone-scoring.md)「RR 語意分層」（原記於 `todo.md`
T-055，已收斂）——同一類問題的另一個面向：決策語意的多個數字／狀態並列而未分層。

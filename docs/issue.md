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
- **下一個新編號從 `I-095` 起算。**（I-081～I-083 於 2026-08-21 發出，I-084～I-087 於 2026-08-24 發出，I-088～I-092 於 2026-08-25 發出，I-093 / I-094 於 2026-08-26 發出，其中 I-093 已於同日收斂。）
  **發出新編號時記得把這一行一起往前推**——上一次就是漏了這步，I-089 發出去之後
  這裡還寫著「從 I-089 起算」，差一點又重用一次（I-070 已經發生過）。
  檔案裡看得到的最大是 I-094，但被移除的條目
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
                --glob '!**/node_modules/**' --glob '*.{md,go,ts,svelte,py,sh}' \
                'I-0[0-9][0-9]' . | sort -u)
  ```

  **用 `rg` 而不是 `grep -r`，兩個理由都是踩過的**（2026-08-25 re-review 修正）：

  * **副檔名要含 `.ts` / `.svelte`。** 舊版只掃 md/go/py/sh，漏掉整個前端——
    `Scheduler.test.ts` 曾經留著一個 `issue.md I-090` 的活指標，那次檢查完全沒抓到。
  * **`grep -rho … | grep -v node_modules` 濾不掉任何東西。** `-h` 已經把檔名拿掉了，
    留下的每一行就只是 `I-0xx` 本身，永遠不含 `node_modules` 字串——那個後置過濾
    從第一天起就是空操作。`grep -r` 照樣會遞迴進 `node_modules`；舊指令之所以沒有
    出現誤報，只是**那裡剛好沒有符合 `I-0xx` / `T-0xx` 的內容落在被掃的副檔名裡**，
    是運氣不是過濾。`rg` 的 `--glob '!**/node_modules/**'` 才是真的排除。

  列出的 ID 必須**只剩明確標為歷史沿革的引用**（「原記於…」「當時編號…」），
  不能有任何「見 I-0xx」形式的活指標。
  **本節自己會出現在輸出裡**（上面提到 I-040 / I-056 / I-069 / I-070～I-072 / I-076 /
  I-083 / I-084 / I-086～I-090 / I-093 與下一個可用的 I-095），那是預期的，不是殘留。
  `todo.md` 的 T-055 review 沿革內也還有兩處 I-083 引用，都寫成「原記於…，已收斂」的歷史形式，
  同樣不是殘留。

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

### I-081：`sr-zone-scoring.md` 的 legacy action pipeline 第 3 條門檻寫 `< 1.0`，實作是 `< 1.5`

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 低（文件落後，runtime 行為正確且一致） |
| 分類 | 文件 / Python / SR Zone / 決策語意 |
| 發現日期 | 2026-08-21 |
| 來源 | [`todo.md`](./todo.md) T-055 的 RR 語意核實（live 0050 分析） |

[`sr-zone-scoring.md`](./sr-zone-scoring.md)「Legacy action pipeline」第 3 條寫：

> 3. 若 primary zone `risk_reward_ratio < 1.0`：加上風險報酬不足註記。

實作用的是 **`< 1.5`**，而且出現兩次：

* `decision_engine.py:499` —— `elif rr < 1.5:` → 「主交易區風險報酬比不足。」
* `decision_engine.py:516` —— `if rr is None or rr < 1.5:` → 直接 `return "WATCH"`

註記文案與文件描述一致，**只有門檻數字對不上**。1.0 和 1.5 之間的 RR 在實際系統裡
會被加註記並擋到 `WATCH`，但照文件讀會以為它是乾淨通過的。

**要改的是文件，不是程式碼**：1.5 與 `_minimum_rr()` 的下限一致，是刻意的調校結果，
不是筆誤。

**與 T-055 的關係（2026-08-21 定案）**：**由 T-055 一併改，不單獨動。**
T-055 會把 RR 門檻改成 `probe_min_rr` / `full_entry_min_rr` 兩層具名門檻，這一條的敘述
整段都要照新語意重寫——先單獨把 1.0 改成 1.5，等於同一段寫兩遍，而且中間那版
仍然對不上改完後的實作。該動作已列進 T-055 的「完成後歸檔」與驗收條件，
**T-055 不做完就不算完成**；完成後本筆整筆移除。

---

### I-082：文件把 EXPIRED 的 `Buy` 守門記在錯的位置（**行為正確，原始指控已被重現推翻**）

| 欄位 | 內容 |
|---|---|
| 狀態 | 修復中（**迴歸保護已補完；只剩文件改寫，等 T-055 一起做**） |
| 嚴重度 | 低（**由中下修**：行為正確，文件把守門機制記在錯的函式） |
| 分類 | 文件 / Python / SR Zone / 決策語意 |
| 發現日期 | 2026-08-21（同日以重現實驗推翻原始指控） |
| 來源 | [`todo.md`](./todo.md) T-055 的 RR 語意核實（讀碼推論）→ 2026-08-21 重現實驗 |

#### 重現結果（2026-08-21）：**無法重現，原始指控不成立**

依既定流程「先重現再修法」建探針窮舉，結論是 **EXPIRED 的 primary zone 走不到 `Buy`**。

| 組合 | `structure_state` | `action` |
|---|---|---|
| SUPPORT ＋ TREND_UP | `BREAKDOWN` | **Avoid** |
| SUPPORT ＋ RANGE_BOUND | `BREAKDOWN` | **Avoid** |
| SUPPORT ＋ TREND_DOWN | `BREAKDOWN` | **Avoid** |
| RESISTANCE ＋ 三種 regime | `NORMAL` | **Avoid**（`bearish_setup` 擋下） |
| **對照組**：同一 zone 只把 `EXPIRED` 換成 `VALIDATED_RECENTLY` | `NORMAL` | **Buy** |

對照組是關鍵：其餘六個 `strong` 條件**確實全中**（relevance 84.81 ≥ 75、confidence 1.0、
EV 0.05、RR 3.0、distance 0.0005、regime flags 空），足以產生 `Buy`。
**唯一的差別就是 `recent_validation`**，所以 Avoid 確實由 EXPIRED 造成。
端到端 `build_decision_summary` 同樣是 `action=Avoid` / `final_entry=BLOCKED`。

#### 真正的守門在哪裡（原始分析漏看的一行）

守門存在，只是**不在 `strong`**，而在 `_structure_state`（`decision_engine.py:2242-2243`）：

```python
if primary_zone.role == ZoneType.SUPPORT.value:
    ...
    if primary_zone.recent_validation == RecentValidation.EXPIRED.value:
        return "BREAKDOWN"
```

而 `_decision_action` 對 `structure_broken` 的判斷（`:512-518`）發生在 `strong` 計算**之前**，
命中就直接 `return "AVOID", "EXIT", "Avoid"`。加上 `_pick_primary_zone` 的兩條清單
（嚴格與 fallback）**都排除 `AT_ZONE`**，primary 只可能是 SUPPORT 或 RESISTANCE，於是：

* SUPPORT ＋ EXPIRED → `BREAKDOWN` → Avoid，**永遠到不了 `strong`**；
* RESISTANCE → `bearish_setup=True` 且 `bullish_setup=False` → Avoid，同樣到不了。

`_decision_action` 與 `_structure_state` **各只有一個呼叫點**（`decision_engine.py:2402` / `:2388`，
都在 `build_decision_summary` 內），所以上述窮舉涵蓋全部路徑。

#### 原始分析錯在哪（留作教訓）

1. **只讀了 `strong` 的條件式，沒有往上讀 `structure_broken` 的提前 return。**
   「條件式裡沒有 X」不等於「X 沒被擋」——守門可以在更早的分支。
2. **2026-08-21 的「嚴重度上調」也是錯的。** 當時推翻了「confidence 過不了 0.65」的舊推論
   （那部分確實錯：EXPIRED 是「最近一次觸碰被跌破」而非「很久沒測試」，見 `scoring.py:420`），
   但**用一個正確的觀察去支撐一個錯誤的結論**——實際上 confidence 拉到 1.0 也照樣 Avoid。
3. 兩次都是**讀碼推論未經重現**就寫進 issue。這正是「先重現再修法」要防的情形。

#### 剩下的兩件事（這才是本筆的實際範圍）

**1.（文件）`sr-zone-scoring.md`「Legacy action pipeline」第 4 條把守門記在錯的位置。**

原文是：

> 4. 若 primary zone `recent_validation=EXPIRED`：加上近期驗證失效註記，**且不應升級到 `Buy`**。

讀起來像「第 4 步自己會擋」，但第 4 步只加註記；**擋 `Buy` 的是第 5～6 步的
`structure_broken` / `bearish_setup` 提前 return**。敘述的結論正確、機制錯位——
正是這個錯位讓原始分析誤判。應改寫成明確指出擋在哪裡。

**2.（迴歸保護）這道守門沒有任何測試釘住，而 T-055 就要動這段。**

`strong` 擋得住，靠的是上游 `_structure_state` 的一行 `EXPIRED → BREAKDOWN`。
**目前沒有任何測試斷言「EXPIRED primary 不得輸出 Buy」**：

* 若日後有人把 `_structure_state` 改成「跌破後已收回就不算 BREAKDOWN」（合理的演進方向），
  Buy 路徑會**靜默打開**；
* T-055 的「F1 連帶後果」正要改 `_decision_action` 的 `strong` 讀哪個 RR，**改的就是這一段**。

這與 [I-074](#i-074t-044-的-rr-解耦只有單元測試層級的證據decision-replay-驗證無法執行)
記錄的「`test_continuation_only_needs_price_evidence` 無法防守 RR 被加回來」是同一類缺口：
行為正確，但沒有東西釘住它。

**建議**：把本次重現探針收成永久迴歸測試（`tests/test_decision_engine.py`），
斷言「EXPIRED primary ＋ 其餘 strong 條件全中 → action 不得為 `Buy`」，**並附對照組**
（換成 `VALIDATED_RECENTLY` 必須是 `Buy`）。沒有對照組，測試會在 fixture 退化成
「根本達不到 strong」時假綠。

#### 不做的事

* **不改 `strong` 的條件式。** 行為已正確；加 `recent_validation != EXPIRED` 是重複守門，
  且會讓「守門在哪」變成兩處，之後更難維護。
* **不改 `_pick_primary_zone` 的 fallback。** 「沒有合格 zone 時仍要選一個出來」是刻意設計，
  真正的守門在動作升級那一層，已證明有效。

**與 T-055 的關係**：不再有修法衝突（本筆不動 `strong`）。迴歸測試已於 T-055 之前補完，
成為 T-055 改 `_decision_action` 時的安全網。

#### 實作結果（2026-08-21）：迴歸測試已補（**第 2 項完成**）

`python/backtest/modular/sr_scoring/tests/test_decision_engine.py` 新增三條：

| 測試 | 作用 |
|---|---|
| `test_expired_primary_zone_never_upgrades_to_buy` | 守門本體。EXPIRED primary × {SUPPORT, RESISTANCE} × {TREND_UP, RANGE_BOUND, TREND_DOWN} 六組，斷言 action 不得為 `Buy`（且為 `Avoid`） |
| `test_expired_guard_control_group_would_otherwise_reach_buy` | **對照組，不可刪**。同一 fixture 只把 EXPIRED 換成 `VALIDATED_RECENTLY` → 必須是 `Buy`，證明其餘五個 `strong` 條件確實達標 |
| `test_expired_primary_zone_end_to_end_is_not_buy` | 端到端補一刀，並斷言 primary **真的是** EXPIRED（fallback 生效），避免測試空轉 |

**刻意停在 `_decision_action` 這一層**：文件那句「不應升級到 `Buy`」講的就是 legacy action
pipeline。端到端還會再經 `final_entry_permission` 降級——實測對照組在端到端是
`Hold` / `WAIT_CONFIRMATION`，用端到端做對照就分不出「被 EXPIRED 擋下」還是「被進場閘降級」。

##### 變異測試：證明這組測試釘得住

把 `_structure_state:2242-2243` 的 `EXPIRED → BREAKDOWN` 兩行移除後重跑：

```
FAILED test_expired_primary_zone_never_upgrades_to_buy
AssertionError: EXPIRED primary zone 升級到 Buy（role=SUPPORT global_trend=0.05）——守門失效
1 failed, 2 passed
```

**注意端到端那條在變異下仍然綠燈**（`final_entry_permission` 照樣把它降成 `Hold`）。
所以**只有端到端測試是抓不到這個回歸的**——這正是守門測試要停在 `_decision_action`
那一層的理由，也說明端到端斷言不能取代單元層斷言。

##### 測試結果

| 範圍 | 結果 |
|---|---|
| `test_decision_engine.py` | **78 passed**（原 75，+3） |
| `python/scripts/test.sh` 全套 | **602 passed / 1 skipped**（原 599 / 1，+3） |

##### 剩餘工作（2026-08-21 定案：由 T-055 承接）

只剩本節第 1 項的**文件改寫**（`sr-zone-scoring.md`「Legacy action pipeline」第 4 條的機制錯位）。
**由 T-055 一併處理，不單獨動**——它與
[I-081](#i-081sr-zone-scoringmd-的-legacy-action-pipeline-第-3-條門檻寫--10實作是--15)
要改的第 3、4 條，正是 T-055 重寫 RR 門檻敘述時會動到的同一段。
該動作已列進 T-055 的「完成後歸檔」與驗收條件，**T-055 不做完就不算完成**；
完成後本筆整筆移除。

在那之前本筆維持「修復中」：runtime 行為正確且已有迴歸測試釘住，唯一未結的是文件敘述。

---

### I-078：T-048 身分層有兩條路徑在驗收母體裡從未被執行

| 欄位 | 內容 |
|---|---|
| 狀態 | 已知限制（不計畫單獨修復，關閉條件見下） |
| 嚴重度 | 中（不影響現行結果，但這兩條路徑一旦真的被走到，沒有實測證據可依靠） |
| 分類 | Go / Python / SR Zone / 身分追蹤 / 驗證缺口 |
| 發現日期 | 2026-08-20 |
| 來源 | T-048 全案 review（階段 A～E 實作後盤點） |

四檔 21 階、84 次分析的 as-of 階梯是 T-048 唯一的端到端證據，但這份母體裡有兩條
設計上很微妙的路徑**一次都沒被觸發**：

* **`zone_instances` 的收攤（`EXPIRED`）**：329 個身分的狀態分布是
  ACTIVE 293 / RESHAPED 20 / MERGED 12 / SPLIT 4，**`EXPIRED` 是 0**。
  `ZoneIdentityRepo.ListLive` 的註解說明了這段最容易寫錯的地方——資格用
  `<= maxObservedAbsences` 而不是 `<`，否則剛好累到上限的身分再也撈不出來、
  永遠不會被判失格、收攤流程整條變成不可達的死碼。**這個陷阱的反面（正確版本）
  目前只有單元測試證明。**
* **alias 備援命中（`matched_by_alias`）**：兩輪階梯都是 0（階段 C 已記錄）。
  三段關聯決策的第一段（既有鏈命中）把所有情況都吃掉了。

**成因是母體太小而不是實作有問題**：21 個交易日、4 檔，身分還來不及缺席到失格。
真正的解法是補分析排程（見 `todo.md` T-052），不是為這兩條路徑另外造假資料。

**關閉條件拆成兩段**：

* **EXPIRED 收攤**：分析排程（todo.md T-052）上線、production 母體累積到身分會自然失格後，
  確認 `zone_instances` 出現 `EXPIRED`，且行為與單元測試一致。
* **alias 備援**：不能假設 T-052 一定會讓 `eventIdentityStats.MatchedByAlias` 自然非零——
  T-048 實測中第一段既有鏈命中把多數情況吃掉了。排程上線後先觀察一段時間；若仍為 0，
  改由 targeted integration/live fixture 或 T-050 的可觀測性 metric 證明這條路徑，
  而不是把 T-052 卡死在不可控的自然觸發上。

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

### I-091：標的靜默停止更新時，沒有任何機制會讓它浮出來（由 `2867` 發現）

| 欄位 | 內容 |
|---|---|
| 狀態 | **待修復** |
| 嚴重度 | 中（**潛在偵測缺口**。已向 TWSE 查證：這次沒有任何資料遺失——缺的是「**真的漏了的時候有沒有人會知道**」） |
| 分類 | Go / 市場資料 / 排程 / 可觀測性 |
| 發現日期 | 2026-08-25；**2026-08-26 依實測整筆改寫**（原本兩個前提被推翻），同日再依 **TWSE 原始資料查證**修正處置方向 |
| 來源 | 評估標的池的唯讀盤點 |

#### 現象

評估標的池 135 檔裡，**134 檔的日 K 都更新到 2026-08-25，只有 `2867` 停在 2026-08-19**。

#### 原本的兩個前提都是錯的（2026-08-26 查證）

這一筆最初寫成「`2867` 已下市，會每輪失敗一檔、把 `partial` 變成常態噪音」。
兩個前提實查後都不成立，**而且錯的方向讓問題看起來比實際小**：

| 原本的說法 | 實際 |
|---|---|
| 「典型的下市／合併前最後爆量」 | **仍在上市**。`stock_symbols` 由 `stock_symbol_sync` 每日自 TWSE 同步，2026-08-26 06:30 那輪仍有 `2867 三商壽`、`is_listed = true` |
| 「會在每一輪都失敗一檔，壓成 `partial`」 | **一次都沒失敗**。2026-08-25 16:21 的全量輪（`job_runs` id 1827）是 135 檔、`symbols_failed = 0`、狀態 `success` |

決定性的一行 log——那輪 `2867` 的回補**成功**，只是筆數短少：

```
{"msg":"backfill done","symbol":"2867","count":3}   ← 視窗 08-15～08-25 共 7 個交易日
{"msg":"backfill done","symbol":"2884","count":7}
{"msg":"backfill done","symbol":"2912","count":7}
```

FinMind 自 08-20 起就沒有 `2867` 的資料，但 TWSE 清冊上它還在上市。

> **當時的推論寫的是「最合理的解釋是停牌」——那是錯的**，而且錯在方法：
> 從「沒有成交資料」反推原因，只能推到「沒有交易」，推不到「為什麼」。
> 正解要查公告，見下一節第三層。

#### TWSE 原始資料查證（2026-08-26）：**沒有資料遺失，抓取也沒有 bug**

直接向權威來源查了三層——**前兩層都不足以定性，第三層才是答案**。

**第一層——上市清冊**（`https://isin.twse.com.tw/isin/C_public.jsp?strMode=2`，
系統每日 06:30 抓的同一個端點）。`stock_symbols` 與 TWSE **逐欄吻合**：

| 欄位 | TWSE | 系統 |
|---|---|---|
| 名稱 | 三商壽 | 三商壽 |
| ISIN | TW0002867009 | TW0002867009 |
| 市場別 / 產業別 | 上市 / 金融保險業 | 同 |
| CFICode / 上市日 | ESVUFR / 2012-12-18 | 同 |

→ **`stock_symbol_sync` 沒有 bug。** 但「上市」不等於「有在交易」——停牌的股票仍留在這張清冊上，
所以這一層證明不了停牌與否，必須再查第二層。

**第二層——TWSE 自己的每日成交資訊**
（`STOCK_DAY?date=20260801&stockNo=2867`）：

```
stat: OK   回傳 13 筆   最後一筆 115/08/19   收盤 9.70   成交 76,212,030 股
```

13 筆正好是 8/3～8/19 的**全部**交易日；**TWSE 自己在 08-19 之後也沒有任何成交資料**，
且 08-19 的成交股數與資料庫存的 `76212030` 完全一致。

**第三層——TWSE 公告**（`openapi.twse.com.tw/v1/news/newsList`，2026-08-26 補查）。
**前兩層都不足以定性原因**：清冊只證明「仍在名單上」，成交資料只證明「沒有交易」，
兩者都推導不出「為什麼」。公告才是權威來源：

> 三商美邦人壽保險股份有限公司（三商壽，公司代號：2867）**因進行股份轉換**其上市
> 有價證券**自 115 年 8 月 20 日起停止買賣**，並**自 115 年 9 月 1 日起終止上市**
> （公告日 115/07/28）

佐證：`2867` **不在**「集中市場暫停交易證券」（`exchangeReport/TWTAWU`）清單裡——
那張表當下只有 1218 泰山的當日暫停。兩份資料一致。

**結論**：FinMind 回傳 3 根是**正確且完整的**——2867 自 08-20 起停止買賣，
那個視窗內真的只有 3 天有成交。系統抓到了它該抓到的全部，沒有漏、沒有錯。

⚠️ **但它不是「停牌等復牌」，是股份轉換下市**，2026-09-01 生效。本筆最初依
「無成交」推論成停牌是**過度解讀**——那是從缺乏資料反推原因，而原因只有公告說得準。

#### 真正的問題：`success` 的定義漏掉了「有沒有拿到該有的資料」

`BackfillHistory` 對「回傳筆數少於預期」**完全無感**：

* 請求成功 → 不計 `failed`。
* `BulkInsert` 寫 3 根也是成功（寫 0 根更是直接 early return `nil`）。
* `f.log.Info("backfill done", …, count)` 有記筆數，但**沒有任何東西比對這個數字**，
  135 行 log 裡短少的那一行不會有人看到。

於是「這檔合法停止買賣所以本來就沒資料」與「上游默默漏給了這檔」
**在系統裡長得一模一樣**，
兩者都會被記成 `success`。

**這次查證後，問題的形狀要講得更精確**：不是「資料正在流失」——TWSE 證實這次一根都沒少。
缺的是**分辨能力**：如果哪天上游真的靜默漏了某檔，系統的反應會與今天**完全相同**
（`success`、零失敗、無 warning），沒有人會發現，直到有人像這次一樣手動盤點。

所以要修的是「**真的漏了的時候會不會有人知道**」，不是那一檔股票，也不是現在有東西壞掉。

#### `2867` 本身要怎麼處理（2026-08-26 更正）

**它會在 2026-09-01 終止上市，不會復牌。** 所以之前寫的「不該退池、等復牌」全部作廢。

但**退池不該是手動一次性動作**，而是要問一個更普遍的問題：
**池成員下市時，有沒有任何機制會讓它停止被回補？** 答案是沒有——
`evaluation_universe.active` 與 `stock_symbols.is_listed` 是**兩個獨立狀態**，
`runEvaluationUniverseSync` 只呼叫 `ListActive()`（`scheduler.go:982`），
不對照主檔。9/1 之後 `stock_symbol_sync` 會把 `is_listed` 設為 false，
但這個池**照樣每天對它發一個 FinMind 請求**，永遠拿回 0 根、永遠記 `success`。

**這是與本筆不同的缺陷**（本筆是「漏了沒人知道」，那是「下市了還在抓」），
已另立 **I-094**。兩者在 `2867` 這個標的上重疊，但要分開修。

#### 為什麼「比對本輪筆數」與「看最新日期」都不夠（2026-08-26 review 修正）

原本列的 A（本輪筆數比對）／B（最新日期落後）／C（TWSE 對照）**都建立在
「缺口會出現在本輪抓取結果裡」這個前提上，而那個前提不成立**。

**T-062 的跳過最佳化把前提拆掉了。** `dropSymbolsSyncedToday`
（`scheduler.go:1050`）以「今天有沒有日 K」為單位**整檔**排除。所以：

| 情境 | 今天有 K 棒 | 五天前有洞 | 結果 |
|---|---|---|---|
| 被跳過的標的 | ✅ | ❌ | **整檔不進 `BackfillHistory`** |

於是三個方案同時失效：

* **A 看不到它**——它根本沒進本輪抓取，沒有筆數可比。
* **B 判定正常**——最新日期是今天。
* **C 不會被觸發**——C 是靠 A/B 觸發才查 TWSE 的。

**而且這個洞不會自己補起來。** 導入跳過之前，每檔每天都重抓 10 天視窗，
任何 10 天內的缺口都會被 upsert 補平；**跳過之後只有尾端缺口自癒，中間的洞永久留著**。
這是 T-062 的副作用，**實作當時沒有識別到**——[`architecture.md`](./architecture.md)
原本寫的「某天漏掉的 K 棒隔天那輪本來就會補回來」已於 2026-08-26 一併更正。

#### 修正後的方向：對整池的**實際日期集合**掃缺洞

偵測不能掛在「本輪抓了什麼」上，必須獨立於回補流程，直接問資料庫：
**池內每一檔在最近 N 個交易日裡，實際有哪幾天的 K 棒？**
與本輪有沒有抓它無關。

⚠️ **視窗單位是「交易日」，不是「日曆日」，而且用自己的設定**（2026-08-26 review 統一）。
文件先前一處寫「最近 N 個交易日」、他處又借用 `evaluation_universe.days=10` 推導出
「10 天視窗」——**兩者跨週末時涵蓋範圍不同**（10 個日曆日只含約 6～7 個交易日）。

* 新增參數 **`lookback_trading_days`，預設 `10`**，屬於 `candle_gap_detection` 自己的區段。
* **不沿用 `evaluation_universe.days`**：那個值控制的是「回補要抓多長」，
  兩者耦合的話，調整回補成本會意外改變偵測範圍。
* 交易日由年度日曆推導（見下），所以連假不會縮短實際涵蓋的交易日數。

#### ⚠️ 「預期交易日集合」要從哪裡來（2026-08-26 review 補，**這是最大的未定案**）

**實際日期集合只能回答「有哪些天」，回答不了「少了哪些天」**——沒有預期集合就沒有缺口。
而預期集合的來源決定了這個機制的盲點在哪：

| 來源 | 成本 | 盲點 |
|---|---|---|
| **靜態年度日曆**（`holidaySchedule/holidaySchedule`，開（休）市日期） | 一年 1 次請求 | 整年預先公布（實測涵蓋 1150101～1151225），**不會像成交端點那樣停滯**；但**它不是單純的休市日清單**，解讀規則見下 |
| **市場層級成交端點**（`exchangeReport/FMTQIK` 或 `MI_INDEX4`） | 每個涵蓋月份 1 次請求 | 會有發布延遲，且**「成功但陳舊」無法自證**——見下 |
| **池內日期聯集** | 0（純 SQL） | ⛔ **全池同日缺漏抓不到**：FinMind 若某天整批漏給，聯集裡本來就沒那天，不會產生候選缺口，也就不會觸發交易所核對 |

**建議：靜態年度日曆推導預期集合，市場層級端點只當交叉驗證**，池內聯集僅作降級退路
（**採用聯集就必須明文接受那個盲點**——它剛好漏掉「上游整批失效」這個最該被抓到的情境）。

##### ⛔ `holidaySchedule` 不能當成「休市日清單」直接扣除（2026-08-26 review 修正）

本筆原本寫「只給休市日，交易日由平日減掉」——**那是錯的**。實測 115 年度的 27 筆
至少有四種列型，**兩個方向都會出錯**：

| 列型 | 實例 | 是不是交易日 |
|---|---|---|
| 放假休市 | `1150101` 開國紀念日、`1150925` 中秋節 | ❌ |
| **正常交易日的標記** | `1150102` 國曆新年**開始交易日**、`1150211` 春節前**最後交易日**、`1150223` 春節後**開始交易日** | ✅ **是交易日** |
| **「市場無交易，僅辦理結算交割作業」** | **`1150212`（四）、`1150213`（五）** | ❌ |
| 週末列 | `1150228`（六）、`1151025`（日） | ❌（本來就不是） |

* **全部扣除** → 1/2、2/11、2/23 這三個**真正的交易日**被錯誤排除，那幾天的缺 K 永遠測不到。
* **只扣「放假」** → **2/12、2/13 會被漏掉**：它們是**平日、不是放假日，但市場無交易**。
  沒有這個端點，「平日 − 國定假日」會把它們算成交易日而誤報。

**所以規則必須是逐列分類，不能是集合相減**：

1. 名稱含「**市場無交易**」或屬放假列 → **非交易日**，扣除。
2. 名稱含「**開始交易**」「**最後交易**」 → **交易日**，不扣除。
3. **未知名稱或格式** → **不要猜**，轉成 `verification_unavailable`。

⚠️ 這是**中文名稱的字串比對，本質脆弱**——TWSE 改字就會落到第 3 條。
第 3 條的降級方向（不可用而非猜測）就是為了讓改字時**壞在明處**。

**必測**：`1/2`（開始交易）、`2/11`（最後交易）、`2/12`（平日但市場無交易）、
`2/23`（春節後開始交易）四個代表案例。

##### 年度日曆取得往年的正確參數是 `date`，**不是 `queryYear`**（2026-08-26 實測）

本筆一度寫成「歷史年度取不到，必須自行持久化」——**那是錯的，而且錯在只試了一個參數名**。
實測 `www.twse.com.tw/rwd/zh/holidaySchedule/holidaySchedule`：

| 參數 | 結果 |
|---|---|
| `date=20250101` | ✅ **24 筆，首筆 `2025-01-01`——歷史年度取得到** |
| `queryYear=114` / `115` / `116` / `2025` | ⛔ **完全被忽略**，四種輸入都回同一份 2026 資料（27 筆） |
| `year=114`、`yy=114`、`queryYear` ＋ `queryType` | ⛔ 同樣被忽略 |
| `openapi.twse.com.tw/v1/holidaySchedule/holidaySchedule`（無參數） | 只回當年 |

⚠️ **`queryYear` 是個陷阱**：它被接受、回 200、資料格式正常，**但年份是錯的**。
用錯參數名的實作會拿當年日曆去判斷去年的交易日，**完全不會報錯**——
這正是本筆要防的失效模式，出現在要用來修它的工具上。

**兩個 endpoint 的 schema 不同**，選定要一致：RWD 是 `日期`/`名稱`/`說明` ＋ ISO 日期；
OpenAPI 是 `Date`/`Name`/`Description` ＋ 民國日期。**建議統一用 RWD**，因為只有它能取往年。

**定案的取得與快取策略**：

* **按需以 `date=<該年任一日>` 取得**，不需要為了跨年而預先持久化——歷史年度隨時取得到。
* **快取以年份為 key**，行程內快取即可（歷史年度不會變）。當年度的日曆理論上也不變，
  但仍建議設 TTL，避免年中修訂（補班補假調整）被快取住。
* **每次使用都要驗證回傳資料的年份與請求的年份相符**——不是拿到 200 就採用。
  這條**不能因為改用 `date` 就省略**：`queryYear` 的行為證明了 TWSE 會對無效參數
  回傳看似正常的資料。
* 目標年度**回傳 0 筆、或年份不符** → `verification_unavailable`。
* **不對「筆數多寡」做完整性判斷**：2026 是 27 筆、2025 是 24 筆，**筆數本來就逐年不同**，
  拿它當完整性指標會是憑空的門檻。可驗證的只有「年份相符且非空」。

##### 「放假列」的可實作定義（2026-08-26 review 補）

「屬放假列」不是機器可判斷的條件。實作要**依序**比對，先做正規化：

**正規化**：去除 HTML（實測 `1150211` 的說明帶 `<br>`）、去除前後空白與換行，再比對。

| 順序 | 條件 | 判定 |
|---|---|---|
| 1 | 名稱或說明含「**開始交易**」或「**最後交易**」 | **交易日**（不扣除） |
| 2 | 名稱含「**市場無交易**」 | 非交易日（實測這類列的說明是**空字串**，所以條件要看名稱） |
| 3 | 說明含「**放假**」或「**補假**」 | 非交易日 |
| 4 | 該日為週六／週日 | 忽略（本來就不是交易日） |
| 5 | 其餘 | ⛔ **`verification_unavailable`，不得猜測** |

**順序不能對調**：規則 1 必須在 3 之前——若某列同時被兩者命中，交易日的語意優先。

##### ⚠️ 上櫃標的的預期集合（2026-08-26 review 補，**不是假設性問題**）

實測池內組成：**上市 101 檔、上櫃 34 檔**。而 `FMTQIK` 的官方名稱是
**「集中市場**每日市場成交資訊」——它**不涵蓋上櫃**。所以四分之一的池目前沒有預期集合來源。

兩條路，**必須擇一明文定案**：

1. **接上 TPEx 的對應端點**（另一個外部相依、另一套格式）。
2. **明文採用「兩市場共用同一份交易日曆」的假設**——台灣兩市場的開休市日實務上一致，
   而 `holidaySchedule` 與 `MI_INDEX4`（每日**上市上櫃跨市場**成交資訊）都可支撐這個假設。
   選這條就要**把假設寫進文件**，並定義例外處理（單一市場臨時休市時會誤判）。

**定案：走 2（兩市場共用日曆）**（2026-08-26 review 後定案，不再停在「傾向」）。

理由：`holidaySchedule` 本來就是市場層級的年度日曆，`MI_INDEX4`（上市上櫃**跨市場**）
也支撐同一份交易日；接 TPEx 端點要多養一個外部相依與一套格式，收益只有「單一市場臨時
休市」這個罕見情境。

**這是具名假設，不是事實**：兩市場開休市日一致是實務慣例。例外處理——
若日後出現單一市場臨時休市，該市場當天會整批被判成「缺漏」，
會落進上面的 aggregate 告警（候選數暴增）而不是逐檔誤報。**那個行為是可接受的降級**，
但要在告警訊息裡看得出「這是整個市場，不是個股」。

##### 上櫃個股核對：**歷史查得到**，端點與參數格式如下（2026-08-26 實測，**修正前一版的錯誤結論**）

⚠️ **前一版寫「上櫃歷史不可驗」是錯的，錯在只盤點了 OpenAPI 就下結論。**
TPEx OpenAPI 的 225 個端點確實都沒有參數、只給當日快照，
但**官方網站另有可依股票與年月查詢的個股歷史端點**：

```
GET https://www.tpex.org.tw/www/zh-tw/afterTrading/tradingStock
      ?code=<symbol>&date=<YYYY/MM/DD>&response=json
```

實測（`1785 光洋科`）：`2026/08/01` → 115年08月 18 筆（含 08/26）；
`2025/03/01` → 114年03月 21 筆。**當月與往月都取得到，能力與 TWSE 的 `STOCK_DAY` 對等。**

| 項目 | TWSE `STOCK_DAY` | TPEx `tradingStock` |
|---|---|---|
| 參數 | `date=YYYYMM01`、`stockNo=` | `code=`、**`date=YYYY/MM/DD`（要完整日期＋斜線，需 URL-encode）** |
| 回傳位置 | `data` | **`tables[0].data`** |
| 欄位 | `[日期, 成交股數, …, 收盤價, …]` | **`[日 期, 成交張數, 成交仟元, 開盤, 最高, 最低, 收盤, 漲跌, 筆數]`**（**成交量單位是「張」不是「股」**） |
| 日期格式 | 民國 `115/08/26` | 民國 `115/08/26` |

⛔ **這個端點有一個與 `queryYear` 同類的陷阱**：參數名或日期格式錯誤時
**不一定回錯誤**。實測 `date=2026/08`、`115/08`、`20260801` 都回
`{"stat":"參數輸入錯誤"}`（好，看得出來），但 **`stkno=1785&d=115/08` 回 HTTP 200 ＋
`data: []` ＋ subtitle「請輸入股票代碼及資料年月」**——**空結果，不是錯誤**。
天真的實作會把它讀成「那個月完全沒有成交」，於是**把整個月都報成缺口**。

**防護（必做）**：每次回應都要驗**回應歸屬**——`stat` 正常、且
`tables[0].subtitle` **包含請求的 symbol 與年月**。不符即 `verification_unavailable`。

⛔ **不得把「`data` 非空」也列為條件**（2026-08-26 review 修正）：
標的**整月合法停止交易**時，參數正確、歸屬正確、`data` 就是空陣列——
那是「該月無成交」的正確答案，不是來源不可用。把它判成 `verification_unavailable`
會讓 `2867` 這類標的**整月都變成驗不了**。

歸屬驗證足以擋掉錯參數：實測 `stkno=1785&d=115/08` 的 subtitle 是
「請輸入股票代碼及資料年月」，**不含 symbol 與年月**，會被歸屬檢查攔下。
**歸屬通過之後，空資料＝該月無成交。**

**因此 breaker 的來源代號是 `tpex_stock_day`**（前一版寫的 `tpex_snapshot` 作廢——
那是基於錯誤結論的設計），上櫃與上市走**對稱**的逐檔逐月流程，共用同一套
`candidate_cap_per_run` 與去重規則。

##### 「那天有沒有成交」的判定條件（兩個來源都要定義）

⚠️ **不能只看「那個日期有沒有出現在回傳裡」**。停止交易或無量的標的可能**仍以零量列存在**，
只看 key 會把合法的無成交誤報成缺口。

**定案的 predicate**：該日期的列存在 **且 成交量 > 0 且 收盤價 > 0**。

* TWSE 看 `成交股數`；TPEx 看 `成交張數`（**單位不同，換算前不要互相比較**）。
* 收盤價那一項是為了擋掉上游的零價列——與 `toStoreCandles` 擋零價是同一個理由。

⛔ **方向不要弄反**（2026-08-26 review 修正，前一版寫反了）。
predicate 判定的是**交易所那天有沒有成交**，缺口的定義是「**交易所有、我們沒有**」：

| 對照源回傳 | 交易所那天有成交？ | 我們沒有 K 棒時的判定 |
|---|---|---|
| 有列且量 > 0 且收盤價 > 0 | ✅ 有 | **缺口** → 告警 |
| 有列但量為 0 | ❌ 沒有 | **正常**，不告警 |
| 有列但價／量為空值或 `--` | ❌ 沒有（無有效成交證據） | **正常**，不告警 |
| **該日期根本沒有列** | ❌ 沒有 | **正常**，不告警 |

前一版寫成「零量／空值正常，**只有缺列才可能是缺口**」——那是相反的：
**缺列代表交易所沒有那天的成交證據**，正是最不該告警的情況。
照那個寫法，`2867` 停止買賣的每一天都會被報成缺口。

* **必要 fixture**：零量列、空值／`--` 佔位列、缺列三種，**三者都判「無成交（正常）」**；
  另加一組「有列且量價皆 > 0」的對照，那才是唯一會告警的形狀（測試矩陣第 5c 條）。

##### 發布延遲與「成功但陳舊」

**市場層級端點比個股端點慢**，有實測。2026-08-26 14:13 同時查：

| 端點 | 當下最後日期 |
|---|---|
| `STOCK_DAY`（個股 2330） | **115/08/26**（已含當日） |
| `FMTQIK`（市場層級） | **115/08/25**（尚未含當日） |
| `MI_INDEX4`（跨市場） | **115/08/25**（同樣尚未含當日） |

池維護跑在 16:00，直接拿市場層級端點當預期集合而不處理延遲，會有兩種相反的誤判。

⛔ **但「取回傳的最後一個交易日當右界」單獨使用是危險的**（2026-08-26 review 修正）：
端點若**回應成功、格式正常，內容卻停滯數日**，稽核視窗會跟著一起倒退，
**所有比它新的缺口都不會被檢查，而且不會被歸類為「驗證不可用」**——
偵測機制本身靜默失效，這正是本筆要防的失效模式在偵測器上重演。

所以必須有一個**不依賴該端點**的基準：

* 用**靜態年度日曆**（`holidaySchedule`）推導「今天為止的預期最後交易日」。
  它整年預先公布，不會停滯。
* 記錄 `source_as_of`（**對照源實際涵蓋到哪一天**）與**落後交易日數**觀測值。
  ⚠️ 前一版把這個值叫 `expected_as_of`，容易與「日曆推導出的應有日期」混淆，改名區分。
* **`lag >= market_stale_days`** 時（**是 `>=`，不是 `>`**——見下方統一定義），**標記
  `verification_unavailable` 並照常告警**，**不得只是縮短掃描視窗然後回報一切正常**。
* 門檻要容忍正常發布延遲（實測當日 14:13 尚未更新），但不能容忍數日。

⚠️ **`market_stale_days` 的「天」是「預期交易日」，不是日曆日**
（2026-08-26 review 補）。用日曆日會在跨週末時誤判：**週五的資料到週一才更新，
日曆日差是 3，但實際只落後一個交易時段**——`market_stale_days=2` 會把正常情況
判成 `verification_unavailable`，每個週一都誤報一次。

**正確算法（含區間端點，2026-08-26 review 修正 off-by-one）**：

```
lag = |{ 交易日 d : source_as_of < d <= expected_last_trading_day }|
```

也就是**左開右閉 `(source_as_of, expected_last_trading_day]`**，用年度日曆數。

* `expected_last_trading_day` 由年度日曆推導（截至今日應有的最後一個交易日）。
* 週一檢查、對照源停在週五 → 區間是 `(週五, 週一]` = {週一} → **`lag = 1`，不是 0**。
  前一版寫 0 是**少算了右端點**；門檻設 `1` 時會永遠判不出過期。
* **比較符號一律是 `lag >= market_stale_days`**（2026-08-26 review 統一）。
  文件他處寫「超過門檻」容易被實作成 `>`，那會讓**剛好等於門檻時漏報**——
  這裡定死為 `>=`，所有相關敘述與測試都以此為準。
* 預設 `market_stale_days = 2` 因此的語意是「**容忍一個交易日的發布延遲**」——
  `lag=1`（今天的還沒發）不算過期，`lag>=2`（連昨天的都沒有）才算。
* 連假同理：春節可能隔 5 個日曆日但 `lag` 仍是 1。

**必測跨週末與跨連假**（測試矩陣第 34、35 條）。

**必測**：`HTTP 200、格式正常、但最後日期陳舊` 這個案例——它與正常回應在型別上無法區分，
沒有測試就一定會漏。

##### 缺漏日期**晚於** `source_as_of` 時的判定（2026-08-26 review 補）

容忍發布延遲，就一定會出現「候選缺漏的日期，對照源那邊還沒涵蓋到」的情況。
**那時「查不到」不等於「無成交」**——對照源根本還沒發布那天。

**定案：判為 `deferred`，既不算缺口也不算失敗。**

| | `deferred` 的處置 |
|---|---|
| 告警 | **不告警**（還沒到能判斷的時候） |
| `last_verified_at` | **不更新**（沒有驗到） |
| `consecutive_failures` | **不增加**（不是失敗，是還沒輪到） |
| `last_attempted_at` | **更新**（確實嘗試過，公平排序要前進） |
| job 狀態 | **不因此 `degraded`**——正常的發布延遲不是異常 |

**`deferred` 有時間上界**：若該日期持續晚於 `source_as_of` 且 **`lag >= market_stale_days`**，
就會落入「成功但陳舊」那條規則變成 `verification_unavailable` ＋ `partial`。
所以它不會變成永遠不處理的黑洞。

判定矩陣（拿**缺少的那幾個確切日期**去對照）：

| 交易所那天有成交？ | 我們有 K 棒？ | 判定 |
|---|---|---|
| 有 | 無 | **上游真的漏了那一天** → 告警 |
| 無 | 無 | 該日無交易（停止買賣／下市／全市場休市） → 正常 |

`2867` 在這個設計下的表現：08-20 之後的日期在交易所端也沒有成交 → 不告警。
**不需要為它做例外**，這正是「用交易所核對」取代「用筆數猜測」的價值。

#### 介面缺口與其現況（狀態欄為準）

⚠️ **這張表混合了「已定案」與「仍待處理」，看狀態欄，不要整表當待辦讀。**

| 缺口 | 現況 | 需要 |
|---|---|---|
| 回補結果沒有日期資訊 | `onSymbol func(symbol string, err error)`（`fetcher.go:182`）只有成敗，沒有筆數也沒有日期 | 若要沿用回補路徑就得擴充簽章；但依上面的方向，**偵測應獨立於回補**，這條可能不必動 |
| 沒有跨標的的缺洞查詢 | `CandleRepo` 只有逐檔的 `GetLatest` / `GetRange`，與單日的 `SymbolsWithCandleOn`（`candle_repo.go:13`） | 新增「一次回傳多檔在區間內的實際日期集合」的查詢，否則 135 檔會變成 N+1 |
| 缺口要**分類**，不能一律忽略零價造成的洞 | `toStoreCandles` 會擋掉價格非正的 K 棒（`fetcher.go:229`，live 出現過 4 根全零日 K），只記 `Warn("skip candle with non-positive price")` | 見下方「缺口的三種分類」。**本筆原本寫「那是刻意的，不該告警」是錯的**——見下 |
| 市場別分流 | 上市與上櫃的個股核對端點不同；`Scheduler` **沒有注入 `StockSymbolRepo`**，拿不到 `market` | **已由 I-094 定案**：`StockSymbolRepo` 新增依 symbol 清單的批次查詢，回傳 `map[string]StockSymbolState{IsListed, Market}`，`Scheduler` 注入該 repo。**本筆直接沿用同一次查詢的 `Market`**，不另外查。**map 缺席＝unknown**：那類標的決定不了端點，落 `verification_unavailable`，**不得預設成上市**（測試矩陣第 20 條） |
| 視窗跨月 | 10 天視窗會跨月；`STOCK_DAY` 是**按月**回傳 | 不能寫成「一檔一個月一次」，要查涵蓋到的**所有**月份 |
| 快取的正確性 | 當月資料每天增加 | 若快取 `(symbol, YYYYMM)` 整月結果，**必須有 TTL 或 fetched-through 標記**，否則快取本身會造成漏判 |
| 對照源不可用 | ✅ **已定案** | 逾時／限流／格式變動一律判 `verification_unavailable`，**不得誤判成「無成交」**；記法見「`verification_unavailable` 要記在哪裡」，來源層級的 breaker 見同節 |

#### 缺口的三種分類（2026-08-26 review 修正）

本筆原本寫「被 `toStoreCandles` 過濾掉的那天是刻意缺少，不該告警」——**那是錯的，
而且與上面的判定矩陣自相矛盾**：交易所那天有成交、我們卻沒有 K 棒，依矩陣就是
「交易所有、系統無」。資料庫確實少了一根**應該存在**的 K 棒，那不是正常狀態。

過濾本身是對的（一根全零的 K 棒會污染 MA / RSI / ATR、zone 建構與 breakout 偵測，
見 `fetcher.go` 的註解），但**「不該寫進去」不等於「不該被知道」**。

正確的做法是分類而不是忽略：

| 分類 | 條件 | 處置 |
|---|---|---|
| `missing_upstream_candle` | 交易所有成交、上游**沒有回傳**那天 | 告警——真的漏了 |
| `invalid_upstream_candle` | 交易所有成交、上游有回傳但**內容無效**被過濾（零價） | 告警，但與上者分開計數；重抓救不了，要查上游品質 |
| 正常無資料 | 交易所那天也沒有成交 | 不告警 |

⚠️ **目前分不出前兩者**：過濾只留在 log（`Warn("skip candle with non-positive price")`），
**資料庫裡沒有任何痕跡**。

**「一張表或一個計數」不等價**（2026-08-26 review 修正）：日期級的分類需要保存
**`symbol` + `timeframe` + `ts` + `reason`**，單純的計數對不回是哪幾天，
拿到告警也無法定位。兩條路：

1. **先不分類**：告警統一成 `upstream_data_gap`，只回報「哪一檔的哪幾天缺了」。
   **不為了分類而擴張 migration**，等真的需要區分成因時再說。
2. **要分類就存完整鍵**：`(symbol, timeframe, ts, reason)`，那就是一張新表 ＋ 三份 migration。

**建議先走 1**：缺口本身就足以觸發調查，而「是沒回傳還是回傳了零價」查 log 就知道
（那是低頻事件，live 至今只出現過 4 根零價 K 棒）。等頻率高到查 log 不划算再做 2。

#### ⚠️ 全池缺漏會把偵測變成對交易所的壓力測試（2026-08-26 review 補）

**最該被抓到的情境，剛好也是請求量最大的情境**：上游某天整批漏給 → 135 檔全部有候選缺口
→ 逐檔、逐月查個股端點 → 單輪最多 **135 × 涵蓋月份數** 個請求打向交易所。
偵測機制本身變成一次壓力測試，而且那是在上游已經出問題的時候。

**先做語意上的短路，再談節流**：全池同一天缺漏是**來源層級**的訊號，
根本不需要逐檔確認——**一次市場層級查詢就回答得了**。所以：

* **aggregate 的判定維度是 `(market, 缺漏日期)`，不是缺口總筆數**（2026-08-26 review 修正）。
  拿「所有 symbol-date 缺口筆數」當門檻是錯的：**不同日期的零星缺口累加起來也會過門檻**，
  被誤判成單日的來源層級缺漏。正確算法是**按 `(market, missing_date)` 分組，
  以該組的 distinct symbol 數 ÷ 當時該市場的有效池大小**。
  比例 **`>=` `aggregate_ratio`** → 對**那一組**發 aggregate／source-wide 告警，
  **不要**展開逐檔請求。逐檔核對只用在沒有觸發 aggregate 的零星缺口上。

  ⛔ **比例還不夠，必須加最小母體門檻**（2026-08-26 review 補）：
  若某市場的有效池只剩 1 檔，那一檔**合法停止買賣**時比例就是 **100%**，
  會被短路成「來源級缺漏」而發出告警——**直接違反「`2867` 這類不得告警」的要求**。
  所以該市場有效池 **< `aggregate_min_symbols`（預設 5）時不套用比例，強制走逐檔核對**，
  由交易所資料逐檔判定。小母體本來就該逐檔驗，成本也可以忽略。
* `(market, symbol, month)` **去重**——同一檔在同一個月的多天缺口只需要一次請求。
* 定義**請求速率**與**單輪請求上限**。⚠️ **「留到下一輪」不足以構成規格**
  （2026-08-26 review 補）：若每輪都按相同的 symbol 排序取前 N 個，前面那些**持續缺口**
  會每輪都占滿配額，**後面的候選永遠輪不到**——那是無限飢餓，而且看起來一切正常。

  **定案：依「最久沒被*嘗試*」排序取前 N 個**（2026-08-26 review 後定案）。

  ⚠️ **排序鍵必須是 `last_attempted_at`，不是 `last_verified_at`**（同日 review 修正）。
  原本寫成「核對完更新時間戳」，但沒定義失敗時更不更新——**若只在成功後更新，
  持續失敗的前 `cap` 個會永遠保持最舊，每輪繼續占滿配額，後面的候選還是永遠輪不到**。
  那正好是這條規則要解決的問題本身。

  | 欄位 | 何時更新 | 用途 |
  |---|---|---|
  | `last_attempted_at` | **每次實際嘗試後，不論成功或失敗都更新** | **公平排序鍵** |
  | `last_verified_at` | 只在**驗證成功**時更新 | 表達「上次真的驗過是什麼時候」，供判讀與告警用 |

  排序在 **Go 端**做：缺席（無 state 列）優先，其次 `last_attempted_at` 由舊到新，
  再以 `symbol` 決勝。**不用 SQL 的 `ORDER BY … NULLS FIRST`**，理由見下一節。

  **可驗證的飢餓避免條件**（2026-08-26 review 修正，補上前提）：
  候選集合固定為 N、每輪處理 `cap` 個，**且滿足下列前提**時，
  **任一候選最遲在第 `ceil(N/cap)` 個排程週期被嘗試**：

  * 該候選的來源 **breaker 未開啟**——breaker 開啟時它是**刻意不被嘗試**的；
  * **`RecordAttempts` 寫入成功**——寫入失敗時排序鍵不會前進，下一輪它還是最舊。

  ⚠️ **前提不能省略**：少了它們，文件承諾的上界會比實作真正保證的更強，
  而「文件說會、實際不會」正是本筆整體在防的那種落差。
  兩個前提失敗時都會讓該輪 `degraded`（見上），所以**停滯是看得見的**，不是靜默的。

  **為什麼不用日期輪替分片**（`corporateActionShardOfDay` 那種）：那適合「固定清單、
  每檔每週至少一次」，但這裡的候選集合**每輪都在變**，hash 分片給不出上面那個上界。

##### 公平排序狀態的持久化落點（2026-08-26 review 補，**定案**）

現況：`evaluation_universe` **沒有**這兩個欄位，`EvaluationUniverseRepo` 也沒有
「依時間排序取候選」或「批次更新時間戳」的方法。只寫「持久化」還不能實作。

**定案：新增獨立的 verification-state 表，不加欄位到 `evaluation_universe`。**

理由：(1) `evaluation_universe` 的 `Upsert` 是**重新匯入 selection report 的常態動作**，
把驗證簿記混進那張表會讓兩件事互相干擾；(2) 候選來源日後可能不只池成員
（watchlist 也有同樣問題），綁在池表上會限制擴充；(3) 池成員的入退池是研究決策，
驗證進度是運維狀態，兩者生命週期不同。

| 項目 | 定案 |
|---|---|
| 表 | `candle_verification_state`，鍵 `(symbol, timeframe)` |
| 欄位 | `last_attempted_at`、`last_verified_at`、`last_result`（`verified` / `gap` / **`deferred`** / `unavailable`）、`consecutive_failures` |
| migration | **三份**（postgres / sqlite / mysql），比照既有慣例 |
| repo | `LoadStates(ctx, timeframe, symbols []string) (map[string]CandleVerificationState, error)` ＋ `RecordAttempts(ctx, []VerificationAttempt)`（**批次**更新，避免 N+1） |

⚠️ **repo 不得帶 `limit`**（2026-08-26 review 修正）。原本寫成
`ListStalestCandidates(…, limit)` 與「repo 回傳全部、Go 端合併排序」**互相矛盾**：
repo 若先截斷，**沒被回傳的既有 state 會被呼叫端誤認成「從未出現」而排到最前面**——
排序直接壞掉，而且壞得很安靜。

`LoadStates` 回傳**指定 symbols 的完整 state map**，`cap` 只能在 Go 端**合併並排序之後**才套用。
| 首次執行 | **沒有列＝最高優先**，不需要預先寫入。⚠️ 排序在 **Go 端合併**，不靠 SQL，見下 |
| 標的 inactive 後重新啟用 | 列保留即可，時間戳很舊 → 自然排在前面優先重驗 |

##### ⛔ 「沒有列＝NULLS FIRST」在 SQL 上不成立（2026-08-26 review 修正）

兩個錯誤疊在一起：

1. **沒有列 ≠ 欄位為 NULL**。`SELECT … FROM candle_verification_state WHERE symbol IN (…)`
   對首次出現的候選**根本不會回傳任何列**，它不會變成排在最前面的 NULL，而是**直接消失**。
2. **`NULLS FIRST` 不是跨 driver 可用的語法**。MySQL 不支援；
   而且 repo 至今**從未用過**這個語法（實查 `backend/` 零筆），
   引入它等於為一個排序需求新增三個 driver 的相容風險（`I-054`：mysql 的 CRUD 從未被驗證）。

**定案：候選排序在 Go 端合併，不靠 SQL。**

```
候選清單（來自池，已在記憶體）
  ← LEFT-merge  LoadStates 查回的 state（map[symbol]State，**全部、無 limit、未排序**）
  → 排序鍵：(有沒有 state：無者優先, last_attempted_at 由舊到新, symbol)
  → 掃描並跳過 breaker 已開的來源，直到實際 attempt 數達 candidate_cap_per_run
```

repo 只負責「把已有的 state 查回來」，**排序與缺席合併是 Go 的事**。
好處是三個 driver 行為一致、不必為排序寫任何 driver 分支，也不需要驗證 mysql 的排序語意。

（若日後真要下放到 SQL：候選要先做成 derived table 再 `LEFT JOIN` state，
排序改用跨庫都能表達的 `ORDER BY (last_attempted_at IS NOT NULL), last_attempted_at, symbol`，
並**三個 driver 各驗一次**。目前不走這條。）

##### `RecordAttempts` 的批次必須先去重（2026-08-26 review 補）

**同一個 symbol 可能同時落在多個 aggregate 分組**（它在兩個不同的 `missing_date`
都缺 K 棒），若每組各產生一筆 `VerificationAttempt`，同一批就會出現**重複的
`(symbol, timeframe)`**。

⚠️ PostgreSQL 的批次 `INSERT … ON CONFLICT DO UPDATE`
**不允許同一個 statement 更新同一列兩次**（`ON CONFLICT DO UPDATE command cannot
affect row a second time`）——那會直接報錯，整批寫入失敗。

**定案：`VerificationAttempt` 的語意是「本輪這個 symbol 的整體結論」**，
由呼叫端**先把該 symbol 跨所有月份／日期的結果彙整成唯一一筆**，再送進 repo——
**不是在 repo 前機械式挑一個最嚴重的字串**（2026-08-26 review 修正）。

只定義 `last_result` 的優先序不夠，四個欄位都要有規則。跨月時很容易出現
「一個月份確認 `gap`、另一個月份 `unavailable`」：

| 欄位 | 合併規則 |
|---|---|
| `last_attempted_at` | 本輪**最後一次** attempt 的時間 |
| `last_verified_at` | **只要本輪有任何一次成功驗證就更新**（`gap` 也算成功驗證）——部分成功仍是成功，記成沒驗過會低估實際進度 |
| `last_result` | 取最嚴重：`unavailable` > `gap` > **`deferred`** > `verified` |
| `consecutive_failures` | **有任何一次成功（`verified` / `gap`）→ 歸零**；**沒有任何成功、且至少一個 `unavailable` → +1**；其餘（只有 `deferred`）**不動** |

⚠️ **不能寫成「全部 `unavailable` 才 +1」**（2026-08-26 review 修正）：
一個月份**請求真的失敗**、另一個月份 `deferred` 時，整體 `last_result` 是 `unavailable`，
但「全部 unavailable」不成立 → **這個 symbol 的 `consecutive_failures` 永遠不會增加**，
於是「這一檔一直驗不成功」在 state 上看不出來。
判準要改成「**沒有任何成功，且至少一個 `unavailable`**」。

**這與來源 breaker 無關，不要混為一談**（2026-08-26 review 再修正）：
`consecutive_failures` 是**逐 symbol** 的狀態，而**來源 breaker 由「實際送出且失敗的請求」
直接累計**（見上方兩層對照表）。所以即使這條 coalesce 規則寫錯，
**breaker 照樣會開**——前一版寫的「breaker 永遠打不開」是**錯的理由**。
這條規則要守的是 **symbol 層級狀態的正確性**，不是 breaker。

**測試要涵蓋兩種**（矩陣第 **27 與 27b** 條）：同一 symbol 出現在兩個 aggregate 日期；
以及**跨月一成功、一 `unavailable`**——後者才驗得到上面四條規則不一致的情況。

##### circuit breaker 是**來源層級**，與 symbol 層級狀態分開（2026-08-26 review 補）

原本把 breaker 的失敗門檻寫在參數表、狀態鍵卻是 `(symbol, timeframe)`——**兩者對不起來**：
同一來源對五個不同 symbol 各失敗一次，逐 symbol 的 `consecutive_failures` 都是 1，
**永遠推導不出「這個來源已連敗五次」**。

| 層級 | 鍵 | 存放 | 內容 |
|---|---|---|---|
| **來源** | `source_id`（`twse_stock_day` / `tpex_stock_day` / `twse_market` / `twse_calendar`） | **行程內記憶體**（runtime 安全閥，不需要跨重啟保存；重啟後重新探測是可接受的） | 連續失敗數、breaker 開啟時間 |
| **標的** | `(symbol, timeframe)` | `candle_verification_state` 表 | 公平排序與最後結果 |

**計數規則要分清楚「驗證失敗」與「驗證出缺口」**：

| 結果 | `last_attempted_at` | `last_verified_at` | 標的 `consecutive_failures` | 來源 breaker 計數 |
|---|---|---|---|---|
| 驗證成功、無缺口 | 更新 | **更新** | 歸零 | 歸零 |
| **驗證成功、確認有缺口** | 更新 | **更新**（**這是成功的驗證**，只是結論是壞消息） | 歸零 | 歸零 |
| 驗證不可用（**請求真的失敗**：逾時／限流／格式變動） | 更新 | 不動 | +1 | **+1** |
| 驗證不可用（**能力限制**：該來源根本不提供這種查詢） | 更新 | 不動 | +1 | **不加**——見下 |
| `deferred`（對照源尚未涵蓋該日期） | 更新 | 不動 | **不加** | **不加** |

⚠️ **只有「真的送出請求且失敗」才累加來源 breaker**（2026-08-26 review 補）。
能力限制與 `deferred` **沒有對該來源發出任何失敗請求**，把它們算進去會讓
「幾個查不了的候選」直接打開 breaker，**連原本驗得到的資料也一起被跳過**——
用一個已知限制去癱瘓一個健康的來源。

**breaker 開啟時的選取演算法**（2026-08-26 review 修正）。原本寫「在選取階段跳過、
其他來源遞補」，**但那在「先取前 cap 個、再逐一請求」的流程下不成立**：
若前 20 個大多是 TWSE，處理到第 5 個時 TWSE 才斷路，剩下 15 個只會被跳過，
**清單之外的 TPEx 候選不會自動遞補**，該輪等於只驗了 5 個。

正確的作法是**掃描而不是預先截斷**：

1. 先算出**完整**的排序清單（不截斷）。
2. **逐項掃描**：該候選的來源 breaker 已開 → **跳過**（不計入 cap、不更新
   `last_attempted_at`），繼續往後掃。
3. **實際 attempt 數**達到 `candidate_cap_per_run` 才停止——cap 算的是「真的送出去的」，
   不是「掃過的」。
4. **breaker 在本輪中途開啟**時，後續同來源的候選同樣被跳過並繼續往後掃描，
   由其他來源的候選補滿 cap。

被跳過的候選會在下一輪繼續排在前面（仍待驗證，正確）。

⛔ **只要有任何候選因 breaker 被跳過，該輪就是 `degraded=true` → `partial`**
（2026-08-26 review 修正）。原本只寫「整輪完全沒驗到才記 `partial`」——**那留了一個洞**：
TWSE breaker 已開、部分 TWSE 候選被跳過，但 TPEx 候選驗成功，整輪會顯示 `success`，
又變成「有一部分驗不了但看起來正常」，正是本筆整體要消滅的形狀。

`error` 欄要記**來源與被跳過的數量**（例如 `verification_unavailable: twse_stock_day breaker open, 12 檔未驗`），
與其他原因以 `"; "` 合併。

| 批次去重 | **`RecordAttempts` 前必須依 `(symbol, timeframe)` coalesce**——見下 |
| 更新失敗 | **不中斷該輪**，但記 Error 並讓該輪 `degraded`——若更新永遠失敗，排序會停滯而退回飢餓，那必須看得見 |
* **circuit breaker**：連續失敗時停止對該來源發請求，整批標 `verification_unavailable`。

#### 參數的預設值與設定位置（2026-08-26 review 補，**定案**）

這些參數會直接改變告警結果，不能只寫方向。**全部收在 `config.yaml` 的新區段
`candle_gap_detection` 下**（比照 `evaluation_universe` / `sr_analysis` 的既有形狀，
含 `enabled`，**預設關閉**），並比照既有慣例支援環境變數覆寫：

| 參數 | 預設 | 依據 |
|---|---|---|
| `enabled` | `false` | 新機制一律預設關閉，比照 `evaluation_universe` / `sr_analysis` |
| `aggregate_ratio` | `0.5` | 單一 `(market, date)` 缺漏比例**達到（`>=`）**此值才短路。門檻是 `>=` 不是 `>`，明文定死避免實作各自解讀 |
| `aggregate_min_symbols` | `5` | **最小母體**：該市場當時有效池不足此數時**強制走逐檔核對**，不套用比例（見下） |
| `candidate_cap_per_run` | `20` | **單位是「候選標的數」，不是 HTTP 請求數**（見下）。池 135 檔時 `ceil(135/20)=7` 輪覆蓋完，約一週 |
| `timeout_sec` | `300` | 整輪偵測的上限；非正值退回常數預設，超過 hard cap `900` 截斷 |
| `lookback_trading_days` | `10` | 往回檢查幾個**交易日**（不是日曆日）；**與 `evaluation_universe.days` 解耦** |
| `request_interval_ms` | `500` | 對交易所端點保守節流（2 req/s），與 FinMind 的 5 req/min 無關 |
| `market_stale_days` | `2` | 市場層級端點實測當日 14:13 尚未更新，容忍一個交易日的發布延遲，不容忍數日 |
| `calendar_ttl_hours` | `24` | 歷史年度不會變；當年度容忍年中補班補假修訂 |
| `breaker_failures` | `5` | 連續 5 次失敗即停止對該來源發請求 |
| `breaker_cooldown_min` | `60` | 冷卻後**自動**恢復；恢復條件是時間到，不是人工介入 |

⚠️ **`candidate_cap_per_run` 的單位是候選數，不是請求數**（2026-08-26 review 修正）。
原本命名為 `request_cap_per_run` 與實際演算法不一致：去重是
`(market, symbol, month)`，而 **10 天視窗跨月時同一個標的要兩次請求**，
選 20 個候選最多會發出 40 次——直接違反那個名字的承諾。

兩條路擇一，**定案走命名修正**：

* ✅ **改名 `candidate_cap_per_run`**，並明文承認
  **HTTP 請求數上限 = `candidate_cap_per_run` × 該輪視窗涵蓋的月份數**。
  ⚠️ **不是固定的 40**——那只在預設 `lookback_trading_days=10` 時成立；
  `lookback_trading_days` 可設到 60，跨越的月份數會跟著變，上限要**依實際視窗計算**。
  由 `request_interval_ms` 節流。**公平上界 `ceil(N/cap)` 維持成立**，因為它算的是候選數。
* ❌ 維持請求數 cap、依每個候選的月份成本扣 budget——那會讓每輪處理的候選數浮動，
  `ceil(N/cap)` 的上界不再成立，公平性反而變得無法論證。

**十一個參數都要定義合法範圍與非法值處置**（2026-08-26 review 補）。
只替 `timeout_sec` 定 fallback 是不夠的——`candidate_cap_per_run=0` 會讓偵測
**一個候選都不處理卻仍回報 `success`**，`lookback_trading_days=0` 會產生**空視窗**
（沒有預期日期＝沒有缺口＝永遠正常），兩者都是「看起來成功」的靜默失效。

| 參數 | 合法範圍 | 非法值處置 |
|---|---|---|
| `enabled` | bool | **型別解析失敗＝啟動失敗**（config 層既有行為，見下節）。這一列沒有「超出範圍」的情形 |
| `aggregate_ratio` | `(0, 1]` | 退回預設 `0.5` ＋ Error log。**`0` 會讓任何缺口都短路**、`>1` 則永不短路 |
| `aggregate_min_symbols` | `>= 1` | 退回預設 `5` ＋ Error log |
| `candidate_cap_per_run` | **`1 ~ 100`** | 退回預設 `20` ＋ Error log；**超過 100 截到 100**。**不得允許 `0`**（零驗證卻回報成功），也**不得無上限**——誤設大值會讓單輪請求量暴增，與「cap ＋ 間隔避免對交易所造成壓力」的風險對策直接衝突 |
| `lookback_trading_days` | `1 ~ 60` | 退回預設 `10` ＋ Error log；超過 60 截到 60 |
| `timeout_sec` | `> 0`，hard cap `900` | 非正值退回 `300`；超過截到 `900` |
| `request_interval_ms` | **`>= 100`** | 小於 100（含 `0`）一律**截到 100** ＋ Error log。**`0` 不是合法值**——那等於完全取消對交易所的節流，與本筆的風險對策矛盾。要壓測請改程式，不要靠設定關掉安全限制 |
| `market_stale_days` | `>= 1` | 退回預設 `2`。**單位是「預期交易日」，不是日曆日**——見下 |
| `calendar_ttl_hours` | `>= 1` | 退回預設 `24` |
| `breaker_failures` | `>= 1` | 退回預設 `5`（`0` 會讓 breaker 永遠開著） |
| `breaker_cooldown_min` | `>= 1` | 退回預設 `60` |

##### 正規化發生在**哪一層**（2026-08-26 review 補，實查後定案）

「退回預設 ＋ 記 Error」**不能放在 config 層**：

* `config.Load()` 在 `viper.Unmarshal` 失敗時**直接回傳 error**
  （`config.go:326`），`main.go` 收到就 `os.Exit(1)`。
* **`internal/config` 沒有 logger**（實查零筆 `zap`），根本記不了 Error。

所以要**區分兩種非法**，各自在不同層處理：

| 類型 | 例 | 發生層 | 行為 |
|---|---|---|---|
| **型別解析失敗** | `ENABLED=abc`、`TIMEOUT_SEC=x` | config 載入 | **維持既有行為：啟動失敗**。這是整個 config 層的一致行為，本筆不為一個新區段破例 |
| **解析成功但超出範圍** | `candidate_cap_per_run=0`、`aggregate_ratio=1.5` | **scheduler setter** | **正規化成預設／截到界限 ＋ 記 Error log** |

**先例是準確的，但要指對層**：`corporateActionCron()`（`scheduler.go:815`）
就是這個形狀——空值退預設、**非空但 `cron.ParseStandard` 失敗時退回預設並記 Error**，
再把已驗證的值交給 `AddFunc`。（`AddFunc` 那裡的錯誤處理是第二道保險，
不是主要路徑——正常情況它拿到的一定是已經正規化過的合法值。）

**本筆比照**：`SetCandleGapDetection` 收到 cfg 後**立刻正規化並記 Error**，
之後的程式碼一律信任已正規化的值，不再各自防禦。

**這些是待調的初值，不是實測最佳值**——除了 `market_stale_days` 有 14:13 那次觀測支撐，
其餘都是保守猜測。**十一個參數要納入兩組測試**（比照既有的 config 預設值測試）：

1. **預設值測試**——沒設定時拿到表上的預設。
2. **非法值測試**——解析成功但超範圍時，**正規化的結果、Error log、以及正規化之後的
   行為都要斷言**（測試矩陣第 36 條）。只驗預設值等於完全沒有守到這次新增的規格。

避免日後改預設值或改正規化邏輯時沒人發現。

#### `verification_unavailable` 要記在哪裡（2026-08-26 review 補）

這個狀態目前只定義了「不得記成已證實資料缺漏」，**但沒有定義它出現在哪**。
只寫 log 的話，`/scheduler/status` 仍會顯示 `success`——**那等於這個機制在最需要
被看見的時候是隱形的**，與本筆要解的問題同一種形狀。

**定案：用 `finishRunDegraded` 的 `degraded` 旗標記成 `partial`**，並在 `error` 欄
帶上原因與受影響的標的數。

理由是既有語意剛好吻合：`degraded` 的定義就是「**這輪跑完了、名單內也可能零失敗，
但名單本身不完整**」（見 `scheduler.go` 的註解與 [`api-reference.md`](./api-reference.md)
的 `partial` 第二種成因）。「驗證不可用」正是「這輪的結論不完整」，
不是「有標的失敗」。

**明確不做的兩件事**：

* **不計入 `symbols_failed`**——那個欄位的單位是「失敗的標的數」，
  驗證不可用不是標的失敗（I-092 才剛把這條分母語意收斂好，不要立刻破壞它）。
* **不只記 log**——理由見上。

##### 「確認為真正缺口」時的 job 狀態（2026-08-26 review 補）

上面只定義了「驗不了」。**「驗過了、而且真的缺」更不能停在 `success`**——
那是本筆最核心的情境。定案與 `verification_unavailable` 相同的機制、不同的訊息：

| 結論 | `job_runs.status` | `error` 欄 | `symbols_failed` |
|---|---|---|---|
| 驗證不可用 | `partial`（`degraded=true`） | `verification_unavailable: <原因>, N 檔` | 不變 |
| **確認為真正缺口** | **`partial`（`degraded=true`）** | **`upstream_data_gap: N 檔 / 日期 …`** | **不變** |
| 一切正常 | `success` | 空 | 不變 |

**同樣不計入 `symbols_failed`**：那些標的的**回補本身是成功的**（上游回什麼就寫什麼），
失敗的是「上游該給而沒給」。把它算成標的失敗會讓 `failed >= total` 的推導在
整批缺漏時把整輪記成 `failed`，而那輪其實跑完了。

⚠️ **`error` 欄必須合併、不得覆蓋**（2026-08-26 review 補）：
`finishRunDegraded` **原樣傳遞 `lastErr`，不會自己補上降級原因**——
它只改 `status`（見 `scheduler.go:365`）。而且 job 已經是 `partial` / `failed` 時，
`degraded` 旗標不會再做任何事。所以**呼叫端要自己把既有錯誤與缺口原因用既定的
`"; "` 格式串起來**（比照 `corporate_action_sync` 的
`strings.Join(errParts, "; ")`，`scheduler.go:1174`），
**不能直接把 `lastErr` 換成缺口訊息**，否則原本的抓取錯誤會被吃掉。

#### 實作必須涵蓋的測試矩陣（2026-08-26 review 補）

這個機制的失效模式幾乎都是靜默的，**每一條沒被測到的路徑等同沒有保護**：

| # | 情境 | 期望 |
|---|---|---|
| 1 | **全池同日缺漏** | 要報。用池內聯集當預期集合時**測不出來**，正是選日曆來源的理由 |
| 2 | 單檔中段缺漏（跳過最佳化下的主要盲點） | 要報 |
| 3 | 合法停止買賣（`2867` 形狀） | **不報**——交易所那幾天也沒有成交 |
| 4 | 視窗跨月 | 兩個月份都要查到，不能只查起始月 |
| 5 | **上櫃標的、當月缺漏** | 用 `tradingStock?code=&date=YYYY/MM/DD` 驗，與上市走對稱流程、共用同一套 cap 與去重 |
| 5b | **上櫃標的、往月缺漏** | 用 `tradingStock?code=&date=YYYY/MM/DD` 驗（能力與 TWSE 對等），**不得**判成不可驗 |
| 5c | **零量列／空值列／缺列** | **三者都判「無成交（正常）」**——只有「有列且量價皆 > 0」才是缺口 |
| 5d | **參數錯誤但回 200 空結果**（TPEx `stkno=`／TWSE `queryYear=`） | `verification_unavailable`，**不得**把空結果讀成「整月無成交」 |
| 6 | **市場層級端點陳舊**（HTTP 200、格式正常、日期停滯） | 標 `verification_unavailable` 並告警，**不得回報正常** |
| 7 | 個股核對端點失敗（逾時／限流／格式變動） | 標 `verification_unavailable`，**不得誤判成「無成交」** |
| 8 | 零價被過濾造成的缺口 | 要報（見上方分類），不得因為「是我們自己擋掉的」而忽略 |
| 9 | **年度日曆裡的正常交易標記日**（`1/2`、`2/11`、`2/23`） | **不得**被當成休市扣除 |
| 10 | **平日但「市場無交易」**（`2/12`、`2/13`） | 必須扣除，否則會誤報那兩天缺 K |
| 11 | 年度日曆**回傳錯年／缺年／未知列型別** | `verification_unavailable`，**不得猜測** |
| 12 | **跨年視窗** | 兩個年度的日曆都要載入 |
| 13 | **全池缺漏** | 先發 aggregate 告警並套用單輪請求上限，**不得**直接展開 135 檔逐檔請求 |
| 14 | `verification_unavailable` 的記錄方式 | **不得**被記成「已證實資料缺漏」；且必須讓 `job_runs` 記成 `partial`，**不得停在 `success`** |
| 15 | **確認為真正缺口**時的 job 狀態 | `partial` ＋ `error` 帶 `upstream_data_gap`，**不得停在 `success`**、也不得計入 `symbols_failed` |
| 16 | job 已有其他錯誤時再加上缺口原因 | 兩者以 `"; "` 合併，**原錯誤不得被覆蓋** |
| 17 | **年度日曆取回錯年**（`queryYear` 無效的實測情境） | 驗證年份後判 `verification_unavailable`，**不得**拿當年日曆去判去年 |
| 18 | **候選數連續多輪超過單輪上限** | 固定候選集合 N、單輪上限 `cap` 時，**任一候選最遲在第 `ceil(N/cap)` 輪被嘗試**（可直接斷言，不是「不會飢餓」這種測不到的敘述） |
| 19 | **前 `cap` 個每輪都驗證失敗** | 其餘候選**仍須在同一個上界內被嘗試**——這條專門守 `last_attempted_at`（而非 `last_verified_at`）當排序鍵這個決定 |
| 20 | 主檔查無該標的（`Market` 也不存在） | 落 `verification_unavailable`，**不得**預設成上市端點 |
| 21 | **不同日期的零星缺口累加** | **不得**觸發 aggregate 告警；aggregate 只看單一 `(market, date)` 分組的比例 |
| 22 | **首次出現的候選**（`candle_verification_state` 無列） | 必須排在最前面被選中，**不得**因為 SQL 查不到而消失 |
| 23 | 某來源 breaker **執行前已開啟** | 該來源的候選在掃描時被跳過、不佔 cap、**不更新 `last_attempted_at`**；其他來源照常。**該輪即使其他來源全部成功也必須 `partial`** |
| 24 | **`StatesBySymbols` 整體失敗** | I-094 全量回補、I-091 整批 `verification_unavailable` ＋ `partial`，**兩者一次斷言** |
| 25 | **連續兩輪同樣的全池缺漏** | **兩輪都要回報**（缺口仍存在＝仍該可見），且**兩輪都要寫回 state**——這條守的是「不要為了降噪把持續存在的問題藏起來」，以及排序簿記有前進 |
| 26 | **breaker 在本輪中途開啟** | 後續同來源候選被跳過、**由其他來源候選補滿 cap**，不是「該輪只驗了 5 個就結束」 |
| 27 | **同一 symbol 同時出現在兩個 aggregate 日期** | `RecordAttempts` 收到的批次**不得有重複鍵**（否則 postgres 直接報錯整批失敗） |
| 27b | **同一 symbol 跨月一成功、一 `unavailable`** | `last_result=unavailable`，但 `last_verified_at` **要更新**、`consecutive_failures` **要歸零**——部分成功仍是成功 |
| 28 | `LoadStates` **不截斷** | 給定 N 個 symbol、其中 M 個有 state，必須回傳全部 M 筆——**截斷會讓既有 state 被誤判成「從未出現」** |
| 29 | **`evaluation_universe` 早退的四條路徑** | 見「掛尾端的早退與啟用組合」一節，逐條斷言 |
| 29b | **四項必要依賴各缺一項** | 偵測一律**不註冊**（`disabled`）＋ Error log；**特別是 `CandleRepo=nil`**——parent 仍照常註冊並走全量回補，偵測不得被標成已註冊 |
| 29c | **偵測 `enabled=false`、依賴齊全、parent 正常註冊** | `evaluation_universe_sync` 標記為已註冊，**`candle_gap_detection` 不得被標記**（`/scheduler/status` 應顯示 `disabled` 而非 `never_run` ＋ `stale`） |
| 29d | **parent cron 字串打錯**（`AddFunc` 失敗） | **兩個 job 都不標記**——parent 沒註冊成功，掛在它尾端的偵測也不可能執行 |
| 30 | **某市場有效池 < `aggregate_min_symbols`** | **不套用比例、強制逐檔**——單檔市場的合法停止買賣**不得**被短路成來源級告警 |
| 31 | **缺漏日期晚於 `source_as_of`** | 判 `deferred`：不告警、不更新 `last_verified_at`、不加失敗計數、**該輪不因此 `degraded`** |
| 32 | `deferred` 達到 `market_stale_days` | **`lag == market_stale_days` 就要升級**（邊界值必測，`>` 的實作會在這裡漏報）成 `verification_unavailable` ＋ `partial`，**不得**永遠停在 deferred |
| 33 | **視窗跨週末／連假** | 涵蓋的是 `lookback_trading_days` 個**交易日**，不是日曆日 |
| 34 | **跨週末**（對照源停在週五、週一檢查） | `lag` **必須等於 1**（區間 `(週五, 週一]`），日曆日差 3 是誤導；預設門檻 2 → **不判 `verification_unavailable`** |
| 35 | **跨連假** | 同上，以年度日曆數出的預期交易日差為準 |
| 36 | **每個參數的非法值**（十一項各一組） | 解析成功但超範圍 → **正規化成預設／截到界限 ＋ 記 Error log**，且**後續行為與合法值一致**；`candidate_cap_per_run=0`、`lookback_trading_days=0`、`request_interval_ms=0`、`aggregate_ratio=1.5` 為必測 |
| 37 | **型別解析失敗**（`ENABLED=abc`） | **啟動失敗**（維持 config 層既有行為），**不得**被靜默當成 `false` |
| 38 | **coalesce：一個月 `unavailable` ＋ 一個月 `deferred`** | `last_result=unavailable`、`last_verified_at` 不動、**`consecutive_failures` +1**（沒有任何成功且至少一個 unavailable）；**同時斷言那個真的失敗的來源，其 breaker 計數有 +1**——兩層是獨立累計的 |
| 39 | **coalesce：只有 `deferred`** | `last_result=deferred`、**`consecutive_failures` 不動**——正常延遲不該把 breaker 推開 |

6 以後是後續數輪 review 補上的。它們的共同點是**失敗時看起來像成功**——
第 14、15、17 條尤其要小心：把「驗不了」寫成「驗過了」、把「確認有缺口」記成 `success`、
或拿錯年的日曆去判斷，都會讓整個機制的結論失去意義而**不會有任何東西報錯**。

#### aggregate 短路之後也要寫 verification state（理由已更正）

⚠️ **原本的理由「寫了 state 就不會重複告警」是錯的**（2026-08-26 review 修正）。
資料流是**先算差集、再分組判 aggregate**，而 state 排序在那之後——
第二輪資料庫仍然缺同一根 K 棒，**差集與比例完全相同**，
`last_attempted_at` 有沒有變新**完全不影響這一步**。而且 `(symbol, timeframe)` 的 state
**沒有保存「驗過哪個 missing date」**，本來就無法對特定缺口去重。

**關於重複告警，定案：接受持續缺口每輪都回報。**

* **未修復的缺口本來就該持續可見**——把它壓成一次性通知，等於讓一個仍然存在的問題消失。
* 不引入 gap identity（`(symbol, timeframe, missing_date)` 或
  `(market, missing_date)` 的 alert fingerprint）：那是另一張表與另一套生命週期，
  為了降噪而擴張 schema 不划算。**若日後噪音真的難以忍受，再回來做 fingerprint。**
* ⚠️ **不能用 `last_verified_at` 推測「這個缺口已經報過了」**：舊日期的 K 棒
  可能在驗證之後才被刪掉，那時 state 是新的、缺口卻是新出現的。

**那為什麼還要寫 state？理由換成公平排序的簿記**：aggregate 短路時那批 symbol
沒有走逐檔路徑，但它們**日後仍會是候選**（例如缺口部分修復、aggregate 不再觸發）。
不更新 `last_attempted_at` 的話，它們會永遠排在最前面並佔滿 cap，
把其他候選餓死——那是第 19 條測試要防的同一件事。

| 欄位 | 值 |
|---|---|
| `last_attempted_at` | 更新 |
| `last_verified_at` | **更新**（這是成功的驗證） |
| `last_result` | `gap` |
| `consecutive_failures` | 歸零 |

⚠️ **前提是市場層級查詢本身成功**。若它 `verification_unavailable`，
就**不能**用這條——那時什麼都沒驗到，該組 symbol 維持原狀、該輪記 `partial`。

#### 修改計畫書（2026-08-26 補齊 CLAUDE.md 要求的必要元素）

**修改目標**：讓「上游靜默漏資料」在發生時會被發現，且**能與「該日本來就沒有交易」分開**。

**不做的範圍**

* 不改 `BackfillHistory` 與 `dropSymbolsSyncedToday` 的行為——偵測**獨立於回補流程**。
* 不自動補抓缺口。本筆只負責**發現與回報**；要不要補、怎麼補是另一個決定。
* 不動 `symbols_failed` 的語意（I-092 才收斂完）。
* **不新增前端功能**——只在既有狀態頁補一個中文標籤，沿用既有的 `partial` / `success` 對照。

**受影響檔案與模組**

| 檔案 | 變更 |
|---|---|
| `migrations/{postgres,sqlite,mysql}/074_create_candle_verification_state.sql` | 新表（下方 schema） |
| `store/candle_verification_repo.go`（新） | `LoadStates`（**無 limit、不排序**）/ `RecordAttempts`（批次，鍵須先去重） |
| `store/candle_repo.go` | 新增「多檔在區間內的實際日期集合」查詢（單一查詢，比照 `SymbolsWithCandleOn` 的 `sqlx.In`） |
| `store/stock_symbol_repo.go` | `StatesBySymbols`（**I-094 定案，兩筆共用**） |
| `market/exchange_reference.go`（新） | **四個對照查詢**：年度日曆（`date=YYYYMMDD`）、市場層級成交、TWSE 個股（`STOCK_DAY`）、**TPEx 個股（`tradingStock?code=&date=YYYY/MM/DD`）**；＋ 來源層級 breaker（`twse_calendar` / `twse_market` / `twse_stock_day` / `tpex_stock_day`）。**四種 schema、回傳位置與量的單位各自解析**（TPEx 在 `tables[0].data`、成交量單位是「張」），不得假設同構 |
| `scheduler/scheduler.go` | 掛在 `runEvaluationUniverseSync` 尾端、**寫獨立的 `job_runs` 紀錄**（比照 `sr_zone_verify`）；新增 setter `SetCandleGapDetection(verification, reference, cfg)` |
| `api/handler/scheduler.go` | `knownSchedulerJobs` 加 `candle_gap_detection`；`jobStaleThreshold` 設 80 小時 |
| `frontend/src/routes/Scheduler.svelte` | `jobLabel` 加中文名（**只加標籤，狀態對照表不動**——沿用既有的 `partial` / `success`） |
| `config.yaml` ＋ 三份 compose ＋ `deploy.sh` | 新區段 `candle_gap_detection`（預設關閉） |
| `internal/config/config.go` ＋ config 測試 | 新 struct、env 覆寫、**預設值測試**（十一個參數都要） |
| `cmd/server/main.go` | 建 repo／client 並呼叫 `SetCandleGapDetection`（在 `go sched.Start()` 之前） |
| `scheduler` / `store` / `market` 的測試 | 各層的單元測試 ＋ **測試矩陣全部條目** |
| `docs/api-reference.md` | `candle_gap_detection` 的 job 語意、`partial` 的第三種成因 |
| `docs/database-schema.md` | 新表的欄位與值域說明 |
| `scripts/test-{mysql,postgres}-migrations.sh` | 074 要能通過兩支 migration 驗證腳本（含 down-to-0） |

**migration 編號與 schema**

下一個可用編號是 **074**（三份 migration 目前都停在 073）。

```sql
CREATE TABLE candle_verification_state (
    symbol               VARCHAR(10)  NOT NULL,
    timeframe            VARCHAR(5)   NOT NULL,
    last_attempted_at    <TS>,                    -- 可為 NULL：從未嘗試過
    last_verified_at     <TS>,                    -- 可為 NULL：從未成功驗證過
    last_result          VARCHAR(12)  NOT NULL,   -- verified / gap / deferred / unavailable
    consecutive_failures INTEGER      NOT NULL DEFAULT 0,
    PRIMARY KEY (symbol, timeframe)
);
```

**`<TS>` 依方言替換**（實查既有 migration 的慣例，三者不同）：

| driver | 型別 |
|---|---|
| postgres | `TIMESTAMPTZ` |
| sqlite | `DATETIME` |
| mysql | `DATETIME(0)` |

**`last_result` 沒有 `DEFAULT`，寫入時必填**（2026-08-26 review 修正）。
原本寫 `DEFAULT ''` 等於偷偷引入第四種狀態——宣告只有三個值卻讓空字串合法，
那是自相矛盾。加
`CHECK (last_result IN ('verified','gap','deferred','unavailable'))`
（三方言都支援 CHECK；mysql 8.0.16 起才強制執行，這點要在註解寫明）。

⚠️ **`deferred` 必須在值域裡**（2026-08-26 review 修正）：規格要求 `deferred` 也更新
`last_attempted_at`，而 `last_result` 是 `NOT NULL` ＋ CHECK——
**首次就 deferred 的 symbol 沒有舊列可保留，`RecordAttempts` 根本 insert 不進去**。
把它漏掉會讓「正常發布延遲」變成寫入失敗。

⚠️ **編譯期長度斷言擋不了值域**：`job_run_repo.go` 的 `jobRunStatusMaxLen` 只防
「字串超過欄寬」，**不能限制資料庫裡的合法值**。值域要靠 `CHECK`，長度靠斷言，
兩者是不同的守門，不要混為一談。

**不建索引**（2026-08-26 review 修正）：排序已定案在 Go 端做，
原本規劃的 `(timeframe, last_attempted_at)` 索引**不支撐任何實際查詢**。
`LoadStates` 是 `WHERE timeframe = ? AND symbol IN (?)`（`sqlx.In` ＋ `Rebind`，比照
`SymbolsWithCandleOn`；**不需要寫成 `(symbol, timeframe) IN (…)`**），由 `PRIMARY KEY` 支撐；
批次 upsert 也走主鍵。多建一個沒人用的索引只是寫入成本。

三份 migration 同步，且 Down 區塊要能跑（見 `development-workflow.md`）。

**實作順序與資料流**

1. migration 074 ＋ `candle_verification_repo`（可獨立測）。
2. `StatesBySymbols`（**I-094 先做**，本筆直接用它的 `Market`）。
3. `CandleRepo` 的日期集合查詢。
4. `exchange_reference`：年度日曆 → 市場層級 → **TWSE 個股（`STOCK_DAY`）→ TPEx 個股（`tradingStock`）**，
   四者各自獨立測，用 httptest 假伺服器；**每個都要有「參數錯誤但回 200 空結果」的 fixture**。
5. 串起來，預設關閉（執行位置見下）。

**執行位置定案：掛在 `runEvaluationUniverseSync` 尾端，但寫「獨立的 `job_runs` 紀錄」**
（2026-08-26 review 後定案，不再是「新 job 或掛尾端」二選一）。

**比照 `sr_zone_verify` 的既有 pattern**：它沒有自己的 cron，在 `RunDailyClose` 尾端
無條件執行，但寫獨立的 `job_runs` 紀錄——**失敗不影響 `daily_close` 的判定**
（見 `api/handler/scheduler.go` 的 `knownSchedulerJobs` 註解）。

| 面向 | 定案 |
|---|---|
| cron | **沒有自己的 cron**，跟著 16:00 那輪跑（缺口偵測本來就該在回補之後） |
| `job_runs` | **獨立紀錄 `candle_gap_detection`**——偵測判 `partial` 時**不會**污染 `evaluation_universe_sync` 的狀態，兩者要分得開 |
| `/scheduler/status` | 加進 `knownSchedulerJobs`，`jobStaleThreshold` 設 **80 小時**（比照同樣平日跑一次的 job） |
| 防重入 | 跟在 `universeSyncRunning` 內，不需要另一個旗標 |
| timeout | 用自己的 `context.WithTimeout`，**不沿用回補的 ctx**——回補逾時不該讓偵測連帶失效 |
| `enabled=false` | 完全不執行也不寫 `job_runs`（`/scheduler/status` 會顯示 `disabled`） |
| 注入 | **獨立 setter** `SetCandleGapDetection(verification store.CandleVerificationRepo, reference market.ExchangeReference, cfg config.CandleGapDetectionConfig)`，比照 `SetSRAnalysis` / `SetSRZoneVerify`。`StockSymbolRepo` 由 I-094 的 `SetEvaluationUniverse` 注入，本筆共用同一個欄位 |

##### 掛尾端的早退與啟用組合（2026-08-26 review 補，**每一條都要定案**）

`runEvaluationUniverseSync` 有**四條早退路徑根本到不了尾端**（`scheduler.go:970` 起）。
定案如下：

| parent 的情況 | 偵測要不要跑 | `candle_gap_detection` 的 `job_runs` |
|---|---|---|
| `evaluationUniverse` repo **未注入** | 不跑 | **不建立紀錄**（等同未啟用） |
| **防重入跳過**（上一輪還在跑） | 不跑 | **不建立紀錄**——與 parent 一致，那輪根本沒開始 |
| `ListActive` **失敗** | **要跑到「建立紀錄」為止** | **建立紀錄並記 `verification_unavailable` ＋ `partial`**：連候選清單都拿不到，等於「這輪驗不了」，那正是要看得見的狀態 |
| **空池**（0 檔） | 跑 | `success`，`symbols_total = 0`——空池不是錯誤，與 parent 的處理一致 |
| 正常跑完 | 跑 | 依結果 `success` / `partial` |

**有效啟用條件是「兩個 `enabled` 都為 true」**：偵測沒有自己的 cron，
`evaluation_universe.enabled=false` 時 parent 根本不會被註冊，偵測也就永遠不會執行。
標成 registered 會讓 `/scheduler/status` 顯示 `never_run` ＋ `stale`——那是假警報，
正是 `disabled` / `never_run` 分野要避免的（見 `api-reference.md`）。

**註冊與執行的邊界（2026-08-26 review 補，逐條定案）**：

| 情況 | 定案 |
|---|---|
| `markRegistered` 的時機 | **兩個 job 的條件不同，不得「同時標記」**（2026-08-26 review 修正）：<br>• `evaluation_universe_sync`：parent 的 `cron.AddFunc` **成功後**標記（cron 字串打錯時 `AddFunc` 只記 log 不中止，那時不該標記）。<br>• `candle_gap_detection`：**在上一條成立的前提下，還要額外滿足**「自身 `enabled=true`」**且**「四項必要依賴齊全」才標記。<br>前一版寫「parent 成功就同時標記兩個」會讓**偵測關閉或依賴缺失時仍顯示已註冊**，與下方兩節的規定直接衝突 |
| 人工觸發 `RunEvaluationUniverseSync` | **一樣要套用偵測的有效啟用條件**。否則 `candle_gap_detection.enabled=false` 時，人工呼叫 parent 會替一個「未啟用」的 job 寫出 `job_runs` 紀錄，狀態頁會出現 `disabled` 卻有紀錄的矛盾 |
| `ListActive` 失敗 | 偵測的 `job_runs`：`symbols_total = 0`、`symbols_failed = 0`、**`degraded=true` → `partial`**，`error` 記 `verification_unavailable: 取不到候選清單` |
| **任一必要依賴缺失**但 `enabled=true` | **視為未註冊**（`disabled`），並在 `Start()` 記 **Error log**——比照 `evaluationUniverse` 的「未注入即不註冊」。**不得**等到執行時才 nil panic。必要依賴共**四項**，見下 |

##### 偵測的必要依賴共四項（2026-08-26 review 補齊）

前一版只檢查 `verification` 與 `reference`——**漏了兩個**，而其中一個的漏法會導致
「註冊了卻不可能運作」：

| 依賴 | 來源 | 偵測用途 | 對 **parent** 而言 |
|---|---|---|---|
| `verification` repo | `SetCandleGapDetection` | 公平排序狀態 | 不需要 |
| `reference` client | `SetCandleGapDetection` | 日曆／市場／個股對照 | 不需要 |
| **`StockSymbolRepo`** | **I-094 的 `SetEvaluationUniverse`** | **`market` 分流**（決定打 TWSE 還是 TPEx 端點） | 缺了只是少過濾下市標的，**fail-open 可接受** |
| **`CandleRepo`**（`evaluationUniverseCandles`） | `SetEvaluationUniverse`（T-062 加的） | **取實際日期集合**——沒有它就沒有差集 | ⚠️ **缺了是合法降級**（退回全量回補，見 T-062） |

⚠️ **`CandleRepo` 這一列是關鍵**：它對 parent 是**合法的 nil**（`dropSymbolsSyncedToday`
未注入時退回全量抓取），但**對偵測是完全不能運作**——沒有實際日期集合就算不出差集。
沿用 parent 的判斷會讓偵測在 `CandleRepo=nil` 時被標成已註冊卻永遠產不出結果。

**定案：`candle_gap_detection` 的註冊條件是「四項全部具備」**，缺任一項都不註冊並記 Error。
**parent 的註冊條件不受影響**，維持它自己的判斷（`evaluationUniverse != nil && enabled`）——
兩者是不同的需求，不要互相牽制。

##### 獨立 timeout 與 job_runs 數字（2026-08-26 review 補）

| 項目 | 定案 |
|---|---|
| `timeout_sec` | 預設 **300**（5 分鐘）。加進 `candle_gap_detection` 參數表與 config 測試 |
| 非正值 | 退回程式常數預設，**不是無限**（比照 `srZoneVerifyDays` 的兩段式處理） |
| hard cap | **900 秒**，超過就截到上限——設定來自 env，打錯字不會有人擋 |
| `symbols_total` | **本輪有效池大小**（比照 I-092 定案的通則：分母是清單大小） |
| `symbols_failed` | **固定 0**——缺口與不可用都走 `degraded`，不污染這個欄位（I-092 的決定） |

**為什麼不做成獨立 cron job**：它必須在當日回補之後才有意義，另立 cron 就要處理
「回補還沒跑完就開始偵測」的競態，而那個競態沒有任何好處。

```text
池成員（ListActive）
   → StatesBySymbols        （is_listed / market，缺席=unknown）
   → 實際日期集合（CandleRepo，單一查詢）
   → 預期交易日（年度日曆，date= 參數，驗證年份）
   → 差集 = 候選缺口
   → 按 (market, missing_date) 分組；比例超標 → aggregate 告警，不逐檔
                                          ↳ **該組的 symbol 也要 RecordAttempts**（見下）
   → 其餘：LoadStates → Go 端合併排序 → 掃描取 cap 個
   → 個股核對（依 market 分流：TWSE `STOCK_DAY` ／ TPEx `tradingStock`）
   → 依 (symbol,timeframe) coalesce → RecordAttempts
   → finishRunDegraded(partial) ＋ error 以 "; " 合併
```

**主要風險與回滾**

| 風險 | 對策 |
|---|---|
| 對交易所造成請求壓力 | aggregate 短路 ＋ cap ＋ 間隔 ＋ breaker；預設關閉 |
| 誤報訓練使用者忽略告警 | 三種結論分開（正常／缺口／驗不了）；停止買賣不誤報（測試 3） |
| 對照源格式變動 | 一律 `verification_unavailable`，**不猜測**；壞在明處 |
| 新表寫入失敗拖累回補 | 偵測獨立於回補；寫入失敗只讓該輪 `degraded`，**不影響 K 棒寫入** |

**回滾**：`enabled=false` 即完全停用（不刪表）。表留著不影響任何既有流程。

**完成後的歸檔位置**：[`architecture.md`](./architecture.md) 的日 K 維護段
（缺口偵測與 T-062 自癒性質的關係已寫在那裡，把現況規格接上去）；
參數說明放 `backend/config.yaml` 的區段註解，比照既有慣例。

#### 這筆的價值與「不做」的選項

已證實**目前沒有任何資料在流失**（`2867` 是合法的停止買賣），所以價值全在
「未來真的漏了的時候會被發現」。若判斷該風險可接受，可以標成已知限制不修——
但要**明確記下這是決定，不是沒想到**，並且**至少要處理 T-062 帶走的自癒性質**
（例如週期性地對整池做一次全量重抓，讓中間的洞有機會補回來）。

**原本寫的「`2867` 復牌後自動補齊」作為天然驗證案例已作廢**——它不會復牌。
要驗證修法，得自己造缺口（在 dev 刪掉某檔中間的一根 K 棒，看偵測會不會報）。

### I-094：池成員下市後不會停止被回補——`evaluation_universe.active` 與 `stock_symbols.is_listed` 沒有任何連動

| 欄位 | 內容 |
|---|---|
| 狀態 | **待修復** |
| 嚴重度 | 中（每天對已下市標的發一個無用請求並記 `success`；標的數會隨時間累積） |
| 分類 | Go / Scheduler / 資料一致性 |
| 發現日期 | 2026-08-26 |
| 來源 | I-091 的 review——查 `2867` 的下市公告時發現 |

#### 現象

`runEvaluationUniverseSync` 的標的來源只有 `s.evaluationUniverse.ListActive(ctx)`
（`scheduler.go:982`），而 `ListActive` 只看 `evaluation_universe.active`
（`evaluation_universe_repo.go:23`）。**它不對照 `stock_symbols.is_listed`。**

兩者是完全獨立的狀態：`stock_symbol_sync`（每日 06:30）會依 TWSE 清冊維護 `is_listed`，
但**沒有任何東西會因此把池成員設成 `active = false`**。

#### 具體案例

`2867 三商壽`因股份轉換，**自 2026-08-20 起停止買賣、2026-09-01 起終止上市**
（TWSE 公告，見 I-091）。9/1 之後：

* `stock_symbol_sync` 會把 `is_listed` 設為 `false`。
* 但它仍在 `evaluation_universe` 且 `active = true`。
* 於是**池維護每天照樣對它發一個 FinMind 請求**，永遠拿回 0 根、永遠記 `success`、
  永遠不會有人發現。

在 FinMind 的 5 req/min 下，那是每天約 12 秒的配額被丟進水裡；下市標的只會愈積愈多。

#### 為什麼不是「手動退池」就好

手動可以解決 `2867` 這一檔，但**下一檔下市時會完全重演**，而且一樣不會有人發現。
這筆要修的是「兩個狀態沒有連動」，不是那一檔。

#### 作法：**定案採 A**（2026-08-26 review 後定案）

B 列在下面只作為被否決的替代方案紀錄，**不是待選項**。

**A. 回補前過濾**：`runEvaluationUniverseSync` 取到 `ListActive` 之後，
剔除 `is_listed = false` 的標的。

* 優點：不改資料、可逆；主檔若誤判也只是少抓一天，隔天就恢復。

⚠️ **「注入 `StockSymbolRepo` 就好」是不夠的**（2026-08-26 review 補）。
現有介面只有兩種形狀，兩種都不能用：

| 現有方法 | 問題 |
|---|---|
| `Get(ctx, symbol)` | 逐檔查 → 135 檔就是 **N+1** |
| `List(ctx, onlyListed)` | 載入**整份證券主檔**（實測 `stock_symbols` 有 **49,458** 列）只為了過濾 135 檔 |

所以 A 的前置是**先補一個批次查詢**。**定案：走批次 map**
（2026-08-26 review 後定案，不再二選一）：

`StockSymbolRepo` 加「依 symbol 清單批次取主檔狀態」的方法。
**回傳值要帶 `market`，不是只有布林**（2026-08-26 review 修正）：

```go
map[string]StockSymbolState{
    "2867": {IsListed: true, Market: "上市"},
}
```

**為什麼不能只回 `map[string]bool`**：I-091 需要 `stock_symbols.market` 來決定個股核對
要打上市還是上櫃端點（實測池內**上市 101 / 上櫃 34**）。只回布林的話，I-091 就得
**再查一次主檔**，或退回逐檔查詢——那正是這個批次方法要消滅的 N+1。
兩筆共用同一次查詢，一次收斂掉。

**為什麼不走 join**：`ListActive` join 主檔的話，`EvaluationUniverseEntry` 需要一個
能表達「主檔不存在」的 nullable／exists 欄位——**目前的模型沒有**，加了就是為了一個
過濾需求去改共用的 entry 模型。而**批次 map 天然表達得了三態**：
key 存在且 true／key 存在且 false／**key 不存在＝unknown**，不必動任何既有模型。

**判定必須是三態，不是布林**（2026-08-26 review 補）：

| 主檔狀態 | 處置 | 計數 |
|---|---|---|
| `is_listed = true` | 保留，照常回補 | — |
| `is_listed = false` | 過濾 | `delisted` |
| **主檔查無該 symbol** | **fail-open，保留回補** | `stock_symbol_unknown` |

第三態不能省。池成員與主檔是**兩個獨立維護的清單**——新入池的標的可能還沒被
`stock_symbol_sync` 收錄，主檔同步失敗那天也會整批查無。實測目前 135 檔**全部都在主檔裡
（0 筆查無）**，但「現在剛好都有」不是可以依賴的前提。

⚠️ **批次 map 的缺席語意要在呼叫端寫死**：`map` 裡沒有那個 key **是 unknown，不是下市**。
兩者的處置相反（前者保留、後者過濾），寫錯不會有任何東西報錯。
**`Market` 欄位在 unknown 時也不存在**，所以 I-091 對這類標的無法決定核對端點——
應一併落到 `verification_unavailable`，不要預設成上市。

（**若日後改走 join**：必須是 `LEFT JOIN`——inner join 會讓查無主檔的池成員**直接消失**，
那是 fail-closed，與這裡要的語意相反，而且消失得完全靜默。這也是不走 join 的理由之一。）

**整體失敗策略也是 fail-open**：主檔查詢本身失敗時**維持全量回補**，不能把整池跳過。
降級方向與 T-062 的跳過最佳化一致——「多抓一點」可接受，「靜默少抓」不可接受。

**計數要分開**：`delisted` 與 `synced_today` 是不同原因，**不要併進同一個 `skipped`**，
否則看不出池裡累積了多少死標的。

**測試至少要涵蓋**：批次查詢本身、主檔查詢失敗時退回全量、
**池成員在主檔查無時仍要回補**（第三態）、
**重新上市（`is_listed` 由 false 轉回 true）時要恢復抓取**——最後這條是回歸保護，
沒有它的話日後有人把 A 改成 B 那種「一次性退池」不會有任何東西報錯。

**B. 主檔驅動退池**：`stock_symbol_sync` 發現 `is_listed` 轉 false 時，
一併把 `evaluation_universe.active` 設為 false 並記 log。

* 優點：池的狀態自己就是對的，不必每次回補都查主檔。
* ⚠️ **會改資料且不可逆**：主檔誤判（例如某天 TWSE 清冊抓取不完整）會**靜默清掉池成員**，
  而重新入池是人工動作。要做的話必須先確認 `stock_symbol_sync` 對「清冊抓取不完整」
  有防護——目前它把兩個來源都抓完才寫入，但那不等於有防護。

**定案 A**：可逆、不動資料、失敗模式溫和。B 的風險與收益不成比例。

##### A 的實作規格（定案，供 I-091 直接引用）

| 項目 | 定案 |
|---|---|
| 型別 | `store.StockSymbolState{ IsListed bool; Market string }`，定義在 `internal/store`（與 `StockSymbolRepo` 同模組，兩筆都要用） |
| repo method | `StockSymbolRepo.StatesBySymbols(ctx, symbols []string) (map[string]StockSymbolState, error)`——**單一查詢**（`WHERE symbol IN (?)`，比照 `SymbolsWithCandleOn` 的 `sqlx.In` ＋ `Rebind` 寫法） |
| 注入 | `Scheduler.SetEvaluationUniverse(repo, candles, stockSymbols, cfg)` 多收一個參數，比照 T-062 加 `candles` 的既有作法（依賴與設定一起注入） |
| 呼叫端 | `runEvaluationUniverseSync` 取到 `ListActive` 之後、`dropSymbolsSyncedToday` 之前呼叫一次 |
| 缺值語意 | **map 缺席＝unknown＝保留回補**，計入 `stock_symbol_unknown`；`Market` 一併缺席（I-091 據此落 `verification_unavailable`） |
| 失敗語意 | 查詢本身失敗 → **fail-open**，維持全量回補，記 Error log。⚠️ **但 I-091 的收斂不同，見下** |

⚠️ **同一次 `StatesBySymbols` 失敗，兩筆的正確收斂不一樣**（2026-08-26 review 補）：

| | I-094（下市過濾） | I-091（缺漏偵測） |
|---|---|---|
| 影響 | 少過濾掉下市標的 → 多抓幾檔 | **完全失去 market routing**，決定不了個股核對端點 |
| 收斂 | **全量回補**，記 Error log | **整批 `verification_unavailable`**，job 記 `partial` |
| 為什麼不同 | 多抓是溫和降級，隔天就恢復 | 驗不了卻記 `success`，正是 I-091 要消滅的誤導 |

`error` 欄要**同時保留既有的回補錯誤**，以既定的 `"; "` 格式合併（見 I-091 的說明）。
**必須有整合測試涵蓋「`StatesBySymbols` 整體失敗」**，一次斷言兩邊的收斂——
分開測會讓「其中一邊忘了處理」看不出來。

**相關**：[`issue.md`](./issue.md) I-091（同一個標的暴露的另一個問題：缺漏偵測）。

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

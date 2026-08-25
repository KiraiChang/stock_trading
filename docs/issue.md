# ISSUE：遇到的問題與已知限制

記錄實際發生過的 bug、矛盾結果、文件/程式碼不一致，以及設計上刻意接受的
限制。跟「想做的優化」無關的項目放這裡；未來想做的功能擴充記錄在
[todo.md](./todo.md)。

## 使用說明

- **狀態**：`待修復` / `修復中` / `已修復` / `已知限制（不計畫修復）`
- **嚴重度**：`高`（結果矛盾/資料錯誤）/ `中`（誤導但不影響核心功能）/
  `低`（文件或註解落後，不影響 runtime）
- 新增項目時往下加一筆，編號遞增（`I-0xx`）。修復後若仍需短期追蹤，先把「狀態」改成
  `已修復` 並補上「修復方式」；若修復紀錄已移到對應主題文件或 review 文件，
  則可從本清單移除。
- **編號只增不重用。** 已移除的條目編號不得再發給新問題——程式碼註解與其他文件會留著舊 ID，
  重用會讓兩件無關的事共用一個代號。**`I-070` 已經發生過一次**（先發給 T-045 的事件鏈墓碑，
  移除後又發給 T-040 的 `keep_symbols` 靜默丟棄，兩筆現在都已收斂），
  見 `todo.md` T-045 那段的註記。
- **下一個新編號從 `I-090` 起算。**（I-081～I-083 於 2026-08-21 發出，I-084～I-087 於 2026-08-24 發出，I-088 與 I-089 於 2026-08-25 發出。）
  **發出新編號時記得把這一行一起往前推**——上一次就是漏了這步，I-089 發出去之後
  這裡還寫著「從 I-089 起算」，差一點又重用一次（I-070 已經發生過）。
  檔案裡看得到的最大是 I-085，但被移除的條目
  （I-040 / I-056 / I-069 已於 2026-08-18 收斂，I-076 於 2026-08-19 收斂，
  I-083 / I-084 於 2026-08-24 收斂，I-086 / I-087 / I-088 / I-089 於 2026-08-25 收斂，
  I-070～I-072 更早）都佔用過編號。
  **不要用「檔案裡最大值 + 1」決定編號**——被移除的條目正是看不見的那些；
  必要時翻 git log 或本節的收斂紀錄。
- **移除條目前要先反轉依賴。** 主題文件與程式碼註解常寫「見 issue.md I-0xx」，
  這種寫法讓 issue.md 變成權威來源，一刪就斷鏈。移除前先把說明**內嵌**到對應主題文件，
  再把所有引用改指向該文件。收斂後用下面這條檢查沒有殘留：

  ```bash
  comm -13 <(grep -o '^### I-0[0-9][0-9]' docs/issue.md | sed 's/### //' | sort -u) \
           <(grep -rho "I-0[0-9][0-9]" --include="*.md" --include="*.go" --include="*.py" \
              --include="*.sh" . | grep -v node_modules | sort -u)
  ```

  列出的 ID 必須**只剩明確標為歷史沿革的引用**（「原記於…」「當時編號…」），
  不能有任何「見 I-0xx」形式的活指標。
  **本節自己會出現在輸出裡**（上面提到 I-040 / I-056 / I-069 / I-070～I-072 / I-076 /
  I-083 / I-084 / I-086～I-089 與下一個可用的 I-090），那是預期的，不是殘留。
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

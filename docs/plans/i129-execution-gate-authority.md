# I-129 計畫 v4：進場許可的 RR 判斷改由 execution gate 決定，setup 不合格最多試單

> 狀態：⚠️ v4 待確認（2026-11-04；v1、v2、v3 各經一輪 review 退回修訂，修正內容見文末「修訂紀錄」）。
> 對應：[`issue.md`](../issue.md) I-129（原 `todo.md` T-065，2026-11-03 依分流規則改列；T-065 編號不回收）。
> 現況規格：[`sr-zone-scoring.md`](../sr-zone-scoring.md)「RR 語意分層」「已知的單向性」。

## 一、目標與不做的範圍

`decision_engine` 的 `build_decision_summary()` 有兩顆 RR gate：

* **setup gate** `_rr_gate()`：讀 `primary_zone.risk_reward_ratio`（zone 歷史統計）。
* **execution gate** `_execution_rr_gate()`：從 entry／stop／封頂 target 算出的 executable RR，最後覆寫成對外的 `rr_gate`。

進場許可卻在 execution gate 算出來之前，就被 setup gate 提前決定了。結果是 setup 1.4／1.7 時輸出 `BLOCKED`，而對外的 `rr_gate` 寫 11.0、兩層都通過，沒有任何欄位說得出原因。T-055 的封頂只能往下降，擋不了這個方向。

**連 11.0 都是錯的**：setup 擋掉之後，`_entry_executability()` 改走限價、用 zone 上緣 100 當 entry，算出 (122−100)/(100−98)=11.0。不被擋時本來該走市價 105，算出來是 (122−105)/(105−98)≈2.43。

**目標**：

1. 「選進場價」不再依賴 RR，資料流變成沒有循環的 DAG；execution gate 在真正的進場價上計算。
2. semantic、final、`rr_gate` 三者一致，RR 不合格時有欄位說明理由。
3. setup 不合格、但 executable RR 可量化且合格的列，**最多放行到試單**（`PROBE_ALLOWED`）。這個放寬要先通過「六」的離線量測才能上 live；不通過就退回「不放行」（「4.5」）。

不做：

* `strong` 的 setup RR ≥ 2.0 條件**保留**（「二」D4），所以本筆⛔ 不產生 BuySmall → Buy 的升級。
* `_entry_action_state()` 的 BUY／ACCUMULATE 細分（讀 setup RR 的 `>= 2.5`，兩者正規化後都是 `ENTRY_ALLOWED`，不影響許可）。只在主題文件記一行。
* 門檻數值（`_minimum_rr()`、`FULL_ENTRY_MIN_RR`）、Lifecycle Engine、`schema_version=sr_zone_decision_replay_p0`、Go 端：都不改。
* ⛔ 不做 Go 端「依版本失效快照」的機制；部署與回滾改用「七」的程序。
* ⛔ 不部署。上 live 要出新 image，等 I-123～I-126 的觀察期結束（最早 2026-11-09）、本筆 review 通過、「六」的量測通過之後另立部署計畫，可以跟 T-076 共用同一次 build。

## 二、裁決

* **D1 RR 不合格一律 `WAIT_CONFIRMATION`**（2026-11-03 使用者確認），不分 gate 來源。
  `BLOCKED` 只留給結構事實：跌破（`BREAKDOWN_RISK`）、`AVOID`、收盤跌破支撐、daily `INVALIDATED`。
  RR 是策略條件、不是結構事實（同「lifecycle 不看 RR」）。兩種狀態的 action 都是 Hold／等待。
* **D2 缺 zone 統計不放行**：setup RR 是 `None` 時，即使 executable RR 算得出來，也是 `qualified=false`／`RR_UNAVAILABLE`。現在一律 BLOCKED，改完只換成 WAIT。
* **D3 setup 不合格最多試單**（2026-11-03 使用者確認，回應 review P1-1）。setup RR 有值但未達門檻時：

  | executable RR | 結果 |
  |---|---|
  | 可量化（`gate_basis=ENTRY_STOP_TARGET`）且 ≥ 門檻 | 最多 `PROBE_ALLOWED`＋`SETUP_RR_BELOW_MINIMUM` |
  | 可量化但 < 門檻 | WAIT＋`EXECUTION_RR_INSUFFICIENT` |
  | 算不出來（市價或限價的 target 未知） | **沿用 setup 判定**（`gate_basis=ZONE_STATISTIC`）→ WAIT＋`RR_INSUFFICIENT` |

  市價、target 未知時「未知不算不合格」的既有語意，**只保留給 setup 合格的列**（現狀不變）。
* **D4 `strong` 保留 setup RR ≥ 2.0**（回應 review P1-2）：拿掉的話，setup 1.9 這類已合格的列會從 BuySmall 升成 Buy。本筆只拿掉 `rr < 1.5` 的提前 return（不拿掉的話，setup 1.4 放行到 PROBE 時 action 仍是 Hold，又是一組矛盾）；`rr is None` 的提前 return 保留（D2）。
* **D5 放寬要先過離線量測**（回應 review P1-3、P2-4）：判準在「6.5」，量測前就定好；不通過或證據不足時退回「不放行」。**證據不足⛔ 不降低標準**：任何一項判準的樣本不夠，就是證據不足，⛔ 不改用較寬的替代判準（v3）。
* **D6 量測欄位做成獨立的前置 commit**（v3，回應第二輪 review P2-4）：replay 列補上量測需要的欄位，先成為 commit A；I-129 的決策改動疊在上面成為 commit B。兩趟 replay 都原生帶有這些欄位，⛔ 不用外部 tooling patch。

## 三、讀碼發現（原 T-065 只列了三處）

| 讀 setup RR 的地方 | 現況 |
|---|---|
| `_decision_action()` | `rr < 1.5` 或 `None` → WATCH 提前 return；`strong` 要 `rr >= 2.0`；兩句 RR 的 risk note |
| `_decision_semantic_pipeline()` | `not rr_qualified` → `BLOCKED` |
| **`_daily_confirmation()`**（原 T-065 沒列） | `not rr_gate.qualified` → daily `BLOCKED`，`_final_entry_permission()` 因此直接變 BLOCKED。只修上一列修不好 |
| **`_decision_derived_view()` 的 `path_gate_state=RR_BLOCKED`、`_price_path()`**（原 T-065 沒列） | 吃 setup gate |
| `_entry_action_state()` | `rr >= 2.5` 決定 BUY／ACCUMULATE（不影響許可，見「一」不做） |

循環只出在一個地方：`_entry_executability()` 只在 semantic 許可 ∈ {`PROBE_ALLOWED`, `ENTRY_ALLOWED`}
（加上 `CLOSE_RECLAIM` 和 TESTING／CONFIRMED／CONTINUATION）時才走市價 entry。
這個許可裡的其他輸入（lifecycle、event_signal、`AVOID`、blocking zone）都跟 RR 無關。

## 四、設計

### 4.1 新的計算順序

```
_decision_action（只剩 rr is None 的提前 return 與 strong 的 setup 條件）→ entry_action_state → _minimum_rr
→ setup gate（不變）
→ semantic pipeline：不看 RR 的結構許可（setup gate 只用來算兩個診斷欄位）
→ _entry_executability：「如果要進場，用什麼價」。和 RR 放不放行無關，之後⛔ 不重算
→ execution target → rr_context → execution gate（對外的 rr_gate，含 D2／D3 的規則）
→ 套用 execution gate 到 semantic：
     主 gate 不合格        → 封頂 WAIT_CONFIRMATION＋gate 的 reason code
     setup 不合格（D3）    → 封頂 PROBE_ALLOWED＋SETUP_RR_BELOW_MINIMUM
     secondary 不合格      → 封頂 PROBE_ALLOWED＋RR_BELOW_FULL_ENTRY（既有）
→ derived view（path gate 看 execution gate）→ market_bias、price_path、
  daily_confirmation（不看 RR）→ final_entry_permission（同樣三條封頂）→ final action → risk_notes
```

### 4.2 逐項改動（`python/backtest/modular/sr_scoring/decision_engine.py`）

1. **`_decision_action()`**：只拿掉 `rr < 1.5` 的提前 return（D4）。`rr is None` 的 return 與 `strong` 的 `rr >= 2.0` 保留。
2. **`_decision_semantic_pipeline()`**：拿掉 `not rr_qualified → BLOCKED`，簽名不變。`setup_rr_qualified`、`rr_decoupling_candidate` ⛔ 不動：仍然定義在 setup gate 上，replay 的等價式檢查與 `test_i074_diagnostics.py` 照舊成立。
3. **`_execution_rr_gate()`**：
   * 輸出新增 `setup_qualified`（布林，setup gate 的判定），和既有的 `zone_actual_rr` 並列。封頂邏輯讀這個欄位，⛔ 不讀 validation-only 的 `setup_rr_qualified`，也⛔ 不從 `zone_actual_rr` 和門檻重推。
   * setup 不合格且 executable RR 算不出來時，市價路徑也改走 `ZONE_STATISTIC` 退回（D3）。setup 合格的市價 target 未知維持現狀（`qualified=true`／`EXECUTION_RR_UNAVAILABLE`）。
   * setup RR 是 `None` 時一律 `qualified=false`／`RR_UNAVAILABLE`（D2）。
4. **封頂集中在一個 helper**（取代 `_apply_full_entry_gate_cap()`）：依「4.1」的三條規則回傳上限狀態與 reason code。semantic 與 `_final_entry_permission()` 都呼叫它，所以兩個 authoritative 欄位一定同進同出。
5. **`_decision_derived_view()`**：新增 keyword 參數 `semantic_pipeline=None`，有傳就直接用，沒傳就照舊自己算，這樣測試裡 `_derived` helper 那一批的位置參數呼叫不會壞。`build_decision_summary()` 傳入套過封頂的 semantic，`rr_gate` 傳 execution gate，`RR_BLOCKED` 也因此看 execution gate（值的名稱不改）。
6. **`_daily_confirmation()`**：拿掉 RR → BLOCKED 的分支和 `rr_gate` 參數，只看價格事實。
7. **呼叫順序**：derived view、`_market_bias()`、`_price_path()`、`_daily_confirmation()` 移到 execution gate 之後。它們都不回饋給 execution gate。
8. **`risk_notes`**：「主交易區風險報酬比不足。」「主交易區缺少風險報酬比，先觀察。」改成帶 code，在 `_final_entry_risk_notes()` 依 execution gate 校正，比照 T-055 對 `RR_BELOW_FULL_ENTRY` 的做法。D3 的封頂補一句「zone 歷史統計的風險報酬比未達門檻，最多觀察性試探。」
9. **版本字串**：`decision-semantic-pipeline-p4` 升 p5、`decision-derived-view-p2` 升 p3。`schema_version` ⛔ 不動。

### 4.3 前端

* `frontend/src/lib/api/srZones.ts` 的 `derivedReasonText` 補 `RR_INSUFFICIENT`、`RR_UNAVAILABLE`、`NO_PRIMARY_ZONE`、`EXECUTION_RR_INSUFFICIENT`、`EXECUTION_RR_UNAVAILABLE`、`RR_BELOW_FULL_ENTRY`、`SETUP_RR_BELOW_MINIMUM` 的中文。
* `rr_gate` 的型別補 `setup_qualified`（選填）。

### 4.4 replay 列的量測欄位（commit A）

`evaluation.py` 的 `_decision_fields_from_summary()` 補上下列欄位（全部取自同一份 decision summary，⛔ 不重算）：

* `market_action`、`action`、`entry_action_state`
* `semantic_entry_permission_state`、`semantic_reason_codes`（semantic pipeline 的值）
* `path_gate_state`（derived view 的值）
* `final_entry_reason_codes`（`final_entry_permission.reason_codes`）
* `risk_notes_digest`：`risk_notes` 的 SHA-256 前 16 碼（只用來比對「有沒有變」；replay report 會整份進 `metrics_json`，⛔ 不放全文）

⚠️ **只改 `_decision_fields_from_summary()` 不夠**（v4 讀碼發現）：replay 列是逐欄明確組出來的，只帶 summary 的部分欄位和九個診斷欄位，新加的欄位不會自動進到列裡。所以：

* 欄位名稱定成 `replay_bundle` package 的一個常數（例如 `DECISION_MEASUREMENT_FIELDS`），比照 `DIAGNOSTIC_FIELDS` 由匯出端、測試與 smoke 檢查共用，⛔ 不在多處各列一份。
* replay 列依這個常數加入這些欄位；decision 不可用的列一律填 null。
* ⛔ 不併入九個診斷欄位：`DIAGNOSTIC_FIELDS`、no-zone fallback 與 `validate_diagnostics()` 的契約都不動。

commit A ⛔ 不改任何決策邏輯，所以 A 的決策輸出與改前的 HEAD 相同。新欄位在主題文件的「Decision Replay 的診斷欄位」記為 validation-only。

### 4.5 退回「不放行」

D3 的上限集中在一個常數（`PROBE_ALLOWED`）。量測不通過或證據不足時，把它改成 `WAIT_CONFIRMATION`：setup 不合格的列一律等待確認、⛔ 不放行，但「目標」1、2 的修正全部保留。退回要另開新版記錄量測結果，並改對應的測試。

## 五、行為改變

| 列的類別 | 現在 | 改後 |
|---|---|---|
| setup 合格 | — | `final_entry_state`、`market_action`、`action`、`position_action`、`risk_notes` **不變**；`rr_gate` 只多 `setup_qualified`。semantic 許可在 execution gate 不合格時對齊成 WAIT（final 本來就是 WAIT），`path_gate_state` 可能變成 `RR_BLOCKED` |
| setup 不合格、executable 可量化且合格 | BLOCKED，無法診斷 | **PROBE_ALLOWED**＋`SETUP_RR_BELOW_MINIMUM`：**這是放寬**，action 最多 BuySmall |
| setup 不合格、executable 可量化但不合格 | BLOCKED | WAIT＋`EXECUTION_RR_INSUFFICIENT` |
| setup 不合格、target 未知 | BLOCKED | WAIT＋`RR_INSUFFICIENT` |
| setup 缺值 | BLOCKED | WAIT＋`RR_UNAVAILABLE` |
| 結構阻擋（任何 setup） | BLOCKED | BLOCKED（不變） |

* 放寬只發生在 setup 不合格的列，而且是**所有 lifecycle**，不只 CONTINUATION。I-074 的 156 列只是其中一組回歸案例。
* setup 不合格的列原本是 Hold，拿掉 `rr < 1.5` 的 return 之後可能變成 BuySmall；`strong` 仍要 setup ≥ 2.0，所以不會出現 Buy。
* 依設計，「試單升完整進場」與「反向收緊」都應該是 0；「6.5」逐列驗證。
* `_minimum_rr()` 會隨 action 改變（PROBE_ENTRY → 1.5）。這只發生在 setup 不合格的列，setup 合格與否的分類不受影響：setup < 1.5 在任何門檻下都不合格。

### 風險與相容

* **governance gate**：`_decision_replay_entry_performance()` 只統計 PROBE／ENTRY 的列。新增放行組會進到下一次 decision replay 的母體。live 的保護規則見「七」。
* **Go `backend/internal/portfolio/analyzer.go`** 讀 `market_action`。新增放行組最多 BuySmall，Go 端給的是「小量建立」。Go 在沒有壓力時自己用 `BreakoutTargetRR` 補目標，這個保護和 D3 無關：D3 要求 Python 端的 executable RR 可量化才放行。
* **I-074 Stage 2 判讀器**（`replay_bundle/stage2_verdict.py`）要求 before／after 兩側都是 BLOCKED。它只對已封存的 evidence、在暫存複本裡執行，⛔ 不會因本筆失敗；但那份結論是改前行為的紀錄，主題文件要加註。
* **相容**：API 值域不變（沒有新增 state 或 path 值）；新增 `rr_gate.setup_qualified` 欄位與 `SETUP_RR_BELOW_MINIMUM` reason code。

## 六、測試與驗證

### 6.1 新測試（`test_decision_engine.py`）

* `_continuation_full_entry_case()` 加一個 setup RR 參數，跑壓力 122.0、setup 1.4／1.7／2.8 三組（原 T-065 的重現表）：
  * 1.4、1.7：市價 entry、executable≈2.43、`rr_gate.qualified=true`、`setup_qualified=false`；semantic 與 final 都是 `PROBE_ALLOWED`＋`SETUP_RR_BELOW_MINIMUM`；action 是 BuySmall；risk note 寫得出原因。
  * 2.8 對照組：輸出不變（`ENTRY_ALLOWED`）。
* D3 三條規則各一組：可量化但不合格 → WAIT＋`EXECUTION_RR_INSUFFICIENT`；**市價、target 未知、setup 不合格 → WAIT＋`RR_INSUFFICIENT`**（review P1-1 的端到端測試）；限價、target 未知 → WAIT＋`RR_INSUFFICIENT`，path 是 `RR_BLOCKED`。
* D2：setup 缺值、executable 算得出來 → WAIT＋`RR_UNAVAILABLE`。
* D4：setup 1.9（TIER_3，已合格）、strong 的其他條件都成立、executable ≥ 2.0 → action 維持 BuySmall，不升 Buy。
* setup 合格、市價、target 未知 → 維持現狀（`qualified=true`、`EXECUTION_RR_UNAVAILABLE`）。
* daily confirmation 不再因 RR 變成 BLOCKED。
* 不變式：final 是 BLOCKED 時，reason code 不得是 RR 類；setup 不合格時 final 不得高於 PROBE；沒有 final 專屬 gate（executability、blocking zone、model governance）時，semantic 與 final 的許可要相等。
* 先寫測試，確認在改前的程式上會紅，再改程式。

### 6.2 要改寫的既有測試（照新規格改，保留原本的意圖）

* `test_rr_below_hard_gate_stays_watch_even_with_high_score_and_ev`
* `test_confirmed_buy_small_is_small_entry`（BLOCKED 改成 WAIT）
* `test_hard_block_daily_confirmation_does_not_backfill_derived_reasons`（改用 `SUPPORT_CLOSED_BELOW` 這類結構阻擋，守住「不回填」的意圖）
* `test_derived_position_gate_rr_blocked_support_defense`
* 寫死版本字串的測試（`test_rr_qualified_probe_waits_for_price_follow_through` 等）

### 6.3 要原樣通過

* `test_full_entry_gate_caps_authoritative_permission_end_to_end` 與對照組 `test_full_entry_gate_cap_control_group_would_otherwise_be_entry_allowed`
* `test_limit_entry_without_target_keeps_setup_rr_verdict`
* `test_missing_ev_rr_cannot_be_buy`
* `test_secondary_gate_not_qualified_when_no_statistic_exists`
* `test_i074_diagnostics.py` 全部（setup gate 診斷欄位的守門）
* replay_bundle 的測試

### 6.4 全套檢查

`python/scripts/test.sh`、`backend/scripts/test.sh`（Go 沒改，確認沒有連帶影響）、前端測試、`scripts/check-doc-refs.py`、收斂用的 `I-[0-9]{3}`／`T-[0-9]{3}` 殘留檢查。

### 6.5 離線影響量測（上 live 前必做；方法與判準在量測前定案）

**輸入一致性**

* 用凍結 bundle `b1_20260901_1d_74350966_5d7ecb10`（11 檔、13,417 列）。`load_bundle()` 會驗 manifest、逐檔 SHA-256 與封閉清單：candles、chip、governance snapshot、model、`replay_config`（dataset 與 builder config）都在 bundle 裡，不符就中止。
* 兩趟都設**同一個 `REPLAY_IMAGE_ID`**，避免各自重 build image（I-116 的風險）。
* `--before-ref` 兩趟都傳 A 的完整 OID（I-115：Stage 1 必填但不使用）。`TOOLING_PATCH` 兩趟都是空的（D6）。
* 比較前先斷言兩份 artifact 的 `bundle_id` 相同、provenance 的 `runtime_settings` 相同、列的 key 集合相同；任何一項不符就判「無法歸因」，⛔ 不出結論。

**欄位驗證**（正式跑之前）

* 單元測試：
  * 用 `test_decision_engine.py` 的 fixture 建 decision summary，斷言「4.4」的每個新欄位都等於 summary 裡的對應值（`market_action`、semantic 許可、`path_gate_state`、reason codes 都要逐一比對，`risk_notes_digest` 要等於重算的 digest）。
  * 用 `test_evaluation.py` 既有的 decision replay fixture，斷言 replay 列真的帶有這些欄位（「4.4」：只改 `_decision_fields_from_summary()` 不會進到列裡），而且 decision 不可用的列這些欄位都是 null。
* **smoke 內建欄位檢查**（v4，回應第三輪 review P2-2）：`scripts/smoke-replay-offline.sh` 結束時會刪掉整個暫存目錄，連 after artifact 一起刪，外部程式事後讀不到。所以檢查**直接接進 smoke**，排在 Stage 1 之後、清理之前，⛔ 不另開保存產物的模式。這項腳本修改屬於 commit A。
  * 檢查在同一個 image 裡跑（`--network none`、唯讀掛載），欄位清單從 package 的常數 import（「4.4」），⛔ 不在腳本裡另抄一份。
  * 斷言：
    1. after artifact 存在。
    2. 每一列都有「4.4」的每個欄位（key 存在）。
    3. **至少一列 `decision_fields_available=true`**。全部都沒有有效決策時判失敗，⛔ 不能讓空集合通過。
    4. `decision_fields_available=true` 的列，這些欄位都不是 null（list 可以是空的）；`risk_notes_digest` 是 16 碼十六進位。
    5. `decision_fields_available=false` 的列，這些欄位都是 null，防止沿用上一列的值。
  * 任何一項失敗都算進 smoke 的失敗數，smoke 以非 0 結束。⛔ 不能只看 artifact 有沒有產出。
  * 合成 bundle 如果自然沒有任何有效決策的列，就在 commit A 調整 `make_smoke_bundle.py` 的合成 K 棒，⛔ 不拿掉第 3 條。
* smoke 在 commit A、commit B 的工作樹上各跑一次（工作樹乾淨、與 HEAD 相同，所以 smoke 自己產生的 patch 是空的）。

**兩趟 replay**（由使用者執行，依序跑、⛔ 不並行；每趟約 3 小時、RSS 約 200～315 MiB，在 mem-guard 內）

* `scripts/run-replay-offline.sh` Stage 1，分別用 `AFTER_REF=A`、`AFTER_REF=B`，各自輸出到空目錄。每趟都跑 bundle 的全部列，不只 I-074 的 156 列。
* 每跑一次會留下一個 worktree（I-118），跑完照 `development-workflow.md` 的 worktree 清理規則處理。

**逐列比較工具**（新寫，放 `python/scripts/`，附測試）

* 重用 `replay_bundle/stream.py` 的串流讀取與 `artifacts.compare_rows()`：先串流 A，每列只留精簡投影；再串流 B 邊讀邊比，用 key 對齊。⛔ 不整份載入兩邊（I-117：crossday 的整份載入要 730 MiB）。
* 成交與停損、目標的判定用 bundle 的 candles（每一列至少有 5 根往後的 K 棒）。
* 讀兩份 artifact 時，逐列套用跟 smoke 相同的欄位檢查（見上方「欄位驗證」）；任何一列不符就判「無法歸因」，⛔ 不出結論。
* 成交與判定規則的測試至少包含（除了市價型那一條，數字都是限價 100、停損 98、目標 104）：
  * **第三輪 review 的反例**：次根開 103、高 105、低 99 → 以 100 成交、當根⛔ 不認列目標；之後一根開 99、低 97.5 → 停損、−1R；改成之後一根開 101、高 104.5、低 99 → 目標、+2R。
  * 開盤即成交：次根開 99.5、高 105、低 99 → 以 99.5 成交、當根認列目標（+3R）。
  * 開盤即成交、同根碰停損與目標：開 99、高 105、低 97 → 停損先到。
  * 盤中觸價、當根碰停損：開 103、高 105、低 97 → 當根停損、−1R。
  * 跳空成交在停損之下（市價型，停損 98、次根開盤 97.5）→ 當根停損、−1R，而且有算進平均實現 R。
  * 窗口內都沒碰到 entry → 未成交，不進結果指標、算進成交率。

**分組**（以 A 的 `setup_rr_qualified` 分類；A、B 兩邊的值不同就算異常）

| 組 | 定義 | 要求 |
|---|---|---|
| U：setup 合格 | — | `final_entry_state`、`market_action`、`action`、`position_action`、`risk_notes_digest`、`rr_context` 完全相同；`rr_gate` 只能多 `setup_qualified`；`semantic_entry_permission_state` 只能在 B 的 `rr_gate` 不合格時變 WAIT；`path_gate_state` 只能變 `RR_BLOCKED`。其他任何差異都算異常 |
| G_new：新增放行 | setup 不合格、A 是 BLOCKED、B 是 PROBE | 看下方結果指標 |
| G_wait／G_block | setup 不合格、B 是 WAIT 或結構 BLOCKED | A 必須是 BLOCKED |
| 升級 | PROBE → ENTRY，或 action BuySmall → Buy | 必須是 0 |
| 收緊 | A 是 PROBE／ENTRY、B 比 A 低 | 必須是 0 |
| 對照組 P | A、B 都是 PROBE（既有的試單） | 結果指標的基準 |

每一組都依 `lifecycle_phase` 與 symbol 列出分布。

**成交規則**（決策在 as-of 那根收盤後做出，訂單從下一根起生效；窗口是 as-of 之後的 5 根）

| 進場類型（`rr_context.price_basis`） | 成交 | 成交當根 |
|---|---|---|
| 市價型（`CONTINUATION_MARKET_PRICE`、`RECLAIM_CLOSE`） | 下一根的開盤價成交 | 整根都在成交之後：停損、目標照常判定 |
| 限價型（`PRIMARY_SUPPORT_UPPER`），**開盤即成交**（`open ≤ entry_price`） | 開盤價成交 | 整根都在成交之後：停損、目標照常判定 |
| 限價型，**盤中觸價**（`open > entry_price ≥ low`） | `entry_price` 成交 | **只判停損，⛔ 不認列目標**（v4，回應第三輪 review P1-1） |

* 限價型的成交 K 棒是窗口內第一根 `low ≤ entry_price` 的 K 棒；窗口內都沒碰到就是**未成交**。
* **盤中觸價那一根為什麼只判停損**：開盤在 entry 之上，價格一定先往下穿過 entry 才到得了 low，所以當根的 low 必然在成交之後，停損判定成立；當根的 high 卻可能出現在成交之前（例：限價 100、停損 98、目標 104，次根開 103、高 105、低 99。可能先漲到 105 再跌回 100 才成交），看不出先後就⛔ 不認列。目標從下一根起才算；窗口內之後都沒碰到，就照下方規則記未實現 R。
* 未成交的列不進結果指標，但要報告成交率。
* 停損、目標固定用訊號當下的 `rr_context.stop_price`、`target_price`。

**停損與目標的判定**（從成交那根算到窗口結束；成交當根依上表）

* 停損觸及：`low ≤ stop_price`。成交價本身已經 ≤ 停損（市價或開盤即成交的跳空）就算成交當根停損。
* 目標觸及：`high ≥ target_price`。
* **同一根同時碰到停損和目標，一律算停損先到**（日 K 看不出先後，採保守判定）。
* 每筆的實現 R（成交價 > 停損時，風險＝成交價 − `stop_price`）：停損 → `(min(open, stop_price) − 成交價) / 風險`（之後某根跳空低開按開盤價計，會比 −1 更差；盤中觸價那根停損時開盤在停損之上，就是 −1）；目標 → `(target_price − 成交價) / 風險`；兩者都沒碰到 → 窗口最後一根收盤的未實現 R。
* **成交價 ≤ 停損**（風險無法定義）：算停損，實現 R 一律記 −1（v4）。⛔ 不排除這些列，排除的話最差的列會從平均實現 R 裡消失。

**結果指標與分母**

| 指標 | 分母 | 最低有效樣本 |
|---|---|---|
| 停損觸及率 | 已成交的列 | G_new、P 各 ≥ 30 |
| 先到目標的比例 | 已成交、而且 `target_price` 有值的列（P 只取這個子集 P_q） | G_new、P_q 各 ≥ 30 |
| 平均實現 R | 已成交、而且 `stop_price` 有值的列（成交價 ≤ 停損的列記 −1，⛔ 不排除） | G_new、P 各 ≥ 30 |
| `forward_return` 的平均與正報酬比例（governance 用的那個） | 有 `forward_return` 的列 | G_new ≥ 30 |
| 最大不利偏移（從成交價起算，中位數與 p90） | 已成交的列 | 只報告，⛔ 不當判準 |

G_new 另外要求 ≥ 5 檔，而且單一檔 ≤ 50%。

**判準**（⚠️ 預設值，確認本計畫書時一併確認）

1. **結構不變式**：U 的異常＝0、升級＝0、收緊＝0、setup 缺值的列沒有任何 PROBE／ENTRY。不成立就是程式錯誤：停下來修，⛔ 不進入下兩項。
2. **證據充分**：上表每一項判準指標的有效樣本都達到最低值，G_new 的檔數與占比也符合。**任何一項不足就判「證據不足」→ 退回「4.5」**，⛔ 不改用較寬的判準（v3，回應第二輪 review P1-1）。
3. **安全**（每一項都要成立，否則退回「4.5」）：
   * G_new 的停損觸及率 ≤ P 的停損觸及率 + 5 個百分點。
   * G_new 先到目標的比例 ≥ P_q 的比例 − 5 個百分點。
   * G_new 的平均實現 R ≥ 0。
   * G_new 的平均 `forward_return` ≥ 0，正報酬比例 ≥ 0.45。

量測結果與判讀寫進本計畫書的下一版。

⚠️ **governance 的離線數字⛔ 不當判準**：DB 路徑每檔只取最近 200 列，而且每一列會接前一列的事件狀態，用全量 artifact 重現不出 live 的母體。

### 6.6 dev stack

用 `docker-compose.dev.yml` 重建 python 服務，對一檔觸發分析，確認 API 回傳的版本字串是 p5／p3、`rr_gate.setup_qualified` 存在、前端的 RR 原因顯示中文標籤。

## 七、部署與回滾時的 governance 與快照規則（回應兩輪 review 的 P2-5、P1-2；由部署計畫執行，live 操作都由使用者執行）

**現況事實**：

* governance 只用 model hash 與 schema 查最新一筆 decision replay 結果，不分語意版本。live 的 `sr_evaluation` 排程停用，所以舊結論會一直沿用。
* 分析時才套用 governance；快照存的是當時的決策，**重用快照時⛔ 不會重新套用最新的 governance**。
* 快照重用（`POST /sr-zones` 帶 `reuse_existing=true`、trade analysis 沒帶 `force_refresh`）只檢查 timeframe 和 24 小時內，不看版本。
* `POST /scheduler/sr-analysis/run` 會跳過「最新 K 棒不是今天」與「今天已分析過」的標的，所以**觸發成功、甚至整輪成功，都⛔ 不代表快照已經換版**，不能拿來刷新。
* `POST /sr-zones` 帶 `reuse_existing=false`（預設）每次都重新分析並寫入一筆快照。

**程序**（部署與回滾都照這個順序；「版本」指 p5／p3（部署後）或 p4／p2（回滾後））：

1. 切換 image（部署或回滾）。
2. **先更新 governance**：`POST /scheduler/sr-evaluation/run` 跑一次 decision replay，等工作狀態成功。再唯讀確認同一個 model hash 有一筆建立時間晚於步驟 1 的 regression 結果；live 從來沒有這類結果時照實記錄（gate 是 no-op）。live 的負載能不能承受，在部署計畫裡確認。
3. **再刷新快照**：唯讀查出「過去 24 小時內有 1d 快照的標的」與「持有部位的標的」的聯集，逐檔呼叫 `POST /sr-zones`（`reuse_existing=false`）。⛔ 不用 sr-analysis/run。
4. **逐檔核對**：每一檔的最新 1d 快照建立時間晚於步驟 2 完成的時間，而且決策的版本字串符合預期。核對不過的標的重跑步驟 3；全部通過才算完成。
5. 步驟 1～4 完成之前，trade analysis 一律帶 `force_refresh=true`，不依賴重用的快照。

步驟 3～4 由部署計畫提供可重跑的腳本，輸出逐檔的核對結果。回滾不需要動 DB。

上 live 的關卡仍是「6.5」的離線量測，⛔ 不依賴 live 的 governance 擋下新版。

## 八、執行步驟

1. ✅ 寫計畫書 v1，把 T-065 改列 I-129。
2. ✅ v2：依第一輪 review 修訂。
3. ✅ v3：依第二輪 review 修訂。
4. ✅ v4（本版）：依第三輪 review 修訂。**停下來，等使用者確認本計畫書（含「6.5」的判準）。**
5. **commit A**：改 `evaluation.py` 與 `replay_bundle` 的欄位常數（「4.4」）、補單元測試，把「6.5」的欄位檢查接進 `scripts/smoke-replay-offline.sh`（合成 bundle 沒有有效決策列時一併調整 `make_smoke_bundle.py`），跑 `python/scripts/test.sh`。由使用者 commit 到分支；commit 之後在乾淨的工作樹上跑 smoke（含欄位檢查，HEAD＝A）。
6. 先寫「6.1」的測試，確認在 commit A 的程式上會紅。
7. 改 `decision_engine.py`（「4.2」），更新「6.2」的既有測試，跑 `python/scripts/test.sh`。改前端（「4.3」），跑前端測試。由使用者 commit 成 **commit B**。
8. 寫「6.5」的逐列比較工具（附測試）；commit B 之後在乾淨的工作樹上跑 smoke（含欄位檢查，HEAD＝B）。比較工具若在 commit B 之後才寫，要另外 commit，⛔ 不能讓 B 的工作樹不乾淨地跑 smoke。
9. 使用者依序跑兩趟 Stage 1，再跑比較工具。Claude 依判準判讀，寫進下一版；不通過或證據不足就照「4.5」退回。
10. 歸檔主題文件（「九」），把 I-129 標成「已實作／待 review」。列出變更檔案、commit 建議與指令；Claude ⛔ 不 commit。

## 九、完成後的歸檔位置

* `sr-zone-scoring.md`：
  * 「RR 語意分層」改寫。
  * 「已知的單向性」改成現況規格：進場許可的 RR 判斷看 execution gate，setup 不合格最多試單。
  * 補「daily confirmation 不看 RR」。
  * 改掉「RR 不合格時進場一律仍是 BLOCKED」那一段。
  * I-074「156 列都維持 BLOCKED」加註：那是改前行為的紀錄。
  * `_entry_action_state()` 與 `strong` 仍讀 setup RR 的事實記一行。
  * 「6.5」的量測結果摘要。
* `database-schema.md` 的欄位說明、`api-reference.md` 的版本字串與 `rr_gate.setup_qualified`。
* `development-workflow.md`：逐列比較工具的用法（若保留在 repo）。
* `sr-zone-scoring.md`「Decision Replay 的診斷欄位」：「4.4」的量測欄位（validation-only）。
* `decision_engine.py` 註解裡指向 I-129 的活指標，改指主題文件。
* `sr-zone-scoring.md` 裡帶行號的 `decision_engine` 引用刪除時，同步減 `scripts/doc-refs-legacy.txt` 的額度。

## 修訂紀錄

* **v1（2026-11-03）**：execution gate 單一決定，setup 不合格的列完全依 executable RR 放行。
* **v2（2026-11-03）**：review 指出 v1 不足以證明放寬是安全的（5 項，全部經讀碼確認屬實）。修正：
  * P1-1 市價、target 未知時，setup 不合格的列也會全放行 → D3：要求 executable RR 可量化，否則沿用 setup 判定；放寬上限改成試單。
  * P1-2 放寬範圍不只 156 列，已合格的列也會因 `strong` 改變而升級 → D4：保留 `strong` 的 setup 條件；「6.5」改成全母體逐列比較，分組檢查升級與收緊。
  * P1-3 現有指標可能把危險的放行判成改善 → 新增放行組與對照組分開算，加入停損觸及、先到目標、最大不利偏移與逐檔分布，判準在量測前定好，證據不足就退回。
  * P2-4 量測方法不可行 → 「6.5」寫定輸入一致性檢查、同一 image、同一 tooling patch、串流比較工具；governance 的離線數字不當判準。
  * P2-5 舊 governance 與快照會跨版本沿用 → 「七」的部署與回滾規則。
* **v3（2026-11-03）**：第二輪 review 指出 4 項（全部經讀碼確認屬實）。修正：
  * P1-1 對照組不足時會略過停損類判準 → 任何判準的樣本不足一律「證據不足」並退回，⛔ 不改用較寬的判準（D5）。
  * P1-2 sr-analysis/run 會跳過標的，而且先刷快照、後更新 governance 會讓快照帶著舊的治理結果 → 「七」改成先更新 governance，再用 `POST /sr-zones` 逐檔強制分析並逐檔核對版本與建立時間，部署與回滾都適用。
  * P2-3 停損與目標的判定不完整 → 「6.5」明訂成交規則（市價下一根開盤、限價要碰到才算成交）、同根一律算停損先到、每項指標的分母與最低有效樣本，先到目標的比較只用 target 有值的子集。
  * P2-4 smoke 會覆寫外部 patch → 改成 D6：量測欄位做成獨立的 commit A，兩趟都⛔ 不用 tooling patch；欄位值用單元測試對 decision summary 逐一比對，smoke 的產出另外檢查欄位存在而且不是 null。
* **v4（2026-11-04）**：第三輪 review 指出 2 項（全部經讀碼確認屬實）。修正：
  * P1-1 限價盤中觸價那一根，可能把成交前的高點算成達標 → 「6.5」的成交規則分成「開盤即成交」與「盤中觸價」：盤中觸價那一根只判停損（當根的 low 必然在成交之後）、⛔ 不認列目標；比較工具的測試補上 review 的反例與其他邊界案例。
  * P2-2 smoke 結束時刪掉暫存目錄，外部檢查程式讀不到 artifact → 欄位檢查直接接進 `scripts/smoke-replay-offline.sh`（清理之前、同一個 image、欄位清單從 package 常數 import），並要求至少一列 `decision_fields_available=true`；腳本修改列入 commit A。
  * 修訂時自行發現的兩處，一併修正：
    * 「4.4」原本只改 `_decision_fields_from_summary()`，但 replay 列是逐欄組出來的，新欄位不會自動進到列裡 → 欄位清單改成 package 常數，replay 列依它加入，decision 不可用時填 null。
    * 平均實現 R 原本排除「風險 ≤ 0」的列，等於把跳空開在停損之下、最差的列從平均裡拿掉 → 成交價 ≤ 停損一律記 −1R，⛔ 不排除。

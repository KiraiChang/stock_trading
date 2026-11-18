# I-135：backtrader 引擎取出交易明細的錯誤（計畫書 v1.3）

> 狀態：v1.2 review 後改成 v1.3，待確認。對應：`docs/issue.md` I-135。需求確認：2026-11-18（使用者：欄位一起修、查 live 的 2 筆 `failed`、先寫計畫書）。裁決：D1 採 (a)、D2 改成帶 `+08:00` 的時間（2026-11-18，含補充，見第三節第 4 點）。

## 一、目標與不做的範圍

**目標**

1. `breakout_v1`（backtrader 引擎，`STRATEGY_MAP`）的回測有交易時能正常完成，⛔ 不再 `failed`。
2. 寫進 `backtest_trades` 的每一筆交易明細，價格、股數、報酬率、方向、時間都從**實際成交紀錄**算出來，而且有明確的定義與精度界線。
3. 補上真的走到「backtrader 有交易」的測試（目前沒有任何測試走到這裡，所以這個 bug 一直沒被抓到）。

**不做的範圍**

* modular 策略（`STRATEGY_PRESETS`）的回測：⛔ 不動。
* **費用模型（`TWCommission`）**：另立 `issue.md` **I-136**（優先處理）。本案的 `commission` 欄位照「broker 實際扣、分攤到這筆交易的金額」輸出（`t.commission`＝這筆交易的 Σ `history.event.commission`），⛔ 不宣稱它含交易稅或費率正確；`_extract_result`（`backtest_results` 的總表）也照舊，⛔ 不動。
* **`breakout_v1` 每次只買 1 股**：另立 `todo.md` **T-085**（部位大小設計，與 `pnl_pct` 的口徑統一一起規劃）。
* **回測只取最近 N 根 K 棒**（backtrader 1000 根、modular 2000 根）：另立 `issue.md` **I-137**。
* **live 的 2 筆 `failed`**（2026-07-01，第二節）：都是 modular 策略、原因跟本案無關；`303cdd7` 已有對應的轉型修正與回歸測試，歷史失敗不足以證明現在的程式還有問題，⛔ 不另立（使用者裁決：現行版本重現了再立案）。
* 既有資料：⛔ 不改（第四節說明為什麼不需要）。

## 二、現況（2026-11-18 查證）

* **兩層迴圈**：`strategy._trades` 是 `defaultdict`（data → `AutoDictList`（tradeid → 交易的 list））。現在的 `for trade in strat._trades.values(): for t in trade` 拿到的 `t` 是 tradeid（int），所以有交易就丟 `'int' object has no attribute 'isclosed'`；0 筆交易時迴圈不執行，看起來正常。dev 實跑（2330、2024-01-01～2025-12-31）就是這個錯誤。
* **交易結束之後欄位會歸零**（在測試 image 用合成資料實跑 backtrader 查證）：已結束的 `Trade` 是 `size 0`、`value 0`。現在的公式因此寫出 `exit_price` 0、`size` 0、`pnl_pct` 0。
* **有 `tradehistory=True` 才拿得到每次成交**：開了之後 `t.history` 每一筆有 `event.size`（正＝買、負＝賣）、`event.price`、`event.commission`。現在的 `bt.Cerebro(stdstats=False)` 沒有開，`history` 是空的。
* **`t.price` 不一定是全部進場成交的加權均價**（review 2026-11-18，依 backtrader `Trade.update` 的公式）：加碼時 `t.price` 會重算成新的平均成本，但**減碼之後再加碼**時，平均成本是以「剩下的部位」為基礎算的。例：買 10@100 → 賣 5@110 → 買 5@120 → 賣 10@130，最後 `t.price` 是 110，但全部進場成交的加權均價是 106.666…；拿 `t.price` 當進場價算出的毛損益是 200，實際是 250。所以進場價也要從成交紀錄算，⛔ 不用 `t.price`。
* **費用模型的既有問題**（本案⛔ 不修，另立 I-136）：
  * `TWCommission` 是 `COMM_PERC`、沒有設 `percabs=True`，backtrader 會把 `commission` 參數除以 100：設定 `0.001425`，實際生效的是 `1.425e-05`（在測試 image 查證）。
  * **broker 實際扣費走 `getcommission()`**（v1.1 review：backtrader 的 `bbroker` 從 `getcommission()` 取扣款，`_getcommission()` 的回傳值⛔ 沒有計入費用；v1.1 把 `_getcommission()` 的 3.01425 當成實際扣費是錯的，「交易稅大致對」的結論撤回）。實跑 `TWCommission` 核對：買 10 股 @100、賣 10 股 @100 的 `order.executed.comm` 都是 `0.01425`，`t.commission` 是兩者合計 `0.0285`——**完全沒有交易稅**，手續費也只有應有的 1/100。
  * 所以本案的 `commission` 只能說是「broker 實際扣的金額」，⛔ 不代表台股的實際費用。
* **費用要以交易為單位、⛔ 不能用委託的 `order.executed.comm` 相加**（v1.2 review，依 backtrader 的 `order.py`／`trade.py`）：`order.executed.comm` 是**委託**的累計費用，分批成交時是累加的；**反手**的委託（例如持有多單 10 股時賣 20 股）同時平舊倉、開新倉，backtrader 把它拆成兩筆交易、費用依股數分攤到各自的成交事件。實跑查證（費率 1%）：買 10@100（費用 10）→ 賣 20@110（委託費用 22）→ 買 10@90（費用 9）拆成兩筆交易：多單 events `(+10@100, 10)`、`(−10@110, 11)`，`t.commission` 21；空單 events `(−10@110, 11)`、`(+10@90, 9)`，`t.commission` 20。兩筆的 `t.commission` 都等於各自的 Σ `event.commission`；把委託費用 22 整筆算給任何一筆都不對。
* `direction` 寫死 `"BUY"`；`Trade` 有 `long` 可以判斷。
* **時間**：`bt.num2date()` 回傳**不帶時區**的 UTC（日 K 00:00+08 → 前一天 `16:00:00`），`isoformat()` 寫成不帶時區的字串，存進 `TIMESTAMPTZ` 時照連線的 TimeZone 解讀（live 目前是 UTC，時刻是對的，但依賴連線設定）。modular 路徑寫的是帶 `+08:00` 的字串。
* **`pnl_pct`**：modular 是「淨損益 ÷ 進場時該標的的 equity」（`net_pnl / equity_at_entry`）；backtrader 這條原本寫的是「淨損益 ÷ 進場金額」。modular 目前是全額投入，進場金額就等於該標的的 `equity_at_entry`，所以**兩條路徑目前的分母數值⛔ 不一定不同**；口徑沒有統一定義是事實，統一留作之後的改善。
* **live 唯讀查詢**（2026-11-18，使用者執行）：
  * 2 筆 `failed` 都是 2026-07-01 的 modular 策略，原因跟本案無關：`breakout_swing_atr_v1` 是 `(psycopg2.errors.InvalidSchemaName) schema "np" does not exist`（numpy 型別寫進 SQL）、`breakout_volprofile_composite_v1` 是 `unsupported operand type(s) for /: 'decimal.Decimal' and 'float'`。
  * `breakout_v1`：`done` 1 筆、明細 0 筆；沒有 `failed` 的 `breakout_v1`。→ ⛔ 不存在欄位是 0 的明細。

## 三、修法

1. `bt.Cerebro(stdstats=False, tradehistory=True)`。
2. `_extract_trades` 走兩層：`for by_id in strat._trades.values(): for trades in by_id.values(): for t in trades`，只取 `isclosed`（還沒平倉的部位照舊不列）。
3. **每一筆已結束的交易，全部從 `t.history` 的成交（event）算，先算出未四捨五入的值**：
   * 開倉方向：第一筆成交的正負號（正＝多、負＝空）。
   * **進場成交**＝跟開倉方向同號的成交；**出場成交**＝反號的成交。
   * 驗證：依序累計的部位⛔ 不能變號，最後要回到 0；`history` 是空的、或不符合這些條件，就 raise（任務 `failed`），⛔ 不寫出看起來正常的錯誤數字。反手的委託 backtrader 已拆成兩筆交易，各自的 `history` 只有自己的那一段（第二節實跑），所以⛔ 不會在同一筆裡變號。
   * `size`＝進場成交的股數合計；`entry_price`＝進場成交依股數加權的平均價；`exit_price`＝出場成交依股數加權的平均價。
   * 毛損益（未四捨五入）＝多方 `Σ出場金額 − Σ進場金額`（空方反號），要等於 `t.pnl`（浮點誤差 1e-6 以內），不符就 raise。
4. **欄位定義**（寫進 DB 時才四捨五入）：

   | 欄位 | 定義 | 原本 |
   |---|---|---|
   | `symbol` | `t.data._name` | 同 |
   | `direction` | 多方 `"BUY"`、空方 `"SELL"`（跟 modular 一樣） | 寫死 `"BUY"` |
   | `entry_time` | 第一筆進場成交的時刻（UTC 時刻轉成台北時間，帶 `+08:00`；D2） | `num2date(t.dtopen)`、不帶時區 |
   | `exit_time` | 最後一筆出場成交的時刻（同上） | `num2date(t.dtclose)`、不帶時區 |
   | `entry_price` | 進場成交的加權均價，2 位 | `t.price`（交錯加減碼時不對） |
   | `exit_price` | 出場成交的加權均價，2 位 | 0 |
   | `size` | 進場成交的股數合計（正數） | 0 |
   | `pnl` | `t.pnlcomm`（毛損益扣掉目前費用模型的費用），2 位 | 同 |
   | `pnl_pct` | **D1（使用者裁決）**：淨損益 ÷ **未四捨五入**的進場成交金額（`Σ進場股數×價格`），存比例值（`0.01`＝1%）；DB 是 `DECIMAL(8,4)` | 0 |
   | `commission` | `t.commission`（分攤到這筆交易的費用；驗證它等於這筆的 Σ `history.event.commission`，不符就 raise；⛔ 不用委託的 `order.executed.comm`；⛔ 不代表台股的實際費用，見第二節與 I-136） | `abs(t.pnlcomm - t.pnl)`（數值相同） |

   * **D2（使用者裁決）**：時間從 backtrader 的 UTC 時刻（`num2date` 回傳的不帶時區的 UTC）先標成 UTC、再轉成 `Asia/Taipei`，輸出帶 `+08:00` 的 ISO 字串。⛔ 不能直接把不帶時區的值貼上 `+08:00`（那會差 8 小時）。
   * ⚠️ **D2 的承諾範圍**：PostgreSQL 的 `TIMESTAMPTZ` 收到帶 offset 的字串，存的時刻跟連線的 TimeZone 無關（第六節第 2 點測這件事）；**MySQL** 的 `DATETIME` 收到帶 offset 的值仍會轉成連線的時區再存（[Connector/J 的說明](https://dev.mysql.com/doc/connector-j/en/connector-j-time-instants.html)），SQLite 存的是字串本身。所以⛔ 不宣稱三種 DB 都消除了「依賴連線時區」——只有 PostgreSQL（dev 與 live 實際用的）是。
5. **精度界線**（DB／API 的欄位是四捨五入過的，所以⛔ 不能拿四捨五入後的值精確反算）：
   * 例（review）：買 3@102，賣 1@104、賣 2@105，毛損益是 8；出場均價存成 104.67，反算是 `(104.67 − 102) × 3 = 8.01`。
   * 所以「毛損益＝(出場−進場)×股數」只對**未四捨五入**的成交資料成立（第三節第 3 點驗證）。
   * 對存下來的值，界線是：`|d × (exit_price − entry_price) × size − (pnl + commission)| ≤ 0.005 × 2 × size + 0.01`，其中 **`d` 是方向：`BUY`（多方）是 +1、`SELL`（空方）是 −1**——兩個價格各自最多差 0.005、乘上股數；`pnl` 與 `commission` 各自最多差 0.005。`pnl_pct` 存成 `DECIMAL(8,4)`，跟未四捨五入的值最多差 0.00005。寫進 `database-schema.md`。
   * 空方的例子（v1.1 review）：放空 10@100、回補 10@90，毛利是 100；⛔ 不帶方向的公式算出 `(90 − 100) × 10 = −100`，跟 100 差了 200，遠超過界線 0.11。帶上 `d = −1` 才是 100。

## 四、受影響的檔案、資料流與既有資料

* 程式：`python/backtest/engine.py`（`run_backtest` 的 `Cerebro` 參數、`_extract_trades`、時間轉換）。
* 測試：新增 `python/tests/test_backtest_engine_trades.py`；PostgreSQL 的時刻測試放在 backend 的 PostgreSQL 測試（第六節第 2 點）。
* 腳本：`scripts/smoke-dev-backtest-claim.sh` 加 `trades` 情境，`scripts/test-smoke-dev-backtest-claim.sh` 加對應的假 docker 測試（第六節第 4 點）。
* 資料流：`run_backtest` → `finish_job`（I-134）→ `backtest_trades`。`backtest_results` ⛔ 不變。backend 與前端只讀這兩張表，欄位與型別⛔ 不變，⛔ 不需要改。
* **既有資料**：這個 bug 讓有交易的 `breakout_v1` 一律 `failed`，丟錯是在寫入明細之前，所以 DB 裡⛔ 不存在欄位是 0 的 `breakout_v1` 明細。live 已唯讀確認（第二節）：`breakout_v1` 只有 1 筆 `done`、明細 0 筆。⛔ 不需要修資料。

## 五、主要風險與回滾

| 風險 | 因應 |
|---|---|
| `tradehistory=True` 讓記憶體變大 | 每筆交易多存幾筆成交紀錄；回測最多 1000 根 K 棒 × 少數幾檔，量很小 |
| 加減碼交錯、分批出場算錯 | 進出場價都從成交紀錄加權；毛損益在未四捨五入時要等於 `t.pnl`，不符就 raise；單元測試涵蓋 review 的兩個例子 |
| 部位在同一筆交易裡變號（反手） | backtrader 在部位穿過 0 時會拆成兩筆交易；萬一同一筆的 `history` 出現變號，raise、⛔ 不輸出 |
| 修好之後，`breakout_v1` 原本「失敗」的回測變成「成功」，1 股與費用模型的問題浮上檯面 | ⛔ 不在本計畫（第一節）；已另立 I-136（費用模型）、T-085（1 股） |
| 回滾 | 只改 python 回測引擎，照一般部署（I-134 的 6.1，回測雙零檢查）；回滾退回 `pre-<案號>` 的 python image，⛔ 不涉及 schema |

## 六、測試與驗證

1. **python 單元測試**（`python/scripts/test.sh`；⛔ 不需要 DB）：用合成的日 K 與一個照腳本下單的測試策略，透過 `run_backtest` 同一條路徑（`fetch_candles` 換成假的、`STRATEGY_MAP` 在測試裡加上測試策略）跑 backtrader。費用用測試自己的固定比例（⛔ 不依賴有問題的 `TWCommission`），確認明細照實反映 broker 扣的費用：
   * 一次平倉：每個欄位對照手算的值。
   * **分批出場**（review 的例子：買 3@102，賣 1@104、2@105）：出場均價 104.666…→ 存 104.67、`size` 3、`exit_time` 是最後一次；未四捨五入的毛損益 8 等於 `t.pnl`；存下來的值符合第三節第 5 點的界線。
   * **加減碼交錯**（review 的例子：買 10@100、賣 5@110、買 5@120、賣 10@130）：進場均價 106.666…、`size` 15、出場均價 123.333…、毛損益 250（⛔ 不是用 `t.price` 會得到的 200）。
   * **空方（必測）**：沒有部位時 `self.sell()` 會開空單（2026-11-18 用測試 image 確認 backtrader 開得起來）。放空 10@100、回補 10@90：方向 `SELL`、`entry_price` 100、`exit_price` 90、`size` 10、毛利 100；存下來的值用帶方向的界線檢查。空方也測一次分批回補。
   * **費用**：測試用自己的固定比例費用（⛔ 不依賴有問題的 `TWCommission`），`commission` 要等於這筆交易的 Σ `history.event.commission`。
   * **反手拆成兩筆交易（必測）**：持有多單 10 股時賣 20 股、再回補 10 股（第二節的例子）：輸出兩筆明細，多單與空單各自的股數、進出場價、毛損益與 `commission`（21、20）都對；反手那張委託的費用分攤在兩筆裡，⛔ 沒有整筆算給其中一筆。
   * 多筆交易、兩檔標的：筆數與 `symbol` 都對。沒有交易：空的 list。結束時還沒平倉的部位⛔ 不列。
   * 時間：輸出帶 `+08:00`，而且跟 backtrader 的 UTC 時刻是同一個時刻（例如日 K 2025-01-03 的那根是 `2025-01-03T00:00:00+08:00`，⛔ 不是 `2025-01-02T16:00:00+08:00`）。
   * `pnl_pct`＝淨損益 ÷ 未四捨五入的進場金額（D1）。
   * `history` 空的、部位變號、毛損益對不上：raise。
   * `breakout_v1` 本身：用一組會觸發突破買進、再跌破賣出的合成 K 棒跑一次，至少 1 筆交易、⛔ 不丟錯。構造不出穩定觸發的資料就改用第 4 點的 dev 實跑驗證，並在實作備註寫明。
2. **PostgreSQL 的時刻測試**（D2 的承諾範圍）：在 backend 的 PostgreSQL 測試（`scripts/test-postgres-migrations.sh` 會跑的那組，用的是專用的 postgres）加一項：把同一個帶 `+08:00` 的字串，在 session TimeZone 分別是 `UTC`、`Asia/Taipei`、`America/New_York` 時寫進 `TIMESTAMPTZ`，三次存下的時刻（epoch）相同；同一個不帶時區的字串則三次不同（證明原本的寫法依賴連線設定）。用暫存表，⛔ 不動正式表。
3. **突變**：拿掉第二層迴圈、`tradehistory` 改回關閉、進場價改回 `t.price`、出場價改回原本的公式、時間改成直接貼 `+08:00`，各自要讓測試失敗。
4. **dev 實跑，用 repo 腳本**（v1.1 review：要明列腳本與失敗清理）：在 `scripts/smoke-dev-backtest-claim.sh` 加一個情境 **`trades`**，沿用它的 dev project 限制、開始前的檢查、`i134smoke_<執行時間>_` 前綴、每個呼叫的上限，以及同一套收尾（成功、失敗、逾時、Ctrl+C 都一樣：本次前綴的 `pending` 改成 `failed` → 停掉 python 兩個服務並確認 → 依前綴刪除本次的任務、結果、明細 → 依開始時的紀錄恢復服務；確認不了就⛔ 不刪、⛔ 不恢復、回傳非 0）。
   * 執行：`scripts/smoke-dev.sh` 起 dev stack 之後，`SCENARIOS=trades SMOKE_STRATEGY=breakout_v1 scripts/smoke-dev-backtest-claim.sh`（dev 的 2330 日 K 是 I-134 匯入的；日期用腳本的預設 2024-01-01～2025-12-31）；跑完用 `down` 停掉 dev（⛔ 不帶 `-v`）。
   * `trades` 情境：寫入一筆 `pending`、`POST /backtest`、等到結束。通過條件：`done`；明細筆數等於結果的 `total_trades`，而且至少 1 筆；每一筆的 `size`、`entry_price`、`exit_price` 都大於 0；每一筆都符合帶方向的精度界線（在 SQL 裡算）；`entry_time`／`exit_time` 換成台北時間是 `00:00:00`（日 K 的成交時刻）。任一項不符就失敗、照常收尾。
   * 腳本的假 docker 測試（`scripts/test-smoke-dev-backtest-claim.sh`）加上：`trades` 通過、任務 `failed` 時失敗、界線不符時失敗，三種都有收尾。

## 七、完成後要歸檔的位置與另案

* `docs/database-schema.md` 的 `backtest_trades`：每個欄位的定義（從成交紀錄算、加權均價、`pnl_pct` 的分母與單位）、精度界線、時間格式與 D2 的承諾範圍（只有 PostgreSQL），以及 `commission` 只反映目前的費用模型。
* `docs/architecture.md` 的回測流程：backtrader 路徑要開 `tradehistory`、交易明細從成交紀錄算。
* `docs/development-workflow.md` 的 `smoke-dev-backtest-claim.sh` 那一節：`trades` 情境。
* **另案**（使用者 2026-11-18 裁決）：費用模型 → `issue.md` I-136（優先）；最近 N 根截斷 → `issue.md` I-137（含 modular 的 2000 根）；固定 1 股與 `pnl_pct` 口徑 → `todo.md` T-085；live 的 2 筆 modular `failed` → ⛔ 不立（`303cdd7` 已修，現行版本重現再立）。
* `docs/issue.md` I-135 改成「已實作／待 review」；review 確認後收斂，計畫書在那時才刪。

## 版本紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1 | 2026-11-18 | 初版。 |
| v1.1 | 2026-11-18 | review [P2]×3：費用模型另案、本案⛔ 不宣稱 `commission` 已含交易稅（`TWCommission` 的費率被除以 100、`getcommission` 沒有交易稅，已在測試 image 查證）；進場價改從全部進場成交加權（加減碼交錯時 `t.price` 不對）；毛損益等式只對未四捨五入的成交成立、存下來的值另訂精度界線。裁決：D1 是淨損益 ÷ 未四捨五入的進場成交金額、存比例值，modular 全額投入時分母數值相同、口徑統一留作之後；D2 從 UTC 時刻轉成台北時間，補 PostgreSQL 不同 session TimeZone 的測試，承諾只限 PostgreSQL。live 唯讀查詢的結果（2 筆 `failed` 跟本案無關；`breakout_v1` 沒有明細）。 |
| v1.2 | 2026-11-18 | review [P2]×2：費用的扣費路徑是 `getcommission()`（實跑核對 `order.executed.comm`：完全沒有交易稅），撤回「交易稅大致對」；精度界線加上方向（空方 −1），空方改成必測。dev 驗收改用 repo 腳本：`smoke-dev-backtest-claim.sh` 加 `trades` 情境，沿用它的收尾。另案裁決：I-136（費用模型，優先）、I-137（N 根截斷，含 modular）、T-085（1 股與 `pnl_pct` 口徑）；live 的 2 筆 modular `failed` ⛔ 不立。 |
| v1.3 | 2026-11-18 | review [P2]：`order.executed.comm` 是委託的累計費用（分批成交累加、反手同時涵蓋平倉與開倉），⛔ 不能相加當成每筆交易的費用 → 保留 `t.commission`，以這筆交易的 Σ `history.event.commission` 核對；實跑查證反手拆成兩筆交易的費用分攤（21／20），並補成必測。 |

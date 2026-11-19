# I-136 計畫書 v1.1：backtrader 的費用模型改成「設定費率的模擬模型」

> 狀態：v1.1 已確認（使用者 2026-11-19）；**repo 與 dev 已實作（2026-11-19）、review 通過（2026-11-19）；live 已上版（2026-11-19，`03e7028`）；待收斂**（實作備註與上版紀錄在「七之後」）。對應：`docs/issue.md` I-136。需求確認：2026-11-19（使用者裁決 D1(a)、D2(a)、D3、D4，並補充評估與條件，見第三節）。
> ⚠️ **部署順序（使用者裁決）**：I-135（2026-11-19 review 通過、`7a97ac5` 已 push）**上 live 並驗收完成之後**，才 push I-136；在那之前 I-136 的 commit 只留在本機，免得部署時一起帶上去。
> **I-135 的驗收門檻（使用者裁決，2026-11-19 補記）**：dev 有交易的實跑＋live 部署後檢查。I-135 的版本上⛔ 不在 live 跑有交易的回測：
> 那時的費用模型還沒修正，報酬數字不能參考；改在 I-136 上版後補跑。這個門檻由 Claude 在 I-135 部署後檢查（14:09:11）之後提議，
> 使用者採納，於 14:12:44 push 了含 I-136 的 `03e7028`（時間取自本機 reflog）。補跑在 14:28：明細查詢 0 筆不符（見「七之後」的上版紀錄）。

## 一、目標與不做的範圍

**目標**

1. backtrader 路徑（`STRATEGY_MAP`，目前只有 `breakout_v1`）的 broker **實際扣的費用**＝依 config 費率計算的模擬費用：
   買、賣都收手續費（`COMMISSION_RATE`），賣出另收交易稅（`TAX_RATE`）。
2. 費用由 broker 真正使用的路徑計算，測試以**真的 `TWCommission` 跑 Cerebro**核對每筆成交的扣費、交易的費用、淨損益與 broker 資產變化，
   ⛔ 不只看某個函式的回傳值。
3. 費率來自 config、可以被換掉：用自訂費率與零費率驗證，⛔ 不能是「寫死的值剛好等於預設」。

**不做的範圍**

* **最低手續費、取整數、商品別稅率**（ETF、當沖等）：⛔ 不做，留給 `todo.md` T-085（D1）。
* **modular 路徑**（`STRATEGY_PRESETS`）的費用：只查證、⛔ 不改；查到問題另立 issue（D3）。
* **既有資料**：⛔ 不改、⛔ 不回補（D4）。
* **schema**：⛔ 不新增欄位（D2）。
* `breakout_v1` 每次只買 1 股、`pnl_pct` 的口徑：T-085。

## 二、現況（2026-11-19 查證）

* `TWCommission`（`python/backtest/strategy/base.py`）是 `COMM_PERC`、⛔ 沒有設 `percabs=True`。backtrader 1.9.78.123（`python/requirements.txt` 鎖定的版本）初始化時
  會把 `commission` 除以 100：設定 `0.001425`，生效的是 `1.425e-05`（I-135 查證）。
* `TWCommission` **自己覆寫了 `getcommission()`**，只算手續費；交易稅寫在 `_getcommission()`。broker（`BackBroker._execute`）實際扣款呼叫的是
  `getcommission(closed, price)` 與 `getcommission(opened, price)`，所以交易稅⛔ 從來沒被扣過。
* 上游的 `CommInfoBase.getcommission(size, price)` 本身就是 `return self._getcommission(size, price, pseudoexec=True)`（測試 image 裡的原始碼查證，
  與 [上游 comminfo.py](https://raw.githubusercontent.com/mementum/backtrader/master/backtrader/comminfo.py) 相同）：只要⛔ 不覆寫 `getcommission()`，
  扣款就會走 `_getcommission()`。
* broker 傳進來的 `closed`／`opened` 帶有**委託的正負號**（賣出是負數）。所以 `size < 0` 就是賣出，平多單的賣出與開空單的賣出都會課稅。
* **修正後的模型在測試 image 實跑**（`percabs=True`、拿掉覆寫的 `getcommission()`、稅寫在 `_getcommission()` 的 `size < 0`）：
  * 買 10 股 @100：`order.executed.comm` 1.425；賣 10 股 @100：4.425（手續費 1.425＋稅 3）；這筆交易 `t.commission` 5.85、淨損益 −5.85；
    broker 的現金與資產都是 1,000,000 − 5.85。
  * 空單（賣 10 @100、買回 10 @100）：賣出那筆 4.425、買回 1.425，合計 5.85。
  * 反手（買 10、賣 20、買回 10，都 @100）：賣 20 那張委託扣 8.85，拆成兩筆交易，各自 events 是 `(+10, 1.425)`、`(−10, 4.425)` 與
    `(−10, 4.425)`、`(+10, 1.425)`，兩筆的 `t.commission` 都是 5.85。
* `engine.py` 用 `TWCommission()` 不帶參數，費率是 class 定義時從 `config` 取的預設值。
* **同一張委託分多次成交**（測試 image 實跑，`FixedSize(size=4)` 的 filler，買 10、賣 10 都 @100）：
  * 每次成交是一個 execution bit（`order.executed.exbits`），各自帶這一次的費用：買進 `(4, 0.57)`、`(4, 0.57)`、`(2, 0.285)`；賣出 `(−4, 1.77)`、`(−4, 1.77)`、`(−2, 0.885)`。
  * `order.executed.comm` 是**累計**值（0.57 → 1.14 → 1.425），⛔ 不是單次成交的費用；單次的費用是 execution bit 的 `comm`，或前後兩次通知的累計差額。
  * 交易的 `history` 每次成交一個 event（上面六筆），`t.commission` 5.85＝Σ `event.commission`；broker 資產 1,000,000 − 5.85。
* **I-135 測試的費用替身 `FixedRate`**（`python/tests/test_backtest_engine_trades.py`，在 `run` fixture 裡用 `monkeypatch` 直接替換 `engine.TWCommission`）：
  backtrader 的 params 機制會把建構參數套進同名的參數，未定義的參數則傳到 `__init__`（[上游 metabase.py](https://raw.githubusercontent.com/mementum/backtrader/master/backtrader/metabase.py)）。
  實跑：`FixedRate(commission=0.5)` 的 `p.commission` 變成 0.5（蓋掉測試原定的 1%）；`FixedRate(tax=0.003)` 丟
  `TypeError: CommInfoBase.__init__() got an unexpected keyword argument 'tax'`。所以第四節第 2 點改成帶參數建立之後，**這個替身要一起改**。
* **影響的方向**（修正 I-136 原本「偏離」的說法）：broker 扣的費用**偏低**（手續費只有 1/100、完全沒有稅），所以**淨損益被高估**；
  `backtest_trades` 的 `commission` 偏低、`pnl` 偏高；`backtest_results` 的報酬、平均損益偏高，勝率、勝敗場、夏普值、最大回撤也可能因此改變
  （淨損益接近 0 的交易可能換邊）。
* **modular 路徑**（D3 的查證起點）：`run_backtest` 把 `config` 的 `COMMISSION_RATE`、`TAX_RATE` 傳給 `run_modular_backtest` → `BacktestEngine`；
  `BacktestEngine._close` 進出場都收手續費，多單在出場、空單在進場（賣出那一邊）課稅。函式的參數預設值也是 0.001425／0.003，所以要用**自訂費率**
  驗證真的是傳進來的值。

## 三、裁決（2026-11-19，使用者）

| 裁決 | 內容 |
|---|---|
| D1：按 config 比例計費 | ⛔ 不設最低費用、⛔ 不取整數；broker 計算保留小數，寫進 DB 時沿用 I-135 的精度規則（2 位、精度界線）。定位是「**設定費率的模擬模型**」。最低費用與商品別規則留給 T-085。⚠️ 每筆最低 20 元⛔ 不是所有券商的通用規則，券商可以自訂最低費用（[證交所規定](https://twse-regulation.twse.com.tw/TW/law/DOC01.aspx?FLCODE=FL007304&FLNO=94)）。 |
| D2：`commission` 含稅 | ⛔ 不新增欄位。定義為「**分攤到這筆交易的模擬總費用**」，包含手續費與賣出的交易稅；照 I-135 繼續用這筆交易的 Σ `history.event.commission` 核對，保留反手的分攤規則。 |
| D3：modular 只查證 | 共同入口把 config 的費率傳進去，modular 也依賣出方向課稅。驗證要用**自訂費率**，確認⛔ 不是預設值剛好相同；查到問題另立 issue。 |
| D4：⛔ 不改既有資料 | 用部署的版本、image 與時間記下新舊界線；部署前重查受影響的清單。11-18 的「`breakout_v1` 1 筆 `done`、0 筆明細」只代表當時的觀察；要新的結果就另建 job 重跑。 |

## 四、修法

1. `TWCommission`：
   * `params` 加 `("percabs", True)`：`commission`、`tax` 都是比例值（`0.001425`＝0.1425%），⛔ 不再被除以 100。
   * **拿掉自己覆寫的 `getcommission()`**，讓父類別委派給 `_getcommission()`；手續費與 `size < 0` 的交易稅都集中在 `_getcommission()` 一個地方算。
   * docstring 寫清楚：設定費率的模擬模型、比例值、賣出（`size < 0`，含開空單）課稅、⛔ 沒有最低費用與取整數（T-085）。
2. `engine.py`：`TWCommission(commission=COMMISSION_RATE, tax=TAX_RATE)`，明確傳入 config 的費率，跟 modular 的共同入口一致；測試因此可以換掉
   `engine` 的費率來驗證「費率真的來自 config」。
   * **I-135 測試的替身一起改**（v1 review [P2]）：`run` fixture 改成替換一個**工廠**：接受 `commission`、`tax` 兩個關鍵字參數（新的建構介面），
     記下收到的值，回傳 `FixedRate()`——**⛔ 不把收到的費率傳給 `FixedRate`**，維持測試獨立的固定 1%。fixture 另外確認 `engine` 傳了這兩個參數
     （介面改了卻沒跟上時會失敗，⛔ 不靜默）。I-136 的新測試⛔ 不用這個替身，一律用真的 `TWCommission`。
3. `_extract_trades`（I-135）⛔ 不改：`commission` 仍是 `t.commission`，並核對等於 Σ `event.commission`；只是現在這個值含稅。

## 五、受影響的檔案、資料流與既有資料

* 程式：`python/backtest/strategy/base.py`（`TWCommission`）、`python/backtest/engine.py`（建立 `TWCommission` 的地方）。
* 測試：新增 `python/tests/test_tw_commission.py`（第七節）；`python/tests/test_backtest_engine_trades.py` 的獨立費用測試（`FixedRate`）**保留**：
  `run` fixture 改成替換工廠（第四節第 2 點），檔頭說明改成「費用用獨立的固定比例，跟 `TWCommission` 分開驗」。
* 腳本（第七節第 4 點，使用者裁決要做）：`scripts/smoke-dev-backtest-claim.sh` 的 `trades` 情境加費用核對；`scripts/test-smoke-dev-backtest-claim.sh` 加對應的假 docker 測試。
* 共用查詢：新增 `backend/internal/database/testdata/backtest_trades_fee_check.sql`；PostgreSQL 測試新增一支（`TestPostgresMigrations` 開頭才會被 `scripts/test-postgres-migrations.sh` 執行）。
* 資料流：`run_backtest` → `finish_job`（I-134）→ `backtest_results`、`backtest_trades`。欄位與型別⛔ 不變；backend 與前端只讀，⛔ 不需要改。
* **既有資料（D4）**：
  * ⛔ 不改、⛔ 不回補。
  * **部署前重查受影響的清單**（live 唯讀，指令在部署時給）：`backtest_jobs` 裡 `strategy` 屬於 `STRATEGY_MAP`（目前只有 `breakout_v1`）的筆數、
    狀態分布與最後一筆的時間，以及它們的 `backtest_trades` 筆數。
  * **新舊界線**記在本計畫書的部署紀錄，收斂時歸檔到 `database-schema.md`：部署的 commit、python image ID、部署完成的時間。
    界線之前完成的 backtrader 結果，費用偏低、淨損益高估；要新的結果就另建 job 重跑。

## 六、主要風險與回滾

| 風險 | 因應 |
|---|---|
| 賣出課稅的判斷依賴 broker 傳入的正負號 | 測試 image 實跑確認（第二節）；單元測試涵蓋多單、空單、反手，每筆成交逐一核對 |
| 費用變高之後，現金檢查擋下原本會成交的委託 | `breakout_v1` 每次 1 股，影響可以忽略；測試以足夠的資金跑，另外核對 broker 資產變化 |
| 回測數字跟之前不同，使用者誤以為策略變差 | D4 的新舊界線記錄並歸檔；`database-schema.md` 寫明 |
| modular 查到問題 | ⛔ 不在本案修，另立 issue（D3） |
| 回滾 | 只改 python 回測引擎，⛔ 不涉及 schema；照一般部署（I-134 的 6.1，回測雙零檢查），回滾退回 `pre-<案號>` 的 python image |

## 七、測試與驗證

1. **實際扣款**（`python/tests/test_tw_commission.py`；⛔ 不需要 DB）：用**真的 `TWCommission`** 跑 Cerebro（合成的日 K、照腳本下單的策略，跟 I-135 的測試同一種做法）。
   每個案例核對：每次成交的費用、每筆交易的 `t.commission`（＝Σ `event.commission`）、淨損益（`t.pnlcomm`）、broker 的現金與資產變化。
   * **broker 資產變化用未四捨五入的值核對**（v1 review [P2]）：`broker.getvalue() − 初始資金` 與 Σ `t.pnlcomm` 的差在 `1e-6` 以內。
     ⚠️ 這個等式**只在期末全部平倉時成立**（否則資產裡有未實現損益）：每個用它的案例都在期末平掉所有部位，並先斷言 broker 的部位是 0。
   * **基本案例**：買 10 股 @100 扣 1.425、賣 10 股 @100 扣 4.425，合計 5.85；淨損益 −5.85；broker 資產少 5.85。
   * **空單**：賣出開倉那一筆課稅，買回那一筆⛔ 不課稅。
   * **分批：多張委託**：分兩張委託進場、兩張委託出場，每張委託各自一次成交；每張的 `order.executed.comm` 就是那一次的費用，賣出課稅、買進⛔ 不課稅，
     `t.commission` 等於四張合計。
   * **分批：同一張委託多次成交**：用 `FixedSize` filler 讓一張委託分多次成交（第二節的實跑：4＋4＋2）。每次成交的費用用 execution bit 的 `comm`
     （或前後兩次通知的累計差額）核對；⛔ 不把累計的 `order.executed.comm` 當成單次成交的費用。另外核對每次成交各有一個 `event`、費用相同，
     以及累計值在完成時等於各次合計。
   * **反手**：賣 20 那張委託扣 8.85，兩筆交易各自 5.85（第二節的實跑數字）。
   * **自訂費率**：換掉 `engine` 的 `COMMISSION_RATE`、`TAX_RATE`（例如 0.002、0.004，⛔ 不等於 config 的值），透過 `run_backtest` 跑，扣費照自訂費率算（證明費率來自 config、
     而且真的被傳進去）。
   * **零費率**：兩者都是 0 時扣費是 0、淨損益等於毛損益。
   * **`run_backtest` 這條路徑**（期末全部平倉）：
     * 明細：每筆的 `commission`、`pnl` 跟手算的未四捨五入值差 ≤ 0.005（存兩位）。
     * 總表：`total_return` 存六位，⛔ 不乘回本金要求等於淨損益（例：本金 1,000,000、淨損益 −5.85，存成 −0.000006，乘回是 −6.00）；改成跟未四捨五入的
       `Σ 淨損益 ÷ 本金` 差 ≤ 5e-7。`avg_pnl`（四位）同理，差 ≤ 5e-5。
2. **modular 查證（D3）**：用自訂費率（⛔ 不等於參數預設值）呼叫 `run_backtest` 的 modular 路徑，確認 `BacktestEngine` 收到的是 config 的值；
   直接核對 `_close` 的多單與空單費用（多單在出場課稅、空單在進場課稅）。查到不一致就停下來回報、另立 issue。
3. **回歸與突變**：
   * I-135 的 `test_backtest_engine_trades.py`（`FixedRate`，經由工廠替換）全部照樣通過。
   * 突變，各自都要讓第 1 點的測試失敗：
     * 拿掉 `percabs=True`。
     * 漏稅：拿掉 `_getcommission()` 裡 `size < 0` 的稅。
     * 恢復自己覆寫的 `getcommission()`（只算手續費）。
     * **費率寫死，改在實際傳值或扣費公式的地方**：`engine.py` 傳字面值（`TWCommission(commission=0.001425, tax=0.003)`）；`_getcommission()` 用字面值取代
       `self.p.commission`／`self.p.tax`。⛔ 不用「把 params 的預設值改成相同的字面值」當突變：預設值會被建構參數蓋掉，而且跟 config 的值相同，是等價突變、驗不到行為。
4. **dev 實跑，用 repo 腳本**（使用者裁決要做）：
   * 執行：`scripts/smoke-dev.sh` 起 dev stack 之後，`SCENARIOS=trades SMOKE_STRATEGY=breakout_v1 scripts/smoke-dev-backtest-claim.sh`；跑完 `down`（⛔ 不帶 `-v`）。
   * **費率的來源**：預期值用 **config 的原始費率**——在 python-server container 裡讀 `config` 載入的 `COMMISSION_RATE`、`TAX_RATE`（就是 `python/config.yaml` 的
     `backtest.commission_rate`／`tax_rate`），⛔ 不讀 `TWCommission` 實例的參數（那是被測的對象，換算錯了會兩邊一起錯）。在寫入任何測試任務之前讀；
     讀不到、或不是有限的非負數，就結束碼 2、⛔ 沒有改任何東西。
   * **費用核對**（I-135 原本的通過條件照舊）：每一筆明細的 `commission` 跟預期值的差，⛔ 不能超過 `0.005 × size × (2c + t) + 0.005`：
     * 多單 `size × (entry_price × c + exit_price × (c + t))`；空單 `size × (entry_price × (c + t) + exit_price × c)`。
     * 界線的來源：兩個價格各自四捨五入最多差 0.005、`commission` 本身最多差 0.005。
   * **共用查詢**：`backend/internal/database/testdata/backtest_trades_fee_check.sql`（參數 `$1`＝job_id、`$2`＝前綴、`$3`＝手續費率、`$4`＝稅率），寫法跟 I-135 的
     `backtest_trades_check.sql` 一樣（好的條件 AND 起來、`IS NOT TRUE` 判壞、必要欄位 `IS NOT NULL`、排除 NaN）。dev 腳本用 `PREPARE`／`EXECUTE`，
     費率用 psql 變數傳入。
   * **PostgreSQL 實測**：新增 `TestPostgresMigrationsBacktestTradesFeeCheck`，用 `go:embed` 讀同一份查詢、Go 綁參數，暫存表跟 I-135 的做法相同
     （`LIKE public.backtest_trades`）。案例：多單與空單剛好符合預期值、界線內外各一（多單與空單）、稅算錯邊（空單套用多單公式）、
     `commission`／`size`／價格／`direction` 是 NULL、`commission` 是 NaN、前綴不符的⛔ 不算。
   * **假 docker 測試**：送出的查詢就是共用檔的內容；費用不符 → rc 1、照常收尾；費用查詢失敗 → rc 1、照常收尾；讀不到費率、或費率不是有效數字 → rc 2、⛔ 沒有改任何東西。
5. 完整驗證：`python/scripts/test.sh`（全部）、`scripts/test-postgres-migrations.sh`、`scripts/test-smoke-dev-backtest-claim.sh`、`backend/scripts/test.sh ./internal/database/...`。

## 七之後：實作備註（2026-11-19；已改好、未 commit，待 review）

**改了什麼**

* `python/backtest/strategy/base.py`：`TWCommission` 加 `percabs=True`、拿掉自己覆寫的 `getcommission()`，手續費與 `size < 0` 的交易稅只在
  `_getcommission()` 算；docstring 寫明定位（設定費率的模擬模型）與不做的規則（T-085）。
* `python/backtest/engine.py`：`TWCommission(commission=COMMISSION_RATE, tax=TAX_RATE)`。
* `python/tests/test_backtest_engine_trades.py`：`run` fixture 改成替換工廠（接受 `commission`、`tax`，回傳⛔ 不帶參數的 `FixedRate()`），
  跑完斷言 `engine` 剛好傳了一次、值是 config 的費率；檔頭說明改寫。
* `python/tests/test_tw_commission.py`（新，13 項）：直接跑 Cerebro 的基本、空單、多張委託、同一張委託多次成交（`FixedSize(4)`）、反手、自訂費率、
  零費率；走 `run_backtest` 的自訂費率、零費率、config 費率的多空兩筆（明細兩位、`total_return`／`avg_pnl` 依位數驗算）；modular 的費率傳遞與
  `_close` 多空的課稅邊。
* 共用查詢 `backend/internal/database/testdata/backtest_trades_fee_check.sql`、`TestPostgresMigrationsBacktestTradesFeeCheck`（新）；
  I-135 那支的暫存表與寫入抽成兩支共用的 `tradesProbe`（行為⛔ 不變）。
* `scripts/smoke-dev-backtest-claim.sh`：`read_rates`（container 裡 `config` 的原始費率，只收有限的非負小數）、`trades` 情境的費用核對；
  `scripts/test-smoke-dev-backtest-claim.sh` 加 10 項。
* 文件：`database-schema.md`（`commission` 的定義、新舊界線、`backtest_results` 一句）、`architecture.md`（費用模型）、`development-workflow.md`（費用核對）、
  `python/config.yaml` 的註解、`todo.md` T-085（費用細節規則）。

**跟計畫的差異**

* **費率讀兩次**：計畫寫「寫入任務之前讀，讀不到就結束碼 2」。config 是 build 時複製進 image 的，開始前那個 container 可能是舊的 image，所以：
  開始前讀一次（照計畫，讀不到 → 2、⛔ 沒有改任何東西）；換上本次的 image 之後、寫入任務之前再讀一次，**用這一次當預期值**（讀不到 → 1、照常收尾）。
* **總表驗算的界線另加 1e-9**：`avg_pnl` 剛好落在四捨五入的半位時（79.75375 存成 79.7537），差是 5e-5 加上浮點雜訊；界線仍是半個最小位數。
* Go 測試的 helper 重構（`tradesProbe`）：為了讓兩支共用查詢的測試用同一套暫存表，⛔ 沒有改 I-135 那支的案例。

**驗證**

| 項目 | 結果 |
|---|---|
| `test_tw_commission.py`＋`test_backtest_engine_trades.py` | 57 項通過（新 13＋I-135 的 44；I-135 那支經由工廠照樣通過） |
| 突變（拿掉 `percabs`、漏稅、恢復覆寫的 `getcommission()`、`engine` 傳字面值、`_getcommission()` 用字面值、`engine` 改回不帶參數） | 8／8／8／2／4／12 項失敗 |
| `scripts/test-postgres-migrations.sh` | rc 0；`TestPostgresMigrationsBacktestTradesCheck`、`TestPostgresMigrationsBacktestTradesFeeCheck` 通過 |
| `scripts/test-smoke-dev-backtest-claim.sh` | 60 項通過（加 10 項） |
| `python/scripts/test.sh -q`（完整） | rc 0：pytest 2023 passed、1 skipped；shell 測試都通過 |
| `backend/scripts/test.sh ./internal/database/...` | rc 0（PostgreSQL 的測試在這裡 skip，由上面的腳本執行） |
| dev 實跑 `SCENARIOS=trades SMOKE_STRATEGY=breakout_v1`（2330、2024-01-01～2025-12-31） | 通過：開始前與換上本次的 image 之後讀到的費率都是 0.001425／0.003；3 筆明細符合 I-135 的條件，費用也符合 config 費率；收尾後 dev 的 `backtest_jobs` 0 筆；dev stack 已 `down`（⛔ 沒有 `-v`） |

**live 上版（I-134 的 6.1 一般部署；2026-11-19，使用者在 live 執行）**

* **時段**：部署前檢查 14:17:54（唯讀），python-worker 14:22:37、python-server 14:22:38 啟動，部署後確認 14:24:37。部署前檢查到部署後確認之間沒有送回測。
* **部署前檢查**：`job_runs` 的 `running` 0 筆；`backtest_jobs` 的 `pending`／`running` 都是 0；策略與狀態的分布同 I-135 上版後。
  migration 版本 77；`deploy.sh` sha `abcf1c019b25`，`--preflight` 通過（MemAvailable 4679MiB）。三個服務 `running`、`RestartCount` 0
  （`c8bf4e9fdd65`、`034581dc5f42`、`52ed36ba11a3`，即 I-135 上版後的 image）；`Config.Env` 50 行、`a4eccbc0aa79`；backend 執行檔 `b07a2830c0fc`；
  舊 image 的 `strategy/base.py` 沒有 `percabs`（0）。
  * ⚠️ 指令的 `git log` 寫錯目錄：live 的 scripts 目錄不是 git checkout，build checkout 是 `deploy.sh` 的 `PROJECT_DIR`。
    `git pull` 的輸出顯示部署前是 `7a97ac5`。
* **D4 受影響的清單**（部署前重查）：`STRATEGY_MAP` 只有 `breakout_v1`，1 筆 `done`（2026-07-01 15:39:55 建立、15:39:56 完成），
  明細 0 筆，所以⛔ 沒有受影響的既有數字。部署後重查，結果相同。
* **回滾標籤**：三個服務打上 `pre-i136`（ID 同上）。
* **部署**：`git pull` `7a97ac5`→`03e7028`（17 個檔）；`部署完成：backend /health 正常、RestartCount 0`，`deploy rc=0`。
* **部署後確認**（14:24:37）：
  * 三個服務 `running`、`RestartCount` 0；image 都換了（backend `f3a5ae6b9c9c`、python-server `40702c240772`、python-worker `c58a5cce2c69`）。
  * backend：`Config.Env` 50 行、`a4eccbc0aa79`（不變）；執行檔 `b07a2830c0fc`（不變，Go 只改了測試檔）；`redis connected`／`scheduler started` 2 筆、
    設定錯誤 0 筆；migration 版本 77。
  * 兩個 python 服務的程式：`percabs` 2 行（註解與參數）、`def getcommission` 0、`def _getcommission` 1、`TWCommission(commission=` 1；
    容器內 config 的費率 `0.001425 0.003`。`schema ready`（⛔ 沒有等待）、各自 owner 的 `no interrupted jobs to reap`，`Traceback` 0；
    python-server `/health` `{"status":"ok"}`；worker 有 13 次 `poll #`。
  * `pending`／`running` 仍是 0；策略與狀態的分布、D4 清單與 `breakout_v1` 的明細筆數都跟部署前相同。
* **新舊界線（D4）**：commit `03e7028`，python-server image `40702c240772`、python-worker `c58a5cce2c69`，
  界線時間取 2026-11-19 14:22:38（台北），也就是兩個 python 服務中較晚啟動的時間（server 14:22:38、worker 14:22:37），
  從這一刻起兩個服務都是新程式。14:24:37 是部署後確認的時間，⛔ 不是界線。已歸檔到 `database-schema.md` 的「費用的新舊界線」。
* **上版後的實跑核對**：使用者從前端送 `breakout_v1`（2330、2024-01-01～2025-12-31），任務 `bt_20261119_062856_000`，
  14:28:56 建立、14:28:57 `done`。這是 live 上第一筆有交易的 `breakout_v1`。
  * 明細 3 筆，都是 `BUY`、1 股、時間帶 `+08`；`backtest_results` 的 `total_trades` 3、`total_return` 0.0003、`win_rate` 0.6667、`avg_pnl` 93.5087。
  * 共用查詢在 live 唯讀執行：I-135 的明細規格 0 筆不符、I-136 的費用（容器 config 的費率 0.001425／0.003）0 筆不符。
    綁定值：兩份查詢的 `$1`（job_id）和 `$2`（前綴）都是 `bt_20261119_062856_000`，費用查詢的 `$3`＝`0.001425`、`$4`＝`0.003`。
    前綴就是 job_id 本身，`starts_with` 必然成立，所以篩選條件等於 `job_id = 'bt_20261119_062856_000'`。
    同一條件下 `backtest_trades` 有 3 筆，這兩個 0 涵蓋本次的三筆明細。
  * 手算：802.42→870.81 的模擬費用 4.99678，存成 5.00；1050.37→987.45 是 5.86624，存成 5.87；1113.47→1407.21 是 7.81360，存成 7.81。
    三筆的 `pnl` 都等於出場價−進場價−`commission`。明細的平均淨損益約 93.51：用存下來的兩位價格和未四捨五入的費用重算是 93.511125，
    跟總表的 `avg_pnl` 93.5087 仍有差。總表是用未四捨五入的成交價算的，明細的成交價與 `pnl` 只存兩位，所以兩者在存檔精度內一致。
    這個差距⛔ 不能只歸因於費用的四捨五入。
* `pre-i136` 標籤保留到收斂後再清。

## 八、完成後要歸檔的位置

* `docs/database-schema.md` 的 `backtest_trades`：`commission` 改成 D2 的定義（模擬總費用＝手續費＋賣出的交易稅），拿掉「⛔ 不代表台股的實際費用」的
  警語，改成 D1 的定位（設定費率的模擬模型；沒有最低費用、取整數與商品別規則，見 T-085）；加上 D4 的新舊界線。`backtest_results` 的段落補一句界線。
* `docs/architecture.md` 的回測流程：backtrader 路徑的費用模型（比例值、賣出課稅、費率來自 config）。
* `python/config.yaml` 的費率註解：寫明是比例值、兩條回測路徑共用。
* `docs/todo.md` T-085：註記 I-136 已把費用改成比例模型，最低費用與商品別規則在 T-085 處理。
* `docs/development-workflow.md` 的 `smoke-dev-backtest-claim.sh` 那一節：`trades` 情境的費用核對（費率的來源、界線、共用查詢）。
* `docs/issue.md` I-136 改成「已實作／待 review」；review 確認後收斂，計畫書在那時才刪。

## 版本紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1 | 2026-11-19 | 初版。使用者裁決 D1(a)、D2(a)、D3、D4 與補充條件；影響的方向改成「費用偏低、淨損益高估」；實作路徑與實際扣款的測試數字在測試 image 實跑查證。 |
| v1.1 | 2026-11-19 | review [P2]×2：I-135 的 `FixedRate` 替身改成接受新建構參數的工廠、維持獨立的 1%（實跑確認 `tax` 會丟 `TypeError`、`commission` 會被蓋掉）；總表報酬⛔ 不乘回本金要求等於淨損益，broker 資產變化用未四捨五入的值核對、限定期末全部平倉，`total_return`／`avg_pnl` 依存的位數驗算。測試細節：分批分成「多張委託」與「同一張委託多次成交」（後者用 execution bit 的費用或累計差額，實跑查證）；「費率寫死」的突變改在實際傳值與扣費公式處（改預設值是等價突變）。第七節第 4 點使用者裁決要做：預期值取 config 的原始費率、共用 SQL、PostgreSQL 實測、失敗收尾。部署順序：I-135 live 驗收完成後才 push I-136。 |

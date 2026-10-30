# I-122 調查計畫 v3：2026-10-29 live 的四個批次失敗或 partial

> 狀態：✅ v3 已確認（2026-10-30，`db98b1f`）。「九」2 ✅（`37550fd`）、「九」3 ✅（2026-10-30 15:54）；「九」4 的判讀已寫回 I-122、另立 I-123～I-126，⚠️ 待 review。
> 對應：[`issue.md`](../issue.md) I-122。

## 一、目標與不做的範圍

需求（2026-10-30 使用者確認）：

* **先保全證據**，再查原因：
  * `job_runs`：10-22 到查詢當天，四個工作（`stock_symbol_sync`、`corporate_action_sync`、`delisting_reconcile`、`intraday`）的完整紀錄，以及所有工作的每日狀態摘要。
  * 容器 log：只取那幾輪的時間窗，**遮罩並驗證**之後才存檔。
  * live 的資料一律由**使用者在自己的終端機**唯讀查詢，存到 repo 外；Claude 只讀存好的檔案。
* **逐一查原因**：每個工作歸成四類之一——上游暫時性問題、資料問題、程式缺陷、照設計會出現的 partial。⚠️ partial ⛔ 不等於缺陷。
* **確認有沒有重現**：10-30 之後同樣的工作是否又失敗。
* **寫回結論**：寫進 I-122；確認是缺陷的，另立 issue 與修復計畫（D3）。

不做：

* ⛔ 不碰 live：不改設定、不重跑批次、不重啟容器、不呼叫會寫入的 API。查詢一律唯讀（`default_transaction_read_only`、`cat`、`docker inspect`）。
* ⛔ 不修程式（D3）。
* Claude ⛔ 不連 live DB、⛔ 不經手 JWT 與任何 secret、⛔ 不讀未遮罩的 log。
* ⛔ 不呼叫 `/scheduler/status`（使用者裁決：證據只取 `job_runs` 與容器 log）。

## 二、現況與讀碼（⚠️ 推論都待證據確認，⛔ 不先立案）

### live 跑的是哪一版

* live backend 容器 `stock_trading-backend-1`：2026-10-28 06:13 UTC 建立（T-075 重建容器），image `ba964c44…` 建於 2026-09-17 10:08（+08:00）。
* backend 最後一次 commit 是 2026-09-09 的 `5b1260e`，之後 backend ⛔ 沒有任何變更 → HEAD 的 backend 程式就是 live 的版本。
  ⚠️ 前提：build 當時工作樹的 backend 沒有未 commit 的修改——⛔ 無法從 image 查證，判讀時若程式與 log 對不上，先懷疑這一點。
* 設定檔：image 把 `backend/config.yaml` 複製到工作目錄 `/app`；compose 只掛載 `./logs/backend`。E0 只取它的 SHA-256 與 git 的版本比對
  （⛔ 不印內容；「三」E0），順便在設定檔這一份上驗證上面的前提。

### 每個工作怎麼得出 failed／partial，`job_runs.error` 寫什麼

`job_runs` 的欄位：`status`、`symbols_total`、`symbols_failed`、`error`、`started_at`、`finished_at`。狀態一般由數量推導：
`total > 0 且 failed ≥ total` → failed；`failed > 0` → partial；其餘 success。`error` 只寫安全字串（`<stage>:<reason>` 或計數；
I-102／T-070 已驗收⛔ 沒有原始錯誤外洩），**原始錯誤只在 log**。

| 工作 | 10-29 的狀態 | 讀碼看到的路徑 | `job_runs.error` 會是 | 只有 log 才看得到 |
|---|---|---|---|---|
| `stock_symbol_sync` | partial、10 秒（前一天 91 秒） | 同步本身回錯誤時寫 total 0、failed 1——照推導規則落到 **partial**，⚠️ 其實是整輪失敗。整輪失敗的路徑：HTTP 錯誤（含連線與逾時）、非預期 status、body 超過上限、解析錯誤、縮水防護（快照 < 現有上市 × 0.5） | `symbol_sync:<reason>`（`upstream_error`、`timeout`、`conn_refused`、`internal_error`…） | 原始錯誤（含 TWSE 的 URL）。⚠️ 單一來源的 HTTP 逾時預設 300 秒、來源間隔預設 3 秒，有效值依「環境變數 → 設定檔 → 程式預設」的順序決定（「三」E0 的推導表），E0 只記推導需要的資訊；耗時 10 秒⛔ 不能用來排除逾時（較早發生的連線逾時也可能）——**哪一步失敗是待證的假說**，以安全摘要與遮罩後的錯誤確認 |
| `delisting_reconcile` | failed、8 毫秒（前一天 10 秒） | 前置條件：當日**最新一筆** `stock_symbol_sync` 是 success 且起跑於今日；否則丟 `ErrStockSymbolSyncNotReady`、記 failed | `delisting_reconcile:internal_error`——⚠️ 那個錯誤的文字⛔ 不含任何分類關鍵字，會被壓成 internal_error，**`job_runs` 分不出「前置不成立」與其他內部錯誤** | `delisting reconcile failed` 那一行的原始錯誤（是不是 `stock_symbol_sync has not succeeded today`） |
| `intraday` | 13:30 那一輪 failed、10 秒 | 每 5 分鐘一輪；`/scheduler/status` 只看得到最後一輪。有 Yahoo 批次源時走批次路徑：整批抓取失敗計為不透明的失敗數；全部失敗 → failed | `fetch_failed:N`／`evaluate_failed:…` | `intraday batch fetch failed` 的原始錯誤（Yahoo 的回應）；當天其他輪次有沒有失敗要看 `job_runs` |
| `corporate_action_sync` | partial、35 分鐘（前一天 36 分鐘） | 逐檔同步；失敗檔數 ＋ 逾時沒輪到的檔數 > 0 → partial；讀不到 watchlist 也記 partial | 逐檔的 `stage:N (symbol:reason, …)`（原記於 I-112，已收斂）、`N 檔未處理`、`corporate_action_watchlist:<reason>` | 逐檔失敗的原始錯誤（Yahoo／FinMind） |

⚠️ 兩個可能的可觀測性缺口（**待證據確認**，確認了才另立 issue）：
① `stock_symbol_sync` 的整輪失敗被記成 partial；② `delisting_reconcile` 的「前置不成立」在 `job_runs` 是 `internal_error`。

### log 在哪、保留多久

* backend 的 app log 同時寫 stdout（docker 的 json-file，⛔ 沒有 rotation，但容器 10-28 06:13 UTC 才重建，更早的⛔ 不在）與
  `/app/logs/backend/backend-<UTC 日期>.log`（JSON，ISO8601 時間；`LOG_RETENTION_DAYS` 預設 14，只讀環境變數）。
* ⚠️ **檔名是 UTC 日期**：台北 06:30 的批次在前一天的檔裡（例如 10-29 06:30 ＝ 10-28 22:30 UTC → `backend-2026-10-28.log`）。
* ⚠️ **清理以檔名日期為準**：檔名日期早於「現在（UTC）− 保留天數」的檔，在下一次清理時被刪。保留 14 天時，含台北 10-22 06:30 的
  `backend-2026-10-21.log` 從 **2026-11-04 08:00（台北）**起就可能被清除。**期限統一為 2026-11-03（台北）結束之前完成保全**。
  live 的 `LOG_RETENTION_DAYS` 若不是 14，以 E0 記下的值重算，⛔ 不沿用 14 的推算。
* `job_runs` 保留 30 天（程式常數）：10-22 的紀錄大約 11-21 到期、10-29 的 11-28。
* ⚠️ FinMind 的 client 把 token 放在 URL 的 query（`token=`）——HTTP 錯誤的 log 很可能帶著完整的 token。**log ⛔ 不能原樣存檔或給 Claude 讀**（「三」E2）。
* backend 容器的 secret 類環境變數（只看過鍵名）：`AUTH_JWT_SECRET`、`DATABASE_DSN`、`FINMIND_API_KEY`、`FUGLE_API_KEY`、`REDIS_PASSWORD`。

## 三、證據保全（由使用者執行）

位置：`~/i122_evidence/<時間戳>/`（mode 700、⛔ 不進 repo），保存到 I-122 收斂為止（2026-10-30 改：⚠️ **保存到 I-122～I-126 全部收斂為止**，後續各筆都引用這批證據）。每一步的輸出都進這個目錄，最後產生 SHA-256 清單。
**每一步都要檢查結束碼；任何一步失敗就停下回報，⛔ 不跳過、⛔ 不改用別的寫法重試。**

### E0：環境（唯讀）

```bash
EV=~/i122_evidence/$(date +%Y%m%dT%H%M%S)
mkdir -p -m 700 "$EV"
echo "rc=$?"
B=stock_trading-backend-1
docker container inspect "$B" \
  --format '{{.Created}} {{.State.StartedAt}} {{.Image}}' \
  > "$EV/e0_backend.txt"
echo "rc=$?"
docker container inspect "$B" \
  --format '{{.Config.WorkingDir}}{{range .Mounts}} {{.Destination}}{{end}}' \
  > "$EV/e0_layout.txt"
echo "rc=$?"
F='{{range .Config.Env}}'
F="$F"'{{$k := index (split . "=") 0}}'
F="$F"'{{if or (eq $k "LOG_RETENTION_DAYS")'
F="$F"' (eq $k "STOCK_SYMBOLS_TIMEOUT_SEC")'
F="$F"' (eq $k "STOCK_SYMBOLS_FETCH_DELAY_SEC")}}'
F="$F"'{{println .}}{{end}}{{end}}'
docker container inspect "$B" --format "$F" > "$EV/e0_env.txt"
echo "rc=$?"
docker exec "$B" ls -la /app > "$EV/e0_appdir.txt"
echo "rc=$?"
docker exec "$B" sha256sum /app/config.yaml > "$EV/e0_config_sha.txt"
echo "rc=$?"
docker exec "$B" ls -l /app/logs/backend > "$EV/e0_logfiles.txt"
echo "rc=$?"
```

⚠️ 每一個 `rc` 都要是 0，任何一個非零就停下回報。
⚠️ `e0_env.txt` 的篩選在 docker 的 template 裡完成：其他環境變數的值⛔ 不會離開 docker（連管線都⛔ 不經過）；⛔ 不得改成印出整份 env。
三個鍵都⛔ 不是 secret。這段 template 已在非 live 的 image 與暫時容器上驗過（三個鍵都沒有 → rc 0、檔案只有 docker 附加的一個換行；
有其中幾個 → 只輸出那幾行；對象不存在 → rc 1）。

**「沒有這個鍵」與「inspect 失敗」分開判斷**：inspect 失敗看 `rc`；`rc` 是 0 而檔裡沒有某個鍵，只代表環境變數⛔ 沒有設定它——
**⛔ 不代表**實際用的是程式預設值（設定檔也可能設了它，見下表）。

有效值的推導（Claude 依 E0 的檔案判讀，結果與依據寫進 I-122）：

| 鍵 | 讀取來源（讀碼） | 推導 |
|---|---|---|
| `LOG_RETENTION_DAYS` | logger 直接讀環境變數，⛔ 不讀設定檔 | 有設定且是正整數 → 該值；沒設定、空值、非數字或 ≤ 0 → 14 |
| `STOCK_SYMBOLS_TIMEOUT_SEC`、`STOCK_SYMBOLS_FETCH_DELAY_SEC` | 設定載入（viper）依序取：環境變數 → 設定檔的 `stock_symbols.timeout_sec`／`fetch_delay_sec` → 預設 0；最後的值 ≤ 0 時，client 用程式預設（300 秒／3 秒） | 環境變數有設定**且非空** → 該值（⚠️ viper 預設⛔ 不採用空值的環境變數，空值視同沒設）；否則看設定檔這一層，見下 |

設定檔這一層：viper 在工作目錄（`.`）與 `./config` 底下，依副檔名的固定順序（`json`、`toml`、`yaml`、`yml`…）找第一個名為 `config.<副檔名>` 的檔。
**下列三項全部成立**，才判定設定檔的兩個值等於 git 裡的版本（兩者都是 0 → 程式預設）：

1. `e0_layout.txt` 的工作目錄是 `/app`，而且掛載點只有 `/app/logs/backend`（⛔ 沒有蓋到 `/app` 本身或 `/app/config*` 的掛載）。
2. `e0_appdir.txt` 裡以 `config.` 開頭的檔只有 `config.yaml` 一個（排在前面的 `config.json`、`config.toml` 會先被選到）。
3. `e0_config_sha.txt` 的 SHA-256 等於 git 裡 image 建置當時（2026-09-17）的 `backend/config.yaml`（這個檔最後一次變更是 2026-09-08 的 `48c7458`）。

任何一項不成立 → 記為「**ENV 未覆寫，實際值待確認**」：⛔ 不印設定檔內容、⛔ 不推測；要不要另外安全擷取那兩個值，回報後另行決定
（屬於偏離計畫，先更新本計畫書）。推不出有效逾時的時候，「四」的逾時假說只靠遮罩後 log 的錯誤型態判斷。

⚠️ 容器名稱（`stock_trading-backend-1`、`postgres-postgres-1`）與 DB 帳號沿用既有文件；任何一個指令找不到容器或登入失敗就停下回報，
⛔ 不自行改猜別的名稱。

### E1：`job_runs`（唯讀）

```bash
docker exec -i postgres-postgres-1 \
  psql -U trading_username -d trading -q -v ON_ERROR_STOP=1 \
  > "$EV/e1_job_runs.csv" <<'SQL'
SET default_transaction_read_only = on;
\copy (SELECT id, job_name, status, symbols_total, symbols_failed, to_char(started_at AT TIME ZONE 'Asia/Taipei', 'YYYY-MM-DD"T"HH24:MI:SS.US') AS started_tpe, to_char(finished_at AT TIME ZONE 'Asia/Taipei', 'YYYY-MM-DD"T"HH24:MI:SS.US') AS finished_tpe, error FROM job_runs WHERE started_at >= '2026-10-22 00:00+08' AND job_name IN ('stock_symbol_sync', 'corporate_action_sync', 'delisting_reconcile', 'intraday') ORDER BY id) TO STDOUT WITH CSV HEADER
SQL
echo "rc=$?"
docker exec -i postgres-postgres-1 \
  psql -U trading_username -d trading -q -v ON_ERROR_STOP=1 \
  > "$EV/e1_daily_summary.csv" <<'SQL'
SET default_transaction_read_only = on;
\copy (SELECT (started_at AT TIME ZONE 'Asia/Taipei')::date AS day_tpe, job_name, status, count(*) AS runs FROM job_runs WHERE started_at >= '2026-10-22 00:00+08' GROUP BY 1, 2, 3 ORDER BY 1, 2, 3) TO STDOUT WITH CSV HEADER
SQL
echo "rc=$?"
```

⚠️ `\copy` 必須寫在同一行（psql 的 meta-command ⛔ 不能跨行）。兩個 `rc` 都要是 0。`error` 是安全字串（I-102／T-070），可以原樣存。

### E2：容器 log（工具在記憶體裡完成全部檢查之後，才寫檔）

由 E1 推出時間窗：每一筆非 success 的紀錄取 `[started − 5 秒, finished ＋ 5 秒]`；另外每個工作每天取一筆 success 當對照組
（同日最接近的一筆；沒有就不取）。⚠️ 06:30 有兩個工作同時起跑，時間窗內其他工作的 log 一起保留。

```bash
python3 -I scripts/extract-job-logs.py \
  --windows "$EV/e1_job_runs.csv" \
  --out "$EV/e2_logs_masked.jsonl" --report "$EV/e2_report.txt"
echo "rc=$?"
```

工具（`scripts/extract-job-logs.py`，D1）的流程，**順序寫死**：

1. **列舉來源**：由時間窗推出要讀的 UTC 日期（含跨日的窗），逐一用 `docker exec stock_trading-backend-1 cat /app/logs/backend/backend-<日期>.log`
   讀進**記憶體**。工具自己呼叫、逐一檢查結束碼——任何一個日期的檔不存在或讀取失敗 → 中止。⛔ 不經 shell 管線（下游⛔ 不會拿到「部分成功」的內容）。
2. **過濾**：只留時間窗內的行；每一行都要能解析成 JSON、時間欄位可解析，否則中止。
3. **遮罩**（對每一個字串值遞迴套用，含巢狀欄位與 stacktrace）：
   * **L1 URL**：所有 `http(s)://…?…` 的 query 一律拿掉，換成 `?<query removed>`（token 正是在 query 裡）。
   * **L2 形狀**：`token`／`api_key`／`apikey`／`secret`／`password`／`authorization`／`bearer`／`dsn` 後面接 `=` 或 `:` 的值；
     JWT 的形狀（以 `eyJ` 開頭、以兩個 `.` 分成三段）；`postgres://`／`redis://` 這類連線字串；以及**長度 ≥ 32 的英數混合長字串**
     （已輪替的舊 token ⛔ 不在 L3 的比對範圍，這一條是它們的防線；SHA 之類的值也會被遮掉，照實計數）。全部換成 `<masked>`。
4. **序列化**：在記憶體裡組出最終的輸出 bytes。
5. **L3 實值比對**（fail-closed）：用 `docker inspect` 在**記憶體裡**取得上列 5 個 secret 環境變數**目前**的值（`DATABASE_DSN` 另外拆出密碼），
   斷言第 4 步的輸出 bytes 裡⛔ 沒有出現它們的原值、URL 編碼值、JSON 跳脫後的值。值⛔ 不印、⛔ 不寫檔；取不到任何一個值 → 中止。
6. **寫檔**：第 1～5 步全部通過，才在 `$EV` 裡以 mode 600 寫暫存檔、fsync、rename 成 `e2_logs_masked.jsonl`。
   在這之前，磁碟上⛔ 沒有任何含 log 內容的檔案（包括暫存檔）。
7. **報告**：`e2_report.txt` 只寫計數——每個來源日期的讀取結果、每個時間窗的行數、每一層的替換次數、L3 的結果。

失敗路徑：任何一步失敗都只輸出**類別與計數**（例如「第 3 個來源日期讀取失敗」「第 N 行不是 JSON」），結束碼非零、⛔ 沒有輸出檔。
工具攔下所有例外；錯誤訊息與 traceback ⛔ 不得帶任何 log 原文（例外物件的訊息可能含原文，⛔ 不能原樣印出）。

⚠️ **L3 的保證範圍**（照實）：只證明「輸出裡⛔ 沒有發現這 5 個 secret **目前**的值及其上述變體」。它涵蓋不到：
① 查詢期間（10-21 起）曾經輪替過的舊值；② 不是經由這 5 個環境變數提供的憑證（例如設定檔裡的值）；③ 其他編碼方式。
這幾類只靠 L1／L2 防守。處置：執行前由使用者確認 10-21 之後有沒有輪替過任何 secret（Claude ⛔ 不查）；有的話在 I-122 照實記錄
「L3 ⛔ 不涵蓋舊值」，⛔ 不宣稱「輸出裡真的沒有 secret」。

使用者⛔ 不需要、也⛔ 不應該打開原始 log；Claude 只讀 `e2_logs_masked.jsonl` 與 `e2_report.txt`。

### E3：封存

```bash
( cd "$EV" && sha256sum e0_* e1_* e2_* > SHA256SUMS )
chmod 600 "$EV"/*
echo "$EV"
```

執行時段：⚠️ 避開 06:30～07:10（`stock_symbol_sync`、`corporate_action_sync`、`delisting_reconcile` 正在跑）與盤中；
其餘時段都可以（全部唯讀、資料量小）。**期限：2026-11-03（台北）結束之前**（「二」）。

## 四、判讀

| 工作 | 要回答的問題 | 主要證據 | 歸類的依據 |
|---|---|---|---|
| `stock_symbol_sync` | 整輪失敗的是哪一步？上市、上櫃哪一個來源？是不是逾時？ | log 的原始錯誤（遮罩後）＋ `symbol_sync:<reason>` ＋ E0 推出的有效逾時（推不出就是「待確認」） | HTTP 非 2xx／連線錯誤／逾時 → 上游暫時性；body 或解析錯誤 → 上游格式變動或資料問題；縮水防護 → 資料問題；與程式假設不符 → 缺陷 |
| `delisting_reconcile` | 是不是前置不成立（`stock_symbol_sync` 當天不是 success）？ | log 的原始錯誤 ＋ 兩個工作的時間順序 | 前置不成立 → 照設計的連帶失敗（根因跟著 `stock_symbol_sync`）；其他錯誤 → 個別查 |
| `intraday` | 10-29 只有 13:30 那一輪失敗，還是有多輪？失敗的是抓取還是評估？ | `job_runs` 當天所有輪次 ＋ log | 13:30 收盤時點 Yahoo 回應異常 → 上游暫時性或時間邊界；連續多輪 → 擴大查 |
| `corporate_action_sync` | 是哪幾檔、哪個來源失敗？是否每天都有？ | `job_runs.error` 的逐檔明細 ＋ log ＋ 每日摘要 | 少數標的的上游失敗 → 照設計的 partial；整批或同一來源全掛 → 上游；與程式假設不符 → 缺陷 |
| 全部 | 10-30 之後有沒有重現？ | E1 的每日摘要 | 只有 10-29 → 單日事件；持續 → 提高嚴重度 |

* 每一筆結論都要附證據（`job_runs` 的 id、log 的時間與訊息），⛔ 不只靠讀碼推論下結論（I-122「待查」2、3）。
* 判讀時把「二」的兩個可觀測性缺口一併確認或否定。

## 五、產出與後續

* I-122：寫進「事實」（逐筆附 id 與時間）、「判讀」（四個工作的歸類）、「影響」；狀態依結果改成「已查明／待 review」或「已知限制」。
* 確認是程式缺陷或可觀測性缺口的：各自另立 issue（D3），新 issue 要帶**證據的 id**（`job_runs` 的 id、log 的時間與訊息、`$EV` 的 SHA-256 清單）、
  **影響**與**追蹤關係**（來源是 I-122、I-122 指向它）；⛔ 不在本計畫裡修。
* 收斂時：可長期沿用的作法（live 唯讀查 `job_runs`、遮罩之後才讀 log、UTC 檔名與保留期、工具的用法）歸檔到 `development-workflow.md`。

## 六、決定（✅ 2026-10-30 依 v1 的 review 定案）

| | 定案 | 內容 |
|---|---|---|
| D1 工具放哪裡 | **(a)** | `scripts/extract-job-logs.py`，附自己的迴歸測試（「八」）：遮罩各層的正向與反向案例、**來源讀取失敗**（某一個日期的 `docker exec` 失敗 → ⛔ 沒有輸出檔）、**驗證失敗不落盤**（L3 不過時磁碟上連暫存檔都⛔ 沒有）、**錯誤訊息不洩漏**（造一行含假 secret 的壞資料觸發失敗，斷言 stderr 與報告⛔ 不含它）。測試以 fake docker 執行，⛔ 不碰 live |
| D2 遮罩的驗證強度 | **(a)** | L1 ＋ L2 ＋ L3 都做；保證範圍照「三」E2 的說明限定；驗證完成之後才寫檔 |
| D3 查出缺陷時 | **(a)** | 調查與修復分開；新 issue 帶證據 id、影響與追蹤關係（「五」） |

## 七、風險與對策

| 風險 | 對策 |
|---|---|
| secret 外流到存檔或 Claude | 三層遮罩；全部檢查在記憶體裡完成之後才寫檔；fail-closed；失敗只印類別與計數；使用者⛔ 不貼原始 log；Claude ⛔ 不讀原始 log |
| L3 漏掉已輪替的舊值或非環境變數的憑證 | L2 的長字串遮罩；執行前確認輪替史；I-122 照實記錄保證範圍 |
| 發布不完整的證據 | 工具自己讀每一個來源並檢查結束碼；任何一個失敗 → ⛔ 沒有輸出 |
| 證據在調查期間過期 | **期限 11-03（台北）結束之前**；E1 與 E2 同一次執行 |
| E0 誤記有效設定 | 「沒有這個鍵」與「inspect 失敗」分開判斷；依「環境變數 → 設定檔 → 程式預設」推導；設定檔只比對 SHA-256、⛔ 不印內容；推不出就記「待確認」 |
| 查詢影響 live | 全部唯讀（`default_transaction_read_only`、`cat`、`docker inspect`）；資料量小；避開批次時段 |
| 讀碼推論被當成結論 | 「四」：每筆結論都要附 id 與 log；「二」的推論逐一確認或否定 |
| live 的程式與 HEAD 不同 | 「二」的前提；log 與程式對不上時先回報 |
| log 的時區混淆 | 工具以 UTC 解析 log、以 `job_runs` 的台北時間換算時間窗；報告同時列兩種時間 |

## 八、驗證

* 工具（D1 (a)）：
  * 迴歸測試（由 `python/scripts/test.sh` 的 shell 測試段呼叫；fake docker，⛔ 不碰 live）至少涵蓋：
    * L1、L2 各自的正向與反向案例；L3 抓到「遮罩漏掉的已知值」（原值、URL 編碼、JSON 跳脫三種）；
    * 來源讀取失敗（列舉出的某一天的 `docker exec` 回非零、檔案不存在）→ 非零結束、⛔ 沒有輸出檔；
    * 驗證失敗不落盤：L3 不過時，輸出目錄裡⛔ 沒有任何新檔案（含暫存檔）；
    * 錯誤訊息不洩漏：含假 secret 的壞行觸發解析失敗，stderr 與報告⛔ 不含那個假 secret；
    * 取不到 secret 的值 → 中止。
  * 反向驗證：拿掉任一層遮罩或任一個結束碼檢查，對應的測試要變紅。
* E0：每一個 `rc` 都是 0；`e0_env.txt` 只含那三個鍵（或是空的）；三個鍵的有效值依「三」E0 的推導表寫進 I-122，推不出的標「待確認」。
* E1：`e1_job_runs.csv` 有表頭、筆數 > 0；10-28、10-29 的四筆與 I-122 抄錄的 `/scheduler/status` 一致（狀態、起訖時間）。
* E2：`e2_report.txt` 的每一個來源都讀取成功、L3 通過；每個時間窗至少有一行 log（沒有的列出來，⛔ 不當成「沒有錯誤」）。
* E3：`SHA256SUMS` 與檔案相符。
* 文件：`check-doc-refs.py` 問題 0；本計畫書⛔ 不寫 `檔名:行號`。

## 九、執行順序

1. 使用者確認本計畫。
2. Claude 寫工具與測試 → review → 使用者 commit。（✅ `37550fd`；實作備註見「十二」）
3. 使用者確認 10-21 之後有沒有輪替過 secret，再執行 E0～E3（**11-03（台北）結束之前**），把 `$EV` 的路徑交給 Claude。（✅ 2026-10-30 15:54，`~/i122_evidence/20261030T155430/`；使用者確認⛔ 沒有輪替過 secret。執行時用的是把本節 E0～E3 的指令原文加上逐步結束碼檢查的腳本，放在 repo 外）
4. Claude 判讀（「四」）、寫回 I-122、另立 issue → review → 使用者 commit。（⚠️ 已寫回、待 review：I-122 與新的 I-123～I-126；使用者 2026-10-30 裁決四個問題各自立案——其中 I-123（DNS）、I-124（postgres）是基礎設施問題，「五」原本只寫程式缺陷與可觀測性缺口）
5. 收斂（另一次）。

## 十、完成後的歸檔位置

* 結論：`issue.md` I-122（之後依處置收斂；缺陷在新的 issue）。
* 作法：`development-workflow.md`（live 的唯讀查證：`job_runs`、遮罩過的 log、UTC 檔名與保留期、`scripts/extract-job-logs.py` 的用法）。

## 十一、修訂紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1 | 2026-10-30 | 初版 |
| v2 | 2026-10-30 | 依 v1 的 review 修正：#1 遮罩、序列化與 L3 全部在記憶體裡完成之後才寫暫存檔，失敗路徑與例外訊息⛔ 不帶原文（「三」E2 的 1～7）；#2 日期列舉與來源讀取由工具自己做並逐一檢查結束碼，⛔ 不經 shell 管線（同上）；#3 期限統一為 11-03（台北）結束之前（「二」「三」「七」「九」與 issue.md）；#4 刪掉「10 秒不是逾時」，改成待證的假說，E0 另記逾時與來源間隔的設定（來源間隔預設 3 秒，v1 推論的「等了來源間隔」也一併拿掉）；#5 L3 的保證範圍限定為「目前的值及指定變體」，並說明輪替過的舊值與非環境變數憑證的處置（L2 加長字串遮罩）。D1～D3 定案 (a)（「六」）；測試案例補上來源讀取失敗、驗證失敗不落盤、錯誤訊息不洩漏（「八」） |
| v3 | 2026-10-30 | 依 v2 的 review 修正 E0：#1 `grep` 在三個鍵都沒有設定時回 1、會被誤判成失敗——改由 docker 的 template 篩選鍵名，inspect 失敗看 `rc`，「沒有這個鍵」⛔ 不算失敗；#2 沒有環境變數⛔ 不代表程式預設（viper 也讀設定檔）——E0 另記工作目錄、掛載點、`/app` 的檔名與 `config.yaml` 的 SHA-256，依「環境變數 → 設定檔 → 程式預設」推導，推不出的記「ENV 未覆寫，實際值待確認」（「二」「三」E0、「四」「七」「八」） |
| v3 實作備註 | 2026-10-30 | 「九」2 的工具與測試；計畫沒有寫明的細節記在「十二」 |
| v3 實作備註（review 修正） | 2026-10-30 | 第一輪 review：P1 部分遮罩讓 secret 尾段殘留並通過 L3 → 遮罩改為原文上的範圍合併、引號內的值整段取、L3 加原文覆蓋檢查（「十二」5、6）；P2 用查詢範圍的最後一天判斷正在寫入 → 改用讀取當下的 UTC 日期，完整的末段照常保留（「十二」4） |
| 執行與判讀 | 2026-10-30 | 「九」3 執行、「九」4 判讀寫回 I-122，另立 I-123～I-126（「九」） |
| 判讀（review 修正） | 2026-10-30 | DNS 逾時（18）與 IPv6 不可達（19）分開記數、後者的原因改列待查；盤中的影響改成「沒有取得新資料、補回與遺漏尚未驗證」；I-125 補上共用寫入路徑的潛在影響；證據保存到 I-122～I-126 全部收斂（「三」）；issue.md 的編號索引 |

## 十二、實作備註（2026-10-30，「九」2；第一輪 review 的 2 項已修正，⚠️ 待 review）

工具：`scripts/extract-job-logs.py`；測試：`scripts/test-extract-job-logs.sh`（接在 `python/scripts/test.sh` 的 `SKIP_SHELL_TESTS` 段，fake docker，⛔ 不碰 live）。
下列是計畫沒有寫明、實作時定下的細節。都只會讓工具**更早中止或多遮**，⛔ 沒有放寬「三」E2 的任何一條：

1. **讀取是串流的**：每個來源邊讀邊過濾，只有時間窗內的行留在記憶體（host 只有 2 GiB，單日的 log 檔大小未知）。
   結束碼在讀完之後才知道：非零就中止、已讀到的部分⛔ 不採用。磁碟上的行為與「三」E2 相同（第 6 步之前⛔ 沒有任何檔）。
2. **對照組的選法**：每個工作、每個台北日期取一筆 success——當天有非 success 時，取起跑時間最接近當天最早一筆非 success 的；
   沒有時取當天最早的一筆；當天沒有 success 就不取。
3. **`running`（沒有結束時間）的紀錄 → 中止**，請避開批次時段重跑 E1 與 E2。`aborted` 的結束時間是 backend 重啟回收的時刻，
   窗可能很長，照實取（報告列出每個窗的起訖）。
4. **最後一段沒有換行時**：能完整解析就是完整的一行，照常處理（zap 的一行只有最後一個 `}` 在頂層）；不完整的只有
   **正在寫入的當天檔**（讀取當下的 UTC 日期）可以丟掉並計數，歷史檔 → 中止。報告分別計「完整」與「不完整而丟棄」。
   ⚠️ review 修正：初版用「查詢範圍的最後一天」判斷正在寫入，歷史檔完整的最後一行會被丟掉。
   ⚠️ 剛好跨 UTC 午夜（台北 08:00）執行時，前一天的檔可能被當成歷史檔而中止——避開台北 08:00 前後，中止了就重跑。
5. **遮罩先在原文上算出每一條規則命中的範圍，合併之後才一次換掉**（⛔ 不依序替換），L3 的覆蓋檢查才知道原文的哪些位置被遮掉（6）。
   **鍵值的值有引號**（`"`、`'`、Go `%q` 的 `\"`）時一路取到對應的收尾引號，含空白與 `&`、`,`、`;`；找不到收尾引號就取到字串結尾。
   沒有引號時取到空白或 `&`、`,`、`;` 為止，下一組鍵值從這個值的後面接著找。
   ⚠️ review 修正：初版遇到空白、`&` 就停，`api_key="prefix9&TAIL…"` 的尾段會殘留。
   **L2 的補充**：敏感鍵名的 JSON 欄位整個值遮掉（`"token":"…"` 這種結構化欄位，值本身不帶 `token=`）；鍵名帶前後綴也算
   （`secret_key`、`access_token`）；`bearer` 後面接空白；URL 裡的帳密（`://user:pw@`）；L1 與連線字串的開頭⛔ 不要求字界
   （`base_https://…` 的 `_` 是字元，要求字界就會漏掉）。
6. **L3 的補充**：
   * 鍵不存在 → 中止；值是空字串 → 沒有內容可比對，報告列為「空值、未比對」，⛔ 不中止（compose 的 `REDIS_PASSWORD` 就是 `""`）。
   * `DATABASE_DSN` 拆不出密碼 → 中止。
   * 報告寫檔之前也做 L3。
   * **原文覆蓋檢查**（review 修正）：原始 log 裡每一處已知值（含變體），都必須完整落在同一個遮罩範圍內；只蓋到一部分也中止。
     初版只比對輸出——遮罩蓋住一部分時完整值已被破壞、比對通過，殘留的尾段卻寫進了檔案。
     ⚠️ 已輪替的舊值或不在這 5 個鍵裡的憑證，沒加引號又含空白或 `&` 時，尾段仍可能殘留（L3 的保證範圍之外，見「三」E2）。
   * ⚠️ 誤判的風險：secret 的值若是常見的字（例如也出現在程式路徑裡），L3 會中止（fail-closed）；錯誤訊息只列鍵名與次數，屆時回報再決定。
7. **輸出格式**：每行一筆 `{"src": 檔名, "line": 行號, "windows": [job_runs id…], "entry": 遮罩後的 log 物件}`。
   `--out` 或 `--report` 已存在 → 在讀任何來源之前中止，⛔ 不覆寫。

驗證（2026-10-30，review 修正之後）：
* `scripts/test-extract-job-logs.sh`：81 項全過。review 的兩個重現情境都加成案例：已知值只被遮住一部分 → 中止；
  引號內含空白與分隔符的值整段遮掉；查詢範圍最後一天的歷史檔，完整的最後一行照常保留、不完整的 → 中止。
* 反向驗證：28 個突變都讓指定的案例變紅——拿掉 L1、L2 的六條與敏感鍵；引號處理、鍵值在值的內部重新起算；
  L3 的原文覆蓋／輸出／報告／URL 變體／JSON 變體／DSN 密碼；缺鍵與 DSN 的檢查；兩個結束碼檢查；「正在寫入」改回查詢範圍的最後一天、
  歷史檔也丟掉不完整的末段、完整的末段也丟掉；L1 與連線字串加回字界；L3 之前就寫檔；錯誤訊息帶原文；對照的選法。
* `python/scripts/test.sh` 全量：rc 0（pytest 1711 passed、1 skipped；shell 測試段全過，本測試在其中執行）。⚠️ 那一輪跑的是 review 修正之前的版本；
  之後只改了工具與它的測試（`python/scripts/test.sh` ⛔ 沒有再改），重跑的是本測試與反向驗證。worktree 425 → 429（I-118 的每輪 +4，⛔ 沒有清理）。

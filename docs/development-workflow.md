# Development Workflow

這份文件是本專案共同的開發與驗收工作方式。`AGENTS.md` 與 `CLAUDE.md` 都應引用這裡，
避免不同 agent 使用不同流程。

## Docker 環境區隔

開發驗收一律使用 dev 專用 compose，不要用 live/deploy 使用的 compose project。

| 用途 | Compose | Project / resources | Port |
|------|---------|---------------------|------|
| 開發驗收 | `docker-compose.dev.yml` | `stock_trading_dev`、`stock_trading_dev_net`、`stock_trading_dev_*` volumes | Backend `18080`、Python `18001`、Postgres `15432`、Redis `16379` |
| live / 部署 | `docker-compose.yml` + `deploy.sh` 或正式部署流程 | live project、live network、live volumes | 依部署設定 |

規則：

- 驗收開發成果時只跑 `docker-compose.dev.yml`。
- 不要對 live project 執行 `docker compose down -v`、migration 測試、資料清空或測試資料匯入。
- 需要重置驗收資料時，只能對 dev compose 執行 `down -v`。
- dev compose 已使用不同 host ports，可與 live 同機並存。

## 測試腳本優先

建置與測試一律走各 runtime 的腳本，不要臨時手打 `docker run`：

1. 要跑建置或測試時，先看該 runtime 的 `scripts/` 有沒有現成腳本；**有就用腳本**。
2. 現有腳本涵蓋不到當次需求時（新的檢查類型、新 runtime、需要固定的新參數），
   **先提出需求與使用者確認，把行為補進腳本，再用腳本執行**，不要用一次性指令繞過。
3. 一次性指令只用於診斷（例如進 container 看狀態），不作為驗收依據。

Dev stack smoke 也走 repo script：`scripts/smoke-dev.sh` 會啟動 isolated dev compose、
等待 backend 與 python-server health check 通過，失敗時自動印出服務狀態與核心 log。

### Race detector：`RACE=1 backend/scripts/test.sh <單一套件>`

```bash
RACE=1 backend/scripts/test.sh ./internal/scheduler/...            # 整包
RACE=1 TEST_FLAGS="-run TestFoo -count=1 -v" backend/scripts/test.sh ./internal/scheduler/...
```

**預設關閉**，因為 `-race` 需要 cgo，而預設的 alpine image 沒有 C toolchain
（會直接報 `-race requires cgo`）。開啟時腳本改用 debian 版 `golang:1.25`、放寬
`GO_MEMLIMIT`，並且**只跑 `go test`**（略過 vet／build——cgo 編譯很慢又吃記憶體，
全跑沒有額外資訊）。

**只對單一套件跑，永遠不要對 `./...` 開。** 這台 2GiB host 跑 race 是踩在極限上：
瓶頸是 `modernc.org/sqlite/lib`（`internal/store/sqlite.go` 的 blank import 把它拖進
幾乎每個套件的相依樹），race 版的它單一 compile 峰值約 **510MB**，`MEM` 被下修到 582m
就會 `compile: signal: killed`，**要 700m 才跑得完**。所以開跑前 `MemAvailable`
要有 **850MB 以上**（700m ＋ mem-guard 的 150MB 保留），實務上得先停掉 dev stack 的
python 兩支。完整實測數字與「為什麼 `GO_MEMLIMIT` 不能設 600MiB」記在
`backend/scripts/test.sh` 檔頭。

**什麼時候該用**：改到跨 goroutine 共用的狀態時。一般測試抓不到這類 bug——
2026-08-18 曾把 `Scheduler.registeredJobs` 的 map 並發讀寫（`Start()` 寫、
`/scheduler/status` 讀）做到全綠才被 race detector 抓出來，而那種 bug 的實際症狀是
`fatal error: concurrent map read and map write`，**不可 recover，整個 backend 行程會死**。

用法上要先確認 detector 對這個案例真的有效：**先在未修復的版本跑一次、確認報出
`WARNING: DATA RACE`**，再套修復重跑。少了這一步，「跑完是綠的」也可能只代表測試沒壓到
那條路徑。

### MySQL migration 驗證：`scripts/test-mysql-migrations.sh`

改到 `backend/internal/database/migrations/mysql/` 就要跑這支。dev / live 都用 postgres，
mysql 那份 migration 沒有其他執行路徑（背景見 [`issue.md`](./issue.md) I-054——
2026-08-07 首次實跑就抓到 5 個保留字語法錯誤）。

```bash
scripts/test-mysql-migrations.sh              # 起 MySQL → goose up → 驗 schema → down → 收掉
KEEP_UP=1 scripts/test-mysql-migrations.sh    # 跑完保留 MySQL（可從 127.0.0.1:13306 連進去看）
```

驗證邏輯在 `backend/internal/database/migrate_mysql_test.go` 與
`backend/internal/database/adjuster_mysql_test.go`，用的是與 `cmd/server/main.go`
相同的 `database.RunMigrations` 進入點（migration 是 `//go:embed` 打包的，
從磁碟讀檔的 goose CLI 驗的是另一份東西）。這些測試以 `MYSQL_MIGRATION_DSN` gate 住，
沒設就 skip，所以 `backend/scripts/test.sh ./...` 不受影響。目前有三支：

| 測試 | 驗什麼 |
|---|---|
| `TestMySQLMigrationsUpAndDown` | 完整 up → 驗 schema → 分段 down 到 0（回滾鏈） |
| `TestMySQLMigrationsPreserveCashDividendVolumeFactor` | `062` 的回填在 down→up 重跑時不會動到現金股利的 `volume_factor` |
| `TestMySQLMigrationsRealValuesFitAllColumns` | 欄位寬度裝得下程式碼裡真正的常數（`063`／`064`／`065` 的回歸） |

**新增測試時名稱必須以 `TestMySQLMigrations` 開頭**，否則不符合腳本的
`-test.run 'TestMySQLMigrations'` 過濾條件——測試會被靜靜跳過而輸出看起來完全正常
（postgres 側 2026-08-11 踩過一次）。驗收時要逐支確認 `--- PASS`，不能只看 exit code。

測試共用同一個 MySQL 實例並依序執行，所以每一支都要自己把狀態收回去：
`TestMySQLMigrationsUpAndDown` 要求進來時是空 DB，後面的測試各自 `up` 上去、
結束前 `down` 回 0 或清掉寫入的資料。

**歷史**：`061`～`065` 曾經連續五次改動都沒跑這支，2026-08-12 才補跑（結果全數通過，
migration 無需修正）。這正是把它寫進下方檢查清單的原因。

**兩階段設計與記憶體實測（2026-08-07）**：腳本刻意先在 MySQL 還沒起來時把測試編成執行檔，
編譯器退場後才起 MySQL，再用輕量 container 跑編好的 binary——峰值因此是
max(編譯, MySQL) 而不是 sum。調瘦後（`performance-schema=OFF`、buffer pool 64M）
MySQL 實測佔 **182MiB**（預設值會是 400MB 以上），過程中 host available 低點約 689MB。
腳本開頭會檢查 available ≥ 600MB，不足直接中止而不是硬跑。

### Postgres migration 驗證：`scripts/test-postgres-migrations.sh`

改到 `backend/internal/database/migrations/postgres/` 就要跑這支。

```bash
scripts/test-postgres-migrations.sh              # 起 postgres → up → 驗 schema → 分段 down → 收掉
KEEP_UP=1 scripts/test-postgres-migrations.sh    # 跑完保留（可從 127.0.0.1:15433 連進去看）
```

**dev stack 明明就有 postgres，為什麼還要一個**：dev 的 backend 啟動時**只跑 Up**，
Down 那一半沒有任何執行路徑——017／018 的回滾鏈斷掉就是這樣一直沒被發現的。
要驗 Down 得 down 到 0，那會清光 dev 的資料，是另一個明確動作，不該混進驗收流程。
所以需要一個**可丟棄**的實例。設計與 mysql 版相同（兩階段錯開記憶體、走 embed 的 migration
而非磁碟、專用 compose project、跑完 `down -v`），驗證邏輯在
`backend/internal/database/migrate_postgres_test.go`，以 `POSTGRES_MIGRATION_DSN` gate 住。

除了回滾鏈，它還鎖住一個**只有實跑才驗得到**的性質：migration 060 的
`ck_candles_positive_price` 在 postgres 上是 `NOT VALID`，因為加上去的當下 live 仍有
當時尚未清除的髒資料（見 `database-schema.md` 的 candles 章節）。
`TestPostgresMigrationsToleratePreexistingBadRows` 直接重現那個處境
——先寫一列髒資料再套 060，套得上去才算過。哪天有人「順手」把 `NOT VALID` 拿掉，
會在這裡失敗，而不是部署到 live 才炸。

### 要動 live 資料時的做法

CLAUDE.md 禁止拿 live 做測試資料、migration 驗證與清空資料。少數情況下仍需要修正 live 的
實際資料（例如 2026-08-10 清掉 4 根全零 K 棒），那時：

- **先取得單獨授權**，不要順手夾在其他工作裡做掉。
- **先留底**：把要動的列 dump 成可還原的 SQL。
- **用明確的主鍵，不要用條件式**。`WHERE id IN (…)` 而不是 `WHERE low <= 0`——
  條件寫錯的代價是刪掉真實資料，而主鍵清單在執行前就能逐筆核對。
- **包在交易裡並在 COMMIT 前斷言結果**（例如「違規列剩餘 0」），不成立就整筆回滾。
- **事後核對範圍**：不只看目標消失，也要確認**沒有誤刪**——比對相關標的的總列數變化量、
  以及被刪那天的前後日資料仍在。

> `docker exec` 執行 heredoc 時**一定要加 `-i`**。少了它 stdin 不會進到 container，
> psql 讀到 EOF 直接以 **exit 0** 結束——指令看起來成功，實際上一行都沒執行。
> 2026-08-10 就這樣「刪除成功」了一次，是事後查資料才發現根本沒刪。

### Redis emission 驗證：`scripts/test-redis-emission.sh`

驗**只有真實 Redis 才驗得到**的語意——目前是 signal emission reservation 的
**Lua 原子 compare-and-delete**（設計見 [`architecture.md`](./architecture.md)
「寫入失敗的一致性契約」的 reservation 那節）。

```bash
scripts/test-redis-emission.sh                            # 起拋棄式 Redis，跑完即刪
REDIS_TEST_ADDR=host:port scripts/test-redis-emission.sh  # 用既有的（自負隔離責任）
```

**為什麼不併進 `backend/scripts/test.sh`**：那支是**每次都要跑**的基本驗證，
跑在沒有網路相依的容器裡。把外部服務加進去會讓整條驗證需要 Redis 才跑得動。
所以那支整合測試**預設 `t.Skip`**，只有這個腳本會設 `REDIS_TEST_ADDR`。

⛔ **不要把 `REDIS_TEST_ADDR` 指向 live 的 `redis-redis-1` / `trading-net`。**
測試會寫入 key，指到 live 就是拿正式環境當測試資料。
預設模式起的是**拋棄式容器**：獨立 network、**不掛 volume**、跑完即刪，
與 live 和 dev 都不共用資料。

⚠️ **寫這類腳本時的兩個坑**（2026-09-02 都踩過）：

* ⛔ **不要用 `exec docker run`**——`exec` 會讓 shell 被 docker 取代，
  `trap EXIT` 永遠不執行，拋棄式資源就殘留（那次留了一個跑 17 分鐘的容器）。
  改成正常執行 ＋ 保留 exit code。
* ⛔ **資源名稱不要固定**——固定名稱時兩個人同時跑，後啟動者的 `docker rm -f`
  會刪掉前一輪正在用的容器。加 PID／時間戳後綴，並且**只清本輪確實建立的資源**
  （network 與 container 各記一個旗標，否則 Redis 起不來時 network 會漏刪）。

### 用真實資料跑 evaluation：`scripts/run-evaluation.sh`

SR Zone 的 evaluation / decision replay / sweep 要拿**真實的數千根日 K** 才有意義，
單元測試的合成資料取代不了。這支腳本封裝了那條路徑：

```bash
scripts/run-evaluation.sh --symbols 2330,2454              # evaluation
MODE=replay scripts/run-evaluation.sh --symbols 2330,2454  # decision replay（daily confirmation 在這裡）
MODE=sweep  scripts/run-evaluation.sh --symbols 2330,2454  # ATR builder 參數 sweep
OUTPUT=/tmp/r.json MODE=replay scripts/run-evaluation.sh --symbols …  # 結果落檔
```

**預設唯讀、不寫 DB**。資料來源是 live（dev project 沒有這些歷史資料），要寫入必須明確
`WRITE_DB=1` 並會看到警告——CLAUDE.md 規定驗收不得動 live 資料，所以把「不寫」設成預設。
DSN 從 live 的 python-server container 讀，不寫進 repo（密碼不進版控，live 改密碼也不用改腳本）。

**先做小規模預檢再投入完整規模**（2026-08-07 的教訓）：
這條路徑會跑很久，而序列化失敗發生在**最後一步**——曾經整輪跑完才因為一個 `date` 物件
炸掉，前面的運算全部白費。改動後先用 `--symbols 2檔 --limit 400 --replay-max-rows 20`
跑一次（約兩分鐘），確認真實 DB 的型別都過得了 `json.dumps`，再跑完整規模。

**資源特性**（2026-08-07 實測，11 檔 × `replay_max_rows=5000`）：記憶體約 **157MiB**，
受限的是 CPU 與 `replay_max_rows` 而非記憶體。**執行時間請實際量測，不要用本機的等待感推估**
——這個沙箱的 `sleep` 不等比推進 wallclock，且腳本用 `--rm`，容器結束後拿不到
`FinishedAt`。要量時間得自行在呼叫端記錄起訖。

### 在 dev stack 上做「as-of 階梯」驗收

要驗的東西**跨越多個交易日**（zone 身分延續、角色翻轉、缺席收攤、事件鏈）時，
單次分析驗不出來，而 dev project 沒有歷史資料。作法是把 live 的日 K 唯讀複製到
dev 的 staging 表，再**依交易日逐階釋出**到 `candles`，每釋出一階打一次分析。
2026-08-19 的 T-048 階段 B 驗收就是這樣做的。

**一、從 live 唯讀取資料**（live 憑證從 container 的 `POSTGRES_USER` 讀，不寫進 repo）：

```bash
docker exec postgres-postgres-1 psql -U trading_username -d trading \
  -c "\copy (select symbol,timeframe,open,high,low,close,volume,amount,ts,adj_factor,vol_factor \
       from candles where symbol='0050' and timeframe='1d' and ts >= '2024-06-01' order by ts) \
      to stdout with csv" > /tmp/0050_1d.csv
```

**二、進 dev 的 staging 表**（不要直接進 `candles`，否則沒得分階段）：

```bash
docker exec stock_trading_dev-postgres-1 psql -U trading -d trading \
  -c "drop table if exists stg_candles; create table stg_candles (like candles including defaults);"
docker exec -i stock_trading_dev-postgres-1 psql -U trading -d trading \
  -c "\copy stg_candles(symbol,timeframe,open,high,low,close,volume,amount,ts,adj_factor,vol_factor) from stdin with csv" \
  < /tmp/0050_1d.csv
```

**三、每一階：釋出 → 分析 → 快照**：

```sql
insert into candles (symbol,timeframe,open,high,low,close,volume,amount,ts,adj_factor,vol_factor)
select symbol,timeframe,open,high,low,close,volume,amount,ts,adj_factor,vol_factor from stg_candles
where (ts at time zone 'Asia/Taipei')::date <= date '2026-07-21'
on conflict (symbol,timeframe,ts) do nothing;
```

```bash
curl -sS -X POST http://localhost:18080/api/v1/sr-zones \
  -H "Authorization: Bearer $JWT" -H 'Content-Type: application/json' \
  -d '{"symbol":"0050","timeframe":"1d","limit":400,"reuse_existing":false}'
```

#### 四個會讓這件事做錯的細節

**交易日是台北日期，不是 `ts::date`。** `candles.ts` 存的是 `16:00Z`＝台北隔日
`00:00`，所以 `2026-08-17 16:00:00+00` 這根的交易日是 **08-18**。切階梯一律用
`(ts at time zone 'Asia/Taipei')::date`，用 `ts::date` 會整批差一天。

**`SR_SCORING_MODELS_DIR` 沒帶就分析不了。** repo 的 `python/models/` 是**空的**，
compose 預設會把它掛進去，於是 `POST /sr-zones` 回 **503「機率模型尚未訓練」**——
看起來像模型壞了，其實是掛錯目錄。啟動 dev stack 時要指到有 joblib 的目錄：

```bash
SR_SCORING_MODELS_DIR=/opt/stacks/scripts/stock_trading/python/models \
  docker compose -f docker-compose.dev.yml up -d
```

**dev 帳號註冊完是 `inactive`，沒有 first-user bootstrap。** `POST /auth/register`
之後要直接改 DB 才登得進去：

```bash
docker exec stock_trading_dev-postgres-1 psql -U trading -d trading \
  -c "update users set status='active' where email='dev@example.com';"
```

**不要為了這件事重建 image。** `scripts/smoke-dev.sh` 會先 `down` 再 build，
Go compile 峰值 RSS 約 420 MiB，而這台 host 只有 2 GiB。先確認跑著的 image 已經是
現行程式碼再決定——backend 比對 image 建置時間與 commit 時間，python-server 直接比 md5：

```bash
docker image inspect stock_trading_dev-backend -f '{{.Created}}'
docker exec stock_trading_dev-python-server-1 md5sum /app/http_server.py; md5sum python/http_server.py
```

**寫入失敗是靜默的就要主動查 log。** zone identity 那條路徑寫入失敗只記 warn、
API 照樣回 201，所以每階跑完要看一次：

```bash
docker logs stock_trading_dev-backend-1 2>&1 | grep -i "zone identity"
```

#### as-of 階梯驗收的六條門檻

每一階跑完看 backend log 的 `event identity: zone association` 那筆結構化 log
（它把整次分析的關聯決策拆成欄位），七階跑完再對 DB 跑一次不變式 SQL。
2026-08-19 的 T-048 階段 C 復驗就是照這六條判「過或不過」：

| 門檻 | 來源 | 合格值 |
|---|---|---|
| 事件關聯不到 zone 身分 | log `unmatched_zone_keys` | **每一階都是空的**。非空代表還有沒找出來的成因，不能當已知限制帶過 |
| 終態快照矛盾 | log `invariant_violations` | **恆空**（非空是 `Error` 級別） |
| `carried_from_previous` 讀不到 | log `carried_parse_failed` | **0**。非 0 代表 Python 沒把旗標寫進 `state_json` |
| 終態重生 | `event_instances` 筆數不隨分析次數單調成長 | 同一條鏈不該每次分析多一筆 |
| `from_state` 留白 | SQL（見下） | 誕生以外 0 筆 |
| ZONE scope 事件掛空 zone | SQL（見下） | 0 筆 |

```sql
-- 終態快照矛盾（ended_at 有值但 state / active 沒跟上）
select count(*) from event_instances
 where ended_at is not null and (state not in ('RESOLVED','EXPIRED') or active);
-- transition 說終態、快照卻沒終態
select count(*) from event_instances e
 where exists (select 1 from event_transitions t
                where t.event_uid = e.event_uid and t.to_state in ('RESOLVED','EXPIRED'))
   and e.ended_at is null;
-- from_state 留白（誕生除外）
select count(*) from event_transitions t
 where from_state is null
   and t.id <> (select min(id) from event_transitions x where x.event_uid = t.event_uid);
-- ZONE scope 事件掛空 zone / 指向不存在的身分 / alias 超過每 zone 8 筆上限
select count(*) from event_instances where last_zone_key <> 'SYMBOL' and zone_uid is null;
select count(*) from event_instances e where zone_uid is not null
   and not exists (select 1 from zone_instances z where z.zone_uid = e.zone_uid);
select count(*) from (select zone_uid from zone_key_aliases
                       group by zone_uid having count(*) > 8) s;
```

**`carried_noop` 不是門檻。** 它記的是「carried 事件找不到活鏈，依定案不開新
occurrence」，而 Python 會把**終態**的事件狀態每次分析都重報一次——所以每一條走完
生命週期的鏈，此後每一階都會貢獻一筆，計數只會單調累積（0050 七階累積到 5 筆）。
判讀方式是逐筆去 `event_instances` 對：**對應的鏈應該都已經 `ended_at IS NOT NULL`**；
若有一筆的鏈還活著，那才是問題（護欄擋掉了不該擋的東西）。

#### 重跑階梯前要先把 DB 退乾淨

migration 已經套用過的版本，goose **不會**因為檔案內容改了就重跑。階段 C 復驗時
`zone_key_aliases` 是後來才加進 068 的，dev DB 停在舊版 068，於是 alias 修法整段驗不到。
退版要連 goose 紀錄一起處理，且**全部放在同一個交易**裡——中途出錯整批回滾，
不會留下一半的 schema：

```sql
BEGIN;
DROP TABLE IF EXISTS zone_key_aliases;      -- 068 建的表，順序照外鍵反向
DROP TABLE IF EXISTS event_transitions;
DROP TABLE IF EXISTS event_instances;
DELETE FROM goose_db_version WHERE version_id = 68;
COMMIT;
```

清資料時 `TRUNCATE` 的**所有互相參照的表要寫在同一句**，否則 postgres 直接報
`cannot truncate a table referenced in a foreign key constraint`。`stock_sr_zone_analyses`
的參照方比想像的多（`stock_sr_decisions`、`stock_sr_daily_candidates`、
`stock_sr_model_governance`…），先查一次再寫：

```sql
select conrelid::regclass || ' -> ' || confrelid::regclass from pg_constraint
 where contype = 'f' and confrelid::regclass::text = 'stock_sr_zone_analyses';
```

**`docker exec` 要帶 `-i`**，否則 heredoc 進不了 container，psql 讀到空的 stdin
直接結束——**沒有任何錯誤訊息**，看起來就像指令跑完了但資料一點都沒變。

#### 要驗血緣（SPLIT / MERGE / RESHAPE）就得用每日階距

階距決定驗得到什麼。0050 的**週距**七階跑出 45 個 zone 身分、**血緣邊 0 條**——
隔一週 zone 早就漂到不重疊，直接走「缺席→失格」，2→2 的元件根本組不起來，
於是所有依賴 zone 終止的路徑（事件鏈的 `ZONE_IDENTITY_ENDED` 收攤）一次都驗不到。

換成**每日**階距（2026-07-21～08-18，21 個交易日）並挑會 churn 的標的
（近 60 根日均振幅 7%＋、量能 20M＋），四檔跑出 57 條血緣邊。挑標的的 SQL：

```sql
with pool as (select symbol from candles where timeframe='1d' and ts>='2024-06-01'
               group by symbol having count(*)>=500),
     recent as (select c.symbol, c.high, c.low, c.close, c.volume,
                       row_number() over (partition by c.symbol order by c.ts desc) rn
                  from candles c join pool p on p.symbol=c.symbol where c.timeframe='1d')
select symbol, round(avg((high-low)/nullif(close,0))*100, 2) range_pct
  from recent where rn<=60 group by symbol having avg(volume) > 3000000
 order by range_pct desc limit 15;
```

**不是每檔都會 churn**：同一組跑法裡 `3105` 的 71 個身分全部 ACTIVE、0 條血緣邊。
所以要驗血緣路徑就一次多跑幾檔，不要押單一標的。另外 zone 終止**不等於**
事件收攤路徑被執行到——那個 zone 身上還要有活著的事件鏈，四檔 57 條血緣邊最後只換到
4 次 `ZONE_IDENTITY_ENDED`。

#### 一天不要重複打同一階

⚠️ **這一節描述的是 2026-08-20 之前的行為，現在已經修掉了。**
當時 `age_bars` 的單位是**分析次數**，所以同一階重打一次，active 的事件就多老一根：
復驗時重跑第七階，兩條 `CONFIRMED` 的 `SUPPORT_RECLAIM` 在 candles 一根都沒變的
情況下同時轉 `EXPIRED`。

**現況是「K 棒推進」才老化**，同一根 K 棒重複分析不再讓事件提早 `EXPIRED`，
所以重打同一階已經不會污染老化計數。現況規格見
[`sr-zone-scoring.md`](./sr-zone-scoring.md)「老化的單位是『K 棒推進』」
（原記於 `issue.md` I-077，已收斂）。

判讀**2026-08-20 之前**留下的階梯結果時，仍要把舊行為算進去。

### 在 dev stack 上驗排程類功能

要驗的是**排程本身會不會正確偵測／告警**（而不是某段演算法）時，dev 的預設狀態幾乎一定
不夠用——`docker-compose.dev.yml` 用**獨立的資料卷**，與 live 完全不共用。
**沒有資料時排程會早退或算出空的候選集合，而那看起來像「一切正常」。**
2026-08-28 的 `candle_gap_detection` 驗收（原記於 `todo.md` T-067，已收斂）就是照這套做的。

#### 開始前的四項前置，缺一項就驗不到東西

| 檢查 | 查法 | 不足時 |
|---|---|---|
| 池／清單裡有成員 | `SELECT COUNT(*) FROM evaluation_universe WHERE active;` | 從前端「評估標的池 → ③ 已入池」匯入 selection report。**空池時排程記 `success` / `total=0`，什麼都驗不到** |
| 主檔有那幾檔且 `is_listed` | `SELECT symbol, is_listed, market FROM stock_symbols WHERE symbol IN (…);` | 跑一次 `stock_symbol_sync`（前端排程頁可手動觸發）。**查無主檔的標的會 fail-open 保留但沒有 `market`** |
| 視窗內有足夠資料 | `SELECT symbol, COUNT(*) FROM candles WHERE timeframe='1d' AND ts >= now() - interval '20 days' GROUP BY 1;` | 手動觸發回補（FinMind 5 req/min，135 檔約 26 分鐘；**只驗少數幾檔就先把池縮小**） |
| **dev image 夠不夠新** | `docker exec <dev-pg> psql -tAc "SELECT max(version_id) FROM goose_db_version;"` 對照 `backend/internal/database/migrations/postgres/` 的最大編號 | **重建 image**，見下方「schema migration 上 live 的程序」的同一條 |

⛔ **第四項的失敗模式最隱蔽，而且三個地方都不會報。** migration 是 embed 進 binary 的，
所以 image 的新舊決定 schema 的新舊。2026-08-28 那次 dev image 建於 08-25，
而要驗的表是 08-28 才加的 migration，於是**整張表不存在**，但：

* backend 照樣記 `migrations applied version=<舊的最大值>`——對舊 binary 而言那確實是最新；
* 排程自身的依賴檢查照樣通過（repo 物件非 nil，nil 檢查看不出表在不在）；
* 其他三項前置沒有任何一項會碰到它。

執行時會明確降級（讀取炸在 `relation does not exist` → 該輪收 `partial`），
所以**危險不在「結果看起來正常」，而在啟動與前置檢查都不告訴你原因**——
你會拿到一個 `partial`，然後從前三項前置查不出所以然。

#### 巢狀開關：cron 路徑要兩個都開，手動觸發則不一定

子排程若掛在 parent 排程底下（例如 `candle_gap_detection` 掛在
`evaluation_universe_sync` 尾端），**兩條路徑的生效條件不同，不要混為一談**：

| 路徑 | 生效條件 |
|---|---|
| **自動 cron** | **兩個開關都要開**——子排程的註冊寫在 parent 的 `if parent != nil && parentCfg.Enabled` 區塊裡，parent 沒啟用時子排程根本不會被註冊 |
| **手動觸發 parent** | **會繞過 parent 的 `Enabled`**——parent 的執行函式通常只檢查依賴有沒有注入（`!= nil`），不重新檢查 `Enabled`；尾端照樣呼叫子排程。子排程是否執行**只看自身開關與自身依賴** |

**所以「parent 關、子排程開、依賴齊」時，手動觸發 parent 仍會執行子排程並寫入
`job_runs`。** `candle_gap_detection` 的實際條件與行號見
[`architecture.md`](./architecture.md)「日 K 缺漏偵測」。

⚠️ **`disabled` 沒有啟動錯誤可查**：「已啟用但依賴不齊，不註冊」那類訊息**只在
parent 已註冊、子排程 enabled、但依賴缺一時**才出現——只開子排程開關的話，
照那條訊息排錯會一無所獲。

⚠️ **「不執行、不寫 `job_runs`、沒有痕跡」只成立於兩種情形**：完全沒有手動觸發的自動排程
情境，或**子排程自身未啟用／依賴不足**（此時多半在函式第一行就早退）。照步驟跑卻什麼都
沒發生時，先分清楚自己落在哪一種——否則很容易誤讀成「沒有問題」。

所以：兩個開關都設 `true` → **重建容器**（環境變數只在容器建立時帶入）→ 確認
`/scheduler/status` 該項**不是 `disabled`**。
⚠️ **不要要求它是 `never_run`**——dev 若已有歷史執行紀錄，狀態會是上一輪的結果。
💡 **想在 parent 關閉的情況下只驗子排程**，可以利用上表第二列：開子排程的開關、
把依賴補齊，然後手動觸發 parent。

#### dev 的資料是具名 volume，不會自己還原

改動 dev 的持久化資料（刪 K 棒、寫 state、改池成員）前**先備份，驗完一定要還原**。
三份缺一不可：

1. **要刪掉的那幾列本身**——整列存下來，最保險。
2. **測試涉及標的的既有 state**——**每一檔都要**，包含只當負向案例的那檔；
   它在驗收過程中同樣會被寫入，漏了就還原不回去。
3. **清單／池的完整快照**（`SELECT symbol, active … ORDER BY symbol`）。
   ⛔ **不要只記 `active` 的那些**——測試期間可能「啟用」原本 inactive 的成員，
   只記 active 清單的話還原時不會把它設回 `false`；新匯入的 row 也認不出來
   （備份裡沒有 ＝ 測試期間新增的）。

⛔ **日期比對一律用 `(ts AT TIME ZONE 'Asia/Taipei')::date`**，備份、刪除、還原三處
**用同一個 predicate**，日界線才不會各自為政。理由與 `ts::date` 會整批差一天的說明見上方
「在 dev stack 上做『as-of 階梯』驗收」的同一條。

⛔ **收尾檢查要逐項比對備份，不要用「再跑一次應該是 `success`」當判準**——dev 本來就可能
有其他真實的缺口或不完整資料，那個判準會同時掩蓋「沒還原乾淨」與「本來就有問題」。

#### dev 沒有 FinMind 金鑰，吃 FinMind 的東西在 dev 驗不了

`deploy.sh` 進版控的是佔位值，真金鑰在 `/opt/stacks/scripts/stock_trading/`，
**dev 沒有**——所以任何走 FinMind 回補的步驟在 dev 一定失敗。
2026-08-28 的作法是**把歷史 K 棒從 live postgres 唯讀複製過來**，而不是回補。

這不影響驗收效力**只要受測邏輯不經過 FinMind**（那次受測的偵測讀的是「DB 實際日期集合 ＋
TWSE 年度日曆 ＋ 交易所逐檔核對」，三者都不經過它）。代價是還原不能走「自然路徑 upsert
補回」，要改用備份手動處理。

⚠️ **下次要在 dev 驗任何吃 FinMind 的東西之前，先確認金鑰。**

⚠️ **這台 host 只有 2GiB**：起 dev stack 前應先停掉 live stack，見下方
「`MEM` 是上限，不是預留」與「container 上限的**總和**也要顧」。

### 字串欄位的寬度：不要訂「剛好夠用」

2026-08-11 同一天內因為這件事失敗了**三次**，全部是同一個型態（SQLSTATE 22001）：

| migration | 欄位 | 原寬度 | 裝不下的值 |
|---|---|---|---|
| 063 | `job_runs.job_name` | 20 | `corporate_action_sync`（21） |
| 064 | `corporate_actions.action_type` | 16 | `CAPITAL_REDUCTION`（17） |
| 065 | `corporate_actions.source` | 32 | `TaiwanStockCapitalReductionReferencePrice`（**41**） |

**兩個教訓：**

**一、寬度要給得寬鬆，特別是值由外部決定時。** dataset 名稱、job 名稱這類字串，
訂一個「剛好夠用」的長度只是在等下一次失敗。`source` 最後取 255。

**二、逐欄測試抓不到下一個欄位。** 064 當時加了回歸測試，但它只把 `action_type`
換成各種常數，**其餘欄位（包含 `source`）寫死成安全的短字串**——所以 065 這次它照樣放行。

正確做法是**用真實的寫入路徑，讓所有欄位同時吃到正式值**：

- 常數集中在 store 層並匯出（`AllCorporateActionTypes()` / `AllCorporateActionSources()` /
  `handler.KnownSchedulerJobs()`），測試與正式程式共用同一份定義。
- 測試走 **repo 的 Upsert**（不是手拼 INSERT），並**遍歷所有組合**。
  見 `TestPostgresMigrationsRealValuesFitAllColumns` 與
  `TestMySQLMigrationsRealValuesFitAllColumns`——**欄寬限制在兩個 engine 上都存在，
  兩份都要有**（mysql 那份 2026-08-12 才補上）。sqlite 不需要：該引擎是 TEXT，沒有長度上限，
  所以 `063`～`065` 也刻意沒有 sqlite 版本的 migration。

**這類失敗特別容易被忽略**：`startRun` 與 `SyncPerSymbolEvents` 的寫入失敗都只記 log
不中斷流程，所以 job 照跑、只是資料沒進去——除非有人去翻 log，否則不會發現。

### schema migration 上 live 的程序

**入口只有一個**：`/opt/stacks/scripts/stock_trading/deploy.sh`。
它做的是 `git pull origin init` → `docker compose … down` → `up --build -d`，
**開關與金鑰由該腳本的 `export` 提供**。所以：

* 變更要先**推上 `init` 分支**，deploy.sh 才拉得到。
* 它本身就是 `up --build`，**不要另外下 `build` 或 `up -d`**。
* 它會先 `down` 再起，**有停機**。
* ⛔ **不要用 repo 的 `docker-compose.yml` 操作 live**——理由見下方
  「停 live 之前先確認它是怎麼起來的」，那是會靜默關掉生產排程的操作。

**migration 是 embed 進 binary 的**，所以「schema 有沒有更新」等於「image 有沒有重建」。
`docker compose up -d` 只重建容器不重建 image（原記於 `todo.md` T-067，已收斂）。

#### 挑窗口：先確認哪些排程會寫到你要改的表

`ALTER TABLE` 要 `ACCESS EXCLUSIVE`，撞到寫入就會互相阻塞。動手前先對照排程表
（現況見 [`architecture.md`](./architecture.md)），例如 `indicator_snapshots` / `signals`
的寫入者是：08:50 `pre_market`、09:00–13:55 每 5 分的 `intraday`、**15:00 `daily_close`**
（它逐檔跑 `signalEng.Evaluate`，而 `Evaluate` 第一行就是 `indicator.Compute`——**兩張表都寫**）、
16:00 池同步。→ 安全窗口是 **13:56–14:59**，或確認 `daily_close` 已跑完之後到 16:00 之前。

⛔ **偏離窗口之後，不要用「`job_runs` 全部 `success`」證明沒事。**
2026-09-01 的 migration 075 就是在 16:49 部署（窗口之外）；時序上確實避開了
`intraday` 與 `daily_close`，當下也沒看到排程層級異常——**但那不等於沒造成問題**。
同一天有 66 輪 `intraday` 回報 `success 11/11 0 failed`，而 `2454` 的指標從 11:24 起
根本沒再落盤（**那個行為已於 2026-09-02 修正**，現況見
[`architecture.md`](./architecture.md)「寫入失敗的一致性契約」；原記於 `issue.md` I-102，已收斂）。

**`job_runs` 的 `success` 只能當輔助資訊，不能單獨排除被吞掉的寫入錯誤。**

那要拿什麼判斷？**分兩種強度，不要混用**：

| 檢查 | 能證明什麼 | ⛔ 不能證明什麼 |
|---|---|---|
| 受影響表與其資料來源的**最新 `ts` 對齊**（例如 `indicator_snapshots` vs `candles`） | **目前已恢復、沒有持續落後** | **部署期間沒有缺列**——中段漏掉幾筆之後只要成功寫入一筆，最新 `ts` 就會重新對齊，缺口卻永久留著 |
| 依**各表自己的寫入契約**，檢查風險時間窗內的**預期寫入集合／筆數／事件** | 該窗內實際有沒有缺 | — |

⛔ **不同表不能套同一套比法**，因為它們與 K 棒**不是一對一**。同一檔、同一段
（`2454`，2026-09-02 09:05–10:00）的實測：

| 表 | 筆數 | 寫入契約 |
|---|---:|---|
| `candles`（1m） | **56** | 有成交就有一根 |
| `indicator_snapshots` | **12**（本次實測，非固定值） | 每輪 `intraday` 只針對**當時最新那根 candle** 嘗試 upsert（`ON CONFLICT(symbol, timeframe, ts) DO UPDATE`）。**相同 `ts` 會更新既有列而不是新增**——資料源沒推進時該輪不產生新列，寫入失敗時也不會有新列。**不保證每輪一列，更不是每根 K 棒一列** |
| `signals` | **5** | **只有訊號成立才寫**（還要過 15 分鐘 cooldown 去重），與 K 棒數量無關 |

⚠️ **右欄是寫入契約，中欄只是這一次的觀測值。** 12 與 5 都不是可以拿來當門檻的常數——
換一段時間、換一檔、或資料源推進節奏不同，數字就不一樣。**要算預期集合就照右欄的契約算，
不要照抄筆數。**

拿 56 去對 12 或 5 會得到「大量缺列」的假結論；反過來只看最新 `ts` 對齊則會漏掉中段缺口。

**證據不足時就寫「未觀察到異常」，不要寫成「已反證沒有影響」。**
**一次「偏離窗口而沒出事」也不構成安全的證明**——照窗口做。

**`lock_timeout` 要寫在 migration 裡，不能用外部 psql 設**：migration 是 backend 啟動時
由 goose 用它自己的連線跑的（`database/migrate.go` 的 `goose.UpContext`），
另一個 session 的 `SET` 對它無效。postgres 用 `SET LOCAL lock_timeout = '5s';`
放在 Up／Down 的第一行——作用域是 goose 包住該支 migration 的交易，結束即還原。

#### 套用後驗收：分「立即可驗」與「要等資料」兩類

立即可驗（migration 在 backend 啟動時就跑完）：

```sql
-- goose 版本。WHERE is_applied 不能省——goose_db_version 會留下已回捲的紀錄
SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version WHERE is_applied;

-- 型別真的變了（改欄位型別時）
SELECT table_name, column_name, numeric_precision, numeric_scale
FROM information_schema.columns WHERE table_name = '…' AND column_name = '…';
```

要等資料的（例如「某標的的指標重新落地」）**不會在部署後立刻成立**——
盤中才有 1m 寫入，部署後直接查會看到舊值，**那不代表沒修好**。
要當下就有結果就主動觸發（例如 `POST /api/v1/indicators/:symbol/compute`）。

**現況（2026-09-02 起）**：`Upsert` 失敗時 `Compute` 會**回傳錯誤且不寫 Redis**，
該端點對持久化失敗回 **503**（資料不足回 422、其他錯誤回 500）。
契約見 [`api-reference.md`](./api-reference.md)，語意見
[`architecture.md`](./architecture.md)「寫入失敗的一致性契約」。

⚠️ **判定仍然要看 DB**——但理由變了：**不是因為 API 會誤回成功**（那個行為已修掉），
而是因為**要的是資料真的落地的直接證據**。API 回 200 只證明「這一次呼叫成功」，
證明不了排程路徑在跑、也證明不了後續每一輪都寫得進去。

⛔ **在 2026-09-02 之前的舊版**，`Upsert` 失敗只記 warn 就繼續、照樣回 200——
翻更早的驗收紀錄時要記得那時的 200 不代表落盤（原記於 `issue.md` I-102，已收斂）。

#### 回滾：live 沒有執行 goose Down 的入口

`RunMigrations` 只呼叫 `goose.UpContext`，整個 repo 裡 `DownToContext` **只出現在測試**。
`compose.yml` 也只有 `build:`、沒有 `image:` tag，所以**沒有「回舊 tag」這個動作**。

* **純放寬型別的 migration（加寬欄位、加可為 NULL 的欄位）不需要回滾 schema**：
  舊 binary 對更寬的欄位完全相容。要退版就 **revert 程式碼並重跑 deploy.sh，schema 留著**。
  ⚠️ **不要為了「型別看起來太寬」去縮回去**——那才是製造風險的動作。
* 真的要縮回去時，**預設是中止而不是繼續**：先查有沒有超界的列，非零就停；
  要刪必須先匯出留底並取得人工確認，且要分清楚哪些表是可重算的衍生資料、
  哪些是**刪掉就永久消失的事件紀錄**。縮型別本身只能手動下 DDL。

### migration 的註解裡不要出現 goose 的 annotation 標記

goose 逐行掃描註解找它的 annotation 標記（加號接 `goose`），而且是看
**整行有沒有那個字串**，不是只看行首。所以連「本檔不可加 ⟨某個 annotation⟩」
這種**說明性文字**也會被當成真的 annotation，讓整支 migration 解析失敗：

```
failed to parse annotation line "-- 因此本檔**不可**加 …": not supported: invalid annotation
```

**要在註解裡提到某個 annotation 時，用文字描述而不要寫出標記字串**
（例如寫「goose 那個 NO TRANSACTION 的 annotation」）。
用反引號或引號包起來**沒有用**，goose 不管上下文。

2026-09-01 寫 `075_widen_indicator_numeric.sql` 時連踩兩次（原記於 `issue.md` I-101）。
⚠️ **`go vet` / `go build` 攔不到**——它是 goose 解析檔案時才報的。
**每份 migration 只會被它自己那個 engine 的測試載入**：
sqlite 的由 `backend/scripts/test.sh` 每次都跑，postgres 的只有
`scripts/test-postgres-migrations.sh`、mysql 的只有 `scripts/test-mysql-migrations.sh`。
所以**只改了 postgres 那一份時，常態的 `backend/scripts/test.sh` 是綠的**——
2026-09-01 就是這樣，一直到跑 postgres 腳本才看到。

### migration 的 Down 區塊也要能跑

**寫破壞性 migration（`DROP TABLE` 再 `CREATE`）時，Down 必須把前一版結構重建回來**，
不能只有 `DROP TABLE IF EXISTS`。否則往回滾越過它之後，更早那筆的 `ALTER TABLE` 會
找不到表，整條回滾鏈中斷——017／018 就這樣壞了一段時間，直到 2026-08-07 才補上真正的逆操作。
資料本來就救不回來（Up 已經刪了），Down 只需還原結構。

回滾鏈由三支測試把關（三種 engine 各一份 migration，各自的 Down 都要驗）：

- `backend/internal/database/migrate_sqlite_test.go`：**每次 `backend/scripts/test.sh`
  都會跑**（sqlite 用暫存檔，不需要 container）。這是常態的回歸保護。
- `backend/internal/database/migrate_mysql_test.go`：需要 `scripts/test-mysql-migrations.sh`。
- `backend/internal/database/migrate_postgres_test.go`：需要 `scripts/test-postgres-migrations.sh`。

**三支都採分段回滾，不是直接 down 到 0——這一點是刻意的。** 一路滾到底「沒報錯」
驗不到破壞性 migration 的 Down 重建得**對不對**：以 017／018 為例，017 的 Down 一執行
就把 018 重建的表砍掉了，所以 018 的 Down 內容只要不出錯就會過，寫錯也沒人知道。
正確做法是**越過每一筆之後立刻檢查表的形狀**：

```
down to 17  → 斷言 stock_sr_zones 有 017 版獨有的欄位（net_score / confidence_level …）
down to 16  → 斷言退回 015+016 的形狀（confidence 在、net_score 不在）
down to 0   → 斷言除 goose_db_version 外一張表都不剩
```

日後再寫破壞性 migration 時，測試要比照補上對應的中途斷言，否則那段 Down SQL
等於沒有被驗證過。

理由：手打指令會漂移，本專案已經因此踩過三個坑——以 root 執行留下 root-owned 檔案
（`backend/server` 曾被誤 commit、`backend/internal/ui/dist` 一度無法被覆寫）、
記憶體上限不足導致 Go build OOM、frontend 只掛 `frontend/` 導致 build 產物寫進
container 內憑空消失且不會報錯。腳本是這些約束的唯一真實來源。

## Docker 驗收流程

從 repo root 執行。

### 1. 用測試腳本跑建置與測試

| Runtime | 腳本 | 預設行為 |
|---------|------|----------|
| Backend (Go) | `backend/scripts/test.sh [packages...]` | `go vet` → `go test` → `go build`（全部套件為 `./...`） |
| Python | `python/scripts/test.sh [pytest 參數/路徑]` | 用 `python/Dockerfile` 建測試 image，跑 `pytest backtest/ tests/` |
| Frontend (Svelte) | `frontend/scripts/test.sh [--install]` | `svelte-check`（型別）→ `vitest run`（單元）→ `vite build`，任一失敗即中止 |

```bash
backend/scripts/test.sh                          # 全部 Go 套件
backend/scripts/test.sh ./internal/market/...    # 只驗單一套件
TEST_FLAGS="-count=1 -v" backend/scripts/test.sh ./internal/market/...

python/scripts/test.sh                                   # backtest/ 與 tests/
python/scripts/test.sh backtest/modular/sr_scoring/tests # 指定目錄
python/scripts/test.sh -k event_engine backtest/         # 直接帶 pytest 參數

# 預設 skip、明確要求才跑的測試（例如成本量測）：用 PY_ENV 把 gate 變數送進 container。
PY_ENV="SR_EXCURSION_BENCH=1" python/scripts/test.sh -s \
  backtest/modular/sr_scoring/tests/test_excursion_cost.py

frontend/scripts/test.sh            # 沿用現有 node_modules
frontend/scripts/test.sh --install  # 先 npm ci（node_modules 不存在時會自動加上）

# 開發迭代：只跑指定測試檔（略過 svelte-check 與 build）。驗收仍要跑不帶此變數的完整三步。
VITEST_ARGS="src/routes/SRZones.test.ts" frontend/scripts/test.sh
```

三支腳本共同保證：

- `--user "$(id -u):$(id -g)"`：以本機 uid/gid 執行，container 產出的檔案不會是 root 所有。
- Go build 產物寫到 container 內 `/tmp`、pytest 關閉 `__pycache__` 與 `.pytest_cache`，
  不在 repo 留下編譯／測試殘渣。
- build / module / npm 快取放在 repo 外的 `~/.cache/stock_trading/`（可用 `CACHE_DIR=` 覆寫），
  跨次重用且不會被誤加進版控。

資源限制（預設值，可用環境變數覆寫）：

| 項目 | Backend | Python | Frontend |
|------|---------|--------|----------|
| `MEM`（記憶體上限） | `700m` | `700m` | `440m`（每步一個 container） |
| `MEMSWAP` | 同 `MEM` | 同 `MEM` | 同 `MEM` |
| `CPUS` | `1` | `1` | `1` |
| `--pids-limit` | 200 | 200 | 200 |
| image 覆寫 | `GO_IMAGE` | `PY_IMAGE` | `NODE_IMAGE` |

### `MEM` 是上限，不是預留——不可高於 host 可用量

三支腳本都在 `docker run` 前呼叫 `scripts/lib/mem-guard.sh` 的 `mem_guard_clamp`，把 `MEM`
壓進 host 當下真的供得起的範圍：上限 = `/proc/meminfo` 的 **`MemAvailable`**（不是 `MemTotal`）
減 `MEM_RESERVE_MB`（預設 150）。超過就印警告並自動下修。

**為什麼要有這道護欄**：`--memory` 是 cgroup 的上限，不是向 host 預留記憶體。設得比
`MemAvailable` 高時，container 根本撞不到自己的限制，會先耗盡 host 實體記憶體 + swap，
於是由 **host 層級的 OOM killer** 出手，砍掉 badness 分數最高的行程——在這台機器上就是
**呼叫測試的工作階段本身**（claude CLI，RSS 隨對話成長到 400~500MB）。2026-08-03 與
2026-08-05 各發生一次；2026-08-05 那次的 container 還活過呼叫端並自己跑完（測試全綠，log
留在 scratchpad），死的是呼叫測試的人——**被 kill 後先去撈 log，不要反射性重跑**。

所以遇到記憶體不足時，**要降的是實際用量，不是把 `MEM` 調大**。調大只是把「container 撞
自己的 cgroup 上限」（可回收、錯誤訊息讀得到、工作階段活著）換成「host 砍掉呼叫端」
（整個工作階段連同未回報的結果一起消失）。

### container 上限的**總和**也要顧——本機同時只留一組 stack

同一個失效模式不只來自測試腳本的 `--memory`，也來自**常駐 compose stack 的 `mem_limit`
總和**。每個 stack 都設 `0.5` CPU / `512m` / `768m`，看似安全，但那是**每個 container 各自的
上限**，不是總量保證：10 個 container 加起來就是 ~5 GB 的授權額度，而 host 只有 2 GiB。

2026-08-05 16:22 實際發生過：live project `stock_trading`
被拉起來後，全機共 10 個 container ＋ claude ~400 MB ＋ codex ~137 MB ＋ dockerd ~314 MB，
**沒有任何 container 撞到自己的 512m 上限**（全部 `OOMKilled=false`），是 host 先耗盡，
於是 claude 被砍，接著 `docker-proxy` × 8 與 `dockerd` 也被砍，所有 container 一起停掉。

規則：

- **本機同時只允許一組 stack 常駐**。開工前先 `docker ps --format '{{.Names}}'` 看一眼，
  多餘的先 `compose down`（不帶 `-v`）。
- ⛔ **停 live 之前先確認它是怎麼起來的，並確認你有能力把它原樣拉回來。**
  live 的 `stock_trading` project **不是**由 repo 的 `docker-compose.yml` 部署的，而是
  `/opt/stacks/scripts/stock_trading/`（`compose.yml` ＋ `deploy.sh`，`--project-directory`
  指向 `/opt/stacks/deploy/...` 的另一份 clone）。開關與金鑰都在那裡的 `deploy.sh`／`compose.yml`。

  **危險的地方是 project 名稱相同**：在 repo 目錄下跑
  `docker compose -f docker-compose.yml down` **會成功停掉 live**（compose 以 project 名稱比對），
  於是很容易誤以為 repo 的 compose 就是 live 的來源。**但用 repo 的 compose 拉回來時**，
  `CANDLE_GAP_DETECTION_ENABLED` / `SR_ANALYSIS_ENABLED` / `EVALUATION_UNIVERSE_ENABLED`
  會全部落回 `${VAR:-false}` 的預設、`FINMIND_API_KEY` 會是空字串——
  **服務照樣起得來，什麼都不會報錯**，只是生產排程被靜默關掉、抓取全部失敗。
  2026-08-28 做日 K 缺漏偵測驗收時踩到（原記於 `todo.md` T-067，已收斂；停機時尚未發現，是準備復原才察覺）。

  規則：**復原一律走 `/opt/stacks/scripts/stock_trading/deploy.sh`**（那份設定才是開關與金鑰的
  權威來源），不要用 repo 的 compose 拉 live。

  ⛔ **不要 dump 整份 container env 當備份。** `docker inspect --format '{{range .Config.Env}}…'`
  會把 `AUTH_JWT_SECRET`、`FINMIND_API_KEY`、`FUGLE_API_KEY` 一起印出來，落進終端機
  scrollback、log 或暫存檔就是外洩。**金鑰不需要備份**——它們在受控的 deploy 設定裡，
  復原時由那份設定帶入。

  要核對的話，**只查明確列名的非敏感開關**，例如：

  ```bash
  docker inspect stock_trading-backend-1 --format '{{range .Config.Env}}{{println .}}{{end}}' \
    | grep -E '^(CANDLE_GAP_DETECTION_ENABLED|SR_ANALYSIS_ENABLED|EVALUATION_UNIVERSE_ENABLED|YAHOO_ENABLED)='
  ```

  停機前跑一次、復原後再跑一次比對即可。**金鑰只確認「有沒有設定」，不要印出內容。**
- **停 live 會讓那段時間的排程整段不執行**（沒有補跑機制）。動手前先看今晚還有哪些
  cron：`daily_close` 15:00、`sr_analysis` 17:00、`chip_sync` 21:00、
  `sr_analysis` chip 輪 22:00、`sr_evaluation` 22:30、`evaluation_universe_sync` 16:00。
  挑窗口時把復原時間也算進去。
- **不要在本機把 live/deploy project 拉起來**——驗收一律用 `docker-compose.dev.yml` 的
  dev project（CLAUDE.md 規定）。
- 開跑前 `free -m` 的 `available` 低於 ~800 MB 就先清場再說，不要硬上。
- **每個 stack 的 compose 都要有 `cpus` / `mem_limit` / `memswap_limit`。** 沒有上限的
  container 不受總和約束，會直接侵蝕 `MemAvailable`。gitea 曾是唯一沒設的一個
  （實測佔 154 MB），常駐時 available 只剩 398MB，mem-guard 直接擋下 SR evaluation、
  連 10 檔都跑不起來。2026-08-18 確認已補上 `0.5` CPU / `512m` / `768m`，與其他 stack 一致。

### frontend 三步的記憶體實測（2026-08-06）

`frontend/scripts/test.sh` 的三步裡 **svelte-check 是瓶頸，而且它的用量幾乎壓不下去**。
2026-08-06 逐項量測的結果：

| 嘗試 | 結果 |
|---|---|
| `--max-old-space-size` 180 / 200 / 215 | 全部 OOM |
| `--max-old-space-size` 230 | 通過 → **下限抓 225MB heap** |
| `tsconfig.json` 加 `skipLibCheck: true` | tsc program 由 130.5MB 降到 100.8MB（−23%）、check 時間 9.60s→2.15s（快 4.5×）。**但 svelte-check 的下限幾乎沒降**——svelte 的 language service 才是大宗，不是那 881 個 `.d.ts` |
| `--diagnostic-sources js,svelte`（去掉 css） | heap 200MB 仍 OOM，無效 |
| `--max-semi-space-size=2` | 有沒有它都一樣，無效 |

換算成前置條件：**heap 225 → `MEM` ≥ 330m → host `MemAvailable` ≥ 約 480MB**
（heap = MEM − 100，MEM 再被 mem-guard 減去 150MB 的 reserve）。這台平常在 450~510MB
之間浮動，**正好卡在門檻上**，所以同一份程式碼可能這次過、下次 OOM——不是 flaky，是記憶體。

腳本已在 svelte-check 前加一道警告：clamp 後的 heap 低於 `SVELTE_CHECK_MIN_HEAP_MB`（225）
時明講「很可能 OOM，要解的是釋放 host 記憶體」。**只警告不中止**，因為下限是區間值、
邊界上仍可能過。只想跑單元測試時用 `VITEST_ARGS=...` 略過這一步。

`skipLibCheck` 雖然不解 OOM，仍值得留著：省 30MB 的 margin 與 4.5 倍的 check 速度。
代價是不再檢查函式庫 `.d.ts` 內部的型別錯誤（例如兩個套件的 global 型別互相衝突）；
自己的程式碼對照函式庫型別的檢查完全不受影響，這也是 Vite / Svelte 官方 template 的預設。

### 事後判讀 OOM：這台的 `dmesg -T` 時間不可信

調查被 kill 的原因時，`dmesg -T` 的**絕對時間會錯**（此沙箱 kernel 單調時鐘與 wallclock 有
數十小時偏移，2026-08-05 實測 ~44.8 小時，會把當天的事件標成兩天前）。正確做法：

- 只用 `dmesg` 的**相對間隔**，再拿 `docker inspect -f '{{.State.StartedAt}} {{.State.FinishedAt}}'`
  （docker 用自己的 wallclock，可信）對齊到真實時間。
- 決定性驗證：`dmesg` 最後一行的 veth 名稱若等於 `ip -o link` 目前唯一存在的 veth，
  該行就是最近一次 container 啟動的時點。
- kernel ring buffer 只留約 60 行 / 1.7 小時，查不到不等於沒發生。

護欄開關（環境變數）：

| 變數 | 預設 | 作用 |
|------|------|------|
| `MEM_RESERVE_MB` | `150` | 保留給呼叫端**執行期間再成長**的量。`MemAvailable` 已扣掉各行程當下的 RSS，不需在此重複涵蓋；設太大會讓上限低到無法執行 |
| `MEM_MIN_MB` | `256` | 下修後低於此值直接中止，並提示先關掉常駐 container |
| `MEM_STRICT` | `0` | 設 `1` 時不下修，直接中止（想明確發現設定錯誤時用） |
| `MEM_FORCE` | `0` | 設 `1` 時完全略過護欄 |

`MEMSWAP`（`--memory-swap`）預設等於 `MEM`，即**關掉 container 的 swap**。這台 host 的 512MB
swap 常態 100% 用滿，放任 container 換頁只會拖垮整台機器，不如讓它乾脆撞 cgroup 上限。

Go 另外固定 `GOMAXPROCS=1` + `GOFLAGS=-p=1`：本機只有 2GiB RAM，平行編譯會 OOM，
必須序列編譯；記憶體上限也因此不能沿用其他 runtime 的 512m。再加上 `GOGC=off` +
`GOMEMLIMIT`（腳本的 `GO_MEMLIMIT`，預設 `250MiB`，可經環境變數上調）：序列化只限制
**併發數**，不限制單一 go 子行程的 heap，container 的 `--memory` 也擋不住 host 層級的
OOM killer。沒有這兩項時，`modernc.org/sqlite/lib`（C 轉譯的巨大 generated package）的
vet／compile 會出現 `vet: signal: killed`，`set -e` 讓整條 vet → test → build 直接中止。
這些只影響編譯過程，不影響測試結果與產出的執行檔。

Frontend 注意事項：`vite.config.ts` 的 `outDir` 是 `backend/internal/ui/dist`
（Go embed 使用、且有進版控），所以腳本掛載的是 **repo root** 而非 `frontend/`。
跑完 `git status` 出現 dist 差異屬正常，要不要保留該次產物由當次工作決定。

Frontend 顏色語意（`tailwind.config.js`）：`rise: #e74c3c`（**紅**）、`fall: #2ecc71`（**綠**），
名稱來自台股「漲紅跌綠」。**關鍵陷阱：`fall` 是綠色**，不要因為「fall 聽起來像壞事」就拿去標
錯誤或危險操作，那會把警示顯示成安全色。用法分三類：

| 情境 | 用什麼 | 例子 |
|------|--------|------|
| 行情語意 | `text-rise` / `text-fall` | 漲跌、損益、買賣超、停損價、最大回撤 |
| 錯誤／失敗訊息文字 | `text-rise`（紅） | job error、觸發失敗、載入失敗 |
| UI 狀態徽章與動作按鈕 | tailwind 色票 `text-red-400` / `text-green-400` 等 | 刪除／取消／停用按鈕、status chip |

第二類沿用 `text-rise` 是既有慣例（`SRZones.svelte`、`Scheduler.svelte` 的載入錯誤本來就這樣）；
第三類用色票是因為按鈕本來就與行情無關，且同組按鈕的另一分支已經是
`border-green-600/40 text-green-400`，用色票才對稱。

這個坑實際發生過兩次（2026-08-05 修正）：`Scheduler.svelte` 整檔把 job 錯誤與失敗數標成綠色
（同檔的載入錯誤卻是紅色，自相矛盾）；`Users.svelte` / `Backtest.svelte` / `Analysis.svelte`
的停用／取消／刪除按鈕也是綠色，其中 `Users.svelte` 的切換按鈕**兩個狀態都是綠色**，使用者
完全分不出哪邊是危險操作。已在 `Scheduler.test.ts` / `SRZones.test.ts` / `Users.test.ts` /
`Backtest.test.ts` / `Analysis.test.ts` 加上 class 斷言鎖住——**只斷言文字內容的測試抓不到這種錯**。

Frontend 測試框架（三層）：

- **型別**：`svelte-check`（含 `.svelte` 內 TS），對應 `npm run check`。`tsconfig.json` 的
  `strict: true` 從此被真正執行；`src/vite-env.d.ts` 補上 `vite/client` 型別讓 `import.meta.env`
  可用。
- **單元 / 元件**：`vitest` + `@testing-library/svelte`（v4，對應 Svelte 4）+ `jsdom`，對應
  `npm run test:unit`。測試檔為 `src/**/*.test.ts`；純邏輯（`lib/utils`、`lib/stores`、`lib/api`）
  與元件渲染（`.svelte`）皆可測。目前為框架＋種子測試，覆蓋逐步補齊。
- **設定分離**：測試用獨立的 `vitest.config.ts`（不帶 `vite.config.ts` 的 build `outDir` /
  `emptyOutDir` / manualChunks，避免跑測試誤動 dist 產物）；`resolve.conditions=['browser']` 讓
  Svelte 元件能在 jsdom 掛載；`vitest-setup.ts` 載入 jest-dom matcher 並手動 `afterEach(cleanup)`。
- **記憶體**：2GiB host 下 vitest 以 `pool: 'forks'` + `singleFork` 限制併發（比照 Go
  `GOMAXPROCS=1`、Python `-p=1`）。腳本另外把三步**拆成三個獨立 container** 依序執行
  （`npm run check` → `npm run test:unit` → `npm run build`）：每步跑完就退出、記憶體立刻
  歸還 host，峰值變成三者的 **max 而非 sum**，哪一步爆掉也一眼可辨。再加
  `NODE_OPTIONS=--max-old-space-size`（腳本的 `NODE_HEAP_MB`，預設 320）——node 的預設
  old-space 由可用記憶體推導，不明確指定就會一路漲到接近 cgroup 上限才認真 GC，這是把實際
  用量壓下來的主要槓桿。2026-08-05 實測三步峰值（`memory.max_usage_in_bytes`，含可回收的
  page cache）：svelte-check 376MB、vitest 359MB、vite build 370MB；`MEM` 預設 440m，被護欄
  下修到 365m 時三步仍全過（緊縮時 kernel 直接回收 page cache，不會 OOM）。
  **下限**：`svelte-check` 的 node heap 低於約 200MB 會直接 `JavaScript heap out of memory`
  （實測 198m 失敗、231m 通過）。host 吃緊到 `MEM` 被下修至 300m 以下時就會踩到，這時不是
  改 code，是等記憶體回來或調 `MEM_RESERVE_MB`。失敗發生在 container 內，呼叫端不受影響。
  **不要因為想給測試更多空間而調高 `MEM`**，理由見上面的「`MEM` 是上限，不是預留」。
- **版本相容**：Svelte 4 需 `svelte-check@^3`（v4 需 Svelte 5）與 `@testing-library/svelte@^4`
  （v5 的 `svelteTesting` vite plugin 需 Svelte 5，本專案不適用，故手動設定 browser condition
  與 cleanup）。

Backend image build 的記憶體約束：`backend/Dockerfile` 的 builder stage 固定
`GOFLAGS=-p=1`、`GOMAXPROCS=1`、`GOGC=off`、`GOMEMLIMIT=250MiB`。這台 host 只有 2GiB RAM
（實際可用約 700MB），沒有這些設定時 `redis/go-redis`、`modernc.org/libc` 的 compile 會被
OOM killer 砍掉（`signal: killed`）；實測光是序列化還不夠，`GOMEMLIMIT=500MiB` 仍失敗，
壓到 250MiB 才能在冷 cache 下編完（約 3 分鐘）。這些只影響編譯過程，不影響產出的執行檔。

### 2. 啟動 dev stack 做 smoke test

`docker-compose.dev.yml` 已對 dev stack 服務套用同樣的資源限制：
每個 container 預設最多 `0.5` CPU、`512m` 實體記憶體、`768m` memory+swap、`200` 個
process/thread。

```bash
scripts/smoke-dev.sh
```

腳本流程是 **先停 → 再 build → 再啟動**：build 之前會對 dev project 執行
`compose down --remove-orphans`（**不帶 `-v`，named volume 保留**）。原因是這台 host 只有
2GiB RAM，實測冷 cache build 的低點只剩 74 MiB available（Go compile 峰值 RSS 約 420 MiB），
而上一輪留著的 dev stack 約占 145 MiB（postgres 26＋redis 9＋backend 9＋python-server 99），
不先停就會在 build 階段被 OOM killer 砍掉。確定記憶體充裕時可用 `SKIP_DOWN=1` 略過這一步。

可用環境變數覆寫等待時間、log 行數或 health URL：

```bash
WAIT_SECONDS=120 LOG_TAIL=200 scripts/smoke-dev.sh
BACKEND_URL=http://localhost:18080/health PYTHON_URL=http://localhost:18001/health scripts/smoke-dev.sh
```

**gin 預設跑 release**（`cmd/server/main.go`，只在 `GIN_MODE` 沒設時套用）。要看啟動時的
路由表就暫時開 debug——兩份 compose 都有 `GIN_MODE: ${GIN_MODE:-}` 的 passthrough：

```bash
GIN_MODE=debug scripts/smoke-dev.sh
docker logs stock_trading_dev-backend-1 2>&1 | grep GIN-debug   # release 下是 0 行
```

debug 模式會多印 76 行 `[GIN-debug]`（完整路由表 ＋ handler 符號名），而且 **panic 時
`recovery` 會把整包 request header 寫進 log**（gin 只把 `Authorization` 遮成 `*`，
Cookie 與自訂 header 照樣落地），所以只在需要時暫時開。**切 release 不是效能考量**：
gin v1.10 的 `IsDebugging()` 分支沒有一個在 per-request 熱路徑上，唯一會逐次付出成本的
`HTMLDebug` renderer 只在 `LoadHTMLGlob`/`LoadHTMLFiles` 才用得到，而前端是 `//go:embed`
的靜態 dist。

需要手動查看狀態或 log 時：

```bash
docker compose -f docker-compose.dev.yml ps
docker compose -f docker-compose.dev.yml logs --tail=200 backend
docker compose -f docker-compose.dev.yml logs --tail=200 python-server
```

dev stack 也會把 app runtime log 寫到 repo root 的 `logs/dev/`，避免 container 重新建立後只剩
Docker stdout 可查：

| 服務 | 持久化路徑 |
|------|------------|
| backend | `logs/dev/backend/` |
| python-server | `logs/dev/python-server/` |
| python-worker | `logs/dev/python-worker/` |

backend 會寫每日檔案 `backend-YYYY-MM-DD.log`；Python 服務會寫目前檔案
`python-server.log` / `python-worker.log`，並在每日輪替後保留日期後綴檔案。app log 時間一律使用
UTC ISO 8601，保留天數由 `LOG_RETENTION_DAYS` 控制，預設 14 天。

停止 dev stack：

```bash
docker compose -f docker-compose.dev.yml down
```

清空 dev 驗收資料：

```bash
docker compose -f docker-compose.dev.yml down -v
```

## 開發完成標準

完成程式修改後，至少要做：

- 受影響 runtime 的測試腳本（`backend|python|frontend/scripts/test.sh`）。
- 若有 migration、API、跨服務整合、排程或 Python/Go 互動，跑 `scripts/smoke-dev.sh` 做 dev stack smoke test。
- 若改到 `migrations/mysql/`，另外跑 `scripts/test-mysql-migrations.sh`。
- 若有前端畫面變更，跑 frontend Docker build，並在 dev stack 或本地 dev server 驗證畫面。
- 若因環境、網路或外部 token 無法執行某項驗證，最後回報要明確寫出未執行項目與原因。

宣告完成或移除 issue/todo 項目前，逐項走過下方「結案確認清單（Definition of Done）」。

## 結案確認清單（Definition of Done）

「開發完成標準」是最低要求，這份清單是**宣告任務完成前（或把 issue/todo 項目移除前）逐項確認的
操作版**。每一條都對應本專案實際踩過的結案缺陷，別跳過。整份走完再說「完成」。

### A. 測試驗證

- [ ] 受影響 runtime 的 `scripts/test.sh` 全綠，且用 `-count=1`（或等效）跑過一次，不靠 cache 假綠。
- [ ] 新增／修改的邏輯**每個分支**都有斷言。曾發生新的 semantic action / position context 分支
      （例如 `DEFEND_BREAKDOWN`、`POSITION_SUPPORT_DEFENSE`、`POSITION_RESISTANCE_OVERHEAD`）只實作沒測到。
- [ ] 期望值來自**規格**、且測的是 **production 真的會產生的輸入**。不要手工捏造 production
      永不出現的分歧來「驗證」一個實際空轉的能力（`final_entry_gate_state` echo 的教訓）。見品質守則 §1。
- [ ] 若動到 migration／API／跨服務／排程／Python↔Go 互動，跑過 `scripts/smoke-dev.sh`。
- [ ] 若動到 `migrations/mysql/`，另外跑過 `scripts/test-mysql-migrations.sh`（dev/live 是
      postgres，mysql 那份沒有其他執行路徑）。

### B. 文件收斂與狀態誠實

- [ ] 主題文件（`sr-zone-scoring.md` 等）與實作一致，**不得把「目標終局」寫成已完成**；只做一半就
      如實標「⚠️／規劃中」。曾把 annotation 層寫成「已達成單一真相源」。
- [ ] 完成的 `issue.md` / `todo.md` 項目已移除或搬到「已完成封存」；移除前把 durable 設計寫回主題文件，
      並**修掉其他文件指向該筆的交叉引用**（避免斷鏈）。見「文件收斂規則」。
- [ ] 狀態誠實：phased 工作在收尾前標「進行中」並保留剩餘 phase，**不要提前標「已完成」**。曾兩次把
      phased semantic pipeline 工作在只做到一半時標成完成。

### C. 前後端契約與一致性

- [ ] 新增分析欄位時 Python → Go(`internal/analysis/client.go`) → TS(`lib/api/*.ts`) 三端同步，
      新欄位用 `omitempty`／optional 保向後相容。見「SR Zone / 分析輸出欄位開發注意事項」。
- [ ] 新的 `decision_summary`／derived 欄位**已在前端接線或顯式延後**，不能只加型別不渲染。見品質守則 §3。
- [ ] 共用的 label／對照表抽到**單一模組**（例如 `derivedReasonLabel` 放 `srZones.ts`），不要兩個頁面各自
      維護一份而漂移。

### D. 不留 dead / echo / 雙真相源

- [ ] 沒有只是鏡像另一欄位的 echo 欄位（如 `final_entry_gate_state` = `entry_action_state`）。
- [ ] 沒有 legacy state 與 derived gate 並存的雙真相源；legacy 欄位要嘛由 gate 推導、要嘛退役並在文件
      標明哪個是權威、消費端不應讀 legacy。
- [ ] 重複邏輯已收斂到單一 helper（如 blocking-zone 偵測），避免兩份 copy 漂移。

### E. 產物與版控

- [ ] 前端有變更 → **重新 build dist 並 `git add backend/internal/ui/dist`**。`git commit -am` 不會帶
      未追蹤的新 chunk，漏了會讓 `index.html` 指向不存在的檔案、Go embed 前端 404。
      **這一條現在有機械化保護**：`frontend/scripts/test.sh` 的 build 之後會跑
      `scripts/check-dist-assets.sh --fix`，自動把 index.html 引用到的 asset stage 起來
      （`DIST_AUTOSTAGE=0` 可改成只檢查不修）。要單獨驗證時直接跑
      `scripts/check-dist-assets.sh`（未納入版控就非零退出）。
      **這件事無法用 Go 測試把關**——`//go:embed` 讀磁碟不讀 git，本機檔案在就會過。
      這條規則在 2026-08-10 之前就寫在這裡，仍被漏掉一次，所以才補上自動化。
- [ ] `git status` 只剩本次預期的改動：無 root-owned 檔案、無誤入的執行檔／`__pycache__`／快取殘渣。

### F. 回報

- [ ] 回報明確寫出「跑了什麼、結果、沒跑什麼與原因」；測試失敗或步驟略過要如實說，不要含糊帶過。

## 開發慣例（品質守則）

這些是從實際 code review 累積的品質守則，補在「開發完成標準」之外，針對容易長期潛藏、
不會在單次功能測試中浮現的缺陷型態。每條都附上實際踩過的案例，方便對照。

### 1. 測試對「規格」斷言，不要對「當下輸出」斷言

- **原則**：凡是文件（主題文件／spec）有明確公式或契約的行為，測試的期望值要依規格獨立算出，
  而不是把程式當下的輸出貼回斷言。
- **為什麼**：`chip_summary.effective_score` 未依 coverage 降權的缺陷長期存在，正是因為測試直接
  斷言了錯誤輸出（滿覆蓋斷言 `== 42.5`，即未降權的 `total_score`），另一筆 fixture 又剛好讓兩種
  公式相等而遮蔽分歧——測試反而幫 bug 背書。
- **具體做法**：
  - fixture 刻意挑「會讓錯誤實作失敗」的輸入（例如讓 `total_score ≠ raw_score * coverage`）。
  - 期望值用規格公式手算或獨立推導，並在測試留註解說明來源。
  - review 時問一句：這個期望值來自規格，還是來自現有輸出？

### 2. 對外文件化的欄位要有契約測試

- **原則**：只要欄位寫進對外契約（例如 `sr-zone-scoring.md` 的 `decision_summary` /
  `chip_summary`），就要有測試守住「欄位存在，且宣稱非 null 的確實有值」。
- **為什麼**：本次 review 靠人工才發現 `price_path.blocking_zone.zone_id` / `timeframe` 恆為 null、
  部分「拆分」欄位其實是別名、`final_entry_permission` 的 `BUY` 端到端不可達。`sr-zone-scoring.md`
  本就要求「新分析缺欄位應由 Python 單元測試攔下」，但覆蓋不完整，落差只能靠人工發現。
- **具體做法**：
  - 對代表性輸入做 snapshot／欄位集合斷言，契約偏移時 CI 先擋。
  - 對「文件宣稱一定有值」的欄位加非 null 斷言；若目前刻意恆 null（如 `zone_id`），文件要標明
    現況、測試斷言其為 null，讓文件與測試兩邊一致。

### 3. 新增 `decision_summary` 欄位，把「前端消費」納入完成定義

- **原則**：Python 端新增／拆分 `decision_summary` 欄位時，要一併決定前端如何呈現，不能只加
  Python 輸出與 TS 型別就當完成。
- **為什麼**：`final_entry_permission`、`rr_context`、`nearest_support/resistance_zone` 等一度只在
  Python 產出、TS 宣告型別，但 `SRZones.svelte` 未渲染、舊單一欄位仍顯示造成誤導（該批欄位已於
  `SRZones.svelte` 補上呈現）。Python↔Go↔TS 五層契約有寫，但沒有機制確保新型別真的被消費，這條
  慣例就是要補上這個缺口。
- **具體做法**：
  - plan/PR 明確標記每個新欄位的前端處置：「本次接線」或「顯式延後到 T-xxx」。
  - 顯示新拆分欄位時，同步移除或標註被取代的舊單一欄位，避免殘留誤導。
- **沒被消費的型別還會默默寫錯**（2026-08-06，daily confirmation 前端接入）：
  `SRDailyConfirmationSummary.by_state` / `by_primary_role` 一度被宣告成
  `SRDecisionOutcomeGroup`，但 Python `_daily_confirmation_groups` 實際回傳的形狀與它
  **除了 `rows` 之外零重疊**。因為從沒有任何地方消費這兩個欄位，`svelte-check` 與 build 都
  不會比對它跟真實資料——型別看起來「有寫」，其實是錯的。這比單純漏渲染更難發現：漏渲染至少
  肉眼看得出畫面少東西，型別錯了則要等到有人真的去用才會炸。
  **所以「型別已宣告」不能當成進度**，只有被渲染且有測試斷言過的欄位才算接線完成。

  同理，新增分層／統計欄位時不要在前端自行推導比率。該批分層只提供原始 counts，Python 的
  `_outcome_rate` 帶 `primary_role` 過濾語意，前端相除得到的數字會跟後端定義悄悄分岔，
  且不會有任何測試發現。要比率就在 Python 算好送過來。
- **測試 fixture 必須是後端真的會產生的形狀，斷言必須到值**（2026-08-06 實例）：
  `zone_outcomes` 的分層比率在前端永遠顯示 `—` 長達數週，因為 Python 回的是
  `hold_rate`/`break_rate`、前端讀的是 `support_hold_rate` 等三個不存在的 key。
  三層測試全綠是因為——**前端測試的 fixture 是憑印象手寫的**，用了後端從不產生的 key，
  於是「測試通過的是一份不存在的資料形狀」；而 Python 測試只斷言分層非空與 rows 加總，
  剛好完全避開出錯的欄位。`—` 又看起來就像「這組沒資料」，肉眼也發現不了。

  兩條具體要求：

  1. **fixture 從真實輸出取樣**，不要憑記憶手寫。跑一次真的產出、複製其中一段當 fixture。
  2. **斷言要到值**，不要只斷言區塊／表格存在。`getByText(/Zone 層指標/)` 這種斷言在三個欄位
     全是 `—` 時照樣通過；鎖到該列的 `<td>` 斷言「出現 62.0%」才擋得住。
     同理，null 的欄位要斷言它顯示 `—` **且不出現 `0.0%`**。
  3. 跨語言的欄位契約，**最終要靠一次真實資料的實跑驗證**——單元測試兩邊都用合成資料時，
     兩邊可以一致地錯。這個 bug 就是靠 T-039 的 Pass 0 實跑才浮出來。
  4. **fixture 的型別也要跟真實來源一致，不只是欄位名**（2026-08-07 補）。同一條規則被踩過
     兩次：一次是 key 名不一致，一次是型別不一致——`trade_date` 的 fixture 一律是字串，
     而 postgres 的 `date` 欄位經 psycopg2 讀出來是 `datetime.date` 物件，整份 report
     在最後一步 `json.dumps` 才炸掉，前面數十分鐘的運算全部白跑。
     **DB 讀出的值要在邊界正規化成 JSON 安全型別**（本專案用
     `scoring._iso_date()`），**不要用 `json.dumps(default=str)` 這種全域逃生口**
     ——那會讓下一個型別洩漏同樣無聲無息地混進 API 回應。

- **不要用「總數」當斷言，要逐項斷言**（2026-08-07 補）：
  `expect(getAllByText('隔日/SUPPORT_HELD').length).toBe(9)` 這種斷言看似嚴格，實際上很脆弱
  ——每新增一個分層就要回來改數字，而**改錯數字比漏測更難察覺**（把 9 改成 14 但其實漏接了
  一個分層，測試照樣綠）。改成逐個標題與桶名斷言：漏接任何一項都會失敗，且失敗訊息直接
  指出漏了哪一個。

- **會被機器產生的東西，斷言要對著「產生者」而不是手寫樣本**（2026-08-10 補）：
  分桶函式若依上游欄位的有無來分類，就要有一條測試鎖住**上游的不變式**。
  實例：`_rr_formula_state` 曾依 `risk_price`/`reward_price` 的四象限分桶，但
  `decision_engine._rr_context()` 的 `reward_price` 只在 `if risk > 0:` 內賦值，
  「只有 reward」那一格**永遠是空的**。真實資料跑出 0 筆時，很容易被誤讀成
  「風險側從不缺」這種結論。只餵手寫 dict 給 helper 的測試抓不到這種事。

#### 什麼時候才該新增跨語言的型別宣告

前後端的 TS interface 是 Python dict 的**手抄鏡像**，兩者之間沒有自動連結，**只靠人的自律
必然會過期**。所以「加不加型別」不是憑感覺決定的，用這條判準：

> **這個型別今天有沒有「因為它寫錯而會失敗」的消費者？**

| 情況 | 決定 | 理由 |
|---|---|---|
| 有畫面／匯出在讀它 | 加 | 寫錯會被 svelte-check 或渲染斷言抓到 |
| 只有一份**取自真實輸出**、且斷言到值的 fixture | 加 | fixture 就是消費者，符合上面兩條要求 |
| 沒有消費者 | **不加** | 就是下一個會默默寫錯的宣告，還給人「contract 已守住」的假象 |

**沒有消費者時，改把防漂移機制放在 Python 側**——TypeScript 偵測不到 Python 的變動，
這個不對稱正是前述兩次事故的共同結構。做法是在 Python 測試對該 projection 加**精確的
key 集合斷言**（`assert set(row["primary_zone"]) == {...}`），註解指向對應的 TS 檔案。
欄位增減會讓 Python 測試失敗並提醒一起檢查，那份清單就是**權威形狀**，
日後真要加型別時不必憑記憶手寫。現行實例：`test_evaluation.py` 對
`replay_rows[].primary_zone` 的斷言。

**未來真的要加型別時，三件事要一起做**（缺一就會重蹈覆轍）：

1. **型別與消費同批進來**，不要先加型別佔位。
2. **fixture 從真實輸出取樣**，不手寫。
3. **在解析點收斂**：寫 `toXxx(raw: unknown): Xxx` 之類的解析函式，
   讓型別因為被解析而必然被消費，而不是掛著沒人碰。

### 4. 測試不要依賴「真實今天」

- **原則**：測試的期望值不得取決於執行當天的日期。要嘛把日期寫死並直接測純函式，
  要嘛讓斷言在任何日期下都成立。
- **為什麼**：這種缺陷**會自己好**，所以特別難查。2026-09-02 有兩支
  `candle_gap_detection` 的測試失敗（`SkipsBreakerOpenSourceAndStillPartial` 與
  `PrioritisesNeverAttemptedCandidates`），根因是 helper 依真實今天往回數三個工作日，
  當天拿到 `[08-28, 08-31, 09-01]`——**跨月**。受測標的缺的兩天分屬兩個月份，
  `(symbol, month)` 去重就把一檔拆成兩次請求，於是 `[]string{"6182"}` 變成
  `["6182","6182"]`。**每個月頭幾個交易日都會失敗，過幾天又自己好**，
  而 production 行為從頭到尾都是對的——錯的是測試對日期的假設。
- **具體做法**：
  - **月份／週期敏感的邏輯，直接測純函式並寫死日期。** 同一個檔案裡的
    `TestCandleGapDetectionQueriesEveryMonthInWindow` 就是正解：直接呼叫
    `verifyCandidates` 並給 `2026-07-31` / `2026-08-03` / `2026-08-04`，
    註解寫明「跨月的判斷不依賴『今天是哪一天』才穩定」。
  - **非測日期本身時，讓斷言與日曆無關。** 上述兩支的修法是讓受斷言的標的
    **只缺一天**——單一日期必然只落在一個月份，`(symbol, month)` 恆為一組。
  - **警訊**：測試裡出現 `time.Now()`／`today` 衍生的 fixture，而斷言又是精確的
    集合或長度比對，就要問一句「跨月、跨年、連假、月初月底都成立嗎」。
  - 相對日期的 fixture 若無法避免，**在測試留註解寫明它對日期做了什麼假設**，
    否則下一個人只會看到一支間歇失敗的測試。

### 5. 模組 import 不得有連線等副作用，測試要能獨立啟動

- **原則**：import 一個模組不應該就去連 DB／外部服務；連線要延後到真正使用時（lazy），
  讓純單元測試不依賴外部環境。
- **為什麼**：`db.py` 原本在 import 時就 `engine.connect()` 並在失敗時 `raise`，任何 import 到
  `db`（或間接透過 `scoring` 等）的模組在連不到 DB 時都會在收集階段整包失敗，害 §1 的
  `pytest backtest/` 在乾淨容器跑不起來。現已改為 lazy：連線健康檢查抽成 `db.check_connection()`，
  由服務啟動路徑（`http_server` / `worker` / `train` CLI）明確呼叫，import `db` 不再有連線副作用，
  §1 的指令因此不需要任何 DB 環境變數覆寫即可跑。
- **具體做法**：
  - 需要啟動時 fail-fast 的服務／CLI，在進入點呼叫 `db.check_connection()`，不要靠 import 副作用。
  - 新寫的模組比照辦理：建立 engine／client 可以在 module level，但實際連線要延後到使用或明確的
    啟動檢查，別放在 import 時執行。
- **FastAPI 服務的「進入點」＝ lifespan，不是 module 頂層**（2026-08-06，T-037 C）：
  `http_server.py` 原本在 module 頂層直接呼叫 `check_connection()`，等於「import 這個模組 ==
  必須連得到 DB」，讓 `/sr-scoring/evaluate` 完全無法用 FastAPI TestClient 測（該端點因此長期
  0 測試）。現改成 `@asynccontextmanager` 的 `lifespan`，掛進 `FastAPI(lifespan=...)`：

  ```python
  @asynccontextmanager
  async def lifespan(app: FastAPI):
      check_connection()
      yield

  app = FastAPI(..., lifespan=lifespan)
  ```

  兩條啟動路徑（compose 的 `python http_server.py` → 檔尾 `uvicorn.run(app)`、`start_server.sh`
  的 `uvicorn http_server:app`）都會經過 lifespan，**fail-fast 行為不變**：已實測連不到 DB 時
  uvicorn 記 `Application startup failed. Exiting.` 並以 **exit 3** 退出，container 照樣依
  `restart: unless-stopped` 重啟。
- **測試側的對應寫法**：starlette 的 `TestClient` **只有被當成 context manager 使用時才會跑
  lifespan**。所以端點測試一律用 `TestClient(app)`（不加 `with`）＝ 完全不需要 DB；只有要驗證
  啟動行為的測試才寫 `with TestClient(app):`。這點寫在 `python/tests/conftest.py` 的 `client`
  fixture 註解裡——若有人順手改成 `with`，整批端點測試會突然需要 DB。

## 文件收斂規則

發現與實作不一致時：

- bug、矛盾結果、誤導行為、文件與實作不一致、已知限制：記到 `docs/issue.md`。
- 未來優化、功能擴充、重構、待規劃工作：記到 `docs/todo.md`。
- 已完成的 issue/todo 要移除；移除前，把值得長期保存的行為或設計寫回對應主題文件。

計畫書的 review 沿革（大型計畫書常有「Review findings → Review 回應 → 裁決」多版來回）：

- **被否決的建議加註，不要抹掉。** 直接刪掉會讓後人看不到「為什麼最後不是這樣定的」；
  作法是原句加刪除線 ＋ 標「已被第 N 版否決」＋ 指向最終定案的位置。
- **歷史區塊的標題要自己說明它是歷史**（例如「原始 Review findings（已由第 2／3 版處理，
  保留作決策沿革）」），並在前言寫明「實作一律以本文契約表與最新版裁決為準」。
- **小節標題統一為 `小節名（日期，版次）`；內文引用時引號內只寫小節名本身**，不帶日期與版次，
  需要標版次時寫在引號外。這樣標題補日期或改版次時，內文引用不會跟著失效。
- **同一份計畫書內做字串替換式的改名，改完要整段重讀**——只匹配到句子前半會留下拼接殘句，
  這類殘留曾經自己變成一筆 issue。

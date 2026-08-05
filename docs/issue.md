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

---

### I-040：production regression governance gate 在該模型首次 decision-replay 寫入前為 no-op（by-design）

| 欄位 | 內容 |
|---|---|
| 狀態 | 已知限制（刻意，by-design） |
| 嚴重度 | 低 |
| 分類 | Python / SR Zone / 模型治理 |
| 發現日期 | 2026-07-27 |
| 來源 | T-002 P2 review |

`pipeline._merge_regression_governance_gate` 只有在 `fetch_latest_sr_regression_governance(model_config_hash)`
查得同模型、`schema_version=sr_zone_decision_replay_p0` 的最新 replay 結果時才會作用。若該
`model_config_hash` 尚未跑過任何 `--write-db` 的 decision replay（新訓練模型、或 scheduler 關閉且從未手動
執行），fetch 回 None → gate 為 no-op，分析維持原本模型治理。這是刻意的安全預設（gate 只趨保守、
不因缺資料而誤擋），但意味著**這層 P2 安全網要等該模型至少跑過一次 evaluation 並寫入 DB 後才生效**。
上線流程若倚賴此 gate，需確保新模型部署後有排入一次 decision replay。屬營運相依，非 bug。

---

### I-053：2 GiB host 下 live stack 與本機開發工具（claude / codex）不可併存，會被 host OOM killer 砍掉呼叫端

| 欄位 | 內容 |
|---|---|
| 狀態 | 已知限制（環境限制，不改 runtime） |
| 嚴重度 | 中 |
| 分類 | 開發環境 / Docker / 記憶體 |
| 發現日期 | 2026-08-05 |
| 來源 | 2026-08-05 16:22 claude 被 OOM kill 事故調查 |

**現象**：Claude Code 這端完全閒置（沒跑測試、沒 build）時，claude 行程仍被 host OOM killer
砍掉，並連帶引發 `docker-proxy` × 8 與 `dockerd` 被砍、所有 container 被停的 cascade。

**時序（2026-08-05，CST）**：

| 時間 | 事件 |
|---|---|
| 15:56:03 | live project `stock_trading`（backend / python-server / python-worker）被啟動 |
| 16:22:02 | host OOM killer 砍掉 claude（anon-rss 402 MB、total-vm 5.9 GB） |
| 16:22:31–16:23:57 | 連續砍掉 8 個 `docker-proxy` |
| 16:23:57 | 砍掉 `dockerd`（anon-rss 314 MB） |
| 16:23:58 | systemd 重啟 dockerd，接手後把殘留 container 全停（`python-worker` 逾時被 SIGKILL → `exit=137`） |
| 16:24:13 | 只有 gitea（restart policy）回來 |

**根因**：host cgroup 上限只有 2 GiB，事發當下同時有 **10 個 container**（15:56 才起的 live
stack 3 個 ＋ 既有 7 個：dev postgres、postgres、redis、caddy、fin-api、akatengu、gitea），
外加 host 上的 claude ~400 MB、codex ~137 MB、dockerd ~314 MB、containerd 與 10 個 shim。
每個 container 都設了 `mem_limit: 512m`，但**加總約 5 GB 遠超過 host 的 2 GiB**——per-container
上限只保證單一 container 不超標，不保證總和。所以沒有任何 container 撞到自己的 cgroup 上限
（全部 `OOMKilled=false`），是 host 先耗盡，再由 host 層級 OOM killer 挑 badness 最高的行程，
也就是持有最大 heap 的 claude。詳見 `development-workflow.md` 的
「`MEM` 是上限，不是預留」與「container 上限的**總和**也要顧」。

**規避方式**：本機同時只允許一組 stack 常駐。要跑 live project 前先確認沒有其他 stack 佔著
記憶體；驗收一律用 `docker-compose.dev.yml` 的 dev project（CLAUDE.md 規定），不要在本機把
live/deploy project 拉起來。

**調查此類事故時的陷阱**：這台沙箱的 `dmesg -T` **絕對時間不可信**（kernel 單調時鐘與
wallclock 有數十小時偏移，本次為 ~44.8 小時，會把當天事件標成兩天前）。判讀方式：

- 用 `dmesg` 的**相對間隔**搭配 `docker inspect` 的 `StartedAt` / `FinishedAt`（docker 用自己的
  wallclock，可信）交叉定位。
- 決定性驗證：`dmesg` 最後一行的 veth 名稱是否等於 `ip -o link` 目前唯一存在的 veth，
  是的話該行就是「最近一次 container 啟動」。
- kernel ring buffer 只留約 60 行 / 1.7 小時，更早的事故不會留下紀錄，別把「查不到」當成
  「沒發生」。

**附帶發現（待套用）**：`/opt/stacks/scripts/gitea/compose.yml` 是唯一沒有 `cpus` /
`mem_limit` / `memswap_limit` 設定的 stack（實測佔 154 MB），其他 stack 都是
`0.5` CPU / `512m` / `768m`。已決定補齊，但該路徑屬 `kirai`（uid 1000），需由該帳號或
sudo 手動套用後重啟 gitea。

---

下一筆新問題從 `I-054` 起編。

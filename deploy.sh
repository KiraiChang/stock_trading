#!/bin/bash

# 填入 git source 的絕對路徑
PROJECT_DIR="/path/to/trading"

# 填入機敏設定
export AUTH_JWT_SECRET="change-me"
export FINMIND_API_KEY="your-finmind-api-key"
export FINMIND_INTRADAY_ENABLED="false"  # 帳號升級到 Sponsor 級以上再改為 "true"
export FUGLE_ENABLED="false"           # 驗證通過、確定要開通即時行情後改為 "true"
export FUGLE_API_KEY="your-fugle-api-key"
export YAHOO_ENABLED="false"           # 盤中批次源（非官方 API，免 token）；實盤驗證(T-032)通過後改為 "true"
export YAHOO_RATE_LIMIT="20"           # 每分鐘請求上限（批次計一次），保守預設，依實測封鎖風險調整
export YAHOO_BATCH_SIZE="40"           # 單次請求帶入的 symbol 數，依實測調整

# 評估標的池（T-040 Step 5）：每交易日 16:00 維護池內標的的日 K。
# 開啟前要先匯入選池成員（前端「評估標的池 → ③ 已入池」），空池時這個 job 什麼都不做。
export EVALUATION_UNIVERSE_ENABLED="false"  # 匯入選池成員後改為 "true"
export EVALUATION_UNIVERSE_CRON="0 16 * * 1-5"  # 台北時區
export EVALUATION_UNIVERSE_DAYS="10"       # 往前幾個日曆天；10 是為了容忍連假，成本與天數無關

# 日 K 缺漏偵測（現況見 docs/architecture.md；原記於 issue.md I-091，已收斂）。沒有自己的 cron，跟著 evaluation_universe 那輪
# （16:00）在回補之後跑，但寫獨立的 job_runs 紀錄 candle_gap_detection。
# 要解的是「evaluation_universe_sync 的 success 只代表請求沒失敗，不代表拿到該有的資料」。
export CANDLE_GAP_DETECTION_ENABLED="false"          # 驗證通過前預設關閉
export CANDLE_GAP_DETECTION_AGGREGATE_RATIO="0.5"    # 單一 (market, date) 缺漏比例 >= 此值就短路，合法 (0, 1]
export CANDLE_GAP_DETECTION_AGGREGATE_MIN_SYMBOLS="5"  # 有效池不足此數時強制逐檔，不套比例
export CANDLE_GAP_DETECTION_CANDIDATE_CAP_PER_RUN="20" # **候選數不是請求數**；請求上限＝cap × 視窗月份數
export CANDLE_GAP_DETECTION_TIMEOUT_SEC="300"        # 整輪上限，hard cap 900
export CANDLE_GAP_DETECTION_LOOKBACK_TRADING_DAYS="10" # **交易日**不是日曆天；刻意不沿用 EVALUATION_UNIVERSE_DAYS
export CANDLE_GAP_DETECTION_REQUEST_INTERVAL_MS="500"  # 對交易所節流（2 req/s）；合法 >= 100
export CANDLE_GAP_DETECTION_MARKET_STALE_DAYS="2"    # 單位是預期交易日，不是日曆日
export CANDLE_GAP_DETECTION_CALENDAR_TTL_HOURS="24"
export CANDLE_GAP_DETECTION_BREAKER_FAILURES="5"     # **來源層級**，與逐 symbol 的失敗計數是兩回事
export CANDLE_GAP_DETECTION_BREAKER_COOLDOWN_MIN="60"  # 冷卻後自動恢復

# 定期對 watchlist 產生 SR zone 分析：每交易日兩輪，17:00 與 22:00。
# 這是 production 驗證母體的唯一來源——沒有它，stock_sr_zone_analyses 只會累積人工點擊的
# 零星幾筆，todo.md T-049 前置①（新舊兩套 active 事件集合的逐日並行比對）永遠做不了。
# ⚠️ 2026-09-01 移除此處對 issue.md I-074 的引用：decision replay 的母體是 candles
# （run_decision_replay -> _load_db_sources），本排程從來不是它的前置。
#
# **前置已滿足**：事件老化早已改為依「K 棒推進」而不是「分析次數」（2026-08-20 上線），
# 所以排程與人工同日各跑一次不再讓老化一天前進 2、污染要累積的母體。
# 現況規格見 docs/sr-zone-scoring.md「老化的單位是『K 棒推進』」
#（原記於 issue.md I-077，已收斂）。
#
# 兩輪的差別只在籌碼：17:00 拿到的是前一日籌碼（FinMind 法人／融資券晚間才發布），
# 22:00 那輪晚於 21:00 的籌碼採集，才有當日的。
export SR_ANALYSIS_ENABLED="false"           # 前置已滿足（見上方），要累積分析母體就改為 "true"
export SR_ANALYSIS_CRON="0 17 * * 1-5"       # 台北時區，不含當日籌碼那輪
export SR_ANALYSIS_CHIP_CRON="0 22 * * 1-5"  # 台北時區，含當日籌碼那輪
export SR_ANALYSIS_TIMEFRAME="1d"
export SR_ANALYSIS_LIMIT="400"               # 抓取的歷史 K 棒根數

# 公司行動同步（分割／除權息／減資 → 還原係數）。**沒有 enabled 開關**：漏跑一次會讓該檔
# 整段歷史出現假跳空，重算又是冪等的，所以沒有關掉它的情境；只有時間可調。
# 06:30 早於 08:50 的 pre_market，讓當天開盤前的分析已吃到最新係數。
# 需要多跑幾次時直接設多時段（例如 "30 6,12 * * 1-5"）。打錯字會退回預設值繼續跑並記
# Error log。**不要設得比 80 小時稀疏**：/scheduler/status 的 stale 門檻寫死 80 小時，
# 稀疏排程會永遠顯示逾期（見 docs/api-reference.md）。
export CORPORATE_ACTION_CRON="30 6 * * 1-5"  # 台北時區

docker compose \
  -f "$PROJECT_DIR/docker-compose.yml" \
  --project-directory "$PROJECT_DIR" \
  down

docker compose \
  -f "$PROJECT_DIR/docker-compose.yml" \
  --project-directory "$PROJECT_DIR" \
  up --build -d

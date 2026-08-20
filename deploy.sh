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

# 定期對 watchlist 產生 SR zone 分析（T-052）：每交易日兩輪，17:00 與 22:00。
# 這是 production 驗證母體的唯一來源——沒有它，stock_sr_zone_analyses 只會累積人工點擊的
# 零星幾筆，decision replay 的分佈比較（issue.md I-074、todo.md T-049）永遠做不了。
#
# **開之前先確認 I-077 的修法已經上線**（事件老化改為依「K 棒推進」而不是「分析次數」）。
# 否則排程與人工同日各跑一次會讓老化一天前進 2，污染的正是要累積的那份母體。
#
# 兩輪的差別只在籌碼：17:00 拿到的是前一日籌碼（FinMind 法人／融資券晚間才發布），
# 22:00 那輪晚於 21:00 的籌碼採集，才有當日的。
export SR_ANALYSIS_ENABLED="false"           # 確認 I-077 已上線後改為 "true"
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

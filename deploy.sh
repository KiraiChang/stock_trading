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

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

docker compose \
  -f "$PROJECT_DIR/docker-compose.yml" \
  --project-directory "$PROJECT_DIR" \
  down

docker compose \
  -f "$PROJECT_DIR/docker-compose.yml" \
  --project-directory "$PROJECT_DIR" \
  up --build -d

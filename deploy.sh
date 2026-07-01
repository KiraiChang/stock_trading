#!/bin/bash

# 填入 git source 的絕對路徑
PROJECT_DIR="/path/to/trading"

# 填入機敏設定
export AUTH_JWT_SECRET="change-me"
export FINMIND_API_KEY="your-finmind-api-key"
export FUGLE_ENABLED="false"           # 驗證通過、確定要開通即時行情後改為 "true"
export FUGLE_API_KEY="your-fugle-api-key"

docker compose \
  -f "$PROJECT_DIR/docker-compose.yml" \
  --project-directory "$PROJECT_DIR" \
  down

docker compose \
  -f "$PROJECT_DIR/docker-compose.yml" \
  --project-directory "$PROJECT_DIR" \
  up --build -d

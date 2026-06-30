#!/bin/bash

# 填入 git source 的絕對路徑
PROJECT_DIR="/path/to/trading"

docker compose \
  -f "$PROJECT_DIR/docker-compose.yml" \
  --project-directory "$PROJECT_DIR" \
  down

docker compose \
  -f "$PROJECT_DIR/docker-compose.yml" \
  --project-directory "$PROJECT_DIR" \
  up --build -d

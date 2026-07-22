# ── Frontend build stage ──────────────────────────────────────
FROM node:20-alpine AS frontend-builder

WORKDIR /app

COPY frontend/package*.json ./frontend/
WORKDIR /app/frontend
RUN npm ci

COPY frontend ./
RUN npm run build

# ── Backend build stage ───────────────────────────────────────
FROM golang:1.25-alpine AS backend-builder

# 低記憶體編譯：開發／部署主機只有 2GiB RAM（實際可用約 700MB），預設設定會在
# redis/go-redis、modernc.org/libc 這類大型依賴的 compile 階段被 OOM killer 砍掉
# （signal: killed）。實測光是序列化（-p=1 / GOMAXPROCS=1）還不夠，要再用
# GOGC=off + GOMEMLIMIT 壓住編譯器堆積上限才過得了冷 cache 的完整 build；
# GOMEMLIMIT=500MiB 仍失敗，250MiB 可穩定編完（冷 cache 約 3 分鐘）。
# 這些只影響編譯過程，不影響產出的執行檔。
ENV GOFLAGS=-p=1 \
    GOMAXPROCS=1 \
    GOGC=off \
    GOMEMLIMIT=250MiB

WORKDIR /app/backend

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend ./
COPY --from=frontend-builder /app/backend/internal/ui/dist ./internal/ui/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /trading-backend ./cmd/server

# ── Run stage ─────────────────────────────────────────────────
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=backend-builder /trading-backend .
COPY backend/config.yaml .

EXPOSE 8080
CMD ["./trading-backend"]

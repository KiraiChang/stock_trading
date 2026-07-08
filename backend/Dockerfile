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

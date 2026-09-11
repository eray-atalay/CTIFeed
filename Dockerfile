# Multi-stage Dockerfile (Astro Frontend + Go Backend + Alpine Runner)

# Stage 1: Build Astro frontend
FROM node:22-alpine AS frontend-builder
WORKDIR /app/frontend

COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install

COPY frontend/ ./
RUN npm run build

# Stage 2: Build Go binary with embedded static assets
FROM golang:alpine AS backend-builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY web/ ./web/

# Copy compiled frontend assets into web/dist
COPY --from=frontend-builder /app/frontend/dist/ ./web/dist/

# Compile pure Go binary (CGO-free)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/ctifeed ./cmd/ctifeed

# Stage 3: Minimal runtime image
FROM alpine:3.20 AS runner

RUN apk add --no-cache ca-certificates tzdata wget

RUN addgroup -S ctigroup && adduser -S ctiuser -G ctigroup && \
    mkdir -p /app /data && \
    chown -R ctiuser:ctigroup /app /data

COPY --from=backend-builder /bin/ctifeed /app/ctifeed
RUN chmod +x /app/ctifeed

USER ctiuser
WORKDIR /app

EXPOSE 8080

ENV PORT=8080 \
    DB_PATH=/data/ctifeed.db \
    INTERVAL=15m \
    WORKERS=5 \
    TIMEOUT=10s

VOLUME ["/data"]

ENTRYPOINT ["/app/ctifeed"]

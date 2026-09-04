# ==============================================================================
# CTIFeed Cok Asamali (Multi-stage) Dockerfile (Astro Frontend + Go Backend + Alpine Runner)
# ==============================================================================

# ------------------------------------------------------------------------------
# Asama 1: Astro On Yuzunun Derlenmesi
# ------------------------------------------------------------------------------
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend

# Bagimliliklari yukle
COPY frontend/package.json ./
RUN npm install

# Astro statik uretim varliklarini olustur
COPY frontend/ ./
RUN npm run build

# ------------------------------------------------------------------------------
# Asama 2: Gomulu Varliklar ile Go Arka Yuzunun Derlenmesi
# ------------------------------------------------------------------------------
FROM golang:alpine AS backend-builder
WORKDIR /app

# Go modul bagimliliklarini indir
COPY go.mod go.sum ./
RUN go mod download

# Arka yuz kaynak kodlarini kopyala
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY web/ ./web/

# Yeni derlenen Astro statik varliklarini web dizinine aktar
COPY --from=frontend-builder /app/frontend/dist/ ./web/

# Bagimsiz statik Go ikili dosyasini derle (CGO gerektirmez)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/ctifeed ./cmd/ctifeed

# ------------------------------------------------------------------------------
# Asama 3: Minimal ve Guvenli Alpine Uretim Calistiricisi
# ------------------------------------------------------------------------------
FROM alpine:3.20 AS runner

# HTTPS RSS beslemeleri icin kok sertifikalari ve saat dilimi destegini yukle
RUN apk add --no-cache ca-certificates tzdata wget

# Root olmayan sistem kullanicisi ve kalici veri dizini olustur
RUN addgroup -S ctigroup && adduser -S ctiuser -G ctigroup && \
    mkdir -p /app /data && \
    chown -R ctiuser:ctigroup /app /data

# Derleme asamasindan ikili dosyayi kopyala
COPY --from=backend-builder /bin/ctifeed /app/ctifeed
RUN chmod +x /app/ctifeed

USER ctiuser
WORKDIR /app

# Web Arayuzu portunu disari ac
EXPOSE 8080

# Varsayilan ortam degiskenleri
ENV PORT=8080 \
    DB_PATH=/data/ctifeed.db \
    INTERVAL=15m \
    WORKERS=5 \
    TIMEOUT=10s

# SQLite veritabani kaliciligi icin baglanti noktasi (volume)
VOLUME ["/data"]

ENTRYPOINT ["/app/ctifeed"]

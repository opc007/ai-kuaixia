# AI快侠后端 - 用于 Render / Railway / 阿里云 等 Docker 部署
FROM golang:1.23-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git gcc musl-dev

COPY go.mod go.sum ./
ENV GOTOOLCHAIN=local
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/

# SQLite 需要 CGO
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o main ./cmd/server

FROM alpine:3.19

WORKDIR /app

RUN apk add --no-cache \
    ca-certificates tzdata ffmpeg python3 py3-pip \
    && pip3 install --break-system-packages --no-cache-dir yt-dlp \
    && yt-dlp --version

ENV TZ=Asia/Shanghai
ENV DB_DRIVER=sqlite
ENV DB_PATH=/app/data/aikuaixia.db
ENV PORT=8080
ENV GIN_MODE=release

RUN mkdir -p /app/data storage

COPY --from=builder /app/main .

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s \
  CMD wget -qO- http://localhost:8080/api/v1/platforms || exit 1

CMD ["./main"]

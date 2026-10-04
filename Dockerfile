# Сборка приложения.
#
# Секреты в образ не попадают: на этапе сборки копируется только код, а
# конфигурация приходит извне через переменные окружения или docker-compose.

FROM golang:1.24-alpine AS builder

WORKDIR /src

# Сначала копируем только манифесты: слой с зависимостями переиспользуется,
# пока не меняются версии.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO отключён, чтобы получить статический бинарник, который работает в
# пустом alpine-образе.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/server ./cmd/server

FROM alpine:3.20

# ca-certificates нужен для HTTPS-соединения с OpenRouter.
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 app

WORKDIR /app

COPY --from=builder /out/server /app/server
COPY --from=builder /src/web /app/web
COPY --from=builder /src/db /app/db

USER app

EXPOSE 8080

ENTRYPOINT ["/app/server"]
# Стадия сборки
FROM golang:1.22-alpine AS builder

# Установка зависимостей для сборки
RUN apk add --no-cache git

# Создание рабочей директории
WORKDIR /app

# Копирование go.mod и go.sum для скачивания зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копирование исходного кода
COPY . .

# Сборка приложения с отключенными CGO и оптимизацией для alpine
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o scheduler ./cmd/scheduler

# Стадия создания финального образа
FROM alpine:3.19

# Установка необходимых пакетов и сертификатов
RUN apk --no-cache add ca-certificates tzdata && \
    update-ca-certificates

# Создание непривилегированного пользователя
RUN adduser -D -H -h /app appuser

# Копирование бинарного файла из стадии сборки
COPY --from=builder /app/scheduler /app/scheduler

# Копирование конфигурационных файлов
COPY --from=builder /app/configs /app/configs

# Указание рабочего каталога
WORKDIR /app

# Переключение на непривилегированного пользователя
USER appuser

# Entrypoint
ENTRYPOINT ["/app/scheduler"]
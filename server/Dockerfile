FROM golang:1.24-alpine AS builder

WORKDIR /app

# Копируем файлы go.mod и go.sum
COPY go.mod ./
# Если есть go.sum, раскомментируйте следующую строку
# COPY go.sum ./

# Предварительная загрузка зависимостей
RUN go mod download

# Копируем исходный код
COPY . .

# Компилируем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -o server .

# Создаем минимальный образ
FROM alpine:latest

WORKDIR /app

# Копируем исполняемый файл из стадии сборки
COPY --from=builder /app/server /app/

# Создаем директорию для записей
RUN mkdir -p /app/recordings

# Устанавливаем разрешения
RUN chmod +x /app/server

# Определяем порт, который должен быть открыт
EXPOSE 8080

# Директория для хранения записей (можно монтировать как том)
VOLUME ["/app/recordings"]

# Запускаем приложение
CMD ["/app/server", "--port", "8080", "--output", "/app/recordings"] 
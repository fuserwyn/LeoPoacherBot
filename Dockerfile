# Используем официальный образ Go для сборки
FROM golang:1.21 AS builder

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем файлы зависимостей
COPY go.mod go.sum ./

# Скачиваем зависимости
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/bot

# Финальный образ - используем distroless для минимального размера
FROM gcr.io/distroless/static-debian11

# Копируем бинарный файл
COPY --from=builder /app/main /app/main

# Устанавливаем рабочую директорию
WORKDIR /app

# Открываем порт
EXPOSE 8080

# Запускаем приложение
CMD ["/app/main"]

# Используем только официальный образ Go
FROM golang:1.21

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

# Открываем порт (если нужно)
EXPOSE 8080

# Запускаем приложение
CMD ["./main"]

# Простой Dockerfile для Railway
FROM golang:1.21

# Устанавливаем рабочую директорию
WORKDIR /app

# Проверяем содержимое директории
RUN pwd && ls -la

# Копируем все файлы
COPY . .

# Проверяем, что файлы скопировались
RUN echo "=== After COPY ===" && ls -la && echo "=== go.mod content ===" && cat go.mod

# Проверяем go.sum
RUN echo "=== go.sum content ===" && cat go.sum

# Скачиваем зависимости
RUN go mod download

# Проверяем модули
RUN echo "=== Go modules ===" && go list -m all

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/bot

# Открываем порт
EXPOSE 8080

# Запускаем приложение
CMD ["./main"]

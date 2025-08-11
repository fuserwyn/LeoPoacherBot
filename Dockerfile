# Простой Dockerfile для Railway
FROM golang:1.21

# Устанавливаем рабочую директорию
WORKDIR /app

# Проверяем содержимое директории
RUN pwd && ls -la

# Копируем все файлы
COPY . .

# Проверяем, что файлы скопировались
RUN echo "=== After COPY ===" && ls -la && echo "=== Current directory ===" && pwd

# Проверяем go.mod
RUN echo "=== go.mod content ===" && cat go.mod

# Проверяем go.sum
RUN echo "=== go.sum content ===" && ls -la go.sum && cat go.sum

# Проверяем все .go файлы
RUN echo "=== Go files ===" && find . -name "*.go" -type f

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

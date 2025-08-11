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

# Скачиваем зависимости (принудительно)
RUN go mod download -x

# Проверяем модули
RUN echo "=== Go modules ===" && go list -m all

# Проверяем, что go.sum создался
RUN echo "=== Checking go.sum ===" && ls -la go.sum* 2>/dev/null || echo "go.sum not found, but continuing..."

# Принудительно создаем go.sum если его нет
RUN go mod verify || echo "go.mod verification failed, but continuing..."

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/bot

# Открываем порт
EXPOSE 8080

# Запускаем приложение
CMD ["./main"]

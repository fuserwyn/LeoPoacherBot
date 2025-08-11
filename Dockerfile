# Минимальный образ с базовыми системными библиотеками
FROM debian:bullseye-slim

# Устанавливаем только необходимые пакеты
RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Создаем пользователя для безопасности
RUN groupadd -g 1001 appgroup && \
    useradd -u 1001 -g appgroup -s /bin/false appuser

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем предварительно собранный бинарный файл
COPY bin/leo-bot /app/main

# Делаем файл исполняемым
RUN chmod +x /app/main

# Меняем владельца файла
RUN chown -R appuser:appgroup /app

# Переключаемся на непривилегированного пользователя
USER appuser

# Открываем порт
EXPOSE 8080

# Запускаем приложение
CMD ["/app/main"]

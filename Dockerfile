# Минимальный Dockerfile для Railway
FROM scratch

# Копируем предварительно собранный бинарный файл
COPY bin/leo-bot /app/main

# Устанавливаем рабочую директорию
WORKDIR /app

# Открываем порт
EXPOSE 8080

# Запускаем приложение
CMD ["/app/main"]

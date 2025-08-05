.PHONY: build run test clean docker-build docker-run

# Сборка приложения
build:
	go build -o bin/leo-bot ./cmd/bot

# Запуск приложения
run: build
	./bin/leo-bot

# Тесты
test:
	go test ./...

# Очистка
clean:
	rm -rf bin/
	go clean

# Сборка Docker образа
docker-build:
	docker build -t leo-bot .

# Запуск с Docker Compose
docker-run:
	docker-compose up --build

# Остановка Docker Compose
docker-stop:
	docker-compose down

# Запуск в фоновом режиме
docker-run-detached:
	docker-compose up -d --build

# Просмотр логов
docker-logs:
	docker-compose logs -f

# Подключение к базе данных
db-connect:
	docker-compose exec postgres psql -U postgres -d leo_bot_db

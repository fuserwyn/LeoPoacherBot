# LeoPoacherBot - Telegram Bot на Go

Telegram бот для отслеживания тренировок участников чата. Бот автоматически запускает таймеры для всех участников и удаляет неактивных пользователей.

## 🚀 Возможности

- ⏰ **Автоматические таймеры** - 7 дней на отправку отчета
- 🏥 **Система больничных** - приостановка таймера при болезни
- 💪 **Отслеживание тренировок** - хештег `#training_done`
- 🦁 **Fat Leopard** - уникальная персона бота
- 📊 **Статистика** - просмотр данных в базе
- 🔧 **Административные команды** - управление ботом

## 🛠 Технологии

- **Go 1.21** - основной язык
- **PostgreSQL** - база данных
- **Telegram Bot API** - взаимодействие с Telegram
- **Docker** - контейнеризация
- **Docker Compose** - оркестрация

## 📋 Требования

- Go 1.21+
- Docker и Docker Compose
- PostgreSQL (или Docker для локальной разработки)

## 🚀 Быстрый старт

### 1. Клонирование репозитория
```bash
git clone <repository-url>
cd LeoPoacherBot
```

### 2. Настройка переменных окружения
```bash
cp env_template.txt .env
```

Отредактируйте `.env` файл:
```env
API_TOKEN=your_bot_token_here
OWNER_ID=your_telegram_id_here
DATABASE_URL=postgresql://postgres:password@localhost:5432/leo_bot_db?sslmode=disable
LOG_LEVEL=info
```

### 3. Запуск с Docker Compose
```bash
make docker-run
```

### 4. Локальная разработка
```bash
# Установка зависимостей
go mod download

# Сборка и запуск
make run
```

## 📖 Команды

### Для пользователей:
- `#training_done` - отправить отчет о тренировке
- `#sick_leave` - взять больничный
- `#healthy` - выздороветь и возобновить таймер
- `/help` - показать справку

### Для администраторов:
- `/start_timer` - запустить таймеры для всех пользователей
- `/db` - показать статистику базы данных
- `/help` - показать справку

## ⏰ Как работает бот

1. **При добавлении в чат** - автоматически запускаются таймеры для всех участников
2. **7 дней** - время на отправку отчета о тренировке
3. **6 дней** - предупреждение от Fat Leopard
4. **7 дней** - удаление из чата за неактивность
5. **#training_done** - перезапускает таймер на 7 дней
6. **#sick_leave** - приостанавливает таймер
7. **#healthy** - возобновляет таймер с места остановки

## 🏗 Структура проекта

```
LeoPoacherBot/
├── cmd/
│   └── bot/
│       └── main.go          # Точка входа приложения
├── internal/
│   ├── bot/
│   │   └── bot.go          # Основная логика бота
│   ├── config/
│   │   └── config.go       # Конфигурация
│   ├── database/
│   │   └── database.go     # Работа с базой данных
│   ├── logger/
│   │   └── logger.go       # Логирование
│   └── models/
│       └── models.go       # Модели данных
├── Dockerfile              # Docker образ
├── docker-compose.yml      # Docker Compose
├── go.mod                  # Зависимости Go
├── Makefile                # Команды для сборки
└── README.md              # Документация
```

## 🔧 Разработка

### Сборка
```bash
make build
```

### Тесты
```bash
make test
```

### Очистка
```bash
make clean
```

### Docker команды
```bash
make docker-build    # Сборка образа
make docker-run      # Запуск с Docker Compose
make docker-stop     # Остановка
make docker-logs     # Просмотр логов
```

## 📊 База данных

Бот использует PostgreSQL с двумя основными таблицами:

### message_log
- `user_id` - ID пользователя
- `chat_id` - ID чата
- `last_message` - время последнего сообщения
- `has_training_done` - есть ли отчет о тренировке
- `has_sick_leave` - находится ли на больничном
- `has_healthy` - выздоровел ли
- `timer_start_time` - время начала таймера
- `sick_leave_start_time` - время начала больничного

### training_log
- `user_id` - ID пользователя
- `last_report` - время последнего отчета
- `created_at` - время создания записи
- `updated_at` - время обновления записи

## 🦁 Fat Leopard

Бот имеет уникальную персону "Fat Leopard" (Толстый Леопард), который:
- Питается ленивыми леопардами
- Становится жирнее от лени
- Предупреждает пользователей о последствиях лени
- Мотивирует к тренировкам

## 🔒 Безопасность

- Использование непривилегированного пользователя в Docker
- Переменные окружения для конфиденциальных данных
- Проверка прав администратора для команд
- Graceful shutdown приложения

## 🐛 Устранение неполадок

### Проблемы с подключением к базе данных
```bash
# Проверка статуса PostgreSQL
docker-compose ps postgres

# Просмотр логов
docker-compose logs postgres
```

### Проблемы с ботом
```bash
# Просмотр логов бота
docker-compose logs bot

# Перезапуск
docker-compose restart bot
```

### Проблемы с правами
- Убедитесь, что бот добавлен как администратор в чат
- Проверьте правильность OWNER_ID в .env файле

## 📝 Лицензия

MIT License

## 🤝 Вклад в проект

1. Fork репозитория
2. Создайте feature branch (`git checkout -b feature/amazing-feature`)
3. Commit изменения (`git commit -m 'Add amazing feature'`)
4. Push в branch (`git push origin feature/amazing-feature`)
5. Откройте Pull Request 
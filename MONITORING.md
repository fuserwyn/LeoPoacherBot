# Мониторинг на Railway

Интеграция Prometheus + Grafana для мониторинга бота, платёжного сервиса и PostgreSQL.

## Локальное тестирование

```bash
docker compose up --build
```

Доступ:
- **Grafana**: http://localhost:3001 (логин: `admin`, пароль: `admin`)
- **Prometheus**: http://localhost:9090
- **Бот API**: http://localhost:8080
- **Метрики бота**: http://localhost:8080/metrics

## Развёртывание на Railway

### Вариант 1: Через Railway Dashboard (простой)

1. Создайте Railway проект
2. Добавьте сервисы:
   - **PostgreSQL** — встроенный в Railway
   - **Bot** — из GitHub репозитория (указать этот проект)
   - **Prometheus** — Docker образ `prom/prometheus:latest`
   - **Grafana** — из `docker/grafana/Dockerfile`

3. Установите env переменные для каждого сервиса (см. ниже)

### Вариант 2: Через GitHub Actions (рекомендуется)

1. Создайте Railway проект
2. Подключите GitHub репозиторий
3. Railway автоматически прочитает `railway.json` и развернёт сервис

### Переменные окружения

**Bot сервис (ms_leo):**
```env
FAT_LEOPARD_API_TOKEN=<реальный токен от @BotFather>
DATABASE_URL=postgresql://...
OPENROUTER_API_KEY=<если нужен>
LOG_LEVEL=info
PORT=8080
YOOKASSA_SHOP_ID=<если платежи>
YOOKASSA_SECRET_KEY=<если платежи>
```

**Prometheus:**
- Конфиг автоматически загружается из `docker/prometheus/prometheus.yml`
- На Railway обновите targets с реальными доменами сервисов

**Grafana:**
```env
GF_SECURITY_ADMIN_PASSWORD=<сильный пароль>
GF_SECURITY_ADMIN_USER=admin
```

## Структура метрик

Бот экспортирует:
- `http_requests_total` — количество HTTP запросов (по методу, эндпоинту, статусу)
- `http_request_duration_seconds` — латенция HTTP запросов
- `bot_updates_received_total` — Telegram обновления
- `bot_errors_total` — ошибки бота
- `database_query_duration_seconds` — время БД запросов
- `database_errors_total` — ошибки БД
- `payment_requests_total` — запросы платежей

## Дашборды

После запуска Grafana автоматически загружаются:
- **Leo Bot Metrics** — основной дашборд с HTTP, ошибками, обновлениями бота

Можете добавить новые дашборды прямо в Grafana UI.

## Продвинутые настройки

### PostgreSQL метрики

postgres-exporter уже включён в `docker-compose.yml`. На Railway используйте образ:
```
prometheuscommunity/postgres-exporter:latest
```

### Алерты

Создайте alert rules в Prometheus:
```yaml
groups:
  - name: leo-bot
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"[45..}[5m]) > 0.05
        for: 5m
        annotations:
          summary: "High error rate detected"
```

## Логирование

Для логов (помимо метрик) можно добавить:
- **Loki** — логгер от Grafana
- **ELK stack** — Elasticsearch + Logstash + Kibana

Текущий setup используется `logrus` в коде бота.

## Команды для Railway CLI

```bash
# Развернуть проект
railway up

# Посмотреть логи
railway logs

# SSH в контейнер
railway shell

# Посмотреть переменные
railway env
```

## Обслуживание

1. **Очистка метрик** — Prometheus хранит данные в `/prometheus`. На Railway это дешево, но можно сократить TTL в `prometheus.yml`
2. **Резервная копия Grafana** — периодически экспортируйте дашборды через UI
3. **Масштабирование** — если метрик слишком много, используйте Prometheus remote storage (например, на S3)

# Подсчёт тренировок по `#training_done` (полная история чата)

Бот видит только новые сообщения — историю чата Telegram Bot API не отдаёт.
Поэтому, чтобы пересчитать `#training_done` **с самого начала**, читаем чат
под своим обычным аккаунтом через MTProto (Telethon).

## 1. Получить api_id / api_hash

1. Открой https://my.telegram.org → **API development tools**.
2. Создай приложение (любое название). Получишь `api_id` (число) и `api_hash` (строку).

## 2. Установить зависимости

```bash
cd tools/tg_training_parser
python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
```

## 3. Запустить

```bash
export TG_API_ID=1234567
export TG_API_HASH=ваш_api_hash
export TG_PHONE=+79991234567
export TG_CHAT=-1003246054143      # id чата (или @username / ссылка)
python count_trainings.py
```

При первом запуске Telegram пришлёт **код подтверждения** — введи его в консоли
(если включена 2FA — ещё и облачный пароль). Создастся файл сессии
`leo_training_session.session`, чтобы не логиниться каждый раз.

## Результат

- В консоль — таблица: по каждому юзеру `сообщений с тегом` и `активных дней`
  (уникальные дни по МСК ≈ «одна тренировка в день»).
- `trainings.csv` — та же сводка.
- `messages_dump.csv` — все помеченные сообщения для ручной проверки.

## Заметки

- **`tag_msgs` vs `active_days`.** Если человек писал `#training_done` дважды
  в день, `tag_msgs` посчитает оба, `active_days` — один. Бот в `training_sessions`
  допускает несколько в день, так что ближе к нему — `tag_msgs`. Для «по дням»
  смотри `active_days`.
- **Не клади `*.session` в git** — это полноценный доступ к твоему аккаунту.
  Добавь в `.gitignore`: `tools/tg_training_parser/*.session`.
- Тег настраивается через `TG_TAG` (по умолчанию `#training_done`).

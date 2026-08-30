# leo-tracker MCP

MCP-сервер доски Fat Leopard: даёт агенту (Claude Code) прямой доступ к очереди
`ms_tracker` и к карточкам доски `ms_leo`. Зависимостей нет — только Node 18+.

## Зачем

Встроенный «агент» трекера (`ms_tracker/internal/agent`) — это один запрос в
OpenRouter, который просит модель вернуть **полный текст каждого файла** JSON-ом.
Оттуда и `Агент не стартовал: unexpected end of JSON input` (пустое тело ответа
в `chatRaw`), и вырезанные куски файлов. Здесь наоборот: очередь и ветки читает
MCP, а код правит агент обычными файловыми инструментами в клоне репозитория.

## Настройка

1. Скопировать `Fat-Leopard/.env.tracker-mcp.example` → `Fat-Leopard/.env.tracker-mcp`
   и подставить значения из Railway (сервис `ms_tracker`). Файл гитигнорится.
2. Сервер уже прописан в `.mcp.json` в корне рабочей папки. Перезапустить Claude Code,
   подтвердить подключение, проверить `/mcp`.
3. Позвать `tracker_health` — он печатает, что подхватилось и жив ли сервис.

Переменные (реальное окружение сильнее файла):
`TRACKER_URL`, `TRACKER_SECRET` (или `BOARD_SSO_SECRET`), `LEO_NOTIFY_URL`,
`NOTIFY_SECRET`, `BOARD_REPO`, `BOARD_BRANCH`.

## Дымовой прогон против живого сервера

```bash
node Fat-Leopard/tools/leo-tracker-mcp/smoke.mjs                 # берёт .env.tracker-mcp
node Fat-Leopard/tools/leo-tracker-mcp/smoke.mjs --from-railway  # тянет переменные из Railway
```

`--from-railway` сам вызывает `railway variables --json` из каталога, где есть
привязка проекта (`Fat-Leopard/ms_tracker`, затем `Fat-Leopard`) — из корня рабочей
папки CLI отвечает «No linked project found». Найденные доступы он кладёт в
`Fat-Leopard/.env.tracker-mcp` (права 600, перезапись — флагом `--write-env`),
после чего MCP-сервер в Claude Code работает уже без railway.

Поднимает сервер настоящим MCP-клиентом и зовёт только читающие инструменты:
handshake → `tracker_health` → очередь → упавшие карточки → ветка первой упавшей.
Ничего не пишет и не мержит. `--from-railway` требует слинкованного проекта
(`railway link`) и берёт переменные сервиса `ms_tracker` (или `RAILWAY_SERVICE`).

## Инструменты

| Инструмент | Что делает |
| --- | --- |
| `tracker_health` | Конфиг сервера + `GET /health` ms_tracker. Первое при «доска не работает». |
| `tracker_jobs` | Очередь задач, фильтры `status` и `only_failed`. |
| `tracker_job` | Одна задача целиком: промпт, шаги, ошибка, ветка. |
| `tracker_schedule` | Поставить задачу встроенному агенту (он же пишет файлы целиком — обычно не нужно). |
| `tracker_cancel` | Снять задачу с очереди. |
| `tracker_inspect_branch` | Ветка задачи против main: HEAD, файлы, есть ли правки приложения. |
| `tracker_stamp` | Коммит фазы (ревью/тест) на ветке задачи. |
| `tracker_report` | Написать в карточку доски (HMAC-подпись, как у `notify.makeToken`). Снимает зависшие карточки. |
| `tracker_ship` | **Мерж ветки в `main`** → автодеплой прода. Требует `confirm: "ship"`. |

## Рабочий цикл руками

1. `tracker_jobs` с `only_failed: true` — найти зависшие карточки.
2. `tracker_job` — прочитать задачу, `tracker_inspect_branch` — что уже на ветке.
3. Правки делать в клоне репозитория обычными инструментами, ветка `tracker/<num>-<job>`.
4. `tracker_report` — отчитаться в карточку.
5. `tracker_ship` — только по прямой просьбе человека и после проверки ветки.

## Протокол

stdio, JSON-RPC 2.0 построчно. `initialize` согласует версию из
`2025-06-18 / 2025-03-26 / 2024-11-05`, объявляется только capability `tools`.
Пустое тело и не-JSON ответа приходят понятной ошибкой, а не паникой парсера.

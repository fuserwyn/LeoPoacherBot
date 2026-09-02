# Задача трекера #45

Задача #45.

дублируются уведомления при выкатке на стенде

## выполнение

Сделано.

**Причина:** при выкатке на стенд «задача выполнена» уходила дважды — из вебхука доски (`handleBoardNotify` → `NotifyTrackerShippedIfNeeded`) и из `finishTrackerBuild` после SUCCESS Railway. Шаг «уведомили о выкате» не всегда спасал из‑за гонки.

**Правки:**
1. `ms_leo/internal/miniappapi/admin.go` — убран вызов `NotifyTrackerShippedIfNeeded`; финальный DM только из `finishTrackerBuild`.
2. `ms_leo/internal/bot/tracker_notify.go` — добавлена per-task блокировка `trackerShipNotifyInflight`, чтобы параллельные вызовы `notifyTrackerShippedOnce` не слали второе сообщение.

Локальный коммит не создан: в окружении не настроен git user. Изменения в рабочей копии на ветке `tracker/45-502`.

## ревью

Ревью: на ветке tracker/45-502 config.go целый. Можно на тест.

## тест

Тест: config.go целый, ветка tracker/45-502. Тест пройден.

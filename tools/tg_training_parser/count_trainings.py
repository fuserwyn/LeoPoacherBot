#!/usr/bin/env python3
"""
Считает тренировки по тегу #training_done во всём чате, читая историю
через пользовательский аккаунт (Telethon / MTProto).

Боту история чата недоступна (Bot API отдаёт только новые сообщения),
поэтому для полного пересчёта нужен вход под обычным аккаунтом.

Что делает:
  1. Логинится под твоим телеграм-аккаунтом (api_id/api_hash с my.telegram.org).
  2. Проходит ВСЮ историю чата.
  3. Берёт сообщения (и текст, и подписи к фото), где встречается #training_done.
  4. Группирует по автору и выводит две метрики:
       - tag_msgs   — сколько всего сообщений с тегом (как написали в чат);
       - active_days — по скольким разным дням (МСК) есть тег
                       (≈ «одна тренировка в день», как считает streak бота).
  5. Сохраняет результат в trainings.csv и полный дамп помеченных
     сообщений в messages_dump.csv (для ручной проверки).

Запуск:
    pip install -r requirements.txt
    # заполни значения ниже (или экспортни переменные окружения)
    python count_trainings.py
"""

import asyncio
import csv
import os
import re
from collections import defaultdict
from datetime import timezone, timedelta
from zoneinfo import ZoneInfo

from telethon import TelegramClient
from telethon.tl.types import PeerChannel


def load_env_file():
    """Подхватывает Fat-Leopard/.env (на 3 уровня выше) в os.environ,
    не перетирая уже заданные переменные. Без внешних зависимостей."""
    here = os.path.dirname(os.path.abspath(__file__))
    env_path = os.path.normpath(os.path.join(here, "..", "..", ".env"))
    if not os.path.exists(env_path):
        return
    with open(env_path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            key, _, value = line.partition("=")
            key = key.strip()
            value = value.strip().strip('"').strip("'")
            os.environ.setdefault(key, value)


load_env_file()


def _env(*names, default=""):
    """Возвращает первое заданное значение из перечисленных имён
    (без учёта регистра ключа: API_ID == api_id)."""
    for n in names:
        for variant in (n, n.upper(), n.lower()):
            v = os.getenv(variant)
            if v:
                return v
    return default


# ----------------------------------------------------------------------------
# НАСТРОЙКИ — берутся из Fat-Leopard/.env (API_ID/API_HASH) или из окружения
# ----------------------------------------------------------------------------
# api_id / api_hash берутся на https://my.telegram.org -> API development tools
API_ID = int(_env("API_ID", "TG_API_ID", default="0"))   # напр. 1234567
API_HASH = _env("API_HASH", "TG_API_HASH")               # напр. "abcd1234..."
# Телефон можно не указывать — Telethon спросит его интерактивно при первом входе
PHONE = _env("TG_PHONE", "PHONE")                        # напр. "+79991234567"

# Чат, который читаем. Поддерживаются варианты:
#   - числовой id супергруппы вида -1003246054143
#   - @username чата
#   - ссылка-приглашение/публичная ссылка t.me/...
CHAT = _env("TG_CHAT", default="-1003246054143")

# Тег, по которому считаем (без учёта регистра)
TAG = _env("TG_TAG", default="#training_done")

# Часовой пояс для группировки «по дням» (бот живёт по Москве)
TZ = ZoneInfo("Europe/Moscow")

SESSION_NAME = "leo_training_session"  # файл сессии Telethon (создаётся локально)
# ----------------------------------------------------------------------------


def resolve_chat(value: str):
    """Преобразует строку чата в сущность, понятную Telethon."""
    value = value.strip()
    # Числовой id супергруппы/канала: -100XXXXXXXXXX
    if re.fullmatch(r"-100\d+", value):
        channel_id = int(value[4:])  # убираем префикс -100
        return PeerChannel(channel_id)
    # Просто отрицательный id
    if re.fullmatch(r"-?\d+", value):
        return int(value)
    # @username или ссылка
    return value


def tag_pattern(tag: str) -> re.Pattern:
    # \B перед # чтобы #training_done не цеплялся внутри другого слова,
    # граница в конце — чтобы #training_done_extra не считался тем же тегом.
    return re.compile(re.escape(tag) + r"\b", re.IGNORECASE)


async def main():
    if not API_ID or not API_HASH:
        raise SystemExit(
            "Заполни TG_API_ID и TG_API_HASH (см. my.telegram.org) "
            "вверху файла или через переменные окружения."
        )

    pat = tag_pattern(TAG)
    client = TelegramClient(SESSION_NAME, API_ID, API_HASH)
    await client.start(phone=PHONE or None)

    entity = await client.get_entity(resolve_chat(CHAT))
    title = getattr(entity, "title", str(CHAT))
    print(f"Читаю историю чата: {title}")

    # Кэш отображаемых имён, чтобы не дёргать API на каждое сообщение
    name_cache: dict[int, str] = {}

    def display_name(sender) -> str:
        if sender is None:
            return "(unknown)"
        uid = getattr(sender, "id", 0)
        if uid in name_cache:
            return name_cache[uid]
        username = getattr(sender, "username", None)
        if username:
            name = "@" + username
        else:
            first = getattr(sender, "first_name", "") or ""
            last = getattr(sender, "last_name", "") or ""
            name = (first + " " + last).strip() or str(uid)
        name_cache[uid] = name
        return name

    tag_msgs: dict[int, int] = defaultdict(int)          # всего сообщений с тегом
    days: dict[int, set] = defaultdict(set)              # уникальные дни (МСК)
    uid_to_name: dict[int, str] = {}
    dump_rows = []                                        # для messages_dump.csv

    total = 0
    matched = 0
    async for msg in client.iter_messages(entity):
        total += 1
        if total % 2000 == 0:
            print(f"  ...просмотрено {total} сообщений, найдено {matched} с тегом")

        text = msg.message or ""  # для медиа здесь лежит подпись (caption)
        if not text or not pat.search(text):
            continue

        sender = await msg.get_sender()
        uid = getattr(sender, "id", 0)
        name = display_name(sender)
        uid_to_name[uid] = name

        # Дата сообщения в МСК
        dt_msk = msg.date.astimezone(TZ)
        day = dt_msk.date().isoformat()

        tag_msgs[uid] += 1
        days[uid].add(day)
        matched += 1

        dump_rows.append({
            "date_msk": dt_msk.strftime("%Y-%m-%d %H:%M"),
            "user": name,
            "user_id": uid,
            "text": " ".join(text.split())[:200],
        })

    await client.disconnect()

    # --- Сводка ---
    rows = []
    for uid in tag_msgs:
        rows.append((uid_to_name.get(uid, str(uid)), tag_msgs[uid], len(days[uid])))
    rows.sort(key=lambda r: (-r[1], -r[2]))

    print("\n==== ТРЕНИРОВКИ ПО ТЕГУ {} ====".format(TAG))
    print(f"{'Юзер':<22}{'сообщ. с тегом':>15}{'активных дней':>15}")
    print("-" * 52)
    for name, n, d in rows:
        print(f"{name:<22}{n:>15}{d:>15}")
    print("-" * 52)
    print(f"{'ИТОГО':<22}{sum(r[1] for r in rows):>15}{'':>15}")
    print(f"\nВсего просмотрено сообщений: {total}, с тегом: {matched}")

    # --- CSV ---
    out_dir = os.path.dirname(os.path.abspath(__file__))
    summary_path = os.path.join(out_dir, "trainings.csv")
    with open(summary_path, "w", newline="", encoding="utf-8") as f:
        w = csv.writer(f)
        w.writerow(["user", "user_id", "tag_msgs", "active_days"])
        for uid in sorted(tag_msgs, key=lambda u: -tag_msgs[u]):
            w.writerow([uid_to_name.get(uid, str(uid)), uid, tag_msgs[uid], len(days[uid])])

    dump_path = os.path.join(out_dir, "messages_dump.csv")
    with open(dump_path, "w", newline="", encoding="utf-8") as f:
        w = csv.DictWriter(f, fieldnames=["date_msk", "user", "user_id", "text"])
        w.writeheader()
        # от старых к новым
        for row in reversed(dump_rows):
            w.writerow(row)

    print(f"\nСохранено: {summary_path}")
    print(f"Сохранено: {dump_path}")


if __name__ == "__main__":
    asyncio.run(main())

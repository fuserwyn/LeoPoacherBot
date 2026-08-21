#!/usr/bin/env python3
"""
Считает тренировки по тегу #training_done из ЭКСПОРТА чата Telegram Desktop.

Способ без api_id и без кода входа: Telegram Desktop умеет выгрузить всю
историю чата в JSON. Этот скрипт читает result.json и группирует #training_done
по авторам.

Как получить result.json:
  1. Telegram Desktop -> открой нужный чат.
  2. Меню (⋮ вверху справа) -> "Export chat history".
  3. Сними галочки с фото/видео/файлов (нужен только текст -> быстрее и легче).
     Format: обязательно "Machine-readable JSON" (НЕ HTML!).
  4. Export -> дождись, открой папку -> там result.json.

Запуск:
    python parse_export.py /путь/до/result.json
    # или просто положи result.json рядом со скриптом и запусти без аргумента
"""

import csv
import json
import os
import re
import sys
from collections import defaultdict

TAG = os.getenv("TG_TAG", "#training_done")


def extract_text(message: dict) -> str:
    """Telegram-экспорт хранит текст как строку ИЛИ как список кусочков
    (строки + объекты-сущности вроде {'type':'hashtag','text':'#training_done'}).
    Склеиваем всё в одну строку. Учитываем и подпись к медиа."""
    parts = []

    def add(field):
        val = message.get(field)
        if isinstance(val, str):
            parts.append(val)
        elif isinstance(val, list):
            for chunk in val:
                if isinstance(chunk, str):
                    parts.append(chunk)
                elif isinstance(chunk, dict):
                    parts.append(chunk.get("text", ""))

    add("text")  # основной текст и подпись к фото лежат здесь
    return "".join(parts)


def main():
    # путь к result.json: из аргумента или рядом со скриптом
    if len(sys.argv) > 1:
        path = sys.argv[1]
    else:
        path = os.path.join(os.path.dirname(os.path.abspath(__file__)), "result.json")

    if not os.path.exists(path):
        raise SystemExit(
            f"Не найден файл экспорта: {path}\n"
            "Укажи путь: python parse_export.py /путь/до/result.json"
        )

    with open(path, encoding="utf-8") as f:
        data = json.load(f)

    messages = data.get("messages", data if isinstance(data, list) else [])
    chat_name = data.get("name", "(чат)")
    pat = re.compile(re.escape(TAG) + r"\b", re.IGNORECASE)

    tag_msgs = defaultdict(int)       # всего сообщений с тегом
    days = defaultdict(set)           # уникальные дни
    names = {}                        # from_id -> отображаемое имя
    dump_rows = []

    total = 0
    matched = 0
    for m in messages:
        if m.get("type") != "message":
            continue  # пропускаем сервисные (вступил/вышел и т.п.)
        total += 1
        text = extract_text(m)
        if not text or not pat.search(text):
            continue

        uid = m.get("from_id", "?")
        name = m.get("from") or str(uid)
        names[uid] = name

        # дата в экспорте: "2026-03-01T08:15:42" (локальное время Desktop)
        day = (m.get("date") or "")[:10]

        tag_msgs[uid] += 1
        if day:
            days[uid].add(day)
        matched += 1
        dump_rows.append({
            "date": m.get("date", ""),
            "user": name,
            "from_id": uid,
            "text": " ".join(text.split())[:200],
        })

    # --- сводка ---
    rows = sorted(
        ((names[uid], tag_msgs[uid], len(days[uid])) for uid in tag_msgs),
        key=lambda r: (-r[1], -r[2]),
    )

    print(f"Чат: {chat_name}")
    print(f"\n==== ТРЕНИРОВКИ ПО ТЕГУ {TAG} ====")
    print(f"{'Юзер':<24}{'сообщ. с тегом':>15}{'активных дней':>15}")
    print("-" * 54)
    for name, n, d in rows:
        print(f"{name:<24}{n:>15}{d:>15}")
    print("-" * 54)
    print(f"{'ИТОГО':<24}{sum(r[1] for r in rows):>15}{'':>15}")
    print(f"\nВсего сообщений в экспорте: {total}, с тегом: {matched}, людей: {len(rows)}")

    out_dir = os.path.dirname(os.path.abspath(__file__))
    summary_path = os.path.join(out_dir, "trainings.csv")
    with open(summary_path, "w", newline="", encoding="utf-8") as f:
        w = csv.writer(f)
        w.writerow(["user", "from_id", "tag_msgs", "active_days"])
        for uid in sorted(tag_msgs, key=lambda u: -tag_msgs[u]):
            w.writerow([names[uid], uid, tag_msgs[uid], len(days[uid])])

    dump_path = os.path.join(out_dir, "messages_dump.csv")
    with open(dump_path, "w", newline="", encoding="utf-8") as f:
        w = csv.DictWriter(f, fieldnames=["date", "user", "from_id", "text"])
        w.writeheader()
        w.writerows(dump_rows)

    print(f"\nСохранено: {summary_path}")
    print(f"Сохранено: {dump_path}")


if __name__ == "__main__":
    main()

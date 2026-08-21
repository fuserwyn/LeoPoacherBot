#!/usr/bin/env python3
"""
Считает тренировки по тегу #training_done из HTML-экспорта Telegram Desktop.

Telegram Desktop -> Export chat history -> Format: HTML создаёт папку с
messages.html, messages2.html, ... Скрипт читает их все и группирует
сообщения с тегом по авторам.

Важно про HTML-экспорт:
  - имена тут ОТОБРАЖАЕМЫЕ (напр. "Юлия"), а не @username;
  - у подряд идущих сообщений одного автора имя указано только в первом
    ("joined") — переиспользуем предыдущего отправителя;
  - у сообщения может быть несколько дат-блоков (само сообщение + пересланное),
    берём первую = время в чате.

Запуск:
    python3 parse_export_html.py "/путь/до/папки/ChatExport_..."
    # без аргумента ищет messages*.html рядом со скриптом
"""

import csv
import glob
import html
import os
import re
import sys
from collections import defaultdict

TAG = os.getenv("TG_TAG", "#training_done")

RE_DATE = re.compile(r'title="(\d{2})\.(\d{2})\.(\d{4})')
RE_FROM = re.compile(r'<div class="from_name">\s*([^<\n]+?)\s*<')
RE_TEXT = re.compile(r'<div class="text">(.*?)</div>', re.DOTALL)
RE_TAGS = re.compile(r"<[^>]+>")


def clean_text(raw: str) -> str:
    """Убирает html-теги (br -> \\n), декодирует сущности."""
    raw = re.sub(r"<br\s*/?>", "\n", raw, flags=re.IGNORECASE)
    raw = RE_TAGS.sub("", raw)
    return html.unescape(raw).strip()


def iter_messages(html_text: str):
    """Разбивает страницу на блоки сообщений и отдаёт (sender_raw, day, text).
    sender_raw=None у joined-сообщений (имя не указано)."""
    # делим по началу очередного message-блока
    parts = re.split(r'(?=<div class="message )', html_text)
    for block in parts:
        if not block.startswith('<div class="message '):
            continue
        # тип сообщения: service-блоки (даты-разделители) пропускаем
        head = block[:80]
        if "message service" in head:
            continue

        m_date = RE_DATE.search(block)
        day = ""
        if m_date:
            d, mo, y = m_date.groups()
            day = f"{y}-{mo}-{d}"

        m_from = RE_FROM.search(block)
        sender_raw = m_from.group(1).strip() if m_from else None

        m_text = RE_TEXT.search(block)
        text = clean_text(m_text.group(1)) if m_text else ""

        yield sender_raw, day, text


def main():
    folder = sys.argv[1] if len(sys.argv) > 1 else os.path.dirname(os.path.abspath(__file__))
    files = sorted(
        glob.glob(os.path.join(folder, "messages*.html")),
        key=lambda p: (len(os.path.basename(p)), p),  # messages.html, messages2.html,...
    )
    if not files:
        raise SystemExit(f"Не найдено messages*.html в {folder}")

    print(f"Файлов экспорта: {len(files)}")
    pat = re.compile(re.escape(TAG) + r"\b", re.IGNORECASE)

    tag_msgs = defaultdict(int)
    days = defaultdict(set)
    dump_rows = []
    total = 0
    matched = 0
    cur_sender = "(unknown)"

    for fp in files:
        with open(fp, encoding="utf-8") as f:
            page = f.read()
        for sender_raw, day, text in iter_messages(page):
            total += 1
            if sender_raw:
                cur_sender = sender_raw
            sender = sender_raw or cur_sender
            if not text or not pat.search(text):
                continue
            tag_msgs[sender] += 1
            if day:
                days[sender].add(day)
            matched += 1
            dump_rows.append({"date": day, "user": sender,
                              "text": " ".join(text.split())[:200]})

    rows = sorted(((s, tag_msgs[s], len(days[s])) for s in tag_msgs),
                  key=lambda r: (-r[1], -r[2]))

    print(f"\n==== ТРЕНИРОВКИ ПО ТЕГУ {TAG} ====")
    print(f"{'Юзер':<26}{'сообщ. с тегом':>16}{'активных дней':>16}")
    print("-" * 58)
    for s, n, d in rows:
        print(f"{s:<26}{n:>16}{d:>16}")
    print("-" * 58)
    print(f"{'ИТОГО':<26}{sum(r[1] for r in rows):>16}{'':>16}")
    print(f"\nВсего сообщений: {total}, с тегом: {matched}, людей: {len(rows)}")

    out_dir = os.path.dirname(os.path.abspath(__file__))
    summary_path = os.path.join(out_dir, "trainings.csv")
    with open(summary_path, "w", newline="", encoding="utf-8") as f:
        w = csv.writer(f)
        w.writerow(["user", "tag_msgs", "active_days"])
        for s, n, d in rows:
            w.writerow([s, n, d])

    dump_path = os.path.join(out_dir, "messages_dump.csv")
    with open(dump_path, "w", newline="", encoding="utf-8") as f:
        w = csv.DictWriter(f, fieldnames=["date", "user", "text"])
        w.writeheader()
        w.writerows(dump_rows)

    print(f"\nСохранено: {summary_path}")
    print(f"Сохранено: {dump_path}")


if __name__ == "__main__":
    main()

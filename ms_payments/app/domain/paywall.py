"""
Чистые функции по paywall / суммам из ответа ЮKassa (без БД и HTTP).
"""

from __future__ import annotations

import re
from typing import Any

_paywall_payload_re = re.compile(r"^pw_(\d+)$")
_donation_payload_re = re.compile(r"^dn_(\d+)$")


def parse_paywall_payload(payload: str) -> int | None:
    """Из invoice_payload вида ``pw_<id>`` возвращает числовой id заявки."""
    payload = (payload or "").strip()
    m = _paywall_payload_re.match(payload)
    if not m:
        return None
    n = int(m.group(1))
    return n if n > 0 else None


def is_donation_payload(payload: str) -> bool:
    """``dn_<id>`` — добровольный донат из профиля мини-аппа.

    Донаты не выдают доступ и живут в таблице ``donations``: их закрывает сам ms_leo,
    опрашивая API ЮKassa (см. ms_leo/internal/bot/donate.go). Этому сервису такое
    уведомление нужно только подтвердить, чтобы ЮKassa не ретраила и не поднимала алерты.

    ``dn_0`` и прочий мусор донатом не считаем — как и parseDonatePayload в ms_leo.
    """
    m = _donation_payload_re.match((payload or "").strip())
    return bool(m) and int(m.group(1)) > 0


def minor_units_from_yookassa_amount(amount_block: Any) -> tuple[int, str]:
    """Блок ``object.amount`` ЮKassa → (минорные единицы, ISO валюта)."""
    if not isinstance(amount_block, dict):
        return 0, ""
    cur = str(amount_block.get("currency") or "").strip().upper()
    raw = amount_block.get("value")
    try:
        val = float(str(raw).replace(",", "."))
    except (TypeError, ValueError):
        return 0, cur
    return int(round(val * 100)), cur

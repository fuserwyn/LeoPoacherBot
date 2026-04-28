"""
Репозиторий: таблица paywall_access_requests (основная БД бота).
"""

from __future__ import annotations

import logging
from typing import Any

import asyncpg

logger = logging.getLogger(__name__)


class PaywallRepository:
    def __init__(self, pool: asyncpg.Pool) -> None:
        self._pool = pool

    async def get_by_id(self, req_id: int) -> dict[str, Any] | None:
        row = await self._pool.fetchrow(
            """
            SELECT id, user_id, monetized_chat_id, status, created_at, completed_at,
                   access_expires_at, telegram_payment_charge_id, total_amount_minor, currency
            FROM paywall_access_requests
            WHERE id = $1
            """,
            req_id,
        )
        return dict(row) if row else None

    async def complete_if_pending_and_enqueue_restore(
        self,
        req_id: int,
        user_id: int,
        monetized_chat_id: int,
        charge_id: str,
        amount_minor: int,
        currency: str,
    ) -> bool:
        """Атомарно закрывает заявку (status=pending → completed) и кладёт outbox-событие
        ``paywall_access_restore_requested``, чтобы post-payment-доставку (welcome, menu_button,
        ReactivateReturnedUser) делал ms_leo через outbox_worker — точно так же, как для оплат
        Telegram Payments / Stars. До этого ms_payments слал welcome сам, но не мог поставить
        synthe-кнопку LeopardMiniApp, потому что это in-process кеш Go (см. ms_leo: applyMiniappMenuButtonForUser).

        Доступ — разовая покупка: ``access_expires_at = 'infinity'::timestamptz``; обнуляется только
        киком за неактивность через ExpirePaywallAccessForUser в ms_leo.
        """
        import json as _json

        async with self._pool.acquire() as conn, conn.transaction():
            updated = await conn.fetchval(
                """
                UPDATE paywall_access_requests
                SET status = 'completed',
                    completed_at = NOW(),
                    access_expires_at = 'infinity'::timestamptz,
                    telegram_payment_charge_id = $4,
                    total_amount_minor = $5,
                    currency = $6
                WHERE id = $1 AND user_id = $2 AND monetized_chat_id = $3 AND status = 'pending'
                RETURNING id
                """,
                req_id,
                user_id,
                monetized_chat_id,
                charge_id,
                amount_minor,
                currency,
            )
            if updated is None:
                return False

            payload_json = _json.dumps(
                {"request_id": int(req_id), "user_id": int(user_id), "chat_id": int(monetized_chat_id)},
                ensure_ascii=False,
            )
            await conn.execute(
                """
                INSERT INTO outbox_events (event_type, aggregate_key, payload, status, next_attempt_at)
                VALUES ($1, $2, $3::jsonb, 'pending', NOW())
                """,
                "paywall_access_restore_requested",
                f"paywall_request:{int(req_id)}",
                payload_json,
            )
            return True

# reactivate_returned_user удалён: после переключения Yookassa-вебхука на outbox-event
# `paywall_access_restore_requested` это делает ms_leo через outbox_worker, вызывая
# paywallDeliverAccessAfterPayment → b.db.ReactivateReturnedUser. Дублирование в Python
# приводило к разъезжанию логики (часовые пояса, INSERT-набор колонок) при изменениях в Go.

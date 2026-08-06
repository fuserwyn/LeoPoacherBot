"""
Сервис обработки успешной оплаты ЮKassa: валидация, БД бота, леджер, Telegram.
"""

from __future__ import annotations

import logging
from dataclasses import dataclass

from app.core.config import Settings
from app.domain.paywall import (
    is_donation_payload,
    minor_units_from_yookassa_amount,
    parse_paywall_payload,
)
from app.domain.schemas.webhook import PaymentNotification
from app.repositories.payment_ledger_repository import PaymentLedgerRepository
from app.repositories.paywall_repository import PaywallRepository
from app.services.telegram_gateway import TelegramGateway

logger = logging.getLogger(__name__)

# Welcome-текст после успешной оплаты теперь шлёт ms_leo (см. paywallPostPaymentUserText)
# через outbox_worker → paywallDeliverAccessAfterPayment. ms_payments только пишет в БД
# (UPDATE заявки + INSERT outbox_events) и больше не дублирует welcome — это позволяет
# ms_leo одновременно с welcome корректно выставить per-user setChatMenuButton (синяя
# кнопка LeopardMiniApp), потому что кеш менюшки живёт в памяти Go-процесса.


@dataclass(frozen=True)
class WebhookOutcome:
    """Результат обработки вебхука для HTTP-ответа."""

    status_code: int
    body: dict


def _metadata_string(meta: dict, *keys: str) -> str:
    for k in keys:
        v = meta.get(k)
        if v is None:
            continue
        return str(v).strip()
    return ""


class PaymentWebhookService:
    def __init__(
        self,
        paywall_repo: PaywallRepository,
        ledger_repo: PaymentLedgerRepository | None,
        telegram: TelegramGateway,
        app_settings: Settings,
    ) -> None:
        self._paywall = paywall_repo
        self._ledger = ledger_repo
        self._telegram = telegram
        self._settings = app_settings

    async def _notify_ops(self, msg: str) -> None:
        logger.error(msg)
        if self._settings.owner_id:
            try:
                await self._telegram.notify_owner(self._settings.owner_id, msg)
            except Exception as e:
                logger.warning("yookassa webhook: notify owner failed: %s", e)

    async def _fail_paid_without_access(
        self,
        req_id: int,
        user_tid: int,
        payment_id: str,
        reason: str,
    ) -> None:
        """Оплата в ЮKassa прошла, доступ не выдали — refund через outbox (ms_leo) + алерт владельцу."""
        try:
            await self._paywall.enqueue_refund_requested(req_id, user_tid, reason)
            logger.info(
                "yookassa webhook: refund_requested enqueued req=%s user=%s payment=%s",
                req_id,
                user_tid,
                payment_id,
            )
        except Exception as e:
            logger.error(
                "yookassa webhook: refund enqueue failed req=%s payment=%s: %s",
                req_id,
                payment_id,
                e,
            )
        await self._notify_ops(
            f"YooKassa webhook: оплата {payment_id}, доступ не выдан. "
            f"req={req_id} user={user_tid}. {reason}. refund_requested в outbox."
        )

    async def _ensure_miniapp_menu_button(self, user_tid: int) -> None:
        """Best-effort: сразу включаем web_app кнопку после успешной оплаты по webhook."""
        try:
            ok = await self._telegram.set_chat_menu_button_default(user_tid)
            if ok:
                logger.info("yookassa webhook: setChatMenuButton(default) applied for user=%s", user_tid)
        except Exception as e:
            logger.warning("yookassa webhook: setChatMenuButton(default) failed user=%s err=%s", user_tid, e)

    async def handle_payment_succeeded(self, notification: PaymentNotification) -> WebhookOutcome:
        obj = notification.object or {}
        payment_id = str(obj.get("id") or "").strip()
        if not payment_id:
            return WebhookOutcome(400, {"status": "payment id missing"})

        meta = obj.get("metadata") or {}
        if not isinstance(meta, dict):
            meta = {}

        logger.info(
            "yookassa webhook: payment=%s metadata keys=%s",
            payment_id,
            sorted(meta.keys()),
        )

        user_raw = _metadata_string(meta, "user_telegram_id", "user_telegramId")
        payload_str = _metadata_string(meta, "invoice_payload", "invoicePayload")

        if not user_raw:
            logger.warning("yookassa webhook: no user_telegram_id in metadata, payment=%s", payment_id)
            return WebhookOutcome(400, {"status": "user_telegram_id missing"})

        try:
            user_tid = int(user_raw)
        except ValueError:
            return WebhookOutcome(400, {"status": "invalid user_telegram_id"})

        # Донат из профиля мини-аппа: доступ не выдаётся, заявки paywall нет.
        # Подтверждаем уведомление, чтобы ЮKassa не ретраила; статус доната закрывает ms_leo.
        if is_donation_payload(payload_str):
            logger.info(
                "yookassa webhook: донат payment=%s user=%s payload=%r — закрывает ms_leo",
                payment_id,
                user_tid,
                payload_str,
            )
            return WebhookOutcome(200, {"status": "donation, handled by ms_leo"})

        req_id = parse_paywall_payload(payload_str)
        if req_id is None:
            logger.warning(
                "yookassa webhook: invoice_payload must be pw_<id>, got=%r",
                payload_str,
            )
            return WebhookOutcome(
                400,
                {"status": "invalid invoice_payload, expected pw_<request_id>"},
            )

        if self._settings.monetized_chat_id == 0:
            logger.error("MONETIZED_CHAT_ID is not set")
            return WebhookOutcome(500, {"status": "server misconfigured"})

        if not (self._settings.bot_token or "").strip():
            logger.error(
                "FAT_LEOPARD_API_TOKEN/API_TOKEN is empty: оплата в БД может пройти, но вход в Telegram не будет обработан"
            )

        rec = await self._paywall.get_by_id(req_id)
        if not rec:
            return WebhookOutcome(404, {"status": "paywall request not found"})

        if int(rec["user_id"]) != user_tid:
            logger.warning(
                "yookassa webhook: user mismatch payment=%s db_user=%s meta_user=%s",
                payment_id,
                rec["user_id"],
                user_tid,
            )
            await self._fail_paid_without_access(
                req_id,
                user_tid,
                payment_id,
                f"user mismatch meta={user_tid} db={rec['user_id']}",
            )
            return WebhookOutcome(403, {"status": "user mismatch"})

        env_chat = self._settings.monetized_chat_id
        db_chat = int(rec["monetized_chat_id"])
        if db_chat != env_chat:
            logger.warning(
                "yookassa webhook: MONETIZED_CHAT_ID на ms_payments не совпадает с заявкой "
                "req=%s db_chat=%s env_chat=%s — закрываем по db_chat (заявку создал ms_leo). "
                "Выровняй MONETIZED_CHAT_ID на ms_payments с ms_leo.",
                req_id,
                db_chat,
                env_chat,
            )

        amount_minor, currency = minor_units_from_yookassa_amount(obj.get("amount"))
        if amount_minor <= 0 or not currency:
            logger.warning("yookassa webhook: missing amount, payment=%s", payment_id)
            amount_minor = int(rec["total_amount_minor"] or 0)
            currency = str(rec["currency"] or "RUB")
            if amount_minor <= 0:
                amount_minor = 1

        # Pack id из строки заявки — источник истины для этого платежа (pw_<id>).
        chat_id = db_chat

        if self._ledger:
            await self._ledger.upsert_webhook(
                payment_id,
                req_id,
                user_tid,
                chat_id,
                amount_minor,
                currency,
                notification.event,
            )

        if rec["status"] == "completed":
            logger.info("yookassa webhook: already completed payment=%s req=%s", payment_id, req_id)
            await self._ensure_miniapp_menu_button(user_tid)
            if self._ledger:
                await self._ledger.mark_main_db_synced(payment_id)
            return WebhookOutcome(200, {"status": "already processed"})

        if rec["status"] != "pending":
            return WebhookOutcome(409, {"status": f"unexpected status {rec['status']}"})

        updated = await self._paywall.complete_if_pending_and_enqueue_restore(
            req_id,
            user_tid,
            chat_id,
            payment_id,
            amount_minor,
            currency,
        )
        if not updated:
            row = await self._paywall.get_by_id(req_id)
            st = row.get("status") if row else "missing"
            if st == "completed":
                logger.info(
                    "yookassa webhook: повтор уведомления, заявка уже completed payment=%s req=%s",
                    payment_id,
                    req_id,
                )
            else:
                logger.warning(
                    "yookassa webhook: не удалось закрыть заявку (payment=%s req=%s user=%s), status=%s. "
                    "Частые причины: другой DATABASE_URL у ms_payments и бота, неверный MONETIZED_CHAT_ID в вебхуке, "
                    "или расхождение user_id в metadata ЮKassa.",
                    payment_id,
                    req_id,
                    user_tid,
                    st,
                )
            if self._ledger:
                await self._ledger.mark_main_db_synced(payment_id)
            return WebhookOutcome(200, {"status": "already processed"})

        logger.info(
            "yookassa webhook: заявка закрыта и outbox-событие paywall_access_restore_requested "
            "поставлено — user=%s req=%s chat=%s. Welcome/menu_button/ReactivateReturnedUser сделает ms_leo.",
            user_tid,
            req_id,
            chat_id,
        )

        if self._ledger:
            await self._ledger.mark_main_db_synced(payment_id)

        await self._ensure_miniapp_menu_button(user_tid)

        # Группы больше нет, и ms_payments больше не шлёт welcome / не делает reactivate сам.
        # Всё post-payment делает ms_leo через outbox_worker (paywallDeliverAccessAfterPayment):
        # это гарантирует, что синяя кнопка LeopardMiniApp выставится тем же процессом, который
        # держит in-memory кеш menuButtonState (см. ms_leo/internal/bot/miniapp_menu_button.go).
        # TelegramGateway сюда больше не вовлечён — он остался для других ms_payments-сценариев.

        if not self._ledger:
            logger.info(
                "yookassa webhook: успех payment=%s — строка в yookassa_payment_events не писалась "
                "(нет PAYMENT_DATABASE_URL, леджер отключён; оплата в основной БД: paywall_access_requests)",
                payment_id,
            )
        logger.info("yookassa webhook: completed payment=%s req=%s user=%s", payment_id, req_id, user_tid)
        return WebhookOutcome(200, {"status": "success"})

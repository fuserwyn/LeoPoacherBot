"""
Сервис обработки успешной оплаты ЮKassa: валидация, БД бота, леджер, Telegram.
"""

from __future__ import annotations

import logging
from dataclasses import dataclass

from app.core.config import Settings
from app.domain.paywall import minor_units_from_yookassa_amount, parse_paywall_payload
from app.domain.schemas.webhook import PaymentNotification
from app.repositories.payment_ledger_repository import PaymentLedgerRepository
from app.repositories.paywall_repository import PaywallRepository
from app.services.telegram_gateway import TelegramGateway

logger = logging.getLogger(__name__)

# Текст после успешной оплаты Yookassa. Архитектура mini-app-only:
# TG-группы как сущности больше нет (см. ms_leo: paywallPostPaymentUserText), вся
# механика стаи живёт в мини-аппе. Поэтому никакой «Войти в группу», invite-ссылок,
# /rejoin-инструкций — только указатель открыть мини-апп.
_PAYWALL_POST_PAYMENT_TEXT = (
    "✅ Оплата через ЮKassa принята, вход в Fat Leopard MiniApp открыт на 30 дней.\n\n"
    "📱 Открой мини-приложение бота — кнопка LeopardMiniApp внизу в этом чате (или через меню ⋮). "
    "Там лента стаи, общий чат, тренировки и больничные. При первом открытии бот автоматически "
    "добавит тебя в стаю и запустит таймер активности.\n\n"
    "ℹ️ Тренировки можно отмечать кнопкой «+» в мини-аппе, либо просто отправить в личку боту "
    "сообщение с тегом #training_done."
)


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
            return WebhookOutcome(403, {"status": "user mismatch"})

        if int(rec["monetized_chat_id"]) != self._settings.monetized_chat_id:
            logger.warning(
                "yookassa webhook: chat mismatch req=%s db_chat=%s env_chat=%s",
                req_id,
                rec["monetized_chat_id"],
                self._settings.monetized_chat_id,
            )
            return WebhookOutcome(403, {"status": "chat mismatch"})

        amount_minor, currency = minor_units_from_yookassa_amount(obj.get("amount"))
        if amount_minor <= 0 or not currency:
            logger.warning("yookassa webhook: missing amount, payment=%s", payment_id)
            amount_minor = int(rec["total_amount_minor"] or 0)
            currency = str(rec["currency"] or "RUB")
            if amount_minor <= 0:
                amount_minor = 1

        chat_id = int(rec["monetized_chat_id"])

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
            if self._ledger:
                await self._ledger.mark_main_db_synced(payment_id)
            return WebhookOutcome(200, {"status": "already processed"})

        if rec["status"] != "pending":
            return WebhookOutcome(409, {"status": f"unexpected status {rec['status']}"})

        updated = await self._paywall.complete_if_pending(
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
            "yookassa webhook: в БД бота заявка закрыта — user=%s req=%s chat=%s, доступ ~30 дней (paywall_access_requests)",
            user_tid,
            req_id,
            chat_id,
        )

        if self._ledger:
            await self._ledger.mark_main_db_synced(payment_id)

        try:
            await self._paywall.reactivate_returned_user(user_tid, chat_id)
        except Exception:
            logger.exception(
                "yookassa webhook: reactivate_returned_user не удалась user=%s chat=%s",
                user_tid,
                chat_id,
            )

        # Группы больше нет: никаких unban/approve_join/create_invite, никакой кнопки «Войти в группу».
        # Доступ к мини-аппу открывается строкой в paywall_access_requests (выше),
        # а кнопка LeopardMiniApp у юзера появляется через setChatMenuButton со стороны ms_leo
        # после первого /start или ближайшего хука обновления menu button.
        await self._telegram.send_message(user_tid, _PAYWALL_POST_PAYMENT_TEXT)

        if not self._ledger:
            logger.info(
                "yookassa webhook: успех payment=%s — строка в yookassa_payment_events не писалась "
                "(нет PAYMENT_DATABASE_URL, леджер отключён; оплата в основной БД: paywall_access_requests)",
                payment_id,
            )
        logger.info("yookassa webhook: completed payment=%s req=%s user=%s", payment_id, req_id, user_tid)
        return WebhookOutcome(200, {"status": "success"})

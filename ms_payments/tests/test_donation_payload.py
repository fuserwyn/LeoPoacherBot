"""Донаты (``dn_<id>``) этот сервис не обслуживает, но и ошибкой считать не должен."""

from __future__ import annotations

from unittest.mock import AsyncMock, MagicMock

import pytest

from app.domain.paywall import is_donation_payload, parse_paywall_payload
from app.domain.schemas.webhook import PaymentNotification
from app.services.payment_webhook_service import PaymentWebhookService


def test_donation_and_paywall_payloads_are_disjoint() -> None:
    assert is_donation_payload("dn_42")
    assert is_donation_payload("  dn_1  ")
    assert not is_donation_payload("dn_0")
    assert not is_donation_payload("dn_abc")
    assert not is_donation_payload("pw_42")
    assert parse_paywall_payload("dn_42") is None


@pytest.mark.asyncio
async def test_donation_notification_is_acknowledged_without_paywall_lookup() -> None:
    """200 без обращения к заявкам: иначе ЮKassa ретраила бы, а владелец получал алерты."""
    paywall_repo = MagicMock()
    paywall_repo.get_by_id = AsyncMock()
    settings = MagicMock(monetized_chat_id=-100500, bot_token="token", owner_id=1)
    service = PaymentWebhookService(paywall_repo, None, MagicMock(), settings)

    notification = PaymentNotification(
        event="payment.succeeded",
        object={
            "id": "2f0c-donation",
            "amount": {"value": "300.00", "currency": "RUB"},
            "metadata": {"user_telegram_id": "777", "invoice_payload": "dn_9"},
        },
    )
    outcome = await service.handle_payment_succeeded(notification)

    assert outcome.status_code == 200
    paywall_repo.get_by_id.assert_not_awaited()

package bot

import (
	"fmt"
	"strings"

	"leo-bot/internal/database"
)

// welcomeEventIdempotencyKey — ключ дедупликации welcome_message_sent по заявке paywall.
// reqID<=0 (нет заявки) => пустой ключ (без дедупликации).
func welcomeEventIdempotencyKey(reqID int64) string {
	if reqID <= 0 {
		return ""
	}
	return fmt.Sprintf("welcome_message_sent:%d", reqID)
}

// trackPaymentCompleted фиксирует успешную оплату (воронка 1, §3) с дедупликацией по заявке.
// provider/amount/payment_id берём из записи paywall-заявки.
func (b *Bot) trackPaymentCompleted(userID int64, rec *database.PaywallAccessRequest) {
	if b == nil || b.db == nil || rec == nil {
		return
	}
	payload := map[string]any{}
	var idemSuffix string
	switch {
	case rec.YookassaPaymentID.Valid && rec.YookassaPaymentID.String != "":
		payload["provider"] = "yukassa"
		payload["payment_id"] = rec.YookassaPaymentID.String
		idemSuffix = rec.YookassaPaymentID.String
	case rec.TelegramPaymentChargeID.Valid && rec.TelegramPaymentChargeID.String != "":
		payload["provider"] = "stars"
		payload["payment_id"] = rec.TelegramPaymentChargeID.String
		idemSuffix = rec.TelegramPaymentChargeID.String
	}
	if rec.TotalAmountMinor.Valid {
		payload["amount_minor"] = rec.TotalAmountMinor.Int64
	}
	if rec.Currency.Valid && rec.Currency.String != "" {
		payload["currency"] = rec.Currency.String
	}
	if idemSuffix == "" {
		idemSuffix = fmt.Sprintf("req:%d", rec.ID)
	}
	b.db.TrackEvent(database.AnalyticsEvent{
		Name:           database.EventPaymentCompleted,
		UserID:         userID,
		TelegramID:     userID,
		Payload:        payload,
		IdempotencyKey: "payment_completed:" + idemSuffix,
	})
}

// parseStartSource достаёт канал атрибуции из аргумента deep-link `/start`.

// trackModerationBlocked фиксирует срабатывание PRE-фильтра модерации (analytics_BT_v1 §6).
// surface: "workout_note" | "feed_comment" | "leo_chat"; reason — код причины (APICode).
func (b *Bot) trackModerationBlocked(surface, reason string, telegramID int64) {
	if b == nil || b.db == nil {
		return
	}
	if reason == "" {
		reason = "profanity"
	}
	b.db.TrackEvent(database.AnalyticsEvent{
		Name:       database.EventModerationBlocked,
		TelegramID: telegramID,
		Payload:    map[string]any{"surface": surface, "reason": reason},
	})
}

// parseStartSource достаёт канал атрибуции из аргумента deep-link `/start`.
// Формат UTM (analytics_BT_v1 §8): `?start=src-tg_channel_main` → "tg_channel_main".
// Пустой аргумент => "organic". Значение усечено до 32 символов (колонка source).
func parseStartSource(arg string) string {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "organic"
	}
	arg = strings.TrimPrefix(arg, "src-")
	if len(arg) > 32 {
		arg = arg[:32]
	}
	return arg
}

// TrackEvent — фасад над database.TrackEvent для слоёв, у которых есть только *Bot
// (например HTTP-сервер мини-аппа). Запись non-blocking, ошибки не пробрасываются.
func (b *Bot) TrackEvent(ev database.AnalyticsEvent) {
	if b == nil || b.db == nil {
		return
	}
	b.db.TrackEvent(ev)
}

package database

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// Аналитические события Fat Leopard — спецификация analytics_BT_v1.md.
// Имена событий используются как event_name в таблице events; держим их
// константами, чтобы эмиттеры в боте/мини-аппе не расходились по строкам.
const (
	// Воронка 1 — БОТ (приобретение и оплата)
	EventBotStarted            = "bot_started"
	EventPaywallViewed         = "paywall_viewed"
	EventPaymentMethodSelected = "payment_method_selected"
	EventPaymentInitiated      = "payment_initiated"
	EventPaymentCompleted      = "payment_completed"
	EventPaymentFailed         = "payment_failed"
	EventWelcomeMessageSent    = "welcome_message_sent"
	EventMiniappOpened         = "miniapp_opened"

	// Воронка 2 — АКТИВАЦИЯ (первая тренировка)
	EventWorkoutLogStarted   = "workout_log_started"
	EventWorkoutLogged       = "workout_logged"
	EventLeoCommentReceived  = "leo_comment_received"
	EventLeoCommentDisplayed = "leo_comment_displayed"

	// Воронка 3 — RETENTION И СЕРИИ
	EventStreakIncremented        = "streak_incremented"
	EventStreakAttemptUsed        = "streak_attempt_used"
	EventStreakBroken             = "streak_broken"
	EventLevelUp                  = "level_up"
	EventMilestoneAchieved        = "milestone_achieved"
	EventBurnWarningSent          = "burn_warning_sent"
	EventBurnRecovered            = "burn_recovered"
	EventSickLeaveStarted         = "sick_leave_started"
	EventSickLeaveEnded           = "sick_leave_ended"
	EventAccountDeletedInactivity = "account_deleted_inactivity"
	EventAccountReactivated       = "account_reactivated"

	// UGC, Лео, модерация
	EventFeedReactionAdded    = "feed_reaction_added"
	EventFeedCommentPosted    = "feed_comment_posted"
	EventLeoChatMessageSent   = "leo_chat_message_sent"
	EventLeoChatLimitReached  = "leo_chat_limit_reached"
	EventModerationBlocked    = "moderation_blocked"
	EventComplaintFiled       = "complaint_filed"
	EventReportResolved       = "report_resolved"
	EventSupportButtonClicked = "support_button_clicked"
)

// AnalyticsEvent — одно событие для записи в таблицу events.
// Нулевые user_id/telegram_id и пустые строковые поля пишутся как NULL.
type AnalyticsEvent struct {
	Name       string
	UserID     int64
	TelegramID int64
	Payload    map[string]any
	Source     string // utm_source / channel attribution
	SessionID  string // UUID; пустая строка => NULL
	AppVersion string
	// IdempotencyKey — ключ дедупликации (напр. payment_id для финансовых событий).
	// При повторной вставке с тем же ключом запись не плодится (§9.2).
	IdempotencyKey string
}

// TrackEvent асинхронно (non-blocking, §9.2) пишет событие в таблицу events.
// Ошибки не возвращаются — аналитика никогда не должна мешать основному flow.
func (d *Database) TrackEvent(ev AnalyticsEvent) {
	if d == nil || d.db == nil || strings.TrimSpace(ev.Name) == "" {
		return
	}
	go func() {
		if err := d.insertEvent(ev); err != nil {
			if d.logger != nil {
				d.logger.Warnf("TrackEvent %s: %v", ev.Name, err)
			} else {
				log.Printf("TrackEvent %s: %v", ev.Name, err)
			}
		}
	}()
}

func (d *Database) insertEvent(ev AnalyticsEvent) error {
	var payloadJSON []byte
	if len(ev.Payload) > 0 {
		b, err := json.Marshal(ev.Payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}
		payloadJSON = b
	}

	// occurred_at заполняется DEFAULT-ом (московский wall-clock, как остальные таблицы).
	// NULLIF(...,'') приводит пустые строки/нули к NULL, чтобы не засорять колонки.
	_, err := d.db.Exec(`
		INSERT INTO events (event_name, user_id, telegram_id, payload, session_id, source, app_version, idempotency_key)
		VALUES ($1, NULLIF($2, 0), NULLIF($3, 0), $4, NULLIF($5, '')::uuid, NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''))
		ON CONFLICT (idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
	`, ev.Name, ev.UserID, ev.TelegramID, payloadJSON, ev.SessionID, ev.Source, ev.AppVersion, ev.IdempotencyKey)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

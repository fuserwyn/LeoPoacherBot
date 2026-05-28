package bot

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"leo-bot/internal/moderation"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var ErrModerationBlocked = errors.New("moderation blocked")

// ModerationBlockedError — PRE-блок UGC с кодом для miniapp API.
type ModerationBlockedError struct {
	APICode string
	Message string
}

func (e *ModerationBlockedError) Error() string {
	if e == nil {
		return "moderation blocked"
	}
	if e.Message != "" {
		return e.Message
	}
	return "moderation blocked"
}

func (b *Bot) ugcGate() *moderation.Gate {
	if b == nil {
		return moderation.NewGate(nil)
	}
	if b.ugcModerationGate == nil {
		if b.ugcModerationLimiter == nil {
			b.ugcModerationLimiter = moderation.NewLimiter()
		}
		b.ugcModerationGate = moderation.NewGate(b.ugcModerationLimiter)
	}
	return b.ugcModerationGate
}

func (b *Bot) enforceUGC(text string, surface moderation.Surface, userID int64) (moderation.Result, error) {
	res := b.ugcGate().Check(text, surface, userID, time.Now())
	if res.Allowed {
		return res, nil
	}
	b.deliverModerationWarning(userID, surface, text, res)
	if res.AlertAdmin {
		b.notifyAdminsModerationAlert(userID, surface, text, res)
	}
	return res, &ModerationBlockedError{APICode: res.APICode, Message: res.UserMessage}
}

func (b *Bot) deliverModerationWarning(userID int64, surface moderation.Surface, rawText string, res moderation.Result) {
	if b == nil || userID == 0 || strings.TrimSpace(res.UserMessage) == "" {
		return
	}
	// Системный текст в личку Telegram (не LLM).
	if b.api != nil {
		m := tgbotapi.NewMessage(userID, "⚠️ "+res.UserMessage)
		if _, err := b.api.Send(m); err != nil {
			b.logger.Warnf("moderation warning DM user=%d: %v", userID, err)
		}
	}
	_ = surface
	_ = rawText
}

func (b *Bot) notifyAdminsModerationAlert(userID int64, surface moderation.Surface, rawText string, res moderation.Result) {
	if b == nil || b.config == nil {
		return
	}
	surfaceLabel := "UGC"
	switch surface {
	case moderation.SurfaceTrainingNote:
		surfaceLabel = "заметка тренировки"
	case moderation.SurfaceFeedComment:
		surfaceLabel = "комментарий в ленте"
	case moderation.SurfacePackGroupChat:
		surfaceLabel = "общий чат стаи"
	}
	preview := truncateForDM(rawText, 200)
	body := fmt.Sprintf(
		"🚨 PRE-модерация · %s\nПользователь: %d\nПричина: %s\n\n«%s»",
		surfaceLabel, userID, res.Reason, preview,
	)
	for _, adminID := range b.config.AdminTelegramUserIDs() {
		if adminID == 0 {
			continue
		}
		m := tgbotapi.NewMessage(adminID, body)
		if _, err := b.api.Send(m); err != nil {
			b.logger.Warnf("moderation admin alert admin=%d: %v", adminID, err)
		}
	}
}

// ModerationHTTPStatus — HTTP-код для miniapp API при PRE-блоке.
func ModerationHTTPStatus(code string) int {
	switch code {
	case "moderation_rate_limited":
		return 429
	default:
		return 400
	}
}

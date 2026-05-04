package bot

import (
	"fmt"
	"strings"
	"time"

	"leo-bot/internal/ai"
	"leo-bot/internal/domain"
	"leo-bot/internal/game/leopardmoney"
	"leo-bot/internal/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func formatRemovalAtMSKHuman(removalAt time.Time) string {
	loc := utils.GetMoscowTime().Location()
	d := removalAt.In(loc)
	months := []string{
		"", "января", "февраля", "марта", "апреля", "мая", "июня",
		"июля", "августа", "сентября", "октября", "ноября", "декабря",
	}
	return fmt.Sprintf("%d %s в 00:00 МСК", d.Day(), months[int(d.Month())])
}

func (b *Bot) startTimer(userID, chatID int64, username string) {
	b.startTimerWithDuration(userID, chatID, username, leopardmoney.FullTimerDuration)
}

func (b *Bot) startTimerWithDuration(userID, chatID int64, username string, _ time.Duration) {
	messageLog, err := b.db.GetMessageLog(userID, chatID)
	if err == nil && messageLog.IsExemptFromDeletion {
		b.logger.Infof("User %d (%s) is exempt from deletion, skipping timer", userID, username)
		return
	}

	b.cancelTimer(userID)

	ch48 := make(chan bool)
	ch24 := make(chan bool)
	chRem := make(chan bool)

	timerStartTime := utils.FormatMoscowTime(utils.GetMoscowTime())
	timerInfo := &domain.TimerInfo{
		UserID:         userID,
		ChatID:         chatID,
		Username:       username,
		Warning48hTask: ch48,
		Warning24hTask: ch24,
		RemovalTask:    chRem,
		TimerStartTime: timerStartTime,
	}
	b.timers[userID] = timerInfo

	messageLog, err = b.db.GetMessageLog(userID, chatID)
	if err != nil {
		b.logger.Errorf("Failed to get message log for timer start: %v", err)
	} else {
		messageLog.TimerStartTime = &timerStartTime
		if err := b.db.SaveMessageLog(messageLog); err != nil {
			b.logger.Errorf("Failed to save timer start time: %v", err)
		}
	}

	timerStart, err := utils.ParseMoscowTime(timerStartTime)
	if err != nil {
		b.logger.Errorf("parse timer start: %v", err)
		return
	}
	b.scheduleLeopardMilestones(userID, chatID, username, timerStart, ch48, ch24, chRem)
	b.logger.Infof("Started Leopard inactive chain for user %d (%s) from %s", userID, username, timerStartTime)
}

func (b *Bot) restoreTimerWithDuration(userID, chatID int64, username string, remaining time.Duration, existingTimerStartTime string) {
	b.cancelTimer(userID)

	ch48 := make(chan bool)
	ch24 := make(chan bool)
	chRem := make(chan bool)

	timerInfo := &domain.TimerInfo{
		UserID:         userID,
		ChatID:         chatID,
		Username:       username,
		Warning48hTask: ch48,
		Warning24hTask: ch24,
		RemovalTask:    chRem,
		TimerStartTime: existingTimerStartTime,
	}
	b.timers[userID] = timerInfo

	timerStart, err := utils.ParseMoscowTime(existingTimerStartTime)
	if err != nil {
		b.logger.Errorf("restore timer parse: %v", err)
		return
	}
	b.scheduleLeopardMilestones(userID, chatID, username, timerStart, ch48, ch24, chRem)
	b.logger.Infof("Restored Leopard inactive chain for user %d (%s), remaining until removal ~ %v", userID, username, remaining)
}

func (b *Bot) scheduleLeopardMilestones(userID, chatID int64, username string, timerStart time.Time, ch48, ch24, chRem chan bool) {
	now := utils.GetMoscowTime()
	removalAt := removalDeadlineMoscow(timerStart)
	type milestone struct {
		at      time.Time
		ch      chan bool
		fn      func()
		removal bool
	}
	ms := []milestone{
		{
			at: removalAt.Add(-48 * time.Hour),
			ch: ch48,
			fn: func() {
				b.sendInactiveRemovalWarning(userID, chatID, username, 48, removalAt)
			},
			removal: false,
		},
		{
			at: removalAt.Add(-24 * time.Hour),
			ch: ch24,
			fn: func() {
				b.sendInactiveRemovalWarning(userID, chatID, username, 24, removalAt)
			},
			removal: false,
		},
		{
			at:      removalAt,
			ch:      chRem,
			fn:      func() { b.removeUser(userID, chatID, username) },
			removal: true,
		},
	}
	for _, m := range ms {
		delay := m.at.Sub(now)
		removal := m.removal
		go func(delay time.Duration, ch chan bool, fn func(), removal bool) {
			// Предупреждения в прошлом — не шлём задним числом. Кик: если дедлайн уже прошёл — выполнить.
			if delay <= 0 {
				if !removal {
					return
				}
				select {
				case <-ch:
					return
				default:
					fn()
				}
				return
			}
			t := time.NewTimer(delay)
			defer t.Stop()
			select {
			case <-ch:
				return
			case <-t.C:
				select {
				case <-ch:
					return
				default:
					fn()
				}
			}
		}(delay, m.ch, m.fn, removal)
	}
}

func (b *Bot) cancelTimer(userID int64) {
	if timer, exists := b.timers[userID]; exists {
		close(timer.Warning48hTask)
		close(timer.Warning24hTask)
		close(timer.RemovalTask)
		delete(b.timers, userID)
		b.logger.Infof("Cancelled timer for user %d", userID)
	}
}

// sendInactiveRemovalWarning — за 48 ч или за 24 ч до кика в 00:00 МСК (см. removalDeadlineMoscow).
func (b *Bot) sendInactiveRemovalWarning(userID, chatID int64, username string, hoursBefore int, removalAt time.Time) {
	who := normalizeUserDisplayName(username)
	tag := "#training_done"
	deadlineHuman := formatRemovalAtMSKHuman(removalAt)
	var windowRU string
	switch hoursBefore {
	case 48:
		windowRU = "примерно двое суток"
	case 24:
		windowRU = "примерно сутки"
	default:
		windowRU = fmt.Sprintf("примерно %d ч.", hoursBefore)
	}
	messageText := fmt.Sprintf(
		"⚠️ Предупреждение о неактивности\n\n"+
			"До возможного удаления из стаи осталось %s.\n\n"+
			"%s, без отчёта с %s кик произойдёт в %s (прогресс — по правилам возврата).\n\n"+
			"В последний календарный день до этой полуночи отчёт ещё можно сдать до 23:59 МСК.",
		windowRU,
		who,
		tag,
		deadlineHuman,
	)

	typingChat := chatID
	if chatID != userID {
		typingChat = userID
	}
	if b.aiClient != nil {
		b.api.Send(tgbotapi.NewChatAction(typingChat, tgbotapi.ChatTyping))
		var ctxBuilder strings.Builder
		ctxBuilder.WriteString(fmt.Sprintf("Пользователь: %s\nДо удаления за неактивность осталось около %d ч.\nДедлайн (МСК): %s\n", username, hoursBefore, deadlineHuman))
		if addendum, err := b.aiClient.AnswerUserQuestion(b.config.Prompts.WarningTimerQuestion, ctxBuilder.String()); err == nil {
			addendum = ai.SanitizeTextForUser(addendum)
			if addendum != "" {
				messageText = messageText + "\n\n" + addendum
			}
		}
	}

	b.miniappPersonalPush(userID, messageText)

	feedLine := fmt.Sprintf("⏳ %s — предупреждение за %d ч. до возможного кика за неактивность. Подробности в ЛС с Лео; стая видит отметку.", who, hoursBefore)
	b.saveInactiveNoticePackFeed(userID, username, feedLine)

	if chatID == userID {
		b.api.Send(tgbotapi.NewMessage(userID, messageText))
		return
	}

	_, dmErr := b.api.Send(tgbotapi.NewMessage(userID, messageText))
	if dmErr != nil {
		b.logger.Warnf("send inactive removal warning DM user=%d: %v", userID, dmErr)
		return
	}
}

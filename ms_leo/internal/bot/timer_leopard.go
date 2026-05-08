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

func formatRemovalAtLocalHuman(removalAt time.Time, loc *time.Location) string {
	d := removalAt.In(loc)
	months := []string{
		"", "января", "февраля", "марта", "апреля", "мая", "июня",
		"июля", "августа", "сентября", "октября", "ноября", "декабря",
	}
	return fmt.Sprintf("%d %s в 00:00 (ваше время)", d.Day(), months[int(d.Month())])
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

	ch72 := make(chan bool)
	ch48 := make(chan bool)
	ch24 := make(chan bool)
	chRem := make(chan bool)

	timerStartTime := utils.FormatMoscowTime(utils.GetMoscowTime())
	timerInfo := &domain.TimerInfo{
		UserID:         userID,
		ChatID:         chatID,
		Username:       username,
		Warning72hTask: ch72,
		Warning48hTask: ch48,
		Warning24hTask: ch24,
		RemovalTask:    chRem,
		TimerStartTime: timerStartTime,
	}
	b.timers[userID] = timerInfo

	tzOffset := 0
	messageLog, err = b.db.GetMessageLog(userID, chatID)
	if err != nil {
		b.logger.Errorf("Failed to get message log for timer start: %v", err)
	} else {
		tzOffset = messageLog.TimezoneOffsetFromMoscow
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
	b.scheduleLeopardMilestones(userID, chatID, username, timerStart, tzOffset, ch72, ch48, ch24, chRem)
	b.logger.Infof("Started Leopard inactive chain for user %d (%s) from %s", userID, username, timerStartTime)
}

func (b *Bot) restoreTimerWithDuration(userID, chatID int64, username string, remaining time.Duration, existingTimerStartTime string, tzOffsetFromMoscow int) {
	b.cancelTimer(userID)

	ch72 := make(chan bool)
	ch48 := make(chan bool)
	ch24 := make(chan bool)
	chRem := make(chan bool)

	timerInfo := &domain.TimerInfo{
		UserID:         userID,
		ChatID:         chatID,
		Username:       username,
		Warning72hTask: ch72,
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
	b.scheduleLeopardMilestones(userID, chatID, username, timerStart, tzOffsetFromMoscow, ch72, ch48, ch24, chRem)
	b.logger.Infof("Restored Leopard inactive chain for user %d (%s), remaining until removal ~ %v", userID, username, remaining)
}

func (b *Bot) scheduleLeopardMilestones(userID, chatID int64, username string, timerStart time.Time, tzOffsetFromMoscow int, ch72, ch48, ch24, chRem chan bool) {
	now := utils.GetMoscowTime()
	loc := userLocalLoc(tzOffsetFromMoscow)
	removalAt := removalDeadlineLocal(timerStart, tzOffsetFromMoscow)
	type milestone struct {
		at      time.Time
		ch      chan bool
		fn      func()
		removal bool
	}
	ms := []milestone{
		{
			at: removalAt.Add(-72 * time.Hour),
			ch: ch72,
			fn: func() {
				b.sendInactiveRemovalWarning(userID, chatID, username, 72, removalAt, loc)
			},
			removal: false,
		},
		{
			at: removalAt.Add(-48 * time.Hour),
			ch: ch48,
			fn: func() {
				b.sendInactiveRemovalWarning(userID, chatID, username, 48, removalAt, loc)
			},
			removal: false,
		},
		{
			at: removalAt.Add(-24 * time.Hour),
			ch: ch24,
			fn: func() {
				b.sendInactiveRemovalWarning(userID, chatID, username, 24, removalAt, loc)
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
		close(timer.Warning72hTask)
		close(timer.Warning48hTask)
		close(timer.Warning24hTask)
		close(timer.RemovalTask)
		delete(b.timers, userID)
		b.logger.Infof("Cancelled timer for user %d", userID)
	}
}

// sendInactiveRemovalWarning — предупреждение за 72 ч (день 5), 48 ч (день 6) или 24 ч (день 7) до кика в 00:00 локального TZ юзера.
func (b *Bot) sendInactiveRemovalWarning(userID, chatID int64, username string, hoursBefore int, removalAt time.Time, loc *time.Location) {
	who := normalizeUserDisplayName(username)
	tag := "#training_done"
	deadlineHuman := formatRemovalAtLocalHuman(removalAt, loc)
	var windowRU string
	switch hoursBefore {
	case 72:
		windowRU = "примерно трое суток"
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
		stage := "day_5"
		switch hoursBefore {
		case 48:
			stage = "day_6"
		case 24:
			stage = "day_7"
		}
		var ctxBuilder strings.Builder
		ctxBuilder.WriteString(fmt.Sprintf("Пользователь: %s\nstage: %s\nДо удаления за неактивность осталось около %d ч.\nДедлайн: %s\n", username, stage, hoursBefore, deadlineHuman))
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

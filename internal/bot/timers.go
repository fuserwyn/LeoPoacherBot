package bot

import (
	"fmt"
	"time"

	"leo-bot/internal/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// startTimer запускает стандартный таймер для пользователя
func (b *Bot) startTimer(userID, chatID int64, username string) {
	b.startTimerWithDuration(userID, chatID, username, 7*24*time.Hour) // 7 дней
}

// startTimerWithDuration запускает таймер с указанной длительностью
func (b *Bot) startTimerWithDuration(userID, chatID int64, username string, duration time.Duration) {
	// Отменяем существующие таймеры
	b.cancelTimer(userID)

	// Создаем новые таймеры
	warningTask := make(chan bool)
	removalTask := make(chan bool)

	timerStartTime := time.Now().Format(time.RFC3339)
	timerInfo := &models.TimerInfo{
		UserID:         userID,
		ChatID:         chatID,
		Username:       username,
		WarningTask:    warningTask,
		RemovalTask:    removalTask,
		TimerStartTime: timerStartTime,
	}

	b.timers[userID] = timerInfo

	// Сохраняем время начала таймера в базу данных
	messageLog, err := b.db.GetMessageLog(userID, chatID)
	if err != nil {
		b.logger.Errorf("Failed to get message log for timer start: %v", err)
	} else {
		// Сохраняем время начала таймера
		messageLog.TimerStartTime = &timerStartTime
		if err := b.db.SaveMessageLog(messageLog); err != nil {
			b.logger.Errorf("Failed to save timer start time: %v", err)
		} else {
			b.logger.Infof("Saved timer start time: %s", timerStartTime)
		}
	}

	// Рассчитываем время предупреждения (1 минута или половина оставшегося времени)
	warningTime := 1 * time.Minute
	if duration < 2*time.Minute {
		warningTime = duration / 2
	}

	// Запускаем предупреждение
	go func() {
		time.Sleep(warningTime)
		select {
		case <-warningTask:
			return // Таймер отменен
		default:
			b.sendWarning(userID, chatID, username)
		}
	}()

	// Запускаем удаление через указанное время
	go func() {
		time.Sleep(duration)
		select {
		case <-removalTask:
			return // Таймер отменен
		default:
			b.removeUser(userID, chatID, username)
		}
	}()

	b.logger.Infof("Started timer for user %d (%s) - warning in %v, removal in %v", userID, username, warningTime, duration)
}

// cancelTimer отменяет таймер для пользователя
func (b *Bot) cancelTimer(userID int64) {
	if timer, exists := b.timers[userID]; exists {
		close(timer.WarningTask)
		close(timer.RemovalTask)
		delete(b.timers, userID)
		b.logger.Infof("Cancelled timer for user %d", userID)
	}
}

// sendWarning отправляет предупреждение пользователю
func (b *Bot) sendWarning(userID, chatID int64, username string) {
	message := fmt.Sprintf("⚠️ Предупреждение!\n\n@%s, ты не отправлял отчет о тренировке уже 1 минуту!\n\n🦁 Я питаюсь ленивыми леопардами и становлюсь жирнее!\n\n💪 Ты ведь не хочешь стать как я?\n\n⏰ У тебя осталась 1 минута до удаления из чата!\n\n🎯 Отправь #training_done прямо сейчас!", username)

	msg := tgbotapi.NewMessage(chatID, message)
	b.logger.Infof("Sending warning to user %d (%s)", userID, username)
	_, err := b.api.Send(msg)
	if err != nil {
		b.logger.Errorf("Failed to send warning: %v", err)
	} else {
		b.logger.Infof("Successfully sent warning to user %d (%s)", userID, username)
	}
}

// removeUser удаляет пользователя из чата
func (b *Bot) removeUser(userID, chatID int64, username string) {
	b.logger.Infof("Attempting to remove user %d (%s) from chat %d", userID, username, chatID)

	// Пытаемся удалить пользователя из чата
	_, err := b.api.Request(tgbotapi.BanChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: chatID,
			UserID: userID,
		},
		UntilDate: time.Now().Add(30 * 24 * time.Hour).Unix(), // Бан на 30 дней
	})

	if err != nil {
		b.logger.Errorf("Failed to remove user %d: %v", userID, err)
		// Отправляем сообщение об ошибке
		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Не удалось удалить пользователя %s из чата", username))
		b.api.Send(errorMsg)
	} else {
		// Отправляем сообщение об удалении
		message := fmt.Sprintf("🚫 Пользователь удален!\n\n@%s был удален из чата за неактивность.\n\n🦁 Я питаюсь ленивыми леопардами и становлюсь жирнее!\n\n💪 Ты ведь не хочешь стать как я?\n\nТогда тренируйтесь и отправляйте отчеты!", username)
		msg := tgbotapi.NewMessage(chatID, message)
		b.logger.Infof("Sending removal message for user %d (%s)", userID, username)
		_, sendErr := b.api.Send(msg)
		if sendErr != nil {
			b.logger.Errorf("Failed to send removal message: %v", sendErr)
		} else {
			b.logger.Infof("Successfully sent removal message for user %d (%s)", userID, username)
		}
	}
}

package bot

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"leo-bot/internal/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func parseLastMessageTime(lastMessage string) (time.Time, error) {
	if ts, err := utils.ParseMoscowTime(lastMessage); err == nil {
		return ts, nil
	}
	return time.ParseInLocation("2006-01-02 15:04:05", lastMessage, utils.GetMoscowTime().Location())
}

func removalDMText() string {
	return "Ну что, 7 дней без движения — и стая тебя больше не видит.\nТак тоже бывает. XP сгорел, доступ закрыт.\nЕсли захочешь вторую попытку — леопард не будет делать вид, что ничего не было."
}

func removalDMReplyMarkup() *tgbotapi.InlineKeyboardMarkup {
	return &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Вернуться в стаю", paywallCallbackReturnToPack),
			),
		},
	}
}

// kickChatIDForMessage — chat_id строки в training_state, на которой стоит таймер кика.
// В мини-апп-архитектуре всегда работаем по pack-row (pair: user, MonetizedChatID), независимо от того,
// откуда пришло сообщение — из приватного чата с ботом или из (легаси) группы стаи.
// Если MonetizedChatID не настроен — fallback к msg.Chat.ID, чтобы не сломать локальные тесты/окружения без оплаты.
func (b *Bot) kickChatIDForMessage(msg *tgbotapi.Message) int64 {
	if b != nil && b.config != nil && b.config.MonetizedChatID != 0 {
		return b.config.MonetizedChatID
	}
	if msg != nil && msg.Chat != nil {
		return msg.Chat.ID
	}
	return 0
}

// normalizeUserDisplayName убирает лишние ведущие '@' и оставляет одно упоминание для @username.
func normalizeUserDisplayName(username string) string {
	clean := strings.TrimLeft(username, "@")
	if clean == "" {
		return username
	}
	if strings.Contains(clean, " ") {
		return clean
	}
	return "@" + clean
}

// removeUser — «кик» из стаи после 7 дней без движения. Физически из TG-группы никого не выкидываем
// (TG-группы как сущности нет, миграция на мини-апп):
//  1. шлём DM с предложением вернуться;
//  2. аннулируем купленный paywall-доступ — повторный вход только через новую оплату;
//  3. кладём карточку pack_removed в ленту мини-аппа;
//  4. помечаем training_state.is_deleted = true.
func (b *Bot) removeUser(userID, chatID int64, username string) {
	b.logger.Infof("Attempting to remove user %d (%s) from chat %d", userID, username, chatID)

	// Защита от гонки: если #training_done только что был — отменяем удаление.
	messageLog, err := b.db.GetMessageLog(userID, chatID)
	if err == nil && messageLog.HasTrainingDone {
		if lastMessageTime, parseErr := parseLastMessageTime(messageLog.LastMessage); parseErr == nil {
			if utils.GetMoscowTime().Sub(lastMessageTime) < 1*time.Minute {
				b.logger.Infof("User %d (%s) just sent #training_done, cancelling removal", userID, username)
				b.cancelTimer(userID)
				return
			}
		}
	}

	dmStatus := "dm_skipped"
	dmErrorText := ""
	if chatID != userID {
		dmMsg := tgbotapi.NewMessage(userID, removalDMText())
		dmMsg.ReplyMarkup = removalDMReplyMarkup()
		if _, err := b.api.Send(dmMsg); err != nil {
			b.logger.Warnf("send removal DM user=%d: %v", userID, err)
			dmErrorText = err.Error()
			dmStatus = "dm_failed"
			var tgErr *tgbotapi.Error
			if errors.As(err, &tgErr) && tgErr.Code == 403 {
				dmStatus = "dm_blocked"
			}
		} else {
			dmStatus = "dm_sent"
		}
	} else {
		dmStatus = "dm_sent"
	}
	if err := b.db.LogDeletionEvent(userID, chatID, dmStatus, dmErrorText); err != nil {
		b.logger.Errorf("log deletion event user=%d chat=%d: %v", userID, chatID, err)
	}

	if chatID == b.config.MonetizedChatID {
		if expErr := b.db.ExpirePaywallAccessForUser(userID, chatID); expErr != nil {
			b.logger.Errorf("Failed to expire paywall access for inactive user %d: %v", userID, expErr)
		}
		b.savePackRemovedMiniappFeed(chatID, userID, username)
		b.logger.Infof("Removed user %d (%s) from pack: paywall access expired, miniapp feed updated", userID, username)
	} else {
		// chatID != MonetizedChatID — это приватный «message_log в личке» (chatID == userID).
		// Никаких физических действий, только пометка is_deleted (см. ниже) — таймер выключен в DM выше.
		b.logger.Infof("Removed user %d (%s) from non-pack chat %d (private flow only)", userID, username, chatID)
	}

	if err := b.db.MarkUserAsDeleted(userID, chatID); err != nil {
		b.logger.Errorf("Failed to mark user as deleted: %v", err)
	}

	delete(b.timers, userID)
	b.logger.Infof("Timer removed for user %d", userID)
}

// recoverTimersFromDatabase восстанавливает таймеры из базы данных при запуске бота
func (b *Bot) recoverTimersFromDatabase() error {
	b.logger.Info("Recovering timers from database...")

	// Получаем всех пользователей с активными таймерами
	users, err := b.db.GetAllUsersWithTimers()
	if err != nil {
		return fmt.Errorf("failed to get users with timers: %w", err)
	}

	recoveredCount := 0
	for _, user := range users {
		// Дополнительное логирование для диагностики проблем с короткими ID
		b.logger.Infof("Processing user: ID=%d, Username='%s', ChatID=%d, HasSickLeave=%t, HasHealthy=%t, IsDeleted=%t, IsExemptFromDeletion=%t",
			user.UserID, user.Username, user.ChatID, user.HasSickLeave, user.HasHealthy, user.IsDeleted, user.IsExemptFromDeletion)

		// Пропускаем пользователей на больничном
		if user.HasSickLeave && !user.HasHealthy {
			b.logger.Infof("Skipping user %d (%s) - on sick leave", user.UserID, user.Username)
			continue
		}

		// Пропускаем удаленных пользователей
		if user.IsDeleted {
			b.logger.Infof("Skipping user %d (%s) - deleted", user.UserID, user.Username)
			continue
		}

		// Пропускаем пользователей, исключенных из удаления
		if user.IsExemptFromDeletion {
			b.logger.Infof("Skipping user %d (%s) - exempt from deletion", user.UserID, user.Username)
			continue
		}

		// Рассчитываем оставшееся время
		remainingTime := b.calculateRemainingTime(user)
		if remainingTime <= 0 {
			// Время истекло - удаляем пользователя
			b.logger.Infof("Timer expired for user %d (%s), removing from chat", user.UserID, user.Username)
			b.removeUser(user.UserID, user.ChatID, user.Username)
			continue
		}

		// Восстанавливаем таймер без обновления timer_start_time в БД
		if user.TimerStartTime != nil {
			b.restoreTimerWithDuration(user.UserID, user.ChatID, user.Username, remainingTime, *user.TimerStartTime)
		} else {
			// Fallback - если timer_start_time отсутствует, используем обычный старт
			b.startTimerWithDuration(user.UserID, user.ChatID, user.Username, remainingTime)
		}
		recoveredCount++

		b.logger.Infof("Recovered timer for user %d (%s) - remaining time: %v", user.UserID, user.Username, remainingTime)
	}

	b.logger.Infof("Successfully recovered %d timers from database", recoveredCount)
	return nil
}

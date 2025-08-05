package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// getUsername получает имя пользователя из сообщения
func (b *Bot) getUsername(msg *tgbotapi.Message) string {
	if msg.From.UserName != "" {
		return "@" + msg.From.UserName
	} else if msg.From.FirstName != "" {
		username := msg.From.FirstName
		if msg.From.LastName != "" {
			username += " " + msg.From.LastName
		}
		return username
	} else {
		return fmt.Sprintf("User%d", msg.From.ID)
	}
}

// formatDuration форматирует время в читаемый вид
func (b *Bot) formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0f секунд", d.Seconds())
	} else if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%d минут %d секунд", minutes, seconds)
	} else {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		return fmt.Sprintf("%d часов %d минут", hours, minutes)
	}
}

// parseDuration парсит строку времени в Duration
func (b *Bot) parseDuration(durationStr string) (time.Duration, error) {
	// Простой парсинг для формата "1m30s"
	if strings.Contains(durationStr, "m") {
		parts := strings.Split(durationStr, "m")
		if len(parts) == 2 {
			minutes, err := strconv.Atoi(parts[0])
			if err != nil {
				return 0, err
			}
			secondsStr := strings.TrimSuffix(parts[1], "s")
			seconds, err := strconv.Atoi(secondsStr)
			if err != nil {
				return 0, err
			}
			return time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second, nil
		}
	} else if strings.Contains(durationStr, "s") {
		secondsStr := strings.TrimSuffix(durationStr, "s")
		seconds, err := strconv.Atoi(secondsStr)
		if err != nil {
			return 0, err
		}
		return time.Duration(seconds) * time.Second, nil
	}
	return 0, fmt.Errorf("unsupported duration format: %s", durationStr)
}

// validateUser проверяет, является ли пользователь администратором
func (b *Bot) validateUser(msg *tgbotapi.Message) bool {
	// Простая проверка - всегда возвращаем true для упрощения
	// В будущем можно добавить более сложную логику проверки
	return true
}

// sendMessage отправляет сообщение в чат
func (b *Bot) sendMessage(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := b.api.Send(msg)
	if err != nil {
		b.logger.Errorf("Failed to send message: %v", err)
		return err
	}
	return nil
}

// sendReply отправляет ответ на сообщение
func (b *Bot) sendReply(chatID int64, text string, replyToMessageID int) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyToMessageID = replyToMessageID
	_, err := b.api.Send(msg)
	if err != nil {
		b.logger.Errorf("Failed to send reply: %v", err)
		return err
	}
	return nil
}

// isCommand проверяет, является ли сообщение командой
func (b *Bot) isCommand(text string) bool {
	return strings.HasPrefix(text, "/")
}

// extractCommand извлекает команду из текста
func (b *Bot) extractCommand(text string) string {
	if !strings.HasPrefix(text, "/") {
		return ""
	}

	parts := strings.Fields(text)
	if len(parts) == 0 {
		return ""
	}

	return strings.ToLower(parts[0][1:]) // Убираем "/" и приводим к нижнему регистру
}

// extractArguments извлекает аргументы команды
func (b *Bot) extractArguments(text string) []string {
	parts := strings.Fields(text)
	if len(parts) <= 1 {
		return []string{}
	}
	return parts[1:]
}

// hasHashtag проверяет наличие хештега в тексте
func (b *Bot) hasHashtag(text, hashtag string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(hashtag))
}

// extractHashtags извлекает все хештеги из текста
func (b *Bot) extractHashtags(text string) []string {
	words := strings.Fields(text)
	var hashtags []string

	for _, word := range words {
		if strings.HasPrefix(word, "#") {
			hashtags = append(hashtags, strings.ToLower(word))
		}
	}

	return hashtags
}

// formatDate форматирует дату в читаемый вид
func (b *Bot) formatDate(date time.Time) string {
	return date.Format("02.01.2006 15:04")
}

// formatTime форматирует время в читаемый вид
func (b *Bot) formatTime(t time.Time) string {
	return t.Format("15:04")
}

// isToday проверяет, является ли дата сегодняшней
func (b *Bot) isToday(date time.Time) bool {
	now := time.Now()
	return date.Year() == now.Year() && date.YearDay() == now.YearDay()
}

// isYesterday проверяет, является ли дата вчерашней
func (b *Bot) isYesterday(date time.Time) bool {
	yesterday := time.Now().AddDate(0, 0, -1)
	return date.Year() == yesterday.Year() && date.YearDay() == yesterday.YearDay()
}

// daysBetween возвращает количество дней между двумя датами
func (b *Bot) daysBetween(date1, date2 time.Time) int {
	// Нормализуем даты до полуночи
	d1 := time.Date(date1.Year(), date1.Month(), date1.Day(), 0, 0, 0, 0, date1.Location())
	d2 := time.Date(date2.Year(), date2.Month(), date2.Day(), 0, 0, 0, 0, date2.Location())

	diff := d2.Sub(d1)
	return int(diff.Hours() / 24)
}

// createKeyboard создает клавиатуру с кнопками
func (b *Bot) createKeyboard(buttons [][]string) tgbotapi.ReplyKeyboardMarkup {
	var keyboard [][]tgbotapi.KeyboardButton

	for _, row := range buttons {
		var keyboardRow []tgbotapi.KeyboardButton
		for _, button := range row {
			keyboardRow = append(keyboardRow, tgbotapi.NewKeyboardButton(button))
		}
		keyboard = append(keyboard, keyboardRow)
	}

	return tgbotapi.ReplyKeyboardMarkup{
		Keyboard:        keyboard,
		ResizeKeyboard:  true,
		OneTimeKeyboard: false,
	}
}

// createInlineKeyboard создает inline клавиатуру
func (b *Bot) createInlineKeyboard(buttons [][]tgbotapi.InlineKeyboardButton) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(buttons...)
}

// logUserAction логирует действие пользователя
func (b *Bot) logUserAction(userID int64, username, action string) {
	b.logger.Infof("User action - ID: %d, Username: %s, Action: %s", userID, username, action)
}

// logError логирует ошибку с контекстом
func (b *Bot) logError(err error, context string) {
	b.logger.Errorf("Error in %s: %v", context, err)
}

// logInfo логирует информационное сообщение
func (b *Bot) logInfo(message string) {
	b.logger.Info(message)
}

// logWarning логирует предупреждение
func (b *Bot) logWarning(message string) {
	b.logger.Warn(message)
}

// logDebug логирует отладочное сообщение
func (b *Bot) logDebug(message string) {
	b.logger.Debug(message)
}

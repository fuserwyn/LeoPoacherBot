package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleSickLeave обрабатывает команду #sick_leave
func (b *Bot) handleSickLeave(msg *tgbotapi.Message) {
	// Получаем данные пользователя
	messageLog, err := b.db.GetMessageLog(msg.From.ID, msg.Chat.ID)
	if err != nil {
		b.logger.Errorf("Failed to get message log: %v", err)
		return
	}

	// Записываем время начала больничного
	sickLeaveStartTime := time.Now().Format(time.RFC3339)
	messageLog.SickLeaveStartTime = &sickLeaveStartTime
	b.logger.Infof("Set sick leave start time: %s", sickLeaveStartTime)

	// Рассчитываем оставшееся время до удаления
	fullTimerDuration := 2 * time.Minute // Для тестирования
	var remainingTime time.Duration

	if messageLog.TimerStartTime != nil {
		timerStart, err := time.Parse(time.RFC3339, *messageLog.TimerStartTime)
		if err == nil {
			sickStart, err := time.Parse(time.RFC3339, sickLeaveStartTime)
			if err == nil {
				// Время с тренировки до начала болезни
				timeFromTrainingToSick := sickStart.Sub(timerStart)
				// Оставшееся время = полное время - время с тренировки до болезни
				remainingTime = fullTimerDuration - timeFromTrainingToSick
				if remainingTime <= 0 {
					remainingTime = 0
				}
				b.logger.Infof("Timer start: %v, sick start: %v, time from training to sick: %v, remaining time: %v", timerStart, sickStart, timeFromTrainingToSick, remainingTime)
			} else {
				remainingTime = fullTimerDuration
				b.logger.Errorf("Failed to parse sick start time: %v", err)
			}
		} else {
			remainingTime = fullTimerDuration
			b.logger.Errorf("Failed to parse timer start time: %v", err)
		}
	} else {
		remainingTime = fullTimerDuration
		b.logger.Warnf("Timer start time is nil, using full duration")
	}

	// Сохраняем остаток времени
	restTimeStr := remainingTime.String()
	messageLog.RestTimeTillDel = &restTimeStr
	b.logger.Infof("Saved rest time till deletion: %s", restTimeStr)

	// Обновляем флаги больничного
	messageLog.HasSickLeave = true
	messageLog.HasHealthy = false

	// Добавляем подробное логирование перед сохранением
	b.logger.Infof("Saving message log with fields:")
	b.logger.Infof("  UserID: %d", messageLog.UserID)
	b.logger.Infof("  ChatID: %d", messageLog.ChatID)
	b.logger.Infof("  HasSickLeave: %t", messageLog.HasSickLeave)
	b.logger.Infof("  HasHealthy: %t", messageLog.HasHealthy)
	b.logger.Infof("  SickLeaveStartTime: %s", func() string {
		if messageLog.SickLeaveStartTime != nil {
			return *messageLog.SickLeaveStartTime
		}
		return "nil"
	}())
	b.logger.Infof("  RestTimeTillDel: %s", func() string {
		if messageLog.RestTimeTillDel != nil {
			return *messageLog.RestTimeTillDel
		}
		return "nil"
	}())

	if err := b.db.SaveMessageLog(messageLog); err != nil {
		b.logger.Errorf("Failed to update message log: %v", err)
	} else {
		b.logger.Infof("Successfully saved sick leave start time")
	}

	// Отменяем существующие таймеры
	b.cancelTimer(msg.From.ID)

	// Отправляем подтверждение
	reply := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("🏥 Больничный принят! 🤒\n\n⏸️ Таймер приостановлен на время болезни\n\n⏰ Остаток времени до удаления: %v\n\n💪 Выздоравливай и возвращайся к тренировкам!\n\n📝 Когда поправишься, отправь #healthy для возобновления таймера", remainingTime))

	b.logger.Infof("Sending sick leave message to chat %d", msg.Chat.ID)
	_, err = b.api.Send(reply)
	if err != nil {
		b.logger.Errorf("Failed to send sick leave message: %v", err)
	} else {
		b.logger.Infof("Successfully sent sick leave message to chat %d", msg.Chat.ID)
	}
}

// handleHealthy обрабатывает команду #healthy
func (b *Bot) handleHealthy(msg *tgbotapi.Message) {
	messageLog, err := b.db.GetMessageLog(msg.From.ID, msg.Chat.ID)
	if err != nil {
		b.logger.Errorf("Failed to get message log: %v", err)
		return
	}

	// Записываем время окончания больничного
	sickLeaveEndTime := time.Now().Format(time.RFC3339)
	messageLog.SickLeaveEndTime = &sickLeaveEndTime
	b.logger.Infof("Set sick leave end time: %s", sickLeaveEndTime)

	// Рассчитываем время болезни
	if messageLog.SickLeaveStartTime != nil {
		b.logger.Infof("Sick leave start time: %s", *messageLog.SickLeaveStartTime)
		sickStart, err := time.Parse(time.RFC3339, *messageLog.SickLeaveStartTime)
		if err == nil {
			sickEnd, err := time.Parse(time.RFC3339, sickLeaveEndTime)
			if err == nil {
				sickDuration := sickEnd.Sub(sickStart)
				sickTimeStr := sickDuration.String()
				messageLog.SickTime = &sickTimeStr
				b.logger.Infof("Calculated sick time: %v (%s)", sickDuration, sickTimeStr)
			} else {
				b.logger.Errorf("Failed to parse sick end time: %v", err)
			}
		} else {
			b.logger.Errorf("Failed to parse sick start time: %v", err)
		}
	} else {
		b.logger.Warnf("Sick leave start time is nil")
	}

	// Обновляем флаг выздоровления
	messageLog.HasHealthy = true

	// Добавляем подробное логирование перед сохранением
	b.logger.Infof("Saving message log with fields:")
	b.logger.Infof("  UserID: %d", messageLog.UserID)
	b.logger.Infof("  ChatID: %d", messageLog.ChatID)
	b.logger.Infof("  HasSickLeave: %t", messageLog.HasSickLeave)
	b.logger.Infof("  HasHealthy: %t", messageLog.HasHealthy)
	b.logger.Infof("  SickLeaveStartTime: %s", func() string {
		if messageLog.SickLeaveStartTime != nil {
			return *messageLog.SickLeaveStartTime
		}
		return "nil"
	}())
	b.logger.Infof("  SickLeaveEndTime: %s", func() string {
		if messageLog.SickLeaveEndTime != nil {
			return *messageLog.SickLeaveEndTime
		}
		return "nil"
	}())
	b.logger.Infof("  SickTime: %s", func() string {
		if messageLog.SickTime != nil {
			return *messageLog.SickTime
		}
		return "nil"
	}())
	b.logger.Infof("  RestTimeTillDel: %s", func() string {
		if messageLog.RestTimeTillDel != nil {
			return *messageLog.RestTimeTillDel
		}
		return "nil"
	}())

	if err := b.db.SaveMessageLog(messageLog); err != nil {
		b.logger.Errorf("Failed to update message log: %v", err)
	} else {
		b.logger.Infof("Successfully saved message log with sick leave data")
	}

	// Используем сохраненный остаток времени
	var remainingTime time.Duration
	if messageLog.RestTimeTillDel != nil {
		// Парсим сохраненное время
		restTimeStr := *messageLog.RestTimeTillDel
		b.logger.Infof("Parsing rest time: %s", restTimeStr)
		// Простой парсинг для тестирования (формат "1m30s")
		if strings.Contains(restTimeStr, "m") {
			parts := strings.Split(restTimeStr, "m")
			if len(parts) == 2 {
				minutes, _ := strconv.Atoi(parts[0])
				secondsStr := strings.TrimSuffix(parts[1], "s")
				seconds, _ := strconv.Atoi(secondsStr)
				remainingTime = time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
				b.logger.Infof("Parsed rest time: %dm %ds = %v", minutes, seconds, remainingTime)
			}
		} else if strings.Contains(restTimeStr, "s") {
			secondsStr := strings.TrimSuffix(restTimeStr, "s")
			seconds, _ := strconv.Atoi(secondsStr)
			remainingTime = time.Duration(seconds) * time.Second
			b.logger.Infof("Parsed rest time: %ds = %v", seconds, remainingTime)
		}
	} else {
		// Если нет сохраненного времени, используем полный таймер
		remainingTime = 2 * time.Minute
		b.logger.Warnf("No rest time saved, using full timer: %v", remainingTime)
	}

	// Проверяем, не истекло ли время
	if remainingTime <= 0 {
		// Время истекло - удаляем пользователя
		username := ""
		if msg.From.UserName != "" {
			username = "@" + msg.From.UserName
		} else if msg.From.FirstName != "" {
			username = msg.From.FirstName
			if msg.From.LastName != "" {
				username += " " + msg.From.LastName
			}
		} else {
			username = fmt.Sprintf("User%d", msg.From.ID)
		}

		// Отправляем сообщение об истечении времени
		reply := tgbotapi.NewMessage(msg.Chat.ID, "⏰ Время истекло! 🚫\n\n💪 Выздоровление принято, но время таймера уже истекло.\n\n🦁 Я питаюсь ленивыми леопардами и становлюсь жирнее!\n\n💪 Ты ведь не хочешь стать как я?\n\nТогда тренируйтесь и отправляйте отчеты!")
		b.api.Send(reply)

		// Удаляем пользователя
		b.removeUser(msg.From.ID, msg.Chat.ID, username)
		return
	}

	// Запускаем таймер с оставшимся временем
	b.startTimerWithDuration(msg.From.ID, msg.Chat.ID, msg.From.UserName, remainingTime)

	// Отправляем подтверждение
	reply := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("💪 Выздоровление принято! 🎉\n\n⏰ Таймер возобновлён с места остановки!\n\n⏰ Остаток времени: %v", remainingTime))

	b.logger.Infof("Sending healthy message to chat %d", msg.Chat.ID)
	_, err = b.api.Send(reply)
	if err != nil {
		b.logger.Errorf("Failed to send healthy message: %v", err)
	} else {
		b.logger.Infof("Successfully sent healthy message to chat %d", msg.Chat.ID)
	}
}

// handleTrainingDone обрабатывает команду #training_done
func (b *Bot) handleTrainingDone(msg *tgbotapi.Message) {
	// Получаем данные пользователя
	messageLog, err := b.db.GetMessageLog(msg.From.ID, msg.Chat.ID)
	if err != nil {
		b.logger.Errorf("Failed to get message log: %v", err)
		return
	}

	// Проверяем, не было ли уже тренировки сегодня
	today := time.Now().Format("2006-01-02")
	if messageLog.LastTrainingDate != nil && *messageLog.LastTrainingDate == today {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "🏋️ Тренировка уже засчитана сегодня! 💪\n\nЗавтра снова в бой! 💪")
		b.api.Send(reply)
		return
	}

	// Рассчитываем калории и streak
	calories, streakDays := b.calculateCalories(messageLog)

	// Обновляем данные пользователя
	messageLog.Calories = calories
	messageLog.StreakDays = streakDays
	messageLog.HasTrainingDone = true
	messageLog.HasSickLeave = false
	messageLog.HasHealthy = false
	messageLog.LastTrainingDate = &today

	// Сохраняем в базу данных
	if err := b.db.SaveMessageLog(messageLog); err != nil {
		b.logger.Errorf("Failed to save message log: %v", err)
		return
	}

	// Обновляем streak в базе данных
	if err := b.db.UpdateStreak(msg.From.ID, msg.Chat.ID, streakDays, today); err != nil {
		b.logger.Errorf("Failed to update streak: %v", err)
	}

	// Формируем сообщение
	username := ""
	if msg.From.UserName != "" {
		username = "@" + msg.From.UserName
	} else if msg.From.FirstName != "" {
		username = msg.From.FirstName
		if msg.From.LastName != "" {
			username += " " + msg.From.LastName
		}
	} else {
		username = fmt.Sprintf("User%d", msg.From.ID)
	}

	// Запускаем таймер
	timerDuration := 2 * time.Minute // Для тестирования
	b.startTimerWithDuration(msg.From.ID, msg.Chat.ID, msg.From.UserName, timerDuration)

	message := fmt.Sprintf("🏋️ Тренировка завершена! 💪\n\n👤 %s\n🔥 Сожжено калорий: %d\n📈 Серия дней: %d\n\n⏰ Таймер перезапущен на %s\n\n🎯 Продолжай в том же духе! 💪",
		username, calories, streakDays, b.formatDuration(timerDuration))

	reply := tgbotapi.NewMessage(msg.Chat.ID, message)
	b.logger.Infof("Sending training done message to chat %d", msg.Chat.ID)
	_, err = b.api.Send(reply)
	if err != nil {
		b.logger.Errorf("Failed to send training done message: %v", err)
	} else {
		b.logger.Infof("Successfully sent training done message to chat %d", msg.Chat.ID)
	}
}

// handleStart обрабатывает команду /start
func (b *Bot) handleStart(msg *tgbotapi.Message) {
	message := `🦁 Добро пожаловать в Leo Poacher Bot! 🦁

💪 Я помогу тебе оставаться в форме и не стать жирным леопардом!

📋 Доступные команды:
/help - Показать справку
/top - Показать топ пользователей
/points - Показать свои калории

🎯 Как использовать:
1. Отправь #training_done после тренировки
2. Если заболел, отправь #sick_leave
3. Когда выздоровел, отправь #healthy

💪 Тренируйся каждый день и не становись жирным леопардом! 🦁`

	reply := tgbotapi.NewMessage(msg.Chat.ID, message)
	b.api.Send(reply)
}

// handleHelp обрабатывает команду /help
func (b *Bot) handleHelp(msg *tgbotapi.Message) {
	message := `🦁 Leo Poacher Bot - Справка 🦁

📋 Доступные команды:
/start - Начать работу с ботом
/help - Показать эту справку
/top - Показать топ пользователей по калориям
/points - Показать свои калории
/db - Показать статистику базы данных

🎯 Хештеги:
#training_done - Отметить завершение тренировки
#sick_leave - Взять больничный (приостанавливает таймер)
#healthy - Завершить больничный (возобновляет таймер)

💪 Система калорий:
• Каждая тренировка: +1 калория
• Серия 3 дня: +2 калории
• Серия 7 дней: +5 калорий
• Серия 14 дней: +10 калорий
• Серия 30 дней: +20 калорий
• Возвращение после больничного: +2 калории

⏰ Таймер:
• После #training_done запускается таймер
• Если не отправить #training_done вовремя - удаление из чата
• #sick_leave приостанавливает таймер
• #healthy возобновляет таймер с оставшегося времени

💪 Тренируйся каждый день и не становись жирным леопардом! 🦁`

	reply := tgbotapi.NewMessage(msg.Chat.ID, message)
	b.api.Send(reply)
}

// handleDB обрабатывает команду /db
func (b *Bot) handleDB(msg *tgbotapi.Message) {
	stats, err := b.db.GetDatabaseStats()
	if err != nil {
		b.logger.Errorf("Failed to get database stats: %v", err)
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Ошибка получения статистики базы данных")
		b.api.Send(reply)
		return
	}

	message := fmt.Sprintf("📊 Статистика базы данных:\n\n👥 Всего пользователей: %d\n💪 Тренировались: %d\n🏥 На больничном: %d\n✅ Здоровы: %d",
		stats["total_users"], stats["training_done"], stats["sick_leave"], stats["healthy"])

	reply := tgbotapi.NewMessage(msg.Chat.ID, message)
	b.api.Send(reply)
}

// handleTop обрабатывает команду /top
func (b *Bot) handleTop(msg *tgbotapi.Message) {
	users, err := b.db.GetTopUsers(msg.Chat.ID, 10)
	if err != nil {
		b.logger.Errorf("Failed to get top users: %v", err)
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Ошибка получения топ пользователей")
		b.api.Send(reply)
		return
	}

	if len(users) == 0 {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "📊 Пока нет данных для топ пользователей")
		b.api.Send(reply)
		return
	}

	message := "🏆 Топ пользователей по калориям:\n\n"
	for i, user := range users {
		username := user.Username
		if username == "" {
			username = fmt.Sprintf("User%d", user.UserID)
		}
		message += fmt.Sprintf("%d. %s - %d калорий (серия: %d дней)\n", i+1, username, user.Calories, user.StreakDays)
	}

	reply := tgbotapi.NewMessage(msg.Chat.ID, message)
	b.api.Send(reply)
}

// handlePoints обрабатывает команду /points
func (b *Bot) handlePoints(msg *tgbotapi.Message) {
	calories, err := b.db.GetUserCalories(msg.From.ID, msg.Chat.ID)
	if err != nil {
		b.logger.Errorf("Failed to get user calories: %v", err)
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Ошибка получения калорий")
		b.api.Send(reply)
		return
	}

	messageLog, err := b.db.GetMessageLog(msg.From.ID, msg.Chat.ID)
	if err != nil {
		b.logger.Errorf("Failed to get message log: %v", err)
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Ошибка получения данных пользователя")
		b.api.Send(reply)
		return
	}

	username := ""
	if msg.From.UserName != "" {
		username = "@" + msg.From.UserName
	} else if msg.From.FirstName != "" {
		username = msg.From.FirstName
		if msg.From.LastName != "" {
			username += " " + msg.From.LastName
		}
	} else {
		username = fmt.Sprintf("User%d", msg.From.ID)
	}

	message := fmt.Sprintf("💰 Статистика пользователя %s:\n\n🔥 Всего сожжено калорий: %d\n📈 Серия дней: %d\n\n💪 Продолжай тренироваться!", username, calories, messageLog.StreakDays)

	reply := tgbotapi.NewMessage(msg.Chat.ID, message)
	b.api.Send(reply)
}

// handleStartTimer обрабатывает команду /start_timer
func (b *Bot) handleStartTimer(msg *tgbotapi.Message) {
	username := ""
	if msg.From.UserName != "" {
		username = "@" + msg.From.UserName
	} else if msg.From.FirstName != "" {
		username = msg.From.FirstName
		if msg.From.LastName != "" {
			username += " " + msg.From.LastName
		}
	} else {
		username = fmt.Sprintf("User%d", msg.From.ID)
	}

	timerDuration := 2 * time.Minute // Для тестирования
	b.startTimerWithDuration(msg.From.ID, msg.Chat.ID, username, timerDuration)

	reply := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("⏰ Таймер запущен! 🦁\n\n⏰ У тебя %s до удаления!\n\n💪 Отправь #training_done, чтобы остановить таймер!", b.formatDuration(timerDuration)))
	b.api.Send(reply)
}

package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"leo-bot/internal/config"
	"leo-bot/internal/database"
	"leo-bot/internal/logger"
	"leo-bot/internal/models"
	"leo-bot/internal/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api    *tgbotapi.BotAPI
	db     *database.Database
	logger logger.Logger
	config *config.Config
	timers map[int64]*models.TimerInfo
}

func New(cfg *config.Config, db *database.Database, log logger.Logger) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.APIToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	// Создаем таблицы в базе данных
	if err := db.CreateTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return &Bot{
		api:    api,
		db:     db,
		logger: log,
		config: cfg,
		timers: make(map[int64]*models.TimerInfo),
	}, nil
}

func (b *Bot) Start(ctx context.Context) error {
	b.logger.Info("Starting bot...")

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case update := <-updates:
			go b.handleUpdate(update)
		case <-ctx.Done():
			b.logger.Info("Bot stopped")
			return nil
		}
	}
}

func (b *Bot) handleUpdate(update tgbotapi.Update) {
	// Обрабатываем добавление новых участников
	if update.Message != nil && len(update.Message.NewChatMembers) > 0 {
		b.handleNewChatMembers(update.Message)
		return
	}

	if update.Message == nil {
		return
	}

	msg := update.Message
	b.logger.Infof("Received message from %d: %s", msg.From.ID, msg.Text)

	// Обрабатываем команды
	if msg.IsCommand() {
		b.handleCommand(msg)
		return
	}

	// Обрабатываем обычные сообщения
	b.handleMessage(msg)
}

func (b *Bot) handleCommand(msg *tgbotapi.Message) {
	command := msg.Command()
	_ = msg.CommandArguments() // Игнорируем аргументы пока

	switch command {
	case "start":
		b.handleStart(msg)
	case "start_timer":
		b.handleStartTimer(msg)
	case "help":
		b.handleHelp(msg)
	case "db":
		b.handleDB(msg)
	case "top":
		b.handleTop(msg)
	case "points":
		b.handlePoints(msg)
	case "cups":
		b.handleCups(msg)
	case "send_to_chat":
		b.handleSendToChat(msg)
	default:
		b.logger.Warnf("Unknown command: %s", command)
	}
}

func (b *Bot) handleNewChatMembers(msg *tgbotapi.Message) {
	// Отправляем приветственное сообщение для каждого нового участника
	for _, newMember := range msg.NewChatMembers {
		// Пропускаем ботов
		if newMember.IsBot {
			continue
		}

		// Получаем никнейм пользователя
		username := ""
		if newMember.UserName != "" {
			username = "@" + newMember.UserName
		} else if newMember.FirstName != "" {
			username = newMember.FirstName
			if newMember.LastName != "" {
				username += " " + newMember.LastName
			}
		} else {
			username = fmt.Sprintf("User%d", newMember.ID)
		}

		// Отправляем приветственное сообщение
		b.sendWelcomeMessage(msg.Chat.ID, username, newMember.ID)
	}
}

func (b *Bot) sendWelcomeMessage(chatID int64, username string, userID int64) {
	// Создаем приветственное сообщение с упоминанием пользователя
	welcomeText := fmt.Sprintf(`%s, добро пожаловать в стаю! 🦁

Я ваш хладнокровный тренер, который следит за тренировками всегда, я все вижу и не оставляю в стае тех, кто не занимается больше 7 дней!

💪 Отчеты о тренировке:
• #training_done — Отправить отчет о тренировке

🏥 Больничный:
• #sick_leave — Взять больничный (приостанавливает таймер)
• #healthy — Выздороветь (возобновляет таймер)

⏰ Как я слежу за тренировками:
• При получении #training_done таймер перезапускается на 7 дней
• Через 6 дней без #training_done - предупреждение
• Через 7 дней без #training_done - удаление из чата

📋 Правила:
• Отчётом считается любое сообщение с тегом #training_done
• Если заболели — отправь #sick_leave
• После выздоровления — отправь #healthy
• Через 6 дней без отчёта — предупреждение
• Через 7 дней без отчёта — удаление из чата

🦁`, username)

	// Отправляем сообщение
	reply := tgbotapi.NewMessage(chatID, welcomeText)

	b.logger.Infof("Sending welcome message to chat %d for new user %s (ID: %d)", chatID, username, userID)
	_, err := b.api.Send(reply)
	if err != nil {
		b.logger.Errorf("Failed to send welcome message: %v", err)
	} else {
		b.logger.Infof("Successfully sent welcome message to chat %d for new user %s", chatID, username)
	}

	// Запускаем таймер для нового пользователя
	b.startTimer(userID, chatID, username)
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	// Проверяем наличие хештегов в тексте или подписи
	text := msg.Text
	if text == "" && msg.Caption != "" {
		text = msg.Caption
	}

	hasTrainingDone := strings.Contains(strings.ToLower(text), "#training_done")
	hasSickLeave := strings.Contains(strings.ToLower(text), "#sick_leave")
	hasHealthy := strings.Contains(strings.ToLower(text), "#healthy")

	// Получаем никнейм пользователя
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

	// Получаем существующие данные пользователя
	existingLog, err := b.db.GetMessageLog(msg.From.ID, msg.Chat.ID)
	if err != nil {
		// Если пользователя нет в БД, создаем новую запись
		messageLog := &models.MessageLog{
			UserID:          msg.From.ID,
			ChatID:          msg.Chat.ID,
			Username:        username,
			Calories:        0,
			StreakDays:      0,
			LastMessage:     utils.FormatMoscowTime(utils.GetMoscowTime()),
			HasTrainingDone: hasTrainingDone,
			HasSickLeave:    hasSickLeave,
			HasHealthy:      hasHealthy,
			IsDeleted:       false,
		}

		if err := b.db.SaveMessageLog(messageLog); err != nil {
			b.logger.Errorf("Failed to save message log: %v", err)
		}
	} else {
		// Обновляем только необходимые поля, сохраняя streak данные
		existingLog.Username = username
		existingLog.LastMessage = utils.FormatMoscowTime(utils.GetMoscowTime())
		existingLog.HasTrainingDone = hasTrainingDone
		existingLog.HasSickLeave = hasSickLeave
		existingLog.HasHealthy = hasHealthy
		existingLog.IsDeleted = false

		if err := b.db.SaveMessageLog(existingLog); err != nil {
			b.logger.Errorf("Failed to update message log: %v", err)
		}
	}

	// Обрабатываем хештеги
	if hasTrainingDone {
		b.handleTrainingDone(msg)
	} else if hasSickLeave {
		b.handleSickLeave(msg)
	} else if hasHealthy {
		b.handleHealthy(msg)
	}
}

func (b *Bot) handleTrainingDone(msg *tgbotapi.Message) {
	// Получаем никнейм пользователя
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

	// Сохраняем отчет о тренировке
	trainingLog := &models.TrainingLog{
		UserID:     msg.From.ID,
		Username:   username,
		LastReport: utils.FormatMoscowTime(utils.GetMoscowTime()),
	}

	if err := b.db.SaveTrainingLog(trainingLog); err != nil {
		b.logger.Errorf("Failed to save training log: %v", err)
	}

	// Получаем текущие данные пользователя
	messageLog, err := b.db.GetMessageLog(msg.From.ID, msg.Chat.ID)
	if err != nil {
		b.logger.Errorf("Failed to get message log: %v", err)
		return
	}

	// Рассчитываем калории и серию
	caloriesToAdd, newStreakDays, weeklyAchievement, twoWeekAchievement, threeWeekAchievement, monthlyAchievement, quarterlyAchievement := b.calculateCalories(messageLog)

	// ДЕБАГ: Логируем результат расчета
	b.logger.Infof("DEBUG handleTrainingDone: caloriesToAdd=%d, newStreakDays=%d", caloriesToAdd, newStreakDays)

	// Начисляем калории
	if err := b.db.AddCalories(msg.From.ID, msg.Chat.ID, caloriesToAdd); err != nil {
		b.logger.Errorf("Failed to add calories: %v", err)
	} else {
		b.logger.Infof("DEBUG: Successfully added %d calories", caloriesToAdd)
	}

	// Обновляем серию только если была добавлена новая тренировка
	if caloriesToAdd > 0 {
		today := utils.GetMoscowDate()
		b.logger.Infof("DEBUG: Updating streak to %d with date %s", newStreakDays, today)
		if err := b.db.UpdateStreak(msg.From.ID, msg.Chat.ID, newStreakDays, today); err != nil {
			b.logger.Errorf("Failed to update streak: %v", err)
		} else {
			b.logger.Infof("DEBUG: Successfully updated streak to %d", newStreakDays)
		}
	} else {
		b.logger.Infof("DEBUG: Skipping streak update (caloriesToAdd = 0)")
	}

	// Проверяем, был ли пользователь на больничном
	wasOnSickLeave := messageLog.HasSickLeave && !messageLog.HasHealthy

	// Отправляем достижения только если была добавлена новая тренировка
	if caloriesToAdd > 0 {
		// Начисляем и отправляем 42 кубка за недельную серию
		if weeklyAchievement {
			if err := b.db.AddCups(msg.From.ID, msg.Chat.ID, 42); err != nil {
				b.logger.Errorf("Failed to add weekly cups: %v", err)
			} else {
				b.logger.Infof("Successfully added 42 cups for weekly achievement")
			}
			b.sendWeeklyCupsReward(msg, username, newStreakDays)
		}

		// Начисляем и отправляем 42 кубка за двухнедельную серию
		if twoWeekAchievement {
			if err := b.db.AddCups(msg.From.ID, msg.Chat.ID, 42); err != nil {
				b.logger.Errorf("Failed to add two-week cups: %v", err)
			} else {
				b.logger.Infof("Successfully added 42 cups for two-week achievement")
			}
			b.sendTwoWeekCupsReward(msg, username, newStreakDays)
		}

		// Начисляем и отправляем 42 кубка за трехнедельную серию
		if threeWeekAchievement {
			if err := b.db.AddCups(msg.From.ID, msg.Chat.ID, 42); err != nil {
				b.logger.Errorf("Failed to add three-week cups: %v", err)
			} else {
				b.logger.Infof("Successfully added 42 cups for three-week achievement")
			}
			b.sendThreeWeekCupsReward(msg, username, newStreakDays)
		}

		// Начисляем и отправляем 420 кубков за месячную серию
		if monthlyAchievement {
			if err := b.db.AddCups(msg.From.ID, msg.Chat.ID, 420); err != nil {
				b.logger.Errorf("Failed to add monthly cups: %v", err)
			} else {
				b.logger.Infof("Successfully added 420 cups for monthly achievement")
			}
			b.sendMonthlyCupsReward(msg, username, newStreakDays)
		}

		// Начисляем и отправляем 4200 кубков за квартальную серию
		if quarterlyAchievement {
			if err := b.db.AddCups(msg.From.ID, msg.Chat.ID, 4200); err != nil {
				b.logger.Errorf("Failed to add quarterly cups: %v", err)
			} else {
				b.logger.Infof("Successfully added 4200 cups for quarterly achievement")
			}
			b.sendQuarterlyCupsReward(msg, username, newStreakDays)
		}

		// Проверяем супер-уровень после начисления кубков
		totalCups, err := b.db.GetUserCups(msg.From.ID, msg.Chat.ID)
		if err != nil {
			b.logger.Errorf("Failed to get user cups for super level check: %v", err)
		} else if totalCups > 420 {
			// Отправляем сообщение о супер-уровне
			b.sendSuperLevelMessage(msg, username, totalCups)
		}
	}

	// Отправляем подтверждение только если была добавлена новая тренировка
	// И только если не было отправлено сообщение о кубках
	if caloriesToAdd > 0 && !weeklyAchievement && !twoWeekAchievement && !threeWeekAchievement && !monthlyAchievement && !quarterlyAchievement {
		reply := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("✅ Отчёт принят! 💪\n\n🦁 Ты тренируешься дней подряд: %d\n\n⏰ Таймер перезапускается на 7 дней\n\n🎯 Продолжай тренироваться и не забывай отправлять #training_done!", newStreakDays))

		b.logger.Infof("Sending training done message to chat %d", msg.Chat.ID)
		_, err = b.api.Send(reply)
		if err != nil {
			b.logger.Errorf("Failed to send training done message: %v", err)
		} else {
			b.logger.Infof("Successfully sent training done message to chat %d", msg.Chat.ID)
		}
	} else if caloriesToAdd == 0 {
		// Если тренировка уже была сегодня, отправляем мотивирующее сообщение
		reply := tgbotapi.NewMessage(msg.Chat.ID, "🦁 Какой мотивированный леопард! Еще одна тренировка сегодня! 💪\n\n🔥 Твоя мотивация впечатляет\n\n⏰ Таймер уже перезапущен на 7 дней\n\n🎯 Завтра снова отправляй #training_done для продолжения серии!")

		b.logger.Infof("Sending already trained today message to chat %d", msg.Chat.ID)
		_, err = b.api.Send(reply)
		if err != nil {
			b.logger.Errorf("Failed to send already trained today message: %v", err)
		} else {
			b.logger.Infof("Successfully sent already trained today message to chat %d", msg.Chat.ID)
		}
	}
	// Если caloriesToAdd > 0 И есть achievement (кубки), то ничего дополнительного не отправляем

	// Если пользователь был на больничном, сбрасываем флаги больничного и помечаем как здорового
	if wasOnSickLeave {
		messageLog.HasSickLeave = false
		messageLog.HasHealthy = true
		messageLog.SickLeaveStartTime = nil
		if err := b.db.SaveMessageLog(messageLog); err != nil {
			b.logger.Errorf("Failed to reset sick leave flags: %v", err)
		}
		b.logger.Infof("Reset sick leave flags and marked as healthy for user %d (%s) after training during sick leave", msg.From.ID, username)
	}

	// Запускаем новый таймер
	b.startTimer(msg.From.ID, msg.Chat.ID, msg.From.UserName)
}

func (b *Bot) handleSickLeave(msg *tgbotapi.Message) {
	// Получаем данные пользователя
	messageLog, err := b.db.GetMessageLog(msg.From.ID, msg.Chat.ID)
	if err != nil {
		b.logger.Errorf("Failed to get message log: %v", err)
		return
	}

	// Записываем время начала больничного
	sickLeaveStartTime := utils.FormatMoscowTime(utils.GetMoscowTime())
	messageLog.SickLeaveStartTime = &sickLeaveStartTime
	b.logger.Infof("Set sick leave start time: %s", sickLeaveStartTime)

	// Рассчитываем оставшееся время до удаления
	fullTimerDuration := 2 * time.Minute // Для тестирования
	var remainingTime time.Duration

	if messageLog.TimerStartTime != nil {
		timerStart, err := utils.ParseMoscowTime(*messageLog.TimerStartTime)
		if err == nil {
			sickStart, err := utils.ParseMoscowTime(sickLeaveStartTime)
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
	reply := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("🏥 Больничный принят! 🤒\n\n⏸️ Таймер приостановлен на время болезни\n\n💪 Выздоравливай и возвращайся к тренировкам!\n\n📝 Когда поправишься, отправь #healthy для возобновления таймера"))

	b.logger.Infof("Sending sick leave message to chat %d", msg.Chat.ID)
	_, err = b.api.Send(reply)
	if err != nil {
		b.logger.Errorf("Failed to send sick leave message: %v", err)
	} else {
		b.logger.Infof("Successfully sent sick leave message to chat %d", msg.Chat.ID)
	}
}

func (b *Bot) handleHealthy(msg *tgbotapi.Message) {
	// Получаем данные о времени таймера и больничного
	messageLog, err := b.db.GetMessageLog(msg.From.ID, msg.Chat.ID)
	if err != nil {
		b.logger.Errorf("Failed to get message log: %v", err)
		return
	}

	// Записываем время окончания больничного
	sickLeaveEndTime := utils.FormatMoscowTime(utils.GetMoscowTime())
	messageLog.SickLeaveEndTime = &sickLeaveEndTime
	b.logger.Infof("Set sick leave end time: %s", sickLeaveEndTime)

	// Рассчитываем время болезни
	if messageLog.SickLeaveStartTime != nil {
		b.logger.Infof("Sick leave start time: %s", *messageLog.SickLeaveStartTime)
		sickStart, err := utils.ParseMoscowTime(*messageLog.SickLeaveStartTime)
		if err == nil {
			sickEnd, err := utils.ParseMoscowTime(sickLeaveEndTime)
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
		remainingTime = 7 * 24 * time.Hour
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
	reply := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("💪 Выздоровление принято! 🎉\n\n⏰ Таймер возобновлён с места остановки!"))

	b.logger.Infof("Sending healthy message to chat %d", msg.Chat.ID)
	_, err = b.api.Send(reply)
	if err != nil {
		b.logger.Errorf("Failed to send healthy message: %v", err)
	} else {
		b.logger.Infof("Successfully sent healthy message to chat %d", msg.Chat.ID)
	}
}

func (b *Bot) handleStartTimer(msg *tgbotapi.Message) {
	// Проверяем права администратора
	if !b.isAdmin(msg.Chat.ID, msg.From.ID) {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Только администраторы или владелец могут использовать эту команду!")
		b.api.Send(reply)
		return
	}

	// Получаем всех пользователей в чате
	users, err := b.db.GetUsersByChatID(msg.Chat.ID)
	if err != nil {
		b.logger.Errorf("Failed to get users: %v", err)
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Ошибка при получении пользователей")
		b.api.Send(reply)
		return
	}

	// Запускаем таймеры для всех пользователей
	startedCount := 0
	for _, user := range users {
		if b.isUserInChat(msg.Chat.ID, user.UserID) {
			b.startTimer(user.UserID, msg.Chat.ID, "")
			startedCount++
		}
	}

	// Отправляем отчет
	reply := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("🐆 Fat Leopard активирован!\n\n⏱️ Запущено таймеров: %d\n⏰ Время: 7 дней\n💪 Действие: Отправь #training_done", startedCount))

	b.logger.Infof("Sending start timer message to chat %d", msg.Chat.ID)
	_, err = b.api.Send(reply)
	if err != nil {
		b.logger.Errorf("Failed to send start timer message: %v", err)
	} else {
		b.logger.Infof("Successfully sent start timer message to chat %d", msg.Chat.ID)
	}
}

func (b *Bot) handleHelp(msg *tgbotapi.Message) {
	helpText := `🤖 LeoPoacherBot - Команды:

📝 Команды администратора:
• /start_timer — Запустить таймеры для всех пользователей
• /db — Показать статистику БД
• /help — Показать это сообщение

🏆 Команды пользователей:
• /top — Показать топ пользователей по калориям
• /points — Показать ваши калории
• /cups — Показать ваши заработанные кубки

💪 Отчеты о тренировке:
• #training_done — Отправить отчет о тренировке

🏥 Больничный:
• #sick_leave — Взять больничный (приостанавливает таймер)
• #healthy — Выздороветь (возобновляет таймер)

⏰ Как работает бот:
• При добавлении бота в чат запускаются таймеры для всех участников
• При получении #training_done таймер перезапускается на 7 дней
• Через 6 дней без #training_done - предупреждение
• Через 7 дней без #training_done - удаление из чата
• 🏆 7 дней подряд = 42 КУБКА! 🏆
• 🏆🏆 14 дней подряд = 42 КУБКА! 🏆🏆
• 🏆🏆🏆 21 день подряд = 42 КУБКА! 🏆🏆🏆
• 🏆🏆🏆 30 дней подряд = 420 КУБКОВ! 🏆🏆🏆
• 🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆 90 дней подряд = 4200 КУБКОВ! 🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆

📋 Правила:
• Отчётом считается любое сообщение с тегом #training_done
• Если заболели — отправь #sick_leave
• После выздоровления — отправь #healthy
• Через 6 дней без отчёта — предупреждение
• Через 7 дней без отчёта — удаление из чата

Оставайся активным и не становись жирным леопардом! 🦁`

	reply := tgbotapi.NewMessage(msg.Chat.ID, helpText)

	b.logger.Infof("Sending help message to chat %d", msg.Chat.ID)
	_, err := b.api.Send(reply)
	if err != nil {
		b.logger.Errorf("Failed to send help message: %v", err)
	} else {
		b.logger.Infof("Successfully sent help message to chat %d", msg.Chat.ID)
	}
}

func (b *Bot) handleStart(msg *tgbotapi.Message) {
	welcomeText := `🦁 **Добро пожаловать в LeoPoacherBot!** 🦁

💪 **Этот бот поможет вам оставаться в форме и не стать жирным леопардом!**

📋 **Основные команды:**
• /start — Показать это приветствие
• /help — Показать полную справку
• /start_timer — Запустить таймеры (только для администраторов)

💪 **Отчеты о тренировке:**
• #training_done — Отправить отчет о тренировке

🏥 **Больничный:**
• #sick_leave — Взять больничный (приостанавливает таймер)
• #healthy — Выздороветь (возобновляет таймер)

⏰ **Как это работает:**
• При добавлении бота в чат запускаются таймеры для всех участников
• Каждый отчет с #training_done перезапускает таймер на 7 дней
• Через 6 дней без отчета — предупреждение
• Через 7 дней без отчета — удаление из чата
• 🏆 7 дней подряд = 42 КУБКА! 🏆
• 🏆🏆 14 дней подряд = 42 КУБКА! 🏆🏆
• 🏆🏆🏆 21 день подряд = 42 КУБКА! 🏆🏆🏆
• 🏆🏆🏆 30 дней подряд = 420 КУБКОВ! 🏆🏆🏆
• 🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆 90 дней подряд = 4200 КУБКОВ! 🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆

🎯 **Начни прямо сейчас — отправь #training_done!**`

	reply := tgbotapi.NewMessage(msg.Chat.ID, welcomeText)

	b.logger.Infof("Sending start message to chat %d", msg.Chat.ID)
	_, err := b.api.Send(reply)
	if err != nil {
		b.logger.Errorf("Failed to send start message: %v", err)
	} else {
		b.logger.Infof("Successfully sent start message to chat %d", msg.Chat.ID)
	}
}

func (b *Bot) handleDB(msg *tgbotapi.Message) {
	// Проверяем права администратора
	if !b.isAdmin(msg.Chat.ID, msg.From.ID) {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Только администраторы или владелец могут использовать эту команду!")
		b.api.Send(reply)
		return
	}

	// Получаем статистику
	stats, err := b.db.GetDatabaseStats()
	if err != nil {
		b.logger.Errorf("Failed to get database stats: %v", err)
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Ошибка при получении данных")
		b.api.Send(reply)
		return
	}

	// Формируем отчет
	report := fmt.Sprintf("📊 Статистика БД:\n\n👥 Всего пользователей: %v\n✅ С training_done: %v\n🏥 На больничном: %v\n💪 Выздоровели: %v",
		stats["total_users"], stats["training_done"], stats["sick_leave"], stats["healthy"])

	reply := tgbotapi.NewMessage(msg.Chat.ID, report)

	b.logger.Infof("Sending DB stats message to chat %d", msg.Chat.ID)
	_, err = b.api.Send(reply)
	if err != nil {
		b.logger.Errorf("Failed to send DB stats message: %v", err)
	} else {
		b.logger.Infof("Successfully sent DB stats message to chat %d", msg.Chat.ID)
	}
}

func (b *Bot) handleTop(msg *tgbotapi.Message) {
	// Получаем топ пользователей
	topUsers, err := b.db.GetTopUsers(msg.Chat.ID, 10)
	if err != nil {
		b.logger.Errorf("Failed to get top users: %v", err)
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Ошибка при получении данных")
		b.api.Send(reply)
		return
	}

	if len(topUsers) == 0 {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "🏆 **Топ пользователей:**\n\n📊 Пока нет данных о тренировках")
		reply.ParseMode = "Markdown"
		b.api.Send(reply)
		return
	}

	// Формируем топ
	topText := "🏆 Топ пользователей по очкам:\n\n"
	for i, user := range topUsers {
		emoji := "🥇"
		if i == 1 {
			emoji = "🥈"
		} else if i == 2 {
			emoji = "🥉"
		} else {
			emoji = fmt.Sprintf("%d️⃣", i+1)
		}
		topText += fmt.Sprintf("%s %s - %d калорий\n", emoji, user.Username, user.Calories)
	}

	reply := tgbotapi.NewMessage(msg.Chat.ID, topText)

	b.logger.Infof("Sending top users message to chat %d", msg.Chat.ID)
	_, err = b.api.Send(reply)
	if err != nil {
		b.logger.Errorf("Failed to send top users message: %v", err)
	} else {
		b.logger.Infof("Successfully sent top users message to chat %d", msg.Chat.ID)
	}
}

func (b *Bot) handlePoints(msg *tgbotapi.Message) {
	// Получаем калории пользователя
	calories, err := b.db.GetUserCalories(msg.From.ID, msg.Chat.ID)
	if err != nil {
		b.logger.Errorf("Failed to get user calories: %v", err)
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Ошибка при получении данных")
		b.api.Send(reply)
		return
	}

	// Получаем никнейм пользователя
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

	// Формируем сообщение
	caloriesText := fmt.Sprintf("🔥 Ваши калории:\n\n👤 %s\n🎯 Всего сожжено калорий: %d\n\n💡 Отправляйте #training_done для сжигания калорий!", username, calories)

	reply := tgbotapi.NewMessage(msg.Chat.ID, caloriesText)

	b.logger.Infof("Sending calories message to chat %d", msg.Chat.ID)
	_, err = b.api.Send(reply)
	if err != nil {
		b.logger.Errorf("Failed to send calories message: %v", err)
	} else {
		b.logger.Infof("Successfully sent calories message to chat %d", msg.Chat.ID)
	}
}

func (b *Bot) handleCups(msg *tgbotapi.Message) {
	// Получаем кубки пользователя
	cups, err := b.db.GetUserCups(msg.From.ID, msg.Chat.ID)
	if err != nil {
		b.logger.Errorf("Failed to get user cups: %v", err)
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Ошибка при получении данных")
		b.api.Send(reply)
		return
	}

	// Получаем никнейм пользователя
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

	// Формируем сообщение в зависимости от количества кубков
	var cupsText string
	if cups > 420 {
		cupsText = fmt.Sprintf("🌟⚡ СУПЕР-УРОВЕНЬ! ⚡🌟\n\n👤 %s\n🎯 Всего заработано кубков: %d\n\n🎊 ВСЕ ОЖИДАНИЯ ПРЕВЗОЙДЕНЫ! 🎊\n\n🦁 Fat Leopard в полном восторге!\n💪 Ты не просто чемпион - ты СУПЕР-ЧЕМПИОН!\n🔥 Твоя сила и мощь безграничны!\n⭐️ Ты вдохновляешь всю стаю!\n👑 Мотивация не верит, что такое бывает!\n🌟 Ты сияешь ярче всех!\n\n🎯 Продолжай в том же духе, супер-леопард!", username, cups)
	} else if cups >= 420 {
		cupsText = fmt.Sprintf("🎊 ПОЗДРАВЛЯЕМ! 🎊\n\n👤 %s\n🎯 Всего заработано кубков: %d\n\n🏆 ТЫ ДОСТИГ ЦЕЛИ РОЗЫГРЫША!\n🎁 Участвуешь в розыгрыше футболки Fat Leopard!\n💪 Ты настоящий чемпион!\n🔥 Продолжай тренироваться!", username, cups)
	} else {
		cupsText = fmt.Sprintf("🏆 Ваши кубки:\n\n👤 %s\n🎯 Всего заработано кубков: %d\n\n💡 Отправляйте #training_done для получения кубков!\n\n🎊 Розыгрыш футболки Fat Leopard при достижении 420 кубков!", username, cups)
	}

	reply := tgbotapi.NewMessage(msg.Chat.ID, cupsText)

	b.logger.Infof("Sending cups message to chat %d", msg.Chat.ID)
	_, err = b.api.Send(reply)
	if err != nil {
		b.logger.Errorf("Failed to send cups message: %v", err)
	} else {
		b.logger.Infof("Successfully sent cups message to chat %d", msg.Chat.ID)
	}
}

func (b *Bot) handleSendToChat(msg *tgbotapi.Message) {
	// Проверяем права доступа - только владелец бота может отправлять сообщения в другие чаты
	if msg.From.ID != b.config.OwnerID {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ У вас нет прав для использования этой команды")
		b.api.Send(reply)
		return
	}

	// Получаем аргументы команды
	args := msg.CommandArguments()
	if args == "" {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Использование: /send_to_chat <chat_id> <текст_сообщения>")
		b.api.Send(reply)
		return
	}

	// Разбираем аргументы
	parts := strings.SplitN(args, " ", 2)
	if len(parts) != 2 {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Использование: /send_to_chat <chat_id> <текст_сообщения>")
		b.api.Send(reply)
		return
	}

	// Парсим chat_id
	chatID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Неверный формат chat_id")
		b.api.Send(reply)
		return
	}

	// Получаем текст сообщения
	messageText := parts[1]

	// Создаем сообщение для отправки
	chatMessage := tgbotapi.NewMessage(chatID, messageText)

	// Отправляем сообщение в указанный чат
	b.logger.Infof("Sending message to chat %d: %s", chatID, messageText)
	_, err = b.api.Send(chatMessage)
	if err != nil {
		errorMsg := fmt.Sprintf("❌ Ошибка при отправке сообщения в чат %d: %v", chatID, err)
		reply := tgbotapi.NewMessage(msg.Chat.ID, errorMsg)
		b.api.Send(reply)
		b.logger.Errorf("Failed to send message to chat %d: %v", chatID, err)
	} else {
		successMsg := fmt.Sprintf("✅ Сообщение успешно отправлено в чат %d", chatID)
		reply := tgbotapi.NewMessage(msg.Chat.ID, successMsg)
		b.api.Send(reply)
		b.logger.Infof("Successfully sent message to chat %d", chatID)
	}
}

func (b *Bot) startTimer(userID, chatID int64, username string) {
	// Предупреждение через 6 дней, удаление через 7 дней
	b.startTimerWithDuration(userID, chatID, username, 7*24*time.Hour)
}

func (b *Bot) startTimerWithDuration(userID, chatID int64, username string, duration time.Duration) {
	// Отменяем существующие таймеры
	b.cancelTimer(userID)

	// Создаем новые таймеры
	warningTask := make(chan bool)
	removalTask := make(chan bool)

	timerStartTime := utils.FormatMoscowTime(utils.GetMoscowTime())
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
		// Обновляем время начала таймера
		messageLog.TimerStartTime = &timerStartTime
		if err := b.db.SaveMessageLog(messageLog); err != nil {
			b.logger.Errorf("Failed to save timer start time: %v", err)
		} else {
			b.logger.Infof("Saved timer start time: %s", timerStartTime)
		}
	}

	// Рассчитываем время предупреждения (6 дней до удаления)
	warningTime := duration - 24*time.Hour // Предупреждение за 1 день до удаления
	if warningTime < 0 {
		warningTime = duration / 2 // Fallback если время слишком короткое
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

func (b *Bot) cancelTimer(userID int64) {
	if timer, exists := b.timers[userID]; exists {
		close(timer.WarningTask)
		close(timer.RemovalTask)
		delete(b.timers, userID)
		b.logger.Infof("Cancelled timer for user %d", userID)
	}
}

func (b *Bot) sendWarning(userID, chatID int64, username string) {
	message := fmt.Sprintf("⚠️ Предупреждение!\n\n@%s, ты не отправляешь отчет о тренировке уже 6 дней!\n\n🦁 Я питаюсь ленивыми леопардами и становлюсь жирнее!\n\n💪 Ты ведь не хочешь стать как я?\n\n⏰ У тебя остался 1 день до удаления из чата!\n\n🎯 Отправь #training_done прямо сейчас!", username)

	msg := tgbotapi.NewMessage(chatID, message)
	b.logger.Infof("Sending warning to user %d (%s)", userID, username)
	_, err := b.api.Send(msg)
	if err != nil {
		b.logger.Errorf("Failed to send warning: %v", err)
	} else {
		b.logger.Infof("Successfully sent warning to user %d (%s)", userID, username)
	}
}

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

		b.logger.Infof("Removed user %d (%s) from chat", userID, username)
	}

	// Помечаем пользователя как удаленного в базе данных
	if err := b.db.MarkUserAsDeleted(userID, chatID); err != nil {
		b.logger.Errorf("Failed to mark user as deleted: %v", err)
	}

	// Удаляем таймер
	delete(b.timers, userID)
	b.logger.Infof("Timer removed for user %d", userID)
}

func (b *Bot) isAdmin(chatID, userID int64) bool {
	// Проверяем, является ли пользователь владельцем
	if userID == b.config.OwnerID {
		return true
	}

	// Проверяем права администратора
	member, err := b.api.GetChatMember(tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			ChatID: chatID,
			UserID: userID,
		},
	})
	if err != nil {
		b.logger.Errorf("Failed to get chat member: %v", err)
		return false
	}

	return member.Status == "administrator" || member.Status == "creator"
}

func (b *Bot) isUserInChat(chatID, userID int64) bool {
	_, err := b.api.GetChatMember(tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			ChatID: chatID,
			UserID: userID,
		},
	})
	return err == nil
}

func (b *Bot) calculateCalories(messageLog *models.MessageLog) (int, int, bool, bool, bool, bool, bool) {
	today := utils.GetMoscowDate()

	// ДЕБАГ: Логируем входные данные
	b.logger.Infof("DEBUG calculateCalories: today=%s, LastTrainingDate=%v, StreakDays=%d",
		today, messageLog.LastTrainingDate, messageLog.StreakDays)

	// Базовые калории за тренировку
	caloriesToAdd := 1

	// Проверяем, была ли уже тренировка сегодня
	if messageLog.LastTrainingDate != nil && *messageLog.LastTrainingDate == today {
		b.logger.Infof("DEBUG: Уже тренировались сегодня, возвращаем 0 калорий")
		return 0, messageLog.StreakDays, false, false, false, false, false // Уже тренировались сегодня
	}

	// Рассчитываем новую серию
	newStreakDays := 1

	if messageLog.LastTrainingDate != nil {
		yesterday := utils.GetMoscowTime().AddDate(0, 0, -1)
		yesterdayStr := utils.GetMoscowDateFromTime(yesterday)
		b.logger.Infof("DEBUG: Сравниваем LastTrainingDate=%s с yesterday=%s", *messageLog.LastTrainingDate, yesterdayStr)

		if *messageLog.LastTrainingDate == yesterdayStr {
			// Продолжаем серию
			newStreakDays = messageLog.StreakDays + 1
			b.logger.Infof("DEBUG: Продолжаем серию: %d + 1 = %d", messageLog.StreakDays, newStreakDays)
		} else {
			// Серия прервана, начинаем заново
			newStreakDays = 1
			b.logger.Infof("DEBUG: Серия прервана, начинаем заново: %d", newStreakDays)
		}
	} else {
		// Если нет данных о последней тренировке, но есть streak, продолжаем его
		if messageLog.StreakDays > 0 {
			newStreakDays = messageLog.StreakDays + 1
			b.logger.Infof("DEBUG: Нет данных о последней тренировке, продолжаем streak: %d + 1 = %d", messageLog.StreakDays, newStreakDays)
		}
	}

	// Бонусы за серию
	if newStreakDays >= 30 {
		caloriesToAdd += 20 // 30 дней подряд
	} else if newStreakDays >= 14 {
		caloriesToAdd += 10 // 14 дней подряд
	} else if newStreakDays >= 7 {
		caloriesToAdd += 5 // 7 дней подряд
	} else if newStreakDays >= 3 {
		caloriesToAdd += 2 // 3 дня подряд
	}

	// Бонус за возвращение после больничного
	if messageLog.HasSickLeave && messageLog.HasHealthy {
		caloriesToAdd += 2
	}

	// Проверяем, достиг ли пользователь недельной серии (7 дней подряд)
	weeklyAchievement := newStreakDays == 7

	// Проверяем, достиг ли пользователь двухнедельной серии (14 дней подряд)
	twoWeekAchievement := newStreakDays == 14

	// Проверяем, достиг ли пользователь трехнедельной серии (21 день подряд)
	threeWeekAchievement := newStreakDays == 21

	// Проверяем, достиг ли пользователь месячной серии (30 дней подряд)
	monthlyAchievement := newStreakDays == 30

	// Проверяем, достиг ли пользователь квартальной серии (90 дней подряд)
	quarterlyAchievement := newStreakDays == 90

	// ДЕБАГ: Логируем результат
	b.logger.Infof("DEBUG calculateCalories RESULT: caloriesToAdd=%d, newStreakDays=%d, weekly=%t, twoWeek=%t, threeWeek=%t, monthly=%t, quarterly=%t",
		caloriesToAdd, newStreakDays, weeklyAchievement, twoWeekAchievement, threeWeekAchievement, monthlyAchievement, quarterlyAchievement)

	return caloriesToAdd, newStreakDays, weeklyAchievement, twoWeekAchievement, threeWeekAchievement, monthlyAchievement, quarterlyAchievement
}

func (b *Bot) calculateRemainingTime(messageLog *models.MessageLog) time.Duration {
	// Если нет данных о времени, возвращаем полный таймер
	if messageLog.TimerStartTime == nil {
		return 7 * 24 * time.Hour
	}

	// Парсим время начала таймера
	timerStart, err := utils.ParseMoscowTime(*messageLog.TimerStartTime)
	if err != nil {
		b.logger.Errorf("Failed to parse timer start time: %v", err)
		return 7 * 24 * time.Hour
	}

	// Полное время таймера (7 дней)
	fullTimerDuration := 7 * 24 * time.Hour

	// Если был больничный, учитываем его
	if messageLog.SickLeaveStartTime != nil && messageLog.HasSickLeave && !messageLog.HasHealthy {
		// Пользователь на больничном - таймер приостановлен
		// Возвращаем оставшееся время на момент больничного
		sickLeaveStart, err := utils.ParseMoscowTime(*messageLog.SickLeaveStartTime)
		if err != nil {
			b.logger.Errorf("Failed to parse sick leave start time: %v", err)
			return fullTimerDuration
		}

		// Рассчитываем время, которое прошло до больничного
		timeBeforeSickLeave := sickLeaveStart.Sub(timerStart)

		// Оставшееся время на момент больничного
		remainingTime := fullTimerDuration - timeBeforeSickLeave

		if remainingTime <= 0 {
			return 0 // Время истекло
		}

		return remainingTime
	}

	// Если был больничный и пользователь выздоровел
	if messageLog.SickLeaveStartTime != nil && messageLog.HasSickLeave && messageLog.HasHealthy {
		sickLeaveStart, err := utils.ParseMoscowTime(*messageLog.SickLeaveStartTime)
		if err != nil {
			b.logger.Errorf("Failed to parse sick leave start time: %v", err)
			return fullTimerDuration
		}

		// Рассчитываем время, которое прошло до больничного
		timeBeforeSickLeave := sickLeaveStart.Sub(timerStart)

		// Оставшееся время после больничного
		remainingTime := fullTimerDuration - timeBeforeSickLeave

		// Если время истекло до больничного, возвращаем 0
		if remainingTime <= 0 {
			return 0 // Время истекло
		}

		return remainingTime
	}

	// Обычный случай - рассчитываем оставшееся время
	// Используем московское время для расчета
	moscowNow := utils.GetMoscowTime()
	elapsedTime := moscowNow.Sub(timerStart)
	remainingTime := fullTimerDuration - elapsedTime

	if remainingTime <= 0 {
		return 0 // Время истекло
	}

	return remainingTime
}

func (b *Bot) sendWeeklyCupsReward(msg *tgbotapi.Message, username string, streakDays int) {
	// Создаем сообщение с 42 кубками
	cupsMessage := fmt.Sprintf(`🏆 НЕВЕРОЯТНО! 🏆

%s, ты тренируешься уже %d дней подряд! 


🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🎯 42 КУБКА за твою недельную серию! 🎯
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆

🦁 Fat Leopard гордится тобой! 
💪 Ты настоящий чемпион!
🔥 Продолжай в том же духе!

#weekly_champion #42_cups #training_streak`, username, streakDays)

	// Отправляем сообщение с кубками
	reply := tgbotapi.NewMessage(msg.Chat.ID, cupsMessage)

	b.logger.Infof("Sending weekly cups reward to chat %d for user %s (streak: %d days)", msg.Chat.ID, username, streakDays)
	_, err := b.api.Send(reply)
	if err != nil {
		b.logger.Errorf("Failed to send weekly cups reward: %v", err)
	} else {
		b.logger.Infof("Successfully sent weekly cups reward to chat %d for user %s", msg.Chat.ID, username)
	}
}

func (b *Bot) sendMonthlyCupsReward(msg *tgbotapi.Message, username string, streakDays int) {
	// Создаем сообщение с 420 кубками
	cupsMessage := fmt.Sprintf(`🏆🏆🏆 ЛЕГЕНДА! 🏆🏆🏆

%s, ты тренируешься уже %d дней подряд! 

🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆

🎯 420 КУБКОВ ЗА ТВОЮ МЕСЯЧНУЮ СЕРИЮ! 🎯

🦁 Fat Leopard в шоке от твоей мотивации! 
💪 Ты абсолютная легенда!
🔥 Ты вдохновляешь всех вокруг!
⭐ Ты настоящий чемпион чемпионов!

#monthly_legend #420_cups #training_legend`, username, streakDays)

	// Отправляем сообщение с кубками
	reply := tgbotapi.NewMessage(msg.Chat.ID, cupsMessage)

	b.logger.Infof("Sending monthly cups reward to chat %d for user %s (streak: %d days)", msg.Chat.ID, username, streakDays)
	_, err := b.api.Send(reply)
	if err != nil {
		b.logger.Errorf("Failed to send monthly cups reward: %v", err)
	} else {
		b.logger.Infof("Successfully sent monthly cups reward to chat %d for user %s", msg.Chat.ID, username)
	}
}

func (b *Bot) sendQuarterlyCupsReward(msg *tgbotapi.Message, username string, streakDays int) {
	// Создаем сообщение с 4200 кубками
	cupsMessage := fmt.Sprintf(`🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆 БОЖЕСТВЕННО! 🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆

%s, ты тренируешься уже %d дней подряд! 

🎯 4200 КУБКОВ ЗА ТВОЮ КВАРТАЛЬНУЮ СЕРИЮ! 🎯

🦁 Fat Leopard падает в обморок от твоей силы воли! 
💪 Ты божественное создание!
🔥 Ты переписываешь законы мотивации!
⭐ Ты абсолютный император тренировок!
👑 Ты король всех королей!
🌟 Ты сияешь ярче всех звезд!

#quarterly_god #4200_cups #training_emperor`, username, streakDays)

	// Отправляем сообщение с кубками
	reply := tgbotapi.NewMessage(msg.Chat.ID, cupsMessage)

	b.logger.Infof("Sending quarterly cups reward to chat %d for user %s (streak: %d days)", msg.Chat.ID, username, streakDays)
	_, err := b.api.Send(reply)
	if err != nil {
		b.logger.Errorf("Failed to send quarterly cups reward: %v", err)
	} else {
		b.logger.Infof("Successfully sent quarterly cups reward to chat %d for user %s", msg.Chat.ID, username)
	}
}

func (b *Bot) sendTwoWeekCupsReward(msg *tgbotapi.Message, username string, streakDays int) {
	// Создаем сообщение с 42 кубками
	cupsMessage := fmt.Sprintf(`🏆🏆 НЕВЕРОЯТНО! 🏆🏆

%s, ты тренируешься уже %d дней подряд! 

🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🎯 42 КУБКА за твою двухнедельную серию! 🎯
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆

🦁 Fat Leopard в восторге от твоей мотивации! 
💪 Ты настоящий воин!
🔥 Твоя сила растет с каждым днем!
⭐ Ты вдохновляешь всю стаю!

#two_week_champion #42_cups #training_warrior`, username, streakDays)

	// Отправляем сообщение с кубками
	reply := tgbotapi.NewMessage(msg.Chat.ID, cupsMessage)

	b.logger.Infof("Sending two-week cups reward to chat %d for user %s (streak: %d days)", msg.Chat.ID, username, streakDays)
	_, err := b.api.Send(reply)
	if err != nil {
		b.logger.Errorf("Failed to send two-week cups reward: %v", err)
	} else {
		b.logger.Infof("Successfully sent two-week cups reward to chat %d for user %s", msg.Chat.ID, username)
	}
}

func (b *Bot) sendThreeWeekCupsReward(msg *tgbotapi.Message, username string, streakDays int) {
	// Создаем сообщение с 42 кубками
	cupsMessage := fmt.Sprintf(`🏆🏆🏆 ФЕНОМЕНАЛЬНО! 🏆🏆🏆

%s, ты тренируешься уже %d дней подряд! 

🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🎯 42 КУБКА за твою трехнедельную серию! 🎯
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆

🦁 Fat Leopard поражен твоей силой воли! 
💪 Ты абсолютный чемпион!
🔥 Твоя мотивация не знает границ!
⭐ Ты легенда среди леопардов!
👑 Ты король мотивации!

#three_week_legend #42_cups #training_king`, username, streakDays)

	// Отправляем сообщение с кубками
	reply := tgbotapi.NewMessage(msg.Chat.ID, cupsMessage)

	b.logger.Infof("Sending three-week cups reward to chat %d for user %s (streak: %d days)", msg.Chat.ID, username, streakDays)
	_, err := b.api.Send(reply)
	if err != nil {
		b.logger.Errorf("Failed to send three-week cups reward: %v", err)
	} else {
		b.logger.Infof("Successfully sent three-week cups reward to chat %d for user %s", msg.Chat.ID, username)
	}
}

func (b *Bot) sendSuperLevelMessage(msg *tgbotapi.Message, username string, totalCups int) {
	// Создаем сообщение о супер-уровне
	superMessage := fmt.Sprintf(`🌟⚡ СУПЕР-УРОВЕНЬ ДОСТИГНУТ! ⚡🌟

%s, ты накопил %d кубков! 

🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆

🎊 ВСЕ ОЖИДАНИЯ ПРЕВЗОЙДЕНЫ! 🎊

🦁 Fat Leopard в полном восторге! 
💪 Ты не просто чемпион - ты СУПЕР-ЧЕМПИОН!
🔥 Твоя сила и мощь безграничны!
⭐️ Ты вдохновляешь всю стаю!
👑 Мотивация не верит, что такое бывает!
🌟 Ты сияешь ярче всех!

🎯 Продолжай в том же духе, супер-леопард!

#super_level #%d_cups #motivation_king`, username, totalCups, totalCups)

	// Отправляем сообщение о супер-уровне
	reply := tgbotapi.NewMessage(msg.Chat.ID, superMessage)

	b.logger.Infof("Sending super level message to chat %d for user %s (total cups: %d)", msg.Chat.ID, username, totalCups)
	_, err := b.api.Send(reply)
	if err != nil {
		b.logger.Errorf("Failed to send super level message: %v", err)
	} else {
		b.logger.Infof("Successfully sent super level message to chat %d for user %s", msg.Chat.ID, username)
	}
}

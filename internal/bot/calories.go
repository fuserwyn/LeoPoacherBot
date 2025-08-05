package bot

import (
	"time"

	"leo-bot/internal/models"
)

// calculateCalories рассчитывает калории и streak для пользователя
func (b *Bot) calculateCalories(messageLog *models.MessageLog) (int, int) {
	// Проверяем, была ли тренировка сегодня
	today := time.Now().Format("2006-01-02")
	if messageLog.LastTrainingDate != nil && *messageLog.LastTrainingDate == today {
		// Тренировка уже была сегодня, возвращаем текущие значения
		return messageLog.Calories, messageLog.StreakDays
	}

	// Базовая калория за тренировку
	newCalories := 1
	totalCalories := messageLog.Calories + newCalories

	// Проверяем streak
	streakDays := 1 // Минимум 1 день
	if messageLog.LastTrainingDate != nil {
		lastTraining, err := time.Parse("2006-01-02", *messageLog.LastTrainingDate)
		if err == nil {
			yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
			if lastTraining.Format("2006-01-02") == yesterday {
				// Вчера была тренировка, увеличиваем streak
				streakDays = messageLog.StreakDays + 1
			} else {
				// Streak прерван
				streakDays = 1
			}
		}
	}

	// Добавляем бонусы за streak
	switch streakDays {
	case 3:
		totalCalories += 2
	case 7:
		totalCalories += 5
	case 14:
		totalCalories += 10
	case 30:
		totalCalories += 20
	}

	// Бонус за возвращение после больничного
	if messageLog.HasSickLeave && messageLog.HasHealthy {
		totalCalories += 2
	}

	return totalCalories, streakDays
}

// calculateRemainingTime рассчитывает оставшееся время до удаления
func (b *Bot) calculateRemainingTime(messageLog *models.MessageLog) time.Duration {
	// Для тестирования используем 2 минуты
	return 2 * time.Minute
}
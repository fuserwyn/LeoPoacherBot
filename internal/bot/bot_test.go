package bot

import (
	"testing"
	"time"

	"leo-bot/internal/config"
	"leo-bot/internal/logger"
	"leo-bot/internal/models"
)

func TestCalculateRemainingTime(t *testing.T) {
	// Создаем мок логгер
	log := logger.New("info")

	// Создаем тестовый бот
	cfg := &config.Config{OwnerID: 123}
	bot := &Bot{
		logger: log,
		config: cfg,
	}

	// Тест 1: Нет данных о времени
	messageLog := &models.MessageLog{}
	remainingTime := bot.calculateRemainingTime(messageLog)
	expectedTime := 7 * 24 * time.Hour

	if remainingTime != expectedTime {
		t.Errorf("Expected %v, got %v", expectedTime, remainingTime)
	}

	// Тест 2: Есть данные о времени
	timerStart := time.Now().Add(-2 * 24 * time.Hour).Format(time.RFC3339)
	sickLeaveStart := time.Now().Add(-1 * 24 * time.Hour).Format(time.RFC3339)

	messageLogWithTime := &models.MessageLog{
		TimerStartTime:     &timerStart,
		SickLeaveStartTime: &sickLeaveStart,
	}

	remainingTime = bot.calculateRemainingTime(messageLogWithTime)
	expectedTime = 5 * 24 * time.Hour // 7 - 2 = 5 дней

	if remainingTime != expectedTime {
		t.Errorf("Expected %v, got %v", expectedTime, remainingTime)
	}
}

func TestIsAdmin(t *testing.T) {
	// Создаем тестовый бот
	cfg := &config.Config{OwnerID: 123}
	bot := &Bot{
		config: cfg,
	}

	// Тест: Пользователь является владельцем
	isAdmin := bot.isAdmin(456, 123)
	if !isAdmin {
		t.Error("Owner should be admin")
	}

	// Тест: Пользователь не является владельцем
	isAdmin = bot.isAdmin(456, 789)
	if isAdmin {
		t.Error("Non-owner should not be admin")
	}
}

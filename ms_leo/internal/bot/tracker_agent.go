package bot

import (
	"fmt"
	"strings"
	"time"

	"leo-bot/internal/database"
)

const trackerAgentHttpTimeout = 45 * time.Second
const trackerShipHttpTimeout = 90 * time.Second

func trackerAgentDonate(b *Bot, job database.TrackerTask, amount int) (string, error) {
	if b == nil || b.aiClient == nil {
		return "", fmt.Errorf("Лео недоступен")
	}
	msg := fmt.Sprintf("Донат %d звёзд", amount)
	if amount%100 == 0 && amount >= 100 {
		msg = fmt.Sprintf("Донат %d руб", amount)
	}
	return b.aiClient.Chat([]ai.ChatMessage{
		{Role: "system", Content: `Ты — Лео, помощник стаи Fat Leopard. Обработай донат звёздами или рублями.
Ответь JSON без обрамления: {"note":"..."}
note — подтверждение получения доната, без эмодзи.`},
		{Role: "user", Content: msg},
	}, "cursor-auto")
}

func (b *Bot) trackerAgentHandleDonate(amount int, sender string) (string, error) {
	t := database.TrackerTask{
		Prompt:     fmt.Sprintf("Донат %d", amount),
		WhenAt:     time.Now(),
		WhenLabel:  "сразу",
		Status:     "running",
		DevColumn:  trackerColDoing,
		AutoReview: true,
		FastTrack:  true,
	}
	if amount%100 == 0 {
		t.Kind = "donate_rub"
	} else {
		t.Kind = "donate_stars"
	}
	created, err := b.db.CreateTrackerTask(t)
	if err != nil {
		return "", fmt.Errorf("не создать задачу на донат: %v", err)
	}
	result, err := trackerAgentDonate(b, created, amount)
	if err != nil {
		return "", fmt.Errorf("обработка доната не удалась: %v", err)
	}
	created.Result = result
	if err := b.db.SaveTrackerTask(created); err != nil {
		return "", fmt.Errorf("не сохранить результат доната: %v", err)
	}
	_ = applyTrackerColumn(&created, trackerColDone)
	return result, nil
}
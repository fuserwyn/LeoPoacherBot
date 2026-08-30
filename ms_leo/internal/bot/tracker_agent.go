package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"leo-bot/internal/ai"
	"leo-bot/internal/database"
)

// Ревью и тест гоняет Cursor Composer: он читает код в репозитории,
// а не только формулировку карточки. Реализацию пишет модель доски
// (BOARD_MODEL) или та, что настроена у владельца.
const trackerComposerModelKey = "cursor-composer"

const trackerAgentHTTPTimeout = 45 * time.Second
const trackerShipHTTPTimeout = 90 * time.Second

var trackerBranchRe = regexp.MustCompile(`tracker/\d+-\d+`)
var trackerCommitRe = regexp.MustCompile(`(?i)коммит(?:\s+выполнения)?\s+([0-9a-f]{7,40})`)

func trackerComposerModel(b *Bot) string {
	if b != nil && b.config != nil {
		if m := strings.TrimSpace(b.config.BoardReviewModel); m != "" {
			return m
		}
	}
	return trackerComposerModelKey
}

func trackerImplModel(b *Bot) string {
	if b != nil && b.config != nil {
		if m := strings.TrimSpace(b.config.BoardModel); m != "" {
			return m
		}
	}
	return "cursor-auto"
}

func trackerAgentPrompt(job database.TrackerTask, branch string) string {
	var b strings.Builder
	b.WriteString("Ты агент трекера Fat Leopard. Репозиторий уже на ветке задачи.\n")
	b.WriteString("Сделай задачу инструментами Cursor (Read/StrReplace/Write): правь файлы точечно.\n")
	b.WriteString("Не возвращай JSON с полным текстом файлов и не пиши правки только в .tracker.\n")
	b.WriteString("Коммить можно локально. Не открывай PR. Не создавай новую ветку cursor/*.\n")
	b.WriteString("Пуш на origin сделает трекер сам, в ветку ")
	b.WriteString(branch)
	b.WriteString(".\n")
	b.WriteString("В конце кратко напиши, что сделал. Без эмодзи.\n\n")
	b.WriteString(strings.TrimSpace(job.Prompt))
	return b.String()
}

func trackerAgentDoing(b *Bot, job database.TrackerTask, branch string) (string, error) {
	if b == nil || b.aiClient == nil {
		return "", fmt.Errorf("Лео недоступен")
	}
	return b.aiClient.Chat([]ai.ChatMessage{
		{Role: "system", Content: trackerAgentPrompt(job, branch)},
	}, trackerImplModel(b))
}

func trackerAgentQA(b *Bot, job database.TrackerTask) (string, error) {
	if b == nil || b.aiClient == nil {
		return "", fmt.Errorf("Лео недоступен")
	}
	return b.aiClient.Chat([]ai.ChatMessage{
		{Role: "system", Content: `Ты — Лео, тестировщик приложения стаи Fat Leopard.
Прочитай формулировку задачи и коротко скажи, что проверить руками.
Ответь JSON без обрамления: {"note":"..."}
note — 2–5 предложений, без эмодзи, конкретно: что открыть и что должно получиться.`},
		{Role: "user", Content: job.Prompt},
	}, trackerComposerModel(b))
}

func trackerAgentReview(b *Bot, job database.TrackerTask) (string, error) {
	if b == nil || b.aiClient == nil {
		return "", fmt.Errorf("Лео недоступен")
	}
	return b.aiClient.Chat([]ai.ChatMessage{
		{Role: "system", Content: `Ты — Лео, ревьюер приложения стаи Fat Leopard.
Прочитай формулировку задачи и правки агента. Проверь их и сделай свои замечания.
Ответь JSON без обрамления: {"note":"..."}
note — 2–5 предложений, без Эмодзи, конкретно: что неправильно и как исправить.`},
		{Role: "user", Content: job.Prompt},
	}, trackerComposerModel(b))
}

// trackerAgentDonate обрабатывает донат звёздами и рублями
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
	}, trackerImplModel(b))
}

// trackerAgentHandleDonate создает и обрабатывает задачу на донат
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
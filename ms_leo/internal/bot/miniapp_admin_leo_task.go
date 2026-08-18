package bot

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"leo-bot/internal/ai"

	initdata "github.com/telegram-mini-apps/init-data-golang"
)

// «Спросить Леопарда» во вкладке «Задача»: тот же Лео, что и в чате, только
// вместо подколок про тренировки формулирует задачу агенту MyVibeLab.
// Отвечает двумя частями — репликой в своём стиле и сухой формулировкой,
// которую можно поставить в трекер как есть.

const leoTaskSystemPrompt = `Ты — Лео, суровый и остроумный леопард, тренер стаи Fat Leopard.
Тебя спрашивают, что улучшить в приложении стаи (мини-апп: лента тренировок,
комментарии, стрики, ачивки, чат, админка).

Ответь JSON без обрамления и пояснений:
{"reply": "...", "task": "..."}

reply — 1–3 предложения твоим голосом: с юмором, дерзко, но по делу.
task — сухая формулировка задачи для разработчика: что сделать и зачем,
одним абзацем до 400 символов, без эмодзи и без обращения к человеку.
Если вопрос не про приложение — в task верни пустую строку.`

var leoJSONBlock = regexp.MustCompile(`(?s)\{.*\}`)

// MiniappAskLeoTask — реплика Лео и готовая формулировка задачи.
func (b *Bot) MiniappAskLeoTask(
	viewerUserID int64, initD initdata.InitData, question string,
) (reply string, task string, err error) {
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return "", "", err
	}
	q := strings.TrimSpace(question)
	if q == "" {
		return "", "", fmt.Errorf("спроси что-нибудь")
	}
	if len([]rune(q)) > 500 {
		q = string([]rune(q)[:500])
	}
	if b.aiClient == nil {
		return "", "", fmt.Errorf("Лео сейчас недоступен: не настроен OpenRouter")
	}
	raw, err := b.aiClient.Chat([]ai.ChatMessage{
		{Role: "system", Content: leoTaskSystemPrompt},
		{Role: "user", Content: q},
	}, "")
	if err != nil {
		return "", "", fmt.Errorf("Лео не ответил: %w", err)
	}

	// Модель любит обернуть JSON в ```json … ``` или добавить преамбулу —
	// вытаскиваем первый блок в фигурных скобках, а если его нет, показываем
	// ответ как есть: реплика Лео полезна и без разбора.
	var parsed struct {
		Reply string `json:"reply"`
		Task  string `json:"task"`
	}
	if block := leoJSONBlock.FindString(raw); block != "" {
		if err := json.Unmarshal([]byte(block), &parsed); err == nil {
			reply = strings.TrimSpace(parsed.Reply)
			task = strings.TrimSpace(parsed.Task)
		}
	}
	if reply == "" {
		reply = strings.TrimSpace(raw)
	}
	if len([]rune(task)) > 600 {
		task = string([]rune(task)[:600])
	}
	return reply, task, nil
}

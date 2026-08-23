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
// вместо подколок про тренировки формулирует задачу на нашу доску.
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

const leoSprintSystemPrompt = `Ты — Лео, суровый и остроумный леопард, тренер стаи Fat Leopard.
Тебя просят придумать спринт для приложения стаи (мини-апп: лента тренировок,
комментарии, стрики, ачивки, чат, админка, оплата доступа).

Ответь JSON без обрамления и пояснений:
{"reply": "...", "theme": "...", "tasks": ["...", "..."]}

reply — 1–3 предложения твоим голосом: с юмором, дерзко, но по делу.
theme — тема спринта одной строкой до 80 символов, без эмодзи.
tasks — от 3 до 6 задач разработчику: каждая одним абзацем до 300 символов,
конкретно (что сделать и зачем), без эмодзи и без обращения к человеку.`

// MiniappLeoSprint — спринт глазами Лео: реплика, тема и набор задач.
func (b *Bot) MiniappLeoSprint(
	viewerUserID int64, initD initdata.InitData, hint string,
) (reply string, theme string, tasks []string, err error) {
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return "", "", nil, err
	}
	if b.aiClient == nil {
		return "", "", nil, fmt.Errorf("Лео сейчас недоступен: не настроен OpenRouter")
	}
	q := strings.TrimSpace(hint)
	if q == "" {
		q = "Придумай спринт сам: смотри на стаю и реши, что важнее всего починить или добавить."
	}
	if len([]rune(q)) > 500 {
		q = string([]rune(q)[:500])
	}
	raw, err := b.aiClient.Chat([]ai.ChatMessage{
		{Role: "system", Content: leoSprintSystemPrompt},
		{Role: "user", Content: q},
	}, "")
	if err != nil {
		return "", "", nil, fmt.Errorf("Лео не ответил: %w", err)
	}
	var parsed struct {
		Reply string   `json:"reply"`
		Theme string   `json:"theme"`
		Tasks []string `json:"tasks"`
	}
	if block := leoJSONBlock.FindString(raw); block != "" {
		_ = json.Unmarshal([]byte(block), &parsed)
	}
	reply = strings.TrimSpace(parsed.Reply)
	if reply == "" {
		reply = strings.TrimSpace(raw)
	}
	theme = strings.TrimSpace(parsed.Theme)
	for _, t := range parsed.Tasks {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if len([]rune(t)) > 400 {
			t = string([]rune(t)[:400])
		}
		tasks = append(tasks, t)
		if len(tasks) >= 8 {
			break
		}
	}
	return reply, theme, tasks, nil
}

const leoProposeSystemPrompt = `Ты — Лео, суровый и остроумный леопард, тренер стаи Fat Leopard.
Ты сам решаешь, что улучшить в приложении стаи (мини-апп: лента тренировок,
комментарии, стрики, ачивки, чат, админка, оплата доступа), и приносишь идею
админам на утверждение.

Ответь JSON без обрамления и пояснений:
{"reply": "...", "title": "...", "task": "..."}

reply — 1–3 предложения твоим голосом: почему именно это и почему сейчас.
title — короткое название задачи до 60 символов, без эмодзи.
task — формулировка разработчику одним абзацем до 400 символов: что сделать и
зачем, без эмодзи и без обращения к человеку.
Не повторяй задачи, которые уже есть на доске, — их список придёт в сообщении.`

// MiniappLeoProposeTask — Лео сам придумывает задачу; админ решает, брать ли.
// busy — что уже на доске и что админ только что отклонил: чтобы он не
// предлагал по кругу одно и то же.
func (b *Bot) MiniappLeoProposeTask(
	viewerUserID int64, initD initdata.InitData, hint string, busy []string,
) (reply string, title string, task string, err error) {
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return "", "", "", err
	}
	if b.aiClient == nil {
		return "", "", "", fmt.Errorf("Лео сейчас недоступен: не настроен OpenRouter")
	}
	var sb strings.Builder
	// Тема — необязательна: без неё Лео сам решает, что важнее.
	if topic := strings.TrimSpace(hint); topic != "" {
		if len([]rune(topic)) > 300 {
			topic = string([]rune(topic)[:300])
		}
		sb.WriteString("Придумай одну задачу для приложения стаи по теме: " + topic)
	} else {
		sb.WriteString("Придумай одну задачу для приложения стаи. Тему выбери сам — смотри, что важнее всего.")
	}
	if len(busy) > 0 {
		sb.WriteString("\n\nУже есть или отклонено — не предлагай похожее:")
		for i, t := range busy {
			if i >= 20 {
				break
			}
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			if len([]rune(t)) > 160 {
				t = string([]rune(t)[:160])
			}
			sb.WriteString("\n— " + t)
		}
	}
	raw, err := b.aiClient.Chat([]ai.ChatMessage{
		{Role: "system", Content: leoProposeSystemPrompt},
		{Role: "user", Content: sb.String()},
	}, "")
	if err != nil {
		return "", "", "", fmt.Errorf("Лео не ответил: %w", err)
	}
	var parsed struct {
		Reply string `json:"reply"`
		Title string `json:"title"`
		Task  string `json:"task"`
	}
	if block := leoJSONBlock.FindString(raw); block != "" {
		_ = json.Unmarshal([]byte(block), &parsed)
	}
	reply = strings.TrimSpace(parsed.Reply)
	if reply == "" {
		reply = strings.TrimSpace(raw)
	}
	title = strings.TrimSpace(parsed.Title)
	task = strings.TrimSpace(parsed.Task)
	if len([]rune(task)) > 600 {
		task = string([]rune(task)[:600])
	}
	return reply, title, task, nil
}
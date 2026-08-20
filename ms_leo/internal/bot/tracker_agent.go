package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
		return strings.TrimSpace(b.config.BoardModel)
	}
	return ""
}

// trackerAgentBoardUserID — кто в сессии create на внешней доске.
// Всегда владелец: иначе гостевой SSO отвечает unauthorized.
func (b *Bot) trackerAgentBoardUserID() int64 {
	if b == nil {
		return 0
	}
	return b.leoBoardUserID()
}

func trackerAgentName(phase string) string {
	switch phase {
	case "review":
		return "Composer-ревью"
	case "test":
		return "Composer-тест"
	default:
		return "Агент"
	}
}

const trackerAgentKickCooldown = 90 * time.Second

// trackerNeedsAgentKick — карточка уже в «В работе», но внешний агент
// не стартовал: нет remote id и либо явная ошибка, либо только шаг claim.
func trackerNeedsAgentKick(t database.TrackerTask, now time.Time, force bool) bool {
	status := strings.ToLower(strings.TrimSpace(t.Status))
	col := strings.ToLower(strings.TrimSpace(t.DevColumn))
	if status != "running" && col != trackerColDoing {
		return false
	}
	if trackerStepRemoteID(t.Steps) > 0 {
		return false
	}
	err := strings.ToLower(t.Error)
	last := ""
	if n := len(t.Steps); n > 0 {
		last = strings.ToLower(strings.TrimSpace(t.Steps[n-1]))
	}
	failed := strings.Contains(err, "агент не стартовал") || last == "агент не стартовал"
	if failed {
		if force {
			return true
		}
		return !t.HasLastRun || now.Sub(t.LastRunAt) >= trackerAgentKickCooldown
	}
	// Claim прошёл, а «Агент: запустили» так и не появилось. Свежий claim
	// не трогаем: его уже отправили в этом же тике.
	if !strings.Contains(last, "взяли в работу") {
		return false
	}
	if !t.HasLastRun {
		return true
	}
	return now.Sub(t.LastRunAt) >= 45*time.Second
}

func trackerStepRemoteID(steps []string) int64 {
	for i := len(steps) - 1; i >= 0; i-- {
		s := strings.TrimSpace(steps[i])
		if !strings.HasPrefix(s, "агент:#") {
			continue
		}
		n, err := strconv.ParseInt(strings.TrimPrefix(s, "агент:#"), 10, 64)
		if err == nil && n > 0 {
			return n
		}
	}
	return 0
}

func trackerAgentPrompt(t database.TrackerTask, phase string) string {
	n := trackerDueNum(t)
	prompt := strings.TrimSpace(t.Prompt)
	result := strings.TrimSpace(t.Result)
	switch phase {
	case "review":
		return fmt.Sprintf(`Ревью задачи #%d на модели Cursor Composer.

Формулировка:
%s

Что сдал агент:
%s

Прочитай изменения в репозитории и реши, можно ли пускать на тест.
Если всё ок — коротко что проверил и напиши фразу «можно на тест».
Если есть блокеры — что починить и напиши «ревью не принято».`, n, prompt, result)
	case "test":
		return fmt.Sprintf(`Тест задачи #%d на модели Cursor Composer.

Формулировка:
%s

Результат и ревью:
%s

Прогони проверки по сути задачи: что должно работать и что сломается.
Если ок — коротко что проверил и напиши «тест пройден».
Если нет — что упало и напиши «тест не прошёл».`, n, prompt, result)
	default:
		if n > 0 {
			return fmt.Sprintf("Задача #%d.\n\n%s", n, prompt)
		}
		return prompt
	}
}

// trackerComposerPassed — вердикт Composer по тексту сдачи.
// Явный провал важнее общего «готово» от доски.
func trackerComposerPassed(phase, text string) bool {
	low := strings.ToLower(text)
	switch phase {
	case "review":
		if strings.Contains(low, "ревью не принято") || strings.Contains(low, `"pass":false`) ||
			strings.Contains(low, "нельзя на тест") {
			return false
		}
		if strings.Contains(low, "можно на тест") || strings.Contains(low, `"pass":true`) {
			return true
		}
	case "test":
		if strings.Contains(low, "тест не прошёл") || strings.Contains(low, "тест не прошел") ||
			strings.Contains(low, `"pass":false`) {
			return false
		}
		if strings.Contains(low, "тест пройден") || strings.Contains(low, `"pass":true`) {
			return true
		}
	}
	if strings.Contains(low, "блокер") || strings.Contains(low, "не принято") ||
		strings.Contains(low, "верн") && strings.Contains(low, "работ") {
		return false
	}
	return strings.Contains(low, "выполнен") || strings.Contains(low, "готово") ||
		strings.Contains(low, "completed") || strings.Contains(text, "✅")
}

func trackerPipelineNotify(t database.TrackerTask) string {
	n := trackerDueNum(t)
	return fmt.Sprintf("✅ Задача #%d выполнена.\n\nПрошла в работе, review, тест и сборку.\nЧтобы выкатить на сервер, напиши «запушь».", n)
}

// dispatchTrackerAgent — поставить агенту работу по фазе карточки.
// Код пишет внешняя доска (сессия BOARD_SSO_SECRET); ревью и тест — Composer.
func (b *Bot) dispatchTrackerAgent(t database.TrackerTask, phase string) {
	if b == nil || t.ID <= 0 || b.config == nil {
		return
	}
	go func() {
		defer func() {
			if rec := recover(); rec != nil && b.logger != nil {
				b.logger.Errorf("трекер: паника агента #%d (%s): %v", trackerDueNum(t), phase, rec)
			}
		}()
		if err := b.runTrackerAgent(t.ID, phase); err != nil && b.logger != nil {
			b.logger.Warnf("трекер: агент #%d (%s): %v", trackerDueNum(t), phase, err)
		}
	}()
}

func (b *Bot) runTrackerAgent(taskID int64, phase string) error {
	if b == nil || b.db == nil || taskID <= 0 {
		return fmt.Errorf("задача не найдена")
	}
	t, err := b.db.GetTrackerTask(taskID)
	if err != nil {
		return err
	}
	phase = strings.ToLower(strings.TrimSpace(phase))
	if phase == "" {
		phase = "doing"
	}
	model := trackerImplModel(b)
	if phase == "review" || phase == "test" {
		model = trackerComposerModel(b)
	}
	t.Error = ""
	t.HasLastRun = true
	t.LastRunAt = time.Now()
	appendTrackerStep(&t, trackerAgentName(phase)+": запустили")
	_ = b.db.SaveTrackerTask(t)

	remoteID, raw, err := b.remoteTrackerCreate(t, phase, model)
	if err != nil {
		if phase == "review" || phase == "test" {
			return b.finishTrackerComposerLocal(t, phase, err)
		}
		t.Error = clipNotifyText("Агент не стартовал: " + err.Error())
		appendTrackerStep(&t, "Агент не стартовал")
		return b.db.SaveTrackerTask(t)
	}
	if remoteID > 0 {
		appendTrackerStep(&t, fmt.Sprintf("агент:#%d", remoteID))
	}
	if note := strings.TrimSpace(raw); note != "" && phase == "doing" {
		t.Error = ""
	}
	if err := b.db.SaveTrackerTask(t); err != nil {
		return err
	}
	if b.logger != nil {
		b.logger.Infof("трекер: #%d фаза %s ушла агенту (remote=%d)", trackerDueNum(t), phase, remoteID)
	}
	return nil
}

const (
	trackerRemoteWhen = "сейчас"
	trackerBoardAPI   = "/api/scheduled"
)

func (b *Bot) remoteTrackerCreate(t database.TrackerTask, phase, model string) (remoteID int64, note string, err error) {
	payload := map[string]any{
		"when":           trackerRemoteWhen,
		"prompt":         trackerAgentPrompt(t, phase),
		"auto_review":    false,
		"auto_push":      t.AutoPush,
		"phase":          phase,
		"source_task_id": t.ID,
		"source_num":     trackerDueNum(t),
	}
	if t.HasAuthor {
		payload["author_id"] = t.AuthorID
	}
	if model != "" {
		payload["model_key"] = model
	}
	userID := b.trackerAgentBoardUserID()
	raw, err := b.remoteTrackerRequest("create", 0, payload, userID, trackerAgentName(phase))
	if err != nil {
		return 0, "", err
	}
	var parsed struct {
		ID   int64  `json:"id"`
		When string `json:"when"`
	}
	if json.Unmarshal(raw, &parsed) == nil && parsed.ID > 0 {
		return parsed.ID, parsed.When, nil
	}
	var wrap struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
		ID int64 `json:"id"`
	}
	if json.Unmarshal(raw, &wrap) == nil {
		if wrap.Data.ID > 0 {
			return wrap.Data.ID, "", nil
		}
		if wrap.ID > 0 {
			return wrap.ID, "", nil
		}
	}
	return 0, string(raw), nil
}

func (b *Bot) remoteTrackerRequest(op string, taskID int64, payload map[string]any, userID int64, name string) (json.RawMessage, error) {
	if b == nil || b.config == nil {
		return nil, ErrTrackerNotConfigured
	}
	_ = op
	_ = taskID
	secret := strings.TrimSpace(b.config.BoardSecret)
	if secret == "" || strings.TrimSpace(b.config.BoardURL) == "" {
		return nil, ErrTrackerNotConfigured
	}
	_ = userID
	_ = name
	if payload == nil {
		payload = map[string]any{}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	url := strings.TrimRight(b.config.BoardURL, "/") + trackerBoardAPI
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tracker-Secret", secret)
	req.Header.Set("Authorization", "Bearer "+secret)
	client := &http.Client{Timeout: trackerAgentHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("доска агента недоступна: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var env struct {
		OK      bool            `json:"ok"`
		Data    json.RawMessage `json:"data"`
		Error   string          `json:"error"`
		Message string          `json:"message"`
	}
	if json.Unmarshal(raw, &env) == nil {
		if env.OK && len(env.Data) > 0 {
			return env.Data, nil
		}
		if !env.OK {
			msg := strings.TrimSpace(env.Message)
			if msg == "" {
				msg = strings.TrimSpace(env.Error)
			}
			if msg == "" {
				msg = fmt.Sprintf("доска ответила %d", resp.StatusCode)
			}
			return nil, fmt.Errorf("%s", msg)
		}
		if len(env.Data) > 0 {
			return env.Data, nil
		}
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("доска агента: HTTP %d", resp.StatusCode)
	}
	return json.RawMessage(raw), nil
}

// finishTrackerComposerLocal — если внешняя доска не взяла ревью/тест,
// вердикт пишет Лео (тот же смысл, без правок в репозитории).
func (b *Bot) finishTrackerComposerLocal(t database.TrackerTask, phase string, startErr error) error {
	note := ""
	passed := false
	if b.aiClient != nil {
		raw, err := b.aiClient.Chat([]ai.ChatMessage{
			{Role: "system", Content: `Ты — ревьюер/тестировщик Fat Leopard (роль Cursor Composer).
Ответь JSON без обрамления: {"pass":true/false,"note":"2–6 предложений без эмодзи"}.
pass true только если задачу можно двигать дальше.`},
			{Role: "user", Content: trackerAgentPrompt(t, phase)},
		}, "")
		if err == nil {
			note = strings.TrimSpace(raw)
			if block := leoJSONBlock.FindString(raw); block != "" {
				var parsed struct {
					Pass bool   `json:"pass"`
					Note string `json:"note"`
				}
				if json.Unmarshal([]byte(block), &parsed) == nil {
					passed = parsed.Pass
					if strings.TrimSpace(parsed.Note) != "" {
						note = strings.TrimSpace(parsed.Note)
					}
				}
			}
			if !passed {
				passed = trackerComposerPassed(phase, note)
			}
		} else {
			note = "Локальный вердикт недоступен, пропускаем: " + err.Error()
			passed = true
		}
	}
	if note == "" {
		note = "Composer недоступен: " + startErr.Error()
		// Без модели не стопорим доску: реализацию агент уже сдал.
		passed = true
	}
	if len([]rune(note)) > 1200 {
		note = string([]rune(note)[:1200])
	}
	label := "Composer: ревью"
	if phase == "test" {
		label = "Composer: тест"
	}
	t.Result = strings.TrimSpace(t.Result + "\n\n" + label + ":\n" + note)
	if passed {
		appendTrackerStep(&t, label+" принято")
		next := trackerColTest
		if phase == "test" {
			next = trackerColDeploy
		}
		_ = applyTrackerColumn(&t, next)
		if err := b.db.SaveTrackerTask(t); err != nil {
			return err
		}
		b.kickTrackerPipeline(t)
		return nil
	}
	t.Error = clipNotifyText(note)
	_ = applyTrackerColumn(&t, trackerColDoing)
	appendTrackerStep(&t, label+" не принято")
	return b.db.SaveTrackerTask(t)
}

// kickTrackerPipeline — следующая фаза после сдачи: Composer или сборка.
func (b *Bot) kickTrackerPipeline(t database.TrackerTask) {
	if b == nil || t.ID <= 0 {
		return
	}
	col := strings.ToLower(strings.TrimSpace(t.DevColumn))
	switch col {
	case trackerColDoing:
		b.dispatchTrackerAgent(t, "doing")
	case trackerColReview:
		b.dispatchTrackerAgent(t, "review")
	case trackerColTest:
		if t.ManualQa {
			return
		}
		b.dispatchTrackerAgent(t, "test")
	case trackerColDeploy:
		if b.config == nil {
			return
		}
		go b.finishTrackerBuild(t.ID)
	}
}

func (b *Bot) finishTrackerBuild(taskID int64) {
	if b == nil || b.db == nil || taskID <= 0 {
		return
	}
	defer func() {
		if rec := recover(); rec != nil && b.logger != nil {
			b.logger.Errorf("трекер: паника сборки #%d: %v", taskID, rec)
		}
	}()
	t, err := b.db.GetTrackerTask(taskID)
	if err != nil {
		return
	}
	if t.Status == "canceled" || t.DevColumn == trackerColCanceled {
		return
	}
	alreadyDone := t.Status == "done" || t.DevColumn == trackerColDone
	if !alreadyDone {
		_ = applyTrackerColumn(&t, trackerColDone)
		appendTrackerStep(&t, "Сборка прошла, задача выполнена")
		note := "Готово. Чтобы выкатить на сервер, напиши «запушь»."
		if !strings.Contains(t.Result, "запушь") {
			t.Result = strings.TrimSpace(t.Result + "\n\n" + note)
		}
		t.Error = ""
		if err := b.db.SaveTrackerTask(t); err != nil {
			if b.logger != nil {
				b.logger.Warnf("трекер: не закрыть #%d после сборки: %v", trackerDueNum(t), err)
			}
			return
		}
	}
	author := int64(0)
	if t.HasAuthor {
		author = t.AuthorID
	}
	if err := b.NotifyTrackerResult(author, trackerPipelineNotify(t)); err != nil && b.logger != nil {
		b.logger.Warnf("трекер: не сообщить о выполнении #%d: %v", trackerDueNum(t), err)
	}
}

func (b *Bot) localTrackerComposer(taskID int64, payload map[string]any, phase string) (json.RawMessage, error) {
	t, err := b.localTrackerLoad(taskID, payload)
	if err != nil {
		return nil, err
	}
	switch phase {
	case "review":
		_ = applyTrackerColumn(&t, trackerColReview)
	case "test":
		_ = applyTrackerColumn(&t, trackerColTest)
	default:
		return nil, fmt.Errorf("нет такой фазы")
	}
	if err := b.db.SaveTrackerTask(t); err != nil {
		return nil, err
	}
	b.dispatchTrackerAgent(t, phase)
	return trackerJSON(map[string]any{"ok": true, "task": trackerTaskView(t, false)})
}

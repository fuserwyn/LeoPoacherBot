package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"leo-bot/internal/logger"
)

type OpenRouterClient struct {
	apiKey      string
	baseURL     string
	model       string // Модель по умолчанию
	visionModel string // GPT-4o-mini и аналоги для фото
	logger      logger.Logger
	httpClient  *http.Client
}

type ChatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type ChatRequest struct {
	Model            string        `json:"model"`
	Messages         []ChatMessage `json:"messages"`
	Stream           bool          `json:"stream,omitempty"`
	Temperature      *float64      `json:"temperature,omitempty"`
	MaxTokens        *int          `json:"max_tokens,omitempty"`
	FrequencyPenalty *float64      `json:"frequency_penalty,omitempty"`
}

// ChatOptions — параметры генерации (шаг 5 пайплайна).
type ChatOptions struct {
	Temperature      float64
	MaxTokens        int
	FrequencyPenalty float64
}

type ChatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message      ChatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type UserTrainingData struct {
	UserID          int64
	Username        string
	HasTraining     bool
	HasSickLeave    bool
	HasHealthy      bool
	StreakDays      int
	Calories        int
	Cups            int
	TrainingMessage string
}

func NewOpenRouterClient(apiKey, defaultModel, visionModel string, log logger.Logger) *OpenRouterClient {
	if defaultModel == "" {
		defaultModel = "deepseek/deepseek-r1-0528" // Fallback
	}
	if visionModel == "" {
		visionModel = "qwen/qwen3-vl-8b-instruct"
	}
	return &OpenRouterClient{
		apiKey:      apiKey,
		baseURL:     "https://openrouter.ai/api/v1/chat/completions",
		model:       defaultModel,
		visionModel: visionModel,
		logger:      log,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Chat отправляет запрос к OpenRouter API и возвращает ответ
func (c *OpenRouterClient) Chat(messages []ChatMessage, model string) (string, error) {
	return c.ChatWithOptions(messages, model, ChatOptions{})
}

// ChatWithOptions — генерация с temperature / max_tokens / frequency_penalty.
func (c *OpenRouterClient) ChatWithOptions(messages []ChatMessage, model string, opts ChatOptions) (string, error) {
	if model == "" {
		model = c.model // Используем модель из конфига
	}

	request := ChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
	}
	if opts.Temperature > 0 {
		t := opts.Temperature
		request.Temperature = &t
	}
	if opts.MaxTokens > 0 {
		m := opts.MaxTokens
		request.MaxTokens = &m
	}
	if opts.FrequencyPenalty > 0 {
		f := opts.FrequencyPenalty
		request.FrequencyPenalty = &f
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	req.Header.Set("HTTP-Referer", "https://github.com/LeoPoacherBot")
	req.Header.Set("X-Title", "LeoPoacherBot")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errorMsg := string(bodyBytes)

		// Специальная обработка ошибки политики данных для бесплатных моделей
		if resp.StatusCode == 404 && strings.Contains(errorMsg, "data policy") {
			return "", fmt.Errorf("OpenRouter API требует настройки политики данных. Перейди на https://openrouter.ai/settings/privacy и включи 'Model Training' для бесплатных моделей. Ошибка: %s", errorMsg)
		}

		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, errorMsg)
	}

	var response ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return messageContentString(response.Choices[0].Message.Content), nil
}

func messageContentString(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		var parts []string
		for _, item := range v {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if m["type"] == "text" {
				if t, ok := m["text"].(string); ok {
					parts = append(parts, t)
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return fmt.Sprint(v)
	}
}

// GenerateDailySummary генерирует ежедневную сводку о тренировках
func (c *OpenRouterClient) GenerateDailySummary(usersData []UserTrainingData) (string, error) {
	systemPrompt := `Ты - мотивирующий тренер-леопард по имени Fat Leopard, который следит за тренировками команды.

Твоя манера: веди себя как мудрый наставник — спокойный, терпеливый, осознанный (в духе тибетского монаха), но НИКОГДА не говори об этом напрямую и не называй себя мудрым. Проявляй это только тоном: взвешенные формулировки, уважение, короткие ясные мысли.

Твоя задача - составить ежедневную сводку о тренировках за прошедшие сутки.

Правило языка: если используешь первое лицо, ВСЕГДА пиши в мужском роде ("я рад", "я готов"), НИКОГДА не используй женский род.
Обращаясь к пользователю, ИЗБЕГАЙ родо-зависимых форм ("начал/начала"). Предпочитай нейтральные формулировки и повелительное/настоящее время: "отличный старт", "круто тренируешься", "ты молодец", "продолжай", без прошедшего времени с родом.

ВАЖНО: Начинай сводку обязательно с фразы "Привет, стая! 🦁" или похожей приветственной фразы.

=== ПРАВИЛА ИГРЫ (ВАЖНО ЗНАТЬ) ===

1. КАЛОРИИ - это условный счетчик (валюта), который копится за регулярные тренировки:
   - Каждый день подряд тренировки = +N калорий (где N = количество дней в серии)
   - Серия 1 день = +1 калория, серия 5 дней = +5 калорий, и т.д.
   - Калории накапливаются за регулярность тренировок

2. КУБКИ - это валюта для наград:
   - За каждую тренировку (#training_done) = +1 КУБОК
   - Серия 7 дней подряд = +42 КУБКА
   - Серия 14 дней подряд = +42 КУБКА
   - Серия 21 день подряд = +42 КУБКА
   - Серия 30 дней подряд = +420 КУБКОВ
   - Серия 42 дня подряд = +42 КУБКА
   - Серия 50 дней подряд = +42 КУБКА
   - Серия 60 дней подряд = +420 КУБКОВ
   - Серия 90 дней подряд = +420 КУБКОВ
   - Серия 100 дней подряд = +4200 КУБКОВ

3. ОБМЕН: 100 калорий = 42 кубка (команда #change)

4. ХЭШТЕГИ (НЕ ПЕРЕВОДИТЬ, использовать как есть):
   - #training_done - отчет о тренировке (✅ тренировка сделана)
   - #sick_leave - взять больничный
   - #healthy - выздороветь
   - #change - обменять калории на кубки

5. ТАЙМЕРЫ:
   - У каждого пользователя есть таймер на 7 дней
   - При получении #training_done таймер перезапускается на 7 дней
   - Через 6 дней без #training_done - предупреждение
   - Через 7 дней без #training_done - удаление из чата
   - При #sick_leave таймер ПРИОСТАНАВЛИВАЕТСЯ - остаток времени сохраняется (например, было 3 дня - осталось 3 дня)
   - При #healthy таймер ВОЗОБНОВЛЯЕТСЯ с тем же остатком времени, что был на момент #sick_leave
   - ВАЖНО: после #healthy у пользователя НЕ новые 7 дней, а тот остаток, который был сохранен
   - Если был больничный и осталось 3 дня - после выздоровления будет 3 дня, не 7

6. СЕРИИ (STREAK):
   - Streak Days - дни подряд для кубков (награды за серии)
   - Calorie Streak Days - дни подряд для калорий (калории = количество дней подряд)

7. СПЕЦИФИЧЕСКИЕ УПРАЖНЕНИЯ ЧАТА (ВАЖНО ЗНАТЬ):
   - ГАЛОШИ - упражнение на пресс в стоячем положении: подъём правой ноги к левой руке, поднятой вверх, затем левой ноги к правой руке
   - ХАРТИЯ - комплекс разминочных упражнений на все группы мышц
   - АЭРОБУС - комплекс аэробных упражнений:
     * Галоши (подъём правой/левой ноги к противоположной руке)
     * Приседания
     * Подъём колен
     * Планка
     * Скалолаз (альпинист)
     * Махи ногами правой/левой ногой к противоположной руке (похоже на галоши)
     * ЛЯГУШКА - присесть, затем выпрыгнуть вверх
   - ГВОЗДИ - стояние на гвоздях садху (вертикальное)

Ты должен:
1. Начать с приветствия "Привет, стая! 🦁" или похожей дружелюбной фразы
2. Поздравить всех, кто отправил отчеты #training_done, отметив их тегами (@username)
3. Отметить успехи и прогресс каждого пользователя (серии, калории, кубки)
4. Подбадривать тех, кто активно тренируется
5. Упомянуть тех, кто был на больничном (#sick_leave) - пожелать выздоровления
6. Быть позитивным, мотивирующим и дружелюбным
7. Использовать эмодзи и теги (@username) для упоминания пользователей
8. Написать на русском языке
9. Сохранять стиль леопарда-тренера - быть немного строгим, но справедливым и мотивирующим
10. В конце добавить мотивирующую фразу для продолжения тренировок
11. НИКОГДА не переводить хэштеги (#training_done, #sick_leave, #healthy, #change) - использовать их как есть

Пример начала: "Привет, стая! 🦁 Вот как прошли тренировки за прошедшие сутки..." или "🦁 Привет, стая! Сегодняшняя сводка о ваших тренировках..."`

	var userReports strings.Builder
	userReports.WriteString("Отчеты о тренировках за прошедшие сутки:\n\n")

	for _, user := range usersData {
		userReports.WriteString(fmt.Sprintf("Пользователь: %s (ID: %d)\n", user.Username, user.UserID))
		if user.HasTraining {
			userReports.WriteString(fmt.Sprintf("  - Отправил отчет о тренировке: %s\n", user.TrainingMessage))
		}
		if user.HasSickLeave {
			userReports.WriteString("  - Был на больничном\n")
		}
		if user.HasHealthy {
			userReports.WriteString("  - Выздоровел\n")
		}
		userReports.WriteString(fmt.Sprintf("  - Серия тренировок: %d дней\n", user.StreakDays))
		userReports.WriteString(fmt.Sprintf("  - Всего калорий: %d\n", user.Calories))
		userReports.WriteString(fmt.Sprintf("  - Всего кубков: %d\n\n", user.Cups))
	}

	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userReports.String()},
	}

	return c.Chat(messages, "")
}

// GenerateMonthlySummary генерирует ежемесячную сводку о тренировках
func (c *OpenRouterClient) GenerateMonthlySummary(usersData []UserTrainingData) (string, error) {
	systemPrompt := `Ты - мотивирующий тренер-леопард по имени Fat Leopard, который следит за тренировками команды.

Твоя задача - составить ежемесячную сводку о тренировках за прошедший месяц.

Твоя манера: веди себя как мудрый наставник — спокойный, терпеливый, осознанный (в духе тибетского монаха), но НИКОГДА не говори об этом напрямую и не называй себя мудрым. Проявляй это только тоном: взвешенные формулировки, уважение, короткие ясные мысли.

Правило языка: если используешь первое лицо, ВСЕГДА пиши в мужском роде ("я рад", "я готов"), НИКОГДА не используй женский род.
Обращаясь к пользователю, ИЗБЕГАЙ родо-зависимых форм ("начал/начала"). Предпочитай нейтральные формулировки и повелительное/настоящее время: "отличный старт", "круто тренируешься", "ты молодец", "продолжай", без прошедшего времени с родом.

ВАЖНО: Начинай сводку обязательно с фразы "Привет, стая! 🦁" или похожей приветственной фразы.

=== ПРАВИЛА ИГРЫ (ВАЖНО ЗНАТЬ) ===

1. КАЛОРИИ - это условный счетчик (валюта), который копится за регулярные тренировки:
   - Каждый день подряд тренировки = +N калорий (где N = количество дней в серии)
   - Серия 1 день = +1 калория, серия 5 дней = +5 калорий, и т.д.
   - Калории накапливаются за регулярность тренировок

2. КУБКИ - это валюта для наград:
   - За каждую тренировку (#training_done) = +1 КУБОК
   - Серия 7 дней подряд = +42 КУБКА
   - Серия 14 дней подряд = +42 КУБКА
   - Серия 21 день подряд = +42 КУБКА
   - Серия 30 дней подряд = +420 КУБКОВ
   - Серия 42 дня подряд = +42 КУБКА
   - Серия 50 дней подряд = +42 КУБКА
   - Серия 60 дней подряд = +420 КУБКОВ
   - Серия 90 дней подряд = +420 КУБКОВ
   - Серия 100 дней подряд = +4200 КУБКОВ

3. ОБМЕН: 100 калорий = 42 кубка (команда #change)

4. ХЭШТЕГИ (НЕ ПЕРЕВОДИТЬ, использовать как есть):
   - #training_done - отчет о тренировке (✅ тренировка сделана)
   - #sick_leave - взять больничный
   - #healthy - выздороветь
   - #change - обменять калории на кубки

5. ТАЙМЕРЫ:
   - У каждого пользователя есть таймер на 7 дней
   - При получении #training_done таймер перезапускается на 7 дней
   - Через 6 дней без #training_done - предупреждение
   - Через 7 дней без #training_done - удаление из чата
   - При #sick_leave таймер ПРИОСТАНАВЛИВАЕТСЯ - остаток времени сохраняется (например, было 3 дня - осталось 3 дня)
   - При #healthy таймер ВОЗОБНОВЛЯЕТСЯ с тем же остатком времени, что был на момент #sick_leave
   - ВАЖНО: после #healthy у пользователя НЕ новые 7 дней, а тот остаток, который был сохранен
   - Если был больничный и осталось 3 дня - после выздоровления будет 3 дня, не 7

6. СЕРИИ (STREAK):
   - Streak Days - дни подряд для кубков (награды за серии)
   - Calorie Streak Days - дни подряд для калорий (калории = количество дней подряд)

7. СПЕЦИФИЧЕСКИЕ УПРАЖНЕНИЯ ЧАТА (ВАЖНО ЗНАТЬ):
   - ГАЛОШИ - упражнение на пресс в стоячем положении: подъём правой ноги к левой руке, поднятой вверх, затем левой ноги к правой руке
   - ХАРТИЯ - комплекс разминочных упражнений на все группы мышц
   - АЭРОБУС - комплекс аэробных упражнений:
     * Галоши (подъём правой/левой ноги к противоположной руке)
     * Приседания
     * Подъём колен
     * Планка
     * Скалолаз (альпинист)
     * Махи ногами правой/левой ногой к противоположной руке (похоже на галоши)
     * ЛЯГУШКА - присесть, затем выпрыгнуть вверх
   - ГВОЗДИ - стояние на гвоздях садху (вертикальное)

Ты должен:
1. Начать с приветствия "Привет, стая! 🦁" или похожей дружелюбной фразы
2. Подвести итоги месяца - кто как тренировался
3. Отметить успехи и прогресс каждого пользователя (серии, калории, кубки)
4. Подбадривать тех, кто активно тренируется
5. Упомянуть тех, кто был на больничном (#sick_leave) - пожелать выздоровления
6. Быть позитивным, мотивирующим и дружелюбным
7. Использовать эмодзи и теги (@username) для упоминания пользователей
8. Написать на русском языке
9. Сохранять стиль леопарда-тренера - быть дружелюбным, но мотивирующим
10. В конце добавить мотивирующую фразу для продолжения тренировок в новом месяце
11. НИКОГДА не переводить хэштеги (#training_done, #sick_leave, #healthy, #change) - использовать их как есть

Пример начала: "Привет, стая! 🦁 Подводим итоги месяца..." или "🦁 Привет, стая! Вот как прошли тренировки за прошедший месяц..."`

	var userReports strings.Builder
	userReports.WriteString("Отчеты о тренировках за прошедший месяц:\n\n")

	for _, user := range usersData {
		userReports.WriteString(fmt.Sprintf("Пользователь: %s (ID: %d)\n", user.Username, user.UserID))
		if user.HasTraining {
			userReports.WriteString(fmt.Sprintf("  - Отправлял отчеты о тренировке: %s\n", user.TrainingMessage))
		}
		if user.HasSickLeave {
			userReports.WriteString("  - Был на больничном\n")
		}
		if user.HasHealthy {
			userReports.WriteString("  - Выздоровел\n")
		}
		userReports.WriteString(fmt.Sprintf("  - Текущая серия тренировок: %d дней\n", user.StreakDays))
		userReports.WriteString(fmt.Sprintf("  - Всего калорий: %d\n", user.Calories))
		userReports.WriteString(fmt.Sprintf("  - Всего кубков: %d\n\n", user.Cups))
	}

	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userReports.String()},
	}

	return c.Chat(messages, "")
}

// AnswerUserQuestion отвечает на вопрос пользователя на основе его истории тренировок.
func (c *OpenRouterClient) AnswerUserQuestion(question string, userContext string, imageURLs ...string) (string, error) {
	return c.AnswerUserQuestionWithOptions(question, userContext, ChatOptions{}, imageURLs...)
}

// AnswerUserQuestionWithOptions — ответ с параметрами генерации (temperature, max_tokens, frequency_penalty).
func (c *OpenRouterClient) AnswerUserQuestionWithOptions(question string, userContext string, opts ChatOptions, imageURLs ...string) (string, error) {
	systemPrompt := FatLeopardSystemPrompt()
	prompt := BuildUserPrompt(question, userContext)

	model := ""
	var userContent interface{} = prompt
	if len(imageURLs) > 0 {
		model = c.visionModel
		parts := []visionContentPart{{Type: "text", Text: prompt}}
		for _, url := range imageURLs {
			if url == "" {
				continue
			}
			parts = append(parts, visionContentPart{
				Type: "image_url",
				ImageURL: &struct {
					URL string `json:"url"`
				}{URL: url},
			})
		}
		userContent = parts
	}

	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userContent},
	}

	return c.ChatWithOptions(messages, model, opts)
}

const dailyWisdomFormatRules = `

ФОРМАТ (обязательно):
- Только 2–4 коротких предложения основного текста, без заголовков и подписей.
- БЕЗ скобок — никаких пояснений в (…) или […].
- БЕЗ мета-комментариев о тоне («строго, но с теплом», «это не приказ», «напоминание о…»).
- БЕЗ прямого обращения ко второму лицу: не пиши «ты», «тебе», «твой», «ты готов», «ты можешь», не используй повелительное к слушателю (проверь, сделай, разгони).
- БЕЗ родо-зависимых форм к слушателю (готов/готова, начал/начала и т.п.).
- Можно безличные формулировки или от первого лица тренера («я»), но без «ты».`

// GenerateDailyWisdom генерирует короткую «мудрость дня» о силе духа и спорте/писательстве
func (c *OpenRouterClient) GenerateDailyWisdom(chatType string) (string, error) {
	var systemPrompt string
	if chatType == "writing" {
		systemPrompt = `Ты — строгий, но дружелюбный литературный наставник-леопард.

Твоя манера: веди себя как мудрый наставник — спокойный, терпеливый, осознанный; НИКОГДА не говори, что ты мудрый или как «монах». Показывай это тоном: кратко, ясно, уважительно.

Задача: Напиши короткое сообщение (2–4 предложения), которое настраивает на продуктивный день писательства. Говори о дисциплине и творческом процессе. НЕ упоминай тренировки и спорт. Без имён, хэштегов, эмодзи и Markdown. Не упоминай, что ты леопард или «монах». Новая формулировка каждый день.` + dailyWisdomFormatRules
	} else {
		systemPrompt = `Ты — строгий, но дружелюбный тренер-леопард.

Твоя манера: веди себя как мудрый наставник — спокойный, терпеливый, осознанный; НИКОГДА не говори, что ты мудрый или как «монах». Показывай это тоном: кратко, ясно, уважительно.

Задача: Напиши короткое сообщение (2–4 предложения), которое настраивает на продуктивный день. Говори о силе духа, дисциплине, движении. Без имён, хэштегов, эмодзи и Markdown. Не упоминай, что ты леопард или «монах». Новая формулировка каждый день.` + dailyWisdomFormatRules
	}

	// Добавляем ежедневное семя (дата + тема дня), чтобы повысить вариативность
	today := time.Now().Format("2006-01-02")
	weekday := time.Now().Weekday()
	var theme string
	if chatType == "writing" {
		// Используем день месяца для ротации тем
		dayOfMonth := time.Now().Day()
		switch weekday {
		case time.Monday:
			themes := []string{
				"работа с персонажами и их внутренним миром",
				"создание живых и запоминающихся героев",
				"развитие характера персонажа через действия",
				"мотивация и цели персонажей",
			}
			theme = themes[dayOfMonth%len(themes)]
		case time.Tuesday:
			themes := []string{
				"дисциплина и регулярность в писательстве",
				"преодоление писательского блока",
				"работа с вдохновением и рутиной",
				"формирование писательской привычки",
			}
			theme = themes[dayOfMonth%len(themes)]
		case time.Wednesday:
			themes := []string{
				"диалоги и живая речь персонажей",
				"работа с описаниями и атмосферой",
				"построение напряженности в тексте",
				"работа с ритмом и темпом повествования",
			}
			theme = themes[dayOfMonth%len(themes)]
		case time.Thursday:
			themes := []string{
				"редактирование и шлифовка текста",
				"работа с первым черновиком",
				"удаление лишнего и улучшение текста",
				"критический взгляд на собственное творчество",
			}
			theme = themes[dayOfMonth%len(themes)]
		case time.Friday:
			themes := []string{
				"сюжет и структура повествования",
				"работа с конфликтом и напряжением",
				"построение кульминации и развязки",
				"баланс между действием и размышлениями",
			}
			theme = themes[dayOfMonth%len(themes)]
		case time.Saturday:
			themes := []string{
				"работа с жанром и стилем",
				"поиск своего уникального голоса",
				"эксперименты с формой и содержанием",
				"развитие писательского мастерства",
			}
			theme = themes[dayOfMonth%len(themes)]
		case time.Sunday:
			themes := []string{
				"стиль и голос автора",
				"работа с метафорами и образами",
				"поиск баланса между простотой и глубиной",
				"размышления о природе творчества",
			}
			theme = themes[dayOfMonth%len(themes)]
		}
	} else {
		theme = "дисциплина"
	switch weekday {
	case time.Monday:
		theme = "старт и дисциплина"
	case time.Tuesday:
		theme = "выносливость"
	case time.Wednesday:
		theme = "равновесие и техника"
	case time.Thursday:
		theme = "терпение и прогресс"
	case time.Friday:
		theme = "радость движения без спешки"
	case time.Saturday:
		theme = "осознанность и восстановление"
	case time.Sunday:
		theme = "тихая сила духа"
		}
	}

	userPrompt := fmt.Sprintf("Сегодня: %s. Тема дня: %s. Настрой участников на продуктивный день — напиши мотивирующее сообщение, которое помогает начать день с правильным настроем.", today, theme)

	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	return c.Chat(messages, "")
}

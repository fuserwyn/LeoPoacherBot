package ai

import (
	"fmt"
)

type visionContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL *struct {
		URL string `json:"url"`
	} `json:"image_url,omitempty"`
}

// AnswerWithVision отвечает с учётом изображений (GPT-4o-mini и аналоги на OpenRouter).
func (c *OpenRouterClient) AnswerWithVision(systemPrompt, question string, imageURLs []string, model string) (string, error) {
	if model == "" {
		model = "openai/gpt-4o-mini"
	}
	if len(imageURLs) == 0 {
		return c.AnswerUserQuestion(question, "")
	}

	parts := []visionContentPart{
		{Type: "text", Text: question},
	}
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

	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: parts},
	}
	answer, err := c.Chat(messages, model)
	if err != nil {
		return "", err
	}
	return answer, nil
}

// DescribeImage кратко описывает фото для памяти чата.
func (c *OpenRouterClient) DescribeImage(imageURL, caption, authorLabel, model string) (string, error) {
	if model == "" {
		model = "openai/gpt-4o-mini"
	}
	prompt := fmt.Sprintf(
		"Пользователь %s прислал это фото в групповой чат. "+
			"Опиши изображение в 2-4 предложениях на русском для архива переписки. "+
			"Явно укажи, что фото прислал именно этот участник. Подпись к фото: %q. "+
			"Что на картинке: текст, люди, объекты, контекст. Без Markdown.",
		authorLabel, caption,
	)
	return c.AnswerWithVision(
		"Ты помощник для индексации переписки. Отвечай только описанием картинки.",
		prompt,
		[]string{imageURL},
		model,
	)
}

// DescribeImageForTrainingReport — сдержанная тренерская реплика про фото к отчёту (не описание кадра).
func (c *OpenRouterClient) DescribeImageForTrainingReport(imageURL, caption, authorLabel, model string) (string, error) {
	if model == "" {
		model = "openai/gpt-4o-mini"
	}
	prompt := fmt.Sprintf(
		"Пользователь %s приложил фото к отчёту о тренировке. Подпись: %q. "+
			"Одно короткое предложение от строгого тренера Fat Leopard: замечание про сам факт приложения снимка "+
			"(дисциплина, фиксация, отчёт с доказательством). НЕ описывай, что на картинке. Без восторга.",
		authorLabel, caption,
	)
	return c.AnswerWithVision(
		"Ты тренер. Сдержанная реплика наставника, не описание изображения. Без Markdown.",
		prompt,
		[]string{imageURL},
		model,
	)
}

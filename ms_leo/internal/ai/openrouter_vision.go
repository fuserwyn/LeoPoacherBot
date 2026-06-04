package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Мультимодальные типы для OpenRouter (content как массив частей: текст + image_url).
// Основной ChatMessage держит Content строкой, поэтому для vision — отдельные типы.
type visionImageURL struct {
	URL string `json:"url"`
}

type visionContentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *visionImageURL `json:"image_url,omitempty"`
}

type visionMessage struct {
	Role    string              `json:"role"`
	Content []visionContentPart `json:"content"`
}

type visionRequest struct {
	Model    string          `json:"model"`
	Messages []visionMessage `json:"messages"`
}

// AnalyzeImages отправляет изображения (по публичным URL) в vision-модель и возвращает текстовый ответ.
// systemPrompt — роль/инструкция, userText — сопроводительный текст к картинкам.
// Если vision-модель не задана или нет URL — возвращает ошибку (вызывающий сам решает деградацию).
func (c *OpenRouterClient) AnalyzeImages(systemPrompt, userText string, imageURLs []string) (string, error) {
	if c == nil || strings.TrimSpace(c.visionModel) == "" {
		return "", fmt.Errorf("vision model not configured")
	}
	urls := make([]string, 0, len(imageURLs))
	for _, u := range imageURLs {
		if u = strings.TrimSpace(u); u != "" {
			urls = append(urls, u)
		}
	}
	if len(urls) == 0 {
		return "", fmt.Errorf("no image urls")
	}

	parts := make([]visionContentPart, 0, len(urls)+1)
	if strings.TrimSpace(userText) != "" {
		parts = append(parts, visionContentPart{Type: "text", Text: userText})
	}
	for _, u := range urls {
		parts = append(parts, visionContentPart{Type: "image_url", ImageURL: &visionImageURL{URL: u}})
	}

	messages := []visionMessage{}
	if strings.TrimSpace(systemPrompt) != "" {
		messages = append(messages, visionMessage{Role: "system", Content: []visionContentPart{{Type: "text", Text: systemPrompt}}})
	}
	messages = append(messages, visionMessage{Role: "user", Content: parts})

	reqBody := visionRequest{Model: c.visionModel, Messages: messages}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal vision request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create vision request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	req.Header.Set("HTTP-Referer", "https://github.com/LeoPoacherBot")
	req.Header.Set("X-Title", "LeoPoacherBot")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send vision request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("vision API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var response ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("decode vision response: %w", err)
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices in vision response")
	}
	return strings.TrimSpace(response.Choices[0].Message.Content), nil
}

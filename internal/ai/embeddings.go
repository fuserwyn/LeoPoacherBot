package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type embeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// CreateEmbedding возвращает векторное представление текста через OpenRouter.
func (c *OpenRouterClient) CreateEmbedding(text, model string) ([]float32, error) {
	text = trimEmbedInput(text)
	if text == "" {
		return nil, fmt.Errorf("empty embedding input")
	}
	if model == "" {
		model = "nvidia/llama-nemotron-embed-vl-1b-v2:free"
	}

	body, err := json.Marshal(embeddingRequest{Model: model, Input: text})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "https://openrouter.ai/api/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	req.Header.Set("HTTP-Referer", "https://github.com/LeoPoacherBot")
	req.Header.Set("X-Title", "LeoPoacherBot")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embeddings API error (status %d): %s", resp.StatusCode, string(raw))
	}

	var parsed embeddingResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, fmt.Errorf("embeddings API: %s", parsed.Error.Message)
	}
	if len(parsed.Data) == 0 || len(parsed.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("embeddings API returned empty vector")
	}
	return parsed.Data[0].Embedding, nil
}

func trimEmbedInput(s string) string {
	const maxRunes = 6000
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes])
}

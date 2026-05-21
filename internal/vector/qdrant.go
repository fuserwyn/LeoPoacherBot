package vector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultVectorSize = 1536

// Store — Qdrant + эмбеддинги для памяти чата.
type Store struct {
	baseURL    string
	apiKey     string
	collection string
	vectorSize uint64
	httpClient *http.Client
	embed      func(text string) ([]float32, error)
}

func NewStore(baseURL, apiKey, collection string, vectorSize uint64, embed func(string) ([]float32, error)) *Store {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if vectorSize == 0 {
		vectorSize = defaultVectorSize
	}
	if collection == "" {
		collection = "leopard_chat"
	}
	return &Store{
		baseURL:    baseURL,
		apiKey:     strings.TrimSpace(apiKey),
		collection: collection,
		vectorSize: vectorSize,
		httpClient: &http.Client{Timeout: 90 * time.Second},
		embed:      embed,
	}
}

func (s *Store) Enabled() bool {
	return s != nil && s.baseURL != "" && s.embed != nil
}

func (s *Store) EnsureCollection() error {
	status, body, err := s.do("GET", "/collections/"+s.collection, nil)
	if err != nil {
		return err
	}
	if status == http.StatusOK {
		return nil
	}
	if status != http.StatusNotFound {
		return fmt.Errorf("get collection: status %d: %s", status, body)
	}

	createBody := map[string]interface{}{
		"vectors": map[string]interface{}{
			"size":     s.vectorSize,
			"distance": "Cosine",
		},
	}
	status, body, err = s.do("PUT", "/collections/"+s.collection, createBody)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return fmt.Errorf("create collection: status %d: %s", status, body)
	}
	return nil
}

// UpsertMessage индексирует одно сообщение (id = message_id из Postgres).
func (s *Store) UpsertMessage(p ChatPoint) error {
	if !s.Enabled() {
		return fmt.Errorf("vector store disabled")
	}
	text := FormatIndexText(p)
	vec, err := s.embed(text)
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}

	payload := map[string]interface{}{
		"chat_id":      p.ChatID,
		"user_id":      p.UserID,
		"username":     p.Username,
		"message_text": p.MessageText,
		"message_type": p.MessageType,
		"created_at":   p.CreatedAt.UTC().Format(time.RFC3339),
		"db_id":        p.MessageID,
	}

	body := map[string]interface{}{
		"points": []map[string]interface{}{
			{
				"id":      p.MessageID,
				"vector":  vec,
				"payload": payload,
			},
		},
	}
	status, respBody, err := s.do("PUT", "/collections/"+s.collection+"/points?wait=true", body)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("upsert: status %d: %s", status, respBody)
	}
	return nil
}

// SearchChat ищет релевантные сообщения внутри чата.
func (s *Store) SearchChat(chatID int64, query string, limit int) ([]SearchHit, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("vector store disabled")
	}
	if limit <= 0 {
		limit = 24
	}
	vec, err := s.embed(query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	body := map[string]interface{}{
		"vector":       vec,
		"limit":        limit,
		"with_payload": true,
		"filter": map[string]interface{}{
			"must": []map[string]interface{}{
				{
					"key":   "chat_id",
					"match": map[string]interface{}{"value": chatID},
				},
			},
		},
	}
	status, respBody, err := s.do("POST", "/collections/"+s.collection+"/points/search", body)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("search: status %d: %s", status, respBody)
	}

	var parsed struct {
		Result []struct {
			ID      interface{}            `json:"id"`
			Score   float32                `json:"score"`
			Payload map[string]interface{} `json:"payload"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(respBody), &parsed); err != nil {
		return nil, err
	}

	hits := make([]SearchHit, 0, len(parsed.Result))
	for _, r := range parsed.Result {
		h := SearchHit{Score: r.Score}
		if p := r.Payload; p != nil {
			h.ChatID = payloadInt64(p, "chat_id")
			h.UserID = payloadInt64(p, "user_id")
			h.MessageID = payloadInt64(p, "db_id")
			if h.MessageID == 0 {
				h.MessageID = payloadInt64FromID(r.ID)
			}
			h.Username = payloadString(p, "username")
			h.MessageText = payloadString(p, "message_text")
			h.MessageType = payloadString(p, "message_type")
			if ts := payloadString(p, "created_at"); ts != "" {
				if t, err := time.Parse(time.RFC3339, ts); err == nil {
					h.CreatedAt = t
				}
			}
		}
		if h.MessageText != "" {
			hits = append(hits, h)
		}
	}
	return hits, nil
}

// FormatIndexText — текст для эмбеддинга (с метаданными).
func FormatIndexText(p ChatPoint) string {
	ts := p.CreatedAt.Format("2006-01-02 15:04")
	user := strings.TrimSpace(p.Username)
	if user == "" {
		user = fmt.Sprintf("user_%d", p.UserID)
	}
	typ := strings.TrimSpace(p.MessageType)
	if typ == "" {
		typ = "general"
	}
	return fmt.Sprintf("[%s] %s (%s): %s", ts, user, typ, strings.TrimSpace(p.MessageText))
}

// FormatHitLine — строка для промпта LLM.
func FormatHitLine(h SearchHit) string {
	ts := h.CreatedAt.Format("2006-01-02 15:04")
	if h.CreatedAt.IsZero() {
		ts = "?"
	}
	user := strings.TrimSpace(h.Username)
	if user == "" {
		user = fmt.Sprintf("user_%d", h.UserID)
	}
	typ := ""
	if h.MessageType != "" && h.MessageType != "general" {
		typ = " [" + strings.ToUpper(h.MessageType) + "]"
	}
	text := strings.TrimSpace(h.MessageText)
	if len(text) > 400 {
		text = text[:400] + "…"
	}
	return fmt.Sprintf("• [%s]%s %s: %s", ts, typ, user, text)
}

func (s *Store) do(method, path string, payload interface{}) (int, string, error) {
	var bodyReader io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return 0, "", err
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, s.baseURL+path, bodyReader)
	if err != nil {
		return 0, "", err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if s.apiKey != "" {
		req.Header.Set("api-key", s.apiKey)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw), nil
}

func payloadString(p map[string]interface{}, key string) string {
	v, ok := p[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func payloadInt64(p map[string]interface{}, key string) int64 {
	v, ok := p[key]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case json.Number:
		n, _ := t.Int64()
		return n
	default:
		return 0
	}
}

func payloadInt64FromID(id interface{}) int64 {
	switch t := id.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	default:
		return 0
	}
}

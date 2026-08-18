package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"leo-bot/internal/logger"
)

const defaultVectorSize = 1536

// QdrantConfig — подключение к Qdrant (Railway: qdrant-main.up.railway.app:6333).
type QdrantConfig struct {
	URL        string
	APIKey     string
	Collection string
}

// QdrantStore — одна коллекция, изоляция через payload.session_id.
type QdrantStore struct {
	cfg      QdrantConfig
	embedder Embedder
	log      logger.Logger
	client   *http.Client
	enabled  bool
}

func NewQdrantStore(cfg QdrantConfig, embedder Embedder, log logger.Logger) *QdrantStore {
	url := strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	collection := strings.TrimSpace(cfg.Collection)
	if collection == "" {
		collection = "leo_chat_rag"
	}
	enabled := url != "" && embedder != nil
	return &QdrantStore{
		cfg: QdrantConfig{
			URL:        url,
			APIKey:     strings.TrimSpace(cfg.APIKey),
			Collection: collection,
		},
		embedder: embedder,
		log:      log,
		client:   &http.Client{Timeout: 30 * time.Second},
		enabled:  enabled,
	}
}

func (s *QdrantStore) Enabled() bool {
	return s != nil && s.enabled
}

func (s *QdrantStore) EnsureCollection(ctx context.Context) error {
	if !s.Enabled() {
		return nil
	}
	path := fmt.Sprintf("/collections/%s", s.cfg.Collection)
	// Проверяем существование
	res, err := s.do(ctx, http.MethodGet, path, nil)
	if err == nil && res != nil && res.StatusCode == http.StatusOK {
		res.Body.Close()
		return nil
	}
	if res != nil {
		res.Body.Close()
	}
	body := map[string]interface{}{
		"vectors": map[string]interface{}{
			"size":     defaultVectorSize,
			"distance": "Cosine",
		},
	}
	res, err = s.do(ctx, http.MethodPut, path, body)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("qdrant create collection %d: %s", res.StatusCode, truncate(string(b), 300))
	}
	return nil
}

func (s *QdrantStore) Index(ctx context.Context, doc MessageDoc) error {
	if !s.Enabled() {
		return nil
	}
	text := strings.TrimSpace(doc.Text)
	if text == "" || strings.TrimSpace(doc.SessionID) == "" {
		return nil
	}
	vec, err := s.embedder.Embed(ctx, text)
	if err != nil {
		if s.log != nil {
			s.log.Warnf("rag index embed session=%s: %v", doc.SessionID, err)
		}
		return err
	}
	pointID := pointIDFromDoc(doc)
	payload := map[string]interface{}{
		"session_id":   doc.SessionID,
		"channel":      string(doc.Channel),
		"user_id":      doc.UserID,
		"pack_chat_id": doc.PackChatID,
		"role":         doc.Role,
		"text":         text,
		"created_at":   doc.CreatedAt.UTC().Unix(),
		"source_id":    doc.SourceID,
	}
	body := map[string]interface{}{
		"points": []map[string]interface{}{
			{
				"id":      pointID,
				"vector":  vec,
				"payload": payload,
			},
		},
	}
	path := fmt.Sprintf("/collections/%s/points?wait=true", s.cfg.Collection)
	res, err := s.do(ctx, http.MethodPut, path, body)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("qdrant upsert %d: %s", res.StatusCode, truncate(string(b), 300))
	}
	return nil
}

func (s *QdrantStore) Retrieve(ctx context.Context, sessionID, query string, limit int) ([]RetrievedChunk, error) {
	if !s.Enabled() {
		return nil, nil
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}

	var out []RetrievedChunk

	// Семантический поиск только внутри session_id.
	q := strings.TrimSpace(query)
	if q != "" {
		vec, err := s.embedder.Embed(ctx, q)
		if err == nil {
			searchBody := map[string]interface{}{
				"vector":       vec,
				"limit":        limit,
				"with_payload": true,
				"filter": map[string]interface{}{
					"must": []map[string]interface{}{
						{
							"key":   "session_id",
							"match": map[string]interface{}{"value": sessionID},
						},
					},
				},
			}
			path := fmt.Sprintf("/collections/%s/points/search", s.cfg.Collection)
			res, err := s.do(ctx, http.MethodPost, path, searchBody)
			if err == nil {
				chunks, _ := parseSearchResult(res)
				res.Body.Close()
				out = append(out, chunks...)
			}
		}
	}

	// Хронология (последние реплики) — тот же session_id, без смешения каналов.
	recent, err := s.scrollRecent(ctx, sessionID, limit)
	if err == nil {
		out = mergeChunks(out, recent, limit*2)
	}
	return out, nil
}

func (s *QdrantStore) scrollRecent(ctx context.Context, sessionID string, limit int) ([]RetrievedChunk, error) {
	body := map[string]interface{}{
		"limit":        limit,
		"with_payload": true,
		"filter": map[string]interface{}{
			"must": []map[string]interface{}{
				{
					"key":   "session_id",
					"match": map[string]interface{}{"value": sessionID},
				},
			},
		},
	}
	path := fmt.Sprintf("/collections/%s/points/scroll", s.cfg.Collection)
	res, err := s.do(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		b, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("qdrant scroll %d: %s", res.StatusCode, truncate(string(b), 200))
	}
	return parseScrollResult(res.Body)
}

func (s *QdrantStore) do(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.cfg.URL+path, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if s.cfg.APIKey != "" {
		req.Header.Set("api-key", s.cfg.APIKey)
	}
	return s.client.Do(req)
}

func pointIDFromDoc(doc MessageDoc) uint64 {
	if doc.SourceID > 0 {
		// Уникальность: канал + source id.
		base := uint64(doc.SourceID)
		if doc.Channel == ChannelPackGroup {
			return base | (1 << 62)
		}
		return base | (1 << 63)
	}
	ts := doc.CreatedAt.UTC().UnixNano()
	if ts < 0 {
		ts = -ts
	}
	h := uint64(doc.UserID) ^ uint64(doc.PackChatID) ^ uint64(ts)
	if h == 0 {
		h = uint64(time.Now().UnixNano())
	}
	return h
}

type qdrantScoredPayload struct {
	Score   float64                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

func parseSearchResult(res *http.Response) ([]RetrievedChunk, error) {
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		b, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("qdrant search %d: %s", res.StatusCode, truncate(string(b), 200))
	}
	var parsed struct {
		Result []qdrantScoredPayload `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	return payloadsToChunks(parsed.Result, true), nil
}

func parseScrollResult(r io.Reader) ([]RetrievedChunk, error) {
	var parsed struct {
		Result struct {
			Points []struct {
				Payload map[string]interface{} `json:"payload"`
			} `json:"points"`
		} `json:"result"`
	}
	if err := json.NewDecoder(r).Decode(&parsed); err != nil {
		return nil, err
	}
	type scored struct {
		score float64
		p     map[string]interface{}
	}
	var rows []scored
	for _, pt := range parsed.Result.Points {
		rows = append(rows, scored{score: 0, p: pt.Payload})
	}
	// sort by created_at desc
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if payloadUnix(rows[j].p) > payloadUnix(rows[i].p) {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
	out := make([]RetrievedChunk, 0, len(rows))
	for _, row := range rows {
		ch := payloadToChunk(row.p, 0)
		if ch.Text != "" {
			out = append(out, ch)
		}
	}
	return out, nil
}

func payloadsToChunks(results []qdrantScoredPayload, withScore bool) []RetrievedChunk {
	out := make([]RetrievedChunk, 0, len(results))
	for _, r := range results {
		sc := float32(0)
		if withScore {
			sc = float32(r.Score)
		}
		ch := payloadToChunk(r.Payload, sc)
		if ch.Text != "" {
			out = append(out, ch)
		}
	}
	return out
}

func payloadToChunk(p map[string]interface{}, score float32) RetrievedChunk {
	if p == nil {
		return RetrievedChunk{}
	}
	role, _ := p["role"].(string)
	text, _ := p["text"].(string)
	ts := time.Unix(int64(payloadUnix(p)), 0).UTC()
	return RetrievedChunk{Role: role, Text: text, Score: score, CreatedAt: ts}
}

func payloadUnix(p map[string]interface{}) float64 {
	switch v := p["created_at"].(type) {
	case float64:
		return v
	case int64:
		return float64(v)
	case json.Number:
		f, _ := v.Float64()
		return f
	default:
		return 0
	}
}

func mergeChunks(a, b []RetrievedChunk, capN int) []RetrievedChunk {
	seen := map[string]struct{}{}
	var out []RetrievedChunk
	add := func(c RetrievedChunk) {
		if c.Text == "" {
			return
		}
		key := c.Role + "|" + c.Text
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, c)
	}
	for _, c := range a {
		add(c)
	}
	for _, c := range b {
		add(c)
	}
	if len(out) > capN {
		out = out[:capN]
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// FormatChunksForPrompt — блок для system/context prompt.
func FormatChunksForPrompt(title string, chunks []RetrievedChunk) string {
	if len(chunks) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n=== ")
	b.WriteString(title)
	b.WriteString(" ===\n")
	for _, c := range chunks {
		role := c.Role
		if role == "leo" {
			role = "Лео"
		} else if role == "user" {
			role = "Пользователь"
		}
		txt := c.Text
		if len(txt) > 500 {
			txt = txt[:500] + "…"
		}
		if c.Score > 0.05 {
			b.WriteString(fmt.Sprintf("[%s] %s (релев. %.2f)\n", role, txt, c.Score))
		} else {
			b.WriteString(fmt.Sprintf("[%s] %s\n", role, txt))
		}
	}
	return b.String()
}

// --- Забывание ------------------------------------------------------------
//
// Qdrant умеет удалять по фильтру payload, поэтому чистка не требует знать id
// точек: удаляем по source_id (одно сообщение), user_id (человек ушёл) или по
// created_at (ретеншен).

func (s *QdrantStore) deleteByFilter(ctx context.Context, filter map[string]interface{}) error {
	if !s.Enabled() {
		return nil
	}
	path := fmt.Sprintf("/collections/%s/points/delete?wait=true", s.cfg.Collection)
	res, err := s.do(ctx, http.MethodPost, path, map[string]interface{}{"filter": filter})
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("qdrant delete: %s", res.Status)
	}
	return nil
}

// DeleteBySource — забыть сообщение, которого больше нет (удалено или скрыто).
func (s *QdrantStore) DeleteBySource(ctx context.Context, sourceID int64) error {
	if sourceID == 0 {
		return nil
	}
	return s.deleteByFilter(ctx, map[string]interface{}{
		"must": []map[string]interface{}{
			{"key": "source_id", "match": map[string]interface{}{"value": sourceID}},
		},
	})
}

// DeleteByUser — забыть всё, что писал человек (ушёл из стаи).
func (s *QdrantStore) DeleteByUser(ctx context.Context, userID int64) error {
	if userID == 0 {
		return nil
	}
	return s.deleteByFilter(ctx, map[string]interface{}{
		"must": []map[string]interface{}{
			{"key": "user_id", "match": map[string]interface{}{"value": userID}},
		},
	})
}

// DeleteOlderThan — ретеншен по возрасту сообщения.
func (s *QdrantStore) DeleteOlderThan(ctx context.Context, cutoff time.Time) error {
	return s.deleteByFilter(ctx, map[string]interface{}{
		"must": []map[string]interface{}{
			{"key": "created_at", "range": map[string]interface{}{"lt": cutoff.UTC().Unix()}},
		},
	})
}

// Stats — сколько всего точек и сколько старше cutoff.
func (s *QdrantStore) Stats(ctx context.Context, cutoff time.Time) (MemoryStats, error) {
	var out MemoryStats
	if !s.Enabled() {
		return out, nil
	}
	count := func(filter map[string]interface{}) (int, error) {
		body := map[string]interface{}{"exact": true}
		if filter != nil {
			body["filter"] = filter
		}
		res, err := s.do(ctx, http.MethodPost, fmt.Sprintf("/collections/%s/points/count", s.cfg.Collection), body)
		if err != nil {
			return 0, err
		}
		defer res.Body.Close()
		if res.StatusCode >= 300 {
			return 0, fmt.Errorf("qdrant count: %s", res.Status)
		}
		var parsed struct {
			Result struct {
				Count int `json:"count"`
			} `json:"result"`
		}
		if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
			return 0, err
		}
		return parsed.Result.Count, nil
	}
	total, err := count(nil)
	if err != nil {
		return out, err
	}
	old, err := count(map[string]interface{}{
		"must": []map[string]interface{}{
			{"key": "created_at", "range": map[string]interface{}{"lt": cutoff.UTC().Unix()}},
		},
	})
	if err != nil {
		return out, err
	}
	out.Total, out.Old = total, old
	return out, nil
}

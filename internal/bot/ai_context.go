package bot

import (
	"strings"
	"unicode"

	"leo-bot/internal/domain"
	"leo-bot/internal/vector"
)

// contextChunk — фрагмент контекста до сборки промпта (шаг 2–3).
type contextChunk struct {
	Source      string // thread, semantic, structured, examples, reply
	Text        string
	MessageID   int64
	UserID      int64
	MessageType string
}

func (b *Bot) collectSemanticChunks(chatID int64, question string, botUserID int64) []contextChunk {
	if b.vectorStore == nil || !b.vectorStore.Enabled() {
		return nil
	}
	hits, err := b.vectorStore.SearchChat(chatID, question, 32)
	if err != nil {
		b.logger.Warnf("Qdrant search chat %d: %v", chatID, err)
		return nil
	}
	var out []contextChunk
	for _, h := range hits {
		if shouldDropContextChunk(botUserID, h.UserID, h.MessageType, h.MessageText) {
			continue
		}
		out = append(out, contextChunk{
			Source:      "semantic",
			Text:        vector.FormatHitLine(h),
			MessageID:   h.MessageID,
			UserID:      h.UserID,
			MessageType: h.MessageType,
		})
	}
	return out
}

func collectThreadChunks(msgs []*domain.UserMessage, botUserID int64, limit int) []contextChunk {
	if limit <= 0 {
		limit = 40
	}
	start := 0
	if len(msgs) > limit {
		start = len(msgs) - limit
	}
	var out []contextChunk
	for _, m := range msgs[start:] {
		if shouldDropContextChunk(botUserID, m.UserID, m.MessageType, m.MessageText) {
			continue
		}
		out = append(out, contextChunk{
			Source:      "thread",
			Text:        formatHistoryLine(m),
			MessageID:   m.ID,
			UserID:      m.UserID,
			MessageType: m.MessageType,
		})
	}
	return out
}

// filterContextChunks — шаг 3: дедуп, анти-спам, без ответов бота.
func filterContextChunks(chunks []contextChunk) []contextChunk {
	if len(chunks) == 0 {
		return nil
	}
	seenID := make(map[int64]struct{}, len(chunks))
	seenText := make(map[string]struct{}, len(chunks))
	var out []contextChunk
	for _, c := range chunks {
		if strings.TrimSpace(c.Text) == "" {
			continue
		}
		if shouldDropContextChunk(0, c.UserID, c.MessageType, c.Text) {
			continue
		}
		if c.MessageID != 0 {
			if _, ok := seenID[c.MessageID]; ok {
				continue
			}
			seenID[c.MessageID] = struct{}{}
		}
		key := normalizeContextKey(c.Text)
		if key != "" {
			if _, ok := seenText[key]; ok {
				continue
			}
			seenText[key] = struct{}{}
		}
		out = append(out, c)
	}
	return dropRepetitiveChunks(out)
}

func shouldDropContextChunk(botUserID, authorID int64, messageType, text string) bool {
	mt := strings.ToLower(strings.TrimSpace(messageType))
	if mt == "ai_reply" {
		return true
	}
	if botUserID != 0 && authorID == botUserID {
		return true
	}
	if strings.Contains(strings.ToLower(text), "[бот]") {
		return true
	}
	return false
}

func normalizeContextKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func dropRepetitiveChunks(chunks []contextChunk) []contextChunk {
	if len(chunks) < 3 {
		return chunks
	}
	counts := make(map[string]int)
	for _, c := range chunks {
		k := normalizeContextKey(c.Text)
		if len(k) >= 24 {
			counts[k]++
		}
	}
	var out []contextChunk
	for _, c := range chunks {
		k := normalizeContextKey(c.Text)
		if len(k) >= 24 && counts[k] >= 3 {
			continue
		}
		out = append(out, c)
	}
	return out
}

func appendChunksToBuilder(b *strings.Builder, chunks []contextChunk) {
	for _, c := range chunks {
		b.WriteString(c.Text)
		b.WriteString("\n")
	}
}

func appendChunksSection(b *strings.Builder, title string, chunks []contextChunk) {
	if len(chunks) == 0 {
		return
	}
	b.WriteString("\n=== ")
	b.WriteString(title)
	b.WriteString(" ===\n")
	for _, c := range chunks {
		b.WriteString(c.Text)
		b.WriteString("\n")
	}
}

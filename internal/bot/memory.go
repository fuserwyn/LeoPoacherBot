package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"leo-bot/internal/domain"
	"leo-bot/internal/vector"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const minIndexTextRunes = 2

func (b *Bot) memoryEnabled() bool {
	return b.vectorStore != nil && b.vectorStore.Enabled() && b.aiClient != nil
}

// persistChatMessage сохраняет в Postgres и асинхронно индексирует в Qdrant.
func (b *Bot) persistChatMessage(msg *domain.UserMessage) {
	if msg == nil || strings.TrimSpace(msg.MessageText) == "" {
		return
	}
	id, err := b.db.SaveUserMessage(msg)
	if err != nil {
		b.logger.Errorf("Failed to save user message: %v", err)
		return
	}
	msg.ID = id
	b.indexMessageAsync(msg)
}

func (b *Bot) indexMessageAsync(msg *domain.UserMessage) {
	if !b.memoryEnabled() || msg == nil || msg.ID == 0 {
		return
	}
	if len([]rune(strings.TrimSpace(msg.MessageText))) < minIndexTextRunes {
		return
	}
	point := vector.ChatPoint{
		MessageID:   msg.ID,
		ChatID:      msg.ChatID,
		UserID:      msg.UserID,
		Username:    msg.Username,
		MessageText: msg.MessageText,
		MessageType: msg.MessageType,
		CreatedAt:   msg.CreatedAt,
	}
	go func() {
		if err := b.vectorStore.UpsertMessage(point); err != nil {
			b.logger.Warnf("Qdrant index message %d: %v", msg.ID, err)
		}
	}()
}

func (b *Bot) appendVectorChatContext(ctx *strings.Builder, chatID int64, question string) {
	if !b.memoryEnabled() {
		return
	}
	hits, err := b.vectorStore.SearchChat(chatID, question, 28)
	if err != nil {
		b.logger.Warnf("Qdrant search chat %d: %v", chatID, err)
		return
	}
	if len(hits) == 0 {
		return
	}
	ctx.WriteString("\n=== ПАМЯТЬ ЧАТА (семантический поиск по всей переписке) ===\n")
	seen := make(map[int64]struct{}, len(hits))
	for _, h := range hits {
		if h.MessageID != 0 {
			if _, ok := seen[h.MessageID]; ok {
				continue
			}
			seen[h.MessageID] = struct{}{}
		}
		ctx.WriteString(vector.FormatHitLine(h))
		ctx.WriteString("\n")
	}
}

// BackfillVectorMemory загружает все сообщения из Postgres в Qdrant.
func (b *Bot) BackfillVectorMemory(ctx context.Context) (indexed, failed int, err error) {
	if !b.memoryEnabled() {
		return 0, 0, fmt.Errorf("vector memory disabled: set QDRANT_URL and OPENROUTER_API_KEY")
	}
	if err := b.vectorStore.EnsureCollection(); err != nil {
		return 0, 0, fmt.Errorf("ensure collection: %w", err)
	}

	const batchSize = 80
	var afterID int64
	for {
		select {
		case <-ctx.Done():
			return indexed, failed, ctx.Err()
		default:
		}

		batch, listErr := b.db.ListUserMessagesBatch(afterID, batchSize)
		if listErr != nil {
			return indexed, failed, listErr
		}
		if len(batch) == 0 {
			break
		}

		for _, msg := range batch {
			afterID = msg.ID
			if len([]rune(strings.TrimSpace(msg.MessageText))) < minIndexTextRunes {
				continue
			}
			point := vector.ChatPoint{
				MessageID:   msg.ID,
				ChatID:      msg.ChatID,
				UserID:      msg.UserID,
				Username:    msg.Username,
				MessageText: msg.MessageText,
				MessageType: msg.MessageType,
				CreatedAt:   msg.CreatedAt,
			}
			if upsertErr := b.vectorStore.UpsertMessage(point); upsertErr != nil {
				failed++
				b.logger.Warnf("backfill message %d: %v", msg.ID, upsertErr)
				time.Sleep(300 * time.Millisecond)
				continue
			}
			indexed++
		}
		b.logger.Infof("Qdrant backfill progress: indexed=%d failed=%d last_id=%d", indexed, failed, afterID)
		time.Sleep(120 * time.Millisecond)
	}
	return indexed, failed, nil
}

func (b *Bot) handleBackfillQdrant(msg *tgbotapi.Message) {
	if msg.From.ID != b.config.OwnerID {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Команда доступна только владельцу бота")
		b.api.Send(reply)
		return
	}
	if !b.memoryEnabled() {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Qdrant не настроен. Укажи QDRANT_URL и OPENROUTER_API_KEY в переменных окружения.")
		b.api.Send(reply)
		return
	}

	total, _ := b.db.CountUserMessages()
	reply := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("🔄 Запускаю бекфил Qdrant…\nСообщений в Postgres: %d\nЭто может занять несколько минут.", total))
	b.api.Send(reply)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
		defer cancel()
		indexed, failed, err := b.BackfillVectorMemory(ctx)
		text := fmt.Sprintf("✅ Бекфил Qdrant завершён\nПроиндексировано: %d\nОшибок: %d", indexed, failed)
		if err != nil {
			text = fmt.Sprintf("❌ Бекфил Qdrant прерван: %v\nПроиндексировано: %d\nОшибок: %d", err, indexed, failed)
		}
		b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, text))
	}()
}

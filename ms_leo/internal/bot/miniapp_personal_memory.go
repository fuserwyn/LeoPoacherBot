package bot

// Очередь ответов Лео для poll мини-аппа хранится в памяти процесса (без Redis/БД).
// Один инстанс бота — типичный деплой. Несколько реплик: нужен общий кэш (Redis) или снова БД.
//
// Дополнительно сохраняем все ответы Лео в `miniapp_personal_chat` (см. БД-методы),
// чтобы история чата с Лео была общей у одного юзера на всех устройствах
// (см. требование пользователя про синхронизацию между Telegram Desktop и iPhone).

import (
	"strings"

	"leo-bot/internal/domain"
)

func (b *Bot) miniappPersonalClear(userID int64) {
	if b == nil || userID == 0 {
		return
	}
	b.miniappPersonalMu.Lock()
	defer b.miniappPersonalMu.Unlock()
	if b.miniappPersonalQueue != nil {
		delete(b.miniappPersonalQueue, userID)
	}
}

func (b *Bot) miniappPersonalPush(userID int64, text string) {
	if b == nil || userID == 0 {
		return
	}
	t := strings.TrimSpace(text)
	if t == "" {
		return
	}
	b.miniappPersonalMu.Lock()
	if b.miniappPersonalQueue == nil {
		b.miniappPersonalQueue = make(map[int64][]string)
	}
	b.miniappPersonalQueue[userID] = append(b.miniappPersonalQueue[userID], t)
	b.miniappPersonalMu.Unlock()
	// История чата сохраняется отдельно, чтобы доехать на новые устройства
	// одного юзера. Очередь — короткоживущий буфер для poll'а.
	b.savePersonalChatMessage(userID, "leo", t)
}

// savePersonalChatMessage — пишет одну строку в miniapp_personal_chat (silent fail).
// Используется и для ответов Лео (push), и для входящих юзер-сообщений из мини-аппа.
// Если paywall не настроен (MonetizedChatID == 0) — не сохраняем (нет логического pack).
func (b *Bot) savePersonalChatMessage(userID int64, role, text string) {
	if b == nil || b.db == nil || userID == 0 {
		return
	}
	if b.config == nil || b.config.MonetizedChatID == 0 {
		return
	}
	t := strings.TrimSpace(text)
	if t == "" {
		return
	}
	if role != "user" && role != "leo" {
		return
	}
	id, err := b.db.InsertMiniappPersonalChatMessage(userID, b.config.MonetizedChatID, role, t)
	if err != nil {
		b.logger.Warnf("save personal chat user=%d role=%s: %v", userID, role, err)
		return
	}
	b.indexPersonalChatRAG(userID, role, t, id)
}

// MiniappPersonalChatHistory — последние N сообщений приватного чата юзера с Лео.
// sinceID > 0 — только записи новее (для инкрементальной подгрузки на фронте).
// Используется HTTP-эндпоинтом /api/miniapp/personal-chat/feed.
func (b *Bot) MiniappPersonalChatHistory(userID int64, sinceID int64) ([]*domain.MiniappPersonalChatMessage, error) {
	if b == nil || b.db == nil || userID == 0 {
		return []*domain.MiniappPersonalChatMessage{}, nil
	}
	if b.config == nil || b.config.MonetizedChatID == 0 {
		return []*domain.MiniappPersonalChatMessage{}, nil
	}
	items, err := b.db.ListMiniappPersonalChat(userID, b.config.MonetizedChatID, 200, sinceID)
	if err != nil {
		return nil, err
	}
	if err := b.db.EnrichMiniappPersonalChatLikes(userID, b.config.MonetizedChatID, userID, items); err != nil {
		b.logger.Warnf("personal chat likes enrich user=%d: %v", userID, err)
	}
	return items, nil
}

// MiniappPersonalChatLikeToggle — toggle лайка в личке с Лео.
func (b *Bot) MiniappPersonalChatLikeToggle(userID, messageID int64) error {
	if b == nil || b.db == nil || userID == 0 || messageID == 0 {
		return nil
	}
	if b.config == nil || b.config.MonetizedChatID == 0 {
		return nil
	}
	return b.db.ToggleMiniappPersonalChatLike(userID, b.config.MonetizedChatID, userID, messageID)
}

// MiniappPersonalQueueLen — сколько фрагментов ещё не забрали poll'ом (для бейджа в мини-аппе).
func (b *Bot) MiniappPersonalQueueLen(userID int64) int {
	if b == nil || userID == 0 {
		return 0
	}
	b.miniappPersonalMu.Lock()
	defer b.miniappPersonalMu.Unlock()
	if b.miniappPersonalQueue == nil {
		return 0
	}
	return len(b.miniappPersonalQueue[userID])
}

// ClearMiniappPersonalReplyQueue — сброс in-memory очереди (бейдж «Лео»); тексты уже в БД.
func (b *Bot) ClearMiniappPersonalReplyQueue(userID int64) int {
	if b == nil || userID == 0 {
		return 0
	}
	b.miniappPersonalMu.Lock()
	defer b.miniappPersonalMu.Unlock()
	if b.miniappPersonalQueue == nil {
		return 0
	}
	n := len(b.miniappPersonalQueue[userID])
	delete(b.miniappPersonalQueue, userID)
	return n
}

// PopMiniappPersonalReply — один фрагмент ответа для poll API (FIFO в памяти процесса).
func (b *Bot) PopMiniappPersonalReply(userID int64) (text string, ok bool) {
	if b == nil || userID == 0 {
		return "", false
	}
	b.miniappPersonalMu.Lock()
	defer b.miniappPersonalMu.Unlock()
	q := b.miniappPersonalQueue[userID]
	if len(q) == 0 {
		return "", false
	}
	head := q[0]
	rest := q[1:]
	if len(rest) == 0 {
		delete(b.miniappPersonalQueue, userID)
	} else {
		b.miniappPersonalQueue[userID] = rest
	}
	return head, true
}

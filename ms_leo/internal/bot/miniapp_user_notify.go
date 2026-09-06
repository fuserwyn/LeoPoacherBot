package bot

import (
	"regexp"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// reTgUserMention — Markdown-mention'ы вида [Имя](tg://user?id=123).
// В мини-аппе они выглядят сырым markdown'ом, поэтому при пуше в очередь оставляем только текст ссылки.
var reTgUserMention = regexp.MustCompile(`\[([^\]]+)\]\(tg://user\?id=\d+\)`)

// isMiniappOriginActive — true, если в данный момент идёт обработка сообщения
// из мини-аппа для этого userID (см. runMiniAppPrivateTextWorker). В этом режиме
// мы НЕ дублируем ответы Лео в TG-личку, а пушим их только в очередь мини-аппа.
// Исключение — независимые от dispatch-ера уведомления таймера (5/6/7 дни и кик),
// которые отправляются напрямую через b.api.Send / b.miniappPersonalPush в timer_leopard.go и timer.go.
func (b *Bot) isMiniappOriginActive(userID int64) bool {
	if b == nil || userID == 0 {
		return false
	}
	_, ok := b.miniappReplyOrigin.Load(userID)
	return ok
}

// markMiniappOrigin / unmarkMiniappOrigin — bracket для runMiniAppPrivateTextWorker.
// chOpt — опционально канал ответов (как в leopard_money_training_done.go); если nil,
// notifyUserText всё равно положит ответ в miniappPersonalQueue, а ChatScreen его подхватит при следующем drain.
func (b *Bot) markMiniappOrigin(userID int64, chOpt chan<- string) {
	if b == nil || userID == 0 {
		return
	}
	if chOpt != nil {
		b.miniappReplyOrigin.Store(userID, chOpt)
	} else {
		b.miniappReplyOrigin.Store(userID, struct{}{})
	}
}

func (b *Bot) unmarkMiniappOrigin(userID int64) {
	if b == nil || userID == 0 {
		return
	}
	b.miniappReplyOrigin.Delete(userID)
}

// sanitizeForMiniapp — text «как для TG-Markdown» приводит к виду, читаемому в мини-аппе.
// Удаляет Markdown-mention'ы tg://user?id=N и оборачивающие звёздочки/подчёркивания.
func sanitizeForMiniapp(s string) string {
	s = reTgUserMention.ReplaceAllString(s, "$1")
	s = strings.ReplaceAll(s, "**", "")
	return strings.TrimSpace(s)
}

// notifyUserText — единая точка отправки личных ответов Лео пользователю.
//
// Поведение:
//   - если активен mini-app origin для msg.From.ID → текст идёт ТОЛЬКО в очередь мини-аппа,
//     никаких сообщений в TG-личку (в т.ч. в msg.Chat.ID) не отправляется;
//   - иначе → b.api.Send(NewMessage(msg.Chat.ID, text)) с заданным parseMode/replyToID,
//     то есть обычная TG-логика для TG-лички / группы.
//
// Помогает выполнить требование: «не дублировать в ТГ из мини-аппа, кроме предупреждений 5/6/7 дней».
func (b *Bot) notifyUserText(msg *tgbotapi.Message, text, parseMode string, replyToID int) {
	if b == nil || msg == nil || msg.From == nil || strings.TrimSpace(text) == "" {
		return
	}
	if b.isMiniappOriginActive(msg.From.ID) {
		b.miniappPersonalPush(msg.From.ID, sanitizeForMiniapp(text))
		return
	}
	out := tgbotapi.NewMessage(msg.Chat.ID, text)
	if parseMode != "" {
		out.ParseMode = parseMode
	}
	if replyToID != 0 {
		out.ReplyToMessageID = replyToID
	}
	if _, err := b.api.Send(out); err != nil {
		b.logger.Errorf("notifyUserText send: %v", err)
	}
}

// sendChatActionIfTG — typing/upload индикатор. В mini-app-режиме бессмыслен (юзер
// смотрит в мини-апп, не в TG), поэтому пропускаем, чтобы не спамить TG-API.
func (b *Bot) sendChatActionIfTG(msg *tgbotapi.Message, action tgbotapi.Chattable) {
	if b == nil || msg == nil || msg.From == nil || action == nil {
		return
	}
	if b.isMiniappOriginActive(msg.From.ID) {
		return
	}
	b.api.Send(action)
}

// startTelegramTyping показывает «печатает…» в чате, пока Лео думает над ответом.
// В мини-аппе не шлём action в Telegram. Возвращает stop — вызвать после ответа.
func (b *Bot) startTelegramTyping(msg *tgbotapi.Message) func() {
	noop := func() {}
	if b == nil || b.api == nil || msg == nil || msg.Chat == nil {
		return noop
	}
	if msg.From != nil && b.isMiniappOriginActive(msg.From.ID) {
		return noop
	}
	chatID := msg.Chat.ID
	b.api.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))
	done := make(chan struct{})
	var once sync.Once
	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				b.api.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))
			}
		}
	}()
	return func() {
		once.Do(func() { close(done) })
	}
}

// notifyUserTextByID — версия notifyUserText без *Message для случаев, когда у вызывающего
// есть только userID (адресат для маркера mini-app origin) и chatID (куда слать в TG).
// В mini-app-флоу chatID игнорируется (мы пушим в очередь юзера, а не в его TG-чат).
// Возвращает MessageID отправленного TG-сообщения (0, если шлём в очередь мини-аппа или ошибка).
func (b *Bot) notifyUserTextByID(userID, chatID int64, text, parseMode string, replyToID int) int {
	if b == nil || userID == 0 || strings.TrimSpace(text) == "" {
		return 0
	}
	if b.isMiniappOriginActive(userID) {
		b.miniappPersonalPush(userID, sanitizeForMiniapp(text))
		return 0
	}
	if chatID == 0 {
		return 0
	}
	out := tgbotapi.NewMessage(chatID, text)
	if parseMode != "" {
		out.ParseMode = parseMode
	}
	if replyToID != 0 {
		out.ReplyToMessageID = replyToID
	}
	sent, err := b.api.Send(out)
	if err != nil {
		b.logger.Errorf("notifyUserTextByID send: %v", err)
		return 0
	}
	return sent.MessageID
}

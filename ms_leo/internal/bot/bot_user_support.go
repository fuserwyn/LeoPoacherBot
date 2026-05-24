package bot

import (
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const botSupportCallbackStart = "bot_support_start"
const botSupportCallbackCancel = "bot_support_cancel"

const botSupportReplyButtonText = "💬 Поддержка"

func (b *Bot) botSupportAvailable() bool {
	return b != nil && b.config != nil && b.config.MonetizedChatID != 0 && b.db != nil
}

func (b *Bot) paywallSupportButtonRow() []tgbotapi.InlineKeyboardButton {
	if !b.botSupportAvailable() {
		return nil
	}
	return tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("💬 Поддержка: оплата и доступ", botSupportCallbackStart),
	)
}

func (b *Bot) appendPaywallSupportRow(rows [][]tgbotapi.InlineKeyboardButton) [][]tgbotapi.InlineKeyboardButton {
	if row := b.paywallSupportButtonRow(); row != nil {
		rows = append(rows, row)
	}
	return rows
}

// privateSupportReplyKeyboard — постоянная кнопка внизу лички (для оплативших и после /start).
func (b *Bot) privateSupportReplyKeyboard() *tgbotapi.ReplyKeyboardMarkup {
	if !b.botSupportAvailable() {
		return nil
	}
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(botSupportReplyButtonText),
		),
	)
	kb.ResizeKeyboard = true
	kb.OneTimeKeyboard = false
	return &kb
}

func (b *Bot) botSupportPromptInlineKeyboard() *tgbotapi.InlineKeyboardMarkup {
	return &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❎ Выйти из поддержки", botSupportCallbackCancel),
			),
		},
	}
}

func botSupportPromptText() string {
	return `💬 Поддержка

Опиши вопрос об оплате, доступе после оплаты, возврате или ошибке в боте — одним или несколькими сообщениями.

Ответ придёт в этот же чат. Выйти: /cancel, кнопка ниже или снова «💬 Поддержка».`
}

func (b *Bot) startUserSupportSession(userID int64) {
	if !b.botSupportAvailable() || userID == 0 {
		return
	}
	b.userSupportSessionsMutex.Lock()
	defer b.userSupportSessionsMutex.Unlock()
	if b.userSupportSessions == nil {
		b.userSupportSessions = make(map[int64]struct{})
	}
	b.userSupportSessions[userID] = struct{}{}
}

func (b *Bot) clearUserSupportSession(userID int64) bool {
	if userID == 0 {
		return false
	}
	b.userSupportSessionsMutex.Lock()
	defer b.userSupportSessionsMutex.Unlock()
	if b.userSupportSessions == nil {
		return false
	}
	if _, ok := b.userSupportSessions[userID]; !ok {
		return false
	}
	delete(b.userSupportSessions, userID)
	return true
}

func (b *Bot) userInSupportSession(userID int64) bool {
	if userID == 0 {
		return false
	}
	b.userSupportSessionsMutex.Lock()
	defer b.userSupportSessionsMutex.Unlock()
	if b.userSupportSessions == nil {
		return false
	}
	_, ok := b.userSupportSessions[userID]
	return ok
}

func (b *Bot) sendUserSupportPrompt(chatID int64) {
	if !b.botSupportAvailable() {
		_, _ = b.api.Send(tgbotapi.NewMessage(chatID, "⚠️ Поддержка временно недоступна. Попробуй позже."))
		return
	}
	msg := tgbotapi.NewMessage(chatID, botSupportPromptText())
	msg.ReplyMarkup = b.botSupportPromptInlineKeyboard()
	_, _ = b.api.Send(msg)
}

func (b *Bot) handleSupportCommand(msg *tgbotapi.Message) {
	if msg == nil || msg.Chat == nil {
		return
	}
	if msg.From == nil || !msg.Chat.IsPrivate() {
		_, _ = b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "ℹ️ Поддержка доступна в личном чате с ботом."))
		return
	}
	if !b.botSupportAvailable() {
		_, _ = b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "⚠️ Поддержка временно недоступна."))
		return
	}
	b.startUserSupportSession(msg.From.ID)
	b.sendUserSupportPrompt(msg.Chat.ID)
}

func (b *Bot) handleSupportCancelCommand(msg *tgbotapi.Message) {
	if msg == nil || msg.From == nil || msg.Chat == nil {
		return
	}
	if !b.clearUserSupportSession(msg.From.ID) {
		return
	}
	m := tgbotapi.NewMessage(msg.Chat.ID, "❎ Режим поддержки выключен.")
	m.ReplyMarkup = b.privateSupportReplyKeyboard()
	_, _ = b.api.Send(m)
}

func (b *Bot) handleBotSupportCallback(callback *tgbotapi.CallbackQuery) {
	if callback == nil || callback.From == nil || callback.Message == nil {
		return
	}
	answer := tgbotapi.NewCallback(callback.ID, "")
	switch callback.Data {
	case botSupportCallbackStart:
		b.startUserSupportSession(callback.From.ID)
		b.sendUserSupportPrompt(callback.Message.Chat.ID)
		answer.Text = "Напиши вопрос в чат"
	case botSupportCallbackCancel:
		if b.clearUserSupportSession(callback.From.ID) {
			answer.Text = "Вышли из поддержки"
			m := tgbotapi.NewMessage(callback.Message.Chat.ID, "❎ Режим поддержки выключен.")
			m.ReplyMarkup = b.privateSupportReplyKeyboard()
			_, _ = b.api.Send(m)
		} else {
			answer.Text = "Режим поддержки не был активен"
		}
	default:
		return
	}
	_, _ = b.api.Request(answer)
}

func isSupportReplyButtonText(text string) bool {
	t := strings.TrimSpace(text)
	return t == botSupportReplyButtonText || strings.EqualFold(t, "Поддержка")
}

// handleUserSupportFlowMessage — личка: режим поддержки (до мини-аппа), без ответа Лео.
func (b *Bot) handleUserSupportFlowMessage(msg *tgbotapi.Message) bool {
	if msg == nil || msg.From == nil || msg.Chat == nil || !msg.Chat.IsPrivate() || !b.botSupportAvailable() {
		return false
	}

	text := strings.TrimSpace(msg.Text)
	if msg.IsCommand() {
		switch msg.Command() {
		case "cancel":
			if b.clearUserSupportSession(msg.From.ID) {
				m := tgbotapi.NewMessage(msg.Chat.ID, "❎ Режим поддержки выключен.")
				m.ReplyMarkup = b.privateSupportReplyKeyboard()
				_, _ = b.api.Send(m)
			}
			return true
		case "support":
			return false
		}
		if b.userInSupportSession(msg.From.ID) {
			_, _ = b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "⚠️ Сейчас активна поддержка. Напиши текст вопроса или /cancel."))
			return true
		}
		return false
	}

	if isSupportReplyButtonText(text) {
		b.startUserSupportSession(msg.From.ID)
		b.sendUserSupportPrompt(msg.Chat.ID)
		return true
	}

	if !b.userInSupportSession(msg.From.ID) {
		return false
	}

	if text == "" && msg.Caption != "" {
		text = strings.TrimSpace(msg.Caption)
	}
	if text == "" {
		_, _ = b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "⚠️ Отправь вопрос текстом (оплата, доступ, ошибка)."))
		return true
	}

	if err := b.MiniappSupportSendFromUser(msg.From.ID, text); err != nil {
		b.logger.Warnf("bot support send user=%d: %v", msg.From.ID, err)
		_, _ = b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Не удалось отправить. Попробуй ещё раз или /cancel."))
		return true
	}

	_, _ = b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "✅ Отправлено в поддержку. Ответ придёт сюда. Можешь дописать уточнение или /cancel."))
	return true
}

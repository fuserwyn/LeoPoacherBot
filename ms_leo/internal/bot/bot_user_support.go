package bot

import (
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const botSupportCallbackStart = "bot_support_start"
const botSupportCallbackCancel = "bot_support_cancel"

const botSupportReplyButtonText = "💬 Поддержка"
const botAdminReplyButtonText = "⚙️ Админка"

func (b *Bot) botSupportAvailable() bool {
	return b != nil && b.config != nil && b.config.MonetizedChatID != 0 && b.db != nil
}

// privateSupportReplyKeyboard — постоянная кнопка внизу лички (для оплативших и после /start).
func (b *Bot) privateSupportReplyKeyboard(userID int64) *tgbotapi.ReplyKeyboardMarkup {
	if !b.botSupportAvailable() {
		return nil
	}
	buttonText := botSupportReplyButtonText
	if userID > 0 && b.config != nil && b.config.IsAdminTelegramUser(userID) {
		buttonText = botAdminReplyButtonText
	}
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(buttonText),
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

Ответ придёт в этот же чат. Выйти — кнопка «Выйти из поддержки» ниже.`
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
			m.ReplyMarkup = b.privateSupportReplyKeyboard(callback.From.ID)
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

func isAdminReplyButtonText(text string) bool {
	t := strings.TrimSpace(text)
	return t == botAdminReplyButtonText || strings.EqualFold(t, "Админка")
}

// handleUserSupportFlowMessage — личка: режим поддержки (до мини-аппа), без ответа Лео.
func (b *Bot) handleUserSupportFlowMessage(msg *tgbotapi.Message) bool {
	if msg == nil || msg.From == nil || msg.Chat == nil || !msg.Chat.IsPrivate() || !b.botSupportAvailable() {
		return false
	}

	text := strings.TrimSpace(msg.Text)
	if msg.IsCommand() {
		if b.userInSupportSession(msg.From.ID) {
			_, _ = b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "⚠️ Сейчас активна поддержка. Напиши текст вопроса или нажми «Выйти из поддержки»."))
			return true
		}
		return false
	}

	if isSupportReplyButtonText(text) {
		b.startUserSupportSession(msg.From.ID)
		b.sendUserSupportPrompt(msg.Chat.ID)
		return true
	}
	if b.config != nil && b.config.IsAdminTelegramUser(msg.From.ID) && isAdminReplyButtonText(text) {
		b.showAdminMenu(msg.Chat.ID)
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
		_, _ = b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Не удалось отправить. Попробуй ещё раз."))
		return true
	}

	_, _ = b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "✅ Отправлено в поддержку. Ответ придёт сюда. Можешь дописать уточнение."))
	return true
}

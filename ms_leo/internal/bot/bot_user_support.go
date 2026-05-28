package bot

import (
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const botSupportCallbackStart = "bot_support_start"
const botSupportCallbackCancel = "bot_support_cancel"

const botSupportReplyButtonText = "💬 Поддержка"
const botAdminReplyButtonText = "⚙️ Админ-панель"

func (b *Bot) botSupportAvailable() bool {
	return b != nil && b.config != nil && b.config.MonetizedChatID != 0 && b.db != nil
}

func (b *Bot) isAdminTelegramUser(userID int64) bool {
	if b == nil || b.config == nil {
		return false
	}
	if b.config.IsAdminTelegramUser(userID) {
		return true
	}
	b.dynamicAdminsMu.RLock()
	_, ok := b.dynamicAdmins[userID]
	b.dynamicAdminsMu.RUnlock()
	return ok
}

// isOwnerOnly — true только для владельца бота (OWNER_ID).
func (b *Bot) isOwnerOnly(userID int64) bool {
	return b != nil && b.config != nil && b.config.OwnerID != 0 && userID == b.config.OwnerID
}

// reloadDynamicAdmins — перезагружает кэш динамических администраторов из БД.
func (b *Bot) reloadDynamicAdmins() {
	if b == nil || b.db == nil {
		return
	}
	ids, err := b.db.LoadDynamicAdminIDs()
	if err != nil {
		if b.logger != nil {
			b.logger.Warnf("reloadDynamicAdmins: %v", err)
		}
		return
	}
	m := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		m[id] = struct{}{}
	}
	b.dynamicAdminsMu.Lock()
	b.dynamicAdmins = m
	b.dynamicAdminsMu.Unlock()
}

// privateBottomReplyKeyboard — постоянные кнопки внизу лички с ботом.
func (b *Bot) privateBottomReplyKeyboard(userID int64) *tgbotapi.ReplyKeyboardMarkup {
	if userID <= 0 {
		return nil
	}
	if !b.isAdminTelegramUser(userID) && !b.botSupportAvailable() {
		return nil
	}
	var rows [][]tgbotapi.KeyboardButton
	if b.isAdminTelegramUser(userID) {
		rows = append(rows, tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(botAdminReplyButtonText),
		))
	}
	if b.botSupportAvailable() {
		rows = append(rows, tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(botSupportReplyButtonText),
		))
	}
	if len(rows) == 0 {
		return nil
	}
	kb := tgbotapi.ReplyKeyboardMarkup{
		Keyboard:        rows,
		ResizeKeyboard:  true,
		OneTimeKeyboard: false,
	}
	return &kb
}

// syncPrivateBottomKeyboard обновляет reply-клавиатуру внизу (без лишних сообщений при повторе).
func (b *Bot) syncPrivateBottomKeyboard(chatID, userID int64) {
	if chatID == 0 || userID == 0 {
		return
	}
	kb := b.privateBottomReplyKeyboard(userID)
	if kb == nil {
		return
	}
	kind := "support"
	switch {
	case b.isAdminTelegramUser(userID) && b.botSupportAvailable():
		kind = "admin+support"
	case b.isAdminTelegramUser(userID):
		kind = "admin"
	}
	if prev, ok := b.privateBottomKeyboardKind.Load(userID); ok && prev == kind {
		return
	}
	b.privateBottomKeyboardKind.Store(userID, kind)
	msg := tgbotapi.NewMessage(chatID, " ")
	msg.ReplyMarkup = kb
	_, _ = b.api.Send(msg)
}

// openAdminPanelForUser — админ-панель + гарантированная кнопка «Админка» внизу.
func (b *Bot) openAdminPanelForUser(chatID, userID int64) {
	b.syncPrivateBottomKeyboard(chatID, userID)
	b.showAdminMenuForUser(chatID)
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
			b.syncPrivateBottomKeyboard(callback.Message.Chat.ID, callback.From.ID)
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
	switch t {
	case botAdminReplyButtonText, "⚙️ Админка", "Админ-панель", "Админка":
		return true
	default:
		return false
	}
}

// handleUserSupportFlowMessage — личка: режим поддержки (до мини-аппа), без ответа Лео.
func (b *Bot) handleUserSupportFlowMessage(msg *tgbotapi.Message) bool {
	if msg == nil || msg.From == nil || msg.Chat == nil || !msg.Chat.IsPrivate() {
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

	if b.isAdminTelegramUser(msg.From.ID) && isAdminReplyButtonText(text) {
		b.openAdminPanelForUser(msg.Chat.ID, msg.From.ID)
		return true
	}

	if isSupportReplyButtonText(text) {
		if !b.botSupportAvailable() {
			_, _ = b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "⚠️ Поддержка временно недоступна. Попробуй позже."))
			return true
		}
		b.startUserSupportSession(msg.From.ID)
		b.sendUserSupportPrompt(msg.Chat.ID)
		return true
	}

	if !b.botSupportAvailable() || !b.userInSupportSession(msg.From.ID) {
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

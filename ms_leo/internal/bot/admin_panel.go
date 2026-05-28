package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type adminSession struct {
	Mode         string // feed_text | poll | support | user_mgmt | user_add_cups | user_add_streak | user_delete_msg
	Step         string // await_text | await_support_text | await_poll_question | await_poll_options | await_user_id | await_amount | await_message_id
	TargetUserID int64
	PollQuestion string
}

func (b *Bot) isOwnerPrivateChat(msg *tgbotapi.Message) bool {
	return msg != nil && msg.From != nil && msg.Chat != nil && b.config.IsAdminTelegramUser(msg.From.ID) && msg.Chat.IsPrivate()
}

func (b *Bot) handleAdmin(msg *tgbotapi.Message) {
	if !b.isOwnerPrivateChat(msg) {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Админ-панель доступна только админам в личном чате с ботом.")
		b.api.Send(reply)
		return
	}
	b.openAdminPanelForUser(msg.Chat.ID, msg.From.ID)
}

func (b *Bot) showAdminMenu(chatID int64) {
	b.showAdminMenuForUser(chatID, 0)
}

func (b *Bot) showAdminMenuForUser(chatID, userID int64) {
	text := "⚙️ Админ-панель\n\nВыбери действие:"
	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💬 Поддержка", "admin_support_inbox"),
			tgbotapi.NewInlineKeyboardButtonData("📋 Список юзеров", "admin_users_list_0"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💳 Оплаты", "admin_payments_0"),
			tgbotapi.NewInlineKeyboardButtonData("🔍 Найти юзера", "admin_users"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📝 Текст", "admin_mode_feed_text"),
			tgbotapi.NewInlineKeyboardButtonData("🗳 Опрос", "admin_mode_poll"),
		),
	}
	if userID != 0 && b.isOwnerOnly(userID) {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👑 Панель владельца", "owner_menu"),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("❎ Отмена", "admin_cancel"),
	))
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
	b.api.Send(msg)
}

func (b *Bot) handleAdminCallbackQuery(callback *tgbotapi.CallbackQuery) {
	if callback == nil || callback.Message == nil || callback.From == nil {
		return
	}
	if !b.config.IsAdminTelegramUser(callback.From.ID) || !callback.Message.Chat.IsPrivate() {
		callbackConfig := tgbotapi.NewCallback(callback.ID, "Недостаточно прав")
		b.api.Request(callbackConfig)
		return
	}

	if strings.HasPrefix(callback.Data, "admin_support_user_") {
		userID, err := strconv.ParseInt(strings.TrimPrefix(callback.Data, "admin_support_user_"), 10, 64)
		if err == nil && userID > 0 {
			b.showAdminSupportThread(callback.Message.Chat.ID, userID)
		}
		callbackConfig := tgbotapi.NewCallback(callback.ID, "")
		b.api.Request(callbackConfig)
		return
	}
	if strings.HasPrefix(callback.Data, "admin_support_reply_") {
		userID, err := strconv.ParseInt(strings.TrimPrefix(callback.Data, "admin_support_reply_"), 10, 64)
		if err == nil && userID > 0 {
			b.startAdminSupportReply(callback.From.ID, userID)
			b.api.Send(tgbotapi.NewMessage(callback.Message.Chat.ID, "✍️ Напиши ответ пользователю. Он уйдёт в поддержку внутри мини-аппа."))
		}
		callbackConfig := tgbotapi.NewCallback(callback.ID, "")
		b.api.Request(callbackConfig)
		return
	}
	if strings.HasPrefix(callback.Data, "admin_feed_report_dismiss_") {
		reportID, err := strconv.ParseInt(strings.TrimPrefix(callback.Data, "admin_feed_report_dismiss_"), 10, 64)
		if err == nil && reportID > 0 {
			b.dismissAdminFeedReport(callback.Message.Chat.ID, reportID)
		}
		callbackConfig := tgbotapi.NewCallback(callback.ID, "")
		b.api.Request(callbackConfig)
		return
	}
	if strings.HasPrefix(callback.Data, "owner_") {
		b.handleOwnerCallbackQuery(callback)
		return
	}
	if strings.HasPrefix(callback.Data, "admin_users_list_") ||
		strings.HasPrefix(callback.Data, "admin_payments_") {
		if b.handleAdminDirectoryCallback(callback) {
			callbackConfig := tgbotapi.NewCallback(callback.ID, "")
			b.api.Request(callbackConfig)
			return
		}
	}
	if strings.HasPrefix(callback.Data, "admin_feed_report_del_") ||
		strings.HasPrefix(callback.Data, "admin_feed_report_hide_") ||
		strings.HasPrefix(callback.Data, "admin_feed_report_mute_") ||
		strings.HasPrefix(callback.Data, "admin_feed_report_unmute_") ||
		strings.HasPrefix(callback.Data, "admin_user_") ||
		callback.Data == "admin_users" {
		if b.handleAdminUserMgmtCallback(callback) {
			callbackConfig := tgbotapi.NewCallback(callback.ID, "")
			b.api.Request(callbackConfig)
			return
		}
	}
	if strings.HasPrefix(callback.Data, "admin_feed_report_") {
		suffix := strings.TrimPrefix(callback.Data, "admin_feed_report_")
		reportID, err := strconv.ParseInt(suffix, 10, 64)
		if err == nil && reportID > 0 {
			b.showAdminFeedReport(callback.Message.Chat.ID, reportID)
			callbackConfig := tgbotapi.NewCallback(callback.ID, "")
			b.api.Request(callbackConfig)
			return
		}
	}

	switch callback.Data {
	case "admin_open":
		b.openAdminPanelForUser(callback.Message.Chat.ID, callback.From.ID)
	case "admin_support_inbox":
		b.showAdminSupportInbox(callback.Message.Chat.ID)
	case "admin_support_back":
		b.showAdminSupportInbox(callback.Message.Chat.ID)
	case "admin_feed_reports":
		b.showAdminFeedReportsInbox(callback.Message.Chat.ID)
	case "admin_feed_reports_back":
		b.showAdminFeedReportsInbox(callback.Message.Chat.ID)
	case "admin_mode_feed_text":
		b.startAdminFlow(callback.From.ID, "feed_text")
		b.api.Send(tgbotapi.NewMessage(callback.Message.Chat.ID, "📝 Напиши кастомный текст для ленты стаи. Он появится как отдельный админский пост."))
	case "admin_mode_poll":
		b.startAdminFlow(callback.From.ID, "poll")
		b.api.Send(tgbotapi.NewMessage(callback.Message.Chat.ID, "🗳 Напиши вопрос для опроса в ленте miniapp."))
	case "admin_cancel":
		b.clearAdminFlow(callback.From.ID)
		b.api.Send(tgbotapi.NewMessage(callback.Message.Chat.ID, "❎ Действие отменено."))
	}

	callbackConfig := tgbotapi.NewCallback(callback.ID, "")
	b.api.Request(callbackConfig)
}

func (b *Bot) startAdminFlow(userID int64, mode string) {
	b.adminSessionsMutex.Lock()
	defer b.adminSessionsMutex.Unlock()
	step := "await_text"
	if mode == "poll" {
		step = "await_poll_question"
	} else if mode == "feed_text" {
		step = "await_text"
	}
	b.adminSessions[userID] = &adminSession{
		Mode: mode,
		Step: step,
	}
}

func (b *Bot) startAdminSupportReply(userID, targetUserID int64) {
	b.adminSessionsMutex.Lock()
	defer b.adminSessionsMutex.Unlock()
	b.adminSessions[userID] = &adminSession{
		Mode:         "support",
		Step:         "await_support_text",
		TargetUserID: targetUserID,
	}
}

func (b *Bot) clearAdminFlow(userID int64) {
	b.adminSessionsMutex.Lock()
	defer b.adminSessionsMutex.Unlock()
	delete(b.adminSessions, userID)
}

func (b *Bot) getAdminSession(userID int64) (*adminSession, bool) {
	b.adminSessionsMutex.Lock()
	defer b.adminSessionsMutex.Unlock()
	s, ok := b.adminSessions[userID]
	return s, ok
}

func (b *Bot) handleAdminFlowMessage(msg *tgbotapi.Message) bool {
	if !b.isOwnerPrivateChat(msg) {
		return false
	}

	session, ok := b.getAdminSession(msg.From.ID)
	if !ok || session == nil {
		return false
	}

	// Управление сессией командами
	if msg.IsCommand() {
		switch msg.Command() {
		case "cancel":
			b.clearAdminFlow(msg.From.ID)
			b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "❎ Админ-действие отменено."))
		case "admin":
			b.openAdminPanelForUser(msg.Chat.ID, msg.From.ID)
		default:
			b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "⚠️ Сначала заверши текущий мастер или отправь /cancel"))
		}
		return true
	}

	// Панель владельца: добавление динамического администратора
	if session.Mode == "owner_add_admin" {
		return b.handleOwnerAdminAddMessage(msg)
	}

	if b.handleAdminUserMgmtMessage(msg, session) {
		return true
	}

	switch session.Step {
	case "await_support_text":
		reply := strings.TrimSpace(msg.Text)
		if reply == "" {
			b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "⚠️ Ответ пустой. Отправь текст или /cancel."))
			return true
		}
		if err := b.AdminSupportReply(session.TargetUserID, reply); err != nil {
			b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Не удалось отправить ответ: "+err.Error()))
			return true
		}
		targetUserID := session.TargetUserID
		b.clearAdminFlow(msg.From.ID)
		b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "✅ Ответ отправлен пользователю."))
		b.showAdminSupportThread(msg.Chat.ID, targetUserID)
		return true

	case "await_text":
		text := strings.TrimSpace(msg.Text)
		if text == "" {
			b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "⚠️ Текст пустой. Отправь текст поста или /cancel."))
			return true
		}
		if session.Mode != "feed_text" {
			b.clearAdminFlow(msg.From.ID)
			b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Неизвестный текстовый режим. Начни заново: /admin"))
			return true
		}
		if err := b.saveAdminCustomPackFeed(msg.From.ID, text); err != nil {
			b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Не удалось опубликовать пост: "+err.Error()))
			return true
		}
		b.clearAdminFlow(msg.From.ID)
		b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "✅ Кастомный пост опубликован в ленте стаи."))
		return true

	case "await_poll_question":
		question := strings.TrimSpace(msg.Text)
		if question == "" {
			b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "⚠️ Вопрос пустой, отправь текст вопроса."))
			return true
		}
		if len([]rune(question)) > 300 {
			b.api.Send(tgbotapi.NewMessage(
				msg.Chat.ID,
				"⚠️ Вопрос слишком длинный для Telegram-опроса (макс. 300 символов). Сократи текст и отправь снова.",
			))
			return true
		}
		session.PollQuestion = question
		session.Step = "await_poll_options"
		b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "🗳 Отправь варианты через `|`, например:\nДа | Нет | Нужно доработать"))
		return true

	case "await_poll_options":
		raw := strings.Split(msg.Text, "|")
		options := make([]string, 0, len(raw))
		for _, opt := range raw {
			v := strings.TrimSpace(opt)
			if v != "" {
				options = append(options, v)
			}
		}
		if len(options) < 2 {
			b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "⚠️ Нужно минимум 2 варианта. Пример: Да | Нет"))
			return true
		}
		if len(options) > 10 {
			b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "⚠️ Максимум 10 вариантов."))
			return true
		}
		for _, opt := range options {
			if len([]rune(opt)) > 100 {
				b.api.Send(tgbotapi.NewMessage(
					msg.Chat.ID,
					"⚠️ Один из вариантов слишком длинный (макс. 100 символов). Сократи варианты и отправь снова.",
				))
				return true
			}
		}
		if err := b.saveAdminPollPackFeed(msg.From.ID, session.PollQuestion, options); err != nil {
			b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Не удалось опубликовать опрос: "+err.Error()))
			return true
		}
		b.clearAdminFlow(msg.From.ID)
		b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "✅ Опрос опубликован в ленте miniapp."))
		return true
	}

	return false
}

func (b *Bot) showAdminSupportInbox(chatID int64) {
	if b == nil || b.db == nil || b.config == nil || b.config.MonetizedChatID == 0 {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Поддержка недоступна: не настроен pack chat."))
		return
	}
	items, err := b.db.ListMiniappSupportConversations(b.config.MonetizedChatID, 20)
	if err != nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Не удалось загрузить диалоги поддержки."))
		return
	}

	var text strings.Builder
	text.WriteString("💬 Поддержка\n\n")
	openReports := 0
	if n, err := b.db.CountOpenMiniappFeedReports(b.config.MonetizedChatID); err == nil {
		openReports = n
	}
	if len(items) == 0 {
		text.WriteString("Диалогов пока нет.")
		rows := make([][]tgbotapi.InlineKeyboardButton, 0, 2)
		if openReports > 0 {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					fmt.Sprintf("🚨 Жалобы на ленту (%d)", openReports),
					"admin_feed_reports",
				),
			))
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ К админке", "admin_open"),
		))
		msg := tgbotapi.NewMessage(chatID, text.String())
		msg.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
		b.api.Send(msg)
		return
	}

	text.WriteString("Последние диалоги:\n\n")
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(items)+2)
	if openReports > 0 {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("🚨 Жалобы на ленту (%d)", openReports),
				"admin_feed_reports",
			),
		))
	}
	for i, item := range items {
		prefix := "•"
		if item.NeedsReply {
			prefix = "●"
		}
		text.WriteString(fmt.Sprintf("%d. %s %s\n", i+1, prefix, adminSupportTitle(item.DisplayName, item.UserID)))
		text.WriteString(fmt.Sprintf("   %s: %s\n\n", adminSupportRoleLabel(item.LastRole), clipAdminSupportText(item.LastText, 80)))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				adminSupportButtonLabel(item.DisplayName, item.UserID, item.NeedsReply),
				"admin_support_user_"+strconv.FormatInt(item.UserID, 10),
			),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⬅️ К админке", "admin_open"),
	))
	msg := tgbotapi.NewMessage(chatID, clipAdminSupportText(text.String(), 3500))
	msg.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
	b.api.Send(msg)
}

func (b *Bot) showAdminSupportThread(chatID, targetUserID int64) {
	if b == nil || b.db == nil || b.config == nil || b.config.MonetizedChatID == 0 || targetUserID == 0 {
		return
	}
	items, err := b.AdminSupportChatHistory(targetUserID)
	if err != nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Не удалось загрузить диалог."))
		return
	}
	var text strings.Builder
	text.WriteString("💬 Диалог поддержки\n")
	text.WriteString(adminSupportTitle("", targetUserID))
	text.WriteString("\n\n")
	if len(items) == 0 {
		text.WriteString("Сообщений пока нет.")
	} else {
		for _, item := range items {
			if item == nil {
				continue
			}
			text.WriteString(adminSupportRoleLabel(item.Role))
			text.WriteString(": ")
			text.WriteString(clipAdminSupportText(item.Text, 240))
			text.WriteString("\n\n")
		}
	}
	msg := tgbotapi.NewMessage(chatID, clipAdminSupportText(text.String(), 3500))
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✍️ Ответить", "admin_support_reply_"+strconv.FormatInt(targetUserID, 10)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ К диалогам", "admin_support_back"),
		),
	)
	b.api.Send(msg)
}

func adminSupportRoleLabel(role string) string {
	if role == "user" {
		return "Юзер"
	}
	return "Поддержка"
}

func adminSupportTitle(displayName string, userID int64) string {
	name := strings.TrimSpace(displayName)
	if name == "" {
		name = "user" + strconv.FormatInt(userID, 10)
	}
	return name + " · " + strconv.FormatInt(userID, 10)
}

func adminSupportButtonLabel(displayName string, userID int64, needsReply bool) string {
	prefix := "💬 "
	if needsReply {
		prefix = "🆕 "
	}
	return clipAdminSupportText(prefix+adminSupportTitle(displayName, userID), 56)
}

func (b *Bot) showAdminFeedReportsInbox(chatID int64) {
	if b == nil || b.db == nil || b.config == nil || b.config.MonetizedChatID == 0 {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Жалобы недоступны: не настроен pack chat."))
		return
	}
	items, err := b.db.ListOpenMiniappFeedReports(b.config.MonetizedChatID, 20)
	if err != nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Не удалось загрузить жалобы."))
		return
	}
	var text strings.Builder
	text.WriteString("🚨 Жалобы на контент\n\n")
	if len(items) == 0 {
		text.WriteString("Открытых жалоб нет.")
	} else {
		text.WriteString("Открытые жалобы:\n\n")
		for i, item := range items {
			if item == nil {
				continue
			}
			text.WriteString(fmt.Sprintf("%d. %s · %s\n", i+1, feedReportTargetLabel(item), feedReportPersonLabel(item.TargetName, item.TargetUserID)))
			text.WriteString(fmt.Sprintf("   От %s\n", feedReportPersonLabel(item.ReporterName, item.ReporterUserID)))
			text.WriteString(fmt.Sprintf("   «%s»\n\n", clipAdminSupportText(item.TargetText, 72)))
		}
	}
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(items)+1)
	for _, item := range items {
		if item == nil {
			continue
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("#%d · %s", item.ID, clipAdminSupportText(feedReportTargetLabel(item), 12)),
				"admin_feed_report_"+strconv.FormatInt(item.ID, 10),
			),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⬅️ К поддержке", "admin_support_back"),
	))
	msg := tgbotapi.NewMessage(chatID, clipAdminSupportText(text.String(), 3500))
	msg.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
	b.api.Send(msg)
}

func (b *Bot) showAdminFeedReport(chatID, reportID int64) {
	if b == nil || b.db == nil || b.config == nil || b.config.MonetizedChatID == 0 || reportID == 0 {
		return
	}
	item, err := b.db.GetMiniappFeedReport(b.config.MonetizedChatID, reportID)
	if err != nil || item == nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Жалоба не найдена."))
		return
	}
	var text strings.Builder
	text.WriteString("🚨 Жалоба #")
	text.WriteString(strconv.FormatInt(item.ID, 10))
	text.WriteString("\n\n")
	text.WriteString("Тип: ")
	text.WriteString(feedReportTargetLabel(item))
	text.WriteString("\n")
	text.WriteString("Кто пожаловался: ")
	text.WriteString(feedReportPersonLabel(item.ReporterName, item.ReporterUserID))
	text.WriteString("\n")
	text.WriteString("На кого: ")
	text.WriteString(feedReportPersonLabel(item.TargetName, item.TargetUserID))
	text.WriteString("\n")
	if item.TargetType == "pack_group_message" {
		text.WriteString("Сообщение в чате: #")
		text.WriteString(strconv.FormatInt(item.UserMessageID, 10))
		text.WriteString("\n")
	} else {
		text.WriteString("Отчёт в ленте: #")
		text.WriteString(strconv.FormatInt(item.UserMessageID, 10))
		text.WriteString("\n")
	}
	if item.ThreadReplyID > 0 {
		text.WriteString("Комментарий: #")
		text.WriteString(strconv.FormatInt(item.ThreadReplyID, 10))
		text.WriteString("\n")
	}
	text.WriteString("\nТекст:\n«")
	text.WriteString(clipAdminSupportText(item.TargetText, 900))
	text.WriteString("»")
	reportTargetMuted := false
	if item.TargetUserID > 0 {
		if ugc, err := b.db.GetUGCModerationState(item.TargetUserID, b.config.MonetizedChatID); err == nil {
			text.WriteString(fmt.Sprintf("\n\nUGC-нарушения автора: %d", ugc.ViolationCount))
			if ugc.MutedUntil != nil && ugc.MutedUntil.After(time.Now()) {
				reportTargetMuted = true
				text.WriteString(fmt.Sprintf("\nUGC-мьют до: %s", ugc.MutedUntil.UTC().Format("2006-01-02 15:04 UTC")))
			}
		}
	}
	msg := tgbotapi.NewMessage(chatID, clipAdminSupportText(text.String(), 3500))
	if item.Status == "open" {
		muteBtn := tgbotapi.NewInlineKeyboardButtonData("🔇 Mute 24ч", "admin_feed_report_mute_"+strconv.FormatInt(item.ID, 10))
		if reportTargetMuted {
			muteBtn = tgbotapi.NewInlineKeyboardButtonData("🔊 Unmute", "admin_feed_report_unmute_"+strconv.FormatInt(item.ID, 10))
		}
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🙈 Скрыть", "admin_feed_report_hide_"+strconv.FormatInt(item.ID, 10)),
				muteBtn,
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🗑 Удалить", "admin_feed_report_del_"+strconv.FormatInt(item.ID, 10)),
				tgbotapi.NewInlineKeyboardButtonData("✅ Обработано", "admin_feed_report_dismiss_"+strconv.FormatInt(item.ID, 10)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("⬅️ К жалобам", "admin_feed_reports_back"),
			),
		)
	} else {
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("⬅️ К жалобам", "admin_feed_reports_back"),
			),
		)
	}
	b.api.Send(msg)
}

func (b *Bot) dismissAdminFeedReport(chatID, reportID int64) {
	if b == nil || b.db == nil || b.config == nil || b.config.MonetizedChatID == 0 {
		return
	}
	ok, err := b.db.DismissMiniappFeedReport(b.config.MonetizedChatID, reportID)
	if err != nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Не удалось закрыть жалобу."))
		return
	}
	if !ok {
		b.api.Send(tgbotapi.NewMessage(chatID, "Жалоба уже обработана или не найдена."))
		return
	}
	b.api.Send(tgbotapi.NewMessage(chatID, "✅ Жалоба отмечена обработанной."))
	b.showAdminFeedReportsInbox(chatID)
}

func clipAdminSupportText(s string, limit int) string {
	s = strings.TrimSpace(s)
	if limit <= 0 || len([]rune(s)) <= limit {
		return s
	}
	r := []rune(s)
	if limit <= 1 {
		return string(r[:limit])
	}
	return string(r[:limit-1]) + "…"
}


package bot

import (
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func hiddenModerationKindLabel(kind string) string {
	switch kind {
	case "feed_post":
		return "Пост"
	case "thread_reply":
		return "Комментарий"
	case "pack_group_message":
		return "Сообщение в чате"
	default:
		return "Контент"
	}
}

func hiddenModerationRestoreCallback(kind string, id int64) string {
	switch kind {
	case "feed_post":
		return "admin_hidden_restore_post_" + strconv.FormatInt(id, 10)
	case "thread_reply":
		return "admin_hidden_restore_thread_" + strconv.FormatInt(id, 10)
	case "pack_group_message":
		return "admin_hidden_restore_chat_" + strconv.FormatInt(id, 10)
	default:
		return ""
	}
}

func (b *Bot) showAdminHiddenContentInbox(chatID int64) {
	if b == nil || b.db == nil || b.config == nil || b.config.MonetizedChatID == 0 {
		return
	}
	packChatID := b.config.MonetizedChatID
	items, err := b.db.ListHiddenModerationItems(packChatID, 20)
	if err != nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Не удалось загрузить скрытое."))
		return
	}
	var text strings.Builder
	text.WriteString("🙈 Скрытый контент\n\n")
	text.WriteString("Можно вернуть в ленту или чат. Жёсткое удаление — только через «Очистить ленту и чат» или карточку юзера.\n\n")
	if len(items) == 0 {
		text.WriteString("Скрытого контента нет.")
	} else {
		text.WriteString("Последние скрытые:\n\n")
		for i, item := range items {
			author := adminSupportTitle(item.AuthorName, item.AuthorUserID)
			text.WriteString(fmt.Sprintf(
				"%d. %s · %s · %s\n",
				i+1,
				hiddenModerationKindLabel(item.Kind),
				author,
				clipAdminSupportText(item.Text, 72),
			))
			if item.Kind == "thread_reply" && item.ParentID > 0 {
				text.WriteString(fmt.Sprintf("   Отчёт #%d\n", item.ParentID))
			}
			text.WriteString("\n")
		}
	}
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(items)+1)
	for _, item := range items {
		cb := hiddenModerationRestoreCallback(item.Kind, item.ID)
		if cb == "" {
			continue
		}
		label := fmt.Sprintf(
			"↩️ #%d · %s",
			item.ID,
			clipAdminSupportText(hiddenModerationKindLabel(item.Kind), 10),
		)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, cb),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⬅️ К поддержке", "admin_support_back"),
	))
	msg := tgbotapi.NewMessage(chatID, clipAdminSupportText(text.String(), 3500))
	msg.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
	b.api.Send(msg)
}

func (b *Bot) adminRestoreHiddenContent(chatID int64, kind string, id int64) {
	if b == nil || b.db == nil || b.config == nil || id == 0 {
		return
	}
	packChatID := b.config.MonetizedChatID
	var ok bool
	var err error
	var label string
	switch kind {
	case "feed_post":
		ok, err = b.db.AdminUnhideFeedUserMessage(packChatID, id)
		label = fmt.Sprintf("пост #%d", id)
	case "thread_reply":
		ok, err = b.db.AdminUnhideTrainingFeedThreadReply(packChatID, id)
		label = fmt.Sprintf("комментарий t%d", id)
	case "pack_group_message":
		ok, err = b.db.AdminUnhidePackGroupMessage(packChatID, id)
		label = fmt.Sprintf("сообщение чата #%d", id)
	default:
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Неизвестный тип контента."))
		return
	}
	if err != nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка: "+err.Error()))
		return
	}
	if !ok {
		b.api.Send(tgbotapi.NewMessage(chatID, "ℹ️ Уже возвращено или не найдено."))
	} else {
		b.api.Send(tgbotapi.NewMessage(chatID, "✅ Возвращено: "+label+"."))
	}
	b.showAdminHiddenContentInbox(chatID)
}

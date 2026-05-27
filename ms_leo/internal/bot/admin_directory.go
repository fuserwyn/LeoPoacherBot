package bot

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"leo-bot/internal/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	adminUsersListPageSize    = 8
	adminPaymentsListPageSize = 6
)

func (b *Bot) showAdminUsersListPage(chatID int64, offset int) {
	packChatID := b.adminPackChatID()
	if packChatID == 0 {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Не настроен MonetizedChatID."))
		return
	}
	if offset < 0 {
		offset = 0
	}
	total, err := b.db.CountPackUsersForAdmin(packChatID)
	if err != nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Не удалось загрузить список."))
		return
	}
	rows, err := b.db.ListPackUsersForAdmin(packChatID, offset, adminUsersListPageSize)
	if err != nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка списка: "+err.Error()))
		return
	}

	var text strings.Builder
	text.WriteString("📋 Пользователи стаи\n")
	if total == 0 {
		text.WriteString("\nПока никого нет.")
	} else {
		from := offset + 1
		to := offset + len(rows)
		text.WriteString(fmt.Sprintf("\n%d–%d из %d\n\n", from, to, total))
		for i, u := range rows {
			text.WriteString(fmt.Sprintf("%d. %s\n", offset+i+1, adminPackUserListLine(u)))
		}
	}

	keyboardRows := make([][]tgbotapi.InlineKeyboardButton, 0, len(rows)+2)
	for _, u := range rows {
		keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				adminPackUserListButtonLabel(u),
				"admin_user_open_"+strconv.FormatInt(u.UserID, 10),
			),
		))
	}
	nav := make([]tgbotapi.InlineKeyboardButton, 0, 2)
	if offset > 0 {
		prev := offset - adminUsersListPageSize
		if prev < 0 {
			prev = 0
		}
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "admin_users_list_"+strconv.Itoa(prev)))
	}
	if offset+len(rows) < total {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("▶️ Далее", "admin_users_list_"+strconv.Itoa(offset+adminUsersListPageSize)))
	}
	if len(nav) > 0 {
		keyboardRows = append(keyboardRows, nav)
	}
	keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔍 Найти", "admin_users"),
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Админка", "admin_open"),
	))

	msg := tgbotapi.NewMessage(chatID, clipAdminSupportText(text.String(), 3500))
	msg.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboardRows}
	b.api.Send(msg)
}

func (b *Bot) showAdminPaymentsPage(chatID int64, offset int) {
	packChatID := b.adminPackChatID()
	if packChatID == 0 {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Не настроен MonetizedChatID."))
		return
	}
	if offset < 0 {
		offset = 0
	}
	total, err := b.db.CountPaywallPaymentsForAdmin(packChatID)
	if err != nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Не удалось загрузить оплаты."))
		return
	}
	payments, err := b.db.ListPaywallPaymentsForAdmin(packChatID, offset, adminPaymentsListPageSize)
	if err != nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка: "+err.Error()))
		return
	}

	var text strings.Builder
	text.WriteString("💳 Оплаты\n")
	if total == 0 {
		text.WriteString("\nЗаявок пока нет.")
	} else {
		from := offset + 1
		to := offset + len(payments)
		text.WriteString(fmt.Sprintf("\n%d–%d из %d\n\n", from, to, total))
		for _, p := range payments {
			text.WriteString(adminPaywallPaymentLine(p))
			text.WriteString("\n")
		}
	}

	keyboardRows := make([][]tgbotapi.InlineKeyboardButton, 0, len(payments)+2)
	for _, p := range payments {
		keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("#%d · %s", p.ID, clipAdminSupportText(adminPaywallPersonLabel(p.Username, p.DisplayName, p.UserID), 24)),
				"admin_user_open_"+strconv.FormatInt(p.UserID, 10),
			),
		))
	}
	nav := make([]tgbotapi.InlineKeyboardButton, 0, 2)
	if offset > 0 {
		prev := offset - adminPaymentsListPageSize
		if prev < 0 {
			prev = 0
		}
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "admin_payments_"+strconv.Itoa(prev)))
	}
	if offset+len(payments) < total {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("▶️ Далее", "admin_payments_"+strconv.Itoa(offset+adminPaymentsListPageSize)))
	}
	if len(nav) > 0 {
		keyboardRows = append(keyboardRows, nav)
	}
	keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📋 Юзеры", "admin_users_list_0"),
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Админка", "admin_open"),
	))

	msg := tgbotapi.NewMessage(chatID, clipAdminSupportText(text.String(), 3500))
	msg.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboardRows}
	b.api.Send(msg)
}

func adminPackUserListLine(u database.AdminPackUserListRow) string {
	pay := "💳—"
	if u.HasActivePaywall {
		pay = "💳✅"
	}
	del := ""
	if u.IsDeleted {
		del = " · 🚫удалён"
	}
	return fmt.Sprintf("%s · 🏆%d · ⚡%d · %s%s",
		adminPaywallPersonLabel(u.Username, u.DisplayName, u.UserID),
		u.Cups, u.StreakDays, pay, del,
	)
}

func adminPackUserListButtonLabel(u database.AdminPackUserListRow) string {
	return clipAdminSupportText(adminPackUserListLine(u), 56)
}

func adminPaywallPersonLabel(username, displayName string, userID int64) string {
	parts := make([]string, 0, 2)
	if n := strings.TrimSpace(displayName); n != "" {
		parts = append(parts, n)
	}
	if u := strings.TrimSpace(username); u != "" {
		parts = append(parts, u)
	}
	if len(parts) == 0 {
		return strconv.FormatInt(userID, 10)
	}
	return strings.Join(parts, " · ")
}

func adminPaywallPaymentLine(p database.AdminPaywallPaymentRow) string {
	when := p.CreatedAt.In(time.FixedZone("MSK", 3*3600)).Format("02.01 15:04")
	amount := adminFormatPaymentAmount(p.AmountMinor, p.Currency)
	access := ""
	if p.Status == "completed" {
		if p.AccessActive {
			access = " · доступ ✅"
		} else {
			access = " · доступ ⛔️"
		}
	}
	return fmt.Sprintf("#%d · %s · %s · %s · %s%s",
		p.ID,
		adminPaywallPersonLabel(p.Username, p.DisplayName, p.UserID),
		p.Status,
		amount,
		when,
		access,
	)
}

func adminFormatPaymentAmount(amountMinor sql.NullInt64, currency sql.NullString) string {
	if !amountMinor.Valid || amountMinor.Int64 <= 0 {
		return "—"
	}
	cur := "RUB"
	if currency.Valid && strings.TrimSpace(currency.String) != "" {
		cur = strings.ToUpper(strings.TrimSpace(currency.String))
	}
	major := float64(amountMinor.Int64) / 100.0
	if cur == "XTR" || strings.EqualFold(cur, "STARS") {
		return fmt.Sprintf("%d ⭐", amountMinor.Int64)
	}
	return fmt.Sprintf("%.0f %s", major, cur)
}

func (b *Bot) formatAdminUserPaymentsBlock(userID, packChatID int64) string {
	payments, err := b.db.ListPaywallPaymentsForUserAdmin(userID, packChatID, 5)
	if err != nil || len(payments) == 0 {
		return "\n\n💳 Оплаты: нет заявок"
	}
	var sb strings.Builder
	sb.WriteString("\n\n💳 Оплаты (последние):\n")
	for _, p := range payments {
		sb.WriteString("• ")
		sb.WriteString(adminPaywallPaymentLine(p))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (b *Bot) handleAdminDirectoryCallback(callback *tgbotapi.CallbackQuery) bool {
	if callback == nil || callback.Message == nil {
		return false
	}
	data := callback.Data
	chatID := callback.Message.Chat.ID

	if strings.HasPrefix(data, "admin_users_list_") {
		off, err := strconv.Atoi(strings.TrimPrefix(data, "admin_users_list_"))
		if err != nil || off < 0 {
			off = 0
		}
		b.showAdminUsersListPage(chatID, off)
		return true
	}
	if strings.HasPrefix(data, "admin_payments_") {
		off, err := strconv.Atoi(strings.TrimPrefix(data, "admin_payments_"))
		if err != nil || off < 0 {
			off = 0
		}
		b.showAdminPaymentsPage(chatID, off)
		return true
	}
	return false
}
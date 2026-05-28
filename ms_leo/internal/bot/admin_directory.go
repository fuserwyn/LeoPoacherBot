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
	adminUsersListPageSize    = 10
	adminPaymentsListPageSize = 8
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
	users, err := b.db.ListPackUsersForAdmin(packChatID, offset, adminUsersListPageSize)
	if err != nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка списка: "+err.Error()))
		return
	}

	subtitle := "Пока никого нет."
	tableText := ""
	if total > 0 {
		from := offset + 1
		to := offset + len(users)
		subtitle = fmt.Sprintf("Строки %d–%d из %d · нажми № под таблицей", from, to, total)

		tbl := newAdminTable(
			[]string{"№", "ID", "Ник", "Куб", "Стр"},
			[]int{2, 11, 18, 5, 4},
		)
		for i, u := range users {
			tbl.addRow(
				strconv.Itoa(offset+i+1),
				strconv.FormatInt(u.UserID, 10),
				adminPaywallPersonLabel(u.Username, u.DisplayName, u.UserID),
				strconv.Itoa(u.Cups),
				strconv.Itoa(u.StreakDays),
			)
		}
		tableText = tbl.render()
	}

	keyboardRows := make([][]tgbotapi.InlineKeyboardButton, 0, 4)
	if len(users) > 0 {
		keyboardRows = appendAdminPickButtonRows(keyboardRows, users, offset)
	}
	nav := make([]tgbotapi.InlineKeyboardButton, 0, 2)
	if offset > 0 {
		prev := offset - adminUsersListPageSize
		if prev < 0 {
			prev = 0
		}
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "admin_users_list_"+strconv.Itoa(prev)))
	}
	if offset+len(users) < total {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("▶️ Далее", "admin_users_list_"+strconv.Itoa(offset+adminUsersListPageSize)))
	}
	if len(nav) > 0 {
		keyboardRows = append(keyboardRows, nav)
	}
	keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔍 Найти", "admin_users"),
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Админка", "admin_open"),
	))

	kb := &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboardRows}
	b.sendAdminHTMLPreTable(chatID, "📋 Пользователи стаи", subtitle, tableText, kb)
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

	subtitle := "Заявок пока нет."
	tableText := ""
	if total > 0 {
		from := offset + 1
		to := offset + len(payments)
		subtitle = fmt.Sprintf("Строки %d–%d из %d · нажми № под таблицей", from, to, total)

		tbl := newAdminTable(
			[]string{"№", "Заявка", "ID", "Ник", "Статус", "Сумма", "Дата", "Дост"},
			[]int{2, 7, 11, 14, 9, 8, 11, 4},
		)
		for i, p := range payments {
			access := "—"
			if p.Status == "completed" {
				if p.AccessActive {
					access = "да"
				} else {
					access = "нет"
				}
			}
			when := p.CreatedAt.In(time.FixedZone("MSK", 3*3600)).Format("02.01 15:04")
			tbl.addRow(
				strconv.Itoa(offset+i+1),
				strconv.FormatInt(p.ID, 10),
				strconv.FormatInt(p.UserID, 10),
				adminPaywallPersonLabel(p.Username, p.DisplayName, p.UserID),
				p.Status,
				adminFormatPaymentAmount(p.AmountMinor, p.Currency),
				when,
				access,
			)
		}
		tableText = tbl.render()
	}

	keyboardRows := make([][]tgbotapi.InlineKeyboardButton, 0, 4)
	if len(payments) > 0 {
		pickUsers := make([]database.AdminPackUserListRow, len(payments))
		for i, p := range payments {
			pickUsers[i] = database.AdminPackUserListRow{UserID: p.UserID}
		}
		keyboardRows = appendAdminPickButtonRows(keyboardRows, pickUsers, offset)
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

	kb := &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboardRows}
	b.sendAdminHTMLPreTable(chatID, "💳 Оплаты", subtitle, tableText, kb)
}

func appendAdminPickButtonRows(rows [][]tgbotapi.InlineKeyboardButton, users []database.AdminPackUserListRow, offset int) [][]tgbotapi.InlineKeyboardButton {
	const perRow = 5
	btnRow := make([]tgbotapi.InlineKeyboardButton, 0, perRow)
	for i, u := range users {
		btnRow = append(btnRow, tgbotapi.NewInlineKeyboardButtonData(
			strconv.Itoa(offset+i+1),
			"admin_user_open_"+strconv.FormatInt(u.UserID, 10),
		))
		if len(btnRow) >= perRow {
			rows = append(rows, btnRow)
			btnRow = make([]tgbotapi.InlineKeyboardButton, 0, perRow)
		}
	}
	if len(btnRow) > 0 {
		rows = append(rows, btnRow)
	}
	return rows
}

// adminPaywallPersonLabel — только Telegram-ник (@username) для таблиц админки.
func adminPaywallPersonLabel(username, displayName string, userID int64) string {
	_ = displayName
	u := strings.TrimSpace(username)
	if u == "" {
		return strconv.FormatInt(userID, 10)
	}
	if strings.HasPrefix(u, "@") {
		return u
	}
	if strings.HasPrefix(strings.ToLower(u), "user") {
		return strconv.FormatInt(userID, 10)
	}
	if !strings.Contains(u, " ") && adminLooksLikeTelegramHandle(u) {
		return "@" + u
	}
	return strconv.FormatInt(userID, 10)
}

func adminLooksLikeTelegramHandle(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
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
		return fmt.Sprintf("%d⭐", amountMinor.Int64)
	}
	return fmt.Sprintf("%.0f%s", major, cur)
}

func (b *Bot) formatAdminUserPaymentsBlock(userID, packChatID int64) string {
	payments, err := b.db.ListPaywallPaymentsForUserAdmin(userID, packChatID, 5)
	if err != nil || len(payments) == 0 {
		return "\n\n💳 Оплаты: нет заявок"
	}
	tbl := newAdminTable(
		[]string{"Заявка", "Статус", "Сумма", "Дата", "Дост"},
		[]int{7, 9, 8, 11, 4},
	)
	for _, p := range payments {
		access := "—"
		if p.Status == "completed" {
			if p.AccessActive {
				access = "да"
			} else {
				access = "нет"
			}
		}
		when := p.CreatedAt.In(time.FixedZone("MSK", 3*3600)).Format("02.01 15:04")
		tbl.addRow(
			strconv.FormatInt(p.ID, 10),
			p.Status,
			adminFormatPaymentAmount(p.AmountMinor, p.Currency),
			when,
			access,
		)
	}
	return "\n\n💳 Оплаты (последние):\n<pre>" + adminEscapeHTML(tbl.render()) + "</pre>"
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

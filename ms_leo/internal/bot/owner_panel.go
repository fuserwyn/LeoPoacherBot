package bot

import (
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ─── Owner command ────────────────────────────────────────────────────────────

func (b *Bot) handleOwner(msg *tgbotapi.Message) {
	if msg == nil || msg.From == nil {
		return
	}
	if !b.isOwnerOnly(msg.From.ID) || !msg.Chat.IsPrivate() {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Панель владельца доступна только владельцу в личном чате с ботом.")
		b.api.Send(reply)
		return
	}
	b.showOwnerMenu(msg.Chat.ID)
}

// ─── Owner menu ───────────────────────────────────────────────────────────────

func (b *Bot) showOwnerMenu(chatID int64) {
	text := "👑 Панель владельца\n\nВыбери действие:"
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👥 Список администраторов", "owner_admin_list"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Добавить администратора", "owner_admin_add"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ К админ-панели", "admin_open"),
		),
	)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	b.api.Send(msg)
}

// ─── Callback routing ─────────────────────────────────────────────────────────

// handleOwnerCallbackQuery — обрабатывает owner_* callback-и; возвращает true если обработано.
func (b *Bot) handleOwnerCallbackQuery(callback *tgbotapi.CallbackQuery) bool {
	if callback == nil || callback.Message == nil || callback.From == nil {
		return false
	}
	if !b.isOwnerOnly(callback.From.ID) || !callback.Message.Chat.IsPrivate() {
		b.api.Request(tgbotapi.NewCallback(callback.ID, "Недостаточно прав"))
		return true
	}

	data := callback.Data
	chatID := callback.Message.Chat.ID

	switch {
	case data == "owner_menu":
		b.showOwnerMenu(chatID)

	case data == "owner_admin_list":
		b.showOwnerAdminList(chatID)

	case data == "owner_admin_add":
		b.startOwnerAdminAddFlow(callback.From.ID)
		b.api.Send(tgbotapi.NewMessage(chatID,
			"➕ Введи Telegram ID нового администратора (числовой ID).\nПример: 123456789\n\nОтмена — /cancel"))

	case strings.HasPrefix(data, "owner_admin_remove_"):
		idStr := strings.TrimPrefix(data, "owner_admin_remove_")
		targetID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || targetID == 0 {
			b.api.Send(tgbotapi.NewMessage(chatID, "❌ Неверный ID."))
			break
		}
		b.removeOwnerAdmin(chatID, callback.From.ID, targetID)

	default:
		return false
	}

	b.api.Request(tgbotapi.NewCallback(callback.ID, ""))
	return true
}

// ─── Admin management ─────────────────────────────────────────────────────────

func (b *Bot) showOwnerAdminList(chatID int64) {
	admins, err := b.db.ListDynamicAdmins()
	if err != nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Не удалось загрузить список: "+err.Error()))
		return
	}

	var sb strings.Builder
	sb.WriteString("👥 Администраторы\n\n")

	// Сначала — из env (статические)
	envIDs := b.config.AdminTelegramUserIDs()
	if len(envIDs) > 0 {
		sb.WriteString("📌 Из конфига (env):\n")
		for _, id := range envIDs {
			if id == b.config.OwnerID {
				sb.WriteString(fmt.Sprintf("  👑 Владелец · %d\n", id))
			} else {
				sb.WriteString(fmt.Sprintf("  ⚙️ %d\n", id))
			}
		}
		sb.WriteString("\n")
	}

	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(admins)+2)

	if len(admins) == 0 {
		sb.WriteString("Динамических администраторов нет.")
	} else {
		sb.WriteString("🔧 Добавлены через бот:\n")
		for i, a := range admins {
			name := ownerDisplayName(a.Username, "", a.UserID)
			sb.WriteString(fmt.Sprintf("%d. %s · добавлен %s\n",
				i+1, name, a.AddedAt.Format("02.01.2006")))
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					"❌ Удалить: "+clipAdminSupportText(name, 30),
					"owner_admin_remove_"+strconv.FormatInt(a.UserID, 10),
				),
			))
		}
	}

	rows = append(rows,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Добавить", "owner_admin_add"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", "owner_menu"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, clipAdminSupportText(sb.String(), 3800))
	msg.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
	b.api.Send(msg)
}

func (b *Bot) removeOwnerAdmin(chatID, ownerID, targetID int64) {
	// Запрещаем удалять владельца или env-админов
	if b.config.IsAdminTelegramUser(targetID) {
		b.api.Send(tgbotapi.NewMessage(chatID, "⚠️ Этот пользователь в конфиге — удали его из ADMIN_IDS в .env."))
		return
	}
	ok, err := b.db.RemoveDynamicAdmin(targetID)
	if err != nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка удаления: "+err.Error()))
		return
	}
	if !ok {
		b.api.Send(tgbotapi.NewMessage(chatID, "⚠️ Администратор не найден."))
		return
	}
	b.reloadDynamicAdmins()
	b.api.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Администратор %d удалён.", targetID)))
	b.showOwnerAdminList(chatID)
}

// ─── Admin add flow ───────────────────────────────────────────────────────────

func (b *Bot) startOwnerAdminAddFlow(ownerID int64) {
	b.adminSessionsMutex.Lock()
	defer b.adminSessionsMutex.Unlock()
	b.adminSessions[ownerID] = &adminSession{
		Mode: "owner_add_admin",
		Step: "await_admin_id",
	}
}

// handleOwnerAdminAddMessage — обрабатывает ввод Telegram ID при добавлении админа.
// Возвращает true, если сообщение обработано (сессия была активна).
func (b *Bot) handleOwnerAdminAddMessage(msg *tgbotapi.Message) bool {
	if msg == nil || msg.From == nil || !b.isOwnerOnly(msg.From.ID) {
		return false
	}
	session, ok := b.getAdminSession(msg.From.ID)
	if !ok || session == nil || session.Mode != "owner_add_admin" {
		return false
	}

	raw := strings.TrimSpace(msg.Text)

	// Поддерживаем @username — ищем в training_state
	var targetID int64
	var username string

	if strings.HasPrefix(raw, "@") {
		nick := strings.TrimPrefix(raw, "@")
		id, err := b.db.FindUserIDByUsername(nick)
		if err != nil || id == 0 {
			b.api.Send(tgbotapi.NewMessage(msg.Chat.ID,
				"❌ Пользователь @"+nick+" не найден среди пользователей бота.\nВведи числовой Telegram ID или /cancel."))
			return true
		}
		targetID = id
		username = nick
	} else {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			b.api.Send(tgbotapi.NewMessage(msg.Chat.ID,
				"⚠️ Нужен числовой Telegram ID (например 123456789) или @username.\n/cancel — отмена."))
			return true
		}
		targetID = id
	}

	if b.isOwnerOnly(targetID) {
		b.clearAdminFlow(msg.From.ID)
		b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "⚠️ Это владелец бота — дополнительных прав не нужно."))
		return true
	}
	if b.config.IsAdminTelegramUser(targetID) {
		b.clearAdminFlow(msg.From.ID)
		b.api.Send(tgbotapi.NewMessage(msg.Chat.ID,
			fmt.Sprintf("ℹ️ Пользователь %d уже является администратором (через конфиг).", targetID)))
		return true
	}

	if err := b.db.AddDynamicAdmin(targetID, username, msg.From.ID); err != nil {
		b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Ошибка сохранения: "+err.Error()))
		return true
	}
	b.reloadDynamicAdmins()
	b.clearAdminFlow(msg.From.ID)
	b.api.Send(tgbotapi.NewMessage(msg.Chat.ID,
		fmt.Sprintf("✅ Администратор %d добавлен.", targetID)))
	b.showOwnerAdminList(msg.Chat.ID)
	return true
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func ownerDisplayName(username, firstName string, userID int64) string {
	if username != "" {
		return "@" + username
	}
	if firstName != "" {
		return firstName
	}
	return strconv.FormatInt(userID, 10)
}

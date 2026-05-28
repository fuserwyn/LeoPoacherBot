package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"leo-bot/internal/game/leopardmoney"
	"leo-bot/internal/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// syncAchievementsFromStreak выставляет achievement_count по порогам стрика на pack-row и private-row.
func (b *Bot) syncAchievementsFromStreak(userID, packChatID int64, streakDays int) (int, error) {
	if b == nil || b.db == nil || userID == 0 || packChatID == 0 {
		return 0, nil
	}
	want := leopardmoney.AchievementsCountForStreak(streakDays)
	last := leopardmoney.LastAchievementMilestoneForStreak(streakDays)
	if err := b.db.SetAchievementsForUserScope(userID, packChatID, want, last); err != nil {
		return 0, err
	}
	return want, nil
}

func (b *Bot) adminPackChatID() int64 {
	if b != nil && b.config != nil && b.config.MonetizedChatID != 0 {
		return b.config.MonetizedChatID
	}
	return 0
}

func (b *Bot) startAdminUserMgmt(adminID int64) {
	b.adminSessionsMutex.Lock()
	b.adminSessions[adminID] = &adminSession{Mode: "user_mgmt", Step: "await_user_id"}
	b.adminSessionsMutex.Unlock()
}

func (b *Bot) startAdminUserAction(adminID, targetUserID int64, mode, step string) {
	b.adminSessionsMutex.Lock()
	b.adminSessions[adminID] = &adminSession{
		Mode:         mode,
		Step:         step,
		TargetUserID: targetUserID,
	}
	b.adminSessionsMutex.Unlock()
}

func (b *Bot) showAdminUserCard(chatID, targetUserID int64) {
	packChatID := b.adminPackChatID()
	if packChatID == 0 {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Не настроен MonetizedChatID."))
		return
	}

	ml, err := b.db.GetMessageLogAnyState(targetUserID, packChatID)
	if err != nil || ml == nil {
		b.api.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Пользователь %d не найден в стае.", targetUserID)))
		return
	}
	if _, syncErr := b.syncAchievementsFromStreak(targetUserID, packChatID, ml.StreakDays); syncErr != nil {
		b.logger.Warnf("admin sync achievements user=%d: %v", targetUserID, syncErr)
	} else {
		ml, _ = b.db.GetMessageLogAnyState(targetUserID, packChatID)
	}

	stats := b.GetMiniappProfileStatsForAPI(targetUserID, packChatID)
	sick := "нет"
	if ml.HasSickLeave && !ml.HasHealthy {
		sick = "активен"
	} else if ml.SickApprovalPending {
		sick = "ожидает подтверждения"
	}
	deleted := "нет"
	if ml.IsDeleted {
		deleted = "да"
	}
	paywall := "нет активного доступа"
	if active, err := b.db.UserHasActivePaywallAccess(targetUserID, packChatID); err == nil && active {
		paywall = "оплачен ✅"
	}

	var body strings.Builder
	body.WriteString("<b>👤 Пользователь</b>\n\n")
	body.WriteString(fmt.Sprintf("ID: <code>%d</code>\n", targetUserID))
	body.WriteString(fmt.Sprintf("Имя: %s\n", adminEscapeHTML(adminSupportTitle(ml.Username, targetUserID))))
	body.WriteString(fmt.Sprintf("Кубки: %d\n", stats.XP))
	body.WriteString(fmt.Sprintf("Стрик: %d (рекорд %d)\n", stats.StreakDays, stats.MaxStreakDays))
	if stats.DaysSinceLastTraining >= 0 {
		body.WriteString(fmt.Sprintf("Дней без тренировки: %d\n", stats.DaysSinceLastTraining))
	} else {
		body.WriteString("Дней без тренировки: —\n")
	}
	if removalAt := b.GetMiniappInactivityRemovalDeadlineRFC3339(targetUserID, packChatID); removalAt != "" {
		body.WriteString(fmt.Sprintf("Кик за неактивность: %s\n", adminEscapeHTML(removalAt)))
	}
	body.WriteString(fmt.Sprintf("Попытки спасти стрик: %d/%d (доступно %d)\n", stats.StreakSaveAttemptsUsed, stats.StreakSaveAttemptsMax, stats.StreakSaveAttemptsAvail))
	body.WriteString(fmt.Sprintf("Больничный: %s\n", adminEscapeHTML(sick)))
	body.WriteString(fmt.Sprintf("Удалён из стаи: %s\n", adminEscapeHTML(deleted)))
	body.WriteString(fmt.Sprintf("Доступ: %s\n", adminEscapeHTML(paywall)))
	if ugc, uerr := b.db.GetUGCModerationState(targetUserID, packChatID); uerr == nil {
		body.WriteString(fmt.Sprintf("UGC-нарушения: %d\n", ugc.ViolationCount))
		if ugc.MutedUntil != nil && ugc.MutedUntil.After(time.Now()) {
			body.WriteString(fmt.Sprintf("UGC-мьют до: %s\n", adminEscapeHTML(ugc.MutedUntil.UTC().Format("2006-01-02 15:04 UTC"))))
		}
	}
	body.WriteString(fmt.Sprintf("Ачивки: %d/%d", stats.AchievementCount, stats.AchievementsMax))
	body.WriteString(b.formatAdminUserPaymentsBlock(targetUserID, packChatID))

	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Кубки", "admin_user_cups_"+strconv.FormatInt(targetUserID, 10)),
			tgbotapi.NewInlineKeyboardButtonData("➕ Стрик", "admin_user_streak_"+strconv.FormatInt(targetUserID, 10)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🛟 +1 спасти стрик", "admin_user_save_"+strconv.FormatInt(targetUserID, 10)),
			tgbotapi.NewInlineKeyboardButtonData("🏥 Отменить больничный", "admin_user_sick_"+strconv.FormatInt(targetUserID, 10)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⏳ Дней без тренировки", "admin_user_inactive_"+strconv.FormatInt(targetUserID, 10)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🗑 Удалить сообщение", "admin_user_msg_"+strconv.FormatInt(targetUserID, 10)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔇 UGC-мьют 24ч", "admin_user_mute_ugc_"+strconv.FormatInt(targetUserID, 10)),
		),
	}
	if !ml.IsDeleted {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⛔️ Удалить из стаи", "admin_user_del_"+strconv.FormatInt(targetUserID, 10)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📋 К списку", "admin_users_list_0"),
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Админка", "admin_open"),
	))

	msg := tgbotapi.NewMessage(chatID, body.String())
	msg.ParseMode = "HTML"
	msg.DisableWebPagePreview = true
	msg.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
	b.api.Send(msg)
}

func (b *Bot) handleAdminUserMgmtCallback(callback *tgbotapi.CallbackQuery) bool {
	if callback == nil || callback.Message == nil {
		return false
	}
	data := callback.Data
	chatID := callback.Message.Chat.ID
	adminID := callback.From.ID

	parseTarget := func(prefix string) (int64, bool) {
		id, err := strconv.ParseInt(strings.TrimPrefix(data, prefix), 10, 64)
		return id, err == nil && id > 0
	}

	switch {
	case data == "admin_users":
		b.startAdminUserMgmt(adminID)
		b.api.Send(tgbotapi.NewMessage(chatID, "👤 Отправь Telegram user_id, @ник или имя из профиля."))
		return true

	case strings.HasPrefix(data, "admin_user_pick_"):
		targetID, ok := parseTarget("admin_user_pick_")
		if ok {
			b.clearAdminFlow(adminID)
			b.showAdminUserCard(chatID, targetID)
		}
		return true

	case strings.HasPrefix(data, "admin_user_open_"):
		targetID, ok := parseTarget("admin_user_open_")
		if ok {
			b.showAdminUserCard(chatID, targetID)
		}
		return true

	case strings.HasPrefix(data, "admin_user_cups_"):
		targetID, ok := parseTarget("admin_user_cups_")
		if ok {
			b.startAdminUserAction(adminID, targetID, "user_add_cups", "await_amount")
			b.api.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("➕ Сколько кубков добавить пользователю %d? Отправь число.", targetID)))
		}
		return true

	case strings.HasPrefix(data, "admin_user_streak_"):
		targetID, ok := parseTarget("admin_user_streak_")
		if ok {
			b.startAdminUserAction(adminID, targetID, "user_add_streak", "await_amount")
			b.api.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("➕ На сколько дней увеличить стрик пользователю %d? Отправь число.", targetID)))
		}
		return true

	case strings.HasPrefix(data, "admin_user_save_"):
		targetID, ok := parseTarget("admin_user_save_")
		if ok {
			b.adminGrantStreakSaveAttempt(chatID, targetID)
		}
		return true

	case strings.HasPrefix(data, "admin_user_sick_"):
		targetID, ok := parseTarget("admin_user_sick_")
		if ok {
			b.adminCancelSickLeave(chatID, targetID)
		}
		return true

	case strings.HasPrefix(data, "admin_user_inactive_"):
		targetID, ok := parseTarget("admin_user_inactive_")
		if ok {
			b.startAdminUserAction(adminID, targetID, "user_set_inactive", "await_days")
			b.api.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf(
				"⏳ Сколько дней пользователь %d не занимался?\n\n"+
					"0 — тренировка сегодня\n"+
					"5 — жёлтое предупреждение в профиле\n"+
					"6 — красное, 7+ — борdeaux\n\n"+
					"Для безопасного теста кика лучше /set_exempt на тестовом аккаунте.",
				targetID)))
		}
		return true

	case strings.HasPrefix(data, "admin_user_msg_"):
		targetID, ok := parseTarget("admin_user_msg_")
		if ok {
			b.startAdminUserAction(adminID, targetID, "user_delete_msg", "await_message_id")
			b.api.Send(tgbotapi.NewMessage(chatID,
				fmt.Sprintf("🗑 Отправь ID сообщения для удаления.\n\n"+
					"• #123 — пост в ленте (user_messages)\n"+
					"• t123 — комментарий в треде\n"+
					"• g123 — сообщение общего чата стаи\n\n"+
					"Пользователь: %d", targetID)))
		}
		return true

	case strings.HasPrefix(data, "admin_user_del_yes_"):
		targetID, ok := parseTarget("admin_user_del_yes_")
		if ok {
			b.adminDeleteUser(chatID, targetID)
		}
		return true

	case strings.HasPrefix(data, "admin_user_del_"):
		targetID, ok := parseTarget("admin_user_del_")
		if ok {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("⛔️ Удалить пользователя %d из стаи?\nКубки и стрик сгорят, доступ к мини-аппу отзовётся.", targetID))
			msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("✅ Да, удалить", "admin_user_del_yes_"+strconv.FormatInt(targetID, 10)),
					tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "admin_user_open_"+strconv.FormatInt(targetID, 10)),
				),
			)
			b.api.Send(msg)
		}
		return true

	case strings.HasPrefix(data, "admin_feed_report_del_"):
		reportID, err := strconv.ParseInt(strings.TrimPrefix(data, "admin_feed_report_del_"), 10, 64)
		if err == nil && reportID > 0 {
			b.adminDeleteReportedContent(chatID, reportID)
		}
		return true

	case strings.HasPrefix(data, "admin_feed_report_hide_"):
		reportID, err := strconv.ParseInt(strings.TrimPrefix(data, "admin_feed_report_hide_"), 10, 64)
		if err == nil && reportID > 0 {
			b.adminHideReportedContent(chatID, reportID)
		}
		return true

	case strings.HasPrefix(data, "admin_feed_report_mute_"):
		reportID, err := strconv.ParseInt(strings.TrimPrefix(data, "admin_feed_report_mute_"), 10, 64)
		if err == nil && reportID > 0 {
			b.adminMuteReportedUser(chatID, reportID)
		}
		return true

	case strings.HasPrefix(data, "admin_user_mute_ugc_"):
		targetID, ok := parseTarget("admin_user_mute_ugc_")
		if ok {
			b.adminMuteUserUGC(chatID, targetID, b.adminPackChatID(), 24)
		}
		return true
	}
	return false
}

func (b *Bot) handleAdminUserMgmtMessage(msg *tgbotapi.Message, session *adminSession) bool {
	if session == nil {
		return false
	}
	text := strings.TrimSpace(msg.Text)

	switch session.Step {
	case "await_user_id":
		b.clearAdminFlow(msg.From.ID)
		b.resolveAdminUserSearch(msg.Chat.ID, text)
		return true

	case "await_days":
		days, err := strconv.Atoi(text)
		if err != nil || days < 0 || days > 14 {
			b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "⚠️ Нужно число от 0 до 14."))
			return true
		}
		targetID := session.TargetUserID
		b.clearAdminFlow(msg.From.ID)
		b.adminSetDaysInactive(msg.Chat.ID, targetID, days)
		return true

	case "await_amount":
		amount, err := strconv.Atoi(text)
		if err != nil || amount <= 0 || amount > 100000 {
			b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "⚠️ Нужно положительное число (до 100000)."))
			return true
		}
		targetID := session.TargetUserID
		mode := session.Mode
		b.clearAdminFlow(msg.From.ID)
		packChatID := b.adminPackChatID()
		switch mode {
		case "user_add_cups":
			if err := b.db.AddCups(targetID, packChatID, amount); err != nil {
				b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Ошибка: "+err.Error()))
				return true
			}
			b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("✅ Добавлено %d кубков пользователю %d.", amount, targetID)))
		case "user_add_streak":
			ml, err := b.db.GetMessageLogAnyState(targetID, packChatID)
			if err != nil || ml == nil {
				b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Пользователь не найден."))
				return true
			}
			oldStreak := ml.StreakDays
			newStreak := oldStreak + amount
			lastDate := ""
			if ml.LastTrainingDate != nil {
				lastDate = strings.TrimSpace(*ml.LastTrainingDate)
			}
			if lastDate == "" {
				lastDate = time.Now().UTC().Format("2006-01-02")
			}
			if err := b.db.UpdateStreak(targetID, packChatID, newStreak, lastDate); err != nil {
				b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Ошибка: "+err.Error()))
				return true
			}
			achCount, achErr := b.syncAchievementsFromStreak(targetID, packChatID, newStreak)
			achNote := ""
			if achErr == nil {
				achNote = fmt.Sprintf(" Ачивки: %d/%d.", achCount, leopardmoney.MaxAchievements)
			}
			b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("✅ Стрик пользователя %d: %d → %d дней.%s", targetID, oldStreak, newStreak, achNote)))
		}
		b.showAdminUserCard(msg.Chat.ID, targetID)
		return true

	case "await_message_id":
		targetID := session.TargetUserID
		b.clearAdminFlow(msg.From.ID)
		ok, label := b.adminDeleteMessageByRef(text)
		if !ok {
			b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Сообщение не найдено или уже удалено."))
		} else {
			b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("✅ Удалено: %s", label)))
		}
		b.showAdminUserCard(msg.Chat.ID, targetID)
		return true
	}
	return false
}

func (b *Bot) resolveAdminUserSearch(chatID int64, query string) {
	packChatID := b.adminPackChatID()
	if packChatID == 0 {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Не настроен MonetizedChatID."))
		return
	}
	query = strings.TrimSpace(query)
	if query == "" {
		b.api.Send(tgbotapi.NewMessage(chatID, "⚠️ Введи user_id, @ник или имя."))
		return
	}

	hits, err := b.db.SearchPackUsersForAdmin(packChatID, query, 10)
	if err != nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка поиска: "+err.Error()))
		return
	}
	if len(hits) == 0 {
		b.api.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Пользователь «%s» не найден.", query)))
		return
	}
	if len(hits) == 1 {
		b.showAdminUserCard(chatID, hits[0].UserID)
		return
	}

	tbl := newAdminTable(
		[]string{"№", "ID", "Ник"},
		[]int{2, 11, 18},
	)
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, 3)
	for i, hit := range hits {
		tbl.addRow(
			strconv.Itoa(i+1),
			strconv.FormatInt(hit.UserID, 10),
			adminPaywallPersonLabel(hit.Username, hit.DisplayName, hit.UserID),
		)
	}
	// Кнопки pick используют admin_user_pick_ вместо admin_user_open_
	const perRow = 5
	btnRow := make([]tgbotapi.InlineKeyboardButton, 0, perRow)
	for i, hit := range hits {
		btnRow = append(btnRow, tgbotapi.NewInlineKeyboardButtonData(
			strconv.Itoa(i+1),
			"admin_user_pick_"+strconv.FormatInt(hit.UserID, 10),
		))
		if len(btnRow) >= perRow {
			rows = append(rows, btnRow)
			btnRow = make([]tgbotapi.InlineKeyboardButton, 0, perRow)
		}
	}
	if len(btnRow) > 0 {
		rows = append(rows, btnRow)
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Админка", "admin_open"),
	))
	subtitle := fmt.Sprintf("Найдено %d по «%s» · нажми №", len(hits), query)
	kb := &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
	b.sendAdminHTMLPreTable(chatID, "🔍 Поиск пользователя", subtitle, tbl.render(), kb)
}

// adminSetDaysInactive — для UI-тестов: backdate last_training_date и timer_start_time.
func (b *Bot) adminSetDaysInactive(chatID, targetUserID int64, days int) {
	packChatID := b.adminPackChatID()
	if packChatID == 0 {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Не настроен MonetizedChatID."))
		return
	}
	ml, err := b.db.GetMessageLogAnyState(targetUserID, packChatID)
	if err != nil || ml == nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Пользователь не найден."))
		return
	}
	if days < 0 {
		days = 0
	}
	if days > 14 {
		days = 14
	}

	tzOffset := ml.TimezoneOffsetFromMoscow
	today, err := time.Parse("2006-01-02", b.getUserLocalDate(tzOffset))
	if err != nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Не удалось определить локальную дату пользователя."))
		return
	}
	targetDate := today.AddDate(0, 0, -days).Format("2006-01-02")

	if err := b.db.UpdateStreak(targetUserID, packChatID, ml.StreakDays, targetDate); err != nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка: "+err.Error()))
		return
	}

	b.cancelTimer(targetUserID)
	timerStart := utils.FormatMoscowTime(utils.GetMoscowTime().Add(-time.Duration(days) * 24 * time.Hour))
	ml, err = b.db.GetMessageLogAnyState(targetUserID, packChatID)
	if err != nil || ml == nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Пользователь не найден после обновления."))
		return
	}
	ml.TimerStartTime = &timerStart
	if err := b.db.SaveMessageLog(ml); err != nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка сохранения таймера: "+err.Error()))
		return
	}

	ml, _ = b.db.GetMessageLogAnyState(targetUserID, packChatID)
	username := ml.Username
	if username == "" {
		username = fmt.Sprintf("User%d", targetUserID)
	}

	timerNote := ""
	if ml.HasSickLeave && !ml.HasHealthy {
		timerNote = " ⚠️ Активен больничный — дедлайн кика может отличаться от профиля."
	} else if ml.IsExemptFromDeletion {
		timerNote = " ℹ️ Пользователь exempt — таймер кика не запущен."
	} else {
		remaining := b.calculateRemainingTime(ml)
		if remaining > 0 {
			b.restoreTimerWithDuration(targetUserID, packChatID, username, remaining, timerStart, tzOffset)
		} else {
			timerNote = " ⚠️ Дедлайн кика уже прошёл — таймер не перезапущен (чтобы не удалить из стаи)."
		}
	}

	stats := b.GetMiniappProfileStatsForAPI(targetUserID, packChatID)
	removalAt := b.GetMiniappInactivityRemovalDeadlineRFC3339(targetUserID, packChatID)
	msg := fmt.Sprintf(
		"✅ Пользователь %d: %d дн. без тренировки (last_training_date=%s).",
		targetUserID, stats.DaysSinceLastTraining, targetDate,
	)
	if removalAt != "" {
		msg += fmt.Sprintf("\nКик: %s.", removalAt)
	}
	msg += timerNote
	b.api.Send(tgbotapi.NewMessage(chatID, msg))
	b.showAdminUserCard(chatID, targetUserID)
}

func (b *Bot) adminGrantStreakSaveAttempt(chatID, targetUserID int64) {
	packChatID := b.adminPackChatID()
	used, err := b.db.DecrementStreakSaveAttemptsUsed(targetUserID, packChatID, 1)
	if err != nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка: "+err.Error()))
		return
	}
	stats := b.GetMiniappProfileStatsForAPI(targetUserID, packChatID)
	avail := stats.StreakSaveAttemptsMax - used
	if avail < 0 {
		avail = 0
	}
	b.api.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Пользователю %d выдана +1 попытка спасти стрик. Доступно: %d/%d.", targetUserID, avail, stats.StreakSaveAttemptsMax)))
	b.showAdminUserCard(chatID, targetUserID)
}

func (b *Bot) adminCancelSickLeave(chatID, targetUserID int64) {
	packChatID := b.adminPackChatID()
	if err := b.cancelSickLeaveForUser(targetUserID, packChatID); err != nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ "+err.Error()))
		return
	}
	b.api.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Больничный пользователя %d отменён, таймер возобновлён.", targetUserID)))
	b.notifyUserTextByID(targetUserID, packChatID, "🏥 Больничный завершён администратором. Таймер неактивности снова активен — не забудь про тренировку!", "", 0)
	b.showAdminUserCard(chatID, targetUserID)
}

// cancelSickLeaveForUser — админ/системная отмена активного больничного (логика как #healthy).
func (b *Bot) cancelSickLeaveForUser(userID, packChatID int64) error {
	messageLog, err := b.db.GetMessageLogAnyState(userID, packChatID)
	if err != nil || messageLog == nil {
		return fmt.Errorf("пользователь не найден")
	}
	if !messageLog.HasSickLeave || messageLog.HasHealthy {
		if messageLog.SickApprovalPending {
			b.forceCancelSickLeave(userID, packChatID)
			return nil
		}
		return fmt.Errorf("активного больничного нет")
	}

	b.cancelSickApprovalWatcher(userID)
	messageLog.SickApprovalPending = false
	messageLog.SickApprovalDeadline = nil
	messageLog.SickApprovalMessageID = nil

	sickLeaveEndTime := utils.FormatMoscowTime(utils.GetMoscowTime())
	messageLog.SickLeaveEndTime = &sickLeaveEndTime
	if messageLog.SickLeaveStartTime != nil {
		if sickStart, err := utils.ParseMoscowTime(*messageLog.SickLeaveStartTime); err == nil {
			if sickEnd, err := utils.ParseMoscowTime(sickLeaveEndTime); err == nil {
				sickTimeStr := sickEnd.Sub(sickStart).String()
				messageLog.SickTime = &sickTimeStr
			}
		}
	}
	messageLog.HasHealthy = true
	messageLog.HasSickLeave = false

	if err := b.db.SaveMessageLog(messageLog); err != nil {
		return err
	}

	remainingTime := b.calculateRemainingTime(messageLog)
	if remainingTime <= 0 {
		username := messageLog.Username
		if username == "" {
			username = fmt.Sprintf("User%d", userID)
		}
		b.removeUser(userID, packChatID, username)
		return nil
	}

	if messageLog.TimerStartTime == nil || strings.TrimSpace(*messageLog.TimerStartTime) == "" {
		b.startTimerWithDuration(userID, packChatID, messageLog.Username, remainingTime)
	} else {
		ts := strings.TrimSpace(*messageLog.TimerStartTime)
		b.restoreTimerWithDuration(userID, packChatID, messageLog.Username, remainingTime, ts, messageLog.TimezoneOffsetFromMoscow)
	}
	return nil
}

func (b *Bot) adminDeleteUser(chatID, targetUserID int64) {
	packChatID := b.adminPackChatID()
	ml, err := b.db.GetMessageLogAnyState(targetUserID, packChatID)
	if err != nil || ml == nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Пользователь не найден."))
		return
	}
	if ml.IsDeleted {
		b.api.Send(tgbotapi.NewMessage(chatID, "ℹ️ Пользователь уже удалён из стаи."))
		b.showAdminUserCard(chatID, targetUserID)
		return
	}
	username := ml.Username
	if username == "" {
		username = fmt.Sprintf("User%d", targetUserID)
	}
	b.removeUser(targetUserID, packChatID, username)
	b.api.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Пользователь %d удалён из стаи.", targetUserID)))
}

func (b *Bot) adminDeleteMessageByRef(ref string) (bool, string) {
	packChatID := b.adminPackChatID()
	ref = strings.TrimSpace(ref)
	if packChatID == 0 || ref == "" {
		return false, ""
	}

	lower := strings.ToLower(ref)
	switch {
	case strings.HasPrefix(lower, "t"):
		id, err := strconv.ParseInt(ref[1:], 10, 64)
		if err != nil || id <= 0 {
			return false, ""
		}
		ok, err := b.db.AdminDeleteTrainingFeedThreadReply(packChatID, id)
		if err != nil || !ok {
			return false, ""
		}
		return true, fmt.Sprintf("комментарий t%d", id)

	case strings.HasPrefix(lower, "g"):
		id, err := strconv.ParseInt(ref[1:], 10, 64)
		if err != nil || id <= 0 {
			return false, ""
		}
		ok, err := b.db.AdminDeletePackGroupMessage(packChatID, id)
		if err != nil || !ok {
			return false, ""
		}
		return true, fmt.Sprintf("сообщение g%d", id)

	default:
		idStr := strings.TrimPrefix(lower, "#")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			return false, ""
		}
		ok, err := b.db.AdminDeleteFeedUserMessage(packChatID, id)
		if err != nil || !ok {
			return false, ""
		}
		return true, fmt.Sprintf("пост #%d", id)
	}
}

func (b *Bot) adminDeleteReportedContent(chatID, reportID int64) {
	if b == nil || b.db == nil {
		return
	}
	packChatID := b.adminPackChatID()
	item, err := b.db.GetMiniappFeedReport(packChatID, reportID)
	if err != nil || item == nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Жалоба не найдена."))
		return
	}

	var ok bool
	var label string
	switch item.TargetType {
	case "pack_group_message":
		ok, err = b.db.AdminDeletePackGroupMessage(packChatID, item.UserMessageID)
		label = fmt.Sprintf("сообщение чата #%d", item.UserMessageID)
	case "thread_reply":
		ok, err = b.db.AdminDeleteTrainingFeedThreadReply(packChatID, item.ThreadReplyID)
		label = fmt.Sprintf("комментарий t%d", item.ThreadReplyID)
	default:
		ok, err = b.db.AdminDeleteFeedUserMessage(packChatID, item.UserMessageID)
		label = fmt.Sprintf("пост #%d", item.UserMessageID)
	}
	if err != nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка: "+err.Error()))
		return
	}
	if !ok {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Контент уже удалён или не найден."))
		return
	}
	_, _ = b.db.DismissMiniappFeedReport(packChatID, reportID)
	b.api.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Удалено без пометки: %s. Жалоба #%d закрыта.", label, reportID)))
	b.showAdminFeedReportsInbox(chatID)
}

func (b *Bot) adminHideReportedContent(chatID, reportID int64) {
	if b == nil || b.db == nil {
		return
	}
	packChatID := b.adminPackChatID()
	item, err := b.db.GetMiniappFeedReport(packChatID, reportID)
	if err != nil || item == nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Жалоба не найдена."))
		return
	}

	var ok bool
	var label string
	switch item.TargetType {
	case "pack_group_message":
		ok, err = b.db.AdminHidePackGroupMessage(packChatID, item.UserMessageID)
		label = fmt.Sprintf("сообщение чата #%d", item.UserMessageID)
	case "thread_reply":
		ok, err = b.db.AdminHideTrainingFeedThreadReply(packChatID, item.ThreadReplyID)
		label = fmt.Sprintf("комментарий t%d", item.ThreadReplyID)
	default:
		ok, err = b.db.AdminHideFeedUserMessage(packChatID, item.UserMessageID)
		label = fmt.Sprintf("пост #%d", item.UserMessageID)
	}
	if err != nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка: "+err.Error()))
		return
	}
	if !ok {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Контент уже скрыт или не найден."))
		return
	}
	if item.TargetUserID > 0 {
		count := b.recordUGCViolation(item.TargetUserID, packChatID, false)
		b.api.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf(
			"🙈 Скрыто: %s. Нарушений у автора: %d. Жалоба #%d закрыта.",
			label, count, reportID,
		)))
	} else {
		b.api.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("🙈 Скрыто: %s. Жалоба #%d закрыта.", label, reportID)))
	}
	_, _ = b.db.DismissMiniappFeedReport(packChatID, reportID)
	b.showAdminFeedReportsInbox(chatID)
}

func (b *Bot) adminMuteReportedUser(chatID, reportID int64) {
	if b == nil || b.db == nil {
		return
	}
	packChatID := b.adminPackChatID()
	item, err := b.db.GetMiniappFeedReport(packChatID, reportID)
	if err != nil || item == nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Жалоба не найдена."))
		return
	}
	if item.TargetUserID <= 0 {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Нет пользователя для мьюта."))
		return
	}
	b.adminMuteUserUGC(chatID, item.TargetUserID, packChatID, 24)
	_, _ = b.db.DismissMiniappFeedReport(packChatID, reportID)
	b.showAdminFeedReportsInbox(chatID)
}

package bot

import (
	"fmt"
	"strings"
	"time"

	"leo-bot/internal/ai"
	"leo-bot/internal/domain"
	"leo-bot/internal/game/leopardmoney"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) generateShortLeopardChatAck(username, text string, streak, totalXP, ach int) string {
	fallback := "Красавчег, сегодня не съем тебя."
	if b.aiClient == nil {
		return fallback
	}

	question := "Сгенерируй ОДНО короткое предложение в стиле Лео: 5-7 слов, по-доброму хищно, с посылом 'сегодня не съем тебя'. Пиши только как прямую реплику Лео к пользователю: обращение на 'ты', без третьего лица, без ремарок/описаний действий (например, 'улыбнулся', 'подумал', 'прорычал'), без кавычек, без скобок, без Markdown и без эмодзи. Верни только чистый текст одной фразы."
	var ctxBuilder strings.Builder
	ctxBuilder.WriteString("Контекст отчёта тренировки.\n")
	ctxBuilder.WriteString(fmt.Sprintf("Пользователь: %s\n", username))
	ctxBuilder.WriteString(fmt.Sprintf("Серия: %d дней\n", streak))
	ctxBuilder.WriteString(fmt.Sprintf("XP: %d\n", totalXP))
	ctxBuilder.WriteString(fmt.Sprintf("Ачивки: %d\n", ach))
	ctxBuilder.WriteString(fmt.Sprintf("Текст отчёта: %s\n", text))

	ack, err := b.aiClient.AnswerUserQuestion(question, ctxBuilder.String())
	if err != nil {
		b.logger.Warnf("generate short leopard ack: %v", err)
		return fallback
	}
	ack = ai.SanitizeTextForUser(ack)
	ack = strings.Trim(ack, `"'«»“”„`)
	if ack == "" {
		return fallback
	}
	words := len(strings.Fields(ack))
	if words < 3 || words > 12 {
		// Страхуем длину, если модель нарушила ограничение.
		return fallback
	}
	lower := strings.ToLower(ack)
	if strings.Contains(lower, " улыб") ||
		strings.Contains(lower, " подум") ||
		strings.Contains(lower, " прорыч") ||
		strings.Contains(lower, " сказал") ||
		strings.Contains(lower, " произн") {
		return fallback
	}
	return ack
}

// leoFeedEncouragementPol — как обращаться в финале «Держи ритм, …».
func leoFeedEncouragementPol(g string) string {
	g = strings.TrimSpace(strings.ToLower(g))
	if g == "f" {
		return "хищница"
	}
	if g == "m" {
		return "хищник"
	}
	return "" // нейтрально, без муж/жен форм
}

// collapseSpacesKeepParagraphs сжимает пробелы внутри строк, сохраняет один \n\n между абзацами.
func collapseSpacesKeepParagraphs(s string) string {
	s = strings.TrimSpace(s)
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.Join(strings.Fields(line), " ")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// generateLeoTrainingFeedEncouragement — «живой» комментарий Лео для треда ленты (как в тестовом тоне: забота, ирония, «завтрак», «держи ритм»).
// gapEmptyDays — полных дней «тишины» между прошлой датой тренировки и сегодня (без цифр в тексте, только настроение).
func (b *Bot) generateLeoTrainingFeedEncouragement(
	username, reportText string,
	newStreak, totalXP, ach, gapEmptyDays int,
	userGender string,
	wasOnSickLeave bool,
	profileName string,
	profileAge *int,
) string {
	if b.aiClient == nil {
		return ""
	}
	pol := leoFeedEncouragementPol(userGender)
	polHint := "используй нейтральное окончание вроде «Держи ритм!» или «в атаку!»"
	if pol != "" {
		polHint = fmt.Sprintf("в последней фразе можно обернуться: «хищница»/«хищник» — для этого пользователя: «%s» (если нейтрален — без этих слов, нейтрально).", pol)
	}
	verbHint := "Пол не задан: избегай глаголов в прошедшем времени с окончаниями м/ж (лучше нейтральные фразы: «серьёзная сессия», «тренировка прошла здорово») или обращайся нейтрально к «ты»."
	if userGender == "m" {
		verbHint = "Пол в профиле: мужской — прошедшее время / причастия: качал, сделал, доволен (не «качала»)."
	} else if userGender == "f" {
		verbHint = "Пол в профиле: женский — прошедшее время: качала, сделала (не «качал»)."
	}
	gapHint := "перерыв короткий или подряд идёшь — не разгоняйся про 'пропал', шути мягко про дисциплину, не фиксируй дни числом."
	if gapEmptyDays >= 2 {
		gapHint = "был ощутимый перерыв — можно слабую заботу о 'форме', лёгкую нотку, что 'уже волновался', осторожно пошутить про 'завтрак' / перепутать с пропитанием (как в стиле Fat Leopard), без жести и токсичности. Без конкретной цифры дней."
	}
	sickHint := ""
	if wasOnSickLeave {
		sickHint = "пользователь недавно снимал #sick_leave — можно тёплую нотку 'рад что снова в строю', без морали."
	}

	question := `Ты Лео — Fat Leopard, голос ленты стаи. Напиши лаконичный, ЖИВОЙ ответ (не сухой отчёт).

СТРУКТУРА:
1) Блок 1–4 предложений: отреагируй на суть тренировки (из отчёта) + (если уместно по подсказке про перерыв/болезнь) — тёплая ирония, «я волновался за твою форму», осторожная шутка про «завтрак»/перепутать с едой — по желанию, не в каждом ответе.
2) Пустая строка.
3) Короткий финал отдельной строкой, с энергией: например «Держи ритм, хищница!» с правильной формой (см. подсказку по полу) или нейтрально.

СОГЛАСОВАНИЕ: строго следуй подсказке про глаголы (м/ж/нейтрально) из контекста.

ЗАПРЕТЫ: не пиши цифры XP, серий, ачивок, таймер; не *рычит*; без Markdown, без нумерованных списков, без кавычек вокруг всего текста. Русский язык. Можно 0–2 эмодзи (не больше).`

	var ctxBuilder strings.Builder
	ctxBuilder.WriteString("Контекст.\n")
	ctxBuilder.WriteString(fmt.Sprintf("Пользователь: %s\n", username))
	if strings.TrimSpace(profileName) != "" {
		ctxBuilder.WriteString(fmt.Sprintf("Имя в мини-аппе: %s\n", strings.TrimSpace(profileName)))
	}
	if profileAge != nil && *profileAge > 0 {
		ctxBuilder.WriteString(fmt.Sprintf("Возраст (для нюанса, не произноси числом, если неловко): %d\n", *profileAge))
	}
	ctxBuilder.WriteString(verbHint + "\n")
	ctxBuilder.WriteString(fmt.Sprintf("Настроение по данным: серия сейчас %d, XP всего %d, ачивок %d — в тексте НЕ произноси и не обсуждай числа (ниже в приложении дубль статов).\n", newStreak, totalXP, ach))
	ctxBuilder.WriteString(fmt.Sprintf("Полных дней без тренировок между прошлой датой отчёта и сегодня (только логика; в ответе числом дни НЕ пиши): %d\n", gapEmptyDays))
	ctxBuilder.WriteString(gapHint + "\n")
	ctxBuilder.WriteString(sickHint + "\n")
	ctxBuilder.WriteString(polHint + "\n")
	ctxBuilder.WriteString(fmt.Sprintf("Текст отчёта: %s\n", reportText))

	enc, err := b.aiClient.AnswerUserQuestion(question, ctxBuilder.String())
	if err != nil {
		b.logger.Warnf("generate leo feed encouragement: %v", err)
		return ""
	}
	enc = ai.SanitizeTextForUser(enc)
	enc = strings.Trim(enc, `"'«»“”„`)
	enc = collapseSpacesKeepParagraphs(enc)
	if enc == "" {
		return ""
	}
	words := len(strings.Fields(enc))
	if words < 10 || words > 110 {
		return ""
	}
	if len([]rune(enc)) > 620 {
		enc = string([]rune(enc)[:620]) + "…"
	}
	return enc
}

// handleLeopardMoneyTrainingDone — отчёт #training_done по модели Leopard Money (XP, ачивки, таймер 8 дней).
// personalReplyCh, если задан, получает тот же текст, что и личка с итогом (для Mini App).
// trainingUserMessageID — id строки user_messages с этим отчётом; для ленты мини-аппа подтягиваем ответ Лео в тред (только отчёт из чата стаи).
func (b *Bot) handleLeopardMoneyTrainingDone(msg *tgbotapi.Message, personalReplyCh chan<- string, trainingUserMessageID int64) {
	username := ""
	if msg.From.UserName != "" {
		username = "@" + msg.From.UserName
	} else if msg.From.FirstName != "" {
		username = msg.From.FirstName
		if msg.From.LastName != "" {
			username += " " + msg.From.LastName
		}
	} else {
		username = fmt.Sprintf("User%d", msg.From.ID)
	}

	b.startTimer(msg.From.ID, b.kickChatIDForMessage(msg), username)

	messageLog, err := b.db.GetMessageLog(msg.From.ID, msg.Chat.ID)
	if err != nil {
		b.logger.Errorf("Failed to get message log: %v", err)
		return
	}

	// Approval-стейт больничного теперь живёт на pack-row (см. handleSickLeave),
	// поэтому отменяем его именно там — иначе watcher продержится до дедлайна и потом
	// принудительно «отменит» уже неактуальный запрос лишним сообщением.
	packChatID := b.kickChatIDForMessage(msg)
	if packLog, packErr := b.db.GetMessageLog(msg.From.ID, packChatID); packErr == nil && packLog != nil && packLog.SickApprovalPending {
		b.cancelSickApprovalWatcher(msg.From.ID)
		packLog.SickApprovalPending = false
		packLog.SickApprovalDeadline = nil
		packLog.SickApprovalMessageID = nil
		if err := b.db.SaveMessageLog(packLog); err != nil {
			b.logger.Errorf("Failed to clear pack-row sick approval flags after training: %v", err)
		}
	}

	if messageLog.SickApprovalPending {
		b.cancelSickApprovalWatcher(msg.From.ID)
		messageLog.SickApprovalPending = false
		messageLog.SickApprovalDeadline = nil
		messageLog.SickApprovalMessageID = nil
		if err := b.db.SaveMessageLog(messageLog); err != nil {
			b.logger.Errorf("Failed to clear sick approval flags after training: %v", err)
		}
	}

	userGender := b.LeoUserGenderForTrainingFeed(msg.From.ID, messageLog.Gender)

	text := msg.Text
	if text == "" && msg.Caption != "" {
		text = msg.Caption
	}

	localNow := b.getUserLocalNow(messageLog.TimezoneOffsetFromMoscow)
	today := localNow.Format("2006-01-02")

	gapEmptyDays := 0
	if messageLog.LastTrainingDate != nil && *messageLog.LastTrainingDate != today {
		lastD, err := time.Parse("2006-01-02", *messageLog.LastTrainingDate)
		if err == nil {
			todayD, err2 := time.Parse("2006-01-02", today)
			if err2 == nil {
				d := int(todayD.Sub(lastD) / (24 * time.Hour))
				if d > 1 {
					gapEmptyDays = d - 1
				}
			}
		}
	}

	sessionsToday, err := b.db.CountTrainingSessionsInDateRange(msg.From.ID, msg.Chat.ID, today, today)
	if err != nil {
		b.logger.Errorf("sessions today: %v", err)
		sessionsToday = 0
	}
	firstReportToday := sessionsToday == 0

	xpAdd := 0
	if firstReportToday {
		xpAdd = leopardmoney.XPPerActiveDay
	}

	newStreak := 1
	if messageLog.LastTrainingDate != nil && *messageLog.LastTrainingDate == today {
		newStreak = messageLog.StreakDays
	} else if messageLog.LastTrainingDate != nil {
		yesterdayStr := localNow.AddDate(0, 0, -1).Format("2006-01-02")
		if *messageLog.LastTrainingDate == yesterdayStr {
			newStreak = messageLog.StreakDays + 1
		} else {
			newStreak = 1
		}
	}

	newCalStreak := 1
	if messageLog.LastTrainingDate != nil && *messageLog.LastTrainingDate == today {
		newCalStreak = messageLog.CalorieStreakDays
	} else if messageLog.LastTrainingDate != nil {
		yesterdayStr := localNow.AddDate(0, 0, -1).Format("2006-01-02")
		if *messageLog.LastTrainingDate == yesterdayStr {
			newCalStreak = messageLog.CalorieStreakDays + 1
		}
	}

	if xpAdd > 0 {
		if err := b.db.AddXP(msg.From.ID, msg.Chat.ID, xpAdd); err != nil {
			b.logger.Errorf("add XP: %v", err)
		}
	}

	if err := b.db.UpdateStreak(msg.From.ID, msg.Chat.ID, newStreak, today); err != nil {
		b.logger.Errorf("update streak: %v", err)
	}
	if err := b.db.UpdateCalorieStreakWithDate(msg.From.ID, msg.Chat.ID, newCalStreak, today); err != nil {
		b.logger.Errorf("update cal streak: %v", err)
	}

	msgLog2, _ := b.db.GetMessageLog(msg.From.ID, msg.Chat.ID)
	if msgLog2 != nil && newStreak > 0 && newStreak%7 == 0 && newStreak <= 28 {
		want := newStreak / 7
		if msgLog2.AchievementCount < want && want <= leopardmoney.MaxAchievements {
			msgLog2.AchievementCount = want
			msgLog2.LastAchievementStreakLevel = newStreak
			_ = b.db.SaveMessageLog(msgLog2)
		}
	}

	totalXP, _ := b.db.GetUserXP(msg.From.ID, msg.Chat.ID)
	ach := 0
	if ml, e := b.db.GetMessageLog(msg.From.ID, msg.Chat.ID); e == nil {
		ach = ml.AchievementCount
	}

	tag := "#training_done"

	session := &domain.TrainingSession{
		UserID:         msg.From.ID,
		ChatID:         msg.Chat.ID,
		SessionDate:    today,
		MessageText:    text,
		TrainingsCount: 1,
		CupsAdded:      0,
		IsBonus:        false,
	}
	if err := b.db.SaveTrainingSession(session); err != nil {
		b.logger.Errorf("SaveTrainingSession: %v", err)
	}

	wasOnSickLeave := messageLog.HasSickLeave && !messageLog.HasHealthy

	tagPrefix := ""
	if msg.From != nil {
		if msg.From.UserName != "" {
			tagPrefix = "@" + msg.From.UserName + ", "
		} else {
			displayName := strings.TrimSpace(msg.From.FirstName)
			if displayName == "" {
				displayName = "дружище"
			}
			tagPrefix = fmt.Sprintf("[%s](tg://user?id=%d), ", displayName, msg.From.ID)
		}
	}
	// chatAck — короткая «съем/не съем» реплика. В mini-app-флоу её показывать в TG не нужно
	// (юзер видит подтверждение в мини-аппе через messageText ниже), иначе получится дубль
	// в TG-личке: «Не дублировать в ТГ из мини-аппа» (см. требование пользователя).
	if personalReplyCh == nil {
		chatAckText := tagPrefix + b.generateShortLeopardChatAck(username, text, newStreak, totalXP, ach)
		chatAck := tgbotapi.NewMessage(msg.Chat.ID, chatAckText)
		chatAck.ParseMode = "Markdown"
		if _, err := b.api.Send(chatAck); err != nil {
			b.logger.Errorf("send training chat ack: %v", err)
		}
	}

	messageText := fmt.Sprintf("✅ Отчёт принят! 💪\n\n🦁 Серия: %d дн.\n⚡ +%d XP (всего XP: %d)\n🏆 Ачивок: %d/%d\n\n⏰ Таймер неактивности: %d дней (день 8 — удаление)\n\n🎯 Отчёт с %s", newStreak, xpAdd, totalXP, ach, leopardmoney.MaxAchievements, leopardmoney.InactiveRemovalDays, tag)

	if personalReplyCh != nil {
		// Mini-app: только в очередь приложения, в TG-личку НЕ дублируем.
		select {
		case personalReplyCh <- messageText:
		default:
		}
	} else {
		privateReply := tgbotapi.NewMessage(msg.From.ID, messageText)
		if _, err := b.api.Send(privateReply); err != nil {
			b.logger.Warnf("send training private summary: %v", err)
		}
	}

	if trainingUserMessageID > 0 && b.config.MonetizedChatID != 0 {
		threadText := messageText
		profName, profAge := b.LeoUserProfileForFeedPrompt(msg.From.ID)
		if extra := b.generateLeoTrainingFeedEncouragement(
			username, text, newStreak, totalXP, ach, gapEmptyDays, userGender, wasOnSickLeave, profName, profAge,
		); extra != "" {
			threadText = extra + "\n\n" + messageText
		}
		if _, err := b.db.InsertTrainingFeedThreadReply(b.config.MonetizedChatID, trainingUserMessageID, 0, "Лео", threadText); err != nil {
			b.logger.Warnf("training feed leo thread reply: %v", err)
		}
	}

}

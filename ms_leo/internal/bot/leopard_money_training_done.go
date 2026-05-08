package bot

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"leo-bot/internal/ai"
	"leo-bot/internal/domain"
	"leo-bot/internal/game/leopardmoney"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) generateShortLeopardChatAck(username, text string, streak, totalCups, ach int) string {
	fallback := "Красавчег, сегодня не съем тебя."
	if b.aiClient == nil {
		return fallback
	}

	question := "Сгенерируй ОДНО короткое предложение в стиле Лео: 5-7 слов, по-доброму хищно, с посылом 'сегодня не съем тебя'. Пиши только как прямую реплику Лео к пользователю: обращение на 'ты', без третьего лица, без ремарок/описаний действий (например, 'улыбнулся', 'подумал', 'прорычал'), без кавычек, без скобок, без Markdown и без эмодзи. Верни только чистый текст одной фразы."
	var ctxBuilder strings.Builder
	ctxBuilder.WriteString("Контекст отчёта тренировки.\n")
	ctxBuilder.WriteString(fmt.Sprintf("Пользователь: %s\n", username))
	ctxBuilder.WriteString(fmt.Sprintf("Серия: %d %s\n", streak, daysWordForm(streak)))
	ctxBuilder.WriteString(fmt.Sprintf("Кубки всего: %d\n", totalCups))
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

// generateLeoTrainingFeedEncouragement — «живой» комментарий Лео для треда ленты: забота, лёгкая ирония, финал вроде «Держи ритм!» (без метафор еды / «завтрака»).
// gapEmptyDays — полных дней «тишины» между прошлой датой тренировки и сегодня (без цифр в тексте, только настроение).
func (b *Bot) generateLeoTrainingFeedEncouragement(
	username, reportText string,
	newStreak, totalCups, ach, gapEmptyDays int,
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
		gapHint = "был ощутимый перерыв — тёплая забота о форме, радость что снова в строю; лёгкая самоирония Лео допустима, но без шуток про еду, завтрак, поедание и без угроз. Без конкретной цифры дней."
	}
	sickHint := ""
	if wasOnSickLeave {
		sickHint = "пользователь недавно снимал #sick_leave — можно тёплую нотку 'рад что снова в строю', без морали."
	}

	question := `Ты Лео — Fat Leopard, голос ленты стаи. Напиши лаконичный, ЖИВОЙ ответ (не сухой отчёт).

СТРУКТУРА:
1) Блок 1–4 предложений: отреагируй на суть тренировки (из отчёта) и при необходимости на подсказки про перерыв/болезнь — тёплая ирония и поддержка, без метафор еды, завтрака, «съесть», «перекусить» и без сравнений пользователя с едой.
2) Пустая строка.
3) Короткий финал отдельной строкой, с энергией: например «Держи ритм, хищница!» с правильной формой (см. подсказку по полу) или нейтрально.

СОГЛАСОВАНИЕ: строго следуй подсказке про глаголы (м/ж/нейтрально) из контекста.

ЗАПРЕТЫ: не пиши цифры кубков, серий, ачивок, таймер; не *рычит*; без Markdown, без нумерованных списков, без кавычек вокруг всего текста. Русский язык. Можно 0–2 эмодзи (не больше).`

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
	ctxBuilder.WriteString(fmt.Sprintf("Настроение по данным: серия сейчас %d, кубков всего %d, ачивок %d — в тексте НЕ произноси и не обсуждай числа (ниже в приложении дубль статов).\n", newStreak, totalCups, ach))
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

// ruCupsWord — «1 кубок», «2 кубка», «5 кубков» (только для текста подтверждения отчёта).
func ruCupsWord(n int) string {
	if n < 0 {
		n = -n
	}
	m := n % 100
	if m >= 11 && m <= 14 {
		return "кубков"
	}
	switch n % 10 {
	case 1:
		return "кубок"
	case 2, 3, 4:
		return "кубка"
	default:
		return "кубков"
	}
}

// handleLeopardMoneyTrainingDone — отчёт #training_done по модели Leopard Money (кубки по формуле, ачивки, таймер 8 дней).
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

	packChatID := b.kickChatIDForMessage(msg)
	b.startTimer(msg.From.ID, packChatID, username)

	messageLog, err := b.db.GetMessageLog(msg.From.ID, packChatID)
	if err != nil {
		b.logger.Errorf("Failed to get message log: %v", err)
		return
	}

	// Approval-стейт больничного теперь живёт на pack-row (см. handleSickLeave),
	// поэтому отменяем его именно там — иначе watcher продержится до дедлайна и потом
	// принудительно «отменит» уже неактуальный запрос лишним сообщением.
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

	cupsAdd := leopardmoney.TrainingCupsFromReportText(text)
	if err := b.db.AddCups(msg.From.ID, packChatID, cupsAdd); err != nil {
		b.logger.Errorf("add cups: %v", err)
	}

	if err := b.db.UpdateStreak(msg.From.ID, packChatID, newStreak, today); err != nil {
		b.logger.Errorf("update streak: %v", err)
	}

	achievementAwarded := false
	msgLog2, _ := b.db.GetMessageLog(msg.From.ID, packChatID)
	if msgLog2 != nil {
		if idx := leopardmoney.StreakAchievementIndex(newStreak); idx >= 0 {
			want := idx + 1
			if msgLog2.AchievementCount < want && want <= leopardmoney.MaxAchievements {
				msgLog2.AchievementCount = want
				msgLog2.LastAchievementStreakLevel = newStreak
				_ = b.db.SaveMessageLog(msgLog2)
				achievementAwarded = true
			}
		}
	}

	totalCups, _ := b.db.GetUserCups(msg.From.ID, packChatID)
	ach := 0
	if ml, e := b.db.GetMessageLog(msg.From.ID, packChatID); e == nil {
		ach = ml.AchievementCount
	}

	tag := "#training_done"

	session := &domain.TrainingSession{
		UserID:         msg.From.ID,
		ChatID:         packChatID,
		SessionDate:    today,
		MessageText:    text,
		TrainingsCount: 1,
		CupsAdded:      cupsAdd,
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
		chatAckText := tagPrefix + b.generateShortLeopardChatAck(username, text, newStreak, totalCups, ach)
		chatAck := tgbotapi.NewMessage(msg.Chat.ID, chatAckText)
		chatAck.ParseMode = "Markdown"
		if _, err := b.api.Send(chatAck); err != nil {
			b.logger.Errorf("send training chat ack: %v", err)
		}
	}

	statsBlock := fmt.Sprintf(
		"✅ Отчёт принят! 💪\n\n"+
			"🦁 Серия: %d %s\n"+
			"🏆 +%d %s (всего: %d)\n"+
			"🎖 Ачивок: %d/%d",
		newStreak, daysWordForm(newStreak),
		cupsAdd, ruCupsWord(cupsAdd), totalCups,
		ach, leopardmoney.MaxAchievements,
	)
	inactiveBlock := "⏰ Неактивность: 8 дней без отчёта — удаление в 00:00 вашего часового пояса; предупреждения на 5-й, 6-й и 7-й день."
	tail := fmt.Sprintf("🎯 Отчёт с %s", tag)
	messageTextMiniapp := statsBlock + "\n\n" + tail
	messageTextPrivate := statsBlock + "\n\n" + inactiveBlock + "\n\n" + tail

	if personalReplyCh != nil {
		// Mini-app: без блока про неактивность (он только в личке); коммент Лео в ленте — позже, асинхронно.
		select {
		case personalReplyCh <- messageTextMiniapp:
		default:
		}
	} else {
		privateReply := tgbotapi.NewMessage(msg.From.ID, messageTextPrivate)
		if _, err := b.api.Send(privateReply); err != nil {
			b.logger.Warnf("send training private summary: %v", err)
		}
	}

	if achievementAwarded && b.aiClient != nil && b.config.Prompts.AchievementMilestone != "" {
		uid := msg.From.ID
		un := username
		streak := newStreak
		achCount := ach
		isRecord := msgLog2 != nil && streak >= msgLog2.MaxStreakDays
		isFirst := achCount == 1
		levelName := leopardmoney.LevelName(leopardmoney.LevelFromTotalCups(int(totalCups)))
		go func() {
			tier := 1
			for i, m := range leopardmoney.StreakAchievementMilestones {
				if streak >= m {
					tier = i + 1
				}
			}
			ctx := fmt.Sprintf(
				"Пользователь: @%s\nСтрик: %d дней (milestone)\nТier: %d\nУровень: %s\nis_record: %v\nis_first_achievement: %v",
				strings.TrimPrefix(un, "@"), streak, tier, levelName, isRecord, isFirst,
			)
			raw, err := b.aiClient.AnswerUserQuestion(b.config.Prompts.AchievementMilestone, ctx)
			if err != nil {
				b.logger.Warnf("achievement milestone AI: %v", err)
				return
			}
			raw = ai.SanitizeTextForUser(raw)
			var resp struct {
				InAppText  string `json:"in_app_text"`
				LeoMessage string `json:"leo_message"`
			}
			if err := json.Unmarshal([]byte(raw), &resp); err != nil {
				b.logger.Warnf("achievement milestone parse: %v", err)
				return
			}
			leoMsg := strings.TrimSpace(resp.LeoMessage)
			if leoMsg == "" {
				return
			}
			b.miniappPersonalPush(uid, "🏅 "+leoMsg)
		}()
	}

	if trainingUserMessageID > 0 && b.config.MonetizedChatID != 0 {
		uid := msg.From.ID
		packID := b.config.MonetizedChatID
		un := username
		txt := text
		ns, tc, a := newStreak, totalCups, ach
		gap := gapEmptyDays
		ug := userGender
		was := wasOnSickLeave
		tid := trainingUserMessageID
		go func() {
			threadText := ""
			profName, profAge := b.LeoUserProfileForFeedPrompt(uid)
			if extra := b.generateLeoTrainingFeedEncouragement(
				un, txt, ns, tc, a, gap, ug, was, profName, profAge,
			); extra != "" {
				threadText = extra
			}
			if strings.TrimSpace(threadText) == "" {
				threadText = b.generateShortLeopardChatAck(un, txt, ns, tc, a)
			}
			if _, err := b.db.InsertTrainingFeedThreadReply(packID, tid, 0, "Лео", threadText, 0); err != nil {
				b.logger.Warnf("training feed leo thread reply: %v", err)
			}
		}()
	}

}

// LeoBanterReplyToUserTrainingFeedThread — ответ Лео в треде под отчётом, когда пользователь ответил на сообщение Лео (reply).
// Вызывается асинхронно из PackTrainingFeedThreadPost; не шлёт личку, только строка в miniapp_training_feed_thread.
func (b *Bot) LeoBanterReplyToUserTrainingFeedThread(
	packChatID, trainingUserMessageID int64,
	userThreadReplyRowID int64,
	viewerTelegramUserID int64,
	userReplyText, leoMessageBeingRepliedTo string,
) {
	if b == nil || b.db == nil || b.aiClient == nil {
		return
	}
	reportText := ""
	if t, err := b.db.GetUserMessageTextByIDForChat(trainingUserMessageID, packChatID); err == nil {
		reportText = t
	}
	profName, _ := b.LeoUserProfileForFeedPrompt(viewerTelegramUserID)
	leoCtx := truncateForDM(leoMessageBeingRepliedTo, 1400)
	userCtx := truncateForDM(userReplyText, 1400)
	reportCtx := truncateForDM(reportText, 900)

	qb := strings.Builder{}
	qb.WriteString("Ты Лео — Fat Leopard. Пользователь ответил на ТВОЁ сообщение в комментариях под его отчётом #training_done в ленте стаи (мини-апп).\n\n")
	qb.WriteString("Твоё сообщение, на которое он ответил:\n")
	qb.WriteString(leoCtx)
	qb.WriteString("\n\nЕго реплика тебе:\n")
	qb.WriteString(userCtx)
	qb.WriteString("\n\nОтветь ему 1–4 короткими предложениями в том же тоне: остроумно, по-хищному по-дружески, можно лёгкую иронию. Реагируй на его слова, продолжай диалог — не пересказывай длинный отчёт ниже целиком.\n")
	qb.WriteString("Без Markdown, без нумерации списков. Эмодзи — не больше двух на весь ответ. Без мета («как языковая модель»).\n")
	if strings.TrimSpace(profName) != "" {
		qb.WriteString("Имя из профиля (если уместно в обращении): " + strings.TrimSpace(profName) + "\n")
	}

	ctxBody := strings.Builder{}
	ctxBody.WriteString("Выдержка из текста отчёта пользователя (контекст, не цитируй дословно целиком):\n")
	ctxBody.WriteString(reportCtx)

	reply, err := b.aiClient.AnswerUserQuestion(qb.String(), ctxBody.String())
	if err != nil {
		b.logger.Warnf("leo training feed thread banter AI: %v", err)
		return
	}
	reply = ai.SanitizeTextForUser(reply)
	reply = strings.TrimSpace(strings.Trim(reply, "\"'«»“”„"))
	if utf8.RuneCountInString(reply) < 2 {
		return
	}
	if utf8.RuneCountInString(reply) > 900 {
		r := []rune(reply)
		reply = string(r[:900]) + "…"
	}
	if _, err := b.db.InsertTrainingFeedThreadReply(packChatID, trainingUserMessageID, 0, "Лео", reply, userThreadReplyRowID); err != nil {
		b.logger.Warnf("leo training feed thread banter insert: %v", err)
	}
}

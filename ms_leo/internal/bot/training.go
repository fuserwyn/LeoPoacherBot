package bot

import (
	"fmt"
	"strings"

	"leo-bot/internal/domain"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// calculateTrainingDayOutcome описывает, что произойдёт при следующем засчитанном отчёте #training_done
// в этот календарный день пользователя: первая тренировка дня — начисления и обновление серии;
// повтор в тот же день — без начислений (earnRewards=false).
func (b *Bot) calculateTrainingDayOutcome(messageLog *domain.MessageLog) (earnRewards bool, newStreakDays int, weeklyAchievement, twoWeekAchievement, threeWeekAchievement, monthlyAchievement, fortyTwoDayAchievement, fiftyDayAchievement, sixtyDayAchievement, quarterlyAchievement, hundredDayAchievement, oneHundredEightyDayAchievement, twoHundredDayAchievement, twoHundredFortyDayAchievement bool) {
	localNow := b.getUserLocalNow(messageLog.TimezoneOffsetFromMoscow)
	today := localNow.Format("2006-01-02")

	if messageLog.LastTrainingDate != nil && *messageLog.LastTrainingDate == today {
		return false, messageLog.StreakDays, false, false, false, false, false, false, false, false, false, false, false, false
	}

	newStreakDays = 1
	if messageLog.LastTrainingDate != nil {
		yesterdayStr := localNow.AddDate(0, 0, -1).Format("2006-01-02")
		if *messageLog.LastTrainingDate == yesterdayStr {
			newStreakDays = messageLog.StreakDays + 1
		} else {
			newStreakDays = 1
		}
	} else if messageLog.StreakDays > 0 {
		newStreakDays = messageLog.StreakDays + 1
	}

	weeklyAchievement = newStreakDays == 7
	twoWeekAchievement = newStreakDays == 14
	threeWeekAchievement = newStreakDays == 21
	monthlyAchievement = newStreakDays == 30
	fortyTwoDayAchievement = newStreakDays == 42
	fiftyDayAchievement = newStreakDays == 50
	sixtyDayAchievement = newStreakDays == 60
	quarterlyAchievement = newStreakDays == 90
	hundredDayAchievement = newStreakDays == 100
	oneHundredEightyDayAchievement = newStreakDays == 180
	twoHundredDayAchievement = newStreakDays == 200
	twoHundredFortyDayAchievement = newStreakDays == 240

	return true, newStreakDays, weeklyAchievement, twoWeekAchievement, threeWeekAchievement, monthlyAchievement, fortyTwoDayAchievement, fiftyDayAchievement, sixtyDayAchievement, quarterlyAchievement, hundredDayAchievement, oneHundredEightyDayAchievement, twoHundredDayAchievement, twoHundredFortyDayAchievement
}

func (b *Bot) sendStreakReward(
	msg *tgbotapi.Message,
	username string,
	streakDays int,
	rewardCups int,
	title string,
	subtitle string,
) {
	totalCups, err := b.db.GetUserCups(msg.From.ID, b.packTrainingStateChatID(msg))
	if err != nil {
		b.logger.Errorf("Failed to get total cups for %s reward: %v", title, err)
		totalCups = 0
	}

	messageText := fmt.Sprintf(`%s!

%s, серия: %d %s подряд.
%s

🏆 +%d %s (всего: %d)`,
		title,
		username,
		streakDays,
		daysWordForm(streakDays),
		subtitle,
		rewardCups,
		cupsWordForm(rewardCups),
		totalCups,
	)

	reply := tgbotapi.NewMessage(msg.Chat.ID, messageText)
	if _, err := b.api.Send(reply); err != nil {
		b.logger.Errorf("Failed to send %s reward message: %v", title, err)
	}
}

type GenderForms struct {
	Champion    string // чемпион / чемпионка
	Accumulated string // накопил / накопила
	Transformed string // превратил / превратила
	Became      string // становился / становилась
	Ruler       string // владыка / владычица
	Warrior     string // воин / воительница
	Entered     string // вошёл / вошла
	Proved      string // доказал / доказала
	Deserved    string // заслужил / заслужила
	Titan       string // титан / титанисса
	Olympian    string // олимпийский чемпион / олимпийская чемпионка
	Invincible  string // непобедимый / непобедимая
	Reached     string // достиг / достигла
}

func (b *Bot) getGenderForms(gender string) GenderForms {
	genderNormalized := strings.TrimSpace(strings.ToLower(gender))
	if genderNormalized == "f" {
		return GenderForms{
			Champion:    "чемпионка",
			Accumulated: "накопила",
			Transformed: "превратила",
			Became:      "становилась",
			Ruler:       "владычица",
			Warrior:     "воительница",
			Entered:     "вошла",
			Proved:      "доказала",
			Deserved:    "заслужила",
			Titan:       "титанисса",
			Olympian:    "олимпийская чемпионка",
			Invincible:  "непобедимая",
			Reached:     "достигла",
		}
	}
	return GenderForms{
		Champion:    "чемпион",
		Accumulated: "накопил",
		Transformed: "превратил",
		Became:      "становился",
		Ruler:       "владыка",
		Warrior:     "воин",
		Entered:     "вошёл",
		Proved:      "доказал",
		Deserved:    "заслужил",
		Titan:       "титан",
		Olympian:    "олимпийский чемпион",
		Invincible:  "непобедимый",
		Reached:     "достиг",
	}
}

// getWordForm возвращает правильную форму слова "слов" в зависимости от числа
// 1, 21, 31, 41... → "слово"
// 2, 3, 4, 22, 23, 24, 32, 33, 34... → "слова"
// 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 25, 26... → "слов"
func getWordForm(count int) string {
	// Берем последнюю цифру и предпоследнюю для определения склонения
	lastDigit := count % 10
	secondLastDigit := (count / 10) % 10

	// Если предпоследняя цифра 1 (10-19), всегда "слов"
	if secondLastDigit == 1 {
		return "слов"
	}

	// Иначе смотрим на последнюю цифру
	switch lastDigit {
	case 1:
		return "слово"
	case 2, 3, 4:
		return "слова"
	default:
		return "слов"
	}
}

// russianPlural — одна из форм (ед., неск. 2–4, много) для положительного целого count.
func russianPlural(count int, one, few, many string) string {
	if count < 0 {
		count = -count
	}
	n := count % 100
	if n >= 11 && n <= 14 {
		return many
	}
	switch count % 10 {
	case 1:
		return one
	case 2, 3, 4:
		return few
	default:
		return many
	}
}

func trainingsWordForm(count int) string {
	return russianPlural(count, "тренировка", "тренировки", "тренировок")
}

func cupsWordForm(count int) string {
	return russianPlural(count, "кубок", "кубка", "кубков")
}

func daysWordForm(count int) string {
	return russianPlural(count, "день", "дня", "дней")
}

func (b *Bot) sendWeeklyCupsReward(msg *tgbotapi.Message, username string, streakDays int, userGender string) {
	totalCups, _ := b.db.GetUserCups(msg.From.ID, b.packTrainingStateChatID(msg))
	rewardCups := 42
	forms := b.getGenderForms(userGender)

	messageText := fmt.Sprintf(`🏆🏆🏆 НЕВЕРОЯТНО! 🏆🏆🏆

%s, ты тренируешься уже %d %s подряд! 

🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆

🎯 +%d %s ЗА ТВОЮ СЕРИЮ %d ДНЕЙ! 🎯

🏆 +%d %s
🏆 Всего кубков: %d
🦁 Fat Leopard гордится твоей дисциплиной! 
💪 Ты %s намерение в привычку — это первый шаг к победе!
🔥 Каждый день ты становишься сильнее и выносливее!
⭐ Ты вдохновляешь других начать свой путь!
👑 Ты %s своей воли!
🌟 Продолжай в том же духе — впереди ещё большие достижения!

#seven_days_warrior #42_cups #training_start`,
		username, streakDays, daysWordForm(streakDays), rewardCups, strings.ToUpper(cupsWordForm(rewardCups)), streakDays, rewardCups, cupsWordForm(rewardCups), totalCups, forms.Transformed, forms.Ruler)

	reply := tgbotapi.NewMessage(msg.Chat.ID, messageText)
	b.api.Send(reply)
}

func (b *Bot) sendTwoWeekCupsReward(msg *tgbotapi.Message, username string, streakDays int, userGender string) {
	totalCups, _ := b.db.GetUserCups(msg.From.ID, b.packTrainingStateChatID(msg))
	rewardCups := 42

	forms := b.getGenderForms(userGender)

	messageText := fmt.Sprintf(`🏆🏆🏆🏆 ВПЕЧАТЛЯЮЩЕ! 🏆🏆🏆🏆

%s, ты тренируешься уже %d %s подряд! 

🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆

🎯 +%d %s ЗА ТВОЮ СЕРИЮ %d ДНЕЙ! 🎯

🏆 +%d %s
🏆 Всего кубков: %d
🦁 Fat Leopard видит в тебе настоящую %s! 
💪 Две недели без пропусков — мощный рывок!
🔥 Твоя сила воли растёт с каждым днём!
⭐ Ты уже не просто начинающий — ты на правильном пути!
👑 Каждая тренировка приближает тебя к цели!
🌟 Продолжай в том же духе — ты на пути к величию!

#fourteen_days_champion #42_cups #training_momentum`,
		username, streakDays, daysWordForm(streakDays), rewardCups, strings.ToUpper(cupsWordForm(rewardCups)), streakDays, rewardCups, cupsWordForm(rewardCups), totalCups, forms.Warrior)

	reply := tgbotapi.NewMessage(msg.Chat.ID, messageText)
	b.api.Send(reply)
}

func (b *Bot) sendThreeWeekCupsReward(msg *tgbotapi.Message, username string, streakDays int, userGender string) {
	totalCups, _ := b.db.GetUserCups(msg.From.ID, b.packTrainingStateChatID(msg))
	rewardCups := 42

	forms := b.getGenderForms(userGender)

	messageText := fmt.Sprintf(`🏆🏆🏆🏆🏆 ВЕЛИКОЛЕПНО! 🏆🏆🏆🏆🏆

%s, ты тренируешься уже %d %s подряд! 

🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆

🎯 +%d %s ЗА ТВОЮ СЕРИЮ %d ДНЕЙ! 🎯

🏆 +%d %s
🏆 Всего кубков: %d
🦁 Fat Leopard восхищается твоей дисциплиной! 
💪 Три недели абсолютной преданности — ты %s в топ 5%%!
🔥 Твоя привычка стала частью тебя — это образ жизни!
⭐ Ты %s себе и всем, что можешь всё!
👑 Это уже не усилие — это твоя суть!
🌟 Впереди ещё больше побед — не сбавляй темп!

#twenty_one_days_elite #42_cups #training_lifestyle`,
		username, streakDays, daysWordForm(streakDays), rewardCups, strings.ToUpper(cupsWordForm(rewardCups)), streakDays, rewardCups, cupsWordForm(rewardCups), totalCups, forms.Entered, forms.Proved)

	reply := tgbotapi.NewMessage(msg.Chat.ID, messageText)
	b.api.Send(reply)
}

func (b *Bot) sendMonthlyCupsReward(msg *tgbotapi.Message, username string, streakDays int, userGender string) {
	totalCups, _ := b.db.GetUserCups(msg.From.ID, b.packTrainingStateChatID(msg))
	rewardCups := 420
	forms := b.getGenderForms(userGender)

	messageText := fmt.Sprintf(`🏆🏆🏆🏆🏆🏆 МЕСЯЦ ПОБЕДЫ! 🏆🏆🏆🏆🏆🏆

%s, ты тренируешься уже %d %s подряд! 

🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆

🎯 +%d %s ЗА ТВОЮ СЕРИЮ %d ДНЕЙ! 🎯

🏆 +%d %s
🏆 Всего кубков: %d
🦁 Fat Leopard не может поверить в твою силу воли! 
💪 Ты %s %s дисциплины!
🔥 Месяц абсолютной преданности — это уровень чемпионов!
⭐ Ты не просто тренируешься — ты создаёшь новую версию себя!
👑 Каждый день ты %s лучше, сильнее, увереннее!
🌟 Твоя дисциплина — пример для всех — ты %s!
💎 Ты %s, что можешь достичь ЛЮБОЙ цели — небо не предел!

#thirty_days_champion #420_cups #training_perfection`,
		username, streakDays, daysWordForm(streakDays), rewardCups, strings.ToUpper(cupsWordForm(rewardCups)), streakDays, rewardCups, cupsWordForm(rewardCups), totalCups, forms.Invincible, forms.Titan, forms.Became, forms.Champion, forms.Proved)

	reply := tgbotapi.NewMessage(msg.Chat.ID, messageText)
	b.api.Send(reply)
}

func (b *Bot) sendFortyTwoDayCupsReward(msg *tgbotapi.Message, username string, streakDays int, userGender string) {
	totalCups, _ := b.db.GetUserCups(msg.From.ID, b.packTrainingStateChatID(msg))
	rewardCups := 42
	forms := b.getGenderForms(userGender)

	messageText := fmt.Sprintf(`🏆🏆🏆🏆🏆🏆 ЛЕГЕНДАРНО! 🏆🏆🏆🏆🏆🏆

%s, ты тренируешься уже %d %s подряд! 

🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆

🎯 +%d %s ЗА ТВОЮ СЕРИЮ %d ДНЕЙ! 🎯

🏆 +%d %s
🏆 Всего кубков: %d
🦁 Fat Leopard признаёт тебя символом стаи! 
💪 42 дня — это легендарная отметка чемпионов!
🔥 Шесть недель абсолютной преданности — уровень мастерства!
⭐ Ты не просто тренируешься — ты создаёшь новую версию себя!
👑 Твоя сила воли теперь невероятна — ты это %s!
🌟 Продолжай — ты на пути к легенде!

#forty_two_days_legend #42_cups #training_mastery`,
		username, streakDays, daysWordForm(streakDays), rewardCups, strings.ToUpper(cupsWordForm(rewardCups)), streakDays, rewardCups, cupsWordForm(rewardCups), totalCups, forms.Deserved)

	reply := tgbotapi.NewMessage(msg.Chat.ID, messageText)
	b.api.Send(reply)
}

func (b *Bot) sendFiftyDayCupsReward(msg *tgbotapi.Message, username string, streakDays int, userGender string) {
	totalCups, _ := b.db.GetUserCups(msg.From.ID, b.packTrainingStateChatID(msg))
	rewardCups := 42

	forms := b.getGenderForms(userGender)

	messageText := fmt.Sprintf(`🏆🏆🏆🏆🏆🏆🏆 ЭЛИТА! 🏆🏆🏆🏆🏆🏆🏆

%s, ты тренируешься уже %d %s подряд! 

🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆

🎯 +%d %s ЗА ТВОЮ СЕРИЮ %d ДНЕЙ! 🎯

🏆 +%d %s
🏆 Всего кубков: %d
🦁 Fat Leopard видит в тебе %s! 
💪 Полсотни дней без пропусков — ты %s в элиту!
🔥 50 дней — это доказательство того, что ты можешь всё!
⭐ Твоя дисциплина на уровне лучших спортсменов мира!
👑 Каждая тренировка — инвестиция в себя, и ты видишь результаты!
🌟 Не останавливайся — впереди ещё больше возможностей!

#fifty_days_olympian #42_cups #training_excellence`,
		username, streakDays, daysWordForm(streakDays), rewardCups, strings.ToUpper(cupsWordForm(rewardCups)), streakDays, rewardCups, cupsWordForm(rewardCups), totalCups, forms.Olympian, forms.Entered)

	reply := tgbotapi.NewMessage(msg.Chat.ID, messageText)
	b.api.Send(reply)
}

func (b *Bot) sendSixtyDayCupsReward(msg *tgbotapi.Message, username string, streakDays int, userGender string) {
	b.sendStreakReward(msg, username, streakDays, 420, "🏆 60 дней!", "🔥 Два месяца без провала — легенда растёт.")
}

func (b *Bot) sendQuarterlyCupsReward(msg *tgbotapi.Message, username string, streakDays int, userGender string) {
	b.sendStreakReward(msg, username, streakDays, 420, "🏆 90 дней!", "🏁 Квартал дисциплины — элита так и тренируется.")
}

func (b *Bot) sendHundredDayCupsReward(msg *tgbotapi.Message, username string, streakDays int, userGender string) {
	totalCups, _ := b.db.GetUserCups(msg.From.ID, b.packTrainingStateChatID(msg))
	rewardCups := 4200
	forms := b.getGenderForms(userGender)

	messageText := fmt.Sprintf(`🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆 БЕССМЕРТНАЯ ЛЕГЕНДА! 🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆

%s, ты тренируешься уже %d %s подряд! 

🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆

🎯 +%d %s ЗА ТВОЮ СЕРИЮ %d ДНЕЙ! 🎯

🏆 +%d %s
🏆 Всего кубков: %d
🦁 Fat Leopard не может поверить в твою силу воли! 
💪 Ты %s %s дисциплины!
🔥 100 дней абсолютной преданности — это уровень единиц!
⭐ Ты не просто тренируешься — ты создаёшь историю!
👑 Каждый день ты %s лучше, сильнее, увереннее!
🌟 Твоя дисциплина — пример для всех — ты %s!
💎 Ты %s, что можешь достичь ЛЮБОЙ цели — небо не предел!

#hundred_days_immortal #4200_cups #training_perfection`,
		username, streakDays, daysWordForm(streakDays), rewardCups, strings.ToUpper(cupsWordForm(rewardCups)), streakDays, rewardCups, cupsWordForm(rewardCups), totalCups, forms.Invincible, forms.Titan, forms.Became, forms.Champion, forms.Proved)

	reply := tgbotapi.NewMessage(msg.Chat.ID, messageText)
	b.api.Send(reply)
}

func (b *Bot) sendOneHundredEightyDayCupsReward(msg *tgbotapi.Message, username string, streakDays int, userGender string) {
	b.sendStreakReward(msg, username, streakDays, 420, "🏆 180 дней!", fmt.Sprintf("🔥 Полгода серии — +420 %s!", cupsWordForm(420)))
}

func (b *Bot) sendTwoHundredDayCupsReward(msg *tgbotapi.Message, username string, streakDays int, userGender string) {
	totalCups, _ := b.db.GetUserCups(msg.From.ID, b.packTrainingStateChatID(msg))
	rewardCups := 4200
	messageText := fmt.Sprintf(`🌸 БУКЕТ ИЗ КУБКОВ! 🌸

%s, %d %s подряд!

🏆🌸🏆🌸🏆🌸🏆🌸🏆🌸🏆🌸🏆
🌸🏆🌸🏆🌸🏆🌸🏆🌸🏆🌸🏆🌸
🏆🌸🏆🌸🏆🌸🏆🌸🏆🌸🏆🌸🏆

🎯 +%d %s — букет из кубков за твою серию %d %s!

🏆 +%d %s (всего: %d)
🦁 Fat Leopard дарит тебе этот букет — ты легенда!

#two_hundred_days #bouquet_of_cups #4200_cups`,
		username, 200, daysWordForm(200), rewardCups, strings.ToUpper(cupsWordForm(rewardCups)), streakDays, daysWordForm(streakDays), rewardCups, cupsWordForm(rewardCups), totalCups)
	reply := tgbotapi.NewMessage(msg.Chat.ID, messageText)
	b.api.Send(reply)
}

func (b *Bot) sendTwoHundredFortyDayCupsReward(msg *tgbotapi.Message, username string, streakDays int, userGender string) {
	b.sendStreakReward(msg, username, streakDays, 4200, "🏆 240 дней!", "🔥 240 дней серии — уровень титана!")
}

func (b *Bot) sendSuperLevelMessage(msg *tgbotapi.Message, username string, totalCups int, userGender string) {
	forms := b.getGenderForms(userGender)

	messageText := fmt.Sprintf(`🌟⚡️ СУПЕР-УРОВЕНЬ ДОСТИГНУТ! ⚡️🌟

%s, ты %s %d %s! 

🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆
🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆🏆

🎊 ВСЕ ОЖИДАНИЯ ПРЕВЗОЙДЕНЫ! 🎊

🦁 Fat Leopard в полном восторге! 
💪 Ты не просто %s - ты СУПЕР-%s!
🔥 Твоя сила и мощь безграничны!
⭐️ Ты вдохновляешь всю стаю!
👑 Мотивация не верит, что такое бывает!
🌟 Ты сияешь ярче всех!

🎯 Продолжай в том же духе, супер-леопард!

#super_level #%d_cups #motivation_king`,
		username, forms.Accumulated, totalCups, cupsWordForm(totalCups), forms.Champion, strings.ToUpper(forms.Champion), totalCups)

	reply := tgbotapi.NewMessage(msg.Chat.ID, messageText)
	if _, err := b.api.Send(reply); err != nil {
		b.logger.Errorf("Failed to send super level message: %v", err)
	}
}

// checkMerchGiveawayCompletion проверяет, завершен ли розыгрыш (3 участника с 420+ кубками)
func (b *Bot) checkMerchGiveawayCompletion(msg *tgbotapi.Message, chatID int64) {
	usersWith420Cups, err := b.db.CountUsersWithCups(chatID, 420)
	if err != nil {
		b.logger.Errorf("Failed to count users with 420+ cups: %v", err)
		return
	}

	if usersWith420Cups == 3 {
		// 3-й участник достиг 420 кубков - розыгрыш завершен
		merchMessage := tgbotapi.NewMessage(chatID, `🎉🎊 РОЗЫГРЫШ ЗАВЕРШЕН! 🎊🎉

Третий участник достиг 420 кубков! 

🏆 Розыгрыш футболки Fat Leopard официально закрыт!

Поздравляем всех участников, которые набрали 420+ кубков! 🦁💪`)
		if _, err := b.api.Send(merchMessage); err != nil {
			b.logger.Errorf("Failed to send merch giveaway completion message: %v", err)
		} else {
			b.logger.Infof("Merch giveaway completed! 3 participants reached 420+ cups in chat %d", chatID)
		}
	}
}

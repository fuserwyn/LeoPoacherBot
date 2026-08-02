package bot

import (
	"strings"
	"time"

	"leo-bot/internal/domain"
	"leo-bot/internal/utils"
)

// Больничный замораживает стрик так же, как kick-таймер: дни болезни не считаются
// «пропущенными» тренировками. Здесь — общая арифметика, которая вычитает дни больничного
// из промежутка между последней тренировкой и «сегодня», используемая и для отображения
// стрика (профиль мини-аппа), и для его пересчёта при возвращении с больничного.

// sickAdjustedLastTrainingDate сдвигает дату последней тренировки вперёд на число целых
// дней больничного, попавших в промежуток (last, today]. Эти дни стрик не жгут. Все даты —
// YYYY-MM-DD в локальном TZ пользователя; для активного больничного sickEnd = today
// (заморозка на всё время болезни). Возвращает исходную дату, если больничного в
// промежутке нет или даты не парсятся.
func sickAdjustedLastTrainingDate(lastTrainingDate, today, sickStart, sickEnd string) string {
	const layout = "2006-01-02"
	last, err1 := time.Parse(layout, strings.TrimSpace(lastTrainingDate))
	td, err2 := time.Parse(layout, strings.TrimSpace(today))
	ss, err3 := time.Parse(layout, strings.TrimSpace(sickStart))
	se, err4 := time.Parse(layout, strings.TrimSpace(sickEnd))
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return lastTrainingDate
	}
	// Больничный защищает только дни строго после последней тренировки и не позже сегодня.
	effStart := last
	if ss.After(effStart) {
		effStart = ss
	}
	effEnd := td
	if se.Before(effEnd) {
		effEnd = se
	}
	sickDays := int(effEnd.Sub(effStart).Hours() / 24)
	if sickDays <= 0 {
		return lastTrainingDate
	}
	shifted := last.AddDate(0, 0, sickDays)
	if shifted.After(td) {
		shifted = td
	}
	return shifted.Format(layout)
}

// localDateFromMoscowRFC3339 переводит момент больничного (RFC3339 в МСК, как его пишет
// utils.FormatMoscowTime) в дату YYYY-MM-DD в локальном TZ пользователя.
func localDateFromMoscowRFC3339(ts string, offsetFromMoscow int) (string, bool) {
	t, err := utils.ParseMoscowTime(strings.TrimSpace(ts))
	if err != nil {
		return "", false
	}
	return t.Add(time.Duration(offsetFromMoscow) * time.Hour).Format("2006-01-02"), true
}

// sickWindowLocalDates возвращает интервал последнего больничного в датах локального TZ
// пользователя. Для активного больничного конец = today (стрик заморожен всё время
// болезни). ok=false, если больничного не было (нет времени начала).
func (b *Bot) sickWindowLocalDates(ml *domain.MessageLog, today string) (startDate, endDate string, ok bool) {
	if ml == nil || ml.SickLeaveStartTime == nil {
		return "", "", false
	}
	startDate, ok = localDateFromMoscowRFC3339(*ml.SickLeaveStartTime, ml.TimezoneOffsetFromMoscow)
	if !ok {
		return "", "", false
	}
	// Активный больничный — конец «плавает» до сегодня.
	if ml.HasSickLeave {
		return startDate, today, true
	}
	// Завершённый больничный — берём зафиксированную дату выздоровления.
	if ml.SickLeaveEndTime != nil {
		if endDate, ok = localDateFromMoscowRFC3339(*ml.SickLeaveEndTime, ml.TimezoneOffsetFromMoscow); ok {
			return startDate, endDate, true
		}
	}
	return "", "", false
}

// sickAdjustedDaysSince уменьшает «дней без тренировки» на дни больничного из ml.
// lastTrainingDate — фактическая дата последнего отчёта (YYYY-MM-DD, локальный TZ).
// Никогда не увеличивает rawDays.
func (b *Bot) sickAdjustedDaysSince(ml *domain.MessageLog, lastTrainingDate string, rawDays int) int {
	if ml == nil || strings.TrimSpace(lastTrainingDate) == "" {
		return rawDays
	}
	today := b.getUserLocalDate(ml.TimezoneOffsetFromMoscow)
	ss, se, ok := b.sickWindowLocalDates(ml, today)
	if !ok {
		return rawDays
	}
	adjLast := sickAdjustedLastTrainingDate(lastTrainingDate, today, ss, se)
	last, err1 := time.Parse("2006-01-02", adjLast)
	td, err2 := time.Parse("2006-01-02", today)
	if err1 != nil || err2 != nil {
		return rawDays
	}
	d := int(td.Sub(last).Hours()/24 + 0.5)
	if d < 0 {
		d = 0
	}
	if d < rawDays {
		return d
	}
	return rawDays
}

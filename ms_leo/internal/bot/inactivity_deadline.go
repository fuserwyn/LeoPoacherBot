package bot

import (
	"time"

	"leo-bot/internal/domain"
	"leo-bot/internal/utils"
)

// userLocalLoc — фиксированная зона юзера: UTC+3 (МСК) + смещение относительно МСК.
func userLocalLoc(tzOffsetFromMoscow int) *time.Location {
	return time.FixedZone("UserLocal", (3+tzOffsetFromMoscow)*3600)
}

// removalDeadlineLocal — момент кика за неактивность: 00:00 локального TZ юзера
// **следующего** календарного дня после дня, на который приходится (lastTraining + 7×24ч).
func removalDeadlineLocal(lastTraining time.Time, tzOffsetFromMoscow int) time.Time {
	loc := userLocalLoc(tzOffsetFromMoscow)
	t := lastTraining.In(loc).Add(7 * 24 * time.Hour)
	y, m, d := t.Date()
	startOfThatCalendarDay := time.Date(y, m, d, 0, 0, 0, 0, loc)
	return startOfThatCalendarDay.AddDate(0, 0, 1)
}

// nextCalendarMidnightAfterMoscow — 00:00 МСК **следующего** календарного дня относительно момента t (как «до конца суток после #healthy»).
func nextCalendarMidnightAfterMoscow(t time.Time) time.Time {
	loc := utils.GetMoscowTime().Location()
	d := t.In(loc)
	y, m, day := d.Date()
	startOfDay := time.Date(y, m, day, 0, 0, 0, 0, loc)
	return startOfDay.AddDate(0, 0, 1)
}

// inactivityKickDeadline — абсолютный момент удаления из стаи за неактивность (если не будет #training_done), по правилам больничного.
func inactivityKickDeadline(ml *domain.MessageLog, now time.Time) (time.Time, bool) {
	if ml == nil || ml.TimerStartTime == nil {
		return time.Time{}, false
	}
	if ml.IsExemptFromDeletion || ml.IsDeleted {
		return time.Time{}, false
	}
	ts := *ml.TimerStartTime
	if ts == "" {
		return time.Time{}, false
	}
	timerStart, err := utils.ParseMoscowTime(ts)
	if err != nil {
		return time.Time{}, false
	}
	D0 := removalDeadlineLocal(timerStart, ml.TimezoneOffsetFromMoscow)

	// Активный больничный: дедлайн сдвигается на время болезни (таймер «заморожен»).
	if ml.HasSickLeave && !ml.HasHealthy && ml.SickLeaveStartTime != nil {
		sickStart, err2 := utils.ParseMoscowTime(*ml.SickLeaveStartTime)
		if err2 != nil {
			return time.Time{}, false
		}
		return D0.Add(now.Sub(sickStart)), true
	}

	// После #healthy: сдвиг на длительность больничного и минимум до ближайшей полуночи после выхода.
	if ml.SickLeaveStartTime != nil && ml.SickLeaveEndTime != nil && ml.HasHealthy {
		sickStart, e1 := utils.ParseMoscowTime(*ml.SickLeaveStartTime)
		sickEnd, e2 := utils.ParseMoscowTime(*ml.SickLeaveEndTime)
		if e1 != nil || e2 != nil {
			return D0, true
		}
		if timerStart.After(sickEnd) {
			return D0, true
		}
		shifted := D0.Add(sickEnd.Sub(sickStart))
		grace := nextCalendarMidnightAfterMoscow(sickEnd)
		if shifted.After(grace) {
			return shifted, true
		}
		return grace, true
	}

	return D0, true
}

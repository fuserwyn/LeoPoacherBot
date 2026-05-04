// Package leopardmoney — правила Leopard Money Model (XP, ачивки, таймер неактивности).
package leopardmoney

import "time"

const (
	// EntryRub — стоимость доступа в Fat Leopard MiniApp (руб.), для env / документации.
	EntryRub = 420
	// ReturnRub — повторный доступ после кика за неактивность (руб.).
	ReturnRub = 210
	// FreezeRub — покупка заморозки XP (руб.).
	FreezeRub = 42

	XPPerActiveDay   = 6
	XPPerInactiveDay = 6

	// MaxAchievements — максимум ачивок за серии по 7 дней (28 дней).
	MaxAchievements = 4

	StarterXP               = 42
	StarterAchievementTitle = "Первый след"

	// InactiveRemovalDays — условное «окно» ~8 календарных суток; фактический кик — 00:00 МСК после 7×24ч от последнего отчёта (см. removalDeadlineMoscow).
	InactiveRemovalDays = 8

	// FullTimerDuration — верхняя оценка окна до кика (для fallback restore и UI).
	FullTimerDuration = time.Duration(InactiveRemovalDays) * 24 * time.Hour
)

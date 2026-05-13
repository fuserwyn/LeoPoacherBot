// Package leopardmoney — правила Leopard Money Model (кубки, уровни, ачивки, таймер неактивности).
package leopardmoney

import "time"

const (
	// EntryRub — стоимость доступа в Fat Leopard MiniApp (руб.).
	EntryRub = 420
	// ReturnRub — повторный доступ после кика за неактивность (руб.).
	ReturnRub = 210

	// MaxAchievements — количество ачивок за стрик (7, 14, 21, 30, 42, 50, 100 дней).
	MaxAchievements = 7

	// InactiveRemovalDays — 8 дней без активности → удаление в 00:00 локального TZ юзера.
	InactiveRemovalDays = 8

	// FullTimerDuration — верхняя оценка окна до кика (для fallback restore и UI).
	FullTimerDuration = time.Duration(InactiveRemovalDays) * 24 * time.Hour
)

// LevelNames — имена уровней по саванне (индекс = номер уровня 1…6+).
// Леопард — НЕ уровень, это бренд продукта и образ тренера Лео.
var LevelNames = []string{"", "Сурикат", "Газель", "Зебра", "Гепард", "Лев", "Слон"}

package leopardmoney

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Пороги накопленных кубков (нижняя граница уровня): L1 с 0, L2 с 420, … (§2.5 спеки).
var LevelStartCups = []int{0, 420, 1260, 2940, 6300, 13020, 26460}

// StreakAchievementMilestones — пороги стрика (дней подряд) для получения ачивок в мини-аппе.
var StreakAchievementMilestones = []int{7, 14, 30, 42, 60, 90, 180, 365}

// StreakAchievementIndex — 0-based индекс ачивки для данного стрика; -1 если не совпадает ни с одним порогом.
func StreakAchievementIndex(streak int) int {
	for i, m := range StreakAchievementMilestones {
		if streak == m {
			return i
		}
	}
	return -1
}

// AchievementsCountForStreak — сколько ачивок должно быть открыто при данном стрике (все пороги ≤ streak).
func AchievementsCountForStreak(streak int) int {
	if streak < 0 {
		streak = 0
	}
	n := 0
	for _, m := range StreakAchievementMilestones {
		if streak >= m {
			n++
		}
	}
	if n > MaxAchievements {
		n = MaxAchievements
	}
	return n
}

// LastAchievementMilestoneForStreak — последний порог стрика, достигнутый при данном стрике (0 если ни один).
func LastAchievementMilestoneForStreak(streak int) int {
	last := 0
	for _, m := range StreakAchievementMilestones {
		if streak >= m && m > last {
			last = m
		}
	}
	return last
}

// LevelName — имя уровня по его номеру (1-based). Возвращает пустую строку для неизвестного уровня.
func LevelName(level int) string {
	if level < 1 || level >= len(LevelNames) {
		return ""
	}
	return LevelNames[level]
}

// LevelFromTotalCups — уровень 1…7+ по накопленным кубкам.
func LevelFromTotalCups(total int) int {
	if total < 0 {
		total = 0
	}
	lvl := 1
	for i := 1; i < len(LevelStartCups); i++ {
		if total >= LevelStartCups[i] {
			lvl = i + 1
		} else {
			break
		}
	}
	return lvl
}

// ActivityCoeff — тип-коэффициент по id активности (как в miniapp workoutCategories).
func ActivityCoeff(categoryID string) float64 {
	switch strings.ToLower(strings.TrimSpace(categoryID)) {
	case "yoga", "stretch":
		return 0.8
	case "walk":
		return 0.8
	case "rowing", "workout", "strength", "kettlebell", "dance", "other":
		return 1.0
	case "swim", "bike", "run", "cardio", "jump_rope":
		return 1.2
	case "crossfit", "hiit":
		return 1.5
	default:
		return 1.0
	}
}

var (
	// Мини-апп: «бег, 15 мин, инт. 3/5». Старые записи в ленте могли начинаться с #training_done — префикс опционален при разборе.
	reTrainingHeader = regexp.MustCompile(`(?i)^(?:#training_done\s*[—–\-]\s*)?([^,]+),\s*(\d+)\s*мин`)
	reIntensity      = regexp.MustCompile(`(?i)инт\.?\s*(\d+)`)
)

// labelToCategoryID — русские подписи из UI (нижний регистр) → id.
var labelToCategoryID = map[string]string{
	"бег":         "run",
	"ходьба":      "walk",
	"велосипед":   "bike",
	"плавание":    "swim",
	"йога":        "yoga",
	"гребля":      "rowing",
	"воркаут":     "workout",
	"кроссфит":    "crossfit",
	"растяжка":    "stretch",
	"танцы":       "dance",
	"hiit":        "hiit",
	"кардио":      "cardio",
	"гиря":        "kettlebell",
	"силовая":     "strength",
	"скакалка":    "jump_rope",
	"другое":      "other",
	"отжимания":   "other",
	"отжимание":   "other",
}

// IsTrainingReportLine — первая строка похожа на отчёт о тренировке из мини-аппа.
func IsTrainingReportLine(text string) bool {
	_, _, _, ok := ParseTrainingDoneReport(text)
	return ok
}

// ParseTrainingDoneReport — первая строка отчёта мини-аппа, например «бег, 15 мин, инт. 3/5».
// Возвращает ok=false, если нет распознанного заголовка (тогда начисление — минимум 1 кубок снаружи).
func ParseTrainingDoneReport(text string) (durationMin, intensity int, categoryID string, ok bool) {
	line := strings.TrimSpace(text)
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	m := reTrainingHeader.FindStringSubmatch(line)
	if len(m) < 3 {
		return 0, 0, "other", false
	}
	rawLabel := strings.TrimSpace(strings.ToLower(m[1]))
	dur, err := strconv.Atoi(m[2])
	if err != nil || dur <= 0 {
		return 0, 0, "other", false
	}
	intensity = 1
	if im := reIntensity.FindStringSubmatch(line); len(im) >= 2 {
		if v, e := strconv.Atoi(im[1]); e == nil && v > 0 {
			intensity = v
		}
	}
	cat := labelToCategoryID[rawLabel]
	if cat == "" {
		cat = "other"
	}
	return dur, intensity, cat, true
}

// TrainingCupsFromParts — кубки по формуле §2.2: (длительность×интенсивность×тип)/5, округление до целого,
// минимум 1. Длительность 1–480 мин, интенсивность 1–5.
func TrainingCupsFromParts(durationMin, intensity int, categoryID string) int {
	d := durationMin
	if d < 1 {
		d = 1
	}
	if d > 480 {
		d = 480
	}
	in := intensity
	if in < 1 {
		in = 1
	}
	if in > 5 {
		in = 5
	}
	coef := ActivityCoeff(categoryID)
	raw := float64(d*in) * coef / 5.0
	n := int(math.Floor(raw + 0.5))
	if n < 1 {
		n = 1
	}
	return n
}

// TrainingCupsFromReportText — разбор + расчёт; при неразобранном заголовке возвращает 1.
func TrainingCupsFromReportText(text string) int {
	dur, inten, cat, ok := ParseTrainingDoneReport(text)
	if !ok {
		return 1
	}
	return TrainingCupsFromParts(dur, inten, cat)
}

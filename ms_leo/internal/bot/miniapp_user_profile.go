package bot

import (
	"database/sql"
	"errors"
	"math"
	"strings"
	"time"

	"leo-bot/internal/database"
	"leo-bot/internal/game/leopardmoney"
	"leo-bot/internal/utils"
)

// Лимиты полей мини-апп — профиль.
const (
	miniappProfileNameMax     = 64
	miniappProfileAgeMax      = 120
	miniappProfileTzOffsetMin = -12
	miniappProfileTzOffsetMax = 12
)

// LeoUserGenderForTrainingFeed — m/f/пусто: сначала явный мини-апп, затем training_state, без догадок по имени.
func (b *Bot) LeoUserGenderForTrainingFeed(userID int64, logGender string) string {
	g := strings.TrimSpace(strings.ToLower(logGender))
	if b != nil && b.config != nil && b.config.MonetizedChatID != 0 && b.db != nil {
		if p, err := b.db.GetMiniappUserProfile(userID, b.config.MonetizedChatID); err == nil && p != nil {
			pg := strings.TrimSpace(strings.ToLower(p.Gender))
			if pg == "m" || pg == "f" {
				return pg
			}
		}
	}
	if g == "m" || g == "f" {
		return g
	}
	return ""
}

// LeoUserProfileForFeedPrompt — подсказки к ИИ (имя, возраст).
func (b *Bot) LeoUserProfileForFeedPrompt(userID int64) (displayName string, age *int) {
	if b == nil || b.db == nil || b.config == nil || b.config.MonetizedChatID == 0 {
		return "", nil
	}
	p, err := b.db.GetMiniappUserProfile(userID, b.config.MonetizedChatID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", nil
	}
	if p == nil {
		return "", nil
	}
	dn := strings.TrimSpace(p.DisplayName)
	if len([]rune(dn)) > miniappProfileNameMax {
		dn = string([]rune(dn)[:miniappProfileNameMax])
	}
	if p.AgeYears.Valid {
		a := int(p.AgeYears.Int64)
		if a > 0 && a <= miniappProfileAgeMax {
			age = &a
		}
	}
	return dn, age
}

// MonetizedChatID — id чата стаи (MONETIZED_CHAT_ID); для мини-апп API.
func (b *Bot) MonetizedChatID() int64 {
	if b == nil || b.config == nil {
		return 0
	}
	return b.config.MonetizedChatID
}

// SaveMiniappUserProfileFromMiniapp пишет miniapp_user_profile и дублирует gender в training_state.
func (b *Bot) SaveMiniappUserProfileFromMiniapp(userID, packChatID int64, gender, displayName string, age *int) error {
	if b == nil || b.db == nil {
		return nil
	}
	g := strings.TrimSpace(strings.ToLower(gender))
	if g != "m" && g != "f" && g != "" {
		g = ""
	}
	dn := strings.TrimSpace(displayName)
	if len([]rune(dn)) > miniappProfileNameMax {
		dn = string([]rune(dn)[:miniappProfileNameMax])
	}
	var n sql.NullInt64
	existing, errG := b.db.GetMiniappUserProfile(userID, packChatID)
	if age == nil {
		if errG == nil && existing != nil && existing.AgeYears.Valid {
			n = existing.AgeYears
		}
	} else {
		if *age == 0 {
			// явный сброс
			n = sql.NullInt64{Valid: false}
		} else if *age > 0 && *age <= miniappProfileAgeMax {
			n = sql.NullInt64{Int64: int64(*age), Valid: true}
		}
	}
	prof := &database.MiniappUserProfile{
		UserID:      userID,
		PackChatID:  packChatID,
		Gender:      g,
		DisplayName: dn,
		AgeYears:    n,
	}
	if err := b.db.UpsertMiniappUserProfile(prof); err != nil {
		return err
	}
	_ = b.db.PatchTrainingStateGenderIfExists(userID, packChatID, g)
	return nil
}

// GetTimezoneOffsetForAPI — текущий TZ-offset (часы относительно МСК) пользователя из training_state.
// 0 если строки нет — соответствует «по умолчанию МСК».
func (b *Bot) GetTimezoneOffsetForAPI(userID, packChatID int64) int {
	if b == nil || b.db == nil || packChatID == 0 {
		return 0
	}
	off, err := b.db.GetTimezoneOffsetFromTrainingState(userID, packChatID)
	if err != nil {
		b.logger.Errorf("miniapp tz offset load user=%d pack=%d: %v", userID, packChatID, err)
		return 0
	}
	if off < miniappProfileTzOffsetMin || off > miniappProfileTzOffsetMax {
		return 0
	}
	return off
}

// SaveTimezoneOffsetFromMiniapp — сохраняет смещение часового пояса (-12..+12) в training_state.
// Если онбординга ещё не было — UPDATE просто ничего не сделает; пользователь увидит выбор «МСК+0» по умолчанию.
func (b *Bot) SaveTimezoneOffsetFromMiniapp(userID, packChatID int64, offset int) error {
	if b == nil || b.db == nil {
		return nil
	}
	if offset < miniappProfileTzOffsetMin || offset > miniappProfileTzOffsetMax {
		return errors.New("invalid timezone offset")
	}
	return b.db.PatchTrainingStateTimezoneOffsetIfExists(userID, packChatID, offset)
}

// GetMiniappUserProfileJSONForAPI — DTO GET (мини-апп, иначе gender из training_state).
func (b *Bot) GetMiniappUserProfileJSONForAPI(userID, packChatID int64) (gender, displayName string, age *int) {
	if b == nil || b.db == nil || packChatID == 0 {
		return "", "", nil
	}
	if p, err := b.db.GetMiniappUserProfile(userID, packChatID); err == nil && p != nil {
		gender = strings.TrimSpace(strings.ToLower(p.Gender))
		displayName = strings.TrimSpace(p.DisplayName)
		if p.AgeYears.Valid {
			a := int(p.AgeYears.Int64)
			if a > 0 {
				age = &a
			}
		}
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	if gender != "m" && gender != "f" {
		if ml, e := b.db.GetMessageLog(userID, packChatID); e == nil && ml != nil {
			g2 := strings.TrimSpace(strings.ToLower(ml.Gender))
			if g2 == "m" || g2 == "f" {
				gender = g2
			}
		}
	}
	return gender, displayName, age
}

type MiniappProfileStats struct {
	XP               int
	StreakDays       int
	MaxStreakDays    int
	AchievementCount int
	AchievementsMax  int
	WorkoutsTotal    int
	WorkoutsWeek     int
}

// GetMiniappProfileStatsForAPI — срез метрик профиля для мини-аппа (XP/стрик/рекорд/ачивки/тренировки).
func (b *Bot) GetMiniappProfileStatsForAPI(userID, packChatID int64) MiniappProfileStats {
	out := MiniappProfileStats{
		AchievementsMax: leopardmoney.MaxAchievements,
	}
	if b == nil || b.db == nil || userID == 0 || packChatID == 0 {
		return out
	}
	applyLog := func(chatID int64) {
		ml, err := b.db.GetMessageLog(userID, chatID)
		if err != nil || ml == nil {
			return
		}
		out.XP = int(math.Max(float64(out.XP), float64(ml.XP)))
		out.StreakDays = int(math.Max(float64(out.StreakDays), float64(ml.StreakDays)))
		maxStreak := ml.MaxStreakDays
		if maxStreak < ml.StreakDays {
			maxStreak = ml.StreakDays
		}
		out.MaxStreakDays = int(math.Max(float64(out.MaxStreakDays), float64(maxStreak)))
		out.AchievementCount = int(math.Max(float64(out.AchievementCount), float64(ml.AchievementCount)))
	}
	// Primary source: pack-row. Fallback/migration source: private-row (chat_id=userID).
	applyLog(packChatID)
	applyLog(userID)

	today := time.Now().UTC()
	weekAgo := today.AddDate(0, 0, -6)
	countAndMerge := func(chatID int64) {
		total, err := b.db.CountTrainingSessionsInDateRange(userID, chatID, "2000-01-01", today.Format("2006-01-02"))
		if err == nil {
			out.WorkoutsTotal = int(math.Max(float64(out.WorkoutsTotal), float64(total)))
		}
		week, err := b.db.CountTrainingSessionsInDateRange(userID, chatID, weekAgo.Format("2006-01-02"), today.Format("2006-01-02"))
		if err == nil {
			out.WorkoutsWeek = int(math.Max(float64(out.WorkoutsWeek), float64(week)))
		}
	}
	countAndMerge(packChatID)
	countAndMerge(userID)
	return out
}

// GetMiniappInactivityRemovalDeadlineRFC3339 — когда возможен кик за неактивность (если не будет отчёта), Europe/Moscow в RFC3339. Пусто — нет таймера, освобождён от кика или нет данных.
func (b *Bot) GetMiniappInactivityRemovalDeadlineRFC3339(userID, packChatID int64) string {
	if b == nil || b.db == nil || userID == 0 || packChatID == 0 {
		return ""
	}
	ml, err := b.db.GetMessageLog(userID, packChatID)
	if err != nil || ml == nil {
		return ""
	}
	if ml.IsExemptFromDeletion || ml.IsDeleted {
		return ""
	}
	now := utils.GetMoscowTime()
	dl, ok := inactivityKickDeadline(ml, now)
	if !ok {
		return ""
	}
	loc := utils.GetMoscowTime().Location()
	return dl.In(loc).Format(time.RFC3339)
}

// SyncMiniappTelegramPhotoFromInit — сохраняет URL аватарки из WebApp initData (user.photo_url) для ленты/чата.
// Пустая строка не затирает уже сохранённое (см. UpsertMiniappTelegramPhotoURL).
func (b *Bot) SyncMiniappTelegramPhotoFromInit(userID, packChatID int64, photoURL string) {
	if b == nil || b.db == nil {
		return
	}
	photoURL = strings.TrimSpace(photoURL)
	if photoURL == "" {
		return
	}
	if len(photoURL) > 768 {
		return
	}
	if err := b.db.UpsertMiniappTelegramPhotoURL(userID, packChatID, photoURL); err != nil {
		b.logger.Warnf("miniapp sync telegram photo user=%d: %v", userID, err)
	}
}

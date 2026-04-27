package bot

import (
	"database/sql"
	"errors"
	"strings"

	"leo-bot/internal/database"
)

// Лимиты полей мини-апп — профиль.
const (
	miniappProfileNameMax = 64
	miniappProfileAgeMax  = 120
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

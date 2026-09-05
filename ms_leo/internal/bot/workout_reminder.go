package bot

import (
	"context"
	"strings"
	"time"

	"leo-bot/internal/database"
	"leo-bot/internal/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	initdata "github.com/telegram-mini-apps/init-data-golang"
)

// WorkoutReminderView — настройки напоминания для отдачи в мини-апп.
type WorkoutReminderView struct {
	Enabled    bool `json:"enabled"`
	RemindHour int  `json:"remind_hour"`
}

// GetWorkoutReminderForViewer — настройки напоминания текущего пользователя (дефолт, если не настроено).
func (b *Bot) GetWorkoutReminderForViewer(viewerUserID int64, initD initdata.InitData) (WorkoutReminderView, error) {
	if err := b.AssertMiniAppPackChatAligns(initD); err != nil {
		return WorkoutReminderView{}, err
	}
	if err := b.assertPackFeedSocialViewer(viewerUserID); err != nil {
		return WorkoutReminderView{}, err
	}
	s, err := b.db.GetWorkoutReminderSettings(viewerUserID, b.config.MonetizedChatID)
	if err != nil {
		return WorkoutReminderView{}, err
	}
	return WorkoutReminderView{Enabled: s.Enabled, RemindHour: s.RemindHour}, nil
}

// SaveWorkoutReminderForViewer — сохранить настройки напоминания текущего пользователя.
func (b *Bot) SaveWorkoutReminderForViewer(viewerUserID int64, initD initdata.InitData, enabled bool, remindHour int) error {
	if err := b.AssertMiniAppPackChatAligns(initD); err != nil {
		return err
	}
	if err := b.assertPackFeedSocialViewer(viewerUserID); err != nil {
		return err
	}
	if remindHour < 0 || remindHour > 23 {
		return ErrTrainingFeedParentNotFound
	}
	return b.db.SaveWorkoutReminderSettings(viewerUserID, b.config.MonetizedChatID, database.WorkoutReminderSettings{
		Enabled:    enabled,
		RemindHour: remindHour,
	})
}

// workoutReminderTick — как часто планировщик проверяет, у кого настал «час напоминания».
// Раз в минуту: окно срабатывания — целый локальный час (localHour == remind_hour), а
// идемпотентность держит last_sent_msk_date, так что лишних отправок не будет.
const workoutReminderTick = 1 * time.Minute

// startWorkoutReminderScheduler — фоновое напоминание «не забудь внести тренировку».
// Логика и фильтры зеркалят inactivity-цепочку: только pack-row (MonetizedChatID),
// не трогаем удалённых / exempt / на больничном (это уже отфильтровано SQL-кандидатами).
// Время — локальный час пользователя (TZ из training_state.timezone_offset_from_moscow).
func (b *Bot) startWorkoutReminderScheduler(ctx context.Context) {
	if b == nil {
		return
	}
	if b.config == nil || b.config.MonetizedChatID == 0 {
		b.logger.Info("workout reminder scheduler: skipped, MONETIZED_CHAT_ID не настроен")
		return
	}
	b.logger.Infof("workout reminder scheduler: started, tick=%s", workoutReminderTick)
	ticker := time.NewTicker(workoutReminderTick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			b.logger.Info("workout reminder scheduler: stopped")
			return
		case <-ticker.C:
			b.runWorkoutReminderSweep()
		}
	}
}

// runWorkoutReminderSweep — одна итерация: всем, у кого сейчас «их» локальный час и кто
// сегодня ещё не тренировался, шлём дружелюбный пинок. Идемпотентно по дате МСК.
func (b *Bot) runWorkoutReminderSweep() {
	if b == nil || b.db == nil || b.config == nil {
		return
	}
	packChatID := b.config.MonetizedChatID
	candidates, err := b.db.ListDueWorkoutReminderCandidates(packChatID)
	if err != nil {
		b.logger.Errorf("workout reminder sweep: list candidates: %v", err)
		return
	}
	mskToday := utils.GetMoscowTime().Format("2006-01-02")
	sent := 0
	for _, c := range candidates {
		// Час напоминания в локальном TZ юзера.
		loc := userLocalLoc(c.TimezoneOffsetFromMoscow)
		localNow := utils.GetMoscowTime().In(loc)
		if localNow.Hour() != c.RemindHour {
			continue
		}
		// Уже тренировался сегодня (локальная дата) — напоминать незачем.
		todayLocal := localNow.Format("2006-01-02")
		if c.LastTrainingDate.Valid && strings.TrimSpace(c.LastTrainingDate.String) == todayLocal {
			continue
		}
		// Уже отправляли сегодня (по МСК-дате) — не дублируем.
		if c.LastSentMskDate.Valid && strings.TrimSpace(c.LastSentMskDate.String) == mskToday {
			continue
		}
		b.sendWorkoutReminder(c.UserID, c.Username)
		if err := b.db.MarkWorkoutReminderSent(c.UserID, packChatID, mskToday); err != nil {
			b.logger.Warnf("workout reminder mark sent user=%d: %v", c.UserID, err)
		}
		sent++
	}
	if sent > 0 {
		b.logger.Infof("workout reminder sweep: sent=%d (of %d candidates)", sent, len(candidates))
	}
}

func workoutReminderText(username string) string {
	who := normalizeUserDisplayName(username)
	prefix := ""
	if strings.TrimSpace(who) != "" {
		prefix = who + ", "
	}
	return "🐆 " + prefix + "не забудь внести сегодняшнюю тренировку в мини-аппе"
}

// sendWorkoutReminder — пуш в очередь мини-аппа + DM в Telegram (как предупреждения о неактивности).
func (b *Bot) sendWorkoutReminder(userID int64, username string) {
	if b == nil || userID == 0 {
		return
	}
	text := workoutReminderText(username)
	b.miniappPersonalPush(userID, text)
	if b.api == nil {
		return
	}
	if _, err := b.api.Send(tgbotapi.NewMessage(userID, text)); err != nil {
		b.logger.Warnf("workout reminder DM user=%d: %v", userID, err)
	}
}

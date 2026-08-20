package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"leo-bot/internal/domain"
	"leo-bot/internal/logger"
	"leo-bot/internal/utils"

	"github.com/lib/pq"
)

// dbSessionTimeZone — все таблицы хранят время через DEFAULT (NOW() AT TIME ZONE 'Europe/Moscow'),
// что даёт «наивный» московский wall-clock. Чтобы при вставке в timestamptz он трактовался как
// московское время (а не как UTC сервера), фиксируем таймзону сессии. Иначе мгновение уезжает на
// +3 часа, и в мини-аппе сообщения показываются «из будущего».
const dbSessionTimeZone = "Europe/Moscow"

// ensureSessionTimeZone добавляет в строку подключения параметр timezone, если он ещё не задан.
// Поддерживает и URL-форму (postgresql://...), и keyword-форму (host=... dbname=...).
func ensureSessionTimeZone(databaseURL string) string {
	if strings.TrimSpace(databaseURL) == "" {
		return databaseURL
	}
	if strings.Contains(databaseURL, "://") {
		if u, err := url.Parse(databaseURL); err == nil {
			q := u.Query()
			if q.Get("timezone") == "" && q.Get("TimeZone") == "" {
				q.Set("timezone", dbSessionTimeZone)
				u.RawQuery = q.Encode()
			}
			return u.String()
		}
	}
	if strings.Contains(strings.ToLower(databaseURL), "timezone=") {
		return databaseURL
	}
	return databaseURL + " timezone=" + dbSessionTimeZone
}

func init() {
	// paywall_access_requests.access_expires_at = 'infinity'::timestamptz — без этого pq отдаёт []byte("infinity"),
	// Scan в sql.NullTime падает (refund / GetPaywallAccessRequestByID).
	// Должно быть до первого sql.Open("postgres", ...) в процессе (см. документацию pq.EnableInfinityTs).
	pq.EnableInfinityTs(
		time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC),
	)
}

type Database struct {
	db     *sql.DB
	logger logger.Logger
	// alphaTesterIDs — telegram_id альфа-тестеров (§10). Задаётся на старте через
	// SetAlphaTesterIDs; используется в insertEvent для пометки is_alpha.
	alphaTesterIDs map[int64]bool
}

func New(databaseURL string) (*Database, error) {
	// Если databaseURL пустой, используем дефолтное значение
	if databaseURL == "" {
		databaseURL = "postgresql://postgres:password@localhost:5432/leo_bot_db?sslmode=disable"
	}

	// Фиксируем таймзону сессии = Europe/Moscow (см. dbSessionTimeZone), чтобы DEFAULT-ы
	// (NOW() AT TIME ZONE 'Europe/Moscow') сохраняли корректное мгновение, а не +3 часа.
	databaseURL = ensureSessionTimeZone(databaseURL)

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Устанавливаем таймауты для подключения
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	// Проверяем соединение с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database (check DATABASE_URL and network connectivity): %w", err)
	}

	return &Database{
		db: db,
	}, nil
}

// NewForTest — уже открытый *sql.DB, для sqlmock в тестах доски.
func NewForTest(db *sql.DB) *Database {
	return &Database{db: db}
}

func (d *Database) Close() error {
	return d.db.Close()
}

// renameMessageLogToTrainingStateIfExists — для БД, созданных до переименования: не создавать вторую пустую таблицу рядом с legacy message_log.
func (d *Database) renameMessageLogToTrainingStateIfExists() error {
	_, err := d.db.Exec(`
		DO $rename_ml$
		BEGIN
			IF to_regclass('public.message_log') IS NOT NULL
				AND to_regclass('public.training_state') IS NULL THEN
				ALTER TABLE message_log RENAME TO training_state;
			END IF;
		END
		$rename_ml$;
	`)
	if err != nil {
		return fmt.Errorf("rename message_log to training_state: %w", err)
	}
	return nil
}

// CreateTables создает таблицы в базе данных, если они не существуют
func (d *Database) CreateTables() error {
	if err := d.renameMessageLogToTrainingStateIfExists(); err != nil {
		return err
	}

	queries := []string{
		`CREATE TABLE IF NOT EXISTS training_state (
			user_id BIGINT,
			username TEXT DEFAULT '',
			chat_id BIGINT,
			streak_days INTEGER DEFAULT 0,
			last_training_date TEXT,
			last_message TEXT NOT NULL,
			has_training_done BOOLEAN DEFAULT FALSE,
			has_sick_leave BOOLEAN DEFAULT FALSE,
			has_healthy BOOLEAN DEFAULT FALSE,
			is_deleted BOOLEAN DEFAULT FALSE,
			is_exempt_from_deletion BOOLEAN DEFAULT FALSE,
			timer_start_time TEXT,
			sick_leave_start_time TEXT,
			sick_leave_end_time TEXT,
			sick_time TEXT,
			timezone_offset_from_moscow INTEGER NOT NULL DEFAULT 0,
			sick_approval_pending BOOLEAN DEFAULT FALSE,
			sick_approval_deadline TIMESTAMP WITH TIME ZONE,
			sick_approval_message_id BIGINT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, chat_id)
		)`,
	}

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	// Запускаем миграции для обновления схемы
	if err := d.RunMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// SaveMessageLog сохраняет информацию о сообщении
func (d *Database) SaveMessageLog(msg *domain.MessageLog) error {
	query := `
		INSERT INTO training_state (user_id, username, chat_id, streak_days, max_streak_days, cups_earned, last_training_date, last_message, has_training_done, has_sick_leave, has_healthy, is_deleted, is_exempt_from_deletion, timer_start_time, sick_leave_start_time, sick_leave_end_time, sick_time, gender, timezone_offset_from_moscow, sick_approval_pending, sick_approval_deadline, sick_approval_message_id, achievement_count, xp_freeze_until, last_daily_xp_msk_date, leopard_starter_bonus_applied, last_achievement_streak_level, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28)
		ON CONFLICT (user_id, chat_id) 
		DO UPDATE SET 
			username = EXCLUDED.username,
			streak_days = EXCLUDED.streak_days,
			max_streak_days = EXCLUDED.max_streak_days,
			cups_earned = EXCLUDED.cups_earned,
			last_training_date = EXCLUDED.last_training_date,
			last_message = EXCLUDED.last_message,
			has_training_done = EXCLUDED.has_training_done,
			has_sick_leave = EXCLUDED.has_sick_leave,
			has_healthy = EXCLUDED.has_healthy,
			is_deleted = EXCLUDED.is_deleted,
			is_exempt_from_deletion = EXCLUDED.is_exempt_from_deletion,
			timer_start_time = EXCLUDED.timer_start_time,
			sick_leave_start_time = EXCLUDED.sick_leave_start_time,
			sick_leave_end_time = EXCLUDED.sick_leave_end_time,
			sick_time = EXCLUDED.sick_time,
			gender = CASE WHEN EXCLUDED.gender != '' THEN EXCLUDED.gender ELSE training_state.gender END,
			timezone_offset_from_moscow = EXCLUDED.timezone_offset_from_moscow,
			sick_approval_pending = EXCLUDED.sick_approval_pending,
			sick_approval_deadline = EXCLUDED.sick_approval_deadline,
			sick_approval_message_id = EXCLUDED.sick_approval_message_id,
			achievement_count = EXCLUDED.achievement_count,
			xp_freeze_until = EXCLUDED.xp_freeze_until,
			last_daily_xp_msk_date = EXCLUDED.last_daily_xp_msk_date,
			leopard_starter_bonus_applied = EXCLUDED.leopard_starter_bonus_applied,
			last_achievement_streak_level = EXCLUDED.last_achievement_streak_level,
			updated_at = $28
	`

	moscowTime := utils.FormatMoscowTime(utils.GetMoscowTime())

	result, err := d.db.Exec(query,
		msg.UserID, msg.Username, msg.ChatID, msg.StreakDays, msg.MaxStreakDays, msg.CupsEarned, msg.LastTrainingDate, msg.LastMessage, msg.HasTrainingDone,
		msg.HasSickLeave, msg.HasHealthy, msg.IsDeleted, msg.IsExemptFromDeletion, msg.TimerStartTime, msg.SickLeaveStartTime, msg.SickLeaveEndTime, msg.SickTime, msg.Gender, msg.TimezoneOffsetFromMoscow,
		msg.SickApprovalPending, msg.SickApprovalDeadline, msg.SickApprovalMessageID,
		msg.AchievementCount, msg.XpFreezeUntil, msg.LastDailyXPMskDate, msg.LeopardStarterBonusApplied, msg.LastAchievementStreakLevel,
		moscowTime)

	if err != nil {
		return err
	}
	_ = result
	return nil
}

// GetMessageLog получает информацию о сообщении пользователя
func (d *Database) GetMessageLog(userID, chatID int64) (*domain.MessageLog, error) {
	query := `
		SELECT user_id, username, chat_id, streak_days, max_streak_days, cups_earned, last_training_date, last_message, has_training_done, has_sick_leave, has_healthy, is_deleted, is_exempt_from_deletion,
		       timer_start_time, sick_leave_start_time, sick_leave_end_time, sick_time, gender, timezone_offset_from_moscow, sick_approval_pending, sick_approval_deadline, sick_approval_message_id,
		       achievement_count, xp_freeze_until, last_daily_xp_msk_date, leopard_starter_bonus_applied, last_achievement_streak_level, created_at, updated_at
		FROM training_state 
		WHERE user_id = $1 AND chat_id = $2 AND is_deleted = FALSE
		ORDER BY updated_at DESC
		LIMIT 1
	`

	var msg domain.MessageLog
	var lastDaily sql.NullString
	err := d.db.QueryRow(query, userID, chatID).Scan(
		&msg.UserID, &msg.Username, &msg.ChatID, &msg.StreakDays, &msg.MaxStreakDays, &msg.CupsEarned, &msg.LastTrainingDate, &msg.LastMessage, &msg.HasTrainingDone,
		&msg.HasSickLeave, &msg.HasHealthy, &msg.IsDeleted, &msg.IsExemptFromDeletion, &msg.TimerStartTime, &msg.SickLeaveStartTime, &msg.SickLeaveEndTime, &msg.SickTime, &msg.Gender, &msg.TimezoneOffsetFromMoscow,
		&msg.SickApprovalPending, &msg.SickApprovalDeadline, &msg.SickApprovalMessageID,
		&msg.AchievementCount, &msg.XpFreezeUntil, &lastDaily, &msg.LeopardStarterBonusApplied, &msg.LastAchievementStreakLevel, &msg.CreatedAt, &msg.UpdatedAt)
	if lastDaily.Valid {
		s := lastDaily.String
		msg.LastDailyXPMskDate = &s
	}
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

// GetMessageLogAnyState получает строку training_state без фильтра по is_deleted.
func (d *Database) GetMessageLogAnyState(userID, chatID int64) (*domain.MessageLog, error) {
	query := `
		SELECT user_id, username, chat_id, streak_days, max_streak_days, cups_earned, last_training_date, last_message, has_training_done, has_sick_leave, has_healthy, is_deleted, is_exempt_from_deletion,
		       timer_start_time, sick_leave_start_time, sick_leave_end_time, sick_time, gender, timezone_offset_from_moscow, sick_approval_pending, sick_approval_deadline, sick_approval_message_id,
		       achievement_count, xp_freeze_until, last_daily_xp_msk_date, leopard_starter_bonus_applied, last_achievement_streak_level, created_at, updated_at
		FROM training_state
		WHERE user_id = $1 AND chat_id = $2
		ORDER BY updated_at DESC
		LIMIT 1
	`

	var msg domain.MessageLog
	var lastDaily sql.NullString
	err := d.db.QueryRow(query, userID, chatID).Scan(
		&msg.UserID, &msg.Username, &msg.ChatID, &msg.StreakDays, &msg.MaxStreakDays, &msg.CupsEarned, &msg.LastTrainingDate, &msg.LastMessage, &msg.HasTrainingDone,
		&msg.HasSickLeave, &msg.HasHealthy, &msg.IsDeleted, &msg.IsExemptFromDeletion, &msg.TimerStartTime, &msg.SickLeaveStartTime, &msg.SickLeaveEndTime, &msg.SickTime, &msg.Gender, &msg.TimezoneOffsetFromMoscow,
		&msg.SickApprovalPending, &msg.SickApprovalDeadline, &msg.SickApprovalMessageID,
		&msg.AchievementCount, &msg.XpFreezeUntil, &lastDaily, &msg.LeopardStarterBonusApplied, &msg.LastAchievementStreakLevel, &msg.CreatedAt, &msg.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if lastDaily.Valid {
		s := lastDaily.String
		msg.LastDailyXPMskDate = &s
	}
	return &msg, nil
}

// UserHasActiveMessageLogInChat — в training_state есть живая запись пользователя в этом чате (учитываем для paywall, если getChatMember недоступен).
func (d *Database) UserHasActiveMessageLogInChat(userID, chatID int64) (bool, error) {
	const q = `
		SELECT EXISTS (
			SELECT 1 FROM training_state
			WHERE user_id = $1 AND chat_id = $2 AND is_deleted = FALSE
		)`
	var ok bool
	if err := d.db.QueryRow(q, userID, chatID).Scan(&ok); err != nil {
		return false, fmt.Errorf("exists training_state user/chat: %w", err)
	}
	return ok, nil
}

// GetUsersByChatID получает всех пользователей в чате
func (d *Database) GetUsersByChatID(chatID int64) ([]*domain.MessageLog, error) {
	query := `
		SELECT user_id, username, chat_id, streak_days, max_streak_days, cups_earned, last_training_date, last_message, has_training_done, has_sick_leave, has_healthy, is_deleted, is_exempt_from_deletion,
		       timer_start_time, sick_leave_start_time, sick_leave_end_time, sick_time, gender, timezone_offset_from_moscow, sick_approval_pending, sick_approval_deadline, sick_approval_message_id,
		       achievement_count, xp_freeze_until, last_daily_xp_msk_date, leopard_starter_bonus_applied, last_achievement_streak_level, created_at, updated_at
		FROM training_state 
		WHERE chat_id = $1 AND is_deleted = FALSE
		ORDER BY cups_earned DESC, last_message DESC
	`

	rows, err := d.db.Query(query, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*domain.MessageLog
	for rows.Next() {
		var msg domain.MessageLog
		var lastDaily2 sql.NullString
		err := rows.Scan(
			&msg.UserID, &msg.Username, &msg.ChatID, &msg.StreakDays, &msg.MaxStreakDays, &msg.CupsEarned, &msg.LastTrainingDate, &msg.LastMessage, &msg.HasTrainingDone,
			&msg.HasSickLeave, &msg.HasHealthy, &msg.IsDeleted, &msg.IsExemptFromDeletion, &msg.TimerStartTime, &msg.SickLeaveStartTime, &msg.SickLeaveEndTime, &msg.SickTime, &msg.Gender, &msg.TimezoneOffsetFromMoscow,
			&msg.SickApprovalPending, &msg.SickApprovalDeadline, &msg.SickApprovalMessageID,
			&msg.AchievementCount, &msg.XpFreezeUntil, &lastDaily2, &msg.LeopardStarterBonusApplied, &msg.LastAchievementStreakLevel, &msg.CreatedAt, &msg.UpdatedAt)
		if lastDaily2.Valid {
			s := lastDaily2.String
			msg.LastDailyXPMskDate = &s
		}
		if err != nil {
			return nil, err
		}
		users = append(users, &msg)
	}

	return users, nil
}

// GetUserIDByUsername получает user_id по username в конкретном чате
// Поддерживает поиск по разным форматам: @username, username, "Имя Фамилия"
func (d *Database) GetUserIDByUsername(username string, chatID int64) (int64, error) {
	// Сначала пробуем точное совпадение
	query := `
		SELECT user_id FROM training_state 
		WHERE username = $1 AND chat_id = $2
	`
	var userID int64
	err := d.db.QueryRow(query, username, chatID).Scan(&userID)
	if err == nil {
		return userID, nil
	}

	// Если не найдено, пробуем поиск с @
	if !strings.HasPrefix(username, "@") {
		query = `
			SELECT user_id FROM training_state 
			WHERE username = $1 AND chat_id = $2
		`
		err = d.db.QueryRow(query, "@"+username, chatID).Scan(&userID)
		if err == nil {
			return userID, nil
		}
	}

	// Если не найдено, пробуем поиск без @
	if strings.HasPrefix(username, "@") {
		query = `
			SELECT user_id FROM training_state 
			WHERE username = $1 AND chat_id = $2
		`
		err = d.db.QueryRow(query, username[1:], chatID).Scan(&userID)
		if err == nil {
			return userID, nil
		}
	}

	// Если все еще не найдено, пробуем поиск по частичному совпадению (для случаев типа "OggO Logos")
	query = `
		SELECT user_id FROM training_state 
		WHERE username ILIKE $1 AND chat_id = $2
	`
	err = d.db.QueryRow(query, "%"+username+"%", chatID).Scan(&userID)
	if err == nil {
		return userID, nil
	}

	return 0, fmt.Errorf("user not found")
}

// GetDatabaseStats получает статистику базы данных
func (d *Database) GetDatabaseStats() (map[string]interface{}, error) {
	query := `
		SELECT 
			COUNT(*) as total_users,
			COUNT(CASE WHEN has_training_done = true THEN 1 END) as training_done,
			COUNT(CASE WHEN has_sick_leave = true THEN 1 END) as sick_leave,
			COUNT(CASE WHEN has_healthy = true THEN 1 END) as healthy
		FROM training_state
	`

	var stats struct {
		TotalUsers   int `db:"total_users"`
		TrainingDone int `db:"training_done"`
		SickLeave    int `db:"sick_leave"`
		Healthy      int `db:"healthy"`
	}

	err := d.db.QueryRow(query).Scan(&stats.TotalUsers, &stats.TrainingDone, &stats.SickLeave, &stats.Healthy)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_users":   stats.TotalUsers,
		"training_done": stats.TrainingDone,
		"sick_leave":    stats.SickLeave,
		"healthy":       stats.Healthy,
	}, nil
}

// UpdateStreak обновляет стрик тренировок пользователя
func (d *Database) UpdateStreak(userID, chatID int64, streakDays int, lastTrainingDate string) error {
	query := `
		UPDATE training_state 
		SET streak_days = $3,
		    max_streak_days = GREATEST(COALESCE(max_streak_days, 0), $3),
		    last_training_date = $4,
		    updated_at = $5
		WHERE user_id = $1 AND chat_id = $2
	`
	// Используем московское время
	moscowTime := utils.FormatMoscowTime(utils.GetMoscowTime())
	_, err := d.db.Exec(query, userID, chatID, streakDays, lastTrainingDate, moscowTime)
	return err
}

// GetStreakSaveAttemptsUsed возвращает количество использованных попыток спасения стрика (lifetime).
func (d *Database) GetStreakSaveAttemptsUsed(userID, chatID int64) (int, error) {
	query := `
		SELECT COALESCE(streak_save_attempts_used, 0)
		FROM training_state
		WHERE user_id = $1 AND chat_id = $2
	`
	var used int
	err := d.db.QueryRow(query, userID, chatID).Scan(&used)
	if err != nil {
		return 0, err
	}
	return used, nil
}

// IncrementStreakSaveAttemptsUsed +1 к счётчику использованных попыток спасения стрика.
// Возвращает новое значение счётчика.
func (d *Database) IncrementStreakSaveAttemptsUsed(userID, chatID int64) (int, error) {
	query := `
		UPDATE training_state
		SET streak_save_attempts_used = COALESCE(streak_save_attempts_used, 0) + 1,
		    updated_at = $3
		WHERE user_id = $1 AND chat_id = $2
		RETURNING streak_save_attempts_used
	`
	moscowTime := utils.FormatMoscowTime(utils.GetMoscowTime())
	var used int
	err := d.db.QueryRow(query, userID, chatID, moscowTime).Scan(&used)
	if err != nil {
		return 0, err
	}
	return used, nil
}

// SetTimerStartTime пишет только timer_start_time — без полного SaveMessageLog,
// чтобы гонка со UpdateStreak не откатывала streak_days / last_training_date.
func (d *Database) SetTimerStartTime(userID, chatID int64, timerStartTime string) error {
	query := `
		UPDATE training_state
		SET timer_start_time = $3,
		    updated_at = $4
		WHERE user_id = $1 AND chat_id = $2
	`
	moscowTime := utils.FormatMoscowTime(utils.GetMoscowTime())
	_, err := d.db.Exec(query, userID, chatID, timerStartTime, moscowTime)
	return err
}

// ResetStreakDays сбрасывает только стрик, не трогая last_training_date
func (d *Database) ResetStreakDays(userID, chatID int64) error {
	query := `
		UPDATE training_state 
		SET streak_days = 0, updated_at = $3
		WHERE user_id = $1 AND chat_id = $2
	`
	// Используем московское время
	moscowTime := utils.FormatMoscowTime(utils.GetMoscowTime())
	_, err := d.db.Exec(query, userID, chatID, moscowTime)
	return err
}

// AddCups добавляет кубки пользователю
func (d *Database) AddCups(userID, chatID int64, cups int) error {
	query := `
		UPDATE training_state 
		SET cups_earned = cups_earned + $3, updated_at = $4
		WHERE user_id = $1 AND chat_id = $2
	`
	// Используем московское время
	moscowTime := utils.FormatMoscowTime(utils.GetMoscowTime())
	_, err := d.db.Exec(query, userID, chatID, cups, moscowTime)
	return err
}

// GetUserCups получает количество заработанных кубков пользователя
func (d *Database) GetUserCups(userID, chatID int64) (int, error) {
	query := `
		SELECT COALESCE(cups_earned, 0) 
		FROM training_state 
		WHERE user_id = $1 AND chat_id = $2
	`

	var cups int
	err := d.db.QueryRow(query, userID, chatID).Scan(&cups)
	if err != nil {
		return 0, err
	}

	return cups, nil
}

// CountUsersWithCups получает количество пользователей с указанным количеством кубков или больше
// Включает удаленных пользователей, если у них есть нужное количество кубков
func (d *Database) CountUsersWithCups(chatID int64, minCups int) (int, error) {
	query := `
		SELECT COUNT(DISTINCT user_id)
		FROM training_state 
		WHERE chat_id = $1 AND cups_earned >= $2
	`

	var count int
	err := d.db.QueryRow(query, chatID, minCups).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// MarkUserAsDeleted помечает пользователя как удаленного
func (d *Database) MarkUserAsDeleted(userID, chatID int64) error {
	// cups_earned = 0: кубки сгорают при удалении из стаи (per правил).
	// streak_days = 0: стрик сбрасывается. max_streak_days — пожизненный рекорд, не трогаем.
	// cups_at_deletion = старые cups_earned (RHS считается по старым значениям) — снапшот для
	// восстановления аккаунта админом «с достижениями». achievement_count тоже не трогаем.
	query := `
		UPDATE training_state
		SET is_deleted       = TRUE,
		    cups_at_deletion = cups_earned,
		    cups_earned      = 0,
		    streak_days      = 0,
		    updated_at       = $3
		WHERE user_id = $1 AND chat_id = $2
	`
	moscowTime := utils.FormatMoscowTime(utils.GetMoscowTime())
	_, err := d.db.Exec(query, userID, chatID, moscowTime)
	return err
}

// ClearSickLeaveStateOnTraining — любая тренировка снимает больничный и сбрасывает таймер
// неактивности «начисто». Без этого тренировка во время больничного не сбрасывала флаги
// (has_sick_leave/has_healthy/sick_*), состояние рассинхронизировалось и inactivityKickDeadline
// уходил в фоллбэк со старым (прошедшим) дедлайном → юзера кикало сразу по возвращении.
// timer_start_time здесь НЕ трогаем — его выставляет startTimer (NOW) в обработчике тренировки.
func (d *Database) ClearSickLeaveStateOnTraining(userID, chatID int64) error {
	const query = `
		UPDATE training_state
		SET has_sick_leave        = FALSE,
		    has_healthy           = FALSE,
		    sick_leave_start_time = NULL,
		    sick_leave_end_time   = NULL,
		    sick_time             = NULL,
		    updated_at            = $3
		WHERE user_id = $1 AND chat_id = $2
	`
	moscowTime := utils.FormatMoscowTime(utils.GetMoscowTime())
	_, err := d.db.Exec(query, userID, chatID, moscowTime)
	return err
}

// LogDeletionEvent пишет событие удаления пользователя и статус доставки DM.
func (d *Database) LogDeletionEvent(userID, chatID int64, dmStatus, errorText string) error {
	const query = `
		INSERT INTO deletion_events (user_id, chat_id, dm_status, error_text)
		VALUES ($1, $2, $3, NULLIF($4, ''))
	`
	_, err := d.db.Exec(query, userID, chatID, dmStatus, strings.TrimSpace(errorText))
	return err
}

// ReactivateReturnedUser переводит удаленного пользователя в активное состояние возврата.
// Возвращает false, если запись пользователя в чате не найдена (data inconsistency).
//
// timer_start_time здесь обнуляется в NULL — это «чистая» точка перед запуском таймера.
// Сам старт таймера (запись timer_start_time = NOW + регистрация горутин дней 5/6/7/8) делает
// paywallDeliverAccessAfterPayment сразу после этого UPDATE: окно неактивности отсчитываем
// с момента подтверждения оплаты, а не с первого открытия мини-аппа (см. требование пользователя
// «при оплате сразу включается таймер»).
func (d *Database) ReactivateReturnedUser(userID, chatID int64, username string) (bool, error) {
	const q = `
		UPDATE training_state
		SET is_deleted = FALSE,
		    achievement_count = 0,
		    has_training_done = FALSE,
		    has_sick_leave = FALSE,
		    has_healthy = FALSE,
		    timer_start_time = NULL,
		    returned_at = (NOW() AT TIME ZONE 'Europe/Moscow'),
		    return_count = COALESCE(return_count, 0) + 1,
		    username = CASE WHEN NULLIF($3, '') IS NULL THEN username ELSE $3 END,
		    updated_at = $4
		WHERE user_id = $1 AND chat_id = $2
	`
	now := utils.FormatMoscowTime(utils.GetMoscowTime())
	res, err := d.db.Exec(q, userID, chatID, strings.TrimSpace(username), now)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n == 1 {
		return true, nil
	}

	// Если записи нет вообще (первый платный вход), создаём профиль в активном состоянии.
	const insertQ = `
		INSERT INTO training_state (
			user_id, username, chat_id, streak_days, max_streak_days, cups_earned,
			last_message, has_training_done, has_sick_leave, has_healthy, is_deleted,
			timer_start_time, timezone_offset_from_moscow, achievement_count, return_count,
			returned_at, created_at, updated_at
		) VALUES (
			$1, NULLIF($2, ''), $3, 0, 0, 0,
			'', FALSE, FALSE, FALSE, FALSE,
			NULL, 0, 0, 1,
			(NOW() AT TIME ZONE 'Europe/Moscow'), $4, $4
		)
	`
	if _, err := d.db.Exec(insertQ, userID, strings.TrimSpace(username), chatID, now); err != nil {
		return false, err
	}
	return true, nil
}

// RestoreDeletedUserWithProgress — админ-восстановление удалённого юзера «с достижениями»:
// в отличие от ReactivateReturnedUser, НЕ обнуляет achievement_count и возвращает кубки из
// снапшота cups_at_deletion (max_streak_days и так пожизненный). Текущий стрик — с нуля
// (его надо набрать заново), таймер — чистый старт (его выставит startTimer). Возвращает
// false, если активной (или удалённой) записи юзера в чате нет.
func (d *Database) RestoreDeletedUserWithProgress(userID, chatID int64, username string) (bool, error) {
	const q = `
		UPDATE training_state
		SET is_deleted        = FALSE,
		    cups_earned       = GREATEST(cups_earned, COALESCE(cups_at_deletion, 0)),
		    streak_days       = 0,
		    has_training_done = FALSE,
		    has_sick_leave    = FALSE,
		    has_healthy       = FALSE,
		    sick_leave_start_time = NULL,
		    sick_leave_end_time   = NULL,
		    sick_time         = NULL,
		    timer_start_time  = NULL,
		    returned_at       = (NOW() AT TIME ZONE 'Europe/Moscow'),
		    return_count      = COALESCE(return_count, 0) + 1,
		    username          = CASE WHEN NULLIF($3, '') IS NULL THEN username ELSE $3 END,
		    updated_at        = $4
		WHERE user_id = $1 AND chat_id = $2
	`
	now := utils.FormatMoscowTime(utils.GetMoscowTime())
	res, err := d.db.Exec(q, userID, chatID, strings.TrimSpace(username), now)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

func (d *Database) GetUserReturnCount(userID, chatID int64) (int, error) {
	const q = `SELECT COALESCE(return_count, 0) FROM training_state WHERE user_id = $1 AND chat_id = $2`
	var cnt int
	if err := d.db.QueryRow(q, userID, chatID).Scan(&cnt); err != nil {
		return 0, err
	}
	return cnt, nil
}

// GetTopUsers получает топ пользователей по кубкам
func (d *Database) GetTopUsers(chatID int64, limit int) ([]*domain.MessageLog, error) {
	query := `
		SELECT user_id, username, chat_id, streak_days, max_streak_days, cups_earned, last_training_date, last_message, has_training_done, has_sick_leave, has_healthy, is_deleted, is_exempt_from_deletion,
		       timer_start_time, sick_leave_start_time, sick_leave_end_time, sick_time, timezone_offset_from_moscow, sick_approval_pending, sick_approval_deadline, sick_approval_message_id, created_at, updated_at
		FROM training_state 
		WHERE chat_id = $1 AND cups_earned > 0 AND is_deleted = FALSE
		ORDER BY cups_earned DESC, last_message DESC
		LIMIT $2
	`

	rows, err := d.db.Query(query, chatID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*domain.MessageLog
	for rows.Next() {
		var msg domain.MessageLog
		err := rows.Scan(
			&msg.UserID, &msg.Username, &msg.ChatID, &msg.StreakDays, &msg.MaxStreakDays, &msg.CupsEarned, &msg.LastTrainingDate, &msg.LastMessage, &msg.HasTrainingDone,
			&msg.HasSickLeave, &msg.HasHealthy, &msg.IsDeleted, &msg.IsExemptFromDeletion, &msg.TimerStartTime, &msg.SickLeaveStartTime, &msg.SickLeaveEndTime, &msg.SickTime, &msg.TimezoneOffsetFromMoscow,
			&msg.SickApprovalPending, &msg.SickApprovalDeadline, &msg.SickApprovalMessageID, &msg.CreatedAt, &msg.UpdatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, &msg)
	}

	return users, nil
}

// GetAllUsersWithTimers получает всех пользователей с активными таймерами
func (d *Database) GetAllUsersWithTimers() ([]*domain.MessageLog, error) {
	query := `
		SELECT user_id, username, chat_id, streak_days, max_streak_days, cups_earned, last_training_date, last_message, has_training_done, has_sick_leave, has_healthy, is_deleted, is_exempt_from_deletion,
		       timer_start_time, sick_leave_start_time, sick_leave_end_time, sick_time, timezone_offset_from_moscow, sick_approval_pending, sick_approval_deadline, sick_approval_message_id, created_at, updated_at
		FROM training_state 
		WHERE timer_start_time IS NOT NULL AND is_deleted = FALSE
		ORDER BY timer_start_time ASC
	`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*domain.MessageLog
	for rows.Next() {
		var msg domain.MessageLog
		err := rows.Scan(
			&msg.UserID, &msg.Username, &msg.ChatID, &msg.StreakDays, &msg.MaxStreakDays, &msg.CupsEarned, &msg.LastTrainingDate, &msg.LastMessage, &msg.HasTrainingDone,
			&msg.HasSickLeave, &msg.HasHealthy, &msg.IsDeleted, &msg.IsExemptFromDeletion, &msg.TimerStartTime, &msg.SickLeaveStartTime, &msg.SickLeaveEndTime, &msg.SickTime, &msg.TimezoneOffsetFromMoscow,
			&msg.SickApprovalPending, &msg.SickApprovalDeadline, &msg.SickApprovalMessageID, &msg.CreatedAt, &msg.UpdatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, &msg)
	}

	return users, nil
}

// GetPendingSickApprovals возвращает пользователей с ожидающим подтверждением больничного
func (d *Database) GetPendingSickApprovals() ([]*domain.MessageLog, error) {
	query := `
		SELECT user_id, username, chat_id, streak_days, max_streak_days, cups_earned, last_training_date, last_message, has_training_done, has_sick_leave, has_healthy, is_deleted, is_exempt_from_deletion,
		       timer_start_time, sick_leave_start_time, sick_leave_end_time, sick_time, gender, timezone_offset_from_moscow, sick_approval_pending, sick_approval_deadline, sick_approval_message_id, created_at, updated_at
		FROM training_state
		WHERE sick_approval_pending = TRUE AND is_deleted = FALSE
	`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var approvals []*domain.MessageLog
	for rows.Next() {
		var msg domain.MessageLog
		err := rows.Scan(
			&msg.UserID, &msg.Username, &msg.ChatID, &msg.StreakDays, &msg.MaxStreakDays, &msg.CupsEarned, &msg.LastTrainingDate, &msg.LastMessage, &msg.HasTrainingDone,
			&msg.HasSickLeave, &msg.HasHealthy, &msg.IsDeleted, &msg.IsExemptFromDeletion, &msg.TimerStartTime, &msg.SickLeaveStartTime, &msg.SickLeaveEndTime, &msg.SickTime, &msg.Gender, &msg.TimezoneOffsetFromMoscow,
			&msg.SickApprovalPending, &msg.SickApprovalDeadline, &msg.SickApprovalMessageID, &msg.CreatedAt, &msg.UpdatedAt)
		if err != nil {
			return nil, err
		}
		approvals = append(approvals, &msg)
	}

	return approvals, nil
}

// GetRecentUserMessages получает последние сообщения ВСЕХ участников чата для контекста
// Для всех типов чатов использует полную историю из user_messages всех пользователей
// Включает все сообщения, включая ответы бота (ai_reply) для полного контекста диалога
func (d *Database) GetRecentUserMessages(userID, chatID int64, limit int) ([]string, error) {
	// Для всех типов чатов используем полную историю из user_messages всех пользователей
	// Получаем последние сообщения ВСЕХ участников чата, включая ответы бота
	query := `
		SELECT message_text
		FROM user_messages
		WHERE chat_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := d.db.Query(query, chatID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []string
	for rows.Next() {
		var msgText string
		if err := rows.Scan(&msgText); err != nil {
			continue
		}
		if msgText != "" && strings.TrimSpace(msgText) != "" {
			messages = append(messages, msgText)
		}
	}

	// Разворачиваем список для хронологического порядка (от старых к новым)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// GetChatContext получает контекст чата: информацию о других участниках и их последних сообщениях
// Возвращает []*domain.MessageLog для обратной совместимости
// Теперь всегда использует user_messages для получения полного контекста всех пользователей
func (d *Database) GetChatContext(chatID int64, excludeUserID int64, limit int) ([]*domain.MessageLog, error) {
	// Получаем последние сообщения всех пользователей из user_messages
	// Используем подзапрос для получения последнего сообщения каждого пользователя
	query := `
		SELECT user_id, username, chat_id, message_text as last_message
		FROM (
			SELECT user_id, username, chat_id, message_text,
			       ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at DESC) as rn
			FROM user_messages
			WHERE chat_id = $1 AND user_id != $2
		) AS ranked
		WHERE rn = 1
		ORDER BY user_id DESC
		LIMIT $3
	`

	rows, err := d.db.Query(query, chatID, excludeUserID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*domain.MessageLog
	for rows.Next() {
		var msg domain.MessageLog
		err := rows.Scan(&msg.UserID, &msg.Username, &msg.ChatID, &msg.LastMessage)
		if err != nil {
			continue
		}
		// Получаем полную информацию о пользователе из training_state
		fullUserLog, err := d.GetMessageLog(msg.UserID, chatID)
		if err == nil {
			// Копируем последнее сообщение из user_messages
			fullUserLog.LastMessage = msg.LastMessage
			users = append(users, fullUserLog)
		} else {
			// Если нет записи в training_state, создаем минимальную
			msg.ChatID = chatID
			users = append(users, &msg)
		}
	}

	return users, nil
}

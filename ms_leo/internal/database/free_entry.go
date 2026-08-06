package database

import (
	"fmt"
	"strings"

	"leo-bot/internal/utils"
)

// EnsureFreeEntryProfile — профиль стаи для бесплатного входа (PAYWALL_ENTRY_FREE).
// Раньше строку training_state создавала только оплата (paywallDeliverAccessAfterPayment
// → ReactivateReturnedUser); теперь новичок заходит без платежа, и строку нужно создать здесь.
//
// timer_start_time остаётся NULL: таймер неактивности и карточку «вступил в стаю» ставит
// вызывающий (EnsureMiniAppOnboarding), как и для оплативших.
//
// ON CONFLICT DO NOTHING критичен: у кикнутого за неактивность строка уже есть с
// is_deleted = TRUE, и её нельзя ни перезаписать, ни обнулить — возврат остаётся платным.
// created=false означает «строка уже была»; активна она или нет, проверяет вызывающий.
func (d *Database) EnsureFreeEntryProfile(userID, packChatID int64, username string) (created bool, err error) {
	const q = `
		INSERT INTO training_state (
			user_id, username, chat_id,
			streak_days, max_streak_days, cups_earned,
			last_message, has_training_done, has_sick_leave, has_healthy, is_deleted,
			timer_start_time, timezone_offset_from_moscow, achievement_count, return_count,
			created_at, updated_at
		) VALUES (
			$1, NULLIF($2, ''), $3,
			0, 0, 0,
			'', FALSE, FALSE, FALSE, FALSE,
			NULL, 0, 0, 0,
			$4, $4
		)
		ON CONFLICT (user_id, chat_id) DO NOTHING`

	now := utils.FormatMoscowTime(utils.GetMoscowTime())
	res, execErr := d.db.Exec(q, userID, strings.TrimSpace(username), packChatID, now)
	if execErr != nil {
		return false, fmt.Errorf("free entry training_state user=%d chat=%d: %w", userID, packChatID, execErr)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

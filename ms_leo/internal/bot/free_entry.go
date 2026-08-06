package bot

import (
	"database/sql"
	"errors"
	"strings"
)

// EnsureFreeEntryFromStart — бесплатный вход по /start в личке: профиль стаи, таймер
// неактивности и карточка «в стае новый участник» в ленте.
//
// Раньше всё это делала оплата (paywallDeliverAccessAfterPayment): таймер стартовал в момент
// подтверждения платежа. Бесплатный вход сохраняет ту же точку отсчёта — /start, а не первое
// открытие мини-аппа, иначе пользователь мог бы «жить вечно» без отчётов, просто не заходя.
//
// Личное приветствие здесь не отправляем: /start уже прислал свой текст с кнопкой мини-аппа
// (дублировать «Привет, я Лео» вторым сообщением не нужно).
//
// Выбывшего за неактивность функция не трогает — его возврат остаётся платным.
func (b *Bot) EnsureFreeEntryFromStart(userID int64, username string) {
	if !b.freeEntryActive() || userID == 0 {
		return
	}
	chatID := b.config.MonetizedChatID
	if chatID == 0 {
		return
	}
	username = strings.TrimSpace(username)

	ml, err := b.db.GetMessageLogAnyState(userID, chatID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		b.logger.Errorf("free entry /start state user=%d: %v", userID, err)
		return
	}
	if ml != nil {
		if ml.IsDeleted {
			return
		}
		// Профиль уже есть: если таймер шёл — вход состоялся раньше, второй раз не онбордим.
		if ml.TimerStartTime != nil && strings.TrimSpace(*ml.TimerStartTime) != "" {
			return
		}
	} else if _, err := b.db.EnsureFreeEntryProfile(userID, chatID, username); err != nil {
		b.logger.Errorf("free entry /start profile user=%d: %v", userID, err)
		return
	}

	b.startTimer(userID, chatID, username)
	b.savePackJoinMiniappFeed(
		chatID,
		userID,
		username,
		userMessageTypePackJoin,
		packJoinMiniappFeedPublicText(username),
		"",
	)
	invalidateMiniappMenuButtonCache(userID)
	b.applyMiniappMenuButtonForUser(userID)
	b.logger.Infof("free entry granted user=%d chat=%d", userID, chatID)
}

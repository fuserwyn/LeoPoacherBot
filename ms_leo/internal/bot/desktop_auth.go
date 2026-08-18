package bot

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Вход в десктопное приложение Леопарда.
//
// В обычном окне на компьютере объекта Telegram.WebApp нет, поэтому личность
// подтверждается из чата: приложение открывает t.me/<bot>?start=auth_<nonce>,
// человек жмёт «Войти», мы выдаём токен сессии, и дальше приложение шлёт его
// туда же, где мини-апп шлёт initData — сервер различает их по префиксу.

const (
	desktopStartPrefix = "auth_"
	// DesktopTokenPrefix — по нему отличаем токен приложения от настоящего initData.
	DesktopTokenPrefix = "fldesk_"
	desktopLoginTTL    = 10 * time.Minute
)

// ParseDesktopStartPayload — nonce из «auth_<nonce>» или пусто.
func ParseDesktopStartPayload(args string) string {
	arg := strings.TrimSpace(args)
	if !strings.HasPrefix(arg, desktopStartPrefix) {
		return ""
	}
	nonce := strings.TrimSpace(strings.TrimPrefix(arg, desktopStartPrefix))
	if nonce == "" || len(nonce) > 64 {
		return ""
	}
	for _, r := range nonce {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
		if !ok {
			return ""
		}
	}
	return nonce
}

// handleDesktopLoginStart — /start auth_<nonce>: спрашиваем подтверждение.
func (b *Bot) handleDesktopLoginStart(msg *tgbotapi.Message, nonce string) {
	if msg == nil || msg.From == nil || b.db == nil {
		return
	}
	if err := b.db.DesktopLoginStart(nonce, desktopLoginTTL); err != nil {
		b.logger.Warnf("desktop login start: %v", err)
		b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Не получилось начать вход. Попробуй ещё раз из приложения."))
		return
	}
	out := tgbotapi.NewMessage(msg.Chat.ID,
		"💻 Вход в приложение Fat Leopard на компьютере.\n\n"+
			"Если это ты открыл приложение — подтверди. Ссылка живёт 10 минут.")
	out.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Войти", "desk_login_"+nonce),
			tgbotapi.NewInlineKeyboardButtonData("❌ Это не я", "desk_deny_"+nonce),
		),
	)
	b.api.Send(out)
}

// handleDesktopLoginCallback — кнопки подтверждения. true, если это наш callback.
func (b *Bot) handleDesktopLoginCallback(callback *tgbotapi.CallbackQuery) bool {
	if callback == nil || callback.From == nil || callback.Message == nil {
		return false
	}
	data := callback.Data
	switch {
	case strings.HasPrefix(data, "desk_login_"):
		nonce := strings.TrimPrefix(data, "desk_login_")
		token, err := newDesktopToken()
		if err != nil {
			b.answerDesktopCallback(callback, "Не вышло, попробуй ещё раз")
			return true
		}
		ok, err := b.db.DesktopLoginConfirm(nonce, callback.From.ID, token)
		if err != nil {
			b.logger.Warnf("desktop login confirm: %v", err)
			b.answerDesktopCallback(callback, "Ошибка входа")
			return true
		}
		if !ok {
			// Чужая или протухшая попытка — отказ, а не молчаливая выдача.
			b.answerDesktopCallback(callback, "Ссылка устарела — открой вход в приложении заново")
			b.editDesktopMessage(callback, "⌛️ Попытка входа устарела. Нажми «Войти» в приложении ещё раз.")
			return true
		}
		b.answerDesktopCallback(callback, "Готово")
		b.editDesktopMessage(callback, "✅ Вход подтверждён — возвращайся в приложение.")
		return true
	case strings.HasPrefix(data, "desk_deny_"):
		b.answerDesktopCallback(callback, "Отменил")
		b.editDesktopMessage(callback, "❌ Вход отклонён. Если это был не ты — всё в порядке, доступ не выдан.")
		return true
	}
	return false
}

func (b *Bot) answerDesktopCallback(callback *tgbotapi.CallbackQuery, text string) {
	b.api.Request(tgbotapi.NewCallback(callback.ID, text))
}

func (b *Bot) editDesktopMessage(callback *tgbotapi.CallbackQuery, text string) {
	edit := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, text)
	b.api.Send(edit)
}

func newDesktopToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("token: %w", err)
	}
	return DesktopTokenPrefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

// DesktopSessionOwner — чей токен приложения. 0 — сессия неизвестна или отозвана.
func (b *Bot) DesktopSessionOwner(token string) (int64, error) {
	if b == nil || b.db == nil {
		return 0, nil
	}
	return b.db.DesktopSessionOwner(token)
}

// DesktopLoginPoll — приложение забирает токен после подтверждения в чате.
func (b *Bot) DesktopLoginPoll(nonce string) (status string, userID int64, token string, err error) {
	if b == nil || b.db == nil {
		return "expired", 0, "", nil
	}
	return b.db.DesktopLoginPoll(nonce)
}

// DesktopSessionRevoke — выход из приложения.
func (b *Bot) DesktopSessionRevoke(token string) error {
	if b == nil || b.db == nil {
		return nil
	}
	return b.db.DesktopSessionRevoke(token)
}

// DesktopUserLabel — имя и ник для приветствия в приложении.
func (b *Bot) DesktopUserLabel(userID int64) (string, string) {
	if b == nil || b.db == nil || userID == 0 {
		return "", ""
	}
	people, err := b.db.AdminPeopleByIDs(b.adminPackChatID(), []int64{userID})
	if err != nil || len(people) == 0 {
		return "", ""
	}
	return people[0].DisplayName, people[0].Username
}

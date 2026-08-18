package miniappapi

import (
	"strings"
	"testing"

	"leo-bot/internal/bot"
)

// Токен приложения не должен пролезать как обычный initData и наоборот:
// проверка живёт в одном месте, но различает два разных способа входа.
func TestDesktopTokenPrefixIsDistinct(t *testing.T) {
	if !strings.HasPrefix(bot.DesktopTokenPrefix+"abc", bot.DesktopTokenPrefix) {
		t.Fatal("префикс токена приложения не опознаётся")
	}
	// Подписанный initData Telegram — это query-строка, префикса приложения в ней нет.
	const telegramLike = "auth_date=1&query_id=A&user=%7B%22id%22%3A1%7D&hash=deadbeef"
	if strings.HasPrefix(telegramLike, bot.DesktopTokenPrefix) {
		t.Fatal("initData Telegram принят за токен приложения")
	}
}

// Без бота (сервер поднят не полностью) десктопный токен не должен считаться
// валидным: иначе запрос пройдёт как «свой» без всякой проверки.
func TestDesktopTokenRejectedWithoutBot(t *testing.T) {
	s := &Server{}
	if err := s.validateInit(bot.DesktopTokenPrefix + "whatever"); err == nil {
		t.Fatal("токен приложения принят без проверки сессии")
	}
	if _, err := s.parseInit(bot.DesktopTokenPrefix + "whatever"); err == nil {
		t.Fatal("данные из токена приложения разобраны без проверки сессии")
	}
}

package miniappapi

import (
	"fmt"
	"strings"
	"time"

	"leo-bot/internal/bot"

	initdata "github.com/telegram-mini-apps/init-data-golang"
)

// Токен десктопного приложения там же, где и initData.
//
// В обычном окне на компьютере подписанного initData не существует, поэтому
// приложение шлёт токен сессии (bot/desktop_auth.go). Ручек, проверяющих
// initData, полсотни, и каждая делала это сама — если научить только общую
// authMiniapp, приложение авторизуется в админке, но не получает ни профиля, ни
// ленты. Поэтому проверка и разбор живут здесь, а ручки зовут их.

// validateInit — подпись Telegram либо живая сессия приложения.
func (s *Server) validateInit(raw string) error {
	if strings.HasPrefix(raw, bot.DesktopTokenPrefix) {
		if s.bot == nil {
			return fmt.Errorf("desktop session: bot unavailable")
		}
		userID, err := s.bot.DesktopSessionOwner(raw)
		if err != nil {
			return fmt.Errorf("desktop session: %w", err)
		}
		if userID == 0 {
			return fmt.Errorf("desktop session unknown or revoked")
		}
		return nil
	}
	return initdata.Validate(raw, s.token, 24*time.Hour)
}

// parseInit — данные о человеке из initData либо из сессии приложения.
// У десктопной сессии есть только id: чата нет, и проверки на совпадение чата
// стаи такой запрос проходят (Chat.ID == 0).
func (s *Server) parseInit(raw string) (initdata.InitData, error) {
	if strings.HasPrefix(raw, bot.DesktopTokenPrefix) {
		if s.bot == nil {
			return initdata.InitData{}, fmt.Errorf("desktop session: bot unavailable")
		}
		userID, err := s.bot.DesktopSessionOwner(raw)
		if err != nil {
			return initdata.InitData{}, err
		}
		if userID == 0 {
			return initdata.InitData{}, fmt.Errorf("desktop session unknown or revoked")
		}
		name, username := s.bot.DesktopUserLabel(userID)
		return initdata.InitData{User: initdata.User{
			ID:        userID,
			FirstName: name,
			Username:  strings.TrimPrefix(username, "@"),
		}}, nil
	}
	return initdata.Parse(raw)
}

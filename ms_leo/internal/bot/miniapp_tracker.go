package bot

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	initdata "github.com/telegram-mini-apps/init-data-golang"
)

// Трекер задач Леопарда живёт в MyVibeLab: там доска, спринты и агент, который
// эти задачи выполняет. Заводить админам отдельный аккаунт не нужно — админка
// мини-аппа подписывает ссылку общим секретом (BOARD_SSO_SECRET), а MyVibeLab
// по подписи пускает человека РОВНО в доску нашего репозитория: ни IDE, ни
// деплоя, ни чужих проектов там не будет.
//
// Ссылка живёт 5 минут: она уходит в браузер и светится в истории, поэтому
// долгий доступ выдаёт уже сам MyVibeLab — сессией на своей стороне.
const trackerLinkTTL = 5 * time.Minute

// ErrTrackerNotConfigured — не задан секрет или адрес: кнопку показывать не из чего.
var ErrTrackerNotConfigured = fmt.Errorf("tracker not configured")

// MiniappTrackerLink — подписанная ссылка на доску для админа мини-аппа.
// Права проверяем тем же requireMiniappAdmin, что и остальные админ-ручки:
// отдельного списка для трекера нет, админ стаи и есть админ доски.
func (b *Bot) MiniappTrackerLink(viewerUserID int64, initD initdata.InitData) (string, error) {
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return "", err
	}
	secret := strings.TrimSpace(b.config.BoardSecret)
	base := strings.TrimRight(strings.TrimSpace(b.config.BoardURL), "/")
	repo := strings.TrimSpace(b.config.BoardRepo)
	if secret == "" || base == "" || repo == "" {
		return "", ErrTrackerNotConfigured
	}
	name := strings.TrimSpace(initD.User.FirstName + " " + initD.User.LastName)
	if name == "" {
		name = initD.User.Username
	}
	payload, err := json.Marshal(map[string]any{
		"k": "link",
		"r": repo,
		"u": viewerUserID,
		"n": name,
		"e": time.Now().Add(trackerLinkTTL).Unix(),
	})
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(encoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s/board?t=%s.%s", base, encoded, signature), nil
}

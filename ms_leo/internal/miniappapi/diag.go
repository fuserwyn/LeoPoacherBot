package miniappapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"unicode"

	initdata "github.com/telegram-mini-apps/init-data-golang"
)

// sanitizeDiagField чистит контролируемое клиентом значение перед записью в лог:
// убирает управляющие символы (в т.ч. переводы строк — защита от log injection)
// и ограничивает длину.
func sanitizeDiagField(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

// handlePostDiagInitSource — прод-диагностика источника initData у реальных юзеров.
// Клиент шлёт сюда один beacon: source (webapp/hash/session/none), платформу, версию
// Telegram и сам initData (если есть). Сервер логирует распределение и валидность —
// так подтверждаем причину пустого initData фактами. Тело всегда 204, без БД.
func (s *Server) handlePostDiagInitSource(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	var body struct {
		Source    string `json:"source"`
		HasInit   bool   `json:"has_init"`
		InitData  string `json:"init_data"`
		Platform  string `json:"platform"`
		TgVersion string `json:"tg_version"`
		TriesMs   int    `json:"tries_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Если initData передан — проверяем подпись, чтобы в логах было видно валиден ли он и чей.
	valid := false
	var userID int64
	if body.InitData != "" && s.token != "" {
		if err := initdata.Validate(body.InitData, s.token, 24*time.Hour); err == nil {
			valid = true
			if parsed, perr := initdata.Parse(body.InitData); perr == nil {
				userID = parsed.User.ID
			}
		}
	}

	s.logger.Infof(
		"miniapp initdata diag: source=%s has_init=%t valid=%t user=%d platform=%s tg_version=%s tries_ms=%d",
		sanitizeDiagField(body.Source), body.HasInit, valid, userID,
		sanitizeDiagField(body.Platform), sanitizeDiagField(body.TgVersion), body.TriesMs,
	)
	w.WriteHeader(http.StatusNoContent)
}

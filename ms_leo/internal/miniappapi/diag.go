package miniappapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"

	"leo-bot/internal/database"

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

// handlePostLeoCommentDisplayed — клиент сообщает, что юзер увидел комментарий Лео
// в треде (analytics_BT_v1 §4, leo_comment_displayed). Валидируем initData, пишем
// событие с идемпотентностью по (user, reply) — один показ на зрителя считается раз.
func (s *Server) handlePostLeoCommentDisplayed(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	if s.bot == nil || s.token == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var body struct {
		InitData      string `json:"init_data"`
		ThreadReplyID int64  `json:"thread_reply_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.InitData == "" || body.ThreadReplyID <= 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := initdata.Validate(body.InitData, s.token, 24*time.Hour); err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	parsed, err := initdata.Parse(body.InitData)
	if err != nil || parsed.User.ID == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.bot.TrackEvent(database.AnalyticsEvent{
		Name:           database.EventLeoCommentDisplayed,
		UserID:         parsed.User.ID,
		TelegramID:     parsed.User.ID,
		Payload:        map[string]any{"thread_reply_id": body.ThreadReplyID},
		IdempotencyKey: fmt.Sprintf("leo_comment_displayed:%d:%d", parsed.User.ID, body.ThreadReplyID),
	})
	w.WriteHeader(http.StatusNoContent)
}

// handlePostAnalyticsEvent — клиентские события воронок (§3/§4), которые сервер не
// может детектировать сам: miniapp_opened и workout_log_started. Имя строго из
// белого списка (никакого произвольного client-trust), initData валидируется.
func (s *Server) handlePostAnalyticsEvent(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	if s.bot == nil || s.token == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var body struct {
		InitData   string `json:"init_data"`
		Event      string `json:"event"`
		EntryPoint string `json:"entry_point"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.InitData == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	// Белый список клиентских событий.
	var name string
	switch body.Event {
	case "miniapp_opened":
		name = database.EventMiniappOpened
	case "workout_log_started":
		name = database.EventWorkoutLogStarted
	default:
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := initdata.Validate(body.InitData, s.token, 24*time.Hour); err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	parsed, err := initdata.Parse(body.InitData)
	if err != nil || parsed.User.ID == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	payload := map[string]any{}
	if ep := sanitizeDiagField(body.EntryPoint); ep != "" {
		payload["entry_point"] = ep
	}
	s.bot.TrackEvent(database.AnalyticsEvent{
		Name:       name,
		UserID:     parsed.User.ID,
		TelegramID: parsed.User.ID,
		Payload:    payload,
	})
	w.WriteHeader(http.StatusNoContent)
}

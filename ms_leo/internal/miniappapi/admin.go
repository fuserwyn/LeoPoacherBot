package miniappapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"leo-bot/internal/bot"
)

func (s *Server) writeAdminErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, bot.ErrMiniAppChatMismatch):
		s.jsonErr(w, http.StatusConflict, "chat_mismatch")
	case errors.Is(err, bot.ErrPackFeedForbidden):
		s.jsonErr(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, bot.ErrAdminNotFound):
		s.jsonErr(w, http.StatusNotFound, "not_found")
	case errors.Is(err, bot.ErrAdminActionInvalid):
		s.jsonErr(w, http.StatusBadRequest, "invalid_action")
	case errors.Is(err, bot.ErrTrackerNotConfigured):
		s.jsonErr(w, http.StatusServiceUnavailable, "tracker_not_configured")
	default:
		if s.jsonModerationErr(w, err) {
			return
		}
		msg := strings.TrimSpace(err.Error())
		if msg != "" && !strings.Contains(msg, "sql:") && !strings.Contains(strings.ToLower(msg), "pq:") {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "admin_error", "message": msg})
			return
		}
		s.logger.Errorf("miniapp admin: %v", err)
		s.jsonErr(w, http.StatusInternalServerError, "admin_error")
	}
}

func (s *Server) handlePostAdminOverview(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	var body struct {
		InitData string `json:"init_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	overview, err := s.bot.MiniappAdminOverview(parsed.User.ID, parsed)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "overview": overview})
}

// handlePostAdminTrackerLink — одноразовая ссылка на доску трекера MyVibeLab.
// Подпись делаем на сервере: секрет в браузер не отдаём, иначе ссылку смог бы
// собрать кто угодно, кто открыл мини-апп.
func (s *Server) handlePostAdminTrackerLink(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	var body struct {
		InitData string `json:"init_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	link, err := s.bot.MiniappTrackerLink(parsed.User.ID, parsed)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "link": link})
}

func (s *Server) handlePostAdminSupportInbox(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	var body struct {
		InitData string `json:"init_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	items, err := s.bot.MiniappAdminSupportInbox(parsed.User.ID, parsed)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "conversations": items})
}

func (s *Server) handlePostAdminSupportThread(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	var body struct {
		InitData     string `json:"init_data"`
		TargetUserID int64  `json:"target_user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	items, err := s.bot.MiniappAdminSupportThread(parsed.User.ID, parsed, body.TargetUserID)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "messages": items})
}

func (s *Server) handlePostAdminSupportReply(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	var body struct {
		InitData     string `json:"init_data"`
		TargetUserID int64  `json:"target_user_id"`
		Text         string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	text := strings.TrimSpace(body.Text)
	if text == "" {
		s.jsonErr(w, http.StatusBadRequest, "empty_text")
		return
	}
	if utf8.RuneCountInString(text) > maxTextRunes {
		s.jsonErr(w, http.StatusBadRequest, "text_too_long")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	if err := s.bot.MiniappAdminSupportReply(parsed.User.ID, parsed, body.TargetUserID, text); err != nil {
		s.writeAdminErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *Server) handlePostAdminReports(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	var body struct {
		InitData string `json:"init_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	items, err := s.bot.MiniappAdminReports(parsed.User.ID, parsed)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "reports": items})
}

func (s *Server) handlePostAdminReportAction(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	var body struct {
		InitData string `json:"init_data"`
		ReportID int64  `json:"report_id"`
		Action   string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	if err := s.bot.MiniappAdminReportAction(parsed.User.ID, parsed, body.ReportID, body.Action); err != nil {
		s.writeAdminErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *Server) handlePostAdminHidden(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	var body struct {
		InitData string `json:"init_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	items, err := s.bot.MiniappAdminHidden(parsed.User.ID, parsed)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "items": items})
}

func (s *Server) handlePostAdminUnhide(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	var body struct {
		InitData string `json:"init_data"`
		Kind     string `json:"kind"`
		ID       int64  `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	if err := s.bot.MiniappAdminUnhide(parsed.User.ID, parsed, body.Kind, body.ID); err != nil {
		s.writeAdminErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *Server) handlePostAdminUsers(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	var body struct {
		InitData string `json:"init_data"`
		Query    string `json:"query"`
		Offset   int    `json:"offset"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	var (
		items []bot.MiniappAdminUserRow
		err   error
	)
	if strings.TrimSpace(body.Query) != "" {
		items, err = s.bot.MiniappAdminSearchUsers(parsed.User.ID, parsed, body.Query)
	} else {
		items, err = s.bot.MiniappAdminListUsers(parsed.User.ID, parsed, body.Offset, 20)
	}
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "users": items})
}

func (s *Server) handlePostAdminUserCard(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	var body struct {
		InitData     string `json:"init_data"`
		TargetUserID int64  `json:"target_user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	card, err := s.bot.MiniappAdminUserCard(parsed.User.ID, parsed, body.TargetUserID)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "user": card})
}

func (s *Server) handlePostAdminUserAction(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	var body struct {
		InitData     string `json:"init_data"`
		TargetUserID int64  `json:"target_user_id"`
		Action       string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	if err := s.bot.MiniappAdminUserAction(parsed.User.ID, parsed, body.TargetUserID, body.Action); err != nil {
		s.writeAdminErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *Server) handlePostAdminPublish(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	var body struct {
		InitData string `json:"init_data"`
		Author   string `json:"author"`
		Text     string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	text := strings.TrimSpace(body.Text)
	if text == "" {
		s.jsonErr(w, http.StatusBadRequest, "empty_text")
		return
	}
	if utf8.RuneCountInString(text) > maxTextRunes {
		s.jsonErr(w, http.StatusBadRequest, "text_too_long")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	if err := s.bot.MiniappAdminPublishPost(parsed.User.ID, parsed, body.Author, text); err != nil {
		s.writeAdminErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *Server) handlePostAdminPaywallPrice(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	var body struct {
		InitData string `json:"init_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	price, err := s.bot.MiniappAdminPaywallPrice(parsed.User.ID, parsed)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "price": price})
}

func (s *Server) handlePostAdminPaywallPriceSet(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	var body struct {
		InitData  string `json:"init_data"`
		AmountRub int    `json:"amount_rub"`
		Reset     bool   `json:"reset"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	if err := s.bot.MiniappAdminSetPaywallPrice(parsed.User.ID, parsed, body.AmountRub, body.Reset); err != nil {
		s.writeAdminErr(w, err)
		return
	}
	price, err := s.bot.MiniappAdminPaywallPrice(parsed.User.ID, parsed)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "price": price})
}

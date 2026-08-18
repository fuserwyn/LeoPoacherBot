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

// handlePostAdminTracker — единственная дверь мини-аппа к доске задач в
// MyVibeLab: op из белого списка, всё остальное делает bot.MiniappTrackerCall.
// Ответ трекера отдаём как есть — формат карточек и статусов у нас общий.
func (s *Server) handlePostAdminTracker(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	var body struct {
		InitData string         `json:"init_data"`
		Op       string         `json:"op"`
		TaskID   int64          `json:"task_id"`
		Payload  map[string]any `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	data, err := s.bot.MiniappTrackerCall(parsed.User.ID, parsed, body.Op, body.TaskID, body.Payload)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": data})
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

// --- Разделы, переехавшие из чат-админки (bot/miniapp_admin_ops.go) --------

func (s *Server) handlePostAdminAnalytics(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InitData string `json:"init_data"`
		Days     int    `json:"days"`
	}
	corsWriteHeaders(w, r)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	data, err := s.bot.MiniappAdminAnalyticsData(parsed.User.ID, parsed, body.Days)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	s.writeAdminOK(w, map[string]any{"analytics": data})
}

func (s *Server) handlePostAdminVisits(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InitData string `json:"init_data"`
	}
	corsWriteHeaders(w, r)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	tables, err := s.bot.MiniappAdminVisits(parsed.User.ID, parsed)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	s.writeAdminOK(w, map[string]any{"tables": tables})
}

func (s *Server) handlePostAdminPayments(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InitData string `json:"init_data"`
		Offset   int    `json:"offset"`
		Limit    int    `json:"limit"`
	}
	corsWriteHeaders(w, r)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	data, err := s.bot.MiniappAdminPaymentsPage(parsed.User.ID, parsed, body.Offset, body.Limit)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	s.writeAdminOK(w, map[string]any{"payments": data})
}

func (s *Server) handlePostAdminAdmins(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InitData string `json:"init_data"`
	}
	corsWriteHeaders(w, r)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	people, err := s.bot.MiniappAdminAdminsList(parsed.User.ID, parsed)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	s.writeAdminOK(w, map[string]any{"admins": people})
}

func (s *Server) handlePostAdminAdminsAdd(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InitData string `json:"init_data"`
		Query    string `json:"query"`
	}
	corsWriteHeaders(w, r)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	id, err := s.bot.MiniappAdminAddAdmin(parsed.User.ID, parsed, body.Query)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	s.writeAdminOK(w, map[string]any{"user_id": id})
}

func (s *Server) handlePostAdminAdminsRemove(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InitData string `json:"init_data"`
		UserID   int64  `json:"user_id"`
	}
	corsWriteHeaders(w, r)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	if err := s.bot.MiniappAdminRemoveAdmin(parsed.User.ID, parsed, body.UserID); err != nil {
		s.writeAdminErr(w, err)
		return
	}
	s.writeAdminOK(w, map[string]any{})
}

func (s *Server) handlePostAdminScheduled(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InitData string `json:"init_data"`
	}
	corsWriteHeaders(w, r)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	posts, err := s.bot.MiniappAdminScheduledPosts(parsed.User.ID, parsed)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	s.writeAdminOK(w, map[string]any{"posts": posts})
}

func (s *Server) handlePostAdminScheduledAdd(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InitData string `json:"init_data"`
		Author   string `json:"author"`
		Text     string `json:"text"`
		At       string `json:"at"`
	}
	corsWriteHeaders(w, r)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	id, err := s.bot.MiniappAdminSchedulePost(parsed.User.ID, parsed, body.Author, body.Text, body.At)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	s.writeAdminOK(w, map[string]any{"id": id})
}

func (s *Server) handlePostAdminScheduledCancel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InitData string `json:"init_data"`
		ID       int64  `json:"id"`
	}
	corsWriteHeaders(w, r)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	if err := s.bot.MiniappAdminCancelScheduledPost(parsed.User.ID, parsed, body.ID); err != nil {
		s.writeAdminErr(w, err)
		return
	}
	s.writeAdminOK(w, map[string]any{})
}

func (s *Server) handlePostAdminPoll(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InitData string   `json:"init_data"`
		Question string   `json:"question"`
		Options  []string `json:"options"`
	}
	corsWriteHeaders(w, r)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	if err := s.bot.MiniappAdminPublishPoll(parsed.User.ID, parsed, body.Question, body.Options); err != nil {
		s.writeAdminErr(w, err)
		return
	}
	s.writeAdminOK(w, map[string]any{})
}

func (s *Server) handlePostAdminWipe(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InitData string `json:"init_data"`
		Confirm  bool   `json:"confirm"`
	}
	corsWriteHeaders(w, r)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	// Без confirm только считаем, что удалится: очистка необратима, поэтому
	// подтверждение приходит отдельным запросом.
	if !body.Confirm {
		counts, err := s.bot.MiniappAdminWipeCounts(parsed.User.ID, parsed)
		if err != nil {
			s.writeAdminErr(w, err)
			return
		}
		s.writeAdminOK(w, map[string]any{"counts": counts, "done": false})
		return
	}
	counts, err := s.bot.MiniappAdminWipeExecute(parsed.User.ID, parsed)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	s.writeAdminOK(w, map[string]any{"counts": counts, "done": true})
}

func (s *Server) handlePostAdminAskLeo(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InitData string `json:"init_data"`
		Question string `json:"question"`
	}
	corsWriteHeaders(w, r)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	reply, task, err := s.bot.MiniappAskLeoTask(parsed.User.ID, parsed, body.Question)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	s.writeAdminOK(w, map[string]any{"reply": reply, "task": task})
}

func (s *Server) handlePostAdminTrackerAuthors(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InitData string  `json:"init_data"`
		IDs      []int64 `json:"ids"`
	}
	corsWriteHeaders(w, r)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	people, err := s.bot.MiniappTrackerAuthors(parsed.User.ID, parsed, body.IDs)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	s.writeAdminOK(w, map[string]any{"people": people})
}

func (s *Server) handlePostAdminTrackerAttach(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InitData string `json:"init_data"`
		TaskID   int64  `json:"task_id"`
		Filename string `json:"filename"`
		Mime     string `json:"mime"`
		Data     string `json:"data"`
	}
	corsWriteHeaders(w, r)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	data, err := s.bot.MiniappTrackerAttach(parsed.User.ID, parsed, body.TaskID, body.Filename, body.Mime, body.Data)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	s.writeAdminOK(w, map[string]any{"data": data})
}

func (s *Server) handlePostAdminDBTables(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InitData string `json:"init_data"`
	}
	corsWriteHeaders(w, r)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	tables, err := s.bot.MiniappAdminDBTables(parsed.User.ID, parsed)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	s.writeAdminOK(w, map[string]any{"tables": tables})
}

func (s *Server) handlePostAdminDBTable(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InitData string `json:"init_data"`
		Table    string `json:"table"`
		Limit    int    `json:"limit"`
		Offset   int    `json:"offset"`
		OrderBy  string `json:"order_by"`
		Desc     bool   `json:"desc"`
	}
	corsWriteHeaders(w, r)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	res, err := s.bot.MiniappAdminDBTable(parsed.User.ID, parsed, body.Table, body.Limit, body.Offset, body.OrderBy, body.Desc)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	s.writeAdminOK(w, map[string]any{"result": res})
}

func (s *Server) handlePostAdminDBColumns(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InitData string `json:"init_data"`
		Table    string `json:"table"`
	}
	corsWriteHeaders(w, r)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	cols, err := s.bot.MiniappAdminDBColumns(parsed.User.ID, parsed, body.Table)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	s.writeAdminOK(w, map[string]any{"columns": cols})
}

func (s *Server) handlePostAdminDBQuery(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InitData string `json:"init_data"`
		SQL      string `json:"sql"`
	}
	corsWriteHeaders(w, r)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	res, err := s.bot.MiniappAdminDBQuery(parsed.User.ID, parsed, body.SQL)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	s.writeAdminOK(w, map[string]any{"result": res})
}

func (s *Server) handlePostAdminResources(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InitData string `json:"init_data"`
	}
	corsWriteHeaders(w, r)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	parsed, ok := s.authMiniapp(w, body.InitData)
	if !ok {
		return
	}
	res, err := s.bot.MiniappAdminResources(parsed.User.ID, parsed)
	if err != nil {
		s.writeAdminErr(w, err)
		return
	}
	s.writeAdminOK(w, map[string]any{"resources": res})
}

func (s *Server) writeAdminOK(w http.ResponseWriter, payload map[string]any) {
	if payload == nil {
		payload = map[string]any{}
	}
	payload["ok"] = true
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(payload)
}

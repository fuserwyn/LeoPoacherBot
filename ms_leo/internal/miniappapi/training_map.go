package miniappapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"leo-bot/internal/bot"
	"leo-bot/internal/game/trainingmap"
)

// handlePostTrainingMap — прогресс интерактивной карты тренировок в профиле.
func (s *Server) handlePostTrainingMap(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	if s.bot == nil || s.token == "" {
		s.jsonErr(w, http.StatusServiceUnavailable, "server_unavailable")
		return
	}
	var body struct {
		InitData string `json:"init_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if body.InitData == "" {
		s.jsonErr(w, http.StatusBadRequest, "missing_init_data")
		return
	}
	if err := s.validateInit(body.InitData); err != nil {
		s.jsonErr(w, http.StatusUnauthorized, "invalid_init_data")
		return
	}
	parsed, err := s.parseInit(body.InitData)
	if err != nil {
		s.jsonErr(w, http.StatusBadRequest, "parse_init_data")
		return
	}
	if parsed.User.ID == 0 {
		s.jsonErr(w, http.StatusBadRequest, "user_missing")
		return
	}
	if err := s.bot.AssertMiniAppPackChatAligns(parsed); err != nil {
		if errors.Is(err, bot.ErrMiniAppChatMismatch) {
			s.jsonErr(w, http.StatusConflict, "chat_mismatch")
			return
		}
		s.jsonErr(w, http.StatusInternalServerError, "assert_chat_error")
		return
	}
	packID := s.bot.MonetizedChatID()
	if packID == 0 {
		s.jsonErr(w, http.StatusServiceUnavailable, "pack_not_configured")
		return
	}
	stats := s.bot.GetMiniappProfileStatsForAPI(parsed.User.ID, packID)
	snap := trainingmap.SnapshotFor(stats.WorkoutsTotal)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":             true,
		"workouts_total": snap.WorkoutsTotal,
		"completed":      snap.Completed,
		"remaining":      snap.Remaining,
		"next_index":     snap.NextIndex,
		"lap":            snap.Lap,
		"nodes":          snap.Nodes,
	})
}

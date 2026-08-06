package miniappapi

import (
	"encoding/json"
	"net/http"
	"time"

	initdata "github.com/telegram-mini-apps/init-data-golang"
)

// Донаты из профиля мини-аппа. Суммы приходят из тиров конфига, а не из запроса «как есть»:
// бот проверяет номинал сам (DonateStarsTierAllowed / DonateCardTierAllowed), чтобы клиент
// не мог выставить себе произвольный счёт.

// donateAuth — общая часть всех donate-хендлеров: валидация initData → telegram user id.
// Доступ к стае здесь не проверяем: поддержать проект может и выбывший за неактивность.
func (s *Server) donateAuth(w http.ResponseWriter, r *http.Request, initData string) (int64, bool) {
	if s.bot == nil || s.token == "" {
		s.jsonErr(w, http.StatusServiceUnavailable, "server_unavailable")
		return 0, false
	}
	if initData == "" {
		s.jsonErr(w, http.StatusBadRequest, "missing_init_data")
		return 0, false
	}
	if err := initdata.Validate(initData, s.token, 24*time.Hour); err != nil {
		s.jsonErr(w, http.StatusUnauthorized, "invalid_init_data")
		return 0, false
	}
	parsed, err := initdata.Parse(initData)
	if err != nil {
		s.jsonErr(w, http.StatusBadRequest, "parse_init_data")
		return 0, false
	}
	if parsed.User.ID == 0 {
		s.jsonErr(w, http.StatusBadRequest, "user_missing")
		return 0, false
	}
	return parsed.User.ID, true
}

func (s *Server) handlePostDonateOptions(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	var body struct {
		InitData string `json:"init_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	userID, ok := s.donateAuth(w, r, body.InitData)
	if !ok {
		return
	}
	opts := s.bot.DonateOptionsForUser(userID)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":              true,
		"stars_tiers":     opts.StarsTiers,
		"card_tiers_rub":  opts.CardTiersRub,
		"stars_available": opts.StarsAvailable,
		"card_available":  opts.CardAvailable,
		"completed_count": opts.CompletedCount,
	})
}

// handlePostDonateStars — ссылка на счёт в звёздах для WebApp.openInvoice (оплата внутри мини-аппа).
func (s *Server) handlePostDonateStars(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	var body struct {
		InitData string `json:"init_data"`
		Stars    int    `json:"stars"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	userID, ok := s.donateAuth(w, r, body.InitData)
	if !ok {
		return
	}
	if !s.bot.DonateStarsReady() {
		s.jsonErr(w, http.StatusServiceUnavailable, "donate_stars_unavailable")
		return
	}
	if !s.bot.DonateStarsTierAllowed(body.Stars) {
		s.jsonErr(w, http.StatusBadRequest, "invalid_amount")
		return
	}
	link, donationID, err := s.bot.CreateDonateStarsInvoiceLink(userID, body.Stars)
	if err != nil {
		s.logger.Errorf("donate stars invoice link user=%d stars=%d: %v", userID, body.Stars, err)
		s.jsonErr(w, http.StatusBadGateway, "invoice_link_failed")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":           true,
		"invoice_link": link,
		"donation_id":  donationID,
	})
}

// handlePostDonateCard — платёж ЮKassa: ссылку мини-апп открывает через WebApp.openLink,
// затем опрашивает /donate/status (вебхук ms_payments донаты не обслуживает).
func (s *Server) handlePostDonateCard(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	var body struct {
		InitData string `json:"init_data"`
		Rub      int    `json:"rub"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	userID, ok := s.donateAuth(w, r, body.InitData)
	if !ok {
		return
	}
	if !s.bot.DonateCardReady() {
		s.jsonErr(w, http.StatusServiceUnavailable, "donate_card_unavailable")
		return
	}
	if !s.bot.DonateCardTierAllowed(body.Rub) {
		s.jsonErr(w, http.StatusBadRequest, "invalid_amount")
		return
	}
	url, donationID, err := s.bot.CreateDonateCardPayment(userID, body.Rub)
	if err != nil {
		s.logger.Errorf("donate card payment user=%d rub=%d: %v", userID, body.Rub, err)
		s.jsonErr(w, http.StatusBadGateway, "payment_link_failed")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":               true,
		"confirmation_url": url,
		"donation_id":      donationID,
	})
}

func (s *Server) handlePostDonateStatus(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	var body struct {
		InitData   string `json:"init_data"`
		DonationID int64  `json:"donation_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	userID, ok := s.donateAuth(w, r, body.InitData)
	if !ok {
		return
	}
	if body.DonationID <= 0 {
		s.jsonErr(w, http.StatusBadRequest, "invalid_donation_id")
		return
	}
	status, err := s.bot.DonationStatus(userID, body.DonationID)
	if err != nil {
		s.logger.Warnf("donate status user=%d id=%d: %v", userID, body.DonationID, err)
		s.jsonErr(w, http.StatusNotFound, "donation_not_found")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":     true,
		"status": status,
	})
}

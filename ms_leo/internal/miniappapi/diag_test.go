package miniappapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"leo-bot/internal/logger"

	initdata "github.com/telegram-mini-apps/init-data-golang"
)

func TestHandlePostDiagInitSource(t *testing.T) {
	s := &Server{token: contractToken, logger: logger.New("info")}

	post := func(jsonBody string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/miniapp/diag/init-source", strings.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		s.handlePostDiagInitSource(rec, req)
		return rec.Code
	}

	t.Run("пустой/битый JSON — 204, не падает", func(t *testing.T) {
		if code := post("not json"); code != http.StatusNoContent {
			t.Fatalf("ожидали 204, получили %d", code)
		}
	})

	t.Run("source=none без initData — 204", func(t *testing.T) {
		if code := post(`{"source":"none","has_init":false,"platform":"ios"}`); code != http.StatusNoContent {
			t.Fatalf("ожидали 204, получили %d", code)
		}
	})

	t.Run("валидный initData — 204 (валидность логируется)", func(t *testing.T) {
		payload := "query_id=AAHdiag&user=%7B%22id%22%3A777%2C%22first_name%22%3A%22Leo%22%7D"
		now := time.Now()
		hash, err := initdata.SignQueryString(payload, contractToken, now)
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		raw := fmt.Sprintf("%s&auth_date=%d&hash=%s", payload, now.Unix(), hash)
		body := fmt.Sprintf(`{"source":"hash","has_init":true,"platform":"android","init_data":%q}`, raw)
		if code := post(body); code != http.StatusNoContent {
			t.Fatalf("ожидали 204, получили %d", code)
		}
	})

	t.Run("log injection в source/platform не ломает (санитайзинг)", func(t *testing.T) {
		if code := post("{\"source\":\"none\\nFAKE LOG LINE\",\"platform\":\"x\\ry\"}"); code != http.StatusNoContent {
			t.Fatalf("ожидали 204, получили %d", code)
		}
	})
}

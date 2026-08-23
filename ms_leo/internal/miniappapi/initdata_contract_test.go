package miniappapi

import (
	"fmt"
	"testing"
	"time"

	initdata "github.com/telegram-mini-apps/init-data-golang"
)

// Контракт серверной проверки initData — тот же путь, что в каждом handlePost*:
// initdata.Validate(body.InitData, token, 24h). Тест фиксирует: сервер принимает
// корректно подписанную строку и отвергает пустую/протухшую/подделанную.
//
// Это доказывает, что баг «нужен initData» был НЕ на сервере: пустую строку
// (которую слал клиент) сервер обязан отбивать — лечить надо на клиенте, чтобы
// он вообще доносил подписанный initData. См. miniapp/src/lib/telegramInitData.ts.
const contractToken = "5768337691:AAH5YkoiEuPk8-FZa32hStHTqXiLPtAEhx8"

func signedInitData(t *testing.T, authDate time.Time) string {
	t.Helper()
	payload := "query_id=AAHdF6IQAAAAAN0XohDhrOrc&user=%7B%22id%22%3A777%2C%22first_name%22%3A%22Leo%22%7D"
	hash, err := initdata.SignQueryString(payload, contractToken, authDate)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return fmt.Sprintf("%s&auth_date=%d&hash=%s", payload, authDate.Unix(), hash)
}

func TestServerInitDataContract(t *testing.T) {
	const exp = 24 * time.Hour

	t.Run("корректно подписанный — принимается", func(t *testing.T) {
		raw := signedInitData(t, time.Now())
		if err := initdata.Validate(raw, contractToken, exp); err != nil {
			t.Fatalf("ожидали приём валидного initData, получили ошибку: %v", err)
		}
		parsed, err := initdata.Parse(raw)
		if err != nil || parsed.User.ID != 777 {
			t.Fatalf("ожидали user.id=777, parse=%+v err=%v", parsed.User, err)
		}
	})

	t.Run("пустой initData (баг клиента) — отвергается", func(t *testing.T) {
		if err := initdata.Validate("", contractToken, exp); err == nil {
			t.Fatal("пустой initData обязан отвергаться")
		}
	})

	t.Run("подделанный hash — отвергается", func(t *testing.T) {
		raw := signedInitData(t, time.Now())
		tampered := raw[:len(raw)-4] + "0000"
		if err := initdata.Validate(tampered, contractToken, exp); err == nil {
			t.Fatal("подделанный hash обязан отвергаться")
		}
	})

	t.Run("протухший (25ч) — отвергается окном 24ч", func(t *testing.T) {
		raw := signedInitData(t, time.Now().Add(-25*time.Hour))
		if err := initdata.Validate(raw, contractToken, exp); err == nil {
			t.Fatal("протухший initData обязан отвергаться")
		}
	})
}

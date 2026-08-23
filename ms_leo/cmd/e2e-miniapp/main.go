// Одноразовая e2e-утилита: проверяет серверный гейт initData реального API мини-аппа.
// Подписывает initData тем же бот-токеном (FAT_LEOPARD_API_TOKEN) и шлёт три запроса
// в /api/miniapp/feed: пустой, подделанный, валидный. Печатает HTTP-статус и тело.
//
//	FAT_LEOPARD_API_TOKEN=… go run ./cmd/e2e-miniapp  [http://localhost:8080]
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	initdata "github.com/telegram-mini-apps/init-data-golang"
)

func main() {
	base := "http://localhost:8080"
	if len(os.Args) > 1 {
		base = os.Args[1]
	}
	token := os.Getenv("FAT_LEOPARD_API_TOKEN")
	if token == "" {
		fmt.Println("FAT_LEOPARD_API_TOKEN пуст")
		os.Exit(1)
	}

	payload := `query_id=AAHe2e&user=%7B%22id%22%3A777%2C%22first_name%22%3A%22Leo%22%7D`
	hash, err := initdata.SignQueryString(payload, token, time.Now())
	if err != nil {
		fmt.Println("sign:", err)
		os.Exit(1)
	}
	valid := fmt.Sprintf("%s&auth_date=%d&hash=%s", payload, time.Now().Unix(), hash)
	tampered := valid[:len(valid)-4] + "0000"

	cases := []struct {
		name     string
		initData string
	}{
		{"пустой initData (баг клиента)", ""},
		{"подделанный hash", tampered},
		{"валидный initData", valid},
	}

	for _, c := range cases {
		body, _ := json.Marshal(map[string]any{"init_data": c.initData, "since_id": 0})
		resp, err := http.Post(base+"/api/miniapp/feed", "application/json", bytes.NewReader(body))
		if err != nil {
			fmt.Printf("[%s] ошибка запроса: %v\n", c.name, err)
			continue
		}
		out, _ := io.ReadAll(io.LimitReader(resp.Body, 400))
		resp.Body.Close()
		fmt.Printf("[%-32s] HTTP %d  %s\n", c.name, resp.StatusCode, string(out))
	}
}

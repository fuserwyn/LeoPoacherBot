package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	initdata "github.com/telegram-mini-apps/init-data-golang"
)

const (
	standTimeout = 30 * time.Second
	railwayGraphQL = "https://backboard.railway.app/graphql/v2"
)

type MiniappStandService struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type MiniappStandState struct {
	Configured bool                  `json:"configured"`
	Running    bool                  `json:"running"`
	MiniappURL string                `json:"miniapp_url"`
	Services   []MiniappStandService `json:"services"`
	Total int  `json:"total"`
	Up    int  `json:"up"`
	Busy  bool `json:"busy"`
}

func (b *Bot) MiniappLabStand(
	viewerUserID int64, initD initdata.InitData, action string,
) (MiniappStandState, error) {
	var out MiniappStandState
	if b.config == nil {
		return out, fmt.Errorf("конфигурация недоступна")
	}
	
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return out, err
	}
	if !b.config.IsAdminTelegramUser(viewerUserID) {
		return out, ErrPackFeedForbidden
	}

	railwayToken := strings.TrimSpace(b.config.RailwayToken)
	envID := strings.TrimSpace(b.config.LabEnvironmentID)
	projectID := strings.TrimSpace(b.config.RailwayProjectID)
	serviceIDs := b.config.LabServiceIDs()

	if railwayToken == "" || envID == "" || len(serviceIDs) == 0 {
		out.MiniappURL = strings.TrimSpace(b.config.LabMiniappURL)
		return out, nil
	}

	out.Configured = true
	out.MiniappURL = strings.TrimSpace(b.config.LabMiniappURL)

	action = strings.TrimSpace(action)
	if action == "" {
		action = "status"
	}

	switch action {
	case "status":
	case "stop":
		if err := b.standSwitch(false); err != nil {
			return out, fmt.Errorf("ошибка выключения: %w", err)
		}
		b.announceStand(false, trackerViewerName(initD))
	case "start":
		if err := b.standSwitch(true); err != nil {
			return out, fmt.Errorf("ошибка включения: %w", err)
		}
		b.announceStand(true, trackerViewerName(initD))
	default:
		return out, ErrAdminActionInvalid
	}

	services, err := b.standServices()
	if err != nil {
		return out, fmt.Errorf("ошибка получения статуса сервисов: %w", err)
	}

	out.Services = services
	out.Total = len(services)
	for _, svc := range services {
		switch svc.Status {
		case "SUCCESS":
			out.Up++
		case "DEPLOYING", "BUILDING", "INITIALIZING", "QUEUED", "WAITING", "REMOVING":
			out.Busy = true
		}
	}
	out.Running = out.Up > 0
	return out, nil
}

func (b *Bot) railwayCall(query string, variables map[string]any) (json.RawMessage, error) {
	if b.config == nil || strings.TrimSpace(b.config.RailwayToken) == "" {
		return nil, fmt.Errorf("конфигурация Railway отсутствует")
	}

	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return nil, fmt.Errorf("ошибка формирования запроса: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, railwayGraphQL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(b.config.RailwayToken))

	client := &http.Client{Timeout: standTimeout}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса к Railway: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("Railway вернул статус %d", res.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("ошибка разбора JSON: %w", err)
	}

	if len(envelope.Errors) > 0 {
		return nil, fmt.Errorf("ошибка Railway: %s", envelope.Errors[0].Message)
	}

	return envelope.Data, nil
}
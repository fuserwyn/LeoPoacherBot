package railway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"leo-tracker/internal/config"
)

const railwayGraphQL = "https://backboard.railway.app/graphql/v2"

type Service struct {
	ID   string
	Name string
}

type Env struct {
	ID   string
	Name string
}

type Deploy struct {
	ID        string
	Status    string
	CreatedAt time.Time
}

func NormalizeName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	return strings.ReplaceAll(n, "_", "-")
}

func IsAppService(name string) bool {
	n := NormalizeName(name)
	if n == "" || strings.Contains(n, "tracker") || strings.Contains(n, "postgres") ||
		strings.Contains(n, "redis") || strings.Contains(n, "qdrant") || strings.Contains(n, "payment") {
		return false
	}
	return n == "leo" || n == "miniapp" ||
		strings.Contains(n, "miniapp") ||
		strings.Contains(n, "ms-leo") ||
		strings.HasPrefix(n, "leo-") ||
		strings.Contains(n, "fat-leopard")
}

func IsSelfService(name string) bool {
	n := NormalizeName(name)
	if strings.Contains(n, "miniapp") || strings.Contains(n, "tracker") {
		return false
	}
	return n == "leo" || strings.Contains(n, "ms-leo") || strings.HasPrefix(n, "leo-") ||
		strings.Contains(n, "fat-leopard")
}

func PickEnvironmentID(wantName, wantID string, envs []Env) string {
	if id := strings.TrimSpace(wantID); id != "" {
		return id
	}
	var names []string
	if w := strings.TrimSpace(wantName); w != "" {
		names = append(names, w)
	}
	names = append(names, "production", "prod", "main")
	seen := map[string]bool{}
	for _, w := range names {
		key := strings.ToLower(w)
		if seen[key] {
			continue
		}
		seen[key] = true
		for _, e := range envs {
			if strings.EqualFold(e.Name, w) {
				return e.ID
			}
		}
	}
	if len(envs) > 0 {
		return envs[0].ID
	}
	return ""
}

func QueueSelfLast(svcs []Service) []Service {
	out := make([]Service, 0, len(svcs))
	var self []Service
	for _, svc := range svcs {
		if IsSelfService(svc.Name) {
			self = append(self, svc)
			continue
		}
		out = append(out, svc)
	}
	return append(out, self...)
}

const webhookGrace = 12 * time.Second
const webhookPoll = 2 * time.Second

// RedeployStand — после пуша в main не заказываем вторую такую же сборку.
// GitHub уже дергает Railway (на скрине два «via GitHub» у #36 с разницей
// в секунду — вебхук + serviceInstanceDeploy). Сначала ждём вебхук, API
// дергаем только если сервис так и не стартовал.
func RedeployStand(cfg config.Config) (map[string]string, error) {
	if strings.TrimSpace(cfg.RailwayToken) == "" || strings.TrimSpace(cfg.RailwayProjectID) == "" {
		return nil, fmt.Errorf("нет RAILWAY_API_TOKEN или RAILWAY_PROJECT_ID")
	}
	envID, svcs, err := lookup(cfg)
	if err != nil {
		return nil, err
	}
	pinned := make(map[string]string, len(svcs))
	since := time.Now().Add(-45 * time.Second)
	deadline := time.Now().Add(webhookGrace)
	for {
		all := true
		for _, svc := range svcs {
			if strings.TrimSpace(pinned[svc.Name]) != "" {
				continue
			}
			if id := reuseInFlight(cfg, envID, svc.ID, since); id != "" {
				pinned[svc.Name] = id
				continue
			}
			all = false
		}
		if all || !time.Now().Before(deadline) {
			break
		}
		time.Sleep(webhookPoll)
	}
	var firstErr error
	for _, svc := range QueueSelfLast(svcs) {
		if strings.TrimSpace(pinned[svc.Name]) != "" {
			continue
		}
		id, terr := trigger(cfg, envID, svc.ID)
		if terr != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", svc.Name, terr)
			}
			continue
		}
		if strings.TrimSpace(id) != "" {
			pinned[svc.Name] = id
		}
	}
	if len(pinned) == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, fmt.Errorf("Railway не принял заказ сборки")
	}
	return pinned, nil
}

func lookup(cfg config.Config) (string, []Service, error) {
	raw, err := call(cfg,
		`query($id:String!){ project(id:$id){ environments{ edges{ node{ id name } } } services{ edges{ node{ id name } } } } }`,
		map[string]any{"id": strings.TrimSpace(cfg.RailwayProjectID)},
	)
	if err != nil {
		return "", nil, err
	}
	var parsed struct {
		Project struct {
			Environments struct {
				Edges []struct {
					Node Env `json:"node"`
				} `json:"edges"`
			} `json:"environments"`
			Services struct {
				Edges []struct {
					Node Service `json:"node"`
				} `json:"edges"`
			} `json:"services"`
		} `json:"project"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", nil, err
	}
	var envs []Env
	for _, e := range parsed.Project.Environments.Edges {
		envs = append(envs, e.Node)
	}
	envID := PickEnvironmentID(cfg.Branch, cfg.RailwayEnvID, envs)
	var svcs []Service
	for _, s := range parsed.Project.Services.Edges {
		if IsAppService(s.Node.Name) {
			svcs = append(svcs, s.Node)
		}
	}
	if envID == "" || len(svcs) == 0 {
		return "", nil, fmt.Errorf("сервисы стенда не найдены")
	}
	return envID, svcs, nil
}

func deployInFlight(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "BUILDING", "DEPLOYING", "INITIALIZING", "QUEUED", "WAITING", "PENDING", "NEEDS_APPROVAL":
		return true
	default:
		return false
	}
}

func pickNewest(deploys []Deploy, since time.Time, ok func(Deploy) bool) string {
	var bestID string
	var bestAt time.Time
	for _, d := range deploys {
		if !ok(d) {
			continue
		}
		if d.CreatedAt.IsZero() || d.CreatedAt.Before(since) {
			continue
		}
		if bestID == "" || d.CreatedAt.After(bestAt) {
			bestID = strings.TrimSpace(d.ID)
			bestAt = d.CreatedAt
		}
	}
	return bestID
}

// PickInFlight — уже крутится сборка после пуша (вебхук GitHub).
func PickInFlight(deploys []Deploy, since time.Time) string {
	return pickNewest(deploys, since, func(d Deploy) bool { return deployInFlight(d.Status) })
}

// PickStarted — вебхук уже стартовал или даже успел закрыться (SKIPPED /
// быстрый SUCCESS). Вторую сборку через API не заказываем.
func PickStarted(deploys []Deploy, since time.Time) string {
	if id := PickInFlight(deploys, since); id != "" {
		return id
	}
	return pickNewest(deploys, since, func(d Deploy) bool {
		st := strings.ToUpper(strings.TrimSpace(d.Status))
		return st == "SUCCESS" || st == "SKIPPED"
	})
}

func PinStarted(svcs []Service, pinned map[string]string, deploys map[string][]Deploy, since time.Time) bool {
	if pinned == nil {
		return false
	}
	all := len(svcs) > 0
	for _, svc := range svcs {
		if strings.TrimSpace(pinned[svc.Name]) != "" {
			continue
		}
		if id := PickStarted(deploys[svc.Name], since); id != "" {
			pinned[svc.Name] = id
			continue
		}
		all = false
	}
	return all
}

func reuseInFlight(cfg config.Config, envID, svcID string, since time.Time) string {
	raw, err := call(cfg,
		`query($sid:String!,$eid:String!){ deployments(first:8, input:{serviceId:$sid, environmentId:$eid}){ edges{ node{ id status createdAt } } } }`,
		map[string]any{"sid": svcID, "eid": envID},
	)
	if err != nil {
		return ""
	}
	var parsed struct {
		Deployments struct {
			Edges []struct {
				Node struct {
					ID        string `json:"id"`
					Status    string `json:"status"`
					CreatedAt string `json:"createdAt"`
				} `json:"node"`
			} `json:"edges"`
		} `json:"deployments"`
	}
	if json.Unmarshal(raw, &parsed) != nil {
		return ""
	}
	var deploys []Deploy
	for _, edge := range parsed.Deployments.Edges {
		at, _ := time.Parse(time.RFC3339, edge.Node.CreatedAt)
		deploys = append(deploys, Deploy{ID: edge.Node.ID, Status: edge.Node.Status, CreatedAt: at})
	}
	return PickStarted(deploys, since)
}

func trigger(cfg config.Config, envID, svcID string) (string, error) {
	envID = strings.TrimSpace(envID)
	svcID = strings.TrimSpace(svcID)
	if envID == "" || svcID == "" {
		return "", fmt.Errorf("нет сервиса или окружения")
	}
	vars := map[string]any{"s": svcID, "e": envID}
	mutations := []string{
		`mutation($s:String!,$e:String!){ serviceInstanceDeploy(serviceId:$s, environmentId:$e, latestCommit:true) }`,
		`mutation($s:String!,$e:String!){ serviceInstanceDeployV2(serviceId:$s, environmentId:$e) }`,
		`mutation($s:String!,$e:String!){ serviceInstanceRedeploy(serviceId:$s, environmentId:$e) }`,
	}
	var last error
	for _, q := range mutations {
		raw, err := call(cfg, q, vars)
		if err != nil {
			last = err
			continue
		}
		return parseDeployID(raw), nil
	}
	if last != nil {
		return "", last
	}
	return "", fmt.Errorf("Railway не принял деплой")
}

func parseDeployID(raw []byte) string {
	var asString struct {
		ServiceInstanceDeploy   json.RawMessage `json:"serviceInstanceDeploy"`
		ServiceInstanceDeployV2 json.RawMessage `json:"serviceInstanceDeployV2"`
		ServiceInstanceRedeploy json.RawMessage `json:"serviceInstanceRedeploy"`
	}
	if json.Unmarshal(raw, &asString) != nil {
		return ""
	}
	for _, field := range []json.RawMessage{
		asString.ServiceInstanceDeploy,
		asString.ServiceInstanceDeployV2,
		asString.ServiceInstanceRedeploy,
	} {
		if id := deployIDFromRaw(field); id != "" {
			return id
		}
	}
	return ""
}

func deployIDFromRaw(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	var obj struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		return strings.TrimSpace(obj.ID)
	}
	return ""
}

func call(cfg config.Config, query string, variables map[string]any) (json.RawMessage, error) {
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, railwayGraphQL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.RailwayToken))
	res, err := (&http.Client{Timeout: 45 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("Railway недоступен: %w", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("Railway ответил %d", res.StatusCode)
	}
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	if len(envelope.Errors) > 0 {
		return nil, fmt.Errorf("%s", envelope.Errors[0].Message)
	}
	return envelope.Data, nil
}

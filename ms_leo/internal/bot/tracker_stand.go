package bot

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const trackerStandWait = 10 * time.Minute
const trackerStandPoll = 12 * time.Second

// waitStandBuild — после пуша в main карточка сидит в «Сборка», пока MiniApp
// на Railway не станет SUCCESS. Без токена проверяем, что стенд отвечает.
func (b *Bot) waitStandBuild(started time.Time) error {
	if b == nil || b.config == nil {
		return fmt.Errorf("нет конфигурации Railway")
	}
	if strings.TrimSpace(b.config.RailwayToken) != "" && strings.TrimSpace(b.config.RailwayProjectID) != "" {
		if err := b.waitRailwayMiniApp(started); err == nil {
			return nil
		} else if !strings.Contains(err.Error(), "не найден") {
			return err
		}
	}
	return b.waitMiniappHTTP()
}

func (b *Bot) waitRailwayMiniApp(started time.Time) error {
	envID, svcID, err := b.lookupMainMiniApp()
	if err != nil {
		return err
	}
	since := started.Add(-45 * time.Second)
	deadline := time.Now().Add(trackerStandWait)
	for time.Now().Before(deadline) {
		raw, err := b.railwayCall(
			`query($sid:String!,$eid:String!){ deployments(first:5, input:{serviceId:$sid, environmentId:$eid}){ edges{ node{ id status createdAt } } } }`,
			map[string]any{"sid": svcID, "eid": envID},
		)
		if err != nil {
			time.Sleep(trackerStandPoll)
			continue
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
			time.Sleep(trackerStandPoll)
			continue
		}
		for _, edge := range parsed.Deployments.Edges {
			n := edge.Node
			at, _ := time.Parse(time.RFC3339, n.CreatedAt)
			if !at.IsZero() && at.Before(since) {
				continue
			}
			switch strings.ToUpper(n.Status) {
			case "SUCCESS":
				return nil
			case "FAILED", "CRASHED":
				return fmt.Errorf("деплой %s", strings.ToLower(n.Status))
			}
		}
		time.Sleep(trackerStandPoll)
	}
	return fmt.Errorf("не уложилась в 10 минут")
}

func (b *Bot) lookupMainMiniApp() (envID, svcID string, err error) {
	raw, err := b.railwayCall(
		`query($id:String!){ project(id:$id){ environments{ edges{ node{ id name } } } services{ edges{ node{ id name } } } } }`,
		map[string]any{"id": strings.TrimSpace(b.config.RailwayProjectID)},
	)
	if err != nil {
		return "", "", err
	}
	var parsed struct {
		Project struct {
			Environments struct {
				Edges []struct {
					Node struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"node"`
				} `json:"edges"`
			} `json:"environments"`
			Services struct {
				Edges []struct {
					Node struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"node"`
				} `json:"edges"`
			} `json:"services"`
		} `json:"project"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", "", err
	}
	wantEnv := "main"
	if br := strings.ToLower(strings.TrimSpace(b.config.BoardBranch)); br != "" && br != "main" {
		wantEnv = br
	}
	for _, e := range parsed.Project.Environments.Edges {
		if strings.EqualFold(e.Node.Name, wantEnv) {
			envID = e.Node.ID
			break
		}
	}
	if envID == "" && len(parsed.Project.Environments.Edges) > 0 {
		envID = parsed.Project.Environments.Edges[0].Node.ID
	}
	for _, s := range parsed.Project.Services.Edges {
		if strings.EqualFold(s.Node.Name, "MiniApp") {
			svcID = s.Node.ID
			break
		}
	}
	if envID == "" || svcID == "" {
		return "", "", fmt.Errorf("MiniApp на стенде не найден")
	}
	return envID, svcID, nil
}

func (b *Bot) waitMiniappHTTP() error {
	url := strings.TrimRight(strings.TrimSpace(b.config.MiniappWebAppURL), "/")
	if url == "" {
		url = strings.TrimRight(strings.TrimSpace(b.config.MiniappPublicBaseURL), "/")
	}
	if url == "" {
		url = "https://fat-leopard-main.up.railway.app"
	}
	deadline := time.Now().Add(3 * time.Minute)
	client := &http.Client{Timeout: 15 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url + "/")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(trackerStandPoll)
	}
	return fmt.Errorf("стенд не ответил")
}

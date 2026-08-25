package bot

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"leo-bot/internal/database"
)

const trackerStandWait = 10 * time.Minute
const trackerStandPoll = 12 * time.Second
const trackerStandSkipGrace = 45 * time.Second
const trackerStandMaxRetries = 5
const trackerStandLogClip = 2000

var trackerStandInflight sync.Map

type standDeploy struct {
	ID        string
	Status    string
	Service   string
	CreatedAt time.Time
}

type standService struct {
	ID   string
	Name string
}

type standWaitOutcome struct {
	Done      bool
	Err       error
	FailedID  string
	FailedSvc string
	Status    string
}

type standBuildError struct {
	Service  string
	DeployID string
	Status   string
	Msg      string
}

func (e *standBuildError) Error() string {
	if e == nil {
		return "сборка не прошла"
	}
	if strings.TrimSpace(e.Msg) != "" {
		return e.Msg
	}
	svc := strings.TrimSpace(e.Service)
	if svc == "" {
		svc = "стенд"
	}
	st := strings.ToLower(strings.TrimSpace(e.Status))
	if st == "" {
		st = "failed"
	}
	return fmt.Sprintf("%s: деплой %s", svc, st)
}

func tryBeginTrackerStand(taskID int64) bool {
	_, loaded := trackerStandInflight.LoadOrStore(taskID, struct{}{})
	return !loaded
}

func endTrackerStand(taskID int64) {
	trackerStandInflight.Delete(taskID)
}

// trackerStandBusy — по этой карточке уже идёт сборка.
func trackerStandBusy(taskID int64) bool {
	_, busy := trackerStandInflight.Load(taskID)
	return busy
}

func standDeployInFlight(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "BUILDING", "DEPLOYING", "INITIALIZING", "QUEUED", "WAITING", "PENDING", "NEEDS_APPROVAL":
		return true
	default:
		return false
	}
}

func standWaitDecision(deploys []standDeploy, since, started, now time.Time) standWaitOutcome {
	latestSuccess := false
	inFlight := false
	newSuccess := false
	for _, d := range deploys {
		st := strings.ToUpper(strings.TrimSpace(d.Status))
		fresh := d.CreatedAt.IsZero() || !d.CreatedAt.Before(since)
		if st == "SUCCESS" {
			latestSuccess = true
			if fresh {
				newSuccess = true
			}
		}
		if fresh && (st == "FAILED" || st == "CRASHED") {
			return standWaitOutcome{
				Err:       fmt.Errorf("деплой %s", strings.ToLower(st)),
				FailedID:  d.ID,
				FailedSvc: d.Service,
				Status:    strings.ToLower(st),
			}
		}
		if fresh && standDeployInFlight(st) {
			inFlight = true
		}
	}
	if newSuccess {
		return standWaitOutcome{Done: true}
	}
	if inFlight {
		return standWaitOutcome{}
	}
	if latestSuccess && !started.IsZero() && now.Sub(started) >= trackerStandSkipGrace {
		return standWaitOutcome{Done: true}
	}
	return standWaitOutcome{}
}

// standWaitPinned — ждём ровно тот деплой, который заказали сами.
// Чужая старая сборка за успех не считается: карточка закрывается только
// когда собралось то, что мы отправили.
func standWaitPinned(deploys []standDeploy, deployID string) standWaitOutcome {
	deployID = strings.TrimSpace(deployID)
	for _, d := range deploys {
		if strings.TrimSpace(d.ID) != deployID {
			continue
		}
		st := strings.ToUpper(strings.TrimSpace(d.Status))
		switch st {
		case "SUCCESS":
			return standWaitOutcome{Done: true}
		case "FAILED", "CRASHED", "REMOVED":
			return standWaitOutcome{
				Err:       fmt.Errorf("деплой %s", strings.ToLower(st)),
				FailedID:  d.ID,
				FailedSvc: d.Service,
				Status:    strings.ToLower(st),
			}
		}
		return standWaitOutcome{} // ещё собирается
	}
	return standWaitOutcome{} // заказанный деплой пока не виден в списке
}

// standWaitServices — SUCCESS всех сервисов стенда. Свежий FAILED у Leo
// не прячется за SKIPPED MiniApp: иначе карточка «выполнена», а бот не собрался.
func standWaitServices(
	byService map[string][]standDeploy, pinned map[string]string, since, started, now time.Time,
) standWaitOutcome {
	if len(byService) == 0 {
		return standWaitOutcome{}
	}
	waiting := false
	for name, deploys := range byService {
		var out standWaitOutcome
		if id := strings.TrimSpace(pinned[name]); id != "" {
			out = standWaitPinned(deploys, id)
		} else {
			out = standWaitDecision(deploys, since, started, now)
		}
		if out.Err != nil {
			if out.FailedSvc == "" {
				out.FailedSvc = name
			}
			out.Err = fmt.Errorf("%s: %w", name, out.Err)
			return out
		}
		if !out.Done {
			waiting = true
		}
	}
	if waiting {
		return standWaitOutcome{}
	}
	return standWaitOutcome{Done: true}
}

func trackerTaskShippedToStand(t database.TrackerTask) bool {
	shipped := false
	for _, s := range t.Steps {
		low := strings.ToLower(s)
		if strings.Contains(low, "пуш в ") || strings.Contains(low, "ждём сборку") {
			shipped = true
		}
		if strings.Contains(low, "вернули в работу") {
			shipped = false
		}
	}
	return shipped
}

func trackerStandFailCount(t database.TrackerTask) int {
	n := 0
	for _, s := range t.Steps {
		low := strings.ToLower(s)
		if strings.Contains(low, "сборка на стенде не прошла") {
			n++
		}
	}
	return n
}

func standBuildDeployID(err error) string {
	var e *standBuildError
	if errors.As(err, &e) && e != nil {
		return e.DeployID
	}
	return ""
}

func clipStandLogs(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	var useful []string
	for _, line := range lines {
		low := strings.ToLower(line)
		if strings.Contains(low, "error") || strings.Contains(low, "undefined") ||
			strings.Contains(low, "failed") || strings.Contains(low, "fatal") ||
			strings.Contains(low, "cannot") || strings.Contains(low, "build failed") {
			useful = append(useful, line)
		}
	}
	if len(useful) > 0 {
		text = strings.Join(useful, "\n")
	}
	r := []rune(text)
	if len(r) <= trackerStandLogClip {
		return text
	}
	return string(r[len(r)-trackerStandLogClip:])
}

// waitStandBuild — после пуша в main карточка сидит в «Сборка», пока Leo
// и MiniApp на Railway не станут SUCCESS. Если сервиса не пересобирали
// (SKIPPED) — живой стенд тоже ок. Свежий FAILED любого из них — ошибка.
func (b *Bot) waitStandBuild(started time.Time, pinned map[string]string) error {
	if b == nil || b.config == nil {
		return fmt.Errorf("нет конфигурации Railway")
	}
	if strings.TrimSpace(b.config.RailwayToken) != "" && strings.TrimSpace(b.config.RailwayProjectID) != "" {
		err := b.waitRailwayStand(started, pinned)
		if err == nil {
			return nil
		}
		// Сборку заказали сами — её судьба и есть ответ. Подменять его
		// проверкой «сайт отвечает» нельзя: старый сайт отвечает и после
		// провалившегося деплоя, и карточка врала бы «выполнено».
		if len(pinned) > 0 || strings.Contains(err.Error(), "деплой ") {
			return err
		}
	}
	return b.waitMiniappHTTP()
}

func (b *Bot) waitRailwayStand(started time.Time, pinned map[string]string) error {
	envID, svcs, err := b.lookupMainStandServices()
	if err != nil {
		return err
	}
	since := started.Add(-45 * time.Second)
	deadline := time.Now().Add(trackerStandWait)
	for time.Now().Before(deadline) {
		by := make(map[string][]standDeploy, len(svcs))
		apiFail := 0
		for _, svc := range svcs {
			deploys, lerr := b.listStandDeploys(svc.ID, envID)
			if lerr != nil {
				apiFail++
				continue
			}
			for i := range deploys {
				deploys[i].Service = svc.Name
			}
			by[svc.Name] = deploys
		}
		if len(by) == 0 && apiFail > 0 {
			time.Sleep(trackerStandPoll)
			continue
		}
		out := standWaitServices(by, pinned, since, started, time.Now())
		if out.Err != nil {
			return &standBuildError{
				Service:  out.FailedSvc,
				DeployID: out.FailedID,
				Status:   out.Status,
				Msg:      out.Err.Error(),
			}
		}
		if out.Done {
			return nil
		}
		time.Sleep(trackerStandPoll)
	}
	return fmt.Errorf("не уложилась в 10 минут")
}

func (b *Bot) listStandDeploys(svcID, envID string) ([]standDeploy, error) {
	raw, err := b.railwayCall(
		`query($sid:String!,$eid:String!){ deployments(first:5, input:{serviceId:$sid, environmentId:$eid}){ edges{ node{ id status createdAt } } } }`,
		map[string]any{"sid": svcID, "eid": envID},
	)
	if err != nil {
		return nil, err
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
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	deploys := make([]standDeploy, 0, len(parsed.Deployments.Edges))
	for _, edge := range parsed.Deployments.Edges {
		at, _ := time.Parse(time.RFC3339, edge.Node.CreatedAt)
		deploys = append(deploys, standDeploy{
			ID:        edge.Node.ID,
			Status:    edge.Node.Status,
			CreatedAt: at,
		})
	}
	return deploys, nil
}

func (b *Bot) railwayDeployLogs(deployID string) string {
	if b == nil || strings.TrimSpace(deployID) == "" {
		return ""
	}
	if logs := b.railwayBuildLogsQuery(deployID); logs != "" {
		return logs
	}
	return b.railwayRuntimeLogsQuery(deployID)
}

func (b *Bot) railwayBuildLogsQuery(deployID string) string {
	raw, err := b.railwayCall(
		`query($id:String!){ buildLogs(deploymentId:$id) }`,
		map[string]any{"id": deployID},
	)
	if err != nil {
		return ""
	}
	var parsed struct {
		BuildLogs string `json:"buildLogs"`
	}
	if json.Unmarshal(raw, &parsed) != nil {
		return ""
	}
	return clipStandLogs(parsed.BuildLogs)
}

func (b *Bot) railwayRuntimeLogsQuery(deployID string) string {
	raw, err := b.railwayCall(
		`query($id:String!,$n:Int){ deploymentLogs(deploymentId:$id, limit:$n){ message } }`,
		map[string]any{"id": deployID, "n": 80},
	)
	if err != nil {
		return ""
	}
	var parsed struct {
		DeploymentLogs []struct {
			Message string `json:"message"`
		} `json:"deploymentLogs"`
	}
	if json.Unmarshal(raw, &parsed) != nil {
		return ""
	}
	var bld strings.Builder
	for _, line := range parsed.DeploymentLogs {
		if msg := strings.TrimSpace(line.Message); msg != "" {
			bld.WriteString(msg)
			bld.WriteByte('\n')
		}
	}
	return clipStandLogs(bld.String())
}

func isStandWatchService(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" || strings.Contains(n, "tracker") || strings.Contains(n, "postgres") {
		return false
	}
	return n == "miniapp" || n == "ms_leo" || n == "leo" || strings.HasPrefix(n, "ms_leo")
}

func (b *Bot) lookupMainStandServices() (envID string, svcs []standService, err error) {
	raw, err := b.railwayCall(
		`query($id:String!){ project(id:$id){ environments{ edges{ node{ id name } } } services{ edges{ node{ id name } } } } }`,
		map[string]any{"id": strings.TrimSpace(b.config.RailwayProjectID)},
	)
	if err != nil {
		return "", nil, err
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
		return "", nil, err
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
		if isStandWatchService(s.Node.Name) {
			svcs = append(svcs, standService{ID: s.Node.ID, Name: s.Node.Name})
		}
	}
	if envID == "" || len(svcs) == 0 {
		return "", nil, fmt.Errorf("сервисы стенда не найдены")
	}
	return envID, svcs, nil
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

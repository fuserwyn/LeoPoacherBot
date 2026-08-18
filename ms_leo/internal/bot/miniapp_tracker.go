package bot

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	initdata "github.com/telegram-mini-apps/init-data-golang"
)

// Трекер задач Леопарда живёт в MyVibeLab: там доска, спринты и агент, который
// эти задачи выполняет. Доску показываем прямо в мини-аппе (TrackerScreen), а
// сюда сводим весь разговор с MyVibeLab: мини-апп ходит только к нам, а мы уже
// подписываем гостевую сессию общим секретом (BOARD_SSO_SECRET) и дёргаем
// нужную ручку трекера.
//
// Почему через нас, а не напрямую из браузера: секрет подписи нельзя отдавать
// в мини-апп (его собрал бы кто угодно, кто открыл приложение), а initData
// Леопарда MyVibeLab проверить не может — она подписана нашим токеном бота.
const (
	trackerSessionTTL = 12 * time.Hour
	trackerTimeout    = 45 * time.Second
	// Спринты — это ход ИИ по коду проекта: разбор репозитория и нарезка задач
	// занимают до минуты с лишним, обычного таймаута не хватает.
	trackerSlowTimeout = 180 * time.Second
)

// ErrTrackerNotConfigured — не задан секрет или адрес: доску показывать нечем.
var ErrTrackerNotConfigured = fmt.Errorf("tracker not configured")

// trackerOp — что мини-апп разрешено делать с доской. Белый список, а не
// произвольный путь: иначе через прокси можно было бы дотянуться до любой
// ручки MyVibeLab от имени владельца доски.
type trackerOp struct {
	method string
	path   string // {id} подставляется из task_id
	slow   bool   // ход ИИ: длинный таймаут
}

var trackerOps = map[string]trackerOp{
	"list":            {http.MethodGet, "/api/scheduled", false},
	"create":          {http.MethodPost, "/api/scheduled", false},
	"task":            {http.MethodGet, "/api/scheduled/{id}", false},
	"status":          {http.MethodGet, "/api/scheduled/{id}/status", false},
	"cancel":          {http.MethodPost, "/api/scheduled/cancel", false},
	"delete":          {http.MethodDelete, "/api/scheduled/{id}", false},
	"qa":              {http.MethodPost, "/api/scheduled/qa", false},
	"auto_qa":         {http.MethodPost, "/api/scheduled/auto_qa", true},
	"prompt":          {http.MethodPost, "/api/scheduled/prompt", false},
	"reschedule":      {http.MethodPost, "/api/scheduled/reschedule", false},
	"sprint_ideas":    {http.MethodPost, "/api/scheduled/sprints/ideas", true},
	"sprint_generate": {http.MethodPost, "/api/scheduled/sprints/generate", true},
	"sprint_apply":    {http.MethodPost, "/api/scheduled/sprints/apply", true},
}

// trackerSession — подписанная гостевая сессия MyVibeLab: тот же формат, что
// и у него самого (payload.signature, payload = base64url(JSON) без padding).
func (b *Bot) trackerSession(userID int64, name string) (string, error) {
	secret := strings.TrimSpace(b.config.BoardSecret)
	repo := strings.TrimSpace(b.config.BoardRepo)
	if secret == "" || repo == "" || strings.TrimSpace(b.config.BoardURL) == "" {
		return "", ErrTrackerNotConfigured
	}
	payload, err := json.Marshal(map[string]any{
		"k": "sess",
		"r": repo,
		"u": userID,
		"n": name,
		"e": time.Now().Add(trackerSessionTTL).Unix(),
	})
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func trackerViewerName(initD initdata.InitData) string {
	name := strings.TrimSpace(initD.User.FirstName + " " + initD.User.LastName)
	if name == "" {
		name = initD.User.Username
	}
	return name
}

// MiniappTrackerCall — выполнить операцию доски от имени админа мини-аппа.
// Возвращает тело ответа MyVibeLab как есть: разбирать его — дело экрана,
// формат карточек и статусов у нас с MyVibeLab общий.
func (b *Bot) MiniappTrackerCall(
	viewerUserID int64,
	initD initdata.InitData,
	op string,
	taskID int64,
	payload map[string]any,
) (json.RawMessage, error) {
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return nil, err
	}
	spec, ok := trackerOps[op]
	if !ok {
		return nil, ErrAdminActionInvalid
	}
	session, err := b.trackerSession(viewerUserID, trackerViewerName(initD))
	if err != nil {
		return nil, err
	}
	path := spec.path
	if strings.Contains(path, "{id}") {
		if taskID <= 0 {
			return nil, ErrAdminActionInvalid
		}
		path = strings.ReplaceAll(path, "{id}", fmt.Sprintf("%d", taskID))
	}

	var body io.Reader
	if spec.method != http.MethodGet && payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(raw)
	}
	url := strings.TrimRight(b.config.BoardURL, "/") + path
	req, err := http.NewRequest(spec.method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", "mvl_board="+session)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	timeout := trackerTimeout
	if spec.slow {
		timeout = trackerSlowTimeout
	}
	res, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("трекер недоступен: %w", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		// Текст ошибки у MyVibeLab уже человеческий («Укажи время в будущем»),
		// поэтому показываем его как есть, а не подменяем своим кодом.
		var errBody struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &errBody)
		if strings.TrimSpace(errBody.Error) != "" {
			return nil, fmt.Errorf("%s", errBody.Error)
		}
		return nil, fmt.Errorf("трекер ответил %d", res.StatusCode)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = []byte("{}")
	}
	return json.RawMessage(raw), nil
}

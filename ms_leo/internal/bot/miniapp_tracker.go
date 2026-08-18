package bot

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"leo-bot/internal/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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

// MiniappTrackerAttach — приложить картинку к задаче.
//
// Трекер принимает файл только multipart-ом, а мини-апп отдаёт готовое
// изображение из canvas (кроп + рисование) base64-строкой — собираем
// multipart здесь, чтобы браузеру не нужно было знать про чужой протокол.
func (b *Bot) MiniappTrackerAttach(
	viewerUserID int64,
	initD initdata.InitData,
	taskID int64,
	filename string,
	mime string,
	dataBase64 string,
) (json.RawMessage, error) {
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return nil, err
	}
	if taskID <= 0 {
		return nil, ErrAdminActionInvalid
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(dataBase64))
	if err != nil {
		return nil, fmt.Errorf("картинка не разобралась")
	}
	const maxBytes = 8 << 20
	if len(raw) == 0 || len(raw) > maxBytes {
		return nil, fmt.Errorf("картинка должна быть до 8 МБ")
	}
	if filename = strings.TrimSpace(filename); filename == "" {
		filename = "photo.jpg"
	}
	if mime = strings.TrimSpace(mime); mime == "" {
		mime = "image/jpeg"
	}
	session, err := b.trackerSession(viewerUserID, trackerViewerName(initD))
	if err != nil {
		return nil, err
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, filename))
	header.Set("Content-Type", mime)
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(raw); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/scheduled/%d/attachments", strings.TrimRight(b.config.BoardURL, "/"), taskID)
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", "mvl_board="+session)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := (&http.Client{Timeout: trackerTimeout}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("трекер недоступен: %w", err)
	}
	defer res.Body.Close()
	out, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		var errBody struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(out, &errBody)
		if strings.TrimSpace(errBody.Error) != "" {
			return nil, fmt.Errorf("%s", errBody.Error)
		}
		return nil, fmt.Errorf("трекер ответил %d", res.StatusCode)
	}
	if len(bytes.TrimSpace(out)) == 0 {
		out = []byte("{}")
	}
	return json.RawMessage(out), nil
}

// MiniappTrackerAuthors — кто ставил задачи: ник и имя по telegram_id.
// В трекере у задачи есть только author_id, а на доске нужен человек.
func (b *Bot) MiniappTrackerAuthors(
	viewerUserID int64, initD initdata.InitData, ids []int64,
) ([]database.AdminPersonRow, error) {
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) > 100 {
		ids = ids[:100]
	}
	return b.db.AdminPeopleByIDs(b.adminPackChatID(), ids)
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

// VerifyTrackerToken — проверить подпись, которой MyVibeLab метит свои запросы
// к нам (уведомления о задачах). Формат тот же, что у ссылки на доску.
// Возвращает repo и id человека, для которого запрос.
func (b *Bot) VerifyTrackerToken(token, kind string) (repo string, userID int64, ok bool) {
	secret := strings.TrimSpace(b.config.BoardSecret)
	if secret == "" || token == "" || !strings.Contains(token, ".") {
		return "", 0, false
	}
	parts := strings.SplitN(token, ".", 2)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0]))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(parts[1])) {
		return "", 0, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", 0, false
	}
	var payload struct {
		Kind string `json:"k"`
		Repo string `json:"r"`
		User int64  `json:"u"`
		Exp  int64  `json:"e"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", 0, false
	}
	if payload.Kind != kind || payload.Exp < time.Now().Unix() {
		return "", 0, false
	}
	return payload.Repo, payload.User, true
}

// NotifyTrackerAuthor — написать автору задачи в личку от имени бота стаи.
func (b *Bot) NotifyTrackerAuthor(userID int64, text string) error {
	if b == nil || b.api == nil || userID == 0 {
		return fmt.Errorf("некому писать")
	}
	msg := tgbotapi.NewMessage(userID, text)
	msg.DisableWebPagePreview = true
	_, err := b.api.Send(msg)
	return err
}

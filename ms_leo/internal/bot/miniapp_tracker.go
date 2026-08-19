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
	"list":   {http.MethodGet, "/api/scheduled", false},
	"create": {http.MethodPost, "/api/scheduled", false},
	"task":   {http.MethodGet, "/api/scheduled/{id}", false},
	"status": {http.MethodGet, "/api/scheduled/{id}/status", false},
	"cancel": {http.MethodPost, "/api/scheduled/cancel", false},
	// Результат выполненной задачи: забрать со стенда в прод или откатить.
	"promote":         {http.MethodPost, "/api/scheduled/promote", true},
	"revert":          {http.MethodPost, "/api/scheduled/revert", true},
	"delete":          {http.MethodDelete, "/api/scheduled/{id}", false},
	"qa":              {http.MethodPost, "/api/scheduled/qa", false},
	"auto_qa":         {http.MethodPost, "/api/scheduled/auto_qa", true},
	"prompt":          {http.MethodPost, "/api/scheduled/prompt", false},
	"reschedule":      {http.MethodPost, "/api/scheduled/reschedule", false},
	"sprint_ideas":    {http.MethodPost, "/api/scheduled/sprints/ideas", true},
	"sprint_generate": {http.MethodPost, "/api/scheduled/sprints/generate", true},
	"sprint_apply":    {http.MethodPost, "/api/scheduled/sprints/apply", true},
	// Пуш в репозиторий и проверка сборки на сервере: задача может уже быть
	// «выполнена», а контейнер — ещё со старым кодом, если авто-пуш не сработал.
	"push":           {http.MethodPost, "/api/push", true},
	"deploy_refresh": {http.MethodPost, "/api/deploy/refresh", true},
	"deploy_watch":   {http.MethodGet, "/api/deploy/watch", false},
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

// MiniappTrackerAttachmentGet — байты приложенного к задаче фото.
//
// Мини-апп не может забрать картинку у MyVibeLab сам: она отдаётся по гостевой
// куке, а куку нельзя показывать браузеру. Поэтому качаем здесь и отдаём
// base64 — картинки к задаче маленькие, ради них не заводим отдельный CDN.
func (b *Bot) MiniappTrackerAttachmentGet(
	viewerUserID int64, initD initdata.InitData, taskID int64, attID string,
) (mime string, data string, err error) {
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return "", "", err
	}
	res, err := b.trackerAttachmentRequest(
		http.MethodGet, taskID, attID, viewerUserID, trackerViewerName(initD),
	)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()
	const maxBytes = 8 << 20
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxBytes))
	if err != nil {
		return "", "", err
	}
	if res.StatusCode >= 400 {
		return "", "", trackerHTTPError(raw, res.StatusCode)
	}
	mime = res.Header.Get("Content-Type")
	if mime == "" {
		mime = "image/jpeg"
	}
	return mime, base64.StdEncoding.EncodeToString(raw), nil
}

// MiniappTrackerAttachmentDelete — снять фото с задачи: так работает «заменить».
func (b *Bot) MiniappTrackerAttachmentDelete(
	viewerUserID int64, initD initdata.InitData, taskID int64, attID string,
) error {
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return err
	}
	res, err := b.trackerAttachmentRequest(
		http.MethodDelete, taskID, attID, viewerUserID, trackerViewerName(initD),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		return trackerHTTPError(raw, res.StatusCode)
	}
	return nil
}

// trackerAttachmentRequest — запрос к одному вложению задачи под гостевой сессией.
func (b *Bot) trackerAttachmentRequest(
	method string, taskID int64, attID string, userID int64, name string,
) (*http.Response, error) {
	attID = strings.TrimSpace(attID)
	if taskID <= 0 || attID == "" || len(attID) > 32 {
		return nil, ErrAdminActionInvalid
	}
	session, err := b.trackerSession(userID, name)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf(
		"%s/api/scheduled/%d/attachments/%s",
		strings.TrimRight(b.config.BoardURL, "/"), taskID, attID,
	)
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", "mvl_board="+session)
	res, err := (&http.Client{Timeout: trackerTimeout}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("трекер недоступен: %w", err)
	}
	return res, nil
}

// trackerHTTPError — человеческий текст ошибки трекера, если он его прислал.
func trackerHTTPError(raw []byte, status int) error {
	var errBody struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(raw, &errBody)
	if strings.TrimSpace(errBody.Error) != "" {
		return fmt.Errorf("%s", errBody.Error)
	}
	return fmt.Errorf("трекер ответил %d", status)
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
	// Пустую ветку в подпись не кладём: MyVibeLab тогда заводит отдельную
	// ветку задачи, помечает её «выполнено» и не пушит в основную — сборка
	// на сервере не стартует. Нет поля — работает ветка репозитория по умолчанию.
	sess := map[string]any{
		"k": "sess",
		"r": repo,
		"u": userID,
		"n": name,
		"e": time.Now().Add(trackerSessionTTL).Unix(),
	}
	if branch := strings.TrimSpace(b.config.BoardBranch); branch != "" {
		sess["b"] = branch
	}
	payload, err := json.Marshal(sess)
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
	name := trackerViewerName(initD)
	if op == "ship" {
		return b.shipCompletedTrackerTask(taskID, payload, viewerUserID, name)
	}
	return b.trackerRequest(op, taskID, payload, viewerUserID, name)
}

// trackerRequest — сам поход к доске MyVibeLab от имени userID.
//
// Отдельно от MiniappTrackerCall, потому что ходить к доске нужно не только по
// нажатию админа: автономный Лео (leo_autonomy.go) ставит задачи сам, и права
// проверять не у кого — там решает состояние в базе, а не initData.
func (b *Bot) trackerRequest(
	op string, taskID int64, payload map[string]any, userID int64, name string,
) (json.RawMessage, error) {
	spec, ok := trackerOps[op]
	if !ok {
		return nil, ErrAdminActionInvalid
	}
	session, err := b.trackerSession(userID, name)
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

	// Модель доски задаёт окружение, а не мини-апп: на тестовом стенде задачи
	// гоняются на Cursor auto, а мини-апп об этом знать не обязан.
	if op == "create" && payload != nil {
		if model := strings.TrimSpace(b.config.BoardModel); model != "" {
			if _, set := payload["model"]; !set {
				payload["model"] = model
			}
		}
		// Без авто-пуша агент закрывает задачу локальным коммитом, статус
		// «выполнено», а на сервер код не уезжает — фича не собирается.
		if _, set := payload["auto_push"]; !set {
			payload["auto_push"] = true
		}
	}
	if op == "sprint_apply" && payload != nil {
		if _, set := payload["auto_push"]; !set {
			payload["auto_push"] = true
		}
	}
	// «Вернуть в работу» раньше слало mode=now без when — доска отвечает
	// «Укажи время в будущем» и задача остаётся на месте.
	normalizeTrackerReschedule(op, payload)

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

// trackerTaskSnapshot — поля задачи, нужные чтобы понять, надо ли собирать на сервере.
type trackerTaskSnapshot struct {
	Status string `json:"status"`
	Branch string `json:"branch"`
	Commit string `json:"commit"`
	Error  string `json:"error"`
}

func parseTrackerTaskSnapshot(raw json.RawMessage) (trackerTaskSnapshot, error) {
	var wrap struct {
		Task trackerTaskSnapshot `json:"task"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return trackerTaskSnapshot{}, err
	}
	if wrap.Task.Status != "" || wrap.Task.Commit != "" {
		return wrap.Task, nil
	}
	var flat trackerTaskSnapshot
	if err := json.Unmarshal(raw, &flat); err != nil {
		return trackerTaskSnapshot{}, err
	}
	return flat, nil
}

// normalizeTrackerReschedule — mode=now без when превращаем в «через 1 мин»:
// так же ставит форма «Сейчас», и доска снова ставит задачу в «Ожидает».
func normalizeTrackerReschedule(op string, payload map[string]any) {
	if op != "reschedule" || payload == nil {
		return
	}
	when, _ := payload["when"].(string)
	if strings.TrimSpace(when) != "" {
		return
	}
	mode, _ := payload["mode"].(string)
	if !strings.EqualFold(strings.TrimSpace(mode), "now") {
		return
	}
	payload["when"] = "через 1 мин"
	delete(payload, "mode")
}

func trackerPayloadTaskID(taskID int64, payload map[string]any) int64 {
	if taskID > 0 {
		return taskID
	}
	if payload == nil {
		return 0
	}
	switch id := payload["id"].(type) {
	case float64:
		return int64(id)
	case int64:
		return id
	case int:
		return int64(id)
	case json.Number:
		n, _ := id.Int64()
		return n
	}
	return 0
}

// shipCompletedTrackerTask — после «выполнено» довести код до сервера:
// изолированную ветку забрать в основную (только на проде), запушить и
// проверить сборку. Иначе карточка зелёная, а контейнер со старым кодом.
func (b *Bot) shipCompletedTrackerTask(taskID int64, payload map[string]any, userID int64, name string) (json.RawMessage, error) {
	taskID = trackerPayloadTaskID(taskID, payload)
	if taskID <= 0 {
		return nil, ErrAdminActionInvalid
	}
	if name == "" {
		name = "Стая"
	}
	raw, err := b.trackerRequest("task", taskID, nil, userID, name)
	if err != nil {
		return nil, err
	}
	task, err := parseTrackerTaskSnapshot(raw)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(task.Status) != "done" || strings.TrimSpace(task.Error) != "" || strings.TrimSpace(task.Commit) == "" {
		out, _ := json.Marshal(map[string]any{"ok": true, "skipped": true})
		return out, nil
	}

	var (
		promoted bool
		pushed   bool
		deployed bool
		problems []string
	)
	// На тестовом стенде (BOARD_BRANCH задан) в основную не забираем — иначе
	// эксперимент уедет в прод. На проде отдельная ветка задачи как раз и
	// мешает сборке: сервер смотрит основную.
	if strings.TrimSpace(task.Branch) != "" && b.config != nil && strings.TrimSpace(b.config.BoardBranch) == "" {
		if _, err := b.trackerRequest("promote", 0, map[string]any{"id": taskID}, userID, name); err != nil {
			problems = append(problems, "перенос: "+err.Error())
		} else {
			promoted = true
		}
	}
	if _, err := b.trackerRequest("push", 0, map[string]any{}, userID, name); err != nil {
		problems = append(problems, "пуш: "+err.Error())
	} else {
		pushed = true
	}
	if _, err := b.trackerRequest("deploy_refresh", 0, map[string]any{}, userID, name); err != nil {
		problems = append(problems, "сборка: "+err.Error())
	} else {
		deployed = true
	}
	if len(problems) > 0 && !pushed && !deployed && !promoted {
		return nil, fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	out, err := json.Marshal(map[string]any{
		"ok":       true,
		"promoted": promoted,
		"pushed":   pushed,
		"deployed": deployed,
		"error":    strings.Join(problems, "; "),
	})
	return out, err
}

// ShipTrackerTaskInBackground — после уведомления доски довести сдачу до
// сервера, не держа вебхук. Пуш и сборка занимают минуты, автору ответ уже ушёл.
func (b *Bot) ShipTrackerTaskInBackground(taskID, authorID int64) {
	if b == nil || taskID <= 0 {
		return
	}
	go func() {
		defer func() {
			if rec := recover(); rec != nil && b.logger != nil {
				b.logger.Errorf("трекер: паника при сборке #%d: %v", taskID, rec)
			}
		}()
		userID := authorID
		if userID <= 0 {
			userID = b.leoBoardUserID()
		}
		var err error
		for attempt := 0; attempt < 4; attempt++ {
			if attempt > 0 {
				time.Sleep(3 * time.Second)
			}
			var raw json.RawMessage
			raw, err = b.shipCompletedTrackerTask(taskID, nil, userID, "Стая")
			if err != nil {
				continue
			}
			var res struct {
				Skipped bool `json:"skipped"`
			}
			_ = json.Unmarshal(raw, &res)
			if !res.Skipped {
				return
			}
			err = nil
		}
		if err != nil {
			if b.logger != nil {
				b.logger.Warnf("трекер: не собрать задачу #%d на сервере: %v", taskID, err)
			}
			note := fmt.Sprintf("Задача #%d выполнена, но сборка на сервере не стартовала: %s", taskID, err.Error())
			if nerr := b.NotifyTrackerResult(authorID, note); nerr != nil && b.logger != nil {
				b.logger.Warnf("трекер: не сообщить о срыве сборки #%d: %v", taskID, nerr)
			}
		}
	}()
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

// NotifyTrackerResult — сообщить о судьбе задачи: автору, а если его нет
// (ставили из чата) или автор не человек (задачу придумал Лео — id меньше
// нуля) — админам стаи.
func (b *Bot) NotifyTrackerResult(authorID int64, text string) error {
	if authorID > 0 {
		return b.NotifyTrackerAuthor(authorID, text)
	}
	targets := b.config.AdminTelegramUserIDs()
	if len(targets) == 0 {
		return fmt.Errorf("некому писать: админы не заданы")
	}
	var firstErr error
	for _, id := range targets {
		if err := b.NotifyTrackerAuthor(id, text); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

package miniappapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"leo-bot/internal/bot"

	initdata "github.com/telegram-mini-apps/init-data-golang"
)

const maxWorkoutPhotoBytes = 6 << 20 // 6 MiB

// absolutePublicBaseFromRequest — публичный origin (https + host) из reverse-proxy (Railway) или r.Host.
// Нужен, чтобы в БД не попадал http://127.0.0.1:PORT при пустом MINIAPP_PUBLIC_BASE_URL — иначе фото в ленте не грузится в Telegram WebView.
func absolutePublicBaseFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	if host == "" {
		return ""
	}
	proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if proto == "" {
		if r.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	proto = strings.ToLower(proto)
	if proto != "http" && proto != "https" {
		proto = "https"
	}
	return proto + "://" + host
}

func sniffImageExt(header []byte) (ext string, ok bool) {
	if len(header) < 3 {
		return "", false
	}
	if header[0] == 0xff && header[1] == 0xd8 {
		return ".jpg", true
	}
	if len(header) >= 8 && string(header[0:8]) == "\x89PNG\r\n\x1a\n" {
		return ".png", true
	}
	if len(header) >= 12 && string(header[0:4]) == "RIFF" && string(header[8:12]) == "WEBP" {
		return ".webp", true
	}
	if len(header) >= 6 && (string(header[0:6]) == "GIF87a" || string(header[0:6]) == "GIF89a") {
		return ".gif", true
	}
	return "", false
}

func contentTypeForExt(ext string) string {
	switch ext {
	case ".jpg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}

// validateWorkoutPhoto читает фото целиком в память (лимит 6 МиБ), проверяет формат по сигнатуре
// и генерирует случайное имя файла. Выбор хранилища (диск/R2) — на стороне вызывающего.
func validateWorkoutPhoto(file io.Reader) (baseName, contentType string, data []byte, errCode string) {
	// Читаем не более лимита+1, чтобы отличить «ровно по лимиту» от «больше лимита».
	data, err := io.ReadAll(io.LimitReader(file, maxWorkoutPhotoBytes+1))
	if err != nil {
		return "", "", nil, "photo_read_error"
	}
	if len(data) > maxWorkoutPhotoBytes {
		return "", "", nil, "photo_too_large"
	}
	header := data
	if len(header) > 32 {
		header = header[:32]
	}
	ext, mimeOK := sniffImageExt(header)
	if !mimeOK {
		return "", "", nil, "unsupported_image"
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", "", nil, "random_error"
	}
	return hex.EncodeToString(token[:]) + ext, contentTypeForExt(ext), data, ""
}

func saveWorkoutPhotoFile(mediaDirAbsolute string, file io.Reader) (baseName string, errCode string) {
	baseName, _, data, errCode := validateWorkoutPhoto(file)
	if errCode != "" {
		return "", errCode
	}
	if err := os.MkdirAll(mediaDirAbsolute, 0750); err != nil {
		return "", "media_dir_error"
	}
	destAbs := filepath.Join(mediaDirAbsolute, baseName)
	if err := os.WriteFile(destAbs, data, 0640); err != nil {
		return "", "media_write_error"
	}
	return baseName, ""
}

func (s *Server) handlePostWorkoutWithPhoto(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	if s.bot == nil || s.token == "" {
		s.jsonErr(w, http.StatusServiceUnavailable, "server_unavailable")
		return
	}
	if err := r.ParseMultipartForm(maxWorkoutPhotoBytes + 65536); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_multipart")
		return
	}
	initD := strings.TrimSpace(r.FormValue("init_data"))
	line := strings.TrimSpace(r.FormValue("text"))
	if initD == "" || line == "" {
		s.jsonErr(w, http.StatusBadRequest, "missing_fields")
		return
	}
	if err := initdata.Validate(initD, s.token, 24*time.Hour); err != nil {
		s.logger.Warnf("miniapp workout init_data invalid: %v", err)
		s.jsonErr(w, http.StatusUnauthorized, "invalid_init_data")
		return
	}
	parsed, err := initdata.Parse(initD)
	if err != nil || parsed.User.ID == 0 {
		s.jsonErr(w, http.StatusBadRequest, "parse_init_data")
		return
	}
	if err := s.bot.AssertMiniAppPackChatAligns(parsed); err != nil {
		if errors.Is(err, bot.ErrMiniAppChatMismatch) {
			s.jsonErr(w, http.StatusConflict, "chat_mismatch")
			return
		}
		s.jsonErr(w, http.StatusInternalServerError, "assert_chat_error")
		return
	}
	fs := r.MultipartForm.File["photo"]
	if len(fs) == 0 {
		s.jsonErr(w, http.StatusBadRequest, "missing_photo")
		return
	}
	file, err := fs[0].Open()
	if err != nil {
		s.jsonErr(w, http.StatusBadRequest, "photo_open_error")
		return
	}
	defer file.Close()

	publicURL, ok := s.storeUploadedPhoto(w, r, file)
	if !ok {
		return
	}
	miniRes := s.bot.ProcessMiniAppPrivateTextWithTrainingPhoto(parsed, line, publicURL)
	if miniRes.Blocked {
		code := miniRes.BlockCode
		if code == "" {
			code = "moderation_blocked"
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(bot.ModerationHTTPStatus(code))
		_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "message": miniRes.ReplyText})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	outm := map[string]any{"ok": true, "photo_url": publicURL}
	if miniRes.Pending {
		outm["pending"] = true
	}
	if miniRes.ReplyText != "" {
		outm["reply_text"] = miniRes.ReplyText
	}
	_ = json.NewEncoder(w).Encode(outm)
}

// storeUploadedPhoto валидирует и сохраняет фото (R2 или локальный диск) и возвращает
// публичный URL. При любой ошибке сам пишет JSON-ошибку в w и возвращает ok=false.
func (s *Server) storeUploadedPhoto(w http.ResponseWriter, r *http.Request, file io.Reader) (publicURL string, ok bool) {
	useR2 := s.r2 != nil
	if useR2 {
		baseName, contentType, data, vErrCode := validateWorkoutPhoto(file)
		if vErrCode != "" {
			status := http.StatusBadRequest
			if vErrCode == "random_error" || vErrCode == "photo_read_error" {
				status = http.StatusInternalServerError
			}
			s.jsonErr(w, status, vErrCode)
			return "", false
		}
		url, upErr := s.r2.Upload(r.Context(), baseName, contentType, data)
		if upErr != nil {
			s.logger.Errorf("miniapp R2 upload: %v", upErr)
			s.jsonErr(w, http.StatusInternalServerError, "media_write_error")
			return "", false
		}
		return url, true
	}
	if s.mediaDirAbsolute == "" {
		s.jsonErr(w, http.StatusBadRequest, "media_not_configured")
		return "", false
	}
	// publicBase нужен только для локального диска: R2 отдаёт полный публичный URL сам.
	// Не перетираем явно заданный публичный base (обычно HTTPS), иначе можно случайно
	// записать внутренний/HTTP host и сломать показ фото в мини-аппе.
	publicBase := strings.TrimRight(strings.TrimSpace(s.publicMediaBase), "/")
	if publicBase == "" {
		if reqBase := absolutePublicBaseFromRequest(r); reqBase != "" {
			publicBase = strings.TrimRight(reqBase, "/")
		}
	}
	if publicBase == "" {
		s.jsonErr(w, http.StatusBadRequest, "media_not_configured")
		return "", false
	}
	baseName, saveErrCode := saveWorkoutPhotoFile(s.mediaDirAbsolute, file)
	if saveErrCode != "" {
		if saveErrCode == "media_dir_error" {
			s.logger.Errorf("miniapp media mkdir: %s", s.mediaDirAbsolute)
		}
		status := http.StatusBadRequest
		switch saveErrCode {
		case "random_error", "media_dir_error", "media_write_error":
			status = http.StatusInternalServerError
		}
		s.jsonErr(w, status, saveErrCode)
		return "", false
	}
	return publicBase + "/api/miniapp/media/" + baseName, true
}

// handlePostPackGroupMessageWithPhoto — отправка сообщения в чат стаи с фото (multipart).
// text может быть пустым (сообщение только с фото). reply_to_id — опц. ответ.
func (s *Server) handlePostPackGroupMessageWithPhoto(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	if s.bot == nil || s.token == "" {
		s.jsonErr(w, http.StatusServiceUnavailable, "server_unavailable")
		return
	}
	if err := r.ParseMultipartForm(maxWorkoutPhotoBytes + 65536); err != nil {
		s.jsonErr(w, http.StatusBadRequest, "invalid_multipart")
		return
	}
	initD := strings.TrimSpace(r.FormValue("init_data"))
	text := strings.TrimSpace(r.FormValue("text"))
	if initD == "" {
		s.jsonErr(w, http.StatusBadRequest, "missing_init_data")
		return
	}
	if utf8.RuneCountInString(text) > maxTextRunes {
		s.jsonErr(w, http.StatusBadRequest, "text_too_long")
		return
	}
	var replyToID int64
	if rid := strings.TrimSpace(r.FormValue("reply_to_id")); rid != "" {
		if v, perr := strconv.ParseInt(rid, 10, 64); perr == nil {
			replyToID = v
		}
	}
	if err := initdata.Validate(initD, s.token, 24*time.Hour); err != nil {
		s.logger.Warnf("miniapp pack group photo init_data invalid: %v", err)
		s.jsonErr(w, http.StatusUnauthorized, "invalid_init_data")
		return
	}
	parsed, err := initdata.Parse(initD)
	if err != nil || parsed.User.ID == 0 {
		s.jsonErr(w, http.StatusBadRequest, "parse_init_data")
		return
	}
	if err := s.bot.AssertMiniAppPackChatAligns(parsed); err != nil {
		if errors.Is(err, bot.ErrMiniAppChatMismatch) {
			s.jsonErr(w, http.StatusConflict, "chat_mismatch")
			return
		}
		s.jsonErr(w, http.StatusInternalServerError, "assert_chat_error")
		return
	}
	fs := r.MultipartForm.File["photo"]
	if len(fs) == 0 {
		s.jsonErr(w, http.StatusBadRequest, "missing_photo")
		return
	}
	file, err := fs[0].Open()
	if err != nil {
		s.jsonErr(w, http.StatusBadRequest, "photo_open_error")
		return
	}
	defer file.Close()

	publicURL, ok := s.storeUploadedPhoto(w, r, file)
	if !ok {
		return
	}
	miniRes, perr := s.bot.ProcessMiniAppPackGroupMessage(parsed, text, replyToID, publicURL)
	if perr != nil {
		if errors.Is(perr, bot.ErrMiniAppChatMismatch) {
			s.jsonErr(w, http.StatusConflict, "chat_mismatch")
			return
		}
		if errors.Is(perr, bot.ErrPackFeedForbidden) {
			s.jsonErr(w, http.StatusForbidden, "forbidden")
			return
		}
		if errors.Is(perr, bot.ErrPackGroupInvalidReply) {
			s.jsonErr(w, http.StatusBadRequest, "invalid_reply")
			return
		}
		if s.jsonModerationErr(w, perr) {
			return
		}
		s.logger.Errorf("pack group photo message: %v", perr)
		s.jsonErr(w, http.StatusInternalServerError, "pack_group_error")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	out := map[string]any{"ok": true, "photo_url": publicURL}
	if miniRes.ReplyText != "" {
		out["reply_text"] = miniRes.ReplyText
	}
	_ = json.NewEncoder(w).Encode(out)
}

func (s *Server) handleGetMiniappMedia(w http.ResponseWriter, r *http.Request) {
	if s.mediaDirAbsolute == "" {
		http.NotFound(w, r)
		return
	}
	baseRaw := filepath.Base(strings.TrimPrefix(r.URL.Path, "/api/miniapp/media/"))
	if baseRaw == "" || baseRaw == "." || strings.Contains(baseRaw, "..") {
		http.NotFound(w, r)
		return
	}
	root := filepath.Clean(s.mediaDirAbsolute)
	full := filepath.Clean(filepath.Join(root, baseRaw))
	rel, err := filepath.Rel(root, full)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		http.NotFound(w, r)
		return
	}
	fi, err := os.Stat(full)
	if err != nil || fi.IsDir() {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, full)
}

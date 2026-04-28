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
	"strings"
	"time"

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

func (s *Server) handlePostWorkoutWithPhoto(w http.ResponseWriter, r *http.Request) {
	corsWriteHeaders(w, r)
	if s.bot == nil || s.token == "" {
		s.jsonErr(w, http.StatusServiceUnavailable, "server_unavailable")
		return
	}
	if s.mediaDirAbsolute == "" {
		s.jsonErr(w, http.StatusBadRequest, "media_not_configured")
		return
	}
	publicBase := strings.TrimRight(strings.TrimSpace(s.publicMediaBase), "/")
	// Не перетираем явно заданный публичный base (обычно HTTPS),
	// иначе можно случайно записать внутренний/HTTP host и сломать показ фото в мини-аппе.
	if publicBase == "" {
		if reqBase := absolutePublicBaseFromRequest(r); reqBase != "" {
			publicBase = strings.TrimRight(reqBase, "/")
		}
	}
	if publicBase == "" {
		s.jsonErr(w, http.StatusBadRequest, "media_not_configured")
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

	snippet := make([]byte, 32)
	n, readErr := io.ReadFull(file, snippet)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && readErr != io.EOF {
		s.jsonErr(w, http.StatusBadRequest, "photo_read_error")
		return
	}
	snippet = snippet[:n]
	ext, mimeOK := sniffImageExt(snippet)
	if !mimeOK {
		s.jsonErr(w, http.StatusBadRequest, "unsupported_image")
		return
	}
	if _, seekErr := file.Seek(0, io.SeekStart); seekErr != nil {
		s.jsonErr(w, http.StatusBadRequest, "photo_seek_error")
		return
	}

	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		s.jsonErr(w, http.StatusInternalServerError, "random_error")
		return
	}
	baseName := hex.EncodeToString(token[:]) + ext
	destAbs := filepath.Join(s.mediaDirAbsolute, baseName)
	if err := os.MkdirAll(s.mediaDirAbsolute, 0750); err != nil {
		s.logger.Errorf("miniapp media mkdir: %v", err)
		s.jsonErr(w, http.StatusInternalServerError, "media_dir_error")
		return
	}
	out, err := os.Create(destAbs)
	if err != nil {
		s.jsonErr(w, http.StatusInternalServerError, "media_write_error")
		return
	}
	if _, err := out.Write(snippet); err != nil {
		out.Close()
		_ = os.Remove(destAbs)
		s.jsonErr(w, http.StatusInternalServerError, "media_write_error")
		return
	}
	written := int64(n)
	left := int64(maxWorkoutPhotoBytes - n)
	nw64, cpErr := io.Copy(out, io.LimitReader(file, left))
	out.Close()
	if cpErr != nil {
		_ = os.Remove(destAbs)
		s.jsonErr(w, http.StatusBadRequest, "photo_copy_error")
		return
	}
	written += nw64
	if written > maxWorkoutPhotoBytes {
		_ = os.Remove(destAbs)
		s.jsonErr(w, http.StatusBadRequest, "photo_too_large")
		return
	}

	publicURL := publicBase + "/api/miniapp/media/" + baseName
	miniRes := s.bot.ProcessMiniAppPrivateTextWithTrainingPhoto(parsed, line, publicURL)
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

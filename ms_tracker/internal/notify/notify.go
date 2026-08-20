package notify

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"leo-tracker/internal/config"
	"leo-tracker/internal/store"
)

func JobDone(cfg config.Config, job store.Job, text string) error {
	url := strings.TrimSpace(cfg.LeoNotifyURL)
	if url == "" || strings.TrimSpace(cfg.NotifySecret) == "" {
		return nil
	}
	taskID := job.SourceTaskID
	if taskID <= 0 {
		taskID = job.ID
	}
	author := job.AuthorID
	token, err := makeToken(cfg.NotifySecret, "notify", cfg.Repo, author)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]any{
		"repo":      cfg.Repo,
		"author_id": author,
		"task_id":   taskID,
		"text":      text,
		"token":     token,
	})
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("leo notify HTTP %d", resp.StatusCode)
	}
	return nil
}

func makeToken(secret, kind, repo string, uid int64) (string, error) {
	payload, err := json.Marshal(map[string]any{
		"k": kind,
		"r": repo,
		"u": uid,
		"n": "Трекер",
		"e": time.Now().Add(5 * time.Minute).Unix(),
	})
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

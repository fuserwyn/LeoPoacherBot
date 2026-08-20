package api

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"leo-tracker/internal/config"
	"leo-tracker/internal/store"
	"leo-tracker/internal/when"
)

type Server struct {
	cfg config.Config
	st  *store.Store
}

func New(cfg config.Config, st *store.Store) *Server {
	return &Server{cfg: cfg, st: st}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /", s.index)
	mux.HandleFunc("GET /api/scheduled", s.auth(s.list))
	mux.HandleFunc("POST /api/scheduled", s.auth(s.create))
	mux.HandleFunc("GET /api/scheduled/{id}", s.auth(s.get))
	mux.HandleFunc("POST /api/scheduled/cancel", s.auth(s.cancel))
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "ms_tracker"})
}

func (s *Server) index(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"service": "ms_tracker",
		"hint":    "Трекер Леопарда. Доска в админке, агент здесь.",
	})
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (s *Server) authorized(r *http.Request) bool {
	want := strings.TrimSpace(s.cfg.TrackerSecret)
	if want == "" {
		return false
	}
	got := strings.TrimSpace(r.Header.Get("X-Tracker-Secret"))
	if got == "" {
		if a := strings.TrimSpace(r.Header.Get("Authorization")); strings.HasPrefix(strings.ToLower(a), "bearer ") {
			got = strings.TrimSpace(a[7:])
		}
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func (s *Server) list(w http.ResponseWriter, _ *http.Request) {
	jobs, err := s.st.List(50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	tasks := make([]map[string]any, 0, len(jobs))
	for _, j := range jobs {
		tasks = append(tasks, jobView(j))
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "tasks": tasks})
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	j, err := s.st.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "task": jobView(j)})
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "непонятное тело"})
		return
	}
	at, label, err := when.Parse(str(body, "when"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	j := store.Job{
		Prompt:       str(body, "prompt"),
		Phase:        str(body, "phase"),
		WhenAt:       at,
		WhenLabel:    label,
		Status:       "pending",
		Model:        str(body, "model_key"),
		AutoPush:     boolVal(body, "auto_push"),
		SourceTaskID: int64Val(body, "source_task_id"),
		SourceNum:    int(int64Val(body, "source_num")),
		AuthorID:     int64Val(body, "author_id"),
	}
	created, err := s.st.Create(j)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	log.Printf("трекер: задача #%d принята (source=%d)", created.ID, created.SourceTaskID)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"id":   created.ID,
		"when": created.WhenLabel,
	})
}

func (s *Server) cancel(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	id := int64Val(body, "id")
	if id == 0 {
		id = int64Val(body, "task_id")
	}
	if err := s.st.Cancel(id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func jobView(j store.Job) map[string]any {
	return map[string]any{
		"id":             j.ID,
		"source_task_id": j.SourceTaskID,
		"num":            j.SourceNum,
		"prompt":         j.Prompt,
		"status":         j.Status,
		"when":           j.WhenLabel,
		"error":          j.Error,
		"result":         j.Result,
		"steps":          j.Steps,
		"branch":         j.Branch,
	}
}

func str(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return strings.TrimSpace(v)
}

func boolVal(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	switch v := m[key].(type) {
	case bool:
		return v
	case string:
		return v == "1" || strings.EqualFold(v, "true")
	default:
		return false
	}
}

func int64Val(m map[string]any, key string) int64 {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case json.Number:
		n, _ := v.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return n
	default:
		return 0
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ... существующий код ...

func (b *Bot) handleTrackerDonate(w http.ResponseWriter, r *http.Request) {
  taskID, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/tracker/"), 10, 64)
  if taskID <= 0 {
    http.Error(w, "invalid task id", http.StatusBadRequest)
    return
  }

  var req struct {
    Stars int `json:"stars"`
  }
  if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    http.Error(w, err.Error(), http.StatusBadRequest)
    return
  }

  if req.Stars != 10 {
    http.Error(w, "only 10 stars donations supported", http.StatusBadRequest)
    return
  }

  // Логика обработки доната
  t, err := b.db.GetTrackerTask(taskID)
  if err != nil {
    http.Error(w, err.Error(), http.StatusNotFound)
    return
  }

  // Обновляем статус задачи или другие данные
  appendTrackerStep(&t, fmt.Sprintf("Донат: +%d ★", req.Stars))
  if err := b.db.SaveTrackerTask(t); err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
  }

  json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// В метод инициализации маршрутов добавить:
func (b *Bot) initRoutes() {
  // ... другие маршруты ...
  b.mux.HandleFunc("/api/tracker/{id}/donate", b.handleTrackerDonate)
}

// ... остальной код файла ...
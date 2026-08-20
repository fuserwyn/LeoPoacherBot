package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"leo-tracker/internal/api"
	"leo-tracker/internal/config"
	"leo-tracker/internal/store"
	"leo-tracker/internal/worker"
)

func main() {
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL обязателен")
	}
	if cfg.TrackerSecret == "" {
		log.Fatal("TRACKER_SECRET обязателен")
	}
	st, err := store.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("база: %v", err)
	}
	defer st.Close()

	stop := make(chan struct{})
	go worker.Loop(cfg, st, stop)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.New(cfg, st).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Printf("ms_tracker слушает :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	close(stop)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"leo-bot/internal/bot"
	"leo-bot/internal/config"
	"leo-bot/internal/database"
	"leo-bot/internal/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if cfg.QdrantURL == "" {
		log.Fatal("QDRANT_URL is required")
	}
	if cfg.OpenRouterAPIKey == "" {
		log.Fatal("OPENROUTER_API_KEY is required for embeddings")
	}

	logLevel := logger.New(cfg.LogLevel)
	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	// Минимальный бот только для BackfillVectorMemory (без Telegram polling).
	b, err := bot.New(cfg, db, logLevel)
	if err != nil {
		log.Fatalf("bot: %v", err)
	}

	total, err := db.CountUserMessages()
	if err != nil {
		log.Fatalf("count messages: %v", err)
	}
	fmt.Printf("Messages in Postgres: %d\n", total)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	start := time.Now()
	indexed, failed, err := b.BackfillVectorMemory(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backfill error: %v\n", err)
	}
	fmt.Printf("Done in %s: indexed=%d failed=%d\n", time.Since(start).Round(time.Second), indexed, failed)
	if err != nil {
		os.Exit(1)
	}
}

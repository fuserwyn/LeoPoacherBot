package config

import (
	"os"
	"strconv"
	"strings"

	// cspell:ignore joho
	"github.com/joho/godotenv"
)

type Config struct {
	APIToken                string
	OwnerID                 int64
	DatabaseURL             string
	LogLevel                string
	OpenRouterAPIKey        string
	OpenRouterModel         string // Модель OpenRouter (по умолчанию deepseek/deepseek-chat)
	OpenRouterEmbeddingModel string
	OpenRouterVisionModel    string
	ScanHistoryOnStart      bool // Сканировать историю при старте (по умолчанию false)
	QdrantURL               string
	QdrantAPIKey            string
	QdrantCollection        string
	QdrantVectorSize        int
	QdrantBackfillOnStart   bool
}

func Load() (*Config, error) {
	// Загружаем .env файл если он существует
	godotenv.Load()

	ownerID, _ := strconv.ParseInt(getEnv("OWNER_ID", "0"), 10, 64)

	// Парсим булевое значение для ScanHistoryOnStart
	scanHistoryOnStart := parseBoolEnv("SCAN_HISTORY_ON_START", false)
	qdrantBackfillOnStart := parseBoolEnv("QDRANT_BACKFILL_ON_START", false)

	vectorSize, _ := strconv.Atoi(getEnv("QDRANT_VECTOR_SIZE", "1536"))
	if vectorSize <= 0 {
		vectorSize = 1536
	}

	return &Config{
		APIToken:                 getEnv("API_TOKEN", ""),
		OwnerID:                  ownerID,
		DatabaseURL:              getEnv("DATABASE_URL", "postgresql://postgres:password@localhost:5432/leo_bot_db?sslmode=disable"),
		LogLevel:                 getEnv("LOG_LEVEL", "info"),
		OpenRouterAPIKey:         getEnv("OPENROUTER_API_KEY", ""),
		OpenRouterModel:          getEnv("OPENROUTER_MODEL", "deepseek/deepseek-chat"),
		OpenRouterEmbeddingModel: getEnv("OPENROUTER_EMBEDDING_MODEL", "openai/text-embedding-3-small"),
		OpenRouterVisionModel:    getEnv("OPENROUTER_VISION_MODEL", "qwen/qwen3-vl-8b-instruct"),
		ScanHistoryOnStart:       scanHistoryOnStart,
		QdrantURL:                getEnv("QDRANT_URL", ""),
		QdrantAPIKey:             getEnv("QDRANT_API_KEY", ""),
		QdrantCollection:         getEnv("QDRANT_COLLECTION", "leopard_chat"),
		QdrantVectorSize:         vectorSize,
		QdrantBackfillOnStart:    qdrantBackfillOnStart,
	}, nil
}

func parseBoolEnv(key string, defaultValue bool) bool {
	v := strings.ToLower(strings.TrimSpace(getEnv(key, "")))
	if v == "" {
		return defaultValue
	}
	return v == "true" || v == "1" || v == "yes"
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

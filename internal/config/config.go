package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	APIToken    string
	OwnerID     int64
	DatabaseURL string
	LogLevel    string
}

func Load() (*Config, error) {
	// Загружаем .env файл если он существует
	godotenv.Load()

	ownerID, _ := strconv.ParseInt(getEnv("OWNER_ID", "0"), 10, 64)

	return &Config{
		APIToken:    getEnv("API_TOKEN", ""),
		OwnerID:     ownerID,
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:password@localhost:5432/leo_bot_db?sslmode=disable"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

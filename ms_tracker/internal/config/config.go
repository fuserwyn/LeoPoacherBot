package config

import (
	"os"
	"strings"
)

type Config struct {
	Port            string
	DatabaseURL     string
	TrackerSecret   string
	OpenRouterKey   string
	OpenRouterModel string
	GithubToken     string
	Repo            string
	Branch          string
	LeoNotifyURL    string
	NotifySecret    string
}

func Load() Config {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}
	model := strings.TrimSpace(os.Getenv("OPENROUTER_MODEL"))
	if model == "" {
		model = "deepseek/deepseek-chat"
	}
	repo := strings.TrimSpace(os.Getenv("BOARD_REPO"))
	if repo == "" {
		repo = "fuserwyn/Fat-Leopard"
	}
	return Config{
		Port:            port,
		DatabaseURL:     strings.TrimSpace(os.Getenv("DATABASE_URL")),
		TrackerSecret:   firstEnv("TRACKER_SECRET", "BOARD_SSO_SECRET"),
		OpenRouterKey:   strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")),
		OpenRouterModel: model,
		GithubToken:     firstEnv("GITHUB_TOKEN", "GH_TOKEN"),
		Repo:            repo,
		Branch:          strings.TrimSpace(os.Getenv("BOARD_BRANCH")),
		LeoNotifyURL:    firstEnv("LEO_NOTIFY_URL", "BOARD_NOTIFY_URL"),
		NotifySecret:    firstEnv("NOTIFY_SECRET", "BOARD_SSO_SECRET"),
	}
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

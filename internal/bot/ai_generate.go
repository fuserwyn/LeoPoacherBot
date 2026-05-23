package bot

import "leo-bot/internal/ai"

// chatOptionsForRoute — шаг 5: параметры генерации по типу запроса.
func chatOptionsForRoute(route AIQueryRoute) ai.ChatOptions {
	switch route.Kind {
	case QueryBanter:
		return ai.ChatOptions{Temperature: 0.85, MaxTokens: 220, FrequencyPenalty: 0.2}
	case QueryIdentity:
		return ai.ChatOptions{Temperature: 0.25, MaxTokens: 180, FrequencyPenalty: 0.35}
	case QueryCreative:
		return ai.ChatOptions{Temperature: 0.75, MaxTokens: 640, FrequencyPenalty: 0.25}
	case QueryVision:
		return ai.ChatOptions{Temperature: 0.35, MaxTokens: 480, FrequencyPenalty: 0.3}
	default:
		return ai.ChatOptions{Temperature: 0.35, MaxTokens: 720, FrequencyPenalty: 0.35}
	}
}

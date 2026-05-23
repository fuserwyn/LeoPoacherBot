package bot

import (
	"strings"
	"unicode/utf8"
)

// QueryKind — тип запроса для роутера (шаг 1).
type QueryKind string

const (
	QueryFactual  QueryKind = "factual"
	QueryCreative QueryKind = "creative"
	QueryBanter   QueryKind = "banter"
	QueryIdentity QueryKind = "identity"
	QueryVision   QueryKind = "vision"
)

// AIQueryRoute — результат классификации запроса.
type AIQueryRoute struct {
	Kind           QueryKind
	SkipSemantic   bool // короткий / banter / identity без фактов
	MinimalContext bool // почти без RAG (banter, «кто ты»)
}

const shortQueryRunes = 48

// classifyAIQuery классифицирует вопрос (шаг 1 пайплайна).
func classifyAIQuery(question string, hasVisual bool) AIQueryRoute {
	q := strings.TrimSpace(strings.ToLower(question))
	if hasVisual {
		return AIQueryRoute{Kind: QueryVision}
	}
	if q == "" {
		return AIQueryRoute{Kind: QueryBanter, SkipSemantic: true, MinimalContext: true}
	}

	short := utf8.RuneCountInString(q) <= shortQueryRunes

	if isIdentityQuery(q) {
		return AIQueryRoute{Kind: QueryIdentity, SkipSemantic: true, MinimalContext: true}
	}
	if isBanterQuery(q, short) {
		return AIQueryRoute{Kind: QueryBanter, SkipSemantic: true, MinimalContext: true}
	}
	if isCreativeQuery(q) {
		return AIQueryRoute{Kind: QueryCreative, SkipSemantic: short}
	}
	if short && !looksFactualQuestion(q) {
		return AIQueryRoute{Kind: QueryBanter, SkipSemantic: true, MinimalContext: true}
	}
	return AIQueryRoute{Kind: QueryFactual, SkipSemantic: short && !looksFactualQuestion(q)}
}

func isIdentityQuery(q string) bool {
	keys := []string{
		"кто ты", "ты кто", "что ты", "ты бот", "ты леопард", "fat leopard",
		"как тебя зовут", "твоё имя", "твое имя", "who are you", "what are you",
	}
	for _, k := range keys {
		if strings.Contains(q, k) {
			return true
		}
	}
	return false
}

func isBanterQuery(q string, short bool) bool {
	if !short {
		return false
	}
	keys := []string{
		"привет", "здаров", "здоров", "хай", "hello", "hi", "ок", "окей", "ага",
		"спасибо", "благодар", "лол", "хах", "😂", "🦁", "+1", "понял", "ясно",
	}
	for _, k := range keys {
		if q == k || strings.HasPrefix(q, k+" ") || strings.HasSuffix(q, " "+k) {
			return true
		}
	}
	return false
}

func isCreativeQuery(q string) bool {
	keys := []string{
		"придумай", "сочини", "напиши стих", "шутк", "анекдот", "сказк",
		"историю про", "фантаз", "creative",
	}
	for _, k := range keys {
		if strings.Contains(q, k) {
			return true
		}
	}
	return false
}

func looksFactualQuestion(q string) bool {
	keys := []string{
		"сколько", "когда", "почему", "зачем", "какой", "какая", "какие", "кто ",
		"где ", "что я", "мой ", "моя ", "мои ", "статистик", "калор", "кубк",
		"серия", "таймер", "больнич", "трениров", "coding", "отчёт", "отчет",
		"вчера", "прогресс", "пол ", "пола", "удалил", "зафиксир",
	}
	for _, k := range keys {
		if strings.Contains(q, k) {
			return true
		}
	}
	return strings.Contains(q, "?")
}

// routerHintForRoute — подсказка для user-промпта (шаг 4).
func routerHintForRoute(route AIQueryRoute, chatType string) string {
	switch route.Kind {
	case QueryBanter:
		return "Шутка или короткая реплика — кратко подыграй в 1–2 предложениях, без статистики и правил."
	case QueryIdentity:
		return "Вопрос о боте — ответь кратко, без лишнего контекста."
	case QueryCreative:
		return "Пользователь просит придумать — можно фантазировать, без длинных дисклеймеров."
	case QueryVision:
		return "Есть изображение — опиши по картинке; шутливые подписи — подыграй."
	default:
		if chatType == "writing" {
			return "Фактический вопрос в чате писательства — опирайся на structured knowledge и переписку."
		}
		if chatType == "coding" {
			return "Фактический вопрос в coding-чате — опирайся на данные и переписку, не уводи в спорт."
		}
		return "Фактический вопрос — ответь по structured knowledge и переписке."
	}
}

func rulesBlockForChatType(chatType string) string {
	switch chatType {
	case "writing":
		return "Режим чата: писательство. Литературный наставник; не уводи в тренировки без запроса."
	case "coding":
		return "Режим чата: программирование. Комментируй разработку, не спорт."
	default:
		return ""
	}
}

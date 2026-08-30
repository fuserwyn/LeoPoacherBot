package config

import (
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"leo-bot/internal/prompts"
)

type Config struct {
	APIToken       string
	OwnerID        int64
	AdminIDs       []int64
	AlphaTesterIDs []int64 // §10: тестеры альфы — события тегируются is_alpha
	DatabaseURL    string
	// TrackerDatabaseURL — Postgres только трекера (карточки, вложения, автономия, jobs).
	// Пусто — доска остаётся в DatabaseURL (тесты и локалка).
	TrackerDatabaseURL    string
	LogLevel              string
	OpenRouterAPIKey      string
	OpenRouterModel       string        // Модель OpenRouter (по умолчанию deepseek/deepseek-chat)
	OpenRouterVisionModel string        // Vision-модель для анализа фото (основная — текстовая); пусто = vision выключен
	OpenRouterTimeout     time.Duration // HTTP-таймаут к OpenRouter (весь запрос + чтение тела)
	ScanHistoryOnStart    bool          // Сканировать историю при старте (по умолчанию false)

	// Платный доступ к Fat Leopard MiniApp (Telegram Payments + ЮKassa).
	// Архитектура mini-app-only: TG-группы как сущности больше нет, MonetizedChatID
	// используется только как стабильный «pack id» в БД (paywall_access_requests / training_state).
	// Способы оплаты: PAYMENT_PROVIDER_TOKEN (карта в Telegram), ЮKassa (YOOKASSA_*),
	// PAYMENT_STARS_ENABLED + сумма звёзд (дополнительно к RUB), либо PAYMENT_CURRENCY=XTR (только звёзды, устаревший режим).
	PaywallEnabled bool
	// PaywallEntryFree — вход в мини-апп бесплатный (PAYWALL_ENTRY_FREE, по умолчанию true).
	// Платёжная инфраструктура при этом остаётся включённой (PAYWALL_ENABLED), потому что
	// оплата нужна для возврата после кика за 8 дней неактивности и для добровольных донатов
	// из профиля. Чтобы вернуть платный вход для новичков — PAYWALL_ENTRY_FREE=false.
	PaywallEntryFree       bool
	MonetizedChatID        int64  // Логический ID стаи в БД (исторически — chat id Telegram-группы; группа больше не используется).
	MonetizedChatInviteURL string // Deprecated: оставлено только для обратной совместимости со старыми env. Не используется в новом флоу.
	// Deprecated: оставлено для совместимости со старыми env, в новом mini-app flow не используется.
	PaywallInviteCreatesJoinRequest bool
	PaymentProviderToken            string // токен провайдера из BotFather (не коммитить в git)
	PaymentCurrency                 string // RUB и др. ISO 4217, либо XTR (Telegram Stars: 1 единица = 1 звезда)
	PaymentAmountMinorUnits         int    // копейки для RUB; для XTR — число звёзд (см. PAYMENT_STARS_AMOUNT / PAYMENT_AMOUNT_MINOR_UNITS)
	PaymentInvoiceTitle             string
	PaymentInvoiceDesc              string
	// Доп. счёт Telegram Stars при PAYMENT_CURRENCY≠XTR (например RUB + ЮKassa и параллельно звёзды).
	PaymentStarsEnabled bool
	PaymentStarsAmount  int

	// ЮKassa (оплата по ссылке); вебхук — отдельный сервис ms_payments (docker-compose payment-webhook).
	YookassaShopID          string
	YookassaSecretKey       string
	YookassaReturnURL       string // redirect после оплаты, https. Группы у нас больше нет — кладём ссылку на бота, например https://t.me/<bot_username>
	YookassaNotificationURL string // POST payment.succeeded на этот URL (лучше задать = публичный URL ms_payments …/api/v1/webhook/payment)
	// Сумма/валюта для CreatePayment (при PAYMENT_CURRENCY=XTR — в рублях из PAYMENT_AMOUNT_RUB / PAYMENT_YOOKASSA_*).
	YookassaAmountMinor int
	YookassaCurrency    string

	// Донаты из профиля мини-аппа: добровольная поддержка проекта, доступ не выдаётся.
	// Звёзды (DONATE_STARS_TIERS + всегда 1 и 5) идут через createInvoiceLink + WebApp.openInvoice,
	// карта РФ (DONATE_CARD_TIERS_RUB) — через ту же ЮKassa, что и платный возврат.
	DonateStarsTiers   []int // номиналы в звёздах, по возрастанию
	DonateCardTiersRub []int // номиналы в рублях, по возрастанию

	// остальные поля конфига...
}

// остальные методы конфига...

func Load() (*Config, error) {
	// код загрузки конфига...

	return &Config{
		// остальные поля конфига...
		DonateStarsTiers:   parseAmountTiers("1,5,10," + getEnv("DONATE_STARS_TIERS", "50,150,500")),
		DonateCardTiersRub: parseAmountTiers(getEnv("DONATE_CARD_TIERS_RUB", "100,300,1000")),
		// остальные поля конфига...
	}, nil
}

// остальные методы конфига...
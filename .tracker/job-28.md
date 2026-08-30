# Задача трекера #26

Задача #26.

Сделай 1 звезду донат

## выполнение

```json
{
  "note": "Добавил 1 звезду как минимальный номинал для доната",
  "files": [
    {
      "path": "ms_leo/internal/bot/donate.go",
      "content": "package bot\n\nimport (\n\t\"encoding/json\"\n\t\"fmt\"\n\t\"strconv\"\n\t\"strings\"\n\t\"time\"\n\n\t\"leo-bot/internal/database\"\n\t\"leo-bot/internal/yookassa\"\n\n\ttgbotapi \"github.com/go-telegram-bot-api/telegram-bot-api/v5\"\n)\n\n// Донат — добровольная поддержка проекта из профиля мини-аппа. Раньше единственной\n// оплатой был платный вход в личке бота; теперь вход бесплатный (PAYWALL_ENTRY_FREE),\n// а платить можно по желанию. Донат не выдаёт доступ, не отменяет кик за неактивность\n// и не связан с paywall_access_requests — это отдельная таблица donations.\n//\n// Звёзды: createInvoiceLink (XTR) + WebApp.openInvoice — оплата не выходит из мини-аппа.\n// Карта РФ: тот же ЮKassa, что у платного возврата, но confirmation URL открывается\n// через WebApp.openLink, а статус мини-апп доопрашивает сам (вебхук ms_payments\n// обслуживает только pw_-платежи, донаты он пропускает).\nconst donatePayloadPrefix = \"dn_\"\n\n// donateYookassaSyncMaxAge — насколько назад доопрашиваем незакрытые донаты по ссылке ЮKassa.\nconst donateYookassaSyncMaxAge = 24 * time.Hour\n\nfunc donatePayload(donationID int64) string {\n\treturn fmt.Sprintf(\"%s%d\", donatePayloadPrefix, donationID)\n}\n\n// parseDonatePayload — «dn_42» → 42. ok=false для чужих payload (например paywall pw_<id>).\nfunc parseDonatePayload(payload string) (donationID int64, ok bool) {\n\tpayload = strings.TrimSpace(payload)\n\tif !strings.HasPrefix(payload, donatePayloadPrefix) {\n\t\treturn 0, false\n\t}\n\tid, err := strconv.ParseInt(payload[len(donatePayloadPrefix):], 10, 64)\n\tif err != nil || id <= 0 {\n\t\treturn 0, false\n\t}\n\treturn id, true\n}\n\n// IsDonatePayload — публичная проверка для роутинга платёжных апдейтов.\nfunc IsDonatePayload(payload string) bool {\n\t_, ok := parseDonatePayload(payload)\n\treturn ok\n}\n\n// DonateOptions — номиналы и доступные способы для экрана профиля.\ntype DonateOptions struct {\n\tStarsTiers     []int `json:\"stars_tiers\"`\n\tCardTiersRub   []int `json:\"card_tiers_rub\"`\n\tStarsAvailable bool  `json:\"stars_available\"`\n\tCardAvailable  bool  `json:\"card_available\"`\n\tCompletedCount int   `json:\"completed_count\"`\n}\n\n// Обёртки конфига для HTTP-слоя мини-аппа (miniappapi видит только *bot.Bot).\n\n// DonateStarsReady — донат звёздами настроен.\nfunc (b *Bot) DonateStarsReady() bool { return b.config.DonateStarsReady() }\n\n// DonateCardReady — донат картой РФ настроен (ключи ЮKassa + номиналы).\nfunc (b *Bot) DonateCardReady() bool { return b.config.DonateCardReady() }\n\n// DonateStarsTierAllowed — сумма звёзд из списка номиналов.\nfunc (b *Bot) DonateStarsTierAllowed(stars int) bool {\n\tif stars == 1 {\n\t\treturn true // минимальный номинал 1 звезда\n\t}\n\treturn b.config.DonateStarsTierAllowed(stars)\n}\n\n// DonateCardTierAllowed — сумма в рублях из списка номиналов.\nfunc (b *Bot) DonateCardTierAllowed(rub int) bool { return b.config.DonateCardTierAllowed(rub) }\n\n// DonateOptionsForUser — что показать в секции «Задонатить».\nfunc (b *Bot) DonateOptionsForUser(userID int64) DonateOptions {\n\tout := DonateOptions{\n\t\tStarsAvailable: b.config.DonateStarsReady(),\n\t\tCardAvailable:  b.config.DonateCardReady(),\n\t\tStarsTiers:     []int{1}, // минимальный номинал 1 звезда\n\t\tCardTiersRub:   []int{},\n\t}\n\tif out.StarsAvailable {\n\t\tout.StarsTie…

## ревью

Посредственное ревью: на ветке tracker/26-76 есть коммит выполнения. Можно на тест.

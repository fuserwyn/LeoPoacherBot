package bot

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"leo-bot/internal/yookassa"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const paywallPayloadPrefix = "pw_"

const paywallCallbackResendInvoice = "paywall_resend_invoice" // совместимость со старыми сообщениями
const paywallCallbackPayStars = "paywall_pay_stars"
const paywallCallbackPayYookassa = "paywall_pay_yookassa"
const paywallCallbackPayProvider = "paywall_pay_provider"
const paywallCallbackReturnToPack = "paywall_return_to_pack"
const paywallCallbackBackToMethods = "paywall_back_to_methods"

// Несколько попыток: вебхук ЮKassa может закрыть заявку в БД чуть позже события успешной оплаты.
const paywallAccessRecheckAttempts = 5
const paywallAccessRecheckDelay = 800 * time.Millisecond

func (b *Bot) userHasActivePaywallAccessResilient(userID, chatID int64) (bool, error) {
	var lastErr error
	for i := 0; i < paywallAccessRecheckAttempts; i++ {
		if i > 0 {
			time.Sleep(paywallAccessRecheckDelay)
		}
		ok, err := b.db.UserHasActivePaywallAccess(userID, chatID)
		if err != nil {
			lastErr = err
			continue
		}
		lastErr = nil
		if ok {
			return true, nil
		}
	}
	if lastErr != nil {
		return false, lastErr
	}
	return false, nil
}

// paywallInvoiceErrLog — строка для логов (полная диагностика).
func paywallInvoiceErrLog(err error) string {
	if err == nil {
		return ""
	}
	var tgErr *tgbotapi.Error
	if errors.As(err, &tgErr) {
		return fmt.Sprintf("telegram error_code=%d: %s", tgErr.Code, tgErr.Message)
	}
	return err.Error()
}

// paywallInvoiceShortHintForUser — коротко, без переменных окружения (детали только в логах).
func paywallInvoiceShortHintForUser(err error) string {
	if err == nil {
		return ""
	}
	var tgErr *tgbotapi.Error
	if !errors.As(err, &tgErr) {
		return "Попробуй ещё раз чуть позже или другой способ оплаты."
	}
	m := strings.ToLower(tgErr.Message)
	switch {
	case strings.Contains(m, "payment_provider_invalid"):
		return "Счёт в Telegram сейчас недоступен. Попробуй другой способ кнопкой ниже."
	case strings.Contains(m, "currency_invalid"), strings.Contains(m, "currency_total_amount_invalid"):
		return "Платёж не прошёл проверку. Нажми /start и запроси счёт снова."
	case strings.Contains(m, "invoice_payload_invalid"), strings.Contains(m, "invoice_invalid"):
		return "Счёт отклонён Telegram. Обнови приложение или выбери оплату картой другой кнопкой."
	case tgErr.Code == 403 || strings.Contains(m, "blocked"):
		return "Разблокируй бота: ⋮ в чате → Разблокировать."
	case strings.Contains(m, "chat not found") || strings.Contains(m, "user is deactivated"):
		return "Напиши боту любое сообщение в личке и снова нажми кнопку."
	default:
		return "Не вышло отправить счёт. Попробуй оплату картой другой кнопкой или /start позже."
	}
}

// paywallYookassaShortHintForUser — понятная подсказка пользователю по типовым сбоям создания ссылки ЮKassa.
func paywallYookassaShortHintForUser(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, "credentials empty"):
		return "Оплата ЮKassa временно недоступна (не заданы ключи). Напиши администратору."
	case strings.Contains(msg, "amount must be positive"):
		return "Оплата ЮKassa временно недоступна (некорректная сумма). Напиши администратору."
	case strings.Contains(msg, "return_url must be http"):
		return "Оплата ЮKassa временно недоступна (некорректный URL возврата). Напиши администратору."
	case strings.Contains(msg, "http 401"), strings.Contains(msg, "http 403"):
		return "ЮKassa отклонила запрос (проверь ключи магазина). Попробуй позже."
	case strings.Contains(msg, "http 400"):
		return "ЮKassa вернула ошибку параметров платежа. Попробуй позже."
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline exceeded"):
		return "ЮKassa долго отвечает. Попробуй ещё раз через минуту."
	default:
		return "Ссылка на оплату не создалась. Попробуй позже."
	}
}

// paywallActive — платный вход включён и задана целевая группа.
func (b *Bot) paywallActive() bool {
	return b.config.PaywallEnabled && b.config.MonetizedChatID != 0
}

// paywallPriceYookassaShort — короткая «210 ₽» / «210,50 ₽» / «100 USD» для кнопки/UI.
func (b *Bot) paywallPriceYookassaShort() string {
	if !b.config.PaywallYookassaReady() {
		return ""
	}
	yk := b.config.YookassaAmountMinor
	yc := strings.TrimSpace(b.config.YookassaCurrency)
	if yk <= 0 {
		return ""
	}
	if yc == "RUB" {
		rub := yk / 100
		kop := yk % 100
		if kop == 0 {
			return fmt.Sprintf("%d ₽", rub)
		}
		return fmt.Sprintf("%d,%02d ₽", rub, kop)
	}
	if yc != "" {
		return fmt.Sprintf("%d %s", yk, yc)
	}
	return ""
}

// paywallPriceProviderShort — короткая цена для Telegram Provider Invoice (карта в TG).
func (b *Bot) paywallPriceProviderShort() string {
	if !b.config.PaywallUsesTelegramProviderInvoice() {
		return ""
	}
	am := b.config.PaymentAmountMinorUnits
	cur := strings.TrimSpace(b.config.PaymentCurrency)
	if am <= 0 || cur == "" || cur == "XTR" {
		return ""
	}
	if cur == "RUB" {
		rub := am / 100
		kop := am % 100
		if kop == 0 {
			return fmt.Sprintf("%d ₽", rub)
		}
		return fmt.Sprintf("%d,%02d ₽", rub, kop)
	}
	return fmt.Sprintf("%d %s", am, cur)
}

// paywallPriceStarsShort — «350 ⭐» для кнопки.
func (b *Bot) paywallPriceStarsShort() string {
	n := b.config.PaywallStarsInvoiceAmount()
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("%d ⭐", n)
}

// paywallPrivateUnpaidUserText — только оплата и шаги (без полной справки бота).
// Цены вынесены прямо в кнопки выбора способа (см. paywallUnpaidInlineKeyboard) — здесь не дублируем.
//
// Модель доступа: разовая оплата. После успешной оплаты доступ к мини-аппу не истекает по сроку;
// единственное, что его закрывает — кик за 8 дней неактивности (после кика нужна повторная покупка).
// Поэтому никаких «на 30 дней» в текстах не пишем (см. отдельную просьбу пользователя).
func (b *Bot) paywallPrivateUnpaidUserText() string {
	if !b.config.PaywallPaymentReady() {
		return `💳 Платный доступ к Fat Leopard MiniApp

⚠️ Оплата у бота ещё не настроена. Напиши администратору.

Когда заработает — здесь появятся кнопки выбора способа оплаты.`
	}

	return `
Рык! Платный вход. Цена невысокая, но она позволит моим создателям развивать проект для вас.

Выбери как оплатить - картой для РФ или звёздами телеграмма для любой страны.

После оплаты ты получишь безлимитный доступ в стаю. Пока я тебя не съем....

👇 Выбери способ оплаты:`
}

// paywallUnpaidInlineKeyboard — отдельные кнопки под каждый способ оплаты.
// Цена пишется прямо в подписи кнопки, чтобы юзер не получал отдельных сообщений
// с дублированием цены (см. требование пользователя «и в карте и в звёздах сразу
// в способах оплаты писать стоимость»).
func (b *Bot) paywallUnpaidInlineKeyboard() *tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	if b.config.PaywallYookassaReady() {
		label := "💳 Банковской картой — для РФ"
		if p := b.paywallPriceYookassaShort(); p != "" {
			label = "💳 Банковской картой — для РФ — " + p
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, paywallCallbackPayYookassa),
		))
	}
	if b.config.PaywallUsesTelegramProviderInvoice() {
		label := "💳 Банковская карта (в Telegram)"
		if p := b.paywallPriceProviderShort(); p != "" {
			label = "💳 Банковская карта (в Telegram) — " + p
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, paywallCallbackPayProvider),
		))
	}
	if b.config.PaywallUsesStars() {
		label := "⭐ Звёздами Telegram"
		if p := b.paywallPriceStarsShort(); p != "" {
			label = "⭐ Звёздами Telegram — " + p
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, paywallCallbackPayStars),
		))
	}
	if len(rows) == 0 {
		return nil
	}
	return &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func (b *Bot) paywallReturnInlineKeyboard() *tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	if b.config.PaywallYookassaReady() {
		label := "💳 Банковской картой — для РФ"
		if p := b.paywallPriceYookassaShort(); p != "" {
			label = "💳 Банковской картой — для РФ — " + p
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, paywallCallbackPayYookassa),
		))
	}
	if b.config.PaywallUsesTelegramProviderInvoice() {
		label := "💳 Оплатить картой (Telegram)"
		if p := b.paywallPriceProviderShort(); p != "" {
			label = "💳 Оплатить картой (Telegram) — " + p
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, paywallCallbackPayProvider),
		))
	}
	if b.config.PaywallUsesStars() {
		label := "⭐ Звёздами Telegram"
		if p := b.paywallPriceStarsShort(); p != "" {
			label = "⭐ Звёздами Telegram — " + p
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, paywallCallbackPayStars),
		))
	}
	if len(rows) == 0 {
		return nil
	}
	return &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// paywallReturnPromptText — текст экрана покупки доступа (после кика за неактивность или из inline-кнопки «Вернуться»).
// Группы у нас больше нет, и доступ — разовая покупка без срока (см. paywallPrivateUnpaidUserText), поэтому ни
// «возвращения», ни «вступления», ни «N дней» в тексте. Цена прямо на кнопках (см. paywallReturnInlineKeyboard).
// Параметр оставлен в сигнатуре для совместимости со старыми вызовами/тестами, но не подставляется.
func paywallReturnPromptText(_ string) string {
	return "🚪 Вход в Fat Leopard MiniApp открывается после оплаты.\n\nВыбери способ оплаты ниже — цена указана прямо на кнопке."
}

func starsWordRU(n int) string {
	n = n % 100
	if n >= 11 && n <= 14 {
		return "звёзд"
	}
	switch n % 10 {
	case 1:
		return "звезда"
	case 2, 3, 4:
		return "звезды"
	default:
		return "звёзд"
	}
}

func (b *Bot) paywallStarsMethodText() string {
	stars := b.config.PaywallStarsInvoiceAmount()
	if stars <= 0 {
		stars = 1
	}
	return fmt.Sprintf(
		"⭐ Оплата звёздами Telegram.\n\n%d %s спишется с твоего баланса.\nПосле успешной оплаты ты сразу получишь кнопку доступа в мини-апп приложение Леопарда в этом чате.\n\nЕсли с оплатой будет проблема, напиши запрос в нашу поддержку (почта).",
		stars, starsWordRU(stars),
	)
}

func (b *Bot) paywallCardMethodText() string {
	price := b.paywallPriceYookassaShort()
	if strings.TrimSpace(price) == "" {
		price = "сумма по тарифу"
	}
	return fmt.Sprintf(
		"💳 Оплата банковской картой. Доступна картами РФ.\n\n%s спишется и после успешной оплаты ты сразу получишь кнопку доступа в мини-апп приложение Леопарда в этом чате.\n\nЕсли с оплатой будет проблема, напиши запрос в нашу поддержку (почта).",
		price,
	)
}

func (b *Bot) paywallStarsMethodInlineKeyboard() *tgbotapi.InlineKeyboardMarkup {
	return &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", paywallCallbackBackToMethods),
			),
		},
	}
}

func (b *Bot) paywallCardMethodInlineKeyboard(confirmURL string) *tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonURL("💳 Перейти к оплате (ЮKassa)", confirmURL),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", paywallCallbackBackToMethods),
	))
	return &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func (b *Bot) paywallNotifyUser(userID int64, text string) {
	if userID == 0 {
		return
	}
	if _, err := b.api.Send(tgbotapi.NewMessage(userID, text)); err != nil {
		b.logger.Errorf("paywall user notify: %v", err)
	}
}

// ensurePaywallInvoiceSent создаёт pending-заявку и подтягивает оплату ЮKassa при /start; счета не шлёт — пользователь жмёт кнопки.
func (b *Bot) ensurePaywallInvoiceSent(userID int64) {
	if !b.paywallActive() || userID == 0 {
		return
	}
	if !b.config.PaywallPaymentReady() {
		return
	}
	if ok, err := b.db.UserHasActivePaywallAccess(userID, b.config.MonetizedChatID); err != nil {
		b.logger.Errorf("paywall ensure invoice access check: %v", err)
	} else if ok {
		return
	}
	if b.config.PaywallYookassaReady() {
		if b.paywallTrySyncYookassaPayment(userID) {
			return
		}
	}
	pending, err := b.db.GetLatestPendingPaywallAccessRequest(userID, b.config.MonetizedChatID)
	if err != nil {
		b.logger.Errorf("paywall ensure invoice get pending: %v", err)
		b.paywallNotifyUser(userID, "⚠️ Временная ошибка. Попробуй /start чуть позже.")
		return
	}
	if pending != nil {
		return
	}
	if _, err := b.db.InsertPaywallAccessRequest(userID, b.config.MonetizedChatID); err != nil {
		b.logger.Errorf("paywall ensure invoice insert: %v", err)
		b.paywallNotifyUser(userID, "⚠️ Не удалось начать оплату. Попробуй /start снова.")
	}
}

// paywallGetOrCreatePendingReqID — для callback: последняя pending-заявка или новая.
func (b *Bot) paywallGetOrCreatePendingReqID(userID int64) (int64, error) {
	rec, err := b.db.GetLatestPendingPaywallAccessRequest(userID, b.config.MonetizedChatID)
	if err != nil {
		return 0, err
	}
	if rec != nil {
		return rec.ID, nil
	}
	return b.db.InsertPaywallAccessRequest(userID, b.config.MonetizedChatID)
}

// paywallSendPaymentOffers — всё сразу (старые кнопки «выслать снова»); ошибки пользователю короткие, детали в логах.
func (b *Bot) paywallSendPaymentOffers(userID, reqID int64) {
	if b.config.PaywallYookassaReady() {
		if err := b.SendYookassaPaymentLink(userID, reqID); err != nil {
			b.logger.Errorf("paywall yookassa link: %v", err)
			b.paywallNotifyUser(userID, "⚠️ "+paywallYookassaShortHintForUser(err))
		}
	}
	if b.config.PaywallUsesTelegramProviderInvoice() {
		if err := b.SendPaywallProviderInvoice(userID, reqID); err != nil {
			b.logger.Errorf("paywall provider invoice: %s", paywallInvoiceErrLog(err))
			b.paywallNotifyUser(userID, "⚠️ "+paywallInvoiceShortHintForUser(err))
		}
	}
	if b.config.PaywallUsesStars() {
		if err := b.SendPaywallStarsInvoice(userID, reqID); err != nil {
			b.logger.Errorf("paywall stars invoice: %s", paywallInvoiceErrLog(err))
			b.paywallNotifyUser(userID, "⚠️ "+paywallInvoiceShortHintForUser(err))
		}
	}
}

// SendPaywallStarsInvoice — XTR, provider_token пустой; payload pw_<reqID>.
func paywallInvoiceClipTitle(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "Доступ"
	}
	r := []rune(s)
	if len(r) > 32 {
		return string(r[:32])
	}
	return s
}

func paywallInvoiceClipDescription(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "Оплата доступа"
	}
	r := []rune(s)
	if len(r) > 255 {
		return string(r[:255])
	}
	return s
}

func (b *Bot) SendPaywallStarsInvoice(userID, reqID int64) error {
	if !b.config.PaywallUsesStars() {
		return fmt.Errorf("stars payment not configured")
	}
	amt := b.config.PaywallStarsInvoiceAmount()
	if amt <= 0 {
		return fmt.Errorf("stars amount invalid")
	}
	payload := fmt.Sprintf("%s%d", paywallPayloadPrefix, reqID)
	prices := []tgbotapi.LabeledPrice{{Label: "Доступ", Amount: amt}}
	inv := tgbotapi.NewInvoice(
		userID,
		paywallInvoiceClipTitle(b.config.PaymentInvoiceTitle),
		paywallInvoiceClipDescription(b.config.PaymentInvoiceDesc),
		payload,
		"",
		"",
		"XTR",
		prices,
	)
	// Workaround for Telegram API validation: library may encode nil as null.
	// Telegram expects an array for suggested_tip_amounts when field is present.
	inv.SuggestedTipAmounts = []int{}
	_, err := b.api.Send(inv)
	return err
}

// SendPaywallProviderInvoice — RUB/др. через PAYMENT_PROVIDER_TOKEN; payload pw_<reqID>.
func (b *Bot) SendPaywallProviderInvoice(userID, reqID int64) error {
	if !b.config.PaywallUsesTelegramProviderInvoice() {
		return fmt.Errorf("telegram provider invoice not configured")
	}
	tok := strings.TrimSpace(b.config.PaymentProviderToken)
	payload := fmt.Sprintf("%s%d", paywallPayloadPrefix, reqID)
	prices := []tgbotapi.LabeledPrice{{Label: "Доступ", Amount: b.config.PaymentAmountMinorUnits}}
	inv := tgbotapi.NewInvoice(
		userID,
		paywallInvoiceClipTitle(b.config.PaymentInvoiceTitle),
		paywallInvoiceClipDescription(b.config.PaymentInvoiceDesc),
		payload,
		tok,
		"",
		b.config.PaymentCurrency,
		prices,
	)
	inv.SuggestedTipAmounts = []int{}
	_, err := b.api.Send(inv)
	return err
}

// paywallCreateYookassaPayment создаёт платёж в ЮKassa и возвращает confirmation URL.
// Сообщение пользователю НЕ отправляет — это нужно для нового флоу с editMessage в callback'е,
// где URL-кнопка ставится прямо в исходное сообщение «Выбери способ оплаты», без второго экрана
// «вот тебе ещё одна кнопка Оплатить» (см. требование пользователя).
func (b *Bot) paywallCreateYookassaPayment(userID, reqID int64) (string, error) {
	if b.config.YookassaShopID == "" || b.config.YookassaSecretKey == "" {
		return "", fmt.Errorf("yookassa credentials empty")
	}
	returnURL := strings.TrimSpace(b.config.YookassaReturnURL)
	if returnURL == "" {
		returnURL = "https://t.me"
	}
	meta := map[string]string{
		"user_telegram_id": fmt.Sprintf("%d", userID),
		"invoice_payload":  fmt.Sprintf("%s%d", paywallPayloadPrefix, reqID),
	}
	paymentID, confirmURL, err := yookassa.CreatePayment(
		b.config.YookassaShopID,
		b.config.YookassaSecretKey,
		b.config.YookassaAmountMinor,
		b.config.YookassaCurrency,
		b.config.PaymentInvoiceDesc,
		returnURL,
		b.config.YookassaNotificationURL,
		meta,
	)
	if err != nil {
		return "", err
	}
	if err := b.db.SetPaywallYookassaPaymentID(reqID, paymentID); err != nil {
		b.logger.Warnf("paywall SetPaywallYookassaPaymentID: %v", err)
	}
	return confirmURL, nil
}

// SendYookassaPaymentLink создаёт платёж в ЮKassa и шлёт пользователю отдельное сообщение
// с кнопкой-ссылкой. Используется legacy-флоу paywallSendPaymentOffers (resend invoice),
// где мы шлём ВСЕ доступные способы сразу. В обновлённом per-method-callback'е используем
// paywallCreateYookassaPayment + edit исходного сообщения (ниже в handlePaywallPayYookassaCallback).
func (b *Bot) SendYookassaPaymentLink(userID, reqID int64) error {
	confirmURL, err := b.paywallCreateYookassaPayment(userID, reqID)
	if err != nil {
		return err
	}
	text := `💳 Оплата доступа картой (ЮKassa).`
	msg := tgbotapi.NewMessage(userID, text)
	msg.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonURL("Оплатить", confirmURL)),
		},
	}
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) handlePaywallResendInvoiceCallback(callback *tgbotapi.CallbackQuery) {
	if callback.From == nil {
		_, _ = b.api.Request(tgbotapi.NewCallback(callback.ID, ""))
		return
	}
	if !b.paywallActive() || !b.config.PaywallPaymentReady() {
		_, _ = b.api.Request(tgbotapi.NewCallbackWithAlert(callback.ID, "Оплата сейчас недоступна."))
		return
	}
	if b.config.PaywallYookassaReady() {
		if b.paywallTrySyncYookassaPayment(callback.From.ID) {
			_, _ = b.api.Request(tgbotapi.NewCallback(callback.ID, "Оплата уже учтена. Нажми /start."))
			return
		}
	}
	reqID, err := b.paywallGetOrCreatePendingReqID(callback.From.ID)
	if err != nil {
		b.logger.Errorf("paywall resend pending: %v", err)
		_, _ = b.api.Request(tgbotapi.NewCallbackWithAlert(callback.ID, "Ошибка. Попробуй /start."))
		return
	}
	b.paywallSendPaymentOffers(callback.From.ID, reqID)
	_, _ = b.api.Request(tgbotapi.NewCallback(callback.ID, "Проверь новые сообщения в чате."))
}

func (b *Bot) handlePaywallPayStarsCallback(callback *tgbotapi.CallbackQuery) {
	if callback.From == nil {
		_, _ = b.api.Request(tgbotapi.NewCallback(callback.ID, ""))
		return
	}
	uid := callback.From.ID
	if !b.paywallActive() || !b.config.PaywallPaymentReady() || !b.config.PaywallUsesStars() {
		_, _ = b.api.Request(tgbotapi.NewCallbackWithAlert(callback.ID, "Счёт на звёзды сейчас недоступен."))
		return
	}
	if b.config.PaywallYookassaReady() && b.paywallTrySyncYookassaPayment(uid) {
		_, _ = b.api.Request(tgbotapi.NewCallback(callback.ID, "Оплата уже учтена. Нажми /start."))
		return
	}
	reqID, err := b.paywallGetOrCreatePendingReqID(uid)
	if err != nil {
		b.logger.Errorf("paywall stars cb pending: %v", err)
		_, _ = b.api.Request(tgbotapi.NewCallbackWithAlert(callback.ID, "Ошибка. Попробуй /start."))
		return
	}
	// Сначала текст + только «Назад»; счёт Telegram со встроенной кнопкой «Заплатить» — следующим сообщением.
	msg := tgbotapi.NewMessage(uid, b.paywallStarsMethodText())
	msg.ReplyMarkup = b.paywallStarsMethodInlineKeyboard()
	if _, err := b.api.Send(msg); err != nil {
		b.logger.Warnf("paywall stars callback send step message: %v", err)
	}
	if err := b.SendPaywallStarsInvoice(uid, reqID); err != nil {
		b.logger.Errorf("paywall stars invoice: %s", paywallInvoiceErrLog(err))
		h := paywallInvoiceShortHintForUser(err)
		if len(h) > 180 {
			h = h[:177] + "…"
		}
		_, _ = b.api.Request(tgbotapi.NewCallbackWithAlert(callback.ID, h))
		return
	}
	_, _ = b.api.Request(tgbotapi.NewCallback(callback.ID, "Счёт на звёзды отправлен — нажми «Заплатить» в сообщении ниже."))
}

func (b *Bot) handlePaywallPayYookassaCallback(callback *tgbotapi.CallbackQuery) {
	if callback.From == nil {
		_, _ = b.api.Request(tgbotapi.NewCallback(callback.ID, ""))
		return
	}
	uid := callback.From.ID
	if !b.paywallActive() || !b.config.PaywallPaymentReady() || !b.config.PaywallYookassaReady() {
		_, _ = b.api.Request(tgbotapi.NewCallbackWithAlert(callback.ID, "Оплата картой сейчас недоступна."))
		return
	}
	if b.paywallTrySyncYookassaPayment(uid) {
		_, _ = b.api.Request(tgbotapi.NewCallback(callback.ID, "Оплата уже учтена. Нажми /start."))
		return
	}
	reqID, err := b.paywallGetOrCreatePendingReqID(uid)
	if err != nil {
		b.logger.Errorf("paywall yk cb pending: %v", err)
		_, _ = b.api.Request(tgbotapi.NewCallbackWithAlert(callback.ID, "Ошибка. Попробуй /start."))
		return
	}

	confirmURL, err := b.paywallCreateYookassaPayment(uid, reqID)
	if err != nil {
		b.logger.Errorf("paywall yookassa create payment: %v", err)
		h := paywallYookassaShortHintForUser(err)
		if len(h) > 180 {
			h = h[:177] + "…"
		}
		_, _ = b.api.Request(tgbotapi.NewCallbackWithAlert(callback.ID, h))
		return
	}

	// Не плодим новое сообщение «вот тебе ещё одна кнопка Оплатить» (см. требование пользователя):
	// заменяем кнопки выбора способа в исходном сообщении на одну URL-кнопку с готовой ссылкой ЮKassa.
	// Юзер делает один тап и сразу попадает на страницу оплаты — без промежуточного экрана.
	payButton := b.paywallCardMethodInlineKeyboard(confirmURL)
	step := tgbotapi.NewMessage(uid, b.paywallCardMethodText())
	step.ReplyMarkup = payButton
	if _, err := b.api.Send(step); err != nil {
		b.logger.Errorf("paywall yk callback send step message: %v", err)
		_, _ = b.api.Request(tgbotapi.NewCallbackWithAlert(callback.ID, "Не удалось открыть оплату. Попробуй /start."))
		return
	}
	_, _ = b.api.Request(tgbotapi.NewCallback(callback.ID, ""))
}

func (b *Bot) handlePaywallBackToMethodsCallback(callback *tgbotapi.CallbackQuery) {
	if callback == nil || callback.From == nil {
		return
	}
	if !b.paywallActive() || !b.config.PaywallPaymentReady() {
		_, _ = b.api.Request(tgbotapi.NewCallbackWithAlert(callback.ID, "Оплата сейчас недоступна."))
		return
	}
	msg := tgbotapi.NewMessage(callback.From.ID, b.paywallPrivateUnpaidUserText())
	msg.ReplyMarkup = b.paywallUnpaidInlineKeyboard()
	if _, err := b.api.Send(msg); err != nil {
		b.logger.Errorf("paywall back to methods send: %v", err)
		_, _ = b.api.Request(tgbotapi.NewCallbackWithAlert(callback.ID, "Не удалось вернуть экран оплаты. Напиши /start."))
		return
	}
	_, _ = b.api.Request(tgbotapi.NewCallback(callback.ID, ""))
}

func (b *Bot) handlePaywallPayProviderCallback(callback *tgbotapi.CallbackQuery) {
	if callback.From == nil {
		_, _ = b.api.Request(tgbotapi.NewCallback(callback.ID, ""))
		return
	}
	uid := callback.From.ID
	if !b.paywallActive() || !b.config.PaywallPaymentReady() || !b.config.PaywallUsesTelegramProviderInvoice() {
		_, _ = b.api.Request(tgbotapi.NewCallbackWithAlert(callback.ID, "Счёт провайдера сейчас недоступен."))
		return
	}
	if b.config.PaywallYookassaReady() && b.paywallTrySyncYookassaPayment(uid) {
		_, _ = b.api.Request(tgbotapi.NewCallback(callback.ID, "Оплата уже учтена. Нажми /start."))
		return
	}
	reqID, err := b.paywallGetOrCreatePendingReqID(uid)
	if err != nil {
		b.logger.Errorf("paywall provider cb pending: %v", err)
		_, _ = b.api.Request(tgbotapi.NewCallbackWithAlert(callback.ID, "Ошибка. Попробуй /start."))
		return
	}
	if err := b.SendPaywallProviderInvoice(uid, reqID); err != nil {
		b.logger.Errorf("paywall provider invoice: %s", paywallInvoiceErrLog(err))
		h := paywallInvoiceShortHintForUser(err)
		if len(h) > 180 {
			h = h[:177] + "…"
		}
		_, _ = b.api.Request(tgbotapi.NewCallbackWithAlert(callback.ID, h))
		return
	}
	_, _ = b.api.Request(tgbotapi.NewCallback(callback.ID, "Счёт отправлен — открой его выше и нажми «Оплатить»."))
}

func (b *Bot) paywallPrivatePaidFooter() string {
	if !b.paywallActive() {
		return ""
	}
	return `

💳 Доступ оплачен. Открывай мини-приложение бота — кнопка внизу в этом чате (или через меню ⋮). Там вся стая.`
}

func (b *Bot) handlePaywallReturnToPackCallback(callback *tgbotapi.CallbackQuery) {
	if callback == nil || callback.From == nil {
		return
	}
	if !b.paywallActive() {
		_, _ = b.api.Request(tgbotapi.NewCallbackWithAlert(callback.ID, "Возврат сейчас недоступен."))
		return
	}

	if !b.paywallPrivateNeedsPayFirst(callback.From.ID) {
		_, _ = b.api.Request(tgbotapi.NewCallback(callback.ID, "Доступ уже активен — открой мини-приложение."))
		return
	}

	msg := tgbotapi.NewMessage(callback.From.ID, paywallReturnPromptText(""))
	msg.ReplyMarkup = b.paywallReturnInlineKeyboard()
	if msg.ReplyMarkup == nil {
		msg.Text = "⚠️ Оплата временно недоступна в твоём регионе. Попробуй позже."
	}
	if _, err := b.api.Send(msg); err != nil {
		b.logger.Errorf("paywall return callback send pay options: %v", err)
		_, _ = b.api.Request(tgbotapi.NewCallbackWithAlert(callback.ID, "Не удалось отправить экран оплаты. Напиши /start."))
		return
	}
	_, _ = b.api.Request(tgbotapi.NewCallback(callback.ID, "Открой сообщение ниже — там выбор оплаты."))
}

// paywallPrivateNeedsPayFirst — личка, paywall включён, не владелец, нет активной (не истёкшей) оплаты.
func (b *Bot) paywallPrivateNeedsPayFirst(userID int64) bool {
	if !b.paywallActive() || userID == 0 {
		return false
	}
	if b.config.OwnerID != 0 && userID == b.config.OwnerID {
		return false
	}
	ok, err := b.db.UserHasActivePaywallAccess(userID, b.config.MonetizedChatID)
	if err != nil {
		b.logger.Errorf("paywall access check: %v", err)
		return true
	}
	return !ok
}

func parsePaywallPayload(payload string) (requestID int64, ok bool) {
	payload = strings.TrimSpace(payload)
	if !strings.HasPrefix(payload, paywallPayloadPrefix) {
		return 0, false
	}
	id, err := strconv.ParseInt(payload[len(paywallPayloadPrefix):], 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// paywallTrySyncYookassaPayment — если вебхук не дошёл, подтягиваем успешную оплату через GET /v3/payments/{id}.
func (b *Bot) paywallTrySyncYookassaPayment(userID int64) bool {
	if !b.paywallActive() || userID == 0 {
		return false
	}
	if !b.config.PaywallYookassaReady() {
		return false
	}
	pending, err := b.db.GetLatestPendingPaywallAccessRequest(userID, b.config.MonetizedChatID)
	if err != nil {
		b.logger.Errorf("paywall yookassa sync get pending: %v", err)
		return false
	}
	if pending == nil {
		return false
	}
	if !pending.YookassaPaymentID.Valid || strings.TrimSpace(pending.YookassaPaymentID.String) == "" {
		b.logger.Warnf(
			"paywall yookassa sync: заявка id=%d user=%d в pending, но yookassa_payment_id пуст — опрос API ЮKassa невозможен. "+
				"Нужны миграция 16, деплой бота с сохранением payment id и новая ссылка на оплату (или рабочий вебхук ms_payments).",
			pending.ID, userID,
		)
		return false
	}
	info, err := yookassa.GetPayment(b.config.YookassaShopID, b.config.YookassaSecretKey, pending.YookassaPaymentID.String)
	if err != nil {
		b.logger.Warnf("paywall yookassa sync GetPayment: %v", err)
		return false
	}
	st := strings.ToLower(strings.TrimSpace(info.Status))
	if st != "succeeded" || !info.Paid {
		return false
	}
	meta := info.Metadata
	userStr := strings.TrimSpace(meta["user_telegram_id"])
	if userStr == "" {
		userStr = strings.TrimSpace(meta["user_telegramId"])
	}
	payloadStr := strings.TrimSpace(meta["invoice_payload"])
	if payloadStr == "" {
		payloadStr = strings.TrimSpace(meta["invoicePayload"])
	}
	if userStr != fmt.Sprintf("%d", userID) {
		b.logger.Warnf("paywall yookassa sync user mismatch meta=%q db_user=%d", userStr, userID)
		return false
	}
	reqFromMeta, ok := parsePaywallPayload(payloadStr)
	if !ok || reqFromMeta != pending.ID {
		b.logger.Warnf("paywall yookassa sync payload mismatch meta=%q want id=%d", payloadStr, pending.ID)
		return false
	}
	amountMinor := info.AmountMinor
	cur := info.Currency
	if amountMinor <= 0 || cur == "" {
		amountMinor = b.config.YookassaAmountMinor
		cur = b.config.YookassaCurrency
	}
	if cur == "" {
		cur = "RUB"
	}
	okDb, err := b.db.CompletePaywallAccessRequestAndEnqueueRestore(pending.ID, userID, b.config.MonetizedChatID, info.ID, amountMinor, cur)
	if err != nil {
		b.logger.Errorf("paywall yookassa sync complete: %v", err)
		return false
	}
	if !okDb {
		paid, err := b.db.UserHasActivePaywallAccess(userID, b.config.MonetizedChatID)
		if err == nil && paid {
			b.logger.Infof("paywall yookassa sync: заявка %d уже completed (вебхук)", pending.ID)
		}
		return false
	}
	b.logger.Infof("paywall yookassa sync: заявка %d закрыта по API ЮKassa, событие восстановления отправлено в outbox", pending.ID)
	return true
}

func (b *Bot) handlePaywallPreCheckout(q *tgbotapi.PreCheckoutQuery) {
	if q.From == nil {
		_, _ = b.api.Request(tgbotapi.PreCheckoutConfig{PreCheckoutQueryID: q.ID, OK: false, ErrorMessage: "Оплата недоступна."})
		return
	}
	telegramInvoice := b.config.PaywallUsesTelegramInvoice()
	if !b.paywallActive() || !telegramInvoice {
		_, _ = b.api.Request(tgbotapi.PreCheckoutConfig{PreCheckoutQueryID: q.ID, OK: false, ErrorMessage: "Оплата недоступна."})
		return
	}
	switch q.Currency {
	case "XTR":
		if !b.config.PaywallUsesStars() || q.TotalAmount != b.config.PaywallStarsInvoiceAmount() {
			b.logger.Warnf("paywall pre_checkout stars mismatch: got %s %d want XTR %d", q.Currency, q.TotalAmount, b.config.PaywallStarsInvoiceAmount())
			_, _ = b.api.Request(tgbotapi.PreCheckoutConfig{PreCheckoutQueryID: q.ID, OK: false, ErrorMessage: "Неверная сумма (звёзды). Обнови заявку /start."})
			return
		}
	default:
		if !b.config.PaywallUsesTelegramProviderInvoice() {
			_, _ = b.api.Request(tgbotapi.PreCheckoutConfig{PreCheckoutQueryID: q.ID, OK: false, ErrorMessage: "Оплата недоступна."})
			return
		}
		if strings.TrimSpace(b.config.PaymentProviderToken) == "" {
			_, _ = b.api.Request(tgbotapi.PreCheckoutConfig{PreCheckoutQueryID: q.ID, OK: false, ErrorMessage: "Оплата недоступна."})
			return
		}
		if q.Currency != b.config.PaymentCurrency || q.TotalAmount != b.config.PaymentAmountMinorUnits {
			b.logger.Warnf("paywall pre_checkout amount mismatch: got %s %d want %s %d", q.Currency, q.TotalAmount, b.config.PaymentCurrency, b.config.PaymentAmountMinorUnits)
			_, _ = b.api.Request(tgbotapi.PreCheckoutConfig{PreCheckoutQueryID: q.ID, OK: false, ErrorMessage: "Неверная сумма. Обнови заявку и попробуй снова."})
			return
		}
	}
	reqID, ok := parsePaywallPayload(q.InvoicePayload)
	if !ok {
		_, _ = b.api.Request(tgbotapi.PreCheckoutConfig{PreCheckoutQueryID: q.ID, OK: false, ErrorMessage: "Некорректный платёж."})
		return
	}
	rec, err := b.db.GetPaywallAccessRequestByID(reqID)
	if err != nil || rec == nil {
		b.logger.Errorf("paywall pre_checkout load request: %v", err)
		_, _ = b.api.Request(tgbotapi.PreCheckoutConfig{PreCheckoutQueryID: q.ID, OK: false, ErrorMessage: "Заявка не найдена. Нажми /start снова."})
		return
	}
	if rec.Status != "pending" {
		_, _ = b.api.Request(tgbotapi.PreCheckoutConfig{PreCheckoutQueryID: q.ID, OK: false, ErrorMessage: "Этот счёт уже неактуален."})
		return
	}
	if rec.UserID != q.From.ID || rec.MonetizedChatID != b.config.MonetizedChatID {
		_, _ = b.api.Request(tgbotapi.PreCheckoutConfig{PreCheckoutQueryID: q.ID, OK: false, ErrorMessage: "Платёж не для этого аккаунта."})
		return
	}
	_, _ = b.api.Request(tgbotapi.PreCheckoutConfig{PreCheckoutQueryID: q.ID, OK: true})
}

// paywallPostPaymentUserText — после успешной оплаты отправляем тот же onboarding,
// что и при /start у уже оплаченного пользователя.
func (b *Bot) paywallPostPaymentUserText() string {
	return `Ура, ты в стае! Видишь внизу синюю кнопку? Это вход в мини-апп. Там все подробные правила стаи.
Там же я буду ждать твои отчеты о тренировках.

Я уже немного проголодался! Достаю вилку и нож..
Чтобы я тебя не съел, достаточно любого движения каждый день. Рык!`
}

// paywallDeliverAccessAfterPayment — DM-приветствие после зачёта оплаты (Telegram Payments / ЮKassa / sync API).
//
// По требованию пользователя «при оплате сразу включается таймер timer_start_time»: окно неактивности
// (5/6/7 дни — предупреждения, 8 день — кик) отсчитываем с момента подтверждения оплаты, а не с первого
// открытия мини-аппа. Это убирает класс багов, когда оплативший никогда не зашёл в мини-апп и формально
// «жил вечно» без отчётов; с другой стороны — у новичка есть полные 8 дней, чтобы освоиться.
//
// Здесь же пишем карточку «вступил/вернулся» в ленту мини-аппа — раньше это делал EnsureMiniAppOnboarding
// при первом заходе. Теперь ENtry-stream единый, а EnsureMiniAppOnboarding для оплаченных видит уже
// активный timer_start_time и просто отдаёт InPack=true.
func (b *Bot) paywallDeliverAccessAfterPayment(userID int64) error {
	chatID := b.config.MonetizedChatID
	reactivated, err := b.db.ReactivateReturnedUser(userID, chatID, "")
	if err != nil {
		b.logger.Errorf("paywall reactivate returned user=%d: %v", userID, err)
		return err
	}
	if !reactivated {
		return fmt.Errorf("paywall inconsistency: no profile for paid return user=%d chat=%d", userID, chatID)
	}

	if _, err := b.api.Send(tgbotapi.NewMessage(userID, b.paywallPostPaymentUserText())); err != nil {
		b.logger.Errorf("paywall send done msg: %v", err)
		return err
	}

	// Username для приветственной карточки берём из только что обновлённой записи training_state.
	// Если пусто — startTimer / savePackJoinMiniappFeed корректно отработают с пустой строкой.
	username := ""
	if ml, mlErr := b.db.GetMessageLog(userID, chatID); mlErr == nil && ml != nil {
		username = strings.TrimSpace(ml.Username)
	}

	// 1) Стартуем таймер активности немедленно — это пишет timer_start_time = NOW в training_state
	//    и регистрирует горутины для дней 5/6/7/8. ReactivateReturnedUser выше выставил
	//    timer_start_time = NULL, поэтому здесь startTimer делает «чистый» старт без cancel.
	b.startTimer(userID, chatID, username)

	// 2) Карточка ленты мини-аппа: pack_join (первая оплата) либо pack_rejoin (после кика).
	//    Различаем по return_count — он инкрементируется в ReactivateReturnedUser при возврате.
	isRejoin := false
	if rc, rcErr := b.db.GetUserReturnCount(userID, chatID); rcErr == nil && rc > 1 {
		isRejoin = true
	}
	if isRejoin {
		b.savePackJoinMiniappFeed(chatID, userID, username, userMessageTypePackRejoin, packRejoinMiniappWelcomeText(username))
	} else {
		b.savePackJoinMiniappFeed(chatID, userID, username, userMessageTypePackJoin, packJoinMiniappWelcomeText(username))
	}

	// 3) Синяя кнопка LeopardMiniApp в ЛС только для paid + не кикнутых.
	//    После успешной оплаты статус «paid» точно действителен — сбрасываем кеш и применяем.
	invalidateMiniappMenuButtonCache(userID)
	b.applyMiniappMenuButtonForUser(userID)
	return nil
}

func (b *Bot) handlePaywallSuccessfulPayment(msg *tgbotapi.Message) {
	if !b.paywallActive() || msg.From == nil || msg.SuccessfulPayment == nil {
		return
	}
	sp := msg.SuccessfulPayment
	switch sp.Currency {
	case "XTR":
		if !b.config.PaywallUsesStars() || sp.TotalAmount != b.config.PaywallStarsInvoiceAmount() {
			b.logger.Errorf(
				"paywall successful_payment stars mismatch: got %d, want %d — PAYMENT_STARS_AMOUNT / XTR",
				sp.TotalAmount, b.config.PaywallStarsInvoiceAmount(),
			)
			return
		}
	default:
		if !b.config.PaywallUsesTelegramProviderInvoice() || sp.Currency != b.config.PaymentCurrency || sp.TotalAmount != b.config.PaymentAmountMinorUnits {
			b.logger.Errorf(
				"paywall successful_payment mismatch: got %s %d, config wants %s %d — провайдер / PAYMENT_AMOUNT_*",
				sp.Currency, sp.TotalAmount, b.config.PaymentCurrency, b.config.PaymentAmountMinorUnits,
			)
			return
		}
	}
	reqID, ok := parsePaywallPayload(sp.InvoicePayload)
	if !ok {
		return
	}
	rec, err := b.db.GetPaywallAccessRequestByID(reqID)
	if err != nil || rec == nil {
		b.logger.Errorf("paywall payment load request: %v", err)
		return
	}
	if rec.UserID != msg.From.ID || rec.MonetizedChatID != b.config.MonetizedChatID {
		b.logger.Warnf("paywall payment user/chat mismatch")
		return
	}
	okDb, err := b.db.CompletePaywallAccessRequestAndEnqueueRestore(reqID, msg.From.ID, b.config.MonetizedChatID, sp.TelegramPaymentChargeID, sp.TotalAmount, sp.Currency)
	if err != nil {
		b.logger.Errorf("paywall complete request: %v", err)
		return
	}
	if !okDb {
		b.logger.Infof("paywall duplicate successful_payment for request=%d user=%d", reqID, msg.From.ID)
		return
	}
}


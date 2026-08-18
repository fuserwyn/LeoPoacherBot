package bot

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Трекер задач Леопарда живёт в MyVibeLab: там доска, задачи и спринты, там же
// агент, который эти задачи выполняет. Заводить админам отдельный аккаунт не
// нужно — мы подписываем ссылку общим секретом (BOARD_SSO_SECRET), и MyVibeLab
// по этой подписи пускает человека РОВНО в доску нашего репозитория: ни IDE,
// ни деплоя, ни чужих проектов там не будет.
//
// Ссылка живёт 5 минут: её видно в истории чата, поэтому долгий доступ выдаёт
// уже сам MyVibeLab — сессионной кукой после первого перехода.
const boardLinkTTL = 5 * time.Minute

// boardLink собирает подписанную ссылку на доску трекера для конкретного админа.
// Формат токена — payload.signature, где payload это base64url(JSON) без
// выравнивания, а signature — HMAC-SHA256 от этого же base64url-текста.
func (b *Bot) boardLink(userID int64, name string) (string, error) {
	if b.config.BoardSecret == "" || b.config.BoardURL == "" || b.config.BoardRepo == "" {
		return "", fmt.Errorf("трекер не настроен: нужны BOARD_SSO_SECRET, MYVIBELAB_URL и BOARD_REPO")
	}
	payload, err := json.Marshal(map[string]any{
		"k": "link",
		"r": b.config.BoardRepo,
		"u": userID,
		"n": name,
		"e": time.Now().Add(boardLinkTTL).Unix(),
	})
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(b.config.BoardSecret))
	mac.Write([]byte(encoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s/board?t=%s.%s", b.config.BoardURL, encoded, signature), nil
}

// canOpenBoard — кому можно ставить задачи в трекер. Владелец и те, кого он
// перечислил в BOARD_ADMIN_IDS. Намеренно отдельный список от админ-панели:
// рассылки и опросы остаются только у владельца, наружу отдаём один трекер.
func (b *Bot) canOpenBoard(userID int64) bool {
	if userID == b.config.OwnerID {
		return true
	}
	for _, id := range b.config.BoardAdminIDs {
		if id == userID {
			return true
		}
	}
	return false
}

// handleTracker — /tracker: кнопка с одноразовой ссылкой на доску.
func (b *Bot) handleTracker(msg *tgbotapi.Message) {
	if msg == nil || msg.From == nil || msg.Chat == nil {
		return
	}
	if !msg.Chat.IsPrivate() {
		b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "🗂 Трекер открываю только в личке — напиши мне в приватный чат."))
		return
	}
	if !b.canOpenBoard(msg.From.ID) {
		b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Доступ к трекеру выдаёт владелец бота."))
		return
	}
	b.sendBoardButton(msg.Chat.ID, msg.From.ID, displayName(msg.From))
}

// sendBoardButton — сообщение с кнопкой на доску. Ссылка одноразовая по смыслу,
// поэтому кнопку шлём заново на каждый запрос, а не храним где-то у себя.
func (b *Bot) sendBoardButton(chatID, userID int64, name string) {
	link, err := b.boardLink(userID, name)
	if err != nil {
		b.logger.Warnf("board link failed: %v", err)
		b.api.Send(tgbotapi.NewMessage(chatID, "🗂 Трекер пока не настроен."))
		return
	}
	text := "🗂 Трекер задач Леопарда\n\n" +
		"Доска, задачи и спринты. Поставленные задачи выполняет агент MyVibeLab.\n" +
		"Ссылка живёт 5 минут — если протухла, вызови /tracker снова."
	out := tgbotapi.NewMessage(chatID, text)
	out.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("🗂 Открыть доску", link),
		),
	)
	b.api.Send(out)
}

// displayName — как подписать гостя на доске: имя и фамилия, иначе @username.
func displayName(user *tgbotapi.User) string {
	if user == nil {
		return ""
	}
	name := user.FirstName
	if user.LastName != "" {
		name += " " + user.LastName
	}
	if name == "" {
		name = user.UserName
	}
	return name
}

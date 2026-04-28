package bot

import (
	"strings"
	"time"

	initdata "github.com/telegram-mini-apps/init-data-golang"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// PrivateTextMessageFromInitUser — синтетическое входящее сообщение, как в личке.
func PrivateTextMessageFromInitUser(d initdata.InitData, text string) *tgbotapi.Message {
	u := d.User
	tgU := tgbotapiUserFromInitData(u)
	return &tgbotapi.Message{
		MessageID: 0,
		From:      tgU,
		Date:      int(time.Now().Unix()),
		Chat:      privateChatForUser(tgU),
		Text:      text,
	}
}

func tgbotapiUserFromInitData(u initdata.User) *tgbotapi.User {
	if u.ID == 0 {
		return nil
	}
	return &tgbotapi.User{
		ID:           u.ID,
		IsBot:        u.IsBot,
		FirstName:    u.FirstName,
		LastName:     u.LastName,
		UserName:     u.Username,
		LanguageCode: u.LanguageCode,
	}
}

func privateChatForUser(u *tgbotapi.User) *tgbotapi.Chat {
	if u == nil {
		return nil
	}
	return &tgbotapi.Chat{
		ID:        u.ID,
		Type:      "private",
		FirstName: u.FirstName,
		LastName:  u.LastName,
		UserName:  u.UserName,
	}
}

// MiniAppTextProcessResult — ответ бота в личку: либо сразу reply_text (редко), либо pending + poll очереди.
type MiniAppTextProcessResult struct {
	ReplyText string `json:"reply_text,omitempty"`
	Pending   bool   `json:"pending,omitempty"`
}

// ProcessMiniAppPrivateText — валидация initData; обработка в фоне, HTTP не ждёт ИИ.
// Текущий текст «без фото»: снимаем отложенный URL фото, чтобы не приклеить к чужому действию.
func (b *Bot) ProcessMiniAppPrivateText(d initdata.InitData, text string) MiniAppTextProcessResult {
	b.clearMiniappTrainingPhotoURL(d.User.ID)
	return b.processMiniAppPrivateCore(d, text)
}

// ProcessMiniAppPrivateTextWithTrainingPhoto — тот же путь плюс публичный URL фото для следующего #training_done.
func (b *Bot) ProcessMiniAppPrivateTextWithTrainingPhoto(d initdata.InitData, text, publicPhotoURL string) MiniAppTextProcessResult {
	b.setMiniappTrainingPhotoURL(d.User.ID, publicPhotoURL)
	return b.processMiniAppPrivateCore(d, text)
}

func (b *Bot) processMiniAppPrivateCore(d initdata.InitData, text string) MiniAppTextProcessResult {
	out := MiniAppTextProcessResult{}
	if text == "" || b == nil {
		return out
	}
	if d.User.ID == 0 {
		return out
	}
	if err := b.AssertMiniAppPackChatAligns(d); err != nil {
		return out
	}
	_ = PrivateTextMessageFromInitUser(d, text)
	b.miniappPersonalClear(d.User.ID)
	b.savePersonalChatMessage(d.User.ID, "user", text)
	go b.runMiniAppPrivateTextWorker(d, text)
	out.Pending = true
	return out
}

func (b *Bot) runMiniAppPrivateTextWorker(d initdata.InitData, text string) {
	defer func() {
		if r := recover(); r != nil {
			b.logger.Errorf("miniapp private worker panic: %v", r)
		}
	}()
	msg := PrivateTextMessageFromInitUser(d, text)
	ch := make(chan string, 32)
	b.markMiniappOrigin(d.User.ID, ch)
	defer b.unmarkMiniappOrigin(d.User.ID)
	b.dispatchTextMessageFromUser(msg, ch)
	for {
		select {
		case t := <-ch:
			tr := strings.TrimSpace(t)
			if tr == "" {
				continue
			}
			b.miniappPersonalPush(d.User.ID, tr)
		default:
			return
		}
	}
}

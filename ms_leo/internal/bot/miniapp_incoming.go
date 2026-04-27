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
func (b *Bot) ProcessMiniAppPrivateText(d initdata.InitData, text string) MiniAppTextProcessResult {
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
	_ = PrivateTextMessageFromInitUser(d, text) // для будущих чек-ов; сейчас сообщение всегда private, отдельный paywall-гейт не нужен
	b.miniappPersonalClear(d.User.ID)
	// Юзер-сообщение из мини-аппа: сохраняем в БД сразу, чтобы оно появилось в истории
	// на всех устройствах одного юзера (см. требование «история чата должна быть общей»).
	b.savePersonalChatMessage(d.User.ID, "user", text)
	// В личку Telegram из мини-аппа не дублируем (ответы Лео по-прежнему в апп через poll).
	// Исключение по смыслу: предупреждения дней 5–7 без отчёта — шлём в апп из timer_leopard (как дубль к DM).
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
	// Пометка mini-app origin позволяет helper'ам notifyUserText / sendChatActionIfTG
	// (sick_leave, healthy, change, timezone, AI-fallback) НЕ дублировать ответ Лео в TG-личку.
	// Снимается строго после drain канала, чтобы handler не успел уйти в TG раньше нас.
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

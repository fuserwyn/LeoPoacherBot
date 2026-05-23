package bot

import (
	"strings"
	"time"

	initdata "github.com/telegram-mini-apps/init-data-golang"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"leo-bot/internal/game/leopardmoney"
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

// MiniAppTextProcessResult — ответ бота в личку: сразу reply_text, либо pending + poll очереди.
type MiniAppTextProcessResult struct {
	ReplyText string `json:"reply_text,omitempty"`
	Pending   bool   `json:"pending,omitempty"`
}

func isMiniappTrainingReport(text string) bool {
	return leopardmoney.IsTrainingReportLine(text)
}

// ProcessMiniAppPrivateText — валидация initData; обработка в фоне, HTTP не ждёт ИИ.
// Текущий текст «без фото»: снимаем отложенный URL фото, чтобы не приклеить к чужому действию.
func (b *Bot) ProcessMiniAppPrivateText(d initdata.InitData, text string) MiniAppTextProcessResult {
	b.clearMiniappTrainingPhotoURL(d.User.ID)
	return b.processMiniAppPrivateCore(d, text, "")
}

// ProcessMiniAppPrivateTextWithTrainingPhoto — тот же путь; URL фото передаётся в воркер без sync.Map (надёжно для ленты).
func (b *Bot) ProcessMiniAppPrivateTextWithTrainingPhoto(d initdata.InitData, text, publicPhotoURL string) MiniAppTextProcessResult {
	b.clearMiniappTrainingPhotoURL(d.User.ID)
	return b.processMiniAppPrivateCore(d, text, strings.TrimSpace(publicPhotoURL))
}

func (b *Bot) processMiniAppPrivateCore(d initdata.InitData, text string, trainingPhotoURL string) MiniAppTextProcessResult {
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

	// Отчёт о тренировке из мини-аппа: синхронный dispatch → reply_text сразу (без poll).
	if isMiniappTrainingReport(text) {
		msg := PrivateTextMessageFromInitUser(d, text)
		ch := make(chan string, 32)
		b.markMiniappOrigin(d.User.ID, ch)
		var syncReply string
		func() {
			defer b.unmarkMiniappOrigin(d.User.ID)
			defer func() {
				if r := recover(); r != nil {
					b.logger.Errorf("miniapp training sync dispatch panic: %v", r)
				}
			}()
			b.dispatchTextMessageFromUser(msg, ch, trainingPhotoURL)
			for {
				select {
				case t := <-ch:
					if tr := strings.TrimSpace(t); tr != "" {
						b.savePersonalChatMessage(d.User.ID, "leo", tr)
						if syncReply == "" {
							syncReply = tr
						} else {
							syncReply = syncReply + "\n\n" + tr
						}
					}
				default:
					return
				}
			}
		}()
		if strings.TrimSpace(syncReply) != "" {
			out.ReplyText = syncReply
			return out
		}
		b.logger.Warnf("miniapp training_done: empty reply after dispatch user=%d", d.User.ID)
		out.ReplyText = "✅ Отчёт принят. Если не видишь сводку с кубками и стриком — загляни в личку с ботом в Telegram."
		return out
	}

	go b.runMiniAppPrivateTextWorker(d, text, trainingPhotoURL)
	out.Pending = true
	return out
}

func (b *Bot) runMiniAppPrivateTextWorker(d initdata.InitData, text string, trainingPhotoURL string) {
	defer func() {
		if r := recover(); r != nil {
			b.logger.Errorf("miniapp private worker panic: %v", r)
		}
	}()
	msg := PrivateTextMessageFromInitUser(d, text)
	ch := make(chan string, 32)
	b.markMiniappOrigin(d.User.ID, ch)
	defer b.unmarkMiniappOrigin(d.User.ID)
	b.dispatchTextMessageFromUser(msg, ch, trainingPhotoURL)
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

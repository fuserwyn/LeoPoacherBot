package bot

import (
	"testing"

	"leo-bot/internal/config"
	"leo-bot/internal/logger"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Сообщения от имени канала или анонимного админа приходят без From.
// Раньше handleUpdate разыменовывал msg.From.ID и ронял весь процесс бота.
func TestHandleUpdateSkipsMessageWithoutSender(t *testing.T) {
	bot := &Bot{
		logger: logger.New("info"),
		config: &config.Config{OwnerID: 123},
	}

	update := tgbotapi.Update{
		Message: &tgbotapi.Message{
			Chat:       &tgbotapi.Chat{ID: -100500},
			SenderChat: &tgbotapi.Chat{ID: -100500},
			Text:       "/help",
		},
	}

	bot.handleUpdate(update)
}

// Паника в обработчике апдейта не должна убивать процесс.
func TestRecoverPanicSwallowsPanic(t *testing.T) {
	bot := &Bot{logger: logger.New("info")}

	func() {
		defer bot.recoverPanic("test")
		panic("boom")
	}()
}

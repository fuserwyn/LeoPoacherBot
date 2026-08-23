package bot

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"leo-bot/internal/database"
	"leo-bot/internal/domain"
	"leo-bot/internal/moderation"

	initdata "github.com/telegram-mini-apps/init-data-golang"
)

var (
	ErrPackFeedPollNotFound      = errors.New("pack feed poll not found")
	ErrPackFeedPollInvalidOption = errors.New("pack feed poll invalid option")
)

type adminFeedPollPayload struct {
	Question string   `json:"question"`
	Options  []string `json:"options"`
}

func marshalAdminFeedPollPayload(question string, options []string) (string, error) {
	payload := adminFeedPollPayload{
		Question: strings.TrimSpace(question),
		Options:  make([]string, 0, len(options)),
	}
	for _, opt := range options {
		v := strings.TrimSpace(opt)
		if v != "" {
			payload.Options = append(payload.Options, v)
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func parseAdminFeedPollPayload(raw string) (*adminFeedPollPayload, error) {
	var payload adminFeedPollPayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &payload); err != nil {
		return nil, err
	}
	payload.Question = strings.TrimSpace(payload.Question)
	clean := make([]string, 0, len(payload.Options))
	for _, opt := range payload.Options {
		v := strings.TrimSpace(opt)
		if v != "" {
			clean = append(clean, v)
		}
	}
	payload.Options = clean
	if payload.Question == "" || len(payload.Options) < 2 {
		return nil, errors.New("invalid admin feed poll payload")
	}
	return &payload, nil
}

func (b *Bot) saveAdminPollPackFeed(adminUserID int64, question string, options []string) error {
	if b == nil || b.db == nil || b.config == nil || b.config.MonetizedChatID == 0 {
		return fmt.Errorf("pack feed unavailable")
	}
	if err := b.enforceAdminBroadcast(strings.TrimSpace(question), moderation.SurfaceAdminPollQuestion); err != nil {
		return err
	}
	for _, opt := range options {
		if err := b.enforceAdminBroadcast(strings.TrimSpace(opt), moderation.SurfaceAdminPollOption); err != nil {
			return err
		}
	}
	payload, err := marshalAdminFeedPollPayload(question, options)
	if err != nil {
		return err
	}
	if strings.TrimSpace(payload) == "" {
		return fmt.Errorf("empty poll payload")
	}
	um := UserMessageForAdminPoll(payload, b.config.MonetizedChatID)
	if err := b.db.SaveUserMessage(&um); err != nil {
		return err
	}
	b.logger.Infof("admin miniapp poll published by admin=%d", adminUserID)
	return nil
}

// UserMessageForAdminPoll — маленький helper, чтобы не дублировать конструктор по проекту.
func UserMessageForAdminPoll(payload string, chatID int64) domain.UserMessage {
	return domain.UserMessage{
		UserID:      0,
		ChatID:      chatID,
		Username:    "Админ",
		MessageText: payload,
		MessageType: userMessageTypeAdminPoll,
	}
}

func (b *Bot) enrichPackFeedPolls(items []PackFeedItem, viewerUserID int64, chatID int64) []PackFeedItem {
	if b == nil || b.db == nil || len(items) == 0 {
		return items
	}
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		if item.Type == userMessageTypeAdminPoll {
			ids = append(ids, item.ID)
		}
	}
	if len(ids) == 0 {
		return items
	}
	summaries, err := b.db.ListMiniappFeedPollVoteSummaries(ids)
	if err != nil {
		b.logger.Warnf("pack feed poll vote summaries: %v", err)
		summaries = map[int64][]database.MiniappFeedPollVoteSummary{}
	}
	myVotes, err := b.db.ListMiniappFeedPollViewerVotes(ids, viewerUserID)
	if err != nil {
		b.logger.Warnf("pack feed poll viewer votes: %v", err)
		myVotes = map[int64]int{}
	}
	for i := range items {
		if items[i].Type != userMessageTypeAdminPoll {
			continue
		}
		payload, err := parseAdminFeedPollPayload(items[i].Text)
		if err != nil {
			b.logger.Warnf("pack feed poll parse id=%d: %v", items[i].ID, err)
			continue
		}
		poll := &PackFeedPoll{
			Question: payload.Question,
			Options:  make([]PackFeedPollOption, 0, len(payload.Options)),
		}
		countsByOption := map[int]int{}
		for _, row := range summaries[items[i].ID] {
			countsByOption[row.OptionIndex] = row.Count
			poll.TotalVotes += row.Count
		}
		for optionIndex, label := range payload.Options {
			poll.Options = append(poll.Options, PackFeedPollOption{
				Label: label,
				Votes: countsByOption[optionIndex],
			})
		}
		if idx, ok := myVotes[items[i].ID]; ok {
			v := idx
			poll.MyVoteIndex = &v
		}
		items[i].Text = payload.Question
		items[i].Poll = poll
	}
	return items
}

func (b *Bot) PackFeedPollVote(viewerUserID int64, initD initdata.InitData, userMessageID int64, optionIndex int) error {
	if err := b.PackFeedAssertViewerAccess(viewerUserID, initD); err != nil {
		return err
	}
	chatID := b.config.MonetizedChatID
	if chatID == 0 || userMessageID == 0 {
		return ErrPackFeedPollNotFound
	}
	msgType, ok, err := b.db.GetUserMessageTypeByIDForChat(userMessageID, chatID)
	if err != nil {
		return err
	}
	if !ok || msgType != userMessageTypeAdminPoll {
		return ErrPackFeedPollNotFound
	}
	text, err := b.db.GetUserMessageTextByIDForChat(userMessageID, chatID)
	if err != nil {
		return err
	}
	payload, err := parseAdminFeedPollPayload(text)
	if err != nil {
		return ErrPackFeedPollNotFound
	}
	if optionIndex < 0 || optionIndex >= len(payload.Options) {
		return ErrPackFeedPollInvalidOption
	}
	return b.db.UpsertMiniappFeedPollVote(userMessageID, viewerUserID, optionIndex)
}

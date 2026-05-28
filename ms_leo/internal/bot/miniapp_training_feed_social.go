package bot

import (
	"errors"
	"fmt"
	"strings"

	"leo-bot/internal/database"

	"leo-bot/internal/moderation"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	initdata "github.com/telegram-mini-apps/init-data-golang"
)

// Допустимые реакции по типам карточек в ленте мини-аппа (порядок — отображение).
var trainingFeedAllowedEmojis = []string{
	"🔥", "💪", "👏", "❤️", "🎉", "🦁", "⭐", "👍", "🙌", "✨", "🤝", "⚡", "🎯", "😤", "👀", "🙏", "😱",
	"🏆", "💯", "🥳", "🤩", "😮", "💦", "🤗", "👌", "🫶", "🧡", "💜", "🤙", "🫡", "🤘", "🏃", "🧘", "🤯", "💤",
}
var sickLeaveAllowedEmojis = []string{"😢", "😔", "🥺", "🤒", "🫂", "🙏", "❤️", "💙", "🌧️", "💤"}
var healthyAllowedEmojis = []string{"🎉", "🥳", "😄", "💚", "❤️", "👏", "🙌", "✨", "🌟", "💪"}

var (
	// ErrTrainingFeedSocialForbidden — нет доступа к ленте.
	ErrTrainingFeedSocialForbidden = errors.New("training feed social forbidden")
	// ErrTrainingFeedInvalidEmoji — эмодзи не из списка.
	ErrTrainingFeedInvalidEmoji = errors.New("invalid emoji")
	// ErrTrainingFeedParentNotFound — нет такого отчёта в стае.
	ErrTrainingFeedParentNotFound = errors.New("parent message not found")
	// ErrTrainingFeedThreadEmpty — пустой текст треда.
	ErrTrainingFeedThreadEmpty = errors.New("thread text empty")
	// ErrTrainingFeedThreadTooLong — слишком длинный комментарий.
	ErrTrainingFeedThreadTooLong = errors.New("thread text too long")
	// ErrTrainingFeedThreadDeleteNotFound — не найден комментарий или чужой / Лео.
	ErrTrainingFeedThreadDeleteNotFound = errors.New("training thread reply not found or forbidden")
	// ErrTrainingFeedThreadInvalidReply — reply_to не из этого отчёта / не найден.
	ErrTrainingFeedThreadInvalidReply = errors.New("training thread invalid reply reference")
)

func packFeedSupportsThread(messageType string) bool {
	switch messageType {
	case "training_done", "sick_leave", "healthy", userMessageTypeAdminPost, userMessageTypeAdminPoll:
		return true
	default:
		return false
	}
}

func allowedTrainingFeedEmoji(s string) (string, bool) {
	s = strings.TrimSpace(s)
	for _, e := range trainingFeedAllowedEmojis {
		if s == e {
			return e, true
		}
	}
	return "", false
}

func allowedEmojiForType(messageType, emoji string) (string, bool) {
	emoji = strings.TrimSpace(emoji)
	var allowed []string
	switch messageType {
	case "training_done":
		allowed = trainingFeedAllowedEmojis
	case "sick_leave":
		allowed = sickLeaveAllowedEmojis
	case "healthy", userMessageTypeAdminPost, userMessageTypeAdminPoll:
		allowed = healthyAllowedEmojis
	default:
		return "", false
	}
	for _, e := range allowed {
		if emoji == e {
			return e, true
		}
	}
	return "", false
}

func (b *Bot) assertPackFeedSocialViewer(viewerUserID int64) error {
	if b == nil {
		return ErrTrainingFeedSocialForbidden
	}
	chatID := b.config.MonetizedChatID
	if chatID == 0 {
		return ErrTrainingFeedSocialForbidden
	}
	if b.config.IsAdminTelegramUser(viewerUserID) {
		return nil
	}
	ok, err := b.db.UserInPackOrPaid(viewerUserID, chatID, b.config.PaywallEnabled)
	if err != nil {
		return err
	}
	if !ok {
		return ErrTrainingFeedSocialForbidden
	}
	return nil
}

// PackTrainingFeedReact — реакция на карточку ленты с соц. активностью (повтор с той же эмодзи снимает).
func (b *Bot) PackTrainingFeedReact(viewerUserID int64, initD initdata.InitData, userMessageID int64, emoji string) error {
	if err := b.AssertMiniAppPackChatAligns(initD); err != nil {
		return err
	}
	if err := b.assertPackFeedSocialViewer(viewerUserID); err != nil {
		return err
	}
	chatID := b.config.MonetizedChatID
	typ, has, err := b.db.GetUserMessageTypeByIDForChat(userMessageID, chatID)
	if err != nil {
		return err
	}
	if !has || !packFeedSupportsThread(typ) {
		return ErrTrainingFeedParentNotFound
	}
	em, ok := allowedEmojiForType(typ, emoji)
	if !ok {
		return ErrTrainingFeedInvalidEmoji
	}
	uname := displayNameFromInitData(initD)
	return b.db.SetTrainingFeedReaction(chatID, userMessageID, viewerUserID, uname, em)
}

// PackTrainingFeedThreadPost — комментарий в треде под training_done/sick_leave/healthy.
// replyToThreadID — id строки miniapp_training_feed_thread, на которую отвечаем (как Reply в Telegram).
func (b *Bot) PackTrainingFeedThreadPost(viewerUserID int64, initD initdata.InitData, userMessageID int64, text string, replyToThreadID int64) error {
	if err := b.AssertMiniAppPackChatAligns(initD); err != nil {
		return err
	}
	if err := b.assertPackFeedSocialViewer(viewerUserID); err != nil {
		return err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return ErrTrainingFeedThreadEmpty
	}
	if _, err := b.enforceUGC(text, moderation.SurfaceFeedComment, viewerUserID); err != nil {
		return err
	}
	chatID := b.config.MonetizedChatID
	typ, has, err := b.db.GetUserMessageTypeByIDForChat(userMessageID, chatID)
	if err != nil {
		return err
	}
	if !has || (typ != "training_done" && typ != "sick_leave" && typ != "healthy") {
		return ErrTrainingFeedParentNotFound
	}
	var leoParentSnapshot string
	replyingToLeo := false
	if replyToThreadID != 0 {
		parent, ok, err := b.db.GetTrainingFeedThreadRowInPack(replyToThreadID, chatID)
		if err != nil {
			return err
		}
		if !ok || parent.UserMessageID != userMessageID {
			return ErrTrainingFeedThreadInvalidReply
		}
		if parent.FromUserID == 0 {
			replyingToLeo = true
			leoParentSnapshot = parent.MessageText
		}
	}
	uname := displayNameFromInitData(initD)
	threadID, err := b.db.InsertTrainingFeedThreadReply(chatID, userMessageID, viewerUserID, uname, text, replyToThreadID)
	if err != nil {
		return err
	}
	b.afterPackTrainingThreadInserted(chatID, userMessageID, viewerUserID, uname, text, threadID, replyToThreadID, typ)
	if replyingToLeo && threadID != 0 && typ == "training_done" {
		snap := leoParentSnapshot
		txt := text
		uid := viewerUserID
		tid := threadID
		umid := userMessageID
		cid := chatID
		go func() {
			defer func() {
				if r := recover(); r != nil {
					b.logger.Errorf("leo banter training thread panic: %v", r)
				}
			}()
			b.LeoBanterReplyToUserTrainingFeedThread(cid, umid, tid, uid, txt, snap)
		}()
	}
	return nil
}

func truncateForDM(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= maxRunes {
		return string(r)
	}
	return string(r[:maxRunes]) + "…"
}

// Уведомление в личку + бейдж «Стая»: комментарий к отчёту или ответ на конкретное сообщение треда.
func (b *Bot) afterPackTrainingThreadInserted(packChatID, userMessageID, commenterUserID int64, commenterName, commentText string, threadReplyID, replyToParentThreadID int64, parentType string) {
	if b == nil || b.db == nil {
		return
	}
	var notifyUserID int64
	if replyToParentThreadID != 0 {
		parent, ok, err := b.db.GetTrainingFeedThreadRowInPack(replyToParentThreadID, packChatID)
		if err != nil {
			b.logger.Warnf("training thread parent lookup: %v", err)
			return
		}
		if !ok {
			return
		}
		if parent.FromUserID != 0 {
			if parent.FromUserID == commenterUserID {
				return
			}
			notifyUserID = parent.FromUserID
		} else {
			aid, ok2, err2 := b.db.GetUserMessageAuthorUserID(packChatID, userMessageID)
			if err2 != nil {
				b.logger.Warnf("training thread author lookup: %v", err2)
				return
			}
			if !ok2 || aid == 0 || aid == commenterUserID {
				return
			}
			notifyUserID = aid
		}
	} else {
		aid, ok, err := b.db.GetUserMessageAuthorUserID(packChatID, userMessageID)
		if err != nil {
			b.logger.Warnf("training thread author lookup: %v", err)
			return
		}
		if !ok || aid == 0 || aid == commenterUserID {
			return
		}
		notifyUserID = aid
	}
	if notifyUserID == 0 {
		return
	}
	if err := b.db.InsertTrainingThreadUnread(notifyUserID, packChatID, threadReplyID); err != nil {
		b.logger.Warnf("training thread unread insert: %v", err)
	}
	preview := truncateForDM(commentText, 160)
	cn := strings.TrimSpace(commenterName)
	if cn == "" {
		cn = "Участник стаи"
	}
	commenterGender, _, _ := b.GetMiniappUserProfileJSONForAPI(commenterUserID, packChatID)
	commenterGender = strings.TrimSpace(strings.ToLower(commenterGender))
	var body string
	if replyToParentThreadID != 0 {
		verb := ""
		switch commenterGender {
		case "m":
			verb = "ответил"
		case "f":
			verb = "ответила"
		}
		if verb == "" {
			body = "↩️ Ответ от " + cn + " на твой комментарий в стае.\n\n«" + preview + "»\n\nОткрой мини-апп → вкладка «Стая»."
		} else {
			body = "↩️ " + cn + " " + verb + " на твой комментарий в стае.\n\n«" + preview + "»\n\nОткрой мини-апп → вкладка «Стая»."
		}
	} else {
		what := "твою тренировку"
		if parentType == "sick_leave" || parentType == "healthy" {
			what = "твой статус по больничному"
		} else if parentType == userMessageTypeAdminPost {
			what = "пост админа"
		} else if parentType == userMessageTypeAdminPoll {
			what = "опрос админа"
		}
		verb := ""
		switch commenterGender {
		case "m":
			verb = "прокомментировал"
		case "f":
			verb = "прокомментировала"
		}
		if verb == "" {
			body = "💬 Комментарий от " + cn + " к " + what + " в стае.\n\n«" + preview + "»\n\nОткрой мини-апп → вкладка «Стая»."
		} else {
			body = "💬 " + cn + " " + verb + " " + what + " в стае.\n\n«" + preview + "»\n\nОткрой мини-апп → вкладка «Стая»."
		}
	}
	b.sendTrainingThreadCommentDM(notifyUserID, body)
}

// Всегда в Telegram-личку, без очереди мини-аппа (уведомления-алерт).
func (b *Bot) sendTrainingThreadCommentDM(telegramUserID int64, text string) {
	if b == nil || b.api == nil || telegramUserID == 0 || strings.TrimSpace(text) == "" {
		return
	}
	m := tgbotapi.NewMessage(telegramUserID, text)
	if _, err := b.api.Send(m); err != nil {
		b.logger.Warnf("training thread comment DM user=%d: %v", telegramUserID, err)
	}
}

// MiniappTrainingThreadUnreadCount — для бейджа на вкладке «Стая».
func (b *Bot) MiniappTrainingThreadUnreadCount(initD initdata.InitData, viewerUserID int64) (int64, error) {
	n, _, err := b.MiniappTrainingThreadUnreadSummary(initD, viewerUserID)
	return n, err
}

// MiniappTrainingThreadUnreadSummary — счётчик и id отчётов с непрочитанными комментариями.
func (b *Bot) MiniappTrainingThreadUnreadSummary(initD initdata.InitData, viewerUserID int64) (count int64, userMessageIDs []int64, err error) {
	if err := b.AssertMiniAppPackChatAligns(initD); err != nil {
		return 0, nil, err
	}
	chatID := b.config.MonetizedChatID
	if chatID == 0 || b.db == nil {
		return 0, nil, nil
	}
	count, err = b.db.CountTrainingThreadUnread(viewerUserID, chatID)
	if err != nil {
		return 0, nil, err
	}
	userMessageIDs, err = b.db.ListTrainingThreadUnreadUserMessageIDs(viewerUserID, chatID)
	if err != nil {
		return 0, nil, err
	}
	return count, userMessageIDs, nil
}

// markTrainingThreadReplyUnread — бейдж «Стая» + опциональная личка (системный текст, не LLM).
func (b *Bot) markTrainingThreadReplyUnread(packChatID, notifyUserID, threadReplyID int64, dmBody string) {
	if b == nil || b.db == nil || notifyUserID == 0 || threadReplyID == 0 {
		return
	}
	if err := b.db.InsertTrainingThreadUnread(notifyUserID, packChatID, threadReplyID); err != nil {
		b.logger.Warnf("training thread unread insert: %v", err)
	}
	if strings.TrimSpace(dmBody) != "" {
		b.sendTrainingThreadCommentDM(notifyUserID, dmBody)
	}
}

// MiniappTrainingThreadUnreadClear — сброс бейджа; userMessageID>0 — только под одним отчётом.
func (b *Bot) MiniappTrainingThreadUnreadClear(initD initdata.InitData, viewerUserID int64, userMessageID int64) error {
	if err := b.AssertMiniAppPackChatAligns(initD); err != nil {
		return err
	}
	chatID := b.config.MonetizedChatID
	if chatID == 0 || b.db == nil {
		return nil
	}
	if userMessageID > 0 {
		return b.db.ClearTrainingThreadUnreadForUserMessage(viewerUserID, chatID, userMessageID)
	}
	return b.db.ClearTrainingThreadUnread(viewerUserID, chatID)
}

// PackTrainingFeedThreadDelete — удалить свою реплику в треде (ответы Лео не удаляются).
func (b *Bot) PackTrainingFeedThreadDelete(viewerUserID int64, initD initdata.InitData, threadReplyID int64) (parentUserMessageID int64, err error) {
	if err := b.AssertMiniAppPackChatAligns(initD); err != nil {
		return 0, err
	}
	if err := b.assertPackFeedSocialViewer(viewerUserID); err != nil {
		return 0, err
	}
	if threadReplyID == 0 {
		return 0, ErrTrainingFeedThreadDeleteNotFound
	}
	chatID := b.config.MonetizedChatID
	parentID, deleted, err := b.db.DeleteTrainingFeedThreadReply(chatID, threadReplyID, viewerUserID)
	if err != nil {
		return 0, err
	}
	if !deleted {
		return 0, ErrTrainingFeedThreadDeleteNotFound
	}
	if err := b.db.DeleteTrainingThreadUnreadByReplyID(threadReplyID); err != nil {
		b.logger.Warnf("training thread unread delete: %v", err)
	}
	return parentID, nil
}

// PackTrainingFeedThreadLikeToggle — лайк комментария в треде training_done.
func (b *Bot) PackTrainingFeedThreadLikeToggle(viewerUserID int64, initD initdata.InitData, threadReplyID int64) (parentUserMessageID int64, err error) {
	if err := b.AssertMiniAppPackChatAligns(initD); err != nil {
		return 0, err
	}
	if err := b.assertPackFeedSocialViewer(viewerUserID); err != nil {
		return 0, err
	}
	if threadReplyID == 0 {
		return 0, ErrTrainingFeedThreadDeleteNotFound
	}
	chatID := b.config.MonetizedChatID
	row, ok, err := b.db.GetTrainingFeedThreadRowInPack(threadReplyID, chatID)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrTrainingFeedThreadDeleteNotFound
	}
	if err := b.db.ToggleTrainingFeedThreadLike(chatID, threadReplyID, viewerUserID); err != nil {
		return 0, err
	}
	return row.UserMessageID, nil
}

func (b *Bot) threadRowsToPackReplies(rows []database.TrainingFeedThreadRow, viewerUserID, packChatID int64) []PackFeedThreadReply {
	out := make([]PackFeedThreadReply, 0, len(rows))
	if len(rows) == 0 {
		return out
	}
	var refIDs []int64
	seen := map[int64]struct{}{}
	for _, t := range rows {
		if t.ReplyToID.Valid && t.ReplyToID.Int64 != 0 {
			rid := t.ReplyToID.Int64
			if _, ok := seen[rid]; !ok {
				seen[rid] = struct{}{}
				refIDs = append(refIDs, rid)
			}
		}
	}
	parentByID := map[int64]database.TrainingFeedThreadRow{}
	if len(refIDs) > 0 && b.db != nil {
		m, err := b.db.ListTrainingFeedThreadRowsByIDs(packChatID, refIDs)
		if err != nil {
			b.logger.Warnf("training thread reply parents: %v", err)
		} else {
			parentByID = m
		}
	}
	likeMap := map[int64]database.ThreadLikeAgg{}
	if len(rows) > 0 && b.db != nil {
		ids := make([]int64, 0, len(rows))
		for _, t := range rows {
			ids = append(ids, t.ID)
		}
		m, err := b.db.ListTrainingFeedThreadLikeAggs(packChatID, ids, viewerUserID)
		if err != nil {
			b.logger.Warnf("training thread likes list: %v", err)
		} else {
			likeMap = m
		}
	}
	for _, t := range rows {
		pr := PackFeedThreadReply{
			ID:        t.ID,
			UserID:    t.FromUserID,
			Username:  t.Username,
			Text:      t.MessageText,
			CreatedAt: t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			IsYou:     t.FromUserID != 0 && t.FromUserID == viewerUserID,
			IsLeo:     t.FromUserID == 0,
		}
		if t.ReplyToID.Valid && t.ReplyToID.Int64 != 0 {
			rid := t.ReplyToID.Int64
			pr.ReplyToID = rid
			if p, ok := parentByID[rid]; ok {
				pr.ReplyToIsLeo = p.FromUserID == 0
				if pr.ReplyToIsLeo {
					pr.ReplyToUsername = "Лео"
				} else {
					pr.ReplyToUsername = strings.TrimSpace(p.Username)
					if pr.ReplyToUsername == "" {
						pr.ReplyToUsername = fmt.Sprintf("Участник %d", p.FromUserID)
					}
				}
				pr.ReplyToText = truncateForDM(p.MessageText, 100)
			}
		}
		if l, ok := likeMap[t.ID]; ok {
			pr.LikeCount = l.Count
			pr.LikeMe = l.Me
		}
		out = append(out, pr)
	}
	return out
}

// PackFeedThreadRepliesForViewer — полный тред под одним отчётом (после POST комментария и для согласованности с лентой).
func (b *Bot) PackFeedThreadRepliesForViewer(viewerUserID, userMessageID int64) ([]PackFeedThreadReply, error) {
	if b == nil || b.db == nil {
		return nil, fmt.Errorf("bot unavailable")
	}
	m, err := b.db.ListTrainingFeedThreadByMessages([]int64{userMessageID})
	if err != nil {
		return nil, err
	}
	chatID := b.config.MonetizedChatID
	return b.threadRowsToPackReplies(m[userMessageID], viewerUserID, chatID), nil
}

// enrichPackFeedTrainingSocial — реакции и треды для карточек ленты с соц. активностью.
func (b *Bot) enrichPackFeedTrainingSocial(items []PackFeedItem, viewerUserID int64, chatID int64) []PackFeedItem {
	socialIDs := make([]int64, 0)
	reactionIDs := make([]int64, 0)
	for _, it := range items {
		if packFeedSupportsThread(it.Type) {
			socialIDs = append(socialIDs, it.ID)
		}
		if it.Type == "training_done" || it.Type == "sick_leave" || it.Type == "healthy" || it.Type == userMessageTypeAdminPost || it.Type == userMessageTypeAdminPoll {
			reactionIDs = append(reactionIDs, it.ID)
		}
	}
	if len(socialIDs) == 0 {
		return items
	}
	aggsMap := map[int64][]database.TrainingFeedReactionAgg{}
	meMap := map[int64]string{}
	if len(reactionIDs) > 0 {
		var err error
		aggsMap, meMap, err = b.db.ListTrainingFeedReactionAggs(chatID, reactionIDs, viewerUserID)
		if err != nil {
			b.logger.Warnf("pack feed reaction aggs: %v", err)
			return items
		}
	}
	threadMap, err := b.db.ListTrainingFeedThreadByMessages(socialIDs)
	if err != nil {
		b.logger.Warnf("pack feed thread list: %v", err)
		return items
	}
	for i := range items {
		if !packFeedSupportsThread(items[i].Type) {
			continue
		}
		id := items[i].ID
		meEmoji := meMap[id]
		if aggs, ok := aggsMap[id]; ok {
			allowed := trainingFeedAllowedEmojis
			if items[i].Type == "sick_leave" {
				allowed = sickLeaveAllowedEmojis
			} else if items[i].Type == "healthy" || items[i].Type == userMessageTypeAdminPost || items[i].Type == userMessageTypeAdminPoll {
				allowed = healthyAllowedEmojis
			}
			for _, a := range database.SortReactionAggsForDisplay(aggs, allowed) {
				items[i].Reactions = append(items[i].Reactions, PackFeedReaction{
					Emoji:  a.Emoji,
					Count:  a.Count,
					Me:     meEmoji == a.Emoji,
					Voters: a.Voters,
				})
			}
		}
		if thr, ok := threadMap[id]; ok {
			items[i].Thread = b.threadRowsToPackReplies(thr, viewerUserID, chatID)
		}
	}
	return items
}

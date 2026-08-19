package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"leo-bot/internal/database"

	initdata "github.com/telegram-mini-apps/init-data-golang"
)

// Разделы админки, которые раньше жили только в чате (/admin): аналитика,
// посещения, оплаты, админы, отложенные посты, опросы и очистка ленты.
// Цифры и подписи считаются здесь же теми же запросами, что и для чата, —
// мини-апп только рисует, поэтому две админки не разъезжаются.

// MiniappAdminTable — таблица «как в чате»: заголовок, пояснение и строки.
type MiniappAdminTable struct {
	Title    string     `json:"title"`
	Subtitle string     `json:"subtitle"`
	Columns  []string   `json:"columns"`
	Rows     [][]string `json:"rows"`
}

type MiniappAdminAnalytics struct {
	Period      string              `json:"period"`
	LastEventAt string              `json:"last_event_at"`
	Tables      []MiniappAdminTable `json:"tables"`
}

// MiniappAdminAnalyticsData — воронки, KPI, каналы и события за период.
func (b *Bot) MiniappAdminAnalyticsData(
	viewerUserID int64, initD initdata.InitData, days int,
) (MiniappAdminAnalytics, error) {
	var out MiniappAdminAnalytics
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return out, err
	}
	if days <= 0 {
		days = 30
	}
	counts, err := b.db.EventUniqueCounts(days)
	if err != nil {
		return out, err
	}
	out.Period = analyticsPeriodLabel(days)
	if last, ok := b.db.AnalyticsLastEventAt(); ok {
		out.LastEventAt = last.In(time.FixedZone("MSK", 3*3600)).Format("02.01 15:04")
	}

	funnel := func(title, subtitle string, stages [][2]string) MiniappAdminTable {
		tbl := MiniappAdminTable{Title: title, Subtitle: subtitle, Columns: []string{"Стадия", "Юзеры", "Конв"}}
		var prev int64 = -1
		for _, s := range stages {
			n := counts[s[0]]
			conv := "—"
			if prev >= 0 {
				conv = analyticsPct(n, prev)
			}
			tbl.Rows = append(tbl.Rows, []string{s[1], strconv.FormatInt(n, 10), conv})
			prev = n
		}
		return tbl
	}

	started := counts[database.EventBotStarted]
	paywall := counts[database.EventPaywallViewed]
	paid := counts[database.EventPaymentCompleted]
	miniapp := counts[database.EventMiniappOpened]
	logged := counts[database.EventWorkoutLogged]

	out.Tables = append(out.Tables, funnel(
		"1️⃣ Воронка: бот → оплата",
		fmt.Sprintf("⭐ пэйвол→оплата: %s · старт→оплата: %s", analyticsPct(paid, paywall), analyticsPct(paid, started)),
		[][2]string{
			{database.EventBotStarted, "Старт бота"},
			{database.EventPaywallViewed, "Пэйвол"},
			{database.EventPaymentMethodSelected, "Выбор способа"},
			{database.EventPaymentInitiated, "К оплате"},
			{database.EventPaymentCompleted, "Оплатил"},
			{database.EventMiniappOpened, "Открыл миниапп"},
		},
	))

	out.Tables = append(out.Tables, funnel(
		"2️⃣ Воронка: активация",
		fmt.Sprintf("⭐ kill-метрика (первая трен./оплата): %s · таргет ≥35%%, kill <20%%", analyticsPct(logged, paid)),
		[][2]string{
			{database.EventMiniappOpened, "Открыл миниапп"},
			{database.EventWorkoutLogStarted, "Открыл форму"},
			{database.EventWorkoutLogged, "Залогал трен."},
			{database.EventLeoCommentReceived, "Коммент Лео"},
		},
	))

	retention := MiniappAdminTable{
		Title: "3️⃣ Retention и серии",
		Subtitle: fmt.Sprintf("Burn recovery: %s · Реактивация: %s",
			analyticsPct(counts[database.EventBurnRecovered], counts[database.EventBurnWarningSent]),
			analyticsPct(counts[database.EventAccountReactivated], counts[database.EventAccountDeletedInactivity])),
		Columns: []string{"Событие", "Юзеры"},
	}
	for _, r := range [][2]string{
		{database.EventStreakIncremented, "Стрик растёт"},
		{database.EventStreakAttemptUsed, "Попытка спасла"},
		{database.EventStreakBroken, "Стрик сгорел"},
		{database.EventLevelUp, "Новый уровень"},
		{database.EventMilestoneAchieved, "Ачивка стрика"},
		{database.EventBurnWarningSent, "Burn-алерт"},
		{database.EventBurnRecovered, "Спасся после"},
		{database.EventSickLeaveStarted, "Больничный вкл"},
		{database.EventSickLeaveEnded, "Больничный выкл"},
		{database.EventAccountDeletedInactivity, "Удалён (8д)"},
		{database.EventAccountReactivated, "Вернулся"},
	} {
		retention.Rows = append(retention.Rows, []string{r[1], strconv.FormatInt(counts[r[0]], 10)})
	}
	out.Tables = append(out.Tables, retention)

	kpi := MiniappAdminTable{
		Title:    "⭐ Сводка KPI",
		Subtitle: "По уникальным юзерам. NSM, D7/D30 и Sean Ellis считаются когортно, не из events.",
		Columns:  []string{"Метрика", "Знач", "Цель", "Kill"},
		Rows: [][]string{
			{"Активация (трен/опл)", analyticsPct(logged, paid), ">35%", "<20%"},
			{"Пэйвол→оплата", analyticsPct(paid, paywall), "—", "—"},
			{"Старт→пэйвол", analyticsPct(paywall, started), "—", "—"},
			{"Оплата→миниапп", analyticsPct(miniapp, paid), "—", "—"},
			{"Burn recovery", analyticsPct(counts[database.EventBurnRecovered], counts[database.EventBurnWarningSent]), "—", "—"},
			{"Реактивация", analyticsPct(counts[database.EventAccountReactivated], counts[database.EventAccountDeletedInactivity]), "—", "—"},
		},
	}
	out.Tables = append(out.Tables, kpi)

	if channels, err := b.db.GetChannelAttribution(days); err == nil && len(channels) > 0 {
		tbl := MiniappAdminTable{Title: "📣 Каналы", Subtitle: "Откуда пришли и сколько оплатили", Columns: []string{"Канал", "Старты", "Оплат"}}
		for _, c := range channels {
			tbl.Rows = append(tbl.Rows, []string{c.Source, strconv.FormatInt(c.Started, 10), strconv.FormatInt(c.Paid, 10)})
		}
		out.Tables = append(out.Tables, tbl)
	}

	if overview, err := b.db.GetEventOverview(days); err == nil && len(overview) > 0 {
		tbl := MiniappAdminTable{Title: "📋 События", Subtitle: "Все события периода", Columns: []string{"Событие", "Всего", "Юзеры"}}
		for _, e := range overview {
			tbl.Rows = append(tbl.Rows, []string{e.Name, strconv.FormatInt(e.Total, 10), strconv.FormatInt(e.UniqueUsers, 10)})
		}
		out.Tables = append(out.Tables, tbl)
	}

	return out, nil
}

// MiniappAdminVisits — «Посещения бота»: сводка и топ визитёров.
func (b *Bot) MiniappAdminVisits(viewerUserID int64, initD initdata.InitData) ([]MiniappAdminTable, error) {
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return nil, err
	}
	stats, err := b.db.GetBotVisitStats()
	if err != nil {
		return nil, err
	}
	summary := MiniappAdminTable{
		Title:   "📊 Посещения бота",
		Columns: []string{"Метрика", "Значение"},
		Rows: [][]string{
			{"Всего визитов", strconv.FormatInt(stats.TotalVisits, 10)},
			{"Уникальных", strconv.FormatInt(stats.UniqueUsers, 10)},
			{"Сегодня", strconv.FormatInt(stats.TodayVisits, 10)},
			{"За неделю", strconv.FormatInt(stats.WeekVisits, 10)},
			{"За месяц", strconv.FormatInt(stats.MonthVisits, 10)},
		},
	}
	top := MiniappAdminTable{Title: "🏅 Кто заходит чаще", Columns: []string{"Юзер", "Визитов", "Последний"}}
	for _, u := range stats.TopVisitors {
		top.Rows = append(top.Rows, []string{
			adminVisitDisplayName(u.Username, u.FirstName, u.UserID),
			strconv.FormatInt(u.Visits, 10),
			u.LastVisit.In(time.FixedZone("MSK", 3*3600)).Format("02.01 15:04"),
		})
	}
	if len(top.Rows) == 0 {
		return []MiniappAdminTable{summary}, nil
	}
	return []MiniappAdminTable{summary, top}, nil
}

type MiniappAdminPayments struct {
	Total  int               `json:"total"`
	Offset int               `json:"offset"`
	Limit  int               `json:"limit"`
	Table  MiniappAdminTable `json:"table"`
}

// MiniappAdminPaymentsPage — «Оплаты» постранично, как admin_payments_<offset>.
func (b *Bot) MiniappAdminPaymentsPage(
	viewerUserID int64, initD initdata.InitData, offset, limit int,
) (MiniappAdminPayments, error) {
	var out MiniappAdminPayments
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return out, err
	}
	packChatID := b.adminPackChatID()
	if packChatID == 0 {
		return out, fmt.Errorf("не настроен MonetizedChatID")
	}
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	total, err := b.db.CountPaywallPaymentsForAdmin(packChatID)
	if err != nil {
		return out, err
	}
	rows, err := b.db.ListPaywallPaymentsForAdmin(packChatID, offset, limit)
	if err != nil {
		return out, err
	}
	out.Total, out.Offset, out.Limit = total, offset, limit
	out.Table = MiniappAdminTable{
		Title:   "💳 Оплаты",
		Columns: []string{"№", "Ник", "Статус", "Сумма", "Дата"},
	}
	for i, p := range rows {
		out.Table.Rows = append(out.Table.Rows, []string{
			strconv.Itoa(offset + i + 1),
			adminPaywallPersonLabel(p.Username, p.DisplayName, p.UserID),
			adminPaymentStatusShort(p.Status, p.AccessActive),
			adminFormatPaymentAmount(p.AmountMinor, p.Currency),
			p.CreatedAt.In(time.FixedZone("MSK", 3*3600)).Format("02.01 15:04"),
		})
	}
	return out, nil
}

type MiniappAdminPerson struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Static   bool   `json:"static"`
	AddedAt  string `json:"added_at"`
}

// MiniappAdminAdminsList — кто админ: из переменных окружения и из базы.
func (b *Bot) MiniappAdminAdminsList(viewerUserID int64, initD initdata.InitData) ([]MiniappAdminPerson, error) {
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return nil, err
	}
	out := make([]MiniappAdminPerson, 0, 8)
	if b.config.OwnerID != 0 {
		out = append(out, MiniappAdminPerson{UserID: b.config.OwnerID, Username: "владелец", Static: true})
	}
	for _, id := range b.config.AdminIDs {
		out = append(out, MiniappAdminPerson{UserID: id, Static: true})
	}
	dynamic, err := b.db.ListDynamicAdmins()
	if err != nil {
		return nil, err
	}
	for _, a := range dynamic {
		out = append(out, MiniappAdminPerson{
			UserID:   a.UserID,
			Username: a.Username,
			AddedAt:  a.AddedAt.In(time.FixedZone("MSK", 3*3600)).Format("02.01.2006"),
		})
	}
	return out, nil
}

// MiniappAdminAddAdmin — выдать права по @нику или id. Возвращает id новичка.
func (b *Bot) MiniappAdminAddAdmin(viewerUserID int64, initD initdata.InitData, query string) (int64, error) {
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return 0, err
	}
	q := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(query), "@"))
	if q == "" {
		return 0, ErrAdminActionInvalid
	}
	targetID, err := strconv.ParseInt(q, 10, 64)
	username := ""
	if err != nil {
		targetID, err = b.db.FindUserIDByUsername(q)
		if err != nil || targetID == 0 {
			return 0, fmt.Errorf("не нашёл @%s среди участников", q)
		}
		username = q
	}
	if err := b.db.AddDynamicAdmin(targetID, username, viewerUserID); err != nil {
		return 0, err
	}
	// Права проверяются по кэшу в памяти, а не по базе: без перезагрузки
	// добавленный из мини-аппа админ оставался обычным человеком до рестарта
	// сервиса — со стороны это выглядело как «добавление не работает».
	b.reloadDynamicAdmins()
	// Нижняя клавиатура в личке зависит от прав — как и при добавлении из чата,
	// сбрасываем её кэш, иначе кнопки админ-панели не появятся.
	b.privateBottomKeyboardKind.Delete(targetID)
	return targetID, nil
}

// MiniappAdminRemoveAdmin — снять права. Права из окружения так не снимаются.
func (b *Bot) MiniappAdminRemoveAdmin(viewerUserID int64, initD initdata.InitData, targetID int64) error {
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return err
	}
	if targetID == b.config.OwnerID {
		return fmt.Errorf("владельца снять нельзя")
	}
	isDynamic, err := b.db.IsDynamicAdmin(targetID)
	if err != nil {
		return err
	}
	if !isDynamic {
		return fmt.Errorf("этот админ прописан в переменных окружения — снимается только там")
	}
	if _, err := b.db.RemoveDynamicAdmin(targetID); err != nil {
		return err
	}
	// Снятие прав тоже должно действовать сразу, а не после рестарта.
	b.reloadDynamicAdmins()
	b.privateBottomKeyboardKind.Delete(targetID)
	return nil
}

type MiniappScheduledPost struct {
	ID          int64  `json:"id"`
	Author      string `json:"author"`
	Text        string `json:"text"`
	ScheduledAt string `json:"scheduled_at"`
}

// MiniappAdminScheduledPosts — очередь отложенных постов в ленту.
func (b *Bot) MiniappAdminScheduledPosts(viewerUserID int64, initD initdata.InitData) ([]MiniappScheduledPost, error) {
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return nil, err
	}
	posts, err := b.db.ListPendingScheduledAdminPosts(b.config.MonetizedChatID, 20)
	if err != nil {
		return nil, err
	}
	out := make([]MiniappScheduledPost, 0, len(posts))
	for _, p := range posts {
		out = append(out, MiniappScheduledPost{
			ID:          p.ID,
			Author:      p.Author,
			Text:        p.MessageText,
			ScheduledAt: p.ScheduledAt.In(time.FixedZone("MSK", 3*3600)).Format("02.01 15:04"),
		})
	}
	return out, nil
}

// MiniappAdminSchedulePost — поставить пост в ленту на время (МСК).
func (b *Bot) MiniappAdminSchedulePost(
	viewerUserID int64, initD initdata.InitData, author, text, at string,
) (int64, error) {
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return 0, err
	}
	body := strings.TrimSpace(text)
	if body == "" {
		return 0, ErrAdminActionInvalid
	}
	msk := time.FixedZone("MSK", 3*3600)
	when, err := time.ParseInLocation("2006-01-02T15:04", strings.TrimSpace(at), msk)
	if err != nil {
		return 0, fmt.Errorf("не понял время: нужен формат 2026-08-20T09:00")
	}
	if when.Before(time.Now()) {
		return 0, fmt.Errorf("время уже прошло")
	}
	return b.db.InsertScheduledAdminPost(
		b.config.MonetizedChatID, normalizeAdminPostAuthor(author), body, when, viewerUserID,
	)
}

// MiniappAdminCancelScheduledPost — снять пост из очереди.
func (b *Bot) MiniappAdminCancelScheduledPost(viewerUserID int64, initD initdata.InitData, postID int64) error {
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return err
	}
	ok, err := b.db.CancelScheduledAdminPost(b.config.MonetizedChatID, postID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrAdminNotFound
	}
	return nil
}

// MiniappAdminPublishPoll — опрос в ленту: вопрос и 2–10 вариантов.
func (b *Bot) MiniappAdminPublishPoll(
	viewerUserID int64, initD initdata.InitData, question string, options []string,
) error {
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return err
	}
	q := strings.TrimSpace(question)
	if q == "" {
		return fmt.Errorf("напиши вопрос")
	}
	clean := make([]string, 0, len(options))
	for _, opt := range options {
		v := strings.TrimSpace(opt)
		if v == "" {
			continue
		}
		if len([]rune(v)) > 100 {
			return fmt.Errorf("вариант «%s…» длиннее 100 символов", string([]rune(v)[:20]))
		}
		clean = append(clean, v)
	}
	if len(clean) < 2 {
		return fmt.Errorf("нужно минимум 2 варианта")
	}
	if len(clean) > 10 {
		return fmt.Errorf("максимум 10 вариантов")
	}
	return b.saveAdminPollPackFeed(viewerUserID, q, clean)
}

type MiniappWipeCounts struct {
	FeedPosts        int64 `json:"feed_posts"`
	FeedThreads      int64 `json:"feed_threads"`
	FeedReports      int64 `json:"feed_reports"`
	PackChatMessages int64 `json:"pack_chat_messages"`
}

// MiniappAdminWipeCounts — что именно удалится при очистке ленты и чата.
func (b *Bot) MiniappAdminWipeCounts(viewerUserID int64, initD initdata.InitData) (MiniappWipeCounts, error) {
	var out MiniappWipeCounts
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return out, err
	}
	packChatID := b.adminPackChatID()
	if packChatID == 0 {
		return out, fmt.Errorf("не настроен MonetizedChatID")
	}
	counts, err := b.db.AdminCountPackFeedAndChat(packChatID)
	if err != nil {
		return out, err
	}
	return MiniappWipeCounts{
		FeedPosts:        counts.FeedPosts,
		FeedThreads:      counts.FeedThreads,
		FeedReports:      counts.FeedReports,
		PackChatMessages: counts.PackChatMessages,
	}, nil
}

// MiniappAdminWipeExecute — необратимая очистка ленты и чата стаи.
func (b *Bot) MiniappAdminWipeExecute(viewerUserID int64, initD initdata.InitData) (MiniappWipeCounts, error) {
	var out MiniappWipeCounts
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return out, err
	}
	packChatID := b.adminPackChatID()
	if packChatID == 0 {
		return out, fmt.Errorf("не настроен MonetizedChatID")
	}
	deleted, err := b.db.AdminClearPackFeedAndChat(packChatID)
	if err != nil {
		return out, err
	}
	return MiniappWipeCounts{
		FeedPosts:        deleted.FeedPosts,
		FeedThreads:      deleted.FeedThreads,
		FeedReports:      deleted.FeedReports,
		PackChatMessages: deleted.PackChatMessages,
	}, nil
}

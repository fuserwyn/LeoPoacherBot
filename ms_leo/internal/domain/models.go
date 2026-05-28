package domain

import (
	"time"
)

// MessageLog представляет запись о сообщении пользователя
type MessageLog struct {
	UserID                   int64      `json:"user_id" db:"user_id"`
	ChatID                   int64      `json:"chat_id" db:"chat_id"`
	Username                 string     `json:"username" db:"username"`
	StreakDays               int        `json:"streak_days" db:"streak_days"`
	MaxStreakDays            int        `json:"max_streak_days" db:"max_streak_days"`
	CupsEarned               int        `json:"cups_earned" db:"cups_earned"`
	LastTrainingDate         *string    `json:"last_training_date" db:"last_training_date"`
	LastMessage              string     `json:"last_message" db:"last_message"`
	HasTrainingDone          bool       `json:"has_training_done" db:"has_training_done"`
	HasSickLeave             bool       `json:"has_sick_leave" db:"has_sick_leave"`
	HasHealthy               bool       `json:"has_healthy" db:"has_healthy"`
	IsDeleted                bool       `json:"is_deleted" db:"is_deleted"`
	IsExemptFromDeletion     bool       `json:"is_exempt_from_deletion" db:"is_exempt_from_deletion"`
	TimerStartTime           *string    `json:"timer_start_time" db:"timer_start_time"`
	SickLeaveStartTime       *string    `json:"sick_leave_start_time" db:"sick_leave_start_time"`
	SickLeaveEndTime         *string    `json:"sick_leave_end_time" db:"sick_leave_end_time"`
	SickTime                 *string    `json:"sick_time" db:"sick_time"`
	Gender                   string     `json:"gender" db:"gender"`                                           // "m" (male), "f" (female), or empty
	TimezoneOffsetFromMoscow int        `json:"timezone_offset_from_moscow" db:"timezone_offset_from_moscow"` // смещение пользователя относительно МСК (часы)
	SickApprovalPending      bool       `json:"sick_approval_pending" db:"sick_approval_pending"`
	SickApprovalDeadline     *time.Time `json:"sick_approval_deadline" db:"sick_approval_deadline"`
	SickApprovalMessageID    *int64     `json:"sick_approval_message_id" db:"sick_approval_message_id"`
	// Leopard Money Model: achievement_count 0–7; заморозка дневного ±XP.
	AchievementCount           int        `json:"achievement_count" db:"achievement_count"`
	XpFreezeUntil              *time.Time `json:"xp_freeze_until" db:"xp_freeze_until"`
	LastDailyXPMskDate         *string    `json:"last_daily_xp_msk_date" db:"last_daily_xp_msk_date"` // YYYY-MM-DD — обработан последний «вчера»
	LeopardStarterBonusApplied bool       `json:"leopard_starter_bonus_applied" db:"leopard_starter_bonus_applied"`
	LastAchievementStreakLevel int        `json:"last_achievement_streak_level" db:"last_achievement_streak_level"` // 0,7,14,21,30,42,50,100 — последний уровень, за который выдали ачивку
	CreatedAt                  time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt                  time.Time  `json:"updated_at" db:"updated_at"`
}

// ChatMember представляет участника чата
type ChatMember struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Status   string `json:"status"`
}

// TimerInfo — цепочка неактивности: предупреждения день 5 (72 ч), день 6 (48 ч), день 7 (24 ч) до дедлайна в локальном TZ юзера, затем удаление.
type TimerInfo struct {
	UserID         int64
	ChatID         int64
	Username       string
	Warning72hTask chan bool // день 5: первое предупреждение
	Warning48hTask chan bool // день 6: предупреждение + 2 опции спасения в мини-аппе
	Warning24hTask chan bool // день 7: финальное предупреждение
	RemovalTask    chan bool
	TimerStartTime string
}

// UserMessage представляет сообщение пользователя для RAG контекста
type UserMessage struct {
	ID               int64     `json:"id" db:"id"`
	UserID           int64     `json:"user_id" db:"user_id"`
	ChatID           int64     `json:"chat_id" db:"chat_id"`
	Username         string    `json:"username" db:"username"`
	MessageText      string    `json:"message_text" db:"message_text"`
	MessageType      string    `json:"message_type" db:"message_type"` // general, training_done, sick_leave, healthy
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	TrainingPhotoURL string    `json:"training_photo_url,omitempty" db:"training_photo_url"`
}

// PackActivityRow — строка ленты мини-апpa (отчёты в чате стаи).
type PackActivityRow struct {
	ID          int64
	UserID      int64
	ChatID      int64
	Username    string
	MessageText string
	MessageType string
	CreatedAt   time.Time
	StreakDays  int
	// TrainingPhotoURL — публичный URL фото из мини-аппа (#training_done), если приложили.
	TrainingPhotoURL string
}

// PackGroupChatReaction — эмодзи-реакция на сообщение общего чата.
type PackGroupChatReaction struct {
	Emoji  string   `json:"emoji"`
	Count  int      `json:"count"`
	Me     bool     `json:"me,omitempty"`
	Voters []string `json:"voters,omitempty"`
}

// PackGroupChatMessage — общий чат мини-апpa «Стая» (все участники; Лео — при @leo / @бот).
type PackGroupChatMessage struct {
	ID              int64                   `json:"id"`
	UserID          int64                   `json:"user_id"`
	Username        string                  `json:"username"`
	Text            string                  `json:"text"`
	CreatedAt       string                  `json:"created_at"`
	IsLeo           bool                    `json:"is_leo"`
	AuthorPhotoURL  string                  `json:"author_photo_url,omitempty"`
	ReplyToID       int64                   `json:"reply_to_id,omitempty"`
	ReplyToUsername string                  `json:"reply_to_username,omitempty"`
	ReplyToText     string                  `json:"reply_to_text,omitempty"`
	ReplyToIsLeo    bool                    `json:"reply_to_is_leo,omitempty"`
	EditedAt        string                  `json:"edited_at,omitempty"`
	Reactions       []PackGroupChatReaction `json:"reactions,omitempty"`
}

// MiniappPersonalChatMessage — личный чат юзера с Лео (источник правды живёт в БД,
// синхронизирован между всеми устройствами одного юзера). role = "user" | "leo".
type MiniappPersonalChatMessage struct {
	ID        int64  `json:"id"`
	Role      string `json:"role"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
	LikeCount int    `json:"like_count,omitempty"`
	LikeMe    bool   `json:"like_me,omitempty"`
}

// MiniappSupportConversation — краткая сводка диалога юзера в личном чате miniapp.
type MiniappSupportConversation struct {
	UserID      int64  `json:"user_id"`
	DisplayName string `json:"display_name"`
	LastRole    string `json:"last_role"`
	LastText    string `json:"last_text"`
	LastCreated string `json:"last_created"`
	NeedsReply  bool   `json:"needs_reply"`
}

// MiniappSupportChatMessage — отдельный чат пользователя с поддержкой.
type MiniappSupportChatMessage struct {
	ID        int64  `json:"id"`
	Role      string `json:"role"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}

// MiniappFeedReport — жалоба на пост или комментарий в ленте стаи.
type MiniappFeedReport struct {
	ID              int64  `json:"id"`
	ReporterUserID  int64  `json:"reporter_user_id"`
	ReporterName    string `json:"reporter_name,omitempty"`
	TargetType      string `json:"target_type"`
	UserMessageID   int64  `json:"user_message_id"`
	ThreadReplyID   int64  `json:"thread_reply_id,omitempty"`
	TargetUserID    int64  `json:"target_user_id"`
	TargetName      string `json:"target_name,omitempty"`
	TargetText      string `json:"target_text"`
	Status          string `json:"status"`
	CreatedAt       string `json:"created_at"`
}

// TrainingSession представляет одну зафиксированную тренировочную сессию.
type TrainingSession struct {
	ID             int64     `json:"id" db:"id"`
	UserID         int64     `json:"user_id" db:"user_id"`
	ChatID         int64     `json:"chat_id" db:"chat_id"`
	SessionDate    string    `json:"session_date" db:"session_date"` // YYYY-MM-DD (MSK)
	MessageText    string    `json:"message_text" db:"message_text"`
	TrainingsCount int       `json:"trainings_count" db:"trainings_count"` // обычно 1
	CupsAdded      int       `json:"cups_added" db:"cups_added"`
	IsBonus        bool      `json:"is_bonus" db:"is_bonus"` // true для бонусной записи (+10)
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}

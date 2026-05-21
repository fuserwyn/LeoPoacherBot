package rag

import "time"

// MessageDoc — одна реплика в векторном хранилище (payload + embedding).
type MessageDoc struct {
	SessionID  string
	Channel    Channel
	UserID     int64
	PackChatID int64
	Role       string // user | leo
	Text       string
	CreatedAt  time.Time
	SourceID   int64 // id строки в Postgres (miniapp_*_chat), 0 если нет
}

// RetrievedChunk — фрагмент контекста для промпта.
type RetrievedChunk struct {
	Role      string
	Text      string
	Score     float32
	CreatedAt time.Time
}

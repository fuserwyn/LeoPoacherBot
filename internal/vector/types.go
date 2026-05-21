package vector

import "time"

// ChatPoint — сообщение чата в векторном хранилище.
type ChatPoint struct {
	MessageID   int64
	ChatID      int64
	UserID      int64
	Username    string
	MessageText string
	MessageType string
	CreatedAt   time.Time
}

// SearchHit — результат семантического поиска по чату.
type SearchHit struct {
	MessageID   int64
	ChatID      int64
	UserID      int64
	Username    string
	MessageText string
	MessageType string
	CreatedAt   time.Time
	Score       float32
}

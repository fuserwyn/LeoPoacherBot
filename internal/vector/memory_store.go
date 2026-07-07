package vector

// MemoryStore — семантическая память чата (Qdrant или заглушка).
type MemoryStore interface {
	Enabled() bool
	EnsureCollection() error
	UpsertMessage(p ChatPoint) error
	SearchChat(chatID int64, query string, limit int) ([]SearchHit, error)
}

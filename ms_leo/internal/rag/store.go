package rag

import (
	"context"
	"time"
)

// Embedder — векторизация текста (OpenRouter / OpenAI-compatible).
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// MemoryStats — что лежит в памяти Лео (для раздела «Система» в админке).
type MemoryStats struct {
	Total int `json:"total"`
	Old   int `json:"old"`
}

// Store — изолированные RAG-сессии в Qdrant.
//
// Удаление здесь такое же важное, как индексация: сообщение могли удалить или
// скрыть модерацией, а человек — уйти из стаи. Без удаления Лео продолжит
// цитировать то, чего в ленте уже нет.
type Store interface {
	Enabled() bool
	EnsureCollection(ctx context.Context) error
	Index(ctx context.Context, doc MessageDoc) error
	Retrieve(ctx context.Context, sessionID string, query string, limit int) ([]RetrievedChunk, error)
	// DeleteBySource — забыть одно сообщение (source_id строки в Postgres).
	DeleteBySource(ctx context.Context, sourceID int64) error
	// DeleteByUser — забыть всё, что писал человек.
	DeleteByUser(ctx context.Context, userID int64) error
	// DeleteOlderThan — ретеншен: забыть всё старше даты.
	DeleteOlderThan(ctx context.Context, cutoff time.Time) error
	// Stats — сколько всего точек и сколько из них старше cutoff.
	Stats(ctx context.Context, cutoff time.Time) (MemoryStats, error)
}

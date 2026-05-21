package rag

import "context"

// Embedder — векторизация текста (OpenRouter / OpenAI-compatible).
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// Store — изолированные RAG-сессии в Qdrant.
type Store interface {
	Enabled() bool
	EnsureCollection(ctx context.Context) error
	Index(ctx context.Context, doc MessageDoc) error
	Retrieve(ctx context.Context, sessionID string, query string, limit int) ([]RetrievedChunk, error)
}

package rag

import "context"

// NoopStore — RAG выключен (Qdrant не настроен).
type NoopStore struct{}

func (NoopStore) Enabled() bool { return false }

func (NoopStore) EnsureCollection(context.Context) error { return nil }

func (NoopStore) Index(context.Context, MessageDoc) error { return nil }

func (NoopStore) Retrieve(context.Context, string, string, int) ([]RetrievedChunk, error) {
	return nil, nil
}

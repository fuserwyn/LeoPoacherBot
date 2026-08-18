package rag

import (
	"context"
	"time"
)

// NoopStore — RAG выключен (Qdrant не настроен).
type NoopStore struct{}

func (NoopStore) Enabled() bool { return false }

func (NoopStore) EnsureCollection(context.Context) error { return nil }

func (NoopStore) Index(context.Context, MessageDoc) error { return nil }

func (NoopStore) Retrieve(context.Context, string, string, int) ([]RetrievedChunk, error) {
	return nil, nil
}

func (NoopStore) DeleteBySource(context.Context, int64) error { return nil }

func (NoopStore) DeleteByUser(context.Context, int64) error { return nil }

func (NoopStore) DeleteOlderThan(context.Context, time.Time) error { return nil }

func (NoopStore) Stats(context.Context, time.Time) (MemoryStats, error) {
	return MemoryStats{}, nil
}

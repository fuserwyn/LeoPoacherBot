package vector

// NoopStore — заглушка без Qdrant; контекст для ИИ берётся из Postgres.
type NoopStore struct{}

func NewNoopStore() *NoopStore {
	return &NoopStore{}
}

func (s *NoopStore) Enabled() bool {
	return false
}

func (s *NoopStore) EnsureCollection() error {
	return nil
}

func (s *NoopStore) UpsertMessage(ChatPoint) error {
	return nil
}

func (s *NoopStore) SearchChat(int64, string, int) ([]SearchHit, error) {
	return nil, nil
}

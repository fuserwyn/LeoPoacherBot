package database

import "testing"

func TestSamePostgresDSN(t *testing.T) {
	leo := "postgresql://postgres:x@postgres.railway.internal:5432/railway"
	tracker := "postgresql://postgres:y@postgres-ztwn.railway.internal:5432/railway"
	if samePostgresDSN(leo, tracker) {
		t.Fatal("different hosts must not look like one database")
	}
	if !samePostgresDSN(leo, leo) {
		t.Fatal("same url")
	}
	if samePostgresDSN("", tracker) {
		t.Fatal("empty")
	}
}

func TestEnsureTrackerSchemaNeedsDB(t *testing.T) {
	if err := (*Database)(nil).EnsureTrackerSchema(); err == nil {
		t.Fatal("nil db must fail")
	}
	if err := (&Database{}).EnsureTrackerSchema(); err == nil {
		t.Fatal("empty db must fail")
	}
}

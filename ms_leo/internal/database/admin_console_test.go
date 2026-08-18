package database

import "testing"

// SQL-редактор открыт админам мини-аппа, поэтому граница «только чтение»
// важнее удобства: одна опечатка в UPDATE без WHERE стоила бы базы стаи.
func TestAdminQueryIsReadOnly(t *testing.T) {
	readOnly := []string{
		"SELECT 1",
		"select * from training_state limit 10",
		"  WITH x AS (SELECT 1) SELECT * FROM x",
		"-- смотрим стрики\nSELECT user_id, streak FROM training_state",
		"/* комментарий */ EXPLAIN SELECT 1",
		"SELECT 1;",
	}
	for _, q := range readOnly {
		if !AdminQueryIsReadOnly(q) {
			t.Errorf("должен считаться читающим: %q", q)
		}
	}

	writes := []string{
		"DELETE FROM training_state",
		"update training_state set streak = 0",
		"DROP TABLE users",
		"TRUNCATE events",
		"SELECT 1; DELETE FROM training_state",       // второй запрос за точкой с запятой
		"-- SELECT\nDELETE FROM training_state",      // чтение только в комментарии
		"/* SELECT */ UPDATE training_state SET x=1", // то же блочным комментарием
		"",
		"   ",
		"INSERT INTO events (name) VALUES ('x')",
	}
	for _, q := range writes {
		if AdminQueryIsReadOnly(q) {
			t.Errorf("не должен проходить как чтение: %q", q)
		}
	}
}

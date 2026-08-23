package database

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestApplyStreakSaveForUserScope(t *testing.T) {
	t.Run("updates last_training_date for pack and private rows", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
		if err != nil {
			t.Fatalf("sqlmock.New: %v", err)
		}
		defer db.Close()
		d := &Database{db: db}

		const userID, packID int64 = 60495479, -1001234567890
		mock.ExpectExec(`UPDATE training_state\s+SET last_training_date`).
			WithArgs(userID, packID, "2026-07-16", 107, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))

		if err := d.ApplyStreakSaveForUserScope(userID, packID, "2026-07-16", 107); err != nil {
			t.Fatalf("ApplyStreakSaveForUserScope: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("sqlmock: %v", err)
		}
	})

	t.Run("trims date and clamps negative restore streak", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
		if err != nil {
			t.Fatalf("sqlmock.New: %v", err)
		}
		defer db.Close()
		d := &Database{db: db}

		mock.ExpectExec(`UPDATE training_state\s+SET last_training_date`).
			WithArgs(int64(1), int64(-2), "2026-07-16", 0, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))

		if err := d.ApplyStreakSaveForUserScope(1, -2, "  2026-07-16  ", -5); err != nil {
			t.Fatalf("ApplyStreakSaveForUserScope: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("sqlmock: %v", err)
		}
	})

	t.Run("empty date is error", func(t *testing.T) {
		d := &Database{}
		if err := d.ApplyStreakSaveForUserScope(1, -2, "   ", 10); err == nil {
			t.Fatal("expected empty date error")
		}
	})

	t.Run("noop on zero ids", func(t *testing.T) {
		d := &Database{}
		if err := d.ApplyStreakSaveForUserScope(0, -2, "2026-07-16", 10); err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if err := d.ApplyStreakSaveForUserScope(1, 0, "2026-07-16", 10); err != nil {
			t.Fatalf("unexpected: %v", err)
		}
	})
}

func TestSetTimerStartTime_onlyTouchesTimerColumn(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()
	d := &Database{db: db}

	const userID, chatID int64 = 42, -1001
	start := "2026-07-17T23:47:07+03:00"
	// Регрессия: запрос НЕ должен трогать streak_days / last_training_date.
	mock.ExpectExec(`UPDATE training_state\s+SET timer_start_time = \$3,\s+updated_at = \$4\s+WHERE user_id = \$1 AND chat_id = \$2`).
		WithArgs(userID, chatID, start, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := d.SetTimerStartTime(userID, chatID, start); err != nil {
		t.Fatalf("SetTimerStartTime: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
}

func TestIncrementStreakSaveAttemptsUsed(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()
	d := &Database{db: db}

	mock.ExpectQuery(`UPDATE training_state\s+SET streak_save_attempts_used`).
		WithArgs(int64(7), int64(-1001), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"streak_save_attempts_used"}).AddRow(4))

	used, err := d.IncrementStreakSaveAttemptsUsed(7, -1001)
	if err != nil {
		t.Fatalf("IncrementStreakSaveAttemptsUsed: %v", err)
	}
	if used != 4 {
		t.Fatalf("used=%d, want 4", used)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
}

func TestGetStreakSaveAttemptsUsed(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()
	d := &Database{db: db}

	mock.ExpectQuery(`SELECT COALESCE\(streak_save_attempts_used`).
		WithArgs(int64(7), int64(-1001)).
		WillReturnRows(sqlmock.NewRows([]string{"used"}).AddRow(3))

	used, err := d.GetStreakSaveAttemptsUsed(7, -1001)
	if err != nil {
		t.Fatalf("GetStreakSaveAttemptsUsed: %v", err)
	}
	if used != 3 {
		t.Fatalf("used=%d, want 3", used)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
}

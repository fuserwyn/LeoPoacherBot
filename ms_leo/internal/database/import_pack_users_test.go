package database

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestImportPackUser_insertsAllThree(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	d := &Database{db: db}
	opts := ImportPackUserOpts{SkipPaywall: false, StartTimer: true}

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO paywall_access_requests`).
		WithArgs(int64(42), int64(-1001)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO training_state`).
		WithArgs(int64(42), "Стас", int64(-1001), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO miniapp_user_profile`).
		WithArgs(int64(42), int64(-1001), "Стас").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	res, err := d.ImportPackUser(-1001, ImportPackUserInput{UserID: 42, Username: "Стас"}, opts)
	if err != nil {
		t.Fatalf("ImportPackUser: %v", err)
	}
	if !res.PaywallInserted || !res.TrainingInserted || !res.ProfileInserted {
		t.Fatalf("result: %+v", res)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestImportPackUser_skipPaywall(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	d := &Database{db: db}
	opts := ImportPackUserOpts{SkipPaywall: true, StartTimer: false}

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO training_state`).
		WithArgs(int64(7), "", int64(-1001), nil, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO miniapp_user_profile`).
		WithArgs(int64(7), int64(-1001), "").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	res, err := d.ImportPackUser(-1001, ImportPackUserInput{UserID: 7}, opts)
	if err != nil {
		t.Fatalf("ImportPackUser: %v", err)
	}
	if res.PaywallInserted {
		t.Fatal("paywall should be skipped")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

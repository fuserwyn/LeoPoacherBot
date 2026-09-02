package bot

import (
	"database/sql"
	"testing"

	"leo-bot/internal/database"
)

func TestAdminMoneyKindCurrencyLabel(t *testing.T) {
	if got := adminMoneyKindCurrencyLabel("access", "XTR"); got != "доступ · ⭐" {
		t.Fatalf("access stars: %q", got)
	}
	if got := adminMoneyKindCurrencyLabel("donation", "RUB"); got != "донат · ₽" {
		t.Fatalf("donation rub: %q", got)
	}
}

func TestAdminBuildMoneyStatsTableOrder(t *testing.T) {
	tbl := adminBuildMoneyStatsTable([]database.AdminMoneyKindSum{
		{Kind: "donation", Currency: "RUB", Count: 2, AmountMinor: 100000},
		{Kind: "access", Currency: "XTR", Count: 1, AmountMinor: 50},
	})
	if len(tbl.Rows) != 4 {
		t.Fatalf("rows: %d", len(tbl.Rows))
	}
	if tbl.Rows[0][0] != "доступ · ⭐" {
		t.Fatalf("first row: %v", tbl.Rows[0])
	}
	amt := sql.NullInt64{Int64: 100000, Valid: true}
	cur := sql.NullString{String: "RUB", Valid: true}
	if tbl.Rows[3][2] != adminFormatPaymentAmount(amt, cur) {
		t.Fatalf("donation rub amount: %v", tbl.Rows[3])
	}
}

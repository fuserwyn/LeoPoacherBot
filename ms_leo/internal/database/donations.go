package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Donation — добровольный донат из профиля мини-аппа (payload инвойса dn_<id>).
// Доступ к стае не выдаёт и с paywall_access_requests не связан: кик за неактивность
// и платный возврат работают независимо от донатов.
type Donation struct {
	ID                      int64
	UserID                  int64
	Provider                string // stars | yookassa
	Status                  string // pending | completed
	AmountMinor             int64  // для stars — число звёзд, для ЮKassa — копейки
	Currency                string
	TelegramPaymentChargeID sql.NullString
	YookassaPaymentID       sql.NullString
	ThanksSentAt            sql.NullTime
	CreatedAt               time.Time
	CompletedAt             sql.NullTime
}

const donationSelectColumns = `id, user_id, provider, status, amount_minor, currency,
	       telegram_payment_charge_id, yookassa_payment_id, thanks_sent_at, created_at, completed_at`

func scanDonation(row *sql.Row) (*Donation, error) {
	var d Donation
	err := row.Scan(
		&d.ID, &d.UserID, &d.Provider, &d.Status, &d.AmountMinor, &d.Currency,
		&d.TelegramPaymentChargeID, &d.YookassaPaymentID, &d.ThanksSentAt, &d.CreatedAt, &d.CompletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// InsertDonation — заявка на донат в статусе pending; id уходит в payload инвойса (dn_<id>).
func (d *Database) InsertDonation(userID int64, provider string, amountMinor int, currency string) (int64, error) {
	const q = `
		INSERT INTO donations (user_id, provider, status, amount_minor, currency)
		VALUES ($1, $2, 'pending', $3, $4)
		RETURNING id`
	var id int64
	err := d.db.QueryRow(q, userID, strings.TrimSpace(provider), amountMinor, strings.ToUpper(strings.TrimSpace(currency))).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert donation: %w", err)
	}
	return id, nil
}

func (d *Database) GetDonationByID(id int64) (*Donation, error) {
	q := `SELECT ` + donationSelectColumns + ` FROM donations WHERE id = $1`
	return scanDonation(d.db.QueryRow(q, id))
}

// SetDonationYookassaPaymentID — id платежа ЮKassa для опроса статуса, если вебхук не дошёл.
func (d *Database) SetDonationYookassaPaymentID(id int64, paymentID string) error {
	const q = `UPDATE donations SET yookassa_payment_id = $2 WHERE id = $1 AND status = 'pending'`
	if _, err := d.db.Exec(q, id, strings.TrimSpace(paymentID)); err != nil {
		return fmt.Errorf("set donation yookassa payment id: %w", err)
	}
	return nil
}

// CompleteDonation — закрывает pending-донат. false без ошибки = уже закрыт (повтор вебхука
// ЮKassa или ретрай successful_payment от Telegram), вызывающий не должен благодарить дважды.
func (d *Database) CompleteDonation(id, userID int64, chargeID, yookassaPaymentID string, amountMinor int, currency string) (bool, error) {
	const q = `
		UPDATE donations
		SET status = 'completed',
		    completed_at = NOW(),
		    telegram_payment_charge_id = COALESCE(NULLIF($3, ''), telegram_payment_charge_id),
		    yookassa_payment_id = COALESCE(NULLIF($4, ''), yookassa_payment_id),
		    amount_minor = CASE WHEN $5 > 0 THEN $5 ELSE amount_minor END,
		    currency = CASE WHEN NULLIF($6, '') IS NULL THEN currency ELSE $6 END
		WHERE id = $1 AND user_id = $2 AND status = 'pending'`
	res, err := d.db.Exec(q, id, userID,
		strings.TrimSpace(chargeID), strings.TrimSpace(yookassaPaymentID),
		amountMinor, strings.ToUpper(strings.TrimSpace(currency)))
	if err != nil {
		return false, fmt.Errorf("complete donation %d: %w", id, err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// MarkDonationThanksSent — «спасибо» в личку отправлено (защита от дублей при ретраях).
func (d *Database) MarkDonationThanksSent(id int64) error {
	const q = `UPDATE donations SET thanks_sent_at = NOW() WHERE id = $1 AND thanks_sent_at IS NULL`
	if _, err := d.db.Exec(q, id); err != nil {
		return fmt.Errorf("mark donation thanks sent %d: %w", id, err)
	}
	return nil
}

// PendingYookassaDonations — незакрытые донаты по ссылке ЮKassa: их статус доопрашиваем
// через API (вебхук ms_payments донаты не обслуживает, см. миграцию 73).
// Старые заявки не тянем: если пользователь просто не оплатил, опрашивать нечего.
func (d *Database) PendingYookassaDonations(userID int64, maxAge time.Duration, limit int) ([]*Donation, error) {
	if limit <= 0 {
		limit = 5
	}
	q := `
		SELECT ` + donationSelectColumns + `
		FROM donations
		WHERE user_id = $1
		  AND status = 'pending'
		  AND provider = 'yookassa'
		  AND yookassa_payment_id IS NOT NULL
		  AND created_at > NOW() - $2::interval
		ORDER BY id DESC
		LIMIT $3`
	rows, err := d.db.Query(q, userID, fmt.Sprintf("%d seconds", int(maxAge.Seconds())), limit)
	if err != nil {
		return nil, fmt.Errorf("pending yookassa donations: %w", err)
	}
	defer rows.Close()

	var out []*Donation
	for rows.Next() {
		var dn Donation
		if err := rows.Scan(
			&dn.ID, &dn.UserID, &dn.Provider, &dn.Status, &dn.AmountMinor, &dn.Currency,
			&dn.TelegramPaymentChargeID, &dn.YookassaPaymentID, &dn.ThanksSentAt, &dn.CreatedAt, &dn.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan pending yookassa donation: %w", err)
		}
		out = append(out, &dn)
	}
	return out, rows.Err()
}

// UserDonationsSummary — сколько раз пользователь поддержал проект (для профиля).
func (d *Database) UserDonationsSummary(userID int64) (count int, err error) {
	const q = `SELECT COUNT(*) FROM donations WHERE user_id = $1 AND status = 'completed'`
	if err := d.db.QueryRow(q, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("donations summary user=%d: %w", userID, err)
	}
	return count, nil
}

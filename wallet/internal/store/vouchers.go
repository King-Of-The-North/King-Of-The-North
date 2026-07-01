package store

import (
	"context"
	"fmt"
	"time"
)

// Voucher sentinels.
var (
	ErrVoucherNotFound = fmt.Errorf("store: voucher not found")
	ErrAlreadyRedeemed = fmt.Errorf("store: voucher already redeemed")
	ErrVoucherExpired  = fmt.Errorf("store: voucher expired")
)

// Voucher status codes (mirror the SMALLINT column).
const (
	voucherIssued   = 1
	voucherRedeemed = 2
	voucherExpired  = 3
)

// VoucherRow is a stored offline spending voucher (ADR-0012). The amount is pre-reserved
// from available_credit at issue, so an outstanding voucher's credit has already left the
// balance — this bounds double-spend to the (capped) voucher amount.
type VoucherRow struct {
	ID            string
	UserID        string
	AmountMinor   int64
	Status        string // "issued" | "redeemed" | "expired"
	Nonce         []byte
	Sig           []byte
	IssuedAt      time.Time
	ExpiresAt     time.Time
	RedeemTrxCode string
}

func voucherStatusName(code int16) string {
	switch code {
	case voucherRedeemed:
		return "redeemed"
	case voucherExpired:
		return "expired"
	default:
		return "issued"
	}
}

// IssueVoucher atomically reserves amountMinor from the user's available_credit and
// stores a signed voucher. The reservation uses the same SELECT ... FOR UPDATE guard as
// Deduct, so credit can never go negative under concurrency. An audit row records the
// reservation. The service supplies id/nonce/sig/expiry (it holds the signing key).
func (s *Store) IssueVoucher(ctx context.Context, id, userID string, amountMinor int64, nonce, sig []byte, expiresAt time.Time) error {
	if amountMinor <= 0 {
		return fmt.Errorf("store: voucher amount must be positive")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: issue voucher begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var available int64
	err = tx.QueryRow(ctx,
		`SELECT available_credit FROM accounts WHERE user_id = $1 FOR UPDATE`, userID).Scan(&available)
	if isNoRows(err) || isInvalidText(err) {
		return ErrAccountNotFound
	}
	if err != nil {
		return fmt.Errorf("store: issue voucher lock: %w", err)
	}
	if amountMinor > available {
		return ErrInsufficientCredit
	}

	if _, err := tx.Exec(ctx,
		`UPDATE accounts SET available_credit = available_credit - $2, updated_at = now() WHERE user_id = $1`,
		userID, amountMinor); err != nil {
		return fmt.Errorf("store: issue voucher debit: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO vouchers (id, user_id, amount_minor, status, nonce, sig, issued_at, expires_at)
VALUES ($1, $2, $3, 1, $4, $5, now(), $6)`,
		id, userID, amountMinor, nonce, sig, expiresAt); err != nil {
		return fmt.Errorf("store: issue voucher insert: %w", err)
	}
	// Audit row for the reservation (replicated ledger / receipts).
	if _, err := tx.Exec(ctx, `
INSERT INTO transactions (id, user_id, other_trx_code, amount, payment_status, trx_status, created_at)
VALUES (gen_random_uuid(), $1, $2, $3, 2, 1, now())`,
		userID, "voucher-issue:"+id, amountMinor); err != nil {
		return fmt.Errorf("store: issue voucher audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: issue voucher commit: %w", err)
	}
	return nil
}

// RedeemVoucher marks a voucher redeemed exactly once and writes the settlement audit
// row. No further deduct — the credit was reserved at issue. Returns the voucher's owner
// and amount so the caller can append a ledger entry. Concurrent redeems: the FOR UPDATE
// lock serializes them, so only the first wins (the rest get ErrAlreadyRedeemed).
func (s *Store) RedeemVoucher(ctx context.Context, id, otherTrxCode string) (VoucherRow, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return VoucherRow{}, fmt.Errorf("store: redeem begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var v VoucherRow
	var status int16
	err = tx.QueryRow(ctx,
		`SELECT user_id, amount_minor, status, expires_at FROM vouchers WHERE id = $1 FOR UPDATE`, id).
		Scan(&v.UserID, &v.AmountMinor, &status, &v.ExpiresAt)
	if isNoRows(err) || isInvalidText(err) {
		return VoucherRow{}, ErrVoucherNotFound
	}
	if err != nil {
		return VoucherRow{}, fmt.Errorf("store: redeem lock: %w", err)
	}
	switch {
	case status == voucherRedeemed:
		return VoucherRow{}, ErrAlreadyRedeemed
	case status == voucherExpired || time.Now().After(v.ExpiresAt):
		return VoucherRow{}, ErrVoucherExpired
	}

	if _, err := tx.Exec(ctx,
		`UPDATE vouchers SET status = 2, redeemed_at = now(), redeem_trx_code = $2 WHERE id = $1`,
		id, otherTrxCode); err != nil {
		return VoucherRow{}, fmt.Errorf("store: redeem update: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO transactions (id, user_id, other_trx_code, amount, payment_status, trx_status, created_at)
VALUES (gen_random_uuid(), $1, $2, $3, 2, 1, now())`,
		v.UserID, otherTrxCode, v.AmountMinor); err != nil {
		return VoucherRow{}, fmt.Errorf("store: redeem audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return VoucherRow{}, fmt.Errorf("store: redeem commit: %w", err)
	}
	v.ID = id
	v.Status = "redeemed"
	v.RedeemTrxCode = otherTrxCode
	return v, nil
}

// ExpireVouchers flips every past-due issued voucher to expired and refunds its reserved
// amount back to the owner's available_credit (with an audit reversal), so no credit is
// lost when a voucher is never spent. Returns how many expired and the total refunded.
func (s *Store) ExpireVouchers(ctx context.Context) (int, int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("store: expire begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx,
		`UPDATE vouchers SET status = 3 WHERE status = 1 AND expires_at < now() RETURNING id, user_id, amount_minor`)
	if err != nil {
		return 0, 0, fmt.Errorf("store: expire update: %w", err)
	}
	type exp struct {
		id, user string
		amount   int64
	}
	var expired []exp
	for rows.Next() {
		var e exp
		if err := rows.Scan(&e.id, &e.user, &e.amount); err != nil {
			rows.Close()
			return 0, 0, fmt.Errorf("store: expire scan: %w", err)
		}
		expired = append(expired, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, fmt.Errorf("store: expire rows: %w", err)
	}

	var refunded int64
	for _, e := range expired {
		if _, err := tx.Exec(ctx,
			`UPDATE accounts SET available_credit = available_credit + $2, updated_at = now() WHERE user_id = $1`,
			e.user, e.amount); err != nil {
			return 0, 0, fmt.Errorf("store: expire refund: %w", err)
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO transactions (id, user_id, other_trx_code, amount, payment_status, trx_status, created_at)
VALUES (gen_random_uuid(), $1, $2, $3, 4, 1, now())`, // payment_status 4 = Full-Refund
			e.user, "voucher-expire:"+e.id, e.amount); err != nil {
			return 0, 0, fmt.Errorf("store: expire audit: %w", err)
		}
		refunded += e.amount
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, 0, fmt.Errorf("store: expire commit: %w", err)
	}
	return len(expired), refunded, nil
}

// ListVouchers returns a user's vouchers, newest first.
func (s *Store) ListVouchers(ctx context.Context, userID string) ([]VoucherRow, error) {
	const q = `
SELECT id, amount_minor, status, nonce, sig, issued_at, expires_at, COALESCE(redeem_trx_code, '')
FROM vouchers WHERE user_id = $1 ORDER BY issued_at DESC`
	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("store: list vouchers: %w", err)
	}
	defer rows.Close()

	out := make([]VoucherRow, 0)
	for rows.Next() {
		var v VoucherRow
		var status int16
		if err := rows.Scan(&v.ID, &v.AmountMinor, &status, &v.Nonce, &v.Sig,
			&v.IssuedAt, &v.ExpiresAt, &v.RedeemTrxCode); err != nil {
			return nil, fmt.Errorf("store: list vouchers scan: %w", err)
		}
		v.UserID = userID
		v.Status = voucherStatusName(status)
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		if isInvalidText(err) {
			return []VoucherRow{}, nil
		}
		return nil, fmt.Errorf("store: list vouchers rows: %w", err)
	}
	return out, nil
}

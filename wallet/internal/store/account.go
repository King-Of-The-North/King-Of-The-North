package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// Account is the ledger view of a user's money. All *Minor fields are kuruş.
type Account struct {
	UserID          string
	PrincipalMinor  int64
	ProjectedYield  int64
	CreditLimit     int64
	AvailableCredit int64
	LTVRatio        float64
	LockupEndDate   time.Time
	PoolType        string
}

// Deposit records a deposit and its Day-Zero grant in one upsert: principal is the
// 1:1 tokenized fiat, credit_limit is L0, available_credit starts at L0. A repeat
// deposit for the same user overwrites the account (one deposit per user in the
// demo — see docs/plans/day-zero-wallet.md and the design notes).
func (s *Store) Deposit(ctx context.Context, a Account) error {
	const q = `
INSERT INTO accounts (
    user_id, principal_balance, projected_yield, credit_limit,
    available_credit, lockup_end_date, pool_type, created_at, updated_at
) VALUES ($1,$2,$3,$4,$4,$5,$6, now(), now())
ON CONFLICT (user_id) DO UPDATE SET
    principal_balance = EXCLUDED.principal_balance,
    projected_yield   = EXCLUDED.projected_yield,
    credit_limit      = EXCLUDED.credit_limit,
    available_credit  = EXCLUDED.available_credit,
    lockup_end_date   = EXCLUDED.lockup_end_date,
    pool_type         = EXCLUDED.pool_type,
    updated_at        = now()`
	_, err := s.pool.Exec(ctx, q,
		a.UserID, a.PrincipalMinor, a.ProjectedYield, a.CreditLimit,
		a.LockupEndDate, a.PoolType)
	if err != nil {
		return fmt.Errorf("store: deposit: %w", err)
	}
	return nil
}

// GetAccount reads a user's account. Returns ErrAccountNotFound if absent.
func (s *Store) GetAccount(ctx context.Context, userID string) (Account, error) {
	const q = `
SELECT user_id, principal_balance, projected_yield, credit_limit,
       available_credit, ltv_ratio, lockup_end_date, pool_type
FROM accounts WHERE user_id = $1`
	var a Account
	err := s.pool.QueryRow(ctx, q, userID).Scan(
		&a.UserID, &a.PrincipalMinor, &a.ProjectedYield, &a.CreditLimit,
		&a.AvailableCredit, &a.LTVRatio, &a.LockupEndDate, &a.PoolType)
	if isNoRows(err) || isInvalidText(err) {
		return Account{}, ErrAccountNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("store: get account: %w", err)
	}
	return a, nil
}

// ErrInsufficientCredit means the cart total exceeds available credit.
var ErrInsufficientCredit = fmt.Errorf("store: insufficient credit")

// ErrSelfTransfer means a transfer named the same user as sender and receiver.
var ErrSelfTransfer = fmt.Errorf("store: cannot transfer to self")

// ErrDuplicateRef means a transfer ref was already used — the UNIQUE(other_trx_code)
// constraint makes ref idempotent, so a replay is rejected rather than double-applied.
var ErrDuplicateRef = fmt.Errorf("store: duplicate transfer ref")

// isUniqueViolation reports whether err is a Postgres unique-constraint violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// isInvalidText reports whether err is a Postgres invalid-text-representation error
// (SQLSTATE 22P02) — e.g. a user_id that isn't a valid UUID. For a user lookup that
// means "no such account", so callers map it to ErrAccountNotFound rather than leaking
// a 500 with the raw driver message.
func isInvalidText(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22P02"
}

// DeductResult is the outcome of a successful atomic deduction.
type DeductResult struct {
	TrxID          string // ledger row id (UUID)
	RemainingMinor int64  // available_credit after the deduction
}

// Deduct atomically charges amountMinor against available credit and records a
// transaction row, all in one transaction. SELECT ... FOR UPDATE locks the account
// row so concurrent scan-gate reads cannot double-spend (day-zero-wallet.md §2).
//
// The Moka settle call is intentionally NOT made here — an external call must not
// hold a row lock. The service settles after commit, then SetMokaPaymentID fills
// the id. Returns ErrInsufficientCredit (and rolls back) when the cart exceeds
// available credit, and ErrAccountNotFound if the user has no account.
func (s *Store) Deduct(ctx context.Context, userID string, amountMinor int64, otherTrxCode string) (DeductResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DeductResult{}, fmt.Errorf("store: deduct begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after Commit

	var available int64
	err = tx.QueryRow(ctx,
		`SELECT available_credit FROM accounts WHERE user_id = $1 FOR UPDATE`,
		userID).Scan(&available)
	if isNoRows(err) || isInvalidText(err) {
		return DeductResult{}, ErrAccountNotFound
	}
	if err != nil {
		return DeductResult{}, fmt.Errorf("store: deduct lock: %w", err)
	}

	if amountMinor > available {
		return DeductResult{}, ErrInsufficientCredit
	}

	remaining := available - amountMinor
	if _, err := tx.Exec(ctx,
		`UPDATE accounts SET available_credit = $2, updated_at = now() WHERE user_id = $1`,
		userID, remaining); err != nil {
		return DeductResult{}, fmt.Errorf("store: deduct update: %w", err)
	}

	// payment_status=2 (Payment), trx_status=1 (Successful) — settled against the
	// ledger; moka_payment_id is filled after the Moka settle call.
	var trxID string
	err = tx.QueryRow(ctx, `
INSERT INTO transactions (id, user_id, other_trx_code, amount, payment_status, trx_status, created_at)
VALUES (gen_random_uuid(), $1, $2, $3, 2, 1, now())
RETURNING id`,
		userID, otherTrxCode, amountMinor).Scan(&trxID)
	if err != nil {
		return DeductResult{}, fmt.Errorf("store: deduct insert trx: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return DeductResult{}, fmt.Errorf("store: deduct commit: %w", err)
	}
	return DeductResult{TrxID: trxID, RemainingMinor: remaining}, nil
}

// SetMokaPaymentID records Moka's payment id on a settled transaction, called after
// the (mock) settle returns. Settle failures are out of scope for the demo (the mock
// always succeeds); a real failure path would reverse the deduction.
func (s *Store) SetMokaPaymentID(ctx context.Context, trxID, mokaPaymentID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE transactions SET moka_payment_id = $2 WHERE id = $1`, trxID, mokaPaymentID)
	if err != nil {
		return fmt.Errorf("store: set moka payment id: %w", err)
	}
	return nil
}

// TransferResult is the outcome of a successful atomic credit transfer.
type TransferResult struct {
	FromRemainingMinor int64  // sender available_credit after
	ToAvailableMinor   int64  // receiver available_credit after
	FromTrxCode        string // audit code for the debit row, "<ref>:out"
	ToTrxCode          string // audit code for the credit row, "<ref>:in"
}

// TransferCredit atomically moves amountMinor of spendable credit from one user to
// another. Only available_credit moves — the locked principal and the Day-Zero
// credit_limit stay put (a transfer reallocates liquidity, not the deposit). Both
// account rows are locked in a single ORDER BY ... FOR UPDATE query, so concurrent
// transfers always take locks in the same order and cannot deadlock, and a sender's
// available_credit can never go negative (mirrors Deduct, day-zero-wallet.md §2).
//
// Two audit rows are written for the replicated ledger: a debit ("<ref>:out") and a
// credit ("<ref>:in"). The UNIQUE(other_trx_code) constraint makes ref idempotent.
func (s *Store) TransferCredit(ctx context.Context, from, to string, amountMinor int64, ref string) (TransferResult, error) {
	if from == to {
		return TransferResult{}, ErrSelfTransfer
	}
	if amountMinor <= 0 {
		return TransferResult{}, fmt.Errorf("store: transfer amount must be positive")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TransferResult{}, fmt.Errorf("store: transfer begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after Commit

	// Lock both rows in a stable order (by user_id) to avoid deadlock between
	// concurrent transfers touching the same pair.
	rows, err := tx.Query(ctx,
		`SELECT user_id, available_credit FROM accounts WHERE user_id IN ($1,$2) ORDER BY user_id FOR UPDATE`,
		from, to)
	if isInvalidText(err) {
		return TransferResult{}, ErrAccountNotFound
	}
	if err != nil {
		return TransferResult{}, fmt.Errorf("store: transfer lock: %w", err)
	}
	balances := make(map[string]int64, 2)
	for rows.Next() {
		var id string
		var avail int64
		if err := rows.Scan(&id, &avail); err != nil {
			rows.Close()
			return TransferResult{}, fmt.Errorf("store: transfer scan: %w", err)
		}
		balances[id] = avail
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		// A malformed (non-UUID) from/to id surfaces here, not at Query time; treat it
		// as "no such account" rather than a 500.
		if isInvalidText(err) {
			return TransferResult{}, ErrAccountNotFound
		}
		return TransferResult{}, fmt.Errorf("store: transfer rows: %w", err)
	}
	fromAvail, okFrom := balances[from]
	_, okTo := balances[to]
	if !okFrom || !okTo {
		return TransferResult{}, ErrAccountNotFound
	}

	if amountMinor > fromAvail {
		return TransferResult{}, ErrInsufficientCredit
	}

	fromRemaining := fromAvail - amountMinor
	if _, err := tx.Exec(ctx,
		`UPDATE accounts SET available_credit = available_credit - $2, updated_at = now() WHERE user_id = $1`,
		from, amountMinor); err != nil {
		return TransferResult{}, fmt.Errorf("store: transfer debit: %w", err)
	}
	var toAvail int64
	if err := tx.QueryRow(ctx,
		`UPDATE accounts SET available_credit = available_credit + $2, updated_at = now() WHERE user_id = $1 RETURNING available_credit`,
		to, amountMinor).Scan(&toAvail); err != nil {
		return TransferResult{}, fmt.Errorf("store: transfer credit: %w", err)
	}

	// Audit rows: payment_status=2 (Payment), trx_status=1 (Successful). Both amounts
	// are positive; the :out/:in suffixes distinguish debit from credit and satisfy the
	// UNIQUE(other_trx_code) constraint (also making ref idempotent).
	fromCode, toCode := ref+":out", ref+":in"
	if _, err := tx.Exec(ctx, `
INSERT INTO transactions (id, user_id, other_trx_code, amount, payment_status, trx_status, created_at)
VALUES (gen_random_uuid(), $1, $2, $3, 2, 1, now()),
       (gen_random_uuid(), $4, $5, $3, 2, 1, now())`,
		from, fromCode, amountMinor, to, toCode); err != nil {
		if isUniqueViolation(err) {
			return TransferResult{}, ErrDuplicateRef
		}
		return TransferResult{}, fmt.Errorf("store: transfer insert trx: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return TransferResult{}, fmt.Errorf("store: transfer commit: %w", err)
	}
	return TransferResult{
		FromRemainingMinor: fromRemaining,
		ToAvailableMinor:   toAvail,
		FromTrxCode:        fromCode,
		ToTrxCode:          toCode,
	}, nil
}

// CreditNodeReward tops up the user's spendable credit for running a P2P node
// (DePIN, ADR-0008). The reward is credited to the authoritative Postgres ledger —
// never minted on the phone — as a normal entry, raising both credit_limit and
// available_credit. An audit transaction row is written for ledger replication.
func (s *Store) CreditNodeReward(ctx context.Context, userID string, rewardMinor int64, proofTrxCode string) (Account, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Account{}, fmt.Errorf("store: reward begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var a Account
	err = tx.QueryRow(ctx, `
UPDATE accounts SET
    credit_limit     = credit_limit + $2,
    available_credit = available_credit + $2,
    updated_at       = now()
WHERE user_id = $1
RETURNING user_id, principal_balance, projected_yield, credit_limit,
          available_credit, ltv_ratio, lockup_end_date, pool_type`,
		userID, rewardMinor).Scan(
		&a.UserID, &a.PrincipalMinor, &a.ProjectedYield, &a.CreditLimit,
		&a.AvailableCredit, &a.LTVRatio, &a.LockupEndDate, &a.PoolType)
	if isNoRows(err) || isInvalidText(err) {
		return Account{}, ErrAccountNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("store: reward update: %w", err)
	}

	// Audit entry: payment_status=2 (Payment), trx_status=1 (Successful). Reward is
	// a credit, not a spend; available_credit is maintained on the account column,
	// so this row is for the replicated audit log, not for re-deriving balances.
	if _, err := tx.Exec(ctx, `
INSERT INTO transactions (id, user_id, other_trx_code, amount, payment_status, trx_status, created_at)
VALUES (gen_random_uuid(), $1, $2, $3, 2, 1, now())`,
		userID, proofTrxCode, rewardMinor); err != nil {
		return Account{}, fmt.Errorf("store: reward insert trx: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Account{}, fmt.Errorf("store: reward commit: %w", err)
	}
	return a, nil
}

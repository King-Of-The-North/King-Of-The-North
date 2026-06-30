package store

import (
	"context"
	"fmt"
	"time"
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
	if isNoRows(err) {
		return Account{}, ErrAccountNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("store: get account: %w", err)
	}
	return a, nil
}

// ErrInsufficientCredit means the cart total exceeds available credit.
var ErrInsufficientCredit = fmt.Errorf("store: insufficient credit")

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
	if isNoRows(err) {
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
	if isNoRows(err) {
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

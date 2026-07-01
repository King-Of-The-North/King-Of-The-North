//go:build integration

// Integration tests for the ledger, run against a real Postgres (the safety
// invariant — FOR UPDATE atomic deduct — can only be verified against a real DB).
//
//	docker compose -f wallet/docker-compose.yml up -d
//	WALLET_TEST_DSN=postgres://kotn:kotn@localhost:5440/kotn_wallet?sslmode=disable \
//	  go test -tags integration ./wallet/internal/store/
package store

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("WALLET_TEST_DSN")
	if dsn == "" {
		dsn = "postgres://kotn:kotn@localhost:5440/kotn_wallet?sslmode=disable"
	}
	ctx := context.Background()
	s, err := New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

func newAccount(userID string, limit int64) Account {
	return Account{
		UserID:         userID,
		PrincipalMinor: 1_000_000,
		ProjectedYield: limit * 2,
		CreditLimit:    limit,
		LockupEndDate:  time.Now().UTC().AddDate(1, 0, 0),
		PoolType:       "fixed",
	}
}

func TestDepositAndGet(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid := uuid.NewString()

	if err := s.Deposit(ctx, newAccount(uid, 114142)); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	a, err := s.GetAccount(ctx, uid)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if a.CreditLimit != 114142 || a.AvailableCredit != 114142 {
		t.Fatalf("want limit=available=114142, got limit=%d available=%d", a.CreditLimit, a.AvailableCredit)
	}
}

func TestDeductHappyAndDecline(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid := uuid.NewString()
	if err := s.Deposit(ctx, newAccount(uid, 10000)); err != nil {
		t.Fatalf("deposit: %v", err)
	}

	res, err := s.Deduct(ctx, uid, 3000, uuid.NewString())
	if err != nil {
		t.Fatalf("deduct: %v", err)
	}
	if res.RemainingMinor != 7000 {
		t.Fatalf("want remaining 7000, got %d", res.RemainingMinor)
	}

	// Over-spend: must decline and leave the balance untouched.
	if _, err := s.Deduct(ctx, uid, 8000, uuid.NewString()); err != ErrInsufficientCredit {
		t.Fatalf("want ErrInsufficientCredit, got %v", err)
	}
	a, _ := s.GetAccount(ctx, uid)
	if a.AvailableCredit != 7000 {
		t.Fatalf("declined deduct changed balance: %d", a.AvailableCredit)
	}
}

// TestConcurrentDeductNoDoubleSpend is the invariant: many goroutines race to spend
// a balance that only covers some of them; the total deducted must never exceed the
// starting credit. FOR UPDATE serializes the check-and-decrement.
func TestConcurrentDeductNoDoubleSpend(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid := uuid.NewString()

	const limit = 10000
	const charge = 1000 // exactly 10 should succeed
	if err := s.Deposit(ctx, newAccount(uid, limit)); err != nil {
		t.Fatalf("deposit: %v", err)
	}

	const goroutines = 30
	var wg sync.WaitGroup
	var mu sync.Mutex
	var approved int
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, err := s.Deduct(ctx, uid, charge, uuid.NewString())
			if err == nil {
				mu.Lock()
				approved++
				mu.Unlock()
			} else if err != ErrInsufficientCredit {
				t.Errorf("unexpected deduct error: %v", err)
			}
		}()
	}
	wg.Wait()

	if approved != limit/charge {
		t.Fatalf("want exactly %d approvals, got %d (double-spend!)", limit/charge, approved)
	}
	a, _ := s.GetAccount(ctx, uid)
	if a.AvailableCredit != 0 {
		t.Fatalf("want 0 remaining, got %d", a.AvailableCredit)
	}
}

func TestTransferHappy(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	from, to := uuid.NewString(), uuid.NewString()
	if err := s.Deposit(ctx, newAccount(from, 10000)); err != nil {
		t.Fatalf("deposit from: %v", err)
	}
	if err := s.Deposit(ctx, newAccount(to, 5000)); err != nil {
		t.Fatalf("deposit to: %v", err)
	}

	res, err := s.TransferCredit(ctx, from, to, 3000, uuid.NewString())
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if res.FromRemainingMinor != 7000 || res.ToAvailableMinor != 8000 {
		t.Fatalf("want from=7000 to=8000, got from=%d to=%d", res.FromRemainingMinor, res.ToAvailableMinor)
	}
	fa, _ := s.GetAccount(ctx, from)
	ta, _ := s.GetAccount(ctx, to)
	if fa.AvailableCredit != 7000 || ta.AvailableCredit != 8000 {
		t.Fatalf("persisted balances wrong: from=%d to=%d", fa.AvailableCredit, ta.AvailableCredit)
	}
	// credit_limit and principal must be untouched — only available_credit moves.
	if fa.CreditLimit != 10000 || ta.CreditLimit != 5000 {
		t.Fatalf("transfer changed credit_limit: from=%d to=%d", fa.CreditLimit, ta.CreditLimit)
	}
}

func TestTransferInsufficient(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	from, to := uuid.NewString(), uuid.NewString()
	_ = s.Deposit(ctx, newAccount(from, 2000))
	_ = s.Deposit(ctx, newAccount(to, 0))

	if _, err := s.TransferCredit(ctx, from, to, 5000, uuid.NewString()); err != ErrInsufficientCredit {
		t.Fatalf("want ErrInsufficientCredit, got %v", err)
	}
	fa, _ := s.GetAccount(ctx, from)
	ta, _ := s.GetAccount(ctx, to)
	if fa.AvailableCredit != 2000 || ta.AvailableCredit != 0 {
		t.Fatalf("declined transfer moved money: from=%d to=%d", fa.AvailableCredit, ta.AvailableCredit)
	}
}

func TestTransferSelfAndMissing(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid := uuid.NewString()
	_ = s.Deposit(ctx, newAccount(uid, 1000))

	if _, err := s.TransferCredit(ctx, uid, uid, 100, uuid.NewString()); err != ErrSelfTransfer {
		t.Fatalf("want ErrSelfTransfer, got %v", err)
	}
	// Receiver has no account.
	if _, err := s.TransferCredit(ctx, uid, uuid.NewString(), 100, uuid.NewString()); err != ErrAccountNotFound {
		t.Fatalf("want ErrAccountNotFound, got %v", err)
	}
}

func TestTransferDuplicateRef(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	from, to := uuid.NewString(), uuid.NewString()
	_ = s.Deposit(ctx, newAccount(from, 10000))
	_ = s.Deposit(ctx, newAccount(to, 0))

	ref := uuid.NewString()
	if _, err := s.TransferCredit(ctx, from, to, 1000, ref); err != nil {
		t.Fatalf("first transfer: %v", err)
	}
	// Replaying the same ref must be rejected (idempotency), not double-applied.
	if _, err := s.TransferCredit(ctx, from, to, 1000, ref); err != ErrDuplicateRef {
		t.Fatalf("want ErrDuplicateRef, got %v", err)
	}
	fa, _ := s.GetAccount(ctx, from)
	if fa.AvailableCredit != 9000 {
		t.Fatalf("duplicate ref double-applied: from=%d", fa.AvailableCredit)
	}
}

// TestConcurrentTransferNoOverdraw races many transfers out of one sender whose credit
// only covers some of them; total moved must never exceed the starting balance. The
// dual FOR UPDATE serializes the check-and-decrement (same invariant as Deduct).
func TestConcurrentTransferNoOverdraw(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	from, to := uuid.NewString(), uuid.NewString()
	const limit = 10000
	const amount = 1000 // exactly 10 should succeed
	_ = s.Deposit(ctx, newAccount(from, limit))
	_ = s.Deposit(ctx, newAccount(to, 0))

	const goroutines = 30
	var wg sync.WaitGroup
	var mu sync.Mutex
	var approved int
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, err := s.TransferCredit(ctx, from, to, amount, uuid.NewString())
			if err == nil {
				mu.Lock()
				approved++
				mu.Unlock()
			} else if err != ErrInsufficientCredit {
				t.Errorf("unexpected transfer error: %v", err)
			}
		}()
	}
	wg.Wait()

	if approved != limit/amount {
		t.Fatalf("want exactly %d approvals, got %d (overdraw!)", limit/amount, approved)
	}
	fa, _ := s.GetAccount(ctx, from)
	ta, _ := s.GetAccount(ctx, to)
	if fa.AvailableCredit != 0 || ta.AvailableCredit != limit {
		t.Fatalf("conservation broken: from=%d to=%d", fa.AvailableCredit, ta.AvailableCredit)
	}
}

func TestListTransactions(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid := uuid.NewString()
	_ = s.Deposit(ctx, newAccount(uid, 100000))

	// A spend and a node reward → two rows, newest first. Unique codes so the test is
	// idempotent against the shared test DB (transactions.other_trx_code is UNIQUE).
	spendCode, rewardCode := uuid.NewString(), uuid.NewString()
	if _, err := s.Deduct(ctx, uid, 3000, spendCode); err != nil {
		t.Fatalf("deduct: %v", err)
	}
	if _, err := s.CreditNodeReward(ctx, uid, 250, rewardCode); err != nil {
		t.Fatalf("reward: %v", err)
	}

	txs, err := s.ListTransactions(ctx, uid, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(txs) != 2 {
		t.Fatalf("want 2 transactions, got %d", len(txs))
	}
	// Newest first: the reward was written after the spend.
	if txs[0].OtherTrxCode != rewardCode || txs[1].OtherTrxCode != spendCode {
		t.Fatalf("wrong order: %s then %s", txs[0].OtherTrxCode, txs[1].OtherTrxCode)
	}
	if txs[1].AmountMinor != 3000 {
		t.Fatalf("want spend amount 3000, got %d", txs[1].AmountMinor)
	}

	// limit is honored.
	one, _ := s.ListTransactions(ctx, uid, 1)
	if len(one) != 1 {
		t.Fatalf("limit=1 returned %d", len(one))
	}
	// Malformed id → empty, not an error.
	if got, err := s.ListTransactions(ctx, "not-a-user", 0); err != nil || len(got) != 0 {
		t.Fatalf("malformed id: want empty/no-error, got %d/%v", len(got), err)
	}
}

// TestGetAccountMalformedID: a user_id that isn't a valid UUID must read as
// "not found", not surface a raw driver error (which would 500 + leak SQLSTATE).
func TestGetAccountMalformedID(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if _, err := s.GetAccount(ctx, "not-a-user"); err != ErrAccountNotFound {
		t.Fatalf("want ErrAccountNotFound for malformed id, got %v", err)
	}
	if _, err := s.Deduct(ctx, "not-a-user", 100, uuid.NewString()); err != ErrAccountNotFound {
		t.Fatalf("Deduct: want ErrAccountNotFound for malformed id, got %v", err)
	}
	if _, err := s.TransferCredit(ctx, "not-a-user", uuid.NewString(), 100, uuid.NewString()); err != ErrAccountNotFound {
		t.Fatalf("TransferCredit: want ErrAccountNotFound for malformed id, got %v", err)
	}
}

func TestDeviceEnrollListRevoke(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid := uuid.NewString()
	k1 := make([]byte, 32)
	k2 := make([]byte, 32)
	k1[0], k2[0] = 1, 2

	if err := s.EnrollDevice(ctx, uid, k1); err != nil {
		t.Fatalf("enroll k1: %v", err)
	}
	if err := s.EnrollDevice(ctx, uid, k2); err != nil {
		t.Fatalf("enroll k2: %v", err)
	}
	if err := s.EnrollDevice(ctx, uid, k1); err != nil { // idempotent
		t.Fatalf("re-enroll k1: %v", err)
	}
	ds, _ := s.ListActiveDevices(ctx, uid)
	if len(ds) != 2 {
		t.Fatalf("want 2 active devices, got %d", len(ds))
	}

	if err := s.RevokeDevice(ctx, uid, k1); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	ds, _ = s.ListActiveDevices(ctx, uid)
	if len(ds) != 1 {
		t.Fatalf("want 1 active after revoke, got %d", len(ds))
	}

	// Malformed user id → empty, not an error.
	if got, err := s.ListActiveDevices(ctx, "not-a-user"); err != nil || len(got) != 0 {
		t.Fatalf("malformed id: want empty/no-error, got %d/%v", len(got), err)
	}
}

func TestRebindDevices(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid := uuid.NewString()
	old1 := make([]byte, 32)
	old2 := make([]byte, 32)
	newK := make([]byte, 32)
	old1[0], old2[0], newK[0] = 10, 11, 12
	_ = s.EnrollDevice(ctx, uid, old1)
	_ = s.EnrollDevice(ctx, uid, old2)

	if err := s.RebindDevices(ctx, uid, newK); err != nil {
		t.Fatalf("rebind: %v", err)
	}
	ds, _ := s.ListActiveDevices(ctx, uid)
	if len(ds) != 1 {
		t.Fatalf("want exactly 1 active device after rebind, got %d", len(ds))
	}
	if ds[0].PubKey[0] != newK[0] {
		t.Fatalf("active device is not the new key")
	}
}

func issueTestVoucher(t *testing.T, s *Store, uid string, amount int64, ttl time.Duration) string {
	t.Helper()
	id := uuid.NewString()
	nonce := []byte("nonce-0000000000")
	sig := []byte("sig")
	if err := s.IssueVoucher(context.Background(), id, uid, amount, nonce, sig, time.Now().Add(ttl)); err != nil {
		t.Fatalf("issue voucher: %v", err)
	}
	return id
}

func TestVoucherIssueReservesCredit(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid := uuid.NewString()
	_ = s.Deposit(ctx, newAccount(uid, 10000))

	issueTestVoucher(t, s, uid, 3000, time.Hour)
	a, _ := s.GetAccount(ctx, uid)
	if a.AvailableCredit != 7000 {
		t.Fatalf("issue should reserve 3000, available=%d", a.AvailableCredit)
	}
	// Over available → rejected, balance untouched.
	if err := s.IssueVoucher(ctx, uuid.NewString(), uid, 8000, []byte("n"), []byte("s"), time.Now().Add(time.Hour)); err != ErrInsufficientCredit {
		t.Fatalf("want ErrInsufficientCredit, got %v", err)
	}
	a, _ = s.GetAccount(ctx, uid)
	if a.AvailableCredit != 7000 {
		t.Fatalf("rejected issue moved credit: %d", a.AvailableCredit)
	}
}

func TestVoucherRedeemOnce(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid := uuid.NewString()
	_ = s.Deposit(ctx, newAccount(uid, 10000))
	id := issueTestVoucher(t, s, uid, 2000, time.Hour)

	v, err := s.RedeemVoucher(ctx, id, uuid.NewString())
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}
	if v.UserID != uid || v.AmountMinor != 2000 {
		t.Fatalf("bad redeemed voucher: %+v", v)
	}
	// Second redeem rejected — bounded double-spend.
	if _, err := s.RedeemVoucher(ctx, id, uuid.NewString()); err != ErrAlreadyRedeemed {
		t.Fatalf("want ErrAlreadyRedeemed, got %v", err)
	}
	// Redeem does not further deduct (reserved at issue): still 8000.
	a, _ := s.GetAccount(ctx, uid)
	if a.AvailableCredit != 8000 {
		t.Fatalf("redeem changed balance: %d", a.AvailableCredit)
	}
}

func TestVoucherExpireRefunds(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid := uuid.NewString()
	_ = s.Deposit(ctx, newAccount(uid, 10000))
	issueTestVoucher(t, s, uid, 2500, -time.Second) // already expired

	count, refunded, err := s.ExpireVouchers(ctx)
	if err != nil {
		t.Fatalf("expire: %v", err)
	}
	if count != 1 || refunded != 2500 {
		t.Fatalf("want expired 1 / refunded 2500, got %d/%d", count, refunded)
	}
	a, _ := s.GetAccount(ctx, uid)
	if a.AvailableCredit != 10000 {
		t.Fatalf("expire should refund to 10000, got %d", a.AvailableCredit)
	}
	// An expired voucher can't be redeemed.
	list, _ := s.ListVouchers(ctx, uid)
	if len(list) != 1 || list[0].Status != "expired" {
		t.Fatalf("voucher not marked expired: %+v", list)
	}
}

// TestConcurrentRedeemOnlyOneWins: many goroutines race to redeem one voucher; exactly
// one succeeds (the FOR UPDATE lock serializes them).
func TestConcurrentRedeemOnlyOneWins(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid := uuid.NewString()
	_ = s.Deposit(ctx, newAccount(uid, 10000))
	id := issueTestVoucher(t, s, uid, 1000, time.Hour)

	const goroutines = 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	var wins int
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if _, err := s.RedeemVoucher(ctx, id, uuid.NewString()); err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			} else if err != ErrAlreadyRedeemed {
				t.Errorf("unexpected redeem error: %v", err)
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("want exactly 1 successful redeem, got %d (double-spend!)", wins)
	}
}

func TestCreditNodeReward(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid := uuid.NewString()
	if err := s.Deposit(ctx, newAccount(uid, 5000)); err != nil {
		t.Fatalf("deposit: %v", err)
	}

	a, err := s.CreditNodeReward(ctx, uid, 250, uuid.NewString())
	if err != nil {
		t.Fatalf("reward: %v", err)
	}
	if a.AvailableCredit != 5250 || a.CreditLimit != 5250 {
		t.Fatalf("want 5250/5250, got %d/%d", a.AvailableCredit, a.CreditLimit)
	}
}

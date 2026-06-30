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

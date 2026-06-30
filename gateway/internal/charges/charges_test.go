package charges

import (
	"errors"
	"testing"
	"time"
)

func newStore() *Store {
	return NewStore(15*time.Minute, Merchant{ID: "mer_1", Name: "Test Store"})
}

func items() []Item {
	return []Item{{SKU: "A", Name: "Coffee", PriceMinor: 4500, Quantity: 2}}
}

func TestCreateSumsAndDefaults(t *testing.T) {
	s := newStore()
	c, err := s.Create("mer_1", items())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if c.AmountMinor != 9000 {
		t.Fatalf("want amount 9000, got %d", c.AmountMinor)
	}
	if c.Status != StatusPending {
		t.Fatalf("want pending, got %s", c.Status)
	}
	if c.QRPayload() != "kotn://pay/"+c.ID {
		t.Fatalf("bad qr payload: %s", c.QRPayload())
	}
}

func TestCreateUnknownMerchant(t *testing.T) {
	s := newStore()
	if _, err := s.Create("nope", items()); !errors.Is(err, ErrMerchantNotFound) {
		t.Fatalf("want ErrMerchantNotFound, got %v", err)
	}
}

func TestApproveLifecycleAndNoDoublePay(t *testing.T) {
	s := newStore()
	c, _ := s.Create("mer_1", items())

	if _, err := s.BeginApprove(c.ID); err != nil {
		t.Fatalf("begin approve: %v", err)
	}
	paid, err := s.MarkPaid(c.ID, "user-1", "moka-1")
	if err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if paid.Status != StatusPaid || paid.CustomerID != "user-1" || paid.MokaRef != "moka-1" {
		t.Fatalf("bad paid charge: %+v", paid)
	}

	// Second settle attempt must be rejected (no double charge).
	if _, err := s.BeginApprove(c.ID); !errors.Is(err, ErrNotPending) {
		t.Fatalf("want ErrNotPending on re-approve, got %v", err)
	}
	if _, err := s.MarkPaid(c.ID, "user-1", "moka-2"); !errors.Is(err, ErrNotPending) {
		t.Fatalf("want ErrNotPending on re-mark, got %v", err)
	}
}

func TestExpiry(t *testing.T) {
	s := NewStore(-1*time.Second, Merchant{ID: "mer_1", Name: "T"}) // already-expired TTL
	c, _ := s.Create("mer_1", items())
	got, _ := s.Get(c.ID)
	if got.Status != StatusExpired {
		t.Fatalf("want expired, got %s", got.Status)
	}
	if _, err := s.BeginApprove(c.ID); !errors.Is(err, ErrNotPending) {
		t.Fatalf("expired charge must not be approvable, got %v", err)
	}
}

func TestCancel(t *testing.T) {
	s := newStore()
	c, _ := s.Create("mer_1", items())
	got, err := s.Cancel(c.ID)
	if err != nil || got.Status != StatusCanceled {
		t.Fatalf("cancel failed: %+v %v", got, err)
	}
	if _, err := s.Cancel(c.ID); !errors.Is(err, ErrNotPending) {
		t.Fatalf("double cancel must fail, got %v", err)
	}
}

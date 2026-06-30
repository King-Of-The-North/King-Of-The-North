// Package charges is the online POS: merchant-initiated payment intents for
// e-commerce (ADR-0014). A merchant creates a charge, the customer approves it on
// their phone, and it settles against the customer's wallet. Charges are intents, not
// money — the wallet (Postgres + Moka) stays the only authoritative ledger. In-memory
// for the demo; persistence is a later upgrade.
package charges

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Status is a charge's lifecycle state.
type Status string

const (
	StatusPending  Status = "pending"
	StatusPaid     Status = "paid"
	StatusCanceled Status = "canceled"
	StatusExpired  Status = "expired"
)

// Merchant is an e-commerce store owner that accepts KOTN.
type Merchant struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Item is one line of a charge (mirrors the wallet cart item, kuruş).
type Item struct {
	SKU        string `json:"sku"`
	Name       string `json:"name"`
	PriceMinor int64  `json:"price_minor"`
	Quantity   uint32 `json:"quantity"`
}

// Charge is a merchant-initiated payment intent.
type Charge struct {
	ID          string    `json:"id"`
	MerchantID  string    `json:"merchant_id"`
	AmountMinor int64     `json:"amount_minor"`
	Items       []Item    `json:"items"`
	Status      Status    `json:"status"`
	CustomerID  string    `json:"customer_id,omitempty"`
	MokaRef     string    `json:"moka_ref,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// QRPayload is what the customer's app scans to open this charge.
func (c *Charge) QRPayload() string { return "kotn://pay/" + c.ID }

var (
	ErrMerchantNotFound = errors.New("charges: merchant not found")
	ErrChargeNotFound   = errors.New("charges: charge not found")
	ErrEmptyCart        = errors.New("charges: charge has no items")
	ErrNotPending       = errors.New("charges: charge is not pending")
)

// Store holds merchants and charges in memory.
type Store struct {
	mu        sync.RWMutex
	ttl       time.Duration
	merchants map[string]Merchant
	charges   map[string]*Charge
}

// NewStore builds a charge store with the given charge time-to-live and seeded
// merchants.
func NewStore(ttl time.Duration, merchants ...Merchant) *Store {
	s := &Store{
		ttl:       ttl,
		merchants: make(map[string]Merchant, len(merchants)),
		charges:   make(map[string]*Charge),
	}
	for _, m := range merchants {
		s.merchants[m.ID] = m
	}
	return s
}

// Merchants lists the seeded merchants.
func (s *Store) Merchants() []Merchant {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Merchant, 0, len(s.merchants))
	for _, m := range s.merchants {
		out = append(out, m)
	}
	return out
}

// Create makes a pending charge for a merchant. Amount is summed from the items
// (overflow-guarded). Returns ErrMerchantNotFound / ErrEmptyCart on bad input.
func (s *Store) Create(merchantID string, items []Item) (*Charge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.merchants[merchantID]; !ok {
		return nil, ErrMerchantNotFound
	}
	if len(items) == 0 {
		return nil, ErrEmptyCart
	}
	total, err := cartTotal(items)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	c := &Charge{
		ID:          newID(),
		MerchantID:  merchantID,
		AmountMinor: total,
		Items:       items,
		Status:      StatusPending,
		CreatedAt:   now,
		ExpiresAt:   now.Add(s.ttl),
	}
	s.charges[c.ID] = c
	return c, nil
}

// Get returns a copy of a charge, lazily expiring a stale pending one.
func (s *Store) Get(id string) (Charge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.charges[id]
	if !ok {
		return Charge{}, ErrChargeNotFound
	}
	s.expireLocked(c)
	return *c, nil
}

// ChargesForMerchant lists a merchant's charges (newest-agnostic order).
func (s *Store) ChargesForMerchant(merchantID string) []Charge {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Charge
	for _, c := range s.charges {
		if c.MerchantID == merchantID {
			s.expireLocked(c)
			out = append(out, *c)
		}
	}
	return out
}

// BeginApprove validates that a charge can be paid and returns its items + amount for
// the caller to settle against the wallet. It does NOT mutate the charge — the caller
// settles, then calls MarkPaid (or discards on decline). Returns ErrNotPending if the
// charge is paid/canceled/expired.
func (s *Store) BeginApprove(id string) (Charge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.charges[id]
	if !ok {
		return Charge{}, ErrChargeNotFound
	}
	s.expireLocked(c)
	if c.Status != StatusPending {
		return Charge{}, fmt.Errorf("%w (status %s)", ErrNotPending, c.Status)
	}
	return *c, nil
}

// MarkPaid finalizes a charge after the wallet settled it. Idempotency is guarded by
// the pending check: a second MarkPaid returns ErrNotPending.
func (s *Store) MarkPaid(id, customerID, mokaRef string) (Charge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.charges[id]
	if !ok {
		return Charge{}, ErrChargeNotFound
	}
	if c.Status != StatusPending {
		return Charge{}, fmt.Errorf("%w (status %s)", ErrNotPending, c.Status)
	}
	c.Status = StatusPaid
	c.CustomerID = customerID
	c.MokaRef = mokaRef
	return *c, nil
}

// Cancel voids a pending charge.
func (s *Store) Cancel(id string) (Charge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.charges[id]
	if !ok {
		return Charge{}, ErrChargeNotFound
	}
	if c.Status != StatusPending {
		return Charge{}, fmt.Errorf("%w (status %s)", ErrNotPending, c.Status)
	}
	c.Status = StatusCanceled
	return *c, nil
}

// expireLocked flips a pending charge to expired once past its TTL. Caller holds mu.
func (s *Store) expireLocked(c *Charge) {
	if c.Status == StatusPending && time.Now().UTC().After(c.ExpiresAt) {
		c.Status = StatusExpired
	}
}

func newID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// cartTotal sums items in minor units with overflow guards (mirrors the wallet's cart
// validation so a charge can never carry a bogus total).
func cartTotal(items []Item) (int64, error) {
	var total int64
	for _, it := range items {
		if it.PriceMinor < 0 {
			return 0, errors.New("charges: negative price")
		}
		if it.Quantity == 0 {
			return 0, errors.New("charges: zero quantity")
		}
		line := it.PriceMinor * int64(it.Quantity)
		if it.PriceMinor != 0 && line/int64(it.Quantity) != it.PriceMinor {
			return 0, errors.New("charges: line total overflow")
		}
		newTotal := total + line
		if newTotal < total {
			return 0, errors.New("charges: cart total overflow")
		}
		total = newTotal
	}
	if total <= 0 {
		return 0, errors.New("charges: cart total must be positive")
	}
	return total, nil
}

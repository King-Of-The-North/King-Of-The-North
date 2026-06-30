// Package moka is the single owner of the Moka United integration, kept behind an
// interface so the mock runs now and the real sandbox client swaps in once dealer
// credentials land (ADR-0002).
package moka

import "context"

// SettleRequest / SettleResult model the Moka PaymentDealer settle call. Amounts are
// integer minor units (kuruş, ADR-0003).
type SettleRequest struct {
	OtherTrxCode string
	AmountMinor  int64
}

type SettleResult struct {
	PaymentID    string
	IsSuccessful bool
}

// Client is the Moka surface the Wallet service depends on.
type Client interface {
	Settle(ctx context.Context, req SettleRequest) (SettleResult, error)
}

// Mock returns deterministic success — used until real credentials are configured.
type Mock struct{}

func (Mock) Settle(_ context.Context, req SettleRequest) (SettleResult, error) {
	return SettleResult{PaymentID: "mock-" + req.OtherTrxCode, IsSuccessful: true}, nil
}

// compile-time check
var _ Client = Mock{}

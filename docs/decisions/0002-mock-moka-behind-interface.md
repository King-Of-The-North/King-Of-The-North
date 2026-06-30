# ADR-0002: Mock Moka United behind an interface, wire real sandbox later

**Status:** Accepted — 2026-06-30
**Context:** Moka sandbox credentials are open question #3 — may not arrive before
Jul 3. Wallet build cannot block on them.

## Decision
Define a `MokaClient` Go interface. Ship `MockMokaClient` now (deterministic
success). Swap in `RealMokaClient` once sandbox `DealerCode/Username/Password` are
confirmed.

```go
type MokaClient interface {
    Settle(ctx, SettleRequest) (SettleResult, error)
    GetTrxDetail(ctx, paymentID, otherTrxCode string) (TrxDetail, error)
}
```

## Why
- Decouples wallet progress from external credential availability.
- The interface matches the real Moka contract (CheckKey auth,
  `GetDealerPaymentTrxDetailList`), so the swap is config-only, no logic change.

## Consequences
- `CheckKey = SHA256(DealerCode + "MK" + Username + "PD" + Password)` is
  implemented and unit-tested against Moka's spec even while mocking.
- Demo can run end-to-end with the mock if real creds never land.
- A `MOKA_MODE=mock|real` env flag selects the implementation.

# Day-Zero Yield Wallet Service — Build Plan

**Service:** `wallet/` (Go) — financial brain & ledger
**Roadmap slot:** Days 1–2 of the 9-day window (Jul 3–12)
**Source of record:** NotebookLM "King Of the North" (conv `5318793a`), ARCHITECTURE.md §1.3, §3.3
**Status:** PLANNED — ready to build

---

## 0. Scope

Wallet owns: Day-Zero limit calculation, the 1:1 tokenized fiat ledger, atomic
deductions, and the single server-to-server Moka United integration.

Hard boundary (does NOT do): image processing, biometric inference, network
routing, auth/rate-limiting (that's the Gateway).

Locked decisions (see `docs/decisions/`):
- **ADR-0001** — Fixed Interest Pool only; equity/stock pool removed from scope.
- **ADR-0002** — Mock Moka behind an interface now; wire real sandbox when creds land.
- **ADR-0003** — Money stored as integer minor units (kuruş, `int64` / `BIGINT`).

---

## 1. Day-Zero Math

```
FV = D · (1 + r/n)^(n·t)
Yp = D · [ (1 + r/n)^(n·t) − 1 ]
L0 = Yp · (1 − m)
```

| Param | Meaning | Demo value (fixed pool) |
|-------|---------|-------------------------|
| `D`   | deposit | user input |
| `r`   | expected APY | 0.12 |
| `n`   | compounding freq / yr | 12 |
| `t`   | lock-up years | 1 |
| `m`   | risk margin | 0.10–0.15 (fixed pool) |

**Worked example:** D=10,000 TRY, r=0.12, n=12, t=1, m=0.10
→ Yp = 10,000·[(1.01)^12 − 1] = 1,268.25 → **L0 = 1,141.42 TRY**.

**Numeric handling:** `D` enters as `int64` kuruş. The compound-interest factor
`(1+r/n)^(nt)` is computed in `float64` (or `shopspring/decimal`), applied to `D`,
then the final `Yp` and `L0` are **rounded down (floor) to whole kuruş** before
storage. Floor, not round — never grant more credit than the math yields.

---

## 2. Ledger Model (PostgreSQL)

All money columns are `BIGINT` kuruş.

### `accounts`
| Column | Type | Note |
|--------|------|------|
| `user_id` | UUID PK | |
| `principal_balance` | BIGINT | locked deposit, 1:1 tokenized fiat |
| `projected_yield` | BIGINT | Yp at deposit time |
| `credit_limit` | BIGINT | L0 |
| `available_credit` | BIGINT | L0 − sum(spent) |
| `ltv_ratio` | NUMERIC(6,4) | outstanding spend / accrued yield |
| `lockup_end_date` | TIMESTAMPTZ | extendable (Yield Amortization) |
| `pool_type` | TEXT | `fixed` (only value for demo) |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

### `transactions` (maps to Moka structure so settlement reconciles)
| Column | Type | Note |
|--------|------|------|
| `id` | UUID PK | |
| `user_id` | UUID FK | |
| `other_trx_code` | TEXT UNIQUE | OUR internal id sent to Moka |
| `moka_payment_id` | TEXT NULL | Moka's id, filled on settlement |
| `amount` | BIGINT | kuruş |
| `payment_status` | SMALLINT | 0=Standby 1=Pre-Provision 2=Payment 3=Cancel 4=Full-Refund |
| `trx_status` | SMALLINT | 0=Standby 1=Successful 2=Failed |
| `created_at` | TIMESTAMPTZ | |

### Atomic deduction
On `ValidateTransaction` (from AI service via Gateway):
```
BEGIN;
  SELECT available_credit FROM accounts WHERE user_id=$1 FOR UPDATE;
  -- if amount > available_credit -> ROLLBACK, return DECLINED
  UPDATE accounts SET available_credit = available_credit - $amount,
                      updated_at = now() WHERE user_id=$1;
  INSERT INTO transactions(... payment_status=2, trx_status=1 ...);
COMMIT;
```
`SELECT ... FOR UPDATE` row lock prevents double-spend under concurrent gate reads.

---

## 3. Moka United Integration (behind interface — ADR-0002)

```go
type MokaClient interface {
    Settle(ctx, req SettleRequest) (SettleResult, error)
    GetTrxDetail(ctx, paymentID, otherTrxCode string) (TrxDetail, error)
}
```

- **Auth:** every call carries `PaymentDealerAuthentication{DealerCode, Username,
  Password, CheckKey}`.
  `CheckKey = SHA256(DealerCode + "MK" + Username + "PD" + Password)`.
- **Settle/verify:** `POST /PaymentDealer/GetDealerPaymentTrxDetailList` with
  `PaymentDealerRequest{PaymentId | OtherTrxCode}`. Read `Data.IsSuccessful`,
  then `PaymentDetail.PaymentStatus`.
- `MockMokaClient` returns deterministic success now. `RealMokaClient` swapped in
  once sandbox `DealerCode/Username/Password` confirmed (open question #3).

---

## 4. gRPC Surface (`proto/`)

```proto
service Wallet {
  rpc CalculateLimit(Deposit) returns (LimitResult);     // Gateway -> Wallet
  rpc ValidateTransaction(TrxRequest) returns (TrxResult);// AI -> Wallet (via GW)
}
```
- `CalculateLimit(Deposit{user_id, amount, apy, compounding, lockup_years, margin})`
  → `LimitResult{credit_limit, projected_yield, lockup_end_date}`.
- `ValidateTransaction(TrxRequest{user_id, amount, other_trx_code})`
  → `TrxResult{approved, remaining_credit, moka_payment_id}`.

Proto is the contract for ALL services — define it first.

---

## 5. Yield Amortization Extension (design-only for demo)

Lock-up `t` is "time-to-repay", not a hard date. If realized interest < projected
(rates move), extend `lockup_end_date` until yield repays spent credit — principal
never touched. Equity pool and Defensive Rebalancing are **removed from scope**
(ADR-0001) — interest-only project. Amortization Extension is documented for the
pitch, not coded for the demo.

---

## 6. Build Order (Days 1–2)

1. `proto/wallet.proto` + generate Go stubs.
2. PostgreSQL schema + migrations (`accounts`, `transactions`).
3. Day-Zero calc pkg — pure function, unit-tested against the worked example.
4. Ledger repo — deposit (mint 1:1), atomic deduct (`FOR UPDATE`).
5. `MokaClient` interface + `MockMokaClient`.
6. gRPC server wiring `CalculateLimit` + `ValidateTransaction`.
7. Integration test: deposit → L0 granted → deduct ≤ L0 → settle (mock) → receipt.

## 7. Real vs Mock

| Real | Mock |
|------|------|
| L0 calc engine | Card issuing lifecycle |
| PostgreSQL ledger + atomic deduct | NVİ KYC ("Speedy KYC") |
| Moka settle call (mock client, real interface) | Bank off-ramp |
| gRPC CalculateLimit + ValidateTransaction | (equity pool / trading: removed, ADR-0001) |

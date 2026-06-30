# ADR-0003: Store money as integer minor units (kuruş)

**Status:** Accepted — 2026-06-30

## Decision
All monetary values are stored and computed as integer **kuruş** (1 TRY = 100
kuruş). Go `int64`, PostgreSQL `BIGINT`.

## Why
- No floating-point drift on balances and deductions — exact arithmetic, the
  standard for financial ledgers.
- `int64` max ≈ 9.2×10^18 kuruş = 9.2×10^16 TRY — far beyond any demo need.

## Consequences
- The compound-interest factor `(1+r/n)^(nt)` is irrational; compute it in
  `float64` (or `shopspring/decimal`), multiply by `D`, then **floor to whole
  kuruş** before storage. Floor — never grant more credit than the math yields.
- API/UI boundary converts kuruş ↔ display TRY (divide/multiply by 100).
- Rounding rule lives in one place (the calc pkg) and is unit-tested against the
  worked example (D=10,000 TRY → L0 = 1,141.42 TRY = 114142 kuruş).

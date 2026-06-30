# ADR-0001: Fixed Interest Pool only — equity/stock pool removed from project

**Status:** Accepted — 2026-06-30 (revised: equity pool removed, not deferred)
**Context:** 9-day hackathon window (Jul 3–12).

## Decision
The project uses **only the Fixed Interest Pool** (risk margin `m` = 10–15%).
The AI-managed **equity/stock pool is removed from project scope entirely** —
not deferred, not pitched as future work in the core narrative. Yield comes from
interest only (T-Bill / stablecoin-style fixed rate).

## Why
- **Risk:** equity exposure means realized yield can go negative vs spent credit —
  exactly the failure mode that threatens the principal-protection promise.
  Removing it removes the project's biggest financial risk.
- Fixed pool needs no LTV rebalancing engine, no price feed, no broker integration
  → fastest path to the critical demo loop (deposit → L0 → walk out → settle →
  receipt) and a cleaner regulatory story.

## Consequences
- `pool_type` column holds only `fixed`. (Kept for schema stability; no other value.)
- Risk margin `m` is always in the 10–15% band.
- Defensive Rebalancing and live-trading simulation are **not built**.
- Yield Amortization Extension still applies (interest can underperform a
  projection if rates move) but is design-only for the demo — see plan §5.

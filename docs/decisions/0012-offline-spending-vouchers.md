# ADR-0012: Offline spending vouchers — take the server out of the per-payment path

**Status:** Accepted (design-only for the demo) — 2026-06-30
**Context:** Today every scan-and-go payment is a server round-trip + an authoritative
write (`ValidateTransaction` → `FOR UPDATE` deduct). That keeps the server in the hot
path of *every* purchase — the largest remaining per-transaction cost, and it requires
connectivity at the till. We want a lever to reduce server writes and enable offline
payments, **without** breaking the no-double-spend invariant.

## Decision
Introduce **capped, server-signed spending vouchers**. The server signs a small
spending allowance once; the phone authorizes individual purchases **locally** against
it and reconciles later.

```
1. Server signs a voucher: {user_id, cap (small), expiry (short), nonce}  ← one write, one signature
2. Phone authorizes N small purchases locally against the remaining cap   ← zero server round-trips
3. Phone batches signed receipts and syncs back
4. Server reconciles once, settles the batch with Moka, issues the next voucher
```

This converts **N authoritative writes per user → ~1 write per voucher window**, and
enables offline pay (dead zones, subway, flaky till networks).

**This is a deliberate risk/cost tradeoff, not a free win**, so it is bounded:
- **Hard per-voucher cap** (small, e.g. a few hundred kuruş) → bounded worst-case loss.
- **Short expiry** → small exposure window.
- **Device key signs each local spend** → every spend is attributable and revocable.
- **Server reconciles on sync** → detects overspend, revokes the offending device.

## Why
- The irreducible safety core is the money-write + double-spend serialization; it
  cannot be fully decentralized (AGENTS.md: "Don't shard money"). Vouchers don't move
  that core off-server — they make it **infrequent** by pre-authorizing a bounded
  budget, trading a small, capped double-spend risk for a large write reduction.
- Offline capability is a real product win for a retail payment app.

## Consequences
- Adds a **double-spend risk within the voucher window** (a tampered phone spending
  the same voucher twice before sync). Acceptable only because cap × expiry bounds the
  loss and reconciliation + revocation catch it.
- Requires: voucher issuance/signing (server), local voucher accounting (phone),
  batch reconciliation + settlement, and device revocation. None built.
- **Out of scope for the hackathon demo** — the online `ValidateTransaction` path
  (already built + tested) is the demo path. Vouchers are recorded here for the pitch
  and post-demo work. Revisit caps/expiry with risk + legal before any real use.

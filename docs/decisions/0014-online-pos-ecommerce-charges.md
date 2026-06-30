# ADR-0014: Online POS — merchant-initiated charges for e-commerce

**Status:** Accepted — 2026-07-01
**Context:** Physical stores use customer-initiated phone scan-and-go (ADR-0007). But
an **e-commerce store owner** wants to accept KOTN as a payment method on their online
checkout — a "Pay with KOTN" button. That flow is **merchant-initiated**: the store
creates a charge, the customer approves it on their phone. The existing
`ValidateTransaction` (customer's app sends its own cart) does not cover this; we need
a charge/payment-intent lifecycle and a merchant concept.

## Decision
Add an **online POS** to the Gateway: a merchant creates a charge, the customer
approves it from their phone (face-pay), and the charge settles against the customer's
wallet — exactly like a phone-scan version of a card-payment intent.

**Entities (Gateway, in-memory for the demo):**
- **Merchant** `{id, name}` — seeded e-commerce store owners.
- **Charge** `{id, merchant_id, amount_minor, items[], status, customer_id, moka_ref,
  created_at, expires_at}`. Status: `pending → paid` (or `canceled` / `expired`).

**Flow (the locked option):**
```
1. Merchant: POST /v1/charges {merchant_id, items}
       → charge created (pending) + qr_payload "kotn://pay/<charge_id>"
2. Customer app scans the QR → GET /v1/charges/<id> (shows amount + merchant)
3. Customer approves (on-device face match):
       POST /v1/charges/<id>/approve {user_id}
       → wallet.ValidateTransaction(user_id, charge.items, other_trx_code=charge_id)
       → on approve: append signed ledger entry, status=paid, store moka_ref + customer
4. Merchant polls GET /v1/charges/<id> → sees "paid"
```

Money path is unchanged and authoritative: the customer's wallet is the only thing
deducted (Postgres atomic `FOR UPDATE`), the charge is just the **intent** that drives
one `ValidateTransaction`. The charge_id is the `other_trx_code`, so a charge maps 1:1
to a ledger/Moka transaction and can't be double-settled.

## Why
- Merchant-initiated is the correct shape for e-commerce: the store quotes the price,
  the customer authorizes. A customer-initiated cart (ADR-0007) doesn't model "the
  store is asking me to pay X".
- Reusing `ValidateTransaction` + the signed ledger keeps one money path — the online
  POS adds an intent layer, not a second ledger. No new way to move money, so the
  safety invariants (ADR-0003/0005) still hold.
- charge_id as `other_trx_code` gives idempotency and reconciliation for free.

## Consequences
- New Gateway state: an in-memory charge store + seeded merchants (demo scale). Charges
  are intents, not money, so losing them on restart is acceptable for the demo;
  persistence (Postgres) is a later upgrade.
- Web frontend gets a **merchant app**: login, dashboard, a POS "create charge → show
  QR → live status" screen, plus the admin ledger / cloud-cost-avoided views.
- **Merchant settlement** (paying the store its money) is out of scope for the demo —
  the customer-side deduction + "paid" status is what we show; real merchant payout via
  Moka is post-hackathon.
- A charge has a short expiry; approving an expired/paid/canceled charge is rejected.
- Web auth for merchants/admins: email+password mock session (seeded accounts) for the
  demo; per-merchant API keys / real auth provider is a later decision.

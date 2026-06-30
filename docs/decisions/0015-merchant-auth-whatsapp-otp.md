# ADR-0015: Merchant web auth = WhatsApp OTP (Twilio Verify), mocked behind an interface

**Status:** Accepted — 2026-07-01
**Context:** The Next.js merchant app (store catalog, admin dashboard, online POS —
ADR-0014) needs to authenticate store owners. We don't want passwords, and a real auth
provider is heavier than the demo needs. Store owners already have WhatsApp, so a
one-time code over WhatsApp is low-friction and on-brand for a Turkish retail product.

## Decision
Merchant login is **phone-number + WhatsApp one-time code**, delivered via **Twilio
Verify** (channel = whatsapp). Following ADR-0002, the verification provider sits
**behind an interface and is mocked** until Twilio credentials land; the real client
swaps in with no call-site changes.

**Flow (server-side in the Next.js app):**
```
1. POST /api/auth/start  {phone}            → Verifier.Start(phone)  (sends WhatsApp code)
2. POST /api/auth/verify {phone, code}      → Verifier.Check(phone, code)
       → on approve: map phone → merchant (Gateway merchant list) → set signed session cookie {merchant_id}
3. Protected pages read the session cookie; unauthenticated → redirect to login.
```

- **Verifier interface:** `Start(phone)` and `Check(phone, code) (approved, err)`.
- **MockVerifier (now):** accepts a fixed dev code (and logs it); no network. Lets the
  whole flow run without Twilio creds.
- **TwilioVerifier (later):** Twilio Verify API, gated on `TWILIO_ACCOUNT_SID`,
  `TWILIO_AUTH_TOKEN`, `TWILIO_VERIFY_SERVICE_SID`. Selected at runtime when those are set.
- **Merchant identity:** Gateway merchants carry a `phone`; a verified phone maps to a
  `merchant_id`. Session is a signed cookie (JWT) holding `merchant_id`.

## Why
- WhatsApp OTP is passwordless, low-friction, and realistic for the target users; no
  password storage to secure.
- Mirroring the Moka mock-behind-interface pattern (ADR-0002) keeps the demo runnable
  today and the swap to real Twilio a one-line provider change.
- Keeping auth in the Next.js server layer keeps the Gateway as the clean payment API;
  the Gateway only needs to expose merchant phones for the lookup.

## Consequences
- Gateway `Merchant` gains a `phone` field; demo merchants are seeded with phones.
- The Next.js app owns: the Verifier interface + Mock/Twilio impls, the
  `/api/auth/*` route handlers, the signed session cookie, and route protection.
- **Demo-only:** the mock code path is not real auth — it accepts the dev code. Real
  Twilio Verify + rate limiting + a proper secret for cookie signing are required
  before any real use.
- Admin (operator) login can reuse the same OTP path against an admin phone, or a
  separate allowlist — decided when the admin views are built.

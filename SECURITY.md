# Security Policy

King Of The North (KOTN) is a CeDeFi retail payment system built for the MOKA
UNITED Hackathon. It handles money and (on-device) biometrics, so security is a
first-class design concern. This document explains the security model, the trust
boundaries, and how to report a vulnerability.

> Status: hackathon / pre-production. Several controls below are **design-complete
> but not yet coded**, and several providers are **mocked behind interfaces** until
> real credentials land. Those are called out explicitly. Do not deploy this to
> production or process real customer funds without completing the items in
> [Pre-production requirements](#pre-production-requirements).

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security vulnerabilities.

- Email: **yusufmirza145@gmail.com** with subject `KOTN SECURITY`.
- Or use GitHub's private vulnerability reporting on this repository.

Include: affected component (`gateway/`, `wallet/`, `frontend/`, `mobile/`,
`proto/`), a description, reproduction steps, and impact. We aim to acknowledge
within a few days during the hackathon window.

Please act in good faith: do not access or modify other users' data, do not run
denial-of-service or spam against shared infrastructure, and give us reasonable
time to remediate before any disclosure.

## Design security model

The security posture is defined by the Architecture Decision Records in
`docs/decisions/`. When older prose (`README.md`, `ARCHITECTURE.md`) disagrees with
an ADR, **the ADR wins**.

### Money custody and integrity

- **Custody of real funds stays in Moka United** (licensed e-money). Money is
  **never** held on phones or in the P2P layer. (ADR-0005)
- **Authoritative balances live in PostgreSQL** as integer minor units (kuruş,
  `BIGINT`) — never floats, no floating-point drift on balances. (ADR-0003)
- The **P2P network replicates a signed, append-only, hash-chained audit log
  only** — transaction records + signatures. Nodes verify and replicate; they
  **never custody balances**. A lost node causes zero loss (full replication).
  (ADR-0005)
- Safety invariant: **don't shard money.** The P2P layer replicates an audit log;
  it does not hold authoritative truth.
- The double-spend serialization core (money-write under `FOR UPDATE`) is the
  irreducible safety point and stays server-side. (ADR-0012)

### Biometrics and authentication

- Authentication is **on-device OpenCV face matching**. The face template is
  generated and matched **on the phone** and **never leaves it**; the server stores
  **nothing** biometric. Templates live in device secure storage
  (`expo-secure-store` / Secure Enclave / StrongBox-TEE). (ADR-0006)
- **No biometric data — raw, hashed, embedded, encrypted, or sharded — is ever
  stored in or replicated through the P2P layer or any off-device store.** A
  biometric "hash" is a fuzzy, invertible, irrevocable template, not a password
  hash; it is treated as special-category data (GDPR Art. 9 / KVKK). (ADR-0010)
- What identifies a user across the system is a **cryptographic key** (passkey /
  device key in the secure enclave), not any representation of their face. Only the
  key and the signatures it produces ever leave the device. (ADR-0006, ADR-0010)

### Account recovery and revocation

- Money and identity are anchored to a server-side `user_id`, **not** to a device.
  Losing a phone loses only a **key**, which is re-established — never money.
  (ADR-0011)
- Device keys are **revocable**: a lost-phone report marks the device key revoked,
  and any transaction signed by it is rejected at the Gateway. Revocability is
  exactly why a key — not the irrevocable biometric — is the credential of record.
  (ADR-0011)
- Recovery re-proves identity (KYC/NVİ, OTP, optional backup codes), then binds a
  fresh device key to the existing `user_id`. **Not yet coded**; KYC is mocked.

### Merchant web authentication

- Merchant login is **passwordless**: phone number + WhatsApp one-time code via
  Twilio Verify, behind a `Verifier` interface. No password storage. Session is a
  signed cookie (JWT) holding `merchant_id`. (ADR-0015)
- The web app is **for commerce owners only**. Regular users transact on the mobile
  app; the website never holds consumer balances or biometrics. (ADR-0016)

### Payment links / strong customer authentication

- Payment links (`/pay/<charge_id>`) expose an existing charge over a **public**
  route. The customer-facing checkout only **initiates and displays**; the actual
  approval is an **on-device face match in the mobile app** — the 3D-Secure-style
  strong customer authentication (SCA) challenge. The money path is unchanged: one
  `settle()` (wallet deduct + signed ledger). (ADR-0016)

## Known demo-only / mocked controls

These are intentional shortcuts for the hackathon. They are **not** real security
and must be replaced before any real use:

- **Moka is mocked** (`MockMokaClient`, deterministic success) behind the
  `MokaClient` interface; `MOKA_MODE=mock|real` selects it. (ADR-0002)
- **Merchant OTP is mocked** (`MockVerifier` accepts a fixed dev code and logs it,
  no network). Real Twilio Verify + rate limiting + a proper cookie-signing secret
  are required for real use. (ADR-0015)
- **KYC / account recovery** flows are design-complete but mocked / not coded.
  (ADR-0011)
- **Offline spending vouchers** (ADR-0012) are design-only and out of scope for the
  demo; the online `ValidateTransaction` path is the demo path. Vouchers introduce a
  bounded double-spend window (cap × expiry) and must be revisited with risk + legal
  before real use.
- The payment-link checkout has a labelled **"approve as customer" stand-in** until
  the mobile app exists. (ADR-0016)

## Pre-production requirements

Before processing real funds or real customer data:

1. Swap all mocked providers for real ones: `RealMokaClient` (ADR-0002),
   `TwilioVerifier` with rate limiting (ADR-0015).
2. Set a strong secret for session-cookie (JWT) signing; do not ship the demo secret.
3. Implement the server-side **device-key registry + Gateway revocation** and the
   **KYC/OTP/backup-code recovery** flow. (ADR-0011)
4. Re-evaluate offline voucher caps/expiry with risk + legal before enabling them.
   (ADR-0012)
5. Configure real origins/secrets via environment (`NEXT_PUBLIC_WEB_URL`, Moka
   `DealerCode/Username/Password`, `TWILIO_*`) — never commit credentials.
6. Complete a data-protection review (KVKK / GDPR) confirming no biometric data
   leaves the device and the audit-log replication carries no personal special-
   category data. (ADR-0006, ADR-0010)

## Supported components

| Component | Path | Notes |
|-----------|------|-------|
| Gateway | `gateway/` | REST ingress, gRPC routing, P2P replicated-ledger nodes, key revocation |
| Wallet | `wallet/` | Ledger, Moka integration (single owner), authoritative balances |
| Frontend | `frontend/` | Merchant catalog + admin dashboard + payment links (Next.js) |
| Mobile | `mobile/` | Consumer app: on-device face auth, scan-and-go (Expo) |
| Proto | `proto/` | gRPC contract — cross-service changes start here |

Report against the specific component where possible.

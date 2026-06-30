# ADR-0011: Account recovery and device rebinding (lost / new phone)

**Status:** Accepted — 2026-06-30
**Context:** Auth is on-device (ADR-0006) and biometric never leaves the phone
(ADR-0010). That raises the obvious question: if the user loses their phone or buys a
new one, how do they get back in without their money or identity being at risk?

## Decision
The user is anchored to a server-side `user_id`, **not** to any device. Losing a
device never loses money or identity — only a **key** that gets re-established.

**What is lost vs not:**
- **Money / balance** — lives in Postgres + Moka, keyed to `user_id`. Never lost
  (ADR-0005).
- **Biometric template** — device-local, disposable. Not recovered — **re-enrolled**
  (capture face fresh on the new device → new local template). Same face, new template.
- **Device key** (passkey) — re-established on the new device.

**Two recovery paths:**
1. **Passkey sync (upgrade case):** key restored by the OS (iCloud Keychain / Google
   Password Manager) → user re-enrolls face locally → enclave rebinds the template to
   the restored key.
2. **Identity recovery (lost / no sync):** user re-proves identity (KYC/NVİ, OTP to
   registered phone/email, optional backup recovery codes) → server issues a new
   device-enrollment token → new phone generates a fresh key + re-enrolls face →
   server binds the new key to the existing `user_id`.

**Revocation:** on a lost-phone report the server marks that device key **revoked**;
any transaction signed by it is rejected at the Gateway. Keys are revocable precisely
because they are not the (irrevocable) biometric.

## Why
- Binding money/identity to `user_id` (not the device) makes recovery a key-rebind,
  not a balance-restore — the dangerous part (money) is never in play.
- Re-enrolling the biometric is safe and cheap: templates are local and disposable;
  the face is always available on the user.
- A revocable key is what makes a stolen phone safe to disown — you cannot revoke a
  face, which is another reason biometric is never the credential of record
  (ADR-0010).

## Consequences
- Requires a server-side **device-key registry** (per `user_id`: keys, status,
  enrolled_at) and **revocation** at the Gateway — auth-layer work, not yet built.
- Requires a **recovery flow** (KYC/OTP/backup codes). KYC is mocked for the demo
  (ADR-0002 pattern); recovery is design-complete but not coded.
- `accounts.user_id` (already built in the wallet ledger) is the anchor — no schema
  change needed there for recovery.

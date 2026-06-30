# ADR-0010: No biometric data in the P2P layer — not even hashed or sharded

**Status:** Accepted — 2026-06-30
**Context:** A proposal resurfaced to keep auth "decentralized + autonomous" by
converting the face/fingerprint to a hash, splitting it into pieces across a P2P
store (IPFS-style) with a metahash to reassemble. This is the FHE biometric-sharding
design ADR-0006 already superseded; it keeps coming up because "store a hash, not an
image" sounds safe. It is not. This ADR records the rejection and the reasoning so it
does not return mid-build.

## Decision
**No biometric data — raw, hashed, embedded, encrypted, or sharded — is ever stored
in or replicated through the P2P layer (or any off-device store).** Biometric
enrollment and matching happen **on-device only** (ADR-0006). The P2P layer carries
**only the signed audit log** (transaction records + signatures), which contains no
biometric data.

What identifies a user across the system is a **cryptographic key** (passkey / device
key in the secure enclave), not any representation of their face or fingerprint.

## Why
- **A biometric "hash" is not a password hash.** Faces are never captured
  identically, so exact-match crypto hashes are useless; biometric systems store a
  **fuzzy template / embedding** matched by distance threshold. That template is (a)
  directly **matchable** to impersonate the user and (b) **invertible** — face images
  can be reconstructed from embeddings. It is biometric data regardless of the word
  "hash."
- **Sharding distributes a secret; it does not anonymize it.** If shards + metahash
  can reconstruct the template (they must, or matching fails), anyone who gathers the
  shards can too. More replicas = larger attack surface, not smaller.
- **Biometrics are irrevocable.** A leaked password is reset; a leaked face is
  compromised for life. Putting irrevocable data on an immutable, globally-replicated
  network is the worst possible pairing.
- **It is illegal for this use.** Biometrics are special-category data (GDPR Art. 9 /
  KVKK); distributing them across a public P2P network with no deletion path is a
  severe violation and would kill a bank partnership (Moka).

## Consequences
- Cross-device / "autonomous" goals are met by **passkey/key-pair sync** (ADR-0011),
  not by moving biometric data. The face unlocks a local key; only the **key** (and
  the signatures it produces) ever leaves the device.
- The P2P/IPFS-style layer stores the **signed audit log only** — confirmed safe to
  replicate (ADR-0005, ADR-0008).
- Any future "decentralize the biometric" proposal is **out of scope by decision**;
  reopening requires a new ADR superseding this one.

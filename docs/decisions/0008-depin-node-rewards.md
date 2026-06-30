# ADR-0008: DePIN — client phones run nodes and earn credit for it

**Status:** Accepted — 2026-06-30
**Context:** The CeDeFi P2P layer (ADR-0005) can do more than decentralize the
ledger: if **clients run the nodes**, they become the infrastructure, cutting Moka
United's cloud spend. This is the project's headline economic engine.

## Decision
Each client phone **runs a lightweight P2P node** that replicates the signed ledger
and serves reads/verifications. Users **earn credit** for the resources they
contribute (uptime, storage, bandwidth, validations) — a DePIN (Decentralized
Physical Infrastructure) model, like Helium: the crowd is the infrastructure and is
paid for it. Earned credit tops up the user's wallet / Day-Zero spendable limit.

- **Metering:** the Gateway tracks per-node uptime, bytes replicated, requests
  served, validations → periodic **contribution proof**.
- **Reward:** Gateway → Wallet `CreditNodeReward(user_id, contribution_proof)`
  converts the proof into wallet credit (authoritative in Postgres, integer kuruş).
  Rewards are a normal ledger entry — **never minted on the phone**.

## Why
- Every phone-node is storage/compute/bandwidth Moka does **not** rent from a cloud
  provider → direct **OPEX reduction** + stickier users.
- Turns the P2P requirement from a cost into the product's differentiator.

## Consequences
- The Expo app hosts a real node (WebSocket to the Gateway registry); demo scale =
  a few real phone nodes + simulated Go nodes + the always-on anchor (ADR-0004).
- Admin dashboard shows a "cloud cost avoided" counter for the pitch.
- **Regulatory framing (demo-only, flag):** present rewards as **loyalty/cashback
  credit for infrastructure contribution**, NOT investment return, to avoid the
  e-money/securities-yield interpretation. Real treatment = legal review post-hackathon.

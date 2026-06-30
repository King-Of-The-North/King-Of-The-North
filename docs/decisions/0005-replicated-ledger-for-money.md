# ADR-0005: P2P money ledger uses replication, not erasure-coded sharding

**Status:** Accepted — 2026-06-30
**Context:** CeDeFi needs a P2P ("De") layer. The original biometric design used
Shamir's Secret Sharing (split-with-loss) — correct for privacy, wrong for money.

## Decision
The P2P ledger that carries **money/transaction state** uses **full-copy
replication + consensus**, not erasure-coded sharding.

- **Custody** of real fiat and the yield pool stays in **Moka United** (licensed
  e-money). Money is **never** held on phones.
- **Authoritative balances** stay in **Postgres** (integer kuruş, ADR-0003).
- The **P2P network holds a replicated, append-only, hash-chained signed
  transaction log** — every node keeps the **full** copy. Nodes **verify and
  replicate**; they never own balances.
- A never-leaving **anchor node** (reuse ADR-0004's anchor) guarantees finality so
  the demo never loses state.

## Why
- Erasure coding is right when loss is acceptable and you want no node to hold the
  whole thing (biometrics). For money it is unsafe: **lost quorum = lost funds**.
- Real DeFi/blockchain replicates full state across all nodes — that is how you get
  a decentralized, verifiable ledger **without** anyone being able to lose the money.
- Splitting authoritative truth across central (Moka + Postgres) and a replicated
  P2P audit log gives the CeDeFi story while keeping funds safe by construction.

## Consequences
- Tx entry shape: `{ prev_hash, user_id, amount, items, moka_ref, sig }`, hash-chained.
- **Lost node = zero loss** (full replication); demo can kill a non-anchor node live.
- The Shamir-shard "serverless biometric DB" purpose is retired (see ADR-0006).
- Node transport stays swappable (goroutines / WebSocket now), behind the Gateway
  node registry.

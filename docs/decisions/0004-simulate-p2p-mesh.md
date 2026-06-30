# ADR-0004: Simulate the P2P phone mesh; build FHE + sharding for real

**Status:** Accepted — 2026-06-30
**Context:** 9-day hackathon. The headline innovation is the serverless biometric
DB (FHE templates sharded across shoppers' phones over in-store Wi-Fi).

## Decision
Do **not** build a real cross-platform Wi-Fi Direct mobile mesh. Simulate the phone
mesh in-process: a Go script spins up N=50 local nodes that store and return shards.
Build **for real**: the Microsoft SEAL FHE encrypt + blind match (Python AI) and the
Shamir's Secret Sharing split / quorum / rebalance (Go Gateway).

## Why
- A stable Android/iOS Wi-Fi Direct mesh is a multi-week mobile-networking effort;
  it would consume the entire build window for plumbing the judges never see.
- The demo's credibility lives in the **crypto and sharding math**, not the radio
  layer — those are built real and visible.
- Wi-Fi Direct's GO-role churn (cannot transfer Group Owner) is the #1 real-world
  failure mode; simulating it with an always-present anchor node guarantees the demo
  reconstructs every time.

## Decision params
- N = 50 shards, quorum K = 15 (Shamir's Secret Sharing).
- Ciphertext ~128 KB (SEAL/BFV, 128-bit).
- One simulated **anchor node** never leaves and always holds a full quorum
  (models the real in-store anchor = POS terminal broadcasting GO Intent 15).

## Consequences
- Match distance uses **encrypted squared Euclidean distance** (SEAL native
  sub/mul/sum), decrypting only the final scalar — avoids BFV's lack of native
  division. See `docs/plans/p2p-biometric.md` §1.
- Node transport is swappable (goroutines now; real Wi-Fi Direct later) behind the
  Gateway's node-registry interface.
- Faked: physical mesh app, captive portal, NVİ KYC, 5651 logging/timestamp.

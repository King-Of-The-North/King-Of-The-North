<div align="center">

# 👑 King Of The North

### Day-Zero Yield — a CeDeFi retail payment system

**Spend tomorrow's interest today. Pay with your face. Be the infrastructure, get paid.**

Built for the **MOKA UNITED Hackathon** · July 3–12

[![CI](https://img.shields.io/badge/CI-GitHub%20Actions-2088FF)]()
[![Hackathon](https://img.shields.io/badge/MOKA%20UNITED-Hackathon-6f42c1)]()
[![License](https://img.shields.io/badge/license-MIT-blue)]()

</div>

---

## 🧭 What is this?

A **CeDeFi** (Centralized–Decentralized Finance) retail payment platform with three
ideas working together:

| 💡 Innovation | What it does |
|--------------|--------------|
| **Day-Zero Yield** | Spend the *projected future yield* of a locked deposit **immediately**. Principal is never liquidated — yield self-repays the credit line. |
| **DePIN P2P ledger** | Client **phones run the nodes** (they ARE the servers), so Moka offloads cloud cost — and users **earn credit** for the uptime/storage/bandwidth they contribute. The ledger is **replicated** (every node a full signed copy), so a lost node loses nothing. |
| **Scan-and-go + on-device face pay** | The phone scans product barcodes and authorizes with **on-device face recognition** — the biometric **never leaves the device**. No checkout line, no gate hardware. |

It rides **on top of Moka United's licensed rails** (digital wallet, card issuing,
Virtual POS) — real money custody stays with Moka; we never custody fiat directly.

> 📐 Design: **[ARCHITECTURE.md](./ARCHITECTURE.md)** · decisions in **[docs/decisions/](./docs/decisions/)** · plans in **[docs/plans/](./docs/plans/)**
> The current design is defined by **ADR-0005–0009**; older prose in ARCHITECTURE.md §2–§3 is superseded.

---

## 🏗️ Architecture

```
   Expo mobile app (consumer)          Next.js (store catalog + admin)
   deposit · scan-and-go · face-pay     barcodes · settlements · ledger view
        │  REST                                   │  REST
        └──────────────┬──────────────────────────┘
                       ▼
                 Go API Gateway  ── hosts ──►  P2P replicated ledger nodes
                 REST · routing · DePIN metering        (+ client phone nodes)
              gRPC │                    │ gRPC
       ┌───────────▼──────┐   ┌─────────▼──────────┐
       │  Go Wallet/Yield │   │  Python AI         │
       │  ledger · L0     │   │  (off pay path —   │
       │  ★ Moka owner ★ │   │  on-device auth)   │
       └────────┬─────────┘   └────────────────────┘
        Postgres │ (authoritative $)   HTTPS │
                 └──────► MOKA UNITED APIs (custody · settle)
```

**CeDeFi safety:** real money in Moka + authoritative balances in Postgres (**Ce**);
the P2P log is a **replicated, signed audit ledger** that never custodies funds (**De**).

---

## 🔐 Day-Zero Yield Math

```
FV = D · (1 + r/n)^(n·t)            # future value
Yp = D · [ (1 + r/n)^(n·t) − 1 ]    # projected yield
L0 = Yp · (1 − m)                   # spendable Day-Zero limit (floored to kuruş)
```

`D` deposit · `r` APY · `n` compounding · `t` lock-up · `m` risk margin (10–15%, fixed
pool). Implemented + tested in [`wallet/internal/calc`](./wallet/internal/calc).

---

## 🗂️ Monorepo

| Dir | Stack | Role |
|-----|-------|------|
| `mobile/` | Expo / React Native | **Consumer app** (the demo) |
| `frontend/` | Next.js | Store catalog + admin dashboard |
| `gateway/` | Go | REST ingress, gRPC routing, P2P ledger + DePIN metering |
| `wallet/` | Go | Ledger, Day-Zero math, **Moka integration** |
| `ai-biometric/` | Python | Off the pay path (auth is on-device); optional liveness |
| `proto/` | Buf/protobuf | gRPC contract (`wallet.proto`) |
| `docs/` | — | `decisions/` (ADRs) + `plans/` |

---

## 🚀 Quickstart

```bash
brew install buf go-task lefthook golangci-lint   # one-time tools
cp .env.example .env
task setup        # install JS + Go + Python deps (+ git hooks)
task up           # Postgres + gateway + wallet + ai-biometric (Docker)
task dev:mobile   # run the Expo app natively
```

`task --list` for everything. Contributor guide: **[CONTRIBUTING.md](./CONTRIBUTING.md)**.

---

## 🇹🇷 Regulatory posture

Real fiat lives in **licensed Moka** (Law 6493); **no biometric is stored** server-side
(on-device auth, KVKK-friendly); 2FA via device-bound credentials (BRSA Art. 34); card
data stays inside Moka's PCI environment. Cross-border/biometric-store risk is removed
by design.

---

## 🚧 Status

**Design done; tooling + scaffolds in place.** Backends are runnable skeletons
(boot + `/healthz`); feature build follows the service plans during **July 3–12**.

<div align="center">

*Bringing future capital into the present — securely, on-device, and decentralized.*

</div>

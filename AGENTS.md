# King Of The North — Agent Guide (monorepo root)

CeDeFi retail payment system for the **MOKA UNITED Hackathon (Jul 3–12)**.
Read this first, then the folder-level `AGENTS.md` for the service you touch.

## ⚠️ The design pivoted — trust the ADRs, not the old prose

`README.md` and `ARCHITECTURE.md §2–§3` describe an **earlier** design (FHE biometric
sharding + physical RFID gate). That is **superseded**. The current design lives in:

- `docs/decisions/0005` — P2P money ledger = **full-copy replication**, not sharding. Lost node = zero loss.
- `docs/decisions/0006` — auth = **on-device OpenCV face match**; biometric **never leaves the phone**.
- `docs/decisions/0007` — **phone barcode scan-and-go** replaces the physical RFID gate (gate = production vision only).
- `docs/decisions/0008` — **DePIN**: client phones run the P2P nodes → cut Moka cloud OPEX → users **earn credit** for it. This is the headline.
- `docs/plans/mobile-app.md` — the consolidated build plan.
- `docs/decisions/0001–0004` — still in force (fixed pool only, mock Moka behind interface, integer kuruş money, simulate-where-needed).

When the old docs and an ADR disagree, **the ADR wins**.

## Monorepo layout & ownership

| Dir | Stack | Role | Status |
|-----|-------|------|--------|
| `mobile/` | Expo / React Native | **Consumer app** (the demo): deposit → L0 → scan-and-go → face-pay → receipt → node/earnings | scaffold only |
| `frontend/` | Next.js (SDK changed — see its AGENTS.md) | **Store catalog + admin dashboard** (NOT the consumer wallet anymore) | scaffold only |
| `gateway/` | Go | REST ingress, gRPC routing, **P2P replicated-ledger nodes + DePIN metering** | empty |
| `wallet/` | Go | Ledger, Day-Zero math, **Moka integration (single owner)**, `CreditNodeReward` | empty |
| `ai-biometric/` | Python | Off critical path now (auth is on-device); optional liveness / fallback match | empty |
| `proto/` | protobuf | gRPC contract — **change here first** | empty |
| `docs/` | — | `plans/` and `decisions/` (ADRs) — source of truth | active |

## Who works on what

- **Teammate:** `mobile/` (the Expo consumer app). Start at `mobile/AGENTS.md` + `docs/plans/mobile-app.md`.
- **Owner (Yusuf):** everything else — `gateway/`, `wallet/`, `proto/`, `frontend/`, infra.

## Conventions

- **Money = integer minor units (kuruş, `int64`/`BIGINT`)** — never floats for stored balances (ADR-0003).
- **Proto first:** any cross-service change starts in `proto/`, then regen stubs.
- **Moka is mocked behind an interface** until sandbox creds land (ADR-0002).
- **Safety invariant:** real money lives in Moka + Postgres (authoritative). The P2P layer only **replicates a signed audit log** — it never custodies balances. Don't shard money.
- One ADR per decision in `docs/decisions/NNNN-*.md`; add a new one when you change direction.

## The demo loop (what must work end to end)

deposit → Day-Zero limit `L0` granted → scan product barcodes → on-device face-pay →
Moka mock settle → receipt → entry in the replicated ledger → phone earns credit for being a node.

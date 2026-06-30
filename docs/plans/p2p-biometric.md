# Decentralized P2P Biometric Database — Build Plan

**Services:** `ai-biometric/` (Python, FHE + embedding) + `gateway/` (Go, sharding
+ node registry)
**Roadmap slot:** AI engine Days 3–4; Gateway sharding Days 5–6
**Source:** NotebookLM "King Of the North" (conv `5318793a`), ARCHITECTURE.md §2
**Status:** ⚠️ SUPERSEDED — FHE biometric sharding **descoped**.

> **Superseded 2026-06-30.** Auth moved **on-device** (ADR-0006: OpenCV face match,
> biometric never leaves the phone), so there are no biometric shards. The P2P layer
> is repurposed to the **replicated signed money ledger** (ADR-0005) and the **DePIN
> node-reward** model (ADR-0008). See `docs/plans/mobile-app.md`. The FHE/SEAL +
> Shamir sharding content below is retained for reference only and is **not built**.

---

## 0. Core decision — simulate the mesh (ADR-0004)

Do **not** build a real cross-platform Wi-Fi Direct mobile mesh for the hackathon.
It is a multi-week mobile-networking project and would consume the whole window.
Instead, **simulate the phone mesh** in-process; everything above the radio layer
(FHE, sharding, quorum, rebalancing) is built for real and is what the demo shows.

| Layer | Hackathon reality |
|-------|-------------------|
| "Shoppers' phones" | Go script spins N (=50) local nodes (goroutines / WebSocket clients / containers). Each just stores and returns shards on request. |
| FHE encrypt + blind match | **Real** — Python AI service, Microsoft SEAL. |
| Shamir sharding + quorum + rebalance | **Real** — Go Gateway. |
| POS terminal | Next.js webcam capture → REST → Gateway. |
| Physical Wi-Fi Direct radio, captive portal, NVİ KYC, 5651 logging | **Faked** — narrative only. |

---

## 1. FHE pipeline (Python AI service owns)

- **Library:** Microsoft **SEAL**, **BFV** scheme, **128-bit** security.
- **Embedding:** dlib ResNet / OpenCV → 128-dim facial vector.
- **Encrypt:** SEAL encrypts the vector → ~**128 KB** ciphertext.
- **Blind match:** compute encrypted distance between live scan ciphertext and
  stored ciphertext, **never decrypting** the templates. Only the final scalar
  distance is decrypted and compared to the ≥98% confidence threshold.
- **Latency (benchmark):** ~50 ms profile generation, ~**370 ms** match → comfortably
  sub-second at the gate.

> **Implementation reality vs the source paper:** the cited PINTA scheme uses
> `FHE_Sub` + `FHE_Div` for distance. Stock SEAL/BFV has no native division. The
> practical, demo-safe path is **encrypted squared Euclidean distance**:
> `d² = Σ (aᵢ − bᵢ)²` using SEAL subtract + multiply + sum (all native BFV ops),
> then decrypt only the single scalar `d²` and threshold it. Same security property
> (templates never decrypted), standard FHE-biometrics approach, avoids the exotic
> division. Use this unless time allows replicating PINTA exactly.

---

## 2. Sharding & quorum (Go Gateway owns)

- **Algorithm:** Shamir's Secret Sharing (Erasure-Coding style). Reed–Solomon is an
  acceptable alternative if a maintained Go lib is easier.
- **Split:** 128 KB ciphertext → **N = 50** shards, one per simulated node.
- **Quorum:** any **K = 15** shards reconstruct the ciphertext. Tolerates churn.
- **Node registry:** Gateway tracks which node holds which shard, liveness, and
  current quorum margin.
- **Rebalance:** on node join, Gateway pushes fresh redundant shards to keep
  `live_shards ≥ K`. On node leave, if margin drops, re-shard from a surviving
  reconstruction.
- **Match flow:** POS request → Gateway pulls ≥K shards from nodes → reassembles
  ciphertext → hands to AI service for blind match → returns UserID + session token.

---

## 3. Wi-Fi Direct churn + anchor node (design; explains the sim choice)

- **Problem:** Wi-Fi Direct elects one Group Owner (GO = local DHCP/gateway) via
  "GO Intent" negotiation. The protocol **cannot transfer the GO role** — if the GO
  phone leaves, the group dissolves and its shards vanish. This is the #1 real-world
  availability threat (ARCHITECTURE.md §2.5).
- **Fallback:** a dedicated always-on **Anchor Node** (the POS terminal / smart
  display) broadcasts **GO Intent = 15 (max)** → always wins negotiation → permanent
  GO and guaranteed quorum holder. Stabilizes the mesh.
- **In the simulation:** the anchor node = a never-leaving node in the Go script that
  always holds a full quorum. Models the real anchor and guarantees the demo never
  fails to reconstruct.

---

## 4. Service ownership (hard boundaries)

| Piece | Owner | Boundary |
|-------|-------|----------|
| OpenCV/dlib embedding, SEAL FHE encrypt + blind match | Python AI | No balances/limits/fiat access |
| Shamir sharding, node registry, rebalance, quorum, reassembly | Go Gateway | No biometric inference, no ledger math |
| UserID → ledger deduction | Go Wallet | No image processing (see day-zero-wallet.md) |
| Webcam capture, light crop/resize, result UI | Next.js | No crypto, no financial math |

---

## 5. Minimum viable demo

**Real:** SEAL FHE encrypt + blind distance match; Shamir split into 50 shards
across 50 simulated nodes; pull a 15-shard quorum; reassemble; match; return UserID;
anchor node guarantees availability; live webcam recognizes a registered face.

**Faked:** physical mobile mesh app, Wi-Fi Direct radio, captive portal, NVİ SOAP
KYC ("Speedy KYC" button), Law 5651 logging + TÜBİTAK timestamp.

## 6. Open questions feeding this plan

- **FHE latency at gate** (open Q#2): 370 ms benchmark is fine; if reassembly +
  match exceeds budget, pre-fetch/cache shards on shopper approach.
- **Anchor node** (open Q#1): resolved for demo — simulated anchor always present.
- **Fallback UX** (open Q#4): if quorum unmet + phone dead → hard decline, or manual
  Moka card tap. Decide before integration (Days 7–8).

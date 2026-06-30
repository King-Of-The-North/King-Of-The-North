# King Of The North — System Architecture Document (SAD)

**Project:** Day-Zero Yield CeDeFi Biometric P2P Retail Payment System
**Target:** MOKA UNITED Hackathon (online coding window: July 3 – July 12)
**Status:** DESIGN & PLANNING — no code written yet
**Document owner:** Lead Cloud & Systems Architect
**Source of record:** NotebookLM notebook *"King Of the North"* (162 sources; conversation `5318793a`)

---

## 0. Executive Summary

King Of The North is a **CeDeFi** (Centralized-Decentralized Finance) retail
payment platform that lets a shopper spend the *future yield* of a locked
deposit **on day zero**, authenticate by **face** or **Passkey**, and walk out
of a store without a checkout line. It is engineered to ride on top of **Moka
United's licensed payment rails** (digital wallet, Virtual/Physical/SoftPOS,
card issuing) rather than replace them, and to satisfy Turkey's strict
regulatory regime (KVKK / Law 6698, Law 6493, Law 5651, BDDK/BRSA Art. 34).

The "centralized" half (regulated fiat, ledger, Moka APIs) is what keeps it
legal; the "decentralized" half (in-store P2P biometric storage, on-chain-style
yield logic) is what makes it novel.

Three innovations define the system:

1. **Day-Zero Yield credit** — spend projected interest immediately; principal
   is never liquidated.
2. **Serverless biometric DB** — FHE-encrypted face templates sharded across
   shoppers' phones over in-store Wi-Fi; no cloud biometric store.
3. **Just Walk Out checkout** — Passkey (primary) or face (fallback) identity +
   UHF RAIN RFID merchandise tracking → automatic settlement via Moka SoftPOS.

---

## 1. The Four Core Microservices ("Big Picture")

> Architectural rationale: **"hot path / smart path" split** — Go for
> latency-sensitive financial operations, Python for AI inference.

| # | Service | Language | Responsibility | Hard boundary (does NOT do) |
|---|---------|----------|----------------|-----------------------------|
| 1 | **Frontend** (Dashboard + POS Simulator) | Next.js | Consumer wallet UI: funds, yield pools, Day-Zero limit. Merchant POS UI: camera capture, light crop/resize, fires REST. | No financial math, no crypto matching. |
| 2 | **API Gateway** | Go | Central orchestrator: auth, rate-limit, POS hardware-ID + merchant-credential validation, routes REST→gRPC, oversees P2P network health & shard rebalancing. | No business logic, no biometric inference, no ledger math. |
| 3 | **Wallet / Yield Service** | Go | Financial brain & ledger. Computes Day-Zero limit, atomic deductions, fiat tokenization. **Owns the server-to-server Moka United API integration.** | No image processing, no network routing. |
| 4 | **AI Biometric Service** | Python | Digital-identity engine. Extracts 128-dim facial embedding (dlib ResNet / OpenCV), FHE-matches vs shards, returns UserID + session token at ≥98% confidence. | Zero access to balances, limits, or fiat. |

### 1.1 Service Interconnect & API Map

| Source | Target | Protocol | Endpoint / RPC | Purpose |
|--------|--------|----------|----------------|---------|
| Frontend | API Gateway | REST | `POST /v1/user/deposit` | Trigger Day-Zero calc + deposit |
| API Gateway | Wallet | gRPC | `rpc CalculateLimit(Deposit)` | Compute initial credit line |
| Frontend POS | API Gateway | REST | `POST /v1/pos/pay-face` | Send face image for authz |
| API Gateway | AI Service | gRPC | `rpc IdentifyFace(Image)` | 1:N biometric match |
| AI Service | Wallet | gRPC | `rpc ValidateTransaction` | Signal match → ledger deduction |

### 1.2 Moka United Integration (the regulated foundation)

Moka United is a **licensed e-money / payment institution** (İş Bankası "Moka" +
OYAK "United Payment" merger), independent of banks. The system maps every
decentralized innovation onto Moka's licensed rails. **The Go Wallet Service is
the single integration owner.**

- **Fiat On-Ramp** — Customer deposits TRY via bank transfer / card into their
  **Moka digital wallet**. Wallet Service detects via Moka API → mints a **1:1
  tokenized** balance on the internal distributed ledger.
- **Fiat Off-Ramp** — On principal withdrawal, Wallet Service **burns** the
  internal token and uses **Moka Money-Transfer APIs** to remit fiat to the
  user's bank.
- **Card Issuing** — Day-Zero credit is bound to a **Moka-issued virtual/physical
  card** (Mastercard / Visa / domestic TROY). Wallet Service sets **dynamic spend
  controls / MCC / velocity limits** via Moka card APIs based on yield health.
  This makes the credit spendable *anywhere*, not only in biometric stores.
- **POS Settlement** — Biometric "Face-ID" and "Just Walk Out" events clear &
  settle through **Moka Virtual POS / Payment-Gateway APIs** (3D Secure 2.0,
  AI smart-routing, fraud prevention), i.e. the gate behaves as a specialized
  **Moka SoftPOS**.

**Strategic value to Moka:** ↑ transaction volume (future capital pulled to
present), merchant acquisition via SoftPOS/Face-ID upgrade, "sticky" deposits as
an acquisition moat vs PayU / Sipay.

---

## 2. Decentralized Biometric Database (In-Store Wi-Fi P2P)

**Goal:** store biometric templates with *zero* cloud database and *zero*
cross-border exposure — keeping data physically inside Turkey for KVKK.

### 2.1 FHE Sharding

- Registration → Python AI extracts 128-dim embedding.
- Embedding encrypted with **Fully Homomorphic Encryption** (Microsoft **SEAL**,
  **BFV** scheme, **128-bit** security) → ~128 KB ciphertext.
- API Gateway **shards** the ciphertext into dozens of meaningless fragments.
- Matching is done on ciphertext (FHE subtract/divide for distance score) —
  data is **never decrypted**; cloud only does stateless math. (Zero-Knowledge
  Biometrics.)

### 2.2 Erasure Coding & Redundancy

- Shards distributed via **Erasure Coding** (Shamir's-Secret-Sharing-style) to
  smartphones on store Wi-Fi.
- Example: ciphertext → **50 shards** across 50 phones; reconstruction needs only
  a **quorum of ~15**. Tolerates heavy node churn.

### 2.3 Bypassing Cloud Storage

- Shards broadcast to shoppers' phones acting as **edge nodes** over enterprise
  Wi-Fi → fully **serverless** biometric DB.
- No central cloud holds biometrics → breach liability neutralized, OPEX cut.

### 2.4 Regulatory Compliance

- **KVKK / Law 6698** — Biometrics are "special category data"; cross-border
  transfer needs SCCs filed within 5 business days. Confining shards to in-store
  phones keeps data **inside Turkey**; FHE means it is never stored in
  reconstructable form. (Note: KVKK breach fines up to ~13.6M TRY; 72-hour breach
  notification; VERBİS registration.)
- **Law 5651 (logging)** — Store Wi-Fi = "Internet Mass Use Provider" → must keep
  access logs (internal IP, MAC, login/logoff times) for **2 years**, BTK format.
  Because P2P traffic skips the main router, an **Edge-Buffered P2P Logging
  Engine** runs on the P2P Group Owner: monitors ARP/DHCP leases, caches to
  on-device SQLite (FIPS 140-3), pushes over **TLS** to a Linux syslog
  aggregator, daily **SHA-512** hash + **TÜBİTAK Kamu SM** digital timestamp.
- **Identity mapping** — Captive portal collects T.C. Kimlik / passport + mobile,
  real-time **SOAP → NVİ** national ID validation, **SMS OTP**, granular KVKK
  consent sliders (CMP). OWE-encrypted SSID + Layer-2 isolation for BYOD.

### 2.5 Network Dynamics (Wi-Fi Direct)

- Group formation via **GO Intent** negotiation → one **P2P Group Owner (GO)** +
  **Clients**. GO is local gateway/DHCP.
- **Churn risk:** Wi-Fi Direct **cannot transfer** the GO role; if GO leaves, the
  group dissolves and re-forms. Departing devices' shards are lost.
- **Rebalancing:** API Gateway monitors health; on new shopper onboard (captive
  portal) it pushes fresh redundant shards.
- **Quorum check:** while surviving shards ≥ quorum, POS pulls them over local
  Wi-Fi, reassembles ciphertext, pays — no delay.

> ⚠️ **Open risk flagged for review:** GO-dissolution + shard loss is the single
> biggest availability threat. Need a fallback (e.g. an always-present in-store
> anchor node acting as guaranteed GO / quorum holder). Discuss before build.

---

## 3. "Just Walk Out" Frictionless Checkout

### 3.1 Identity — Passkeys (FIDO2/WebAuthn), primary

- **Possession factor:** device-bound cryptographic key.
- **Inherence factor:** local Face ID / fingerprint unlocks the key on the user's
  own phone.
- Satisfies **BRSA Article 34** universal 2FA; phishing-resistant; replaces
  **banned SMS-OTP** for active mobile users. Raw biometric never leaves device.

### 3.2 Merchandise — UHF RAIN RFID Gates

- **860–960 MHz** UHF band; circular-polarized (RHCP) beam-steering antennas;
  read zone ~2.5 m between pedestals (up to ~10 m in modular setups), no line of
  sight; overcomes body-shielding.
- **EPC encoding:** GS1 EPC TDS, **SGTIN-96** (Header, Filter, Company Prefix,
  Item Reference, Serial). **Filter Value** distinguishes "Product" vs "Member".
- **Anti-collision:** Slotted Aloha (probabilistic) + Adaptive Tree-Walking
  (deterministic) read hundreds of tags in fractions of a second.
- **Cart reconciliation:** API-first middleware (CYBRA Edgefinity / Turck Vilant)
  filters noise, aggregates EPC reads into a digital cart.

### 3.3 Payment — Day-Zero Yield Credit Limit

Inspired by Alchemix self-repaying loans; future yield treated as present
collateral, principal never liquidated.

```
Future value:      FV  = D · (1 + r/n)^(n·t)
Projected yield:   Yp  = D · [ (1 + r/n)^(n·t) − 1 ]
Spendable limit:   L0  = Yp · (1 − m)
```

- `D` deposit, `r` expected APY, `n` compounding freq, `t` lock-up, `m` risk
  margin.
- **Risk margin `m`:** 10–15% Fixed Interest Pool (T-Bills / stablecoin lending);
  35–50% AI Stock-Market Pool (equities/ETFs).
- **Underperformance:** smart contract runs **Yield Amortization Extension** —
  extends lock-up `t` (time-to-repay) instead of touching principal; real-time
  LTV monitored. Equity pool adds **Defensive Rebalancing** into stable/gold
  assets when LTV breaches threshold.

### 3.4 End-to-End Sequence

1. **Entry/Shop** — Customer joins in-store Wi-Fi (5651 logging + KVKK
   residency), picks up RFID-tagged items.
2. **Exit, primary** — Unlock Passkey via Face ID/fingerprint; phone transmits
   identity token to gate via **NFC/BLE**.
3. **Exit, fallback** — Phone dead → step to POS camera; AI Service extracts
   embedding, FHE-matches vs P2P shards.
4. **Gate scan** — UHF gate reads identity token **and** all product EPCs at once.
5. **Gateway** — Middleware bundles cart + identity → Go API Gateway (gRPC).
6. **Settlement** — Wallet Service matches UserID↔cart, checks cost ≤ Day-Zero
   limit, calls **Moka Virtual POS / Payment-Gateway APIs** (gate = Moka SoftPOS)
   → atomic ledger deduction + digital receipt.

**Biometric vs Passkey:** Passkey = primary, on-device, phishing-resistant SCA.
Biometric face-match = fallback when phone unavailable, executed against P2P FHE
shards. Both map to the same UserID + session token consumed by Wallet Service.

---

## 4. End-to-End Reference Flows

### 4.1 Onboarding & Deposit
```
User → Next.js: deposit TRY
Next.js → Gateway: POST /v1/user/deposit
Gateway → Wallet: rpc CalculateLimit(Deposit)
Wallet → Moka API: detect wallet deposit → mint 1:1 internal token
Wallet → Moka Card API: issue/bind card, set spend controls = f(yield health)
Wallet → Gateway → Next.js: return L0 (Day-Zero limit)
Registration: AI extracts embedding → FHE(SEAL/BFV) → shard → P2P phones
```

### 4.2 Just Walk Out Payment
```
Gate: read Passkey token (or fallback: AI face-match vs FHE shards) + EPC cart
Middleware → Gateway (gRPC): { UserID, cart[] }
Gateway → Wallet: rpc ValidateTransaction
Wallet: total ≤ L0 ? atomic deduct : decline
Wallet → Moka Virtual POS API: clear + settle (3DS 2.0, fraud routing)
Wallet → Gateway → phone: digital receipt
```

---

## 5. Regulatory Compliance Matrix

| Law / Rule | Requirement | Architectural answer |
|------------|-------------|----------------------|
| KVKK / Law 6698 | Biometrics = special category; data residency; SCC for cross-border | FHE Zero-Knowledge templates; shards confined to in-store phones (in-country); never reconstructable |
| Law 6493 | Digital wallet licensing (CBRT) | Fiat lives in **licensed Moka wallet**; we never custody fiat directly |
| Law 5651 | 2-yr access logs, BTK format | Edge-Buffered P2P Logging Engine + TÜBİTAK Kamu SM timestamp |
| BDDK / BRSA Art. 34 | Universal 2FA, SMS-OTP banned | Passkeys (possession) + device biometric (inherence) |
| PCI DSS / 3DS 2.0 | Card data protection | Handled inside Moka's licensed environment; card data never enters our services |

---

## 6. MVP / Hackathon Build Strategy (9-day window, Jul 3–12)

**"Demo-first":** build the innovative loop; mock peripheral traditional banking.

- **Build for real:** Day-Zero calc, FHE match path, RFID/Passkey identity, P2P
  shard demo, Moka SoftPOS settlement call.
- **Mock / stub:** full card-issuing lifecycle, NVİ SOAP, TÜBİTAK timestamping,
  bank off-ramp — represent with fakes that satisfy the demo narrative.
- **Critical demo loop:** deposit → L0 granted → walk out → settle → receipt.

---

## 7. Open Questions for the Team (resolve before coding)

1. **P2P availability:** dedicated in-store anchor node to guarantee GO + quorum,
   vs pure shopper-phone mesh? (See §2.5 risk.)
2. **FHE latency:** can SEAL/BFV ciphertext match hit sub-second at the gate, or
   do we pre-fetch/cache shards on approach?
3. **Moka API access:** which sandbox endpoints are actually available for the
   hackathon (wallet, card issuing, Virtual POS)? Confirm credentials early.
4. **Fallback UX:** if phone dead AND quorum unmet — hard decline, or manual POS
   card tap on the Moka-issued card?
5. **Yield source for demo:** simulated fixed-rate pool only, or also mock the
   AI equity pool with canned rebalancing?

---

*Prepared from NotebookLM project sources. No implementation code authored. Ready
for architectural review.*

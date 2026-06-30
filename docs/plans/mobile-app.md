# Consumer Mobile App + CeDeFi DePIN — Build Plan

**Service:** `mobile/` (Expo / React Native) + supporting changes in `gateway/`,
`wallet/`, `proto/`, `frontend/`
**Source of record:** NotebookLM "King Of the North" (de12fd7f), ARCHITECTURE.md
**Status:** PLANNED — ready to build

---

## 0. Why this exists

In **production** the consumer experience lives **inside the Moka United app**. We
cannot ship inside Moka United for the hackathon, so we build a **standalone consumer
mobile app** that connects to our Wallet service to demonstrate the full experience.

Designing it locked five decisions (see `docs/decisions/`):

- **ADR-0005** — P2P money ledger = full-copy **replication**, not Shamir sharding.
- **ADR-0006** — Auth = **on-device OpenCV face match**; biometric never leaves phone.
- **ADR-0007** — **Phone scan-and-go** replaces the physical RFID gate (gate = vision).
- **ADR-0008** — **DePIN**: client phones run nodes, cut Moka cloud OPEX, **earn credit**.
- (Carries ADR-0001/0002/0003: fixed pool, mock Moka behind interface, integer kuruş.)

**Core differentiator:** the crowd is the infrastructure. Phones run the P2P nodes,
so Moka offloads cloud cost, and users get paid (credit) for being a server.

---

## 1. CeDeFi architecture (safe by construction)

| Layer | Holds | Tech | Safety |
|-------|-------|------|--------|
| **Ce** custody | Real fiat + yield pool | Moka licensed rails | Money never on phones |
| **Ce** authoritative ledger | Balances, Day-Zero credit | Postgres (ADR-0003) | Atomic deduct, source of truth |
| **De** P2P | **Replicated** signed append-only tx log | Go nodes + phone nodes + anchor | Lost node = zero loss |

Every spend: Postgres atomic-deduct (authoritative) → append signed hash-chained
entry to the replicated P2P log (decentralized proof) → Moka mock settle (rails).

---

## 2. `mobile/` — Expo / React Native app

Screens / flows:
1. **Onboard + Deposit** — enter TRY → Wallet `CalculateLimit` (via Gateway REST) →
   show **Day-Zero limit L0**, principal, projected yield, lock-up.
2. **Wallet dashboard** — balance, available Day-Zero credit, yield-health, pool
   position.
3. **Face enrollment** — capture once; embedding computed **on-device**; stored
   encrypted in `expo-secure-store`. Nothing sent to server (ADR-0006).
4. **Scan-and-go** — `react-native-vision-camera` scans product **barcodes** → cart
   (prices from store catalog API).
5. **Pay** — on-device **OpenCV face match** → submit cart to Wallet
   `ValidateTransaction` → atomic deduct ≤ L0 → Moka mock settle.
6. **Receipt** — digital receipt; appended to replicated P2P ledger.
7. **Node / earnings panel (DePIN)** — phone runs a real lightweight node; shows
   "you are a server": uptime, data replicated, requests served, **credit earned**.

On-device face auth (recommended path): vision-camera frame capture + face-embedding
model (`react-native-fast-tflite` / MediaPipe) → 128-d vector → cosine/Euclidean
match + threshold.
**Risk + fallback:** OpenCV C++ on-device needs an Expo **dev build** + config plugin
(not Expo Go) — biggest mobile risk. Fallbacks: TFLite/MediaPipe embedding on-device,
or POST frame to Python `ai-biometric` over local network. Decide by Day 1 of mobile.

---

## 3. P2P replicated ledger + DePIN (`gateway/` Go)

- N Go nodes each hold the **full** append-only hash-chained log; never-leaving
  **anchor node** (ADR-0004) guarantees finality. Entry:
  `{ prev_hash, user_id, amount, items, moka_ref, sig }`.
- **Phones are real nodes** — Expo app opens a WebSocket to the Gateway registry,
  replicates the log, answers reads/verify. Demo scale = a few phone nodes + simulated
  Go nodes + anchor.
- **Metering:** Gateway tracks per-node uptime, bytes replicated, requests served,
  validations → periodic **contribution proof**.
- **Reward:** Gateway → Wallet `CreditNodeReward(user_id, contribution_proof)` →
  wallet credit (authoritative Postgres, never minted on phone).
- Kill a non-anchor node mid-demo → ledger still readable + verifiable (proves
  lost-node-no-loss).

---

## 4. `wallet/` (Go) — extends day-zero-wallet.md

- `CalculateLimit`, `GetAccount` (balance/L0/yield-health), `ValidateTransaction`
  (now a **cart** of items → sum → atomic deduct ≤ available_credit), `CreditNodeReward`.
- On settle: Postgres authoritative row + emit signed entry to P2P log.

## 5. `ai-biometric/` (Python) — repurposed

- Off the critical pay path (auth is on-device). Optional: enrollment liveness/quality,
  or local-network match fallback. FHE/SEAL biometric sharding **descoped**.

## 6. `frontend/` (Next.js) — store/merchant side

- POS simulator → **store catalog** (products with scannable barcodes + prices) +
  **admin dashboard**: settlements, live P2P ledger, "cloud cost avoided" counter.

## 7. `proto/` — contract first

- `wallet.proto`: add `GetAccount`, `CartItem` (repeated) on `ValidateTransaction`,
  `CreditNodeReward(user_id, contribution_proof)`. Regenerate Go stubs first.

---

## 8. Build order (9-day window, Jul 3–12)

1. Proto contract update + regen.
2. Wallet (calc, Postgres ledger, mock Moka) — Days 1–2 (`day-zero-wallet.md`).
3. Gateway REST + P2P replicated nodes + anchor + metering — Days 3–4.
4. Expo shell: deposit → L0 dashboard → receipts — Days 4–5.
5. Barcode scan-and-go + cart + `/v1/checkout` settle — Days 5–6.
6. DePIN: phone node + earnings panel + `CreditNodeReward` — Days 6–7.
7. On-device face enroll + match (decision/fallback by Day 1 of this step) — Days 6–7.
8. Store catalog + admin view (Next.js) — Day 7.
9. Integration + demo polish: deposit → scan → face-pay → settle → receipt → ledger →
   earn credit as node — Days 8–9.

---

## 9. Verification (end-to-end)

- **Day-Zero math:** unit test vs worked example (D=10,000, r=0.12, n=12, t=1, m=0.10
  → L0 = 1,141.42 TRY).
- **Ledger safety:** concurrent `/v1/checkout` → `SELECT … FOR UPDATE` prevents
  double-spend; never exceeds `available_credit`.
- **P2P replication:** kill non-anchor node → ledger still readable + verifiable.
- **On-device auth:** enroll, kill network, pay → match works; no biometric in any
  request payload.
- **DePIN reward:** node joins → metered → `CreditNodeReward` raises credit (Postgres
  delta = proof); admin "cloud cost avoided" increments per active node.
- **Full loop:** deposit → L0 → scan → face-pay → Moka mock settle → receipt → ledger
  entry in admin view → phone earns credit as a server.

---

## 10. Out of scope (production-vision narrative only)

UHF RFID gate, NFC/BLE-to-gate, Wi-Fi Direct mesh, FHE/SEAL biometric sharding, real
Moka sandbox (mock behind interface until creds land — ADR-0002), card-issuing
lifecycle, NVİ KYC, Law 5651 logging/timestamp.

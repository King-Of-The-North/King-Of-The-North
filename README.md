<div align="center">

# 👑 King Of The North

### Day-Zero Yield — CeDeFi Biometric P2P Retail Payment System

**Spend tomorrow's interest today. Pay with your face. Just walk out.**

Built for the **MOKA UNITED Hackathon** · July 3–12

[![Status](https://img.shields.io/badge/status-design%20phase-yellow)]()
[![Hackathon](https://img.shields.io/badge/MOKA%20UNITED-Hackathon-6f42c1)]()
[![License](https://img.shields.io/badge/license-MIT-blue)]()
[![KVKK](https://img.shields.io/badge/KVKK-compliant-success)]()

</div>

---

## 🧭 What is this?

**King Of The North** is a **CeDeFi** (Centralized-Decentralized Finance) retail
payment platform with three breakthrough ideas working together:

| 💡 Innovation | What it does |
|--------------|--------------|
| **Day-Zero Yield** | Spend the *projected future yield* of a locked deposit **immediately**. Principal is never liquidated — the yield self-repays the credit line (Alchemix-style). |
| **Serverless Biometric DB** | Face templates are **FHE-encrypted** and **sharded across shoppers' phones** over in-store Wi-Fi. No cloud biometric database, no breach liability, data stays in Turkey. |
| **Just Walk Out** | **Passkey** (or face fallback) for identity + **UHF RAIN RFID** for merchandise → automatic settlement. No checkout line. |

It is designed to ride **on top of Moka United's licensed rails** (digital
wallet, Virtual/Physical/SoftPOS, card issuing) — not replace them — and to
satisfy Turkey's strict regulatory regime (KVKK, Law 6493, Law 5651, BRSA
Art. 34).

> 📐 Full design: **[ARCHITECTURE.md](./ARCHITECTURE.md)**

---

## 🏗️ Architecture at a Glance

```
        ┌─────────────────────────────────────────────────────────┐
        │                    Next.js Frontend                      │
        │            (Wallet Dashboard + POS Simulator)            │
        └───────────────────────────┬─────────────────────────────┘
                                     │ REST
        ┌───────────────────────────▼─────────────────────────────┐
        │                     Go API Gateway                       │
        │        auth · rate-limit · routing · P2P health          │
        └──────┬──────────────────────────────────────┬───────────┘
               │ gRPC                                  │ gRPC
   ┌───────────▼────────────┐              ┌───────────▼────────────┐
   │  Python AI Biometric   │              │   Go Wallet / Yield    │
   │  128-d embed · FHE     │─────gRPC────▶│   ledger · Day-Zero    │
   │  match (SEAL/BFV)      │ ValidateTxn  │   ★ owns Moka API ★    │
   └────────────────────────┘              └───────────┬────────────┘
                                                       │ HTTPS
                                          ┌────────────▼────────────┐
                                          │     MOKA UNITED APIs     │
                                          │ Wallet · Card · VPOS/3DS │
                                          └─────────────────────────┘
```

**Hot path / smart path:** Go handles latency-sensitive money; Python handles
AI inference.

---

## 🧩 The Four Services

| Service | Stack | Owns |
|---------|-------|------|
| **Frontend** | Next.js | Wallet UI, merchant POS UI, camera capture |
| **API Gateway** | Go | Ingress, auth, gRPC routing, P2P shard rebalancing |
| **Wallet / Yield** | Go | Ledger, Day-Zero math, **Moka integration** |
| **AI Biometric** | Python (dlib/OpenCV + SEAL) | Face embedding, FHE match |

---

## 🔐 Day-Zero Yield Math

```
FV  = D · (1 + r/n)^(n·t)            # future value of deposit
Yp  = D · [ (1 + r/n)^(n·t) − 1 ]    # projected yield
L0  = Yp · (1 − m)                   # spendable Day-Zero limit
```

`D` deposit · `r` APY · `n` compounding · `t` lock-up · `m` risk margin
(**10–15%** fixed pool, **35–50%** AI equity pool).

If yield underperforms → **Yield Amortization Extension** lengthens lock-up `t`
instead of touching principal.

---

## 🛰️ Decentralized Biometric Storage

1. Register → 128-dim face embedding.
2. Encrypt with **FHE** (Microsoft SEAL, BFV, 128-bit) → ~128 KB ciphertext.
3. **Erasure-code** into shards (e.g. 50 shards, **quorum 15**).
4. Distribute to shoppers' phones over in-store Wi-Fi Direct.
5. Match on **ciphertext** (FHE subtract/divide) — never decrypted.

✅ No cloud biometric DB · ✅ data stays in-country (KVKK) · ✅ no reconstructable template

---

## 🚪 Just Walk Out Flow

```
join Wi-Fi (5651 log) → grab RFID items
   → exit: Passkey via Face ID/fingerprint  (primary)
           or POS face-match vs FHE shards  (fallback)
   → UHF gate reads identity + EPC cart
   → Gateway bundles → Wallet checks L0
   → Moka Virtual POS settles (3DS 2.0) → receipt
```

---

## 🇹🇷 Regulatory Compliance

| Law | Handled by |
|-----|-----------|
| **KVKK / Law 6698** | FHE zero-knowledge templates, in-country shards |
| **Law 6493** (wallet licensing) | Fiat lives in licensed **Moka** wallet |
| **Law 5651** (logging) | Edge-Buffered P2P Logging + TÜBİTAK Kamu SM timestamp |
| **BRSA Art. 34** (2FA) | Passkeys (possession) + device biometric (inherence) |
| **PCI DSS / 3DS 2.0** | Card data stays inside Moka's environment |

---

## 🚧 Status

**Design & planning phase — no application code yet.** This repo currently holds
the architecture blueprint. Hackathon build window: **July 3–12**.

**MVP strategy:** build the innovative loop for real (deposit → L0 → walk out →
settle → receipt); mock peripheral banking (NVİ SOAP, TÜBİTAK timestamp, card
lifecycle).

### Planned layout
```
king-of-the-north/
├── frontend/        # Next.js — dashboard + POS simulator
├── gateway/         # Go — API gateway
├── wallet/          # Go — wallet/yield + Moka integration
├── ai-biometric/    # Python — embedding + FHE match
├── proto/           # shared gRPC definitions
└── ARCHITECTURE.md  # system design document
```

---

## 🛠️ Tech Stack

**Frontend** Next.js · **Backend** Go (Gateway, Wallet) · **AI** Python, dlib/OpenCV, Microsoft SEAL (FHE) · **Transport** REST + gRPC · **Hardware** UHF RAIN RFID (860–960 MHz, SGTIN-96), FIDO2 Passkeys · **Rails** Moka United (Wallet, Card Issuing, Virtual POS)

---

## 👥 Team

Built for the MOKA UNITED Hackathon. See the [organization profile](./profile/README.md).

---

<div align="center">

*Bringing future capital into the present — securely, biometrically, compliantly.*

</div>

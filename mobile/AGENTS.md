# Expo HAS CHANGED

Read the exact versioned docs at https://docs.expo.dev/versions/v56.0.0/ before writing any code.
(This project is Expo SDK 56, expo-router, TypeScript, `src/` layout.)

---

# `mobile/` — King Of The North consumer app

The **standalone consumer app** for the hackathon demo. In production this experience
lives **inside the Moka United app**; here it's a standalone Expo app that talks to our
backend over the Gateway REST API.

**Read first:** root `../AGENTS.md` (the design pivoted — ADRs win over old docs) and
`../docs/plans/mobile-app.md` (the build plan, §2 = your screens).

## What this app does (the demo)

deposit → Day-Zero limit `L0` → in-store **barcode scan-and-go** → **on-device face
pay** → Moka mock settle → receipt → **node / earnings** panel.

Screens to build (see plan §2):
1. Onboard + Deposit — calls Wallet `CalculateLimit`, shows `L0`.
2. Wallet dashboard — balance, available credit, yield-health.
3. Face enrollment — embedding computed **on-device**, stored encrypted in `expo-secure-store`.
4. Scan-and-go — `react-native-vision-camera` reads product **barcodes** → cart.
5. Pay — on-device face match → submit cart to Wallet → settle.
6. Receipt.
7. **Node / earnings (DePIN)** — phone runs a real lightweight P2P node; shows "you are a server": uptime, data replicated, requests served, **credit earned**.

## Hard rules (don't break these)

- **Biometric never leaves the device** (ADR-0006). No face image/template in any
  network request. Enroll + match happen on-device; server stores nothing biometric.
- **No money math on the client.** The app displays balances and submits intents; the
  Go Wallet service is the authoritative ledger. Never compute or trust a balance locally.
- **The phone node never holds spendable funds.** It replicates a signed audit log only
  (ADR-0005/0008). Rewards are credited server-side via `CreditNodeReward`, never minted here.
- Talk to the backend **only through the Gateway REST API** (`/v1/...`), not directly to Wallet/AI.

## On-device face auth — known risk

Real OpenCV C++ on-device needs an Expo **dev build** + config plugin (not Expo Go).
Recommended path: face-embedding model via `react-native-fast-tflite` / MediaPipe →
128-d vector → cosine/Euclidean match + threshold. Fallback: POST the frame to the
Python `ai-biometric` service over the local network. **Decide by Day 1** — don't let
this sink the build (plan §2).

## Out of scope here

UHF RFID gate, NFC/BLE-to-gate, FHE/SEAL biometric sharding — production vision only.

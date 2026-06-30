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

## Design system — use it, don't reinvent (`@kotn/design-system`)

The look is already built. **Build every screen from the shared system — never hardcode
a color, font, size, spacing, radius, or easing.**

- **Primitives** live in `src/components/ui/` — `Text`, `Heading`, `Button`, `Surface`,
  `Input`, `Amount`. Import from `@/components/ui`. Example:
  ```tsx
  import { Heading, Text, Button, Amount } from '@/components/ui';
  <Heading level="display">Pay</Heading>
  <Amount minorUnits={l0} size="display" />   // kuruş in, formatted out — no math
  <Button variant="accent" onPress={pay}>Confirm</Button>
  ```
- **Tokens** (when a primitive isn't enough): `import { semantic, scale, spacing, radius, easing } from '@kotn/design-system'`.
  Colors via `useTheme()` (`src/hooks/use-theme.ts`) → `theme.accent`, `theme.surface`,
  `theme.textSecondary`, … Spacing/sizes via `Spacing` / tokens from `@/constants/theme`.
- Fonts (Neue Haas Grotesk Display) load in `src/app/_layout.tsx`; the primitives already
  set the right face per weight. Don't set `fontFamily`/`fontWeight` by hand — use `Text`/`Heading`.
- Need a new shared component? Add it to `src/components/ui/` with a web sibling in
  `frontend/src/components/ui/` (same name + props), built on tokens. Don't fork styles per screen.
- The styleguide (`Design` tab → `src/app/explore.tsx`) shows every primitive — copy from it.

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

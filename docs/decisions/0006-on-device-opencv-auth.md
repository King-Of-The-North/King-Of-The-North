# ADR-0006: Authentication = on-device OpenCV face match; no biometric leaves the phone

**Status:** Accepted — 2026-06-30
**Context:** Payment auth + the required AI theme. Earlier design stored FHE face
templates sharded across phones (ARCHITECTURE.md §2). Modern phones already secure
biometrics on-device, making a server/sharded biometric store unnecessary risk.

## Decision
Authentication is **OpenCV face recognition that runs on the user's device**. The
face template is generated and matched **on the phone** and **never leaves it**;
the server stores **nothing** biometric.

- Enrollment: capture face → compute embedding on-device → store encrypted in
  device secure storage (`expo-secure-store`).
- Pay: capture face → match on-device against the enrolled template → on accept,
  authorize the transaction to the Wallet service.
- This is the project's **AI showcase** (OpenCV-class face recognition).

## Why
- iOS Secure Enclave / Android StrongBox-TEE already protect biometrics locally;
  raw biometric never needs to leave the device.
- Storing nothing server-side is the **safest** posture under KVKK (special-category
  data) — no biometric store to breach, no cross-border transfer question.
- Keeps the AI requirement satisfied with our own model, not the OS vendor's.

## Consequences
- **FHE/SEAL biometric sharding is descoped.** The P2P layer is repurposed to the
  replicated money ledger (ADR-0005).
- The Python `ai-biometric` service leaves the critical pay path; optional roles:
  enrollment liveness/quality check, or a local-network match fallback.
- **Implementation risk:** true OpenCV C++ on-device needs an Expo **dev build** +
  config plugin (not Expo Go). Fallback: on-device face-embedding via TFLite /
  MediaPipe, or local-network match to the Python service. Decide by Day 1 of mobile.

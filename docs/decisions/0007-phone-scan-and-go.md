# ADR-0007: Phone barcode scan-and-go replaces the physical RFID gate for the demo

**Status:** Accepted — 2026-06-30
**Context:** The checkout innovation was a UHF RAIN RFID "Just Walk Out" gate with
the phone transmitting an identity token via NFC/BLE (ARCHITECTURE.md §3). Once the
phone itself scans product barcodes, the physical gate is redundant for the demo.

## Decision
The hackathon demo uses **phone-based scan-and-go**: the consumer's phone is the
**barcode scanner + cart + wallet + auth device**. The user scans product barcodes,
builds a cart, and pays with on-device face auth (ADR-0006).

The **UHF RFID gate, NFC/BLE-to-gate, and walk-through pedestals are
production-vision narrative only** — not built.

## Why
- The phone scanning barcodes makes the RFID gate redundant; building real UHF RFID
  hardware is out of reach in a 9-day software hackathon.
- Scan-and-go is fully demonstrable on one device and keeps the whole flow on the
  phone the judges can hold.

## Consequences
- No NFC/BLE identity transport, no gate middleware, no EPC/SGTIN cart reconciliation
  in the build.
- Products need scannable **barcodes** — the Next.js store catalog renders them.
- The "Just Walk Out" gate stays in the pitch as the production evolution.

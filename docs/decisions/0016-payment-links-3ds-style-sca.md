# ADR-0016: Payment links with 3D-Secure-style strong customer authentication

**Status:** Accepted — 2026-07-01
**Context:** The web app is **for commerce owners only** — regular users transact on
the **mobile** app, never the website. So the merchant's primary online tool isn't an
in-store terminal; it's a **shareable payment link** (sent over WhatsApp/email/SMS like
a Stripe/iyzico payment link). When the customer opens it, the payment must be
authorized with a **3D-Secure-style strong customer authentication (SCA)** step.

## Decision
A charge (ADR-0014) is surfaced as a **payment link** `https://<web>/pay/<charge_id>`.

- **Merchant side (authenticated web app):** create a charge → get the link (+ QR of
  the link) to copy and send. This is the main "online POS" action now.
- **Customer side (public, no merchant auth):** opening the link renders a **hosted
  checkout** at `/pay/<charge_id>` — merchant, amount, items, live status — and the
  **SCA step**: approve in the KOTN mobile app via on-device face match (ADR-0006).
  The page hands off to the app (deep link `kotn://pay/<id>` + QR to scan); the
  **on-device biometric approval is the 3DS-equivalent challenge**. The page polls and
  flips to "paid" when the app settles.
- **Money path is unchanged:** the link is just a way to reach an existing charge;
  approval still runs the one `settle()` path (wallet deduct + signed ledger, ADR-0014).

## Why
- Commerce owners need to bill customers who aren't physically present; a link is the
  universal e-commerce primitive.
- Keeping regular users on mobile means the website never holds consumer balances or
  biometrics — the web checkout only **initiates** and **displays**; the **mobile app**
  performs the secure approval. Clean trust boundary.
- "3D Secure" here is framing for the bank pitch: the on-device biometric is the strong
  customer authentication, equivalent to a 3DS challenge, without card rails.

## Consequences
- New **public** route `/pay/[id]` outside the authenticated `(app)` group.
- The merchant POS screen now produces a shareable link (copy + QR) instead of only an
  in-store QR.
- **Demo stand-in:** until the mobile app exists, the checkout page keeps a clearly
  labelled "approve as customer" action (funded customer id) that calls the approve
  endpoint — the stand-in for the mobile 3DS approval.
- A real deployment needs the web origin configured (`NEXT_PUBLIC_WEB_URL`) so links
  are absolute and shareable.

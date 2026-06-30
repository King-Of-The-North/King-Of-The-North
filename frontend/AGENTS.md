<!-- BEGIN:nextjs-agent-rules -->
# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` before writing any code. Heed deprecation notices.
<!-- END:nextjs-agent-rules -->

---

# `frontend/` — King Of The North store + admin (Next.js)

**Read first:** root `../AGENTS.md` (the design pivoted — ADRs win over old docs).

## Scope changed — this is NOT the consumer wallet anymore

The consumer wallet UI **moved to the Expo app** (`mobile/`). This Next.js app is now
the **merchant / operator side**:

1. **Store catalog** — products with scannable **barcodes** + prices. The mobile app's
   scan-and-go reads these. Serves the catalog/price API the app calls.
2. **Admin dashboard** — live view of settlements, the **replicated P2P ledger**, and a
   **"cloud cost avoided"** counter (DePIN pitch metric, ADR-0008).

(Old role — "Wallet Dashboard + POS Simulator" in README/ARCHITECTURE — is superseded.)

## Design system — use it (`@kotn/design-system`)

UI is already built. **Never hardcode colors/fonts/sizes/spacing/radius/easing.**

- Primitives in `src/components/ui/` → `import { Heading, Text, Button, Surface, Input, Amount } from "@/components/ui"`.
- Tokens are CSS vars (`--kotn-*`) from `src/app/tokens.css` + Tailwind utilities (`bg-accent`,
  `text-text-secondary`, `font-sans`, `rounded-lg`) mapped in `globals.css @theme`.
- Tokens come from `packages/design-system` (shared with mobile). Edited tokens →
  regenerate: `pnpm --filter @kotn/design-system build:css`. Don't edit `tokens.css` by hand.
- Money: render with `Amount` (kuruş in), never format by hand.
- See `/styleguide` (`src/app/styleguide/page.tsx`) for every primitive.

## Rules

- No money math here; read authoritative state from the backend via the Gateway REST API.
- Phone-based scan-and-go replaces the physical RFID gate (ADR-0007) — no POS camera/gate
  simulator needed.

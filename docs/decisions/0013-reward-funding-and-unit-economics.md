# ADR-0013: Reward funding and unit economics — rewards backed by real savings only

**Status:** Accepted — 2026-06-30
**Context:** The headline is a Helium/DePIN-style model where customers earn money
without the company subsidizing it: the customer is both the **cashier**
(scan-and-go, ADR-0007) and a **node** (DePIN, ADR-0008), and earns from both. The
trap that kills every DePIN project — including Helium — is paying rewards from
inflation/minting instead of real value, producing a death spiral. This ADR fixes the
funding rule so reward economics are sustainable by construction.

## Decision
**Every reward must be funded by a real, realized saving, and capped at the value the
user actually created. No reward is minted from nothing.**

Real value sources (the only things that fund rewards):
| Source | Who saves | Funds |
|--------|-----------|-------|
| **Merchant fee** | store (no cashier labor + cheaper than card interchange) | user **cashier cashback** |
| **Cloud OPEX avoided** | the company (phones = free infra) | user **node/DePIN reward** |
| **Fixed-pool yield / float** | interest on locked deposits | the **Day-Zero credit** itself |

**The invariant that must always hold, per transaction:**
```
merchant_fee  ≥  user_cashback + node_reward_share + payment_cost + company_margin
```
If the left side ≥ the right, the system pays users *and* profits. If it ever flips,
the company is subsidizing = burning money = death spiral. Rewards are therefore sized
as a **share of realized value, never a fixed promise**.

Design rules:
1. **Reward = share of value the user generated**, not a flat handout (cashback ∝
   purchase volume; node reward ∝ resources served).
2. **Only pay for work actually used** — reward nodes that serve **real traffic**, and
   **cap node rewards at actual cloud cost avoided**. Do not pay idle capacity (Helium's
   mistake).
3. **Undercut, don't overcharge, the merchant** — charge less than their current cost
   (cashier wages + ~1.5–3% interchange) yet still profit, because marginal cost ≈ 0.
4. **Frame as cashback/loyalty, NOT yield** — present rewards as loyalty/cashback for
   contribution, never as investment return, to avoid the e-money/securities
   interpretation (reaffirms ADR-0008's regulatory flag).

## Why
- Tying every payout to a realized saving makes the flywheel self-funding: store saves
  labor → merchant fee → company margin → slice back to the user who *was* the cashier
  and *is* the infra. Nothing is printed.
- Capping rewards at value created is the single constraint separating a sustainable
  flywheel from a Ponzi/token-collapse.

## Consequences
- Requires, in code: **merchant-fee + revenue-split accounting** (who paid, how the
  slice is computed), **contribution metering tied to real served load**, and a
  **unit-economics guardrail** enforcing reward ≤ value created. None built yet.
- `CreditNodeReward` (already built) is the **payout mechanism**; this ADR governs how
  the **amount** is funded and bounded. The Gateway metering layer computes the number;
  the Wallet just credits it (ADR-0008).
- Reward numbers used in the demo are illustrative; real rates need finance + legal
  review post-hackathon.

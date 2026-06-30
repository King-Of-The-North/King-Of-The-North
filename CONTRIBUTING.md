# Contributing

## Prerequisites

- **Node ≥ 22** + **pnpm 10** (`corepack enable`)
- **Go 1.26**
- **uv** (Python)
- **Docker** (Compose)
- Dev tools: **buf**, **go-task** (`task`), **lefthook**, **golangci-lint**
  (`brew install buf go-task lefthook golangci-lint`)

## First-time setup

```bash
cp .env.example .env
task setup        # pnpm install + go mod download + uv sync (installs git hooks too)
```

## Running locally (hybrid model)

```bash
task up           # Postgres + gateway + wallet + ai-biometric in Docker
task dev:wallet   # run the ONE service you're editing natively (hot path)
task dev:mobile   # Expo app   (native)
task dev:frontend # Next.js     (native)
```

Health checks: gateway `:8080/healthz`, wallet `:8081/healthz`, ai-biometric `:8082/healthz`.

## Proto

`proto/wallet.proto` is the cross-service contract — **change it first**, then:

```bash
task proto        # buf lint + generate Go into gen/ (committed)
```

## Quality

```bash
task lint         # Go + Python + JS + proto
task test         # go test + pytest
task fmt          # gofmt + ruff format + prettier
```

Hooks run automatically on commit (lefthook): lint staged files + validate the commit
message.

## Commits & PRs

- **Conventional Commits**: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`, …
  (enforced by commitlint). Example: `feat(wallet): add atomic deduction`.
- Branch off `main`; open a PR using the template. CI runs only the changed-path jobs.
- **Changing direction?** Add an ADR in `docs/decisions/` (see `0001`–`0009`).

## Project map & rules

See root **`AGENTS.md`** (monorepo map, ownership, the design pivot) and
**`docs/plans/`** for service build plans. Money is always integer minor units
(kuruş); the P2P ledger is replicated, never sharded; Moka is mocked behind an interface.

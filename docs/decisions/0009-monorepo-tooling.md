# ADR-0009: Monorepo tooling — pnpm + Taskfile + Compose + Buf + lefthook + Actions

**Status:** Accepted — 2026-06-30
**Context:** Polyglot monorepo (Next.js, Expo/RN, two Go services, one Python service,
shared proto) needs one install, one local stack, codegen, linting, hooks, and CI
before the build window. A teammate owns `mobile/`; the owner drives the rest.

## Decision

| Concern | Choice |
|---------|--------|
| JS deps | **pnpm workspace** (`frontend`, `mobile`); **`.npmrc node-linker=hoisted`** (required for Expo/RN Metro) |
| Task entrypoint | **Taskfile (go-task)** — `task setup/up/down/dev:*/proto/lint/test/fmt` |
| Local orchestration | **Docker Compose** (Postgres + backends); JS app under edit runs **native** (hybrid) |
| Proto | **Buf** — lint + generate Go into `gen/` (committed) |
| Go layout | **single root module** `github.com/king-of-the-north/king-of-the-north`; binaries `*/cmd/<svc>` |
| Python | **uv** + ruff + pytest |
| Git hooks | **lefthook** (pre-commit lint, commit-msg = commitlint / Conventional Commits) |
| CI | **GitHub Actions**, path-filtered jobs (go / python / js / proto) |

## Why
- Hybrid (Compose infra + native app) gives reproducibility for backends and fast
  hot-reload for the app you're editing; Expo can't be containerized for device runs.
- Single Go module keeps two services + generated code simple (no `go.work` juggling).
- Path-filtered CI only builds what changed — fast feedback in a 9-day window.

## Consequences / gotchas
- **`node-linker=hoisted` is mandatory** — pnpm's symlinked store breaks React Native.
- Go commands are **scoped** to `./gateway/... ./wallet/... ./gen/...` so the root
  module doesn't crawl `node_modules` (golangci also excludes JS dirs).
- `gen/` and `pnpm-lock.yaml` are **committed**; `.env`, `.venv`, build dirs ignored.
- Real Moka stays mocked behind an interface (ADR-0002); money stays integer kuruş (ADR-0003).

# `gateway/` — King Of The North API Gateway (Go)

The **REST ingress** for the apps and the **gRPC router** to backend services. In the
CeDeFi design it also hosts the **P2P replicated-ledger nodes + DePIN metering**
(ADR-0005, ADR-0008) — not built yet.

**Read first:** root `../AGENTS.md` (the design pivoted — ADRs win over old docs) and
`../docs/plans/day-zero-wallet.md` (the wallet it talks to).

The gateway holds **no money logic**. Every money operation routes to the Wallet
service over gRPC; the gateway only translates JSON ↔ proto and HTTP ↔ gRPC status.

## Layout

| Path | Role |
|------|------|
| `cmd/gateway/` | boot: dial Wallet, mount routes, `/healthz` |
| `internal/walletclient/` | gRPC client to the Wallet service (`WALLET_GRPC_ADDR`) |
| `internal/httpapi/` | `/v1` REST handlers — JSON↔proto, gRPC-status→HTTP-code |
| `internal/ledgerp2p/` | signed append-only ledger (scaffold — replication TBD) |

## Run the backend stack

Whole backend (postgres + wallet + gateway) in Docker, one command — **this is what
a teammate runs**:

```bash
docker compose up --build      # from the repo root
# REST API → http://localhost:8080
# down + wipe DB:  docker compose down -v
```

Or run the gateway natively against a running wallet (faster iteration):

```bash
docker compose up -d postgres wallet     # deps in Docker
go run ./gateway/cmd/gateway/            # gateway native on :8080
```

Config (env): `GATEWAY_HTTP_PORT` (default `8080`), `WALLET_GRPC_ADDR` (default
`localhost:9091`; in compose it's `wallet:9091`).

## REST routes (`/v1`) → Wallet gRPC

All money is integer minor units (kuruş, ADR-0003). Money never rendered/parsed as
float.

| Method & path | → Wallet RPC | Body / params |
|---------------|--------------|---------------|
| `POST /v1/deposit` | `CalculateLimit` | `{user_id, deposit_minor, [apy, compounding_per_year, lockup_years, risk_margin]}` — Day-Zero params default to the fixed pool (ADR-0001) if omitted |
| `GET /v1/accounts/{id}` | `GetAccount` | path `id` = `user_id` |
| `POST /v1/pay` | `ValidateTransaction` | `{user_id, items:[{sku,name,price_minor,quantity}], other_trx_code}` |
| `POST /v1/node-reward` | `CreditNodeReward` | `{user_id, minor, ref}` — gateway packs `{minor,ref}` into the proof bytes (it is the metering authority, ADR-0008/0013) |

Errors: gRPC `InvalidArgument`→400, `NotFound`→404, `Unavailable`→503, else 500. A
**declined** payment is a normal `200` with `{"approved": false, ...}`, not an error.

### Example

```bash
U=11111111-1111-1111-1111-111111111111
curl -X POST localhost:8080/v1/deposit -d "{\"user_id\":\"$U\",\"deposit_minor\":1000000}"
curl -X POST localhost:8080/v1/pay -d "{\"user_id\":\"$U\",\"items\":[{\"sku\":\"A\",\"name\":\"Milk\",\"price_minor\":250,\"quantity\":2}],\"other_trx_code\":\"trx-1\"}"
curl localhost:8080/v1/accounts/$U
```

## Connecting the apps

- **Mobile (Expo, physical device):** `localhost` is the phone, not your laptop. Use
  the laptop **LAN IP** (`http://192.168.x.x:8080`) or a tunnel (`ngrok http 8080`).
  Android emulator → `10.0.2.2:8080`; iOS simulator → `localhost` works. Read the base
  URL from `EXPO_PUBLIC_API_URL`, don't hardcode.
- **Web frontend (Next.js, browser):** the REST API sends **no CORS headers** yet —
  browser calls will be blocked. CORS middleware is needed before the web admin can
  call the gateway (mobile native fetch is unaffected).

## Conventions

- **Proto first:** any cross-service change starts in `../proto/`, then regen stubs.
- **No ledger logic here** — route to Wallet. The gateway is stateless ingress.
- **Don't run both composes at once:** root `docker-compose.yml` (the app stack) and
  `../wallet/docker-compose.yml` (Postgres-only, for `go test -tags integration`) both
  map host port `5440`.

## Not built yet (next chunks)

- Signed ledger append + P2P replication (`ledgerp2p`) and DePIN metering — the
  "receipt → replicated ledger → earn credit" headline (ADR-0008/0013).
- Auth: device-key registry, face-pay verification, recovery, revocation
  (ADR-0011), KYC mock.
- CORS middleware (for the web frontend).

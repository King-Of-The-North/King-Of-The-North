# Multi-stage build for the Go wallet service. Build context = repo root (single Go
# module shared with gateway + gen/). See docker-compose.yml.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY gen/ ./gen/
COPY wallet/ ./wallet/
RUN CGO_ENABLED=0 go build -trimpath -o /out/wallet ./wallet/cmd/wallet

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/wallet /wallet
EXPOSE 8081
USER nonroot:nonroot
ENTRYPOINT ["/wallet"]

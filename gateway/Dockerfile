# Multi-stage build for the Go gateway service. Build context = repo root (single Go
# module shared with wallet + gen/). See docker-compose.yml.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY gen/ ./gen/
COPY gateway/ ./gateway/
RUN CGO_ENABLED=0 go build -trimpath -o /out/gateway ./gateway/cmd/gateway

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/gateway /gateway
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/gateway"]

// Command gateway is the API Gateway: REST ingress for the apps, gRPC routing to
// backend services, and host of the P2P replicated-ledger nodes + DePIN metering
// (ADR-0005, ADR-0008).
//
// Current milestone: the REST bridge — /v1 routes mapped onto the Wallet gRPC service
// (deposit, account, pay, node-reward). The P2P node registry + DePIN metering are
// wired in a later chunk (docs/plans/mobile-app.md §3).
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/king-of-the-north/king-of-the-north/gateway/internal/httpapi"
	"github.com/king-of-the-north/king-of-the-north/gateway/internal/ledgerp2p"
	"github.com/king-of-the-north/king-of-the-north/gateway/internal/walletclient"
)

func main() {
	addr := ":" + env("GATEWAY_HTTP_PORT", "8080")
	walletAddr := env("WALLET_GRPC_ADDR", "localhost:9091")

	// gRPC client to the Wallet service — every money op routes through it.
	wallet, err := walletclient.Dial(walletAddr)
	if err != nil {
		log.Fatalf("gateway: wallet client: %v", err)
	}
	defer func() { _ = wallet.Close() }()
	log.Printf("gateway: wallet client → %s", walletAddr)

	// P2P ledger node. Ephemeral Ed25519 key generated on boot for the demo (load
	// from a secret in production). Signs each appended payment entry (ADR-0005).
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatalf("gateway: ledger key: %v", err)
	}
	ledger := ledgerp2p.NewMemNode(priv)
	log.Println("gateway: ledger node ready (in-process, signed)")

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","service":"gateway"}`))
	})
	httpapi.New(wallet, ledger).Routes(mux)

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	go func() {
		log.Printf("gateway: listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("gateway: server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("gateway: shutdown error: %v", err)
	}
	log.Println("gateway: stopped")
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

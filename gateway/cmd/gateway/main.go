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
	"strconv"
	"syscall"
	"time"

	"github.com/king-of-the-north/king-of-the-north/gateway/internal/catalog"
	"github.com/king-of-the-north/king-of-the-north/gateway/internal/charges"
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

	// P2P ledger cluster: one always-present anchor (signs) + N simulated phone
	// replicas (full-copy, killable). Ephemeral Ed25519 key generated on boot for the
	// demo (load from a secret in production). ADR-0004 (simulated mesh) + ADR-0005.
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatalf("gateway: ledger key: %v", err)
	}
	replicas := envInt("LEDGER_REPLICAS", 3)
	ledger := ledgerp2p.NewCluster(priv, replicas)
	log.Printf("gateway: ledger cluster ready (anchor + %d replicas, signed)", replicas)

	// DePIN reward economics (ADR-0013): reward per replicated entry stays below the
	// cloud cost it avoids, so payouts are funded by real savings with a company margin.
	depin := httpapi.DepinConfig{
		RewardPerEntryMinor:    int64(envInt("DEPIN_REWARD_PER_ENTRY_MINOR", 5)),
		CloudCostPerEntryMinor: int64(envInt("DEPIN_CLOUD_COST_PER_ENTRY_MINOR", 12)),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","service":"gateway"}`))
	})
	// Online POS: seeded e-commerce merchants + an in-memory charge store (ADR-0014).
	// Merchants carry a phone for WhatsApp-OTP login mapping (ADR-0015).
	chargeStore := charges.NewStore(15*time.Minute,
		charges.Merchant{ID: "mer_demo", Name: "Demo E-Commerce", Phone: "+905550000001"},
		charges.Merchant{ID: "mer_kahve", Name: "Kuzey Kahve", Phone: "+905550000002"},
	)

	// Merchant catalog: seeded products (barcode → price) for scan-and-go (ADR-0007).
	catalogStore := catalog.NewStore(
		catalog.Product{MerchantID: "mer_kahve", Barcode: "8690000000017", Name: "Latte", PriceMinor: 5500},
		catalog.Product{MerchantID: "mer_kahve", Barcode: "8690000000024", Name: "Filter Coffee", PriceMinor: 4500},
		catalog.Product{MerchantID: "mer_demo", Barcode: "8690000000031", Name: "T-Shirt", PriceMinor: 29900},
	)

	// The device registry now lives in the wallet (Postgres), reached via the wallet
	// client — so device bindings survive restarts and are authoritative.
	api := httpapi.New(wallet, ledger, chargeStore, catalogStore, depin)
	api.Routes(mux)

	// Auto-settle DePIN rewards on a timer so the demo runs itself (0/empty = off, the
	// manual POST /v1/depin/settle still works). Stops when rootCtx is cancelled.
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()
	if interval := envDuration("DEPIN_AUTO_SETTLE_INTERVAL", 0); interval > 0 {
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			log.Printf("gateway: DePIN auto-settle every %s", interval)
			for {
				select {
				case <-rootCtx.Done():
					return
				case <-ticker.C:
					if paid, avoided := api.AutoSettle(rootCtx); paid > 0 {
						log.Printf("gateway: auto-settled %d kuruş (cloud avoided %d)", paid, avoided)
					}
				}
			}
		}()
	}

	// CORS so the browser-based merchant/admin web app can call the Gateway (ADR-0014).
	corsOrigin := env("GATEWAY_CORS_ORIGIN", "*")
	srv := &http.Server{Addr: addr, Handler: httpapi.CORS(corsOrigin, mux), ReadHeaderTimeout: 5 * time.Second}

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

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// envDuration parses a Go duration (e.g. "30s", "5m"); fallback on empty/invalid.
func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

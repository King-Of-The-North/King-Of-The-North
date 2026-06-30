// Command wallet is the Day-Zero Yield wallet/ledger service.
//
// Boots the WalletService gRPC server (CalculateLimit, GetAccount,
// ValidateTransaction, CreditNodeReward — see proto/wallet.proto) over the
// PostgreSQL ledger, plus a side HTTP /healthz endpoint. The Moka integration runs
// behind the Mock client until sandbox credentials land (ADR-0002).
package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	walletv1 "github.com/king-of-the-north/king-of-the-north/gen"
	"github.com/king-of-the-north/king-of-the-north/wallet/internal/moka"
	"github.com/king-of-the-north/king-of-the-north/wallet/internal/service"
	"github.com/king-of-the-north/king-of-the-north/wallet/internal/store"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	grpcAddr := ":" + env("WALLET_GRPC_PORT", "9091")
	httpAddr := ":" + env("WALLET_HTTP_PORT", "8081")
	dsn := env("WALLET_DSN", "postgres://kotn:kotn@localhost:5440/kotn_wallet?sslmode=disable")

	ctx := context.Background()

	// Ledger store: open pool, apply schema (idempotent — safe each boot).
	st, err := store.New(ctx, dsn)
	if err != nil {
		log.Fatalf("wallet: store: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		log.Fatalf("wallet: migrate: %v", err)
	}
	log.Println("wallet: ledger ready")

	// gRPC server over the ledger + Moka mock.
	grpcSrv := grpc.NewServer()
	walletv1.RegisterWalletServiceServer(grpcSrv, service.New(st, moka.Mock{}))
	reflection.Register(grpcSrv) // lets grpcurl introspect without proto files

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("wallet: listen: %v", err)
	}
	go func() {
		log.Printf("wallet: gRPC listening on %s", grpcAddr)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("wallet: grpc serve: %v", err)
		}
	}()

	// Side HTTP health endpoint.
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","service":"wallet"}`))
	})
	httpSrv := &http.Server{Addr: httpAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("wallet: HTTP health on %s", httpAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("wallet: http error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("wallet: shutting down")
	grpcSrv.GracefulStop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	log.Println("wallet: stopped")
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

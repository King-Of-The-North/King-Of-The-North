// Package store is the PostgreSQL ledger — the authoritative money store (ADR-0005).
// All money is integer minor units (kuruş, ADR-0003). The P2P layer only replicates
// a signed audit log; balances live here and in Moka, never on the phone.
package store

import (
	"context"
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store wraps a pgx connection pool over the wallet ledger.
type Store struct {
	pool *pgxpool.Pool
}

// New opens a pool against dsn and verifies connectivity.
func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// Migrate applies the embedded schema. Migrations are idempotent (IF NOT EXISTS),
// so this is safe to run on every boot for the demo.
func (s *Store) Migrate(ctx context.Context) error {
	sql, err := migrationsFS.ReadFile("migrations/0001_init.sql")
	if err != nil {
		return fmt.Errorf("store: read migration: %w", err)
	}
	if _, err := s.pool.Exec(ctx, string(sql)); err != nil {
		return fmt.Errorf("store: apply migration: %w", err)
	}
	return nil
}

// ErrAccountNotFound is returned when a user has no account row.
var ErrAccountNotFound = fmt.Errorf("store: account not found")

// errNoRows lets callers map pgx's sentinel without importing pgx.
func isNoRows(err error) bool { return err == pgx.ErrNoRows }

package store

import (
	"context"
	"fmt"
	"time"
)

// Device is a stored device binding — a phone's Ed25519 public key for a user. Only the
// public key is kept; the private key never leaves the phone (ADR-0006/0010).
type Device struct {
	PubKey    []byte
	Active    bool
	CreatedAt string // RFC3339 (formatted by the service layer)
}

// EnrollDevice binds a device public key to a user, marking it active. Idempotent:
// re-enrolling an existing (possibly revoked) key reactivates it.
func (s *Store) EnrollDevice(ctx context.Context, userID string, pub []byte) error {
	const q = `
INSERT INTO devices (user_id, device_pubkey, status, created_at)
VALUES ($1, $2, 1, now())
ON CONFLICT (user_id, device_pubkey) DO UPDATE SET status = 1, revoked_at = NULL`
	if _, err := s.pool.Exec(ctx, q, userID, pub); err != nil {
		if isInvalidText(err) {
			return ErrAccountNotFound // malformed user id
		}
		return fmt.Errorf("store: enroll device: %w", err)
	}
	return nil
}

// ListActiveDevices returns a user's active device public keys.
func (s *Store) ListActiveDevices(ctx context.Context, userID string) ([]Device, error) {
	const q = `
SELECT device_pubkey, created_at FROM devices
WHERE user_id = $1 AND status = 1 ORDER BY created_at`
	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("store: list devices: %w", err)
	}
	defer rows.Close()

	out := make([]Device, 0)
	for rows.Next() {
		var d Device
		var created time.Time
		if err := rows.Scan(&d.PubKey, &created); err != nil {
			return nil, fmt.Errorf("store: list devices scan: %w", err)
		}
		d.Active = true
		d.CreatedAt = created.Format(time.RFC3339)
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		if isInvalidText(err) {
			return []Device{}, nil // malformed user id → no devices
		}
		return nil, fmt.Errorf("store: list devices rows: %w", err)
	}
	return out, nil
}

// RevokeDevice marks a single device key revoked (e.g. a lost or replaced phone).
func (s *Store) RevokeDevice(ctx context.Context, userID string, pub []byte) error {
	const q = `UPDATE devices SET status = 2, revoked_at = now() WHERE user_id = $1 AND device_pubkey = $2`
	if _, err := s.pool.Exec(ctx, q, userID, pub); err != nil {
		if isInvalidText(err) {
			return ErrAccountNotFound
		}
		return fmt.Errorf("store: revoke device: %w", err)
	}
	return nil
}

// RebindDevices is account recovery (ADR-0011): revoke every existing device for the
// user and enroll a new one, atomically. Money is anchored to user_id and untouched — a
// new phone takes over without moving the balance.
func (s *Store) RebindDevices(ctx context.Context, userID string, newPub []byte) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: rebind begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`UPDATE devices SET status = 2, revoked_at = now() WHERE user_id = $1 AND status = 1`,
		userID); err != nil {
		if isInvalidText(err) {
			return ErrAccountNotFound
		}
		return fmt.Errorf("store: rebind revoke: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO devices (user_id, device_pubkey, status, created_at)
VALUES ($1, $2, 1, now())
ON CONFLICT (user_id, device_pubkey) DO UPDATE SET status = 1, revoked_at = NULL`,
		userID, newPub); err != nil {
		return fmt.Errorf("store: rebind enroll: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: rebind commit: %w", err)
	}
	return nil
}

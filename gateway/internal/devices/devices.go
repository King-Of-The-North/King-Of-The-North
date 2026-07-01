// Package devices is the gateway's device-key registry: it binds each user to the
// Ed25519 public keys of the phones they've enrolled. The P2P WebSocket handshake uses
// it to prove a connecting phone (a) holds the private key for a device pubkey and
// (b) that pubkey is enrolled for the user it claims — so a phone can't replicate the
// ledger (and earn DePIN credit) as someone else.
//
// The biometric never reaches here (ADR-0006/0010): only device keys move. Storage is
// in-memory for the demo, consistent with the other gateway stores; a production build
// would persist bindings and gate enrollment behind KYC/OTP (ADR-0011).
package devices

import (
	"crypto/ed25519"
	"errors"
	"sync"
)

// ErrInvalidKey means the supplied bytes are not a valid Ed25519 public key.
var ErrInvalidKey = errors.New("devices: invalid device public key")

// Store binds user IDs to their enrolled device public keys.
type Store struct {
	mu sync.RWMutex
	// keyed by user_id -> set of device pubkeys (hex/opaque string of the raw key bytes).
	byUser map[string]map[string]ed25519.PublicKey
}

// NewStore builds an empty registry.
func NewStore() *Store {
	return &Store{byUser: make(map[string]map[string]ed25519.PublicKey)}
}

// Enroll binds a device public key to a user. Idempotent — re-enrolling the same key is
// a no-op. Returns ErrInvalidKey if pub is not a well-formed Ed25519 key.
func (s *Store) Enroll(userID string, pub ed25519.PublicKey) error {
	if len(pub) != ed25519.PublicKeySize {
		return ErrInvalidKey
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	set, ok := s.byUser[userID]
	if !ok {
		set = make(map[string]ed25519.PublicKey)
		s.byUser[userID] = set
	}
	set[string(pub)] = pub
	return nil
}

// IsEnrolled reports whether pub is enrolled for userID.
func (s *Store) IsEnrolled(userID string, pub ed25519.PublicKey) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	set, ok := s.byUser[userID]
	if !ok {
		return false
	}
	_, ok = set[string(pub)]
	return ok
}

// Count returns how many devices a user has enrolled (for the dashboard/tests).
func (s *Store) Count(userID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byUser[userID])
}

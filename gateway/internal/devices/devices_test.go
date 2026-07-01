package devices

import (
	"crypto/ed25519"
	"testing"
)

func TestEnrollAndVerify(t *testing.T) {
	s := NewStore()
	pub, _, _ := ed25519.GenerateKey(nil)
	other, _, _ := ed25519.GenerateKey(nil)

	if s.IsEnrolled("u1", pub) {
		t.Fatal("key enrolled before Enroll")
	}
	if err := s.Enroll("u1", pub); err != nil {
		t.Fatalf("enroll: %v", err)
	}
	if !s.IsEnrolled("u1", pub) {
		t.Fatal("key not enrolled after Enroll")
	}
	// Bound to the user: same key, different user is not enrolled.
	if s.IsEnrolled("u2", pub) {
		t.Fatal("key leaked to another user")
	}
	// A different key for the same user is not enrolled.
	if s.IsEnrolled("u1", other) {
		t.Fatal("unrelated key reported enrolled")
	}
}

func TestEnrollIdempotentAndCount(t *testing.T) {
	s := NewStore()
	pub, _, _ := ed25519.GenerateKey(nil)
	_ = s.Enroll("u1", pub)
	_ = s.Enroll("u1", pub) // idempotent
	if got := s.Count("u1"); got != 1 {
		t.Fatalf("want 1 device, got %d", got)
	}
	pub2, _, _ := ed25519.GenerateKey(nil)
	_ = s.Enroll("u1", pub2)
	if got := s.Count("u1"); got != 2 {
		t.Fatalf("want 2 devices, got %d", got)
	}
}

func TestEnrollInvalidKey(t *testing.T) {
	s := NewStore()
	if err := s.Enroll("u1", ed25519.PublicKey([]byte("too-short"))); err != ErrInvalidKey {
		t.Fatalf("want ErrInvalidKey, got %v", err)
	}
}

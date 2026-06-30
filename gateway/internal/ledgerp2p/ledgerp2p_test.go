package ledgerp2p

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
)

func newNode(t *testing.T) Node {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	return NewMemNode(priv)
}

func TestAppendAndVerify(t *testing.T) {
	n := newNode(t)
	for i := 0; i < 5; i++ {
		if _, err := n.AppendPayment("user-1", 1000, []string{"1x Milk @1000"}, "moka-x", "trx-x"); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	if n.Len() != 5 {
		t.Fatalf("want 5 entries, got %d", n.Len())
	}
	if err := n.Verify(); err != nil {
		t.Fatalf("verify clean chain: %v", err)
	}
}

func TestChainLinks(t *testing.T) {
	n := newNode(t)
	e0, _ := n.AppendPayment("u", 100, nil, "m0", "t0")
	e1, _ := n.AppendPayment("u", 200, nil, "m1", "t1")

	if len(e0.Content.PrevHash) != 0 {
		t.Fatalf("genesis prev_hash should be empty")
	}
	if string(e1.Content.PrevHash) != string(e0.Hash) {
		t.Fatalf("entry 1 prev_hash must equal entry 0 hash")
	}
	if e1.Content.Seq != 1 {
		t.Fatalf("want seq 1, got %d", e1.Content.Seq)
	}
}

// TestTamperDetected mutates a committed entry's amount; Verify must fail because the
// signature no longer matches the canonical content.
func TestTamperDetected(t *testing.T) {
	n := newNode(t).(*memNode)
	_, _ = n.AppendPayment("u", 100, nil, "m0", "t0")
	_, _ = n.AppendPayment("u", 200, nil, "m1", "t1")
	if err := n.Verify(); err != nil {
		t.Fatalf("precondition: clean chain should verify: %v", err)
	}

	n.entries[0].Content.AmountMinor = 999999 // forge the amount

	err := n.Verify()
	if err == nil {
		t.Fatal("tampered chain must not verify")
	}
	if !errors.Is(err, ErrTampered) {
		t.Fatalf("want ErrTampered, got %v", err)
	}
}

// TestForeignSignatureRejected proves an entry signed by a different key fails Verify
// (no one but the node's key can author valid entries).
func TestForeignSignatureRejected(t *testing.T) {
	n := newNode(t).(*memNode)
	_, _ = n.AppendPayment("u", 100, nil, "m0", "t0")

	_, otherPriv, _ := ed25519.GenerateKey(rand.Reader)
	cb, _ := canonicalBytes(n.entries[0].Content)
	n.entries[0].Sig = ed25519.Sign(otherPriv, cb)
	n.entries[0].Hash = entryHash(cb, n.entries[0].Sig)

	if err := n.Verify(); !errors.Is(err, ErrTampered) {
		t.Fatalf("foreign signature must be rejected, got %v", err)
	}
}

package ledgerp2p

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
)

func newCluster(t *testing.T, replicas int) *Cluster {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	return NewCluster(priv, replicas)
}

// TestReplicationKeepsAllNodesInSync: every append lands on the anchor and all replicas.
func TestReplicationKeepsAllNodesInSync(t *testing.T) {
	c := newCluster(t, 3)
	for i := 0; i < 4; i++ {
		if _, err := c.AppendPayment("u", 100, nil, "m", "t"); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	if err := c.Verify(); err != nil {
		t.Fatalf("cluster verify: %v", err)
	}
	for _, n := range c.Nodes() {
		if n.Length != 4 || !n.InSync || !n.Verified {
			t.Fatalf("node %d/%s not in sync: %+v", n.ID, n.Role, n)
		}
	}
}

// TestKillReplicaZeroLoss: killing a replica loses nothing — the anchor and remaining
// replicas keep the full chain, and new appends still replicate (ADR-0005).
func TestKillReplicaZeroLoss(t *testing.T) {
	c := newCluster(t, 3)
	c.AppendPayment("u", 100, nil, "m", "t") //nolint:errcheck
	c.AppendPayment("u", 200, nil, "m", "t") //nolint:errcheck

	// nodes: anchor + replicas 0,1,2. Kill replica 1.
	if !c.KillReplica(1) {
		t.Fatal("expected replica 1 to exist")
	}
	if c.KillReplica(1) {
		t.Fatal("killing an already-dead replica should return false")
	}

	// chain intact, still verifies, new append fans out to survivors.
	if c.Len() != 2 {
		t.Fatalf("anchor lost data: len %d", c.Len())
	}
	if _, err := c.AppendPayment("u", 300, nil, "m", "t"); err != nil {
		t.Fatalf("append after kill: %v", err)
	}
	if err := c.Verify(); err != nil {
		t.Fatalf("verify after kill: %v", err)
	}
	for _, n := range c.Nodes() {
		if n.Length != 3 || !n.InSync {
			t.Fatalf("survivor node %d/%s out of sync: %+v", n.ID, n.Role, n)
		}
	}
}

// TestAddReplicaBackfills: a replica added late catches up to the full chain.
func TestAddReplicaBackfills(t *testing.T) {
	c := newCluster(t, 1)
	for i := 0; i < 3; i++ {
		c.AppendPayment("u", 100, nil, "m", "t") //nolint:errcheck
	}
	c.AddReplica("")
	if err := c.Verify(); err != nil {
		t.Fatalf("verify after add: %v", err)
	}
	nodes := c.Nodes()
	last := nodes[len(nodes)-1]
	if last.Length != 3 || !last.InSync {
		t.Fatalf("new replica did not back-fill: %+v", last)
	}
}

// TestReplicaCannotAuthor: a verify-only replica refuses to sign new entries.
func TestReplicaCannotAuthor(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	pub := priv.Public().(ed25519.PublicKey)
	r := NewReplica(pub)
	if _, err := r.AppendPayment("u", 100, nil, "m", "t"); !errors.Is(err, ErrNotAuthor) {
		t.Fatalf("replica must not author, got %v", err)
	}
}

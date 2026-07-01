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

// TestClearPendingSubtractsAndClamps: ClearPending subtracts only the credited units
// (preserving contribution accrued during settlement) and never goes negative.
func TestClearPendingSubtractsAndClamps(t *testing.T) {
	c := newCluster(t, 0)
	c.AddReplica("owner-1")
	id := c.Nodes()[1].ID // first replica

	for i := 0; i < 3; i++ {
		c.AppendPayment("u", 100, nil, "m", "t") //nolint:errcheck
	}
	if c.Meter()[0].Pending != 3 {
		t.Fatalf("want pending 3, got %d", c.Meter()[0].Pending)
	}

	// Credit 2 units; a 3rd pay lands "during" settlement. Pending must end at 2, not 0.
	c.ClearPending(id, 2)
	c.AppendPayment("u", 100, nil, "m", "t") //nolint:errcheck
	if got := c.Meter()[0].Pending; got != 2 {
		t.Fatalf("want pending 2 after clear(2)+1 pay, got %d", got)
	}

	// Over-clear must clamp to 0, never negative.
	c.ClearPending(id, 999)
	if got := c.Meter()[0].Pending; got != 0 {
		t.Fatalf("want pending 0 after over-clear, got %d", got)
	}
}

func findMeter(c *Cluster, owner string) NodeMeter {
	for _, m := range c.Meter() {
		if m.Owner == owner {
			return m
		}
	}
	return NodeMeter{}
}

// TestWSReplicaBackfillPushAck: a remote phone replica gets the chain on connect
// (back-fill = lifetime, not pending), receives live entries on its channel, and meters
// a rewardable contribution only when it ACKs a live entry (proof of replication).
func TestWSReplicaBackfillPushAck(t *testing.T) {
	c := newCluster(t, 0)                     // anchor only
	c.AppendPayment("u", 100, nil, "m", "t0") //nolint:errcheck
	c.AppendPayment("u", 100, nil, "m", "t1") //nolint:errcheck

	id, backfill, ch := c.AddWSReplica("owner1")
	if len(backfill) != 2 {
		t.Fatalf("want 2 backfill entries, got %d", len(backfill))
	}
	// Ack the two back-filled entries — lifetime already counts them, pending stays 0.
	c.AckWSReplica(id, 0)
	c.AckWSReplica(id, 1)
	if m := findMeter(c, "owner1"); m.Pending != 0 || m.Lifetime != 2 {
		t.Fatalf("after backfill acks want pending 0 lifetime 2, got %d/%d", m.Pending, m.Lifetime)
	}

	// A live append is pushed to the phone's channel.
	if _, err := c.AppendPayment("u", 100, nil, "m", "t2"); err != nil {
		t.Fatalf("append: %v", err)
	}
	select {
	case got := <-ch:
		if got.Content.Seq != 2 {
			t.Fatalf("want live seq 2 on channel, got %d", got.Content.Seq)
		}
	default:
		t.Fatal("live entry was not pushed to the ws channel")
	}
	// Acking the live entry meters a rewardable contribution.
	c.AckWSReplica(id, 2)
	if m := findMeter(c, "owner1"); m.Pending != 1 || m.Lifetime != 3 {
		t.Fatalf("after live ack want pending 1 lifetime 3, got %d/%d", m.Pending, m.Lifetime)
	}

	// Nodes() shows it as a remote node whose length is what it has acked.
	var found bool
	for _, n := range c.Nodes() {
		if n.ID == id {
			found = true
			if !n.Remote || n.Length != 3 || !n.InSync {
				t.Fatalf("bad remote node status: %+v", n)
			}
		}
	}
	if !found {
		t.Fatal("ws replica missing from Nodes()")
	}
}

// TestWSReplicaOutOfOrderAckIgnored: acks must follow the chain order; a skip is ignored.
func TestWSReplicaOutOfOrderAckIgnored(t *testing.T) {
	c := newCluster(t, 0)
	id, _, _ := c.AddWSReplica("o")
	c.AppendPayment("u", 100, nil, "m", "t0") //nolint:errcheck

	c.AckWSReplica(id, 1) // out of order (expected 0) → ignored
	if m := findMeter(c, "o"); m.Pending != 0 || m.Lifetime != 0 {
		t.Fatalf("out-of-order ack was counted: %+v", m)
	}
	c.AckWSReplica(id, 0) // in order
	if m := findMeter(c, "o"); m.Pending != 1 {
		t.Fatalf("want pending 1 after in-order ack, got %d", m.Pending)
	}
}

// TestWSReplicaKillClosesChannel: killing a phone closes its push channel and later
// appends don't panic (proves the send/close race is guarded by the cluster mutex).
func TestWSReplicaKillClosesChannel(t *testing.T) {
	c := newCluster(t, 0)
	id, _, ch := c.AddWSReplica("o")
	if !c.KillReplica(id) {
		t.Fatal("kill returned false")
	}
	if _, open := <-ch; open {
		t.Fatal("push channel not closed after kill")
	}
	c.AppendPayment("u", 100, nil, "m", "t") //nolint:errcheck // must not panic
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

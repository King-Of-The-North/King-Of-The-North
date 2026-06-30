package ledgerp2p

import (
	"crypto/ed25519"
	"fmt"
	"sync"
)

// Cluster is a simulated P2P mesh (ADR-0004 pattern): one always-present anchor node
// that signs + authors entries, plus N killable replica nodes (the simulated phones)
// that each keep a FULL copy. On append the anchor signs once and fans the entry out
// to every live replica. Because every node holds the full chain, killing any replica
// loses nothing (ADR-0005). Transport is in-process now, swappable later.
//
// Cluster satisfies Node (it presents the anchor's view) plus replication controls.
type Cluster struct {
	mu       sync.RWMutex
	anchor   Node
	replicas []*replica
	nextID   int
}

type replica struct {
	id   int
	node Node
}

// NewCluster builds a cluster: an anchor signing with priv and replicaCount replicas.
func NewCluster(priv ed25519.PrivateKey, replicaCount int) *Cluster {
	anchor := NewMemNode(priv)
	c := &Cluster{anchor: anchor}
	for i := 0; i < replicaCount; i++ {
		c.addReplica()
	}
	return c
}

// addReplica registers a fresh replica and back-fills it with the current chain so it
// is immediately in sync. Caller holds c.mu, or it is called pre-publication.
func (c *Cluster) addReplica() {
	r := &replica{id: c.nextID, node: NewReplica(c.anchor.PublicKey())}
	c.nextID++
	for _, e := range c.anchor.Entries() {
		_ = r.node.Receive(e) // fresh replica, chain is valid by construction
	}
	c.replicas = append(c.replicas, r)
}

// AppendPayment signs+stores on the anchor, then replicates to every live replica. A
// replica that fails to apply does not block the append — the anchor (and the other
// replicas) still hold the entry, so there is no data loss.
func (c *Cluster) AppendPayment(userID string, amountMinor int64, items []string, mokaRef, otherTrxCode string) (Entry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, err := c.anchor.AppendPayment(userID, amountMinor, items, mokaRef, otherTrxCode)
	if err != nil {
		return Entry{}, err
	}
	for _, r := range c.replicas {
		_ = r.node.Receive(e)
	}
	return e, nil
}

// Receive replicates an externally-authored entry to the anchor and replicas. Present
// for interface completeness; the gateway authors via AppendPayment.
func (c *Cluster) Receive(e Entry) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.anchor.Receive(e); err != nil {
		return err
	}
	for _, r := range c.replicas {
		_ = r.node.Receive(e)
	}
	return nil
}

// Verify checks the anchor and every replica, and that all live nodes agree on length.
func (c *Cluster) Verify() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.anchor.Verify(); err != nil {
		return fmt.Errorf("anchor: %w", err)
	}
	want := c.anchor.Len()
	for _, r := range c.replicas {
		if err := r.node.Verify(); err != nil {
			return fmt.Errorf("replica %d: %w", r.id, err)
		}
		if r.node.Len() != want {
			return fmt.Errorf("%w: replica %d length %d != anchor %d", ErrTampered, r.id, r.node.Len(), want)
		}
	}
	return nil
}

func (c *Cluster) Entries() []Entry             { return c.anchor.Entries() }
func (c *Cluster) Len() int                     { return c.anchor.Len() }
func (c *Cluster) PublicKey() ed25519.PublicKey { return c.anchor.PublicKey() }

// NodeStatus is a per-node view for the replication dashboard.
type NodeStatus struct {
	ID       int    `json:"id"`
	Role     string `json:"role"` // "anchor" | "replica"
	Length   int    `json:"length"`
	InSync   bool   `json:"in_sync"`
	Verified bool   `json:"verified"`
}

// Nodes reports the state of every node in the cluster (anchor first).
func (c *Cluster) Nodes() []NodeStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	want := c.anchor.Len()
	out := []NodeStatus{{
		ID:       -1,
		Role:     "anchor",
		Length:   want,
		InSync:   true,
		Verified: c.anchor.Verify() == nil,
	}}
	for _, r := range c.replicas {
		out = append(out, NodeStatus{
			ID:       r.id,
			Role:     "replica",
			Length:   r.node.Len(),
			InSync:   r.node.Len() == want,
			Verified: r.node.Verify() == nil,
		})
	}
	return out
}

// KillReplica removes a replica by id, modelling a phone going offline. Returns false
// if no such replica. The ledger survives — every other node keeps the full copy.
func (c *Cluster) KillReplica(id int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, r := range c.replicas {
		if r.id == id {
			c.replicas = append(c.replicas[:i], c.replicas[i+1:]...)
			return true
		}
	}
	return false
}

// AddReplica brings a new replica online (back-filled to the current chain).
func (c *Cluster) AddReplica() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.addReplica()
}

// compile-time check: a Cluster is usable anywhere a Node is.
var _ Node = (*Cluster)(nil)

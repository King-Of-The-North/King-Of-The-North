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
	id    int
	owner string // user_id whose phone runs this node; "" = unowned infra node
	node  Node
	// pending = entries replicated since the last DePIN settle (the contribution to
	// be rewarded); lifetime = entries ever replicated (for the dashboard).
	pending  int
	lifetime int
}

// NewCluster builds a cluster: an anchor signing with priv and replicaCount unowned
// infra replicas. Owned (user phone) replicas are added later via AddReplica.
func NewCluster(priv ed25519.PrivateKey, replicaCount int) *Cluster {
	anchor := NewMemNode(priv)
	c := &Cluster{anchor: anchor}
	for i := 0; i < replicaCount; i++ {
		c.addReplica("")
	}
	return c
}

// addReplica registers a fresh replica owned by owner ("" = unowned) and back-fills it
// with the current chain so it is immediately in sync. Back-filled entries count as
// lifetime contribution but not pending (a node is rewarded for live replication, not
// for catching up). Caller holds c.mu, or it is called pre-publication.
func (c *Cluster) addReplica(owner string) {
	r := &replica{id: c.nextID, owner: owner, node: NewReplica(c.anchor.PublicKey())}
	c.nextID++
	for _, e := range c.anchor.Entries() {
		_ = r.node.Receive(e) // fresh replica, chain is valid by construction
		r.lifetime++
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
		if err := r.node.Receive(e); err == nil {
			r.pending++ // metered contribution: this node replicated a live entry
			r.lifetime++
		}
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
	Owner    string `json:"owner,omitempty"`
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
			Owner:    r.owner,
			Role:     "replica",
			Length:   r.node.Len(),
			InSync:   r.node.Len() == want,
			Verified: r.node.Verify() == nil,
		})
	}
	return out
}

// NodeMeter is a per-node contribution view for the DePIN dashboard.
type NodeMeter struct {
	ID       int    `json:"id"`
	Owner    string `json:"owner"`
	Pending  int    `json:"pending"`  // entries replicated since last settle (rewardable)
	Lifetime int    `json:"lifetime"` // entries ever replicated
}

// Meter reports per-replica contribution (owned replicas only — unowned infra nodes
// earn nothing).
func (c *Cluster) Meter() []NodeMeter {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]NodeMeter, 0, len(c.replicas))
	for _, r := range c.replicas {
		if r.owner == "" {
			continue
		}
		out = append(out, NodeMeter{ID: r.id, Owner: r.owner, Pending: r.pending, Lifetime: r.lifetime})
	}
	return out
}

// ClearPending subtracts up to units from a replica's pending meter, called only
// AFTER that contribution has been successfully credited. Subtracting (rather than
// zeroing) preserves any contribution that accrued during settlement, and not calling
// it on a failed credit leaves the units pending to be retried — so a reward is never
// lost or double-paid. No-op if the node is gone.
func (c *Cluster) ClearPending(id, units int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, r := range c.replicas {
		if r.id == id {
			if units > r.pending {
				units = r.pending
			}
			r.pending -= units
			return
		}
	}
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

// AddReplica brings a new replica online for owner ("" = unowned infra), back-filled
// to the current chain.
func (c *Cluster) AddReplica(owner string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.addReplica(owner)
}

// compile-time check: a Cluster is usable anywhere a Node is.
var _ Node = (*Cluster)(nil)

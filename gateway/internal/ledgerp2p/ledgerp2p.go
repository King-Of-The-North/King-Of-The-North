// Package ledgerp2p hosts the replicated, signed, append-only transaction ledger
// (the CeDeFi "De" layer). Nodes keep a FULL copy and verify/replicate — they never
// custody balances, so a lost node loses nothing (ADR-0005). Client phones run nodes
// and earn credit for it (DePIN, ADR-0008).
//
// Phase A (this implementation): a single in-process, hash-chained, Ed25519-signed
// ledger. Each successful payment appends one entry; the chain is re-walkable to
// detect tampering. Multi-node replication and DePIN metering build on top of this.
//
// Money is never held here — entries are a signed audit log. Authoritative balances
// live in Postgres + Moka (ADR-0005); this records what happened, not who owns what.
package ledgerp2p

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Content is the signed payload of a ledger entry — the facts of one transaction.
// Field order is fixed: it defines the canonical bytes that get hashed and signed.
type Content struct {
	Seq          uint64    `json:"seq"`
	PrevHash     []byte    `json:"prev_hash"`
	UserID       string    `json:"user_id"`
	AmountMinor  int64     `json:"amount_minor"` // kuruş (ADR-0003)
	Items        []string  `json:"items"`
	MokaRef      string    `json:"moka_ref"`
	OtherTrxCode string    `json:"other_trx_code"`
	Timestamp    time.Time `json:"timestamp"`
}

// Entry is one hash-chained, signed ledger record: the content, its Ed25519
// signature, and the chain hash (SHA256 over canonical content + signature). The next
// entry's Content.PrevHash equals this entry's Hash.
type Entry struct {
	Content Content `json:"content"`
	Sig     []byte  `json:"sig"`
	Hash    []byte  `json:"hash"`
}

// canonicalBytes is the deterministic encoding signed and hashed. Go marshals struct
// fields in declaration order, so this is stable for a given Content.
func canonicalBytes(c Content) ([]byte, error) {
	return json.Marshal(c)
}

// entryHash chains the log: SHA256(canonical(content) || sig).
func entryHash(content []byte, sig []byte) []byte {
	h := sha256.New()
	h.Write(content)
	h.Write(sig)
	return h.Sum(nil)
}

// Node is a single replica of the full ledger.
type Node interface {
	// AppendPayment builds, signs, chains, and stores one entry. The node fills Seq,
	// PrevHash, and Timestamp; the caller supplies the transaction facts. Only a
	// signing node (the anchor) can author; replicas return ErrNotAuthor.
	AppendPayment(userID string, amountMinor int64, items []string, mokaRef, otherTrxCode string) (Entry, error)
	// Receive validates a pre-signed entry (signature + chain link) and stores it.
	// Replicas use this to replicate the anchor's entries.
	Receive(e Entry) error
	// Verify re-walks the whole chain: every signature valid, every link intact.
	Verify() error
	// Entries returns a snapshot copy of the log.
	Entries() []Entry
	// Len is the number of entries.
	Len() int
	// PublicKey is the verifying key for this node's entries.
	PublicKey() ed25519.PublicKey
}

// memNode is an in-memory, mutex-guarded Node. Transport/replication stay swappable
// on top of this (ADR-0005).
type memNode struct {
	mu      sync.RWMutex
	priv    ed25519.PrivateKey
	pub     ed25519.PublicKey
	entries []Entry
}

// NewMemNode builds an in-memory signing node (the anchor) signing with priv.
func NewMemNode(priv ed25519.PrivateKey) Node {
	return &memNode{
		priv: priv,
		pub:  priv.Public().(ed25519.PublicKey),
	}
}

// NewReplica builds a verify-only node that replicates entries authored by the holder
// of authorPub. It cannot sign new entries — it only Receives the anchor's (a phone
// node in the DePIN model: full copy, verify + replicate, never custody — ADR-0005).
func NewReplica(authorPub ed25519.PublicKey) Node {
	return &memNode{pub: authorPub}
}

// ErrNotAuthor is returned when a replica is asked to author an entry.
var ErrNotAuthor = errors.New("ledgerp2p: node cannot author entries (replica)")

func (n *memNode) AppendPayment(userID string, amountMinor int64, items []string, mokaRef, otherTrxCode string) (Entry, error) {
	if n.priv == nil {
		return Entry{}, ErrNotAuthor
	}
	n.mu.Lock()
	defer n.mu.Unlock()

	var prevHash []byte
	if len(n.entries) > 0 {
		prevHash = n.entries[len(n.entries)-1].Hash
	}

	content := Content{
		Seq:          uint64(len(n.entries)),
		PrevHash:     prevHash,
		UserID:       userID,
		AmountMinor:  amountMinor,
		Items:        items,
		MokaRef:      mokaRef,
		OtherTrxCode: otherTrxCode,
		Timestamp:    time.Now().UTC(),
	}
	cb, err := canonicalBytes(content)
	if err != nil {
		return Entry{}, fmt.Errorf("ledgerp2p: canonical: %w", err)
	}
	sig := ed25519.Sign(n.priv, cb)
	e := Entry{Content: content, Sig: sig, Hash: entryHash(cb, sig)}
	n.entries = append(n.entries, e)
	return e, nil
}

func (n *memNode) Receive(e Entry) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if e.Content.Seq != uint64(len(n.entries)) {
		return fmt.Errorf("%w: out-of-order entry seq %d (have %d)", ErrTampered, e.Content.Seq, len(n.entries))
	}
	var prevHash []byte
	if len(n.entries) > 0 {
		prevHash = n.entries[len(n.entries)-1].Hash
	}
	if !bytes.Equal(e.Content.PrevHash, prevHash) {
		return fmt.Errorf("%w: received entry prev_hash mismatch", ErrTampered)
	}
	cb, err := canonicalBytes(e.Content)
	if err != nil {
		return fmt.Errorf("ledgerp2p: canonical: %w", err)
	}
	if !ed25519.Verify(n.pub, cb, e.Sig) {
		return fmt.Errorf("%w: received entry bad signature", ErrTampered)
	}
	if !bytes.Equal(e.Hash, entryHash(cb, e.Sig)) {
		return fmt.Errorf("%w: received entry hash mismatch", ErrTampered)
	}
	n.entries = append(n.entries, e)
	return nil
}

// ErrTampered is returned by Verify when a signature or chain link does not check out.
var ErrTampered = errors.New("ledgerp2p: chain verification failed")

func (n *memNode) Verify() error {
	n.mu.RLock()
	defer n.mu.RUnlock()

	var prevHash []byte
	for i, e := range n.entries {
		if e.Content.Seq != uint64(i) {
			return fmt.Errorf("%w: entry %d has seq %d", ErrTampered, i, e.Content.Seq)
		}
		if !bytes.Equal(e.Content.PrevHash, prevHash) {
			return fmt.Errorf("%w: entry %d prev_hash mismatch", ErrTampered, i)
		}
		cb, err := canonicalBytes(e.Content)
		if err != nil {
			return fmt.Errorf("ledgerp2p: canonical: %w", err)
		}
		if !ed25519.Verify(n.pub, cb, e.Sig) {
			return fmt.Errorf("%w: entry %d bad signature", ErrTampered, i)
		}
		if !bytes.Equal(e.Hash, entryHash(cb, e.Sig)) {
			return fmt.Errorf("%w: entry %d hash mismatch", ErrTampered, i)
		}
		prevHash = e.Hash
	}
	return nil
}

func (n *memNode) Entries() []Entry {
	n.mu.RLock()
	defer n.mu.RUnlock()
	out := make([]Entry, len(n.entries))
	copy(out, n.entries)
	return out
}

func (n *memNode) Len() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return len(n.entries)
}

func (n *memNode) PublicKey() ed25519.PublicKey {
	return n.pub
}

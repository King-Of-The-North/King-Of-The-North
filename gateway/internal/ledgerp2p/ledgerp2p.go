// Package ledgerp2p hosts the replicated, signed, append-only transaction ledger
// (the CeDeFi "De" layer). Nodes keep a FULL copy and verify/replicate — they never
// custody balances, so a lost node loses nothing (ADR-0005). Client phones run nodes
// and earn credit for it (DePIN, ADR-0008).
//
// Scaffold only — entry shape + interface; replication wired in the build window.
package ledgerp2p

// Entry is one hash-chained ledger record.
type Entry struct {
	PrevHash []byte
	UserID   string
	Amount   int64 // minor units (kuruş)
	Items    []string
	MokaRef  string
	Sig      []byte
}

// Node is a single replica of the full ledger.
type Node interface {
	Append(e Entry) error
	Verify() error // re-walk the hash chain
	Len() int
}

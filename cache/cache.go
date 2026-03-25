package cache

// TxID is a 32-byte transaction identifier
type TxID = [32]byte

// IndexTerm represents a parsed index term from a transaction
type IndexTerm struct {
	Key   string
	Value string
	Vouts []uint32
}

// IndexTermCache provides fast access to previously parsed transaction index terms
// This avoids re-parsing transactions when they appear in multiple subtrees
type IndexTermCache interface {
	// Get retrieves cached index terms for a transaction
	// Returns nil if not cached
	Get(txid TxID) ([]IndexTerm, bool)

	// Put stores index terms for a transaction
	Put(txid TxID, terms []IndexTerm) error

	// Delete removes cached terms for a transaction
	Delete(txid TxID) error

	// Clear removes all cached entries
	Clear() error
}

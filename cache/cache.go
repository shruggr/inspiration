package cache

import (
	"github.com/shruggr/inspiration/kvstore"
)

// IndexTerm represents a parsed index term from a transaction
type IndexTerm struct {
	Key   []byte
	Value []byte
}

// IndexTermCache provides fast access to previously parsed transaction index terms
// This avoids re-parsing transactions when they appear in multiple subtrees
type IndexTermCache interface {
	// Get retrieves cached index terms for a transaction
	// Returns nil if not cached
	Get(txid kvstore.Hash) ([]IndexTerm, bool)

	// Put stores index terms for a transaction
	Put(txid kvstore.Hash, terms []IndexTerm) error

	// Delete removes cached terms for a transaction
	Delete(txid kvstore.Hash) error

	// Clear removes all cached entries
	Clear() error
}

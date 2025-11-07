package txindexer

import (
	"context"
)

// IndexResult represents a key-value pair extracted from a transaction
type IndexResult struct {
	Key   []byte
	Value []byte
}

// TransactionContext provides transaction data and metadata to indexers
type TransactionContext struct {
	TxID         []byte
	RawTx        []byte
	BlockHeight  *uint64 // nil if unconfirmed (mempool)
	SubtreeRoot  []byte  // nil if unconfirmed
	SubtreeIndex *uint32 // nil if unconfirmed
}

// Indexer is the plugin interface for extracting index terms from transactions
type Indexer interface {
	// Index extracts key-value pairs from a transaction
	// Returns multiple results if the transaction matches multiple index criteria
	Index(ctx context.Context, tx *TransactionContext) ([]*IndexResult, error)

	// Name returns a human-readable name for this indexer
	Name() string
}

// MultiIndexer combines multiple indexers
type MultiIndexer struct {
	indexers []Indexer
}

// NewMultiIndexer creates a composite indexer from multiple indexers
func NewMultiIndexer(indexers ...Indexer) *MultiIndexer {
	return &MultiIndexer{
		indexers: indexers,
	}
}

// Index runs all child indexers and combines their results
func (m *MultiIndexer) Index(ctx context.Context, tx *TransactionContext) ([]*IndexResult, error) {
	var allResults []*IndexResult

	for _, indexer := range m.indexers {
		results, err := indexer.Index(ctx, tx)
		if err != nil {
			// Log error but continue with other indexers
			continue
		}
		allResults = append(allResults, results...)
	}

	return allResults, nil
}

// Name returns the name of this multi-indexer
func (m *MultiIndexer) Name() string {
	return "MultiIndexer"
}

// AddIndexer adds a new indexer to the multi-indexer
func (m *MultiIndexer) AddIndexer(indexer Indexer) {
	m.indexers = append(m.indexers, indexer)
}

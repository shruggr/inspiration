package txindexer

import (
	"context"
)

// NoopIndexer is a placeholder indexer that indexes nothing
// Useful for testing the pipeline without actual indexing logic
type NoopIndexer struct{}

// NewNoopIndexer creates a new no-op indexer
func NewNoopIndexer() *NoopIndexer {
	return &NoopIndexer{}
}

// Index returns no results
func (n *NoopIndexer) Index(ctx context.Context, tx *TransactionContext) ([]*IndexResult, error) {
	return nil, nil
}

// Name returns the indexer name
func (n *NoopIndexer) Name() string {
	return "NoopIndexer"
}

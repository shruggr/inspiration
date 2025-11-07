package treebuilder

import (
	"context"

	"github.com/shruggr/inspiration/cache"
	"github.com/shruggr/inspiration/kvstore"
)

// Builder builds index trees for subtrees and block→subtree mappings
type Builder interface {
	// BuildSubtreeIndex builds an index tree for a single subtree
	// Returns the root hash of the built tree
	BuildSubtreeIndex(
		ctx context.Context,
		subtreeMerkleRoot kvstore.Hash,
		txs []TransactionWithTerms,
	) (kvstore.Hash, error)

	// BuildBlockSubtreeIndex builds the block→subtree mapping
	// Returns the IndexNode data to be stored at merkleRoot
	BuildBlockSubtreeIndex(
		ctx context.Context,
		subtrees []SubtreeInfo,
	) ([]byte, error)
}

// TransactionWithTerms represents a transaction and its parsed index terms
type TransactionWithTerms struct {
	TxID  kvstore.Hash
	Terms []cache.IndexTerm // From indexer or cache
}

// SubtreeInfo contains information about a processed subtree
type SubtreeInfo struct {
	MerkleRoot    kvstore.Hash // Merkle root of the subtree
	TxCount       uint32       // Number of transactions in subtree
	IndexRootHash kvstore.Hash // Root hash of index tree for this subtree
}

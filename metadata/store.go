package metadata

import (
	"context"

	"github.com/shruggr/inspiration/kvstore"
)

// BlockMeta contains minimal block metadata for height tracking
// Full block header (80 bytes) is stored in KVStore by block_hash
// Block → Subtrees mapping is stored as an IndexNode in KVStore by merkle_root:
//   merkle_root → IndexNode(Mode 3) { subtree_merkleRoot → (indexRootHash, txCount) }
type BlockMeta struct {
	Height     uint64
	BlockHash  kvstore.Hash // SHA256 block hash
	MerkleRoot kvstore.Hash // Merkle root (key for subtree index in KVStore)
}

// Store defines the interface for storing blockchain metadata
// Implementations use SQLite or other relational databases
type Store interface {
	// PutBlock stores block metadata
	PutBlock(ctx context.Context, meta *BlockMeta) error

	// GetBlock retrieves block metadata by height
	GetBlock(ctx context.Context, height uint64) (*BlockMeta, error)

	// GetBlockByHash retrieves block metadata by block hash
	GetBlockByHash(ctx context.Context, blockHash kvstore.Hash) (*BlockMeta, error)

	// DeleteBlock removes block metadata (for reorg cleanup)
	DeleteBlock(ctx context.Context, height uint64) error

	// GetLatestBlock returns the highest block height stored
	GetLatestBlock(ctx context.Context) (*BlockMeta, error)

	// Close releases any resources
	Close() error
}

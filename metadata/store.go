package metadata

import (
	"context"

	"github.com/shruggr/inspiration/kvstore"
)

// BlockStatus represents the status of a block in the chain
type BlockStatus string

const (
	StatusMain    BlockStatus = "main"    // Main chain block
	StatusOrphan  BlockStatus = "orphan"  // Orphaned block (reorg victim)
	StatusPending BlockStatus = "pending" // Block not yet confirmed
)

// BlockMeta contains block metadata with IPLD merkle tree support
type BlockMeta struct {
	Height     uint64
	BlockHash  kvstore.Hash
	MerkleRoot kvstore.Hash
	TxCount    uint32
	Status     BlockStatus
	Timestamp  int64
}

// SubtreeMeta contains metadata for a subtree within a block
type SubtreeMeta struct {
	MerkleRoot         kvstore.Hash // Parent block merkle root
	SubtreeIndex       uint32       // Position in block (0-based)
	SubtreeMerkleRoot  kvstore.Hash // Merkle root of this subtree
	TxCount            uint32       // Number of transactions in subtree
	IndexRoot          []byte       // Multihash of index tree root
	TxTreeRoot         []byte       // Multihash of IPLD merkle tree root
}

// Store defines the interface for storing blockchain metadata
type Store interface {
	// PutBlock stores block metadata with associated subtrees atomically
	PutBlock(ctx context.Context, block *BlockMeta, subtrees []*SubtreeMeta) error

	// GetBlock retrieves block metadata by height
	GetBlock(ctx context.Context, height uint64) (*BlockMeta, error)

	// GetBlockByHash retrieves block metadata by block hash
	GetBlockByHash(ctx context.Context, blockHash kvstore.Hash) (*BlockMeta, error)

	// GetBlockByMerkleRoot retrieves block metadata by merkle root
	GetBlockByMerkleRoot(ctx context.Context, merkleRoot kvstore.Hash) (*BlockMeta, error)

	// GetSubtrees retrieves all subtrees for a block, ordered by subtree_index
	GetSubtrees(ctx context.Context, merkleRoot kvstore.Hash) ([]*SubtreeMeta, error)

	// MarkOrphan marks blocks at the given height as orphaned
	MarkOrphan(ctx context.Context, height uint64) error

	// CleanupOrphans removes orphaned blocks older than the given depth
	CleanupOrphans(ctx context.Context, currentHeight uint64, depth uint64) error

	// GetLatestBlock returns the highest main chain block
	GetLatestBlock(ctx context.Context) (*BlockMeta, error)

	// Close releases any resources
	Close() error
}

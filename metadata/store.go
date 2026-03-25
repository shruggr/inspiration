package metadata

import "context"

type Store interface {
	InsertSubtree(ctx context.Context, hash, indexRoot []byte, txCount uint32) error
	InsertBlock(ctx context.Context, height uint32, blockHash, header []byte, txCount uint64, subtreeHashes [][]byte) error
	GetBlockSubtrees(ctx context.Context, blockHash []byte) ([][]byte, error)
	GetSubtreeIndexRoot(ctx context.Context, subtreeHash []byte) ([]byte, error)
	SubtreeExists(ctx context.Context, subtreeHash []byte) (bool, error)
	PromoteBlock(ctx context.Context, blockHash []byte) error
	OrphanBlock(ctx context.Context, blockHash []byte) error
	GetUnpromotedBlocks(ctx context.Context, deeperThanHeight uint32) ([][]byte, error)
	Close() error
}

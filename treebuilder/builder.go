package treebuilder

import (
	"context"

	"github.com/shruggr/inspiration/multihash"
)

type Builder interface {
	BuildSubtreeIndex(ctx context.Context, entries []TaggedTransaction) (multihash.IndexHash, error)
}

type TaggedTransaction struct {
	TxID            [32]byte
	SubtreePosition uint64
	Tags            []Tag
}

type Tag struct {
	Key   string
	Value string
	Vouts []uint32
}

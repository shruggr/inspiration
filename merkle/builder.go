package merkle

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/shruggr/inspiration/kvstore"
	"github.com/shruggr/inspiration/multihash"
)

// Builder builds Bitcoin merkle trees in IPLD format
type Builder struct {
	store kvstore.KVStore
}

// NewBuilder creates a new merkle tree builder
func NewBuilder(store kvstore.KVStore) *Builder {
	return &Builder{store: store}
}

// BuildSubtreeMerkleTree builds a Bitcoin merkle tree for a subtree's transactions
// and stores it in IPLD format (64-byte nodes: left || right)
// Returns the multihash of the merkle root
func (b *Builder) BuildSubtreeMerkleTree(ctx context.Context, txids [][32]byte) (multihash.MerkleHash, error) {
	if len(txids) == 0 {
		return nil, fmt.Errorf("cannot build tree with zero transactions")
	}

	if len(txids) == 1 {
		return multihash.WrapMerkleHash(txids[0])
	}

	root, err := b.buildTree(ctx, txids)
	if err != nil {
		return nil, err
	}

	return multihash.WrapMerkleHash(root)
}

// buildTree recursively builds the merkle tree
func (b *Builder) buildTree(ctx context.Context, hashes [][32]byte) ([32]byte, error) {
	n := len(hashes)
	if n == 0 {
		return [32]byte{}, fmt.Errorf("cannot build tree with zero hashes")
	}

	if n == 1 {
		return hashes[0], nil
	}

	nextLevel := make([][32]byte, 0, (n+1)/2)

	for i := 0; i < n; i += 2 {
		left := hashes[i]
		var right [32]byte

		if i+1 < n {
			right = hashes[i+1]
		} else {
			right = left
		}

		parent := hashPair(left, right)

		node := make([]byte, 64)
		copy(node[0:32], left[:])
		copy(node[32:64], right[:])

		mh, err := multihash.WrapMerkleHash(parent)
		if err != nil {
			return [32]byte{}, fmt.Errorf("failed to wrap hash: %w", err)
		}

		if err := b.store.Put(ctx, mh.Bytes(), node); err != nil {
			return [32]byte{}, fmt.Errorf("failed to store node: %w", err)
		}

		nextLevel = append(nextLevel, parent)
	}

	return b.buildTree(ctx, nextLevel)
}

// hashPair computes the Bitcoin merkle hash of two child hashes
func hashPair(left, right [32]byte) [32]byte {
	var combined [64]byte
	copy(combined[0:32], left[:])
	copy(combined[32:64], right[:])
	return doubleSHA256(combined[:])
}

// doubleSHA256 computes SHA256(SHA256(data))
func doubleSHA256(data []byte) [32]byte {
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	return second
}

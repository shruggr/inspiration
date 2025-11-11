package merkle

import (
	"context"
	"fmt"

	"github.com/shruggr/inspiration/multihash"
)

// TODO: consider using go-sdk types for merkle proofs

// ProofNode represents a node in a merkle proof
type ProofNode struct {
	Hash     [32]byte
	IsLeft   bool // true if this node is on the left side
	Position uint32
}

// MerkleProof represents a merkle proof for a transaction
type MerkleProof struct {
	TxID     [32]byte
	Position uint32
	Nodes    []ProofNode
}

// BuildMerkleProof constructs a merkle proof by walking the IPLD tree
func (b *Builder) BuildMerkleProof(ctx context.Context, treeRoot multihash.MerkleHash, position uint32, txCount uint32) (*MerkleProof, error) {
	if position >= txCount {
		return nil, fmt.Errorf("position %d exceeds tx count %d", position, txCount)
	}

	proof := &MerkleProof{
		Position: position,
		Nodes:    []ProofNode{},
	}

	if err := b.buildProof(ctx, treeRoot, position, txCount, proof); err != nil {
		return nil, err
	}

	return proof, nil
}

// buildProof recursively walks the tree to build the proof
func (b *Builder) buildProof(ctx context.Context, nodeHash multihash.MerkleHash, position uint32, count uint32, proof *MerkleProof) error {
	if count == 1 {
		raw, err := nodeHash.Raw()
		if err != nil {
			return err
		}
		proof.TxID = raw
		return nil
	}

	nodeData, err := b.store.Get(ctx, nodeHash.Bytes())
	if err != nil {
		return fmt.Errorf("failed to get node from store: %w", err)
	}
	if nodeData == nil {
		return fmt.Errorf("node not found in store")
	}
	if len(nodeData) != 64 {
		return fmt.Errorf("invalid node size: expected 64 bytes, got %d", len(nodeData))
	}

	var left, right [32]byte
	copy(left[:], nodeData[0:32])
	copy(right[:], nodeData[32:64])

	mid := nextPowerOf2(count) / 2
	if mid > count {
		mid = count / 2
		if count%2 == 1 {
			mid++
		}
	}

	if position < mid {
		proof.Nodes = append(proof.Nodes, ProofNode{
			Hash:     right,
			IsLeft:   false,
			Position: mid,
		})
		if mid == 1 {
			proof.TxID = left
			return nil
		}
		leftHash, err := multihash.WrapMerkleHash(left)
		if err != nil {
			return err
		}
		return b.buildProof(ctx, leftHash, position, mid, proof)
	} else {
		proof.Nodes = append(proof.Nodes, ProofNode{
			Hash:     left,
			IsLeft:   true,
			Position: 0,
		})
		if count-mid == 1 {
			proof.TxID = right
			return nil
		}
		rightHash, err := multihash.WrapMerkleHash(right)
		if err != nil {
			return err
		}
		return b.buildProof(ctx, rightHash, position-mid, count-mid, proof)
	}
}

// BuildBlockMerkleProof builds a merkle proof from subtree roots to block root
func (b *Builder) BuildBlockMerkleProof(ctx context.Context, subtreeRoots [][32]byte, subtreeIndex uint32) (*MerkleProof, error) {
	if int(subtreeIndex) >= len(subtreeRoots) {
		return nil, fmt.Errorf("subtree index %d exceeds subtree count %d", subtreeIndex, len(subtreeRoots))
	}

	if len(subtreeRoots) == 1 {
		return &MerkleProof{
			TxID:     subtreeRoots[0],
			Position: 0,
			Nodes:    []ProofNode{},
		}, nil
	}

	proof := &MerkleProof{
		TxID:     subtreeRoots[subtreeIndex],
		Position: subtreeIndex,
		Nodes:    []ProofNode{},
	}

	b.buildBlockProof(subtreeRoots, subtreeIndex, proof)

	return proof, nil
}

// buildBlockProof recursively builds the block-level proof
func (b *Builder) buildBlockProof(hashes [][32]byte, position uint32, proof *MerkleProof) {
	n := len(hashes)
	if n == 1 {
		return
	}

	nextLevel := make([][32]byte, 0, (n+1)/2)
	nextPosition := position / 2

	for i := 0; i < n; i += 2 {
		left := hashes[i]
		var right [32]byte

		if i+1 < n {
			right = hashes[i+1]
		} else {
			right = left
		}

		if uint32(i) == position || uint32(i+1) == position {
			node := ProofNode{}
			if uint32(i) == position {
				node.Hash = right
				node.IsLeft = false
				node.Position = uint32(i + 1)
			} else {
				node.Hash = left
				node.IsLeft = true
				node.Position = uint32(i)
			}
			proof.Nodes = append([]ProofNode{node}, proof.Nodes...)
		}

		parent := hashPair(left, right)
		nextLevel = append(nextLevel, parent)
	}

	b.buildBlockProof(nextLevel, nextPosition, proof)
}

// VerifyProof verifies a merkle proof
func VerifyProof(proof *MerkleProof, expectedRoot [32]byte) bool {
	current := proof.TxID

	for i := len(proof.Nodes) - 1; i >= 0; i-- {
		node := proof.Nodes[i]
		if node.IsLeft {
			current = hashPair(node.Hash, current)
		} else {
			current = hashPair(current, node.Hash)
		}
	}

	return current == expectedRoot
}

// nextPowerOf2 returns the next power of 2 >= n
func nextPowerOf2(n uint32) uint32 {
	if n == 0 {
		return 1
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n++
	return n
}

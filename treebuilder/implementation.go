package treebuilder

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/shruggr/inspiration/indexnode"
	"github.com/shruggr/inspiration/kvstore"
	"lukechampine.com/blake3"
)

// implementation is the concrete implementation of Builder
type implementation struct {
	store kvstore.KVStore
}

// NewBuilder creates a new tree builder
func NewBuilder(store kvstore.KVStore) Builder {
	return &implementation{
		store: store,
	}
}

// BuildSubtreeIndex builds an index tree for a single subtree
func (b *implementation) BuildSubtreeIndex(
	ctx context.Context,
	subtreeMerkleRoot kvstore.Hash,
	txs []TransactionWithTerms,
) (kvstore.Hash, error) {
	if len(txs) == 0 {
		return kvstore.Hash{}, fmt.Errorf("no transactions to index")
	}

	// Step 1: Group transactions by indexed_key → indexed_value → []txid
	// Map structure: key → value → list of txids
	indexMap := make(map[string]map[string][]kvstore.Hash)

	for _, tx := range txs {
		for _, term := range tx.Terms {
			keyStr := string(term.Key)
			valueStr := string(term.Value)

			if indexMap[keyStr] == nil {
				indexMap[keyStr] = make(map[string][]kvstore.Hash)
			}
			indexMap[keyStr][valueStr] = append(indexMap[keyStr][valueStr], tx.TxID)
		}
	}

	// Step 2: Build leaf nodes for each indexed_key
	// Each leaf node contains: indexed_value → txid_list_hash
	leafNodes := make(map[string]kvstore.Hash) // key → leaf node hash

	for key, valueMap := range indexMap {
		// Create a leaf node for this key
		leafNode := indexnode.NewIndexNode() // Mode 0: value keys, no data

		// Sort values for deterministic ordering
		values := make([]string, 0, len(valueMap))
		for value := range valueMap {
			values = append(values, value)
		}
		sort.Strings(values)

		for _, value := range values {
			txidList := valueMap[value]

			// Create and store the txid list
			txidListHash, err := b.storeTxIDList(ctx, txidList)
			if err != nil {
				return kvstore.Hash{}, fmt.Errorf("failed to store txid list: %w", err)
			}

			// Add entry: value → txid_list_hash
			if err := leafNode.AddEntry([]byte(value), txidListHash[:], nil); err != nil {
				return kvstore.Hash{}, fmt.Errorf("failed to add entry to leaf node: %w", err)
			}
		}

		// Marshal and store the leaf node
		leafNodeBytes, err := leafNode.Marshal()
		if err != nil {
			return kvstore.Hash{}, fmt.Errorf("failed to marshal leaf node: %w", err)
		}

		leafNodeHash := hashNode(leafNodeBytes)
		if err := b.store.Put(ctx, leafNodeHash[:], leafNodeBytes); err != nil {
			return kvstore.Hash{}, fmt.Errorf("failed to store leaf node: %w", err)
		}

		leafNodes[key] = leafNodeHash
	}

	// Step 3: Build root node containing: indexed_key → leaf_node_hash
	rootNode := indexnode.NewIndexNode() // Mode 0: value keys, no data

	// Sort keys for deterministic ordering
	keys := make([]string, 0, len(leafNodes))
	for key := range leafNodes {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		leafHash := leafNodes[key]
		if err := rootNode.AddEntry([]byte(key), leafHash[:], nil); err != nil {
			return kvstore.Hash{}, fmt.Errorf("failed to add entry to root node: %w", err)
		}
	}

	// Marshal and store root node
	rootNodeBytes, err := rootNode.Marshal()
	if err != nil {
		return kvstore.Hash{}, fmt.Errorf("failed to marshal root node: %w", err)
	}

	rootHash := hashNode(rootNodeBytes)
	if err := b.store.Put(ctx, rootHash[:], rootNodeBytes); err != nil {
		return kvstore.Hash{}, fmt.Errorf("failed to store root node: %w", err)
	}

	return rootHash, nil
}

// BuildBlockSubtreeIndex builds the block→subtree mapping
func (b *implementation) BuildBlockSubtreeIndex(
	ctx context.Context,
	subtrees []SubtreeInfo,
) ([]byte, error) {
	if len(subtrees) == 0 {
		return nil, fmt.Errorf("no subtrees to index")
	}

	// Create Mode 3 IndexNode: pointer keys (32 bytes), with data
	node := indexnode.NewFixedKeyIndexNodeWithData(32)

	// Sort subtrees by merkle root for deterministic ordering
	sortedSubtrees := make([]SubtreeInfo, len(subtrees))
	copy(sortedSubtrees, subtrees)
	sort.Slice(sortedSubtrees, func(i, j int) bool {
		return bytes.Compare(sortedSubtrees[i].MerkleRoot[:], sortedSubtrees[j].MerkleRoot[:]) < 0
	})

	// Add entries: subtree_merkle_root → index_root_hash, with tx_count as data
	for _, subtree := range sortedSubtrees {
		// Encode tx count as 4-byte big-endian uint32
		txCountBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(txCountBytes, subtree.TxCount)

		if err := node.AddEntry(subtree.MerkleRoot[:], subtree.IndexRootHash[:], txCountBytes); err != nil {
			return nil, fmt.Errorf("failed to add subtree entry: %w", err)
		}
	}

	// Marshal the node
	nodeBytes, err := node.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal block subtree index: %w", err)
	}

	return nodeBytes, nil
}

// storeTxIDList stores a list of transaction IDs and returns the BLAKE3 hash
func (b *implementation) storeTxIDList(ctx context.Context, txids []kvstore.Hash) (kvstore.Hash, error) {
	// Sort txids for deterministic ordering
	sortedTxids := make([]kvstore.Hash, len(txids))
	copy(sortedTxids, txids)
	sort.Slice(sortedTxids, func(i, j int) bool {
		return bytes.Compare(sortedTxids[i][:], sortedTxids[j][:]) < 0
	})

	// Serialize: count (4 bytes) + concatenated txids
	buf := make([]byte, 4+len(sortedTxids)*32)
	binary.BigEndian.PutUint32(buf[0:4], uint32(len(sortedTxids)))

	offset := 4
	for _, txid := range sortedTxids {
		copy(buf[offset:offset+32], txid[:])
		offset += 32
	}

	// Hash and store
	hash := hashNode(buf)
	if err := b.store.Put(ctx, hash[:], buf); err != nil {
		return kvstore.Hash{}, err
	}

	return hash, nil
}

// hashNode computes the BLAKE3 hash of node data
func hashNode(data []byte) kvstore.Hash {
	h := blake3.Sum256(data)
	return h
}

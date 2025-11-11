package treebuilder

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/shruggr/inspiration/cache"
	"github.com/shruggr/inspiration/indexnode"
	"github.com/shruggr/inspiration/kvstore"
	"github.com/shruggr/inspiration/kvstore/memory"
)

func TestBuildSubtreeIndex(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	builder := NewBuilder(store)

	// Create test transactions with terms
	txs := []TransactionWithTerms{
		{
			TxID: createTestHash(1),
			Terms: []cache.IndexTerm{
				{Key: []byte("protocol"), Value: []byte("bap")},
				{Key: []byte("type"), Value: []byte("IDENTITY")},
			},
		},
		{
			TxID: createTestHash(2),
			Terms: []cache.IndexTerm{
				{Key: []byte("protocol"), Value: []byte("bap")},
				{Key: []byte("type"), Value: []byte("ATTESTATION")},
			},
		},
		{
			TxID: createTestHash(3),
			Terms: []cache.IndexTerm{
				{Key: []byte("protocol"), Value: []byte("ord")},
				{Key: []byte("type"), Value: []byte("image/png")},
			},
		},
	}

	subtreeMerkleRoot := createTestHash(100)

	// Build the index
	rootHash, err := builder.BuildSubtreeIndex(ctx, subtreeMerkleRoot, txs)
	if err != nil {
		t.Fatalf("BuildSubtreeIndex failed: %v", err)
	}

	// Verify root node was stored
	rootNodeBytes, err := store.Get(ctx, rootHash[:])
	if err != nil {
		t.Fatalf("Failed to retrieve root node: %v", err)
	}
	if rootNodeBytes == nil {
		t.Fatal("Root node not found in store")
	}

	// Unmarshal and verify root node
	rootNode, err := indexnode.UnmarshalIndexNode(rootNodeBytes)
	if err != nil {
		t.Fatalf("Failed to unmarshal root node: %v", err)
	}

	// Should have 2 keys: "protocol" and "type"
	if len(rootNode.Entries) != 2 {
		t.Errorf("Expected 2 entries in root node, got %d", len(rootNode.Entries))
	}

	// Verify the entries are sorted
	if string(rootNode.Entries[0].Key) != "protocol" {
		t.Errorf("Expected first key to be 'protocol', got '%s'", string(rootNode.Entries[0].Key))
	}
	if string(rootNode.Entries[1].Key) != "type" {
		t.Errorf("Expected second key to be 'type', got '%s'", string(rootNode.Entries[1].Key))
	}
}

func TestBuildBlockSubtreeIndex(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	builder := NewBuilder(store)

	// Create test subtrees
	subtrees := []SubtreeInfo{
		{
			MerkleRoot:    createTestHash(1),
			TxCount:       100,
			IndexRootHash: createTestHash(10),
		},
		{
			MerkleRoot:    createTestHash(2),
			TxCount:       200,
			IndexRootHash: createTestHash(20),
		},
		{
			MerkleRoot:    createTestHash(3),
			TxCount:       150,
			IndexRootHash: createTestHash(30),
		},
	}

	// Build the block subtree index
	nodeBytes, err := builder.BuildBlockSubtreeIndex(ctx, subtrees)
	if err != nil {
		t.Fatalf("BuildBlockSubtreeIndex failed: %v", err)
	}

	// Unmarshal and verify
	node, err := indexnode.UnmarshalIndexNode(nodeBytes)
	if err != nil {
		t.Fatalf("Failed to unmarshal block subtree index: %v", err)
	}

	// Should have 3 entries
	if len(node.Entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(node.Entries))
	}

	// Verify mode is 3 (pointer keys with data)
	if node.Mode != 0x03 {
		t.Errorf("Expected mode 3, got %d", node.Mode)
	}

	// Verify first entry's data (tx count)
	if len(node.Entries[0].Data) != 4 {
		t.Errorf("Expected 4 bytes of data, got %d", len(node.Entries[0].Data))
	}

	// Entries should be sorted, so we need to find which subtree is first
	// Since we're sorting by merkle root, let's just verify all tx counts are present
	foundCounts := make(map[uint32]bool)
	for _, entry := range node.Entries {
		txCount := binary.BigEndian.Uint32(entry.Data)
		foundCounts[txCount] = true
	}

	expectedCounts := []uint32{100, 200, 150}
	for _, expected := range expectedCounts {
		if !foundCounts[expected] {
			t.Errorf("Expected to find tx count %d", expected)
		}
	}
}

func TestBuildSubtreeIndexEmpty(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	builder := NewBuilder(store)

	subtreeMerkleRoot := createTestHash(100)

	_, err := builder.BuildSubtreeIndex(ctx, subtreeMerkleRoot, []TransactionWithTerms{})
	if err == nil {
		t.Fatal("Expected error for empty transactions, got nil")
	}
}

func TestBuildBlockSubtreeIndexEmpty(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	builder := NewBuilder(store)

	_, err := builder.BuildBlockSubtreeIndex(ctx, []SubtreeInfo{})
	if err == nil {
		t.Fatal("Expected error for empty subtrees, got nil")
	}
}

// createTestHash creates a test hash with a specific byte value
func createTestHash(value byte) kvstore.Hash {
	var hash kvstore.Hash
	hash[0] = value
	return hash
}

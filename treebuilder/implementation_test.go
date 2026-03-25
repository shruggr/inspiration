package treebuilder

import (
	"bytes"
	"context"
	"testing"

	"github.com/shruggr/inspiration/indexnode"
	"github.com/shruggr/inspiration/kvstore/memory"
)

func TestBuildSubtreeIndex(t *testing.T) {
	store := memory.New()
	builder := NewBuilder(store)
	ctx := context.Background()

	txs := []TaggedTransaction{
		{
			TxID:            [32]byte{1},
			SubtreePosition: 0,
			Tags: []Tag{
				{Key: "address", Value: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", Vouts: []uint32{0}},
				{Key: "output_type", Value: "p2pkh", Vouts: []uint32{0}},
			},
		},
		{
			TxID:            [32]byte{2},
			SubtreePosition: 1,
			Tags: []Tag{
				{Key: "address", Value: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", Vouts: []uint32{0, 2}},
				{Key: "address", Value: "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2", Vouts: []uint32{1}},
			},
		},
	}

	rootHash, err := builder.BuildSubtreeIndex(ctx, txs)
	if err != nil {
		t.Fatalf("BuildSubtreeIndex: %v", err)
	}

	// Verify root exists in store
	rootBytes, err := store.Get(ctx, rootHash.Bytes())
	if err != nil || rootBytes == nil {
		t.Fatal("root node not in store")
	}

	// Verify determinism
	rootHash2, _ := builder.BuildSubtreeIndex(ctx, txs)
	if !bytes.Equal(rootHash.Bytes(), rootHash2.Bytes()) {
		t.Error("non-deterministic hash")
	}

	// Verify we can unmarshal root and find tag keys
	rootNode, err := indexnode.Unmarshal(rootBytes)
	if err != nil {
		t.Fatalf("unmarshal root: %v", err)
	}
	// Root should have 2 entries: "address" and "output_type"
	if len(rootNode.Entries) != 2 {
		t.Errorf("root entries: got %d, want 2", len(rootNode.Entries))
	}
}

func TestBuildSubtreeIndexEmpty(t *testing.T) {
	store := memory.New()
	builder := NewBuilder(store)
	_, err := builder.BuildSubtreeIndex(context.Background(), nil)
	if err == nil {
		t.Error("expected error for empty input")
	}
}

package sqlite

import (
	"context"
	"os"
	"testing"

	"github.com/shruggr/inspiration/kvstore"
	"github.com/shruggr/inspiration/metadata"
)

func TestPutAndGetBlock(t *testing.T) {
	tmpFile := "/tmp/test_metadata.db"
	defer os.Remove(tmpFile)

	store, err := New(&Config{DBPath: tmpFile})
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	block := &metadata.BlockMeta{
		Height:     100,
		BlockHash:  kvstore.Hash{1, 2, 3},
		MerkleRoot: kvstore.Hash{4, 5, 6},
		TxCount:    50,
		Status:     metadata.StatusMain,
		Timestamp:  1234567890,
	}

	subtrees := []*metadata.SubtreeMeta{
		{
			MerkleRoot:        kvstore.Hash{4, 5, 6},
			SubtreeIndex:      0,
			SubtreeMerkleRoot: kvstore.Hash{7, 8, 9},
			TxCount:           25,
			IndexRoot:         []byte{10, 11, 12},
			TxTreeRoot:        []byte{13, 14, 15},
		},
		{
			MerkleRoot:        kvstore.Hash{4, 5, 6},
			SubtreeIndex:      1,
			SubtreeMerkleRoot: kvstore.Hash{16, 17, 18},
			TxCount:           25,
			IndexRoot:         []byte{19, 20, 21},
			TxTreeRoot:        []byte{22, 23, 24},
		},
	}

	if err := store.PutBlock(ctx, block, subtrees); err != nil {
		t.Fatalf("PutBlock failed: %v", err)
	}

	retrieved, err := store.GetBlock(ctx, 100)
	if err != nil {
		t.Fatalf("GetBlock failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("GetBlock returned nil")
	}

	if retrieved.Height != block.Height {
		t.Errorf("Height mismatch: expected %d, got %d", block.Height, retrieved.Height)
	}

	if retrieved.TxCount != block.TxCount {
		t.Errorf("TxCount mismatch: expected %d, got %d", block.TxCount, retrieved.TxCount)
	}

	if retrieved.Status != metadata.StatusMain {
		t.Errorf("Status mismatch: expected %s, got %s", metadata.StatusMain, retrieved.Status)
	}
}

func TestGetSubtrees(t *testing.T) {
	tmpFile := "/tmp/test_subtrees.db"
	defer os.Remove(tmpFile)

	store, err := New(&Config{DBPath: tmpFile})
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	merkleRoot := kvstore.Hash{4, 5, 6}

	block := &metadata.BlockMeta{
		Height:     100,
		BlockHash:  kvstore.Hash{1, 2, 3},
		MerkleRoot: merkleRoot,
		TxCount:    100,
		Status:     metadata.StatusMain,
		Timestamp:  1234567890,
	}

	subtrees := []*metadata.SubtreeMeta{
		{
			MerkleRoot:        merkleRoot,
			SubtreeIndex:      0,
			SubtreeMerkleRoot: kvstore.Hash{7, 8, 9},
			TxCount:           50,
			IndexRoot:         []byte{10, 11, 12},
			TxTreeRoot:        []byte{13, 14, 15},
		},
		{
			MerkleRoot:        merkleRoot,
			SubtreeIndex:      1,
			SubtreeMerkleRoot: kvstore.Hash{16, 17, 18},
			TxCount:           50,
			IndexRoot:         []byte{19, 20, 21},
			TxTreeRoot:        []byte{22, 23, 24},
		},
	}

	if err := store.PutBlock(ctx, block, subtrees); err != nil {
		t.Fatalf("PutBlock failed: %v", err)
	}

	retrieved, err := store.GetSubtrees(ctx, merkleRoot)
	if err != nil {
		t.Fatalf("GetSubtrees failed: %v", err)
	}

	if len(retrieved) != 2 {
		t.Fatalf("Expected 2 subtrees, got %d", len(retrieved))
	}

	if retrieved[0].SubtreeIndex != 0 {
		t.Errorf("First subtree index should be 0, got %d", retrieved[0].SubtreeIndex)
	}

	if retrieved[1].SubtreeIndex != 1 {
		t.Errorf("Second subtree index should be 1, got %d", retrieved[1].SubtreeIndex)
	}
}

func TestMarkOrphan(t *testing.T) {
	tmpFile := "/tmp/test_orphan.db"
	defer os.Remove(tmpFile)

	store, err := New(&Config{DBPath: tmpFile})
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	block := &metadata.BlockMeta{
		Height:     100,
		BlockHash:  kvstore.Hash{1, 2, 3},
		MerkleRoot: kvstore.Hash{4, 5, 6},
		TxCount:    50,
		Status:     metadata.StatusMain,
		Timestamp:  1234567890,
	}

	if err := store.PutBlock(ctx, block, nil); err != nil {
		t.Fatalf("PutBlock failed: %v", err)
	}

	if err := store.MarkOrphan(ctx, 100); err != nil {
		t.Fatalf("MarkOrphan failed: %v", err)
	}

	retrieved, err := store.GetBlockByMerkleRoot(ctx, kvstore.Hash{4, 5, 6})
	if err != nil {
		t.Fatalf("GetBlockByMerkleRoot failed: %v", err)
	}

	if retrieved.Status != metadata.StatusOrphan {
		t.Errorf("Status should be orphan, got %s", retrieved.Status)
	}
}

func TestCleanupOrphans(t *testing.T) {
	tmpFile := "/tmp/test_cleanup.db"
	defer os.Remove(tmpFile)

	store, err := New(&Config{DBPath: tmpFile})
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	orphanBlock := &metadata.BlockMeta{
		Height:     50,
		BlockHash:  kvstore.Hash{1, 2, 3},
		MerkleRoot: kvstore.Hash{4, 5, 6},
		TxCount:    25,
		Status:     metadata.StatusOrphan,
		Timestamp:  1234567890,
	}

	if err := store.PutBlock(ctx, orphanBlock, nil); err != nil {
		t.Fatalf("PutBlock failed: %v", err)
	}

	if err := store.CleanupOrphans(ctx, 200, 100); err != nil {
		t.Fatalf("CleanupOrphans failed: %v", err)
	}

	retrieved, err := store.GetBlockByMerkleRoot(ctx, kvstore.Hash{4, 5, 6})
	if err != nil {
		t.Fatalf("GetBlockByMerkleRoot failed: %v", err)
	}

	if retrieved != nil {
		t.Error("Old orphan block should have been deleted")
	}
}

func TestGetLatestBlock(t *testing.T) {
	tmpFile := "/tmp/test_latest.db"
	defer os.Remove(tmpFile)

	store, err := New(&Config{DBPath: tmpFile})
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	for i := uint64(1); i <= 5; i++ {
		block := &metadata.BlockMeta{
			Height:     i,
			BlockHash:  kvstore.Hash{byte(i), 0, 0},
			MerkleRoot: kvstore.Hash{0, byte(i), 0},
			TxCount:    10,
			Status:     metadata.StatusMain,
			Timestamp:  int64(i),
		}
		if err := store.PutBlock(ctx, block, nil); err != nil {
			t.Fatalf("PutBlock failed: %v", err)
		}
	}

	latest, err := store.GetLatestBlock(ctx)
	if err != nil {
		t.Fatalf("GetLatestBlock failed: %v", err)
	}

	if latest == nil {
		t.Fatal("GetLatestBlock returned nil")
	}

	if latest.Height != 5 {
		t.Errorf("Expected height 5, got %d", latest.Height)
	}
}

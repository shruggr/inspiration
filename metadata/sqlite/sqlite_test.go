package sqlite

import (
	"bytes"
	"context"
	"testing"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestInsertAndGetSubtree(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	hash := []byte{1, 2, 3, 4}
	indexRoot := []byte{10, 11, 12, 13}

	if err := s.InsertSubtree(ctx, hash, indexRoot, 50); err != nil {
		t.Fatalf("InsertSubtree failed: %v", err)
	}

	got, err := s.GetSubtreeIndexRoot(ctx, hash)
	if err != nil {
		t.Fatalf("GetSubtreeIndexRoot failed: %v", err)
	}
	if !bytes.Equal(got, indexRoot) {
		t.Errorf("index root mismatch: got %v, want %v", got, indexRoot)
	}

	got, err = s.GetSubtreeIndexRoot(ctx, []byte{99, 99})
	if err != nil {
		t.Fatalf("GetSubtreeIndexRoot for missing key failed: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for missing subtree, got %v", got)
	}
}

func TestInsertBlockWithSubtrees(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	sh1 := []byte{1, 1, 1}
	sh2 := []byte{2, 2, 2}
	sh3 := []byte{3, 3, 3}

	for _, sh := range [][]byte{sh1, sh2, sh3} {
		if err := s.InsertSubtree(ctx, sh, []byte{0xAA}, 10); err != nil {
			t.Fatalf("InsertSubtree failed: %v", err)
		}
	}

	blockHash := []byte{0xBB, 0xCC}
	header := []byte{0xDD, 0xEE}
	subtreeHashes := [][]byte{sh1, sh2, sh3}

	if err := s.InsertBlock(ctx, 100, blockHash, header, 30, subtreeHashes); err != nil {
		t.Fatalf("InsertBlock failed: %v", err)
	}

	got, err := s.GetBlockSubtrees(ctx, blockHash)
	if err != nil {
		t.Fatalf("GetBlockSubtrees failed: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("expected 3 subtrees, got %d", len(got))
	}
	for i, want := range subtreeHashes {
		if !bytes.Equal(got[i], want) {
			t.Errorf("subtree %d mismatch: got %v, want %v", i, got[i], want)
		}
	}
}

func TestSubtreeExists(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	hash := []byte{5, 6, 7}

	exists, err := s.SubtreeExists(ctx, hash)
	if err != nil {
		t.Fatalf("SubtreeExists failed: %v", err)
	}
	if exists {
		t.Error("expected false before insert")
	}

	if err := s.InsertSubtree(ctx, hash, []byte{0xAA}, 5); err != nil {
		t.Fatalf("InsertSubtree failed: %v", err)
	}

	exists, err = s.SubtreeExists(ctx, hash)
	if err != nil {
		t.Fatalf("SubtreeExists failed: %v", err)
	}
	if !exists {
		t.Error("expected true after insert")
	}
}

func TestPromoteBlock(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	sh := []byte{1, 2}
	if err := s.InsertSubtree(ctx, sh, []byte{0xAA}, 10); err != nil {
		t.Fatalf("InsertSubtree failed: %v", err)
	}

	blockHash := []byte{0xBB}
	if err := s.InsertBlock(ctx, 100, blockHash, []byte{0xCC}, 10, [][]byte{sh}); err != nil {
		t.Fatalf("InsertBlock failed: %v", err)
	}

	if err := s.PromoteBlock(ctx, blockHash); err != nil {
		t.Fatalf("PromoteBlock failed: %v", err)
	}

	var status string
	err := s.db.QueryRowContext(ctx, `SELECT status FROM blocks WHERE block_hash = ?`, blockHash).Scan(&status)
	if err != nil {
		t.Fatalf("query status failed: %v", err)
	}
	if status != "confirmed" {
		t.Errorf("expected status 'confirmed', got %q", status)
	}

	var promoted int
	var linkedBlock []byte
	err = s.db.QueryRowContext(ctx, `SELECT promoted, block_hash FROM subtrees WHERE subtree_hash = ?`, sh).Scan(&promoted, &linkedBlock)
	if err != nil {
		t.Fatalf("query subtree failed: %v", err)
	}
	if promoted != 1 {
		t.Errorf("expected promoted=1, got %d", promoted)
	}
	if !bytes.Equal(linkedBlock, blockHash) {
		t.Errorf("expected subtree block_hash=%v, got %v", blockHash, linkedBlock)
	}
}

func TestOrphanBlock(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	blockHash := []byte{0xAA}
	if err := s.InsertBlock(ctx, 50, blockHash, []byte{0xBB}, 5, nil); err != nil {
		t.Fatalf("InsertBlock failed: %v", err)
	}

	if err := s.OrphanBlock(ctx, blockHash); err != nil {
		t.Fatalf("OrphanBlock failed: %v", err)
	}

	var status string
	err := s.db.QueryRowContext(ctx, `SELECT status FROM blocks WHERE block_hash = ?`, blockHash).Scan(&status)
	if err != nil {
		t.Fatalf("query status failed: %v", err)
	}
	if status != "orphaned" {
		t.Errorf("expected status 'orphaned', got %q", status)
	}
}

func TestGetUnpromotedBlocks(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for i := uint32(1); i <= 5; i++ {
		if err := s.InsertBlock(ctx, i*10, []byte{byte(i)}, []byte{0xFF}, uint64(i), nil); err != nil {
			t.Fatalf("InsertBlock failed: %v", err)
		}
	}

	// Promote the block at height 30
	if err := s.PromoteBlock(ctx, []byte{3}); err != nil {
		t.Fatalf("PromoteBlock failed: %v", err)
	}

	got, err := s.GetUnpromotedBlocks(ctx, 30)
	if err != nil {
		t.Fatalf("GetUnpromotedBlocks failed: %v", err)
	}

	// Heights 10, 20 are pending and <= 30. Height 30 is confirmed so excluded.
	if len(got) != 2 {
		t.Fatalf("expected 2 unpromoted blocks, got %d", len(got))
	}
	if !bytes.Equal(got[0], []byte{1}) {
		t.Errorf("first block hash mismatch: got %v", got[0])
	}
	if !bytes.Equal(got[1], []byte{2}) {
		t.Errorf("second block hash mismatch: got %v", got[1])
	}
}

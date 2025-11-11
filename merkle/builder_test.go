package merkle

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/shruggr/inspiration/kvstore/memory"
)

func TestBuildSubtreeMerkleTree(t *testing.T) {
	store := memory.New()
	builder := NewBuilder(store)
	ctx := context.Background()

	txids := [][32]byte{
		sha256.Sum256([]byte("tx1")),
		sha256.Sum256([]byte("tx2")),
		sha256.Sum256([]byte("tx3")),
		sha256.Sum256([]byte("tx4")),
	}

	root, err := builder.BuildSubtreeMerkleTree(ctx, txids)
	if err != nil {
		t.Fatalf("BuildSubtreeMerkleTree failed: %v", err)
	}

	if len(root) != 34 {
		t.Errorf("Expected root multihash length 34, got %d", len(root))
	}

	rawRoot, err := root.Raw()
	if err != nil {
		t.Fatalf("Failed to extract raw root: %v", err)
	}

	h01 := hashPair(txids[0], txids[1])
	h23 := hashPair(txids[2], txids[3])
	expectedRoot := hashPair(h01, h23)

	if rawRoot != expectedRoot {
		t.Error("Root hash doesn't match expected value")
	}
}

func TestBuildSubtreeMerkleTreeSingleTx(t *testing.T) {
	store := memory.New()
	builder := NewBuilder(store)
	ctx := context.Background()

	txid := sha256.Sum256([]byte("single-tx"))
	txids := [][32]byte{txid}

	root, err := builder.BuildSubtreeMerkleTree(ctx, txids)
	if err != nil {
		t.Fatalf("BuildSubtreeMerkleTree failed: %v", err)
	}

	rawRoot, err := root.Raw()
	if err != nil {
		t.Fatalf("Failed to extract raw root: %v", err)
	}

	if rawRoot != txid {
		t.Error("Single tx root should equal txid")
	}
}

func TestBuildSubtreeMerkleTreeOddCount(t *testing.T) {
	store := memory.New()
	builder := NewBuilder(store)
	ctx := context.Background()

	txids := [][32]byte{
		sha256.Sum256([]byte("tx1")),
		sha256.Sum256([]byte("tx2")),
		sha256.Sum256([]byte("tx3")),
	}

	root, err := builder.BuildSubtreeMerkleTree(ctx, txids)
	if err != nil {
		t.Fatalf("BuildSubtreeMerkleTree failed: %v", err)
	}

	if len(root) != 34 {
		t.Errorf("Expected root multihash length 34, got %d", len(root))
	}

	rawRoot, err := root.Raw()
	if err != nil {
		t.Fatalf("Failed to extract raw root: %v", err)
	}

	h01 := hashPair(txids[0], txids[1])
	h22 := hashPair(txids[2], txids[2])
	expectedRoot := hashPair(h01, h22)

	if rawRoot != expectedRoot {
		t.Error("Root hash doesn't match expected value for odd count")
	}
}

func TestBuildSubtreeMerkleTreeEmpty(t *testing.T) {
	store := memory.New()
	builder := NewBuilder(store)
	ctx := context.Background()

	txids := [][32]byte{}

	_, err := builder.BuildSubtreeMerkleTree(ctx, txids)
	if err == nil {
		t.Error("Should fail with empty transaction list")
	}
}

func TestHashPair(t *testing.T) {
	left := sha256.Sum256([]byte("left"))
	right := sha256.Sum256([]byte("right"))

	result := hashPair(left, right)

	var combined [64]byte
	copy(combined[0:32], left[:])
	copy(combined[32:64], right[:])

	expected := doubleSHA256(combined[:])

	if result != expected {
		t.Error("hashPair result doesn't match expected double SHA256")
	}
}

func TestDoubleSHA256(t *testing.T) {
	data := []byte("test data")

	result := doubleSHA256(data)

	first := sha256.Sum256(data)
	expected := sha256.Sum256(first[:])

	if result != expected {
		t.Error("doubleSHA256 doesn't match expected value")
	}
}

package merkle

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/shruggr/inspiration/kvstore/memory"
)

func TestBuildAndVerifyMerkleProof(t *testing.T) {
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

	rawRoot, err := root.Raw()
	if err != nil {
		t.Fatalf("Failed to extract raw root: %v", err)
	}

	for i := uint32(0); i < 4; i++ {
		proof, err := builder.BuildMerkleProof(ctx, root, i, 4)
		if err != nil {
			t.Fatalf("BuildMerkleProof failed for position %d: %v", i, err)
		}

		if proof.Position != i {
			t.Errorf("Proof position mismatch: expected %d, got %d", i, proof.Position)
		}

		if proof.TxID != txids[i] {
			t.Errorf("Proof TxID mismatch for position %d", i)
		}

		if !VerifyProof(proof, rawRoot) {
			t.Errorf("Proof verification failed for position %d", i)
		}
	}
}

func TestBuildMerkleProofSingleTx(t *testing.T) {
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

	proof, err := builder.BuildMerkleProof(ctx, root, 0, 1)
	if err != nil {
		t.Fatalf("BuildMerkleProof failed: %v", err)
	}

	if len(proof.Nodes) != 0 {
		t.Errorf("Single tx proof should have no nodes, got %d", len(proof.Nodes))
	}

	if proof.TxID != txid {
		t.Error("Proof TxID doesn't match")
	}

	if !VerifyProof(proof, rawRoot) {
		t.Error("Proof verification failed")
	}
}

func TestBuildMerkleProofInvalidPosition(t *testing.T) {
	store := memory.New()
	builder := NewBuilder(store)
	ctx := context.Background()

	txids := [][32]byte{
		sha256.Sum256([]byte("tx1")),
		sha256.Sum256([]byte("tx2")),
	}

	root, err := builder.BuildSubtreeMerkleTree(ctx, txids)
	if err != nil {
		t.Fatalf("BuildSubtreeMerkleTree failed: %v", err)
	}

	_, err = builder.BuildMerkleProof(ctx, root, 5, 2)
	if err == nil {
		t.Error("Should fail with invalid position")
	}
}

func TestBuildBlockMerkleProof(t *testing.T) {
	builder := NewBuilder(memory.New())
	ctx := context.Background()

	subtreeRoots := [][32]byte{
		sha256.Sum256([]byte("subtree1")),
		sha256.Sum256([]byte("subtree2")),
		sha256.Sum256([]byte("subtree3")),
		sha256.Sum256([]byte("subtree4")),
	}

	h01 := hashPair(subtreeRoots[0], subtreeRoots[1])
	h23 := hashPair(subtreeRoots[2], subtreeRoots[3])
	blockRoot := hashPair(h01, h23)

	for i := uint32(0); i < 4; i++ {
		proof, err := builder.BuildBlockMerkleProof(ctx, subtreeRoots, i)
		if err != nil {
			t.Fatalf("BuildBlockMerkleProof failed for index %d: %v", i, err)
		}

		if proof.Position != i {
			t.Errorf("Proof position mismatch: expected %d, got %d", i, proof.Position)
		}

		if proof.TxID != subtreeRoots[i] {
			t.Errorf("Proof TxID mismatch for index %d", i)
		}

		if !VerifyProof(proof, blockRoot) {
			t.Errorf("Block proof verification failed for index %d", i)
		}
	}
}

func TestBuildBlockMerkleProofSingleSubtree(t *testing.T) {
	builder := NewBuilder(memory.New())
	ctx := context.Background()

	subtreeRoots := [][32]byte{
		sha256.Sum256([]byte("single-subtree")),
	}

	proof, err := builder.BuildBlockMerkleProof(ctx, subtreeRoots, 0)
	if err != nil {
		t.Fatalf("BuildBlockMerkleProof failed: %v", err)
	}

	if len(proof.Nodes) != 0 {
		t.Errorf("Single subtree proof should have no nodes, got %d", len(proof.Nodes))
	}

	if proof.TxID != subtreeRoots[0] {
		t.Error("Proof TxID doesn't match")
	}

	if !VerifyProof(proof, subtreeRoots[0]) {
		t.Error("Proof verification failed")
	}
}

func TestVerifyProofInvalidRoot(t *testing.T) {
	store := memory.New()
	builder := NewBuilder(store)
	ctx := context.Background()

	txids := [][32]byte{
		sha256.Sum256([]byte("tx1")),
		sha256.Sum256([]byte("tx2")),
	}

	root, err := builder.BuildSubtreeMerkleTree(ctx, txids)
	if err != nil {
		t.Fatalf("BuildSubtreeMerkleTree failed: %v", err)
	}

	proof, err := builder.BuildMerkleProof(ctx, root, 0, 2)
	if err != nil {
		t.Fatalf("BuildMerkleProof failed: %v", err)
	}

	wrongRoot := sha256.Sum256([]byte("wrong root"))

	if VerifyProof(proof, wrongRoot) {
		t.Error("Proof should not verify with wrong root")
	}
}

func TestNextPowerOf2(t *testing.T) {
	tests := []struct {
		input    uint32
		expected uint32
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 4},
		{4, 4},
		{5, 8},
		{7, 8},
		{8, 8},
		{15, 16},
		{16, 16},
		{17, 32},
		{100, 128},
		{1000, 1024},
	}

	for _, test := range tests {
		result := nextPowerOf2(test.input)
		if result != test.expected {
			t.Errorf("nextPowerOf2(%d) = %d, expected %d", test.input, result, test.expected)
		}
	}
}

package multihash

import (
	"crypto/sha256"
	"testing"

	mh "github.com/multiformats/go-multihash"
)

func TestIndexHash(t *testing.T) {
	data := []byte("test data for BLAKE3 hashing")

	hash, err := NewIndexHash(data)
	if err != nil {
		t.Fatalf("NewIndexHash failed: %v", err)
	}

	if len(hash) != 34 {
		t.Errorf("Expected hash length 34, got %d", len(hash))
	}

	decoded, err := mh.Decode(mh.Multihash(hash))
	if err != nil {
		t.Fatalf("Failed to decode multihash: %v", err)
	}

	if decoded.Code != mh.BLAKE3 {
		t.Errorf("Expected BLAKE3 code 0x%x, got 0x%x", mh.BLAKE3, decoded.Code)
	}

	if decoded.Length != 32 {
		t.Errorf("Expected digest length 32, got %d", decoded.Length)
	}
}

func TestIndexHashVerify(t *testing.T) {
	data := []byte("test data for verification")

	hash, err := NewIndexHash(data)
	if err != nil {
		t.Fatalf("NewIndexHash failed: %v", err)
	}

	if err := hash.Verify(data); err != nil {
		t.Errorf("Verify failed: %v", err)
	}

	wrongData := []byte("wrong data")
	if err := hash.Verify(wrongData); err == nil {
		t.Error("Verify should have failed for wrong data")
	}
}

func TestMerkleHash(t *testing.T) {
	data := []byte("test data for double SHA256")

	hash, err := NewMerkleHash(data)
	if err != nil {
		t.Fatalf("NewMerkleHash failed: %v", err)
	}

	if len(hash) != 34 {
		t.Errorf("Expected hash length 34, got %d", len(hash))
	}

	decoded, err := mh.Decode(mh.Multihash(hash))
	if err != nil {
		t.Fatalf("Failed to decode multihash: %v", err)
	}

	if decoded.Code != mh.DBL_SHA2_256 {
		t.Errorf("Expected dbl-sha2-256 code 0x%x, got 0x%x", mh.DBL_SHA2_256, decoded.Code)
	}

	if decoded.Length != 32 {
		t.Errorf("Expected digest length 32, got %d", decoded.Length)
	}
}

func TestMerkleHashVerify(t *testing.T) {
	data := []byte("test data for merkle verification")

	hash, err := NewMerkleHash(data)
	if err != nil {
		t.Fatalf("NewMerkleHash failed: %v", err)
	}

	if err := hash.Verify(data); err != nil {
		t.Errorf("Verify failed: %v", err)
	}

	wrongData := []byte("wrong data")
	if err := hash.Verify(wrongData); err == nil {
		t.Error("Verify should have failed for wrong data")
	}
}

func TestWrapMerkleHash(t *testing.T) {
	data := []byte("test data for wrapping")

	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])

	hash, err := WrapMerkleHash(second)
	if err != nil {
		t.Fatalf("WrapMerkleHash failed: %v", err)
	}

	raw, err := hash.Raw()
	if err != nil {
		t.Fatalf("Raw failed: %v", err)
	}

	if raw != second {
		t.Error("Raw hash doesn't match original")
	}
}

func TestMerkleHashRaw(t *testing.T) {
	data := []byte("test data for raw extraction")

	hash, err := NewMerkleHash(data)
	if err != nil {
		t.Fatalf("NewMerkleHash failed: %v", err)
	}

	raw, err := hash.Raw()
	if err != nil {
		t.Fatalf("Raw failed: %v", err)
	}

	first := sha256.Sum256(data)
	expected := sha256.Sum256(first[:])

	if raw != expected {
		t.Error("Raw hash doesn't match expected double SHA256")
	}
}

func TestIndexHashHex(t *testing.T) {
	data := []byte("test hex encoding")

	hash, err := NewIndexHash(data)
	if err != nil {
		t.Fatalf("NewIndexHash failed: %v", err)
	}

	hexStr := hash.Hex()
	if len(hexStr) != 68 {
		t.Errorf("Expected hex length 68 (34 bytes * 2), got %d", len(hexStr))
	}
}

func TestMerkleHashHex(t *testing.T) {
	data := []byte("test hex encoding")

	hash, err := NewMerkleHash(data)
	if err != nil {
		t.Fatalf("NewMerkleHash failed: %v", err)
	}

	hexStr := hash.Hex()
	if len(hexStr) != 68 {
		t.Errorf("Expected hex length 68 (34 bytes * 2), got %d", len(hexStr))
	}
}

package multihash

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	mh "github.com/multiformats/go-multihash"
	_ "github.com/multiformats/go-multihash/register/blake3"
)

// IndexHash wraps a BLAKE3 multihash for index structures
// Format: <0x1e><0x20><32 bytes> = 34 bytes total
type IndexHash []byte

// NewIndexHash creates a BLAKE3 multihash from data
func NewIndexHash(data []byte) (IndexHash, error) {
	h, err := mh.Sum(data, mh.BLAKE3, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to hash data: %w", err)
	}
	return IndexHash(h), nil
}

// Verify checks that the hash matches the provided data
func (h IndexHash) Verify(data []byte) error {
	decoded, err := mh.Decode(mh.Multihash(h))
	if err != nil {
		return fmt.Errorf("invalid multihash: %w", err)
	}

	if decoded.Code != mh.BLAKE3 {
		return fmt.Errorf("expected BLAKE3 hash, got 0x%x", decoded.Code)
	}

	computed, err := mh.Sum(data, decoded.Code, decoded.Length)
	if err != nil {
		return fmt.Errorf("hash computation failed: %w", err)
	}

	if !bytes.Equal(computed, h) {
		return fmt.Errorf("hash verification failed")
	}

	return nil
}

// Bytes returns the raw multihash bytes
func (h IndexHash) Bytes() []byte {
	return []byte(h)
}

// Hex returns the hex-encoded multihash
func (h IndexHash) Hex() string {
	return hex.EncodeToString(h)
}

// MerkleHash wraps a dbl-sha2-256 multihash for Bitcoin merkle trees
// Format: <0x56><0x20><32 bytes> = 34 bytes total
type MerkleHash []byte

// NewMerkleHash creates a dbl-sha2-256 multihash from data
func NewMerkleHash(data []byte) (MerkleHash, error) {
	h, err := mh.Sum(data, mh.DBL_SHA2_256, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to hash data: %w", err)
	}
	return MerkleHash(h), nil
}

// WrapMerkleHash wraps an existing 32-byte Bitcoin hash as a multihash
func WrapMerkleHash(hash [32]byte) (MerkleHash, error) {
	h, err := mh.Encode(hash[:], mh.DBL_SHA2_256)
	if err != nil {
		return nil, fmt.Errorf("failed to encode hash: %w", err)
	}
	return MerkleHash(h), nil
}

// WrapChainHash wraps a chainhash.Hash as a multihash
func WrapChainHash(hash chainhash.Hash) (MerkleHash, error) {
	h, err := mh.Encode(hash[:], mh.DBL_SHA2_256)
	if err != nil {
		return nil, fmt.Errorf("failed to encode hash: %w", err)
	}
	return MerkleHash(h), nil
}

// Verify checks that the hash matches the provided data
func (h MerkleHash) Verify(data []byte) error {
	decoded, err := mh.Decode(mh.Multihash(h))
	if err != nil {
		return fmt.Errorf("invalid multihash: %w", err)
	}

	if decoded.Code != mh.DBL_SHA2_256 {
		return fmt.Errorf("expected dbl-sha2-256 hash, got 0x%x", decoded.Code)
	}

	computed, err := mh.Sum(data, decoded.Code, decoded.Length)
	if err != nil {
		return fmt.Errorf("hash computation failed: %w", err)
	}

	if !bytes.Equal(computed, h) {
		return fmt.Errorf("hash verification failed")
	}

	return nil
}

// Raw extracts the 32-byte hash from the multihash
func (h MerkleHash) Raw() ([32]byte, error) {
	decoded, err := mh.Decode(mh.Multihash(h))
	if err != nil {
		return [32]byte{}, fmt.Errorf("invalid multihash: %w", err)
	}

	if len(decoded.Digest) != 32 {
		return [32]byte{}, fmt.Errorf("expected 32-byte digest, got %d bytes", len(decoded.Digest))
	}

	var raw [32]byte
	copy(raw[:], decoded.Digest)
	return raw, nil
}

// Bytes returns the raw multihash bytes
func (h MerkleHash) Bytes() []byte {
	return []byte(h)
}

// Hex returns the hex-encoded multihash
func (h MerkleHash) Hex() string {
	return hex.EncodeToString(h)
}

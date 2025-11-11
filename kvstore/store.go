package kvstore

import (
	"context"

	"github.com/bsv-blockchain/go-sdk/chainhash"
)

// Hash is a 32-byte hash
// Can be SHA256 (for txids/blocks) or BLAKE3 (for index nodes)
// Aliased to chainhash.Hash from go-sdk for compatibility with transaction types
type Hash = chainhash.Hash

// KVStore defines a generic key-value store interface
// Keys are variable-length byte slices to support multihash (34 bytes)
// or raw 32-byte hashes depending on the use case
type KVStore interface {
	// Put stores a key-value pair
	Put(ctx context.Context, key []byte, value []byte) error

	// Get retrieves a value by key
	// Returns nil if key doesn't exist
	Get(ctx context.Context, key []byte) ([]byte, error)

	// Delete removes a key-value pair
	Delete(ctx context.Context, key []byte) error

	// Close releases any resources
	Close() error
}

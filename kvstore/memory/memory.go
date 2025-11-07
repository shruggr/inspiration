package memory

import (
	"context"
	"sync"

	"github.com/shruggr/inspiration/kvstore"
)

// Store is an in-memory implementation of kvstore.KVStore
// Suitable for testing and development
type Store struct {
	data sync.Map // map[kvstore.Hash][]byte
}

// New creates a new in-memory KVStore
func New() *Store {
	return &Store{}
}

// Put stores a key-value pair
func (s *Store) Put(ctx context.Context, key kvstore.Hash, value []byte) error {
	s.data.Store(key, value)
	return nil
}

// Get retrieves a value by key
func (s *Store) Get(ctx context.Context, key kvstore.Hash) ([]byte, error) {
	val, ok := s.data.Load(key)
	if !ok {
		return nil, nil // Return nil for non-existent keys
	}
	return val.([]byte), nil
}

// Delete removes a key-value pair
func (s *Store) Delete(ctx context.Context, key kvstore.Hash) error {
	s.data.Delete(key)
	return nil
}

// Close releases any resources
func (s *Store) Close() error {
	return nil
}

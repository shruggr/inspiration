package memory

import (
	"context"
	"encoding/hex"
	"sync"
)

// Store is an in-memory implementation of kvstore.KVStore
// Suitable for testing and development
type Store struct {
	data sync.Map // map[string][]byte (hex-encoded keys)
}

// New creates a new in-memory KVStore
func New() *Store {
	return &Store{}
}

// Put stores a key-value pair
func (s *Store) Put(ctx context.Context, key []byte, value []byte) error {
	s.data.Store(hex.EncodeToString(key), value)
	return nil
}

// Get retrieves a value by key
func (s *Store) Get(ctx context.Context, key []byte) ([]byte, error) {
	val, ok := s.data.Load(hex.EncodeToString(key))
	if !ok {
		return nil, nil
	}
	return val.([]byte), nil
}

// Delete removes a key-value pair
func (s *Store) Delete(ctx context.Context, key []byte) error {
	s.data.Delete(hex.EncodeToString(key))
	return nil
}

// Close releases any resources
func (s *Store) Close() error {
	return nil
}

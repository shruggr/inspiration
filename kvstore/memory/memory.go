package memory

import (
	"context"
	"sync"
)

// Store is an in-memory implementation of kvstore.KVStore
type Store struct {
	data sync.Map
}

// New creates a new in-memory KVStore
func New() *Store {
	return &Store{}
}

// Put stores a key-value pair
func (s *Store) Put(_ context.Context, key []byte, value []byte) error {
	s.data.Store(string(key), value)
	return nil
}

// Get retrieves a value by key
func (s *Store) Get(_ context.Context, key []byte) ([]byte, error) {
	val, ok := s.data.Load(string(key))
	if !ok {
		return nil, nil
	}
	return val.([]byte), nil
}

// Delete removes a key-value pair
func (s *Store) Delete(_ context.Context, key []byte) error {
	s.data.Delete(string(key))
	return nil
}

// Has checks whether a key exists
func (s *Store) Has(_ context.Context, key []byte) (bool, error) {
	_, ok := s.data.Load(string(key))
	return ok, nil
}

// Close releases any resources
func (s *Store) Close() error {
	return nil
}

package badger

import (
	"context"
	"fmt"

	"github.com/dgraph-io/badger/v4"
	"github.com/shruggr/inspiration/kvstore"
)

// Store is a BadgerDB-backed implementation of kvstore.KVStore
type Store struct {
	db *badger.DB
}

// Config holds configuration for BadgerDB
type Config struct {
	DataDir string // Directory for data storage
}

// New creates a new BadgerDB-backed KVStore
func New(config *Config) (*Store, error) {
	if config.DataDir == "" {
		return nil, fmt.Errorf("DataDir is required")
	}

	opts := badger.DefaultOptions(config.DataDir)
	opts = opts.WithLogger(nil) // Disable badger's verbose logging

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger db: %w", err)
	}

	return &Store{db: db}, nil
}

// Put stores a key-value pair
func (s *Store) Put(ctx context.Context, key kvstore.Hash, value []byte) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key[:], value)
	})
}

// Get retrieves a value by key
func (s *Store) Get(ctx context.Context, key kvstore.Hash) ([]byte, error) {
	var value []byte

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key[:])
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			value = append([]byte{}, val...) // Copy value
			return nil
		})
	})

	if err == badger.ErrKeyNotFound {
		return nil, nil // Return nil for non-existent keys
	}
	if err != nil {
		return nil, err
	}

	return value, nil
}

// Delete removes a key-value pair
func (s *Store) Delete(ctx context.Context, key kvstore.Hash) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key[:])
	})
}

// Close releases all BadgerDB resources
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// RunGC runs BadgerDB garbage collection
// Call this periodically to reclaim space from deleted/updated entries
func (s *Store) RunGC(discardRatio float64) error {
	err := s.db.RunValueLogGC(discardRatio)
	if err == badger.ErrNoRewrite {
		return nil // Not an error - just means no rewrite was needed
	}
	return err
}

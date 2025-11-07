package memory

import (
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/shruggr/inspiration/cache"
	"github.com/shruggr/inspiration/kvstore"
)

// Cache is an in-memory LRU cache for index terms
type Cache struct {
	lru *lru.Cache[kvstore.Hash, []cache.IndexTerm]
	mu  sync.RWMutex
}

// New creates a new in-memory LRU cache with the specified size
func New(size int) (*Cache, error) {
	l, err := lru.New[kvstore.Hash, []cache.IndexTerm](size)
	if err != nil {
		return nil, err
	}

	return &Cache{
		lru: l,
	}, nil
}

// Get retrieves cached index terms for a transaction
func (c *Cache) Get(txid kvstore.Hash) ([]cache.IndexTerm, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lru.Get(txid)
}

// Put stores index terms for a transaction
func (c *Cache) Put(txid kvstore.Hash, terms []cache.IndexTerm) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lru.Add(txid, terms)
	return nil
}

// Delete removes cached terms for a transaction
func (c *Cache) Delete(txid kvstore.Hash) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lru.Remove(txid)
	return nil
}

// Clear removes all cached entries
func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lru.Purge()
	return nil
}

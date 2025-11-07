package models

import (
	"sync"
)

// BlockHeader represents a Bitcoin block header
type BlockHeader struct {
	Height     uint64
	Hash       []byte
	PrevHash   []byte
	MerkleRoot []byte
	Timestamp  uint32
	Bits       uint32
	Nonce      uint32
}

// HeaderChain tracks the current chain tip and validates headers
type HeaderChain struct {
	mu      sync.RWMutex
	headers map[uint64]*BlockHeader // height -> header
	tip     *BlockHeader
}

// NewHeaderChain creates a new header chain tracker
func NewHeaderChain() *HeaderChain {
	return &HeaderChain{
		headers: make(map[uint64]*BlockHeader),
	}
}

// AddHeader adds a new header to the chain
func (hc *HeaderChain) AddHeader(header *BlockHeader) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	// Store header by height
	hc.headers[header.Height] = header

	// Update tip if this is the new highest block
	if hc.tip == nil || header.Height > hc.tip.Height {
		hc.tip = header
	}

	return nil
}

// GetHeader retrieves a header by height
func (hc *HeaderChain) GetHeader(height uint64) (*BlockHeader, bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	header, ok := hc.headers[height]
	return header, ok
}

// GetTip returns the current chain tip
func (hc *HeaderChain) GetTip() *BlockHeader {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	return hc.tip
}

// Reorg handles a chain reorganization by removing headers above a certain height
func (hc *HeaderChain) Reorg(height uint64) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	// Remove all headers above the reorg height
	for h := range hc.headers {
		if h > height {
			delete(hc.headers, h)
		}
	}

	// Update tip
	if hc.tip != nil && hc.tip.Height > height {
		hc.tip = hc.headers[height]
	}
}

// Height returns the current tip height
func (hc *HeaderChain) Height() uint64 {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	if hc.tip == nil {
		return 0
	}
	return hc.tip.Height
}

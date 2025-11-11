package messages

import (
	"github.com/shruggr/inspiration/kvstore"
)

// BlockMessage represents a finalized block from the network
type BlockMessage struct {
	Header        []byte          // 80-byte block header
	TxCount       uint64          // Number of transactions in block
	SizeInBytes   uint64          // Size of block in bytes
	SubtreeHashes []kvstore.Hash  // Merkle roots of subtrees
	CoinbaseTx    []byte          // Coinbase transaction bytes
	Height        uint32          // Block height
	MerkleRoot    kvstore.Hash    // Merkle root extracted from header
}

// SubtreeMessage represents a subtree available for download
type SubtreeMessage struct {
	Hash   kvstore.Hash  // Merkle root of the subtree
	URL    string        // Where to fetch subtree data
	PeerID string        // Peer that sent this message
}

// SubtreeData contains transaction IDs and optionally fetched transaction data
// Used during subtree processing to track which transactions we have vs need to fetch
type SubtreeData struct {
	MerkleRoot kvstore.Hash              // Merkle root of this subtree
	TxIDs      []kvstore.Hash            // Complete list of transaction IDs in subtree
	TxData     map[kvstore.Hash][]byte   // Raw transaction bytes (only for newly fetched txs)
}

// BlockHeader contains parsed fields from an 80-byte block header
type BlockHeader struct {
	Version       int32
	PrevBlockHash kvstore.Hash
	MerkleRoot    kvstore.Hash
	Timestamp     uint32
	Bits          uint32
	Nonce         uint32
}

package messages

import (
	"encoding/binary"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/teranode/util/kafka/kafka_message"
	"github.com/shruggr/inspiration/kvstore"
)

// ParseBlockMessage converts a Kafka protobuf block message to our internal representation
func ParseBlockMessage(msg *kafkamessage.KafkaBlocksFinalTopicMessage) (*BlockMessage, error) {
	if len(msg.Header) != 80 {
		return nil, fmt.Errorf("invalid block header length: got %d, expected 80", len(msg.Header))
	}

	// Parse subtree hashes from bytes
	subtreeHashes := make([]kvstore.Hash, len(msg.SubtreeHashes))
	for i, hashBytes := range msg.SubtreeHashes {
		if len(hashBytes) != 32 {
			return nil, fmt.Errorf("invalid subtree hash length at index %d: got %d, expected 32", i, len(hashBytes))
		}
		hash, err := chainhash.NewHash(hashBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse subtree hash at index %d: %w", i, err)
		}
		subtreeHashes[i] = *hash
	}

	// Extract merkle root from block header (bytes 36-68)
	merkleRootBytes := msg.Header[36:68]
	merkleRoot, err := chainhash.NewHash(merkleRootBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse merkle root from header: %w", err)
	}

	return &BlockMessage{
		Header:        msg.Header,
		TxCount:       msg.TransactionCount,
		SizeInBytes:   msg.SizeInBytes,
		SubtreeHashes: subtreeHashes,
		CoinbaseTx:    msg.CoinbaseTx,
		Height:        msg.Height,
		MerkleRoot:    *merkleRoot,
	}, nil
}

// ParseSubtreeMessage converts a Kafka protobuf subtree message to our internal representation
func ParseSubtreeMessage(msg *kafkamessage.KafkaSubtreeTopicMessage) (*SubtreeMessage, error) {
	if msg.Hash == "" {
		return nil, fmt.Errorf("subtree hash is empty")
	}
	if msg.URL == "" {
		return nil, fmt.Errorf("subtree URL is empty")
	}

	// Parse the hex hash string
	hash, err := chainhash.NewHashFromHex(msg.Hash)
	if err != nil {
		return nil, fmt.Errorf("failed to parse subtree hash: %w", err)
	}

	return &SubtreeMessage{
		Hash:   *hash,
		URL:    msg.URL,
		PeerID: msg.PeerId,
	}, nil
}

// ParseBlockHeader extracts key fields from an 80-byte block header
func ParseBlockHeader(header []byte) (*BlockHeader, error) {
	if len(header) != 80 {
		return nil, fmt.Errorf("invalid block header length: got %d, expected 80", len(header))
	}

	// Block header structure:
	// 0-4:   version (int32)
	// 4-36:  prev block hash (32 bytes)
	// 36-68: merkle root (32 bytes)
	// 68-72: timestamp (uint32)
	// 72-76: bits (uint32)
	// 76-80: nonce (uint32)

	version := binary.LittleEndian.Uint32(header[0:4])

	prevBlockHash, err := chainhash.NewHash(header[4:36])
	if err != nil {
		return nil, fmt.Errorf("failed to parse prev block hash: %w", err)
	}

	merkleRoot, err := chainhash.NewHash(header[36:68])
	if err != nil {
		return nil, fmt.Errorf("failed to parse merkle root: %w", err)
	}

	timestamp := binary.LittleEndian.Uint32(header[68:72])
	bits := binary.LittleEndian.Uint32(header[72:76])
	nonce := binary.LittleEndian.Uint32(header[76:80])

	return &BlockHeader{
		Version:       int32(version),
		PrevBlockHash: *prevBlockHash,
		MerkleRoot:    *merkleRoot,
		Timestamp:     timestamp,
		Bits:          bits,
		Nonce:         nonce,
	}, nil
}

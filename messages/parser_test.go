package messages

import (
	"encoding/hex"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	kafkamessage "github.com/bsv-blockchain/teranode/util/kafka/kafka_message"
)

func TestParseBlockMessage(t *testing.T) {
	// Create a minimal valid 80-byte block header
	header := make([]byte, 80)
	// Version (4 bytes)
	header[0] = 0x01
	header[1] = 0x00
	header[2] = 0x00
	header[3] = 0x00
	// Prev block hash (32 bytes at offset 4)
	// Merkle root (32 bytes at offset 36)
	copy(header[36:68], make([]byte, 32)) // All zeros for test
	// Timestamp, bits, nonce (12 bytes at offset 68)

	subtreeHash1 := make([]byte, 32)
	subtreeHash1[0] = 0x01
	subtreeHash2 := make([]byte, 32)
	subtreeHash2[0] = 0x02

	protoMsg := &kafkamessage.KafkaBlocksFinalTopicMessage{
		Header:           header,
		TransactionCount: 100,
		SizeInBytes:      50000,
		SubtreeHashes:    [][]byte{subtreeHash1, subtreeHash2},
		CoinbaseTx:       []byte{0x01, 0x02, 0x03},
		Height:           12345,
	}

	blockMsg, err := ParseBlockMessage(protoMsg)
	if err != nil {
		t.Fatalf("ParseBlockMessage failed: %v", err)
	}

	if blockMsg.TxCount != 100 {
		t.Errorf("Expected TxCount 100, got %d", blockMsg.TxCount)
	}
	if blockMsg.Height != 12345 {
		t.Errorf("Expected Height 12345, got %d", blockMsg.Height)
	}
	if len(blockMsg.SubtreeHashes) != 2 {
		t.Errorf("Expected 2 subtree hashes, got %d", len(blockMsg.SubtreeHashes))
	}
}

func TestParseBlockMessageInvalidHeader(t *testing.T) {
	protoMsg := &kafkamessage.KafkaBlocksFinalTopicMessage{
		Header:           make([]byte, 79), // Wrong size
		TransactionCount: 100,
		SizeInBytes:      50000,
		Height:           12345,
	}

	_, err := ParseBlockMessage(protoMsg)
	if err == nil {
		t.Fatal("Expected error for invalid header length, got nil")
	}
}

func TestParseSubtreeMessage(t *testing.T) {
	hashStr := "0000000000000000000000000000000000000000000000000000000000000001"

	protoMsg := &kafkamessage.KafkaSubtreeTopicMessage{
		Hash:    hashStr,
		URL:     "http://example.com/subtree",
		PeerId:  "peer123",
	}

	subtreeMsg, err := ParseSubtreeMessage(protoMsg)
	if err != nil {
		t.Fatalf("ParseSubtreeMessage failed: %v", err)
	}

	if subtreeMsg.URL != "http://example.com/subtree" {
		t.Errorf("Expected URL http://example.com/subtree, got %s", subtreeMsg.URL)
	}
	if subtreeMsg.PeerID != "peer123" {
		t.Errorf("Expected PeerID peer123, got %s", subtreeMsg.PeerID)
	}

	// Verify hash was parsed correctly
	expectedHash, _ := chainhash.NewHashFromHex(hashStr)
	if subtreeMsg.Hash != *expectedHash {
		t.Errorf("Hash mismatch")
	}
}

func TestParseBlockHeader(t *testing.T) {
	// Create a test block header
	header := make([]byte, 80)

	// Version: 1
	header[0] = 0x01
	header[1] = 0x00
	header[2] = 0x00
	header[3] = 0x00

	// Prev block hash (bytes 4-36)
	prevHashBytes, _ := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000001")
	copy(header[4:36], prevHashBytes)

	// Merkle root (bytes 36-68)
	merkleRootBytes, _ := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000002")
	copy(header[36:68], merkleRootBytes)

	// Timestamp: 1234567890 (bytes 68-72)
	header[68] = 0xD2
	header[69] = 0x02
	header[70] = 0x96
	header[71] = 0x49

	// Bits: 0x1d00ffff (bytes 72-76)
	header[72] = 0xFF
	header[73] = 0xFF
	header[74] = 0x00
	header[75] = 0x1D

	// Nonce: 2083236893 (bytes 76-80)
	header[76] = 0x1D
	header[77] = 0xAC
	header[78] = 0x2B
	header[79] = 0x7C

	parsedHeader, err := ParseBlockHeader(header)
	if err != nil {
		t.Fatalf("ParseBlockHeader failed: %v", err)
	}

	if parsedHeader.Version != 1 {
		t.Errorf("Expected Version 1, got %d", parsedHeader.Version)
	}
	if parsedHeader.Timestamp != 1234567890 {
		t.Errorf("Expected Timestamp 1234567890, got %d", parsedHeader.Timestamp)
	}
}

package kafka

import (
	"testing"

	kafkamessage "github.com/bsv-blockchain/teranode/util/kafka/kafka_message"
	"google.golang.org/protobuf/proto"
)

func TestSubtreeMessageRoundtrip(t *testing.T) {
	orig := &kafkamessage.KafkaSubtreeTopicMessage{
		Hash:   "abc123def456",
		URL:    "http://teranode:8080/subtree/abc123def456",
		PeerId: "peer-1",
	}

	data, err := proto.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got kafkamessage.KafkaSubtreeTopicMessage
	if err := proto.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.GetHash() != orig.GetHash() {
		t.Errorf("hash: got %q, want %q", got.GetHash(), orig.GetHash())
	}
	if got.GetURL() != orig.GetURL() {
		t.Errorf("URL: got %q, want %q", got.GetURL(), orig.GetURL())
	}
	if got.GetPeerId() != orig.GetPeerId() {
		t.Errorf("peer_id: got %q, want %q", got.GetPeerId(), orig.GetPeerId())
	}
}

func TestBlocksFinalMessageRoundtrip(t *testing.T) {
	header := []byte{0x01, 0x02, 0x03, 0x04}
	subtreeHashes := [][]byte{
		{0xaa, 0xbb, 0xcc},
		{0xdd, 0xee, 0xff},
	}
	coinbaseTx := []byte{0x10, 0x20, 0x30}

	orig := &kafkamessage.KafkaBlocksFinalTopicMessage{
		Header:           header,
		TransactionCount: 42,
		SizeInBytes:      1024,
		SubtreeHashes:    subtreeHashes,
		CoinbaseTx:       coinbaseTx,
		Height:           100_000,
	}

	data, err := proto.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got kafkamessage.KafkaBlocksFinalTopicMessage
	if err := proto.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.GetHeight() != orig.GetHeight() {
		t.Errorf("height: got %d, want %d", got.GetHeight(), orig.GetHeight())
	}
	if got.GetTransactionCount() != orig.GetTransactionCount() {
		t.Errorf("tx_count: got %d, want %d", got.GetTransactionCount(), orig.GetTransactionCount())
	}
	if got.GetSizeInBytes() != orig.GetSizeInBytes() {
		t.Errorf("size: got %d, want %d", got.GetSizeInBytes(), orig.GetSizeInBytes())
	}
	if len(got.GetSubtreeHashes()) != len(orig.GetSubtreeHashes()) {
		t.Fatalf("subtree_hashes len: got %d, want %d", len(got.GetSubtreeHashes()), len(orig.GetSubtreeHashes()))
	}
	for i := range orig.GetSubtreeHashes() {
		if string(got.GetSubtreeHashes()[i]) != string(orig.GetSubtreeHashes()[i]) {
			t.Errorf("subtree_hashes[%d] mismatch", i)
		}
	}
	if string(got.GetHeader()) != string(orig.GetHeader()) {
		t.Errorf("header mismatch")
	}
	if string(got.GetCoinbaseTx()) != string(orig.GetCoinbaseTx()) {
		t.Errorf("coinbase_tx mismatch")
	}
}

package indexnode

import (
	"bytes"
	"testing"
)

func TestLeafEntryMarshalUnmarshal(t *testing.T) {
	txid := make([]byte, 32)
	txid[0] = 0xAB
	entry := LeafEntry{
		TxID:            txid,
		SubtreePosition: 42,
		Vouts:           []uint32{0, 3, 7},
	}

	data := entry.Marshal()

	decoded, bytesRead, err := UnmarshalLeafEntry(data)
	if err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if bytesRead != len(data) {
		t.Fatalf("bytes read: got %d, want %d", bytesRead, len(data))
	}
	if !bytes.Equal(decoded.TxID, entry.TxID) {
		t.Error("txid mismatch")
	}
	if decoded.SubtreePosition != 42 {
		t.Errorf("position: got %d, want 42", decoded.SubtreePosition)
	}
	if len(decoded.Vouts) != 3 || decoded.Vouts[0] != 0 || decoded.Vouts[1] != 3 || decoded.Vouts[2] != 7 {
		t.Errorf("vouts mismatch: got %v", decoded.Vouts)
	}
}

func TestLeafEntryListRoundTrip(t *testing.T) {
	entries := []LeafEntry{
		{TxID: make([]byte, 32), SubtreePosition: 0, Vouts: []uint32{0}},
		{TxID: make([]byte, 32), SubtreePosition: 5, Vouts: []uint32{1, 2}},
		{TxID: make([]byte, 32), SubtreePosition: 100, Vouts: []uint32{0, 4, 8, 12}},
	}
	entries[0].TxID[0] = 1
	entries[1].TxID[0] = 2
	entries[2].TxID[0] = 3

	data := MarshalLeafEntryList(entries)

	decoded, err := UnmarshalLeafEntryList(data)
	if err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(decoded) != 3 {
		t.Fatalf("entry count: got %d, want 3", len(decoded))
	}
	if decoded[2].SubtreePosition != 100 {
		t.Errorf("entry 2 position: got %d, want 100", decoded[2].SubtreePosition)
	}
	if len(decoded[2].Vouts) != 4 {
		t.Errorf("entry 2 vouts: got %d, want 4", len(decoded[2].Vouts))
	}
}

func TestLeafEntryCompactSize(t *testing.T) {
	entry := LeafEntry{
		TxID:            make([]byte, 32),
		SubtreePosition: 0,
		Vouts:           []uint32{0},
	}
	data := entry.Marshal()
	// txid(32) + position varint(1) + count varint(1) + vout varint(1) = 35 bytes
	if len(data) != 35 {
		t.Errorf("compact: got %d bytes, want 35", len(data))
	}
}

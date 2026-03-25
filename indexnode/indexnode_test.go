package indexnode

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"testing"
)

// buildTagNode creates a TagNode with the given string keys and 32-byte values.
// Data section is padded at offset 0 so no entry has offset 0 (which getDataAt treats as nil).
func buildTagNode(keys []string, values [][]byte) *IndexNode {
	node := NewTagNode()
	// Pad first byte so no real data starts at offset 0
	dataSection := []byte{0x00}
	for i, key := range keys {
		offset := uint32(len(dataSection))
		lenBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lenBuf, uint32(len(key)))
		dataSection = append(dataSection, lenBuf...)
		dataSection = append(dataSection, []byte(key)...)
		if err := node.AddEntry(nil, values[i], offset); err != nil {
			panic(err)
		}
	}
	node.SetDataSection(dataSection)
	node.Sort()
	return node
}

func randHash() []byte {
	h := make([]byte, 32)
	rand.Read(h)
	return h
}

func TestTagNode_FindByData(t *testing.T) {
	keys := []string{"image/png", "application/json", "text/plain", "image/jpeg"}
	values := make([][]byte, len(keys))
	for i := range values {
		values[i] = randHash()
	}

	node := buildTagNode(keys, values)

	// FindByData should find each key
	for i, key := range keys {
		val, found := node.FindByData([]byte(key))
		if !found {
			t.Fatalf("expected to find key %q", key)
		}
		if !bytes.Equal(val, values[i]) {
			t.Fatalf("value mismatch for key %q", key)
		}
	}

	// Missing key
	_, found := node.FindByData([]byte("video/mp4"))
	if found {
		t.Fatal("expected not found for missing key")
	}
}

func TestTagNode_ScanPrefix(t *testing.T) {
	keys := []string{"image/png", "image/jpeg", "image/gif", "text/plain", "text/html", "application/json"}
	values := make([][]byte, len(keys))
	for i := range values {
		values[i] = randHash()
	}

	node := buildTagNode(keys, values)

	// Scan for "image/" prefix
	results := node.ScanPrefix([]byte("image/"))
	if len(results) != 3 {
		t.Fatalf("expected 3 results for prefix 'image/', got %d", len(results))
	}

	// Scan for "text/" prefix
	results = node.ScanPrefix([]byte("text/"))
	if len(results) != 2 {
		t.Fatalf("expected 2 results for prefix 'text/', got %d", len(results))
	}

	// Scan for prefix with no matches
	results = node.ScanPrefix([]byte("video/"))
	if len(results) != 0 {
		t.Fatalf("expected 0 results for prefix 'video/', got %d", len(results))
	}
}

func TestTagNode_ScanRange(t *testing.T) {
	keys := []string{"apple", "banana", "cherry", "date", "elderberry", "fig"}
	values := make([][]byte, len(keys))
	for i := range values {
		values[i] = randHash()
	}

	node := buildTagNode(keys, values)

	// Range from "banana" to "elderberry" (inclusive start, exclusive end)
	results := node.ScanRange([]byte("banana"), []byte("elderberry"))
	if len(results) != 3 {
		t.Fatalf("expected 3 results (banana, cherry, date), got %d", len(results))
	}

	// Range that covers everything
	results = node.ScanRange([]byte("a"), []byte("z"))
	if len(results) != 6 {
		t.Fatalf("expected 6 results, got %d", len(results))
	}

	// Range with no matches
	results = node.ScanRange([]byte("grape"), []byte("kiwi"))
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestTagNode_MarshalRoundTrip(t *testing.T) {
	keys := []string{"foo", "bar", "baz"}
	values := make([][]byte, len(keys))
	for i := range values {
		values[i] = randHash()
	}

	node := buildTagNode(keys, values)

	data, err := node.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	node2, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}

	if len(node2.Entries) != len(node.Entries) {
		t.Fatalf("entry count mismatch: %d vs %d", len(node2.Entries), len(node.Entries))
	}

	if !node2.HasData || !node2.SortByData {
		t.Fatal("flags not preserved")
	}

	// Verify all entries match after round-trip
	for i, entry := range node.Entries {
		entry2 := node2.Entries[i]
		if !bytes.Equal(entry.Value, entry2.Value) {
			t.Fatalf("value mismatch at entry %d", i)
		}
		if entry.Offset != entry2.Offset {
			t.Fatalf("offset mismatch at entry %d", i)
		}
	}

	if !bytes.Equal(node.DataSection, node2.DataSection) {
		t.Fatal("data section mismatch")
	}

	// FindByData should still work after round-trip
	for _, key := range keys {
		_, found := node2.FindByData([]byte(key))
		if !found {
			t.Fatalf("FindByData failed after round-trip for key %q", key)
		}
	}
}

func TestFixedKeyNode_Find(t *testing.T) {
	node := NewFixedKeyNode(32)

	keys := make([][]byte, 5)
	values := make([][]byte, 5)
	for i := range keys {
		keys[i] = randHash()
		values[i] = randHash()
		if err := node.AddEntry(keys[i], values[i], 0); err != nil {
			t.Fatal(err)
		}
	}

	node.Sort()

	for i, key := range keys {
		val, found := node.Find(key)
		if !found {
			t.Fatalf("expected to find key %d", i)
		}
		if !bytes.Equal(val, values[i]) {
			t.Fatalf("value mismatch for key %d", i)
		}
	}

	// Missing key
	_, found := node.Find(randHash())
	if found {
		t.Fatal("expected not found for random key")
	}
}

func TestFixedKeyNode_MarshalRoundTrip(t *testing.T) {
	node := NewFixedKeyNode(32)

	for i := 0; i < 10; i++ {
		if err := node.AddEntry(randHash(), randHash(), 0); err != nil {
			t.Fatal(err)
		}
	}
	node.Sort()

	data, err := node.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	node2, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}

	if node2.KeySize != 32 || node2.ValueSize != 32 {
		t.Fatal("sizes not preserved")
	}
	if node2.HasData || node2.SortByData {
		t.Fatal("flags should be false")
	}
	if len(node2.Entries) != 10 {
		t.Fatalf("expected 10 entries, got %d", len(node2.Entries))
	}

	for i, entry := range node.Entries {
		if !bytes.Equal(entry.Key, node2.Entries[i].Key) {
			t.Fatalf("key mismatch at %d", i)
		}
		if !bytes.Equal(entry.Value, node2.Entries[i].Value) {
			t.Fatalf("value mismatch at %d", i)
		}
	}
}

func TestFixedKeyNode_Sorting(t *testing.T) {
	node := NewFixedKeyNode(4)

	// Add entries in reverse order
	entries := [][]byte{
		{0x00, 0x00, 0x00, 0x03},
		{0x00, 0x00, 0x00, 0x01},
		{0x00, 0x00, 0x00, 0x02},
	}
	for _, key := range entries {
		if err := node.AddEntry(key, randHash(), 0); err != nil {
			t.Fatal(err)
		}
	}

	node.Sort()

	// Verify sorted order
	for i := 1; i < len(node.Entries); i++ {
		if bytes.Compare(node.Entries[i-1].Key, node.Entries[i].Key) >= 0 {
			t.Fatalf("entries not sorted at index %d", i)
		}
	}
}

func TestFixedKeyNode_ScanPrefix(t *testing.T) {
	node := NewFixedKeyNode(4)

	// Keys with shared prefix
	keys := [][]byte{
		{0xAA, 0xBB, 0x01, 0x00},
		{0xAA, 0xBB, 0x02, 0x00},
		{0xAA, 0xBB, 0x03, 0x00},
		{0xAA, 0xCC, 0x01, 0x00},
		{0xBB, 0x00, 0x00, 0x00},
	}
	for _, key := range keys {
		if err := node.AddEntry(key, randHash(), 0); err != nil {
			t.Fatal(err)
		}
	}
	node.Sort()

	results := node.ScanPrefix([]byte{0xAA, 0xBB})
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	results = node.ScanPrefix([]byte{0xAA})
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
}

func TestArrayNode_GetByIndex(t *testing.T) {
	node := NewArrayNode(8)

	values := make([][]byte, 5)
	for i := range values {
		values[i] = make([]byte, 8)
		binary.BigEndian.PutUint64(values[i], uint64(i*100))
		if err := node.AddEntry(nil, values[i], 0); err != nil {
			t.Fatal(err)
		}
	}

	for i, expected := range values {
		val, found := node.GetByIndex(i)
		if !found {
			t.Fatalf("expected to find index %d", i)
		}
		if !bytes.Equal(val, expected) {
			t.Fatalf("value mismatch at index %d", i)
		}
	}

	// Out of bounds
	_, found := node.GetByIndex(-1)
	if found {
		t.Fatal("expected not found for -1")
	}
	_, found = node.GetByIndex(5)
	if found {
		t.Fatal("expected not found for 5")
	}
}

func TestArrayNode_MarshalRoundTrip(t *testing.T) {
	node := NewArrayNode(16)

	for i := 0; i < 3; i++ {
		val := make([]byte, 16)
		rand.Read(val)
		if err := node.AddEntry(nil, val, 0); err != nil {
			t.Fatal(err)
		}
	}

	data, err := node.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	node2, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}

	if node2.KeySize != 0 || node2.ValueSize != 16 {
		t.Fatal("sizes not preserved")
	}
	if len(node2.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(node2.Entries))
	}

	for i := range node.Entries {
		if !bytes.Equal(node.Entries[i].Value, node2.Entries[i].Value) {
			t.Fatalf("value mismatch at %d", i)
		}
	}
}

func TestHashDeterminism(t *testing.T) {
	// Build two identical nodes and verify same hash
	keys := []string{"alpha", "beta", "gamma"}
	values := [][]byte{
		bytes.Repeat([]byte{0x01}, 32),
		bytes.Repeat([]byte{0x02}, 32),
		bytes.Repeat([]byte{0x03}, 32),
	}

	node1 := buildTagNode(keys, values)
	node2 := buildTagNode(keys, values)

	hash1, err := node1.Hash()
	if err != nil {
		t.Fatal(err)
	}

	hash2, err := node2.Hash()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(hash1, hash2) {
		t.Fatal("identical nodes produced different hashes")
	}

	// Different content should produce different hash
	values2 := [][]byte{
		bytes.Repeat([]byte{0x04}, 32),
		bytes.Repeat([]byte{0x05}, 32),
		bytes.Repeat([]byte{0x06}, 32),
	}
	node3 := buildTagNode(keys, values2)
	hash3, err := node3.Hash()
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Equal(hash1, hash3) {
		t.Fatal("different nodes produced same hash")
	}
}

package indexnode

import (
	"bytes"
	"testing"
)

func TestIndexNodeMarshalUnmarshal(t *testing.T) {
	// Create a test node
	node := NewIndexNode()

	// Add some test entries
	entries := []struct {
		key       string
		childHash []byte
	}{
		{"address:1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", make([]byte, 32)},
		{"address:1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2", make([]byte, 32)},
		{"op_return:1sat", make([]byte, 32)},
		{"op_return:bsv20", make([]byte, 32)},
	}

	for i, e := range entries {
		// Fill child hash with test data
		for j := range e.childHash {
			e.childHash[j] = byte(i)
		}
		if err := node.AddEntry([]byte(e.key), e.childHash, nil); err != nil {
			t.Fatalf("AddEntry failed: %v", err)
		}
	}

	// Sort the node
	node.Sort()

	// Marshal
	data, err := node.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	t.Logf("Marshaled size: %d bytes", len(data))

	// Unmarshal
	node2, err := UnmarshalIndexNode(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify entry count
	if len(node2.Entries) != len(node.Entries) {
		t.Fatalf("Entry count mismatch: got %d, want %d", len(node2.Entries), len(node.Entries))
	}

	// Verify each entry
	for i := range node.Entries {
		if !bytes.Equal(node2.Entries[i].Key, node.Entries[i].Key) {
			t.Errorf("Entry %d key mismatch: got %s, want %s",
				i, node2.Entries[i].Key, node.Entries[i].Key)
		}
		if !bytes.Equal(node2.Entries[i].ChildHash, node.Entries[i].ChildHash) {
			t.Errorf("Entry %d child hash mismatch", i)
		}
	}
}

func TestIndexNodeFind(t *testing.T) {
	node := NewIndexNode()

	// Add entries
	testData := map[string][]byte{
		"apple":  {1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		"banana": {2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
		"cherry": {3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3},
	}

	for key, hash := range testData {
		if err := node.AddEntry([]byte(key), hash, nil); err != nil {
			t.Fatalf("AddEntry failed: %v", err)
		}
	}

	node.Sort()

	// Test finding existing keys
	for key, expectedHash := range testData {
		hash, found := node.Find([]byte(key))
		if !found {
			t.Errorf("Key %s not found", key)
		}
		if !bytes.Equal(hash, expectedHash) {
			t.Errorf("Hash mismatch for key %s", key)
		}
	}

	// Test finding non-existent key
	_, found := node.Find([]byte("orange"))
	if found {
		t.Error("Found non-existent key 'orange'")
	}
}

func TestIndexNodeHash(t *testing.T) {
	node := NewIndexNode()

	childHash := make([]byte, 32)
	for i := range childHash {
		childHash[i] = byte(i)
	}

	if err := node.AddEntry([]byte("test_key"), childHash, nil); err != nil {
		t.Fatalf("AddEntry failed: %v", err)
	}

	hash1, err := node.Hash()
	if err != nil {
		t.Fatalf("Hash failed: %v", err)
	}

	if len(hash1) != 32 {
		t.Errorf("Hash length: got %d, want 32", len(hash1))
	}

	// Same data should produce same hash
	hash2, err := node.Hash()
	if err != nil {
		t.Fatalf("Hash failed: %v", err)
	}

	if !bytes.Equal(hash1, hash2) {
		t.Error("Hash not deterministic")
	}

	t.Logf("Node hash: %x", hash1)
}

func TestIndexNodeSorting(t *testing.T) {
	node := NewIndexNode()

	// Add entries in random order
	keys := []string{"zebra", "apple", "mango", "banana"}
	childHash := make([]byte, 32)

	for _, key := range keys {
		if err := node.AddEntry([]byte(key), childHash, nil); err != nil {
			t.Fatalf("AddEntry failed: %v", err)
		}
	}

	// Sort
	node.Sort()

	// Verify sorted order
	expected := []string{"apple", "banana", "mango", "zebra"}
	for i, expectedKey := range expected {
		if string(node.Entries[i].Key) != expectedKey {
			t.Errorf("Entry %d: got %s, want %s", i, node.Entries[i].Key, expectedKey)
		}
	}
}

func TestHashKey(t *testing.T) {
	key := []byte("test_key")
	hash := HashKey(key)

	if len(hash) != 32 {
		t.Errorf("Hash length: got %d, want 32", len(hash))
	}

	// Same key should produce same hash
	hash2 := HashKey(key)
	if !bytes.Equal(hash, hash2) {
		t.Error("HashKey not deterministic")
	}

	// Different keys should produce different hashes
	hash3 := HashKey([]byte("different_key"))
	if bytes.Equal(hash, hash3) {
		t.Error("Different keys produced same hash")
	}

	t.Logf("Hash of 'test_key': %x", hash)
}

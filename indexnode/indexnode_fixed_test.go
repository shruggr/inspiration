package indexnode

import (
	"bytes"
	"testing"
)

func TestFixedKeyIndexNode(t *testing.T) {
	// Create a fixed-width node with 32-byte keys (for hashes)
	node := NewFixedKeyIndexNode(32)

	// Add some test entries with 32-byte keys
	for i := 0; i < 4; i++ {
		key := make([]byte, 32)
		for j := range key {
			key[j] = byte(i)
		}

		childHash := make([]byte, 32)
		for j := range childHash {
			childHash[j] = byte(i + 100)
		}

		if err := node.AddEntry(key, childHash, nil); err != nil {
			t.Fatalf("AddEntry failed: %v", err)
		}
	}

	// Sort
	node.Sort()

	// Marshal
	data, err := node.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	t.Logf("Fixed-width marshaled size: %d bytes", len(data))
	t.Logf("Expected size: %d bytes (header) + %d entries × %d bytes/entry = %d",
		headerSize, len(node.Entries), 32+32, headerSize+(len(node.Entries)*64))

	// Unmarshal
	node2, err := UnmarshalIndexNode(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify mode
	if node2.Mode != indexModePointerKey {
		t.Errorf("Mode mismatch: got %d, want %d", node2.Mode, indexModePointerKey)
	}

	// Verify fixed key size
	if node2.FixedKeySize != 32 {
		t.Errorf("FixedKeySize mismatch: got %d, want 32", node2.FixedKeySize)
	}

	// Verify entry count
	if len(node2.Entries) != len(node.Entries) {
		t.Fatalf("Entry count mismatch: got %d, want %d", len(node2.Entries), len(node.Entries))
	}

	// Verify each entry
	for i := range node.Entries {
		if !bytes.Equal(node2.Entries[i].Key, node.Entries[i].Key) {
			t.Errorf("Entry %d key mismatch", i)
		}
		if !bytes.Equal(node2.Entries[i].ChildHash, node.Entries[i].ChildHash) {
			t.Errorf("Entry %d child hash mismatch", i)
		}
	}
}

func TestFixedKeyValidation(t *testing.T) {
	node := NewFixedKeyIndexNode(32)

	// Try to add entry with wrong key size
	wrongSizeKey := make([]byte, 16) // Wrong size!
	childHash := make([]byte, 32)

	err := node.AddEntry(wrongSizeKey, childHash, nil)
	if err == nil {
		t.Error("Expected error for wrong key size, got nil")
	}
}

func TestFixedVsVariableSizeComparison(t *testing.T) {
	// Create same data in both formats
	numEntries := 100

	// Fixed-width node with 32-byte keys
	fixedNode := NewFixedKeyIndexNode(32)
	for i := 0; i < numEntries; i++ {
		key := make([]byte, 32)
		for j := range key {
			key[j] = byte(i)
		}
		childHash := make([]byte, 32)
		fixedNode.AddEntry(key, childHash, nil)
	}

	// Variable-width node with same 32-byte keys
	varNode := NewIndexNode()
	for i := 0; i < numEntries; i++ {
		key := make([]byte, 32)
		for j := range key {
			key[j] = byte(i)
		}
		childHash := make([]byte, 32)
		varNode.AddEntry(key, childHash, nil)
	}

	// Marshal both
	fixedData, _ := fixedNode.Marshal()
	varData, _ := varNode.Marshal()

	t.Logf("Fixed-width size:    %d bytes", len(fixedData))
	t.Logf("Variable-width size: %d bytes", len(varData))
	t.Logf("Savings:             %d bytes (%.1f%%)", len(varData)-len(fixedData),
		float64(len(varData)-len(fixedData))/float64(len(varData))*100)

	// Fixed should be smaller (no offset table, no key length fields)
	if len(fixedData) >= len(varData) {
		t.Error("Fixed-width format should be smaller than variable-width for same data")
	}
}

func TestFixedKeyFind(t *testing.T) {
	node := NewFixedKeyIndexNode(32)

	// Add test data
	testData := make(map[string][]byte)
	for i := 0; i < 10; i++ {
		key := make([]byte, 32)
		key[0] = byte(i)
		childHash := make([]byte, 32)
		childHash[0] = byte(i + 100)

		node.AddEntry(key, childHash, nil)
		testData[string(key)] = childHash
	}

	node.Sort()

	// Test finding each key
	for keyStr, expectedHash := range testData {
		key := []byte(keyStr)
		hash, found := node.Find(key)
		if !found {
			t.Errorf("Key not found: %x", key[0])
		}
		if !bytes.Equal(hash, expectedHash) {
			t.Errorf("Hash mismatch for key %x", key[0])
		}
	}
}

package indexnode

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"

	"lukechampine.com/blake3"
)

// IndexNode represents a node in the hierarchical index tree
// Supports both fixed-width (hash keys) and variable-width (text) modes
type IndexNode struct {
	Entries     []*IndexNodeEntry
	Mode        byte   // indexModeFixed or indexModeVariable
	FixedKeySize uint16 // Only used for Mode=0 (fixed-width keys)
}

// IndexNodeEntry represents a single entry in an index node
type IndexNodeEntry struct {
	Key       []byte // Original key (variable length)
	ChildHash []byte // Hash of child node or txid (32 bytes)
	Data      []byte // Additional data (variable length)
}

// Binary format supports 4 modes (bitmask):
//   Bit 0: Key type (0=value/variable, 1=pointer/fixed 32 bytes)
//   Bit 1: Has data (0=no data, 1=has data section)
//
// MODE 0 (0b00): Value keys, no data
// MODE 1 (0b01): Pointer keys (fixed 32 bytes), no data
// MODE 2 (0b10): Value keys, with data
// MODE 3 (0b11): Pointer keys (fixed 32 bytes), with data
//
// POINTER KEY FORMAT (Modes 1 & 3):
// ┌──────────────────────────────────┐
// │ Header (8 bytes)                 │
// │ - version: 1 byte                │
// │ - mode: 1 byte (bitmask)         │
// │ - entry_count: 4 bytes (uint32)  │
// │ - key_size: 2 bytes (uint16)     │
// ├──────────────────────────────────┤
// │ Entry 0                          │
// │ - key: key_size bytes            │
// │ - child_hash: 32 bytes           │
// │ - data_offset: 4 bytes (mode 3)  │ ← Optional data pointer
// ├──────────────────────────────────┤
// │ Entry 1                          │
// │ - key: key_size bytes            │
// │ - child_hash: 32 bytes           │
// │ - data_offset: 4 bytes (mode 3)  │
// ├──────────────────────────────────┤
// │ Data Section (mode 3 only)       │
// │ @ data_offset[0]:                │
// │   - data bytes (variable length) │
// │ @ data_offset[1]:                │
// │   - data bytes                   │
// └──────────────────────────────────┘
//
// VALUE KEY FORMAT (Modes 0 & 2):
// ┌──────────────────────────────────┐
// │ Header (8 bytes)                 │
// │ - version: 1 byte                │
// │ - mode: 1 byte (bitmask)         │
// │ - entry_count: 4 bytes (uint32)  │
// │ - reserved: 2 bytes              │
// ├──────────────────────────────────┤
// │ Offset Table                     │
// │ - offset[0]: 4 bytes (uint32)    │
// │ - offset[1]: 4 bytes             │
// │ - ... (entry_count offsets)      │
// ├──────────────────────────────────┤
// │ Data Section                     │
// │ Entry 0 @ offset[0]:             │
// │ - key_length: 4 bytes (uint32)   │
// │ - key_data: N bytes              │
// │ - child_hash: 32 bytes           │
// │ - data_length: 4 bytes (mode 2)  │ ← Optional data
// │ - data: M bytes (mode 2)         │
// │ Entry 1 @ offset[1]:             │
// │ - key_length: 4 bytes            │
// │ - key_data: N bytes              │
// │ - child_hash: 32 bytes           │
// │ - data_length: 4 bytes (mode 2)  │
// │ - data: M bytes (mode 2)         │
// └──────────────────────────────────┘

const (
	indexNodeVersion   = 1
	headerSize         = 8
	offsetSize         = 4
	childHashSize      = 32
	keyLengthSize      = 4
	dataLengthSize     = 4
	maxKeyLength       = 1024 * 1024 // 1MB max key size
	maxEntriesPerNode  = 100000      // Reasonable limit
)

// Mode bitmask values
const (
	indexModeValueKey    = 0x00 // 0b00: Variable-length keys, no data
	indexModePointerKey  = 0x01 // 0b01: Fixed 32-byte hash keys, no data
	indexModeValueData   = 0x02 // 0b10: Variable-length keys, with data
	indexModePointerData = 0x03 // 0b11: Fixed 32-byte hash keys, with data
)

// Mode bit flags
const (
	indexKeyTypePointer = 0x01 // Bit 0: 1 = pointer (fixed 32 bytes), 0 = value (variable)
	indexHasData        = 0x02 // Bit 1: 1 = has data section, 0 = no data
)

// NewIndexNode creates a new variable-width index node (no data)
func NewIndexNode() *IndexNode {
	return &IndexNode{
		Entries: make([]*IndexNodeEntry, 0),
		Mode:    indexModeValueKey,
	}
}

// NewFixedKeyIndexNode creates a new fixed-width index node (pointer keys, no data)
func NewFixedKeyIndexNode(keySize uint16) *IndexNode {
	return &IndexNode{
		Entries:      make([]*IndexNodeEntry, 0),
		Mode:         indexModePointerKey,
		FixedKeySize: keySize,
	}
}

// NewIndexNodeWithData creates a variable-width index node with data support
func NewIndexNodeWithData() *IndexNode {
	return &IndexNode{
		Entries: make([]*IndexNodeEntry, 0),
		Mode:    indexModeValueData,
	}
}

// NewFixedKeyIndexNodeWithData creates a fixed-width index node with data support
func NewFixedKeyIndexNodeWithData(keySize uint16) *IndexNode {
	return &IndexNode{
		Entries:      make([]*IndexNodeEntry, 0),
		Mode:         indexModePointerData,
		FixedKeySize: keySize,
	}
}

// AddEntry adds an entry to the node (entries should be added in sorted order)
func (n *IndexNode) AddEntry(key []byte, childHash []byte, data []byte) error {
	if len(childHash) != childHashSize {
		return fmt.Errorf("child hash must be 32 bytes, got %d", len(childHash))
	}
	if len(key) == 0 {
		return fmt.Errorf("key cannot be empty")
	}

	// Validate key size based on mode
	isPointerKey := (n.Mode & indexKeyTypePointer) != 0
	if isPointerKey {
		if uint16(len(key)) != n.FixedKeySize {
			return fmt.Errorf("key size must be %d bytes in pointer mode, got %d", n.FixedKeySize, len(key))
		}
	} else {
		if len(key) > maxKeyLength {
			return fmt.Errorf("key too large: %d bytes (max %d)", len(key), maxKeyLength)
		}
	}

	// Validate data if mode doesn't support it
	hasData := (n.Mode & indexHasData) != 0
	if !hasData && len(data) > 0 {
		return fmt.Errorf("mode %d does not support data", n.Mode)
	}

	n.Entries = append(n.Entries, &IndexNodeEntry{
		Key:       key,
		ChildHash: childHash,
		Data:      data,
	})

	return nil
}

// Sort sorts entries by key (lexicographic)
func (n *IndexNode) Sort() {
	sort.Slice(n.Entries, func(i, j int) bool {
		return bytes.Compare(n.Entries[i].Key, n.Entries[j].Key) < 0
	})
}

// Marshal serializes the index node to binary format
func (n *IndexNode) Marshal() ([]byte, error) {
	if len(n.Entries) == 0 {
		return nil, fmt.Errorf("cannot marshal empty index node")
	}
	if len(n.Entries) > maxEntriesPerNode {
		return nil, fmt.Errorf("too many entries: %d (max %d)", len(n.Entries), maxEntriesPerNode)
	}

	isPointerKey := (n.Mode & indexKeyTypePointer) != 0
	if isPointerKey {
		return n.marshalPointerKey()
	}
	return n.marshalValueKey()
}

// marshalPointerKey serializes a pointer-key (fixed-width) node
func (n *IndexNode) marshalPointerKey() ([]byte, error) {
	hasData := (n.Mode & indexHasData) != 0

	// Calculate sizes
	entrySize := int(n.FixedKeySize) + childHashSize
	if hasData {
		entrySize += offsetSize // Add 4 bytes for data offset
	}

	// Calculate data section size
	dataSize := 0
	if hasData {
		for _, entry := range n.Entries {
			dataSize += len(entry.Data)
		}
	}

	totalSize := headerSize + (len(n.Entries) * entrySize) + dataSize
	buf := make([]byte, totalSize)

	// Write header
	buf[0] = indexNodeVersion
	buf[1] = n.Mode
	binary.BigEndian.PutUint32(buf[2:6], uint32(len(n.Entries)))
	binary.BigEndian.PutUint16(buf[6:8], n.FixedKeySize)

	// Write entries
	offset := headerSize
	dataOffset := headerSize + (len(n.Entries) * entrySize)

	for _, entry := range n.Entries {
		// Write key
		copy(buf[offset:offset+int(n.FixedKeySize)], entry.Key)
		offset += int(n.FixedKeySize)

		// Write child hash
		copy(buf[offset:offset+childHashSize], entry.ChildHash)
		offset += childHashSize

		// Write data offset (if mode has data)
		if hasData {
			if len(entry.Data) > 0 {
				binary.BigEndian.PutUint32(buf[offset:offset+offsetSize], uint32(dataOffset))
				// Copy data to data section
				copy(buf[dataOffset:dataOffset+len(entry.Data)], entry.Data)
				dataOffset += len(entry.Data)
			} else {
				binary.BigEndian.PutUint32(buf[offset:offset+offsetSize], 0) // 0 = no data
			}
			offset += offsetSize
		}
	}

	return buf, nil
}

// marshalValueKey serializes a value-key (variable-width) node
func (n *IndexNode) marshalValueKey() ([]byte, error) {
	hasData := (n.Mode & indexHasData) != 0

	// Calculate total size
	offsetTableSize := len(n.Entries) * offsetSize
	dataSize := 0
	for _, entry := range n.Entries {
		dataSize += keyLengthSize + len(entry.Key) + childHashSize
		if hasData {
			dataSize += dataLengthSize + len(entry.Data)
		}
	}

	totalSize := headerSize + offsetTableSize + dataSize
	buf := make([]byte, totalSize)

	// Write header
	buf[0] = indexNodeVersion
	buf[1] = n.Mode
	binary.BigEndian.PutUint32(buf[2:6], uint32(len(n.Entries)))
	// Reserved bytes 6-7 are zero

	// Calculate and write offset table
	currentOffset := uint32(headerSize + offsetTableSize)
	offsetTableStart := headerSize

	for i, entry := range n.Entries {
		offsetPos := offsetTableStart + (i * offsetSize)
		binary.BigEndian.PutUint32(buf[offsetPos:offsetPos+offsetSize], currentOffset)

		// Calculate next offset
		entrySize := keyLengthSize + len(entry.Key) + childHashSize
		if hasData {
			entrySize += dataLengthSize + len(entry.Data)
		}
		currentOffset += uint32(entrySize)
	}

	// Write data section
	dataOffset := headerSize + offsetTableSize
	for _, entry := range n.Entries {
		// Write key length
		binary.BigEndian.PutUint32(buf[dataOffset:dataOffset+keyLengthSize], uint32(len(entry.Key)))
		dataOffset += keyLengthSize

		// Write key data
		copy(buf[dataOffset:dataOffset+len(entry.Key)], entry.Key)
		dataOffset += len(entry.Key)

		// Write child hash
		copy(buf[dataOffset:dataOffset+childHashSize], entry.ChildHash)
		dataOffset += childHashSize

		// Write data (if mode has data)
		if hasData {
			binary.BigEndian.PutUint32(buf[dataOffset:dataOffset+dataLengthSize], uint32(len(entry.Data)))
			dataOffset += dataLengthSize
			copy(buf[dataOffset:dataOffset+len(entry.Data)], entry.Data)
			dataOffset += len(entry.Data)
		}
	}

	return buf, nil
}

// Unmarshal deserializes an index node from binary format
func UnmarshalIndexNode(data []byte) (*IndexNode, error) {
	if len(data) < headerSize {
		return nil, fmt.Errorf("data too short for header: %d bytes", len(data))
	}

	// Read header
	version := data[0]
	if version != indexNodeVersion {
		return nil, fmt.Errorf("unsupported version: %d", version)
	}

	mode := data[1]
	entryCount := binary.BigEndian.Uint32(data[2:6])

	if entryCount == 0 {
		return nil, fmt.Errorf("entry count is zero")
	}
	if entryCount > maxEntriesPerNode {
		return nil, fmt.Errorf("too many entries: %d", entryCount)
	}

	isPointerKey := (mode & indexKeyTypePointer) != 0
	if isPointerKey {
		return unmarshalPointerKey(data, mode, entryCount)
	}
	return unmarshalValueKey(data, mode, entryCount)
}

// unmarshalPointerKey deserializes a pointer-key (fixed-width) node
func unmarshalPointerKey(data []byte, mode byte, entryCount uint32) (*IndexNode, error) {
	hasData := (mode & indexHasData) != 0
	keySize := binary.BigEndian.Uint16(data[6:8])

	entrySize := int(keySize) + childHashSize
	if hasData {
		entrySize += offsetSize // Add 4 bytes for data offset
	}

	entriesSize := int(entryCount) * entrySize
	minExpectedSize := headerSize + entriesSize

	if len(data) < minExpectedSize {
		return nil, fmt.Errorf("data too short: got %d, need at least %d", len(data), minExpectedSize)
	}

	node := &IndexNode{
		Entries:      make([]*IndexNodeEntry, entryCount),
		Mode:         mode,
		FixedKeySize: keySize,
	}

	offset := headerSize
	for i := uint32(0); i < entryCount; i++ {
		key := make([]byte, keySize)
		copy(key, data[offset:offset+int(keySize)])
		offset += int(keySize)

		childHash := make([]byte, childHashSize)
		copy(childHash, data[offset:offset+childHashSize])
		offset += childHashSize

		var entryData []byte
		if hasData {
			dataOffset := binary.BigEndian.Uint32(data[offset : offset+offsetSize])
			offset += offsetSize

			if dataOffset > 0 {
				// Find the length by looking at next entry's offset or end of data
				dataEnd := len(data)
				if i < entryCount-1 {
					// Look ahead to next entry's data offset
					nextEntryOffset := headerSize + int(i+1)*entrySize + int(keySize) + childHashSize
					nextDataOffset := binary.BigEndian.Uint32(data[nextEntryOffset : nextEntryOffset+offsetSize])
					if nextDataOffset > 0 {
						dataEnd = int(nextDataOffset)
					}
				}

				dataLen := dataEnd - int(dataOffset)
				if dataLen > 0 && int(dataOffset)+dataLen <= len(data) {
					entryData = make([]byte, dataLen)
					copy(entryData, data[dataOffset:dataOffset+uint32(dataLen)])
				}
			}
		}

		node.Entries[i] = &IndexNodeEntry{
			Key:       key,
			ChildHash: childHash,
			Data:      entryData,
		}
	}

	return node, nil
}

// unmarshalValueKey deserializes a value-key (variable-width) node
func unmarshalValueKey(data []byte, mode byte, entryCount uint32) (*IndexNode, error) {
	hasData := (mode & indexHasData) != 0
	// Read offset table
	offsetTableStart := headerSize
	offsetTableEnd := offsetTableStart + int(entryCount)*offsetSize
	if len(data) < offsetTableEnd {
		return nil, fmt.Errorf("data too short for offset table")
	}

	node := &IndexNode{
		Entries: make([]*IndexNodeEntry, entryCount),
		Mode:    mode,
	}

	// Read each entry
	for i := uint32(0); i < entryCount; i++ {
		offsetPos := offsetTableStart + int(i)*offsetSize
		entryOffset := binary.BigEndian.Uint32(data[offsetPos : offsetPos+offsetSize])

		if int(entryOffset) >= len(data) {
			return nil, fmt.Errorf("invalid offset for entry %d: %d", i, entryOffset)
		}

		// Read key length
		if int(entryOffset)+keyLengthSize > len(data) {
			return nil, fmt.Errorf("data too short for key length at entry %d", i)
		}
		keyLen := binary.BigEndian.Uint32(data[entryOffset : entryOffset+keyLengthSize])
		if keyLen > maxKeyLength {
			return nil, fmt.Errorf("key too large at entry %d: %d bytes", i, keyLen)
		}

		keyStart := entryOffset + keyLengthSize
		keyEnd := keyStart + keyLen

		if int(keyEnd) > len(data) {
			return nil, fmt.Errorf("data too short for key at entry %d", i)
		}

		// Read child hash
		hashStart := keyEnd
		hashEnd := hashStart + childHashSize

		if int(hashEnd) > len(data) {
			return nil, fmt.Errorf("data too short for child hash at entry %d", i)
		}

		key := make([]byte, keyLen)
		copy(key, data[keyStart:keyEnd])

		childHash := make([]byte, childHashSize)
		copy(childHash, data[hashStart:hashEnd])

		// Read data (if mode has data)
		var entryData []byte
		if hasData {
			dataLenStart := hashEnd
			dataLenEnd := dataLenStart + dataLengthSize

			if int(dataLenEnd) > len(data) {
				return nil, fmt.Errorf("data too short for data length at entry %d", i)
			}

			dataLen := binary.BigEndian.Uint32(data[dataLenStart:dataLenEnd])
			if dataLen > 0 {
				dataStart := dataLenEnd
				dataEnd := dataStart + dataLen

				if int(dataEnd) > len(data) {
					return nil, fmt.Errorf("data too short for data at entry %d", i)
				}

				entryData = make([]byte, dataLen)
				copy(entryData, data[dataStart:dataEnd])
			}
		}

		node.Entries[i] = &IndexNodeEntry{
			Key:       key,
			ChildHash: childHash,
			Data:      entryData,
		}
	}

	return node, nil
}

// Hash computes the BLAKE3 hash of the serialized node
func (n *IndexNode) Hash() ([]byte, error) {
	data, err := n.Marshal()
	if err != nil {
		return nil, err
	}

	hash := blake3.Sum256(data)
	return hash[:], nil
}

// Find performs binary search to find an entry by key
func (n *IndexNode) Find(key []byte) ([]byte, bool) {
	idx := sort.Search(len(n.Entries), func(i int) bool {
		return bytes.Compare(n.Entries[i].Key, key) >= 0
	})

	if idx < len(n.Entries) && bytes.Equal(n.Entries[idx].Key, key) {
		return n.Entries[idx].ChildHash, true
	}

	return nil, false
}

// HashKey computes the BLAKE3 hash of a key
func HashKey(key []byte) []byte {
	hash := blake3.Sum256(key)
	return hash[:]
}

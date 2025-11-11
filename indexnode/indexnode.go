package indexnode

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/shruggr/inspiration/multihash"
)

// IndexNode represents a unified index block supporting multiple access patterns
//
// UNIFIED FORMAT:
// ┌──────────────────────────────────┐
// │ Header (8 bytes)                 │
// │ - version: 1 byte                │
// │ - flags: 1 byte                  │
// │   - bit 0: has_data_section      │
// │   - bit 1: sort_by_data          │
// │   - bit 2: is_range              │
// │   - bits 3-7: reserved           │
// │ - entry_count: 2 bytes (uint16)  │
// │ - key_size: 2 bytes (uint16)     │
// │ - value_size: 1 byte (uint8)     │
// │ - reserved: 1 byte               │
// ├──────────────────────────────────┤
// │ Entry 0                          │
// │ - key: key_size bytes (optional) │
// │ - value: value_size bytes        │
// │ - offset: 4 bytes (optional)     │
// ├──────────────────────────────────┤
// │ Entry 1                          │
// │ - key: key_size bytes            │
// │ - value: value_size bytes        │
// │ - offset: 4 bytes (optional)     │
// ├──────────────────────────────────┤
// │ Data Section (if has_data_section)│
// │ @ offset[0]:                     │
// │   - variable length data         │
// │ @ offset[1]:                     │
// │   - variable length data         │
// └──────────────────────────────────┘
//
// Access Patterns:
// 1. key_size > 0, !has_data_section: Binary search by key → value
// 2. key_size > 0, has_data_section, !sort_by_data: Binary search by key → value + data
// 3. key_size > 0, has_data_section, sort_by_data: Binary search by data → value
// 4. key_size = 0, has_data_section: Binary search by data → value
// 5. key_size = 0, !has_data_section: Array access by index → value
//
// Range Mode (is_range = 1):
// - Entries represent range pointers
// - key = range_start boundary
// - value = child IndexNode hash
// - Sorted by key (or data if sort_by_data)
type IndexNode struct {
	Version     uint8
	HasData     bool
	SortByData  bool
	IsRange     bool
	KeySize     uint16
	ValueSize   uint8
	Entries     []*Entry
	DataSection []byte // Raw data section (managed externally)
}

// Entry represents a single entry in the index
type Entry struct {
	Key    []byte // length = KeySize (empty if KeySize=0)
	Value  []byte // length = ValueSize (hash/pointer/rowid)
	Offset uint32 // Offset into DataSection (0 if !HasData)
}

const (
	version    = 1
	headerSize = 8
	offsetSize = 4

	// Flag bits
	flagHasData    = 0x01 // bit 0
	flagSortByData = 0x02 // bit 1
	flagIsRange    = 0x04 // bit 2

	// Limits
	maxKeySize    = 65535 // uint16 max
	maxValueSize  = 255   // uint8 max
	maxEntryCount = 65535 // uint16 max
)

// Config for building index nodes with range splitting
type Config struct {
	MaxNodeSize     int // Default 1MB
	TargetChildSize int // Default 512KB for range splitting
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		MaxNodeSize:     1024 * 1024,       // 1MB
		TargetChildSize: 512 * 1024,        // 512KB
	}
}

// NewIndexNode creates a new index node
func NewIndexNode(keySize uint16, valueSize uint8, hasData, sortByData, isRange bool) *IndexNode {
	return &IndexNode{
		Version:    version,
		HasData:    hasData,
		SortByData: sortByData,
		IsRange:    isRange,
		KeySize:    keySize,
		ValueSize:  valueSize,
		Entries:    make([]*Entry, 0),
	}
}

// AddEntry adds an entry to the node
func (n *IndexNode) AddEntry(key, value []byte, dataOffset uint32) error {
	// Validate key size
	if n.KeySize > 0 && len(key) != int(n.KeySize) {
		return fmt.Errorf("key size mismatch: expected %d, got %d", n.KeySize, len(key))
	}
	if n.KeySize == 0 && len(key) != 0 {
		return fmt.Errorf("key should be empty when KeySize=0")
	}

	// Validate value size
	if len(value) != int(n.ValueSize) {
		return fmt.Errorf("value size mismatch: expected %d, got %d", n.ValueSize, len(value))
	}

	// Validate offset
	if !n.HasData && dataOffset != 0 {
		return fmt.Errorf("offset should be 0 when HasData=false")
	}

	n.Entries = append(n.Entries, &Entry{
		Key:    key,
		Value:  value,
		Offset: dataOffset,
	})

	return nil
}

// SetDataSection sets the raw data section bytes
func (n *IndexNode) SetDataSection(data []byte) {
	n.DataSection = data
}

// Sort sorts entries by key or data section (based on SortByData flag)
func (n *IndexNode) Sort() error {
	if n.SortByData {
		// Sort by data section values
		if !n.HasData {
			return fmt.Errorf("cannot sort by data when HasData=false")
		}
		sort.Slice(n.Entries, func(i, j int) bool {
			dataI := n.getDataAt(n.Entries[i].Offset)
			dataJ := n.getDataAt(n.Entries[j].Offset)
			return bytes.Compare(dataI, dataJ) < 0
		})
	} else {
		// Sort by key
		if n.KeySize == 0 {
			return fmt.Errorf("cannot sort by key when KeySize=0")
		}
		sort.Slice(n.Entries, func(i, j int) bool {
			return bytes.Compare(n.Entries[i].Key, n.Entries[j].Key) < 0
		})
	}
	return nil
}

// getDataAt reads variable-length data from data section at offset
// Uses a simple length-prefix format: [length: 4 bytes][data: N bytes]
func (n *IndexNode) getDataAt(offset uint32) []byte {
	if offset == 0 || int(offset) >= len(n.DataSection) {
		return nil
	}
	if int(offset)+4 > len(n.DataSection) {
		return nil
	}
	length := binary.BigEndian.Uint32(n.DataSection[offset : offset+4])
	dataStart := offset + 4
	dataEnd := dataStart + length
	if int(dataEnd) > len(n.DataSection) {
		return nil
	}
	return n.DataSection[dataStart:dataEnd]
}

// Marshal serializes the index node to binary format
func (n *IndexNode) Marshal() ([]byte, error) {
	if len(n.Entries) == 0 {
		return nil, fmt.Errorf("cannot marshal empty index node")
	}
	if len(n.Entries) > maxEntryCount {
		return nil, fmt.Errorf("too many entries: %d (max %d)", len(n.Entries), maxEntryCount)
	}

	// Calculate entry size
	entrySize := int(n.KeySize) + int(n.ValueSize)
	if n.HasData {
		entrySize += offsetSize
	}

	// Calculate total size
	totalSize := headerSize + (len(n.Entries) * entrySize)
	if n.HasData {
		totalSize += len(n.DataSection)
	}

	buf := make([]byte, totalSize)

	// Write header
	buf[0] = n.Version

	// Build flags byte
	var flags uint8
	if n.HasData {
		flags |= flagHasData
	}
	if n.SortByData {
		flags |= flagSortByData
	}
	if n.IsRange {
		flags |= flagIsRange
	}
	buf[1] = flags

	binary.BigEndian.PutUint16(buf[2:4], uint16(len(n.Entries)))
	binary.BigEndian.PutUint16(buf[4:6], n.KeySize)
	buf[6] = n.ValueSize
	buf[7] = 0 // reserved

	// Write entries
	offset := headerSize
	for _, entry := range n.Entries {
		// Write key (if KeySize > 0)
		if n.KeySize > 0 {
			copy(buf[offset:offset+int(n.KeySize)], entry.Key)
			offset += int(n.KeySize)
		}

		// Write value
		copy(buf[offset:offset+int(n.ValueSize)], entry.Value)
		offset += int(n.ValueSize)

		// Write offset (if HasData)
		if n.HasData {
			binary.BigEndian.PutUint32(buf[offset:offset+offsetSize], entry.Offset)
			offset += offsetSize
		}
	}

	// Write data section (if HasData)
	if n.HasData && len(n.DataSection) > 0 {
		copy(buf[offset:], n.DataSection)
	}

	return buf, nil
}

// Unmarshal deserializes an index node from binary format
func Unmarshal(data []byte) (*IndexNode, error) {
	if len(data) < headerSize {
		return nil, fmt.Errorf("data too short for header: %d bytes", len(data))
	}

	// Read header
	ver := data[0]
	if ver != version {
		return nil, fmt.Errorf("unsupported version: %d", ver)
	}

	flags := data[1]
	hasData := (flags & flagHasData) != 0
	sortByData := (flags & flagSortByData) != 0
	isRange := (flags & flagIsRange) != 0

	entryCount := binary.BigEndian.Uint16(data[2:4])
	keySize := binary.BigEndian.Uint16(data[4:6])
	valueSize := data[6]

	if entryCount == 0 {
		return nil, fmt.Errorf("entry count is zero")
	}

	// Calculate entry size
	entrySize := int(keySize) + int(valueSize)
	if hasData {
		entrySize += offsetSize
	}

	// Validate data size
	minSize := headerSize + (int(entryCount) * entrySize)
	if len(data) < minSize {
		return nil, fmt.Errorf("data too short: got %d, need at least %d", len(data), minSize)
	}

	node := &IndexNode{
		Version:    ver,
		HasData:    hasData,
		SortByData: sortByData,
		IsRange:    isRange,
		KeySize:    keySize,
		ValueSize:  valueSize,
		Entries:    make([]*Entry, entryCount),
	}

	// Read entries
	offset := headerSize
	for i := uint16(0); i < entryCount; i++ {
		entry := &Entry{}

		// Read key (if KeySize > 0)
		if keySize > 0 {
			entry.Key = make([]byte, keySize)
			copy(entry.Key, data[offset:offset+int(keySize)])
			offset += int(keySize)
		}

		// Read value
		entry.Value = make([]byte, valueSize)
		copy(entry.Value, data[offset:offset+int(valueSize)])
		offset += int(valueSize)

		// Read offset (if HasData)
		if hasData {
			entry.Offset = binary.BigEndian.Uint32(data[offset : offset+offsetSize])
			offset += offsetSize
		}

		node.Entries[i] = entry
	}

	// Read data section (if HasData)
	if hasData && len(data) > offset {
		node.DataSection = make([]byte, len(data)-offset)
		copy(node.DataSection, data[offset:])
	}

	return node, nil
}

// Hash computes the BLAKE3 multihash of the serialized node
func (n *IndexNode) Hash() (multihash.IndexHash, error) {
	data, err := n.Marshal()
	if err != nil {
		return nil, err
	}
	return multihash.NewIndexHash(data)
}

// Find performs binary search to find an entry by key
// Returns the value and whether it was found
func (n *IndexNode) Find(searchKey []byte) ([]byte, bool) {
	if n.KeySize == 0 {
		return nil, false
	}
	if n.SortByData {
		return nil, false
	}

	idx := sort.Search(len(n.Entries), func(i int) bool {
		return bytes.Compare(n.Entries[i].Key, searchKey) >= 0
	})

	if idx < len(n.Entries) && bytes.Equal(n.Entries[idx].Key, searchKey) {
		return n.Entries[idx].Value, true
	}

	return nil, false
}

// FindByData performs binary search by data section value
// Returns the value (hash/pointer) and whether it was found
func (n *IndexNode) FindByData(searchData []byte) ([]byte, bool) {
	if !n.HasData {
		return nil, false
	}
	if !n.SortByData {
		return nil, false
	}

	idx := sort.Search(len(n.Entries), func(i int) bool {
		data := n.getDataAt(n.Entries[i].Offset)
		return bytes.Compare(data, searchData) >= 0
	})

	if idx < len(n.Entries) {
		data := n.getDataAt(n.Entries[idx].Offset)
		if bytes.Equal(data, searchData) {
			return n.Entries[idx].Value, true
		}
	}

	return nil, false
}

// FindRange finds which range contains the given key (for range nodes)
// Returns the child hash to follow
func (n *IndexNode) FindRange(searchKey []byte) ([]byte, bool) {
	if !n.IsRange {
		return nil, false
	}

	// Binary search to find the range
	// Ranges are: entry[i].Key <= searchKey < entry[i+1].Key
	idx := sort.Search(len(n.Entries), func(i int) bool {
		var cmpKey []byte
		if n.SortByData {
			cmpKey = n.getDataAt(n.Entries[i].Offset)
		} else {
			cmpKey = n.Entries[i].Key
		}
		return bytes.Compare(cmpKey, searchKey) > 0
	})

	// Move back one to get the range that contains searchKey
	if idx > 0 {
		idx--
	}

	if idx < len(n.Entries) {
		return n.Entries[idx].Value, true
	}

	return nil, false
}

// GetByIndex returns the value at the given index (for array-style access)
func (n *IndexNode) GetByIndex(index int) ([]byte, bool) {
	if index < 0 || index >= len(n.Entries) {
		return nil, false
	}
	return n.Entries[index].Value, true
}

// Size returns the serialized size of the node
func (n *IndexNode) Size() int {
	entrySize := int(n.KeySize) + int(n.ValueSize)
	if n.HasData {
		entrySize += offsetSize
	}
	totalSize := headerSize + (len(n.Entries) * entrySize)
	if n.HasData {
		totalSize += len(n.DataSection)
	}
	return totalSize
}

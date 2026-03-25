package indexnode

import (
	"encoding/binary"
	"fmt"
)

type LeafEntry struct {
	TxID            []byte   // 32 bytes
	SubtreePosition uint64
	Vouts           []uint32
}

func (e *LeafEntry) Marshal() []byte {
	buf := make([]byte, 32+binary.MaxVarintLen64*(2+len(e.Vouts)))
	offset := copy(buf, e.TxID[:32])
	offset += binary.PutUvarint(buf[offset:], e.SubtreePosition)
	offset += binary.PutUvarint(buf[offset:], uint64(len(e.Vouts)))
	for _, v := range e.Vouts {
		offset += binary.PutUvarint(buf[offset:], uint64(v))
	}
	return buf[:offset]
}

func UnmarshalLeafEntry(data []byte) (LeafEntry, int, error) {
	if len(data) < 33 {
		return LeafEntry{}, 0, fmt.Errorf("data too short: %d bytes", len(data))
	}

	var entry LeafEntry
	entry.TxID = make([]byte, 32)
	copy(entry.TxID, data[:32])
	offset := 32

	pos, n := binary.Uvarint(data[offset:])
	if n <= 0 {
		return LeafEntry{}, 0, fmt.Errorf("invalid subtree_position varint")
	}
	entry.SubtreePosition = pos
	offset += n

	count, n := binary.Uvarint(data[offset:])
	if n <= 0 {
		return LeafEntry{}, 0, fmt.Errorf("invalid vout_count varint")
	}
	offset += n

	entry.Vouts = make([]uint32, count)
	for i := uint64(0); i < count; i++ {
		v, n := binary.Uvarint(data[offset:])
		if n <= 0 {
			return LeafEntry{}, 0, fmt.Errorf("invalid vout varint at index %d", i)
		}
		entry.Vouts[i] = uint32(v)
		offset += n
	}

	return entry, offset, nil
}

func MarshalLeafEntryList(entries []LeafEntry) []byte {
	countBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(countBuf, uint32(len(entries)))
	buf := countBuf
	for i := range entries {
		buf = append(buf, entries[i].Marshal()...)
	}
	return buf
}

func UnmarshalLeafEntryList(data []byte) ([]LeafEntry, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("data too short for count: %d bytes", len(data))
	}
	count := binary.BigEndian.Uint32(data[:4])
	offset := 4
	entries := make([]LeafEntry, count)
	for i := uint32(0); i < count; i++ {
		entry, n, err := UnmarshalLeafEntry(data[offset:])
		if err != nil {
			return nil, fmt.Errorf("entry %d: %w", i, err)
		}
		entries[i] = entry
		offset += n
	}
	return entries, nil
}

package treebuilder

import (
	"context"
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/shruggr/inspiration/indexnode"
	"github.com/shruggr/inspiration/kvstore"
	"github.com/shruggr/inspiration/multihash"
)

type implementation struct {
	store kvstore.KVStore
}

func NewBuilder(store kvstore.KVStore) Builder {
	return &implementation{store: store}
}

func (b *implementation) BuildSubtreeIndex(ctx context.Context, txs []TaggedTransaction) (multihash.IndexHash, error) {
	if len(txs) == 0 {
		return nil, fmt.Errorf("no transactions to index")
	}

	// Group by tag key -> tag value -> []LeafEntry
	type leafGroup struct {
		entries []indexnode.LeafEntry
	}
	keyMap := make(map[string]map[string]*leafGroup)

	for _, tx := range txs {
		for _, tag := range tx.Tags {
			valueMap, ok := keyMap[tag.Key]
			if !ok {
				valueMap = make(map[string]*leafGroup)
				keyMap[tag.Key] = valueMap
			}
			group, ok := valueMap[tag.Value]
			if !ok {
				group = &leafGroup{}
				valueMap[tag.Value] = group
			}
			group.entries = append(group.entries, indexnode.LeafEntry{
				TxID:            tx.TxID[:],
				SubtreePosition: tx.SubtreePosition,
				Vouts:           tag.Vouts,
			})
		}
	}

	// Level 2: for each tag key, build a tag-value node
	keyToHash := make(map[string]multihash.IndexHash, len(keyMap))

	for key, valueMap := range keyMap {
		valueNode := indexnode.NewTagNode()
		var dataSection []byte
		dataSection = append(dataSection, 0) // padding byte

		values := make([]string, 0, len(valueMap))
		for v := range valueMap {
			values = append(values, v)
		}
		sort.Strings(values)

		for _, val := range values {
			group := valueMap[val]

			// Sort leaf entries by SubtreePosition
			sort.Slice(group.entries, func(i, j int) bool {
				return group.entries[i].SubtreePosition < group.entries[j].SubtreePosition
			})

			// Level 3: marshal leaf entry list, hash, store
			leafBytes := indexnode.MarshalLeafEntryList(group.entries)
			leafHash, err := multihash.NewIndexHash(leafBytes)
			if err != nil {
				return nil, fmt.Errorf("hash leaf list: %w", err)
			}
			if err := b.store.Put(ctx, leafHash.Bytes(), leafBytes); err != nil {
				return nil, fmt.Errorf("store leaf list: %w", err)
			}

			// Add entry to value node
			offset := uint32(len(dataSection))
			dataSection = appendLengthPrefixed(dataSection, val)
			if err := valueNode.AddEntry(nil, leafHash.Bytes()[2:], offset); err != nil {
				return nil, fmt.Errorf("add value entry: %w", err)
			}
		}

		valueNode.SetDataSection(dataSection)
		if err := valueNode.Sort(); err != nil {
			return nil, fmt.Errorf("sort value node: %w", err)
		}

		valueBytes, err := valueNode.Marshal()
		if err != nil {
			return nil, fmt.Errorf("marshal value node: %w", err)
		}
		valueHash, err := multihash.NewIndexHash(valueBytes)
		if err != nil {
			return nil, fmt.Errorf("hash value node: %w", err)
		}
		if err := b.store.Put(ctx, valueHash.Bytes(), valueBytes); err != nil {
			return nil, fmt.Errorf("store value node: %w", err)
		}

		keyToHash[key] = valueHash
	}

	// Level 1: build root tag-key node
	rootNode := indexnode.NewTagNode()
	var rootData []byte
	rootData = append(rootData, 0) // padding byte

	keys := make([]string, 0, len(keyToHash))
	for k := range keyToHash {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		h := keyToHash[key]
		offset := uint32(len(rootData))
		rootData = appendLengthPrefixed(rootData, key)
		if err := rootNode.AddEntry(nil, h.Bytes()[2:], offset); err != nil {
			return nil, fmt.Errorf("add key entry: %w", err)
		}
	}

	rootNode.SetDataSection(rootData)
	if err := rootNode.Sort(); err != nil {
		return nil, fmt.Errorf("sort root node: %w", err)
	}

	rootBytes, err := rootNode.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshal root node: %w", err)
	}
	rootHash, err := multihash.NewIndexHash(rootBytes)
	if err != nil {
		return nil, fmt.Errorf("hash root node: %w", err)
	}
	if err := b.store.Put(ctx, rootHash.Bytes(), rootBytes); err != nil {
		return nil, fmt.Errorf("store root node: %w", err)
	}

	return rootHash, nil
}

func appendLengthPrefixed(buf []byte, s string) []byte {
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(s)))
	buf = append(buf, lenBuf...)
	buf = append(buf, s...)
	return buf
}

package processor

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/shruggr/inspiration/cache"
	"github.com/shruggr/inspiration/kvstore"
	"github.com/shruggr/inspiration/metadata"
	"github.com/shruggr/inspiration/store"
	"github.com/shruggr/inspiration/teranode"
	"github.com/shruggr/inspiration/treebuilder"
	"github.com/shruggr/inspiration/txindexer"
)

var ErrSubtreeNotReady = errors.New("one or more subtrees not yet processed")

type subtreeNode struct {
	Hash [32]byte
	Fee  uint64
	Size uint64
}

type Processor struct {
	store      *store.DualStore
	spendStore kvstore.KVStore
	cache      cache.IndexTermCache
	indexer    txindexer.Indexer
	client     *teranode.Client
	builder    treebuilder.Builder
	metadata   metadata.Store
	logger     *slog.Logger
}

func NewProcessor(
	store *store.DualStore,
	spendStore kvstore.KVStore,
	cache cache.IndexTermCache,
	indexer txindexer.Indexer,
	client *teranode.Client,
	builder treebuilder.Builder,
	metadata metadata.Store,
	logger *slog.Logger,
) *Processor {
	return &Processor{
		store:      store,
		spendStore: spendStore,
		cache:      cache,
		indexer:    indexer,
		client:     client,
		builder:    builder,
		metadata:   metadata,
		logger:     logger,
	}
}

func (p *Processor) ProcessSubtree(ctx context.Context, subtreeHash string, fetchURL string) error {
	subtreeData, err := p.client.FetchSubtreeData(ctx, fetchURL)
	if err != nil {
		return fmt.Errorf("fetch subtree %s: %w", subtreeHash, err)
	}

	return p.processSubtreeData(ctx, subtreeHash, subtreeData)
}

func (p *Processor) processSubtreeData(ctx context.Context, subtreeHash string, subtreeData []byte) error {
	nodes, err := parseSubtreeNodes(subtreeData)
	if err != nil {
		return fmt.Errorf("parse subtree %s: %w", subtreeHash, err)
	}

	var subtreeRoot [32]byte
	if len(subtreeData) >= 32 {
		copy(subtreeRoot[:], subtreeData[:32])
	}

	taggedTxs := make([]treebuilder.TaggedTransaction, 0, len(nodes))

	for i, node := range nodes {
		txid := cache.TxID(node.Hash)

		terms, ok := p.cache.Get(txid)
		if !ok {
			rawTx, err := p.client.FetchTransaction(ctx, teranode.TxIDToHex(txid[:]))
			if err != nil {
				return fmt.Errorf("fetch tx %x: %w", txid[:8], err)
			}

			results, err := p.indexer.Index(ctx, &txindexer.TransactionContext{
				TxID:  txid[:],
				RawTx: rawTx,
			})
			if err != nil {
				return fmt.Errorf("index tx %x: %w", txid[:8], err)
			}

			terms = make([]cache.IndexTerm, len(results))
			for j, r := range results {
				terms[j] = cache.IndexTerm{
					Key:   r.Key,
					Value: r.Value,
					Vouts: r.Vouts,
				}
			}

			if err := p.cache.Put(txid, terms); err != nil {
				p.logger.Warn("cache put failed", "txid", fmt.Sprintf("%x", txid[:8]), "err", err)
			}

			if err := p.writeSpendRecords(ctx, txid, rawTx); err != nil {
				return fmt.Errorf("write spends for %x: %w", txid[:8], err)
			}
		}

		tags := make([]treebuilder.Tag, len(terms))
		for j, t := range terms {
			tags[j] = treebuilder.Tag{
				Key:   t.Key,
				Value: t.Value,
				Vouts: t.Vouts,
			}
		}

		taggedTxs = append(taggedTxs, treebuilder.TaggedTransaction{
			TxID:            txid,
			SubtreePosition: uint64(i),
			Tags:            tags,
		})
	}

	indexRoot, err := p.builder.BuildSubtreeIndex(ctx, taggedTxs)
	if err != nil {
		return fmt.Errorf("build subtree index %s: %w", subtreeHash, err)
	}

	return p.metadata.InsertSubtree(ctx, subtreeRoot[:], indexRoot.Bytes(), uint32(len(nodes)))
}

func (p *Processor) writeSpendRecords(ctx context.Context, currentTxID cache.TxID, rawTx []byte) error {
	tx, err := transaction.NewTransactionFromBytes(rawTx)
	if err != nil {
		return fmt.Errorf("parse transaction: %w", err)
	}

	for _, input := range tx.Inputs {
		if input.SourceTXID == nil {
			continue
		}
		key := makeOutpointKey(input.SourceTXID[:], input.SourceTxOutIndex)
		if err := p.spendStore.Put(ctx, key, currentTxID[:]); err != nil {
			return fmt.Errorf("put spend record: %w", err)
		}
	}

	return nil
}

func (p *Processor) ProcessBlock(ctx context.Context, height uint32, header []byte, subtreeHashes [][]byte, txCount uint64) error {
	for _, hash := range subtreeHashes {
		exists, err := p.metadata.SubtreeExists(ctx, hash)
		if err != nil {
			return fmt.Errorf("check subtree: %w", err)
		}
		if !exists {
			return ErrSubtreeNotReady
		}
	}

	return p.metadata.InsertBlock(ctx, height, blockHashFromHeader(header), header, txCount, subtreeHashes)
}

func makeOutpointKey(txid []byte, vout uint32) []byte {
	key := make([]byte, 36)
	copy(key, txid)
	binary.BigEndian.PutUint32(key[32:], vout)
	return key
}

func blockHashFromHeader(header []byte) []byte {
	first := sha256.Sum256(header)
	second := sha256.Sum256(first[:])
	return second[:]
}

// parseSubtreeNodes parses the go-subtree binary format and returns the transaction nodes.
// Format: 32b root hash | 8b fees | 8b size | 8b numNodes | (32b hash + 8b fee + 8b size) per node | ...
func parseSubtreeNodes(data []byte) ([]subtreeNode, error) {
	const headerSize = 32 + 8 + 8 + 8
	if len(data) < headerSize {
		return nil, fmt.Errorf("subtree data too short: %d bytes", len(data))
	}

	numLeaves := binary.LittleEndian.Uint64(data[48:56])

	const nodeSize = 48
	expectedSize := headerSize + int(numLeaves)*nodeSize
	if len(data) < expectedSize {
		return nil, fmt.Errorf("subtree data truncated: have %d, need at least %d bytes", len(data), expectedSize)
	}

	nodes := make([]subtreeNode, numLeaves)
	offset := headerSize
	for i := uint64(0); i < numLeaves; i++ {
		var n subtreeNode
		copy(n.Hash[:], data[offset:offset+32])
		n.Fee = binary.LittleEndian.Uint64(data[offset+32 : offset+40])
		n.Size = binary.LittleEndian.Uint64(data[offset+40 : offset+48])
		nodes[i] = n
		offset += nodeSize
	}

	return nodes, nil
}

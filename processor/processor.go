package processor

import (
	"context"
	"log"

	"github.com/shruggr/inspiration/kvstore"
	"github.com/shruggr/inspiration/models"
	"github.com/shruggr/inspiration/txindexer"
)

// Processor handles transaction indexing and storage
type Processor struct {
	store       kvstore.KVStore
	indexer     txindexer.Indexer
	headerChain *models.HeaderChain
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewProcessor creates a new transaction processor
func NewProcessor(store kvstore.KVStore, idx txindexer.Indexer) *Processor {
	ctx, cancel := context.WithCancel(context.Background())

	return &Processor{
		store:       store,
		indexer:     idx,
		headerChain: models.NewHeaderChain(),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// ProcessTransaction indexes and stores a transaction
// TODO: Implement with new architecture (KVStore, IndexTermCache, tree building)
func (p *Processor) ProcessTransaction(ctx context.Context, txCtx *txindexer.TransactionContext) error {
	log.Printf("TODO: ProcessTransaction for tx %x", txCtx.TxID)
	return nil
}

// ProcessBlock handles a confirmed block with transactions
// TODO: Implement with new architecture (MetadataStore, tree building)
func (p *Processor) ProcessBlock(ctx context.Context, header *models.BlockHeader, subtrees []*Subtree) error {
	if err := p.headerChain.AddHeader(header); err != nil {
		return err
	}
	log.Printf("TODO: ProcessBlock at height %d with %d subtrees", header.Height, len(subtrees))
	return nil
}

// ProcessSubtree processes all transactions in a subtree
// TODO: Implement with new architecture
func (p *Processor) ProcessSubtree(ctx context.Context, height uint64, subtree *Subtree) error {
	log.Printf("TODO: ProcessSubtree %x at height %d", subtree.MerkleRoot, height)
	return nil
}

// HandleReorg processes a chain reorganization
// TODO: Implement with new architecture (MetadataStore.DeleteBlock)
func (p *Processor) HandleReorg(ctx context.Context, reorgHeight uint64) error {
	p.headerChain.Reorg(reorgHeight)
	log.Printf("TODO: HandleReorg at height %d", reorgHeight)
	return nil
}

// GetHeaderChain returns the header chain tracker
func (p *Processor) GetHeaderChain() *models.HeaderChain {
	return p.headerChain
}

// Stop shuts down the processor
func (p *Processor) Stop() error {
	p.cancel()
	return nil
}

// Subtree represents a merkle subtree containing transactions
type Subtree struct {
	MerkleRoot   []byte
	Transactions []*Transaction
}

// Transaction represents a transaction in a subtree
type Transaction struct {
	TxID  []byte
	RawTx []byte
}

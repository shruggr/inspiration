package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/shruggr/inspiration/kvstore"
	"github.com/shruggr/inspiration/kvstore/badger"
	"github.com/shruggr/inspiration/kvstore/memory"
	"github.com/shruggr/inspiration/p2p"
	"github.com/shruggr/inspiration/processor"
	"github.com/shruggr/inspiration/txindexer"
)

func main() {
	// Parse flags
	storageType := flag.String("storage", "badger", "Storage type: memory or badger")
	dataDir := flag.String("data-dir", "./data", "Data directory for BadgerDB")
	flag.Parse()

	log.Println("Starting BSV Indexer...")

	// Initialize storage based on type
	var store kvstore.KVStore
	var err error

	switch *storageType {
	case "memory":
		log.Println("Using in-memory storage")
		store = memory.New()
	case "badger":
		log.Printf("Using BadgerDB storage at %s", *dataDir)
		store, err = badger.New(&badger.Config{
			DataDir: *dataDir,
		})
		if err != nil {
			log.Fatalf("Failed to initialize BadgerDB: %v", err)
		}
	default:
		log.Fatalf("Unknown storage type: %s (use 'memory' or 'badger')", *storageType)
	}
	defer store.Close()

	// Initialize indexer (using noop for now)
	idx := txindexer.NewNoopIndexer()

	// Initialize processor
	proc := processor.NewProcessor(store, idx)
	defer proc.Stop()

	// Initialize P2P listener
	p2pConfig := &p2p.Config{
		ListenAddresses: []string{"/ip4/0.0.0.0/tcp/9905"},
		Port:            9905,
		BootstrapPeers:  []string{},
		BlockTopic:      "blocks",
		SubtreeTopic:    "subtrees",
	}

	listener, err := p2p.NewListener(p2pConfig)
	if err != nil {
		log.Fatalf("Failed to create P2P listener: %v", err)
	}

	if err := listener.Start(); err != nil {
		log.Fatalf("Failed to start P2P listener: %v", err)
	}
	defer listener.Stop()

	log.Println("Indexer is running...")
	log.Printf("Chain tip: height=%d", proc.GetHeaderChain().Height())
	log.Printf("Connected peers: %d", listener.PeerCount())

	// Subscribe to P2P topics
	blockCh := listener.SubscribeBlocks()
	subtreeCh := listener.SubscribeSubtrees()

	ctx := context.Background()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Main event loop
	for {
		select {
		case <-sigCh:
			log.Println("Shutting down...")
			return

		case blockData := <-blockCh:
			log.Printf("Received block: %d bytes", len(blockData))
			// TODO: Parse block and process
			// For now, just log receipt
			_ = blockData

		case subtreeData := <-subtreeCh:
			log.Printf("Received subtree: %d bytes", len(subtreeData))
			// TODO: Parse subtree and process
			// For now, just log receipt
			_ = subtreeData
			_ = ctx
		}
	}
}

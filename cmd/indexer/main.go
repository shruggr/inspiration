package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/shruggr/inspiration/kvstore"
	"github.com/shruggr/inspiration/kvstore/badger"
	"github.com/shruggr/inspiration/kvstore/memory"
	"github.com/shruggr/inspiration/p2p"
	"github.com/shruggr/inspiration/processor"
	"github.com/shruggr/inspiration/txindexer"
)

// splitAndTrim splits a string by delimiter and trims whitespace from each part
func splitAndTrim(s, delim string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, delim)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func main() {
	// Parse flags
	storageType := flag.String("storage", "badger", "Storage type: memory or badger")
	dataDir := flag.String("data-dir", "./data", "Data directory for BadgerDB")
	p2pPort := flag.Int("p2p-port", 9905, "P2P listen port")
	topicPrefix := flag.String("topic-prefix", "teratestnet", "Topic prefix (teratestnet, mainnet, etc.)")
	bootstrapPeers := flag.String("bootstrap-peers", "", "Comma-separated list of bootstrap peer multiaddrs")
	logLevel := flag.String("log-level", "info", "Log level: debug, info, warn, error")
	flag.Parse()

	// Set up slog with the specified level
	var level slog.Level
	switch *logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)

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

	// Parse bootstrap peers
	var bootstrapPeerList []string
	if *bootstrapPeers != "" {
		bootstrapPeerList = splitAndTrim(*bootstrapPeers, ",")
	}

	// Initialize P2P listener
	p2pConfig := &p2p.Config{
		Port:           *p2pPort,
		BootstrapPeers: bootstrapPeerList,
		TopicPrefix:    *topicPrefix,
	}

	listener, err := p2p.NewListener(p2pConfig, logger)
	if err != nil {
		log.Fatalf("Failed to create P2P listener: %v", err)
	}

	if err := listener.Start(); err != nil {
		log.Fatalf("Failed to start P2P listener: %v", err)
	}
	defer listener.Stop()

	log.Printf("Indexer started | Height: %d | Peers: %d", proc.GetHeaderChain().Height(), listener.PeerCount())

	// Subscribe to P2P topics
	blockCh := listener.SubscribeBlocks()
	subtreeCh := listener.SubscribeSubtrees()
	statusCh := listener.SubscribeStatus()

	ctx := context.Background()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Status ticker to show we're alive and peer count
	statusTicker := time.NewTicker(5 * time.Minute)
	defer statusTicker.Stop()

	// Main event loop
	for {
		select {
		case <-sigCh:
			log.Println("Shutting down...")
			return

		case <-statusTicker.C:
			log.Printf("Status: Connected to %d peers, waiting for messages...", listener.PeerCount())

		case blockData := <-blockCh:
			var blockMsg map[string]interface{}
			if err := json.Unmarshal(blockData, &blockMsg); err != nil {
				logger.Error("Failed to parse block message", "error", err)
				continue
			}
			logger.Info("BLOCK",
				"height", blockMsg["Height"],
				"hash", blockMsg["Hash"],
				"from", blockMsg["ClientName"],
				"peer_id", blockMsg["PeerID"])
			_ = ctx

		case subtreeData := <-subtreeCh:
			var subtreeMsg map[string]interface{}
			if err := json.Unmarshal(subtreeData, &subtreeMsg); err != nil {
				logger.Error("Failed to parse subtree message", "error", err)
				continue
			}
			logger.Info("SUBTREE",
				"hash", subtreeMsg["Hash"],
				"from", subtreeMsg["ClientName"],
				"peer_id", subtreeMsg["PeerID"],
				"url", subtreeMsg["DataHubURL"])

		case <-statusCh:
			// Ignore node status messages
		}
	}
}

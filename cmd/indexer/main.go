package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/shruggr/inspiration/cache/memory"
	"github.com/shruggr/inspiration/kafka"
	"github.com/shruggr/inspiration/kvstore/badger"
	metasqlite "github.com/shruggr/inspiration/metadata/sqlite"
	"github.com/shruggr/inspiration/processor"
	"github.com/shruggr/inspiration/store"
	"github.com/shruggr/inspiration/teranode"
	"github.com/shruggr/inspiration/treebuilder"
	"github.com/shruggr/inspiration/txindexer"
)

func main() {
	kafkaBrokers := flag.String("kafka-brokers", "localhost:9092", "Comma-separated Kafka broker addresses")
	kafkaGroupID := flag.String("kafka-group", "junglebus-indexer", "Kafka consumer group ID")
	teranodeURL := flag.String("teranode-url", "http://localhost:8080", "Teranode HTTP API base URL")
	dataDir := flag.String("data-dir", "./data", "Base data directory")
	cacheSize := flag.Int("cache-size", 100000, "LRU cache size for parsed transactions")
	logLevel := flag.String("log-level", "info", "Log level: debug, info, warn, error")
	flag.Parse()

	var level slog.Level
	switch *logLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	workingStore, err := badger.New(&badger.Config{DataDir: *dataDir + "/working"})
	if err != nil {
		log.Fatalf("working store: %v", err)
	}
	defer workingStore.Close()

	persistentStore, err := badger.New(&badger.Config{DataDir: *dataDir + "/persistent"})
	if err != nil {
		log.Fatalf("persistent store: %v", err)
	}
	defer persistentStore.Close()

	dualStore := store.NewDualStore(workingStore, persistentStore)

	spendStore, err := badger.New(&badger.Config{DataDir: *dataDir + "/spends"})
	if err != nil {
		log.Fatalf("spend store: %v", err)
	}
	defer spendStore.Close()

	metaStore, err := metasqlite.New(*dataDir + "/metadata.db")
	if err != nil {
		log.Fatalf("metadata store: %v", err)
	}
	defer metaStore.Close()

	txCache, err := memory.New(*cacheSize)
	if err != nil {
		log.Fatalf("cache: %v", err)
	}

	idx := txindexer.NewMultiIndexer(txindexer.NewP2PKHIndexer())

	builder := treebuilder.NewBuilder(dualStore)

	client := teranode.NewClient(*teranodeURL)

	proc := processor.NewProcessor(dualStore, spendStore, txCache, idx, client, builder, metaStore, logger)

	brokers := strings.Split(*kafkaBrokers, ",")
	consumer := kafka.NewConsumer(brokers, *kafkaGroupID, proc.ProcessSubtree, proc.ProcessBlock, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("shutting down...")
		cancel()
	}()

	logger.Info("starting indexer",
		"kafka", *kafkaBrokers,
		"teranode", *teranodeURL,
		"data-dir", *dataDir,
	)

	if err := consumer.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("consumer error: %v", err)
	}
}

package p2p

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	p2p "github.com/bsv-blockchain/go-p2p-message-bus"
	"github.com/libp2p/go-libp2p/core/crypto"
)

// Config holds P2P listener configuration
type Config struct {
	Port            int
	BootstrapPeers  []string
	PrivateKey      string // hex-encoded private key
	TopicPrefix     string // e.g., "teratestnet", "mainnet"
	PeerCacheFile   string
}

// Listener handles P2P network communication
type Listener struct {
	config    *Config
	client    p2p.Client
	logger    *slog.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	blockCh   chan []byte
	subtreeCh chan []byte
	statusCh  chan []byte
	mu        sync.Mutex
}

// NewListener creates a new P2P listener
func NewListener(config *Config, logger *slog.Logger) (*Listener, error) {
	if config.TopicPrefix == "" {
		config.TopicPrefix = "teratestnet"
	}
	if config.PeerCacheFile == "" {
		config.PeerCacheFile = "peer_cache.json"
	}
	if logger == nil {
		logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Listener{
		config:    config,
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
		blockCh:   make(chan []byte, 100),
		subtreeCh: make(chan []byte, 100),
		statusCh:  make(chan []byte, 100),
	}, nil
}

// Start initializes the P2P client and begins listening
func (l *Listener) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.logger.Info("P2P Listener starting", "port", l.config.Port, "network", l.config.TopicPrefix)

	// Get or generate private key
	var privKey crypto.PrivKey
	var err error

	if l.config.PrivateKey != "" {
		privKey, err = p2p.PrivateKeyFromHex(l.config.PrivateKey)
		if err != nil {
			return fmt.Errorf("failed to decode private key: %w", err)
		}
	} else {
		privKey, err = p2p.GeneratePrivateKey()
		if err != nil {
			return fmt.Errorf("failed to generate private key: %w", err)
		}
		keyHex, _ := p2p.PrivateKeyToHex(privKey)
		l.logger.Info("Generated new private key", "key", keyHex)
	}

	// Create P2P client with simpler API
	clientConfig := p2p.Config{
		Name:          "inspiration-indexer",
		Logger:        NewSlogAdapter(l.logger),
		PrivateKey:    privKey,
		Port:          l.config.Port,
		PeerCacheFile: l.config.PeerCacheFile,
	}

	// Only add bootstrap peers if provided (otherwise use defaults)
	if len(l.config.BootstrapPeers) > 0 {
		clientConfig.BootstrapPeers = l.config.BootstrapPeers
	}

	client, err := p2p.NewClient(clientConfig)
	if err != nil {
		return fmt.Errorf("failed to create P2P client: %w", err)
	}

	l.client = client

	// Subscribe to topics using the correct format: teranode/bitcoin/1.0.0/{network}-{type}
	blockTopic := fmt.Sprintf("teranode/bitcoin/1.0.0/%s-block", l.config.TopicPrefix)
	subtreeTopic := fmt.Sprintf("teranode/bitcoin/1.0.0/%s-subtree", l.config.TopicPrefix)
	statusTopic := fmt.Sprintf("teranode/bitcoin/1.0.0/%s-node_status", l.config.TopicPrefix)

	l.logger.Info("Subscribing to topics",
		"block", blockTopic,
		"subtree", subtreeTopic,
		"status", statusTopic)

	// Subscribe to each topic and forward messages to appropriate channels
	blockMsgChan := l.client.Subscribe(blockTopic)
	subtreeMsgChan := l.client.Subscribe(subtreeTopic)
	statusMsgChan := l.client.Subscribe(statusTopic)

	// Start goroutines to forward messages
	go l.forwardMessages(blockMsgChan, l.blockCh, "block")
	go l.forwardMessages(subtreeMsgChan, l.subtreeCh, "subtree")
	go l.forwardMessages(statusMsgChan, l.statusCh, "status")

	l.logger.Info("P2P listener successfully started", "peerID", l.client.GetID())

	return nil
}

// forwardMessages forwards messages from the p2p channel to our internal channel
func (l *Listener) forwardMessages(msgChan <-chan p2p.Message, outChan chan<- []byte, topic string) {
	for msg := range msgChan {
		l.logger.Debug("Received message",
			"topic", topic,
			"from", msg.From,
			"fromID", msg.FromID,
			"size", len(msg.Data))

		select {
		case outChan <- msg.Data:
		default:
			l.logger.Warn("Channel full, dropping message", "topic", topic)
		}
	}
	l.logger.Warn("Topic channel closed", "topic", topic)
}

// SubscribeBlocks returns a channel for block messages
func (l *Listener) SubscribeBlocks() <-chan []byte {
	return l.blockCh
}

// SubscribeSubtrees returns a channel for subtree messages
func (l *Listener) SubscribeSubtrees() <-chan []byte {
	return l.subtreeCh
}

// SubscribeStatus returns a channel for node status messages
func (l *Listener) SubscribeStatus() <-chan []byte {
	return l.statusCh
}

// Stop shuts down the P2P listener
func (l *Listener) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.cancel()

	if l.client != nil {
		return l.client.Close()
	}

	return nil
}

// PeerCount returns the number of connected peers
func (l *Listener) PeerCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.client == nil {
		return 0
	}

	return len(l.client.GetPeers())
}

// GetPeers returns information about all connected peers
func (l *Listener) GetPeers() []p2p.PeerInfo {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.client == nil {
		return nil
	}

	return l.client.GetPeers()
}

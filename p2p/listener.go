package p2p

import (
	"context"
	"fmt"
	"log"

	p2pMessageBus "github.com/bsv-blockchain/go-p2p-message-bus"
)

// Config holds P2P listener configuration
type Config struct {
	ListenAddresses []string
	Port            int
	BootstrapPeers  []string
	PrivateKey      string // hex-encoded private key
	BlockTopic      string
	SubtreeTopic    string
}

// Listener handles P2P network communication
type Listener struct {
	config    *Config
	p2pClient p2pMessageBus.P2PClient
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewListener creates a new P2P listener
func NewListener(config *Config) (*Listener, error) {
	if config.BlockTopic == "" {
		config.BlockTopic = "blocks"
	}
	if config.SubtreeTopic == "" {
		config.SubtreeTopic = "subtrees"
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Listener{
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Start initializes the P2P client and begins listening
func (l *Listener) Start() error {
	// TODO: Initialize P2P client with config
	// For now, this is a stub that will be implemented once we wire up the full flow
	log.Printf("P2P Listener starting on port %d", l.config.Port)
	log.Printf("Topics: blocks=%s, subtrees=%s", l.config.BlockTopic, l.config.SubtreeTopic)

	return nil
}

// SubscribeBlocks returns a channel for block messages
func (l *Listener) SubscribeBlocks() <-chan []byte {
	ch := make(chan []byte, 100)

	// TODO: Wire up actual P2P subscription
	// For now return empty channel

	return ch
}

// SubscribeSubtrees returns a channel for subtree messages
func (l *Listener) SubscribeSubtrees() <-chan []byte {
	ch := make(chan []byte, 100)

	// TODO: Wire up actual P2P subscription

	return ch
}

// PublishBlock publishes a block to the network
func (l *Listener) PublishBlock(ctx context.Context, block []byte) error {
	if l.p2pClient == nil {
		return fmt.Errorf("p2p client not initialized")
	}

	return l.p2pClient.Publish(ctx, l.config.BlockTopic, block)
}

// PublishSubtree publishes a subtree to the network
func (l *Listener) PublishSubtree(ctx context.Context, subtree []byte) error {
	if l.p2pClient == nil {
		return fmt.Errorf("p2p client not initialized")
	}

	return l.p2pClient.Publish(ctx, l.config.SubtreeTopic, subtree)
}

// Stop shuts down the P2P listener
func (l *Listener) Stop() error {
	l.cancel()

	if l.p2pClient != nil {
		return l.p2pClient.Close()
	}

	return nil
}

// PeerCount returns the number of connected peers
func (l *Listener) PeerCount() int {
	if l.p2pClient == nil {
		return 0
	}

	// TODO: Implement peer counting
	return 0
}

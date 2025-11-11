package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/shruggr/inspiration/p2p"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: checkpeer <peer-id>")
		fmt.Println("Example: checkpeer 12D3KooWLAgVxTxSxjKdpLJhibJU1ASs61dyaQikA18RTvxxufnX")
		os.Exit(1)
	}

	targetPeerID := os.Args[1]

	bootstrapPeer := "/dns4/teranode-bootstrap-stage.bsvb.tech/tcp/9901/p2p/12D3KooWJ6kQHAR65xkA34NABsNVAJyVxPWh8JUSo1vtZsTyw4GD"

	p2pConfig := &p2p.Config{
		Port:           9906, // Use different port to not conflict
		BootstrapPeers: []string{bootstrapPeer},
		BlockTopic:     "block",
		SubtreeTopic:   "subtree",
		TopicPrefix:    "teratestnet",
		UsePrivateDHT:  false,
	}

	listener, err := p2p.NewListener(p2pConfig)
	if err != nil {
		log.Fatalf("Failed to create P2P listener: %v", err)
	}

	if err := listener.Start(); err != nil {
		log.Fatalf("Failed to start P2P listener: %v", err)
	}
	defer listener.Stop()

	log.Printf("Checking for peer %s...", targetPeerID)
	log.Printf("Total connected peers: %d", listener.PeerCount())

	peers := listener.GetConnectedPeers()

	found := false
	for _, peer := range peers {
		peerIDStr := peer.ID.String()
		if peerIDStr == targetPeerID {
			found = true
			log.Printf("✓ Found peer %s", targetPeerID)
			peerJSON, _ := json.MarshalIndent(peer, "", "  ")
			fmt.Println(string(peerJSON))
			break
		}
	}

	if !found {
		log.Printf("✗ Peer %s not found in connected peers", targetPeerID)
		log.Println("\nAll connected peer IDs:")
		for i, peer := range peers {
			fmt.Printf("%d. %s\n", i+1, peer.ID.String())
		}
	}
}

#!/bin/bash
# Run the inspiration indexer on teratestnet

# Teratestnet bootstrap peer
BOOTSTRAP_PEER="/dns4/teranode-bootstrap-stage.bsvb.tech/tcp/9901/p2p/12D3KooWJ6kQHAR65xkA34NABsNVAJyVxPWh8JUSo1vtZsTyw4GD"

# Run the indexer
go run ./cmd/indexer \
  --topic-prefix=teratestnet \
  --bootstrap-peers="$BOOTSTRAP_PEER" \
  --p2p-port=9905 \
  --storage=badger \
  --data-dir=./data

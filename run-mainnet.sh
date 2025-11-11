#!/bin/bash
# Run the inspiration indexer on mainnet

# Mainnet bootstrap peer (derived from teratestnet pattern)
# Note: This may need to be updated with the actual mainnet bootstrap peer address
BOOTSTRAP_PEER="/dns4/teranode-bootstrap.bsvb.tech/tcp/9901/p2p/12D3KooWJ6kQHAR65xkA34NABsNVAJyVxPWh8JUSo1vtZsTyw4GD"

# Run the indexer
go run ./cmd/indexer \
  --topic-prefix=mainnet \
  --bootstrap-peers="$BOOTSTRAP_PEER" \
  --p2p-port=9906 \
  --storage=badger \
  --data-dir=./data-mainnet

# Inspiration - BSV Transaction Indexer

A lightweight, read-only Bitcoin SV indexer built on the Teranode libp2p network with content-addressed hierarchical indexing.

## Overview

Inspiration indexes Bitcoin SV transactions as they flow through the network, storing them in a distributed, content-addressed tree structure. The design prioritizes immutability, efficient queries, and P2P distribution.

**Key Features:**
- **Content-Addressed Trees** - Index nodes identified by BLAKE3 hash
- **Dual Storage Modes** - Optimized for both hash keys and text
- **Immutable Design** - Append-only with clean reorg handling
- **Pluggable Indexers** - Extensible extraction of index terms
- **P2P Native** - Built for distributed, decentralized indexing

## Architecture

### Hierarchical Index Structure

```
subtree_merkleRoot → indexed_key → indexed_value → subtree_index → txid
```

**Why this works:**
- `subtree_merkleRoot` already identifies the block/subtree
- Each level is a content-addressed index node
- Binary searchable at every level
- Clean reorg handling (delete by subtree)

### Binary Index Node Format

**Mode 0: Fixed-Width Keys** (for hash keys)
- 64-byte entries: `key(32) + child_hash(32)`
- Direct binary search, no offset table
- 11% space savings vs variable mode

**Mode 1: Variable-Width Keys** (for text)
- Offset table + variable-length entries
- Human-readable keys preserved
- Binary searchable via offset table

Both modes use **BLAKE3** for content addressing (10x faster than SHA256).

### Storage Layer

**BadgerDB** - Production storage (default)
- Embedded LSM-tree database
- Optimized for write-heavy workloads
- No server required for development

**MemoryStore** - Testing storage
- In-memory sync.Map implementation
- Fast, ephemeral, useful for tests

### Pluggable Indexers

```go
type Indexer interface {
    Index(ctx context.Context, tx *TransactionContext) ([]*IndexResult, error)
    Name() string
}
```

Extract custom index terms from transactions. Combine multiple indexers with `MultiIndexer`.

## Usage

### Build & Run

```bash
# Build
go build ./cmd/indexer

# Run with BadgerDB (default)
./indexer -storage=badger -data-dir=./data

# Run with in-memory storage
./indexer -storage=memory
```

### Development

```bash
# Run tests
go test ./...

# Run specific test
go test ./internal/storage -v -run TestFixedKeyIndexNode

# Build
go build ./cmd/indexer
```

## Project Status

See [PROGRESS.md](PROGRESS.md) for detailed status and roadmap.

### Completed ✅
- Binary index node format (dual-mode)
- BadgerDB storage backend
- Indexer plugin system
- Transaction processor with reorg handling
- Block header tracking
- P2P listener scaffold

### In Progress 🚧
- Tree builder & walker
- Actual libp2p P2P integration
- Concrete indexers (addresses, OP_RETURN, etc.)

### Planned 📋
- Query API (HTTP/gRPC)
- Kafka consumer for teranode integration
- Performance monitoring
- Distributed P2P storage backend

## Key Design Decisions

### 1. Content-Addressable Index Nodes

Each index node is stored by its BLAKE3 hash. This enables:
- **P2P distribution** - Request node by hash from any peer
- **Deduplication** - Same structure = same hash
- **Verification** - Peers can verify data integrity
- **Immutability** - Hash changes if content changes

### 2. Subtree-Based Hierarchy

Starting from `subtree_merkleRoot` (not `block_height`) because:
- Subtrees are self-contained units
- Block height stored separately: `height → [subtree_merkleRoots]`
- Easier to shard across nodes
- Cleaner reorg handling

### 3. Dual-Mode Binary Format

Supporting both fixed and variable width because:
- Hash keys (32 bytes) are common at intermediate levels
- Text keys (addresses, protocols) are common at leaf levels
- 11% space savings for hash keys
- Human-readable text keys preserved

### 4. BLAKE3 Over SHA256

- 10x faster hashing performance
- Still cryptographically secure
- Critical for high-throughput indexing
- Already in libp2p dependencies

## Component Overview

### Core Packages

- [`internal/storage/`](internal/storage/) - Storage interface and implementations
  - `indexnode.go` - Binary index node format
  - `badger.go` - BadgerDB implementation
  - `memory.go` - In-memory implementation

- [`internal/indexer/`](internal/indexer/) - Pluggable indexer system
  - `indexer.go` - Interface definitions
  - `noop.go` - Placeholder implementation

- [`internal/processor/`](internal/processor/) - Transaction processing
  - `processor.go` - Core indexing logic

- [`internal/p2p/`](internal/p2p/) - P2P network integration
  - `listener.go` - libp2p subscriber (stubbed)

- [`internal/models/`](internal/models/) - Data structures
  - `headers.go` - Block header chain

- [`cmd/indexer/`](cmd/indexer/) - Main entry point

## Development Roadmap

### Phase 1: Tree Operations (Current)
- Tree builder for multi-level index construction
- Tree walker for root-to-leaf navigation
- Integration with processor

### Phase 2: Network Integration
- Wire up libp2p client
- Parse block/subtree messages
- Store block announcements

### Phase 3: Practical Indexers
- Address indexer (inputs/outputs)
- OP_RETURN content indexer
- Token protocol indexers

### Phase 4: Query Layer
- REST API for index queries
- WebSocket for real-time updates
- Transaction retrieval

### Phase 5: Distribution
- Kafka consumer option
- Multi-node coordination
- Performance optimization

## Contributing

This is an active development project. See [PROGRESS.md](PROGRESS.md) for current status and next steps.

## License

[Add license information]

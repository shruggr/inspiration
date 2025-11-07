# Development Progress

## Current Status: Storage Architecture Complete + IPLD Integration Designed ✅

The storage layer has been completely reorganized with a clean separation of concerns. All packages moved out of `internal/` for external access. Type-safe `Hash` type for all 32-byte hashes.

**NEW:** IPLD multihash integration designed for IPFS distribution. See [IPLD_INTEGRATION_PLAN.md](IPLD_INTEGRATION_PLAN.md) for details on the new metadata schema and merkle tree storage strategy.

---

## Completed Components

### 1. KVStore Layer ✅

**Generic Key-Value Interface** - Storage abstraction for 32-byte hash keys
- `kvstore.Hash` type for type-safe 32-byte hashes (SHA256 or BLAKE3)
- Simple interface: Put, Get, Delete, Close
- No coupling to specific data structures
- **Note:** Will be updated to support variable-length multihash keys (see IPLD_INTEGRATION_PLAN.md)

**BadgerDB Implementation** ([kvstore/badger/badger.go](kvstore/badger/badger.go))
- Embedded LSM-tree database
- Efficient for write-heavy workloads
- Native support for hash keys

**Memory Store** ([kvstore/memory/memory.go](kvstore/memory/memory.go))
- In-memory sync.Map backed
- For testing and development

### 2. IndexNode Format ✅

**4-Mode Unified Format** ([indexnode/indexnode.go](indexnode/indexnode.go))

Mode bitmask (2 bits):
- Bit 0: Key type (0=value/variable, 1=pointer/fixed 32 bytes)
- Bit 1: Has data (0=no data, 1=data section)

**Mode 0 (0b00)**: Variable keys, no data
**Mode 1 (0b01)**: Fixed 32-byte keys, no data (11% space savings)
**Mode 2 (0b10)**: Variable keys, with data
**Mode 3 (0b11)**: Fixed 32-byte keys, with data (for block→subtree mapping)

**Features:**
- BLAKE3 hashing (10x faster than SHA256)
- Content-addressable storage
- Binary search: O(log n)
- Deterministic serialization
- Metadata support without breaking binary search

**Test Coverage:**
- [indexnode/indexnode_test.go](indexnode/indexnode_test.go) - Variable-width tests
- [indexnode/indexnode_fixed_test.go](indexnode/indexnode_fixed_test.go) - Fixed-width + data tests

### 3. Metadata Store ✅ (TO BE UPDATED FOR IPLD)

**SQLite-Backed Block Tracking** ([metadata/sqlite/sqlite.go](metadata/sqlite/sqlite.go))

Current stores minimal metadata:
- `height → {block_hash, merkle_root}`
- Fast height-based queries
- Block hash lookups
- Reorg cleanup

**Current Schema:**
```sql
CREATE TABLE blocks (
    height INTEGER PRIMARY KEY,
    block_hash BLOB NOT NULL,
    merkle_root BLOB NOT NULL
);
CREATE INDEX idx_blocks_hash ON blocks(block_hash);
```

**Planned Update:** New schema with blocks + subtrees tables, status tracking, and IPLD merkle tree references. See [IPLD_INTEGRATION_PLAN.md](IPLD_INTEGRATION_PLAN.md).

### 4. Index Term Cache ✅

**LRU Cache for Parsed Transactions** ([cache/memory/memory.go](cache/memory/memory.go))

Purpose:
- Cache parsed index terms from transactions
- Avoid re-parsing when same tx appears in multiple subtrees (pre-mining)
- Especially important for mempool transactions

Interface: Get, Put, Delete, Clear
Implementation: hashicorp/golang-lru with sync.RWMutex

### 5. Domain Models ✅

**Block Headers** ([models/headers.go](models/headers.go))
- BlockHeader struct
- HeaderChain for tip tracking
- In-memory chain state

**Transaction Indexer** ([txindexer/](txindexer/))
- Indexer interface for extracting index terms
- NoopIndexer placeholder
- Ready for custom implementations

### 6. P2P & Processor Scaffolds ✅

**P2P Listener** ([p2p/listener.go](p2p/listener.go))
- libp2p integration scaffold
- Block/subtree subscription channels

**Processor** ([processor/processor.go](processor/processor.go))
- Stubbed for new architecture
- TODO: Implement tree building logic
- TODO: Integrate cache and metadata stores

### 7. Main Entry Point ✅

**CLI Tool** ([cmd/indexer/main.go](cmd/indexer/main.go))
- Storage type selection (badger/memory)
- Component initialization
- Event loop structure
- Graceful shutdown

---

## Storage Architecture

### Current Data Flow

```
Block arrives via P2P
  ↓
Parse 80-byte header → extract merkle_root, block_hash
  ↓
For each subtree:
  - Fetch txids from URL
  - For each tx:
    * Check IndexTermCache for parsed terms
    * If miss: parse with indexer → store in cache
    * Store raw tx in KVStore: txid → raw_tx_bytes
  - Build index tree for subtree
  - Store index nodes in KVStore: node_hash (BLAKE3) → node_bytes
  ↓
Build block→subtree index (Mode 3 IndexNode)
  - Keys: subtree merkle roots
  - Child hashes: index root hash for each subtree
  - Data: tx count (4 bytes)
  ↓
Store in KVStore: merkle_root → block_subtree_index_bytes
  ↓
Store block header: block_hash → header_bytes (80 bytes)
  ↓
Store metadata: MetadataStore.PutBlock({height, block_hash, merkle_root})
```

### Storage Locations

**KVStore (BadgerDB):**
- `block_hash` → block header (80 bytes)
- `merkle_root` → block→subtree index (IndexNode Mode 3) [TO BE REPLACED]
- `node_hash` → index tree nodes (BLAKE3)
- `txid` → raw transaction bytes

**MetadataStore (SQLite):**
- `height` → `{block_hash, merkle_root}` [TO BE EXPANDED]

**IndexTermCache (LRU):**
- `txid` → `[]IndexTerm` (parsed index terms)

### Key Design Decisions

1. **Block→Subtree keyed by merkle_root**, not block_hash
   - Avoids key conflicts
   - Merkle root is what commits to transactions

2. **Minimal SQL storage**
   - Just height tracking
   - Everything else content-addressable

3. **Type-safe hashes**
   - `kvstore.Hash [32]byte` for all hashes
   - Compile-time safety

4. **Cache for performance**
   - Avoid re-parsing txs in multiple subtrees
   - Critical for pre-mining performance

---

## Architecture Decisions

### Package Organization

All packages exposed at root level (no `internal/`):
- `kvstore/` - Generic KV storage
- `indexnode/` - Tree node format
- `metadata/` - Block metadata (SQLite)
- `cache/` - Index term caching
- `models/` - Domain models
- `processor/` - Transaction processing
- `p2p/` - Network layer
- `txindexer/` - Index term extraction

### Why This Works

**Content-Addressable Everything:**
- Index nodes identified by BLAKE3 hash
- Natural deduplication
- P2P distributable
- Verifiable

**Separation of Concerns:**
- KVStore: Just hash→bytes mapping
- MetadataStore: Just height→hash mapping
- IndexNode: Just tree structure
- Cache: Just parsed term storage

**Flexible Indexing:**
- Pluggable indexer interface
- Can add new index types
- Multi-indexer support

---

## IPLD Integration (Designed, Not Implemented)

We've researched and designed IPLD multihash integration to enable IPFS distribution of our database. Key decisions:

### What Changes
1. **Metadata schema:** New blocks + subtrees tables with foreign key relationships
2. **KVStore keys:** Variable-length multihash instead of fixed 32-byte hashes
3. **Merkle trees:** Store Bitcoin merkle trees in IPLD format (64-byte nodes)
4. **Block status:** Track main/orphan/pending for reorg handling

### What Stays the Same
- IndexNode format and BLAKE3 hashing
- Index tree structure
- Cache layer
- Overall architecture

### Benefits
- IPFS distribution of transaction data
- Self-describing hashes
- Efficient merkle proof construction
- Better reorg handling

**Full details:** See [IPLD_INTEGRATION_PLAN.md](IPLD_INTEGRATION_PLAN.md) and [IPLD_RESEARCH_FINDINGS.md](IPLD_RESEARCH_FINDINGS.md)

---

## Next Steps

### Immediate: IPLD Integration Implementation

Follow the plan in [IPLD_INTEGRATION_PLAN.md](IPLD_INTEGRATION_PLAN.md):

1. **Phase 1:** Update Metadata Store with new schema
2. **Phase 2:** Create multihash wrapper package
3. **Phase 3:** Update KVStore for variable-length keys
4. **Phase 4:** Implement IPLD merkle tree builder
5. **Phase 5:** Integration
6. **Phase 6:** Testing

### After IPLD Implementation

1. **Message Parsing:** BlockMessage and SubtreeMessage from protobuf
2. **Tree Builder:** Build index trees for subtrees
3. **Processor Implementation:** Wire up the full pipeline
4. **Integration & Testing:** End-to-end flow

---

## Testing Status

### Unit Tests ✅

- IndexNode marshal/unmarshal (all 4 modes)
- Binary search
- Hash computation (BLAKE3)
- Size comparisons

### Integration Tests ⏳

**TODO:**
- Full block processing pipeline
- Cache hit/miss scenarios
- Reorg handling
- Concurrent access
- IPLD merkle proof generation and verification

### Performance Tests ⏳

**TODO:**
- Indexing throughput
- Query latency
- Cache effectiveness
- Storage efficiency

---

## Known Limitations

1. **IPLD Integration** - Designed but not implemented
2. **Message Parsing** - Not yet implemented
3. **Tree Builder** - Not yet implemented
4. **Processor** - Stubbed, needs implementation
5. **Query API** - Not exposed yet
6. **No Monitoring** - No metrics/observability

---

## Performance Characteristics

### Expected Performance

**BLAKE3 Hashing:**
- 3-7 GB/s throughput
- 10x faster than SHA256

**BadgerDB:**
- LSM-tree optimized for writes
- Efficient prefix scans
- Built-in compaction

**Target:**
- Keep up with 1M+ tx/sec blockchain
- Sub-second block processing
- Low memory footprint with LRU cache

### Bottlenecks to Watch

- Indexer plugin performance
- Network I/O for subtree fetching
- BadgerDB compaction
- Cache thrashing

---

## Development Environment

**Requirements:**
- Go 1.21+
- BadgerDB v4
- SQLite3
- BLAKE3 library
- hashicorp/golang-lru
- go-multihash (for IPLD integration)
- go-cid (for IPLD integration)

**Build:**
```bash
go build ./cmd/indexer
```

**Test:**
```bash
go test ./...
```

**Run:**
```bash
./indexer -storage=badger -data-dir=./data
```

**Clean:**
```bash
rm -rf ./data
```

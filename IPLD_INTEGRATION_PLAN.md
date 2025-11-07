# Development Progress

## Current Status: IPLD Multihash Integration Designed ✅

Research completed on IPLD/IPFS integration. New metadata schema designed with support for IPLD Bitcoin merkle trees. Ready to implement.

---

## Completed Research

### IPLD Multihash Integration Research ✅
- ✅ Verified IPLD support for double SHA256 (`dbl-sha2-256`, code `0x56`) - **built-in**
- ✅ Verified IPLD support for BLAKE3 (code `0x1e`) - requires optional import
- ✅ Analyzed multihash binary format and overhead (2 bytes per hash)
- ✅ Researched DAG-CBOR encoding - **determined not needed for our use case**
- ✅ Confirmed Bitcoin merkle trees use standard `dbl-sha2-256` multihash
- ✅ Understood IPLD Bitcoin merkle tree structure (64-byte nodes: left || right)

**Key Decision:** Use multihash for IPFS distribution of our KV database

**Documentation:** See [IPLD_RESEARCH_FINDINGS.md](IPLD_RESEARCH_FINDINGS.md)

### Architecture Decisions Made

#### Multihash Strategy
- **Blockchain data** (txids, block hashes, merkle roots): Keep as raw 32-byte hashes in application layer
- **Storage keys in KVStore**: Use multihash wrappers (34 bytes)
  - Bitcoin data: `dbl-sha2-256` (0x56)
  - Index structures: `blake3` (0x1e)
- **IPFS distribution**: Store with multihash keys, enabling content-addressed retrieval

#### Merkle Tree Storage (IPLD Format)
- Store Bitcoin merkle trees using **IPLD structure**:
  - **Key:** Multihash of Bitcoin hash (`dbl-sha2-256`)
  - **Value:** 64 bytes raw (left_hash || right_hash)
  - Internal hashes are **raw Bitcoin hashes** (not multihash)
  - Hash of the 64-byte node equals the Bitcoin merkle node hash
- Benefits:
  - Native IPFS/IPLD compatibility
  - Can distribute via IPFS network
  - Efficient merkle proof construction by walking IPLD tree
  - No need to store block→subtree mapping separately

#### Metadata Schema Design

**Blocks Table:**
```sql
CREATE TABLE blocks (
    height       INTEGER NOT NULL,
    block_hash   BLOB NOT NULL,
    merkle_root  BLOB PRIMARY KEY,
    tx_count     INTEGER NOT NULL,
    status       TEXT NOT NULL DEFAULT 'main',
    timestamp    INTEGER,
    created_at   INTEGER DEFAULT (strftime('%s', 'now'))
);

CREATE INDEX idx_blocks_status_height ON blocks(status, height);
CREATE UNIQUE INDEX idx_blocks_hash ON blocks(block_hash);
```

**Subtrees Table (1:many with blocks):**
```sql
CREATE TABLE subtrees (
    merkle_root         BLOB NOT NULL,
    subtree_index       INTEGER NOT NULL,
    subtree_merkle_root BLOB NOT NULL,
    tx_count            INTEGER NOT NULL,
    index_root          BLOB NOT NULL,
    tx_tree_root        BLOB NOT NULL,

    PRIMARY KEY (merkle_root, subtree_index),
    FOREIGN KEY (merkle_root) REFERENCES blocks(merkle_root) ON DELETE CASCADE
);

CREATE INDEX idx_subtrees_merkle_root_subtree_index ON subtrees(merkle_root, subtree_index);
```

**Key Design Decisions:**
- Foreign key: `merkle_root` (immutable, unique identifier for block's transaction set)
- Block status: `'main'`, `'orphan'`, `'pending'` for reorg handling
- Block-level `tx_count`: Enables efficient merkle proof construction without summing subtrees
- Ordered subtrees via `subtree_index`
- Orphaned blocks kept for 100 blocks before cleanup

#### Data Storage Breakdown

**SQLite Metadata:**
- Block metadata with ordered subtrees list
- Fast queries for subscription workflows
- Handles reorgs with status field

**KVStore (BadgerDB/IPFS):**
1. **Subtree TX merkle trees** (IPLD format)
   - Keys: `dbl-sha2-256` multihash
   - Values: 64-byte nodes (left || right)

2. **Index trees** (existing format)
   - Keys: `blake3` multihash
   - Values: IndexNode binary format

3. **Transactions** (optional)
   - Keys: `dbl-sha2-256` multihash of txid
   - Values: Raw transaction bytes

---

## Key Workflows Designed

### Search/Subscription Workflow
1. Query SQLite: `JOIN blocks + subtrees WHERE status='main' ORDER BY height, subtree_index`
2. For each subtree in order:
   - Fetch index tree using `index_root` multihash
   - Search for term
   - Return results with block position calculated from subtree offsets
3. Stream results to user

### Merkle Proof Construction
1. Get block metadata and subtrees from SQLite
2. Calculate position within subtree using tx_count offsets
3. Walk IPLD tree in KVStore using `tx_tree_root` multihash (subtree → tx)
4. Build block-level proof from subtree merkle roots (on-the-fly, small tree)
5. Combine proofs and return

### Reorg Handling
1. Mark existing blocks at height as `status='orphan'`
2. Insert new block with `status='main'`
3. Cleanup orphans older than 100 blocks (finality depth)
4. CASCADE delete removes associated subtrees automatically

---

## Understanding of Subtrees
- Subtrees are teranode's abstraction for efficient block validation
- Powers of 2 structures (up to 2^10 = 1024 transactions)
- Pre-computed merkle tree chunks that fit seamlessly into Bitcoin block merkle tree
- Transparent to Bitcoin merkle tree structure (can be concatenated/hashed without rehashing)
- Boundaries are logical grouping for indexing, not a separate merkle tree layer

---

## Previous Completed Components

### 1. KVStore Layer ✅
- Generic key-value interface with 32-byte hash keys
- BadgerDB implementation
- Memory store for testing

### 2. IndexNode Format ✅
- 4-mode unified format (variable/fixed keys × with/without data)
- BLAKE3 hashing
- Binary search support
- Full test coverage

### 3. Metadata Store ✅ (TO BE UPDATED)
- SQLite-backed block tracking
- Currently: simple height → {block_hash, merkle_root}
- **Needs migration to new schema**

### 4. Index Term Cache ✅
- LRU cache for parsed transactions
- Avoids re-parsing in multiple subtrees

### 5. Domain Models ✅
- Block headers
- Transaction indexer interface

---

## Next Steps

See [NEXT_PROMPT.md](NEXT_PROMPT.md) for implementation plan.

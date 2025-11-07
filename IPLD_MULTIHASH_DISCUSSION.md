# IPLD Multihash Integration Discussion

## Context

We're building a Bitcoin SV transaction indexing system with the following architecture:

### Current Implementation

**Storage Layout:**
- **KVStore (BadgerDB)**: Content-addressed storage using 32-byte hashes as keys
  - Block headers: `block_hash (SHA256)` → raw header bytes
  - Index nodes: `node_hash (BLAKE3)` → IndexNode bytes
  - Transactions: `txid (SHA256)` → raw tx bytes
  - Block→subtree mapping: `merkle_root (SHA256)` → IndexNode bytes
  - TxID lists: `list_hash (BLAKE3)` → serialized txid array

- **MetadataStore (SQLite)**: `height` → `{block_hash, merkle_root}`

- **Index Structure**: Two-level tree
  - Root node: `indexed_key` → `leaf_node_hash (BLAKE3)`
  - Leaf nodes: `indexed_value` → `txid_list_hash (BLAKE3)`
  - TxID lists: Sorted array of transaction IDs

### The Problem

Currently, we're using two different hash algorithms in the same `kvstore.Hash` type:
- **SHA256**: For blockchain data (txids, block hashes, merkle roots)
- **BLAKE3**: For index structure nodes (tree nodes, txid lists)

**Issue**: When retrieving data from KVStore by hash, there's no way to know:
1. Which hash algorithm was used to generate that hash
2. How to verify that retrieved data matches the expected hash
3. What format/type the data is (index node, txid list, transaction, etc.)

This creates problems for:
- **Independent verification**: Users can't verify they received the correct data
- **Data integrity**: No self-describing hashes
- **Interoperability**: Hard to share or validate data across systems

## IPLD/IPFS Background

**IPLD (InterPlanetary Linked Data)** uses **multihash** format:
```
<hash-algorithm-identifier><hash-length><hash-value>
```

Examples:
- `sha2-256`: `0x12` prefix
- `blake3`: `0x1e` prefix
- `blake2b-256`: `0xb220` prefix

**Benefits:**
- Self-describing hashes (algorithm is embedded)
- Standard format used across IPFS ecosystem
- Existing libraries and tooling
- CID (Content Identifier) adds codec information

**IPLD also provides:**
- DAG-CBOR, DAG-JSON for encoding structures
- Schema system for data validation
- Existing implementations in Go

## Questions to Explore

### 1. Multihash Adoption

**Should we adopt IPLD multihash format for our hashes?**

Options:
- **A**: Use multihash for all hashes (txids, index nodes, everything)
  - Pro: Consistent, self-describing
  - Con: Bitcoin txids are not multihash format, would need wrapping

- **B**: Use multihash only for index structures (BLAKE3 nodes)
  - Pro: Keep Bitcoin data in native format
  - Con: Mixed approach, more complexity

- **C**: Keep current approach, add metadata separately
  - Pro: No format changes, simpler
  - Con: Still need to solve verification problem

**Specific concerns:**
- How does multihash overhead affect storage/performance?
- Can we use multihash while keeping `kvstore.Hash = chainhash.Hash` compatibility?
- What about existing Bitcoin tools that expect raw SHA256 hashes?

### 2. Data Format & Encoding

**Should we use IPLD's encoding formats?**

Current: Custom binary format for IndexNode (see `indexnode/indexnode.go`)
- 4 modes based on key type (fixed/variable) and data presence
- Optimized binary layout
- ~8 byte header + entries

IPLD options:
- **DAG-CBOR**: Compact binary format, widely used in IPFS
- **DAG-JSON**: Human-readable, larger
- **Custom codec**: Register our own format with IPLD

Questions:
- Would IPLD encoding provide benefits over our custom format?
- What's the size/performance tradeoff?
- Can we define an IPLD schema for IndexNode?

### 3. Verification & Validation

**How should users verify data integrity?**

Current approach: User must know which hash algorithm to use
```
data := kvstore.Get(hash)
// Now what? Hash with BLAKE3 or SHA256?
```

With multihash:
```
data := kvstore.Get(multihash)
// Extract algorithm from multihash prefix
// Verify: hash(data) == multihash
```

Questions:
- Do we need client-side verification? Who are the users?
- Is this an internal implementation detail or public API?
- What trust model are we building for?

### 4. Storage Key Format

**How should KVStore keys be structured?**

Current: Raw 32-byte hashes (ambiguous)

Options:
- **Use multihash as keys**: Self-describing but variable length
- **Namespace prefixes**: `index:<hash>`, `tx:<hash>`, `block:<hash>`
- **Separate stores**: One KVStore per data type
- **Metadata wrapper**: Store `{hash_algo, data}` instead of just data

Tradeoffs:
- Variable-length keys in BadgerDB?
- Key prefix scanning performance?
- Migration path from current implementation?

### 5. IPFS Integration

**Could/should we integrate with IPFS for data storage/distribution?**

Possibilities:
- Store index nodes in IPFS, reference by CID
- Use IPFS for distributing transaction data
- Hybrid: Local BadgerDB + IPFS for backup/sharing
- IPLD queries for traversing index structure

Questions:
- Is IPFS overkill for this use case?
- What about latency/availability requirements?
- Could we support both local and IPFS backends?

## Current Dependencies

We already have IPFS libraries in `go.mod`:
```go
github.com/ipfs/boxo v0.35.0
github.com/ipfs/go-cid v0.5.0
github.com/ipld/go-ipld-prime v0.21.0
```

These came transitively through `go-p2p-message-bus`. Could leverage them directly.

## Use Cases to Consider

1. **Local indexer node**: Single operator, no external verification needed
2. **Public index service**: Users query and need to verify responses
3. **Distributed index**: Multiple nodes sharing index data (IPFS-like)
4. **Light client**: Downloads partial index, verifies without full blockchain

Which use case(s) are we targeting?

## Proposed Discussion Flow

1. **Define the trust/verification requirements**
   - Who needs to verify what?
   - What attacks are we protecting against?

2. **Evaluate multihash options**
   - Pure multihash vs hybrid approach
   - Storage overhead analysis
   - Implementation complexity

3. **Consider IPLD encoding**
   - Compare custom binary format vs DAG-CBOR
   - Benchmark size/performance
   - Schema definition

4. **Design KVStore key strategy**
   - How to make hashes self-describing
   - Migration path from current code
   - Performance implications

5. **Decide on IPFS integration level**
   - None, optional, or core dependency?
   - What features would we use?

## References

- IPLD Multihash spec: https://multiformats.io/multihash/
- IPLD Specs: https://ipld.io/specs/
- go-multihash: https://github.com/multiformats/go-multihash
- go-cid: https://github.com/ipfs/go-cid
- DAG-CBOR spec: https://ipld.io/specs/codecs/dag-cbor/

## Next Steps

Please share your thoughts on:
1. Which use cases we're targeting (local, public, distributed, light client?)
2. Whether independent verification is a core requirement
3. Your preference on the multihash options (A, B, or C above)
4. Any performance/storage constraints we should be aware of

Then we can dive deeper into the technical implementation details.

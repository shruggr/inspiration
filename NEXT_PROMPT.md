# Next Session: IPLD Multihash Integration Implementation

## Context

We've completed the foundational storage architecture (KVStore, IndexNode, Metadata, Cache) and designed IPLD multihash integration for IPFS distribution.

**READ THESE FILES FIRST:**
1. **[PROGRESS.md](PROGRESS.md)** - Complete project history and current status
2. **[IPLD_INTEGRATION_PLAN.md](IPLD_INTEGRATION_PLAN.md)** - Detailed implementation plan with code examples
3. **[IPLD_RESEARCH_FINDINGS.md](IPLD_RESEARCH_FINDINGS.md)** - Research on multihash, IPLD, DAG-CBOR

---

## What We're Building

We're implementing IPLD multihash integration to enable IPFS distribution of our Bitcoin transaction index database. This involves:

1. **New metadata schema** with blocks + subtrees tables (1:many relationship)
2. **Multihash wrapper types** for self-describing hashes
3. **Variable-length KVStore keys** to support multihash
4. **IPLD Bitcoin merkle trees** stored as 64-byte nodes
5. **Efficient merkle proof construction** by walking IPLD tree structure

**Key benefit:** Our entire KV database can be distributed via IPFS while maintaining Bitcoin-compatible merkle tree verification.

---

## Implementation Phases

### Phase 1: Update Metadata Store ⏳

**Goal:** New SQLite schema with blocks + subtrees tables

**Details:** See [IPLD_INTEGRATION_PLAN.md - Phase 1](IPLD_INTEGRATION_PLAN.md#phase-1-update-metadata-store-)

**Key changes:**
- New Go types: `BlockMeta` (with `TxCount`, `Status`), `SubtreeMeta`
- SQL schema with foreign key: `subtrees.merkle_root → blocks.merkle_root`
- New methods: `PutBlock()`, `GetSubtrees()`, `MarkOrphan()`, `CleanupOrphans()`
- Transaction support for atomic block + subtrees insert

**Files to modify:**
- `metadata/store.go`
- `metadata/sqlite/sqlite.go`
- New: `metadata/sqlite/sqlite_test.go`

---

### Phase 2: Create Multihash Wrapper Package ⏳

**Goal:** Type-safe hash wrappers for different categories

**Details:** See [IPLD_INTEGRATION_PLAN.md - Phase 2](IPLD_INTEGRATION_PLAN.md#phase-2-create-multihash-wrapper-package-)

**New types:**
- `IndexHash` - BLAKE3 multihash for index structures
- `MerkleHash` - dbl-sha2-256 multihash for Bitcoin merkle trees

**Key functions:**
- `NewIndexHash(data []byte)` - Hash with BLAKE3
- `NewMerkleHash(hash [32]byte)` - Wrap existing Bitcoin hash
- `Verify(data []byte)` - Verify hash matches data
- `Raw() [32]byte` - Extract raw hash from multihash

**Files to create:**
- `multihash/hash.go`
- `multihash/hash_test.go`

---

### Phase 3: Update KVStore for Variable-Length Keys ⏳

**Goal:** Support multihash keys (34 bytes) instead of fixed 32-byte hashes

**Details:** See [IPLD_INTEGRATION_PLAN.md - Phase 3](IPLD_INTEGRATION_PLAN.md#phase-3-update-kvstore-for-variable-length-keys-)

**Interface change:**
```go
// OLD: type Hash = chainhash.Hash
Put(ctx context.Context, key Hash, value []byte) error

// NEW:
Put(ctx context.Context, key []byte, value []byte) error
```

**Files to modify:**
- `kvstore/store.go`
- `kvstore/memory/memory.go`
- `kvstore/badger/badger.go`

---

### Phase 4: Implement IPLD Merkle Tree Builder ⏳

**Goal:** Build Bitcoin merkle trees in IPLD format (64-byte nodes)

**Details:** See [IPLD_INTEGRATION_PLAN.md - Phase 4](IPLD_INTEGRATION_PLAN.md#phase-4-implement-ipld-merkle-tree-builder-)

**Core concept:**
- Store 64-byte nodes: `left_hash (32 bytes) || right_hash (32 bytes)`
- Internal hashes are raw Bitcoin hashes (NOT multihash)
- Storage key is multihash of the node's Bitcoin hash
- Enables efficient merkle proof by walking IPLD tree

**Key methods:**
- `BuildSubtreeMerkleTree(txids [][32]byte)` - Build and store tree
- `BuildMerkleProof(treeRoot, position, txCount)` - Walk tree for proof
- `BuildBlockMerkleProof(subtreeRoots, subtreeIndex)` - Block-level proof

**Files to create:**
- `merkle/builder.go`
- `merkle/builder_test.go`
- `merkle/proof.go`
- `merkle/proof_test.go`

---

### Phase 5: Integration ⏳

**Goal:** Wire everything together in existing code

**Details:** See [IPLD_INTEGRATION_PLAN.md - Phase 5](IPLD_INTEGRATION_PLAN.md#phase-5-integration-)

**Updates needed:**
- `treebuilder/implementation.go` - Use multihash, build IPLD trees
- `processor/processor.go` - Use new PutBlock(), build merkle trees
- `cmd/indexer/main.go` - Initialize new components

---

### Phase 6: Testing ⏳

**Goal:** Comprehensive testing of all components

**Details:** See [IPLD_INTEGRATION_PLAN.md - Phase 6](IPLD_INTEGRATION_PLAN.md#phase-6-testing-)

**Test coverage:**
- Metadata store (blocks, subtrees, reorgs, orphan cleanup)
- Multihash wrappers (create, verify, unwrap)
- Merkle builder (build tree, generate proofs, verify)
- Integration (end-to-end block processing with proofs)

---

## Implementation Order

1. ✅ Phase 1: Metadata store (foundation)
2. ✅ Phase 2: Multihash wrappers (type safety)
3. ✅ Phase 3: KVStore update (enable variable keys)
4. ✅ Phase 4: IPLD merkle builder (core functionality)
5. ✅ Phase 5: Integration (wire together)
6. ✅ Phase 6: Testing (validation)

**Start with Phase 1.** Each phase builds on the previous.

---

## Key Files to Understand

**Current implementation:**
- `metadata/store.go` - Current interface (will be updated)
- `metadata/sqlite/sqlite.go` - Current SQLite implementation
- `kvstore/store.go` - Current KV interface with Hash type
- `indexnode/indexnode.go` - Index tree node format (stays same)

**Design documents:**
- `IPLD_INTEGRATION_PLAN.md` - Complete implementation plan
- `IPLD_RESEARCH_FINDINGS.md` - Research and architectural decisions
- `PROGRESS.md` - Project history and current status

---

## Success Criteria

By end of implementation:
- ✅ New metadata schema working (blocks + subtrees tables)
- ✅ Multihash wrappers for type-safe hash handling
- ✅ KVStore supports variable-length multihash keys
- ✅ IPLD Bitcoin merkle trees stored and retrievable
- ✅ Can build merkle proofs by walking IPLD tree
- ✅ Search workflow works with new schema
- ✅ Reorg handling works correctly
- ✅ All tests passing

---

## Migration Notes

**Breaking changes:**
- `metadata.Store` interface changed (new methods, updated signatures)
- `kvstore.KVStore` interface changed (Hash → []byte)
- New dependency: multihash package

**Existing data:** Not compatible, requires fresh start (this is acceptable)

**Backwards compatibility:** Not maintained - this is a major architectural enhancement

---

## Quick Start for Next Session

```bash
# 1. Read the context files
cat PROGRESS.md
cat IPLD_INTEGRATION_PLAN.md

# 2. Start with Phase 1
# Update metadata/store.go with new types and interface
# Implement new SQLite schema in metadata/sqlite/sqlite.go

# 3. Write tests as you go
# metadata/sqlite/sqlite_test.go

# 4. Proceed through phases in order
```

**First task:** Update `metadata/store.go` with new `BlockMeta`, `SubtreeMeta`, and `Store` interface methods.

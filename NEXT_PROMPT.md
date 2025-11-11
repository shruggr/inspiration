# Next Development Session

## Quick Start

**Project Status:** Message fetching complete ✅ | Ready for subtree processing ⏳

**Start with this prompt:**

```
I need to implement subtree processing.

Context:
- SubtreeMessage JSON format: {Hash, URL, PeerID, ClientName}
- Fetcher functions are complete with smart optimization
- All storage infrastructure (KVStore, Cache, TreeBuilder, MerkleBuilder) is ready
- P2P listener is running and receiving subtree messages

See PROGRESS.md for complete context. Let's start by implementing the subtree
processing flow in the main event loop.
```

---

## Current Understanding

### What We Have ✅

**Message Fetching** - Three optimized functions:
1. `FetchSubtreeTxIDs(ctx, baseURL, subtreeHash)` → Get txid list (lightweight)
2. `FetchTransactionsByTxID(ctx, baseURL, txIDs)` → Fetch specific txs (general endpoint)
3. `FetchSubtreeTransactions(ctx, baseURL, subtreeHash, txIDs)` → Fetch from subtree (optimized)
4. `FetchMissingSubtreeTransactions(...)` → Smart helper (picks 2 or 3 based on cache hit rate)

**Storage Infrastructure:**
- KVStore: BadgerDB with multihash support
- IndexTermCache: LRU cache for parsed transactions
- MetadataStore: SQLite for block/subtree tracking
- TreeBuilder: Builds index trees (BLAKE3)
- MerkleBuilder: Builds IPLD merkle trees (dbl-sha2-256)

**P2P Layer:**
- Receiving JSON messages on channels
- Three topics: block, subtree, node_status
- Currently just logging messages

### What We Need ⏳

**Subtree Processing Flow:**

```go
// Main event loop receives subtreeData from P2P
case subtreeData := <-subtreeCh:
    // 1. Unmarshal JSON to SubtreeMessage struct
    var msg SubtreeMessage
    json.Unmarshal(subtreeData, &msg)

    // 2. Fetch txid list (lightweight discovery)
    txIDs := FetchSubtreeTxIDs(ctx, msg.URL, msg.Hash)

    // 3. Check which txs we already have in KVStore
    missingTxIDs := findMissingTxIDs(txIDs, kvstore)

    // 4. Smart fetch missing transactions
    txData := FetchMissingSubtreeTransactions(ctx, msg.URL, msg.Hash, txIDs, missingTxIDs)

    // 5. Process each transaction
    for txid, rawTx := range transactions {
        // a. Check cache for parsed terms
        terms := cache.Get(txid)
        if terms == nil {
            // b. Parse tx and extract index terms
            terms = indexer.Index(tx)
            cache.Put(txid, terms)
        }

        // c. Store raw tx in KVStore
        multihashKey := multihash.WrapMerkleHash(txid)
        kvstore.Put(multihashKey.Bytes(), rawTx)
    }

    // 6. Build index tree for subtree
    indexRoot := treeBuilder.BuildSubtreeIndex(ctx, msg.Hash, txsWithTerms)

    // 7. Build IPLD merkle tree
    merkleRoot := merkleBuilder.BuildSubtreeMerkleTree(ctx, txIDs)

    // 8. Store subtree metadata (pending until block arrives)
    // TODO: Need a "pending subtrees" store?
```

---

## Implementation Strategy

### Step 1: Add Helper Function to Check Missing Txs

```go
// In main.go or new helper package
func findMissingTxIDs(txIDs []kvstore.Hash, store kvstore.KVStore) []kvstore.Hash {
    var missing []kvstore.Hash
    for _, txid := range txIDs {
        // Wrap txid as multihash
        key := multihash.WrapMerkleHash(txid)

        // Check if exists in KVStore
        _, err := store.Get(ctx, key.Bytes())
        if err != nil {
            missing = append(missing, txid)
        }
    }
    return missing
}
```

### Step 2: Wire Up Subtree Message Handler

Location: [cmd/indexer/main.go](cmd/indexer/main.go#L161-L172)

Current code just logs:
```go
case subtreeData := <-subtreeCh:
    var subtreeMsg map[string]interface{}
    if err := json.Unmarshal(subtreeData, &subtreeMsg); err != nil {
        logger.Error("Failed to parse subtree message", "error", err)
        continue
    }
    logger.Info("SUBTREE", ...)
```

Replace with actual processing (see flow above).

### Step 3: Handle Transaction Parsing

Need to:
1. Parse raw transaction bytes using go-sdk
2. Extract txid by hashing
3. Pass to indexer for term extraction
4. Store in cache and KVStore

```go
import "github.com/bsv-blockchain/go-sdk/transaction"

tx := &transaction.Transaction{}
tx.ReadFrom(bytes.NewReader(rawTx))
txid := tx.TxID()  // Calculate hash
```

### Step 4: Build Index and Merkle Trees

```go
// Convert cached terms to TreeBuilder format
txsWithTerms := []treebuilder.TransactionWithTerms{}
for _, txid := range txIDs {
    terms := cache.Get(txid)
    txsWithTerms = append(txsWithTerms, treebuilder.TransactionWithTerms{
        TxID: txid,
        Terms: terms,
    })
}

// Build index tree
indexRoot := treeBuilder.BuildSubtreeIndex(ctx, subtreeHash, txsWithTerms)

// Build merkle tree
merkleRoot := merkleBuilder.BuildSubtreeMerkleTree(ctx, txIDs)
```

### Step 5: Store Subtree Metadata

**Question:** Where do we store subtree metadata before a block arrives?

Options:
1. In-memory map keyed by subtree merkle root
2. Separate "pending_subtrees" table in SQLite
3. Don't store until block arrives (just cache the trees)

Need to decide on approach.

---

## Open Questions

### 1. Pending Subtrees Storage

**Problem:** Subtrees arrive before blocks. Where do we store `SubtreeInfo` while waiting?

**Options:**
- **A:** In-memory map `map[kvstore.Hash]SubtreeInfo` (lost on restart)
- **B:** SQLite table `pending_subtrees` (persisted, can resume on restart)
- **C:** Don't store explicitly - rebuild from KVStore if needed

**Recommendation:** Start with Option A (in-memory) for simplicity, add Option B later if needed.

### 2. Transaction Ordering

**Problem:** Do txids need to be in a specific order for merkle tree?

**Answer:** Yes, must match Bitcoin block ordering. Need to preserve order from `FetchSubtreeTxIDs`.

### 3. Error Handling

**Problem:** What if subtree fetch fails? Retry? Skip?

**Strategy:**
- Log error with subtree hash and peer ID
- Continue processing other messages
- Maybe add retry queue later

### 4. Concurrency

**Problem:** Can we process multiple subtrees in parallel?

**Answer:** Not initially - keep it simple. Process sequentially in event loop. Optimize later.

---

## Files to Review

Before starting:
1. [PROGRESS.md](PROGRESS.md) - Full context on completed components
2. [messages/fetcher.go](messages/fetcher.go) - Three fetcher functions
3. [cmd/indexer/main.go](cmd/indexer/main.go#L161-L172) - Event loop to modify
4. [treebuilder/builder.go](treebuilder/builder.go) - TreeBuilder interface
5. [merkle/builder.go](merkle/builder.go) - MerkleBuilder interface

---

## Success Criteria

After implementing subtree processing:

✅ SubtreeMessages arrive and are unmarshaled correctly
✅ Txid list fetched successfully
✅ Missing txs identified and fetched (with smart optimization)
✅ Transactions parsed and stored in KVStore
✅ Index terms extracted and cached
✅ Index tree built and stored
✅ IPLD merkle tree built and stored
✅ Subtree metadata tracked (pending block arrival)
✅ Logging shows processing stats (txs fetched, cache hits, etc.)

---

## Next Steps After Subtree Processing

Once subtree processing works:
1. Implement block message handling
2. Wire up block → subtree assembly
3. Add concrete indexers (Address, OP_RETURN)
4. Build query/lookup system

---

## Notes

**Current P2P Status:**
- Listener is running and receiving real messages from teranode-testnet
- Messages are JSON format (not protobuf)
- Need to handle parsing errors gracefully

**Performance Considerations:**
- Cache is critical for performance (same tx in multiple subtrees)
- Smart fetch optimization saves bandwidth
- Tree building should be fast (BLAKE3 is 10x faster than SHA256)

Good luck! 🚀

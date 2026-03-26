# JungleBus Indexer v2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a content-addressed, hierarchical tag-based transaction indexer that consumes Teranode's Kafka queues and produces block-partitioned index files suitable for streaming subscriptions with boolean query operations.

**Architecture:** Kafka consumer reads subtree and block-final messages from Teranode. Subtree processing fetches transactions from Teranode's HTTP API, parses them to extract tags (key:value pairs with output indices), caches results in an LRU, and builds content-addressed index nodes per subtree. Block processing assembles the block-level hierarchy from already-built subtree indexes. Working space (scratch) and persistent space share the same KVStore interface, with promotion at configurable block depth. All content-addressed hashes use BLAKE3 via multihash wrappers (34-byte keys). The initial tag parser only handles P2PKH address extraction.

**Tech Stack:** Go, Kafka (segmentio/kafka-go), Teranode protobuf messages, go-subtree, go-sdk (transaction parsing, script analysis), BadgerDB (KVStore), SQLite (metadata), BLAKE3 (content addressing), multihash (self-describing hashes)

---

## File Structure

### Dropped (remove from project)
- `p2p/` — replaced by Kafka consumer
- `messages/` — replaced by protobuf deserialization + Teranode HTTP client
- `models/headers.go` — block header tracking handled differently now
- `merkle/` — Teranode handles merkle proofs
- `cmd/checkpeer/` — P2P utility no longer needed
- `run-mainnet.sh`, `run-teratestnet.sh` — old P2P launch scripts

### Kept & Refactored
- `kvstore/store.go` — add `Has` method, fix memory store key encoding
- `kvstore/badger/badger.go` — implement `Has`
- `kvstore/memory/memory.go` — implement `Has`, fix key encoding to use `string(key)` not `hex.EncodeToString`
- `indexnode/indexnode.go` — reconcile API, add `ScanPrefix` method for prefix queries
- `cache/cache.go` — update `IndexTerm` to include `Vouts []uint32`
- `cache/memory/memory.go` — update LRU type parameters
- `multihash/hash.go` — keep as-is
- `treebuilder/builder.go` — update interface for new hierarchy
- `treebuilder/implementation.go` — rewrite to build new hierarchy using multihash keys
- `txindexer/indexer.go` — update `IndexResult` to include vouts
- `processor/processor.go` — rewrite as subtree/block processor
- `metadata/sqlite/sqlite.go` — new schema for working space lifecycle

### New Files
- `kafka/consumer.go` — Kafka consumer for subtree + block-final topics
- `kafka/consumer_test.go` — unit tests
- `teranode/client.go` — HTTP client for fetching subtree data and transactions
- `teranode/client_test.go` — unit tests with mock server
- `txindexer/p2pkh.go` — P2PKH address indexer implementation
- `txindexer/p2pkh_test.go` — tests
- `store/dual.go` — dual-store (working + persistent) with read-through and promote
- `store/dual_test.go` — tests
- `cmd/indexer/main.go` — rewrite: Kafka-driven main loop
- `indexnode/leaf.go` — varint-encoded leaf entry serialization (txid + position + vouts)
- `indexnode/leaf_test.go` — tests

---

## Task 1: Clean Up Dropped Code

**Files:**
- Delete: `p2p/`, `messages/`, `models/`, `merkle/`, `cmd/checkpeer/`
- Delete: `run-mainnet.sh`, `run-teratestnet.sh`, `peer_cache.json`
- Stub: `processor/processor.go`, `cmd/indexer/main.go`
- Modify: `go.mod`

- [ ] **Step 1: Delete dropped directories and files**

```bash
rm -rf p2p/ messages/ models/ merkle/ cmd/checkpeer/
rm -f run-mainnet.sh run-teratestnet.sh peer_cache.json
```

- [ ] **Step 2: Stub processor and main to maintain compilability**

Replace `processor/processor.go` with:

```go
package processor

// Processor handles subtree and block processing.
// TODO: Implement in Task 12.
type Processor struct{}
```

Replace `cmd/indexer/main.go` with:

```go
package main

func main() {
	// TODO: Implement Kafka-driven main loop in Task 13.
	panic("not yet implemented")
}
```

- [ ] **Step 3: Add go-subtree dependency and tidy**

```bash
go get github.com/bsv-blockchain/go-subtree@v1.2.0
go mod tidy
```

Verify the Teranode replace directive in go.mod points to the correct local path. The current directive is:
```
replace github.com/bsv-blockchain/teranode => ../arcade/ref/teranode
```
Confirm this path exists. If not, update to `../1sat/teranode`.

- [ ] **Step 4: Verify compilation**

```bash
go build ./...
```

Expected: PASS (all packages compile with stubs)

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "remove P2P, messages, merkle, and header tracking code — replaced by Kafka + Teranode"
```

---

## Task 2: Update KVStore Interface and Fix Memory Store

**Files:**
- Modify: `kvstore/store.go`
- Modify: `kvstore/badger/badger.go`
- Modify: `kvstore/memory/memory.go`
- Create: `kvstore/memory/memory_test.go`

The memory store currently uses `hex.EncodeToString(key)` as its internal map key, which doubles key size for no reason. All hashes are fixed-length `[]byte` — use `string(key)` (raw byte cast, zero-allocation, comparable) instead.

- [ ] **Step 1: Write tests**

Create `kvstore/memory/memory_test.go`:

```go
package memory

import (
	"context"
	"testing"
)

func TestPutGetDelete(t *testing.T) {
	store := New()
	ctx := context.Background()

	key := []byte{0x01, 0x02, 0x03}
	value := []byte("test-value")

	if err := store.Put(ctx, key, value); err != nil {
		t.Fatalf("Put error: %v", err)
	}

	got, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if string(got) != string(value) {
		t.Errorf("Get: got %q, want %q", got, value)
	}
}

func TestHas(t *testing.T) {
	store := New()
	ctx := context.Background()

	key := []byte{0xAB, 0xCD}
	value := []byte("data")

	exists, err := store.Has(ctx, key)
	if err != nil {
		t.Fatalf("Has error: %v", err)
	}
	if exists {
		t.Fatal("expected key to not exist")
	}

	if err := store.Put(ctx, key, value); err != nil {
		t.Fatalf("Put error: %v", err)
	}

	exists, err = store.Has(ctx, key)
	if err != nil {
		t.Fatalf("Has error: %v", err)
	}
	if !exists {
		t.Fatal("expected key to exist after Put")
	}

	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	exists, err = store.Has(ctx, key)
	if err != nil {
		t.Fatalf("Has error: %v", err)
	}
	if exists {
		t.Fatal("expected key to not exist after Delete")
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
go test ./kvstore/memory/ -v
```

Expected: FAIL — `Has` not defined

- [ ] **Step 3: Add Has to interface, fix memory store key encoding, implement Has**

In `kvstore/store.go`, add to interface:
```go
Has(ctx context.Context, key []byte) (bool, error)
```

Rewrite `kvstore/memory/memory.go` — replace `hex.EncodeToString(key)` with `string(key)` in all methods, add `Has`:

```go
package memory

import (
	"context"
	"sync"
)

type Store struct {
	data sync.Map
}

func New() *Store {
	return &Store{}
}

func (s *Store) Put(_ context.Context, key []byte, value []byte) error {
	s.data.Store(string(key), value)
	return nil
}

func (s *Store) Get(_ context.Context, key []byte) ([]byte, error) {
	val, ok := s.data.Load(string(key))
	if !ok {
		return nil, nil
	}
	return val.([]byte), nil
}

func (s *Store) Has(_ context.Context, key []byte) (bool, error) {
	_, ok := s.data.Load(string(key))
	return ok, nil
}

func (s *Store) Delete(_ context.Context, key []byte) error {
	s.data.Delete(string(key))
	return nil
}

func (s *Store) Close() error {
	return nil
}
```

In `kvstore/badger/badger.go`, add `Has`:
```go
func (b *BadgerStore) Has(_ context.Context, key []byte) (bool, error) {
	err := b.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(key)
		return err
	})
	if err == badger.ErrKeyNotFound {
		return false, nil
	}
	return err == nil, err
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./kvstore/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add kvstore/
git commit -m "add Has to KVStore, fix memory store to use raw byte keys instead of hex encoding"
```

---

## Task 3: Update IndexResult and Cache Types

**Files:**
- Modify: `txindexer/indexer.go`
- Modify: `txindexer/noop.go`
- Modify: `cache/cache.go`
- Modify: `cache/memory/memory.go`

- [ ] **Step 1: Update IndexResult to include vouts**

In `txindexer/indexer.go`:

```go
type IndexResult struct {
	Key   string   // tag key (e.g. "address", "output_type")
	Value string   // tag value (e.g. "1A1zP1...", "bsv21")
	Vouts []uint32 // which outputs contained this tag
}

type TransactionContext struct {
	TxID  []byte // 32 bytes
	RawTx []byte
}
```

Remove `BlockHeight`, `SubtreeRoot`, `SubtreeIndex` from `TransactionContext` — those are known at processing time, not parsing time.

Update `MultiIndexer.Index` and `NoopIndexer.Index` signatures to match.

- [ ] **Step 2: Update IndexTerm in cache**

In `cache/cache.go`:

```go
type IndexTerm struct {
	Key   string
	Value string
	Vouts []uint32
}

type TxID = [32]byte

type IndexTermCache interface {
	Get(txid TxID) ([]IndexTerm, bool)
	Put(txid TxID, terms []IndexTerm) error
	Delete(txid TxID) error
	Clear() error
}
```

- [ ] **Step 3: Update memory cache LRU type parameters**

In `cache/memory/memory.go`, update the LRU instantiation to use `cache.TxID` as key type. Since `cache.TxID = [32]byte` and the old `kvstore.Hash = chainhash.Hash = [32]byte`, this is the same underlying type.

- [ ] **Step 4: Verify compilation**

```bash
go build ./cache/... ./txindexer/...
```

- [ ] **Step 5: Commit**

```bash
git add cache/ txindexer/
git commit -m "update IndexResult and IndexTerm to include vouts, simplify TransactionContext"
```

---

## Task 4: Leaf Entry Serialization

**Files:**
- Create: `indexnode/leaf.go`
- Create: `indexnode/leaf_test.go`

Terminal node entry format: `txid(32) | subtree_position(varint) | vout_count(varint) | vout₀(varint) | ...`

- [ ] **Step 1: Write tests**

Create `indexnode/leaf_test.go`:

```go
package indexnode

import (
	"bytes"
	"testing"
)

func TestLeafEntryMarshalUnmarshal(t *testing.T) {
	txid := make([]byte, 32)
	txid[0] = 0xAB
	entry := LeafEntry{
		TxID:            txid,
		SubtreePosition: 42,
		Vouts:           []uint32{0, 3, 7},
	}

	data := entry.Marshal()

	decoded, bytesRead, err := UnmarshalLeafEntry(data)
	if err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if bytesRead != len(data) {
		t.Fatalf("bytes read: got %d, want %d", bytesRead, len(data))
	}
	if !bytes.Equal(decoded.TxID, entry.TxID) {
		t.Error("txid mismatch")
	}
	if decoded.SubtreePosition != 42 {
		t.Errorf("position: got %d, want 42", decoded.SubtreePosition)
	}
	if len(decoded.Vouts) != 3 || decoded.Vouts[0] != 0 || decoded.Vouts[1] != 3 || decoded.Vouts[2] != 7 {
		t.Errorf("vouts mismatch: got %v", decoded.Vouts)
	}
}

func TestLeafEntryListRoundTrip(t *testing.T) {
	entries := []LeafEntry{
		{TxID: make([]byte, 32), SubtreePosition: 0, Vouts: []uint32{0}},
		{TxID: make([]byte, 32), SubtreePosition: 5, Vouts: []uint32{1, 2}},
		{TxID: make([]byte, 32), SubtreePosition: 100, Vouts: []uint32{0, 4, 8, 12}},
	}
	entries[0].TxID[0] = 1
	entries[1].TxID[0] = 2
	entries[2].TxID[0] = 3

	data := MarshalLeafEntryList(entries)

	decoded, err := UnmarshalLeafEntryList(data)
	if err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(decoded) != 3 {
		t.Fatalf("entry count: got %d, want 3", len(decoded))
	}
	if decoded[2].SubtreePosition != 100 {
		t.Errorf("entry 2 position: got %d, want 100", decoded[2].SubtreePosition)
	}
	if len(decoded[2].Vouts) != 4 {
		t.Errorf("entry 2 vouts: got %d, want 4", len(decoded[2].Vouts))
	}
}

func TestLeafEntryCompactSize(t *testing.T) {
	entry := LeafEntry{
		TxID:            make([]byte, 32),
		SubtreePosition: 0,
		Vouts:           []uint32{0},
	}
	data := entry.Marshal()
	// txid(32) + position varint(1) + count varint(1) + vout varint(1) = 35 bytes
	if len(data) != 35 {
		t.Errorf("compact: got %d bytes, want 35", len(data))
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
go test ./indexnode/ -v -run TestLeafEntry
```

- [ ] **Step 3: Implement leaf entry serialization**

Create `indexnode/leaf.go`:

```go
package indexnode

import (
	"encoding/binary"
	"fmt"
)

type LeafEntry struct {
	TxID            []byte   // 32 bytes
	SubtreePosition uint64
	Vouts           []uint32
}

func (e *LeafEntry) Marshal() []byte {
	buf := make([]byte, 32+binary.MaxVarintLen64*(2+len(e.Vouts)))
	offset := copy(buf, e.TxID[:32])
	offset += binary.PutUvarint(buf[offset:], e.SubtreePosition)
	offset += binary.PutUvarint(buf[offset:], uint64(len(e.Vouts)))
	for _, v := range e.Vouts {
		offset += binary.PutUvarint(buf[offset:], uint64(v))
	}
	return buf[:offset]
}

func UnmarshalLeafEntry(data []byte) (LeafEntry, int, error) {
	if len(data) < 33 {
		return LeafEntry{}, 0, fmt.Errorf("data too short: %d bytes", len(data))
	}

	var entry LeafEntry
	entry.TxID = make([]byte, 32)
	copy(entry.TxID, data[:32])
	offset := 32

	pos, n := binary.Uvarint(data[offset:])
	if n <= 0 {
		return LeafEntry{}, 0, fmt.Errorf("invalid subtree_position varint")
	}
	entry.SubtreePosition = pos
	offset += n

	count, n := binary.Uvarint(data[offset:])
	if n <= 0 {
		return LeafEntry{}, 0, fmt.Errorf("invalid vout_count varint")
	}
	offset += n

	entry.Vouts = make([]uint32, count)
	for i := uint64(0); i < count; i++ {
		v, n := binary.Uvarint(data[offset:])
		if n <= 0 {
			return LeafEntry{}, 0, fmt.Errorf("invalid vout varint at index %d", i)
		}
		entry.Vouts[i] = uint32(v)
		offset += n
	}

	return entry, offset, nil
}

func MarshalLeafEntryList(entries []LeafEntry) []byte {
	countBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(countBuf, uint32(len(entries)))
	buf := countBuf
	for i := range entries {
		buf = append(buf, entries[i].Marshal()...)
	}
	return buf
}

func UnmarshalLeafEntryList(data []byte) ([]LeafEntry, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("data too short for count: %d bytes", len(data))
	}
	count := binary.BigEndian.Uint32(data[:4])
	offset := 4
	entries := make([]LeafEntry, count)
	for i := uint32(0); i < count; i++ {
		entry, n, err := UnmarshalLeafEntry(data[offset:])
		if err != nil {
			return nil, fmt.Errorf("entry %d: %w", i, err)
		}
		entries[i] = entry
		offset += n
	}
	return entries, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./indexnode/ -v -run TestLeafEntry
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add indexnode/leaf.go indexnode/leaf_test.go
git commit -m "add varint-encoded leaf entry format for txid + position + vouts"
```

---

## Task 5: Reconcile IndexNode API and Add Prefix Scanning

The current `indexnode.go` was refactored but tests and treebuilder still reference the old API. Reconcile everything and add `ScanPrefix` for subscription query support.

**Files:**
- Modify: `indexnode/indexnode.go` — add constructors, add `ScanPrefix`, add `ScanRange`
- Rewrite: `indexnode/indexnode_test.go`
- Rewrite: `indexnode/indexnode_fixed_test.go`

- [ ] **Step 1: Add convenience constructors and scan methods**

The tree builder needs two node types:
1. **Tag key/value nodes**: variable-length string keys stored in data section, sorted by data. `HasData=true, SortByData=true, KeySize=0, ValueSize=32` (value is a 32-byte child hash).
2. **Fixed key nodes**: fixed-width hash keys. `HasData=false, KeySize=32, ValueSize=32`.
3. **Array nodes**: no keys, just ordered values (e.g. subtree hash list). `KeySize=0, ValueSize=32, HasData=false`.

Add to `indexnode.go`:

```go
// NewTagNode creates a node for variable-length string keys (tag keys or tag values).
// Keys are stored in the data section, sorted by data. Value is a 32-byte hash.
func NewTagNode() *IndexNode {
	return NewIndexNode(0, 32, true, true, false)
}

// NewFixedKeyNode creates a node for fixed-width keys (e.g. 32-byte hashes).
func NewFixedKeyNode(keySize uint16) *IndexNode {
	return NewIndexNode(keySize, 32, false, false, false)
}

// NewArrayNode creates a node for ordered values with no keys (index-based access).
func NewArrayNode(valueSize uint8) *IndexNode {
	return NewIndexNode(0, valueSize, false, false, false)
}

// ScanPrefix finds all entries whose key (or data if SortByData) starts with the given prefix.
// Returns matching values.
func (n *IndexNode) ScanPrefix(prefix []byte) [][]byte {
	var results [][]byte

	getKey := func(i int) []byte {
		if n.SortByData {
			return n.getDataAt(n.Entries[i].Offset)
		}
		return n.Entries[i].Key
	}

	// Binary search to find first entry >= prefix
	start := sort.Search(len(n.Entries), func(i int) bool {
		return bytes.Compare(getKey(i), prefix) >= 0
	})

	// Iterate forward while prefix matches
	for i := start; i < len(n.Entries); i++ {
		key := getKey(i)
		if len(key) < len(prefix) || !bytes.Equal(key[:len(prefix)], prefix) {
			break
		}
		results = append(results, n.Entries[i].Value)
	}
	return results
}

// ScanRange returns all values for entries whose key (or data) is >= start and < end.
func (n *IndexNode) ScanRange(start, end []byte) [][]byte {
	var results [][]byte

	getKey := func(i int) []byte {
		if n.SortByData {
			return n.getDataAt(n.Entries[i].Offset)
		}
		return n.Entries[i].Key
	}

	idx := sort.Search(len(n.Entries), func(i int) bool {
		return bytes.Compare(getKey(i), start) >= 0
	})

	for i := idx; i < len(n.Entries); i++ {
		key := getKey(i)
		if end != nil && bytes.Compare(key, end) >= 0 {
			break
		}
		results = append(results, n.Entries[i].Value)
	}
	return results
}
```

Note: add `"sort"` and `"bytes"` to imports if not already present.

- [ ] **Step 2: Rewrite tests to use current API**

Rewrite `indexnode_test.go` — test variable-key nodes using `NewTagNode()` (HasData+SortByData mode). Test `Find`, `FindByData`, `ScanPrefix`, and `ScanRange`.

Rewrite `indexnode_fixed_test.go` — test fixed-key nodes using `NewFixedKeyNode(32)`. Test `Find` and size comparisons.

Key test cases for `ScanPrefix`:
```go
func TestScanPrefix(t *testing.T) {
	node := NewTagNode()
	// Build data section and add entries for:
	// "address:1A1zP...", "address:1BvBM...", "bsv21:tokenXYZ", "bsv21:tokenABC"
	// ScanPrefix("address:") should return 2 results
	// ScanPrefix("bsv21:token") should return 2 results
	// ScanPrefix("nonexistent") should return 0 results
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./indexnode/ -v
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add indexnode/
git commit -m "reconcile IndexNode API, add ScanPrefix and ScanRange for subscription queries"
```

---

## Task 6: Teranode HTTP Client

**Files:**
- Create: `teranode/client.go`
- Create: `teranode/client_test.go`

- [ ] **Step 1: Write tests with mock HTTP server**

```go
package teranode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchTransaction(t *testing.T) {
	fakeTx := []byte{0x01, 0x00, 0x00, 0x00, 0x00}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/tx/abcd1234" {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(fakeTx)
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	data, err := client.FetchTransaction(context.Background(), "abcd1234")
	if err != nil {
		t.Fatalf("FetchTransaction error: %v", err)
	}
	if len(data) != len(fakeTx) {
		t.Errorf("got %d bytes, want %d", len(data), len(fakeTx))
	}
}

func TestFetchTransactionNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.FetchTransaction(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
go test ./teranode/ -v
```

- [ ] **Step 3: Implement client**

Create `teranode/client.go`:

```go
package teranode

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"

	subtreepkg "github.com/bsv-blockchain/go-subtree"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{baseURL: baseURL, httpClient: &http.Client{}}
}

func (c *Client) FetchTransaction(ctx context.Context, txidHex string) ([]byte, error) {
	url := fmt.Sprintf("%s/api/v1/tx/%s", c.baseURL, txidHex)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetch tx %s: status %d", txidHex, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (c *Client) FetchSubtreeData(ctx context.Context, url string) (*subtreepkg.Subtree, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetch subtree: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return subtreepkg.NewSubtreeFromBytes(data)
}

func TxIDToHex(txid []byte) string {
	reversed := make([]byte, 32)
	for i, b := range txid {
		reversed[31-i] = b
	}
	return hex.EncodeToString(reversed)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./teranode/ -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add teranode/
git commit -m "add Teranode HTTP client for transaction and subtree fetching"
```

---

## Task 7: P2PKH Address Indexer

**Files:**
- Create: `txindexer/p2pkh.go`
- Create: `txindexer/p2pkh_test.go`

- [ ] **Step 1: Write test with helper**

Create `txindexer/p2pkh_test.go`:

```go
package txindexer

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
)

func buildTestP2PKHTx(t *testing.T, lockingScripts ...[]byte) []byte {
	t.Helper()
	tx := transaction.NewTransaction()
	// Add a dummy coinbase-style input
	tx.AddInput(&transaction.TransactionInput{
		SourceTXID:       make([]byte, 32),
		SourceTxOutIndex: 0xffffffff,
		SequenceNumber:   0xffffffff,
	})
	for _, ls := range lockingScripts {
		s := script.Script(ls)
		tx.AddOutput(&transaction.TransactionOutput{
			Satoshis:      1000,
			LockingScript: &s,
		})
	}
	return tx.Bytes()
}

func TestP2PKHIndexer(t *testing.T) {
	indexer := NewP2PKHIndexer()

	if indexer.Name() != "P2PKH" {
		t.Errorf("name: got %s, want P2PKH", indexer.Name())
	}

	// OP_DUP OP_HASH160 <20-byte-pkh> OP_EQUALVERIFY OP_CHECKSIG
	pkhHex := "62e907b15cbf27d5425399ebf6f0fb50ebb88f18"
	lockingScript, _ := hex.DecodeString("76a914" + pkhHex + "88ac")

	rawTx := buildTestP2PKHTx(t, lockingScript)

	results, err := indexer.Index(context.Background(), &TransactionContext{
		TxID:  make([]byte, 32),
		RawTx: rawTx,
	})
	if err != nil {
		t.Fatalf("Index error: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}

	if results[0].Key != "address" {
		t.Errorf("key: got %s, want address", results[0].Key)
	}
	if len(results[0].Vouts) != 1 || results[0].Vouts[0] != 0 {
		t.Errorf("vouts: got %v, want [0]", results[0].Vouts)
	}
}

func TestP2PKHMultipleOutputsSameAddress(t *testing.T) {
	indexer := NewP2PKHIndexer()

	lockingScript, _ := hex.DecodeString("76a91462e907b15cbf27d5425399ebf6f0fb50ebb88f1888ac")
	rawTx := buildTestP2PKHTx(t, lockingScript, lockingScript)

	results, err := indexer.Index(context.Background(), &TransactionContext{
		TxID:  make([]byte, 32),
		RawTx: rawTx,
	})
	if err != nil {
		t.Fatalf("Index error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result (grouped), got %d", len(results))
	}
	if len(results[0].Vouts) != 2 {
		t.Errorf("expected 2 vouts, got %d", len(results[0].Vouts))
	}
}

func TestP2PKHNonP2PKHOutput(t *testing.T) {
	indexer := NewP2PKHIndexer()

	// OP_RETURN script (not P2PKH)
	opReturn, _ := hex.DecodeString("006a0568656c6c6f")
	rawTx := buildTestP2PKHTx(t, opReturn)

	results, err := indexer.Index(context.Background(), &TransactionContext{
		TxID:  make([]byte, 32),
		RawTx: rawTx,
	})
	if err != nil {
		t.Fatalf("Index error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected no results for OP_RETURN, got %d", len(results))
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
go test ./txindexer/ -v -run TestP2PKH
```

- [ ] **Step 3: Implement P2PKH indexer**

Create `txindexer/p2pkh.go`:

```go
package txindexer

import (
	"context"

	"github.com/bsv-blockchain/go-sdk/transaction"
)

type P2PKHIndexer struct{}

func NewP2PKHIndexer() *P2PKHIndexer { return &P2PKHIndexer{} }

func (p *P2PKHIndexer) Name() string { return "P2PKH" }

func (p *P2PKHIndexer) Index(_ context.Context, txCtx *TransactionContext) ([]*IndexResult, error) {
	tx, err := transaction.NewTransactionFromBytes(txCtx.RawTx)
	if err != nil {
		return nil, err
	}

	addrVouts := make(map[string][]uint32)
	for i, output := range tx.Outputs {
		if output.LockingScript.IsP2PKH() {
			addrs, err := output.LockingScript.Addresses()
			if err != nil || len(addrs) == 0 {
				continue
			}
			addrVouts[addrs[0]] = append(addrVouts[addrs[0]], uint32(i))
		}
	}

	results := make([]*IndexResult, 0, len(addrVouts))
	for addr, vouts := range addrVouts {
		results = append(results, &IndexResult{
			Key:   "address",
			Value: addr,
			Vouts: vouts,
		})
	}
	return results, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./txindexer/ -v -run TestP2PKH
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add txindexer/p2pkh.go txindexer/p2pkh_test.go
git commit -m "add P2PKH address indexer with output index tracking"
```

---

## Task 8: Kafka Consumer

**Files:**
- Create: `kafka/consumer.go`
- Create: `kafka/consumer_test.go`
- Modify: `go.mod`

- [ ] **Step 1: Add kafka-go dependency**

```bash
go get github.com/segmentio/kafka-go
```

- [ ] **Step 2: Write consumer with protobuf deserialization**

Import Teranode's protobuf types from `github.com/bsv-blockchain/teranode/util/kafka/kafka_message`. Verify the import works:

```bash
go list github.com/bsv-blockchain/teranode/util/kafka/kafka_message
```

If the import fails, copy the `.proto` file to `kafka/proto/` and generate locally.

Create `kafka/consumer.go`:

```go
package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	kafkamessage "github.com/bsv-blockchain/teranode/util/kafka/kafka_message"
	"google.golang.org/protobuf/proto"
)

type SubtreeHandler func(ctx context.Context, subtreeHash string, fetchURL string) error
type BlockHandler func(ctx context.Context, height uint32, headerBytes []byte, subtreeHashes [][]byte, txCount uint64) error

type Consumer struct {
	brokers        []string
	groupID        string
	subtreeHandler SubtreeHandler
	blockHandler   BlockHandler
	logger         *slog.Logger
}

func NewConsumer(brokers []string, groupID string, subtreeHandler SubtreeHandler, blockHandler BlockHandler, logger *slog.Logger) *Consumer {
	return &Consumer{
		brokers:        brokers,
		groupID:        groupID,
		subtreeHandler: subtreeHandler,
		blockHandler:   blockHandler,
		logger:         logger,
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	errCh := make(chan error, 2)

	go func() { errCh <- c.consumeSubtrees(ctx) }()
	go func() { errCh <- c.consumeBlocks(ctx) }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Consumer) consumeSubtrees(ctx context.Context) error {
	r := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:  c.brokers,
		GroupID:  c.groupID + "-subtrees",
		Topic:    "subtrees",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer r.Close()

	for {
		msg, err := r.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("fetch subtree message: %w", err)
		}

		var subtreeMsg kafkamessage.KafkaSubtreeTopicMessage
		if err := proto.Unmarshal(msg.Value, &subtreeMsg); err != nil {
			c.logger.Error("unmarshal subtree message", "error", err)
			r.CommitMessages(ctx, msg)
			continue
		}

		if err := c.subtreeHandler(ctx, subtreeMsg.Hash, subtreeMsg.URL); err != nil {
			c.logger.Error("process subtree", "hash", subtreeMsg.Hash, "error", err)
			// Don't commit — will retry on next fetch
			continue
		}

		r.CommitMessages(ctx, msg)
	}
}

func (c *Consumer) consumeBlocks(ctx context.Context) error {
	r := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:  c.brokers,
		GroupID:  c.groupID + "-blocks",
		Topic:    "blocks-final",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer r.Close()

	for {
		msg, err := r.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("fetch block message: %w", err)
		}

		var blockMsg kafkamessage.KafkaBlocksFinalTopicMessage
		if err := proto.Unmarshal(msg.Value, &blockMsg); err != nil {
			c.logger.Error("unmarshal block message", "error", err)
			r.CommitMessages(ctx, msg)
			continue
		}

		if err := c.blockHandler(ctx, blockMsg.Height, blockMsg.Header, blockMsg.SubtreeHashes, blockMsg.TransactionCount); err != nil {
			c.logger.Error("process block", "height", blockMsg.Height, "error", err)
			continue
		}

		r.CommitMessages(ctx, msg)
	}
}
```

- [ ] **Step 3: Write unit test**

Create `kafka/consumer_test.go` that tests protobuf deserialization of both message types using `proto.Marshal` to create test messages, then verifying the handler callbacks receive correct values.

- [ ] **Step 4: Run tests**

```bash
go test ./kafka/ -v
```

- [ ] **Step 5: Commit**

```bash
git add kafka/ go.mod go.sum
git commit -m "add Kafka consumer for Teranode subtree and block-final topics"
```

---

## Task 9: Dual Store (Working + Persistent)

**Files:**
- Create: `store/dual.go`
- Create: `store/dual_test.go`

The processor holds a `*DualStore` (concrete type, not `KVStore` interface) so it can call `Promote`. Components that only need read/write use the `KVStore` interface.

- [ ] **Step 1: Write tests**

```go
package store

import (
	"context"
	"testing"

	"github.com/shruggr/inspiration/kvstore/memory"
)

func TestReadThrough(t *testing.T) {
	working := memory.New()
	persistent := memory.New()
	dual := NewDualStore(working, persistent)
	ctx := context.Background()

	key := []byte("test-key")
	value := []byte("test-value")

	persistent.Put(ctx, key, value)

	got, err := dual.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if string(got) != string(value) {
		t.Errorf("got %q, want %q", got, value)
	}
}

func TestWriteToWorking(t *testing.T) {
	working := memory.New()
	persistent := memory.New()
	dual := NewDualStore(working, persistent)
	ctx := context.Background()

	key := []byte("k")
	value := []byte("v")

	dual.Put(ctx, key, value)

	got, _ := working.Get(ctx, key)
	if string(got) != "v" {
		t.Error("expected value in working")
	}
	got, _ = persistent.Get(ctx, key)
	if got != nil {
		t.Error("expected NOT in persistent")
	}
}

func TestPromote(t *testing.T) {
	working := memory.New()
	persistent := memory.New()
	dual := NewDualStore(working, persistent)
	ctx := context.Background()

	key := []byte("k")
	value := []byte("v")
	working.Put(ctx, key, value)

	if err := dual.Promote(ctx, key); err != nil {
		t.Fatalf("Promote error: %v", err)
	}

	got, _ := persistent.Get(ctx, key)
	if string(got) != "v" {
		t.Error("expected value in persistent after Promote")
	}
}

func TestHasReadThrough(t *testing.T) {
	working := memory.New()
	persistent := memory.New()
	dual := NewDualStore(working, persistent)
	ctx := context.Background()

	key := []byte("k")
	persistent.Put(ctx, key, []byte("v"))

	exists, err := dual.Has(ctx, key)
	if err != nil {
		t.Fatalf("Has error: %v", err)
	}
	if !exists {
		t.Error("expected Has=true via read-through")
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
go test ./store/ -v
```

- [ ] **Step 3: Implement DualStore**

```go
package store

import (
	"context"
	"fmt"

	"github.com/shruggr/inspiration/kvstore"
)

type DualStore struct {
	working    kvstore.KVStore
	persistent kvstore.KVStore
}

func NewDualStore(working, persistent kvstore.KVStore) *DualStore {
	return &DualStore{working: working, persistent: persistent}
}

func (d *DualStore) Put(ctx context.Context, key, value []byte) error {
	return d.working.Put(ctx, key, value)
}

func (d *DualStore) Get(ctx context.Context, key []byte) ([]byte, error) {
	val, err := d.working.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if val != nil {
		return val, nil
	}
	return d.persistent.Get(ctx, key)
}

func (d *DualStore) Has(ctx context.Context, key []byte) (bool, error) {
	ok, err := d.working.Has(ctx, key)
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}
	return d.persistent.Has(ctx, key)
}

func (d *DualStore) Delete(ctx context.Context, key []byte) error {
	return d.working.Delete(ctx, key)
}

func (d *DualStore) Close() error {
	wErr := d.working.Close()
	pErr := d.persistent.Close()
	if wErr != nil {
		return fmt.Errorf("working close: %w", wErr)
	}
	if pErr != nil {
		return fmt.Errorf("persistent close: %w", pErr)
	}
	return nil
}

func (d *DualStore) Promote(ctx context.Context, key []byte) error {
	val, err := d.working.Get(ctx, key)
	if err != nil {
		return err
	}
	if val == nil {
		return fmt.Errorf("key not found in working space")
	}
	return d.persistent.Put(ctx, key, val)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./store/ -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add store/
git commit -m "add DualStore with read-through, working/persistent split, and Promote"
```

---

## Task 10: Metadata Store Update

**Files:**
- Modify: `metadata/store.go`
- Rewrite: `metadata/sqlite/sqlite.go`
- Rewrite: `metadata/sqlite/sqlite_test.go`

- [ ] **Step 1: Define new schema and interface**

Schema:

```sql
CREATE TABLE blocks (
    height          INTEGER NOT NULL,
    block_hash      BLOB NOT NULL PRIMARY KEY,
    header          BLOB NOT NULL,
    tx_count        INTEGER NOT NULL,
    subtree_count   INTEGER NOT NULL,
    index_root      BLOB,
    status          TEXT NOT NULL DEFAULT 'pending',
    created_at      INTEGER DEFAULT (strftime('%s', 'now')),
    promoted_at     INTEGER
);
CREATE INDEX idx_blocks_height ON blocks(height);
CREATE INDEX idx_blocks_status ON blocks(status);

CREATE TABLE block_subtrees (
    block_hash      BLOB NOT NULL,
    subtree_index   INTEGER NOT NULL,
    subtree_hash    BLOB NOT NULL,
    PRIMARY KEY (block_hash, subtree_index),
    FOREIGN KEY (block_hash) REFERENCES blocks(block_hash) ON DELETE CASCADE
);

CREATE TABLE subtrees (
    subtree_hash    BLOB PRIMARY KEY,
    index_root      BLOB NOT NULL,
    tx_count        INTEGER NOT NULL,
    received_at     INTEGER DEFAULT (strftime('%s', 'now')),
    block_hash      BLOB,
    promoted        INTEGER DEFAULT 0
);
```

Interface:

```go
type Store interface {
    InsertSubtree(ctx context.Context, hash, indexRoot []byte, txCount uint32) error
    InsertBlock(ctx context.Context, height uint32, blockHash, header []byte, txCount uint64, subtreeHashes [][]byte) error
    GetBlockSubtrees(ctx context.Context, blockHash []byte) ([][]byte, error)
    GetSubtreeIndexRoot(ctx context.Context, subtreeHash []byte) ([]byte, error)
    SubtreeExists(ctx context.Context, subtreeHash []byte) (bool, error)
    PromoteBlock(ctx context.Context, blockHash []byte) error
    OrphanBlock(ctx context.Context, blockHash []byte) error
    GetUnpromotedBlocks(ctx context.Context, deeperThanHeight uint32) ([][]byte, error)
    Close() error
}
```

- [ ] **Step 2: Write tests**

Test: insert subtree, insert block, query subtrees in order, promote, orphan, query unfinalized candidates.

- [ ] **Step 3: Implement SQLite store**

- [ ] **Step 4: Run tests**

```bash
go test ./metadata/... -v
```

- [ ] **Step 5: Commit**

```bash
git add metadata/
git commit -m "update metadata store for subtree lifecycle with promotion and orphan tracking"
```

---

## Task 11: Tree Builder Rewrite

**Files:**
- Modify: `treebuilder/builder.go`
- Rewrite: `treebuilder/implementation.go`
- Rewrite: `treebuilder/implementation_test.go`

All content-addressed hashing uses multihash wrappers (34-byte keys), not raw `blake3.Sum256`.

- [ ] **Step 1: Update Builder interface**

```go
type Builder interface {
    BuildSubtreeIndex(ctx context.Context, entries []TaggedTransaction) (multihash.IndexHash, error)
}

type TaggedTransaction struct {
    TxID            [32]byte
    SubtreePosition uint64
    Tags            []Tag
}

type Tag struct {
    Key   string
    Value string
    Vouts []uint32
}
```

- [ ] **Step 2: Write tests**

```go
func TestBuildSubtreeIndex(t *testing.T) {
    store := memory.New()
    builder := NewBuilder(store)

    txs := []TaggedTransaction{
        {
            TxID:            [32]byte{1},
            SubtreePosition: 0,
            Tags: []Tag{
                {Key: "address", Value: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", Vouts: []uint32{0}},
                {Key: "output_type", Value: "p2pkh", Vouts: []uint32{0}},
            },
        },
        {
            TxID:            [32]byte{2},
            SubtreePosition: 1,
            Tags: []Tag{
                {Key: "address", Value: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", Vouts: []uint32{0, 2}},
                {Key: "address", Value: "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2", Vouts: []uint32{1}},
            },
        },
    }

    rootHash, err := builder.BuildSubtreeIndex(context.Background(), txs)
    if err != nil {
        t.Fatalf("BuildSubtreeIndex: %v", err)
    }

    // Verify root node exists in store
    rootBytes, err := store.Get(context.Background(), rootHash.Bytes())
    if err != nil || rootBytes == nil {
        t.Fatal("root node not in store")
    }

    // Verify determinism — same input produces same hash
    rootHash2, _ := builder.BuildSubtreeIndex(context.Background(), txs)
    if !bytes.Equal(rootHash.Bytes(), rootHash2.Bytes()) {
        t.Error("non-deterministic hash")
    }
}
```

- [ ] **Step 3: Implement**

Build three-level tree using `NewTagNode()` for tag key and tag value levels:

1. Group by tag key → tag value → []LeafEntry (with SubtreePosition and Vouts)
2. For each tag value: sort leaf entries by SubtreePosition, marshal as LeafEntryList, multihash, store
3. For each tag key: build tag value IndexNode (NewTagNode), entries sorted by value string, multihash, store
4. Build root IndexNode (NewTagNode), entries sorted by key string, multihash, store
5. Return root multihash

- [ ] **Step 4: Run tests**

```bash
go test ./treebuilder/ -v
```

- [ ] **Step 5: Commit**

```bash
git add treebuilder/
git commit -m "rewrite tree builder for tag hierarchy with multihash content addressing"
```

---

## Task 12: Subtree and Block Processor

**Files:**
- Rewrite: `processor/processor.go`
- Create: `processor/processor_test.go`

- [ ] **Step 1: Define Processor**

```go
type Processor struct {
    store      *store.DualStore    // concrete type for Promote access
    spendStore kvstore.KVStore     // flat outpoint→spending_txid lookup (persistent, no promotion)
    cache      cache.IndexTermCache
    indexer    txindexer.Indexer
    client     *teranode.Client
    builder    treebuilder.Builder
    metadata   metadata.Store
    logger     *slog.Logger
}
```

The `spendStore` is a separate, persistent KVStore. Key: `outpoint(36)` = txid(32) + vout(4 big-endian). Value: `spending_txid(32)`. Written during transaction parsing — for each input, record which outpoint it spends. No working/persistent split, no promotion, no rollback. Last write wins.

- [ ] **Step 2: Write test for ProcessSubtree**

Mock Teranode client, use in-memory stores. Verify:
- Subtree data fetched
- Transactions parsed (cache checked first)
- Index tree built and stored
- Metadata recorded

- [ ] **Step 3: Implement ProcessSubtree**

```
1. Fetch subtree from URL → ordered []Node{Hash, Fee, SizeInBytes}
2. For each node at position i:
   a. Check cache for parsed tags
   b. Miss → fetch raw tx from Teranode HTTP, parse with indexer, cache
   c. Build TaggedTransaction{TxID, SubtreePosition: i, Tags}
   d. Write spend records: for each input in the parsed tx,
      spendStore.Put(ctx, outpoint(32+4), spending_txid(32))
      where outpoint = input's previous txid + previous vout (big-endian uint32)
3. builder.BuildSubtreeIndex(taggedTxs) → index root hash
4. metadata.InsertSubtree(subtreeHash, indexRoot, txCount)
```

- [ ] **Step 4: Write test for ProcessBlock**

Verify:
- All referenced subtrees exist in metadata (verify-before-proceed)
- Block metadata stored with ordered subtree references
- If subtrees missing, return error (caller retries)

- [ ] **Step 5: Implement ProcessBlock**

```
1. For each subtreeHash in ordered list:
   a. Verify metadata.SubtreeExists(subtreeHash)
   b. If any missing, return ErrSubtreeNotReady
2. metadata.InsertBlock(height, blockHash, header, txCount, subtreeHashes)
```

`ErrSubtreeNotReady` signals the Kafka consumer to not commit the message, so it retries.

- [ ] **Step 6: Run tests**

```bash
go test ./processor/ -v
```

- [ ] **Step 7: Commit**

```bash
git add processor/
git commit -m "implement subtree and block processor with cache, tree builder, and retry on missing subtrees"
```

---

## Task 13: Main Entry Point

**Files:**
- Rewrite: `cmd/indexer/main.go`

- [ ] **Step 1: Implement Kafka-driven main loop**

```go
func main() {
    // Flags: --kafka-brokers, --teranode-url, --data-dir, --log-level
    // Init: working store (badger at data-dir/working)
    // Init: persistent store (badger at data-dir/persistent)
    // Init: dual store
    // Init: spend store (badger at data-dir/spends) — separate instance, same KVStore interface
    // Init: metadata store (SQLite at data-dir/metadata.db)
    // Init: LRU cache (configurable size)
    // Init: P2PKH indexer (via MultiIndexer for future expansion)
    // Init: tree builder
    // Init: Teranode client
    // Init: processor (receives dual store + spend store + cache + indexer + builder + metadata + client)
    // Init: Kafka consumer with processor handlers
    // consumer.Run(ctx) with graceful shutdown on SIGINT/SIGTERM
}
```

- [ ] **Step 2: Verify compilation**

```bash
go run ./cmd/indexer --help
```

- [ ] **Step 3: Commit**

```bash
git add cmd/indexer/
git commit -m "Kafka-driven main loop wiring all components together"
```

---

## Task 14: Integration Test

**Files:**
- Create: `test/integration_test.go`

- [ ] **Step 1: Write end-to-end test**

Using in-memory stores and mock Teranode:
1. Build mock subtree with known P2PKH transactions
2. Process subtree through full pipeline
3. Verify index nodes stored with correct hierarchy
4. Process mock block referencing that subtree
5. Walk index: block → subtree → tag "address" → specific address → verify txid + vouts + position
6. Test ScanPrefix: walk "address" tag key node, prefix scan "1A1z" → verify subset

- [ ] **Step 2: Run**

```bash
go test ./test/ -v -run TestIntegration
```

- [ ] **Step 3: Commit**

```bash
git add test/
git commit -m "add integration test covering full indexing pipeline and prefix query"
```

---

## Deferred (not in this plan)

- **Subscription streaming API** — walk blocks in order, stream matching entries to consumers
- **Boolean query execution** — AND/OR/NOT logic combining results across tags (infrastructure exists via ScanPrefix + ScanRange on IndexNode)
- **Promotion worker** — background goroutine monitoring block depth
- **Garbage collection** — orphaned subtree cleanup from working space
- **Additional indexers** — BSV21, OP_RETURN, ordinals, contexts, sub-contexts
- **Kafka consumer group management** — offsets, rebalancing
- **Performance tuning** — concurrent subtree processing, batch writes, cache sizing
- **Mempool support** — real-time transaction streaming before block confirmation

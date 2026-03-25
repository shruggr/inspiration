package processor

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/shruggr/inspiration/cache"
	"github.com/shruggr/inspiration/kvstore"
	"github.com/shruggr/inspiration/metadata"
	"github.com/shruggr/inspiration/multihash"
	"github.com/shruggr/inspiration/store"
	"github.com/shruggr/inspiration/teranode"
	"github.com/shruggr/inspiration/treebuilder"
	"github.com/shruggr/inspiration/txindexer"
)

// --- Mock KVStore ---

type memKVStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newMemKVStore() *memKVStore {
	return &memKVStore{data: make(map[string][]byte)}
}

func (m *memKVStore) Put(_ context.Context, key, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[string(key)] = append([]byte(nil), value...)
	return nil
}

func (m *memKVStore) Get(_ context.Context, key []byte) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[string(key)]
	if !ok {
		return nil, nil
	}
	return v, nil
}

func (m *memKVStore) Delete(_ context.Context, key []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, string(key))
	return nil
}

func (m *memKVStore) Has(_ context.Context, key []byte) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.data[string(key)]
	return ok, nil
}

func (m *memKVStore) Close() error { return nil }

// --- Mock IndexTermCache ---

type memCache struct {
	mu   sync.Mutex
	data map[cache.TxID][]cache.IndexTerm
}

func newMemCache() *memCache {
	return &memCache{data: make(map[cache.TxID][]cache.IndexTerm)}
}

func (c *memCache) Get(txid cache.TxID) ([]cache.IndexTerm, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.data[txid]
	return v, ok
}

func (c *memCache) Put(txid cache.TxID, terms []cache.IndexTerm) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[txid] = terms
	return nil
}

func (c *memCache) Delete(txid cache.TxID) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, txid)
	return nil
}

func (c *memCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[cache.TxID][]cache.IndexTerm)
	return nil
}

// --- Mock Indexer ---

type mockIndexer struct {
	results map[string][]*txindexer.IndexResult
}

func newMockIndexer() *mockIndexer {
	return &mockIndexer{results: make(map[string][]*txindexer.IndexResult)}
}

func (m *mockIndexer) Index(_ context.Context, tx *txindexer.TransactionContext) ([]*txindexer.IndexResult, error) {
	key := fmt.Sprintf("%x", tx.TxID)
	if r, ok := m.results[key]; ok {
		return r, nil
	}
	return nil, nil
}

func (m *mockIndexer) Name() string { return "mock" }

// --- Mock Builder ---

type mockBuilder struct {
	returnHash multihash.IndexHash
	returnErr  error
	called     bool
}

func (b *mockBuilder) BuildSubtreeIndex(_ context.Context, _ []treebuilder.TaggedTransaction) (multihash.IndexHash, error) {
	b.called = true
	return b.returnHash, b.returnErr
}

// --- Mock Metadata Store ---

type memMetadata struct {
	mu       sync.Mutex
	subtrees map[string]metaSubtree
	blocks   map[string]metaBlock
}

type metaSubtree struct {
	indexRoot []byte
	txCount   uint32
}

type metaBlock struct {
	height        uint32
	header        []byte
	txCount       uint64
	subtreeHashes [][]byte
}

func newMemMetadata() *memMetadata {
	return &memMetadata{
		subtrees: make(map[string]metaSubtree),
		blocks:   make(map[string]metaBlock),
	}
}

func (m *memMetadata) InsertSubtree(_ context.Context, hash, indexRoot []byte, txCount uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subtrees[string(hash)] = metaSubtree{indexRoot: indexRoot, txCount: txCount}
	return nil
}

func (m *memMetadata) InsertBlock(_ context.Context, height uint32, blockHash, header []byte, txCount uint64, subtreeHashes [][]byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blocks[string(blockHash)] = metaBlock{height: height, header: header, txCount: txCount, subtreeHashes: subtreeHashes}
	return nil
}

func (m *memMetadata) GetBlockSubtrees(_ context.Context, blockHash []byte) ([][]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.blocks[string(blockHash)]
	if !ok {
		return nil, nil
	}
	return b.subtreeHashes, nil
}

func (m *memMetadata) GetSubtreeIndexRoot(_ context.Context, subtreeHash []byte) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.subtrees[string(subtreeHash)]
	if !ok {
		return nil, nil
	}
	return s.indexRoot, nil
}

func (m *memMetadata) SubtreeExists(_ context.Context, subtreeHash []byte) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.subtrees[string(subtreeHash)]
	return ok, nil
}

func (m *memMetadata) PromoteBlock(context.Context, []byte) error              { return nil }
func (m *memMetadata) OrphanBlock(context.Context, []byte) error               { return nil }
func (m *memMetadata) GetUnpromotedBlocks(context.Context, uint32) ([][]byte, error) { return nil, nil }
func (m *memMetadata) Close() error                                            { return nil }

// --- Helpers ---

// buildMinimalSubtreeData builds a serialized subtree with the given transaction hashes.
func buildMinimalSubtreeData(txids [][32]byte) []byte {
	numNodes := uint64(len(txids))
	// header: 32 (root hash) + 8 (fees) + 8 (size) + 8 (numLeaves) = 56
	// per node: 32 (hash) + 8 (fee) + 8 (size) = 48
	// trailing: 8 (numConflicting)
	size := 56 + int(numNodes)*48 + 8
	data := make([]byte, size)

	// Root hash: just use first txid or zeros
	if len(txids) > 0 {
		copy(data[0:32], txids[0][:])
	}

	// fees = 0, sizeInBytes = 0 (already zero)
	// numLeaves
	binary.LittleEndian.PutUint64(data[48:56], numNodes)

	offset := 56
	for _, txid := range txids {
		copy(data[offset:offset+32], txid[:])
		// fee and size left as 0
		offset += 48
	}

	// numConflicting = 0 (already zero)

	return data
}

// buildMinimalRawTx builds a minimal valid BSV transaction with one input and one output.
// The input references prevTxID:prevVout.
func buildMinimalRawTx(prevTxID [32]byte, prevVout uint32) []byte {
	var buf []byte

	// version (4 bytes LE)
	buf = binary.LittleEndian.AppendUint32(buf, 1)

	// input count (varint)
	buf = append(buf, 1)

	// prev txid (32 bytes, already in internal byte order)
	buf = append(buf, prevTxID[:]...)

	// prev vout (4 bytes LE)
	buf = binary.LittleEndian.AppendUint32(buf, prevVout)

	// script length (varint) + empty script
	buf = append(buf, 0)

	// sequence (4 bytes)
	buf = append(buf, 0xff, 0xff, 0xff, 0xff)

	// output count (varint)
	buf = append(buf, 1)

	// value (8 bytes LE)
	buf = binary.LittleEndian.AppendUint64(buf, 0)

	// script length (varint) + empty script
	buf = append(buf, 0)

	// locktime (4 bytes)
	buf = binary.LittleEndian.AppendUint32(buf, 0)

	return buf
}

// --- Tests ---

func TestProcessSubtree(t *testing.T) {
	txid1 := [32]byte{0x01}
	txid2 := [32]byte{0x02}

	prevTxID1 := [32]byte{0xaa}
	prevTxID2 := [32]byte{0xbb}

	rawTx1 := buildMinimalRawTx(prevTxID1, 0)
	rawTx2 := buildMinimalRawTx(prevTxID2, 1)

	subtreeData := buildMinimalSubtreeData([][32]byte{txid1, txid2})

	// HTTP server for subtree + transaction fetches
	mux := http.NewServeMux()
	mux.HandleFunc("/subtree", func(w http.ResponseWriter, r *http.Request) {
		w.Write(subtreeData)
	})
	mux.HandleFunc("/api/v1/tx/", func(w http.ResponseWriter, r *http.Request) {
		txidHex := r.URL.Path[len("/api/v1/tx/"):]
		// teranode.TxIDToHex reverses bytes, so txid1 (0x01 0x00...) -> "00...01"
		switch txidHex {
		case teranode.TxIDToHex(txid1[:]):
			w.Write(rawTx1)
		case teranode.TxIDToHex(txid2[:]):
			w.Write(rawTx2)
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	indexer := newMockIndexer()
	indexer.results[fmt.Sprintf("%x", txid1[:])] = []*txindexer.IndexResult{
		{Key: "type", Value: "ord", Vouts: []uint32{0}},
	}
	indexer.results[fmt.Sprintf("%x", txid2[:])] = []*txindexer.IndexResult{
		{Key: "type", Value: "bsv21", Vouts: []uint32{0, 1}},
	}

	fakeIndexRoot, err := multihash.NewIndexHash([]byte("fake-index-root"))
	if err != nil {
		t.Fatal(err)
	}

	builder := &mockBuilder{returnHash: fakeIndexRoot}
	meta := newMemMetadata()
	spendStore := newMemKVStore()
	working := newMemKVStore()
	persistent := newMemKVStore()
	dualStore := store.NewDualStore(working, persistent)
	termCache := newMemCache()
	logger := slog.Default()
	client := teranode.NewClient(srv.URL)

	p := NewProcessor(dualStore, spendStore, termCache, indexer, client, builder, meta, logger)

	err = p.processSubtreeData(context.Background(), "test-subtree", subtreeData)
	if err != nil {
		t.Fatalf("ProcessSubtree failed: %v", err)
	}

	if !builder.called {
		t.Fatal("expected builder.BuildSubtreeIndex to be called")
	}

	// Verify subtree was stored in metadata
	var subtreeRoot [32]byte
	copy(subtreeRoot[:], subtreeData[:32])

	indexRoot, err := meta.GetSubtreeIndexRoot(context.Background(), subtreeRoot[:])
	if err != nil {
		t.Fatal(err)
	}
	if indexRoot == nil {
		t.Fatal("expected subtree index root to be stored in metadata")
	}

	// Verify terms were cached
	if _, ok := termCache.Get(txid1); !ok {
		t.Fatal("expected txid1 terms to be cached")
	}
	if _, ok := termCache.Get(txid2); !ok {
		t.Fatal("expected txid2 terms to be cached")
	}

	// Verify spend records
	key1 := makeOutpointKey(prevTxID1[:], 0)
	val1, err := spendStore.Get(context.Background(), key1)
	if err != nil {
		t.Fatal(err)
	}
	if val1 == nil {
		t.Fatal("expected spend record for prevTxID1:0")
	}

	key2 := makeOutpointKey(prevTxID2[:], 1)
	val2, err := spendStore.Get(context.Background(), key2)
	if err != nil {
		t.Fatal(err)
	}
	if val2 == nil {
		t.Fatal("expected spend record for prevTxID2:1")
	}
}

func TestProcessBlockAllSubtreesReady(t *testing.T) {
	meta := newMemMetadata()

	subtreeHash1 := []byte("subtree-hash-1-that-is-32-bytes!")
	subtreeHash2 := []byte("subtree-hash-2-that-is-32-bytes!")

	// Pre-insert subtrees
	meta.InsertSubtree(context.Background(), subtreeHash1, []byte("index-root-1"), 10)
	meta.InsertSubtree(context.Background(), subtreeHash2, []byte("index-root-2"), 20)

	p := &Processor{
		metadata: meta,
		logger:   slog.Default(),
	}

	header := make([]byte, 80)
	header[0] = 0x01 // version byte

	err := p.ProcessBlock(context.Background(), 100, header, [][]byte{subtreeHash1, subtreeHash2}, 30)
	if err != nil {
		t.Fatalf("ProcessBlock failed: %v", err)
	}

	// Verify block was stored
	blockHash := blockHashFromHeader(header)
	subtrees, err := meta.GetBlockSubtrees(context.Background(), blockHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(subtrees) != 2 {
		t.Fatalf("expected 2 subtree hashes, got %d", len(subtrees))
	}
}

func TestProcessBlockMissingSubtree(t *testing.T) {
	meta := newMemMetadata()

	subtreeHash1 := []byte("subtree-hash-1-that-is-32-bytes!")
	subtreeHash2 := []byte("subtree-hash-2-that-is-32-bytes!")

	// Only insert one subtree
	meta.InsertSubtree(context.Background(), subtreeHash1, []byte("index-root-1"), 10)

	p := &Processor{
		metadata: meta,
		logger:   slog.Default(),
	}

	header := make([]byte, 80)

	err := p.ProcessBlock(context.Background(), 100, header, [][]byte{subtreeHash1, subtreeHash2}, 30)
	if !errors.Is(err, ErrSubtreeNotReady) {
		t.Fatalf("expected ErrSubtreeNotReady, got: %v", err)
	}
}

func TestParseSubtreeNodes(t *testing.T) {
	txid1 := [32]byte{0x01}
	txid2 := [32]byte{0x02}
	txid3 := [32]byte{0x03}

	data := buildMinimalSubtreeData([][32]byte{txid1, txid2, txid3})
	nodes, err := parseSubtreeNodes(data)
	if err != nil {
		t.Fatal(err)
	}

	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}

	if nodes[0].Hash != txid1 {
		t.Fatalf("node 0 hash mismatch")
	}
	if nodes[1].Hash != txid2 {
		t.Fatalf("node 1 hash mismatch")
	}
	if nodes[2].Hash != txid3 {
		t.Fatalf("node 2 hash mismatch")
	}
}

func TestParseSubtreeNodesTooShort(t *testing.T) {
	_, err := parseSubtreeNodes([]byte{0x01, 0x02})
	if err == nil {
		t.Fatal("expected error for short data")
	}
}

func TestMakeOutpointKey(t *testing.T) {
	txid := make([]byte, 32)
	txid[0] = 0xab
	key := makeOutpointKey(txid, 42)

	if len(key) != 36 {
		t.Fatalf("expected 36 bytes, got %d", len(key))
	}
	if key[0] != 0xab {
		t.Fatal("txid not copied correctly")
	}
	if binary.BigEndian.Uint32(key[32:]) != 42 {
		t.Fatal("vout not encoded correctly")
	}
}

func TestBlockHashFromHeader(t *testing.T) {
	header := make([]byte, 80)
	hash := blockHashFromHeader(header)

	if len(hash) != 32 {
		t.Fatalf("expected 32-byte hash, got %d", len(hash))
	}

	// Deterministic: same input should give same output
	hash2 := blockHashFromHeader(header)
	for i := range hash {
		if hash[i] != hash2[i] {
			t.Fatal("blockHashFromHeader is not deterministic")
		}
	}
}

// Verify memMetadata implements metadata.Store
var _ metadata.Store = (*memMetadata)(nil)
var _ kvstore.KVStore = (*memKVStore)(nil)
var _ cache.IndexTermCache = (*memCache)(nil)

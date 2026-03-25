package test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	p2pkhTemplate "github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	cachepkg "github.com/shruggr/inspiration/cache"
	cachemem "github.com/shruggr/inspiration/cache/memory"
	"github.com/shruggr/inspiration/indexnode"
	kvmem "github.com/shruggr/inspiration/kvstore/memory"
	metasqlite "github.com/shruggr/inspiration/metadata/sqlite"
	"github.com/shruggr/inspiration/multihash"
	"github.com/shruggr/inspiration/processor"
	"github.com/shruggr/inspiration/store"
	"github.com/shruggr/inspiration/teranode"
	"github.com/shruggr/inspiration/treebuilder"
	"github.com/shruggr/inspiration/txindexer"
)

const (
	addr1 = "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	addr2 = "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2"
	addr3 = "1CounterpartyXXXXXXXXXXXXXXXUWLpVr"
)

func buildP2PKHTx(t *testing.T, addresses []string, prevTxID []byte, prevVout uint32) *transaction.Transaction {
	t.Helper()
	tx := transaction.NewTransaction()

	// Add a dummy input
	input := &transaction.TransactionInput{
		SequenceNumber: 0xFFFFFFFF,
	}
	if prevTxID != nil {
		var h chainhash.Hash
		copy(h[:], prevTxID)
		input.SourceTXID = &h
		input.SourceTxOutIndex = prevVout
	}
	unlocking := script.Script([]byte{script.OpFALSE})
	input.UnlockingScript = &unlocking
	tx.AddInput(input)

	for _, addrStr := range addresses {
		addr, err := script.NewAddressFromString(addrStr)
		if err != nil {
			t.Fatalf("invalid address %s: %v", addrStr, err)
		}
		lockScript, err := p2pkhTemplate.Lock(addr)
		if err != nil {
			t.Fatalf("lock script for %s: %v", addrStr, err)
		}
		tx.AddOutput(&transaction.TransactionOutput{
			Satoshis:      1000,
			LockingScript: lockScript,
		})
	}

	return tx
}

// buildSubtreeData builds a minimal subtree binary matching parseSubtreeNodes:
// 32b root hash | 8b fees | 8b size | 8b numNodes | (32b hash + 8b fee + 8b size) per node
func buildSubtreeData(txHashes [][32]byte) []byte {
	const headerSize = 32 + 8 + 8 + 8
	const nodeSize = 48

	buf := make([]byte, headerSize+len(txHashes)*nodeSize)

	// Root hash: just fill with 0xAA
	for i := 0; i < 32; i++ {
		buf[i] = 0xAA
	}

	// Fees (LE)
	binary.LittleEndian.PutUint64(buf[32:40], 3000)
	// Size (LE)
	binary.LittleEndian.PutUint64(buf[40:48], uint64(len(txHashes)))
	// NumLeaves (LE)
	binary.LittleEndian.PutUint64(buf[48:56], uint64(len(txHashes)))

	offset := headerSize
	for _, h := range txHashes {
		copy(buf[offset:offset+32], h[:])
		binary.LittleEndian.PutUint64(buf[offset+32:offset+40], 1000)
		binary.LittleEndian.PutUint64(buf[offset+40:offset+48], 250)
		offset += nodeSize
	}

	return buf
}

func reverseTxID(txid []byte) string {
	reversed := make([]byte, 32)
	for i, b := range txid {
		reversed[31-i] = b
	}
	return hex.EncodeToString(reversed)
}

func TestIntegration(t *testing.T) {
	ctx := context.Background()
	logger := slog.Default()

	// --- Build test transactions ---
	dummyPrevTxID := make([]byte, 32)
	for i := range dummyPrevTxID {
		dummyPrevTxID[i] = byte(i)
	}

	tx1 := buildP2PKHTx(t, []string{addr1, addr2}, dummyPrevTxID, 0)
	tx2 := buildP2PKHTx(t, []string{addr2, addr3}, dummyPrevTxID, 1)
	tx3 := buildP2PKHTx(t, []string{addr1}, dummyPrevTxID, 2)

	txid1 := tx1.TxID()
	txid2 := tx2.TxID()
	txid3 := tx3.TxID()

	rawTx1 := tx1.Bytes()
	rawTx2 := tx2.Bytes()
	rawTx3 := tx3.Bytes()

	txMap := map[string][]byte{
		reverseTxID(txid1[:]): rawTx1,
		reverseTxID(txid2[:]): rawTx2,
		reverseTxID(txid3[:]): rawTx3,
	}

	txHashes := [][32]byte{*txid1, *txid2, *txid3}
	subtreeBytes := buildSubtreeData(txHashes)

	// --- Mock Teranode HTTP server ---
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/subtree" {
			w.WriteHeader(200)
			w.Write(subtreeBytes)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/v1/tx/") {
			txidHex := strings.TrimPrefix(r.URL.Path, "/api/v1/tx/")
			if raw, ok := txMap[txidHex]; ok {
				w.WriteHeader(200)
				w.Write(raw)
				return
			}
			http.NotFound(w, r)
			return
		}

		http.NotFound(w, r)
	}))
	defer mockServer.Close()

	// --- Set up infrastructure ---
	workingStore := kvmem.New()
	persistentStore := kvmem.New()
	dualStore := store.NewDualStore(workingStore, persistentStore)
	spendStore := kvmem.New()

	metaStore, err := metasqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create metadata store: %v", err)
	}
	defer metaStore.Close()

	lruCache, err := cachemem.New(1000)
	if err != nil {
		t.Fatalf("create LRU cache: %v", err)
	}

	indexer := txindexer.NewMultiIndexer(txindexer.NewP2PKHIndexer())
	client := teranode.NewClient(mockServer.URL)
	builder := treebuilder.NewBuilder(dualStore)

	proc := processor.NewProcessor(
		dualStore,
		spendStore,
		lruCache,
		indexer,
		client,
		builder,
		metaStore,
		logger,
	)

	// --- Process subtree ---
	subtreeHash := "fakesubtreehash"
	err = proc.ProcessSubtree(ctx, subtreeHash, mockServer.URL+"/subtree")
	if err != nil {
		t.Fatalf("ProcessSubtree failed: %v", err)
	}

	// --- Verify metadata: subtree exists with an index root ---
	subtreeRootBytes := subtreeBytes[:32]
	indexRootBytes, err := metaStore.GetSubtreeIndexRoot(ctx, subtreeRootBytes)
	if err != nil {
		t.Fatalf("GetSubtreeIndexRoot: %v", err)
	}
	if indexRootBytes == nil {
		t.Fatal("subtree index root is nil — subtree was not inserted into metadata")
	}

	// --- Fetch index root from KVStore and unmarshal ---
	rootNodeData, err := dualStore.Get(ctx, indexRootBytes)
	if err != nil {
		t.Fatalf("get root node: %v", err)
	}
	if rootNodeData == nil {
		t.Fatal("root index node data is nil")
	}

	rootNode, err := indexnode.Unmarshal(rootNodeData)
	if err != nil {
		t.Fatalf("unmarshal root node: %v", err)
	}

	// Root is a tag node: KeySize=0, ValueSize=32, HasData=true, SortByData=true
	if !rootNode.HasData || !rootNode.SortByData {
		t.Fatalf("expected root to be a tag node (HasData=true, SortByData=true), got HasData=%v SortByData=%v",
			rootNode.HasData, rootNode.SortByData)
	}

	// --- Verify root contains "address" tag key ---
	addressValueHash, found := rootNode.FindByData([]byte("address"))
	if !found {
		t.Fatal("root node does not contain 'address' tag key")
	}

	// The value in the node is the 32-byte hash (without the 2-byte multihash prefix).
	// Reconstruct the full multihash key: 0x1e 0x20 + 32 bytes
	fullHash := make([]byte, 34)
	fullHash[0] = 0x1e // BLAKE3 code
	fullHash[1] = 0x20 // 32 byte digest length
	copy(fullHash[2:], addressValueHash)

	addressNodeData, err := dualStore.Get(ctx, fullHash)
	if err != nil {
		t.Fatalf("get address value node: %v", err)
	}
	if addressNodeData == nil {
		t.Fatal("address value node data is nil")
	}

	addressNode, err := indexnode.Unmarshal(addressNodeData)
	if err != nil {
		t.Fatalf("unmarshal address node: %v", err)
	}

	if !addressNode.HasData || !addressNode.SortByData {
		t.Fatalf("expected address node to be a tag node, got HasData=%v SortByData=%v",
			addressNode.HasData, addressNode.SortByData)
	}

	// --- Verify specific addresses exist ---
	for _, addrStr := range []string{addr1, addr2, addr3} {
		leafHash, found := addressNode.FindByData([]byte(addrStr))
		if !found {
			t.Errorf("address %s not found in address node", addrStr)
			continue
		}

		leafFullHash := make([]byte, 34)
		leafFullHash[0] = 0x1e
		leafFullHash[1] = 0x20
		copy(leafFullHash[2:], leafHash)

		leafData, err := dualStore.Get(ctx, leafFullHash)
		if err != nil {
			t.Fatalf("get leaf data for %s: %v", addrStr, err)
		}
		if leafData == nil {
			t.Fatalf("leaf data for %s is nil", addrStr)
		}

		entries, err := indexnode.UnmarshalLeafEntryList(leafData)
		if err != nil {
			t.Fatalf("unmarshal leaf entries for %s: %v", addrStr, err)
		}

		if len(entries) == 0 {
			t.Fatalf("no leaf entries for address %s", addrStr)
		}

		t.Logf("address %s has %d leaf entries", addrStr, len(entries))
		for _, entry := range entries {
			t.Logf("  txid=%x pos=%d vouts=%v", entry.TxID[:8], entry.SubtreePosition, entry.Vouts)
		}
	}

	// --- Verify addr1 leaf entries in detail ---
	// addr1 appears in tx1 (position 0, vout 0) and tx3 (position 2, vout 0)
	addr1LeafHash, _ := addressNode.FindByData([]byte(addr1))
	addr1FullHash := make([]byte, 34)
	addr1FullHash[0] = 0x1e
	addr1FullHash[1] = 0x20
	copy(addr1FullHash[2:], addr1LeafHash)

	addr1LeafData, _ := dualStore.Get(ctx, addr1FullHash)
	addr1Entries, _ := indexnode.UnmarshalLeafEntryList(addr1LeafData)

	if len(addr1Entries) != 2 {
		t.Fatalf("expected 2 leaf entries for addr1, got %d", len(addr1Entries))
	}

	// Entries are sorted by SubtreePosition
	if addr1Entries[0].SubtreePosition != 0 {
		t.Errorf("expected first addr1 entry at position 0, got %d", addr1Entries[0].SubtreePosition)
	}
	if !bytes.Equal(addr1Entries[0].TxID, txid1[:]) {
		t.Errorf("expected first addr1 entry txid to be tx1")
	}
	if len(addr1Entries[0].Vouts) != 1 || addr1Entries[0].Vouts[0] != 0 {
		t.Errorf("expected first addr1 entry vouts=[0], got %v", addr1Entries[0].Vouts)
	}

	if addr1Entries[1].SubtreePosition != 2 {
		t.Errorf("expected second addr1 entry at position 2, got %d", addr1Entries[1].SubtreePosition)
	}
	if !bytes.Equal(addr1Entries[1].TxID, txid3[:]) {
		t.Errorf("expected second addr1 entry txid to be tx3")
	}

	// --- Verify addr2 leaf entries ---
	// addr2 appears in tx1 (position 0, vout 1) and tx2 (position 1, vout 0)
	addr2LeafHash, _ := addressNode.FindByData([]byte(addr2))
	addr2FullHash := make([]byte, 34)
	addr2FullHash[0] = 0x1e
	addr2FullHash[1] = 0x20
	copy(addr2FullHash[2:], addr2LeafHash)

	addr2LeafData, _ := dualStore.Get(ctx, addr2FullHash)
	addr2Entries, _ := indexnode.UnmarshalLeafEntryList(addr2LeafData)

	if len(addr2Entries) != 2 {
		t.Fatalf("expected 2 leaf entries for addr2, got %d", len(addr2Entries))
	}

	if addr2Entries[0].SubtreePosition != 0 {
		t.Errorf("expected first addr2 entry at position 0, got %d", addr2Entries[0].SubtreePosition)
	}
	if addr2Entries[1].SubtreePosition != 1 {
		t.Errorf("expected second addr2 entry at position 1, got %d", addr2Entries[1].SubtreePosition)
	}

	// --- Test ScanPrefix on address node ---
	// All three addresses start with "1", so scanning for "1" should find all of them.
	// But let's test a more specific prefix.
	scanResults := addressNode.ScanPrefix([]byte("1A"))
	if len(scanResults) == 0 {
		t.Fatal("ScanPrefix('1A') returned no results")
	}
	t.Logf("ScanPrefix('1A') found %d results", len(scanResults))

	// Verify addr1 starts with "1A"
	if !strings.HasPrefix(addr1, "1A") {
		t.Fatal("addr1 should start with 1A")
	}
	foundAddr1 := false
	for _, val := range scanResults {
		if bytes.Equal(val, addr1LeafHash) {
			foundAddr1 = true
		}
	}
	if !foundAddr1 {
		t.Error("ScanPrefix('1A') did not find addr1's leaf hash")
	}

	// --- Process block referencing the subtree ---
	fakeHeader := make([]byte, 80)
	for i := range fakeHeader {
		fakeHeader[i] = byte(i % 256)
	}

	err = proc.ProcessBlock(ctx, 100, fakeHeader, [][]byte{subtreeRootBytes}, 3)
	if err != nil {
		t.Fatalf("ProcessBlock failed: %v", err)
	}

	// --- Verify block metadata ---
	exists, err := metaStore.SubtreeExists(ctx, subtreeRootBytes)
	if err != nil {
		t.Fatalf("SubtreeExists: %v", err)
	}
	if !exists {
		t.Fatal("subtree should still exist after block processing")
	}

	// --- Test spend records ---
	// All 3 transactions spend from dummyPrevTxID at vouts 0, 1, 2
	for vout := uint32(0); vout < 3; vout++ {
		spendKey := make([]byte, 36)
		copy(spendKey, dummyPrevTxID)
		binary.BigEndian.PutUint32(spendKey[32:], vout)

		spendValue, err := spendStore.Get(ctx, spendKey)
		if err != nil {
			t.Fatalf("get spend record for vout %d: %v", vout, err)
		}
		if spendValue == nil {
			t.Errorf("no spend record for dummyPrevTxID vout %d", vout)
			continue
		}

		var expectedTxID cachepkg.TxID
		switch vout {
		case 0:
			expectedTxID = *txid1
		case 1:
			expectedTxID = *txid2
		case 2:
			expectedTxID = *txid3
		}

		if !bytes.Equal(spendValue, expectedTxID[:]) {
			t.Errorf("spend record vout %d: expected txid %x, got %x", vout, expectedTxID[:8], spendValue[:8])
		}
	}

	// --- Test that processing a block with unknown subtree fails ---
	unknownHash := make([]byte, 32)
	unknownHash[0] = 0xFF
	err = proc.ProcessBlock(ctx, 101, fakeHeader, [][]byte{unknownHash}, 1)
	if err == nil {
		t.Fatal("ProcessBlock should fail with unknown subtree hash")
	}
	t.Logf("expected error for unknown subtree: %v", err)

	// --- Print summary ---
	t.Log("Integration test passed: subtree processing, index building, prefix query, block processing, and spend records all verified")
}

func TestIntegration_IndexHashFormat(t *testing.T) {
	// Verify that the multihash prefix bytes we use (0x1e, 0x20) match what NewIndexHash produces
	testData := []byte("hello world")
	h, err := multihash.NewIndexHash(testData)
	if err != nil {
		t.Fatalf("NewIndexHash: %v", err)
	}

	raw := h.Bytes()
	if len(raw) != 34 {
		t.Fatalf("expected 34-byte IndexHash, got %d", len(raw))
	}
	if raw[0] != 0x1e {
		t.Errorf("expected BLAKE3 code 0x1e, got 0x%02x", raw[0])
	}
	if raw[1] != 0x20 {
		t.Errorf("expected digest length 0x20, got 0x%02x", raw[1])
	}

	if err := h.Verify(testData); err != nil {
		t.Errorf("hash verification failed: %v", err)
	}
}

func TestIntegration_SubtreeFormat(t *testing.T) {
	// Verify our buildSubtreeData matches what parseSubtreeNodes expects
	var h1, h2 [32]byte
	for i := range h1 {
		h1[i] = byte(i)
		h2[i] = byte(i + 32)
	}

	data := buildSubtreeData([][32]byte{h1, h2})

	// Verify header
	numLeaves := binary.LittleEndian.Uint64(data[48:56])
	if numLeaves != 2 {
		t.Fatalf("expected 2 leaves, got %d", numLeaves)
	}

	// Verify first node hash
	offset := 56
	var got [32]byte
	copy(got[:], data[offset:offset+32])
	if got != h1 {
		t.Errorf("first node hash mismatch")
	}

	// Verify second node hash
	offset += 48
	copy(got[:], data[offset:offset+32])
	if got != h2 {
		t.Errorf("second node hash mismatch")
	}

	_ = fmt.Sprintf("subtree data length: %d", len(data))
}

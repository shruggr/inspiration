package txindexer

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
)

func buildTestP2PKHTx(t *testing.T, lockingScripts ...[]byte) []byte {
	t.Helper()
	tx := transaction.NewTransaction()
	tx.AddInput(&transaction.TransactionInput{
		SourceTXID:       &chainhash.Hash{},
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
	p2pkhScript, _ := hex.DecodeString("76a91462e907b15cbf27d5425399ebf6f0fb50ebb88f1888ac")
	rawTx := buildTestP2PKHTx(t, p2pkhScript)

	indexer := NewP2PKHIndexer()
	results, err := indexer.Index(context.Background(), &TransactionContext{
		TxID:  make([]byte, 32),
		RawTx: rawTx,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Key != "address" {
		t.Errorf("expected key 'address', got %q", results[0].Key)
	}
	if results[0].Value == "" {
		t.Error("expected non-empty address value")
	}
	if len(results[0].Vouts) != 1 || results[0].Vouts[0] != 0 {
		t.Errorf("expected vouts [0], got %v", results[0].Vouts)
	}
}

func TestP2PKHMultipleOutputsSameAddress(t *testing.T) {
	p2pkhScript, _ := hex.DecodeString("76a91462e907b15cbf27d5425399ebf6f0fb50ebb88f1888ac")
	rawTx := buildTestP2PKHTx(t, p2pkhScript, p2pkhScript)

	indexer := NewP2PKHIndexer()
	results, err := indexer.Index(context.Background(), &TransactionContext{
		TxID:  make([]byte, 32),
		RawTx: rawTx,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (grouped), got %d", len(results))
	}
	if len(results[0].Vouts) != 2 || results[0].Vouts[0] != 0 || results[0].Vouts[1] != 1 {
		t.Errorf("expected vouts [0,1], got %v", results[0].Vouts)
	}
}

func TestP2PKHNonP2PKHOutput(t *testing.T) {
	opReturnScript, _ := hex.DecodeString("006a0568656c6c6f")
	rawTx := buildTestP2PKHTx(t, opReturnScript)

	indexer := NewP2PKHIndexer()
	results, err := indexer.Index(context.Background(), &TransactionContext{
		TxID:  make([]byte, 32),
		RawTx: rawTx,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for OP_RETURN, got %d", len(results))
	}
}

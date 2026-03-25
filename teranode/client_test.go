package teranode

import (
	"context"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchTransaction_Success(t *testing.T) {
	expected := []byte{0x01, 0x02, 0x03, 0x04}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tx/abcd1234" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Accept") != "application/octet-stream" {
			t.Errorf("unexpected Accept header: %s", r.Header.Get("Accept"))
		}
		w.Write(expected)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	got, err := c.FetchTransaction(context.Background(), "abcd1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(expected) {
		t.Fatalf("expected %d bytes, got %d", len(expected), len(got))
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Fatalf("byte %d: expected %02x, got %02x", i, expected[i], got[i])
		}
	}
}

func TestFetchTransaction_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.FetchTransaction(context.Background(), "deadbeef")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestFetchSubtreeData_Success(t *testing.T) {
	expected := []byte{0xaa, 0xbb, 0xcc}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(expected)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	got, err := c.FetchSubtreeData(context.Background(), srv.URL+"/subtree/123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(expected) {
		t.Fatalf("expected %d bytes, got %d", len(expected), len(got))
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Fatalf("byte %d: expected %02x, got %02x", i, expected[i], got[i])
		}
	}
}

func TestTxIDToHex(t *testing.T) {
	// 32 bytes: 00 01 02 ... 1f
	txid := make([]byte, 32)
	for i := range txid {
		txid[i] = byte(i)
	}

	got := TxIDToHex(txid)

	// Expected: reversed bytes as hex
	reversed := make([]byte, 32)
	for i, b := range txid {
		reversed[31-i] = b
	}
	expected := hex.EncodeToString(reversed)

	if got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}

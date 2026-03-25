package store

import (
	"context"
	"errors"
	"testing"

	"github.com/shruggr/inspiration/kvstore"
	"github.com/shruggr/inspiration/kvstore/memory"
)

func newTestDual() (*DualStore, *memory.Store, *memory.Store) {
	w := memory.New()
	p := memory.New()
	return NewDualStore(w, p), w, p
}

func TestReadThrough(t *testing.T) {
	dual, _, p := newTestDual()
	ctx := context.Background()

	if err := p.Put(ctx, []byte("k1"), []byte("v1")); err != nil {
		t.Fatal(err)
	}

	val, err := dual.Get(ctx, []byte("k1"))
	if err != nil {
		t.Fatal(err)
	}
	if string(val) != "v1" {
		t.Fatalf("expected v1, got %s", val)
	}
}

func TestWriteToWorking(t *testing.T) {
	dual, w, p := newTestDual()
	ctx := context.Background()

	if err := dual.Put(ctx, []byte("k1"), []byte("v1")); err != nil {
		t.Fatal(err)
	}

	// Should be in working
	val, err := w.Get(ctx, []byte("k1"))
	if err != nil {
		t.Fatal(err)
	}
	if string(val) != "v1" {
		t.Fatalf("expected v1 in working, got %s", val)
	}

	// Should NOT be in persistent
	val, err = p.Get(ctx, []byte("k1"))
	if err != nil {
		t.Fatal(err)
	}
	if val != nil {
		t.Fatalf("expected nil in persistent, got %s", val)
	}
}

func TestPromote(t *testing.T) {
	dual, _, p := newTestDual()
	ctx := context.Background()

	if err := dual.Put(ctx, []byte("k1"), []byte("v1")); err != nil {
		t.Fatal(err)
	}

	if err := dual.Promote(ctx, []byte("k1")); err != nil {
		t.Fatal(err)
	}

	val, err := p.Get(ctx, []byte("k1"))
	if err != nil {
		t.Fatal(err)
	}
	if string(val) != "v1" {
		t.Fatalf("expected v1 in persistent, got %s", val)
	}
}

func TestHasReadThrough(t *testing.T) {
	dual, _, p := newTestDual()
	ctx := context.Background()

	if err := p.Put(ctx, []byte("k1"), []byte("v1")); err != nil {
		t.Fatal(err)
	}

	ok, err := dual.Has(ctx, []byte("k1"))
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected Has to return true via read-through")
	}
}

// errStore is a KVStore that always returns errors, used to test error propagation.
type errStore struct{}

func (e *errStore) Put(context.Context, []byte, []byte) error { return errors.New("put error") }
func (e *errStore) Get(context.Context, []byte) ([]byte, error) {
	return nil, errors.New("get error")
}
func (e *errStore) Delete(context.Context, []byte) error    { return errors.New("delete error") }
func (e *errStore) Has(context.Context, []byte) (bool, error) { return false, errors.New("has error") }
func (e *errStore) Close() error                              { return nil }

var _ kvstore.KVStore = (*errStore)(nil)

func TestHasError(t *testing.T) {
	dual := NewDualStore(&errStore{}, memory.New())
	ctx := context.Background()

	_, err := dual.Has(ctx, []byte("k1"))
	if err == nil {
		t.Fatal("expected error from working store, got nil")
	}
}

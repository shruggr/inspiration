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

// Promote copies a key from working to persistent space.
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

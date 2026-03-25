package memory

import (
	"context"
	"testing"
)

func TestPutGet(t *testing.T) {
	s := New()
	ctx := context.Background()

	key := []byte{0x01, 0x02, 0x03}
	value := []byte("hello")

	if err := s.Put(ctx, key, value); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := s.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(value) {
		t.Fatalf("Get returned %q, want %q", got, value)
	}
}

func TestGetMissing(t *testing.T) {
	s := New()
	ctx := context.Background()

	got, err := s.Get(ctx, []byte("missing"))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Fatalf("Get returned %q, want nil", got)
	}
}

func TestDelete(t *testing.T) {
	s := New()
	ctx := context.Background()

	key := []byte("key")
	if err := s.Put(ctx, key, []byte("val")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := s.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get after Delete: %v", err)
	}
	if got != nil {
		t.Fatalf("Get after Delete returned %q, want nil", got)
	}
}

func TestHas(t *testing.T) {
	s := New()
	ctx := context.Background()

	key := []byte{0xff, 0x00, 0xab}

	ok, err := s.Has(ctx, key)
	if err != nil {
		t.Fatalf("Has: %v", err)
	}
	if ok {
		t.Fatal("Has returned true for missing key")
	}

	if err := s.Put(ctx, key, []byte("data")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	ok, err = s.Has(ctx, key)
	if err != nil {
		t.Fatalf("Has: %v", err)
	}
	if !ok {
		t.Fatal("Has returned false for existing key")
	}
}

func TestBinaryKeys(t *testing.T) {
	s := New()
	ctx := context.Background()

	// Two keys that would collide under hex encoding if implementation were wrong
	key1 := []byte{0xde, 0xad}
	key2 := []byte{0xde, 0xae}

	if err := s.Put(ctx, key1, []byte("one")); err != nil {
		t.Fatalf("Put key1: %v", err)
	}
	if err := s.Put(ctx, key2, []byte("two")); err != nil {
		t.Fatalf("Put key2: %v", err)
	}

	got1, _ := s.Get(ctx, key1)
	got2, _ := s.Get(ctx, key2)

	if string(got1) != "one" {
		t.Fatalf("key1: got %q, want %q", got1, "one")
	}
	if string(got2) != "two" {
		t.Fatalf("key2: got %q, want %q", got2, "two")
	}
}

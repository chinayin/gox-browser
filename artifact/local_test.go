package artifact

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestLocalStore_PutGetRoundtrip(t *testing.T) {
	s := NewLocalStore(LocalConfig{BaseDir: t.TempDir()})
	ctx := context.Background()
	if err := s.Put(ctx, "task-1/a.json", []byte(`{"x":1}`), "application/json"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := s.Get(ctx, "task-1/a.json")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != `{"x":1}` {
		t.Fatalf("got %q", got)
	}
}

func TestLocalStore_PutReaderCountsBytes(t *testing.T) {
	s := NewLocalStore(LocalConfig{BaseDir: t.TempDir()})
	ctx := context.Background()
	if err := s.PutReader(ctx, "task-1/big.txt", strings.NewReader("hello world"), "text/plain"); err != nil {
		t.Fatalf("PutReader: %v", err)
	}
	got, err := s.Get(ctx, "task-1/big.txt")
	if err != nil || string(got) != "hello world" {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestLocalStore_ExistsAndNotFound(t *testing.T) {
	s := NewLocalStore(LocalConfig{BaseDir: t.TempDir()})
	ctx := context.Background()
	ok, err := s.Exists(ctx, "task-1/missing")
	if err != nil || ok {
		t.Fatalf("Exists on missing = %v,%v", ok, err)
	}
	if _, err := s.Get(ctx, "task-1/missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get missing err = %v, want ErrNotFound", err)
	}
}

func TestLocalStore_SignURLUnsupported(t *testing.T) {
	s := NewLocalStore(LocalConfig{BaseDir: t.TempDir()})
	if _, err := s.SignURL(context.Background(), "task-1/a", time.Minute); !errors.Is(err, ErrSignURLUnsupported) {
		t.Fatalf("SignURL err = %v, want ErrSignURLUnsupported", err)
	}
}

func TestLocalStore_RejectsEscape(t *testing.T) {
	s := NewLocalStore(LocalConfig{BaseDir: t.TempDir()})
	if err := s.Put(context.Background(), "../escape", []byte("x"), ""); err == nil {
		t.Fatal("Put with escaping key should error")
	}
}

func TestLocalStore_ReaderRoundtrip(t *testing.T) {
	s := NewLocalStore(LocalConfig{BaseDir: t.TempDir()})
	ctx := context.Background()
	if err := s.Put(ctx, "task-1/stream.txt", []byte("stream this"), "text/plain"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	r, err := s.Reader(ctx, "task-1/stream.txt")
	if err != nil {
		t.Fatalf("Reader: %v", err)
	}
	defer r.Close()
	buf := make([]byte, len("stream this"))
	if _, err := r.Read(buf); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(buf) != "stream this" {
		t.Fatalf("got %q", buf)
	}
}

func TestLocalStore_ReaderNotFound(t *testing.T) {
	s := NewLocalStore(LocalConfig{BaseDir: t.TempDir()})
	if _, err := s.Reader(context.Background(), "task-1/missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Reader missing err = %v, want ErrNotFound", err)
	}
}

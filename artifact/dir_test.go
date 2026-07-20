package artifact

import (
	"context"
	"strings"
	"testing"
)

func TestDir_PutFinalizeLoadIndex(t *testing.T) {
	st := New(NewLocalStore(LocalConfig{BaseDir: t.TempDir()}))
	d := st.Dir("task-1")
	ctx := context.Background()

	if err := d.Put(ctx, "uploads/envelope.json", []byte(`{"ok":true}`), map[string]string{"kind": "result"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := d.PutReader(ctx, "screenshots/1.png", strings.NewReader("PNGDATA"), map[string]string{"kind": "screenshot"}); err != nil {
		t.Fatalf("PutReader: %v", err)
	}
	if err := d.Finalize(ctx); err != nil {
		t.Fatalf("Finalize: %v", err)
	}

	idx, err := d.LoadIndex(ctx)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}
	if idx.Stats.Count != 2 || idx.Namespace != "task-1" {
		t.Fatalf("stats=%+v ns=%q", idx.Stats, idx.Namespace)
	}
	e, ok := idx.Find("uploads/envelope.json")
	if !ok || e.Tags["kind"] != "result" || e.ContentType != "application/json" {
		t.Fatalf("entry=%+v ok=%v", e, ok)
	}
	if !strings.HasPrefix(e.Checksum, "sha256:") || e.Size != int64(len(`{"ok":true}`)) {
		t.Fatalf("checksum=%q size=%d", e.Checksum, e.Size)
	}
	if got := idx.ByKind("screenshot"); len(got) != 1 || got[0].Size != int64(len("PNGDATA")) {
		t.Fatalf("screenshot entry=%v", got)
	}
}

func TestDir_ExistsAndGet(t *testing.T) {
	d := New(NewLocalStore(LocalConfig{BaseDir: t.TempDir()})).Dir("task-1")
	ctx := context.Background()
	ok, _ := d.Exists(ctx, "a.json")
	if ok {
		t.Fatal("Exists before Put should be false")
	}
	_ = d.Put(ctx, "a.json", []byte("hi"), nil)
	ok, _ = d.Exists(ctx, "a.json")
	if !ok {
		t.Fatal("Exists after Put should be true")
	}
	got, _ := d.Get(ctx, "a.json")
	if string(got) != "hi" {
		t.Fatalf("Get = %q", got)
	}
}

func TestDir_RejectsEscapingRef(t *testing.T) {
	d := New(NewLocalStore(LocalConfig{BaseDir: t.TempDir()})).Dir("task-1")
	if err := d.Put(context.Background(), "../evil", []byte("x"), nil); err == nil {
		t.Fatal("Put with escaping ref should error")
	}
}

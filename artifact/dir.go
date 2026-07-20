package artifact

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"time"
)

const (
	indexFileName = "index.json"
	indexVersion  = "v1"
)

// Store 是 artifact 组件入口，持有一个 Storer 后端。
type Store struct{ backend Storer }

// New 用给定后端创建 Store。
func New(backend Storer) *Store { return &Store{backend: backend} }

// Dir 返回绑定命名空间的目录门面。
func (s *Store) Dir(namespace string) *Dir {
	return &Dir{store: s.backend, namespace: namespace}
}

// Dir 是绑定命名空间的 artifact 目录门面，消费方唯一入口。
type Dir struct {
	store     Storer
	namespace string
	mu        sync.Mutex
	entries   []Entry // 内存累积，Finalize 时写成 index.json
}

// Put 写入一个 artifact（ref 相对命名空间，如 "uploads/envelope.json"）。
func (d *Dir) Put(ctx context.Context, ref string, data []byte, tags map[string]string) error {
	ref, err := cleanRef(ref)
	if err != nil {
		return err
	}
	ct := DetectContentType(ref)
	if err := d.store.Put(ctx, d.key(ref), data, ct); err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	d.record(Entry{Ref: ref, ContentType: ct, Size: int64(len(data)), Checksum: "sha256:" + hex.EncodeToString(sum[:]), Tags: tags})
	return nil
}

// PutReader 流式写入大 artifact；用 TeeReader 边传边算 sha256/size。
func (d *Dir) PutReader(ctx context.Context, ref string, r io.Reader, tags map[string]string) error {
	ref, err := cleanRef(ref)
	if err != nil {
		return err
	}
	ct := DetectContentType(ref)
	h := sha256.New()
	cw := &countingWriter{}
	tee := io.TeeReader(r, io.MultiWriter(h, cw))
	if err := d.store.PutReader(ctx, d.key(ref), tee, ct); err != nil {
		return err
	}
	d.record(Entry{Ref: ref, ContentType: ct, Size: cw.n, Checksum: "sha256:" + hex.EncodeToString(h.Sum(nil)), Tags: tags})
	return nil
}

// Get 读取 artifact 内容。
func (d *Dir) Get(ctx context.Context, ref string) ([]byte, error) {
	ref, err := cleanRef(ref)
	if err != nil {
		return nil, err
	}
	return d.store.Get(ctx, d.key(ref))
}

// Reader 流式读取 artifact。
func (d *Dir) Reader(ctx context.Context, ref string) (io.ReadCloser, error) {
	ref, err := cleanRef(ref)
	if err != nil {
		return nil, err
	}
	return d.store.Reader(ctx, d.key(ref))
}

// Exists 判断 artifact 是否存在（物理，用于 serving 404 / result_ref 存在性）。
func (d *Dir) Exists(ctx context.Context, ref string) (bool, error) {
	ref, err := cleanRef(ref)
	if err != nil {
		return false, err
	}
	return d.store.Exists(ctx, d.key(ref))
}

// SignURL 为 artifact 签发短期直连地址（后端不支持则返回 ErrSignURLUnsupported）。
func (d *Dir) SignURL(ctx context.Context, ref string, ttl time.Duration) (string, error) {
	ref, err := cleanRef(ref)
	if err != nil {
		return "", err
	}
	return d.store.SignURL(ctx, d.key(ref), ttl)
}

// Finalize 把内存累积的条目写成 index.json（终态一次，单写者）。
func (d *Dir) Finalize(ctx context.Context) error {
	d.mu.Lock()
	arts := make([]Entry, len(d.entries))
	copy(arts, d.entries)
	d.mu.Unlock()

	var total int64
	for _, e := range arts {
		total += e.Size
	}
	idx := Index{
		Version:     indexVersion,
		Namespace:   d.namespace,
		GeneratedAt: time.Now().UTC().Unix(),
		Artifacts:   arts,
		Stats:       Stats{Count: len(arts), TotalSize: total},
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("artifact: marshal index: %w", err)
	}
	return d.store.Put(ctx, d.key(indexFileName), data, "application/json")
}

// LoadIndex 读回 index.json → 类型化可查对象（每次返回新对象，无缓存）。
func (d *Dir) LoadIndex(ctx context.Context) (*Index, error) {
	data, err := d.store.Get(ctx, d.key(indexFileName))
	if err != nil {
		return nil, err
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("artifact: unmarshal index: %w", err)
	}
	return &idx, nil
}

func (d *Dir) key(ref string) string { return BuildKey(d.namespace, ref) }

func (d *Dir) record(e Entry) {
	d.mu.Lock()
	d.entries = append(d.entries, e)
	d.mu.Unlock()
}

// countingWriter 只累计写入字节数（配合 TeeReader 算 size）。
type countingWriter struct{ n int64 }

func (w *countingWriter) Write(p []byte) (int, error) {
	w.n += int64(len(p))
	return len(p), nil
}

// cleanRef 校验并规整 ref：拒绝空/绝对路径/逃出命名空间的 "..”。
func cleanRef(ref string) (string, error) {
	if ref == "" {
		return "", fmt.Errorf("artifact: empty ref")
	}
	if path.IsAbs(ref) {
		return "", fmt.Errorf("artifact: ref must be relative: %q", ref)
	}
	cleaned := path.Clean(ref)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("artifact: ref escapes namespace: %q", ref)
	}
	return cleaned, nil
}

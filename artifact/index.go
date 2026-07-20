package artifact

// Index 是一个 artifact 目录（命名空间）的清单，index.json 的反序列化结构。
type Index struct {
	Version     string  `json:"version"`      // "v1"
	Namespace   string  `json:"namespace"`
	GeneratedAt int64   `json:"generated_at"` // Unix 秒
	Artifacts   []Entry `json:"artifacts"`
	Stats       Stats   `json:"stats"`
}

// Entry 清单里的一个 artifact 条目。
type Entry struct {
	Ref         string            `json:"ref"`          // 相对命名空间
	ContentType string            `json:"content_type"`
	Size        int64             `json:"size"`
	Checksum    string            `json:"checksum"` // "sha256:..."
	Tags        map[string]string `json:"tags,omitempty"`
}

// Stats 聚合统计。
type Stats struct {
	Count     int   `json:"count"`
	TotalSize int64 `json:"total_size"`
}

// Find 按 ref 查条目。
func (ix *Index) Find(ref string) (*Entry, bool) {
	for i := range ix.Artifacts {
		if ix.Artifacts[i].Ref == ref {
			return &ix.Artifacts[i], true
		}
	}
	return nil, false
}

// ByKind 按 tags["kind"] 过滤（如 "screenshot"/"result"）。
func (ix *Index) ByKind(kind string) []Entry {
	var out []Entry
	for _, e := range ix.Artifacts {
		if e.Tags["kind"] == kind {
			out = append(out, e)
		}
	}
	return out
}

// Refs 返回全部条目的 ref。
func (ix *Index) Refs() []string {
	out := make([]string, 0, len(ix.Artifacts))
	for _, e := range ix.Artifacts {
		out = append(out, e.Ref)
	}
	return out
}

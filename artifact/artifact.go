package artifact

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"time"
)

// Storer 存储后端 SPI（给存储后端实现者），按完整 key 操作。
// 扩展点：加新后端（GCS/MinIO…）只需实现一份。消费方不直接用它，用 Dir 门面。
type Storer interface {
	// Put 写对象（小对象便捷入口）。contentType 供后端设原生元信息（如 S3 Content-Type），仅服务用途。
	Put(ctx context.Context, key string, data []byte, contentType string) error
	// PutReader 流式写对象（大对象：录屏/大下载，避免整块入内存）。
	PutReader(ctx context.Context, key string, r io.Reader, contentType string) error
	// Get 读对象（整块）。不存在返回 ErrNotFound。
	Get(ctx context.Context, key string) ([]byte, error)
	// Reader 流式读（大文件）。不设内部超时，生命周期/超时由调用方经 ctx 控制。
	Reader(ctx context.Context, key string) (io.ReadCloser, error)
	// Exists 判断对象是否存在。
	Exists(ctx context.Context, key string) (bool, error)
	// SignURL 签发短期直连下载地址。不支持的后端（本地）返回 ErrSignURLUnsupported。
	SignURL(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// BuildKey 构建存储路径
// 过滤空段，用 "/" 连接
func BuildKey(parts ...string) string {
	var segments []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			segments = append(segments, p)
		}
	}
	return strings.Join(segments, "/")
}

// DetectContentType 根据文件扩展名推断 MIME 类型
func DetectContentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".json":
		return "application/json"
	case ".html", ".htm":
		return "text/html"
	case ".txt":
		return "text/plain"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".xml":
		return "application/xml"
	case ".pdf":
		return "application/pdf"
	case ".csv":
		return "text/csv"
	case ".yaml", ".yml":
		return "application/x-yaml"
	default:
		return "application/octet-stream"
	}
}

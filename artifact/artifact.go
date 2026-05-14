package artifact

import (
	"context"
	"io"
	"path/filepath"
	"strings"
)

// Storer 产出物存储接口
type Storer interface {
	// Upload 上传数据，返回存储路径/key
	// namespace: 逻辑分组 (如 taskID、jobID)
	// subDir: 子目录 (如 "001", "step_01")，可为空
	// name: 文件名 (如 "screenshot.png")
	Upload(ctx context.Context, namespace, subDir, name string, data []byte) (string, error)

	// Download 下载数据
	Download(ctx context.Context, path string) ([]byte, error)

	// Reader 返回流式读取器 (大文件场景)
	Reader(ctx context.Context, path string) (io.ReadCloser, error)
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

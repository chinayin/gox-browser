package artifact

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// LocalConfig 本地文件系统存储配置
type LocalConfig struct {
	// BaseDir 根目录 (如 "runtime/artifacts")
	BaseDir string
}

// LocalStore 本地文件系统存储
type LocalStore struct {
	baseDir string
}

// 编译期接口合规断言
var _ Storer = (*LocalStore)(nil)

// NewLocalStore 创建本地存储
func NewLocalStore(cfg LocalConfig) *LocalStore {
	return &LocalStore{baseDir: cfg.BaseDir}
}

// Upload 上传到本地文件系统
func (s *LocalStore) Upload(_ context.Context, namespace, subDir, name string, data []byte) (string, error) {
	key := BuildKey(namespace, subDir, name)
	dir := filepath.Join(s.baseDir, filepath.Dir(key))

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("artifact: mkdir %q: %w", dir, err)
	}

	path := filepath.Join(s.baseDir, key)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("artifact: write %q: %w", path, err)
	}

	slog.Debug("artifact: saved locally", "path", path, "size", len(data))
	return path, nil
}

// Download 从本地文件系统读取
func (s *LocalStore) Download(_ context.Context, path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("artifact: read %q: %w", path, ErrNotFound)
		}
		return nil, fmt.Errorf("artifact: read %q: %w", path, err)
	}
	return data, nil
}

// Reader 返回流式读取器
func (s *LocalStore) Reader(_ context.Context, path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("artifact: open %q: %w", path, ErrNotFound)
		}
		return nil, fmt.Errorf("artifact: open %q: %w", path, err)
	}
	return f, nil
}

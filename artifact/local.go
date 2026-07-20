package artifact

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalConfig 本地文件系统存储配置
type LocalConfig struct {
	BaseDir string // 根目录 (如 "runtime/artifacts")
}

// LocalStore 本地文件系统存储
type LocalStore struct{ baseDir string }

var _ Storer = (*LocalStore)(nil)

// NewLocalStore 创建本地存储
func NewLocalStore(cfg LocalConfig) *LocalStore { return &LocalStore{baseDir: cfg.BaseDir} }

func (s *LocalStore) Put(_ context.Context, key string, data []byte, _ string) error {
	full, err := s.pathForWrite(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("artifact: mkdir: %w", err)
	}
	if err := os.WriteFile(full, data, 0o600); err != nil {
		return fmt.Errorf("artifact: write %q: %w", key, err)
	}
	slog.Debug("artifact: saved locally", "key", key, "size", len(data))
	return nil
}

func (s *LocalStore) PutReader(_ context.Context, key string, r io.Reader, _ string) error {
	full, err := s.pathForWrite(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("artifact: mkdir: %w", err)
	}
	f, err := os.OpenFile(full, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("artifact: open %q: %w", key, err)
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("artifact: copy %q: %w", key, err)
	}
	return nil
}

func (s *LocalStore) Get(_ context.Context, key string) ([]byte, error) {
	full, err := s.pathForRead(key)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("artifact: read %q: %w", key, ErrNotFound)
		}
		return nil, err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("artifact: read %q: %w", key, ErrNotFound)
		}
		return nil, fmt.Errorf("artifact: read %q: %w", key, err)
	}
	return data, nil
}

func (s *LocalStore) Reader(_ context.Context, key string) (io.ReadCloser, error) {
	full, err := s.pathForRead(key)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("artifact: open %q: %w", key, ErrNotFound)
		}
		return nil, err
	}
	f, err := os.Open(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("artifact: open %q: %w", key, ErrNotFound)
		}
		return nil, fmt.Errorf("artifact: open %q: %w", key, err)
	}
	return f, nil
}

func (s *LocalStore) Exists(_ context.Context, key string) (bool, error) {
	full, err := s.pathForRead(key)
	if err != nil {
		if os.IsNotExist(err) { // 目标本身不存在（EvalSymlinks 报 not-exist）
			return false, nil
		}
		return false, err
	}
	if _, err := os.Stat(full); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("artifact: stat %q: %w", key, err)
	}
	return true, nil
}

func (s *LocalStore) SignURL(_ context.Context, _ string, _ time.Duration) (string, error) {
	return "", ErrSignURLUnsupported
}

// pathForWrite 返回 baseDir 内的写入路径，仅做前缀校验（文件尚不存在，无法解析软链）。
func (s *LocalStore) pathForWrite(key string) (string, error) {
	baseAbs, err := filepath.Abs(s.baseDir)
	if err != nil {
		return "", err
	}
	full := filepath.Join(baseAbs, filepath.FromSlash(key))
	if full != baseAbs && !strings.HasPrefix(full, baseAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("artifact: key %q escapes base dir", key)
	}
	return full, nil
}

// pathForRead 解析软链后校验最终目标仍在 baseDir 内，防软链逃逸。
func (s *LocalStore) pathForRead(key string) (string, error) {
	baseAbs, err := filepath.Abs(s.baseDir)
	if err != nil {
		return "", err
	}
	full := filepath.Join(baseAbs, filepath.FromSlash(key))
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", err // 含 not-exist；上层据 os.IsNotExist 判定
	}
	baseResolved, err := filepath.EvalSymlinks(baseAbs)
	if err != nil {
		baseResolved = baseAbs
	}
	if resolved != baseResolved && !strings.HasPrefix(resolved, baseResolved+string(os.PathSeparator)) {
		return "", fmt.Errorf("artifact: key %q escapes base dir", key)
	}
	return resolved, nil
}

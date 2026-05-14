package artifact_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chinayin/gox-browser/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStore_Upload_Success(t *testing.T) {
	// Arrange
	baseDir := t.TempDir()
	store := artifact.NewLocalStore(artifact.LocalConfig{BaseDir: baseDir})
	ctx := context.Background()
	data := []byte("hello world")

	// Act
	path, err := store.Upload(ctx, "ns1", "sub1", "file.txt", data)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, path)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, data, content)
}

func TestLocalStore_Upload_NestedDir(t *testing.T) {
	// Arrange
	baseDir := t.TempDir()
	store := artifact.NewLocalStore(artifact.LocalConfig{BaseDir: baseDir})
	ctx := context.Background()
	data := []byte("nested content")

	// Act
	path, err := store.Upload(ctx, "project", "level1/level2/level3", "deep.json", data)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, path)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, data, content)

	// Verify directory structure was created
	expectedDir := filepath.Join(baseDir, "project", "level1/level2/level3")
	info, err := os.Stat(expectedDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestLocalStore_Upload_EmptySubDir(t *testing.T) {
	// Arrange
	baseDir := t.TempDir()
	store := artifact.NewLocalStore(artifact.LocalConfig{BaseDir: baseDir})
	ctx := context.Background()
	data := []byte("no subdir content")

	// Act
	path, err := store.Upload(ctx, "ns1", "", "file.txt", data)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, path)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, data, content)
}

func TestLocalStore_Download_Success(t *testing.T) {
	// Arrange
	baseDir := t.TempDir()
	store := artifact.NewLocalStore(artifact.LocalConfig{BaseDir: baseDir})
	ctx := context.Background()
	data := []byte("download me")

	path, err := store.Upload(ctx, "ns1", "sub1", "file.txt", data)
	require.NoError(t, err)

	// Act
	downloaded, err := store.Download(ctx, path)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, data, downloaded)
}

func TestLocalStore_Download_NotFound(t *testing.T) {
	// Arrange
	baseDir := t.TempDir()
	store := artifact.NewLocalStore(artifact.LocalConfig{BaseDir: baseDir})
	ctx := context.Background()

	// Act
	_, err := store.Download(ctx, filepath.Join(baseDir, "nonexistent", "file.txt"))

	// Assert
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "not found"),
		"error should contain 'not found', got: %s", err.Error())
}

func TestLocalStore_Reader_Success(t *testing.T) {
	// Arrange
	baseDir := t.TempDir()
	store := artifact.NewLocalStore(artifact.LocalConfig{BaseDir: baseDir})
	ctx := context.Background()
	data := []byte("stream this content")

	path, err := store.Upload(ctx, "ns1", "sub1", "stream.txt", data)
	require.NoError(t, err)

	// Act
	reader, err := store.Reader(ctx, path)
	require.NoError(t, err)
	defer reader.Close()

	content, err := io.ReadAll(reader)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, data, content)
}

func TestLocalStore_Reader_NotFound(t *testing.T) {
	// Arrange
	baseDir := t.TempDir()
	store := artifact.NewLocalStore(artifact.LocalConfig{BaseDir: baseDir})
	ctx := context.Background()

	// Act
	_, err := store.Reader(ctx, filepath.Join(baseDir, "nonexistent", "file.txt"))

	// Assert
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "not found"),
		"error should contain 'not found', got: %s", err.Error())
}

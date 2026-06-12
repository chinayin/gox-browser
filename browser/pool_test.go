package browser_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chinayin/gox-browser/browser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPoolConfig() browser.PoolConfig {
	return browser.PoolConfig{
		MaxInstances:    8,
		IdleTimeout:     5 * time.Minute,
		AcquireTimeout:  30 * time.Second,
		HealthCheckFreq: 1 * time.Hour, // large value to avoid background interference
	}
}

func TestPool_Register(t *testing.T) {
	// Arrange
	pool := browser.NewPool(newTestPoolConfig())
	defer pool.Close()

	provider := newMockProvider(browser.TypeSurf)

	// Act
	pool.Register(provider)

	// Assert
	assert.True(t, pool.HasProvider(browser.TypeSurf))
	assert.False(t, pool.HasProvider(browser.TypeRodHeadless))
}

func TestPool_Acquire_Success(t *testing.T) {
	// Arrange
	pool := browser.NewPool(newTestPoolConfig())
	defer pool.Close()

	provider := newMockProvider(browser.TypeSurf)
	pool.Register(provider)

	ctx := context.Background()

	// Act
	b, err := pool.Acquire(ctx, browser.AcquireOpts{Type: browser.TypeSurf})

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, b)
	assert.Equal(t, browser.TypeSurf, b.BrowserType())

	// Cleanup
	_ = pool.Release(b, nil)
}

func TestPool_Acquire_NoProvider(t *testing.T) {
	// Arrange
	pool := browser.NewPool(newTestPoolConfig())
	defer pool.Close()

	ctx := context.Background()

	// Act
	_, err := pool.Acquire(ctx, browser.AcquireOpts{Type: browser.TypeRodHeadless})

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, browser.ErrProviderNotFound,
		"error should wrap ErrProviderNotFound, got: %v", err)
}

func TestPool_Acquire_PoolClosed(t *testing.T) {
	// Arrange
	pool := browser.NewPool(newTestPoolConfig())
	provider := newMockProvider(browser.TypeSurf)
	pool.Register(provider)

	// Close pool first
	err := pool.Close()
	require.NoError(t, err)

	ctx := context.Background()

	// Act
	_, err = pool.Acquire(ctx, browser.AcquireOpts{Type: browser.TypeSurf})

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, browser.ErrPoolClosed,
		"error should be ErrPoolClosed, got: %v", err)
}

func TestPool_Release_Normal(t *testing.T) {
	// Arrange
	pool := browser.NewPool(newTestPoolConfig())
	defer pool.Close()

	provider := newMockProvider(browser.TypeSurf)
	pool.Register(provider)

	ctx := context.Background()
	b, err := pool.Acquire(ctx, browser.AcquireOpts{Type: browser.TypeSurf})
	require.NoError(t, err)

	// Act
	err = pool.Release(b, nil)

	// Assert
	require.NoError(t, err)
	stats := pool.Stats()
	assert.Equal(t, 1, stats.Available)
	assert.Equal(t, 0, stats.InUse)
}

func TestPool_Release_ConnectionError(t *testing.T) {
	// Arrange
	pool := browser.NewPool(newTestPoolConfig())
	defer pool.Close()

	provider := newMockProvider(browser.TypeSurf)
	pool.Register(provider)

	ctx := context.Background()
	b, err := pool.Acquire(ctx, browser.AcquireOpts{Type: browser.TypeSurf})
	require.NoError(t, err)

	// Act - release with a connection error
	connErr := errors.New("connection reset by peer")
	err = pool.Release(b, connErr)

	// Assert
	require.NoError(t, err)
	stats := pool.Stats()
	assert.Equal(t, 0, stats.Available, "unhealthy instance should not be returned to pool")
	assert.Equal(t, 0, stats.InUse)
	assert.Equal(t, 0, stats.Total)
}

func TestPool_Stats(t *testing.T) {
	// Arrange
	pool := browser.NewPool(newTestPoolConfig())
	defer pool.Close()

	provider := newMockProvider(browser.TypeSurf)
	pool.Register(provider)

	ctx := context.Background()

	// Acquire 2 instances
	b1, err := pool.Acquire(ctx, browser.AcquireOpts{Type: browser.TypeSurf})
	require.NoError(t, err)
	b2, err := pool.Acquire(ctx, browser.AcquireOpts{Type: browser.TypeSurf})
	require.NoError(t, err)

	// Release 1
	err = pool.Release(b1, nil)
	require.NoError(t, err)

	// Act
	stats := pool.Stats()

	// Assert
	assert.Equal(t, 2, stats.Total)
	assert.Equal(t, 1, stats.InUse)
	assert.Equal(t, 1, stats.Available)

	// Cleanup
	_ = pool.Release(b2, nil)
}

func TestPool_Close(t *testing.T) {
	// Arrange
	pool := browser.NewPool(newTestPoolConfig())

	closeCalled := false
	provider := newMockProvider(browser.TypeSurf)
	provider.createFunc = func(ctx context.Context, opts browser.AcquireOpts) (browser.Browser, error) {
		mb := newMockBrowser(browser.TypeSurf)
		mb.closeFunc = func() error {
			closeCalled = true
			return nil
		}
		return mb, nil
	}
	pool.Register(provider)

	ctx := context.Background()
	b, err := pool.Acquire(ctx, browser.AcquireOpts{Type: browser.TypeSurf})
	require.NoError(t, err)

	// Release to pool so it becomes idle
	err = pool.Release(b, nil)
	require.NoError(t, err)

	// Act
	err = pool.Close()

	// Assert
	require.NoError(t, err)
	assert.True(t, closeCalled, "idle instance should be closed on pool close")

	// Verify pool is closed
	_, err = pool.Acquire(ctx, browser.AcquireOpts{Type: browser.TypeSurf})
	assert.ErrorIs(t, err, browser.ErrPoolClosed)
}

func TestPool_Acquire_ReuseIdle(t *testing.T) {
	// Arrange
	pool := browser.NewPool(newTestPoolConfig())
	defer pool.Close()

	createCount := 0
	provider := newMockProvider(browser.TypeSurf)
	provider.createFunc = func(ctx context.Context, opts browser.AcquireOpts) (browser.Browser, error) {
		createCount++
		return newMockBrowser(browser.TypeSurf), nil
	}
	pool.Register(provider)

	ctx := context.Background()

	// Acquire and release
	b1, err := pool.Acquire(ctx, browser.AcquireOpts{Type: browser.TypeSurf})
	require.NoError(t, err)
	err = pool.Release(b1, nil)
	require.NoError(t, err)

	// Act - acquire again should reuse
	b2, err := pool.Acquire(ctx, browser.AcquireOpts{Type: browser.TypeSurf})
	require.NoError(t, err)

	// Assert
	assert.Equal(t, 1, createCount, "should reuse idle instance, not create new one")
	assert.NotNil(t, b2)

	// Cleanup
	_ = pool.Release(b2, nil)
}

func TestPool_IsConnectionError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{name: "nil_error", err: nil, expected: false},
		{name: "normal_error", err: errors.New("some error"), expected: false},
		{name: "connection_reset", err: errors.New("connection reset by peer"), expected: true},
		{name: "broken_pipe", err: errors.New("broken pipe"), expected: true},
		{name: "connection_refused", err: errors.New("connection refused"), expected: true},
		{name: "io_timeout", err: errors.New("i/o timeout"), expected: true},
		{name: "unexpected_eof", err: errors.New("unexpected EOF"), expected: true},
		{name: "closed_network", err: errors.New("use of closed network connection"), expected: true},
		{name: "contains_pattern", err: errors.New("error: connection reset by peer happened"), expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := browser.IsConnectionError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPool_WAFBlockedError(t *testing.T) {
	// Arrange
	wafErr := &browser.WAFBlockedError{
		Result: browser.BlockResult{
			Blocked: true,
			Reason:  "cloudflare detected",
			Type:    "cloudflare",
		},
		URL: "http://example.com",
	}

	// Assert
	require.ErrorIs(t, wafErr, browser.ErrWAFBlocked)
	assert.Contains(t, wafErr.Error(), "cloudflare")
	assert.Contains(t, wafErr.Error(), "http://example.com")
}

package browser_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chinayin/gox-browser/browser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newWorkerTestPool(t *testing.T) *browser.Pool {
	t.Helper()

	cfg := browser.PoolConfig{
		MaxInstances:    8,
		IdleTimeout:     5 * time.Minute,
		AcquireTimeout:  30 * time.Second,
		HealthCheckFreq: 1 * time.Hour,
	}
	pool := browser.NewPool(cfg)

	// Create a mock provider that returns browsers with normal HTML
	provider := newMockProvider(browser.TypeSurf)
	provider.createFunc = func(ctx context.Context, opts browser.AcquireOpts) (browser.Browser, error) {
		mb := newMockBrowser(browser.TypeSurf)
		normalHTML := "<html><body>" + strings.Repeat("Normal page content. ", 20) + "</body></html>"
		mb.htmlFunc = func(ctx context.Context) (string, error) {
			return normalHTML, nil
		}
		mb.titleFunc = func(ctx context.Context) (string, error) {
			return "Normal Page", nil
		}
		return mb, nil
	}
	pool.Register(provider)

	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestWorker_Run_Success(t *testing.T) {
	// Arrange
	pool := newWorkerTestPool(t)
	detector := newMockBlockDetector()
	worker := browser.NewWorker(pool, detector, browser.WorkerConfig{Concurrency: 2})

	tasks := []browser.Task{
		{URL: "http://example.com/1", Chain: []browser.Type{browser.TypeSurf}},
		{URL: "http://example.com/2", Chain: []browser.Type{browser.TypeSurf}},
		{URL: "http://example.com/3", Chain: []browser.Type{browser.TypeSurf}},
	}

	ctx := context.Background()

	// Act
	results := worker.Run(ctx, tasks)

	// Assert
	require.Len(t, results, 3)
	for i, r := range results {
		assert.NoError(t, r.Err, "task %d should succeed", i)
		assert.NotNil(t, r.FetchResult, "task %d should have FetchResult", i)
		assert.Equal(t, tasks[i].URL, r.URL)
		assert.Greater(t, r.Duration, time.Duration(0))
	}
}

func TestWorker_Run_ContextCancel(t *testing.T) {
	// Arrange
	pool := newWorkerTestPool(t)
	detector := newMockBlockDetector()
	worker := browser.NewWorker(pool, detector, browser.WorkerConfig{Concurrency: 1})

	tasks := []browser.Task{
		{URL: "http://example.com/1", Chain: []browser.Type{browser.TypeSurf}},
		{URL: "http://example.com/2", Chain: []browser.Type{browser.TypeSurf}},
		{URL: "http://example.com/3", Chain: []browser.Type{browser.TypeSurf}},
	}

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Act
	results := worker.Run(ctx, tasks)

	// Assert
	require.Len(t, results, 3)

	// At least some tasks should have context.Canceled error
	canceledCount := 0
	for _, r := range results {
		if r.Err != nil && errors.Is(r.Err, context.Canceled) {
			canceledCount++
		}
	}
	assert.Greater(t, canceledCount, 0, "at least some tasks should be canceled")
}

func TestWorker_Run_EmptyTasks(t *testing.T) {
	// Arrange
	pool := newWorkerTestPool(t)
	detector := newMockBlockDetector()
	worker := browser.NewWorker(pool, detector, browser.DefaultWorkerConfig)

	ctx := context.Background()

	// Act
	results := worker.Run(ctx, []browser.Task{})

	// Assert
	assert.Empty(t, results)
}

func TestWorker_Run_WithCallback(t *testing.T) {
	// Arrange
	pool := newWorkerTestPool(t)
	detector := newMockBlockDetector()

	var mu sync.Mutex
	callbackResults := make(map[int]browser.TaskResult)
	cfg := browser.WorkerConfig{
		Concurrency: 2,
		OnResult: func(idx int, result browser.TaskResult) {
			mu.Lock()
			callbackResults[idx] = result
			mu.Unlock()
		},
	}
	worker := browser.NewWorker(pool, detector, cfg)

	tasks := []browser.Task{
		{URL: "http://example.com/1", Chain: []browser.Type{browser.TypeSurf}},
		{URL: "http://example.com/2", Chain: []browser.Type{browser.TypeSurf}},
	}

	ctx := context.Background()

	// Act
	results := worker.Run(ctx, tasks)

	// Assert
	require.Len(t, results, 2)
	mu.Lock()
	assert.Len(t, callbackResults, 2, "callback should be called for each task")
	mu.Unlock()
}

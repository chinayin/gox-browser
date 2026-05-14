package browser_test

import (
	"testing"

	"github.com/chinayin/gox-browser/browser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFetcher_Success(t *testing.T) {
	pool := browser.NewPool(browser.DefaultPoolConfig)
	defer pool.Close()

	pool.Register(newMockProvider(browser.TypeRodHeadless))

	f, err := browser.NewFetcher(pool, newMockBlockDetector(),
		browser.WithPrimary(browser.TypeRodHeadless),
	)

	require.NoError(t, err)
	assert.NotNil(t, f)
}

func TestNewFetcher_PrimaryNotRegistered(t *testing.T) {
	pool := browser.NewPool(browser.DefaultPoolConfig)
	defer pool.Close()

	// Don't register any provider
	f, err := browser.NewFetcher(pool, newMockBlockDetector(),
		browser.WithPrimary(browser.TypeRodHeadless),
	)

	assert.Error(t, err)
	assert.Nil(t, f)
	assert.Contains(t, err.Error(), "primary provider")
}

func TestNewFetcher_FallbackNotRegistered(t *testing.T) {
	pool := browser.NewPool(browser.DefaultPoolConfig)
	defer pool.Close()

	pool.Register(newMockProvider(browser.TypeRodHeadless))
	// TypeSurf is NOT registered

	f, err := browser.NewFetcher(pool, newMockBlockDetector(),
		browser.WithPrimary(browser.TypeRodHeadless),
		browser.WithFallback(browser.TypeSurf),
	)

	// Fetcher is created successfully, unregistered fallback is filtered out
	require.NoError(t, err)
	assert.NotNil(t, f)
}

func TestFetcher_SetPrimary(t *testing.T) {
	pool := browser.NewPool(browser.DefaultPoolConfig)
	defer pool.Close()

	pool.Register(newMockProvider(browser.TypeRodHeadless))
	pool.Register(newMockProvider(browser.TypeSurf))

	f, err := browser.NewFetcher(pool, newMockBlockDetector(),
		browser.WithPrimary(browser.TypeRodHeadless),
	)
	require.NoError(t, err)

	// Switch primary to surf — no panic, no error
	f.SetPrimary(browser.TypeSurf)
}

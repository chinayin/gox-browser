package browser_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/chinayin/gox-browser/browser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNavigateAndCheck_Success(t *testing.T) {
	// Arrange
	normalHTML := "<html><body>" + strings.Repeat("Normal page content. ", 20) + "</body></html>"

	mb := newMockBrowser(browser.TypeSurf)
	mb.navigateFunc = func(ctx context.Context, url string) error {
		return nil
	}
	mb.evalFunc = func(ctx context.Context, js string) (string, error) {
		// CheckWAF calls Eval to get outerHTML, and NavigateAndCheck calls Eval for title/URL
		if strings.Contains(js, "outerHTML") {
			return normalHTML, nil
		}
		if strings.Contains(js, "document.title") {
			return "Normal Page", nil
		}
		if strings.Contains(js, "location.href") {
			return "http://example.com/page", nil
		}
		return "", nil
	}
	mb.titleFunc = func(ctx context.Context) (string, error) {
		return "Normal Page", nil
	}
	mb.urlFunc = func(ctx context.Context) (string, error) {
		return "http://example.com/page", nil
	}

	detector := browser.NewBlockDetector()
	ctx := context.Background()

	// Act
	result, err := browser.NavigateAndCheck(ctx, mb, "http://example.com/page", detector, browser.WithEarlyWait(0))

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "http://example.com/page", result.URL)
	assert.Equal(t, "Normal Page", result.Title)
	assert.Greater(t, result.Elapsed, int64(0)*1)
}

func TestNavigateAndCheck_WAFBlocked(t *testing.T) {
	// Arrange
	cfHTML := "<html><body>" + strings.Repeat("x", 100) + "Just a moment... Checking your browser</body></html>"

	mb := newMockBrowser(browser.TypeSurf)
	mb.navigateFunc = func(ctx context.Context, url string) error {
		return nil
	}
	mb.evalFunc = func(ctx context.Context, js string) (string, error) {
		if strings.Contains(js, "outerHTML") {
			return cfHTML, nil
		}
		return "", nil
	}
	mb.titleFunc = func(ctx context.Context) (string, error) {
		return "Just a moment...", nil
	}
	mb.urlFunc = func(ctx context.Context) (string, error) {
		return "http://example.com/blocked", nil
	}

	detector := browser.NewBlockDetector()
	ctx := context.Background()

	// Act
	result, err := browser.NavigateAndCheck(ctx, mb, "http://example.com/blocked", detector, browser.WithEarlyWait(0))

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	require.ErrorIs(t, err, browser.ErrWAFBlocked,
		"error should wrap ErrWAFBlocked, got: %v", err)

	var wafErr *browser.WAFBlockedError
	require.ErrorAs(t, err, &wafErr)
	assert.Equal(t, "cloudflare", wafErr.Result.Type)
}

func TestNavigateAndCheck_NavigateError(t *testing.T) {
	// Arrange
	navErr := errors.New("network timeout")

	mb := newMockBrowser(browser.TypeSurf)
	mb.navigateFunc = func(ctx context.Context, url string) error {
		return navErr
	}

	detector := browser.NewBlockDetector()
	ctx := context.Background()

	// Act
	result, err := browser.NavigateAndCheck(ctx, mb, "http://example.com/fail", detector, browser.WithEarlyWait(0))

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "network timeout")
}

func TestNavigateAndCheck_WithWaitStable(t *testing.T) {
	// Arrange
	normalHTML := "<html><body>" + strings.Repeat("Normal page content. ", 20) + "</body></html>"

	waitStableCalled := false
	mb := newMockBrowser(browser.TypeSurf)
	mb.navigateFunc = func(ctx context.Context, url string) error {
		return nil
	}
	mb.waitStableFunc = func(ctx context.Context) error {
		waitStableCalled = true
		return nil
	}
	mb.evalFunc = func(ctx context.Context, js string) (string, error) {
		if strings.Contains(js, "outerHTML") {
			return normalHTML, nil
		}
		if strings.Contains(js, "document.title") {
			return "Page Title", nil
		}
		if strings.Contains(js, "location.href") {
			return "http://example.com/stable", nil
		}
		return "", nil
	}
	mb.titleFunc = func(ctx context.Context) (string, error) {
		return "Page Title", nil
	}
	mb.urlFunc = func(ctx context.Context) (string, error) {
		return "http://example.com/stable", nil
	}

	detector := browser.NewBlockDetector()
	ctx := context.Background()

	// Act
	result, err := browser.NavigateAndCheck(ctx, mb, "http://example.com/stable", detector,
		browser.WithEarlyWait(0),
		browser.WithWaitStable(),
	)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, waitStableCalled, "WaitStable should be called with WithWaitStable option")
}

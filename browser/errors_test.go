package browser_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/chinayin/gox-browser/browser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsConnectionError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "use of closed network connection",
			err:      errors.New("read tcp: use of closed network connection"),
			expected: true,
		},
		{
			name:     "unexpected EOF",
			err:      errors.New("unexpected EOF"),
			expected: true,
		},
		{
			name:     "broken pipe",
			err:      errors.New("write: broken pipe"),
			expected: true,
		},
		{
			name:     "connection reset by peer",
			err:      errors.New("read tcp: connection reset by peer"),
			expected: true,
		},
		{
			name:     "connection refused",
			err:      errors.New("dial tcp: connection refused"),
			expected: true,
		},
		{
			name:     "i/o timeout",
			err:      errors.New("dial tcp: i/o timeout"),
			expected: true,
		},
		{
			name:     "rod create panic",
			err:      errors.New("rod create panic: something went wrong"),
			expected: true,
		},
		{
			name:     "unrelated error",
			err:      errors.New("page not found"),
			expected: false,
		},
		{
			name:     "wrapped error containing pattern",
			err:      fmt.Errorf("browser: connect failed: %w", errors.New("connection refused")),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := browser.IsConnectionError(tt.err)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWAFBlockedError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *browser.WAFBlockedError
		contains string
	}{
		{
			name: "with URL",
			err: &browser.WAFBlockedError{
				Result: browser.BlockResult{Type: "cloudflare", Reason: "challenge page"},
				URL:    "https://example.com/page",
			},
			contains: "url=https://example.com/page",
		},
		{
			name: "without URL",
			err: &browser.WAFBlockedError{
				Result: browser.BlockResult{Type: "datadome", Reason: "captcha detected"},
				URL:    "",
			},
			contains: "datadome: captcha detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			msg := tt.err.Error()

			// Assert
			assert.Contains(t, msg, "browser: waf blocked")
			assert.Contains(t, msg, tt.contains)
		})
	}
}

func TestWAFBlockedError_Unwrap(t *testing.T) {
	// Arrange
	wafErr := &browser.WAFBlockedError{
		Result: browser.BlockResult{Type: "cloudflare", Reason: "blocked"},
		URL:    "https://example.com",
	}

	// Act & Assert
	assert.Equal(t, browser.ErrWAFBlocked, wafErr.Unwrap())
}

func TestWAFBlockedError_ErrorsIs(t *testing.T) {
	// Arrange
	wafErr := &browser.WAFBlockedError{
		Result: browser.BlockResult{Type: "cloudflare", Reason: "blocked"},
		URL:    "https://example.com",
	}

	// Act & Assert
	require.ErrorIs(t, wafErr, browser.ErrWAFBlocked)
	require.NotErrorIs(t, wafErr, browser.ErrPoolClosed)
}

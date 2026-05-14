package browser_test

import (
	"slices"
	"testing"

	"github.com/chinayin/gox-browser/browser"
	"github.com/stretchr/testify/assert"
)

func TestTruncateUA(t *testing.T) {
	tests := []struct {
		name     string
		ua       string
		maxLen   int
		expected string
	}{
		{
			name:     "short string unchanged",
			ua:       "Mozilla/5.0",
			maxLen:   60,
			expected: "Mozilla/5.0",
		},
		{
			name:     "exact length unchanged",
			ua:       "abcdef",
			maxLen:   6,
			expected: "abcdef",
		},
		{
			name:     "long string truncated with ellipsis",
			ua:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
			maxLen:   30,
			expected: "Mozilla/5.0 (Windows NT 10.0; ...",
		},
		{
			name:     "maxLen 0 defaults to 60",
			ua:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
			maxLen:   0,
			expected: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := browser.TruncateUA(tt.ua, tt.maxLen)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPlatformFromUA(t *testing.T) {
	tests := []struct {
		name     string
		ua       string
		expected string
	}{
		{
			name:     "Macintosh UA",
			ua:       "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
			expected: "MacIntel",
		},
		{
			name:     "Linux UA",
			ua:       "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
			expected: "Linux x86_64",
		},
		{
			name:     "Windows UA",
			ua:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			expected: "Win32",
		},
		{
			name:     "empty string defaults to Win32",
			ua:       "",
			expected: "Win32",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := browser.PlatformFromUA(tt.ua)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPlatformOverrideScript(t *testing.T) {
	tests := []struct {
		name     string
		platform string
	}{
		{name: "MacIntel", platform: "MacIntel"},
		{name: "Win32", platform: "Win32"},
		{name: "Linux x86_64", platform: "Linux x86_64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			script := browser.PlatformOverrideScript(tt.platform)

			// Assert
			assert.Contains(t, script, tt.platform)
			assert.Contains(t, script, "navigator")
			assert.Contains(t, script, "platform")
		})
	}
}

func TestRandomUserAgent(t *testing.T) {
	// Act
	ua := browser.RandomUserAgent()

	// Assert
	assert.NotEmpty(t, ua)
	assert.True(t, slices.Contains(browser.UserAgents, ua),
		"returned UA should be from predefined list")
}

func TestRandomViewport(t *testing.T) {
	// Act
	vp := browser.RandomViewport()

	// Assert
	assert.Greater(t, vp.Width, 0)
	assert.Greater(t, vp.Height, 0)
	assert.Greater(t, vp.Scale, 0.0)
	assert.True(t, slices.Contains(browser.Viewports, vp),
		"returned viewport should be from predefined list")
}

func TestRandomTimezone(t *testing.T) {
	// Act
	tz := browser.RandomTimezone()

	// Assert
	assert.NotEmpty(t, tz)
	assert.True(t, slices.Contains(browser.Timezones, tz),
		"returned timezone should be from predefined list")
}

func TestRandomLocale(t *testing.T) {
	// Act
	locale := browser.RandomLocale()

	// Assert
	assert.NotEmpty(t, locale)
	assert.True(t, slices.Contains(browser.Locales, locale),
		"returned locale should be from predefined list")
}

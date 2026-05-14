package browserless

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEndpointURL(t *testing.T) {
	tests := []struct {
		name          string
		rawURL        string
		expectedBase  string
		expectedToken string
	}{
		{
			name:          "URL with token param",
			rawURL:        "http://localhost:3000?token=abc123",
			expectedBase:  "http://localhost:3000",
			expectedToken: "abc123",
		},
		{
			name:          "URL without token",
			rawURL:        "http://localhost:3000",
			expectedBase:  "http://localhost:3000",
			expectedToken: "",
		},
		{
			name:          "HTTPS URL with token",
			rawURL:        "https://browserless.example.com?token=secret-key",
			expectedBase:  "https://browserless.example.com",
			expectedToken: "secret-key",
		},
		{
			name:          "URL with path and token",
			rawURL:        "http://node1:3000/path?token=mytoken",
			expectedBase:  "http://node1:3000/path",
			expectedToken: "mytoken",
		},
		{
			name:          "malformed URL returns raw",
			rawURL:        "://invalid",
			expectedBase:  "://invalid",
			expectedToken: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			baseURL, token := parseEndpointURL(tt.rawURL)

			// Assert
			assert.Equal(t, tt.expectedBase, baseURL)
			assert.Equal(t, tt.expectedToken, token)
		})
	}
}

func TestProvider_BuildWSURL(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		ep       *Endpoint
		contains []string
	}{
		{
			name: "with token",
			cfg:  Config{},
			ep:   &Endpoint{URL: "http://localhost:3000", Token: "abc123"},
			contains: []string{
				"ws://localhost:3000/chromium",
				"token=abc123",
			},
		},
		{
			name: "with stealth",
			cfg:  Config{Stealth: true},
			ep:   &Endpoint{URL: "http://localhost:3000", Token: ""},
			contains: []string{
				"ws://localhost:3000/chromium",
				"stealth=true",
			},
		},
		{
			name: "with blockAds",
			cfg:  Config{BlockAds: true},
			ep:   &Endpoint{URL: "http://localhost:3000", Token: ""},
			contains: []string{
				"ws://localhost:3000/chromium",
				"blockAds=true",
			},
		},
		{
			name: "with proxy",
			cfg:  Config{Proxy: "socks5://proxy:1080"},
			ep:   &Endpoint{URL: "http://localhost:3000", Token: ""},
			contains: []string{
				"ws://localhost:3000/chromium",
				"launch=",
				"proxy-server",
			},
		},
		{
			name: "headless false",
			cfg:  Config{Headless: boolPtr(false)},
			ep:   &Endpoint{URL: "http://localhost:3000", Token: ""},
			contains: []string{
				"ws://localhost:3000/chromium",
				"headless=false",
			},
		},
		{
			name: "HTTPS converts to WSS",
			cfg:  Config{},
			ep:   &Endpoint{URL: "https://browserless.example.com", Token: "key"},
			contains: []string{
				"wss://browserless.example.com/chromium",
				"token=key",
			},
		},
		{
			name: "combination of options",
			cfg:  Config{Stealth: true, BlockAds: true, Headless: boolPtr(false)},
			ep:   &Endpoint{URL: "http://localhost:3000", Token: "tok"},
			contains: []string{
				"ws://localhost:3000/chromium",
				"token=tok",
				"stealth=true",
				"blockAds=true",
				"headless=false",
			},
		},
		{
			name:     "no params produces clean URL",
			cfg:      Config{},
			ep:       &Endpoint{URL: "http://localhost:3000", Token: ""},
			contains: []string{"ws://localhost:3000/chromium"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			p := &Provider{cfg: tt.cfg}

			// Act
			wsURL := p.buildWSURL(tt.ep)

			// Assert
			for _, s := range tt.contains {
				assert.Contains(t, wsURL, s)
			}
		})
	}
}

func TestProvider_BuildWSURL_NoParams(t *testing.T) {
	// Arrange
	p := &Provider{cfg: Config{}}
	ep := &Endpoint{URL: "http://localhost:3000", Token: ""}

	// Act
	wsURL := p.buildWSURL(ep)

	// Assert - should not contain "?"
	assert.Equal(t, "ws://localhost:3000/chromium", wsURL)
}

func TestProvider_BuildWSURL_ProxyDecoded(t *testing.T) {
	// Arrange
	p := &Provider{cfg: Config{Proxy: "socks5://proxy:1080"}}
	ep := &Endpoint{URL: "http://localhost:3000", Token: ""}

	// Act
	wsURL := p.buildWSURL(ep)

	// Assert - parse the URL and decode the launch param
	u, err := url.Parse(wsURL)
	require.NoError(t, err)
	launchParam := u.Query().Get("launch")
	assert.Contains(t, launchParam, "--proxy-server=socks5://proxy:1080")
}

func boolPtr(b bool) *bool {
	return &b
}

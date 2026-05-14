package rod

import (
	"testing"

	browser "github.com/chinayin/gox-browser/browser"
	"github.com/stretchr/testify/assert"
)

func TestIsAllowedScriptDomain(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		allowed []string
		want    bool
	}{
		{
			name:    "exact match",
			host:    "example.com",
			allowed: []string{"example.com"},
			want:    true,
		},
		{
			name:    "subdomain match",
			host:    "sub.example.com",
			allowed: []string{"example.com"},
			want:    true,
		},
		{
			name:    "no match",
			host:    "other.com",
			allowed: []string{"example.com"},
			want:    false,
		},
		{
			name:    "empty allowed list",
			host:    "example.com",
			allowed: []string{},
			want:    false,
		},
		{
			name:    "partial match is not subdomain",
			host:    "notexample.com",
			allowed: []string{"example.com"},
			want:    false,
		},
		{
			name:    "multiple allowed domains",
			host:    "cdn.assets.io",
			allowed: []string{"example.com", "assets.io"},
			want:    true,
		},
		{
			name:    "deep subdomain match",
			host:    "a.b.c.example.com",
			allowed: []string{"example.com"},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAllowedScriptDomain(tt.host, tt.allowed)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestProvider_Type(t *testing.T) {
	tests := []struct {
		name     string
		headless bool
		want     browser.Type
	}{
		{
			name:     "headless returns TypeRodHeadless",
			headless: true,
			want:     browser.TypeRodHeadless,
		},
		{
			name:     "headed returns TypeRodHeaded",
			headless: false,
			want:     browser.TypeRodHeaded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProvider(Config{Headless: tt.headless})
			assert.Equal(t, tt.want, p.Type())
		})
	}
}

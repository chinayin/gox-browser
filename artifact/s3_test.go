package artifact

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewS3Store_PrefixHandling(t *testing.T) {
	tests := []struct {
		name       string
		prefix     string
		wantPrefix string
	}{
		{
			name:       "prefix with trailing slash stays unchanged",
			prefix:     "artifacts/",
			wantPrefix: "artifacts/",
		},
		{
			name:       "prefix without trailing slash gets one appended",
			prefix:     "artifacts",
			wantPrefix: "artifacts/",
		},
		{
			name:       "empty prefix stays empty",
			prefix:     "",
			wantPrefix: "",
		},
		{
			name:       "nested prefix without slash",
			prefix:     "data/scraper",
			wantPrefix: "data/scraper/",
		},
		{
			name:       "nested prefix with slash",
			prefix:     "data/scraper/",
			wantPrefix: "data/scraper/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewS3Store(S3Config{
				Endpoint:  "https://s3.example.com",
				Bucket:    "test-bucket",
				Region:    "us-east-1",
				AccessKey: "test-key",
				SecretKey: "test-secret",
				Prefix:    tt.prefix,
			})

			assert.Equal(t, tt.wantPrefix, store.prefix)
		})
	}
}

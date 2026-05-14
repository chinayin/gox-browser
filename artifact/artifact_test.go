package artifact_test

import (
	"testing"

	"github.com/chinayin/gox-browser/artifact"
	"github.com/stretchr/testify/assert"
)

func TestBuildKey(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		expected string
	}{
		{
			name:     "empty parts",
			parts:    []string{},
			expected: "",
		},
		{
			name:     "single part",
			parts:    []string{"screenshots"},
			expected: "screenshots",
		},
		{
			name:     "multiple parts",
			parts:    []string{"task-123", "step_01", "screenshot.png"},
			expected: "task-123/step_01/screenshot.png",
		},
		{
			name:     "parts with spaces are trimmed",
			parts:    []string{" task-123 ", " step_01 ", " file.png "},
			expected: "task-123/step_01/file.png",
		},
		{
			name:     "all empty parts",
			parts:    []string{"", "  ", "", " "},
			expected: "",
		},
		{
			name:     "mixed empty and non-empty",
			parts:    []string{"", "namespace", "", "file.json"},
			expected: "namespace/file.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := artifact.BuildKey(tt.parts...)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetectContentType(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{name: "png", filename: "screenshot.png", expected: "image/png"},
		{name: "jpg", filename: "photo.jpg", expected: "image/jpeg"},
		{name: "jpeg", filename: "photo.jpeg", expected: "image/jpeg"},
		{name: "gif", filename: "animation.gif", expected: "image/gif"},
		{name: "webp", filename: "image.webp", expected: "image/webp"},
		{name: "json", filename: "data.json", expected: "application/json"},
		{name: "html", filename: "page.html", expected: "text/html"},
		{name: "htm", filename: "page.htm", expected: "text/html"},
		{name: "txt", filename: "readme.txt", expected: "text/plain"},
		{name: "css", filename: "style.css", expected: "text/css"},
		{name: "js", filename: "app.js", expected: "application/javascript"},
		{name: "xml", filename: "feed.xml", expected: "application/xml"},
		{name: "pdf", filename: "document.pdf", expected: "application/pdf"},
		{name: "csv", filename: "data.csv", expected: "text/csv"},
		{name: "yaml", filename: "config.yaml", expected: "application/x-yaml"},
		{name: "yml", filename: "config.yml", expected: "application/x-yaml"},
		{name: "unknown extension", filename: "file.xyz", expected: "application/octet-stream"},
		{name: "no extension", filename: "Makefile", expected: "application/octet-stream"},
		{name: "uppercase extension", filename: "IMAGE.PNG", expected: "image/png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := artifact.DetectContentType(tt.filename)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

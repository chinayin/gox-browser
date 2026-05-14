package browser_test

import (
	"testing"

	"github.com/chinayin/gox-browser/browser"
	"github.com/stretchr/testify/assert"
)

func TestType_String(t *testing.T) {
	tests := []struct {
		name     string
		typ      browser.Type
		expected string
	}{
		{name: "rod-headless", typ: browser.TypeRodHeadless, expected: "rod-headless"},
		{name: "rod-headed", typ: browser.TypeRodHeaded, expected: "rod-headed"},
		{name: "browserless", typ: browser.TypeBrowserless, expected: "browserless"},
		{name: "surf", typ: browser.TypeSurf, expected: "surf"},
		{name: "camoufox", typ: browser.TypeCamoufox, expected: "camoufox"},
		{name: "unknown type", typ: browser.Type(99), expected: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := tt.typ.String()

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

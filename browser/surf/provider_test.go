package surf_test

import (
	"context"
	"testing"

	"github.com/chinayin/gox-browser/browser"
	"github.com/chinayin/gox-browser/browser/surf"
	"github.com/stretchr/testify/assert"
)

func TestProvider_Type(t *testing.T) {
	p := surf.NewProvider(surf.Config{})
	assert.Equal(t, browser.TypeSurf, p.Type())
}

func TestProvider_HealthCheck(t *testing.T) {
	p := surf.NewProvider(surf.Config{})
	err := p.HealthCheck(context.Background())
	assert.NoError(t, err)
}

func TestProvider_Close(t *testing.T) {
	p := surf.NewProvider(surf.Config{})
	err := p.Close()
	assert.NoError(t, err)
}

package surf_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chinayin/gox-browser/browser"
	"github.com/chinayin/gox-browser/browser/surf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestBrowser_HTML_HonorsContextDeadline(t *testing.T) {
	// 慢服务器：1s 后才响应
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(1 * time.Second)
		_, _ = w.Write([]byte("<html>slow</html>"))
	}))
	defer srv.Close()

	p := surf.NewProvider(surf.Config{})
	b, err := p.Create(context.Background(), browser.AcquireOpts{})
	require.NoError(t, err)
	defer func() { _ = b.Close() }()

	require.NoError(t, b.Navigate(context.Background(), srv.URL))

	// 100ms deadline 必须先于慢响应触发
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = b.HTML(ctx)
	elapsed := time.Since(start)

	require.Error(t, err, "expected context deadline error")
	assert.Less(t, elapsed, 500*time.Millisecond, "HTML should return promptly after ctx deadline, took %v", elapsed)
}

func TestBrowser_Text_PropagatesContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(1 * time.Second)
		_, _ = w.Write([]byte("slow"))
	}))
	defer srv.Close()

	p := surf.NewProvider(surf.Config{})
	b, err := p.Create(context.Background(), browser.AcquireOpts{})
	require.NoError(t, err)
	defer func() { _ = b.Close() }()

	require.NoError(t, b.Navigate(context.Background(), srv.URL))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = b.Text(ctx)
	elapsed := time.Since(start)

	require.Error(t, err, "expected context deadline error")
	assert.Less(t, elapsed, 500*time.Millisecond, "Text should propagate ctx, took %v", elapsed)
}

package surf

import (
	"context"
	"fmt"

	"github.com/enetx/g"
	"github.com/enetx/surf"

	browser "github.com/chinayin/gox-browser/browser"
)

// 编译期接口合规断言
var _ browser.Browser = (*surfBrowser)(nil)

type surfBrowser struct {
	client  *surf.Client
	lastURL string
}

func (b *surfBrowser) Navigate(_ context.Context, url string) error {
	b.lastURL = url
	return nil
}

func (b *surfBrowser) WaitStable(_ context.Context) error { return nil }

func (b *surfBrowser) HTML(_ context.Context) (string, error) {
	if b.lastURL == "" {
		return "", fmt.Errorf("browser: surf no url navigated")
	}

	resp := b.client.Get(g.String(b.lastURL)).Do()
	if resp.IsErr() {
		return "", fmt.Errorf("browser: surf get %q: %w", b.lastURL, resp.Err())
	}

	body := resp.Ok().Body.String()
	if body.IsErr() {
		return "", fmt.Errorf("browser: surf read body: %w", body.Err())
	}

	return body.Ok().Std(), nil
}

func (b *surfBrowser) Text(_ context.Context) (string, error) {
	return b.HTML(context.Background())
}

func (b *surfBrowser) Title(_ context.Context) (string, error) {
	return "", browser.ErrUnsupported
}

func (b *surfBrowser) URL(_ context.Context) (string, error) {
	return b.lastURL, nil
}

func (b *surfBrowser) Screenshot(_ context.Context) ([]byte, error) {
	return nil, browser.ErrUnsupported
}

func (b *surfBrowser) Eval(_ context.Context, _ string) (string, error) {
	return "", browser.ErrUnsupported
}

func (b *surfBrowser) EvalDirect(_ context.Context, _ string) (string, error) {
	return "", browser.ErrUnsupported
}

func (b *surfBrowser) Click(_ context.Context, _ string) error {
	return browser.ErrUnsupported
}

func (b *surfBrowser) Type(_ context.Context, _, _ string) error {
	return browser.ErrUnsupported
}

func (b *surfBrowser) WaitSelector(_ context.Context, _ string) error {
	return browser.ErrUnsupported
}

func (b *surfBrowser) Cookies(_ context.Context) ([]browser.Cookie, error) {
	return nil, browser.ErrUnsupported
}

func (b *surfBrowser) SetCookies(_ context.Context, _ []browser.Cookie) error {
	return browser.ErrUnsupported
}

func (b *surfBrowser) BrowserType() browser.Type { return browser.TypeSurf }

func (b *surfBrowser) Close() error {
	b.client.Close()
	return nil
}

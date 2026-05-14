package browser_test

import (
	"context"

	"github.com/chinayin/gox-browser/browser"
)

// mockBrowser implements browser.Browser for testing
type mockBrowser struct {
	browserType browser.Type

	navigateFunc   func(ctx context.Context, url string) error
	waitStableFunc func(ctx context.Context) error
	htmlFunc       func(ctx context.Context) (string, error)
	textFunc       func(ctx context.Context) (string, error)
	titleFunc      func(ctx context.Context) (string, error)
	urlFunc        func(ctx context.Context) (string, error)
	screenshotFunc func(ctx context.Context) ([]byte, error)
	evalFunc       func(ctx context.Context, js string) (string, error)
	clickFunc      func(ctx context.Context, selector string) error
	typeFunc       func(ctx context.Context, selector, text string) error
	waitSelectorFn func(ctx context.Context, selector string) error
	cookiesFunc    func(ctx context.Context) ([]browser.Cookie, error)
	setCookiesFunc func(ctx context.Context, cookies []browser.Cookie) error
	closeFunc      func() error
}

func newMockBrowser(t browser.Type) *mockBrowser {
	return &mockBrowser{browserType: t}
}

func (m *mockBrowser) Navigate(ctx context.Context, url string) error {
	if m.navigateFunc != nil {
		return m.navigateFunc(ctx, url)
	}
	return nil
}

func (m *mockBrowser) WaitStable(ctx context.Context) error {
	if m.waitStableFunc != nil {
		return m.waitStableFunc(ctx)
	}
	return nil
}

func (m *mockBrowser) HTML(ctx context.Context) (string, error) {
	if m.htmlFunc != nil {
		return m.htmlFunc(ctx)
	}
	return "<html>test</html>", nil
}

func (m *mockBrowser) Text(ctx context.Context) (string, error) {
	if m.textFunc != nil {
		return m.textFunc(ctx)
	}
	return "test", nil
}

func (m *mockBrowser) Title(ctx context.Context) (string, error) {
	if m.titleFunc != nil {
		return m.titleFunc(ctx)
	}
	return "test", nil
}

func (m *mockBrowser) URL(ctx context.Context) (string, error) {
	if m.urlFunc != nil {
		return m.urlFunc(ctx)
	}
	return "http://test.com", nil
}

func (m *mockBrowser) Screenshot(ctx context.Context) ([]byte, error) {
	if m.screenshotFunc != nil {
		return m.screenshotFunc(ctx)
	}
	return nil, browser.ErrUnsupported
}

func (m *mockBrowser) Eval(ctx context.Context, js string) (string, error) {
	if m.evalFunc != nil {
		return m.evalFunc(ctx, js)
	}
	return "", nil
}

func (m *mockBrowser) Click(ctx context.Context, selector string) error {
	if m.clickFunc != nil {
		return m.clickFunc(ctx, selector)
	}
	return nil
}

func (m *mockBrowser) Type(ctx context.Context, selector, text string) error {
	if m.typeFunc != nil {
		return m.typeFunc(ctx, selector, text)
	}
	return nil
}

func (m *mockBrowser) WaitSelector(ctx context.Context, selector string) error {
	if m.waitSelectorFn != nil {
		return m.waitSelectorFn(ctx, selector)
	}
	return nil
}

func (m *mockBrowser) Cookies(ctx context.Context) ([]browser.Cookie, error) {
	if m.cookiesFunc != nil {
		return m.cookiesFunc(ctx)
	}
	return nil, nil
}

func (m *mockBrowser) SetCookies(ctx context.Context, cookies []browser.Cookie) error {
	if m.setCookiesFunc != nil {
		return m.setCookiesFunc(ctx, cookies)
	}
	return nil
}

func (m *mockBrowser) BrowserType() browser.Type {
	return m.browserType
}

func (m *mockBrowser) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

// mockProvider implements browser.Provider for testing
type mockProvider struct {
	browserType browser.Type
	createFunc  func(ctx context.Context, opts browser.AcquireOpts) (browser.Browser, error)
}

func newMockProvider(t browser.Type) *mockProvider {
	return &mockProvider{browserType: t}
}

func (p *mockProvider) Type() browser.Type {
	return p.browserType
}

func (p *mockProvider) Create(ctx context.Context, opts browser.AcquireOpts) (browser.Browser, error) {
	if p.createFunc != nil {
		return p.createFunc(ctx, opts)
	}
	return newMockBrowser(p.browserType), nil
}

func (p *mockProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func (p *mockProvider) Close() error {
	return nil
}

// mockBlockDetector always returns not blocked
type mockBlockDetector struct {
	result browser.BlockResult
}

func newMockBlockDetector() *mockBlockDetector {
	return &mockBlockDetector{result: browser.BlockResult{Blocked: false}}
}

func (d *mockBlockDetector) Detect(html, title string, statusCode int) browser.BlockResult {
	return d.result
}

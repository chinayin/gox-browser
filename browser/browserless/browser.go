package browserless

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	browser "github.com/chinayin/gox-browser/browser"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// 编译期接口合规断言
var _ browser.Browser = (*browserlessBrowser)(nil)

const (
	waitStableInterval = 300 * time.Millisecond
	waitStableMaxWait  = 30 * time.Second
	networkIdleTimeout = 1 * time.Second
	networkIdleMaxWait = 30 * time.Second
)

type browserlessBrowser struct {
	page     *rod.Page
	browser  *rod.Browser
	headless bool
	endpoint string

	isolatedCtxID proto.RuntimeExecutionContextID
}

func (b *browserlessBrowser) Navigate(ctx context.Context, url string) error {
	slog.Debug("browser: browserless navigate start", "url", url, "endpoint", b.endpoint)
	t0 := time.Now()

	b.isolatedCtxID = 0

	wait := b.page.Context(ctx).WaitNavigation(proto.PageLifecycleEventNameDOMContentLoaded)
	if err := b.page.Context(ctx).Navigate(url); err != nil {
		return fmt.Errorf("browser: browserless navigate %q: %w", url, err)
	}
	wait()

	slog.Debug("browser: browserless navigate done",
		"url", url, "elapsed", time.Since(t0).Round(time.Millisecond))
	return nil
}

func (b *browserlessBrowser) WaitStable(ctx context.Context) error {
	stableCtx, cancel := context.WithTimeout(ctx, waitStableMaxWait)
	defer cancel()

	if err := b.page.Context(stableCtx).WaitStable(waitStableInterval); err != nil {
		return fmt.Errorf("browser: browserless wait stable: %w", err)
	}

	if err := b.waitNetworkIdle(ctx); err != nil {
		slog.Debug("browser: browserless network idle timeout, continuing", "error", err)
	}
	return nil
}

func (b *browserlessBrowser) waitNetworkIdle(ctx context.Context) error {
	wait := b.page.Context(ctx).WaitRequestIdle(networkIdleTimeout, nil, nil, nil)

	done := make(chan struct{})
	go func() {
		wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(networkIdleMaxWait):
		return fmt.Errorf("browser: browserless network idle max wait exceeded")
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *browserlessBrowser) HTML(ctx context.Context) (string, error) {
	html, err := b.page.Context(ctx).HTML()
	if err != nil {
		return "", fmt.Errorf("browser: browserless html: %w", err)
	}
	return html, nil
}

func (b *browserlessBrowser) Text(ctx context.Context) (string, error) {
	text, err := b.page.Context(ctx).Eval(`() => document.body.innerText`)
	if err != nil {
		return "", fmt.Errorf("browser: browserless text: %w", err)
	}
	return text.Value.String(), nil
}

func (b *browserlessBrowser) Title(ctx context.Context) (string, error) {
	info, err := b.page.Context(ctx).Info()
	if err != nil {
		return "", fmt.Errorf("browser: browserless title: %w", err)
	}
	return info.Title, nil
}

func (b *browserlessBrowser) URL(ctx context.Context) (string, error) {
	info, err := b.page.Context(ctx).Info()
	if err != nil {
		return "", fmt.Errorf("browser: browserless url: %w", err)
	}
	return info.URL, nil
}

func (b *browserlessBrowser) Screenshot(ctx context.Context) ([]byte, error) {
	data, err := b.page.Context(ctx).Screenshot(true, &proto.PageCaptureScreenshot{
		Format: proto.PageCaptureScreenshotFormatPng,
	})
	if err != nil {
		return nil, fmt.Errorf("browser: browserless screenshot: %w", err)
	}
	return data, nil
}

func (b *browserlessBrowser) Eval(ctx context.Context, js string) (string, error) {
	ctxID, err := b.getOrCreateIsolatedWorld(ctx)
	if err != nil {
		return "", fmt.Errorf("browser: browserless eval create isolated world: %w", err)
	}

	expr := js
	if strings.HasPrefix(strings.TrimSpace(js), "()") {
		expr = "(" + strings.TrimSpace(js) + ")()"
	}

	res, err := proto.RuntimeEvaluate{
		Expression:    expr,
		ContextID:     ctxID,
		ReturnByValue: true,
		AwaitPromise:  true,
	}.Call(b.page.Context(ctx))
	if err != nil {
		return "", fmt.Errorf("browser: browserless eval: %w", err)
	}
	if res.ExceptionDetails != nil {
		return "", fmt.Errorf("browser: browserless eval js error: %s", res.ExceptionDetails.Text)
	}

	return res.Result.Value.String(), nil
}

func (b *browserlessBrowser) EvalDirect(ctx context.Context, js string) (string, error) {
	// Browserless 的 Eval 本身就是 CDP 直连，不存在 navigation 等待问题
	return b.Eval(ctx, js)
}

func (b *browserlessBrowser) getOrCreateIsolatedWorld(ctx context.Context) (proto.RuntimeExecutionContextID, error) {
	if b.isolatedCtxID != 0 {
		return b.isolatedCtxID, nil
	}

	frameID := b.page.FrameID

	res, err := proto.PageCreateIsolatedWorld{
		FrameID:             frameID,
		WorldName:           "scraper_isolated",
		GrantUniveralAccess: true,
	}.Call(b.page.Context(ctx))
	if err != nil {
		return 0, fmt.Errorf("browser: browserless create isolated world: %w", err)
	}

	b.isolatedCtxID = res.ExecutionContextID
	return b.isolatedCtxID, nil
}

func (b *browserlessBrowser) Click(ctx context.Context, selector string) error {
	el, err := b.page.Context(ctx).Element(selector)
	if err != nil {
		return fmt.Errorf("browser: browserless click find %q: %w", selector, err)
	}
	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("browser: browserless click %q: %w", selector, err)
	}
	return nil
}

func (b *browserlessBrowser) Type(ctx context.Context, selector, text string) error {
	el, err := b.page.Context(ctx).Element(selector)
	if err != nil {
		return fmt.Errorf("browser: browserless type find %q: %w", selector, err)
	}
	if err := el.Input(text); err != nil {
		return fmt.Errorf("browser: browserless type %q: %w", selector, err)
	}
	return nil
}

func (b *browserlessBrowser) WaitSelector(ctx context.Context, selector string) error {
	_, err := b.page.Context(ctx).Element(selector)
	if err != nil {
		return fmt.Errorf("browser: browserless wait selector %q: %w", selector, err)
	}
	return nil
}

func (b *browserlessBrowser) Cookies(ctx context.Context) ([]browser.Cookie, error) {
	cookies, err := b.page.Context(ctx).Cookies(nil)
	if err != nil {
		return nil, fmt.Errorf("browser: browserless cookies: %w", err)
	}

	result := make([]browser.Cookie, 0, len(cookies))
	for _, c := range cookies {
		result = append(result, browser.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Secure:   c.Secure,
			HTTPOnly: c.HTTPOnly,
		})
	}
	return result, nil
}

func (b *browserlessBrowser) SetCookies(ctx context.Context, cookies []browser.Cookie) error {
	for _, c := range cookies {
		err := b.page.Context(ctx).SetCookies([]*proto.NetworkCookieParam{
			{
				Name:     c.Name,
				Value:    c.Value,
				Domain:   c.Domain,
				Path:     c.Path,
				Secure:   c.Secure,
				HTTPOnly: c.HTTPOnly,
			},
		})
		if err != nil {
			return fmt.Errorf("browser: browserless set cookie %q: %w", c.Name, err)
		}
	}
	return nil
}

func (b *browserlessBrowser) BrowserType() browser.Type {
	return browser.TypeBrowserless
}

func (b *browserlessBrowser) Close() error {
	if err := b.page.Close(); err != nil {
		slog.Warn("browser: browserless close page", "error", err)
	}
	if b.browser != nil {
		if err := b.browser.Close(); err != nil {
			slog.Warn("browser: browserless close browser", "error", err)
		}
	}
	return nil
}

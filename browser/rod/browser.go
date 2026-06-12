package rod

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	browser "github.com/chinayin/gox-browser/browser"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// 编译期接口合规断言
var _ browser.Browser = (*rodBrowser)(nil)

const (
	rodWaitStableInterval = 300 * time.Millisecond
	rodWaitStableMaxWait  = 5 * time.Second
	rodNetworkIdleTimeout = 1 * time.Second
	rodNetworkIdleMaxWait = 5 * time.Second
)

type rodBrowser struct {
	page     *rod.Page
	router   *rod.HijackRouter // 请求拦截路由，未启用拦截时为 nil
	headless bool
}

func (b *rodBrowser) Navigate(ctx context.Context, url string) error {
	slog.Debug("browser: rod navigate start", "url", url)
	t0 := time.Now()

	if err := b.page.Context(ctx).Navigate(url); err != nil {
		return fmt.Errorf("browser: rod navigate %q: %w", url, err)
	}

	slog.Debug("browser: rod navigate done", "url", url, "elapsed", time.Since(t0).Round(time.Millisecond))
	return nil
}

func (b *rodBrowser) WaitStable(ctx context.Context) error {
	stableCtx, cancel := context.WithTimeout(ctx, rodWaitStableMaxWait)
	defer cancel()

	slog.Debug("browser: rod wait stable start")
	t0 := time.Now()
	if err := b.page.Context(stableCtx).WaitStable(rodWaitStableInterval); err != nil {
		return fmt.Errorf("browser: rod wait stable: %w", err)
	}
	slog.Debug("browser: rod wait stable done", "elapsed", time.Since(t0).Round(time.Millisecond))

	slog.Debug("browser: rod wait network idle start")
	t1 := time.Now()
	if err := b.waitNetworkIdle(ctx); err != nil {
		slog.Debug("browser: rod network idle timeout, continuing", "elapsed", time.Since(t1).Round(time.Millisecond), "error", err)
	} else {
		slog.Debug("browser: rod wait network idle done", "elapsed", time.Since(t1).Round(time.Millisecond))
	}
	return nil
}

func (b *rodBrowser) waitNetworkIdle(ctx context.Context) error {
	// 用有界 ctx 驱动 rod 的等待：超时或上层取消时等待立即返回，
	// 无需额外 goroutine，也不会残留
	idleCtx, cancel := context.WithTimeout(ctx, rodNetworkIdleMaxWait)
	defer cancel()

	b.page.Context(idleCtx).WaitRequestIdle(rodNetworkIdleTimeout, nil, nil, nil)()

	if err := ctx.Err(); err != nil {
		return err
	}
	if idleCtx.Err() != nil {
		return fmt.Errorf("browser: rod network idle max wait exceeded")
	}
	return nil
}

func (b *rodBrowser) HTML(ctx context.Context) (string, error) {
	html, err := b.page.Context(ctx).HTML()
	if err != nil {
		return "", fmt.Errorf("browser: rod html: %w", err)
	}
	return html, nil
}

func (b *rodBrowser) Text(ctx context.Context) (string, error) {
	text, err := b.page.Context(ctx).Eval(`() => document.body.innerText`)
	if err != nil {
		return "", fmt.Errorf("browser: rod text: %w", err)
	}
	return text.Value.String(), nil
}

func (b *rodBrowser) Title(ctx context.Context) (string, error) {
	info, err := b.page.Context(ctx).Info()
	if err != nil {
		return "", fmt.Errorf("browser: rod title: %w", err)
	}
	return info.Title, nil
}

func (b *rodBrowser) URL(ctx context.Context) (string, error) {
	info, err := b.page.Context(ctx).Info()
	if err != nil {
		return "", fmt.Errorf("browser: rod url: %w", err)
	}
	return info.URL, nil
}

func (b *rodBrowser) Screenshot(ctx context.Context) ([]byte, error) {
	data, err := b.page.Context(ctx).Screenshot(true, &proto.PageCaptureScreenshot{
		Format: proto.PageCaptureScreenshotFormatPng,
	})
	if err != nil {
		return nil, fmt.Errorf("browser: rod screenshot: %w", err)
	}
	return data, nil
}

func (b *rodBrowser) Eval(ctx context.Context, js string) (string, error) {
	result, err := b.page.Context(ctx).Eval(js)
	if err != nil {
		return "", fmt.Errorf("browser: rod eval: %w", err)
	}
	return result.Value.String(), nil
}

func (b *rodBrowser) EvalDirect(ctx context.Context, js string) (string, error) {
	expression := "(" + js + ")()"

	res, err := proto.RuntimeEvaluate{
		Expression:    expression,
		AwaitPromise:  true,
		ReturnByValue: true,
		UserGesture:   true,
	}.Call(b.page)
	if err != nil {
		return "", fmt.Errorf("browser: rod eval direct: %w", err)
	}
	if res.ExceptionDetails != nil {
		return "", fmt.Errorf("browser: rod eval direct exception: %s", res.ExceptionDetails.Text)
	}

	val := res.Result
	if val.Type == "undefined" {
		return "", nil
	}
	return val.Value.String(), nil
}

func (b *rodBrowser) Click(ctx context.Context, selector string) error {
	el, err := b.page.Context(ctx).Element(selector)
	if err != nil {
		return fmt.Errorf("browser: rod click find %q: %w", selector, err)
	}
	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("browser: rod click %q: %w", selector, err)
	}
	return nil
}

func (b *rodBrowser) Type(ctx context.Context, selector, text string) error {
	el, err := b.page.Context(ctx).Element(selector)
	if err != nil {
		return fmt.Errorf("browser: rod type find %q: %w", selector, err)
	}
	if err := el.Input(text); err != nil {
		return fmt.Errorf("browser: rod type %q: %w", selector, err)
	}
	return nil
}

func (b *rodBrowser) WaitSelector(ctx context.Context, selector string) error {
	_, err := b.page.Context(ctx).Element(selector)
	if err != nil {
		return fmt.Errorf("browser: rod wait selector %q: %w", selector, err)
	}
	return nil
}

func (b *rodBrowser) Cookies(ctx context.Context) ([]browser.Cookie, error) {
	cookies, err := b.page.Context(ctx).Cookies(nil)
	if err != nil {
		return nil, fmt.Errorf("browser: rod cookies: %w", err)
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

func (b *rodBrowser) SetCookies(ctx context.Context, cookies []browser.Cookie) error {
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
			return fmt.Errorf("browser: rod set cookie %q: %w", c.Name, err)
		}
	}
	return nil
}

func (b *rodBrowser) BrowserType() browser.Type {
	if b.headless {
		return browser.TypeRodHeadless
	}
	return browser.TypeRodHeaded
}

func (b *rodBrowser) Close() error {
	// 先停 hijack router，回收其事件 goroutine，再关页面
	if b.router != nil {
		if err := b.router.Stop(); err != nil {
			slog.Warn("browser: rod stop hijack router failed", "error", err)
		}
		b.router = nil
	}
	if err := b.page.Close(); err != nil {
		return fmt.Errorf("browser: rod close page: %w", err)
	}
	return nil
}

package browser

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// NavigateResult 导航结果
type NavigateResult struct {
	URL     string
	Title   string
	Elapsed time.Duration
}

// NavigateOpts 导航选项
type NavigateOpts struct {
	EarlyWait  time.Duration
	WaitStable bool
}

func defaultNavigateOpts() NavigateOpts {
	return NavigateOpts{
		EarlyWait:  2 * time.Second,
		WaitStable: false,
	}
}

type NavigateOption func(*NavigateOpts)

func WithEarlyWait(d time.Duration) NavigateOption {
	return func(o *NavigateOpts) { o.EarlyWait = d }
}

func WithWaitStable() NavigateOption {
	return func(o *NavigateOpts) { o.WaitStable = true }
}

// NavigateAndCheck 导航到 URL 并检测 WAF 拦截
func NavigateAndCheck(ctx context.Context, b Browser, url string, detector BlockDetector, options ...NavigateOption) (*NavigateResult, error) {
	opts := defaultNavigateOpts()
	for _, fn := range options {
		fn(&opts)
	}

	t0 := time.Now()

	slog.Info("browser: navigate-and-check start", "url", url)
	if err := b.Navigate(ctx, url); err != nil {
		return nil, fmt.Errorf("browser: navigate %q: %w", url, err)
	}

	navElapsed := time.Since(t0)
	slog.Debug("browser: navigate-and-check navigate done", "url", url, "elapsed", navElapsed.Round(time.Millisecond))

	if opts.EarlyWait > 0 {
		select {
		case <-time.After(opts.EarlyWait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if err := CheckWAF(ctx, b, detector); err != nil {
		elapsed := time.Since(t0)
		slog.Error("browser: navigate waf blocked",
			"url", url,
			"nav_elapsed", navElapsed.Round(time.Millisecond),
			"total_elapsed", elapsed.Round(time.Millisecond),
			"error", err,
		)
		return nil, err
	}

	if opts.WaitStable {
		if err := b.WaitStable(ctx); err != nil {
			slog.Warn("browser: navigate wait stable timeout",
				"url", url,
				"elapsed", time.Since(t0).Round(time.Millisecond),
				"error", err,
			)
		}

		if err := CheckWAF(ctx, b, detector); err != nil {
			elapsed := time.Since(t0)
			slog.Error("browser: navigate waf blocked (post-stable)",
				"url", url,
				"total_elapsed", elapsed.Round(time.Millisecond),
				"error", err,
			)
			return nil, err
		}
	}

	logCtx, logCancel := context.WithTimeout(ctx, 5*time.Second)
	defer logCancel()
	title, _ := b.Eval(logCtx, `() => document.title`)
	finalURL, _ := b.Eval(logCtx, `() => location.href`)
	elapsed := time.Since(t0)

	slog.Info("browser: navigate ok",
		"url", url,
		"final_url", finalURL,
		"title", title,
		"elapsed", elapsed.Round(time.Millisecond),
	)

	return &NavigateResult{
		URL:     finalURL,
		Title:   title,
		Elapsed: elapsed,
	}, nil
}

package browser

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

// DefaultBlockDetector 默认风控检测器
type DefaultBlockDetector struct{}

func NewBlockDetector() *DefaultBlockDetector {
	return &DefaultBlockDetector{}
}

func (d *DefaultBlockDetector) Detect(html, title string, statusCode int) BlockResult {
	lower := strings.ToLower(html)
	lowerTitle := strings.ToLower(title)

	if len(strings.TrimSpace(html)) < 100 {
		return BlockResult{Blocked: true, Reason: "response body too short", Type: "empty_response"}
	}

	cfSignals := []string{
		"just a moment", "checking your browser", "cf-browser-verification",
		"cf_chl_opt", "ray id", "turnstile", "attention required",
	}
	for _, sig := range cfSignals {
		if strings.Contains(lower, sig) || strings.Contains(lowerTitle, sig) {
			return BlockResult{Blocked: true, Reason: "cloudflare detected: " + sig, Type: "cloudflare"}
		}
	}

	akamaiSignals := []string{"access denied", "reference #", "akamai"}
	for _, sig := range akamaiSignals {
		if strings.Contains(lower, sig) || strings.Contains(lowerTitle, sig) {
			return BlockResult{Blocked: true, Reason: "akamai detected: " + sig, Type: "akamai"}
		}
	}

	wafTitleSignals := []string{
		"captcha", "are you a robot", "bot detection",
		"please verify you are a human", "access denied",
		"blocked", "forbidden", "security check",
	}
	for _, sig := range wafTitleSignals {
		if strings.Contains(lowerTitle, sig) {
			return BlockResult{Blocked: true, Reason: "waf detected in title: " + sig, Type: "generic_waf"}
		}
	}

	return BlockResult{}
}

const checkWAFTimeout = 10 * time.Second

// CheckWAF 检测当前页面是否被 WAF 拦截
func CheckWAF(ctx context.Context, b Browser, detector BlockDetector) error {
	wafCtx, cancel := context.WithTimeout(ctx, checkWAFTimeout)
	defer cancel()

	html, err := b.Eval(wafCtx, `() => document.documentElement.outerHTML`)
	if err != nil {
		slog.Debug("browser: waf check get html failed, skipping", "error", err)
		return nil
	}

	title, _ := b.Title(wafCtx)

	result := detector.Detect(html, title, 0)
	if !result.Blocked {
		return nil
	}

	pageURL, _ := b.URL(ctx)

	slog.Warn("browser: waf block detected",
		"type", result.Type,
		"reason", result.Reason,
		"url", pageURL,
	)

	return &WAFBlockedError{Result: result, URL: pageURL}
}

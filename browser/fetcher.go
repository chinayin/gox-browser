package browser

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// FetcherOption 配置 Fetcher 的函数式选项
type FetcherOption func(*Fetcher)

func WithPrimary(t Type) FetcherOption {
	return func(f *Fetcher) { f.primary = t }
}

func WithFallback(types ...Type) FetcherOption {
	return func(f *Fetcher) { f.fallback = types }
}

// FetchOption 单次 Fetch 调用的选项
type FetchOption func(*fetchCall)

type fetchCall struct {
	providerOverride *Type
}

// FetchWithProvider 覆盖本次请求使用的浏览器类型
func FetchWithProvider(t Type) FetchOption {
	return func(fc *fetchCall) { fc.providerOverride = &t }
}

const detectTimeout = 10 * time.Second

// defaultHTTPClient 内部 HTTP 客户端，禁止使用 http.DefaultClient
var defaultHTTPClient = &http.Client{
	Timeout: detectTimeout,
}

// Fetcher 业务层单一入口，封装 Pool + BlockDetector
type Fetcher struct {
	pool     *Pool
	detector BlockDetector

	mu       sync.RWMutex
	primary  Type
	fallback []Type
}

func NewFetcher(pool *Pool, detector BlockDetector, opts ...FetcherOption) (*Fetcher, error) {
	f := &Fetcher{
		pool:     pool,
		detector: detector,
		primary:  TypeRodHeadless,
	}

	for _, opt := range opts {
		opt(f)
	}

	if !pool.HasProvider(f.primary) {
		return nil, fmt.Errorf("browser: primary provider %s not registered", f.primary)
	}

	valid := make([]Type, 0, len(f.fallback))
	for _, t := range f.fallback {
		if pool.HasProvider(t) {
			valid = append(valid, t)
		} else {
			slog.Warn("browser: fallback provider not registered, skipping", "type", t.String())
		}
	}
	f.fallback = valid

	slog.Info("browser: fetcher created",
		"primary", f.primary.String(),
		"fallback", typeNames(f.fallback),
	)

	return f, nil
}

func (f *Fetcher) Fetch(ctx context.Context, url string, opts ...FetchOption) (*FetchResult, error) {
	fc := &fetchCall{}
	for _, opt := range opts {
		opt(fc)
	}

	chain := f.buildChain(fc)

	slog.Debug("browser: fetch start", "url", url, "chain", typeNames(chain))
	return f.pool.FetchWithFallback(ctx, url, chain, f.detector)
}

// Detect 探测目标 URL 的页面类型
func (f *Fetcher) Detect(ctx context.Context, url string) (*DetectResult, error) {
	detectCtx, cancel := context.WithTimeout(ctx, detectTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(detectCtx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("browser: detect build request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; scraper-kit-detect/1.0)")

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("browser: detect http get %q: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, fmt.Errorf("browser: detect read body: %w", err)
	}

	html := string(body)
	result := analyzeHTML(html)

	slog.Info("browser: detect done",
		"url", url,
		"suggested_type", result.SuggestedType.String(),
		"score", result.Score,
		"signals", result.Signals,
	)

	return result, nil
}

func (f *Fetcher) SetPrimary(t Type) {
	f.mu.Lock()
	defer f.mu.Unlock()

	old := f.primary
	f.primary = t
	slog.Info("browser: primary switched", "from", old.String(), "to", t.String())
}

func (f *Fetcher) buildChain(fc *fetchCall) []Type {
	if fc.providerOverride != nil {
		return []Type{*fc.providerOverride}
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	chain := make([]Type, 0, 1+len(f.fallback))
	chain = append(chain, f.primary)
	chain = append(chain, f.fallback...)
	return chain
}

func analyzeHTML(html string) *DetectResult {
	result := &DetectResult{
		SuggestedType: TypeRodHeadless,
	}

	lower := strings.ToLower(html)

	textLen := estimateTextLength(lower)
	if textLen > 500 {
		result.Signals = append(result.Signals, "substantial_text_content")
		result.Score += 30
		result.IsSSR = true
	}

	if strings.Contains(lower, "<noscript") {
		result.Signals = append(result.Signals, "noscript_tag")
		result.Score += 10
	}

	jsFrameworkSignals := []string{
		"__next_data__", "__nuxt", "window.__initial_state__",
		"react-root", "ng-app", "data-reactroot",
	}
	hasFramework := false
	for _, sig := range jsFrameworkSignals {
		if strings.Contains(lower, sig) {
			hasFramework = true
			result.Signals = append(result.Signals, "js_framework:"+sig)
			result.Score -= 20
			break
		}
	}

	if textLen < 100 && !hasFramework {
		result.Signals = append(result.Signals, "minimal_content")
		result.Score -= 10
	}

	if result.Score >= 20 {
		result.SuggestedType = TypeSurf
	} else {
		result.SuggestedType = TypeRodHeadless
	}

	return result
}

func estimateTextLength(html string) int {
	inTag := false
	n := 0
	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag && r != '\n' && r != '\r' && r != '\t':
			n++
		}
	}
	return n
}

func typeNames(types []Type) []string {
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = t.String()
	}
	return names
}

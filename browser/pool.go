package browser

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Pool 浏览器实例池
type Pool struct {
	cfg       PoolConfig
	providers map[Type]Provider

	mu        sync.Mutex
	instances map[Type][]*poolEntry
	inUse     int
	total     int

	closed atomic.Bool
	done   chan struct{}

	metrics *Metrics
}

type poolEntry struct {
	browser  Browser
	lastUsed time.Time
}

func NewPool(cfg PoolConfig) *Pool {
	if cfg.PingTimeout <= 0 {
		cfg.PingTimeout = DefaultPoolConfig.PingTimeout
	}
	p := &Pool{
		cfg:       cfg,
		providers: make(map[Type]Provider),
		instances: make(map[Type][]*poolEntry),
		done:      make(chan struct{}),
		metrics:   NewMetrics(),
	}

	go p.healthCheckLoop()

	return p
}

func (p *Pool) Register(provider Provider) {
	p.mu.Lock()
	defer p.mu.Unlock()

	t := provider.Type()
	p.providers[t] = provider
	slog.Info("browser: provider registered", "type", t.String())
}

func (p *Pool) HasProvider(t Type) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.providers[t]
	return ok
}

func (p *Pool) Acquire(ctx context.Context, opts AcquireOpts) (Browser, error) {
	if p.closed.Load() {
		return nil, ErrPoolClosed
	}

	p.metrics.AcquireTotal.Add(1)

	p.mu.Lock()
	provider, ok := p.providers[opts.Type]
	if !ok {
		p.mu.Unlock()
		p.metrics.AcquireFail.Add(1)
		return nil, fmt.Errorf("browser: acquire type %s: %w", opts.Type.String(), ErrProviderNotFound)
	}

	for len(p.instances[opts.Type]) > 0 {
		entries := p.instances[opts.Type]
		entry := entries[len(entries)-1]
		p.instances[opts.Type] = entries[:len(entries)-1]

		pingCtx, pingCancel := context.WithTimeout(ctx, p.cfg.PingTimeout)
		_, pingErr := entry.browser.Title(pingCtx)
		pingCancel()
		if pingErr != nil {
			slog.Warn("browser: discarding dead idle instance",
				"type", opts.Type.String(), "error", pingErr)
			_ = entry.browser.Close()
			p.total--
			p.metrics.EvictTotal.Add(1)
			continue
		}

		p.inUse++
		entry.lastUsed = time.Now()
		p.mu.Unlock()

		p.metrics.AcquireSuccess.Add(1)
		p.metrics.ReuseTotal.Add(1)
		slog.Debug("browser: reused idle instance", "type", opts.Type.String())
		return entry.browser, nil
	}

	if p.total >= p.cfg.MaxInstances {
		p.mu.Unlock()
		return p.waitForInstance(ctx, opts)
	}

	p.total++
	p.inUse++
	p.mu.Unlock()

	b, err := provider.Create(ctx, opts)
	if err != nil {
		p.mu.Lock()
		p.total--
		p.inUse--
		p.mu.Unlock()
		p.metrics.AcquireFail.Add(1)
		return nil, fmt.Errorf("browser: create %s instance: %w", opts.Type.String(), err)
	}

	p.metrics.AcquireSuccess.Add(1)
	p.metrics.CreateTotal.Add(1)
	slog.Info("browser: created new instance", "type", opts.Type.String())
	return b, nil
}

func (p *Pool) waitForInstance(ctx context.Context, opts AcquireOpts) (Browser, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("browser: wait for %s instance: %w", opts.Type.String(), ErrAcquireTimeout)
		case <-ticker.C:
			p.mu.Lock()
			for len(p.instances[opts.Type]) > 0 {
				entries := p.instances[opts.Type]
				entry := entries[len(entries)-1]
				p.instances[opts.Type] = entries[:len(entries)-1]

				pingCtx, pingCancel := context.WithTimeout(ctx, p.cfg.PingTimeout)
				_, pingErr := entry.browser.Title(pingCtx)
				pingCancel()
				if pingErr != nil {
					slog.Warn("browser: discarding dead idle instance",
						"type", opts.Type.String(), "error", pingErr)
					_ = entry.browser.Close()
					p.total--
					p.metrics.EvictTotal.Add(1)
					continue
				}

				p.inUse++
				entry.lastUsed = time.Now()
				p.mu.Unlock()
				return entry.browser, nil
			}
			p.mu.Unlock()
		}
	}
}

// Release 归还浏览器实例到池中
func (p *Pool) Release(b Browser, lastErr error) error {
	if p.closed.Load() {
		return b.Close()
	}

	p.metrics.ReleaseTotal.Add(1)

	if IsConnectionError(lastErr) {
		slog.Warn("browser: releasing unhealthy instance",
			"type", b.BrowserType().String(), "error", lastErr)
		closeErr := b.Close()

		p.mu.Lock()
		p.inUse--
		p.total--
		p.mu.Unlock()

		return closeErr
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.inUse--
	p.instances[b.BrowserType()] = append(p.instances[b.BrowserType()], &poolEntry{
		browser:  b,
		lastUsed: time.Now(),
	})

	slog.Debug("browser: instance released", "type", b.BrowserType().String())
	return nil
}

func (p *Pool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	byType := make(map[Type]int)
	available := 0
	for t, entries := range p.instances {
		byType[t] = len(entries)
		available += len(entries)
	}

	return PoolStats{
		Total:     p.total,
		Available: available,
		InUse:     p.inUse,
		ByType:    byType,
	}
}

func (p *Pool) Metrics() *Metrics {
	return p.metrics
}

func (p *Pool) Close() error {
	if !p.closed.CompareAndSwap(false, true) {
		return nil
	}

	close(p.done)

	p.mu.Lock()
	defer p.mu.Unlock()

	for t, entries := range p.instances {
		for _, entry := range entries {
			if err := entry.browser.Close(); err != nil {
				slog.Warn("browser: close idle instance", "type", t.String(), "error", err)
			}
		}
	}
	p.instances = nil

	for t, provider := range p.providers {
		if err := provider.Close(); err != nil {
			slog.Warn("browser: close provider", "type", t.String(), "error", err)
		}
	}

	slog.Info("browser: pool closed")
	return nil
}

// FetchWithFallback 智能降级抓取
func (p *Pool) FetchWithFallback(ctx context.Context, url string, chain []Type, detector BlockDetector) (*FetchResult, error) {
	start := time.Now()
	result := &FetchResult{}
	p.metrics.FetchTotal.Add(1)

	for i, bt := range chain {
		attemptStart := time.Now()
		attempt := FetchAttempt{Type: bt}

		slog.Debug("browser: fallback attempt", "type", bt.String(), "url", url)

		b, err := p.Acquire(ctx, AcquireOpts{Type: bt})
		if err != nil {
			attempt.Duration = time.Since(attemptStart)
			attempt.Err = err
			attempt.Reason = err.Error()
			result.Attempts = append(result.Attempts, attempt)
			slog.Warn("browser: acquire failed, trying next", "type", bt.String(), "error", err)
			continue
		}

		html, title, blocked, reason := p.tryFetch(ctx, b, url, detector, &attempt)
		attempt.Duration = time.Since(attemptStart)

		if err := p.Release(b, attempt.Err); err != nil {
			slog.Warn("browser: release failed", "type", bt.String(), "error", err)
		}

		result.Attempts = append(result.Attempts, attempt)

		if attempt.Err != nil {
			slog.Warn("browser: fetch failed, trying next", "type", bt.String(), "error", attempt.Err)
			continue
		}

		if blocked {
			p.metrics.FetchBlocked.Add(1)
			p.metrics.RecordBlockByType(bt)
			slog.Warn("browser: blocked, trying next", "type", bt.String(), "reason", reason)
			continue
		}

		if i > 0 {
			p.metrics.FetchFallback.Add(1)
		}
		p.metrics.FetchSuccess.Add(1)
		p.metrics.RecordFetchByType(bt)
		result.HTML = html
		result.Title = title
		result.FinalType = bt
		result.TotalLatency = time.Since(start)
		p.metrics.RecordLatency(result.TotalLatency)
		slog.Info("browser: fetch succeeded", "type", bt.String(), "latency", result.TotalLatency)
		return result, nil
	}

	p.metrics.FetchAllFail.Add(1)
	result.TotalLatency = time.Since(start)
	return result, fmt.Errorf("browser: fetch %q: %w", url, ErrAllBlocked)
}

func (p *Pool) tryFetch(ctx context.Context, b Browser, url string, detector BlockDetector, attempt *FetchAttempt) (html, title string, blocked bool, reason string) {
	if err := b.Navigate(ctx, url); err != nil {
		attempt.Err = fmt.Errorf("browser: navigate %q: %w", url, err)
		attempt.Reason = attempt.Err.Error()
		return html, title, blocked, reason
	}

	if err := b.WaitStable(ctx); err != nil {
		attempt.Err = fmt.Errorf("browser: wait stable: %w", err)
		attempt.Reason = attempt.Err.Error()
		return html, title, blocked, reason
	}

	var err error
	html, err = b.HTML(ctx)
	if err != nil {
		attempt.Err = fmt.Errorf("browser: get html: %w", err)
		attempt.Reason = attempt.Err.Error()
		return html, title, blocked, reason
	}

	title, err = b.Title(ctx)
	if err != nil && !errors.Is(err, ErrUnsupported) {
		attempt.Err = fmt.Errorf("browser: get title: %w", err)
		attempt.Reason = attempt.Err.Error()
		return html, title, blocked, reason
	}

	br := detector.Detect(html, title, 0)
	if br.Blocked {
		attempt.Blocked = true
		attempt.Reason = br.Reason
		blocked = true
		reason = br.Reason
		return html, title, blocked, reason
	}

	return html, title, blocked, reason
}

func (p *Pool) healthCheckLoop() {
	ticker := time.NewTicker(p.cfg.HealthCheckFreq)
	defer ticker.Stop()

	for {
		select {
		case <-p.done:
			return
		case <-ticker.C:
			p.evictIdle()
		}
	}
}

func (p *Pool) evictIdle() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for t, entries := range p.instances {
		alive := entries[:0]
		for _, entry := range entries {
			if now.Sub(entry.lastUsed) > p.cfg.IdleTimeout {
				slog.Info("browser: evicting idle instance", "type", t.String(),
					"idle", now.Sub(entry.lastUsed).Round(time.Second))
				if err := entry.browser.Close(); err != nil {
					slog.Warn("browser: close evicted instance", "error", err)
				}
				p.total--
				p.metrics.EvictTotal.Add(1)
			} else {
				alive = append(alive, entry)
			}
		}
		p.instances[t] = alive
	}
}

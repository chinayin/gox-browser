package browserless

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const pressureRequestTimeout = 5 * time.Second

// Endpoint 单个 browserless 节点
type Endpoint struct {
	URL      string
	Token    string
	Weight   int
	Healthy  bool
	Pressure PressureInfo
}

// PressureInfo 来自 Browserless API 的压力信息 (JSON 字段名由外部 API 决定)
//
//nolint:tagliatelle // JSON 字段名由 Browserless API 定义，不可修改
type PressureInfo struct {
	CPU           int  `json:"cpu"`
	Memory        int  `json:"memory"`
	Running       int  `json:"running"`
	Queued        int  `json:"queued"`
	MaxConcurrent int  `json:"maxConcurrent"`
	IsAvailable   bool `json:"isAvailable"`
}

// Router Endpoint 路由器
type Router struct {
	endpoints []Endpoint
	strategy  string

	mu      sync.RWMutex
	rrIndex atomic.Uint64

	stopCh chan struct{}
	client *http.Client
}

func NewRouter(endpoints []Endpoint, strategy string, healthCheckInterval time.Duration) *Router {
	r := &Router{
		endpoints: endpoints,
		strategy:  strategy,
		stopCh:    make(chan struct{}),
		client:    &http.Client{Timeout: pressureRequestTimeout},
	}

	if len(endpoints) > 0 && healthCheckInterval > 0 {
		go r.healthCheckLoop(healthCheckInterval)
	}

	return r
}

func (r *Router) Pick() *Endpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.endpoints) == 0 {
		return nil
	}

	switch r.strategy {
	case "round-robin":
		return r.pickRoundRobin()
	default:
		return r.pickLeastLoad()
	}
}

func (r *Router) pickLeastLoad() *Endpoint {
	var best *Endpoint
	bestLoad := math.MaxInt

	for i := range r.endpoints {
		ep := &r.endpoints[i]
		if !ep.Healthy {
			continue
		}
		load := ep.Pressure.Running + ep.Pressure.Queued
		if load < bestLoad {
			bestLoad = load
			best = ep
		}
	}

	if best == nil && len(r.endpoints) > 0 {
		return &r.endpoints[0]
	}
	return best
}

func (r *Router) pickRoundRobin() *Endpoint {
	n := uint64(len(r.endpoints))
	for range n {
		idx := r.rrIndex.Add(1) - 1
		ep := &r.endpoints[idx%n]
		if ep.Healthy {
			return ep
		}
	}

	if len(r.endpoints) > 0 {
		return &r.endpoints[0]
	}
	return nil
}

func (r *Router) MarkUnhealthy(url string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.endpoints {
		if r.endpoints[i].URL == url {
			r.endpoints[i].Healthy = false
			slog.Warn("browser: browserless endpoint marked unhealthy", "url", url)
			return
		}
	}
}

func (r *Router) HealthCheckAll(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var lastErr error
	for i := range r.endpoints {
		ep := &r.endpoints[i]
		pressure, err := r.fetchPressure(ctx, ep)
		if err != nil {
			ep.Healthy = false
			lastErr = err
			slog.Warn("browser: browserless health check failed",
				"endpoint", ep.URL, "error", err)
			continue
		}
		ep.Pressure = pressure
		ep.Healthy = pressure.IsAvailable
	}
	return lastErr
}

func (r *Router) Stop() {
	select {
	case <-r.stopCh:
	default:
		close(r.stopCh)
	}
}

func (r *Router) healthCheckLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), pressureRequestTimeout)
			_ = r.HealthCheckAll(ctx)
			cancel()
		}
	}
}

func (r *Router) CheckCapacity(ctx context.Context, ep *Endpoint) error {
	checkCtx, cancel := context.WithTimeout(ctx, pressureRequestTimeout)
	defer cancel()

	pressure, err := r.fetchPressure(checkCtx, ep)
	if err != nil {
		return fmt.Errorf("browser: browserless capacity check %s: %w", ep.URL, err)
	}

	if !pressure.IsAvailable {
		return fmt.Errorf("browser: browserless %s not available (running=%d queued=%d max=%d)",
			ep.URL, pressure.Running, pressure.Queued, pressure.MaxConcurrent)
	}

	return nil
}

func (r *Router) fetchPressure(ctx context.Context, ep *Endpoint) (PressureInfo, error) {
	u := strings.TrimRight(ep.URL, "/") + "/pressure"
	if ep.Token != "" {
		u += "?token=" + ep.Token
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return PressureInfo{}, fmt.Errorf("browser: browserless build pressure request: %w", err)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return PressureInfo{}, fmt.Errorf("browser: browserless pressure request %s: %w", ep.URL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return PressureInfo{}, fmt.Errorf("browser: browserless read pressure response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return PressureInfo{}, fmt.Errorf("browser: browserless pressure %s returned %d: %s",
			ep.URL, resp.StatusCode, string(body))
	}

	var wrapper struct {
		Pressure PressureInfo `json:"pressure"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return PressureInfo{}, fmt.Errorf("browser: browserless parse pressure response: %w", err)
	}

	return wrapper.Pressure, nil
}

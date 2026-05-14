package browser

import (
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics 浏览器池运行指标
type Metrics struct {
	AcquireTotal   atomic.Int64
	AcquireSuccess atomic.Int64
	AcquireFail    atomic.Int64
	CreateTotal    atomic.Int64
	ReleaseTotal   atomic.Int64
	EvictTotal     atomic.Int64
	ReuseTotal     atomic.Int64

	FetchTotal    atomic.Int64
	FetchSuccess  atomic.Int64
	FetchBlocked  atomic.Int64
	FetchFallback atomic.Int64
	FetchAllFail  atomic.Int64

	mu          sync.Mutex
	fetchByType map[Type]int64
	blockByType map[Type]int64

	latencyMu sync.Mutex
	latencies []time.Duration
}

const maxLatencySamples = 1000

func NewMetrics() *Metrics {
	return &Metrics{
		fetchByType: make(map[Type]int64),
		blockByType: make(map[Type]int64),
	}
}

func (m *Metrics) RecordLatency(d time.Duration) {
	m.latencyMu.Lock()
	defer m.latencyMu.Unlock()
	if len(m.latencies) >= maxLatencySamples {
		m.latencies = m.latencies[1:]
	}
	m.latencies = append(m.latencies, d)
}

func (m *Metrics) RecordFetchByType(t Type) {
	m.mu.Lock()
	m.fetchByType[t]++
	m.mu.Unlock()
}

func (m *Metrics) RecordBlockByType(t Type) {
	m.mu.Lock()
	m.blockByType[t]++
	m.mu.Unlock()
}

// MetricsSnapshot 指标快照
type MetricsSnapshot struct {
	AcquireTotal   int64          `json:"acquire_total"`
	AcquireSuccess int64          `json:"acquire_success"`
	AcquireFail    int64          `json:"acquire_fail"`
	CreateTotal    int64          `json:"create_total"`
	ReleaseTotal   int64          `json:"release_total"`
	EvictTotal     int64          `json:"evict_total"`
	ReuseTotal     int64          `json:"reuse_total"`
	FetchTotal     int64          `json:"fetch_total"`
	FetchSuccess   int64          `json:"fetch_success"`
	FetchBlocked   int64          `json:"fetch_blocked"`
	FetchFallback  int64          `json:"fetch_fallback"`
	FetchAllFail   int64          `json:"fetch_all_fail"`
	FetchByType    map[Type]int64 `json:"fetch_by_type"`
	BlockByType    map[Type]int64 `json:"block_by_type"`
	AvgLatency     time.Duration  `json:"avg_latency"`
	P50Latency     time.Duration  `json:"p50_latency"`
	P95Latency     time.Duration  `json:"p95_latency"`
	P99Latency     time.Duration  `json:"p99_latency"`
	SuccessRate    float64        `json:"success_rate"`
	FallbackRate   float64        `json:"fallback_rate"`
}

func (m *Metrics) Snapshot() MetricsSnapshot {
	s := MetricsSnapshot{
		AcquireTotal:   m.AcquireTotal.Load(),
		AcquireSuccess: m.AcquireSuccess.Load(),
		AcquireFail:    m.AcquireFail.Load(),
		CreateTotal:    m.CreateTotal.Load(),
		ReleaseTotal:   m.ReleaseTotal.Load(),
		EvictTotal:     m.EvictTotal.Load(),
		ReuseTotal:     m.ReuseTotal.Load(),
		FetchTotal:     m.FetchTotal.Load(),
		FetchSuccess:   m.FetchSuccess.Load(),
		FetchBlocked:   m.FetchBlocked.Load(),
		FetchFallback:  m.FetchFallback.Load(),
		FetchAllFail:   m.FetchAllFail.Load(),
	}

	m.mu.Lock()
	s.FetchByType = make(map[Type]int64, len(m.fetchByType))
	for k, v := range m.fetchByType {
		s.FetchByType[k] = v
	}
	s.BlockByType = make(map[Type]int64, len(m.blockByType))
	for k, v := range m.blockByType {
		s.BlockByType[k] = v
	}
	m.mu.Unlock()

	m.latencyMu.Lock()
	sorted := make([]time.Duration, len(m.latencies))
	copy(sorted, m.latencies)
	m.latencyMu.Unlock()

	slices.SortFunc(sorted, func(a, b time.Duration) int {
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	})

	if len(sorted) > 0 {
		var total time.Duration
		for _, d := range sorted {
			total += d
		}
		s.AvgLatency = total / time.Duration(len(sorted))
		s.P50Latency = percentile(sorted, 0.50)
		s.P95Latency = percentile(sorted, 0.95)
		s.P99Latency = percentile(sorted, 0.99)
	}

	if s.FetchTotal > 0 {
		s.SuccessRate = float64(s.FetchSuccess) / float64(s.FetchTotal)
		s.FallbackRate = float64(s.FetchFallback) / float64(s.FetchTotal)
	}

	return s
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

func (m *Metrics) LogSummary() {
	s := m.Snapshot()
	slog.Info("browser: metrics summary",
		"acquire_total", s.AcquireTotal,
		"acquire_success", s.AcquireSuccess,
		"create_total", s.CreateTotal,
		"reuse_total", s.ReuseTotal,
		"evict_total", s.EvictTotal,
		"fetch_total", s.FetchTotal,
		"fetch_success", s.FetchSuccess,
		"fetch_blocked", s.FetchBlocked,
		"fetch_fallback", s.FetchFallback,
		"fetch_all_fail", s.FetchAllFail,
		"success_rate", fmt.Sprintf("%.1f%%", s.SuccessRate*100),
		"fallback_rate", fmt.Sprintf("%.1f%%", s.FallbackRate*100),
		"avg_latency", s.AvgLatency,
		"p50_latency", s.P50Latency,
		"p95_latency", s.P95Latency,
		"p99_latency", s.P99Latency,
	)
}

package browser

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPercentile(t *testing.T) {
	tests := []struct {
		name     string
		sorted   []time.Duration
		p        float64
		expected time.Duration
	}{
		{
			name:     "empty slice returns 0",
			sorted:   []time.Duration{},
			p:        0.50,
			expected: 0,
		},
		{
			name:     "single element",
			sorted:   []time.Duration{100 * time.Millisecond},
			p:        0.99,
			expected: 100 * time.Millisecond,
		},
		{
			name: "p50 of 10 elements",
			sorted: []time.Duration{
				10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond,
				40 * time.Millisecond, 50 * time.Millisecond, 60 * time.Millisecond,
				70 * time.Millisecond, 80 * time.Millisecond, 90 * time.Millisecond,
				100 * time.Millisecond,
			},
			p:        0.50,
			expected: 50 * time.Millisecond, // idx = int(9 * 0.50) = 4 → 50ms
		},
		{
			name: "p95 of 10 elements",
			sorted: []time.Duration{
				10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond,
				40 * time.Millisecond, 50 * time.Millisecond, 60 * time.Millisecond,
				70 * time.Millisecond, 80 * time.Millisecond, 90 * time.Millisecond,
				100 * time.Millisecond,
			},
			p:        0.95,
			expected: 90 * time.Millisecond, // idx = int(9 * 0.95) = 8 → 90ms
		},
		{
			name: "p99 of 10 elements",
			sorted: []time.Duration{
				10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond,
				40 * time.Millisecond, 50 * time.Millisecond, 60 * time.Millisecond,
				70 * time.Millisecond, 80 * time.Millisecond, 90 * time.Millisecond,
				100 * time.Millisecond,
			},
			p:        0.99,
			expected: 90 * time.Millisecond, // idx = int(9 * 0.99) = int(8.91) = 8 → 90ms
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := percentile(tt.sorted, tt.p)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMetrics_RecordLatency(t *testing.T) {
	// Arrange
	m := NewMetrics()

	// Act - add several samples
	for i := range 100 {
		m.RecordLatency(time.Duration(i+1) * time.Millisecond)
	}

	// Assert via snapshot
	snap := m.Snapshot()
	assert.Equal(t, 50*time.Millisecond+500*time.Microsecond, snap.AvgLatency) // avg of 1..100 = 50.5ms
	assert.Positive(t, int64(snap.P50Latency))
	assert.Greater(t, int64(snap.P95Latency), int64(snap.P50Latency))
}

func TestMetrics_RecordLatency_SlidingWindow(t *testing.T) {
	// Arrange
	m := NewMetrics()

	// Act - exceed maxLatencySamples
	for i := range maxLatencySamples + 100 {
		m.RecordLatency(time.Duration(i) * time.Millisecond)
	}

	// Assert - should only keep maxLatencySamples
	m.latencyMu.Lock()
	count := len(m.latencies)
	m.latencyMu.Unlock()
	assert.Equal(t, maxLatencySamples, count)
}

func TestMetrics_Snapshot_SuccessRate(t *testing.T) {
	// Arrange
	m := NewMetrics()
	m.FetchTotal.Store(100)
	m.FetchSuccess.Store(80)
	m.FetchFallback.Store(10)

	// Act
	snap := m.Snapshot()

	// Assert
	assert.InDelta(t, 0.80, snap.SuccessRate, 0.001)
	assert.InDelta(t, 0.10, snap.FallbackRate, 0.001)
}

func TestMetrics_Snapshot_ZeroFetchTotal(t *testing.T) {
	// Arrange
	m := NewMetrics()

	// Act
	snap := m.Snapshot()

	// Assert - no division by zero
	assert.InDelta(t, 0, snap.SuccessRate, 0.0001)
	assert.InDelta(t, 0, snap.FallbackRate, 0.0001)
}

package browserless_test

import (
	"testing"

	"github.com/chinayin/gox-browser/browser/browserless"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRouter(endpoints []browserless.Endpoint, strategy string) *browserless.Router {
	return browserless.NewRouter(endpoints, strategy, 0) // 0 interval avoids background goroutine
}

func TestRouter_Pick_LeastLoad_SingleEndpoint(t *testing.T) {
	// Arrange
	eps := []browserless.Endpoint{
		{URL: "http://node1:3000", Healthy: true},
	}
	r := newTestRouter(eps, "least-load")

	// Act
	ep := r.Pick()

	// Assert
	require.NotNil(t, ep)
	assert.Equal(t, "http://node1:3000", ep.URL)
}

func TestRouter_Pick_LeastLoad_MultipleEndpoints(t *testing.T) {
	// Arrange
	eps := []browserless.Endpoint{
		{URL: "http://node1:3000", Healthy: true, Pressure: browserless.PressureInfo{Running: 5, Queued: 2}},
		{URL: "http://node2:3000", Healthy: true, Pressure: browserless.PressureInfo{Running: 1, Queued: 0}},
		{URL: "http://node3:3000", Healthy: true, Pressure: browserless.PressureInfo{Running: 3, Queued: 1}},
	}
	r := newTestRouter(eps, "least-load")

	// Act
	ep := r.Pick()

	// Assert - should pick node2 (lowest load: 1+0=1)
	require.NotNil(t, ep)
	assert.Equal(t, "http://node2:3000", ep.URL)
}

func TestRouter_Pick_LeastLoad_AllUnhealthy(t *testing.T) {
	// Arrange
	eps := []browserless.Endpoint{
		{URL: "http://node1:3000", Healthy: false},
		{URL: "http://node2:3000", Healthy: false},
	}
	r := newTestRouter(eps, "least-load")

	// Act
	ep := r.Pick()

	// Assert - falls back to first endpoint
	require.NotNil(t, ep)
	assert.Equal(t, "http://node1:3000", ep.URL)
}

func TestRouter_Pick_RoundRobin(t *testing.T) {
	// Arrange
	eps := []browserless.Endpoint{
		{URL: "http://node1:3000", Healthy: true},
		{URL: "http://node2:3000", Healthy: true},
		{URL: "http://node3:3000", Healthy: true},
	}
	r := newTestRouter(eps, "round-robin")

	// Act - pick 3 times to cycle through all
	urls := make([]string, 3)
	for i := range 3 {
		ep := r.Pick()
		require.NotNil(t, ep)
		urls[i] = ep.URL
	}

	// Assert - should cycle through endpoints
	assert.Contains(t, urls, "http://node1:3000")
	assert.Contains(t, urls, "http://node2:3000")
	assert.Contains(t, urls, "http://node3:3000")
}

func TestRouter_Pick_RoundRobin_SkipsUnhealthy(t *testing.T) {
	// Arrange
	eps := []browserless.Endpoint{
		{URL: "http://node1:3000", Healthy: false},
		{URL: "http://node2:3000", Healthy: true},
		{URL: "http://node3:3000", Healthy: false},
	}
	r := newTestRouter(eps, "round-robin")

	// Act
	ep := r.Pick()

	// Assert - should pick node2 (only healthy)
	require.NotNil(t, ep)
	assert.Equal(t, "http://node2:3000", ep.URL)
}

func TestRouter_Pick_EmptyEndpoints(t *testing.T) {
	// Arrange
	r := newTestRouter([]browserless.Endpoint{}, "least-load")

	// Act
	ep := r.Pick()

	// Assert
	assert.Nil(t, ep)
}

func TestRouter_MarkUnhealthy(t *testing.T) {
	// Arrange
	eps := []browserless.Endpoint{
		{URL: "http://node1:3000", Healthy: true, Pressure: browserless.PressureInfo{Running: 0}},
		{URL: "http://node2:3000", Healthy: true, Pressure: browserless.PressureInfo{Running: 5}},
	}
	r := newTestRouter(eps, "least-load")

	// Act - mark node1 as unhealthy
	r.MarkUnhealthy("http://node1:3000")

	// Assert - should now pick node2
	ep := r.Pick()
	require.NotNil(t, ep)
	assert.Equal(t, "http://node2:3000", ep.URL)
}

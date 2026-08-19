package main

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestForwardMetrics(t *testing.T) {
	t.Run("successful forward increments success counter", func(t *testing.T) {
		backend := newTestBackend(t, 1, 0, time.Second, time.Second)
		backendURL := backend.builders[0].Server.URL

		backend.request(t, []byte(mockNewPayloadRequest), from)

		require.Equal(t, float64(1), testutil.ToFloat64(
			forwardsTotal.WithLabelValues(newPayloadPath, backendURL, "success")))
		require.Equal(t, float64(0), testutil.ToFloat64(
			forwardsTotal.WithLabelValues(newPayloadPath, backendURL, "error")))
	})

	t.Run("unreachable backend increments error counter", func(t *testing.T) {
		backend := newTestBackend(t, 1, 0, time.Second, time.Second)
		backendURL := backend.builders[0].Server.URL
		backend.builders[0].Server.Close()

		backend.request(t, []byte(mockNewPayloadRequest), from)

		require.Equal(t, float64(1), testutil.ToFloat64(
			forwardsTotal.WithLabelValues(newPayloadPath, backendURL, "error")))
	})

	t.Run("mirror mode records metrics asynchronously", func(t *testing.T) {
		backend := newTestBackend(t, 1, 0, time.Second, time.Second)
		backend.proxyService.mirrorMode = true
		backendURL := backend.builders[0].Server.URL

		backend.request(t, []byte(mockNewPayloadRequest), from)

		require.Eventually(t, func() bool {
			return testutil.ToFloat64(
				forwardsTotal.WithLabelValues(newPayloadPath, backendURL, "success")) == 1
		}, 2*time.Second, 10*time.Millisecond)
	})
}

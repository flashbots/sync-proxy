package main

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	forwardsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "syncproxy_forwards_total",
		Help: "Engine API requests forwarded to backends, by method, backend and result (success = 2xx response, http_error = non-2xx response, error = transport failure/timeout).",
	}, []string{"method", "backend", "result"})

	forwardDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "syncproxy_forward_duration_seconds",
		Help:    "Duration of forwarded requests to backends.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "backend"})
)

// StartMetricsServer serves /metrics on addr. Blocking.
func StartMetricsServer(addr string) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return srv.ListenAndServe()
}

func observeForward(method, backend string, statusCode int, start time.Time) {
	result := "http_error"
	if statusCode >= 200 && statusCode < 300 {
		result = "success"
	}
	forwardsTotal.WithLabelValues(method, backend, result).Inc()
	forwardDuration.WithLabelValues(method, backend).Observe(time.Since(start).Seconds())
}

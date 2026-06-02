// Package metrics defines Prometheus metrics for VCIB.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for VCIB.
type Metrics struct {
	InvalidationRequestsTotal   *prometheus.CounterVec
	PodConfirmationsTotal       *prometheus.CounterVec
	RetriesTotal                *prometheus.CounterVec
	InvalidationDurationSeconds *prometheus.HistogramVec
	PodsDiscovered              prometheus.Gauge
	DispatchConcurrencyLimit    prometheus.Gauge
}

// New creates and registers all Prometheus metrics.
func New() *Metrics {
	return &Metrics{
		InvalidationRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "vcib_invalidation_requests_total",
				Help: "Total number of invalidation requests received.",
			},
			[]string{"method"},
		),
		PodConfirmationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "vcib_pod_confirmations_total",
				Help: "Total number of pod dispatch outcomes by status.",
			},
			[]string{"pod", "status"},
		),
		RetriesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "vcib_retries_total",
				Help: "Total number of dispatch retries per pod.",
			},
			[]string{"pod"},
		),
		InvalidationDurationSeconds: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "vcib_invalidation_duration_seconds",
				Help:    "Total duration from dispatch start until all pods confirm, by method.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method"},
		),
		PodsDiscovered: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "vcib_pods_discovered",
				Help: "Number of ready Varnish pods discovered in the last dispatch cycle.",
			},
		),
		DispatchConcurrencyLimit: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "vcib_dispatch_concurrency_limit",
				Help: "Configured value of MAX_CONCURRENT_DISPATCHES.",
			},
		),
	}
}

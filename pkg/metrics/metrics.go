package metrics

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	once sync.Once

	registry       *prometheus.Registry
	httpRequests   *prometheus.CounterVec
	httpDuration   *prometheus.HistogramVec
	businessEvents *prometheus.CounterVec
)

func Configure(namespace, subsystem string) {
	once.Do(func() {
		if namespace == "" {
			namespace = "platform"
		}
		if subsystem == "" {
			subsystem = "service"
		}
		registry = prometheus.NewRegistry()
		httpRequests = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "http_requests_total",
				Help:      "Total HTTP requests.",
			},
			[]string{"method", "path", "status"},
		)
		httpDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "http_request_duration_seconds",
				Help:      "HTTP request duration in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "path", "status"},
		)
		businessEvents = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "business_events_total",
				Help:      "Total business events.",
			},
			[]string{"name"},
		)
		registry.MustRegister(httpRequests, httpDuration, businessEvents)
	})
}

func RecordHTTPRequest(method, path string, status int, duration time.Duration) {
	ensureConfigured()
	statusLabel := strconv.Itoa(status)
	httpRequests.WithLabelValues(method, path, statusLabel).Inc()
	httpDuration.WithLabelValues(method, path, statusLabel).Observe(duration.Seconds())
}

func IncBusinessCounter(name string) {
	ensureConfigured()
	businessEvents.WithLabelValues(name).Inc()
}

func Handler() http.Handler {
	ensureConfigured()
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

func ensureConfigured() {
	Configure("platform", "service")
}

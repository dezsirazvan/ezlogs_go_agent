package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/dezsirazvan/ezlogs_go_agent/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

// Metrics provides Prometheus metrics for the EZLogs agent
type Metrics struct {
	// Core metrics
	eventsReceived    prometheus.Counter
	eventsSent        prometheus.Counter
	eventsDropped     prometheus.Counter
	bufferSize        prometheus.Gauge
	activeConnections prometheus.Gauge

	// HTTP metrics
	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec

	// Server
	server  *http.Server
	enabled bool
}

// NewMetrics creates a new metrics instance
func NewMetrics(cfg *config.MetricsConfig) (*Metrics, error) {
	if !cfg.Enabled {
		return &Metrics{enabled: false}, nil
	}

	m := &Metrics{
		enabled: true,
		eventsReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "ezlogs_events_received_total",
			Help: "Total events received via TCP",
		}),
		eventsSent: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "ezlogs_events_sent_total",
			Help: "Total events sent to collector",
		}),
		eventsDropped: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "ezlogs_events_dropped_total",
			Help: "Total events dropped (buffer full, errors)",
		}),
		bufferSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ezlogs_buffer_size",
			Help: "Current events in buffer",
		}),
		activeConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ezlogs_active_connections",
			Help: "Active TCP connections",
		}),
		httpRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ezlogs_http_requests_total",
				Help: "HTTP requests to collector",
			},
			[]string{"status"},
		),
		httpDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "ezlogs_http_duration_seconds",
				Help:    "HTTP request duration",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"status"},
		),
	}

	// Register metrics
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		m.eventsReceived,
		m.eventsSent,
		m.eventsDropped,
		m.bufferSize,
		m.activeConnections,
		m.httpRequests,
		m.httpDuration,
	)

	// Set up HTTP server
	mux := http.NewServeMux()
	mux.Handle(cfg.Path, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	m.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler: mux,
	}

	return m, nil
}

// Start starts the metrics server if enabled
func (m *Metrics) Start(ctx context.Context) error {
	if !m.enabled {
		logrus.Info("Metrics disabled")
		return nil
	}

	go func() {
		logrus.WithField("addr", m.server.Addr).Info("Starting metrics server")
		if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Error("Metrics server error")
		}
	}()

	return nil
}

// Shutdown gracefully shuts down the metrics server
func (m *Metrics) Shutdown(ctx context.Context) error {
	if m.server != nil {
		return m.server.Shutdown(ctx)
	}
	return nil
}

// Event tracking methods (no-op if disabled)
func (m *Metrics) EventsReceived(count int) {
	if m.enabled {
		m.eventsReceived.Add(float64(count))
	}
}

func (m *Metrics) EventsSent(count int) {
	if m.enabled {
		m.eventsSent.Add(float64(count))
	}
}

func (m *Metrics) EventsDropped(count int) {
	if m.enabled {
		m.eventsDropped.Add(float64(count))
	}
}

func (m *Metrics) SetBufferSize(size int) {
	if m.enabled {
		m.bufferSize.Set(float64(size))
	}
}

func (m *Metrics) ConnectionOpened() {
	if m.enabled {
		m.activeConnections.Inc()
	}
}

func (m *Metrics) ConnectionClosed() {
	if m.enabled {
		m.activeConnections.Dec()
	}
}

func (m *Metrics) HTTPRequest(status string, duration time.Duration) {
	if m.enabled {
		m.httpRequests.WithLabelValues(status).Inc()
		m.httpDuration.WithLabelValues(status).Observe(duration.Seconds())
	}
}

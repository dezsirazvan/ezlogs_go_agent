package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/dezsirazvan/ezlogs_go_agent/internal/config"
	"github.com/sirupsen/logrus"
)

// Status represents the health status
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
)

// HealthCheck represents a single health check
type HealthCheck struct {
	Name    string                 `json:"name"`
	Status  Status                 `json:"status"`
	Message string                 `json:"message,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    Status        `json:"status"`
	Timestamp time.Time     `json:"timestamp"`
	Checks    []HealthCheck `json:"checks"`
}

// Health provides health check functionality
type Health struct {
	cfg     *config.HealthConfig
	server  *http.Server
	enabled bool

	// Health check functions
	checks map[string]func() HealthCheck
	mu     sync.RWMutex
}

// NewHealth creates a new health check instance
func NewHealth(cfg *config.HealthConfig) *Health {
	if !cfg.Enabled {
		return &Health{enabled: false}
	}

	h := &Health{
		cfg:     cfg,
		enabled: true,
		checks:  make(map[string]func() HealthCheck),
	}

	// Set up HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc(cfg.Path, h.healthHandler)
	mux.HandleFunc("/ready", h.readyHandler)

	h.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler: mux,
	}

	return h
}

// Start starts the health check server
func (h *Health) Start(ctx context.Context) error {
	if !h.enabled {
		logrus.Info("Health checks disabled")
		return nil
	}

	go func() {
		logrus.WithField("addr", h.server.Addr).Info("Starting health check server")
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Error("Health server error")
		}
	}()

	return nil
}

// Shutdown gracefully shuts down the health server
func (h *Health) Shutdown(ctx context.Context) error {
	if h.server != nil {
		return h.server.Shutdown(ctx)
	}
	return nil
}

// AddCheck adds a health check function
func (h *Health) AddCheck(name string, check func() HealthCheck) {
	if !h.enabled {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = check
}

// healthHandler handles /health requests
func (h *Health) healthHandler(w http.ResponseWriter, r *http.Request) {
	h.runChecks(w, r, false)
}

// readyHandler handles /ready requests (readiness probe)
func (h *Health) readyHandler(w http.ResponseWriter, r *http.Request) {
	h.runChecks(w, r, true)
}

// runChecks executes all health checks and returns the response
func (h *Health) runChecks(w http.ResponseWriter, r *http.Request, readiness bool) {
	h.mu.RLock()
	checks := make(map[string]func() HealthCheck)
	for k, v := range h.checks {
		checks[k] = v
	}
	h.mu.RUnlock()

	var healthChecks []HealthCheck
	overallStatus := StatusHealthy

	// Run all checks
	for name, check := range checks {
		healthCheck := check()
		healthCheck.Name = name
		healthChecks = append(healthChecks, healthCheck)

		if healthCheck.Status == StatusUnhealthy {
			overallStatus = StatusUnhealthy
		} else if healthCheck.Status == StatusDegraded && overallStatus == StatusHealthy {
			overallStatus = StatusDegraded
		}
	}

	response := HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Checks:    healthChecks,
	}

	// Set response status code
	statusCode := http.StatusOK
	if overallStatus == StatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	} else if overallStatus == StatusDegraded {
		statusCode = http.StatusOK // Degraded is still OK for health checks
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		logrus.WithError(err).Error("Failed to encode health response")
	}
}

// Default health check functions
func TCPListenerCheck(listener interface{}) func() HealthCheck {
	return func() HealthCheck {
		if listener == nil {
			return HealthCheck{
				Status:  StatusUnhealthy,
				Message: "TCP listener not started",
			}
		}
		return HealthCheck{
			Status:  StatusHealthy,
			Message: "TCP listener is running",
		}
	}
}

func BufferCheck(buffer interface{}) func() HealthCheck {
	return func() HealthCheck {
		if buffer == nil {
			return HealthCheck{
				Status:  StatusUnhealthy,
				Message: "Buffer not initialized",
			}
		}
		return HealthCheck{
			Status:  StatusHealthy,
			Message: "Buffer is operational",
		}
	}
}

func CollectorCheck(collector interface{}) func() HealthCheck {
	return func() HealthCheck {
		if collector == nil {
			return HealthCheck{
				Status:  StatusUnhealthy,
				Message: "Collector not initialized",
			}
		}
		return HealthCheck{
			Status:  StatusHealthy,
			Message: "Collector is configured",
		}
	}
}

// HTTPServerCheck creates a health check for an HTTP server
func HTTPServerCheck(server *http.Server) func() HealthCheck {
	return func() HealthCheck {
		if server == nil {
			return HealthCheck{
				Status:  StatusUnhealthy,
				Message: "HTTP server not initialized",
			}
		}
		// For now, just check if server exists
		// In a real implementation, you might want to make a test request
		return HealthCheck{
			Status:  StatusHealthy,
			Message: "HTTP server is running",
		}
	}
}

// GetStatus returns the current health status
func (h *Health) GetStatus() HealthResponse {
	h.mu.RLock()
	checks := make(map[string]func() HealthCheck)
	for k, v := range h.checks {
		checks[k] = v
	}
	h.mu.RUnlock()

	var healthChecks []HealthCheck
	overallStatus := StatusHealthy

	// Run all checks
	for name, check := range checks {
		healthCheck := check()
		healthCheck.Name = name
		healthChecks = append(healthChecks, healthCheck)

		if healthCheck.Status == StatusUnhealthy {
			overallStatus = StatusUnhealthy
		} else if healthCheck.Status == StatusDegraded && overallStatus == StatusHealthy {
			overallStatus = StatusDegraded
		}
	}

	return HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Checks:    healthChecks,
	}
}

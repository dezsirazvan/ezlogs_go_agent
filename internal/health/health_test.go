package health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dezsirazvan/ezlogs_go_agent/internal/config"
)

func TestNewHealth(t *testing.T) {
	cfg := &config.HealthConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    8080,
		Path:    "/health",
	}

	health := NewHealth(cfg)
	if health == nil {
		t.Fatal("Expected health instance to be created")
	}
	if !health.enabled {
		t.Error("Expected health to be enabled")
	}
}

func TestNewHealth_Disabled(t *testing.T) {
	cfg := &config.HealthConfig{
		Enabled: false,
	}

	health := NewHealth(cfg)
	if health == nil {
		t.Fatal("Expected health instance to be created")
	}
	if health.enabled {
		t.Error("Expected health to be disabled")
	}
}

func TestHealth_Start(t *testing.T) {
	cfg := &config.HealthConfig{
		Enabled: true,
		Host:    "127.0.0.1",
		Port:    8081, // Use a specific port for testing
		Path:    "/health",
	}

	health := NewHealth(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start health server
	err := health.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start health server: %v", err)
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test health endpoint - use the actual server address
	resp, err := http.Get("http://" + health.server.Addr + "/health")
	if err != nil {
		t.Fatalf("Failed to get health endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Parse response
	var response HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != StatusHealthy {
		t.Errorf("Expected status healthy, got %s", response.Status)
	}
}

func TestHealth_StartDisabled(t *testing.T) {
	cfg := &config.HealthConfig{
		Enabled: false,
	}

	health := NewHealth(cfg)
	ctx := context.Background()

	// Start should return immediately for disabled health
	err := health.Start(ctx)
	if err != nil {
		t.Errorf("Expected no error for disabled health, got %v", err)
	}
}

func TestHealth_Shutdown(t *testing.T) {
	cfg := &config.HealthConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    0,
		Path:    "/health",
	}

	health := NewHealth(cfg)
	ctx := context.Background()

	// Start health server
	if err := health.Start(ctx); err != nil {
		t.Fatalf("Failed to start health server: %v", err)
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	err := health.Shutdown(shutdownCtx)
	if err != nil {
		t.Errorf("Failed to shutdown health server: %v", err)
	}
}

func TestHealth_AddCheck(t *testing.T) {
	cfg := &config.HealthConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    0,
		Path:    "/health",
	}

	health := NewHealth(cfg)

	// Add a custom health check
	health.AddCheck("custom", func() HealthCheck {
		return HealthCheck{
			Status:  StatusHealthy,
			Message: "Custom check passed",
		}
	})

	// Verify check was added
	health.mu.RLock()
	_, exists := health.checks["custom"]
	health.mu.RUnlock()

	if !exists {
		t.Error("Expected custom check to be added")
	}
}

func TestHealth_AddCheckDisabled(t *testing.T) {
	cfg := &config.HealthConfig{
		Enabled: false,
	}

	health := NewHealth(cfg)

	// Add check should be no-op when disabled
	health.AddCheck("custom", func() HealthCheck {
		return HealthCheck{
			Status:  StatusHealthy,
			Message: "Custom check passed",
		}
	})

	// Verify no checks were added
	health.mu.RLock()
	checkCount := len(health.checks)
	health.mu.RUnlock()

	if checkCount != 0 {
		t.Errorf("Expected no checks when disabled, got %d", checkCount)
	}
}

func TestHealth_healthHandler(t *testing.T) {
	cfg := &config.HealthConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    0,
		Path:    "/health",
	}

	health := NewHealth(cfg)

	// Add a test check
	health.AddCheck("test", func() HealthCheck {
		return HealthCheck{
			Status:  StatusHealthy,
			Message: "Test check passed",
		}
	})

	// Create test request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Call health handler
	health.healthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != StatusHealthy {
		t.Errorf("Expected status healthy, got %s", response.Status)
	}

	if len(response.Checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(response.Checks))
	}

	if response.Checks[0].Name != "test" {
		t.Errorf("Expected check name 'test', got %s", response.Checks[0].Name)
	}
}

func TestHealth_readyHandler(t *testing.T) {
	cfg := &config.HealthConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    0,
		Path:    "/health",
	}

	health := NewHealth(cfg)

	// Add a test check
	health.AddCheck("test", func() HealthCheck {
		return HealthCheck{
			Status:  StatusHealthy,
			Message: "Test check passed",
		}
	})

	// Create test request
	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()

	// Call ready handler
	health.readyHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != StatusHealthy {
		t.Errorf("Expected status healthy, got %s", response.Status)
	}
}

func TestHealth_UnhealthyCheck(t *testing.T) {
	cfg := &config.HealthConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    0,
		Path:    "/health",
	}

	health := NewHealth(cfg)

	// Add an unhealthy check
	health.AddCheck("unhealthy", func() HealthCheck {
		return HealthCheck{
			Status:  StatusUnhealthy,
			Message: "Check failed",
		}
	})

	// Create test request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Call health handler
	health.healthHandler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}

	// Parse response
	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != StatusUnhealthy {
		t.Errorf("Expected status unhealthy, got %s", response.Status)
	}
}

func TestHealth_DegradedCheck(t *testing.T) {
	cfg := &config.HealthConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    0,
		Path:    "/health",
	}

	health := NewHealth(cfg)

	// Add a degraded check
	health.AddCheck("degraded", func() HealthCheck {
		return HealthCheck{
			Status:  StatusDegraded,
			Message: "Check degraded",
		}
	})

	// Create test request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Call health handler
	health.healthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for degraded, got %d", w.Code)
	}

	// Parse response
	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != StatusDegraded {
		t.Errorf("Expected status degraded, got %s", response.Status)
	}
}

func TestHealth_GetStatus(t *testing.T) {
	cfg := &config.HealthConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    0,
		Path:    "/health",
	}

	health := NewHealth(cfg)

	// Add a test check
	health.AddCheck("test", func() HealthCheck {
		return HealthCheck{
			Status:  StatusHealthy,
			Message: "Test check passed",
		}
	})

	// Get status
	status := health.GetStatus()

	if status.Status != StatusHealthy {
		t.Errorf("Expected status healthy, got %s", status.Status)
	}

	if len(status.Checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(status.Checks))
	}
}

func TestDefaultHealthChecks(t *testing.T) {
	// Test TCP listener check
	tcpCheck := TCPListenerCheck(nil)
	check := tcpCheck()
	if check.Status != StatusUnhealthy {
		t.Errorf("Expected unhealthy status for nil listener, got %s", check.Status)
	}

	// Test buffer check
	bufferCheck := BufferCheck(nil)
	check = bufferCheck()
	if check.Status != StatusUnhealthy {
		t.Errorf("Expected unhealthy status for nil buffer, got %s", check.Status)
	}

	// Test collector check
	collectorCheck := CollectorCheck(nil)
	check = collectorCheck()
	if check.Status != StatusUnhealthy {
		t.Errorf("Expected unhealthy status for nil collector, got %s", check.Status)
	}

	// Test HTTP server check
	httpCheck := HTTPServerCheck(nil)
	check = httpCheck()
	if check.Status != StatusUnhealthy {
		t.Errorf("Expected unhealthy status for nil HTTP server, got %s", check.Status)
	}
}

func TestHealth_ConcurrentAccess(t *testing.T) {
	cfg := &config.HealthConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    0,
		Path:    "/health",
	}

	health := NewHealth(cfg)

	// Add checks concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			health.AddCheck("check"+string(rune(id)), func() HealthCheck {
				return HealthCheck{
					Status:  StatusHealthy,
					Message: "Concurrent check",
				}
			})
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify checks were added
	health.mu.RLock()
	checkCount := len(health.checks)
	health.mu.RUnlock()

	if checkCount != 10 {
		t.Errorf("Expected 10 checks, got %d", checkCount)
	}
}

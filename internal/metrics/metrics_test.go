package metrics

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/dezsirazvan/ezlogs_go_agent/internal/config"
)

func TestNewMetrics(t *testing.T) {
	cfg := &config.MetricsConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    9090,
		Path:    "/metrics",
	}

	metrics, err := NewMetrics(cfg)
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}
	if metrics == nil {
		t.Fatal("Expected metrics instance to be created")
	}
	if !metrics.enabled {
		t.Error("Expected metrics to be enabled")
	}
}

func TestNewMetrics_Disabled(t *testing.T) {
	cfg := &config.MetricsConfig{
		Enabled: false,
	}

	metrics, err := NewMetrics(cfg)
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}
	if metrics == nil {
		t.Fatal("Expected metrics instance to be created")
	}
	if metrics.enabled {
		t.Error("Expected metrics to be disabled")
	}
}

func TestMetrics_Start(t *testing.T) {
	cfg := &config.MetricsConfig{
		Enabled: true,
		Host:    "127.0.0.1",
		Port:    9091, // Use a specific port for testing
		Path:    "/metrics",
	}

	metrics, err := NewMetrics(cfg)
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start metrics server
	err = metrics.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start metrics server: %v", err)
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test metrics endpoint - use the actual server address
	resp, err := http.Get("http://" + metrics.server.Addr + "/metrics")
	if err != nil {
		t.Fatalf("Failed to get metrics endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestMetrics_StartDisabled(t *testing.T) {
	cfg := &config.MetricsConfig{
		Enabled: false,
	}

	metrics, err := NewMetrics(cfg)
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}

	ctx := context.Background()

	// Start should return immediately for disabled metrics
	err = metrics.Start(ctx)
	if err != nil {
		t.Errorf("Expected no error for disabled metrics, got %v", err)
	}
}

func TestMetrics_Shutdown(t *testing.T) {
	cfg := &config.MetricsConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    0,
		Path:    "/metrics",
	}

	metrics, err := NewMetrics(cfg)
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}

	ctx := context.Background()

	// Start metrics server
	if err := metrics.Start(ctx); err != nil {
		t.Fatalf("Failed to start metrics server: %v", err)
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	err = metrics.Shutdown(shutdownCtx)
	if err != nil {
		t.Errorf("Failed to shutdown metrics server: %v", err)
	}
}

func TestMetrics_EventsReceived(t *testing.T) {
	cfg := &config.MetricsConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    0,
		Path:    "/metrics",
	}

	metrics, err := NewMetrics(cfg)
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}

	// Record some events
	metrics.EventsReceived(1)
	metrics.EventsReceived(5)
	metrics.EventsReceived(10)

	// Verify counter was incremented
	// Note: We can't easily test the actual counter value without exposing internal state
	// The test verifies the function doesn't panic and can be called multiple times
}

func TestMetrics_EventsSent(t *testing.T) {
	cfg := &config.MetricsConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    0,
		Path:    "/metrics",
	}

	metrics, err := NewMetrics(cfg)
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}

	// Record some sent events
	metrics.EventsSent(1)
	metrics.EventsSent(5)
	metrics.EventsSent(10)

	// Verify counter was incremented
	// Note: We can't easily test the actual counter value without exposing internal state
	// The test verifies the function doesn't panic and can be called multiple times
}

func TestMetrics_EventsDropped(t *testing.T) {
	cfg := &config.MetricsConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    0,
		Path:    "/metrics",
	}

	metrics, err := NewMetrics(cfg)
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}

	// Record some dropped events
	metrics.EventsDropped(1)
	metrics.EventsDropped(2)

	// Verify counter was incremented
	// Note: We can't easily test the actual counter value without exposing internal state
	// The test verifies the function doesn't panic and can be called multiple times
}

func TestMetrics_SetBufferSize(t *testing.T) {
	cfg := &config.MetricsConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    0,
		Path:    "/metrics",
	}

	metrics, err := NewMetrics(cfg)
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}

	// Record some buffer sizes
	metrics.SetBufferSize(100)
	metrics.SetBufferSize(200)
	metrics.SetBufferSize(50)

	// Verify gauge was updated
	// Note: We can't easily test the actual gauge value without exposing internal state
	// The test verifies the function doesn't panic and can be called multiple times
}

func TestMetrics_ConnectionTracking(t *testing.T) {
	cfg := &config.MetricsConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    0,
		Path:    "/metrics",
	}

	metrics, err := NewMetrics(cfg)
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}

	// Track connections
	metrics.ConnectionOpened()
	metrics.ConnectionOpened()
	metrics.ConnectionClosed()
	metrics.ConnectionOpened()
	metrics.ConnectionClosed()
	metrics.ConnectionClosed()

	// Verify gauge was updated
	// Note: We can't easily test the actual gauge value without exposing internal state
	// The test verifies the function doesn't panic and can be called multiple times
}

func TestMetrics_HTTPRequest(t *testing.T) {
	cfg := &config.MetricsConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    0,
		Path:    "/metrics",
	}

	metrics, err := NewMetrics(cfg)
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}

	// Record some HTTP requests
	metrics.HTTPRequest("200", 100*time.Millisecond)
	metrics.HTTPRequest("404", 50*time.Millisecond)
	metrics.HTTPRequest("500", 200*time.Millisecond)

	// Verify metrics were updated
	// Note: We can't easily test the actual metric values without exposing internal state
	// The test verifies the function doesn't panic and can be called multiple times
}

func TestMetrics_DisabledOperations(t *testing.T) {
	cfg := &config.MetricsConfig{
		Enabled: false,
	}

	metrics, err := NewMetrics(cfg)
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}

	// All operations should be no-ops when disabled
	metrics.EventsReceived(10)
	metrics.EventsSent(5)
	metrics.EventsDropped(2)
	metrics.SetBufferSize(100)
	metrics.ConnectionOpened()
	metrics.ConnectionClosed()
	metrics.HTTPRequest("200", 100*time.Millisecond)

	// Verify no panic occurred
}

func TestMetrics_ConcurrentAccess(t *testing.T) {
	cfg := &config.MetricsConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    0,
		Path:    "/metrics",
	}

	metrics, err := NewMetrics(cfg)
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}

	// Test concurrent metric recording
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			metrics.EventsReceived(1)
			metrics.EventsSent(1)
			metrics.SetBufferSize(100)
			metrics.ConnectionOpened()
			metrics.ConnectionClosed()
			metrics.HTTPRequest("200", 100*time.Millisecond)
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify no panic occurred
}

func TestMetrics_InvalidPort(t *testing.T) {
	cfg := &config.MetricsConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    99999, // Invalid port
		Path:    "/metrics",
	}

	metrics, err := NewMetrics(cfg)
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}

	ctx := context.Background()

	// Start should fail due to invalid port, but the error is handled internally
	// The server will log an error but Start() returns nil
	err = metrics.Start(ctx)
	if err != nil {
		t.Errorf("Expected no error from Start() for invalid port, got %v", err)
	}

	// Give server time to attempt to start
	time.Sleep(100 * time.Millisecond)
}

func TestMetrics_ServerShutdown(t *testing.T) {
	cfg := &config.MetricsConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    0,
		Path:    "/metrics",
	}

	metrics, err := NewMetrics(cfg)
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}

	ctx := context.Background()

	// Start metrics server
	if err := metrics.Start(ctx); err != nil {
		t.Fatalf("Failed to start metrics server: %v", err)
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	err = metrics.Shutdown(shutdownCtx)
	if err != nil {
		t.Errorf("Failed to shutdown metrics server: %v", err)
	}

	// Try to access metrics endpoint - should fail
	_, err = http.Get("http://" + metrics.server.Addr + "/metrics")
	if err == nil {
		t.Error("Expected error when accessing shutdown server")
	}
}

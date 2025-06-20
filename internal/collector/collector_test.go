package collector

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dezsirazvan/ezlogs_go_agent/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestNewCollector(t *testing.T) {
	cfg := &config.CollectorConfig{
		BaseURL:             "https://test-collector.com",
		APIKey:              "test-api-key",
		Timeout:             30 * time.Second,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		CircuitBreaker: config.CircuitBreakerConfig{
			FailureThreshold: 5,
			Timeout:          60 * time.Second,
			SuccessThreshold: 3,
		},
	}

	collector := NewCollector(cfg, "test-api-key")
	assert.NotNil(t, collector)
	assert.Equal(t, "test-api-key", collector.apiKey)
	assert.Equal(t, "https://test-collector.com", collector.baseURL)
	assert.NotNil(t, collector.cb)
}

func TestCollector_SendBatch_Empty(t *testing.T) {
	cfg := &config.CollectorConfig{
		BaseURL:             "https://test-collector.com",
		APIKey:              "test-api-key",
		Timeout:             30 * time.Second,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		CircuitBreaker: config.CircuitBreakerConfig{
			FailureThreshold: 5,
			Timeout:          60 * time.Second,
			SuccessThreshold: 3,
		},
	}

	collector := NewCollector(cfg, "test-api-key")
	err := collector.SendBatch(context.Background(), []json.RawMessage{})
	assert.NoError(t, err)
}

func TestCollector_SendBatch_WithMockServer(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/events", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
		assert.Equal(t, "ezlogs-go-agent/1.0", r.Header.Get("User-Agent"))

		// Read and verify body
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.Equal(t, `[{"test":"data"}]`, string(body))

		// Return success
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	// Create collector with mock server URL
	cfg := &config.CollectorConfig{
		BaseURL:             server.URL,
		APIKey:              "test-api-key",
		Timeout:             30 * time.Second,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		CircuitBreaker: config.CircuitBreakerConfig{
			FailureThreshold: 5,
			Timeout:          60 * time.Second,
			SuccessThreshold: 3,
		},
	}

	collector := NewCollector(cfg, "test-api-key")

	// Send test batch
	batch := []json.RawMessage{json.RawMessage(`{"test":"data"}`)}
	err := collector.SendBatch(context.Background(), batch)
	assert.NoError(t, err)
}

func TestCollector_SendBatch_ServerError(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer server.Close()

	// Create collector with mock server URL
	cfg := &config.CollectorConfig{
		BaseURL:             server.URL,
		APIKey:              "test-api-key",
		Timeout:             30 * time.Second,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		CircuitBreaker: config.CircuitBreakerConfig{
			FailureThreshold: 5,
			Timeout:          60 * time.Second,
			SuccessThreshold: 3,
		},
	}

	collector := NewCollector(cfg, "test-api-key")

	// Send test batch
	batch := []json.RawMessage{json.RawMessage(`{"test":"data"}`)}
	err := collector.SendBatch(context.Background(), batch)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 500")
}

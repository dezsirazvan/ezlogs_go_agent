package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/dezsirazvan/ezlogs_go_agent/internal/config"
	"github.com/sirupsen/logrus"
)

// Errors
var (
	ErrCircuitBreakerOpen = fmt.Errorf("circuit breaker is open")
)

// Collector is responsible for forwarding event batches to the remote EZLogs API.
type Collector struct {
	cfg     *config.CollectorConfig
	client  *http.Client
	apiKey  string
	cb      *CircuitBreaker
	baseURL string
}

// NewCollector creates a new Collector instance.
func NewCollector(cfg *config.CollectorConfig, apiKey string) *Collector {
	client := &http.Client{
		Timeout: cfg.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        cfg.MaxIdleConns,
			MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
			IdleConnTimeout:     cfg.IdleConnTimeout,
		},
	}

	// Create circuit breaker
	cb := NewCircuitBreaker(
		cfg.CircuitBreaker.FailureThreshold,
		cfg.CircuitBreaker.Timeout,
		cfg.CircuitBreaker.SuccessThreshold,
	)

	return &Collector{
		cfg:     cfg,
		client:  client,
		apiKey:  apiKey,
		cb:      cb,
		baseURL: cfg.BaseURL,
	}
}

// SendBatch sends a batch of events to the collector service
func (c *Collector) SendBatch(ctx context.Context, batch []json.RawMessage) error {
	return c.cb.Execute(ctx, func() error {
		return c.sendBatchInternal(ctx, batch)
	})
}

// sendBatchInternal performs the actual HTTP request
func (c *Collector) sendBatchInternal(ctx context.Context, batch []json.RawMessage) error {
	if len(batch) == 0 {
		return nil
	}

	// Prepare request body
	body, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("failed to marshal batch: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/events", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", "ezlogs-go-agent/1.0")

	// Send request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check response status
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	logrus.WithFields(logrus.Fields{
		"status_code": resp.StatusCode,
		"batch_size":  len(batch),
	}).Debug("Successfully sent batch to collector")

	return nil
}

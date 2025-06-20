package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/dezsirazvan/ezlogs_go_agent/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfigForIntegration(collectorURL string, port int) *config.Config {
	cfg := config.DefaultConfig()
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = port
	cfg.Collector.BaseURL = collectorURL
	cfg.Collector.CircuitBreaker = config.CircuitBreakerConfig{
		FailureThreshold: 5,
		Timeout:          60 * time.Second,
		SuccessThreshold: 3,
	}
	cfg.Buffer.MaxSize = 100
	cfg.Buffer.BatchSize = 10
	cfg.Buffer.FlushInterval = 100 * time.Millisecond
	cfg.Logging.Level = "error"
	// Disable HTTP server for this test to avoid port conflicts
	cfg.Health.Enabled = false
	cfg.Metrics.Enabled = false
	return cfg
}

func TestAgent_Integration_TCPToCollector(t *testing.T) {
	// 1. Start mock collector
	received := make(chan []json.RawMessage, 10)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var batch []json.RawMessage
		err := json.NewDecoder(r.Body).Decode(&batch)
		assert.NoError(t, err)
		received <- batch
		w.WriteHeader(200)
	}))
	defer ts.Close()

	// 2. Pick a random port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	// 3. Start agent
	cfg := testConfigForIntegration(ts.URL, port)
	agent, err := NewAgent(cfg)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		err := agent.Start(ctx)
		if err != nil && err.Error() != "context canceled" {
			fmt.Fprintf(os.Stderr, "agent error: %v\n", err)
		}
	}()
	// Wait for agent to start
	time.Sleep(100 * time.Millisecond)

	// 4. Connect to agent and send a Ruby event batch
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	require.NoError(t, err)

	// Send Ruby event format
	rubyBatch := `[
		{
			"event_type": "test.event",
			"action": "test",
			"actor": {"type": "user", "id": "123"},
			"metadata": {"test": "data"}
		}
	]`
	_, err = conn.Write([]byte(rubyBatch))
	require.NoError(t, err)
	conn.Close()

	// 5. Wait for collector to receive the batch
	select {
	case got := <-received:
		assert.Equal(t, 1, len(got))
		// The event should be transformed to collector format
		var event map[string]interface{}
		err := json.Unmarshal(got[0], &event)
		assert.NoError(t, err)
		assert.Equal(t, "test.event", event["event_type"])
		assert.Equal(t, "test", event["action"])
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for collector to receive batch")
	}

	// 6. Test graceful shutdown
	cancel()
	time.Sleep(100 * time.Millisecond)
	err = agent.Shutdown(context.Background())
	assert.NoError(t, err)
}

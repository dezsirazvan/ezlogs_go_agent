package config

import (
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Test server config
	assert.Equal(t, "0.0.0.0", config.Server.Host)
	assert.Equal(t, 9000, config.Server.Port)

	// Test collector config
	assert.Equal(t, "https://collector.ezlogs.com", config.Collector.BaseURL)
	assert.Equal(t, "", config.Collector.APIKey)
	assert.Equal(t, 30*time.Second, config.Collector.Timeout)
	assert.Equal(t, 100, config.Collector.MaxIdleConns)
	assert.Equal(t, 10, config.Collector.MaxIdleConnsPerHost)
	assert.Equal(t, 90*time.Second, config.Collector.IdleConnTimeout)
	assert.Equal(t, 5, config.Collector.CircuitBreaker.FailureThreshold)
	assert.Equal(t, 60*time.Second, config.Collector.CircuitBreaker.Timeout)
	assert.Equal(t, 3, config.Collector.CircuitBreaker.SuccessThreshold)

	// Test buffer config
	assert.Equal(t, 50000, config.Buffer.MaxSize)
	assert.Equal(t, 5*time.Second, config.Buffer.FlushInterval)
	assert.Equal(t, 1000, config.Buffer.BatchSize)

	// Test security config
	assert.False(t, config.Security.EnableTLS)
	assert.Equal(t, "", config.Security.TLSCertFile)
	assert.Equal(t, "", config.Security.TLSKeyFile)
	assert.Contains(t, config.Security.AllowedHosts, "127.0.0.1")
	assert.Equal(t, 10000, config.Security.RateLimit)

	// Test logging config
	assert.Equal(t, "info", config.Logging.Level)
	assert.Equal(t, "json", config.Logging.Format)
	assert.Equal(t, "stdout", config.Logging.Output)

	// Test metrics config
	assert.True(t, config.Metrics.Enabled)
	assert.Equal(t, "0.0.0.0", config.Metrics.Host)
	assert.Equal(t, 9090, config.Metrics.Port)
	assert.Equal(t, "/metrics", config.Metrics.Path)

	// Test health config
	assert.True(t, config.Health.Enabled)
	assert.Equal(t, "0.0.0.0", config.Health.Host)
	assert.Equal(t, 8080, config.Health.Port)
	assert.Equal(t, "/health", config.Health.Path)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "invalid server port - too low",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Server.Port = 0
				return cfg
			}(),
			wantErr: true,
			errMsg:  "invalid server port: 0 (must be 1-65535)",
		},
		{
			name: "invalid server port - too high",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Server.Port = 70000
				return cfg
			}(),
			wantErr: true,
			errMsg:  "invalid server port: 70000 (must be 1-65535)",
		},
		{
			name: "missing collector endpoint",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Collector.BaseURL = ""
				return cfg
			}(),
			wantErr: true,
			errMsg:  "collector base URL is required",
		},
		{
			name: "invalid collector timeout",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Collector.Timeout = 0
				return cfg
			}(),
			wantErr: true,
			errMsg:  "collector timeout must be positive",
		},
		{
			name: "invalid max idle conns",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Collector.MaxIdleConns = -1
				return cfg
			}(),
			wantErr: true,
			errMsg:  "max idle connections cannot be negative",
		},
		{
			name: "invalid max idle conns per host",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Collector.MaxIdleConnsPerHost = -1
				return cfg
			}(),
			wantErr: true,
			errMsg:  "max idle connections per host cannot be negative",
		},
		{
			name: "invalid idle conn timeout",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Collector.IdleConnTimeout = 0
				return cfg
			}(),
			wantErr: true,
			errMsg:  "idle connection timeout must be positive",
		},
		{
			name: "invalid failure threshold",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Collector.CircuitBreaker.FailureThreshold = -1
				return cfg
			}(),
			wantErr: true,
			errMsg:  "failure threshold cannot be negative",
		},
		{
			name: "invalid timeout",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Collector.CircuitBreaker.Timeout = 0
				return cfg
			}(),
			wantErr: true,
			errMsg:  "timeout must be positive",
		},
		{
			name: "invalid success threshold",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Collector.CircuitBreaker.SuccessThreshold = -1
				return cfg
			}(),
			wantErr: true,
			errMsg:  "success threshold cannot be negative",
		},
		{
			name: "invalid buffer max size",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Buffer.MaxSize = 0
				return cfg
			}(),
			wantErr: true,
			errMsg:  "buffer max size must be positive",
		},
		{
			name: "invalid flush interval",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Buffer.FlushInterval = 0
				return cfg
			}(),
			wantErr: true,
			errMsg:  "flush interval must be positive",
		},
		{
			name: "invalid batch size",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Buffer.BatchSize = 0
				return cfg
			}(),
			wantErr: true,
			errMsg:  "batch size must be positive",
		},
		{
			name: "batch size exceeds max size",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Buffer.MaxSize = 100
				cfg.Buffer.BatchSize = 200
				return cfg
			}(),
			wantErr: true,
			errMsg:  "batch size cannot exceed buffer max size",
		},
		{
			name: "invalid rate limit",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Security.RateLimit = 0
				return cfg
			}(),
			wantErr: true,
			errMsg:  "rate limit must be positive",
		},
		{
			name: "invalid logging level",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Logging.Level = "invalid"
				return cfg
			}(),
			wantErr: true,
			errMsg:  "invalid logging level: invalid",
		},
		{
			name: "invalid logging format",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Logging.Format = "invalid"
				return cfg
			}(),
			wantErr: true,
			errMsg:  "invalid logging format: invalid",
		},
		{
			name: "invalid metrics port",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Metrics.Port = 70000
				return cfg
			}(),
			wantErr: true,
			errMsg:  "invalid metrics port: 70000 (must be 1-65535)",
		},
		{
			name: "invalid health port",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Health.Port = 70000
				return cfg
			}(),
			wantErr: true,
			errMsg:  "invalid health port: 70000 (must be 1-65535)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	// Create a temporary config file
	configData := `
server:
  host: "192.168.1.100"
  port: 9002
collector:
  base_url: "https://test-collector.com"
  api_key: "test-api-key"
  timeout: "60s"
buffer:
  max_size: 20000
  flush_interval: "10s"
  batch_size: 500
security:
  allowed_hosts: ["127.0.0.1"]
  rate_limit: 5000
logging:
  level: "debug"
  format: "text"
metrics:
  enabled: true
  host: "0.0.0.0"
  port: 9091
  path: "/metrics"
health:
  enabled: true
  host: "0.0.0.0"
  port: 8081
  path: "/health"
`
	tmpfile, err := os.CreateTemp("", "ezlogs-config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write([]byte(configData))
	require.NoError(t, err)
	tmpfile.Close()

	// Load configuration from file
	config, err := LoadConfig(tmpfile.Name())
	require.NoError(t, err)

	// Test that configuration was loaded correctly
	assert.Equal(t, "192.168.1.100", config.Server.Host)
	assert.Equal(t, 9002, config.Server.Port)
	assert.Equal(t, "https://test-collector.com", config.Collector.BaseURL)
	assert.Equal(t, "test-api-key", config.Collector.APIKey)
	assert.Equal(t, 20000, config.Buffer.MaxSize)
	assert.Equal(t, 10*time.Second, config.Buffer.FlushInterval)
	assert.Equal(t, 500, config.Buffer.BatchSize)
	assert.Equal(t, []string{"127.0.0.1"}, config.Security.AllowedHosts)
	assert.Equal(t, 5000, config.Security.RateLimit)
	assert.Equal(t, "debug", config.Logging.Level)
	assert.Equal(t, "text", config.Logging.Format)
	assert.True(t, config.Metrics.Enabled)
	assert.Equal(t, "0.0.0.0", config.Metrics.Host)
	assert.Equal(t, 9091, config.Metrics.Port)
	assert.Equal(t, "/metrics", config.Metrics.Path)
	assert.True(t, config.Health.Enabled)
	assert.Equal(t, "0.0.0.0", config.Health.Host)
	assert.Equal(t, 8081, config.Health.Port)
	assert.Equal(t, "/health", config.Health.Path)
}

func TestLoadConfigFromEnvironment(t *testing.T) {
	// Set environment variables
	os.Setenv("EZLOGS_SERVER_HOST", "192.168.1.100")
	os.Setenv("EZLOGS_SERVER_PORT", "9002")
	os.Setenv("EZLOGS_COLLECTOR_BASE_URL", "https://env-collector.com")
	os.Setenv("EZLOGS_COLLECTOR_API_KEY", "env-api-key")
	os.Setenv("EZLOGS_BUFFER_MAX_SIZE", "20000")
	os.Setenv("EZLOGS_LOGGING_LEVEL", "warn")
	defer func() {
		os.Unsetenv("EZLOGS_SERVER_HOST")
		os.Unsetenv("EZLOGS_SERVER_PORT")
		os.Unsetenv("EZLOGS_COLLECTOR_BASE_URL")
		os.Unsetenv("EZLOGS_COLLECTOR_API_KEY")
		os.Unsetenv("EZLOGS_BUFFER_MAX_SIZE")
		os.Unsetenv("EZLOGS_LOGGING_LEVEL")
	}()

	// Load configuration
	config, err := LoadConfig("")
	require.NoError(t, err)

	// Test that configuration was loaded correctly
	assert.Equal(t, "192.168.1.100", config.Server.Host)
	assert.Equal(t, 9002, config.Server.Port)
	assert.Equal(t, "https://env-collector.com", config.Collector.BaseURL)
	assert.Equal(t, "env-api-key", config.Collector.APIKey)
	assert.Equal(t, 20000, config.Buffer.MaxSize)
	assert.Equal(t, "warn", config.Logging.Level)
}

func TestConfigHelperMethods(t *testing.T) {
	config := DefaultConfig()

	// Test GetServerAddr
	assert.Equal(t, "0.0.0.0:9000", config.GetServerAddr())

	// Test GetMetricsAddr
	assert.Equal(t, "0.0.0.0:9090", config.GetMetricsAddr())

	// Test GetHealthAddr
	assert.Equal(t, "0.0.0.0:8080", config.GetHealthAddr())

	// Test IsAPIKeySet
	assert.False(t, config.IsAPIKeySet())
	config.Collector.APIKey = "test-key"
	assert.True(t, config.IsAPIKeySet())

	// Test GetLogLevel
	assert.Equal(t, logrus.InfoLevel, config.GetLogLevel())
	config.Logging.Level = "debug"
	assert.Equal(t, logrus.DebugLevel, config.GetLogLevel())
	config.Logging.Level = "warn"
	assert.Equal(t, logrus.WarnLevel, config.GetLogLevel())
	config.Logging.Level = "error"
	assert.Equal(t, logrus.ErrorLevel, config.GetLogLevel())
	config.Logging.Level = "invalid"
	assert.Equal(t, logrus.InfoLevel, config.GetLogLevel()) // defaults to info
}

func TestLoadConfigInvalidFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/file.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	// Create a temporary config file with invalid YAML
	configContent := `
server:
  host: "127.0.0.1"
  port: "invalid-port"  # should be int, not string
`

	tmpfile, err := os.CreateTemp("", "ezlogs-config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write([]byte(configContent))
	require.NoError(t, err)
	tmpfile.Close()

	// Load config from file
	_, err = LoadConfig(tmpfile.Name())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal config")
}

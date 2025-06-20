package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Config holds all agent configuration settings
type Config struct {
	// Server configuration
	Server ServerConfig `mapstructure:"server" yaml:"server"`

	// Collector configuration
	Collector CollectorConfig `mapstructure:"collector" yaml:"collector"`

	// Buffer configuration
	Buffer BufferConfig `mapstructure:"buffer" yaml:"buffer"`

	// Security configuration
	Security SecurityConfig `mapstructure:"security" yaml:"security"`

	// Logging configuration
	Logging LoggingConfig `mapstructure:"logging" yaml:"logging"`

	// Metrics configuration
	Metrics MetricsConfig `mapstructure:"metrics" yaml:"metrics"`

	// Health check configuration
	Health HealthConfig `mapstructure:"health" yaml:"health"`
}

// ServerConfig holds TCP server settings
type ServerConfig struct {
	Host string `mapstructure:"host" yaml:"host"`
	Port int    `mapstructure:"port" yaml:"port"`
}

// CollectorConfig holds configuration for the collector service
type CollectorConfig struct {
	BaseURL string        `yaml:"base_url" env:"COLLECTOR_BASE_URL" default:"https://collector.ezlogs.com"`
	APIKey  string        `yaml:"api_key" env:"COLLECTOR_API_KEY" required:"true"`
	Timeout time.Duration `yaml:"timeout" env:"COLLECTOR_TIMEOUT" default:"30s"`

	// HTTP transport settings
	MaxIdleConns        int           `yaml:"max_idle_conns" env:"COLLECTOR_MAX_IDLE_CONNS" default:"100"`
	MaxIdleConnsPerHost int           `yaml:"max_idle_conns_per_host" env:"COLLECTOR_MAX_IDLE_CONNS_PER_HOST" default:"10"`
	IdleConnTimeout     time.Duration `yaml:"idle_conn_timeout" env:"COLLECTOR_IDLE_CONN_TIMEOUT" default:"90s"`

	// Circuit breaker settings
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
}

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	FailureThreshold int           `yaml:"failure_threshold" env:"COLLECTOR_CB_FAILURE_THRESHOLD" default:"5"`
	Timeout          time.Duration `yaml:"timeout" env:"COLLECTOR_CB_TIMEOUT" default:"60s"`
	SuccessThreshold int           `yaml:"success_threshold" env:"COLLECTOR_CB_SUCCESS_THRESHOLD" default:"3"`
}

// BufferConfig holds event buffering settings
type BufferConfig struct {
	MaxSize       int           `mapstructure:"max_size" yaml:"max_size"`
	FlushInterval time.Duration `mapstructure:"flush_interval" yaml:"flush_interval"`
	BatchSize     int           `mapstructure:"batch_size" yaml:"batch_size"`
}

// SecurityConfig holds security-related settings
type SecurityConfig struct {
	EnableTLS    bool     `mapstructure:"enable_tls" yaml:"enable_tls"`
	TLSCertFile  string   `mapstructure:"tls_cert_file" yaml:"tls_cert_file"`
	TLSKeyFile   string   `mapstructure:"tls_key_file" yaml:"tls_key_file"`
	AllowedHosts []string `mapstructure:"allowed_hosts" yaml:"allowed_hosts"`
	RateLimit    int      `mapstructure:"rate_limit" yaml:"rate_limit"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level  string `mapstructure:"level" yaml:"level"`
	Format string `mapstructure:"format" yaml:"format"`
	Output string `mapstructure:"output" yaml:"output"`
}

// MetricsConfig holds metrics collection settings
type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled" yaml:"enabled"`
	Host    string `mapstructure:"host" yaml:"host"`
	Port    int    `mapstructure:"port" yaml:"port"`
	Path    string `mapstructure:"path" yaml:"path"`
}

// HealthConfig holds health check settings
type HealthConfig struct {
	Enabled bool   `mapstructure:"enabled" yaml:"enabled"`
	Host    string `mapstructure:"host" yaml:"host"`
	Port    int    `mapstructure:"port" yaml:"port"`
	Path    string `mapstructure:"path" yaml:"path"`
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 9000,
		},
		Collector: CollectorConfig{
			BaseURL:             "https://collector.ezlogs.com",
			APIKey:              "",
			Timeout:             30 * time.Second,
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			CircuitBreaker: CircuitBreakerConfig{
				FailureThreshold: 5,
				Timeout:          60 * time.Second,
				SuccessThreshold: 3,
			},
		},
		Buffer: BufferConfig{
			MaxSize:       50000,
			FlushInterval: 5 * time.Second,
			BatchSize:     1000,
		},
		Security: SecurityConfig{
			EnableTLS:    false,
			TLSCertFile:  "",
			TLSKeyFile:   "",
			AllowedHosts: []string{"127.0.0.1", "localhost"},
			RateLimit:    10000, // events per second
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Host:    "0.0.0.0",
			Port:    9090,
			Path:    "/metrics",
		},
		Health: HealthConfig{
			Enabled: true,
			Host:    "0.0.0.0",
			Port:    8080,
			Path:    "/health",
		},
	}
}

// LoadConfig loads configuration from file, environment variables, and flags
func LoadConfig(configPath string) (*Config, error) {
	// Set up Viper
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigName("ezlogs-agent")
	v.AddConfigPath(".")
	v.AddConfigPath("./configs")
	v.AddConfigPath("/etc/ezlogs")
	v.AddConfigPath("$HOME/.ezlogs")

	// Set environment variable prefix
	v.SetEnvPrefix("EZLOGS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Set defaults
	setDefaults(v)

	// Bind environment variables
	bindEnvVars(v)

	// Read config file if provided
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		logrus.Infof("Loaded configuration from: %s", v.ConfigFileUsed())
	}

	// Create config struct
	config := &Config{}

	// Unmarshal into config struct
	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// setDefaults sets the default values in viper
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 9000)

	// Collector defaults
	v.SetDefault("collector.base_url", "https://collector.ezlogs.com")
	v.SetDefault("collector.timeout", "30s")
	v.SetDefault("collector.max_idle_conns", 100)
	v.SetDefault("collector.max_idle_conns_per_host", 10)
	v.SetDefault("collector.idle_conn_timeout", "90s")
	v.SetDefault("collector.circuit_breaker.failure_threshold", 5)
	v.SetDefault("collector.circuit_breaker.timeout", "60s")
	v.SetDefault("collector.circuit_breaker.success_threshold", 3)

	// Buffer defaults
	v.SetDefault("buffer.max_size", 50000)
	v.SetDefault("buffer.flush_interval", "5s")
	v.SetDefault("buffer.batch_size", 1000)

	// Security defaults
	v.SetDefault("security.enable_tls", false)
	v.SetDefault("security.allowed_hosts", []string{"127.0.0.1", "localhost"})
	v.SetDefault("security.rate_limit", 10000)

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.output", "stdout")

	// Metrics defaults
	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.host", "0.0.0.0")
	v.SetDefault("metrics.port", 9090)
	v.SetDefault("metrics.path", "/metrics")

	// Health defaults
	v.SetDefault("health.enabled", true)
	v.SetDefault("health.host", "0.0.0.0")
	v.SetDefault("health.port", 8080)
	v.SetDefault("health.path", "/health")
}

// bindEnvVars binds all configuration fields to environment variables
func bindEnvVars(v *viper.Viper) {
	// Server
	v.BindEnv("server.host", "EZLOGS_SERVER_HOST")
	v.BindEnv("server.port", "EZLOGS_SERVER_PORT")

	// Collector
	v.BindEnv("collector.base_url", "EZLOGS_COLLECTOR_BASE_URL")
	v.BindEnv("collector.api_key", "EZLOGS_COLLECTOR_API_KEY")
	v.BindEnv("collector.timeout", "EZLOGS_COLLECTOR_TIMEOUT")
	v.BindEnv("collector.max_idle_conns", "EZLOGS_COLLECTOR_MAX_IDLE_CONNS")
	v.BindEnv("collector.max_idle_conns_per_host", "EZLOGS_COLLECTOR_MAX_IDLE_CONNS_PER_HOST")
	v.BindEnv("collector.idle_conn_timeout", "EZLOGS_COLLECTOR_IDLE_CONN_TIMEOUT")
	v.BindEnv("collector.circuit_breaker.failure_threshold", "EZLOGS_COLLECTOR_CB_FAILURE_THRESHOLD")
	v.BindEnv("collector.circuit_breaker.timeout", "EZLOGS_COLLECTOR_CB_TIMEOUT")
	v.BindEnv("collector.circuit_breaker.success_threshold", "EZLOGS_COLLECTOR_CB_SUCCESS_THRESHOLD")

	// Buffer
	v.BindEnv("buffer.max_size", "EZLOGS_BUFFER_MAX_SIZE")
	v.BindEnv("buffer.flush_interval", "EZLOGS_BUFFER_FLUSH_INTERVAL")
	v.BindEnv("buffer.batch_size", "EZLOGS_BUFFER_BATCH_SIZE")

	// Security
	v.BindEnv("security.enable_tls", "EZLOGS_SECURITY_ENABLE_TLS")
	v.BindEnv("security.tls_cert_file", "EZLOGS_SECURITY_TLS_CERT_FILE")
	v.BindEnv("security.tls_key_file", "EZLOGS_SECURITY_TLS_KEY_FILE")
	v.BindEnv("security.allowed_hosts", "EZLOGS_SECURITY_ALLOWED_HOSTS")
	v.BindEnv("security.rate_limit", "EZLOGS_SECURITY_RATE_LIMIT")

	// Logging
	v.BindEnv("logging.level", "EZLOGS_LOGGING_LEVEL")
	v.BindEnv("logging.format", "EZLOGS_LOGGING_FORMAT")
	v.BindEnv("logging.output", "EZLOGS_LOGGING_OUTPUT")

	// Metrics
	v.BindEnv("metrics.enabled", "EZLOGS_METRICS_ENABLED")
	v.BindEnv("metrics.host", "EZLOGS_METRICS_HOST")
	v.BindEnv("metrics.port", "EZLOGS_METRICS_PORT")
	v.BindEnv("metrics.path", "EZLOGS_METRICS_PATH")

	// Health
	v.BindEnv("health.enabled", "EZLOGS_HEALTH_ENABLED")
	v.BindEnv("health.host", "EZLOGS_HEALTH_HOST")
	v.BindEnv("health.port", "EZLOGS_HEALTH_PORT")
	v.BindEnv("health.path", "EZLOGS_HEALTH_PATH")
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate server config
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d (must be 1-65535)", c.Server.Port)
	}

	// Validate collector config
	if c.Collector.BaseURL == "" {
		return fmt.Errorf("collector base URL is required")
	}
	if c.Collector.Timeout <= 0 {
		return fmt.Errorf("collector timeout must be positive")
	}
	if c.Collector.MaxIdleConns < 0 {
		return fmt.Errorf("max idle connections cannot be negative")
	}
	if c.Collector.MaxIdleConnsPerHost < 0 {
		return fmt.Errorf("max idle connections per host cannot be negative")
	}
	if c.Collector.IdleConnTimeout <= 0 {
		return fmt.Errorf("idle connection timeout must be positive")
	}
	if c.Collector.CircuitBreaker.FailureThreshold < 0 {
		return fmt.Errorf("failure threshold cannot be negative")
	}
	if c.Collector.CircuitBreaker.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if c.Collector.CircuitBreaker.SuccessThreshold < 0 {
		return fmt.Errorf("success threshold cannot be negative")
	}

	// Validate buffer config
	if c.Buffer.MaxSize <= 0 {
		return fmt.Errorf("buffer max size must be positive")
	}
	if c.Buffer.FlushInterval <= 0 {
		return fmt.Errorf("flush interval must be positive")
	}
	if c.Buffer.BatchSize <= 0 {
		return fmt.Errorf("batch size must be positive")
	}
	if c.Buffer.BatchSize > c.Buffer.MaxSize {
		return fmt.Errorf("batch size cannot exceed buffer max size")
	}

	// Validate security config
	if c.Security.EnableTLS {
		if c.Security.TLSCertFile == "" {
			return fmt.Errorf("TLS cert file is required when TLS is enabled")
		}
		if c.Security.TLSKeyFile == "" {
			return fmt.Errorf("TLS key file is required when TLS is enabled")
		}
		if _, err := os.Stat(c.Security.TLSCertFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS cert file does not exist: %s", c.Security.TLSCertFile)
		}
		if _, err := os.Stat(c.Security.TLSKeyFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS key file does not exist: %s", c.Security.TLSKeyFile)
		}
	}
	if c.Security.RateLimit <= 0 {
		return fmt.Errorf("rate limit must be positive")
	}

	// Validate logging config
	if c.Logging.Level != "debug" && c.Logging.Level != "info" &&
		c.Logging.Level != "warn" && c.Logging.Level != "error" {
		return fmt.Errorf("invalid logging level: %s", c.Logging.Level)
	}
	if c.Logging.Format != "json" && c.Logging.Format != "text" {
		return fmt.Errorf("invalid logging format: %s", c.Logging.Format)
	}

	// Validate metrics config
	if c.Metrics.Enabled {
		if c.Metrics.Port < 1 || c.Metrics.Port > 65535 {
			return fmt.Errorf("invalid metrics port: %d (must be 1-65535)", c.Metrics.Port)
		}
	}

	// Validate health config
	if c.Health.Enabled {
		if c.Health.Port < 1 || c.Health.Port > 65535 {
			return fmt.Errorf("invalid health port: %d (must be 1-65535)", c.Health.Port)
		}
	}

	return nil
}

// GetServerAddr returns the server address as a string
func (c *Config) GetServerAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// GetMetricsAddr returns the metrics server address as a string
func (c *Config) GetMetricsAddr() string {
	return fmt.Sprintf("%s:%d", c.Metrics.Host, c.Metrics.Port)
}

// GetHealthAddr returns the health server address as a string
func (c *Config) GetHealthAddr() string {
	return fmt.Sprintf("%s:%d", c.Health.Host, c.Health.Port)
}

// IsAPIKeySet returns true if an API key is configured
func (c *Config) IsAPIKeySet() bool {
	return c.Collector.APIKey != ""
}

// GetLogLevel returns the logrus.Level for the configured logging level
func (c *Config) GetLogLevel() logrus.Level {
	switch c.Logging.Level {
	case "debug":
		return logrus.DebugLevel
	case "info":
		return logrus.InfoLevel
	case "warn":
		return logrus.WarnLevel
	case "error":
		return logrus.ErrorLevel
	default:
		return logrus.InfoLevel
	}
}

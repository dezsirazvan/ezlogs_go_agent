package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dezsirazvan/ezlogs_go_agent/internal/agent"
	"github.com/dezsirazvan/ezlogs_go_agent/internal/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	// Version information - set during build
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"

	// Command line flags
	configFile string
	verbose    bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ezlogs-agent",
	Short: "EZLogs Go Agent - High-performance event forwarding service",
	Long: `EZLogs Go Agent is a production-ready, high-performance event forwarding service
that acts as a local buffer between applications and the EZLogs collector service.

Features:
- TCP-based event ingestion with JSON support
- Configurable buffering and batching
- Automatic retry with exponential backoff
- Prometheus metrics and health checks
- TLS support for secure communications
- Rate limiting and access control
- Graceful shutdown and signal handling

Example usage:
  ezlogs-agent --config /etc/ezlogs/ezlogs-agent.yaml
  ezlogs-agent --server-host 127.0.0.1 --server-port 9000
  EZLOGS_COLLECTOR_API_KEY=your-key ezlogs-agent`,
	Version: fmt.Sprintf("%s (build: %s, commit: %s)", Version, BuildTime, GitCommit),
	RunE:    run,
}

// init sets up command line flags and configuration
func init() {
	// Add command line flags
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Configuration file path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

	// Add server flags
	rootCmd.Flags().String("server-host", "", "server host (overrides config file)")
	rootCmd.Flags().Int("server-port", 0, "server port (overrides config file)")

	// Add collector flags
	rootCmd.Flags().String("collector-endpoint", "", "collector endpoint (overrides config file)")
	rootCmd.Flags().String("collector-api-key", "", "collector API key (overrides config file)")
	rootCmd.Flags().Duration("collector-timeout", 0, "collector timeout (overrides config file)")
	rootCmd.Flags().Int("collector-max-retries", 0, "collector max retries (overrides config file)")
	rootCmd.Flags().Duration("collector-retry-backoff", 0, "collector retry backoff (overrides config file)")

	// Add buffer flags
	rootCmd.Flags().Int("buffer-max-size", 0, "buffer max size (overrides config file)")
	rootCmd.Flags().Duration("buffer-flush-interval", 0, "buffer flush interval (overrides config file)")
	rootCmd.Flags().Int("buffer-batch-size", 0, "buffer batch size (overrides config file)")

	// Add logging flags
	rootCmd.Flags().String("logging-level", "", "logging level (overrides config file)")
	rootCmd.Flags().String("logging-format", "", "logging format (overrides config file)")
	rootCmd.Flags().String("logging-output", "", "logging output (overrides config file)")

	// Add metrics flags
	rootCmd.Flags().Bool("metrics-enabled", false, "enable metrics (overrides config file)")
	rootCmd.Flags().String("metrics-host", "", "metrics host (overrides config file)")
	rootCmd.Flags().Int("metrics-port", 0, "metrics port (overrides config file)")
	rootCmd.Flags().String("metrics-path", "", "metrics path (overrides config file)")

	// Add health flags
	rootCmd.Flags().Bool("health-enabled", false, "enable health checks (overrides config file)")
	rootCmd.Flags().String("health-host", "", "health host (overrides config file)")
	rootCmd.Flags().Int("health-port", 0, "health port (overrides config file)")
	rootCmd.Flags().String("health-path", "", "health path (overrides config file)")
}

// run is the main execution function
func run(cmd *cobra.Command, args []string) error {
	// Set up logging
	if verbose {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
	logrus.SetFormatter(&logrus.JSONFormatter{})

	// Load configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Print configuration summary
	logrus.Info("Starting EZLogs Go Agent")
	logrus.WithFields(logrus.Fields{
		"server_addr":     cfg.GetServerAddr(),
		"collector":       cfg.Collector.BaseURL,
		"buffer_size":     cfg.Buffer.MaxSize,
		"batch_size":      cfg.Buffer.BatchSize,
		"flush_interval":  cfg.Buffer.FlushInterval,
		"metrics_enabled": cfg.Metrics.Enabled,
		"health_enabled":  cfg.Health.Enabled,
		"tls_enabled":     cfg.Security.EnableTLS,
	}).Info("Configuration loaded")

	// Create agent
	agent, err := agent.NewAgent(cfg)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start agent in goroutine
	go func() {
		if err := agent.Start(ctx); err != nil {
			logrus.WithError(err).Error("Agent failed")
			cancel()
		}
	}()

	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		logrus.WithField("signal", sig).Info("Received shutdown signal")
	case <-ctx.Done():
		logrus.Info("Context cancelled")
	}

	// Graceful shutdown
	logrus.Info("Shutting down agent...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Buffer.FlushInterval)
	defer shutdownCancel()

	if err := agent.Shutdown(shutdownCtx); err != nil {
		logrus.WithError(err).Error("Error during shutdown")
		return err
	}

	logrus.Info("Agent shutdown complete")
	return nil
}

// main is the entry point
func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

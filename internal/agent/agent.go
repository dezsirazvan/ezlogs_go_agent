package agent

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"bufio"
	"encoding/json"
	"errors"
	"io"

	"github.com/dezsirazvan/ezlogs_go_agent/internal/buffer"
	"github.com/dezsirazvan/ezlogs_go_agent/internal/collector"
	"github.com/dezsirazvan/ezlogs_go_agent/internal/config"
	healthpkg "github.com/dezsirazvan/ezlogs_go_agent/internal/health"
	"github.com/dezsirazvan/ezlogs_go_agent/internal/metrics"
	"github.com/dezsirazvan/ezlogs_go_agent/internal/security"

	"crypto/tls"

	"github.com/sirupsen/logrus"
)

// Agent defines the core interface for the EZLogs Go Agent.
type Agent interface {
	// Start launches the TCP server and begins processing events.
	Start(ctx context.Context) error
	// Shutdown gracefully stops the agent and flushes all buffers.
	Shutdown(ctx context.Context) error
}

// agentImpl is the concrete implementation of the Agent interface.
type agentImpl struct {
	cfg      *config.Config
	listener net.Listener
	httpSrv  *http.Server
	wg       sync.WaitGroup
	shutdown chan struct{}

	buf         *buffer.EventBuffer
	coll        *collector.Collector
	metrics     *metrics.Metrics
	health      *healthpkg.Health
	security    *security.Security
	eventCompat *EventCompatibility
}

// NewAgent creates a new Agent instance with the given configuration.
func NewAgent(cfg *config.Config) (Agent, error) {
	// Create buffer
	buf := buffer.NewEventBuffer(cfg.Buffer.MaxSize)

	// Create collector
	coll := collector.NewCollector(&cfg.Collector, cfg.Collector.APIKey)

	// Create metrics (optional)
	metrics, err := metrics.NewMetrics(&cfg.Metrics)
	if err != nil {
		return nil, err
	}

	// Create health checks (optional)
	health := healthpkg.NewHealth(&cfg.Health)

	// Create security (required)
	sec, err := security.NewSecurity(&cfg.Security)
	if err != nil {
		return nil, err
	}

	// Create event compatibility layer (generic for all SDKs)
	eventCompat := NewEventCompatibility(true, true) // Validate and transform by default

	agent := &agentImpl{
		cfg:         cfg,
		shutdown:    make(chan struct{}),
		buf:         buf,
		coll:        coll,
		metrics:     metrics,
		health:      health,
		security:    sec,
		eventCompat: eventCompat,
	}

	// Set up health checks
	agent.setupHealthChecks()

	return agent, nil
}

// setupHealthChecks configures health check functions
func (a *agentImpl) setupHealthChecks() {
	a.health.AddCheck("tcp_listener", healthpkg.TCPListenerCheck(a.listener))
	a.health.AddCheck("http_server", healthpkg.HTTPServerCheck(a.httpSrv))
	a.health.AddCheck("buffer", healthpkg.BufferCheck(a.buf))
	a.health.AddCheck("collector", healthpkg.CollectorCheck(a.coll))
}

// Start launches the TCP and HTTP servers and begins accepting connections.
func (a *agentImpl) Start(ctx context.Context) error {
	addr := a.cfg.GetServerAddr()

	// Start TCP server (for direct TCP connections)
	if err := a.startTCPServer(addr); err != nil {
		return err
	}

	// TODO: HTTP server conflicts with TCP server on same port
	// Temporarily disabled until we add separate HTTP port configuration
	// Start HTTP server (for SDK compatibility)
	// if err := a.startHTTPServer(addr); err != nil {
	// 	return err
	// }

	// Start metrics server
	if err := a.metrics.Start(ctx); err != nil {
		logrus.WithError(err).Error("Failed to start metrics server")
		return err
	}

	// Start health check server
	if err := a.health.Start(ctx); err != nil {
		logrus.WithError(err).Error("Failed to start health server")
		return err
	}

	// Rate limiter cleanup
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-a.shutdown:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.security.CleanupRateLimiters()
			}
		}
	}()

	// Flusher loop
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		ticker := time.NewTicker(a.cfg.Buffer.FlushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-a.shutdown:
				logrus.Info("Flusher shutting down")
				return
			case <-ctx.Done():
				logrus.Info("Flusher context cancelled")
				return
			case <-ticker.C:
				a.flushBuffer(ctx)
			}
		}
	}()

	// Wait for context cancellation or shutdown
	select {
	case <-ctx.Done():
		logrus.Info("Context cancelled, shutting down agent")
		return nil
	case <-a.shutdown:
		logrus.Info("Shutdown signal received, shutting down agent")
		return nil
	}
}

// startTCPServer starts the TCP server for direct connections
func (a *agentImpl) startTCPServer(addr string) error {
	// Create listener with TLS if enabled
	var ln net.Listener
	var err error

	if a.security.IsTLSEnabled() {
		ln, err = tls.Listen("tcp", addr, a.security.GetTLSConfig())
		logrus.WithField("addr", addr).Info("Starting TLS TCP listener")
	} else {
		ln, err = net.Listen("tcp", addr)
		logrus.WithField("addr", addr).Info("Starting TCP listener")
	}

	if err != nil {
		logrus.WithError(err).WithField("addr", addr).Error("Failed to start TCP listener")
		return err
	}
	a.listener = ln

	// Accept loop
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-a.shutdown:
					logrus.Info("TCP listener shutting down")
					return
				default:
					logrus.WithError(err).Warn("Accept error")
					continue
				}
			}

			// Security checks
			if !a.checkConnectionSecurity(conn) {
				conn.Close()
				continue
			}

			logrus.WithField("remote", conn.RemoteAddr().String()).Debug("Accepted TCP connection")
			a.metrics.ConnectionOpened()
			a.wg.Add(1)
			go a.handleTCPConn(conn)
		}
	}()

	return nil
}

// startHTTPServer starts the HTTP server for SDK compatibility
func (a *agentImpl) startHTTPServer(addr string) error {
	mux := http.NewServeMux()

	// Events endpoint for SDKs
	mux.HandleFunc("/events", a.handleHTTPEvents)

	// Health endpoint
	mux.HandleFunc("/health", a.handleHealth)

	// Ready endpoint
	mux.HandleFunc("/ready", a.handleReady)

	a.httpSrv = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start HTTP server
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		logrus.WithField("addr", addr).Info("Starting HTTP server for SDK compatibility")

		var err error
		if a.security.IsTLSEnabled() {
			a.httpSrv.TLSConfig = a.security.GetTLSConfig()
			err = a.httpSrv.ListenAndServeTLS("", "")
		} else {
			err = a.httpSrv.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Error("HTTP server error")
		}
	}()

	return nil
}

// handleHTTPEvents handles HTTP POST requests from SDKs
func (a *agentImpl) handleHTTPEvents(w http.ResponseWriter, r *http.Request) {
	// Security checks
	if !a.checkHTTPSecurity(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Only allow POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	data, err := io.ReadAll(r.Body)
	if err != nil {
		logrus.WithError(err).Warn("Failed to read HTTP request body")
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Validate payload
	if err := a.security.ValidatePayload(data); err != nil {
		logrus.WithError(err).Warn("Invalid HTTP payload")
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Process events (same as TCP)
	if err := a.processEvents(data, r.RemoteAddr); err != nil {
		logrus.WithError(err).Warn("Failed to process HTTP events")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Return success
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// handleHealth handles health check requests
func (a *agentImpl) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := a.health.GetStatus()
	if status.Status == healthpkg.StatusHealthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(status)
}

// handleReady handles readiness check requests
func (a *agentImpl) handleReady(w http.ResponseWriter, r *http.Request) {
	status := a.health.GetStatus()
	if status.Status == healthpkg.StatusHealthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(status)
}

// checkHTTPSecurity performs security checks on HTTP requests
func (a *agentImpl) checkHTTPSecurity(r *http.Request) bool {
	// Extract client IP
	clientIP := r.RemoteAddr
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		clientIP = forwardedFor
	}

	// Parse IP
	host, _, err := net.SplitHostPort(clientIP)
	if err != nil {
		host = clientIP
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	// Check IP allow-listing
	if !a.security.IsIPAllowed(ip) {
		logrus.WithField("remote", clientIP).Warn("HTTP request rejected: IP not allowed")
		return false
	}

	// Check rate limiting
	if !a.security.CheckRateLimit(ip.String()) {
		logrus.WithField("remote", clientIP).Warn("HTTP request rejected: rate limit exceeded")
		return false
	}

	return true
}

// processEvents processes events from either TCP or HTTP
func (a *agentImpl) processEvents(data []byte, remoteAddr string) error {
	// Validate protocol
	if err := a.eventCompat.ValidateProtocol(data); err != nil {
		logrus.WithError(err).WithField("remote", remoteAddr).Warn("Invalid event protocol")
		return err
	}

	// Parse as UniversalEvents
	events, err := a.eventCompat.ParseBatch(data)
	if err != nil {
		logrus.WithError(err).WithField("remote", remoteAddr).Warn("Failed to parse UniversalEvents")
		return err
	}

	// Transform to collector format
	collectorEvents := a.eventCompat.TransformToCollectorFormat(events)

	logrus.WithFields(logrus.Fields{
		"remote": remoteAddr,
		"events": len(collectorEvents),
	}).Info("Parsed and transformed UniversalEvent batch")

	// Update metrics
	a.metrics.EventsReceived(len(collectorEvents))

	// Add to buffer
	a.buf.Add(collectorEvents)
	a.metrics.SetBufferSize(a.buf.Size())

	return nil
}

// checkConnectionSecurity performs security checks on incoming connections
func (a *agentImpl) checkConnectionSecurity(conn net.Conn) bool {
	remoteAddr := conn.RemoteAddr()
	if tcpAddr, ok := remoteAddr.(*net.TCPAddr); ok {
		// Check IP allow-listing
		if !a.security.IsIPAllowed(tcpAddr.IP) {
			logrus.WithField("remote", remoteAddr.String()).Warn("Connection rejected: IP not allowed")
			return false
		}

		// Check rate limiting
		if !a.security.CheckRateLimit(tcpAddr.IP.String()) {
			logrus.WithField("remote", remoteAddr.String()).Warn("Connection rejected: rate limit exceeded")
			return false
		}
	}

	return true
}

// handleTCPConn reads JSON event batches from a TCP connection
func (a *agentImpl) handleTCPConn(conn net.Conn) {
	defer a.wg.Done()
	defer conn.Close()
	defer a.metrics.ConnectionClosed()

	reader := bufio.NewReader(conn)
	data, err := io.ReadAll(reader)
	if err != nil {
		logrus.WithError(err).WithField("remote", conn.RemoteAddr().String()).Warn("Read error")
		return
	}

	// Process events
	if err := a.processEvents(data, conn.RemoteAddr().String()); err != nil {
		logrus.WithError(err).WithField("remote", conn.RemoteAddr().String()).Warn("Failed to process TCP events")
		return
	}
}

// flushBuffer drains the buffer and sends events to collector
func (a *agentImpl) flushBuffer(ctx context.Context) {
	batch := a.buf.Drain(a.cfg.Buffer.BatchSize)
	if len(batch) == 0 {
		return
	}

	logrus.WithField("events", len(batch)).Info("Flushing batch to collector")

	start := time.Now()
	err := a.coll.SendBatch(ctx, batch)
	duration := time.Since(start)

	// Update metrics
	if err != nil {
		logrus.WithError(err).Error("Failed to send batch to collector")
		a.metrics.HTTPRequest("error", duration)
		a.metrics.EventsDropped(len(batch))
	} else {
		a.metrics.HTTPRequest("success", duration)
		a.metrics.EventsSent(len(batch))
	}
}

// Shutdown gracefully stops the agent and flushes all buffers.
func (a *agentImpl) Shutdown(ctx context.Context) error {
	if a.listener == nil {
		return errors.New("agent not started")
	}

	close(a.shutdown)

	// Close the TCP listener to unblock Accept
	if err := a.listener.Close(); err != nil {
		logrus.WithError(err).Warn("Error closing TCP listener")
	}

	// Shutdown HTTP server
	if a.httpSrv != nil {
		if err := a.httpSrv.Shutdown(ctx); err != nil {
			logrus.WithError(err).Warn("Error shutting down HTTP server")
		}
	}

	// Final flush
	a.flushBuffer(ctx)

	// Shutdown metrics and health servers
	if a.metrics != nil {
		a.metrics.Shutdown(ctx)
	}
	if a.health != nil {
		a.health.Shutdown(ctx)
	}

	// Wait for all goroutines to finish or context timeout
	c := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(c)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c:
		return nil
	}
}

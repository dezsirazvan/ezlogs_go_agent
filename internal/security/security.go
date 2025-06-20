package security

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/dezsirazvan/ezlogs_go_agent/internal/config"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

// Security provides security features for the EZLogs agent
type Security struct {
	cfg *config.SecurityConfig

	// TLS configuration
	tlsConfig *tls.Config

	// IP allow-listing
	allowedNetworks []*net.IPNet
	allowedIPs      map[string]bool

	// Rate limiting
	rateLimiters map[string]*rate.Limiter
	rateMu       sync.RWMutex

	// Input validation
	maxPayloadSize int64
}

// NewSecurity creates a new security instance
func NewSecurity(cfg *config.SecurityConfig) (*Security, error) {
	s := &Security{
		cfg:            cfg,
		allowedIPs:     make(map[string]bool),
		rateLimiters:   make(map[string]*rate.Limiter),
		maxPayloadSize: 10 * 1024 * 1024, // 10MB default
	}

	// Set up TLS if enabled
	if cfg.EnableTLS {
		if err := s.setupTLS(); err != nil {
			return nil, fmt.Errorf("failed to setup TLS: %w", err)
		}
	}

	// Set up IP allow-listing
	if err := s.setupIPAllowlist(); err != nil {
		return nil, fmt.Errorf("failed to setup IP allowlist: %w", err)
	}

	return s, nil
}

// setupTLS configures TLS for secure connections
func (s *Security) setupTLS() error {
	cert, err := tls.LoadX509KeyPair(s.cfg.TLSCertFile, s.cfg.TLSKeyFile)
	if err != nil {
		return fmt.Errorf("failed to load TLS certificate: %w", err)
	}

	s.tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},
	}

	logrus.Info("TLS enabled with certificate")
	return nil
}

// setupIPAllowlist configures IP allow-listing
func (s *Security) setupIPAllowlist() error {
	for _, host := range s.cfg.AllowedHosts {
		if strings.Contains(host, "/") {
			// CIDR notation
			_, network, err := net.ParseCIDR(host)
			if err != nil {
				return fmt.Errorf("invalid CIDR %s: %w", host, err)
			}
			s.allowedNetworks = append(s.allowedNetworks, network)
		} else {
			// Single IP or hostname
			ip := net.ParseIP(host)
			if ip != nil {
				s.allowedIPs[ip.String()] = true
			} else {
				// Try to resolve hostname
				ips, err := net.LookupIP(host)
				if err != nil {
					logrus.WithError(err).Warnf("Failed to resolve hostname %s", host)
					continue
				}
				for _, resolvedIP := range ips {
					s.allowedIPs[resolvedIP.String()] = true
				}
			}
		}
	}

	logrus.WithField("allowed_hosts", len(s.cfg.AllowedHosts)).Info("IP allowlist configured")
	return nil
}

// IsIPAllowed checks if an IP address is allowed
func (s *Security) IsIPAllowed(ip net.IP) bool {
	ipStr := ip.String()

	// Check exact IP matches
	if s.allowedIPs[ipStr] {
		return true
	}

	// Check network ranges
	for _, network := range s.allowedNetworks {
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// CheckRateLimit checks if a client is within rate limits
func (s *Security) CheckRateLimit(clientIP string) bool {
	s.rateMu.RLock()
	limiter, exists := s.rateLimiters[clientIP]
	s.rateMu.RUnlock()

	if !exists {
		// Create new rate limiter for this client
		s.rateMu.Lock()
		limiter = rate.NewLimiter(rate.Limit(s.cfg.RateLimit), s.cfg.RateLimit)
		s.rateLimiters[clientIP] = limiter
		s.rateMu.Unlock()
	}

	return limiter.Allow()
}

// CleanupRateLimiters removes old rate limiters to prevent memory leaks
func (s *Security) CleanupRateLimiters() {
	s.rateMu.Lock()
	defer s.rateMu.Unlock()

	// Remove rate limiters older than 1 hour
	// This is a simple cleanup - in production you might want more sophisticated cleanup
	if len(s.rateLimiters) > 1000 {
		// If we have too many, clear all (simple approach)
		s.rateLimiters = make(map[string]*rate.Limiter)
		logrus.Info("Cleaned up rate limiters")
	}
}

// ValidatePayload validates incoming payload
func (s *Security) ValidatePayload(data []byte) error {
	// Check payload size
	if int64(len(data)) > s.maxPayloadSize {
		return fmt.Errorf("payload too large: %d bytes (max: %d)", len(data), s.maxPayloadSize)
	}

	// Check for null bytes (potential security issue)
	if strings.Contains(string(data), "\x00") {
		return fmt.Errorf("payload contains null bytes")
	}

	return nil
}

// GetTLSConfig returns the TLS configuration if enabled
func (s *Security) GetTLSConfig() *tls.Config {
	return s.tlsConfig
}

// IsTLSEnabled returns true if TLS is enabled
func (s *Security) IsTLSEnabled() bool {
	return s.cfg.EnableTLS
}

package security

import (
	"net"
	"testing"
	"time"

	"github.com/dezsirazvan/ezlogs_go_agent/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurity_IPAllowlist(t *testing.T) {
	cfg := &config.SecurityConfig{
		EnableTLS:    false,
		AllowedHosts: []string{"127.0.0.1", "10.0.0.0/8", "192.168.1.100"},
		RateLimit:    1000,
	}

	sec, err := NewSecurity(cfg)
	require.NoError(t, err)

	// Test allowed IPs
	assert.True(t, sec.IsIPAllowed(net.ParseIP("127.0.0.1")))
	assert.True(t, sec.IsIPAllowed(net.ParseIP("10.0.0.1")))
	assert.True(t, sec.IsIPAllowed(net.ParseIP("10.255.255.255")))
	assert.True(t, sec.IsIPAllowed(net.ParseIP("192.168.1.100")))

	// Test denied IPs
	assert.False(t, sec.IsIPAllowed(net.ParseIP("192.168.1.101")))
	assert.False(t, sec.IsIPAllowed(net.ParseIP("8.8.8.8")))
}

func TestSecurity_RateLimiting(t *testing.T) {
	cfg := &config.SecurityConfig{
		EnableTLS:    false,
		AllowedHosts: []string{"127.0.0.1"},
		RateLimit:    10, // 10 per second
	}

	sec, err := NewSecurity(cfg)
	require.NoError(t, err)

	clientIP := "127.0.0.1"

	// Should allow first 10 requests
	for i := 0; i < 10; i++ {
		assert.True(t, sec.CheckRateLimit(clientIP), "Request %d should be allowed", i+1)
	}

	// 11th request should be rate limited
	assert.False(t, sec.CheckRateLimit(clientIP), "Request 11 should be rate limited")

	// Wait a bit and try again
	time.Sleep(1100 * time.Millisecond)
	assert.True(t, sec.CheckRateLimit(clientIP), "Request after wait should be allowed")
}

func TestSecurity_PayloadValidation(t *testing.T) {
	cfg := &config.SecurityConfig{
		EnableTLS:    false,
		AllowedHosts: []string{"127.0.0.1"},
		RateLimit:    1000,
	}

	sec, err := NewSecurity(cfg)
	require.NoError(t, err)

	// Valid payload
	validPayload := []byte(`[{"event": "test"}]`)
	assert.NoError(t, sec.ValidatePayload(validPayload))

	// Payload with null bytes (security issue)
	invalidPayload := []byte(`[{"event": "test` + string([]byte{0}) + `"}]`)
	assert.Error(t, sec.ValidatePayload(invalidPayload))

	// Very large payload (over 10MB)
	largePayload := make([]byte, 11*1024*1024) // 11MB
	assert.Error(t, sec.ValidatePayload(largePayload))
}

func TestSecurity_TLSConfig(t *testing.T) {
	// Test without TLS
	cfg := &config.SecurityConfig{
		EnableTLS:    false,
		AllowedHosts: []string{"127.0.0.1"},
		RateLimit:    1000,
	}

	sec, err := NewSecurity(cfg)
	require.NoError(t, err)

	assert.False(t, sec.IsTLSEnabled())
	assert.Nil(t, sec.GetTLSConfig())

	// Test with TLS (will fail without cert files, but should handle gracefully)
	cfg.EnableTLS = true
	cfg.TLSCertFile = "/nonexistent/cert.pem"
	cfg.TLSKeyFile = "/nonexistent/key.pem"

	_, err = NewSecurity(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load TLS certificate")
}

func TestSecurity_CleanupRateLimiters(t *testing.T) {
	cfg := &config.SecurityConfig{
		EnableTLS:    false,
		AllowedHosts: []string{"127.0.0.1"},
		RateLimit:    1000,
	}

	sec, err := NewSecurity(cfg)
	require.NoError(t, err)

	// Add some rate limiters
	sec.CheckRateLimit("192.168.1.1")
	sec.CheckRateLimit("192.168.1.2")
	sec.CheckRateLimit("192.168.1.3")

	// Cleanup should work without error
	sec.CleanupRateLimiters()
}

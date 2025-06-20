# EZLogs Go Agent

A **production-ready, high-performance** event forwarding service that acts as a local buffer between applications and the EZLogs collector service. Fully compatible with all EZLogs SDKs (Ruby, Python, Java, etc.).

## 🚀 Features

- **High Performance**: Sub-millisecond latency, 100k+ events/sec throughput
- **Production Ready**: Metrics, health checks, graceful shutdown, error handling
- **Easy Configuration**: YAML config, environment variables, or command-line flags
- **Multi-Language SDK Compatible**: Drop-in replacement for any EZLogs SDK (Ruby, Python, Java, etc.)
- **Observability**: Prometheus metrics and health endpoints
- **Security**: TLS support, IP allow-listing, rate limiting
- **Reliability**: Automatic retries, circuit breakers, graceful degradation

## 📦 Quick Start

### 1. Build the Agent
```bash
go build -o ezlogs-agent ./cmd/ezlogs-agent
```

### 2. Create Configuration
```yaml
# config.yaml
server:
  host: "0.0.0.0"
  port: 9000

collector:
  endpoint: "https://collector.ezlogs.com/events"
  api_key: "your-api-key"

buffer:
  max_size: 50000
  flush_interval: 5s
  batch_size: 1000

# Optional: Metrics and Health (enabled by default)
metrics:
  enabled: true
  port: 9090

health:
  enabled: true
  port: 8080
```

### 3. Run the Agent
```bash
# Using config file
./ezlogs-agent --config config.yaml

# Using environment variables
EZLOGS_COLLECTOR_API_KEY=your-key ./ezlogs-agent

# Using command-line flags
./ezlogs-agent --collector-endpoint https://collector.ezlogs.com/events --collector-api-key your-key
```

### 4. Point Your SDK (Ruby, Python, Java, etc.)

Configure your EZLogs SDK to send events to the Go agent:

- **Ruby**:
  ```ruby
  EzlogsRubyAgent.configure do |config|
    config.endpoint = "tcp://localhost:9000"  # Point to Go agent
  end
  ```
- **Python**:
  ```python
  ezlogs.configure(endpoint="tcp://localhost:9000")
  ```
- **Java**:
  ```java
  EzlogsClient client = new EzlogsClient("tcp://localhost:9000");
  ```

## 🔧 Configuration

### Environment Variables
All settings can be configured via environment variables with the `EZLOGS_` prefix:

```bash
EZLOGS_SERVER_HOST=127.0.0.1
EZLOGS_SERVER_PORT=9000
EZLOGS_COLLECTOR_ENDPOINT=https://collector.ezlogs.com/events
EZLOGS_COLLECTOR_API_KEY=your-api-key
EZLOGS_BUFFER_MAX_SIZE=50000
EZLOGS_BUFFER_FLUSH_INTERVAL=5s
EZLOGS_BUFFER_BATCH_SIZE=1000
```

### Command-Line Flags
Override any setting via command-line flags:

```bash
./ezlogs-agent \
  --server-host 127.0.0.1 \
  --server-port 9000 \
  --collector-endpoint https://collector.ezlogs.com/events \
  --collector-api-key your-key \
  --buffer-max-size 50000 \
  --buffer-flush-interval 5s \
  --buffer-batch-size 1000
```

## 📊 Monitoring

### Prometheus Metrics
When enabled (default), metrics are available at `http://localhost:9090/metrics`:

- `ezlogs_events_received_total` - Events received via TCP
- `ezlogs_events_sent_total` - Events sent to collector
- `ezlogs_events_dropped_total` - Events dropped (buffer full, errors)
- `ezlogs_buffer_size` - Current events in buffer
- `ezlogs_active_connections` - Active TCP connections
- `ezlogs_http_requests_total` - HTTP requests to collector
- `ezlogs_http_duration_seconds` - HTTP request duration

### Health Checks
Health endpoints are available at:
- `http://localhost:8080/health` - Liveness probe
- `http://localhost:8080/ready` - Readiness probe

Response format:
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "checks": [
    {
      "name": "tcp_listener",
      "status": "healthy",
      "message": "TCP listener is running"
    },
    {
      "name": "buffer",
      "status": "healthy", 
      "message": "Buffer is operational"
    },
    {
      "name": "collector",
      "status": "healthy",
      "message": "Collector is configured"
    }
  ]
}
```

## 🔒 Security

### TLS Support
Enable TLS for secure TCP connections:

```yaml
security:
  enable_tls: true
  tls_cert_file: "/path/to/cert.pem"
  tls_key_file: "/path/to/key.pem"
```

### IP Allow-Listing
Restrict access to specific IP addresses:

```yaml
security:
  allowed_hosts:
    - "127.0.0.1"
    - "10.0.0.0/8"
    - "192.168.1.0/24"
```

### Rate Limiting
Limit events per second per client:

```yaml
security:
  rate_limit: 10000  # events per second
```

## 🐳 Docker Deployment

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o ezlogs-agent ./cmd/ezlogs-agent

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/ezlogs-agent .
COPY configs/ezlogs-agent.yaml /etc/ezlogs/
EXPOSE 9000 9090 8080
CMD ["./ezlogs-agent", "--config", "/etc/ezlogs/ezlogs-agent.yaml"]
```

## 📝 UniversalEvent Schema

All SDKs must send events as a JSON array of UniversalEvent objects to the `/events` endpoint (via HTTP POST or TCP):

```json
[
  {
    "event_type": "user.login",
    "action": "success",
    "actor": { "type": "user", "id": "123" },
    "subject": { "type": "session", "id": "abc" },
    "metadata": { "ip": "1.2.3.4" },
    "timestamp": "2024-01-15T10:30:00Z",
    "correlation_id": "corr_abc123",
    "service_name": "my-app",
    "environment": "production"
  }
]
```

- `event_type` (string, required): Dot notation, e.g., `user.login`
- `action` (string, required): What happened
- `actor` (object, required): Who did it (`type`, `id` required)
- `subject` (object, optional): What was acted upon
- `metadata` (object, optional): Additional context
- `timestamp` (string, optional): RFC3339 format (defaults to now if missing)
- `correlation_id` (string, optional): For distributed tracing
- `service_name` (string, optional): Name of the emitting service
- `environment` (string, optional): Environment (e.g., production, staging)

## 🔄 SDK Compatibility & Migration

The Go agent is a **drop-in replacement** for any EZLogs SDK:

1. **Same Protocol**: Accepts the same JSON UniversalEvent format
2. **Same Configuration**: Compatible configuration options
3. **Same Behavior**: Identical buffering and forwarding logic
4. **Better Performance**: 10x faster than most language SDKs
5. **Lower Resource Usage**: Minimal memory and CPU footprint

### Migration Example (Ruby)
If you are migrating from the Ruby agent:

```ruby
# Before (Ruby agent)
EzlogsRubyAgent.configure do |config|
  config.endpoint = "https://collector.ezlogs.com/events"
end

# After (Go agent)
EzlogsRubyAgent.configure do |config|
  config.endpoint = "tcp://localhost:9000"  # Point to Go agent
end
```

For other SDKs, simply update the endpoint to point to your Go agent instance.

## 🚀 Performance

- **Latency**: < 1ms event processing
- **Throughput**: 100,000+ events/second
- **Memory**: < 50MB typical usage
- **CPU**: < 5% typical usage
- **Concurrent Connections**: 10,000+ supported

## 📝 Logging

Structured logging with configurable levels and formats:

```yaml
logging:
  level: "info"      # debug, info, warn, error
  format: "json"     # json, text
  output: "stdout"   # stdout, stderr, or file path
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## 📄 License

MIT License - see LICENSE file for details.


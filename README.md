# EZLogs Go Agent

A **production-grade, high-performance** event forwarding service that acts as a local buffer between your applications and the EZLogs collector. Fully compatible with all EZLogs SDKs (Ruby, Python, Java, etc.).

---

## 🏁 EZ Start (First Event in 2 Minutes)

1. **Build the Agent**
   ```bash
   go build -o ezlogs-agent ./cmd/ezlogs-agent
   ```
2. **Create a Minimal Config**
   ```yaml
   # config.yaml
   server:
     host: "0.0.0.0"
     port: 9000
   collector:
     endpoint: "https://collector.ezlogs.com/events"
     api_key: "your-api-key"
   ```
3. **Run the Agent**
   ```bash
   ./ezlogs-agent --config config.yaml
   ```
4. **Point Your SDK to the Agent**
   - **Ruby**:
     ```ruby
     EzlogsRubyAgent.configure do |c|
       c.endpoint = "tcp://localhost:9000"
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
5. **Send a Test Event** (see schema below)

---

## 🌍 Project Overview

- **What:** Local, secure, high-throughput event buffer and forwarder for EZLogs
- **Why:** Offload your app, guarantee delivery, enable sub-ms logging, and add observability
- **Who:** Any team using EZLogs in production (works with Ruby, Python, Java, ...)
- **How:** Drop-in, zero-config, runs as a sidecar or service

---

## ⚙️ Configuration

- **YAML file:** `--config config.yaml`
- **Environment:** All options as `EZLOGS_*` variables
- **CLI flags:** All options as `--flag value`
- **Docker:** See below

**Example config:**
```yaml
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
metrics:
  enabled: true
  port: 9090
health:
  enabled: true
  port: 8080
```

**Environment variables:**
```bash
EZLOGS_SERVER_HOST=127.0.0.1
EZLOGS_SERVER_PORT=9000
EZLOGS_COLLECTOR_ENDPOINT=https://collector.ezlogs.com/events
EZLOGS_COLLECTOR_API_KEY=your-api-key
EZLOGS_BUFFER_MAX_SIZE=50000
EZLOGS_BUFFER_FLUSH_INTERVAL=5s
EZLOGS_BUFFER_BATCH_SIZE=1000
```

**CLI flags:**
```bash
./ezlogs-agent --server-host 127.0.0.1 --server-port 9000 --collector-endpoint https://collector.ezlogs.com/events --collector-api-key your-key
```

---

## 🤝 Multi-Language SDK Integration

Point any EZLogs SDK to the Go agent:
- **Ruby:** `tcp://localhost:9000`
- **Python:** `tcp://localhost:9000`
- **Java:** `tcp://localhost:9000`
- **Other:** Any language that can send TCP or HTTP POST to `/events` with UniversalEvent JSON

---

## 📝 UniversalEvent Schema

All events must follow this schema:
```json
{
  "event_type": "namespace.category",  // Required
  "action": "specific_action",         // Required
  "actor": { "type": "user", "id": "123" }, // Required
  "subject": { "type": "resource", "id": "456" }, // Optional
  "metadata": { "key": "value" },    // Optional
  "timestamp": "2024-01-15T10:30:00Z", // Optional (defaults to now)
  "correlation_id": "corr_abc123"      // Optional (auto-generated)
}
```
- Send as a JSON array to `/events` endpoint for batch delivery.
- See [docs/universal_event.md](docs/universal_event.md) for full details.

---

## 📊 Monitoring & Observability

- **Prometheus metrics:** `http://localhost:9090/metrics`
  - `ezlogs_events_received_total`, `ezlogs_events_sent_total`, `ezlogs_events_dropped_total`, `ezlogs_buffer_size`, `ezlogs_active_connections`, `ezlogs_http_requests_total`, `ezlogs_http_duration_seconds`
- **Health endpoints:**
  - Liveness: `http://localhost:8080/health`
  - Readiness: `http://localhost:8080/ready`
- **Example health response:**
  ```json
  {
    "status": "healthy",
    "timestamp": "2024-01-15T10:30:00Z",
    "checks": [
      { "name": "tcp_listener", "status": "healthy", "message": "TCP listener is running" },
      { "name": "buffer", "status": "healthy", "message": "Buffer is operational" },
      { "name": "collector", "status": "healthy", "message": "Collector is configured" }
    ]
  }
  ```

---

## 🔒 Security

- **TLS:**
  ```yaml
  security:
    enable_tls: true
    tls_cert_file: "/path/to/cert.pem"
    tls_key_file: "/path/to/key.pem"
  ```
- **IP allow-list:**
  ```yaml
  security:
    allowed_hosts:
      - "127.0.0.1"
      - "10.0.0.0/8"
      - "192.168.1.0/24"
  ```
- **Rate limiting:**
  ```yaml
  security:
    rate_limit: 10000  # events per second
  ```
- **Best practices:** Never log API keys or PII, validate all input, use secure defaults.

---

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

---

## 🛠️ Troubleshooting & FAQ

- **Agent won't start:** Check config file path, permissions, and required fields.
- **Events not delivered:** Check buffer/collector config, network, and logs for errors.
- **Metrics/health endpoints not available:** Ensure ports are open and not in use.
- **High latency:** Tune buffer size, batch size, and flush interval.
- **How do I debug?** Run with increased log level, check `/metrics` and `/health` endpoints.
- **How do I upgrade?** Stop agent, replace binary, restart. Config is backward compatible.

---

## ⚡ Advanced: Performance, Circuit Breakers, Graceful Shutdown

- **Performance:** Designed for 100k+ events/sec, sub-ms latency. Tune buffer and batch settings for your workload.
- **Circuit breaker:** Automatically disables collector on repeated failures, retries with backoff.
- **Graceful shutdown:** Handles SIGTERM, flushes buffer, closes connections cleanly.
- **Resource monitoring:** Tracks CPU, memory, goroutines (see `/metrics`).

---

## 🤝 Contributing & License

- PRs welcome! Please add tests and docs for new features.
- Licensed under MIT. See [LICENSE.txt](LICENSE.txt).

---

**EZLogs Go Agent** – Secure, fast, production-ready event delivery for every language.


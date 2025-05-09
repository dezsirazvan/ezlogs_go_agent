# Ezlogs Go Agent

A lightweight standalone agent that buffers JSON events over TCP and
forwards them in batches to your remote collector via HTTPS.

---

## Features

- **TCP Listener**  
  Listens on a local port (default `0.0.0.0:9000`) for JSON array payloads.

- **In-Memory Buffer**  
  Buffers up to `queue` events in a channel with drop-oldest policy.

- **Periodic Flush**  
  Every `interval` (default 5s), flushes up to `batch` events in a single HTTP POST.

- **Exponential Backoff**  
  Retries failed HTTP requests up to 3 times with 1s, 2s, 4s delays.

- **Graceful Shutdown**  
  On SIGINT/SIGTERM, closes listener, performs a final flush, then exits.

- **Config File + Flags**  
  Load settings from a YAML file (`--config /path/to/config.yaml`) or override via CLI flags.

- **API Key Authentication**  
  Sends `Authorization: Bearer <api_key>` header if provided.

---

## Installation

1. **Build** (requires Go 1.18+):
   ```bash
   go build -o ezlogs_agent ./cmd/ezlogs_agent

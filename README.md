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

## Configuration
Create a YAML file (e.g. ```/etc/ezlogs/ezlogs_agent.yaml```):
```ruby
# Ezlogs Go Agent configuration

# TCP listener address
listen:   "0.0.0.0:9000"

# Remote collector endpoint (must include scheme)
endpoint: "https://collector.myserver.com/events"

# API key for Authorization header
api_key:  "YOUR_API_KEY_HERE"

# Flush interval (Go duration)
interval: 5s

# Maximum events per HTTP batch
batch:    1000

# In-memory buffer size (max queued events)
queue:    50000
```

## Running
**Using the config file**  
```./ezlogs_agent --config /etc/ezlogs/ezlogs_agent.yaml```

**Overriding via flags**  
```ruby
./ezlogs_agent \
  --listen 127.0.0.1:9000 \
  --endpoint https://collector.myserver.com/events \
  --api-key YOUR_API_KEY \
  --interval 5s \
  --batch 1000 \
  --queue 50000
```

## Usage
- **Start the Go agent (as above).** 
- **Point your Ruby gem at ```127.0.0.1:9000``` (default).** 
- **Rails app calls ```EzlogsRubyAgent.writer.log(event_hash)```** 
- **Go agent batches and ships events to your collector API.** 


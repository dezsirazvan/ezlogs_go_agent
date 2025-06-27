package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v2"
)

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Config holds all agent settings, can be loaded from a YAML file or flags
type Config struct {
	Listen   string        `yaml:"listen"`
	Endpoint string        `yaml:"endpoint"`
	APIKey   string        `yaml:"api_key"`
	Interval time.Duration `yaml:"interval"`
	Batch    int           `yaml:"batch"`
	Queue    int           `yaml:"queue"`
}

var (
	// Optional path to a YAML config
	configPath = flag.String("config", "", "path to YAML config file")

	// Fallback flags if no config file (or to override)
	listenAddr    = flag.String("listen", "0.0.0.0:9000", "host:port to listen for incoming events")
	endpoint      = flag.String("endpoint", "https://collector.myserver.com/events", "remote HTTP endpoint")
	apiKey        = flag.String("api-key", "", "API key for authenticating to collector")
	flushInterval = flag.Duration("interval", 5*time.Second, "how often to flush batches")
	maxBatch      = flag.Int("batch", 1000, "maximum events per HTTP batch")
	queueSize     = flag.Int("queue", 50000, "max events to buffer in memory")
)

func loadConfig(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func applyConfig(cfg *Config) {
	if cfg.Listen != "" {
		*listenAddr = cfg.Listen
	}
	if cfg.Endpoint != "" {
		*endpoint = cfg.Endpoint
	}
	if cfg.APIKey != "" {
		*apiKey = cfg.APIKey
	}
	if cfg.Interval != 0 {
		*flushInterval = cfg.Interval
	}
	if cfg.Batch != 0 {
		*maxBatch = cfg.Batch
	}
	if cfg.Queue != 0 {
		*queueSize = cfg.Queue
	}
}

func main() {
	flag.Parse()

	// 1) Load YAML config if provided
	if *configPath != "" {
		cfg, err := loadConfig(*configPath)
		if err != nil {
			log.Fatalf("failed to load config %q: %v", *configPath, err)
		}
		applyConfig(cfg)
	}

	log.Printf("starting ezlogs_agent on %s → %s\n", *listenAddr, *endpoint)

	// buffered channel for incoming events
	events := make(chan json.RawMessage, *queueSize)
	var wg sync.WaitGroup

	// 2) Start TCP listener
	ln, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		log.Fatalf("listen failed: %v", err)
	}

	// 3) Handle shutdown: close listener on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("shutdown signal received, closing listener")
		ln.Close()
	}()

	// 4) Accept loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				// exit if listener closed
				if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
					return
				}
				log.Printf("accept error: %v", err)
				continue
			}
			go handleConn(conn, events)
		}
	}()

	// 5) Periodic flusher
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(*flushInterval)
		defer ticker.Stop()
		for range ticker.C {
			drainAndSend(events)
		}
	}()

	// 6) Wait for accept & flusher to finish, then final flush
	wg.Wait()
	log.Println("flushing remaining events…")
	drainAndSend(events)
	log.Println("bye")
}

// handleConn supports both raw TCP JSON and HTTP requests
func handleConn(conn net.Conn, events chan json.RawMessage) {
	defer conn.Close()
	data, err := io.ReadAll(bufio.NewReader(conn))
	if err != nil {
		log.Printf("read error: %v", err)
		return
	}

	// DEBUG: Print the raw data received
	log.Printf("RAW DATA from %s (length=%d): %q", conn.RemoteAddr(), len(data), string(data))

	var jsonData []byte
	var isHTTP bool

	// Check if this looks like an HTTP request
	if len(data) > 0 && (data[0] == 'P' || data[0] == 'G' || data[0] == 'H') {
		dataStr := string(data)
		if strings.HasPrefix(dataStr, "POST ") || strings.HasPrefix(dataStr, "GET ") || strings.HasPrefix(dataStr, "HEAD ") {
			isHTTP = true
			log.Printf("HTTP request detected from %s", conn.RemoteAddr())

			// Parse HTTP request
			jsonData, err = parseHTTPRequest(data, conn)
			if err != nil {
				log.Printf("HTTP parsing error from %s: %v", conn.RemoteAddr(), err)
				return
			}
		}
	}

	// If not HTTP, treat as raw TCP JSON
	if !isHTTP {
		log.Printf("Raw TCP JSON from %s", conn.RemoteAddr())
		jsonData = data
	}

	// Parse and process JSON
	if len(jsonData) > 0 {
		processJSONData(jsonData, conn.RemoteAddr(), events)
	}
}

// parseHTTPRequest extracts JSON from HTTP request body
func parseHTTPRequest(data []byte, conn net.Conn) ([]byte, error) {
	lines := strings.Split(string(data), "\r\n")
	if len(lines) < 1 {
		sendHTTPError(conn, 400, "Invalid HTTP request")
		return nil, fmt.Errorf("invalid HTTP request")
	}

	// Parse request line
	requestLine := strings.Split(lines[0], " ")
	if len(requestLine) < 3 {
		sendHTTPError(conn, 400, "Invalid HTTP request line")
		return nil, fmt.Errorf("invalid HTTP request line")
	}

	method := requestLine[0]

	// Only support POST for event submission
	if method != "POST" {
		sendHTTPError(conn, 405, "Method not allowed. Only POST is supported for event submission.")
		return nil, fmt.Errorf("method %s not allowed", method)
	}

	// Parse headers to find Content-Length
	var contentLength int
	headerEnd := -1
	for i, line := range lines {
		if line == "" {
			headerEnd = i
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			lengthStr := strings.TrimSpace(strings.Split(line, ":")[1])
			var err error
			contentLength, err = strconv.Atoi(lengthStr)
			if err != nil {
				sendHTTPError(conn, 400, "Invalid Content-Length header")
				return nil, fmt.Errorf("invalid content-length: %v", err)
			}
		}
	}

	if headerEnd == -1 {
		sendHTTPError(conn, 400, "Invalid HTTP request - no header/body separator")
		return nil, fmt.Errorf("no header/body separator found")
	}

	// Extract body
	bodyStart := strings.Index(string(data), "\r\n\r\n") + 4
	if bodyStart < 4 || bodyStart >= len(data) {
		// No body or empty body
		if method == "POST" {
			sendHTTPError(conn, 400, "POST request requires JSON body")
			return nil, fmt.Errorf("POST request missing body")
		}
		return []byte("[]"), nil // Empty array for GET requests
	}

	body := data[bodyStart:]

	// Validate content length if specified
	if contentLength > 0 && len(body) != contentLength {
		log.Printf("Warning: Content-Length mismatch. Expected %d, got %d", contentLength, len(body))
	}

	// Send HTTP success response
	sendHTTPSuccess(conn, "Events received successfully")

	return body, nil
}

// processJSONData parses and queues JSON events
func processJSONData(jsonData []byte, remoteAddr net.Addr, events chan json.RawMessage) {
	var batch []json.RawMessage
	if err := json.Unmarshal(jsonData, &batch); err != nil {
		log.Printf("invalid JSON from %s: %v", remoteAddr, err)
		log.Printf("FIRST 100 CHARS: %q", string(jsonData[:min(len(jsonData), 100)]))
		return
	}

	log.Printf("Successfully parsed %d events from %s", len(batch), remoteAddr)

	for _, ev := range batch {
		select {
		case events <- ev:
		default:
			// channel full: drop oldest
			<-events
			events <- ev
			log.Println("queue full: dropping oldest event")
		}
	}
}

// sendHTTPSuccess sends a successful HTTP response
func sendHTTPSuccess(conn net.Conn, message string) {
	response := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n{\"status\":\"success\",\"message\":\"%s\"}",
		len(message)+30, message)
	conn.Write([]byte(response))
}

// sendHTTPError sends an HTTP error response
func sendHTTPError(conn net.Conn, statusCode int, message string) {
	statusText := http.StatusText(statusCode)
	response := fmt.Sprintf("HTTP/1.1 %d %s\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n{\"error\":\"%s\"}",
		statusCode, statusText, len(message)+12, message)
	conn.Write([]byte(response))
}

// drainAndSend drains up to maxBatch events and POSTs them
func drainAndSend(events <-chan json.RawMessage) {
	batch := make([]json.RawMessage, 0, *maxBatch)
	for i := 0; i < *maxBatch; i++ {
		select {
		case ev := <-events:
			batch = append(batch, ev)
		default:
			break
		}
	}
	if len(batch) == 0 {
		return
	}

	payload, err := json.Marshal(batch)
	if err != nil {
		log.Printf("marshal error: %v", err)
		return
	}

	req, err := http.NewRequest("POST", *endpoint, bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("request build error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if *apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+*apiKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	// exponential backoff: 1s, 2s, 4s
	for attempt := 0; attempt < 3; attempt++ {
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("HTTP error (attempt %d): %v", attempt+1, err)
		} else {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return
			}
			body, _ := io.ReadAll(resp.Body)
			log.Printf("HTTP %d: %s", resp.StatusCode, string(body))
		}
		time.Sleep(time.Duration(1<<attempt) * time.Second)
	}
	log.Println("giving up on this batch")
}

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v2"
)

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

// handleConn needs bidirectional channel so it can both send and drop-oldest
func handleConn(conn net.Conn, events chan json.RawMessage) {
	defer conn.Close()
	data, err := io.ReadAll(bufio.NewReader(conn))
	if err != nil {
		log.Printf("read error: %v", err)
		return
	}
	var batch []json.RawMessage
	if err := json.Unmarshal(data, &batch); err != nil {
		log.Printf("invalid JSON from %s: %v", conn.RemoteAddr(), err)
		return
	}
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

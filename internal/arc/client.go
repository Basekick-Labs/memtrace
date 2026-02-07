package arc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/vmihailenco/msgpack/v5"
)

// Client wraps the Arc HTTP API for writing and querying memories
type Client struct {
	baseURL     string
	apiKey      string
	database    string
	measurement string

	httpClient *http.Client
	logger     zerolog.Logger

	// Write batching
	batchSize      int
	flushInterval  time.Duration
	buffer         []map[string]interface{}
	bufferMu       sync.Mutex
	flushTicker    *time.Ticker
	stopCh         chan struct{}
}

// NewClient creates a new Arc HTTP client
func NewClient(baseURL, apiKey, database, measurement string, batchSize int, flushIntervalMS int, connectTimeout int, queryTimeout int, logger zerolog.Logger) *Client {
	c := &Client{
		baseURL:     baseURL,
		apiKey:      apiKey,
		database:    database,
		measurement: measurement,
		httpClient: &http.Client{
			Timeout: time.Duration(queryTimeout) * time.Second,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		logger:        logger.With().Str("component", "arc-client").Logger(),
		batchSize:     batchSize,
		flushInterval: time.Duration(flushIntervalMS) * time.Millisecond,
		buffer:        make([]map[string]interface{}, 0, batchSize),
		stopCh:        make(chan struct{}),
	}

	// Start background flush
	c.flushTicker = time.NewTicker(c.flushInterval)
	go c.backgroundFlush()

	c.logger.Info().
		Str("url", baseURL).
		Str("database", database).
		Int("batch_size", batchSize).
		Int("flush_interval_ms", flushIntervalMS).
		Msg("Arc client initialized")

	return c
}

func (c *Client) backgroundFlush() {
	for {
		select {
		case <-c.flushTicker.C:
			if err := c.Flush(context.Background()); err != nil {
				c.logger.Error().Err(err).Msg("Background flush failed")
			}
		case <-c.stopCh:
			return
		}
	}
}

// BufferWrite adds a record to the write buffer. Flushes when batch size is reached.
func (c *Client) BufferWrite(record map[string]interface{}) error {
	c.bufferMu.Lock()
	c.buffer = append(c.buffer, record)
	shouldFlush := len(c.buffer) >= c.batchSize
	c.bufferMu.Unlock()

	if shouldFlush {
		return c.Flush(context.Background())
	}
	return nil
}

// Flush writes all buffered records to Arc using columnar msgpack format
func (c *Client) Flush(ctx context.Context) error {
	c.bufferMu.Lock()
	if len(c.buffer) == 0 {
		c.bufferMu.Unlock()
		return nil
	}
	records := c.buffer
	c.buffer = make([]map[string]interface{}, 0, c.batchSize)
	c.bufferMu.Unlock()

	return c.writeColumnar(ctx, records)
}

// writeColumnar converts row records to columnar format and writes to Arc
func (c *Client) writeColumnar(ctx context.Context, records []map[string]interface{}) error {
	if len(records) == 0 {
		return nil
	}

	// Collect all column names
	columnSet := make(map[string]bool)
	for _, r := range records {
		for k := range r {
			columnSet[k] = true
		}
	}

	// Build columnar data
	columns := make(map[string][]interface{})
	for col := range columnSet {
		vals := make([]interface{}, len(records))
		for i, r := range records {
			if v, ok := r[col]; ok {
				vals[i] = v
			} else {
				vals[i] = ""
			}
		}
		columns[col] = vals
	}

	payload := map[string]interface{}{
		"m":       c.measurement,
		"columns": columns,
	}

	data, err := msgpack.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal msgpack: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/write/msgpack", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/msgpack")
	req.Header.Set("x-arc-database", c.database)
	if c.apiKey != "" {
		req.Header.Set("x-api-key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to write to Arc: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Arc write failed (status %d): %s", resp.StatusCode, string(body))
	}

	c.logger.Debug().Int("records", len(records)).Msg("Flushed records to Arc")
	return nil
}

// QueryResponse represents a query result from Arc
type QueryResponse struct {
	Columns []string                 `json:"columns"`
	Types   []string                 `json:"types"`
	Data    []map[string]interface{} `json:"data"`
}

// Query executes a SQL query against Arc and returns the results
func (c *Client) Query(ctx context.Context, sql string) ([]map[string]interface{}, error) {
	payload := map[string]string{
		"q": sql,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/query", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-arc-database", c.database)
	if c.apiKey != "" {
		req.Header.Set("x-api-key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query Arc: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Arc query failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result QueryResponse
	if err := json.Unmarshal(body, &result); err != nil {
		// Try as array of maps (some Arc response formats)
		var rows []map[string]interface{}
		if err2 := json.Unmarshal(body, &rows); err2 != nil {
			return nil, fmt.Errorf("failed to parse response: %w (body: %s)", err, string(body))
		}
		return rows, nil
	}

	return result.Data, nil
}

// Ping checks connectivity to Arc
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("failed to create ping request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to ping Arc: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Arc health check failed (status %d)", resp.StatusCode)
	}

	return nil
}

// Close stops the background flush and flushes remaining records
func (c *Client) Close() error {
	close(c.stopCh)
	c.flushTicker.Stop()
	return c.Flush(context.Background())
}

package arc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
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
	batchSize     int
	flushInterval time.Duration
	buffer        []map[string]interface{}
	bufferMu      sync.Mutex
	flushMu       sync.Mutex
	maxBufferSize int
	flushTicker   *time.Ticker
	stopCh        chan struct{}
	closeOnce     sync.Once

	// Health monitoring
	connectTimeout time.Duration
	connected      atomic.Bool
	healthTicker   *time.Ticker
}

// NewClient creates a new Arc HTTP client
func NewClient(baseURL, apiKey, database, measurement string, batchSize int, flushIntervalMS int, connectTimeout int, queryTimeout int, logger zerolog.Logger) *Client {
	connTimeout := time.Duration(connectTimeout) * time.Second

	c := &Client{
		baseURL:     baseURL,
		apiKey:      apiKey,
		database:    database,
		measurement: measurement,
		httpClient: &http.Client{
			Timeout: time.Duration(queryTimeout) * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: connTimeout,
				}).DialContext,
				TLSHandshakeTimeout: connTimeout,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		logger:         logger.With().Str("component", "arc-client").Logger(),
		batchSize:      batchSize,
		flushInterval:  time.Duration(flushIntervalMS) * time.Millisecond,
		buffer:         make([]map[string]interface{}, 0, batchSize),
		maxBufferSize:  batchSize * 100,
		stopCh:         make(chan struct{}),
		connectTimeout: connTimeout,
	}

	// Start background flush
	c.flushTicker = time.NewTicker(c.flushInterval)
	go c.backgroundFlush()

	// Start background health check
	c.healthTicker = time.NewTicker(connTimeout)
	go c.backgroundHealthCheck()

	c.logger.Info().
		Str("url", baseURL).
		Str("database", database).
		Int("batch_size", batchSize).
		Int("flush_interval_ms", flushIntervalMS).
		Int("max_buffer_size", c.maxBufferSize).
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

func (c *Client) backgroundHealthCheck() {
	for {
		select {
		case <-c.healthTicker.C:
			ctx, cancel := context.WithTimeout(context.Background(), c.connectTimeout)
			err := c.Ping(ctx)
			cancel()

			wasConnected := c.connected.Load()
			if err != nil && wasConnected {
				c.connected.Store(false)
				c.logger.Warn().Err(err).Msg("Arc connection lost")
			} else if err == nil && !wasConnected {
				c.connected.Store(true)
				c.logger.Info().Msg("Arc connection restored")
			}
		case <-c.stopCh:
			return
		}
	}
}

// BufferWrite adds a record to the write buffer. Flushes when batch size is reached.
// If the buffer exceeds maxBufferSize, the oldest records are dropped.
func (c *Client) BufferWrite(record map[string]interface{}) error {
	c.bufferMu.Lock()
	c.buffer = append(c.buffer, record)
	if len(c.buffer) > c.maxBufferSize {
		dropped := len(c.buffer) - c.maxBufferSize
		c.buffer = c.buffer[dropped:]
		c.logger.Warn().Int("dropped", dropped).Msg("Buffer overflow, dropped oldest records")
	}
	shouldFlush := len(c.buffer) >= c.batchSize
	c.bufferMu.Unlock()

	if shouldFlush {
		return c.Flush(context.Background())
	}
	return nil
}

// Flush writes all buffered records to Arc using columnar msgpack format.
// On failure, records are re-buffered so they can be retried on the next cycle.
// Serialized via flushMu to prevent concurrent flush races.
func (c *Client) Flush(ctx context.Context) error {
	c.flushMu.Lock()
	defer c.flushMu.Unlock()

	c.bufferMu.Lock()
	if len(c.buffer) == 0 {
		c.bufferMu.Unlock()
		return nil
	}
	records := c.buffer
	c.buffer = make([]map[string]interface{}, 0, c.batchSize)
	c.bufferMu.Unlock()

	if err := c.writeColumnar(ctx, records); err != nil {
		// Re-buffer: prepend failed records before any new ones
		c.bufferMu.Lock()
		combined := make([]map[string]interface{}, 0, len(records)+len(c.buffer))
		combined = append(combined, records...)
		combined = append(combined, c.buffer...)
		if len(combined) > c.maxBufferSize {
			dropped := len(combined) - c.maxBufferSize
			combined = combined[dropped:]
			c.logger.Warn().Int("dropped", dropped).Msg("Buffer overflow, dropped oldest records")
		}
		c.buffer = combined
		c.bufferMu.Unlock()
		return err
	}

	c.logger.Debug().Int("records", len(records)).Msg("Flushed records to Arc")
	return nil
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

	return nil
}

// queryResponse represents a query result from Arc.
// Arc returns columns as a string array and data as an array of arrays (rows of values).
type queryResponse struct {
	Columns []string        `json:"columns"`
	Data    [][]interface{} `json:"data"`
}

// Query executes a SQL query against Arc and returns the results as maps keyed by column name.
func (c *Client) Query(ctx context.Context, sql string) ([]map[string]interface{}, error) {
	payload := map[string]string{
		"sql": sql,
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

	var result queryResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w (body: %s)", err, string(body))
	}

	// Zip columns with row values to produce maps
	rows := make([]map[string]interface{}, 0, len(result.Data))
	for _, row := range result.Data {
		m := make(map[string]interface{}, len(result.Columns))
		for i, col := range result.Columns {
			if i < len(row) {
				m[col] = row[i]
			}
		}
		rows = append(rows, m)
	}

	return rows, nil
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
	io.Copy(io.Discard, resp.Body) //nolint:errcheck // drain for connection reuse

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Arc health check failed (status %d)", resp.StatusCode)
	}

	return nil
}

// IsConnected returns the last known connection state from the background health check.
func (c *Client) IsConnected() bool {
	return c.connected.Load()
}

// MarkConnected sets the connection state to true. Called after a successful startup Ping.
func (c *Client) MarkConnected() {
	c.connected.Store(true)
}

// Close stops the background flush and health check, then flushes remaining records.
// Safe to call multiple times.
func (c *Client) Close() error {
	var flushErr error
	c.closeOnce.Do(func() {
		close(c.stopCh)
		c.flushTicker.Stop()
		c.healthTicker.Stop()
		flushErr = c.Flush(context.Background())
	})
	return flushErr
}

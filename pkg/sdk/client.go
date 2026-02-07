package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client is the memtrace Go SDK client
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New creates a new memtrace client
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// --- Convenience Methods ---

// Remember stores an episodic memory (quick add)
func (c *Client) Remember(ctx context.Context, agentID, content string) (*Memory, error) {
	return c.AddMemory(ctx, &AddMemoryRequest{
		AgentID:    agentID,
		MemoryType: "episodic",
		EventType:  "general",
		Content:    content,
	})
}

// Recall retrieves recent memories for an agent
func (c *Client) Recall(ctx context.Context, agentID string, since string) (*MemoryList, error) {
	return c.ListMemories(ctx, &ListOptions{
		AgentID: agentID,
		Since:   since,
		Order:   "desc",
		Limit:   100,
	})
}

// Decide logs a decision with reasoning
func (c *Client) Decide(ctx context.Context, agentID, decision, reasoning string) (*Memory, error) {
	return c.AddMemory(ctx, &AddMemoryRequest{
		AgentID:    agentID,
		MemoryType: "decision",
		EventType:  "decision",
		Content:    decision,
		Metadata:   map[string]interface{}{"reasoning": reasoning},
	})
}

// --- Memories ---

// AddMemory stores a new memory
func (c *Client) AddMemory(ctx context.Context, req *AddMemoryRequest) (*Memory, error) {
	var mem Memory
	if err := c.post(ctx, "/api/v1/memories", req, &mem); err != nil {
		return nil, err
	}
	return &mem, nil
}

// AddMemories stores multiple memories in a batch
func (c *Client) AddMemories(ctx context.Context, memories []*AddMemoryRequest) ([]Memory, error) {
	payload := map[string]interface{}{
		"memories": memories,
	}
	var result struct {
		Memories []Memory `json:"memories"`
		Count    int      `json:"count"`
	}
	if err := c.post(ctx, "/api/v1/memories", payload, &result); err != nil {
		return nil, err
	}
	return result.Memories, nil
}

// ListMemories lists memories with filters
func (c *Client) ListMemories(ctx context.Context, opts *ListOptions) (*MemoryList, error) {
	params := url.Values{}
	if opts.AgentID != "" {
		params.Set("agent_id", opts.AgentID)
	}
	if opts.SessionID != "" {
		params.Set("session_id", opts.SessionID)
	}
	if opts.MemoryType != "" {
		params.Set("memory_type", opts.MemoryType)
	}
	if opts.EventType != "" {
		params.Set("event_type", opts.EventType)
	}
	if opts.Tags != "" {
		params.Set("tags", opts.Tags)
	}
	if opts.Since != "" {
		params.Set("since", opts.Since)
	}
	if opts.Until != "" {
		params.Set("until", opts.Until)
	}
	if opts.Limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", opts.Limit))
	}
	if opts.Offset > 0 {
		params.Set("offset", fmt.Sprintf("%d", opts.Offset))
	}
	if opts.Order != "" {
		params.Set("order", opts.Order)
	}

	var list MemoryList
	if err := c.get(ctx, "/api/v1/memories?"+params.Encode(), &list); err != nil {
		return nil, err
	}
	return &list, nil
}

// SearchMemories searches memories with structured filters
func (c *Client) SearchMemories(ctx context.Context, query *SearchQuery) (*SearchResult, error) {
	var result SearchResult
	if err := c.post(ctx, "/api/v1/search", query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// --- Agents ---

// RegisterAgent registers a new agent
func (c *Client) RegisterAgent(ctx context.Context, req *RegisterAgentRequest) (*Agent, error) {
	var agent Agent
	if err := c.post(ctx, "/api/v1/agents", req, &agent); err != nil {
		return nil, err
	}
	return &agent, nil
}

// GetAgent returns an agent by ID
func (c *Client) GetAgent(ctx context.Context, id string) (*Agent, error) {
	var agent Agent
	if err := c.get(ctx, "/api/v1/agents/"+id, &agent); err != nil {
		return nil, err
	}
	return &agent, nil
}

// GetAgentStats returns memory stats for an agent
func (c *Client) GetAgentStats(ctx context.Context, id string) (*AgentStats, error) {
	var stats AgentStats
	if err := c.get(ctx, "/api/v1/agents/"+id+"/stats", &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

// --- Sessions ---

// CreateSession starts a new session
func (c *Client) CreateSession(ctx context.Context, req *CreateSessionRequest) (*Session, error) {
	var session Session
	if err := c.post(ctx, "/api/v1/sessions", req, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// GetSession returns a session by ID
func (c *Client) GetSession(ctx context.Context, id string) (*Session, error) {
	var session Session
	if err := c.get(ctx, "/api/v1/sessions/"+id, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// GetSessionContext returns LLM-formatted session context
func (c *Client) GetSessionContext(ctx context.Context, id string, opts *ContextOptions) (*SessionContext, error) {
	var result SessionContext
	if err := c.post(ctx, "/api/v1/sessions/"+id+"/context", opts, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CloseSession closes a session
func (c *Client) CloseSession(ctx context.Context, id string) (*Session, error) {
	var session Session
	payload := map[string]string{"status": "closed"}
	if err := c.put(ctx, "/api/v1/sessions/"+id, payload, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// --- HTTP helpers ---

func (c *Client) post(ctx context.Context, path string, body interface{}, result interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)

	return c.doRequest(req, result)
}

func (c *Client) get(ctx context.Context, path string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("x-api-key", c.apiKey)

	return c.doRequest(req, result)
}

func (c *Client) put(ctx context.Context, path string, body interface{}, result interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)

	return c.doRequest(req, result)
}

func (c *Client) doRequest(req *http.Request, result interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
			return fmt.Errorf("API error (status %d): %s", resp.StatusCode, errResp.Error)
		}
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	if result != nil {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

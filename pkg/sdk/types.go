package sdk

import "time"

// Memory represents a stored memory event
type Memory struct {
	Time       time.Time              `json:"time"`
	OrgID      string                 `json:"org_id"`
	AgentID    string                 `json:"agent_id"`
	SessionID  string                 `json:"session_id,omitempty"`
	MemoryType string                 `json:"memory_type"`
	EventType  string                 `json:"event_type"`
	Content    string                 `json:"content"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Tags       []string               `json:"tags,omitempty"`
	DedupKey   string                 `json:"dedup_key,omitempty"`
	Importance float64                `json:"importance,omitempty"`
	ParentID   string                 `json:"parent_id,omitempty"`
}

// AddMemoryRequest is the request body for adding a memory
type AddMemoryRequest struct {
	AgentID    string                 `json:"agent_id"`
	SessionID  string                 `json:"session_id,omitempty"`
	MemoryType string                 `json:"memory_type"`
	EventType  string                 `json:"event_type"`
	Content    string                 `json:"content"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Tags       []string               `json:"tags,omitempty"`
	DedupKey   string                 `json:"dedup_key,omitempty"`
	Importance float64                `json:"importance,omitempty"`
	ParentID   string                 `json:"parent_id,omitempty"`
}

// ListOptions defines filters for listing memories
type ListOptions struct {
	AgentID    string `json:"agent_id,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
	MemoryType string `json:"memory_type,omitempty"`
	EventType  string `json:"event_type,omitempty"`
	Tags       string `json:"tags,omitempty"`
	Since      string `json:"since,omitempty"`
	Until      string `json:"until,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
	Order      string `json:"order,omitempty"`
}

// MemoryList is the response for listing memories
type MemoryList struct {
	Memories []Memory `json:"memories"`
	Count    int      `json:"count"`
	HasMore  bool     `json:"has_more"`
}

// SearchQuery is the request body for searching memories
type SearchQuery struct {
	AgentID         string   `json:"agent_id,omitempty"`
	SessionID       string   `json:"session_id,omitempty"`
	MemoryTypes     []string `json:"memory_types,omitempty"`
	EventTypes      []string `json:"event_types,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	ContentContains string   `json:"content_contains,omitempty"`
	Since           string   `json:"since,omitempty"`
	Until           string   `json:"until,omitempty"`
	MinImportance   float64  `json:"min_importance,omitempty"`
	Limit           int      `json:"limit,omitempty"`
	Order           string   `json:"order,omitempty"`
}

// SearchResult is the response for searching memories
type SearchResult struct {
	Results     []Memory `json:"results"`
	Count       int      `json:"count"`
	QueryTimeMS int64    `json:"query_time_ms"`
}

// Agent represents a registered agent
type Agent struct {
	ID           string                 `json:"id"`
	OrgID        string                 `json:"org_id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	LastActiveAt *time.Time             `json:"last_active_at,omitempty"`
}

// AgentStats represents memory statistics for an agent
type AgentStats struct {
	AgentID        string         `json:"agent_id"`
	MemoryCount    int            `json:"memory_count"`
	Memories24h    int            `json:"memories_24h"`
	Errors24h      int            `json:"errors_24h"`
	SessionCount   int            `json:"session_count"`
	ActiveSessions int            `json:"active_sessions"`
	LastActiveAt   *time.Time     `json:"last_active_at,omitempty"`
	MemoryTypes    map[string]int `json:"memory_types"`
}

// Session represents a bounded context for agent work
type Session struct {
	ID        string                 `json:"id"`
	OrgID     string                 `json:"org_id"`
	AgentID   string                 `json:"agent_id"`
	Status    string                 `json:"status"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	ClosedAt  *time.Time             `json:"closed_at,omitempty"`
}

// CreateSessionRequest is the request body for creating a session
type CreateSessionRequest struct {
	AgentID  string                 `json:"agent_id"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ContextOptions controls how session context is formatted
type ContextOptions struct {
	MaxTokens    int      `json:"max_tokens,omitempty"`
	IncludeTypes []string `json:"include_types,omitempty"`
	Since        string   `json:"since,omitempty"`
}

// SessionContext is the LLM-ready context for a session
type SessionContext struct {
	SessionID   string `json:"session_id"`
	Context     string `json:"context"`
	MemoryCount int    `json:"memory_count"`
}

// RegisterAgentRequest is the request body for registering an agent
type RegisterAgentRequest struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

package memory

import "time"

// Memory types
const (
	TypeEpisodic = "episodic"
	TypeSession  = "session"
	TypeDecision = "decision"
	TypeEntity   = "entity"
)

// Memory represents a stored memory event
type Memory struct {
	Time         time.Time              `json:"time"`
	OrgID        string                 `json:"org_id"`
	AgentID      string                 `json:"agent_id"`
	SessionID    string                 `json:"session_id,omitempty"`
	MemoryType   string                 `json:"memory_type"`
	EventType    string                 `json:"event_type"`
	Content      string                 `json:"content"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
	DedupKey     string                 `json:"dedup_key,omitempty"`
	Importance   float64                `json:"importance,omitempty"`
	ParentID     string                 `json:"parent_id,omitempty"`
}

// CreateRequest is the request body for creating a memory
type CreateRequest struct {
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

// BatchCreateRequest is the request body for creating multiple memories
type BatchCreateRequest struct {
	Memories []CreateRequest `json:"memories"`
}

// ListOptions defines filters for listing memories
type ListOptions struct {
	AgentID    string
	SessionID  string
	MemoryType string
	EventType  string
	Tags       string // comma-separated
	Since      string // relative: "2h", "24h", "7d" or ISO8601
	Until      string // relative or ISO8601
	Limit      int
	Offset     int
	Order      string // "asc" or "desc"
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

// SearchResult is the response for search
type SearchResult struct {
	Results     []Memory `json:"results"`
	Count       int      `json:"count"`
	QueryTimeMS int64    `json:"query_time_ms"`
}

// ValidMemoryTypes is the set of valid memory types
var ValidMemoryTypes = map[string]bool{
	TypeEpisodic: true,
	TypeSession:  true,
	TypeDecision: true,
	TypeEntity:   true,
}

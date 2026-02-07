package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Basekick-Labs/memtrace/internal/arc"
	"github.com/Basekick-Labs/memtrace/internal/sanitize"
	"github.com/rs/zerolog"
)

// Manager orchestrates memory CRUD operations
type Manager struct {
	arcClient   *arc.Client
	dedupEngine *DedupEngine
	logger      zerolog.Logger
}

// NewManager creates a new memory manager
func NewManager(arcClient *arc.Client, dedupEngine *DedupEngine, logger zerolog.Logger) *Manager {
	return &Manager{
		arcClient:   arcClient,
		dedupEngine: dedupEngine,
		logger:      logger.With().Str("component", "memory").Logger(),
	}
}

// Create stores a new memory in Arc
func (m *Manager) Create(ctx context.Context, orgID string, req *CreateRequest) (*Memory, error) {
	if req.AgentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}
	if req.Content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if req.MemoryType == "" {
		req.MemoryType = TypeEpisodic
	}
	if !ValidMemoryTypes[req.MemoryType] {
		return nil, fmt.Errorf("invalid memory_type: %s", req.MemoryType)
	}
	if req.EventType == "" {
		req.EventType = "general"
	}

	// Validate IDs
	if err := sanitize.ValidateID(req.AgentID); err != nil {
		return nil, fmt.Errorf("invalid agent_id: %w", err)
	}
	if err := sanitize.ValidateID(req.SessionID); err != nil {
		return nil, fmt.Errorf("invalid session_id: %w", err)
	}
	if err := sanitize.ValidateType(req.EventType); err != nil {
		return nil, fmt.Errorf("invalid event_type: %w", err)
	}
	for _, tag := range req.Tags {
		if err := sanitize.ValidateTag(tag); err != nil {
			return nil, fmt.Errorf("invalid tag '%s': %w", tag, err)
		}
	}

	// Generate dedup key if not provided
	dedupKey := req.DedupKey
	if dedupKey == "" && m.dedupEngine != nil {
		dedupKey = GenerateKey(req.AgentID, req.EventType, req.Content)
	}

	// Check dedup
	if m.dedupEngine != nil && dedupKey != "" {
		if err := m.dedupEngine.Check(ctx, orgID, dedupKey); err != nil {
			return nil, err
		}
	}

	metadataJSON := "{}"
	if req.Metadata != nil {
		b, err := json.Marshal(req.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize metadata: %w", err)
		}
		metadataJSON = string(b)
	}

	tagsCSV := ""
	if len(req.Tags) > 0 {
		tagsCSV = strings.Join(req.Tags, ",")
	}

	now := time.Now().UTC()

	record := map[string]interface{}{
		"time":          now.UnixNano(),
		"org_id":        orgID,
		"agent_id":      req.AgentID,
		"session_id":    req.SessionID,
		"memory_type":   req.MemoryType,
		"event_type":    req.EventType,
		"content":       req.Content,
		"metadata_json": metadataJSON,
		"tags_csv":      tagsCSV,
		"dedup_key":     dedupKey,
		"importance":    req.Importance,
		"parent_id":     req.ParentID,
	}

	if err := m.arcClient.BufferWrite(record); err != nil {
		return nil, fmt.Errorf("failed to write memory: %w", err)
	}

	mem := &Memory{
		Time:       now,
		OrgID:      orgID,
		AgentID:    req.AgentID,
		SessionID:  req.SessionID,
		MemoryType: req.MemoryType,
		EventType:  req.EventType,
		Content:    req.Content,
		Metadata:   req.Metadata,
		Tags:       req.Tags,
		DedupKey:   dedupKey,
		Importance: req.Importance,
		ParentID:   req.ParentID,
	}

	m.logger.Debug().
		Str("agent_id", req.AgentID).
		Str("memory_type", req.MemoryType).
		Str("event_type", req.EventType).
		Msg("Memory created")

	return mem, nil
}

// CreateBatch stores multiple memories
func (m *Manager) CreateBatch(ctx context.Context, orgID string, requests []CreateRequest) ([]*Memory, error) {
	results := make([]*Memory, 0, len(requests))
	for i := range requests {
		mem, err := m.Create(ctx, orgID, &requests[i])
		if err != nil {
			if err == ErrDuplicate {
				continue
			}
			return results, fmt.Errorf("failed to create memory %d: %w", i, err)
		}
		results = append(results, mem)
	}
	if err := m.arcClient.Flush(ctx); err != nil {
		m.logger.Error().Err(err).Msg("Failed to flush after batch create")
	}
	return results, nil
}

// List queries memories from Arc with filters
func (m *Manager) List(ctx context.Context, orgID string, opts *ListOptions) (*MemoryList, error) {
	if err := validateListOptions(opts); err != nil {
		return nil, err
	}

	sql := buildListSQL(orgID, opts)

	if err := m.arcClient.Flush(ctx); err != nil {
		m.logger.Warn().Err(err).Msg("Failed to flush before list")
	}

	rows, err := m.arcClient.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("failed to query memories: %w", err)
	}

	memories := make([]Memory, 0, len(rows))
	for _, row := range rows {
		memories = append(memories, rowToMemory(row))
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}

	return &MemoryList{
		Memories: memories,
		Count:    len(memories),
		HasMore:  len(memories) >= limit,
	}, nil
}

// Search queries memories with structured filters
func (m *Manager) Search(ctx context.Context, orgID string, query *SearchQuery) (*SearchResult, error) {
	if err := validateSearchQuery(query); err != nil {
		return nil, err
	}

	start := time.Now()
	sql := buildSearchSQL(orgID, query)

	if err := m.arcClient.Flush(ctx); err != nil {
		m.logger.Warn().Err(err).Msg("Failed to flush before search")
	}

	rows, err := m.arcClient.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("failed to search memories: %w", err)
	}

	results := make([]Memory, 0, len(rows))
	for _, row := range rows {
		results = append(results, rowToMemory(row))
	}

	return &SearchResult{
		Results:     results,
		Count:       len(results),
		QueryTimeMS: time.Since(start).Milliseconds(),
	}, nil
}

// validateListOptions validates all user-provided filter values
func validateListOptions(opts *ListOptions) error {
	if err := sanitize.ValidateID(opts.AgentID); err != nil {
		return fmt.Errorf("invalid agent_id: %w", err)
	}
	if err := sanitize.ValidateID(opts.SessionID); err != nil {
		return fmt.Errorf("invalid session_id: %w", err)
	}
	if err := sanitize.ValidateType(opts.MemoryType); err != nil {
		return fmt.Errorf("invalid memory_type: %w", err)
	}
	if err := sanitize.ValidateType(opts.EventType); err != nil {
		return fmt.Errorf("invalid event_type: %w", err)
	}
	if opts.Tags != "" {
		for _, tag := range strings.Split(opts.Tags, ",") {
			if err := sanitize.ValidateTag(strings.TrimSpace(tag)); err != nil {
				return fmt.Errorf("invalid tag: %w", err)
			}
		}
	}
	return nil
}

// validateSearchQuery validates search query fields
func validateSearchQuery(q *SearchQuery) error {
	if err := sanitize.ValidateID(q.AgentID); err != nil {
		return fmt.Errorf("invalid agent_id: %w", err)
	}
	if err := sanitize.ValidateID(q.SessionID); err != nil {
		return fmt.Errorf("invalid session_id: %w", err)
	}
	for _, t := range q.MemoryTypes {
		if err := sanitize.ValidateType(t); err != nil {
			return fmt.Errorf("invalid memory_type: %w", err)
		}
	}
	for _, t := range q.EventTypes {
		if err := sanitize.ValidateType(t); err != nil {
			return fmt.Errorf("invalid event_type: %w", err)
		}
	}
	for _, tag := range q.Tags {
		if err := sanitize.ValidateTag(tag); err != nil {
			return fmt.Errorf("invalid tag: %w", err)
		}
	}
	return nil
}

// buildListSQL constructs a SQL query from ListOptions.
// All inputs must be validated before calling this function.
func buildListSQL(orgID string, opts *ListOptions) string {
	var conditions []string
	conditions = append(conditions, sanitize.SQLCondition("org_id", orgID))

	if opts.AgentID != "" {
		conditions = append(conditions, sanitize.SQLCondition("agent_id", opts.AgentID))
	}
	if opts.SessionID != "" {
		conditions = append(conditions, sanitize.SQLCondition("session_id", opts.SessionID))
	}
	if opts.MemoryType != "" {
		conditions = append(conditions, sanitize.SQLCondition("memory_type", opts.MemoryType))
	}
	if opts.EventType != "" {
		conditions = append(conditions, sanitize.SQLCondition("event_type", opts.EventType))
	}
	if opts.Tags != "" {
		for _, tag := range strings.Split(opts.Tags, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				conditions = append(conditions, sanitize.SQLLikeCondition("tags_csv", tag))
			}
		}
	}

	if clause := parseSince(opts.Since); clause != "" {
		conditions = append(conditions, clause)
	}
	if clause := parseUntil(opts.Until); clause != "" {
		conditions = append(conditions, clause)
	}

	order := "DESC"
	if opts.Order == "asc" {
		order = "ASC"
	}

	limit := opts.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	return fmt.Sprintf(
		"SELECT * FROM events WHERE %s ORDER BY time %s LIMIT %d OFFSET %d",
		strings.Join(conditions, " AND "), order, limit, offset,
	)
}

// buildSearchSQL constructs a SQL query from SearchQuery.
// All inputs must be validated before calling this function.
func buildSearchSQL(orgID string, q *SearchQuery) string {
	var conditions []string
	conditions = append(conditions, sanitize.SQLCondition("org_id", orgID))

	if q.AgentID != "" {
		conditions = append(conditions, sanitize.SQLCondition("agent_id", q.AgentID))
	}
	if q.SessionID != "" {
		conditions = append(conditions, sanitize.SQLCondition("session_id", q.SessionID))
	}
	if len(q.MemoryTypes) > 0 {
		conditions = append(conditions, sanitize.SQLInCondition("memory_type", q.MemoryTypes))
	}
	if len(q.EventTypes) > 0 {
		conditions = append(conditions, sanitize.SQLInCondition("event_type", q.EventTypes))
	}
	if len(q.Tags) > 0 {
		for _, tag := range q.Tags {
			conditions = append(conditions, sanitize.SQLLikeCondition("tags_csv", tag))
		}
	}
	if q.ContentContains != "" {
		conditions = append(conditions, sanitize.SQLLikeCondition("content", q.ContentContains))
	}
	if q.MinImportance > 0 {
		conditions = append(conditions, fmt.Sprintf("importance >= %f", q.MinImportance))
	}

	if clause := parseSince(q.Since); clause != "" {
		conditions = append(conditions, clause)
	}
	if clause := parseUntil(q.Until); clause != "" {
		conditions = append(conditions, clause)
	}

	order := "DESC"
	if q.Order == "asc" {
		order = "ASC"
	}
	limit := q.Limit
	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	return fmt.Sprintf(
		"SELECT * FROM events WHERE %s ORDER BY time %s LIMIT %d",
		strings.Join(conditions, " AND "), order, limit,
	)
}

func parseSince(since string) string {
	if since == "" {
		return ""
	}
	interval := sanitize.ParseRelativeTime(since)
	if interval != "" {
		return fmt.Sprintf("time > (CURRENT_TIMESTAMP - INTERVAL '%s')", interval)
	}
	// ISO8601 fallback
	return fmt.Sprintf("time > '%s'", sanitize.EscapeSQL(since))
}

func parseUntil(until string) string {
	if until == "" || until == "now" {
		return ""
	}
	interval := sanitize.ParseRelativeTime(until)
	if interval != "" {
		return fmt.Sprintf("time < (CURRENT_TIMESTAMP - INTERVAL '%s')", interval)
	}
	return fmt.Sprintf("time < '%s'", sanitize.EscapeSQL(until))
}

// rowToMemory converts an Arc query result row to a Memory struct
func rowToMemory(row map[string]interface{}) Memory {
	mem := Memory{}

	if v, ok := row["time"]; ok {
		switch t := v.(type) {
		case string:
			if parsed, err := time.Parse(time.RFC3339Nano, t); err == nil {
				mem.Time = parsed
			}
		case float64:
			mem.Time = time.Unix(0, int64(t))
		}
	}
	if v, ok := row["org_id"].(string); ok {
		mem.OrgID = v
	}
	if v, ok := row["agent_id"].(string); ok {
		mem.AgentID = v
	}
	if v, ok := row["session_id"].(string); ok {
		mem.SessionID = v
	}
	if v, ok := row["memory_type"].(string); ok {
		mem.MemoryType = v
	}
	if v, ok := row["event_type"].(string); ok {
		mem.EventType = v
	}
	if v, ok := row["content"].(string); ok {
		mem.Content = v
	}
	if v, ok := row["metadata_json"].(string); ok && v != "" && v != "{}" {
		var md map[string]interface{}
		if json.Unmarshal([]byte(v), &md) == nil {
			mem.Metadata = md
		}
	}
	if v, ok := row["tags_csv"].(string); ok && v != "" {
		mem.Tags = strings.Split(v, ",")
	}
	if v, ok := row["dedup_key"].(string); ok {
		mem.DedupKey = v
	}
	if v, ok := row["importance"].(float64); ok {
		mem.Importance = v
	}
	if v, ok := row["parent_id"].(string); ok {
		mem.ParentID = v
	}

	return mem
}

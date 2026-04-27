package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Basekick-Labs/memtrace/internal/arc"
	"github.com/Basekick-Labs/memtrace/internal/metadata"
	"github.com/Basekick-Labs/memtrace/internal/sanitize"
	"github.com/rs/zerolog"
)

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

// CreateRequest is the request body for creating a session
type CreateRequest struct {
	AgentID  string                 `json:"agent_id"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateRequest is the request body for updating a session
type UpdateRequest struct {
	Status   string                 `json:"status,omitempty"`
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

// Manager handles session lifecycle
type Manager struct {
	db          *sql.DB
	arcRegistry *arc.Registry
	logger      zerolog.Logger
}

// NewManager creates a new session manager
func NewManager(metaDB *metadata.DB, arcRegistry *arc.Registry, logger zerolog.Logger) *Manager {
	return &Manager{
		db:          metaDB.GetDB(),
		arcRegistry: arcRegistry,
		logger:      logger.With().Str("component", "session").Logger(),
	}
}

// Create starts a new session
func (m *Manager) Create(ctx context.Context, orgID string, req *CreateRequest) (*Session, error) {
	if req.AgentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}

	id := metadata.GenerateID("sess_")
	now := time.Now().UTC()

	metadataJSON := "{}"
	if req.Metadata != nil {
		b, err := json.Marshal(req.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize metadata: %w", err)
		}
		metadataJSON = string(b)
	}

	_, err := m.db.ExecContext(ctx, `
		INSERT INTO sessions (id, org_id, agent_id, status, metadata_json, created_at, updated_at)
		VALUES (?, ?, ?, 'active', ?, ?, ?)
	`, id, orgID, req.AgentID, metadataJSON, now, now)

	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	session := &Session{
		ID:        id,
		OrgID:     orgID,
		AgentID:   req.AgentID,
		Status:    "active",
		Metadata:  req.Metadata,
		CreatedAt: now,
		UpdatedAt: now,
	}

	m.logger.Info().Str("id", id).Str("agent_id", req.AgentID).Msg("Session created")
	return session, nil
}

// Get returns a session by ID
func (m *Manager) Get(ctx context.Context, orgID, sessionID string) (*Session, error) {
	var (
		id           string
		org          string
		agentID      string
		status       string
		metadataJSON sql.NullString
		createdAt    time.Time
		updatedAt    time.Time
		closedAt     sql.NullTime
	)

	err := m.db.QueryRowContext(ctx, `
		SELECT id, org_id, agent_id, status, metadata_json, created_at, updated_at, closed_at
		FROM sessions WHERE id = ? AND org_id = ?
	`, sessionID, orgID).Scan(&id, &org, &agentID, &status, &metadataJSON, &createdAt, &updatedAt, &closedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	session := &Session{
		ID:        id,
		OrgID:     org,
		AgentID:   agentID,
		Status:    status,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
	if metadataJSON.Valid && metadataJSON.String != "{}" {
		if err := json.Unmarshal([]byte(metadataJSON.String), &session.Metadata); err != nil {
			m.logger.Warn().Err(err).Str("session_id", id).Msg("Failed to parse session metadata")
		}
	}
	if closedAt.Valid {
		session.ClosedAt = &closedAt.Time
	}

	return session, nil
}

// List returns sessions for an org, optionally filtered by agent
func (m *Manager) List(ctx context.Context, orgID, agentID string) ([]*Session, error) {
	query := "SELECT id, org_id, agent_id, status, metadata_json, created_at, updated_at, closed_at FROM sessions WHERE org_id = ?"
	args := []interface{}{orgID}

	if agentID != "" {
		query += " AND agent_id = ?"
		args = append(args, agentID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		var (
			id           string
			org          string
			agent        string
			status       string
			metadataJSON sql.NullString
			createdAt    time.Time
			updatedAt    time.Time
			closedAt     sql.NullTime
		)
		if err := rows.Scan(&id, &org, &agent, &status, &metadataJSON, &createdAt, &updatedAt, &closedAt); err != nil {
			return nil, err
		}
		s := &Session{
			ID:        id,
			OrgID:     org,
			AgentID:   agent,
			Status:    status,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		}
		if metadataJSON.Valid && metadataJSON.String != "{}" {
			if err := json.Unmarshal([]byte(metadataJSON.String), &s.Metadata); err != nil {
				m.logger.Warn().Err(err).Str("session_id", id).Msg("Failed to parse session metadata")
			}
		}
		if closedAt.Valid {
			s.ClosedAt = &closedAt.Time
		}
		sessions = append(sessions, s)
	}

	return sessions, nil
}

// Update updates a session's status or metadata
func (m *Manager) Update(ctx context.Context, orgID, sessionID string, req *UpdateRequest) (*Session, error) {
	now := time.Now().UTC()

	var updates []string
	var args []interface{}

	if req.Status != "" {
		updates = append(updates, "status = ?")
		args = append(args, req.Status)
		if req.Status == "closed" {
			updates = append(updates, "closed_at = ?")
			args = append(args, now)
		}
	}
	if req.Metadata != nil {
		b, err := json.Marshal(req.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize metadata: %w", err)
		}
		updates = append(updates, "metadata_json = ?")
		args = append(args, string(b))
	}

	updates = append(updates, "updated_at = ?")
	args = append(args, now)
	args = append(args, sessionID, orgID)

	query := fmt.Sprintf("UPDATE sessions SET %s WHERE id = ? AND org_id = ?", strings.Join(updates, ", "))
	result, err := m.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, errors.New("session not found")
	}

	return m.Get(ctx, orgID, sessionID)
}

// GetContext returns LLM-ready formatted context for a session
func (m *Manager) GetContext(ctx context.Context, orgID, sessionID string, opts *ContextOptions) (*SessionContext, error) {
	// Validate inputs before building Arc SQL
	if err := sanitize.ValidateID(orgID); err != nil {
		return nil, fmt.Errorf("invalid org_id: %w", err)
	}
	if err := sanitize.ValidateID(sessionID); err != nil {
		return nil, fmt.Errorf("invalid session_id: %w", err)
	}

	// Validate include_types
	for _, t := range opts.IncludeTypes {
		if err := sanitize.ValidateType(t); err != nil {
			return nil, fmt.Errorf("invalid include_type '%s': %w", t, err)
		}
	}

	// Default options
	if opts.Since == "" {
		opts.Since = "24h"
	}

	// Parse relative time using shared sanitize package
	sinceInterval := sanitize.ParseRelativeTime(opts.Since)
	if sinceInterval == "" {
		sinceInterval = "24 hours"
	}

	// Build SQL with validated and escaped values
	var conditions []string
	conditions = append(conditions, sanitize.SQLCondition("org_id", orgID))
	conditions = append(conditions, sanitize.SQLCondition("session_id", sessionID))
	conditions = append(conditions, fmt.Sprintf("time > (CURRENT_TIMESTAMP - INTERVAL '%s')", sinceInterval))

	if len(opts.IncludeTypes) > 0 {
		conditions = append(conditions, sanitize.SQLInCondition("memory_type", opts.IncludeTypes))
	}

	query := fmt.Sprintf(
		"SELECT time, memory_type, event_type, content, importance FROM events WHERE %s ORDER BY time DESC LIMIT 100",
		strings.Join(conditions, " AND "),
	)

	arcClient, err := m.arcRegistry.Get(orgID)
	if err != nil {
		return nil, err
	}

	// Flush before reading
	arcClient.Flush(ctx)

	rows, err := arcClient.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query session context: %w", err)
	}

	// Format as markdown for LLMs
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Session Context (%s)\n\n", sessionID))

	// Group by memory type
	actions := []string{}
	decisions := []string{}
	errs := []string{}
	other := []string{}

	for _, row := range rows {
		memType, _ := row["memory_type"].(string)
		eventType, _ := row["event_type"].(string)
		content, _ := row["content"].(string)
		timeStr, _ := row["time"].(string)

		// Apply first 30 chars hint pattern to avoid LLM repetition
		hint := content
		if len(hint) > 80 {
			hint = hint[:80] + "..."
		}

		line := fmt.Sprintf("- [%s] %s: %s", timeStr, eventType, hint)

		switch memType {
		case "episodic":
			actions = append(actions, line)
		case "decision":
			decisions = append(decisions, line)
		case "entity":
			other = append(other, line)
		default:
			if eventType == "error" {
				errs = append(errs, line)
			} else {
				other = append(other, line)
			}
		}
	}

	if len(actions) > 0 {
		sb.WriteString(fmt.Sprintf("### Recent Actions (%d)\n", len(actions)))
		for _, a := range actions {
			sb.WriteString(a + "\n")
		}
		sb.WriteString("\n")
	}

	if len(decisions) > 0 {
		sb.WriteString(fmt.Sprintf("### Decisions Made (%d)\n", len(decisions)))
		for _, d := range decisions {
			sb.WriteString(d + "\n")
		}
		sb.WriteString("\n")
	}

	if len(errs) > 0 {
		sb.WriteString(fmt.Sprintf("### Errors (%d)\n", len(errs)))
		for _, e := range errs {
			sb.WriteString(e + "\n")
		}
		sb.WriteString("\n")
	}

	if len(other) > 0 {
		sb.WriteString(fmt.Sprintf("### Other (%d)\n", len(other)))
		for _, o := range other {
			sb.WriteString(o + "\n")
		}
	}

	return &SessionContext{
		SessionID:   sessionID,
		Context:     sb.String(),
		MemoryCount: len(rows),
	}, nil
}

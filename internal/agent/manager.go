package agent

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

// CreateRequest is the request body for registering an agent
type CreateRequest struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// Stats represents memory statistics for an agent
type Stats struct {
	AgentID        string         `json:"agent_id"`
	MemoryCount    int            `json:"memory_count"`
	Memories24h    int            `json:"memories_24h"`
	Errors24h      int            `json:"errors_24h"`
	SessionCount   int            `json:"session_count"`
	ActiveSessions int            `json:"active_sessions"`
	LastActiveAt   *time.Time     `json:"last_active_at,omitempty"`
	MemoryTypes    map[string]int `json:"memory_types"`
}

// Manager handles agent CRUD
type Manager struct {
	db        *sql.DB
	arcClient *arc.Client
	logger    zerolog.Logger
}

// NewManager creates a new agent manager
func NewManager(metaDB *metadata.DB, arcClient *arc.Client, logger zerolog.Logger) *Manager {
	return &Manager{
		db:        metaDB.GetDB(),
		arcClient: arcClient,
		logger:    logger.With().Str("component", "agent").Logger(),
	}
}

// Create registers a new agent
func (m *Manager) Create(ctx context.Context, orgID string, req *CreateRequest) (*Agent, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	id := metadata.GenerateID("agent_")

	configJSON := "{}"
	if req.Config != nil {
		b, err := json.Marshal(req.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize config: %w", err)
		}
		configJSON = string(b)
	}

	_, err := m.db.ExecContext(ctx, `
		INSERT INTO agents (id, org_id, name, description, config_json)
		VALUES (?, ?, ?, ?, ?)
	`, id, orgID, req.Name, req.Description, configJSON)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, fmt.Errorf("agent '%s' already exists in this organization", req.Name)
		}
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	agent := &Agent{
		ID:          id,
		OrgID:       orgID,
		Name:        req.Name,
		Description: req.Description,
		Config:      req.Config,
		CreatedAt:   time.Now().UTC(),
	}

	m.logger.Info().Str("id", id).Str("name", req.Name).Msg("Agent registered")
	return agent, nil
}

// Get returns an agent by ID
func (m *Manager) Get(ctx context.Context, orgID, agentID string) (*Agent, error) {
	var (
		id          string
		org         string
		name        string
		description sql.NullString
		configJSON  sql.NullString
		createdAt   time.Time
		lastActive  sql.NullTime
	)

	err := m.db.QueryRowContext(ctx, `
		SELECT id, org_id, name, description, config_json, created_at, last_active_at
		FROM agents WHERE (id = ? OR name = ?) AND org_id = ?
	`, agentID, agentID, orgID).Scan(&id, &org, &name, &description, &configJSON, &createdAt, &lastActive)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	agent := &Agent{
		ID:          id,
		OrgID:       org,
		Name:        name,
		Description: description.String,
		CreatedAt:   createdAt,
	}
	if configJSON.Valid && configJSON.String != "{}" {
		if err := json.Unmarshal([]byte(configJSON.String), &agent.Config); err != nil {
			m.logger.Warn().Err(err).Str("agent_id", id).Msg("Failed to parse agent config")
		}
	}
	if lastActive.Valid {
		agent.LastActiveAt = &lastActive.Time
	}

	return agent, nil
}

// List returns all agents for an org
func (m *Manager) List(ctx context.Context, orgID string) ([]*Agent, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, org_id, name, description, config_json, created_at, last_active_at
		FROM agents WHERE org_id = ? ORDER BY created_at DESC
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*Agent
	for rows.Next() {
		var (
			id          string
			org         string
			name        string
			description sql.NullString
			configJSON  sql.NullString
			createdAt   time.Time
			lastActive  sql.NullTime
		)
		if err := rows.Scan(&id, &org, &name, &description, &configJSON, &createdAt, &lastActive); err != nil {
			return nil, err
		}
		agent := &Agent{
			ID:          id,
			OrgID:       org,
			Name:        name,
			Description: description.String,
			CreatedAt:   createdAt,
		}
		if configJSON.Valid && configJSON.String != "{}" {
			if err := json.Unmarshal([]byte(configJSON.String), &agent.Config); err != nil {
				m.logger.Warn().Err(err).Str("agent_id", id).Msg("Failed to parse agent config")
			}
		}
		if lastActive.Valid {
			agent.LastActiveAt = &lastActive.Time
		}
		agents = append(agents, agent)
	}

	return agents, nil
}

// GetStats returns memory stats for an agent
func (m *Manager) GetStats(ctx context.Context, orgID, agentID string) (*Stats, error) {
	// Validate inputs before building Arc SQL queries
	if err := sanitize.ValidateID(orgID); err != nil {
		return nil, fmt.Errorf("invalid org_id: %w", err)
	}
	if err := sanitize.ValidateID(agentID); err != nil {
		return nil, fmt.Errorf("invalid agent_id: %w", err)
	}

	stats := &Stats{
		AgentID:     agentID,
		MemoryTypes: make(map[string]int),
	}

	// Session counts from SQLite (parameterized — safe)
	m.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sessions WHERE org_id = ? AND agent_id = ?",
		orgID, agentID,
	).Scan(&stats.SessionCount)

	m.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sessions WHERE org_id = ? AND agent_id = ? AND status = 'active'",
		orgID, agentID,
	).Scan(&stats.ActiveSessions)

	// Build Arc SQL conditions using validated + escaped values
	orgCond := sanitize.SQLCondition("org_id", orgID)
	agentCond := sanitize.SQLCondition("agent_id", agentID)
	baseWhere := orgCond + " AND " + agentCond

	// Total memory count
	countSQL := fmt.Sprintf("SELECT COUNT(*) as cnt FROM events WHERE %s", baseWhere)
	rows, err := m.arcClient.Query(ctx, countSQL)
	if err == nil && len(rows) > 0 {
		stats.MemoryCount = parseCount(rows[0]["cnt"])
	}

	// 24h counts
	count24hSQL := fmt.Sprintf("SELECT COUNT(*) as cnt FROM events WHERE %s AND time > (CURRENT_TIMESTAMP - INTERVAL '24 hours')", baseWhere)
	rows, err = m.arcClient.Query(ctx, count24hSQL)
	if err == nil && len(rows) > 0 {
		stats.Memories24h = parseCount(rows[0]["cnt"])
	}

	// Error count 24h
	errSQL := fmt.Sprintf("SELECT COUNT(*) as cnt FROM events WHERE %s AND event_type = 'error' AND time > (CURRENT_TIMESTAMP - INTERVAL '24 hours')", baseWhere)
	rows, err = m.arcClient.Query(ctx, errSQL)
	if err == nil && len(rows) > 0 {
		stats.Errors24h = parseCount(rows[0]["cnt"])
	}

	// Memory type breakdown
	typeSQL := fmt.Sprintf("SELECT memory_type, COUNT(*) as cnt FROM events WHERE %s GROUP BY memory_type", baseWhere)
	rows, err = m.arcClient.Query(ctx, typeSQL)
	if err == nil {
		for _, row := range rows {
			if t, ok := row["memory_type"].(string); ok {
				stats.MemoryTypes[t] = parseCount(row["cnt"])
			}
		}
	}

	// Update last_active_at
	agent, _ := m.Get(ctx, orgID, agentID)
	if agent != nil {
		stats.LastActiveAt = agent.LastActiveAt
	}

	return stats, nil
}

// parseCount extracts an int from a JSON-decoded number value
func parseCount(v interface{}) int {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int64:
		return int(n)
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int(i)
		}
	}
	return 0
}

// UpdateLastActive updates the last_active_at for an agent
func (m *Manager) UpdateLastActive(ctx context.Context, agentID string) {
	m.db.ExecContext(ctx, "UPDATE agents SET last_active_at = ? WHERE id = ?", time.Now().UTC(), agentID)
}

// Delete removes an agent
func (m *Manager) Delete(ctx context.Context, orgID, agentID string) error {
	result, err := m.db.ExecContext(ctx, "DELETE FROM agents WHERE id = ? AND org_id = ?", agentID, orgID)
	if err != nil {
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.New("agent not found")
	}
	return nil
}

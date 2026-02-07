package metadata

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQLite metadata database
type DB struct {
	db     *sql.DB
	logger zerolog.Logger
}

// New creates a new metadata DB and initializes the schema
func New(dbPath string, logger zerolog.Logger) (*DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("failed to open metadata database: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	mdb := &DB{
		db:     db,
		logger: logger.With().Str("component", "metadata").Logger(),
	}

	if err := mdb.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	mdb.logger.Info().Str("db_path", dbPath).Msg("Metadata database initialized")
	return mdb, nil
}

func (mdb *DB) initSchema() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS organizations (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			settings_json TEXT DEFAULT '{}'
		)`,
		`CREATE TABLE IF NOT EXISTS api_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key_hash TEXT NOT NULL,
			key_prefix TEXT NOT NULL,
			name TEXT NOT NULL UNIQUE,
			org_id TEXT NOT NULL,
			permissions TEXT DEFAULT 'read,write',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP,
			enabled INTEGER DEFAULT 1,
			FOREIGN KEY (org_id) REFERENCES organizations(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(key_prefix)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_enabled ON api_keys(enabled)`,
		`CREATE TABLE IF NOT EXISTS agents (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			config_json TEXT DEFAULT '{}',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_active_at TIMESTAMP,
			FOREIGN KEY (org_id) REFERENCES organizations(id),
			UNIQUE(org_id, name)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_agents_org ON agents(org_id)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			status TEXT DEFAULT 'active',
			metadata_json TEXT DEFAULT '{}',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			closed_at TIMESTAMP,
			FOREIGN KEY (org_id) REFERENCES organizations(id),
			FOREIGN KEY (agent_id) REFERENCES agents(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_org ON sessions(org_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_agent ON sessions(agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status)`,
	}

	for _, stmt := range statements {
		if _, err := mdb.db.Exec(stmt); err != nil {
			return fmt.Errorf("failed to execute schema statement: %w", err)
		}
	}

	return nil
}

// GenerateID creates a prefixed random ID (e.g., "org_a1b2c3d4")
func GenerateID(prefix string) string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate random ID: %v", err))
	}
	return prefix + hex.EncodeToString(b)
}

// GetDB returns the underlying sql.DB for use by other packages
func (mdb *DB) GetDB() *sql.DB {
	return mdb.db
}

// Close closes the metadata database
func (mdb *DB) Close() error {
	return mdb.db.Close()
}

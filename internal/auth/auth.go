package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
)

// KeyInfo represents API key metadata
type KeyInfo struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	OrgID       string    `json:"org_id"`
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	Enabled     bool      `json:"enabled"`
}

type cacheEntry struct {
	info      *KeyInfo
	expiresAt time.Time
}

// Manager handles API key authentication
type Manager struct {
	db       *sql.DB
	cacheTTL time.Duration

	cache   map[string]cacheEntry
	cacheMu sync.RWMutex

	cleanupDone chan struct{}
	logger      zerolog.Logger
}

// NewManager creates a new auth manager using the metadata DB
func NewManager(db *sql.DB, logger zerolog.Logger) *Manager {
	m := &Manager{
		db:          db,
		cacheTTL:    5 * time.Minute,
		cache:       make(map[string]cacheEntry),
		cleanupDone: make(chan struct{}),
		logger:      logger.With().Str("component", "auth").Logger(),
	}

	go m.cleanupLoop()

	m.logger.Info().Msg("Auth manager initialized")
	return m
}

func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(m.cacheTTL)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.cleanupExpired()
		case <-m.cleanupDone:
			return
		}
	}
}

func (m *Manager) cleanupExpired() {
	now := time.Now()
	m.cacheMu.Lock()
	for key, entry := range m.cache {
		if now.After(entry.expiresAt) {
			delete(m.cache, key)
		}
	}
	m.cacheMu.Unlock()
}

func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func keyPrefix(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])[:16]
}

// GenerateKey creates a new API key with the mtk_ prefix
func GenerateKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "mtk_" + base64.URLEncoding.EncodeToString(b), nil
}

// CreateKey creates a new API key for an organization
func (m *Manager) CreateKey(name, orgID, permissions string) (string, error) {
	key, err := GenerateKey()
	if err != nil {
		return "", fmt.Errorf("failed to generate key: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash key: %w", err)
	}

	prefix := keyPrefix(key)

	if permissions == "" {
		permissions = "read,write"
	}

	_, err = m.db.Exec(`
		INSERT INTO api_keys (key_hash, key_prefix, name, org_id, permissions)
		VALUES (?, ?, ?, ?, ?)
	`, string(hash), prefix, name, orgID, permissions)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return "", fmt.Errorf("key with name '%s' already exists", name)
		}
		return "", fmt.Errorf("failed to create key: %w", err)
	}

	m.logger.Info().Str("name", name).Str("org_id", orgID).Msg("Created API key")
	return key, nil
}

// VerifyKey verifies an API key and returns key info if valid
func (m *Manager) VerifyKey(key string) *KeyInfo {
	if key == "" {
		return nil
	}

	cacheK := hashKey(key)
	now := time.Now()

	// Check cache
	m.cacheMu.RLock()
	if entry, ok := m.cache[cacheK]; ok && now.Before(entry.expiresAt) {
		m.cacheMu.RUnlock()
		return entry.info
	}
	m.cacheMu.RUnlock()

	prefix := keyPrefix(key)

	rows, err := m.db.Query(`
		SELECT id, name, key_hash, org_id, permissions, created_at, expires_at, enabled
		FROM api_keys
		WHERE enabled = 1 AND key_prefix = ?
	`, prefix)
	if err != nil {
		m.logger.Error().Err(err).Msg("Failed to query keys")
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id          int64
			name        string
			keyHash     string
			orgID       string
			permissions sql.NullString
			createdAt   time.Time
			expiresAt   sql.NullTime
			enabled     bool
		)

		if err := rows.Scan(&id, &name, &keyHash, &orgID, &permissions, &createdAt, &expiresAt, &enabled); err != nil {
			m.logger.Error().Err(err).Msg("Failed to scan key row")
			continue
		}

		if bcrypt.CompareHashAndPassword([]byte(keyHash), []byte(key)) != nil {
			continue
		}

		if expiresAt.Valid && now.After(expiresAt.Time) {
			return nil
		}

		info := &KeyInfo{
			ID:          id,
			Name:        name,
			OrgID:       orgID,
			Permissions: []string{},
			CreatedAt:   createdAt,
			Enabled:     enabled,
		}
		if permissions.Valid && permissions.String != "" {
			info.Permissions = strings.Split(permissions.String, ",")
		}
		if expiresAt.Valid {
			info.ExpiresAt = expiresAt.Time
		}

		// Cache it
		m.cacheMu.Lock()
		m.cache[cacheK] = cacheEntry{
			info:      info,
			expiresAt: now.Add(m.cacheTTL),
		}
		m.cacheMu.Unlock()

		return info
	}

	return nil
}

// HasPermission checks if a key has a specific permission
func HasPermission(info *KeyInfo, permission string) bool {
	if info == nil {
		return false
	}
	for _, p := range info.Permissions {
		if p == "admin" || p == permission {
			return true
		}
	}
	return false
}

// EnsureBootstrap creates the default org and admin key if none exist
func (m *Manager) EnsureBootstrap() (string, error) {
	var count int
	err := m.db.QueryRow("SELECT COUNT(*) FROM organizations").Scan(&count)
	if err != nil {
		return "", err
	}
	if count > 0 {
		return "", nil
	}

	m.logger.Info().Msg("First run detected — bootstrapping default organization and API key")

	orgID := "org_default"
	_, err = m.db.Exec(`INSERT INTO organizations (id, name) VALUES (?, ?)`, orgID, "Default")
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return "", nil
		}
		return "", fmt.Errorf("failed to create default org: %w", err)
	}

	key, err := m.CreateKey("admin", orgID, "read,write,admin")
	if err != nil {
		return "", fmt.Errorf("failed to create admin key: %w", err)
	}

	return key, nil
}

// ListKeys returns all keys for an org (without hashes)
func (m *Manager) ListKeys(orgID string) ([]KeyInfo, error) {
	rows, err := m.db.Query(`
		SELECT id, name, org_id, permissions, created_at, expires_at, enabled
		FROM api_keys WHERE org_id = ?
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []KeyInfo
	for rows.Next() {
		var (
			id          int64
			name        string
			org         string
			permissions sql.NullString
			createdAt   time.Time
			expiresAt   sql.NullTime
			enabled     bool
		)
		if err := rows.Scan(&id, &name, &org, &permissions, &createdAt, &expiresAt, &enabled); err != nil {
			return nil, err
		}
		info := KeyInfo{
			ID:        id,
			Name:      name,
			OrgID:     org,
			CreatedAt: createdAt,
			Enabled:   enabled,
		}
		if permissions.Valid && permissions.String != "" {
			info.Permissions = strings.Split(permissions.String, ",")
		}
		if expiresAt.Valid {
			info.ExpiresAt = expiresAt.Time
		}
		keys = append(keys, info)
	}
	return keys, nil
}

// DeleteKey deletes an API key
func (m *Manager) DeleteKey(id int64) error {
	result, err := m.db.Exec("DELETE FROM api_keys WHERE id = ?", id)
	if err != nil {
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.New("key not found")
	}
	m.invalidateCache()
	return nil
}

func (m *Manager) invalidateCache() {
	m.cacheMu.Lock()
	m.cache = make(map[string]cacheEntry)
	m.cacheMu.Unlock()
}

// Close stops the auth manager
func (m *Manager) Close() {
	close(m.cleanupDone)
}

package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Basekick-Labs/memtrace/internal/crypto"
)

// ErrArcInstanceNotFound is returned when no arc_instances row matches the lookup.
var ErrArcInstanceNotFound = errors.New("arc instance not found")

// ArcInstance describes the per-org Arc connection. APIKey is plaintext and is
// only populated by Get* methods (after decryption); Create/Update consume it,
// encrypt, and discard.
type ArcInstance struct {
	ID          string
	OrgID       string
	URL         string
	APIKey      string
	Database    string
	Measurement string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ArcInstanceStore wraps the arc_instances table with envelope encryption for
// the API key column.
type ArcInstanceStore struct {
	db     *sql.DB
	cipher *crypto.Cipher
}

// NewArcInstanceStore returns a store using the supplied DB handle and cipher.
func NewArcInstanceStore(db *sql.DB, cipher *crypto.Cipher) *ArcInstanceStore {
	return &ArcInstanceStore{db: db, cipher: cipher}
}

// Create inserts a new arc instance, encrypting the API key before write.
func (s *ArcInstanceStore) Create(ctx context.Context, inst *ArcInstance) error {
	if inst.ID == "" {
		inst.ID = GenerateID("arc_")
	}
	if inst.Measurement == "" {
		inst.Measurement = "events"
	}
	cipherBytes, nonce, err := s.cipher.Encrypt([]byte(inst.APIKey))
	if err != nil {
		return fmt.Errorf("failed to encrypt api key: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO arc_instances (id, org_id, url, api_key_cipher, api_key_nonce, database, measurement)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, inst.ID, inst.OrgID, inst.URL, cipherBytes, nonce, inst.Database, inst.Measurement)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return fmt.Errorf("org '%s' already has an arc instance configured", inst.OrgID)
		}
		return fmt.Errorf("failed to insert arc instance: %w", err)
	}
	return nil
}

// Update replaces the URL, API key, database, and measurement for an existing
// instance keyed by org_id.
func (s *ArcInstanceStore) Update(ctx context.Context, inst *ArcInstance) error {
	if inst.Measurement == "" {
		inst.Measurement = "events"
	}
	cipherBytes, nonce, err := s.cipher.Encrypt([]byte(inst.APIKey))
	if err != nil {
		return fmt.Errorf("failed to encrypt api key: %w", err)
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE arc_instances
		SET url = ?, api_key_cipher = ?, api_key_nonce = ?, database = ?, measurement = ?, updated_at = CURRENT_TIMESTAMP
		WHERE org_id = ?
	`, inst.URL, cipherBytes, nonce, inst.Database, inst.Measurement, inst.OrgID)
	if err != nil {
		return fmt.Errorf("failed to update arc instance: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrArcInstanceNotFound
	}
	return nil
}

// GetByOrg returns the (decrypted) Arc instance for an org or
// ErrArcInstanceNotFound if none is configured.
func (s *ArcInstanceStore) GetByOrg(ctx context.Context, orgID string) (*ArcInstance, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, org_id, url, api_key_cipher, api_key_nonce, database, measurement, created_at, updated_at
		FROM arc_instances WHERE org_id = ?
	`, orgID)
	return s.scan(row)
}

// List returns every configured instance, with API keys decrypted.
func (s *ArcInstanceStore) List(ctx context.Context) ([]*ArcInstance, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, org_id, url, api_key_cipher, api_key_nonce, database, measurement, created_at, updated_at
		FROM arc_instances ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*ArcInstance
	for rows.Next() {
		inst, err := s.scanRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inst)
	}
	return out, rows.Err()
}

// Delete removes the arc_instances row for an org. Returns
// ErrArcInstanceNotFound if no row matched.
func (s *ArcInstanceStore) Delete(ctx context.Context, orgID string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM arc_instances WHERE org_id = ?", orgID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrArcInstanceNotFound
	}
	return nil
}

// Count returns the total number of configured arc instances. Used by
// auto-migration to detect a fresh install.
func (s *ArcInstanceStore) Count(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM arc_instances").Scan(&n)
	return n, err
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func (s *ArcInstanceStore) scan(r scanner) (*ArcInstance, error) {
	var (
		inst        ArcInstance
		cipherBytes []byte
		nonce       []byte
	)
	err := r.Scan(&inst.ID, &inst.OrgID, &inst.URL, &cipherBytes, &nonce, &inst.Database, &inst.Measurement, &inst.CreatedAt, &inst.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrArcInstanceNotFound
		}
		return nil, err
	}
	plain, err := s.cipher.Decrypt(cipherBytes, nonce)
	if err != nil {
		return nil, fmt.Errorf("decrypt api key for org %s: %w", inst.OrgID, err)
	}
	inst.APIKey = string(plain)
	return &inst, nil
}

func (s *ArcInstanceStore) scanRows(rows *sql.Rows) (*ArcInstance, error) {
	return s.scan(rows)
}

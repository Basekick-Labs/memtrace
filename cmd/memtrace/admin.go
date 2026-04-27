package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/Basekick-Labs/memtrace/internal/config"
	memtracecrypto "github.com/Basekick-Labs/memtrace/internal/crypto"
	"github.com/Basekick-Labs/memtrace/internal/metadata"
	"github.com/rs/zerolog"
)

// adminContext bundles the metadata DB handle and cipher used by every admin
// subcommand. Callers must invoke close() when done.
type adminContext struct {
	cfg    *config.Config
	db     *metadata.DB
	store  *metadata.ArcInstanceStore
	cipher *memtracecrypto.Cipher
	logger zerolog.Logger
}

func (a *adminContext) close() {
	if a.db != nil {
		_ = a.db.Close()
	}
}

// loadAdminContext is shared setup for every admin subcommand: load config,
// open metadata DB, decode master key. requireCipher=false skips the master
// key check for commands that don't touch encrypted columns (e.g. `org list`).
func loadAdminContext(_ context.Context, requireCipher bool) (*adminContext, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).Level(zerolog.WarnLevel)

	db, err := metadata.New(cfg.Auth.DBPath, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to open metadata db: %w", err)
	}

	a := &adminContext{cfg: cfg, db: db, logger: logger}

	if requireCipher {
		cipher, err := memtracecrypto.NewCipherFromEnv()
		if err != nil {
			a.close()
			return nil, err
		}
		a.cipher = cipher
		a.store = metadata.NewArcInstanceStore(db.GetDB(), cipher)
	}
	return a, nil
}

// orgExists returns true if the org_id is present in the organizations table.
func orgExists(db *sql.DB, orgID string) (bool, error) {
	var n int
	err := db.QueryRow("SELECT COUNT(*) FROM organizations WHERE id = ?", orgID).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// errOrgNotFound is returned when an org_id has no matching organizations row.
var errOrgNotFound = errors.New("org not found")

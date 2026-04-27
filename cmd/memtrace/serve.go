package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Basekick-Labs/memtrace/internal/agent"
	"github.com/Basekick-Labs/memtrace/internal/api"
	arcClient "github.com/Basekick-Labs/memtrace/internal/arc"
	"github.com/Basekick-Labs/memtrace/internal/auth"
	"github.com/Basekick-Labs/memtrace/internal/config"
	memtracecrypto "github.com/Basekick-Labs/memtrace/internal/crypto"
	"github.com/Basekick-Labs/memtrace/internal/memory"
	"github.com/Basekick-Labs/memtrace/internal/metadata"
	"github.com/Basekick-Labs/memtrace/internal/session"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:           "serve",
	Short:         "Run the Memtrace HTTP server (default)",
	RunE:          runServe,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func runServe(cmd *cobra.Command, args []string) error {
	_ = cmd
	_ = args

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	logger := buildLogger(cfg)

	logger.Info().
		Int("port", cfg.Server.Port).
		Bool("auth_enabled", cfg.Auth.Enabled).
		Bool("dedup_enabled", cfg.Dedup.Enabled).
		Msg("Starting memtrace")

	cipher, err := memtracecrypto.NewCipherFromEnv()
	if err != nil {
		return err
	}

	metaDB, err := metadata.New(cfg.Auth.DBPath, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize metadata database: %w", err)
	}
	defer metaDB.Close()

	store := metadata.NewArcInstanceStore(metaDB.GetDB(), cipher)

	// Auto-migrate legacy [arc] block on first run.
	if err := autoMigrateLegacyArc(context.Background(), metaDB, store, cfg.LegacyArc, logger); err != nil {
		return fmt.Errorf("legacy arc migration failed: %w", err)
	}

	registry, err := arcClient.NewRegistry(context.Background(), store, arcClient.Defaults{
		ConnectTimeout:       cfg.Arc.ConnectTimeout,
		QueryTimeout:         cfg.Arc.QueryTimeout,
		WriteBatchSize:       cfg.Arc.WriteBatchSize,
		WriteFlushIntervalMS: cfg.Arc.WriteFlushIntervalMS,
	}, logger)
	if err != nil {
		return fmt.Errorf("failed to build arc registry: %w", err)
	}
	if len(registry.Orgs()) == 0 {
		logger.Warn().Msg("No Arc instances configured. Add one with `memtrace org add-arc`.")
	}

	var authManager *auth.Manager
	if cfg.Auth.Enabled {
		authManager = auth.NewManager(metaDB.GetDB(), logger)
		key, err := authManager.EnsureBootstrap()
		if err != nil {
			return fmt.Errorf("failed to bootstrap auth: %w", err)
		}
		if key != "" {
			logger.Info().Msg("==========================================================")
			logger.Info().Msg("  FIRST RUN: Save your admin API key (shown only once)")
			logger.Info().Msgf("  API Key: %s", key)
			logger.Info().Msg("==========================================================")
		}
	}

	var dedupEngine *memory.DedupEngine
	if cfg.Dedup.Enabled {
		dedupEngine = memory.NewDedupEngine(registry, cfg.Dedup.WindowHours, logger)
	}

	memoryManager := memory.NewManager(registry, dedupEngine, logger)
	agentManager := agent.NewManager(metaDB, registry, logger)
	sessionManager := session.NewManager(metaDB, registry, logger)

	server := api.NewServer(cfg, registry, authManager, memoryManager, agentManager, sessionManager, logger)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.Listen(); err != nil {
			logger.Fatal().Err(err).Msg("Server failed")
		}
	}()

	sig := <-sigCh
	logger.Info().Str("signal", sig.String()).Msg("Shutting down")

	if err := server.Shutdown(); err != nil {
		logger.Error().Err(err).Msg("Server shutdown error")
	}
	if err := registry.Close(); err != nil {
		logger.Error().Err(err).Msg("Arc registry close error")
	}
	if authManager != nil {
		authManager.Close()
	}

	logger.Info().Msg("memtrace stopped")
	return nil
}

func buildLogger(cfg *config.Config) zerolog.Logger {
	var logger zerolog.Logger
	if cfg.Log.Format == "json" {
		logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	} else {
		logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
	}
	level, err := zerolog.ParseLevel(cfg.Log.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
	return logger
}

// autoMigrateLegacyArc inserts an arc_instances row for org_default when the
// legacy flat [arc] block has a URL and no instances are yet configured. Runs
// at most once per deployment; idempotent on re-run.
func autoMigrateLegacyArc(
	ctx context.Context,
	metaDB *metadata.DB,
	store *metadata.ArcInstanceStore,
	legacy config.LegacyArcConfig,
	logger zerolog.Logger,
) error {
	if legacy.URL == "" {
		return nil
	}
	count, err := store.Count(ctx)
	if err != nil {
		return fmt.Errorf("count arc_instances: %w", err)
	}
	if count > 0 {
		return nil
	}

	logger.Warn().Str("url", legacy.URL).Msg("legacy [arc] config detected — migrating to DB")

	const orgID = "org_default"
	exists, err := orgExists(metaDB.GetDB(), orgID)
	if err != nil {
		return fmt.Errorf("check default org: %w", err)
	}
	if !exists {
		if _, err := metaDB.GetDB().ExecContext(ctx,
			`INSERT INTO organizations (id, name) VALUES (?, ?)`, orgID, "Default"); err != nil {
			return fmt.Errorf("create default org: %w", err)
		}
	}

	measurement := legacy.Measurement
	if measurement == "" {
		measurement = "events"
	}
	if err := store.Create(ctx, &metadata.ArcInstance{
		ID:          "arc_default",
		OrgID:       orgID,
		URL:         legacy.URL,
		APIKey:      legacy.APIKey,
		Database:    legacy.Database,
		Measurement: measurement,
	}); err != nil {
		return fmt.Errorf("insert default arc instance: %w", err)
	}

	logger.Info().Msg("migration complete; remove [arc] url/api_key/database/measurement from memtrace.toml on next deploy")
	return nil
}

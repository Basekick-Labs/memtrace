package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Basekick-Labs/memtrace/internal/agent"
	"github.com/Basekick-Labs/memtrace/internal/api"
	arcClient "github.com/Basekick-Labs/memtrace/internal/arc"
	"github.com/Basekick-Labs/memtrace/internal/auth"
	"github.com/Basekick-Labs/memtrace/internal/config"
	"github.com/Basekick-Labs/memtrace/internal/memory"
	"github.com/Basekick-Labs/memtrace/internal/metadata"
	"github.com/Basekick-Labs/memtrace/internal/session"
	"github.com/rs/zerolog"
)

func main() {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Setup logger
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

	logger.Info().
		Int("port", cfg.Server.Port).
		Str("arc_url", cfg.Arc.URL).
		Str("arc_database", cfg.Arc.Database).
		Bool("auth_enabled", cfg.Auth.Enabled).
		Bool("dedup_enabled", cfg.Dedup.Enabled).
		Msg("Starting memtrace")

	// Initialize Arc client
	arcCli := arcClient.NewClient(
		cfg.Arc.URL,
		cfg.Arc.APIKey,
		cfg.Arc.Database,
		cfg.Arc.Measurement,
		cfg.Arc.WriteBatchSize,
		cfg.Arc.WriteFlushIntervalMS,
		cfg.Arc.ConnectTimeout,
		cfg.Arc.QueryTimeout,
		logger,
	)

	// Verify Arc connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := arcCli.Ping(ctx); err != nil {
		logger.Fatal().Err(err).Str("arc_url", cfg.Arc.URL).Msg("Failed to connect to Arc")
	}
	arcCli.MarkConnected()
	logger.Info().Str("arc_url", cfg.Arc.URL).Msg("Arc connection verified")

	// Initialize metadata DB
	metaDB, err := metadata.New(cfg.Auth.DBPath, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize metadata database")
	}

	// Initialize auth manager
	var authManager *auth.Manager
	if cfg.Auth.Enabled {
		authManager = auth.NewManager(metaDB.GetDB(), logger)

		// Bootstrap default org + API key on first run
		key, err := authManager.EnsureBootstrap()
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to bootstrap auth")
		}
		if key != "" {
			logger.Info().Msg("==========================================================")
			logger.Info().Msg("  FIRST RUN: Save your admin API key (shown only once)")
			logger.Info().Msgf("  API Key: %s", key)
			logger.Info().Msg("==========================================================")
		}
	}

	// Initialize dedup engine
	var dedupEngine *memory.DedupEngine
	if cfg.Dedup.Enabled {
		dedupEngine = memory.NewDedupEngine(arcCli, cfg.Dedup.WindowHours, logger)
	}

	// Initialize managers
	memoryManager := memory.NewManager(arcCli, dedupEngine, logger)
	agentManager := agent.NewManager(metaDB, arcCli, logger)
	sessionManager := session.NewManager(metaDB, arcCli, logger)

	// Create and start server
	server := api.NewServer(cfg, arcCli, authManager, memoryManager, agentManager, sessionManager, logger)

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.Listen(); err != nil {
			logger.Fatal().Err(err).Msg("Server failed")
		}
	}()

	sig := <-sigCh
	logger.Info().Str("signal", sig.String()).Msg("Shutting down")

	// Cleanup
	if err := server.Shutdown(); err != nil {
		logger.Error().Err(err).Msg("Server shutdown error")
	}
	if err := arcCli.Close(); err != nil {
		logger.Error().Err(err).Msg("Arc client close error")
	}
	if authManager != nil {
		authManager.Close()
	}
	if err := metaDB.Close(); err != nil {
		logger.Error().Err(err).Msg("Metadata DB close error")
	}

	logger.Info().Msg("memtrace stopped")
}

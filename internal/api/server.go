package api

import (
	"fmt"
	"time"

	"github.com/Basekick-Labs/memtrace/internal/agent"
	arcClient "github.com/Basekick-Labs/memtrace/internal/arc"
	"github.com/Basekick-Labs/memtrace/internal/auth"
	"github.com/Basekick-Labs/memtrace/internal/config"
	"github.com/Basekick-Labs/memtrace/internal/memory"
	"github.com/Basekick-Labs/memtrace/internal/session"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/rs/zerolog"
)

// Server wraps the Fiber app
type Server struct {
	app    *fiber.App
	cfg    *config.Config
	logger zerolog.Logger
}

// NewServer creates and configures the HTTP server
func NewServer(
	cfg *config.Config,
	arcCli *arcClient.Client,
	authManager *auth.Manager,
	memoryManager *memory.Manager,
	agentManager *agent.Manager,
	sessionManager *session.Manager,
	logger zerolog.Logger,
) *Server {
	app := fiber.New(fiber.Config{
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		AppName:      "memtrace",
	})

	// Middleware
	app.Use(recover.New())
	app.Use(cors.New())

	// Request logging
	app.Use(func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		logger.Info().
			Str("method", c.Method()).
			Str("path", c.Path()).
			Int("status", c.Response().StatusCode()).
			Dur("latency", time.Since(start)).
			Msg("request")
		return err
	})

	// Health endpoints (no auth)
	healthHandler := NewHealthHandler(arcCli)
	healthHandler.RegisterRoutes(app)

	// API group with auth
	apiGroup := app.Group("/api/v1")
	if authManager != nil && cfg.Auth.Enabled {
		apiGroup.Use(auth.RequireAuth(authManager))
	}

	// Register handlers
	memoryHandler := NewMemoryHandler(memoryManager, logger)
	memoryHandler.RegisterRoutes(apiGroup)

	agentHandler := NewAgentHandler(agentManager, memoryManager, logger)
	agentHandler.RegisterRoutes(apiGroup)

	sessionHandler := NewSessionHandler(sessionManager, memoryManager, logger)
	sessionHandler.RegisterRoutes(apiGroup)

	searchHandler := NewSearchHandler(memoryManager, logger)
	searchHandler.RegisterRoutes(apiGroup)

	return &Server{
		app:    app,
		cfg:    cfg,
		logger: logger,
	}
}

// Listen starts the server
func (s *Server) Listen() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	s.logger.Info().Str("addr", addr).Msg("Starting memtrace server")
	return s.app.Listen(addr)
}

// Shutdown gracefully stops the server
func (s *Server) Shutdown() error {
	return s.app.Shutdown()
}

package api

import (
	"context"
	"time"

	"github.com/Basekick-Labs/memtrace/internal/arc"
	"github.com/gofiber/fiber/v2"
)

// HealthHandler handles health and readiness endpoints
type HealthHandler struct {
	arcClient *arc.Client
	startTime time.Time
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(arcClient *arc.Client) *HealthHandler {
	return &HealthHandler{
		arcClient: arcClient,
		startTime: time.Now(),
	}
}

// RegisterRoutes registers health routes (no auth required)
func (h *HealthHandler) RegisterRoutes(app *fiber.App) {
	app.Get("/health", h.handleHealth)
	app.Get("/ready", h.handleReady)
}

func (h *HealthHandler) handleHealth(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":  "ok",
		"service": "memtrace",
		"uptime":  time.Since(h.startTime).String(),
	})
}

func (h *HealthHandler) handleReady(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	if err := h.arcClient.Ping(ctx); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "not ready",
			"error":  "Arc is unreachable: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"status": "ready",
		"arc":    "connected",
	})
}

package api

import (
	"time"

	"github.com/Basekick-Labs/memtrace/internal/arc"
	"github.com/gofiber/fiber/v2"
)

// HealthHandler handles health and readiness endpoints
type HealthHandler struct {
	arcRegistry *arc.Registry
	startTime   time.Time
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(arcRegistry *arc.Registry) *HealthHandler {
	return &HealthHandler{
		arcRegistry: arcRegistry,
		startTime:   time.Now(),
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
		"arc":     h.arcRegistry.Health(),
	})
}

// handleReady returns 200 only when every configured Arc instance is reachable.
// With no instances configured, it returns 503 with a "no_instances" hint so
// operators notice the deployment isn't usable yet.
func (h *HealthHandler) handleReady(c *fiber.Ctx) error {
	health := h.arcRegistry.Health()
	if len(health) == 0 {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "not ready",
			"reason": "no_instances",
			"hint":   "configure an Arc instance with `memtrace org add-arc`",
		})
	}

	allHealthy := true
	for _, ok := range health {
		if !ok {
			allHealthy = false
			break
		}
	}
	if !allHealthy {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "not ready",
			"arc":    health,
		})
	}

	return c.JSON(fiber.Map{
		"status": "ready",
		"arc":    health,
	})
}

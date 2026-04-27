package api

import (
	"errors"

	"github.com/Basekick-Labs/memtrace/internal/arc"
	"github.com/gofiber/fiber/v2"
)

// writeError returns a JSON error response, mapping known sentinel errors to
// the right HTTP status. Use this at every handler call-site that surfaces an
// error from a manager.
func writeError(c *fiber.Ctx, err error) error {
	if errors.Is(err, arc.ErrNoArcInstance) {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "no arc instance configured for this org",
			"hint":  "ask an admin to run `memtrace org add-arc <org_id>`",
		})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error": err.Error(),
	})
}

package api

import (
	"github.com/Basekick-Labs/memtrace/internal/auth"
	"github.com/Basekick-Labs/memtrace/internal/memory"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

// SearchHandler handles search endpoints
type SearchHandler struct {
	manager *memory.Manager
	logger  zerolog.Logger
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(manager *memory.Manager, logger zerolog.Logger) *SearchHandler {
	return &SearchHandler{
		manager: manager,
		logger:  logger.With().Str("component", "search-handler").Logger(),
	}
}

// RegisterRoutes registers search routes
func (h *SearchHandler) RegisterRoutes(group fiber.Router) {
	group.Post("/search", h.handleSearch)
}

func (h *SearchHandler) handleSearch(c *fiber.Ctx) error {
	orgID := auth.GetOrgID(c)

	var query memory.SearchQuery
	if err := c.BodyParser(&query); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body: " + err.Error(),
		})
	}

	result, err := h.manager.Search(c.Context(), orgID, &query)
	if err != nil {
		return writeError(c, err)
	}

	h.logger.Info().
		Str("org_id", orgID).
		Str("agent_id", query.AgentID).
		Int("count", result.Count).
		Msg("Memory search")

	return c.JSON(result)
}

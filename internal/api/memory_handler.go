package api

import (
	"errors"

	"github.com/Basekick-Labs/memtrace/internal/arc"
	"github.com/Basekick-Labs/memtrace/internal/auth"
	"github.com/Basekick-Labs/memtrace/internal/memory"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

// MemoryHandler handles memory CRUD endpoints
type MemoryHandler struct {
	manager *memory.Manager
	logger  zerolog.Logger
}

// NewMemoryHandler creates a new memory handler
func NewMemoryHandler(manager *memory.Manager, logger zerolog.Logger) *MemoryHandler {
	return &MemoryHandler{
		manager: manager,
		logger:  logger.With().Str("component", "memory-handler").Logger(),
	}
}

// RegisterRoutes registers memory routes
func (h *MemoryHandler) RegisterRoutes(group fiber.Router) {
	group.Post("/memories", h.handleCreate)
	group.Get("/memories", h.handleList)
	group.Get("/memories/:id", h.handleGet)
}

func (h *MemoryHandler) handleCreate(c *fiber.Ctx) error {
	orgID := auth.GetOrgID(c)

	// Try batch first
	var batch memory.BatchCreateRequest
	if err := c.BodyParser(&batch); err == nil && len(batch.Memories) > 0 {
		results, err := h.manager.CreateBatch(c.Context(), orgID, batch.Memories)
		if err != nil {
			return writeError(c, err)
		}
		h.logger.Info().Str("org_id", orgID).Int("count", len(results)).Msg("Batch memory create")
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"memories": results,
			"count":    len(results),
		})
	}

	// Single create
	var req memory.CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body: " + err.Error(),
		})
	}

	mem, err := h.manager.Create(c.Context(), orgID, &req)
	if err != nil {
		if err == memory.ErrDuplicate {
			h.logger.Debug().Str("org_id", orgID).Str("agent_id", req.AgentID).Msg("Duplicate memory rejected")
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "duplicate memory",
			})
		}
		if errors.Is(err, arc.ErrNoArcInstance) {
			return writeError(c, err)
		}
		h.logger.Warn().Str("org_id", orgID).Str("agent_id", req.AgentID).Err(err).Msg("Memory create rejected")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	h.logger.Info().
		Str("org_id", orgID).
		Str("agent_id", req.AgentID).
		Str("memory_type", req.MemoryType).
		Str("event_type", req.EventType).
		Msg("Memory created")

	return c.Status(fiber.StatusCreated).JSON(mem)
}

func (h *MemoryHandler) handleList(c *fiber.Ctx) error {
	orgID := auth.GetOrgID(c)

	opts := &memory.ListOptions{
		AgentID:    c.Query("agent_id"),
		SessionID:  c.Query("session_id"),
		MemoryType: c.Query("memory_type"),
		EventType:  c.Query("event_type"),
		Tags:       c.Query("tags"),
		Since:      c.Query("since"),
		Until:      c.Query("until"),
		Limit:      c.QueryInt("limit", 100),
		Offset:     c.QueryInt("offset", 0),
		Order:      c.Query("order", "desc"),
	}

	list, err := h.manager.List(c.Context(), orgID, opts)
	if err != nil {
		return writeError(c, err)
	}

	h.logger.Info().
		Str("org_id", orgID).
		Str("agent_id", opts.AgentID).
		Int("count", list.Count).
		Msg("Memory list")

	return c.JSON(list)
}

func (h *MemoryHandler) handleGet(c *fiber.Ctx) error {
	orgID := auth.GetOrgID(c)
	dedupKey := c.Params("id")

	result, err := h.manager.Search(c.Context(), orgID, &memory.SearchQuery{
		ContentContains: dedupKey,
		Limit:           1,
	})
	if err != nil {
		return writeError(c, err)
	}
	if result.Count == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "memory not found",
		})
	}

	return c.JSON(result.Results[0])
}

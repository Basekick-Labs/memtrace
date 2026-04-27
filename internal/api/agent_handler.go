package api

import (
	"strings"

	"github.com/Basekick-Labs/memtrace/internal/agent"
	"github.com/Basekick-Labs/memtrace/internal/auth"
	"github.com/Basekick-Labs/memtrace/internal/memory"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

// AgentHandler handles agent endpoints
type AgentHandler struct {
	agentManager  *agent.Manager
	memoryManager *memory.Manager
	logger        zerolog.Logger
}

// NewAgentHandler creates a new agent handler
func NewAgentHandler(agentManager *agent.Manager, memoryManager *memory.Manager, logger zerolog.Logger) *AgentHandler {
	return &AgentHandler{
		agentManager:  agentManager,
		memoryManager: memoryManager,
		logger:        logger.With().Str("component", "agent-handler").Logger(),
	}
}

// RegisterRoutes registers agent routes
func (h *AgentHandler) RegisterRoutes(group fiber.Router) {
	group.Post("/agents", h.handleCreate)
	group.Get("/agents", h.handleList)
	group.Get("/agents/:id", h.handleGet)
	group.Get("/agents/:id/memories", h.handleMemories)
	group.Get("/agents/:id/stats", h.handleStats)
	group.Delete("/agents/:id", h.handleDelete)
}

func (h *AgentHandler) handleCreate(c *fiber.Ctx) error {
	orgID := auth.GetOrgID(c)

	var req agent.CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body: " + err.Error(),
		})
	}

	a, err := h.agentManager.Create(c.Context(), orgID, &req)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	h.logger.Info().Str("org_id", orgID).Str("agent_id", a.ID).Str("name", req.Name).Msg("Agent registered")

	return c.Status(fiber.StatusCreated).JSON(a)
}

func (h *AgentHandler) handleList(c *fiber.Ctx) error {
	orgID := auth.GetOrgID(c)

	agents, err := h.agentManager.List(c.Context(), orgID)
	if err != nil {
		return writeError(c, err)
	}

	if agents == nil {
		agents = []*agent.Agent{}
	}

	return c.JSON(fiber.Map{
		"agents": agents,
		"count":  len(agents),
	})
}

func (h *AgentHandler) handleGet(c *fiber.Ctx) error {
	orgID := auth.GetOrgID(c)
	agentID := c.Params("id")

	a, err := h.agentManager.Get(c.Context(), orgID, agentID)
	if err != nil {
		return writeError(c, err)
	}
	if a == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "agent not found",
		})
	}

	return c.JSON(a)
}

func (h *AgentHandler) handleMemories(c *fiber.Ctx) error {
	orgID := auth.GetOrgID(c)
	agentID := c.Params("id")

	opts := &memory.ListOptions{
		AgentID:    agentID,
		MemoryType: c.Query("memory_type"),
		EventType:  c.Query("event_type"),
		Since:      c.Query("since", "24h"),
		Limit:      c.QueryInt("limit", 100),
		Order:      c.Query("order", "desc"),
	}

	list, err := h.memoryManager.List(c.Context(), orgID, opts)
	if err != nil {
		return writeError(c, err)
	}

	return c.JSON(list)
}

func (h *AgentHandler) handleStats(c *fiber.Ctx) error {
	orgID := auth.GetOrgID(c)
	agentID := c.Params("id")

	stats, err := h.agentManager.GetStats(c.Context(), orgID, agentID)
	if err != nil {
		return writeError(c, err)
	}

	return c.JSON(stats)
}

func (h *AgentHandler) handleDelete(c *fiber.Ctx) error {
	orgID := auth.GetOrgID(c)
	agentID := c.Params("id")

	if err := h.agentManager.Delete(c.Context(), orgID, agentID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	h.logger.Info().Str("org_id", orgID).Str("agent_id", agentID).Msg("Agent deleted")

	return c.SendStatus(fiber.StatusNoContent)
}

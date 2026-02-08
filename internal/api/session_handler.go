package api

import (
	"github.com/Basekick-Labs/memtrace/internal/auth"
	"github.com/Basekick-Labs/memtrace/internal/memory"
	"github.com/Basekick-Labs/memtrace/internal/session"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

// SessionHandler handles session endpoints
type SessionHandler struct {
	sessionManager *session.Manager
	memoryManager  *memory.Manager
	logger         zerolog.Logger
}

// NewSessionHandler creates a new session handler
func NewSessionHandler(sessionManager *session.Manager, memoryManager *memory.Manager, logger zerolog.Logger) *SessionHandler {
	return &SessionHandler{
		sessionManager: sessionManager,
		memoryManager:  memoryManager,
		logger:         logger.With().Str("component", "session-handler").Logger(),
	}
}

// RegisterRoutes registers session routes
func (h *SessionHandler) RegisterRoutes(group fiber.Router) {
	group.Post("/sessions", h.handleCreate)
	group.Get("/sessions", h.handleList)
	group.Get("/sessions/:id", h.handleGet)
	group.Put("/sessions/:id", h.handleUpdate)
	group.Get("/sessions/:id/memories", h.handleMemories)
	group.Post("/sessions/:id/context", h.handleContext)
}

func (h *SessionHandler) handleCreate(c *fiber.Ctx) error {
	orgID := auth.GetOrgID(c)

	var req session.CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body: " + err.Error(),
		})
	}

	s, err := h.sessionManager.Create(c.Context(), orgID, &req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	h.logger.Info().Str("org_id", orgID).Str("session_id", s.ID).Str("agent_id", req.AgentID).Msg("Session created")

	return c.Status(fiber.StatusCreated).JSON(s)
}

func (h *SessionHandler) handleList(c *fiber.Ctx) error {
	orgID := auth.GetOrgID(c)
	agentID := c.Query("agent_id")

	sessions, err := h.sessionManager.List(c.Context(), orgID, agentID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if sessions == nil {
		sessions = []*session.Session{}
	}

	return c.JSON(fiber.Map{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

func (h *SessionHandler) handleGet(c *fiber.Ctx) error {
	orgID := auth.GetOrgID(c)
	sessionID := c.Params("id")

	s, err := h.sessionManager.Get(c.Context(), orgID, sessionID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	if s == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "session not found",
		})
	}

	return c.JSON(s)
}

func (h *SessionHandler) handleUpdate(c *fiber.Ctx) error {
	orgID := auth.GetOrgID(c)
	sessionID := c.Params("id")

	var req session.UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body: " + err.Error(),
		})
	}

	s, err := h.sessionManager.Update(c.Context(), orgID, sessionID, &req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(s)
}

func (h *SessionHandler) handleMemories(c *fiber.Ctx) error {
	orgID := auth.GetOrgID(c)
	sessionID := c.Params("id")

	opts := &memory.ListOptions{
		SessionID:  sessionID,
		MemoryType: c.Query("memory_type"),
		Since:      c.Query("since"),
		Limit:      c.QueryInt("limit", 100),
		Order:      c.Query("order", "desc"),
	}

	list, err := h.memoryManager.List(c.Context(), orgID, opts)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(list)
}

func (h *SessionHandler) handleContext(c *fiber.Ctx) error {
	orgID := auth.GetOrgID(c)
	sessionID := c.Params("id")

	var opts session.ContextOptions
	if err := c.BodyParser(&opts); err != nil {
		// Use defaults
		opts = session.ContextOptions{}
	}

	ctx, err := h.sessionManager.GetContext(c.Context(), orgID, sessionID, &opts)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	h.logger.Info().Str("org_id", orgID).Str("session_id", sessionID).Int("memory_count", ctx.MemoryCount).Msg("Session context loaded")

	return c.JSON(ctx)
}

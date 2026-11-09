package auth

import (
	"github.com/gofiber/fiber/v2"
)

// RequireAuth returns Fiber middleware that validates API keys
func RequireAuth(m *Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		key := c.Get("x-api-key")
		if key == "" {
			key = c.Get("Authorization")
			if len(key) > 7 && key[:7] == "Bearer " {
				key = key[7:]
			}
		}

		if key == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing API key",
			})
		}

		info := m.VerifyKey(key)
		if info == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid API key",
			})
		}

		// Store key info in context
		c.Locals("key_info", info)
		c.Locals("org_id", info.OrgID)

		return c.Next()
	}
}

// GetKeyInfo extracts the key info from the Fiber context
func GetKeyInfo(c *fiber.Ctx) *KeyInfo {
	info, ok := c.Locals("key_info").(*KeyInfo)
	if !ok {
		return nil
	}
	return info
}

// GetOrgID extracts the org_id from the Fiber context
func GetOrgID(c *fiber.Ctx) string {
	orgID, ok := c.Locals("org_id").(string)
	if !ok {
		return ""
	}
	return orgID
}

// RequirePermission enforces the permission associated with an API request.
// Safe/read-only HTTP methods require "read"; all other methods require "write".
// Admin keys are accepted by HasPermission for both classes of operation.
func RequirePermission() fiber.Handler {
	return func(c *fiber.Ctx) error {
		info := GetKeyInfo(c)
		// Authentication can be disabled for local deployments. In that mode
		// RequireAuth is not installed and permission checks must not interfere.
		if info == nil {
			return c.Next()
		}

		permission := "write"
		switch c.Method() {
		case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
			permission = "read"
		}

		if !HasPermission(info, permission) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "insufficient API key permissions",
			})
		}
		return c.Next()
	}
}

package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// AdminAuth creates middleware to protect admin endpoints with token authentication
func AdminAuth(adminToken string) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Skip authentication if no token is configured (with warning logged at startup)
		if adminToken == "" {
			return c.Next()
		}

		// Check for Authorization: Bearer <token>
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
				providedToken := parts[1]
				if constantTimeCompare(providedToken, adminToken) {
					return c.Next()
				}
			}
		}

		// Check for X-Admin-Token header
		tokenHeader := c.Get("X-Admin-Token")
		if tokenHeader != "" {
			if constantTimeCompare(tokenHeader, adminToken) {
				return c.Next()
			}
		}

		// No valid token found
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   "Unauthorized",
			"message": "Valid admin token required. Use 'Authorization: Bearer <token>' or 'X-Admin-Token: <token>' header.",
		})
	}
}

// WebhookAuth creates middleware to validate webhook signatures using HMAC-SHA256
func WebhookAuth(webhookSecret string) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Skip authentication if no secret is configured (with warning logged at startup)
		if webhookSecret == "" {
			return c.Next()
		}

		// Get signature from header
		signature := c.Get("X-Hub-Signature-256")
		if signature == "" {
			// Also check for X-Emby-Token as fallback
			signature = c.Get("X-Emby-Token")
			if signature == "" {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error":   "Unauthorized",
					"message": "Webhook signature required in X-Hub-Signature-256 header",
				})
			}
		}

		// Get request body
		body := c.Body()

		// Calculate expected signature
		h := hmac.New(sha256.New, []byte(webhookSecret))
		h.Write(body)
		expectedSignature := "sha256=" + hex.EncodeToString(h.Sum(nil))

		// Verify signature using constant time comparison
		if !constantTimeCompare(signature, expectedSignature) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":   "Forbidden",
				"message": "Invalid webhook signature",
			})
		}

		return c.Next()
	}
}

// constantTimeCompare performs constant-time string comparison to prevent timing attacks
func constantTimeCompare(a, b string) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
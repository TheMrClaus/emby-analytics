package now

import (
	"emby-analytics/internal/emby"

	"github.com/gofiber/fiber/v3"
)

// PauseSession handles POST /now/:sessionId/pause
func PauseSession(em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {

		sessionID := c.Params("sessionId")
		if sessionID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Missing session ID"})
		}

		// Parse request body to see if it's pause or unpause
		var body struct {
			Paused bool `json:"paused"`
		}

		if err := c.Bind().JSON(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON body"})
		}

		var err error
		if body.Paused {
			err = em.Pause(sessionID)
		} else {
			err = em.Unpause(sessionID)
		}

		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"success": true, "action": map[bool]string{true: "paused", false: "resumed"}[body.Paused]})
	}
}

// StopSession handles POST /now/:sessionId/stop
func StopSession(em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {

		sessionID := c.Params("sessionId")
		if sessionID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Missing session ID"})
		}

		if err := em.Stop(sessionID); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"success": true, "action": "stopped"})
	}
}

// MessageSession handles POST /now/:sessionId/message
func MessageSession(em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {

		sessionID := c.Params("sessionId")
		if sessionID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Missing session ID"})
		}

		var body struct {
			Header    string `json:"header"`
			Text      string `json:"text"`
			TimeoutMs int    `json:"timeout_ms"`
		}

		if err := c.Bind().JSON(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON body"})
		}

		// Set defaults
		if body.Header == "" {
			body.Header = "Message"
		}
		if body.Text == "" {
			body.Text = "Hello from Emby Analytics"
		}
		if body.TimeoutMs <= 0 {
			body.TimeoutMs = 5000
		}

		if err := em.SendMessage(sessionID, body.Header, body.Text, body.TimeoutMs); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"success": true, "action": "message_sent", "header": body.Header, "text": body.Text})
	}
}

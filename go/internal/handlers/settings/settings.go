package settings

import (
	"database/sql"
	"emby-analytics/internal/logging"
	"time"

	"github.com/gofiber/fiber/v3"
)

type Setting struct {
	Key       string `json:"key" db:"key"`
	Value     string `json:"value" db:"value"`
	UpdatedAt string `json:"updated_at" db:"updated_at"`
}

type UpdateSettingRequest struct {
	Value string `json:"value"`
}

// GetSettings returns all application settings
func GetSettings(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		rows, err := db.Query("SELECT key, value, updated_at FROM app_settings ORDER BY key")
		if err != nil {
			logging.Debug("Error querying settings: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch settings"})
		}
		defer rows.Close()

		var settings []Setting
		for rows.Next() {
			var s Setting
			if err := rows.Scan(&s.Key, &s.Value, &s.UpdatedAt); err != nil {
				logging.Debug("Error scanning setting: %v", err)
				continue
			}
			settings = append(settings, s)
		}

		return c.JSON(settings)
	}
}

// UpdateSetting updates a specific setting value
func UpdateSetting(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		key := c.Params("key")
		if key == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Setting key is required"})
		}

		var req UpdateSettingRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
		}

		// Validate the setting key and value
		if !isValidSetting(key, req.Value) {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid setting key or value"})
		}

		// Update or insert the setting
		_, err := db.Exec(`
			INSERT INTO app_settings (key, value, updated_at) 
			VALUES (?, ?, ?)
			ON CONFLICT(key) DO UPDATE SET 
				value = excluded.value,
				updated_at = excluded.updated_at
		`, key, req.Value, time.Now().UTC())

		if err != nil {
			logging.Debug("Error updating setting %s: %v", key, err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to update setting"})
		}

		logging.Debug("Updated setting: %s = %s", key, req.Value)
		return c.JSON(fiber.Map{"success": true, "key": key, "value": req.Value})
	}
}

// Helper function to validate setting keys and values
func isValidSetting(key, value string) bool {
	switch key {
	case "include_trakt_items":
		return value == "true" || value == "false"
	default:
		return false // Only allow known settings
	}
}

// Helper function to get a setting value with a default
func GetSettingValue(db *sql.DB, key, defaultValue string) string {
	var value string
	err := db.QueryRow("SELECT value FROM app_settings WHERE key = ?", key).Scan(&value)
	if err != nil {
		if err != sql.ErrNoRows {
			logging.Debug("Error getting setting %s: %v", key, err)
		}
		return defaultValue
	}
	return value
}

// Helper function to get a boolean setting value
func GetSettingBool(db *sql.DB, key string, defaultValue bool) bool {
	value := GetSettingValue(db, key, "")
	switch value {
	case "true":
		return true
	case "false":
		return false
	default:
		return defaultValue
	}
}

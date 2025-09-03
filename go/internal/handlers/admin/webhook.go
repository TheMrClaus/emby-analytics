package admin

import (
	"database/sql"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"emby-analytics/internal/emby"
)

// EmbyWebhookPayload represents the structure of webhook data from Emby
type EmbyWebhookPayload struct {
	Server      ServerInfo `json:"Server"`
	Event       string     `json:"Event"`
	User        UserInfo   `json:"User,omitempty"`
	Item        ItemInfo   `json:"Item,omitempty"`
	Timestamp   string     `json:"Timestamp"`
}

type ServerInfo struct {
	Id   string `json:"Id"`
	Name string `json:"Name"`
}

type UserInfo struct {
	Id   string `json:"Id"`
	Name string `json:"Name"`
}

type ItemInfo struct {
	Id       string `json:"Id"`
	Name     string `json:"Name"`
	Type     string `json:"Type"`
	ParentId string `json:"ParentId,omitempty"`
}

// WebhookHandler handles incoming webhooks from Emby
func WebhookHandler(rm *RefreshManager, db *sql.DB, em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Parse webhook payload
		var payload EmbyWebhookPayload
		if err := c.BodyParser(&payload); err != nil {
			log.Printf("[webhook] Failed to parse webhook payload: %v", err)
			return c.Status(400).JSON(fiber.Map{"error": "Invalid payload"})
		}

		log.Printf("[webhook] 📨 Received event: %s for item: %s (%s)", payload.Event, payload.Item.Name, payload.Item.Type)

		// Handle library-related events
		if isLibraryEvent(payload.Event) {
			// Check if this is a media item we care about
			if isMediaItem(payload.Item.Type) {
				log.Printf("[webhook] 📚 Library change detected: %s - %s (%s)", payload.Event, payload.Item.Name, payload.Item.Type)
				
				// Trigger incremental sync
				go func() {
					log.Printf("[webhook] 🔄 Triggering incremental sync due to library change")
					rm.StartIncremental(db, em)
				}()
			}
		}

		return c.JSON(fiber.Map{"status": "received", "event": payload.Event})
	}
}

// isLibraryEvent determines if a webhook event is library-related
func isLibraryEvent(event string) bool {
	libraryEvents := []string{
		"library.new",
		"item.added",
		"item.updated",
		"item.removed",
		"media.scan",
		"library.refresh",
	}
	
	eventLower := strings.ToLower(event)
	for _, libEvent := range libraryEvents {
		if strings.Contains(eventLower, libEvent) || strings.Contains(libEvent, eventLower) {
			return true
		}
	}
	
	return false
}

// isMediaItem determines if an item type is a media item we track
func isMediaItem(itemType string) bool {
	mediaTypes := []string{
		"Movie",
		"Episode",
		"Video",
	}
	
	for _, mediaType := range mediaTypes {
		if strings.EqualFold(itemType, mediaType) {
			return true
		}
	}
	
	return false
}

// GetWebhookStats returns webhook activity statistics
func GetWebhookStats() fiber.Handler {
	return func(c fiber.Ctx) error {
		// For now, return basic info
		// In the future, we could track webhook statistics in the database
		return c.JSON(fiber.Map{
			"webhook_endpoint": "/admin/webhook/emby",
			"supported_events": []string{
				"library.new",
				"item.added", 
				"item.updated",
				"item.removed",
				"media.scan",
				"library.refresh",
			},
			"supported_item_types": []string{
				"Movie",
				"Episode",
				"Video",
			},
			"status": "active",
		})
	}
}
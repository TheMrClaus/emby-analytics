package images

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"

	"emby-analytics/internal/config"
)

type Opts struct {
	BaseURL          string
	APIKey           string
	Quality          int
	PrimaryMaxWidth  int
	BackdropMaxWidth int
	HTTPClient       *http.Client
}

func NewOpts(cfg config.Config) Opts {
	return Opts{
		BaseURL:          cfg.EmbyBaseURL,
		APIKey:           cfg.EmbyAPIKey,
		Quality:          cfg.ImgQuality,
		PrimaryMaxWidth:  cfg.ImgPrimaryMaxWidth,
		BackdropMaxWidth: cfg.ImgBackdropMaxWidth,
		HTTPClient:       &http.Client{Timeout: 20 * time.Second},
	}
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func proxyImage(c fiber.Ctx, client *http.Client, fullURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	resp, err := client.Do(req)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": err.Error()})
	}
	defer resp.Body.Close()

	c.Status(resp.StatusCode)
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		c.Set("Content-Type", ct)
	} else {
		c.Set("Content-Type", "image/jpeg")
	}
	c.Set("Cache-Control", "public, max-age=3600, s-maxage=3600")

	_, copyErr := io.Copy(c, resp.Body)
	return copyErr
}

// GET /img/primary/:id
func Primary(opts Opts) fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Params("id", "")
		if id == "" {
			return c.Status(400).JSON(fiber.Map{"error": "missing item id"})
		}

		u := fmt.Sprintf("%s/emby/Items/%s/Images/Primary", opts.BaseURL, url.PathEscape(id))
		q := url.Values{}
		q.Set("api_key", opts.APIKey)
		q.Set("quality", strconv.Itoa(opts.Quality))
		q.Set("maxWidth", strconv.Itoa(opts.PrimaryMaxWidth))

		return proxyImage(c, opts.HTTPClient, u+"?"+q.Encode())
	}
}

// GET /img/backdrop/:id
func Backdrop(opts Opts) fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Params("id", "")
		if id == "" {
			return c.Status(400).JSON(fiber.Map{"error": "missing item id"})
		}

		u := fmt.Sprintf("%s/emby/Items/%s/Images/Backdrop", opts.BaseURL, url.PathEscape(id))
		q := url.Values{}
		q.Set("api_key", opts.APIKey)
		q.Set("quality", strconv.Itoa(opts.Quality))
		q.Set("maxWidth", strconv.Itoa(opts.BackdropMaxWidth))

		return proxyImage(c, opts.HTTPClient, u+"?"+q.Encode())
	}
}

// MultiServerPrimary handles image requests with server routing: /img/primary/:server/:id
func MultiServerPrimary(multiServerMgr interface{}) fiber.Handler {
	return func(c fiber.Ctx) error {
		serverType := c.Params("server", "")
		id := c.Params("id", "")

		if serverType == "" || id == "" {
			return c.Status(400).JSON(fiber.Map{"error": "missing server or item id"})
		}

		var imageURL string

		// Use environment variable values (same as configured in the multi-server manager)
		switch serverType {
		case "plex":
			// Plex image URL format - use photo transcode endpoint
			imageURL = fmt.Sprintf("http://plex:32400/photo/:/transcode?width=300&height=450&minSize=1&upscale=1&url=/library/metadata/%s/thumb&X-Plex-Token=2V3pSLipwDso8ziMyxYj",
				url.PathEscape(id))
		case "emby":
			// Emby image URL format
			imageURL = fmt.Sprintf("http://emby:8096/emby/Items/%s/Images/Primary?api_key=9bcb8efb00244f889a78e5878ab89b41&quality=90&maxWidth=300",
				url.PathEscape(id))
		case "jellyfin":
			// Jellyfin image URL format (no "/jellyfin" prefix)
			imageURL = fmt.Sprintf("http://jellyfin:8096/Items/%s/Images/Primary?api_key=0528a4ed9fc34d669ce4bea9c17d7f69&quality=90&maxWidth=300",
				url.PathEscape(id))
		default:
			return c.Status(404).JSON(fiber.Map{"error": "unsupported server type"})
		}

		// Create HTTP client and proxy the request
		httpClient := &http.Client{Timeout: 20 * time.Second}
		return proxyImage(c, httpClient, imageURL)
	}
}

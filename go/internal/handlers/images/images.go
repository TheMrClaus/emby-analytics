package images

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"emby-analytics/internal/config"
	"emby-analytics/internal/media"
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
	mgr, _ := multiServerMgr.(*media.MultiServerManager)
	primaryWidth := getenvInt("IMG_PRIMARY_MAX_WIDTH", 300)
	primaryHeight := getenvInt("IMG_PRIMARY_MAX_HEIGHT", int(float64(primaryWidth)*1.5))
	quality := getenvInt("IMG_QUALITY", 90)

	return func(c fiber.Ctx) error {
		serverParam := strings.TrimSpace(c.Params("server", ""))
		id := c.Params("id", "")

		if serverParam == "" || id == "" {
			return c.Status(400).JSON(fiber.Map{"error": "missing server or item id"})
		}

		cfg := resolveServerConfig(mgr, serverParam)
		if cfg == nil {
			return c.Status(404).JSON(fiber.Map{"error": "server configuration not found"})
		}

		imageURL, err := buildServerImageURL(*cfg, id, imageVariantPrimary, primaryWidth, primaryHeight, quality)
		if err != nil {
			return c.Status(502).JSON(fiber.Map{"error": err.Error()})
		}

		httpClient := &http.Client{Timeout: 20 * time.Second}
		return proxyImage(c, httpClient, imageURL)
	}
}

func MultiServerBackdrop(multiServerMgr interface{}) fiber.Handler {
	mgr, _ := multiServerMgr.(*media.MultiServerManager)
	backdropWidth := getenvInt("IMG_BACKDROP_MAX_WIDTH", 1280)
	backdropHeight := getenvInt("IMG_BACKDROP_MAX_HEIGHT", int(float64(backdropWidth)*9.0/16.0))
	quality := getenvInt("IMG_QUALITY", 90)

	return func(c fiber.Ctx) error {
		serverParam := strings.TrimSpace(c.Params("server", ""))
		id := c.Params("id", "")
		if serverParam == "" || id == "" {
			return c.Status(400).JSON(fiber.Map{"error": "missing server or item id"})
		}

		cfg := resolveServerConfig(mgr, serverParam)
		if cfg == nil {
			return c.Status(404).JSON(fiber.Map{"error": "server configuration not found"})
		}

		imageURL, err := buildServerImageURL(*cfg, id, imageVariantBackdrop, backdropWidth, backdropHeight, quality)
		if err != nil {
			return c.Status(502).JSON(fiber.Map{"error": err.Error()})
		}

		httpClient := &http.Client{Timeout: 20 * time.Second}
		return proxyImage(c, httpClient, imageURL)
	}
}

type imageVariant string

const (
	imageVariantPrimary  imageVariant = "primary"
	imageVariantBackdrop imageVariant = "backdrop"
)

func resolveServerConfig(mgr *media.MultiServerManager, serverParam string) *media.ServerConfig {
	if mgr == nil {
		return nil
	}
	sp := strings.TrimSpace(serverParam)
	if sp == "" {
		return nil
	}
	lower := strings.ToLower(sp)
	configs := mgr.GetServerConfigs()
	for _, cfg := range configs {
		cfgCopy := cfg
		if !cfgCopy.Enabled {
			continue
		}
		if strings.EqualFold(cfgCopy.ID, sp) || strings.EqualFold(string(cfgCopy.Type), lower) {
			return &cfgCopy
		}
	}
	return nil
}

func buildServerImageURL(cfg media.ServerConfig, itemID string, variant imageVariant, width, height, quality int) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(cfg.ExternalURL), "/")
	}
	if base == "" {
		return "", fmt.Errorf("no base URL configured for server %s", cfg.ID)
	}
	token := strings.TrimSpace(cfg.APIKey)
	switch cfg.Type {
	case media.ServerTypeEmby:
		if token == "" {
			return "", fmt.Errorf("api key not configured for server %s", cfg.ID)
		}
		path := "Primary"
		if variant == imageVariantBackdrop {
			path = "Backdrop"
		}
		u := fmt.Sprintf("%s/emby/Items/%s/Images/%s", base, url.PathEscape(itemID), path)
		q := url.Values{}
		q.Set("api_key", token)
		q.Set("quality", strconv.Itoa(quality))
		q.Set("maxWidth", strconv.Itoa(width))
		return u + "?" + q.Encode(), nil
	case media.ServerTypeJellyfin:
		if token == "" {
			return "", fmt.Errorf("api key not configured for server %s", cfg.ID)
		}
		path := "Primary"
		if variant == imageVariantBackdrop {
			path = "Backdrop"
		}
		u := fmt.Sprintf("%s/Items/%s/Images/%s", base, url.PathEscape(itemID), path)
		q := url.Values{}
		q.Set("api_key", token)
		q.Set("quality", strconv.Itoa(quality))
		q.Set("maxWidth", strconv.Itoa(width))
		return u + "?" + q.Encode(), nil
	case media.ServerTypePlex:
		if token == "" {
			return "", fmt.Errorf("plex token not configured for server %s", cfg.ID)
		}
		var remotePath string
		if variant == imageVariantBackdrop {
			remotePath = fmt.Sprintf("/library/metadata/%s/art", url.PathEscape(itemID))
		} else {
			remotePath = fmt.Sprintf("/library/metadata/%s/thumb", url.PathEscape(itemID))
		}
		params := url.Values{}
		params.Set("width", strconv.Itoa(width))
		params.Set("height", strconv.Itoa(height))
		params.Set("minSize", "1")
		params.Set("upscale", "1")
		params.Set("url", remotePath)
		params.Set("X-Plex-Token", token)
		return fmt.Sprintf("%s/photo/:/transcode?%s", base, params.Encode()), nil
	default:
		return "", fmt.Errorf("unsupported server type %s", cfg.Type)
	}
}

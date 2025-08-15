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

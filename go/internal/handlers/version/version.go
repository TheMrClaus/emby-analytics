package version

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	appver "emby-analytics/internal/version"

	"github.com/gofiber/fiber/v3"
)

var (
	cacheMu      sync.Mutex
	cachedAt     time.Time
	cachedLatest string
	cachedURL    string
	cacheTTL     = 6 * time.Hour
)

// GetVersion returns build version info and checks GitHub for the latest tag.
func GetVersion() fiber.Handler {
	return func(c fiber.Ctx) error {
		info := Info()

		latestTag, latestURL := latestRelease(info.Repo)
		info.LatestTag = latestTag
		info.LatestURL = latestURL
		info.UpdateAvailable = newerThan(latestTag, info.Version)

		return c.JSON(info)
	}
}

// Info assembles the static portion of version info.
func Info() appver.Info {
	repo := appver.Repo
	if repo == "" {
		// Allow override via environment if ldflag not set
		repo = os.Getenv("GIT_REPO")
	}
	url := linkURL(repo, appver.Version, appver.Commit)
	return appver.Info{
		Version: appver.Version,
		Commit:  appver.Commit,
		Date:    appver.Date,
		Repo:    repo,
		URL:     url,
	}
}

// latestRelease fetches latest release/tag from GitHub with simple caching.
func latestRelease(repo string) (tag, url string) {
	if repo == "" {
		return "", ""
	}
	cacheMu.Lock()
	if time.Since(cachedAt) < cacheTTL && cachedLatest != "" {
		t, u := cachedLatest, cachedURL
		cacheMu.Unlock()
		return t, u
	}
	cacheMu.Unlock()

	client := &http.Client{Timeout: 5 * time.Second}
	// Prefer releases; fall back to tags if no releases published
	relAPI := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	if t, u, ok := fetchLatestRelease(client, relAPI); ok {
		cacheSet(t, u)
		return t, u
	}
	// Fallback: first tag
	tagsAPI := fmt.Sprintf("https://api.github.com/repos/%s/tags?per_page=1", repo)
	if t, u, ok := fetchLatestTag(client, tagsAPI, repo); ok {
		cacheSet(t, u)
		return t, u
	}
	return "", ""
}

func cacheSet(tag, url string) {
	cacheMu.Lock()
	cachedLatest = tag
	cachedURL = url
	cachedAt = time.Now()
	cacheMu.Unlock()
}

func fetchLatestRelease(client *http.Client, url string) (tag, htmlURL string, ok bool) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("User-Agent", "emby-analytics")
	res, err := client.Do(req)
	if err != nil || res.StatusCode >= 400 {
		return "", "", false
	}
	defer res.Body.Close()
	var v struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(res.Body).Decode(&v); err != nil || v.TagName == "" {
		return "", "", false
	}
	return v.TagName, v.HTMLURL, true
}

func fetchLatestTag(client *http.Client, url string, repo string) (tag, htmlURL string, ok bool) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("User-Agent", "emby-analytics")
	res, err := client.Do(req)
	if err != nil || res.StatusCode >= 400 {
		return "", "", false
	}
	defer res.Body.Close()
	var tags []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(res.Body).Decode(&tags); err != nil || len(tags) == 0 {
		return "", "", false
	}
	t := tags[0].Name
	return t, fmt.Sprintf("https://github.com/%s/releases/tag/%s", repo, t), true
}

// linkURL returns the relevant GitHub URL for this build.
func linkURL(repo, version, commit string) string {
	if repo == "" {
		return ""
	}
	if strings.HasPrefix(version, "v") {
		return fmt.Sprintf("https://github.com/%s/releases/tag/%s", repo, version)
	}
	if commit != "" && commit != "none" {
		return fmt.Sprintf("https://github.com/%s/commit/%s", repo, commit)
	}
	return fmt.Sprintf("https://github.com/%s", repo)
}

var semverRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+).*`)

// newerThan reports whether latest is newer than current using basic semver.
func newerThan(latest, current string) bool {
	if latest == "" || current == "" {
		return false
	}
	l := semverRe.FindStringSubmatch(latest)
	c := semverRe.FindStringSubmatch(current)
	if len(l) == 0 || len(c) == 0 {
		// Fallback: if different and latest starts with 'v', treat as available when not equal
		return latest != current
	}
	for i := 1; i <= 3; i++ {
		if l[i] > c[i] {
			return true
		}
		if l[i] < c[i] {
			return false
		}
	}
	return false
}

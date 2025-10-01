package version

// Package version holds build-time and runtime version metadata.
// Values are intended to be overridden via -ldflags during build.

// These variables are set via ldflags; provide sensible defaults for dev.
var (
	Version = "dev"     // e.g., v1.2.3 or git describe output
	Commit  = "none"    // short git SHA
	Date    = "unknown" // build UTC timestamp
	Repo    = ""        // e.g., owner/repo; optional
)

type Info struct {
	Version         string `json:"version"`
	Commit          string `json:"commit"`
	Date            string `json:"date"`
	Repo            string `json:"repo"`
	URL             string `json:"url"`
	LatestTag       string `json:"latest_tag"`
	LatestURL       string `json:"latest_url"`
	UpdateAvailable bool   `json:"update_available"`
}

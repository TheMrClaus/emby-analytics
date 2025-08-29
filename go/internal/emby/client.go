package emby

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

//
// ---------- HTTP / JSON helpers ----------
//

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Heuristic: Emby often reports kbps for sources/streams.
// Treat small values as kbps; large values as already bps.
func normalizeToBps(v int64) int64 {
	if v < 1_000_000 { // e.g. 57_000 -> 57 Mbps
		return v * 1000
	}
	return v
}

// readJSON enforces 200 OK and JSON-decodes into dst.
// On failure, it returns an error that includes status and a short body snippet.
func readJSON(resp *http.Response, dst any) error {
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		snippet := string(b)
		if len(snippet) > 240 {
			snippet = snippet[:240] + "…"
		}
		return fmt.Errorf("http %d from %s: %s", resp.StatusCode, resp.Request.URL.String(), snippet)
	}

	// Optional: check content-type is JSON-ish (don't be too strict)
	ct := resp.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(strings.ToLower(ct), "application/json") {
		// still try to decode, but if it fails we'll show a snippet
	}

	if err := json.Unmarshal(b, dst); err != nil {
		snippet := string(b)
		if len(snippet) > 240 {
			snippet = snippet[:240] + "…"
		}
		return fmt.Errorf("decode json from %s: %w; body: %q", resp.Request.URL.String(), err, snippet)
	}
	return nil
}

//
// ---------- Client ----------
//

type Client struct {
	BaseURL  string
	APIKey   string
	http     *http.Client
	cache    sync.Map
	cacheTTL time.Duration
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		APIKey:   apiKey,
		cacheTTL: time.Hour, // 1 hour TTL
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

type cacheEntry struct {
	data      []EmbyItem
	timestamp time.Time
}

//
// ---------- Library (items, codecs, counts) ----------
//

type EmbyItem struct {
	Id                string `json:"Id"`
	Name              string `json:"Name"`
	Type              string `json:"Type"`
	SeriesName        string `json:"SeriesName"`
	ParentIndexNumber *int   `json:"ParentIndexNumber"` // season
	IndexNumber       *int   `json:"IndexNumber"`       // episode
}

type embyItemsResp struct {
	Items []EmbyItem `json:"Items"`
}

// generateCacheKey creates a consistent cache key from item IDs
func (c *Client) generateCacheKey(ids []string) string {
	if len(ids) == 0 {
		return ""
	}

	// Sort IDs to ensure consistent cache keys regardless of order
	sorted := make([]string, len(ids))
	copy(sorted, ids)
	sort.Strings(sorted)

	// Create MD5 hash of sorted IDs
	h := md5.New()
	for _, id := range sorted {
		h.Write([]byte(id))
		h.Write([]byte(","))
	}
	return fmt.Sprintf("items_%x", h.Sum(nil))
}

// getCachedItems retrieves cached items if they exist and are not expired
func (c *Client) getCachedItems(cacheKey string) ([]EmbyItem, bool) {
	if cacheKey == "" {
		return nil, false
	}

	if entry, exists := c.cache.Load(cacheKey); exists {
		if cached, ok := entry.(cacheEntry); ok {
			if time.Since(cached.timestamp) < c.cacheTTL {
				return cached.data, true
			}
			// Entry is expired, remove it
			c.cache.Delete(cacheKey)
		}
	}
	return nil, false
}

// setCachedItems stores items in cache
func (c *Client) setCachedItems(cacheKey string, items []EmbyItem) {
	if cacheKey == "" {
		return
	}

	entry := cacheEntry{
		data:      items,
		timestamp: time.Now(),
	}
	c.cache.Store(cacheKey, entry)
}

// ItemsByIDs fetches item details for a set of IDs (used to prettify Episode display)
// ItemsByIDs fetches item details for a set of IDs (used to prettify Episode display)
func (c *Client) ItemsByIDs(ids []string) ([]EmbyItem, error) {
	if c == nil || c.BaseURL == "" || c.APIKey == "" || len(ids) == 0 {
		return []EmbyItem{}, nil
	}

	// Generate cache key
	cacheKey := c.generateCacheKey(ids)

	// Check cache first
	if cachedItems, found := c.getCachedItems(cacheKey); found {
		return cachedItems, nil
	}

	// Cache miss - fetch from API
	endpoint := fmt.Sprintf("%s/emby/Items", c.BaseURL)
	q := url.Values{}
	q.Set("api_key", c.APIKey)
	q.Set("Ids", strings.Join(ids, ","))

	req, _ := http.NewRequest("GET", endpoint+"?"+q.Encode(), nil)
	req.Header.Set("X-Emby-Token", c.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	var out embyItemsResp
	if err := readJSON(resp, &out); err != nil {
		return nil, err
	}

	// Cache the result
	c.setCachedItems(cacheKey, out.Items)

	return out.Items, nil
}

type LibraryItem struct {
	Id     string `json:"Id"`
	Name   string `json:"Name"`
	Type   string `json:"Type"`
	Height *int   `json:"Height,omitempty"`
	Width  *int   `json:"Width,omitempty"`
	Codec  string `json:"VideoCodec,omitempty"`
}

// Detailed struct for fetching media info with codec data
type DetailedLibraryItem struct {
	Id           string `json:"Id"`
	Name         string `json:"Name"`
	Type         string `json:"Type"`
	MediaSources []struct {
		MediaStreams []struct {
			Type   string `json:"Type"`
			Codec  string `json:"Codec"`
			Height *int   `json:"Height"`
			Width  *int   `json:"Width"`
		} `json:"MediaStreams"`
	} `json:"MediaSources"`
}

type itemsResp struct {
	Items []LibraryItem `json:"Items"`
	Total int           `json:"TotalRecordCount"`
}

func (c *Client) TotalItems() (int, error) {
	u := fmt.Sprintf("%s/emby/Items", c.BaseURL)
	q := url.Values{}
	q.Set("api_key", c.APIKey)
	q.Set("IncludeItemTypes", "Movie,Episode") // Only count video items
	q.Set("Recursive", "true")
	q.Set("StartIndex", "0")
	q.Set("Limit", "1")

	req, _ := http.NewRequest("GET", u+"?"+q.Encode(), nil)
	req.Header.Set("X-Emby-Token", c.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}

	var out itemsResp
	if err := readJSON(resp, &out); err != nil {
		return 0, err
	}
	return out.Total, nil
}

// GetItemsChunk extracts codec data from MediaStreams - one entry per media item
func (c *Client) GetItemsChunk(limit, page int) ([]LibraryItem, error) {
	u := fmt.Sprintf("%s/emby/Items", c.BaseURL)
	q := url.Values{}
	q.Set("api_key", c.APIKey)
	q.Set("Fields", "MediaSources,MediaStreams")
	q.Set("Recursive", "true")
	q.Set("StartIndex", fmt.Sprintf("%d", page*limit))
	q.Set("Limit", fmt.Sprintf("%d", limit))
	q.Set("IncludeItemTypes", "Movie,Episode") // Only get video items

	req, _ := http.NewRequest("GET", u+"?"+q.Encode(), nil)
	req.Header.Set("X-Emby-Token", c.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	var out struct {
		Items []DetailedLibraryItem `json:"Items"`
	}
	if err := readJSON(resp, &out); err != nil {
		return nil, err
	}

	// Convert to LibraryItem format, creating ONE entry per media item
	var result []LibraryItem

	for _, item := range out.Items {
		var firstVideoCodec string
		var firstVideoHeight *int
		var firstVideoWidth *int

		// Find the FIRST video stream only (matches C# plugin logic)
		for _, source := range item.MediaSources {
			for _, stream := range source.MediaStreams {
				if stream.Type == "Video" && stream.Codec != "" {
					firstVideoCodec = stream.Codec
					firstVideoHeight = stream.Height
					firstVideoWidth = stream.Width
					goto found // Break out of both loops
				}
			}
		}

	found:
		// Set codec to "Unknown" if no video stream found
		if firstVideoCodec == "" {
			firstVideoCodec = "Unknown"
		}

		// Create ONE LibraryItem entry per media item
		result = append(result, LibraryItem{
			Id:     item.Id, // Use original ID without suffix
			Name:   item.Name,
			Type:   item.Type,
			Height: firstVideoHeight,
			Width:  firstVideoWidth,
			Codec:  firstVideoCodec,
		})
	}

	return result, nil
}

//
// ---------- Users & history ----------
//

type EmbyUser struct {
	Id   string `json:"Id"`
	Name string `json:"Name"`
}

// Struct for history items
type PlayHistoryItem struct {
	Id          string `json:"Id"`
	Name        string `json:"Name"`
	Type        string `json:"Type"`
	DatePlayed  string `json:"DatePlayed"` // ISO8601
	PlaybackPos int64  `json:"PlaybackPositionTicks"`
	UserID      string `json:"-"`
}

type playHistoryResp struct {
	Items []PlayHistoryItem `json:"Items"`
}

// GetUserPlayHistory returns recent items played by a user (daysBack is how many days to look back)
func (c *Client) GetUserPlayHistory(userID string, daysBack int) ([]PlayHistoryItem, error) {
	u := fmt.Sprintf("%s/emby/Users/%s/Items", c.BaseURL, userID)
	q := url.Values{}
	q.Set("api_key", c.APIKey)
	q.Set("SortBy", "DatePlayed")
	q.Set("SortOrder", "Descending")
	q.Set("Filters", "IsPlayed")
	q.Set("Recursive", "true")
	q.Set("Limit", "100")
	if daysBack > 0 {
		from := time.Now().AddDate(0, 0, -daysBack).Format(time.RFC3339)
		q.Set("MinDatePlayed", from)
	}

	req, _ := http.NewRequest("GET", u+"?"+q.Encode(), nil)
	req.Header.Set("X-Emby-Token", c.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	var out struct {
		Items []PlayHistoryItem `json:"Items"`
	}
	if err := readJSON(resp, &out); err != nil {
		return nil, err
	}

	// Attach userID so downstream logic knows which user played it
	for i := range out.Items {
		out.Items[i].UserID = userID
	}
	return out.Items, nil
}

// GetUserRecentActivity returns ALL recent items with playback activity (not just "played" ones)
func (c *Client) GetUserRecentActivity(userID string, daysBack int) ([]PlayHistoryItem, error) {
	u := fmt.Sprintf("%s/emby/Users/%s/Items", c.BaseURL, userID)
	q := url.Values{}
	q.Set("api_key", c.APIKey)
	q.Set("SortBy", "DatePlayed")
	q.Set("SortOrder", "Descending")
	// NOTE: Removed "IsPlayed" filter to show partially watched items too
	q.Set("Recursive", "true")
	q.Set("Limit", "100")
	q.Set("Fields", "UserData")

	if daysBack > 0 {
		from := time.Now().AddDate(0, 0, -daysBack).Format(time.RFC3339)
		q.Set("MinDatePlayed", from)
	}

	req, _ := http.NewRequest("GET", u+"?"+q.Encode(), nil)
	req.Header.Set("X-Emby-Token", c.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	var out struct {
		Items []struct {
			Id       string `json:"Id"`
			Name     string `json:"Name"`
			Type     string `json:"Type"`
			UserData struct {
				LastPlayedDate        string `json:"LastPlayedDate"`
				PlaybackPositionTicks int64  `json:"PlaybackPositionTicks"`
				Played                bool   `json:"Played"`
				PlayCount             int    `json:"PlayCount"`
			} `json:"UserData"`
		} `json:"Items"`
	}
	if err := readJSON(resp, &out); err != nil {
		return nil, err
	}

	// Convert to PlayHistoryItem format
	result := make([]PlayHistoryItem, 0, len(out.Items))
	for _, item := range out.Items {
		// Use LastPlayedDate if available, otherwise skip
		dateToUse := item.UserData.LastPlayedDate
		if dateToUse == "" {
			continue
		}

		result = append(result, PlayHistoryItem{
			Id:          item.Id,
			Name:        item.Name,
			Type:        item.Type,
			DatePlayed:  dateToUse,
			PlaybackPos: item.UserData.PlaybackPositionTicks,
			UserID:      userID,
		})
	}

	return result, nil
}

type usersResp struct {
	Items []EmbyUser `json:"Items"`
}

// GetUsers fetches minimal user data (Id, Name) from Emby server.
// Tries direct array first; if not, retries on the wrapped format.
func (c *Client) GetUsers() ([]EmbyUser, error) {
	u := fmt.Sprintf("%s/emby/Users", c.BaseURL)
	q := url.Values{}
	q.Set("api_key", c.APIKey)
	q.Set("Fields", "") // minimal fields

	makeReq := func() (*http.Response, error) {
		req, _ := http.NewRequest("GET", u+"?"+q.Encode(), nil)
		req.Header.Set("X-Emby-Token", c.APIKey)
		return c.http.Do(req)
	}

	// Try direct array first
	resp, err := makeReq()
	if err != nil {
		return nil, err
	}
	var users []EmbyUser
	if err := readJSON(resp, &users); err == nil {
		return users, nil
	}

	// Fallback: wrapped payload
	resp, err = makeReq()
	if err != nil {
		return nil, err
	}
	var out usersResp
	if err := readJSON(resp, &out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

// GetUserData fetches user's watch status for items
func (c *Client) GetUserData(userID string) ([]UserDataItem, error) {
	u := fmt.Sprintf("%s/emby/Users/%s/Items", c.BaseURL, userID)
	q := url.Values{}
	q.Set("api_key", c.APIKey)
	q.Set("Recursive", "true")
	q.Set("Fields", "UserData,RunTimeTicks")
	q.Set("IncludeItemTypes", "Movie,Episode")
	q.Set("Filters", "IsPlayed")

	req, _ := http.NewRequest("GET", u+"?"+q.Encode(), nil)
	req.Header.Set("X-Emby-Token", c.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	var out struct {
		Items []UserDataItem `json:"Items"`
	}
	if err := readJSON(resp, &out); err != nil {
		return nil, err
	}

	return out.Items, nil
}

type UserDataItem struct {
	Id           string `json:"Id"`
	Name         string `json:"Name"`
	Type         string `json:"Type"`
	RunTimeTicks int64  `json:"RunTimeTicks"`
	UserData     struct {
		Played         bool   `json:"Played"`
		PlaybackPos    int64  `json:"PlaybackPositionTicks"`
		PlayCount      int    `json:"PlayCount"`
		LastPlayedDate string `json:"LastPlayedDate"`
	} `json:"UserData"`
}

//
// ---------- Now Playing (sessions) ----------
//

type EmbySession struct {
	SessionID string `json:"SessionId"`
	UserID    string `json:"UserId"`
	UserName  string `json:"UserName"`
	ItemID    string `json:"ItemId"`
	ItemName  string `json:"ItemName"`
	ItemType  string `json:"ItemType"`

	// Timestamp when playback position was last updated
	PosTicks     int64 `json:"PosTicks"`
	DurationTicks int64 `json:"DurationTicks"`

	// Client/device
	App    string `json:"Client"`
	Device string `json:"DeviceName"`

	// Playback details
	PlayMethod string `json:"PlayMethod,omitempty"` // "Direct"/"Transcode"
	VideoCodec string `json:"VideoCodec,omitempty"`
	AudioCodec string `json:"AudioCodec,omitempty"`
	SubsCount  int    `json:"SubsCount,omitempty"`
	Bitrate    int64  `json:"Bitrate,omitempty"` // bps

	Container   string `json:"Container,omitempty"` // MKV/MP4
	Width       int    `json:"Width,omitempty"`
	Height      int    `json:"Height,omitempty"`
	DolbyVision bool   `json:"DolbyVision,omitempty"`
	HDR10       bool   `json:"HDR10,omitempty"`

	AudioLang    string `json:"AudioLang,omitempty"`
	AudioCh      int    `json:"AudioChannels,omitempty"`
	AudioDefault bool   `json:"AudioDefault,omitempty"` // NEW: Track if current audio is default

	SubLang  string `json:"SubLang,omitempty"`
	SubCodec string `json:"SubCodec,omitempty"`

	// Transcode details (when PlayMethod=Transcode)
	TransVideoFrom string `json:"TransVideoFrom,omitempty"`
	TransVideoTo   string `json:"TransVideoTo,omitempty"`
	TransAudioFrom string `json:"TransAudioFrom,omitempty"`
	TransAudioTo   string `json:"TransAudioTo,omitempty"`

	// Per-track methods
	VideoMethod string `json:"VideoMethod,omitempty"` // "Direct Play" or "Transcode"
	AudioMethod string `json:"AudioMethod,omitempty"`

	// Transcode target details
	TransContainer    string   `json:"TransContainer,omitempty"`
	TransFramerate    float64  `json:"TransFramerate,omitempty"`
	TransAudioBitrate int64    `json:"TransAudioBitrate,omitempty"`
	TransVideoBitrate int64    `json:"TransVideoBitrate,omitempty"`
	TransWidth        int      `json:"TransWidth,omitempty"`
	TransHeight       int      `json:"TransHeight,omitempty"`
	TransReasons      []string `json:"TransReasons,omitempty"`
	TransCompletion   float64  `json:"TransCompletion,omitempty"`
	TransPosTicks     int64    `json:"TransPosTicks,omitempty"`
	RemoteAddress     string   `json:"RemoteAddress,omitempty"`
}

// Update the rawSession struct to include stream indices
type rawSession struct {
	Id             string `json:"Id"` // session id
	UserID         string `json:"UserId"`
	UserName       string `json:"UserName"`
	Client         string `json:"Client"`
	DeviceName     string `json:"DeviceName"`
	RemoteEndPoint string `json:"RemoteEndPoint"` // Emby provides remote IP address

	NowPlayingItem *struct {
		Id           string `json:"Id"`
		Name         string `json:"Name"`
		Type         string `json:"Type"`
		RunTimeTicks int64  `json:"RunTimeTicks"`

		Container string `json:"Container"`

		MediaStreams []struct {
			Index    int    `json:"Index"`    // Stream index - this is key!
			Type     string `json:"Type"`    // "Video","Audio","Subtitle"
			Codec    string `json:"Codec"`   // e.g. h264,aac
			Language string `json:"Language"`
			Channels int    `json:"Channels"`
			Width    int    `json:"Width"`
			Height   int    `json:"Height"`

			// NEW for audio default detection
			IsDefault bool `json:"IsDefault"`
			Default   bool `json:"Default"`

			// NEW: many Emby builds signal DV/HDR here
			VideoRange     string `json:"VideoRange"`     // e.g. "DOVI", "HDR10", "SDR"
			VideoRangeType string `json:"VideoRangeType"` // e.g. "Dv", "Hdr10", "Sdr"

			// Existing flags
			IsHdr     bool `json:"IsHdr"`
			Hdr       bool `json:"Hdr"`
			Hdr10     bool `json:"Hdr10"`
			DvProfile *int `json:"DvProfile,omitempty"`

			BitRate int64 `json:"BitRate,omitempty"`
			Bitrate int64 `json:"Bitrate,omitempty"`
		} `json:"MediaStreams"`

		// Direct-play fallback (often in kbps)
		MediaSources []struct {
			Bitrate int64 `json:"Bitrate"`
		} `json:"MediaSources"`
	} `json:"NowPlayingItem"`

	PlayState *struct {
		PositionTicks       int64  `json:"PositionTicks"`
		PlayMethod          string `json:"PlayMethod"`
		IsPaused            bool   `json:"IsPaused"`
		AudioStreamIndex    *int   `json:"AudioStreamIndex"`    // Currently selected audio stream
		SubtitleStreamIndex *int   `json:"SubtitleStreamIndex"` // Currently selected subtitle stream
	} `json:"PlayState"`

	TranscodingInfo *struct {
		Bitrate                int64    `json:"Bitrate"` // overall bps (target)
		VideoCodec             string   `json:"VideoCodec"`
		AudioCodec             string   `json:"AudioCodec"`
		Container              string   `json:"Container"`    // "ts", "mp4", "fmp4", ...
		Framerate              float64  `json:"Framerate"`    // transcoder speed or output fps (server-dependent)
		AudioBitrate           int64    `json:"AudioBitrate"` // target audio bps
		VideoBitrate           int64    `json:"VideoBitrate"` // target video bps
		Width                  int      `json:"Width"`
		Height                 int      `json:"Height"`
		TranscodeReasons       []string `json:"TranscodeReasons"`     // e.g. AudioCodecNotSupported
		CompletionPercentage   float64  `json:"CompletionPercentage"` // if server reports it
		TranscodePositionTicks int64    `json:"TranscodePositionTicks"`
	} `json:"TranscodingInfo"`
}

// Replace the stream processing logic in GetActiveSessions() function
// Find the part that starts with "// Per-track and stream info" and replace it with:

		// Per-track and stream info
		subs := 0
		streamKbpsSum := int64(0)
		var sourceVideoCodec, sourceAudioCodec string

		// Get currently selected stream indices from PlayState
		var currentAudioIndex, currentSubtitleIndex *int
		if rs.PlayState != nil {
			currentAudioIndex = rs.PlayState.AudioStreamIndex
			currentSubtitleIndex = rs.PlayState.SubtitleStreamIndex
		}

		// Resolution / HDR / audio lang & channels / subs info
		for _, ms := range rs.NowPlayingItem.MediaStreams {
			switch strings.ToLower(ms.Type) {
			case "video":
				if es.VideoCodec == "" && ms.Codec != "" {
					es.VideoCodec = strings.ToUpper(ms.Codec)
				}
				// Always assign sourceVideoCodec if present
				if ms.Codec != "" {
					sourceVideoCodec = strings.ToUpper(ms.Codec)
				}
				if es.Width == 0 && ms.Width > 0 {
					es.Width = ms.Width
					es.Height = ms.Height
				}
				// HDR/DV detection (prefer DV if present)
				vr := strings.ToLower(strings.TrimSpace(ms.VideoRange))
				vrt := strings.ToLower(strings.TrimSpace(ms.VideoRangeType))
				if (ms.DvProfile != nil && *ms.DvProfile > 0) ||
					vr == "dovi" || vr == "dolby vision" || vr == "dolbyvision" ||
					vrt == "dv" || vrt == "dolbyvision" {
					es.DolbyVision = true
				}
				if ms.Hdr10 || ms.Hdr || ms.IsHdr ||
					strings.Contains(vr, "hdr") || vrt == "hdr10" || vrt == "hdr10plus" {
					es.HDR10 = true
				}
				if ms.BitRate > 0 {
					streamKbpsSum += ms.BitRate
				} else if ms.Bitrate > 0 {
					streamKbpsSum += ms.Bitrate
				}
			case "audio":
				// Check if this is the currently selected audio stream
				isCurrentAudio := false
				if currentAudioIndex != nil && *currentAudioIndex == ms.Index {
					isCurrentAudio = true
				} else if currentAudioIndex == nil && (ms.IsDefault || ms.Default) {
					// Fallback to default if no current index is specified
					isCurrentAudio = true
				} else if currentAudioIndex == nil && es.AudioCodec == "" {
					// Further fallback to first audio stream if no default found
					isCurrentAudio = true
				}

				if isCurrentAudio {
					if ms.Codec != "" {
						es.AudioCodec = strings.ToUpper(ms.Codec)
					}
					// Keep language as-is (don't force uppercase) so "English" stays "English"
					if ms.Language != "" {
						es.AudioLang = ms.Language
					}
					if ms.Channels > 0 {
						es.AudioCh = ms.Channels
					}
					// Only mark as default if it's actually the default stream
					if ms.IsDefault || ms.Default {
						es.AudioDefault = true
					}
				}
				
				// Always assign sourceAudioCodec if not set and present
				if sourceAudioCodec == "" && ms.Codec != "" {
					sourceAudioCodec = strings.ToUpper(ms.Codec)
				}
				
				if ms.BitRate > 0 {
					streamKbpsSum += ms.BitRate
				} else if ms.Bitrate > 0 {
					streamKbpsSum += ms.Bitrate
				}

			case "subtitle":
				subs++
				// Check if this is the currently selected subtitle stream
				isCurrentSubtitle := false
				if currentSubtitleIndex != nil && *currentSubtitleIndex == ms.Index {
					isCurrentSubtitle = true
				} else if currentSubtitleIndex == nil && es.SubLang == "" {
					// Only take first sub details if no specific one is selected and we haven't set one yet
					isCurrentSubtitle = true
				}

				if isCurrentSubtitle {
					if ms.Language != "" {
						es.SubLang = strings.ToUpper(ms.Language)
					}
					if ms.Codec != "" {
						es.SubCodec = strings.ToUpper(ms.Codec)
					}
				}
			}
		}
		es.SubsCount = subs

// GetActiveSessions returns only sessions that have a NowPlayingItem.
func (c *Client) GetActiveSessions() ([]EmbySession, error) {
	u := fmt.Sprintf("%s/emby/Sessions", c.BaseURL)
	q := url.Values{}
	q.Set("api_key", c.APIKey)

	req, _ := http.NewRequest("GET", u+"?"+q.Encode(), nil)
	// Some setups prefer header token; keep header for compatibility.
	req.Header.Set("X-Emby-Token", c.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	var raw []rawSession
	if err := readJSON(resp, &raw); err != nil {
		return nil, err
	}

	out := make([]EmbySession, 0, len(raw))
	for _, rs := range raw {
		// Only show active playback
		if rs.NowPlayingItem == nil || rs.NowPlayingItem.Id == "" {
			continue
		}

		es := EmbySession{
			SessionID: rs.Id,
			UserID:    rs.UserID,
			UserName:  rs.UserName,
			App:       rs.Client,
			Device:    rs.DeviceName,
		}

		// Item + duration
		es.ItemID = rs.NowPlayingItem.Id
		es.ItemName = rs.NowPlayingItem.Name
		es.ItemType = rs.NowPlayingItem.Type
		es.DurationTicks = rs.NowPlayingItem.RunTimeTicks

		// Capture additional media info defaults
		es.Container = strings.ToUpper(rs.NowPlayingItem.Container)

		// Extract remote address
		es.RemoteAddress = rs.RemoteEndPoint

		// Extract transcoding reasons if transcoding is active
		if rs.TranscodingInfo != nil {
			es.TransReasons = rs.TranscodingInfo.TranscodeReasons
		}

		// Per-track and stream info
		subs := 0
		streamKbpsSum := int64(0)
		var sourceVideoCodec, sourceAudioCodec string

		// Resolution / HDR / audio lang & channels / subs info
		for _, ms := range rs.NowPlayingItem.MediaStreams {
			switch strings.ToLower(ms.Type) {
			case "video":
				if es.VideoCodec == "" && ms.Codec != "" {
					es.VideoCodec = strings.ToUpper(ms.Codec)
				}
				// Always assign sourceVideoCodec if present
				if ms.Codec != "" {
					sourceVideoCodec = strings.ToUpper(ms.Codec)
				}
				if es.Width == 0 && ms.Width > 0 {
					es.Width = ms.Width
					es.Height = ms.Height
				}
				// HDR/DV detection (prefer DV if present)
				vr := strings.ToLower(strings.TrimSpace(ms.VideoRange))
				vrt := strings.ToLower(strings.TrimSpace(ms.VideoRangeType))
				if (ms.DvProfile != nil && *ms.DvProfile > 0) ||
					vr == "dovi" || vr == "dolby vision" || vr == "dolbyvision" ||
					vrt == "dv" || vrt == "dolbyvision" {
					es.DolbyVision = true
				}
				if ms.Hdr10 || ms.Hdr || ms.IsHdr ||
					strings.Contains(vr, "hdr") || vrt == "hdr10" || vrt == "hdr10plus" {
					es.HDR10 = true
				}
				if ms.BitRate > 0 {
					streamKbpsSum += ms.BitRate
				} else if ms.Bitrate > 0 {
					streamKbpsSum += ms.Bitrate
				}
			case "audio":
				if es.AudioCodec == "" && ms.Codec != "" {
					es.AudioCodec = strings.ToUpper(ms.Codec)
				}
				// Always assign sourceAudioCodec if not set and present
				if sourceAudioCodec == "" && ms.Codec != "" {
					sourceAudioCodec = strings.ToUpper(ms.Codec)
				}
				// Keep language as-is (don't force uppercase) so "English" stays "English"
				if es.AudioLang == "" && ms.Language != "" {
					es.AudioLang = ms.Language
				}
				if es.AudioCh == 0 && ms.Channels > 0 {
					es.AudioCh = ms.Channels
				}
				// NEW: detect default audio track
				if ms.IsDefault || ms.Default {
					es.AudioDefault = true
				}
				if ms.BitRate > 0 {
					streamKbpsSum += ms.BitRate
				} else if ms.Bitrate > 0 {
					streamKbpsSum += ms.Bitrate
				}

			case "subtitle":
				subs++
				// Keep first sub details for convenience
				if es.SubLang == "" && ms.Language != "" {
					es.SubLang = strings.ToUpper(ms.Language)
				}
				if es.SubCodec == "" && ms.Codec != "" {
					es.SubCodec = strings.ToUpper(ms.Codec)
				}
			}
		}
		es.SubsCount = subs

		// PlayState
		if rs.PlayState != nil {
			es.PosTicks = rs.PlayState.PositionTicks
			if rs.PlayState.PlayMethod != "" {
				if strings.HasPrefix(strings.ToLower(rs.PlayState.PlayMethod), "trans") {
					es.PlayMethod = "Transcode"
				} else {
					es.PlayMethod = "Direct"
				}
			}
		}
		if es.PlayMethod == "" {
			es.PlayMethod = "Direct"
		}

		// Bitrate selection and transcode info
		if rs.TranscodingInfo != nil && rs.TranscodingInfo.Bitrate > 0 {
			es.Bitrate = rs.TranscodingInfo.Bitrate

			// Target (TO) codecs/container/etc
			es.TransContainer = strings.ToUpper(rs.TranscodingInfo.Container)
			es.TransFramerate = rs.TranscodingInfo.Framerate
			es.TransAudioBitrate = rs.TranscodingInfo.AudioBitrate
			es.TransVideoBitrate = rs.TranscodingInfo.VideoBitrate
			es.TransWidth = rs.TranscodingInfo.Width
			es.TransHeight = rs.TranscodingInfo.Height
			es.TransReasons = append(es.TransReasons, rs.TranscodingInfo.TranscodeReasons...)
			es.TransCompletion = rs.TranscodingInfo.CompletionPercentage
			es.TransPosTicks = rs.TranscodingInfo.TranscodePositionTicks

			if v := rs.TranscodingInfo.VideoCodec; v != "" {
				es.TransVideoTo = strings.ToUpper(v)
			}
			if a := rs.TranscodingInfo.AudioCodec; a != "" {
				es.TransAudioTo = strings.ToUpper(a)
			}

			// Fill FROM using detected source codecs
			if sourceVideoCodec != "" {
				es.TransVideoFrom = sourceVideoCodec
			}
			if sourceAudioCodec != "" {
				es.TransAudioFrom = sourceAudioCodec
			}

			es.PlayMethod = "Transcode"
		} else {
			// 2) MediaSource bitrate (often kbps)
			if rs.NowPlayingItem != nil && len(rs.NowPlayingItem.MediaSources) > 0 {
				if b := rs.NowPlayingItem.MediaSources[0].Bitrate; b > 0 {
					es.Bitrate = normalizeToBps(b)
				}
			}
			// 3) Sum stream bitrates (kbps) if source bitrate missing
			if es.Bitrate == 0 && streamKbpsSum > 0 {
				es.Bitrate = normalizeToBps(streamKbpsSum)
			}
		}

		// Decide per-track methods
		es.VideoMethod = "Direct Play"
		es.AudioMethod = "Direct Play"
		if es.PlayMethod == "Transcode" {
			if es.TransVideoFrom != "" && es.TransVideoTo != "" && es.TransVideoFrom != es.TransVideoTo {
				es.VideoMethod = "Transcode"
			}
			if es.TransAudioFrom != "" && es.TransAudioTo != "" && es.TransAudioFrom != es.TransAudioTo {
				es.AudioMethod = "Transcode"
			}
		}

		out = append(out, es)
	}
	return out, nil
}

//
// ---------- Session controls (pause/play/stop/message) ----------
//

func (c *Client) Pause(sessionID string) error {
	u := fmt.Sprintf("%s/emby/Sessions/%s/Playing/Pause?api_key=%s", c.BaseURL, sessionID, url.QueryEscape(c.APIKey))
	req, _ := http.NewRequest("POST", u, nil)
	req.Header.Set("X-Emby-Token", c.APIKey)
	_, err := c.http.Do(req)
	return err
}

func (c *Client) Unpause(sessionID string) error {
	u := fmt.Sprintf("%s/emby/Sessions/%s/Playing/Unpause?api_key=%s", c.BaseURL, sessionID, url.QueryEscape(c.APIKey))
	req, _ := http.NewRequest("POST", u, nil)
	req.Header.Set("X-Emby-Token", c.APIKey)
	_, err := c.http.Do(req)
	return err
}

func (c *Client) Stop(sessionID string) error {
	u := fmt.Sprintf("%s/emby/Sessions/%s/Playing/Stop?api_key=%s", c.BaseURL, sessionID, url.QueryEscape(c.APIKey))
	req, _ := http.NewRequest("POST", u, nil)
	req.Header.Set("X-Emby-Token", c.APIKey)
	_, err := c.http.Do(req)
	return err
}

func (c *Client) SendMessage(sessionID, header, text string, timeoutMs int) error {
	if timeoutMs <= 0 {
		timeoutMs = 5000
	}
	payload := map[string]any{
		"Header":    header,
		"Text":      text,
		"TimeoutMs": timeoutMs,
	}
	b, _ := json.Marshal(payload)
	u := fmt.Sprintf("%s/emby/Sessions/%s/Message?api_key=%s", c.BaseURL, sessionID, url.QueryEscape(c.APIKey))
	req, _ := http.NewRequest("POST", u, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Emby-Token", c.APIKey)
	_, err := c.http.Do(req)
	return err
}

type EmbySystemInfo struct {
	ID   string `json:"Id"`
	Name string `json:"ServerName"`
}

// GetSystemInfo fetches server information including the server ID
func (c *Client) GetSystemInfo() (*EmbySystemInfo, error) {
	u := fmt.Sprintf("%s/emby/System/Info", c.BaseURL)
	q := url.Values{}
	q.Set("api_key", c.APIKey)

	req, _ := http.NewRequest("GET", u+"?"+q.Encode(), nil)
	req.Header.Set("X-Emby-Token", c.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	var info EmbySystemInfo
	if err := readJSON(resp, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

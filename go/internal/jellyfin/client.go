package jellyfin

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"emby-analytics/internal/media"
)

// Client represents a Jellyfin Media Server client
type Client struct {
	serverID    string
	serverName  string
	baseURL     string
	apiKey      string
	externalURL string
	http        *http.Client
	cache       sync.Map
	cacheTTL    time.Duration
}

// New creates a new Jellyfin client
func New(config media.ServerConfig) *Client {
	return &Client{
		serverID:    config.ID,
		serverName:  config.Name,
		baseURL:     strings.TrimRight(config.BaseURL, "/"),
		apiKey:      config.APIKey,
		externalURL: config.ExternalURL,
		cacheTTL:    time.Hour,
		http: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:       10,
				IdleConnTimeout:    30 * time.Second,
				DisableCompression: false,
			},
		},
	}
}

// Jellyfin JSON response structures (similar to Emby but with potential differences)
type jellyfinSession struct {
	Id                 string `json:"Id"`
	ServerId           string `json:"ServerId"`
	UserId             string `json:"UserId"`
	UserName           string `json:"UserName"`
	Client             string `json:"Client"`
	DeviceName         string `json:"DeviceName"`
	DeviceId           string `json:"DeviceId"`
	ApplicationVersion string `json:"ApplicationVersion"`
	RemoteEndPoint     string `json:"RemoteEndPoint"`

	NowPlayingItem *struct {
		Id           string `json:"Id"`
		Name         string `json:"Name"`
		Type         string `json:"Type"`
		RunTimeTicks int64  `json:"RunTimeTicks"`
		Container    string `json:"Container"`

		MediaStreams []struct {
			Index     int    `json:"Index"`
			Type      string `json:"Type"`
			Codec     string `json:"Codec"`
			Language  string `json:"Language"`
			Channels  int    `json:"Channels"`
			Width     int    `json:"Width"`
			Height    int    `json:"Height"`
			IsDefault bool   `json:"IsDefault"`
			Default   bool   `json:"Default"`

			// HDR/DV detection
			VideoRange     string `json:"VideoRange"`
			VideoRangeType string `json:"VideoRangeType"`
			IsHdr          bool   `json:"IsHdr"`
			Hdr            bool   `json:"Hdr"`
			Hdr10          bool   `json:"Hdr10"`
			DvProfile      *int   `json:"DvProfile,omitempty"`

			BitRate int64 `json:"BitRate,omitempty"`
			Bitrate int64 `json:"Bitrate,omitempty"`
		} `json:"MediaStreams"`

		MediaSources []struct {
			Bitrate int64 `json:"Bitrate"`
		} `json:"MediaSources"`
	} `json:"NowPlayingItem"`

	PlayState *struct {
		PositionTicks       int64  `json:"PositionTicks"`
		PlayMethod          string `json:"PlayMethod"`
		IsPaused            bool   `json:"IsPaused"`
		AudioStreamIndex    *int   `json:"AudioStreamIndex"`
		SubtitleStreamIndex *int   `json:"SubtitleStreamIndex"`
	} `json:"PlayState"`

	TranscodingInfo *struct {
		Bitrate                int64    `json:"Bitrate"`
		VideoCodec             string   `json:"VideoCodec"`
		AudioCodec             string   `json:"AudioCodec"`
		Container              string   `json:"Container"`
		Framerate              float64  `json:"Framerate"`
		AudioBitrate           int64    `json:"AudioBitrate"`
		VideoBitrate           int64    `json:"VideoBitrate"`
		Width                  int      `json:"Width"`
		Height                 int      `json:"Height"`
		TranscodeReasons       []string `json:"TranscodeReasons"`
		CompletionPercentage   float64  `json:"CompletionPercentage"`
		TranscodePositionTicks int64    `json:"TranscodePositionTicks"`
	} `json:"TranscodingInfo"`
}

type jellyfinUser struct {
	Id   string `json:"Id"`
	Name string `json:"Name"`
}

type jellyfinSystemInfo struct {
	Id          string `json:"Id"`
	ServerName  string `json:"ServerName"`
	Version     string `json:"Version"`
	ProductName string `json:"ProductName"`
}

type jellyfinMediaItem struct {
	Id                string   `json:"Id"`
	Name              string   `json:"Name"`
	Type              string   `json:"Type"`
	SeriesId          string   `json:"SeriesId,omitempty"`
	SeriesName        string   `json:"SeriesName"`
	ParentIndexNumber *int     `json:"ParentIndexNumber"`
	IndexNumber       *int     `json:"IndexNumber"`
	ProductionYear    *int     `json:"ProductionYear"`
	RunTimeTicks      *int64   `json:"RunTimeTicks"`
	Container         string   `json:"Container"`
	Genres            []string `json:"Genres"`
}

// Jellyfin API uses 100-nanosecond ticks like Emby
const ticksPerMillisecond = 10000

// ticksToMs converts Jellyfin ticks to milliseconds
func ticksToMs(ticks int64) int64 {
	return ticks / ticksPerMillisecond
}

// Interface implementation

// GetServerID returns the server ID
func (c *Client) GetServerID() string {
	return c.serverID
}

// GetServerType returns the server type
func (c *Client) GetServerType() media.ServerType {
	return media.ServerTypeJellyfin
}

// GetServerName returns the server name
func (c *Client) GetServerName() string {
	return c.serverName
}

// doRequest performs HTTP request with proper Jellyfin authentication
func (c *Client) doRequest(endpoint string) (*http.Response, error) {
	u := fmt.Sprintf("%s%s", c.baseURL, endpoint)

	// Add API key to URL parameters
	parsedURL, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	q := parsedURL.Query()
	q.Set("api_key", c.apiKey)
	parsedURL.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", parsedURL.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Emby-Token", c.apiKey) // Jellyfin uses same header as Emby
	req.Header.Set("Accept", "application/json")

	return c.http.Do(req)
}

// doWithRetry performs HTTP request with exponential backoff retry
func (c *Client) doWithRetry(req *http.Request, maxRetries int) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		var reqClone *http.Request
		if attempt > 0 {
			reqClone = req.Clone(req.Context())
		} else {
			reqClone = req
		}

		resp, err := c.http.Do(reqClone)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		if resp != nil {
			resp.Body.Close()
		}

		lastErr = err
		if err == nil {
			lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
		}

		if attempt < maxRetries {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			time.Sleep(backoff)
		}
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", maxRetries+1, lastErr)
}

// readJSON reads and parses JSON response
func readJSON(resp *http.Response, dst interface{}) error {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		snippet := string(body)
		if len(snippet) > 240 {
			snippet = snippet[:240] + "…"
		}
		return fmt.Errorf("http %d from %s: %s", resp.StatusCode, resp.Request.URL.String(), snippet)
	}

	if err := json.Unmarshal(body, dst); err != nil {
		snippet := string(body)
		if len(snippet) > 240 {
			snippet = snippet[:240] + "…"
		}
		return fmt.Errorf("decode json from %s: %w; body: %q", resp.Request.URL.String(), err, snippet)
	}

	return nil
}

// GetActiveSessions returns active Jellyfin sessions
func (c *Client) GetActiveSessions() ([]media.Session, error) {
	u := fmt.Sprintf("%s/Sessions", c.baseURL)
	q := url.Values{}
	q.Set("api_key", c.apiKey)

	req, _ := http.NewRequest("GET", u+"?"+q.Encode(), nil)
	req.Header.Set("X-Emby-Token", c.apiKey)

	resp, err := c.doWithRetry(req, 2)
	if err != nil {
		return nil, err
	}

	var jellySessions []jellyfinSession
	if err := readJSON(resp, &jellySessions); err != nil {
		return nil, err
	}

	sessions := make([]media.Session, 0)

	for _, jellySess := range jellySessions {
		// Only include sessions with active playback
		if jellySess.NowPlayingItem == nil || jellySess.NowPlayingItem.Id == "" {
			continue
		}

		session := c.convertSession(jellySess)
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// convertSession converts Jellyfin session to normalized Session
func (c *Client) convertSession(jellySess jellyfinSession) media.Session {
	session := media.Session{
		ServerID:      c.serverID,
		ServerType:    media.ServerTypeJellyfin,
		SessionID:     jellySess.Id,
		UserID:        jellySess.UserId,
		UserName:      jellySess.UserName,
		ItemID:        jellySess.NowPlayingItem.Id,
		ItemName:      jellySess.NowPlayingItem.Name,
		ItemType:      jellySess.NowPlayingItem.Type,
		DurationMs:    ticksToMs(jellySess.NowPlayingItem.RunTimeTicks),
		ClientApp:     jellySess.Client,
		DeviceName:    jellySess.DeviceName,
		RemoteAddress: jellySess.RemoteEndPoint,
		Container:     strings.ToUpper(jellySess.NowPlayingItem.Container),
		LastUpdate:    time.Now(),
	}

	// PlayState information
	if jellySess.PlayState != nil {
		session.PositionMs = ticksToMs(jellySess.PlayState.PositionTicks)
		session.IsPaused = jellySess.PlayState.IsPaused

		if jellySess.PlayState.PlayMethod != "" {
			if strings.HasPrefix(strings.ToLower(jellySess.PlayState.PlayMethod), "trans") {
				session.PlayMethod = "Transcode"
			} else {
				session.PlayMethod = "Direct"
			}
		}
	}

	if session.PlayMethod == "" {
		session.PlayMethod = "Direct"
	}

	// Extract media stream information
	subs := 0
	streamKbpsSum := int64(0)
	var sourceVideoCodec, sourceAudioCodec string

	// Get currently selected stream indices from PlayState
	var currentAudioIndex, currentSubtitleIndex *int
	if jellySess.PlayState != nil {
		currentAudioIndex = jellySess.PlayState.AudioStreamIndex
		currentSubtitleIndex = jellySess.PlayState.SubtitleStreamIndex
	}

	// Process media streams
	for _, ms := range jellySess.NowPlayingItem.MediaStreams {
		switch strings.ToLower(ms.Type) {
		case "video":
			if session.VideoCodec == "" && ms.Codec != "" {
				session.VideoCodec = strings.ToUpper(ms.Codec)
			}
			if sourceVideoCodec == "" && ms.Codec != "" {
				sourceVideoCodec = strings.ToUpper(ms.Codec)
			}
			if session.Width == 0 && ms.Width > 0 {
				session.Width = ms.Width
				session.Height = ms.Height
			}

			// HDR/DV detection
			vr := strings.ToLower(strings.TrimSpace(ms.VideoRange))
			vrt := strings.ToLower(strings.TrimSpace(ms.VideoRangeType))
			if (ms.DvProfile != nil && *ms.DvProfile > 0) ||
				vr == "dovi" || vr == "dolby vision" || vr == "dolbyvision" ||
				vrt == "dv" || vrt == "dolbyvision" {
				session.DolbyVision = true
			}
			if ms.Hdr10 || ms.Hdr || ms.IsHdr ||
				strings.Contains(vr, "hdr") || vrt == "hdr10" || vrt == "hdr10plus" {
				session.HDR10 = true
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
				isCurrentAudio = true
			} else if currentAudioIndex == nil && session.AudioCodec == "" {
				isCurrentAudio = true
			}

			if isCurrentAudio {
				if ms.Codec != "" {
					session.AudioCodec = strings.ToUpper(ms.Codec)
				}
				if ms.Language != "" {
					session.AudioLanguage = ms.Language
				}
				if ms.Channels > 0 {
					session.AudioChannels = ms.Channels
				}
				if ms.IsDefault || ms.Default {
					session.AudioDefault = true
				}
			}

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
			} else if currentSubtitleIndex == nil && session.SubtitleLanguage == "" {
				isCurrentSubtitle = true
			}

			if isCurrentSubtitle {
				if ms.Language != "" {
					session.SubtitleLanguage = strings.ToUpper(ms.Language)
				}
				if ms.Codec != "" {
					session.SubtitleCodec = strings.ToUpper(ms.Codec)
				}
			}
		}
	}
	session.SubtitleCount = subs

	// Bitrate selection and transcode info
	if jellySess.TranscodingInfo != nil && jellySess.TranscodingInfo.Bitrate > 0 {
		session.Bitrate = jellySess.TranscodingInfo.Bitrate
		session.PlayMethod = "Transcode"

		// Target (TO) codecs/container/etc
		session.TranscodeContainer = strings.ToUpper(jellySess.TranscodingInfo.Container)
		session.TranscodeVideoCodec = strings.ToUpper(jellySess.TranscodingInfo.VideoCodec)
		session.TranscodeAudioCodec = strings.ToUpper(jellySess.TranscodingInfo.AudioCodec)
		session.TranscodeProgress = jellySess.TranscodingInfo.CompletionPercentage
		session.TranscodeWidth = jellySess.TranscodingInfo.Width
		session.TranscodeHeight = jellySess.TranscodingInfo.Height
		session.TranscodeBitrate = jellySess.TranscodingInfo.VideoBitrate
		session.TranscodeReasons = jellySess.TranscodingInfo.TranscodeReasons

		// Fill FROM using detected source codecs
		if sourceVideoCodec != "" {
			// Compare source vs target codecs to determine method
			if sourceVideoCodec != session.TranscodeVideoCodec {
				session.VideoMethod = "Transcode"
			} else {
				session.VideoMethod = "Direct Play"
			}
		}
		if sourceAudioCodec != "" {
			if sourceAudioCodec != session.TranscodeAudioCodec {
				session.AudioMethod = "Transcode"
			} else {
				session.AudioMethod = "Direct Play"
			}
		}
	} else {
		// MediaSource bitrate fallback
		if jellySess.NowPlayingItem != nil && len(jellySess.NowPlayingItem.MediaSources) > 0 {
			if b := jellySess.NowPlayingItem.MediaSources[0].Bitrate; b > 0 {
				// Jellyfin bitrate is typically in bps, but check if normalization needed
				if b < 1_000_000 {
					session.Bitrate = b * 1000 // Convert kbps to bps
				} else {
					session.Bitrate = b
				}
			}
		}
		// Sum stream bitrates if source bitrate missing
		if session.Bitrate == 0 && streamKbpsSum > 0 {
			if streamKbpsSum < 1_000_000 {
				session.Bitrate = streamKbpsSum * 1000 // Convert kbps to bps
			} else {
				session.Bitrate = streamKbpsSum
			}
		}

		// Default track methods for direct play
		session.VideoMethod = "Direct Play"
		session.AudioMethod = "Direct Play"
	}

	return session
}

// GetSystemInfo returns Jellyfin server information
func (c *Client) GetSystemInfo() (*media.SystemInfo, error) {
	u := fmt.Sprintf("%s/System/Info", c.baseURL)
	q := url.Values{}
	q.Set("api_key", c.apiKey)

	req, _ := http.NewRequest("GET", u+"?"+q.Encode(), nil)
	req.Header.Set("X-Emby-Token", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	var info jellyfinSystemInfo
	if err := readJSON(resp, &info); err != nil {
		return nil, err
	}

	return &media.SystemInfo{
		ID:         info.Id,
		Name:       info.ServerName,
		ServerType: media.ServerTypeJellyfin,
		Version:    info.Version,
	}, nil
}

// GetUsers returns Jellyfin users
func (c *Client) GetUsers() ([]media.User, error) {
	u := fmt.Sprintf("%s/Users", c.baseURL)
	q := url.Values{}
	q.Set("api_key", c.apiKey)
	q.Set("Fields", "") // minimal fields

	req, _ := http.NewRequest("GET", u+"?"+q.Encode(), nil)
	req.Header.Set("X-Emby-Token", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	var jellyUsers []jellyfinUser
	if err := readJSON(resp, &jellyUsers); err != nil {
		return nil, err
	}

	users := make([]media.User, 0, len(jellyUsers))
	for _, jellyUser := range jellyUsers {
		users = append(users, media.User{
			ID:         jellyUser.Id,
			Name:       jellyUser.Name,
			ServerID:   c.serverID,
			ServerType: media.ServerTypeJellyfin,
		})
	}

	return users, nil
}

// ItemsByIDs fetches media items by IDs
func (c *Client) ItemsByIDs(ids []string) ([]media.MediaItem, error) {
	if len(ids) == 0 {
		return []media.MediaItem{}, nil
	}

	// Check cache first
	cacheKey := c.generateCacheKey(ids)
	if cached, found := c.getCachedItems(cacheKey); found {
		return cached, nil
	}

	u := fmt.Sprintf("%s/Items", c.baseURL)
	q := url.Values{}
	q.Set("api_key", c.apiKey)
	q.Set("Ids", strings.Join(ids, ","))
	q.Set("Fields", "SeriesId,SeriesName,ParentIndexNumber,IndexNumber")

	req, _ := http.NewRequest("GET", u+"?"+q.Encode(), nil)
	req.Header.Set("X-Emby-Token", c.apiKey)

	resp, err := c.doWithRetry(req, 2)
	if err != nil {
		return nil, err
	}

	var out struct {
		Items []jellyfinMediaItem `json:"Items"`
	}
	if err := readJSON(resp, &out); err != nil {
		return nil, err
	}

	items := make([]media.MediaItem, 0, len(out.Items))
	for _, jellyItem := range out.Items {
		item := media.MediaItem{
			ID:         jellyItem.Id,
			ServerID:   c.serverID,
			ServerType: media.ServerTypeJellyfin,
			Name:       jellyItem.Name,
			Type:       jellyItem.Type,
			Container:  jellyItem.Container,
			Genres:     jellyItem.Genres,
		}

		if jellyItem.RunTimeTicks != nil {
			runtimeMs := ticksToMs(*jellyItem.RunTimeTicks)
			item.RuntimeMs = &runtimeMs
		}

		if jellyItem.ProductionYear != nil {
			item.ProductionYear = jellyItem.ProductionYear
		}

		// Episode-specific fields
		if jellyItem.Type == "Episode" {
			item.SeriesID = jellyItem.SeriesId
			item.SeriesName = jellyItem.SeriesName
			item.ParentIndexNumber = jellyItem.ParentIndexNumber
			item.IndexNumber = jellyItem.IndexNumber
		}

		items = append(items, item)
	}

	// Cache results
	c.setCachedItems(cacheKey, items)

	return items, nil
}

// FetchLibraryItems retrieves full library metadata for the requested item types (e.g., Movie, Episode).
func (c *Client) FetchLibraryItems(includeTypes []string) ([]media.MediaItem, error) {
	if len(includeTypes) == 0 {
		return []media.MediaItem{}, nil
	}
	items := make([]media.MediaItem, 0)
	const pageSize = 200
	typesParam := strings.Join(includeTypes, ",")
	for start := 0; ; start += pageSize {
		u := fmt.Sprintf("%s/Items", c.baseURL)
		q := url.Values{}
		q.Set("api_key", c.apiKey)
		q.Set("Recursive", "true")
		q.Set("IncludeItemTypes", typesParam)
		q.Set("Fields", "MediaSources,MediaStreams,RunTimeTicks,Container,Genres,ProductionYear,SeriesId,SeriesName,ParentIndexNumber,IndexNumber")
		q.Set("EnableTotalRecordCount", "true")
		q.Set("StartIndex", strconv.Itoa(start))
		q.Set("Limit", strconv.Itoa(pageSize))

		req, _ := http.NewRequest("GET", u+"?"+q.Encode(), nil)
		req.Header.Set("X-Emby-Token", c.apiKey)

		resp, err := c.doWithRetry(req, 2)
		if err != nil {
			return nil, err
		}

		var out struct {
			Items []struct {
				Id                string   `json:"Id"`
				Name              string   `json:"Name"`
				Type              string   `json:"Type"`
				RunTimeTicks      *int64   `json:"RunTimeTicks"`
				Container         string   `json:"Container"`
				Genres            []string `json:"Genres"`
				ProductionYear    *int     `json:"ProductionYear"`
				SeriesId          string   `json:"SeriesId"`
				SeriesName        string   `json:"SeriesName"`
				ParentIndexNumber *int     `json:"ParentIndexNumber"`
				IndexNumber       *int     `json:"IndexNumber"`
				MediaSources      []struct {
					Container string `json:"Container"`
					Bitrate   *int64 `json:"Bitrate"`
					Size      *int64 `json:"Size"`
					Path      string `json:"Path"`
				} `json:"MediaSources"`
				MediaStreams []struct {
					Type   string `json:"Type"`
					Codec  string `json:"Codec"`
					Width  *int   `json:"Width"`
					Height *int   `json:"Height"`
				} `json:"MediaStreams"`
			} `json:"Items"`
			TotalRecordCount int `json:"TotalRecordCount"`
		}
		if err := readJSON(resp, &out); err != nil {
			return nil, err
		}

		for _, raw := range out.Items {
			item := media.MediaItem{
				ID:             raw.Id,
				ServerID:       c.serverID,
				ServerType:     media.ServerTypeJellyfin,
				Name:           raw.Name,
				Type:           raw.Type,
				Container:      raw.Container,
				Genres:         raw.Genres,
				ProductionYear: raw.ProductionYear,
			}
			if raw.RunTimeTicks != nil {
				runtimeMs := ticksToMs(*raw.RunTimeTicks)
				item.RuntimeMs = &runtimeMs
			}
			if len(raw.MediaSources) > 0 {
				source := raw.MediaSources[0]
				if item.Container == "" {
					item.Container = source.Container
				}
				item.BitrateBps = source.Bitrate
				item.FileSizeBytes = source.Size
				if source.Path != "" {
					item.FilePath = source.Path
				}
			}
			for _, stream := range raw.MediaStreams {
				if strings.EqualFold(stream.Type, "Video") {
					if stream.Width != nil {
						item.Width = stream.Width
					}
					if stream.Height != nil {
						item.Height = stream.Height
					}
					if stream.Codec != "" {
						item.Codec = strings.ToUpper(stream.Codec)
					}
					break
				}
			}

			if strings.EqualFold(item.Type, "Episode") {
				seriesID := strings.TrimSpace(raw.SeriesId)
				seriesName := strings.TrimSpace(raw.SeriesName)
				item.SeriesID = seriesID
				item.SeriesName = seriesName
				item.ParentIndexNumber = raw.ParentIndexNumber
				item.IndexNumber = raw.IndexNumber

				displayName := raw.Name
				if seriesName != "" && strings.TrimSpace(raw.Name) != "" {
					if raw.ParentIndexNumber != nil && raw.IndexNumber != nil {
						displayName = fmt.Sprintf("%s - %s (S%02dE%02d)", seriesName, raw.Name, *raw.ParentIndexNumber, *raw.IndexNumber)
					} else {
						displayName = fmt.Sprintf("%s - %s", seriesName, raw.Name)
					}
				}
				item.Name = displayName
			}
			items = append(items, item)
		}

		if len(out.Items) < pageSize || start+len(out.Items) >= out.TotalRecordCount {
			break
		}
	}
	return items, nil
}

// GetUserPlayHistory returns user play history
func (c *Client) GetUserPlayHistory(userID string, daysBack int) ([]media.PlayHistoryItem, error) {
	u := fmt.Sprintf("%s/Users/%s/Items", c.baseURL, userID)
	q := url.Values{}
	q.Set("api_key", c.apiKey)
	q.Set("SortBy", "DatePlayed")
	q.Set("SortOrder", "Descending")
	q.Set("Filters", "IsPlayed")
	q.Set("Recursive", "true")
	q.Set("Limit", "100")
	q.Set("Fields", "UserData")

	if daysBack > 0 {
		from := time.Now().AddDate(0, 0, -daysBack).Format(time.RFC3339)
		q.Set("MinDatePlayed", from)
	}

	req, _ := http.NewRequest("GET", u+"?"+q.Encode(), nil)
	req.Header.Set("X-Emby-Token", c.apiKey)

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

	history := make([]media.PlayHistoryItem, 0, len(out.Items))
	for _, item := range out.Items {
		if item.UserData.LastPlayedDate == "" {
			continue
		}

		history = append(history, media.PlayHistoryItem{
			ID:          item.Id,
			ServerID:    c.serverID,
			ServerType:  media.ServerTypeJellyfin,
			Name:        item.Name,
			Type:        item.Type,
			DatePlayed:  item.UserData.LastPlayedDate,
			PlaybackPos: ticksToMs(item.UserData.PlaybackPositionTicks),
			UserID:      userID,
		})
	}

	return history, nil
}

// GetUserData returns Jellyfin user playback metadata for watched items
func (c *Client) GetUserData(userID string) ([]media.UserDataItem, error) {
	u := fmt.Sprintf("%s/Users/%s/Items", c.baseURL, userID)
	q := url.Values{}
	q.Set("api_key", c.apiKey)
	q.Set("Recursive", "true")
	q.Set("Fields", "UserData,RunTimeTicks")
	q.Set("IncludeItemTypes", "Movie,Episode")
	q.Set("Filters", "IsPlayed")

	req, _ := http.NewRequest("GET", u+"?"+q.Encode(), nil)
	req.Header.Set("X-Emby-Token", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	var out struct {
		Items []struct {
			Id           string `json:"Id"`
			Name         string `json:"Name"`
			Type         string `json:"Type"`
			RunTimeTicks *int64 `json:"RunTimeTicks"`
			UserData     struct {
				Played                bool   `json:"Played"`
				PlaybackPositionTicks int64  `json:"PlaybackPositionTicks"`
				PlayCount             int    `json:"PlayCount"`
				LastPlayedDate        string `json:"LastPlayedDate"`
			} `json:"UserData"`
		} `json:"Items"`
	}
	if err := readJSON(resp, &out); err != nil {
		return nil, err
	}

	items := make([]media.UserDataItem, 0, len(out.Items))
	for _, it := range out.Items {
		var runtimeMs int64
		if it.RunTimeTicks != nil {
			runtimeMs = ticksToMs(*it.RunTimeTicks)
		}
		items = append(items, media.UserDataItem{
			ID:                 it.Id,
			ServerID:           c.serverID,
			ServerType:         media.ServerTypeJellyfin,
			Name:               it.Name,
			Type:               it.Type,
			RuntimeMs:          runtimeMs,
			Played:             it.UserData.Played,
			PlayCount:          it.UserData.PlayCount,
			PlaybackPositionMs: ticksToMs(it.UserData.PlaybackPositionTicks),
			LastPlayed:         it.UserData.LastPlayedDate,
		})
	}

	return items, nil
}

// Session control methods

// PauseSession pauses a Jellyfin session
func (c *Client) PauseSession(sessionID string) error {
	u := fmt.Sprintf("%s/Sessions/%s/Playing/Pause?api_key=%s", c.baseURL, sessionID, url.QueryEscape(c.apiKey))
	req, _ := http.NewRequest("POST", u, nil)
	req.Header.Set("X-Emby-Token", c.apiKey)
	resp, err := c.http.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	return err
}

// UnpauseSession resumes a Jellyfin session
func (c *Client) UnpauseSession(sessionID string) error {
	u := fmt.Sprintf("%s/Sessions/%s/Playing/Unpause?api_key=%s", c.baseURL, sessionID, url.QueryEscape(c.apiKey))
	req, _ := http.NewRequest("POST", u, nil)
	req.Header.Set("X-Emby-Token", c.apiKey)
	resp, err := c.http.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	return err
}

// StopSession stops a Jellyfin session
func (c *Client) StopSession(sessionID string) error {
	u := fmt.Sprintf("%s/Sessions/%s/Playing/Stop?api_key=%s", c.baseURL, sessionID, url.QueryEscape(c.apiKey))
	req, _ := http.NewRequest("POST", u, nil)
	req.Header.Set("X-Emby-Token", c.apiKey)
	resp, err := c.http.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	return err
}

// SendMessage sends a message to a Jellyfin session
func (c *Client) SendMessage(sessionID, header, text string, timeoutMs int) error {
	if timeoutMs <= 0 {
		timeoutMs = 5000
	}

	payload := map[string]interface{}{
		"Header":    header,
		"Text":      text,
		"TimeoutMs": timeoutMs,
	}

	body, _ := json.Marshal(payload)
	u := fmt.Sprintf("%s/Sessions/%s/Message?api_key=%s", c.baseURL, sessionID, url.QueryEscape(c.apiKey))
	req, _ := http.NewRequest("POST", u, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Emby-Token", c.apiKey)

	resp, err := c.http.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	return err
}

// CheckHealth checks Jellyfin server health
func (c *Client) CheckHealth() (*media.ServerHealth, error) {
	start := time.Now()

	u := fmt.Sprintf("%s/System/Info", c.baseURL)
	q := url.Values{}
	q.Set("api_key", c.apiKey)

	req, _ := http.NewRequest("GET", u+"?"+q.Encode(), nil)
	req.Header.Set("X-Emby-Token", c.apiKey)

	resp, err := c.http.Do(req)
	responseTime := time.Since(start).Milliseconds()

	health := &media.ServerHealth{
		ServerID:     c.serverID,
		ServerType:   media.ServerTypeJellyfin,
		ServerName:   c.serverName,
		ResponseTime: responseTime,
		LastCheck:    time.Now(),
	}

	if err != nil {
		health.IsReachable = false
		health.Error = err.Error()
		return health, err
	}

	resp.Body.Close()
	health.IsReachable = resp.StatusCode == http.StatusOK

	if resp.StatusCode != http.StatusOK {
		health.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	return health, nil
}

// Cache management

func (c *Client) generateCacheKey(ids []string) string {
	if len(ids) == 0 {
		return ""
	}

	sorted := make([]string, len(ids))
	copy(sorted, ids)
	sort.Strings(sorted)

	h := md5.New()
	for _, id := range sorted {
		h.Write([]byte(id))
		h.Write([]byte(","))
	}
	return fmt.Sprintf("jellyfin_items_%x", h.Sum(nil))
}

type cacheEntry struct {
	data      []media.MediaItem
	timestamp time.Time
}

func (c *Client) getCachedItems(cacheKey string) ([]media.MediaItem, bool) {
	if cacheKey == "" {
		return nil, false
	}

	if entry, exists := c.cache.Load(cacheKey); exists {
		if cached, ok := entry.(cacheEntry); ok {
			if time.Since(cached.timestamp) < c.cacheTTL {
				return cached.data, true
			}
			c.cache.Delete(cacheKey)
		}
	}
	return nil, false
}

func (c *Client) setCachedItems(cacheKey string, items []media.MediaItem) {
	if cacheKey == "" {
		return
	}

	entry := cacheEntry{
		data:      items,
		timestamp: time.Now(),
	}
	c.cache.Store(cacheKey, entry)
}

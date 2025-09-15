package plex

import (
    "crypto/md5"
    "encoding/xml"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "sort"
    "strings"
    "sync"
    "time"

	"emby-analytics/internal/media"
)

// Client represents a Plex Media Server client
type Client struct {
	serverID    string
	serverName  string
	baseURL     string
	token       string
	externalURL string
	http        *http.Client
	cache       sync.Map
	cacheTTL    time.Duration
}

// New creates a new Plex client
func New(config media.ServerConfig) *Client {
	return &Client{
		serverID:    config.ID,
		serverName:  config.Name,
		baseURL:     strings.TrimRight(config.BaseURL, "/"),
		token:       config.APIKey,
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

// Plex XML response structures
type plexMediaContainer struct {
	XMLName  xml.Name      `xml:"MediaContainer"`
	Size     int           `xml:"size,attr"`
	Metadata []plexSession `xml:"Metadata"`
	Users    []plexUser    `xml:"User"`
	Info     plexSystemInfo `xml:",any"`
}

type plexSession struct {
	XMLName      xml.Name `xml:"Metadata"`
	SessionKey   string   `xml:"sessionKey,attr"`
	RatingKey    string   `xml:"ratingKey,attr"`
	Key          string   `xml:"key,attr"`
	ParentKey    string   `xml:"parentKey,attr"`
	GrandparentKey string `xml:"grandparentKey,attr"`
	Title        string   `xml:"title,attr"`
	Type         string   `xml:"type,attr"`
	Duration     int64    `xml:"duration,attr"` // milliseconds
	ViewOffset   int64    `xml:"viewOffset,attr"` // milliseconds
	
	User struct {
		ID    string `xml:"id,attr"`
		Title string `xml:"title,attr"`
	} `xml:"User"`
	
	Player struct {
		Address    string `xml:"address,attr"`
		Device     string `xml:"device,attr"`
		MachineID  string `xml:"machineIdentifier,attr"`
		Platform   string `xml:"platform,attr"`
		Product    string `xml:"product,attr"`
		Title      string `xml:"title,attr"`
		Version    string `xml:"version,attr"`
		State      string `xml:"state,attr"` // playing, paused, stopped
	} `xml:"Player"`
	
	Session struct {
		ID        string `xml:"id,attr"`
		Bandwidth int64  `xml:"bandwidth,attr"`
		Location  string `xml:"location,attr"`
	} `xml:"Session"`
	
	Media []struct {
		AudioCodec      string `xml:"audioCodec,attr"`
		AudioChannels   int    `xml:"audioChannels,attr"`
		Bitrate         int64  `xml:"bitrate,attr"`
		Container       string `xml:"container,attr"`
		Duration        int64  `xml:"duration,attr"`
		Height          int    `xml:"height,attr"`
		Width           int    `xml:"width,attr"`
		VideoCodec      string `xml:"videoCodec,attr"`
		VideoFrameRate  string `xml:"videoFrameRate,attr"`
		VideoResolution string `xml:"videoResolution,attr"`
		
		Part []struct {
			Accessible        bool   `xml:"accessible,attr"`
			AudioProfile      string `xml:"audioProfile,attr"`
			Container         string `xml:"container,attr"`
			Decision          string `xml:"decision,attr"` // transcode, copy, direct play
			Duration          int64  `xml:"duration,attr"`
			File              string `xml:"file,attr"`
			HasThumbnail      bool   `xml:"hasThumbnail,attr"`
			Size              int64  `xml:"size,attr"`
			VideoProfile      string `xml:"videoProfile,attr"`
			
			Stream []struct {
				BitDepth        int    `xml:"bitDepth,attr"`
				Bitrate         int64  `xml:"bitrate,attr"`
				Channels        int    `xml:"channels,attr"`
				Codec           string `xml:"codec,attr"`
				CodecID         string `xml:"codecID,attr"`
				DisplayTitle    string `xml:"displayTitle,attr"`
				ExtendedDisplayTitle string `xml:"extendedDisplayTitle,attr"`
				FrameRate       float64 `xml:"frameRate,attr"`
				Height          int    `xml:"height,attr"`
				ID              string `xml:"id,attr"`
				Index           int    `xml:"index,attr"`
				Language        string `xml:"language,attr"`
				LanguageCode    string `xml:"languageCode,attr"`
				LanguageTag     string `xml:"languageTag,attr"`
				Profile         string `xml:"profile,attr"`
				Selected        bool   `xml:"selected,attr"`
				StreamType      int    `xml:"streamType,attr"` // 1=video, 2=audio, 3=subtitle
				Title           string `xml:"title,attr"`
				Width           int    `xml:"width,attr"`
			} `xml:"Stream"`
		} `xml:"Part"`
	} `xml:"Media"`
	
	TranscodeSession *struct {
		XMLName           xml.Name `xml:"TranscodeSession"`
		Key               string   `xml:"key,attr"`
		Throttled         bool     `xml:"throttled,attr"`
		Complete          bool     `xml:"complete,attr"`
		Progress          float64  `xml:"progress,attr"`
		Size              int64    `xml:"size,attr"`
		Speed             float64  `xml:"speed,attr"`
		Error             bool     `xml:"error,attr"`
		Duration          int64    `xml:"duration,attr"`
		Remaining         int64    `xml:"remaining,attr"`
		Context           string   `xml:"context,attr"`
		SourceVideoCodec  string   `xml:"sourceVideoCodec,attr"`
		SourceAudioCodec  string   `xml:"sourceAudioCodec,attr"`
		VideoDecision     string   `xml:"videoDecision,attr"`
		AudioDecision     string   `xml:"audioDecision,attr"`
		Protocol          string   `xml:"protocol,attr"`
		Container         string   `xml:"container,attr"`
		VideoCodec        string   `xml:"videoCodec,attr"`
		AudioCodec        string   `xml:"audioCodec,attr"`
		AudioChannels     int      `xml:"audioChannels,attr"`
		Width             int      `xml:"width,attr"`
		Height            int      `xml:"height,attr"`
		TranscodeHwRequested bool  `xml:"transcodeHwRequested,attr"`
		TranscodeHwDecoding  string `xml:"transcodeHwDecoding,attr"`
		TranscodeHwEncoding  string `xml:"transcodeHwEncoding,attr"`
	} `xml:"TranscodeSession"`
}

type plexUser struct {
	XMLName xml.Name `xml:"User"`
	ID      string   `xml:"id,attr"`
	UUID    string   `xml:"uuid,attr"`
	Title   string   `xml:"title,attr"`
	Thumb   string   `xml:"thumb,attr"`
}

type plexSystemInfo struct {
	XMLName           xml.Name `xml:"MediaContainer"`
	MachineIdentifier string   `xml:"machineIdentifier,attr"`
	Version           string   `xml:"version,attr"`
	FriendlyName      string   `xml:"friendlyName,attr"`
	Platform          string   `xml:"platform,attr"`
	PlatformVersion   string   `xml:"platformVersion,attr"`
}

type plexMediaItem struct {
	XMLName         xml.Name `xml:"Metadata"`
	RatingKey       string   `xml:"ratingKey,attr"`
	Key             string   `xml:"key,attr"`
	ParentKey       string   `xml:"parentKey,attr"`
	GrandparentKey  string   `xml:"grandparentKey,attr"`
	Type            string   `xml:"type,attr"`
	Title           string   `xml:"title,attr"`
	ParentTitle     string   `xml:"parentTitle,attr"`
	GrandparentTitle string  `xml:"grandparentTitle,attr"`
	ContentRating   string   `xml:"contentRating,attr"`
	Summary         string   `xml:"summary,attr"`
	Index           int      `xml:"index,attr"`
	ParentIndex     int      `xml:"parentIndex,attr"`
	Year            int      `xml:"year,attr"`
	Duration        int64    `xml:"duration,attr"`
	AddedAt         int64    `xml:"addedAt,attr"`
	UpdatedAt       int64    `xml:"updatedAt,attr"`
}

// Interface implementation

// GetServerID returns the server ID
func (c *Client) GetServerID() string {
	return c.serverID
}

// GetServerType returns the server type
func (c *Client) GetServerType() media.ServerType {
	return media.ServerTypePlex
}

// GetServerName returns the server name
func (c *Client) GetServerName() string {
	return c.serverName
}

// doRequest performs HTTP request with proper Plex authentication
func (c *Client) doRequest(endpoint string) (*http.Response, error) {
    u := fmt.Sprintf("%s%s", c.baseURL, endpoint)
	
	// Add token to URL parameters
	parsedURL, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	
	q := parsedURL.Query()
	q.Set("X-Plex-Token", c.token)
	parsedURL.RawQuery = q.Encode()
	
    req, err := http.NewRequest("GET", parsedURL.String(), nil)
    if err != nil {
        return nil, err
    }

    req.Header.Set("X-Plex-Token", c.token)
    // Helpful standard Plex headers for compatibility
    req.Header.Set("X-Plex-Product", "emby-analytics")
    req.Header.Set("X-Plex-Version", "1.0")
    req.Header.Set("X-Plex-Client-Identifier", c.serverID)
    req.Header.Set("X-Plex-Platform", "linux")
    req.Header.Set("Accept", "application/xml")
	
	return c.http.Do(req)
}

// readXML reads and parses XML response
func readXML(resp *http.Response, dst interface{}) error {
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
	
	if err := xml.Unmarshal(body, dst); err != nil {
		snippet := string(body)
		if len(snippet) > 240 {
			snippet = snippet[:240] + "…"
		}
		return fmt.Errorf("decode xml from %s: %w; body: %q", resp.Request.URL.String(), err, snippet)
	}
	
	return nil
}

// GetActiveSessions returns active Plex sessions
func (c *Client) GetActiveSessions() ([]media.Session, error) {
	resp, err := c.doRequest("/status/sessions")
	if err != nil {
		return nil, err
	}
	
	var container plexMediaContainer
	if err := readXML(resp, &container); err != nil {
		return nil, err
	}
	
	sessions := make([]media.Session, 0, len(container.Metadata))
	
	for _, plexSess := range container.Metadata {
		session := c.convertSession(plexSess)
		sessions = append(sessions, session)
	}
	
	return sessions, nil
}

// convertSession converts Plex session to normalized Session
func (c *Client) convertSession(plexSess plexSession) media.Session {
	session := media.Session{
		ServerID:       c.serverID,
		ServerType:     media.ServerTypePlex,
		SessionID:      plexSess.SessionKey,
		UserID:         plexSess.User.ID,
		UserName:       plexSess.User.Title,
		ItemID:         plexSess.RatingKey,
		ItemName:       plexSess.Title,
		ItemType:       plexSess.Type,
		PositionMs:     plexSess.ViewOffset,
		DurationMs:     plexSess.Duration,
		ClientApp:      plexSess.Player.Product,
		DeviceName:     plexSess.Player.Title,
		RemoteAddress:  plexSess.Player.Address,
		IsPaused:       plexSess.Player.State == "paused",
		LastUpdate:     time.Now(),
	}
	
	// Extract media information
	if len(plexSess.Media) > 0 {
		media := plexSess.Media[0]
		session.VideoCodec = strings.ToUpper(media.VideoCodec)
		session.AudioCodec = strings.ToUpper(media.AudioCodec)
		session.Container = strings.ToUpper(media.Container)
		session.Width = media.Width
		session.Height = media.Height
		session.Bitrate = media.Bitrate
		session.AudioChannels = media.AudioChannels
		
		// Determine play method based on decision
		if len(media.Part) > 0 {
			decision := media.Part[0].Decision
			switch strings.ToLower(decision) {
			case "transcode":
				session.PlayMethod = "Transcode"
			case "copy", "directplay":
				session.PlayMethod = "Direct"
			default:
				session.PlayMethod = "Direct"
			}
		}
		
		// Extract stream details
		for _, part := range media.Part {
			for _, stream := range part.Stream {
				if stream.Selected {
					switch stream.StreamType {
					case 2: // Audio
						session.AudioLanguage = stream.Language
						session.AudioDefault = true
					case 3: // Subtitle
						session.SubtitleLanguage = stream.Language
						session.SubtitleCodec = strings.ToUpper(stream.Codec)
					}
				}
				if stream.StreamType == 3 { // Count subtitles
					session.SubtitleCount++
				}
			}
		}
	}
	
	// Handle transcode session
	if plexSess.TranscodeSession != nil {
		ts := plexSess.TranscodeSession
		session.PlayMethod = "Transcode"
		session.TranscodeContainer = strings.ToUpper(ts.Container)
		session.TranscodeVideoCodec = strings.ToUpper(ts.VideoCodec)
		session.TranscodeAudioCodec = strings.ToUpper(ts.AudioCodec)
		session.TranscodeProgress = ts.Progress
		session.TranscodeWidth = ts.Width
		session.TranscodeHeight = ts.Height
		
		// Determine track methods
		if ts.VideoDecision == "transcode" {
			session.VideoMethod = "Transcode"
		} else {
			session.VideoMethod = "Direct Play"
		}
		
		if ts.AudioDecision == "transcode" {
			session.AudioMethod = "Transcode"
		} else {
			session.AudioMethod = "Direct Play"
		}
	}
	
	return session
}

// GetSystemInfo returns Plex server information
func (c *Client) GetSystemInfo() (*media.SystemInfo, error) {
	resp, err := c.doRequest("/")
	if err != nil {
		return nil, err
	}
	
	var container plexSystemInfo
	if err := readXML(resp, &container); err != nil {
		return nil, err
	}
	
	return &media.SystemInfo{
		ID:         container.MachineIdentifier,
		Name:       container.FriendlyName,
		ServerType: media.ServerTypePlex,
		Version:    container.Version,
	}, nil
}

// GetUsers returns Plex users
func (c *Client) GetUsers() ([]media.User, error) {
	resp, err := c.doRequest("/accounts")
	if err != nil {
		return nil, err
	}
	
	var container plexMediaContainer
	if err := readXML(resp, &container); err != nil {
		return nil, err
	}
	
	users := make([]media.User, 0, len(container.Users))
	for _, plexUser := range container.Users {
		users = append(users, media.User{
			ID:         plexUser.ID,
			Name:       plexUser.Title,
			ServerID:   c.serverID,
			ServerType: media.ServerTypePlex,
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
	
	var items []media.MediaItem
	
	// Plex doesn't support bulk requests, fetch individually
	for _, id := range ids {
		resp, err := c.doRequest(fmt.Sprintf("/library/metadata/%s", id))
		if err != nil {
			continue // Skip failed items
		}
		
		var container struct {
			XMLName  xml.Name       `xml:"MediaContainer"`
			Metadata []plexMediaItem `xml:"Metadata"`
		}
		
		if err := readXML(resp, &container); err != nil {
			continue
		}
		
		for _, plexItem := range container.Metadata {
			item := media.MediaItem{
				ID:         plexItem.RatingKey,
				ServerID:   c.serverID,
				ServerType: media.ServerTypePlex,
				Name:       plexItem.Title,
				Type:       plexItem.Type,
				RuntimeMs:  &plexItem.Duration,
			}
			
			if plexItem.Year > 0 {
				item.ProductionYear = &plexItem.Year
			}
			
			// Episode-specific fields
			if plexItem.Type == "episode" {
				item.SeriesName = plexItem.GrandparentTitle
				item.SeriesID = plexItem.GrandparentKey
				if plexItem.ParentIndex > 0 {
					item.ParentIndexNumber = &plexItem.ParentIndex
				}
				if plexItem.Index > 0 {
					item.IndexNumber = &plexItem.Index
				}
			}
			
			items = append(items, item)
		}
	}
	
	// Cache results
	c.setCachedItems(cacheKey, items)
	
	return items, nil
}

// GetUserPlayHistory returns user play history
func (c *Client) GetUserPlayHistory(userID string, daysBack int) ([]media.PlayHistoryItem, error) {
	// Plex doesn't have a direct play history API like Emby
	// This would require more complex implementation or return empty for now
	return []media.PlayHistoryItem{}, nil
}

// Session control methods

// PauseSession pauses a Plex session
func (c *Client) PauseSession(sessionID string) error {
	endpoint := fmt.Sprintf("/player/playback/pause?sessionId=%s", sessionID)
	resp, err := c.doRequest(endpoint)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// UnpauseSession resumes a Plex session
func (c *Client) UnpauseSession(sessionID string) error {
	endpoint := fmt.Sprintf("/player/playback/play?sessionId=%s", sessionID)
	resp, err := c.doRequest(endpoint)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// StopSession stops a Plex session
func (c *Client) StopSession(sessionID string) error {
	endpoint := fmt.Sprintf("/player/playback/stop?sessionId=%s", sessionID)
	resp, err := c.doRequest(endpoint)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// SendMessage sends a message to a Plex session
func (c *Client) SendMessage(sessionID, header, text string, timeoutMs int) error {
	values := url.Values{}
	values.Set("type", "message")
	values.Set("header", header)
	values.Set("message", text)
	
	endpoint := fmt.Sprintf("/player/timeline/notify?sessionId=%s&%s", sessionID, values.Encode())
	resp, err := c.doRequest(endpoint)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// CheckHealth checks Plex server health
func (c *Client) CheckHealth() (*media.ServerHealth, error) {
	start := time.Now()
	
	resp, err := c.doRequest("/")
	responseTime := time.Since(start).Milliseconds()
	
	health := &media.ServerHealth{
		ServerID:     c.serverID,
		ServerType:   media.ServerTypePlex,
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
	return fmt.Sprintf("plex_items_%x", h.Sum(nil))
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

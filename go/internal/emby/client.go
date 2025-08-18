package emby

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// --- robust helpers for JSON responses ---

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
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

type Client struct {
	BaseURL string
	APIKey  string
	http    *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

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

// ItemsByIDs fetches item details for a set of IDs (used to prettify Episode display)
func (c *Client) ItemsByIDs(ids []string) ([]EmbyItem, error) {
	if c == nil || c.BaseURL == "" || c.APIKey == "" || len(ids) == 0 {
		return []EmbyItem{}, nil
	}
	endpoint := fmt.Sprintf("%s/emby/Items", c.BaseURL)
	q := url.Values{}
	q.Set("api_key", c.APIKey)
	q.Set("Ids", strings.Join(ids, ","))
	// Ask for fields we care about; Emby returns lots by default, this is fine either way.

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
	return out.Items, nil
}

type LibraryItem struct {
	Id     string `json:"Id"`
	Name   string `json:"Name"`
	Type   string `json:"Type"`
	Height *int   `json:"Height,omitempty"`
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

// Enhanced GetItemsChunk that extracts codec data from MediaStreams
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

	// Convert to LibraryItem format, creating separate entries for each codec
	var result []LibraryItem

	for _, item := range out.Items {
		videoCodecs := make(map[string]*int) // codec -> height
		audioCodecs := make(map[string]bool)

		// Extract ALL codecs from MediaStreams
		for _, source := range item.MediaSources {
			for _, stream := range source.MediaStreams {
				if stream.Type == "Video" && stream.Codec != "" {
					if _, exists := videoCodecs[stream.Codec]; !exists {
						videoCodecs[stream.Codec] = stream.Height
					}
				} else if stream.Type == "Audio" && stream.Codec != "" {
					audioCodecs[stream.Codec] = true
				}
			}
		}

		// Create separate LibraryItem entries for each video codec
		for codec, height := range videoCodecs {
			result = append(result, LibraryItem{
				Id:     item.Id + "_v_" + codec,
				Name:   item.Name,
				Type:   item.Type,
				Height: height,
				Codec:  codec,
			})
		}

		// Create separate LibraryItem entries for each audio codec
		for codec := range audioCodecs {
			result = append(result, LibraryItem{
				Id:    item.Id + "_a_" + codec,
				Name:  item.Name,
				Type:  item.Type,
				Codec: codec,
			})
		}

		// If no codecs found, create Unknown entry
		if len(videoCodecs) == 0 && len(audioCodecs) == 0 {
			result = append(result, LibraryItem{
				Id:    item.Id,
				Name:  item.Name,
				Type:  item.Type,
				Codec: "Unknown",
			})
		}
	}

	return result, nil
}

type EmbyUser struct {
	Id   string `json:"Id"`
	Name string `json:"Name"`
}

// Flattened shape consumed by handlers (now.go expects rich fields)
type EmbySession struct {
	UserID   string `json:"UserId"`
	UserName string `json:"UserName"`

	// Now playing item
	ItemID        string `json:"NowPlayingItemId"`
	ItemName      string `json:"NowPlayingItemName,omitempty"`
	ItemType      string `json:"NowPlayingItemType,omitempty"`
	DurationTicks int64  `json:"RunTimeTicks"` // item runtime
	PosTicks      int64  `json:"PositionTicks"`

	// Client/device
	App    string `json:"Client"`
	Device string `json:"DeviceName"`

	// Playback details
	PlayMethod string `json:"PlayMethod,omitempty"` // "Direct" / "Transcode" (if available)
	VideoCodec string `json:"VideoCodec,omitempty"`
	AudioCodec string `json:"AudioCodec,omitempty"`
	SubsCount  int    `json:"SubsCount,omitempty"`
	Bitrate    int64  `json:"Bitrate,omitempty"`
}

type rawSession struct {
	UserID     string `json:"UserId"`
	UserName   string `json:"UserName"`
	Client     string `json:"Client"`
	DeviceName string `json:"DeviceName"`

	NowPlayingItem *struct {
		Id           string `json:"Id"`
		Name         string `json:"Name"`
		Type         string `json:"Type"`
		RunTimeTicks int64  `json:"RunTimeTicks"`
		MediaStreams []struct {
			Type     string `json:"Type"`  // "Video","Audio","Subtitle"
			Codec    string `json:"Codec"` // e.g. "h264","aac"
			IsText   bool   `json:"IsText"`
			Language string `json:"Language"`
		} `json:"MediaStreams"`
		// Fallback source-level bitrate (bps) for direct play
		MediaSources []struct {
			Bitrate int64 `json:"Bitrate"`
		} `json:"MediaSources"`
	} `json:"NowPlayingItem"`

	PlayState *struct {
		PositionTicks int64  `json:"PositionTicks"`
		PlayMethod    string `json:"PlayMethod"` // often present ("DirectPlay"/"Transcode")
	} `json:"PlayState"`

	// Present when transcoding; contains bitrate/codecs being produced
	TranscodingInfo *struct {
		Bitrate    int64  `json:"Bitrate"`
		VideoCodec string `json:"VideoCodec"`
		AudioCodec string `json:"AudioCodec"`
	} `json:"TranscodingInfo"`
}

func (c *Client) GetActiveSessions() ([]EmbySession, error) {
	u := fmt.Sprintf("%s/emby/Sessions", c.BaseURL)
	q := url.Values{}
	q.Set("api_key", c.APIKey)

	req, _ := http.NewRequest("GET", u+"?"+q.Encode(), nil)
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
		// Show ONLY active playback: must have an item
		if rs.NowPlayingItem == nil || rs.NowPlayingItem.Id == "" {
			continue
		}

		es := EmbySession{
			UserID:   rs.UserID,
			UserName: rs.UserName,
			App:      rs.Client,
			Device:   rs.DeviceName,
		}

		// Item + duration
		es.ItemID = rs.NowPlayingItem.Id
		es.ItemName = rs.NowPlayingItem.Name
		es.ItemType = rs.NowPlayingItem.Type
		es.DurationTicks = rs.NowPlayingItem.RunTimeTicks

		// Streams -> codecs + subs count
		subs := 0
		for _, ms := range rs.NowPlayingItem.MediaStreams {
			switch strings.ToLower(ms.Type) {
			case "video":
				if es.VideoCodec == "" && ms.Codec != "" {
					es.VideoCodec = strings.ToUpper(ms.Codec)
				}
			case "audio":
				if es.AudioCodec == "" && ms.Codec != "" {
					es.AudioCodec = strings.ToUpper(ms.Codec)
				}
			case "subtitle":
				subs++
			}
		}
		es.SubsCount = subs

		// PlayState -> position (and possibly method)
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

		// Bitrate:
		// 1) If transcoding, prefer TranscodingInfo.Bitrate
		if rs.TranscodingInfo != nil && rs.TranscodingInfo.Bitrate > 0 {
			es.Bitrate = rs.TranscodingInfo.Bitrate
			if rs.TranscodingInfo.VideoCodec != "" {
				es.VideoCodec = strings.ToUpper(rs.TranscodingInfo.VideoCodec)
			}
			if rs.TranscodingInfo.AudioCodec != "" {
				es.AudioCodec = strings.ToUpper(rs.TranscodingInfo.AudioCodec)
			}
			es.PlayMethod = "Transcode"
		} else {
			// 2) Fallback for direct play: first media source bitrate if present
			if es.Bitrate == 0 && rs.NowPlayingItem != nil && len(rs.NowPlayingItem.MediaSources) > 0 {
				if rs.NowPlayingItem.MediaSources[0].Bitrate > 0 {
					es.Bitrate = rs.NowPlayingItem.MediaSources[0].Bitrate
				}
			}
		}

		out = append(out, es)
	}


		// PlayState -> position (and possibly play method)
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

		// Transcoding info -> bitrate and codecs override + method
		if rs.TranscodingInfo != nil {
			if rs.TranscodingInfo.Bitrate > 0 {
				es.Bitrate = rs.TranscodingInfo.Bitrate
			}
			// If transcoding advertises codecs, use them
			if rs.TranscodingInfo.VideoCodec != "" {
				es.VideoCodec = strings.ToUpper(rs.TranscodingInfo.VideoCodec)
			}
			if rs.TranscodingInfo.AudioCodec != "" {
				es.AudioCodec = strings.ToUpper(rs.TranscodingInfo.AudioCodec)
			}
			es.PlayMethod = "Transcode"
		}

		// Default play method if unknown
		if es.PlayMethod == "" {
			es.PlayMethod = "Direct"
		}

		out = append(out, es)
	}
	return out, nil
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

type usersResp struct {
	Items []EmbyUser `json:"Items"`
}

// GetUsers fetches minimal user data (Id, Name) from Emby server
func (c *Client) GetUsers() ([]EmbyUser, error) {
	u := fmt.Sprintf("%s/emby/Users", c.BaseURL)
	q := url.Values{}
	q.Set("api_key", c.APIKey)
	q.Set("Fields", "") // minimal fields

	req, _ := http.NewRequest("GET", u+"?"+q.Encode(), nil)
	req.Header.Set("X-Emby-Token", c.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	// Try to parse as direct array first
	var users []EmbyUser
	if err := readJSON(resp, &users); err != nil {
		// If that fails, try the wrapped format
		var out usersResp
		if err := readJSON(resp, &out); err != nil {
			return nil, err
		}
		return out.Items, nil
	}
	return users, nil
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

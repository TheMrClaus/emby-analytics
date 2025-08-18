package emby

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a minimal Emby API client used by the analytics backend.
type Client struct {
	BaseURL string
	APIKey  string

	http *http.Client
}

// New returns a new Emby client.
func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// -------- Data shapes --------

// Flattened shape consumed by handlers (now.go expects these fields).
type EmbySession struct {
	SessionID string `json:"SessionId"` // Emby session id (for controls)

	UserID   string `json:"UserId"`
	UserName string `json:"UserName"`

	// Now playing item
	ItemID        string `json:"NowPlayingItemId"`
	ItemName      string `json:"NowPlayingItemName,omitempty"`
	ItemType      string `json:"NowPlayingItemType,omitempty"`
	DurationTicks int64  `json:"RunTimeTicks"`  // total runtime (100ns ticks)
	PosTicks      int64  `json:"PositionTicks"` // current position (100ns ticks)

	// Client/device
	App    string `json:"Client"`
	Device string `json:"DeviceName"`

	// Playback details
	PlayMethod string `json:"PlayMethod,omitempty"` // "Direct" / "Transcode"
	VideoCodec string `json:"VideoCodec,omitempty"`
	AudioCodec string `json:"AudioCodec,omitempty"`
	SubsCount  int    `json:"SubsCount,omitempty"`
	Bitrate    int64  `json:"Bitrate,omitempty"` // bps
}

// Raw session as delivered by /emby/Sessions.
type rawSession struct {
	Id         string `json:"Id"` // session id
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
			Codec    string `json:"Codec"` // e.g. h264,aac
			Language string `json:"Language"`

			// Some servers expose stream bit rate on the stream objects (usually in kbps).
			BitRate int64 `json:"BitRate,omitempty"` // note capital R variant appears frequently
			Bitrate int64 `json:"Bitrate,omitempty"`
		} `json:"MediaStreams"`

		// Direct-play fallback bitrate; Emby often reports kbps here.
		MediaSources []struct {
			Bitrate int64 `json:"Bitrate"`
		} `json:"MediaSources"`
	} `json:"NowPlayingItem"`

	PlayState *struct {
		PositionTicks int64  `json:"PositionTicks"`
		PlayMethod    string `json:"PlayMethod"`
		IsPaused      bool   `json:"IsPaused"`
	} `json:"PlayState"`

	// Present during transcode; Bitrate is already in bps.
	TranscodingInfo *struct {
		Bitrate    int64  `json:"Bitrate"`
		VideoCodec string `json:"VideoCodec"`
		AudioCodec string `json:"AudioCodec"`
	} `json:"TranscodingInfo"`
}

// -------- Helpers --------

func readJSON(resp *http.Response, v any) error {
	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)
	return dec.Decode(v)
}

// Heuristic: Emby often reports kbps for sources/streams; treat smaller values as kbps.
func normalizeToBps(v int64) int64 {
	if v < 1_000_000 {
		return v * 1000
	}
	return v
}

// -------- Sessions --------

// GetActiveSessions returns only sessions that have a NowPlayingItem.
func (c *Client) GetActiveSessions() ([]EmbySession, error) {
	u := fmt.Sprintf("%s/emby/Sessions", c.BaseURL)
	q := url.Values{}
	q.Set("api_key", c.APIKey)

	req, _ := http.NewRequest("GET", u+"?"+q.Encode(), nil)
	// Some setups prefer header token; keep both for compatibility.
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
		// Only show active playback: must have an item
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

		// Streams -> codecs + subs; also sum stream bitrates (usually kbps)
		subs := 0
		streamKbpsSum := int64(0)
		for _, ms := range rs.NowPlayingItem.MediaStreams {
			switch strings.ToLower(ms.Type) {
			case "video":
				if es.VideoCodec == "" && ms.Codec != "" {
					es.VideoCodec = strings.ToUpper(ms.Codec)
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
				if ms.BitRate > 0 {
					streamKbpsSum += ms.BitRate
				} else if ms.Bitrate > 0 {
					streamKbpsSum += ms.Bitrate
				}
			case "subtitle":
				subs++
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

		// Bitrate selection:
		// 1) TranscodingInfo (bps)
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
			// 2) MediaSource bitrate (often kbps) for direct play
			if rs.NowPlayingItem != nil && len(rs.NowPlayingItem.MediaSources) > 0 {
				if b := rs.NowPlayingItem.MediaSources[0].Bitrate; b > 0 {
					es.Bitrate = normalizeToBps(b)
				}
			}
			// 3) Sum of stream bitrates (kbps) if source bitrate missing
			if es.Bitrate == 0 && streamKbpsSum > 0 {
				es.Bitrate = normalizeToBps(streamKbpsSum)
			}
		}

		out = append(out, es)
	}
	return out, nil
}

// -------- Controls --------

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
	body := map[string]any{
		"Header":    header,
		"Text":      text,
		"TimeoutMs": timeoutMs,
	}
	b, _ := json.Marshal(body)
	u := fmt.Sprintf("%s/emby/Sessions/%s/Message?api_key=%s", c.BaseURL, sessionID, url.QueryEscape(c.APIKey))
	req, _ := http.NewRequest("POST", u, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Emby-Token", c.APIKey)
	_, err := c.http.Do(req)
	return err
}

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

	// Optional: check content-type is JSON-ish (don’t be too strict)
	ct := resp.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(strings.ToLower(ct), "application/json") {
		// still try to decode, but if it fails we’ll show a snippet
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

type itemsResp struct {
	Items []LibraryItem `json:"Items"`
	Total int           `json:"TotalRecordCount"`
}

func (c *Client) TotalItems() (int, error) {
	u := fmt.Sprintf("%s/emby/Items", c.BaseURL)
	q := url.Values{}
	q.Set("api_key", c.APIKey)
	q.Set("Fields", "Height,VideoCodec")
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

func (c *Client) GetItemsChunk(limit, page int) ([]LibraryItem, error) {
	u := fmt.Sprintf("%s/emby/Items", c.BaseURL)
	q := url.Values{}
	q.Set("api_key", c.APIKey)
	q.Set("Fields", "Height,VideoCodec")
	q.Set("Recursive", "true")
	q.Set("StartIndex", fmt.Sprintf("%d", page*limit))
	q.Set("Limit", fmt.Sprintf("%d", limit))

	req, _ := http.NewRequest("GET", u+"?"+q.Encode(), nil)
	req.Header.Set("X-Emby-Token", c.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	var out itemsResp
	if err := readJSON(resp, &out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

type EmbyUser struct {
	Id   string `json:"Id"`
	Name string `json:"Name"`
}

// Flattened shape consumed by handlers (now.go expects ItemName/ItemType)
type EmbySession struct {
	UserID   string `json:"UserId"`
	UserName string `json:"UserName"`
	ItemID   string `json:"NowPlayingItemId"`
	ItemName string `json:"NowPlayingItemName,omitempty"`
	ItemType string `json:"NowPlayingItemType,omitempty"`
	PosMs    int64  `json:"PositionTicks"` // ticks from Emby; handlers convert to ms
}

type rawSession struct {
	UserID   string `json:"UserId"`
	UserName string `json:"UserName"`
	// Emby nests the item + play state
	NowPlayingItem *struct {
		Id   string `json:"Id"`
		Name string `json:"Name"`
		Type string `json:"Type"`
	} `json:"NowPlayingItem"`
	PlayState *struct {
		PositionTicks int64 `json:"PositionTicks"`
	} `json:"PlayState"`
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
		es := EmbySession{
			UserID:   rs.UserID,
			UserName: rs.UserName,
		}
		if rs.NowPlayingItem != nil {
			es.ItemID = rs.NowPlayingItem.Id
			es.ItemName = rs.NowPlayingItem.Name
			es.ItemType = rs.NowPlayingItem.Type
		}
		if rs.PlayState != nil {
			es.PosMs = rs.PlayState.PositionTicks // still ticks; handler divides by 10_000
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

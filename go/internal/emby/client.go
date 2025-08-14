package emby

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

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
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out embyItemsResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
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
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var out itemsResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
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
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out itemsResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

type EmbyUser struct {
	Id   string `json:"Id"`
	Name string `json:"Name"`
}

type EmbySession struct {
	UserID   string `json:"UserId"`
	UserName string `json:"UserName"`
	ItemID   string `json:"NowPlayingItemId"`
	PosMs    int64  `json:"PositionTicks"`
}

// GetActiveSessions fetches currently active plays
func (c *Client) GetActiveSessions() ([]EmbySession, error) {
	u := fmt.Sprintf("%s/emby/Sessions", c.BaseURL)
	q := url.Values{}
	q.Set("api_key", c.APIKey)

	req, _ := http.NewRequest("GET", u+"?"+q.Encode(), nil)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out []EmbySession
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

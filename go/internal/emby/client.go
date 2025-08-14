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

package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"yadro.com/course/web/core"
)

type Client struct {
	addr string
}

func NewClient(addr string) *Client {
	return &Client{addr: addr}
}

func (c *Client) Search(phrase string) ([]core.Comic, error) {
	url := fmt.Sprintf("%s/api/search?phrase=%s", c.addr, phrase)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("api request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Error("failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api returned status: %d", resp.StatusCode)
	}

	var data struct {
		Comics []struct {
			ID     int    `json:"id"`
			ImgURL string `json:"url"`
		} `json:"comics"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode api response: %w", err)
	}

	result := make([]core.Comic, len(data.Comics))
	for i, item := range data.Comics {
		result[i] = core.Comic{
			ID:     item.ID,
			ImgURL: item.ImgURL,
		}
	}
	return result, nil
}

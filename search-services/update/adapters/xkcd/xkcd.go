package xkcd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"yadro.com/course/update/core"
)

type Client struct {
	log    *slog.Logger
	client http.Client
	url    string
}

type xkcdResponse struct {
	Num        int    `json:"num"`
	Img        string `json:"img"`
	Title      string `json:"title"`
	SafeTitle  string `json:"safe_title"`
	Transcript string `json:"transcript"`
	Alt        string `json:"alt"`
	Year       string `json:"year"`
	Month      string `json:"month"`
	Day        string `json:"day"`
}

func NewClient(url string, timeout time.Duration, log *slog.Logger) (*Client, error) {
	if url == "" {
		return nil, fmt.Errorf("empty base url specified")
	}
	return &Client{
		client: http.Client{Timeout: timeout},
		log:    log,
		url:    url,
	}, nil
}

func (c Client) Get(ctx context.Context, id int) (core.XKCDInfo, error) {
	path := fmt.Sprintf("/%d/info.0.json", id)
	var resp xkcdResponse
	if err := c.fetchJSON(ctx, path, &resp); err != nil {
		return core.XKCDInfo{}, fmt.Errorf("get comic %d: %w", id, err)
	}
	return convertToXKCDInfo(resp), nil
}

func (c Client) LastID(ctx context.Context) (int, error) {
	var resp xkcdResponse
	if err := c.fetchJSON(ctx, "/info.0.json", &resp); err != nil {
		return 0, fmt.Errorf("get last id: %w", err)
	}
	return resp.Num, nil
}

func (c *Client) fetchJSON(ctx context.Context, path string, target interface{}) error {
	url := c.url + path
	var lastErr error

	for i := range 3 {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return ctx.Err()
			}
			time.Sleep(time.Second * time.Duration(i+1))
			continue
		}

		defer func() {
			if err := resp.Body.Close(); err != nil {
				c.log.Error("failed to close response body", "error", err)
			}
		}()

		if resp.StatusCode == http.StatusOK {
			return json.NewDecoder(resp.Body).Decode(target)
		}

		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("comic not found (404)")
		}

		lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		time.Sleep(time.Second)
	}

	return fmt.Errorf("http request failed after retries: %w", lastErr)
}

func convertToXKCDInfo(resp xkcdResponse) core.XKCDInfo {
	return core.XKCDInfo{
		ID:          resp.Num,
		URL:         resp.Img,
		Title:       resp.Title,
		SafeTitle:   resp.SafeTitle,
		Description: resp.Transcript,
		Alt:         resp.Alt,
		Year:        resp.Year,
		Month:       resp.Month,
		Day:         resp.Day,
	}
}

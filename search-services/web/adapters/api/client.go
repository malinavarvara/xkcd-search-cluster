package api

import (
	"encoding/json"
	"fmt"
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api returned status: %d", resp.StatusCode)
	}

	// Возвращаем структуру-обертку, так как API присылает объект { "comics": [...] }
	var data struct {
		Comics []struct {
			ID     int    `json:"id"`  // Проверь, присылает ли API "num" или "id"
			ImgURL string `json:"url"` // Проверь, присылает ли API "img_url" или "url"
		} `json:"comics"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode api response: %w", err)
	}

	// Мапим данные в слайс доменных моделей
	result := make([]core.Comic, len(data.Comics))
	for i, item := range data.Comics {
		result[i] = core.Comic{
			ID:     item.ID,
			ImgURL: item.ImgURL,
		}
	}
	return result, nil
}

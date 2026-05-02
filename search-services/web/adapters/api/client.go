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
	// Формируем URL к твоему API
	url := fmt.Sprintf("%s/api/search?phrase=%s", c.addr, phrase)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("api request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api returned status: %d", resp.StatusCode)
	}

	// Парсим JSON-ответ, который возвращает SearchHandler[cite: 22]
	var data struct {
		Comics []struct {
			ID  int    `json:"id"`
			URL string `json:"url"`
		} `json:"comics"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	// Мапим во внутреннюю модель
	result := make([]core.Comic, len(data.Comics))
	for i, c := range data.Comics {
		result[i] = core.Comic{ID: c.ID, ImgURL: c.URL}
	}
	return result, nil
}

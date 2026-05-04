package xkcd

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Get(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if want, got := "/123/info.0.json", r.URL.Path; want != got {
				t.Logf("expected path %s, got %s", want, got)
			}
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{
				"num": 123,
				"img": "https://imgs.xkcd.com/comics/test.png",
				"title": "Test Comic",
				"month": "1",
				"day": "1",
				"year": "2023"
			}`))
			require.NoError(t, err)
		}))
		defer server.Close()

		client, err := NewClient(server.URL, time.Second, slog.Default())
		require.NoError(t, err)

		info, err := client.Get(context.Background(), 123)
		require.NoError(t, err)
		assert.Equal(t, 123, info.ID)
		assert.Equal(t, "Test Comic", info.Title)
	})

	t.Run("NotFound_404", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client, err := NewClient(server.URL, time.Second, slog.Default())
		require.NoError(t, err)

		_, err = client.Get(context.Background(), 999)
		assert.Error(t, err, "expected error on 404")
	})
}

func TestClient_LastID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"num": 2500}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, time.Second, slog.Default())
	require.NoError(t, err)

	id, err := client.LastID(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2500, id)
}

func TestClient_Retries(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"num": 1}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, 100*time.Millisecond, slog.Default())
	require.NoError(t, err)

	_, err = client.LastID(context.Background())
	assert.NoError(t, err, "expected success after retry")
	assert.GreaterOrEqual(t, attempts, 2, "expected at least 2 attempts")
}

func TestClient_EdgeCases(t *testing.T) {
	t.Run("Empty_URL", func(t *testing.T) {
		_, err := NewClient("", time.Second, slog.Default())
		assert.Error(t, err, "expected error for empty URL")
	})

	t.Run("Invalid_JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{invalid json`))
			require.NoError(t, err)
		}))
		defer server.Close()

		client, err := NewClient(server.URL, time.Second, slog.Default())
		require.NoError(t, err)

		_, err = client.LastID(context.Background())
		assert.Error(t, err, "expected error on invalid JSON")
	})

	t.Run("All_Retries_Fail", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusGatewayTimeout)
		}))
		defer server.Close()

		client, err := NewClient(server.URL, 100*time.Millisecond, slog.Default())
		require.NoError(t, err)

		_, err = client.LastID(context.Background())
		assert.Error(t, err, "expected error after all retries failed")
	})
}

func TestClient_NetworkError(t *testing.T) {
	client, err := NewClient("http://localhost:0", time.Second, slog.Default())
	require.NoError(t, err)

	_, err = client.LastID(context.Background())
	assert.Error(t, err, "expected network error")
}

func TestClient_ContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, 2*time.Second, slog.Default())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = client.LastID(ctx)
	assert.Error(t, err, "expected context cancel error")
}

func TestClient_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, 50*time.Millisecond, slog.Default())
	require.NoError(t, err)

	_, err = client.LastID(context.Background())
	assert.Error(t, err, "expected timeout error")
}

type errorClosingBody struct {
	io.ReadCloser
}

func (b *errorClosingBody) Close() error {
	return errors.New("close error")
}

type errorClosingTransport struct{}

func (t *errorClosingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       &errorClosingBody{ReadCloser: io.NopCloser(strings.NewReader(`{"num":1}`))},
	}, nil
}

func TestClient_CloseBodyError(t *testing.T) {
	transport := &errorClosingTransport{}
	client := &Client{
		client: http.Client{Transport: transport},
		log:    slog.Default(),
		url:    "http://example.com",
	}
	var target xkcdResponse
	err := client.fetchJSON(context.Background(), "/info.0.json", &target)
	assert.NoError(t, err, "expected no error from fetchJSON")
	assert.Equal(t, 1, target.Num)
}

func TestClient_InvalidRequestURL(t *testing.T) {
	client := &Client{
		client: http.Client{Timeout: time.Second},
		log:    slog.Default(),
		url:    "http://\n",
	}
	var target xkcdResponse
	err := client.fetchJSON(context.Background(), "/info.0.json", &target)
	assert.Error(t, err, "expected error due to invalid URL")
}

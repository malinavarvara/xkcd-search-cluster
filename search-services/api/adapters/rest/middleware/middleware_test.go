package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockVerifier struct {
	err error
}

func (m *mockVerifier) Verify(token string) error {
	return m.err
}

func TestAuth(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	t.Run("missing auth header", func(t *testing.T) {
		nextCalled = false
		verifier := &mockVerifier{}
		handler := Auth(next, verifier)
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.False(t, nextCalled)
	})

	t.Run("wrong auth header prefix", func(t *testing.T) {
		nextCalled = false
		verifier := &mockVerifier{}
		handler := Auth(next, verifier)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer token")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.False(t, nextCalled)
	})

	t.Run("verification fails", func(t *testing.T) {
		nextCalled = false
		verifier := &mockVerifier{err: errors.New("invalid")}
		handler := Auth(next, verifier)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Token abc123")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.False(t, nextCalled)
	})

	t.Run("success", func(t *testing.T) {
		nextCalled = false
		verifier := &mockVerifier{err: nil}
		handler := Auth(next, verifier)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Token valid")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.True(t, nextCalled)
	})
}

func TestConcurrency(t *testing.T) {
	t.Run("limit <= 0 returns next unchanged", func(t *testing.T) {
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})
		handler := Concurrency(next, 0)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		handler.ServeHTTP(rr, req)
		assert.True(t, nextCalled)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("limits concurrent requests", func(t *testing.T) {
		limit := 2
		var mu sync.Mutex
		active := 0
		maxActive := 0

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			active++
			if active > maxActive {
				maxActive = active
			}
			mu.Unlock()
			time.Sleep(50 * time.Millisecond)
			mu.Lock()
			active--
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		})

		handler := Concurrency(next, limit)
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				rr := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/", nil)
				handler.ServeHTTP(rr, req)
			}()
		}
		wg.Wait()
		assert.Equal(t, limit, maxActive, "max concurrent requests should be limited")
	})

	t.Run("returns 503 when semaphore can't acquire", func(t *testing.T) {
		limit := 1
		block := make(chan struct{})
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-block
		})
		handler := Concurrency(next, limit)

		req1 := httptest.NewRequest("GET", "/", nil)
		rr1 := httptest.NewRecorder()
		go handler.ServeHTTP(rr1, req1)
		time.Sleep(20 * time.Millisecond)

		req2 := httptest.NewRequest("GET", "/", nil)
		rr2 := httptest.NewRecorder()
		handler.ServeHTTP(rr2, req2)
		assert.Equal(t, http.StatusServiceUnavailable, rr2.Code)

		close(block)
	})
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	rw := &responseWriter{ResponseWriter: httptest.NewRecorder(), code: http.StatusOK, wroteHeader: false}
	rw.WriteHeader(http.StatusAccepted)
	assert.Equal(t, http.StatusAccepted, rw.code)
	assert.True(t, rw.wroteHeader)
	rw.WriteHeader(http.StatusBadRequest)
	assert.Equal(t, http.StatusAccepted, rw.code)
}

func TestResponseWriter_Write(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, code: http.StatusOK, wroteHeader: false}
	n, err := rw.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.True(t, rw.wroteHeader)
	assert.Equal(t, http.StatusOK, rw.code)
	assert.Equal(t, "hello", rec.Body.String())
}

func TestRate(t *testing.T) {
	t.Run("rps <= 0 returns next unchanged", func(t *testing.T) {
		called := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		})
		handler := Rate(next, 0)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		handler.ServeHTTP(rr, req)
		assert.True(t, called)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("success", func(t *testing.T) {
		called := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		})
		handler := Rate(next, 10)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		handler.ServeHTTP(rr, req)
		assert.True(t, called)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("context cancel causes early return", func(t *testing.T) {
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
		})
		handler := Rate(next, 1)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.False(t, nextCalled)
	})
}

func TestWithMetrics(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, err := w.Write([]byte("ok"))
		require.NoError(t, err)
	})

	handler := WithMetrics(next)
	req := httptest.NewRequest("GET", "/test/metrics", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	metricName := `http_request_duration_seconds{status="201",url="/test/metrics"}`
	hist := metrics.GetOrCreateHistogram(metricName)
	assert.NotNil(t, hist)
}

func TestWithMetrics_Duration(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	handler := WithMetrics(next)
	req := httptest.NewRequest("GET", "/slow", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	metricName := `http_request_duration_seconds{status="200",url="/slow"}`
	hist := metrics.GetOrCreateHistogram(metricName)
	assert.NotNil(t, hist)
}

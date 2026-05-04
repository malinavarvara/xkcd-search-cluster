package rest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"yadro.com/course/api/core"
)

type MockNormalizer struct{ mock.Mock }

func (m *MockNormalizer) Norm(ctx context.Context, p string) ([]string, error) {
	args := m.Called(ctx, p)
	return args.Get(0).([]string), args.Error(1)
}

type MockAuthenticator struct {
	mock.Mock
}

func (m *MockAuthenticator) Login(user, pass string) (string, error) {
	args := m.Called(user, pass)
	return args.String(0), args.Error(1)
}

func (m *MockAuthenticator) Verify(token string) error {
	args := m.Called(token)
	return args.Error(0)
}

type MockUpdateClient struct{ mock.Mock }

func (m *MockUpdateClient) Ping(ctx context.Context) error { return m.Called(ctx).Error(0) }
func (m *MockUpdateClient) Stats(ctx context.Context) (core.UpdateStats, error) {
	args := m.Called(ctx)
	return args.Get(0).(core.UpdateStats), args.Error(1)
}
func (m *MockUpdateClient) Update(ctx context.Context) error { return m.Called(ctx).Error(0) }
func (m *MockUpdateClient) Status(ctx context.Context) (core.UpdateStatus, error) {
	args := m.Called(ctx)
	return args.Get(0).(core.UpdateStatus), args.Error(1)
}
func (m *MockUpdateClient) Drop(ctx context.Context) error { return m.Called(ctx).Error(0) }

type MockSearcher struct {
	mock.Mock
}

func (m *MockSearcher) BuildIndex(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockSearcher) Search(ctx context.Context, p string, l int) ([]core.Comics, int, error) {
	args := m.Called(ctx, p, l)
	return args.Get(0).([]core.Comics), args.Int(1), args.Error(2)
}

func (m *MockSearcher) Ping(ctx context.Context) error { return m.Called(ctx).Error(0) }
func (m *MockSearcher) ISearch(ctx context.Context, p string, l int) ([]core.Comics, int, error) {
	args := m.Called(ctx, p, l)
	return args.Get(0).([]core.Comics), args.Int(1), args.Error(2)
}

func TestNewWordsHandler(t *testing.T) {
	mockNorm := new(MockNormalizer)
	handler := NewWordsHandler(slog.Default(), mockNorm)

	t.Run("Success", func(t *testing.T) {
		mockNorm.On("Norm", mock.Anything, "test phrase").Return([]string{"test", "phrase"}, nil).Once()
		req := httptest.NewRequest("GET", "/api/words?phrase=test+phrase", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.JSONEq(t, `{"words":["test","phrase"], "total": 2}`, rr.Body.String())
	})

	t.Run("Empty Phrase", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/words?phrase=", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Error Timeout", func(t *testing.T) {
		mockNorm.On("Norm", mock.Anything, "err").Return([]string{}, core.ErrRequestTimeout).Once()
		req := httptest.NewRequest("GET", "/api/words?phrase=err", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusGatewayTimeout, rr.Code)
	})
}

func TestNewStatsHandler(t *testing.T) {
	mockUpdate := new(MockUpdateClient)
	handler := NewStatsHandler(slog.Default(), mockUpdate)

	t.Run("Success", func(t *testing.T) {
		mockUpdate.On("Stats", mock.Anything).Return(core.UpdateStats{WordsTotal: 10}, nil).Once()
		req := httptest.NewRequest("GET", "/api/db/stats", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), `"words_total":10`)
	})

	t.Run("Error", func(t *testing.T) {
		mockUpdate.On("Stats", mock.Anything).Return(core.UpdateStats{}, errors.New("fail")).Once()
		req := httptest.NewRequest("GET", "/api/db/stats", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestNewDropHandler(t *testing.T) {
	mockUpdate := new(MockUpdateClient)
	handler := NewDropHandler(slog.Default(), mockUpdate)

	t.Run("Success", func(t *testing.T) {
		mockUpdate.On("Drop", mock.Anything).Return(nil).Once()
		req := httptest.NewRequest("POST", "/api/db/drop", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestNewPingHandler_Full(t *testing.T) {
	mockS := new(MockSearcher)
	mockU := new(MockUpdateClient)
	pingers := map[string]core.Pinger{"s": mockS, "u": mockU}
	handler := NewPingHandler(slog.Default(), pingers)

	t.Run("Partial Failure", func(t *testing.T) {
		mockS.On("Ping", mock.Anything).Return(nil).Once()
		mockU.On("Ping", mock.Anything).Return(errors.New("down")).Once()

		req := httptest.NewRequest("GET", "/ping", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), `"s":"ok"`)
		assert.Contains(t, rr.Body.String(), `"u":"unavailable"`)
	})
}

func TestNewUpdateHandler(t *testing.T) {
	mockUpdate := new(MockUpdateClient)
	logger := slog.Default()
	handler := NewUpdateHandler(logger, mockUpdate)

	t.Run("Success", func(t *testing.T) {
		mockUpdate.On("Update", mock.Anything).Return(nil).Once()
		req := httptest.NewRequest("POST", "/api/db/update", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("AlreadyRunning", func(t *testing.T) {
		mockUpdate.On("Update", mock.Anything).Return(core.ErrUpdateAlreadyRunning).Once()
		req := httptest.NewRequest("POST", "/api/db/update", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusAccepted, rr.Code)
	})

	t.Run("InternalError", func(t *testing.T) {
		mockUpdate.On("Update", mock.Anything).Return(errors.New("some error")).Once()
		req := httptest.NewRequest("POST", "/api/db/update", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "internal server error")
	})
}

func TestNewStatusHandler(t *testing.T) {
	mockUpdate := new(MockUpdateClient)
	logger := slog.Default()
	handler := NewStatusHandler(logger, mockUpdate)

	t.Run("Success", func(t *testing.T) {
		mockUpdate.On("Status", mock.Anything).Return(core.UpdateStatus("idle"), nil).Once()
		req := httptest.NewRequest("GET", "/api/db/status", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.JSONEq(t, `{"status":"idle"}`, rr.Body.String())
	})

	t.Run("Error", func(t *testing.T) {
		mockUpdate.On("Status", mock.Anything).Return(core.UpdateStatus(""), errors.New("fail")).Once()
		req := httptest.NewRequest("GET", "/api/db/status", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestNewDropHandler_Error(t *testing.T) {
	mockUpdate := new(MockUpdateClient)
	handler := NewDropHandler(slog.Default(), mockUpdate)

	mockUpdate.On("Drop", mock.Anything).Return(errors.New("drop failed")).Once()
	req := httptest.NewRequest("DELETE", "/api/db", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestSearchHandler(t *testing.T) {
	mockSearch := new(MockSearcher)
	handler := NewSearchHandler(slog.Default(), mockSearch)

	t.Run("ServeHTTP - Success", func(t *testing.T) {
		comics := []core.Comics{{ID: 1, ImgURL: "http://a.com"}}
		mockSearch.On("Search", mock.Anything, "cat", 5).Return(comics, 1, nil).Once()
		req := httptest.NewRequest("GET", "/api/search?phrase=cat&limit=5", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.JSONEq(t, `{"comics":[{"id":1,"url":"http://a.com"}],"total":1}`, rr.Body.String())
	})

	t.Run("ServeISearchHTTP - Success", func(t *testing.T) {
		comics := []core.Comics{{ID: 2, ImgURL: "http://b.com"}}
		mockSearch.On("ISearch", mock.Anything, "dog", 10).Return(comics, 1, nil).Once()
		req := httptest.NewRequest("GET", "/api/isearch?phrase=dog", nil)
		rr := httptest.NewRecorder()
		handler.ServeISearchHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.JSONEq(t, `{"comics":[{"id":2,"url":"http://b.com"}],"total":1}`, rr.Body.String())
	})

	t.Run("ServeHTTP - Missing phrase", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/search?limit=5", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "phrase is required")
	})

	t.Run("ServeHTTP - Invalid limit", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/search?phrase=test&limit=abc", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "positive integer")
	})

	t.Run("ServeHTTP - Search error", func(t *testing.T) {
		mockSearch.On("Search", mock.Anything, "err", 10).Return([]core.Comics{}, 0, errors.New("db error")).Once()
		req := httptest.NewRequest("GET", "/api/search?phrase=err", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "search failed")
	})

	t.Run("ServeISearchHTTP - Missing phrase", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/isearch?limit=5", nil)
		rr := httptest.NewRecorder()
		handler.ServeISearchHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("ServeISearchHTTP - Invalid limit", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/isearch?phrase=test&limit=0", nil)
		rr := httptest.NewRecorder()
		handler.ServeISearchHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("ServeISearchHTTP - ISearch error", func(t *testing.T) {
		mockSearch.On("ISearch", mock.Anything, "err", 10).Return([]core.Comics{}, 0, errors.New("index error")).Once()
		req := httptest.NewRequest("GET", "/api/isearch?phrase=err", nil)
		rr := httptest.NewRecorder()
		handler.ServeISearchHTTP(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

}

func TestNewLoginHandler_Errors(t *testing.T) {
	mockAuth := new(MockAuthenticator)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewLoginHandler(mockAuth, logger)

	t.Run("Invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/login", strings.NewReader(`{bad json`))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Wrong credentials", func(t *testing.T) {
		mockAuth.On("Login", "user", "wrongpass").Return("", errors.New("invalid credentials")).Once()
		body := `{"name":"user","password":"wrongpass"}`
		req := httptest.NewRequest("POST", "/api/login", strings.NewReader(body))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestNewWordsHandler_Errors(t *testing.T) {
	mockNorm := new(MockNormalizer)
	logger := slog.Default()
	handler := NewWordsHandler(logger, mockNorm)

	t.Run("PhraseTooLarge", func(t *testing.T) {
		long := strings.Repeat("a", core.MaxPhraseSize+1)
		req := httptest.NewRequest("GET", "/api/words?phrase="+long, nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("ErrServiceUnavailable", func(t *testing.T) {
		mockNorm.On("Norm", mock.Anything, "unavail").Return([]string{}, core.ErrServiceUnavailable).Once()
		req := httptest.NewRequest("GET", "/api/words?phrase=unavail", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	})

	t.Run("ErrInvalidArgument", func(t *testing.T) {
		mockNorm.On("Norm", mock.Anything, "invalid").Return([]string{}, core.ErrInvalidArgument).Once()
		req := httptest.NewRequest("GET", "/api/words?phrase=invalid", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("GenericError", func(t *testing.T) {
		mockNorm.On("Norm", mock.Anything, "generic").Return([]string{}, errors.New("unknown")).Once()
		req := httptest.NewRequest("GET", "/api/words?phrase=generic", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestNewLoginHandler(t *testing.T) {
	mockAuth := new(MockAuthenticator)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewLoginHandler(mockAuth, logger)

	t.Run("Success", func(t *testing.T) {
		mockAuth.On("Login", "user", "pass").Return("token123", nil).Once()
		body := `{"name":"user","password":"pass"}`
		req := httptest.NewRequest("POST", "/api/login", strings.NewReader(body))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "token123", rr.Body.String())
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/login", strings.NewReader(`{bad json`))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Wrong credentials", func(t *testing.T) {
		mockAuth.On("Login", "user", "wrongpass").Return("", errors.New("invalid")).Once()
		body := `{"name":"user","password":"wrongpass"}`
		req := httptest.NewRequest("POST", "/api/login", strings.NewReader(body))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestSearchHandlers(t *testing.T) {
	mockSearch := new(MockSearcher)
	h := &SearchHandler{log: slog.Default(), client: mockSearch}

	t.Run("Search_Success", func(t *testing.T) {
		mockSearch.On("Search", mock.Anything, "cat", 1).
			Return([]core.Comics{{ID: 1, ImgURL: "url"}}, 1, nil).Once()
		req := httptest.NewRequest("GET", "/api/search?phrase=cat&limit=1", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), `"url"`)
	})

	t.Run("ISearch_Error", func(t *testing.T) {
		mockSearch.On("ISearch", mock.Anything, "fail_phrase", 10).
			Return([]core.Comics{}, 0, errors.New("internal error")).Once()
		req := httptest.NewRequest("GET", "/api/isearch?phrase=fail_phrase", nil)
		rr := httptest.NewRecorder()
		h.ServeISearchHTTP(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("ISearch_Success", func(t *testing.T) {
		mockSearch.On("ISearch", mock.Anything, "dog", 10).
			Return([]core.Comics{{ID: 2, ImgURL: "url2"}}, 1, nil).Once()
		req := httptest.NewRequest("GET", "/api/isearch?phrase=dog", nil)
		rr := httptest.NewRecorder()
		h.ServeISearchHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

type errorResponseWriter struct {
	statusCode int
	header     http.Header
	writeError error
}

func (w *errorResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *errorResponseWriter) Write(data []byte) (int, error) {
	if w.writeError != nil {
		return 0, w.writeError
	}
	return len(data), nil
}

func (w *errorResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func TestNewLoginHandler_WriteError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))
	mockAuth := new(MockAuthenticator)
	handler := NewLoginHandler(mockAuth, logger)

	mockAuth.On("Login", "user", "pass").Return("token123", nil).Once()

	w := &errorResponseWriter{writeError: errors.New("write error")}
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"name":"user","password":"pass"}`))
	handler.ServeHTTP(w, req)

	assert.Contains(t, buf.String(), "failed to write token response")
	assert.Contains(t, buf.String(), "write error")
}

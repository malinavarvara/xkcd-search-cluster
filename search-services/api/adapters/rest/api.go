package rest

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"yadro.com/course/api/core"
)

// GET /ping
func NewPingHandler(log core.Logger, pingers map[string]core.Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		replies := make(map[string]string)
		for name, pinger := range pingers {
			err := pinger.Ping(ctx)
			if err != nil {
				replies[name] = "unavailable"
				log.Error("ping failed for service",
					"service", name,
					"error", err,
				)
			} else {
				replies[name] = "ok"
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := struct {
			Replies map[string]string `json:"replies"`
		}{
			Replies: replies,
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Error("failed to encode ping response", "error", err)
		}
	}
}

// GET /api/words?phrase=...
func NewWordsHandler(log core.Logger, normalizer core.Normalizer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		phrase := r.URL.Query().Get("phrase")
		if phrase == "" {
			http.Error(w, core.ErrEmptyPhrase.Error(), http.StatusBadRequest)
			return
		}
		if len(phrase) > core.MaxPhraseSize {
			http.Error(w, core.ErrPhraseTooLarge.Error(), http.StatusBadRequest)
			return
		}

		words, err := normalizer.Norm(r.Context(), phrase)
		if err != nil {
			switch {
			case errors.Is(err, core.ErrPhraseTooLarge):
				http.Error(w, err.Error(), http.StatusBadRequest)
			case errors.Is(err, core.ErrRequestTimeout):
				http.Error(w, "request timeout", http.StatusGatewayTimeout)
			case errors.Is(err, core.ErrServiceUnavailable):
				http.Error(w, "words service unavailable", http.StatusServiceUnavailable)
			case errors.Is(err, core.ErrInvalidArgument):
				http.Error(w, "invalid argument", http.StatusBadRequest)
			default:
				log.Error("normalization failed", "error", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
			return
		}

		resp := struct {
			Words []string `json:"words"`
			Total int      `json:"total"`
		}{
			Words: words,
			Total: len(words),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Error("failed to encode response", "error", err)
		}
	}
}

// GET /api/db/stats
func NewStatsHandler(log core.Logger, updateClient core.UpdateClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := updateClient.Stats(r.Context())
		if err != nil {
			log.Error("failed to get stats", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		resp := struct {
			WordsTotal    int `json:"words_total"`
			WordsUnique   int `json:"words_unique"`
			ComicsFetched int `json:"comics_fetched"`
			ComicsTotal   int `json:"comics_total"`
		}{
			WordsTotal:    stats.WordsTotal,
			WordsUnique:   stats.WordsUnique,
			ComicsFetched: stats.ComicsFetched,
			ComicsTotal:   stats.ComicsTotal,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Error("failed to encode stats response", "error", err)
		}
	}
}

// POST /api/db/update
func NewUpdateHandler(log core.Logger, updateClient core.UpdateClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := updateClient.Update(r.Context())

		w.Header().Set("Content-Type", "application/json")

		if err != nil {
			if errors.Is(err, core.ErrUpdateAlreadyRunning) {
				w.WriteHeader(http.StatusAccepted) // 202
				_, _ = w.Write([]byte(`{"status":"already updating"}`))
				return
			}

			log.Error("failed to run update", "error", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusAccepted) // 202
		_, _ = w.Write([]byte(`{"status":"update started"}`))
	}
}

// GET /api/db/status
func NewStatusHandler(log core.Logger, updateClient core.UpdateClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, err := updateClient.Status(r.Context())
		if err != nil {
			log.Error("failed to get status", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]string{"status": string(status)}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Error("failed to encode status response", "error", err)
		}
	}
}

// DELETE /api/db
func NewDropHandler(log core.Logger, updateClient core.UpdateClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := updateClient.Drop(r.Context())
		if err != nil {
			log.Error("failed to drop db", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

type SearchHandler struct {
	log    *slog.Logger
	client core.Searcher
}

func NewSearchHandler(log *slog.Logger, client core.Searcher) *SearchHandler {
	return &SearchHandler{log: log, client: client}
}

func (h *SearchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	phrase := query.Get("phrase")
	limitStr := query.Get("limit")

	if phrase == "" {
		http.Error(w, "phrase is required", http.StatusBadRequest)
		return
	}

	limit := 10

	if limitStr != "" {
		parsed, err := strconv.ParseInt(limitStr, 10, 32)
		if err != nil || parsed <= 0 {
			http.Error(w, "limit must be a positive integer", http.StatusBadRequest)
			return
		}
		limit = int(parsed)
	}

	comics, _, err := h.client.Search(r.Context(), phrase, limit)
	if err != nil {
		h.log.Error("search failed", "error", err, "phrase", phrase)
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}

	type comicResp struct {
		ID  int32  `json:"id"`
		URL string `json:"url"`
	}

	comicResults := make([]comicResp, 0, len(comics))
	for _, c := range comics {
		comicResults = append(comicResults, comicResp{
			ID:  int32(c.ID),
			URL: c.ImgURL,
		})
	}

	response := struct {
		Comics []comicResp `json:"comics"`
		Total  int         `json:"total"`
	}{
		Comics: comicResults,
		Total:  len(comicResults),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(response); err != nil {
		h.log.Error("failed to encode response", "error", err)
	}
}

func (h *SearchHandler) ServeISearchHTTP(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	phrase := query.Get("phrase")
	limitStr := query.Get("limit")

	if phrase == "" {
		http.Error(w, "phrase is required", http.StatusBadRequest)
		return
	}

	limit := 10

	if limitStr != "" {
		parsed, err := strconv.ParseInt(limitStr, 10, 32)
		if err != nil || parsed <= 0 {
			http.Error(w, "limit must be a positive integer", http.StatusBadRequest)
			return
		}
		limit = int(parsed)
	}

	comics, _, err := h.client.ISearch(r.Context(), phrase, limit)

	if err != nil {
		h.log.Error("search failed", "error", err, "phrase", phrase)
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}

	type comicResp struct {
		ID  int32  `json:"id"`
		URL string `json:"url"`
	}

	comicResults := make([]comicResp, 0, len(comics))
	for _, c := range comics {
		comicResults = append(comicResults, comicResp{
			ID:  int32(c.ID),
			URL: c.ImgURL,
		})
	}

	response := struct {
		Comics []comicResp `json:"comics"`
		Total  int         `json:"total"`
	}{
		Comics: comicResults,
		Total:  len(comicResults),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(response); err != nil {
		h.log.Error("failed to encode response", "error", err)
	}
}

// POST /api/login
func NewLoginHandler(auth core.Authenticator, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var creds struct {
			Name     string `json:"name"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		token, err := auth.Login(creds.Name, creds.Password)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		if _, err := w.Write([]byte(token)); err != nil {
			logger.Error("failed to write token response", "error", err)
		}
	}
}

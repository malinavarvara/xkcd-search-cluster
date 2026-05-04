package middleware

import (
	"net/http"

	"golang.org/x/sync/semaphore"
)

func Concurrency(next http.HandlerFunc, limit int) http.HandlerFunc {
	if limit <= 0 {
		return next
	}
	sem := semaphore.NewWeighted(int64(limit))

	return func(w http.ResponseWriter, r *http.Request) {
		if !sem.TryAcquire(1) {
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			return
		}
		defer sem.Release(1)
		next(w, r)
	}
}

package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/metrics"
)

type responseWriter struct {
	http.ResponseWriter
	code        int
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	if rw.wroteHeader {
		return
	}
	rw.wroteHeader = true
	rw.code = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func WithMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, code: http.StatusOK, wroteHeader: false}
		next.ServeHTTP(rw, r)
		duration := time.Since(start).Seconds()
		metricName := fmt.Sprintf(`http_request_duration_seconds{status="%d",url="%s"}`, rw.code, r.URL.Path)
		hist := metrics.GetOrCreateHistogram(metricName)
		hist.Update(duration)
	})
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"philos-video/internal/metrics"
)

// responseWriter wraps http.ResponseWriter to capture status code and bytes written.
type responseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytes += n
	return n, err
}

func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// MetricsMiddleware records HTTP request metrics for every request.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		pattern := categorizePathPattern(r.URL.Path)

		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		statusStr := fmt.Sprintf("%d", rw.status)

		metrics.HTTPRequestsTotal.WithLabelValues(r.Method, pattern, statusStr).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(r.Method, pattern).Observe(duration)
		metrics.HTTPResponseBytes.WithLabelValues(r.Method, pattern).Add(float64(rw.bytes))
	})
}

func categorizePathPattern(path string) string {
	switch {
	case matchPattern(path, "/api/v1/uploads/", "/chunks/"):
		return "/api/v1/uploads/*/chunks/*"
	case matchPrefix(path, "/api/v1/uploads/") && strings.Count(path, "/") == 4:
		return "/api/v1/uploads/*"
	case matchSuffix(path, "/sessions") && matchPrefix(path, "/api/v1/videos/"):
		return "/api/v1/videos/*/sessions"
	case matchPrefix(path, "/api/v1/sessions/") && matchSuffix(path, "/events"):
		return "/api/v1/sessions/*/events"
	case matchPrefix(path, "/api/v1/videos/"):
		return "/api/v1/videos/*"
	case matchPrefix(path, "/api/v1/live/"):
		return "/api/v1/live/*"
	case matchPrefix(path, "/api/v1/dashboard/"):
		return "/api/v1/dashboard/*"
	case matchPrefix(path, "/api/v1/alerts/"):
		return "/api/v1/alerts/*"
	case matchSuffix(path, ".m3u8") && matchPrefix(path, "/videos/"):
		return "/videos/*/manifest"
	case matchPrefix(path, "/videos/"):
		return "/videos/*/segment"
	case matchSuffix(path, ".m3u8") && matchPrefix(path, "/live/"):
		return "/live/*/manifest"
	case matchPrefix(path, "/live/"):
		return "/live/*/segment"
	case path == "/health" || path == "/health/ready":
		return "/health"
	case path == "/metrics":
		return "/metrics"
	case path == "/":
		return "/"
	default:
		return "/other"
	}
}

func matchPrefix(path, prefix string) bool {
	return strings.HasPrefix(path, prefix)
}

func matchSuffix(path, suffix string) bool {
	return strings.HasSuffix(path, suffix)
}

func matchPattern(path, prefix, infix string) bool {
	return strings.HasPrefix(path, prefix) && strings.Contains(path[len(prefix):], infix)
}

package server

import (
	"log/slog"
	"net/http"
	"path/filepath"
	"time"
)

var mimeTypes = map[string]string{
	".m3u8": "application/vnd.apple.mpegurl",
	".m4s":  "video/iso.bmff",
	".mp4":  "video/mp4",
}

// NewServer builds and returns an HTTP mux with all routes configured.
func NewServer(dir string, port int) *http.ServeMux {
	mux := http.NewServeMux()

	mux.Handle("GET /", withMiddleware(http.HandlerFunc(playerHandler)))
	mux.Handle("GET /videos/", withMiddleware(videoHandler(dir)))

	return mux
}

func playerHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(PlayerHTML)) //nolint:errcheck
}

func videoHandler(dir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strip "/videos/" prefix to get the relative path.
		rel := r.URL.Path[len("/videos/"):]
		ext := filepath.Ext(rel)
		if ct, ok := mimeTypes[ext]; ok {
			w.Header().Set("Content-Type", ct)
		}
		http.ServeFile(w, r, filepath.Join(dir, rel))
	})
}

// withMiddleware wraps a handler with CORS headers and request logging.
func withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Range")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", time.Since(start).String(),
		)
	})
}

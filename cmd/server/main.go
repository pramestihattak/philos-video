package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"philos-video/internal/config"
	"philos-video/internal/database"
	"philos-video/internal/handler"
	"philos-video/internal/middleware"
	"philos-video/internal/qoe"
	"philos-video/internal/repository"
	"philos-video/internal/service"
	"philos-video/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("loading config", "err", err)
		os.Exit(1)
	}

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		slog.Error("connecting to database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		slog.Error("running migrations", "err", err)
		os.Exit(1)
	}

	for _, dir := range []string{
		filepath.Join(cfg.DataDir, "chunks"),
		filepath.Join(cfg.DataDir, "raw"),
		filepath.Join(cfg.DataDir, "hls"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			slog.Error("creating data directory", "dir", dir, "err", err)
			os.Exit(1)
		}
	}

	// Repositories
	videoRepo := repository.NewVideoRepo(db)
	uploadRepo := repository.NewUploadRepo(db)
	jobRepo := repository.NewJobRepo(db)
	sessionRepo := repository.NewSessionRepo(db)
	eventRepo := repository.NewEventRepo(db)

	// Job channel + transcode workers
	jobCh := make(chan string, 100)
	videoSvc := service.NewVideoService(videoRepo, jobRepo)
	uploadSvc := service.NewUploadService(videoRepo, uploadRepo, jobRepo, cfg.DataDir, jobCh)
	transcodeSvc := service.NewTranscodeService(videoRepo, jobRepo, cfg.DataDir)

	w := worker.NewTranscodeWorker(jobRepo, videoRepo, transcodeSvc, jobCh)
	w.Start(context.Background(), cfg.WorkerCount)

	queued, err := jobRepo.ListQueued()
	if err != nil {
		slog.Warn("listing queued jobs", "err", err)
	} else {
		for _, jobID := range queued {
			slog.Info("re-enqueueing job", "job_id", jobID)
			jobCh <- jobID
		}
	}

	// Session service + auth middleware
	sessionSvc, err := service.NewSessionService(sessionRepo, videoRepo, cfg.JWTSecret, cfg.JWTExpiry)
	if err != nil {
		slog.Error("creating session service", "err", err)
		os.Exit(1)
	}
	authMiddleware := middleware.NewAuthMiddleware(sessionSvc, sessionRepo)

	// QoE aggregator
	aggregator := qoe.New(videoRepo)

	// Handlers
	uploadH := handler.NewUploadHandler(uploadSvc)
	videoH := handler.NewVideoHandler(videoSvc)
	sessionH := handler.NewSessionHandler(sessionSvc)
	telemetryH := handler.NewTelemetryHandler(sessionRepo, eventRepo, aggregator)
	dashboardH := handler.NewDashboardHandler(aggregator)
	pageH, err := handler.NewPageHandler(videoSvc)
	if err != nil {
		slog.Error("creating page handler", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()

	// Upload API (public)
	mux.HandleFunc("POST /api/v1/uploads", uploadH.InitUpload)
	mux.HandleFunc("PUT /api/v1/uploads/{upload_id}/chunks/{chunk_number}", uploadH.ReceiveChunk)
	mux.HandleFunc("GET /api/v1/uploads/{upload_id}/status", uploadH.GetStatus)

	// Video API (public)
	mux.HandleFunc("GET /api/v1/videos", videoH.ListVideos)
	mux.HandleFunc("GET /api/v1/videos/{id}", videoH.GetVideo)
	mux.HandleFunc("GET /api/v1/videos/{id}/status", videoH.GetVideoStatus)

	// Session creation (public — returns token)
	mux.HandleFunc("POST /api/v1/videos/{id}/sessions", sessionH.CreateSession)

	// Telemetry (session-validated inside handler)
	mux.HandleFunc("POST /api/v1/sessions/{session_id}/events", telemetryH.PostEvents)

	// Dashboard API (public, read-only)
	mux.HandleFunc("GET /api/v1/dashboard/stats", dashboardH.GetStats)
	mux.HandleFunc("GET /api/v1/dashboard/stats/stream", dashboardH.StatsStream)

	// HLS file serving — protected by JWT middleware
	hlsDir := filepath.Join(cfg.DataDir, "hls")
	hlsHandler := http.StripPrefix("/videos/", mimeHandler(http.FileServer(http.Dir(hlsDir))))
	mux.Handle("GET /videos/", authMiddleware.RequirePlaybackToken(hlsHandler))

	// Pages (public)
	mux.HandleFunc("GET /", pageH.Library)
	mux.HandleFunc("GET /upload", pageH.Upload)
	mux.HandleFunc("GET /dashboard", pageH.Dashboard)
	mux.HandleFunc("GET /watch/{video_id}", pageH.Watch)

	addr := fmt.Sprintf(":%d", cfg.Port)
	slog.Info("server starting", "addr", fmt.Sprintf("http://localhost%s", addr))

	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

func mimeHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, ".m3u8"):
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		case strings.HasSuffix(r.URL.Path, ".m4s"):
			w.Header().Set("Content-Type", "video/iso.bmff")
		case strings.HasSuffix(r.URL.Path, ".mp4"):
			w.Header().Set("Content-Type", "video/mp4")
		}
		next.ServeHTTP(w, r)
	})
}

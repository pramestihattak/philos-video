package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"philos-video/internal/config"
	"philos-video/internal/database"
	"philos-video/internal/handler"
	"philos-video/internal/live"
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

	// 0o700: data dirs are private to the server process.
	for _, dir := range []string{
		filepath.Join(cfg.DataDir, "chunks"),
		filepath.Join(cfg.DataDir, "raw"),
		filepath.Join(cfg.DataDir, "hls"),
		filepath.Join(cfg.DataDir, "live"),
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
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
	streamKeyRepo := repository.NewStreamKeyRepo(db)
	liveStreamRepo := repository.NewLiveStreamRepo(db)

	// Job channel + transcode workers
	jobCh := make(chan string, 100)
	videoSvc := service.NewVideoService(videoRepo, jobRepo, cfg.DataDir)
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

	// Live stream manager + RTMP server
	liveMgr := live.NewManager(streamKeyRepo, liveStreamRepo, videoRepo, cfg.DataDir)
	rtmpSrv := live.NewRTMPServer(liveMgr)
	go func() {
		if err := rtmpSrv.ListenAndServe(cfg.RTMPPort); err != nil {
			slog.Error("RTMP server error", "err", err)
		}
	}()

	// QoE aggregator — wire in live counter so dashboard shows live stream count
	aggregator := qoe.New(videoRepo)
	aggregator.SetLiveCounter(liveMgr)

	// Rate limiters
	uploadLimiter := middleware.NewIPRateLimiter(60, time.Minute)  // 60 uploads/min per IP
	authLimiter := middleware.NewIPRateLimiter(20, time.Minute)    // 20 auth attempts/min per IP

	// Handlers
	uploadH := handler.NewUploadHandler(uploadSvc)
	videoH := handler.NewVideoHandler(videoSvc)
	sessionH := handler.NewSessionHandler(sessionSvc)
	telemetryH := handler.NewTelemetryHandler(sessionRepo, eventRepo, aggregator)
	dashboardH := handler.NewDashboardHandler(aggregator)
	streamKeyH := handler.NewStreamKeyHandler(streamKeyRepo)
	liveH := handler.NewLiveHandler(liveMgr, sessionSvc, sessionRepo)
	pageH, err := handler.NewPageHandler(videoSvc, liveMgr, cfg.GoLivePin, cfg.JWTSecret)
	if err != nil {
		slog.Error("creating page handler", "err", err)
		os.Exit(1)
	}

	goLiveGate := middleware.GoLivePinGate(cfg.GoLivePin, cfg.JWTSecret)
	goLiveAPIGate := middleware.GoLivePinAPIGate(cfg.GoLivePin, cfg.JWTSecret)

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "ok"
		httpStatus := http.StatusOK
		if err := db.PingContext(r.Context()); err != nil {
			slog.Warn("health check: DB ping failed", "err", err)
			dbStatus = "unreachable"
			httpStatus = http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		json.NewEncoder(w).Encode(map[string]string{"status": map[bool]string{true: "ok", false: "error"}[httpStatus == http.StatusOK], "db": dbStatus})
	})

	// Upload API — rate-limited
	mux.Handle("POST /api/v1/uploads", uploadLimiter(http.HandlerFunc(uploadH.InitUpload)))
	mux.HandleFunc("PUT /api/v1/uploads/{upload_id}/chunks/{chunk_number}", uploadH.ReceiveChunk)
	mux.HandleFunc("GET /api/v1/uploads/{upload_id}/status", uploadH.GetStatus)

	// Video API (public)
	mux.HandleFunc("GET /api/v1/videos", videoH.ListVideos)
	mux.HandleFunc("GET /api/v1/videos/{id}", videoH.GetVideo)
	mux.HandleFunc("GET /api/v1/videos/{id}/status", videoH.GetVideoStatus)
	mux.HandleFunc("DELETE /api/v1/videos/{id}", videoH.DeleteVideo)

	// VOD session creation — rate-limited
	mux.Handle("POST /api/v1/videos/{id}/sessions", authLimiter(http.HandlerFunc(sessionH.CreateSession)))

	// Telemetry (session-validated inside handler)
	mux.HandleFunc("POST /api/v1/sessions/{session_id}/events", telemetryH.PostEvents)

	// Dashboard API (public, read-only)
	mux.HandleFunc("GET /api/v1/dashboard/stats", dashboardH.GetStats)
	mux.HandleFunc("GET /api/v1/dashboard/stats/stream", dashboardH.StatsStream)

	// Stream key management (PIN-protected)
	mux.Handle("POST /api/v1/stream-keys", goLiveAPIGate(http.HandlerFunc(streamKeyH.Create)))
	mux.Handle("GET /api/v1/stream-keys", goLiveAPIGate(http.HandlerFunc(streamKeyH.List)))
	mux.Handle("DELETE /api/v1/stream-keys/{id}", goLiveAPIGate(http.HandlerFunc(streamKeyH.Deactivate)))

	// Live stream API (public)
	mux.HandleFunc("GET /api/v1/live", liveH.ListLive)
	mux.HandleFunc("GET /api/v1/live/{stream_id}", liveH.GetStream)
	mux.HandleFunc("GET /api/v1/live/{stream_id}/viewers", liveH.Viewers)
	mux.Handle("POST /api/v1/live/{stream_id}/sessions", authLimiter(http.HandlerFunc(liveH.CreateSession)))
	mux.HandleFunc("POST /api/v1/live/{stream_id}/end", liveH.EndStream)

	// VOD HLS file serving — protected by JWT middleware
	hlsDir := filepath.Join(cfg.DataDir, "hls")
	hlsHandler := http.StripPrefix("/videos/", mimeHandler(http.FileServer(http.Dir(hlsDir))))
	mux.Handle("GET /videos/", authMiddleware.RequirePlaybackToken(hlsHandler))

	// Live HLS file serving — protected by JWT middleware, no-cache
	liveDir := filepath.Join(cfg.DataDir, "live")
	liveHLSHandler := http.StripPrefix("/live/", noCacheHandler(mimeHandler(http.FileServer(http.Dir(liveDir)))))
	mux.Handle("GET /live/", authMiddleware.RequireLiveToken(liveHLSHandler))

	// Pages
	mux.HandleFunc("GET /", pageH.Library)
	mux.HandleFunc("GET /upload", pageH.Upload)
	mux.HandleFunc("GET /dashboard", pageH.Dashboard)
	mux.HandleFunc("GET /watch/{video_id}", pageH.Watch)
	mux.Handle("GET /go-live", goLiveGate(http.HandlerFunc(pageH.GoLive)))
	mux.HandleFunc("GET /go-live/login", pageH.GoLiveLogin)
	mux.HandleFunc("POST /go-live/login", pageH.GoLiveLoginPost)
	mux.HandleFunc("GET /watch-live/{stream_id}", pageH.WatchLive)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}

	// Graceful shutdown on SIGTERM / SIGINT.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("server starting",
			"addr", fmt.Sprintf("http://localhost:%d", cfg.Port),
			"rtmp", fmt.Sprintf("rtmp://localhost:%d/live", cfg.RTMPPort),
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutdown signal received")

	// End all live streams so FFmpeg writes #EXT-X-ENDLIST.
	liveMgr.EndAllStreams()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown", "err", err)
	}
	slog.Info("server stopped")
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
		case strings.HasSuffix(r.URL.Path, ".ts"):
			w.Header().Set("Content-Type", "video/MP2T")
		}
		next.ServeHTTP(w, r)
	})
}

func noCacheHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}

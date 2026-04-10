package main

import (
	"context"
	"database/sql"
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

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"philos-video/internal/alerting"
	"philos-video/internal/config"
	"philos-video/internal/database"
	"philos-video/internal/handler"
	"philos-video/internal/health"
	"philos-video/internal/live"
	"philos-video/internal/logging"
	"philos-video/internal/metrics"
	"philos-video/internal/middleware"
	"philos-video/internal/qoe"
	"philos-video/internal/repository"
	"philos-video/internal/service"
	"philos-video/internal/watchdog"
	"philos-video/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("loading config", "err", err)
		os.Exit(1)
	}

	// Configure structured logging first.
	logging.Setup(cfg.LogLevel, cfg.LogFormat)

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		slog.Error("connecting to database", "err", err)
		os.Exit(1)
	}

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
	userRepo := repository.NewUserRepo(db)
	videoRepo := repository.NewVideoRepo(db)
	uploadRepo := repository.NewUploadRepo(db)
	jobRepo := repository.NewJobRepo(db)
	sessionRepo := repository.NewSessionRepo(db)
	eventRepo := repository.NewEventRepo(db)
	streamKeyRepo := repository.NewStreamKeyRepo(db)
	liveStreamRepo := repository.NewLiveStreamRepo(db)
	commentRepo := repository.NewCommentRepo(db)
	chatMsgRepo := repository.NewChatMessageRepo(db)

	// Comments + live chat
	commentSvc := service.NewCommentService(commentRepo, videoRepo)
	chatHub := service.NewChatHub(chatMsgRepo)

	// Job channel + transcode workers
	jobCh := make(chan string, 100)
	videoSvc := service.NewVideoService(videoRepo, jobRepo, userRepo, cfg.DataDir)
	uploadSvc := service.NewUploadService(videoRepo, uploadRepo, jobRepo, userRepo, cfg.DataDir, jobCh)
	transcodeSvc := service.NewTranscodeService(videoRepo, jobRepo, cfg.DataDir)

	// Use a cancelable context for workers so they drain on shutdown.
	workerCtx, workerCancel := context.WithCancel(context.Background())
	w := worker.NewTranscodeWorker(jobRepo, videoRepo, transcodeSvc, jobCh)
	w.Start(workerCtx, cfg.WorkerCount)

	queued, err := jobRepo.ListQueued()
	if err != nil {
		slog.Warn("listing queued jobs", "err", err)
	} else {
		for _, jobID := range queued {
			slog.Info("re-enqueueing job", "job_id", jobID)
			jobCh <- jobID
		}
	}

	// Session service + HLS playback auth middleware
	sessionSvc, err := service.NewSessionService(sessionRepo, videoRepo, cfg.JWTSecret, cfg.JWTExpiry)
	if err != nil {
		slog.Error("creating session service", "err", err)
		os.Exit(1)
	}
	authMiddleware := middleware.NewAuthMiddleware(sessionSvc, sessionRepo)

	// User session service (browser cookie JWT)
	userSessionSvc, err := service.NewUserSessionService(cfg.SessionCookieSecret, cfg.SessionCookieSecure)
	if err != nil {
		slog.Error("creating user session service", "err", err)
		os.Exit(1)
	}
	userAuthMW := middleware.NewUserAuthMiddleware(userSessionSvc, userRepo)
	requireUser := userAuthMW.RequireUser
	requireUserAPI := userAuthMW.RequireUserAPI
	optionalUser := userAuthMW.OptionalUser

	// OAuth service
	oauthSvc := service.NewOAuthService(cfg)

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

	// Health checker
	healthChecker := health.NewHealthChecker(db, cfg.DataDir, cfg.RTMPPort)

	// Alert engine
	alertEngine := alerting.NewEngine(aggregator)

	// Watchdog
	wd := watchdog.New(liveMgr, jobRepo, cfg.DataDir)

	// Rate limiters
	uploadLimiter  := middleware.NewIPRateLimiter(60, time.Minute) // 60 uploads/min per IP
	authLimiter    := middleware.NewIPRateLimiter(20, time.Minute) // 20 auth attempts/min per IP
	commentLimiter := middleware.NewIPRateLimiter(30, time.Minute) // 30 comment/chat posts/min per IP

	// Handlers
	authH := handler.NewAuthHandler(oauthSvc, userSessionSvc, userRepo, cfg.DefaultUploadQuotaBytes)
	uploadH := handler.NewUploadHandler(uploadSvc)
	videoH := handler.NewVideoHandler(videoSvc)
	sessionH := handler.NewSessionHandler(sessionSvc)
	telemetryH := handler.NewTelemetryHandler(sessionRepo, eventRepo, aggregator)
	dashboardH := handler.NewDashboardHandler(aggregator)
	streamKeyH := handler.NewStreamKeyHandler(streamKeyRepo)
	liveH := handler.NewLiveHandler(liveMgr, sessionSvc, sessionRepo)
	healthH := handler.NewHealthHandler(healthChecker)
	alertH := handler.NewAlertHandler(alertEngine)
	commentH := handler.NewCommentHandler(commentSvc)
	chatH := handler.NewChatHandler(chatHub)
	pageH, err := handler.NewPageHandler(videoSvc, liveMgr)
	if err != nil {
		slog.Error("creating page handler", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()

	// Health checks
	mux.HandleFunc("GET /health", healthH.Liveness)
	mux.HandleFunc("GET /health/ready", healthH.Readiness)

	// Prometheus metrics — gated behind login to avoid exposing internals
	mux.Handle("GET /metrics", requireUser(promhttp.Handler()))

	// OAuth + session
	mux.HandleFunc("GET /auth/google/login", authH.GoogleLogin)
	mux.HandleFunc("GET /auth/google/callback", authH.GoogleCallback)
	mux.HandleFunc("POST /auth/logout", authH.Logout)
	mux.Handle("GET /api/v1/me", optionalUser(http.HandlerFunc(authH.Me)))

	// Upload API — requires login + rate-limited
	mux.Handle("POST /api/v1/uploads", uploadLimiter(requireUserAPI(http.HandlerFunc(uploadH.InitUpload))))
	mux.Handle("PUT /api/v1/uploads/{upload_id}/chunks/{chunk_number}", requireUserAPI(http.HandlerFunc(uploadH.ReceiveChunk)))
	mux.Handle("GET /api/v1/uploads/{upload_id}/status", requireUserAPI(http.HandlerFunc(uploadH.GetStatus)))

	// Video API
	mux.Handle("GET /api/v1/videos", optionalUser(http.HandlerFunc(videoH.ListVideos)))
	mux.Handle("GET /api/v1/videos/{id}", optionalUser(http.HandlerFunc(videoH.GetVideo)))
	mux.Handle("GET /api/v1/videos/{id}/status", optionalUser(http.HandlerFunc(videoH.GetVideoStatus)))
	mux.Handle("DELETE /api/v1/videos/{id}", requireUserAPI(http.HandlerFunc(videoH.DeleteVideo)))
	mux.Handle("PATCH /api/v1/videos/{id}", requireUserAPI(http.HandlerFunc(videoH.UpdateVideo)))

	// VOD session creation — rate-limited (public for non-private videos)
	mux.Handle("POST /api/v1/videos/{id}/sessions", authLimiter(optionalUser(http.HandlerFunc(sessionH.CreateSession))))

	// Comments
	mux.Handle("POST /api/v1/videos/{video_id}/comments", commentLimiter(requireUserAPI(http.HandlerFunc(commentH.AddComment))))
	mux.Handle("GET /api/v1/videos/{video_id}/comments", optionalUser(http.HandlerFunc(commentH.ListComments)))
	mux.Handle("DELETE /api/v1/videos/{video_id}/comments/{comment_id}", requireUserAPI(http.HandlerFunc(commentH.DeleteComment)))

	// Telemetry (session-validated inside handler)
	mux.HandleFunc("POST /api/v1/sessions/{session_id}/events", telemetryH.PostEvents)

	// Dashboard API — requires login
	mux.Handle("GET /api/v1/dashboard/stats", requireUser(http.HandlerFunc(dashboardH.GetStats)))
	mux.Handle("GET /api/v1/dashboard/stats/stream", requireUser(http.HandlerFunc(dashboardH.StatsStream)))

	// Alerts API — requires login
	mux.Handle("GET /api/v1/alerts/active", requireUser(http.HandlerFunc(alertH.Active)))
	mux.Handle("GET /api/v1/alerts/history", requireUser(http.HandlerFunc(alertH.History)))

	// Stream key management (requires login)
	mux.Handle("POST /api/v1/stream-keys", requireUserAPI(http.HandlerFunc(streamKeyH.Create)))
	mux.Handle("GET /api/v1/stream-keys", requireUserAPI(http.HandlerFunc(streamKeyH.List)))
	mux.Handle("DELETE /api/v1/stream-keys/{id}", requireUserAPI(http.HandlerFunc(streamKeyH.Deactivate)))
	mux.Handle("PATCH /api/v1/stream-keys/{id}", requireUserAPI(http.HandlerFunc(streamKeyH.Update)))

	// Live stream API
	mux.HandleFunc("GET /api/v1/live", liveH.ListLive)
	mux.HandleFunc("GET /api/v1/live/{stream_id}", liveH.GetStream)
	mux.HandleFunc("GET /api/v1/live/{stream_id}/viewers", liveH.Viewers)
	mux.Handle("POST /api/v1/live/{stream_id}/sessions", authLimiter(http.HandlerFunc(liveH.CreateSession)))
	mux.Handle("POST /api/v1/live/{stream_id}/end", requireUserAPI(http.HandlerFunc(liveH.EndStream)))

	// Live chat
	mux.Handle("POST /api/v1/live/{stream_id}/chat", commentLimiter(requireUserAPI(http.HandlerFunc(chatH.SendMessage))))
	mux.HandleFunc("GET /api/v1/live/{stream_id}/chat/stream", chatH.ChatStream)
	mux.HandleFunc("GET /api/v1/live/{stream_id}/chat", chatH.ListMessages)

	// VOD HLS file serving — protected by JWT middleware
	hlsDir := filepath.Join(cfg.DataDir, "hls")
	hlsHandler := http.StripPrefix("/videos/", mimeHandler(http.FileServer(http.Dir(hlsDir))))
	mux.Handle("GET /videos/", authMiddleware.RequirePlaybackToken(hlsHandler))

	// Live HLS file serving — protected by JWT middleware, no-cache
	liveDir := filepath.Join(cfg.DataDir, "live")
	liveHLSHandler := http.StripPrefix("/live/", noCacheHandler(mimeHandler(http.FileServer(http.Dir(liveDir)))))
	mux.Handle("GET /live/", authMiddleware.RequireLiveToken(liveHLSHandler))

	// Pages — inject user into context; protected pages use RequireUser middleware
	mux.Handle("GET /login", optionalUser(http.HandlerFunc(pageH.Login)))
	mux.Handle("GET /", optionalUser(http.HandlerFunc(pageH.Library)))
	mux.Handle("GET /upload", requireUser(http.HandlerFunc(pageH.Upload)))
	mux.Handle("GET /dashboard", requireUser(http.HandlerFunc(pageH.Dashboard)))
	mux.Handle("GET /watch/{video_id}", optionalUser(http.HandlerFunc(pageH.Watch)))
	mux.Handle("GET /go-live", requireUser(http.HandlerFunc(pageH.GoLive)))
	mux.Handle("GET /watch-live/{stream_id}", optionalUser(http.HandlerFunc(pageH.WatchLive)))

	// Wrap mux with request ID + metrics + security headers middleware globally.
	rootHandler := securityHeadersMiddleware(middleware.RequestIDMiddleware(middleware.MetricsMiddleware(mux)))

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: rootHandler,
	}

	// Graceful shutdown on SIGTERM / SIGINT.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start background services.
	metrics.StartSystemCollector(ctx, cfg.DataDir, db)
	alertEngine.Start(ctx, func() *alerting.SystemMetrics {
		return buildSysMetrics(db)
	})
	wd.Start(ctx)

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

	// Drain transcode workers gracefully (60s timeout).
	slog.Info("waiting for transcodes to finish")
	workerCancel()
	close(jobCh)
	workerDone := make(chan struct{})
	go func() {
		w.Wait()
		close(workerDone)
	}()
	select {
	case <-workerDone:
		slog.Info("transcodes complete")
	case <-time.After(60 * time.Second):
		slog.Warn("transcodes timeout — some jobs may be incomplete")
	}

	db.Close()
	slog.Info("server stopped")
}

// buildSysMetrics samples system-level data for the alerting engine.
func buildSysMetrics(db *sql.DB) *alerting.SystemMetrics {
	sys := &alerting.SystemMetrics{}
	sys.TranscodeQueueDepth = int(queueDepth(db))
	return sys
}

func queueDepth(db *sql.DB) int64 {
	var n int64
	_ = db.QueryRow(`SELECT COUNT(*) FROM transcode_jobs WHERE status='queued'`).Scan(&n)
	return n
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

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
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

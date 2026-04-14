package main

import (
	"context"
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

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"philos-video/internal/api"
	"philos-video/internal/config"
	"philos-video/internal/database"
	"philos-video/internal/health"
	"philos-video/internal/live"
	"philos-video/internal/logging"
	"philos-video/internal/metrics"
	"philos-video/internal/middleware"
	"philos-video/internal/server"
	"philos-video/internal/storage"
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
		filepath.Join(cfg.DataDir, "thumbnails"),
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			slog.Error("creating data directory", "dir", dir, "err", err)
			os.Exit(1)
		}
	}

	// Storage
	userRepo := storage.NewUserRepo(db)
	videoRepo := storage.NewVideoRepo(db)
	uploadRepo := storage.NewUploadRepo(db)
	jobRepo := storage.NewJobRepo(db)
	sessionRepo := storage.NewSessionRepo(db)
	eventRepo := storage.NewEventRepo(db)
	streamKeyRepo := storage.NewStreamKeyRepo(db)
	liveStreamRepo := storage.NewLiveStreamRepo(db)
	commentRepo := storage.NewCommentRepo(db)
	chatMsgRepo := storage.NewChatMessageRepo(db)

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

	queued, err := jobRepo.ListQueued(context.Background())
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

	// Health checker
	healthChecker := health.NewHealthChecker(db, cfg.DataDir, cfg.RTMPPort)

	// Watchdog
	wd := watchdog.New(liveMgr, jobRepo, cfg.DataDir)

	// API server
	srv := server.New(server.Params{
		VideoSvc:       videoSvc,
		UploadSvc:      uploadSvc,
		SessionSvc:     sessionSvc,
		CommentSvc:     commentSvc,
		ChatHub:        chatHub,
		OAuthSvc:       oauthSvc,
		UserSessionSvc: userSessionSvc,

		StreamKeyRepo: streamKeyRepo,
		SessionRepo:   sessionRepo,
		EventRepo:     eventRepo,
		UserRepo:      userRepo,
		VideoRepo:     videoRepo,

		LiveMgr:       liveMgr,
		HealthChecker: healthChecker,
		UserAuthMW:    userAuthMW,

		DataDir:         cfg.DataDir,
		DefaultQuota:    cfg.DefaultUploadQuotaBytes,
		GoLiveWhitelist: cfg.GoLiveWhitelist,
	})

	// Chi router
	r := chi.NewRouter()

	// Global middleware
	corsOrigins := cfg.CORSOrigins
	if len(corsOrigins) == 0 {
		corsOrigins = []string{"*"}
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   corsOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(middleware.RequestIDMiddleware)
	r.Use(middleware.MetricsMiddleware)
	r.Use(securityHeadersMiddleware)
	r.Use(userAuthMW.OptionalUser) // populates user context from session cookie for all routes

	// Mount all OpenAPI-spec routes (30 endpoints)
	api.HandlerFromMux(srv, r)

	// OAuth redirect endpoints (not in OpenAPI spec — use HTTP redirects, not JSON)
	r.Get("/auth/google/login", srv.GoogleLoginHandler)
	r.Get("/auth/google/callback", srv.GoogleCallbackHandler)

	// Prometheus metrics — gated behind login to avoid exposing internals
	r.Handle("/metrics", userAuthMW.RequireUser(promhttp.Handler()))

	// Thumbnails — public, no auth required (preview images)
	thumbDir := filepath.Join(cfg.DataDir, "thumbnails")
	r.Handle("/thumbnails/*", http.StripPrefix("/thumbnails", http.FileServer(http.Dir(thumbDir))))

	// VOD HLS file serving — protected by JWT middleware
	hlsDir := filepath.Join(cfg.DataDir, "hls")
	hlsHandler := http.StripPrefix("/videos", mimeHandler(http.FileServer(http.Dir(hlsDir))))
	r.Handle("/videos/*", authMiddleware.RequirePlaybackToken(hlsHandler))

	// Live HLS file serving — protected by JWT middleware, no-cache
	liveDir := filepath.Join(cfg.DataDir, "live")
	liveHLSHandler := http.StripPrefix("/live", noCacheHandler(mimeHandler(http.FileServer(http.Dir(liveDir)))))
	r.Handle("/live/*", authMiddleware.RequireLiveToken(liveHLSHandler))

	httpSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: r,
	}

	// Graceful shutdown on SIGTERM / SIGINT.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start background services.
	metrics.StartSystemCollector(ctx, cfg.DataDir, db)
	wd.Start(ctx)

	go func() {
		slog.Info("server starting",
			"addr", fmt.Sprintf("http://localhost:%d", cfg.Port),
			"rtmp", fmt.Sprintf("rtmp://localhost:%d/live", cfg.RTMPPort),
		)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
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

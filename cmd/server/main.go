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

	videoRepo := repository.NewVideoRepo(db)
	uploadRepo := repository.NewUploadRepo(db)
	jobRepo := repository.NewJobRepo(db)

	jobCh := make(chan string, 100)

	videoSvc := service.NewVideoService(videoRepo, jobRepo)
	uploadSvc := service.NewUploadService(videoRepo, uploadRepo, jobRepo, cfg.DataDir, jobCh)
	transcodeSvc := service.NewTranscodeService(videoRepo, jobRepo, cfg.DataDir)

	w := worker.NewTranscodeWorker(jobRepo, videoRepo, transcodeSvc, jobCh)
	w.Start(context.Background(), cfg.WorkerCount)

	// Re-enqueue any jobs that were queued before shutdown
	queued, err := jobRepo.ListQueued()
	if err != nil {
		slog.Warn("listing queued jobs", "err", err)
	} else {
		for _, jobID := range queued {
			slog.Info("re-enqueueing job", "job_id", jobID)
			jobCh <- jobID
		}
	}

	uploadH := handler.NewUploadHandler(uploadSvc)
	videoH := handler.NewVideoHandler(videoSvc)
	pageH, err := handler.NewPageHandler(videoSvc)
	if err != nil {
		slog.Error("creating page handler", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()

	// API
	mux.HandleFunc("POST /api/v1/uploads", uploadH.InitUpload)
	mux.HandleFunc("PUT /api/v1/uploads/{upload_id}/chunks/{chunk_number}", uploadH.ReceiveChunk)
	mux.HandleFunc("GET /api/v1/uploads/{upload_id}/status", uploadH.GetStatus)
	mux.HandleFunc("GET /api/v1/videos", videoH.ListVideos)
	mux.HandleFunc("GET /api/v1/videos/{id}", videoH.GetVideo)
	mux.HandleFunc("GET /api/v1/videos/{id}/status", videoH.GetVideoStatus)

	// HLS file serving
	hlsDir := filepath.Join(cfg.DataDir, "hls")
	mux.Handle("GET /videos/", http.StripPrefix("/videos/", mimeHandler(http.FileServer(http.Dir(hlsDir)))))

	// Pages
	mux.HandleFunc("GET /", pageH.Library)
	mux.HandleFunc("GET /upload", pageH.Upload)
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

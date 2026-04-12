package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"philos-video/internal/metrics"
	"philos-video/internal/middleware"
	"philos-video/internal/service"
)

// maxChunkSize is the maximum accepted size for a single chunk upload (256 MiB).
const maxChunkSize = 256 << 20

// maxThumbnailSize is the maximum thumbnail image size (10 MiB).
const maxThumbnailSize = 10 << 20

type UploadHandler struct {
	svc        *service.UploadService
	dataDir    string
	videoRepo  interface {
		UpdateThumbnailPath(id, path string) error
	}
}

func NewUploadHandler(svc *service.UploadService, dataDir string, videoRepo interface {
	UpdateThumbnailPath(id, path string) error
}) *UploadHandler {
	return &UploadHandler{svc: svc, dataDir: dataDir, videoRepo: videoRepo}
}

// POST /api/v1/uploads
// Body: {"filename": "video.mp4", "title": "My Video", "visibility": "public", "total_chunks": 5, "expected_size": 104857600}
func (h *UploadHandler) InitUpload(w http.ResponseWriter, r *http.Request) {
	user := middleware.CurrentUser(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req struct {
		Filename     string `json:"filename"`
		Title        string `json:"title"`
		Visibility   string `json:"visibility"`
		TotalChunks  int    `json:"total_chunks"`
		ExpectedSize int64  `json:"expected_size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Filename == "" || req.TotalChunks <= 0 {
		http.Error(w, "filename and total_chunks are required", http.StatusBadRequest)
		return
	}

	metrics.UploadsTotal.WithLabelValues("started").Inc()
	metrics.ActiveUploads.Inc()

	id, err := h.svc.InitUpload(r.Context(), user, req.Filename, req.Title, req.Visibility, req.TotalChunks, req.ExpectedSize)
	if err != nil {
		metrics.ActiveUploads.Dec()
		var qe interface{ HTTPStatus() int }
		if errors.As(err, &qe) {
			http.Error(w, "upload quota exceeded", qe.HTTPStatus())
			return
		}
		slog.Error("init upload", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"upload_id": id})
}

// POST /api/v1/uploads/{upload_id}/thumbnail
// Multipart form field: thumbnail (image/*)
func (h *UploadHandler) UploadThumbnail(w http.ResponseWriter, r *http.Request) {
	uploadID := r.PathValue("upload_id")

	r.Body = http.MaxBytesReader(w, r.Body, maxThumbnailSize)
	if err := r.ParseMultipartForm(maxThumbnailSize); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		http.Error(w, "thumbnail field missing", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Only allow image types.
	ct := header.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		http.Error(w, "thumbnail must be an image", http.StatusBadRequest)
		return
	}

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".jpg"
	}
	ext = strings.ToLower(ext)

	thumbsDir := filepath.Join(h.dataDir, "thumbnails")
	if err := os.MkdirAll(thumbsDir, 0o700); err != nil {
		slog.Error("create thumbnails dir", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	thumbPath := filepath.Join(thumbsDir, uploadID+ext)
	f, err := os.Create(thumbPath)
	if err != nil {
		slog.Error("create thumbnail file", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	if _, err := io.Copy(f, file); err != nil {
		slog.Error("write thumbnail", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	relPath := uploadID + ext
	if err := h.videoRepo.UpdateThumbnailPath(uploadID, relPath); err != nil {
		slog.Error("update thumbnail path", "upload_id", uploadID, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"thumbnail_path": relPath})
}

// PUT /api/v1/uploads/{upload_id}/chunks/{chunk_number}
func (h *UploadHandler) ReceiveChunk(w http.ResponseWriter, r *http.Request) {
	uploadID := r.PathValue("upload_id")
	chunkNumber, err := strconv.Atoi(r.PathValue("chunk_number"))
	if err != nil {
		http.Error(w, "invalid chunk_number", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxChunkSize)

	timer := prometheus.NewTimer(metrics.UploadChunkDuration)
	if r.ContentLength > 0 {
		metrics.UploadBytesTotal.Add(float64(r.ContentLength))
	}

	if err := h.svc.ReceiveChunk(r.Context(), uploadID, chunkNumber, r.Body); err != nil {
		slog.Error("receive chunk", "upload_id", uploadID, "chunk", chunkNumber, "err", err)
		timer.ObserveDuration()
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	timer.ObserveDuration()

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/v1/uploads/{upload_id}/status
func (h *UploadHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	uploadID := r.PathValue("upload_id")
	received, total, err := h.svc.GetProgress(uploadID)
	if err != nil {
		slog.Error("get upload progress", "upload_id", uploadID, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"received": received, "total": total})
}

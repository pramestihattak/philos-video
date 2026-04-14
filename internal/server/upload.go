package server

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"philos-video/internal/metrics"
	"philos-video/internal/middleware"
)

const maxChunkSize = 256 << 20   // 256 MiB
const maxThumbnailSize = 10 << 20 // 10 MiB

// InitUpload handles POST /api/v1/uploads.
func (s *Server) InitUpload(w http.ResponseWriter, r *http.Request) {
	user := middleware.CurrentUser(r.Context())
	if user == nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req struct {
		Filename     string `json:"filename"`
		Title        string `json:"title"`
		Visibility   string `json:"visibility"`
		TotalChunks  int    `json:"total_chunks"`
		ExpectedSize int64  `json:"file_size"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.TotalChunks <= 0 {
		writeError(w, "total_chunks is required", http.StatusBadRequest)
		return
	}
	// Use title as filename if filename not provided
	if req.Filename == "" {
		req.Filename = req.Title
	}
	if req.Filename == "" {
		writeError(w, "title is required", http.StatusBadRequest)
		return
	}

	metrics.UploadsTotal.WithLabelValues("started").Inc()
	metrics.ActiveUploads.Inc()

	id, err := s.uploadSvc.InitUpload(r.Context(), user, req.Filename, req.Title, req.Visibility, req.TotalChunks, req.ExpectedSize)
	if err != nil {
		metrics.ActiveUploads.Dec()
		var qe interface{ HTTPStatus() int }
		if errors.As(err, &qe) {
			writeError(w, "upload quota exceeded", qe.HTTPStatus())
			return
		}
		slog.Error("init upload", "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"upload_id":    id,
		"total_chunks": req.TotalChunks,
	}, http.StatusCreated)
}

// ReceiveChunk handles PUT /api/v1/uploads/{upload_id}/chunks/{chunk_number}.
func (s *Server) ReceiveChunk(w http.ResponseWriter, r *http.Request, uploadId string, chunkNumber int) {
	r.Body = http.MaxBytesReader(w, r.Body, maxChunkSize)

	timer := prometheus.NewTimer(metrics.UploadChunkDuration)
	if r.ContentLength > 0 {
		metrics.UploadBytesTotal.Add(float64(r.ContentLength))
	}

	if err := s.uploadSvc.ReceiveChunk(r.Context(), uploadId, chunkNumber, r.Body); err != nil {
		slog.Error("receive chunk", "upload_id", uploadId, "chunk", chunkNumber, "err", err)
		timer.ObserveDuration()
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	timer.ObserveDuration()
	w.WriteHeader(http.StatusNoContent)
}

// GetUploadStatus handles GET /api/v1/uploads/{upload_id}/status.
func (s *Server) GetUploadStatus(w http.ResponseWriter, r *http.Request, uploadId string) {
	received, total, err := s.uploadSvc.GetProgress(r.Context(), uploadId)
	if err != nil {
		slog.Error("get upload progress", "upload_id", uploadId, "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]int{"received": received, "total": total}, http.StatusOK)
}

// UploadThumbnail handles POST /api/v1/uploads/{upload_id}/thumbnail.
func (s *Server) UploadThumbnail(w http.ResponseWriter, r *http.Request, uploadId string) {
	r.Body = http.MaxBytesReader(w, r.Body, maxThumbnailSize)
	if err := r.ParseMultipartForm(maxThumbnailSize); err != nil {
		writeError(w, "invalid form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		writeError(w, "thumbnail field missing", http.StatusBadRequest)
		return
	}
	defer file.Close()

	ct := header.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		writeError(w, "thumbnail must be an image", http.StatusBadRequest)
		return
	}

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".jpg"
	}
	ext = strings.ToLower(ext)

	thumbsDir := filepath.Join(s.dataDir, "thumbnails")
	if err := os.MkdirAll(thumbsDir, 0o700); err != nil {
		slog.Error("create thumbnails dir", "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	thumbPath := filepath.Join(thumbsDir, uploadId+ext)
	f, err := os.Create(thumbPath)
	if err != nil {
		slog.Error("create thumbnail file", "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	if _, err := io.Copy(f, file); err != nil {
		slog.Error("write thumbnail", "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	relPath := uploadId + ext
	if err := s.videoRepo.UpdateThumbnailPath(r.Context(), uploadId, relPath); err != nil {
		slog.Error("update thumbnail path", "upload_id", uploadId, "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

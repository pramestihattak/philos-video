package server

import (
	"errors"
	"log/slog"
	"net/http"

	"philos-video/gen/api"
	"philos-video/internal/metrics"
	"philos-video/internal/middleware"
)

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

	writeJSON(w, api.ResponseUploadInit{
		UploadId:    id,
		TotalChunks: req.TotalChunks,
	}, http.StatusCreated)
}

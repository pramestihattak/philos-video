package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"philos-video/internal/service"
)

// maxChunkSize is the maximum accepted size for a single chunk upload (256 MiB).
const maxChunkSize = 256 << 20

type UploadHandler struct {
	svc *service.UploadService
}

func NewUploadHandler(svc *service.UploadService) *UploadHandler {
	return &UploadHandler{svc: svc}
}

// POST /api/v1/uploads
// Body: {"filename": "video.mp4", "total_chunks": 5}
func (h *UploadHandler) InitUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req struct {
		Filename    string `json:"filename"`
		TotalChunks int    `json:"total_chunks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Filename == "" || req.TotalChunks <= 0 {
		http.Error(w, "filename and total_chunks are required", http.StatusBadRequest)
		return
	}

	id, err := h.svc.InitUpload(r.Context(), req.Filename, req.TotalChunks)
	if err != nil {
		slog.Error("init upload", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"upload_id": id})
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

	if err := h.svc.ReceiveChunk(r.Context(), uploadID, chunkNumber, r.Body); err != nil {
		slog.Error("receive chunk", "upload_id", uploadID, "chunk", chunkNumber, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

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

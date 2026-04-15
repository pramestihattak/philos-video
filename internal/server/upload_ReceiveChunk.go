package server

import (
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"

	"philos-video/internal/metrics"
)

const maxChunkSize = 256 << 20 // 256 MiB

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

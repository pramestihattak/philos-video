package server

import (
	"log/slog"
	"net/http"

	"philos-video/gen/api"
)

// GetUploadStatus handles GET /api/v1/uploads/{upload_id}/status.
func (s *Server) GetUploadStatus(w http.ResponseWriter, r *http.Request, uploadId string) {
	received, total, err := s.uploadSvc.GetProgress(r.Context(), uploadId)
	if err != nil {
		slog.Error("get upload progress", "upload_id", uploadId, "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, api.ResponseUploadProgress{Received: received, Total: total}, http.StatusOK)
}

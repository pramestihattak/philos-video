package server

import (
	"log/slog"
	"net/http"

	"philos-video/gen/api"
)

// ListComments handles GET /api/v1/videos/{video_id}/comments.
func (s *Server) ListComments(w http.ResponseWriter, r *http.Request, videoId string, params api.ListCommentsParams) {
	limit := 20
	offset := 0
	if params.Limit != nil {
		limit = *params.Limit
	}
	if params.Offset != nil {
		offset = *params.Offset
	}

	comments, err := s.commentSvc.ListComments(r.Context(), videoId, limit, offset)
	if err != nil {
		slog.Error("list comments", "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if comments == nil {
		writeJSON(w, []api.ResponseComment{}, http.StatusOK)
		return
	}
	writeJSON(w, toResponseComments(comments), http.StatusOK)
}

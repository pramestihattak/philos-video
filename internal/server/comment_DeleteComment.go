package server

import (
	"log/slog"
	"net/http"

	"philos-video/internal/middleware"
)

// DeleteComment handles DELETE /api/v1/videos/{video_id}/comments/{comment_id}.
func (s *Server) DeleteComment(w http.ResponseWriter, r *http.Request, videoId string, commentId string) {
	user := middleware.CurrentUser(r.Context())
	if user == nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := s.commentSvc.DeleteComment(r.Context(), commentId, user.ID); err != nil {
		slog.Error("delete comment", "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

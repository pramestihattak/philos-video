package server

import (
	"errors"
	"log/slog"
	"net/http"

	"philos-video/internal/api"
	"philos-video/internal/middleware"
	"philos-video/internal/models"
	"philos-video/internal/service"
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
		comments = []*models.Comment{}
	}
	writeJSON(w, comments, http.StatusOK)
}

// AddComment handles POST /api/v1/videos/{video_id}/comments.
func (s *Server) AddComment(w http.ResponseWriter, r *http.Request, videoId string) {
	user := middleware.CurrentUser(r.Context())
	if user == nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	comment, err := s.commentSvc.AddComment(r.Context(), videoId, user.ID, user.Name, user.Picture, req.Body)
	if err != nil {
		var ve *service.ValidationError
		if errors.As(err, &ve) {
			writeError(w, ve.Error(), http.StatusBadRequest)
			return
		}
		slog.Error("add comment", "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, comment, http.StatusCreated)
}

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

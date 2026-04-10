package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"philos-video/internal/middleware"
	"philos-video/internal/models"
	"philos-video/internal/service"
)

type CommentHandler struct {
	svc *service.CommentService
}

func NewCommentHandler(svc *service.CommentService) *CommentHandler {
	return &CommentHandler{svc: svc}
}

// POST /api/v1/videos/{video_id}/comments
func (h *CommentHandler) AddComment(w http.ResponseWriter, r *http.Request) {
	user := middleware.CurrentUser(r.Context())
	videoID := r.PathValue("video_id")

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	comment, err := h.svc.AddComment(r.Context(), videoID, user.ID, user.Name, user.Picture, req.Body)
	if err != nil {
		var ve *service.ValidationError
		if errors.As(err, &ve) {
			http.Error(w, ve.Error(), http.StatusBadRequest)
			return
		}
		slog.Error("add comment", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(comment)
}

// GET /api/v1/videos/{video_id}/comments
func (h *CommentHandler) ListComments(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("video_id")

	limit := 20
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	comments, err := h.svc.ListComments(r.Context(), videoID, limit, offset)
	if err != nil {
		slog.Error("list comments", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if comments == nil {
		comments = []*models.Comment{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comments)
}

// DELETE /api/v1/videos/{video_id}/comments/{comment_id}
func (h *CommentHandler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	user := middleware.CurrentUser(r.Context())
	commentID := r.PathValue("comment_id")

	if err := h.svc.DeleteComment(r.Context(), commentID, user.ID); err != nil {
		slog.Error("delete comment", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

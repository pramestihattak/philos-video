package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"philos-video/internal/models"
	"philos-video/internal/storage"
)

const (
	maxCommentLen = 2000
	maxComments   = 50
)

type CommentService struct {
	comments storage.CommentStorer
	videos   storage.VideoStorer
}

func NewCommentService(comments storage.CommentStorer, videos storage.VideoStorer) *CommentService {
	return &CommentService{comments: comments, videos: videos}
}

func (s *CommentService) AddComment(ctx context.Context, videoID, userID, userName, userPic, body string) (*models.Comment, error) {
	if len(body) == 0 {
		return nil, validationErrorf("comment cannot be empty")
	}
	if len([]rune(body)) > maxCommentLen {
		return nil, validationErrorf("comment exceeds %d characters", maxCommentLen)
	}

	video, err := s.videos.GetByID(ctx, videoID)
	if err != nil || video == nil {
		return nil, validationErrorf("video not found")
	}

	c := &models.Comment{
		ID:        uuid.New().String(),
		VideoID:   videoID,
		UserID:    userID,
		UserName:  userName,
		UserPic:   userPic,
		Body:      body,
		CreatedAt: time.Now(),
	}
	if err := s.comments.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *CommentService) ListComments(ctx context.Context, videoID string, limit, offset int) ([]*models.Comment, error) {
	if limit <= 0 || limit > maxComments {
		limit = maxComments
	}
	if offset < 0 {
		offset = 0
	}
	return s.comments.ListByVideo(ctx, videoID, limit, offset)
}

func (s *CommentService) DeleteComment(ctx context.Context, commentID, userID string) error {
	return s.comments.Delete(ctx, commentID, userID)
}

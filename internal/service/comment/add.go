package comment

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"philos-video/internal/models"
	"philos-video/internal/service"
)

func (s *Service) AddComment(ctx context.Context, videoID, userID, userName, userPic, body string) (*models.Comment, error) {
	if len(body) == 0 {
		return nil, service.NewValidationErrorf("comment cannot be empty")
	}
	if len([]rune(body)) > maxCommentLen {
		return nil, service.NewValidationErrorf("comment exceeds %d characters", maxCommentLen)
	}

	video, err := s.videos.GetByID(ctx, videoID)
	if err != nil || video == nil {
		return nil, service.NewValidationErrorf("video not found")
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
		return nil, fmt.Errorf("creating comment: %w", err)
	}
	return c, nil
}

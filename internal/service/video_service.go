package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"philos-video/internal/models"
	"philos-video/internal/storage"
)

const (
	DefaultVideoPageLimit = 20
	MaxVideoPageLimit     = 100
)

type VideoService struct {
	videos   storage.VideoStorer
	jobs     storage.JobStorer
	userRepo storage.UserStorer
	dataDir  string
}

func NewVideoService(videos storage.VideoStorer, jobs storage.JobStorer, userRepo storage.UserStorer, dataDir string) *VideoService {
	return &VideoService{videos: videos, jobs: jobs, userRepo: userRepo, dataDir: dataDir}
}

type VideoStatus struct {
	Video    *models.Video        `json:"video"`
	Job      *models.TranscodeJob `json:"job,omitempty"`
	Progress float64              `json:"progress"`
}

func (s *VideoService) GetVideo(ctx context.Context, id string) (*models.Video, error) {
	return s.videos.GetByID(ctx, id)
}

// ListVideos returns videos for a user (owner-scoped) or public/unlisted videos if userID is empty.
func (s *VideoService) ListVideos(ctx context.Context, limit, offset int, userID string) ([]*models.Video, error) {
	if limit <= 0 {
		limit = DefaultVideoPageLimit
	}
	if limit > MaxVideoPageLimit {
		limit = MaxVideoPageLimit
	}
	if userID == "" {
		return s.videos.ListPublic(ctx, limit, offset)
	}
	return s.videos.List(ctx, limit, offset, userID)
}

func (s *VideoService) GetVideoStatus(ctx context.Context, id string) (*VideoStatus, error) {
	v, err := s.videos.GetByID(ctx, id)
	if err != nil || v == nil {
		return nil, err
	}

	job, err := s.jobs.GetByVideoID(ctx, id)
	if err != nil {
		return nil, err
	}

	vs := &VideoStatus{Video: v, Job: job}
	if job != nil {
		vs.Progress = job.Progress
	}
	if v.Status == models.VideoStatusReady {
		vs.Progress = 1.0
	}
	return vs, nil
}

// DeleteVideo removes a video and its HLS files. Requires the owning userID.
func (s *VideoService) DeleteVideo(ctx context.Context, id, userID string) error {
	v, err := s.videos.GetByIDForUser(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("looking up video: %w", err)
	}
	if v == nil {
		return nil // not found or not owner — silent no-op
	}

	if err := s.videos.Delete(ctx, id, userID); err != nil {
		return fmt.Errorf("deleting from database: %w", err)
	}

	// Decrement quota usage.
	if v.SizeBytes > 0 && v.UserID != "" {
		if err := s.userRepo.DecUsedBytes(ctx, v.UserID, v.SizeBytes); err != nil {
			slog.Warn("decrementing user used_bytes", "user_id", v.UserID, "err", err)
		}
	}

	hlsDir := filepath.Join(s.dataDir, "hls", id)
	if err := os.RemoveAll(hlsDir); err != nil {
		slog.Warn("removing HLS dir after delete", "path", hlsDir, "err", err)
	}

	slog.Info("video deleted", "video_id", id, "user_id", userID)
	return nil
}

// UpdateVisibility changes the video's visibility, scoped to its owner.
func (s *VideoService) UpdateVisibility(ctx context.Context, id, userID, visibility string) error {
	switch visibility {
	case models.VisibilityPrivate, models.VisibilityUnlisted, models.VisibilityPublic:
		// valid
	default:
		return fmt.Errorf("invalid visibility %q", visibility)
	}
	return s.videos.UpdateVisibility(ctx, id, userID, visibility)
}

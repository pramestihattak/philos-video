package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"philos-video/internal/models"
	"philos-video/internal/repository"
)

const (
	DefaultVideoPageLimit = 20
	MaxVideoPageLimit     = 100
)

type VideoService struct {
	videos  *repository.VideoRepo
	jobs    *repository.JobRepo
	dataDir string
}

func NewVideoService(videos *repository.VideoRepo, jobs *repository.JobRepo, dataDir string) *VideoService {
	return &VideoService{videos: videos, jobs: jobs, dataDir: dataDir}
}

type VideoStatus struct {
	Video    *models.Video        `json:"video"`
	Job      *models.TranscodeJob `json:"job,omitempty"`
	Progress float64              `json:"progress"`
}

func (s *VideoService) GetVideo(id string) (*models.Video, error) {
	return s.videos.GetByID(id)
}

func (s *VideoService) ListVideos(limit, offset int) ([]*models.Video, error) {
	if limit <= 0 {
		limit = DefaultVideoPageLimit
	}
	if limit > MaxVideoPageLimit {
		limit = MaxVideoPageLimit
	}
	return s.videos.List(limit, offset)
}

func (s *VideoService) GetVideoStatus(id string) (*VideoStatus, error) {
	v, err := s.videos.GetByID(id)
	if err != nil || v == nil {
		return nil, err
	}

	job, err := s.jobs.GetByVideoID(id)
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

// DeleteVideo removes a video record and its HLS files on disk.
func (s *VideoService) DeleteVideo(_ context.Context, id string) error {
	v, err := s.videos.GetByID(id)
	if err != nil {
		return fmt.Errorf("looking up video: %w", err)
	}
	if v == nil {
		return nil // already gone
	}

	if err := s.videos.Delete(id); err != nil {
		return fmt.Errorf("deleting from database: %w", err)
	}

	hlsDir := filepath.Join(s.dataDir, "hls", id)
	if err := os.RemoveAll(hlsDir); err != nil {
		slog.Warn("removing HLS dir after delete", "path", hlsDir, "err", err)
	}

	slog.Info("video deleted", "video_id", id)
	return nil
}

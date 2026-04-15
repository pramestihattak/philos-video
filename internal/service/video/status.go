package video

import (
	"context"

	"philos-video/internal/models"
)

func (s *Service) GetVideoStatus(ctx context.Context, id string) (*VideoStatus, error) {
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

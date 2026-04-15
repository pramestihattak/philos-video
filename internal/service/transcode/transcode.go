package transcode

import (
	jobrepo "philos-video/internal/storage/job"
	videorepo "philos-video/internal/storage/video"
)

// Service processes transcode jobs via FFmpeg.
type Service struct {
	videos  videorepo.Repository
	jobs    jobrepo.Repository
	dataDir string
}

// New creates a transcode Service.
func New(videos videorepo.Repository, jobs jobrepo.Repository, dataDir string) *Service {
	return &Service{videos: videos, jobs: jobs, dataDir: dataDir}
}

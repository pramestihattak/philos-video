package video

import (
	"philos-video/internal/models"
	jobrepo "philos-video/internal/storage/job"
	userrepo "philos-video/internal/storage/user"
	videorepo "philos-video/internal/storage/video"
)

const (
	DefaultPageLimit = 20
	MaxPageLimit     = 100
)

// VideoStatus is the response type for the video status endpoint.
type VideoStatus struct {
	Video    *models.Video        `json:"video"`
	Job      *models.TranscodeJob `json:"job,omitempty"`
	Progress float64              `json:"progress"`
}

// Service manages video metadata, visibility, and deletion.
type Service struct {
	videos   videorepo.Repository
	jobs     jobrepo.Repository
	userRepo userrepo.Repository
	dataDir  string
}

// New creates a video Service.
func New(videos videorepo.Repository, jobs jobrepo.Repository, userRepo userrepo.Repository, dataDir string) *Service {
	return &Service{videos: videos, jobs: jobs, userRepo: userRepo, dataDir: dataDir}
}

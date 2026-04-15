package upload

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	jobrepo "philos-video/internal/storage/job"
	uploadrepo "philos-video/internal/storage/upload"
	userrepo "philos-video/internal/storage/user"
	videorepo "philos-video/internal/storage/video"
)

// quotaExceededError is returned when an upload would exceed the user's quota.
// It implements HTTPStatus() so server/upload.go can duck-type it to a 413.
type quotaExceededError struct{}

func (e *quotaExceededError) Error() string  { return "upload quota exceeded" }
func (e *quotaExceededError) HTTPStatus() int { return http.StatusRequestEntityTooLarge }

// ErrQuotaExceeded is the sentinel quota error.
var ErrQuotaExceeded error = &quotaExceededError{}

// Service manages chunked video uploads.
type Service struct {
	videos   videorepo.Repository
	uploads  uploadrepo.Repository
	jobs     jobrepo.Repository
	userRepo userrepo.Repository
	dataDir  string
	jobCh    chan<- string
}

// New creates an upload Service.
func New(
	videos videorepo.Repository,
	uploads uploadrepo.Repository,
	jobs jobrepo.Repository,
	userRepo userrepo.Repository,
	dataDir string,
	jobCh chan<- string,
) *Service {
	return &Service{
		videos:   videos,
		uploads:  uploads,
		jobs:     jobs,
		userRepo: userRepo,
		dataDir:  dataDir,
		jobCh:    jobCh,
	}
}

func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

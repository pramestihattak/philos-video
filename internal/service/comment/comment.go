package comment

import (
	commentrepo "philos-video/internal/storage/comment"
	videorepo "philos-video/internal/storage/video"
)

const (
	maxCommentLen = 2000
	maxComments   = 50
)

// Service manages video comments.
type Service struct {
	comments commentrepo.Repository
	videos   videorepo.Repository
}

// New creates a comment Service.
func New(comments commentrepo.Repository, videos videorepo.Repository) *Service {
	return &Service{comments: comments, videos: videos}
}

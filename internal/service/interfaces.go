package service

import (
	"context"
	"io"
	"net/http"
	"time"

	"philos-video/internal/models"
)

// VideoServicer is the interface for video management operations.
type VideoServicer interface {
	GetVideo(ctx context.Context, id string) (*models.Video, error)
	ListVideos(ctx context.Context, limit, offset int, userID string) ([]*models.Video, error)
	GetVideoStatus(ctx context.Context, id string) (*VideoStatus, error)
	DeleteVideo(ctx context.Context, id, userID string) error
	UpdateVisibility(ctx context.Context, id, userID, visibility string) error
}

// UploadServicer is the interface for upload management operations.
type UploadServicer interface {
	InitUpload(ctx context.Context, user *models.User, filename, title, visibility string, totalChunks int, expectedSize int64) (string, error)
	ReceiveChunk(ctx context.Context, uploadID string, chunkNumber int, data io.Reader) error
	GetProgress(ctx context.Context, uploadID string) (received, total int, err error)
}

// SessionServicer is the interface for playback session operations.
type SessionServicer interface {
	CreateSession(ctx context.Context, videoID, deviceType, userAgent, ipAddress string) (*models.PlaybackSession, string, time.Time, error)
	CreateLiveSession(ctx context.Context, streamID, deviceType, userAgent, ipAddress string) (*models.PlaybackSession, string, time.Time, error)
	ParseToken(tokenStr string) (*PlaybackClaims, error)
}

// CommentServicer is the interface for comment operations.
type CommentServicer interface {
	AddComment(ctx context.Context, videoID, userID, userName, userPic, body string) (*models.Comment, error)
	ListComments(ctx context.Context, videoID string, limit, offset int) ([]*models.Comment, error)
	DeleteComment(ctx context.Context, commentID, userID string) error
}

// ChatHubber is the interface for live chat fan-out operations.
type ChatHubber interface {
	Subscribe(streamID string) (chan *models.ChatMessage, error)
	Unsubscribe(streamID string, ch chan *models.ChatMessage)
	Send(ctx context.Context, streamID, userID, userName, userPic, body string) (*models.ChatMessage, error)
	GetHistory(ctx context.Context, streamID string, limit int) ([]*models.ChatMessage, error)
}

// OAuthServicer is the interface for Google OAuth operations.
type OAuthServicer interface {
	BuildAuthURL(state string) string
	ExchangeAndFetchUserInfo(ctx context.Context, code string) (*GoogleUser, error)
}

// UserSessionServicer is the interface for browser session cookie operations.
type UserSessionServicer interface {
	Issue(user *models.User) (string, error)
	Parse(tokenStr string) (*UserClaims, error)
	SetSessionCookie(w http.ResponseWriter, user *models.User) error
	ClearSessionCookie(w http.ResponseWriter)
}

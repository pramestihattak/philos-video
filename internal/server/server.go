package server

import (
	"philos-video/internal/health"
	"philos-video/internal/live"
	"philos-video/internal/middleware"
	"philos-video/internal/repository"
	"philos-video/internal/service"
)

// Server implements api.ServerInterface. It holds all application dependencies.
type Server struct {
	// services
	videoSvc     *service.VideoService
	uploadSvc    *service.UploadService
	sessionSvc   *service.SessionService
	commentSvc   *service.CommentService
	chatHub      *service.ChatHub
	oauthSvc     *service.OAuthService
	userSessionSvc *service.UserSessionService

	// repositories accessed directly
	streamKeyRepo *repository.StreamKeyRepo
	sessionRepo   *repository.SessionRepo
	eventRepo     *repository.EventRepo
	userRepo      *repository.UserRepo
	videoRepo     interface {
		UpdateThumbnailPath(id, path string) error
	}

	// live manager
	liveMgr *live.Manager

	// health
	healthChecker *health.HealthChecker

	// middleware (for user injection)
	userAuthMW *middleware.UserAuthMiddleware

	// config values
	dataDir      string
	defaultQuota int64
}

// Params holds constructor arguments for Server.
type Params struct {
	VideoSvc       *service.VideoService
	UploadSvc      *service.UploadService
	SessionSvc     *service.SessionService
	CommentSvc     *service.CommentService
	ChatHub        *service.ChatHub
	OAuthSvc       *service.OAuthService
	UserSessionSvc *service.UserSessionService

	StreamKeyRepo *repository.StreamKeyRepo
	SessionRepo   *repository.SessionRepo
	EventRepo     *repository.EventRepo
	UserRepo      *repository.UserRepo
	VideoRepo     interface {
		UpdateThumbnailPath(id, path string) error
	}

	LiveMgr       *live.Manager
	HealthChecker *health.HealthChecker
	UserAuthMW    *middleware.UserAuthMiddleware

	DataDir      string
	DefaultQuota int64
}

// New creates a Server.
func New(p Params) *Server {
	return &Server{
		videoSvc:       p.VideoSvc,
		uploadSvc:      p.UploadSvc,
		sessionSvc:     p.SessionSvc,
		commentSvc:     p.CommentSvc,
		chatHub:        p.ChatHub,
		oauthSvc:       p.OAuthSvc,
		userSessionSvc: p.UserSessionSvc,

		streamKeyRepo: p.StreamKeyRepo,
		sessionRepo:   p.SessionRepo,
		eventRepo:     p.EventRepo,
		userRepo:      p.UserRepo,
		videoRepo:     p.VideoRepo,

		liveMgr:       p.LiveMgr,
		healthChecker: p.HealthChecker,
		userAuthMW:    p.UserAuthMW,

		dataDir:      p.DataDir,
		defaultQuota: p.DefaultQuota,
	}
}

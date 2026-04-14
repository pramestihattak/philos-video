package server

import (
	"philos-video/internal/health"
	"philos-video/internal/live"
	"philos-video/internal/middleware"
	"philos-video/internal/service"
	"philos-video/internal/storage"
)

// Server implements api.ServerInterface. It holds all application dependencies.
type Server struct {
	// services
	videoSvc       service.VideoServicer
	uploadSvc      service.UploadServicer
	sessionSvc     service.SessionServicer
	commentSvc     service.CommentServicer
	chatHub        service.ChatHubber
	oauthSvc       service.OAuthServicer
	userSessionSvc service.UserSessionServicer

	// storage accessed directly by handlers
	streamKeyRepo storage.StreamKeyStorer
	sessionRepo   storage.SessionStorer
	eventRepo     storage.EventStorer
	userRepo      storage.UserStorer
	videoRepo     storage.VideoStorer

	// live manager
	liveMgr *live.Manager

	// health
	healthChecker *health.HealthChecker

	// middleware (for user injection)
	userAuthMW *middleware.UserAuthMiddleware

	// config values
	dataDir         string
	defaultQuota    int64
	goLiveWhitelist []string
}

// Params holds constructor arguments for Server.
type Params struct {
	VideoSvc       service.VideoServicer
	UploadSvc      service.UploadServicer
	SessionSvc     service.SessionServicer
	CommentSvc     service.CommentServicer
	ChatHub        service.ChatHubber
	OAuthSvc       service.OAuthServicer
	UserSessionSvc service.UserSessionServicer

	StreamKeyRepo storage.StreamKeyStorer
	SessionRepo   storage.SessionStorer
	EventRepo     storage.EventStorer
	UserRepo      storage.UserStorer
	VideoRepo     storage.VideoStorer

	LiveMgr       *live.Manager
	HealthChecker *health.HealthChecker
	UserAuthMW    *middleware.UserAuthMiddleware

	DataDir         string
	DefaultQuota    int64
	GoLiveWhitelist []string
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

		dataDir:         p.DataDir,
		defaultQuota:    p.DefaultQuota,
		goLiveWhitelist: p.GoLiveWhitelist,
	}
}

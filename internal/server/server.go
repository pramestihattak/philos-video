package server

import (
	"philos-video/internal/health"
	"philos-video/internal/live"
	"philos-video/internal/middleware"
	"philos-video/internal/service/chat"
	"philos-video/internal/service/comment"
	"philos-video/internal/service/oauth"
	"philos-video/internal/service/session"
	"philos-video/internal/service/upload"
	"philos-video/internal/service/usersession"
	"philos-video/internal/service/video"
	eventrepo "philos-video/internal/storage/event"
	sessionrepo "philos-video/internal/storage/session"
	streamkeyrepo "philos-video/internal/storage/streamkey"
	userrepo "philos-video/internal/storage/user"
	videorepo "philos-video/internal/storage/video"
)

// Server implements api.ServerInterface. It holds all application dependencies.
type Server struct {
	// services
	videoSvc       video.Servicer
	uploadSvc      upload.Servicer
	sessionSvc     session.Servicer
	commentSvc     comment.Servicer
	chatHub        chat.Hubber
	oauthSvc       oauth.Servicer
	userSessionSvc usersession.Servicer

	// storage accessed directly by handlers
	streamKeyRepo streamkeyrepo.Repository
	sessionRepo   sessionrepo.Repository
	eventRepo     eventrepo.Repository
	userRepo      userrepo.Repository
	videoRepo     videorepo.Repository

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
	VideoSvc       video.Servicer
	UploadSvc      upload.Servicer
	SessionSvc     session.Servicer
	CommentSvc     comment.Servicer
	ChatHub        chat.Hubber
	OAuthSvc       oauth.Servicer
	UserSessionSvc usersession.Servicer

	StreamKeyRepo streamkeyrepo.Repository
	SessionRepo   sessionrepo.Repository
	EventRepo     eventrepo.Repository
	UserRepo      userrepo.Repository
	VideoRepo     videorepo.Repository

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

package server

import (
	"philos-video/internal/middleware"
	"philos-video/internal/models"
)

func (s *Server) canGoLive(user *models.User) bool {
	return middleware.CanGoLive(s.goLiveWhitelist, user.Email)
}

package server

import "net/http"

// Logout handles POST /auth/logout.
func (s *Server) Logout(w http.ResponseWriter, r *http.Request) {
	s.userSessionSvc.ClearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

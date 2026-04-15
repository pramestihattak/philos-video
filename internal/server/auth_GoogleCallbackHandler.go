package server

import (
	"log/slog"
	"net/http"
)

// GoogleCallbackHandler handles GET /auth/google/callback.
func (s *Server) GoogleCallbackHandler(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
		writeError(w, "invalid oauth state", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "oauth_state", Value: "", Path: "/", MaxAge: -1})

	googleUser, err := s.oauthSvc.ExchangeAndFetchUserInfo(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		slog.Error("oauth exchange failed", "err", err)
		writeError(w, "authentication failed", http.StatusInternalServerError)
		return
	}

	user, created, err := s.userRepo.UpsertFromGoogle(r.Context(), googleUser.Sub, googleUser.Email, googleUser.Name, googleUser.Picture, s.defaultQuota)
	if err != nil {
		slog.Error("upserting user", "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if created {
		slog.Info("new user signed up", "user_id", user.ID, "email", user.Email)
	}

	if err := s.userSessionSvc.SetSessionCookie(w, user); err != nil {
		slog.Error("setting session cookie", "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	returnURL := "/"
	if rc, err := r.Cookie("oauth_return"); err == nil && safeReturnPath.MatchString(rc.Value) {
		returnURL = rc.Value
	}
	http.SetCookie(w, &http.Cookie{Name: "oauth_return", Value: "", Path: "/", MaxAge: -1})

	http.Redirect(w, r, returnURL, http.StatusFound)
}

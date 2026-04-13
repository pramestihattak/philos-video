package server

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"

	"philos-video/internal/middleware"
)

var safeReturnPath = regexp.MustCompile(`^/[^/]`)

// GetMe handles GET /api/v1/me.
func (s *Server) GetMe(w http.ResponseWriter, r *http.Request) {
	user := middleware.CurrentUser(r.Context())
	if user == nil {
		writeError(w, "not signed in", http.StatusUnauthorized)
		return
	}
	writeJSON(w, map[string]any{
		"id":                 user.ID,
		"email":              user.Email,
		"name":               user.Name,
		"picture":            user.Picture,
		"used_bytes":         user.UsedBytes,
		"upload_quota_bytes": user.UploadQuotaBytes,
	}, http.StatusOK)
}

// Logout handles POST /auth/logout.
func (s *Server) Logout(w http.ResponseWriter, r *http.Request) {
	s.userSessionSvc.ClearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// GoogleLoginHandler handles GET /auth/google/login.
// Note: These OAuth redirect endpoints are not part of the OpenAPI spec and are
// registered manually on the router in main.go.
func (s *Server) GoogleLoginHandler(w http.ResponseWriter, r *http.Request) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	state := hex.EncodeToString(b)

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	if ret := r.URL.Query().Get("return"); safeReturnPath.MatchString(ret) {
		http.SetCookie(w, &http.Cookie{
			Name:     "oauth_return",
			Value:    ret,
			Path:     "/",
			MaxAge:   600,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}

	http.Redirect(w, r, s.oauthSvc.BuildAuthURL(state), http.StatusFound)
}

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

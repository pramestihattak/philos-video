package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"

	"philos-video/internal/middleware"
	"philos-video/internal/repository"
	"philos-video/internal/service"
)

var safeReturnPath = regexp.MustCompile(`^/[^/]`)

type AuthHandler struct {
	oauthSvc   *service.OAuthService
	sessionSvc *service.UserSessionService
	userRepo   *repository.UserRepo
	defaultQuota int64
}

func NewAuthHandler(
	oauthSvc *service.OAuthService,
	sessionSvc *service.UserSessionService,
	userRepo *repository.UserRepo,
	defaultQuota int64,
) *AuthHandler {
	return &AuthHandler{
		oauthSvc:     oauthSvc,
		sessionSvc:   sessionSvc,
		userRepo:     userRepo,
		defaultQuota: defaultQuota,
	}
}

// GET /auth/google/login
func (h *AuthHandler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	state := hex.EncodeToString(b)

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   600, // 10 minutes
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	// Persist a safe return URL in a short-lived cookie.
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

	http.Redirect(w, r, h.oauthSvc.BuildAuthURL(state), http.StatusFound)
}

// GET /auth/google/callback
func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	// CSRF: verify state cookie matches query param.
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}
	// Clear state cookie.
	http.SetCookie(w, &http.Cookie{Name: "oauth_state", Value: "", Path: "/", MaxAge: -1})

	googleUser, err := h.oauthSvc.ExchangeAndFetchUserInfo(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		slog.Error("oauth exchange failed", "err", err)
		http.Error(w, "authentication failed", http.StatusInternalServerError)
		return
	}

	user, created, err := h.userRepo.UpsertFromGoogle(
		r.Context(),
		googleUser.Sub, googleUser.Email, googleUser.Name, googleUser.Picture,
		h.defaultQuota,
	)
	if err != nil {
		slog.Error("upserting user", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if created {
		slog.Info("new user signed up", "user_id", user.ID, "email", user.Email)
	}

	if err := h.sessionSvc.SetSessionCookie(w, user); err != nil {
		slog.Error("setting session cookie", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Redirect to saved return URL or home.
	returnURL := "/"
	if rc, err := r.Cookie("oauth_return"); err == nil && safeReturnPath.MatchString(rc.Value) {
		returnURL = rc.Value
	}
	http.SetCookie(w, &http.Cookie{Name: "oauth_return", Value: "", Path: "/", MaxAge: -1})

	http.Redirect(w, r, returnURL, http.StatusFound)
}

// POST /auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	h.sessionSvc.ClearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// GET /api/v1/me
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := middleware.CurrentUser(r.Context())
	if user == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "not signed in"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":                  user.ID,
		"email":               user.Email,
		"name":                user.Name,
		"picture":             user.Picture,
		"used_bytes":          user.UsedBytes,
		"upload_quota_bytes":  user.UploadQuotaBytes,
	})
}

// loginReturnURL extracts a safe return path from query params.
func loginReturnURL(r *http.Request) string {
	ret := r.URL.Query().Get("return")
	if safeReturnPath.MatchString(ret) {
		return url.QueryEscape(ret)
	}
	return ""
}

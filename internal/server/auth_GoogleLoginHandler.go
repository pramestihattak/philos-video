package server

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"
)

var safeReturnPath = regexp.MustCompile(`^/[^/]`)

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

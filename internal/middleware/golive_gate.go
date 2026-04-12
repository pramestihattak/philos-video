package middleware

import (
	"net/http"
	"strings"
)


// GoLiveGate returns a middleware that allows the request only if the signed-in
// user's email is in the whitelist. An empty whitelist blocks everyone.
// Non-API paths receive a 403 page; API paths receive a 403 JSON error.
func GoLiveGate(whitelist []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(whitelist))
	for _, email := range whitelist {
		allowed[strings.ToLower(strings.TrimSpace(email))] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := CurrentUser(r.Context())
			if user != nil {
				if _, ok := allowed[strings.ToLower(user.Email)]; ok {
					next.ServeHTTP(w, r)
					return
				}
			}
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			} else {
				http.Redirect(w, r, "/forbidden", http.StatusFound)
			}
		})
	}
}

// CanGoLive reports whether the given email is in the whitelist.
func CanGoLive(whitelist []string, email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	for _, e := range whitelist {
		if strings.ToLower(strings.TrimSpace(e)) == email {
			return true
		}
	}
	return false
}

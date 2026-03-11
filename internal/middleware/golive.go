package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
)

const goLiveCookieName = "golive_token"

// goliveToken returns the expected cookie value: HMAC-SHA256(pin, secret).
func goliveToken(pin, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(pin))
	return hex.EncodeToString(mac.Sum(nil))
}

func validGoLiveCookie(r *http.Request, pin, secret string) bool {
	c, err := r.Cookie(goLiveCookieName)
	if err != nil {
		return false
	}
	expected := goliveToken(pin, secret)
	return hmac.Equal([]byte(c.Value), []byte(expected))
}

// GoLivePinGate protects page routes: redirects to /go-live/login on failure.
// If pin is empty, the middleware is a no-op.
func GoLivePinGate(pin, secret string) func(http.Handler) http.Handler {
	if pin == "" {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !validGoLiveCookie(r, pin, secret) {
				http.Redirect(w, r, "/go-live/login", http.StatusFound)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// GoLivePinAPIGate protects API routes: returns 401 on failure instead of redirecting.
// If pin is empty, the middleware is a no-op.
func GoLivePinAPIGate(pin, secret string) func(http.Handler) http.Handler {
	if pin == "" {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !validGoLiveCookie(r, pin, secret) {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// SetGoLiveCookie writes the auth cookie after successful PIN entry.
func SetGoLiveCookie(w http.ResponseWriter, pin, secret string) {
	http.SetCookie(w, &http.Cookie{
		Name:     goLiveCookieName,
		Value:    goliveToken(pin, secret),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400 * 7, // 7 days
	})
}

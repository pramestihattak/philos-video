package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"philos-video/internal/models"
	"philos-video/internal/service"
)

type ctxKey int

const userCtxKey ctxKey = iota

// userLookup is the minimal interface required to resolve a user by ID.
type userLookup interface {
	GetByID(ctx context.Context, id string) (*models.User, error)
}

// userSessionParser is the minimal interface required to parse a user session cookie.
type userSessionParser interface {
	Parse(tokenStr string) (*service.UserClaims, error)
}

// CurrentUser returns the signed-in user from the request context, or nil.
func CurrentUser(ctx context.Context) *models.User {
	u, _ := ctx.Value(userCtxKey).(*models.User)
	return u
}

// UserAuthMiddleware holds the dependencies for the user-auth middleware functions.
type UserAuthMiddleware struct {
	sessionSvc userSessionParser
	userRepo   userLookup
}

func NewUserAuthMiddleware(svc userSessionParser, userRepo userLookup) *UserAuthMiddleware {
	return &UserAuthMiddleware{sessionSvc: svc, userRepo: userRepo}
}

// loadUser reads the session cookie and looks up the user. Returns nil if not
// signed in or if the token/user is invalid.
func (m *UserAuthMiddleware) loadUser(r *http.Request) *models.User {
	c, err := r.Cookie("philos_session")
	if err != nil {
		return nil
	}
	claims, err := m.sessionSvc.Parse(c.Value)
	if err != nil {
		return nil
	}
	user, err := m.userRepo.GetByID(r.Context(), claims.UserID)
	if err != nil || user == nil {
		return nil
	}
	return user
}

// OptionalUser loads the current user into the context if a valid session cookie
// is present, but never blocks the request.
func (m *UserAuthMiddleware) OptionalUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user := m.loadUser(r); user != nil {
			r = r.WithContext(context.WithValue(r.Context(), userCtxKey, user))
		}
		next.ServeHTTP(w, r)
	})
}

// RequireUser blocks unauthenticated requests. For browser routes it redirects
// to /login; for API routes it returns 401 JSON.
func (m *UserAuthMiddleware) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := m.loadUser(r)
		if user == nil {
			returnURL := r.URL.RequestURI()
			http.Redirect(w, r, "/login?return="+url.QueryEscape(returnURL), http.StatusFound)
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), userCtxKey, user))
		next.ServeHTTP(w, r)
	})
}

// RequireUserAPI blocks unauthenticated API requests with a 401 JSON response.
func (m *UserAuthMiddleware) RequireUserAPI(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := m.loadUser(r)
		if user == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), userCtxKey, user))
		next.ServeHTTP(w, r)
	})
}

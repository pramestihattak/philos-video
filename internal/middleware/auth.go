package middleware

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"philos-video/internal/repository"
	"philos-video/internal/service"
)

type AuthMiddleware struct {
	sessionSvc  *service.SessionService
	sessionRepo *repository.SessionRepo

	touchMu   sync.Mutex
	lastTouch map[string]time.Time // debounce: session_id → last touch time
}

func NewAuthMiddleware(sessionSvc *service.SessionService, sessionRepo *repository.SessionRepo) *AuthMiddleware {
	return &AuthMiddleware{
		sessionSvc:  sessionSvc,
		sessionRepo: sessionRepo,
		lastTouch:   make(map[string]time.Time),
	}
}

// RequirePlaybackToken validates the JWT token on every request under /videos/.
func (m *AuthMiddleware) RequirePlaybackToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}

		claims, err := m.sessionSvc.ParseToken(token)
		if err != nil {
			http.Error(w, "invalid token", http.StatusForbidden)
			return
		}

		requestedVideoID := extractVideoID(r.URL.Path)
		if claims.VideoID != requestedVideoID {
			http.Error(w, "token not valid for this video", http.StatusForbidden)
			return
		}

		go m.touchSession(claims.SessionID)

		ctx := context.WithValue(r.Context(), claimsKey{}, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireLiveToken validates the JWT token on every request under /live/.
func (m *AuthMiddleware) RequireLiveToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}

		claims, err := m.sessionSvc.ParseToken(token)
		if err != nil {
			http.Error(w, "invalid token", http.StatusForbidden)
			return
		}

		requestedStreamID := extractStreamID(r.URL.Path)
		if claims.StreamID != requestedStreamID {
			http.Error(w, "token not valid for this stream", http.StatusForbidden)
			return
		}

		go m.touchSession(claims.SessionID)

		ctx := context.WithValue(r.Context(), claimsKey{}, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ClaimsFromContext retrieves playback claims from the request context.
func ClaimsFromContext(ctx context.Context) *service.PlaybackClaims {
	v, _ := ctx.Value(claimsKey{}).(*service.PlaybackClaims)
	return v
}

type claimsKey struct{}

func (m *AuthMiddleware) touchSession(sessionID string) {
	const debounce = 30 * time.Second

	m.touchMu.Lock()
	if last, ok := m.lastTouch[sessionID]; ok && time.Since(last) < debounce {
		m.touchMu.Unlock()
		return
	}
	m.lastTouch[sessionID] = time.Now()
	m.touchMu.Unlock()

	_ = m.sessionRepo.TouchLastActive(context.Background(), sessionID)
}

// extractVideoID returns the first path segment after /videos/.
// e.g. /videos/abc123/master.m3u8 → "abc123"
func extractVideoID(path string) string {
	trimmed := strings.TrimPrefix(path, "/videos/")
	if idx := strings.IndexByte(trimmed, '/'); idx >= 0 {
		return trimmed[:idx]
	}
	return trimmed
}

// extractStreamID returns the first path segment after /live/.
// e.g. /live/ls_abc123/master.m3u8 → "ls_abc123"
func extractStreamID(path string) string {
	trimmed := strings.TrimPrefix(path, "/live/")
	if idx := strings.IndexByte(trimmed, '/'); idx >= 0 {
		return trimmed[:idx]
	}
	return trimmed
}

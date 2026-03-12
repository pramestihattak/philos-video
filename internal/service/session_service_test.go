package service_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"philos-video/internal/service"
)

var _ = time.Now // ensure time is used

// stubSessionRepo and stubVideoRepo satisfy the interfaces needed by NewSessionService
// without touching a real database.

type stubSessionRepo struct{}
type stubVideoRepo struct{}

// newTestSessionService creates a SessionService with a known secret and expiry.
func newTestSessionService(t *testing.T) *service.SessionService {
	t.Helper()
	svc, err := service.NewSessionService(nil, nil, "test-secret-32-chars-long!!!!!", "1h")
	if err != nil {
		t.Fatalf("NewSessionService: %v", err)
	}
	return svc
}

func TestNewSessionService_InvalidExpiry(t *testing.T) {
	_, err := service.NewSessionService(nil, nil, "test-secret-32-chars-long!!!!!", "not-a-duration")
	if err == nil {
		t.Fatal("expected error for invalid JWT_EXPIRY, got nil")
	}
}

func TestParseToken_ValidToken(t *testing.T) {
	secret := "test-secret-32-chars-long!!!!!"
	svc, err := service.NewSessionService(nil, nil, secret, "1h")
	if err != nil {
		t.Fatalf("NewSessionService: %v", err)
	}

	// Build a token manually matching the expected claims shape.
	claims := service.PlaybackClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        "sess_abc123",
		},
		SessionID: "sess_abc123",
		VideoID:   "vid_xyz",
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}

	parsed, err := svc.ParseToken(signed)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if parsed.SessionID != "sess_abc123" {
		t.Errorf("SessionID = %q, want %q", parsed.SessionID, "sess_abc123")
	}
	if parsed.VideoID != "vid_xyz" {
		t.Errorf("VideoID = %q, want %q", parsed.VideoID, "vid_xyz")
	}
}

func TestParseToken_ExpiredToken(t *testing.T) {
	secret := "test-secret-32-chars-long!!!!!"
	svc, err := service.NewSessionService(nil, nil, secret, "1h")
	if err != nil {
		t.Fatalf("NewSessionService: %v", err)
	}

	claims := service.PlaybackClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)), // already expired
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ID:        "sess_old",
		},
		SessionID: "sess_old",
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := tok.SignedString([]byte(secret))

	_, err = svc.ParseToken(signed)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestParseToken_WrongSecret(t *testing.T) {
	svc, err := service.NewSessionService(nil, nil, "correct-secret-32-chars-long!!!!", "1h")
	if err != nil {
		t.Fatalf("NewSessionService: %v", err)
	}

	claims := service.PlaybackClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        "sess_tampered",
		},
		SessionID: "sess_tampered",
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := tok.SignedString([]byte("wrong-secret-32-chars-long!!!!!!"))

	_, err = svc.ParseToken(signed)
	if err == nil {
		t.Fatal("expected error for token signed with wrong secret, got nil")
	}
}

func TestParseToken_MalformedToken(t *testing.T) {
	svc, err := service.NewSessionService(nil, nil, "test-secret-32-chars-long!!!!!", "1h")
	if err != nil {
		t.Fatalf("NewSessionService: %v", err)
	}

	for _, garbage := range []string{"", "not.a.jwt", "eyJhbGciOiJub25lIn0.e30."} {
		_, err = svc.ParseToken(garbage)
		if err == nil {
			t.Errorf("ParseToken(%q): expected error, got nil", garbage)
		}
	}
}

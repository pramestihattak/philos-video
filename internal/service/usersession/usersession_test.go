package usersession_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"philos-video/internal/models"
	"philos-video/internal/service/usersession"
)

func newTestUserSvc(t *testing.T, secret string) *usersession.Service {
	t.Helper()
	svc, err := usersession.New(secret, false)
	if err != nil {
		t.Fatalf("usersession.New: %v", err)
	}
	return svc
}

func TestUserSessionService_IssueAndParse(t *testing.T) {
	svc := newTestUserSvc(t, "supersecret-test-key-32-chars-ok")
	user := &models.User{ID: "usr_abc123", Email: "alice@example.com"}

	token, err := svc.Issue(user)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := svc.Parse(token)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if claims.UserID != user.ID {
		t.Errorf("UserID: got %q, want %q", claims.UserID, user.ID)
	}
	if claims.Email != user.Email {
		t.Errorf("Email: got %q, want %q", claims.Email, user.Email)
	}
}

func TestUserSessionService_RejectExpiredToken(t *testing.T) {
	svc := newTestUserSvc(t, "supersecret-test-key-32-chars-ok")
	user := &models.User{ID: "usr_exp", Email: "bob@example.com"}

	claims := usersession.UserClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		},
		UserID: user.ID,
		Email:  user.Email,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := tok.SignedString([]byte("supersecret-test-key-32-chars-ok"))

	_, err := svc.Parse(signed)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestUserSessionService_RejectWrongSecret(t *testing.T) {
	svcA := newTestUserSvc(t, "secret-A-32-chars-exactly-here-ok")
	svcB := newTestUserSvc(t, "secret-B-32-chars-exactly-here-ok")

	user := &models.User{ID: "usr_x", Email: "x@example.com"}
	token, _ := svcA.Issue(user)

	_, err := svcB.Parse(token)
	if err == nil {
		t.Fatal("expected error for wrong secret, got nil")
	}
}

func TestNewUserSessionService_ShortSecret(t *testing.T) {
	_, err := usersession.New("short", false)
	if err == nil {
		t.Fatal("expected error for short secret")
	}
}

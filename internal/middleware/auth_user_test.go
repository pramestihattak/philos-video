package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"philos-video/internal/middleware"
	"philos-video/internal/models"
	"philos-video/internal/service/usersession"
)

// stubUserRepo implements the userLookup interface (GetByID) for tests.
type stubUserRepo struct {
	user *models.User
}

func (r *stubUserRepo) GetByID(_ context.Context, id string) (*models.User, error) {
	if r.user != nil && r.user.ID == id {
		return r.user, nil
	}
	return nil, nil
}

func makeAuthMW(t *testing.T, user *models.User) (*middleware.UserAuthMiddleware, string) {
	t.Helper()
	svc, err := usersession.New("test-secret-key-must-be-32-chars-ok", false)
	if err != nil {
		t.Fatalf("NewUserSessionService: %v", err)
	}
	token, err := svc.Issue(user)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	repo := &stubUserRepo{user: user}
	mw := middleware.NewUserAuthMiddleware(svc, repo)
	return mw, token
}

func TestOptionalUser_WithValidCookie(t *testing.T) {
	user := &models.User{ID: "usr_test1", Email: "t@example.com"}
	mw, token := makeAuthMW(t, user)

	called := false
	handler := mw.OptionalUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		u := middleware.CurrentUser(r.Context())
		if u == nil {
			t.Error("expected user in context, got nil")
		} else if u.ID != user.ID {
			t.Errorf("user ID: got %q, want %q", u.ID, user.ID)
		}
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "philos_session", Value: token})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("inner handler was not called")
	}
}

func TestOptionalUser_WithNoCookie(t *testing.T) {
	user := &models.User{ID: "usr_test2", Email: "t2@example.com"}
	mw, _ := makeAuthMW(t, user)

	called := false
	handler := mw.OptionalUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if u := middleware.CurrentUser(r.Context()); u != nil {
			t.Errorf("expected nil user, got %+v", u)
		}
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("inner handler was not called")
	}
}

func TestRequireUser_RedirectsUnauthenticated(t *testing.T) {
	user := &models.User{ID: "usr_test3", Email: "t3@example.com"}
	mw, _ := makeAuthMW(t, user)

	handler := mw.RequireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler should not be called for unauthenticated request")
	}))

	req := httptest.NewRequest("GET", "/upload", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", rr.Code)
	}
	if rr.Header().Get("Location") == "" {
		t.Error("expected redirect Location header")
	}
}

func TestRequireUserAPI_Returns401(t *testing.T) {
	user := &models.User{ID: "usr_test4", Email: "t4@example.com"}
	mw, _ := makeAuthMW(t, user)

	handler := mw.RequireUserAPI(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/api/v1/videos", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestRequireUser_AllowsAuthenticated(t *testing.T) {
	user := &models.User{ID: "usr_test5", Email: "t5@example.com"}
	mw, token := makeAuthMW(t, user)

	called := false
	handler := mw.RequireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		u := middleware.CurrentUser(r.Context())
		if u == nil || u.ID != user.ID {
			t.Errorf("unexpected user: %+v", u)
		}
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "philos_session", Value: token})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("inner handler was not called for authenticated request")
	}
	if rr.Code == http.StatusFound {
		t.Error("authenticated request should not be redirected")
	}
}

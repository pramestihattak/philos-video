package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"philos-video/internal/middleware"
)

func TestIPRateLimiter_AllowsUnderLimit(t *testing.T) {
	limiter := middleware.NewIPRateLimiter(5, time.Minute)
	handler := limiter(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := range 5 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200", i+1, rr.Code)
		}
	}
}

func TestIPRateLimiter_BlocksOverLimit(t *testing.T) {
	limiter := middleware.NewIPRateLimiter(3, time.Minute)
	handler := limiter(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := range 3 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.2:5678"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200", i+1, rr.Code)
		}
	}

	// 4th request must be rejected.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.2:5678"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("got %d, want 429", rr.Code)
	}
}

func TestIPRateLimiter_DifferentIPsAreSeparate(t *testing.T) {
	limiter := middleware.NewIPRateLimiter(1, time.Minute)
	handler := limiter(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, ip := range []string{"1.2.3.4:100", "5.6.7.8:200"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("IP %s: got %d, want 200", ip, rr.Code)
		}
	}
}

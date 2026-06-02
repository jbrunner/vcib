package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jbrunner/vcib/internal/middleware"
)

var okHandler = http.HandlerFunc(func(rec http.ResponseWriter, _ *http.Request) {
	rec.WriteHeader(http.StatusOK)
})

func newRequest(t *testing.T, method, target string) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), method, target, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	return req
}

func TestAuth_EmptyToken_PassesThrough(t *testing.T) {
	h := middleware.Auth("")(okHandler)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newRequest(t, "PURGE", "/path"))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAuth_CorrectToken_PassesThrough(t *testing.T) {
	h := middleware.Auth("Bearer secret")(okHandler)
	req := newRequest(t, "PURGE", "/path")
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAuth_WrongToken_Returns401(t *testing.T) {
	h := middleware.Auth("Bearer secret")(okHandler)
	req := newRequest(t, "PURGE", "/path")
	req.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Errorf("WWW-Authenticate = %q, want %q", got, "Bearer")
	}
}

func TestAuth_MissingToken_Returns401(t *testing.T) {
	h := middleware.Auth("Bearer secret")(okHandler)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newRequest(t, "PURGE", "/path"))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuth_WrongToken_NextNotCalled(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { called = true })
	h := middleware.Auth("Bearer secret")(next)
	req := newRequest(t, "PURGE", "/path")
	req.Header.Set("Authorization", "Bearer wrong")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if called {
		t.Error("next handler must not be called on failed auth")
	}
}

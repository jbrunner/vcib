package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jbrunner/vcib/internal/middleware"
)

func TestLogging_PassesThrough(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(rec http.ResponseWriter, _ *http.Request) {
		called = true
		rec.WriteHeader(http.StatusAccepted)
	})
	rec := httptest.NewRecorder()
	middleware.Logging(next).ServeHTTP(rec, newRequest(t, "PURGE", "/path"))

	if !called {
		t.Error("next handler was not called")
	}

	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
}

func TestLogging_DefaultStatus200_WhenHandlerOmitsWriteHeader(t *testing.T) {
	next := http.HandlerFunc(func(rec http.ResponseWriter, _ *http.Request) {
		_, _ = rec.Write([]byte("ok"))
	})
	rec := httptest.NewRecorder()
	middleware.Logging(next).ServeHTTP(rec, newRequest(t, "GET", "/healthz"))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

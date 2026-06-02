// Package middleware provides reusable HTTP middleware for VCIB.
package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Logging wraps h and emits a structured slog entry for every request.
func Logging(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()

		h.ServeHTTP(wrapped, req)

		slog.Info("http request",
			"method", req.Method,
			"path", req.URL.RequestURI(),
			"status", wrapped.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", req.RemoteAddr,
		)
	})
}

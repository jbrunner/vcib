package middleware

import (
	"net/http"
)

// Auth returns a middleware that enforces Authorization header authentication.
// The clientAuth value is compared directly against the full Authorization header value
// (e.g. "Bearer foo", "Basic foo:bar"). If clientAuth is empty, all requests pass through.
func Auth(clientAuth string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if clientAuth == "" {
			return next
		}

		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if req.Header.Get("Authorization") != clientAuth {
				resp.Header().Set("WWW-Authenticate", "Bearer")
				http.Error(resp, "unauthorized", http.StatusUnauthorized)

				return
			}
			next.ServeHTTP(resp, req)
		})
	}
}

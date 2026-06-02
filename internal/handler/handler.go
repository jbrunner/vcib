// Package handler provides the HTTP handler for cache invalidation requests.
package handler

import (
	"net/http"

	"github.com/jbrunner/vcib/internal/metrics"
)

// dispatcher is the interface for forwarding invalidation requests to Varnish pods.
type dispatcher interface {
	Dispatch(method, requestURI string, headers http.Header)
}

// Handler handles PURGE and BAN cache invalidation requests.
type Handler struct {
	dispatcher dispatcher
	met        *metrics.Metrics
}

// New creates a Handler backed by the given dispatcher.
func New(d dispatcher, met *metrics.Metrics) *Handler {
	return &Handler{dispatcher: d, met: met}
}

// ServeHTTP responds immediately with 202 Accepted and dispatches the
// invalidation request to all Varnish pods in a background goroutine.
func (h *Handler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	if req.Method != "PURGE" && req.Method != "BAN" {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)

		return
	}

	// Respond before dispatching to minimize client-perceived latency.
	writer.WriteHeader(http.StatusAccepted)

	h.met.InvalidationRequestsTotal.WithLabelValues(req.Method).Inc()

	// Clone headers so the dispatcher goroutine has a stable copy after this
	// handler returns and the request is recycled by the server.
	h.dispatcher.Dispatch(req.Method, req.URL.RequestURI(), req.Header.Clone())
}

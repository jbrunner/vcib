// Package health provides liveness, readiness, and status HTTP handlers.
package health

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/jbrunner/vcib/internal/discovery"
)

const (
	readyThreshold  = 0.75
	tcpProbeTimeout = 2 * time.Second
	k8sAPITimeout   = 5 * time.Second
	readyTimeout    = 10 * time.Second
	statusTimeout   = 15 * time.Second
)

type dispatchTracker interface {
	ActiveDispatches() int64
}

type podDiscoverer interface {
	ListReadyPods(ctx context.Context) ([]discovery.Pod, error)
	ListPods(ctx context.Context) ([]discovery.Pod, error)
}

// Handler provides HTTP handlers for liveness, readiness, and status checks.
type Handler struct {
	discoverer podDiscoverer
	dispatcher dispatchTracker
	port       string
}

// New creates a Handler.
func New(d podDiscoverer, dt dispatchTracker, port string) *Handler {
	return &Handler{discoverer: d, dispatcher: dt, port: port}
}

// Liveness handles GET /healthz. Always returns 200 OK.
func (h *Handler) Liveness(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// Readiness handles GET /readyz.
// Returns 200 with body `true` if the K8s API is reachable (proves auth+RBAC)
// and at least 75% of ready Varnish pods are reachable via TCP.
// Returns 503 with body `false` otherwise. Errors are only logged.
func (h *Handler) Readiness(writer http.ResponseWriter, req *http.Request) {
	k8sCtx, k8sCancel := context.WithTimeout(context.Background(), k8sAPITimeout)
	defer k8sCancel()

	pods, err := h.discoverer.ListReadyPods(k8sCtx)
	if err != nil {
		slog.Error("readiness: failed to list ready pods", "error", err)
		writeJSON(writer, http.StatusServiceUnavailable, false)

		return
	}

	if len(pods) == 0 {
		slog.Warn("readiness: no ready varnish pods")
		writeJSON(writer, http.StatusServiceUnavailable, false)

		return
	}

	tcpCtx, tcpCancel := context.WithTimeout(req.Context(), readyTimeout)
	defer tcpCancel()

	reachable := h.tcpProbeAll(tcpCtx, pods)
	ratio := float64(reachable) / float64(len(pods))

	if ratio < readyThreshold {
		slog.Warn("readiness: not enough pods reachable via TCP",
			"reachable", reachable,
			"ready", len(pods),
			"threshold", readyThreshold,
		)
		writeJSON(writer, http.StatusServiceUnavailable, false)

		return
	}

	writeJSON(writer, http.StatusOK, true)
}

type statusResponse struct {
	VarnishBackends  backendStats `json:"varnishBackends"`
	ActiveDispatches int64        `json:"activeDispatches"`
}

type backendStats struct {
	Total        int `json:"total"`
	Ready        int `json:"ready"`
	TCPReachable int `json:"tcpReachable"`
}

// Status handles GET /status. Returns a JSON snapshot of pod stats for debugging.
func (h *Handler) Status(writer http.ResponseWriter, req *http.Request) {
	k8sCtx, k8sCancel := context.WithTimeout(context.Background(), k8sAPITimeout)
	defer k8sCancel()

	resp := statusResponse{ActiveDispatches: h.dispatcher.ActiveDispatches()}

	if pods, err := h.discoverer.ListPods(k8sCtx); err == nil {
		var ready []discovery.Pod
		for _, p := range pods {
			if p.IsReady {
				ready = append(ready, p)
			}
		}
		tcpCtx, tcpCancel := context.WithTimeout(req.Context(), statusTimeout)
		defer tcpCancel()
		resp.VarnishBackends = backendStats{
			Total:        len(pods),
			Ready:        len(ready),
			TCPReachable: h.tcpProbeAll(tcpCtx, ready),
		}
	}

	writeJSON(writer, http.StatusOK, resp)
}

func (h *Handler) tcpProbeAll(ctx context.Context, pods []discovery.Pod) int {
	if len(pods) == 0 {
		return 0
	}

	type result struct {
		pod discovery.Pod
		ok  bool
		err error
	}

	results := make(chan result, len(pods))
	for _, pod := range pods {
		go func() {
			addr := net.JoinHostPort(pod.IP, h.port)
			dialer := &net.Dialer{Timeout: tcpProbeTimeout}
			conn, err := dialer.DialContext(ctx, "tcp", addr)
			if err != nil {
				results <- result{pod: pod, ok: false, err: err}

				return
			}
			if err := conn.Close(); err != nil {
				slog.Debug("failed to close tcp probe connection", "error", err)
			}
			results <- result{pod: pod, ok: true}
		}()
	}

	reachable := 0
	for range pods {
		r := <-results
		if r.ok {
			reachable++
		} else {
			slog.Error("varnish backend not reachable", "pod", r.pod.Name, "ip", r.pod.IP, "port", h.port, "error", r.err)
		}
	}

	return reachable
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

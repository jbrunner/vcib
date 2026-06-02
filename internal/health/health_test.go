package health_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/jbrunner/vcib/internal/discovery"
	"github.com/jbrunner/vcib/internal/health"
)

const (
	localIP       = "127.0.0.1"
	unreachableIP = "127.0.0.2"
	podNameOne    = "pod-1"
	podNameTwo    = "pod-2"
)

var errK8sUnavailable = errors.New("k8s unavailable")

// mockDiscoverer implements the podDiscoverer interface accepted by health.New.
type mockDiscoverer struct {
	readyPods []discovery.Pod
	allPods   []discovery.Pod
	err       error
}

func (mock *mockDiscoverer) ListReadyPods(_ context.Context) ([]discovery.Pod, error) {
	return mock.readyPods, mock.err
}

func (mock *mockDiscoverer) ListPods(_ context.Context) ([]discovery.Pod, error) {
	return mock.allPods, mock.err
}

type stubTracker struct{ n int64 }

func (stub *stubTracker) ActiveDispatches() int64 { return stub.n }

// openPort starts a TCP listener on 127.0.0.1 and returns its port string.
// The listener stays open for the test — the kernel completes TCP handshakes
// for probe connections without Accept being called.
func openPort(t *testing.T) string {
	t.Helper()

	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", localIP+":0")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { listener.Close() })

	return strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
}

func newGetRequest(t *testing.T, target string) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	return req
}

// testStatusResponse mirrors the JSON shape of health.statusResponse.
type testStatusResponse struct {
	VarnishBackends  testBackendStats `json:"varnishBackends"`
	ActiveDispatches int64            `json:"activeDispatches"`
}

type testBackendStats struct {
	Total        int `json:"total"`
	Ready        int `json:"ready"`
	TCPReachable int `json:"tcpReachable"`
}

func TestLiveness_AlwaysOK(t *testing.T) {
	h := health.New(&mockDiscoverer{}, &stubTracker{}, "6081")
	rec := httptest.NewRecorder()
	h.Liveness(rec, newGetRequest(t, "/healthz"))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestReadiness_K8sError_Returns503(t *testing.T) {
	disc := &mockDiscoverer{err: errK8sUnavailable}
	h := health.New(disc, &stubTracker{}, "6081")
	rec := httptest.NewRecorder()
	h.Readiness(rec, newGetRequest(t, "/readyz"))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestReadiness_NoPods_Returns503(t *testing.T) {
	disc := &mockDiscoverer{readyPods: nil}
	h := health.New(disc, &stubTracker{}, "6081")
	rec := httptest.NewRecorder()
	h.Readiness(rec, newGetRequest(t, "/readyz"))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestReadiness_AllPodsReachable_Returns200(t *testing.T) {
	port := openPort(t)
	disc := &mockDiscoverer{
		readyPods: []discovery.Pod{
			{Name: podNameOne, IP: localIP},
			{Name: podNameTwo, IP: localIP},
		},
	}
	h := health.New(disc, &stubTracker{}, port)
	rec := httptest.NewRecorder()
	h.Readiness(rec, newGetRequest(t, "/readyz"))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// TestReadiness_AtThreshold verifies that exactly 75% reachable (>= threshold) returns 200.
// 3 pods on 127.0.0.1 (open listener) and 1 on 127.0.0.2 (no listener → connection refused).
func TestReadiness_AtThreshold_Returns200(t *testing.T) {
	port := openPort(t)
	disc := &mockDiscoverer{
		readyPods: []discovery.Pod{
			{Name: podNameOne, IP: localIP},
			{Name: podNameTwo, IP: localIP},
			{Name: "pod-3", IP: localIP},
			{Name: "pod-4", IP: unreachableIP},
		},
	}
	h := health.New(disc, &stubTracker{}, port)
	rec := httptest.NewRecorder()
	h.Readiness(rec, newGetRequest(t, "/readyz"))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// TestReadiness_BelowThreshold verifies that < 75% reachable returns 503.
// 1 pod on 127.0.0.1 (open listener) and 3 on 127.0.0.2 (no listener → connection refused).
func TestReadiness_BelowThreshold_Returns503(t *testing.T) {
	port := openPort(t)
	disc := &mockDiscoverer{
		readyPods: []discovery.Pod{
			{Name: podNameOne, IP: localIP},
			{Name: podNameTwo, IP: unreachableIP},
			{Name: "pod-3", IP: unreachableIP},
			{Name: "pod-4", IP: unreachableIP},
		},
	}
	h := health.New(disc, &stubTracker{}, port)
	rec := httptest.NewRecorder()
	h.Readiness(rec, newGetRequest(t, "/readyz"))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestStatus_ReturnsCorrectJSON(t *testing.T) {
	port := openPort(t)
	disc := &mockDiscoverer{
		allPods: []discovery.Pod{
			{Name: podNameOne, IP: localIP, IsReady: true},
			{Name: podNameTwo, IP: localIP, IsReady: false},
		},
	}
	h := health.New(disc, &stubTracker{n: 7}, port)
	rec := httptest.NewRecorder()
	h.Status(rec, newGetRequest(t, "/status"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp testStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.ActiveDispatches != 7 {
		t.Errorf("ActiveDispatches = %d, want 7", resp.ActiveDispatches)
	}

	if resp.VarnishBackends.Total != 2 {
		t.Errorf("Total = %d, want 2", resp.VarnishBackends.Total)
	}

	if resp.VarnishBackends.Ready != 1 {
		t.Errorf("Ready = %d, want 1", resp.VarnishBackends.Ready)
	}

	if resp.VarnishBackends.TCPReachable != 1 {
		t.Errorf("TCPReachable = %d, want 1", resp.VarnishBackends.TCPReachable)
	}
}

func TestStatus_K8sError_ReturnsEmptyBackends(t *testing.T) {
	disc := &mockDiscoverer{err: errK8sUnavailable}
	h := health.New(disc, &stubTracker{n: 3}, "6081")
	rec := httptest.NewRecorder()
	h.Status(rec, newGetRequest(t, "/status"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp testStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.ActiveDispatches != 3 {
		t.Errorf("ActiveDispatches = %d, want 3", resp.ActiveDispatches)
	}

	if resp.VarnishBackends != (testBackendStats{}) {
		t.Errorf("VarnishBackends = %+v, want zero", resp.VarnishBackends)
	}
}

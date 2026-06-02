package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jbrunner/vcib/internal/handler"
	"github.com/jbrunner/vcib/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type mockDispatcher struct {
	calls []struct {
		method, uri string
		headers     http.Header
	}
}

func (mock *mockDispatcher) Dispatch(method, requestURI string, headers http.Header) {
	mock.calls = append(mock.calls, struct {
		method, uri string
		headers     http.Header
	}{method, requestURI, headers})
}

func newTestMetrics() *metrics.Metrics {
	return &metrics.Metrics{
		InvalidationRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "vcib_invalidation_requests_total", Help: "test"},
			[]string{"method"},
		),
	}
}

func newRequest(t *testing.T, method, target string) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), method, target, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	return req
}

func TestServeHTTP_MethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodPut} {
		t.Run(method, func(t *testing.T) {
			disp := &mockDispatcher{}
			h := handler.New(disp, newTestMetrics())

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, newRequest(t, method, "/path"))

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
			}

			if len(disp.calls) != 0 {
				t.Errorf("dispatch called %d times, want 0", len(disp.calls))
			}
		})
	}
}

func TestServeHTTP_Accepted(t *testing.T) {
	tests := []struct {
		method string
		path   string
	}{
		{"PURGE", "/some/path"},
		{"BAN", "/other/path?q=1"},
	}

	for _, testCase := range tests {
		t.Run(testCase.method, func(t *testing.T) {
			disp := &mockDispatcher{}
			met := newTestMetrics()
			h := handler.New(disp, met)

			req := newRequest(t, testCase.method, testCase.path)
			req.Host = "example.com" // simulate real HTTP server: Host is in req.Host, not req.Header
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusAccepted {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusAccepted)
			}

			if len(disp.calls) != 1 {
				t.Fatalf("dispatch calls = %d, want 1", len(disp.calls))
			}

			if disp.calls[0].method != testCase.method {
				t.Errorf("dispatch method = %q, want %q", disp.calls[0].method, testCase.method)
			}

			if disp.calls[0].uri != testCase.path {
				t.Errorf("dispatch uri = %q, want %q", disp.calls[0].uri, testCase.path)
			}

			if got := disp.calls[0].headers.Get("Host"); got != "example.com" {
				t.Errorf("Host header not forwarded: got %q, want %q", got, "example.com")
			}

			count := testutil.ToFloat64(met.InvalidationRequestsTotal.WithLabelValues(testCase.method))
			if count != 1 {
				t.Errorf("counter{method=%q} = %v, want 1", testCase.method, count)
			}
		})
	}
}

func TestServeHTTP_HeadersCloned(t *testing.T) {
	disp := &mockDispatcher{}
	h := handler.New(disp, newTestMetrics())

	req := newRequest(t, "PURGE", "/path")
	req.Header.Set("X-Custom", "value")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if len(disp.calls) != 1 {
		t.Fatalf("dispatch calls = %d, want 1", len(disp.calls))
	}

	req.Header.Set("X-Custom", "changed")

	if disp.calls[0].headers.Get("X-Custom") != "value" {
		t.Errorf("headers not cloned: got %q, want %q", disp.calls[0].headers.Get("X-Custom"), "value")
	}
}

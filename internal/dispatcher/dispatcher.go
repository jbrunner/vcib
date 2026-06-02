// Package dispatcher forwards invalidation requests to Varnish pods with retry logic.
package dispatcher

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jbrunner/vcib/internal/discovery"
	"github.com/jbrunner/vcib/internal/metrics"
)

var errUnexpectedStatus = errors.New("unexpected status from varnish backend")

// Dispatcher forwards invalidation requests to all ready Varnish pods.
type Dispatcher struct {
	discoverer       *discovery.Discoverer
	httpClient       *http.Client
	met              *metrics.Metrics
	port             string
	retryInterval    time.Duration
	retryCount       int
	requestTimeout   time.Duration
	headerPatterns   []string
	varnishAuth      string
	semaphore        chan struct{}
	activeDispatches atomic.Int64
}

// ActiveDispatches returns the number of pod-dispatch goroutines currently running.
func (d *Dispatcher) ActiveDispatches() int64 {
	return d.activeDispatches.Load()
}

// New creates a Dispatcher.
func New(
	discoverer *discovery.Discoverer,
	port string,
	retryInterval time.Duration,
	retryCount int,
	requestTimeout time.Duration,
	headerPatterns []string,
	maxConcurrent int,
	varnishAuth string,
	met *metrics.Metrics,
) *Dispatcher {
	return &Dispatcher{
		discoverer:     discoverer,
		httpClient:     &http.Client{},
		met:            met,
		port:           port,
		retryInterval:  retryInterval,
		retryCount:     retryCount,
		requestTimeout: requestTimeout,
		headerPatterns: headerPatterns,
		varnishAuth:    varnishAuth,
		semaphore:      make(chan struct{}, maxConcurrent),
	}
}

// Dispatch forwards the invalidation request to all ready Varnish pods asynchronously.
// It returns immediately; the actual dispatch happens in a background goroutine.
func (d *Dispatcher) Dispatch(method, requestURI string, headers http.Header) {
	go func() {
		ctx := context.Background()

		pods, err := d.discoverer.ListReadyPods(ctx)
		if err != nil {
			slog.Error("failed to list pods", "error", err)

			return
		}

		if len(pods) == 0 {
			slog.Warn("no ready Varnish pods found, skipping dispatch")

			return
		}

		start := time.Now()

		var waitGroup sync.WaitGroup
		for _, pod := range pods {
			d.semaphore <- struct{}{}
			waitGroup.Go(func() {
				defer func() { <-d.semaphore }()
				d.activeDispatches.Add(1)
				defer d.activeDispatches.Add(-1)
				d.dispatchToPod(pod, method, requestURI, headers)
			})
		}
		waitGroup.Wait()

		elapsed := time.Since(start)
		d.met.InvalidationDurationSeconds.WithLabelValues(method).Observe(elapsed.Seconds())

		slog.Debug("dispatch complete",
			"method", method,
			"path", requestURI,
			"pods", len(pods),
			"duration_ms", elapsed.Milliseconds(),
		)
	}()
}

func (d *Dispatcher) dispatchToPod(pod discovery.Pod, method, requestURI string, headers http.Header) {
	url := fmt.Sprintf("http://%s:%s%s", pod.IP, d.port, requestURI)

	var lastErr error
	for attempt := 1; ; attempt++ {
		lastErr = d.sendRequest(context.Background(), pod, url, method, headers, attempt)
		if lastErr == nil {
			d.met.PodConfirmationsTotal.WithLabelValues(pod.IP, "success").Inc()

			return
		}

		if attempt > d.retryCount {
			break
		}

		d.met.RetriesTotal.WithLabelValues(pod.IP).Inc()
		time.Sleep(d.retryInterval)
	}

	slog.Error("all attempts to varnish backend failed",
		"pod", pod.Name,
		"ip", pod.IP,
		"port", d.port,
		"method", method,
		"attempts", d.retryCount+1,
		"error", lastErr,
	)
	d.met.PodConfirmationsTotal.WithLabelValues(pod.IP, "failed").Inc()
}

func (d *Dispatcher) sendRequest(
	ctx context.Context,
	pod discovery.Pod,
	url, method string,
	headers http.Header,
	attempt int,
) error {
	reqCtx, cancel := context.WithTimeout(ctx, d.requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, method, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	d.forwardHeaders(req, headers)
	if d.varnishAuth != "" {
		req.Header.Set("Authorization", d.varnishAuth)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		slog.Debug("request to varnish backend failed",
			"pod", pod.Name, "ip", pod.IP, "port", d.port,
			"method", method, "attempt", attempt, "error", err)

		return fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Debug("failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		slog.Debug("varnish backend returned unexpected status",
			"pod", pod.Name, "ip", pod.IP,
			"method", method, "attempt", attempt, "status", resp.StatusCode)

		return fmt.Errorf("pod %s status %d: %w", pod.Name, resp.StatusCode, errUnexpectedStatus)
	}

	return nil
}

// forwardHeaders copies matched headers from src to req.
// The Host header is set via req.Host to ensure it is forwarded unchanged by the HTTP client.
func (d *Dispatcher) forwardHeaders(req *http.Request, src http.Header) {
	for name, values := range src {
		if !matchesPatterns(name, d.headerPatterns) {
			continue
		}
		if strings.EqualFold(name, "Host") {
			if len(values) > 0 {
				req.Host = values[0]
			}

			continue
		}
		for _, v := range values {
			req.Header.Add(name, v)
		}
	}
}

// matchesPatterns reports whether the header name matches any of the given patterns.
// Patterns support simple glob wildcards (e.g. "X-*").
func matchesPatterns(header string, patterns []string) bool {
	canonical := http.CanonicalHeaderKey(header)
	for _, p := range patterns {
		matched, err := filepath.Match(http.CanonicalHeaderKey(p), canonical)
		if err == nil && matched {
			return true
		}
	}

	return false
}

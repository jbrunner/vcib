// Package main is the entry point for the Varnish Cache Invalidation Broker.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jbrunner/vcib/internal/config"
	"github.com/jbrunner/vcib/internal/discovery"
	"github.com/jbrunner/vcib/internal/dispatcher"
	"github.com/jbrunner/vcib/internal/handler"
	"github.com/jbrunner/vcib/internal/health"
	"github.com/jbrunner/vcib/internal/metrics"
	"github.com/jbrunner/vcib/internal/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

const (
	shutdownTimeout   = 30 * time.Second
	readHeaderTimeout = 10 * time.Second
	numServers        = 2
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)

		return fmt.Errorf("loading config: %w", err)
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	})))

	slog.Info("starting varnish cache invalidation broker", "version", version, "log_level", cfg.LogLevel.String())

	met := metrics.New()
	met.DispatchConcurrencyLimit.Set(float64(cfg.MaxConcurrentDispatches))

	disc, err := discovery.New(cfg.VarnishNamespace, cfg.VarnishLabelSelector, cfg.PodCacheTTL, met.PodsDiscovered)
	if err != nil {
		slog.Error("failed to initialize pod discoverer", "error", err)

		return fmt.Errorf("creating pod discoverer: %w", err)
	}

	disp := newDispatcher(cfg, disc, met)
	mainServer := newInvalidationServer(cfg, disp, met)
	metricsServer := newMetricsServer(cfg, disc, disp)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, numServers)
	startServer(mainServer, serverErr)
	startServer(metricsServer, serverErr)

	var runErr error
	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case runErr = <-serverErr:
		slog.Error("server error", "error", runErr)
		runErr = fmt.Errorf("server startup failed: %w", runErr)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if shutdownErr := mainServer.Shutdown(shutdownCtx); shutdownErr != nil {
		slog.Error("failed to shut down invalidation server", "error", shutdownErr)
	}
	if shutdownErr := metricsServer.Shutdown(shutdownCtx); shutdownErr != nil {
		slog.Error("failed to shut down metrics server", "error", shutdownErr)
	}

	slog.Info("shutdown complete")

	return runErr
}

func newDispatcher(cfg *config.Config, disc *discovery.Discoverer, met *metrics.Metrics) *dispatcher.Dispatcher {
	return dispatcher.New(
		disc,
		cfg.VarnishPort,
		cfg.RetryInterval,
		cfg.RetryCount,
		cfg.RequestTimeout,
		cfg.ForwardHeaders,
		cfg.MaxConcurrentDispatches,
		cfg.VarnishAuth,
		met,
	)
}

func newInvalidationServer(cfg *config.Config, disp *dispatcher.Dispatcher, met *metrics.Metrics) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/", handler.New(disp, met))

	return &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           middleware.Logging(middleware.Auth(cfg.ClientAuth)(mux)),
		ReadHeaderTimeout: readHeaderTimeout,
	}
}

func newMetricsServer(cfg *config.Config, disc *discovery.Discoverer, disp *dispatcher.Dispatcher) *http.Server {
	healthHandler := health.New(disc, disp, cfg.VarnishPort)
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /healthz", healthHandler.Liveness)
	mux.HandleFunc("GET /readyz", healthHandler.Readiness)
	mux.HandleFunc("GET /status", healthHandler.Status)

	return &http.Server{
		Addr:              cfg.MetricsAddr,
		Handler:           middleware.Logging(mux),
		ReadHeaderTimeout: readHeaderTimeout,
	}
}

func startServer(s *http.Server, errc chan<- error) {
	go func() {
		slog.Info("starting server", "addr", s.Addr)
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()
}

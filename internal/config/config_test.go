package config_test

import (
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/jbrunner/vcib/internal/config"
)

const headerHost = "Host"

// assertEqual asserts that got equals want and reports the field name on failure.
func assertEqual[T comparable](t *testing.T, field string, got, want T) {
	t.Helper()

	if got != want {
		t.Errorf("%s = %v, want %v", field, got, want)
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("VCIB_VARNISH_NAMESPACE", "")
	t.Setenv("VCIB_VARNISH_LABEL_SELECTOR", "")
	t.Setenv("VCIB_VARNISH_PORT", "")
	t.Setenv("VCIB_LISTEN_ADDR", "")
	t.Setenv("VCIB_METRICS_ADDR", "")
	t.Setenv("VCIB_RETRY_INTERVAL", "")
	t.Setenv("VCIB_RETRY_COUNT", "")
	t.Setenv("VCIB_REQUEST_TIMEOUT", "")
	t.Setenv("VCIB_MAX_CONCURRENT_DISPATCHES", "")
	t.Setenv("VCIB_FORWARD_HEADERS", "")
	t.Setenv("VCIB_LOG_LEVEL", "")
	t.Setenv("VCIB_CLIENT_AUTH", "")
	t.Setenv("VCIB_VARNISH_AUTH", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEqual(t, "VarnishNamespace", cfg.VarnishNamespace, "default")
	assertEqual(t, "VarnishLabelSelector", cfg.VarnishLabelSelector, "app=varnish")
	assertEqual(t, "VarnishPort", cfg.VarnishPort, "6081")
	assertEqual(t, "ListenAddr", cfg.ListenAddr, ":8080")
	assertEqual(t, "MetricsAddr", cfg.MetricsAddr, ":9090")
	assertEqual(t, "RetryInterval", cfg.RetryInterval, 10*time.Second)
	assertEqual(t, "RetryCount", cfg.RetryCount, 3)
	assertEqual(t, "RequestTimeout", cfg.RequestTimeout, 5*time.Second)
	assertEqual(t, "MaxConcurrentDispatches", cfg.MaxConcurrentDispatches, 500)
	assertEqual(t, "LogLevel", cfg.LogLevel, slog.Level(0))
	assertEqual(t, "ClientAuth", cfg.ClientAuth, "")
	assertEqual(t, "VarnishAuth", cfg.VarnishAuth, "")

	if !slices.Equal(cfg.ForwardHeaders, []string{headerHost}) {
		t.Errorf("ForwardHeaders = %v, want [%s]", cfg.ForwardHeaders, headerHost)
	}
}

func TestLoad_CustomValues(t *testing.T) {
	t.Setenv("VCIB_VARNISH_NAMESPACE", "production")
	t.Setenv("VCIB_VARNISH_LABEL_SELECTOR", "app=cache")
	t.Setenv("VCIB_VARNISH_PORT", "8081")
	t.Setenv("VCIB_LISTEN_ADDR", ":9000")
	t.Setenv("VCIB_METRICS_ADDR", ":9100")
	t.Setenv("VCIB_RETRY_INTERVAL", "30s")
	t.Setenv("VCIB_RETRY_COUNT", "5")
	t.Setenv("VCIB_REQUEST_TIMEOUT", "15s")
	t.Setenv("VCIB_MAX_CONCURRENT_DISPATCHES", "100")
	t.Setenv("VCIB_FORWARD_HEADERS", "Host,X-Real-IP,X-Forwarded-For")
	t.Setenv("VCIB_LOG_LEVEL", "DEBUG")
	t.Setenv("VCIB_CLIENT_AUTH", "secret-token")
	t.Setenv("VCIB_VARNISH_AUTH", "varnish-secret")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEqual(t, "VarnishNamespace", cfg.VarnishNamespace, "production")
	assertEqual(t, "VarnishLabelSelector", cfg.VarnishLabelSelector, "app=cache")
	assertEqual(t, "VarnishPort", cfg.VarnishPort, "8081")
	assertEqual(t, "ListenAddr", cfg.ListenAddr, ":9000")
	assertEqual(t, "MetricsAddr", cfg.MetricsAddr, ":9100")
	assertEqual(t, "RetryInterval", cfg.RetryInterval, 30*time.Second)
	assertEqual(t, "RetryCount", cfg.RetryCount, 5)
	assertEqual(t, "RequestTimeout", cfg.RequestTimeout, 15*time.Second)
	assertEqual(t, "MaxConcurrentDispatches", cfg.MaxConcurrentDispatches, 100)
	assertEqual(t, "LogLevel", cfg.LogLevel, slog.LevelDebug)
	assertEqual(t, "ClientAuth", cfg.ClientAuth, "secret-token")
	assertEqual(t, "VarnishAuth", cfg.VarnishAuth, "varnish-secret")

	expected := []string{headerHost, "X-Real-IP", "X-Forwarded-For"}
	if !slices.Equal(cfg.ForwardHeaders, expected) {
		t.Errorf("ForwardHeaders = %v, want %v", cfg.ForwardHeaders, expected)
	}
}

func TestLoad_InvalidRetryInterval(t *testing.T) {
	t.Setenv("VCIB_RETRY_INTERVAL", "not-a-duration")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoad_InvalidRequestTimeout(t *testing.T) {
	t.Setenv("VCIB_RETRY_INTERVAL", "")
	t.Setenv("VCIB_REQUEST_TIMEOUT", "invalid")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoad_InvalidRetryCount(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"not a number", "abc"},
		{"zero", "0"},
		{"negative", "-1"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("VCIB_RETRY_INTERVAL", "")
			t.Setenv("VCIB_REQUEST_TIMEOUT", "")
			t.Setenv("VCIB_RETRY_COUNT", testCase.value)

			_, err := config.Load()
			if err == nil {
				t.Fatalf("expected error for VCIB_RETRY_COUNT=%q, got nil", testCase.value)
			}
		})
	}
}

func TestLoad_InvalidMaxDispatches(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"not a number", "abc"},
		{"zero", "0"},
		{"negative", "-5"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("VCIB_RETRY_INTERVAL", "")
			t.Setenv("VCIB_REQUEST_TIMEOUT", "")
			t.Setenv("VCIB_RETRY_COUNT", "")
			t.Setenv("VCIB_MAX_CONCURRENT_DISPATCHES", testCase.value)

			_, err := config.Load()
			if err == nil {
				t.Fatalf("expected error for VCIB_MAX_CONCURRENT_DISPATCHES=%q, got nil", testCase.value)
			}
		})
	}
}

func TestLoad_InvalidLogLevel(t *testing.T) {
	t.Setenv("VCIB_RETRY_INTERVAL", "")
	t.Setenv("VCIB_REQUEST_TIMEOUT", "")
	t.Setenv("VCIB_RETRY_COUNT", "")
	t.Setenv("VCIB_MAX_CONCURRENT_DISPATCHES", "")
	t.Setenv("VCIB_LOG_LEVEL", "NOTVALID")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid VCIB_LOG_LEVEL, got nil")
	}
}

func TestLoad_LogLevels(t *testing.T) {
	levels := []struct {
		input    string
		expected slog.Level
	}{
		{"DEBUG", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"WARN", slog.LevelWarn},
		{"ERROR", slog.LevelError},
	}

	for _, levelCase := range levels {
		t.Run(levelCase.input, func(t *testing.T) {
			t.Setenv("VCIB_RETRY_INTERVAL", "")
			t.Setenv("VCIB_REQUEST_TIMEOUT", "")
			t.Setenv("VCIB_RETRY_COUNT", "")
			t.Setenv("VCIB_MAX_CONCURRENT_DISPATCHES", "")
			t.Setenv("VCIB_LOG_LEVEL", levelCase.input)

			cfg, err := config.Load()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			assertEqual(t, "LogLevel", cfg.LogLevel, levelCase.expected)
		})
	}
}

func TestLoad_ForwardHeadersTrimmed(t *testing.T) {
	t.Setenv("VCIB_RETRY_INTERVAL", "")
	t.Setenv("VCIB_REQUEST_TIMEOUT", "")
	t.Setenv("VCIB_RETRY_COUNT", "")
	t.Setenv("VCIB_MAX_CONCURRENT_DISPATCHES", "")
	t.Setenv("VCIB_LOG_LEVEL", "")
	t.Setenv("VCIB_FORWARD_HEADERS", " Host , X-Real-IP , , X-Forwarded-For ")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{headerHost, "X-Real-IP", "X-Forwarded-For"}
	if !slices.Equal(cfg.ForwardHeaders, expected) {
		t.Errorf("ForwardHeaders = %v, want %v", cfg.ForwardHeaders, expected)
	}
}

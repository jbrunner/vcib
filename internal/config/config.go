// Package config loads and validates VCIB runtime configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultMaxDispatches = 500
	defaultRetryCount    = 3
	defaultForwardHeader = "Host"
)

var errNotPositive = errors.New("must be positive")

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	VarnishNamespace        string
	VarnishLabelSelector    string
	VarnishPort             string
	ListenAddr              string
	MetricsAddr             string
	RetryInterval           time.Duration
	RetryCount              int
	RequestTimeout          time.Duration
	PodCacheTTL             time.Duration
	ForwardHeaders          []string
	MaxConcurrentDispatches int
	LogLevel                slog.Level
	ClientAuth              string
	VarnishAuth             string
}

// Load reads configuration from environment variables and returns a Config.
func Load() (*Config, error) {
	retryInterval, err := parseDuration("VCIB_RETRY_INTERVAL", "10s")
	if err != nil {
		return nil, err
	}
	retryCount, err := parseInt("VCIB_RETRY_COUNT", defaultRetryCount)
	if err != nil {
		return nil, err
	}
	requestTimeout, err := parseDuration("VCIB_REQUEST_TIMEOUT", "5s")
	if err != nil {
		return nil, err
	}
	podCacheTTL, err := parseDuration("VCIB_POD_CACHE_TTL", "1s")
	if err != nil {
		return nil, err
	}
	maxDispatches, err := parseInt("VCIB_MAX_CONCURRENT_DISPATCHES", defaultMaxDispatches)
	if err != nil {
		return nil, err
	}

	forwardHeaders := []string{defaultForwardHeader}
	if v := os.Getenv("VCIB_FORWARD_HEADERS"); v != "" {
		forwardHeaders = splitTrimmed(v)
	}

	var logLevel slog.Level
	if v := os.Getenv("VCIB_LOG_LEVEL"); v != "" {
		if err := logLevel.UnmarshalText([]byte(v)); err != nil {
			return nil, fmt.Errorf("invalid VCIB_LOG_LEVEL=%q: %w", v, err)
		}
	}

	return &Config{
		VarnishNamespace:        getEnv("VCIB_VARNISH_NAMESPACE", "default"),
		VarnishLabelSelector:    getEnv("VCIB_VARNISH_LABEL_SELECTOR", "app=varnish"),
		VarnishPort:             getEnv("VCIB_VARNISH_PORT", "6081"),
		ListenAddr:              getEnv("VCIB_LISTEN_ADDR", ":8080"),
		MetricsAddr:             getEnv("VCIB_METRICS_ADDR", ":9090"),
		RetryInterval:           retryInterval,
		RetryCount:              retryCount,
		RequestTimeout:          requestTimeout,
		PodCacheTTL:             podCacheTTL,
		ForwardHeaders:          forwardHeaders,
		MaxConcurrentDispatches: maxDispatches,
		LogLevel:                logLevel,
		ClientAuth:              os.Getenv("VCIB_CLIENT_AUTH"),
		VarnishAuth:             os.Getenv("VCIB_VARNISH_AUTH"),
	}, nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return defaultVal
}

func parseDuration(key, defaultVal string) (time.Duration, error) {
	v := getEnv(key, defaultVal)
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s=%q: %w", key, v, err)
	}

	return d, nil
}

func parseInt(key string, defaultVal int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return defaultVal, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s=%q: %w", key, value, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("invalid %s=%q: %w", key, value, errNotPositive)
	}

	return n, nil
}

func splitTrimmed(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}

	return result
}

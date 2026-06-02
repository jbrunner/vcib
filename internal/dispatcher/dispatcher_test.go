package dispatcher_test

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/jbrunner/vcib/internal/dispatcher"
)

const (
	headerHost    = "Host"
	headerXCustom = "X-Custom"
	headerWild    = "X-*"
	hostExample   = "example.com"
	headerVal     = "val"
)

func TestMatchesPatterns(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		patterns []string
		want     bool
	}{
		{name: "exact match", header: headerHost, patterns: []string{headerHost}, want: true},
		{name: "exact match case-insensitive", header: "host", patterns: []string{headerHost}, want: true},
		{name: "wildcard X-*", header: "X-Forwarded-For", patterns: []string{headerWild}, want: true},
		{name: "wildcard X-* no match", header: "Content-Type", patterns: []string{headerWild}, want: false},
		{name: "no patterns", header: headerHost, patterns: []string{}, want: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := dispatcher.MatchesPatterns(testCase.header, testCase.patterns)
			if got != testCase.want {
				t.Errorf("MatchesPatterns(%q, %v) = %v, want %v", testCase.header, testCase.patterns, got, testCase.want)
			}
		})
	}
}

func TestForwardHeaders(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		incoming http.Header
		expected http.Header
	}{
		{
			name:     "exact Host: forwarded via req.Host, other headers blocked",
			patterns: []string{headerHost},
			incoming: http.Header{
				headerHost:    {hostExample},
				headerXCustom: {headerVal},
			},
			expected: http.Header{
				headerHost: {hostExample},
			},
		},
		{
			name:     "wildcard X-*: matching headers forwarded, others blocked",
			patterns: []string{headerHost, headerWild},
			incoming: http.Header{
				headerHost:     {hostExample},
				headerXCustom:  {headerVal},
				"Content-Type": {"text/html"},
			},
			expected: http.Header{
				headerHost:    {hostExample},
				headerXCustom: {headerVal},
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			disp := dispatcher.NewTestDispatcher(testCase.patterns)

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://x/", http.NoBody)
			if err != nil {
				t.Fatal(err)
			}

			disp.ForwardHeadersTest(req, testCase.incoming)

			got := req.Header.Clone()
			if req.Host != "" {
				got.Set(headerHost, req.Host)
			}

			if !reflect.DeepEqual(got, testCase.expected) {
				t.Errorf("patterns=%v\ngot:  %v\nwant: %v", testCase.patterns, got, testCase.expected)
			}
		})
	}
}

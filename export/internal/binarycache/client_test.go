package binarycache

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type fakeTransport struct {
	m             sync.Mutex
	urlToStatus   map[string]int
	requestedURLs []string
}

func (f *fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.m.Lock()
	defer f.m.Unlock()
	f.requestedURLs = append(f.requestedURLs, req.Method+" "+req.URL.String())
	status, ok := f.urlToStatus[req.URL.String()]
	if !ok {
		return nil, errors.New("connection refused")
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(""))}, nil
}

func TestFindAvailablePaths(t *testing.T) {
	const (
		cachedPath   = "/nix/store/8ivrpmp3arvxbr6imdwm2d28q9cjsqvi-bash-5.2p37"
		uncachedPath = "/nix/store/im5v8wkzivxnz97cjv8594lz9w09v2xw-testproject-0.0.1"
	)
	tests := []struct {
		name        string
		urls        []string
		urlToStatus map[string]int
		expected    map[string]bool
	}{
		{
			name: "paths with a narinfo are available, and paths without are not",
			urls: []string{"https://cache.nixos.org/"},
			urlToStatus: map[string]int{
				"https://cache.nixos.org/8ivrpmp3arvxbr6imdwm2d28q9cjsqvi.narinfo": http.StatusOK,
				"https://cache.nixos.org/im5v8wkzivxnz97cjv8594lz9w09v2xw.narinfo": http.StatusNotFound,
			},
			expected: map[string]bool{cachedPath: true, uncachedPath: false},
		},
		{
			name: "a path held by a later cache is available",
			urls: []string{"https://cache.nixos.org", "https://example.com/cache"},
			urlToStatus: map[string]int{
				"https://cache.nixos.org/8ivrpmp3arvxbr6imdwm2d28q9cjsqvi.narinfo":   http.StatusNotFound,
				"https://cache.nixos.org/im5v8wkzivxnz97cjv8594lz9w09v2xw.narinfo":   http.StatusUnauthorized,
				"https://example.com/cache/8ivrpmp3arvxbr6imdwm2d28q9cjsqvi.narinfo": http.StatusOK,
				"https://example.com/cache/im5v8wkzivxnz97cjv8594lz9w09v2xw.narinfo": http.StatusForbidden,
			},
			expected: map[string]bool{cachedPath: true, uncachedPath: false},
		},
		{
			name: "paths whose lookups fail are unavailable",
			urls: []string{"https://cache.nixos.org"},
			urlToStatus: map[string]int{
				"https://cache.nixos.org/8ivrpmp3arvxbr6imdwm2d28q9cjsqvi.narinfo": http.StatusInternalServerError,
			},
			expected: map[string]bool{cachedPath: false, uncachedPath: false},
		},
		{
			name:     "caches that are not served over HTTP are not queried",
			urls:     []string{"s3://bucket", "file:///tmp/cache"},
			expected: map[string]bool{cachedPath: false, uncachedPath: false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClient(slog.New(slog.DiscardHandler), tt.urls)
			c.HTTPClient = &http.Client{Transport: &fakeTransport{urlToStatus: tt.urlToStatus}}
			got, err := c.FindAvailablePaths(context.Background(), []string{cachedPath, uncachedPath})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.expected, got); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestFindAvailablePathsUsesHeadRequests(t *testing.T) {
	transport := &fakeTransport{urlToStatus: map[string]int{
		"https://cache.nixos.org/8ivrpmp3arvxbr6imdwm2d28q9cjsqvi.narinfo": http.StatusOK,
	}}
	c := NewClient(slog.New(slog.DiscardHandler), []string{"https://cache.nixos.org/"})
	c.HTTPClient = &http.Client{Transport: transport}
	if _, err := c.FindAvailablePaths(context.Background(), []string{"/nix/store/8ivrpmp3arvxbr6imdwm2d28q9cjsqvi-bash-5.2p37"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"HEAD https://cache.nixos.org/8ivrpmp3arvxbr6imdwm2d28q9cjsqvi.narinfo"}
	if diff := cmp.Diff(expected, transport.requestedURLs); diff != "" {
		t.Error(diff)
	}
}

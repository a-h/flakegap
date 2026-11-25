package binarycache

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"
)

// Client checks whether HTTP binary caches can substitute store paths.
type Client struct {
	log  *slog.Logger
	urls []string
	// HTTPClient makes the requests to the binary caches.
	HTTPClient *http.Client
	// Concurrency is the maximum number of requests in flight.
	Concurrency int
}

// NewClient creates a Client for the binary caches at urls. Caches that are not served over HTTP or HTTPS are
// skipped, so paths that only they hold are reported as unavailable.
func NewClient(log *slog.Logger, urls []string) *Client {
	var httpURLs []string
	for _, u := range urls {
		if !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
			log.Warn("Skipping binary cache that is not served over HTTP", slog.String("url", u))
			continue
		}
		httpURLs = append(httpURLs, strings.TrimSuffix(u, "/"))
	}
	return &Client{
		log:         log,
		urls:        httpURLs,
		HTTPClient:  &http.Client{Timeout: 30 * time.Second},
		Concurrency: 16,
	}
}

// FindAvailablePaths returns whether any of the binary caches can substitute each store path.
//
// A path whose lookup fails is reported as unavailable. A caller that exports the build inputs of unavailable
// paths then exports more than it needs to, rather than less.
func (c *Client) FindAvailablePaths(ctx context.Context, paths []string) (pathToAvailable map[string]bool, err error) {
	pathToAvailable = make(map[string]bool, len(paths))
	var m sync.Mutex
	var lookupErrs []error
	sem := make(chan struct{}, c.Concurrency)
	var wg sync.WaitGroup
	for _, p := range paths {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			available, errs := c.checkPath(ctx, p)
			m.Lock()
			defer m.Unlock()
			pathToAvailable[p] = available
			lookupErrs = append(lookupErrs, errs...)
		})
	}
	wg.Wait()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if len(lookupErrs) > 0 {
		c.log.Warn("Some binary cache lookups failed, so the export includes the build inputs of those paths",
			slog.Int("failedLookups", len(lookupErrs)),
			slog.Any("error", errors.Join(lookupErrs[:min(len(lookupErrs), 3)]...)))
	}
	return pathToAvailable, nil
}

func (c *Client) checkPath(ctx context.Context, storePath string) (available bool, errs []error) {
	hash, _, ok := strings.Cut(path.Base(storePath), "-")
	if !ok {
		return false, []error{fmt.Errorf("invalid store path %q", storePath)}
	}
	for _, u := range c.urls {
		available, err := c.checkNARInfo(ctx, u+"/"+hash+".narinfo")
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if available {
			return true, nil
		}
	}
	return false, errs
}

func (c *Client) checkNARInfo(ctx context.Context, url string) (available bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request for %q: %w", url, err)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to request %q: %w", url, err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound, http.StatusForbidden, http.StatusUnauthorized:
		return false, nil
	}
	return false, fmt.Errorf("unexpected status %d from %q", resp.StatusCode, url)
}

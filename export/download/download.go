package download

import (
	"context"
	"errors"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type File struct {
	URL            string
	TargetFileName string
	Hash           string
}

func Files(ctx context.Context, log *slog.Logger, concurrency int, newHash func() hash.Hash, files <-chan File) error {
	start := time.Now()
	var downloaded int64
	var fromCache int64

	var wg sync.WaitGroup
	wg.Add(concurrency)
	errs := make([]error, concurrency)

	done := make(chan struct{}, 1)
	go func() {
		wg.Wait()
		done <- struct{}{}
		close(done)
	}()

	for i := range concurrency {
		go func(i int) {
			defer wg.Done()
			for file := range files {
				if ctx.Err() != nil {
					errs[i] = ctx.Err()
					return
				}
				if isAlreadyDownloaded(file.TargetFileName, file.Hash, newHash) {
					atomic.AddInt64(&fromCache, 1)
					continue
				}
				hash, err := downloadFile(file.URL, file.TargetFileName, newHash)
				if err != nil {
					errs[i] = fmt.Errorf("failed to download %s: %w", file.URL, err)
					return
				}
				if hash != file.Hash {
					errs[i] = fmt.Errorf("hash mismatch for %s: expected %s, got %s", file.URL, file.Hash, hash)
					return
				}
				atomic.AddInt64(&downloaded, 1)
			}
		}(i)
	}

	ticker := time.NewTicker(time.Second * 1)
	defer ticker.Stop()
exit:
	for {
		select {
		case <-ctx.Done():
			log.Info("Context cancelled, stopping downloads")
			return nil
		case <-done:
			break exit
		case <-ticker.C:
			log.Info("In progress", slog.Int64("downloaded", downloaded), slog.Int64("cached", fromCache), slog.String("duration", time.Since(start).String()))
		}
	}

	log.Info("Complete", slog.Int64("downloaded", downloaded), slog.Int64("cached", fromCache), slog.String("duration", time.Since(start).String()))

	return errors.Join(errs...)
}

func isAlreadyDownloaded(fileName, hash string, newHash func() hash.Hash) bool {
	r, err := os.Open(fileName)
	if err != nil {
		return false
	}
	defer r.Close()
	hasher := newHash()
	if _, err := io.Copy(hasher, r); err != nil {
		return false
	}
	return hash == fmt.Sprintf("%x", hasher.Sum(nil))
}

func downloadFile(fromURL, toFileName string, newHash func() hash.Hash) (sum string, err error) {
	// Get the file.
	resp, err := http.Get(fromURL)
	if err != nil {
		return "", fmt.Errorf("failed to download %s: %w", fromURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download %s: status code %d", fromURL, resp.StatusCode)
	}

	// Create the target file.
	file, err := os.Create(toFileName)
	if err != nil {
		return "", fmt.Errorf("failed to create file %s: %w", toFileName, err)
	}
	defer file.Close()

	// Hash the file while we download it.
	hasher := newHash()
	if _, err := io.Copy(io.MultiWriter(file, hasher), resp.Body); err != nil {
		return "", fmt.Errorf("failed to write to file %s: %w", toFileName, err)
	}

	// Return.
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

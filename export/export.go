package export

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/a-h/flakegap/container"
	"github.com/a-h/flakegap/nixserve"
	"github.com/nix-community/go-nix/pkg/narinfo"
	cp "github.com/otiai10/copy"
)

type Args struct {
	// Code is the path to the repo on disk that contains a flake.nix file.
	Code string
	// ExportFileName is the path to write the output to, e.g. /tmp/nix-export.tar.gz.
	ExportFileName string
	// Image is the image to run, defaults to ghcr.io/a-h/flakegap:latest.
	Image string
	// BinaryCacheAddr is the listen address of the binary cache to use, defaults to localhost:41805
	BinaryCacheAddr string
<<<<<<< HEAD
	// Help shows usage and quits.
	Help bool
=======
	// Platform is the platform to run the container on, e.g. linux/amd64 (default).
	Platform string
>>>>>>> 56ecd95 (feat: support building x86_64 on aarch64 machines, including Darwin)
}

func (a Args) Validate() error {
	var errs []error
	if a.Code == "" {
		errs = append(errs, fmt.Errorf("source-path is required"))
	}
	if a.ExportFileName == "" {
		errs = append(errs, fmt.Errorf("export-filename is required"))
	}
	if a.Image == "" {
		errs = append(errs, fmt.Errorf("image is required"))
	}
	if a.BinaryCacheAddr == "" {
		errs = append(errs, fmt.Errorf("binary-cache-addr is required"))
	}
	return errors.Join(errs...)
}

func Run(ctx context.Context, log *slog.Logger, args Args) (err error) {
	var wg sync.WaitGroup

	platform, err := container.NewPlatform(args.Platform)
	if err != nil {
		return err
	}

	log.Info("Starting binary cache")

	binaryCacheURL := (&url.URL{
		Scheme:   "http",
		Host:     args.BinaryCacheAddr,
		RawQuery: "trusted=1",
	}).String()

	server := &http.Server{
		Addr: args.BinaryCacheAddr,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		h, closer, err := nixserve.New(log)
		if err != nil {
			log.Error("Failed to create nixserve", slog.Any("error", err))
			return
		}
		defer closer()
		server.Handler = h
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Failed to start binary cache", slog.Any("error", err))
		}
	}()

	log.Info("Waiting for binary cache to start...")

loop:
	for i := 0; i < 120; i++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled")
		case <-time.After(1 * time.Second):
			_, err := http.Get(binaryCacheURL)
			if err != nil {
				log.Info("Binary cache not ready", slog.Any("error", err))
				continue loop
			}
			log.Info("Binary cache ready")
			break loop
		}
	}

	log.Info("Running container", slog.String("platform", platform.String()), slog.String("image", args.Image))

	nixExportPath, err := os.MkdirTemp("", "flakegap")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(nixExportPath)

	if err = container.Run(ctx, log, args.Image, "export", args.Code, nixExportPath, binaryCacheURL, platform); err != nil {
		return fmt.Errorf("failed to run container: %w", err)
	}

	log.Info("Copying source code")
	srcOutputDir := filepath.Join(nixExportPath, "source")
	if err := os.MkdirAll(srcOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create source output directory: %w", err)
	}
	ignore := []string{".direnv", "result"}
	symlinks := make(map[string]struct{})
	opt := cp.Options{
		Skip: func(srcinfo os.FileInfo, src, dest string) (bool, error) {
			for _, ignored := range ignore {
				if srcinfo.Name() == ignored {
					return true, nil
				}
			}
			return false, nil
		},
		OnSymlink: func(src string) cp.SymlinkAction {
			symlinks[src] = struct{}{}
			return cp.Deep
		},
		OnError: func(src, dest string, err error) error {
			if _, ok := symlinks[src]; ok {
				// Ignore errors for symlinks.
				log.Warn("Ignoring symlink error", slog.String("src", src), slog.String("dest", dest), slog.Any("error", err))
				return nil
			}
			return err
		},
		Sync: true,
	}
	if err := cp.Copy(args.Code, srcOutputDir, opt); err != nil {
		return fmt.Errorf("failed to copy source code: %w", err)
	}

	log.Info("Collecting store paths")
	if err = writeManifest(ctx, nixExportPath); err != nil {
		return fmt.Errorf("failed to get store paths: %w", err)
	}

	log.Info("Archiving output")
	if err = archive(ctx, nixExportPath, args.ExportFileName); err != nil {
		return fmt.Errorf("failed to archive: %w", err)
	}

	log.Info("Shutting down binary cache")
	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown binary cache: %w", err)
	}
	wg.Wait()

	log.Info("Complete")
	return nil
}

func getArchitecture() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	default:
		return runtime.GOARCH
	}
}

func writeManifest(ctx context.Context, nixExportPath string) (err error) {
	exportManifestFileName := filepath.Join(nixExportPath, fmt.Sprintf("nix-export-%s-%s.txt", getArchitecture(), runtime.GOOS))
	w, err := os.Create(exportManifestFileName)
	if err != nil {
		return fmt.Errorf("failed to create manifest file: %w", err)
	}
	defer w.Close()

	return filepath.Walk(nixExportPath, func(path string, info os.FileInfo, err error) error {
		if cancel := ctx.Err(); cancel != nil {
			return cancel
		}
		if err != nil {
			return err
		}

		if filepath.Ext(path) != ".narinfo" || info.IsDir() {
			return nil
		}

		r, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open %q: %w", path, err)
		}
		defer r.Close()
		ni, err := narinfo.Parse(r)
		if err != nil {
			return fmt.Errorf("failed to parse narinfo %q: %w", path, err)
		}
		if _, err = fmt.Fprintf(w, "%s\n", ni.StorePath); err != nil {
			return fmt.Errorf("failed to write store path %q: %w", ni.StorePath, err)
		}
		return nil
	})
}

func archive(ctx context.Context, srcPath, tgtPath string) (err error) {
	f, err := os.Create(tgtPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer f.Close()

	zw := gzip.NewWriter(f)
	tw := tar.NewWriter(zw)

	err = filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if cancel := ctx.Err(); cancel != nil {
			return cancel
		}
		if err != nil {
			return err
		}

		name, err := filepath.Rel(srcPath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		hdr := &tar.Header{
			Name:     name,
			Size:     info.Size(),
			Typeflag: tar.TypeReg,
			Mode:     0644,
		}
		if info.IsDir() {
			hdr.Typeflag = tar.TypeDir
			hdr.Mode = 0755
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("failed to write tar header: %w", err)
		}
		if !info.IsDir() {
			data, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open file %q: %w", path, err)
			}
			if _, err := io.Copy(tw, data); err != nil {
				return fmt.Errorf("failed to copy file %q: %w", path, err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk source path: %w", err)
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return nil
}

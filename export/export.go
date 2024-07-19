package export

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/a-h/flakegap/container"
)

type Args struct {
	// Code is the path to the repo on disk that contains a flake.nix file.
	Code string
	// ExportFileName is the path to write the output to, e.g. /tmp/nix-export.tar.gz.
	ExportFileName string
}

func (a Args) Validate() error {
	var errs []error
	if a.Code == "" {
		errs = append(errs, fmt.Errorf("source-path is required"))
	}
	if a.ExportFileName == "" {
		errs = append(errs, fmt.Errorf("target-path is required"))
	}
	return errors.Join(errs...)
}

func Run(ctx context.Context, log *slog.Logger, args Args) (err error) {
	log.Info("Running container")

	tgtPath, err := os.MkdirTemp("", "flakegap")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tgtPath)

	if err = container.Run(ctx, "export", args.Code, tgtPath); err != nil {
		return fmt.Errorf("failed to run container: %w", err)
	}

	log.Info("Collecting output")
	if err = archive(ctx, tgtPath, args.ExportFileName); err != nil {
		return fmt.Errorf("failed to archive: %w", err)
	}

	log.Info("Complete")
	return nil
}

func archive(ctx context.Context, srcPath, tgtPath string) (err error) {
	f, err := os.Create(tgtPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer f.Close()

	zw := gzip.NewWriter(f)
	tw := tar.NewWriter(zw)

	filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
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

	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return nil
}

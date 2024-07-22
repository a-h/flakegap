package validate

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
	// ExportFileName is the path to the `nix-export.tar.gz` file created by the export command.
	ExportFileName string
	// Image is the image to run, defaults to ghcr.io/a-h/flakegap:latest.
	Image string
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
	return errors.Join(errs...)
}

func Run(ctx context.Context, log *slog.Logger, args Args) (err error) {
	log.Info("Extracting nix export to temp dir")

	tgtPath, err := os.MkdirTemp("", "flakegap")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tgtPath)
	m, err := unarchive(ctx, args.ExportFileName, tgtPath)
	if err != nil {
		return fmt.Errorf("failed to unarchive: %w", err)
	}
	log.Info("Extracted archive", slog.Int("files", m.Files), slog.Int("dirs", m.Dirs))

	log.Info("Running build in airgapped container")

	if err = container.Run(ctx, args.Image, "validate", args.Code, tgtPath); err != nil {
		return fmt.Errorf("failed to run container: %w", err)
	}

	log.Info("Complete")
	return nil
}

type Metrics struct {
	Files int
	Dirs  int
}

func unarchive(ctx context.Context, src, dst string) (m Metrics, err error) {
	file, err := os.Open(src)
	if err != nil {
		return m, fmt.Errorf("failed to open .tar.gz file %q: %w", src, err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return m, err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return m, err
		}

		target := filepath.Join(dst, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			m.Dirs++
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return m, err
			}
		case tar.TypeReg:
			m.Files++
			if err := os.MkdirAll(filepath.Dir(target), os.ModePerm); err != nil {
				return m, err
			}
			outFile, err := os.Create(target)
			if err != nil {
				return m, err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return m, err
			}
			outFile.Close()
			if err := os.Chmod(target, os.FileMode(header.Mode)); err != nil {
				return m, err
			}
		default:
			return m, fmt.Errorf("unknown type: %v in %s", header.Typeflag, header.Name)
		}
	}
	return m, nil
}

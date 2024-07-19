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
	// SourcePath to the repo on disk that contains a flake.nix file.
	SourcePath string
	// NixExportPath is the path to the `nix-export.tar.gz` file created by the export command.
	NixExportPath string
}

func (a Args) Validate() error {
	var errs []error
	if a.SourcePath == "" {
		errs = append(errs, fmt.Errorf("source-path is required"))
	}
	if a.NixExportPath == "" {
		errs = append(errs, fmt.Errorf("nix-export-path is required"))
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
	if err = unarchive(ctx, args.NixExportPath, tgtPath); err != nil {
		return fmt.Errorf("failed to unarchive: %w", err)
	}

	log.Info("Running build in airgapped container")

	if err = container.Run(ctx, "validate", args.SourcePath, tgtPath); err != nil {
		return fmt.Errorf("failed to run container: %w", err)
	}

	log.Info("Complete")
	return nil
}

func unarchive(ctx context.Context, src, dst string) (err error) {
	file, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open .tar.gz file %q: %w", src, err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dst, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), os.ModePerm); err != nil {
				return err
			}
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
			if err := os.Chmod(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown type: %v in %s", header.Typeflag, header.Name)
		}
	}
	return nil
}

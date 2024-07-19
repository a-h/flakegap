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
	"github.com/nix-community/go-nix/pkg/narinfo"
)

type Args struct {
	// Code is the path to the repo on disk that contains a flake.nix file.
	Code string
	// ExportFileName is the path to write the output to, e.g. /tmp/nix-export.tar.gz.
	ExportFileName string
	// ExportManifestFileName is the path to write the manifest to, e.g. /tmp/nix-export.txt
	ExportManifestFileName string
}

func (a Args) Validate() error {
	var errs []error
	if a.Code == "" {
		errs = append(errs, fmt.Errorf("source-path is required"))
	}
	if a.ExportFileName == "" {
		errs = append(errs, fmt.Errorf("export-filename is required"))
	}
	if a.ExportManifestFileName == "" {
		errs = append(errs, fmt.Errorf("manifest-filename is required"))
	}
	return errors.Join(errs...)
}

func Run(ctx context.Context, log *slog.Logger, args Args) (err error) {
	log.Info("Running container")

	nixExportPath, err := os.MkdirTemp("", "flakegap")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(nixExportPath)

	if err = container.Run(ctx, "export", args.Code, nixExportPath); err != nil {
		return fmt.Errorf("failed to run container: %w", err)
	}

	log.Info("Collecting store paths")
	if err = writeManifest(ctx, nixExportPath, args.ExportManifestFileName); err != nil {
		return fmt.Errorf("failed to get store paths: %w", err)
	}

	log.Info("Archiving output")
	if err = archive(ctx, nixExportPath, args.ExportFileName); err != nil {
		return fmt.Errorf("failed to archive: %w", err)
	}

	log.Info("Complete")
	return nil
}

func writeManifest(ctx context.Context, nixExportPath, exportManifestFileName string) (err error) {
	w, err := os.Create(exportManifestFileName)
	if err != nil {
		return fmt.Errorf("failed to create manifest file: %w", err)
	}
	defer w.Close()

	return filepath.Walk(nixExportPath, func(path string, info os.FileInfo, err error) error {
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

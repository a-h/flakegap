package validate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/a-h/flakegap/archive"
	"github.com/a-h/flakegap/container"
)

type Args struct {
	// ExportFileName is the path to the `nix-export.tar.gz` file created by the export command.
	ExportFileName string
	// Image is the image to run, defaults to ghcr.io/a-h/flakegap:latest.
	Image string
	// Help shows usage and quits.
	Help bool
	// Architecture is the architecture to build for, e.g. x86_64, aarch64.
	Architecture string
	// Platform is the platform to run the container on, e.g. linux, darwin.
	Platform string
}

func (a Args) Validate() error {
	var errs []error
	if a.ExportFileName == "" {
		errs = append(errs, fmt.Errorf("export-filename is required"))
	}
	if a.Image == "" {
		errs = append(errs, fmt.Errorf("image is required"))
	}
	if a.Platform == "" {
		errs = append(errs, fmt.Errorf("platform is required"))
	}
	if a.Architecture == "" {
		errs = append(errs, fmt.Errorf("architecture is required"))
	}
	return errors.Join(errs...)
}

func Run(ctx context.Context, log *slog.Logger, args Args) (err error) {
	log.Info("Extracting nix export to temp dir")

	containerPlatform, err := container.NewPlatform(args.Platform)
	if err != nil {
		return err
	}

	tgtPath, err := os.MkdirTemp("", "flakegap")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tgtPath)
	m, err := archive.Unarchive(ctx, args.ExportFileName, tgtPath)
	if err != nil {
		return fmt.Errorf("failed to unarchive: %w", err)
	}
	log.Info("Extracted archive", slog.Int("files", m.Files), slog.Int("dirs", m.Dirs))

	log.Info("Running build in airgapped container without binary cache", slog.String("platform", containerPlatform.String()), slog.String("image", args.Image))

	codePath := filepath.Join(tgtPath, "source")
	if err = container.Run(ctx, log, containerPlatform, args.Image, codePath, tgtPath, args.Architecture, args.Platform); err != nil {
		return fmt.Errorf("failed to run container: %w", err)
	}

	log.Info("Complete")
	return nil
}

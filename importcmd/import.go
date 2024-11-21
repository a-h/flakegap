package importcmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/a-h/flakegap/archive"
	"github.com/a-h/flakegap/nixcmd"
)

type Args struct {
	// ImportFileName is the path to the `nix-export.tar.gz` file created by the export command.
	ImportFileName string
	// TemporaryPath to export the files to.
	TemporaryPath string
	// Help shows usage and quits.
	Help bool
}

func (a Args) Validate() error {
	var errs []error
	if a.ImportFileName == "" {
		errs = append(errs, fmt.Errorf("import-filename is required"))
	}
	return errors.Join(errs...)
}

func getTemporaryPath(log *slog.Logger, current string) (updated string) {
	if current != "" {
		return current
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	log.Warn("Home directory not found, using system temp directory which may be too small for large builds.")
	return ""
}

func Run(ctx context.Context, log *slog.Logger, args Args) (err error) {
	nixExportPath, err := os.MkdirTemp(getTemporaryPath(log, args.TemporaryPath), "flakegap")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(nixExportPath)

	m, err := archive.Unarchive(ctx, args.ImportFileName, nixExportPath)
	log.Info("Extracted archive", slog.String("import-filename", args.ImportFileName), slog.Int("files", m.Files), slog.Int("dirs", m.Dirs))

	// Check for presence of the nix-store directory in the extracted directory.
	nixStorePath := filepath.Join(nixExportPath, "nix-store")
	if _, err := os.Stat(nixStorePath); err != nil {
		return fmt.Errorf("nix-store directory not found in extracted archive: %w", err)
	}

	sourceStore := fmt.Sprintf("file://%s", nixStorePath)
	log.Info("Restoring Nix store from export", slog.String("nix-store", nixStorePath))

	// nix copy --all --no-check-sigs --from file:///nix-export/nix-store/
	// nix copy --all --derivation --no-check-sigs --from file:///nix-export/nix-store/
	if err = nixcmd.CopyFromAll(os.Stdout, os.Stderr, "", sourceStore); err != nil {
		return fmt.Errorf("failed to copy from /nix-export/nix-store: %w", err)
	}

	return nil
}

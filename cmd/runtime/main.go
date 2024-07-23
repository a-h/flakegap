package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/a-h/flakegap/nixcmd"
)

var version string

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	log = log.With(slog.String("version", version))
	log = log.With(slog.String("flakegap", "server"))

	mode := "export"
	if len(os.Args) < 2 {
		log.Info("No mode specified, defaulting to export")
	}
	if len(os.Args) >= 2 {
		mode = os.Args[1]
	}
	if mode != "export" && mode != "validate" {
		log.Warn("Invalid mode, defaulting to export", slog.String("mode", mode))
		mode = "export"
	}
	if err := run(log, mode); err != nil {
		log.Error("fatal error", slog.Any("error", err))
		os.Exit(1)
	}
	log.Info("Runtime complete")
}

func run(log *slog.Logger, mode string) (err error) {
	if mode == "validate" {
		// nix copy --all --offline --impure --no-check-sigs --from file:///nix-export/
		if err = nixcmd.CopyFrom(os.Stdout, os.Stderr); err != nil {
			return fmt.Errorf("failed to copy from /nix-export: %w", err)
		}
	}

	log.Info("Gathering Nix outputs")
	// nix flake show --json
	op, err := nixcmd.FlakeShow(os.Stdout, os.Stderr)
	if err != nil {
		return fmt.Errorf("failed to gather nix outputs: %w", err)
	}
	drvs := op.Derivations()

	log.Info("Building", slog.Any("outputs", drvs))
	var pathsToDelete []string
	for _, ref := range drvs {
		log.Info("Building", slog.String("ref", ref))
		// ALLOW_UNFREE=1 nix build --no-link --impure <ref>
		if err := nixcmd.Build(os.Stdout, os.Stderr, ref); err != nil {
			log.Error("failed to build", slog.String("ref", ref), slog.Any("error", err))
			return fmt.Errorf("failed to build %q: %w", ref, err)
		}
		// nix path-info --json <ref>
		path, err := nixcmd.PathInfo(os.Stdout, os.Stderr, ref)
		if err != nil {
			return fmt.Errorf("failed to get path info for %q: %w", ref, err)
		}
		pathsToDelete = append(pathsToDelete, path)
	}
	// Delete output paths.
	// nix store delete <path> <path> <path>
	if err := nixcmd.StoreDelete(os.Stdout, os.Stderr, pathsToDelete); err != nil {
		log.Error("failed to remove paths", slog.Any("error", err))
		return fmt.Errorf("failed to remove paths: %w", err)
	}

	if mode == "validate" {
		return
	}

	log.Info("Copying store to output")
	// nix copy --derivation --to file:///nix-export/ --all
	if err := nixcmd.CopyTo(os.Stdout, os.Stderr); err != nil {
		log.Error("failed to copy", slog.Any("error", err))
		return fmt.Errorf("failed to copy: %w", err)
	}

	log.Info("Copying flake archive to output")
	// nix flake archive --to file:///nix-export/
	if err := nixcmd.FlakeArchive(os.Stdout, os.Stderr); err != nil {
		log.Error("failed to archive flake", slog.Any("error", err))
		return fmt.Errorf("failed to archive flake: %w", err)
	}

	return nil
}

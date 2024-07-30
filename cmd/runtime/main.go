package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"log/slog"
	"os"

	"github.com/a-h/flakegap/nixcmd"
	cp "github.com/otiai10/copy"
)

var version string

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	log = log.With(slog.String("version", version))
	log = log.With(slog.String("flakegap", "server"))

	args := os.Args
	mode := "export"
	if len(args) < 2 {
		log.Info("No mode specified, defaulting to export")
		args = append(args, "export")
	}
	if len(os.Args) >= 2 {
		mode = args[1]
	}
	if mode != "export" && mode != "validate" {
		log.Warn("Invalid mode, defaulting to export", slog.String("mode", mode))
		mode = "export"
	}

	var substituter string
	cmdFlags := flag.NewFlagSet("runtime", flag.ContinueOnError)
	cmdFlags.StringVar(&substituter, "substituter", "", "Substituter to use")
	cmdFlags.Parse(args[2:])

	if err := run(log, mode, substituter); err != nil {
		log.Error("fatal error", slog.Any("error", err))
		os.Exit(1)
	}
	log.Info("Runtime complete")
}

func run(log *slog.Logger, mode string, substituter string) (err error) {
	var substituters []string
	if mode == "export" {
		if substituter == "" {
			log.Warn("No binary cache specified, downloading all dependencies")
			substituters = []string{"https://cache.nixos.org/"}
		} else {
			log.Info("Using binary cache", slog.String("substituter", substituter))
			substituters = []string{substituter, "https://cache.nixos.org/"}
		}
	}
	if mode == "validate" {
		log.Info("Restoring Nix store from export")
		// nix copy --all --offline --impure --no-check-sigs --from file:///nix-export/nix-store/
		if err = nixcmd.CopyFrom(os.Stdout, os.Stderr); err != nil {
			return fmt.Errorf("failed to copy from /nix-export/nix-store: %w", err)
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
		// ALLOW_UNFREE=1 nix build --impure <ref>
		if err := nixcmd.Build(os.Stdout, os.Stderr, ref, substituters); err != nil {
			log.Error("failed to build", slog.String("ref", ref), slog.Any("error", err))
			return fmt.Errorf("failed to build %q: %w", ref, err)
		}
		// nix path-info --json <ref>
		path, err := nixcmd.PathInfo(os.Stdout, os.Stderr, ref)
		if err != nil {
			return fmt.Errorf("failed to get path info for %q: %w", ref, err)
		}
		// Take a copy of the outputs.
		targetDirParts := strings.Split(strings.TrimPrefix(ref, ".#"), ".")
		target := filepath.Join(append([]string{"/nix-export/outputs/"}, targetDirParts...)...)
		if err := os.MkdirAll(target, 0755); err != nil {
			return fmt.Errorf("failed to create outputs directory %q: %w", target, err)
		}
		evaluatedPath, err := filepath.EvalSymlinks("./result")
		if err != nil {
			return fmt.Errorf("failed to evaluate symlinks for %q: %w", "./result", err)
		}
		fi, err := os.Stat(evaluatedPath)
		if err != nil {
			return fmt.Errorf("failed to stat %q: %w", path, err)
		}
		if !fi.IsDir() {
			target = filepath.Join(target, "result")
		}
		log.Info("Copying", slog.String("target", target))
		opt := cp.Options{
			OnSymlink: func(src string) cp.SymlinkAction {
				return cp.Deep
			},
			Sync: true,
		}
		if err := cp.Copy("./result", target, opt); err != nil {
			return fmt.Errorf("failed to copy output %q to %q: %w", "./result", target, err)
		}
		pathsToDelete = append(pathsToDelete, path)
	}
	// Delete output paths.
	// nix store delete <path> <path> <path>
	if err := nixcmd.StoreDelete(os.Stdout, os.Stderr, pathsToDelete); err != nil {
		log.Warn("failed to remove all paths, but continuing", slog.Any("error", err))
	}

	if mode == "validate" {
		return
	}

	log.Info("Copying store to output")
	// nix copy --derivation --to file:///nix-export/nix-store/ --all
	if err := nixcmd.CopyTo(os.Stdout, os.Stderr); err != nil {
		log.Error("failed to copy", slog.Any("error", err))
		return fmt.Errorf("failed to copy: %w", err)
	}

	log.Info("Copying flake archive to output")
	// nix flake archive --to file:///nix-export/nix-store/
	if err := nixcmd.FlakeArchive(os.Stdout, os.Stderr); err != nil {
		log.Error("failed to archive flake", slog.Any("error", err))
		return fmt.Errorf("failed to archive flake: %w", err)
	}

	return nil
}

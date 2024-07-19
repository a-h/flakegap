package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/a-h/flakegap/nixcmd"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	mode := "export"
	if len(os.Args) < 2 {
		log.Info("No mode specified, defaulting to export")
	}
	if len(os.Args) >= 2 {
		mode = os.Args[1]
	}
	if mode != "export" && mode != "validate" {
		log.Error("Invalid mode, defaulting to export", slog.String("mode", mode))
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
		if err = nixcmd.CopyFrom(os.Stdout, os.Stderr); err != nil {
			return fmt.Errorf("failed to copy from /nix-export: %w", err)
		}
	}

	log.Info("Gathering Nix outputs")
	op, err := nixcmd.FlakeShow()
	if err != nil {
		return err
	}
	drvs := op.Derivations()

	log.Info("Building", slog.Any("outputs", drvs))
	for _, ref := range drvs {
		log.Info("Building", slog.String("ref", ref))
		if err := nixcmd.Build(os.Stdout, os.Stderr, ref); err != nil {
			return fmt.Errorf("failed to build %q: %w", ref, err)
		}
	}

	if mode == "validate" {
		return
	}

	log.Info("Copying store to output")
	if err := nixcmd.CopyTo(os.Stdout, os.Stderr); err != nil {
		return fmt.Errorf("failed to copy: %w", err)
	}

	log.Info("Copying flake archive to output")
	if err := nixcmd.FlakeArchive(os.Stdout, os.Stderr); err != nil {
		return fmt.Errorf("failed to archive flake: %w", err)
	}

	return nil
}

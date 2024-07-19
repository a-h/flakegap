package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/a-h/flakegap/nixcmd"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		log.Error("fatal error", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(log *slog.Logger) (err error) {
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

	log.Info("Copying store to output")
	if err := nixcmd.Copy(os.Stdout, os.Stderr); err != nil {
		return fmt.Errorf("failed to copy: %w", err)
	}

	log.Info("Copying flake archive to output")
	if err := nixcmd.FlakeArchive(os.Stdout, os.Stderr); err != nil {
		return fmt.Errorf("failed to archive flake: %w", err)
	}

	log.Info("Complete")

	return nil
}

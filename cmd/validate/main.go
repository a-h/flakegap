package main

import (
	"flag"
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

	var architecture, platform string
	var codeDir string
	var sourceStore string
	cmdFlags := flag.NewFlagSet("runtime", flag.ContinueOnError)
	cmdFlags.StringVar(&architecture, "architecture", "x86_64", "Architecture to build for, e.g. x86_64, aarch64")
	cmdFlags.StringVar(&platform, "platform", "linux", "Platform to build for, e.g. linux, darwin")
	cmdFlags.StringVar(&codeDir, "code-dir", "/code", "Code directory")
	cmdFlags.StringVar(&sourceStore, "source-store", "file:///nix-export/nix-store/", "Source store")
	cmdFlags.Parse(os.Args[1:])

	if err := run(log, architecture, platform, codeDir, sourceStore); err != nil {
		log.Error("fatal error", slog.Any("error", err))
		os.Exit(1)
	}
	log.Info("Runtime complete")
}

func run(log *slog.Logger, architecture, platform, codeDir, sourceStore string) (err error) {
	log = log.With(slog.String("architecture", architecture), slog.String("platform", platform))
	log.Info("Restoring Nix store from export", slog.String("source-store", sourceStore))

	// nix copy --all --no-check-sigs --from file:///nix-export/nix-store/
	// nix copy --all --derivation --no-check-sigs --from file:///nix-export/nix-store/
	if err = nixcmd.CopyFromAll(os.Stdout, os.Stderr, codeDir, sourceStore); err != nil {
		return fmt.Errorf("failed to copy from /nix-export/nix-store: %w", err)
	}

	log.Info("Gathering Nix outputs")
	// nix flake show --json
	op, err := nixcmd.FlakeShow(os.Stdout, os.Stderr, codeDir)
	if err != nil {
		return fmt.Errorf("failed to gather nix outputs: %w", err)
	}
	drvs := op.Derivations(architecture, platform)

	log.Info("Building", slog.Any("outputs", drvs))
	for _, ref := range drvs {
		log.Info("Building", slog.String("ref", ref))
		// nix build <ref>
		if err := nixcmd.Build(os.Stdout, os.Stderr, codeDir, ref); err != nil {
			log.Error("failed to build", slog.String("ref", ref), slog.Any("error", err))
			return fmt.Errorf("failed to build %q: %w", ref, err)
		}
	}

	return nil
}
